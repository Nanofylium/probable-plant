// Pacote rede roda redes neurais no formato ONNX, em Go puro e no processador
// (sem GPU, sem Python). Implementa as operações que o Nepenthes usa, como as
// do Depth Anything V2 (transformer de visão + cabeça DPT).
package rede

import (
	"errors"
	"fmt"
	"math"
	"os"
	"sort"
)

// Modelo é uma rede carregada.
type Modelo struct {
	g       *grafo
	usos    map[string]int // quantos nós leem cada valor (para liberar memória)
	Entrada string
	Saida   string
}

// Abrir carrega um arquivo .onnx.
func Abrir(caminho string) (*Modelo, error) {
	b, err := os.ReadFile(caminho)
	if err != nil {
		return nil, err
	}
	return Carregar(b)
}

// Carregar lê um modelo ONNX da memória.
func Carregar(b []byte) (*Modelo, error) {
	g, err := lerModelo(b)
	if err != nil {
		return nil, err
	}
	m := &Modelo{g: g, usos: map[string]int{}}
	for _, n := range g.nos {
		if _, ok := ops[n.op]; !ok {
			return nil, fmt.Errorf("onnx: operação %s ainda não suportada", n.op)
		}
		for _, e := range n.ent {
			m.usos[e]++
		}
	}
	for _, e := range g.entradas {
		if _, ok := g.iniciais[e]; !ok {
			m.Entrada = e
			break
		}
	}
	if len(g.saidas) > 0 {
		m.Saida = g.saidas[0]
	}
	if m.Entrada == "" || m.Saida == "" {
		return nil, errors.New("onnx: o modelo não tem entrada ou saída")
	}
	return m, nil
}

// Rodar executa a rede com a entrada dada e devolve a primeira saída.
func (m *Modelo) Rodar(entrada *Tensor) (*Tensor, error) {
	vals := map[string]*Tensor{m.Entrada: entrada}
	restam := make(map[string]int, len(m.usos))
	for k, v := range m.usos {
		restam[k] = v
	}
	pega := func(nome string) *Tensor {
		if nome == "" {
			return nil
		}
		if t, ok := vals[nome]; ok {
			return t
		}
		return m.g.iniciais[nome]
	}
	for _, n := range m.g.nos {
		ent := make([]*Tensor, len(n.ent))
		for i, e := range n.ent {
			ent[i] = pega(e)
			if ent[i] == nil && e != "" {
				return nil, fmt.Errorf("onnx: %s (%s) sem a entrada %s", n.nome, n.op, e)
			}
		}
		saida, err := ops[n.op](n, ent)
		if err != nil {
			return nil, fmt.Errorf("onnx: %s (%s): %w", n.nome, n.op, err)
		}
		for i, s := range n.sai {
			if i < len(saida) {
				vals[s] = saida[i]
			}
		}
		// libera o que ninguém mais vai ler
		for _, e := range n.ent {
			if restam[e]--; restam[e] == 0 && e != m.Saida {
				delete(vals, e)
			}
		}
	}
	out, ok := vals[m.Saida]
	if !ok {
		return nil, errors.New("onnx: a saída não foi calculada")
	}
	return out, nil
}

type funcOp func(n *noProto, e []*Tensor) ([]*Tensor, error)

func um(t *Tensor, err error) ([]*Tensor, error) {
	if err != nil {
		return nil, err
	}
	return []*Tensor{t}, nil
}

func bin(op opBin) funcOp {
	return func(n *noProto, e []*Tensor) ([]*Tensor, error) { return um(binario(op, e[0], e[1])) }
}

func opcional(e []*Tensor, i int) *Tensor {
	if i < len(e) {
		return e[i]
	}
	return nil
}

var ops map[string]funcOp

