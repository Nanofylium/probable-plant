package web

// Páginas visuais: o construtor (GrapesJS, BSD-3-Clause) edita a página
// INTEIRA arrastando, no estilo Wix, sem perder o código do Nepenthes.
//
// A fonte da página é dividida em duas coisas: o HTML comum, que vira blocos
// editáveis, e as "ilhas" (laços, condições, formulários, regiões ao vivo,
// {{ expressões }}, SVG, tudo que não é HTML simples), que aparecem no
// construtor como blocos fechados, com a prévia de verdade, e voltam
// intactas ao salvar. A página montada fica no banco do site
// (visual:pagina:/rota) e substitui o miolo da <page> na hora de carregar o
// site: o arquivo .nep.html não muda, então funciona também no CGI e nos
// moldes. "Voltar ao original" só apaga o registro.

import (
	"encoding/json"
	"errors"
	"fmt"
	"html"
	"net/http"
	"net/url"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
)

const paginaMeta = "visual:pagina:"

type paginaVisual struct {
	Fonte  string `json:"fonte"`            // miolo da <page>, já com as ilhas
	Titulo string `json:"titulo,omitempty"` // só para páginas criadas no construtor
	Nova   bool   `json:"nova,omitempty"`
	Em     string `json:"em"`
	Por    string `json:"por"`
}

func (s *Server) paginaVisual(route string) *paginaVisual {
	var p paginaVisual
	if json.Unmarshal([]byte(s.Store.Meta(paginaMeta+route)), &p) != nil || p.Fonte == "" && !p.Nova {
		return nil
	}
	return &p
}

// rotasVisuais: as páginas que têm versão montada no construtor.
func (s *Server) rotasVisuais() []string {
	var out []string
	if v := s.Store.Meta(paginaMeta + "*"); v != "" {
		json.Unmarshal([]byte(v), &out)
	}
	sort.Strings(out)
	return out
}

func (s *Server) lembrarRota(route string, tem bool) error {
	rs := s.rotasVisuais()
	var out []string
	for _, r := range rs {
		if r != route {
			out = append(out, r)
		}
	}
	if tem {
		out = append(out, route)
	}
	b, _ := json.Marshal(out)
	return s.Store.SetMeta(paginaMeta+"*", string(b))
}

// ---------- achar a <page> na fonte ----------

var (
	rePageAbre  = regexp.MustCompile(`(?i)<page\b((?:[^>"']|"[^"]*"|'[^']*')*)>`)
	rePageFecha = regexp.MustCompile(`(?i)</page\s*>`)
)

// miolo devolve onde começa e termina o conteúdo da <page route="...">.
func miolo(src, route string) (ini, fim int, ok bool) {
	for _, m := range rePageAbre.FindAllStringSubmatchIndex(src, -1) {
		at := parseAttrs(src[m[2]:m[3]])
		if html.UnescapeString(at["route"]) != route {
			continue
		}
		f := rePageFecha.FindStringIndex(src[m[1]:])
		if f == nil {
			return 0, 0, false
		}
		return m[1], m[1] + f[0], true
	}
	return 0, 0, false
}

// aplicarPaginasVisuais troca o miolo das páginas montadas no construtor e
// acrescenta as páginas novas (antes da 404, ou depois da última página).
func (s *Server) aplicarPaginasVisuais(src string) string {
	for _, r := range s.rotasVisuais() {
		p := s.paginaVisual(r)
		if p == nil {
			continue
		}
		if ini, fim, ok := miolo(src, r); ok {
			src = src[:ini] + "\n" + p.Fonte + "\n" + src[fim:]
			continue
		}
		if !p.Nova {
			continue // a página saiu do arquivo: a versão visual fica guardada, sem uso
		}
		nova := `<page route="` + html.EscapeString(r) + `" title="` + html.EscapeString(p.Titulo) + `">` + "\n" + p.Fonte + "\n</page>\n"
		if ini, _, ok := miolo(src, "404"); ok {
			if m := strings.LastIndex(strings.ToLower(src[:ini]), "<page"); m >= 0 {
				src = src[:m] + nova + "  " + src[m:]
				continue
			}
		}
		if f := rePageFecha.FindAllStringIndex(src, -1); len(f) > 0 {
			e := f[len(f)-1][1]
			src = src[:e] + "\n" + nova + src[e:]
		}
	}
	return src
}

