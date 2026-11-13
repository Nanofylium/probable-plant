package gerador

import (
	"strings"
)

// páginas de cada módulo (a ordem das rotas importa: a primeira que casa vale)
func (g *ger) paginasModulos() string {
	var b strings.Builder
	w := func(x string) { b.WriteString(x) }
	ctaServ := `<a class="botao" href="{{ url('/contato') }}">Fale com a gente</a>`
	if g.tem("agenda") {
		ctaServ = `<a class="botao" href="{{ url('«agU»') }}">«agT»</a>`
	}

	if g.tem("servicos") {
		w(`
    <!-- ================= «servR» ================= -->
    <page route="«servU»" title="«servR»">
      <section class="secao">
        <span class="chapeu">«nome»</span>
        <h1>«servR»</h1>
        <p class="intro">«servI»</p>
        <div class="grade` + quatro(len(g.s.Servicos)) + `">
          <each s in="servicos">
            <a class="cartao servico" href="{{ url('«servU»/' + s.slug) }}">
              <div class="icone">{{ default(s.icone, "✦") }}</div>
              <h3>{{ tr(s.titulo) }}</h3>
              <p>{{ tr(default(s.resumo, "")) }}</p>
              <span class="preco">{{ preco_txt(s.preco) }}</span>
            </a>
          </each>
        </div>
      </section>
    </page>

    <page route="«servU»/:slug" conteudo="servicos">
      <div class="colunas">
        <article class="texto" style="padding:0">
          <a class="chapeu" href="{{ url('«servU»') }}">← «servR»</a>
          <h1>{{ default(item.icone, "") }} {{ tr(item.titulo) }}</h1>
          <p class="intro">{{ tr(default(item.resumo, "")) }}</p>
          <if test="item.capa"><img src="{{ item.capa.url }}" alt=""></if>
          {{ markdown(tr(default(item.corpo, ""))) }}
          <p class="acoes">` + ctaServ + `</p>
        </article>
        <aside class="barra">
          <div class="widget"><h3>Valor</h3><p class="preco-grande">{{ preco_txt(item.preco) }}</p></div>
          <div class="widget"><h3>Veja também</h3>
            <ul><each s in="servicos |> filter(fn(x) do x.id != item.id end)"><li><a href="{{ url('«servU»/' + s.slug) }}">{{ default(s.icone, "") }} {{ s.titulo }}</a></li></each></ul>
          </div>
        </aside>
      </div>
    </page>
`)
	}

	if g.tem("equipe") {
		w(`
    <!-- ================= «equipeR» ================= -->
    <page route="«equipeU»" title="«equipeR»">
      <section class="secao">
        <span class="chapeu">«nome»</span>
        <h1>«equipeR»</h1>
        <p class="intro" editavel="equipe.intro">Gente que gosta do que faz e cuida de cada detalhe.</p>
        <div class="grade">
          <each pe in="db.all('equipe') |> reverse">` + pessoaCartao + `
          </each>
        </div>
      </section>
    </page>
`)
	}

	if g.tem("agenda") {
		if g.s.AgendaModo == "diarias" {
			w(g.reservasDiarias())
		} else {
			w(g.agendaHorarios())
		}
	}

	if g.tem("catalogo") {
		w(`
    <!-- ================= «lojaR» (loja com carrinho) ================= -->
    <page route="«lojaU»" title="«lojaR»">
      <section class="secao">
        <span class="chapeu">Peça pelo site</span>
        <h1>«lojaR»</h1>
        <nav class="chips" aria-label="Categorias">
          <a href="{{ url('«lojaU»') }}" aria-current="{{ request.query.cat and 'false' or 'page' }}">Tudo</a>
          <each c in="CATEGORIAS"><a href="{{ url('«lojaU»?cat=' + urlencode(c)) }}" aria-current="{{ request.query.cat == c and 'page' or 'false' }}">{{ c }}</a></each>
        </nav>
        <div class="grade quatro">
          <each p in="produtos |> filter(fn(x) do not request.query.cat or x.categoria == request.query.cat end)">` + produtoCartao + `
          <else>
            <p>Nenhum produto nesta categoria.</p>
          </each>
        </div>
      </section>
    </page>

    <page route="«lojaU»/:slug" conteudo="produtos">
      <section class="secao">
        <a class="chapeu" href="{{ url('«lojaU»') }}">← «lojaR»</a>
        <div class="produto-detalhe">
          <div class="foto-produto grande"><if test="item.foto"><img src="{{ item.foto.url }}" alt="{{ item.titulo }}"><else><span>{{ default(item.icone, "🛍️") }}</span></if></div>
          <div>
            <span class="meta">{{ item.categoria }}</span>
            <h1>{{ tr(item.titulo) }}</h1>
            <p class="preco-grande">{{ money(item.preco) }}</p>
            <div class="texto" style="padding:0">{{ markdown(tr(default(item.corpo, item.resumo))) }}</div>
            <form run="adicionar" success="" class="comprar-grande">
              <input type="hidden" name="produto" value="{{ item.slug }}">
              <label>Quantidade <input type="number" name="qtd" value="1" min="1" max="50" required></label>
              <button>Adicionar ao carrinho</button>
            </form>
          </div>
        </div>
      </section>
    </page>

    <page route="/carrinho" title="Carrinho" noindex>
      <section class="secao">
        <span class="chapeu">Seu pedido</span>
        <h1>Carrinho</h1>
        <if test="len(carrinho()) > 0">
          <div class="carrinho">
            <div class="cartao">
              <table>
                <thead><tr><th>Produto</th><th>Qtd.</th><th>Valor</th><th></th></tr></thead>
                <tbody>
                  <each i in="carrinho()">
                    <tr>
                      <td><a href="{{ url('«lojaU»/' + i.slug) }}">{{ i.titulo }}</a></td>
                      <td>{{ i.qtd }}</td>
                      <td>{{ money(i.preco * i.qtd) }}</td>
                      <td><form run="remover" success=""><input type="hidden" name="produto" value="{{ i.slug }}"><button class="pequeno secundario" aria-label="Tirar">✕</button></form></td>
                    </tr>
                  </each>
                </tbody>
                <tfoot><tr><th>Total</th><th>{{ itens_carrinho() }}</th><th>{{ money(total_carrinho()) }}</th><th></th></tr></tfoot>
              </table>
              <p><a href="{{ url('«lojaU»') }}">← Continuar comprando</a></p>
            </div>
            <form class="cartao" save="pedidos" run="fechar_pedido" limit="5" avisar="dono" success=""
                  confirmar="Recebemos seu pedido! Vamos confirmar a disponibilidade e o pagamento pelo WhatsApp em instantes."
                  confirmar-assunto="Recebemos seu pedido">
              <h3 style="margin-top:0">Finalizar pedido</h3>
              <label>Nome <input name="nome" required maxlength="80"></label>
              <label>WhatsApp <input name="whatsapp" type="tel" required maxlength="20"></label>
              <label>E-mail <input name="email" type="email" required></label>
              <label>Como prefere receber
                <select name="entrega" required><option>Retirar no local</option><option>Entrega</option></select>
              </label>
              <label>Endereço (para entrega) <input name="endereco" maxlength="200"></label>
              <label>Pagamento
                <select name="pagamento" required><option>Pix</option><option>Cartão na entrega</option><option>Dinheiro</option></select>
              </label>
              <label>Observações <textarea name="obs" maxlength="600" rows="2"></textarea></label>
              <label><input type="checkbox" name="aceite" required data-label="Autorização"> Autorizo o uso destes dados para o pedido (veja a <a href="{{ url('/privacidade') }}">política de privacidade</a>).</label>
              <button>Enviar pedido · {{ money(total_carrinho()) }}</button>
            </form>
          </div>
        <else>
          <div class="cartao centro" style="max-width:520px">
            <div class="icone">🛒</div>
            <h3>Seu carrinho está vazio</h3>
            <p><a class="botao" href="{{ url('«lojaU»') }}">Ver «lojaR»</a></p>
          </div>
        </if>
      </section>
    </page>

    <page route="/pedido/ok" title="Pedido enviado" noindex>
      <section class="secao">
        <div class="ok-grande">
          <div class="icone">✓</div>
          <h1>Pedido enviado!</h1>
          <p class="intro" style="margin:0 auto 1.4em">Total de <b>{{ default(request.query.total, "") }}</b>. A confirmação foi para o seu e-mail e vamos chamar você no WhatsApp.</p>
          <a class="botao" href="{{ url('/') }}">Voltar ao início</a>
        </div>
      </section>
    </page>
`)
	}

	if g.tem("eventos") {
		w(`
    <!-- ================= eventos ================= -->
    <page route="/eventos" title="Eventos">
      <section class="secao">
        <span class="chapeu">Agenda</span>
        <h1>Eventos</h1>
        <p class="intro" editavel="eventos.intro">Inscreva-se pelo site: a confirmação chega no seu e-mail.</p>
        <div class="grade">
          <each e in="proximos_eventos">` + eventoCartao + `
          <else>
            <p>Nenhum evento marcado. Volte em breve!</p>
          </each>
        </div>
        <if test="len(eventos_passados) > 0">
          <h2>Já aconteceram</h2>
          <ul class="lista-simples"><each e in="eventos_passados |> take(10)"><li><a href="{{ url('/eventos/' + e.slug) }}">{{ e.titulo }}</a> <span class="meta">· {{ date(e.data, "d/m/Y") }}</span></li></each></ul>
        </if>
      </section>
    </page>

    <page route="/eventos/:slug" conteudo="eventos">
      <div class="colunas">
        <article class="texto" style="padding:0">
          <a class="chapeu" href="{{ url('/eventos') }}">← Eventos</a>
          <h1>{{ tr(item.titulo) }}</h1>
          <p class="meta">📅 {{ date(item.data, "d/m/Y") }} · 🕒 {{ default(item.hora, "") }} · 📍 {{ default(item.local, "") }}</p>
          <if test="item.capa"><img src="{{ item.capa.url }}" alt=""></if>
          {{ markdown(tr(default(item.corpo, item.resumo))) }}
        </article>
        <aside class="barra">
          <div class="widget" id="inscricao">
            <h3>Inscrição</h3>
            <if test="request.query.inscrito">
              <p class="aviso-ok">✓ Inscrição feita! A confirmação foi para o seu e-mail.</p>
            </if>
            <live>
              <if test="restam(item) != nil"><p class="vagas"><b>{{ restam(item) }}</b> de {{ item.vagas }} vagas livres</p></if>
            </live>
            <if test="item.data < hoje">
              <p>Este evento já aconteceu.</p>
            <else>
              <form save="inscricoes" run="inscrever" limit="5" avisar="dono" success=""
                    confirmar="Sua inscrição está confirmada! Guarde este e-mail. Se não puder ir, responda para liberar a vaga."
                    confirmar-assunto="Inscrição confirmada">
                <input type="hidden" name="evento" value="{{ item.slug }}">
                <label>Nome <input name="nome" required maxlength="80"></label>
                <label>E-mail <input name="email" type="email" required></label>
                <label>WhatsApp <input name="whatsapp" type="tel" maxlength="20"></label>
                <label><input type="checkbox" name="aceite" required data-label="Autorização"> Autorizo o uso destes dados para a inscrição.</label>
                <button>Garantir minha vaga</button>
              </form>
            </if>
          </div>
        </aside>
      </div>
    </page>
`)
	}

	if g.tem("galeria") {
		w(`
    <!-- ================= galeria ================= -->
    <page route="/galeria" title="Galeria">
      <section class="secao">
        <span class="chapeu">Fotos</span>
        <h1>Galeria</h1>
        <nav class="chips" aria-label="Álbuns">
          <a href="{{ url('/galeria') }}" aria-current="{{ request.query.album and 'false' or 'page' }}">Todas</a>
          <each c in="ALBUNS"><a href="{{ url('/galeria?album=' + urlencode(c)) }}" aria-current="{{ request.query.album == c and 'page' or 'false' }}">{{ c }}</a></each>
        </nav>
        <div class="fotos">
          <each f in="fotos()">` + fotoCartao + `
          <else>
            <p>Nenhuma foto neste álbum.</p>
          </each>
        </div>
      </section>
    </page>
`)
	}

	if g.tem("blog") {
		w(g.blog())
	}
	if g.tem("wiki") {
		w(g.wiki())
	}
	if g.tem("forum") {
		w(g.forum())
	}

	if g.tem("downloads") {
		w(`
    <!-- ================= downloads ================= -->
    <page route="/downloads" title="Downloads">
      <section class="secao">
        <span class="chapeu">Documentos e materiais</span>
        <h1>Downloads</h1>
        <p class="intro" editavel="downloads.intro">Tudo em um lugar, para baixar quando precisar.</p>
        <each c in="arquivos_por_categoria()">
          <h2 class="grupo">{{ c.nome }}</h2>
          <div class="arquivos">
            <each d in="c.itens">
              <div class="cartao arquivo">
                <div class="icone">📄</div>
                <div>
                  <h3>{{ d.titulo }}</h3>
                  <p>{{ default(d.resumo, "") }}</p>
                </div>
                <if test="d.arquivo"><a class="botao pequeno" href="{{ d.arquivo.url }}" download>Baixar</a><else><span class="meta">em breve</span></if>
              </div>
            </each>
          </div>
        <else>
          <p>Nenhum arquivo publicado ainda.</p>
        </each>
      </section>
    </page>
`)
	}

	if g.tem("vagas") {
		w(`
    <!-- ================= trabalhe conosco ================= -->
    <page route="/trabalhe-conosco" title="Trabalhe conosco">
      <section class="secao">
        <span class="chapeu">Carreiras</span>
        <h1>Trabalhe conosco</h1>
        <if test="request.query.enviado"><p class="aviso-ok">✓ Recebemos seu currículo. Obrigado! Se o seu perfil combinar, chamamos você.</p></if>
        <p class="intro" editavel="vagas.intro">Gostamos de gente curiosa e que cuida bem dos outros. Veja as vagas abertas.</p>
        <div class="grade">
          <each v in="db.all('vagas')">
            <a class="cartao" href="{{ url('/trabalhe-conosco/' + v.slug) }}">
              <span class="selo">{{ default(v.tipo, "") }}</span>
              <h3>{{ v.titulo }}</h3>
              <p class="meta">{{ default(v.area, "") }} · {{ default(v.local, "") }}</p>
              <p>{{ default(v.resumo, "") }}</p>
            </a>
          <else>
            <p>Nenhuma vaga aberta no momento.</p>
          </each>
        </div>
      </section>
    </page>

    <page route="/trabalhe-conosco/:slug" conteudo="vagas">
      <div class="colunas">
        <article class="texto" style="padding:0">
          <a class="chapeu" href="{{ url('/trabalhe-conosco') }}">← Vagas</a>
          <h1>{{ item.titulo }}</h1>
          <p class="meta">{{ default(item.tipo, "") }} · {{ default(item.area, "") }} · {{ default(item.local, "") }}</p>
          {{ markdown(default(item.corpo, item.resumo)) }}
        </article>
        <aside class="barra">
          <div class="widget">
            <h3>Candidate-se</h3>
            <form save="candidaturas" run="candidatar" limit="3" avisar="dono" success=""
                  confirmar="Recebemos sua candidatura. Obrigado pelo interesse!" confirmar-assunto="Recebemos sua candidatura">
              <input type="hidden" name="vaga" value="{{ item.slug }}">
              <label>Nome <input name="nome" required maxlength="80"></label>
              <label>E-mail <input name="email" type="email" required></label>
              <label>Telefone <input name="telefone" type="tel" maxlength="20"></label>
              <label>LinkedIn ou portfólio <input name="link" type="url" maxlength="200"></label>
              <label>Currículo (PDF) <input type="file" name="curriculo" accept=".pdf,.doc,.docx" data-max-mb="5" required></label>
              <label>Por que você? <textarea name="mensagem" maxlength="1500" rows="3"></textarea></label>
              <label><input type="checkbox" name="aceite" required data-label="Autorização"> Autorizo guardar meus dados por 12 meses para processos seletivos.</label>
              <button>Enviar candidatura</button>
            </form>
          </div>
        </aside>
      </div>
    </page>
`)
	}

	if g.tem("faq") {
		w(`
    <!-- ================= perguntas frequentes ================= -->
    <page route="/perguntas-frequentes" title="Perguntas frequentes">
      <section class="secao">
        <div class="estreito">
          <span class="chapeu">Ajuda</span>
          <h1>Perguntas frequentes</h1>
          <each q in="perguntas">
            <details><summary>{{ tr(q.titulo) }}</summary><p>{{ tr(q.resposta) }}</p></details>
          </each>
          <div class="cartao" style="margin-top:28px"><b>Não achou a resposta?</b> <a href="{{ url('/contato') }}">Pergunte para a gente</a>.</div>
        </div>
      </section>
    </page>
`)
	}
	return b.String()
}

