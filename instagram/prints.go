package instagram

// Fotos a partir de prints (capturas de tela) do Instagram: a pessoa tira
// print do próprio perfil (a grade de fotos) ou de um post, e o Nepenthes acha
// os retângulos de foto e recorta cada um. Nada de login nem de API: só as
// imagens que a pessoa mesma mandou.
//
// Como acha: a cor do fundo é a das bordas do print (branco no tema claro,
// preto no escuro). O print é cortado em pedaços pelas linhas e colunas
// inteiras dessa cor, e cada pedaço de novo, até não dar mais para cortar
// (assim a grade sai foto por foto, mesmo com menu ao lado no computador).
// Um pedaço vira foto se for grande, no formato das fotos do Instagram
// (quadrada, retrato 4:5 ou 3:4, ou paisagem ocupando a largura), cheio
// (quase sem fundo dentro: a foto redonda do perfil e os destaques ficam de
// fora) e com detalhe (não um botão liso). Pedaço cortado pela borda de cima
// ou de baixo do print fica de fora: a foto não veio inteira.

import (
	"image"
	"image/color"
	"sort"
)

// Recortar devolve os retângulos de foto achados no print, de cima para baixo
// e da esquerda para a direita.
func Recortar(img image.Image) []image.Rectangle {
	b := img.Bounds()
	w, h := b.Dx(), b.Dy()
	if w < 200 || h < 200 {
		return nil
	}
	lum := make([]uint8, w*h)
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			r, g, bl, _ := img.At(b.Min.X+x, b.Min.Y+y).RGBA()
			lum[y*w+x] = uint8((299*r + 587*g + 114*bl) / 1000 >> 8)
		}
	}
	c := &cortador{lum: lum, w: w, h: h, fundo: corDoFundo(lum, w, h)}
	c.cortar(image.Rect(0, 0, w, h), 0)
	achadas := alinhar(c.achadas)
	sort.SliceStable(achadas, func(i, j int) bool {
		if achadas[i].Min.Y != achadas[j].Min.Y {
			return achadas[i].Min.Y < achadas[j].Min.Y
		}
		return achadas[i].Min.X < achadas[j].Min.X
	})
	for i := range achadas {
		achadas[i] = achadas[i].Add(b.Min)
	}
	return achadas
}

// alinhar estica na altura uma foto que saiu mais baixa que as vizinhas da
// mesma linha da grade: acontece quando a foto tem uma faixa da cor do fundo
// (um céu branco no tema claro) colada na borda.
func alinhar(rs []image.Rectangle) []image.Rectangle {
	out := append([]image.Rectangle(nil), rs...)
	for i, r := range rs {
		alvo := r
		for j, o := range rs {
			if i == j || absInt(o.Dx()-r.Dx())*25 > r.Dx() {
				continue // largura diferente: não é da mesma grade
			}
			comum := min(r.Max.Y, o.Max.Y) - max(r.Min.Y, o.Min.Y)
			if comum*2 < r.Dy() || o.Dy() <= r.Dy() {
				continue
			}
			alvo.Min.Y, alvo.Max.Y = min(alvo.Min.Y, o.Min.Y), max(alvo.Max.Y, o.Max.Y)
		}
		livre := true
		for j, o := range rs {
			if i != j && alvo.Overlaps(o) {
				livre = false
				break
			}
		}
		if livre {
			out[i] = alvo
		}
	}
	return out
}

func absInt(a int) int {
	if a < 0 {
		return -a
	}
	return a
}

type cortador struct {
	lum     []uint8
	w, h    int
	fundo   uint8
	achadas []image.Rectangle
}

func (c *cortador) perto(x, y int) bool { return absDif(c.lum[y*c.w+x], c.fundo) <= 14 }

