package web

// Leitura do arquivo .nep.html e tradução para código Nepenthes.
//
// O HTML vira dois trechos de código Nepenthes que rodam na mesma VM:
//   1. "prévio": os blocos <nep> fora de <each>/<if>, na ordem do arquivo
//      (primeiro os globais, depois os da página pedida);
//   2. "página": o HTML em si, como chamadas __w (texto fixo) e __e (valor escapado),
//      com <each> virando for e <if> virando if.
// Cada pedaço de código gerado fica na MESMA linha do arquivo original, então
// mensagens de erro apontam a linha certa do .nep.html.

import (
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"
)

type nodeKind int

const (
	kText nodeKind = iota
	kExpr
	kCode
	kPage
	kEach
	kIf
	kForm
	kLive
	kAPI
	kRoot
)

type node struct {
	kind     nodeKind
	line     int
	text     string // kText: índice em Site.texts; kExpr/kCode: código
	textIdx  int
	children []*node
	els      []*node
	inElse   bool
	// each
	v1, v2, expr string
	id           int // form, live, api, page
	attrs        []*node
	prerun       bool
	capIdx       int
}

func (n *node) add(c *node) {
	if n.inElse {
		n.els = append(n.els, c)
	} else {
		n.children = append(n.children, c)
	}
}

// Page é uma <page route="...">.
type Page struct {
	ID    int
	Route string
	Title string
	Auth  bool
	Line  int
	// Description e Image viram as etiquetas de busca e de prévia em redes sociais.
	Description string
	Image       string
	NoIndex     bool
	re          *regexp.Regexp
	params      []string
	node        *node
}

// Form é um <form save="..."> ou <form run="...">.
type Form struct {
	ID       int
	Save     string
	Run      string
	Success  string
	Redirect string
	Notify   string
	Limit    int
	Line     int
	Fields   []*Field
	HasFile  bool // tem <input type="file">: o envio é multipart
	start    int
}

type Field struct {
	Name     string
	Type     string
	Label    string
	Required bool
	Multi    bool
	MaxLen   int
	Min, Max *float64
	Options  []string // nil: qualquer valor
	// campos de arquivo
	Accept string
	MaxMB  float64
	Public bool
}

// API é um <api route="/api/x" method="GET"> com código que devolve JSON.
type API struct {
	ID     int
	Route  string
	Method string
	Line   int
	code   string
	re     *regexp.Regexp
	params []string
}

type Live struct {
	ID    int
	Every int // segundos; 0 = só quando os dados mudam
}

type dataBlock struct {
	Name string
	JSON string
	Line int
}

// Site é um arquivo .nep.html já lido.
type Site struct {
	File   string
	Src    string
	Config map[string]string
	Data   []dataBlock
	Pages  []*Page
	Forms  []*Form
	APIs   []*API
	Lives  []*Live
	texts  []string
	root   *node
	caps   int
	cache  map[int][2]string
}

// TemplateError é um erro de estrutura no .nep.html.
type TemplateError struct {
	File string
	Line int
	Msg  string
}

func (e *TemplateError) Error() string { return fmt.Sprintf("%s:%d: %s", e.File, e.Line, e.Msg) }

var tagAlias = map[string]string{"pagina": "page", "cada": "each", "se": "if", "senao": "else", "vivo": "live"}

