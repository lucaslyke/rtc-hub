// Comando api: servidor HTTP do rtc-hub.
package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"rtchub/internal/httpapi"
	"rtchub/internal/motores"
	"rtchub/internal/store"
	"rtchub/internal/tabelas"
)

func env(k, padrao string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	if padrao == "" {
		log.Fatalf("variável %s obrigatória", k)
	}
	return padrao
}

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	tab, err := tabelas.Abrir(env("TABELAS_DB", "/tabelas/calculadora-pro.db"))
	if err != nil {
		log.Fatalf("tabelas: %v", err)
	}
	defer tab.Close()
	st, err := store.Conectar(ctx, env("DATABASE_URL", ""))
	if err != nil {
		log.Fatalf("banco: %v", err)
	}
	defer st.Close()
	supa := env("SUPABASE_URL", "")
	kf, err := httpapi.JWKS(ctx, supa)
	if err != nil {
		log.Fatalf("jwks: %v", err)
	}
	regular, simples := env("CALC_REGULAR_URL", "http://calc-regular:8080"), env("CALC_SIMPLES_URL", "http://calc-simples:8082")
	s := &httpapi.Servidor{
		Store: st, Tabelas: tab, Motores: motores.Novo(regular, simples, nil), Keyfunc: kf,
		// a api pode falar com o Supabase por outro host (host.docker.internal) que não o do iss dos tokens
		Issuer:     env("SUPABASE_JWT_ISSUER", httpapi.Emissor(supa)),
		CORSOrigin: os.Getenv("CORS_ORIGIN"),
		Sondas:     []string{regular + "/api/swagger-ui.html", simples + "/api/versao"},
	}
	srv := &http.Server{Addr: ":" + env("PORT", "8090"), Handler: s.Rotas(), ReadHeaderTimeout: 10 * time.Second}
	go func() {
		<-ctx.Done()
		sh, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		srv.Shutdown(sh)
	}()
	log.Printf("api ouvindo em %s", srv.Addr)
	if err := srv.ListenAndServe(); err != http.ErrServerClosed {
		log.Fatal(err)
	}
}
