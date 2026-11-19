# Módulo 2 — Conferência de notas fiscais — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** o upload de XML de saída de um cliente passa a conferir cada nota (estrutura, grupo IBS/CBS, classificação, valores e totais) nos motores da Receita e mostra o resultado por competência, com exportação XLSX e demo pronta.

**Architecture:** o parser (`importa`) passa a extrair o grupo IBS/CBS/IS e o cabeçalho da nota. Um pacote novo, `confere`, recebe as notas e produz as conferências. Ele não sabe de HTTP nem de banco: fala com motores e tabelas por uma interface, agrupa as chamadas ao motor por data de emissão e aplica as regras do design. `httpapi.importar` grava o cadastro primeiro, chama `confere` e grava as conferências numa tabela nova com RLS. A web ganha a página `clientes/[id]/conferencia`.

**Tech Stack:** Go 1.x (api, `encoding/xml`, pgx), Supabase Postgres (RLS), Next.js da pasta `web/` (leia `web/AGENTS.md`: a versão do Next tem mudanças incompatíveis, consulte `web/node_modules/next/dist/docs/`), Playwright, motores Java da Receita via HTTP.

**Spec:** `docs/superpowers/specs/2026-11-19-conferencia-notas-design.md` (e, para o contexto do módulo 1, `docs/superpowers/specs/2026-11-18-rtc-hub-design.md`).

## Global Constraints

- Toda mensagem para o usuário em pt-BR, na linguagem do contador: o que está na nota, o que era esperado, por quê. Nada de jargão de programador.
- Achado sem certeza vira "não consegui conferir" com o motivo, **nunca ✗** (anti-objetivo: falso ✗).
- Tolerância de valores: **R$ 0,01 por item**.
- Toda regra usa a **data de emissão** da nota, nunca a data de hoje.
- Grupo IBS/CBS ausente: **2026 → aviso** (todo regime); **2027+ → erro no regular**; Simples → aviso. Nota emitida **antes de 2026-01-01** não gera achado de ausência.
- NFS-e: só estrutura e classificação (valores e totais são do Ambiente Nacional).
- Simples: regime pelo cadastro do cliente; `CRT` da nota divergente → aviso. Em 2026, valores "não conferidos (Simples em 2026)". 2027+: motor do Simples com `cAtivSN`/`pIBSSN`/`pCBSSN` do cadastro; se faltarem → "não consegui conferir: preencha a atividade e as alíquotas do Simples no cadastro do cliente".
- 2027+ regular: confere a conta com as alíquotas da própria nota; aviso de alíquota diferente da estimada **só em operação interna** (destino na mesma UF e no mesmo município do cliente).
- Divergência classificação × cadastro: **erro** só com o item *confirmado* ou *manual*; com o item *sugerido* ou *provável* é **aviso**.
- Item novo entra no cadastro com a **sugestão do sistema** (origem `sugerido`), e não com o cClassTrib da nota.
- Reenvio de nota já importada: **não** soma ocorrências; refaz e **substitui** a conferência.
- Limites de upload iguais aos do módulo 1 (50 MB, 2000 arquivos no ZIP). Meta: ZIP de 500 notas em cerca de 1 minuto.
- Tabelas novas seguem o padrão RLS do `20261118000001_schema.sql`: `user_id default auth.uid()`, FK `(cliente_id, user_id)`, política `_dono`, revoke de `anon` e de truncate/references/trigger de `authenticated`.
- Nada de serviço novo na VPS; o processamento fica dentro da api.
- Commits terminam com `Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>`.

## Review Focus

1. **`nfeProc` real com `protNFe` e `Signature`** (o que o contador tem em mãos) precisa passar na validação de estrutura. É o caso mais comum, e os exemplos da Receita usam `enviNFe`. Teste na Task 2.
2. **Nota anterior à reforma (2025) sem grupo IBS/CBS** no mesmo ZIP: não pode virar aviso nem erro. Situação ✓ com uma informação "nota anterior a 2026". Teste na Task 4.
3. **Motor fora do ar no meio do upload:** o cadastro é gravado, as notas ficam "não consegui conferir (motor indisponível)", e a resposta é 200, não 500. Teste na Task 7.
4. **NFC-e (modelo 65)** precisa ser validada como `NFCE`, não como `NFE`. Teste na Task 2.
5. **Item com IS e item sem IS na mesma nota:** o IS é conferido só onde existe, e o total `ISTot` só quando há IS. Teste na Task 5.

---

### Task 1: Parser lê o grupo IBS/CBS/IS e o cabeçalho da nota