func (g *ger) agendaHorarios() string {
	s := g.s
	extra := ""
	if g.tem("servicos") {
		var ops []string
		for _, it := range s.Servicos {
			ops = append(ops, it.Titulo)
		}
		extra = `            <label>«servR»
              <select name="servico" required>` + opcoes(ops) + `</select>
            </label>
`
	}
	if s.ID == "restaurante" {
		extra += `            <label>Pessoas
              <select name="pessoas" required><option>1</option><option selected>2</option><option>3</option><option>4</option><option>5</option><option>6</option><option>7</option><option>8</option><option>10</option><option>12</option></select>
            </label>
`
	}
	return `
    <!-- ================= «agV» ================= -->
    <page route="«agU»" title="«agT»">
      <section class="secao">
        <span class="chapeu">Online, na hora</span>
        <h1>«agT»</h1>
        <p class="intro">Escolha o dia e o horário. A confirmação chega no seu e-mail.</p>
        <div class="agenda">
          <form class="cartao" save="agendamentos" run="agendar" limit="5" avisar="dono" success=""
                confirmar="Olá! Seu horário com «nome» está confirmado. Para remarcar, responda este e-mail."
                confirmar-assunto="Horário confirmado">
            <div class="duplo">
              <label>Seu nome <input name="nome" required maxlength="80"></label>
              <label>WhatsApp <input name="whatsapp" type="tel" required maxlength="20"></label>
            </div>
            <label>E-mail (para a confirmação) <input name="email" type="email" required></label>
` + extra + `            <div class="duplo">
              <label>Dia <input name="data" type="date" required min="{{ hoje }}"></label>
              <label>Horário
                <select name="horario" required><option value="">Escolha…</option>` + opcoes(s.Horarios) + `</select>
              </label>
            </div>
            <label>Observações (opcional) <textarea name="obs" maxlength="800" rows="3"></textarea></label>
            <label><input type="checkbox" name="aceite" required data-label="Autorização"> Autorizo o uso destes dados para o atendimento.</label>
            <button>Confirmar</button>
          </form>
          <aside class="cartao">
            <h3 style="margin-top:0">Horários livres</h3>
            <live>
              <p class="vivo"><i></i>atualiza sozinho</p>
              <div class="dias">
                <each d in="proximos_dias(5)">
                  <div class="dia">
                    <h4>{{ d.txt }}</h4>
                    <div class="horas"><each h in="livres(d.data)"><span>{{ h }}</span><else><em>Sem horários livres</em></each></div>
                  </div>
                </each>
              </div>
            </live>
          </aside>
        </div>
      </section>
    </page>

    <page route="«agU»/ok" title="Confirmado" noindex>
      <section class="secao">
        <div class="ok-grande">
          <div class="icone">✓</div>
          <h1>Tudo certo!</h1>
          <if test="request.query.data">
            <p class="intro" style="margin:0 auto 1.4em">Esperamos você em <b>{{ date(request.query.data, 'd/m/Y') }}</b> às <b>{{ default(request.query.h, '') }}</b>. A confirmação foi para o seu e-mail.</p>
          </if>
          <a class="botao" href="{{ url('/') }}">Voltar ao início</a>
        </div>
      </section>
    </page>
`
}

