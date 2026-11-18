// Package exporta gera a planilha do cadastro de itens classificados de um cliente.
package exporta

import (
	"encoding/json"
	"io"
	"strconv"
	"strings"

	"github.com/xuri/excelize/v2"

	"rtchub/internal/motores"
	"rtchub/internal/store"
)

var cabecalho = []any{"Código", "Descrição", "NCM", "NBS", "Situação", "CST", "cClassTrib", "Classificação", "% Redução",
	"Fundamentação", "Origem", "CST IS", "cClassTrib IS", "Valor base", "vCBS", "vIBS", "vIS", "Observação"}

var situacoes = map[string]string{"ok": "Pronto", "provavel": "Provável (confirmar)", "revisar": "Revisar", "invalido": "Inválido"}
var origens = map[string]string{"sugerido": "Sugerido", "confirmado": "Confirmado", "manual": "Manual"}
var regimes = map[string]string{"simples": "Simples Nacional", "regular": "Regime regular"}

// numero: valor do motor como número na planilha; vazio fica vazio.
func numero(s string) any {
	if v, err := strconv.ParseFloat(s, 64); err == nil {
		return v
	}
	return nil
}

func Cadastro(w io.Writer, cliente store.Cliente, itens []store.Item, ano int, al motores.Aliquotas) error {
	f := excelize.NewFile()
	defer f.Close()
	f.SetSheetName("Sheet1", "Cadastro")
	f.SetSheetRow("Cadastro", "A1", &cabecalho)
	for n, it := range itens {
		var op struct {
			IbsCbs []struct {
				CClassTrib, Descricao, Fundamentacao string
				PercReducao                          float64
			} `json:"ibsCbs"`
		}
		json.Unmarshal(it.Opcoes, &op) // opcoes ausentes/ilegíveis: colunas ficam vazias
		var calc map[string]motores.Resultado
		json.Unmarshal(it.Calculos, &calc)
		c, calculado := calc[strconv.Itoa(ano)]
		calculado = calculado && it.Situacao == "ok" // valor de item ainda em revisão não é resultado
		if !calculado {
			c = motores.Resultado{Mensagem: "Não calculado para " + strconv.Itoa(ano)}
		}
		var classif, fund string
		var red any
		for _, o := range op.IbsCbs {
			if o.CClassTrib == it.CClassTrib {
				classif, red, fund = o.Descricao, o.PercReducao, o.Fundamentacao
				break
			}
		}
		var obs []string
		for _, s := range []string{it.Condicao, c.Mensagem} {
			if s != "" {
				obs = append(obs, s)
			}
		}
		base := it.UltimoValor
		if calculado && base <= 0 {
			base = 100 // a api calcula item sem valor sobre R$ 100 (está na Observação)
		}
		row := []any{it.Codigo, it.Descricao, it.NCM, it.NBS, situacoes[it.Situacao], it.CST, it.CClassTrib, classif, red, fund,
			origens[it.Origem], it.ISCST, it.ISCClassTrib, base, numero(c.VCBS), numero(c.VIBS), numero(c.VIS), strings.Join(obs, "; ")}
		cel, _ := excelize.CoordinatesToCellName(1, n+2)
		f.SetSheetRow("Cadastro", cel, &row)
	}
	if len(itens) > 0 {
		dinheiro, _ := f.NewStyle(&excelize.Style{NumFmt: 4}) // #,##0.00
		pct, _ := f.NewStyle(&excelize.Style{NumFmt: 2})      // 0.00
		fim := strconv.Itoa(len(itens) + 1)
		f.SetCellStyle("Cadastro", "N2", "Q"+fim, dinheiro)
		f.SetCellStyle("Cadastro", "I2", "I"+fim, pct)
	}

	f.NewSheet("Parâmetros")
	p := [][]any{{"Cliente", cliente.RazaoSocial}, {"CNPJ", cliente.CNPJ}, {"Regime", regimes[cliente.Regime]}, {"Ano", ano}}
	if cliente.Regime == "simples" { // o motor do Simples usa as alíquotas efetivas do cadastro, não as estimadas
		var pibs, pcbs any
		if cliente.PIBSSN != nil && cliente.PCBSSN != nil {
			pibs, pcbs = *cliente.PIBSSN, *cliente.PCBSSN
		}
		p = append(p, []any{"Atividade (cAtivSN)", cliente.CAtivSN}, []any{"Alíquota efetiva IBS no Simples (pIBSSN, %)", pibs},
			[]any{"Alíquota efetiva CBS no Simples (pCBSSN, %)", pcbs})
	} else {
		p = append(p, []any{"Alíquota CBS (%)", al.CBS}, []any{"Alíquota IBS UF (%)", al.IBSUF}, []any{"Alíquota IBS município (%)", al.IBSMun},
			[]any{"Alíquota IS ad valorem (%)", al.ISAdValorem})
	}
	if ano >= 2027 {
		p = append(p, []any{"Simulação: alíquotas estimadas, não oficiais"})
	}
	for n, r := range p {
		cel, _ := excelize.CoordinatesToCellName(1, n+1)
		f.SetSheetRow("Parâmetros", cel, &r)
	}
	return f.Write(w)
}