**Files:**
- Modify: `api/internal/importa/xml.go`
- Create: `api/internal/importa/testdata/nfe_ibscbs.xml`: cópia de `calculadora-rtc-regime-regular/src/test/resources/xml/nfe/valido/nota/nfe_valido.xml` (MIT), com o elemento `NFe` envolvido em `nfeProc` + um `protNFe` mínimo, e o `emit/CNPJ` mantido.
- Create: `api/internal/importa/testdata/nfse_ibscbs.xml`: cópia de `.../xml/nfse/valido/nota/nfse_valido_com_IBSCBS.xml`.
- Test: `api/internal/importa/xml_test.go`

O repo da Receita está em `https://github.com/ReceitaFederal/calculadora-rtc-regime-regular`. Clone raso fora do repo.

**Interfaces:**
- Produces (em `importa`):
```go
type TribItem struct {
	CST, CClassTrib string
	Tem             bool // grupo IBSCBS presente no item
	VBC, PIBSUF, PRedIBSUF, VIBSUF, PIBSMun, PRedIBSMun, VIBSMun, VIBS, PCBS, PRedCBS, VCBS string // texto como no XML; "" = ausente
	ISCST, ISCClassTrib, VIS string
}
type Totais struct{ VBC, VIBS, VCBS, VIS string; TemIBSCBS, TemIS bool }
// campos novos de Nota:
//   Modelo string ("55"|"65"|"nfse"), Numero string, Emissao string (YYYY-MM-DD), CRT string,
//   DestUF string, DestMun string, Totais Totais, XML []byte (bytes originais)
// campo novo de ItemBruto: NItem int, Trib TribItem
```
- NF-e: caminhos `det/@nItem`, `det/imposto/IBSCBS/{CST,cClassTrib,gIBSCBS/...}`, `det/imposto/IS/{CSTIS,cClassTribIS,vIS}`, `total/IBSCBSTot/{vBCIBSCBS,gIBS/vIBS,gCBS/vCBS}`, `total/ISTot/vIS`, `ide/{mod,nNF,dhEmi}`, `emit/CRT`, `dest/enderDest/{UF,cMun}`. Confira os nomes dos campos do IS no `grupo.xsd` da NF-e.
- NFS-e: `Emissao` vem de `infDPS/dhEmi` (data local); a classificação vem de `infDPS/IBSCBS/valores/trib/gIBSCBS/{CST,cClassTrib}`; os valores ficam vazios de propósito (são do Ambiente Nacional).

- [ ] **Step 1: Escrever os testes que falham**

```go
func TestNFeComIBSCBS(t *testing.T) {
	n := lerArquivo(t, "nfe_ibscbs.xml")
	if n.Modelo != "55" || n.Numero != "5" || n.Emissao != "2025-09-01" || n.CRT != "3" { t.Fatalf("%+v", n) }
	it := n.Itens[0].Trib
	if !it.Tem || it.CST != "200" || it.CClassTrib != "200022" || it.VBC != "100.00" || it.PIBSUF != "10.00" || it.PRedIBSUF != "100.00" || it.VCBS != "0.00" { t.Fatalf("%+v", it) }
	if !n.Totais.TemIBSCBS || n.Totais.VBC != "100.00" { t.Fatalf("%+v", n.Totais) }
	if len(n.XML) == 0 { t.Fatal("XML original não guardado") }
}
func TestNFeSemGrupo(t *testing.T) { // fixture antiga do módulo 1
	n := lerArquivo(t, "nfe_saida.xml")
	if n.Itens[0].Trib.Tem || n.Totais.TemIBSCBS { t.Fatal("não devia ter grupo") }
}
func TestNFSeComIBSCBS(t *testing.T) {
	n := lerArquivo(t, "nfse_ibscbs.xml")
	it := n.Itens[0].Trib
	if n.Modelo != "nfse" || !it.Tem || it.CST != "000" || it.CClassTrib != "000001" || it.VCBS != "" { t.Fatalf("%+v %+v", n, it) }
}
```
Use (ou crie) um helper `lerArquivo(t, nome) Nota` que abre `testdata/<nome>` e chama `LerNota`.

- [ ] **Step 2:** `cd api && go test ./internal/importa/ -run 'IBSCBS|SemGrupo'`. Esperado: FAIL (campos indefinidos).
- [ ] **Step 3: Implementar** os campos novos nos structs de unmarshal de `xml.go`. Não muda a assinatura de `LerNota`/`LerArquivos`. `Emissao` = os 10 primeiros caracteres de `dhEmi`.
- [ ] **Step 4:** `cd api && go test ./internal/importa/`. Esperado: PASS (inclusive os testes antigos).
- [ ] **Step 5: Commit** `feat(importa): lê grupo IBS/CBS/IS e cabeçalho da nota`

