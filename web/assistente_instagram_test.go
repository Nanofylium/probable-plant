package web

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"image"
	"image/color"
	"image/jpeg"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"os"
	"regexp"
	"strings"
	"testing"

	"github.com/Nanofylium/probable-plant/instagram"
)

func jpegDe(c color.RGBA) []byte {
	img := image.NewRGBA(image.Rect(0, 0, 40, 40))
	for i := range img.Pix {
		img.Pix[i] = []uint8{c.R, c.G, c.B, 255}[i%4]
	}
	var b bytes.Buffer
	jpeg.Encode(&b, img, nil)
	return b.Bytes()
}

// zipInstagram imita o "Baixar suas informações" do Instagram (em português,
// com o defeito de acentos do arquivo original).
func zipInstagram(t *testing.T) []byte {
	l1 := func(s string) string {
		var b strings.Builder
		for _, c := range []byte(s) {
			b.WriteRune(rune(c))
		}
		return strings.ReplaceAll(b.String(), `"`, `\"`)
	}
	var buf bytes.Buffer
	z := zip.NewWriter(&buf)
	add := func(n string, b []byte) { w, _ := z.Create(n); w.Write(b) }
	add("personal_information/personal_information/personal_information.json", []byte(`{"profile_user":[{"string_map_data":{
		"Nome":{"value":"`+l1("Fornalha de Olinda")+`"},"Nome de usuário":{"value":"fornalhaolinda"},
		"Biografia":{"value":"`+l1("Padaria artesanal no Carmo. Pão de fermentação natural e bolo de rolo.")+`"}}}]}`))
	add("your_instagram_activity/content/posts_1.json", []byte(`[
		{"title":"`+l1("Fornada das seis")+`","creation_timestamp":1700000300,"media":[{"uri":"media/posts/1/a.jpg"},{"uri":"media/posts/1/b.jpg"}]},
		{"title":"Bolo de rolo","creation_timestamp":1700000200,"media":[{"uri":"media/posts/2/c.jpg"}]},
		{"title":"<script>alert(1)</script>","creation_timestamp":1700000100,"media":[{"uri":"media/posts/3/d.jpg"},{"uri":"media/posts/3/e.mp4"}]}]`))
	add("media/posts/1/a.jpg", jpegDe(color.RGBA{200, 120, 40, 255}))
	add("media/posts/1/b.jpg", jpegDe(color.RGBA{90, 60, 30, 255}))
	add("media/posts/2/c.jpg", jpegDe(color.RGBA{240, 200, 150, 255}))
	add("media/posts/3/d.jpg", []byte("isto não é uma foto"))
	z.Close()
	return buf.Bytes()
}