func (g *ger) reservasDiarias() string {
	var ops []string
	for _, it := range g.s.Servicos {
		ops = append(ops, it.Titulo)
	}
	return `
    <!-- ================= reservas por diária ================= -->
    <page route="«agU»" title="«agT»">
      <section class="secao">
        <span class="chapeu">Reserva online</span>
        <h1>«agT»</h1>
        <p class="intro">Escolha as datas e a acomodação. Confirmamos a pré-reserva por e-mail com as instruções de pagamento.</p>
        <div class="agenda">
          <form class="cartao" save="reservas" run="reservar" limit="5" avisar="dono" success=""
                confirmar="Recebemos sua pré-reserva com «nome». Em breve enviamos as instruções de pagamento para confirmar."
                confirmar-assunto="Pré-reserva recebida">
            <div class="duplo">
              <label>Seu nome <input name="nome" required maxlength="80"></label>
              <label>WhatsApp <input name="whatsapp" type="tel" required maxlength="20"></label>
            </div>
            <label>E-mail <input name="email" type="email" required></label>
            <label>Acomodação <select name="quarto" required>` + opcoes(ops) + `</select></label>
            <div class="duplo">
              <label>Entrada <input name="entrada" type="date" required min="{{ hoje }}"></label>
              <label>Saída <input name="saida" type="date" required min="{{ hoje }}"></label>
            </div>
            <label>Hóspedes <select name="hospedes" required><option>1</option><option selected>2</option><option>3</option><option>4</option></select></label>
            <label>Pedidos especiais (opcional) <textarea name="obs" maxlength="800" rows="3"></textarea></label>
            <label><input type="checkbox" name="aceite" required data-label="Autorização"> Autorizo o uso destes dados para a reserva.</label>
            <button>Pedir reserva</button>
          </form>
          <aside class="cartao">
            <h3 style="margin-top:0">Datas livres</h3>
            <live>
              <p class="vivo"><i></i>atualiza sozinho</p>
              <div class="dias">
                <each d in="mapa_dias(10)">
                  <div class="dia">
                    <h4>{{ d.txt }}</h4>
                    <div class="horas"><each q in="d.livres"><span>{{ q }}</span><else><em>Lotado</em></each></div>
                  </div>
                </each>
              </div>
            </live>
          </aside>
        </div>
      </section>
    </page>

    <page route="«agU»/ok" title="Pré-reserva recebida" noindex>
      <section class="secao">
        <div class="ok-grande">
          <div class="icone">✓</div>
          <h1>Pré-reserva recebida!</h1>
          <if test="request.query.entrada">
            <p class="intro" style="margin:0 auto 1.4em"><b>{{ default(request.query.q, '') }}</b>, de <b>{{ date(request.query.entrada, 'd/m') }}</b> a <b>{{ date(request.query.saida, 'd/m/Y') }}</b>. As instruções de pagamento vão para o seu e-mail.</p>
          </if>
          <a class="botao" href="{{ url('/') }}">Voltar ao início</a>
        </div>
      </section>
    </page>
`
}

