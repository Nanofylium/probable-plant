package web

// E-mail de verdade, sem biblioteca de fora:
//
//   - <form save="contatos" avisar="contato@empresa.com.br"> manda um aviso a cada envio
//     (avisar sem valor, ou avisar="dono", usa o endereço configurado no painel);
//   - confirmar="Recebemos sua mensagem…" responde para o e-mail que a pessoa digitou;
//   - enviar_email({para:, assunto:, texto:, html:, responder_para:}) no código.
//
// Como sai: pelo servidor SMTP configurado no painel (ou nas variáveis NEP_SMTP_*);
// sem SMTP, pelo sendmail da hospedagem (cPanel/VPS); sem nenhum dos dois
// (no computador, com nep dev), o e-mail fica guardado em data/emails/ e aparece no painel.

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"mime"
	"mime/multipart"
	"mime/quotedprintable"
	"net"
	"net/mail"
	"net/smtp"
	"net/textproto"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/Nanofylium/probable-plant/lang"
)

var multiNL = regexp.MustCompile(`\n{3,}`)

func urlQ(s string) string { return url.QueryEscape(s) }

const (
	emailMeta    = "email"
	emailLogMeta = "email:log"
)

// emailConf é a configuração de envio guardada no painel (só o dono vê).
type emailConf struct {
	De      string `json:"de,omitempty"`      // remetente (ex.: contato@empresa.com.br)
	Nome    string `json:"nome,omitempty"`    // nome do remetente
	Avisos  string `json:"avisos,omitempty"`  // para onde vão os avisos de formulário
	Host    string `json:"host,omitempty"`    // servidor SMTP (vazio: usa o sendmail)
	Porta   int    `json:"porta,omitempty"`   // 587 (STARTTLS) ou 465 (TLS)
	Usuario string `json:"usuario,omitempty"` // login do SMTP
	Senha   string `json:"senha,omitempty"`
}

// emailConf lê o painel; as variáveis de ambiente NEP_SMTP_* (VPS) valem por cima.
func (s *Server) emailConf() emailConf {
	var c emailConf
	_ = json.Unmarshal([]byte(s.Store.Meta(emailMeta)), &c)
	for k, p := range map[string]*string{"NEP_SMTP_HOST": &c.Host, "NEP_SMTP_USUARIO": &c.Usuario, "NEP_SMTP_SENHA": &c.Senha, "NEP_EMAIL_DE": &c.De} {
		if v := os.Getenv(k); v != "" {
			*p = v
		}
	}
	if v, err := strconv.Atoi(os.Getenv("NEP_SMTP_PORTA")); err == nil && v > 0 {
		c.Porta = v
	}
	if c.Porta == 0 {
		c.Porta = 587
	}
	return c
}

func (s *Server) saveEmailConf(c emailConf) error {
	b, err := json.Marshal(c)
	if err != nil {
		return err
	}
	return s.Store.SetMeta(emailMeta, string(b))
}

// mailMsg é um e-mail a enviar.
type mailMsg struct {
	To      []string
	ReplyTo string
	Subject string
	Text    string
	HTML    string
}

// emailLog é uma linha do histórico de envios mostrado no painel.
type emailLog struct {
	Quando  string `json:"quando"`
	Para    string `json:"para"`
	Assunto string `json:"assunto"`
	Via     string `json:"via"`
	Erro    string `json:"erro,omitempty"`
}

func (s *Server) emailLogs() []emailLog {
	var l []emailLog
	_ = json.Unmarshal([]byte(s.Store.Meta(emailLogMeta)), &l)
	return l
}

func (s *Server) addEmailLog(e emailLog) {
	l := append([]emailLog{e}, s.emailLogs()...)
	if len(l) > 50 {
		l = l[:50]
	}
	if b, err := json.Marshal(l); err == nil {
		_ = s.Store.SetMeta(emailLogMeta, string(b))
	}
}

// Pontos trocáveis nos testes.
var (
	sendmailPaths = []string{"/usr/sbin/sendmail", "/usr/lib/sendmail", "/usr/bin/sendmail"}
	smtpTimeout   = 20 * time.Second
)

