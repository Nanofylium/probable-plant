package web

// Instagram na tela da tag <nepenthes>: a pessoa manda o zip que o próprio
// Instagram entrega ("Baixar suas informações", formato JSON) e o Nepenthes
// usa o perfil, a bio, as legendas e as fotos para montar o site (pacote
// instagram). Também dá para informar só o @, que vira o link do rodapé.

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/Nanofylium/probable-plant/gerador"
	"github.com/Nanofylium/probable-plant/instagram"
)

const (
	instaMeta     = "assistente:instagram"
	instaMaxZip   = 4 << 30 // o arquivo do Instagram com fotos pode ser grande
	instaMaxFotos = 12
	instaMaxFoto  = 8 << 20
)

// instaDados é o que fica guardado entre o envio e a criação do site.
type instaDados struct {
	Usuario string         `json:"usuario"`
	Nome    string         `json:"nome"`
	Bio     string         `json:"bio"`
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
	var zipTmp string
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
	}
	b, _ := json.Marshal(d)
	if err := s.Store.SetMeta(instaMeta, string(b)); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, map[string]any{"instagram": s.instaResposta(d)})
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
	for _, foto := range d.Fotos { // trocou o zip: as fotos antigas saem
		s.Store.removeFiles([]string{filepath.Base(foto.URL)})
	}
	d.Fotos = nil
	if perfil.Usuario != "" {
		d.Usuario = strings.ToLower(perfil.Usuario)
	}
	d.Nome, d.Bio, d.Posts = perfil.Nome, perfil.Bio, len(perfil.Posts)
	d.Resumo = perfil.Resumo(2500)
	dir := filepath.Join(s.opt.DataDir, "arquivos")
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return err
	}
	// primeiro a foto principal de cada post, depois as outras dos carrosséis
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
				if foto, err := s.instaCopiar(arq, caminho, dir); err == nil {
					foto.Legenda = po.Legenda
					d.Fotos = append(d.Fotos, foto)
				}
			}
		}
	}
	return nil
}

func (s *Server) instaCopiar(arq *instagram.Arquivo, caminho, dir string) (gerador.Foto, error) {
	r, tam, err := arq.Foto(caminho)
	if err != nil {
		return gerador.Foto{}, err
	}
	defer r.Close()
	if tam > instaMaxFoto {
		return gerador.Foto{}, fmt.Errorf("foto grande demais")
	}
	b, err := io.ReadAll(io.LimitReader(r, instaMaxFoto+1))
	if err != nil {
		return gerador.Foto{}, err
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
	return gerador.Foto{URL: "/_nep/arquivo/" + nome, Nome: filepath.Base(caminho), Tipo: tipo, Tamanho: int64(len(b))}, nil
}
