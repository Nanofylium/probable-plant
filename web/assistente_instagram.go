package web

// Instagram na tela da tag <nepenthes>: a pessoa manda o zip que o próprio
// Instagram entrega ("Baixar suas informações", formato JSON) e o Nepenthes
// usa o perfil, a bio, as legendas e as fotos para montar o site (pacote
// instagram). Também dá para informar só o @, que vira o link do rodapé, ou
// mandar prints (capturas de tela) do perfil ou de posts: o Nepenthes acha as
// fotos dentro de cada print e recorta (instagram.Recortar).

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"image"
	"image/jpeg"
	_ "image/png"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/Nanofylium/probable-plant/gerador"
	"github.com/Nanofylium/probable-plant/instagram"
)

const (
	instaMeta     = "assistente:instagram"
	instaMaxZip   = 4 << 30 // o arquivo do Instagram com fotos pode ser grande
	instaMaxFotos = 12
	instaMaxFoto  = 8 << 20
	instaMaxPrint = 25 << 20
	instaMaxPixel = 40_000_000 // 40 megapixels por print
)

// instaDados é o que fica guardado entre o envio e a criação do site.
type instaDados struct {
	Usuario string         `json:"usuario"`
	Nome    string         `json:"nome"`
	Bio     string         `json:"bio"`
	Site    string         `json:"site,omitempty"`
	Resumo  string         `json:"resumo"`
	Posts   int            `json:"posts"`
	Fotos   []gerador.Foto `json:"fotos"`
}

func (s *Server) instaGuardado() *instaDados {
	var d instaDados
	if json.Unmarshal([]byte(s.Store.Meta(instaMeta)), &d) != nil || d.Usuario == "" && len(d.Fotos) == 0 && d.Resumo == "" {
		return nil
	}
	return &d
}

func (s *Server) instaResposta(d *instaDados) map[string]any {
	if d == nil {
		return nil
	}
	var urls []string
	for _, f := range d.Fotos {
		urls = append(urls, s.url(f.URL))
	}
	return map[string]any{"usuario": d.Usuario, "nome": d.Nome, "bio": d.Bio, "posts": d.Posts, "fotos": urls}
}

// assistenteInstagram: POST multipart com "usuario" e/ou "zip"; DELETE esquece.
func (s *Server) assistenteInstagram(rq *request) {
	w, r := rq.w, rq.r
	if o := r.Header.Get("Origin"); o != "" && !sameOrigin(o, r) {
		http.Error(w, "use a própria tela", http.StatusBadRequest)
		return
	}
	switch r.Method {
	case http.MethodGet:
		writeJSON(w, map[string]any{"instagram": s.instaResposta(s.instaGuardado())})
		return
	case http.MethodDelete:
		if d := s.instaGuardado(); d != nil {
			for _, f := range d.Fotos {
				s.Store.removeFiles([]string{filepath.Base(f.URL)})
			}
		}
		_ = s.Store.SetMeta(instaMeta, "")
		writeJSON(w, map[string]any{"ok": true})
		return
	case http.MethodPost:
	default:
		http.Error(w, "use POST", http.StatusMethodNotAllowed)
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, instaMaxZip)
	leitor, err := r.MultipartReader()
	if err != nil {
		http.Error(w, "envie o formulário da tela", http.StatusBadRequest)
		return
	}
	d := &instaDados{}
	if velho := s.instaGuardado(); velho != nil {
		*d = *velho
	}
	var zipTmp, token string
	prints, semFoto := 0, 0
	defer func() {
		if zipTmp != "" {
			os.Remove(zipTmp)
		}
	}()
	for {
		part, err := leitor.NextPart()
		if err == io.EOF {
			break
		}
		if err != nil {
			http.Error(w, "o envio foi interrompido: "+err.Error(), http.StatusBadRequest)
			return
		}
		switch part.FormName() {
		case "usuario":
			b, _ := io.ReadAll(io.LimitReader(part, 200))
			if u := limparUsuario(string(b)); u != "" {
				d.Usuario = u
			}
		case "token": // token gerado no painel do app Meta, colado na tela
			b, _ := io.ReadAll(io.LimitReader(part, 2048))
			token = strings.TrimSpace(string(b))
		case "print": // print do perfil ou de um post: recorta as fotos
			prints++
			fotos, err := s.instaPrint(part, d)
			if err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			if fotos == 0 {
				semFoto++
			}
		case "zip":
			f, err := os.CreateTemp("", "nep-instagram-*.zip")
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			zipTmp = f.Name()
			_, err = io.Copy(f, part)
			f.Close()
			if err != nil {
				http.Error(w, "o envio do zip foi interrompido: "+err.Error(), http.StatusBadRequest)
				return
			}
		}
		part.Close()
	}
	if zipTmp != "" {
		if err := s.instaLerZip(zipTmp, d); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
	} else if token != "" {
		if err := s.instaLerAPI(r.Context(), token, d); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
	}
	b, _ := json.Marshal(d)
	if err := s.Store.SetMeta(instaMeta, string(b)); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	res := map[string]any{"instagram": s.instaResposta(d)}
	if prints > 0 {
		res["prints"] = map[string]int{"enviados": prints, "sem_foto": semFoto}
	}
	writeJSON(w, res)
}

