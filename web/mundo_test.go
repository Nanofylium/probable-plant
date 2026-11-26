package web

import (
	"bytes"
	"image"
	"image/jpeg"
	"io"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Nanofylium/probable-plant/mundo"
)

func TestMundoNoSite(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "site.nep.html")
	os.WriteFile(file, []byte(`<html><head><title>M</title></head><body>
<page route="/" title="Passeio">{{ mundo("loja", {titulo: "Conheça a loja", rolar: true}) }}</page>
<page route="/falta" title="Falta">{{ mundo("nao-existe") }}</page>
</body></html>`), 0o644)
	m := &mundo.Mundo{Nome: "loja", Largura: 832, Altura: 480, FPS: 20, Fonte: "simulado",
		Caminhos: []mundo.Caminho{{ID: "frente", Nome: "Entrar", Ext: "jpg"}, {ID: "redor", Nome: "Olhar ao redor", Ext: "jpg"}}}
	if err := mundo.Salvar(filepath.Join(dir, "mundos"), m, map[string][][]byte{"frente": {[]byte("q1"), []byte("q2")}, "redor": {[]byte("r1")}}); err != nil {
		t.Fatal(err)
	}
	s, err := New(Options{SiteFile: file, Log: io.Discard, Dev: true})
	if err != nil {
		t.Fatal(err)
	}
	get := func(p string) *httptest.ResponseRecorder {
		rec := httptest.NewRecorder()
		s.ServeHTTP(rec, httptest.NewRequest("GET", "http://localhost"+p, nil))
		return rec
	}
	r := get("/")
	b := r.Body.String()
	if r.Code != 200 {
		t.Fatalf("%d %s", r.Code, b)
	}
	must(t, b, `class="nep-mundo"`, `<img src="/mundos/loja/frente/001.jpg"`, `alt="Conheça a loja"`, `&#34;rolar&#34;:true`, `/_nep/mundo.js`)
	if q := get("/mundos/loja/frente/002.jpg"); q.Code != 200 || q.Body.String() != "q2" {
		t.Fatalf("quadro: %d", q.Code)
	}
	if js := get("/_nep/mundo.js"); js.Code != 200 || !strings.Contains(js.Body.String(), "nepMundos") {
		t.Fatal("mundo.js")
	}
	m3 := &mundo.Mundo{Nome: "sala", Fonte: "estimado"}
	var fb bytes.Buffer
	jpeg.Encode(&fb, image.NewRGBA(image.Rect(0, 0, 200, 100)), nil)
	mapa, _ := mundo.EstimarProfundidade(fb.Bytes())
	if err := mundo.Salvar3D(filepath.Join(dir, "mundos"), m3, fb.Bytes(), mapa); err != nil {
		t.Fatal(err)
	}
	os.WriteFile(file, []byte(strings.Replace(string(must2(os.ReadFile(file))), `<page route="/falta"`, `<page route="/sala" title="Sala">{{ mundo("sala") }}</page><page route="/falta"`, 1)), 0o644)
	s2, _ := New(Options{SiteFile: file, Log: io.Discard, Dev: true})
	rec := httptest.NewRecorder()
	s2.ServeHTTP(rec, httptest.NewRequest("GET", "http://localhost/sala", nil))
	must(t, rec.Body.String(), `<img src="/mundos/sala/foto.jpg"`, `&#34;tipo&#34;:&#34;3d&#34;`, `&#34;profundidade&#34;:&#34;profundidade.png&#34;`)
	if f := get("/falta"); f.Code != 500 || !strings.Contains(f.Body.String(), "nep mundo foto.jpg") {
		t.Fatalf("mundo ausente devia explicar como gerar: %d", f.Code)
	}
}

func must2(b []byte, _ error) []byte { return b }
