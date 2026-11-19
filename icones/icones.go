// Package icones troca emojis por ícones de traço do Lucide (https://lucide.dev,
// licença ISC, ver LICENSE-lucide.txt), que dão cara profissional aos sites.
// Emoji sem ícone correspondente continua emoji.
package icones

import (
	"html"
	"strings"
)

// SVG devolve o ícone (SVG em linha) para um emoji ou nome do Lucide, e ok=false
// se não houver.
func SVG(x string) (string, bool) {
	x = strings.TrimSpace(strings.ReplaceAll(x, "️", ""))
	nome := porEmoji[x]
	if nome == "" {
		nome = x
	}
	d, ok := desenhos[nome]
	if !ok {
		return "", false
	}
	return `<svg class="nep-ic" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.75" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true">` + d + `</svg>`, true
}

// HTML devolve o ícone ou, se não houver, o próprio texto escapado.
func HTML(x string) string {
	if s, ok := SVG(x); ok {
		return s
	}
	return html.EscapeString(x)
}

// Tem diz se há ícone para o emoji ou nome.
func Tem(x string) bool { _, ok := SVG(x); return ok }
