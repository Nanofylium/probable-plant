// Package gerador monta um site .nep.html completo a partir de uma descrição
// em português: "uma escola de música chamada Sol Maior em Salvador, com blog,
// eventos e wiki, azul e elegante".
//
// Não usa IA: lê a descrição com regras simples e previsíveis. O que a pessoa
// escreveu manda ("com X" acrescenta, "só X" deixa apenas X, "sem X" tira X);
// o que ela não disse vem do padrão do ramo (setor).
package gerador

import (
	"regexp"
	"sort"
	"strings"
	"unicode"
)

// Plano é o que o gerador entendeu da descrição.
type Plano struct {
	Descricao string
	Setor     Setor
	Nome      string
	Cidade    string
	Cor       string
	Fonte     string
	Modulos   []string // na ordem de ModulosTodos
	Idiomas   []string // códigos para a tradução (villosa), se pediu idiomas
	Notas     []string // frases curtas do que foi entendido (o comando mostra)
	Pedidos   []string // módulos que a pessoa pediu com todas as letras
	Tirados   []string // módulos que a pessoa pediu para tirar
}

// ModulosTodos, na ordem em que aparecem no site.
var ModulosTodos = []string{"servicos", "catalogo", "agenda", "eventos", "planos", "equipe", "galeria", "depoimentos",
	"blog", "wiki", "forum", "downloads", "vagas", "faq", "newsletter", "idiomas", "atendente"}

// NomeModulo é o nome para gente.
var NomeModulo = map[string]string{
	"servicos": "serviços", "catalogo": "loja / cardápio com carrinho e pedidos", "agenda": "agendamento / reservas online",
	"eventos": "eventos com inscrição e vagas", "planos": "planos e preços", "equipe": "equipe", "galeria": "galeria / portfólio",
	"depoimentos": "depoimentos", "blog": "blog (categorias, tags, arquivo, comentários)", "wiki": "wiki (links [[Página]], índice A–Z, sugestões de edição)",
	"forum": "fórum moderado", "downloads": "área de downloads", "vagas": "trabalhe conosco (vagas + currículo)", "faq": "perguntas frequentes",
	"newsletter": "newsletter", "idiomas": "tradução automática (com a IA ligada)", "atendente": "atendente virtual (lowii, com a IA ligada)",
}

// Palavras que pedem cada módulo (radicais sem acento; "x$" = palavra inteira).
var chavesModulo = map[string][]string{
	"blog":        {"blog", "noticia", "artigo", "revista", "jornal", "novidades", "posts$"},
	"servicos":    {"servico", "especialidade", "areas de atuacao", "modalidade", "acomodac"},
	"equipe":      {"equipe", "time$", "professores", "profissionais", "corpo docente", "medicos", "advogados", "diretoria", "quem somos"},
	"agenda":      {"agendamento", "agendar", "agenda online", "agenda de horarios", "reserva", "marcar", "marcacao"},
	"catalogo":    {"loja", "produto", "cardapio", "catalogo", "pedido", "carrinho", "e-commerce", "ecommerce", "delivery", "encomenda", "vender", "vendas"},
	"eventos":     {"evento", "inscric", "workshop", "palestra", "shows$", "programacao", "agenda de eventos", "agenda cultural"},
	"galeria":     {"galeria", "portfolio", "fotos", "album", "trabalhos"},
	"wiki":        {"wiki", "documentac", "base de conhecimento", "enciclopedia", "tiki"},
	"forum":       {"forum", "comunidade", "perguntas e respostas", "discuss", "debate"},
	"faq":         {"faq", "perguntas frequentes", "duvidas"},
	"depoimentos": {"depoimento", "avaliac", "testemunh", "opinio"},
	"planos":      {"plano", "precos", "tabela de preco", "mensalidade", "assinatura", "pacotes"},
	"vagas":       {"vaga", "trabalhe conosco", "curriculo", "emprego", "carreira", "recrutamento"},
	"downloads":   {"download", "arquivos", "documentos", "edital", "editais", "pdf", "transparencia", "atas$"},
	"newsletter":  {"newsletter", "boletim", "lista de e-mail", "lista de email", "inscrever no e-mail"},
	"idiomas":     {"outros idiomas", "varios idiomas", "dois idiomas", "tres idiomas", "multi-idioma", "em ingles", "em espanhol", "em frances", "em alemao", "em italiano", "bilingue", "multilingue", "traduc", "traduzid"},
	"atendente":   {"atendente", "chatbot", "chat$", "ia$", "inteligencia artificial", "assistente virtual"},
}

var codIdioma = map[string]string{"ingles": "en", "espanhol": "es", "frances": "fr", "alemao": "de", "italiano": "it", "japones": "ja", "chines": "zh", "portugues": "pt"}

