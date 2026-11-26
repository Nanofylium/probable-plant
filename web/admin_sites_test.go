package web

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestHtaccessIgualAoDaHospedagem(t *testing.T) {
	b, err := os.ReadFile("../hospedagem/.htaccess")
	if err != nil {
		t.Fatal(err)
	}
	if string(b) != htaccessCGI {
		t.Fatal("web/assets/htaccess.txt e hospedagem/.htaccess devem ser iguais")
	}
}

func TestCriarSiteNaHospedagem(t *testing.T) {
	raiz := t.TempDir()
	mural := filepath.Join(raiz, "mural")
	os.Mkdir(mural, 0o755)
	f := filepath.Join(mural, "index.html")
	os.WriteFile(f, []byte(`<nepenthes tipo="blog" nome="Mural">x</nepenthes>`), 0o644)
	s, err := New(Options{SiteFile: f, DataDir: filepath.Join(mural, "data")})
	if s == nil {
		t.Fatal(err)
	}
	s.Store.SetMeta("admin_hash", hashPassword("senha-do-dono-123"))
	dados := map[string]string{"tipo": "portal", "nome": `Folha "do" Vale`, "cidade": "Aveiro", "descricao": "Portal de notícias <b>da</b> cidade."}
	dest := filepath.Join(s.raizWeb(), "folha")
	if err := s.criarSiteNaPasta(dest, func(k string) string { return dados[k] }); err != nil {
		t.Fatal(err)
	}
	st, err := os.Stat(filepath.Join(dest, "nep.cgi"))
	if err != nil || st.Mode().Perm() != 0o755 {
		t.Fatalf("nep.cgi: %v %v", st, err)
	}
	if ht, _ := os.ReadFile(filepath.Join(dest, ".htaccess")); !strings.Contains(string(ht), "nep.cgi") {
		t.Fatal(".htaccess")
	}
	novo, err := New(Options{SiteFile: filepath.Join(dest, "index.html"), DataDir: filepath.Join(dest, "data")})
	if novo == nil {
		t.Fatal(err)
	}
	if novo.Store.Meta("admin_hash") != s.Store.Meta("admin_hash") {
		t.Fatal("o site novo deve abrir com a senha do painel atual")
	}
	w := pedir(novo, "GET", "/", "", "203.0.113.5")
	if w.Code != 200 || !strings.Contains(w.Body.String(), "Folha &#34;do&#34; Vale") && !strings.Contains(w.Body.String(), "Folha &quot;do&quot; Vale") {
		t.Fatalf("%d %.300s", w.Code, w.Body.String())
	}
	if got := s.sitesNaRaiz(); len(got) != 1 || got[0].Pasta != "folha" {
		t.Fatalf("%+v", got)
	}
	if err := s.criarSiteNaPasta(dest, func(k string) string { return dados[k] }); err == nil {
		t.Fatal("não pode sobrescrever uma pasta que já existe")
	}
}
