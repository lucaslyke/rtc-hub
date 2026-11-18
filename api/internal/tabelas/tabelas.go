// Package tabelas sugere CST/cClassTrib a partir das tabelas oficiais (SQLite) da calculadora RTC.
package tabelas

import (
	"database/sql"
	"strings"

	_ "modernc.org/sqlite"
)

type Tipo string // "NCM" | "NBS"

const (
	NCM Tipo = "NCM"
	NBS Tipo = "NBS"
)

type Situacao string // "ok" | "provavel" | "revisar" | "invalido"

const (
	Ok       Situacao = "ok"
	Provavel Situacao = "provavel"
	Revisar  Situacao = "revisar"
	Invalido Situacao = "invalido"
)

type Opcao struct {
	CST, CClassTrib, Descricao, Fundamentacao, Condicao string
	PercReducao                                         float64
}

type Classificacao struct {
	Situacao  Situacao
	Escolhida *Opcao
	Opcoes    []Opcao
}

type Sugestao struct {
	Situacao Situacao // a pior entre IBSCBS e IS
	Motivo   string   // preenchido quando invalido
	IBSCBS   Classificacao
	IS       *Classificacao // nil quando o NCM não tem IS
}

type Tabelas struct{ db *sql.DB }

// Abrir abre o banco oficial somente leitura.
func Abrir(path string) (*Tabelas, error) {
	db, err := sql.Open("sqlite", "file:"+path+"?mode=ro")
	if err != nil {
		return nil, err
	}
	if err := db.Ping(); err != nil {
		db.Close()
		return nil, err
	}
	return &Tabelas{db: db}, nil
}

func (t *Tabelas) Close() error { return t.db.Close() }

// NormalizaCodigo tira pontos, espaços e traços.
func NormalizaCodigo(s string) string {
	return strings.NewReplacer(".", "", " ", "", "-", "", "\t", "").Replace(s)
}
