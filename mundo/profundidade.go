package mundo

// Passeio 3D sem GPU: o Depth Anything V2 Small (Apache 2.0) descobre a
// distância de cada ponto da foto, rodando no processador pelo pacote rede.
// O navegador de quem visita monta a cena 3D com a foto e essa profundidade.

import (
	"bytes"
	"errors"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/jpeg"
	"image/png"
	"math"
	"os"
	"path/filepath"

	"github.com/Nanofylium/probable-plant/ialocal"
	"github.com/Nanofylium/probable-plant/rede"
)

// Modelo de profundidade (pesos fixados pelo SHA-256).
const (
	ModeloProfNome = "Depth Anything V2 Small"
	modeloProfSHA  = "afb6a5c28f3b6bf1618c6e43f02073ef9dfdc70e937502d51603e57b0a1df10c"
	ModeloProfMB   = 95
)

var modeloProfURLs = []string{
	"https://huggingface.co/onnx-community/depth-anything-v2-small/resolve/main/onnx/model.onnx",
}

// CaminhoModeloProf é onde o modelo fica guardado (NEP_PROFUNDIDADE troca).
func CaminhoModeloProf() string {
	if c := os.Getenv("NEP_PROFUNDIDADE"); c != "" {
		return c
	}
	return filepath.Join(filepath.Dir(ialocal.Pasta()), "profundidade", "depth-anything-v2-small.onnx")
}

// ModeloProfBaixado diz se o modelo já está no computador.
func ModeloProfBaixado() bool {
	st, err := os.Stat(CaminhoModeloProf())
	return err == nil && st.Size() > 1<<20
}

// BaixarModeloProf baixa o modelo uma vez só (95 MB).
func BaixarModeloProf(progresso func(feito, total int64)) error {
	if ModeloProfBaixado() {
		return nil
	}
	var feito int64
	total := int64(99060839)
	return ialocal.Buscar(modeloProfURLs, modeloProfSHA, CaminhoModeloProf(), func(n int64) {
		feito += n
		if progresso != nil {
			progresso(min(feito, total), total)
		}
	})
}

// Mapa de profundidade: Disp[i] vai de 0 (longe) a 1 (perto).
type Mapa struct {
	Largura, Altura int
	Disp            []float32
}

func decodificar(foto []byte) (*image.RGBA, error) {
	img, _, err := image.Decode(bytes.NewReader(foto))
	if err != nil {
		return nil, errors.New("não consegui ler a foto (use JPEG ou PNG)")
	}
	r := image.NewRGBA(img.Bounds())
	draw.Draw(r, r.Bounds(), img, img.Bounds().Min, draw.Src)
	if r.Bounds().Dx() < 32 || r.Bounds().Dy() < 32 {
		return nil, errors.New("a foto é pequena demais")
	}
	return r, nil
}

// redimensiona com interpolação bilinear
func redim(src *image.RGBA, w, h int) *image.RGBA {
	dst := image.NewRGBA(image.Rect(0, 0, w, h))
	sw, sh := float64(src.Bounds().Dx()), float64(src.Bounds().Dy())
	// reduções grandes: primeiro pela metade, para não serrilhar
	for sw > float64(w)*2.5 && sh > float64(h)*2.5 {
		meia := image.NewRGBA(image.Rect(0, 0, int(sw)/2, int(sh)/2))
		amostrar(meia, src, 0, 0, 2, 2)
		src, sw, sh = meia, float64(int(sw)/2), float64(int(sh)/2)
	}
	amostrar(dst, src, 0, 0, sw/float64(w), sh/float64(h))
	return dst
}

// CalcularProfundidade roda o modelo na foto. lado é o maior lado da entrada
// do modelo (múltiplo de 14; 518 é o tamanho do treino).
func CalcularProfundidade(m *rede.Modelo, foto []byte, lado int) (*Mapa, error) {
	img, err := decodificar(foto)
	if err != nil {
		return nil, err
	}
	if lado <= 0 {
		lado = 518
	}
	W, H := float64(img.Bounds().Dx()), float64(img.Bounds().Dy())
	s := float64(lado) / math.Max(W, H)
	w := max(14, int(math.Round(W*s/14))*14)
	h := max(14, int(math.Round(H*s/14))*14)
	pequena := redim(img, w, h)
	media := [3]float32{.485, .456, .406}
	desvio := [3]float32{.229, .224, .225}
	x := rede.NovoTensor(1, 3, h, w)
	for y := 0; y < h; y++ {
		for xx := 0; xx < w; xx++ {
			o := pequena.PixOffset(xx, y)
			for c := 0; c < 3; c++ {
				x.F[(c*h+y)*w+xx] = (float32(pequena.Pix[o+c])/255 - media[c]) / desvio[c]
			}
		}
	}
	out, err := m.Rodar(x)
	if err != nil {
		return nil, err
	}
	if out.Len() != w*h {
		return nil, fmt.Errorf("profundidade com forma inesperada %v", out.Forma)
	}
	return normalizar(&Mapa{Largura: w, Altura: h, Disp: append([]float32(nil), out.F...)}), nil
}

