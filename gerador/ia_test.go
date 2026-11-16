package gerador

import (
	"strings"
	"testing"
)

const respostaIA = "Aqui está:\n```json\n" + `{"nome": "Cacau & Coco", "ramo": "restaurante", "cidade": "Salvador", "cor": "#C2410C", "estilo": "divertido",
 "chapeu": "Doceria artesanal", "titulo": "Doce de verdade <script>alert(1)</script>", "texto": "Brigadeiros e bolos feitos no dia.",
 "chamada": "Encomende pelo WhatsApp", "servicos_titulo": "Cardápio",
 "servicos": [{"icone": "🍫", "titulo": "Brigadeiro gourmet", "texto": "Cento com 12 sabores.", "preco": "R$ 1.250,00"},
              {"icone": "🎂", "titulo": "Bolo de festa", "texto": "Massa amanteigada.", "preco": 180},
              {"icone": "🥥", "titulo": "Cocada de forno", "texto": "Receita da avó.", "preco": "12,50"}],
 "numeros": [{"numero": 15, "rotulo": "anos de doce"}, {"numero": "4.9", "rotulo": "nota no Google"}],
 "faq": [{"pergunta": "Entregam?", "resposta": "Sim, em Salvador."}, {"pergunta": "Tem sem lactose?", "resposta": "Tem, por encomenda."}],
 "depoimentos": [{"texto": "\"Melhor brigadeiro da Bahia\"", "autor": "Lia, Barra"}, {"texto": "Bolo lindo", "autor": "Rui, Pituba"}],
 "equipe": [{"nome": "Dona Cida", "cargo": "Confeiteira"}],
 "posts": [{"titulo": "Como guardar brigadeiro", "resumo": "Dicas rápidas."}],
 "secoes": ["catalogo", "faq", "blog", "inexistente"]}` + "\n```"

func TestAplicarIA(t *testing.T) {
	c, err := LerConteudo(respostaIA)
	if err != nil {
		t.Fatal(err)
	}
	p := Entender("doceria em Salvador sem blog")
	Aplicar(p, c)
	if p.Nome != "Cacau & Coco" || p.Setor.ID != "restaurante" || p.Cor != "#c2410c" || p.Fonte != "arredondada" {
		t.Fatalf("plano: %+v", p)
	}
	if len(p.Setor.Servicos) != 3 || p.Setor.Servicos[0].Preco != "1250.00" || p.Setor.Servicos[1].Preco != "180" || p.Setor.Servicos[2].Preco != "12.50" {
		t.Errorf("serviços: %+v", p.Setor.Servicos)
	}
	for _, m := range p.Modulos {
		if m == "blog" || m == "inexistente" {
			t.Errorf("seção %q não devia entrar (tirado ou inexistente): %v", m, p.Modulos)
		}
	}
	site := Montar(p)
	for _, want := range []string{"Cacau &amp; Coco", "Bolo de festa", "Cocada de forno", "Melhor brigadeiro da Bahia", "Tem sem lactose?"} {
		if !strings.Contains(site, want) {
			t.Errorf("site sem %q", want)
		}
	}
	if strings.Contains(site, "<script>alert(1)") {
		t.Error("texto da IA entrou sem escapar")
	}

	// lixo da IA não estraga o plano
	p2 := Entender("academia em Natal")
	antes := p2.Setor.Titulo
	c2, _ := LerConteudo(`{"cor": "vermelho", "ramo": "foguete", "servicos": [{"titulo": "só um"}], "titulo": ""}`)
	Aplicar(p2, c2)
	if p2.Setor.ID != "academia" || p2.Setor.Titulo != antes || len(p2.Setor.Servicos) < 2 || p2.Cor == "vermelho" {
		t.Errorf("IA com lixo mudou o plano: %+v", p2)
	}
	if _, err := LerConteudo("desculpe, não sei"); err == nil {
		t.Error("devia reclamar de resposta sem JSON")
	}
}

