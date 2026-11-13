package gerador

import (
	"fmt"
	"strings"
)

type link struct{ txt, url string }

// menu principal: até 5 seções + contato; o resto vai para o rodapé.
func (g *ger) secoes() []link {
	var l []link
	add := func(m, txt, u string) {
		if g.tem(m) {
			l = append(l, link{txt, u})
		}
	}
	add("servicos", "«servR»", "«servU»")
	add("catalogo", "«lojaR»", "«lojaU»")
	add("eventos", "Eventos", "/eventos")
	add("blog", "«blogR»", "«blogU»")
	add("wiki", "Wiki", "/wiki")
	add("forum", "Fórum", "/forum")
	add("planos", "Planos", "/#planos")
	add("equipe", "«equipeR»", "«equipeU»")
	add("galeria", "Galeria", "/galeria")
	add("downloads", "Downloads", "/downloads")
	return l
}

func a(l link, extra string) string {
	return `<a href="{{ url('` + l.url + `') }}"` + extra + `>` + l.txt + `</a>`
}

func (g *ger) html() string {
	var b strings.Builder
	w := func(x string) { b.WriteString(x) }
	w(`
<html lang="pt-BR">
<head>
  <meta charset="utf-8">
  <title>{{ page.title }} · «nome»</title>
  <style>
` + g.css() + `  </style>
</head>
<body>
`)
	// faixa do topo
	w(`  <div class="topo-faixa">
    <span editavel="topo.texto">` + g.faixa() + `</span>
    <span class="topo-acoes">`)
	if g.tem("idiomas") {
		w(`<if test="ia_ligada()"><span class="idiomas"><a href="?idioma=pt">PT</a>`)
		for _, c := range g.p.Idiomas {
			w(`<a href="?idioma=` + c + `">` + strings.ToUpper(c) + `</a>`)
		}
		w(`</span></if>`)
	}
	w(`<a href="tel:+5500000000000" editavel="topo.fone">(00) 0000-0000</a></span>
  </div>
  <header>
    <a class="marca" href="{{ url('/') }}"><span>«emoji»</span><b editavel="marca">«nome»</b></a>
    <nav menu-editavel="principal">
      <a href="{{ url('/') }}">Início</a>
`)
	sec := g.secoes()
	for i, l := range sec {
		if i < 5 {
			w("      " + a(l, "") + "\n")
		}
	}
	w("      <a href=\"{{ url('/contato') }}\">Contato</a>\n")
	w("    </nav>\n")
	if g.tem("catalogo") {
		w(`    <a class="carrinho-topo" href="{{ url('/carrinho') }}" aria-label="Carrinho">🛒 <b>{{ itens_carrinho() }}</b></a>
`)
	}
	if g.tem("agenda") {
		w(`    <a class="botao pequeno" href="{{ url('«agU»') }}">«agV»</a>
`)
	}
	w("  </header>\n\n  <main>\n")
	w(g.inicio())
	w(g.paginasModulos())
	w(g.paginasFixas())
	w("  </main>\n")
	w(g.rodape())
	w(`
  <if test="ia_ligada()">{{ lowii() }}</if>
</body>
</html>
`)
	return b.String()
}

func (g *ger) faixa() string {
	if g.p.Cidade != "" {
		return "Atendimento em " + esc(g.p.Cidade) + " e região · Segunda a sábado"
	}
	return "Segunda a sábado, das 8h às 18h"
}

// ---------------------------------------------------------------- página inicial

