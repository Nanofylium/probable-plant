package web

import (
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
	if f := get("/falta"); f.Code != 500 || !strings.Contains(f.Body.String(), "nep mundo foto.jpg") {
		t.Fatalf("mundo ausente devia explicar como gerar: %d", f.Code)
	}
}