// instaPrint lê um print (png ou jpg), acha as fotos dentro dele e guarda cada
// recorte como foto do site (até 12 no total). Devolve quantas aproveitou.
func (s *Server) instaPrint(part io.Reader, d *instaDados) (int, error) {
	b, err := io.ReadAll(io.LimitReader(part, instaMaxPrint+1))
	if err != nil {
		return 0, fmt.Errorf("o envio do print foi interrompido: %v", err)
	}
	if len(b) > instaMaxPrint {
		return 0, fmt.Errorf("print grande demais (até %d MB)", instaMaxPrint>>20)
	}
	cfg, _, err := image.DecodeConfig(bytes.NewReader(b))
	if err != nil {
		return 0, fmt.Errorf("o print precisa ser PNG ou JPG")
	}
	if cfg.Width*cfg.Height > instaMaxPixel {
		return 0, fmt.Errorf("print grande demais (%dx%d)", cfg.Width, cfg.Height)
	}
	img, _, err := image.Decode(bytes.NewReader(b))
	if err != nil {
		return 0, fmt.Errorf("não consegui ler o print: %v", err)
	}
	dir := filepath.Join(s.opt.DataDir, "arquivos")
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return 0, err
	}
	var marcas []instagram.Marca // das fotos que já vieram de prints: não repetir
	for _, f := range d.Fotos {
		if strings.HasPrefix(f.Nome, "print-") {
			if fi, err := os.Open(filepath.Join(dir, filepath.Base(f.URL))); err == nil {
				if im, err := jpeg.Decode(fi); err == nil {
					marcas = append(marcas, instagram.Marcar(im))
				}
				fi.Close()
			}
		}
	}
	n := 0
recortes:
	for _, r := range instagram.Recortar(img) {
		if len(d.Fotos) >= instaMaxFotos {
			break
		}
		foto := instagram.Sub(img, r)
		m := instagram.Marcar(foto)
		for _, o := range marcas {
			if m.Parecida(o) {
				n++ // já está no site: conta como aproveitada
				continue recortes
			}
		}
		marcas = append(marcas, m)
		var out bytes.Buffer
		if err := jpeg.Encode(&out, foto, &jpeg.Options{Quality: 90}); err != nil {
			continue
		}
		nome := "p-" + randHex(16) + ".jpg"
		if err := os.WriteFile(filepath.Join(dir, nome), out.Bytes(), 0o640); err != nil {
			return n, err
		}
		d.Fotos = append(d.Fotos, gerador.Foto{URL: "/_nep/arquivo/" + nome, Nome: fmt.Sprintf("print-%d.jpg", len(d.Fotos)+1),
			Tipo: "image/jpeg", Tamanho: int64(out.Len())})
		n++
	}
	return n, nil
}

