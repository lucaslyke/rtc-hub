// Package motores chama os motores de cálculo da Receita (regime regular e Simples Nacional).
package motores

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

type Aliquotas struct{ CBS, IBSUF, IBSMun, ISAdValorem float64 }

type ClienteCalc struct {
	Regime                  string
	UF                      string
	MunicipioIBGE           int
	CAtivSN, PIBSSN, PCBSSN string
}

type ItemCalc struct {
	ID, NCM, NBS, CST, CClassTrib, ISCST, ISCClassTrib string
	Valor                                              float64
}

type Resultado struct {
	ItemID                            string
	Estado                            string // do motor, ou "NAO_SUPORTADO"
	Simulacao                         bool
	VCBS, VIBS, VIS, PRedCBS, PRedIBS string
	Mensagem                          string
	Bruto                             json.RawMessage
}

type Motores struct {
	regular, simples string
	hc               *http.Client
}

func Novo(urlRegular, urlSimples string, hc *http.Client) *Motores {
	if hc == nil {
		hc = &http.Client{Timeout: 60 * time.Second}
	}
	return &Motores{urlRegular, urlSimples, hc}
}

func (m *Motores) post(ctx context.Context, url string, req any) ([]byte, error) {
	b, err := json.Marshal(req)
	if err != nil {
		return nil, err
	}
	r, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(b))
	if err != nil {
		return nil, err
	}
	r.Header.Set("Content-Type", "application/json")
	resp, err := m.hc.Do(r)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	out, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode >= 300 {
		return nil, fmt.Errorf("motor %s: HTTP %d: %.300s", url, resp.StatusCode, out)
	}
	return out, nil
}
