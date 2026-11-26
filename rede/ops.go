package rede

import (
	"fmt"
	"math"
	"runtime"
	"sync"
)

// ---------- operações elemento a elemento ----------

type opBin int

const (
	opAdd opBin = iota
	opSub
	opMul
	opDiv
	opPow
	opEqual
)

func aplicaF(op opBin, x, y float32) float32 {
	switch op {
	case opAdd:
		return x + y
	case opSub:
		return x - y
	case opMul:
		return x * y
	case opDiv:
		return x / y
	case opPow:
		switch y {
		case 2:
			return x * x
		case .5:
			return float32(math.Sqrt(float64(x)))
		case 1:
			return x
		}
		return float32(math.Pow(float64(x), float64(y)))
	}
	if x == y {
		return 1
	}
	return 0
}

func aplicaI(op opBin, x, y int64) int64 {
	switch op {
	case opAdd:
		return x + y
	case opSub:
		return x - y
	case opMul:
		return x * y
	case opDiv:
		if y == 0 {
			return 0
		}
		return x / y
	case opPow:
		return int64(math.Pow(float64(x), float64(y)))
	}
	if x == y {
		return 1
	}
	return 0
}

func binario(op opBin, a, b *Tensor) (*Tensor, error) {
	forma, err := formaBroadcast(a.Forma, b.Forma)
	if err != nil {
		return nil, err
	}
	inteiro := a.Inteiro && b.Inteiro
	if a.Inteiro != b.Inteiro { // mistura rara: converte para float
		if a.Inteiro {
			a = paraFloat(a)
		} else {
			b = paraFloat(b)
		}
	}
	n := tamanho(forma)
	var out *Tensor
	if inteiro || op == opEqual { // Equal devolve bool (0/1)
		out = novoInt(forma)
	} else {
		out = &Tensor{Forma: forma, F: make([]float32, n)}
	}
	la, lb := a.Len(), b.Len()
	// casos rápidos
	if !inteiro && !out.Inteiro {
		af, bf, of := a.F, b.F, out.F
		switch {
		case la == n && lb == n:
			paralelo(n, func(i0, i1 int) {
				for i := i0; i < i1; i++ {
					of[i] = aplicaF(op, af[i], bf[i])
				}
			})
			return out, nil
		case la == n && lb == 1:
			y := bf[0]
			paralelo(n, func(i0, i1 int) {
				for i := i0; i < i1; i++ {
					of[i] = aplicaF(op, af[i], y)
				}
			})
			return out, nil
		case la == n && n%lb == 0 && sufixo(b.Forma, forma):
			paralelo(n/lb, func(r0, r1 int) {
				for r := r0; r < r1; r++ {
					o, x := of[r*lb:(r+1)*lb], af[r*lb:(r+1)*lb]
					for j, y := range bf {
						o[j] = aplicaF(op, x[j], y)
					}
				}
			})
			return out, nil
		case la == 1 && lb == n:
			x := af[0]
			for i := range of {
				of[i] = aplicaF(op, x, bf[i])
			}
			return out, nil
		}
	}
	// caso geral com broadcast
	sa, sb := passosBroadcast(a.Forma, forma), passosBroadcast(b.Forma, forma)
	idx := make([]int, len(forma))
	ia, ib := 0, 0
	for i := 0; i < n; i++ {
		switch {
		case out.Inteiro && inteiro:
			out.I[i] = aplicaI(op, a.I[ia], b.I[ib])
		case out.Inteiro:
			out.I[i] = int64(aplicaF(op, a.F[ia], b.F[ib]))
		default:
			out.F[i] = aplicaF(op, a.F[ia], b.F[ib])
		}
		for d := len(forma) - 1; d >= 0; d-- {
			idx[d]++
			ia += sa[d]
			ib += sb[d]
			if idx[d] < forma[d] {
				break
			}
			ia -= sa[d] * forma[d]
			ib -= sb[d] * forma[d]
			idx[d] = 0
		}
	}
	return out, nil
}

// b ocupa as últimas dimensões de forma (sem broadcast no meio)
func sufixo(b, forma []int) bool {
	bb := b
	for len(bb) > 0 && bb[0] == 1 {
		bb = bb[1:]
	}
	if len(bb) > len(forma) {
		return false
	}
	off := len(forma) - len(bb)
	for i := range bb {
		if bb[i] != forma[off+i] {
			return false
		}
	}
	return true
}

func paraFloat(t *Tensor) *Tensor {
	if !t.Inteiro {
		return t
	}
	f := make([]float32, len(t.I))
	for i, v := range t.I {
		f[i] = float32(v)
	}
	return &Tensor{Forma: t.Forma, F: f}
}

