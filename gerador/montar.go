package gerador

import (
	"fmt"
	"html"
	"strconv"
	"strings"
	"time"
)

// Montar escreve o site.nep.html do plano.
func Montar(p *Plano) string {
	g := &ger{p: p, s: p.Setor}
	g.rotas()
	var b strings.Builder
	b.WriteString(g.cabecalho())
	b.WriteString(g.guarda())
	fmt.Fprintf(&b, "\n<nep-config tema=\"nepenthes\" cor=\"%s\" fonte=\"%s\" nome=\"%s\" movimento=\"suave\" lang=\"pt-BR\">\n", p.Cor, p.Fonte, esc(p.Nome))
	b.WriteString(g.conteudos())
	b.WriteString(g.codigo())
	b.WriteString(g.apis())
	b.WriteString(g.html())
	return g.troca(b.String())
}

type ger struct {
	p    *Plano
	s    Setor
	vars map[string]string
}

func (g *ger) tem(m string) bool { return g.p.Tem(m) }

func esc(s string) string { return html.EscapeString(s) }

// lit escreve um texto como literal Nepenthes ("...").
func lit(s string) string {
	r := strings.NewReplacer(`\`, `\\`, `"`, `\"`, "\n", `\n`)
	return `"` + r.Replace(s) + `"`
}

func slugify(s string) string {
	s = semAcento(s)
	var b strings.Builder
	traco := false
	for _, r := range s {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			b.WriteRune(r)
			traco = false
		} else if !traco && b.Len() > 0 {
			b.WriteByte('-')
			traco = true
		}
	}
	return strings.TrimSuffix(b.String(), "-")
}

func (g *ger) rotas() {
	s, p := g.s, g.p
	ag := "/agendar"
	if s.AgendaVerbo == "Reservar" {
		ag = "/reservar"
	}
	servU := "/" + slugify(s.ServRotulo)
	blogU := "/" + slugify(s.BlogRotulo)
	lojaU := "/" + slugify(s.LojaRotulo)
	equipeU := "/" + slugify(s.EquipeRotulo)
	// nada de rotas repetidas entre módulos
	usadas := map[string]bool{"/contato": true, "/busca": true, "/eventos": true, "/galeria": true, "/wiki": true, "/forum": true, "/downloads": true, "/trabalhe-conosco": true, "/perguntas-frequentes": true, ag: true, "/carrinho": true}
	unico := func(u, alt string) string {
		if usadas[u] || u == "/" {
			u = alt
		}
		usadas[u] = true
		return u
	}
	servU = unico(servU, "/servicos")
	blogU = unico(blogU, "/blog")
	lojaU = unico(lojaU, "/loja")
	equipeU = unico(equipeU, "/equipe")

	cidade := ""
	if p.Cidade != "" {
		cidade = " em " + p.Cidade
	}
	cta, ctaU := "Fale com a gente", "/contato"
	switch {
	case g.tem("agenda"):
		cta, ctaU = s.AgendaTitulo, ag
	case g.tem("catalogo"):
		cta, ctaU = "Ver "+strings.ToLower(s.LojaRotulo), lojaU
	case g.tem("eventos"):
		cta, ctaU = "Ver os próximos eventos", "/eventos"
	case g.tem("wiki"):
		cta, ctaU = "Explorar a wiki", "/wiki"
	case g.tem("forum"):
		cta, ctaU = "Entrar no fórum", "/forum"
	}
	cta2, cta2U := "Fale com a gente", "/contato"
	switch {
	case g.tem("catalogo") && ctaU != lojaU:
		cta2, cta2U = "Ver "+strings.ToLower(s.LojaRotulo), lojaU
	case g.tem("servicos") && ctaU != servU:
		cta2, cta2U = "Conhecer "+strings.ToLower(s.ServRotulo), servU
	case g.tem("planos"):
		cta2, cta2U = "Ver planos", "/#planos"
	case g.tem("blog"):
		cta2, cta2U = "Ler "+strings.ToLower(s.BlogRotulo), blogU
	}
	if cta2U == ctaU {
		cta2, cta2U = "Fale com a gente", "/contato"
	}
	g.vars = map[string]string{
		"«nome»": esc(p.Nome), "«nomeLit»": lit(p.Nome), "«emoji»": s.Emoji, "«cidade»": esc(cidade),
		"«servR»": esc(s.ServRotulo), "«servU»": servU, "«servI»": esc(s.ServIntro),
		"«equipeR»": esc(s.EquipeRotulo), "«equipeU»": equipeU,
		"«blogR»": esc(s.BlogRotulo), "«blogU»": blogU,
		"«lojaR»": esc(s.LojaRotulo), "«lojaU»": lojaU,
		"«agV»": esc(s.AgendaVerbo), "«agT»": esc(s.AgendaTitulo), "«agU»": ag,
		"«cta»": esc(cta), "«ctaU»": ctaU, "«cta2»": esc(cta2), "«cta2U»": cta2U,
		"«ano»": strconv.Itoa(time.Now().Year()),
	}
}

func (g *ger) troca(s string) string {
	var pares []string
	for k, v := range g.vars {
		pares = append(pares, k, v)
	}
	return strings.NewReplacer(pares...).Replace(s)
}

// ---------------------------------------------------------------- cabeçalho

