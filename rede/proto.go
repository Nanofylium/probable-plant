package rede

// Leitor mínimo de Protocol Buffers, só para o formato ONNX (onnx.proto3).

import (
	"encoding/binary"
	"errors"
	"math"
)

var errProto = errors.New("onnx: arquivo inválido ou cortado")

type pb struct {
	b []byte
	i int
}

func (p *pb) varint() (uint64, error) {
	var v uint64
	for s := uint(0); s < 64; s += 7 {
		if p.i >= len(p.b) {
			return 0, errProto
		}
		c := p.b[p.i]
		p.i++
		v |= uint64(c&0x7f) << s
		if c < 0x80 {
			return v, nil
		}
	}
	return 0, errProto
}

// campo lê o próximo (número, tipo); valor fica para as funções abaixo.
func (p *pb) campo() (int, int, error) {
	k, err := p.varint()
	if err != nil {
		return 0, 0, err
	}
	return int(k >> 3), int(k & 7), nil
}

func (p *pb) bytes() ([]byte, error) {
	n, err := p.varint()
	if err != nil {
		return nil, err
	}
	if n > uint64(len(p.b)-p.i) {
		return nil, errProto
	}
	s := p.b[p.i : p.i+int(n)]
	p.i += int(n)
	return s, nil
}

func (p *pb) pular(tipo int) error {
	switch tipo {
	case 0:
		_, err := p.varint()
		return err
	case 1:
		p.i += 8
	case 2:
		_, err := p.bytes()
		return err
	case 5:
		p.i += 4
	default:
		return errProto
	}
	if p.i > len(p.b) {
		return errProto
	}
	return nil
}

func (p *pb) fixed32() (uint32, error) {
	if p.i+4 > len(p.b) {
		return 0, errProto
	}
	v := binary.LittleEndian.Uint32(p.b[p.i:])
	p.i += 4
	return v, nil
}

// inteiros: aceita o campo "packed" (tipo 2) e o solto (tipo 0)
func (p *pb) int64s(tipo int, dst []int64) ([]int64, error) {
	if tipo == 0 {
		v, err := p.varint()
		return append(dst, int64(v)), err
	}
	b, err := p.bytes()
	if err != nil {
		return nil, err
	}
	q := &pb{b: b}
	for q.i < len(b) {
		v, err := q.varint()
		if err != nil {
			return nil, err
		}
		dst = append(dst, int64(v))
	}
	return dst, nil
}

func (p *pb) float32s(tipo int, dst []float32) ([]float32, error) {
	if tipo == 5 {
		v, err := p.fixed32()
		return append(dst, math.Float32frombits(v)), err
	}
	b, err := p.bytes()
	if err != nil {
		return nil, err
	}
	for j := 0; j+4 <= len(b); j += 4 {
		dst = append(dst, math.Float32frombits(binary.LittleEndian.Uint32(b[j:])))
	}
	return dst, nil
}

type noProto struct {
	op, nome string
	ent, sai []string
	attr     map[string]*atributo
}

type atributo struct {
	f    float32
	i    int64
	s    string
	t    *Tensor
	fs   []float32
	is   []int64
	temI bool
}

type grafo struct {
	nos      []*noProto
	iniciais map[string]*Tensor
	entradas []string
	saidas   []string
}

func lerModelo(b []byte) (*grafo, error) {
	p := &pb{b: b}
	for p.i < len(b) {
		f, t, err := p.campo()
		if err != nil {
			return nil, err
		}
		if f == 7 && t == 2 {
			gb, err := p.bytes()
			if err != nil {
				return nil, err
			}
			return lerGrafo(gb)
		}
		if err := p.pular(t); err != nil {
			return nil, err
		}
	}
	return nil, errors.New("onnx: o arquivo não tem grafo")
}

func lerGrafo(b []byte) (*grafo, error) {
	g := &grafo{iniciais: map[string]*Tensor{}}
	p := &pb{b: b}
	for p.i < len(b) {
		f, t, err := p.campo()
		if err != nil {
			return nil, err
		}
		if t != 2 || (f != 1 && f != 5 && f != 11 && f != 12) {
			if err := p.pular(t); err != nil {
				return nil, err
			}
			continue
		}
		sub, err := p.bytes()
		if err != nil {
			return nil, err
		}
		switch f {
		case 1:
			n, err := lerNo(sub)
			if err != nil {
				return nil, err
			}
			g.nos = append(g.nos, n)
		case 5:
			nome, tt, err := lerTensor(sub)
			if err != nil {
				return nil, err
			}
			g.iniciais[nome] = tt
		case 11, 12:
			nome, err := nomeDe(sub)
			if err != nil {
				return nil, err
			}
			if f == 11 {
				g.entradas = append(g.entradas, nome)
			} else {
				g.saidas = append(g.saidas, nome)
			}
		}
	}
	return g, nil
}

func nomeDe(b []byte) (string, error) {
	p := &pb{b: b}
	for p.i < len(b) {
		f, t, err := p.campo()
		if err != nil {
			return "", err
		}
		if f == 1 && t == 2 {
			s, err := p.bytes()
			return string(s), err
		}
		if err := p.pular(t); err != nil {
			return "", err
		}
	}
	return "", nil
}

