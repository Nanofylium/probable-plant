package web

import (
	"encoding/json"
	"io"
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

const siteCMS = `<!doctype html>
<html lang="pt-BR"><head><title>{{ page.title }}</title></head><body>
<conteudo nome="artigos" titulo="Artigos" item="artigo">
  <label>Título <input name="titulo" required maxlength="80"></label>
  <label>Resumo <textarea name="resumo"></textarea></label>
  <label>Texto <textarea name="corpo" data-editor></textarea></label>
  <label>Destaque <input type="checkbox" name="destaque" value="sim"></label>
  <label>Seção <select name="secao"><option>Geral</option><option>Esporte &amp; lazer</option></select></label>
  <label>Capa <input type="file" name="capa" accept="image/*" data-publico></label>
</conteudo>
<h1 editavel="titulo">Bem-vindo</h1>
<div editavel="sobre"><p>Texto <strong>padrão</strong></p></div>
<img editavel="foto" src="/padrao.jpg" alt="">
<page route="/" title="Início">
  <p class="n">{{ db.count("artigos") }} artigo(s)</p>
  <each a in="db.all('artigos')"><p class="a">{{ a.titulo }}</p></each>
  <p>{{ texto("aviso", "sem aviso") }}</p>
</page>
<page route="/artigos/:slug" conteudo="artigos">
  <h2>{{ item.titulo }}</h2>{{ markdown(item.corpo) }}
</page>
<page route="404" title="404">nada</page>
</body></html>`

func newTCCMS(t *testing.T) *tc {
	dir := t.TempDir()
	file := filepath.Join(dir, "site.nep.html")
	os.WriteFile(file, []byte(siteCMS), 0o644)
	s, err := New(Options{SiteFile: file, Log: io.Discard})
	if err != nil {
		t.Fatal(err)
	}
	ts := httptest.NewServer(s)
	t.Cleanup(ts.Close)
	jar, _ := cookiejar.New(nil)
	return &tc{t: t, srv: ts, dir: dir, c: &http.Client{Jar: jar, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}}
}

// loginAdmin faz a instalação do painel e deixa o cliente logado.
func loginAdmin(c *tc) string {
	tok := c.csrf("/_admin")
	b, err := os.ReadFile(filepath.Join(c.dir, "data", "SETUP_TOKEN.txt"))
	if err != nil {
		c.t.Fatal(err)
	}
	code, _, _ := c.post("/_admin", url.Values{"_nep_csrf": {tok}, "token": {strings.TrimSpace(string(b))}, "senha": {"senha-muito-boa"}, "senha2": {"senha-muito-boa"}})
	if code != 303 {
		c.t.Fatalf("instalação do painel: %d", code)
	}
	return tok
}

func anon(c *tc) *tc {
	jar, _ := cookiejar.New(nil)
	return &tc{t: c.t, srv: c.srv, dir: c.dir, c: &http.Client{Jar: jar, CheckRedirect: c.c.CheckRedirect}}
}

func TestMarkdown(t *testing.T) {
	got := Markdown("# Olá *mundo*\n\nTexto com **negrito** e [link](https://ex.com) e [ruim](javascript:alert(1)).\nOutra linha <script>x</script>\n\n- um\n- dois\n\n> citação\n\n```\n<b>código</b>\n```\n\n---\n![foto](/_nep/arquivo/p-x.png)")
	for _, w := range []string{
		"<h2>Olá <em>mundo</em></h2>",
		"<strong>negrito</strong>",
		`<a href="https://ex.com" rel="noopener">link</a>`,
		"e ruim",
		"<br>\nOutra linha &lt;script&gt;x&lt;/script&gt;",
		"<ul>\n<li>um</li>\n<li>dois</li>\n</ul>",
		"<blockquote><p>citação</p></blockquote>",
		"<pre><code>&lt;b&gt;código&lt;/b&gt;</code></pre>",
		"<hr>",
		`<img src="/_nep/arquivo/p-x.png" alt="foto" loading="lazy">`,
	} {
		if !strings.Contains(got, w) {
			t.Errorf("markdown sem %q:\n%s", w, got)
		}
	}
	if strings.Contains(got, "javascript") || strings.Contains(got, "<script>") {
		t.Errorf("markdown inseguro:\n%s", got)
	}
}

func TestSlug(t *testing.T) {
	for in, want := range map[string]string{"Pão de Queijo, a receita!": "pao-de-queijo-a-receita", "  Ação & Reação  ": "acao-reacao", "😀": ""} {
		if got := slugify(in); got != want {
			t.Errorf("slugify(%q) = %q, quero %q", in, got, want)
		}
	}
}

func TestConteudoNoPainel(t *testing.T) {
	c := newTCCMS(t)
	visitante := anon(c)
	loginAdmin(c)
	srv := c.srv.Config.Handler.(*Server)

	_, body, _ := c.get("/_admin")
	must(t, body, `/_admin/conteudo?c=artigos">Artigos</a>`, `/_admin/textos">Textos e menus</a>`)
	_, body, _ = c.get("/_admin/conteudo/editar?c=artigos")
	must(t, body, `name="titulo"`, `name="_slug"`, `name="_publicado" checked`, `enctype="multipart/form-data"`, "**negrito**")
	tok := csrfRe.FindStringSubmatch(body)[1]

	// validação vem das regras do próprio HTML
	code, body, _ := c.multipart("/_admin/conteudo/editar?c=artigos", map[string]string{"_nep_csrf": tok, "titulo": "", "_publicado": "on"}, nil)
	if code != 200 || !strings.Contains(body, "Preencha o campo") {
		t.Fatalf("validação: %d", code)
	}
	code, _, _ = c.multipart("/_admin/conteudo/editar?c=artigos", map[string]string{"_nep_csrf": tok, "titulo": "Feira de Sábado!", "corpo": "Tem **pastel**.", "destaque": "sim", "secao": "Esporte & lazer", "_publicado": "on"}, map[string][]byte{"capa": pngBytes()})
	if code != 303 {
		t.Fatalf("criar artigo: %d", code)
	}
	// mesmo título: endereço não repete
	c.multipart("/_admin/conteudo/editar?c=artigos", map[string]string{"_nep_csrf": tok, "titulo": "Feira de Sábado!", "corpo": "rascunho", "_publicado": ""}, nil)
	rs := srv.Store.All("artigos", 0)
	if len(rs) != 2 {
		t.Fatalf("artigos: %d", len(rs))
	}
	var pub, rasc = rs[1].ToValue(), rs[0].ToValue()
	if s, _ := pub.Get("slug"); s != "feira-de-sabado" {
		t.Errorf("slug: %v", s)
	}
	if s, _ := rasc.Get("slug"); s != "feira-de-sabado-2" {
		t.Errorf("slug repetido: %v", s)
	}

	// visitante: rascunho escondido em db.all, db.count e na página do item
	_, body, _ = visitante.get("/")
	must(t, body, `<p class="n">1 artigo(s)</p>`, "Feira de Sábado!")
	if code, _, _ := visitante.get("/artigos/feira-de-sabado-2"); code != 404 {
		t.Errorf("rascunho visível para visitante: %d", code)
	}
	code, body, _ = visitante.get("/artigos/feira-de-sabado")
	if code != 200 {
		t.Fatalf("página do item: %d", code)
	}
	must(t, body, "<title>Feira de Sábado!</title>", "<h2>Feira de Sábado!</h2>", "<p>Tem <strong>pastel</strong>.</p>",
		`<meta name="description" content="Tem pastel.">`, `property="og:image" content="`+c.srv.URL+`/_nep/arquivo/p-`)
	if code, _, _ := visitante.get("/artigos/nao-existe"); code != 404 {
		t.Errorf("item inexistente: %d", code)
	}
	// quem está no painel vê o rascunho (pré-visualização)
	if code, _, _ := c.get("/artigos/feira-de-sabado-2"); code != 200 {
		t.Errorf("admin deveria ver o rascunho: %d", code)
	}
	_, sm, _ := visitante.get("/sitemap.xml")
	if !strings.Contains(sm, "/artigos/feira-de-sabado</loc>") || strings.Contains(sm, "feira-de-sabado-2") {
		t.Errorf("sitemap: %s", sm)
	}

	// editar: valores voltam ao formulário (inclusive a caixa marcada) e trocar a capa apaga a antiga
	id := itoa(int(rs[1].ID))
	_, body, _ = c.get("/_admin/conteudo/editar?c=artigos&id=" + id)
	must(t, body, `value="Feira de Sábado!"`, `value="sim" checked`, `<option selected>Esporte &amp; lazer</option>`, "Tem **pastel**.", "Arquivo atual")
	oldFile := regexp.MustCompile(`p-[0-9a-f]{32}\.png`).FindString(rs[1].Data)
	code, _, _ = c.multipart("/_admin/conteudo/editar?c=artigos&id="+id, map[string]string{"_nep_csrf": tok, "titulo": "Feira de domingo", "_slug": "feira", "corpo": "x", "_publicado": "on"}, map[string][]byte{"capa": pngBytes()})
	if code != 303 {
		t.Fatalf("editar: %d", code)
	}
	if _, err := os.Stat(filepath.Join(c.dir, "data", "arquivos", oldFile)); !os.IsNotExist(err) {
		t.Error("a capa antiga ficou para trás")
	}
	if code, body, _ := visitante.get("/artigos/feira"); code != 200 || !strings.Contains(body, "Feira de domingo") {
		t.Errorf("endereço novo: %d", code)
	}
	_, body, _ = c.get("/_admin/conteudo?c=artigos")
	must(t, body, "Feira de domingo", "Rascunho", "Publicado", "/artigos/feira")
}

func TestTextosEditaveis(t *testing.T) {
	c := newTCCMS(t)
	visitante := anon(c)
	_, body, _ := visitante.get("/")
	must(t, body, `<h1 data-nep-ed="titulo" data-nep-tipo="texto">Bem-vindo</h1>`, `<div data-nep-ed="sobre" data-nep-tipo="rico"><p>Texto <strong>padrão</strong></p></div>`,
		`<img data-nep-ed="foto" data-nep-tipo="imagem" src="/padrao.jpg" alt="">`, "sem aviso")
	if strings.Contains(body, "editar.js") {
		t.Error("visitante não pode receber o editor")
	}
	tok := loginAdmin(c)
	_, body, _ = c.get("/")
	must(t, body, "/_nep/editar.js", `data-csrf="`)

	_, body, _ = c.get("/_admin/textos")
	must(t, body, `id="ed-titulo"`, ">Bem-vindo</textarea>", "Texto padrão</textarea>", `src="/padrao.jpg"`)
	tok = csrfRe.FindStringSubmatch(body)[1]

	// edição direto na página (fetch do editar.js)
	code, js, _ := c.post("/_admin/textos", url.Values{"_nep_csrf": {tok}, "chave": {"titulo"}, "valor": {"Olá <b>vizinho</b>\nsegunda linha"}, "ajax": {"1"}})
	var res map[string]string
	if code != 200 || json.Unmarshal([]byte(js), &res) != nil || res["html"] != "Olá &lt;b&gt;vizinho&lt;/b&gt;<br>\nsegunda linha" {
		t.Fatalf("ajax: %d %s", code, js)
	}
	c.post("/_admin/textos", url.Values{"_nep_csrf": {tok}, "chave": {"sobre"}, "valor": {"Somos **do bairro**."}})
	if code, _, _ := c.post("/_admin/textos", url.Values{"_nep_csrf": {"x"}, "chave": {"titulo"}, "valor": {"hack"}}); code != 400 {
		t.Errorf("sem csrf: %d", code)
	}
	code, _, _ = c.multipart("/_admin/textos", map[string]string{"_nep_csrf": tok, "chave": "foto"}, map[string][]byte{"imagem": pngBytes()})
	if code != 303 {
		t.Fatalf("trocar imagem: %d", code)
	}
	_, body, _ = visitante.get("/")
	must(t, body, `data-nep-tipo="texto">Olá &lt;b&gt;vizinho&lt;/b&gt;<br>`, `data-nep-tipo="rico"><p>Somos <strong>do bairro</strong>.</p></div>`, `data-nep-tipo="imagem" src="/_nep/arquivo/p-`)

	// voltar ao original
	c.post("/_admin/textos", url.Values{"_nep_csrf": {tok}, "chave": {"titulo"}, "restaurar": {"1"}})
	_, body, _ = visitante.get("/")
	must(t, body, `data-nep-tipo="texto">Bem-vindo</h1>`)
}

func TestConteudoErros(t *testing.T) {
	for src, want := range map[string]string{
		`<conteudo nome="x"><input name="slug"></conteudo>`:                                    `"slug" é criado sozinho`,
		`<conteudo nome="x"></conteudo>`:                                                       "não tem campos",
		`<page route="/a" conteudo="x">a</page><conteudo nome="x"><input name="t"></conteudo>`: "parte variável",
		`<page route="/a/:s" conteudo="y">a</page>`:                                            `não existe <conteudo nome="y">`,
		`<p editavel="Bad Key">x</p>`:                                                          "letras minúsculas",
		`<div editavel="a"><p editavel="b">x</p></div>`:                                        "não pode ficar dentro de outro",
		`<img editavel="a">`:                                                                   `precisa de src=`,
		`<p editavel="a">sem fim`:                                                              "<p editavel> aberta na linha 1 não foi fechada",
	} {
		_, err := ParseSite("t.nep.html", src)
		if err == nil || !strings.Contains(err.Error(), want) {
			t.Errorf("%s\n  erro: %v\n  quero: %s", src, err, want)
		}
	}
	// tags iguais aninhadas dentro do trecho editável
	s, err := ParseSite("t.nep.html", `<div editavel="a"><div>x</div><div>y</div></div><p>fim</p>`)
	if err != nil || s.Editable("a") == nil || s.Editable("a").Default != "<div>x</div><div>y</div>" {
		t.Fatalf("aninhado: %v %+v", err, s)
	}
}
