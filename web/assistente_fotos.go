package web

import (
	"bytes"
	"encoding/json"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/jpeg"
	"image/png"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	_ "image/gif"

	"github.com/Nanofylium/probable-plant/gerador"
)

// Logo e fotos que a pessoa envia na tela de criar o site. As imagens são
// lidas, reduzidas e gravadas de novo (sem os dados do celular, como o GPS)
// nos arquivos públicos do site: data/arquivos/p-....png (logo) e p-....jpg (fotos).

const (
	fotosMeta     = "assistente:fotos"
	fotosMaxFotos = 12
	fotosMaxBytes = 20 << 20
	fotoLado      = 1600 // maior lado de uma foto
	logoLado      = 480
)

type fotosDados struct {
	Logo  *gerador.Foto  `json:"logo,omitempty"`
	Fotos []gerador.Foto `json:"fotos"`
}

func (s *Server) fotosGuardadas() *fotosDados {
	var d fotosDados
	if json.Unmarshal([]byte(s.Store.Meta(fotosMeta)), &d) != nil {
		return &fotosDados{}
	}
	return &d
}

func (s *Server) fotosResposta(d *fotosDados) map[string]any {
	out := map[string]any{"fotos": []map[string]string{}}
	var fs []map[string]string
	for _, f := range d.Fotos {
		fs = append(fs, map[string]string{"id": filepath.Base(f.URL), "url": s.url(f.URL)})
	}
	if fs != nil {
		out["fotos"] = fs
	}
	if d.Logo != nil {
		out["logo"] = map[string]string{"id": filepath.Base(d.Logo.URL), "url": s.url(d.Logo.URL)}
	}
	return out
}

// reduzir encolhe a imagem para caber em lado×lado (média da área: sem
// serrilhado). Imagens menores voltam como estão.
func reduzir(src image.Image, lado int) image.Image {
	b := src.Bounds()
	w, h := b.Dx(), b.Dy()
	if w <= lado && h <= lado {
		return src
	}
	nw, nh := lado, h*lado/w
	if h > w {
		nw, nh = w*lado/h, lado
	}
	if nw < 1 {
		nw = 1
	}
	if nh < 1 {
		nh = 1
	}
	rgba := image.NewNRGBA(b)
	draw.Draw(rgba, b, src, b.Min, draw.Src)
	out := image.NewNRGBA(image.Rect(0, 0, nw, nh))
	for y := 0; y < nh; y++ {
		y0, y1 := y*h/nh, (y+1)*h/nh
		if y1 <= y0 {
			y1 = y0 + 1
		}
		for x := 0; x < nw; x++ {
			x0, x1 := x*w/nw, (x+1)*w/nw
			if x1 <= x0 {
				x1 = x0 + 1
			}
			var r, g, bl, a, n uint32
			for yy := y0; yy < y1; yy++ {
				i := rgba.PixOffset(b.Min.X+x0, b.Min.Y+yy)
				for xx := x0; xx < x1; xx++ {
					p := rgba.Pix[i : i+4]
					al := uint32(p[3])
					r += uint32(p[0]) * al
					g += uint32(p[1]) * al
					bl += uint32(p[2]) * al
					a += al
					n++
					i += 4
				}
			}
			o := out.PixOffset(x, y)
			if a > 0 {
				out.Pix[o], out.Pix[o+1], out.Pix[o+2] = uint8(r/a), uint8(g/a), uint8(bl/a)
			}
			out.Pix[o+3] = uint8(a / n)
		}
	}
	return out
}

// aparar tira as bordas transparentes do logo (senão ele fica pequeno e
// longe do nome no topo do site).
func aparar(src image.Image) image.Image {
	b := src.Bounds()
	x0, y0, x1, y1 := b.Max.X, b.Max.Y, b.Min.X, b.Min.Y
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			if _, _, _, a := src.At(x, y).RGBA(); a > 0x0800 {
				x0, y0, x1, y1 = min(x0, x), min(y0, y), max(x1, x+1), max(y1, y+1)
			}
		}
	}
	if x1 <= x0 || y1 <= y0 || (x0 == b.Min.X && y0 == b.Min.Y && x1 == b.Max.X && y1 == b.Max.Y) {
		return src
	}
	out := image.NewNRGBA(image.Rect(0, 0, x1-x0, y1-y0))
	draw.Draw(out, out.Bounds(), src, image.Pt(x0, y0), draw.Src)
	return out
}

// lerImagem lê PNG, JPG ou GIF com limites de tamanho.
func lerImagem(part io.Reader) (image.Image, error) {
	b, err := io.ReadAll(io.LimitReader(part, fotosMaxBytes+1))
	if err != nil {
		return nil, fmt.Errorf("o envio foi interrompido: %v", err)
	}
	if len(b) > fotosMaxBytes {
		return nil, fmt.Errorf("imagem grande demais (até %d MB)", fotosMaxBytes>>20)
	}
	cfg, _, err := image.DecodeConfig(bytes.NewReader(b))
	if err != nil {
		return nil, fmt.Errorf("a imagem precisa ser PNG, JPG ou GIF")
	}
	if cfg.Width*cfg.Height > instaMaxPixel {
		return nil, fmt.Errorf("imagem grande demais (%dx%d)", cfg.Width, cfg.Height)
	}
	img, _, err := image.Decode(bytes.NewReader(b))
	if err != nil {
		return nil, fmt.Errorf("não consegui ler a imagem: %v", err)
	}
	return img, nil
}

