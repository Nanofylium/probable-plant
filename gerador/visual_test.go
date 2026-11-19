package gerador

import (
	"strings"
	"testing"
)

func TestVisualPedidoESorteado(t *testing.T) {
	casos := map[string]string{
		"Barbearia moderna com visual escuro":                "noturno",
		"Pousada de luxo em Trancoso":                        "luxo",
		"Escritório de contabilidade, site minimalista":      "minimo",
		"Loja de roupas urbana e ousada":                     "brutal",
		"Escolinha infantil alegre":                          "suave",
		"Padaria tradicional, jeito de revista":              "editorial",
		"Academia de crossfit com visual impactante e forte": "vitrine",
	}
	for desc, quer := range casos {
		if p := Entender(desc); p.Visual != quer || !p.VisualPedido {
			t.Errorf("%q: visual %q (pedido %v), esperava %q", desc, p.Visual, p.VisualPedido, quer)
		}
	}
	// sem pedido: um dos que combinam com o ramo, igual para a mesma descrição
	a, b := Entender("Clínica veterinária em Recife"), Entender("Clínica veterinária em Recife")
	if a.Visual != b.Visual || a.VisualPedido {
		t.Fatalf("sorteio sem semente devia repetir: %q %q", a.Visual, b.Visual)
	}
	ok := false
	for _, v := range visuaisDoRamo["veterinaria"] {
		ok = ok || v == a.Visual
	}
	if !ok {
		t.Fatalf("%q não combina com veterinária", a.Visual)
	}
	// com sementes diferentes, aparecem visuais diferentes
	vistos := map[string]bool{}
	for i := int64(1); i <= 40; i++ {
		vistos[EntenderComSemente("Restaurante em Olinda", i).Visual] = true
	}
	if len(vistos) < 3 {
		t.Fatalf("sorteio pouco variado: %v", vistos)
	}
}

func TestMontarComVisualEIcones(t *testing.T) {
	p := Entender("Cafeteria aconchegante em Natal com cardápio, site escuro")
	src := Montar(p)
	if !strings.Contains(src, `visual="noturno"`) {
		t.Fatal("nep-config sem o visual")
	}
	if strings.Contains(src, "«ic:") || !strings.Contains(src, `class="nep-ic"`) {
		t.Fatal("ícones não foram trocados")
	}
	if !strings.Contains(src, "{{ icone(default(") {
		t.Fatal("ícones dos itens deviam passar por icone()")
	}
	if !TrocarVisual(p, "luxo") || TrocarVisual(p, "inexistente") || !strings.Contains(Montar(p), `visual="luxo"`) {
		t.Fatal("TrocarVisual")
	}
}