func lerNo(b []byte) (*noProto, error) {
	n := &noProto{attr: map[string]*atributo{}}
	p := &pb{b: b}
	for p.i < len(b) {
		f, t, err := p.campo()
		if err != nil {
			return nil, err
		}
		if t != 2 {
			if err := p.pular(t); err != nil {
				return nil, err
			}
			continue
		}
		s, err := p.bytes()
		if err != nil {
			return nil, err
		}
		switch f {
		case 1:
			n.ent = append(n.ent, string(s))
		case 2:
			n.sai = append(n.sai, string(s))
		case 3:
			n.nome = string(s)
		case 4:
			n.op = string(s)
		case 5:
			nome, a, err := lerAtributo(s)
			if err != nil {
				return nil, err
			}
			n.attr[nome] = a
		}
	}
	return n, nil
}

func lerAtributo(b []byte) (string, *atributo, error) {
	a := &atributo{}
	nome := ""
	p := &pb{b: b}
	for p.i < len(b) {
		f, t, err := p.campo()
		if err != nil {
			return "", nil, err
		}
		switch {
		case f == 1 && t == 2:
			s, err := p.bytes()
			if err != nil {
				return "", nil, err
			}
			nome = string(s)
		case f == 2 && t == 5:
			v, err := p.fixed32()
			if err != nil {
				return "", nil, err
			}
			a.f = math.Float32frombits(v)
		case f == 3 && t == 0:
			v, err := p.varint()
			if err != nil {
				return "", nil, err
			}
			a.i, a.temI = int64(v), true
		case f == 4 && t == 2:
			s, err := p.bytes()
			if err != nil {
				return "", nil, err
			}
			a.s = string(s)
		case f == 5 && t == 2:
			s, err := p.bytes()
			if err != nil {
				return "", nil, err
			}
			_, a.t, err = lerTensor(s)
			if err != nil {
				return "", nil, err
			}
		case f == 7:
			if a.fs, err = p.float32s(t, a.fs); err != nil {
				return "", nil, err
			}
		case f == 8:
			if a.is, err = p.int64s(t, a.is); err != nil {
				return "", nil, err
			}
		default:
			if err := p.pular(t); err != nil {
				return "", nil, err
			}
		}
	}
	return nome, a, nil
}

const (
	tipoFloat = 1
	tipoInt32 = 6
	tipoInt64 = 7
	tipoBool  = 9
)

func lerTensor(b []byte) (string, *Tensor, error) {
	var dims []int64
	var tipo int
	var nome string
	var raw []byte
	var fs []float32
	var is []int64
	p := &pb{b: b}
	for p.i < len(b) {
		f, t, err := p.campo()
		if err != nil {
			return "", nil, err
		}
		switch {
		case f == 1:
			if dims, err = p.int64s(t, dims); err != nil {
				return "", nil, err
			}
		case f == 2 && t == 0:
			v, err := p.varint()
			if err != nil {
				return "", nil, err
			}
			tipo = int(v)
		case f == 4:
			if fs, err = p.float32s(t, fs); err != nil {
				return "", nil, err
			}
		case f == 5 || f == 7:
			if is, err = p.int64s(t, is); err != nil {
				return "", nil, err
			}
		case f == 8 && t == 2:
			s, err := p.bytes()
			if err != nil {
				return "", nil, err
			}
			nome = string(s)
		case f == 9 && t == 2:
			if raw, err = p.bytes(); err != nil {
				return "", nil, err
			}
		case f == 14 && t == 0:
			v, _ := p.varint()
			if v == 1 {
				return "", nil, errors.New("onnx: pesos em arquivo externo não são suportados")
			}
		default:
			if err := p.pular(t); err != nil {
				return "", nil, err
			}
		}
	}
	forma := make([]int, len(dims))
	n := 1
	for i, d := range dims {
		forma[i] = int(d)
		n *= int(d)
	}
	switch tipo {
	case tipoFloat:
		if raw != nil {
			if len(raw) != 4*n {
				return "", nil, errProto
			}
			fs = make([]float32, n)
			for i := range fs {
				fs[i] = math.Float32frombits(binary.LittleEndian.Uint32(raw[4*i:]))
			}
		}
		if len(fs) != n {
			return "", nil, errProto
		}
		return nome, &Tensor{Forma: forma, F: fs}, nil
	case tipoInt64, tipoInt32, tipoBool:
		if raw != nil {
			tam := map[int]int{tipoInt64: 8, tipoInt32: 4, tipoBool: 1}[tipo]
			if len(raw) != tam*n {
				return "", nil, errProto
			}
			is = make([]int64, n)
			for i := range is {
				switch tam {
				case 8:
					is[i] = int64(binary.LittleEndian.Uint64(raw[8*i:]))
				case 4:
					is[i] = int64(int32(binary.LittleEndian.Uint32(raw[4*i:])))
				default:
					is[i] = int64(raw[i])
				}
			}
		}
		if len(is) != n {
			return "", nil, errProto
		}
		return nome, &Tensor{Forma: forma, I: is, Inteiro: true}, nil
	}
	return "", nil, errors.New("onnx: tipo de tensor não suportado")
}
