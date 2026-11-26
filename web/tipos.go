package web

// Tipos de conteúdo desenhados por uma frase (como as "collections" do
// Payload, mas sem escrever configuração): "imóveis com preço, bairro,
// quartos, fotos e corretor". A IA (do servidor ou o Granite local) ou as
// regras do Nepenthes viram um <conteudo> com página de lista e página de
// item, guardado nos dados do site e somado ao arquivo quando ele é lido.

import (
	"encoding/json"
	"fmt"
	"html"
	"net/http"
	"net/url"
	"path/filepath"
	"regexp"
	"strings"
	"unicode"
)

const metaTipos = "tipos:lista"

type campoTipo struct {
	Nome        string   `json:"nome"`
	Rotulo      string   `json:"rotulo"`
	Tipo        string   `json:"tipo"`
	Obrigatorio bool     `json:"obrigatorio,omitempty"`
	Opcoes      []string `json:"opcoes,omitempty"`
	Relacao     string   `json:"relacao,omitempty"`
}

type tipoSpec struct {
	Nome   string      `json:"nome"`
	Titulo string      `json:"titulo"`
	Item   string      `json:"item"`
	Campos []campoTipo `json:"campos"`
}

type tipoGuardado struct {
	tipoSpec
	Desc  string `json:"desc,omitempty"`
	Motor string `json:"motor,omitempty"`
	Em    string `json:"em"`
	Por   string `json:"por,omitempty"`
}

var tiposDeCampo = []string{"texto", "texto_longo", "numero", "dinheiro", "data", "email", "telefone", "link", "imagem", "arquivo", "sim_nao", "escolha", "relacao"}

func (s *Server) tiposGuardados() []tipoGuardado {
	var ts []tipoGuardado
	_ = json.Unmarshal([]byte(s.Store.Meta(metaTipos)), &ts)
	return ts
}

func (s *Server) salvarTipos(ts []tipoGuardado) error {
	b, _ := json.Marshal(ts)
	return s.Store.SetMeta(metaTipos, string(b))
}

// ---------- nomes ----------

func nomeCampo(x string) string {
	n := strings.ReplaceAll(slugify(x), "-", "_")
	if n == "" {
		return ""
	}
	if n[0] >= '0' && n[0] <= '9' {
		n = "c_" + n
	}
	if len(n) > 40 {
		n = strings.TrimRight(n[:40], "_")
	}
	if reservadoLang[n] {
		n += "_"
	}
	return n
}

var reservadoLang = map[string]bool{"let": true, "fn": true, "do": true, "end": true, "if": true, "elif": true, "else": true, "for": true, "in": true,
	"while": true, "return": true, "true": true, "false": true, "nil": true, "and": true, "or": true, "not": true, "break": true, "continue": true,
	"try": true, "catch": true, "import": true, "as": true}

func maiuscula(x string) string {
	r := []rune(strings.TrimSpace(x))
	if len(r) == 0 {
		return ""
	}
	r[0] = unicode.ToUpper(r[0])
	return string(r)
}

// singular e plural do português (o bastante para nomes de tipos).
func singular(p string) string {
	l := strings.ToLower(p)
	for _, r := range [][2]string{{"ões", "ão"}, {"ães", "ão"}, {"éis", "el"}, {"eis", "el"}, {"óis", "ol"}, {"ais", "al"}, {"res", "r"}, {"zes", "z"}, {"ses", "s"}, {"ns", "m"}} {
		if strings.HasSuffix(l, r[0]) && len([]rune(l)) > len([]rune(r[0]))+1 {
			return p[:len(p)-len(r[0])] + r[1]
		}
	}
	if strings.HasSuffix(l, "s") && len(l) > 2 {
		return p[:len(p)-1]
	}
	return p
}

func plural(p string) string {
	l := strings.ToLower(p)
	switch {
	case strings.HasSuffix(l, "s"):
		return p
	case strings.HasSuffix(l, "ão"):
		return p[:len(p)-len("ão")] + "ões"
	case strings.HasSuffix(l, "el"):
		return p[:len(p)-2] + "eis"
	case strings.HasSuffix(l, "al"), strings.HasSuffix(l, "ol"):
		return p[:len(p)-1] + "is"
	case strings.HasSuffix(l, "r"), strings.HasSuffix(l, "z"):
		return p + "es"
	case strings.HasSuffix(l, "m"):
		return p[:len(p)-1] + "ns"
	}
	return p + "s"
}

// ---------- regras ----------

var (
	reAbreTipo  = regexp.MustCompile(`(?i)^(quero|preciso( de)?|gostaria( de)?|criar?|crie|fa[çc]a|cadastr(o|ar)( de)?|lista( de)?|cat[aá]logo( de)?|guardar|registr(o|ar)( de)?|controle( de)?|tipo( de)?|um|uma|uns|umas|os|as|o|a|de|do|da|dos|das|meus|minhas|nossos|nossas|novo|nova)\s+`)
	reSepCampos = regexp.MustCompile(`(?i)\s+(com|contendo|que tem|que t[eê]m|tendo|onde cada um tem|e cada um tem)\s+|:\s*`)
	reArtigo    = regexp.MustCompile(`(?i)^(o|a|os|as|um|uma|seu|sua|seus|suas|de|do|da|e|tamb[eé]m)\s+`)
	reParen     = regexp.MustCompile(`\(([^)]*)\)`)
)

