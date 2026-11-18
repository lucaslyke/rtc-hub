package httpapi

import (
	"context"
	"encoding/json"
	"net/http"
	"os"
	"strings"
	"testing"
)

// Token real do Supabase local (login anônimo) validado pelas chaves do JWKS.
func TestJWKSDoSupabase(t *testing.T) {
	url, key := os.Getenv("SUPABASE_URL"), os.Getenv("SUPABASE_ANON_KEY")
	if url == "" || key == "" {
		t.Skip("SUPABASE_URL/SUPABASE_ANON_KEY não definidas")
	}
	req, _ := http.NewRequest("POST", url+"/auth/v1/signup", strings.NewReader(`{}`))
	req.Header.Set("apikey", key)
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	var sess struct {
		AccessToken string `json:"access_token"`
		User        struct{ ID string }
	}
	if err := json.NewDecoder(resp.Body).Decode(&sess); err != nil || sess.AccessToken == "" {
		t.Fatalf("login anônimo: %d %v", resp.StatusCode, err)
	}
	kf, err := JWKS(context.Background(), url)
	if err != nil {
		t.Fatal(err)
	}
	s := &Servidor{Keyfunc: kf}
	r, _ := http.NewRequest("GET", "/", nil)
	r.Header.Set("Authorization", "Bearer "+sess.AccessToken)
	uid, err := s.usuario(r)
	if err != nil || uid != sess.User.ID {
		t.Errorf("usuario = %q, %v; quer %q", uid, err, sess.User.ID)
	}
}
