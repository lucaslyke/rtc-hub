// gerar-semente-demo gera a semente da demo: cria um usuário temporário, cadastra os três clientes,
// importa XMLs fictícios pela api real (a semente reflete o motor de sugestão de verdade), calcula 2027
// e grava as linhas no fim de supabase/migrations/20261118000002_demo.sql, como dados do usuário-modelo.
//
// Pré-requisitos: Supabase local, motores e api rodando (veja o README). Uso, na raiz do repositório:
//
//	SUPABASE_ANON_KEY=... API_URL=http://localhost:8090 go run scripts/gerar-semente-demo.go
//
// Depois: npx supabase db reset (aplica a migration com a semente nova).
package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"maps"
	"math/rand/v2"
	"mime/multipart"
	"net/http"
	"os"
	"os/exec"
	"slices"
	"strings"
	"time"
)

const (
	modelo   = "00000000-0000-0000-0000-00000000de00"
	saida    = "supabase/migrations/20261118000002_demo.sql"
	marcador = "-- semente (gerada por scripts/gerar-semente-demo.go; não editar daqui para baixo)"
)

type produto struct {
	cod, desc, ncm string
	preco          float64
}

type servico struct {
	cTribNac, nbs, desc string
	preco               float64
}

type cliente struct {
	Razao    string   `json:"razaoSocial"`
	CNPJ     string   `json:"cnpj"`
	Regime   string   `json:"regime"`
	UF       string   `json:"uf"`
	Mun      int      `json:"municipioIbge"`
	CAtivSN  string   `json:"cAtivSn,omitempty"`
	PIBSSN   *float64 `json:"pIbsSn,omitempty"`
	PCBSSN   *float64 `json:"pCbsSn,omitempty"`
	mod      string   // 55 NF-e, 65 NFC-e
	produtos []produto
	servicos []servico
}

func f(v float64) *float64 { return &v }

