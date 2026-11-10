package web

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

const site2 = `<!doctype html>
<html lang="pt-BR"><head><title>{{ page.title }}</title></head><body>
<page route="/" title="Início" description="Pães de fermentação natural">
  <p>{{ ` + "`Olá, {request.query.nome |> default('visitante')}!`" + ` }}</p>
  <p>{{ [3, 1, 2].sort().join("-") }}</p>
</page>
<page route="/produto/:id" title="Produto"><nep>page.description = "Produto " + params.id</nep>x</page>
<page route="/privada" title="P" noindex>p</page>
<page route="/fotos" title="Fotos">
  <form save="fotos" success="ok">
    <input name="titulo" required>
    <input type="file" name="foto" accept="image/*" required data-publico>
    <input type="file" name="doc" accept=".pdf" data-max-mb="1">
  </form>
</page>
<page route="404" title="404">nada</page>
</body></html>`

func newTC2(t *testing.T) *tc {
	dir := t.TempDir()
	file := filepath.Join(dir, "site.nep.html")
	os.WriteFile(file, []byte(site2), 0o644)
	s, err := New(Options{SiteFile: file, Log: io.Discard})
	if err != nil {
		t.Fatal(err)
	}
	ts := httptest.NewServer(s)
	t.Cleanup(ts.Close)
	jar, _ := cookiejar.New(nil)
	return &tc{t: t, srv: ts, dir: dir, c: &http.Client{Jar: jar, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}}
}

func TestInterpolacaoEMetodosNoSite(t *testing.T) {
	c := newTC2(t)
	_, body, _ := c.get("/?nome=<Ana>")
	must(t, body, "Olá, &lt;Ana&gt;!", "<p>1-2-3</p>")
}

func TestSEO(t *testing.T) {
	c := newTC2(t)
	_, body, _ := c.get("/")
	must(t, body, `<meta name="description" content="Pães de fermentação natural">`, `<meta property="og:title" content="Início">`, `rel="canonical"`, `twitter:card`)
	_, body, _ = c.get("/produto/7")
	must(t, body, `content="Produto 7"`)
	_, body, _ = c.get("/privada")
	must(t, body, `<meta name="robots" content="noindex">`)
	code, sm, h := c.get("/sitemap.xml")
	if code != 200 || !strings.Contains(h.Get("Content-Type"), "xml") {
		t.Fatalf("sitemap %d", code)
	}
	must(t, sm, "<loc>"+c.srv.URL+"/</loc>", "<loc>"+c.srv.URL+"/fotos</loc>")
	if strings.Contains(sm, "privada") || strings.Contains(sm, ":id") || strings.Contains(sm, "404") {
		t.Errorf("sitemap com páginas que não devia: %s", sm)
	}
	_, rb, _ := c.get("/robots.txt")
	must(t, rb, "Disallow: /_admin", "Sitemap: "+c.srv.URL+"/sitemap.xml")
}

func TestEstatisticas(t *testing.T) {
	c := newTC2(t)
	get := func(ua, path string) {
		req, _ := http.NewRequest("GET", c.srv.URL+path, nil)
		req.Header.Set("User-Agent", ua)
		r, err := http.DefaultClient.Do(req)
		if err == nil {
			r.Body.Close()
		}
	}
	get("Mozilla/5.0 (iPhone) Mobile", "/")
	get("Mozilla/5.0 (iPhone) Mobile", "/fotos")
	get("Mozilla/5.0 (Windows)", "/")
	get("Googlebot/2.1", "/")
	get("Mozilla/5.0 (Windows)", "/nao-existe")
	srv := c.srv.Config.Handler.(*Server)
	st := srv.loadStats(7)
	if st.Total != 3 || st.Total2 != 2 || st.Pages["/"] != 2 || st.Mobile != 2 {
		t.Fatalf("estatísticas: %+v", st)
	}
	logs, _ := filepath.Glob(filepath.Join(c.dir, "data", "visitas-*.log"))
	b, _ := os.ReadFile(logs[0])
	if strings.Contains(string(b), "127.0.0.1") || strings.Contains(string(b), "Mozilla") {
		t.Error("o registro de visitas guardou IP ou navegador")
	}
}

func pngBytes() []byte {
	img := image.NewRGBA(image.Rect(0, 0, 2, 2))
	img.Set(0, 0, color.RGBA{255, 0, 0, 255})
	var b bytes.Buffer
	png.Encode(&b, img)
	return b.Bytes()
}

func (c *tc) multipart(path string, fields map[string]string, files map[string][]byte) (int, string, http.Header) {
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	for k, v := range fields {
		mw.WriteField(k, v)
	}
	for k, v := range files {
		w, _ := mw.CreateFormFile(k, k+"-original.bin")
		w.Write(v)
	}
	mw.Close()
	r, err := c.c.Post(c.srv.URL+path, mw.FormDataContentType(), &buf)
	if err != nil {
		c.t.Fatal(err)
	}
	defer r.Body.Close()
	b, _ := io.ReadAll(r.Body)
	return r.StatusCode, string(b), r.Header
}

func TestUpload(t *testing.T) {
	c := newTC2(t)
	_, body, _ := c.get("/fotos")
	must(t, body, `enctype="multipart/form-data"`)
	tok := csrfRe.FindStringSubmatch(body)[1]
	f := map[string]string{"_nep_form": "0", "_nep_csrf": tok, "titulo": "Bolo"}

	code, body, _ := c.multipart("/fotos", f, map[string][]byte{"foto": []byte("<html><script>alert(1)</script>")})
	if code != 422 || !strings.Contains(body, "Tipo de arquivo não aceito") {
		t.Fatalf("html disfarçado: %d", code)
	}
	code, body, _ = c.multipart("/fotos", f, map[string][]byte{"foto": pngBytes(), "doc": pngBytes()})
	if code != 422 || !strings.Contains(body, "Tipo de arquivo não aceito em &#34;Doc&#34;") {
		t.Fatalf("png no campo de pdf: %d", code)
	}
	code, _, _ = c.multipart("/fotos", f, map[string][]byte{"foto": pngBytes()})
	if code != 303 {
		t.Fatalf("envio válido: %d", code)
	}
	srv := c.srv.Config.Handler.(*Server)
	rs := srv.Store.All("fotos", 0)
	if len(rs) != 1 {
		t.Fatalf("registros: %d", len(rs))
	}
	m := regexp.MustCompile(`/_nep/arquivo/(p-[0-9a-f]{32}\.png)`).FindStringSubmatch(rs[0].Data)
	if m == nil {
		t.Fatalf("sem url do arquivo: %s", rs[0].Data)
	}
	code, img, h := c.get(m[0])
	if code != 200 || h.Get("Content-Type") != "image/png" || !bytes.Equal([]byte(img), pngBytes()) || !strings.Contains(h.Get("Content-Security-Policy"), "sandbox") {
		t.Fatalf("servir arquivo público: %d %s", code, h.Get("Content-Type"))
	}
	if code, _, _ := c.get("/_nep/arquivo/r-00000000000000000000000000000000.png"); code != 403 {
		t.Errorf("arquivo restrito sem login: %d", code)
	}
	if code, _, _ := c.get("/_nep/arquivo/..%2Fnep.db"); code != 404 {
		t.Errorf("caminho inválido: %d", code)
	}
	// apagar o registro apaga o arquivo
	srv.Store.Delete("fotos", rs[0].ID)
	if _, err := os.Stat(filepath.Join(c.dir, "data", "arquivos", m[1])); !os.IsNotExist(err) {
		t.Error("o arquivo continuou depois de apagar o registro")
	}
}
