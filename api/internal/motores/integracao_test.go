//go:build integracao

package motores

import (
	"context"
	"testing"
)

func TestMotoresReais(t *testing.T) {
	m := Novo("http://127.0.0.1:8080", "http://127.0.0.1:8082", nil)
	r, err := m.Calcular(context.Background(), 2027,
		ClienteCalc{Regime: "regular", UF: "RS", MunicipioIBGE: 4314902},
		Aliquotas{CBS: 8.8, IBSUF: 0.05, IBSMun: 0.05},
		[]ItemCalc{{ID: "a", NCM: "10063021", CST: "200", CClassTrib: "200003", Valor: 100}})
	if err != nil {
		t.Fatal(err)
	}
	if r[0].PRedCBS != "100.00" {
		t.Fatalf("%+v", r[0])
	}
}