func (g *ger) cabecalho() string {
	var mods []string
	for _, m := range g.p.Modulos {
		mods = append(mods, NomeModulo[m])
	}
	mods = append(mods, "contato", "páginas livres (Sobre, Privacidade...)", "busca", "SEO, RSS e mapa do site")
	desc := strings.ReplaceAll(g.p.Descricao, "--", "- -")
	return `<!doctype html>
<!--
  «nome» · site criado com:  nep criar "` + desc + `"

  O que entrou:
    - ` + strings.Join(mods, "\n    - ") + `

  Rode com:        nep dev site.nep.html     (abre em http://localhost:8000)
  Painel:          /_admin  (textos, menus, conteúdo, pedidos, mensagens, usuários, backup)
  Ver sem servidor: nep previa site.nep.html  (gera site-previa.html)

  Tudo aqui é seu: mude textos, cores e páginas à vontade. O conteúdo de exemplo
  entra só na primeira vez que o site abre; depois, cuide de tudo pelo painel.
-->
`
}

func (g *ger) guarda() string {
	return `<!-- Só aparece se alguém abrir este arquivo direto (sem o nep): esconde o código e explica. O nep tira os trechos nep-fonte. -->
<style nep-fonte>
  html, body { background: #14201c !important; margin: 0 !important; }
  body > :not([nep-fonte]) { display: none !important; }
  aside[nep-fonte] { display: block !important; box-sizing: border-box; max-width: 600px; margin: 9vh auto; padding: 36px 34px; border-radius: 22px;
    background: #fff; color: #1d2420; font: 17px/1.6 system-ui, -apple-system, "Segoe UI", sans-serif; box-shadow: 0 30px 70px rgba(0,0,0,.35); }
  aside[nep-fonte] small { font-weight: 800; letter-spacing: .14em; font-size: .72rem; color: ` + g.p.Cor + `; }
  aside[nep-fonte] h1 { font-size: 1.9rem; line-height: 1.15; margin: 8px 0 12px; }
  aside[nep-fonte] ol { padding-left: 1.2em; } aside[nep-fonte] li { margin: .5em 0; }
  aside[nep-fonte] code { background: #eef2ee; padding: 2px 6px; border-radius: 6px; font-size: .9em; }
  aside[nep-fonte] a { color: ` + g.p.Cor + `; font-weight: 700; }
</style>
<aside nep-fonte>
  <small>NEPENTHES · CÓDIGO DO SITE</small>
  <h1>«nome»</h1>
  <p>Este arquivo é o código do site, como um arquivo .php: quem monta as páginas, guarda os dados e manda os e-mails é o programa <b>nep</b>. Aberto direto, ele não mostra o site.</p>
  <ol>
    <li><b>Ver como ficou:</b> abra a <a href="site-previa.html">prévia (site-previa.html)</a>. Para criar ou atualizar a prévia: <code>nep previa site.nep.html</code></li>
    <li><b>No computador:</b> <code>nep dev site.nep.html</code> e abra <code>http://localhost:8000</code></li>
    <li><b>No ar:</b> envie para a hospedagem com o <code>nep.cgi</code> e o <code>.htaccess</code>.</li>
  </ol>
</aside>
`
}

// ---------------------------------------------------------------- conteúdos (o que o painel edita)

func opcoes(xs []string) string {
	var b strings.Builder
	for _, x := range xs {
		b.WriteString("<option>" + esc(x) + "</option>")
	}
	return b.String()
}

