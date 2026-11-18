# rtc-hub — Design

Data: 2026-11-18 · Status: revisado (builder check), aguardando aprovação

## Intenção e porquê

Projeto de **portfólio para vagas de backend**. O objetivo é mostrar arquitetura,
containers e integração de serviços num domínio real e atual: a Reforma
Tributária do Consumo (LC 214/2025, CBS/IBS/IS).

O sistema é um **SaaS web** que orquestra os três motores open source (MIT)
publicados pela Receita Federal:

| Motor | Repo | Stack | Papel no rtc-hub |
|---|---|---|---|
| Calculadora regime regular | `ReceitaFederal/calculadora-rtc-regime-regular` | Java 21 / Spring / SQLite | cálculo CBS/IBS/IS, geração dos grupos de tributação em XML, validação XSD de nota completa, dados abertos (NCM, NBS, cClassTrib, alíquotas) |
| Calculadora Simples Nacional | `ReceitaFederal/calculadora-rtc-simples-nacional` | Java 21 / Spring / SQLite | cálculo CBS/IBS para empresas do Simples, atividades por anexo |
| Gerador DANFSe | `ReceitaFederal/calculadora-rtc-gerador-danfse` | Go | `POST /api/danfse` (XML completo da NFS-e nacional → PDF) |

Quem lê o código é um recrutador ou tech lead. O código **próprio** (API e worker
em Go) é a vitrine; os motores são dependências.

## Objetivos

1. Fluxo ponta a ponta: **empresa → operação de serviço → cálculo → NFS-e completa
   montada e validada → DANFSe em PDF**.
2. Multi-tenant de verdade: cada empresa só vê os próprios dados (Supabase Auth + RLS).
3. Arquitetura visível: serviços separados, rede interna, fila, processamento
   assíncrono, health checks, tracing distribuído, CI.
4. `docker compose up` sobe tudo localmente; demo pública na VPS.
5. README que conta a arquitetura em 2 minutos (diagrama, decisões em ADRs).

## Não-objetivos

- **Não transmite** nada à Sefin Nacional / ADN e não usa certificado ICP-Brasil.
  Toda nota é gerada com `tpAmb=2` (homologação); o DANFSe sai marcado como
  "Ambiente de Homologação" / "SEM VALIDADE JURÍDICA".
- Sem cobrança, planos ou billing (o "SaaS" é o formato multi-tenant, não o comercial).
- Sem NF-e de produto com DANFE (não há motor de DANFE). Produtos (NCM) entram
  só no **simulador de cálculo**, sem documento.
- Não modifica os motores. Eles entram fixados num commit, sem fork.

## Anti-objetivos (falha mesmo funcionando)

- Um documento que possa passar por nota fiscal válida.
- Motores expostos à internet ou chamados direto pelo front.
- Vazamento de dados entre empresas.
- Um "CRUD com Docker" sem decisões de arquitetura explicadas.

## Restrições

- Os repos Java **não têm Dockerfile**: o rtc-hub fornece um Dockerfile
  multi-stage por motor, que clona o commit fixado, roda o Flyway para gerar o
  SQLite somente leitura e empacota o JAR com o `.db`.
- O README do Simples Nacional manda copiar uma pasta `flyway/` que **não existe no
  próprio repo** (o link aponta para ele mesmo). Provável origem: o repo do regime
  regular. Verificar no build.
- O `/xml/generate` da calculadora **não gera NFS-e** (`TipoDocumento.NFSE` tem
  `geracao=false`). O `/xml/validate?tipo=nfse&subtipo=nota` valida o elemento
  `<NFSe>` (`TCNFSe`), que exige `<ds:Signature>` e campos que na vida real a
  Sefin preenche na autorização (`nNFSe`, `Id`/chave de acesso, `xLocEmi`,
  `dhProc`...). Portanto **toda a montagem da NFS-e, incluindo o grupo IBSCBS,
  é código do rtc-hub, nos dois regimes**.
