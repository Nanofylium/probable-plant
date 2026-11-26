package web

import (
	"strings"
	"testing"
)

func TestPixCopiaECola(t *testing.T) {
	// exemplo do manual do BR Code (Banco Central), chave aleatória e valor
	got := PixCopiaECola("123e4567-e12b-12d1-a456-426655440000", "Fulano de Tal", "BRASILIA", 0, "***")
	want := "00020126580014br.gov.bcb.pix0136123e4567-e12b-12d1-a456-42665544000052040000530398658" +
		"02BR5913Fulano de Tal6008BRASILIA62070503***6304"
	if !strings.HasPrefix(got, want) {
		t.Fatalf("\n%s\n%s", got, want)
	}
	if got != want+"1D3D" {
		t.Fatalf("CRC: %s", got[len(got)-4:])
	}
	v := PixCopiaECola("a@b.com", "Associação São João", "São Félix do Xingu", 30, "M-12/AB")
	for _, p := range []string{"540530.00", "5919Associacao Sao Joao", "6015Sao Felix do Xi", "0505M12AB"} {
		if !strings.Contains(v, p) {
			t.Errorf("faltou %q em %s", p, v)
		}
	}
	svg, err := QRCodeSVG(v)
	if err != nil || !strings.HasPrefix(svg, "<svg") {
		t.Fatal(err)
	}
}
