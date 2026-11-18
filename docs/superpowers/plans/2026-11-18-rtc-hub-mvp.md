# rtc-hub MVP (Classificação do Cadastro) Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Sistema web em que um escritório contábil importa os itens de cada cliente (XML ou planilha), recebe a sugestão de CST/cClassTrib a partir das tabelas oficiais da Receita, revisa, calcula CBS/IBS/IS por ano e exporta XLSX, com uma demo pronta para entrevista.

**Architecture:** Monorepo. `engines/` empacota os dois motores Java da Receita (fixados por commit) e gera o SQLite oficial num volume compartilhado. `api/` (Go) lê esse SQLite em modo somente leitura para sugerir classificações, faz o parse de XML e planilhas, chama os motores por HTTP e persiste no Supabase Postgres com RLS. `web/` (Next.js) faz login pelo Supabase Auth e conversa só com a api.

**Tech Stack:** Go 1.27 (`net/http`, `pgx/v5`, `modernc.org/sqlite`, `excelize/v2`, `golang-jwt/jwt/v5` + `MicahParks/keyfunc/v3`), Next.js (App Router, TypeScript, `@supabase/ssr`), Supabase (Auth, Postgres, Storage, pg_cron), Docker Compose, Java 21 (só dentro das imagens dos motores).

**Spec:** `docs/superpowers/specs/2026-11-18-rtc-hub-design.md`

## Global Constraints

- Motores fixados: regular `ReceitaFederal/calculadora-rtc-regime-regular@150cccb`, simples `ReceitaFederal/calculadora-rtc-simples-nacional@c5b0f01`. Não modificar o código deles.
- Motores só na rede interna do compose, **sem porta publicada** em produção.
- A api abre o SQLite oficial **somente leitura** (`file:...?mode=ro`).
- Linguagem da interface: a do contador (pt-BR), sem jargão de programador.
- Toda sugestão mostra o **porquê** (descrição da classificação + fundamentação legal curta). Caso ambíguo vira ⚠. "Provável" não conta como pronto até ser confirmado.
- Situações do item: `ok | provavel | revisar | invalido`. Origem: `sugerido | confirmado | manual`. Escolha `confirmado`/`manual` **nunca** é sobrescrita por nova importação.
- Só notas de **saída** entram (emitente = CNPJ do cliente). As demais são ignoradas com aviso.
- Ano de cálculo: 2026 → sem `aliquotasNominais` (o motor proíbe). 2027+ → `aliquotasNominais` obrigatório, vindo de `aliquotas_estimadas`, com o resultado marcado "simulação". Seletor abre em 2027. Simples em 2026: mensagem explicativa, sem chamar o motor.
- Alíquotas estimadas padrão: CBS 8,80 / IBS estadual 15,00 / IBS municipal 2,70 (soma 26,5%, estimativa do Ministério da Fazenda de 2024; a divisão UF/município é aproximação, editável). A fonte fica num comentário no código.
- Simples: o contador digita `cAtivSN` (2 dígitos), `pIBSSN` e `pCBSSN`. O sistema não calcula RBT12 nem Fator R.
- Valor base do cálculo = último valor visto do item, ou 100,00 se não houver.
- Os arquivos enviados não são guardados no Storage, só os itens extraídos (desvio consciente do spec: poupa o Storage grátis e evita guardar notas de terceiros).
- Repo público. Deploy não pode afetar n8n/Evolution/multivozes (projeto Easypanel separado, limite de memória).

## Review Focus

1. **A mesma nota importada duas vezes** (ou o mesmo ZIP enviado de novo) → as ocorrências não podem dobrar. Teste em Task 7 (`TestImportarNotaRepetidaNaoDuplica`).
2. **CSV exportado pelo Excel brasileiro** (separador `;`, codificação Windows-1252, vírgula decimal) → precisa ler acentos e valores certos. Teste em Task 4 (`TestCSVExcelBR`).
3. **NCM vindo com pontos ou espaços** (`1006.30.21`, ` 10063021 `) e NCM com menos de 8 dígitos na planilha → normalizar; NCM incompleto vira ✗ com motivo "NCM incompleto". Teste em Task 2 (`TestNormalizaNCM`).
4. **XML que não é nota** (evento de cancelamento, NF-e de entrada, arquivo corrompido dentro do ZIP) → o arquivo é ignorado com motivo, e a importação dos outros continua. Teste em Task 3 (`TestZipComArquivosRuins`).
5. **Cliente do Simples com item de serviço (NFS-e)** → o motor do Simples só calcula NF-e/NFC-e (`tpDoc` 55/65). O item fica classificado e o cálculo mostra "o motor do Simples ainda não calcula serviços", sem erro. Teste em Task 6 (`TestSimplesServicoNaoCalcula`).

---