type pistaTipo struct {
	re   *regexp.Regexp
	tipo string
}

var pistas = []pistaTipo{
	{regexp.MustCompile(`(?i)pre[çc]o|valor|custo|sal[aá]rio|mensalidade|taxa|aluguel|or[çc]amento|reais`), "dinheiro"},
	{regexp.MustCompile(`(?i)foto|imagem|capa|logo|retrato|banner|galeria`), "imagem"},
	{regexp.MustCompile(`(?i)e-?mail`), "email"},
	{regexp.MustCompile(`(?i)telefone|whats|celular|fone`), "telefone"},
	{regexp.MustCompile(`(?i)site|link|url|endere[çc]o (web|eletr)|instagram|facebook|youtube|v[ií]deo`), "link"},
	{regexp.MustCompile(`(?i)^data|nascimento|prazo|vencimento|quando|dia d[aoe]`), "data"},
	{regexp.MustCompile(`(?i)descri[çc][aã]o|texto|hist[oó]ria|biografia|bio|resumo|observa[çc]|detalhes|sobre|conte[uú]do|receita|modo de preparo|ingredientes|corpo`), "texto_longo"},
	{regexp.MustCompile(`(?i)quantidade|quartos|banheiros|vagas|n[uú]mero de|idade|ano|estoque|metros|m2|m²|[aá]rea|peso|altura|nota|pontos|dura[çc][aã]o|minutos|horas|lugares|capacidade|por[çc][oõ]es`), "numero"},
	{regexp.MustCompile(`(?i)pdf|documento|arquivo|anexo|card[aá]pio em|contrato|curr[ií]culo`), "arquivo"},
	{regexp.MustCompile(`(?i)^(destaque|ativo|dispon[ií]vel|vendido|aceita|tem |possui|é |pet)`), "sim_nao"},
}

// tipoPorRegras entende frases como "imóveis com preço, bairro, quartos,
// fotos, situação (à venda, vendido) e corretor".
func tipoPorRegras(desc string, site *Site) tipoSpec {
	d := strings.TrimSpace(strings.TrimRight(strings.TrimSpace(desc), "."))
	cabeca, cauda := d, ""
	if loc := reSepCampos.FindStringIndex(d); loc != nil {
		cabeca, cauda = d[:loc[0]], d[loc[1]:]
	}
	for i := 0; i < 6; i++ {
		c := reAbreTipo.ReplaceAllString(cabeca, "")
		if c == cabeca {
			break
		}
		cabeca = c
	}
	palavra := strings.Fields(cabeca)
	nome := "itens"
	if len(palavra) > 0 {
		nome = strings.Trim(palavra[0], ",.;:!?\"'")
		if len(palavra) > 1 && len([]rune(palavra[0])) <= 3 { // "pet shop", "ex alunos"
			nome = palavra[0] + " " + palavra[1]
		}
	}
	nome = plural(strings.ToLower(nome))
	sp := tipoSpec{Nome: nome, Titulo: maiuscula(nome), Item: singular(nome)}
	// guarda as opções entre parênteses antes de separar por vírgula
	var grupos []string
	cauda = reParen.ReplaceAllStringFunc(cauda, func(m string) string {
		grupos = append(grupos, m[1:len(m)-1])
		return fmt.Sprintf("\x00%d\x00", len(grupos)-1)
	})
	cauda = regexp.MustCompile(`(?i)\s+e\s+`).ReplaceAllString(cauda, ", ")
	for _, parte := range strings.FieldsFunc(cauda, func(r rune) bool { return r == ',' || r == ';' || r == '\n' }) {
		parte = strings.TrimSpace(parte)
		var opcoes []string
		if m := regexp.MustCompile("\x00(\\d+)\x00").FindStringSubmatch(parte); m != nil {
			var i int
			fmt.Sscanf(m[1], "%d", &i)
			for _, o := range strings.FieldsFunc(regexp.MustCompile(`(?i)\s+(ou|e)\s+`).ReplaceAllString(grupos[i], ","), func(r rune) bool { return r == ',' || r == '/' || r == '|' }) {
				o = strings.TrimSpace(regexp.MustCompile(`(?i)^(ou|e)\s+`).ReplaceAllString(strings.TrimSpace(o), ""))
				if o != "" {
					opcoes = append(opcoes, maiuscula(o))
				}
			}
			parte = strings.TrimSpace(strings.Replace(parte, m[0], "", 1))
		}
		for i := 0; i < 3; i++ {
			parte = reArtigo.ReplaceAllString(parte, "")
		}
		parte = strings.Trim(parte, " .!?\"'")
		if parte == "" || len([]rune(parte)) > 50 {
			continue
		}
		c := campoTipo{Rotulo: maiuscula(parte), Tipo: "texto"}
		for _, p := range pistas {
			if p.re.MatchString(parte) {
				c.Tipo = p.tipo
				break
			}
		}
		if len(opcoes) > 1 {
			c.Tipo, c.Opcoes = "escolha", opcoes
		}
		if rel := relacaoPara(parte, site); rel != "" && c.Tipo == "texto" {
			c.Tipo, c.Relacao = "relacao", rel
		}
		if strings.EqualFold(parte, "fotos") || strings.EqualFold(parte, "imagens") {
			c.Rotulo = maiuscula(singular(parte))
		}
		sp.Campos = append(sp.Campos, c)
	}
	return sp
}

