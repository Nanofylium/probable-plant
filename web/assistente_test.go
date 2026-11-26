package web

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const conteudoNavegador = `{"nome": "Pão de Mel", "ramo": "restaurante", "cidade": "Olinda", "cor": "#b45309", "estilo": "acolhedor",
 "chapeu": "Padaria de bairro", "titulo": "Pão quentinho às seis da manhã", "texto": "Fermentação natural e café coado na hora.",
 "chamada": "Passe aqui ou peça pelo WhatsApp",
 "servicos": [{"icone": "🥖", "titulo": "Pão de fermentação natural", "texto": "Casca crocante.", "preco": 18},
              {"icone": "🥐", "titulo": "Croissant de manteiga", "texto": "Folhado de verdade.", "preco": 9.5}],
 "depoimentos": [{"texto": "O melhor pão de Olinda", "autor": "Bia, Carmo"}, {"texto": "Café ótimo", "autor": "Zé, Bairro Novo"}]}`

func assistenteSrv(t *testing.T, conteudo string) (*Server, string) {
	dir := t.TempDir()
	f := filepath.Join(dir, "index.html")
	if err := os.WriteFile(f, []byte(conteudo), 0o644); err != nil {
		t.Fatal(err)
	}
	s, err := New(Options{SiteFile: f, DataDir: filepath.Join(dir, "dados"), AposCriar: func(site string) []string {
		extra := site + ".extra"
		os.WriteFile(extra, []byte("ok"), 0o644)
		return []string{extra}
	}})
	if s == nil {
		t.Fatal(err)
	}
	return s, f
}

func pedir(s *Server, metodo, alvo, corpo, ip string) *httptest.ResponseRecorder {
	var b io.Reader
	if corpo != "" {
		b = strings.NewReader(corpo)
	}
	r := httptest.NewRequest(metodo, "http://localhost:8000"+alvo, b)
	r.RemoteAddr = ip + ":40000"
	if corpo != "" {
		r.Header.Set("Content-Type", "application/json")
		r.Header.Set("Origin", "http://localhost:8000")
	}
	w := httptest.NewRecorder()
	s.ServeHTTP(w, r)
	return w
}

func TestAssistenteNepenthes(t *testing.T) {
	s, f := assistenteSrv(t, "<!doctype html>\n<nepenthes>uma padaria em Olinda</nepenthes>\n")

	// visitante de fora: só "em construção"; não pode criar
	if w := pedir(s, "GET", "/", "", "203.0.113.9"); w.Code != 503 || strings.Contains(w.Body.String(), "Como você quer") {
		t.Fatalf("visitante viu o assistente: %d", w.Code)
	}
	if w := pedir(s, "POST", "/_nep/assistente/criar", `{"descricao":"x","motor":"regras"}`, "203.0.113.9"); w.Code != 503 {
		t.Fatalf("visitante criou: %d", w.Code)
	}
	// no próprio computador: a tela
	if w := pedir(s, "GET", "/qualquer/pagina", "", "127.0.0.1"); w.Code != 200 || !strings.Contains(w.Body.String(), "Como você quer o seu site?") {
		t.Fatalf("assistente: %d %.200s", w.Code, w.Body.String())
	}
	w := pedir(s, "GET", "/_nep/assistente/estado", "", "127.0.0.1")
	var est map[string]any
	json.Unmarshal(w.Body.Bytes(), &est)
	if est["ideia"] != "uma padaria em Olinda" || est["servidor"] != "" || !strings.Contains(est["prompt_site"].(string), "JSON") {
		t.Fatalf("estado: %v", est)
	}
	// a IA do Nepenthes (ialocal) aparece no estado; visitante não vê nem baixa
	if loc, _ := est["local"].(map[string]any); loc == nil || loc["modelo"] != "Granite 3.3 2B" {
		t.Fatalf("estado da IA local: %v", est["local"])
	}
	if w := pedir(s, "POST", "/_nep/assistente/local", "", "203.0.113.9"); w.Code != 503 {
		t.Fatalf("visitante mandou baixar a IA: %d", w.Code)
	}
	w = pedir(s, "POST", "/_nep/assistente/perguntas", `{"descricao":"uma padaria em Olinda","motor":"regras"}`, "127.0.0.1")
	if !strings.Contains(w.Body.String(), "nome") {
		t.Fatalf("perguntas: %s", w.Body.String())
	}
	// outro site tentando mandar o navegador de quem está criando
	r := httptest.NewRequest("POST", "http://localhost:8000/_nep/assistente/criar", strings.NewReader(`{"descricao":"x"}`))
	r.RemoteAddr, _ = "127.0.0.1:1", r.Header
	r.Header.Set("Origin", "https://malvado.example")
	rec := httptest.NewRecorder()
	s.ServeHTTP(rec, r)
	if rec.Code != 400 {
		t.Fatalf("POST de outra origem: %d", rec.Code)
	}

	// cria com o texto que o Qwen do navegador escreveu
	corpo, _ := json.Marshal(map[string]any{"descricao": "uma padaria em Olinda", "motor": "navegador",
		"respostas": [][2]string{{"Qual é o nome?", "Pão de Mel"}}, "conteudo": "```json\n" + conteudoNavegador + "\n```"})
	w = pedir(s, "POST", "/_nep/assistente/criar", string(corpo), "127.0.0.1")
	var out map[string]any
	json.Unmarshal(w.Body.Bytes(), &out)
	if w.Code != 200 || out["ok"] != true || out["motor"] != "navegador" || out["nome"] != "Pão de Mel" {
		t.Fatalf("criar: %d %s", w.Code, w.Body.String())
	}
	if arq := out["arquivos"].([]any); len(arq) != 2 {
		t.Errorf("arquivos: %v", arq)
	}
	if b, _ := os.ReadFile(out["copia"].(string)); !strings.Contains(string(b), "<nepenthes>") {
		t.Error("não guardou o arquivo anterior")
	}
	novo, _ := os.ReadFile(f)
	if strings.Contains(string(novo), "<nepenthes>") {
		t.Fatal("a tag continuou no site")
	}
	// agora o site abre para todo mundo, com o texto da IA
	w = pedir(s, "GET", "/", "", "203.0.113.9")
	if w.Code != 200 || !strings.Contains(w.Body.String(), "Pão quentinho às seis da manhã") || !strings.Contains(w.Body.String(), "Pão de Mel") {
		t.Fatalf("site novo: %d %.300s", w.Code, w.Body.String())
	}
}

