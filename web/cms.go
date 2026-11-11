package web

// O lado "CMS" do Nepenthes: o programador escreve o tema (o .nep.html) e
// quem cuida do site troca textos, imagens e publica conteúdo pelo painel,
// sem mexer no código e sem construtor de arrastar e soltar.
//
//   - <conteudo nome="artigos"> declara um tipo de conteúdo com seus campos;
//     o painel ganha criar, editar, rascunho/publicado e endereço (slug).
//   - <page route="/blog/:slug" conteudo="artigos"> mostra um item: a variável
//     `item` chega pronta, com 404, título, descrição, imagem e sitemap sozinhos.
//   - editavel="chave" em qualquer tag deixa o texto (ou a imagem) editável
//     no painel e direto na página, para quem está logado.

import (
	"fmt"
	"html"
	"regexp"
	"strconv"
	"strings"
	"unicode"

	"github.com/Nanofylium/probable-plant/lang"
)

// ---------- textos editáveis ----------

const edPrefix = "ed:"

// edGet devolve o texto trocado no painel (ok=false: vale o padrão do HTML).
func (s *Server) edGet(key string) (string, bool) {
	v := s.Store.Meta(edPrefix + key)
	if v == "" {
		return "", false
	}
	return v[1:], true // guardado como "=" + valor, para o texto vazio também valer
}

func (s *Server) edSet(key, val string) error { return s.Store.SetMeta(edPrefix+key, "="+val) }

func (s *Server) edReset(key string) error { return s.Store.SetMeta(edPrefix+key, "") }

// edHTML mostra um texto editável: texto simples com quebras de linha, ou markdown nos blocos.
func edHTML(kind, v string) lang.HTML {
	if kind == "rico" {
		return lang.HTML(Markdown(v))
	}
	return lang.HTML(strings.ReplaceAll(html.EscapeString(v), "\n", "<br>\n"))
}

// ---------- endereços (slug) ----------

var slugDash = regexp.MustCompile(`-+`)

var accents = func() map[rune]rune {
	m := map[rune]rune{}
	for to, from := range map[rune]string{'a': "áàâãäå", 'e': "éèêë", 'i': "íìîï", 'o': "óòôõö", 'u': "úùûü", 'c': "ç", 'n': "ñ", 'y': "ýÿ"} {
		for _, r := range from {
			m[r] = to
		}
	}
	return m
}()

// slugify: "Pão de Queijo, a receita!" -> "pao-de-queijo-a-receita".
func slugify(s string) string {
	var b strings.Builder
	for _, r := range strings.ToLower(s) {
		if a, ok := accents[r]; ok {
			r = a
		}
		switch {
		case r < 128 && (unicode.IsLetter(r) || unicode.IsDigit(r)):
			b.WriteRune(r)
		default:
			b.WriteByte('-')
		}
	}
	out := strings.Trim(slugDash.ReplaceAllString(b.String(), "-"), "-")
	if len(out) > 80 {
		out = strings.TrimRight(out[:80], "-")
	}
	return out
}

var slugRe = regexp.MustCompile(`^[a-z0-9]+(-[a-z0-9]+)*$`)

// uniqueSlug evita dois itens com o mesmo endereço.
func (s *Server) uniqueSlug(col, want string, self int64) string {
	if want == "" {
		want = "item"
	}
	used := map[string]bool{}
	for _, r := range s.Store.All(col, 0) {
		if r.ID == self {
			continue
		}
		if v, _ := r.ToValue().Get("slug"); v != nil {
			used[lang.Str(v)] = true
		}
	}
	slug := want
	for i := 2; used[slug]; i++ {
		slug = want + "-" + strconv.Itoa(i)
	}
	return slug
}

// ---------- rascunhos ----------

// published diz se o registro aparece no site (só publicado=false esconde).
func published(t *lang.Table) bool {
	v, ok := t.Get("publicado")
	if ok && v == false {
		return false
	}
	_, future := scheduledFuture(t) // publicar_em no futuro: ainda não aparece
	return !future
}