var clientes = []cliente{
	{Razao: "Padaria Pão Quente Ltda", CNPJ: cnpj("88900160"), Regime: "simples", UF: "SP", Mun: 3509502, CAtivSN: "01", PIBSSN: f(1.25), PCBSSN: f(3.00), mod: "65",
		produtos: []produto{
			{"001", "Pão francês (kg)", "19059090", 18.90},
			{"002", "Pão de forma tradicional 500g", "19059010", 9.50},
			{"003", "Bolo de fubá (fatia)", "19059090", 7.00},
			{"004", "Panetone 500g", "19052010", 29.90},
			{"005", "Biscoito de polvilho 100g", "19059020", 6.50},
			{"006", "Wafer de chocolate 140g", "19053200", 4.80},
			{"007", "Queijo muçarela fatiado (kg)", "04061010", 54.90},
			{"008", "Presunto cozido fatiado (kg)", "16024100", 42.90},
			{"009", "Mortadela fatiada (kg)", "16010000", 24.90},
			{"010", "Manteiga com sal 200g", "04051000", 15.90},
			{"011", "Leite UHT integral 1L", "04012010", 5.49},
			{"012", "Ovos brancos (dúzia)", "04072100", 11.90},
			{"013", "Café torrado e moído 500g", "09012100", 22.90},
			{"014", "Arroz tipo 1 5kg", "10063021", 27.90},
			{"015", "Açúcar refinado 1kg", "17019900", 4.99},
			{"016", "Farinha de trigo 1kg", "11010010", 5.79},
			{"017", "Macarrão espaguete 500g", "19021900", 4.59},
			{"018", "Óleo de soja 900ml", "15079011", 7.49},
			{"019", "Refrigerante cola 2L", "22021000", 10.90},
			{"020", "Cerveja pilsen lata 350ml", "22030000", 4.29},
			{"021", "Água mineral sem gás 500ml", "22011000", 2.50},
			{"022", "Suco de laranja integral 1L", "20091200", 12.90},
			{"023", "Chocolate ao leite 90g", "18069000", 6.99},
			{"024", "Cigarro maço 20un", "24022000", 11.50},
			{"025", "Iogurte natural 170g", "04039000", 3.49},
		}},
	{Razao: "Farmácia Saúde Ltda", CNPJ: cnpj("87832350"), Regime: "regular", UF: "SP", Mun: 3550308, mod: "65",
		produtos: []produto{
			{"7891001", "Paracetamol 750mg 20 comp.", "30049045", 12.90},
			{"7891002", "Dipirona sódica 500mg 10 comp.", "30049099", 6.90},
			{"7891003", "Amoxicilina 500mg 21 cáps.", "30042099", 34.50},
			{"7891004", "Losartana potássica 50mg 30 comp.", "30049069", 18.90},
			{"7891005", "Omeprazol 20mg 28 cáps.", "30049079", 22.40},
			{"7891006", "Ibuprofeno 400mg 10 comp.", "30049039", 14.70},
			{"7891007", "Soro fisiológico 0,9% 500ml", "30049029", 8.90},
			{"7891008", "Xarope expectorante 120ml", "30039099", 27.80},
			{"7891009", "Curativo adesivo 40un", "30051090", 11.50},
			{"7891010", "Seringa descartável 5ml", "90183119", 1.20},
			{"7891011", "Luva de procedimento (cx 100)", "40151900", 39.90},
			{"7891012", "Creme dental 90g", "33061000", 5.99},
			{"7891013", "Escova dental macia", "96032100", 8.49},
			{"7891014", "Sabonete em barra 85g", "34011190", 2.79},
			{"7891015", "Xampu anticaspa 200ml", "33051000", 24.90},
			{"7891016", "Desodorante roll-on 50ml", "33072010", 14.90},
			{"7891017", "Absorvente higiênico (8un)", "96190000", 6.49},
			{"7891018", "Creme hidratante facial 50g", "33049910", 49.90},
			{"7891019", "Batom hidratante", "33041000", 29.90},
			{"7891020", "Perfume feminino 50ml", "33030010", 129.00},
			{"7891021", "Protetor solar FPS 50 120ml", "33049990", 59.90},
			{"7891022", "Suplemento de vitamina C 60 cáps.", "21069030", 32.90},
			{"7891023", "Fórmula infantil 800g", "19011010", 64.90},
			{"7891024", "Água mineral 500ml", "22011000", 3.50},
			{"7891025", "Bebida isotônica 500ml", "22029900", 6.90},
		}},
	{Razao: "Escola Aprender Ltda", CNPJ: cnpj("81214139"), Regime: "regular", UF: "SP", Mun: 3547809, mod: "55",
		servicos: []servico{
			{"080101", "122011100", "Mensalidade berçário/creche", 1450.00},
			{"080101", "122011200", "Mensalidade pré-escola", 1380.00},
			{"080101", "122012000", "Mensalidade ensino fundamental", 1620.00},
			{"080101", "122013000", "Mensalidade ensino médio", 1890.00},
			{"080101", "122020000", "Curso técnico em administração", 980.00},
			{"080101", "122031000", "EJA ensino fundamental", 420.00},
			{"080101", "122032000", "EJA ensino médio", 460.00},
			{"080201", "122051100", "Oficina de música", 220.00},
			{"080201", "122051200", "Escolinha de futebol", 180.00},
			{"080201", "122051300", "Curso de inglês", 310.00},
			{"080201", "122051400", "Palestra para pais", 60.00},
			{"080201", "122051900", "Reforço escolar", 250.00},
			{"080201", "122052000", "Orientação pedagógica", 150.00},
			{"160201", "104011210", "Transporte escolar", 390.00},
			{"080101", "", "Período integral", 690.00}, // sem NBS: cai em Revisar com as opções da LC 116
		},
		produtos: []produto{
			{"LIV01", "Livro didático 5º ano", "49019900", 189.00},
			{"APO01", "Apostila bimestral", "49011000", 95.00},
			{"UNI01", "Camiseta do uniforme", "61091000", 59.90},
			{"UNI02", "Calça do uniforme", "62034200", 89.90},
			{"AGD01", "Agenda escolar", "48201000", 35.00},
		}},
}

// cnpj completa a raiz com a filial 0001 e os dígitos verificadores.
func cnpj(raiz string) string {
	c := raiz + "0001"
	for _, pesos := range [][]int{{5, 4, 3, 2, 9, 8, 7, 6, 5, 4, 3, 2}, {6, 5, 4, 3, 2, 9, 8, 7, 6, 5, 4, 3, 2}} {
		s := 0
		for i, p := range pesos {
			s += int(c[i]-'0') * p
		}
		d := 11 - s%11
		if d >= 10 {
			d = 0
		}
		c += fmt.Sprint(d)
	}
	return c
}

// dv é o dígito verificador módulo 11 da chave de acesso.
func dv(chave string) int {
	s, p := 0, 2
	for i := len(chave) - 1; i >= 0; i-- {
		s += int(chave[i]-'0') * p
		if p++; p > 9 {
			p = 2
		}
	}
	if d := 11 - s%11; d < 10 {
		return d
	}
	return 0
}

