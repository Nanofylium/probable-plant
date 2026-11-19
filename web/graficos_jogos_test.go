package web

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/Nanofylium/probable-plant/lang"
)

const siteGJ = `<!doctype html>
<html><head><title>GJ</title></head><body>
<page route="/" title="Gráficos">
  {{ grafico("barras", {Jan: 10, Fev: "12,5"}, {titulo: "Vendas", moeda: true}) }}
  {{ grafico("linhas", [{mes: "Jan", a: 1, b: 2}, {mes: "Fev", a: 3, b: 4}]) }}
  {{ grafico("pizza", [3, 5]) }}
</page>
<page route="/roleta" title="Roleta">
  {{ jogo("roleta", {premios: ["10% off", {nome: "Nada", chance: 0, cupom: false}], prefixo: "doce"}) }}
</page>
<page route="/raspa" title="Raspa">
  {{ jogo("raspadinha", {premio: "Brinde", contato: false}) }}
</page>
<page route="/memoria" title="Memória">
  {{ jogo("memoria", {premio: "Brinde", contato: false}) }}
</page>
</body></html>`

type gj struct {
	t   *testing.T
	s   *Server
	srv *httptest.Server
	c   *http.Client
}

func novoGJ(t *testing.T) *gj {
	t.Helper()
	dir := t.TempDir()
	file := filepath.Join(dir, "site.nep.html")
	os.WriteFile(file, []byte(siteGJ), 0o644)
	s, err := New(Options{SiteFile: file, Log: io.Discard})
	if err != nil {
		t.Fatal(err)
	}
	ts := httptest.NewServer(s)
	t.Cleanup(ts.Close)
	jar, _ := cookiejar.New(nil)
	return &gj{t: t, s: s, srv: ts, c: &http.Client{Jar: jar}}
}

func (g *gj) get(p string) (int, string) {
	r, err := g.c.Get(g.srv.URL + p)
	if err != nil {
		g.t.Fatal(err)
	}
	defer r.Body.Close()
	b, _ := io.ReadAll(r.Body)
	return r.StatusCode, string(b)
}

var reJogo = regexp.MustCompile(`data-jogo="([^"]+)" data-csrf="([0-9a-f]+)"`)

// abre a página do jogo e devolve o id e o token CSRF
func (g *gj) abrir(p string) (string, string) {
	_, b := g.get(p)
	m := reJogo.FindStringSubmatch(b)
	if m == nil {
		g.t.Fatalf("sem jogo em %s:\n%s", p, b)
	}
	var cfg map[string]any
	if err := json.Unmarshal([]byte(strings.ReplaceAll(m[1], "&#34;", `"`)), &cfg); err != nil {
		g.t.Fatalf("data-jogo: %v %s", err, m[1])
	}
	return cfg["id"].(string), m[2]
}

func (g *gj) jogar(csrf string, corpo map[string]any) (int, map[string]any) {
	j, _ := json.Marshal(corpo)
	rq, _ := http.NewRequest("POST", g.srv.URL+"/_nep/jogo", bytes.NewReader(j))
	rq.Header.Set("X-Nep-Csrf", csrf)
	rq.Header.Set("Content-Type", "application/json")
	r, err := g.c.Do(rq)
	if err != nil {
		g.t.Fatal(err)
	}
	defer r.Body.Close()
	var out map[string]any
	json.NewDecoder(r.Body).Decode(&out)
	return r.StatusCode, out
}

func TestMontarGrafico(t *testing.T) {
	tab := lang.NewTable()
	tab.SetStr("Jan", int64(10))
	tab.SetStr("Fev", "12,5")
	tab.SetStr("x", "não é número")
	g, err := montarGrafico("Colunas", tab, nil)
	if err != nil || g.Tipo != "barras" || len(g.Rotulos) != 2 || g.Series[0].Valores[1] != 12.5 {
		t.Fatalf("tabela: %+v %v", g, err)
	}
	r1, r2 := lang.NewTable(), lang.NewTable()
	r1.SetStr("mes", "Jan")
	r1.SetStr("a", int64(1))
	r1.SetStr("b", 2.5)
	r2.SetStr("mes", "Fev")
	r2.SetStr("a", int64(3))
	r2.SetStr("b", int64(4))
	g, err = montarGrafico("linhas", &lang.List{Items: []lang.Value{r1, r2}}, nil)
	if err != nil || len(g.Series) != 2 || g.Series[1].Nome != "b" || g.Rotulos[1] != "Fev" || g.Series[1].Valores[0] != 2.5 {
		t.Fatalf("registros: %+v %v", g, err)
	}
	g, _ = montarGrafico("rosca", &lang.List{Items: []lang.Value{r1, r2}}, nil)
	if len(g.Series) != 1 {
		t.Fatalf("rosca deve ter uma série: %+v", g)
	}
	if _, err := montarGrafico("radar", nil, nil); err == nil {
		t.Fatal("tipo desconhecido devia dar erro")
	}
	sem := lang.NewTable()
	sem.SetStr("nome", "só texto")
	if _, err := montarGrafico("barras", &lang.List{Items: []lang.Value{sem}}, nil); err == nil {
		t.Fatal("registros sem número deviam dar erro")
	}
}