// ParseSite lê o .nep.html.
func ParseSite(file, src string) (*Site, error) {
	s := &Site{File: file, Src: src, Config: map[string]string{}, cache: map[int][2]string{}}
	s.root = &node{kind: kRoot, line: 1}
	stack := []*node{s.root}
	var page *node
	fail := func(line int, f string, a ...any) error {
		return &TemplateError{File: file, Line: line, Msg: fmt.Sprintf(f, a...)}
	}
	lineAt := func(off int) int { return 1 + strings.Count(src[:off], "\n") }

	pos, textStart := 0, 0
	flush := func(end int) error {
		if end > textStart {
			top := stack[len(stack)-1]
			chunk := src[textStart:end]
			if strings.TrimSpace(chunk) == "" && strings.Count(chunk, "\n") > 1 {
				chunk = "\n" // linhas que só tinham <nep>, <data>… não viram linhas em branco
			}
			ns, err := s.textNodes(chunk, lineAt(textStart))
			if err != nil {
				return err
			}
			for _, n := range ns {
				top.add(n)
			}
		}
		return nil
	}
	for {
		i := strings.IndexByte(src[pos:], '<')
		if i < 0 {
			break
		}
		at := pos + i
		rest := src[at:]
		low := strings.ToLower(rest[:min(len(rest), 16)])
		ln := lineAt(at)
		switch {
		case strings.HasPrefix(low, "<!--"):
			e := strings.Index(rest, "-->")
			if e < 0 {
				return nil, fail(ln, "comentário <!-- sem -->")
			}
			// comentários HTML ficam como estão (sem {{ }}) e não escondem tags
			if err := flush(at); err != nil {
				return nil, err
			}
			stack[len(stack)-1].add(s.lit(rest[:e+3], ln))
			pos = at + e + 3
			textStart = pos
			continue
		}

		name, attrs, tagEnd, closing, ok := scanTag(rest)
		if !ok {
			pos = at + 1
			continue
		}
		lname := strings.ToLower(name)
		if a, ok := tagAlias[lname]; ok {
			lname = a
		}
		top := stack[len(stack)-1]
		end := at + tagEnd

		if closing {
			switch lname {
			case "page", "each", "if", "live":
				if err := flush(at); err != nil {
					return nil, err
				}
				want := map[string]nodeKind{"page": kPage, "each": kEach, "if": kIf, "live": kLive}[lname]
				if top.kind != want {
					open := "nenhuma tag aberta"
					if top.kind != kRoot {
						open = fmt.Sprintf("<%s> aberta na linha %d", kindName(top.kind), top.line)
					}
					return nil, fail(ln, "fechamento </%s> não combina (%s)", lname, open)
				}
				stack = stack[:len(stack)-1]
				if lname == "page" {
					page = nil
				}
				pos, textStart = end, end
				continue
			case "form":
				if top.kind == kForm {
					if err := flush(at); err != nil {
						return nil, err
					}
					f := s.Forms[top.id]
					f.Fields = extractFields(src[f.start:at])
					for _, fd := range f.Fields {
						if fd.Type == "file" {
							f.HasFile = true
						}
					}
					stack = stack[:len(stack)-1]
					pos, textStart = end, end
					continue
				}
			}
			pos = at + 1
			continue
		}

		switch lname {
		case "nep", "api":
			closeTag := "</" + strings.ToLower(name) + ">"
			e := strings.Index(strings.ToLower(src[end:]), closeTag)
			if e < 0 {
				return nil, fail(ln, "<%s> sem %s", name, closeTag)
			}
			if err := flush(at); err != nil {
				return nil, err
			}
			code := src[end : end+e]
			if lname == "nep" {
				n := &node{kind: kCode, line: ln, text: code}
				top.add(n)
			} else {
				if top.kind != kRoot {
					return nil, fail(ln, "<api> precisa ficar fora das outras tags")
				}
				a := parseAttrs(attrs)
				route := first(a, "route", "rota")
				if route == "" {
					return nil, fail(ln, `<api> precisa de route="/api/caminho"`)
				}
				re, params := routeRegex(route)
				method := strings.ToUpper(first(a, "method", "metodo"))
				if method == "" {
					method = "GET"
				}
				s.APIs = append(s.APIs, &API{ID: len(s.APIs), Route: route, Method: method, Line: ln, code: code, re: re, params: params})
			}
			pos = end + e + len(closeTag)
			textStart = pos
			continue
		case "data":
			e := strings.Index(strings.ToLower(src[end:]), "</data>")
			if e < 0 {
				return nil, fail(ln, "<data> sem </data>")
			}
			a := parseAttrs(attrs)
			nm := first(a, "name", "nome")
			if !isIdent(nm) {
				return nil, fail(ln, `<data> precisa de name="nome_valido"`)
			}
			if err := flush(at); err != nil {
				return nil, err
			}
			s.Data = append(s.Data, dataBlock{Name: nm, JSON: src[end : end+e], Line: lineAt(end)})
			pos = end + e + len("</data>")
			textStart = pos
			continue
		case "nep-config":
			if err := flush(at); err != nil {
				return nil, err
			}
			for k, v := range parseAttrs(attrs) {
				s.Config[k] = v
			}
			pos, textStart = end, end
			continue
		case "page", "each", "if", "live":
			if err := flush(at); err != nil {
				return nil, err
			}
			n := &node{line: ln}
			a := parseAttrs(attrs)
			switch lname {
			case "page":
				if page != nil {
					return nil, fail(ln, "uma <page> não pode ficar dentro de outra")
				}
				if top.kind != kRoot {
					return nil, fail(ln, "<page> não pode ficar dentro de <%s>", kindName(top.kind))
				}
				route := first(a, "route", "rota")
				if route == "" {
					return nil, fail(ln, `<page> precisa do atributo route="/caminho"`)
				}
				n.kind = kPage
				n.id = len(s.Pages)
				re, params := routeRegex(route)
				_, auth := a["auth"]
				_, restr := a["restrita"]
				_, noindex := a["noindex"]
				s.Pages = append(s.Pages, &Page{ID: n.id, Route: route, Title: first(a, "title", "titulo"), Auth: auth || restr, Line: ln, re: re, params: params, node: n,
					Description: first(a, "description", "descricao"), Image: first(a, "image", "imagem"), NoIndex: noindex})
				page = n
			case "each":
				m := eachRe.FindStringSubmatch(attrs)
				if m == nil {
					return nil, fail(ln, `use <each item in="lista"> (ou <cada item em="lista">)`)
				}
				n.kind, n.v1, n.v2 = kEach, m[1], m[2]
				n.expr = m[4] + m[5]
			case "if":
				cond, ok := a["test"]
				if !ok {
					cond, ok = a["teste"]
				}
				if !ok || strings.TrimSpace(cond) == "" {
					return nil, fail(ln, `use <if test="condição"> (ou <se teste="condição">)`)
				}
				n.kind, n.expr = kIf, cond
			case "live":
				n.kind = kLive
				n.id = len(s.Lives)
				every, _ := strconv.Atoi(first(a, "every", "cada"))
				if every < 0 {
					every = 0
				}
				if every > 0 && every < 2 {
					every = 2
				}
				s.Lives = append(s.Lives, &Live{ID: n.id, Every: every})
			}
			top.add(n)
			stack = append(stack, n)
			pos, textStart = end, end
			continue
		case "else":
			if err := flush(at); err != nil {
				return nil, err
			}
			if (top.kind != kIf && top.kind != kEach) || top.inElse {
				return nil, fail(ln, "<else> só pode aparecer dentro de <if> ou <each>")
			}
			top.inElse = true
			pos, textStart = end, end
			continue
		case "form":
			a := parseAttrs(attrs)
			save, run := first(a, "save", "salvar"), first(a, "run", "executar")
			if save == "" && run == "" {
				pos = at + 1
				continue
			}
			if save != "" && !ValidCol(save) {
				return nil, fail(ln, "nome de coleção inválido em save=%q (use letras, números, - e _)", save)
			}
			if run != "" && !isIdent(run) {
				return nil, fail(ln, "run=%q precisa ser o nome de uma função", run)
			}
			if err := flush(at); err != nil {
				return nil, err
			}
			limit := 5
			if l, err := strconv.Atoi(first(a, "limit", "limite")); err == nil && l > 0 {
				limit = l
			}
			success := first(a, "success", "sucesso")
			if _, has := a["success"]; !has && success == "" {
				if _, has2 := a["sucesso"]; !has2 {
					success = "Recebido! Obrigado pelo contato."
				}
			}
			f := &Form{ID: len(s.Forms), Save: save, Run: run, Success: success, Redirect: first(a, "redirect", "redirecionar"),
				Notify: first(a, "notify", "avisar"), Limit: limit, Line: ln, start: end}
			s.Forms = append(s.Forms, f)
			n := &node{kind: kForm, line: ln, id: f.ID}
			clean := formAttrStrip.ReplaceAllString(" "+attrs, "")
			clean = strings.TrimRight(clean, " /")
			ns, err := s.textNodes(clean, ln)
			if err != nil {
				return nil, err
			}
			n.attrs = ns
			top.add(n)
			stack = append(stack, n)
			pos, textStart = end, end
			continue
		}
		pos = at + 1
	}
	if err := flush(len(src)); err != nil {
		return nil, err
	}
	if len(stack) > 1 {
		t := stack[len(stack)-1]
		return nil, fail(t.line, "a tag <%s> aberta na linha %d não foi fechada", kindName(t.kind), t.line)
	}
	s.markPrerun(s.root)
	return s, nil
}