// relacaoPara acha um tipo do site com o mesmo nome (corretor → corretores).
func relacaoPara(palavra string, site *Site) string {
	if site == nil {
		return ""
	}
	w := slugify(palavra)
	for _, c := range site.Contents {
		for _, cand := range []string{c.Name, slugify(c.Title), slugify(c.Item), slugify(singular(c.Title)), slugify(singular(c.Name))} {
			if cand != "" && (w == cand || slugify(plural(palavra)) == cand) {
				return c.Name
			}
		}
	}
	return ""
}

// ---------- IA ----------

func promptTipo(site *Site) string {
	var b strings.Builder
	b.WriteString(`Você desenha tipos de conteúdo para o painel de um site (como as coleções do Payload CMS).
Responda só com JSON neste formato:
{"nome":"imoveis","titulo":"Imóveis","item":"imóvel","campos":[{"nome":"titulo","rotulo":"Título","tipo":"texto","obrigatorio":true}, ...]}
Regras:
- "nome": plural, minúsculas, sem acento, sem espaço (use -).
- "item": o singular, em português, como a pessoa diz.
- tipos de campo: texto, texto_longo, numero, dinheiro, data, email, telefone, link, imagem, arquivo, sim_nao, escolha, relacao.
- "escolha" precisa de "opcoes" (lista curta de textos).
- "relacao" precisa de "relacao" com o nome de um tipo que já existe (lista abaixo).
- O primeiro campo é o título do item (titulo ou nome), obrigatório.
- Entre 3 e 12 campos. Use os campos que a pessoa pediu e acrescente só os óbvios que faltam.
- Rótulos em português do Brasil, curtos.
`)
	if site != nil && len(site.Contents) > 0 {
		b.WriteString("Tipos que já existem (para relacao):")
		for _, c := range site.Contents {
			b.WriteString(" " + c.Name + " (" + c.Item + ")")
		}
		b.WriteString("\n")
	}
	return b.String()
}

func schemaTipo() map[string]any {
	campo := map[string]any{"type": "object", "required": []string{"nome", "rotulo", "tipo"}, "properties": map[string]any{
		"nome": map[string]any{"type": "string"}, "rotulo": map[string]any{"type": "string"},
		"tipo":        map[string]any{"type": "string", "enum": tiposDeCampo},
		"obrigatorio": map[string]any{"type": "boolean"},
		"opcoes":      map[string]any{"type": "array", "items": map[string]any{"type": "string"}, "maxItems": 12},
		"relacao":     map[string]any{"type": "string"}}}
	return map[string]any{"type": "object", "required": []string{"nome", "titulo", "item", "campos"}, "properties": map[string]any{
		"nome": map[string]any{"type": "string"}, "titulo": map[string]any{"type": "string"}, "item": map[string]any{"type": "string"},
		"campos": map[string]any{"type": "array", "minItems": 2, "maxItems": 14, "items": campo}}}
}

var reJSONObj = regexp.MustCompile(`(?s)\{.*\}`)

func lerTipoIA(txt string) (tipoSpec, bool) {
	var sp tipoSpec
	m := reJSONObj.FindString(txt)
	if m == "" || json.Unmarshal([]byte(m), &sp) != nil || len(sp.Campos) == 0 {
		return sp, false
	}
	return sp, true
}

// desenharTipo usa a IA escolhida e cai nas regras se ela falhar.
func (s *Server) desenharTipo(rq *request, desc, motor string, site *Site) (tipoSpec, string, string) {
	var txt string
	var err error
	switch {
	case motor == "servidor" && s.iaConf().ready():
		txt, err = s.ask("tipos", promptTipo(site), desc, 900, 0.3)
	case motor == "local":
		txt, err = s.askLocal(rq, schemaTipo(), promptTipo(site), desc, 900, 0.3)
	default:
		return tipoPorRegras(desc, site), "regras", ""
	}
	if err == nil {
		if sp, ok := lerTipoIA(txt); ok {
			return sp, motor, ""
		}
		return tipoPorRegras(desc, site), "regras", "A IA respondeu fora do formato; montei pelas regras."
	}
	return tipoPorRegras(desc, site), "regras", "A IA não respondeu (" + err.Error() + "); montei pelas regras."
}

