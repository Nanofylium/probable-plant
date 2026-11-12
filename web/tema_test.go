package web

import (
	"strings"
	"testing"
)

const siteSemCSS = `<!doctype html><html><head><title>Oi</title></head><body>
<!-- comentário com <head> dentro, que não pode confundir -->
<page route="/" title="Início"><h1>Olá</h1></page></body></html>`

func TestTemaEntraSozinhoSemCSS(t *testing.T) {
	c, _ := newTCSite(t, siteSemCSS)
	_, body, _ := c.get("/")
	must(t, body, `/_nep/tema.css?v=`+temaVersao, `name="viewport"`)
	if i, j := strings.Index(body, "tema.css"), strings.Index(body, "<title>"); i > j {
		t.Error("o tema tem de vir no começo do <head>, antes do CSS do site")
	}
	code, css, _ := c.get("/_nep/tema.css")
	if code != 200 || !strings.Contains(css, "--nep-cor") {
		t.Fatalf("tema.css: %d", code)
	}
}

func TestTemaRespeitaCSSProprio(t *testing.T) {
	c, _ := newTCSite(t, strings.Replace(siteSemCSS, "<title>", "<style>h1{color:red}</style><title>", 1))
	_, body, _ := c.get("/")
	if strings.Contains(body, "tema.css") {
		t.Error("site com CSS próprio não deveria ganhar o tema sem pedir")
	}
	c2, _ := newTCSite(t, strings.Replace(siteSemCSS, "<body>", `<body><nep-config tema="nepenthes" cor="#9a5317" fonte="serifa"><style>h1{color:red}</style>`, 1))
	_, body, _ = c2.get("/")
	must(t, body, "tema.css", "--nep-cor:#9a5317", "--nep-fonte:Georgia")
	c3, _ := newTCSite(t, strings.Replace(siteSemCSS, "<body>", `<body><nep-config tema="nenhum">`, 1))
	_, body, _ = c3.get("/")
	if strings.Contains(body, "tema.css") {
		t.Error(`tema="nenhum" deveria desligar o tema`)
	}
}

func TestTemaCorInvalidaIgnorada(t *testing.T) {
	c, _ := newTCSite(t, strings.Replace(siteSemCSS, "<body>", `<body><nep-config cor="red;}body{display:none">`, 1))
	_, body, _ := c.get("/")
	if strings.Contains(body, "display:none") {
		t.Error("cor inválida entrou no CSS")
	}
}
