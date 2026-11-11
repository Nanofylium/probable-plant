package web

// Usuários do painel: o dono (criado na instalação, com acesso a tudo) e
// editores, que só cuidam do conteúdo, dos textos e dos menus. Editores não
// veem os dados enviados pelos visitantes (formulários, arquivos restritos,
// LGPD, visitas) nem as páginas com `auth`.

import (
	"encoding/json"
	"net/http"
	"net/url"
	"regexp"
	"sort"
	"strings"
	"unicode/utf8"
)

type panelUser struct {
	Nome string `json:"nome"`
	Hash string `json:"hash"`
	V    string `json:"v"` // muda ao trocar a senha ou remover: derruba as sessões abertas
}

var userRe = regexp.MustCompile(`^[a-z0-9][a-z0-9._-]{2,29}$`)

func (s *Server) users() map[string]panelUser {
	m := map[string]panelUser{}
	if v := s.Store.Meta("usuarios"); v != "" {
		_ = json.Unmarshal([]byte(v), &m)
	}
	return m
}

func (s *Server) saveUsers(m map[string]panelUser) error {
	b, err := json.Marshal(m)
	if err != nil {
		return err
	}
	return s.Store.SetMeta("usuarios", string(b))
}

// checkUser confere a sessão de um editor: removido ou com senha trocada, sai.
func (s *Server) checkUser(sess *Session) {
	if sess.AdminAt == 0 || sess.U == "" {
		return
	}
	if u, ok := s.users()[sess.U]; !ok || u.V != sess.UV {
		sess.logout()
	}
}

// editorPaths são as telas do painel que um editor pode abrir.
var editorPaths = map[string]bool{"/": true, "/logout": true, "/conteudo": true, "/conteudo/editar": true, "/textos": true, "/delete": true, "/construtor/arquivo": true,
	"/conteudo/versao": true, "/lixeira": true, "/comentarios": true, "/midia": true}

// usersPage é /_admin/usuarios (só o dono).
func (a *adminPage) usersPage(post bool) error {
	r := a.rq.r
	msg, ok := "", ""
	if post {
		m := a.s.users()
		login := strings.ToLower(strings.TrimSpace(r.PostForm.Get("usuario")))
		switch r.PostForm.Get("acao") {
		case "criar", "senha":
			nome := strings.TrimSpace(r.PostForm.Get("nome"))
			pw := r.PostForm.Get("senha")
			u, exists := m[login]
			switch {
			case !userRe.MatchString(login):
				msg = "O usuário precisa ter de 3 a 30 letras minúsculas, números, ponto, - ou _."
			case r.PostForm.Get("acao") == "criar" && exists:
				msg = "Já existe um usuário com esse nome."
			case r.PostForm.Get("acao") == "senha" && !exists:
				msg = "Usuário não encontrado."
			case utf8.RuneCountInString(pw) < 10:
				msg = "A senha precisa ter pelo menos 10 caracteres."
			case utf8.RuneCountInString(nome) > 60:
				msg = "O nome pode ter no máximo 60 caracteres."
			default:
				if nome == "" {
					nome = u.Nome
				}
				if nome == "" {
					nome = login
				}
				m[login] = panelUser{Nome: nome, Hash: hashPassword(pw), V: randHex(8)}
				if err := a.s.saveUsers(m); err != nil {
					return err
				}
				ok = "Pronto. Passe o usuário e a senha para a pessoa por um canal seguro."
			}
		case "remover":
			if _, exists := m[login]; exists {
				delete(m, login)
				if err := a.s.saveUsers(m); err != nil {
					return err
				}
				ok = "Usuário removido. Se estava com o painel aberto, já foi desconectado."
			}
		}
		if msg == "" {
			a.s.redirect(a.rq, a.base+"/usuarios?ok="+url.QueryEscape(ok), http.StatusSeeOther)
			return nil
		}
	}
	if ok == "" {
		ok = r.URL.Query().Get("ok")
	}
	var b strings.Builder
	b.WriteString(cmsCSS + errP(msg))
	if ok != "" {
		b.WriteString(`<p class="ok">` + esc(ok) + `</p>`)
	}
	b.WriteString(`<p class="muted">Você é o <strong>dono</strong>: vê tudo. <strong>Editores</strong> entram com usuário e senha e só cuidam do conteúdo, dos textos e dos menus. Eles não veem formulários enviados, arquivos restritos, visitas nem a página LGPD.</p>`)
	m := a.s.users()
	names := make([]string, 0, len(m))
	for k := range m {
		names = append(names, k)
	}
	sort.Strings(names)
	if len(names) > 0 {
		b.WriteString(`<table><tr><th>Usuário</th><th>Nome</th><th></th></tr>`)
		for _, k := range names {
			b.WriteString(`<tr><td>` + esc(k) + `</td><td>` + esc(m[k].Nome) + `</td><td class="row">` +
				`<form method="post" class="row">` + a.tok() + `<input type="hidden" name="acao" value="senha"><input type="hidden" name="usuario" value="` + esc(k) + `">` +
				`<input type="password" name="senha" placeholder="Nova senha" minlength="10" required autocomplete="new-password"><button>Trocar senha</button></form>` +
				`<form method="post" onsubmit="return confirm('Remover ` + esc(k) + `?')">` + a.tok() + `<input type="hidden" name="acao" value="remover"><input type="hidden" name="usuario" value="` + esc(k) + `"><button class="ghost">Remover</button></form></td></tr>`)
		}
		b.WriteString(`</table>`)
	}
	b.WriteString(`<h2 style="font-size:1.1rem;margin-top:24px">Novo editor</h2><form class="cms" method="post" style="max-width:420px">` + a.tok() + `<input type="hidden" name="acao" value="criar">` +
		`<label>Usuário (para entrar) <input name="usuario" required pattern="[a-z0-9][a-z0-9._\-]{2,29}" autocomplete="off" placeholder="maria"></label>` +
		`<label>Nome <input name="nome" maxlength="60" placeholder="Maria Souza"></label>` +
		`<label>Senha (mín. 10) <input type="password" name="senha" required minlength="10" autocomplete="new-password"></label><button>Criar editor</button></form>`)
	return a.out("Usuários do painel", b.String(), true)
}
