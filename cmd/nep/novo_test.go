package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// O modelo "blog" é o mesmo arquivo de exemplos/ascidia.
func TestModeloBlogIgualAoExemplo(t *testing.T) {
	a, err := os.ReadFile(filepath.Join("..", "..", "exemplos", "ascidia", "site.nep.html"))
	if err != nil {
		t.Fatal(err)
	}
	b, _ := modelosFS.ReadFile("modelos/blog.nep.html")
	if string(a) != string(b) {
		t.Error("cmd/nep/modelos/blog.nep.html está diferente de exemplos/ascidia/site.nep.html: copie de novo")
	}
}

func TestNovoCriaModelos(t *testing.T) {
	for _, m := range modelos() {
		dir := filepath.Join(t.TempDir(), m)
		if novo([]string{dir, "--modelo", m}) != 0 {
			t.Fatalf("nep novo --modelo %s falhou", m)
		}
		if code := checkSite(filepath.Join(dir, "site.nep.html")); code != 0 {
			t.Errorf("modelo %s não passa no nep check", m)
		}
	}
	if novo([]string{filepath.Join(t.TempDir(), "x"), "--modelo", "nao-existe"}) == 0 {
		t.Error("modelo inexistente deveria falhar")
	}
}

func TestPreviaDaClinica(t *testing.T) {
	src, err := os.ReadFile(filepath.Join("..", "..", "exemplos", "clinica", "site.nep.html"))
	if err != nil {
		t.Fatal(err)
	}
	f := filepath.Join(t.TempDir(), "site.nep.html")
	os.WriteFile(f, src, 0o644)
	html, n, err := gerarPrevia(f)
	if err != nil {
		t.Fatal(err)
	}
	if n < 8 {
		t.Errorf("prévia com poucas páginas: %d", n)
	}
	for _, ruim := range []string{"{{", "<nep>", "nep-fonte>", `href="/servicos"`, "/_nep/tema.css"} {
		if strings.Contains(html, ruim) {
			t.Errorf("a prévia não deveria ter %q", ruim)
		}
	}
	for _, bom := range []string{`data-rota="/agendar"`, `href="#/servicos"`, "--nep-cor"} {
		if !strings.Contains(html, bom) {
			t.Errorf("a prévia deveria ter %q", bom)
		}
	}
	if _, err := os.Stat(filepath.Join(filepath.Dir(f), "data")); err == nil {
		t.Error("a prévia não deve criar data/ ao lado do site")
	}
}