func (g *ger) blog() string {
	return `
    <!-- ================= «blogR» (blog completo) ================= -->
    <page route="«blogU»" title="«blogR»">
      <div class="colunas">
        <div>
          <span class="chapeu">«blogR»</span>
          <h1>{{ filtro_txt() != "" and filtro_txt() or "«blogR»" }}</h1>
          <if test="filtro_txt() != ''"><p><a href="{{ url('«blogU»') }}">← Ver tudo</a></p></if>
          <div class="lista-posts">
            <each p in="pagina_de(posts_filtrados())">
              <article class="post-linha">
                <a class="capa-post" href="{{ url('«blogU»/' + p.slug) }}"><if test="p.capa"><img src="{{ p.capa.url }}" alt="" loading="lazy"><else><span>«emoji»</span></if></a>
                <div>
                  <span class="meta"><a href="{{ url('«blogU»?categoria=' + urlencode(default(p.categoria, ''))) }}">{{ default(p.categoria, "") }}</a> · {{ date(p.created_at, "d/m/Y") }} · {{ leitura(p) }} min de leitura</span>
                  <h2><a href="{{ url('«blogU»/' + p.slug) }}">{{ tr(p.titulo) }}</a></h2>
                  <p>{{ tr(default(p.resumo, "")) }}</p>
                </div>
              </article>
            <else>
              <p>Nada publicado por aqui ainda.</p>
            </each>
          </div>
          <if test="total_paginas(posts_filtrados()) > 1">
            <nav class="paginacao">
              <each n in="range(1, total_paginas(posts_filtrados()) + 1)">
                <a href="{{ link_pagina(n) }}" aria-current="{{ n == pagina_atual() and 'page' or 'false' }}">{{ n }}</a>
              </each>
            </nav>
          </if>
        </div>
        ` + g.barraBlog() + `
      </div>
    </page>

    <page route="«blogU»/:slug" conteudo="posts">
      <div class="colunas">
        <article class="texto" style="padding:0">
          <a class="chapeu" href="{{ url('«blogU»?categoria=' + urlencode(default(item.categoria, ''))) }}">{{ default(item.categoria, "«blogR»") }}</a>
          <h1>{{ tr(item.titulo) }}</h1>
          <p class="meta">{{ default(item.autor, "«nome»") }} · {{ date(item.created_at, "d/m/Y") }} · {{ leitura(item) }} min de leitura</p>
          <if test="item.capa"><img src="{{ item.capa.url }}" alt=""></if>
          {{ markdown(tr(item.corpo)) }}
          <if test="len(tags_de(item)) > 0">
            <p class="tags"><each t in="tags_de(item)"><a href="{{ url('«blogU»?tag=' + urlencode(t)) }}">#{{ t }}</a></each></p>
          </if>
          <if test="len(relacionados(item)) > 0">
            <h3>Leia também</h3>
            <ul class="lista-simples"><each r in="relacionados(item)"><li><a href="{{ url('«blogU»/' + r.slug) }}">{{ r.titulo }}</a></li></each></ul>
          </if>
          {{ comentarios() }}
        </article>
        ` + g.barraBlog() + `
      </div>
    </page>
`
}