func unario(t *Tensor, f func(float32) float32) *Tensor {
	out := &Tensor{Forma: t.Forma, F: make([]float32, len(t.F))}
	paralelo(len(t.F), func(i0, i1 int) {
		for i := i0; i < i1; i++ {
			out.F[i] = f(t.F[i])
		}
	})
	return out
}

// ---------- paralelismo ----------

var nucleos = runtime.NumCPU()

// paralelo divide [0, n) entre os núcleos (só quando vale a pena)
func paralelo(n int, f func(i0, i1 int)) {
	w := nucleos
	if n < 4096 || w <= 1 {
		f(0, n)
		return
	}
	paraleloEm(n, w, f)
}

func paraleloEm(n, w int, f func(i0, i1 int)) {
	if w > n {
		w = n
	}
	if w <= 1 {
		f(0, n)
		return
	}
	var wg sync.WaitGroup
	passo := (n + w - 1) / w
	for i0 := 0; i0 < n; i0 += passo {
		i1 := min(i0+passo, n)
		wg.Add(1)
		go func(i0, i1 int) {
			defer wg.Done()
			f(i0, i1)
		}(i0, i1)
	}
	wg.Wait()
}

// ---------- multiplicação de matrizes ----------

// mm faz c = a×b com a [M×K], b [K×N], c [M×N] (c é sobrescrito).
func mm(a, b, c []float32, M, K, N int) {
	linhas := func(i0, i1, j0, j1 int) {
		for i := i0; i < i1; i++ {
			cr := c[i*N+j0 : i*N+j1]
			for j := range cr {
				cr[j] = 0
			}
			ar := a[i*K : (i+1)*K]
			for k, x := range ar {
				if x == 0 {
					continue
				}
				br := b[k*N+j0 : k*N+j1]
				br = br[:len(cr)]
				for j, y := range br {
					cr[j] += x * y
				}
			}
		}
	}
	trabalho := M * K * N
	if trabalho < 1<<16 || nucleos <= 1 {
		linhas(0, M, 0, N)
		return
	}
	if M >= nucleos*2 {
		paraleloEm(M, nucleos*2, func(i0, i1 int) { linhas(i0, i1, 0, N) })
		return
	}
	// poucas linhas (ex.: conv de 1 canal): divide as colunas
	paraleloEm(N, nucleos*2, func(j0, j1 int) { linhas(0, M, j0, j1) })
}

func matmul(a, b *Tensor) (*Tensor, error) {
	if len(a.Forma) < 2 || len(b.Forma) < 2 {
		return nil, fmt.Errorf("MatMul: formas %v e %v não suportadas", a.Forma, b.Forma)
	}
	M, K := a.Forma[len(a.Forma)-2], a.Forma[len(a.Forma)-1]
	K2, N := b.Forma[len(b.Forma)-2], b.Forma[len(b.Forma)-1]
	if K != K2 {
		return nil, fmt.Errorf("MatMul: %v × %v", a.Forma, b.Forma)
	}
	lotesA, lotesB := a.Forma[:len(a.Forma)-2], b.Forma[:len(b.Forma)-2]
	lotes, err := formaBroadcast(lotesA, lotesB)
	if err != nil {
		return nil, err
	}
	forma := append(append([]int(nil), lotes...), M, N)
	out := &Tensor{Forma: forma, F: make([]float32, tamanho(forma))}
	nl := tamanho(lotes)
	sa, sb := passosBroadcast(lotesA, lotes), passosBroadcast(lotesB, lotes)
	idx := make([]int, len(lotes))
	for l := 0; l < nl; l++ {
		oa, ob := 0, 0
		for d := range lotes {
			oa += idx[d] * sa[d]
			ob += idx[d] * sb[d]
		}
		mm(a.F[oa*M*K:(oa+1)*M*K], b.F[ob*K*N:(ob+1)*K*N], out.F[l*M*N:(l+1)*M*N], M, K, N)
		for d := len(lotes) - 1; d >= 0; d-- {
			idx[d]++
			if idx[d] < lotes[d] {
				break
			}
			idx[d] = 0
		}
	}
	return out, nil
}

// ---------- convoluções ----------

func intsAttr(n *noProto, nome string, padrao []int) []int {
	a, ok := n.attr[nome]
	if !ok || len(a.is) == 0 {
		return padrao
	}
	out := make([]int, len(a.is))
	for i, v := range a.is {
		out[i] = int(v)
	}
	return out
}

func intAttr(n *noProto, nome string, padrao int) int {
	if a, ok := n.attr[nome]; ok && a.temI {
		return int(a.i)
	}
	return padrao
}

