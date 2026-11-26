// Package moldes transforma um index.html com a tag <nepenthes> num site
// inteiro (blog, portal, wiki ou site estático), sem tela e sem IA:
//
//	<nepenthes tipo="portal" nome="Folha do Vale" cidade="Aveiro" email="contato@...">
//	  Portal de notícias da cidade, com agenda cultural e guia do comércio.
//	</nepenthes>
//
// O nep lê a tag, escolhe o molde pelo tipo (ou pela descrição, com
// tipo="auto") e monta o .nep.html na memória a cada mudança do arquivo. O
// arquivo da pessoa não é reescrito: ele continua sendo a fonte do site, e o
// conteúdo (posts, notícias, páginas da wiki, mensagens) fica no banco e é
// editado no painel (/_admin). Funciona igual no computador e no CGI da
// hospedagem (Mindnet), porque nada precisa ser gravado no arquivo.
//
// Os moldes vêm de modelos gratuitos do GitHub com licença MIT (Start
// Bootstrap, sobre o Bootstrap 5); os créditos estão em LICENCA-STARTBOOTSTRAP.txt.
package moldes

import (
	"bytes"
	"embed"
	"fmt"
	"html"
	"net/http"
	"path"
	"regexp"
	"strconv"
	"strings"
	"text/template"
	"time"

	"github.com/Nanofylium/probable-plant/gerador"
)

//go:embed arquivos
var arquivosFS embed.FS

//go:embed modelos/*.tmpl
var modelosFS embed.FS

//go:embed icones/*.svg
var iconesFS embed.FS

// Molde é um modelo de site.
type Molde struct {
	ID, Nome, Para, Origem, URL string
	Cor                         string // cor primária do modelo original
}

// Lista dos moldes, na ordem em que aparecem na ajuda.
var Lista = []Molde{
	{"blog", "Blog", "textos com categorias, comentários, busca e RSS", "Start Bootstrap Clean Blog", "https://github.com/StartBootstrap/startbootstrap-clean-blog", "#0085A1"},
	{"portal", "Portal", "notícias, agenda de eventos, guia, newsletter e contato", "Start Bootstrap Modern Business", "https://github.com/StartBootstrap/startbootstrap-modern-business", "#0d6efd"},
	{"wiki", "Wiki", "páginas ligadas por [[links]], índice A–Z, histórico e sugestões de edição", "Start Bootstrap Simple Sidebar", "https://github.com/StartBootstrap/startbootstrap-simple-sidebar", "#0d6efd"},
	{"estatico", "Site estático", "uma página de apresentação com serviços, depoimentos e contato", "Start Bootstrap Landing Page", "https://github.com/StartBootstrap/startbootstrap-landing-page", "#0d6efd"},
}

// Achar devolve o molde pelo id ou por um apelido (site, landing, noticias...).
func Achar(id string) (Molde, bool) {
	id = strings.ToLower(strings.TrimSpace(id))
	if a, ok := apelidos[id]; ok {
		id = a
	}
	for _, m := range Lista {
		if m.ID == id {
			return m, true
		}
	}
	return Molde{}, false
}

var apelidos = map[string]string{
	"estático": "estatico", "site": "estatico", "landing": "estatico", "pagina": "estatico", "página": "estatico",
	"institucional": "estatico", "empresa": "estatico", "noticias": "portal", "notícias": "portal", "revista": "portal",
	"jornal": "portal", "documentacao": "wiki", "documentação": "wiki", "base": "wiki", "diario": "blog", "diário": "blog",
	"clean-blog": "blog", "modern-business": "portal", "simple-sidebar": "wiki", "landing-page": "estatico",
}

// Pedido é o que veio na tag.
type Pedido struct {
	Tipo                                                                               string // id do molde; "" ou "auto" = deduzir da descrição
	Descricao                                                                          string // o texto de dentro da tag
	Nome, Slogan, Cidade, WhatsApp, Telefone, Email, Endereco, Horario, Instagram, Cor string
}