func TestPerguntasERespostas(t *testing.T) {
	q := PerguntasPorRegras("uma padaria")
	if len(q) < 3 || !strings.Contains(q[0], "nome") {
		t.Fatalf("perguntas: %v", q)
	}
	d := DescricaoComRespostas("uma padaria", [][2]string{{q[0], "Pão de Mel"}, {q[1], "Olinda"}, {q[3], "galeria e blog"}})
	p := Entender(d)
	if p.Nome != "Pão de Mel" || p.Cidade != "Olinda" || !p.Tem("galeria") || !p.Tem("blog") {
		t.Errorf("%q -> nome %q cidade %q mods %v", d, p.Nome, p.Cidade, p.Modulos)
	}
	if got := LerPerguntas(`{"perguntas": ["Qual o nome?", "", "Tem delivery?"]}`); len(got) != 2 {
		t.Errorf("LerPerguntas: %v", got)
	}
}

// Modelos pequenos (como o Granite 2B do motor local) erram o ramo, trocam a
// cor que a pessoa pediu, põem telefone em "números" e copiam a lista de seções.
func TestAplicarModeloPequeno(t *testing.T) {
	c, err := LerConteudo(`{"nome":"Pão de Olinda","ramo":"pousada","cor":"#FFD700",
	 "servicos":[{"titulo":"Pães","preco":"A partir de R$ 3,00"},{"titulo":"Bolos","preco":"R$ 1.200,50"}],
	 "numeros":[{"numero":"39922-3000","rotulo":"Telefone"},{"numero":"20","rotulo":"anos de casa"},{"numero":"98%","rotulo":"clientes felizes"}],
	 "secoes":["catalogo","agenda","eventos","planos","equipe","galeria","depoimentos","blog","forum","downloads","vagas","faq"]}`)
	if err != nil {
		t.Fatal(err)
	}
	p := Entender("padaria artesanal em Olinda, cores marrom")
	antes := append([]string(nil), p.Modulos...)
	Aplicar(p, c)
	if p.Setor.ID != "restaurante" || p.Cor == "#ffd700" {
		t.Errorf("ramo/cor da pessoa trocados pela IA: %s %s", p.Setor.ID, p.Cor)
	}
	if got := p.Setor.Servicos; got[0].Preco != "3.00" || got[1].Preco != "1200.50" {
		t.Errorf("preços: %+v", got)
	}
	if len(p.Setor.Numeros) != 2 || p.Setor.Numeros[0][0] != "20" {
		t.Errorf("números: %v", p.Setor.Numeros)
	}
	if strings.Join(p.Modulos, ",") != strings.Join(antes, ",") {
		t.Errorf("seções copiadas da lista entraram: %v (antes %v)", p.Modulos, antes)
	}
}

func TestCardapioComPrecosNaoViraPlanos(t *testing.T) {
	p := Entender("padaria em Olinda, quero cardápio com preços e depoimentos")
	if p.Tem("planos") || !p.Tem("catalogo") {
		t.Errorf("módulos: %v", p.Modulos)
	}
	if p := Entender("academia com tabela de preços dos planos e loja"); !p.Tem("planos") {
		t.Errorf("planos pedidos sumiram: %v", p.Modulos)
	}
}

func TestLerConteudoTolerante(t *testing.T) {
	c, err := LerConteudo(`{"nome":"Pão de Olinda","secoes":[{"id":"faq"},"galeria"],"cor":7,
	 "faq":[{"pergunta":"Abre domingo?","resposta":"Sim"},{"pergunta":["x"]}]}`)
	if err != nil {
		t.Fatal(err)
	}
	if c.Nome != "Pão de Olinda" || strings.Join(c.Secoes, ",") != "galeria" || c.Cor != "" || len(c.Faq) != 1 {
		t.Errorf("%+v", c)
	}
}
