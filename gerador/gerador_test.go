package gerador

import (
	"strings"
	"testing"
)

func TestEntender(t *testing.T) {
	casos := []struct {
		desc, setor, nome, cidade, cor string
		tem, naoTem                    []string
	}{
		{"uma escola de música chamada Sol Maior em Salvador, com blog, eventos e wiki, azul, elegante",
			"escola", "Sol Maior", "Salvador", "#1d4ed8", []string{"blog", "eventos", "wiki", "servicos"}, []string{"catalogo", "idiomas"}},
		{"só loja e contato, chamada Doce Encanto", "loja", "Doce Encanto", "", "", []string{"catalogo"}, []string{"blog", "faq", "depoimentos"}},
		{"associação de moradores do Jardim Europa sem blog", "associacao", "Associação de moradores do Jardim Europa", "", "", []string{"eventos", "downloads"}, []string{"blog"}},
		{"restaurante italiano chamado Nonna Rosa em Belo Horizonte com cardápio e reservas, vermelho", "restaurante", "Nonna Rosa", "Belo Horizonte", "#b91c1c", []string{"catalogo", "agenda"}, []string{"idiomas"}},
		{"academia de crossfit sem planos e sem equipe", "academia", "", "", "", []string{"servicos", "agenda"}, []string{"planos", "equipe"}},
		{"pousada \"Maré Mansa\" com galeria e site em inglês e espanhol", "hotel", "Maré Mansa", "", "", []string{"galeria", "agenda", "idiomas"}, nil},
		{"qualquer coisa", "generico", "Sua Empresa", "", "", []string{"servicos"}, nil},
	}
	for _, c := range casos {
		p := Entender(c.desc)
		if p.Setor.ID != c.setor {
			t.Errorf("%q: setor %s, queria %s", c.desc, p.Setor.ID, c.setor)
		}
		if c.nome != "" && p.Nome != c.nome {
			t.Errorf("%q: nome %q, queria %q", c.desc, p.Nome, c.nome)
		}
		if p.Cidade != c.cidade {
			t.Errorf("%q: cidade %q, queria %q", c.desc, p.Cidade, c.cidade)
		}
		if c.cor != "" && p.Cor != c.cor {
			t.Errorf("%q: cor %s, queria %s", c.desc, p.Cor, c.cor)
		}
		for _, m := range c.tem {
			if !p.Tem(m) {
				t.Errorf("%q: faltou %s (tem %v)", c.desc, m, p.Modulos)
			}
		}
		for _, m := range c.naoTem {
			if p.Tem(m) {
				t.Errorf("%q: não devia ter %s", c.desc, m)
			}
		}
	}
	if p := Entender("hotel com site em inglês e francês"); strings.Join(p.Idiomas, ",") != "en,fr" {
		t.Errorf("idiomas: %v", p.Idiomas)
	}
}

func TestSoContato(t *testing.T) {
	if p := Entender("só uma página de contato, chamada Mínimo"); len(p.Modulos) != 0 {
		t.Errorf("só contato virou %v", p.Modulos)
	}
}
