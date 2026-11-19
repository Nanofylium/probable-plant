package gerador

// Com IA (Qwen): as regras de Entender acham o ramo e as seções; a IA escreve
// o conteúdo do site (nome, frases da capa, serviços, perguntas, depoimentos,
// cor e estilo), então cada pedido sai com um site diferente. A IA só devolve
// um JSON; quem monta o .nep.html continua sendo o gerador, que garante o
// visual profissional e o código que funciona. Tudo o que vem da IA é
// conferido: tamanho, cor em hex, ramo e seções que existem.

import (
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"
)

// Conteudo é o que a IA escreve. Campos vazios ficam com o padrão do ramo.
type Conteudo struct {
	Nome       string `json:"nome"`
	Ramo       string `json:"ramo"`
	Cidade     string `json:"cidade"`
	Cor        string `json:"cor"`
	Estilo     string `json:"estilo"`
	Chapeu     string `json:"chapeu"`
	Titulo     string `json:"titulo"`
	Texto      string `json:"texto"`
	Chamada    string `json:"chamada"`
	ServTitulo string `json:"servicos_titulo"`
	ServIntro  string `json:"servicos_intro"`
	Servicos   []struct {
		Icone  string `json:"icone"`
		Titulo string `json:"titulo"`
		Texto  string `json:"texto"`
		Preco  any    `json:"preco"`
	} `json:"servicos"`
	Numeros []struct {
		Numero any    `json:"numero"`
		Rotulo string `json:"rotulo"`
	} `json:"numeros"`
	Faq []struct {
		Pergunta string `json:"pergunta"`
		Resposta string `json:"resposta"`
	} `json:"faq"`
	Depoimentos []struct {
		Texto string `json:"texto"`
		Autor string `json:"autor"`
	} `json:"depoimentos"`
	Equipe []struct {
		Nome  string `json:"nome"`
		Cargo string `json:"cargo"`
		Texto string `json:"texto"`
	} `json:"equipe"`
	Posts []struct {
		Titulo string `json:"titulo"`
		Resumo string `json:"resumo"`
	} `json:"posts"`
	Secoes []string `json:"secoes"`
}

// Ramos lista os ramos que a IA pode escolher (id e rótulo).
func Ramos() [][2]string {
	r := [][2]string{{generico.ID, generico.Rotulo}}
	for _, s := range setores {
		r = append(r, [2]string{s.ID, s.Rotulo})
	}
	return r
}

// PromptPerguntas é a instrução para a IA fazer de 2 a 4 perguntas antes de montar.
func PromptPerguntas() string {
	return `Você é o assistente do Nepenthes, que cria sites. A pessoa descreveu o site que quer.
Faça de 2 a 4 perguntas curtas, em português do Brasil, só sobre o que falta para o site ficar com a cara dela
(por exemplo: nome, cidade, público, o que mais vende, tom do texto, cores, o que não pode faltar).
Não pergunte o que ela já disse. Responda só com JSON: {"perguntas": ["...", "..."]}`
}

