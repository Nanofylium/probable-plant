package web

// Painel administrativo embutido em /_admin: coleções, busca, exclusão,
// exportação CSV para o Excel brasileiro e ferramenta LGPD.

import (
	"crypto/pbkdf2"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/csv"
	"encoding/hex"
	"fmt"
	"html"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/Nanofylium/probable-plant/lang"
)

const pbkdfIter = 210_000

func hashPassword(pw string) string {
	salt := randHex(16)
	k, _ := pbkdf2.Key(sha256.New, pw, []byte(salt), pbkdfIter, 32)
	return fmt.Sprintf("pbkdf2-sha256$%d$%s$%s", pbkdfIter, salt, hex.EncodeToString(k))
}

func checkPassword(pw, stored string) bool {
	p := strings.Split(stored, "$")
	if len(p) != 4 || p[0] != "pbkdf2-sha256" {
		return false
	}
	iter, err := strconv.Atoi(p[1])
	if err != nil || iter < 1000 {
		return false
	}
	k, _ := pbkdf2.Key(sha256.New, pw, []byte(p[2]), iter, 32)
	return subtle.ConstantTimeCompare([]byte(hex.EncodeToString(k)), []byte(p[3])) == 1
}

func valuesTable(v url.Values) *lang.Table {
	t := lang.NewTable()
	for k, vs := range v {
		if strings.HasSuffix(k, "[]") {
			l := lang.NewList()
			for _, x := range vs {
				l.Items = append(l.Items, x)
			}
			t.SetStr(strings.TrimSuffix(k, "[]"), l)
		} else if len(vs) > 0 {
			t.SetStr(k, vs[0])
		}
	}
	return t
}

func (s *Server) admin(rq *request) error {
	w, r := rq.w, rq.r
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("X-Robots-Tag", "noindex")
	base := s.opt.Base + "/_admin"
	sub := strings.TrimPrefix(rq.path, "/_admin")
	if sub == "" {
		sub = "/"
	}
	post := r.Method == http.MethodPost
	if post {
		var err error
		if strings.HasPrefix(r.Header.Get("Content-Type"), "multipart/form-data") {
			r.Body = http.MaxBytesReader(w, r.Body, maxUpload)
			if err = r.ParseMultipartForm(8 << 20); err == nil {
				defer r.MultipartForm.RemoveAll()
			}
		} else {
			r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
			err = r.ParseForm()
		}
		if err != nil || !rq.sess.csrfOK(r.PostForm.Get("_nep_csrf")) {
			s.simple(rq, 400, "Sessão expirada", `<p>Volte e tente de novo.</p>`)
			return nil
		}
	}
	a := &adminPage{s: s, rq: rq, base: base}
	if s.Store.Meta("admin_hash") == "" {
		return a.setup(post)
	}
	if sub == "/login" {
		return a.login(post)
	}
	if !rq.sess.isAdmin() {
		s.redirect(rq, base+"/login", http.StatusFound)
		return nil
	}
	switch sub {
	case "/logout":
		if post {
			rq.sess.logout()
		}
		s.redirect(rq, s.opt.Base+"/", http.StatusSeeOther)
	case "/delete":
		c := r.PostForm.Get("c")
		if post && ValidCol(c) {
			id, _ := strconv.ParseInt(r.PostForm.Get("id"), 10, 64)
			if _, err := s.Store.Delete(c, id); err != nil {
				return err
			}
		}
		back := base + "?c=" + url.QueryEscape(c)
		if v := r.PostForm.Get("volta"); strings.HasPrefix(v, "conteudo?") {
			back = base + "/" + v
		}
		s.redirect(rq, back, http.StatusSeeOther)
	case "/conteudo":
		return a.contentList()
	case "/conteudo/editar":
		return a.contentEdit(post)
	case "/textos":
		return a.texts(post)
	case "/csv":
		return a.csv(r.URL.Query().Get("c"))
	case "/lgpd":
		return a.lgpd(post)
	case "/visitas":
		return a.statsPage()
	case "/":
		return a.dash()
	default:
		s.simple(rq, 404, "Não encontrado", "<p>Página do painel não encontrada.</p>")
	}
	return nil
}

type adminPage struct {
	s    *Server
	rq   *request
	base string
	st   *Site
}

func esc(s string) string { return html.EscapeString(s) }

