package web

// Passeio vivo: {{ mundo("loja") }} mostra o passeio que `nep mundo` gerou em
// mundos/loja/ (quadros do LingBot-World ou simulados). O visitante anda
// segurando ↑ (ou o botão, ou rolando a página) e escolhe o caminho com ← e →.
// Sem JavaScript, fica a foto inicial.

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"html"
	"path/filepath"
	"strings"

	"github.com/Nanofylium/probable-plant/lang"
	"github.com/Nanofylium/probable-plant/mundo"
)

//go:embed assets/mundo.js
var mundoJS []byte

func (s *Server) mundoBuiltin(a []lang.Value) (lang.Value, error) {
	nome := strings.TrimSpace(strOrEmpty(argv(a, 0)))
	op, _ := argv(a, 1).(*lang.Table)
	opc := func(k string) lang.Value {
		if op == nil {
			return nil
		}
		v, _ := op.Get(k)
		return v
	}
	m, err := mundo.Abrir(filepath.Join(s.dir, "mundos"), nome)
	if err != nil {
		return nil, fmt.Errorf("mundo(%q): não achei mundos/%s/mundo.json; gere com: nep mundo foto.jpg \"descrição\" --nome %s", nome, nome, nome)
	}
	if m.Tipo != "3d" && len(m.Caminhos) == 0 {
		return nil, fmt.Errorf("mundo(%q): o passeio não tem caminhos", nome)
	}
	titulo := strOrEmpty(opc("titulo"))
	if titulo == "" {
		titulo = m.Titulo
	}
	rolar := false
	if b, ok := opc("rolar").(bool); ok {
		rolar = b
	}
	base := s.opt.Base + "/mundos/" + m.Nome + "/"
	dados := map[string]any{"base": base, "fps": m.FPS, "largura": m.Largura, "altura": m.Altura, "fonte": m.Fonte, "caminhos": m.Caminhos, "rolar": rolar}
	primeira := ""
	if m.Tipo == "3d" {
		dados["tipo"], dados["foto"], dados["profundidade"] = "3d", m.Foto, m.Profundidade
		primeira = base + m.Foto
	} else {
		c0 := m.Caminhos[0]
		primeira = fmt.Sprintf("%s%s/001.%s", base, c0.ID, c0.Ext)
	}
	j, _ := json.Marshal(dados)
	var b strings.Builder
	b.WriteString(`<figure class="nep-mundo" data-mundo="` + html.EscapeString(string(j)) + `">`)
	b.WriteString(fmt.Sprintf(`<img src="%s" width="%d" height="%d" alt="%s" loading="lazy">`,
		html.EscapeString(primeira), m.Largura, m.Altura, html.EscapeString(titulo)))
	if titulo != "" || opc("texto") != nil {
		b.WriteString(`<figcaption>`)
		if titulo != "" {
			b.WriteString(`<strong>` + html.EscapeString(titulo) + `</strong>`)
		}
		if t := strOrEmpty(opc("texto")); t != "" {
			b.WriteString(` <span>` + html.EscapeString(t) + `</span>`)
		}
		b.WriteString(`</figcaption>`)
	}
	b.WriteString(`</figure><script src="` + html.EscapeString(s.opt.Base) + `/_nep/mundo.js" defer></script>`)
	return lang.HTML(b.String()), nil
}
