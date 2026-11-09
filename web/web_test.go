package web

import (
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"github.com/Nanofylium/probable-plant/lang"
)

const testSite = `<!doctype html>
<nep-config movimento="true">
<data name="itens">[{"nome": "Pão", "preco": 6.5}, {"nome": "Bolo <b>", "preco": 30}]</data>
<nep>
  let titulo = "Loja"
  fn validar(d) do
    if d.nome == "proibido" do recusar("Nome não permitido.") end
    db.insert("pedidos", {nome: upper(d.nome)})
    return "/obrigado"
  end
</nep>
<api route="/api/itens">
  return itens |> map(fn(i) do i.nome end)
</api>
<api route="/api/item/:n" method="GET">
  let i = int(params.n)
  if i >= len(itens) do abort(404, "sem item") end
  return itens[i]
</api>
<html><head><title>{{ page.title }} · {{ titulo }}</title></head><body>
<page route="/" title="Início">
  <ul><each i in="itens"><li>{{ loop.index }}:{{ i.nome }}:{{ i.preco |> money }}</li></each></ul>
  <each x in="[]">nunca<else><p>vazio</p></each>
  <if test="len(itens) > 1"><p>muitos</p><else><p>poucos</p></if>
  <p>{{ raw("<em>ok</em>") }}</p>
</page>
<page route="/item/:n" title="Item">
  <nep>
    let item = itens[int(params.n)]
    if not item do not_found() end
    page.title = item.nome
  </nep>
  <h1>{{ item.nome }}</h1>
</page>
<page route="/contato" title="Contato">
  <form save="contatos" success="Valeu!" limit="3">
    <label>Nome <input name="nome" required maxlength="10"></label>
    <label>E-mail <input name="email" type="email" required></label>
    <label>Idade <input name="idade" type="number" min="18" max="99"></label>
    <select name="cor"><option>azul</option><option value="vd">verde</option></select>
    <button>Enviar</button>
  </form>
</page>
<page route="/pedido" title="Pedido">
  <form run="validar" success="">
    <input name="nome" required>
  </form>
</page>
<page route="/obrigado" title="Obrigado"><p>obrigado</p></page>
<page route="/ao-vivo" title="Vivo">
  <live><b>{{ db.count("contatos") }}</b></live>
</page>
<page route="/laco" title="Laço"><nep>while true do end</nep></page>
<page route="/erro" title="Erro">
  <p>
    {{ nada.campo }}
  </p>
</page>
<page route="/restrita" title="R" auth><p>segredo</p></page>
<page route="404" title="Não achei"><h1>404 personalizado</h1></page>
</body></html>
`

type tc struct {
	t   *testing.T
	srv *httptest.Server
	c   *http.Client
	dir string
}

func newTC(t *testing.T, dev bool) *tc {
	t.Helper()
	dir := t.TempDir()
	file := filepath.Join(dir, "site.nep.html")
	if err := os.WriteFile(file, []byte(testSite), 0o644); err != nil {
		t.Fatal(err)
	}
	os.WriteFile(filepath.Join(dir, "estilo.css"), []byte("body{}"), 0o644)
	s, err := New(Options{SiteFile: file, Dev: dev, Log: io.Discard})
	if err != nil {
		t.Fatal(err)
	}
	ts := httptest.NewServer(s)
	t.Cleanup(ts.Close)
	jar, _ := cookiejar.New(nil)
	return &tc{t: t, srv: ts, dir: dir, c: &http.Client{Jar: jar, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}}
}

func (c *tc) get(path string) (int, string, http.Header) {
	c.t.Helper()
	r, err := c.c.Get(c.srv.URL + path)
	if err != nil {
		c.t.Fatal(err)
	}
	defer r.Body.Close()
	b, _ := io.ReadAll(r.Body)
	return r.StatusCode, string(b), r.Header
}

func (c *tc) post(path string, v url.Values) (int, string, http.Header) {
	c.t.Helper()
	r, err := c.c.PostForm(c.srv.URL+path, v)
	if err != nil {
		c.t.Fatal(err)
	}
	defer r.Body.Close()
	b, _ := io.ReadAll(r.Body)
	return r.StatusCode, string(b), r.Header
}

var csrfRe = regexp.MustCompile(`name="_nep_csrf" value="([0-9a-f]+)"`)