var (
	reTag  = regexp.MustCompile(`(?is)<nepenthes(\s[^>]*)?>(.*?)(?:</nepenthes>|$)`)
	reAttr = regexp.MustCompile(`([A-Za-zÀ-ÿ_-]+)\s*=\s*(?:"([^"]*)"|'([^']*)'|([^\s"'>]+))`)
	reTags = regexp.MustCompile(`(?s)<[^>]*>`)
	reCor  = regexp.MustCompile(`^#[0-9a-fA-F]{6}$`)
)

// campos: atributo da tag -> campo do pedido (com os apelidos).
var campos = map[string]string{
	"tipo": "tipo", "molde": "tipo", "modelo": "tipo",
	"nome": "nome", "titulo": "nome", "título": "nome",
	"slogan": "slogan", "frase": "slogan", "subtitulo": "slogan", "subtítulo": "slogan",
	"cidade": "cidade", "whatsapp": "whatsapp", "zap": "whatsapp",
	"telefone": "telefone", "fone": "telefone", "email": "email", "e-mail": "email",
	"endereco": "endereco", "endereço": "endereco", "horario": "horario", "horário": "horario",
	"instagram": "instagram", "cor": "cor",
}

// LerTag procura a tag <nepenthes> com atributos. Sem a tag, ou com a tag
// sem nenhum atributo conhecido (<nepenthes>ideia</nepenthes>), devolve false:
// nesse caso quem atende é o assistente, como antes.
func LerTag(src []byte) (Pedido, bool) {
	m := reTag.FindSubmatch(src)
	if m == nil || len(bytes.TrimSpace(m[1])) == 0 {
		return Pedido{}, false
	}
	var p Pedido
	achou := false
	for _, a := range reAttr.FindAllSubmatch(m[1], -1) {
		campo, ok := campos[strings.ToLower(string(a[1]))]
		if !ok {
			continue
		}
		v := string(a[2]) + string(a[3]) + string(a[4])
		v = strings.TrimSpace(html.UnescapeString(v))
		achou = true
		switch campo {
		case "tipo":
			p.Tipo = v
		case "nome":
			p.Nome = v
		case "slogan":
			p.Slogan = v
		case "cidade":
			p.Cidade = v
		case "whatsapp":
			p.WhatsApp = v
		case "telefone":
			p.Telefone = v
		case "email":
			p.Email = v
		case "endereco":
			p.Endereco = v
		case "horario":
			p.Horario = v
		case "instagram":
			p.Instagram = strings.TrimPrefix(v, "@")
		case "cor":
			p.Cor = v
		}
	}
	if !achou {
		return Pedido{}, false
	}
	d := html.UnescapeString(reTags.ReplaceAllString(string(m[2]), " "))
	p.Descricao = strings.Join(strings.Fields(d), " ")
	return p, true
}

// Deduzir escolhe o molde pela descrição (para tipo="auto" ou sem tipo).
func Deduzir(desc string) string {
	d := " " + semAcento(strings.ToLower(desc)) + " "
	tem := func(ps ...string) bool {
		for _, p := range ps {
			if strings.Contains(d, p) {
				return true
			}
		}
		return false
	}
	switch {
	case tem("wiki", "documentac", "base de conhecimento", "enciclopedia", "manual", "tutoriais"):
		return "wiki"
	case tem("portal", "noticia", "jornal", "revista", "informativo", "agenda cultural", "guia da cidade", "guia de comercio"):
		return "portal"
	case tem("blog", "diario", "artigos", "cronicas", "posts", "escrevo", "receitas"):
		return "blog"
	}
	return "estatico"
}

func semAcento(s string) string {
	return strings.NewReplacer("á", "a", "à", "a", "â", "a", "ã", "a", "é", "e", "ê", "e", "í", "i", "ó", "o", "ô", "o", "õ", "o", "ú", "u", "ç", "c").Replace(s)
}

