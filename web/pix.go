package web

// Pix "copia e cola" (BR Code do Banco Central, padrão EMV) e QR Code, gerados
// aqui mesmo, sem serviço externo:
//
//	{{ qrcode(pix(chave, "Associação X", "Aveiro", 30, "M12AB")) }}
//
// O código é o Pix estático com valor: qualquer app de banco paga. A baixa
// (marcar como pago) fica no painel, ou num webhook do provedor da conta.

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"html"
	"strconv"
	"strings"
	"unicode"

	"github.com/Nanofylium/probable-plant/lang"
	"rsc.io/qr"
)

func campoEMV(id, v string) string { return fmt.Sprintf("%s%02d%s", id, len(v), v) }

// semAcentoASCII: o BR Code pede nome e cidade sem acento.
func semAcentoASCII(s string, max int) string {
	var b strings.Builder
	s = strings.NewReplacer("á", "a", "à", "a", "â", "a", "ã", "a", "ä", "a", "é", "e", "ê", "e", "è", "e", "í", "i", "ì", "i", "ó", "o", "ô", "o", "õ", "o", "ò", "o", "ú", "u", "ü", "u", "ç", "c",
		"Á", "A", "À", "A", "Â", "A", "Ã", "A", "É", "E", "Ê", "E", "Í", "I", "Ó", "O", "Ô", "O", "Õ", "O", "Ú", "U", "Ç", "C").Replace(s)
	for _, r := range s {
		if r < 32 || r > 126 {
			continue
		}
		b.WriteRune(r)
	}
	out := strings.TrimSpace(b.String())
	if len(out) > max {
		out = strings.TrimSpace(out[:max])
	}
	return out
}

// PixCopiaECola monta o BR Code. valor <= 0 = a pessoa digita o valor no app.
func PixCopiaECola(chave, nome, cidade string, valor float64, txid string) string {
	chave = strings.TrimSpace(chave)
	conta := campoEMV("00", "br.gov.bcb.pix") + campoEMV("01", chave)
	var b strings.Builder
	b.WriteString(campoEMV("00", "01"))
	b.WriteString(campoEMV("26", conta))
	b.WriteString(campoEMV("52", "0000"))
	b.WriteString(campoEMV("53", "986"))
	if valor > 0 {
		b.WriteString(campoEMV("54", fmt.Sprintf("%.2f", valor)))
	}
	b.WriteString(campoEMV("58", "BR"))
	n := semAcentoASCII(nome, 25)
	if n == "" {
		n = "RECEBEDOR"
	}
	c := semAcentoASCII(cidade, 15)
	if c == "" {
		c = "BRASIL"
	}
	b.WriteString(campoEMV("59", n))
	b.WriteString(campoEMV("60", c))
	t := ""
	for _, r := range txid {
		if r < 128 && (unicode.IsLetter(r) || unicode.IsDigit(r)) {
			t += string(r)
		}
	}
	if len(t) > 25 {
		t = t[:25]
	}
	if t == "" {
		t = "***"
	}
	b.WriteString(campoEMV("62", campoEMV("05", t)))
	b.WriteString("6304")
	return b.String() + fmt.Sprintf("%04X", crc16CCITT(b.String()))
}

func crc16CCITT(s string) uint16 {
	crc := uint16(0xFFFF)
	for i := 0; i < len(s); i++ {
		crc ^= uint16(s[i]) << 8
		for j := 0; j < 8; j++ {
			if crc&0x8000 != 0 {
				crc = crc<<1 ^ 0x1021
			} else {
				crc <<= 1
			}
		}
	}
	return crc
}

// QRCodeSVG desenha o QR Code em SVG (nítido em qualquer tamanho).
func QRCodeSVG(texto string) (string, error) {
	c, err := qr.Encode(texto, qr.M)
	if err != nil {
		return "", err
	}
	n := c.Size
	var b strings.Builder
	fmt.Fprintf(&b, `<svg class="nep-qr" xmlns="http://www.w3.org/2000/svg" viewBox="-4 -4 %d %d" shape-rendering="crispEdges" role="img" aria-label="QR Code"><rect x="-4" y="-4" width="%d" height="%d" fill="#fff"/><path fill="#000" d="`, n+8, n+8, n+8, n+8)
	for y := 0; y < n; y++ {
		for x := 0; x < n; x++ {
			if c.Black(x, y) {
				fmt.Fprintf(&b, "M%d %dh1v1h-1z", x, y)
			}
		}
	}
	b.WriteString(`"/></svg>`)
	return b.String(), nil
}

func (s *Server) pixBuiltins(def func(string, func([]lang.Value) (lang.Value, error))) {
	def("pix", func(a []lang.Value) (lang.Value, error) {
		v, _ := toFloat(argv(a, 3))
		return PixCopiaECola(strOrEmpty(argv(a, 0)), strOrEmpty(argv(a, 1)), strOrEmpty(argv(a, 2)), v, strOrEmpty(argv(a, 4))), nil
	})
	def("qrcode", func(a []lang.Value) (lang.Value, error) {
		svg, err := QRCodeSVG(strOrEmpty(argv(a, 0)))
		if err != nil {
			return lang.HTML(`<span class="nep-erros">` + html.EscapeString(err.Error()) + `</span>`), nil
		}
		return lang.HTML(svg), nil
	})
	// token(n): código aleatório (link privado, número de carteirinha...)
	def("token", func(a []lang.Value) (lang.Value, error) {
		n := 8
		if x, ok := toInt(argv(a, 0)); ok && x >= 4 && x <= 32 {
			n = int(x)
		}
		b := make([]byte, (n+1)/2)
		if _, err := rand.Read(b); err != nil {
			return nil, err
		}
		return strings.ToUpper(hex.EncodeToString(b))[:n], nil
	})
}

func toFloat(v lang.Value) (float64, bool) {
	switch x := v.(type) {
	case int64:
		return float64(x), true
	case float64:
		return x, true
	case string:
		f, err := strconv.ParseFloat(strings.ReplaceAll(strings.TrimSpace(x), ",", "."), 64)
		return f, err == nil
	}
	return 0, false
}