---

### Task 2: Validação de estrutura no motor

**Files:**
- Create: `api/internal/motores/xml.go`
- Test: `api/internal/motores/xml_test.go` (unitário com `httptest`) e um caso a mais em `api/internal/motores/integracao_test.go` (motor real, segue o padrão de skip já existente)

**Interfaces:**
- Produces:
```go
// ValidarXML chama POST {regular}/api/calculadora/xml/validate?tipo=<T>&subtipo=NOTA (Content-Type application/xml).
// erroSchema != "" quando o motor recusa o XML (mensagem do validador, com linha/coluna);
// err só para falha de transporte ou HTTP 5xx.
func (m *Motores) ValidarXML(ctx context.Context, modelo string, xml []byte) (erroSchema string, err error)
// modelo: "55"→NFE, "65"→NFCE, "nfse"→NFSE
func TraduzErroSchema(msg string) string // tradução das mensagens mais comuns; sem tradução, devolve "Estrutura do XML recusada pelo schema oficial: " + msg
```
- Descubra o status HTTP e o corpo de `ErroXmlException` (`ProblemType.ERRO_XML`, formato problem+json) no motor real e trate esse status como `erroSchema`, não como `err`.

- [ ] **Step 1: Testes que falham**
  - `TestValidarXMLModeloParaTipo`: o motor falso registra a query; `"65"` → `tipo=NFCE`, `"55"` → `NFE`, `"nfse"` → `NFSE`, todos com `subtipo=NOTA`.
  - `TestValidarXMLRecusado`: o motor falso responde com o status/corpo de ERRO_XML (copiar do motor real) → `erroSchema` contém "linha", `err == nil`.
  - `TestValidarXMLMotorFora`: 500 → `err != nil`.
  - `TestTraduzErroSchema`: uma mensagem `cvc-pattern-valid` de `pCBS` com 4 casas vira texto que contém `"pCBS"` e `"casas decimais"`.
  - Integração (`TestMotoresReais`): `importa/testdata/nfe_ibscbs.xml` (`nfeProc` com `protNFe`) → `erroSchema == ""`. Se o `protNFe` mínimo ou a `Signature` impedirem a validação, ajuste o fixture para um `protNFe` válido pelo schema. **Não** remova `protNFe` no código, porque o contador sobe o `nfeProc` inteiro.
- [ ] **Step 2:** `cd api && go test ./internal/motores/ -run 'Validar|Traduz'`. Esperado: FAIL.
- [ ] **Step 3: Implementar** `ValidarXML` e `TraduzErroSchema` (comece por 3 padrões: `cvc-pattern-valid`, `cvc-complex-type.2.4.a` (campo inesperado/faltando), `cvc-maxLength-valid`; extraia o nome do campo da mensagem).
- [ ] **Step 4:** testes unitários PASS; com os motores locais rodando (`docker compose -f docker-compose.yml -f docker-compose.dev.yml up -d`), `go test ./internal/motores/ -run Reais` PASS.
- [ ] **Step 5: Commit** `feat(motores): validação de XML no schema oficial`

---

### Task 3: Cálculo na data da nota e alíquotas na resposta

**Files:**
- Modify: `api/internal/motores/calcular.go`, `api/internal/motores/motores.go`
- Test: `api/internal/motores/calcular_test.go`

**Interfaces:**
- Produces:
```go
// CalcularEm: igual a Calcular, mas com a data do fato gerador (YYYY-MM-DD).
// Calcular(ctx, ano, ...) passa a chamar CalcularEm(ctx, fmt.Sprintf("%d-03-10", ano), ...). Comportamento atual preservado.
func (m *Motores) CalcularEm(ctx context.Context, data string, c ClienteCalc, al Aliquotas, itens []ItemCalc) ([]Resultado, error)
// Resultado ganha (omitempty): VIBSUF, VIBSMun, PIBSUF, PIBSMun, PCBS string  // do tribCalc do regular
```
- `sim` (alíquotas nominais) passa a valer para `data >= "2027-01-01"`. A data do Simples é a própria `data`.

- [ ] **Step 1: Testes que falham**
  - `TestCalcularEmUsaData`: o motor falso captura o corpo; `CalcularEm(ctx, "2026-05-18", ...)` → `dhFatoGerador` começa com `"2026-05-18"`, sem `aliquotasNominais`.
  - `TestCalcularEm2027MandaNominais`: `"2027-02-01"` → `aliquotasNominais.cbs == al.CBS`.
  - `TestResultadoTrazAliquotas`: com `testdata/regular-resposta.json` → `PCBS`, `PIBSUF`, `VIBSUF` não vazios e iguais aos do JSON.
