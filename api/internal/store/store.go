// Package store é o acesso ao Postgres (Supabase). Toda consulta roda em ComoUsuario,
// que ativa o RLS: um escritório nunca enxerga os dados de outro.
package store

import (
	"context"
	"encoding/json"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Store struct{ pool *pgxpool.Pool }

func Conectar(ctx context.Context, databaseURL string) (*Store, error) {
	p, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		return nil, err
	}
	if err := p.Ping(ctx); err != nil {
		p.Close()
		return nil, err
	}
	return &Store{pool: p}, nil
}

func (s *Store) Close() { s.pool.Close() }

// ComoUsuario roda fn numa transação como role authenticated com o sub do JWT = userID.
func (s *Store) ComoUsuario(ctx context.Context, userID string, fn func(pgx.Tx) error) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	claims, _ := json.Marshal(map[string]string{"sub": userID, "role": "authenticated"})
	if _, err := tx.Exec(ctx, `set local role authenticated`); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `select set_config('request.jwt.claims', $1, true)`, string(claims)); err != nil {
		return err
	}
	if err := fn(tx); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

type Cliente struct {
	ID            string   `json:"id"`
	RazaoSocial   string   `json:"razaoSocial"`
	CNPJ          string   `json:"cnpj"`
	Regime        string   `json:"regime"`
	UF            string   `json:"uf"`
	MunicipioIBGE int      `json:"municipioIbge"`
	CAtivSN       string   `json:"cAtivSn"`
	PIBSSN        *float64 `json:"pIbsSn"`
	PCBSSN        *float64 `json:"pCbsSn"`
}

type Item struct {
	ID           string          `json:"id"`
	ClienteID    string          `json:"clienteId"`
	Codigo       string          `json:"codigo"`
	Descricao    string          `json:"descricao"`
	NCM          string          `json:"ncm"`
	NBS          string          `json:"nbs"`
	CTribNac     string          `json:"cTribNac"`
	Ocorrencias  int             `json:"ocorrencias"`
	UltimoValor  float64         `json:"ultimoValor"`
	Situacao     string          `json:"situacao"`
	Origem       string          `json:"origem"`
	CST          string          `json:"cst"`
	CClassTrib   string          `json:"cClassTrib"`
	ISCST        string          `json:"isCst"`
	ISCClassTrib string          `json:"isCClassTrib"`
	Condicao     string          `json:"condicao"`
	Opcoes       json.RawMessage `json:"opcoes"`
	Calculos     json.RawMessage `json:"calculos"` // chave = ano
}

// ItemEntrada é a identidade do item mais a sugestão do motor.
type ItemEntrada struct {
	Codigo, Descricao, NCM, NBS, CTribNac string
	Valor                                 float64
	Situacao, CST, CClassTrib             string
	ISCST, ISCClassTrib, Condicao         string
	Opcoes                                json.RawMessage
}

type AliquotasEst struct {
	CBS         float64 `json:"cbs"`
	IBSUF       float64 `json:"ibsUf"`
	IBSMun      float64 `json:"ibsMun"`
	ISAdValorem float64 `json:"isAdValorem"`
}

const colsCliente = `id, razao_social, cnpj, regime, uf, municipio_ibge, coalesce(c_ativ_sn,''), p_ibs_sn, p_cbs_sn`

func scanCliente(r pgx.Row) (c Cliente, err error) {
	err = r.Scan(&c.ID, &c.RazaoSocial, &c.CNPJ, &c.Regime, &c.UF, &c.MunicipioIBGE, &c.CAtivSN, &c.PIBSSN, &c.PCBSSN)
	return
}