// PromptSite é a instrução para a IA escrever o conteúdo do site em JSON.
func PromptSite() string {
	var ramos []string
	for _, r := range Ramos() {
		ramos = append(ramos, r[0]+" ("+r[1]+")")
	}
	return `Você é o redator do Nepenthes, que cria sites profissionais. Escreva o conteúdo do site em português do Brasil,
com o jeito e os detalhes que a pessoa contou: nada genérico, nada de "Lorem ipsum", nada de "Sua Empresa".
Invente detalhes plausíveis quando faltar (preços em reais, números, depoimentos com nomes brasileiros), sem exagero.
Responda SÓ com um objeto JSON com estes campos:
{"nome": "nome do negócio", "ramo": "um destes ids: ` + strings.Join(ramos, ", ") + `",
 "cidade": "", "cor": "#RRGGBB que combine com o negócio", "estilo": "moderno|elegante|classico|divertido|acolhedor",
 "chapeu": "2 a 4 palavras acima do título", "titulo": "frase de capa com até 60 letras", "texto": "1 ou 2 frases da capa",
 "chamada": "frase da faixa final convidando a entrar em contato",
 "servicos_titulo": "", "servicos_intro": "", "servicos": [{"icone": "um emoji", "titulo": "", "texto": "uma frase", "preco": "0 se não tiver"}],
 "numeros": [{"numero": "número de destaque, como 20 ou 98%, nunca telefone", "rotulo": "anos de casa, clientes..."}], "faq": [{"pergunta": "", "resposta": ""}],
 "depoimentos": [{"texto": "", "autor": "Nome, detalhe"}], "equipe": [{"nome": "", "cargo": "", "texto": ""}],
 "posts": [{"titulo": "", "resumo": ""}],
 "secoes": ["de 2 a 5 ids, só os que o negócio precisa, desta lista: ` + strings.Join(ModulosTodos, ", ") + `"]}
Use de 3 a 4 serviços, 3 números, 3 perguntas, 3 depoimentos, 2 a 3 pessoas na equipe e 2 posts.`
}

// MensagemSite junta a descrição e as respostas para mandar à IA.
func MensagemSite(desc string, respostas [][2]string) string {
	var b strings.Builder
	b.WriteString("Pedido: " + strings.TrimSpace(desc) + "\n")
	for _, r := range respostas {
		if strings.TrimSpace(r[1]) != "" {
			b.WriteString("Pergunta: " + r[0] + "\nResposta: " + strings.TrimSpace(r[1]) + "\n")
		}
	}
	return b.String()
}

var reJSON = regexp.MustCompile(`(?s)\{.*\}`)

// LerConteudo tira o JSON da resposta da IA (que às vezes vem com ``` em volta).
func LerConteudo(txt string) (Conteudo, error) {
	var c Conteudo
	m := reJSON.FindString(txt)
	if m == "" {
		return c, errors.New("a IA não devolveu JSON")
	}
	var campos map[string]json.RawMessage
	if err := json.Unmarshal([]byte(m), &campos); err != nil {
		return c, fmt.Errorf("JSON da IA inválido: %w", err)
	}
	// campo por campo (e item por item nas listas): um campo no formato
	// errado, comum em modelos pequenos, não joga fora o resto
	for k, v := range campos {
		um := func(raw json.RawMessage) error {
			b, _ := json.Marshal(map[string]json.RawMessage{k: raw})
			return json.Unmarshal(b, &c)
		}
		if um(v) == nil {
			continue
		}
		var itens []json.RawMessage
		if json.Unmarshal(v, &itens) != nil {
			continue
		}
		var bons []json.RawMessage
		for _, it := range itens {
			var teste Conteudo
			b, _ := json.Marshal(map[string][]json.RawMessage{k: {it}})
			if json.Unmarshal(b, &teste) == nil {
				bons = append(bons, it)
			}
		}
		if b, err := json.Marshal(bons); err == nil {
			um(b)
		}
	}
	return c, nil
}

