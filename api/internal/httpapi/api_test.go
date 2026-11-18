package httpapi

import (
	"bytes"
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"rtchub/internal/motores"
	"rtchub/internal/store"
	"rtchub/internal/tabelas"
)

const (
	testdata     = "../importa/testdata/"
	emissorTeste = "http://supabase.teste/auth/v1"
)

type ambiente struct {
	t     *testing.T
	srv   *httptest.Server
	st    *store.Store
	chave *ecdsa.PrivateKey
	user  string
}

// novoAmbiente sobe a api com chave JWT do teste, Supabase local e um motor falso.
func novoAmbiente(t *testing.T, motor http.HandlerFunc) *ambiente {
	url := os.Getenv("SUPABASE_DB_URL")
	if url == "" {
		t.Skip("SUPABASE_DB_URL não definida")
	}
	ctx := context.Background()
	st, err := store.Conectar(ctx, url)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(st.Close)
	tab, err := tabelas.Abrir("../../../.data/calculadora-pro.db")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { tab.Close() })
	if motor == nil {
		motor = func(w http.ResponseWriter, r *http.Request) { http.Error(w, "x", 500) }
	}
	fm := httptest.NewServer(motor)
	t.Cleanup(fm.Close)

	chave, _ := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	s := &Servidor{
		Store: st, Tabelas: tab, Motores: motores.Novo(fm.URL, fm.URL, nil),
		Keyfunc: func(*jwt.Token) (any, error) { return &chave.PublicKey, nil }, Issuer: emissorTeste,
	}
	srv := httptest.NewServer(s.Rotas())
	t.Cleanup(srv.Close)

	conn, err := pgx.Connect(ctx, url)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { conn.Close(ctx) })
	user := uuid.NewString()
	if _, err := conn.Exec(ctx, `insert into auth.users (id, email, aud, role, instance_id)
		values ($1, $2, 'authenticated', 'authenticated', '00000000-0000-0000-0000-000000000000')`, user, user+"@teste.local"); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { conn.Exec(ctx, `delete from auth.users where id=$1`, user) })
	return &ambiente{t: t, srv: srv, st: st, chave: chave, user: user}
}

func (a *ambiente) token(aud string) string { return a.tokenDe(aud, emissorTeste) }

func (a *ambiente) tokenDe(aud, iss string) string {
	tk := jwt.NewWithClaims(jwt.SigningMethodES256, jwt.MapClaims{
		"sub": a.user, "aud": aud, "iss": iss, "role": "authenticated", "exp": time.Now().Add(time.Hour).Unix(),
	})
	s, err := tk.SignedString(a.chave)
	if err != nil {
		a.t.Fatal(err)
	}
	return s
}