func (g *ger) inicio() string {
	s := g.s
	var b strings.Builder
	w := func(x string) { b.WriteString(x) }
	w(`    <!-- ================= início ================= -->
    <page route="/" title="Início" description="` + esc(s.Texto) + `">
      <section class="capa">
        <div class="lado">
          <div>
            <span class="chapeu" editavel="capa.chapeu">` + esc(s.Chapeu) + `</span>
            <h1 editavel="capa.titulo">` + esc(s.Titulo) + `</h1>
            <p editavel="capa.texto">` + esc(s.Texto) + `</p>
            <div class="acoes">
              <a class="botao" href="{{ url('«ctaU»') }}">«cta»</a>
              <a class="botao secundario" href="{{ url('«cta2U»') }}">«cta2»</a>
            </div>
          </div>
          <div class="ilustracao" nep-inclinar="8" aria-hidden="true"><span>«emoji»</span></div>
        </div>
      </section>
`)
	if g.tem("servicos") {
		w(`
      <section class="secao alt" id="servicos">
        <div class="centro">
          <span class="chapeu">O que oferecemos</span>
          <h2 editavel="servicos.titulo">«servR»</h2>
          <p class="intro" editavel="servicos.intro">«servI»</p>
        </div>
        <div class="grade` + quatro(len(s.Servicos)) + `">
          <each s in="servicos |> take(6)">
            <a class="cartao servico" href="{{ url('«servU»/' + s.slug) }}">
              <div class="icone">{{ default(s.icone, "✦") }}</div>
              <h3>{{ tr(s.titulo) }}</h3>
              <p>{{ tr(default(s.resumo, "")) }}</p>
              <if test="s.preco"><span class="preco">a partir de {{ money(s.preco) }}</span></if>
            </a>
          </each>
        </div>
      </section>
`)
	}
	// números
	w(`
      <section class="secao">
        <div class="numeros">
`)
	for i, n := range s.Numeros {
		fmt.Fprintf(&b, "          <div><strong nep-contar editavel=\"num%d\">%s</strong><span editavel=\"num%d.txt\">%s</span></div>\n", i+1, esc(n[0]), i+1, esc(n[1]))
	}
	w("        </div>\n      </section>\n")

	if g.tem("catalogo") {
		w(`
      <section class="secao alt">
        <div class="centro">
          <span class="chapeu">Peça pelo site</span>
          <h2>Em destaque</h2>
        </div>
        <div class="grade quatro">
          <each p in="produtos |> take(4)">` + produtoCartao + `
          </each>
        </div>
        <p class="centro"><a class="botao secundario" href="{{ url('«lojaU»') }}">Ver tudo</a></p>
      </section>
`)
	}
	if g.tem("eventos") {
		w(`
      <section class="secao">
        <span class="chapeu">Agenda</span>
        <h2>Próximos eventos</h2>
        <div class="grade">
          <each e in="proximos_eventos |> take(3)">` + eventoCartao + `
          <else>
            <p>Nenhum evento marcado. Volte em breve!</p>
          </each>
        </div>
      </section>
`)
	}
	if g.tem("planos") {
		w(g.planos())
	}
	if g.tem("equipe") {
		w(`
      <section class="secao">
        <div class="centro">
          <span class="chapeu">Quem cuida de você</span>
          <h2>«equipeR»</h2>
        </div>
        <div class="grade">
          <each pe in="db.all('equipe') |> reverse |> take(3)">` + pessoaCartao + `
          </each>
        </div>
        <p class="centro"><a class="botao secundario" href="{{ url('«equipeU»') }}">Conhecer todos</a></p>
      </section>
`)
	}
	if g.tem("galeria") {
		w(`
      <section class="secao alt">
        <span class="chapeu">Galeria</span>
        <h2>Um pouco do nosso dia a dia</h2>
        <div class="fotos">
          <each f in="db.all('galeria') |> take(6)">` + fotoCartao + `
          </each>
        </div>
        <p class="centro"><a class="botao secundario" href="{{ url('/galeria') }}">Ver a galeria</a></p>
      </section>
`)
	}
	if g.tem("depoimentos") {
		w(`
      <section class="secao">
        <div class="centro">
          <span class="chapeu">Quem já conhece</span>
          <h2>O que dizem</h2>
        </div>
        <div class="grade">
`)
		for i, d := range s.Depo {
			fmt.Fprintf(&b, "          <blockquote class=\"cartao depoimento\"><span class=\"estrelas\">★★★★★</span><p editavel=\"depo%d\">“%s”</p><footer editavel=\"depo%d.autor\">%s</footer></blockquote>\n", i+1, esc(d[0]), i+1, esc(d[1]))
		}
		w("        </div>\n      </section>\n")
	}
	if g.tem("blog") {
		w(`
      <section class="secao alt">
        <span class="chapeu">«blogR»</span>
        <h2>Últimas publicações</h2>
        <div class="grade">
          <each p in="db.all('posts', 3)">` + postCartao + `
          <else>
            <div class="cartao"><h3>Nada publicado ainda</h3><p>Entre no <a href="{{ url('/_admin') }}">painel</a> e escreva o primeiro.</p></div>
          </each>
        </div>
      </section>
`)
	}
	if g.tem("wiki") || g.tem("forum") {
		w(`
      <section class="secao">
        <span class="chapeu">Comunidade</span>
        <h2>Feito por muita gente</h2>
        <div class="grade duas">
`)
		if g.tem("wiki") {
			w(`          <div class="cartao">
            <div class="icone">📖</div>
            <h3>Wiki</h3>
            <p>Atualizadas há pouco:</p>
            <ul class="lista-simples">
              <each p in="wiki_recentes() |> take(5)"><li><a href="{{ url('/wiki/' + p.slug) }}">{{ p.titulo }}</a></li></each>
            </ul>
            <a class="botao pequeno" href="{{ url('/wiki') }}">Abrir a wiki</a>
          </div>
`)
		}
		if g.tem("forum") {
			w(`          <div class="cartao">
            <div class="icone">💬</div>
            <h3>Fórum</h3>
            <p>Conversas recentes:</p>
            <ul class="lista-simples">
              <each t in="db.all('topicos', 5)"><li><a href="{{ url('/forum/' + t.slug) }}">{{ t.titulo }}</a></li></each>
            </ul>
            <a class="botao pequeno" href="{{ url('/forum') }}">Entrar no fórum</a>
          </div>
`)
		}
		w("        </div>\n      </section>\n")
	}
	if g.tem("faq") {
		w(`
      <section class="secao alt">
        <div class="estreito">
          <span class="chapeu">Dúvidas</span>
          <h2>Perguntas frequentes</h2>
          <each q in="perguntas |> take(5)">
            <details><summary>{{ q.titulo }}</summary><p>{{ q.resposta }}</p></details>
          </each>
          <p><a href="{{ url('/perguntas-frequentes') }}">Ver todas as perguntas →</a></p>
        </div>
      </section>
`)
	}
	w(`
      <section class="capa cor centro">
        <h2 editavel="chamada.titulo" style="margin-top:0">` + esc(s.Chamada) + `</h2>
        <p editavel="chamada.texto">Respondemos rápido, por WhatsApp ou e-mail.</p>
        <div class="acoes" style="justify-content:center"><a class="botao" href="{{ url('«ctaU»') }}">«cta»</a></div>
      </section>
    </page>
`)
	return b.String()
}