- [ ] **Step 2:** `go test ./internal/motores/ -run 'CalcularEm|TrazAliquotas'`. Esperado: FAIL.
- [ ] **Step 3: Implementar.** Os testes antigos de `Calcular` continuam sem mudança.
- [ ] **Step 4:** `go test ./internal/motores/`. Esperado: PASS.
- [ ] **Step 5: Commit** `feat(motores): cálculo na data da nota`

---

### Task 4: Pacote `confere`: estrutura, ausência do grupo e classificação

**Files:**
- Create: `api/internal/confere/confere.go`, `api/internal/confere/regras.go`
- Test: `api/internal/confere/confere_test.go` (só fakes, sem rede nem banco)

**Interfaces:**
- Consumes: `importa.Nota` (Task 1), `motores.ValidarXML`/`TraduzErroSchema` (Task 2), `motores.CalcularEm` (Task 3), `tabelas.Classificacao`/`ValeParaCodigo` (existentes).
- Produces:
```go
type Gravidade string // "erro" | "aviso" | "info"
type Situacao string  // "ok" | "aviso" | "divergencia" | "nao_conferido"
type Achado struct {
	NItem        int       `json:"nItem"`        // 0 = a nota toda
	Codigo       string    `json:"codigo"`       // código do produto/serviço
	Tipo         string    `json:"tipo"`         // "estrutura"|"ausente"|"classificacao"|"cadastro"|"valor"|"total"|"regime"|"aliquota_estimada"|"nao_conferido"
	Gravidade    Gravidade `json:"gravidade"`
	NaNota       string    `json:"naNota"`
	Esperado     string    `json:"esperado"`
	Diferenca    float64   `json:"diferenca,omitempty"` // R$, só tipo "valor"/"total"
	Explicacao   string    `json:"explicacao"`
	Fundamentacao string   `json:"fundamentacao,omitempty"`
}
type Conferencia struct {
	Chave, Modelo, Numero, Emissao string
	Situacao Situacao
	Achados  []Achado
}
type ItemCadastro struct{ CST, CClassTrib, Origem, Situacao string } // origem/situação do módulo 1
type Cliente struct {
	Regime, UF string; MunicipioIBGE int
	CAtivSN, PIBSSN, PCBSSN string
	Estimadas motores.Aliquotas
}
type Deps interface {
	ValidarXML(ctx context.Context, modelo string, xml []byte) (string, error)
	CalcularEm(ctx context.Context, data string, c motores.ClienteCalc, al motores.Aliquotas, itens []motores.ItemCalc) ([]motores.Resultado, error)
	Classificacao(ctx context.Context, cClassTrib, data string, is bool) (*tabelas.Opcao, error)
	ValeParaCodigo(ctx context.Context, tipo tabelas.Tipo, codigo, cClassTrib, data string, is bool) (bool, error)
}
// cadastro: chave = importa item "codigo|ncm|nbs" (mesma unicidade de itens).
func ConferirLote(ctx context.Context, d Deps, cli Cliente, notas []importa.Nota, cadastro map[string]ItemCadastro) []Conferencia
// regras.go — a regra de ausência fica num só lugar, com fonte/data num comentário:
func gravidadeAusencia(regime, emissao string) (Gravidade, bool) // bool=false: não gera achado (antes de 2026)
```
- A situação da nota é derivada dos achados: algum `erro` → `divergencia`; senão algum `nao_conferido` → `nao_conferido`; senão algum `aviso` → `aviso`; senão `ok` (`info` não rebaixa).
- `ValidarXML` com `err` (motor fora) → achado `nao_conferido` "motor da Receita indisponível; tente de novo", e as outras regras que não dependem do motor continuam.
- Classificação: inexistente ou não vigente na data → `erro` com `Esperado` = "um cClassTrib vigente em <data>"; `ValeParaCodigo` falso → `erro` citando o NCM/NBS.
- Cadastro: item ausente do mapa → sem achado de cadastro; diferente e `Origem ∈ {confirmado, manual}` → `erro`; diferente e `sugerido`/`provavel` → `aviso` "a nota usa X; o sistema sugeriu Y, ainda não confirmado".
- `CRT` da nota ≠ regime do cadastro (CRT 1/2/4 = Simples/MEI; 3 = regular) → `aviso` tipo `regime`.