// limparTipo deixa o desenho seguro e completo, venha de onde vier.
func limparTipo(sp tipoSpec, site *Site) tipoSpec {
	nome := slugify(sp.Nome)
	if nome == "" {
		nome = slugify(sp.Titulo)
	}
	if nome == "" || !ValidCol(nome) || nome[0] < 'a' || nome[0] > 'z' {
		nome = "itens"
	}
	if len(nome) > 40 {
		nome = strings.TrimRight(nome[:40], "-")
	}
	switch nome {
	case "api", "admin", "globais", "globals", "tipos":
		nome += "-site"
	}
	sp.Nome = nome
	sp.Titulo = limparRotulo(sp.Titulo, 60)
	if sp.Titulo == "" {
		sp.Titulo = maiuscula(strings.ReplaceAll(nome, "-", " "))
	}
	sp.Item = limparRotulo(strings.ToLower(sp.Item), 40)
	if sp.Item == "" {
		sp.Item = strings.ToLower(singular(sp.Titulo))
	}
	tipos := map[string]bool{}
	for _, t := range tiposDeCampo {
		tipos[t] = true
	}
	visto := map[string]bool{"id": true, "slug": true, "publicado": true, "created_at": true, "updated_at": true, schedField: true}
	var cs []campoTipo
	for _, c := range sp.Campos {
		c.Rotulo = limparRotulo(c.Rotulo, 50)
		n := nomeCampo(c.Nome)
		if n == "" {
			n = nomeCampo(c.Rotulo)
		}
		if n == "" {
			continue
		}
		for i := 2; visto[n]; i++ {
			n = fmt.Sprintf("%s_%d", strings.TrimRight(n, "_0123456789"), i)
		}
		visto[n] = true
		c.Nome = n
		if c.Rotulo == "" {
			c.Rotulo = maiuscula(strings.ReplaceAll(n, "_", " "))
		}
		c.Tipo = strings.ToLower(strings.TrimSpace(c.Tipo))
		if !tipos[c.Tipo] {
			c.Tipo = "texto"
		}
		var ops []string
		for _, o := range c.Opcoes {
			if o = maiuscula(limparRotulo(o, 40)); o != "" && len(ops) < 12 {
				ops = append(ops, o)
			}
		}
		c.Opcoes = ops
		if c.Tipo == "escolha" && len(ops) < 2 {
			c.Tipo, c.Opcoes = "texto", nil
		}
		if c.Tipo != "escolha" {
			c.Opcoes = nil
		}
		if c.Tipo == "relacao" {
			rel := ""
			if site != nil && site.Content(c.Relacao) != nil {
				rel = c.Relacao
			} else {
				rel = relacaoPara(c.Relacao, site)
				if rel == "" {
					rel = relacaoPara(c.Rotulo, site)
				}
			}
			if rel == "" || rel == nome {
				c.Tipo, c.Relacao = "texto", ""
				if strings.HasPrefix(n, "relac") { // campo "relação" genérico, sem tipo para ligar
					continue
				}
			} else {
				c.Relacao = rel
			}
		} else {
			c.Relacao = ""
			if strings.HasPrefix(n, "relac") { // a IA às vezes inventa uma "relação" que é só uma lista
				continue
			}
		}
		cs = append(cs, c)
		if len(cs) == 20 {
			break
		}
	}
	tit := -1
	for i, c := range cs {
		if (c.Nome == "titulo" || c.Nome == "nome" || c.Nome == "title") && c.Tipo == "texto" {
			tit = i
			break
		}
	}
	if tit < 0 {
		n := "titulo"
		if visto["titulo"] {
			n = "titulo_item"
		}
		cs = append([]campoTipo{{Nome: n, Rotulo: "Título", Tipo: "texto", Obrigatorio: true}}, cs...)
	} else {
		cs[tit].Obrigatorio = true
		if strings.EqualFold(cs[tit].Rotulo, sp.Titulo) || strings.EqualFold(cs[tit].Rotulo, sp.Item) {
			cs[tit].Rotulo = maiuscula(strings.ReplaceAll(cs[tit].Nome, "titulo", "título"))
		}
		cs = append(append([]campoTipo{cs[tit]}, cs[:tit]...), cs[tit+1:]...)
	}
	sp.Campos = cs
	return sp
}

func limparRotulo(x string, max int) string {
	x = strings.Join(strings.Fields(strings.NewReplacer("{", "", "}", "", "<", "", ">", "", "\"", "", "`", "").Replace(x)), " ")
	if r := []rune(x); len(r) > max {
		x = strings.TrimSpace(string(r[:max]))
	}
	return x
}

// ---------- fonte gerada ----------

