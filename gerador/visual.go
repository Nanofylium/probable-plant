package gerador

import (
	"hash/fnv"
	"math/rand"
)

// Visuais: as famílias de aparência que o nep serve (web/assets/visuais). Cada
// uma muda a estrutura do site (cabeçalho, capa, cartões, botões, rodapé) e
// traz suas fontes.
var Visuais = []struct{ ID, Nome, Descricao string }{
	{"nepenthes", "Nepenthes", "o clássico: capa dividida, cartões arredondados"},
	{"editorial", "Editorial", "jeito de revista: marca grande no centro, serifa itálica, fios"},
	{"noturno", "Noturno", "escuro, com brilho da cor do site e cartões de vidro"},
	{"vitrine", "Vitrine", "capa de ponta a ponta com a foto ao fundo, títulos altos"},
	{"brutal", "Brutal", "bordas pretas grossas, sombras duras, amarelo de destaque"},
	{"suave", "Suave", "pastel, tudo arredondado, cabeçalho em pílula"},
	{"minimo", "Mínimo", "suíço: preto e branco, letras enormes, seções numeradas"},
	{"luxo", "Luxo", "capa escura com moldura dourada, Cormorant, muito respiro"},
}

// palavras que pedem um visual (sem acento, minúsculas; ver achar)
var chavesVisual = []struct {
	id     string
	chaves []string
}{
	{"noturno", []string{"escuro", "dark", "noturn", "neon", "gamer", "futurist"}},
	{"luxo", []string{"luxo", "luxuos", "sofisticad", "premium", "requintad", "chique", "exclusiv", "elegante", "dourad"}},
	{"brutal", []string{"brutal", "ousad", "irreverent", "vibrante", "descolad", "urbano", "street", "criativ", "jovem"}},
	{"editorial", []string{"editorial", "revista", "jornal", "literari", "vintage", "retro", "classic", "tradicional"}},
	{"minimo", []string{"minimalist", "minimo", "clean", "limpo", "sobrio", "discreto", "simples"}},
	{"suave", []string{"divertid", "infantil", "fofo", "alegre", "colorid", "delicad", "acolhedor", "leve$"}},
	{"vitrine", []string{"impactant", "impacto", "marcante", "chamativ", "energia", "esportiv", "forte$"}},
	{"nepenthes", []string{"moderno", "profissional", "corporativ"}},
}

// os visuais que combinam com cada ramo (o sorteio escolhe entre eles)
var visuaisDoRamo = map[string][]string{
	"restaurante": {"editorial", "suave", "vitrine", "luxo", "nepenthes"},
	"hotel":       {"luxo", "editorial", "vitrine", "nepenthes"},
	"beleza":      {"luxo", "suave", "editorial", "minimo"},
	"saude":       {"nepenthes", "minimo", "suave"},
	"veterinaria": {"suave", "nepenthes", "brutal"},
	"escola":      {"suave", "nepenthes", "brutal", "minimo"},
	"academia":    {"vitrine", "noturno", "brutal"},
	"advocacia":   {"luxo", "editorial", "minimo", "nepenthes"},
	"tecnologia":  {"noturno", "minimo", "brutal", "nepenthes"},
	"loja":        {"minimo", "vitrine", "brutal", "suave", "luxo"},
	"associacao":  {"nepenthes", "editorial", "suave"},
	"comunidade":  {"minimo", "nepenthes", "noturno"},
}

// VisualValido diz se o nome é de um visual conhecido.
func VisualValido(id string) bool {
	for _, v := range Visuais {
		if v.ID == id {
			return true
		}
	}
	return false
}

// escolherVisual: o que a pessoa pediu, ou um dos que combinam com o ramo.
// Sem semente, a mesma descrição dá sempre o mesmo visual; o assistente passa
// uma semente nova a cada pedido, para os sites não saírem iguais.
func escolherVisual(p *Plano, txt string, semente int64) {
	for _, c := range chavesVisual {
		for _, k := range c.chaves {
			if len(achar(txt, k)) > 0 {
				p.Visual, p.VisualPedido = c.id, true
				p.Notas = append(p.Notas, "visual: "+nomeVisual(c.id)+" (pedido)")
				return
			}
		}
	}
	p.Visual = SortearVisual(p.Setor.ID, txt, semente)
	p.Notas = append(p.Notas, "visual: "+nomeVisual(p.Visual)+" (um dos que combinam com o ramo)")
}

// SortearVisual escolhe um visual que combina com o ramo.
func SortearVisual(ramo, txt string, semente int64) string {
	lista := visuaisDoRamo[ramo]
	if len(lista) == 0 {
		lista = []string{}
		for _, v := range Visuais {
			lista = append(lista, v.ID)
		}
	}
	if semente == 0 {
		h := fnv.New64a()
		h.Write([]byte(txt))
		semente = int64(h.Sum64() >> 1)
	}
	return lista[rand.New(rand.NewSource(semente)).Intn(len(lista))]
}

func nomeVisual(id string) string {
	for _, v := range Visuais {
		if v.ID == id {
			return v.Nome
		}
	}
	return id
}

// TrocarVisual muda o visual do plano (quando a pessoa ou a IA escolhe outro).
func TrocarVisual(p *Plano, id string) bool {
	if !VisualValido(id) {
		return false
	}
	p.Visual = id
	for i, n := range p.Notas {
		if len(n) > 8 && n[:8] == "visual: " {
			p.Notas[i] = "visual: " + nomeVisual(id)
		}
	}
	return true
}
