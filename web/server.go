// Package web transforma um arquivo .nep.html num site completo, servido pelo
// próprio executável nep: páginas, rotas, banco de dados, formulários,
// painel administrativo, APIs JSON e regiões que se atualizam sozinhas.
package web

import (
	_ "embed"
	"errors"
	"fmt"
	"html"
	"io"
	"mime"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/Nanofylium/probable-plant/lang"
)

//go:embed assets/movimento.js
var movimentoJS []byte

//go:embed assets/vivo.js
var vivoJS []byte

//go:embed assets/editar.js
var editarJS []byte

// Options configura o servidor.
type Options struct {
	SiteFile string // o .nep.html
	DataDir  string // padrão: pasta "data" ao lado do site
	Dev      bool   // mostra erros detalhados e relê o arquivo a cada mudança
	CGI      bool   // rodando como CGI (hospedagem compartilhada)
	Base     string // prefixo de URL quando o site fica numa subpasta
	Log      io.Writer
}

type Server struct {
	opt     Options
	dir     string
	file    string
	mu      sync.Mutex
	site    *Site
	siteErr error
	mtime   time.Time
	Store   *Store
	secret  string
}

// New prepara o servidor. O site é lido agora e relido quando o arquivo muda.
func New(opt Options) (*Server, error) {
	abs, err := filepath.Abs(opt.SiteFile)
	if err != nil {
		return nil, err
	}
	if _, err := os.Stat(abs); err != nil {
		return nil, fmt.Errorf("arquivo do site não encontrado: %s", opt.SiteFile)
	}
	s := &Server{opt: opt, file: abs, dir: filepath.Dir(abs)}
	if opt.DataDir == "" {
		opt.DataDir = filepath.Join(s.dir, "data")
		s.opt.DataDir = opt.DataDir
	}
	if s.opt.Log == nil {
		s.opt.Log = os.Stderr
	}
	s.opt.Base = strings.TrimRight(opt.Base, "/")
	if s.Store, err = OpenStore(opt.DataDir); err != nil {
		return nil, err
	}
	if s.secret, err = s.Store.MetaOrInit("secret", func() string { return randHex(32) }); err != nil {
		return nil, err
	}
	if _, err := s.load(); err != nil {
		return s, err // o servidor sobe e mostra o erro na página
	}
	return s, nil
}

// load lê o .nep.html (de novo, se mudou).
func (s *Server) load() (*Site, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	st, err := os.Stat(s.file)
	if err != nil {
		return nil, err
	}
	if s.site != nil || s.siteErr != nil {
		if st.ModTime().Equal(s.mtime) {
			return s.site, s.siteErr
		}
	}
	src, err := os.ReadFile(s.file)
	if err != nil {
		return nil, err
	}
	s.mtime = st.ModTime()
	s.site, s.siteErr = ParseSite(filepath.Base(s.file), strings.ReplaceAll(string(src), "\r\n", "\n"))
	return s.site, s.siteErr
}

// Check lê o site e compila todo o código, sem rodar nada. Usado por `nep check`.
func Check(file string) (*Site, error) {
	src, err := os.ReadFile(file)
	if err != nil {
		return nil, err
	}
	site, err := ParseSite(filepath.Base(file), strings.ReplaceAll(string(src), "\r\n", "\n"))
	if err != nil {
		return nil, err
	}
	compile := func(code string) error {
		prog, err := lang.Parse(code, site.File)
		if err != nil {
			return err
		}
		_, err = lang.Compile(prog, lang.NewModule("site", file), site.File)
		return err
	}
	for i := -1; i < len(site.Pages); i++ {
		pre, body := site.Sources(i)
		if err := compile(pre); err != nil {
			return site, err
		}
		if err := compile(body); err != nil {
			return site, err
		}
	}
	for _, a := range site.APIs {
		if err := compile(site.APISource(a)); err != nil {
			return site, err
		}
	}
	for _, d := range site.Data {
		if _, err := lang.FromJSON(d.JSON); err != nil {
			return site, &TemplateError{File: site.File, Line: d.Line, Msg: fmt.Sprintf("JSON inválido em <data name=%q>: %v", d.Name, err)}
		}
	}
	return site, nil
}