// ---------- dividir a fonte em HTML e ilhas ----------

type noFonte struct {
	tag      string // "" = texto
	ini, fim int    // trecho inteiro na fonte
	abreFim  int    // fim da tag de abertura
	attrs    string
	filhos   []*noFonte
	fechado  bool
}

var (
	vazias = map[string]bool{"img": true, "br": true, "hr": true, "source": true, "input": true, "meta": true, "link": true, "area": true, "col": true, "wbr": true, "track": true, "embed": true, "param": true}
	cruas  = map[string]bool{"script": true, "style": true, "textarea": true, "nep": true, "pre": true, "code": true, "svg": true, "math": true}
	// atributos que a página visual guarda além dos do construtor de trechos
	reAttrExtra = regexp.MustCompile(`^(aria-[a-z-]+|data-[a-z0-9-]+|role|editavel|menu-editavel|nep-[a-z-]+|lang|dir|rel|name|tabindex|hidden|aria)$`)
	reURLFonte  = regexp.MustCompile(`^\{\{\s*url\(\s*(?:'([^'{}]*)'|"([^"{}]*)")\s*\)\s*\}\}$`)
	inlineCtx   = map[string]bool{"p": true, "a": true, "span": true, "h1": true, "h2": true, "h3": true, "h4": true, "h5": true, "h6": true, "li": true, "strong": true, "em": true, "b": true, "i": true, "small": true, "label": true, "button": true, "td": true, "th": true, "figcaption": true, "blockquote": true, "dt": true, "dd": true}
)

// arvore lê a fonte numa árvore tolerante (só para achar os elementos).
func arvore(src string) []*noFonte {
	raiz := &noFonte{tag: "#raiz"}
	pilha := []*noFonte{raiz}
	pos := 0
	topo := func() *noFonte { return pilha[len(pilha)-1] }
	texto := func(a, b int) {
		if b > a {
			topo().filhos = append(topo().filhos, &noFonte{ini: a, fim: b})
		}
	}
	for pos < len(src) {
		loc := tokRe.FindStringSubmatchIndex(src[pos:])
		if loc == nil {
			break
		}
		a, b := pos+loc[0], pos+loc[1]
		texto(pos, a)
		pos = b
		if loc[4] < 0 { // comentário: vira texto (fica como está)
			texto(a, b)
			continue
		}
		fecha := loc[3] > loc[2]
		tag := strings.ToLower(src[pos-loc[1]+loc[4] : pos-loc[1]+loc[5]])
		if fecha {
			for i := len(pilha) - 1; i > 0; i-- {
				if pilha[i].tag == tag {
					for j := len(pilha) - 1; j >= i; j-- {
						pilha[j].fim = a
					}
					pilha[i].fim, pilha[i].fechado = b, true
					pilha = pilha[:i]
					break
				}
			}
			continue
		}
		n := &noFonte{tag: tag, ini: a, abreFim: b, fim: b, attrs: src[pos-loc[1]+loc[6] : pos-loc[1]+loc[7]]}
		topo().filhos = append(topo().filhos, n)
		if vazias[tag] || strings.HasSuffix(strings.TrimSpace(n.attrs), "/") {
			n.fechado = true
			continue
		}
		if cruas[tag] { // até fechar, sem olhar dentro
			re := regexp.MustCompile(`(?i)</` + regexp.QuoteMeta(tag) + `\s*>`)
			if f := re.FindStringIndex(src[b:]); f != nil {
				n.fim, n.fechado = b+f[1], true
				pos = b + f[1]
			} else {
				n.fim, pos = len(src), len(src)
			}
			continue
		}
		pilha = append(pilha, n)
	}
	texto(pos, len(src))
	for _, n := range pilha[1:] {
		n.fim = len(src)
	}
	return raiz.filhos
}

