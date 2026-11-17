package main

import (
	"fmt"
	"net/http"
	"os/exec"
	"regexp"
	"runtime"
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
			if r, err := http.Get("http://127.0.0.1:8000/_nep/assistente/estado"); err == nil {
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