func nfe(c cliente, n int, its []produto, rnd *rand.Rand) string {
	ch := fmt.Sprintf("35%s%s%s001%09d1%08d", "2610", c.CNPJ, c.mod, n, n*7919%100000000)
	ch += fmt.Sprint(dv(ch))
	var b strings.Builder
	fmt.Fprintf(&b, `<?xml version="1.0" encoding="UTF-8"?>
<nfeProc xmlns="http://www.portalfiscal.inf.br/nfe" versao="4.00"><NFe xmlns="http://www.portalfiscal.inf.br/nfe"><infNFe Id="NFe%s" versao="4.00"><ide><mod>%s</mod><nNF>%d</nNF></ide><emit><CNPJ>%s</CNPJ><xNome>%s</xNome></emit>
`, ch, c.mod, n, c.CNPJ, c.Razao)
	for i, p := range its {
		q := 1 + rnd.IntN(12)
		fmt.Fprintf(&b, `<det nItem="%d"><prod><cProd>%s</cProd><xProd>%s</xProd><NCM>%s</NCM><qCom>%d</qCom><vProd>%.2f</vProd></prod></det>
`, i+1, p.cod, p.desc, p.ncm, q, p.preco*float64(q))
	}
	b.WriteString("</infNFe></NFe></nfeProc>\n")
	return b.String()
}

