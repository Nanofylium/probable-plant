package main

import (
	"bytes"
	"encoding/json"
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

}
