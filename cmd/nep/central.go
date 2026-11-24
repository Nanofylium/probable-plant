package main

// Central do Nepenthes: o que abre com dois cliques no Nepenthes.exe. Uma tela
// no navegador com tudo num lugar só: meus sites (abrir, painel e banco de
// dados, prévia, pasta), criar site descrevendo, assistente com IA, modelos
// prontos, passeio 3D a partir de uma foto e a IA local. Cada site roda neste
// mesmo programa, numa porta própria (8001, 8002…).

import (
	"crypto/rand"
	_ "embed"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/Nanofylium/probable-plant/gerador"
	"github.com/Nanofylium/probable-plant/ialocal"
	"github.com/Nanofylium/probable-plant/lang"
	"github.com/Nanofylium/probable-plant/mundo"
	"github.com/Nanofylium/probable-plant/rede"
	"github.com/Nanofylium/probable-plant/web"
)

//go:embed central.html
var centralHTML string

type siteRodando struct {
	porta int
	hs    *http.Server
}

type central struct {
	base   string // pasta onde os sites novos são criados
	extras []string
	token  string
	mu     sync.Mutex
	sites  map[string]*siteRodando // pasta -> servidor
	tarefa struct {
		Ativa    bool   `json:"ativa"`
		Texto    string `json:"texto"`
		Progr    int    `json:"progresso"`
		Erro     string `json:"erro,omitempty"`
		Resposta any    `json:"resposta,omitempty"`
	}
}

type siteInfo struct {
	Nome     string   `json:"nome"`
	Pasta    string   `json:"pasta"`
	Arquivo  string   `json:"arquivo"`
	Rodando  bool     `json:"rodando"`
	URL      string   `json:"url,omitempty"`
	Previa   bool     `json:"previa"`
	Mundos   []string `json:"mundos"`
	Alterado string   `json:"alterado"`
	Novo     bool     `json:"novo"` // ainda só com a tag <nepenthes> (assistente)
}

