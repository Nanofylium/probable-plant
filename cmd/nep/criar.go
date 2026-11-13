package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/Nanofylium/probable-plant/gerador"
	"github.com/Nanofylium/probable-plant/web"
)

// criar monta um site completo a partir de uma descrição em português:
//
//	nep criar "pousada em Paraty chamada Maré Mansa, com blog e galeria, azul"
//	nep criar "só loja e contato, chamada Ateliê Ipê" --pasta ateliê
//
// Escreve pasta/site.nep.html e pasta/site-previa.html e conta o que entendeu.
func criar(args []string) int {
	var partes []string
	dir, verPlano, forcar := "", false, false
	for i := 0; i < len(args); i++ {
		a := args[i]
		switch {
		case a == "--pasta" || a == "-p":
			if i+1 < len(args) {
				dir = args[i+1]
				i++
			}
		case strings.HasPrefix(a, "--pasta="):
			dir = strings.TrimPrefix(a, "--pasta=")
		case a == "--plano" || a == "--so-plano":
			verPlano = true
		case a == "--forcar" || a == "--force":
			forcar = true
		default:
			partes = append(partes, a)
		}
	}
	desc := strings.TrimSpace(strings.Join(partes, " "))
	if desc == "" {
		fmt.Fprintln(os.Stderr, `descreva o site: nep criar "clínica veterinária chamada Bem-te-vi em Campinas, com agenda e blog, verde"`)
		return 2
	}
	p := gerador.Entender(desc)
	fmt.Println("Entendi assim:")
	for _, n := range p.Notas {
		fmt.Println("  ·", n)
	}
	fmt.Println("  · seções:")
	for _, m := range p.Modulos {
		marca := " "
		for _, x := range p.Pedidos {
			if x == m {
				marca = "*"
			}
		}
		fmt.Printf("     %s %s\n", marca, gerador.NomeModulo[m])
	}
	for _, m := range p.Tirados {
		fmt.Printf("     - sem %s (você pediu para tirar)\n", gerador.NomeModulo[m])
	}
	fmt.Println("     + contato, páginas (Sobre, Privacidade), busca, SEO, RSS e painel")
	if len(p.Pedidos) > 0 {
		fmt.Println("    (* = você pediu; o resto é o que todo site desse ramo costuma ter)")
	}
	if verPlano {
		return 0
	}
	if dir == "" {
		dir = gerador.Pasta(p)
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	f := filepath.Join(dir, "site.nep.html")
	if _, err := os.Stat(f); err == nil && !forcar {
		fmt.Fprintln(os.Stderr, f, "já existe; nada foi alterado (use --forcar para trocar)")
		return 1
	}
	src := gerador.Montar(p)
	if err := os.WriteFile(f, []byte(src), 0o644); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	if _, err := web.Check(f); err != nil {
		fmt.Fprintln(os.Stderr, "o site gerado tem um erro (avise quem mantém o Nepenthes):", err)
		return 1
	}
	prev := filepath.Join(dir, "site-previa.html")
	html, n, err := gerarPrevia(f)
	if err == nil {
		err = os.WriteFile(prev, []byte(html), 0o644)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "prévia não saiu:", err)
	}
	fmt.Printf("\nPronto: %s\n", f)
	if err == nil {
		fmt.Printf("Prévia com %d páginas: %s (abre com dois cliques)\n", n, prev)
	}
	fmt.Printf("Rode:   nep dev %s   e abra http://localhost:8000  (painel em /_admin)\n", f)
	return 0
}