func (g *ger) conteudos() string {
	var b strings.Builder
	b.WriteString("\n<!-- ================= Conteúdo que se publica pelo painel ================= -->\n")
	if g.tem("servicos") {
		b.WriteString(`<conteudo nome="servicos" titulo="«servR»" item="item">
  <label>Nome <input name="titulo" required maxlength="100"></label>
  <label>Ícone (um emoji) <input name="icone" maxlength="8"></label>
  <label>Resumo <textarea name="resumo" maxlength="300" rows="2"></textarea></label>
  <label>Descrição completa <textarea name="corpo" data-editor maxlength="20000"></textarea></label>
  <label>Preço (vazio = sob consulta) <input name="preco" type="number" step="0.01" min="0"></label>
  <label>Ordem <input name="ordem" type="number" min="0" max="999"></label>
  <label>Imagem <input type="file" name="capa" accept="image/*" data-max-mb="5" data-publico></label>
</conteudo>
`)
	}
	if g.tem("catalogo") {
		b.WriteString(`<conteudo nome="produtos" titulo="«lojaR»" item="produto">
  <label>Nome <input name="titulo" required maxlength="100"></label>
  <label>Categoria <input name="categoria" required maxlength="40"></label>
  <label>Preço <input name="preco" type="number" step="0.01" min="0" required></label>
  <label>Ícone (um emoji, aparece quando não há foto) <input name="icone" maxlength="8"></label>
  <label>Resumo <textarea name="resumo" maxlength="300" rows="2"></textarea></label>
  <label>Descrição <textarea name="corpo" data-editor maxlength="20000"></textarea></label>
  <label>Foto <input type="file" name="foto" accept="image/*" data-max-mb="5" data-publico></label>
</conteudo>
`)
	}
	if g.tem("eventos") {
		b.WriteString(`<conteudo nome="eventos" titulo="Eventos" item="evento">
  <label>Nome do evento <input name="titulo" required maxlength="120"></label>
  <label>Data <input name="data" type="date" required></label>
  <label>Horário <input name="hora" maxlength="20" placeholder="19:00"></label>
  <label>Local <input name="local" maxlength="120"></label>
  <label>Vagas (vazio = sem limite) <input name="vagas" type="number" min="0" max="100000"></label>
  <label>Resumo <textarea name="resumo" maxlength="300" rows="2"></textarea></label>
  <label>Programação e detalhes <textarea name="corpo" data-editor maxlength="20000"></textarea></label>
  <label>Imagem <input type="file" name="capa" accept="image/*" data-max-mb="5" data-publico></label>
</conteudo>
`)
	}
	if g.tem("equipe") {
		b.WriteString(`<conteudo nome="equipe" titulo="«equipeR»" item="pessoa">
  <label>Nome <input name="titulo" required maxlength="80"></label>
  <label>Cargo ou função <input name="cargo" maxlength="80"></label>
  <label>Sobre <textarea name="resumo" maxlength="400" rows="3"></textarea></label>
  <label>Foto <input type="file" name="foto" accept="image/*" data-max-mb="5" data-publico></label>
</conteudo>
`)
	}
	if g.tem("galeria") {
		b.WriteString(`<conteudo nome="galeria" titulo="Galeria" item="foto">
  <label>Título <input name="titulo" required maxlength="100"></label>
  <label>Álbum <input name="categoria" maxlength="40"></label>
  <label>Foto <input type="file" name="foto" accept="image/*" data-max-mb="8" data-publico></label>
  <label>Legenda <textarea name="resumo" maxlength="300" rows="2"></textarea></label>
</conteudo>
`)
	}
	if g.tem("blog") {
		b.WriteString(`<conteudo nome="posts" titulo="«blogR»" item="post">
  <label>Título <input name="titulo" required maxlength="140"></label>
  <label>Resumo <textarea name="resumo" maxlength="300" rows="2"></textarea></label>
  <label>Texto <textarea name="corpo" data-editor required maxlength="60000"></textarea></label>
  <label>Categoria <select name="categoria">` + opcoes(g.s.Cats) + `</select></label>
  <label>Tags (separe por vírgula) <input name="tags" maxlength="200"></label>
  <label>Autor <input name="autor" maxlength="80"></label>
  <label>Imagem de capa <input type="file" name="capa" accept="image/*" data-max-mb="5" data-publico></label>
</conteudo>
`)
	}
	if g.tem("wiki") {
		b.WriteString(`<conteudo nome="wiki" titulo="Wiki" item="página da wiki">
  <label>Título <input name="titulo" required maxlength="120"></label>
  <label>Categoria <input name="categoria" maxlength="40"></label>
  <label>Texto (use [[Nome de outra página]] para criar links) <textarea name="corpo" data-editor required maxlength="80000"></textarea></label>
</conteudo>
`)
	}
	if g.tem("forum") {
		b.WriteString(`<conteudo nome="topicos" titulo="Fórum" item="tópico">
  <label>Título <input name="titulo" required maxlength="140"></label>
  <label>Categoria <select name="categoria"><option>Geral</option><option>Dúvidas</option><option>Ideias</option><option>Avisos</option></select></label>
  <label>Mensagem <textarea name="corpo" required maxlength="10000" rows="8"></textarea></label>
  <label>Autor <input name="autor" required maxlength="60"></label>
  <label>E-mail do autor (não aparece no site) <input name="email" type="email"></label>
</conteudo>
`)
	}
	if g.tem("downloads") {
		b.WriteString(`<conteudo nome="arquivos" titulo="Downloads" item="arquivo">
  <label>Título <input name="titulo" required maxlength="120"></label>
  <label>Categoria <input name="categoria" maxlength="40"></label>
  <label>Descrição <textarea name="resumo" maxlength="300" rows="2"></textarea></label>
  <label>Arquivo <input type="file" name="arquivo" accept=".pdf,.doc,.docx,.odt,.xls,.xlsx,.ods,.zip,image/*" data-max-mb="20" data-publico></label>
</conteudo>
`)
	}
	if g.tem("vagas") {
		b.WriteString(`<conteudo nome="vagas" titulo="Vagas" item="vaga">
  <label>Cargo <input name="titulo" required maxlength="100"></label>
  <label>Área <input name="area" maxlength="60"></label>
  <label>Tipo <select name="tipo"><option>CLT</option><option>PJ</option><option>Estágio</option><option>Temporário</option><option>Voluntário</option></select></label>
  <label>Local <input name="local" maxlength="80"></label>
  <label>Resumo <textarea name="resumo" maxlength="300" rows="2"></textarea></label>
  <label>Descrição, requisitos e benefícios <textarea name="corpo" data-editor maxlength="20000"></textarea></label>
</conteudo>
`)
	}
	if g.tem("faq") {
		b.WriteString(`<conteudo nome="perguntas" titulo="Perguntas frequentes" item="pergunta">
  <label>Pergunta <input name="titulo" required maxlength="200"></label>
  <label>Resposta <textarea name="resposta" required maxlength="3000" rows="4"></textarea></label>
  <label>Ordem <input name="ordem" type="number" min="0" max="999"></label>
</conteudo>
`)
	}
	b.WriteString(`<conteudo nome="paginas" titulo="Páginas" item="página">
  <label>Título <input name="titulo" required maxlength="100"></label>
  <label>Texto <textarea name="corpo" data-editor maxlength="60000"></textarea></label>
  <label>Montagem visual (opcional) <textarea name="visual" data-construtor></textarea></label>
</conteudo>
`)
	return b.String()
}

// ---------------------------------------------------------------- código (<nep>)

func dinheiro(v string) string {
	if v == "" {
		return ""
	}
	return v
}

