package web

import (
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const siteMenu = `<!doctype html>
<html><head><title>{{ page.title }}</title></head><body>
<conteudo nome="noticias"><input name="titulo" required></conteudo>
<nav menu-editavel="principal">
  <a class="item" href="{{ url('/') }}">Início</a>
  <a class="item" href="/contato">Contato</a>
  <each p in="[1]"><a href="/x">dinâmico</a></each>
</nav>
<page route="/" title="Início"><each m in="menu('principal')"><i>{{ m.texto }}={{ m.ativo }}</i></each></page>
<page route="/contato" title="Contato">c</page>
<page route="/pedidos" title="Pedidos" auth>pedidos secretos</page>
<page route="404" title="404">nada</page>
</body></html>`

func newTCMenu(t *testing.T) *tc {
	dir := t.TempDir()
	file := filepath.Join(dir, "site.nep.html")
	os.WriteFile(file, []byte(siteMenu), 0o644)
	s, err := New(Options{SiteFile: file, Log: io.Discard})
	if err != nil {
		t.Fatal(err)
	}
	ts := httptest.NewServer(s)
	t.Cleanup(ts.Close)
	jar, _ := cookiejar.New(nil)
	return &tc{t: t, srv: ts, dir: dir, c: &http.Client{Jar: jar, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}}
}

func TestMenuEditavel(t *testing.T) {
	c := newTCMenu(t)
	v := anon(c)
	_, body, _ := v.get("/contato")
	must(t, body, `<nav data-nep-ed="principal" data-nep-tipo="menu">`, `<a class="item" href="/">Início</a>`, "dinâmico")
	tok := loginAdmin(c)
	_, body, _ = c.get("/_admin/textos")
	must(t, body, "Início | /\nContato | /contato</textarea>", "Contato | /contato</code>")
	tok = csrfRe.FindStringSubmatch(body)[1]
	code, body, _ := c.post("/_admin/textos", url.Values{"_nep_csrf": {tok}, "chave": {"principal"}, "valor": {"Início | /\nRuim | javascript:alert(1)"}})
	if code != 200 || !strings.Contains(body, "Linha 2 do menu") {
		t.Fatalf("menu inválido aceito: %d", code)
	}
	c.post("/_admin/textos", url.Values{"_nep_csrf": {tok}, "chave": {"principal"}, "valor": {"Contato | /contato\nBlog <b> | https://blog.ex.com\nInício|/"}})
	_, body, _ = v.get("/contato")
	must(t, body, `<a class="item" href="/contato" aria-current="page">Contato</a>`, `<a class="item" href="https://blog.ex.com">Blog &lt;b&gt;</a>`, `<a class="item" href="/">Início</a></nav>`)
	if strings.Contains(body, "dinâmico") {
		t.Error("o conteúdo antigo do menu continuou")
	}
	_, body, _ = v.get("/")
	must(t, body, "<i>Contato=false</i><i>Blog &lt;b&gt;=false</i><i>Início=true</i>")
}

func TestUsuariosEditores(t *testing.T) {
	c := newTCMenu(t)
	tok := loginAdmin(c)
	_, body, _ := c.get("/_admin/usuarios")
	tok = csrfRe.FindStringSubmatch(body)[1]
	code, body, _ := c.post("/_admin/usuarios", url.Values{"_nep_csrf": {tok}, "acao": {"criar"}, "usuario": {"maria"}, "nome": {"Maria"}, "senha": {"curta"}})
	if code != 200 || !strings.Contains(body, "pelo menos 10") {
		t.Fatalf("senha curta: %d", code)
	}
	if code, _, _ := c.post("/_admin/usuarios", url.Values{"_nep_csrf": {tok}, "acao": {"criar"}, "usuario": {"maria"}, "nome": {"Maria"}, "senha": {"senha-da-maria"}}); code != 303 {
		t.Fatalf("criar editor: %d", code)
	}

	ed := anon(c)
	etok := ed.csrf("/_admin/login")
	if code, body, _ := ed.post("/_admin/login", url.Values{"_nep_csrf": {etok}, "usuario": {"maria"}, "senha": {"senha-do-dono"}}); code != 200 || !strings.Contains(body, "incorretos") {
		t.Fatalf("senha errada do editor: %d", code)
	}
	if code, _, _ := ed.post("/_admin/login", url.Values{"_nep_csrf": {etok}, "usuario": {"Maria "}, "senha": {"senha-da-maria"}}); code != 303 {
		t.Fatalf("login do editor: %d", code)
	}
	if code, _, h := ed.get("/_admin"); code != 302 || !strings.HasSuffix(h.Get("Location"), "/_admin/conteudo?c=noticias") {
		t.Errorf("início do editor: %d %s", code, h.Get("Location"))
	}
	_, body, _ = ed.get("/_admin/textos")
	must(t, body, "Textos e menus", `name="chave" value="principal"`)
	if strings.Contains(body, "LGPD") || strings.Contains(body, "Usuários") {
		t.Error("editor vê links do dono")
	}
	for _, p := range []string{"/_admin/lgpd", "/_admin/csv?c=noticias", "/_admin/visitas", "/_admin/usuarios"} {
		if _, body, _ := ed.get(p); !strings.Contains(body, "só para o dono") {
			t.Errorf("editor abriu %s", p)
		}
	}
	if code, body, _ := ed.get("/_admin?c=contatos"); code != 302 || strings.Contains(body, "a@b.com") {
		t.Errorf("editor viu coleção: %d", code)
	}
	if code, _, _ := ed.get("/pedidos"); code != 403 {
		t.Errorf("editor abriu página auth: %d", code)
	}
	if code, _, _ := c.get("/pedidos"); code != 200 {
		t.Errorf("dono não abriu página auth: %d", code)
	}
	// editor publica conteúdo
	_, body, _ = ed.get("/_admin/conteudo/editar?c=noticias")
	etok = csrfRe.FindStringSubmatch(body)[1]
	if code, _, _ := ed.multipart("/_admin/conteudo/editar?c=noticias", map[string]string{"_nep_csrf": etok, "titulo": "Oi", "_publicado": "on"}, nil); code != 303 {
		t.Errorf("editor criando conteúdo: %d", code)
	}
	// apagar só em coleções de conteúdo
	srv := c.srv.Config.Handler.(*Server)
	id, _ := srv.Store.Insert("contatos", mkTable(map[string]any{"email": "a@b.com"}))
	ed.post("/_admin/delete", url.Values{"_nep_csrf": {etok}, "c": {"contatos"}, "id": {itoa(int(id))}})
	if srv.Store.Count("contatos") != 1 {
		t.Error("editor apagou dados de formulário")
	}
	// remover o editor derruba a sessão dele
	c.post("/_admin/usuarios", url.Values{"_nep_csrf": {tok}, "acao": {"remover"}, "usuario": {"maria"}})
	if code, _, h := ed.get("/_admin/textos"); code != 302 || !strings.Contains(h.Get("Location"), "/login") {
		t.Errorf("editor removido continuou logado: %d", code)
	}
}
