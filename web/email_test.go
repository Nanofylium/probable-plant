package web

import (
	"bufio"
	"io"
	"mime"
	"net"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"net/mail"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"
)

// fakeSMTP é um servidor SMTP mínimo que guarda o que recebe.
type fakeSMTP struct {
	ln   net.Listener
	mu   sync.Mutex
	msgs []smtpMsg
	auth []string
}

type smtpMsg struct {
	from string
	to   []string
	data string
}

func newFakeSMTP(t *testing.T) *fakeSMTP {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	f := &fakeSMTP{ln: ln}
	t.Cleanup(func() { ln.Close() })
	go func() {
		for {
			c, err := ln.Accept()
			if err != nil {
				return
			}
			go f.serve(c)
		}
	}()
	return f
}

func (f *fakeSMTP) port() int { return f.ln.Addr().(*net.TCPAddr).Port }

func (f *fakeSMTP) serve(c net.Conn) {
	defer c.Close()
	r := bufio.NewReader(c)
	say := func(s string) { io.WriteString(c, s+"\r\n") }
	say("220 teste ESMTP")
	var cur smtpMsg
	for {
		line, err := r.ReadString('\n')
		if err != nil {
			return
		}
		line = strings.TrimRight(line, "\r\n")
		up := strings.ToUpper(line)
		switch {
		case strings.HasPrefix(up, "EHLO"):
			say("250-teste")
			say("250 AUTH PLAIN")
		case strings.HasPrefix(up, "AUTH"):
			f.mu.Lock()
			f.auth = append(f.auth, line)
			f.mu.Unlock()
			say("235 ok")
		case strings.HasPrefix(up, "MAIL FROM:"):
			cur = smtpMsg{from: strings.Trim(line[10:], "<> ")}
			say("250 ok")
		case strings.HasPrefix(up, "RCPT TO:"):
			to := strings.Trim(line[8:], "<> ")
			if strings.HasSuffix(to, "@recusa.com") {
				say("550 nao existe")
				continue
			}
			cur.to = append(cur.to, to)
			say("250 ok")
		case up == "DATA":
			say("354 manda")
			var b strings.Builder
			for {
				l, err := r.ReadString('\n')
				if err != nil {
					return
				}
				if l == ".\r\n" {
					break
				}
				b.WriteString(l)
			}
			cur.data = b.String()
			f.mu.Lock()
			f.msgs = append(f.msgs, cur)
			f.mu.Unlock()
			say("250 guardado")
		case up == "QUIT":
			say("221 tchau")
			return
		default:
			say("250 ok")
		}
	}
}

func (f *fakeSMTP) all() []smtpMsg {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]smtpMsg(nil), f.msgs...)
}

const siteEmail = `<!doctype html><html><head><title>t</title></head><body>
<nep>
fn manda() do
  return enviar_email({para: "cliente@ex.com", assunto: "Olá, açaí", texto: "linha 1\nlinha 2", html: "<p>Oi <b>você</b></p>"})
end
</nep>
<page route="/" title="Contato">
  <form save="pedidos" avisar="dono@empresa.com.br" confirmar="Recebemos seu pedido. Respondemos em até 1 dia." confirmar-assunto="Pedido recebido">
    <label>Nome <input name="nome" required></label>
    <label>E-mail <input name="email" type="email" required></label>
    <label>Mensagem <textarea name="msg"></textarea></label>
    <button>Enviar</button>
  </form>
</page>
<page route="/codigo" title="C">{{ manda() }}</page>
<page route="/ruim" title="R">{{ enviar_email("a@ex.com\r\nBcc: b@ex.com", "x", "y") }}</page>
<page route="/dono" title="D"><form save="x" avisar><input name="email" type="email"><button>ok</button></form></page>
</body></html>`

func newTCSite(t *testing.T, src string) (*tc, *Server) {
	dir := t.TempDir()
	file := filepath.Join(dir, "site.nep.html")
	os.WriteFile(file, []byte(src), 0o644)
	s, err := New(Options{SiteFile: file, Log: io.Discard})
	if err != nil {
		t.Fatal(err)
	}
	ts := httptest.NewServer(s)
	t.Cleanup(ts.Close)
	jar, _ := cookiejar.New(nil)
	return &tc{t: t, srv: ts, dir: dir, c: &http.Client{Jar: jar, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}}, s
}

func noSendmail(t *testing.T) {
	old := sendmailPaths
	sendmailPaths = []string{filepath.Join(t.TempDir(), "nao-existe")}
	t.Cleanup(func() { sendmailPaths = old })
}

