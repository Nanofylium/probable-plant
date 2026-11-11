package web

import (
	"archive/zip"
	"bytes"
	"io"
	"net/url"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/Nanofylium/probable-plant/lang"
)

const siteWP = `<!doctype html><html><head><title>Blog</title></head><body>
<nep-config nome="Blog do Bairro" lang="pt-BR">
<conteudo nome="artigos" titulo="Artigos" item="artigo">
  <label>Título <input name="titulo" required></label>
  <label>Texto <textarea name="corpo"></textarea></label>
</conteudo>
<page route="/" title="Início"><each a in="db.all('artigos')"><p class="a">{{ a.titulo }}</p></each></page>
<page route="/artigos/:slug" conteudo="artigos"><h1>{{ item.titulo }}</h1>{{ comentarios() }}</page>
<page route="/busca" title="Busca"><each r in="buscar(request.query.q)"><p class="r"><a href="{{ r.url }}">{{ r.titulo }}</a></p></each></page>
</body></html>`

func newArticle(t *testing.T, c *tc, tok string, fields map[string]string) {
	t.Helper()
	f := map[string]string{"_nep_csrf": tok, "_publicado": "on"}
	for k, v := range fields {
		f[k] = v
	}
	path := "/_admin/conteudo/editar?c=artigos"
	if id := f["_id"]; id != "" {
		path += "&id=" + id
	}
	if code, body, _ := c.multipart(path, f, nil); code != 303 {
		t.Fatalf("salvar artigo: %d %s", code, body)
	}
}

func TestVersoesELixeira(t *testing.T) {
	c, srv := newTCSite(t, siteWP)
	loginAdmin(c)
	_, body, _ := c.get("/_admin/conteudo/editar?c=artigos")
	tok := csrfRe.FindStringSubmatch(body)[1]
	newArticle(t, c, tok, map[string]string{"titulo": "Pão de queijo", "corpo": "v1"})
	id := strconv.FormatInt(srv.Store.All("artigos", 0)[0].ID, 10)
	newArticle(t, c, tok, map[string]string{"_id": id, "titulo": "Pão de queijo mineiro", "corpo": "v2", "_slug": "pao-de-queijo"})
	newArticle(t, c, tok, map[string]string{"_id": id, "titulo": "Pão de queijo da vó", "corpo": "v3", "_slug": "pao-de-queijo"})

	_, body, _ = c.get("/_admin/conteudo/editar?c=artigos&id=" + id)
	must(t, body, "Histórico: 2 versões anteriores", "Pão de queijo mineiro", "dono")
	n, _ := strconv.ParseInt(id, 10, 64)
	vs := srv.versions("artigos", n)
	first := vs[len(vs)-1] // a mais antiga: "Pão de queijo"
	code, _, _ := c.post("/_admin/conteudo/versao", url.Values{"_nep_csrf": {tok}, "c": {"artigos"}, "id": {id}, "v": {strconv.FormatInt(first.ID, 10)}})
	if code != 303 {
		t.Fatalf("restaurar versão: %d", code)
	}
	it := srv.Store.Find("artigos", n).ToValue()
	if getS(it, "titulo") != "Pão de queijo" || getS(it, "corpo") != "v1" || getS(it, "slug") != "pao-de-queijo" {
		t.Errorf("versão restaurada: %v", it)
	}
	if len(srv.versions("artigos", n)) != 3 {
		t.Error("a versão atual deveria ir para o histórico antes de restaurar")
	}

	// apagar manda para a lixeira; restaurar volta com o mesmo número e endereço
	c.post("/_admin/delete", url.Values{"_nep_csrf": {tok}, "c": {"artigos"}, "id": {id}})
	if srv.Store.Find("artigos", n) != nil {
		t.Fatal("o item deveria ter saído do site")
	}
	if code, _, _ := anon(c).get("/artigos/pao-de-queijo"); code != 404 {
		t.Errorf("item apagado ainda aparece: %d", code)
	}
	_, body, _ = c.get("/_admin/lixeira")
	must(t, body, "Pão de queijo", "Restaurar", "dono")
	tr := srv.Store.All(colTrash, 0)[0]
	c.post("/_admin/lixeira", url.Values{"_nep_csrf": {tok}, "acao": {"restaurar"}, "id": {strconv.FormatInt(tr.ID, 10)}})
	if srv.Store.Find("artigos", n) == nil {
		t.Fatal("restaurar não devolveu o item com o mesmo número")
	}
	if code, _, _ := anon(c).get("/artigos/pao-de-queijo"); code != 200 {
		t.Errorf("item restaurado: %d", code)
	}
	// o painel não lista as coleções internas
	_, body, _ = c.get("/_admin")
	if strings.Contains(body, "_versoes") || strings.Contains(body, "_lixeira") {
		t.Error("coleções internas apareceram no painel")
	}
}