func conv(n *noProto, x, w, bias *Tensor) (*Tensor, error) {
	if len(x.Forma) != 4 || len(w.Forma) != 4 {
		return nil, fmt.Errorf("Conv: só 2D (%v, %v)", x.Forma, w.Forma)
	}
	B, C, H, W := x.Forma[0], x.Forma[1], x.Forma[2], x.Forma[3]
	M, Cg, KH, KW := w.Forma[0], w.Forma[1], w.Forma[2], w.Forma[3]
	g := intAttr(n, "group", 1)
	st := intsAttr(n, "strides", []int{1, 1})
	dl := intsAttr(n, "dilations", []int{1, 1})
	pd := intsAttr(n, "pads", []int{0, 0, 0, 0})
	if C != Cg*g || M%g != 0 {
		return nil, fmt.Errorf("Conv: canais %d, pesos %v, grupos %d", C, w.Forma, g)
	}
	OH := (H+pd[0]+pd[2]-dl[0]*(KH-1)-1)/st[0] + 1
	OW := (W+pd[1]+pd[3]-dl[1]*(KW-1)-1)/st[1] + 1
	out := NovoTensor(B, M, OH, OW)
	Mg, CK := M/g, Cg*KH*KW
	// im2col em faixas de linhas da saída, para não estourar a memória
	faixa := max(1, (1<<22)/max(CK*OW, 1))
	cols := make([]float32, CK*min(faixa, OH)*OW)
	tmp := make([]float32, Mg*min(faixa, OH)*OW)
	for b := 0; b < B; b++ {
		for gi := 0; gi < g; gi++ {
			wg := w.F[gi*Mg*CK : (gi+1)*Mg*CK]
			for oy0 := 0; oy0 < OH; oy0 += faixa {
				oy1 := min(oy0+faixa, OH)
				P := (oy1 - oy0) * OW
				cs := cols[:CK*P]
				paraleloEm(CK, nucleos, func(r0, r1 int) {
					for r := r0; r < r1; r++ {
						c := gi*Cg + r/(KH*KW)
						ky := (r / KW) % KH
						kx := r % KW
						src := x.F[(b*C+c)*H*W : (b*C+c+1)*H*W]
						dst := cs[r*P : (r+1)*P]
						i := 0
						for oy := oy0; oy < oy1; oy++ {
							iy := oy*st[0] - pd[0] + ky*dl[0]
							for ox := 0; ox < OW; ox++ {
								ix := ox*st[1] - pd[1] + kx*dl[1]
								if iy < 0 || iy >= H || ix < 0 || ix >= W {
									dst[i] = 0
								} else {
									dst[i] = src[iy*W+ix]
								}
								i++
							}
						}
					}
				})
				t := tmp[:Mg*P]
				mm(wg, cs, t, Mg, CK, P)
				for m := 0; m < Mg; m++ {
					oc := gi*Mg + m
					var bv float32
					if bias != nil {
						bv = bias.F[oc]
					}
					dst := out.F[(b*M+oc)*OH*OW+oy0*OW : (b*M+oc)*OH*OW+oy1*OW]
					for i, v := range t[m*P : (m+1)*P] {
						dst[i] = v + bv
					}
				}
			}
		}
	}
	return out, nil
}

func convTransposta(n *noProto, x, w, bias *Tensor) (*Tensor, error) {
	if len(x.Forma) != 4 || len(w.Forma) != 4 || intAttr(n, "group", 1) != 1 {
		return nil, fmt.Errorf("ConvTranspose: caso não suportado (%v, %v)", x.Forma, w.Forma)
	}
	B, C, H, W := x.Forma[0], x.Forma[1], x.Forma[2], x.Forma[3]
	M, KH, KW := w.Forma[1], w.Forma[2], w.Forma[3]
	st := intsAttr(n, "strides", []int{1, 1})
	dl := intsAttr(n, "dilations", []int{1, 1})
	pd := intsAttr(n, "pads", []int{0, 0, 0, 0})
	op := intsAttr(n, "output_padding", []int{0, 0})
	OH := (H-1)*st[0] - pd[0] - pd[2] + dl[0]*(KH-1) + 1 + op[0]
	OW := (W-1)*st[1] - pd[1] - pd[3] + dl[1]*(KW-1) + 1 + op[1]
	out := NovoTensor(B, M, OH, OW)
	for b := 0; b < B; b++ {
		paraleloEm(M, nucleos, func(m0, m1 int) {
			for m := m0; m < m1; m++ {
				dst := out.F[(b*M+m)*OH*OW : (b*M+m+1)*OH*OW]
				if bias != nil {
					for i := range dst {
						dst[i] = bias.F[m]
					}
				}
				for c := 0; c < C; c++ {
					src := x.F[(b*C+c)*H*W : (b*C+c+1)*H*W]
					for ky := 0; ky < KH; ky++ {
						for kx := 0; kx < KW; kx++ {
							wv := w.F[((c*M+m)*KH+ky)*KW+kx]
							for iy := 0; iy < H; iy++ {
								oy := iy*st[0] - pd[0] + ky*dl[0]
								if oy < 0 || oy >= OH {
									continue
								}
								for ix := 0; ix < W; ix++ {
									ox := ix*st[1] - pd[1] + kx*dl[1]
									if ox >= 0 && ox < OW {
										dst[oy*OW+ox] += src[iy*W+ix] * wv
									}
								}
							}
						}
					}
				}
			}
		})
	}
	return out, nil
}