- [ ] **Step 1: Testes que falham** (um fake `Deps` configurável por campo):
  - `TestNotaOkSemAchados`: item 000001 vigente, igual ao cadastro confirmado, motor válido → `Situacao == "ok"`, sem achados.
  - `TestEstruturaRecusada`: `ValidarXML` devolve mensagem → achado `estrutura`/`erro`, situação `divergencia`.
  - `TestMotorForaNaoViraErro`: `ValidarXML` devolve `err` → situação `nao_conferido`, nenhum achado `erro`.
  - `TestAusente2026Aviso`: regular, emissão `2026-03-10`, `Trib.Tem=false` → `ausente`/`aviso`.
  - `TestAusente2027ErroRegular` / `TestAusente2027AvisoSimples`.
  - `TestNotaAnterior2026SemAchadoDeAusencia` (Review Focus 2): emissão `2025-09-01` → `Situacao == "ok"`, só `info`.
  - `TestClassificacaoInexistente`: `Classificacao` nil → `classificacao`/`erro`.
  - `TestCadastroConfirmadoDivergeErro` / `TestCadastroSugeridoDivergeAviso`.
  - `TestCRTDivergeDoCadastro`: CRT `"1"` com cliente regular → `regime`/`aviso`.
- [ ] **Step 2:** `cd api && go test ./internal/confere/`. Esperado: FAIL.
- [ ] **Step 3: Implementar** `ConferirLote` sem a parte de valores (Task 5), `gravidadeAusencia` e a derivação da situação.
- [ ] **Step 4:** `go test ./internal/confere/`. Esperado: PASS.
- [ ] **Step 5: Commit** `feat(confere): estrutura, grupo ausente e classificação`

---

### Task 5: `confere`: valores, totais, Simples e 2027+

**Files:**
- Modify: `api/internal/confere/confere.go` (+ `valores.go` se ajudar)
- Test: `api/internal/confere/valores_test.go`

**Interfaces:**
- Consumes: Task 4.
- Agrupamento: **uma chamada `CalcularEm` por data de emissão** dentro do lote (as notas do mesmo dia vão num lote só). O `ItemCalc.ID` liga o resultado ao par (nota, nItem).
- Regular, NF-e/NFC-e, item com `Trib.Tem`: `ItemCalc{CST, CClassTrib, NCM, Valor: vBC da nota, IS* quando houver}`. Em 2027+, `Aliquotas` = as da própria nota (`pCBS`, `pIBSUF`, `pIBSMun`). Compara `vIBSUF`, `vIBSMun`, `vCBS` e `vIS` com tolerância de 0,01; cada diferença vira achado `valor`/`erro` com `Diferenca` = nota − esperado. Em 2026, compara também `pCBS`/`pIBSUF` com o motor.
- Estado do motor por item ≠ `CALCULADO`/`CALCULO_SIMULADO` → achado `nao_conferido` com a `Mensagem` do motor (nunca `erro`).
- 2027+ operação interna (`DestUF == cli.UF` e `DestMun == cli.MunicipioIBGE`) e alíquota da nota ≠ estimada → `aliquota_estimada`/`aviso`.
- Totais (NF-e/NFC-e, `Totais.TemIBSCBS`): soma dos itens vs `IBSCBSTot` (`vBC`, `vIBS`, `vCBS`); `ISTot` só se `TemIS`. Diferença > 0,01 → `total`/`erro`.
- NFS-e: não calcula nem confere totais.
- Simples: 2026 → um achado `info` por nota "valores não conferidos (Simples em 2026)"; 2027+ → `CalcularEm` com `ClienteCalc{Regime:"simples", CAtivSN, PIBSSN, PCBSSN}` do cadastro e compara `vCBS`/`vIBS`; se faltar algum dos três → `nao_conferido` com o texto da Global Constraint.

- [ ] **Step 1: Testes que falham**
  - `TestValorCBSErrado`: o motor devolve vCBS 0.90; a nota diz 1.00 → `valor`/`erro`, `Diferenca == 0.10`.
  - `TestValorDentroTolerancia`: diferença de 0.01 → sem achado.
  - `TestUmaChamadaPorData`: 3 notas em 2 datas → o fake conta 2 chamadas `CalcularEm`.
  - `TestTotalNaoBate`: itens somam vCBS 0.90; `IBSCBSTot.gCBS.vCBS` = 1.90 → `total`/`erro`.
  - `TestISSoOndeExiste` (Review Focus 5): nota com um item com IS e outro sem; o fake confere que só o primeiro `ItemCalc` tem `ISCClassTrib`; com `TemIS=false` não há achado de `ISTot`.
  - `TestNFSeNaoConfereValores`: nota `nfse` → o fake não recebe chamada `CalcularEm`.
  - `TestSimples2026Info` / `TestSimples2027SemAliquotasNaoConferido` / `TestSimples2027Calcula`.
  - `Test2027AliquotaEstimadaSoInterna`: mesma UF/município → aviso; outra UF → sem aviso.
  - `TestEstadoMotorErroViraNaoConferido`.