var (
	eachRe        = regexp.MustCompile(`^\s*([\pL_][\pL\pN_]*)(?:\s*,\s*([\pL_][\pL\pN_]*))?\s+(?:in|em)\s*=\s*("([^"]*)"|'([^']*)')\s*/?\s*$`)
	formAttrStrip = regexp.MustCompile(`(?i)\s(save|salvar|run|executar|success|sucesso|redirect|redirecionar|notify|avisar|limit|limite|method|action)(\s*=\s*("[^"]*"|'[^']*'|[^\s>]+))?`)
	identRe       = regexp.MustCompile(`^[\pL_][\pL\pN_]*$`)
)

func isIdent(s string) bool { return identRe.MatchString(s) }

func kindName(k nodeKind) string {
	return map[nodeKind]string{kPage: "page", kEach: "each", kIf: "if", kForm: "form", kLive: "live"}[k]
}

func first(a map[string]string, keys ...string) string {
	for _, k := range keys {
		if v, ok := a[k]; ok {
			return v
		}
	}
	return ""
}

// scanTag lê "<nome atributos>" ou "</nome>" respeitando aspas.
func scanTag(s string) (name, attrs string, end int, closing, ok bool) {
	i := 1
	if i < len(s) && s[i] == '/' {
		closing = true
		i++
	}
	st := i
	for i < len(s) && (s[i] == '-' || s[i] == '_' || unicode.IsLetter(rune(s[i])) || (i > st && s[i] >= '0' && s[i] <= '9')) {
		i++
	}
	if i == st {
		return
	}
	name = s[st:i]
	if i < len(s) && !(s[i] == '>' || s[i] == '/' || s[i] == ' ' || s[i] == '\t' || s[i] == '\n' || s[i] == '\r') {
		return
	}
	as := i
	var q byte
	for ; i < len(s); i++ {
		c := s[i]
		if q != 0 {
			if c == q {
				q = 0
			}
			continue
		}
		if c == '"' || c == '\'' {
			q = c
			continue
		}
		if c == '>' {
			return name, s[as:i], i + 1, closing, true
		}
	}
	return
}

