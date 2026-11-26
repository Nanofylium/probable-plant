package gerador

import (
	"net/url"
	"regexp"
	"strings"
)

// Dados são as informações que a pessoa digitou na tela de criar o site. Elas
// valem mais que o que a IA ou as regras acharam na descrição.
type Dados struct {
	Nome      string `json:"nome"`
	Tipo      string `json:"tipo"` // um ID de setor (ver Tipos); vazio = o Nepenthes descobre
	Cidade    string `json:"cidade"`
	WhatsApp  string `json:"whatsapp"`
	Email     string `json:"email"`
	Endereco  string `json:"endereco"`
	Horario   string `json:"horario"`
	Instagram string `json:"instagram"`
	Facebook  string `json:"facebook"`
	TikTok    string `json:"tiktok"`
	YouTube   string `json:"youtube"`
	LinkedIn  string `json:"linkedin"`
	X         string `json:"x"`
}

// redesConhecidas: nome, endereço do perfil (para quem digita só o @) e os
// domínios aceitos num link colado.
var redesConhecidas = []struct {
	Nome, Base string
	Dominios   []string
}{
	{"Facebook", "https://facebook.com/", []string{"facebook.com", "fb.com"}},
	{"TikTok", "https://www.tiktok.com/@", []string{"tiktok.com"}},
	{"YouTube", "https://www.youtube.com/@", []string{"youtube.com", "youtu.be"}},
	{"LinkedIn", "https://www.linkedin.com/company/", []string{"linkedin.com"}},
	{"X", "https://x.com/", []string{"x.com", "twitter.com"}},
}

var reUsuarioRede = regexp.MustCompile(`^@?([\p{L}\p{N}._-]{1,60})$`)

// LinkRede transforma o que a pessoa digitou (@perfil, perfil ou um link) no
// endereço do perfil. Link de outro site não vale (vazio).
func LinkRede(nome, v string) string {
	v = strings.TrimSpace(v)
	if v == "" {
		return ""
	}
	for _, r := range redesConhecidas {
		if r.Nome != nome {
			continue
		}
		if m := reUsuarioRede.FindStringSubmatch(v); m != nil {
			return r.Base + m[1]
		}
		u := v
		if !strings.Contains(u, "://") {
			u = "https://" + u
		}
		pu, err := url.Parse(u)
		if err != nil || pu.Scheme != "https" && pu.Scheme != "http" || pu.User != nil || len(u) > 200 {
			return ""
		}
		h := strings.TrimPrefix(strings.ToLower(pu.Hostname()), "www.")
		h = strings.TrimPrefix(h, "m.")
		for _, d := range r.Dominios {
			if h == d || strings.HasSuffix(h, "."+d) {
				pu.Scheme = "https"
				return pu.String()
			}
		}
	}
	return ""
}

// Tipos são os ramos que a tela oferece para escolher (vazio = o Nepenthes
// descobre pela descrição).
func Tipos() [][2]string {
	out := [][2]string{{generico.ID, generico.Rotulo}}
	for _, s := range setores {
		out = append(out, [2]string{s.ID, s.Rotulo})
	}
	return out
}

var (
	reEmail   = regexp.MustCompile(`^[^@\s<>"]{1,64}@[^@\s<>"]{1,120}\.[a-zA-Z]{2,}$`)
	reDigitos = regexp.MustCompile(`\D`)
)

// TemTipo diz se o ID existe entre os ramos.
func TemTipo(id string) bool {
	for _, s := range append([]Setor{generico}, setores...) {
		if s.ID == id {
			return true
		}
	}
	return false
}

