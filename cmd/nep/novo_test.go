package main

import (
	"os"
	"path/filepath"
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