func (g *ger) barraBlog() string {
	return `<aside class="barra">
          <div class="widget"><h3>Buscar</h3>
            <form method="get" action="{{ url('/busca') }}" class="busca-mini"><input name="q" aria-label="Buscar" placeholder="Buscar…"><button>Ir</button></form>
          </div>
          <div class="widget"><h3>Categorias</h3>
            <ul><each c in="categorias_blog()"><li><a href="{{ url('«blogU»?categoria=' + urlencode(c.nome)) }}">{{ c.nome }}</a> <span class="meta">({{ c.n }})</span></li></each></ul>
          </div>
          <div class="widget"><h3>Recentes</h3>
            <ul><each r in="db.all('posts', 5)"><li><a href="{{ url('«blogU»/' + r.slug) }}">{{ r.titulo }}</a></li></each></ul>
          </div>
          <div class="widget"><h3>Tags</h3>
            <p class="tags"><each t in="todas_tags()"><a href="{{ url('«blogU»?tag=' + urlencode(t)) }}">#{{ t }}</a></each></p>
          </div>
          <div class="widget"><h3>Arquivo</h3>
            <ul><each m in="arquivo_blog()"><li><a href="{{ url('«blogU»?mes=' + m) }}">{{ mes_txt(m) }}</a></li></each></ul>
          </div>
          <div class="widget"><h3>Assine</h3><p><a href="{{ url('/feed.xml') }}">Assinar por RSS</a></p></div>
        </aside>`
}

