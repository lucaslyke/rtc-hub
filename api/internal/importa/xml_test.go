package importa

import (
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
	want := map[string]string{"nfe_entrada.xml": "emitente diferente do cliente", "evento_cancelamento.xml": "não é nota fiscal", "corrompido.xml": "XML inválido"}
	if len(got) != 3 {
		t.Fatalf("%v", got)
	}
	for k, v := range want {
		if got[k] != v {
			t.Errorf("%s: %q", k, got[k])
		}
	}
}
