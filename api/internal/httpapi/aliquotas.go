package httpapi

import (
	"net/http"

	"github.com/jackc/pgx/v5"

	"rtchub/internal/store"
)

func (s *Servidor) obterAliquotas(w http.ResponseWriter, r *http.Request, uid string) error {
	var a store.AliquotasEst
	err := s.tx(r, uid, func(tx pgx.Tx) (err error) {
		a, err = store.Aliquotas(r.Context(), tx)
		return
	})
	if err != nil {
		return err
	}
	responder(w, 200, a)
	return nil
}

func (s *Servidor) salvarAliquotas(w http.ResponseWriter, r *http.Request, uid string) error {
	var a store.AliquotasEst
	if err := lerJSON(r, &a); err != nil {
		return err
	}
	for _, v := range []float64{a.CBS, a.IBSUF, a.IBSMun, a.ISAdValorem} {
		if v < 0 || v > 100 {
			return falha(http.StatusBadRequest, "Alíquotas devem estar entre 0 e 100%.")
		}
	}
	if err := s.tx(r, uid, func(tx pgx.Tx) error { return store.SalvarAliquotas(r.Context(), tx, a) }); err != nil {
		return err
	}
	w.WriteHeader(http.StatusNoContent)
	return nil
}