var attrRe = regexp.MustCompile(`([^\s=/>"']+)(?:\s*=\s*(?:"([^"]*)"|'([^']*)'|([^\s>"']+)))?`)

func parseAttrs(s string) map[string]string {
	r := map[string]string{}
	for _, m := range attrRe.FindAllStringSubmatch(s, -1) {
		r[strings.ToLower(m[1])] = m[2] + m[3] + m[4]
	}
	return r
}

func routeRegex(route string) (*regexp.Regexp, []string) {
	if route == "404" {
		return nil, nil
	}
	r := "/" + strings.Trim(route, "/")
	var names []string
	var b strings.Builder
	b.WriteString("^")
	for _, seg := range strings.Split(r, "/")[1:] {
		b.WriteString("/")
		switch {
		case strings.HasPrefix(seg, ":") && isIdent(seg[1:]):
			names = append(names, seg[1:])
			b.WriteString("([^/]+)")
		case seg == "*":
			names = append(names, "resto")
			b.WriteString("(.*)")
		default:
			b.WriteString(regexp.QuoteMeta(seg))
		}
	}
	if r == "/" {
		return regexp.MustCompile(`^/$`), nil
	}
	b.WriteString("$")
	return regexp.MustCompile(b.String()), names
}

func (s *Site) lit(text string, line int) *node {
	s.texts = append(s.texts, text)
	return &node{kind: kText, line: line, textIdx: len(s.texts) - 1}
}

