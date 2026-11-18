package store

import (
	"context"
	"os"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

func novoStore(t *testing.T) (*Store, context.Context) {
	url := os.Getenv("SUPABASE_DB_URL")
	if url == "" {
		t.Skip("SUPABASE_DB_URL não definida")
	}
	ctx := context.Background()
	s, err := Conectar(ctx, url)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(s.Close)
	return s, ctx
}

func novoUsuario(t *testing.T, s *Store, ctx context.Context) string {
	id := uuid.NewString()
	_, err := s.pool.Exec(ctx, `insert into auth.users (id, email, aud, role, instance_id)
		values ($1, $2, 'authenticated', 'authenticated', '00000000-0000-0000-0000-000000000000')`, id, id+"@teste.local")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { s.pool.Exec(ctx, `delete from auth.users where id=$1`, id) })
	return id
}

func novoCliente(t *testing.T, s *Store, ctx context.Context, user string) Cliente {
	var c Cliente
	err := s.ComoUsuario(ctx, user, func(tx pgx.Tx) error {
		var err error
		c, err = CriarCliente(ctx, tx, Cliente{RazaoSocial: "Acme", CNPJ: "12345678000199", Regime: "regular", UF: "SP", MunicipioIBGE: 3550308})
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	return c
}

func TestRLSIsolaEscritorios(t *testing.T) {
	s, ctx := novoStore(t)
	a, b := novoUsuario(t, s, ctx), novoUsuario(t, s, ctx)
	ca := novoCliente(t, s, ctx, a)

	err := s.ComoUsuario(ctx, b, func(tx pgx.Tx) error {
		cs, err := ListarClientes(ctx, tx)
		if err != nil {
			return err
		}
		if len(cs) != 0 {
			t.Errorf("B vê %d clientes de A", len(cs))
		}
		tag, err := tx.Exec(ctx, `update clientes set razao_social='x' where id=$1`, ca.ID)
		if err != nil {
			return err
		}
		if tag.RowsAffected() != 0 {
			t.Errorf("B alterou %d linhas de A", tag.RowsAffected())
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	err = s.ComoUsuario(ctx, a, func(tx pgx.Tx) error {
		cs, err := ListarClientes(ctx, tx)
		if err != nil {
			return err
		}
		if len(cs) != 1 || cs[0].RazaoSocial != "Acme" {
			t.Errorf("A deveria ver seu cliente intacto: %+v", cs)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

// B não pode pendurar linhas no cliente de A (a FK composta fecha o furo do RLS).
func TestRLSBloqueiaFilhosEmClienteAlheio(t *testing.T) {
	s, ctx := novoStore(t)
	a, b := novoUsuario(t, s, ctx), novoUsuario(t, s, ctx)
	ca := novoCliente(t, s, ctx, a)
	for nome, q := range map[string]string{
		"itens":   `insert into itens (cliente_id, codigo) values ($1,'X')`,
		"notas":   `insert into notas_importadas (cliente_id, chave) values ($1,'K')`,
		"import.": `insert into importacoes (cliente_id, tipo) values ($1,'xml')`,
	} {
		err := s.ComoUsuario(ctx, b, func(tx pgx.Tx) error { _, err := tx.Exec(ctx, q, ca.ID); return err })
		if err == nil {
			t.Errorf("%s: B inseriu no cliente de A", nome)
		}
	}
}

func TestConfirmarEAtualizarItem(t *testing.T) {
	s, ctx := novoStore(t)
	u := novoUsuario(t, s, ctx)
	c := novoCliente(t, s, ctx, u)
	err := s.ComoUsuario(ctx, u, func(tx pgx.Tx) error {
		if _, _, err := UpsertItens(ctx, tx, c.ID, []ItemEntrada{
			{Codigo: "P", Situacao: "provavel", CST: "000", Condicao: "c"},
			{Codigo: "R", Situacao: "revisar"},
		}); err != nil {
			return err
		}
		its, err := ListarItens(ctx, tx, c.ID)
		if err != nil {
			return err
		}
		ids := []string{its[0].ID, its[1].ID}
		if n, err := ConfirmarItens(ctx, tx, c.ID, ids); err != nil || n != 1 {
			t.Fatalf("confirmar: %d %v", n, err)
		}
		its, _ = ListarItens(ctx, tx, c.ID)
		if its[0].Situacao != "ok" || its[0].Origem != "confirmado" || its[1].Situacao != "revisar" || its[1].Origem != "sugerido" {
			t.Errorf("confirmar: %+v", its)
		}
		if err := AtualizarItem(ctx, tx, its[1].ID, "200", "200001", "", ""); err != nil {
			return err
		}
		its, _ = ListarItens(ctx, tx, c.ID)
		i := its[1]
		if i.Origem != "manual" || i.Situacao != "ok" || i.CST != "200" || i.CClassTrib != "200001" || i.Condicao != "" || i.ISCST != "" {
			t.Errorf("manual: %+v", i)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestUpsertPreservaManual(t *testing.T) {
	s, ctx := novoStore(t)
	u := novoUsuario(t, s, ctx)
	c := novoCliente(t, s, ctx, u)
	base := ItemEntrada{Codigo: "P1", Descricao: "Prod", NCM: "1234", Valor: 10, CST: "000", CClassTrib: "000001"}

	err := s.ComoUsuario(ctx, u, func(tx pgx.Tx) error {
		n, a, err := UpsertItens(ctx, tx, c.ID, []ItemEntrada{base})
		if err != nil || n != 1 || a != 0 {
			t.Fatalf("1ª carga: %d %d %v", n, a, err)
		}
		its, _ := ListarItens(ctx, tx, c.ID)
		if err := AtualizarItem(ctx, tx, its[0].ID, "200", "200001", "", ""); err != nil {
			t.Fatal(err)
		}
		novo := base
		novo.Valor, novo.CST, novo.CClassTrib, novo.Descricao = 25, "000", "000002", "Prod novo"
		n, a, err = UpsertItens(ctx, tx, c.ID, []ItemEntrada{novo})
		if err != nil || n != 0 || a != 1 {
			t.Fatalf("2ª carga: %d %d %v", n, a, err)
		}
		its, _ = ListarItens(ctx, tx, c.ID)
		i := its[0]
		if i.CST != "200" || i.CClassTrib != "200001" || i.Origem != "manual" {
			t.Errorf("manual não preservado: %+v", i)
		}
		if i.Ocorrencias != 2 || i.UltimoValor != 25 || i.Descricao != "Prod novo" {
			t.Errorf("contagem/valor: %+v", i)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestRegistrarNotasEAliquotas(t *testing.T) {
	s, ctx := novoStore(t)
	u := novoUsuario(t, s, ctx)
	c := novoCliente(t, s, ctx, u)
	err := s.ComoUsuario(ctx, u, func(tx pgx.Tx) error {
		if j, err := RegistrarNotas(ctx, tx, c.ID, []string{"k1", "k2"}); err != nil || len(j) != 0 {
			t.Errorf("1ª: %v %v", j, err)
		}
		if j, err := RegistrarNotas(ctx, tx, c.ID, []string{"k2", "k3"}); err != nil || len(j) != 1 || j[0] != "k2" {
			t.Errorf("2ª: %v %v", j, err)
		}
		al, err := Aliquotas(ctx, tx)
		if err != nil || al.CBS != 8.8 {
			t.Errorf("default: %+v %v", al, err)
		}
		if err := SalvarAliquotas(ctx, tx, AliquotasEst{CBS: 9, IBSUF: 10, IBSMun: 3}); err != nil {
			t.Error(err)
		}
		al, _ = Aliquotas(ctx, tx)
		if al.CBS != 9 || al.IBSUF != 10 {
			t.Errorf("salvo: %+v", al)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestClienteAtualizarExcluirEImportacao(t *testing.T) {
	s, ctx := novoStore(t)
	a, b := novoUsuario(t, s, ctx), novoUsuario(t, s, ctx)
	c := novoCliente(t, s, ctx, a)
	err := s.ComoUsuario(ctx, b, func(tx pgx.Tx) error {
		if _, err := ObterCliente(ctx, tx, c.ID); err != pgx.ErrNoRows {
			t.Errorf("B obteve cliente de A: %v", err)
		}
		if ok, err := ExcluirCliente(ctx, tx, c.ID); ok || err != nil {
			t.Errorf("B excluiu cliente de A: %v %v", ok, err)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	err = s.ComoUsuario(ctx, a, func(tx pgx.Tx) error {
		c.RazaoSocial = "Acme 2"
		got, err := AtualizarCliente(ctx, tx, c)
		if err != nil || got.RazaoSocial != "Acme 2" {
			t.Errorf("atualizar: %+v %v", got, err)
		}
		if err := RegistrarImportacao(ctx, tx, c.ID, "xml", 2, 1, 0, []map[string]string{{"arquivo": "a.xml"}}); err != nil {
			t.Error(err)
		}
		var n int
		tx.QueryRow(ctx, `select count(*) from importacoes where cliente_id=$1 and ignorados->0->>'arquivo'='a.xml'`, c.ID).Scan(&n)
		if n != 1 {
			t.Errorf("importacao: %d", n)
		}
		if ok, err := ExcluirCliente(ctx, tx, c.ID); !ok || err != nil {
			t.Errorf("excluir: %v %v", ok, err)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestDefinirNBSEContagem(t *testing.T) {
	s, ctx := novoStore(t)
	u := novoUsuario(t, s, ctx)
	c := novoCliente(t, s, ctx, u)
	err := s.ComoUsuario(ctx, u, func(tx pgx.Tx) error {
		if _, _, err := UpsertItens(ctx, tx, c.ID, []ItemEntrada{
			{Codigo: "S", CTribNac: "010101", Situacao: "revisar"},
			{Codigo: "P", NCM: "10063021", Situacao: "provavel"},
		}); err != nil {
			return err
		}
		its, _ := ListarItens(ctx, tx, c.ID) // P, S
		for _, i := range its {
			if err := DefinirNBS(ctx, tx, i.ID, "115022000"); err != nil {
				return err
			}
		}
		its, _ = ListarItens(ctx, tx, c.ID)
		if its[0].NBS != "" || its[1].NBS != "115022000" {
			t.Errorf("nbs: %+v", its)
		}
		cont, err := ContagemPorSituacao(ctx, tx)
		if err != nil || cont[c.ID]["revisar"] != 1 || cont[c.ID]["provavel"] != 1 {
			t.Errorf("contagem: %v %v", cont, err)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

// Cálculo guardado não pode sobreviver a uma mudança que altera o resultado.
func TestCalculosDescartadosAoMudar(t *testing.T) {
	s, ctx := novoStore(t)
	u := novoUsuario(t, s, ctx)
	c := novoCliente(t, s, ctx, u)
	calc := `{"vCbs":"1.00"}`
	err := s.ComoUsuario(ctx, u, func(tx pgx.Tx) error {
		base := []ItemEntrada{{Codigo: "A", NCM: "1", Situacao: "ok", CST: "000", CClassTrib: "000001"}, {Codigo: "B", NCM: "2", Situacao: "ok", CST: "000", CClassTrib: "000001"}}
		UpsertItens(ctx, tx, c.ID, base)
		its, _ := ListarItens(ctx, tx, c.ID)
		calcular := func() {
			for _, i := range its {
				if err := SalvarCalculo(ctx, tx, i.ID, "2027", []byte(calc)); err != nil {
					t.Fatal(err)
				}
			}
		}
		vazios := func(passo string, want ...bool) {
			atual, _ := ListarItens(ctx, tx, c.ID)
			for k, i := range atual {
				if (string(i.Calculos) == "{}") != want[k] {
					t.Errorf("%s: %s calculos=%s", passo, i.Codigo, i.Calculos)
				}
			}
		}
		calcular()
		mudou := base
		mudou[1].CClassTrib = "000002"
		UpsertItens(ctx, tx, c.ID, mudou) // reimportação com a mesma classificação mantém, com outra descarta
		vazios("reimportar", false, true)

		calcular()
		AtualizarItem(ctx, tx, its[0].ID, "200", "200003", "", "")
		vazios("patch", true, false)

		calcular()
		c.RazaoSocial = "Outro nome"
		AtualizarCliente(ctx, tx, c)
		vazios("razão social", false, false)
		c.UF = "RJ"
		if _, err := AtualizarCliente(ctx, tx, c); err != nil {
			t.Fatal(err)
		}
		vazios("UF", true, true)

		calcular()
		if err := SalvarAliquotas(ctx, tx, AliquotasEst{CBS: 9}); err != nil {
			t.Fatal(err)
		}
		vazios("alíquotas", true, true)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestAtualizarItemCondicaoDoIS(t *testing.T) {
	s, ctx := novoStore(t)
	u := novoUsuario(t, s, ctx)
	c := novoCliente(t, s, ctx, u)
	err := s.ComoUsuario(ctx, u, func(tx pgx.Tx) error {
		UpsertItens(ctx, tx, c.ID, []ItemEntrada{{Codigo: "C", NCM: "22030000", Situacao: "provavel", ISCST: "000", ISCClassTrib: "000001", Condicao: "IS"}})
		its, _ := ListarItens(ctx, tx, c.ID)
		AtualizarItem(ctx, tx, its[0].ID, "000", "000001", "000", "000001")
		if i, _ := ObterItem(ctx, tx, its[0].ID); i.Condicao != "IS" {
			t.Errorf("com IS: %q", i.Condicao)
		}
		AtualizarItem(ctx, tx, its[0].ID, "000", "000001", "", "")
		if i, _ := ObterItem(ctx, tx, its[0].ID); i.Condicao != "" || i.ISCClassTrib != "" {
			t.Errorf("sem IS (revenda): %+v", i)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}
