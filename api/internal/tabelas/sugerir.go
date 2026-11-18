package tabelas

import (
	"context"
	"database/sql"
	"fmt"
	"slices"
	"strconv"
	"strings"
)

const (
	condicaoIS          = "O IS incide uma vez, no primeiro fornecimento (fabricante ou importador). Na revenda não há IS."
	condicaoMedicamento = "Medicamentos: a redução depende de registro na Anvisa."
	condicaoAnvisa      = "Só se o medicamento tiver registro na Anvisa"
)

// classesMedicamento valem para todo NCM do capítulo 30: a tabela da Receita não lista os
// NCMs dos medicamentos, a redução depende do registro na Anvisa (o motor aceita).
var classesMedicamento = []string{"200009", "200032"}

func medicamento(tipo Tipo, codigo string) bool {
	return tipo == NCM && strings.HasPrefix(codigo, "30")
}

// vig: linha com prefixo p vigente na data :d (datas TEXT 'YYYY-MM-DD').
func vig(p string) string {
	return fmt.Sprintf("%[1]s_INICIO_VIGENCIA <= :d AND (%[1]s_FIM_VIGENCIA IS NULL OR %[1]s_FIM_VIGENCIA >= :d)", p)
}

// sqlOpcao lista classificações vigentes de um grupo (:is = 1 → IS, 0 → IBS/CBS).
// O grupo é estrutural (TRST_TBTO_ID = 1), sem filtro de vigência no vínculo.
var sqlOpcao = `
SELECT ct.CLTR_CD, ct.CLTR_DESCRICAO, s.SITR_CD,
  COALESCE((SELECT group_concat(f.FDLG_TEXTO_CURTO, '; ') FROM FUNDAMENTACAO_CLASSIFICACAO fc
    JOIN FUNDAMENTACAO_LEGAL f ON f.FDLG_ID = fc.FDCL_FDLG_ID
    WHERE fc.FDCL_CLTR_ID = ct.CLTR_ID AND ` + vig("fc.FDCL") + ` AND ` + vig("f.FDLG") + `), ''),
  COALESCE((SELECT max(p.PERE_VALOR) FROM PERCENTUAL_REDUCAO p
    WHERE p.PERE_CLTR_ID = ct.CLTR_ID AND ` + vig("p.PERE") + `), 0),
  COALESCE((SELECT group_concat(td.TPDF_SIGLA, ', ') FROM TIPO_DFE_CLASSIFICACAO tc
    JOIN TIPO_DFE td ON td.TPDF_ID = tc.TDCL_TPDF_ID
    WHERE tc.TDCL_CLTR_ID = ct.CLTR_ID AND ` + vig("tc.TDCL") + `), '')
FROM CLASSIFICACAO_TRIBUTARIA ct
JOIN SITUACAO_TRIBUTARIA s ON s.SITR_ID = ct.CLTR_SITR_ID
WHERE ` + vig("ct.CLTR") + `
  AND EXISTS (SELECT 1 FROM TRIBUTO_SITUACAO_TRIBUTARIA t
    WHERE t.TRST_SITR_ID = ct.CLTR_SITR_ID AND t.TRST_TBTO_ID = 1) = :is
`

// Casamento por prefixo como NcmAplicavelRepository.java: {X} tabela, {A} aplicável, {E} exceção.
var filtroCandidatos = `
  AND ct.CLTR_ID IN (SELECT n.{A}_CLTR_ID FROM {X}_APLICAVEL n
    WHERE n.{A}_{X}_CD = substr(:cod, 1, length(n.{A}_{X}_CD)) AND ` + vig("n.{A}") + `
      AND NOT EXISTS (SELECT 1 FROM EXCECAO_{X}_APLICAVEL e
        WHERE e.{E}_{A}_ID = n.{A}_ID AND e.{E}_{X}_CD = substr(:cod, 1, length(e.{E}_{X}_CD)) AND ` + vig("e.{E}") + `))
ORDER BY ct.CLTR_CD`

var (
	sqlCandidatos = map[Tipo]string{NCM: porTipo(NCM, sqlOpcao+filtroCandidatos), NBS: porTipo(NBS, sqlOpcao+filtroCandidatos)}
	sqlVigente    = map[Tipo]string{NCM: porTipo(NCM, "SELECT 1 FROM {X} WHERE {X}_CD = :cod AND "+vig("{X}")), NBS: porTipo(NBS, "SELECT 1 FROM {X} WHERE {X}_CD = :cod AND "+vig("{X}"))}
	sqlPorCodigo  = sqlOpcao + "  AND ct.CLTR_CD = :cc ORDER BY ct.CLTR_ID LIMIT 1"
	sqlTemLista   = map[Tipo]string{NCM: porTipo(NCM, sqlListaAplicavel), NBS: porTipo(NBS, sqlListaAplicavel)}
)