func limparUsuario(u string) string {
	u = strings.TrimSpace(u)
	u = strings.TrimSuffix(u, "/")
	if i := strings.LastIndex(u, "/"); i >= 0 { // colou o link do perfil
		u = u[i+1:]
	}
	u = strings.TrimPrefix(u, "@")
	ok := u != "" && len(u) <= 30
	for _, c := range u {
		ok = ok && (c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9' || c == '.' || c == '_')
	}
	if !ok {
		return ""
	}
	return strings.ToLower(u)
}

// instaLerZip lê o zip e copia as fotos dos posts mais novos para os
// arquivos públicos do site (data/arquivos/p-....jpg).
func (s *Server) instaLerZip(caminho string, d *instaDados) error {
	f, err := os.Open(caminho)
	if err != nil {
		return err
	}
	defer f.Close()
	fi, _ := f.Stat()
	arq, perfil, err := instagram.Abrir(f, fi.Size())
	if err != nil {
		return err
	}
	return s.instaUsar(perfil, d, arq.Foto)
}

// instaLerAPI lê pela API oficial (token do login ou colado) e baixa as fotos.
func (s *Server) instaLerAPI(ctx context.Context, token string, d *instaDados) error {
	perfil, err := instagram.LerPelaAPI(ctx, token, 30)
	if err != nil {
		return err
	}
	return s.instaUsar(perfil, d, func(u string) (io.ReadCloser, int64, error) {
		if !strings.HasPrefix(u, "https://") {
			return nil, 0, fmt.Errorf("endereço de foto estranho")
		}
		req, _ := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
		res, err := instaHTTP.Do(req)
		if err != nil {
			return nil, 0, err
		}
		if res.StatusCode != 200 {
			res.Body.Close()
			return nil, 0, fmt.Errorf("foto respondeu %d", res.StatusCode)
		}
		return res.Body, res.ContentLength, nil
	})
}

var instaHTTP = &http.Client{Timeout: 30 * time.Second}

// instaUsar guarda perfil, bio, legendas e até 12 fotos (primeiro a principal
// de cada post, depois as outras dos carrosséis) nos arquivos públicos do site.
func (s *Server) instaUsar(perfil *instagram.Perfil, d *instaDados, abrir func(string) (io.ReadCloser, int64, error)) error {
	for _, foto := range d.Fotos { // trocou a fonte: as fotos antigas saem
		s.Store.removeFiles([]string{filepath.Base(foto.URL)})
	}
	d.Fotos = nil
	if perfil.Usuario != "" {
		d.Usuario = strings.ToLower(perfil.Usuario)
	}
	d.Nome, d.Bio, d.Site, d.Posts = perfil.Nome, perfil.Bio, perfil.Site, len(perfil.Posts)
	d.Resumo = perfil.Resumo(2500)
	dir := filepath.Join(s.opt.DataDir, "arquivos")
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return err
	}
	for rodada := 0; rodada < 2 && len(d.Fotos) < instaMaxFotos; rodada++ {
		for _, po := range perfil.Posts {
			if len(d.Fotos) >= instaMaxFotos {
				break
			}
			fotos := po.Fotos
			if rodada == 0 && len(fotos) > 1 {
				fotos = fotos[:1]
			} else if rodada == 1 {
				if len(fotos) < 2 {
					continue
				}
				fotos = fotos[1:]
			}
			for _, caminho := range fotos {
				if len(d.Fotos) >= instaMaxFotos {
					break
				}
				if foto, err := s.instaCopiar(abrir, caminho, dir); err == nil {
					foto.Legenda, foto.Link = po.Legenda, po.Link
					d.Fotos = append(d.Fotos, foto)
				}
			}
		}
	}
	return nil
}

func (s *Server) instaCopiar(abrir func(string) (io.ReadCloser, int64, error), caminho, dir string) (gerador.Foto, error) {
	r, tam, err := abrir(caminho)
	if err != nil {
		return gerador.Foto{}, err
	}
	defer r.Close()
	if tam > instaMaxFoto {
		return gerador.Foto{}, fmt.Errorf("foto grande demais")
	}
	b, err := io.ReadAll(io.LimitReader(r, instaMaxFoto+1))
	if err != nil || len(b) > instaMaxFoto {
		return gerador.Foto{}, fmt.Errorf("foto grande demais ou incompleta")
	}
	tipo := http.DetectContentType(b)
	ext, ok := fileKinds[tipo]
	if !ok || ext == ".pdf" || ext == ".gif" {
		return gerador.Foto{}, fmt.Errorf("não é foto")
	}
	nome := "p-" + randHex(16) + ext
	if err := os.WriteFile(filepath.Join(dir, nome), b, 0o640); err != nil {
		return gerador.Foto{}, err
	}
	base := filepath.Base(caminho)
	if i := strings.IndexAny(base, "?#"); i >= 0 {
		base = base[:i]
	}
	return gerador.Foto{URL: "/_nep/arquivo/" + nome, Nome: base, Tipo: tipo, Tamanho: int64(len(b))}, nil
}

// ---------- Entrar com Instagram (API oficial) ----------

const (
	instaAppMeta    = "instagram:app"
	instaEstadoMeta = "instagram:estado"
)

type instaApp struct {
	ID      string `json:"id"`
	Segredo string `json:"segredo"`
}

// instaAppConf: o app Meta do dono do Nepenthes (painel do app > Instagram >
// API setup with Instagram login). Vem da tela ou de NEP_INSTA_APP_ID e
// NEP_INSTA_SEGREDO. O segredo nunca volta para o navegador.
func (s *Server) instaAppConf() instaApp {
	var a instaApp
	_ = json.Unmarshal([]byte(s.Store.Meta(instaAppMeta)), &a)
	if v := os.Getenv("NEP_INSTA_APP_ID"); v != "" {
		a.ID = v
	}
	if v := os.Getenv("NEP_INSTA_SEGREDO"); v != "" {
		a.Segredo = v
	}
	return a
}

