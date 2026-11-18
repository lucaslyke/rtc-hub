package httpapi

import (
	"context"
	"errors"
	"net/http"
	"strings"

	"github.com/MicahParks/keyfunc/v3"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

// JWKS busca (e mantém atualizadas) as chaves públicas do Supabase Auth.
func JWKS(ctx context.Context, supabaseURL string) (jwt.Keyfunc, error) {
	k, err := keyfunc.NewDefaultCtx(ctx, []string{strings.TrimRight(supabaseURL, "/") + "/auth/v1/.well-known/jwks.json"})
	if err != nil {
		return nil, err
	}
	return k.Keyfunc, nil
}

// usuario valida o Bearer (assinatura assimétrica, aud authenticated, exp) e devolve o sub.
func (s *Servidor) usuario(r *http.Request) (string, error) {
	tok, ok := strings.CutPrefix(r.Header.Get("Authorization"), "Bearer ")
	if !ok {
		return "", errors.New("sem token")
	}
	var c jwt.RegisteredClaims
	_, err := jwt.ParseWithClaims(tok, &c, s.Keyfunc,
		jwt.WithAudience("authenticated"), jwt.WithExpirationRequired(),
		jwt.WithValidMethods([]string{"ES256", "RS256", "EdDSA"}))
	if err != nil {
		return "", err
	}
	if uuid.Validate(c.Subject) != nil {
		return "", errors.New("sub inválido")
	}
	return c.Subject, nil
}