func (g *ger) codigo() string {
	s, p := g.s, g.p
	var b strings.Builder
	w := func(x string) { b.WriteString(x) }
	w("\n<nep>\n")
	w(`  let hoje = date(now(), "Y-m-d")
  let MESES = ["janeiro", "fevereiro", "março", "abril", "maio", "junho", "julho", "agosto", "setembro", "outubro", "novembro", "dezembro"]
  let MES_CURTO = ["jan", "fev", "mar", "abr", "mai", "jun", "jul", "ago", "set", "out", "nov", "dez"]
  fn dias_depois(n) do return date(int(date(now(), "U")) + n * 86400, "Y-m-d") end
  # Inicial para o avatar: pula "Dr.", "Prof.ª" e afins.
  fn inicial(n) do
    let r = gsub(default(n, "?"), "^%S+%.%S*%s+", "")
    return upper(substr(r, 0, 1))
  end
  fn preco_txt(v) do
    if not v do return "Sob consulta" end
    return money(v)
  end

`)
	// ---- conteúdo de exemplo: entra uma vez só (a marca fica na coleção "instalacao")
	w("  # Conteúdo de exemplo: entra só na primeira vez. Depois, tudo pelo painel.\n")
	w("  if db.count(\"instalacao\") == 0 do\n")
	w("    db.insert(\"instalacao\", {feito: true, versao: \"nep criar\"})\n")
	if g.tem("servicos") {
		for i, it := range s.Servicos {
			preco := ""
			if it.Preco != "" {
				preco = ", preco: " + it.Preco
			}
			corpo := it.Texto + "\n\n## Como funciona\n\n- Atendimento com hora marcada\n- Explicamos tudo antes de começar\n- Acompanhamento depois do serviço"
			fmt.Fprintf(&b, "    db.insert(\"servicos\", {titulo: %s, slug: %s, publicado: true, icone: %s, resumo: %s, corpo: %s, ordem: %d%s})\n",
				lit(it.Titulo), lit(slugify(it.Titulo)), lit(it.Icone), lit(it.Texto), lit(corpo), i+1, preco)
		}
	}
	if g.tem("catalogo") {
		for _, it := range s.Produtos {
			fmt.Fprintf(&b, "    db.insert(\"produtos\", {titulo: %s, slug: %s, publicado: true, categoria: %s, preco: %s, icone: %s, resumo: %s, corpo: %s})\n",
				lit(it.Titulo), lit(slugify(it.Titulo)), lit(it.Cat), strconv.FormatFloat(it.Preco, 'f', -1, 64), lit(it.Icone), lit(it.Texto), lit(it.Texto))
		}
	}
	if g.tem("eventos") {
		for _, e := range s.Eventos {
			vagas := ""
			if e.Vagas > 0 {
				vagas = fmt.Sprintf(", vagas: %d", e.Vagas)
			}
			corpo := e.Resumo + "\n\n## Programação\n\n- Recepção e boas-vindas\n- Atividade principal\n- Encerramento e conversa\n\nA inscrição é gratuita e as vagas são limitadas."
			fmt.Fprintf(&b, "    db.insert(\"eventos\", {titulo: %s, slug: %s, publicado: true, data: dias_depois(%d), hora: %s, local: %s, resumo: %s, corpo: %s%s})\n",
				lit(e.Titulo), lit(slugify(e.Titulo)), e.Dias, lit(e.Hora), lit(e.Local), lit(e.Resumo), lit(corpo), vagas)
		}
	}
	if g.tem("equipe") {
		for _, pe := range s.Equipe {
			fmt.Fprintf(&b, "    db.insert(\"equipe\", {titulo: %s, slug: %s, publicado: true, cargo: %s, resumo: %s})\n",
				lit(pe.Nome), lit(slugify(pe.Nome)), lit(pe.Cargo), lit(pe.Texto))
		}
	}
	if g.tem("galeria") {
		for i, t := range s.Galeria {
			alb := "Nosso espaço"
			if i%2 == 1 {
				alb = "Trabalhos"
			}
			fmt.Fprintf(&b, "    db.insert(\"galeria\", {titulo: %s, slug: %s, publicado: true, categoria: %s, resumo: %s})\n",
				lit(t), lit(slugify(t)), lit(alb), lit("Troque por uma foto de verdade no painel."))
		}
	}
	if g.tem("blog") {
		for i, po := range s.Posts {
			corpo := po.Resumo + "\n\n## O que você precisa saber\n\nEste é um texto de exemplo criado junto com o site. Edite ou apague no painel, em **" + s.BlogRotulo + "**.\n\n- Escreva como fala com o cliente\n- Use intertítulos para facilitar a leitura\n- Termine com um convite para o próximo passo\n\n> Dica: posts com imagem de capa aparecem melhor no WhatsApp e nas redes."
			fmt.Fprintf(&b, "    db.insert(\"posts\", {titulo: %s, slug: %s, publicado: true, resumo: %s, corpo: %s, categoria: %s, tags: %s, autor: %s})\n",
				lit(po.Titulo), lit(slugify(po.Titulo)), lit(po.Resumo), lit(corpo), lit(po.Cat), lit(po.Tags), lit([]string{"Equipe " + p.Nome, s.Equipe[0].Nome}[i%2]))
		}
	}
	if g.tem("wiki") {
		paginas := [][3]string{
			{"Página inicial", "Geral", "Bem-vindo à wiki de **" + p.Nome + "**. Aqui fica o conhecimento que a gente junta com o tempo.\n\n## Por onde começar\n\n- [[Como contribuir]]\n- [[Glossário]]\n- [[Perguntas comuns]]\n\nQualquer pessoa pode sugerir uma edição: a equipe revisa e publica."},
			{"Como contribuir", "Ajuda", "Viu algo errado ou faltando? Clique em **Sugerir edição** no fim da página.\n\n## Links entre páginas\n\nEscreva o nome de outra página entre colchetes duplos, assim: [[Glossário]]. Se a página ainda não existe, o link leva para criá-la.\n\nVolte para a [[Página inicial]]."},
			{"Glossário", "Referência", "Termos que aparecem por aqui, em ordem alfabética.\n\n**Página**: um texto da wiki.\n\n**Sugestão**: uma edição enviada por um visitante, que a equipe revisa.\n\nVeja também: [[Como contribuir]]."},
		}
		for _, pg := range paginas {
			fmt.Fprintf(&b, "    db.insert(\"wiki\", {titulo: %s, slug: %s, publicado: true, categoria: %s, corpo: %s})\n", lit(pg[0]), lit(slugify(pg[0])), lit(pg[1]), lit(pg[2]))
		}
	}
	if g.tem("forum") {
		fmt.Fprintf(&b, "    db.insert(\"topicos\", {titulo: %s, slug: \"boas-vindas\", publicado: true, categoria: \"Geral\", autor: \"Moderação\", corpo: %s})\n",
			lit("Boas-vindas! Apresente-se aqui"), lit("Que bom ter você por aqui. Conte quem você é e o que te trouxe. Responda nos comentários abaixo."))
		fmt.Fprintf(&b, "    db.insert(\"topicos\", {titulo: %s, slug: \"regras\", publicado: true, categoria: \"Avisos\", autor: \"Moderação\", corpo: %s})\n",
			lit("Regras da comunidade"), lit("1. Respeito acima de tudo.\n2. Procure antes de perguntar: talvez já tenham respondido.\n3. Nada de propaganda.\n\nTópicos novos passam pela moderação antes de aparecer."))
	}
	if g.tem("downloads") {
		var docs [][3]string
		switch s.ID {
		case "associacao":
			docs = [][3]string{{"Estatuto social", "Institucional", "O estatuto em vigor, aprovado em assembleia."}, {"Ficha de associação", "Formulários", "Preencha e entregue na sede."}, {"Balancete do mês", "Prestação de contas", "Entradas e saídas, com comprovantes."}, {"Ata da última assembleia", "Atas", "Resumo das decisões."}}
		case "comunidade":
			docs = [][3]string{{"Manual do usuário", "Manuais", "O guia completo, em PDF."}, {"Guia rápido", "Manuais", "Uma página para imprimir e deixar do lado."}, {"Kit de marca", "Materiais", "Logotipos e cores para divulgação."}}
		default:
			docs = [][3]string{{"Catálogo completo", "Materiais", "Tudo o que oferecemos, com fotos."}, {"Tabela de preços", "Materiais", "Valores atualizados."}, {"Política de privacidade (PDF)", "Institucional", "Como tratamos os seus dados (LGPD)."}}
		}
		for _, d := range docs {
			fmt.Fprintf(&b, "    db.insert(\"arquivos\", {titulo: %s, slug: %s, publicado: true, categoria: %s, resumo: %s})\n", lit(d[0]), lit(slugify(d[0])), lit(d[1]), lit(d[2]))
		}
	}
	if g.tem("vagas") {
		fmt.Fprintf(&b, "    db.insert(\"vagas\", {titulo: \"Atendimento ao cliente\", slug: \"atendimento-ao-cliente\", publicado: true, area: \"Atendimento\", tipo: \"CLT\", local: %s, resumo: \"Atender clientes no balcão, por telefone e WhatsApp.\", corpo: %s})\n",
			lit(strings.TrimPrefix(g.vars["«cidade»"], " em ")+map[bool]string{true: "Presencial", false: ""}[p.Cidade == ""]), lit("## O que você vai fazer\n\n- Atender clientes com simpatia\n- Organizar a agenda\n\n## O que esperamos\n\n- Gostar de gente\n- Ensino médio completo\n\n## Benefícios\n\n- Vale-transporte e vale-refeição\n- Plano de saúde"))
		w("    db.insert(\"vagas\", {titulo: \"Banco de talentos\", slug: \"banco-de-talentos\", publicado: true, area: \"Todas\", tipo: \"CLT\", local: \"A combinar\", resumo: \"Não achou a vaga certa? Deixe seu currículo para as próximas.\", corpo: \"Guardamos o seu currículo por 12 meses e chamamos quando surgir algo com o seu perfil.\"})\n")
	}
	if g.tem("faq") {
		for i, f := range s.Faq {
			fmt.Fprintf(&b, "    db.insert(\"perguntas\", {titulo: %s, slug: %s, publicado: true, resposta: %s, ordem: %d})\n", lit(f[0]), lit(slugify(f[0])), lit(f[1]), i+1)
		}
	}
	sobre := "Somos **" + p.Nome + "**" + strings.ReplaceAll(g.vars["«cidade»"], "&#39;", "'") + ". " + s.Texto + "\n\n## Nossa história\n\nConte aqui como tudo começou, o que move a equipe e o que torna o seu trabalho diferente.\n\n## Missão\n\nUma frase que resume por que vocês existem."
	priv := "Esta política explica como **" + p.Nome + "** trata os dados pessoais de quem usa este site, conforme a Lei Geral de Proteção de Dados (Lei 13.709/2018).\n\n## Que dados coletamos\n\nSó o que você digita nos formulários (nome, e-mail, telefone e a mensagem) e dados técnicos de acesso, sem identificar você.\n\n## Para que usamos\n\nPara responder, prestar o serviço pedido e cumprir obrigações legais. Não vendemos nem compartilhamos seus dados para propaganda.\n\n## Seus direitos\n\nVocê pode pedir para ver, corrigir ou apagar seus dados a qualquer momento pela página de contato."
	fmt.Fprintf(&b, "    db.insert(\"paginas\", {titulo: \"Sobre\", slug: \"sobre\", publicado: true, corpo: %s})\n", lit(sobre))
	fmt.Fprintf(&b, "    db.insert(\"paginas\", {titulo: \"Política de privacidade\", slug: \"privacidade\", publicado: true, corpo: %s})\n", lit(priv))
	w("  end\n\n")

	// ---- variáveis e funções por módulo
	if g.tem("servicos") {
		w(`  let servicos = db.all("servicos") |> sort_by("ordem")
`)
	}
	if g.tem("faq") {
		w(`  let perguntas = db.all("perguntas") |> sort_by("ordem")
`)
	}
	if g.tem("idiomas") {
		w(`
  # ---------- tradução (villosa, com a IA ligada): ?idioma=en ----------
  let IDIOMA = default(request.query.idioma, "")
  fn tr(t) do
    if IDIOMA != "" and IDIOMA != "pt" and t and ia_ligada() do return villosa.traduzir(t, IDIOMA) end
    return t
  end
`)
	} else {
		w("  fn tr(t) do return t end\n")
	}
	if g.tem("agenda") {
		if s.AgendaModo == "diarias" {
			var qs []string
			for _, it := range s.Servicos {
				qs = append(qs, lit(it.Titulo))
			}
			w(`
  # ---------- reservas por diária ----------
  let QUARTOS = [` + strings.Join(qs, ", ") + `]
  fn conflita(r, q, ent, sai) do return r.quarto == q and r.entrada < sai and ent < r.saida end
  fn quarto_livre(q, ent, sai) do
    return len(db.all("reservas") |> filter(fn(r) do conflita(r, q, ent, sai) end)) == 0
  end
  fn noites(ent, sai) do return round((int(date(sai, "U")) - int(date(ent, "U"))) / 86400) end
  # Ocupação dos próximos dias: para cada dia, quais acomodações estão livres.
  fn mapa_dias(n) do
    let out = []
    for i in range(n) do
      let d = dias_depois(i)
      let fim = dias_depois(i + 1)
      push(out, {txt: date(d, "d/m"), livres: QUARTOS |> filter(fn(q) do quarto_livre(q, d, fim) end)})
    end
    return out
  end
  # Chamada pelo formulário de reserva, com os dados já conferidos.
  fn reservar(a) do
    if a.entrada < hoje do recusar("A entrada precisa ser de hoje em diante.") end
    if a.saida <= a.entrada do recusar("A saída precisa ser depois da entrada.") end
    if noites(a.entrada, a.saida) > 30 do recusar("Para mais de 30 noites, fale com a gente pelo contato.") end
    if not quarto_livre(a.quarto, a.entrada, a.saida) do
      recusar("Essa acomodação já tem reserva em parte dessas datas. Veja o quadro de datas livres ao lado.")
    end
    a.noites = noites(a.entrada, a.saida)
`)
			if g.tem("servicos") {
				w(`    let q = find(servicos, fn(s) do s.titulo == a.quarto end)
    if q and q.preco do a.total = a.noites * q.preco end
`)
			}
			w(`    a.status = "Pré-reserva"
    return "«agU»/ok?entrada=" + a.entrada + "&saida=" + a.saida + "&q=" + urlencode(a.quarto)
  end
`)
		} else {
			var hs []string
			for _, h := range s.Horarios {
				hs = append(hs, lit(h))
			}
			w(`
  # ---------- agenda ----------
  let HORARIOS = [` + strings.Join(hs, ", ") + `]
  let POR_HORARIO = ` + strconv.Itoa(s.PorHorario) + `   # quantas reservas cabem no mesmo horário
  let DIAS = ["domingo", "segunda", "terça", "quarta", "quinta", "sexta", "sábado"]
  fn ocupacao(dia, h) do
    return len(db.where("agendamentos", "data", dia) |> filter(fn(a) do a.horario == h end))
  end
  fn livres(dia) do
    let lista = HORARIOS |> filter(fn(h) do ocupacao(dia, h) < POR_HORARIO end)
    if dia == hoje do
      let agora = date(now(), "H:i")
      lista = lista |> filter(fn(h) do h > agora end)
    end
    return lista
  end
  # Próximos dias de atendimento (segunda a sábado).
  fn proximos_dias(n) do
    let out = []
    let t = int(date(now(), "U"))
    while len(out) < n do
      let d = date(t, "Y-m-d")
      if date(t, "w") != "0" do push(out, {data: d, txt: DIAS[int(date(t, "w"))] + ", " + date(t, "d/m")}) end
      t = t + 86400
    end
    return out
  end
  # Chamada pelo formulário, com os dados já conferidos.
  fn agendar(a) do
    if a.data < hoje do recusar("Escolha uma data de hoje em diante.") end
    if date(a.data, "w") == "0" do recusar("Aos domingos não abrimos. Escolha outro dia.") end
    if not contains(livres(a.data), a.horario) do
      recusar("Esse horário acabou de ser reservado ou já passou. Veja os horários livres ao lado.")
    end
    a.status = "Confirmado"
    return "«agU»/ok?data=" + a.data + "&h=" + urlencode(a.horario)
  end
`)
		}
	}
	if g.tem("catalogo") {
		w(`
  # ---------- loja: carrinho na sessão do visitante ----------
  let produtos = db.all("produtos")
  let CATEGORIAS = produtos |> pluck("categoria") |> unique
  fn carrinho() do return default(session.carrinho, []) end
  fn total_carrinho() do
    let t = 0
    for i in carrinho() do t = t + i.preco * i.qtd end
    return t
  end
  fn itens_carrinho() do
    let n = 0
    for i in carrinho() do n = n + i.qtd end
    return n
  end
  fn adicionar(f) do
    let p = first(db.where("produtos", "slug", f.produto))
    if not p do recusar("Produto não encontrado.") end
    let q = max(1, min(50, int(default(f.qtd, 1))))
    let c = carrinho()
    let achou = false
    for i in c do
      if i.slug == p.slug do
        i.qtd = min(50, i.qtd + q)
        i.preco = p.preco
        achou = true
      end
    end
    if not achou do push(c, {slug: p.slug, titulo: p.titulo, preco: p.preco, qtd: q}) end
    session.carrinho = c
    return "/carrinho"
  end
  fn remover(f) do
    session.carrinho = carrinho() |> filter(fn(i) do i.slug != f.produto end)
    return "/carrinho"
  end
  # Fecha o pedido: os preços vêm do banco, nunca do navegador.
  fn fechar_pedido(a) do
    let c = carrinho()
    if len(c) == 0 do recusar("Seu carrinho está vazio.") end
    if a.entrega == "Entrega" and not a.endereco do recusar("Para entrega, informe o endereço.") end
    let linhas = []
    let total = 0
    for i in c do
      let p = first(db.where("produtos", "slug", i.slug))
      if p do
        push(linhas, str(i.qtd) + " × " + p.titulo + " = " + money(p.preco * i.qtd))
        total = total + p.preco * i.qtd
      end
    end
    a.itens = join(linhas, "\n")
    a.total = total
    a.status = "Novo"
    session.carrinho = []
    return "/pedido/ok?total=" + urlencode(money(total))
  end
`)
	}
	if g.tem("eventos") {
		w(`
  # ---------- eventos com inscrição ----------
  let eventos = db.all("eventos") |> sort_by("data")
  let proximos_eventos = eventos |> filter(fn(e) do default(e.data, "") >= hoje end)
  let eventos_passados = eventos |> filter(fn(e) do default(e.data, "") < hoje end) |> reverse
  fn inscritos(e) do return len(db.where("inscricoes", "evento", e.slug)) end
  fn restam(e) do
    if not e.vagas do return nil end
    return max(0, int(e.vagas) - inscritos(e))
  end
  fn dia_ev(e) do return date(e.data, "d") end
  fn mes_ev(e) do return MES_CURTO[int(date(e.data, "m")) - 1] end
  fn inscrever(a) do
    let e = first(db.where("eventos", "slug", a.evento))
    if not e do recusar("Evento não encontrado.") end
    if e.data < hoje do recusar("Este evento já aconteceu.") end
    let r = restam(e)
    if r != nil and r <= 0 do recusar("As vagas acabaram. Fique de olho nos próximos eventos!") end
    let ja = db.where("inscricoes", "evento", e.slug) |> filter(fn(i) do lower(i.email) == lower(a.email) end)
    if len(ja) > 0 do recusar("Este e-mail já está inscrito neste evento.") end
    a.evento_nome = e.titulo
    a.data_evento = e.data
    return "/eventos/" + e.slug + "?inscrito=1"
  end
`)
	}
	if g.tem("blog") {
		w(`
  # ---------- blog ----------
  let POR_PAGINA = 6
  fn tags_de(p) do
    let out = []
    for t in split(default(p.tags, ""), ",") do
      let x = trim(t)
      if x != "" do push(out, x) end
    end
    return out
  end
  fn leitura(p) do
    let n = 0
    for w in gmatch(default(p.corpo, ""), "%S+") do n = n + 1 end
    return max(1, int(ceil(n / 200)))
  end
  fn mes_txt(chave) do
    return MESES[int(substr(chave, 5, 2)) - 1] + " de " + substr(chave, 0, 4)
  end
  # Lista do blog com os filtros da URL: ?categoria= ?tag= ?mes=2026-11 ?q=
  fn posts_filtrados() do
    let q = request.query
    let lista = db.all("posts")
    if q.categoria do lista = lista |> filter(fn(p) do p.categoria == q.categoria end) end
    if q.tag do lista = lista |> filter(fn(p) do contains(tags_de(p), q.tag) end) end
    if q.mes do lista = lista |> filter(fn(p) do date(p.created_at, "Y-m") == q.mes end) end
    return lista
  end
  fn pagina_atual() do
    if match(default(request.query.pagina, ""), "^%d+$") do return max(1, int(request.query.pagina)) end
    return 1
  end
  fn pagina_de(lista) do return lista |> skip((pagina_atual() - 1) * POR_PAGINA) |> take(POR_PAGINA) end
  fn total_paginas(lista) do return max(1, int(ceil(len(lista) / POR_PAGINA))) end
  fn link_pagina(n) do
    let u = "«blogU»?pagina=" + str(n)
    for k in ["categoria", "tag", "mes"] do
      if request.query[k] do u = u + "&" + k + "=" + urlencode(request.query[k]) end
    end
    return url(u)
  end
  fn filtro_txt() do
    let q = request.query
    if q.categoria do return "Categoria: " + q.categoria end
    if q.tag do return "Tag: " + q.tag end
    if q.mes do return "Arquivo: " + mes_txt(q.mes) end
    return ""
  end
  fn categorias_blog() do
    let c = db.all("posts") |> count_by("categoria")
    let out = []
    for k in keys(c) do push(out, {nome: k, n: c[k]}) end
    return out |> sort_by("nome")
  end
  fn todas_tags() do
    let out = []
    for p in db.all("posts") do
      for t in tags_de(p) do push(out, t) end
    end
    return out |> unique |> sort
  end
  fn arquivo_blog() do
    let out = []
    for p in db.all("posts") do push(out, date(p.created_at, "Y-m")) end
    return out |> unique
  end
  fn relacionados(p) do
    return db.all("posts") |> filter(fn(o) do o.categoria == p.categoria and o.id != p.id end) |> take(3)
  end
`)
	}
	if g.tem("wiki") {
		w(`
  # ---------- wiki ----------
  # [[Página]] e [[Página|texto]] viram links; página que não existe leva para criá-la.
  fn wiki_links(t) do
    let r = gsub(default(t, ""), "%[%[([^%]|]+)|?([^%]]*)%]%]", fn(alvo, texto) do
      if texto == "" do texto = alvo end
      return "[" + texto + "](" + url("/wiki/" + slug(alvo)) + ")"
    end)
    return r
  end
  fn wiki_pagina(s) do return first(db.where("wiki", "slug", s)) end
  fn wiki_busca() do
    let q = lower(default(request.query.q, ""))
    let todas = db.all("wiki") |> sort_by("titulo")
    if q == "" do return todas end
    return todas |> filter(fn(p) do contains(lower(p.titulo), q) or contains(lower(default(p.corpo, "")), q) end)
  end
  fn wiki_letras() do
    let grupos = wiki_busca() |> group_by(fn(p) do upper(substr(p.titulo, 0, 1)) end)
    let out = []
    for k in keys(grupos) do push(out, {letra: k, paginas: grupos[k]}) end
    return out |> sort_by("letra")
  end
  fn wiki_recentes() do
    return db.all("wiki") |> sort_by(fn(p) do default(p.updated_at, p.created_at) end) |> reverse |> take(8)
  end
  fn wiki_categorias() do
    let c = db.all("wiki") |> count_by(fn(p) do default(p.categoria, "Geral") end)
    let out = []
    for k in keys(c) do push(out, {nome: k, n: c[k]}) end
    return out |> sort_by("nome")
  end
  fn apontam_para(p) do
    return db.all("wiki") |> filter(fn(o) do o.id != p.id and contains(lower(default(o.corpo, "")), "[[" + lower(p.titulo)) end)
  end
  fn sugerir(a) do
    if not a.pagina do a.pagina = slug(a.titulo) end
    a.status = "Para revisar"
    return "/wiki/" + a.pagina + "?sugestao=1"
  end
`)
	}
	if g.tem("forum") {
		w(`
  # ---------- fórum (moderado: tópico novo entra como rascunho) ----------
  let FORUM_CATS = ["Geral", "Dúvidas", "Ideias", "Avisos"]
  fn topicos_lista() do
    let lista = db.all("topicos")
    if request.query.cat do lista = lista |> filter(fn(t) do t.categoria == request.query.cat end) end
    return lista
  end
  fn novo_topico(a) do
    a.slug = slug(a.titulo) + "-" + date(now(), "U")
    a.publicado = false
    return "/forum?enviado=1"
  end
`)
	}
	if g.tem("downloads") {
		w(`
  fn arquivos_por_categoria() do
    let g = db.all("arquivos") |> group_by(fn(a) do default(a.categoria, "Outros") end)
    let out = []
    for k in keys(g) do push(out, {nome: k, itens: g[k]}) end
    return out |> sort_by("nome")
  end
`)
	}
	if g.tem("vagas") {
		w(`
  fn candidatar(a) do
    let v = first(db.where("vagas", "slug", a.vaga))
    if v do a.vaga_nome = v.titulo end
    return "/trabalhe-conosco?enviado=1"
  end
`)
	}
	if g.tem("galeria") {
		w(`
  fn fotos() do
    let lista = db.all("galeria")
    if request.query.album do lista = lista |> filter(fn(f) do f.categoria == request.query.album end) end
    return lista
  end
  let ALBUNS = db.all("galeria") |> pluck("categoria") |> unique
`)
	}
	w("</nep>\n")
	return b.String()
}