// sqlListaAplicavel: a classificação está restrita a uma lista de NCM/NBS vigente?
var sqlListaAplicavel = `SELECT EXISTS (SELECT 1 FROM {X}_APLICAVEL n JOIN CLASSIFICACAO_TRIBUTARIA ct ON ct.CLTR_ID = n.{A}_CLTR_ID
	WHERE ct.CLTR_CD = :cc AND ` + vig("n.{A}") + ` AND ` + vig("ct.CLTR") + `
	  AND EXISTS (SELECT 1 FROM TRIBUTO_SITUACAO_TRIBUTARIA t WHERE t.TRST_SITR_ID = ct.CLTR_SITR_ID AND t.TRST_TBTO_ID = 1) = :is)`

func porTipo(tipo Tipo, s string) string {
	a, e := "NCMA", "ENCM"
	if tipo == NBS {
		a, e = "NBSA", "ENBS"
	}
	return strings.NewReplacer("{X}", string(tipo), "{A}", a, "{E}", e).Replace(s)
}

func (t *Tabelas) opcoes(ctx context.Context, q string, args ...any) ([]Opcao, error) {
	rows, err := t.db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Opcao
	visto := map[string]bool{}
	for rows.Next() {
		var o Opcao
		var dfe string
		if err := rows.Scan(&o.CClassTrib, &o.Descricao, &o.CST, &o.Fundamentacao, &o.PercReducao, &dfe); err != nil {
			return nil, err
		}
		if visto[o.CClassTrib] {
			continue
		}
		visto[o.CClassTrib] = true
		if dfe != "" {
			o.Condicao = "Só em: " + dfe
		}
		if strings.HasPrefix(o.Fundamentacao, "Art.") {
			o.Fundamentacao = "LC 214/2025, " + o.Fundamentacao
		}
		out = append(out, o)
	}
	return out, rows.Err()
}

// Classificacao busca um cClassTrib vigente no grupo IS (is=true) ou IBS/CBS; nil se não existe.
func (t *Tabelas) Classificacao(ctx context.Context, cClassTrib, data string, is bool) (*Opcao, error) {
	g := 0
	if is {
		g = 1
	}
	os, err := t.opcoes(ctx, sqlPorCodigo, sql.Named("cc", cClassTrib), sql.Named("d", data), sql.Named("is", g))
	if err != nil || len(os) == 0 {
		return nil, err
	}
	return &os[0], nil
}

// ValeParaCodigo: false só quando a classificação tem lista de NCM/NBS na tabela da Receita e o código
// não está nela (mesma regra de prefixo e exceções das sugestões). Medicamentos do capítulo 30 valem.
func (t *Tabelas) ValeParaCodigo(ctx context.Context, tipo Tipo, codigo, cClassTrib, data string, is bool) (bool, error) {
	codigo = NormalizaCodigo(codigo)
	if medicamento(tipo, codigo) && slices.Contains(classesMedicamento, cClassTrib) {
		return true, nil
	}
	g := 0
	if is {
		g = 1
	}
	var tem bool
	if err := t.db.QueryRowContext(ctx, sqlTemLista[tipo], sql.Named("cc", cClassTrib), sql.Named("d", data), sql.Named("is", g)).Scan(&tem); err != nil || !tem {
		return !tem, err
	}
	os, err := t.opcoes(ctx, sqlCandidatos[tipo], sql.Named("cod", codigo), sql.Named("d", data), sql.Named("is", g))
	return slices.ContainsFunc(os, func(o Opcao) bool { return o.CClassTrib == cClassTrib }), err
}

// padrao: cClassTrib 000001 procurado dentro do grupo (o código existe no IBS/CBS e no IS).
func (t *Tabelas) padrao(ctx context.Context, data string, is int) (Opcao, error) {
	o, err := t.Classificacao(ctx, "000001", data, is == 1)
	if err != nil {
		return Opcao{}, err
	}
	if o == nil {
		return Opcao{}, fmt.Errorf("cClassTrib 000001 (is=%d) não vigente em %s", is, data)
	}
	return *o, nil
}

var peso = map[Situacao]int{Ok: 0, Provavel: 1, Revisar: 2, Invalido: 3}

