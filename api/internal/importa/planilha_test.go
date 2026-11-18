package importa

import (
	"bytes"
	"os"
	"strings"
	"testing"

	"github.com/xuri/excelize/v2"
)

var mapa = Mapeamento{Codigo: 0, Descricao: 1, NCM: 2, NBS: -1, Valor: 3}

func TestCSVExcelBR(t *testing.T) {
	f, _ := os.Open("testdata/excel_br.csv")
	defer f.Close()
	itens, ign, err := LerPlanilha("excel_br.csv", f, mapa)
	if err != nil || len(itens) != 1 {
		t.Fatalf("itens=%v err=%v", itens, err)
	}
	if itens[0].Descricao != "Pão francês" || itens[0].Valor != 1234.56 || itens[0].Origem != "planilha" {
		t.Errorf("item=%+v", itens[0])
	}
	if len(ign) != 1 || ign[0] != (Ignorado{"excel_br.csv", "linha 3: sem NCM/NBS"}) {
		t.Errorf("ign=%v", ign)
	}
}

func xlsxBytes(t *testing.T) *bytes.Reader {
	x := excelize.NewFile()
	for i, r := range [][]any{{"cod", "desc", "ncm", "valor"}, {"A1", "Arroz", " 1006.30.21 ", 5.5}, {"A2", "Feijão", "0713.33.19", "3.20"}, {"A3", "Carne", 2011000, "12.345"}} {
		c, _ := excelize.CoordinatesToCellName(1, i+1)
		x.SetSheetRow("Sheet1", c, &r)
	}
	b, err := x.WriteToBuffer()
	if err != nil {
		t.Fatal(err)
	}
	return bytes.NewReader(b.Bytes())
}

func TestXLSX(t *testing.T) {
	itens, _, err := LerPlanilha("i.xlsx", xlsxBytes(t), mapa)
	if err != nil || len(itens) != 3 || itens[0].NCM != "10063021" || itens[1].NCM != "07133319" || itens[0].Valor != 5.5 || itens[1].Valor != 3.2 {
		t.Fatalf("itens=%+v err=%v", itens, err)
	}
	if itens[2].NCM != "02011000" || itens[2].Valor != 12.345 { // numérico perde o zero; "12.345" no XLSX é decimal
		t.Errorf("item=%+v", itens[2])
	}
}

func TestPrevia(t *testing.T) {
	var sb strings.Builder
	sb.WriteString("a,b\n")
	for i := 0; i < 9; i++ {
		sb.WriteString("1,2\n")
	}
	p, err := PreviaPlanilha("x.csv", strings.NewReader(sb.String()))
	if err != nil || len(p.Cabecalho) != 2 || len(p.Linhas) != 5 {
		t.Fatalf("%+v %v", p, err)
	}
	px, _ := PreviaPlanilha("i.xlsx", xlsxBytes(t))
	if len(px.Cabecalho) != 4 || len(px.Linhas) != 3 {
		t.Errorf("%+v", px)
	}
}

func TestLinhaSemCodigo(t *testing.T) {
	itens, ign, _ := LerPlanilha("x.csv", strings.NewReader("d,n\nA,\nB,123\n C ,123\n"), Mapeamento{Descricao: 0, NCM: 1, Codigo: -1, NBS: -1, Valor: -1})
	if len(itens) != 2 || len(ign) != 1 || ign[0].Motivo != "linha 2: sem NCM/NBS" {
		t.Errorf("%v %v", itens, ign)
	}
	if itens[0].Codigo != "B" || itens[1].Codigo != "C" { // sem código: descrição vira o código
		t.Errorf("%+v", itens)
	}
}

func TestCodigoFiscal(t *testing.T) {
	for _, c := range []struct {
		in   string
		tam  int
		want string
	}{{"1006.30.21", 8, "10063021"}, {"2011000", 8, "02011000"}, {"201100", 8, "00201100"}, {"20110", 8, "20110"},
		{"1.0101-00 ", 9, "001010100"}, {"ABC.1", 8, "ABC1"}} {
		if got := codigoFiscal(c.in, c.tam); got != c.want {
			t.Errorf("%q -> %q, quer %q", c.in, got, c.want)
		}
	}
}

func TestValores(t *testing.T) {
	for in, want := range map[string]float64{"R$ 1.234,56": 1234.56, "1.234": 1234, "1.234.567": 1234567, "3.20": 3.2, "1234.5": 1234.5, "": 0} {
		if v, ok := parseValor(in, false); !ok || v != want {
			t.Errorf("%q -> %v %v", in, v, ok)
		}
	}
	if v, _ := parseValor("12.345", true); v != 12.345 {
		t.Errorf("xlsx 12.345 -> %v", v)
	}
	if v, _ := parseValor("1.234,5", true); v != 1234.5 { // texto BR numa célula XLSX ainda funciona
		t.Errorf("xlsx 1.234,5 -> %v", v)
	}
	itens, ign, _ := LerPlanilha("x.csv", strings.NewReader("d,n,v\nA,1,abc\nB,2,-\n , , \nC,3,\n"), Mapeamento{Descricao: 0, NCM: 1, Valor: 2, Codigo: -1, NBS: -1})
	if len(itens) != 1 || len(ign) != 2 || ign[0].Motivo != "linha 2: valor inválido" {
		t.Errorf("%v %v", itens, ign)
	}
}

func TestXLSXFormatoNumerico(t *testing.T) {
	x := excelize.NewFile()
	x.SetSheetRow("Sheet1", "A1", &[]any{"d", "n", "v"})
	x.SetSheetRow("Sheet1", "A2", &[]any{"A", "123", 1234.56})
	fmtID := 4 // #,##0.00
	st, _ := x.NewStyle(&excelize.Style{NumFmt: fmtID})
	x.SetCellStyle("Sheet1", "C2", "C2", st)
	b, _ := x.WriteToBuffer()
	itens, _, err := LerPlanilha("i.xlsx", bytes.NewReader(b.Bytes()), Mapeamento{Descricao: 0, NCM: 1, Valor: 2, Codigo: -1, NBS: -1})
	if err != nil || len(itens) != 1 || itens[0].Valor != 1234.56 {
		t.Fatalf("%v %v", itens, err)
	}
}
