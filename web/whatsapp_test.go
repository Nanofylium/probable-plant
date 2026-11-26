package web

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"testing"
)

const siteZap = `<!doctype html>
<conteudo nome="produtos" titulo="Cardápio" item="produto">
  <label>Nome <input name="titulo" required></label>
  <label>Categoria <select name="categoria" required><option value="">—</option><option>Bolos</option><option>Salgados</option></select></label>
  <label>Preço <input name="preco" type="number" step="0.01"></label>
  <label>Foto <input type="file" name="foto" accept="image/*" data-publico></label>
</conteudo>
<conteudo nome="posts" titulo="Blog"><input name="titulo"><textarea name="corpo"></textarea></conteudo>
<html><head><title>{{ page.title }}</title></head><body class="x">
<p editavel="horario">Seg a sex, 8h às 18h</p>
<page route="/" title="Início"><each p in="db.all('produtos')"><p class="p">{{ p.titulo }}|{{ p.preco }}</p></each></page>
<page route="/cardapio/:slug" conteudo="produtos"><h1>{{ item.titulo }}</h1><if test="item.foto"><img src="{{ item.foto.url }}"></if></page>
<page route="404" title="404">nada</page>
</body></html>`

func newTCZap(t *testing.T) *tc {
	t.Setenv("NEP_IA_LOCAL", t.TempDir()) // sem a IA do computador nos testes
	dir := t.TempDir()
	file := filepath.Join(dir, "site.nep.html")
	os.WriteFile(file, []byte(siteZap), 0o644)
	s, err := New(Options{SiteFile: file, Log: io.Discard})
	if err != nil {
		t.Fatal(err)
	}
	ts := httptest.NewServer(s)
	t.Cleanup(ts.Close)
	jar, _ := cookiejar.New(nil)
	return &tc{t: t, srv: ts, dir: dir, c: &http.Client{Jar: jar, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}}
}

func TestEntenderZap(t *testing.T) {
	for msg, quer := range map[string]string{
		"chegou bolo de cenoura, R$ 12":        "novo|Bolo de cenoura|12",
		"Novidade: coxinha 6,50":               "novo|Coxinha|6.5",
		"acabou o bolo de cenoura":             "acabou|Bolo de cenoura|",
		"voltou a coxinha":                     "voltou|Coxinha|",
		"brigadeiro agora R$ 5":                "preco_ou_novo|Brigadeiro|5",
		"amanhã fecho às 14h":                  "aviso||",
		"aviso: café em dobro hoje":            "aviso||",
		"tirar aviso":                          "tirar_aviso||",
		"horário: seg a sáb, 7h às 19h":        "horario||",
		"categoria: Bolos":                     "campo||",
		"desfazer":                             "desfazer||",
		"oi":                                   "ajuda||",
		"fiz pão de queijo hoje, 3 reais cada": "preco_ou_novo|Pão de queijo|3",
		"quero saber quanto vendi ontem kk":    "||",
	} {
		a := entenderZap(msg, false)
		got := a.Acao + "|" + a.Item + "|"
		if a.Preco != nil {
			got += fmtNum(*a.Preco)
		}
		if got != quer {
			t.Errorf("%q → %q, quero %q", msg, got, quer)
		}
	}
	if a := entenderZap("Pudim R$ 9", true); a.Acao != "novo" || a.Item != "Pudim" {
		t.Errorf("foto com legenda: %+v", a)
	}
}

func (c *tc) zap(tok, texto string, foto []byte) string {
	c.t.Helper()
	f := map[string][]byte{}
	if foto != nil {
		f["foto"] = foto
	}
	code, body, _ := c.multipart("/_admin/whatsapp/enviar", map[string]string{"_nep_csrf": tok, "texto": texto}, f)
	if code != 200 {
		c.t.Fatalf("simulador: %d %s", code, body)
	}
	var r map[string]string
	json.Unmarshal([]byte(body), &r)
	return r["resposta"]
}