// gravarImagem grava a foto (JPG, fundo branco onde era transparente) ou o
// logo (PNG, mantém a transparência).
func (s *Server) gravarImagem(img image.Image, logo bool, nomeOriginal string) (gerador.Foto, error) {
	dir := filepath.Join(s.opt.DataDir, "arquivos")
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return gerador.Foto{}, err
	}
	var out bytes.Buffer
	var nome, tipo string
	if logo {
		img = reduzir(aparar(img), logoLado)
		if err := png.Encode(&out, img); err != nil {
			return gerador.Foto{}, err
		}
		nome, tipo = "p-"+randHex(16)+".png", "image/png"
	} else {
		img = reduzir(img, fotoLado)
		fundo := image.NewRGBA(img.Bounds())
		draw.Draw(fundo, fundo.Bounds(), &image.Uniform{color.White}, image.Point{}, draw.Src)
		draw.Draw(fundo, fundo.Bounds(), img, img.Bounds().Min, draw.Over)
		if err := jpeg.Encode(&out, fundo, &jpeg.Options{Quality: 86}); err != nil {
			return gerador.Foto{}, err
		}
		nome, tipo = "p-"+randHex(16)+".jpg", "image/jpeg"
	}
	if err := os.WriteFile(filepath.Join(dir, nome), out.Bytes(), 0o640); err != nil {
		return gerador.Foto{}, err
	}
	legenda := strings.TrimSpace(strings.TrimSuffix(filepath.Base(nomeOriginal), filepath.Ext(nomeOriginal)))
	if len([]rune(legenda)) > 60 || strings.HasPrefix(strings.ToLower(legenda), "img") || strings.HasPrefix(strings.ToLower(legenda), "dsc") ||
		strings.HasPrefix(strings.ToLower(legenda), "whatsapp") || strings.ContainsAny(legenda, "0123456789") && len(legenda) > 6 {
		legenda = "" // nomes de câmera não viram legenda
	}
	legenda = strings.NewReplacer("-", " ", "_", " ").Replace(legenda)
	return gerador.Foto{URL: "/_nep/arquivo/" + nome, Nome: nome, Tipo: tipo, Tamanho: int64(out.Len()), Legenda: legenda, Album: "Fotos"}, nil
}

// assistenteFotos: POST multipart com "logo" e/ou "fotos"; DELETE ?id=…
// tira uma (ou todas, sem id); GET mostra o que já foi enviado.
func (s *Server) assistenteFotos(rq *request) {
	w, r := rq.w, rq.r
	if o := r.Header.Get("Origin"); o != "" && !sameOrigin(o, r) {
		http.Error(w, "use a própria tela", http.StatusBadRequest)
		return
	}
	d := s.fotosGuardadas()
	salvar := func() bool {
		b, _ := json.Marshal(d)
		if err := s.Store.SetMeta(fotosMeta, string(b)); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return false
		}
		return true
	}
	switch r.Method {
	case http.MethodGet:
		writeJSON(w, s.fotosResposta(d))
		return
	case http.MethodDelete:
		id := r.URL.Query().Get("id")
		if d.Logo != nil && (id == "" || id == filepath.Base(d.Logo.URL)) {
			s.Store.removeFiles([]string{filepath.Base(d.Logo.URL)})
			d.Logo = nil
		}
		var ficam []gerador.Foto
		for _, f := range d.Fotos {
			if id == "" || id == filepath.Base(f.URL) {
				s.Store.removeFiles([]string{filepath.Base(f.URL)})
				continue
			}
			ficam = append(ficam, f)
		}
		d.Fotos = ficam
		if salvar() {
			writeJSON(w, s.fotosResposta(d))
		}
		return
	case http.MethodPost:
	default:
		http.Error(w, "use POST", http.StatusMethodNotAllowed)
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, (fotosMaxFotos+1)*fotosMaxBytes)
	leitor, err := r.MultipartReader()
	if err != nil {
		http.Error(w, "envie o formulário da tela", http.StatusBadRequest)
		return
	}
	var avisos []string
	for {
		part, err := leitor.NextPart()
		if err == io.EOF {
			break
		}
		if err != nil {
			http.Error(w, "o envio foi interrompido", http.StatusBadRequest)
			return
		}
		campo := part.FormName()
		if campo != "logo" && campo != "fotos" {
			part.Close()
			continue
		}
		if campo == "fotos" && len(d.Fotos) >= fotosMaxFotos {
			avisos = append(avisos, fmt.Sprintf("o site usa até %d fotos; as outras ficaram de fora", fotosMaxFotos))
			part.Close()
			continue
		}
		img, err := lerImagem(part)
		part.Close()
		if err != nil {
			avisos = append(avisos, part.FileName()+": "+err.Error())
			continue
		}
		f, err := s.gravarImagem(img, campo == "logo", part.FileName())
		if err != nil {
			http.Error(w, "não consegui gravar a imagem: "+err.Error(), http.StatusInternalServerError)
			return
		}
		if campo == "logo" {
			if d.Logo != nil {
				s.Store.removeFiles([]string{filepath.Base(d.Logo.URL)})
			}
			d.Logo = &f
		} else {
			d.Fotos = append(d.Fotos, f)
		}
	}
	if !salvar() {
		return
	}
	resp := s.fotosResposta(d)
	if len(avisos) > 0 {
		resp["avisos"] = uniq(avisos)
	}
	writeJSON(w, resp)
}

func uniq(xs []string) []string {
	visto := map[string]bool{}
	var out []string
	for _, x := range xs {
		if !visto[x] {
			visto[x] = true
			out = append(out, x)
		}
	}
	return out
}