// instaVolta é o endereço que vai em "Valid OAuth Redirect URIs" no app Meta.
func (s *Server) instaVolta(r *http.Request) string {
	esquema := "http"
	if r.TLS != nil || strings.EqualFold(r.Header.Get("X-Forwarded-Proto"), "https") {
		esquema = "https"
	}
	return esquema + "://" + r.Host + s.url("/_nep/assistente/instagram/volta")
}

func (s *Server) instaAppEstado(r *http.Request) map[string]any {
	a := s.instaAppConf()
	return map[string]any{"ligado": a.ID != "" && a.Segredo != "", "app_id": a.ID, "volta": s.instaVolta(r)}
}

func (s *Server) assistenteInstagramApp(rq *request) {
	w, r := rq.w, rq.r
	if r.Method == http.MethodPost {
		if o := r.Header.Get("Origin"); o != "" && !sameOrigin(o, r) {
			http.Error(w, "use a própria tela", http.StatusBadRequest)
			return
		}
		var in struct {
			ID      string `json:"app_id"`
			Segredo string `json:"segredo"`
		}
		if json.NewDecoder(io.LimitReader(r.Body, 4096)).Decode(&in) != nil {
			http.Error(w, "dados inválidos", http.StatusBadRequest)
			return
		}
		a := s.instaAppConf()
		if id := strings.TrimSpace(in.ID); id != "" {
			a.ID = id
		}
		if seg := strings.TrimSpace(in.Segredo); seg != "" {
			a.Segredo = seg
		}
		b, _ := json.Marshal(a)
		if err := s.Store.SetMeta(instaAppMeta, string(b)); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	}
	writeJSON(w, s.instaAppEstado(r))
}

// assistenteInstagramEntrar manda a pessoa para o login do Instagram.
func (s *Server) assistenteInstagramEntrar(rq *request) {
	a := s.instaAppConf()
	if a.ID == "" || a.Segredo == "" {
		http.Error(rq.w, "ligue o app do Instagram primeiro (App ID e chave secreta)", http.StatusBadRequest)
		return
	}
	estado := randHex(16)
	_ = s.Store.SetMeta(instaEstadoMeta, estado+" "+fmt.Sprint(time.Now().Unix()))
	http.Redirect(rq.w, rq.r, instagram.LinkEntrar(a.ID, s.instaVolta(rq.r), estado), http.StatusFound)
}

// assistenteInstagramVolta recebe a volta do login, lê o perfil e os posts e
// volta para a tela.
func (s *Server) assistenteInstagramVolta(rq *request) {
	r := rq.r
	fim := func(msg string) {
		dest := s.url("/") + "?instagram=ok"
		if msg != "" {
			dest = s.url("/") + "?instagram_erro=" + urlQ(msg)
		}
		http.Redirect(rq.w, r, dest, http.StatusFound)
	}
	partes := strings.Fields(s.Store.Meta(instaEstadoMeta))
	_ = s.Store.SetMeta(instaEstadoMeta, "")
	var quando int64
	if len(partes) == 2 {
		fmt.Sscan(partes[1], &quando)
	}
	if len(partes) != 2 || r.URL.Query().Get("state") != partes[0] || time.Now().Unix()-quando > 900 {
		fim("o login do Instagram expirou ou não veio desta tela; tente de novo")
		return
	}
	if e := r.URL.Query().Get("error_description"); e != "" {
		fim("o Instagram não autorizou: " + e)
		return
	}
	codigo := r.URL.Query().Get("code")
	if codigo == "" {
		fim("o Instagram não mandou o código de acesso")
		return
	}
	a := s.instaAppConf()
	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Minute)
	defer cancel()
	token, err := instagram.TrocarCodigo(ctx, a.ID, a.Segredo, s.instaVolta(r), codigo)
	if err != nil {
		fim(err.Error())
		return
	}
	d := &instaDados{}
	if velho := s.instaGuardado(); velho != nil {
		*d = *velho
	}
	if err := s.instaLerAPI(ctx, token, d); err != nil {
		fim(err.Error())
		return
	}
	b, _ := json.Marshal(d)
	if err := s.Store.SetMeta(instaMeta, string(b)); err != nil {
		fim(err.Error())
		return
	}
	fim("")
}
