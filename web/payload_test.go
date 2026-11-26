package web

import (
	"encoding/json"
	"html"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

const sitePayload = `<!doctype html>
<nep>
  fn antes_imovel(d, info) do
    if d.titulo == "proibido" do recusar("Título não permitido.") end
    if info.operacao == "apagar" and d.titulo == "fixo" do recusar("Este não sai.") end
    d.titulo = trim(d.titulo)
    d.codigo = "IM-" + upper(d.bairro)
    return d
  end
  fn depois_imovel(d, info) do
    db.insert("registro", {op: info.operacao, id: info.id})
  end
</nep>
<conteudo nome="corretores" titulo="Corretores" item="corretor">
  <label>Nome <input name="nome" required></label>
  <label>CRECI <input name="creci"></label>
</conteudo>
<conteudo nome="imoveis" titulo="Imóveis" item="imóvel" antes="antes_imovel" depois="depois_imovel">
  <label>Título <input name="titulo" required maxlength="80"></label>
  <label>Bairro <input name="bairro"></label>
  <label>Preço <input name="preco" type="number" step="0.01"></label>
  <label>Situação <select name="situacao"><option value="">—</option><option>À venda</option><option>Vendido</option></select></label>
  <label>Corretor <select name="corretor" data-relacao="corretores"><option value="">—</option></select></label>
  <label>Planta <input type="file" name="planta" accept="application/pdf"></label>
</conteudo>
<conteudo nome="segredos" titulo="Segredos" api="nao"><input name="x"></conteudo>
<global nome="contato" titulo="Contato">
  <label>Telefone <input name="telefone" type="tel"></label>
  <label>Horário <input name="horario"></label>
</global>
<html><head><title>{{ page.title }}</title></head><body>
<page route="/" title="Início"><p class="tel">{{ global("contato").telefone }}</p></page>
<page route="/imoveis/:slug" conteudo="imoveis"><h1>{{ item.titulo }}</h1><p class="c">{{ item.corretor.nome }}</p><p class="cod">{{ item.codigo }}</p></page>
<page route="404" title="404">nada</page>
</body></html>`

func newTCPayload(t *testing.T) *tc {
	dir := t.TempDir()
	file := filepath.Join(dir, "site.nep.html")
	os.WriteFile(file, []byte(sitePayload), 0o644)
	s, err := New(Options{SiteFile: file, Log: io.Discard})
	if err != nil {
		t.Fatal(err)
	}
	ts := httptest.NewServer(s)
	t.Cleanup(ts.Close)
	jar, _ := cookiejar.New(nil)
	return &tc{t: t, srv: ts, dir: dir, c: &http.Client{Jar: jar, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}}
}

func (c *tc) json(method, path, chave, body string) (int, map[string]any) {
	c.t.Helper()
	rq, _ := http.NewRequest(method, c.srv.URL+path, strings.NewReader(body))
	if body != "" {
		rq.Header.Set("Content-Type", "application/json")
	}
	if chave != "" {
		rq.Header.Set("Authorization", "Bearer "+chave)
	}
	r, err := http.DefaultClient.Do(rq)
	if err != nil {
		c.t.Fatal(err)
	}
	defer r.Body.Close()
	b, _ := io.ReadAll(r.Body)
	var out map[string]any
	if err := json.Unmarshal(b, &out); err != nil {
		c.t.Fatalf("%s %s: não é JSON (%d): %s", method, path, r.StatusCode, b)
	}
	return r.StatusCode, out
}

var reChaveNova = regexp.MustCompile(`nep_[0-9a-f]{48}`)

func novaChave(c *tc, tok string, escrita bool) string {
	v := url.Values{"_nep_csrf": {tok}, "acao": {"criar"}, "nome": {"teste"}}
	if escrita {
		v.Set("escrita", "on")
	}
	code, body, _ := c.post("/_admin/api", v)
	k := reChaveNova.FindString(body)
	if code != 200 || k == "" {
		c.t.Fatalf("chave: %d %s", code, body)
	}
	return k
}

func TestPayloadAPI(t *testing.T) {
	c := newTCPayload(t)
	loginAdmin(c)
	tok := c.csrf("/_admin")
	ler := novaChave(c, tok, false)
	chave := novaChave(c, tok, true)

	// sem chave não cria
	if code, out := c.json("POST", "/api/corretores", "", `{"nome":"Ana"}`); code != 401 || out["errors"] == nil {
		t.Fatalf("sem chave: %d %v", code, out)
	}
	if code, _ := c.json("POST", "/api/corretores", ler, `{"nome":"Ana"}`); code != 401 {
		t.Fatalf("chave só de leitura criou: %d", code)
	}
	if code, _ := c.json("GET", "/api/corretores", "nep_errada", ""); code != 401 {
		t.Fatalf("chave errada: %d", code)
	}
	code, out := c.json("POST", "/api/corretores", chave, `{"nome":"Ana Souza","creci":"123"}`)
	if code != 201 {
		t.Fatalf("criar corretor: %d %v", code, out)
	}
	ana := out["doc"].(map[string]any)
	if ana["slug"] != "ana-souza" || ana["publicado"] != true {
		t.Fatalf("doc: %v", ana)
	}
	// validação do HTML vale na API
	if code, out := c.json("POST", "/api/imoveis", chave, `{"bairro":"Centro"}`); code != 400 || !strings.Contains(out["erro"].(string), "Título") {
		t.Fatalf("obrigatório: %d %v", code, out)
	}
	if code, out := c.json("POST", "/api/imoveis", chave, `{"titulo":"x","situacao":"Alugado"}`); code != 400 || !strings.Contains(out["erro"].(string), "Opção inválida") {
		t.Fatalf("opção: %d %v", code, out)
	}
	if code, out := c.json("POST", "/api/imoveis", chave, `{"titulo":"x","corretor":99}`); code != 400 || !strings.Contains(out["erro"].(string), "inexistente") {
		t.Fatalf("relação: %d %v", code, out)
	}
	if code, out := c.json("POST", "/api/imoveis", chave, `{"titulo":"x","cor":"azul"}`); code != 400 || !strings.Contains(out["erro"].(string), "desconhecido") {
		t.Fatalf("campo desconhecido: %d %v", code, out)
	}
	// gancho antes recusa e muda dados
	if code, out := c.json("POST", "/api/imoveis", chave, `{"titulo":"proibido"}`); code != 400 || out["erro"] != "Título não permitido." {
		t.Fatalf("gancho recusa: %d %v", code, out)
	}
	for i, j := range []string{
		`{"titulo":"  Casa no Centro ","bairro":"centro","preco":450000,"situacao":"À venda","corretor":"ana-souza"}`,
		`{"titulo":"Apto Jardim","bairro":"jardim","preco":"320000.50","corretor":1}`,
		`{"titulo":"Sítio","bairro":"rural","preco":900000,"_status":"draft"}`,
		`{"titulo":"fixo","bairro":"centro","preco":100}`,
	} {
		if code, out := c.json("POST", "/api/imoveis", chave, j); code != 201 {
			t.Fatalf("imóvel %d: %d %v", i, code, out)
		}
	}
	code, out = c.json("GET", "/api/imoveis/casa-no-centro", "", "")
	if code != 200 || out["titulo"] != "Casa no Centro" || out["codigo"] != "IM-CENTRO" {
		t.Fatalf("gancho mudou? %d %v", code, out)
	}
	if cor, ok := out["corretor"].(map[string]any); !ok || cor["nome"] != "Ana Souza" {
		t.Fatalf("depth=1: %v", out["corretor"])
	}
	if _, out := c.json("GET", "/api/imoveis/casa-no-centro?depth=0", "", ""); out["corretor"] != float64(1) {
		t.Fatalf("depth=0: %v", out["corretor"])
	}
	// lista pública: sem rascunho; filtros; ordem; páginas
	_, out = c.json("GET", "/api/imoveis?sort=preco", "", "")
	if out["totalDocs"] != float64(3) {
		t.Fatalf("rascunho apareceu: %v", out)
	}
	docs := out["docs"].([]any)
	if docs[0].(map[string]any)["titulo"] != "fixo" || docs[2].(map[string]any)["titulo"] != "Casa no Centro" {
		t.Fatalf("ordem: %v", docs)
	}
	_, out = c.json("GET", "/api/imoveis?where[preco][menor]=400000&where[bairro][contem]=JARD", "", "")
	if out["totalDocs"] != float64(1) {
		t.Fatalf("filtro: %v", out)
	}
	_, out = c.json("GET", "/api/imoveis?where[preco][greater_than]=200000&limit=1&page=2&sort=-preco", "", "")
	if out["totalDocs"] != float64(2) || out["totalPages"] != float64(2) || out["hasPrevPage"] != true || out["hasNextPage"] != false ||
		out["docs"].([]any)[0].(map[string]any)["titulo"] != "Apto Jardim" {
		t.Fatalf("páginas: %v", out)
	}
	if code, _ := c.json("GET", "/api/imoveis?where[nada]=1", "", ""); code != 200 {
		t.Fatalf("filtro ruim: %d", code)
	}
	if _, out := c.json("GET", "/api/imoveis", ler, ""); out["totalDocs"] != float64(4) {
		t.Fatalf("chave vê rascunho: %v", out["totalDocs"])
	}
	if code, _ := c.json("GET", "/api/imoveis/sitio", "", ""); code != 404 {
		t.Fatalf("rascunho público: %d", code)
	}
	// PATCH muda só o que veio
	code, out = c.json("PATCH", "/api/imoveis/apto-jardim", chave, `{"situacao":"Vendido"}`)
	if d, _ := out["doc"].(map[string]any); code != 200 || d["situacao"] != "Vendido" || d["preco"] != 320000.5 || d["bairro"] != "jardim" {
		t.Fatalf("patch: %d %v", code, out)
	}
	// DELETE: gancho pode impedir; o resto vai para a lixeira
	if code, out := c.json("DELETE", "/api/imoveis/fixo", chave, ""); code != 400 || out["erro"] != "Este não sai." {
		t.Fatalf("delete recusado: %d %v", code, out)
	}
	if code, _ := c.json("DELETE", "/api/imoveis/apto-jardim", chave, ""); code != 200 {
		t.Fatalf("delete: %d", code)
	}
	if code, _ := c.json("GET", "/api/imoveis/apto-jardim", "", ""); code != 404 {
		t.Fatalf("apagado ainda aparece: %d", code)
	}
	// depois= registrou as operações
	if _, out := c.json("GET", "/api", "", ""); len(out["colecoes"].([]any)) != 2 || len(out["globais"].([]any)) != 1 {
		t.Fatalf("índice: %v", out)
	}
	_, body, _ := c.get("/_admin/conteudo?c=imoveis")
	if !strings.Contains(body, "Casa no Centro") {
		t.Fatal("painel não lista")
	}
	// tipo com api="nao" não tem API
	if code, _, _ := anon(c).get("/api/segredos"); code != 404 {
		t.Fatalf("api=nao: %d", code)
	}
	// página do item traz a relação e o que o gancho gravou
	_, body, _ = anon(c).get("/imoveis/casa-no-centro")
	if !strings.Contains(body, `<p class="c">Ana Souza</p>`) || !strings.Contains(body, "IM-CENTRO") {
		t.Fatalf("página do item: %s", body)
	}
	// revogar a chave
	_, body, _ = c.get("/_admin/api")
	ids := regexp.MustCompile(`name="id" value="([0-9a-f]+)"`).FindAllStringSubmatch(body, -1)
	for _, m := range ids {
		c.post("/_admin/api", url.Values{"_nep_csrf": {tok}, "acao": {"revogar"}, "id": {m[1]}})
	}
	if code, _ := c.json("POST", "/api/corretores", chave, `{"nome":"Bia"}`); code != 401 {
		t.Fatalf("chave revogada ainda vale: %d", code)
	}
}

func TestPayloadPainel(t *testing.T) {
	c := newTCPayload(t)
	loginAdmin(c)
	tok := c.csrf("/_admin")
	// relação no formulário do painel
	code, _, _ := c.multipart("/_admin/conteudo/editar?c=corretores", map[string]string{"_nep_csrf": tok, "nome": "Rui Lima", "_publicado": "on"}, nil)
	if code != 303 {
		t.Fatalf("corretor: %d", code)
	}
	_, body, _ := c.get("/_admin/conteudo/editar?c=imoveis")
	if !strings.Contains(body, `<option value="1">Rui Lima</option>`) {
		t.Fatalf("opções da relação: %s", body)
	}
	code, body, _ = c.multipart("/_admin/conteudo/editar?c=imoveis", map[string]string{"_nep_csrf": tok, "titulo": "proibido", "_publicado": "on"}, nil)
	if code != 200 || !strings.Contains(body, "Título não permitido.") {
		t.Fatalf("gancho no painel: %d", code)
	}
	code, _, _ = c.multipart("/_admin/conteudo/editar?c=imoveis", map[string]string{"_nep_csrf": tok, "titulo": "Loja", "bairro": "sul", "corretor": "1", "_publicado": "on"}, nil)
	if code != 303 {
		t.Fatalf("imóvel no painel: %d", code)
	}
	_, body, _ = anon(c).get("/imoveis/loja")
	if !strings.Contains(body, "Rui Lima") || !strings.Contains(body, "IM-SUL") {
		t.Fatalf("item: %s", body)
	}
	// global
	code, _, _ = c.multipart("/_admin/global?g=contato", map[string]string{"_nep_csrf": tok, "telefone": "(11) 4000-1234", "horario": "9h às 18h"}, nil)
	if code != 303 {
		t.Fatalf("global: %d", code)
	}
	_, body, _ = anon(c).get("/")
	if !strings.Contains(body, `<p class="tel">(11) 4000-1234</p>`) {
		t.Fatalf("global na página: %s", body)
	}
	if _, out := c.json("GET", "/api/globais/contato", "", ""); out["horario"] != "9h às 18h" {
		t.Fatalf("global na API: %v", out)
	}
	_, body, _ = c.get("/_admin")
	if !strings.Contains(body, "/_admin/global?g=contato") || !strings.Contains(body, "/_admin/tipos") {
		t.Fatal("menu do painel")
	}
}

func TestTiposPorRegras(t *testing.T) {
	site, err := ParseSite("t.nep.html", `<conteudo nome="corretores" item="corretor"><input name="nome"></conteudo>`)
	if err != nil {
		t.Fatal(err)
	}
	sp := limparTipo(tipoPorRegras("Quero cadastrar imóveis com preço, bairro, quartos, foto, situação (à venda, alugado ou vendido), descrição e corretor.", site), site)
	if sp.Nome != "imoveis" || sp.Item != "imóvel" {
		t.Fatalf("nome: %+v", sp)
	}
	quer := map[string]string{"titulo": "texto", "preco": "dinheiro", "bairro": "texto", "quartos": "numero", "foto": "imagem", "situacao": "escolha", "descricao": "texto_longo", "corretor": "relacao"}
	if len(sp.Campos) != len(quer) {
		t.Fatalf("campos: %+v", sp.Campos)
	}
	for _, c := range sp.Campos {
		if quer[c.Nome] != c.Tipo {
			t.Errorf("%s: %s (quero %s)", c.Nome, c.Tipo, quer[c.Nome])
		}
		if c.Nome == "situacao" && strings.Join(c.Opcoes, "|") != "À venda|Alugado|Vendido" {
			t.Errorf("opções: %v", c.Opcoes)
		}
		if c.Nome == "corretor" && c.Relacao != "corretores" {
			t.Errorf("relação: %v", c.Relacao)
		}
	}
	if sp.Campos[0].Nome != "titulo" || !sp.Campos[0].Obrigatorio {
		t.Errorf("título primeiro: %+v", sp.Campos[0])
	}
	for frase, nome := range map[string]string{"receitas: ingredientes, modo de preparo, tempo em minutos, porções e foto": "receitas",
		"lista de eventos com data, local e preço do ingresso": "eventos", "professor com nome, e-mail e telefone": "professores"} {
		if sp := limparTipo(tipoPorRegras(frase, nil), nil); sp.Nome != nome || len(sp.Campos) < 3 {
			t.Errorf("%q → %+v", frase, sp)
		}
	}
	// o que vier da IA é limpo: nomes, tipos e relações que não existem
	sp = limparTipo(tipoSpec{Nome: "Ádmin", Campos: []campoTipo{{Nome: "end", Rotulo: "Fim {{x}}", Tipo: "foo"}, {Nome: "dono", Tipo: "relacao", Relacao: "nada"}, {Nome: "end", Tipo: "escolha", Opcoes: []string{"só"}}}}, site)
	if sp.Nome != "admin-site" || sp.Campos[0].Nome != "titulo" || sp.Campos[1].Nome != "end_" || sp.Campos[1].Tipo != "texto" || strings.Contains(sp.Campos[1].Rotulo, "{") ||
		sp.Campos[2].Tipo != "texto" || sp.Campos[3].Nome != "end_2" || sp.Campos[3].Tipo != "texto" {
		t.Fatalf("limpeza: %+v", sp)
	}
	sp = limparTipo(tipoSpec{Nome: "casas", Titulo: "Casas", Campos: []campoTipo{{Nome: "titulo", Rotulo: "Casas", Tipo: "texto"}, {Nome: "relacao", Rotulo: "Relacionamento", Tipo: "escolha", Opcoes: []string{"a", "b"}}}}, site)
	if len(sp.Campos) != 1 || sp.Campos[0].Rotulo != "Título" {
		t.Fatalf("relação inventada: %+v", sp.Campos)
	}
	if _, err := ParseSite("t.nep.html", `<html><body>`+fonteTipo(limparTipo(tipoPorRegras("imóveis com preço, foto, situação (a, b), descrição e corretor", site), site), site)+`</body></html>`+
		`<conteudo nome="corretores" item="corretor"><input name="nome"></conteudo>`); err != nil {
		t.Fatalf("fonte gerada não compila: %v", err)
	}
}

func TestTiposNoPainel(t *testing.T) {
	c := newTCPayload(t)
	loginAdmin(c)
	tok := c.csrf("/_admin")
	code, body, _ := c.post("/_admin/tipos", url.Values{"_nep_csrf": {tok}, "acao": {"desenhar"}, "motor": {"regras"},
		"desc": {"Receitas com foto, tempo de preparo em minutos, dificuldade (fácil, média, difícil), ingredientes e preço"}})
	if code != 200 || !strings.Contains(body, "Criar no site") {
		t.Fatalf("desenhar: %d %s", code, body)
	}
	spec := regexp.MustCompile(`name="spec" value="([^"]+)"`).FindStringSubmatch(body)
	if spec == nil {
		t.Fatal("sem spec")
	}
	code, _, h := c.post("/_admin/tipos", url.Values{"_nep_csrf": {tok}, "acao": {"criar"}, "spec": {html.UnescapeString(spec[1])}})
	if code != 303 || !strings.Contains(h.Get("Location"), "c=receitas") {
		_, b2, _ := c.post("/_admin/tipos", url.Values{"_nep_csrf": {tok}, "acao": {"criar"}, "spec": {html.UnescapeString(spec[1])}})
		t.Fatalf("criar: %d %v\n%s", code, h.Get("Location"), regexp.MustCompile(`(?s)<p class="err">.*?</p>`).FindString(b2))
	}
	_, body, _ = c.get("/_admin/conteudo/editar?c=receitas")
	if !strings.Contains(body, `name="dificuldade"`) || !strings.Contains(body, `type="file"`) {
		t.Fatalf("formulário do tipo novo: %s", body)
	}
	code, body, _ = c.multipart("/_admin/conteudo/editar?c=receitas", map[string]string{"_nep_csrf": tok, "titulo": "Bolo de fubá", "dificuldade": "Fácil",
		"tempo_de_preparo_em_minutos": "50", "preco": "12.5", "ingredientes": "**fubá**, ovos", "_publicado": "on"}, map[string][]byte{"foto": pngBytes()})
	if code != 303 {
		t.Fatalf("salvar receita: %d %s", code, body)
	}
	_, body, _ = anon(c).get("/receitas")
	if !strings.Contains(body, "Bolo de fubá") || !strings.Contains(body, "nep-tipo-cartao") || !strings.Contains(body, "R$ 12,50") {
		t.Fatalf("lista: %s", body)
	}
	_, body, _ = anon(c).get("/receitas/bolo-de-fuba")
	if !strings.Contains(body, "<strong>fubá</strong>") || !strings.Contains(body, "Fácil") || !strings.Contains(body, "<img") {
		t.Fatalf("item: %s", body)
	}
	if _, out := c.json("GET", "/api/receitas", "", ""); out["totalDocs"] != float64(1) {
		t.Fatalf("api do tipo novo: %v", out)
	}
	// tirar do site: os itens ficam guardados
	c.post("/_admin/tipos", url.Values{"_nep_csrf": {tok}, "acao": {"remover"}, "nome": {"receitas"}})
	if code, _, _ := anon(c).get("/receitas"); code != 404 {
		t.Fatalf("tipo tirado ainda aparece: %d", code)
	}
}