func TestAssistenteInstagramEAtendente(t *testing.T) {
	t.Setenv("NEP_IA_LOCAL", t.TempDir()) // sem IA do Nepenthes baixada
	s, f := assistenteSrv(t, "<!doctype html>\n<nepenthes></nepenthes>\n")
	srv := httptest.NewServer(s)
	defer srv.Close()
	jar, _ := cookiejar.New(nil)
	c := &http.Client{Jar: jar}

	var corpo bytes.Buffer
	mw := multipart.NewWriter(&corpo)
	mw.WriteField("usuario", "https://instagram.com/outro_nome/")
	fw, _ := mw.CreateFormFile("zip", "instagram.zip")
	fw.Write(zipInstagram(t))
	mw.Close()
	req, _ := http.NewRequest("POST", srv.URL+"/_nep/assistente/instagram", &corpo)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	res, err := c.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	b, _ := io.ReadAll(res.Body)
	res.Body.Close()
	var out struct {
		Instagram struct {
			Usuario, Nome, Bio string
			Posts              int
			Fotos              []string
		}
	}
	json.Unmarshal(b, &out)
	in := out.Instagram
	// o @ do arquivo vale mais que o digitado; o que não é foto fica de fora
	if res.StatusCode != 200 || in.Usuario != "fornalhaolinda" || in.Nome != "Fornalha de Olinda" || in.Posts != 3 || len(in.Fotos) != 3 {
		t.Fatalf("instagram: %d %s", res.StatusCode, b)
	}
	if r, _ := c.Get(srv.URL + in.Fotos[0]); r.StatusCode != 200 || r.Header.Get("Content-Type") != "image/jpeg" {
		t.Fatalf("foto: %d %s", r.StatusCode, r.Header.Get("Content-Type"))
	}

	// cria sem IA: o ramo vem da bio, o nome do perfil, as fotos vão para capa e galeria
	res, err = c.Post(srv.URL+"/_nep/assistente/criar", "application/json", strings.NewReader(`{"descricao":"quero um site bonito","motor":"regras"}`))
	if err != nil {
		t.Fatal(err)
	}
	b, _ = io.ReadAll(res.Body)
	res.Body.Close()
	if res.StatusCode != 200 || !strings.Contains(string(b), `"nome":"Fornalha de Olinda"`) || !strings.Contains(string(b), "restaurante") {
		t.Fatalf("criar: %d %s", res.StatusCode, b)
	}
	site, _ := os.ReadFile(f)
	for _, want := range []string{"https://instagram.com/fornalhaolinda", `class="ilustracao foto"`, "{{ lowii() }}"} {
		if !strings.Contains(string(site), want) {
			t.Errorf("o site não tem %s", want)
		}
	}
	_, home := getBody(t, c, srv.URL+"/")
	if !strings.Contains(home, "/_nep/arquivo/p-") || !strings.Contains(home, "nep-lowii") {
		t.Fatalf("início sem a foto ou sem o atendente: %.400s", home)
	}
	if code, gal := getBody(t, c, srv.URL+"/galeria"); code != 200 || strings.Count(gal, `<img src="/_nep/arquivo/p-`) != 3 || strings.Contains(gal, "<script>alert") {
		t.Fatalf("galeria: %d %d fotos", code, strings.Count(gal, `<img src="/_nep/arquivo/p-`))
	}

	// atendente sem IA: responde com o que a busca do site acha
	csrf := regexp.MustCompile(`data-csrf="([0-9a-f]+)"`).FindStringSubmatch(home)
	if csrf == nil {
		t.Fatal("atendente sem csrf")
	}
	req, _ = http.NewRequest("POST", srv.URL+"/_nep/lowii", strings.NewReader(`{"pergunta":"vocês entregam?"}`))
	req.Header.Set("X-Nep-Csrf", csrf[1])
	req.Header.Set("Content-Type", "application/json")
	res, err = c.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	var resp map[string]any
	json.NewDecoder(res.Body).Decode(&resp)
	res.Body.Close()
	if r, _ := resp["resposta"].(string); r == "" || resp["sem_ia"] != true {
		t.Fatalf("atendente sem IA: %v", resp)
	}
}

func getBody(t *testing.T, c *http.Client, u string) (int, string) {
	r, err := c.Get(u)
	if err != nil {
		t.Fatal(err)
	}
	defer r.Body.Close()
	b, _ := io.ReadAll(r.Body)
	return r.StatusCode, string(b)
}

