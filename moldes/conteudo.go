package moldes

import (
	"strings"
	"time"

	"github.com/Nanofylium/probable-plant/gerador"
)

// Conteúdo de partida dos moldes: entra no banco só na primeira vez e depois
// é editado (ou apagado) no painel.

type postSemente struct {
	Titulo, Cat, Resumo, Tags, Corpo, Data, Autor string
	Fixado                                        bool
}

type eventoSemente struct {
	Titulo, Data, Local, Resumo string
	Vagas                       int
}

func (d dados) local() string {
	if d.Cidade != "" {
		return d.Cidade
	}
	return "a cidade"
}

// Posts: os do ramo (pelas regras do gerador) e, no portal, mais algumas pautas.
func (d dados) Posts() []postSemente {
	ps := append([]gerador.Post{}, d.Setor.Posts...)
	if d.Molde.ID == "portal" && len(ps) < 6 {
		c := d.local()
		extra := []gerador.Post{
			{Titulo: "Agenda do fim de semana: o que fazer em " + c, Cat: "Cultura", Resumo: "Shows, feiras e passeios de graça ou quase, escolhidos pela redação.", Tags: "agenda, fim de semana"},
			{Titulo: "Guia: onde comer bem gastando pouco", Cat: "Guia", Resumo: "Dez endereços testados pela equipe, do café da manhã ao jantar.", Tags: "guia, comida"},
			{Titulo: "Coluna: a cidade que a gente quer", Cat: "Opinião", Resumo: "Calçadas, praças e transporte: o que dá para mudar já, sem grandes obras.", Tags: "opinião, cidade"},
		}
		ps = append(ps, extra[:6-len(ps)]...)
	}
	if d.Molde.ID == "blog" && len(ps) < 4 {
		ps = append(ps, gerador.Post{Titulo: "Por que comecei este blog", Cat: "Bastidores", Resumo: "O que você vai encontrar por aqui e a frequência dos textos.", Tags: "bastidores"})
	}
	autores := []string{"Equipe " + d.Nome}
	for _, p := range d.Setor.Equipe {
		autores = append(autores, p.Nome)
	}
	out := []postSemente{}
	hoje := time.Now()
	for i, p := range ps {
		out = append(out, postSemente{
			Titulo: p.Titulo, Cat: p.Cat, Resumo: p.Resumo, Tags: p.Tags,
			Corpo:  corpoPost(d, p),
			Data:   hoje.AddDate(0, 0, -3*i-1).Format("2006-01-02") + "T09:00",
			Autor:  autores[(i+1)%len(autores)],
			Fixado: i == 0,
		})
	}
	return out
}

func corpoPost(d dados, p gerador.Post) string {
	var b strings.Builder
	b.WriteString("Este é um texto de exemplo para você ver como o " + d.Nome + " fica com conteúdo de verdade. ")
	b.WriteString("Troque por um texto seu no painel: entre em /_admin, abra este item, edite e salve. A página muda na hora.\n\n")
	if d.Molde.ID != "portal" {
		b.WriteString("## O que você vai encontrar aqui\n\n")
		b.WriteString("- O passo a passo, do jeito que fazemos no dia a dia.\n")
		b.WriteString("- Os erros mais comuns e como evitar cada um.\n")
		b.WriteString("- Uma dica extra no final, para quem quer ir além.\n\n")
		b.WriteString("> Ficou alguma dúvida? Deixe um comentário aqui embaixo.\n")
		return b.String()
	}
	b.WriteString("## Em resumo\n\n")
	b.WriteString("- O que aconteceu e por que importa para quem mora em " + d.local() + ".\n")
	b.WriteString("- Quem está envolvido e o que vem depois.\n")
	b.WriteString("- Onde saber mais e como participar.\n\n")
	b.WriteString("> Tem uma sugestão sobre este assunto? Escreva para a gente pela página de contato.\n")
	return b.String()
}

// Eventos da agenda (portal).
func (d dados) Eventos() []eventoSemente {
	es := d.Setor.Eventos
	out := []eventoSemente{}
	hoje := time.Now()
	for i, e := range es {
		dias := e.Dias
		if dias <= 0 {
			dias = 7 * (i + 1)
		}
		hora := e.Hora
		if len(hora) != 5 {
			hora = "19:00"
		}
		out = append(out, eventoSemente{e.Titulo, hoje.AddDate(0, 0, dias).Format("2006-01-02") + " " + hora, e.Local, e.Resumo, e.Vagas})
	}
	if len(out) < 3 {
		out = append(out, eventoSemente{"Feira de produtores locais", hoje.AddDate(0, 0, 5).Format("2006-01-02") + " 08:00", "Praça central", "Frutas, queijos, pães e artesanato direto de quem produz.", 0})
	}
	return out
}

// Páginas da wiki.
type paginaSemente struct{ Titulo, Cat, Corpo string }

func (d dados) Paginas() []paginaSemente {
	n := d.Nome
	sobre := d.Descricao
	if sobre == "" {
		sobre = "Um lugar para juntar o que a gente sabe e deixar fácil de achar."
	}
	faq := "Respostas curtas para o que mais perguntam.\n\n"
	for _, f := range d.Setor.Faq {
		faq += "## " + f[0] + "\n\n" + f[1] + "\n\n"
	}
	faq += "Não achou? Veja o [[Glossário]] ou sugira uma página nova."
	return []paginaSemente{
		{"Página inicial", "Geral", "Bem-vindo à wiki **" + n + "**.\n\n" + sobre + "\n\n## Por onde começar\n\n- [[Sobre " + n + "]]\n- [[Como contribuir]]\n- [[Glossário]]\n- [[Perguntas comuns]]\n\nUse a busca no alto da página ou o Índice A–Z, no menu ao lado, para achar qualquer assunto."},
		{"Sobre " + n, "Geral", sobre + "\n\n## Quem mantém\n\nA equipe " + n + ", com ajuda de quem lê e sugere melhorias.\n\nVeja também: [[Como contribuir]]."},
		{"Como contribuir", "Ajuda", "Qualquer pessoa pode melhorar esta wiki.\n\n1. Abra a página que quer melhorar.\n2. Clique em **Sugerir edição** no fim da página.\n3. Escreva o texto novo e envie. A equipe revisa e publica.\n\n## Links entre páginas\n\nEscreva o nome de outra página entre colchetes duplos, assim: `[[Glossário]]`. Se a página ainda não existe, o link leva para sugerir que ela seja criada."},
		{"Glossário", "Referência", "Termos que aparecem por aqui.\n\n**Página**: um texto da wiki, com título e categoria.\n\n**Link**: o nome de uma página entre colchetes duplos.\n\n**Sugestão**: uma edição enviada por um visitante, que a equipe revisa.\n\nVeja também: [[Como contribuir]]."},
		{"Perguntas comuns", "Ajuda", faq},
	}
}
