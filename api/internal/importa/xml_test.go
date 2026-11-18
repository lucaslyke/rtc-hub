package importa

import (
	"archive/zip"
	"bytes"
	"os"
	"testing"
)

const cnpj = "11222333000181"

func ler(t *testing.T, f string) Nota {
	t.Helper()
	b, err := os.Open("testdata/" + f)
	if err != nil {
		t.Fatal(err)
	}
	defer b.Close()
	n, err := LerNota(f, b)
	if err != nil {
		t.Fatal(err)
	}
	return n
}

func TestNFeSaida(t *testing.T) {
	n := ler(t, "nfe_saida.xml")
	if len(n.Itens) != 2 || n.Itens[0].NCM != "10063021" || n.Itens[0].Valor != 100.5 || len(n.Chave) != 44 || n.EmitenteCNPJ != cnpj || n.Itens[0].Origem != "nfe" {
		t.Fatalf("%+v", n)
	}
}

func TestNFCe(t *testing.T) {
	n := ler(t, "nfce.xml")
	if len(n.Itens) != 2 || n.Itens[1].Valor != 40 || len(n.Chave) != 44 {
		t.Fatalf("%+v", n)
	}
}

func TestNFSeComESemNBS(t *testing.T) {
	c, s := ler(t, "nfse_com_nbs.xml").Itens[0], ler(t, "nfse_sem_nbs.xml").Itens[0]
	if c.NBS != "115022000" || s.NBS != "" || s.CTribNac == "" || s.Codigo != s.CTribNac || c.Valor != 1500 || c.Origem != "nfse" {
		t.Fatalf("%+v %+v", c, s)
	}
}

func TestZipComArquivosRuins(t *testing.T) {
	f, err := os.Open("testdata/lote.zip")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	notas, ign := LerArquivos("lote.zip", f, cnpj)
	if len(notas) != 4 {
		t.Fatalf("notas=%d", len(notas))
	}
	got := map[string]string{}
	for _, i := range ign {
		got[i.Arquivo] = i.Motivo
	}
	want := map[string]string{"nfe_entrada.xml": "nota emitida por 99888777000166, não por este cliente (nota de entrada?)", "evento_cancelamento.xml": "não é nota fiscal", "corrompido.xml": "XML inválido"}
	if len(got) != 3 {
		t.Fatalf("%v", got)
	}
	for k, v := range want {
		if got[k] != v {
			t.Errorf("%s: %q", k, got[k])
		}
	}
}

func TestZipLimite50MB(t *testing.T) {
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	for _, n := range []string{"a.xml", "b.xml"} {
		w, _ := zw.Create(n)
		w.Write(make([]byte, 30<<20)) // zeros comprimem: o ZIP é pequeno, o conteúdo não
	}
	zw.Close()
	_, ign := LerArquivos("grande.zip", &buf, cnpj)
	last := ign[len(ign)-1]
	if last != (Ignorado{"grande.zip", "demais arquivos do ZIP não lidos (limite de 50 MB)"}) {
		t.Errorf("%v", ign)
	}
}