// findItem acha um item pelo endereço (slug) ou pelo número.
func (s *Server) findItem(col string, key lang.Value) *lang.Table {
	if id, ok := toInt(key); ok {
		if r := s.Store.Find(col, id); r != nil {
			return r.ToValue()
		}
		if _, isStr := key.(string); !isStr {
			return nil
		}
	}
	k, _ := key.(string)
	if k == "" {
		return nil
	}
	for _, r := range s.Store.All(col, 0) {
		t := r.ToValue()
		if v, _ := t.Get("slug"); v != nil && lang.Str(v) == k {
			return t
		}
	}
	return nil
}

// ---------- página de um item ----------

// itemTitle escolhe o título do item: campo titulo/nome/title, ou o primeiro campo de texto.
func itemTitle(c *Content, t *lang.Table) string {
	for _, k := range []string{"titulo", "título", "nome", "title", "name"} {
		if v, ok := t.Get(k); ok && v != nil && lang.Str(v) != "" {
			return lang.Str(v)
		}
	}
	for _, f := range c.Fields {
		if f.Type != "file" && f.Type != "textarea" && f.Type != "checkbox" {
			if v, ok := t.Get(f.Name); ok && v != nil {
				return lang.Str(v)
			}
		}
	}
	return ""
}

func itemDescription(c *Content, t *lang.Table) string {
	pick := ""
	for _, k := range []string{"resumo", "descricao", "descrição", "description", "chamada"} {
		if v, ok := t.Get(k); ok && v != nil && lang.Str(v) != "" {
			pick = lang.Str(v)
			break
		}
	}
	if pick == "" {
		for _, f := range c.Fields {
			if f.Type == "textarea" {
				if v, ok := t.Get(f.Name); ok && v != nil && strings.TrimSpace(lang.Str(v)) != "" {
					pick = tagRe.ReplaceAllString(brRe.ReplaceAllString(Markdown(lang.Str(v)), " "), "")
					break
				}
			}
		}
	}
	pick = strings.Join(strings.Fields(html.UnescapeString(pick)), " ")
	if r := []rune(pick); len(r) > 160 {
		pick = strings.TrimSpace(string(r[:157])) + "…"
	}
	return pick
}

func itemImage(c *Content, t *lang.Table) string {
	for _, f := range c.Fields {
		if f.Type == "file" && f.Public {
			if ft, ok := t.Get(f.Name); ok {
				if ft, ok := ft.(*lang.Table); ok {
					if tp, _ := ft.Get("tipo"); strings.HasPrefix(lang.Str(tp), "image/") {
						u, _ := ft.Get("url")
						return lang.Str(u)
					}
				}
			}
		}
	}
	return ""
}

// itemURL monta o endereço do item no site (a primeira página com conteudo= do tipo).
func (site *Site) itemURL(col string, t *lang.Table) string {
	for _, p := range site.Pages {
		if p.Conteudo != col || len(p.params) == 0 {
			continue
		}
		slug, _ := t.Get("slug")
		if slug == nil || lang.Str(slug) == "" {
			slug, _ = t.Get("id")
		}
		return paramRe.ReplaceAllStringFunc(p.Route, func(string) string { return lang.Str(slug) })
	}
	return ""
}

var tagRe = regexp.MustCompile(`<[^>]*>`)

var paramRe = regexp.MustCompile(`:[A-Za-z_][A-Za-z0-9_]*|\*`)

// ---------- menus editáveis (menu-editavel="principal") ----------

type menuItem struct{ Text, URL string }

var (
	anchorRe  = regexp.MustCompile(`(?is)<a\b((?:[^>"']|"[^"]*"|'[^']*')*)>(.*?)</a>`)
	urlCallRe = regexp.MustCompile(`^\{\{\s*url\(\s*['"]([^'"]*)['"]\s*\)\s*\}\}$`)
	hrefRe    = regexp.MustCompile(`(?i)\s*\bhref\s*=\s*("[^"]*"|'[^']*'|[^\s>]+)`)
	dynBlocks = []*regexp.Regexp{regexp.MustCompile(`(?is)<each\b.*?</each>`), regexp.MustCompile(`(?is)<cada\b.*?</cada>`), regexp.MustCompile(`(?is)<if\b.*?</if>`), regexp.MustCompile(`(?is)<se\b.*?</se>`)}
	ariaCurRe = regexp.MustCompile(`(?i)\s*\baria-current\s*=\s*("[^"]*"|'[^']*'|[^\s>]+)`)
)