func (c *tc) csrf(path string) string {
	_, body, _ := c.get(path)
	m := csrfRe.FindStringSubmatch(body)
	if m == nil {
		c.t.Fatalf("sem token CSRF em %s", path)
	}
	return m[1]
}

func must(t *testing.T, body string, want ...string) {
	t.Helper()
	for _, w := range want {
		if !strings.Contains(body, w) {
			t.Errorf("faltou %q em:\n%s", w, body)
		}
	}
}

func TestPaginas(t *testing.T) {
	c := newTC(t, false)
	code, body, h := c.get("/")
	if code != 200 {
		t.Fatalf("status %d", code)
	}
	must(t, body, "<title>Início · Loja</title>", "1:Pão:R$ 6,50", "2:Bolo &lt;b&gt;:R$ 30,00", "<p>vazio</p>", "<p>muitos</p>", "<em>ok</em>", "/_nep/movimento.js")
	if strings.Contains(body, "nunca") || strings.Contains(body, "poucos") {
		t.Error("each/else ou if/else errado")
	}
	if h.Get("Content-Security-Policy") == "" || h.Get("X-Frame-Options") != "DENY" {
		t.Error("faltam cabeçalhos de segurança")
	}
	_, body, _ = c.get("/item/1")
	must(t, body, "<title>Bolo &lt;b&gt; · Loja</title>", "<h1>Bolo &lt;b&gt;</h1>")
	code, body, _ = c.get("/item/9")
	if code != 404 || !strings.Contains(body, "404 personalizado") {
		t.Errorf("not_found(): %d %s", code, body)
	}
	code, _, _ = c.get("/nao/existe")
	if code != 404 {
		t.Errorf("rota inexistente: %d", code)
	}
}

func TestEstaticos(t *testing.T) {
	c := newTC(t, false)
	if code, _, _ := c.get("/estilo.css"); code != 200 {
		t.Errorf("css: %d", code)
	}
	for _, p := range []string{"/site.nep.html", "/data/nep.db", "/data/.htaccess", "/../site.nep.html"} {
		if code, body, _ := c.get(p); code == 200 && !strings.Contains(body, "404 personalizado") {
			t.Errorf("%s deveria estar protegido", p)
		}
	}
}

func TestFormulario(t *testing.T) {
	c := newTC(t, false)
	tok := c.csrf("/contato")
	base := url.Values{"_nep_form": {"0"}, "nome": {"Ana"}, "email": {"ana@ex.com"}, "idade": {"30"}, "cor": {"vd"}}

	v := cloneV(base)
	code, body, _ := c.post("/contato", v)
	if code != 422 || !strings.Contains(body, "sessão expirou") {
		t.Errorf("sem CSRF deveria falhar: %d", code)
	}

	v = cloneV(base)
	v.Set("_nep_csrf", tok)
	v.Set("email", "nao-e-email")
	v.Set("idade", "12")
	v.Set("cor", "roxo")
	code, body, _ = c.post("/contato", v)
	if code != 422 {
		t.Fatalf("validação: %d", code)
	}
	must(t, body, "e-mail válido", "no mínimo 18", "Opção inválida", `value="Ana"`)

	v = cloneV(base)
	v.Set("_nep_csrf", tok)
	v.Set("email", "")
	_, body, _ = c.post("/contato", v)
	must(t, body, "Preencha o campo", `<option value="vd" selected>`, `value="30"`)

	v = cloneV(base)
	v.Set("_nep_csrf", tok)
	v.Set("admin", "true") // campo que não existe no formulário: ignorado
	code, _, h := c.post("/contato", v)
	if code != 303 {
		t.Fatalf("envio válido: %d", code)
	}
	_, body, _ = c.get(h.Get("Location"))
	must(t, body, `class="nep-ok"`, "Valeu!")

	srv := c.srv.Config.Handler.(*Server)
	rs := srv.Store.All("contatos", 0)
	if len(rs) != 1 {
		t.Fatalf("registros: %d", len(rs))
	}
	if strings.Contains(rs[0].Data, "admin") || !strings.Contains(rs[0].Data, `"idade":30`) {
		t.Errorf("dados salvos: %s", rs[0].Data)
	}

	// limite de envios (limit="3")
	for i := 0; i < 3; i++ {
		v = cloneV(base)
		v.Set("_nep_csrf", tok)
		c.post("/contato", v)
	}
	if n := srv.Store.Count("contatos"); n != 3 {
		t.Errorf("limite: %d registros, esperava 3", n)
	}

	// armadilha anti-robô: finge sucesso e não salva
	v = cloneV(base)
	v.Set("_nep_csrf", tok)
	v.Set("_nep_hp", "spam")
	c2 := newTC(t, false)
	tok2 := c2.csrf("/contato")
	v.Set("_nep_csrf", tok2)
	if code, _, _ := c2.post("/contato", v); code != 303 {
		t.Errorf("honeypot: %d", code)
	}
	if n := c2.srv.Config.Handler.(*Server).Store.Count("contatos"); n != 0 {
		t.Error("honeypot salvou")
	}
}

