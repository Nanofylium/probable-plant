package web

// markdown() transforma texto simples com marcações leves em HTML seguro.
// Serve para quem cuida do site escrever artigos e textos longos no painel sem
// saber HTML. Todo HTML digitado é mostrado como texto (nunca executado), e
// links só aceitam http, https, mailto, tel e endereços do próprio site.
//
//	# Título          -> <h2>   (## -> <h3>, ### -> <h4>)
//	**negrito**  *itálico*  `código`
//	[texto](https://...)   ![descrição](/_nep/arquivo/...)
//	- item / 1. item  > citação  --- linha  ``` bloco de código ```

import (
	"html"
	"regexp"
	"strconv"
	"strings"
)

var (
	mdHeading = regexp.MustCompile(`^(#{1,6})\s+(.*?)\s*#*\s*$`)
	mdUL      = regexp.MustCompile(`^\s*[-*+]\s+(.*)$`)
	mdOL      = regexp.MustCompile(`^\s*\d{1,9}[.)]\s+(.*)$`)
	mdHR      = regexp.MustCompile(`^\s*(?:(?:-\s*){3,}|(?:\*\s*){3,}|(?:_\s*){3,})$`)
	mdCode    = regexp.MustCompile("`([^`\n]+)`")
	mdImg     = regexp.MustCompile(`!\[([^\]\n]*)\]\(([^)\s]+)\)`)
	mdLink    = regexp.MustCompile(`\[([^\]\n]+)\]\(([^)\s]+)\)`)
	mdBold    = regexp.MustCompile(`\*\*([^*\n]+)\*\*|__([^_\n]+)__`)
	mdItalic  = regexp.MustCompile(`\*([^*\n]+)\*|\b_([^_\n]+)_\b`)
	mdPH      = regexp.MustCompile("\x00(\\d+)\x00")
)

func safeURL(u string) bool {
	l := strings.ToLower(u)
	switch {
	case strings.HasPrefix(l, "http://"), strings.HasPrefix(l, "https://"), strings.HasPrefix(l, "mailto:"), strings.HasPrefix(l, "tel:"):
		return true
	case strings.HasPrefix(u, "/") && !strings.HasPrefix(u, "//"), strings.HasPrefix(u, "#"):
		return true
	}
	return false
}

// mdInline trata negrito, itálico, código, links e imagens de uma linha.
func mdInline(t string) string {
	var keep []string
	hold := func(s string) string {
		keep = append(keep, s)
		return "\x00" + strconv.Itoa(len(keep)-1) + "\x00"
	}
	t = strings.ReplaceAll(t, "\x00", "")
	t = mdCode.ReplaceAllStringFunc(t, func(m string) string {
		return hold("<code>" + html.EscapeString(mdCode.FindStringSubmatch(m)[1]) + "</code>")
	})
	t = mdImg.ReplaceAllStringFunc(t, func(m string) string {
		p := mdImg.FindStringSubmatch(m)
		if !safeURL(p[2]) {
			return hold(html.EscapeString(p[1]))
		}
		return hold(`<img src="` + html.EscapeString(p[2]) + `" alt="` + html.EscapeString(p[1]) + `" loading="lazy">`)
	})
	t = mdLink.ReplaceAllStringFunc(t, func(m string) string {
		p := mdLink.FindStringSubmatch(m)
		txt := mdInline(p[1])
		if !safeURL(p[2]) {
			return hold(txt)
		}
		extra := ""
		if strings.HasPrefix(strings.ToLower(p[2]), "http") {
			extra = ` rel="noopener"`
		}
		return hold(`<a href="` + html.EscapeString(p[2]) + `"` + extra + `>` + txt + `</a>`)
	})
	t = html.EscapeString(t)
	t = mdBold.ReplaceAllStringFunc(t, func(m string) string {
		p := mdBold.FindStringSubmatch(m)
		return "<strong>" + p[1] + p[2] + "</strong>"
	})
	t = mdItalic.ReplaceAllStringFunc(t, func(m string) string {
		p := mdItalic.FindStringSubmatch(m)
		return "<em>" + p[1] + p[2] + "</em>"
	})
	for i := 0; i < 3 && strings.Contains(t, "\x00"); i++ {
		t = mdPH.ReplaceAllStringFunc(t, func(m string) string {
			n, _ := strconv.Atoi(mdPH.FindStringSubmatch(m)[1])
			return keep[n]
		})
	}
	return t
}

// Markdown converte o texto em HTML seguro.
func Markdown(src string) string {
	lines := strings.Split(strings.ReplaceAll(src, "\r\n", "\n"), "\n")
	var b strings.Builder
	var para []string
	list := "" // "ul" ou "ol" aberta
	flushPara := func() {
		if len(para) > 0 {
			parts := make([]string, len(para))
			for i, l := range para {
				parts[i] = mdInline(strings.TrimSpace(l))
			}
			b.WriteString("<p>" + strings.Join(parts, "<br>\n") + "</p>\n")
			para = nil
		}
	}
	closeList := func() {
		if list != "" {
			b.WriteString("</" + list + ">\n")
			list = ""
		}
	}
	for i := 0; i < len(lines); i++ {
		l := lines[i]
		trim := strings.TrimSpace(l)
		switch {
		case strings.HasPrefix(trim, "```"):
			flushPara()
			closeList()
			var code []string
			for i++; i < len(lines) && !strings.HasPrefix(strings.TrimSpace(lines[i]), "```"); i++ {
				code = append(code, lines[i])
			}
			b.WriteString("<pre><code>" + html.EscapeString(strings.Join(code, "\n")) + "</code></pre>\n")
		case trim == "":
			flushPara()
			closeList()
		case mdHeading.MatchString(trim):
			flushPara()
			closeList()
			m := mdHeading.FindStringSubmatch(trim)
			n := len(m[1]) + 1
			if n > 6 {
				n = 6
			}
			h := "h" + strconv.Itoa(n)
			b.WriteString("<" + h + ">" + mdInline(m[2]) + "</" + h + ">\n")
		case mdHR.MatchString(trim):
			flushPara()
			closeList()
			b.WriteString("<hr>\n")
		case strings.HasPrefix(trim, ">"):
			flushPara()
			closeList()
			var q []string
			for ; i < len(lines) && strings.HasPrefix(strings.TrimSpace(lines[i]), ">"); i++ {
				q = append(q, strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(lines[i]), ">")))
			}
			i--
			b.WriteString("<blockquote>" + strings.TrimSpace(Markdown(strings.Join(q, "\n"))) + "</blockquote>\n")
		case mdUL.MatchString(l) || mdOL.MatchString(l):
			flushPara()
			kind, m := "ul", mdUL.FindStringSubmatch(l)
			if m == nil {
				kind, m = "ol", mdOL.FindStringSubmatch(l)
			}
			if list != kind {
				closeList()
				b.WriteString("<" + kind + ">\n")
				list = kind
			}
			b.WriteString("<li>" + mdInline(m[1]) + "</li>\n")
		default:
			closeList()
			para = append(para, l)
		}
	}
	flushPara()
	closeList()
	return strings.TrimSpace(b.String())
}
