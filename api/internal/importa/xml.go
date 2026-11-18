// Package importa lê XMLs fiscais (NF-e, NFC-e, NFS-e nacional) e extrai os itens.
package importa

import (
	"bytes"
	"encoding/xml"
	"errors"
	"io"
	"strconv"
	"strings"
)

type ItemBruto struct {
	Codigo, Descricao, NCM, NBS, CTribNac string
	Valor                                 float64
	Origem                                string // "nfe"|"nfse"|"planilha"
}

type Nota struct {
	Chave, EmitenteCNPJ string
	Itens               []ItemBruto
}

type Ignorado struct {
	Arquivo string `json:"arquivo"`
	Motivo  string `json:"motivo"`
}

var (
	ErrNaoNota     = errors.New("não é nota fiscal")
	ErrXMLInvalido = errors.New("XML inválido")
)

type infNFe struct {
	ID   string `xml:"Id,attr"`
	Emit struct {
		CNPJ string `xml:"CNPJ"`
	} `xml:"emit"`
	Det []struct {
		Prod struct {
			CProd string `xml:"cProd"`
			XProd string `xml:"xProd"`
			NCM   string `xml:"NCM"`
			VProd string `xml:"vProd"`
		} `xml:"prod"`
	} `xml:"det"`
}

type docNFe struct {
	InfNFe infNFe `xml:"infNFe"`
	NFe    struct {
		InfNFe infNFe `xml:"infNFe"`
	} `xml:"NFe"`
}

type docNFSe struct {
	Inf struct {
		ID   string `xml:"Id,attr"`
		Emit struct {
			CNPJ string `xml:"CNPJ"`
		} `xml:"emit"`
		DPS struct {
			Inf struct {
				Serv struct {
					CServ struct {
						CTribNac string `xml:"cTribNac"`
						CNBS     string `xml:"cNBS"`
						XDesc    string `xml:"xDescServ"`
					} `xml:"cServ"`
				} `xml:"serv"`
				Valores struct {
					VServPrest struct {
						VServ string `xml:"vServ"`
					} `xml:"vServPrest"`
				} `xml:"valores"`
			} `xml:"infDPS"`
		} `xml:"DPS"`
	} `xml:"infNFSe"`
}

func valor(s string) (float64, error) {
	return strconv.ParseFloat(strings.TrimSpace(s), 64)
}

// LerNota detecta NF-e/NFC-e (nfeProc ou NFe) e NFS-e nacional (NFSe) pelo elemento raiz.
func LerNota(nome string, r io.Reader) (Nota, error) {
	b, err := io.ReadAll(r)
	if err != nil {
		return Nota{}, err
	}
	var raiz string
	dec := xml.NewDecoder(bytes.NewReader(b))
	for raiz == "" {
		t, err := dec.Token()
		if err != nil {
			return Nota{}, ErrXMLInvalido
		}
		if se, ok := t.(xml.StartElement); ok {
			raiz = se.Name.Local
		}
	}
	switch raiz {
	case "nfeProc", "NFe":
		var d docNFe
		if err := xml.Unmarshal(b, &d); err != nil {
			return Nota{}, ErrXMLInvalido
		}
		inf := d.InfNFe
		if raiz == "nfeProc" {
			inf = d.NFe.InfNFe
		}
		n := Nota{Chave: strings.TrimPrefix(inf.ID, "NFe"), EmitenteCNPJ: inf.Emit.CNPJ}
		for _, det := range inf.Det {
			v, err := valor(det.Prod.VProd)
			if err != nil {
				return Nota{}, ErrXMLInvalido
			}
			n.Itens = append(n.Itens, ItemBruto{Codigo: det.Prod.CProd, Descricao: det.Prod.XProd, NCM: det.Prod.NCM, Valor: v, Origem: "nfe"})
		}
		return n, nil
	case "NFSe":
		var d docNFSe
		if err := xml.Unmarshal(b, &d); err != nil {
			return Nota{}, ErrXMLInvalido
		}
		i := d.Inf
		v, err := valor(i.DPS.Inf.Valores.VServPrest.VServ)
		if err != nil {
			return Nota{}, ErrXMLInvalido
		}
		c := i.DPS.Inf.Serv.CServ
		return Nota{Chave: strings.TrimPrefix(i.ID, "NFS"), EmitenteCNPJ: i.Emit.CNPJ, Itens: []ItemBruto{
			{Codigo: c.CTribNac, CTribNac: c.CTribNac, NBS: c.CNBS, Descricao: c.XDesc, Valor: v, Origem: "nfse"},
		}}, nil
	}
	return Nota{}, ErrNaoNota
}
