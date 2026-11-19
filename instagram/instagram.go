// Package instagram lê o arquivo que o próprio Instagram entrega em
// "Baixar suas informações" (Central de Contas > Suas informações e
// permissões > Baixar suas informações, formato JSON) e tira dele o perfil
// (nome, @, bio, site), as legendas e as fotos dos posts.
//
// É o caminho que o Instagram oferece ao dono da conta: não usa login, senha
// nem raspagem do site do Instagram (que os termos de uso proíbem e que o
// Instagram bloqueia), e funciona com conta pessoal, comercial ou de criador.
//
// O formato muda com o tempo e com o idioma da conta: o leitor procura os
// arquivos pelo nome em qualquer pasta, aceita os rótulos em inglês e em
// português e conserta o texto que vem com acentos estragados (o arquivo do
// Instagram grava UTF-8 como se fosse Latin-1).
package instagram

import (
	"archive/zip"
	"encoding/json"
	"errors"
	"io"
	"path"
	"sort"
	"strings"
	"time"
	"unicode/utf8"
)

// Perfil é o que sai do arquivo.
type Perfil struct {
	Nome    string `json:"nome"`
	Usuario string `json:"usuario"`
	Bio     string `json:"bio"`
	Site    string `json:"site"`
	Posts   []Post `json:"posts"` // do mais novo para o mais velho
}

// Post é um post com as fotos (caminhos dentro do zip).
type Post struct {
	Legenda string    `json:"legenda"`
	Data    time.Time `json:"data"`
	Fotos   []string  `json:"fotos"` // caminhos no zip ou endereços https (API)
	Link    string    `json:"link,omitempty"`
}

// Arquivo é o zip aberto, para copiar as fotos depois de ler o perfil.
type Arquivo struct {
	z *zip.Reader
	f map[string]*zip.File
}

// Abrir lê o perfil e os posts de um zip do Instagram.
func Abrir(r io.ReaderAt, tamanho int64) (*Arquivo, *Perfil, error) {
	z, err := zip.NewReader(r, tamanho)
	if err != nil {
		return nil, nil, errors.New("o arquivo não é um .zip")
	}
	a := &Arquivo{z: z, f: map[string]*zip.File{}}
	for _, f := range z.File {
		a.f[path.Clean(f.Name)] = f
	}
	p := &Perfil{}
	achou := false
	for _, f := range z.File {
		base := path.Base(f.Name)
		switch {
		case base == "personal_information.json":
			if err := a.lerPerfil(f, p); err == nil {
				achou = true
			}
		case strings.HasPrefix(base, "posts_") && strings.HasSuffix(base, ".json") && !strings.Contains(f.Name, "recently_deleted"):
			if posts, err := a.lerPosts(f); err == nil {
				p.Posts = append(p.Posts, posts...)
				achou = true
			}
		}
	}
	if !achou {
		return nil, nil, errors.New("não achei os dados do Instagram neste zip (use \"Baixar suas informações\", formato JSON)")
	}
	sort.SliceStable(p.Posts, func(i, j int) bool { return p.Posts[i].Data.After(p.Posts[j].Data) })
	return a, p, nil
}

// Foto devolve o conteúdo de uma foto do zip.
func (a *Arquivo) Foto(caminho string) (io.ReadCloser, int64, error) {
	f := a.f[path.Clean(caminho)]
	if f == nil {
		// alguns arquivos citam a mídia a partir de uma subpasta
		for nome, g := range a.f {
			if strings.HasSuffix(nome, "/"+path.Clean(caminho)) {
				f = g
				break
			}
		}
	}
	if f == nil {
		return nil, 0, errors.New("foto não está no zip: " + caminho)
	}
	r, err := f.Open()
	return r, int64(f.UncompressedSize64), err
}

func lerJSON(f *zip.File, v any) error {
	if f.UncompressedSize64 > 64<<20 {
		return errors.New("arquivo grande demais")
	}
	r, err := f.Open()
	if err != nil {
		return err
	}
	defer r.Close()
	return json.NewDecoder(r).Decode(v)
}

type valor struct {
	Value string `json:"value"`
	Href  string `json:"href"`
}

