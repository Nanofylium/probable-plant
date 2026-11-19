package web

// Gráficos com o Apache ECharts (criado no Baidu, licença Apache 2.0; ver
// LICENCA-ECHARTS.txt). Na página: {{ grafico("barras", dados, {titulo: "..."}) }}.
// O nep serve o ECharts (/_nep/echarts.js) e um script pequeno (/_nep/grafico.js)
// que desenha cada gráfico com a cor e a fonte do site. Sem JavaScript (ou na
// prévia), aparece a tabela com os mesmos números.

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"html"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/Nanofylium/probable-plant/lang"
)

//go:embed assets/echarts.min.js
var echartsJS []byte

//go:embed assets/grafico.js
var graficoJS []byte

// Grafico é o que o script recebe (data-grafico).
type Grafico struct {
	Tipo    string       `json:"tipo"`
	Titulo  string       `json:"-"`
	Rotulos []string     `json:"rotulos"`
	Series  []SerieDados `json:"series"`
	Moeda   bool         `json:"moeda,omitempty"`
	Altura  int          `json:"altura"`
	Cores   []string     `json:"cores,omitempty"`
}

// SerieDados é uma série de valores com nome.
type SerieDados struct {
	Nome    string    `json:"nome"`
	Valores []float64 `json:"valores"`
}

var tiposGrafico = map[string]string{
	"barras": "barras", "barra": "barras", "colunas": "barras", "coluna": "barras",
	"horizontal": "horizontal", "barras-deitadas": "horizontal", "deitado": "horizontal",
	"linhas": "linhas", "linha": "linhas",
	"area": "area", "área": "area",
	"pizza": "pizza", "torta": "pizza",
	"rosca": "rosca", "donut": "rosca",
}

func serveAssetJS(w http.ResponseWriter, r *http.Request, b []byte, imutavel bool) {
	w.Header().Set("Content-Type", "text/javascript; charset=utf-8")
	if imutavel {
		w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
	} else {
		w.Header().Set("Cache-Control", "public, max-age=86400")
	}
	http.ServeContent(w, r, "", time.Time{}, strings.NewReader(string(b)))
}