// ---------- pedido ----------

type halt struct {
	kind   string // not_found, redirect, abort, reject
	to     string
	status int
	msg    string
}

func (h halt) String() string { return h.kind }

type request struct {
	s       *Server
	site    *Site
	w       http.ResponseWriter
	r       *http.Request
	path    string
	sess    *Session
	https   bool
	vm      *lang.VM
	module  *lang.Module
	out     *strings.Builder
	outs    []*strings.Builder
	caps    map[int]string
	forms   []int // pilha de formulários abertos
	split   []int
	formSt  map[int]*formState
	formN   map[int]int
	liveAt  map[int][2]int
	lives   bool
	page    *Page
	sessVal lang.Value
	sessRaw string
	item    *lang.Table // <page conteudo=...>: o item mostrado
	itemCol string
}

type formState struct {
	inst   int // qual cópia do formulário (dentro de <each>) foi enviada
	errors []string
	old    map[string][]string
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	rq := &request{s: s, w: w, r: r}
	rq.https = r.TLS != nil || strings.EqualFold(r.Header.Get("X-Forwarded-Proto"), "https") || os.Getenv("HTTPS") == "on"
	defer func() {
		if v := recover(); v != nil {
			s.fail(rq, fmt.Errorf("erro interno: %v", v))
		}
	}()

	p := r.URL.Path
	if s.opt.Base != "" && strings.HasPrefix(p, s.opt.Base) {
		p = p[len(s.opt.Base):]
	}
	rq.path = "/" + strings.Trim(p, "/")
	s.headers(w, rq)

	switch rq.path {
	case "/_nep/movimento.js":
		asset(w, r, movimentoJS)
		return
	case "/_nep/vivo.js":
		asset(w, r, vivoJS)
		return
	case "/_nep/editar.js":
		asset(w, r, editarJS)
		return
	case "/_nep/tema.css":
		serveTema(w, r)
		return
	case "/_nep/versao":
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.Header().Set("Cache-Control", "no-store")
		io.WriteString(w, s.Store.Version())
		return
	case "/_nep/eventos":
		s.events(w, r)
		return
	}
	if strings.HasPrefix(rq.path, "/_nep/construtor/") {
		serveConstrutor(w, r, strings.TrimPrefix(rq.path, "/_nep/construtor/"))
		return
	}
	if strings.HasPrefix(rq.path, "/_nep/arquivo/") {
		s.serveFile(w, r, strings.TrimPrefix(rq.path, "/_nep/arquivo/"))
		return
	}
	if s.static(w, r, rq.path) {
		return
	}
	rq.sess = loadSession(r, s.secret)
	s.checkUser(rq.sess)

	if rq.path == "/_admin" || strings.HasPrefix(rq.path, "/_admin/") {
		if err := s.admin(rq); err != nil {
			s.fail(rq, err)
		}
		return
	}

	site, err := s.load()
	if err != nil {
		s.fail(rq, err)
		return
	}
	rq.site = site
	if rq.r.Method == "GET" || rq.r.Method == "HEAD" {
		switch rq.path {
		case "/sitemap.xml":
			if !site.hasRoute(rq.path) {
				s.sitemap(rq)
				return
			}
		case "/robots.txt":
			if !site.hasRoute(rq.path) {
				s.robots(rq)
				return
			}
		case "/feed.xml":
			if !site.hasRoute(rq.path) && site.hasFeed() {
				s.feed(rq)
				return
			}
		}
	}
	if rq.path == "/_nep/comentar" {
		s.comment(rq)
		return
	}
	if rq.path == "/_nep/lowii" {
		s.lowii(rq)
		return
	}
	if err := s.route(rq); err != nil {
		s.fail(rq, err)
	}
}

func asset(w http.ResponseWriter, r *http.Request, b []byte) {
	w.Header().Set("Content-Type", "text/javascript; charset=utf-8")
	w.Header().Set("Cache-Control", "public, max-age=86400")
	http.ServeContent(w, r, "", time.Time{}, strings.NewReader(string(b)))
}