- A calculadora do Simples só calcula (não gera nem valida XML).
- A pasta `flyway/` que falta no Simples está no repo regular
  (`flyway/sql/manutencao/V0047__…`, `V0049__…`): confirmado pelo revisor.
- Conferir no build se os motores pedem internet ao subir (o Simples tem
  `VERSAO_URL` e `DB_DOWNLOAD_URL`); devem rodar 100% offline.
- As duas JVMs consomem RAM (ordem de 300–500 MB cada). A VPS já roda n8n,
  Evolution e multivozes no Easypanel: o deploy **não pode afetar** esses serviços.
  Limitar memória por container.
- Banco: **Supabase na nuvem** (plano grátis): Postgres, Auth, Storage, pgmq.

## Abordagens consideradas

| | Abordagem | Trade-off | Decisão |
|---|---|---|---|
| A | **Go (API + worker) + Next.js** | destaca mais para backend, imagens pequenas, combina com o motor DANFSe; duas linguagens | **escolhida pelo usuário** |
| B | TypeScript ponta a ponta (NestJS + Next.js) | mais rápido e mais vagas; portfólio comum | descartada |
| C | Python (FastAPI) | não ajuda nos objetivos | descartada |
| — | Java | usuário não gosta | descartada |

## Arquitetura

```
             internet
                │
          ┌─────▼─────┐        ┌──────────────── Supabase (cloud) ────────────────┐
          │  web      │──JWT──▶│ Auth                                             │
          │ (Next.js) │        └──────────────────────────────────────────────────┘
          └─────┬─────┘                 ▲ Postgres (RLS)  ▲ pgmq   ▲ Storage (PDFs)
                │ HTTPS + JWT           │                 │        │
          ┌─────▼─────┐   pgx           │                 │        │
          │ api (Go)  │─────────────────┴──── enfileira ──┘        │
          └─────┬─────┘                                            │
   rede interna │                      ┌──────────────┐            │
   ┌────────────┼──────────────┐       │ worker (Go)  │────────────┘
   ▼            ▼              │       └──────┬───────┘ consome pgmq
 calc-regular  calc-simples    │              ▼
 (Java)        (Java)          └──────▶ danfse (Go)
```

- **web** (Next.js, TypeScript): login via Supabase Auth, telas de empresa,
  simulador, emissão, histórico, download do PDF. Fala **só** com a `api`.
- **api** (Go): valida o JWT do Supabase (JWKS), resolve o tenant, escolhe o motor
  **de cálculo** pelo regime da empresa, monta a NFS-e, assina, valida, persiste
  e enfileira o PDF. A validação XSD usa sempre o motor regular (é validação de
  schema, igual para os dois regimes).
- **"Sefin simulada"** (pacote da api): cumpre o papel da autorização. Preenche
  `nNFSe`, chave de acesso, `dhProc`, `xLocEmi` etc. e assina a `<NFSe>` com
  XMLDSig usando um **certificado autoassinado de demo** (gerado no setup e
  guardado como secret; nunca um certificado real). Fica isolada num pacote
  próprio para deixar claro o que é simulação.
- **worker** (Go, mesmo módulo, outro binário): consome a fila pgmq, chama o
  `danfse`, sobe o PDF no Storage e atualiza o status da nota. Retry com backoff;
  falha definitiva fica em status `erro_pdf`.
- **motores**: só na rede interna do compose, sem porta publicada.

### Fluxos

1. **Simular cálculo** (síncrono): itens (NCM ou NBS) → motor do regime → resposta com
   memória de cálculo e erros por item (endpoint de observabilidade). Salva no histórico.
2. **Emitir NFS-e** (híbrido, regular e Simples): serviço + tomador → cálculo no
   motor do regime → monta a DPS/NFS-e → Sefin simulada assina → motor regular
   `xml/validate?tipo=nfse&subtipo=nota` → grava a nota `validada` → responde 202 → o worker
   gera o PDF → nota `pdf_pronto`. O front acompanha o status (polling ou Supabase Realtime).
3. **Dados abertos**: autocomplete de NBS/NCM/cClassTrib via api → motor, com cache na api.

### Dados (Postgres / Supabase)

