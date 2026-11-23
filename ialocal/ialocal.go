// Package ialocal é a IA própria do Nepenthes: roda no computador (ou no
// servidor) de quem usa, sem placa de vídeo, sem navegador e sem chave.
//
//   - motor: llama.cpp (github.com/ggml-org/llama.cpp, licença MIT), o
//     programa llama-server, que fala a mesma API de chat que o resto do nep;
//   - modelo: IBM Granite 3.3 2B Instruct (licença Apache 2.0), 1,5 GB.
//
// Nada disso vai dentro do executável: na primeira vez que alguém pede, o nep
// baixa o motor do seu sistema e o modelo dos Releases do probable-plant
// (ou, se lá não estiver, das fontes originais: GitHub do llama.cpp e Hugging
// Face da IBM), confere o SHA-256 de cada arquivo e guarda tudo em
// <pasta de cache>/nepenthes/ia (ou em NEP_IA_LOCAL). Depois roda sem internet.
package ialocal

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"
)

const (
	Versao        = "b12428" // llama.cpp
	ModeloNome    = "Granite 3.3 2B"
	modeloArquivo = "granite-3.3-2b-instruct-Q4_K_M.gguf"
	modeloSHA     = "ac71e9e32c0bea919b409c5918f69ca74339854b0319c5065e4e9fb6d95c4852"
	modeloTamanho = 1545303328

	// Espelho: os mesmos arquivos, nos Releases do próprio Nepenthes.
	Espelho    = "https://github.com/Nanofylium/probable-plant/releases/download/ia-1/"
	origMotor  = "https://github.com/ggml-org/llama.cpp/releases/download/" + Versao + "/"
	origModelo = "https://huggingface.co/ibm-granite/granite-3.3-2b-instruct-GGUF/resolve/main/"
)

type arquivo struct{ nome, sha string }

// motores: o pacote do llama.cpp de cada sistema (só CPU; no Mac usa a GPU pelo Metal).
var motores = map[string]arquivo{
	"linux/amd64":   {"llama-" + Versao + "-bin-ubuntu-x64.tar.gz", "a9b243a3bc4ca172010e96c64062fc02afd1dc4f8b699716f813ae6845ada4c6"},
	"linux/arm64":   {"llama-" + Versao + "-bin-ubuntu-arm64.tar.gz", "68fdee3e3086de8d1b6b3ef3bb791ff03ec4a6f2a8e7e199130fb70167d33d6d"},
	"darwin/arm64":  {"llama-" + Versao + "-bin-macos-arm64.tar.gz", "7d72477a2a375cc1d93e308b34e26fb9a608c2f93750517d2bafc59af578d3e2"},
	"darwin/amd64":  {"llama-" + Versao + "-bin-macos-x64.tar.gz", "27e4277bdbabcc9a4925cac77d99cd792abe0600c71bc33f265c43b8ea379d87"},
	"windows/amd64": {"llama-" + Versao + "-bin-win-cpu-x64.zip", "4ebc7fb961cf707e3b61396806e62c9e4065e42990cfc31f238eb48264a465b2"},
	"windows/arm64": {"llama-" + Versao + "-bin-win-cpu-arm64.zip", "ea2770a224400f69842135710a1c110d65bbfb7cd94584fdfd2f4caadb2b1c66"},
}

// Fontes de onde baixar (trocável nos testes).
var fontes = func(nome string) []string {
	orig := origMotor
	if strings.HasSuffix(nome, ".gguf") {
		orig = origModelo
	}
	return []string{Espelho + nome, orig + nome}
}

// Suportado diz se há motor para este sistema.
func Suportado() bool { _, ok := motores[runtime.GOOS+"/"+runtime.GOARCH]; return ok }

// Pasta onde motor e modelo ficam guardados.
func Pasta() string {
	if d := os.Getenv("NEP_IA_LOCAL"); d != "" {
		return d
	}
	d, err := os.UserCacheDir()
	if err != nil {
		d = os.TempDir()
	}
	return filepath.Join(d, "nepenthes", "ia")
}

// Estado é o que a tela mostra.
type Estado struct {
	Fase    string `json:"fase"` // ausente | baixando | pronto | ligando | ligado | erro
	Baixado int64  `json:"baixado"`
	Total   int64  `json:"total"`
	Erro    string `json:"erro,omitempty"`
	Modelo  string `json:"modelo"`
}