func nfse(c cliente, n int, s servico) string {
	id := fmt.Sprintf("%d21%s%013d2610%09d", c.Mun, c.CNPJ, n, n)
	id += fmt.Sprint(dv(id))
	nbs := ""
	if s.nbs != "" {
		nbs = "<cNBS>" + s.nbs + "</cNBS>"
	}
	return fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<NFSe xmlns="http://www.sped.fazenda.gov.br/nfse" versao="1.00"><infNFSe Id="NFS%s"><emit><CNPJ>%s</CNPJ><xNome>%s</xNome></emit>
<valores><vLiq>%.2f</vLiq></valores>
<DPS><infDPS><serv><cServ><cTribNac>%s</cTribNac>%s<xDescServ>%s</xDescServ></cServ></serv><valores><vServPrest><vServ>%.2f</vServ></vServPrest></valores></infDPS></DPS></infNFSe></NFSe>
`, id, c.CNPJ, c.Razao, s.preco, s.cTribNac, nbs, s.desc, s.preco)
}

// arquivos monta as notas do mês: cada produto aparece em 1 a 4 notas, cada serviço em 1 a 3.
func arquivos(c cliente) map[string]string {
	rnd := rand.New(rand.NewPCG(uint64(c.Mun), 42)) // semente fixa: rodar de novo dá as mesmas notas
	out := map[string]string{}
	if len(c.produtos) > 0 {
		for n := 1; n <= 12; n++ {
			var its []produto
			for _, p := range c.produtos {
				if rnd.IntN(4) == 0 {
					its = append(its, p)
				}
			}
			if len(its) > 0 {
				out[fmt.Sprintf("nfe-%03d.xml", n)] = nfe(c, n, its, rnd)
			}
		}
		out["nfe-999.xml"] = nfe(c, 999, c.produtos, rnd) // todo produto sai em ao menos uma nota
	}
	for i, s := range c.servicos {
		for k := range 1 + rnd.IntN(3) {
			m := 100 + i*10 + k
			out[fmt.Sprintf("nfse-%03d.xml", m)] = nfse(c, m, s)
		}
	}
	return out
}

type sessao struct{ api, token string }

func (s sessao) chamar(metodo, rota, tipo string, corpo io.Reader, v any) {
	req, _ := http.NewRequest(metodo, s.api+rota, corpo)
	req.Header.Set("Authorization", "Bearer "+s.token)
	if tipo != "" {
		req.Header.Set("Content-Type", tipo)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		log.Fatalf("%s %s: %v", metodo, rota, err)
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 300 {
		log.Fatalf("%s %s: %d %s", metodo, rota, resp.StatusCode, b)
	}
	if v != nil {
		if err := json.Unmarshal(b, v); err != nil {
			log.Fatalf("%s %s: %v", metodo, rota, err)
		}
	}
}

func (s sessao) json(metodo, rota string, in, out any) {
	b, _ := json.Marshal(in)
	s.chamar(metodo, rota, "application/json", bytes.NewReader(b), out)
}

func env(k, padrao string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	if padrao == "" {
		log.Fatalf("defina %s", k)
	}
	return padrao
}

func psql(db, sql string) string {
	out, err := exec.Command("psql", db, "-v", "ON_ERROR_STOP=1", "-Atc", sql).Output()
	if err != nil {
		log.Fatalf("psql: %v", err)
	}
	return strings.TrimSpace(string(out))
}

func main() {
	supa, anon := env("SUPABASE_URL", "http://127.0.0.1:54321"), env("SUPABASE_ANON_KEY", "")
	api, db := env("API_URL", "http://localhost:8090"), env("DATABASE_URL", "postgresql://postgres:postgres@127.0.0.1:54322/postgres")

	// usuário temporário: as linhas dele viram as do modelo no SQL e ele é apagado no fim
	email := fmt.Sprintf("semente-%d@rtc-hub.invalid", time.Now().UnixNano())
	b, _ := json.Marshal(map[string]string{"email": email, "password": "semente-" + fmt.Sprint(time.Now().UnixNano())})
	req, _ := http.NewRequest("POST", supa+"/auth/v1/signup", bytes.NewReader(b))
	req.Header.Set("apikey", anon)
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		log.Fatal(err)
	}
	var auth struct {
		Token string `json:"access_token"`
		User  struct {
			ID string `json:"id"`
		} `json:"user"`
	}
	json.NewDecoder(resp.Body).Decode(&auth)
	resp.Body.Close()
	if auth.Token == "" {
		log.Fatalf("signup falhou (%d); confira SUPABASE_ANON_KEY", resp.StatusCode)
	}
	defer psql(db, fmt.Sprintf("delete from auth.users where id = '%s'", auth.User.ID))
	s := sessao{api, auth.Token}
	s.json("PUT", "/aliquotas", map[string]float64{"cbs": 8.80, "ibsUf": 15.00, "ibsMun": 2.70, "isAdValorem": 0}, nil)

	for _, c := range clientes {
		var criado struct {
			ID string `json:"id"`
		}
		s.json("POST", "/clientes", c, &criado)
		var corpo bytes.Buffer
		mp := multipart.NewWriter(&corpo)
		xmls := arquivos(c)
		for _, nome := range slices.Sorted(maps.Keys(xmls)) { // ordem fixa: o último valor de cada item é estável
			w, _ := mp.CreateFormFile("arquivos[]", nome)
			io.WriteString(w, xmls[nome])
		}
		mp.Close()
		var imp struct {
			Novos     int `json:"itensNovos"`
			Ignorados []struct{ Arquivo, Motivo string }
		}
		s.chamar("POST", "/clientes/"+criado.ID+"/importar", mp.FormDataContentType(), &corpo, &imp)
		if len(imp.Ignorados) > 0 {
			log.Fatalf("%s: arquivos ignorados: %+v", c.Razao, imp.Ignorados)
		}
		s.json("POST", "/clientes/"+criado.ID+"/calcular", map[string]int{"ano": 2027}, nil)
		log.Printf("%s: %d itens, situações %s", c.Razao, imp.Novos,
			psql(db, fmt.Sprintf("select string_agg(situacao || '=' || n, ' ') from (select situacao, count(*) n from itens where cliente_id = '%s' group by 1 order by 1) t", criado.ID)))
	}

	// uma linha JSON por registro; o user_id é trocado pelo do modelo na carga
	var sql strings.Builder
	for _, t := range []struct{ tabela, ordem string }{{"clientes", "razao_social"}, {"itens", "(select razao_social from clientes c where c.id = t.cliente_id), codigo, ncm, nbs"}, {"aliquotas_estimadas", "user_id"}} {
		linhas := psql(db, fmt.Sprintf("select string_agg((to_jsonb(t) - 'user_id')::text, E',\\n' order by %s) from %s t where user_id = '%s'", t.ordem, t.tabela, auth.User.ID))
		fmt.Fprintf(&sql, "\ninsert into %s select (jsonb_populate_record(null::%s, r || '{\"user_id\": \"%s\"}')).*\nfrom jsonb_array_elements($semente$[\n%s\n]$semente$::jsonb) r;\n", t.tabela, t.tabela, modelo, linhas)
	}
	atual, err := os.ReadFile(saida)
	if err != nil {
		log.Fatal(err)
	}
	cab, _, ok := strings.Cut(string(atual), marcador)
	if !ok {
		log.Fatalf("%s sem a linha marcadora", saida)
	}
	if err := os.WriteFile(saida, []byte(cab+marcador+"\n"+sql.String()), 0o644); err != nil {
		log.Fatal(err)
	}
	log.Printf("semente gravada em %s", saida)
}