// textNodes divide um trecho de HTML em texto fixo e {{ expressões }}.
func (s *Site) textNodes(t string, line int) ([]*node, error) {
	var out []*node
	for {
		i := strings.Index(t, "{{")
		if i < 0 {
			break
		}
		j := strings.Index(t[i+2:], "}}")
		if j < 0 {
			return nil, &TemplateError{File: s.File, Line: line + strings.Count(t[:i], "\n"), Msg: "{{ sem }}"}
		}
		if i > 0 {
			out = append(out, s.lit(t[:i], line))
			line += strings.Count(t[:i], "\n")
		}
		expr := t[i+2 : i+2+j]
		if strings.TrimSpace(expr) == "" {
			return nil, &TemplateError{File: s.File, Line: line, Msg: "{{ }} vazio"}
		}
		out = append(out, &node{kind: kExpr, line: line, text: expr})
		line += strings.Count(expr, "\n")
		t = t[i+2+j+2:]
	}
	if t != "" {
		out = append(out, s.lit(t, line))
	}
	return out, nil
}

// markPrerun marca os <nep> que rodam antes do HTML (fora de each/if/live).
func (s *Site) markPrerun(n *node) {
	for _, c := range n.children {
		switch c.kind {
		case kCode:
			c.prerun = true
			c.capIdx = s.caps
			s.caps++
		case kPage, kForm:
			s.markPrerun(c)
		}
	}
}

// ---------- geração de código ----------

type gen struct {
	b    strings.Builder
	line int
	tmp  int
}

func (g *gen) at(line int) {
	if g.line < line {
		g.b.WriteString(strings.Repeat("\n", line-g.line))
		g.line = line
	} else {
		g.b.WriteByte(' ')
	}
}

func (g *gen) put(s string) {
	g.b.WriteString(s)
	g.line += strings.Count(s, "\n")
}

// code escreve um bloco de código do usuário e termina a linha (um # no fim não engole o que vem depois).
func (g *gen) code(line int, src string) {
	g.at(line)
	g.put(src)
	g.put("\n")
}

// Sources devolve o código prévio e o código da página para a página pedida
// (page < 0: nenhuma, usado pelas APIs).
func (s *Site) Sources(page int) (pre, body string) {
	if c, ok := s.cache[page]; ok {
		return c[0], c[1]
	}
	var p, r gen
	p.line, r.line = 1, 1
	var walkPre func(n *node)
	walkPre = func(n *node) {
		for _, c := range n.children {
			switch c.kind {
			case kCode:
				if c.prerun {
					p.at(c.line)
					p.put(fmt.Sprintf("__cap(%d) ", c.capIdx))
					p.put(c.text)
					p.put(fmt.Sprintf("\n__endcap(%d)", c.capIdx))
				}
			case kPage:
				if c.id == page {
					walkPre(c)
				}
			case kForm:
				walkPre(c)
			}
		}
	}
	walkPre(s.root)
	s.walk(&r, s.root.children, page)
	pre, body = p.b.String(), r.b.String()
	s.cache[page] = [2]string{pre, body}
	return
}