func numero(v lang.Value) (float64, bool) {
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

// montarGrafico lê grafico(tipo, dados, opções).
func montarGrafico(tipo string, dados lang.Value, op *lang.Table) (*Grafico, error) {
	t, ok := tiposGrafico[strings.ToLower(strings.TrimSpace(tipo))]
	if !ok {
		return nil, fmt.Errorf("grafico: tipo %q não existe; use barras, horizontal, linhas, area, pizza ou rosca", tipo)
	}
	g := &Grafico{Tipo: t, Altura: 320}
	opc := func(k string) lang.Value {
		if op == nil {
			return nil
		}
		v, _ := op.Get(k)
		return v
	}
	g.Titulo = strOrEmpty(opc("titulo"))
	if b, ok := opc("moeda").(bool); ok {
		g.Moeda = b
	}
	if n, ok := toInt(opc("altura")); ok {
		g.Altura = int(min(max(n, 160), 900))
	}
	nomeUnico := strOrEmpty(opc("nome"))
	const maxPontos = 500

	switch d := dados.(type) {
	case *lang.Table: // {"Jan": 10, "Fev": 12}
		s := SerieDados{Nome: nomeUnico}
		for i := 0; i < d.Len() && i < maxPontos; i++ {
			k, v := d.Entry(i)
			f, ok := numero(v)
			if !ok {
				continue
			}
			g.Rotulos = append(g.Rotulos, lang.Str(k))
			s.Valores = append(s.Valores, f)
		}
		g.Series = []SerieDados{s}
	case *lang.List:
		itens := d.Items
		if len(itens) > maxPontos {
			itens = itens[:maxPontos]
		}
		if len(itens) > 0 {
			if _, ok := itens[0].(*lang.Table); !ok { // [3, 5, 8]
				s := SerieDados{Nome: nomeUnico}
				for i, v := range itens {
					if f, ok := numero(v); ok {
						g.Rotulos = append(g.Rotulos, strconv.Itoa(i+1))
						s.Valores = append(s.Valores, f)
					}
				}
				g.Series = []SerieDados{s}
				break
			}
		}
		// lista de registros: x = campo do rótulo, y = campo(s) dos valores
		x := strOrEmpty(opc("x"))
		var ys []string
		switch y := opc("y").(type) {
		case string:
			ys = []string{y}
		case *lang.List:
			for _, it := range y.Items {
				ys = append(ys, lang.Str(it))
			}
		}
		if len(itens) > 0 {
			primeiro := itens[0].(*lang.Table)
			for i := 0; i < primeiro.Len(); i++ {
				k, v := primeiro.Entry(i)
				nome := lang.Str(k)
				_, eNum := numero(v)
				if _, eTexto := v.(string); eTexto {
					eNum = false
				}
				if x == "" && !eNum {
					x = nome
				} else if opc("y") == nil && eNum && nome != "id" {
					ys = append(ys, nome)
				}
			}
		}
		if len(ys) == 0 {
			return nil, fmt.Errorf("grafico: não achei campo com números; diga qual com {y: \"campo\"}")
		}
		g.Series = make([]SerieDados, len(ys))
		for j, y := range ys {
			g.Series[j].Nome = y
			if len(ys) == 1 && nomeUnico != "" {
				g.Series[j].Nome = nomeUnico
			}
		}
		for _, it := range itens {
			r, ok := it.(*lang.Table)
			if !ok {
				continue
			}
			rot, _ := r.Get(x)
			g.Rotulos = append(g.Rotulos, strOrEmpty(rot))
			for j, y := range ys {
				v, _ := r.Get(y)
				f, _ := numero(v)
				g.Series[j].Valores = append(g.Series[j].Valores, f)
			}
		}
	case nil:
		g.Series = []SerieDados{{Nome: nomeUnico}}
	default:
		return nil, fmt.Errorf("grafico: os dados precisam ser uma tabela {rótulo: valor} ou uma lista")
	}
	if (t == "pizza" || t == "rosca") && len(g.Series) > 1 {
		g.Series = g.Series[:1]
	}
	return g, nil
}

// HTML do gráfico: a figura com os dados para o script e a tabela de reserva.
func (g *Grafico) HTML(base string) string {
	j, _ := json.Marshal(g)
	var b strings.Builder
	b.WriteString(`<figure class="nep-grafico" data-grafico="` + html.EscapeString(string(j)) + `">`)
	if g.Titulo != "" {
		b.WriteString(`<figcaption>` + html.EscapeString(g.Titulo) + `</figcaption>`)
	}
	b.WriteString(`<table class="nep-grafico-dados"><thead><tr><th></th>`)
	for _, s := range g.Series {
		b.WriteString(`<th>` + html.EscapeString(s.Nome) + `</th>`)
	}
	b.WriteString(`</tr></thead><tbody>`)
	for i, r := range g.Rotulos {
		b.WriteString(`<tr><th>` + html.EscapeString(r) + `</th>`)
		for _, s := range g.Series {
			v := ""
			if i < len(s.Valores) {
				if g.Moeda {
					v = "R$ " + strings.ReplaceAll(strconv.FormatFloat(s.Valores[i], 'f', 2, 64), ".", ",")
				} else {
					v = strings.ReplaceAll(strconv.FormatFloat(s.Valores[i], 'f', -1, 64), ".", ",")
				}
			}
			b.WriteString(`<td>` + v + `</td>`)
		}
		b.WriteString(`</tr>`)
	}
	b.WriteString(`</tbody></table></figure>`)
	b.WriteString(`<script src="` + html.EscapeString(base) + `/_nep/grafico.js" data-echarts="` + html.EscapeString(base) + `/_nep/echarts.js" defer></script>`)
	return b.String()
}

func (s *Server) graficoBuiltin(a []lang.Value) (lang.Value, error) {
	op, _ := argv(a, 2).(*lang.Table)
	g, err := montarGrafico(strOrEmpty(argv(a, 0)), argv(a, 1), op)
	if err != nil {
		return nil, err
	}
	return lang.HTML(g.HTML(s.opt.Base)), nil
}
