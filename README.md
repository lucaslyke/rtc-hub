# RTC Hub

Classificação tributária da reforma (IBS, CBS e Imposto Seletivo) para escritórios de contabilidade, com base nas tabelas e nos motores de cálculo oficiais da Receita Federal.

**(link da demo: em breve)** — na tela de entrada, clique em **Entrar como demo**.

![Demonstração: entrar como demo, revisar um item, confirmar os prováveis e calcular 2027](docs/demo.gif)

## O problema

Com a reforma tributária (EC 132/2023 e LC 214/2025), cada produto e cada serviço que o cliente vende passa a precisar, na nota fiscal, do **CST** e da **classificação tributária (cClassTrib)** do IBS e da CBS, e também do Imposto Seletivo quando ele incide (bebidas açucaradas, cervejas, cigarros...).

Ou seja: o cadastro de produtos de **cada cliente** do escritório tem que ser reclassificado. Uma padaria tem dezenas de itens, uma farmácia tem centenas, e a resposta certa depende de anexos da lei (cesta básica, redução de 60%, higiene pessoal, educação...). Fazer isso item por item, consultando as tabelas da Receita, é lento e sujeito a erro.

## O que o sistema faz

- **Lê o que o cliente já tem**: os XMLs das notas (NF-e, NFC-e, NFS-e nacional, soltos ou em ZIP) ou a planilha de produtos (CSV ou XLSX). Cada item aparece uma vez só, com quantas vezes saiu nas notas e o último valor.
- **Sugere a classificação** de cada item pelo NCM (produtos) ou pela NBS (serviços), usando as tabelas oficiais da Receita, e mostra o porquê: o artigo da lei, o anexo e o percentual de redução.
- **Separa o que precisa da sua atenção**:
  - ✓ **Pronto** — nenhum tratamento especial para esse NCM/NBS nas tabelas oficiais (tributação integral), ou classificação já confirmada por você.
  - ? **Provável** — há uma classificação especial que provavelmente se aplica; você confirma em lote;
  - ⚠ **Revisar** — há mais de uma possibilidade; você escolhe na lista;
  - ✗ **Inválido** — NCM/NBS inexistente ou incompleto.
- **Calcula** CBS, IBS e Imposto Seletivo de cada item nos motores oficiais da Receita, para qualquer ano de 2026 a 2033, no regime regular ou no Simples Nacional.
- **Exporta** o cadastro classificado em XLSX, para conferir com o cliente e atualizar o cadastro no sistema emissor.

## Como funciona

1. O escritório cadastra o cliente (CNPJ, regime, UF e município) e importa as notas ou a planilha.
2. Para cada NCM/NBS, o sistema consulta as tabelas oficiais da Receita e lista as classificações possíveis.
3. Se só uma serve, o item fica pronto; se há uma especial, fica como provável; se há várias, vai para revisão.
4. O contador confirma ou escolhe; nada é dado como certo sem ele.
5. Os itens prontos vão para os motores de cálculo oficiais da Receita, que devolvem os valores de CBS, IBS e IS.

## Experimente a demo

O botão **Entrar como demo** cria uma sessão anônima, sem cadastro, com um escritório de exemplo e três clientes já importados e classificados:

| Cliente | Regime | O que mostra |
|---|---|---|
| Padaria Pão Quente | Simples Nacional | cesta básica com redução de 100%, Imposto Seletivo identificado em refrigerante, cerveja e cigarro (não incide na revenda; o contador decide) |
| Farmácia Saúde | Regular (SP) | medicamentos e dispositivos médicos para revisar, higiene pessoal com redução, cosméticos com tributação integral |
| Escola Aprender | Regular (SP) | serviços de educação por NBS (redução de 60%), uma NFS-e sem NBS para revisar, livros e uniformes |

Os dados são fictícios. Cada sessão de demo é independente e é apagada depois de 7 dias.

**Importante:** os valores de 2027 em diante são **simulações** com alíquotas estimadas (ajustáveis em *Alíquotas*), porque as alíquotas de referência ainda não foram fixadas. Em 2026 o cálculo usa as alíquotas de teste da Receita. As sugestões dependem do vínculo entre NCM/NBS e classificação nas tabelas oficiais; benefícios que dependem de outra condição (por exemplo, medicamento registrado na Anvisa) precisam da conferência do contador.

---

## Parte técnica

### Arquitetura

```mermaid
flowchart LR
  N[Navegador<br/>Next.js] -- "login e demo<br/>(RPC clonar_demo)" --> S[(Supabase<br/>Auth + Postgres com RLS)]
  N -- "JWT" --> A[api Go]
  A -- "consultas como o usuário (RLS)" --> S
  A -- "sugestão" --> T[(SQLite com as<br/>tabelas oficiais)]
  A -- "cálculo" --> R[motor regime regular]
  A -- "cálculo" --> SN[motor Simples Nacional]
  R --- T
  SN --- T
```