func cloneV(v url.Values) url.Values {
	o := url.Values{}
	for k, x := range v {
		o[k] = append([]string(nil), x...)
	}
	return o
}

func TestFormularioRun(t *testing.T) {
	c := newTC(t, false)
	tok := c.csrf("/pedido")
	code, body, _ := c.post("/pedido", url.Values{"_nep_form": {"1"}, "_nep_csrf": {tok}, "nome": {"proibido"}})
	if code != 422 || !strings.Contains(body, "Nome não permitido.") {
		t.Errorf("recusar(): %d", code)
	}
	code, _, h := c.post("/pedido", url.Values{"_nep_form": {"1"}, "_nep_csrf": {tok}, "nome": {"ana"}})
	if code != 303 || h.Get("Location") != "/obrigado" {
		t.Errorf("run=: %d %s", code, h.Get("Location"))
	}
	rs := c.srv.Config.Handler.(*Server).Store.All("pedidos", 0)
	if len(rs) != 1 || !strings.Contains(rs[0].Data, "ANA") {
		t.Errorf("pedido salvo pela função: %v", rs)
	}
	// formulário de outra página não é aceito aqui
	code, _, _ = c.post("/pedido", url.Values{"_nep_form": {"0"}, "_nep_csrf": {tok}})
	if code != 400 {
		t.Errorf("form de outra página: %d", code)
	}
}

func TestAPI(t *testing.T) {
	c := newTC(t, false)
	code, body, h := c.get("/api/itens")
	if code != 200 || !strings.HasPrefix(body, `["Pão","Bolo `) || !strings.HasPrefix(h.Get("Content-Type"), "application/json") {
		t.Errorf("api: %d %s", code, body)
	}
	code, body, _ = c.get("/api/item/5")
	if code != 404 || !strings.Contains(body, "sem item") {
		t.Errorf("abort: %d %s", code, body)
	}
}

func TestLive(t *testing.T) {
	c := newTC(t, false)
	_, body, _ := c.get("/ao-vivo")
	must(t, body, `<div nep-live="0"><b>0</b></div>`, "/_nep/vivo.js")
	code, frag, _ := c.get("/ao-vivo?_nep_live=0")
	if code != 200 || frag != "<b>0</b>" {
		t.Errorf("fragmento: %d %q", code, frag)
	}
	_, v1, _ := c.get("/_nep/versao")
	c.c.Get(c.srv.URL) // nada muda
	_, v2, _ := c.get("/_nep/versao")
	if v1 != v2 {
		t.Error("versão mudou sem escrita")
	}
}

func TestLimiteDeExecucao(t *testing.T) {
	c := newTC(t, false)
	code, body, _ := c.get("/laco")
	if code != 500 || strings.Contains(body, "while") {
		t.Errorf("laço infinito: %d", code)
	}
}

func TestErroComLinha(t *testing.T) {
	c := newTC(t, true)
	code, body, _ := c.get("/erro")
	if code != 500 {
		t.Fatalf("status %d", code)
	}
	lines := strings.Split(testSite, "\n")
	want := 0
	for i, l := range lines {
		if strings.Contains(l, "nada.campo") {
			want = i + 1
		}
	}
	must(t, body, "linha "+itoa(want), "nada")
	// em produção, nada de detalhes
	c2 := newTC(t, false)
	_, body, _ = c2.get("/erro")
	if strings.Contains(body, "nada") {
		t.Error("produção mostrou detalhes do erro")
	}
}

func itoa(n int) string { return strconv.Itoa(n) }