// ---------- redimensionar (Resize) ----------

type taps struct {
	i []int
	w []float32
}

func pesosEixo(in, out int, modo, coord string, a float32) []taps {
	escala := float64(out) / float64(in)
	t := make([]taps, out)
	for o := 0; o < out; o++ {
		var x float64
		switch coord {
		case "align_corners":
			if out > 1 {
				x = float64(o) * float64(in-1) / float64(out-1)
			}
		case "asymmetric":
			x = float64(o) / escala
		default: // half_pixel
			x = (float64(o)+.5)/escala - .5
		}
		switch modo {
		case "cubic":
			x0 := math.Floor(x)
			s := float32(x - x0)
			ws := [4]float32{}
			for k := -1; k <= 2; k++ {
				d := float32(math.Abs(float64(float32(k) - s)))
				var v float32
				switch {
				case d <= 1:
					v = ((a+2)*d-(a+3))*d*d + 1
				case d < 2:
					v = ((a*d-5*a)*d+8*a)*d - 4*a
				}
				ws[k+1] = v
			}
			for k := -1; k <= 2; k++ {
				t[o].i = append(t[o].i, clamp(int(x0)+k, in-1))
				t[o].w = append(t[o].w, ws[k+1])
			}
		case "nearest":
			t[o].i = []int{clamp(int(math.Floor(x)), in-1)}
			t[o].w = []float32{1}
		default: // linear
			if x < 0 {
				x = 0
			}
			if x > float64(in-1) {
				x = float64(in - 1)
			}
			x0 := int(math.Floor(x))
			x1 := min(x0+1, in-1)
			f := float32(x - float64(x0))
			t[o].i = []int{x0, x1}
			t[o].w = []float32{1 - f, f}
		}
	}
	return t
}

func clamp(v, m int) int {
	if v < 0 {
		return 0
	}
	if v > m {
		return m
	}
	return v
}

func redimensionar(n *noProto, x *Tensor, escalas, tamanhos *Tensor) (*Tensor, error) {
	if len(x.Forma) != 4 {
		return nil, fmt.Errorf("Resize: só 4D (%v)", x.Forma)
	}
	B, C, H, W := x.Forma[0], x.Forma[1], x.Forma[2], x.Forma[3]
	OH, OW := H, W
	switch {
	case tamanhos != nil && tamanhos.Len() == 4:
		s := tamanhos.ints()
		if int(s[0]) != B || int(s[1]) != C {
			return nil, fmt.Errorf("Resize: só redimensiona altura e largura")
		}
		OH, OW = int(s[2]), int(s[3])
	case escalas != nil && escalas.Len() == 4:
		e := paraFloat(escalas).F
		OH, OW = int(math.Floor(float64(H)*float64(e[2]))), int(math.Floor(float64(W)*float64(e[3])))
	default:
		return nil, fmt.Errorf("Resize: faltam escalas ou tamanhos")
	}
	modo, coord := "nearest", "half_pixel"
	if a, ok := n.attr["mode"]; ok {
		modo = a.s
	}
	if a, ok := n.attr["coordinate_transformation_mode"]; ok {
		coord = a.s
	}
	ca := float32(-.75)
	if a, ok := n.attr["cubic_coeff_a"]; ok {
		ca = a.f
	}
	ty, tx := pesosEixo(H, OH, modo, coord, ca), pesosEixo(W, OW, modo, coord, ca)
	out := NovoTensor(B, C, OH, OW)
	paraleloEm(B*C, nucleos, func(p0, p1 int) {
		linha := make([]float32, W)
		for p := p0; p < p1; p++ {
			src := x.F[p*H*W : (p+1)*H*W]
			dst := out.F[p*OH*OW : (p+1)*OH*OW]
			for oy := 0; oy < OH; oy++ {
				for j := range linha {
					linha[j] = 0
				}
				for k, iy := range ty[oy].i {
					wy := ty[oy].w[k]
					r := src[iy*W : (iy+1)*W]
					for j, v := range r {
						linha[j] += wy * v
					}
				}
				d := dst[oy*OW : (oy+1)*OW]
				for ox := range d {
					var s float32
					for k, ix := range tx[ox].i {
						s += tx[ox].w[k] * linha[ix]
					}
					d[ox] = s
				}
			}
		}
	})
	return out, nil
}
