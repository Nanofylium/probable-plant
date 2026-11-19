package main

import (
	"bufio"
	_ "embed"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"time"
)

var (
	reTagNepenthes = regexp.MustCompile(`(?i)<nepenthes[\s>]`)
	// "nep serve index.html", "./nep ...", ".\nep.exe ...", "nep-windows.exe ..."
	reComandoNep = regexp.MustCompile(`^(?:\.[\\/])?nep[\w-]*(?:\.exe)?\s+((?:serve|servir|dev|criar|novo|previa|ia|check|run|test|version|ajuda)\b.*)$`)
)

// abrirAssistente roda o site e abre o navegador nele (dois cliques no nep).
func abrirAssistente(arquivo string) int {
	fmt.Printf("Achei <nepenthes> em %s: abrindo a tela do Nepenthes no navegador.\n", arquivo)
	fmt.Println("(Para o modo de programar, rode: nep repl)")
	// abre o navegador só quando o servidor já responde (no Windows, o
	// antivírus pode segurar o primeiro arranque por vários segundos)
	go func() {
		for i := 0; i < 240; i++ {
			time.Sleep(250 * time.Millisecond)
			if r, err := http.Get("http://127.0.0.1:8000/"); err == nil {
				r.Body.Close()
				abrirNavegador("http://localhost:8000/")
				return
			}
		}
	}()
	return serve([]string{arquivo}, false)
}

func abrirNavegador(url string) {
	var c *exec.Cmd
	switch runtime.GOOS {
	case "windows":
		c = exec.Command("rundll32", "url.dll,FileProtocolHandler", url)
	case "darwin":
		c = exec.Command("open", url)
	default:
		c = exec.Command("xdg-open", url)
	}
	_ = c.Start()
}

//go:embed inicio.html
var inicioHTML []byte

// modoClique: dois cliques no nep (Windows), sem index.html por perto. Cria a
// pasta do site com um index.html que tem <nepenthes>, liga o servidor e abre
// o navegador. Se o nep já estiver rodando, só abre o navegador.
func modoClique() int {
	if r, err := http.Get("http://127.0.0.1:8000/"); err == nil {
		r.Body.Close()
		fmt.Println("O Nepenthes já está rodando: abrindo o navegador.")
		abrirNavegador("http://localhost:8000/")
		time.Sleep(2 * time.Second)
		return 0
	}
	pasta := pastaDoSite()
	if err := os.MkdirAll(pasta, 0o755); err != nil {
		return esperaErro(err)
	}
	site := filepath.Join(pasta, "index.html")
	if _, err := os.Stat(site); err != nil {
		if err := os.WriteFile(site, inicioHTML, 0o644); err != nil {
			return esperaErro(err)
		}
	}
	if err := os.Chdir(pasta); err != nil {
		return esperaErro(err)
	}
	fmt.Println("Seu site fica nesta pasta:", pasta)
	fmt.Println("Deixe esta janela aberta enquanto usa. Para parar, feche-a.")
	fmt.Println()
	if c := abrirAssistente("index.html"); c != 0 {
		return esperaErro(fmt.Errorf("o Nepenthes parou (código %d)", c))
	}
	return 0
}

// pastaDoSite: ao lado do executável; se ele rodou de dentro de um zip (pasta
// temporária) ou de uma pasta sem permissão, em Documentos\Nepenthes.
func pastaDoSite() string {
	if exe, err := os.Executable(); err == nil {
		d := filepath.Dir(exe)
		tmp := strings.ToLower(os.TempDir())
		if !strings.HasPrefix(strings.ToLower(d), tmp) && !strings.Contains(strings.ToLower(d), `\temp\`) && podeEscrever(d) {
			return filepath.Join(d, "Meu site")
		}
	}
	home, err := os.UserHomeDir()
	if err != nil {
		home = "."
	}
	return filepath.Join(home, "Documents", "Nepenthes")
}

func podeEscrever(d string) bool {
	f, err := os.CreateTemp(d, ".nep-teste-*")
	if err != nil {
		return false
	}
	f.Close()
	os.Remove(f.Name())
	return true
}

// esperaErro mostra o erro e espera um Enter, para a janela não sumir.
func esperaErro(err error) int {
	fmt.Println()
	fmt.Println("Erro:", err)
	fmt.Println("Aperte Enter para fechar.")
	bufio.NewReader(os.Stdin).ReadString('\n')
	return 1
}
