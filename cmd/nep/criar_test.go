package main

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/Nanofylium/probable-plant/gerador"
	"github.com/Nanofylium/probable-plant/web"
)

// Todo site que o nep criar monta precisa passar no check e abrir todas as
// páginas sem erro, em todos os ramos e com todos os módulos.
func TestCriarSitesAbremSemErro(t *testing.T) {
	descs := []string{
		"uma escola de música chamada Sol Maior em Salvador, com blog, eventos e wiki, azul, elegante",
		"loja de roupas chamada Ateliê Ipê em Curitiba, com newsletter, vagas, downloads, fórum, galeria, planos e versão em inglês, rosa",
		"pousada em Paraty chamada Maré Mansa, com reservas, galeria e blog",
		"restaurante chamado Nonna Rosa com cardápio, delivery e reservas, vermelho",
		"associação de moradores do Jardim Europa sem blog",
		"clínica veterinária com agenda, equipe e loja",
		"comunidade de software livre com wiki, fórum e downloads",
		"escritório de advocacia, clássico",
		"só contato, chamada Mínimo",
		"qualquer coisa",
	}
	href := regexp.MustCompile(`href="(/[^"#?]*)`)
	for _, d := range descs {
		p := gerador.Entender(d)
		dir := t.TempDir()
		f := filepath.Join(dir, "site.nep.html")
		if err := os.WriteFile(f, []byte(gerador.Montar(p)), 0o644); err != nil {
			t.Fatal(err)
		}
		if _, err := web.Check(f); err != nil {
			t.Errorf("%q: check: %v", d, err)
			continue
		}
		srv, err := web.New(web.Options{SiteFile: f, DataDir: filepath.Join(dir, "dados")})
		if srv == nil {
			t.Fatalf("%q: %v", d, err)
		}
		vistos, fila := map[string]bool{}, []string{"/"}
		for len(fila) > 0 {
			u := fila[0]
			fila = fila[1:]
			if vistos[u] || strings.HasPrefix(u, "/_") || strings.Contains(u, ".") && u != "/feed.xml" {
				continue
			}
			vistos[u] = true
			rec := httptest.NewRecorder()
			srv.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "http://x"+u, nil))
			if rec.Code >= 500 {
				t.Errorf("%q: %s deu %d: %.300s", d, u, rec.Code, rec.Body.String())
			}
			for _, m := range href.FindAllStringSubmatch(rec.Body.String(), -1) {
				fila = append(fila, m[1])
			}
		}
		if len(vistos) < 5 {
			t.Errorf("%q: só %d páginas", d, len(vistos))
		}
	}
}
