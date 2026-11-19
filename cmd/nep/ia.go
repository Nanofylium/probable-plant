package main

import (
	"fmt"
	"os"

	"github.com/Nanofylium/probable-plant/ialocal"
)

// nep ia [baixar]: a IA do Nepenthes (llama.cpp + Granite), que a tela da tag
// <nepenthes> usa para escrever sites sem GPU e sem chave.
func ia(args []string) int {
	e := ialocal.Ver()
	if len(args) == 0 {
		fmt.Printf("IA do Nepenthes: %s com o motor llama.cpp %s\n", ialocal.ModeloNome, ialocal.Versao)
		fmt.Printf("Pasta: %s\n", ialocal.Pasta())
		switch e.Fase {
		case "pronto":
			fmt.Println("Estado: baixada e pronta.")
		case "erro":
			fmt.Println("Estado: " + e.Erro)
			return 1
		default:
			fmt.Println("Estado: ainda não baixada. Rode: nep ia baixar")
		}
		return 0
	}
	if args[0] != "baixar" && args[0] != "download" {
		fmt.Fprintln(os.Stderr, "uso: nep ia [baixar]")
		return 2
	}
	if e.Fase == "pronto" {
		fmt.Println("A IA do Nepenthes já está baixada em", ialocal.Pasta())
		return 0
	}
	fmt.Printf("Baixando %s e o llama.cpp para %s (só desta vez)…\n", ialocal.ModeloNome, ialocal.Pasta())
	ultimo := -1
	err := ialocal.BaixarAgora(func(feito, total int64) {
		if p := int(feito * 100 / total); p != ultimo {
			ultimo = p
			fmt.Printf("\r  %3d%%  %d de %d MB", p, feito>>20, total>>20)
		}
	})
	fmt.Println()
	if err != nil {
		fmt.Fprintln(os.Stderr, "erro:", err)
		return 1
	}
	fmt.Println("Pronto. Escreva <nepenthes>sua ideia</nepenthes> num .html e rode nep serve nele.")
	return 0
}