func (g *ger) wiki() string {
	return `
    <!-- ================= wiki (estilo TikiWiki) ================= -->
    <page route="/wiki" title="Wiki">
      <div class="colunas">
        <div>
          <span class="chapeu">Wiki</span>
          <h1>Índice da wiki</h1>
          <form method="get" action="{{ url('/wiki') }}" class="busca-mini" style="margin-bottom:24px">
            <input name="q" value="{{ default(request.query.q, '') }}" placeholder="Procurar na wiki…" aria-label="Procurar na wiki"><button>Procurar</button>
          </form>
          <nav class="letras"><each l in="wiki_letras()"><a href="#letra-{{ l.letra }}">{{ l.letra }}</a></each></nav>
          <each l in="wiki_letras()">
            <h2 class="grupo" id="letra-{{ l.letra }}">{{ l.letra }}</h2>
            <ul class="lista-wiki"><each p in="l.paginas"><li><a href="{{ url('/wiki/' + p.slug) }}">{{ p.titulo }}</a> <span class="meta">{{ default(p.categoria, "") }}</span></li></each></ul>
          <else>
            <p>Nenhuma página encontrada. <a href="{{ url('/wiki/' + slug(default(request.query.q, 'nova-pagina')) + '/editar') }}">Sugira a criação dela</a>.</p>
          </each>
        </div>
        <aside class="barra">
          <div class="widget"><h3>Mudanças recentes</h3>
            <ul><each p in="wiki_recentes()"><li><a href="{{ url('/wiki/' + p.slug) }}">{{ p.titulo }}</a> <span class="meta">{{ date(default(p.updated_at, p.created_at), "d/m") }}</span></li></each></ul>
          </div>
          <div class="widget"><h3>Categorias</h3>
            <ul><each c in="wiki_categorias()"><li>{{ c.nome }} <span class="meta">({{ c.n }})</span></li></each></ul>
          </div>
          <div class="widget"><h3>Contribua</h3><p>Viu algo faltando? Abra a página e clique em <b>Sugerir edição</b>.</p></div>
        </aside>
      </div>
    </page>

    <page route="/wiki/:slug/editar" title="Sugerir edição" noindex>
      <section class="secao">
        <div class="estreito">
          <a class="chapeu" href="{{ url('/wiki/' + params.slug) }}">← Voltar à página</a>
          <h1>Sugerir edição</h1>
          <p class="intro">A equipe revisa a sugestão antes de publicar. O histórico de versões fica guardado no painel.</p>
          <form class="cartao" save="sugestoes" run="sugerir" limit="5" avisar="dono" success="">
            <input type="hidden" name="pagina" value="{{ params.slug }}">
            <label>Título da página <input name="titulo" required maxlength="120" value="{{ wiki_pagina(params.slug) and wiki_pagina(params.slug).titulo or '' }}"></label>
            <label>Texto (use [[Outra página]] para criar links) <textarea name="texto" required maxlength="80000" rows="14">{{ wiki_pagina(params.slug) and wiki_pagina(params.slug).corpo or '' }}</textarea></label>
            <label>O que você mudou? <input name="resumo" maxlength="200" placeholder="Corrigi um link, acrescentei um exemplo…"></label>
            <div class="duplo">
              <label>Seu nome <input name="nome" required maxlength="80"></label>
              <label>E-mail <input name="email" type="email" required></label>
            </div>
            <button>Enviar sugestão</button>
          </form>
        </div>
      </section>
    </page>

    <page route="/wiki/:slug" title="Wiki">
      <div class="colunas">
        <each p in="db.where('wiki', 'slug', params.slug)">
          <article class="texto" style="padding:0">
            <a class="chapeu" href="{{ url('/wiki') }}">← Wiki</a>
            <if test="request.query.sugestao"><p class="aviso-ok">✓ Sugestão enviada! A equipe revisa e publica.</p></if>
            <h1>{{ tr(p.titulo) }}</h1>
            <p class="meta">{{ default(p.categoria, "Geral") }} · atualizada em {{ date(default(p.updated_at, p.created_at), "d/m/Y") }}</p>
            {{ markdown(wiki_links(tr(p.corpo))) }}
            <p class="acoes"><a class="botao secundario pequeno" href="{{ url('/wiki/' + p.slug + '/editar') }}">✎ Sugerir edição</a></p>
          </article>
          <aside class="barra">
            <div class="widget"><h3>Páginas que apontam para cá</h3>
              <ul><each o in="apontam_para(p)"><li><a href="{{ url('/wiki/' + o.slug) }}">{{ o.titulo }}</a></li><else><li class="meta">Nenhuma ainda.</li></each></ul>
            </div>
            <div class="widget"><h3>Mudanças recentes</h3>
              <ul><each r in="wiki_recentes() |> take(5)"><li><a href="{{ url('/wiki/' + r.slug) }}">{{ r.titulo }}</a></li></each></ul>
            </div>
          </aside>
        <else>
          <article class="texto" style="padding:0">
            <a class="chapeu" href="{{ url('/wiki') }}">← Wiki</a>
            <if test="request.query.sugestao"><p class="aviso-ok">✓ Sugestão enviada! A página aparece aqui assim que a equipe aprovar.</p></if>
            <h1>Esta página ainda não existe</h1>
            <p>Ninguém escreveu sobre isso ainda. Que tal começar?</p>
            <p><a class="botao" href="{{ url('/wiki/' + params.slug + '/editar') }}">Sugerir esta página</a></p>
          </article>
        </each>
      </div>
    </page>
`
}

