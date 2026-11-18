package httpapi

import (
	"bytes"
	"encoding/json"
	"log"
	"net/http"
	"strconv"

	"github.com/jackc/pgx/v5"

	"rtchub/internal/exporta"
	"rtchub/internal/motores"
	"rtchub/internal/store"
)

const motoresFora = "Os motores de cálculo da Receita não responderam. Tente de novo em instantes."

func pct(p *float64) string {
	if p == nil {
		return ""
	}
	return strconv.FormatFloat(*p, 'f', 2, 64)
}

// calcular manda aos motores só os itens 'ok'; os outros voltam em pendentes.
func (s *Servidor) calcular(w http.ResponseWriter, r *http.Request, uid string) error {
	cid, err := id(r)
	if err != nil {
		return err
	}
	var in struct {
		Ano int `json:"ano"`
	}
	if err := lerJSON(r, &in); err != nil {
		return err
	}
	if in.Ano < 2026 || in.Ano > 2033 {
		return falha(http.StatusBadRequest, "O ano deve estar entre 2026 e 2033.")
	}
	var cli store.Cliente
	var al store.AliquotasEst
	var prontos []motores.ItemCalc
	pendentes := []store.Item{}
	err = s.tx(r, uid, func(tx pgx.Tx) error {
		if cli, err = store.ObterCliente(r.Context(), tx, cid); err != nil {
			return err
		}
		if al, err = store.Aliquotas(r.Context(), tx); err != nil {
			return err
		}
		its, err := store.ListarItens(r.Context(), tx, cid)
		for _, i := range its {
			if i.Situacao != "ok" {
				pendentes = append(pendentes, i)
				continue
			}
			prontos = append(prontos, motores.ItemCalc{ID: i.ID, NCM: i.NCM, NBS: i.NBS, CST: i.CST, CClassTrib: i.CClassTrib,
				ISCST: i.ISCST, ISCClassTrib: i.ISCClassTrib, Valor: i.UltimoValor})
		}
		return err
	})
	if err != nil {
		return err
	}

	rs := []motores.Resultado{}
	if len(prontos) > 0 {
		// fora da transação: o motor pode levar segundos
		rs, err = s.Motores.Calcular(r.Context(), in.Ano,
			motores.ClienteCalc{Regime: cli.Regime, UF: cli.UF, MunicipioIBGE: cli.MunicipioIBGE, CAtivSN: cli.CAtivSN, PIBSSN: pct(cli.PIBSSN), PCBSSN: pct(cli.PCBSSN)},
			motores.Aliquotas(al), prontos)
		if err != nil {
			log.Printf("motores (cliente %s, ano %d): %v", cid, in.Ano, err)
			return falha(http.StatusBadGateway, motoresFora)
		}
		ano := strconv.Itoa(in.Ano)
		err = s.tx(r, uid, func(tx pgx.Tx) error {
			for _, res := range rs {
				b, err := json.Marshal(res)
				if err != nil {
					return err
				}
				if err := store.SalvarCalculo(r.Context(), tx, res.ItemID, ano, b); err != nil {
					return err
				}
			}
			return nil
		})
		if err != nil {
			return err
		}
		for i := range rs {
			rs[i].Bruto = nil // a resposta crua do motor fica só no banco
		}
	}
	responder(w, 200, map[string]any{"resultados": rs, "pendentes": pendentes})
	return nil
}

func (s *Servidor) exportar(w http.ResponseWriter, r *http.Request, uid string) error {
	cid, err := id(r)
	if err != nil {
		return err
	}
	ano, err := strconv.Atoi(r.URL.Query().Get("ano"))
	if err != nil || ano < 2026 || ano > 2033 {
		return falha(http.StatusBadRequest, "Informe um ano entre 2026 e 2033.")
	}
	var cli store.Cliente
	var al store.AliquotasEst
	var its []store.Item
	err = s.tx(r, uid, func(tx pgx.Tx) (err error) {
		if cli, err = store.ObterCliente(r.Context(), tx, cid); err != nil {
			return err
		}
		if al, err = store.Aliquotas(r.Context(), tx); err != nil {
			return err
		}
		its, err = store.ListarItens(r.Context(), tx, cid)
		return
	})
	if err != nil {
		return err
	}
	var buf bytes.Buffer
	if err := exporta.Cadastro(&buf, cli, its, ano, motores.Aliquotas(al)); err != nil {
		return err
	}
	w.Header().Set("Content-Type", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")
	w.Header().Set("Content-Disposition", `attachment; filename="cadastro-`+cli.CNPJ+"-"+strconv.Itoa(ano)+`.xlsx"`)
	_, err = w.Write(buf.Bytes())
	return err
}