func init() {
	ops = map[string]funcOp{
		"Add": bin(opAdd), "Sub": bin(opSub), "Mul": bin(opMul), "Div": bin(opDiv), "Pow": bin(opPow), "Equal": bin(opEqual),
		"MatMul": func(n *noProto, e []*Tensor) ([]*Tensor, error) { return um(matmul(e[0], e[1])) },
		"Conv": func(n *noProto, e []*Tensor) ([]*Tensor, error) {
			return um(conv(n, e[0], e[1], opcional(e, 2)))
		},
		"ConvTranspose": func(n *noProto, e []*Tensor) ([]*Tensor, error) {
			return um(convTransposta(n, e[0], e[1], opcional(e, 2)))
		},
		"Resize": func(n *noProto, e []*Tensor) ([]*Tensor, error) {
			return um(redimensionar(n, e[0], opcional(e, 2), opcional(e, 3)))
		},
		"Relu": func(n *noProto, e []*Tensor) ([]*Tensor, error) {
			return um(unario(e[0], func(x float32) float32 { return max(x, 0) }), nil)
		},
		"Sqrt": func(n *noProto, e []*Tensor) ([]*Tensor, error) {
			return um(unario(e[0], func(x float32) float32 { return float32(math.Sqrt(float64(x))) }), nil)
		},
		"Erf": func(n *noProto, e []*Tensor) ([]*Tensor, error) {
			return um(unario(e[0], func(x float32) float32 { return float32(math.Erf(float64(x))) }), nil)
		},
		"Softmax":    softmax,
		"ReduceMean": reduceMean,
		"Shape": func(n *noProto, e []*Tensor) ([]*Tensor, error) {
			t := novoInt([]int{len(e[0].Forma)})
			for i, d := range e[0].Forma {
				t.I[i] = int64(d)
			}
			return um(t, nil)
		},
		"Reshape":   reshape,
		"Transpose": transpose,
		"Concat":    concat,
		"Gather":    gather,
		"Slice":     slice,
		"Unsqueeze": func(n *noProto, e []*Tensor) ([]*Tensor, error) { return um(unsqueeze(e[0], eixos(n, e)), nil) },
		"Squeeze":   func(n *noProto, e []*Tensor) ([]*Tensor, error) { return um(squeeze(e[0], eixos(n, e)), nil) },
		"Expand": func(n *noProto, e []*Tensor) ([]*Tensor, error) {
			alvo := e[1].ints()
			f := make([]int, len(alvo))
			for i, v := range alvo {
				f[i] = int(v)
			}
			forma, err := formaBroadcast(e[0].Forma, f)
			if err != nil {
				return nil, err
			}
			return um(expandir(e[0], forma), nil)
		},
		"Where": func(n *noProto, e []*Tensor) ([]*Tensor, error) { return um(onde(e[0], e[1], e[2])) },
	}
}

func eixos(n *noProto, e []*Tensor) []int {
	var v []int64
	if len(e) > 1 && e[1] != nil {
		v = e[1].ints()
	} else if a, ok := n.attr["axes"]; ok {
		v = a.is
	}
	out := make([]int, len(v))
	for i, x := range v {
		out[i] = int(x)
	}
	return out
}

func vista(t *Tensor, forma []int) *Tensor {
	return &Tensor{Forma: forma, F: t.F, I: t.I, Inteiro: t.Inteiro}
}

func unsqueeze(t *Tensor, ax []int) *Tensor {
	r := len(t.Forma) + len(ax)
	marca := make([]bool, r)
	for _, a := range ax {
		marca[eixo(a, r)] = true
	}
	forma := make([]int, 0, r)
	j := 0
	for i := 0; i < r; i++ {
		if marca[i] {
			forma = append(forma, 1)
		} else {
			forma = append(forma, t.Forma[j])
			j++
		}
	}
	return vista(t, forma)
}

func squeeze(t *Tensor, ax []int) *Tensor {
	var forma []int
	tira := map[int]bool{}
	for _, a := range ax {
		tira[eixo(a, len(t.Forma))] = true
	}
	for i, d := range t.Forma {
		if (len(ax) == 0 && d == 1) || tira[i] {
			continue
		}
		forma = append(forma, d)
	}
	return vista(t, forma)
}

func reshape(n *noProto, e []*Tensor) ([]*Tensor, error) {
	alvo := e[1].ints()
	forma := make([]int, len(alvo))
	livre, conhecido := -1, 1
	for i, v := range alvo {
		switch {
		case v == 0 && intAttr(n, "allowzero", 0) == 0:
			forma[i] = e[0].Forma[i]
		case v == -1:
			livre = i
			continue
		default:
			forma[i] = int(v)
		}
		conhecido *= forma[i]
	}
	if livre >= 0 {
		if conhecido == 0 {
			return nil, errors.New("Reshape: dimensão -1 com zero")
		}
		forma[livre] = e[0].Len() / conhecido
	}
	if tamanho(forma) != e[0].Len() {
		return nil, fmt.Errorf("Reshape: %v para %v", e[0].Forma, alvo)
	}
	return um(vista(e[0], forma), nil)
}

