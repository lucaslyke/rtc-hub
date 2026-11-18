package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"path/filepath"
	"strings"

	"github.com/jackc/pgx/v5"

	"rtchub/internal/importa"
	"rtchub/internal/store"
	"rtchub/internal/tabelas"
)

const maxUpload = 50 << 20

const semNBS = "Serviço sem NBS e sem correspondência na tabela da Receita"

type resultadoImportacao struct {
	ItensNovos       int                `json:"itensNovos"`
	ItensAtualizados int                `json:"itensAtualizados"`
	NotasRepetidas   int                `json:"notasRepetidas"`
	Ignorados        []importa.Ignorado `json:"ignorados"`
}

// opcoes é o que vai para itens.opcoes: as escolhas possíveis de IBS/CBS e de IS.
type opcoes struct {
	IBSCBS []tabelas.Opcao `json:"ibsCbs"`
	IS     []tabelas.Opcao `json:"is,omitempty"`
}

func planilha(nome string) bool {
	ext := strings.ToLower(filepath.Ext(nome))
	return ext == ".csv" || ext == ".xlsx"
}

// lerMultipart limita o corpo a 50 MB e traduz os erros para o usuário.
func lerMultipart(w http.ResponseWriter, r *http.Request) error {
	r.Body = http.MaxBytesReader(w, r.Body, maxUpload)
	if err := r.ParseMultipartForm(8 << 20); err != nil {
		var mb *http.MaxBytesError
		if errors.As(err, &mb) {
			return falha(http.StatusRequestEntityTooLarge, "O envio passa de 50 MB. Divida os arquivos em partes.")
		}
		return falha(http.StatusBadRequest, "Envio inválido: use multipart/form-data.")
	}
	return nil
}

func (s *Servidor) importar(w http.ResponseWriter, r *http.Request, uid string) error {
	cid, err := id(r)
	if err != nil {
		return err
	}
	if err := lerMultipart(w, r); err != nil {
		return err
	}
	defer r.MultipartForm.RemoveAll()
	arquivos := r.MultipartForm.File["arquivos[]"]
	if len(arquivos) == 0 {
		return falha(http.StatusBadRequest, "Nenhum arquivo enviado.")
	}
	var cli store.Cliente
	if err := s.tx(r, uid, func(tx pgx.Tx) (err error) { cli, err = store.ObterCliente(r.Context(), tx, cid); return }); err != nil {
		return err
	}

	res := resultadoImportacao{Ignorados: []importa.Ignorado{}}
	var notas []importa.Nota
	var soltos []importa.ItemBruto // itens de planilha, sem nota
	tipos := map[string]bool{}
	for _, fh := range arquivos {
		f, err := fh.Open()
		if err != nil {
			return err
		}
		if planilha(fh.Filename) {
			tipos["planilha"] = true
			m := importa.Mapeamento{Descricao: -1, Codigo: -1, NCM: -1, NBS: -1, Valor: -1} // coluna omitida ≠ coluna 0
			if err := json.Unmarshal([]byte(r.FormValue("mapeamento")), &m); err != nil {
				f.Close()
				return falha(http.StatusBadRequest, "Informe o mapeamento das colunas da planilha.")
			}
			its, ign, err := importa.LerPlanilha(fh.Filename, f, m)
			if err != nil {
				log.Printf("planilha %s: %v", fh.Filename, err)
				ign = append(ign, importa.Ignorado{Arquivo: fh.Filename, Motivo: "planilha ilegível (use CSV ou XLSX)"})
			}
			soltos = append(soltos, its...)
			res.Ignorados = append(res.Ignorados, ign...)
		} else {
			tipos["xml"] = true
			ns, ign := importa.LerArquivos(fh.Filename, f, cli.CNPJ)
			notas = append(notas, ns...)
			res.Ignorados = append(res.Ignorados, ign...)
		}
		f.Close()
	}

	// a mesma nota duas vezes no mesmo envio também é repetida
	var chaves []string
	unicas := notas[:0]
	vista := map[string]bool{}
	for _, n := range notas {
		switch {
		case n.Chave == "":
			res.Ignorados = append(res.Ignorados, importa.Ignorado{Arquivo: "nota do emitente " + n.EmitenteCNPJ, Motivo: "nota sem chave de acesso"})
		case vista[n.Chave]:
			res.NotasRepetidas++
		default:
			vista[n.Chave] = true
			chaves = append(chaves, n.Chave)
			unicas = append(unicas, n)
		}
	}

	tipo := "xml"
	if tipos["planilha"] {
		tipo = map[bool]string{true: "misto", false: "planilha"}[tipos["xml"]]
	}
	data := dataSugestao()
	err = s.tx(r, uid, func(tx pgx.Tx) error {
		ja, err := store.RegistrarNotas(r.Context(), tx, cid, chaves)
		if err != nil {
			return err
		}
		repetida := map[string]bool{}
		for _, k := range ja {
			repetida[k] = true
		}
		res.NotasRepetidas += len(ja)
		brutos := soltos
		for _, n := range unicas {
			if !repetida[n.Chave] {
				brutos = append(brutos, n.Itens...)
			}
		}
		cache := map[string]store.ItemEntrada{}
		entradas := make([]store.ItemEntrada, 0, len(brutos))
		for _, b := range brutos {
			chave := b.NCM + "|" + b.NBS + "|" + b.CTribNac
			e, ok := cache[chave]
			if !ok {
				if e, err = s.sugerir(r.Context(), b, data); err != nil {
					return err
				}
				cache[chave] = e
			}
			e.Codigo, e.Descricao, e.NCM, e.NBS, e.CTribNac, e.Valor = b.Codigo, b.Descricao, b.NCM, b.NBS, b.CTribNac, b.Valor
			entradas = append(entradas, e)
		}
		if res.ItensNovos, res.ItensAtualizados, err = store.UpsertItens(r.Context(), tx, cid, entradas); err != nil {
			return err
		}
		return store.RegistrarImportacao(r.Context(), tx, cid, tipo, len(arquivos), res.ItensNovos, res.ItensAtualizados, res.Ignorados)
	})
	if err != nil {
		return err
	}
	responder(w, 200, res)
	return nil
}