func (a *adminPage) tok() string {
	return `<input type="hidden" name="_nep_csrf" value="` + a.rq.sess.CSRF + `">`
}

func (a *adminPage) out(title, body string, nav bool) error {
	n := ""
	if nav {
		n = `<nav class="row" style="justify-content:space-between;margin-bottom:16px"><div><a href="` + a.base + `">Coleções</a>` + a.navCMS() + `<a href="` + a.base + `/visitas">Visitas</a><a href="` + a.base + `/lgpd">LGPD</a>` +
			`<a href="` + esc(a.s.opt.Base+"/") + `">Ver site</a></div>` +
			`<form method="post" action="` + a.base + `/logout">` + a.tok() + `<button class="ghost">Sair</button></form></nav>`
	}
	a.s.simple(a.rq, 200, title, n+body)
	return nil
}

func errP(msg string) string {
	if msg == "" {
		return ""
	}
	return `<p class="err">` + esc(msg) + `</p>`
}

func (a *adminPage) setup(post bool) error {
	tf := filepath.Join(a.s.opt.DataDir, "SETUP_TOKEN.txt")
	if _, err := os.Stat(tf); err != nil {
		_ = os.WriteFile(tf, []byte(randHex(12)+"\n"), 0o600)
	}
	msg := ""
	if post {
		r := a.rq.r
		b, _ := os.ReadFile(tf)
		tok := strings.TrimSpace(string(b))
		p1, p2 := r.PostForm.Get("senha"), r.PostForm.Get("senha2")
		ok, _ := a.s.Store.Rate("setup:"+a.s.ipKey(r), 10, 900, true)
		switch {
		case !ok:
			msg = "Muitas tentativas. Aguarde 15 minutos."
		case tok == "" || subtle.ConstantTimeCompare([]byte(tok), []byte(strings.TrimSpace(r.PostForm.Get("token")))) != 1:
			msg = "Código de instalação incorreto."
		case utf8.RuneCountInString(p1) < 10:
			msg = "A senha precisa ter pelo menos 10 caracteres."
		case p1 != p2:
			msg = "As senhas não são iguais."
		default:
			if err := a.s.Store.SetMeta("admin_hash", hashPassword(p1)); err != nil {
				return err
			}
			_ = os.Remove(tf)
			a.rq.sess.login()
			a.s.redirect(a.rq, a.base, http.StatusSeeOther)
			return nil
		}
	}
	return a.out("Configurar o painel", errP(msg)+
		`<p>Primeiro acesso. Para provar que você é o dono do site, abra o arquivo <code>data/SETUP_TOKEN.txt</code> (pelo gerenciador de arquivos da hospedagem, FTP ou terminal) e copie o código.</p>`+
		`<form method="post" class="row" style="flex-direction:column;align-items:stretch;max-width:380px">`+a.tok()+
		`<label>Código de instalação<br><input name="token" required autocomplete="off" style="width:100%"></label>`+
		`<label>Nova senha do painel (mín. 10)<br><input type="password" name="senha" required minlength="10" autocomplete="new-password" style="width:100%"></label>`+
		`<label>Repita a senha<br><input type="password" name="senha2" required minlength="10" autocomplete="new-password" style="width:100%"></label>`+
		`<button>Criar acesso</button></form>`, false)
}

func (a *adminPage) login(post bool) error {
	r := a.rq.r
	next := r.URL.Query().Get("next")
	if post {
		next = r.PostForm.Get("next")
	}
	if !localPath(next) {
		next = ""
	}
	msg := ""
	if post {
		key := "login:" + a.s.ipKey(r)
		if ok, _ := a.s.Store.Rate(key, 5, 900, false); !ok {
			msg = "Muitas tentativas. Aguarde 15 minutos."
		} else if !checkPassword(r.PostForm.Get("senha"), a.s.Store.Meta("admin_hash")) {
			_, _ = a.s.Store.Rate(key, 1<<30, 900, true) // só erros contam
			time.Sleep(300 * time.Millisecond)
			msg = "Senha incorreta."
		} else {
			a.rq.sess.login()
			to := a.base
			if next != "" {
				to = a.s.opt.Base + next
			}
			a.s.redirect(a.rq, to, http.StatusSeeOther)
			return nil
		}
	}
	return a.out("Entrar no painel", errP(msg)+
		`<form method="post" class="row">`+a.tok()+`<input type="hidden" name="next" value="`+esc(next)+`">`+
		`<input type="password" name="senha" placeholder="Senha" required autofocus autocomplete="current-password"><button>Entrar</button></form>`, false)
}

