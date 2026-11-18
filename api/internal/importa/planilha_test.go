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
	for i, r := range [][]any{{"cod", "desc", "ncm", "valor"}, {"A1", "Arroz", " 1006.30.21 ", 5.5}, {"A2", "Feijão", "0713.33.19", "3.20"}} {
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
	if err != nil || len(itens) != 2 || itens[0].NCM != "1006.30.21" || itens[0].Valor != 5.5 || itens[1].Valor != 3.2 {
		t.Fatalf("itens=%+v err=%v", itens, err)
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
	if len(px.Cabecalho) != 4 || len(px.Linhas) != 2 {
		t.Errorf("%+v", px)
	}
}

func TestLinhaSemCodigo(t *testing.T) {
	itens, ign, _ := LerPlanilha("x.csv", strings.NewReader("d,n\nA,\nB,123\n"), Mapeamento{Descricao: 0, NCM: 1, Codigo: -1, NBS: -1, Valor: -1})
	if len(itens) != 1 || len(ign) != 1 || ign[0].Motivo != "linha 2: sem NCM/NBS" {
		t.Errorf("%v %v", itens, ign)
	}
}