// Expandir: se o arquivo traz a tag com atributos, devolve o .nep.html do
// site montado pelo molde. ok=false quando o arquivo não pede molde.
func Expandir(src []byte) (string, bool, error) {
	p, ok := LerTag(src)
	if !ok {
		return "", false, nil
	}
	out, err := Montar(p)
	return out, true, err
}

// dados é o que os modelos (.tmpl) recebem.
type dados struct {
	Pedido
	Molde  Molde
	Setor  gerador.Setor
	Ramo   bool // as regras reconheceram o ramo pela descrição
	Ano    int
	Rgb    string // a cor em "r, g, b"
	Zap    string // link wa.me
	Insta  string // link do Instagram
	Resumo string // a descrição numa frase curta
}

// Montar escreve o .nep.html do pedido.
func Montar(p Pedido) (string, error) {
	tipo := p.Tipo
	if tipo == "" || strings.EqualFold(tipo, "auto") {
		tipo = Deduzir(p.Descricao + " " + p.Nome)
	}
	m, ok := Achar(tipo)
	if !ok {
		ids := []string{}
		for _, x := range Lista {
			ids = append(ids, x.ID)
		}
		return "", fmt.Errorf("tipo=%q não existe; use %s ou auto", p.Tipo, strings.Join(ids, ", "))
	}
	p.Tipo = m.ID
	pl := gerador.EntenderComSemente(p.Descricao+" "+p.Nome, 1)
	d := dados{Pedido: p, Molde: m, Setor: pl.Setor, Ramo: pl.RamoAchado, Ano: time.Now().Year()}
	if d.Nome == "" {
		d.Nome = pl.Nome
		if !pl.NomeAchado {
			d.Nome = map[string]string{"blog": "Meu Blog", "portal": "Portal da Cidade", "wiki": "Nossa Wiki", "estatico": pl.Setor.Nome}[m.ID]
		}
	}
	d.Resumo = frase(p.Descricao, 150)
	if !reCor.MatchString(d.Cor) {
		d.Cor = m.Cor
		if pl.RamoAchado && reCor.MatchString(pl.Setor.Cor) {
			d.Cor = pl.Setor.Cor
		}
	}
	r, _ := strconv.ParseUint(d.Cor[1:3], 16, 8)
	g, _ := strconv.ParseUint(d.Cor[3:5], 16, 8)
	b, _ := strconv.ParseUint(d.Cor[5:7], 16, 8)
	d.Rgb = fmt.Sprintf("%d, %d, %d", r, g, b)
	if n := soDigitos(p.WhatsApp); len(n) >= 10 {
		if len(n) <= 11 {
			n = "55" + n
		}
		d.Zap = "https://wa.me/" + n
	}
	if d.Telefone == "" && d.WhatsApp != "" {
		d.Telefone = d.WhatsApp
	}
	if d.Instagram != "" {
		d.Insta = "https://instagram.com/" + url_(d.Instagram)
	}

	t := template.New(m.ID).Delims("⟦", "⟧").Funcs(funcs)
	if _, err := t.ParseFS(modelosFS, "modelos/comum.tmpl", "modelos/"+m.ID+".tmpl"); err != nil {
		return "", err
	}
	var out strings.Builder
	if err := t.ExecuteTemplate(&out, m.ID+".tmpl", d); err != nil {
		return "", err
	}
	return out.String(), nil
}

func soDigitos(s string) string {
	var b strings.Builder
	for _, r := range s {
		if r >= '0' && r <= '9' {
			b.WriteRune(r)
		}
	}
	return b.String()
}

func url_(s string) string {
	var b strings.Builder
	for _, r := range s {
		if r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || r == '.' || r == '_' {
			b.WriteRune(r)
		}
	}
	return b.String()
}

// frase: a primeira frase do texto, cortada numa palavra.
func frase(s string, max int) string {
	s = strings.TrimSpace(s)
	if i := strings.IndexAny(s, ".!?"); i > 20 {
		s = s[:i+1]
	}
	if len([]rune(s)) <= max {
		return s
	}
	rs := []rune(s)[:max]
	if i := strings.LastIndex(string(rs), " "); i > max/2 {
		return string(rs)[:i] + "…"
	}
	return string(rs) + "…"
}

