package web

import (
	"net/url"
	"os"
	"strings"
	"testing"
	"time"
)

func TestLimpezaDoConstrutor(t *testing.T) {
	in := `<style>body{margin:0;background:red} .a, h1{color:blue} @media (max-width:480px){.a{color:green}} @import url(x.css); .b{background:url(javascript:alert(1))} .c{background:url(data:image/svg+xml,x)} .d{background:url(data:image/png;base64,AA)}</style>` +
		`<section id="s1" class="a" onclick="alert(1)" data-gjs-type="x"><h1 style="color:red;behavior:url(x)">Oi &amp; tchau</h1>` +
		`<a href="javascript:alert(1)">ruim</a><a href="https://ex.com" target="_blank">bom</a>` +
		`<img src="/_nep/arquivo/p-x.png" onerror="alert(1)"><img src="data:image/svg+xml;base64,PHN2Zz4=">` +
		`<script>alert(2)</script><svg><script>alert(3)</script></svg><iframe src="https://evil.com"></iframe>` +
		`<iframe src="https://www.youtube.com/embed/abc"></iframe><form><input></form><!-- c -->texto > fim</section>`
	got := string(Visual(in))
	for _, w := range []string{
		`<div class="nep-visual"><style>`,
		`.nep-visual{margin:0;background:red}`,
		`.nep-visual .a,.nep-visual h1{color:blue}`,
		`@media (max-width:480px){.nep-visual .a{color:green}}`,
		`.nep-visual .d{background:url(data:image/png;base64,AA)}`,
		`<section class="a" id="s1"><h1>Oi &amp; tchau</h1>`,
		`<a>ruim</a>`,
		`<a href="https://ex.com" target="_blank" rel="noopener">bom</a>`,
		`<img src="/_nep/arquivo/p-x.png"><img>`,
		`<iframe src="https://www.youtube.com/embed/abc" sandbox=`,
		`texto &gt; fim</section></div>`,
	} {
		if !strings.Contains(got, w) {
			t.Errorf("faltou %q em:\n%s", w, got)
		}
	}
	for _, bad := range []string{"onclick", "onerror", "javascript", "alert(2)", "alert(3)", "evil.com", "@import", "svg+xml", "data-gjs", "<form", "<input", "<!--", "behavior"} {
		if strings.Contains(got, bad) {
			t.Errorf("sobrou %q em:\n%s", bad, got)
		}
	}
	if Visual("  ") != "" {
		t.Error("vazio deveria dar vazio")
	}
	if g := string(Visual(`<style></style><p>x</p>`)); g != `<div class="nep-visual"><p>x</p></div>` {
		t.Errorf("sem css: %s", g)
	}
	if g := scopeCSS(`</style><script>alert(1)</script>{}`); strings.Contains(g, "<") {
		t.Errorf("css fechando a tag: %s", g)
	}
}

func TestConstrutorNoPainel(t *testing.T) {
	c := newTCMenu(t)
	srv := c.srv.Config.Handler.(*Server)
	site := `<!doctype html><html><head></head><body>
<conteudo nome="paginas"><input name="titulo" required><textarea name="layout" data-construtor></textarea></conteudo>
<section construtor="home.faixa"><p>padrão</p></section>
<page route="/" title="I">i</page>
<page route="/p/:slug" conteudo="paginas">{{ visual(item.layout) }}</page>
</body></html>`
	writeSite(t, srv, site)
	v := anon(c)
	_, body, _ := v.get("/")
	must(t, body, `<section data-nep-ed="home.faixa" data-nep-tipo="visual"><p>padrão</p></section>`)
	if code, _, _ := v.get("/_nep/construtor/grapes.min.js"); code != 200 {
		t.Errorf("grapes.min.js: %d", code)
	}
	if code, _, _ := v.get("/_nep/construtor/..%2Fvivo.js"); code != 404 {
		t.Errorf("caminho fora da pasta: %d", code)
	}
	tok := loginAdmin(c)
	_, body, _ = c.get("/_admin/textos")
	must(t, body, `<textarea name="valor" data-construtor>&lt;p&gt;padrão&lt;/p&gt;</textarea>`, "/_nep/construtor/nep.js", "GrapesJS")
	tok = csrfRe.FindStringSubmatch(body)[1]
	c.post("/_admin/textos", url.Values{"_nep_csrf": {tok}, "chave": {"home.faixa"}, "valor": {`<style>body{color:red}</style><h2 onclick="x()">Promo</h2><script>x()</script>`}})
	_, body, _ = v.get("/")
	must(t, body, `data-nep-tipo="visual"><div class="nep-visual"><style>.nep-visual{color:red}</style><h2>Promo</h2></div></section>`)

	_, body, _ = c.get("/_admin/conteudo/editar?c=paginas")
	must(t, body, `data-construtor`, "/_nep/construtor/grapes.min.js")
	long := `<p>` + strings.Repeat("a", 20000) + `</p>`
	if code, _, _ := c.multipart("/_admin/conteudo/editar?c=paginas", map[string]string{"_nep_csrf": tok, "titulo": "Promo", "layout": long, "_publicado": "on"}, nil); code != 303 {
		t.Fatalf("layout grande (acima do limite comum de textarea): %d", code)
	}
	_, body, _ = v.get("/p/promo")
	must(t, body, `<div class="nep-visual"><p>aaaa`)

	code, js, _ := c.multipart("/_admin/construtor/arquivo", map[string]string{"_nep_csrf": tok}, map[string][]byte{"arquivos[]": pngBytes()})
	if code != 200 || !strings.Contains(js, `"src":"/_nep/arquivo/p-`) {
		t.Errorf("upload do construtor: %d %s", code, js)
	}
	code, _, _ = c.multipart("/_admin/construtor/arquivo", map[string]string{"_nep_csrf": tok}, map[string][]byte{"arquivos[]": []byte("<html>")})
	if code != 422 {
		t.Errorf("upload de html: %d", code)
	}
	if code, _, _ := v.multipart("/_admin/construtor/arquivo", map[string]string{"_nep_csrf": "x"}, map[string][]byte{"arquivos[]": pngBytes()}); code == 200 {
		t.Error("upload sem login")
	}
}

// writeSite troca o arquivo do site e força a releitura.
func writeSite(t *testing.T, s *Server, src string) {
	t.Helper()
	if err := os.WriteFile(s.file, []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	later := time.Now().Add(2 * time.Second)
	os.Chtimes(s.file, later, later)
}

func TestCampoVazioNaoViraNil(t *testing.T) {
	c := newTCMenu(t)
	srv := c.srv.Config.Handler.(*Server)
	writeSite(t, srv, `<!doctype html><html><head></head><body>
<nep>let x = {}</nep>
<page route="/" title="I">[{{ visual(x.nada) }}|{{ markdown(x.nada) }}]</page>
</body></html>`)
	_, body, _ := anon(c).get("/")
	must(t, body, `[|]`)
}