func sendmailPath() string {
	for _, p := range sendmailPaths {
		if st, err := os.Stat(p); err == nil && !st.IsDir() && st.Mode()&0o111 != 0 {
			return p
		}
	}
	return ""
}

var errNoHeader = errors.New("quebra de linha não é permitida em endereço ou assunto")

// cleanAddr confere um endereço de e-mail (e recusa truques de cabeçalho).
func cleanAddr(a string) (string, error) {
	a = strings.TrimSpace(a)
	if strings.ContainsAny(a, "\r\n") {
		return "", errNoHeader
	}
	p, err := mail.ParseAddress(a)
	if err != nil || !emailRe.MatchString(p.Address) {
		return "", fmt.Errorf("endereço de e-mail inválido: %q", a)
	}
	return p.Address, nil
}

// splitAddrs aceita "a@x.com, b@y.com" ou "a@x.com; b@y.com".
func splitAddrs(v string) []string {
	var out []string
	for _, p := range strings.FieldsFunc(v, func(r rune) bool { return r == ',' || r == ';' }) {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return out
}

func hostOnly(h string) string {
	if x, _, err := net.SplitHostPort(h); err == nil {
		h = x
	}
	h = strings.Trim(strings.ToLower(h), "[]")
	if h == "" || net.ParseIP(h) != nil || !strings.Contains(h, ".") {
		return "nepenthes.local" // computador local: o e-mail fica guardado ou sai pelo SMTP configurado
	}
	return strings.TrimPrefix(h, "www.")
}

// buildMail monta a mensagem MIME (texto e, se houver, HTML), em UTF-8.
func buildMail(from, fromName string, m mailMsg, domain string) ([]byte, error) {
	if strings.ContainsAny(m.Subject+fromName, "\r\n") {
		return nil, errNoHeader
	}
	var b bytes.Buffer
	h := func(k, v string) { b.WriteString(k + ": " + v + "\r\n") }
	fa := (&mail.Address{Name: fromName, Address: from}).String()
	h("From", fa)
	h("To", strings.Join(m.To, ", "))
	if m.ReplyTo != "" {
		h("Reply-To", m.ReplyTo)
	}
	h("Subject", mime.QEncoding.Encode("utf-8", m.Subject))
	h("Date", time.Now().Format(time.RFC1123Z))
	h("Message-ID", "<"+randHex(12)+"@"+domain+">")
	h("MIME-Version", "1.0")
	h("X-Mailer", "Nepenthes")
	text := m.Text
	if text == "" && m.HTML != "" {
		text = htmlToText(m.HTML)
	}
	qp := func(w *bytes.Buffer, s string) error {
		q := quotedprintable.NewWriter(w)
		if _, err := q.Write([]byte(strings.ReplaceAll(s, "\r\n", "\n"))); err != nil {
			return err
		}
		return q.Close()
	}
	if m.HTML == "" {
		h("Content-Type", "text/plain; charset=utf-8")
		h("Content-Transfer-Encoding", "quoted-printable")
		b.WriteString("\r\n")
		if err := qp(&b, text); err != nil {
			return nil, err
		}
		return b.Bytes(), nil
	}
	var body bytes.Buffer
	mw := multipart.NewWriter(&body)
	h("Content-Type", `multipart/alternative; boundary="`+mw.Boundary()+`"`)
	b.WriteString("\r\n")
	for _, part := range []struct{ typ, s string }{{"text/plain", text}, {"text/html", m.HTML}} {
		w, err := mw.CreatePart(textproto.MIMEHeader{
			"Content-Type":              {part.typ + "; charset=utf-8"},
			"Content-Transfer-Encoding": {"quoted-printable"},
		})
		if err != nil {
			return nil, err
		}
		var p bytes.Buffer
		if err := qp(&p, part.s); err != nil {
			return nil, err
		}
		if _, err := w.Write(p.Bytes()); err != nil {
			return nil, err
		}
	}
	if err := mw.Close(); err != nil {
		return nil, err
	}
	b.Write(body.Bytes())
	return b.Bytes(), nil
}

// htmlToText faz uma versão em texto simples para quem lê e-mail sem HTML.
func htmlToText(h string) string {
	h = brRe.ReplaceAllString(h, "\n")
	h = tagRe.ReplaceAllString(h, "")
	h = htmlUnescape(h)
	lines := strings.Split(h, "\n")
	for i := range lines {
		lines[i] = strings.TrimSpace(lines[i])
	}
	return strings.TrimSpace(multiNL.ReplaceAllString(strings.Join(lines, "\n"), "\n\n"))
}

// sendMail envia de verdade. host é o domínio do site (para o remetente padrão).
// Devolve por onde saiu: "smtp", "sendmail" ou "guardado" (sem servidor de e-mail).
func (s *Server) sendMail(m mailMsg, host string) (via string, err error) {
	c := s.emailConf()
	domain := hostOnly(host)
	logIt := func() {
		e := emailLog{Quando: stamp(), Para: strings.Join(m.To, ", "), Assunto: m.Subject, Via: via}
		if err != nil {
			e.Erro = err.Error()
		}
		s.addEmailLog(e)
		if s.opt.Log != nil && (s.opt.Dev || err != nil) {
			st := "ok"
			if err != nil {
				st = "ERRO: " + err.Error()
			}
			fmt.Fprintf(s.opt.Log, "e-mail para %s (%s) via %s: %s\n", e.Para, m.Subject, via, st)
		}
	}
	defer logIt()

	from := c.De
	if from == "" {
		from = "nao-responda@" + domain
	}
	if from, err = cleanAddr(from); err != nil {
		return "", fmt.Errorf("remetente: %w", err)
	}
	if len(m.To) == 0 {
		return "", errors.New("e-mail sem destinatário")
	}
	if len(m.To) > 20 {
		return "", errors.New("no máximo 20 destinatários por e-mail")
	}
	for i, t := range m.To {
		if m.To[i], err = cleanAddr(t); err != nil {
			return "", err
		}
	}
	if m.ReplyTo != "" {
		if m.ReplyTo, err = cleanAddr(m.ReplyTo); err != nil {
			return "", fmt.Errorf("responder para: %w", err)
		}
	}
	if strings.TrimSpace(m.Subject) == "" {
		m.Subject = "(sem assunto)"
	}
	if r := []rune(m.Subject); len(r) > 200 {
		m.Subject = string(r[:200])
	}
	if len(m.Text)+len(m.HTML) > 1<<20 {
		return "", errors.New("e-mail grande demais (máximo 1 MB de texto)")
	}
	msg, err := buildMail(from, c.Nome, m, domain)
	if err != nil {
		return "", err
	}
	switch {
	case c.Host != "":
		via = "smtp"
		err = smtpSend(c, from, m.To, msg)
	case sendmailPath() != "":
		via = "sendmail"
		err = sendmailSend(sendmailPath(), from, msg)
	default:
		via = "guardado"
		err = s.saveOutbox(msg)
	}
	return via, err
}

// saveOutbox guarda o e-mail em data/emails/ quando não há como enviar (computador local).
func (s *Server) saveOutbox(msg []byte) error {
	dir := filepath.Join(s.opt.DataDir, "emails")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	name := time.Now().Format("20060102-150405") + "-" + randHex(4) + ".eml"
	return os.WriteFile(filepath.Join(dir, name), msg, 0o600)
}

func sendmailSend(path, from string, msg []byte) error {
	ctx, cancel := context.WithTimeout(context.Background(), smtpTimeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, path, "-t", "-i", "-f", from)
	cmd.Stdin = bytes.NewReader(msg)
	out, err := cmd.CombinedOutput()
	if err != nil {
		o := strings.TrimSpace(string(out))
		if len(o) > 300 {
			o = o[:300]
		}
		return fmt.Errorf("sendmail falhou: %v %s", err, o)
	}
	return nil
}

func smtpSend(c emailConf, from string, to []string, msg []byte) error {
	addr := net.JoinHostPort(c.Host, strconv.Itoa(c.Porta))
	d := &net.Dialer{Timeout: smtpTimeout}
	var conn net.Conn
	var err error
	if c.Porta == 465 {
		conn, err = tls.DialWithDialer(d, "tcp", addr, &tls.Config{ServerName: c.Host})
	} else {
		conn, err = d.Dial("tcp", addr)
	}
	if err != nil {
		return fmt.Errorf("não consegui conectar em %s: %v", addr, err)
	}
	_ = conn.SetDeadline(time.Now().Add(2 * smtpTimeout))
	cl, err := smtp.NewClient(conn, c.Host)
	if err != nil {
		conn.Close()
		return fmt.Errorf("o servidor %s não respondeu como SMTP: %v", addr, err)
	}
	defer cl.Close()
	if c.Porta != 465 {
		if ok, _ := cl.Extension("STARTTLS"); ok {
			if err := cl.StartTLS(&tls.Config{ServerName: c.Host}); err != nil {
				return fmt.Errorf("falha ao proteger a conexão (STARTTLS): %v", err)
			}
		}
	}
	if c.Usuario != "" {
		if ok, _ := cl.Extension("AUTH"); !ok {
			return errors.New("o servidor SMTP não aceita login nesta porta (tente 587 ou 465)")
		}
		if err := cl.Auth(smtp.PlainAuth("", c.Usuario, c.Senha, c.Host)); err != nil {
			return fmt.Errorf("usuário ou senha do SMTP recusados: %v", err)
		}
	}
	if err := cl.Mail(from); err != nil {
		return fmt.Errorf("o servidor recusou o remetente %s: %v", from, err)
	}
	for _, t := range to {
		if err := cl.Rcpt(t); err != nil {
			return fmt.Errorf("o servidor recusou o destinatário %s: %v", t, err)
		}
	}
	w, err := cl.Data()
	if err != nil {
		return err
	}
	if _, err := w.Write(msg); err != nil {
		return err
	}
	if err := w.Close(); err != nil {
		return fmt.Errorf("o servidor não aceitou a mensagem: %v", err)
	}
	return cl.Quit()
}

// ---------- formulários: avisar= e confirmar= ----------

// formEmails manda o aviso para o dono e a confirmação para quem enviou.
// Falhas não desfazem o envio do formulário: ficam no histórico do painel.
func (s *Server) formEmails(rq *request, f *Form, data *lang.Table) {
	host := rq.r.Host
	if f.Notify != "" {
		to := f.Notify
		if to == "dono" || to == "owner" || to == "painel" {
			to = s.emailConf().Avisos
		}
		if to == "" {
			s.addEmailLog(emailLog{Quando: stamp(), Assunto: "Aviso do formulário", Via: "-", Erro: "avisar=\"dono\", mas nenhum endereço de avisos foi configurado no painel (E-mail)"})
		} else if ok, _ := s.Store.Rate("email:aviso", 300, 3600, true); ok {
			_, _ = s.sendMail(s.noticeMail(rq, f, data, splitAddrs(to)), host)
		}
	}
	if f.Confirm != "" {
		addr := submitterEmail(f, data)
		if addr == "" {
			return
		}
		// texto fixo do autor do site: o formulário não vira um jeito de mandar spam
		if ok, _ := s.Store.Rate("email:confirma", 60, 3600, true); ok {
			subj := f.ConfirmSubject
			if subj == "" {
				subj = "Recebemos seu envio"
				if n := s.emailConf().Nome; n != "" {
					subj += " · " + n
				}
			}
			_, _ = s.sendMail(mailMsg{To: []string{addr}, Subject: subj, Text: f.Confirm, ReplyTo: s.replyToOwner()}, host)
		}
	}
}

func (s *Server) replyToOwner() string {
	if a := splitAddrs(s.emailConf().Avisos); len(a) > 0 {
		if v, err := cleanAddr(a[0]); err == nil {
			return v
		}
	}
	return ""
}

// submitterEmail acha o e-mail que a pessoa digitou (o primeiro campo type=email preenchido).
func submitterEmail(f *Form, data *lang.Table) string {
	for _, fd := range f.Fields {
		if fd.Type != "email" {
			continue
		}
		if v, ok := data.Get(fd.Name); ok && v != nil {
			if a, err := cleanAddr(lang.Str(v)); err == nil {
				return a
			}
		}
	}
	return ""
}

// noticeMail monta o aviso: todos os campos, com resposta direto para quem enviou.
func (s *Server) noticeMail(rq *request, f *Form, data *lang.Table, to []string) mailMsg {
	what := f.Save
	if what == "" {
		what = "formulário"
	}
	page := ""
	if rq.page != nil {
		page = rq.page.Title
	}
	subj := "Novo envio: " + what
	if page != "" {
		subj += " (" + page + ")"
	}
	var t, h strings.Builder
	h.WriteString(`<div style="font-family:system-ui,sans-serif;font-size:15px;color:#1c1b22"><p>Chegou um novo envio pelo site` + map[bool]string{true: ", na página <b>" + esc(page) + "</b>", false: ""}[page != ""] + `.</p><table cellpadding="6" style="border-collapse:collapse">`)
	t.WriteString("Chegou um novo envio pelo site.\n\n")
	for _, fd := range f.Fields {
		v, ok := data.Get(fd.Name)
		if !ok || v == nil {
			continue
		}
		label := fd.Label
		if label == "" {
			label = fd.Name
		}
		val := fieldText(v)
		t.WriteString(label + ": " + val + "\n")
		h.WriteString(`<tr><th align="left" valign="top" style="border-bottom:1px solid #eee">` + esc(label) + `</th><td style="border-bottom:1px solid #eee">` + strings.ReplaceAll(esc(val), "\n", "<br>") + `</td></tr>`)
	}
	h.WriteString(`</table>`)
	if f.Save != "" {
		link := s.siteURL(rq) + "/_admin?c=" + f.Save
		t.WriteString("\nVer no painel: " + link + "\n")
		h.WriteString(`<p><a href="` + esc(link) + `">Ver no painel</a></p>`)
	}
	h.WriteString(`<p style="color:#888;font-size:12px">Responda este e-mail para falar direto com quem enviou.</p></div>`)
	return mailMsg{To: to, Subject: subj, Text: t.String(), HTML: h.String(), ReplyTo: submitterEmail(f, data)}
}

// fieldText mostra o valor de um campo no e-mail (listas, sim/não, arquivos).
func fieldText(v lang.Value) string {
	switch x := v.(type) {
	case bool:
		if x {
			return "sim"
		}
		return "não"
	case *lang.Table:
		if n, ok := x.Get("nome"); ok {
			return "arquivo " + lang.Str(n)
		}
		var parts []string
		for i := 0; ; i++ {
			it, ok := x.Get(int64(i))
			if !ok {
				break
			}
			parts = append(parts, fieldText(it))
		}
		if len(parts) > 0 {
			return strings.Join(parts, ", ")
		}
	}
	return lang.Str(v)
}

// ---------- no código: enviar_email(...) ----------

// emailBuiltin: enviar_email({para:, assunto:, texto:, html:, responder_para:})
// ou enviar_email(para, assunto, texto). Devolve true se saiu (ou ficou guardado).
func (s *Server) emailBuiltin(rq *request) func(a []lang.Value) (lang.Value, error) {
	return func(a []lang.Value) (lang.Value, error) {
		var m mailMsg
		get := func(t *lang.Table, keys ...string) string {
			for _, k := range keys {
				if v, ok := t.Get(k); ok && v != nil {
					return lang.Str(v)
				}
			}
			return ""
		}
		var para lang.Value
		if t, ok := argv(a, 0).(*lang.Table); ok && len(a) == 1 {
			para, _ = t.Get("para")
			if para == nil {
				para, _ = t.Get("to")
			}
			m.Subject = get(t, "assunto", "subject")
			m.Text = get(t, "texto", "text")
			m.HTML = get(t, "html")
			m.ReplyTo = get(t, "responder_para", "reply_to")
		} else {
			para = argv(a, 0)
			m.Subject = lang.Str(argv(a, 1))
			if argv(a, 2) != nil {
				m.Text = lang.Str(argv(a, 2))
			}
		}
		switch p := para.(type) {
		case string:
			m.To = splitAddrs(p)
		case *lang.Table:
			for i := 0; ; i++ {
				it, ok := p.Get(int64(i))
				if !ok {
					break
				}
				m.To = append(m.To, lang.Str(it))
			}
		}
		if ok, _ := s.Store.Rate("email:codigo", 200, 3600, true); !ok {
			s.addEmailLog(emailLog{Quando: stamp(), Para: strings.Join(m.To, ", "), Assunto: m.Subject, Via: "-", Erro: "limite de 200 e-mails por hora atingido"})
			return false, nil
		}
		_, err := s.sendMail(m, rq.r.Host)
		return err == nil, nil
	}
}

// ---------- painel: /_admin/email ----------

func (a *adminPage) emailPage(post bool) error {
	r := a.rq.r
	c := a.s.emailConf()
	msg, ok := "", r.URL.Query().Get("ok")
	if post {
		switch r.PostForm.Get("acao") {
		case "salvar":
			n := emailConf{
				De:      strings.TrimSpace(r.PostForm.Get("de")),
				Nome:    strings.TrimSpace(r.PostForm.Get("nome")),
				Avisos:  strings.TrimSpace(r.PostForm.Get("avisos")),
				Host:    strings.TrimSpace(r.PostForm.Get("host")),
				Usuario: strings.TrimSpace(r.PostForm.Get("usuario")),
				Senha:   r.PostForm.Get("senha"),
			}
			n.Porta, _ = strconv.Atoi(r.PostForm.Get("porta"))
			if n.Senha == "" { // campo de senha vazio: mantém a senha guardada
				var old emailConf
				_ = json.Unmarshal([]byte(a.s.Store.Meta(emailMeta)), &old)
				n.Senha = old.Senha
			}
			if r.PostForm.Get("apagar_senha") == "1" {
				n.Senha = ""
			}
			switch {
			case strings.ContainsAny(n.Nome+n.Host+n.Usuario, "\r\n<>"):
				msg = "Use só texto simples nos campos."
			case n.De != "" && func() bool { _, err := cleanAddr(n.De); return err != nil }():
				msg = "O remetente precisa ser um e-mail válido."
			case n.Porta != 0 && (n.Porta < 1 || n.Porta > 65535):
				msg = "Porta inválida."
			}
			if msg == "" {
				for _, x := range splitAddrs(n.Avisos) {
					if _, err := cleanAddr(x); err != nil {
						msg = "Endereço de avisos inválido: " + x
					}
				}
			}
			if msg == "" {
				if err := a.s.saveEmailConf(n); err != nil {
					return err
				}
				a.s.redirect(a.rq, a.base+"/email?ok="+urlQ("Configuração salva."), 303)
				return nil
			}
		case "teste":
			to := strings.TrimSpace(r.PostForm.Get("para"))
			via, err := a.s.sendMail(mailMsg{To: []string{to}, Subject: "Teste de e-mail do site",
				Text: "Se você recebeu esta mensagem, o envio de e-mail do site está funcionando.\n\nEnviado pelo painel do Nepenthes."}, r.Host)
			if err != nil {
				msg = "Não foi: " + err.Error()
			} else {
				t := map[string]string{"smtp": "Enviado pelo servidor SMTP. Confira a caixa de entrada (e o spam).",
					"sendmail": "Enviado pelo e-mail da hospedagem. Confira a caixa de entrada (e o spam).",
					"guardado": "Não há servidor de e-mail aqui, então o e-mail ficou guardado em data/emails/. Na hospedagem ele sai de verdade."}[via]
				a.s.redirect(a.rq, a.base+"/email?ok="+urlQ(t), 303)
				return nil
			}
		}
	}
	way := "pelo e-mail da hospedagem (sendmail)"
	switch {
	case c.Host != "":
		way = "pelo servidor SMTP " + c.Host
	case sendmailPath() == "":
		way = "<b>em lugar nenhum ainda</b>: não há SMTP configurado nem sendmail neste computador, então os e-mails ficam guardados em data/emails/"
	}
	var b strings.Builder
	b.WriteString(cmsCSS + errP(msg))
	if ok != "" {
		b.WriteString(`<p class="ok">` + esc(ok) + `</p>`)
	}
	b.WriteString(`<p class="muted">Os e-mails saem ` + way + `. Na maioria das hospedagens (cPanel) funciona sem configurar nada; para cair menos no spam, use como remetente um e-mail do seu domínio ou configure o SMTP dele.</p>`)
	def := "nao-responda@" + hostOnly(r.Host)
	senhaPH := "Senha do e-mail"
	if c.Senha != "" {
		senhaPH = "•••••• (guardada; deixe vazio para manter)"
	}
	b.WriteString(`<form class="cms" method="post" style="max-width:560px">` + a.tok() + `<input type="hidden" name="acao" value="salvar">` +
		`<label>Avisos de formulário vão para (avisar="dono") <input name="avisos" value="` + esc(c.Avisos) + `" placeholder="contato@seusite.com.br, outro@gmail.com"></label>` +
		`<label>Remetente <input name="de" type="email" value="` + esc(c.De) + `" placeholder="` + esc(def) + `"></label>` +
		`<label>Nome do remetente <input name="nome" maxlength="60" value="` + esc(c.Nome) + `" placeholder="Nome do site"></label>` +
		`<details` + map[bool]string{true: " open", false: ""}[c.Host != ""] + `><summary>Servidor SMTP (opcional)</summary>` +
		`<label>Servidor <input name="host" value="` + esc(c.Host) + `" placeholder="mail.seusite.com.br ou smtp.gmail.com"></label>` +
		`<label>Porta <select name="porta">` + portOpt(c.Porta, 587, "587 (STARTTLS)") + portOpt(c.Porta, 465, "465 (SSL/TLS)") + portOpt(c.Porta, 25, "25") + `</select></label>` +
		`<label>Usuário <input name="usuario" value="` + esc(c.Usuario) + `" autocomplete="off"></label>` +
		`<label>Senha <input name="senha" type="password" autocomplete="new-password" placeholder="` + esc(senhaPH) + `"></label>` +
		map[bool]string{true: `<label><input type="checkbox" name="apagar_senha" value="1" style="width:auto"> apagar a senha guardada</label>`, false: ""}[c.Senha != ""] +
		`</details><button>Salvar</button></form>`)
	b.WriteString(`<h2 style="font-size:1.1rem;margin-top:24px">Enviar um teste</h2><form class="row" method="post">` + a.tok() + `<input type="hidden" name="acao" value="teste">` +
		`<input name="para" type="email" required placeholder="seu@email.com" value="` + esc(firstAddr(c.Avisos)) + `"><button>Enviar teste</button></form>`)
	if l := a.s.emailLogs(); len(l) > 0 {
		b.WriteString(`<h2 style="font-size:1.1rem;margin-top:24px">Últimos envios</h2><table><tr><th>Quando</th><th>Para</th><th>Assunto</th><th>Situação</th></tr>`)
		for _, e := range l {
			st := map[string]string{"smtp": "enviado (SMTP)", "sendmail": "enviado (hospedagem)", "guardado": "guardado em data/emails"}[e.Via]
			if e.Erro != "" {
				st = `<span style="color:#b3261e">erro: ` + esc(e.Erro) + `</span>`
			}
			b.WriteString(`<tr><td>` + esc(e.Quando) + `</td><td>` + esc(e.Para) + `</td><td>` + esc(e.Assunto) + `</td><td>` + st + `</td></tr>`)
		}
		b.WriteString(`</table>`)
	}
	return a.out("E-mail", b.String(), true)
}

func portOpt(cur, v int, label string) string {
	sel := ""
	if cur == v {
		sel = " selected"
	}
	return `<option value="` + strconv.Itoa(v) + `"` + sel + `>` + label + `</option>`
}

func firstAddr(v string) string {
	if a := splitAddrs(v); len(a) > 0 {
		return a[0]
	}
	return ""
}