// estatico: elemento que o construtor pode editar por dentro.
func (n *noFonte) estatico(src string) bool {
	if !visTags[n.tag] || cruas[n.tag] || n.tag == "iframe" || !n.fechado && !vazias[n.tag] {
		return false
	}
	for k, v := range parseAttrs(n.attrs) {
		if strings.HasPrefix(k, "on") || !visAttrs[k] && !reAttrExtra.MatchString(k) || k == "data-nep-ilha" || k == "data-nep-novo" {
			return false
		}
		if strings.Contains(v, "{{") && !reURLFonte.MatchString(strings.TrimSpace(v)) {
			return false
		}
	}
	return true
}

// Decompor divide o miolo da página: HTML editável (com marcadores
// <div data-nep-ilha="k"></div>) e as fontes das ilhas.
func (s *Server) Decompor(src string) (string, []string) {
	var b strings.Builder
	var ilhas []string
	var anda func(ns []*noFonte, pai string)
	ilha := func(n *noFonte, pai string) {
		t := "div"
		if inlineCtx[pai] {
			t = "span"
		}
		fmt.Fprintf(&b, `<%s data-nep-ilha="%d"></%s>`, t, len(ilhas), t)
		ilhas = append(ilhas, src[n.ini:n.fim])
	}
	anda = func(ns []*noFonte, pai string) {
		for _, n := range ns {
			if n.tag == "" {
				t := src[n.ini:n.fim]
				switch {
				case strings.HasPrefix(t, "<!--"):
					// comentários somem no construtor
				case strings.Contains(t, "{{") || strings.Contains(t, "{%"):
					// texto com expressões: as partes fixas seguem editáveis
					resto := t
					for {
						i := strings.Index(resto, "{{")
						if i < 0 {
							b.WriteString(resto)
							break
						}
						j := strings.Index(resto[i:], "}}")
						if j < 0 {
							b.WriteString(resto)
							break
						}
						b.WriteString(resto[:i])
						fmt.Fprintf(&b, `<span data-nep-ilha="%d"></span>`, len(ilhas))
						ilhas = append(ilhas, resto[i:i+j+2])
						resto = resto[i+j+2:]
					}
				default:
					b.WriteString(t)
				}
				continue
			}
			if !n.estatico(src) {
				ilha(n, pai)
				continue
			}
			b.WriteString("<" + n.tag)
			at := parseAttrs(n.attrs)
			for _, k := range ordem(at) {
				v := at[k]
				if m := reURLFonte.FindStringSubmatch(strings.TrimSpace(v)); m != nil {
					v = s.url(m[1] + m[2])
				}
				b.WriteString(" " + k + `="` + html.EscapeString(html.UnescapeString(v)) + `"`)
			}
			b.WriteString(">")
			if vazias[n.tag] {
				continue
			}
			anda(n.filhos, n.tag)
			b.WriteString("</" + n.tag + ">")
		}
	}
	anda(arvore(src), "")
	return strings.TrimSpace(b.String()), ilhas
}

func ordem(m map[string]string) []string {
	ks := make([]string, 0, len(m))
	for k := range m {
		ks = append(ks, k)
	}
	sort.Strings(ks)
	return ks
}

// ---------- voltar do construtor para a fonte ----------

var reMarcaIlha = regexp.MustCompile(`(?i)<(div|span)\b[^>]*\bdata-nep-(ilha|novo)="([a-z0-9-]+)"[^>]*>`)