- `empresas` (cnpj, razão social, regime `regular|simples`, município IBGE, UF, atividade do Simples)
- `membros` (user_id → empresa_id, papel `dono|membro`)
- `tomadores` (por empresa)
- `calculos` (empresa_id, motor, entrada jsonb, saída jsonb, criado_em)
- `notas` (empresa_id, numero sequencial por empresa, xml, status, pdf_path, calculo_id)
- fila pgmq `danfse_jobs`

RLS em todas as tabelas por `empresa_id ∈ empresas do auth.uid()`. A api acessa
via pgx e, em cada transação, aplica `set local role authenticated` +
`set_config('request.jwt.claims', …)`, de modo que o RLS vale também no backend
(defesa em profundidade, não só filtro no código). O worker usa um role de
serviço próprio, restrito às tabelas que precisa.

### Acesso à demo

- Cadastro aberto (Supabase Auth, e-mail). Quem cria a empresa vira `dono`;
  um usuário, uma empresa no MVP (sem convites). CNPJ validado pelo dígito.
- Botão **"Entrar como demo"**: cria uma sessão numa empresa pré-carregada
  (seed com tomadores, cálculos e notas), resetada periodicamente.
- Contra abuso: rate limit por usuário/IP na api, cota diária de emissões por
  empresa, e limpeza de PDFs com mais de N dias (Storage grátis é pequeno).

### Qualidade e operação

- Logs JSON estruturados, `request_id`; health checks `live`/`ready` em todos os serviços.
- OpenTelemetry: trace web→api→motor→worker (o DANFSe já suporta OTLP). Jaeger
  no compose local.
- Testes: unitários na montagem do XML e na escolha de motor; integração da api
  contra os motores reais via compose; teste de RLS (usuário A não lê dados de B).
- Teste da Sefin simulada: a nota assinada passa no `xml/validate` real do motor.
- CI (GitHub Actions): lint, testes, build das imagens; publica no GHCR.
- Repositório **público** no GitHub (exceção consciente à regra de repos
  privados, que vale para sites de clientes).
- Deploy: serviços criados no Easypanel (via UI, como o projeto "sistema"),
  num projeto separado de `maturare_leads`, puxando as imagens do GHCR, com
  limite de memória por serviço. Domínio e TLS definidos na fase de deploy.
- ADRs em `docs/adr/` para cada decisão relevante (pgmq vs Redis, RLS no backend,
  motores fixados por commit, etc.).

### Layout do repositório (monorepo)

```
api/            módulo Go: cmd/api, cmd/worker, internal/...
web/            Next.js
engines/        Dockerfiles + commit fixado de cada motor
supabase/       migrations SQL (Supabase CLI)
docs/adr/       decisões
docker-compose.yml
```

## Decisões tomadas pelo usuário

- Público: recrutadores de backend; foco em arquitetura e containers.
- Formato SaaS.
- Sistema web.
- Banco: Supabase.
- Stack: Go no backend + Next.js no front (abordagem A).
- NFS-e assinada com certificado autoassinado de demo (Sefin simulada).
- Demo com cadastro aberto + botão "entrar como demo".
- Repositório público.

## Decisões minhas (podem ser corrigidas)

- Escopo do documento: só NFS-e (o único motor de PDF é o DANFSe).
- pgmq em vez de Redis/RabbitMQ (menos uma peça, roda no próprio Supabase).
- RLS aplicado também via backend.
- Nome provisório `rtc-hub`.
- Supabase cloud em vez de self-hosted.
- Simples também emite NFS-e; validação XSD sempre no motor regular.
- Um usuário, uma empresa no MVP (sem convites).
- Deploy no Easypanel em projeto próprio; domínio decidido depois.

## Deixado para quem implementar

- Router HTTP (stdlib `net/http` 1.22+ ou chi), geração de queries (sqlc ou não).
- UI e componentes do Next.js.
- Polling vs Realtime para o status da nota.
- Estratégia de numeração da nota e formato exato do XML a partir do XSD da NFS-e nacional.
- Limites de memória dos containers Java.