// SchemaSite é o formato da resposta em JSON Schema. Motores que aceitam
// (o llama.cpp da IA do Nepenthes) ficam presos a ele e não erram os tipos.
func SchemaSite() map[string]any {
	str := map[string]any{"type": "string"}
	obj := func(campos ...string) map[string]any {
		props := map[string]any{}
		for _, c := range campos {
			props[c] = str
		}
		return map[string]any{"type": "object", "properties": props, "required": campos}
	}
	lista := func(item map[string]any, min, max int) map[string]any {
		return map[string]any{"type": "array", "items": item, "minItems": min, "maxItems": max}
	}
	var ramos []any
	for _, r := range Ramos() {
		ramos = append(ramos, r[0])
	}
	var mods []any
	for _, m := range ModulosTodos {
		mods = append(mods, m)
	}
	props := map[string]any{
		"nome": str, "ramo": map[string]any{"type": "string", "enum": ramos}, "cidade": str,
		"cor":    map[string]any{"type": "string", "pattern": "^#[0-9a-fA-F]{6}$"},
		"estilo": map[string]any{"type": "string", "enum": []any{"moderno", "elegante", "classico", "divertido", "acolhedor"}},
		"chapeu": str, "titulo": str, "texto": str, "chamada": str, "servicos_titulo": str, "servicos_intro": str,
		"servicos":    lista(obj("icone", "titulo", "texto", "preco"), 3, 4),
		"numeros":     lista(obj("numero", "rotulo"), 3, 3),
		"faq":         lista(obj("pergunta", "resposta"), 3, 3),
		"depoimentos": lista(obj("texto", "autor"), 3, 3),
		"equipe":      lista(obj("nome", "cargo", "texto"), 2, 3),
		"posts":       lista(obj("titulo", "resumo"), 2, 2),
		"secoes":      lista(map[string]any{"type": "string", "enum": mods}, 2, 5),
	}
	var req []string
	for k := range props {
		req = append(req, k)
	}
	sort.Strings(req)
	return map[string]any{"type": "object", "properties": props, "required": req}
}

// LerPerguntas tira a lista de perguntas da resposta da IA.
func LerPerguntas(txt string) []string {
	var v struct {
		Perguntas []string `json:"perguntas"`
	}
	_ = json.Unmarshal([]byte(reJSON.FindString(txt)), &v)
	var out []string
	for _, p := range v.Perguntas {
		if p = curto(p, 160); p != "" && len(out) < 4 {
			out = append(out, p)
		}
	}
	return out
}

// PerguntasPorRegras: sem IA, pergunta o que as regras não acharam.
func PerguntasPorRegras(desc string) []string {
	p := Entender(desc)
	var q []string
	if strings.Contains(strings.Join(p.Notas, " "), "nome: não achei") {
		q = append(q, "Qual é o nome do seu negócio ou projeto?")
	}
	if p.Cidade == "" {
		q = append(q, "Em que cidade (ou bairro) você atende?")
	}
	q = append(q, "Como você quer que o site pareça: moderno, elegante, clássico, divertido ou acolhedor? E alguma cor preferida?")
	q = append(q, "Tem alguma seção que não pode faltar? (agenda, loja, cardápio, blog, galeria, equipe, planos, eventos…)")
	return q
}

// DescricaoComRespostas: sem IA, as respostas viram parte da descrição que as regras leem.
func DescricaoComRespostas(desc string, respostas [][2]string) string {
	out := strings.TrimSpace(desc)
	for _, r := range respostas {
		a := strings.TrimSpace(r[1])
		if a == "" {
			continue
		}
		q := semAcento(r[0])
		switch {
		case strings.Contains(q, "nome"):
			out += `, chamado "` + strings.Trim(a, `"“”.`) + `"`
		case strings.Contains(q, "cidade"):
			out += ", em " + strings.TrimSuffix(a, ".")
		case strings.Contains(q, "secao") || strings.Contains(q, "faltar"):
			out += ", com " + strings.TrimSuffix(a, ".")
		default:
			out += ", " + strings.TrimSuffix(a, ".")
		}
	}
	return out
}

var (
	reCorIA   = regexp.MustCompile(`^#[0-9a-fA-F]{6}$`)
	rePrecoIA = regexp.MustCompile(`\d{1,6}(\.\d{1,2})?`)
	reDigito  = regexp.MustCompile(`\d`)
)

func curto(s string, n int) string {
	s = strings.Join(strings.Fields(s), " ")
	if utf8.RuneCountInString(s) > n {
		r := []rune(s)
		s = strings.TrimSpace(string(r[:n-1])) + "…"
	}
	return s
}

func texto(v any) string {
	switch x := v.(type) {
	case string:
		return x
	case float64:
		if x == float64(int64(x)) {
			return fmt.Sprintf("%d", int64(x))
		}
		return fmt.Sprintf("%.2f", x)
	}
	return ""
}

