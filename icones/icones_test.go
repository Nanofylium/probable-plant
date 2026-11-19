package icones

import (
	"strings"
	"testing"
)

func TestSVG(t *testing.T) {
	for _, e := range []string{"🛍️", "🛍", "✂️", "☕", "croissant", "👨‍👩‍👧"} {
		if s, ok := SVG(e); !ok || !strings.HasPrefix(s, "<svg") || !strings.Contains(s, "<path") && !strings.Contains(s, "<circle") {
			t.Errorf("%q: %v %q", e, ok, s)
		}
	}
	if HTML("🦄") != "🦄" || HTML("<b>") != "&lt;b&gt;" {
		t.Error("sem ícone devia voltar o texto escapado")
	}
	for e, n := range porEmoji {
		if desenhos[n] == "" {
			t.Errorf("%s aponta para %s, que não existe", e, n)
		}
	}
}