### Task 1: Esqueleto do repo, toolchain e imagens dos motores

**Files:**
- Create: `.gitignore`, `README.md` (stub), `docker-compose.yml`, `engines/java.Dockerfile`, `engines/README.md`
- Create: `scripts/extrair-tabelas.sh`

**Interfaces:**
- Produces: serviços compose `calc-regular` (porta interna 8080, base `/api/calculadora`) e `calc-simples` (porta interna 8082, base `/api/calculadora`); volume nomeado `tabelas` montado em `/app/calculadora/db` com `calculadora-pro.db`; arquivo local `.data/calculadora-pro.db` (gitignored) para os testes Go, via `scripts/extrair-tabelas.sh`.

- [ ] **Step 1: Instalar Go 1.27 sem sudo**: baixar o tarball oficial de go.dev para `~/.local/go`, colocar `~/.local/go/bin` no PATH do fish (`fish_add_path`). Verificar: `go version` → `go1.27.x`.
- [ ] **Step 2: Escrever `engines/java.Dockerfile`** (multi-stage, um arquivo para os dois motores com `ARG REPO` e `ARG COMMIT`):
  - stage `build` (`eclipse-temurin:21-jdk`): `git clone` do REPO, `git checkout $COMMIT`; clonar também o regular@150cccb para pegar `flyway/` (o Simples não tem a pasta); `./mvnw flyway:migrate -Dflyway.configFiles=./flyway/flyway-pro.conf`; `./mvnw -DskipTests package`.
  - stage final (`eclipse-temurin:21-jre`): copia o JAR e `calculadora/db/calculadora-pro.db`; `ENV SPRING_PROFILES_ACTIVE=offline`; `-XX:MaxRAMPercentage=75`.
- [ ] **Step 3: Escrever `docker-compose.yml`** com `calc-regular` e `calc-simples` (build com os ARGs), volume `tabelas` em `/app/calculadora/db` nos dois (o regular popula o volume na primeira criação), `mem_limit: 768m` cada e healthcheck em `/api/versao` (simples) / `/api/swagger-ui.html` (regular). Comentário: `# shortcut: o volume só é populado na criação; ao trocar o COMMIT, rodar "docker volume rm rtc-hub_tabelas"`.
- [ ] **Step 4: Subir e conferir que roda offline**: `docker compose up -d --build calc-regular calc-simples`, depois `docker compose logs calc-simples | grep -i "VERSAO_URL\|DB_DOWNLOAD\|UnknownHost"`. Esperado: nenhuma tentativa de rede bloqueando a subida. Se houver, sobrescrever as variáveis para vazio no compose e documentar em `engines/README.md`.
- [ ] **Step 5: Smoke test dos dois motores**: `POST /api/calculadora/observabilidade/regime-geral` com o `operacao.json` de `docs/exemplos.md` do regular → esperado `objetos[0].estadoItem == "CALCULO_SIMULADO"`. Simples: `POST /api/calculadora/observabilidade/simples-nacional` com `{"id":"t","versao":"2.0.0","data":"2027-03-15","tpDoc":"55","CRT":"1","itens":[{"numero":1,"cAtivSN":"01","cClassTrib":"000001","baseCalculo":"100.00","valorImpostoSeletivo":"0.00","pIBSSN":"1.25","pCBSSN":"3.00"}]}` → esperado HTTP 200 com o item calculado. Salvar as duas respostas em `api/internal/motores/testdata/` (servem de fixture na Task 6).
- [ ] **Step 6: `scripts/extrair-tabelas.sh`**: `docker compose cp calc-regular:/app/calculadora/db/calculadora-pro.db .data/`. Rodar e conferir com `ls -lh .data/calculadora-pro.db`.
- [ ] **Step 7: Commit** `chore: motores da Receita em containers com SQLite compartilhado`.

### Task 2: Motor de sugestão (`tabelas`)

**Files:**
- Create: `api/go.mod` (module `rtchub`), `api/internal/tabelas/tabelas.go`, `api/internal/tabelas/sugerir.go`, `api/internal/tabelas/sugerir_test.go`

**Interfaces:**
- Produces:
  ```go
  type Tipo string // "NCM" | "NBS"
  type Situacao string // "ok" | "provavel" | "revisar" | "invalido"
  type Opcao struct { CST, CClassTrib, Descricao, Fundamentacao, Condicao string; PercReducao float64 }
  type Classificacao struct { Situacao Situacao; Escolhida *Opcao; Opcoes []Opcao }
  type Sugestao struct {
      Situacao Situacao          // a pior entre IBSCBS e IS
      Motivo   string            // preenchido quando invalido
      IBSCBS   Classificacao
      IS       *Classificacao    // nil quando o NCM não tem IS
  }
  func Abrir(path string) (*Tabelas, error)                 // abre com mode=ro
  func NormalizaCodigo(s string) string                     // tira pontos, espaços, traços
  func (t *Tabelas) Sugerir(ctx context.Context, tipo Tipo, codigo string, data string) (Sugestao, error)
  func (t *Tabelas) NBSPorServicoLC116(ctx context.Context, cTribNac, data string) ([]string, error)
  ```

