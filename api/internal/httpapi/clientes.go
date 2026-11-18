package httpapi

import (
	"errors"
	"net/http"
	"regexp"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"rtchub/internal/store"
)

var (
	reUF     = regexp.MustCompile(`^[A-Z]{2}$`)
	reCAtiv  = regexp.MustCompile(`^\d{2}$`)
	reDigito = regexp.MustCompile(`^\d+$`)
)

// lerCliente decodifica e valida o corpo; o CNPJ aceita pontuação.
func lerCliente(r *http.Request) (store.Cliente, error) {
	var c store.Cliente
	if err := lerJSON(r, &c); err != nil {
		return c, err
	}
	c.CNPJ = strings.Map(func(r rune) rune { // aceita 11.222.333/0001-81
		if strings.ContainsRune("./- ", r) {
			return -1
		}
		return r
	}, c.CNPJ)
	ruim := func(msg string) (store.Cliente, error) { return c, falha(http.StatusBadRequest, msg) }
	switch {
	case c.RazaoSocial == "":
		return ruim("Informe a razão social.")
	case len(c.CNPJ) != 14 || !reDigito.MatchString(c.CNPJ):
		return ruim("CNPJ deve ter 14 dígitos.")
	case c.Regime != "regular" && c.Regime != "simples":
		return ruim("Regime deve ser regular ou simples.")
	case !reUF.MatchString(c.UF):
		return ruim("UF inválida.")
	case c.MunicipioIBGE < 1000000 || c.MunicipioIBGE > 9999999:
		return ruim("Código IBGE do município deve ter 7 dígitos.")
	case c.Regime == "simples" && (!reCAtiv.MatchString(c.CAtivSN) || c.PIBSSN == nil || c.PCBSSN == nil):
		return ruim("Para o Simples, informe a atividade (cAtivSN) e as alíquotas efetivas de IBS e CBS.")
	}
	if c.Regime == "regular" {
		c.CAtivSN, c.PIBSSN, c.PCBSSN = "", nil, nil
	}
	return c, nil
}

func unico(err error) bool {
	var pe *pgconn.PgError
	return errors.As(err, &pe) && pe.Code == "23505"
}

func cnpjDuplicado(err error) error {
	if unico(err) {
		return falha(http.StatusConflict, "Já existe um cliente com este CNPJ.")
	}
	return err
}

type clienteComContagem struct {
	store.Cliente
	Contagem map[string]int `json:"contagem"`
}

func (s *Servidor) listarClientes(w http.ResponseWriter, r *http.Request, uid string) error {
	cs := []clienteComContagem{}
	err := s.tx(r, uid, func(tx pgx.Tx) error {
		l, err := store.ListarClientes(r.Context(), tx)
		if err != nil {
			return err
		}
		cont, err := store.ContagemPorSituacao(r.Context(), tx)
		for _, c := range l {
			n := map[string]int{"ok": 0, "provavel": 0, "revisar": 0, "invalido": 0}
			for sit, q := range cont[c.ID] {
				n[sit] = q
			}
			cs = append(cs, clienteComContagem{c, n})
		}
		return err
	})
	if err != nil {
		return err
	}
	responder(w, 200, cs)
	return nil
}

func (s *Servidor) criarCliente(w http.ResponseWriter, r *http.Request, uid string) error {
	c, err := lerCliente(r)
	if err != nil {
		return err
	}
	err = s.tx(r, uid, func(tx pgx.Tx) error {
		c, err = store.CriarCliente(r.Context(), tx, c)
		return err
	})
	if err != nil {
		return cnpjDuplicado(err)
	}
	responder(w, http.StatusCreated, c)
	return nil
}

func (s *Servidor) obterCliente(w http.ResponseWriter, r *http.Request, uid string) error {
	cid, err := id(r)
	if err != nil {
		return err
	}
	var c store.Cliente
	err = s.tx(r, uid, func(tx pgx.Tx) error {
		c, err = store.ObterCliente(r.Context(), tx, cid)
		return err
	})
	if err != nil {
		return err
	}
	responder(w, 200, c)
	return nil
}

func (s *Servidor) atualizarCliente(w http.ResponseWriter, r *http.Request, uid string) error {
	cid, err := id(r)
	if err != nil {
		return err
	}
	c, err := lerCliente(r)
	if err != nil {
		return err
	}
	c.ID = cid
	err = s.tx(r, uid, func(tx pgx.Tx) error {
		c, err = store.AtualizarCliente(r.Context(), tx, c)
		return err
	})
	if err != nil {
		return cnpjDuplicado(err)
	}
	responder(w, 200, c)
	return nil
}

func (s *Servidor) excluirCliente(w http.ResponseWriter, r *http.Request, uid string) error {
	cid, err := id(r)
	if err != nil {
		return err
	}
	err = s.tx(r, uid, func(tx pgx.Tx) error {
		ok, err := store.ExcluirCliente(r.Context(), tx, cid)
		if err == nil && !ok {
			return naoEncontrado
		}
		return err
	})
	if err != nil {
		return err
	}
	w.WriteHeader(http.StatusNoContent)
	return nil
}
