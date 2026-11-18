package httpapi

import (
	"net/http"
	"regexp"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"rtchub/internal/store"
	"rtchub/internal/tabelas"
)

var (
	reCST        = regexp.MustCompile(`^\d{3}$`)
	reCClassTrib = regexp.MustCompile(`^\d{6}$`)
)

func (s *Servidor) listarItens(w http.ResponseWriter, r *http.Request, uid string) error {
	cid, err := id(r)
	if err != nil {
		return err
	}
	filtro := r.URL.Query().Get("situacao")
	contagem := map[string]int{"ok": 0, "provavel": 0, "revisar": 0, "invalido": 0}
	if _, ok := contagem[filtro]; filtro != "" && !ok {
		return falha(http.StatusBadRequest, "Situação inválida.")
	}
	itens := []store.Item{}
	err = s.tx(r, uid, func(tx pgx.Tx) error {
		if _, err := store.ObterCliente(r.Context(), tx, cid); err != nil {
			return err
		}
		todos, err := store.ListarItens(r.Context(), tx, cid)
		for _, i := range todos {
			contagem[i.Situacao]++
			if filtro == "" || i.Situacao == filtro {
				itens = append(itens, i)
			}
		}
		return err
	})
	if err != nil {
		return err
	}
	responder(w, 200, map[string]any{"itens": itens, "contagem": contagem})
	return nil
}

func (s *Servidor) atualizarItem(w http.ResponseWriter, r *http.Request, uid string) error {
	iid, err := id(r)
	if err != nil {
		return err
	}
	var in struct {
		CST          string `json:"cst"`
		CClassTrib   string `json:"cClassTrib"`
		ISCST        string `json:"isCst"`
		ISCClassTrib string `json:"isCClassTrib"`
		NBS          string `json:"nbs"` // só vale para serviço que veio sem NCM e sem NBS
	}
	if err := lerJSON(r, &in); err != nil {
		return err
	}
	isVazio := in.ISCST == "" && in.ISCClassTrib == ""
	if !reCST.MatchString(in.CST) || !reCClassTrib.MatchString(in.CClassTrib) ||
		!isVazio && (!reCST.MatchString(in.ISCST) || !reCClassTrib.MatchString(in.ISCClassTrib)) {
		return falha(http.StatusBadRequest, "CST deve ter 3 dígitos e cClassTrib 6 dígitos.")
	}
	in.NBS = tabelas.NormalizaCodigo(in.NBS)
	if in.NBS != "" && (len(in.NBS) != 9 || !reDigito.MatchString(in.NBS)) {
		return falha(http.StatusBadRequest, "NBS deve ter 9 dígitos.")
	}
	var it store.Item
	err = s.tx(r, uid, func(tx pgx.Tx) error {
		if err := store.AtualizarItem(r.Context(), tx, iid, in.CST, in.CClassTrib, in.ISCST, in.ISCClassTrib); err != nil {
			return err
		}
		if in.NBS != "" {
			if err := store.DefinirNBS(r.Context(), tx, iid, in.NBS); err != nil {
				if unico(err) {
					return falha(http.StatusConflict, "Já existe um item com esse código e essa NBS neste cliente.")
				}
				return err
			}
		}
		it, err = store.ObterItem(r.Context(), tx, iid)
		return err
	})
	if err != nil {
		return err
	}
	responder(w, 200, it)
	return nil
}

func (s *Servidor) confirmar(w http.ResponseWriter, r *http.Request, uid string) error {
	cid, err := id(r)
	if err != nil {
		return err
	}
	var in struct {
		IDs []string `json:"ids"`
	}
	if err := lerJSON(r, &in); err != nil {
		return err
	}
	if len(in.IDs) == 0 {
		return falha(http.StatusBadRequest, "Selecione ao menos um item.")
	}
	for _, i := range in.IDs {
		if uuid.Validate(i) != nil {
			return falha(http.StatusBadRequest, "Item inválido.")
		}
	}
	var n int64
	err = s.tx(r, uid, func(tx pgx.Tx) (err error) {
		n, err = store.ConfirmarItens(r.Context(), tx, cid, in.IDs)
		return
	})
	if err != nil {
		return err
	}
	responder(w, 200, map[string]int64{"confirmados": n})
	return nil
}