var (
	mu      sync.Mutex
	est     = Estado{Modelo: ModeloNome}
	proc    *exec.Cmd
	acabou  chan struct{} // fecha quando o llama-server para
	url     string
	ligando chan struct{}
)

func dirMotor() string      { return filepath.Join(Pasta(), "llama-"+Versao) }
func caminhoModelo() string { return filepath.Join(Pasta(), modeloArquivo) }

func exe() string {
	n := "llama-server"
	if runtime.GOOS == "windows" {
		n += ".exe"
	}
	return filepath.Join(dirMotor(), n)
}

func instalado() bool {
	if _, err := os.Stat(exe()); err != nil {
		return false
	}
	fi, err := os.Stat(caminhoModelo())
	return err == nil && fi.Size() == modeloTamanho
}

// Ver devolve o estado atual.
func Ver() Estado {
	mu.Lock()
	defer mu.Unlock()
	e := est
	if e.Fase == "" || e.Fase == "ausente" {
		e.Fase = "ausente"
		if instalado() {
			e.Fase = "pronto"
		}
	}
	if !Suportado() {
		e.Fase, e.Erro = "erro", "não há motor do llama.cpp para "+runtime.GOOS+"/"+runtime.GOARCH
	}
	return e
}

func muda(f func(*Estado)) { mu.Lock(); f(&est); mu.Unlock() }

// Baixar começa a baixar motor e modelo em segundo plano (uma vez só).
func Baixar() {
	if !Suportado() || instalado() {
		return
	}
	mu.Lock()
	if est.Fase == "baixando" {
		mu.Unlock()
		return
	}
	est.Fase, est.Erro = "baixando", ""
	mu.Unlock()
	go func() {
		err := BaixarAgora(nil)
		muda(func(e *Estado) {
			if err != nil {
				e.Fase, e.Erro = "erro", err.Error()
			} else {
				e.Fase = "pronto"
			}
		})
	}()
}

// BaixarAgora baixa e espera (para o comando `nep ia baixar`).
func BaixarAgora(progresso func(baixado, total int64)) error {
	m, ok := motores[runtime.GOOS+"/"+runtime.GOARCH]
	if !ok {
		return fmt.Errorf("não há motor do llama.cpp para %s/%s", runtime.GOOS, runtime.GOARCH)
	}
	if err := os.MkdirAll(Pasta(), 0o755); err != nil {
		return err
	}
	var feito int64
	total := int64(modeloTamanho) + 30<<20
	muda(func(e *Estado) { e.Fase, e.Total, e.Baixado = "baixando", total, 0 })
	conta := func(n int64) {
		feito += n
		muda(func(e *Estado) { e.Baixado = feito })
		if progresso != nil {
			progresso(feito, total)
		}
	}
	if _, err := os.Stat(exe()); err != nil {
		pac := filepath.Join(Pasta(), m.nome)
		if err := buscar(m, pac, conta); err != nil {
			return err
		}
		if err := extrair(pac, Pasta()); err != nil {
			return fmt.Errorf("não consegui abrir %s: %v", m.nome, err)
		}
		os.Remove(pac)
		if _, err := os.Stat(exe()); err != nil {
			return fmt.Errorf("o pacote %s não trouxe o llama-server", m.nome)
		}
	}
	if fi, err := os.Stat(caminhoModelo()); err != nil || fi.Size() != modeloTamanho {
		if err := buscar(arquivo{modeloArquivo, modeloSHA}, caminhoModelo(), conta); err != nil {
			return err
		}
	}
	return nil
}

// buscar baixa de cada fonte até uma dar certo, retomando um download
// interrompido (.parte) e conferindo o SHA-256.
func buscar(a arquivo, destino string, conta func(int64)) error {
	parte := destino + ".parte"
	var erros []string
	for _, u := range fontes(a.nome) {
		err := baixarDe(u, parte, conta)
		if err == nil {
			if err = conferir(parte, a.sha); err == nil {
				return os.Rename(parte, destino)
			}
			os.Remove(parte)
		}
		erros = append(erros, err.Error())
	}
	return fmt.Errorf("não consegui baixar %s (%s)", a.nome, strings.Join(erros, "; "))
}

var cliente = &http.Client{Timeout: 0}

