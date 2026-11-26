package main

import (
	"encoding/base64"
	"fmt"
	"html"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/Nanofylium/probable-plant/moldes"
	"github.com/Nanofylium/probable-plant/web"
)

// previa gera um único arquivo .html que abre com dois cliques, sem servidor,
// mostrando todas as páginas do site como ficam no ar (com o conteúdo de exemplo).
//
//	nep previa site.nep.html [saida.html]
//
// Formulários, busca e regiões ao vivo não funcionam na prévia: ela é uma foto.
func previa(args []string) int {
	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, "informe o site: nep previa site.nep.html [saida.html]")
		return 2
	}
	file := args[0]
	out := strings.TrimSuffix(strings.TrimSuffix(file, ".html"), ".nep") + "-previa.html"
	if len(args) > 1 {
		out = args[1]
	}
	html, n, err := gerarPrevia(file)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	if err := os.WriteFile(out, []byte(html), 0o644); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	fmt.Printf("Prévia com %d página(s) em %s\nAbra com dois cliques: não precisa de servidor.\n", n, out)
	return 0
}

var (
	hrefRe   = regexp.MustCompile(`(href|action)="([^"]*)"`)
	headRe   = regexp.MustCompile(`(?is)<head[^>]*>(.*?)</head>`)
	bodyRe   = regexp.MustCompile(`(?is)<body([^>]*)>(.*)</body>`)
	scriptRe = regexp.MustCompile(`(?is)<script\b[^>]*>.*?</script>`)
	fonteURL = regexp.MustCompile(`url\("((?:/_nep|\.\.)/fontes/[a-z0-9-]+\.woff2)"\)`)
	// imagens dos moldes (/_nep/molde/img/x.jpg): entram uma vez só na prévia
	moldeSrc = regexp.MustCompile(`src="/_nep/molde/img/([a-z0-9]+)\.jpg"`)
	moldeURL = regexp.MustCompile(`url\((?:'|"|&#39;|&#34;|&quot;)?(?:/_nep/molde/)?img/([a-z0-9]+)\.jpg(?:'|"|&#39;|&#34;|&quot;)?\)`)
	linkCSS  = regexp.MustCompile(`(?is)<link\b[^>]*rel="stylesheet"[^>]*href="([^"]+)"[^>]*>`)
	styleAt  = regexp.MustCompile(`(?is)\bstyle="([^"]*)"`)
	classAt  = regexp.MustCompile(`(?is)\bclass="([^"]*)"`)
)

var srcLocal = regexp.MustCompile(`src="/([^"/][^"]*)"`)