func (t *Tabelas) Sugerir(ctx context.Context, tipo Tipo, codigo string, data string) (Sugestao, error) {
	codigo = NormalizaCodigo(codigo)
	if tipo != NCM && tipo != NBS {
		return Sugestao{}, fmt.Errorf("tipo inválido: %q", tipo)
	}
	if tipo == NCM && len(codigo) != 8 {
		return Sugestao{Situacao: Invalido, Motivo: "NCM incompleto"}, nil
	}
	var um int
	err := t.db.QueryRowContext(ctx, sqlVigente[tipo], sql.Named("cod", codigo), sql.Named("d", data)).Scan(&um)
	if err == sql.ErrNoRows {
		return Sugestao{Situacao: Invalido, Motivo: string(tipo) + " inexistente ou fora de vigência"}, nil
	}
	if err != nil {
		return Sugestao{}, err
	}

	cand := func(is int) ([]Opcao, error) {
		return t.opcoes(ctx, sqlCandidatos[tipo], sql.Named("cod", codigo), sql.Named("d", data), sql.Named("is", is))
	}
	var s Sugestao

	ibs, err := cand(0)
	if err != nil {
		return Sugestao{}, err
	}
	p, err := t.padrao(ctx, data, 0)
	if err != nil {
		return Sugestao{}, err
	}
	if medicamento(tipo, codigo) {
		// nunca "ok": sem o registro na Anvisa a tabela não decide, o contador decide
		for _, c := range classesMedicamento {
			o, err := t.Classificacao(ctx, c, data, false)
			if err != nil {
				return Sugestao{}, err
			}
			if o != nil && !slices.ContainsFunc(ibs, func(x Opcao) bool { return x.CClassTrib == c }) {
				o.Condicao = condicaoAnvisa
				ibs = append(ibs, *o)
			}
		}
		s.Motivo = condicaoMedicamento
		s.IBSCBS = Classificacao{Situacao: Revisar, Opcoes: append(ibs, p)}
	} else {
		switch len(ibs) {
		case 0:
			s.IBSCBS = Classificacao{Situacao: Ok, Escolhida: &p, Opcoes: []Opcao{p}}
		case 1:
			s.IBSCBS = Classificacao{Situacao: Provavel, Escolhida: &ibs[0], Opcoes: []Opcao{ibs[0], p}}
		default:
			s.IBSCBS = Classificacao{Situacao: Revisar, Opcoes: append(ibs, p)}
		}
	}

	is, err := cand(1)
	if err != nil {
		return Sugestao{}, err
	}
	if len(is) > 0 {
		// Candidatos de IS são tipos de operação; quem revende não paga IS, então nunca "ok".
		pi, err := t.padrao(ctx, data, 1)
		if err != nil {
			return Sugestao{}, err
		}
		pi.Condicao = condicaoIS
		s.IS = &Classificacao{Situacao: Provavel, Escolhida: &pi, Opcoes: is}
	}

	s.Situacao = s.IBSCBS.Situacao
	if s.IS != nil && peso[s.IS.Situacao] > peso[s.Situacao] {
		s.Situacao = s.IS.Situacao
	}
	return s, nil
}

// NBSPorServicoLC116 lista as NBS ligadas a um cTribNac de 6 dígitos (item, subitem, desdobro).
// LSLC_CD usa "1.01" para itens comuns e "99.01.01" no item 99; nenhum código existe nas duas formas.
func (t *Tabelas) NBSPorServicoLC116(ctx context.Context, cTribNac, data string) ([]string, error) {
	c := NormalizaCodigo(cTribNac)
	if _, err := strconv.ParseUint(c, 10, 32); len(c) != 6 || err != nil {
		return nil, fmt.Errorf("cTribNac inválido: %q", cTribNac)
	}
	item, _ := strconv.Atoi(c[:2])
	sub := fmt.Sprintf("%d.%s", item, c[2:4])
	rows, err := t.db.QueryContext(ctx, `SELECT DISTINCT CNIL_NBS_CD FROM CLASSIF_NBS_INDOP_LC
		WHERE CNIL_LSLC_CD IN (:c3, :c2) AND `+vig("CNIL")+` ORDER BY 1`,
		sql.Named("c3", sub+"."+c[4:6]), sql.Named("c2", sub), sql.Named("d", data))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var n string
		if err := rows.Scan(&n); err != nil {
			return nil, err
		}
		out = append(out, n)
	}
	return out, rows.Err()
}