// Entrar com Instagram: app ligado, login, volta com o código, leitura pela API.
func TestAssistenteEntrarComInstagram(t *testing.T) {
	t.Setenv("NEP_IA_LOCAL", t.TempDir())
	foto := jpegDe(color.RGBA{180, 100, 40, 255})
	var api *httptest.Server
	api = httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/oauth/access_token":
			r.ParseForm()
			if r.Form.Get("code") != "CODIGO" || r.Form.Get("client_secret") != "segredo123" {
				http.Error(w, `{"error_message":"código errado"}`, 400)
				return
			}
			w.Write([]byte(`{"access_token":"CURTO","user_id":1}`))
		case "/access_token":
			w.Write([]byte(`{"access_token":"LONGO","expires_in":5183944}`))
		case "/me":
			if r.URL.Query().Get("access_token") != "LONGO" {
				http.Error(w, `{"error":{"message":"token"}}`, 400)
				return
			}
			if strings.Contains(r.URL.Query().Get("fields"), "biography") { // como se a API recusasse bio
				http.Error(w, `{"error":{"message":"Tried accessing nonexisting field (biography)"}}`, 400)
				return
			}
			w.Write([]byte(`{"username":"fornalhaolinda","name":"Fornalha de Olinda"}`))
		case "/me/media":
			w.Write([]byte(`{"data":[
				{"id":"1","caption":"Pão de fermentação natural","media_type":"IMAGE","media_url":"` + api.URL + `/f/1.jpg","permalink":"https://www.instagram.com/p/AAA/","timestamp":"2026-11-01T10:00:00+0000"},
				{"id":"2","caption":"Carrossel do sábado","media_type":"CAROUSEL_ALBUM","permalink":"https://www.instagram.com/p/BBB/","timestamp":"2026-10-30T10:00:00+0000",
				 "children":{"data":[{"media_type":"IMAGE","media_url":"` + api.URL + `/f/2.jpg"},{"media_type":"VIDEO","media_url":"` + api.URL + `/f/v.mp4"}]}},
				{"id":"3","caption":"Bolo de rolo","media_type":"IMAGE","media_url":"` + api.URL + `/f/3.jpg"}]}`))
		default:
			if strings.HasPrefix(r.URL.Path, "/f/") {
				w.Write(foto)
				return
			}
			http.NotFound(w, r)
		}
	}))
	defer api.Close()
	velhos := []string{instagram.URLToken, instagram.URLGraph}
	velhoCli, velhoHTTP := instagram.Cliente, instaHTTP
	instagram.URLToken, instagram.URLGraph = api.URL+"/oauth/access_token", api.URL
	instagram.Cliente, instaHTTP = api.Client(), api.Client()
	defer func() {
		instagram.URLToken, instagram.URLGraph = velhos[0], velhos[1]
		instagram.Cliente, instaHTTP = velhoCli, velhoHTTP
	}()

	s, _ := assistenteSrv(t, "<!doctype html>\n<nepenthes></nepenthes>\n")
	srv := httptest.NewServer(s)
	defer srv.Close()
	c := &http.Client{CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}

	// sem app ligado, não entra
	if r, _ := c.Get(srv.URL + "/_nep/assistente/instagram/entrar"); r.StatusCode != 400 {
		t.Fatalf("entrar sem app: %d", r.StatusCode)
	}
	r, _ := c.Post(srv.URL+"/_nep/assistente/instagram/app", "application/json", strings.NewReader(`{"app_id":"123","segredo":"segredo123"}`))
	b, _ := io.ReadAll(r.Body)
	if !strings.Contains(string(b), `"ligado":true`) || strings.Contains(string(b), "segredo123") || !strings.Contains(string(b), "/_nep/assistente/instagram/volta") {
		t.Fatalf("app: %s", b)
	}
	r, _ = c.Get(srv.URL + "/_nep/assistente/instagram/entrar")
	loc, _ := r.Location()
	if r.StatusCode != 302 || loc.Host != "www.instagram.com" || loc.Query().Get("client_id") != "123" || loc.Query().Get("scope") != "instagram_business_basic" {
		t.Fatalf("entrar: %d %v", r.StatusCode, loc)
	}
	estado := loc.Query().Get("state")
	// volta com estado errado: recusa
	r, _ = c.Get(srv.URL + "/_nep/assistente/instagram/volta?code=CODIGO&state=outro")
	if l, _ := r.Location(); !strings.Contains(l.RawQuery, "instagram_erro") {
		t.Fatalf("estado errado aceito: %v", l)
	}
	// de novo, agora certo
	r, _ = c.Get(srv.URL + "/_nep/assistente/instagram/entrar")
	loc, _ = r.Location()
	estado = loc.Query().Get("state")
	r, _ = c.Get(srv.URL + "/_nep/assistente/instagram/volta?code=CODIGO%23_&state=" + estado)
	if l, _ := r.Location(); r.StatusCode != 302 || l.RawQuery != "instagram=ok" {
		t.Fatalf("volta: %d %v", r.StatusCode, l)
	}
	d := s.instaGuardado()
	if d == nil || d.Usuario != "fornalhaolinda" || d.Nome != "Fornalha de Olinda" || d.Posts != 3 || len(d.Fotos) != 3 || d.Fotos[0].Link != "https://www.instagram.com/p/AAA/" {
		t.Fatalf("dados: %+v", d)
	}
	if !strings.Contains(d.Resumo, "Carrossel do sábado") {
		t.Errorf("resumo: %s", d.Resumo)
	}
}