// sugerir transforma a Sugestao das tabelas na classificação inicial do item.
func (s *Servidor) sugerir(ctx context.Context, b importa.ItemBruto, data string) (store.ItemEntrada, error) {
	var sg tabelas.Sugestao
	var err error
	switch {
	case b.NBS != "":
		sg, err = s.Tabelas.Sugerir(ctx, tabelas.NBS, b.NBS, data)
	case b.NCM != "":
		sg, err = s.Tabelas.Sugerir(ctx, tabelas.NCM, b.NCM, data)
	default:
		return s.sugerirServico(ctx, b.CTribNac, data)
	}
	if err != nil {
		return store.ItemEntrada{}, err
	}
	e := store.ItemEntrada{Situacao: string(sg.Situacao), Condicao: sg.Motivo}
	op := opcoes{IBSCBS: sg.IBSCBS.Opcoes}
	if esc := sg.IBSCBS.Escolhida; esc != nil {
		e.CST, e.CClassTrib = esc.CST, esc.CClassTrib
	}
	if sg.IS != nil {
		op.IS = sg.IS.Opcoes
		if esc := sg.IS.Escolhida; esc != nil {
			e.ISCST, e.ISCClassTrib, e.Condicao = esc.CST, esc.CClassTrib, esc.Condicao
		}
	}
	e.Opcoes, err = json.Marshal(op)
	return e, err
}

// sugerirServico: NFS-e sem NBS → opções de todas as NBS que a Receita liga ao cTribNac.
// shortcut: ignora as opções de IS das NBS candidatas (serviços não têm IS hoje); somar se a Receita incluir.
func (s *Servidor) sugerirServico(ctx context.Context, cTribNac, data string) (store.ItemEntrada, error) {
	invalido := store.ItemEntrada{Situacao: string(tabelas.Invalido), Condicao: semNBS}
	c := tabelas.NormalizaCodigo(cTribNac)
	if len(c) != 6 || !reDigito.MatchString(c) {
		return invalido, nil
	}
	nbs, err := s.Tabelas.NBSPorServicoLC116(ctx, c, data)
	if err != nil {
		return store.ItemEntrada{}, err
	}
	var op opcoes
	visto := map[string]bool{}
	for _, n := range nbs {
		sg, err := s.Tabelas.Sugerir(ctx, tabelas.NBS, n, data)
		if err != nil {
			return store.ItemEntrada{}, err
		}
		for _, o := range sg.IBSCBS.Opcoes {
			if !visto[o.CClassTrib] {
				visto[o.CClassTrib] = true
				op.IBSCBS = append(op.IBSCBS, o)
			}
		}
	}
	if len(op.IBSCBS) == 0 {
		return invalido, nil
	}
	b, err := json.Marshal(op)
	return store.ItemEntrada{Situacao: string(tabelas.Revisar), Opcoes: b}, err
}

func (s *Servidor) previa(w http.ResponseWriter, r *http.Request, uid string) error {
	if _, err := id(r); err != nil {
		return err
	}
	if err := lerMultipart(w, r); err != nil {
		return err
	}
	defer r.MultipartForm.RemoveAll()
	fhs := r.MultipartForm.File["arquivo"]
	if len(fhs) == 0 || !planilha(fhs[0].Filename) {
		return falha(http.StatusBadRequest, "Envie uma planilha CSV ou XLSX.")
	}
	f, err := fhs[0].Open()
	if err != nil {
		return err
	}
	defer f.Close()
	p, err := importa.PreviaPlanilha(fhs[0].Filename, f)
	if err != nil {
		log.Printf("prévia %s: %v", fhs[0].Filename, err)
		return falha(http.StatusBadRequest, "Não foi possível ler a planilha (use CSV ou XLSX).")
	}
	responder(w, 200, p)
	return nil
}
