package importa

import (
	"bytes"
	"encoding/csv"
	"io"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/xuri/excelize/v2"
	"golang.org/x/text/encoding/charmap"
)

// Mapeamento guarda o índice de cada coluna; -1 = ausente.
type Mapeamento struct{ Descricao, Codigo, NCM, NBS, Valor int }

type Previa struct {
	Cabecalho []string
	Linhas    [][]string // até 5 linhas
}

func lerLinhas(nome string, r io.Reader) ([][]string, error) {
	if strings.HasSuffix(strings.ToLower(nome), ".xlsx") {
		f, err := excelize.OpenReader(r)
		if err != nil {
			return nil, err
		}
		defer f.Close()
		return f.GetRows(f.GetSheetName(0)) // primeira aba
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

func parseValor(s string) float64 {
	s = strings.TrimSpace(s)
	if strings.Contains(s, ",") { // formato BR: 1.234,56
		s = strings.ReplaceAll(strings.ReplaceAll(s, ".", ""), ",", ".")
	}
	v, _ := strconv.ParseFloat(s, 64)
	return v
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
		it := ItemBruto{Codigo: col(m.Codigo), Descricao: col(m.Descricao), NCM: col(m.NCM), NBS: col(m.NBS), Valor: parseValor(col(m.Valor)), Origem: "planilha"}
		if it.NCM == "" && it.NBS == "" {
			if strings.Join(row, "") != "" { // linha totalmente vazia não é item
				ign = append(ign, Ignorado{nome, "linha " + strconv.Itoa(i+2) + ": sem NCM/NBS"})
			}
			continue
		}
		itens = append(itens, it)
	}
	return itens, ign, nil
}