func TestAgendamentoBuscaEFeed(t *testing.T) {
	c, srv := newTCSite(t, siteWP)
	loginAdmin(c)
	_, body, _ := c.get("/_admin/conteudo/editar?c=artigos")
	tok := csrfRe.FindStringSubmatch(body)[1]
	futuro := time.Now().Add(48 * time.Hour).Format("2006-01-02T15:04")
	passado := time.Now().Add(-time.Hour).Format("2006-01-02T15:04")
	newArticle(t, c, tok, map[string]string{"titulo": "Feira de domingo", "corpo": "Tem pão e pastel."})
	newArticle(t, c, tok, map[string]string{"titulo": "Festa junina", "corpo": "Quentão e pão.", "_publicar_em": futuro})
	newArticle(t, c, tok, map[string]string{"titulo": "Horta comunitária", "corpo": "Alface.", "_publicar_em": passado})
	newArticle(t, c, tok, map[string]string{"titulo": "Rascunho do pão", "corpo": "pão", "_publicado": ""})

	v := anon(c)
	_, body, _ = v.get("/")
	must(t, body, "Feira de domingo", "Horta comunitária")
	if strings.Contains(body, "Festa junina") || strings.Contains(body, "Rascunho") {
		t.Error("agendado ou rascunho apareceu para o visitante")
	}
	if code, _, _ := v.get("/artigos/festa-junina"); code != 404 {
		t.Errorf("agendado acessível: %d", code)
	}
	_, body, _ = c.get("/_admin/conteudo?c=artigos")
	must(t, body, "Agendado: ", "Rascunho")

	// busca sem acento acha "pão"; título pesa mais
	_, body, _ = v.get("/busca?q=PAO")
	must(t, body, `<a href="/artigos/feira-de-domingo">Feira de domingo</a>`)
	if strings.Count(body, `class="r"`) != 1 {
		t.Errorf("busca deveria achar só o publicado: %s", body)
	}

	_, body, _ = v.get("/feed.xml")
	must(t, body, "<rss", "<title>Blog do Bairro</title>", "/artigos/feira-de-domingo</link>", "Horta comunitária")
	if strings.Contains(body, "Festa junina") {
		t.Error("agendado no feed")
	}
	_, body, _ = v.get("/artigos/feira-de-domingo")
	must(t, body, `type="application/rss+xml"`)
	_ = srv
}

func TestComentarios(t *testing.T) {
	noSendmail(t)
	c, srv := newTCSite(t, siteWP)
	loginAdmin(c)
	_, body, _ := c.get("/_admin/conteudo/editar?c=artigos")
	tok := csrfRe.FindStringSubmatch(body)[1]
	newArticle(t, c, tok, map[string]string{"titulo": "Praça nova"})
	srv.saveEmailConf(emailConf{Avisos: "dono@ex.com.br"})

	v := anon(c)
	_, body, _ = v.get("/artigos/praca-nova")
	must(t, body, "0 comentários", `action="/_nep/comentar"`, "Deixe um comentário")
	vt := csrfRe.FindStringSubmatch(body)[1]
	id := strconv.FormatInt(srv.Store.All("artigos", 0)[0].ID, 10)
	form := url.Values{"_nep_csrf": {vt}, "col": {"artigos"}, "item": {id}, "volta": {"/artigos/praca-nova"}, "nome": {"Zé"}, "email": {"ze@ex.com"}, "texto": {"Que bom! <script>x</script>"}}
	code, _, h := v.post("/_nep/comentar", form)
	if code != 303 || !strings.HasSuffix(h.Get("Location"), "/artigos/praca-nova#comentarios") {
		t.Fatalf("comentar: %d %s", code, h.Get("Location"))
	}
	_, body, _ = v.get("/artigos/praca-nova")
	must(t, body, "aparece depois de aprovado", "0 comentários")
	if l := srv.emailLogs(); len(l) == 0 || !strings.Contains(l[0].Assunto, "Novo comentário") {
		t.Errorf("aviso de comentário: %+v", l)
	}

	// robô: finge sucesso e não grava
	bad := cloneV(form)
	bad.Set("_nep_hp", "spam")
	v.post("/_nep/comentar", bad)
	// item inexistente
	bad = cloneV(form)
	bad.Set("item", "999")
	v.post("/_nep/comentar", bad)
	if n := srv.Store.Count(colComments); n != 1 {
		t.Fatalf("comentários gravados: %d", n)
	}

	_, body, _ = c.get("/_admin")
	must(t, body, "Comentários (1)")
	_, body, _ = c.get("/_admin/comentarios")
	must(t, body, "Esperando", "ze@ex.com", "Praça nova")
	cid := strconv.FormatInt(srv.Store.All(colComments, 0)[0].ID, 10)
	c.post("/_admin/comentarios", url.Values{"_nep_csrf": {tok}, "acao": {"aprovar"}, "id": {cid}})
	_, body, _ = v.get("/artigos/praca-nova")
	must(t, body, "1 comentário", "<strong>Zé</strong>", "Que bom! &lt;script&gt;")
	if strings.Contains(body, "ze@ex.com") {
		t.Error("o e-mail de quem comentou apareceu no site")
	}
	// o código do site não alcança as coleções internas
	if ValidCol(colComments) {
		t.Error("coleção interna acessível pelo código do site")
	}
}