func (s *Site) walk(g *gen, nodes []*node, page int) {
	for _, n := range nodes {
		switch n.kind {
		case kText:
			g.at(n.line)
			g.put(fmt.Sprintf("__w(%d)", n.textIdx))
		case kExpr:
			g.at(n.line)
			g.put("__e(" + n.text + ")")
		case kCode:
			if n.prerun {
				g.at(n.line)
				g.put(fmt.Sprintf("__wcap(%d)", n.capIdx))
			} else {
				g.code(n.line, n.text)
			}
		case kPage:
			if n.id == page {
				s.walk(g, n.children, page)
			}
		case kIf:
			g.at(n.line)
			g.put("if (" + n.expr + ") do")
			s.walk(g, n.children, page)
			if len(n.els) > 0 {
				g.put(" else")
				s.walk(g, n.els, page)
			}
			g.put(" end")
		case kEach:
			g.tmp++
			c, i := fmt.Sprintf("__c%d", g.tmp), fmt.Sprintf("__i%d", g.tmp)
			g.at(n.line)
			if n.v2 == "" {
				g.put(fmt.Sprintf("if true do let %s = __vals(%s) for %s, %s in %s do let loop = __loop(%s, %s)", c, n.expr, i, n.v1, c, i, c))
			} else {
				pv := fmt.Sprintf("__p%d", g.tmp)
				g.put(fmt.Sprintf("if true do let %s = __pairs(%s) for %s, %s in %s do let %s = %s[0] let %s = %s[1] let loop = __loop(%s, %s)",
					c, n.expr, i, pv, c, n.v1, pv, n.v2, pv, i, c))
			}
			s.walk(g, n.children, page)
			g.put(" end")
			if len(n.els) > 0 { // <else> no <each>: quando a lista está vazia
				g.put(fmt.Sprintf(" if len(%s) == 0 do", c))
				s.walk(g, n.els, page)
				g.put(" end")
			}
			g.put(" end")
		case kForm:
			g.at(n.line)
			g.put(fmt.Sprintf("__fo(%d)", n.id))
			s.walk(g, n.attrs, page)
			g.put(" __fa()")
			s.walk(g, n.children, page)
			g.put(" __fe()")
		case kLive:
			g.at(n.line)
			g.put(fmt.Sprintf("__lo(%d)", n.id))
			s.walk(g, n.children, page)
			g.put(fmt.Sprintf(" __lc(%d)", n.id))
		}
	}
}

// APISource gera o código de uma <api>: uma função cujo valor de retorno vira JSON.
func (s *Site) APISource(a *API) string {
	var g gen
	g.line = 1
	g.at(a.Line)
	g.put("fn __api() do ")
	g.put(a.code)
	g.put("\nend")
	return g.b.String()
}

// ---------- campos de formulário ----------

var (
	fieldRe  = regexp.MustCompile(`(?is)<(input|select|textarea)\b((?:[^>"']|"[^"]*"|'[^']*')*)>`)
	selectRe = regexp.MustCompile(`(?is)<select\b(?:[^>"']|"[^"]*"|'[^']*')*>(.*?)</select>`)
	optionRe = regexp.MustCompile(`(?is)<option\b((?:[^>"']|"[^"]*"|'[^']*')*)>`)
	tagsRe   = regexp.MustCompile(`(?s)<[^>]*>`)
	spaceRe  = regexp.MustCompile(`\s+`)
	keyRe    = regexp.MustCompile(`^[\pL\pN_\-]{1,64}$`)
)

func stripTags(s string) string {
	return strings.TrimSpace(spaceRe.ReplaceAllString(tagsRe.ReplaceAllString(s, " "), " "))
}

// labelFor acha o texto do <label> que envolve o campo, para mensagens de erro legíveis.
func labelFor(html string, off, length int) string {
	before := strings.ToLower(html[:off])
	l := strings.LastIndex(before, "<label")
	if l < 0 || strings.Contains(before[l:], "</label>") {
		return ""
	}
	gt := strings.IndexByte(html[l:off], '>')
	if gt < 0 {
		return ""
	}
	txt := stripTags(html[l+gt+1 : off])
	if txt == "" {
		after := html[off+length:]
		if e := strings.Index(strings.ToLower(after), "</label>"); e >= 0 {
			txt = stripTags(after[:e])
		}
	}
	if txt == "" || strings.Contains(txt, "{{") {
		return ""
	}
	if utf8.RuneCountInString(txt) > 40 {
		txt = strings.TrimSpace(string([]rune(txt)[:40])) + "…"
	}
	return txt
}