- [ ] **Step 2:** `go test ./internal/confere/`. Esperado: FAIL nos novos.
- [ ] **Step 3: Implementar.** Converta os textos do XML com `strconv.ParseFloat`. Texto ilegível → `nao_conferido` "valor ilegível no XML: <campo>".
- [ ] **Step 4:** `go test ./internal/confere/`. Esperado: PASS.
- [ ] **Step 5: Commit** `feat(confere): valores, totais e Simples`

---

### Task 6: Tabela `conferencias`, store e cópia na demo

**Files:**
- Create: `supabase/migrations/20261119000001_conferencias.sql`
- Modify: `api/internal/store/store.go`
- Test: `api/internal/store/store_test.go`, `api/internal/store/demo_test.go`

**Interfaces:**
- Tabela:
```sql
create table conferencias (
  id uuid primary key default gen_random_uuid(),
  user_id uuid not null default auth.uid() references auth.users on delete cascade,
  cliente_id uuid not null,
  chave text not null,
  modelo text not null,          -- '55'|'65'|'nfse'
  numero text not null default '',
  emissao date not null,
  situacao text not null check (situacao in ('ok','aviso','divergencia','nao_conferido')),
  achados jsonb not null default '[]',
  conferido_em timestamptz not null default now(),
  unique (cliente_id, chave),
  foreign key (cliente_id, user_id) references clientes (id, user_id) on delete cascade
);
```
  RLS/revokes no mesmo padrão do schema; índice `(cliente_id, emissao)`. `create or replace function clonar_demo()` com o corpo atual mais a cópia de `conferencias` (pelo mesmo `mapa`); mantenha os mesmos `revoke`/`grant`.
- Produces (em `store`):
```go
type Conferencia struct {
	ID, ClienteID, Chave, Modelo, Numero, Emissao, Situacao string
	Achados json.RawMessage
}
func SalvarConferencias(ctx context.Context, tx pgx.Tx, clienteID string, cs []Conferencia) error // upsert por (cliente_id, chave): substitui situação, achados, conferido_em
func ListarConferencias(ctx context.Context, tx pgx.Tx, clienteID, competencia, situacao string) ([]Conferencia, error) // competencia "YYYY-MM" ("" = todas); situacao "" = todas; ordem emissao desc, numero
func ResumoConferencias(ctx context.Context, tx pgx.Tx, clienteID string) ([]ResumoCompetencia, error)
type ResumoCompetencia struct {
	Competencia string         `json:"competencia"` // YYYY-MM
	Total       int            `json:"total"`
	PorSituacao map[string]int `json:"porSituacao"`
}
func ObterConferencia(ctx context.Context, tx pgx.Tx, id string) (Conferencia, error)
func ItensDoCadastro(ctx context.Context, tx pgx.Tx, clienteID string) (map[string]Item, error) // chave "codigo|ncm|nbs"
```
- O JSON de `Conferencia` usa camelCase como o resto (`clienteId`, `emissao`…).

- [ ] **Step 1: Testes que falham** (precisam de `SUPABASE_DB_URL`, como os atuais)
  - `TestSalvarConferenciaSubstitui`: salvar a chave X com `divergencia` e depois com `ok` → uma linha, `ok`.
  - `TestListarPorCompetencia`: notas em 2026-03 e 2026-04 → o filtro `"2026-04"` devolve só a de abril.
  - `TestConferenciaRLS`: outro usuário não vê a linha (no padrão dos testes de RLS existentes).
  - `TestClonarDemoCopiaConferencias` em `demo_test.go`: o modelo com 1 conferência → o anônimo clonado tem 1.
- [ ] **Step 2:** `npx supabase db reset && cd api && go test ./internal/store/`. Esperado: FAIL.
- [ ] **Step 3: Implementar** a migration e as funções.
- [ ] **Step 4:** `npx supabase db reset && go test ./internal/store/`. Esperado: PASS.
- [ ] **Step 5: Commit** `feat(store): tabela de conferências`

---

### Task 7: Upload único: importar alimenta o cadastro e confere

**Files:**
- Modify: `api/internal/httpapi/importacao.go`, `api/internal/httpapi/rotas.go`
- Create: `api/internal/httpapi/conferencia.go`
- Test: `api/internal/httpapi/api_test.go`

