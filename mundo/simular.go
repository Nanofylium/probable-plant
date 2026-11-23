package mundo

// Passeio simulado, sem GPU: a câmera se aproxima e vira sobre a própria foto.
// Não inventa o que está fora da foto (isso só o LingBot-World faz), mas deixa
// o site pronto e o passeio funcionando até haver um servidor com GPU.

import (
	"bytes"
	"errors"
	"image"
	"image/draw"
	"image/jpeg"
	_ "image/png"
	"math"
)

// Simular gera os quadros de um caminho a partir da foto (JPEG ou PNG).
func Simular(foto []byte, caminho string, n, largura, altura int) ([][]byte, error) {
	img, _, err := image.Decode(bytes.NewReader(foto))
	if err != nil {
		return nil, errors.New("não consegui ler a foto (use JPEG ou PNG)")
	}
	if n <= 0 {
		n = 48
	}
	if largura <= 0 || altura <= 0 {
		largura, altura = 832, 480
	}
	src := image.NewRGBA(img.Bounds())
	draw.Draw(src, src.Bounds(), img, img.Bounds().Min, draw.Src)
	sw, sh := float64(src.Bounds().Dx()), float64(src.Bounds().Dy())
	if sw < 16 || sh < 16 {
		return nil, errors.New("a foto é pequena demais")
	}
	// recorte que cobre o quadro inteiro (como object-fit: cover)
	asp := float64(largura) / float64(altura)
	cw, ch := sw, sw/asp
	if ch > sh {
		ch, cw = sh, sh*asp
	}
	out := make([][]byte, n)
	dst := image.NewRGBA(image.Rect(0, 0, largura, altura))
	for f := 0; f < n; f++ {
		t := float64(f) / float64(max(n-1, 1))
		z, cx, cy := camera(caminho, t)
		w, h := cw/z, ch/z
		// centro em coordenadas da foto, sem sair dela
		x0 := sw/2 + cx*(sw-w)/2 - w/2
		y0 := sh/2 + cy*(sh-h)/2 - h/2
		x0 = math.Max(0, math.Min(sw-w, x0))
		y0 = math.Max(0, math.Min(sh-h, y0))
		amostrar(dst, src, x0, y0, w/float64(largura), h/float64(altura))
		var b bytes.Buffer
		if err := jpeg.Encode(&b, dst, &jpeg.Options{Quality: 72}); err != nil {
			return nil, err
		}
		out[f] = b.Bytes()
	}
	return out, nil
}

func suave(t float64) float64 { return t * t * (3 - 2*t) }

// camera devolve o zoom e o centro (-1 a 1 em cada eixo) no instante t.
func camera(caminho string, t float64) (z, cx, cy float64) {
	passo := math.Sin(t*math.Pi*10) * 0.012 // balanço leve de quem anda
	switch caminho {
	case "esquerda", "direita":
		lado := -1.0
		if caminho == "direita" {
			lado = 1
		}
		if t < .3 {
			v := suave(t / .3)
			return 1.25, lado * .8 * v, .1 * v
		}
		v := suave((t - .3) / .7)
		return 1.25 + .55*v, lado * (.8 - .3*v), .1 + .15*v + passo
	case "redor":
		return 1.35, -1 + 2*suave(t), 0
	default:
		return 1 + .7*suave(t), 0, .25*suave(t) + passo
	}
}

// amostrar copia para dst o retângulo (x0, y0) da foto, com escala sx/sy por
// pixel e interpolação bilinear.
func amostrar(dst, src *image.RGBA, x0, y0, sx, sy float64) {
	b := src.Bounds()
	mx, my := b.Dx()-1, b.Dy()-1
	for y := 0; y < dst.Rect.Dy(); y++ {
		fy := y0 + (float64(y)+.5)*sy - .5
		iy := int(math.Floor(fy))
		ay := fy - float64(iy)
		y1, y2 := clamp(iy, my), clamp(iy+1, my)
		for x := 0; x < dst.Rect.Dx(); x++ {
			fx := x0 + (float64(x)+.5)*sx - .5
			ix := int(math.Floor(fx))
			ax := fx - float64(ix)
			x1, x2 := clamp(ix, mx), clamp(ix+1, mx)
			p11 := src.PixOffset(b.Min.X+x1, b.Min.Y+y1)
			p21 := src.PixOffset(b.Min.X+x2, b.Min.Y+y1)
			p12 := src.PixOffset(b.Min.X+x1, b.Min.Y+y2)
			p22 := src.PixOffset(b.Min.X+x2, b.Min.Y+y2)
			d := dst.PixOffset(x, y)
			for c := 0; c < 3; c++ {
				v := (float64(src.Pix[p11+c])*(1-ax)+float64(src.Pix[p21+c])*ax)*(1-ay) +
					(float64(src.Pix[p12+c])*(1-ax)+float64(src.Pix[p22+c])*ax)*ay
				dst.Pix[d+c] = uint8(v + .5)
			}
			dst.Pix[d+3] = 255
		}
	}
}

func clamp(v, m int) int {
	if v < 0 {
		return 0
	}
	if v > m {
		return m
	}
	return v
}