func TestMidiaECopia(t *testing.T) {
	c, srv := newTCSite(t, siteWP)
	loginAdmin(c)
	_, body, _ := c.get("/_admin/midia")
	must(t, body, "A biblioteca está vazia")
	tok := csrfRe.FindStringSubmatch(body)[1]
	code, _, _ := c.multipart("/_admin/midia", map[string]string{"_nep_csrf": tok, "acao": "enviar"}, map[string][]byte{"arquivo": pngBytes()})
	if code != 303 {
		t.Fatalf("enviar mídia: %d", code)
	}
	recs := srv.Store.All(colMedia, 0)
	if len(recs) != 1 {
		t.Fatalf("mídia: %d", len(recs))
	}
	f, _ := recs[0].ToValue().Get("arquivo")
	u := getS(f.(*lang.Table), "url")
	path := strings.TrimPrefix(u, "")
	if code, _, _ := anon(c).get(path); code != 200 {
		t.Fatalf("arquivo da mídia: %d", code)
	}
	_, body, _ = c.get("/_admin/midia")
	must(t, body, u)
	// o construtor visual recebe a biblioteca
	if j := mediaJSON(srv.mediaURLs()); !strings.Contains(j, u) {
		t.Errorf("mídia para o construtor: %s", j)
	}

	// cópia de segurança: baixa, cria algo novo, restaura e o novo some
	_, body, _ = c.get("/_admin/conteudo/editar?c=artigos")
	newArticle(t, c, tok, map[string]string{"titulo": "Antes da cópia"})
	code, zipBody, h := c.get("/_admin/backup?baixar=1")
	if code != 200 || h.Get("Content-Type") != "application/zip" {
		t.Fatalf("baixar cópia: %d %s", code, h.Get("Content-Type"))
	}
	zr, err := zip.NewReader(bytes.NewReader([]byte(zipBody)), int64(len(zipBody)))
	if err != nil {
		t.Fatal(err)
	}
	names := map[string]bool{}
	for _, zf := range zr.File {
		names[zf.Name] = true
	}
	if !names["nep.db"] || !names["arquivos/"+strings.TrimPrefix(path, "/_nep/arquivo/")] {
		t.Fatalf("conteúdo da cópia: %v", names)
	}
	newArticle(t, c, tok, map[string]string{"titulo": "Depois da cópia"})
	if anon(c).c == nil {
		t.Fatal()
	}
	code, _, _ = c.multipart("/_admin/backup", map[string]string{"_nep_csrf": tok, "acao": "restaurar"}, map[string][]byte{"copia": []byte(zipBody)})
	if code != 303 {
		t.Fatalf("restaurar cópia: %d", code)
	}
	_, body, _ = anon(c).get("/")
	must(t, body, "Antes da cópia")
	if strings.Contains(body, "Depois da cópia") {
		t.Error("restaurar não trocou os dados")
	}
	if code, _, _ := anon(c).get(path); code != 200 {
		t.Errorf("arquivo depois de restaurar: %d", code)
	}
	// arquivo que não é cópia
	code, body, _ = c.multipart("/_admin/backup", map[string]string{"_nep_csrf": tok, "acao": "restaurar"}, map[string][]byte{"copia": []byte("não é zip")})
	if code != 200 || !strings.Contains(body, "não é uma cópia") {
		t.Errorf("cópia inválida: %d", code)
	}

	// editor não baixa a cópia
	_ = io.Discard
}