func campoHTML(c campoTipo) string {
	e := html.EscapeString
	req := ""
	if c.Obrigatorio {
		req = " required"
	}
	n := `name="` + c.Nome + `"`
	switch c.Tipo {
	case "texto_longo":
		return `<label>` + e(c.Rotulo) + ` <textarea ` + n + req + `></textarea></label>`
	case "numero":
		return `<label>` + e(c.Rotulo) + ` <input ` + n + ` type="number" step="any"` + req + `></label>`
	case "dinheiro":
		return `<label>` + e(c.Rotulo) + ` <input ` + n + ` type="number" step="0.01" min="0"` + req + `></label>`
	case "data":
		return `<label>` + e(c.Rotulo) + ` <input ` + n + ` type="date"` + req + `></label>`
	case "email":
		return `<label>` + e(c.Rotulo) + ` <input ` + n + ` type="email"` + req + `></label>`
	case "telefone":
		return `<label>` + e(c.Rotulo) + ` <input ` + n + ` type="tel" maxlength="20"` + req + `></label>`
	case "link":
		return `<label>` + e(c.Rotulo) + ` <input ` + n + ` type="url"` + req + `></label>`
	case "imagem":
		return `<label>` + e(c.Rotulo) + ` <input ` + n + ` type="file" accept="image/*" data-publico` + req + `></label>`
	case "arquivo":
		return `<label>` + e(c.Rotulo) + ` <input ` + n + ` type="file" data-publico data-max-mb="10"` + req + `></label>`
	case "sim_nao":
		return `<label><input type="checkbox" ` + n + `> ` + e(c.Rotulo) + `</label>`
	case "escolha":
		var b strings.Builder
		b.WriteString(`<label>` + e(c.Rotulo) + ` <select ` + n + req + `><option value="">—</option>`)
		for _, o := range c.Opcoes {
			b.WriteString(`<option>` + e(o) + `</option>`)
		}
		return b.String() + `</select></label>`
	case "relacao":
		return `<label>` + e(c.Rotulo) + ` <select ` + n + ` data-relacao="` + c.Relacao + `"` + req + `><option value="">—</option></select></label>`
	}
	return `<label>` + e(c.Rotulo) + ` <input ` + n + ` maxlength="200"` + req + `></label>`
}

// valorHTML mostra o valor de um campo (v é a expressão, ex.: "item.preco").
func valorHTML(c campoTipo, v string, site *Site) string {
	switch c.Tipo {
	case "texto_longo":
		return `{{ markdown(` + v + `) }}`
	case "dinheiro":
		return `{{ money(` + v + `) }}`
	case "data":
		return `{{ date(` + v + `, "d/m/Y") }}`
	case "sim_nao":
		return `Sim`
	case "link":
		return `<a href="{{ ` + v + ` }}" target="_blank" rel="noopener">{{ ` + v + ` }}</a>`
	case "email":
		return `<a href="mailto:{{ ` + v + ` }}">{{ ` + v + ` }}</a>`
	case "imagem":
		return `<img src="{{ ` + v + `.url }}" alt="" loading="lazy">`
	case "arquivo":
		return `<a href="{{ ` + v + `.url }}" target="_blank">{{ ` + v + `.nome }}</a>`
	case "relacao":
		if site != nil {
			if rc := site.Content(c.Relacao); rc != nil {
				tf := campoTitulo(rc)
				if r := rotaItem(site, rc.Name); r != "" {
					return `<a href="{{ url('` + r + `' + ` + v + `.slug) }}">{{ ` + v + `.` + tf + ` }}</a>`
				}
				return `{{ ` + v + `.` + tf + ` }}`
			}
		}
	}
	return `{{ ` + v + ` }}`
}

func campoTitulo(c *Content) string {
	for _, k := range []string{"titulo", "título", "nome", "title", "name"} {
		for _, f := range c.Fields {
			if f.Name == k {
				return k
			}
		}
	}
	for _, f := range c.Fields {
		if f.Type != "file" && f.Type != "textarea" && f.Type != "checkbox" && isIdent(f.Name) {
			return f.Name
		}
	}
	return "id"
}

// rotaItem devolve o começo do endereço do item ("/corretores/").
func rotaItem(site *Site, col string) string {
	for _, p := range site.Pages {
		if p.Conteudo == col && len(p.params) > 0 && strings.HasSuffix(p.Route, "/:"+p.params[0]) {
			return strings.TrimSuffix(p.Route, ":"+p.params[0])
		}
	}
	return ""
}

const cssTipo = `<style>
.nep-tipo{max-width:1120px;margin:0 auto;padding:clamp(28px,5vw,64px) 20px}
.nep-tipo h1{font-size:clamp(1.8rem,4vw,2.6rem);margin:.1em 0 .5em;line-height:1.15}
.nep-tipo .nep-volta{display:inline-block;margin-bottom:.6em;text-decoration:none;opacity:.75}
.nep-tipo-grade{display:grid;gap:20px;grid-template-columns:repeat(auto-fill,minmax(260px,1fr))}
.nep-tipo-cartao{display:flex;flex-direction:column;border:1px solid rgba(127,127,127,.22);border-radius:14px;overflow:hidden;text-decoration:none;color:inherit;background:rgba(127,127,127,.04);transition:transform .15s,box-shadow .15s}
.nep-tipo-cartao:hover{transform:translateY(-2px);box-shadow:0 10px 28px rgba(0,0,0,.10)}
.nep-tipo-cartao img{width:100%;aspect-ratio:4/3;object-fit:cover;display:block}
.nep-tipo-cartao div{padding:14px 16px 16px}
.nep-tipo-cartao h3{margin:0 0 6px;font-size:1.1rem}
.nep-tipo-meta{margin:0;opacity:.75;font-size:.95rem}
.nep-tipo-item{display:grid;gap:32px;grid-template-columns:minmax(0,1.4fr) minmax(0,1fr);align-items:start}
.nep-tipo-item>img,.nep-tipo-capa{width:100%;border-radius:14px;display:block;object-fit:cover;max-height:520px}
.nep-tipo-dados{margin:0;display:grid;grid-template-columns:auto 1fr;gap:10px 18px;border-top:1px solid rgba(127,127,127,.22);padding-top:16px}
.nep-tipo-dados dt{font-weight:600;opacity:.8}
.nep-tipo-dados dd{margin:0}
.nep-tipo-dados dd img{max-width:100%;border-radius:10px}
.nep-tipo-texto{margin-top:28px;line-height:1.7;max-width:72ch}
.nep-tipo-vazio{opacity:.7}
@media (max-width:760px){.nep-tipo-item{grid-template-columns:1fr} }
</style>`

