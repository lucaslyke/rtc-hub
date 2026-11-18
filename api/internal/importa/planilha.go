package importa

import (
	"bytes"
	"encoding/csv"
	"io"
	"regexp"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/xuri/excelize/v2"
	"golang.org/x/text/encoding/charmap"
)

// Mapeamento guarda o índice de cada coluna; -1 = ausente.
type Mapeamento struct{ Descricao, Codigo, NCM, NBS, Valor int }

type Previa struct {
	Cabecalho []string   `json:"cabecalho"`
	Linhas    [][]string `json:"linhas"` // até 5 linhas
}

func lerLinhas(nome string, r io.Reader) ([][]string, error) {
	if strings.HasSuffix(strings.ToLower(nome), ".xlsx") {
		f, err := excelize.OpenReader(r)
		if err != nil {
			return nil, err
		}
		defer f.Close()
		return f.GetRows(f.GetSheetName(0), excelize.Options{RawCellValue: true}) // primeira aba
	}
	b, err := io.ReadAll(r)
	if err != nil {
		return nil, err
	}
	if !utf8.Valid(b) {
		if b, err = charmap.Windows1252.NewDecoder().Bytes(b); err != nil {
			return nil, err
		}
	}
	b = bytes.TrimPrefix(b, []byte("\xef\xbb\xbf"))
	first, _, _ := bytes.Cut(b, []byte("\n"))
	cr := csv.NewReader(bytes.NewReader(b))
	cr.Comma = ','
	if bytes.Count(first, []byte(";")) > bytes.Count(first, []byte(",")) {
		cr.Comma = ';'
	}
	cr.FieldsPerRecord = -1
	cr.LazyQuotes = true
	return cr.ReadAll()
}

func PreviaPlanilha(nome string, r io.Reader) (Previa, error) {
	rows, err := lerLinhas(nome, r)
	if err != nil || len(rows) == 0 {
		return Previa{}, err
	}
	p := Previa{Cabecalho: rows[0], Linhas: rows[1:]}
	if len(p.Linhas) > 5 {
		p.Linhas = p.Linhas[:5]
	}
	return p, nil
}

var milharBR = regexp.MustCompile(`^\d{1,3}(\.\d{3})+$`)

// parseValor: vazio = 0; sem vírgula, "1.234"/"1.234.567" são milhares BR; senão ponto decimal.
func parseValor(s string) (float64, bool) {
	s = strings.NewReplacer("R$", "", " ", "", "\u00a0", "").Replace(s)
	if s == "" {
		return 0, true
	}
	if strings.Contains(s, ",") || milharBR.MatchString(s) {
		s = strings.ReplaceAll(strings.ReplaceAll(s, ".", ""), ",", ".")
	}
	v, err := strconv.ParseFloat(s, 64)
	return v, err == nil
}

func LerPlanilha(nome string, r io.Reader, m Mapeamento) ([]ItemBruto, []Ignorado, error) {
	rows, err := lerLinhas(nome, r)
	if err != nil {
		return nil, nil, err
	}
	var itens []ItemBruto
	var ign []Ignorado
	for i, row := range rows[min(1, len(rows)):] {
		col := func(c int) string {
			if c < 0 || c >= len(row) {
				return ""
			}
			return strings.TrimSpace(row[c])
		}
		it := ItemBruto{Codigo: col(m.Codigo), Descricao: col(m.Descricao), NCM: col(m.NCM), NBS: col(m.NBS), Origem: "planilha"}
		if it.NCM == "" && it.NBS == "" {
			if strings.TrimSpace(strings.Join(row, "")) != "" { // linha vazia não é item
				ign = append(ign, Ignorado{nome, "linha " + strconv.Itoa(i+2) + ": sem NCM/NBS"})
			}
			continue
		}
		v, ok := parseValor(col(m.Valor))
		if !ok {
			ign = append(ign, Ignorado{nome, "linha " + strconv.Itoa(i+2) + ": valor inválido"})
			continue
		}
		it.Valor = v
		itens = append(itens, it)
	}
	return itens, ign, nil
}
