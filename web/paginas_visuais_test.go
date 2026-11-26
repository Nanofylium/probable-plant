package web

import (
	"encoding/json"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

const sitePV = `<!doctype html>
<html lang="pt-BR"><head><title>{{ page.title }}</title><style>.marca{color:green}</style></head><body>
<nav><a href="{{ url('/') }}">Início</a></nav>
<page route="/" title="Início">
  <section class="capa">
    <h1>Bem-vindo à Padaria</h1>
    <p>Pão quentinho <strong>todo dia</strong>. Veja o <a href="{{ url('/contato') }}">contato</a>.</p>
    <p class="n">Temos {{ db.count("contatos") }} recados.</p>
  </section>
  <each c in="db.all('contatos')"><p class="recado">{{ c.nome }}</p></each>
  <form save="contatos" success="ok"><label>Nome <input name="nome" required></label><button>Enviar</button></form>
  <svg viewBox="0 0 10 10"><circle r="4" cx="5" cy="5"/></svg>
</page>
<page route="/contato" title="Contato"><h1>Contato</h1></page>
<page route="/blog/:slug" title="Post"><h1>{{ params.slug }}</h1></page>
<page route="404" title="Não achei"><p>nada</p></page>
</body></html>`

func TestDecomporRecompor(t *testing.T) {
	s := &Server{}
	src := sitePV[strings.Index(sitePV, `<page route="/" title="Início">`)+len(`<page route="/" title="Início">`) : strings.Index(sitePV, "</page>")]
	h, ilhas := s.Decompor(src)
	for _, want := range []string{`<h1>Bem-vindo à Padaria</h1>`, `href="/contato"`, `<strong>todo dia</strong>`, `<p class="n">Temos <span data-nep-ilha="0"></span> recados.</p>`} {
		if !strings.Contains(h, want) {
			t.Errorf("decompor sem %s:\n%s", want, h)
		}
	}
	if len(ilhas) != 4 || !strings.HasPrefix(ilhas[1], "<each") || !strings.HasPrefix(ilhas[2], "<form") || !strings.HasPrefix(ilhas[3], "<svg") {
		t.Fatalf("ilhas: %q", ilhas)
	}
	// o construtor devolve o HTML mexido: texto novo, ilhas trocadas de lugar, uma apagada
	novo := strings.Replace(h, "Bem-vindo à Padaria", "Bem-vindo à Padaria {{ segredo }} <script>alert(1)</script>", 1)
	novo = strings.Replace(novo, `<div data-nep-ilha="3"></div>`, "", 1)
	novo = `<div data-nep-ilha="2"></div>` + strings.Replace(novo, `<div data-nep-ilha="2"></div>`, "", 1) + `<div data-nep-novo="contato"></div><p onclick="x()" data-x="1" aria-label="a">fim</p>`
	f, err := s.Recompor(novo, "body{margin:0} .capa h1{color:red;background:url(javascript:x)} #i9{padding:4px}", ilhas)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"&#123;&#123; segredo }}", `href="{{ url('/contato') }}"`, `<each c in="db.all('contatos')">`, `save="contatos" avisar="dono"`, `<p aria-label="a" data-x="1">fim</p>`, `#i9{padding:4px}`} {
		if !strings.Contains(f, want) {
			t.Errorf("recompor sem %s:\n%s", want, f)
		}
	}
	for _, nao := range []string{"<script", "onclick", "<svg", "javascript", "body{"} {
		if strings.Contains(f, nao) {
			t.Errorf("recompor deixou %s:\n%s", nao, f)
		}
	}
	if strings.Index(f, "<form save=\"contatos\" success") > strings.Index(f, "<h1>") {
		t.Error("a ilha movida devia vir antes")
	}
	if _, err := s.Recompor(`<div data-nep-ilha="9"></div>`, "", ilhas); err == nil {
		t.Error("ilha inexistente devia falhar")
	}
	if _, err := ParseSite("x", "<page route=\"/\">"+f+"</page>"); err != nil {
		t.Fatalf("a página recomposta não compila: %v\n%s", err, f)
	}
}