func cell(v lang.Value) string {
	if t, ok := v.(*lang.Table); ok {
		if u, ok := t.Get("url"); ok {
			return lang.Str(u)
		}
	}
	if l, ok := v.(*lang.List); ok {
		parts := make([]string, len(l.Items))
		for i, x := range l.Items {
			parts[i] = lang.Str(x)
		}
		return strings.Join(parts, ", ")
	}
	if v == nil {
		return ""
	}
	return lang.Str(v)
}

func fieldKeys(rows []*lang.Table, skip map[string]bool) []string {
	var keys []string
	seen := map[string]bool{}
	for _, t := range rows {
		for i := 0; i < t.Len(); i++ {
			k, _ := t.Entry(i)
			ks := lang.Str(k)
			if !seen[ks] && !skip[ks] {
				seen[ks] = true
				keys = append(keys, ks)
			}
		}
	}
	return keys
}

func (a *adminPage) dash() error {
	q := a.rq.r.URL.Query()
	c := q.Get("c")
	if c == "" {
		cols := a.s.Store.Collections()
		if len(cols) == 0 {
			return a.out("Painel", `<p class="muted">Nenhum dado ainda. Quando alguém enviar um formulário do site, aparece aqui.</p>`, true)
		}
		names := make([]string, 0, len(cols))
		for n := range cols {
			names = append(names, n)
		}
		sort.Strings(names)
		b := `<table><tr><th>Coleção</th><th>Registros</th><th></th></tr>`
		for _, n := range names {
			e := url.QueryEscape(n)
			b += `<tr><td><a href="` + a.base + `?c=` + e + `">` + esc(n) + `</a></td><td>` + strconv.Itoa(cols[n]) +
				`</td><td><a href="` + a.base + `/csv?c=` + e + `">Exportar CSV</a></td></tr>`
		}
		return a.out("Painel", b+`</table>`, true)
	}
	if !ValidCol(c) {
		a.s.simple(a.rq, 400, "Coleção inválida", "")
		return nil
	}
	term := strings.TrimSpace(q.Get("q"))
	var rows []*lang.Table
	for _, r := range a.s.Store.All(c, 0) {
		if term != "" && !strings.Contains(strings.ToLower(r.Data), strings.ToLower(term)) && !strings.Contains(strings.ToLower(jsonUnescape(r.Data)), strings.ToLower(term)) {
			continue
		}
		rows = append(rows, r.ToValue())
	}
	keys := fieldKeys(rows, map[string]bool{"id": true, "created_at": true, "updated_at": true})
	var b strings.Builder
	b.WriteString(`<form class="row" method="get"><input type="hidden" name="c" value="` + esc(c) + `"><input name="q" value="` + esc(term) + `" placeholder="Buscar"><button>Buscar</button>` +
		`<a href="` + a.base + `/csv?c=` + url.QueryEscape(c) + `">Exportar CSV</a></form><p class="muted">` + strconv.Itoa(len(rows)) + ` registro(s)</p><div class="scroll"><table><tr><th>#</th><th>Recebido</th>`)
	for _, k := range keys {
		b.WriteString("<th>" + esc(k) + "</th>")
	}
	b.WriteString("<th></th></tr>")
	for _, t := range rows {
		id, _ := t.Get("id")
		created, _ := t.Get("created_at")
		when := lang.Str(created)
		if tm, err := time.ParseInLocation("2006-01-02 15:04:05", when, time.Local); err == nil {
			when = tm.Format("02/01/2006 15:04")
		}
		b.WriteString("<tr><td>" + lang.Str(id) + "</td><td>" + esc(when) + "</td>")
		for _, k := range keys {
			v, _ := t.Get(k)
			if ft, ok := v.(*lang.Table); ok {
				if u, ok := ft.Get("url"); ok {
					nm, _ := ft.Get("nome")
					b.WriteString(`<td><a href="` + esc(lang.Str(u)) + `" target="_blank" rel="noopener">` + esc(lang.Str(nm)) + `</a></td>`)
					continue
				}
			}
			b.WriteString("<td>" + strings.ReplaceAll(esc(cell(v)), "\n", "<br>") + "</td>")
		}
		b.WriteString(`<td><form method="post" action="` + a.base + `/delete" onsubmit="return confirm('Apagar este registro?')">` + a.tok() +
			`<input type="hidden" name="c" value="` + esc(c) + `"><input type="hidden" name="id" value="` + lang.Str(id) + `"><button class="ghost">Apagar</button></form></td></tr>`)
	}
	b.WriteString("</table></div>")
	return a.out("Coleção: "+c, b.String(), true)
}

