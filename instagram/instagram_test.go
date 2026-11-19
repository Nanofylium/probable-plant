package instagram

import (
	"archive/zip"
	"bytes"
	"io"
	"strings"
	"testing"
)

// latin1 imita o defeito do arquivo do Instagram (UTF-8 gravado como Latin-1).
func latin1(s string) string {
	var b strings.Builder
	for _, c := range []byte(s) {
		b.WriteRune(rune(c))
	}
	return b.String()
}

func zipDe(t *testing.T, arquivos map[string]string) *bytes.Reader {
	var buf bytes.Buffer
	z := zip.NewWriter(&buf)
	for n, c := range arquivos {
		w, _ := z.Create(n)
		w.Write([]byte(c))
	}
	z.Close()
	return bytes.NewReader(buf.Bytes())
}

func TestAbrir(t *testing.T) {
	bio := latin1("Pães de fermentação natural em Olinda 🥖")
	r := zipDe(t, map[string]string{
		"personal_information/personal_information/personal_information.json": `{"profile_user":[{"string_map_data":{
			"Nome":{"value":"` + latin1("Pão de Olinda") + `"},"Nome de usuário":{"value":"paodeolinda"},
			"Biografia":{"value":"` + bio + `"},"Site":{"value":"https://wa.me/5581999999999"}}}]}`,
		"your_instagram_activity/content/posts_1.json": `[
			{"media":[{"uri":"media/posts/202310/a.jpg","creation_timestamp":1696000000,"title":"Bolo de rolo saindo!"}]},
			{"title":"Fornada de sábado","creation_timestamp":1700000000,"media":[{"uri":"media/posts/202311/b.jpg"},{"uri":"media/posts/202311/c.mp4"}]}]`,
		"media/posts/202310/a.jpg": "JPEG-A",
		"media/posts/202311/b.jpg": "JPEG-B",
	})
	a, p, err := Abrir(r, r.Size())
	if err != nil {
		t.Fatal(err)
	}
	if p.Nome != "Pão de Olinda" || p.Usuario != "paodeolinda" || p.Bio != "Pães de fermentação natural em Olinda 🥖" || p.Site == "" {
		t.Fatalf("perfil: %+v", p)
	}
	if len(p.Posts) != 2 || p.Posts[0].Legenda != "Fornada de sábado" || len(p.Posts[0].Fotos) != 1 || p.Posts[1].Legenda != "Bolo de rolo saindo!" {
		t.Fatalf("posts: %+v", p.Posts)
	}
	f, _, err := a.Foto(p.Posts[0].Fotos[0])
	if err != nil {
		t.Fatal(err)
	}
	b, _ := io.ReadAll(f)
	if string(b) != "JPEG-B" {
		t.Fatalf("foto: %q", b)
	}
	if res := p.Resumo(2000); !strings.Contains(res, "@paodeolinda") || !strings.Contains(res, "Fornada de sábado") {
		t.Errorf("resumo: %s", res)
	}
}

func TestZipQueNaoEhDoInstagram(t *testing.T) {
	r := zipDe(t, map[string]string{"leiame.txt": "oi"})
	if _, _, err := Abrir(r, r.Size()); err == nil {
		t.Fatal("aceitou zip qualquer")
	}
	if _, _, err := Abrir(strings.NewReader("não é zip"), 9); err == nil {
		t.Fatal("aceitou arquivo que não é zip")
	}
}
