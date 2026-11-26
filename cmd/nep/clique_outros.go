//go:build !windows

package main

import (
	"os"
	"os/exec"
	"os/signal"
	"syscall"
)

// cliqueDuplo: fora do Windows, o nep sem argumentos abre o modo interativo.
func cliqueDuplo() bool { return false }

func escondeJanela(c *exec.Cmd) {}

// fora do Windows não há ícone na bandeja: a Central roda até Ctrl+C
func bandeja(c *central, fechar func()) {
	sinal := make(chan os.Signal, 1)
	signal.Notify(sinal, os.Interrupt, syscall.SIGTERM)
	<-sinal
	fechar()
}

func avisoErro(msg string) {}

func iniciarComSistema() (bool, error) { return false, nil }

func mudarIniciarComSistema(bool) error { return nil }

func criarAtalho() error { return nil }