**Interfaces:**
- Consumes: Tasks 1–6. `*Servidor` satisfaz `confere.Deps` com um adaptador pequeno sobre `s.Motores` e `s.Tabelas`.
- Fluxo em `importar`: (1) a transação atual grava o cadastro como hoje, e as notas repetidas continuam sem somar ocorrências. (2) Fora da transação, lê `ItensDoCadastro`/cliente/`store.Aliquotas`, chama `confere.ConferirLote` com **todas** as notas únicas do envio, repetidas inclusive. (3) Numa segunda transação, `SalvarConferencias`. A falha do motor nunca derruba o passo 1.
- Alternativa da nota: no passo (1), quando o item vem de uma nota com `Trib.CClassTrib` vigente (`Tabelas.Classificacao`) e ele não está nas opções sugeridas, acrescente essa `Opcao` em `opcoes.ibsCbs` com `Condicao: "usado na nota fiscal <número>"`. Não muda `cst`/`cclasstrib`/`origem` do item.
- `resultadoImportacao` ganha `Conferencia map[string]int \`json:"conferencia"\`` (contagem por situação).
- Rotas novas:
  - `GET /clientes/{id}/conferencias?competencia=&situacao=` → `{"resumo": []ResumoCompetencia, "notas": []store.Conferencia}`
  - `GET /conferencias/{id}` → `store.Conferencia`
  - `GET /clientes/{id}/conferencias.xlsx?competencia=` → XLSX (Task 8)

- [ ] **Step 1: Testes que falham** (o motor falso responde a `/xml/validate` e ao cálculo)
  - `TestImportarConfere`: importa `nfe_ibscbs.xml` (o CNPJ do cliente = emitente do fixture) → `conferencia` tem uma contagem > 0 e `GET /clientes/{id}/conferencias` lista a nota com a `emissao` certa.
  - `TestReenvioSubstituiSemSomarOcorrencias`: importa duas vezes → `ocorrencias` do item continua 1 e há uma só conferência.
  - `TestImportarMotorForaGravaCadastro` (Review Focus 3): o motor falso devolve 500 em tudo → status 200, o item está no cadastro e a conferência é `nao_conferido`.
  - `TestConferenciaDeOutroUsuario404`.
  - `TestItemNovoGanhaAlternativaDaNota`: nota com cClassTrib vigente diferente da sugestão → o item fica com a sugestão (`origem == "sugerido"`) e `opcoes.ibsCbs` contém o cClassTrib da nota com a condição "usado na nota fiscal".
  - Ajuste os testes antigos de importação se passarem a receber o campo novo (não remova asserções).
- [ ] **Step 2:** `cd api && go test ./internal/httpapi/`. Esperado: FAIL nos novos.
- [ ] **Step 3: Implementar.**
- [ ] **Step 4:** `go test ./...`. Esperado: PASS. `go vet ./... && gofmt -l .` sem saída.
- [ ] **Step 5: Commit** `feat(api): importação de XML confere as notas`

---

### Task 8: Exportação XLSX das divergências

**Files:**
- Modify: `api/internal/exporta/xlsx.go`, `api/internal/httpapi/conferencia.go`
- Test: `api/internal/exporta/xlsx_test.go`, `api/internal/httpapi/api_test.go`

**Interfaces:**
- Produces: `func Divergencias(w io.Writer, cliente store.Cliente, competencia string, cs []store.Conferencia) error`. Uma linha por achado `erro`/`aviso`/`nao_conferido` (sem `info`). Colunas, nesta ordem: Nota, Modelo, Emissão, Item, Código, Gravidade, Tipo, Na nota, Esperado, Diferença (R$), Explicação, Fundamentação. Use a mesma biblioteca de `Cadastro`.
- Rota: nome do arquivo `conferencia-<cnpj>-<competencia|todas>.xlsx`.

- [ ] **Step 1: Testes que falham**: `TestDivergenciasColunas` (abre o XLSX gerado; cabeçalho exato; 2 achados → 2 linhas; `info` não aparece) e `TestExportarConferenciasXLSX` (rota devolve `Content-Type` de planilha).
- [ ] **Step 2:** `go test ./internal/exporta/ ./internal/httpapi/ -run 'Divergencias|ExportarConferencias'`. Esperado: FAIL.
- [ ] **Step 3: Implementar.**
- [ ] **Step 4:** os mesmos testes PASS.
- [ ] **Step 5: Commit** `feat(exporta): XLSX das divergências`

---

### Task 9: Tela "Conferência de notas"