const produtoCartao = `
            <div class="cartao produto">
              <a class="foto-produto" href="{{ url('«lojaU»/' + p.slug) }}">
                <if test="p.foto"><img src="{{ p.foto.url }}" alt="{{ p.titulo }}" loading="lazy"><else><span>{{ default(p.icone, "🛍️") }}</span></if>
              </a>
              <span class="meta">{{ p.categoria }}</span>
              <h3><a href="{{ url('«lojaU»/' + p.slug) }}">{{ tr(p.titulo) }}</a></h3>
              <p class="preco">{{ money(p.preco) }}</p>
              <form run="adicionar" success="" class="comprar">
                <input type="hidden" name="produto" value="{{ p.slug }}">
                <input type="hidden" name="qtd" value="1">
                <button class="pequeno">Adicionar</button>
              </form>
            </div>`

const eventoCartao = `
            <a class="cartao evento" href="{{ url('/eventos/' + e.slug) }}">
              <div class="data-ev"><b>{{ dia_ev(e) }}</b><span>{{ mes_ev(e) }}</span></div>
              <div>
                <h3>{{ tr(e.titulo) }}</h3>
                <p class="meta">{{ default(e.hora, "") }} · {{ default(e.local, "") }}</p>
                <p>{{ tr(default(e.resumo, "")) }}</p>
                <if test="restam(e) != nil"><span class="selo">{{ restam(e) }} vagas</span></if>
              </div>
            </a>`

const pessoaCartao = `
            <div class="cartao pessoa">
              <div class="foto"><if test="pe.foto"><img src="{{ pe.foto.url }}" alt="{{ pe.titulo }}"><else>{{ inicial(pe.titulo) }}</if></div>
              <h3>{{ pe.titulo }}</h3>
              <p class="meta">{{ default(pe.cargo, "") }}</p>
              <p>{{ default(pe.resumo, "") }}</p>
            </div>`

const fotoCartao = `
            <figure class="foto-item">
              <if test="f.foto"><a href="{{ f.foto.url }}"><img src="{{ f.foto.url }}" alt="{{ f.titulo }}" loading="lazy"></a><else><div class="sem-foto">«emoji»</div></if>
              <figcaption>{{ f.titulo }}</figcaption>
            </figure>`

const postCartao = `
            <a class="cartao" href="{{ url('«blogU»/' + p.slug) }}">
              <if test="p.capa"><img src="{{ p.capa.url }}" alt="" loading="lazy"></if>
              <span class="meta">{{ default(p.categoria, "") }} · {{ date(p.created_at, "d/m/Y") }}</span>
              <h3>{{ tr(p.titulo) }}</h3>
              <p>{{ tr(default(p.resumo, "")) }}</p>
            </a>`

func (g *ger) planos() string {
	var b strings.Builder
	b.WriteString(`
      <section class="secao" id="planos">
        <div class="centro">
          <span class="chapeu">Planos</span>
          <h2 editavel="planos.titulo">Escolha o seu plano</h2>
          <p class="intro" editavel="planos.intro">Sem letra miúda: troque de plano quando quiser.</p>
        </div>
        <div class="grade precos">
`)
	for i, pl := range g.s.Planos {
		cls := "cartao plano"
		selo := ""
		if pl.Destaque {
			cls += " destaque"
			selo = `<span class="selo">Mais escolhido</span>`
		}
		fmt.Fprintf(&b, "          <div class=\"%s\">%s\n            <h3 editavel=\"plano%d.nome\">%s</h3>\n            <p class=\"valor\"><b editavel=\"plano%d.preco\">%s</b><span editavel=\"plano%d.periodo\">%s</span></p>\n            <ul>", cls, selo, i+1, esc(pl.Nome), i+1, esc(pl.Preco), i+1, esc(pl.Periodo))
		for _, it := range pl.Itens {
			b.WriteString("<li>" + esc(it) + "</li>")
		}
		btn := "botao secundario"
		if pl.Destaque {
			btn = "botao"
		}
		fmt.Fprintf(&b, "</ul>\n            <a class=\"%s\" href=\"{{ url('/contato?assunto=' + urlencode(%s)) }}\">Quero este</a>\n          </div>\n", btn, "'Plano "+strings.ReplaceAll(pl.Nome, "'", "")+"'")
	}
	b.WriteString("        </div>\n      </section>\n")
	return b.String()
}

// quatro itens ficam melhor em 4 colunas (2 no celular) do que em 3 + 1
func quatro(n int) string {
	if n == 4 {
		return " quatro"
	}
	return ""
}