func TestGraficoNaPagina(t *testing.T) {
	g := novoGJ(t)
	code, b := g.get("/")
	if code != 200 || strings.Count(b, `class="nep-grafico"`) != 3 {
		t.Fatalf("%d %s", code, b)
	}
	// a tabela de reserva traz os números, com moeda em reais
	must(t, b, "<figcaption>Vendas</figcaption>", "<td>R$ 12,50</td>", "/_nep/grafico.js", `data-echarts="/_nep/echarts.js"`)
	for _, p := range []string{"/_nep/echarts.js", "/_nep/grafico.js", "/_nep/hilo.js", "/_nep/jogos.js"} {
		r, err := g.c.Get(g.srv.URL + p)
		if err != nil || r.StatusCode != 200 || !strings.HasPrefix(r.Header.Get("Content-Type"), "text/javascript") {
			t.Fatalf("%s: %v %v", p, r, err)
		}
		r.Body.Close()
	}
}

func TestJogoRoleta(t *testing.T) {
	g := novoGJ(t)
	id, csrf := g.abrir("/roleta")
	if code, _ := g.jogar("errado", map[string]any{"id": id, "acao": "jogar"}); code != 403 {
		t.Fatalf("sem CSRF devia ser 403, veio %d", code)
	}
	if code, out := g.jogar(csrf, map[string]any{"id": id, "acao": "jogar", "nome": "Ana"}); code != 400 {
		t.Fatalf("sem contato devia recusar: %d %v", code, out)
	}
	if code, _ := g.jogar(csrf, map[string]any{"id": id, "acao": "jogar", "nome": "Ana", "contato": "ana@ex.com"}); code != 400 {
		t.Fatal("sem aceite devia recusar")
	}
	code, out := g.jogar(csrf, map[string]any{"id": id, "acao": "jogar", "nome": "Ana", "contato": "ana@ex.com", "aceite": true})
	// chance 0 no "Nada": sempre sai o cupom de 10%
	if code != 200 || out["premio"] != "10% off" || out["cupom"] != true || !regexp.MustCompile(`^DOCE-[A-Z0-9]{6}$`).MatchString(out["codigo"].(string)) {
		t.Fatalf("%d %v", code, out)
	}
	if n := g.s.Store.Count("cupons"); n != 1 {
		t.Fatalf("cupons gravados: %d", n)
	}
	if code, out := g.jogar(csrf, map[string]any{"id": id, "acao": "jogar", "nome": "Ana", "contato": "outro@ex.com", "aceite": true}); code != 429 {
		t.Fatalf("segunda jogada no dia devia ser recusada: %d %v", code, out)
	}
	if code, _ := g.jogar(csrf, map[string]any{"id": "0123456789abcdef", "acao": "jogar"}); code != 404 {
		t.Fatal("jogo inexistente devia ser 404")
	}
}

func TestJogoMemoriaTempoMinimo(t *testing.T) {
	g := novoGJ(t)
	id, csrf := g.abrir("/memoria")
	_, b := g.get("/memoria")
	if strings.Contains(b, "Brinde") {
		t.Fatal("os prêmios da memória não devem ir para a página")
	}
	code, out := g.jogar(csrf, map[string]any{"id": id, "acao": "jogar"})
	tok, _ := out["token"].(string)
	if code != 200 || len(tok) != 32 {
		t.Fatalf("%d %v", code, out)
	}
	if code, _ := g.jogar(csrf, map[string]any{"id": id, "acao": "concluir", "token": tok}); code != 400 {
		t.Fatal("concluir na hora devia ser recusado")
	}
	// volta o relógio do começo do jogo
	g.s.Store.SetMeta("jogo:tok:"+tok, id+"|"+itoa(int(time.Now().Unix()-30)))
	code, out = g.jogar(csrf, map[string]any{"id": id, "acao": "concluir", "token": tok})
	if code != 200 || out["premio"] != "Brinde" || out["codigo"] == nil {
		t.Fatalf("%d %v", code, out)
	}
	if code, _ := g.jogar(csrf, map[string]any{"id": id, "acao": "concluir", "token": tok}); code != 400 {
		t.Fatal("o mesmo token não pode valer duas vezes")
	}
}

func TestLerConfJogo(t *testing.T) {
	if _, err := lerConfJogo("bingo", nil); err == nil {
		t.Fatal("tipo desconhecido devia dar erro")
	}
	if _, err := lerConfJogo("roleta", nil); err == nil {
		t.Fatal("roleta sem prêmios devia dar erro")
	}
}
