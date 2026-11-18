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
		atual, err := store.ObterItem(r.Context(), tx, iid)
		if err != nil {
			return err
		}
		tipo, codigo := tabelas.NCM, atual.NCM
		switch {
		case atual.NCM != "":
		case atual.NBS != "":
			tipo, codigo = tabelas.NBS, atual.NBS
		default: // serviço sem NBS: vale a NBS escolhida agora, se houver
			tipo, codigo = tabelas.NBS, in.NBS
		}
		if err := s.validarClassificacao(r, tipo, codigo, in.CST, in.CClassTrib, false); err != nil {
			return err
		}
		if !isVazio {
			if err := s.validarClassificacao(r, tipo, codigo, in.ISCST, in.ISCClassTrib, true); err != nil {
				return err
			}
		}
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

// validarClassificacao: o par CST/cClassTrib existe, está vigente e vale para o NCM/NBS do item.
func (s *Servidor) validarClassificacao(r *http.Request, tipo tabelas.Tipo, codigo, cst, cClassTrib string, is bool) error {
	data := dataSugestao()
	o, err := s.Tabelas.Classificacao(r.Context(), cClassTrib, data, is)
	if err != nil {
		return err
	}
	if o == nil {
		return falha(http.StatusBadRequest, "Classificação inexistente ou fora de vigência: "+cClassTrib+".")
	}
	if o.CST != cst {
		return falha(http.StatusBadRequest, "A classificação "+cClassTrib+" é do CST "+o.CST+", não do "+cst+".")
	}
	if codigo == "" {
		return nil
	}
	ok, err := s.Tabelas.ValeParaCodigo(r.Context(), tipo, codigo, cClassTrib, data, is)
	if err != nil {
		return err
	}
	if !ok {
		return falha(http.StatusBadRequest, "Essa classificação não vale para o "+string(tipo)+" "+codigo+" na tabela da Receita.")
	}
	return nil
}
