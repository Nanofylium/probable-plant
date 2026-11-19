package web

import (
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Nanofylium/probable-plant/gerador"
)

func TestVisuaisServidosEInjetados(t *testing.T) {
	// todo visual que o gerador escolhe tem CSS aqui, e toda fonte citada existe
	for _, v := range gerador.Visuais {
		if !visualExiste(v.ID) {
			t.Errorf("visual %s sem CSS em assets/visuais", v.ID)
		}
	}
	s, _ := assistenteSrv(t, `<nep-config visual="noturno"><html><head><style>.x{}</style></head><body><page route="/" title="Oi"><h1>Oi</h1><p>{{ icone("☕") }} {{ icone("🦄") }}</p></page></body></html>`)
	get := func(p string) *httptest.ResponseRecorder {
		rec := httptest.NewRecorder()
		s.ServeHTTP(rec, httptest.NewRequest("GET", "http://localhost"+p, nil))
		return rec
	}
	pag := get("/").Body.String()
	i, j := strings.Index(pag, "/_nep/visual/noturno.css"), strings.Index(pag, "<style>.x{}</style>")
	if i < 0 || j < 0 || i < j {
		t.Fatalf("o visual devia vir depois do CSS do site:\n%s", pag)
	}
	if !strings.Contains(pag, `<svg class="nep-ic"`) || !strings.Contains(pag, "🦄") {
		t.Fatalf("icone(): %s", pag)
	}
	for _, v := range Visuais {
		r := get("/_nep/visual/" + v + ".css")
		css := r.Body.String()
		if r.Code != 200 || strings.Contains(css, "{{fontes}}") {
			t.Fatalf("%s: %d", v, r.Code)
		}
		for _, m := range strings.Split(css, `url("`)[1:] {
			u := m[:strings.Index(m, `"`)]
			if f := get(u); f.Code != 200 || f.Header().Get("Content-Type") != "font/woff2" {
				t.Errorf("%s cita %s, que não abre (%d)", v, u, f.Code)
			}
		}
	}
	if get("/_nep/fontes/nada.woff2").Code != 404 || get("/_nep/visual/nada.css").Code != 404 {
		t.Fatal("arquivo inexistente devia dar 404")
	}
	// visual desconhecido não entra
	s2, _ := assistenteSrv(t, `<nep-config visual="xyz"><html><head></head><body><page route="/" title="Oi">oi</page></body></html>`)
	rec := httptest.NewRecorder()
	s2.ServeHTTP(rec, httptest.NewRequest("GET", "http://localhost/", nil))
	if strings.Contains(rec.Body.String(), "/_nep/visual/") {
		t.Fatal("visual desconhecido entrou")
	}
}
