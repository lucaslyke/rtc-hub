package exporta

import (
	"bytes"
	"strings"
	"testing"

	"github.com/xuri/excelize/v2"

	"rtchub/internal/motores"
	"rtchub/internal/store"
)

func gera(t *testing.T, ano int) *excelize.File {
	t.Helper()
	it := store.Item{Codigo: "P1", Descricao: "Arroz", NCM: "10063021", Situacao: "provavel", Origem: "sugerido",
		CST: "200", CClassTrib: "200003", Condicao: "cesta",
		Opcoes:   []byte(`{"ibsCbs":[{"cst":"200","cClassTrib":"200003","descricao":"Cesta básica","fundamentacao":"Art. 125","percReducao":100}],"is":[]}`),
		Calculos: []byte(`{"2027":{"vCbs":"1.00","vIbs":"2.00","vIs":"0.00","mensagem":"estimado"}}`)}
	var b bytes.Buffer
	if err := Cadastro(&b, store.Cliente{RazaoSocial: "ACME", CNPJ: "123", Regime: "regular"}, []store.Item{it}, ano, motores.Aliquotas{CBS: 8.8}); err != nil {
		t.Fatal(err)
	}
	f, err := excelize.OpenReader(&b)
	if err != nil {
		t.Fatal(err)
	}
	return f
}

func TestExportaColunasEAviso(t *testing.T) {
	f := gera(t, 2027)
	if v, _ := f.GetCellValue("Cadastro", "A1"); v != "Código" {
		t.Fatalf("A1 = %q", v)
	}
	linha, _ := f.GetRows("Cadastro")
	l := linha[1]
	if l[4] != "Provável (confirmar)" || l[7] != "Cesta básica" || l[13] != "1.00" || l[16] != "cesta; estimado" {
		t.Fatalf("linha = %q", l)
	}
	tem := func(f *excelize.File) bool {
		rows, _ := f.GetRows("Parâmetros")
		return strings.Contains(strings.Join(func() (o []string) {
			for _, r := range rows {
				o = append(o, r...)
			}
			return
		}(), "|"), "Simulação")
	}
	if !tem(f) || tem(gera(t, 2026)) {
		t.Fatal("aviso de simulação errado")
	}
}
