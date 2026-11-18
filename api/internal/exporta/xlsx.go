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
	"Fundamentação", "Origem", "CST IS", "cClassTrib IS", "vCBS", "vIBS", "vIS", "Observação"}

var situacoes = map[string]string{"ok": "Pronto", "provavel": "Provável (confirmar)", "revisar": "Revisar", "invalido": "Inválido"}
var origens = map[string]string{"sugerido": "Sugerido", "confirmado": "Confirmado", "manual": "Manual"}

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
		c := calc[strconv.Itoa(ano)]
		var classif, red, fund string
		for _, o := range op.IbsCbs {
			if o.CClassTrib == it.CClassTrib {
				classif, red, fund = o.Descricao, strconv.FormatFloat(o.PercReducao, 'f', -1, 64), o.Fundamentacao
				break
			}
		}
		var obs []string
		for _, s := range []string{it.Condicao, c.Mensagem} {
			if s != "" {
				obs = append(obs, s)
			}
		}
		row := []any{it.Codigo, it.Descricao, it.NCM, it.NBS, situacoes[it.Situacao], it.CST, it.CClassTrib, classif, red, fund,
			origens[it.Origem], it.ISCST, it.ISCClassTrib, c.VCBS, c.VIBS, c.VIS, strings.Join(obs, "; ")}
		cel, _ := excelize.CoordinatesToCellName(1, n+2)
		f.SetSheetRow("Cadastro", cel, &row)
	}

	f.NewSheet("Parâmetros")
	p := [][]any{{"Cliente", cliente.RazaoSocial}, {"CNPJ", cliente.CNPJ}, {"Regime", cliente.Regime}, {"Ano", ano},
		{"Alíquota CBS (%)", al.CBS}, {"Alíquota IBS UF (%)", al.IBSUF}, {"Alíquota IBS município (%)", al.IBSMun}, {"Alíquota IS ad valorem (%)", al.ISAdValorem}}
	if ano >= 2027 {
		p = append(p, []any{"Simulação: alíquotas estimadas, não oficiais"})
	}
	for n, r := range p {
		cel, _ := excelize.CoordinatesToCellName(1, n+1)
		f.SetSheetRow("Parâmetros", cel, &r)
	}
	return f.Write(w)
}
