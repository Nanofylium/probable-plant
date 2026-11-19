package web

// Visuais: famílias de aparência que vão por cima do tema Nepenthes e mudam a
// estrutura do site (cabeçalho, capa, cartões, botões, rodapé), não só a cor.
// <nep-config visual="noturno"> liga um deles. Cada um traz suas fontes
// (Fontsource, SIL OFL 1.1, servidas pelo próprio nep: nada de Google Fonts).

import (
	"crypto/sha256"
	"embed"
	"encoding/hex"
	"html"
	"net/http"
	"path"
	"strings"
	"time"
)

//go:embed assets/visuais/*.css assets/fontes/*.woff2
var visuaisFS embed.FS

// Visuais lista os nomes aceitos em visual="...".
var Visuais = func() []string {
	ls, _ := visuaisFS.ReadDir("assets/visuais")
	var out []string
	for _, f := range ls {
		out = append(out, strings.TrimSuffix(f.Name(), ".css"))
	}
	return out
}()

var visualVersao = func() string {
	h := sha256.New()
	for _, n := range Visuais {
		b, _ := visuaisFS.ReadFile("assets/visuais/" + n + ".css")
		h.Write(b)
	}
	return hex.EncodeToString(h.Sum(nil)[:4])
}()

func visualExiste(n string) bool {
	for _, v := range Visuais {
		if v == n {
			return true
		}
	}
	return false
}

// visualTag: o <link> do visual, para o fim do <head> (depois do CSS do site,
// porque o visual reorganiza peças que o site gerado também estiliza).
func (s *Server) visualTag(cfg map[string]string) string {
	n := strings.ToLower(strings.TrimSpace(cfg["visual"]))
	if n == "" || !visualExiste(n) {
		return ""
	}
	return `<link rel="stylesheet" href="` + html.EscapeString(s.opt.Base) + `/_nep/visual/` + n + `.css?v=` + visualVersao + `">`
}

func (s *Server) serveVisual(w http.ResponseWriter, r *http.Request, nome string) {
	nome = path.Base(nome)
	switch {
	case strings.HasSuffix(nome, ".css"):
		b, err := visuaisFS.ReadFile("assets/visuais/" + nome)
		if err != nil {
			http.NotFound(w, r)
			return
		}
		css := strings.ReplaceAll(string(b), "{{fontes}}", s.opt.Base+"/_nep/fontes")
		w.Header().Set("Content-Type", "text/css; charset=utf-8")
		w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
		http.ServeContent(w, r, "", time.Time{}, strings.NewReader(css))
	case strings.HasSuffix(nome, ".woff2"):
		b, err := visuaisFS.ReadFile("assets/fontes/" + nome)
		if err != nil {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "font/woff2")
		w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
		http.ServeContent(w, r, "", time.Time{}, strings.NewReader(string(b)))
	default:
		http.NotFound(w, r)
	}
}

// noFimDoHead põe a tag logo antes de </head> (ou no começo, se não houver).
func noFimDoHead(out, tag string) string {
	if tag == "" {
		return out
	}
	low := strings.ToLower(out)
	if i := strings.Index(low, "</head>"); i >= 0 {
		return out[:i] + tag + "\n" + out[i:]
	}
	return noComecoDoHead(out, []string{tag})
}