func TestEmailFormularioPorSMTP(t *testing.T) {
	noSendmail(t)
	smtpd := newFakeSMTP(t)
	c, srv := newTCSite(t, siteEmail)
	if err := srv.saveEmailConf(emailConf{De: "site@empresa.com.br", Nome: "Empresa Ltda", Host: "127.0.0.1", Porta: smtpd.port(), Usuario: "u", Senha: "s"}); err != nil {
		t.Fatal(err)
	}
	_, body, _ := c.get("/")
	if strings.Contains(body, "avisar") || strings.Contains(body, "confirmar") || strings.Contains(body, "dono@") {
		t.Fatalf("atributos de e-mail vazaram no HTML: %s", body)
	}
	tok := csrfRe.FindStringSubmatch(body)[1]
	code, _, _ := c.post("/", url.Values{"_nep_form": {"0"}, "_nep_csrf": {tok}, "nome": {"Zé Ação"}, "email": {"ze@cliente.com"}, "msg": {"Quero 2 vagas\npara janeiro"}})
	if code != 303 {
		t.Fatalf("envio: %d", code)
	}
	msgs := smtpd.all()
	if len(msgs) != 2 {
		t.Fatalf("esperava aviso + confirmação, chegaram %d", len(msgs))
	}
	if len(smtpd.auth) == 0 {
		t.Error("não fez login no SMTP")
	}
	aviso, conf := parseMail(t, msgs[0].data), parseMail(t, msgs[1].data)
	if msgs[0].from != "site@empresa.com.br" || msgs[0].to[0] != "dono@empresa.com.br" {
		t.Errorf("envelope do aviso: %+v", msgs[0])
	}
	if aviso.Header.Get("Reply-To") != "ze@cliente.com" {
		t.Errorf("aviso deveria responder para quem enviou: %q", aviso.Header.Get("Reply-To"))
	}
	if subj := decodeH(aviso.Header.Get("Subject")); subj != "Novo envio: pedidos (Contato)" {
		t.Errorf("assunto do aviso: %q", subj)
	}
	if from := decodeH(aviso.Header.Get("From")); !strings.Contains(from, "Empresa Ltda") {
		t.Errorf("remetente: %q", from)
	}
	raw := msgs[0].data
	must(t, raw, "Z=C3=A9 A=C3=A7=C3=A3o", "Quero 2 vagas", "/_admin?c=3Dpedidos", "multipart/alternative")
	if msgs[1].to[0] != "ze@cliente.com" || decodeH(conf.Header.Get("Subject")) != "Pedido recebido" {
		t.Errorf("confirmação: %+v %q", msgs[1].to, conf.Header.Get("Subject"))
	}
	if strings.Contains(msgs[1].data, "Quero 2 vagas") {
		t.Error("a confirmação não deve repetir o texto digitado (evita spam pelo formulário)")
	}

	// no código
	_, body, _ = c.get("/codigo")
	if !strings.Contains(body, "true") {
		t.Fatalf("enviar_email devolveu: %s", body)
	}
	m := smtpd.all()[2]
	if m.to[0] != "cliente@ex.com" || decodeH(parseMail(t, m.data).Header.Get("Subject")) != "Olá, açaí" {
		t.Errorf("e-mail do código: %+v", m)
	}
	must(t, m.data, "text/plain", "linha 2", "text/html", "<b>voc=C3=AA</b>")

	// truque de cabeçalho é recusado
	_, body, _ = c.get("/ruim")
	if !strings.Contains(body, "false") || len(smtpd.all()) != 3 {
		t.Errorf("injeção de cabeçalho passou: %s", body)
	}

	// histórico e página do painel
	tok = loginAdmin(c)
	_, body, _ = c.get("/_admin/email")
	must(t, body, "Últimos envios", "enviado (SMTP)", "dono@empresa.com.br", "quebra de linha", "guardada; deixe vazio")
	if strings.Contains(body, `value="s"`) {
		t.Error("a senha do SMTP apareceu no painel")
	}
}

func decodeH(s string) string {
	d, err := new(mime.WordDecoder).DecodeHeader(s)
	if err != nil {
		return s
	}
	return d
}

func parseMail(t *testing.T, raw string) *mail.Message {
	m, err := mail.ReadMessage(strings.NewReader(raw))
	if err != nil {
		t.Fatalf("e-mail malformado: %v\n%s", err, raw)
	}
	return m
}

func TestEmailPeloSendmail(t *testing.T) {
	dir := t.TempDir()
	out := filepath.Join(dir, "saida.txt")
	script := filepath.Join(dir, "sendmail")
	os.WriteFile(script, []byte("#!/bin/sh\necho \"$@\" > "+out+".args\ncat > "+out+"\n"), 0o755)
	old := sendmailPaths
	sendmailPaths = []string{script}
	t.Cleanup(func() { sendmailPaths = old })

	c, _ := newTCSite(t, siteEmail)
	if _, body, _ := c.get("/codigo"); !strings.Contains(body, "true") {
		t.Fatalf("sendmail: %s", body)
	}
	b, _ := os.ReadFile(out)
	args, _ := os.ReadFile(out + ".args")
	must(t, string(b), "To: cliente@ex.com", "Subject: =?utf-8?q?Ol=C3=A1,_a=C3=A7a=C3=AD?=")
	if !strings.Contains(string(args), "-t -i -f nao-responda@") {
		t.Errorf("argumentos do sendmail: %s", args)
	}
}

