package main

import (
	"embed"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// Modelos de site do "nep novo". Todos usam o tema Nepenthes e saem com cara
// profissional; o blog é o mesmo de exemplos/ascidia.
//
//go:embed modelos/*.nep.html
var modelosFS embed.FS

func modelos() []string {
	ents, _ := modelosFS.ReadDir("modelos")
	var n []string
	for _, e := range ents {
		n = append(n, strings.TrimSuffix(e.Name(), ".nep.html"))
	}
	sort.Strings(n)
	return n
}

// novo cria uma pasta com um site inicial: nep novo meu-site [--modelo blog]
func novo(args []string) int {
	dir, modelo := "", "empresa"
	for i := 0; i < len(args); i++ {
		a := args[i]
		switch {
		case a == "--modelo" || a == "-modelo" || a == "-m":
			if i+1 < len(args) {
				modelo = args[i+1]
				i++
			}
		case strings.HasPrefix(a, "--modelo="):
			modelo = strings.TrimPrefix(a, "--modelo=")
		case dir == "":
			dir = a
		}
	}
	if dir == "" {
		fmt.Fprintln(os.Stderr, "informe a pasta: nep novo meu-site [--modelo "+strings.Join(modelos(), "|")+"]")
		return 2
	}
	src, err := modelosFS.ReadFile("modelos/" + modelo + ".nep.html")
	if err != nil {
		fmt.Fprintf(os.Stderr, "modelo %q não existe; use um destes: %s\n", modelo, strings.Join(modelos(), ", "))
		return 2
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	f := filepath.Join(dir, "site.nep.html")
	if _, err := os.Stat(f); err == nil {
		fmt.Fprintln(os.Stderr, f, "já existe; nada foi alterado")
		return 1
	}
	if err := os.WriteFile(f, src, 0o644); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	fmt.Printf("Site (modelo %s) criado em %s\nPara ver: nep dev %s\nOutros modelos: %s\n", modelo, f, f, strings.Join(modelos(), ", "))
	return 0
}