func gerarPrevia(file string) (string, int, error) {
	dados, err := os.MkdirTemp("", "nep-previa-")
	if err != nil {
		return "", 0, err
	}
	defer os.RemoveAll(dados)
	srv, err := web.New(web.Options{SiteFile: file, DataDir: dados})
	if srv == nil {
		return "", 0, err
	}
	get := func(p string) (int, string) {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "http://localhost"+p, nil)
		srv.ServeHTTP(rec, req)
		return rec.Code, rec.Body.String()
	}
	// chave da página: o caminho sem a consulta (?x=1)
	rota := func(h string) (string, bool) {
		u, err := url.Parse(html.UnescapeString(h))
		if err != nil || u.Scheme != "" || u.Host != "" || !strings.HasPrefix(u.Path, "/") {
			return "", false
		}
		p := u.Path
		if strings.HasPrefix(p, "/_") || strings.HasPrefix(p, "/api/") || strings.Contains(p, ".") {
			return "", false
		}
		return p, true
	}

	type pagina struct{ rota, body, attrs string }
	var paginas []pagina
	feitas := map[string]bool{}
	fila := []string{"/"}
	var head string
	for len(fila) > 0 && len(paginas) < 80 {
		p := fila[0]
		fila = fila[1:]
		if feitas[p] {
			continue
		}
		feitas[p] = true
		code, doc := get(p)
		if code != 200 {
			continue
		}
		if head == "" {
			if m := headRe.FindStringSubmatch(doc); m != nil {
				head = m[1]
			}
		}
		m := bodyRe.FindStringSubmatch(doc)
		if m == nil {
			continue
		}
		body := scriptRe.ReplaceAllString(m[2], "")
		for _, l := range hrefRe.FindAllStringSubmatch(body, -1) {
			if r, ok := rota(l[2]); ok && !feitas[r] {
				fila = append(fila, r)
			}
		}
		paginas = append(paginas, pagina{p, body, m[1]})
	}
	if len(paginas) == 0 {
		return "", 0, fmt.Errorf("a página inicial não abriu: rode nep dev %s e veja o erro", file)
	}

	// arquivos da pasta do site (fotos, quadros do passeio) viram caminhos
	// relativos, para a prévia achá-los ao lado dela
	pastaSite := filepath.Dir(file)
	relativos := func(s string) string {
		return srcLocal.ReplaceAllStringFunc(s, func(m string) string {
			sm := srcLocal.FindStringSubmatch(m)
			p := html.UnescapeString(sm[1])
			if strings.HasPrefix(p, "_") || strings.Contains(p, "..") {
				return m
			}
			if st, err := os.Stat(filepath.Join(pastaSite, filepath.FromSlash(p))); err == nil && !st.IsDir() {
				return `src="` + sm[1] + `"`
			}
			return m
		})
	}
	// links internos viram #/rota; o resto fica como está
	usadas := map[string]bool{}
	imagensMolde := func(s string) string {
		s = moldeSrc.ReplaceAllStringFunc(s, func(m string) string {
			n := moldeSrc.FindStringSubmatch(m)[1]
			usadas[n] = true
			return `src="data:image/gif;base64,R0lGODlhAQABAAAAACw=" data-nepm="` + n + `"`
		})
		return moldeURL.ReplaceAllStringFunc(s, func(m string) string {
			n := moldeURL.FindStringSubmatch(m)[1]
			usadas[n] = true
			return "var(--nepm-" + n + ")"
		})
	}
	trocaLinks := func(s string) string {
		s = imagensMolde(relativos(s))
		return hrefRe.ReplaceAllStringFunc(s, func(m string) string {
			sm := hrefRe.FindStringSubmatch(m)
			if r, ok := rota(sm[2]); ok && feitas[r] {
				frag := ""
				if u, err := url.Parse(html.UnescapeString(sm[2])); err == nil && u.Fragment != "" {
					frag = "|" + u.Fragment
				}
				return sm[1] + `="#` + html.EscapeString(r+frag) + `"`
			}
			return m
		})
	}

	// CSS: o tema e as folhas do próprio site entram no arquivo
	head = scriptRe.ReplaceAllString(head, "")
	head = linkCSS.ReplaceAllStringFunc(head, func(m string) string {
		href := html.UnescapeString(linkCSS.FindStringSubmatch(m)[1])
		if code, css := get(href); code == 200 && strings.HasPrefix(href, "/") {
			// as fontes dos visuais entram no arquivo, para abrir sem o nep
			css = fonteURL.ReplaceAllStringFunc(css, func(u string) string {
				if code, b := get("/_nep/fontes/" + path.Base(fonteURL.FindStringSubmatch(u)[1])); code == 200 {
					return `url("data:font/woff2;base64,` + base64.StdEncoding.EncodeToString([]byte(b)) + `")`
				}
				return u
			})
			return "<style>\n" + imagensMolde(css) + "\n</style>"
		}
		return m
	})
	head = regexp.MustCompile(`(?is)<link\b[^>]*rel="(alternate|canonical)"[^>]*>`).ReplaceAllString(head, "")

	var b strings.Builder
	b.WriteString("<!doctype html>\n<html lang=\"pt-BR\">\n<head>\n<!-- Prévia gerada por: nep previa " + html.EscapeString(filepath.Base(file)) + ". É uma foto do site: formulários e partes ao vivo funcionam só com o nep. -->\n")
	var corpo strings.Builder
	for _, p := range paginas {
		st, cl := "", ""
		if m := styleAt.FindStringSubmatch(p.attrs); m != nil {
			st = m[1]
		}
		if m := classAt.FindStringSubmatch(p.attrs); m != nil {
			cl = m[1]
		}
		corpo.WriteString("\n<template data-rota=\"" + html.EscapeString(p.rota) + "\" data-style=\"" + st + "\" data-class=\"" + cl + "\">" + trocaLinks(p.body) + "</template>")
	}
	inicio := trocaLinks(paginas[0].body)
	b.WriteString(head)
	if len(usadas) > 0 {
		nomes := make([]string, 0, len(usadas))
		for n := range usadas {
			nomes = append(nomes, n)
		}
		sort.Strings(nomes)
		var vars, mapa strings.Builder
		for _, n := range nomes {
			if img, ok := moldes.Arquivo("img/" + n + ".jpg"); ok {
				d := "data:image/jpeg;base64," + base64.StdEncoding.EncodeToString(img)
				vars.WriteString("--nepm-" + n + ":url(" + d + ");")
				fmt.Fprintf(&mapa, "%q:%q,", n, d)
			}
		}
		b.WriteString("\n<style>:root{" + vars.String() + "}</style>\n<script>window.NEPM={" + strings.TrimSuffix(mapa.String(), ",") + "};</script>")
	}
	b.WriteString(corpo.String())
	b.WriteString("\n</head>\n<body" + paginas[0].attrs + ">")
	b.WriteString(inicio)
	b.WriteString(`
<script>
/* Prévia do Nepenthes: troca de página sem servidor. */
(function () {
  function ir(h) {
    var r = decodeURIComponent((h || "#/").slice(1)).split("|"), alvo = r[1];
    var t = document.querySelector('template[data-rota="' + (r[0] || "/").replace(/"/g, "") + '"]');
    if (!t) return;
    document.body.innerHTML = t.innerHTML;
    document.body.setAttribute("style", t.getAttribute("data-style") || "");
    document.body.className = t.getAttribute("data-class") || "";
    var el = alvo && document.getElementById(alvo);
    imgs();
    if (el) el.scrollIntoView(); else window.scrollTo(0, 0);
  }
  window.addEventListener("hashchange", function () { ir(location.hash); });
  document.addEventListener("submit", function (e) {
    e.preventDefault();
    alert("Prévia: no site de verdade (rodando com o nep) este formulário envia, salva e manda os e-mails.");
  });
  function imgs() {
    var m = window.NEPM || {};
    document.querySelectorAll("img[data-nepm]").forEach(function (i) { if (m[i.dataset.nepm]) i.src = m[i.dataset.nepm]; });
  }
  imgs();
  if (location.hash.length > 2) ir(location.hash);
})();
</script>
</body>
</html>
`)
	return b.String(), len(paginas), nil
}
