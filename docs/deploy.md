# Deploy

Produção: **web** em https://rtc-hub.zcqucd.easypanel.host e **API** em https://rtc-hub-api.zcqucd.easypanel.host.

## Supabase cloud

Projeto `rtc-hub` (região `sa-east-1`). Schema e demo vão por migration:

```sh
npx supabase link --project-ref <ref>
npx supabase db push        # aplica supabase/migrations (inclui pg_cron e o usuário-modelo da demo)
npx supabase config push    # login anônimo ligado, confirmação de e-mail desligada
```

O `config push` leva o `site_url` local; depois dele, ajuste em Authentication → URL Configuration
(ou pela Management API) o `site_url` para a URL pública do web.

Os tokens do Supabase precisam ser assimétricos (ES256), porque a API valida pelo JWKS.

## Easypanel

Projeto `rtc-hub`, separado de `maturare_leads`, com **um serviço Compose** `app`:

- Fonte: Git `https://github.com/lucaslyke/rtc-hub.git`, ref `main`, arquivo `docker-compose.yml`.
  O build dos motores (Maven) e do web roda na VPS; o primeiro leva alguns minutos.
- "Create .env" ligado, com as variáveis:

| Variável | Valor |
|---|---|
| `DATABASE_URL` | connection string do pooler de sessão do Supabase (porta 5432) |
| `SUPABASE_URL` | `https://<ref>.supabase.co` |
| `SUPABASE_JWT_ISSUER` | `https://<ref>.supabase.co/auth/v1` |
| `CORS_ORIGIN` | URL pública do web |
| `NEXT_PUBLIC_SUPABASE_URL` | igual a `SUPABASE_URL` |
| `NEXT_PUBLIC_SUPABASE_ANON_KEY` | chave *publishable* do projeto |
| `NEXT_PUBLIC_API_URL` | URL pública da API |

As `NEXT_PUBLIC_*` são embutidas no build do web: mudou, faça deploy de novo.

- Domínios (HTTPS, Let's Encrypt): web → serviço `web`, porta 3000; API → serviço `api`, porta 8090.
  Os motores ficam sem domínio, só na rede interna.

## Verificação

```sh
curl https://rtc-hub-api.zcqucd.easypanel.host/health/ready   # ok
```

Abrir o web e clicar em **Entrar como demo**. Depois, confirmar que os outros serviços da VPS
continuam no ar (`ssh maturare-vps docker ps`).