// fonteTipo escreve o <conteudo> e as duas páginas do tipo.
func fonteTipo(sp tipoSpec, site *Site) string {
	e := html.EscapeString
	var b strings.Builder
	b.WriteString("\n<!-- tipo criado em /_admin/tipos: " + e(sp.Titulo) + " -->\n")
	b.WriteString(`<conteudo nome="` + sp.Nome + `" titulo="` + e(sp.Titulo) + `" item="` + e(sp.Item) + `">` + "\n")
	for _, c := range sp.Campos {
		b.WriteString("  " + campoHTML(c) + "\n")
	}
	b.WriteString("</conteudo>\n")

	tit := sp.Campos[0].Nome
	capa := ""
	var curtos, longos, outros []campoTipo
	for _, c := range sp.Campos[1:] {
		switch {
		case c.Tipo == "imagem" && capa == "":
			capa = c.Nome
		case c.Tipo == "texto_longo":
			longos = append(longos, c)
		default:
			outros = append(outros, c)
		}
	}
	for _, tp := range []string{"dinheiro", "escolha", "data", "texto", "numero"} { // o mais útil no cartão primeiro
		for _, c := range outros {
			if c.Tipo == tp && len(curtos) < 2 {
				curtos = append(curtos, c)
			}
		}
	}
	rota := "/" + sp.Nome
	// lista
	b.WriteString(`<page route="` + rota + `" title="` + e(sp.Titulo) + `">` + "\n" + cssTipo + "\n")
	b.WriteString(`<section class="nep-tipo">` + "\n  <h1>" + e(sp.Titulo) + "</h1>\n" + `  <div class="nep-tipo-grade">` + "\n")
	b.WriteString(`    <each x in="db.all('` + sp.Nome + `')">` + "\n")
	b.WriteString(`      <a class="nep-tipo-cartao" href="{{ url('` + rota + `/' + x.slug) }}">` + "\n")
	if capa != "" {
		b.WriteString(`        <if test="x.` + capa + `"><img src="{{ x.` + capa + `.url }}" alt="" loading="lazy"></if>` + "\n")
	}
	b.WriteString(`        <div><h3>{{ x.` + tit + ` }}</h3>`)
	for _, c := range curtos {
		rot := ""
		if c.Tipo == "numero" || c.Tipo == "texto" {
			rot = e(c.Rotulo) + ": "
		}
		b.WriteString(`<if test="x.` + c.Nome + `"><p class="nep-tipo-meta">` + rot + valorHTML(c, "x."+c.Nome, site) + `</p></if>`)
	}
	b.WriteString("</div>\n      </a>\n    <else>\n")
	b.WriteString(`      <p class="nep-tipo-vazio">Nada publicado ainda.</p>` + "\n    </each>\n  </div>\n</section>\n</page>\n")
	// item
	b.WriteString(`<page route="` + rota + `/:slug" conteudo="` + sp.Nome + `">` + "\n" + cssTipo + "\n")
	b.WriteString(`<article class="nep-tipo">` + "\n" + `  <a class="nep-volta" href="{{ url('` + rota + `') }}">← ` + e(sp.Titulo) + "</a>\n")
	b.WriteString("  <h1>{{ item." + tit + " }}</h1>\n" + `  <div class="nep-tipo-item">` + "\n")
	if capa != "" {
		b.WriteString(`    <div><if test="item.` + capa + `"><img class="nep-tipo-capa" src="{{ item.` + capa + `.url }}" alt=""></if></div>` + "\n")
	}
	b.WriteString(`    <dl class="nep-tipo-dados">` + "\n")
	for _, c := range outros {
		b.WriteString(`      <if test="item.` + c.Nome + `"><dt>` + e(c.Rotulo) + `</dt><dd>` + valorHTML(c, "item."+c.Nome, site) + `</dd></if>` + "\n")
	}
	b.WriteString("    </dl>\n  </div>\n")
	for _, c := range longos {
		b.WriteString(`  <if test="item.` + c.Nome + `"><div class="nep-tipo-texto">` + valorHTML(c, "item."+c.Nome, site) + `</div></if>` + "\n")
	}
	b.WriteString("</article>\n</page>\n")
	return b.String()
}