func baixarDe(u, parte string, conta func(int64)) error {
	f, err := os.OpenFile(parte, os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	defer f.Close()
	ja, _ := f.Seek(0, io.SeekEnd)
	req, _ := http.NewRequest(http.MethodGet, u, nil)
	if ja > 0 {
		req.Header.Set("Range", fmt.Sprintf("bytes=%d-", ja))
	}
	res, err := cliente.Do(req)
	if err != nil {
		return fmt.Errorf("%s: %v", u, err)
	}
	defer res.Body.Close()
	switch {
	case res.StatusCode == http.StatusPartialContent:
		conta(ja)
	case res.StatusCode == http.StatusOK:
		if err := f.Truncate(0); err != nil {
			return err
		}
		f.Seek(0, io.SeekStart)
	case res.StatusCode == http.StatusRequestedRangeNotSatisfiable && ja > 0:
		conta(ja)
		return nil // já estava inteiro
	default:
		return fmt.Errorf("%s respondeu %d", u, res.StatusCode)
	}
	buf := make([]byte, 1<<20)
	for {
		n, err := res.Body.Read(buf)
		if n > 0 {
			if _, werr := f.Write(buf[:n]); werr != nil {
				return werr
			}
			conta(int64(n))
		}
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return fmt.Errorf("%s: %v", u, err)
		}
	}
}

func conferir(caminho, sha string) error {
	f, err := os.Open(caminho)
	if err != nil {
		return err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return err
	}
	if got := hex.EncodeToString(h.Sum(nil)); got != sha {
		return fmt.Errorf("%s veio diferente do esperado (SHA-256 %s)", filepath.Base(caminho), got[:12])
	}
	return nil
}