var cores = []struct{ chave, hex, nome string }{
	{"verde-agua", "#0f766e", "verde-água"}, {"verde agua", "#0f766e", "verde-água"}, {"azul-marinho", "#1e3a5f", "azul-marinho"},
	{"azul marinho", "#1e3a5f", "azul-marinho"}, {"marinho", "#1e3a5f", "azul-marinho"}, {"turquesa", "#0e7490", "turquesa"},
	{"verde", "#2f6b4f", "verde"}, {"azul", "#1d4ed8", "azul"}, {"vermelh", "#b91c1c", "vermelho"}, {"vinho", "#7f1d1d", "vinho"},
	{"bordo", "#7f1d1d", "vinho"}, {"roxo", "#6d28d9", "roxo"}, {"lilas", "#8b5cf6", "lilás"}, {"violeta", "#7c3aed", "violeta"},
	{"laranja", "#c2410c", "laranja"}, {"rosa", "#be185d", "rosa"}, {"pink", "#db2777", "rosa-choque"}, {"amarel", "#b45309", "amarelo (tom escuro, para dar leitura)"},
	{"dourad", "#a16207", "dourado"}, {"preto", "#111827", "preto"}, {"grafite", "#374151", "grafite"}, {"cinza", "#4b5563", "cinza"},
	{"marrom", "#7c4a2d", "marrom"}, {"terracota", "#b4532a", "terracota"}, {"coral", "#e2574c", "coral"},
}

var estilos = []struct{ chave, fonte, nome string }{
	{"elegante", "titulo-serifa", "elegante"}, {"sofisticad", "titulo-serifa", "sofisticado"}, {"luxo", "titulo-serifa", "luxo"},
	{"classic", "classica", "clássico"}, {"tradicional", "classica", "tradicional"}, {"serio", "classica", "sério"},
	{"modern", "geometrica", "moderno"}, {"minimalist", "geometrica", "minimalista"}, {"tecnologic", "geometrica", "tecnológico"},
	{"divertid", "arredondada", "divertido"}, {"infantil", "arredondada", "infantil"}, {"alegre", "arredondada", "alegre"},
	{"descontraid", "arredondada", "descontraído"}, {"fofo", "arredondada", "fofo"}, {"acolhedor", "humanista", "acolhedor"},
	{"humano", "humanista", "humano"}, {"leve", "humanista", "leve"},
}

var tiraAcento = strings.NewReplacer("á", "a", "à", "a", "â", "a", "ã", "a", "ä", "a", "é", "e", "è", "e", "ê", "e", "ë", "e",
	"í", "i", "ì", "i", "î", "i", "ï", "i", "ó", "o", "ò", "o", "ô", "o", "õ", "o", "ö", "o", "ú", "u", "ù", "u", "û", "u", "ü", "u", "ç", "c", "ñ", "n")

// semAcento: minúsculas e sem acentos, para comparar palavras.
func semAcento(s string) string {
	return tiraAcento.Replace(strings.ToLower(s))
}

// achar devolve as posições em que o radical aparece começando uma palavra.
func achar(txt, chave string) [][]int {
	inteira := strings.HasSuffix(chave, "$")
	chave = strings.TrimSuffix(chave, "$")
	p := `(?:^|[^\p{L}\p{N}])(` + regexp.QuoteMeta(chave) + `)`
	if inteira {
		p += `(?:$|[^\p{L}\p{N}])`
	}
	var out [][]int
	for _, m := range regexp.MustCompile(p).FindAllStringSubmatchIndex(txt, -1) {
		out = append(out, []int{m[2], m[3]})
	}
	return out
}

var negacao = regexp.MustCompile(`(?:^|[^\p{L}])(sem|nao|nem|exceto|menos|tirar|tire|tirando)\s+(?:(?:o|a|os|as|de|do|da|um|uma|quero|precisa|precisa de|preciso|preciso de|ter|nenhum|nenhuma)\s+){0,3}$`)

// mencao conta quantas vezes o módulo foi pedido e quantas vezes foi negado.
func mencao(txt string, chaves []string) (sim, nao int) {
	for _, c := range chaves {
		for _, m := range achar(txt, c) {
			ini := max(0, m[0]-40)
			antes := txt[ini:m[0]]
			// a negação vale só dentro da mesma oração
			if i := strings.LastIndexAny(antes, ",.;:"); i >= 0 {
				antes = antes[i+1:]
			}
			// "sem blog e sem loja": a palavra anterior nega; "e" depois de "sem X" também ("sem blog e loja")
			if negacao.MatchString(antes) || semEncadeado(antes) {
				nao++
			} else {
				sim++
			}
		}
	}
	return
}

var encadeado = regexp.MustCompile(`(?:^|[^\p{L}])(sem|nem)\s+[\p{L}-]+(?:\s+[\p{L}-]+)?\s+(?:e|ou|nem)\s+$`)