func transpose(n *noProto, e []*Tensor) ([]*Tensor, error) {
	t := e[0]
	r := len(t.Forma)
	perm := intsAttr(n, "perm", nil)
	if perm == nil {
		perm = make([]int, r)
		for i := range perm {
			perm[i] = r - 1 - i
		}
	}
	forma := make([]int, r)
	for i, p := range perm {
		forma[i] = t.Forma[p]
	}
	sIn := passos(t.Forma)
	sp := make([]int, r) // passo de leitura para cada eixo da saída
	for i, p := range perm {
		sp[i] = sIn[p]
	}
	var out *Tensor
	if t.Inteiro {
		out = novoInt(forma)
	} else {
		out = NovoTensor(forma...)
	}
	N := tamanho(forma)
	if r == 0 || N == 0 {
		return um(out, nil)
	}
	// o último eixo vira laço interno; os de fora são divididos entre os núcleos
	ult := forma[r-1]
	externos := N / ult
	paraleloEm(externos, nucleos, func(o0, o1 int) {
		idx := make([]int, r-1)
		// posiciona o contador no início do bloco
		rest := o0
		for d := r - 2; d >= 0; d-- {
			idx[d] = rest % forma[d]
			rest /= forma[d]
		}
		for o := o0; o < o1; o++ {
			base := 0
			for d := 0; d < r-1; d++ {
				base += idx[d] * sp[d]
			}
			dst := o * ult
			if t.Inteiro {
				for k := 0; k < ult; k++ {
					out.I[dst+k] = t.I[base+k*sp[r-1]]
				}
			} else {
				s := sp[r-1]
				d := out.F[dst : dst+ult]
				for k := range d {
					d[k] = t.F[base+k*s]
				}
			}
			for d := r - 2; d >= 0; d-- {
				idx[d]++
				if idx[d] < forma[d] {
					break
				}
				idx[d] = 0
			}
		}
	})
	return um(out, nil)
}

func concat(n *noProto, e []*Tensor) ([]*Tensor, error) {
	r := len(e[0].Forma)
	ax := eixo(intAttr(n, "axis", 0), r)
	forma := append([]int(nil), e[0].Forma...)
	forma[ax] = 0
	inteiro := true
	for _, t := range e {
		forma[ax] += t.Forma[ax]
		inteiro = inteiro && t.Inteiro
	}
	if !inteiro {
		for i := range e {
			e[i] = paraFloat(e[i])
		}
	}
	var out *Tensor
	if inteiro {
		out = novoInt(forma)
	} else {
		out = NovoTensor(forma...)
	}
	fora := tamanho(forma[:ax])
	dentro := tamanho(forma[ax+1:])
	pos := 0
	for o := 0; o < fora; o++ {
		for _, t := range e {
			bloco := t.Forma[ax] * dentro
			if inteiro {
				copy(out.I[pos:pos+bloco], t.I[o*bloco:(o+1)*bloco])
			} else {
				copy(out.F[pos:pos+bloco], t.F[o*bloco:(o+1)*bloco])
			}
			pos += bloco
		}
	}
	return um(out, nil)
}

func gather(n *noProto, e []*Tensor) ([]*Tensor, error) {
	t, ind := e[0], e[1].ints()
	ax := eixo(intAttr(n, "axis", 0), len(t.Forma))
	forma := append(append(append([]int(nil), t.Forma[:ax]...), e[1].Forma...), t.Forma[ax+1:]...)
	var out *Tensor
	if t.Inteiro {
		out = novoInt(forma)
	} else {
		out = NovoTensor(forma...)
	}
	fora, dim, dentro := tamanho(t.Forma[:ax]), t.Forma[ax], tamanho(t.Forma[ax+1:])
	pos := 0
	for o := 0; o < fora; o++ {
		for _, ix := range ind {
			i := int(ix)
			if i < 0 {
				i += dim
			}
			if i < 0 || i >= dim {
				return nil, fmt.Errorf("Gather: índice %d fora de %d", ix, dim)
			}
			src := (o*dim + i) * dentro
			if t.Inteiro {
				copy(out.I[pos:pos+dentro], t.I[src:src+dentro])
			} else {
				copy(out.F[pos:pos+dentro], t.F[src:src+dentro])
			}
			pos += dentro
		}
	}
	return um(out, nil)
}

func slice(n *noProto, e []*Tensor) ([]*Tensor, error) {
	t := e[0]
	r := len(t.Forma)
	ini, fim := e[1].ints(), e[2].ints()
	var axs, sts []int64
	if x := opcional(e, 3); x != nil {
		axs = x.ints()
	} else {
		for i := range ini {
			axs = append(axs, int64(i))
		}
	}
	if x := opcional(e, 4); x != nil {
		sts = x.ints()
	} else {
		for range ini {
			sts = append(sts, 1)
		}
	}
	comeco := make([]int, r)
	passo := make([]int, r)
	forma := append([]int(nil), t.Forma...)
	for i := range passo {
		passo[i] = 1
	}
	for k, a := range axs {
		ax := eixo(int(a), r)
		d := int64(t.Forma[ax])
		s, f, p := ini[k], fim[k], sts[k]
		if p == 0 {
			return nil, errors.New("Slice: passo zero")
		}
		if s < 0 {
			s += d
		}
		if f < 0 {
			f += d
		}
		if p > 0 {
			s = min(max(s, 0), d)
			f = min(max(f, 0), d)
		} else {
			s = min(max(s, 0), d-1)
			f = min(max(f, -1), d-1)
		}
		cnt := int64(0)
		if p > 0 && f > s {
			cnt = (f - s + p - 1) / p
		} else if p < 0 && s > f {
			cnt = (s - f - p - 1) / (-p)
		}
		comeco[ax], passo[ax], forma[ax] = int(s), int(p), int(cnt)
	}
	var out *Tensor
	if t.Inteiro {
		out = novoInt(forma)
	} else {
		out = NovoTensor(forma...)
	}
	sIn := passos(t.Forma)
	N := tamanho(forma)
	idx := make([]int, r)
	for i := 0; i < N; i++ {
		src := 0
		for d := 0; d < r; d++ {
			src += (comeco[d] + idx[d]*passo[d]) * sIn[d]
		}
		if t.Inteiro {
			out.I[i] = t.I[src]
		} else {
			out.F[i] = t.F[src]
		}
		for d := r - 1; d >= 0; d-- {
			idx[d]++
			if idx[d] < forma[d] {
				break
			}
			idx[d] = 0
		}
	}
	return um(out, nil)
}

