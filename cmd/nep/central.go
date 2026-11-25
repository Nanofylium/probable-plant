package main

// Central do Nepenthes: o que abre com dois cliques no Nepenthes.exe. Uma tela
// no navegador com tudo num lugar só: meus sites (abrir, painel e banco de
// dados, prévia, pasta), criar site descrevendo, assistente com IA, modelos
// prontos e a IA local. Cada site roda neste
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
	db     *nepDB
	tarefa struct {
		Ativa    bool   `json:"ativa"`
		Texto    string `json:"texto"`
		Progr    int    `json:"progresso"`
		Erro     string `json:"erro,omitempty"`
		Resposta any    `json:"resposta,omitempty"`
	}
}

type siteInfo struct {
	Nome     string `json:"nome"`
	Pasta    string `json:"pasta"`
	Arquivo  string `json:"arquivo"`
	Rodando  bool   `json:"rodando"`
	URL      string `json:"url,omitempty"`
	Previa   bool   `json:"previa"`
	Alterado string `json:"alterado"`
	Novo     bool   `json:"novo"` // ainda só com a tag <nepenthes> (assistente)
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
	c := &central{base: base, token: hex.EncodeToString(b), sites: map[string]*siteRodando{}, db: novoNepDB(base)}
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
		si := siteInfo{Nome: filepath.Base(abs), Pasta: abs, Arquivo: filepath.Base(f)}
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
	case r.URL.Path == "/_nep/echarts.js":
		w.Header().Set("Content-Type", "text/javascript; charset=utf-8")
		w.Header().Set("Cache-Control", "max-age=86400")
		w.Write(web.EChartsJS())
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
		Pasta, Descricao, Visual, Nome, Modelo, Titulo, SQL, ID string
		Escrever                                                bool
		Painel                                                  painel
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
			"ia": ialocal.Ver(), "db": c.db.Estado(), "visuais": visuais, "modelos": modelos(),
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
	case "/api/db/ligar": // Nepenthes DB: PostgreSQL sem Docker
		c.db.LembrarLigado(true)
		go c.db.Ligar()
		resp(map[string]bool{"ok": true}, nil)
	case "/api/db/parar":
		c.db.LembrarLigado(false)
		resp(map[string]bool{"ok": true}, c.db.Desligar())
	case "/api/db/consulta":
		resp(c.db.Consultar(r.Context(), in.SQL, in.Escrever))
	case "/api/db/exemplo":
		resp(map[string]bool{"ok": true}, c.db.Exemplo(r.Context()))
	case "/api/db/paineis":
		if r.Method == http.MethodPost {
			if in.ID != "" {
				resp(map[string]bool{"ok": true}, c.db.ApagarPainel(in.ID))
			} else {
				resp(c.db.SalvarPainel(in.Painel))
			}
			return
		}
		resp(c.db.Paineis(), nil)
	case "/api/db/pasta":
		os.MkdirAll(c.db.pasta, 0o755)
		abrirPasta(c.db.pasta)
		resp(map[string]bool{"ok": true}, nil)
	case "/api/ia/baixar":
		go ialocal.Baixar()
		resp(map[string]bool{"ok": true}, nil)
	default:
		http.NotFound(w, r)
	}
}

// pararTudo desliga os sites, o banco e a IA local (ao sair).
func (c *central) pararTudo() {
	c.mu.Lock()
	var ps []string
	for p := range c.sites {
		ps = append(ps, p)
	}
	c.mu.Unlock()
	for _, p := range ps {
		c.parar(p)
	}
	c.db.Desligar()
	ialocal.Parar()
}

// modoCentral: dois cliques no Nepenthes.exe abrem a Central no navegador e
// põem o ícone do Nepenthes perto do relógio. abrir=false (--bandeja, ao
// ligar o computador) só deixa tudo pronto, sem abrir o navegador.
func modoCentral(abrir bool) int {
	if r, err := http.Get("http://127.0.0.1:8000/api/estado"); err == nil {
		r.Body.Close()
		if abrir {
			fmt.Println("O Nepenthes já está aberto: abrindo a Central no navegador.")
			abrirNavegador("http://localhost:8000/")
			time.Sleep(2 * time.Second)
		}
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
	if c.db.cfg.Ligar {
		go c.db.Ligar()
	}
	// na primeira vez, um atalho na área de trabalho
	marca := filepath.Join(c.base, ".atalho")
	if _, err := os.Stat(marca); err != nil {
		go func() {
			if criarAtalho() == nil {
				os.WriteFile(marca, []byte("ok"), 0o644)
			}
		}()
	}
	if abrir {
		go func() {
			time.Sleep(300 * time.Millisecond)
			abrirNavegador("http://localhost:8000/")
		}()
	}
	hs := &http.Server{Handler: c, ReadHeaderTimeout: 10 * time.Second}
	go hs.Serve(l)
	bandeja(c, func() {
		hs.Close()
		c.pararTudo()
	})
	return 0
}
