# Motores da Receita

`java.Dockerfile` empacota os dois motores a partir de commits fixos (ARGs `REPO`/`COMMIT` no `docker-compose.yml`):

- `calc-regular` (regime-regular@150cccb), porta 8080, `/api/calculadora`
- `calc-simples` (simples-nacional@c5b0f01), porta 8082, `/api/calculadora`

O Simples não tem `flyway/`; o build usa o do regular@150cccb. O SQLite `calculadora-pro.db` é gerado no build e fica no volume `tabelas` (compartilhado). Ao trocar o COMMIT: `docker volume rm rtc-hub_tabelas`.

Os dois sobem offline (perfil `offline`); nenhuma variável de rede precisou ser anulada (logs sem VERSAO_URL/DB_DOWNLOAD/UnknownHost).

Dev local: `docker compose -f docker-compose.yml -f docker-compose.dev.yml up -d` publica 127.0.0.1:8080 e :8082. `scripts/extrair-tabelas.sh` copia o DB para `.data/`.