// req faz a chamada autenticada e decodifica a resposta em out (se não for nil).
func (a *ambiente) req(metodo, caminho, tipo string, corpo io.Reader, out any) int {
	a.t.Helper()
	r, _ := http.NewRequest(metodo, a.srv.URL+caminho, corpo)
	r.Header.Set("Authorization", "Bearer "+a.token("authenticated"))
	if tipo != "" {
		r.Header.Set("Content-Type", tipo)
	}
	resp, err := http.DefaultClient.Do(r)
	if err != nil {
		a.t.Fatal(err)
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	if out != nil && resp.StatusCode < 300 {
		if err := json.Unmarshal(b, out); err != nil {
			a.t.Fatalf("%s %s: %v: %s", metodo, caminho, err, b)
		}
	}
	if resp.StatusCode >= 300 && out != nil {
		a.t.Logf("%s %s → %d %s", metodo, caminho, resp.StatusCode, b)
	}
	return resp.StatusCode
}

func (a *ambiente) json(metodo, caminho string, in, out any) int {
	b, _ := json.Marshal(in)
	return a.req(metodo, caminho, "application/json", bytes.NewReader(b), out)
}

func (a *ambiente) cliente(regime string) store.Cliente {
	var c store.Cliente
	if st := a.json("POST", "/clientes", map[string]any{
		"razaoSocial": "Padaria", "cnpj": "11.222.333/0001-81", "regime": regime, "uf": "SP", "municipioIbge": 3550308,
	}, &c); st != 201 {
		a.t.Fatalf("criar cliente: %d", st)
	}
	return c
}

func (a *ambiente) importar(clienteID string, arquivos ...string) resultadoImportacao {
	return a.importarCom(clienteID, "", arquivos...)
}

func (a *ambiente) importarCom(clienteID, mapeamento string, arquivos ...string) (res resultadoImportacao) {
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	if mapeamento != "" {
		mw.WriteField("mapeamento", mapeamento)
	}
	for _, nome := range arquivos {
		caminho := nome
		if !filepath.IsAbs(nome) {
			caminho = testdata + nome
		}
		b, err := os.ReadFile(caminho)
		if err != nil {
			a.t.Fatal(err)
		}
		fw, _ := mw.CreateFormFile("arquivos[]", filepath.Base(nome))
		fw.Write(b)
	}
	mw.Close()
	if st := a.req("POST", "/clientes/"+clienteID+"/importar", mw.FormDataContentType(), &buf, &res); st != 200 {
		a.t.Fatalf("importar: %d", st)
	}
	return
}

type listaItens struct {
	Itens    []store.Item   `json:"itens"`
	Contagem map[string]int `json:"contagem"`
}

func (a *ambiente) itens(clienteID, situacao string) (l listaItens) {
	if st := a.req("GET", "/clientes/"+clienteID+"/itens?situacao="+situacao, "", nil, &l); st != 200 {
		a.t.Fatalf("itens: %d", st)
	}
	return
}

func porCodigo(its []store.Item, codigo string) store.Item {
	for _, i := range its {
		if i.Codigo == codigo {
			return i
		}
	}
	return store.Item{}
}

func TestSemTokenDa401(t *testing.T) {
	a := novoAmbiente(t, nil)
	for _, h := range []string{"", "Bearer lixo", "Bearer " + a.token("outro"), "Bearer " + a.tokenDe("authenticated", "http://outro/auth/v1")} {
		r, _ := http.NewRequest("GET", a.srv.URL+"/clientes", nil)
		if h != "" {
			r.Header.Set("Authorization", h)
		}
		resp, err := http.DefaultClient.Do(r)
		if err != nil {
			t.Fatal(err)
		}
		resp.Body.Close()
		if resp.StatusCode != 401 {
			t.Errorf("%q: status %d, quer 401", h, resp.StatusCode)
		}
	}
	resp, err := http.Get(a.srv.URL + "/health/live")
	if err != nil || resp.StatusCode != 200 {
		t.Errorf("health/live: %v %v", resp, err)
	}
}

func TestImportarNFeSugere(t *testing.T) {
	a := novoAmbiente(t, nil)
	c := a.cliente("regular")
	res := a.importar(c.ID, "nfe_saida.xml")
	if res.ItensNovos != 2 {
		t.Fatalf("itensNovos = %d, quer 2 (%+v)", res.ItensNovos, res)
	}
	l := a.itens(c.ID, "")
	arroz := porCodigo(l.Itens, "A1")
	if arroz.Situacao == "" {
		t.Errorf("arroz sem sugestão: %+v", arroz)
	}
	soma := 0
	for _, n := range l.Contagem {
		soma += n
	}
	if soma != 2 {
		t.Errorf("contagem: %v", l.Contagem)
	}
	f := a.itens(c.ID, arroz.Situacao)
	if len(f.Itens) == 0 || f.Itens[0].Situacao != arroz.Situacao {
		t.Errorf("filtro por situação: %+v", f.Itens)
	}
}

func TestImportarNotaRepetidaNaoDuplica(t *testing.T) {
	a := novoAmbiente(t, nil)
	c := a.cliente("regular")
	a.importar(c.ID, "nfe_saida.xml")
	res := a.importar(c.ID, "nfe_saida.xml")
	if res.NotasRepetidas != 1 || res.ItensAtualizados != 0 || res.ItensNovos != 0 {
		t.Errorf("2ª importação: %+v", res)
	}
	for _, i := range a.itens(c.ID, "").Itens {
		if i.Ocorrencias != 1 {
			t.Errorf("%s: ocorrencias = %d", i.Codigo, i.Ocorrencias)
		}
	}
}

func TestImportarNFSeSemNBSOfereceOpcoes(t *testing.T) {
	a := novoAmbiente(t, nil)
	c := a.cliente("regular")
	res := a.importar(c.ID, "nfse_sem_nbs.xml", "nfe_entrada.xml")
	if res.ItensNovos != 1 || len(res.Ignorados) != 1 || res.Ignorados[0].Arquivo != "nfe_entrada.xml" {
		t.Fatalf("importação: %+v", res)
	}
	it := a.itens(c.ID, "").Itens[0]
	if it.Situacao != "revisar" || !strings.Contains(string(it.Opcoes), "cClassTrib") {
		t.Errorf("NFS-e sem NBS: %+v %s", it, it.Opcoes)
	}
}

func seed(t *testing.T, a *ambiente, clienteID string, its ...store.ItemEntrada) map[string]string {
	ids := map[string]string{}
	err := a.st.ComoUsuario(context.Background(), a.user, func(tx pgx.Tx) error {
		if _, _, err := store.UpsertItens(context.Background(), tx, clienteID, its); err != nil {
			return err
		}
		l, err := store.ListarItens(context.Background(), tx, clienteID)
		for _, i := range l {
			ids[i.Codigo] = i.ID
		}
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	return ids
}

func TestConfirmarEmLote(t *testing.T) {
	a := novoAmbiente(t, nil)
	c := a.cliente("regular")
	ids := seed(t, a, c.ID,
		store.ItemEntrada{Codigo: "P", NCM: "10063021", Situacao: "provavel", CST: "200", CClassTrib: "200003"},
		store.ItemEntrada{Codigo: "R", NCM: "22030000", Situacao: "revisar"})
	var out struct{ Confirmados int64 }
	if st := a.json("POST", "/clientes/"+c.ID+"/itens/confirmar", map[string]any{"ids": []string{ids["P"], ids["R"]}}, &out); st != 200 || out.Confirmados != 1 {
		t.Fatalf("confirmar: %d %+v", st, out)
	}
	// id de outro cliente do mesmo usuário não muda pela rota deste cliente
	var outro store.Cliente
	a.json("POST", "/clientes", map[string]any{"razaoSocial": "Outro", "cnpj": "99888777000166", "regime": "regular", "uf": "SP", "municipioIbge": 3550308}, &outro)
	idsOutro := seed(t, a, outro.ID, store.ItemEntrada{Codigo: "P", NCM: "10063021", Situacao: "provavel", CST: "200", CClassTrib: "200003"})
	if st := a.json("POST", "/clientes/"+c.ID+"/itens/confirmar", map[string]any{"ids": []string{idsOutro["P"]}}, &out); st != 200 || out.Confirmados != 0 {
		t.Errorf("confirmou item de outro cliente: %d %+v", st, out)
	}
	its := a.itens(c.ID, "").Itens
	if p := porCodigo(its, "P"); p.Situacao != "ok" || p.Origem != "confirmado" {
		t.Errorf("P: %+v", p)
	}
	if r := porCodigo(its, "R"); r.Situacao != "revisar" || r.Origem != "sugerido" {
		t.Errorf("R: %+v", r)
	}
}

func TestPatchViraManual(t *testing.T) {
	a := novoAmbiente(t, nil)
	c := a.cliente("regular")
	ids := seed(t, a, c.ID, store.ItemEntrada{Codigo: "R", NCM: "22030000", Situacao: "revisar", Condicao: "x"})
	var it store.Item
	if st := a.json("PATCH", "/itens/"+ids["R"], map[string]any{"cst": "000", "cClassTrib": "000001", "isCst": "000", "isCClassTrib": "000001"}, &it); st != 200 {
		t.Fatalf("patch: %d", st)
	}
	if it.Origem != "manual" || it.Situacao != "ok" || it.CClassTrib != "000001" || it.ISCClassTrib != "000001" || it.Condicao != "x" {
		t.Errorf("patch: %+v", it)
	}
	// "não incide (revenda)": IS vazio, condição do IS some
	if st := a.json("PATCH", "/itens/"+ids["R"], map[string]any{"cst": "000", "cClassTrib": "000001"}, &it); st != 200 || it.ISCClassTrib != "" || it.Condicao != "" {
		t.Errorf("revenda: %d %+v", st, it)
	}
	if st := a.json("PATCH", "/itens/"+ids["R"], map[string]any{"cst": "0", "cClassTrib": "000001"}, nil); st != 400 {
		t.Errorf("CST inválido: %d, quer 400", st)
	}
	if st := a.json("PATCH", "/itens/"+uuid.NewString(), map[string]any{"cst": "000", "cClassTrib": "000001"}, nil); st != 404 {
		t.Errorf("item inexistente: %d, quer 404", st)
	}
}

func TestCalcularSoItensProntos(t *testing.T) {
	var recebidos atomic.Int64
	a := novoAmbiente(t, func(w http.ResponseWriter, r *http.Request) {
		var req struct{ Itens []any }
		json.NewDecoder(r.Body).Decode(&req)
		recebidos.Add(int64(len(req.Itens)))
		fmt.Fprint(w, `{"objetos":[{"nObj":1,"estadoItem":"CALCULO_SIMULADO","tribCalc":{"IBSCBS":{"gIBSCBS":{"vIBS":"1.00","gCBS":{"vCBS":"2.00"}}}}}]}`)
	})
	c := a.cliente("regular")
	ids := seed(t, a, c.ID,
		store.ItemEntrada{Codigo: "OK", NCM: "10063021", Valor: 100, Situacao: "ok", CST: "000", CClassTrib: "000001"},
		store.ItemEntrada{Codigo: "R", NCM: "22030000", Situacao: "revisar"})
	var out struct {
		Resultados []motores.Resultado
		Pendentes  []store.Item
	}
	if st := a.json("POST", "/clientes/"+c.ID+"/calcular", map[string]any{"ano": 2027}, &out); st != 200 {
		t.Fatalf("calcular: %d", st)
	}
	if recebidos.Load() != 1 || len(out.Resultados) != 1 || out.Resultados[0].ItemID != ids["OK"] || out.Resultados[0].VCBS != "2.00" {
		t.Errorf("resultados: %d %+v", recebidos.Load(), out.Resultados)
	}
	if len(out.Pendentes) != 1 || out.Pendentes[0].ID != ids["R"] {
		t.Errorf("pendentes: %+v", out.Pendentes)
	}
	if ok := porCodigo(a.itens(c.ID, "").Itens, "OK"); !strings.Contains(string(ok.Calculos), `"2027"`) {
		t.Errorf("cálculo não salvo: %s", ok.Calculos)
	}
}

func TestCalcularMotorForaDa502(t *testing.T) {
	a := novoAmbiente(t, nil) // motor falso responde 500
	c := a.cliente("regular")
	seed(t, a, c.ID, store.ItemEntrada{Codigo: "OK", NCM: "10063021", Valor: 100, Situacao: "ok", CST: "000", CClassTrib: "000001"})
	r, _ := http.NewRequest("POST", a.srv.URL+"/clientes/"+c.ID+"/calcular", strings.NewReader(`{"ano":2027}`))
	r.Header.Set("Authorization", "Bearer "+a.token("authenticated"))
	resp, err := http.DefaultClient.Do(r)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != 502 || !strings.Contains(string(b), "Os motores de cálculo da Receita não responderam") {
		t.Errorf("%d %s", resp.StatusCode, b)
	}
}

func TestClientesCRUDEAliquotas(t *testing.T) {
	a := novoAmbiente(t, nil)
	c := a.cliente("regular")
	c.RazaoSocial = "Padaria Nova"
	var got store.Cliente
	if st := a.json("PUT", "/clientes/"+c.ID, c, &got); st != 200 || got.RazaoSocial != "Padaria Nova" {
		t.Errorf("put: %d %+v", st, got)
	}
	if st := a.json("POST", "/clientes", map[string]any{"razaoSocial": "X", "cnpj": "123", "regime": "regular", "uf": "SP", "municipioIbge": 1}, nil); st != 400 {
		t.Errorf("cnpj inválido: %d", st)
	}
	if st := a.req("DELETE", "/clientes/"+c.ID, "", nil, nil); st != 204 {
		t.Errorf("delete: %d", st)
	}
	if st := a.req("GET", "/clientes/"+c.ID, "", nil, nil); st != 404 {
		t.Errorf("get após delete: %d", st)
	}
	var al store.AliquotasEst
	if st := a.json("PUT", "/aliquotas", map[string]any{"cbs": 9, "ibsUf": 10, "ibsMun": 3, "isAdValorem": 0}, nil); st != 204 {
		t.Errorf("put aliquotas: %d", st)
	}
	if st := a.req("GET", "/aliquotas", "", nil, &al); st != 200 || al.CBS != 9 || al.IBSUF != 10 {
		t.Errorf("get aliquotas: %d %+v", st, al)
	}
}

func TestImportarPlanilha(t *testing.T) {
	a := novoAmbiente(t, nil)
	c := a.cliente("regular")
	res := a.importarCom(c.ID, `{"codigo":0,"descricao":1,"ncm":2,"valor":3}`, "excel_br.csv")
	if res.ItensNovos != 1 || len(res.Ignorados) != 1 {
		t.Fatalf("planilha: %+v", res)
	}
	it := a.itens(c.ID, "").Itens[0]
	if it.Descricao != "Pão francês" || it.NBS != "" || it.UltimoValor != 1234.56 || it.Situacao == "" {
		t.Errorf("item: %+v", it)
	}
}

type opcoesLidas struct {
	IBSCBS []struct {
		CST        string `json:"cst"`
		CClassTrib string `json:"cClassTrib"`
		Descricao  string `json:"descricao"`
	} `json:"ibsCbs"`
	IS []any `json:"is"`
}

func lerOpcoes(t *testing.T, it store.Item) opcoesLidas {
	var o opcoesLidas
	if err := json.Unmarshal(it.Opcoes, &o); err != nil || o.IBSCBS == nil || o.IS == nil {
		t.Fatalf("opcoes fora do formato {ibsCbs:[],is:[]}: %s %v", it.Opcoes, err)
	}
	return o
}

func TestServicoSemNBSEscolheNBSECalcula(t *testing.T) {
	var corpo atomic.Value
	a := novoAmbiente(t, func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		corpo.Store(string(b))
		fmt.Fprint(w, `{"objetos":[{"nObj":1,"estadoItem":"CALCULO_SIMULADO","tribCalc":{}}]}`)
	})
	c := a.cliente("regular")
	a.importar(c.ID, "nfse_sem_nbs.xml")
	it := a.itens(c.ID, "").Itens[0]
	o := lerOpcoes(t, it)
	if len(o.IBSCBS) == 0 || !strings.HasPrefix(o.IBSCBS[0].Descricao, "NBS ") || !strings.Contains(o.IBSCBS[0].Descricao, " — ") {
		t.Fatalf("opções sem a NBS na descrição: %s", it.Opcoes)
	}
	nbs := strings.TrimPrefix(strings.SplitN(o.IBSCBS[0].Descricao, " — ", 2)[0], "NBS ")
	var got store.Item
	if st := a.json("PATCH", "/itens/"+it.ID, map[string]any{"cst": o.IBSCBS[0].CST, "cClassTrib": o.IBSCBS[0].CClassTrib, "nbs": nbs}, &got); st != 200 || got.NBS != tabelas.NormalizaCodigo(nbs) {
		t.Fatalf("patch nbs: %d %+v", st, got)
	}
	if st := a.json("POST", "/clientes/"+c.ID+"/calcular", map[string]any{"ano": 2027}, nil); st != 200 {
		t.Fatalf("calcular: %d", st)
	}
	b, _ := corpo.Load().(string)
	if !strings.Contains(b, `"nbs":"`+got.NBS+`"`) || strings.Contains(b, `"ncm"`) {
		t.Errorf("motor recebeu: %s", b)
	}
	// num item que já tem NCM a nbs é ignorada
	ids := seed(t, a, c.ID, store.ItemEntrada{Codigo: "N", NCM: "10063021", Situacao: "revisar"})
	if st := a.json("PATCH", "/itens/"+ids["N"], map[string]any{"cst": "000", "cClassTrib": "000001", "nbs": nbs}, &got); st != 200 || got.NBS != "" {
		t.Errorf("nbs gravada em item com NCM: %d %+v", st, got)
	}
}

func TestServicoSemCandidatosOpcoesVazias(t *testing.T) {
	a := novoAmbiente(t, nil)
	c := a.cliente("regular")
	b, _ := os.ReadFile(testdata + "nfse_sem_nbs.xml")
	arq := filepath.Join(t.TempDir(), "sem_candidato.xml")
	os.WriteFile(arq, bytes.Replace(b, []byte("<cTribNac>010101<"), []byte("<cTribNac>999999<"), 1), 0o600)
	if res := a.importar(c.ID, arq); res.ItensNovos != 1 {
		t.Fatalf("importação: %+v", res)
	}
	it := a.itens(c.ID, "").Itens[0]
	if o := lerOpcoes(t, it); it.Situacao != "invalido" || len(o.IBSCBS) != 0 || it.Condicao != "Serviço sem NBS e sem correspondência na tabela da Receita" {
		t.Errorf("sem candidatos: %+v %s", it, it.Opcoes)
	}
}

func TestOpcoesNFeFormatoUnico(t *testing.T) {
	a := novoAmbiente(t, nil)
	c := a.cliente("regular")
	a.importar(c.ID, "nfe_saida.xml")
	for _, it := range a.itens(c.ID, "").Itens {
		lerOpcoes(t, it)
	}
}

func TestListarClientesComContagem(t *testing.T) {
	a := novoAmbiente(t, nil)
	c := a.cliente("regular")
	seed(t, a, c.ID, store.ItemEntrada{Codigo: "P", NCM: "10063021", Situacao: "provavel"},
		store.ItemEntrada{Codigo: "R", NCM: "22030000", Situacao: "revisar"})
	var cs []struct {
		ID       string         `json:"id"`
		Contagem map[string]int `json:"contagem"`
	}
	if st := a.req("GET", "/clientes", "", nil, &cs); st != 200 || len(cs) != 1 {
		t.Fatalf("listar: %d %+v", st, cs)
	}
	want := map[string]int{"ok": 0, "provavel": 1, "revisar": 1, "invalido": 0}
	if fmt.Sprint(cs[0].Contagem) != fmt.Sprint(want) {
		t.Errorf("contagem = %v, quer %v", cs[0].Contagem, want)
	}
}

func TestImportarAcimaDe50MBDa413(t *testing.T) {
	a := novoAmbiente(t, nil)
	c := a.cliente("regular")
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	fw, _ := mw.CreateFormFile("arquivos[]", "grande.xml")
	fw.Write(bytes.Repeat([]byte("x"), 50<<20+1))
	mw.Close()
	if st := a.req("POST", "/clientes/"+c.ID+"/importar", mw.FormDataContentType(), &buf, nil); st != 413 {
		t.Errorf("status %d, quer 413", st)
	}
}

func TestExportarXLSX(t *testing.T) {
	a := novoAmbiente(t, nil)
	c := a.cliente("regular")
	get := func(q string) (*http.Response, string) {
		r, _ := http.NewRequest("GET", a.srv.URL+"/clientes/"+c.ID+"/exportar.xlsx"+q, nil)
		r.Header.Set("Authorization", "Bearer "+a.token("authenticated"))
		resp, err := http.DefaultClient.Do(r)
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		b, _ := io.ReadAll(resp.Body)
		return resp, string(b)
	}
	for _, q := range []string{"", "?ano=abc", "?ano=2025"} {
		if resp, b := get(q); resp.StatusCode != 400 || !strings.Contains(b, "ano") {
			t.Errorf("%q: %d %s", q, resp.StatusCode, b)
		}
	}
	resp, b := get("?ano=2027")
	if resp.StatusCode != 200 || resp.Header.Get("Content-Type") != "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet" ||
		resp.Header.Get("Content-Disposition") != `attachment; filename="cadastro-`+c.CNPJ+`-2027.xlsx"` || !strings.HasPrefix(b, "PK") {
		t.Errorf("200: %d %v", resp.StatusCode, resp.Header)
	}
}

func TestPatchValidaClassificacao(t *testing.T) {
	a := novoAmbiente(t, nil)
	c := a.cliente("regular")
	ids := seed(t, a, c.ID, store.ItemEntrada{Codigo: "C", NCM: "84713012", Situacao: "ok", CST: "000", CClassTrib: "000001"},
		store.ItemEntrada{Codigo: "M", NCM: "30049045", Situacao: "revisar"})
	for _, c := range []struct {
		item string
		in   map[string]any
		msg  string
	}{
		{"C", map[string]any{"cst": "000", "cClassTrib": "999999"}, "Classificação inexistente ou fora de vigência"},
		{"C", map[string]any{"cst": "000", "cClassTrib": "200003"}, "é do CST 200"},
		{"C", map[string]any{"cst": "200", "cClassTrib": "200003"}, "Essa classificação não vale para o NCM 84713012 na tabela da Receita."},
		{"C", map[string]any{"cst": "000", "cClassTrib": "000001", "isCst": "000", "isCClassTrib": "999999"}, "Classificação inexistente"},
		{"C", map[string]any{"cst": "000", "cClassTrib": "000001", "isCst": "000", "isCClassTrib": "000001"}, "não vale para o NCM"}, // computador não tem IS
	} {
		r, _ := http.NewRequest("PATCH", a.srv.URL+"/itens/"+ids[c.item], strings.NewReader(mustJSON(c.in)))
		r.Header.Set("Authorization", "Bearer "+a.token("authenticated"))
		resp, err := http.DefaultClient.Do(r)
		if err != nil {
			t.Fatal(err)
		}
		b, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		if resp.StatusCode != 400 || !strings.Contains(string(b), c.msg) {
			t.Errorf("%v: %d %s", c.in, resp.StatusCode, b)
		}
	}
	var it store.Item
	if st := a.json("PATCH", "/itens/"+ids["M"], map[string]any{"cst": "200", "cClassTrib": "200009"}, &it); st != 200 || it.CClassTrib != "200009" {
		t.Errorf("medicamento 200009: %d %+v", st, it)
	}
}

func mustJSON(v any) string { b, _ := json.Marshal(v); return string(b) }

func TestCalcularItemSemValorUsaBase100(t *testing.T) {
	var corpo atomic.Value
	a := novoAmbiente(t, func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		corpo.Store(string(b))
		fmt.Fprint(w, `{"objetos":[{"nObj":1,"estadoItem":"CALCULO_SIMULADO","tribCalc":{}}]}`)
	})
	c := a.cliente("regular")
	seed(t, a, c.ID, store.ItemEntrada{Codigo: "Z", NCM: "84713012", Situacao: "ok", CST: "000", CClassTrib: "000001"})
	var out struct{ Resultados []motores.Resultado }
	if st := a.json("POST", "/clientes/"+c.ID+"/calcular", map[string]any{"ano": 2027}, &out); st != 200 {
		t.Fatalf("calcular: %d", st)
	}
	if b, _ := corpo.Load().(string); !strings.Contains(b, `"baseCalculo":100`) {
		t.Errorf("motor recebeu: %s", b)
	}
	if out.Resultados[0].Mensagem != "Base de referência de R$ 100,00 (item sem valor)" {
		t.Errorf("mensagem: %q", out.Resultados[0].Mensagem)
	}
}

func TestClienteSimplesValidaFaixas(t *testing.T) {
	a := novoAmbiente(t, nil)
	base := map[string]any{"razaoSocial": "S", "cnpj": "11222333000181", "regime": "simples", "uf": "SP", "municipioIbge": 3550308, "cAtivSn": "01", "pIbsSn": 1.0, "pCbsSn": 2.0}
	for k, v := range map[string]any{"cAtivSn": "16", "pIbsSn": 101.0, "pCbsSn": -1.0} {
		in := map[string]any{}
		for x, y := range base {
			in[x] = y
		}
		in[k] = v
		if st := a.json("POST", "/clientes", in, nil); st != 400 {
			t.Errorf("%s=%v: %d, quer 400", k, v, st)
		}
	}
	if st := a.json("POST", "/clientes", base, nil); st != 201 {
		t.Errorf("válido: %d", st)
	}
}