// Recompor junta o HTML do construtor e as ilhas de volta numa fonte de
// página. O HTML passa pela mesma limpeza do construtor (sem scripts nem
// eventos), as chaves viram &#123; (nada de código escondido no texto) e os
// links do próprio site voltam a ser {{ url('...') }}.
func (s *Server) Recompor(htm, css string, ilhas []string) (string, error) {
	var b strings.Builder
	if c := cssDaPagina(css); c != "" {
		b.WriteString("<style>" + c + "</style>\n")
	}
	pos := 0
	for {
		m := reMarcaIlha.FindStringSubmatchIndex(htm[pos:])
		if m == nil {
			b.WriteString(s.limparPagina(htm[pos:]))
			break
		}
		a := pos + m[0]
		b.WriteString(s.limparPagina(htm[pos:a]))
		tag := strings.ToLower(htm[pos+m[2] : pos+m[3]])
		tipo, id := htm[pos+m[4]:pos+m[5]], htm[pos+m[6]:pos+m[7]]
		// pula até fechar a marca (o construtor manda vazia, mas por garantia)
		fim := pos + m[1]
		prof := 1
		reT := regexp.MustCompile(`(?i)<(/?)` + tag + `\b[^>]*>`)
		for prof > 0 {
			t := reT.FindStringSubmatchIndex(htm[fim:])
			if t == nil {
				fim = len(htm)
				break
			}
			if t[3] > t[2] {
				prof--
			} else {
				prof++
			}
			fim += t[1]
		}
		pos = fim
		if tipo == "ilha" {
			k, err := strconv.Atoi(id)
			if err != nil || k < 0 || k >= len(ilhas) {
				return "", fmt.Errorf("bloco do Nepenthes %q não existe nesta página", id)
			}
			b.WriteString(ilhas[k])
		} else if blk, ok := blocosNep[id]; ok {
			b.WriteString(blk.Fonte)
		}
	}
	return strings.TrimSpace(b.String()), nil
}

