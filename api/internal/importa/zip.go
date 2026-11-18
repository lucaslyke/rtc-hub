package importa

import (
	"archive/zip"
	"bytes"
	"io"
	"strings"
)

const (
	maxArquivosZip = 2000
	maxBytesZip    = 50 << 20
)

// LerArquivos aceita .xml e .zip; notas de outro emitente que não cnpjCliente vão para Ignorado.
func LerArquivos(nome string, r io.Reader, cnpjCliente string) ([]Nota, []Ignorado) {
	var notas []Nota
	var ign []Ignorado
	add := func(arq string, r io.Reader) {
		n, err := LerNota(arq, r)
		switch {
		case err != nil:
			ign = append(ign, Ignorado{arq, err.Error()})
		case n.EmitenteCNPJ != cnpjCliente:
			ign = append(ign, Ignorado{arq, "nota emitida por " + n.EmitenteCNPJ + ", não por este cliente (nota de entrada?)"})
		default:
			notas = append(notas, n)
		}
	}
	lower := strings.ToLower(nome)
	switch {
	case strings.HasSuffix(lower, ".xml"):
		add(nome, io.LimitReader(r, maxBytesZip))
	case strings.HasSuffix(lower, ".zip"):
		b, err := io.ReadAll(io.LimitReader(r, maxBytesZip+1))
		if err != nil || len(b) > maxBytesZip {
			return nil, []Ignorado{{nome, "ZIP grande demais ou ilegível"}}
		}
		zr, err := zip.NewReader(bytes.NewReader(b), int64(len(b)))
		if err != nil {
			return nil, []Ignorado{{nome, "ZIP inválido"}}
		}
		if len(zr.File) > maxArquivosZip {
			return nil, []Ignorado{{nome, "ZIP com mais de 2000 arquivos"}}
		}
		restante := int64(maxBytesZip)
		for _, f := range zr.File {
			el := strings.ToLower(f.Name)
			switch {
			case strings.HasSuffix(el, ".zip"):
				ign = append(ign, Ignorado{f.Name, "ZIP dentro de ZIP não é suportado"})
				continue
			case f.FileInfo().IsDir() || !strings.HasSuffix(el, ".xml"):
				ign = append(ign, Ignorado{f.Name, "não é XML"})
				continue
			}
			rc, err := f.Open()
			if err != nil {
				ign = append(ign, Ignorado{f.Name, "XML inválido"})
				continue
			}
			dados, err := io.ReadAll(io.LimitReader(rc, restante+1))
			rc.Close()
			restante -= int64(len(dados))
			if restante < 0 {
				ign = append(ign, Ignorado{f.Name, "limite de 50 MB descompactados excedido"}, Ignorado{nome, "demais arquivos do ZIP não lidos (limite de 50 MB)"})
				break
			}
			if err != nil {
				ign = append(ign, Ignorado{f.Name, "XML inválido"})
				continue
			}
			add(f.Name, bytes.NewReader(dados))
		}
	default:
		ign = append(ign, Ignorado{nome, "não é XML"})
	}
	return notas, ign
}
