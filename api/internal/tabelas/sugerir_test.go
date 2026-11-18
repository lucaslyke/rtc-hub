package tabelas

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// Fixtures descobertas em .data/calculadora-pro.db para a data 2027-01-01.
// Candidatos: NCM_APLICAVEL n JOIN CLASSIFICACAO_TRIBUTARIA ct ON ct.CLTR_ID = n.NCMA_CLTR_ID
//
//	WHERE n.NCMA_NCM_CD = substr(:ncm, 1, length(n.NCMA_NCM_CD)), ambos vigentes,
//	sem EXCECAO_NCM_APLICAVEL vigente (ENCM_NCMA_ID = n.NCMA_ID, ENCM_NCM_CD prefixo de :ncm);
//	grupo IS quando TRIBUTO_SITUACAO_TRIBUTARIA.TRST_TBTO_ID = 1 para CLTR_SITR_ID.
const (
	data = "2027-01-01"
	// (a) GROUP BY ncm HAVING count(distinct cClassTrib IBSCBS) = 1 AND count(IS) = 0 -> 200003
	ncmUmTratamento = "02011000"
	// (b) HAVING count(distinct cClassTrib IBSCBS) >= 2 -> 200038, 515001.
	// 200038 tem 4 linhas em TIPO_DFE_CLASSIFICACAO (NF-e, NFC-e, NFS-e, Duimp).
	ncmAmbiguo = "01022110"
	// (d) EXCECAO_NCM_APLICAVEL de 8 dígitos vigente cujo cClassTrib não volta por nenhuma
	// outra linha de NCM_APLICAVEL sem exceção -> 11010010 perde 200034.
	ncmExcecao      = "11010010"
	cClassExcetuado = "200034"
	// Sem nenhuma linha em NCM_APLICAVEL.
	ncmComum = "84713012"
	// 2203 está ligado às 22 cClassTrib do IS (SITR 19..25).
	ncmCerveja = "22030000"
	// CLASSIF_NBS_INDOP_LC com CNIL_LSLC_CD = '1.01' vigente -> 11 NBS.
	cTribNacProgramacao = "010101"
)

func abrirTeste(t *testing.T) *Tabelas {
	t.Helper()
	_, arq, _, _ := runtime.Caller(0)
	db := filepath.Join(filepath.Dir(arq), "../../../.data/calculadora-pro.db")
	if _, err := os.Stat(db); err != nil {
		t.Skip("banco oficial ausente:", db)
	}
	tb, err := Abrir(db)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { tb.Close() })
	return tb
}

func sugerir(t *testing.T, tipo Tipo, codigo string) Sugestao {
	t.Helper()
	s, err := abrirTeste(t).Sugerir(context.Background(), tipo, codigo, data)
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func TestNormalizaNCM(t *testing.T) {
	if got := NormalizaCodigo(" 1006.30.21 "); got != "10063021" {
		t.Fatalf("NormalizaCodigo = %q", got)
	}
	s := sugerir(t, NCM, "100630")
	if s.Situacao != "invalido" || !strings.Contains(s.Motivo, "incompleto") {
		t.Fatalf("got %+v", s)
	}
}

func TestNCMSemTratamento(t *testing.T) {
	s := sugerir(t, NCM, ncmComum)
	if s.Situacao != "ok" || s.IBSCBS.Escolhida == nil || s.IBSCBS.Escolhida.CClassTrib != "000001" || s.IS != nil {
		t.Fatalf("got %+v", s)
	}
	if s.IBSCBS.Escolhida.CST != "000" || s.IBSCBS.Escolhida.Descricao == "" {
		t.Fatalf("escolhida %+v", s.IBSCBS.Escolhida)
	}
}

func TestNCMUmTratamento(t *testing.T) {
	s := sugerir(t, NCM, ncmUmTratamento)
	c := s.IBSCBS
	if s.Situacao != "provavel" || len(c.Opcoes) != 2 || c.Opcoes[1].CClassTrib != "000001" {
		t.Fatalf("got %+v", s)
	}
	if c.Escolhida == nil || c.Escolhida.CClassTrib != "200003" || c.Escolhida.Fundamentacao == "" || c.Escolhida.PercReducao != 100 {
		t.Fatalf("escolhida %+v", c.Escolhida)
	}
}

func TestNCMAmbiguo(t *testing.T) {
	s := sugerir(t, NCM, ncmAmbiguo)
	if s.Situacao != "revisar" || s.IBSCBS.Escolhida != nil || len(s.IBSCBS.Opcoes) < 3 {
		t.Fatalf("got %+v", s)
	}
}

func TestCondicaoTipoDFE(t *testing.T) {
	s := sugerir(t, NCM, ncmAmbiguo)
	for _, o := range s.IBSCBS.Opcoes {
		if o.CClassTrib == "200038" {
			if !strings.HasPrefix(o.Condicao, "Só em: ") || !strings.Contains(o.Condicao, "NF-e") {
				t.Fatalf("condicao %q", o.Condicao)
			}
			return
		}
	}
	t.Fatal("200038 ausente")
}

func TestExcecao(t *testing.T) {
	s := sugerir(t, NCM, ncmExcecao)
	for _, o := range s.IBSCBS.Opcoes {
		if o.CClassTrib == cClassExcetuado {
			t.Fatalf("%s deveria estar excetuado: %+v", cClassExcetuado, s)
		}
	}
}

func TestCervejaTemIS(t *testing.T) {
	s := sugerir(t, NCM, ncmCerveja)
	if s.IS == nil || s.IS.Situacao != "provavel" || s.IS.Escolhida == nil || s.IS.Escolhida.CClassTrib != "000001" {
		t.Fatalf("got %+v", s.IS)
	}
	if !strings.Contains(s.IS.Escolhida.Condicao, "primeiro fornecimento") || s.IS.Escolhida.CST != "000" {
		t.Fatalf("escolhida %+v", s.IS.Escolhida)
	}
	if len(s.IS.Opcoes) != 22 {
		t.Fatalf("opcoes IS = %d", len(s.IS.Opcoes))
	}
	// IBSCBS sem tratamento (ok), IS provavel -> pior = provavel
	if s.IBSCBS.Situacao != "ok" || s.Situacao != "provavel" {
		t.Fatalf("got %+v", s)
	}
}

func TestNCMInexistente(t *testing.T) {
	if s := sugerir(t, NCM, "99999999"); s.Situacao != "invalido" {
		t.Fatalf("got %+v", s)
	}
}

func TestNBSPorServico(t *testing.T) {
	nbs, err := abrirTeste(t).NBSPorServicoLC116(context.Background(), cTribNacProgramacao, data)
	if err != nil || len(nbs) == 0 {
		t.Fatalf("nbs=%v err=%v", nbs, err)
	}
}