// limparPagina: cleanVisual com os atributos a mais da página e as chaves
// escapadas; links internos voltam a usar url().
func (s *Server) limparPagina(h string) string {
	if strings.TrimSpace(h) == "" {
		return h
	}
	out := limparComExtras(h)
	out = strings.ReplaceAll(out, "{", "&#123;")
	base := s.opt.Base
	return reHrefInterno.ReplaceAllStringFunc(out, func(m string) string {
		sm := reHrefInterno.FindStringSubmatch(m)
		v := html.UnescapeString(sm[2])
		if base != "" {
			if !strings.HasPrefix(v, base+"/") && v != base {
				return m
			}
			v = strings.TrimPrefix(v, base)
			if v == "" {
				v = "/"
			}
		}
		v = strings.ReplaceAll(strings.ReplaceAll(v, `\`, `\\`), `'`, `\'`)
		return ` ` + sm[1] + `="{{ url('` + html.EscapeString(v) + `') }}"`
	})
}

var reHrefInterno = regexp.MustCompile(` (href|src|poster)="(/[^/"][^"]*|/)"`)

// limparComExtras roda a limpeza com os atributos que a página visual aceita.
func limparComExtras(h string) string {
	return cleanVisualCom(h, func(k string) bool {
		return reAttrExtra.MatchString(k) && k != "data-nep-ilha" && k != "data-nep-novo" && k != "data-nep-previa"
	})
}

// cssDaPagina: o CSS do construtor sem regras para body/html/* e sem
// endereços perigosos (a página não muda o resto do site).
func cssDaPagina(css string) string {
	return strings.ReplaceAll(scopeCSSCom(css, ""), "</", "<\\/")
}

// ---------- blocos prontos do Nepenthes ----------

type blocoNep struct {
	ID, Nome, Fonte string
}

var blocosNep = map[string]blocoNep{
	"contato": {"contato", "Formulário de contato", `<section style="padding:56px 24px"><div style="max-width:640px;margin:0 auto"><h2>Fale com a gente</h2>
<form save="contatos" avisar="dono" success="Recebemos sua mensagem. Respondemos em breve!" class="nep-form">
  <label>Nome <input name="nome" required maxlength="80"></label>
  <label>E-mail <input name="email" type="email" required></label>
  <label>Mensagem <textarea name="mensagem" required maxlength="2000"></textarea></label>
  <button>Enviar</button>
</form>
</div></section>`},
	"novidades": {"novidades", "Receber novidades por e-mail", `<section style="padding:48px 24px;text-align:center"><div style="max-width:520px;margin:0 auto"><h2>Novidades por e-mail</h2><p>Uma mensagem por mês, sem spam.</p>
<form save="assinantes" limit="3" success="Pronto! Você vai receber as novidades." class="nep-form">
  <label>Seu e-mail <input name="email" type="email" required></label>
  <button>Quero receber</button>
</form>
</div></section>`},
}

// ---------- prévia das ilhas ----------

type gravador struct {
	h      http.Header
	b      strings.Builder
	status int
}

func (g *gravador) Header() http.Header         { return g.h }
func (g *gravador) Write(p []byte) (int, error) { return g.b.Write(p) }
func (g *gravador) WriteHeader(c int)           { g.status = c }

var reMarcaPrevia = regexp.MustCompile(`<(div|span) data-nep-previa="([a-z0-9-]+)" style="display:contents">`)

// previas renderiza a página com cada ilha embrulhada e recorta o HTML de
// cada uma. Devolve também os estilos do site (para o construtor parecer o site).
func (s *Server) previas(rq *request, src, route string, ilhas []string) (map[string]string, []map[string]string, string, error) {
	ini, fim, ok := miolo(src, route)
	if !ok {
		return nil, nil, "", errors.New("página não encontrada")
	}
	var b strings.Builder
	for k, il := range ilhas {
		fmt.Fprintf(&b, `<div data-nep-previa="%d" style="display:contents">%s</div>`, k, il)
	}
	for id, bl := range blocosNep {
		fmt.Fprintf(&b, `<div data-nep-previa="%s" style="display:contents">%s</div>`, id, bl.Fonte)
	}
	site, err := ParseSite("previa", src[:ini]+b.String()+src[fim:])
	if err != nil {
		return nil, nil, "", err
	}
	var pg *Page
	for _, p := range site.Pages {
		if p.Route == route {
			pg = p
		}
	}
	if pg == nil {
		return nil, nil, "", errors.New("página não encontrada")
	}
	r2, _ := http.NewRequest("GET", s.url(route), nil)
	r2.Header = rq.r.Header.Clone()
	g := &gravador{h: http.Header{}}
	sub := &request{s: s, site: site, w: g, r: r2, path: route, sess: rq.sess, https: rq.https, page: pg}
	if pg.Route == "404" {
		sub.path = "/_nep/404"
	}
	if err := s.renderPage(sub, 200, nil); err != nil {
		return nil, nil, "", err
	}
	out := g.b.String()
	prev := map[string]string{}
	for _, m := range reMarcaPrevia.FindAllStringSubmatchIndex(out, -1) {
		tag, id := out[m[2]:m[3]], out[m[4]:m[5]]
		reT := regexp.MustCompile(`<(/?)` + tag + `\b[^>]*>`)
		pos, prof := m[1], 1
		for prof > 0 {
			t := reT.FindStringSubmatchIndex(out[pos:])
			if t == nil {
				pos = len(out)
				break
			}
			if t[3] > t[2] {
				prof--
			} else {
				prof++
			}
			pos += t[1]
		}
		fecho := strings.LastIndex(out[:pos], "<")
		if fecho < m[1] {
			fecho = m[1]
		}
		if _, ja := prev[id]; !ja {
			prev[id] = out[m[1]:fecho]
		}
	}
	// os estilos do site, na ordem da página: {"href": …} ou {"css": …}
	var estilos []map[string]string
	cabeca := out
	if i := strings.Index(strings.ToLower(out), "</head>"); i >= 0 {
		cabeca = out[:i]
	}
	for _, m := range regexp.MustCompile(`(?is)<link\b[^>]*rel=["']?stylesheet[^>]*>|<style[^>]*>(.*?)</style>`).FindAllStringSubmatch(cabeca, -1) {
		if strings.HasPrefix(strings.ToLower(m[0]), "<link") {
			if h := parseAttrs(m[0][5 : len(m[0])-1])["href"]; h != "" {
				estilos = append(estilos, map[string]string{"href": html.UnescapeString(h)})
			}
		} else {
			estilos = append(estilos, map[string]string{"css": m[1]})
		}
	}
	return prev, estilos, "", nil
}

// ---------- telas do painel ----------

// fonteDoSite: o texto do site como o servidor vê (moldes expandidos e
// páginas visuais aplicadas).
func (s *Server) fonteDoSite() (string, error) {
	if _, err := s.load(); err != nil {
		return "", err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.fonte, nil
}

func rotaEditavel(r string) bool {
	return r == "404" || strings.HasPrefix(r, "/") && !strings.ContainsAny(r, ":*")
}

func (a *adminPage) paginasPage(post bool) error {
	s := a.s
	r := a.rq.r
	msg := ""
	if post {
		switch r.PostForm.Get("acao") {
		case "original":
			rt := r.PostForm.Get("r")
			if err := s.Store.SetMeta(paginaMeta+rt, ""); err != nil {
				return err
			}
			if err := s.lembrarRota(rt, false); err != nil {
				return err
			}
			s.recarregar()
			msg = "A página " + rt + " voltou ao original."
		case "nova":
			rt := "/" + slugify(r.PostForm.Get("endereco"))
			tit := strings.TrimSpace(r.PostForm.Get("titulo"))
			site, err := s.load()
			switch {
			case err != nil:
				return err
			case rt == "/" || tit == "":
				msg = "Dê um título e um endereço para a página."
			case site.hasRoute(rt):
				msg = "Já existe uma página em " + rt + "."
			default:
				p := paginaVisual{Nova: true, Titulo: tit, Em: time.Now().Format(time.RFC3339), Por: a.who(),
					Fonte: `<section class="nep-secao"><h1>` + html.EscapeString(tit) + `</h1><p>Escreva aqui. Arraste blocos da direita para montar a página.</p></section>`}
				bs, _ := json.Marshal(p)
				if err := s.Store.SetMeta(paginaMeta+rt, string(bs)); err != nil {
					return err
				}
				if err := s.lembrarRota(rt, true); err != nil {
					return err
				}
				s.recarregar()
				s.redirect(a.rq, a.base+"/paginas/editar?r="+url.QueryEscape(rt), http.StatusSeeOther)
				return nil
			}
		}
	}
	site, err := s.load()
	if err != nil {
		return err
	}
	var b strings.Builder
	b.WriteString(errP(msg))
	b.WriteString(`<p class="muted">Monte qualquer página arrastando blocos, como num construtor de sites. O que é código do Nepenthes (listas, formulários, regiões ao vivo) aparece como bloco fechado e continua funcionando. O arquivo do site não muda: “Voltar ao original” desfaz tudo.</p>`)
	b.WriteString(`<table><tr><th>Página</th><th>Endereço</th><th></th></tr>`)
	for _, p := range site.Pages {
		if !rotaEditavel(p.Route) {
			continue
		}
		tit := p.Title
		if tit == "" {
			tit = p.Route
		}
		marca := ""
		if s.paginaVisual(p.Route) != nil {
			marca = ` <small class="muted">· montada no construtor</small>`
		}
		b.WriteString(`<tr><td>` + esc(tit) + marca + `</td><td><code>` + esc(p.Route) + `</code></td><td><div class="row" style="gap:8px;justify-content:flex-end">`)
		b.WriteString(`<a class="btn" href="` + a.base + `/paginas/editar?r=` + url.QueryEscape(p.Route) + `">Editar arrastando</a>`)
		if s.paginaVisual(p.Route) != nil {
			b.WriteString(`<form method="post">` + a.tok() + `<input type="hidden" name="acao" value="original"><input type="hidden" name="r" value="` + esc(p.Route) + `"><button class="ghost">Voltar ao original</button></form>`)
		}
		b.WriteString(`</div></td></tr>`)
	}
	b.WriteString(`</table><h2>Página nova</h2><form method="post" class="row" style="gap:10px;align-items:flex-end;flex-wrap:wrap">` + a.tok() +
		`<input type="hidden" name="acao" value="nova"><label>Título<input name="titulo" maxlength="80" required></label><label>Endereço<input name="endereco" maxlength="60" placeholder="quem-somos" required></label><button>Criar e montar</button></form>`)
	return a.out("Montar páginas", b.String(), true)
}

// paginaEditar: a tela cheia do construtor para uma página.
func (a *adminPage) paginaEditar(post bool) error {
	s := a.s
	r := a.rq.r
	rt := r.URL.Query().Get("r")
	if post {
		rt = r.PostForm.Get("r")
	}
	site, err := s.load()
	if err != nil {
		return err
	}
	if !rotaEditavel(rt) || !site.hasRoute(rt) && rt != "404" {
		a.s.simple(a.rq, 404, "Página não encontrada", "<p>Essa página não existe ou não pode ser montada no construtor.</p>")
		return nil
	}
	src, err := s.fonteDoSite()
	if err != nil {
		return err
	}
	ini, fim, ok := miolo(src, rt)
	if !ok {
		a.s.simple(a.rq, 404, "Página não encontrada", "")
		return nil
	}
	htm, ilhas := s.Decompor(src[ini:fim])
	if post {
		a.rq.w.Header().Set("Content-Type", "application/json; charset=utf-8")
		falha := func(e error) error {
			a.rq.w.WriteHeader(http.StatusUnprocessableEntity)
			return json.NewEncoder(a.rq.w).Encode(map[string]string{"erro": e.Error()})
		}
		fonte, err := s.Recompor(r.PostForm.Get("html"), r.PostForm.Get("css"), ilhas)
		if err != nil {
			return falha(err)
		}
		if _, err := ParseSite("check", src[:ini]+"\n"+fonte+"\n"+src[fim:]); err != nil {
			return falha(fmt.Errorf("a página ficou com erro e não foi salva: %v", err))
		}
		p := paginaVisual{Fonte: fonte, Em: time.Now().Format(time.RFC3339), Por: a.who()}
		if velho := s.paginaVisual(rt); velho != nil && velho.Nova {
			p.Nova, p.Titulo = true, velho.Titulo
		}
		bs, _ := json.Marshal(p)
		if err := s.Store.SetMeta(paginaMeta+rt, string(bs)); err != nil {
			return err
		}
		if err := s.lembrarRota(rt, true); err != nil {
			return err
		}
		s.recarregar()
		return json.NewEncoder(a.rq.w).Encode(map[string]any{"ok": true, "ver": s.url(map[bool]string{true: "/", false: rt}[rt == "404"])})
	}
	prev, hrefs, css, err := s.previas(a.rq, src, rt, ilhas)
	if err != nil {
		fmt.Fprintf(s.opt.Log, "prévia de %s: %v\n", rt, err)
		prev = map[string]string{}
	}
	var blocos []map[string]string
	for _, id := range []string{"contato", "novidades"} {
		blocos = append(blocos, map[string]string{"id": id, "nome": blocosNep[id].Nome})
	}
	dados, _ := json.Marshal(map[string]any{
		"html": htm, "previas": prev, "estilos": hrefs, "css": css, "blocos": blocos,
		"rota": rt, "base": s.opt.Base, "csrf": a.rq.sess.CSRF, "salvar": a.base + "/paginas/editar", "voltar": a.base + "/paginas",
		"ver": s.url(map[bool]string{true: "/", false: rt}[rt == "404"]), "upload": a.base + "/construtor/arquivo",
		"midia": json.RawMessage(mediaJSON(s.mediaURLs())),
	})
	p := esc(s.opt.Base) + "/_nep/construtor/"
	w := a.rq.w
	a.rq.saveSession()
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	fmt.Fprintf(w, `<!doctype html><html lang="pt-BR"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1">
<title>Montar %s · Painel</title>
<link rel="stylesheet" href="%sgrapes.min.css"><link rel="stylesheet" href="%spagina.css">
</head><body>
<header class="pv-topo"><a href="%s" class="pv-volta">← Páginas</a><strong>%s</strong><span id="pv-estado"></span>
<a class="pv-btn" id="pv-ver" href="%s" target="_blank" rel="noopener">Ver no site</a><button class="pv-btn pv-pri" id="pv-salvar" type="button">Salvar</button></header>
<div id="pv-editor"></div>
<script type="application/json" id="pv-dados">%s</script>
<script src="%sgrapes.min.js"></script><script src="%sblocks-basic.js"></script><script src="%spt.js"></script><script src="%spagina.js"></script>
</body></html>`, esc(rt), p, p, esc(a.base+"/paginas"), esc(rt), esc(s.url(rt)), strings.ReplaceAll(string(dados), "</", `<\/`), p, p, p, p)
	return nil
}
