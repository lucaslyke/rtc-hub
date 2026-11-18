package exporta

import (
	"bytes"
	"fmt"
	"strings"
	"testing"

	"github.com/xuri/excelize/v2"

	"rtchub/internal/motores"
	"rtchub/internal/store"
)

var item = store.Item{Codigo: "P1", Descricao: "Arroz", NCM: "10063021", Situacao: "ok", Origem: "confirmado",
	CST: "200", CClassTrib: "200003", Condicao: "cesta", UltimoValor: 12.5,
	Opcoes:   []byte(`{"ibsCbs":[{"cst":"200","cClassTrib":"200003","descricao":"Cesta básica","fundamentacao":"Art. 125","percReducao":100}],"is":[]}`),
	Calculos: []byte(`{"2027":{"vCbs":"1.00","vIbs":"2.00","vIs":"0.00","mensagem":"estimado"}}`)}

var regular = store.Cliente{RazaoSocial: "ACME", CNPJ: "123", Regime: "regular"}

func gera(t *testing.T, ano int) *excelize.File { return geraCom(t, ano, regular, item) }

func geraCom(t *testing.T, ano int, cli store.Cliente, its ...store.Item) *excelize.File {
	t.Helper()
	var b bytes.Buffer
	if err := Cadastro(&b, cli, its, ano, motores.Aliquotas{CBS: 8.8}); err != nil {
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
	linha, _ := f.GetRows("Cadastro", excelize.Options{RawCellValue: true})
	l := linha[1]
	if l[4] != "Pronto" || l[7] != "Cesta básica" || l[8] != "100" || l[13] != "12.5" || l[14] != "1" || l[17] != "cesta; estimado" {
		t.Fatalf("linha = %q", l)
	}
	if tp, _ := f.GetCellType("Cadastro", "O2"); tp != excelize.CellTypeNumber && tp != excelize.CellTypeUnset {
		t.Fatalf("vCBS não é número: %v", tp)
	}
	if v, _ := f.GetCellValue("Cadastro", "O2"); v != "1.00" { // formato #,##0.00
		t.Fatalf("vCBS formatado = %q", v)
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

func TestExportaSoValoresDeItemProntoENaoCalculado(t *testing.T) {
	rev := item
	rev.Situacao, rev.Codigo = "revisar", "P2"
	f := geraCom(t, 2027, regular, rev)
	l, _ := f.GetRows("Cadastro")
	if len(l[1]) > 14 && l[1][14] != "" || !strings.HasSuffix(l[1][len(l[1])-1], "Não calculado para 2027") {
		t.Fatalf("item em revisão: %q", l[1])
	}
	f = gera(t, 2028)
	l, _ = f.GetRows("Cadastro")
	if l[1][14] != "" || l[1][17] != "cesta; Não calculado para 2028" {
		t.Fatalf("ano sem cálculo: %q", l[1])
	}
}

func TestExportaParametrosDoSimples(t *testing.T) {
	ibs, cbs := 1.25, 3.0
	f := geraCom(t, 2027, store.Cliente{RazaoSocial: "S", Regime: "simples", CAtivSN: "01", PIBSSN: &ibs, PCBSSN: &cbs}, item)
	rows, _ := f.GetRows("Parâmetros")
	tudo := fmt.Sprint(rows)
	if !strings.Contains(tudo, "Simples Nacional") || !strings.Contains(tudo, "pIBSSN") || !strings.Contains(tudo, "1.25") || strings.Contains(tudo, "Alíquota CBS (%)") {
		t.Fatalf("%v", rows)
	}
	rows, _ = gera(t, 2027).GetRows("Parâmetros")
	if !strings.Contains(fmt.Sprint(rows), "Regime regular") {
		t.Fatalf("%v", rows)
	}
}
