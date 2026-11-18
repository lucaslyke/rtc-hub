// Package httpapi expõe a api JSON do rtc-hub. Toda rota (exceto health) exige o JWT do
// Supabase e roda no banco como o usuário dele (RLS).
package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"rtchub/internal/motores"
	"rtchub/internal/store"
	"rtchub/internal/tabelas"
)

type Servidor struct {
	Store      *store.Store
	Tabelas    *tabelas.Tabelas
	Motores    *motores.Motores
	Keyfunc    jwt.Keyfunc
	Issuer     string // iss exigido no JWT
	CORSOrigin string
	Sondas     []string // URLs dos motores que /health/ready consulta
}

// erroHTTP é um erro com mensagem para o usuário (pt-BR).
type erroHTTP struct {
	status int
	msg    string
}

func (e erroHTTP) Error() string { return e.msg }

func falha(status int, msg string) error { return erroHTTP{status, msg} }

var naoEncontrado = falha(http.StatusNotFound, "Não encontrado.")

type handler func(w http.ResponseWriter, r *http.Request, uid string) error

func (s *Servidor) Rotas() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /health/live", func(w http.ResponseWriter, r *http.Request) { responder(w, 200, map[string]string{"status": "ok"}) })
	mux.HandleFunc("GET /health/ready", s.pronto)
	rotas := map[string]handler{
		"GET /clientes":                       s.listarClientes,
		"POST /clientes":                      s.criarCliente,
		"GET /clientes/{id}":                  s.obterCliente,
		"PUT /clientes/{id}":                  s.atualizarCliente,
		"DELETE /clientes/{id}":               s.excluirCliente,
		"POST /clientes/{id}/importar":        s.importar,
		"POST /clientes/{id}/planilha/previa": s.previa,
		"GET /clientes/{id}/itens":            s.listarItens,
		"PATCH /itens/{id}":                   s.atualizarItem,
		"POST /clientes/{id}/itens/confirmar": s.confirmar,
		"POST /clientes/{id}/calcular":        s.calcular,
		"GET /clientes/{id}/exportar.xlsx":    s.exportar,
		"GET /aliquotas":                      s.obterAliquotas,
		"PUT /aliquotas":                      s.salvarAliquotas,
	}
	for padrao, h := range rotas {
		mux.Handle(padrao, s.autenticado(h))
	}
	return s.cors(mux)
}

func (s *Servidor) cors(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if s.CORSOrigin != "" {
			w.Header().Set("Access-Control-Allow-Origin", s.CORSOrigin)
			w.Header().Set("Vary", "Origin")
			if r.Method == http.MethodOptions {
				w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE")
				w.Header().Set("Access-Control-Allow-Headers", "Authorization, Content-Type")
				w.Header().Set("Access-Control-Max-Age", "600")
				w.WriteHeader(http.StatusNoContent)
				return
			}
		}
		h.ServeHTTP(w, r)
	})
}

func (s *Servidor) autenticado(h handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		uid, err := s.usuario(r)
		if err != nil {
			responder(w, http.StatusUnauthorized, map[string]string{"erro": "Sessão inválida ou expirada. Entre de novo."})
			return
		}
		if err := h(w, r, uid); err != nil {
			var e erroHTTP
			switch {
			case errors.As(err, &e):
			case errors.Is(err, pgx.ErrNoRows):
				e = naoEncontrado.(erroHTTP)
			default:
				log.Printf("%s %s: %v", r.Method, r.URL.Path, err)
				e = erroHTTP{http.StatusInternalServerError, "Erro interno. Tente de novo."}
			}
			responder(w, e.status, map[string]string{"erro": e.msg})
		}
	})
}

func (s *Servidor) pronto(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()
	if _, err := s.Tabelas.Sugerir(ctx, tabelas.NCM, "10063021", dataSugestao()); err != nil {
		log.Printf("ready: tabelas: %v", err)
		responder(w, 503, map[string]string{"status": "tabelas indisponíveis"})
		return
	}
	for _, u := range s.Sondas {
		req, _ := http.NewRequestWithContext(ctx, "GET", u, nil)
		resp, err := http.DefaultClient.Do(req)
		if err == nil {
			resp.Body.Close()
		}
		if err != nil || resp.StatusCode >= 500 {
			log.Printf("ready: motor %s: %v", u, err)
			responder(w, 503, map[string]string{"status": "motores indisponíveis"})
			return
		}
	}
	responder(w, 200, map[string]string{"status": "ok"})
}

func responder(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

func lerJSON(r *http.Request, v any) error {
	if err := json.NewDecoder(http.MaxBytesReader(nil, r.Body, 1<<20)).Decode(v); err != nil {
		return falha(http.StatusBadRequest, "JSON inválido.")
	}
	return nil
}

// id lê o {id} da rota; um id que não é UUID nunca existe.
func id(r *http.Request) (string, error) {
	v := r.PathValue("id")
	if uuid.Validate(v) != nil {
		return "", naoEncontrado
	}
	return v, nil
}

func (s *Servidor) tx(r *http.Request, uid string, fn func(pgx.Tx) error) error {
	return s.Store.ComoUsuario(r.Context(), uid, fn)
}

// dataSugestao: max(hoje, 2027-01-01) — o IS e a classificação valem a partir de 2027.
func dataSugestao() string {
	return max(time.Now().Format(time.DateOnly), "2027-01-01")
}