// ---------------------------------------------------------------- APIs (JSON para app, integrações)

func (g *ger) apis() string {
	var b strings.Builder
	if g.tem("agenda") && g.s.AgendaModo != "diarias" {
		b.WriteString(`
<!-- Horários livres de um dia, em JSON: /api/horarios?data=2026-12-01 -->
<api route="/api/horarios">
  let dia = default(request.query.data, hoje)
  return {data: dia, livres: livres(dia)}
</api>
`)
	}
	if g.tem("agenda") && g.s.AgendaModo == "diarias" {
		b.WriteString(`
<!-- Acomodações livres nos próximos 14 dias, em JSON -->
<api route="/api/disponibilidade">
  return mapa_dias(14)
</api>
`)
	}
	if g.tem("catalogo") {
		b.WriteString(`
<!-- Produtos em JSON (para um aplicativo ou integração): /api/produtos -->
<api route="/api/produtos">
  let out = []
  for p in produtos do push(out, {nome: p.titulo, categoria: p.categoria, preco: p.preco, url: url("«lojaU»/" + p.slug)}) end
  return out
</api>
`)
	}
	if g.tem("eventos") {
		b.WriteString(`
<!-- Próximos eventos em JSON: /api/eventos -->
<api route="/api/eventos">
  let out = []
  for e in proximos_eventos do push(out, {nome: e.titulo, data: e.data, hora: e.hora, local: e.local, vagas_restantes: restam(e), url: url("/eventos/" + e.slug)}) end
  return out
</api>
`)
	}
	return b.String()
}