// Aplicar põe o conteúdo da IA no plano. O que vier vazio ou fora do formato
// fica como as regras decidiram.
func Aplicar(p *Plano, c Conteudo) {
	// modelos pequenos erram o ramo e a cor com facilidade: quando a pessoa
	// já disse (e as regras acharam), vale o que ela disse
	if c.Ramo != "" && c.Ramo != p.Setor.ID && !p.RamoAchado {
		for _, s := range append([]Setor{generico}, setores...) {
			if s.ID == c.Ramo {
				p.Setor = completar(s)
				if len(p.Pedidos) == 0 && len(p.Tirados) == 0 {
					p.Modulos = append([]string(nil), p.Setor.Modulos...)
				}
				p.Notas = append(p.Notas, "ramo (IA): "+s.Rotulo)
				break
			}
		}
	}
	if n := curto(c.Nome, 60); n != "" {
		p.Nome = n
	}
	if n := curto(c.Cidade, 40); n != "" {
		p.Cidade = n
	}
	if reCorIA.MatchString(c.Cor) && !p.CorPedida {
		p.Cor = strings.ToLower(c.Cor)
	}
	if e := semAcento(c.Estilo); e != "" {
		for _, es := range estilos {
			if strings.HasPrefix(e, es.chave) || strings.HasPrefix(es.chave, e) {
				p.Fonte = es.fonte
				break
			}
		}
	}
	s := &p.Setor
	set := func(dst *string, v string, n int) {
		if v = curto(v, n); v != "" {
			*dst = v
		}
	}
	set(&s.Chapeu, c.Chapeu, 40)
	set(&s.Titulo, c.Titulo, 90)
	set(&s.Texto, c.Texto, 260)
	set(&s.Chamada, c.Chamada, 120)
	set(&s.ServRotulo, c.ServTitulo, 40)
	set(&s.ServIntro, c.ServIntro, 160)

	var serv []Item
	for _, x := range c.Servicos {
		if t := curto(x.Titulo, 50); t != "" && len(serv) < 6 {
			ic := curto(x.Icone, 4)
			if strings.IndexFunc(ic, unicode.IsLetter) >= 0 { // "loaf", "bas…": queria um emoji
				ic = ""
			}
			if ic == "" {
				ic = "✦"
			}
			// "R$ 12,50", "A partir de R$ 3,00" -> 12.50, 3.00
			preco := texto(x.Preco)
			if strings.Contains(preco, ",") { // 1.250,00
				preco = strings.ReplaceAll(strings.ReplaceAll(preco, ".", ""), ",", ".")
			}
			preco = rePrecoIA.FindString(preco)
			if strings.Trim(preco, "0.") == "" {
				preco = ""
			}
			serv = append(serv, Item{ic, t, curto(x.Texto, 160), preco})
		}
	}
	if len(serv) >= 2 {
		s.Servicos = serv
	}
	var nums [][2]string
	for _, x := range c.Numeros {
		n, r := curto(texto(x.Numero), 10), curto(x.Rotulo, 40)
		// número de destaque ("20", "+3 mil", "98%"), não telefone
		if len(reDigito.FindAllString(n, -1)) > 6 || !reDigito.MatchString(n) {
			continue
		}
		if r != "" && len(nums) < 4 {
			nums = append(nums, [2]string{n, r})
		}
	}
	if len(nums) >= 2 {
		s.Numeros = nums
	}
	var faq [][2]string
	for _, x := range c.Faq {
		if q, a := curto(x.Pergunta, 120), curto(x.Resposta, 300); q != "" && a != "" && len(faq) < 8 {
			faq = append(faq, [2]string{q, a})
		}
	}
	if len(faq) >= 2 {
		s.Faq = faq
	}
	var dep [][2]string
	for _, x := range c.Depoimentos {
		if t, a := curto(x.Texto, 240), curto(x.Autor, 60); t != "" && a != "" && len(dep) < 6 {
			dep = append(dep, [2]string{strings.Trim(t, `"“”`), a})
		}
	}
	if len(dep) >= 2 {
		s.Depo = dep
	}
	var eq []Pessoa
	for _, x := range c.Equipe {
		if n := curto(x.Nome, 50); n != "" && len(eq) < 6 {
			eq = append(eq, Pessoa{n, curto(x.Cargo, 50), curto(x.Texto, 200)})
		}
	}
	if len(eq) >= 1 {
		s.Equipe = eq
	}
	var posts []Post
	cat := "Novidades"
	if len(s.Cats) > 0 {
		cat = s.Cats[0]
	}
	for _, x := range c.Posts {
		if t := curto(x.Titulo, 90); t != "" && len(posts) < 4 {
			posts = append(posts, Post{t, cat, curto(x.Resumo, 200), ""})
		}
	}
	if len(posts) >= 1 {
		s.Posts = posts
	}

	// seções: a IA pode acrescentar às que as regras escolheram, mas o que a
	// pessoa tirou continua fora
	// quem lista quase todas as seções só copiou a lista do pedido: ignora
	if len(c.Secoes) > 0 && len(c.Secoes) <= 7 {
		quer := map[string]bool{}
		for _, m := range p.Modulos {
			quer[m] = true
		}
		tinhaPlanos := quer["planos"]
		for _, m := range c.Secoes {
			quer[strings.TrimSpace(strings.ToLower(m))] = true
		}
		// loja/cardápio já mostra os preços: planos só se a pessoa pediu
		if quer["catalogo"] && !tinhaPlanos {
			delete(quer, "planos")
		}
		tirado := map[string]bool{}
		for _, m := range p.Tirados {
			tirado[m] = true
		}
		var mods []string
		for _, m := range ModulosTodos {
			if quer[m] && !tirado[m] {
				mods = append(mods, m)
			}
		}
		p.Modulos = mods
	}

	// loja/cardápio: os itens com preço que a IA escreveu viram os produtos
	if p.Tem("catalogo") && len(serv) >= 2 {
		cat := "Destaques"
		if len(s.Produtos) > 0 && s.Produtos[0].Cat != "" {
			cat = s.Produtos[0].Cat
		}
		var prods []Produto
		for _, it := range serv {
			if v, err := strconv.ParseFloat(it.Preco, 64); err == nil && v > 0 {
				prods = append(prods, Produto{Titulo: it.Titulo, Cat: cat, Texto: it.Texto, Icone: it.Icone, Preco: v})
			}
		}
		if len(prods) >= 2 {
			s.Produtos = prods
		}
	}
	p.Notas = append(p.Notas, "textos, cor e seções escritos pela IA")
}