func TestZapSimulador(t *testing.T) {
	c := newTCZap(t)
	loginAdmin(c)
	tok := c.csrf("/_admin")
	r := c.zap(tok, "chegou bolo de cenoura, R$ 12", pngBytes())
	if !strings.Contains(r, "falta Categoria (Bolos, Salgados)") {
		t.Fatalf("pede a categoria: %s", r)
	}
	_, home, _ := anon(c).get("/")
	if strings.Contains(home, "Bolo de cenoura") {
		t.Fatal("foi ao site sem categoria")
	}
	r = c.zap(tok, "categoria: bolos", nil)
	if !strings.Contains(r, "Agora está no site") || !strings.Contains(r, "/cardapio/bolo-de-cenoura") {
		t.Fatalf("categoria: %s", r)
	}
	_, home, _ = anon(c).get("/")
	if !strings.Contains(home, `<p class="p">Bolo de cenoura|12</p>`) {
		t.Fatalf("site: %s", home)
	}
	_, item, _ := anon(c).get("/cardapio/bolo-de-cenoura")
	if !strings.Contains(item, `<img src="/_nep/arquivo/p-`) {
		t.Fatalf("foto: %s", item)
	}
	if r = c.zap(tok, "bolo de cenoura agora R$ 14,50", nil); !strings.Contains(r, "R$ 14,50") {
		t.Fatalf("preço: %s", r)
	}
	if r = c.zap(tok, "acabou o bolo", nil); !strings.Contains(r, "Você quis dizer *Bolo de cenoura*") {
		t.Fatalf("nome parcial: %s", r)
	}
	if r = c.zap(tok, "acabou o bolo de cenoura", nil); !strings.Contains(r, "Tirei") {
		t.Fatalf("acabou: %s", r)
	}
	_, home, _ = anon(c).get("/")
	if strings.Contains(home, "Bolo de cenoura") {
		t.Fatal("acabou e continua no site")
	}
	if r = c.zap(tok, "desfazer", nil); !strings.Contains(r, "Desfeito") {
		t.Fatalf("desfazer: %s", r)
	}
	_, home, _ = anon(c).get("/")
	if !strings.Contains(home, "Bolo de cenoura|14.5") {
		t.Fatalf("desfazer não voltou: %s", home)
	}
	// aviso no topo, horário editável
	if r = c.zap(tok, "amanhã fecho às 14h", nil); !strings.Contains(r, "Aviso no topo") {
		t.Fatalf("aviso: %s", r)
	}
	_, home, _ = anon(c).get("/")
	if !strings.Contains(home, `<body class="x"><div class="nep-aviso"`) || !strings.Contains(home, "Amanhã fecho às 14h") {
		t.Fatalf("aviso no site: %s", home)
	}
	c.zap(tok, "tirar aviso", nil)
	if _, home, _ = anon(c).get("/"); strings.Contains(home, "nep-aviso") {
		t.Fatal("aviso não saiu")
	}
	if r = c.zap(tok, "horário: seg a sáb, 7h às 19h", nil); !strings.Contains(r, "Horário atualizado") {
		t.Fatalf("horário: %s", r)
	}
	if _, home, _ = anon(c).get("/"); !strings.Contains(home, "seg a sáb, 7h às 19h") {
		t.Fatal("horário não mudou no site")
	}
	if r = c.zap(tok, "lista", nil); !strings.Contains(r, "Bolo de cenoura — R$ 14,50") {
		t.Fatalf("lista: %s", r)
	}
	// sem IA, frase solta recebe a ajuda
	if r = c.zap(tok, "quanto vendi ontem?", nil); !strings.Contains(r, "Eu cuido do seu site") {
		t.Fatalf("ajuda: %s", r)
	}
	_, body, _ := c.get("/_admin/whatsapp")
	if !strings.Contains(body, "Bolo de cenoura") || !strings.Contains(body, "zap-form") {
		t.Fatal("conversa no painel")
	}
}