- `web/` — Next.js (React, Tailwind). Fala com o Supabase só para autenticação e com a api para o resto.
- `api/` — Go. Valida o JWT do Supabase, roda cada consulta no Postgres como o usuário (as políticas de RLS isolam os escritórios), sugere a classificação lendo o SQLite oficial e chama os motores.
- `engines/` — os motores oficiais da Receita Federal, empacotados em containers a partir de commits fixos (veja `engines/README.md`):
  - [calculadora-rtc-regime-regular](https://github.com/ReceitaFederal/calculadora-rtc-regime-regular) (licença MIT)
  - [calculadora-rtc-simples-nacional](https://github.com/ReceitaFederal/calculadora-rtc-simples-nacional) (licença MIT)
- `supabase/migrations/` — schema com RLS e a demo (`clonar_demo()`, usuário-modelo e limpeza diária via `pg_cron`).

### Rodar localmente

Pré-requisitos: Docker, Node 24, Go 1.27 e a [CLI do Supabase](https://supabase.com/docs/guides/local-development) (`npx supabase`).

```sh
npx supabase start              # Postgres, Auth etc. em 127.0.0.1:54321 (DB em :54322); aplica as migrations
npx supabase status -o env      # mostra a ANON_KEY
```

O build da imagem `web` embute as variáveis públicas, então elas precisam estar no shell ou num `.env` na raiz do repositório:

```sh
# .env (raiz)
NEXT_PUBLIC_SUPABASE_URL=http://127.0.0.1:54321
NEXT_PUBLIC_SUPABASE_ANON_KEY=<ANON_KEY do supabase status>
NEXT_PUBLIC_API_URL=http://localhost:8090
```

```sh
docker compose -f docker-compose.yml -f docker-compose.dev.yml up -d --build
```

O primeiro build dos motores demora (baixa e compila os projetos da Receita). Depois: web em http://localhost:3000, api em http://localhost:8090 (`/health/ready` confere tabelas e motores).

Variáveis da api (o `docker-compose.dev.yml` já preenche para o Supabase local):

| Variável | Uso |
|---|---|
| `DATABASE_URL` | Postgres do Supabase (obrigatória) |
| `SUPABASE_URL` | de onde a api busca as chaves públicas (JWKS) para validar o login (obrigatória) |
| `TABELAS_DB` | caminho do SQLite oficial (padrão `/tabelas/calculadora-pro.db`, volume compartilhado com os motores) |
| `CALC_REGULAR_URL`, `CALC_SIMPLES_URL` | endereços dos motores |
| `CORS_ORIGIN` | origem do web, ex. `http://localhost:3000` |
| `PORT` | porta da api (padrão 8090) |

Para desenvolver o web fora do Docker: `cp web/.env.example web/.env.local`, preencha a chave e rode `npm run dev` em `web/`.

### Testes

```sh
cd api && go test ./...         # sem ambiente, os testes de SQLite e de Supabase são pulados
```

Suíte completa da api, com o Supabase local e os motores no ar:

```sh
scripts/extrair-tabelas.sh      # copia o SQLite oficial para .data/
cd api && SUPABASE_DB_URL=postgresql://postgres:postgres@127.0.0.1:54322/postgres \
  SUPABASE_URL=http://127.0.0.1:54321 SUPABASE_ANON_KEY=<ANON_KEY> go test ./...
go test -tags integracao ./internal/motores   # chama os motores reais em :8080 e :8082
```

Web: `npm run lint` e `npx playwright test` em `web/` (end-to-end; precisa de Supabase, api e motores no ar).

### Dados da demo

A semente fica no fim de `supabase/migrations/20261118000002_demo.sql`. Ela é gerada passando XMLs fictícios pela própria api, para refletir o motor de sugestão de verdade:

```sh
SUPABASE_ANON_KEY=<ANON_KEY> API_URL=http://localhost:8090 go run scripts/gerar-semente-demo.go
npx supabase db reset           # reaplica as migrations com a semente nova
```

### CI

`.github/workflows/ci.yml`: testes Go, lint e build do web, build das imagens `api` e `web`. Na branch `main` as imagens são publicadas no GitHub Container Registry (`ghcr.io/<dono>/<repo>-api` e `-web`). Para a imagem `web` publicada apontar para o Supabase e a api de produção, defina as *variables* `NEXT_PUBLIC_SUPABASE_URL`, `NEXT_PUBLIC_SUPABASE_ANON_KEY` e `NEXT_PUBLIC_API_URL` no repositório.