var reConteudoNome = regexp.MustCompile(`(?is)<(?:conteudo|content)\b[^>]*\b(?:nome|name)\s*=\s*["']([^"']+)["']`)

// aplicarTipos soma ao arquivo os tipos criados pelo painel (os que o
// arquivo já declara ficam de fora: o arquivo manda).
func (s *Server) aplicarTipos(src string) string {
	ts := s.tiposGuardados()
	if len(ts) == 0 {
		return src
	}
	tem := map[string]bool{}
	for _, m := range reConteudoNome.FindAllStringSubmatch(src, -1) {
		tem[m[1]] = true
	}
	// a primeira leitura sem os tipos serve para ligar relações e rotas
	base, _ := ParseSite(filepath.Base(s.file), src)
	var bloco strings.Builder
	for _, t := range ts {
		if tem[t.Nome] {
			continue
		}
		tem[t.Nome] = true
		bloco.WriteString(fonteTipo(t.tipoSpec, base))
	}
	if bloco.Len() == 0 {
		return src
	}
	return inserirPaginas(src, bloco.String())
}

// inserirPaginas põe o trecho antes da página 404 ou depois da última </page>.
func inserirPaginas(src, bloco string) string {
	if ini, _, ok := miolo(src, "404"); ok {
		if m := strings.LastIndex(strings.ToLower(src[:ini]), "<page"); m >= 0 {
			return src[:m] + bloco + "  " + src[m:]
		}
	}
	if f := rePageFecha.FindAllStringIndex(src, -1); len(f) > 0 {
		e := f[len(f)-1][1]
		return src[:e] + "\n" + bloco + src[e:]
	}
	if i := strings.LastIndex(strings.ToLower(src), "</body>"); i >= 0 {
		return src[:i] + bloco + src[i:]
	}
	return src + bloco
}

// ---------- tela /_admin/tipos ----------

