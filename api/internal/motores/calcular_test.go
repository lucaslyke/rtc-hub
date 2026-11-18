package motores

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
)

// srv devolve a fixture e captura o corpo enviado; calls conta requests.
func srv(t *testing.T, fixture string) (*httptest.Server, *string, *int) {
	t.Helper()
	b, err := os.ReadFile("testdata/" + fixture)
	if err != nil {
		t.Fatal(err)
	}
	var body string
	var calls int
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		x, _ := io.ReadAll(r.Body)
		body = string(x)
		w.Header().Set("Content-Type", "application/json")
		w.Write(b)
	}))
	t.Cleanup(s.Close)
	return s, &body, &calls
}

var regCli = ClienteCalc{Regime: "regular", UF: "RS", MunicipioIBGE: 4314902}
var al = Aliquotas{CBS: 8.8, IBSUF: 0.05, IBSMun: 0.05, ISAdValorem: 10}
var itens4 = []ItemCalc{
	{ID: "a", NCM: "10063021", CST: "200", CClassTrib: "200003", Valor: 100},
	{ID: "b", NCM: "22030000", CST: "000", CClassTrib: "000001", ISCST: "000", ISCClassTrib: "000001", Valor: 150},
	{ID: "c", NBS: "122011200", CST: "200", CClassTrib: "200028", Valor: 1200},
	{ID: "d", NCM: "02013000", CST: "200", CClassTrib: "200035", Valor: 80},
}

func TestRegular2026SemAliquotas(t *testing.T) {
	s, body, _ := srv(t, "regular-resposta.json")
	r, err := Novo(s.URL, "", nil).Calcular(context.Background(), 2026, regCli, al, itens4)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(*body, "aliquotasNominais") {
		t.Fatalf("2026 não deve enviar aliquotasNominais: %s", *body)
	}
	if r[0].Simulacao {
		t.Fatal("Simulacao deveria ser false")
	}
}

func TestRegular2027ComAliquotas(t *testing.T) {
	s, body, _ := srv(t, "regular-resposta.json")
	r, err := Novo(s.URL, "", nil).Calcular(context.Background(), 2027, regCli, al, itens4)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(*body, `"cbs":8.8`) || !strings.Contains(*body, "impostoSeletivo") {
		t.Fatalf("corpo sem cbs/IS: %s", *body)
	}
	if !r[1].Simulacao || r[1].VCBS != "14.52" || r[1].VIS != "15.00" || r[1].VIBS != "0.16" {
		t.Fatalf("resultado inesperado: %+v", r[1])
	}
	if r[0].PRedCBS != "100.00" || r[1].PRedCBS != "0.00" || r[0].ItemID != "a" {
		t.Fatalf("PRed/ID inesperado: %+v %+v", r[0], r[1])
	}
}

func TestSimples2026NaoChamaMotor(t *testing.T) {
	s, _, calls := srv(t, "simples-resposta.json")
	c := ClienteCalc{Regime: "simples", CAtivSN: "01", PIBSSN: "1.25", PCBSSN: "3.00"}
	r, err := Novo("", s.URL, nil).Calcular(context.Background(), 2026, c, al, []ItemCalc{{ID: "a", NCM: "10063021", CClassTrib: "000001", Valor: 100}})
	if err != nil {
		t.Fatal(err)
	}
	if *calls != 0 || r[0].Estado != "NAO_SUPORTADO" || !strings.Contains(r[0].Mensagem, "a partir de 2027") {
		t.Fatalf("calls=%d %+v", *calls, r[0])
	}
}

func TestSimplesServicoNaoCalcula(t *testing.T) {
	s, _, calls := srv(t, "simples-resposta.json")
	c := ClienteCalc{Regime: "simples", CAtivSN: "01", PIBSSN: "1.25", PCBSSN: "3.00"}
	r, _ := Novo("", s.URL, nil).Calcular(context.Background(), 2027, c, al, []ItemCalc{{ID: "a", NBS: "122011200", CClassTrib: "000001", Valor: 100}})
	if *calls != 0 || r[0].Estado != "NAO_SUPORTADO" || !strings.Contains(r[0].Mensagem, "ainda não calcula serviços") {
		t.Fatalf("calls=%d %+v", *calls, r[0])
	}
}

func TestSimples2027Calcula(t *testing.T) {
	s, body, _ := srv(t, "simples-resposta.json")
	c := ClienteCalc{Regime: "simples", CAtivSN: "01", PIBSSN: "1.25", PCBSSN: "3.00"}
	r, err := Novo("", s.URL, nil).Calcular(context.Background(), 2027, c, al, []ItemCalc{{ID: "a", NCM: "10063021", CClassTrib: "000001", Valor: 100}})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(*body, `"baseCalculo":"100.00"`) || r[0].VCBS != "3.00" || r[0].VIBS != "1.25" || r[0].VIS != "0.00" || !r[0].Simulacao {
		t.Fatalf("body=%s r=%+v", *body, r[0])
	}
}

func TestErroPorItem(t *testing.T) {
	s, _, _ := srv(t, "regular-resposta.json")
	r, _ := Novo(s.URL, "", nil).Calcular(context.Background(), 2027, regCli, al, itens4)
	if r[3].Estado != "INCONSISTENCIA_ENTRADA" || !strings.HasPrefix(r[3].Mensagem, "NCM não vinculada: NCM de código 02013000") {
		t.Fatalf("%+v", r[3])
	}
}

func TestBrutoSoDoItemEMemoria(t *testing.T) {
	s, _, _ := srv(t, "regular-resposta.json")
	r, _ := Novo(s.URL, "", nil).Calcular(context.Background(), 2027, regCli, al, itens4)
	if b := string(r[1].Bruto); !strings.HasPrefix(b, `{"nObj":2,`) || strings.Contains(b, `"nObj":1,`) {
		t.Fatalf("bruto: %.200s", r[1].Bruto)
	}
	if !strings.Contains(r[1].Memoria, "alíquota de 8.800000%") {
		t.Fatalf("memoria: %q", r[1].Memoria)
	}
}

func TestMensagemSemTituloRepetido(t *testing.T) {
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"objetos":[{"nObj":1,"estadoItem":"ERRO","erros":[{"title":"NCM inválida","detail":"NCM inválida: 123"}]}]}`))
	}))
	defer s.Close()
	r, _ := Novo(s.URL, "", nil).Calcular(context.Background(), 2027, regCli, al, itens4[:1])
	if r[0].Mensagem != "NCM inválida: 123" {
		t.Fatalf("%q", r[0].Mensagem)
	}
}