func TestZapWebhook(t *testing.T) {
	c := newTCZap(t)
	loginAdmin(c)
	tok := c.csrf("/_admin")
	var mu sync.Mutex
	var enviadas []map[string]any
	meta := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer tok-secreto" {
			w.WriteHeader(401)
			return
		}
		switch {
		case r.URL.Path == "/m1":
			json.NewEncoder(w).Encode(map[string]string{"url": "https://" + r.Host + "/baixar/m1"})
		case r.URL.Path == "/baixar/m1":
			w.Write(pngBytes())
		case strings.HasSuffix(r.URL.Path, "/messages"):
			var m map[string]any
			json.NewDecoder(r.Body).Decode(&m)
			mu.Lock()
			enviadas = append(enviadas, m)
			mu.Unlock()
			w.Write([]byte(`{"messages":[{"id":"x"}]}`))
		}
	}))
	defer meta.Close()
	antes, antesC := graphURL, clienteZap
	graphURL, clienteZap = meta.URL, meta.Client()
	defer func() { graphURL, clienteZap = antes, antesC }()

	code, _, _ := c.post("/_admin/whatsapp", url.Values{"_nep_csrf": {tok}, "acao": {"config"}, "phone_id": {"123"}, "token": {"tok-secreto"},
		"app_secret": {"segredo"}, "donos": {"+55 (71) 98888-1234"}})
	if code != 200 {
		t.Fatalf("config: %d", code)
	}
	_, painel, _ := c.get("/_admin/whatsapp")
	if strings.Contains(painel, "tok-secreto") || strings.Contains(painel, `value="segredo"`) {
		t.Fatal("o painel mostrou o segredo")
	}
	verify := regexp.MustCompile(`Token de verificação: <code>([0-9a-f]+)</code>`).FindStringSubmatch(painel)[1]
	// verificação da Meta
	if code, body, _ := anon(c).get("/_nep/whatsapp?hub.mode=subscribe&hub.verify_token=" + verify + "&hub.challenge=42"); code != 200 || body != "42" {
		t.Fatalf("verify: %d %s", code, body)
	}
	if code, _, _ := anon(c).get("/_nep/whatsapp?hub.mode=subscribe&hub.verify_token=errado&hub.challenge=42"); code != 403 {
		t.Fatalf("verify errado: %d", code)
	}
	manda := func(corpo string, segredo string) int {
		m := hmac.New(sha256.New, []byte(segredo))
		m.Write([]byte(corpo))
		rq, _ := http.NewRequest("POST", c.srv.URL+"/_nep/whatsapp", bytes.NewReader([]byte(corpo)))
		rq.Header.Set("X-Hub-Signature-256", "sha256="+hex.EncodeToString(m.Sum(nil)))
		rq.Header.Set("Content-Type", "application/json")
		r, err := http.DefaultClient.Do(rq)
		if err != nil {
			t.Fatal(err)
		}
		r.Body.Close()
		return r.StatusCode
	}
	msg := func(id, de, tipo, extra string) string {
		return `{"entry":[{"changes":[{"value":{"messages":[{"id":"` + id + `","from":"` + de + `","type":"` + tipo + `",` + extra + `}]}}]}]}`
	}
	foto := msg("w1", "557188881234", "image", `"image":{"id":"m1","caption":"Chegou pudim R$ 9"}`) // sem o nono dígito
	if code := manda(foto, "outro"); code != 401 {
		t.Fatalf("assinatura falsa aceita: %d", code)
	}
	if code := manda(foto, "segredo"); code != 200 {
		t.Fatalf("webhook: %d", code)
	}
	manda(foto, "segredo") // repetida: ignorada
	manda(msg("w2", "5511999990000", "text", `"text":{"body":"acabou o pudim"}`), "segredo")
	mu.Lock()
	n := len(enviadas)
	var txt string
	if n > 0 {
		txt = enviadas[0]["text"].(map[string]any)["body"].(string)
	}
	mu.Unlock()
	if n != 1 || !strings.Contains(txt, "Guardei *Pudim*") || enviadas[0]["to"] != "557188881234" {
		t.Fatalf("respostas: %d %v", n, enviadas)
	}
	c.zap(tok, "categoria: Bolos", nil)
	_, item, _ := anon(c).get("/cardapio/pudim")
	if !strings.Contains(item, "<h1>Pudim</h1>") || !strings.Contains(item, "/_nep/arquivo/p-") {
		t.Fatalf("pudim com foto: %s", item)
	}
}