Regra (o código do motor em `NcmAplicavelRepository.java` é a referência de casamento por prefixo):

```
1. codigo := NormalizaCodigo(codigo). NCM com len != 8 → invalido "NCM incompleto".
   Não existe vigente em NCM/NBS (… <= data, fim nulo ou >= data) → invalido "NCM inexistente ou fora de vigência".
2. candidatos := linhas de NCM_APLICAVEL (ou NBS_APLICAVEL) n JOIN CLASSIFICACAO_TRIBUTARIA ct
   onde n.COD = substr(codigo, 1, length(n.COD)), n e ct vigentes na data,
   e NÃO existe EXCECAO_* e (e.*_NCMA_ID = n.ID, e.COD = substr(codigo,1,length(e.COD)), e vigente).
   Para cada candidato: CST (SITUACAO_TRIBUTARIA via CLTR_SITR_ID), tributo pelo TRIBUTO_SITUACAO_TRIBUTARIA
   (TBTO_ID = 1 → grupo IS; senão grupo IBSCBS), fundamentação = FUNDAMENTACAO_LEGAL.FDLG_TEXTO_CURTO
   vigente via FUNDAMENTACAO_CLASSIFICACAO, redução = PERCENTUAL_REDUCAO (maior PERE_VALOR vigente),
   condição = junção de "Exige comprador: " + TOMADOR_ESPECIFICO.TMES_NOME (se CLTR_TMES_ID)
   e "Só em: " + tipos de TIPO_DFE_CLASSIFICACAO (se houver linhas vigentes). Deduplicar por cClassTrib.
3. IBSCBS: 0 → ok, Escolhida = CST 000 / 000001 (descrição lida do banco);
           1 → provavel, Escolhida = o candidato, Opcoes = [candidato, 000001];
          >1 → revisar, Escolhida = nil, Opcoes = candidatos + 000001.
   IS:     0 → nil; 1 → ok; >1 → revisar.
4. Situacao = pior entre os grupos (invalido > revisar > provavel > ok).
```

- [ ] **Step 1: Descobrir valores reais para os testes**: com `sqlite3 .data/calculadora-pro.db`, achar (a) um NCM de 8 dígitos com exatamente um candidato IBSCBS e sem IS, (b) um com ≥2 candidatos IBSCBS, (c) um candidato com `CLTR_TMES_ID` não nulo, (d) um NCM coberto por uma exceção vigente em 2027-01-01. Anotar os códigos no topo do arquivo de teste como constantes, com o SQL usado em comentário.
- [ ] **Step 2: Escrever os testes que falham** (`sugerir_test.go`, abre `../../../.data/calculadora-pro.db`, `t.Skip` se não existir; data `"2027-01-01"`):
  - `TestNormalizaNCM`: `NormalizaCodigo(" 1006.30.21 ") == "10063021"`; `Sugerir(NCM, "100630")` → `invalido`, Motivo contém `"incompleto"`.
  - `TestNCMSemTratamento`: NCM comum (ex. `84713012`) → `ok`, `IBSCBS.Escolhida.CClassTrib == "000001"`, `IS == nil`.
  - `TestNCMUmTratamento`: NCM (a) → `provavel`, `len(Opcoes)==2`, `Opcoes[1].CClassTrib=="000001"`, `Escolhida.Fundamentacao != ""`.
  - `TestNCMAmbiguo`: NCM (b) → `revisar`, `Escolhida == nil`, `len(Opcoes) >= 3`.
  - `TestCondicaoComprador`: candidato (c) → `Condicao` começa com `"Exige comprador: "`.
  - `TestExcecao`: NCM (d) não traz o cClassTrib excetuado.
  - `TestCervejaTemIS`: `22030000` → `IS != nil`, `IS.Situacao == "ok"`.
  - `TestNCMInexistente`: `99999999` → `invalido`.
  - `TestNBSPorServico`: um `cTribNac` real (consultar o formato de `LISTA_SERVICOS_LC_116.LSLC_CD` e a conversão a partir do `cTribNac` de 6 dígitos) → lista não vazia.
- [ ] **Step 3: Rodar** `cd api && go test ./internal/tabelas/` → FAIL (funções não definidas).
- [ ] **Step 4: Implementar** `tabelas.go` (Abrir, NormalizaCodigo) e `sugerir.go` (uma query SQL por grupo; usar `database/sql` + `modernc.org/sqlite`).
- [ ] **Step 5: Rodar** os testes → PASS.
- [ ] **Step 6: Commit** `feat(api): sugestão de CST/cClassTrib pelas tabelas oficiais`.