func (s *Server) headers(w http.ResponseWriter, rq *request) {
	h := w.Header()
	h.Set("X-Content-Type-Options", "nosniff")
	h.Set("X-Frame-Options", "DENY")
	h.Set("Referrer-Policy", "strict-origin-when-cross-origin")
	h.Set("Permissions-Policy", "camera=(), microphone=(), geolocation=()")
	csp := "default-src 'self'; img-src 'self' data: https:; style-src 'self' 'unsafe-inline' https:; script-src 'self' 'unsafe-inline' https:; " +
		"font-src 'self' data: https:; frame-src https:; connect-src 'self'; frame-ancestors 'none'; base-uri 'self'; form-action 'self'; object-src 'none'"
	if s.site != nil && s.site.Config["csp"] != "" {
		csp = s.site.Config["csp"]
	}
	h.Set("Content-Security-Policy", csp)
	if rq.https {
		h.Set("Strict-Transport-Security", "max-age=31536000")
	}
}

var staticExt = map[string]bool{
	".css": true, ".js": true, ".mjs": true, ".png": true, ".jpg": true, ".jpeg": true, ".gif": true, ".svg": true, ".webp": true,
	".avif": true, ".ico": true, ".woff": true, ".woff2": true, ".ttf": true, ".otf": true, ".pdf": true, ".txt": true,
	".mp4": true, ".webm": true, ".mp3": true, ".ogg": true, ".wav": true, ".xml": true, ".webmanifest": true, ".html": true,
}

// static entrega arquivos da pasta do site (imagens, CSS…), nunca os internos.
func (s *Server) static(w http.ResponseWriter, r *http.Request, p string) bool {
	if s.opt.CGI || p == "/" {
		return false // no CGI, o Apache entrega os arquivos
	}
	low := strings.ToLower(p)
	if !staticExt[filepath.Ext(low)] || strings.HasSuffix(low, ".nep.html") {
		return false
	}
	for _, seg := range strings.Split(p, "/") {
		if strings.HasPrefix(seg, ".") {
			return false
		}
	}
	full := filepath.Join(s.dir, filepath.FromSlash(p))
	rel, err := filepath.Rel(s.dir, full)
	if err != nil || strings.HasPrefix(rel, "..") {
		return false
	}
	if dd, err := filepath.Abs(s.opt.DataDir); err == nil {
		if r2, err := filepath.Rel(dd, full); err == nil && !strings.HasPrefix(r2, "..") {
			return false
		}
	}
	st, err := os.Stat(full)
	if err != nil || st.IsDir() {
		return false
	}
	if ct := mime.TypeByExtension(filepath.Ext(low)); ct != "" {
		w.Header().Set("Content-Type", ct)
	}
	http.ServeFile(w, r, full)
	return true
}

