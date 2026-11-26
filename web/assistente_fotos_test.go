package web

import (
	"bytes"
	"encoding/json"
	"image"
	"image/color"
	"image/png"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
)

func pngGrande(w, h int) []byte {
	img := image.NewNRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			img.SetNRGBA(x, y, color.NRGBA{uint8(x), uint8(y), 120, uint8(255 * (x % 2))})
		}
	}
	var b bytes.Buffer
	png.Encode(&b, img)
	return b.Bytes()
}

func TestAssistenteLogoFotosERedes(t *testing.T) {
	t.Setenv("NEP_IA_LOCAL", t.TempDir())
	s, f := assistenteSrv(t, "<!doctype html>\n<nepenthes></nepenthes>\n")
	srv := httptest.NewServer(s)
	defer srv.Close()
	jar, _ := cookiejar.New(nil)
	c := &http.Client{Jar: jar}

	enviar := func(campos map[string][][]byte) map[string]any {
		var corpo bytes.Buffer
		mw := multipart.NewWriter(&corpo)
		for campo, arqs := range campos {
			for i, a := range arqs {
				fw, _ := mw.CreateFormFile(campo, campo+string(rune('a'+i))+".png")
				fw.Write(a)
			}
		}
		mw.Close()
		req, _ := http.NewRequest("POST", srv.URL+"/_nep/assistente/fotos", &corpo)
		req.Header.Set("Content-Type", mw.FormDataContentType())
		res, err := c.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		b, _ := io.ReadAll(res.Body)
		res.Body.Close()
		if res.StatusCode != 200 {
			t.Fatalf("%d %s", res.StatusCode, b)
		}
		var out map[string]any
		json.Unmarshal(b, &out)
		return out
	}
	out := enviar(map[string][][]byte{
		"logo":  {pngGrande(900, 300)},
		"fotos": {pngGrande(2000, 1000), jpegDe(color.RGBA{200, 10, 10, 255}), jpegDe(color.RGBA{10, 200, 10, 255}), []byte("não é imagem")},
	})
	fotos := out["fotos"].([]any)
	if out["logo"] == nil || len(fotos) != 3 || out["avisos"] == nil {
		t.Fatalf("%v", out)
	}
	// a foto grande foi reduzida e virou JPG; o logo segue PNG
	u := fotos[0].(map[string]any)["url"].(string)
	r, _ := c.Get(srv.URL + u)
	img, formato, err := image.Decode(r.Body)
	r.Body.Close()
	if err != nil || formato != "jpeg" || img.Bounds().Dx() != fotoLado || img.Bounds().Dy() != fotoLado/2 {
		t.Fatalf("foto: %v %s %v", err, formato, img.Bounds())
	}
	logo := out["logo"].(map[string]any)["url"].(string)
	r, _ = c.Get(srv.URL + logo)
	img, formato, _ = image.Decode(r.Body)
	r.Body.Close()
	if formato != "png" || img.Bounds().Dx() != logoLado {
		t.Fatalf("logo: %s %v", formato, img.Bounds())
	}
	// tira uma foto
	id := fotos[2].(map[string]any)["id"].(string)
	req, _ := http.NewRequest("DELETE", srv.URL+"/_nep/assistente/fotos?id="+id, nil)
	res, _ := c.Do(req)
	var dep map[string]any
	json.NewDecoder(res.Body).Decode(&dep)
	res.Body.Close()
	if len(dep["fotos"].([]any)) != 2 || dep["logo"] == nil {
		t.Fatalf("depois de tirar: %v", dep)
	}
	if r, _ := c.Get(srv.URL + fotos[2].(map[string]any)["url"].(string)); r.StatusCode != 404 {
		t.Fatal("a foto tirada devia sumir dos arquivos", r.StatusCode)
	}

	enviar(map[string][][]byte{"fotos": {jpegDe(color.RGBA{10, 10, 200, 255})}}) // 3 fotos: galeria
	corpo := `{"descricao":"uma oficina de bicicletas","motor":"regras","dados":{"nome":"Pedal Certo","instagram":"@pedalcerto",
		"facebook":"facebook.com/pedalcerto","tiktok":"@pedal.certo","youtube":"https://youtube.com/@pedalcerto","linkedin":"https://evil.example/x","x":"pedalcerto"}}`
	res, err = c.Post(srv.URL+"/_nep/assistente/criar", "application/json", strings.NewReader(corpo))
	if err != nil {
		t.Fatal(err)
	}
	b, _ := io.ReadAll(res.Body)
	res.Body.Close()
	if res.StatusCode != 200 {
		t.Fatalf("criar: %s", b)
	}
	site, _ := os.ReadFile(f)
	for _, want := range []string{`<img class="logo" src="{{ url("` + strings.TrimPrefix(logo, s.opt.Base) + `") }}"`, "https://instagram.com/pedalcerto",
		"https://facebook.com/pedalcerto", "https://www.tiktok.com/@pedal.certo", "https://youtube.com/@pedalcerto", "https://x.com/pedalcerto", `categoria: "Fotos"`, `class="ilustracao foto"`} {
		if !strings.Contains(string(site), want) {
			t.Errorf("o site não tem %s", want)
		}
	}
	if strings.Contains(string(site), "evil.example") {
		t.Error("link de outro site não pode virar rede social")
	}
	_, home := getBody(t, c, srv.URL+"/")
	if !strings.Contains(home, `class="logo"`) || !strings.Contains(home, "/_nep/arquivo/p-") || !strings.Contains(home, "tiktok.com/@pedal.certo") {
		t.Fatalf("início sem logo, foto ou redes: %.300s", home)
	}
}