func mkTable(m map[string]any) *lang.Table {
	t := lang.NewTable()
	for k, v := range m {
		t.SetStr(k, v)
	}
	return t
}

func TestPainel(t *testing.T) {
	c := newTC(t, false)
	if code, _, h := c.get("/restrita"); code != 302 || !strings.Contains(h.Get("Location"), "/_admin/login") {
		t.Fatalf("página restrita sem login: %d", code)
	}
	tok := c.csrf("/_admin")
	b, err := os.ReadFile(filepath.Join(c.dir, "data", "SETUP_TOKEN.txt"))
	if err != nil {
		t.Fatal(err)
	}
	code, body, _ := c.post("/_admin", url.Values{"_nep_csrf": {tok}, "token": {"errado"}, "senha": {"senha-muito-boa"}, "senha2": {"senha-muito-boa"}})
	if code != 200 || !strings.Contains(body, "incorreto") {
		t.Errorf("token errado: %d", code)
	}
	code, _, _ = c.post("/_admin", url.Values{"_nep_csrf": {tok}, "token": {strings.TrimSpace(string(b))}, "senha": {"senha-muito-boa"}, "senha2": {"senha-muito-boa"}})
	if code != 303 {
		t.Fatalf("setup: %d", code)
	}
	if _, err := os.Stat(filepath.Join(c.dir, "data", "SETUP_TOKEN.txt")); err == nil {
		t.Error("o código de instalação deveria ser apagado")
	}
	if code, body, _ := c.get("/restrita"); code != 200 || !strings.Contains(body, "segredo") {
		t.Errorf("restrita logado: %d", code)
	}
	srv := c.srv.Config.Handler.(*Server)
	tb := mkTable(map[string]any{"nome": "=HYPERLINK(1)", "email": "maria@ex.com"})
	srv.Store.Insert("contatos", tb)
	_, csv, h := c.get("/_admin/csv?c=contatos")
	if !strings.HasPrefix(h.Get("Content-Type"), "text/csv") || !strings.Contains(csv, "'=HYPERLINK(1)") || !strings.HasPrefix(csv, "\ufeff") {
		t.Errorf("csv: %q", csv)
	}
	_, body, _ = c.get("/_admin/lgpd?q=maria@ex")
	must(t, body, "1 registro(s) encontrado(s)")
	tok = csrfRe.FindStringSubmatch(body)[1]
	c.post("/_admin/lgpd", url.Values{"_nep_csrf": {tok}, "q": {"maria@ex"}, "acao": {"apagar"}})
	if n := srv.Store.Count("contatos"); n != 0 {
		t.Errorf("lgpd não apagou: %d", n)
	}

	// login com outra sessão
	c2 := &tc{t: t, srv: c.srv, dir: c.dir}
	jar, _ := cookiejar.New(nil)
	c2.c = &http.Client{Jar: jar, CheckRedirect: c.c.CheckRedirect}
	tok = c2.csrf("/_admin/login")
	code, body, _ = c2.post("/_admin/login", url.Values{"_nep_csrf": {tok}, "senha": {"errada"}})
	if !strings.Contains(body, "Senha incorreta") {
		t.Errorf("senha errada: %d", code)
	}
	code, _, _ = c2.post("/_admin/login", url.Values{"_nep_csrf": {tok}, "senha": {"senha-muito-boa"}})
	if code != 303 {
		t.Errorf("login: %d", code)
	}
}

func TestCheck(t *testing.T) {
	dir := t.TempDir()
	f := filepath.Join(dir, "s.nep.html")
	os.WriteFile(f, []byte("<p>\n<if test=\"x\">\n</p>"), 0o644)
	if _, err := Check(f); err == nil || !strings.Contains(err.Error(), ":2:") {
		t.Errorf("tag aberta: %v", err)
	}
	os.WriteFile(f, []byte("<p>\n\n{{ 1 + }}</p>"), 0o644)
	if _, err := Check(f); err == nil || !strings.Contains(err.Error(), ":3") {
		t.Errorf("erro de sintaxe na linha 3: %v", err)
	}
}

func TestSenha(t *testing.T) {
	h := hashPassword("abc12345678")
	if !checkPassword("abc12345678", h) || checkPassword("abc1234567", h) {
		t.Error("pbkdf2")
	}
}