**Files:**
- Create: `web/app/clientes/[id]/conferencia/page.tsx`, `web/components/ListaConferencias.tsx`
- Modify: `web/app/clientes/[id]/page.tsx` (link/aba "Conferência de notas" ao lado do cadastro), `web/app/clientes/[id]/importar/page.tsx` (texto e resultado)
- Test: `web/e2e/fluxo.spec.ts`; fixture `web/e2e/fixtures/nfe_ibscbs.xml` (cópia do da Task 1)

**Interfaces:**
- Consumes: rotas da Task 7/8 via `api()`/`baixar()` de `web/lib/api.ts`; componentes de `web/components/ui.tsx`.
- Página: seletor de competência (do `resumo`, a mais recente por padrão), cartões ✓ / ⚠ / ✗ / "não consegui conferir" com contagem que filtram a lista, lista de notas (número, modelo, emissão, situação). Clicar na nota expande os achados: "Na nota" × "Esperado" × "Por quê" (e fundamentação). Botão "Exportar divergências (XLSX)".
- Rótulos fixos: `ok` "✓ Sem divergências", `aviso` "⚠ Avisos", `divergencia` "✗ Divergências", `nao_conferido` "Não consegui conferir".
- Importação: o texto do upload de XML passa a dizer que as notas são **incluídas no cadastro e conferidas**. Depois de enviar, mostra a contagem de `conferencia` com um link para a página nova.
- Mesmo padrão responsivo do módulo 1: no celular (390 px) a lista rola na horizontal e a página não.

- [ ] **Step 1: Teste e2e que falha** em `fluxo.spec.ts`: cria o cliente com o CNPJ emitente do fixture, importa `nfe_ibscbs.xml`, vê o resumo pós-importação, abre "Conferência de notas", vê uma nota listada e expande os achados.
- [ ] **Step 2:** `cd web && npx playwright test fluxo`. Esperado: FAIL.
- [ ] **Step 3: Implementar** (leia antes os docs do Next em `node_modules/next/dist/docs/` para as rotas dinâmicas desta versão).
- [ ] **Step 4:** `npm run lint && npm run build && npx playwright test`. Esperado: PASS, sem warnings novos de lint.
- [ ] **Step 5: Commit** `feat(web): tela de conferência de notas`

---

### Task 10: Demo com notas conferidas, deploy e verificação

**Files:**
- Modify: `scripts/gerar-semente-demo.go`
- Create: `supabase/migrations/20261119000002_demo_conferencia.sql` (gerado)
- Modify: `web/e2e/demo.spec.ts`, `docs/deploy.md` (se mudar algum passo)

**Interfaces:**
- O gerador passa a escrever na **migration nova** (constante `saida`), e não mais na `20261118000002`. Ela começa com `delete from clientes where user_id = '<modelo>'` (o cascade apaga os itens e as conferências) e reinsere clientes, itens e conferências do modelo, mais `aliquotas_estimadas` com `on conflict do update`. A migration 0002 fica como está (histórico).
- As notas geradas ganham o grupo IBS/CBS e a data de emissão em 2026. Por cliente, plante pelo menos:
  - **farmácia:** uma nota certa; uma com cClassTrib 000001 num medicamento que o cadastro tem confirmado com redução (✗ cadastro); uma com vCBS sobre a base errada (✗ valor);
  - **escola (NFS-e):** uma certa; uma sem o grupo (⚠ ausente 2026);
  - **padaria (Simples):** notas com o grupo (info "Simples em 2026") e uma com o total que não bate.
  Os itens que o teste exige *confirmados* no cadastro são confirmados pelo gerador via `POST /clientes/{id}/itens/confirmar` antes do upload das notas.
- `demo.spec.ts` ganha: entrar como demo → farmácia → "Conferência de notas" → existem ✓ e ✗; escola → existe ⚠.

- [ ] **Step 1: Teste e2e que falha** em `demo.spec.ts` (acima).
- [ ] **Step 2:** stack local (`npx supabase start`, motores + api) e `npx playwright test demo`. Esperado: FAIL.
- [ ] **Step 3: Implementar** o gerador; rode-o (`SUPABASE_ANON_KEY=... API_URL=http://localhost:8090 go run scripts/gerar-semente-demo.go`) e depois `npx supabase db reset`.
- [ ] **Step 4:** `npx playwright test` e `cd api && go test ./...` PASS.
- [ ] **Step 5: Commit** `feat(demo): notas conferidas na demo` e push.
- [ ] **Step 6: Produção:** `npx supabase db push --remote --dry-run` mostra as duas migrations novas. Peça ao Lucas para confirmar o push em produção e rode-o com `--yes`. Redeploy do serviço `app` no Easypanel (veja `docs/deploy.md`). Verifique no site: entrar como demo → farmácia → conferência com ✓ e ✗; layout em 390 px sem rolagem horizontal da página.