// h escreve texto para o HTML do site: escapado, e sem "{" (que abriria uma
// expressão {{ }} do Nepenthes).
func h(s string) string {
	return strings.ReplaceAll(html.EscapeString(s), "{", "&#123;")
}

// lit escreve um texto como literal Nepenthes ("...").
func lit(s string) string {
	r := strings.NewReplacer(`\`, `\\`, `"`, `\"`, "\n", `\n`, "\r", "")
	return `"` + r.Replace(s) + `"`
}

var reEmail = regexp.MustCompile(`^[^@\s",;<>]+@[^@\s",;<>]+\.[a-zA-Z]{2,}$`)

var funcs = template.FuncMap{
	"h":       h,
	"emailok": func(s string) bool { return reEmail.MatchString(s) },
	"s":       lit,
	"icone": func(nome string) string {
		b, err := iconesFS.ReadFile("icones/" + nome + ".svg")
		if err != nil {
			return ""
		}
		svg := strings.TrimSpace(string(b))
		svg = strings.Replace(svg, `width="16" height="16"`, `width="1em" height="1em" aria-hidden="true"`, 1)
		return svg
	},
	"img": func(nome string) string { return "{{ url('/_nep/molde/img/" + nome + "') }}" },
	"arq": func(nome string) string { return "{{ url('/_nep/molde/" + nome + "') }}" },
	"slug": func(s string) string {
		s = semAcento(strings.ToLower(s))
		var b strings.Builder
		traco := false
		for _, r := range s {
			if r >= 'a' && r <= 'z' || r >= '0' && r <= '9' {
				b.WriteRune(r)
				traco = false
			} else if !traco && b.Len() > 0 {
				b.WriteByte('-')
				traco = true
			}
		}
		return strings.TrimSuffix(b.String(), "-")
	},
	"list": func(xs ...string) []string { return xs },
	"cats": func(d dados) []string {
		vistos := map[string]bool{}
		out := []string{}
		add := func(c string) {
			if c != "" && !vistos[c] {
				vistos[c] = true
				out = append(out, c)
			}
		}
		for _, c := range d.Setor.Cats {
			add(c)
		}
		for _, p := range d.Posts() {
			add(p.Cat)
		}
		if d.Molde.ID == "portal" {
			for _, s := range d.Setor.Servicos {
				add(s.Titulo)
			}
		}
		return out
	},
	"n":    func(i int) int { return i + 1 },
	"mod":  func(i, n int) int { return i % n },
	"pega": func(xs []string, i int) string { return xs[i%len(xs)] },
	"ate": func(n int) []int {
		r := make([]int, n)
		for i := range r {
			r[i] = i
		}
		return r
	},
	"maior": func(s, t string) string {
		if s != "" {
			return s
		}
		return t
	},
}

var tiposArquivo = map[string]string{
	".css": "text/css; charset=utf-8", ".js": "text/javascript; charset=utf-8",
	".jpg": "image/jpeg", ".png": "image/png", ".svg": "image/svg+xml",
}

// Servir entrega /_nep/molde/ARQUIVO (CSS, JS e imagens dos moldes).
func Servir(w http.ResponseWriter, r *http.Request, nome string) {
	nome = path.Clean("/" + nome)[1:]
	if nome == "" || strings.Count(nome, "/") > 1 || strings.Contains(nome, "..") {
		http.NotFound(w, r)
		return
	}
	b, err := arquivosFS.ReadFile("arquivos/" + nome)
	ct, ok := tiposArquivo[path.Ext(nome)]
	if err != nil || !ok {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", ct)
	w.Header().Set("Cache-Control", "public, max-age=604800")
	w.Write(b)
}

// Arquivo devolve um arquivo embutido (para a prévia).
func Arquivo(nome string) ([]byte, bool) {
	b, err := arquivosFS.ReadFile("arquivos/" + path.Clean("/" + nome)[1:])
	return b, err == nil
}
