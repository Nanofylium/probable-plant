package gerador

import (
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