### Task 3: Parse de XML (NF-e/NFC-e/NFS-e) e ZIP

**Files:**
- Create: `api/internal/importa/xml.go`, `api/internal/importa/zip.go`, `api/internal/importa/xml_test.go`, `api/internal/importa/testdata/*.xml|*.zip`

**Interfaces:**
- Produces:
  ```go
  type ItemBruto struct { Codigo, Descricao, NCM, NBS, CTribNac string; Valor float64; Origem string /* "nfe"|"nfse"|"planilha" */ }
  type Nota struct { Chave, EmitenteCNPJ string; Itens []ItemBruto }
  type Ignorado struct { Arquivo, Motivo string }
  func LerNota(nome string, r io.Reader) (Nota, error)            // detecta NF-e/NFC-e (nfeProc ou NFe) e NFS-e nacional (NFSe)
  func LerArquivos(nome string, r io.Reader, cnpjCliente string) ([]Nota, []Ignorado)   // aceita .xml e .zip
  ```
- NF-e: `infNFe/@Id` (sem prefixo "NFe") = Chave, `emit/CNPJ`, `det/prod/{cProd,xProd,NCM,vProd}`. NFS-e: `infNFSe/@Id` = Chave, `infNFSe/emit/CNPJ`, `DPS/infDPS/serv/cServ/{cTribNac,cNBS,xDescServ}`, `valores/vServPrest/vServ`; `Codigo` = `cTribNac`.

- [ ] **Step 1: Criar fixtures mínimas** à mão a partir dos leiautes: `nfe_saida.xml` (nfeProc, 2 itens, emitente 11222333000181), `nfce.xml` (NFe sem nfeProc, mod 65), `nfse_com_nbs.xml`, `nfse_sem_nbs.xml` (só cTribNac), `nfe_entrada.xml` (outro emitente), `evento_cancelamento.xml` (raiz procEventoNFe), `lote.zip` contendo todos + um `corrompido.xml`.
- [ ] **Step 2: Escrever os testes que falham**:
  - `TestNFeSaida`: 2 itens, `NCM=="10063021"`, `Valor` com o vProd, `Chave` com 44 dígitos.
  - `TestNFCe`: lida igual à NF-e.
  - `TestNFSeComESemNBS`: com NBS → `NBS` preenchido; sem → `NBS==""` e `CTribNac != ""`.
  - `TestZipComArquivosRuins`: `LerArquivos("lote.zip", …, "11222333000181")` → 4 notas válidas; `Ignorado` contém `nfe_entrada.xml` com motivo `"emitente diferente do cliente"`, o evento com `"não é nota fiscal"` e o corrompido com `"XML inválido"`.
- [ ] **Step 3: Rodar** → FAIL. **Step 4: Implementar** com `encoding/xml` (structs só com os campos acima; ignorar o namespace no match) e `archive/zip` (limite de 2.000 arquivos e 50 MB descompactados por ZIP). **Step 5: Rodar** → PASS.
- [ ] **Step 6: Commit** `feat(api): leitura de XML de NF-e, NFC-e e NFS-e`.

### Task 4: Planilha CSV/XLSX

**Files:**
- Create: `api/internal/importa/planilha.go`, `api/internal/importa/planilha_test.go`, `testdata/excel_br.csv`, `testdata/itens.xlsx`

**Interfaces:**
- Produces:
  ```go
  type Mapeamento struct { Descricao, Codigo, NCM, NBS, Valor int } // índice da coluna, -1 = ausente
  type Previa struct { Cabecalho []string; Linhas [][]string } // até 5 linhas
  func PreviaPlanilha(nome string, r io.Reader) (Previa, error)
  func LerPlanilha(nome string, r io.Reader, m Mapeamento) ([]ItemBruto, []Ignorado, error)
  ```
- CSV: detectar separador (`;` ou `,`) pela primeira linha; se o conteúdo não for UTF-8 válido, decodificar como Windows-1252 (`golang.org/x/text/encoding/charmap`). Valor aceita `1.234,56` e `1234.56`. Linha sem NCM e sem NBS → Ignorado `"linha N: sem NCM/NBS"`.

- [ ] **Step 1: Testes que falham**: `TestCSVExcelBR` (arquivo `;` em CP1252 com "Pão francês" e "1.234,56" → `Descricao=="Pão francês"`, `Valor==1234.56`), `TestXLSX` (primeira aba, mapeamento por índice), `TestPrevia` (cabeçalho + no máximo 5 linhas), `TestLinhaSemCodigo`.
- [ ] **Step 2: Rodar** → FAIL. **Step 3: Implementar** (XLSX com `excelize/v2`, primeira aba). **Step 4: Rodar** → PASS.
- [ ] **Step 5: Commit** `feat(api): importação de planilha CSV/XLSX`.