func semEncadeado(antes string) bool { return encadeado.MatchString(antes) }

var (
	reAspas   = regexp.MustCompile(`["“”«»']([^"“”«»']{2,60})["“”«»']`)
	reChamado = regexp.MustCompile(`(?i)\b(?:chamad[oa]s?|de nome|com o nome|nomead[oa]|nome)\s*:?\s+(.{2,60}?)(?:\s+(?:em|no|na|nos|nas|com|que|para|pra|onde|e|sem|so|só|apenas|cor|de cor)\s|[,.;!?(]|$)`)
	reCidade  = regexp.MustCompile(`\b(?:em|de)\s+((?:São|Santo|Santa|Rio|Belo|Porto|Campo|Vila|Nova|Novo|Foz|Juiz|Ribeirão|Feira|Praia|[A-ZÁÉÍÓÚÂÊÔÃÕÇ])[\p{L}'-]*(?:\s+(?:do|da|de|dos|das)?\s*[A-ZÁÉÍÓÚÂÊÔÃÕÇ][\p{L}'-]*){0,3})`)
	reInicio  = regexp.MustCompile(`^(?:(?:uma?|o|a|site d[aeo]|um site d[aeo]|site para|um site para)\s+)*((?:[\p{Ll}\p{Lu}]+\s+){1,4}(?:de|do|da|dos|das)\s+\p{Lu}[\p{L}'-]*(?:\s+(?:de|do|da|dos|das)?\s*\p{Lu}[\p{L}'-]*){0,3})`)
	reHex     = regexp.MustCompile(`#[0-9a-fA-F]{6}\b`)
	reSo      = regexp.MustCompile(`(?:^|[^\p{L}])(so|somente|apenas|unicamente|so com|so quero)\s`)
)

// Entender lê a descrição e devolve o plano do site.
func Entender(desc string) *Plano {
	desc = strings.TrimSpace(desc)
	txt := semAcento(desc)
	p := &Plano{Descricao: desc}

	// ---- ramo: o setor com mais palavras-chave ganha; empate fica com o primeiro da lista
	melhor, pontos := generico, 0
	for _, s := range setores {
		n := 0
		for _, c := range s.Chaves {
			n += len(achar(txt, c))
		}
		if n > pontos {
			melhor, pontos = s, n
		}
	}
	p.Setor = completar(melhor)
	if pontos > 0 {
		p.Notas = append(p.Notas, "ramo: "+p.Setor.Rotulo)
	} else {
		p.Notas = append(p.Notas, "ramo: não reconheci um ramo específico, usei o modelo de empresa")
	}

	// ---- nome
	if m := reChamado.FindStringSubmatch(desc); m != nil {
		p.Nome = limparNome(m[1])
	}
	if p.Nome == "" {
		if m := reAspas.FindStringSubmatch(desc); m != nil {
			p.Nome = limparNome(m[1])
		}
	}
	if p.Nome == "" {
		if m := reInicio.FindStringSubmatch(desc); m != nil {
			n := strings.TrimSpace(m[1])
			r := []rune(n)
			r[0] = unicode.ToUpper(r[0])
			p.Nome = limparNome(string(r))
		}
	}
	if p.Nome == "" {
		p.Nome = p.Setor.Nome
		p.Notas = append(p.Notas, "nome: não achei, usei \""+p.Nome+"\" (troque no painel ou escreva \"chamada X\")")
	} else {
		p.Notas = append(p.Notas, "nome: "+p.Nome)
	}

	// ---- cidade ("em Salvador", "de Belo Horizonte"), sem pegar o próprio nome
	for _, m := range reCidade.FindAllStringSubmatch(desc, -1) {
		c := strings.TrimSpace(m[1])
		if c == "" || strings.Contains(p.Nome, c) || semAcento(c) == "nepenthes" {
			continue
		}
		p.Cidade = c
		p.Notas = append(p.Notas, "cidade: "+c)
		break
	}

	// ---- cor
	p.Cor = p.Setor.Cor
	if m := reHex.FindString(desc); m != "" {
		p.Cor = strings.ToLower(m)
		p.Notas = append(p.Notas, "cor: "+p.Cor)
	} else {
		achou := false
		for _, c := range cores {
			if len(achar(txt, c.chave)) > 0 {
				p.Cor = c.hex
				p.Notas = append(p.Notas, "cor: "+c.nome)
				achou = true
				break
			}
		}
		if !achou {
			p.Notas = append(p.Notas, "cor: a do ramo ("+p.Cor+")")
		}
	}

	// ---- estilo / fonte
	p.Fonte = p.Setor.Fonte
	for _, e := range estilos {
		if len(achar(txt, e.chave)) > 0 {
			p.Fonte = e.fonte
			p.Notas = append(p.Notas, "estilo: "+e.nome+" (fonte "+e.fonte+")")
			break
		}
	}

	// ---- módulos: com / só / sem
	pedidos, tirados := map[string]bool{}, map[string]bool{}
	for _, m := range ModulosTodos {
		sim, nao := mencao(txt, chavesModulo[m])
		if nao > 0 {
			tirados[m] = true
		} else if sim > 0 {
			pedidos[m] = true
		}
	}
	// palavras do próprio ramo ("loja", "comunidade") não contam como pedido de módulo
	// quando são só o tipo do negócio; mas contam se vierem depois de "com".
	so := reSo.MatchString(txt)
	escolhidos := map[string]bool{}
	soContato := so && len(pedidos) == 0 && len(achar(txt, "contato")) > 0
	if (!so || len(pedidos) == 0) && !soContato {
		for _, m := range p.Setor.Modulos {
			escolhidos[m] = true
		}
	}
	for m := range pedidos {
		escolhidos[m] = true
	}
	for m := range tirados {
		delete(escolhidos, m)
	}
	// planos sem tabela de preços do ramo usa a genérica; idiomas e atendente dependem da IA
	for _, m := range ModulosTodos {
		if escolhidos[m] {
			p.Modulos = append(p.Modulos, m)
		}
		if pedidos[m] && !tirados[m] {
			p.Pedidos = append(p.Pedidos, m)
		}
		if tirados[m] {
			p.Tirados = append(p.Tirados, m)
		}
	}
	if so && (len(pedidos) > 0 || soContato) {
		p.Notas = append(p.Notas, "\"só\": deixei apenas o que você pediu (mais contato e páginas)")
	}
	if p.Tem("idiomas") {
		for nome, cod := range codIdioma {
			if cod != "pt" && len(achar(txt, nome)) > 0 {
				p.Idiomas = append(p.Idiomas, cod)
			}
		}
		sort.Strings(p.Idiomas)
		if len(p.Idiomas) == 0 {
			p.Idiomas = []string{"en", "es"}
		}
	}
	return p
}