// events manda a versão dos dados sempre que ela muda (Server-Sent Events).
func (s *Server) events(w http.ResponseWriter, r *http.Request) {
	fl, ok := w.(http.Flusher)
	if !ok || s.opt.CGI {
		http.Error(w, "sem suporte", http.StatusNotImplemented)
		return
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-store")
	last := ""
	tick := time.NewTicker(time.Second)
	defer tick.Stop()
	timeout := time.After(10 * time.Minute)
	for {
		if v := s.Store.Version(); v != last {
			last = v
			fmt.Fprintf(w, "data: %s\n\n", v)
			fl.Flush()
		}
		select {
		case <-r.Context().Done():
			return
		case <-timeout:
			return
		case <-tick.C:
		}
	}
}

func (s *Server) url(p string) string {
	if p == "" {
		p = "/"
	}
	if extURL.MatchString(p) {
		return p
	}
	return s.opt.Base + "/" + strings.TrimLeft(p, "/")
}

var extURL = regexp.MustCompile(`(?i)^([a-z][a-z0-9+.\-]*:|//|#)`)

func localPath(p string) bool {
	return strings.HasPrefix(p, "/") && !strings.HasPrefix(p, "//") && !strings.HasPrefix(p, "/\\")
}

// ---------- rotas ----------

func (s *Server) route(rq *request) error {
	site := rq.site
	r := rq.r
	for _, a := range site.APIs {
		if m := a.re.FindStringSubmatch(rq.path); m != nil {
			if a.Method != r.Method && !(a.Method == "GET" && r.Method == "HEAD") && a.Method != "*" {
				continue
			}
			return s.api(rq, a, m[1:])
		}
	}
	var params []string
	for _, p := range site.Pages {
		if p.re == nil {
			continue
		}
		if m := p.re.FindStringSubmatch(rq.path); m != nil {
			rq.page, params = p, m[1:]
			break
		}
	}
	status := http.StatusOK
	if rq.page == nil {
		status = http.StatusNotFound
		rq.page = site.page404()
		if rq.page == nil {
			s.simple(rq, 404, "Página não encontrada", "<p>O endereço <code>"+html.EscapeString(rq.path)+"</code> não existe.</p>")
			return nil
		}
	}
	if rq.page.Auth && rq.sess.isAdmin() && !rq.sess.isOwner() {
		s.simple(rq, 403, "Só para o dono do site", "<p>Esta página é só para o dono do site.</p>")
		return nil
	}
	if rq.page.Auth && !rq.sess.isAdmin() {
		s.redirect(rq, s.opt.Base+"/_admin/login?next="+url.QueryEscape(rq.path), http.StatusFound)
		return nil
	}
	switch r.Method {
	case "GET", "HEAD", "POST":
	default:
		rq.w.Header().Set("Allow", "GET, POST")
		http.Error(rq.w, "método não permitido", http.StatusMethodNotAllowed)
		return nil
	}
	if r.Method == "POST" && status == 404 {
		s.simple(rq, 404, "Página não encontrada", "")
		return nil
	}
	return s.renderPage(rq, status, params)
}

func (site *Site) page404() *Page {
	for _, p := range site.Pages {
		if p.Route == "404" {
			return p
		}
	}
	return nil
}

func (s *Server) renderPage(rq *request, status int, params []string) error {
	if err := s.setup(rq, rq.page.ID, params); err != nil {
		return s.handleHalt(rq, err, status)
	}
	pre, body := rq.site.Sources(rq.page.ID)
	if _, err := rq.vm.RunSource(pre, rq.site.File, rq.mod()); err != nil {
		return s.handleHalt(rq, err, status)
	}
	if rq.r.Method == "POST" {
		done, err := s.handlePost(rq)
		if err != nil {
			return s.handleHalt(rq, err, status)
		}
		if done {
			return nil
		}
		status = http.StatusUnprocessableEntity
	}
	if _, err := rq.vm.RunSource(body, rq.site.File, rq.mod()); err != nil {
		return s.handleHalt(rq, err, status)
	}
	out := rq.out.String()

	if id := rq.r.URL.Query().Get("_nep_live"); id != "" {
		n, _ := strconv.Atoi(id)
		at, ok := rq.liveAt[n]
		if !ok || at[1] < at[0] {
			http.NotFound(rq.w, rq.r)
			return nil
		}
		rq.saveSession()
		rq.w.Header().Set("Content-Type", "text/html; charset=utf-8")
		rq.w.Header().Set("Cache-Control", "no-store")
		io.WriteString(rq.w, out[at[0]:at[1]])
		return nil
	}

	out = s.inject(rq, out)
	if status == http.StatusOK && s.statsOn(rq.site) {
		s.countVisit(rq)
	}
	rq.saveSession()
	rq.w.Header().Set("Content-Type", "text/html; charset=utf-8")
	rq.w.WriteHeader(status)
	if rq.r.Method != "HEAD" {
		io.WriteString(rq.w, out)
	}
	return nil
}

// handleHalt trata not_found(), redirect() e abort() vindos do código.
func (s *Server) handleHalt(rq *request, err error, status int) error {
	h, ok := haltOf(err)
	if !ok {
		return err
	}
	switch h.kind {
	case "redirect":
		rq.saveSession()
		s.redirect(rq, h.to, http.StatusFound)
		return nil
	case "abort":
		s.simple(rq, h.status, "Erro "+strconv.Itoa(h.status), "<p>"+html.EscapeString(h.msg)+"</p>")
		return nil
	case "not_found":
		p404 := rq.site.page404()
		if p404 == nil || rq.page == p404 {
			s.simple(rq, 404, "Página não encontrada", "<p>O conteúdo procurado não existe.</p>")
			return nil
		}
		rq.page = p404
		rq.r.Method = "GET"
		return s.renderPage(rq, http.StatusNotFound, nil)
	}
	return err
}

// inject acrescenta os scripts de movimento e de regiões vivas.
func (s *Server) inject(rq *request, out string) string {
	cfg := rq.site.Config
	inicio, _ := s.temaTags(cfg, out)
	out = noComecoDoHead(out, inicio)
	tags := s.seoTags(rq, out)
	mv := strings.ToLower(strings.TrimSpace(cfg["movimento"]))
	if mv != "" && mv != "false" && mv != "0" && mv != "nao" && mv != "não" {
		t := `<script src="` + html.EscapeString(s.opt.Base) + `/_nep/movimento.js" defer`
		if mv == "suave" || mv == "forte" {
			t += ` data-intensidade="` + mv + `"`
		}
		if _, ok := cfg["progresso"]; ok {
			t += ` data-progresso="true"`
		}
		if c := cfg["cor"]; c != "" {
			t += ` data-cor="` + html.EscapeString(c) + `"`
		}
		tags = append(tags, t+`></script>`)
	}
	if rq.lives {
		sse := "0"
		if !s.opt.CGI {
			sse = "1"
		}
		tags = append(tags, `<script src="`+html.EscapeString(s.opt.Base)+`/_nep/vivo.js" defer data-base="`+html.EscapeString(s.opt.Base)+`" data-sse="`+sse+`"></script>`)
	}
	var tail string
	if rq.sess != nil && rq.sess.isAdmin() && strings.Contains(out, "data-nep-ed=") {
		// quem está no painel edita os textos ali mesmo (rq.sess.CSRF só vai para o próprio admin)
		tail = `<script src="` + html.EscapeString(s.opt.Base) + `/_nep/editar.js" defer data-base="` + html.EscapeString(s.opt.Base) + `" data-csrf="` + html.EscapeString(rq.sess.CSRF) + `"></script>`
		rq.w.Header().Set("Cache-Control", "private, no-store")
	}
	if tail != "" {
		tags = append(tags, tail)
	}
	if len(tags) == 0 {
		return out
	}
	t := strings.Join(tags, "\n")
	if i := strings.Index(strings.ToLower(out), "</head>"); i >= 0 {
		return out[:i] + t + "\n" + out[i:]
	}
	return t + out
}

func (s *Server) redirect(rq *request, to string, code int) {
	rq.saveSession()
	http.Redirect(rq.w, rq.r, to, code)
}

// ---------- API JSON ----------

func (s *Server) api(rq *request, a *API, params []string) error {
	rq.w.Header().Set("Cache-Control", "no-store")
	if err := s.setup(rq, -1, nil); err != nil {
		return s.apiErr(rq, err)
	}
	pt := lang.NewTable()
	for i, n := range a.params {
		if i < len(params) {
			pt.SetStr(n, params[i])
		}
	}
	rq.mod().Define("params", pt)
	if rq.r.Method != "GET" && rq.r.Method != "HEAD" {
		// proteção contra envio de outro site: exige JSON ou o mesmo Origin
		ct := rq.r.Header.Get("Content-Type")
		origin := rq.r.Header.Get("Origin")
		if !strings.HasPrefix(ct, "application/json") && origin != "" && !sameOrigin(origin, rq.r) {
			return s.apiJSON(rq, 403, errObj("origem não permitida"))
		}
		body, _ := io.ReadAll(io.LimitReader(rq.r.Body, 1<<20))
		req, _ := rq.mod().Globals["request"].(*lang.Table)
		if strings.HasPrefix(ct, "application/json") && len(body) > 0 {
			v, err := lang.FromJSON(string(body))
			if err != nil {
				return s.apiJSON(rq, 400, errObj("JSON inválido"))
			}
			req.SetStr("json", v)
		} else {
			vals, _ := url.ParseQuery(string(body))
			req.SetStr("form", valuesTable(vals))
		}
	}
	pre, _ := rq.site.Sources(-1)
	if _, err := rq.vm.RunSource(pre, rq.site.File, rq.mod()); err != nil {
		return s.apiErr(rq, err)
	}
	if _, err := rq.vm.RunSource(rq.site.APISource(a), rq.site.File, rq.mod()); err != nil {
		return s.apiErr(rq, err)
	}
	v, err := rq.vm.Call(rq.mod().Globals["__api"], nil)
	if err != nil {
		return s.apiErr(rq, err)
	}
	rq.saveSession()
	return s.apiJSON(rq, 200, v)
}

func errObj(msg string) lang.Value {
	t := lang.NewTable()
	t.SetStr("erro", msg)
	return t
}

// haltOf extrai a parada (not_found, redirect, abort, reject) de um erro.
func haltOf(err error) (halt, bool) {
	var lh *lang.Halt
	if errors.As(err, &lh) {
		h, ok := lh.Reason.(halt)
		return h, ok
	}
	var re *lang.RuntimeError
	if errors.As(err, &re) && re.Halt != nil {
		h, ok := re.Halt.(halt)
		return h, ok
	}
	return halt{}, false
}

func (s *Server) apiErr(rq *request, err error) error {
	if h, ok := haltOf(err); ok {
		switch h.kind {
		case "abort":
			return s.apiJSON(rq, h.status, errObj(h.msg))
		case "not_found":
			return s.apiJSON(rq, 404, errObj("não encontrado"))
		case "redirect":
			s.redirect(rq, h.to, http.StatusFound)
			return nil
		}
	}
	s.logErr(rq, err)
	msg := "erro interno"
	if s.opt.Dev {
		msg = err.Error()
	}
	return s.apiJSON(rq, 500, errObj(msg))
}

func (s *Server) apiJSON(rq *request, status int, v lang.Value) error {
	j, err := lang.ToJSON(v, "")
	if err != nil {
		j = `{"erro":"valor não pode virar JSON"}`
		status = 500
	}
	rq.w.Header().Set("Content-Type", "application/json; charset=utf-8")
	rq.w.WriteHeader(status)
	io.WriteString(rq.w, j)
	return nil
}

func sameOrigin(origin string, r *http.Request) bool {
	u, err := url.Parse(origin)
	return err == nil && strings.EqualFold(u.Host, r.Host)
}

// ---------- erros ----------

func (s *Server) logErr(rq *request, err error) {
	line := fmt.Sprintf("%s %s %s: %s\n", time.Now().Format(time.RFC3339), rq.r.Method, rq.path, strings.ReplaceAll(err.Error(), "\n", " | "))
	if f, e := os.OpenFile(filepath.Join(s.opt.DataDir, "erros.log"), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o640); e == nil {
		f.WriteString(line)
		f.Close()
	}
	if s.opt.Dev {
		fmt.Fprint(s.opt.Log, line)
	}
}

var traceLine = regexp.MustCompile(`\(([^():]+):(\d+)\)$`)

// errLine acha a linha do .nep.html onde o erro aconteceu.
func errLine(err error, file string) int {
	var te *TemplateError
	if errors.As(err, &te) {
		return te.Line
	}
	var se *lang.SyntaxError
	if errors.As(err, &se) {
		return se.Line
	}
	var re *lang.RuntimeError
	if errors.As(err, &re) {
		for _, t := range re.Trace {
			if m := traceLine.FindStringSubmatch(t); m != nil && m[1] == file {
				n, _ := strconv.Atoi(m[2])
				return n
			}
		}
	}
	return 0
}

func (s *Server) fail(rq *request, err error) {
	s.logErr(rq, err)
	if !s.opt.Dev {
		s.simple(rq, 500, "Algo deu errado", "<p>Tente novamente em instantes.</p>")
		return
	}
	msg := err.Error()
	var re *lang.RuntimeError
	if errors.As(err, &re) {
		msg = strings.SplitN(re.Error(), "\n", 2)[0]
		msg = strings.TrimPrefix(msg, "Erro: ")
	}
	body := `<p class="err"><strong>` + html.EscapeString(msg) + `</strong></p>`
	src := ""
	if rq.site != nil {
		src = rq.site.Src
	} else if b, e := os.ReadFile(s.file); e == nil {
		src = string(b)
	}
	if line := errLine(err, filepath.Base(s.file)); line > 0 && src != "" {
		lines := strings.Split(src, "\n")
		from, to := max(1, line-4), min(len(lines), line+4)
		body += `<p class="muted">` + html.EscapeString(filepath.Base(s.file)) + ", linha " + strconv.Itoa(line) + "</p><pre>"
		for i := from; i <= to; i++ {
			t := fmt.Sprintf("%4d  %s", i, html.EscapeString(strings.TrimRight(lines[i-1], " \t")))
			if i == line {
				body += `<span class="hl">` + t + "</span>\n"
			} else {
				body += t + "\n"
			}
		}
		body += "</pre>"
	}
	if re != nil && len(re.Trace) > 1 {
		body += "<pre>" + html.EscapeString(strings.Join(re.Trace, "\n")) + "</pre>"
	}
	body += `<p class="muted">Você vê os detalhes porque o servidor está em modo de desenvolvimento (nep serve --dev). Em produção, o visitante vê só uma mensagem genérica.</p>`
	s.simple(rq, 500, "Erro no site", body)
}

const sysCSS = `body{font:16px/1.5 system-ui,-apple-system,"Segoe UI",Roboto,sans-serif;margin:0;background:#f6f7f4;color:#1d2a22}` +
	`.nep-box{max-width:960px;margin:40px auto;padding:24px;background:#fff;border:1px solid #dfe3dc;border-radius:12px}` +
	`h1{font-size:1.5rem;margin:0 0 16px}a{color:#2b6b45}code,pre{background:#f0f2ee;border-radius:6px}code{padding:1px 5px}` +
	`pre{padding:12px;overflow:auto;font-size:13px}.err{color:#a3261f}table{border-collapse:collapse;width:100%;font-size:14px}` +
	`th,td{border-bottom:1px solid #e5e8e2;padding:8px;text-align:left;vertical-align:top}th{background:#f4f6f2}` +
	`input,button{font:inherit;padding:8px 10px;border:1px solid #c9cfc5;border-radius:8px}button{background:#2b6b45;color:#fff;border-color:#2b6b45;cursor:pointer}` +
	`button.ghost{background:#fff;color:#a3261f;border-color:#e2b8b4}.row{display:flex;gap:8px;flex-wrap:wrap;align-items:center}` +
	`nav a{margin-right:14px}.muted{color:#68756c;font-size:14px}.ok{background:#e9f5ec;padding:10px;border-radius:8px}` +
	`.scroll{overflow:auto}.hl{background:#fde2df;display:block}`

func sysPage(title, body string) string {
	return `<!doctype html><html lang="pt-BR"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1">` +
		`<meta name="robots" content="noindex"><title>` + html.EscapeString(title) + `</title><style>` + sysCSS + `</style></head><body><main class="nep-box"><h1>` +
		html.EscapeString(title) + `</h1>` + body + `</main></body></html>`
}

func (s *Server) simple(rq *request, status int, title, body string) {
	if rq.sess != nil {
		rq.saveSession()
	}
	rq.w.Header().Set("Content-Type", "text/html; charset=utf-8")
	rq.w.WriteHeader(status)
	io.WriteString(rq.w, sysPage(title, body))
}

func (rq *request) saveSession() {
	if rq.sess == nil {
		return
	}
	if rq.sessVal != nil {
		if j, err := lang.ToJSON(rq.sessVal, ""); err == nil && j != rq.sessRaw {
			rq.sess.Data = []byte(j)
			rq.sess.changed = true
			rq.sessRaw = j
		}
	}
	path := rq.s.opt.Base + "/"
	if err := rq.sess.save(rq.w, rq.s.secret, path, rq.https); err != nil {
		rq.s.logErr(rq, err)
	}
	rq.sess.changed = false
}