func extractFields(html string) []*Field {
	var list []*Field
	byName := map[string]*Field{}
	for _, m := range fieldRe.FindAllStringSubmatchIndex(html, -1) {
		full := html[m[0]:m[1]]
		tag := strings.ToLower(html[m[2]:m[3]])
		at := parseAttrs(html[m[4]:m[5]])
		name := at["name"]
		typ := strings.ToLower(at["type"])
		if typ == "" {
			typ = tag
			if tag == "input" {
				typ = "text"
			}
		}
		if name == "" || strings.HasPrefix(name, "_nep") {
			continue
		}
		switch typ {
		case "submit", "button", "reset", "image":
			continue
		}
		multi := strings.HasSuffix(name, "[]") && typ != "file"
		key := strings.TrimSuffix(name, "[]")
		if !keyRe.MatchString(key) {
			continue
		}
		f := byName[key]
		if f == nil {
			label := at["data-label"]
			if label == "" {
				label = at["title"]
			}
			if label == "" {
				label = labelFor(html, m[0], len(full))
			}
			if label == "" {
				label = strings.NewReplacer("_", " ", "-", " ").Replace(key)
				r := []rune(label)
				r[0] = unicode.ToUpper(r[0])
				label = string(r)
			}
			f = &Field{Name: key, Type: typ, Label: label, Multi: multi}
			byName[key] = f
			list = append(list, f)
		}
		if _, ok := at["required"]; ok {
			f.Required = true
		}
		if typ == "file" {
			f.Accept = strings.ToLower(at["accept"])
			f.MaxMB = 5
			if v, err := strconv.ParseFloat(first(at, "data-max-mb", "data-max"), 64); err == nil && v > 0 {
				f.MaxMB = min(v, 20)
			}
			_, pub := at["data-publico"]
			_, pub2 := at["data-public"]
			f.Public = pub || pub2
		}
		if v, err := strconv.Atoi(at["maxlength"]); err == nil {
			f.MaxLen = v
		}
		if v, err := strconv.ParseFloat(at["min"], 64); err == nil {
			f.Min = &v
		}
		if v, err := strconv.ParseFloat(at["max"], 64); err == nil {
			f.Max = &v
		}
		dynamic := []string{"dyn"}
		if typ == "radio" || typ == "checkbox" {
			val, has := at["value"]
			if !has {
				val = "on"
			}
			if strings.Contains(val, "{{") {
				f.Options = dynamic
			} else if !(len(f.Options) == 1 && f.Options[0] == "dyn") {
				f.Options = append(f.Options, val)
			}
		}
		if tag == "select" {
			sm := selectRe.FindStringSubmatch(html[m[0]:])
			if sm != nil {
				body := sm[1]
				lb := strings.ToLower(body)
				if strings.Contains(body, "{{") || strings.Contains(lb, "<each") || strings.Contains(lb, "<cada") {
					f.Options = dynamic
				} else {
					var opts []string
					idx := optionRe.FindAllStringSubmatchIndex(body, -1)
					for k, o := range idx {
						oa := parseAttrs(body[o[2]:o[3]])
						if v, ok := oa["value"]; ok {
							opts = append(opts, v)
							continue
						}
						end := len(body)
						if k+1 < len(idx) {
							end = idx[k+1][0]
						}
						opts = append(opts, stripTags(body[o[1]:end]))
					}
					f.Options = opts
				}
			}
		}
	}
	for _, f := range list {
		if len(f.Options) == 1 && f.Options[0] == "dyn" {
			f.Options = nil // opções geradas pelo código: aceita qualquer valor
		} else if f.Options != nil {
			sort.Strings(f.Options)
		}
	}
	return list
}

func hasOption(opts []string, v string) bool {
	i := sort.SearchStrings(opts, v)
	return i < len(opts) && opts[i] == v
}

// formOn diz se o formulário aparece na página (no layout ou dentro dela).
func (s *Site) formOn(page, form int) bool {
	var walk func(ns []*node) bool
	walk = func(ns []*node) bool {
		for _, n := range ns {
			switch n.kind {
			case kForm:
				if n.id == form || walk(n.children) {
					return true
				}
			case kPage:
				if n.id == page && walk(n.children) {
					return true
				}
			case kIf, kEach:
				if walk(n.children) || walk(n.els) {
					return true
				}
			case kLive:
				if walk(n.children) {
					return true
				}
			}
		}
		return false
	}
	return walk(s.root.children)
}

func (s *Site) hasRoute(path string) bool {
	for _, p := range s.Pages {
		if p.re != nil && p.re.MatchString(path) {
			return true
		}
	}
	for _, a := range s.APIs {
		if a.re.MatchString(path) {
			return true
		}
	}
	return false
}