// Tem diz se o módulo entrou no site.
func (p *Plano) Tem(m string) bool {
	for _, x := range p.Modulos {
		if x == m {
			return true
		}
	}
	return false
}

func limparNome(s string) string {
	s = strings.TrimSpace(strings.Trim(s, `"“”«»' `))
	s = strings.Map(func(r rune) rune {
		if strings.ContainsRune("{}<>[]|`\\", r) {
			return -1
		}
		return r
	}, s)
	if len([]rune(s)) > 50 {
		s = string([]rune(s)[:50])
	}
	// "sol maior" -> "Sol Maior" (só se veio tudo minúsculo)
	if s == strings.ToLower(s) {
		ws := strings.Fields(s)
		for i, w := range ws {
			if i > 0 && len(w) <= 3 && (w == "de" || w == "da" || w == "do" || w == "dos" || w == "das" || w == "e") {
				continue
			}
			r := []rune(w)
			r[0] = unicode.ToUpper(r[0])
			ws[i] = string(r)
		}
		s = strings.Join(ws, " ")
	}
	return s
}

// completar preenche o que o setor não definiu com o modelo genérico.
func completar(s Setor) Setor {
	g := generico
	if s.Servicos == nil {
		s.Servicos, s.ServRotulo, s.ServIntro = g.Servicos, g.ServRotulo, g.ServIntro
	}
	if s.Equipe == nil {
		s.Equipe, s.EquipeRotulo = g.Equipe, g.EquipeRotulo
	}
	if s.Posts == nil {
		s.Posts, s.Cats, s.BlogRotulo = g.Posts, g.Cats, g.BlogRotulo
	}
	if s.Produtos == nil {
		s.Produtos = g.Produtos
	}
	if s.LojaRotulo == "" {
		s.LojaRotulo = g.LojaRotulo
	}
	if s.Horarios == nil && s.AgendaModo == "" {
		s.Horarios, s.PorHorario = g.Horarios, g.PorHorario
	}
	if s.AgendaVerbo == "" {
		s.AgendaVerbo, s.AgendaTitulo = g.AgendaVerbo, g.AgendaTitulo
	}
	if s.PorHorario == 0 {
		s.PorHorario = 1
	}
	if s.Faq == nil {
		s.Faq = g.Faq
	}
	if s.Planos == nil {
		s.Planos = g.Planos
	}
	if s.Depo == nil {
		s.Depo = g.Depo
	}
	if s.Eventos == nil {
		s.Eventos = g.Eventos
	}
	if s.Galeria == nil {
		s.Galeria = g.Galeria
	}
	if s.Numeros == nil {
		s.Numeros = g.Numeros
	}
	return s
}

// Pasta sugere o nome da pasta do site a partir do nome.
func Pasta(p *Plano) string {
	if s := slugify(p.Nome); s != "" {
		return s
	}
	return "meu-site"
}