func TestAssistenteComQwenNoServidor(t *testing.T) {
	var sistemas []string
	fake := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var in struct {
			Messages []iaMsg `json:"messages"`
		}
		json.NewDecoder(r.Body).Decode(&in)
		sistemas = append(sistemas, in.Messages[0].Content)
		out := `{"perguntas": ["Qual o horário de funcionamento?", "Vende bolo por encomenda?"]}`
		if strings.Contains(in.Messages[0].Content, "redator") {
			out = conteudoNavegador
		}
		b, _ := json.Marshal(map[string]any{"choices": []any{map[string]any{"message": map[string]any{"role": "assistant", "content": out}}}, "usage": map[string]any{"total_tokens": 10}})
		w.Write(b)
	}))
	defer fake.Close()
	t.Setenv("NEP_IA_URL", fake.URL+"/v1")
	t.Setenv("NEP_IA_CHAVE", "teste")
	t.Setenv("NEP_IA_MODELO", "qwen-plus")

	s, f := assistenteSrv(t, "<nepenthes></nepenthes>")
	w := pedir(s, "GET", "/_nep/assistente/estado", "", "[::1]")
	if !strings.Contains(w.Body.String(), `"servidor":"qwen-plus"`) {
		t.Fatalf("estado: %s", w.Body.String())
	}
	w = pedir(s, "POST", "/_nep/assistente/perguntas", `{"descricao":"padaria","motor":"servidor"}`, "[::1]")
	if !strings.Contains(w.Body.String(), "horário de funcionamento") {
		t.Fatalf("perguntas do Qwen: %s", w.Body.String())
	}
	w = pedir(s, "POST", "/_nep/assistente/criar", `{"descricao":"padaria","motor":"servidor","respostas":[["Qual o horário?","6h às 20h"]]}`, "[::1]")
	if !strings.Contains(w.Body.String(), `"motor":"servidor"`) {
		t.Fatalf("criar: %s", w.Body.String())
	}
	if len(sistemas) != 2 || !strings.Contains(sistemas[1], "redator") {
		t.Errorf("chamadas ao Qwen: %d", len(sistemas))
	}
	if b, _ := os.ReadFile(f); !strings.Contains(string(b), "Pão quentinho") {
		t.Error("site sem o texto do Qwen")
	}
}
