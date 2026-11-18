package tabelas

import (
	"context"
	"database/sql"
	"fmt"
	"strconv"
	"strings"
)

const condicaoIS = "O IS incide uma vez, no primeiro fornecimento (fabricante ou importador). Na revenda não há IS."

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
	sqlPadrao     = sqlOpcao + "  AND ct.CLTR_CD = '000001' ORDER BY ct.CLTR_ID LIMIT 1"
)

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
		out = append(out, o)
	}
	return out, rows.Err()
}

// padrao: cClassTrib 000001 procurado dentro do grupo (o código existe no IBS/CBS e no IS).
func (t *Tabelas) padrao(ctx context.Context, data string, is int) (Opcao, error) {
	os, err := t.opcoes(ctx, sqlPadrao, sql.Named("d", data), sql.Named("is", is))
	if err != nil {
		return Opcao{}, err
	}
	if len(os) == 0 {
		return Opcao{}, fmt.Errorf("cClassTrib 000001 (is=%d) não vigente em %s", is, data)
	}
	return os[0], nil
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
	switch len(ibs) {
	case 0:
		s.IBSCBS = Classificacao{Situacao: Ok, Escolhida: &p, Opcoes: []Opcao{p}}
	case 1:
		s.IBSCBS = Classificacao{Situacao: Provavel, Escolhida: &ibs[0], Opcoes: []Opcao{ibs[0], p}}
	default:
		s.IBSCBS = Classificacao{Situacao: Revisar, Opcoes: append(ibs, p)}
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