// csv exporta para o Excel brasileiro (; e BOM), protegido contra injeção de fórmulas.
func (a *adminPage) csv(c string) error {
	if !ValidCol(c) {
		a.s.simple(a.rq, 400, "Coleção inválida", "")
		return nil
	}
	var rows []*lang.Table
	for _, r := range a.s.Store.All(c, 0) {
		rows = append(rows, r.ToValue())
	}
	keys := append([]string{"id", "created_at"}, fieldKeys(rows, map[string]bool{"id": true, "created_at": true, "updated_at": true})...)
	w := a.rq.w
	a.rq.saveSession()
	w.Header().Set("Content-Type", "text/csv; charset=utf-8")
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s-%s.csv"`, c, time.Now().Format("2006-01-02")))
	w.Write([]byte("\ufeff"))
	cw := csv.NewWriter(w)
	cw.Comma = ';'
	cw.UseCRLF = true
	safe := func(v string) string {
		if v != "" && strings.ContainsRune("=+-@\t\r", rune(v[0])) {
			return "'" + v
		}
		return v
	}
	_ = cw.Write(keys)
	for _, t := range rows {
		rec := make([]string, len(keys))
		for i, k := range keys {
			v, _ := t.Get(k)
			rec[i] = safe(cell(v))
		}
		_ = cw.Write(rec)
	}
	cw.Flush()
	return cw.Error()
}

// lgpd atende pedidos de titulares (LGPD art. 18): localizar e apagar todos os dados de uma pessoa.
func (a *adminPage) lgpd(post bool) error {
	r := a.rq.r
	q := strings.TrimSpace(r.URL.Query().Get("q"))
	if post {
		q = strings.TrimSpace(r.PostForm.Get("q"))
	}
	msg := ""
	if post && r.PostForm.Get("acao") == "apagar" && utf8.RuneCountInString(q) >= 4 {
		n := 0
		for _, rec := range a.s.Store.Search(q) {
			if ok, _ := a.s.Store.Delete(rec.Col, rec.ID); ok {
				n++
			}
		}
		msg = `<p class="ok">` + strconv.Itoa(n) + ` registro(s) apagado(s) com "` + esc(q) + `".</p>`
	}
	b := msg + `<p>Busque por e-mail, telefone ou nome para atender um pedido de acesso ou exclusão de dados (LGPD).</p>` +
		`<form class="row" method="get"><input name="q" value="` + esc(q) + `" placeholder="ex.: maria@email.com" style="min-width:260px"><button>Localizar</button></form>`
	if utf8.RuneCountInString(q) >= 4 {
		found := a.s.Store.Search(q)
		b += `<p class="muted">` + strconv.Itoa(len(found)) + ` registro(s) encontrado(s).</p>`
		if len(found) > 0 {
			b += `<div class="scroll"><table><tr><th>Coleção</th><th>#</th><th>Dados</th></tr>`
			for _, rec := range found {
				b += `<tr><td>` + esc(rec.Col) + `</td><td>` + strconv.FormatInt(rec.ID, 10) + `</td><td><code>` + esc(jsonUnescape(rec.Data)) + `</code></td></tr>`
			}
			b += `</table></div><form method="post" onsubmit="return confirm('Apagar todos esses registros? Não dá para desfazer.')" style="margin-top:12px">` + a.tok() +
				`<input type="hidden" name="q" value="` + esc(q) + `"><input type="hidden" name="acao" value="apagar"><button class="ghost">Apagar todos</button></form>`
		}
	} else if q != "" {
		b += `<p class="err">Digite pelo menos 4 caracteres.</p>`
	}
	return a.out("LGPD: dados de um titular", b, true)
}