func (a *adminPage) tiposPage(post bool) error {
	r := a.rq.r
	site := a.site()
	var b strings.Builder
	b.WriteString(cmsCSS)
	desc := ""
	motor := "regras"
	if a.s.iaConf().ready() {
		motor = "servidor"
	}
	var previa *tipoSpec
	aviso, usado := "", ""
	if post {
		desc = strings.TrimSpace(r.PostForm.Get("desc"))
		if m := r.PostForm.Get("motor"); m != "" {
			motor = m
		}
		switch r.PostForm.Get("acao") {
		case "desenhar":
			if desc == "" {
				b.WriteString(errP("Escreva o que você quer guardar."))
				break
			}
			if len([]rune(desc)) > 600 {
				desc = string([]rune(desc)[:600])
			}
			sp, m, av := a.s.desenharTipo(a.rq, desc, motor, site)
			sp = limparTipo(sp, site)
			previa, usado, aviso = &sp, m, av
		case "criar":
			var sp tipoSpec
			if json.Unmarshal([]byte(r.PostForm.Get("spec")), &sp) != nil {
				b.WriteString(errP("Desenho inválido. Tente de novo."))
				break
			}
			sp = limparTipo(sp, site)
			if site != nil && site.Content(sp.Nome) != nil {
				b.WriteString(errP("Já existe um tipo chamado “" + sp.Nome + "”. Troque o nome no desenho."))
				previa = &sp
				break
			}
			teste := inserirPaginas(a.s.fonte, fonteTipo(sp, site))
			if _, err := ParseSite(filepath.Base(a.s.file), teste); err != nil {
				b.WriteString(errP("O tipo não coube no site: " + err.Error()))
				previa = &sp
				break
			}
			ts := a.s.tiposGuardados()
			ts = append(ts, tipoGuardado{tipoSpec: sp, Desc: desc, Motor: r.PostForm.Get("usado"), Em: stamp(), Por: a.who()})
			if err := a.s.salvarTipos(ts); err != nil {
				return err
			}
			a.s.recarregar()
			a.s.redirect(a.rq, a.base+"/conteudo?c="+url.QueryEscape(sp.Nome)+"&ok=1", http.StatusSeeOther)
			return nil
		case "remover":
			var ficam []tipoGuardado
			for _, t := range a.s.tiposGuardados() {
				if t.Nome != r.PostForm.Get("nome") {
					ficam = append(ficam, t)
				}
			}
			if err := a.s.salvarTipos(ficam); err != nil {
				return err
			}
			a.s.recarregar()
			a.s.redirect(a.rq, a.base+"/tipos?removido=1", http.StatusSeeOther)
			return nil
		}
	}
	if r.URL.Query().Get("removido") != "" {
		b.WriteString(`<p class="ok">Tipo tirado do site. Os itens continuam guardados: criando o tipo de novo com o mesmo nome, eles voltam.</p>`)
	}
	b.WriteString(`<p>Descreva o que o site precisa guardar e o Nepenthes cria o tipo de conteúdo inteiro: o formulário do painel, a página com a lista, a página de cada item e a API (<code>/api/nome</code>).</p>`)
	b.WriteString(`<form class="cms card" method="post" action="` + a.base + `/tipos">` + a.tok() + `<input type="hidden" name="acao" value="desenhar">` +
		`<label>O que você quer guardar? <textarea name="desc" rows="3" maxlength="600" placeholder="Ex.: imóveis com preço, bairro, quartos, foto, situação (à venda, alugado, vendido), descrição e corretor">` + esc(desc) + `</textarea></label>` +
		`<label>Quem desenha <select name="motor">`)
	for _, o := range [][2]string{{"regras", "Regras do Nepenthes (na hora)"}, {"servidor", "IA do servidor (configurada em IA)"}, {"local", "IA do Nepenthes neste computador (Granite, leva 1 a 3 min)"}} {
		sel := ""
		if o[0] == motor {
			sel = " selected"
		}
		if o[0] == "servidor" && !a.s.iaConf().ready() {
			continue
		}
		b.WriteString(`<option value="` + o[0] + `"` + sel + `>` + o[1] + `</option>`)
	}
	b.WriteString(`</select></label><p><button>Desenhar o tipo</button></p></form>`)

	if previa != nil {
		if aviso != "" {
			b.WriteString(`<p class="muted">` + esc(aviso) + `</p>`)
		}
		spec, _ := json.Marshal(previa)
		b.WriteString(`<div class="card"><h2>` + esc(previa.Titulo) + ` <span class="tag">/` + esc(previa.Nome) + `</span></h2><p class="muted">Cada ` + esc(previa.Item) + ` terá:</p>`)
		b.WriteString(`<div class="scroll"><table><tr><th>Campo</th><th>Tipo</th><th>Detalhe</th></tr>`)
		nomes := map[string]string{"texto": "texto curto", "texto_longo": "texto longo", "numero": "número", "dinheiro": "dinheiro (R$)", "data": "data", "email": "e-mail", "telefone": "telefone",
			"link": "link", "imagem": "imagem", "arquivo": "arquivo", "sim_nao": "sim ou não", "escolha": "escolha", "relacao": "ligado a outro tipo"}
		for _, c := range previa.Campos {
			det := ""
			if c.Obrigatorio {
				det = "obrigatório"
			}
			if len(c.Opcoes) > 0 {
				det = strings.TrimPrefix(det+" · "+strings.Join(c.Opcoes, ", "), " · ")
			}
			if c.Relacao != "" {
				det = strings.TrimPrefix(det+" · "+c.Relacao, " · ")
			}
			b.WriteString(`<tr><td>` + esc(c.Rotulo) + ` <code>` + esc(c.Nome) + `</code></td><td>` + esc(nomes[c.Tipo]) + `</td><td>` + esc(det) + `</td></tr>`)
		}
		b.WriteString(`</table></div>`)
		b.WriteString(`<details><summary>Ver o código Nepenthes que será somado ao site</summary><pre style="white-space:pre-wrap;font-size:12.5px;max-height:420px;overflow:auto">` + esc(fonteTipo(*previa, site)) + `</pre></details>`)
		b.WriteString(`<form method="post" action="` + a.base + `/tipos" class="row">` + a.tok() + `<input type="hidden" name="acao" value="criar">` +
			`<input type="hidden" name="desc" value="` + esc(desc) + `"><input type="hidden" name="usado" value="` + esc(usado) + `">` +
			`<input type="hidden" name="spec" value="` + esc(string(spec)) + `"><button>Criar no site</button><span class="muted">Desenhado por: ` + esc(map[string]string{"regras": "regras", "servidor": "IA do servidor", "local": "IA local (Granite)"}[usado]) + `</span></form></div>`)
	}

	if ts := a.s.tiposGuardados(); len(ts) > 0 {
		b.WriteString(`<h2 style="font-size:1.05rem;margin-top:24px">Tipos criados aqui</h2><div class="scroll"><table><tr><th>Tipo</th><th>Itens</th><th>Criado</th><th></th></tr>`)
		for _, t := range ts {
			b.WriteString(`<tr><td><a href="` + a.base + `/conteudo?c=` + esc(url.QueryEscape(t.Nome)) + `">` + esc(t.Titulo) + `</a><div class="muted">` + esc(t.Desc) + `</div></td><td>` +
				fmt.Sprint(a.s.Store.Count(t.Nome)) + `</td><td>` + esc(fmtWhen(t.Em)) + `</td><td class="row"><a href="` + esc(a.s.url("/"+t.Nome)) + `" target="_blank">Ver</a>` +
				`<a href="` + esc(a.s.url("/api/"+t.Nome)) + `" target="_blank">API</a>` +
				`<form method="post" action="` + a.base + `/tipos" onsubmit="return confirm('Tirar este tipo do site? Os itens ficam guardados.')">` + a.tok() +
				`<input type="hidden" name="acao" value="remover"><input type="hidden" name="nome" value="` + esc(t.Nome) + `"><button class="ghost">Tirar</button></form></td></tr>`)
		}
		b.WriteString(`</table></div>`)
	}
	return a.out("Tipos com IA", b.String(), true)
}
