package motores

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
)

const lote = 100

type resp struct {
	Objetos []json.RawMessage `json:"objetos"`
}

type objeto struct {
	NObj       int             `json:"nObj"`
	EstadoItem string          `json:"estadoItem"`
	TribCalc   json.RawMessage `json:"tribCalc"`
	Erros      []struct {
		Title  string `json:"title"`
		Detail string `json:"detail"`
	} `json:"erros"`
}

func (m *Motores) Calcular(ctx context.Context, ano int, c ClienteCalc, al Aliquotas, itens []ItemCalc) ([]Resultado, error) {
	sim := ano >= 2027
	if c.Regime == "simples" {
		return m.simplesCalc(ctx, ano, c, itens)
	}
	res := make([]Resultado, 0, len(itens))
	for i := 0; i < len(itens); i += lote {
		fim := min(i+lote, len(itens))
		rs, err := m.regularLote(ctx, ano, c, al, itens[i:fim], sim)
		if err != nil {
			return nil, err
		}
		res = append(res, rs...)
	}
	return res, nil
}

func (m *Motores) regularLote(ctx context.Context, ano int, c ClienteCalc, al Aliquotas, itens []ItemCalc, sim bool) ([]Resultado, error) {
	req := make([]map[string]any, len(itens))
	for i, it := range itens {
		o := map[string]any{"numero": i + 1, "cst": it.CST, "cClassTrib": it.CClassTrib, "baseCalculo": it.Valor}
		if it.NBS != "" {
			o["nbs"] = it.NBS
		} else {
			o["ncm"], o["quantidade"], o["unidade"] = it.NCM, 1, "UN"
		}
		if it.ISCClassTrib != "" {
			o["impostoSeletivo"] = map[string]any{"cst": it.ISCST, "cClassTrib": it.ISCClassTrib, "baseCalculo": it.Valor, "quantidade": 1, "unidade": "UN"}
		}
		if sim {
			an := map[string]any{"cbs": al.CBS, "ibsEstadual": al.IBSUF, "ibsMunicipal": al.IBSMun}
			if it.ISCClassTrib != "" {
				an["impostoSeletivo"] = map[string]any{"adValorem": al.ISAdValorem}
			}
			o["aliquotasNominais"] = an
		}
		req[i] = o
	}
	body := map[string]any{
		"id": "rtc-hub", "versao": "1.0.0", "dhFatoGerador": fmt.Sprintf("%d-03-10T10:00:00-03:00", ano),
		"municipio": c.MunicipioIBGE, "uf": c.UF, "itens": req,
	}
	raw, err := m.post(ctx, m.regular+"/api/calculadora/observabilidade/regime-geral", body)
	if err != nil {
		return nil, err
	}
	return mapear(raw, itens, sim, func(tc map[string]any) (vcbs, vibs, vis, pc, pi string) {
		g := sub(sub(tc, "IBSCBS"), "gIBSCBS")
		return str(sub(g, "gCBS")["vCBS"]), str(g["vIBS"]), str(sub(tc, "IS")["vIS"]),
			red(sub(g, "gCBS")), red(sub(g, "gIBSUF"))
	})
}

func (m *Motores) simplesCalc(ctx context.Context, ano int, c ClienteCalc, itens []ItemCalc) ([]Resultado, error) {
	sim := ano >= 2027
	res := make([]Resultado, len(itens))
	var idx []int
	var req []map[string]any
	for i, it := range itens {
		res[i] = Resultado{ItemID: it.ID, Simulacao: sim, Estado: "NAO_SUPORTADO"}
		switch {
		case !sim:
			res[i].Mensagem = "Simples Nacional só calcula a partir de 2027"
		case it.NBS != "":
			res[i].Mensagem = "O motor do Simples Nacional ainda não calcula serviços (NBS)"
		default:
			idx = append(idx, i)
			req = append(req, map[string]any{"numero": len(idx), "cAtivSN": c.CAtivSN, "cClassTrib": it.CClassTrib,
				"baseCalculo": fmt.Sprintf("%.2f", it.Valor), "valorImpostoSeletivo": "0.00", "pIBSSN": c.PIBSSN, "pCBSSN": c.PCBSSN})
		}
	}
	for i := 0; i < len(idx); i += lote {
		fim := min(i+lote, len(idx))
		body := map[string]any{"id": "rtc-hub", "versao": "2.0.0", "data": fmt.Sprintf("%d-03-15", ano), "tpDoc": "55", "CRT": "1", "itens": req[i:fim]}
		raw, err := m.post(ctx, m.simples+"/api/calculadora/observabilidade/simples-nacional", body)
		if err != nil {
			return nil, err
		}
		subitens := make([]ItemCalc, fim-i)
		for k := range subitens {
			subitens[k] = itens[idx[i+k]]
		}
		rs, err := mapear(raw, subitens, sim, func(tc map[string]any) (a, b, d, e, f string) {
			return str(tc["vCBSSN"]), str(tc["vIBSSN"]), str(sub(tc, "IS")["valor"]), "", ""
		})
		if err != nil {
			return nil, err
		}
		for k, r := range rs {
			res[idx[i+k]] = r
		}
	}
	return res, nil
}

// mapear liga cada objeto da resposta (nObj, base 1) ao item do lote.
func mapear(raw []byte, itens []ItemCalc, sim bool, ext func(map[string]any) (string, string, string, string, string)) ([]Resultado, error) {
	var r resp
	if err := json.Unmarshal(raw, &r); err != nil {
		return nil, fmt.Errorf("resposta do motor inválida: %w", err)
	}
	out := make([]Resultado, len(itens))
	for i, it := range itens {
		out[i] = Resultado{ItemID: it.ID, Estado: "SEM_RESPOSTA", Simulacao: sim}
	}
	for _, bruto := range r.Objetos {
		var o objeto
		if json.Unmarshal(bruto, &o) != nil || o.NObj < 1 || o.NObj > len(itens) {
			continue
		}
		x := &out[o.NObj-1]
		x.Estado, x.Bruto = o.EstadoItem, bruto // só o objeto deste item, não o lote inteiro
		if len(o.Erros) > 0 {
			e := o.Erros[0]
			x.Mensagem = strings.TrimSpace(e.Title + ": " + e.Detail)
			if strings.HasPrefix(e.Detail, e.Title) {
				x.Mensagem = e.Detail
			}
		}
		var tc map[string]any
		json.Unmarshal(o.TribCalc, &tc)
		x.VCBS, x.VIBS, x.VIS, x.PRedCBS, x.PRedIBS = ext(tc)
		x.Memoria = str(sub(sub(sub(tc, "IBSCBS"), "gIBSCBS"), "gCBS")["memoriaCalculo"]) // só o regime regular devolve
	}
	return out, nil
}

func sub(m map[string]any, k string) map[string]any {
	v, _ := m[k].(map[string]any)
	return v // nil map lê como vazio
}

func str(v any) string { s, _ := v.(string); return s }

func red(g map[string]any) string {
	if s := str(sub(g, "gRed")["pRedAliq"]); s != "" {
		return s
	}
	return "0.00"
}