// UsarInstagram põe no plano o que veio do Instagram: as fotos (capa e
// galeria), o @ no rodapé e o nome do perfil quando a descrição não trouxe um.
func UsarInstagram(p *Plano, fotos []Foto, usuario, nome, site string) {
	if strings.Contains(site, "wa.me/") || strings.Contains(site, "api.whatsapp.com") {
		p.WhatsApp = linkSeguro(site)
	}
	p.Instagram = strings.TrimPrefix(strings.TrimSpace(usuario), "@")
	if nome = curto(nome, 60); nome != "" && !p.NomeAchado {
		p.Nome = nome
		p.Notas = append(p.Notas, "nome: "+nome+" (do Instagram)")
	}
	if len(fotos) == 0 {
		return
	}
	p.Fotos = fotos
	tirado := false
	for _, m := range p.Tirados {
		tirado = tirado || m == "galeria"
	}
	if len(fotos) >= 3 && !tirado && !p.Tem("galeria") {
		quer := map[string]bool{"galeria": true}
		for _, m := range p.Modulos {
			quer[m] = true
		}
		p.Modulos = nil
		for _, m := range ModulosTodos {
			if quer[m] {
				p.Modulos = append(p.Modulos, m)
			}
		}
	}
	p.Notas = append(p.Notas, fmt.Sprintf("fotos: %d do Instagram (capa e galeria)", len(fotos)))
}
