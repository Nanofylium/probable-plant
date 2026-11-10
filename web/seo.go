package web

// SEO automático: /sitemap.xml e /robots.txt gerados a partir das páginas, e as
// etiquetas de descrição, prévia em redes sociais (Open Graph) e endereço
// canônico colocadas em cada página, sem o autor precisar escrever nada.

import (
	"html"
	"io"
	"strings"
	"time"

	"github.com/Nanofylium/probable-plant/lang"
)

// siteURL é o endereço público do site: <nep-config url="https://..."> ou o do pedido.
func (s *Server) siteURL(rq *request) string {
	if u := strings.TrimRight(rq.site.Config["url"], "/"); u != "" {
		return u
	}
	scheme := "http"
	if rq.https {
		scheme = "https"
	}
	return scheme + "://" + rq.r.Host + s.opt.Base
}

func (s *Server) sitemap(rq *request) {
	base := s.siteURL(rq)
	var b strings.Builder
	b.WriteString(`<?xml version="1.0" encoding="UTF-8"?>` + "\n" + `<urlset xmlns="http://www.sitemaps.org/schemas/sitemap/0.9">` + "\n")
	day := time.Now().Format("2006-01-02")
	for _, p := range rq.site.Pages {
		r := "/" + strings.Trim(p.Route, "/")
		if p.Conteudo != "" && !p.Auth && !p.NoIndex { // um endereço por item publicado
			for _, rec := range s.Store.All(p.Conteudo, 0) {
				t := rec.ToValue()
				if !published(t) {
					continue
				}
				u := rq.site.itemURL(p.Conteudo, t)
				mod := rec.Updated
				if mod == "" {
					mod = rec.Created
				}
				if len(mod) >= 10 {
					mod = mod[:10]
				} else {
					mod = day
				}
				b.WriteString("  <url><loc>" + html.EscapeString(base+"/"+strings.Trim(u, "/")) + "</loc><lastmod>" + mod + "</lastmod></url>\n")
			}
			continue
		}
		if p.Auth || p.Route == "404" || strings.ContainsAny(r, ":*") || p.NoIndex {
			continue
		}
		if r == "/" {
			r = "/"
		}
		b.WriteString("  <url><loc>" + html.EscapeString(base+r) + "</loc><lastmod>" + day + "</lastmod></url>\n")
	}
	b.WriteString("</urlset>\n")
	rq.w.Header().Set("Content-Type", "application/xml; charset=utf-8")
	io.WriteString(rq.w, b.String())
}

func (s *Server) robots(rq *request) {
	rq.w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	io.WriteString(rq.w, "User-agent: *\nDisallow: "+s.opt.Base+"/_admin\nDisallow: "+s.opt.Base+"/_nep/\n\nSitemap: "+s.siteURL(rq)+"/sitemap.xml\n")
}

// seoTags monta as etiquetas que a página ainda não tem.
func (s *Server) seoTags(rq *request, out string) []string {
	if rq.module == nil {
		return nil
	}
	pg, _ := rq.module.Globals["page"].(*lang.Table)
	if pg == nil {
		return nil
	}
	get := func(k string) string {
		v, _ := pg.Get(k)
		if v == nil {
			return ""
		}
		return strings.TrimSpace(lang.Str(v))
	}
	head := strings.ToLower(out)
	if i := strings.Index(head, "</head>"); i >= 0 {
		head = head[:i]
	}
	has := func(needle string) bool { return strings.Contains(head, needle) }
	e := html.EscapeString
	var t []string
	title, desc, img := get("title"), get("description"), get("image")
	if title == "" {
		if i := strings.Index(head, "<title>"); i >= 0 {
			if j := strings.Index(head[i:], "</title>"); j > 0 {
				title = strings.TrimSpace(html.UnescapeString(out[i+7 : i+j]))
			}
		}
	}
	page := rq.site.Pages[rq.page.ID]
	if page.NoIndex || rq.page.Auth {
		if !has(`name="robots"`) {
			t = append(t, `<meta name="robots" content="noindex">`)
		}
	}
	if desc != "" && !has(`name="description"`) {
		t = append(t, `<meta name="description" content="`+e(desc)+`">`)
	}
	canon := s.siteURL(rq) + rq.path
	if rq.path == "/" {
		canon = s.siteURL(rq) + "/"
	}
	if !has(`rel="canonical"`) && rq.page.Route != "404" {
		t = append(t, `<link rel="canonical" href="`+e(canon)+`">`)
	}
	if !has(`property="og:`) {
		if title != "" {
			t = append(t, `<meta property="og:title" content="`+e(title)+`">`)
		}
		if desc != "" {
			t = append(t, `<meta property="og:description" content="`+e(desc)+`">`)
		}
		t = append(t, `<meta property="og:type" content="website">`, `<meta property="og:url" content="`+e(canon)+`">`)
		if img != "" {
			if strings.HasPrefix(img, "/") {
				img = s.siteURL(rq) + img
			}
			t = append(t, `<meta property="og:image" content="`+e(img)+`">`)
		}
		if lc := rq.site.Config["lang"]; lc != "" {
			t = append(t, `<meta property="og:locale" content="`+e(strings.ReplaceAll(lc, "-", "_"))+`">`)
		}
	}
	if !has(`name="twitter:card"`) {
		card := "summary"
		if img != "" {
			card = "summary_large_image"
		}
		t = append(t, `<meta name="twitter:card" content="`+card+`">`)
	}
	return t
}