func expandir(t *Tensor, forma []int) *Tensor {
	var out *Tensor
	if t.Inteiro {
		out = novoInt(forma)
	} else {
		out = NovoTensor(forma...)
	}
	s := passosBroadcast(t.Forma, forma)
	idx := make([]int, len(forma))
	for i := 0; i < tamanho(forma); i++ {
		src := 0
		for d := range forma {
			src += idx[d] * s[d]
		}
		if t.Inteiro {
			out.I[i] = t.I[src]
		} else {
			out.F[i] = t.F[src]
		}
		for d := len(forma) - 1; d >= 0; d-- {
			idx[d]++
			if idx[d] < forma[d] {
				break
			}
			idx[d] = 0
		}
	}
	return out
}

func onde(c, x, y *Tensor) (*Tensor, error) {
	f1, err := formaBroadcast(c.Forma, x.Forma)
	if err != nil {
		return nil, err
	}
	forma, err := formaBroadcast(f1, y.Forma)
	if err != nil {
		return nil, err
	}
	ce, xe, ye := expandir(c, forma), expandir(x, forma), expandir(y, forma)
	if x.Inteiro != y.Inteiro {
		xe, ye = paraFloat(xe), paraFloat(ye)
	}
	out := xe
	cv := ce.ints()
	for i, v := range cv {
		if v == 0 {
			if out.Inteiro {
				out.I[i] = ye.I[i]
			} else {
				out.F[i] = ye.F[i]
			}
		}
	}
	return out, nil
}

func softmax(n *noProto, e []*Tensor) ([]*Tensor, error) {
	t := e[0]
	ax := eixo(intAttr(n, "axis", -1), len(t.Forma))
	if ax != len(t.Forma)-1 {
		return nil, fmt.Errorf("Softmax: só no último eixo (pedido %d)", ax)
	}
	L := t.Forma[ax]
	out := NovoTensor(t.Forma...)
	paraleloEm(t.Len()/L, nucleos, func(r0, r1 int) {
		for r := r0; r < r1; r++ {
			x, o := t.F[r*L:(r+1)*L], out.F[r*L:(r+1)*L]
			m := float32(math.Inf(-1))
			for _, v := range x {
				m = max(m, v)
			}
			var s float32
			for j, v := range x {
				ev := float32(math.Exp(float64(v - m)))
				o[j] = ev
				s += ev
			}
			for j := range o {
				o[j] /= s
			}
		}
	})
	return um(out, nil)
}

func reduceMean(n *noProto, e []*Tensor) ([]*Tensor, error) {
	t := e[0]
	axs := eixos(n, e)
	r := len(t.Forma)
	if len(axs) == 0 {
		for i := 0; i < r; i++ {
			axs = append(axs, i)
		}
	}
	for i := range axs {
		axs[i] = eixo(axs[i], r)
	}
	sort.Ints(axs)
	// caso comum: últimos eixos contíguos (LayerNorm)
	contiguo := true
	for i, a := range axs {
		if a != r-len(axs)+i {
			contiguo = false
		}
	}
	if !contiguo {
		return nil, fmt.Errorf("ReduceMean: eixos %v não suportados", axs)
	}
	L := tamanho(t.Forma[r-len(axs):])
	forma := append([]int(nil), t.Forma...)
	for _, a := range axs {
		forma[a] = 1
	}
	if intAttr(n, "keepdims", 1) == 0 {
		forma = forma[:r-len(axs)]
	}
	out := NovoTensor(forma...)
	for i := range out.F {
		var s float64
		for _, v := range t.F[i*L : (i+1)*L] {
			s += float64(v)
		}
		out.F[i] = float32(s / float64(L))
	}
	return um(out, nil)
}