### Task 5: Banco no Supabase, RLS e camada `store`

**Files:**
- Create: `supabase/config.toml` (via `npx supabase init`; habilitar `enable_anonymous_sign_ins = true`), `supabase/migrations/20261118000001_schema.sql`, `api/internal/store/store.go`, `api/internal/store/store_test.go`

**Interfaces:**
- Schema (tudo com `user_id uuid not null default auth.uid() references auth.users on delete cascade`, RLS `user_id = auth.uid()` para select/insert/update/delete):
  - `clientes(id uuid pk, razao_social, cnpj char(14), regime text check in ('regular','simples'), uf char(2), municipio_ibge int, c_ativ_sn char(2), p_ibs_sn numeric, p_cbs_sn numeric, criado_em)`; unique (user_id, cnpj).
  - `notas_importadas(cliente_id fk cascade, chave text, primary key(cliente_id, chave))`.
  - `importacoes(id, cliente_id, tipo text, arquivos int, itens_novos int, itens_atualizados int, ignorados jsonb, criado_em)`.
  - `itens(id uuid, cliente_id fk cascade, codigo, descricao, ncm, nbs, c_trib_nac, ocorrencias int, ultimo_valor numeric, situacao text, origem text, cst, cclasstrib, is_cst, is_cclasstrib, condicao text, opcoes jsonb, calculos jsonb default '{}' /* chave = ano */, atualizado_em)`; unique (cliente_id, codigo, coalesce(ncm,''), coalesce(nbs,'')).
  - `aliquotas_estimadas(user_id pk, cbs numeric default 8.80, ibs_uf numeric default 15.00, ibs_mun numeric default 2.70, is_ad_valorem numeric default 0)`.
- Produces (Go):
  ```go
  func Conectar(ctx context.Context, databaseURL string) (*Store, error)
  // Executa fn numa transação com role authenticated e request.jwt.claims = {"sub": userID, "role":"authenticated"}
  func (s *Store) ComoUsuario(ctx context.Context, userID string, fn func(pgx.Tx) error) error
  ```
  As queries de cada recurso ficam em funções no mesmo pacote (`ListarClientes`, `CriarCliente`, `UpsertItens`, `ListarItens`, `AtualizarItem`, `ConfirmarItens`, `SalvarCalculo`, `Aliquotas`, `SalvarAliquotas`, `RegistrarNotas` que devolve as chaves já existentes), todas recebendo `pgx.Tx`.

- [ ] **Step 1: `npx supabase init` e `npx supabase start`** (Supabase local em Docker). Escrever a migration. `npx supabase db reset` → sem erro.
- [ ] **Step 2: Teste que falha** `TestRLSIsolaEscritorios` (usa `SUPABASE_DB_URL` local; cria 2 usuários em `auth.users`; A cria cliente; `ComoUsuario(B)` lista 0 clientes e um `UPDATE` no cliente de A afeta 0 linhas). `TestUpsertPreservaManual`: item com origem `manual` mantém cst/cclasstrib após `UpsertItens` com sugestão diferente, mas soma `ocorrencias` e atualiza `ultimo_valor`.
- [ ] **Step 3: Rodar** → FAIL. **Step 4: Implementar** `store.go` (pgxpool; `set local role authenticated; select set_config('request.jwt.claims', $1, true)`). **Step 5: Rodar** → PASS.
- [ ] **Step 6: Commit** `feat: schema com RLS e acesso como usuário`.

### Task 6: Cliente dos motores e regra de cálculo por ano

**Files:**
- Create: `api/internal/motores/motores.go`, `api/internal/motores/calcular.go`, `api/internal/motores/calcular_test.go`

**Interfaces:**
- Consumes: fixtures de resposta da Task 1 (`testdata/`).
- Produces:
  ```go
  type Aliquotas struct { CBS, IBSUF, IBSMun, ISAdValorem float64 }
  type ClienteCalc struct { Regime string; UF string; MunicipioIBGE int; CAtivSN, PIBSSN, PCBSSN string }
  type ItemCalc struct { ID, NCM, NBS, CST, CClassTrib, ISCST, ISCClassTrib string; Valor float64 }
  type Resultado struct {
      ItemID string; Estado string /* do motor, ou "NAO_SUPORTADO" */; Simulacao bool
      VCBS, VIBS, VIS, PRedCBS, PRedIBS string; Mensagem string; Bruto json.RawMessage
  }
  func Novo(urlRegular, urlSimples string, hc *http.Client) *Motores
  func (m *Motores) Calcular(ctx context.Context, ano int, c ClienteCalc, al Aliquotas, itens []ItemCalc) ([]Resultado, error)
  ```