func ListarClientes(ctx context.Context, tx pgx.Tx) ([]Cliente, error) {
	rows, err := tx.Query(ctx, `select `+colsCliente+` from clientes order by razao_social`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Cliente
	for rows.Next() {
		c, err := scanCliente(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

func CriarCliente(ctx context.Context, tx pgx.Tx, c Cliente) (Cliente, error) {
	return scanCliente(tx.QueryRow(ctx, `insert into clientes (razao_social, cnpj, regime, uf, municipio_ibge, c_ativ_sn, p_ibs_sn, p_cbs_sn)
		values ($1,$2,$3,$4,$5,nullif($6,''),$7,$8) returning `+colsCliente,
		c.RazaoSocial, c.CNPJ, c.Regime, c.UF, c.MunicipioIBGE, c.CAtivSN, c.PIBSSN, c.PCBSSN))
}

// UpsertItens: reimportar soma ocorrencias; a classificação só muda se a origem ainda é 'sugerido'.
func UpsertItens(ctx context.Context, tx pgx.Tx, clienteID string, itens []ItemEntrada) (novos, atualizados int, err error) {
	for _, it := range itens {
		if it.Opcoes == nil {
			it.Opcoes = json.RawMessage(`[]`)
		}
		var inserido bool
		err = tx.QueryRow(ctx, `insert into itens (cliente_id, codigo, descricao, ncm, nbs, c_trib_nac, ultimo_valor,
				situacao, cst, cclasstrib, is_cst, is_cclasstrib, condicao, opcoes)
			values ($1,$2,$3,$4,$5,$6,$7,coalesce(nullif($8,''),'revisar'),$9,$10,$11,$12,$13,$14)
			on conflict (cliente_id, codigo, ncm, nbs) do update set
				ocorrencias = itens.ocorrencias + 1,
				ultimo_valor = case when excluded.ultimo_valor > 0 then excluded.ultimo_valor else itens.ultimo_valor end,
				descricao = excluded.descricao,
				atualizado_em = now(),
				situacao = case when itens.origem = 'sugerido' then excluded.situacao else itens.situacao end,
				cst = case when itens.origem = 'sugerido' then excluded.cst else itens.cst end,
				cclasstrib = case when itens.origem = 'sugerido' then excluded.cclasstrib else itens.cclasstrib end,
				is_cst = case when itens.origem = 'sugerido' then excluded.is_cst else itens.is_cst end,
				is_cclasstrib = case when itens.origem = 'sugerido' then excluded.is_cclasstrib else itens.is_cclasstrib end,
				condicao = case when itens.origem = 'sugerido' then excluded.condicao else itens.condicao end,
				opcoes = case when itens.origem = 'sugerido' then excluded.opcoes else itens.opcoes end
			returning (xmax = 0)`,
			clienteID, it.Codigo, it.Descricao, it.NCM, it.NBS, it.CTribNac, it.Valor,
			it.Situacao, it.CST, it.CClassTrib, it.ISCST, it.ISCClassTrib, it.Condicao, it.Opcoes).Scan(&inserido)
		if err != nil {
			return
		}
		if inserido {
			novos++
		} else {
			atualizados++
		}
	}
	return
}

const colsItem = `id, cliente_id, codigo, descricao, ncm, nbs, c_trib_nac, ocorrencias, ultimo_valor, situacao, origem,
	cst, cclasstrib, is_cst, is_cclasstrib, condicao, opcoes, calculos`

func scanItem(r pgx.Row) (i Item, err error) {
	err = r.Scan(&i.ID, &i.ClienteID, &i.Codigo, &i.Descricao, &i.NCM, &i.NBS, &i.CTribNac, &i.Ocorrencias, &i.UltimoValor,
		&i.Situacao, &i.Origem, &i.CST, &i.CClassTrib, &i.ISCST, &i.ISCClassTrib, &i.Condicao, &i.Opcoes, &i.Calculos)
	return
}

func ListarItens(ctx context.Context, tx pgx.Tx, clienteID string) ([]Item, error) {
	rows, err := tx.Query(ctx, `select `+colsItem+` from itens where cliente_id=$1 order by codigo, ncm, nbs`, clienteID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Item
	for rows.Next() {
		i, err := scanItem(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, i)
	}
	return out, rows.Err()
}

// AtualizarItem é a edição manual: origem 'manual', situacao 'ok', condição limpa.
func AtualizarItem(ctx context.Context, tx pgx.Tx, id, cst, cClassTrib, isCST, isCClassTrib string) error {
	_, err := tx.Exec(ctx, `update itens set origem='manual', situacao='ok', cst=$2, cclasstrib=$3, is_cst=$4, is_cclasstrib=$5,
		condicao='', atualizado_em=now() where id=$1`, id, cst, cClassTrib, isCST, isCClassTrib)
	return err
}

// ConfirmarItens aprova em lote só os 'provavel' ainda 'sugerido' do cliente; devolve quantos mudaram.
func ConfirmarItens(ctx context.Context, tx pgx.Tx, clienteID string, ids []string) (int64, error) {
	tag, err := tx.Exec(ctx, `update itens set origem='confirmado', situacao='ok', atualizado_em=now()
		where id = any($1::uuid[]) and cliente_id=$2 and origem='sugerido' and situacao='provavel'`, ids, clienteID)
	return tag.RowsAffected(), err
}

// SalvarCalculo guarda o resultado do motor em itens.calculos[ano].
func SalvarCalculo(ctx context.Context, tx pgx.Tx, itemID, ano string, resultado json.RawMessage) error {
	_, err := tx.Exec(ctx, `update itens set calculos = jsonb_set(calculos, array[$2::text], $3::jsonb), atualizado_em=now()
		where id=$1`, itemID, ano, string(resultado))
	return err
}

func Aliquotas(ctx context.Context, tx pgx.Tx) (a AliquotasEst, err error) {
	err = tx.QueryRow(ctx, `select cbs, ibs_uf, ibs_mun, is_ad_valorem from aliquotas_estimadas`).
		Scan(&a.CBS, &a.IBSUF, &a.IBSMun, &a.ISAdValorem)
	if err == pgx.ErrNoRows { // sem linha: valores padrão da tabela
		return AliquotasEst{CBS: 8.80, IBSUF: 15.00, IBSMun: 2.70}, nil
	}
	return
}

func SalvarAliquotas(ctx context.Context, tx pgx.Tx, a AliquotasEst) error {
	_, err := tx.Exec(ctx, `insert into aliquotas_estimadas (cbs, ibs_uf, ibs_mun, is_ad_valorem) values ($1,$2,$3,$4)
		on conflict (user_id) do update set cbs=$1, ibs_uf=$2, ibs_mun=$3, is_ad_valorem=$4`,
		a.CBS, a.IBSUF, a.IBSMun, a.ISAdValorem)
	return err
}

// RegistrarNotas insere as chaves e devolve as que já existiam.
func RegistrarNotas(ctx context.Context, tx pgx.Tx, clienteID string, chaves []string) ([]string, error) {
	var ja []string
	for _, k := range chaves {
		tag, err := tx.Exec(ctx, `insert into notas_importadas (cliente_id, chave) values ($1,$2) on conflict do nothing`, clienteID, k)
		if err != nil {
			return nil, err
		}
		if tag.RowsAffected() == 0 {
			ja = append(ja, k)
		}
	}
	return ja, nil
}

// ObterCliente devolve pgx.ErrNoRows se o cliente não existe ou é de outro escritório.
func ObterCliente(ctx context.Context, tx pgx.Tx, id string) (Cliente, error) {
	return scanCliente(tx.QueryRow(ctx, `select `+colsCliente+` from clientes where id=$1`, id))
}

func AtualizarCliente(ctx context.Context, tx pgx.Tx, c Cliente) (Cliente, error) {
	return scanCliente(tx.QueryRow(ctx, `update clientes set razao_social=$2, cnpj=$3, regime=$4, uf=$5, municipio_ibge=$6,
		c_ativ_sn=nullif($7,''), p_ibs_sn=$8, p_cbs_sn=$9 where id=$1 returning `+colsCliente,
		c.ID, c.RazaoSocial, c.CNPJ, c.Regime, c.UF, c.MunicipioIBGE, c.CAtivSN, c.PIBSSN, c.PCBSSN))
}

// ExcluirCliente apaga o cliente e, em cascata, itens, notas e importações; false se não existia.
func ExcluirCliente(ctx context.Context, tx pgx.Tx, id string) (bool, error) {
	tag, err := tx.Exec(ctx, `delete from clientes where id=$1`, id)
	return tag.RowsAffected() == 1, err
}

func ObterItem(ctx context.Context, tx pgx.Tx, id string) (Item, error) {
	return scanItem(tx.QueryRow(ctx, `select `+colsItem+` from itens where id=$1`, id))
}

// RegistrarImportacao guarda o histórico de uma importação (ignorados em JSON).
func RegistrarImportacao(ctx context.Context, tx pgx.Tx, clienteID, tipo string, arquivos, novos, atualizados int, ignorados any) error {
	ign, err := json.Marshal(ignorados)
	if err != nil {
		return err
	}
	_, err = tx.Exec(ctx, `insert into importacoes (cliente_id, tipo, arquivos, itens_novos, itens_atualizados, ignorados)
		values ($1,$2,$3,$4,$5,$6)`, clienteID, tipo, arquivos, novos, atualizados, string(ign))
	return err
}

// DefinirNBS grava a NBS escolhida num serviço que veio sem NCM e sem NBS; nos demais não faz nada.
func DefinirNBS(ctx context.Context, tx pgx.Tx, id, nbs string) error {
	_, err := tx.Exec(ctx, `update itens set nbs=$2, atualizado_em=now() where id=$1 and ncm='' and nbs=''`, id, nbs)
	return err
}

// ContagemPorSituacao: cliente_id → situação → quantidade de itens.
func ContagemPorSituacao(ctx context.Context, tx pgx.Tx) (map[string]map[string]int, error) {
	rows, err := tx.Query(ctx, `select cliente_id, situacao, count(*) from itens group by 1, 2`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string]map[string]int{}
	for rows.Next() {
		var c, sit string
		var n int
		if err := rows.Scan(&c, &sit, &n); err != nil {
			return nil, err
		}
		if out[c] == nil {
			out[c] = map[string]int{}
		}
		out[c][sit] = n
	}
	return out, rows.Err()
}