// pastaBase: "Sites do Nepenthes" ao lado do executável; se ele rodou de
// dentro de um zip ou de uma pasta sem permissão, Documentos\Nepenthes.
func pastaBase() string {
	if exe, err := os.Executable(); err == nil {
		d := filepath.Dir(exe)
		tmp := strings.ToLower(os.TempDir())
		if !strings.HasPrefix(strings.ToLower(d), tmp) && !strings.Contains(strings.ToLower(d), `\temp\`) && podeEscrever(d) {
			return filepath.Join(d, "Sites do Nepenthes")
		}
	}
	home, err := os.UserHomeDir()
	if err != nil {
		home = "."
	}
	return filepath.Join(home, "Documents", "Nepenthes")
}

func novaCentral(base string) *central {
	b := make([]byte, 16)
	rand.Read(b)
	c := &central{base: base, token: hex.EncodeToString(b), sites: map[string]*siteRodando{}}
	// os sites das versões anteriores ("Meu site" ao lado do programa, ou
	// Documentos\Nepenthes) continuam aparecendo
	if exe, err := os.Executable(); err == nil {
		c.extras = append(c.extras, filepath.Join(filepath.Dir(exe), "Meu site"))
	}
	c.extras = append(c.extras, base)
	return c
}

// arquivoDoSite acha o site da pasta: site.nep.html ou index.html.
func arquivoDoSite(pasta string) string {
	for _, n := range []string{"site.nep.html", "index.html"} {
		f := filepath.Join(pasta, n)
		if st, err := os.Stat(f); err == nil && !st.IsDir() {
			return f
		}
	}
	return ""
}

func (c *central) listar() []siteInfo {
	vistos := map[string]bool{}
	var pastas []string
	if ents, err := os.ReadDir(c.base); err == nil {
		for _, e := range ents {
			if e.IsDir() && !strings.HasPrefix(e.Name(), ".") {
				pastas = append(pastas, filepath.Join(c.base, e.Name()))
			}
		}
	}
	pastas = append(pastas, c.extras...)
	out := []siteInfo{}
	c.mu.Lock()
	defer c.mu.Unlock()
	for _, p := range pastas {
		abs, _ := filepath.Abs(p)
		f := arquivoDoSite(abs)
		if f == "" || vistos[abs] {
			continue
		}
		vistos[abs] = true
		si := siteInfo{Nome: filepath.Base(abs), Pasta: abs, Arquivo: filepath.Base(f), Mundos: []string{}}
		if st, err := os.Stat(f); err == nil {
			si.Alterado = st.ModTime().Format("02/01/2006 15:04")
		}
		if b, err := os.ReadFile(f); err == nil && reTagNepenthes.Match(b) {
			si.Novo = true
		}
		prev := strings.TrimSuffix(strings.TrimSuffix(f, ".html"), ".nep") + "-previa.html"
		if _, err := os.Stat(prev); err == nil {
			si.Previa = true
		}
		if ms, err := os.ReadDir(filepath.Join(abs, "mundos")); err == nil {
			for _, m := range ms {
				if m.IsDir() {
					si.Mundos = append(si.Mundos, m.Name())
				}
			}
		}
		if r := c.sites[abs]; r != nil {
			si.Rodando, si.URL = true, fmt.Sprintf("http://localhost:%d/", r.porta)
		}
		out = append(out, si)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Alterado > out[j].Alterado })
	return out
}

// pastaConhecida: só mexe em pastas que a Central lista (nunca num caminho qualquer).
func (c *central) pastaConhecida(p string) (string, bool) {
	abs, err := filepath.Abs(p)
	if err != nil {
		return "", false
	}
	for _, s := range c.listar() {
		if s.Pasta == abs {
			return abs, true
		}
	}
	return "", false
}

func portaLivre(de int) int {
	for p := de; p < de+200; p++ {
		l, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", p))
		if err == nil {
			l.Close()
			return p
		}
	}
	return 0
}

// ligar roda o site da pasta numa porta própria (se já não estiver rodando).
func (c *central) ligar(pasta string) (string, error) {
	c.mu.Lock()
	if r := c.sites[pasta]; r != nil {
		c.mu.Unlock()
		return fmt.Sprintf("http://localhost:%d/", r.porta), nil
	}
	c.mu.Unlock()
	f := arquivoDoSite(pasta)
	if f == "" {
		return "", errors.New("a pasta não tem site.nep.html nem index.html")
	}
	srv, err := web.New(web.Options{SiteFile: f, AposCriar: previaAoCriar})
	if srv == nil {
		return "", err
	}
	porta := portaLivre(8001)
	if porta == 0 {
		return "", errors.New("não achei porta livre")
	}
	l, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", porta))
	if err != nil {
		return "", err
	}
	hs := &http.Server{Handler: srv, ReadHeaderTimeout: 10 * time.Second, IdleTimeout: 2 * time.Minute}
	go hs.Serve(l)
	c.mu.Lock()
	c.sites[pasta] = &siteRodando{porta: porta, hs: hs}
	c.mu.Unlock()
	return fmt.Sprintf("http://localhost:%d/", porta), nil
}

func (c *central) parar(pasta string) {
	c.mu.Lock()
	r := c.sites[pasta]
	delete(c.sites, pasta)
	c.mu.Unlock()
	if r != nil {
		r.hs.Close()
	}
}

// pastaNova cria uma pasta com nome único dentro da base.
func (c *central) pastaNova(nome string) (string, error) {
	n := slugMundo(nome)
	if n == "passeio" && strings.TrimSpace(nome) == "" {
		n = "meu-site"
	}
	for i := 1; i < 100; i++ {
		p := filepath.Join(c.base, n)
		if i > 1 {
			p = filepath.Join(c.base, fmt.Sprintf("%s-%d", n, i))
		}
		if _, err := os.Stat(p); os.IsNotExist(err) {
			return p, os.MkdirAll(p, 0o755)
		}
	}
	return "", errors.New("não consegui criar a pasta")
}

func abrirPasta(p string) {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "windows":
		cmd = exec.Command("explorer", p)
	case "darwin":
		cmd = exec.Command("open", p)
	default:
		cmd = exec.Command("xdg-open", p)
	}
	_ = cmd.Start()
}

func (c *central) comecaTarefa(texto string) bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.tarefa.Ativa {
		return false
	}
	c.tarefa.Ativa, c.tarefa.Texto, c.tarefa.Progr, c.tarefa.Erro, c.tarefa.Resposta = true, texto, 0, "", nil
	return true
}

func (c *central) tarefaPasso(texto string, p int) {
	c.mu.Lock()
	c.tarefa.Texto, c.tarefa.Progr = texto, p
	c.mu.Unlock()
}

func (c *central) fimTarefa(resp any, err error) {
	c.mu.Lock()
	c.tarefa.Ativa, c.tarefa.Progr, c.tarefa.Resposta = false, 100, resp
	if err != nil {
		c.tarefa.Erro = err.Error()
	}
	c.mu.Unlock()
}

// passeio3D faz o passeio de uma foto dentro de um site (com o modelo de
// profundidade; sem internet na primeira vez, com a profundidade estimada).
func (c *central) passeio3D(pasta string, foto []byte, nome, titulo string) {
	m := &mundo.Mundo{Nome: nome, Titulo: titulo, Fonte: "depth-anything"}
	var mapa *mundo.Mapa
	usar := true
	if !mundo.ModeloProfBaixado() {
		c.tarefaPasso("Baixando o modelo de profundidade (95 MB, só desta vez)…", 1)
		if err := mundo.BaixarModeloProf(func(f, t int64) {
			c.tarefaPasso("Baixando o modelo de profundidade (95 MB, só desta vez)…", int(f*80/t))
		}); err != nil {
			usar = false
		}
	}
	if usar {
		c.tarefaPasso("Calculando a profundidade da foto no processador (uns 10 segundos)…", 85)
		rd, err := rede.Abrir(mundo.CaminhoModeloProf())
		if err == nil {
			mapa, err = mundo.CalcularProfundidade(rd, foto, 518)
		}
		if err != nil {
			usar = false
		}
	}
	if !usar {
		m.Fonte = "estimado"
		var err error
		if mapa, err = mundo.EstimarProfundidade(foto); err != nil {
			c.fimTarefa(nil, err)
			return
		}
	}
	err := mundo.Salvar3D(filepath.Join(pasta, "mundos"), m, foto, mapa)
	c.fimTarefa(map[string]any{"pasta": pasta, "nome": nome, "fonte": m.Fonte, "codigo": fmt.Sprintf(`{{ mundo(%q) }}`, nome)}, err)
}

func (c *central) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// só quem está neste computador usa a Central
	if h, _, _ := net.SplitHostPort(r.RemoteAddr); h != "127.0.0.1" && h != "::1" {
		http.Error(w, "a Central só abre neste computador", http.StatusForbidden)
		return
	}
	// contra páginas de fora fingindo ser localhost (DNS rebinding)
	if h, _, err := net.SplitHostPort(r.Host); err != nil || (h != "localhost" && h != "127.0.0.1" && h != "[::1]" && h != "::1") {
		http.Error(w, "endereço inválido", http.StatusForbidden)
		return
	}
	switch {
	case r.URL.Path == "/":
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Header().Set("Cache-Control", "no-store")
		io.WriteString(w, strings.Replace(centralHTML, "{{token}}", c.token, 1))
		return
	case r.URL.Path == "/_nep/mundo.js":
		w.Header().Set("Content-Type", "text/javascript; charset=utf-8")
		w.Write(web.MundoJS())
		return
	case strings.HasPrefix(r.URL.Path, "/ver/"):
		c.verMundo(w, r)
		return
	case !strings.HasPrefix(r.URL.Path, "/api/"):
		http.NotFound(w, r)
		return
	}
	if r.Method == http.MethodPost && r.Header.Get("X-Central") != c.token {
		http.Error(w, "recarregue a página", http.StatusForbidden)
		return
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	resp := func(v any, err error) {
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(map[string]string{"erro": err.Error()})
			return
		}
		json.NewEncoder(w).Encode(v)
	}
	var in struct {
		Pasta, Descricao, Visual, Nome, Modelo, Titulo string
	}
	if r.Method == http.MethodPost && !strings.HasPrefix(r.Header.Get("Content-Type"), "multipart/") {
		json.NewDecoder(io.LimitReader(r.Body, 64<<10)).Decode(&in)
	}
	switch r.URL.Path {
	case "/api/estado":
		c.mu.Lock()
		tarefa := c.tarefa
		c.mu.Unlock()
		var visuais []map[string]string
		for _, v := range gerador.Visuais {
			visuais = append(visuais, map[string]string{"id": v.ID, "nome": v.Nome})
		}
		resp(map[string]any{
			"sites": c.listar(), "base": c.base, "versao": lang.Version, "tarefa": tarefa,
			"ia": ialocal.Ver(), "profundidade": mundo.ModeloProfBaixado(), "visuais": visuais, "modelos": modelos(),
		}, nil)
	case "/api/abrir":
		p, ok := c.pastaConhecida(in.Pasta)
		if !ok {
			resp(nil, errors.New("site não encontrado"))
			return
		}
		u, err := c.ligar(p)
		resp(map[string]string{"url": u}, err)
	case "/api/parar":
		if p, ok := c.pastaConhecida(in.Pasta); ok {
			c.parar(p)
		}
		resp(map[string]bool{"ok": true}, nil)
	case "/api/pasta":
		p := c.base
		if in.Pasta != "" {
			var ok bool
			if p, ok = c.pastaConhecida(in.Pasta); !ok {
				resp(nil, errors.New("site não encontrado"))
				return
			}
		}
		os.MkdirAll(p, 0o755)
		abrirPasta(p)
		resp(map[string]bool{"ok": true}, nil)
	case "/api/criar": // descreva e o Nepenthes monta (regras, na hora)
		d := strings.TrimSpace(in.Descricao)
		if d == "" {
			resp(nil, errors.New("descreva o site"))
			return
		}
		p := gerador.EntenderComSemente(d, time.Now().UnixNano())
		if in.Visual != "" {
			gerador.TrocarVisual(p, in.Visual)
		}
		pasta, err := c.pastaNova(gerador.Pasta(p))
		if err != nil {
			resp(nil, err)
			return
		}
		f := filepath.Join(pasta, "site.nep.html")
		if err := os.WriteFile(f, []byte(gerador.Montar(p)), 0o644); err != nil {
			resp(nil, err)
			return
		}
		previaAoCriar(f)
		u, err := c.ligar(pasta)
		resp(map[string]any{"url": u, "pasta": pasta, "notas": p.Notas}, err)
	case "/api/assistente": // pasta nova com a tag <nepenthes>: a tela pergunta e a IA escreve
		pasta, err := c.pastaNova(in.Nome)
		if err != nil {
			resp(nil, err)
			return
		}
		if err := os.WriteFile(filepath.Join(pasta, "index.html"), inicioHTML, 0o644); err != nil {
			resp(nil, err)
			return
		}
		u, err := c.ligar(pasta)
		resp(map[string]any{"url": u, "pasta": pasta}, err)
	case "/api/modelo": // site pronto a partir de um modelo
		src, err := modelosFS.ReadFile("modelos/" + in.Modelo + ".nep.html")
		if err != nil {
			resp(nil, errors.New("modelo não existe"))
			return
		}
		pasta, err := c.pastaNova(in.Modelo)
		if err != nil {
			resp(nil, err)
			return
		}
		f := filepath.Join(pasta, "site.nep.html")
		if err := os.WriteFile(f, src, 0o644); err != nil {
			resp(nil, err)
			return
		}
		previaAoCriar(f)
		u, err := c.ligar(pasta)
		resp(map[string]any{"url": u, "pasta": pasta}, err)
	case "/api/ia/baixar":
		go ialocal.Baixar()
		resp(map[string]bool{"ok": true}, nil)
	case "/api/passeio": // foto -> passeio 3D dentro de um site
		if err := r.ParseMultipartForm(40 << 20); err != nil {
			resp(nil, errors.New("foto grande demais (até 40 MB)"))
			return
		}
		p, ok := c.pastaConhecida(r.FormValue("pasta"))
		if !ok {
			resp(nil, errors.New("escolha um site"))
			return
		}
		f, _, err := r.FormFile("foto")
		if err != nil {
			resp(nil, errors.New("escolha uma foto"))
			return
		}
		foto, _ := io.ReadAll(io.LimitReader(f, 40<<20))
		f.Close()
		nome := slugMundo(r.FormValue("nome"))
		if strings.TrimSpace(r.FormValue("nome")) == "" {
			nome = "passeio"
		}
		if _, err := mundo.EstimarProfundidade(foto); err != nil { // confere a foto antes
			resp(nil, err)
			return
		}
		if !c.comecaTarefa("Preparando o passeio 3D…") {
			resp(nil, errors.New("espere a tarefa anterior terminar"))
			return
		}
		go c.passeio3D(p, foto, nome, r.FormValue("titulo"))
		resp(map[string]bool{"ok": true}, nil)
	default:
		http.NotFound(w, r)
	}
}

// /ver/<site>/<mundo>/ mostra o passeio 3D na Central; /ver/<site>/<mundo>/arq/… entrega os arquivos.
func (c *central) verMundo(w http.ResponseWriter, r *http.Request) {
	partes := strings.SplitN(strings.TrimPrefix(r.URL.Path, "/ver/"), "/", 4)
	if len(partes) < 3 {
		http.NotFound(w, r)
		return
	}
	var site *siteInfo
	for _, s := range c.listar() {
		if s.Nome == partes[0] {
			site = &s
			break
		}
	}
	if site == nil {
		http.NotFound(w, r)
		return
	}
	pasta := filepath.Join(site.Pasta, "mundos")
	m, err := mundo.Abrir(pasta, partes[1])
	if err != nil {
		http.NotFound(w, r)
		return
	}
	if len(partes) == 4 && partes[2] == "arq" {
		arq := filepath.Base(partes[3])
		if arq != m.Foto && arq != m.Profundidade {
			http.NotFound(w, r)
			return
		}
		http.ServeFile(w, r, filepath.Join(pasta, m.Nome, arq))
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	fig := web.FiguraMundo("/ver/"+url.PathEscape(site.Nome)+"/"+m.Nome+"/arq/", "", m, m.Titulo, "", false)
	fmt.Fprintf(w, `<!doctype html><html lang="pt-BR"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1"><title>Passeio · %s</title>
<style>body{margin:0;background:#0f1512;color:#eef3ef;font:16px/1.5 system-ui,-apple-system,"Segoe UI",sans-serif;padding:20px}h1{font-size:1.1rem;font-weight:600;margin:0 0 6px;text-align:center}p{text-align:center;opacity:.75;margin:0}</style></head>
<body><h1>Passeio 3D: %s</h1><p>No site, escreva {{ mundo(%q) }} onde ele deve aparecer.</p>%s</body></html>`, m.Nome, m.Nome, m.Nome, fig)
}

// modoCentral: dois cliques no Nepenthes.exe abrem a Central no navegador.
func modoCentral() int {
	if r, err := http.Get("http://127.0.0.1:8000/api/estado"); err == nil {
		r.Body.Close()
		fmt.Println("O Nepenthes já está aberto: abrindo a Central no navegador.")
		abrirNavegador("http://localhost:8000/")
		time.Sleep(2 * time.Second)
		return 0
	}
	c := novaCentral(pastaBase())
	if err := os.MkdirAll(c.base, 0o755); err != nil {
		return esperaErro(err)
	}
	l, err := net.Listen("tcp", "127.0.0.1:8000")
	if err != nil {
		return esperaErro(fmt.Errorf("a porta 8000 está ocupada por outro programa: %v", err))
	}
	fmt.Println("Central do Nepenthes: http://localhost:8000/")
	fmt.Println("Seus sites ficam em:", c.base)
	fmt.Println("Deixe esta janela aberta enquanto usa. Para parar, feche-a.")
	go func() {
		time.Sleep(300 * time.Millisecond)
		abrirNavegador("http://localhost:8000/")
	}()
	defer ialocal.Parar()
	hs := &http.Server{Handler: c, ReadHeaderTimeout: 10 * time.Second}
	if err := hs.Serve(l); err != nil {
		return esperaErro(err)
	}
	return 0
}