func (a *Arquivo) lerPerfil(f *zip.File, p *Perfil) error {
	var v struct {
		ProfileUser []struct {
			StringMapData map[string]valor `json:"string_map_data"`
		} `json:"profile_user"`
	}
	if err := lerJSON(f, &v); err != nil || len(v.ProfileUser) == 0 {
		return errors.New("perfil vazio")
	}
	m := v.ProfileUser[0].StringMapData
	pega := func(rotulos ...string) string {
		for _, r := range rotulos {
			for k, x := range m {
				if strings.EqualFold(conserta(k), r) && x.Value != "" {
					return strings.TrimSpace(conserta(x.Value))
				}
			}
		}
		return ""
	}
	p.Nome = pega("Name", "Nome")
	p.Usuario = strings.TrimPrefix(pega("Username", "Nome de usuário", "Nome de utilizador"), "@")
	p.Bio = pega("Bio", "Biografia")
	p.Site = pega("Website", "Site", "Site da Web", "Website pessoal")
	return nil
}

type midia struct {
	URI       string `json:"uri"`
	Title     string `json:"title"`
	Timestamp int64  `json:"creation_timestamp"`
}

func (a *Arquivo) lerPosts(f *zip.File) ([]Post, error) {
	var lista []struct {
		Media     []midia `json:"media"`
		Title     string  `json:"title"`
		Timestamp int64   `json:"creation_timestamp"`
	}
	if err := lerJSON(f, &lista); err != nil {
		// versões antigas: {"ig_posts": [...]}
		var v struct {
			Posts []struct {
				Media []midia `json:"media"`
				Title string  `json:"title"`
			} `json:"ig_posts"`
		}
		if err2 := lerJSON(f, &v); err2 != nil {
			return nil, err
		}
		for _, x := range v.Posts {
			lista = append(lista, struct {
				Media     []midia `json:"media"`
				Title     string  `json:"title"`
				Timestamp int64   `json:"creation_timestamp"`
			}{Media: x.Media, Title: x.Title})
		}
	}
	var out []Post
	for _, x := range lista {
		po := Post{Legenda: strings.TrimSpace(conserta(x.Title))}
		ts := x.Timestamp
		for _, m := range x.Media {
			if po.Legenda == "" {
				po.Legenda = strings.TrimSpace(conserta(m.Title))
			}
			if ts == 0 {
				ts = m.Timestamp
			}
			switch strings.ToLower(path.Ext(m.URI)) {
			case ".jpg", ".jpeg", ".png", ".webp":
				po.Fotos = append(po.Fotos, m.URI)
			}
		}
		po.Data = time.Unix(ts, 0)
		if len(po.Fotos) > 0 || po.Legenda != "" {
			out = append(out, po)
		}
	}
	return out, nil
}

// conserta o "Ã£" do arquivo do Instagram: cada byte do UTF-8 veio como um
// caractere Latin-1. Se o texto não estiver nesse estado, volta como está.
func conserta(s string) string {
	b := make([]byte, 0, len(s))
	for _, r := range s {
		if r > 0xff {
			return s
		}
		b = append(b, byte(r))
	}
	if !utf8.Valid(b) {
		return s
	}
	return string(b)
}

// Resumo junta bio e legendas num texto curto para a IA e as regras.
func (p *Perfil) Resumo(max int) string {
	var b strings.Builder
	if p.Nome != "" {
		b.WriteString("Nome no Instagram: " + p.Nome + "\n")
	}
	if p.Usuario != "" {
		b.WriteString("Instagram: @" + p.Usuario + "\n")
	}
	if p.Bio != "" {
		b.WriteString("Bio: " + p.Bio + "\n")
	}
	if p.Site != "" {
		b.WriteString("Site: " + p.Site + "\n")
	}
	n := 0
	for _, po := range p.Posts {
		if po.Legenda == "" {
			continue
		}
		if n == 0 {
			b.WriteString("Legendas dos posts mais recentes:\n")
		}
		l := po.Legenda
		if r := []rune(l); len(r) > 220 {
			l = string(r[:220]) + "…"
		}
		b.WriteString("- " + strings.ReplaceAll(l, "\n", " ") + "\n")
		n++
		if b.Len() > max || n >= 12 {
			break
		}
	}
	s := b.String()
	if len(s) > max {
		s = s[:max]
		for !utf8.ValidString(s) {
			s = s[:len(s)-1]
		}
	}
	return s
}