// cortar divide r pelas linhas de fundo; se não der, pelas colunas; se nem
// assim, r é um pedaço final e é testado como foto.
func (c *cortador) cortar(r image.Rectangle, nivel int) {
	if nivel > 8 {
		return
	}
	linhas := make([]bool, r.Dy())
	for y := r.Min.Y; y < r.Max.Y; y++ {
		fora := 0
		for x := r.Min.X; x < r.Max.X; x++ {
			if !c.perto(x, y) {
				fora++
			}
		}
		linhas[y-r.Min.Y] = fora <= r.Dx()/100 // 1% de tolerância, por ruído
	}
	if ts := trechos(linhas, c.h/40); len(ts) != 1 || ts[0][1]-ts[0][0] != r.Dy() {
		for _, t := range ts {
			c.cortar(image.Rect(r.Min.X, r.Min.Y+t[0], r.Max.X, r.Min.Y+t[1]), nivel+1)
		}
		return
	}
	colunas := make([]bool, r.Dx())
	for x := r.Min.X; x < r.Max.X; x++ {
		fora := 0
		for y := r.Min.Y; y < r.Max.Y; y++ {
			if !c.perto(x, y) {
				fora++
			}
		}
		colunas[x-r.Min.X] = fora <= r.Dy()/100
	}
	if ts := trechos(colunas, c.w/40); len(ts) != 1 || ts[0][1]-ts[0][0] != r.Dx() {
		for _, t := range ts {
			c.cortar(image.Rect(r.Min.X+t[0], r.Min.Y, r.Min.X+t[1], r.Max.Y), nivel+1)
		}
		return
	}
	if c.ehFoto(r) {
		c.achadas = append(c.achadas, c.aparar(r))
	}
}

// distancia: quanto a linha (ou coluna) de pontos difere do fundo, em média.
func (c *cortador) distancia(x0, y0, dx, dy, n int) int {
	soma := 0
	for i := 0; i < n; i++ {
		soma += int(absDif(c.lum[(y0+i*dy)*c.w+x0+i*dx], c.fundo))
	}
	return soma / max(n, 1)
}

// aparar tira das bordas da foto as linhas de 1 ou 2 pontos que ainda são o
// vão entre as fotos, borrado pela compressão do print (JPG): ficam bem mais
// perto do fundo que a linha logo para dentro.
func (c *cortador) aparar(r image.Rectangle) image.Rectangle {
	for i := 0; i < 3 && r.Dx() > 20; i++ {
		esq, dentro := c.distancia(r.Min.X, r.Min.Y, 0, 1, r.Dy()), c.distancia(r.Min.X+1, r.Min.Y, 0, 1, r.Dy())
		if esq*2 >= dentro || dentro < 20 {
			break
		}
		r.Min.X++
	}
	for i := 0; i < 3 && r.Dx() > 20; i++ {
		dir, dentro := c.distancia(r.Max.X-1, r.Min.Y, 0, 1, r.Dy()), c.distancia(r.Max.X-2, r.Min.Y, 0, 1, r.Dy())
		if dir*2 >= dentro || dentro < 20 {
			break
		}
		r.Max.X--
	}
	for i := 0; i < 3 && r.Dy() > 20; i++ {
		cima, dentro := c.distancia(r.Min.X, r.Min.Y, 1, 0, r.Dx()), c.distancia(r.Min.X, r.Min.Y+1, 1, 0, r.Dx())
		if cima*2 >= dentro || dentro < 20 {
			break
		}
		r.Min.Y++
	}
	for i := 0; i < 3 && r.Dy() > 20; i++ {
		baixo, dentro := c.distancia(r.Min.X, r.Max.Y-1, 1, 0, r.Dx()), c.distancia(r.Min.X, r.Max.Y-2, 1, 0, r.Dx())
		if baixo*2 >= dentro || dentro < 20 {
			break
		}
		r.Max.Y--
	}
	return r
}

func (c *cortador) ehFoto(r image.Rectangle) bool {
	cw, ch := r.Dx(), r.Dy()
	if r.Min.Y == 0 || r.Max.Y == c.h || cw < 100 || cw < c.w/8 {
		return false
	}
	retrato := ch*100 >= cw*85 && ch*100 <= cw*150 // 1:1, 4:5, 3:4
	paisagem := ch*100 >= cw*50 && cw >= c.w/2     // post deitado, na largura da tela
	if !retrato && !paisagem {
		return false
	}
	// cheia: quase nada de fundo dentro (círculos e textos têm muito)
	fundo, n := 0, 0
	for y := r.Min.Y; y < r.Max.Y; y += 3 {
		for x := r.Min.X; x < r.Max.X; x += 3 {
			if c.perto(x, y) {
				fundo++
			}
			n++
		}
	}
	if fundo*100 > n*12 {
		return false
	}
	return detalhe(c.lum, c.w, r) >= 8 // bloco liso: botão, faixa, área vazia
}

