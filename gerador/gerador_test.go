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
		{"Escritório de advocacia Duarte & Lemos em Belo Horizonte, com equipe", "advocacia", "Duarte & Lemos", "Belo Horizonte", "", []string{"equipe"}, nil},
		{"Fotógrafa Lia Campos, só galeria e contato", "", "Lia Campos", "", "", []string{"galeria"}, []string{"blog"}},
		{"qualquer coisa", "generico", "Sua Empresa", "", "", []string{"servicos"}, nil},
	}
	for _, c := range casos {
		p := Entender(c.desc)
		if c.setor != "" && p.Setor.ID != c.setor {
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

func TestDadosDigitados(t *testing.T) {
	d := Dados{Tipo: "portal", Nome: "Folha do Sertão", Cidade: "Juazeiro", WhatsApp: "(74) 98888-7777", Email: "redacao@folha.com.br", Endereco: "Rua das Flores, 10 {{ x }}", Horario: "Todos os dias"}
	desc := DescricaoComDados("", d)
	if !strings.Contains(desc, `chamado "Folha do Sertão"`) || !strings.Contains(desc, "em Juazeiro") || !strings.Contains(desc, "portal") {
		t.Fatal(desc)
	}
	p := Entender(desc)
	UsarDados(p, d)
	if p.Setor.ID != "portal" || p.Nome != "Folha do Sertão" || p.WhatsApp != "https://wa.me/5574988887777" || p.Email != "redacao@folha.com.br" {
		t.Fatalf("%+v", p)
	}
	src := Montar(p)
	for _, s := range []string{"Folha do Sertão", "(74) 98888-7777", "redacao@folha.com.br", "Rua das Flores, 10 &#123;&#123; x }}", "Notícias"} {
		if !strings.Contains(src, s) {
			t.Errorf("faltou %q", s)
		}
	}
	if strings.Contains(src, "{{ x }}") {
		t.Error("texto digitado não pode virar código")
	}
	// e-mail inválido e tipo desconhecido são ignorados
	p2 := Entender("padaria")
	UsarDados(p2, Dados{Email: "não é e-mail", Tipo: "xyz"})
	if p2.Email != "" || p2.Setor.ID != "restaurante" {
		t.Fatalf("%+v", p2.Setor.ID)
	}
	if ps := TirarPerguntasRespondidas([]string{"Qual é o nome do seu negócio?", "Em que cidade?", "Que cores?"}, Dados{Nome: "X", Cidade: "Y"}); len(ps) != 1 {
		t.Fatal(ps)
	}
	if !strings.Contains(Entender("portal de notícias da cidade").Setor.ID, "portal") {
		t.Fatal("portal")
	}
}