- Regras: regular → `POST {urlRegular}/api/calculadora/observabilidade/regime-geral`, `dhFatoGerador = "<ano>-03-10T10:00:00-03:00"`, `quantidade 1`, `unidade "UN"`, `aliquotasNominais` só se `ano >= 2027`, grupo `impostoSeletivo` só se `ISCClassTrib != ""`; lotes de 100 itens. Simples → `POST {urlSimples}/api/calculadora/observabilidade/simples-nacional` com `versao "2.0.0"`, `data "<ano>-03-15"`, `tpDoc "55"`, `CRT "1"`, valores como string com 2 casas. `Simulacao = ano >= 2027`.

- [ ] **Step 1: Testes que falham** (com `httptest.Server` devolvendo as fixtures e capturando o request):
  - `TestRegular2026SemAliquotas`: o JSON enviado não tem a chave `aliquotasNominais`; `Simulacao == false`.
  - `TestRegular2027ComAliquotas`: o envio tem `"cbs":8.8`; `Simulacao == true`; `VCBS` extraído da fixture.
  - `TestSimples2026NaoChamaMotor`: nenhum request; `Estado == "NAO_SUPORTADO"`, `Mensagem` contém `"a partir de 2027"`.
  - `TestSimplesServicoNaoCalcula`: item com `NBS` e regime simples → `NAO_SUPORTADO`, `Mensagem` contém `"ainda não calcula serviços"`.
  - `TestErroPorItem`: item com `erros` na resposta → `Mensagem` = `title` + `detail` do motor.
- [ ] **Step 2: Rodar** → FAIL. **Step 3: Implementar.** **Step 4: Rodar** → PASS.
- [ ] **Step 5: Teste de integração** `//go:build integracao` `TestMotoresReais` contra o compose (`calc-regular` publicado só no override de dev `docker-compose.dev.yml` em 127.0.0.1): arroz 10063021 / CST 200 / 200003 em 2027 → `PRedCBS == "100.00"`. Rodar com `go test -tags integracao ./internal/motores/`.
- [ ] **Step 6: Commit** `feat(api): cálculo nos motores com regra por ano e regime`.

### Task 7: API HTTP (auth, clientes, importação, revisão, cálculo, alíquotas)

**Files:**
- Create: `api/cmd/api/main.go`, `api/internal/httpapi/{auth.go,rotas.go,clientes.go,importacao.go,itens.go,calculo.go,aliquotas.go}`, `api/internal/httpapi/*_test.go`, `api/Dockerfile`
- Modify: `docker-compose.yml` (serviço `api`, volume `tabelas` read-only em `/tabelas`)

**Interfaces:**
- Consumes: `tabelas.Sugerir`, `importa.LerArquivos/PreviaPlanilha/LerPlanilha`, `store.*`, `motores.Calcular`.
- Env: `DATABASE_URL`, `SUPABASE_URL` (JWKS em `/auth/v1/.well-known/jwks.json`), `TABELAS_DB=/tabelas/calculadora-pro.db`, `CALC_REGULAR_URL`, `CALC_SIMPLES_URL`, `CORS_ORIGIN`, `PORT=8090`.
- Rotas (todas JSON, `Authorization: Bearer <jwt>`, exceto health):
  - `GET /health/live`, `GET /health/ready` (SQLite aberto + motores respondem)
  - `GET|POST /clientes`, `GET|PUT|DELETE /clientes/{id}`
  - `POST /clientes/{id}/importar` multipart `arquivos[]` (+ `mapeamento` JSON quando for planilha) → `{itensNovos, itensAtualizados, notasRepetidas, ignorados:[{arquivo,motivo}]}`
  - `POST /clientes/{id}/planilha/previa` multipart `arquivo` → `Previa`
  - `GET /clientes/{id}/itens?situacao=` → itens + contagem por situação
  - `PATCH /itens/{id}` `{cst,cClassTrib,isCst?,isCClassTrib?}` → origem `manual`, situação `ok`
  - `POST /clientes/{id}/itens/confirmar` `{ids:[]}` → `provavel` → `ok`, origem `confirmado`
  - `POST /clientes/{id}/calcular` `{ano}` → calcula os itens `ok`; itens não prontos voltam em `pendentes`
  - `GET|PUT /aliquotas`
- Importação: notas cujas `Chave` já estão em `notas_importadas` são puladas (contam em `notasRepetidas`). Itens NFS-e sem NBS: `Sugestao` = `revisar` com as opções vindas de `NBSPorServicoLC116`. Limite do upload: 50 MB por requisição.