func (g *ger) forum() string {
	return `
    <!-- ================= fórum (moderado) ================= -->
    <page route="/forum" title="Fórum">
      <div class="colunas">
        <div>
          <span class="chapeu">Comunidade</span>
          <h1>Fórum</h1>
          <if test="request.query.enviado"><p class="aviso-ok">✓ Tópico enviado! Ele aparece aqui assim que a moderação aprovar.</p></if>
          <nav class="chips" aria-label="Categorias">
            <a href="{{ url('/forum') }}" aria-current="{{ request.query.cat and 'false' or 'page' }}">Todos</a>
            <each c in="FORUM_CATS"><a href="{{ url('/forum?cat=' + urlencode(c)) }}" aria-current="{{ request.query.cat == c and 'page' or 'false' }}">{{ c }}</a></each>
          </nav>
          <div class="topicos">
            <each t in="topicos_lista()">
              <a class="topico" href="{{ url('/forum/' + t.slug) }}">
                <span class="avatar">{{ inicial(t.autor) }}</span>
                <span><b>{{ t.titulo }}</b><br><span class="meta">{{ t.categoria }} · por {{ default(t.autor, "anônimo") }} · {{ date(t.created_at, "d/m/Y") }}</span></span>
              </a>
            <else>
              <p>Nenhum tópico nesta categoria ainda.</p>
            </each>
          </div>
        </div>
        <aside class="barra">
          <div class="widget"><h3>Participe</h3><p>Tem uma dúvida ou ideia?</p><a class="botao pequeno" href="{{ url('/forum/novo') }}">Novo tópico</a></div>
          <div class="widget"><h3>Regras</h3><p>Respeito sempre. Tópicos novos passam pela moderação antes de aparecer.</p></div>
        </aside>
      </div>
    </page>

    <page route="/forum/novo" title="Novo tópico" noindex>
      <section class="secao">
        <div class="estreito">
          <a class="chapeu" href="{{ url('/forum') }}">← Fórum</a>
          <h1>Novo tópico</h1>
          <form class="cartao" save="topicos" run="novo_topico" limit="3" avisar="dono" success="">
            <label>Título <input name="titulo" required maxlength="140"></label>
            <label>Categoria <select name="categoria" required><option>Geral</option><option>Dúvidas</option><option>Ideias</option><option>Avisos</option></select></label>
            <label>Mensagem <textarea name="corpo" required maxlength="10000" rows="8"></textarea></label>
            <div class="duplo">
              <label>Seu nome (aparece no tópico) <input name="autor" required maxlength="60"></label>
              <label>E-mail (não aparece) <input name="email" type="email" required></label>
            </div>
            <button>Enviar para a moderação</button>
          </form>
        </div>
      </section>
    </page>

    <page route="/forum/:slug" conteudo="topicos">
      <section class="secao">
        <div class="estreito">
          <a class="chapeu" href="{{ url('/forum?cat=' + urlencode(default(item.categoria, ''))) }}">← {{ default(item.categoria, "Fórum") }}</a>
          <h1>{{ item.titulo }}</h1>
          <div class="cartao mensagem">
            <p class="meta"><span class="avatar">{{ inicial(item.autor) }}</span> {{ default(item.autor, "anônimo") }} · {{ date(item.created_at, "d/m/Y H:i") }}</p>
            <p>{{ nl2br(item.corpo) }}</p>
          </div>
          <h2>Respostas</h2>
          {{ comentarios() }}
        </div>
      </section>
    </page>
`
}

// ---------------------------------------------------------------- páginas de todo site