// menuDefaults lê os links fixos escritos pelo programador dentro do menu e
// os atributos do primeiro link (classes etc.), usados em todos os itens.
func menuDefaults(e *Editable) (items []menuItem, attrs string) {
	src := e.Default
	for _, re := range dynBlocks { // links gerados por <each>/<if> não são itens fixos
		src = re.ReplaceAllString(src, "")
	}
	for i, m := range anchorRe.FindAllStringSubmatch(src, -1) {
		if i == 0 {
			attrs = ariaCurRe.ReplaceAllString(hrefRe.ReplaceAllString(m[1], ""), "")
			if strings.Contains(attrs, "{{") {
				attrs = ""
			}
		}
		text := strings.TrimSpace(htmlUnescape(tagRe.ReplaceAllString(m[2], "")))
		href := ""
		if h := hrefRe.FindStringSubmatch(m[1]); h != nil {
			href = strings.Trim(h[1], `"'`)
		}
		if strings.Contains(href, "{{") {
			u := urlCallRe.FindStringSubmatch(strings.TrimSpace(href))
			if u == nil {
				continue
			}
			href = u[1]
		}
		if text == "" || strings.Contains(text, "{{") || href == "" {
			continue
		}
		items = append(items, menuItem{text, htmlUnescape(href)})
	}
	return
}

// parseMenu lê o menu escrito no painel: uma linha por item, "Texto | /endereco".
func parseMenu(v string) ([]menuItem, string) {
	var out []menuItem
	for n, l := range strings.Split(v, "\n") {
		l = strings.TrimSpace(l)
		if l == "" {
			continue
		}
		i := strings.LastIndex(l, "|")
		if i < 0 {
			return nil, fmt.Sprintf("Linha %d do menu: use \"Texto | /endereço\".", n+1)
		}
		t, u := strings.TrimSpace(l[:i]), strings.TrimSpace(l[i+1:])
		if t == "" || !safeURL(u) {
			return nil, fmt.Sprintf("Linha %d do menu: o endereço precisa começar com / ou https://.", n+1)
		}
		out = append(out, menuItem{t, u})
	}
	if len(out) > 40 {
		return nil, "O menu pode ter no máximo 40 itens."
	}
	return out, ""
}

func formatMenu(items []menuItem) string {
	var l []string
	for _, it := range items {
		l = append(l, it.Text+" | "+it.URL)
	}
	return strings.Join(l, "\n")
}

// menuItems devolve os itens atuais do menu (trocados no painel ou os do HTML).
func (s *Server) menuItems(e *Editable) []menuItem {
	if v, ok := s.edGet(e.Key); ok {
		items, _ := parseMenu(v)
		return items
	}
	items, _ := menuDefaults(e)
	return items
}

func (s *Server) menuHref(u string) string {
	if strings.HasPrefix(u, "/") && !strings.HasPrefix(u, "//") {
		return s.url(u)
	}
	return u
}

func samePath(u, path string) bool {
	if !strings.HasPrefix(u, "/") {
		return false
	}
	if i := strings.IndexAny(u, "?#"); i >= 0 {
		u = u[:i]
	}
	return "/"+strings.Trim(u, "/") == path
}

// menuHTML monta os links do menu com os atributos do primeiro link do programador.
func (s *Server) menuHTML(e *Editable, path string) lang.HTML {
	_, attrs := menuDefaults(e)
	var b strings.Builder
	for i, it := range s.menuItems(e) {
		if i > 0 {
			b.WriteString("\n")
		}
		b.WriteString("<a" + attrs + ` href="` + html.EscapeString(s.menuHref(it.URL)) + `"`)
		if samePath(it.URL, path) {
			b.WriteString(` aria-current="page"`)
		}
		b.WriteString(">" + html.EscapeString(it.Text) + "</a>")
	}
	return lang.HTML(b.String())
}