// extrair abre o .tar.gz (Linux/Mac, com links) ou o .zip (Windows) do
// llama.cpp em <pasta>/llama-<versão>/, sem deixar nada sair dessa pasta.
func extrair(pac, pasta string) error {
	alvo := dirMotor()
	os.RemoveAll(alvo)
	dentro := func(nome string) (string, error) {
		nome = filepath.FromSlash(nome)
		// o tar já vem com a pasta llama-<versão>/; o zip não
		if p := strings.SplitN(nome, string(filepath.Separator), 2); len(p) == 2 && strings.HasPrefix(p[0], "llama-") {
			nome = p[1]
		}
		d := filepath.Join(alvo, nome)
		if d != alvo && !strings.HasPrefix(d, alvo+string(filepath.Separator)) {
			return "", fmt.Errorf("caminho estranho no pacote: %s", nome)
		}
		return d, nil
	}
	escreve := func(d string, r io.Reader, modo os.FileMode) error {
		if err := os.MkdirAll(filepath.Dir(d), 0o755); err != nil {
			return err
		}
		f, err := os.OpenFile(d, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, modo|0o600)
		if err != nil {
			return err
		}
		if _, err := io.Copy(f, r); err != nil {
			f.Close()
			return err
		}
		return f.Close()
	}
	if strings.HasSuffix(pac, ".zip") {
		z, err := zip.OpenReader(pac)
		if err != nil {
			return err
		}
		defer z.Close()
		for _, f := range z.File {
			if f.FileInfo().IsDir() {
				continue
			}
			d, err := dentro(f.Name)
			if err != nil {
				return err
			}
			r, err := f.Open()
			if err != nil {
				return err
			}
			err = escreve(d, r, 0o755)
			r.Close()
			if err != nil {
				return err
			}
		}
		return nil
	}
	f, err := os.Open(pac)
	if err != nil {
		return err
	}
	defer f.Close()
	gz, err := gzip.NewReader(f)
	if err != nil {
		return err
	}
	tr := tar.NewReader(gz)
	for {
		h, err := tr.Next()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return err
		}
		d, err := dentro(h.Name)
		if err != nil {
			return err
		}
		switch h.Typeflag {
		case tar.TypeDir:
			os.MkdirAll(d, 0o755)
		case tar.TypeSymlink:
			// só links para arquivos da mesma pasta (libggml.so.1 -> libggml.so.1.0.0)
			if strings.ContainsAny(h.Linkname, `/\`) {
				return fmt.Errorf("link estranho no pacote: %s -> %s", h.Name, h.Linkname)
			}
			os.MkdirAll(filepath.Dir(d), 0o755)
			os.Remove(d)
			if err := os.Symlink(h.Linkname, d); err != nil {
				return err
			}
		case tar.TypeReg:
			if err := escreve(d, tr, os.FileMode(h.Mode)&0o755); err != nil {
				return err
			}
		}
	}
}

// URL liga o llama-server (se ainda não estiver ligado) e devolve o endereço
// da API de chat, como "http://127.0.0.1:41234/v1". Só escuta no próprio
// computador. Espera até o modelo carregar.
func URL(ctx context.Context) (string, error) {
	mu.Lock()
	if url != "" && vivo() {
		u := url
		mu.Unlock()
		return u, nil
	}
	if ligando != nil {
		ch := ligando
		mu.Unlock()
		select {
		case <-ch:
		case <-ctx.Done():
			return "", ctx.Err()
		}
		return URL(ctx)
	}
	if !instalado() {
		mu.Unlock()
		return "", errors.New("a IA do Nepenthes ainda não foi baixada")
	}
	ligando = make(chan struct{})
	est.Fase = "ligando"
	mu.Unlock()

	u, cmd, fim, err := ligar(ctx)
	mu.Lock()
	close(ligando)
	ligando = nil
	if err != nil {
		est.Fase, est.Erro = "erro", err.Error()
	} else {
		proc, acabou, url, est.Fase, est.Erro = cmd, fim, u, "ligado", ""
	}
	mu.Unlock()
	return u, err
}

func vivo() bool {
	if acabou == nil {
		return false
	}
	select {
	case <-acabou:
		return false
	default:
		return true
	}
}

func ligar(ctx context.Context) (string, *exec.Cmd, chan struct{}, error) {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return "", nil, nil, err
	}
	porta := l.Addr().(*net.TCPAddr).Port
	l.Close()
	log, _ := os.Create(filepath.Join(Pasta(), "llama-server.log"))
	cmd := exec.Command(exe(), "-m", caminhoModelo(), "--host", "127.0.0.1", "--port", fmt.Sprint(porta),
		"-c", "8192", "--parallel", "1", "--no-webui")
	cmd.Dir = dirMotor()
	cmd.Stdout, cmd.Stderr = log, log
	if err := cmd.Start(); err != nil {
		return "", nil, nil, fmt.Errorf("não consegui ligar o llama-server: %v", err)
	}
	fim := make(chan struct{})
	go func() { cmd.Wait(); close(fim); log.Close() }()
	base := fmt.Sprintf("http://127.0.0.1:%d", porta)
	limite := time.After(3 * time.Minute)
	for {
		select {
		case <-fim:
			return "", nil, nil, fmt.Errorf("o llama-server parou ao ligar (veja %s)", filepath.Join(Pasta(), "llama-server.log"))
		case <-limite:
			cmd.Process.Kill()
			return "", nil, nil, errors.New("o llama-server demorou demais para carregar o modelo")
		case <-ctx.Done():
			cmd.Process.Kill()
			return "", nil, nil, ctx.Err()
		case <-time.After(300 * time.Millisecond):
		}
		if r, err := http.Get(base + "/health"); err == nil {
			r.Body.Close()
			if r.StatusCode == 200 {
				return base + "/v1", cmd, fim, nil
			}
		}
	}
}

// Parar desliga o llama-server (o nep chama ao sair).
func Parar() {
	mu.Lock()
	defer mu.Unlock()
	if proc != nil && vivo() {
		proc.Process.Kill()
	}
	proc, acabou, url = nil, nil, ""
	if est.Fase == "ligado" {
		est.Fase = "pronto"
	}
}

// Buscar baixa um arquivo (de cada endereço, até um dar certo), retomando um
// download interrompido e conferindo o SHA-256. Usado também pelo modelo de
// profundidade do passeio 3D.
func Buscar(enderecos []string, sha, destino string, conta func(int64)) error {
	if err := os.MkdirAll(filepath.Dir(destino), 0o755); err != nil {
		return err
	}
	parte := destino + ".parte"
	var erros []string
	for _, u := range enderecos {
		err := baixarDe(u, parte, conta)
		if err == nil {
			if err = conferir(parte, sha); err == nil {
				return os.Rename(parte, destino)
			}
			os.Remove(parte)
		}
		erros = append(erros, err.Error())
	}
	return fmt.Errorf("não consegui baixar %s (%s)", filepath.Base(destino), strings.Join(erros, "; "))
}
