package web

// Construtor visual (GrapesJS, código aberto, BSD-3-Clause) dentro do painel.
// O programador decide ONDE o cliente pode montar blocos arrastando:
//
//	<textarea name="layout" data-construtor></textarea>   num <conteudo>
//	<section construtor="home.meio">padrão</section>       em qualquer página
//
// e o tema continua sendo o código dele. O HTML montado é limpo no servidor
// (lista de tags e atributos permitidos, sem scripts nem eventos) e o CSS é
// isolado dentro de .nep-visual, para não bagunçar o resto do site.

import (
	"embed"
	"encoding/json"
	"html"
	"mime/multipart"
	"net/http"
	"path"
	"regexp"
	"sort"
	"strings"

	"github.com/Nanofylium/probable-plant/lang"
)

//go:embed assets/construtor
var construtorFS embed.FS

var construtorTypes = map[string]string{".js": "text/javascript; charset=utf-8", ".css": "text/css; charset=utf-8", ".txt": "text/plain; charset=utf-8", ".woff2": "font/woff2"}

// serveConstrutor entrega /_nep/construtor/ARQUIVO (só os arquivos embutidos).
func serveConstrutor(w http.ResponseWriter, r *http.Request, name string) {
	if strings.Contains(name, "/") || strings.HasPrefix(name, ".") {
		http.NotFound(w, r)
		return
	}
	b, err := construtorFS.ReadFile("assets/construtor/" + name)
	ct, ok := construtorTypes[path.Ext(name)]
	if err != nil || !ok {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", ct)
	w.Header().Set("Cache-Control", "public, max-age=86400")
	w.Write(b)
}

// ---------- limpeza do HTML montado ----------

var (
	visTags = map[string]bool{
		"div": true, "section": true, "article": true, "header": true, "footer": true, "main": true, "aside": true, "nav": true,
		"h1": true, "h2": true, "h3": true, "h4": true, "h5": true, "h6": true, "p": true, "span": true, "a": true, "img": true,
		"ul": true, "ol": true, "li": true, "strong": true, "em": true, "b": true, "i": true, "u": true, "s": true, "small": true,
		"blockquote": true, "figure": true, "figcaption": true, "br": true, "hr": true, "table": true, "thead": true, "tbody": true,
		"tfoot": true, "tr": true, "th": true, "td": true, "iframe": true, "video": true, "source": true, "picture": true, "sup": true,
		"sub": true, "code": true, "pre": true, "mark": true, "time": true, "address": true, "dl": true, "dt": true, "dd": true,
	}
	visAttrs = map[string]bool{"id": true, "class": true, "style": true, "title": true, "alt": true, "src": true, "href": true, "target": true,
		"width": true, "height": true, "colspan": true, "rowspan": true, "loading": true, "allowfullscreen": true, "controls": true,
		"poster": true, "datetime": true, "type": true, "srcset": false}
	// vídeos e mapas que o construtor sabe incluir
	frameOK  = regexp.MustCompile(`^https://(www\.youtube(-nocookie)?\.com/embed/|player\.vimeo\.com/video/|maps\.google\.com/maps\?|www\.google\.com/maps/)`)
	tokRe    = regexp.MustCompile(`(?s)<!--.*?-->|<(/?)([a-zA-Z][a-zA-Z0-9-]*)((?:[^>"']|"[^"]*"|'[^']*')*)>`)
	badStyle = regexp.MustCompile(`(?i)expression\s*\(|javascript:|vbscript:|-moz-binding|behavior\s*:|@import`)
	cssData  = regexp.MustCompile(`(?i)url\s*\(\s*['"]?\s*data:([a-z/+.-]*)`)
	styleBlk = regexp.MustCompile(`(?is)<style[^>]*>(.*?)</style>`)
	dropWith = map[string]bool{"script": true, "style": true, "template": true, "noscript": true, "textarea": true, "title": true, "svg": true, "math": true, "object": true, "embed": true}
)

func okDataImg(t string) bool {
	switch strings.ToLower(t) {
	case "image/png", "image/jpeg", "image/jpg", "image/gif", "image/webp":
		return true
	}
	return false
}

// badCSS: estilo com código ou endereço perigoso.
func badCSS(v string) bool {
	if badStyle.MatchString(v) {
		return true
	}
	for _, m := range cssData.FindAllStringSubmatch(v, -1) {
		if !okDataImg(m[1]) {
			return true
		}
	}
	return false
}

func voidVis(t string) bool { return t == "img" || t == "br" || t == "hr" || t == "source" }

func isDataImg(v string) bool {
	l := strings.ToLower(v)
	if !strings.HasPrefix(l, "data:") {
		return false
	}
	t := l[5:]
	if i := strings.IndexAny(t, ";,"); i >= 0 {
		t = t[:i]
	}
	return okDataImg(t)
}

// cleanVisual limpa o HTML: tira tags e atributos fora da lista, eventos e links perigosos.
func cleanVisual(src string) string {
	var b strings.Builder
	last := 0
	skip := "" // dentro de <script>... até fechar
	for _, m := range tokRe.FindAllStringSubmatchIndex(src, -1) {
		if skip == "" {
			b.WriteString(strings.ReplaceAll(src[last:m[0]], ">", "&gt;"))
		}
		last = m[1]
		if m[4] < 0 { // comentário
			continue
		}
		closing := m[3] > m[2]
		tag := strings.ToLower(src[m[4]:m[5]])
		if skip != "" {
			if closing && tag == skip {
				skip = ""
			}
			continue
		}
		if dropWith[tag] {
			if !closing && !strings.HasSuffix(strings.TrimSpace(src[m[6]:m[7]]), "/") {
				skip = tag
			}
			continue
		}
		if !visTags[tag] {
			continue
		}
		if closing {
			if !voidVis(tag) {
				b.WriteString("</" + tag + ">")
			}
			continue
		}
		b.WriteString("<" + tag)
		at := parseAttrs(src[m[6]:m[7]])
		keys := make([]string, 0, len(at))
		for k := range at {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			v := html.UnescapeString(at[k])
			if !visAttrs[k] {
				continue
			}
			switch k {
			case "href", "src", "poster":
				if tag == "iframe" && k == "src" {
					if !frameOK.MatchString(v) {
						v = ""
					}
				} else if !safeURL(v) && !(k != "href" && isDataImg(v)) {
					v = ""
				}
				if v == "" {
					continue
				}
			case "style":
				if badCSS(v) {
					continue
				}
			case "target":
				if v != "_blank" && v != "_self" {
					continue
				}
			}
			b.WriteString(" " + k + `="` + html.EscapeString(v) + `"`)
		}
		if tag == "a" && strings.EqualFold(at["target"], "_blank") {
			b.WriteString(` rel="noopener"`)
		}
		if tag == "iframe" {
			b.WriteString(` sandbox="allow-scripts allow-same-origin allow-presentation allow-popups" referrerpolicy="strict-origin-when-cross-origin"`)
		}
		b.WriteString(">")
	}
	if skip == "" {
		b.WriteString(strings.ReplaceAll(src[last:], ">", "&gt;"))
	}
	return b.String()
}

// scopeCSS prende todas as regras dentro de .nep-visual (body/html viram o próprio bloco).
func scopeCSS(css string) string {
	css = strings.ReplaceAll(css, "<", "")
	css = badStyle.ReplaceAllString(css, "x-")
	css = cssData.ReplaceAllStringFunc(css, func(m string) string {
		if okDataImg(cssData.FindStringSubmatch(m)[1]) {
			return m
		}
		return "url(x-"
	})
	var b strings.Builder
	i := 0
	for i < len(css) {
		j := strings.IndexByte(css[i:], '{')
		if j < 0 {
			break
		}
		sel := strings.TrimSpace(css[i : i+j])
		depth, k := 1, i+j+1
		for k < len(css) && depth > 0 {
			switch css[k] {
			case '{':
				depth++
			case '}':
				depth--
			}
			k++
		}
		body := css[i+j+1 : max(k-1, i+j+1)]
		switch {
		case strings.HasPrefix(sel, "@media") || strings.HasPrefix(sel, "@supports"):
			b.WriteString(sel + "{" + scopeCSS(body) + "}")
		case strings.HasPrefix(sel, "@keyframes") || strings.HasPrefix(sel, "@font-face"):
			b.WriteString(sel + "{" + body + "}")
		case strings.HasPrefix(sel, "@"):
			// outras regras @ ficam de fora
		default:
			var parts []string
			for _, s := range strings.Split(sel, ",") {
				s = strings.TrimSpace(s)
				if s == "" {
					continue
				}
				low := strings.ToLower(s)
				switch {
				case low == "body" || low == "html" || low == "*" || low == ":root":
					parts = append(parts, ".nep-visual")
				case strings.HasPrefix(low, "body ") || strings.HasPrefix(low, "html "):
					parts = append(parts, ".nep-visual "+strings.TrimSpace(s[5:]))
				default:
					parts = append(parts, ".nep-visual "+s)
				}
			}
			if len(parts) > 0 {
				b.WriteString(strings.Join(parts, ",") + "{" + body + "}")
			}
		}
		i = k
	}
	return b.String()
}

// splitVisual separa o CSS (<style>) do HTML guardados juntos.
func splitVisual(v string) (htm, css string) {
	var cs []string
	htm = styleBlk.ReplaceAllStringFunc(v, func(m string) string {
		cs = append(cs, styleBlk.FindStringSubmatch(m)[1])
		return ""
	})
	return strings.TrimSpace(htm), strings.Join(cs, "\n")
}

// Visual devolve o bloco montado no construtor pronto para a página.
func Visual(v string) lang.HTML {
	if strings.TrimSpace(v) == "" {
		return ""
	}
	htm, css := splitVisual(v)
	out := `<div class="nep-visual">`
	if c := scopeCSS(css); c != "" {
		out += "<style>" + c + "</style>"
	}
	return lang.HTML(out + cleanVisual(htm) + `</div>`)
}

// uploadBuilder recebe imagens do construtor (/_admin/construtor/arquivo).
func (a *adminPage) uploadBuilder() error {
	w, r := a.rq.w, a.rq.r
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	if r.Method != http.MethodPost || r.MultipartForm == nil {
		w.WriteHeader(http.StatusBadRequest)
		return nil
	}
	var all []*multipart.FileHeader
	for _, fhs := range r.MultipartForm.File {
		all = append(all, fhs...)
	}
	if len(all) > 10 {
		all = all[:10]
	}
	var urls []map[string]string
	for _, fh := range all {
		{
			r.MultipartForm.File = map[string][]*multipart.FileHeader{"imagem": {fh}}
			files, errs := checkFiles([]*Field{imgField}, r)
			if len(errs) > 0 {
				w.WriteHeader(http.StatusUnprocessableEntity)
				return json.NewEncoder(w).Encode(map[string]string{"erro": errs[0]})
			}
			t := lang.NewTable()
			if _, err := a.s.saveFiles(files, t); err != nil {
				return err
			}
			img, _ := t.Get("imagem")
			a.s.addMedia(img, a.who()) // entra na biblioteca de mídia
			u, _ := img.(*lang.Table).Get("url")
			urls = append(urls, map[string]string{"src": lang.Str(u), "type": "image"})
		}
	}
	return json.NewEncoder(w).Encode(map[string]any{"data": urls})
}