// trechos devolve os intervalos [início, fim) onde sep é falso, com pelo
// menos min de tamanho.
func trechos(sep []bool, min int) [][2]int {
	var out [][2]int
	ini := -1
	for i := 0; i <= len(sep); i++ {
		dentro := i < len(sep) && !sep[i]
		if dentro && ini < 0 {
			ini = i
		}
		if !dentro && ini >= 0 {
			if i-ini >= min {
				out = append(out, [2]int{ini, i})
			}
			ini = -1
		}
	}
	return out
}

// corDoFundo: a luminância mais comum nas bordas do print.
func corDoFundo(lum []uint8, w, h int) uint8 {
	var hist [256]int
	for x := 0; x < w; x++ {
		hist[lum[x]]++
		hist[lum[(h-1)*w+x]]++
	}
	for y := 0; y < h; y++ {
		hist[lum[y*w]]++
		hist[lum[y*w+w-1]]++
	}
	best := 0
	for v := range hist {
		if hist[v] > hist[best] {
			best = v
		}
	}
	return uint8(best)
}

// detalhe: desvio médio da luminância na região (foto tem; botão liso não).
func detalhe(lum []uint8, w int, r image.Rectangle) int {
	var soma, n int
	passo := 3
	for y := r.Min.Y; y < r.Max.Y; y += passo {
		for x := r.Min.X; x < r.Max.X; x += passo {
			soma += int(lum[y*w+x])
			n++
		}
	}
	if n == 0 {
		return 0
	}
	media := soma / n
	var dev int
	for y := r.Min.Y; y < r.Max.Y; y += passo {
		for x := r.Min.X; x < r.Max.X; x += passo {
			d := int(lum[y*w+x]) - media
			if d < 0 {
				d = -d
			}
			dev += d
		}
	}
	return dev / n
}

func absDif(a, b uint8) uint8 {
	if a > b {
		return a - b
	}
	return b - a
}

// Sub recorta a imagem (cópia, para poder gravar).
func Sub(img image.Image, r image.Rectangle) image.Image {
	out := image.NewRGBA(image.Rect(0, 0, r.Dx(), r.Dy()))
	for y := 0; y < r.Dy(); y++ {
		for x := 0; x < r.Dx(); x++ {
			out.Set(x, y, color.RGBAModel.Convert(img.At(r.Min.X+x, r.Min.Y+y)))
		}
	}
	return out
}

// Marca é uma miniatura 8 x 8 do brilho da foto: a mesma foto em dois
// prints, em tamanhos diferentes, dá marcas quase iguais. Serve para não
// repetir foto quando os prints se cobrem.
type Marca [64]uint8

// Marcar tira a marca de uma foto.
func Marcar(img image.Image) Marca {
	b := img.Bounds()
	var m Marca
	for j := 0; j < 8; j++ {
		for i := 0; i < 8; i++ {
			x0, x1 := b.Min.X+i*b.Dx()/8, b.Min.X+(i+1)*b.Dx()/8
			y0, y1 := b.Min.Y+j*b.Dy()/8, b.Min.Y+(j+1)*b.Dy()/8
			var soma, n int
			for y := y0; y < y1; y += 2 {
				for x := x0; x < x1; x += 2 {
					r, g, bl, _ := img.At(x, y).RGBA()
					soma += int(299*r+587*g+114*bl) / 1000 >> 8
					n++
				}
			}
			if n > 0 {
				m[j*8+i] = uint8(soma / n)
			}
		}
	}
	return m
}

// Parecida diz se as duas marcas são da mesma foto (diferença média pequena).
func (m Marca) Parecida(o Marca) bool {
	d := 0
	for i := range m {
		d += int(absDif(m[i], o[i]))
	}
	return d <= 64*6
}