func (g *ger) paginasFixas() string {
	busca := ""
	if g.tem("blog") || g.tem("wiki") || g.tem("forum") || g.tem("catalogo") || g.tem("servicos") || g.tem("eventos") {
		busca = `
    <page route="/busca" title="Busca" noindex>
      <section class="secao">
        <div class="estreito">
          <span class="chapeu">Busca</span>
          <h1>Buscar no site</h1>
          <form method="get" action="{{ url('/busca') }}" class="busca-mini" style="margin-bottom:28px">
            <input name="q" value="{{ default(request.query.q, '') }}" aria-label="Buscar"><button>Buscar</button>
          </form>
          <if test="request.query.q">
            <each r in="buscar(request.query.q)">
              <p><a href="{{ r.url }}"><strong>{{ r.titulo }}</strong></a><br><span class="meta">{{ r.resumo }}</span></p>
            <else>
              <p>Nada encontrado para “{{ request.query.q }}”.</p>
            </each>
          </if>
        </div>
      </section>
    </page>
`
	}
	return busca + `
    <!-- ================= contato ================= -->
    <page route="/contato" title="Contato">
      <section class="secao">
        <div class="contato">
          <div>
            <span class="chapeu">Contato</span>
            <h1 editavel="contato.titulo">Fale com a gente</h1>
            <p class="intro" editavel="contato.texto">Conte o que você precisa. Respondemos em até um dia útil.</p>
            <dl>
              <div><dt>WhatsApp</dt><dd editavel="contato.whatsapp">(00) 00000-0000</dd></div>
              <div><dt>E-mail</dt><dd editavel="contato.email">contato@` + slugify(g.p.Nome) + `.com.br</dd></div>
              <div><dt>Endereço</dt><dd editavel="contato.endereco">Rua Exemplo, 123` + g.vars["«cidade»"] + `</dd></div>
              <div><dt>Horário</dt><dd editavel="contato.horario">Segunda a sexta, 8h às 18h · Sábado, 8h às 12h</dd></div>
            </dl>
          </div>
          <form class="cartao" save="contatos" avisar="dono" limit="5" success="Mensagem enviada! Respondemos em breve."
                confirmar="Recebemos sua mensagem e vamos responder em até um dia útil." confirmar-assunto="Recebemos sua mensagem">
            <label>Nome <input name="nome" required maxlength="80"></label>
            <label>E-mail <input name="email" type="email" required></label>
            <label>Telefone <input name="telefone" type="tel" maxlength="20"></label>
            <label>Assunto <input name="assunto" maxlength="120" value="{{ default(request.query.assunto, '') }}"></label>
            <label>Mensagem <textarea name="mensagem" required maxlength="3000"></textarea></label>
            <label><input type="checkbox" name="aceite" required data-label="Autorização"> Autorizo o uso destes dados para responder (veja a <a href="{{ url('/privacidade') }}">política de privacidade</a>).</label>
            <button>Enviar mensagem</button>
          </form>
        </div>
      </section>
    </page>

    <page route="404" title="Não encontrado">
      <section class="secao centro">
        <span class="chapeu">Erro 404</span>
        <h1>Esta página não existe</h1>
        <p class="intro">O endereço pode ter mudado. Que tal voltar ao início?</p>
        <a class="botao" href="{{ url('/') }}">Ir para o início</a>
      </section>
    </page>

    <!-- Páginas criadas no painel (Sobre, Privacidade...). Fica por último. -->
    <page route="/:slug" conteudo="paginas">
      <article class="texto">
        <h1>{{ tr(item.titulo) }}</h1>
        {{ markdown(tr(default(item.corpo, ""))) }}
        {{ visual(item.visual) }}
      </article>
    </page>
`
}

func (g *ger) rodape() string {
	var nav, inst strings.Builder
	nav.WriteString("          <a href=\"{{ url('/') }}\">Início</a>\n")
	for _, l := range g.secoes() {
		nav.WriteString("          " + a(l, "") + "\n")
	}
	nav.WriteString("          <a href=\"{{ url('/contato') }}\">Contato</a>\n")
	inst.WriteString("          <a href=\"{{ url('/sobre') }}\">Sobre</a>\n")
	if g.tem("faq") {
		inst.WriteString("          <a href=\"{{ url('/perguntas-frequentes') }}\">Perguntas frequentes</a>\n")
	}
	if g.tem("vagas") {
		inst.WriteString("          <a href=\"{{ url('/trabalhe-conosco') }}\">Trabalhe conosco</a>\n")
	}
	inst.WriteString("          <a href=\"{{ url('/privacidade') }}\">Política de privacidade</a>\n")
	if g.tem("blog") {
		inst.WriteString("          <a href=\"{{ url('/feed.xml') }}\">RSS</a>\n")
	}
	ultima := `      <div>
        <h4>Siga</h4>
        <nav menu-editavel="redes">
          <a href="https://instagram.com/">Instagram</a>
          <a href="https://wa.me/5500000000000">WhatsApp</a>
        </nav>
      </div>`
	if g.tem("newsletter") {
		ultima = `      <div>
        <h4>Novidades por e-mail</h4>
        <p>Uma mensagem por mês, sem spam.</p>
        <form save="assinantes" limit="3" success="Pronto! Você vai receber as novidades." class="assinar">
          <input name="email" type="email" required placeholder="seu@email.com" aria-label="Seu e-mail">
          <button>Assinar</button>
        </form>
      </div>`
	}
	return `
  <footer>
    <div class="colunas-rodape">
      <div>
        <h4 editavel="marca">«nome»</h4>
        <p editavel="rodape.texto">` + esc(g.s.Texto) + `</p>
      </div>
      <div>
        <h4>Navegue</h4>
        <nav menu-editavel="rodape">
` + nav.String() + `        </nav>
      </div>
      <div>
        <h4>Institucional</h4>
        <nav menu-editavel="institucional">
` + inst.String() + `        </nav>
      </div>
` + ultima + `
    </div>
    <div class="base">
      <span editavel="rodape.base">© «ano» «nome»«cidade». Todos os direitos reservados.</span>
      <span>Feito com Nepenthes</span>
    </div>
  </footer>
`
}