func TestPaginasVisuaisNoPainel(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "site.nep.html")
	os.WriteFile(file, []byte(sitePV), 0o644)
	s, err := New(Options{SiteFile: file, Log: os.Stderr})
	if err != nil {
		t.Fatal(err)
	}
	ts := httptest.NewServer(s)
	defer ts.Close()
	jar, _ := cookiejar.New(nil)
	c := &tc{t: t, srv: ts, dir: dir, c: &http.Client{Jar: jar, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}}
	tok := loginAdmin(c)

	_, lista, _ := c.get("/_admin/paginas")
	if !strings.Contains(lista, "paginas/editar?r=%2Fcontato") || strings.Contains(lista, "blog") {
		t.Fatalf("lista de páginas: %s", lista)
	}
	code, tela, _ := c.get("/_admin/paginas/editar?r=%2F")
	m := regexp.MustCompile(`(?s)<script type="application/json" id="pv-dados">(.*?)</script>`).FindStringSubmatch(tela)
	if code != 200 || m == nil {
		t.Fatalf("editor: %d %.300s", code, tela)
	}
	var d struct {
		HTML    string            `json:"html"`
		Previas map[string]string `json:"previas"`
		Estilos json.RawMessage   `json:"estilos"`
		CSRF    string            `json:"csrf"`
	}
	if err := json.Unmarshal([]byte(m[1]), &d); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(d.Previas["0"], "0") || !strings.Contains(d.Previas["2"], "<form") || !strings.Contains(d.Previas["2"], "_nep_csrf") || !strings.Contains(d.Previas["contato"], "Mensagem") || !strings.Contains(string(d.Estilos), ".marca") {
		t.Fatalf("prévias: %v estilos=%s", d.Previas, d.Estilos)
	}
	tok = d.CSRF
	// salva com o título trocado e um formulário de contato novo
	h := strings.Replace(d.HTML, "Bem-vindo à Padaria", "Padaria do Zé", 1) + `<div data-nep-novo="contato"></div>`
	code, body, _ := c.post("/_admin/paginas/editar", url.Values{"_nep_csrf": {tok}, "r": {"/"}, "html": {h}, "css": {".capa h1{color:#b00}"}})
	if code != 200 || !strings.Contains(body, `"ok":true`) {
		t.Fatalf("salvar: %d %s", code, body)
	}
	_, home, _ := c.get("/")
	if !strings.Contains(home, "Padaria do Zé") || !strings.Contains(home, "Temos 0 recados") || !strings.Contains(home, `name="mensagem"`) || !strings.Contains(home, ".capa h1{color:#b00}") {
		t.Fatalf("home depois de salvar: %s", home)
	}
	if b, _ := os.ReadFile(file); string(b) != sitePV {
		t.Fatal("o arquivo do site não pode mudar")
	}
	// o formulário novo funciona
	vis := anon(c)
	vt := vis.csrf("/")
	_, home2, _ := vis.get("/")
	fm := regexp.MustCompile(`name="_nep_form" value="([^"]+)"`).FindAllStringSubmatch(home2, -1)
	if len(fm) < 2 {
		t.Fatalf("formulários: %v", fm)
	}
	code, _, _ = vis.post("/", url.Values{"_nep_csrf": {vt}, "_nep_form": {fm[len(fm)-1][1]}, "nome": {"Ana"}, "email": {"ana@ex.com"}, "mensagem": {"Oi"}})
	if code != 303 {
		t.Fatalf("enviar o formulário novo: %d", code)
	}
	// página nova e voltar ao original
	code, _, hd := c.post("/_admin/paginas", url.Values{"_nep_csrf": {tok}, "acao": {"nova"}, "titulo": {"Quem somos"}, "endereco": {"Quem Somos"}})
	if code != 303 || !strings.Contains(hd.Get("Location"), "r=%2Fquem-somos") {
		t.Fatalf("página nova: %d %v", code, hd)
	}
	if code, qs, _ := c.get("/quem-somos"); code != 200 || !strings.Contains(qs, "Quem somos") {
		t.Fatalf("/quem-somos: %d", code)
	}
	c.post("/_admin/paginas", url.Values{"_nep_csrf": {tok}, "acao": {"original"}, "r": {"/"}})
	if _, home, _ := c.get("/"); !strings.Contains(home, "Bem-vindo à Padaria") {
		t.Fatal("devia voltar ao original")
	}
	// editor de outro usuário sem login: nada
	if code, _, _ := vis.get("/_admin/paginas/editar?r=%2F"); code != 302 {
		t.Fatalf("sem login: %d", code)
	}
}
