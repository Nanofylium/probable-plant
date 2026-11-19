package instagram

import (
	"image"
	"image/color"
	"math/rand"
	"testing"
)

// printFalso desenha um print de perfil: cabeçalho (foto redonda, textos,
// botão liso), destaques e a grade 3 x n de fotos 3:4 com 2 px entre elas.
func printFalso(fundo, texto color.RGBA, linhas int, parcial bool) (*image.RGBA, []image.Rectangle) {
	const w = 585
	cel := (w - 4) / 3
	alt := cel * 4 / 3
	topo := 520
	h := topo + linhas*(alt+2) + 60
	if parcial {
		h -= alt / 2 // a última linha da grade sai cortada no print
	}
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	pinta := func(r image.Rectangle, f func(x, y int) color.RGBA) {
		for y := r.Min.Y; y < r.Max.Y && y < h; y++ {
			for x := r.Min.X; x < r.Max.X; x++ {
				img.Set(x, y, f(x, y))
			}
		}
	}
	pinta(img.Bounds(), func(int, int) color.RGBA { return fundo })
	rnd := rand.New(rand.NewSource(1))
	ruido := func(base color.RGBA) func(x, y int) color.RGBA {
		return func(x, y int) color.RGBA {
			d := uint8(rnd.Intn(90))
			return color.RGBA{base.R/2 + d, base.G/2 + d/2, base.B/2 + d/3, 255}
		}
	}
	// foto do perfil (redonda, ~22% da largura) e textos
	pinta(image.Rect(20, 30, 150, 160), func(x, y int) color.RGBA {
		if (x-85)*(x-85)+(y-95)*(y-95) < 65*65 {
			return ruido(color.RGBA{200, 120, 60, 255})(x, y)
		}
		return fundo
	})
	for i := 0; i < 6; i++ {
		pinta(image.Rect(20, 190+i*24, 20+rnd.Intn(400)+60, 190+i*24+12), func(int, int) color.RGBA { return texto })
	}
	// botão "Editar perfil" liso, largo
	cinza := color.RGBA{fundo.R/2 + 60, fundo.G/2 + 60, fundo.B/2 + 60, 255}
	pinta(image.Rect(20, 350, w-20, 400), func(int, int) color.RGBA { return cinza })
	// destaques (bolinhas pequenas)
	for i := 0; i < 4; i++ {
		pinta(image.Rect(20+i*90, 420, 90+i*90, 490), ruido(color.RGBA{120, 160, 200, 255}))
	}
	var fotos []image.Rectangle
	for l := 0; l < linhas; l++ {
		for c := 0; c < 3; c++ {
			r := image.Rect(c*(cel+2), topo+l*(alt+2), c*(cel+2)+cel, topo+l*(alt+2)+alt)
			pinta(r, ruido(color.RGBA{uint8(60 + 60*c), uint8(200 - 50*l), 120, 255}))
			if r.Max.Y <= h {
				fotos = append(fotos, r)
			}
		}
	}
	return img, fotos
}

func TestRecortarGrade(t *testing.T) {
	for _, tema := range []struct {
		nome         string
		fundo, texto color.RGBA
	}{{"claro", color.RGBA{255, 255, 255, 255}, color.RGBA{20, 20, 20, 255}}, {"escuro", color.RGBA{0, 0, 0, 255}, color.RGBA{240, 240, 240, 255}}} {
		img, quer := printFalso(tema.fundo, tema.texto, 3, true)
		got := Recortar(img)
		if len(got) != len(quer) {
			t.Fatalf("%s: achei %d fotos, esperava %d: %v", tema.nome, len(got), len(quer), got)
		}
		for i := range quer {
			if got[i] != quer[i] {
				t.Errorf("%s: foto %d em %v, esperava %v", tema.nome, i, got[i], quer[i])
			}
		}
	}
}

func TestRecortarPostSozinho(t *testing.T) {
	// print de um post: barra de cima, a foto ocupando a largura, ícones e legenda
	w, h := 585, 1100
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	rnd := rand.New(rand.NewSource(2))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			c := color.RGBA{255, 255, 255, 255}
			switch {
			case y >= 120 && y < 120+w: // a foto, quadrada
				d := uint8(rnd.Intn(120))
				c = color.RGBA{100 + d, 80 + d/2, 40, 255}
			case y >= 760 && y < 772 && x < 300: // legenda
				c = color.RGBA{30, 30, 30, 255}
			}
			img.Set(x, y, c)
		}
	}
	got := Recortar(img)
	if len(got) != 1 || got[0] != image.Rect(0, 120, w, 120+w) {
		t.Fatalf("post: %v", got)
	}
}

func TestRecortarNadaEmTelaSemFoto(t *testing.T) {
	img, _ := printFalso(color.RGBA{255, 255, 255, 255}, color.RGBA{0, 0, 0, 255}, 0, false)
	if got := Recortar(img); len(got) != 0 {
		t.Fatalf("achou foto em tela sem grade: %v", got)
	}
}

func TestRecortarFotoComCeuBranco(t *testing.T) {
	// a foto do meio tem uma faixa branca no alto: tem de sair inteira, alinhada às vizinhas
	img, quer := printFalso(color.RGBA{255, 255, 255, 255}, color.RGBA{20, 20, 20, 255}, 1, false)
	ceu := quer[1]
	for y := ceu.Min.Y; y < ceu.Min.Y+50; y++ {
		for x := ceu.Min.X; x < ceu.Max.X; x++ {
			img.Set(x, y, color.RGBA{250, 250, 252, 255})
		}
	}
	got := Recortar(img)
	if len(got) != 3 || got[1] != quer[1] {
		t.Fatalf("achei %v, esperava %v", got, quer)
	}
}