func TestEmailSemServidorFicaGuardado(t *testing.T) {
	noSendmail(t)
	c, srv := newTCSite(t, siteEmail)
	_, body, _ := c.get("/codigo")
	if !strings.Contains(body, "true") {
		t.Fatal(body)
	}
	files, _ := filepath.Glob(filepath.Join(c.dir, "data", "emails", "*.eml"))
	if len(files) != 1 {
		t.Fatalf("esperava 1 e-mail guardado, achei %d", len(files))
	}
	if l := srv.emailLogs(); len(l) != 1 || l[0].Via != "guardado" {
		t.Errorf("histórico: %+v", l)
	}

	// avisar sem valor usa o endereço do painel; sem endereço, registra o motivo
	tok := c.csrf("/dono")
	c.post("/dono", url.Values{"_nep_form": {"1"}, "_nep_csrf": {tok}, "email": {"a@b.com"}})
	if l := srv.emailLogs(); !strings.Contains(l[0].Erro, "nenhum endereço de avisos") {
		t.Errorf("avisar sem endereço: %+v", l[0])
	}
	srv.saveEmailConf(emailConf{Avisos: "dono@x.com.br"})
	c.post("/dono", url.Values{"_nep_form": {"1"}, "_nep_csrf": {tok}, "email": {"a@b.com"}})
	if l := srv.emailLogs(); l[0].Para != "dono@x.com.br" || l[0].Erro != "" {
		t.Errorf("avisar=dono: %+v", l[0])
	}
}

func TestEmailPainel(t *testing.T) {
	noSendmail(t)
	smtpd := newFakeSMTP(t)
	c, srv := newTCSite(t, siteEmail)
	tok := loginAdmin(c)
	_, body, _ := c.get("/_admin/email")
	tok = csrfRe.FindStringSubmatch(body)[1]
	must(t, body, "em lugar nenhum ainda")
	v := url.Values{"_nep_csrf": {tok}, "acao": {"salvar"}, "de": {"contato@loja.com.br"}, "nome": {"Loja"}, "avisos": {"dono@loja.com.br"},
		"host": {"127.0.0.1"}, "porta": {strconv.Itoa(smtpd.port())}, "usuario": {"u"}, "senha": {"segredo"}}
	if code, _, _ := c.post("/_admin/email", v); code != 303 {
		t.Fatalf("salvar: %d", code)
	}
	v.Set("senha", "") // vazio mantém a senha
	c.post("/_admin/email", v)
	if srv.emailConf().Senha != "segredo" {
		t.Error("salvar sem senha apagou a senha guardada")
	}
	v.Set("avisos", "isso não é email")
	if _, body, _ := c.post("/_admin/email", v); !strings.Contains(body, "Endereço de avisos inválido") {
		t.Error("deveria recusar endereço inválido")
	}
	if code, _, h := c.post("/_admin/email", url.Values{"_nep_csrf": {tok}, "acao": {"teste"}, "para": {"eu@ex.com"}}); code != 303 || !strings.Contains(h.Get("Location"), "SMTP") {
		t.Fatalf("teste: %d %s", code, h.Get("Location"))
	}
	if m := smtpd.all(); len(m) != 1 || m[0].to[0] != "eu@ex.com" {
		t.Fatalf("teste não chegou: %+v", m)
	}
	if _, body, _ := c.post("/_admin/email", url.Values{"_nep_csrf": {tok}, "acao": {"teste"}, "para": {"x@recusa.com"}}); !strings.Contains(body, "recusou o destinatário") {
		t.Error("erro do servidor deveria aparecer")
	}
}

func TestEmailAvisarInvalido(t *testing.T) {
	_, err := parseSiteForTest(`<page route="/"><form save="x" avisar="não é email"><input name="a"></form></page>`)
	if err == nil || !strings.Contains(err.Error(), "não é um e-mail válido") {
		t.Fatalf("erro esperado, veio %v", err)
	}
}

func parseSiteForTest(src string) (*Site, error) {
	dir, _ := os.MkdirTemp("", "nep")
	defer os.RemoveAll(dir)
	f := filepath.Join(dir, "site.nep.html")
	os.WriteFile(f, []byte(src), 0o644)
	return Check(f)
}
