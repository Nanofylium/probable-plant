package main

import (
	"bytes"
	"encoding/json"
	"image"
	"image/color"
	"image/jpeg"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func pedidoCentral(c *central, metodo, caminho string, corpo []byte, tipo string, token bool) *httptest.ResponseRecorder {
	r := httptest.NewRequest(metodo, caminho, bytes.NewReader(corpo))
	r.RemoteAddr, r.Host = "127.0.0.1:5000", "localhost:8000"
	if tipo != "" {
		r.Header.Set("Content-Type", tipo)
	}
	if token {
		r.Header.Set("X-Central", c.token)
	}
	w := httptest.NewRecorder()
	c.ServeHTTP(w, r)
	return w
}

func TestCentral(t *testing.T) {
	t.Setenv("NEP_PROFUNDIDADE", filepath.Join(t.TempDir(), "nao-existe.onnx"))
	t.Setenv("HTTPS_PROXY", "http://127.0.0.1:1") // sem internet: o passeio usa a profundidade estimada
	c := novaCentral(t.TempDir())
	c.extras = nil
	defer func() {
		for p := range c.sites {
			c.parar(p)
		}
	}()

	w := pedidoCentral(c, "GET", "/", nil, "", false)
	if w.Code != 200 || !strings.Contains(w.Body.String(), c.token) || strings.Contains(w.Body.String(), "{{token}}") {
		t.Fatalf("página: %d", w.Code)
	}
	// de fora do computador, ou com outro endereço, não abre
	r := httptest.NewRequest("GET", "/", nil)
	r.RemoteAddr, r.Host = "10.0.0.5:5000", "localhost:8000"
	w = httptest.NewRecorder()
	c.ServeHTTP(w, r)
	if w.Code != 403 {
		t.Fatal("acesso de fora devia ser negado")
	}
	r = httptest.NewRequest("GET", "/api/estado", nil)
	r.RemoteAddr, r.Host = "127.0.0.1:5000", "malicioso.exemplo:8000"
	w = httptest.NewRecorder()
	c.ServeHTTP(w, r)
	if w.Code != 403 {
		t.Fatal("Host de fora devia ser negado")
	}
	// sem o token, nada muda
	if w := pedidoCentral(c, "POST", "/api/criar", []byte(`{"descricao":"padaria"}`), "application/json", false); w.Code != 403 {
		t.Fatalf("sem token: %d", w.Code)
	}

	w = pedidoCentral(c, "POST", "/api/criar", []byte(`{"descricao":"padaria artesanal em Curitiba","visual":"suave"}`), "application/json", true)
	var criado struct{ URL, Pasta string }
	json.Unmarshal(w.Body.Bytes(), &criado)
	if w.Code != 200 || criado.URL == "" {
		t.Fatalf("criar: %d %s", w.Code, w.Body)
	}
	if _, err := os.Stat(filepath.Join(criado.Pasta, "site-previa.html")); err != nil {
		t.Fatal("o site novo devia vir com a prévia")
	}
	cli := &http.Client{Timeout: 10 * time.Second}
	res, err := cli.Get(criado.URL)
	if err != nil || res.StatusCode != 200 {
		t.Fatalf("o site não abriu: %v", err)
	}
	res.Body.Close()
	if res, err := cli.Get(criado.URL + "_admin"); err != nil || res.StatusCode >= 500 {
		t.Fatalf("painel: %v", err)
	} else {
		res.Body.Close()
	}

	if w := pedidoCentral(c, "POST", "/api/modelo", []byte(`{"modelo":"blog"}`), "application/json", true); w.Code != 200 {
		t.Fatalf("modelo: %s", w.Body)
	}
	if w := pedidoCentral(c, "POST", "/api/modelo", []byte(`{"modelo":"../x"}`), "application/json", true); w.Code != 400 {
		t.Fatal("modelo inválido devia dar erro")
	}
	if w := pedidoCentral(c, "POST", "/api/assistente", []byte(`{"nome":"Minha Loja"}`), "application/json", true); w.Code != 200 {
		t.Fatalf("assistente: %s", w.Body)
	}
	if w := pedidoCentral(c, "POST", "/api/abrir", []byte(`{"pasta":"/etc"}`), "application/json", true); w.Code != 400 {
		t.Fatal("pasta fora da Central devia dar erro")
	}

	sites := c.listar()
	if len(sites) != 3 {
		t.Fatalf("%d sites", len(sites))
	}
	novos := 0
	for _, s := range sites {
		if s.Novo {
			novos++
			if s.Nome != "minha-loja" {
				t.Fatalf("nome da pasta do assistente: %q", s.Nome)
			}
		}
	}
	if novos != 1 {
		t.Fatal("o assistente devia aparecer como novo")
	}

	// passeio 3D a partir de uma foto
	img := image.NewRGBA(image.Rect(0, 0, 320, 240))
	for y := 0; y < 240; y++ {
		for x := 0; x < 320; x++ {
			img.Set(x, y, color.RGBA{uint8(x), uint8(y), 120, 255})
		}
	}
	var foto bytes.Buffer
	jpeg.Encode(&foto, img, nil)
	var corpo bytes.Buffer
	mp := multipart.NewWriter(&corpo)
	mp.WriteField("pasta", criado.Pasta)
	mp.WriteField("nome", "Fachada da Loja")
	fw, _ := mp.CreateFormFile("foto", "f.jpg")
	fw.Write(foto.Bytes())
	mp.Close()
	if w := pedidoCentral(c, "POST", "/api/passeio", corpo.Bytes(), mp.FormDataContentType(), true); w.Code != 200 {
		t.Fatalf("passeio: %s", w.Body)
	}
	for i := 0; i < 300; i++ {
		c.mu.Lock()
		ativa := c.tarefa.Ativa
		c.mu.Unlock()
		if !ativa {
			break
		}
		time.Sleep(50 * time.Millisecond)
	}
	if c.tarefa.Ativa || c.tarefa.Erro != "" {
		t.Fatalf("tarefa: %+v", c.tarefa)
	}
	nome := filepath.Base(criado.Pasta)
	for _, s := range c.listar() {
		if s.Pasta == criado.Pasta {
			if len(s.Mundos) != 1 || s.Mundos[0] != "fachada-da-loja" {
				t.Fatalf("mundos: %v", s.Mundos)
			}
		}
	}
	w = pedidoCentral(c, "GET", "/ver/"+nome+"/fachada-da-loja/", nil, "", false)
	if w.Code != 200 || !strings.Contains(w.Body.String(), `/ver/`+nome+`/fachada-da-loja/arq/foto.jpg`) || !strings.Contains(w.Body.String(), `src="/_nep/mundo.js"`) {
		t.Fatalf("ver: %d %s", w.Code, w.Body)
	}
	if w := pedidoCentral(c, "GET", "/ver/"+nome+"/fachada-da-loja/arq/profundidade.png", nil, "", false); w.Code != 200 {
		t.Fatalf("arquivo: %d", w.Code)
	}
	if w := pedidoCentral(c, "GET", "/ver/"+nome+"/fachada-da-loja/arq/mundo.json", nil, "", false); w.Code != 404 {
		t.Fatal("só a foto e a profundidade são entregues")
	}
	if w := pedidoCentral(c, "GET", "/ver/nao-existe/x/", nil, "", false); w.Code != 404 {
		t.Fatal("site inexistente")
	}
	if w := pedidoCentral(c, "GET", "/_nep/mundo.js", nil, "", false); w.Code != 200 || w.Body.Len() < 1000 {
		t.Fatal("mundo.js")
	}
}