- [ ] **Step 1: Testes que falham** (`httptest`, Supabase local, JWT assinado no teste com chave injetada no verificador):
  - `TestSemTokenDa401`.
  - `TestImportarNFeSugere`: importa `nfe_saida.xml` → `itensNovos == 2`; `GET itens` traz o arroz com situação ≠ `""`.
  - `TestImportarNotaRepetidaNaoDuplica`: importa o mesmo arquivo 2× → na segunda, `notasRepetidas == 1`, `itensAtualizados == 0`, e `ocorrencias` continua 1.
  - `TestConfirmarEmLote` e `TestPatchViraManual`.
  - `TestCalcularSoItensProntos`: item `revisar` volta em `pendentes`.
- [ ] **Step 2: Rodar** → FAIL. **Step 3: Implementar** (stdlib `net/http` com padrões de rota do Go 1.22+; verificador JWT com `keyfunc` + `jwt/v5`, exigindo `aud == "authenticated"`). **Step 4: Rodar** → PASS.
- [ ] **Step 5: `api/Dockerfile`** multi-stage (`golang:1.27` → `gcr.io/distroless/static`), `CGO_ENABLED=0`. `docker compose up -d --build api` → `curl localhost:8090/health/ready` → `{"status":"ok"}`.
- [ ] **Step 6: Commit** `feat(api): rotas de clientes, importação, revisão e cálculo`.

### Task 8: Exportação XLSX

**Files:**
- Create: `api/internal/exporta/xlsx.go`, `api/internal/exporta/xlsx_test.go`
- Modify: `api/internal/httpapi/rotas.go` (rota `GET /clientes/{id}/exportar.xlsx?ano=`)

**Interfaces:**
- Produces: `func Cadastro(w io.Writer, cliente store.Cliente, itens []store.Item, ano int, al motores.Aliquotas) error`
- Colunas na ordem: Código, Descrição, NCM, NBS, Situação, CST, cClassTrib, Classificação, % Redução, Fundamentação, Origem, CST IS, cClassTrib IS, vCBS, vIBS, vIS, Observação. Segunda aba "Parâmetros": cliente, regime, ano, alíquotas usadas e o aviso "Simulação: alíquotas estimadas, não oficiais" quando `ano >= 2027`.

- [ ] **Step 1: Teste que falha** `TestExportaColunasEAviso`: reabrir o arquivo gerado com excelize; `A1 == "Código"`; a aba "Parâmetros" contém `"Simulação"` para 2027 e não contém para 2026.
- [ ] **Step 2: Rodar** → FAIL. **Step 3: Implementar.** **Step 4: Rodar** → PASS.
- [ ] **Step 5: Commit** `feat(api): exportação do cadastro em XLSX`.

### Task 9: Web — login, demo, clientes e alíquotas

**Files:**
- Create: `web/` (via `npx create-next-app@latest web --ts --app --eslint --tailwind --no-src-dir`), `web/lib/supabase.ts`, `web/lib/api.ts`, `web/app/login/page.tsx`, `web/app/clientes/page.tsx`, `web/app/clientes/novo/page.tsx`, `web/app/aliquotas/page.tsx`, `web/Dockerfile`
- Modify: `docker-compose.yml` (serviço `web`)

**Interfaces:**
- Consumes: rotas da Task 7; `supabase.rpc('clonar_demo')` (Task 11).
- Produces: `api<T>(path: string, init?: RequestInit): Promise<T>` em `web/lib/api.ts`, que injeta o `access_token` da sessão.
- Telas: Login (e-mail + senha, e botão **"Entrar como demo"** = `signInAnonymously()` e depois `rpc('clonar_demo')`); Clientes (lista com contagem ✓/⚠/✗ por cliente); Novo cliente (para o Simples aparecem os campos "Atividade do Simples (cAtivSN)", "Alíquota efetiva IBS (%)" e "Alíquota efetiva CBS (%)", com texto de ajuda "as mesmas do cálculo do DAS"); Alíquotas (formulário com o aviso "estimativas para simulação, não são alíquotas oficiais").

- [ ] **Step 1: Teste e2e que falha** (Playwright, `web/e2e/demo.spec.ts`): clicar "Entrar como demo" → a página de clientes mostra "Padaria" (depende da Task 11; até lá, marcar `test.fixme`). Teste `novo-cliente.spec.ts`: escolher "Simples" mostra o campo "Atividade do Simples".
- [ ] **Step 2: Implementar** as telas. **Step 3: Rodar** `npx playwright test novo-cliente` → PASS.
- [ ] **Step 4: Commit** `feat(web): login, demo, clientes e alíquotas`.

### Task 10: Web — importação, revisão, cálculo e exportação

**Files:**
- Create: `web/app/clientes/[id]/page.tsx` (tabela de itens), `web/app/clientes/[id]/importar/page.tsx`, `web/components/TabelaItens.tsx`, `web/components/MapearColunas.tsx`, `web/e2e/fluxo.spec.ts`