// UsarDados aplica o que a pessoa digitou ao plano.
func UsarDados(p *Plano, d Dados) {
	if d.Tipo != "" && d.Tipo != p.Setor.ID {
		for _, s := range append([]Setor{generico}, setores...) {
			if s.ID == d.Tipo {
				p.Setor = completar(s)
				if len(p.Pedidos) == 0 && len(p.Tirados) == 0 {
					p.Modulos = append([]string(nil), p.Setor.Modulos...)
				} else {
					p.Modulos = juntarModulos(p.Setor.Modulos, p.Pedidos, p.Tirados)
				}
				p.RamoAchado = true
				p.Notas = append(p.Notas, "tipo escolhido: "+s.Rotulo)
				break
			}
		}
	}
	if n := curto(d.Nome, 60); n != "" {
		p.Nome, p.NomeAchado = n, true
	}
	if c := curto(d.Cidade, 40); c != "" {
		p.Cidade = c
	}
	if w := reDigitos.ReplaceAllString(d.WhatsApp, ""); len(w) >= 10 && len(w) <= 13 {
		if len(w) <= 11 {
			w = "55" + w // número do Brasil sem o 55
		}
		p.WhatsApp = "https://wa.me/" + w
		p.Telefone = curto(d.WhatsApp, 30)
	}
	if e := strings.TrimSpace(d.Email); reEmail.MatchString(e) {
		p.Email = e
	}
	p.Endereco = curto(d.Endereco, 120)
	p.Horario = curto(d.Horario, 120)
	if i := strings.Trim(strings.TrimSpace(d.Instagram), "@/ "); i != "" && regexp.MustCompile(`^[A-Za-z0-9._]{1,30}$`).MatchString(i) {
		p.Instagram = i
	}
	for _, r := range [][2]string{{"Facebook", d.Facebook}, {"TikTok", d.TikTok}, {"YouTube", d.YouTube}, {"LinkedIn", d.LinkedIn}, {"X", d.X}} {
		if u := LinkRede(r[0], r[1]); u != "" {
			p.Redes = append(p.Redes, Rede{Nome: r[0], URL: u})
		}
	}
}

// juntarModulos: os do ramo, mais os pedidos, menos os tirados, na ordem de ModulosTodos.
func juntarModulos(base, pedidos, tirados []string) []string {
	tem := map[string]bool{}
	for _, m := range base {
		tem[m] = true
	}
	for _, m := range pedidos {
		tem[m] = true
	}
	for _, m := range tirados {
		delete(tem, m)
	}
	var out []string
	for _, m := range ModulosTodos {
		if tem[m] {
			out = append(out, m)
		}
	}
	return out
}

// ouExemplo: o dado digitado (escapado), ou o texto de exemplo.
func ouExemplo(v, exemplo string) string {
	if strings.TrimSpace(v) == "" {
		return exemplo
	}
	return strings.ReplaceAll(esc(v), "{", "&#123;")
}

// DescricaoComDados junta os dados digitados à descrição, no jeito que as
// regras e a IA entendem.
func DescricaoComDados(desc string, d Dados) string {
	out := strings.TrimSpace(desc)
	junta := func(s string) {
		if out == "" {
			out = s
		} else {
			out += ", " + s
		}
	}
	if d.Tipo != "" {
		for _, t := range Tipos() {
			if t[0] == d.Tipo {
				junta(t[1])
			}
		}
	}
	if n := curto(d.Nome, 60); n != "" {
		junta(`chamado "` + strings.Trim(n, `"“”`) + `"`)
	}
	if c := curto(d.Cidade, 40); c != "" {
		junta("em " + c)
	}
	return out
}

// TirarPerguntasRespondidas: não pergunta de novo o que a pessoa já digitou.
func TirarPerguntasRespondidas(ps []string, d Dados) []string {
	var out []string
	for _, p := range ps {
		q := semAcento(p)
		if d.Nome != "" && strings.Contains(q, "nome") ||
			d.Cidade != "" && (strings.Contains(q, "cidade") || strings.Contains(q, "bairro")) ||
			d.WhatsApp != "" && (strings.Contains(q, "whatsapp") || strings.Contains(q, "telefone")) ||
			d.Email != "" && strings.Contains(q, "e-mail") ||
			d.Endereco != "" && strings.Contains(q, "endereco") ||
			d.Horario != "" && strings.Contains(q, "horario de funcionamento") {
			continue
		}
		out = append(out, p)
	}
	return out
}
