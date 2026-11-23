package rede

import "fmt"

// Tensor guarda números float32 (F) ou inteiros (I, quando Inteiro).
type Tensor struct {
	Forma   []int
	F       []float32
	I       []int64
	Inteiro bool
}

func tamanho(forma []int) int {
	n := 1
	for _, d := range forma {
		n *= d
	}
	return n
}

// NovoTensor cria um tensor de float32 zerado.
func NovoTensor(forma ...int) *Tensor {
	return &Tensor{Forma: append([]int(nil), forma...), F: make([]float32, tamanho(forma))}
}

func novoInt(forma []int) *Tensor {
	return &Tensor{Forma: append([]int(nil), forma...), I: make([]int64, tamanho(forma)), Inteiro: true}
}

func (t *Tensor) Len() int { return tamanho(t.Forma) }

func (t *Tensor) String() string {
	if t.Inteiro {
		return fmt.Sprintf("int%v", t.Forma)
	}
	return fmt.Sprintf("float%v", t.Forma)
}

// valores inteiros (shapes, índices)
func (t *Tensor) ints() []int64 {
	if t.Inteiro {
		return t.I
	}
	out := make([]int64, len(t.F))
	for i, v := range t.F {
		out[i] = int64(v)
	}
	return out
}

// passos (strides) em elementos
func passos(forma []int) []int {
	s := make([]int, len(forma))
	p := 1
	for i := len(forma) - 1; i >= 0; i-- {
		s[i] = p
		p *= forma[i]
	}
	return s
}

func eixo(a, n int) int {
	if a < 0 {
		a += n
	}
	return a
}

// forma resultante do broadcast entre duas formas (regras do numpy)
func formaBroadcast(a, b []int) ([]int, error) {
	n := max(len(a), len(b))
	out := make([]int, n)
	for i := 0; i < n; i++ {
		da, db := 1, 1
		if j := i - (n - len(a)); j >= 0 {
			da = a[j]
		}
		if j := i - (n - len(b)); j >= 0 {
			db = b[j]
		}
		switch {
		case da == db, db == 1:
			out[i] = da
		case da == 1:
			out[i] = db
		default:
			return nil, fmt.Errorf("formas incompatíveis %v e %v", a, b)
		}
	}
	return out, nil
}

// índices de leitura de um tensor de forma f dentro de uma forma maior (broadcast)
func passosBroadcast(f, saida []int) []int {
	s := passos(f)
	out := make([]int, len(saida))
	off := len(saida) - len(f)
	for i := range f {
		if f[i] != 1 {
			out[i+off] = s[i]
		}
	}
	return out
}