// normalizar leva a disparidade para 0..1, ignorando os 1% mais extremos.
func normalizar(m *Mapa) *Mapa {
	v := append([]float32(nil), m.Disp...)
	lo, hi := percentil(v, .01), percentil(v, .99)
	if hi-lo < 1e-6 {
		hi = lo + 1
	}
	for i, d := range m.Disp {
		m.Disp[i] = float32(math.Min(1, math.Max(0, float64((d-lo)/(hi-lo)))))
	}
	return m
}

func percentil(v []float32, p float64) float32 {
	// histograma simples (rápido e sem ordenar tudo)
	lo, hi := v[0], v[0]
	for _, x := range v {
		lo, hi = min(lo, x), max(hi, x)
	}
	if hi == lo {
		return lo
	}
	const B = 2048
	var h [B]int
	for _, x := range v {
		h[min(B-1, int(float32(B)*(x-lo)/(hi-lo)))]++
	}
	alvo, soma := int(p*float64(len(v))), 0
	for i, c := range h {
		soma += c
		if soma > alvo {
			return lo + (hi-lo)*float32(i)/B
		}
	}
	return hi
}

// EstimarProfundidade é o plano B sem modelo: embaixo fica perto, em cima
// longe, com o centro um pouco mais fundo (como numa rua ou num salão).
func EstimarProfundidade(foto []byte) (*Mapa, error) {
	img, err := decodificar(foto)
	if err != nil {
		return nil, err
	}
	W, H := float64(img.Bounds().Dx()), float64(img.Bounds().Dy())
	w := 256
	h := max(16, int(math.Round(float64(w)*H/W)))
	m := &Mapa{Largura: w, Altura: h, Disp: make([]float32, w*h)}
	for y := 0; y < h; y++ {
		vy := float64(y) / float64(h-1)
		for x := 0; x < w; x++ {
			vx := float64(x)/float64(w-1) - .5
			d := .15 + .85*math.Pow(vy, 1.6)  // chão chegando perto
			d += .18 * vx * vx * 4 * (1 - vy) // laterais um pouco mais perto
			m.Disp[y*w+x] = float32(math.Min(1, d))
		}
	}
	return m, nil
}

// PNG da profundidade: 16 bits guardados no vermelho (alto) e verde (baixo),
// porque o navegador lê PNG cinza de 16 bits com só 8.
func (m *Mapa) PNG() ([]byte, error) {
	img := image.NewNRGBA(image.Rect(0, 0, m.Largura, m.Altura))
	for i, d := range m.Disp {
		v := uint16(math.Round(float64(d) * 65535))
		img.SetNRGBA(i%m.Largura, i/m.Largura, color.NRGBA{uint8(v >> 8), uint8(v), 0, 255})
	}
	var b bytes.Buffer
	err := png.Encode(&b, img)
	return b.Bytes(), err
}

// FotoParaSite reduz a foto (lado maior até 1600) e grava em JPEG.
func FotoParaSite(foto []byte) ([]byte, int, int, error) {
	img, err := decodificar(foto)
	if err != nil {
		return nil, 0, 0, err
	}
	W, H := img.Bounds().Dx(), img.Bounds().Dy()
	if m := max(W, H); m > 1600 {
		W, H = W*1600/m, H*1600/m
		img = redim(img, W, H)
	}
	var b bytes.Buffer
	err = jpeg.Encode(&b, img, &jpeg.Options{Quality: 86})
	return b.Bytes(), W, H, err
}

// Salvar3D grava o passeio 3D em pasta/nome: foto.jpg, profundidade.png e mundo.json.
func Salvar3D(pasta string, m *Mundo, foto []byte, mapa *Mapa) error {
	if !NomeValido(m.Nome) {
		return fmt.Errorf("nome %q: use só letras minúsculas, números e hífen", m.Nome)
	}
	dir := filepath.Join(pasta, m.Nome)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	f, w, h, err := FotoParaSite(foto)
	if err != nil {
		return err
	}
	p, err := mapa.PNG()
	if err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(dir, "foto.jpg"), f, 0o644); err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(dir, "profundidade.png"), p, 0o644); err != nil {
		return err
	}
	m.Tipo, m.Foto, m.Profundidade, m.Largura, m.Altura = "3d", "foto.jpg", "profundidade.png", w, h
	m.Caminhos = nil
	return salvarJSON(dir, m)
}
