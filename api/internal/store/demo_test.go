package store

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

const usuarioModelo = "00000000-0000-0000-0000-00000000de00"

func novoAnonimo(t *testing.T, s *Store, ctx context.Context) string {
	id := uuid.NewString()
	_, err := s.pool.Exec(ctx, `insert into auth.users (id, aud, role, instance_id, is_anonymous)
		values ($1, 'authenticated', 'authenticated', '00000000-0000-0000-0000-000000000000', true)`, id)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { s.pool.Exec(ctx, `delete from auth.users where id=$1`, id) })
	return id
}

// contar devolve (clientes, itens, itens calculados em 2027, linhas de alíquotas) visíveis ao usuário.
func contar(t *testing.T, s *Store, ctx context.Context, u string) (cl, it, calc, al int) {
	err := s.ComoUsuario(ctx, u, func(tx pgx.Tx) error {
		return tx.QueryRow(ctx, `select (select count(*) from clientes), (select count(*) from itens),
			(select count(*) from itens where calculos ? '2027'), (select count(*) from aliquotas_estimadas)`).Scan(&cl, &it, &calc, &al)
	})
	if err != nil {
		t.Fatal(err)
	}
	return
}

func TestClonarDemoIdempotente(t *testing.T) {
	s, ctx := novoStore(t)
	a, b := novoAnonimo(t, s, ctx), novoAnonimo(t, s, ctx)
	clonar := func(u string) {
		if err := s.ComoUsuario(ctx, u, func(tx pgx.Tx) error { _, err := tx.Exec(ctx, `select clonar_demo()`); return err }); err != nil {
			t.Fatal(err)
		}
	}
	clonar(a)
	clonar(a)
	cl, it, calc, al := contar(t, s, ctx, a)
	if cl != 3 || it < 45 || calc == 0 || al != 1 {
		t.Errorf("A: %d clientes, %d itens, %d calculados, %d alíquotas; quer 3, >=45, >0, 1", cl, it, calc, al)
	}

	clonar(b)
	if cl2, it2, _, _ := contar(t, s, ctx, b); cl2 != 3 || it2 != it {
		t.Errorf("B: %d clientes e %d itens; quer 3 e %d", cl2, it2, it)
	}
	if cl3, it3, _, _ := contar(t, s, ctx, usuarioModelo); cl3 != 3 || it3 != it {
		t.Errorf("modelo alterado: %d clientes e %d itens", cl3, it3)
	}
	// cópia com ids novos: nenhum cliente de A é do modelo
	var comuns int
	if err := s.pool.QueryRow(ctx, `select count(*) from clientes a join clientes m on m.id = a.id
		where a.user_id = $1 and m.user_id = $2`, a, usuarioModelo).Scan(&comuns); err != nil || comuns != 0 {
		t.Errorf("ids compartilhados com o modelo: %d (%v)", comuns, err)
	}
}