**Interfaces:**
- Consumes: rotas das Tasks 7 e 8.
- Tela do cliente: filtros ✓ ok / ✓ provável / ⚠ revisar / ✗ (com contagem); coluna "Por quê" com a classificação, a fundamentação e a condição; seleção múltipla + botão **"Confirmar selecionados"** (só para `provavel`); itens `revisar` com um seletor das opções; seletor de ano (2026, 2027…2033; padrão 2027) + botão "Calcular"; valores calculados com o selo "simulação" quando `ano >= 2027`; botão "Exportar XLSX".
- Importar: arrastar XML/ZIP/CSV/XLSX; se for planilha, a tela `MapearColunas` mostra a prévia e pede para escolher as colunas; no fim, o resumo (novos, atualizados, repetidos, ignorados com motivo).

- [ ] **Step 1: Teste e2e que falha** `fluxo.spec.ts`: criar cliente regular → importar `api/internal/importa/testdata/nfe_saida.xml` → ver o arroz com "provável" → selecionar e confirmar → calcular 2027 → o selo "simulação" aparece → exportar baixa um `.xlsx`.
- [ ] **Step 2: Implementar.** **Step 3: Rodar** `npx playwright test fluxo` com o compose e o Supabase local de pé → PASS.
- [ ] **Step 4: Commit** `feat(web): importação, revisão em lote, cálculo e exportação`.

### Task 11: Demo pronta, README e CI

**Files:**
- Create: `supabase/migrations/20261118000002_demo.sql`, `scripts/gerar-semente-demo.go` (ou `.sh`), `.github/workflows/ci.yml`, `docs/demo.gif`
- Modify: `README.md`

**Interfaces:**
- Produces: `clonar_demo()` (SQL, `security definer`, `search_path = public`): copia os clientes, itens (com sugestões e cálculos já gravados) e alíquotas do usuário-modelo fixo `00000000-0000-0000-0000-00000000de00` para `auth.uid()`, uma vez só por usuário (idempotente). Job `pg_cron` diário: `delete from auth.users where is_anonymous and created_at < now() - interval '7 days'`.
- Semente: 3 clientes (Padaria Pão Quente, Simples, cAtivSN `01`; Farmácia Saúde, regular, SP; Escola Aprender, regular, serviços com NBS), cada um com 15–40 itens reais, gerados passando XMLs fictícios pela própria api (assim a semente reflete o motor de sugestão de verdade) e exportados para o SQL da migration.

- [ ] **Step 1: Teste que falha**: `TestClonarDemoIdempotente` (Go, Supabase local): chamar `clonar_demo()` 2× como o mesmo usuário anônimo → 3 clientes, não 6; outro usuário anônimo recebe a própria cópia.
- [ ] **Step 2: Implementar** a migration e o gerador da semente. **Step 3: Rodar** → PASS. Tirar o `test.fixme` do `demo.spec.ts` (Task 9) e rodar → PASS.
- [ ] **Step 4: README** para quem vai avaliar: o problema (a reforma obriga a reclassificar o cadastro de cada cliente), o GIF de 30 s do fluxo da demo, o link da demo, "como funciona" em 5 linhas (os motores oficiais da Receita + sugestão + revisão), e no fim a seção técnica (diagrama, `docker compose up`, testes).
- [ ] **Step 5: CI** (`ci.yml`): `go test ./...` (os testes do SQLite são pulados sem `.data`), `npm run lint && npm run build` em `web/`, build das imagens `api` e `web`, publicando no GHCR na `main`.
- [ ] **Step 6: Commit** `feat: demo pronta, README e CI`.

### Task 12: Deploy no Easypanel

**Files:**
- Create: `docs/deploy.md`

- [ ] **Step 1:** Criar o projeto Supabase cloud e aplicar as migrations (`npx supabase link` + `npx supabase db push`); habilitar o login anônimo e o pg_cron no painel.
- [ ] **Step 2:** No Easypanel (pela UI), criar o projeto **`rtc-hub`**, separado de `maturare_leads`, com os serviços `calc-regular`, `calc-simples` (imagens construídas a partir de `engines/java.Dockerfile`, sem domínio, 768 MB cada), `api` (512 MB) e `web` (256 MB), com o volume `tabelas` compartilhado. Antes, conferir a RAM livre da VPS; se não couber, parar e avisar.
- [ ] **Step 3:** O domínio e o TLS ficam a cargo do usuário. Sem domínio, usar o subdomínio padrão do Easypanel.
- [ ] **Step 4:** Verificar: o botão "Entrar como demo" funciona na URL pública; n8n, Evolution e multivozes continuam no ar (`ssh maturare-vps docker ps`; a CLI `maturare` não existe nesta máquina).
- [ ] **Step 5: Commit** `docs: deploy`.
