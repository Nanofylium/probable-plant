package mundo

// MessagePack mínimo (só o que o protocolo do SGLang usa): mapas, listas,
// textos, bytes, números e booleanos. Especificação: https://msgpack.org.

import (
	"encoding/binary"
	"errors"
	"fmt"
	"math"
	"sort"
)

// Empacotar codifica v em MessagePack. Mapas saem com as chaves em ordem.
func Empacotar(v any) ([]byte, error) {
	var b []byte
	return empacotar(b, v)
}

func empacotar(b []byte, v any) ([]byte, error) {
	switch x := v.(type) {
	case nil:
		return append(b, 0xc0), nil
	case bool:
		if x {
			return append(b, 0xc3), nil
		}
		return append(b, 0xc2), nil
	case int:
		return inteiro(b, int64(x)), nil
	case int64:
		return inteiro(b, x), nil
	case float64:
		b = append(b, 0xcb)
		return binary.BigEndian.AppendUint64(b, math.Float64bits(x)), nil
	case string:
		n := len(x)
		switch {
		case n < 32:
			b = append(b, 0xa0|byte(n))
		case n < 1<<8:
			b = append(b, 0xd9, byte(n))
		case n < 1<<16:
			b = binary.BigEndian.AppendUint16(append(b, 0xda), uint16(n))
		default:
			b = binary.BigEndian.AppendUint32(append(b, 0xdb), uint32(n))
		}
		return append(b, x...), nil
	case []byte:
		n := len(x)
		switch {
		case n < 1<<8:
			b = append(b, 0xc4, byte(n))
		case n < 1<<16:
			b = binary.BigEndian.AppendUint16(append(b, 0xc5), uint16(n))
		default:
			b = binary.BigEndian.AppendUint32(append(b, 0xc6), uint32(n))
		}
		return append(b, x...), nil
	case []string:
		l := make([]any, len(x))
		for i, s := range x {
			l[i] = s
		}
		return empacotar(b, l)
	case [][]string:
		l := make([]any, len(x))
		for i, s := range x {
			l[i] = s
		}
		return empacotar(b, l)
	case []any:
		b = cabeca(b, len(x), 0x90, 0xdc, 0xdd)
		var err error
		for _, it := range x {
			if b, err = empacotar(b, it); err != nil {
				return nil, err
			}
		}
		return b, nil
	case map[string]any:
		b = cabeca(b, len(x), 0x80, 0xde, 0xdf)
		ks := make([]string, 0, len(x))
		for k := range x {
			ks = append(ks, k)
		}
		sort.Strings(ks)
		var err error
		for _, k := range ks {
			b, _ = empacotar(b, k)
			if b, err = empacotar(b, x[k]); err != nil {
				return nil, err
			}
		}
		return b, nil
	}
	return nil, fmt.Errorf("msgpack: tipo %T não suportado", v)
}

func inteiro(b []byte, n int64) []byte {
	switch {
	case n >= 0 && n < 128:
		return append(b, byte(n))
	case n < 0 && n >= -32:
		return append(b, byte(n))
	case n >= 0:
		return binary.BigEndian.AppendUint64(append(b, 0xcf), uint64(n))
	default:
		return binary.BigEndian.AppendUint64(append(b, 0xd3), uint64(n))
	}
}

func cabeca(b []byte, n int, fix, c16, c32 byte) []byte {
	switch {
	case n < 16:
		return append(b, fix|byte(n))
	case n < 1<<16:
		return binary.BigEndian.AppendUint16(append(b, c16), uint16(n))
	default:
		return binary.BigEndian.AppendUint32(append(b, c32), uint32(n))
	}
}

var errCurto = errors.New("msgpack: mensagem cortada")

// Desempacotar lê um valor MessagePack. Mapas viram map[string]any, listas
// []any, inteiros int64, decimais float64, bytes []byte.
func Desempacotar(b []byte) (any, error) {
	d := &leitor{b: b}
	v, err := d.valor(0)
	if err != nil {
		return nil, err
	}
	return v, nil
}

type leitor struct {
	b []byte
	i int
}

func (d *leitor) pega(n int) ([]byte, error) {
	if n < 0 || d.i+n > len(d.b) {
		return nil, errCurto
	}
	p := d.b[d.i : d.i+n]
	d.i += n
	return p, nil
}

func (d *leitor) uint(n int) (uint64, error) {
	p, err := d.pega(n)
	if err != nil {
		return 0, err
	}
	var v uint64
	for _, c := range p {
		v = v<<8 | uint64(c)
	}
	return v, nil
}

func (d *leitor) valor(prof int) (any, error) {
	if prof > 64 {
		return nil, errors.New("msgpack: aninhado demais")
	}
	p, err := d.pega(1)
	if err != nil {
		return nil, err
	}
	c := p[0]
	switch {
	case c <= 0x7f:
		return int64(c), nil
	case c >= 0xe0:
		return int64(int8(c)), nil
	case c&0xe0 == 0xa0:
		s, err := d.pega(int(c & 0x1f))
		return string(s), err
	case c&0xf0 == 0x90:
		return d.lista(int(c&0x0f), prof)
	case c&0xf0 == 0x80:
		return d.mapa(int(c&0x0f), prof)
	}
	switch c {
	case 0xc0:
		return nil, nil
	case 0xc2:
		return false, nil
	case 0xc3:
		return true, nil
	case 0xc4, 0xc5, 0xc6, 0xd9, 0xda, 0xdb:
		tam := map[byte]int{0xc4: 1, 0xc5: 2, 0xc6: 4, 0xd9: 1, 0xda: 2, 0xdb: 4}[c]
		n, err := d.uint(tam)
		if err != nil {
			return nil, err
		}
		s, err := d.pega(int(n))
		if err != nil {
			return nil, err
		}
		if c >= 0xd9 {
			return string(s), nil
		}
		return append([]byte(nil), s...), nil
	case 0xcc, 0xcd, 0xce, 0xcf:
		n, err := d.uint(1 << (c - 0xcc))
		return int64(n), err
	case 0xd0, 0xd1, 0xd2, 0xd3:
		tam := 1 << (c - 0xd0)
		n, err := d.uint(tam)
		if err != nil {
			return nil, err
		}
		shift := 64 - 8*tam
		return int64(n<<shift) >> shift, nil
	case 0xca:
		n, err := d.uint(4)
		return float64(math.Float32frombits(uint32(n))), err
	case 0xcb:
		n, err := d.uint(8)
		return math.Float64frombits(n), err
	case 0xdc, 0xdd:
		n, err := d.uint(2 << (c - 0xdc))
		if err != nil {
			return nil, err
		}
		return d.lista(int(n), prof)
	case 0xde, 0xdf:
		n, err := d.uint(2 << (c - 0xde))
		if err != nil {
			return nil, err
		}
		return d.mapa(int(n), prof)
	case 0xd4, 0xd5, 0xd6, 0xd7, 0xd8, 0xc7, 0xc8, 0xc9: // extensões: pula
		var n int
		switch c {
		case 0xc7, 0xc8, 0xc9:
			t, err := d.uint(1 << (c - 0xc7))
			if err != nil {
				return nil, err
			}
			n = int(t)
		default:
			n = 1 << (c - 0xd4)
		}
		_, err := d.pega(n + 1)
		return nil, err
	}
	return nil, fmt.Errorf("msgpack: byte 0x%x desconhecido", c)
}

func (d *leitor) lista(n, prof int) (any, error) {
	if n > len(d.b)-d.i {
		return nil, errCurto
	}
	l := make([]any, n)
	for i := range l {
		v, err := d.valor(prof + 1)
		if err != nil {
			return nil, err
		}
		l[i] = v
	}
	return l, nil
}

func (d *leitor) mapa(n, prof int) (any, error) {
	if n > len(d.b)-d.i {
		return nil, errCurto
	}
	m := make(map[string]any, n)
	for i := 0; i < n; i++ {
		k, err := d.valor(prof + 1)
		if err != nil {
			return nil, err
		}
		v, err := d.valor(prof + 1)
		if err != nil {
			return nil, err
		}
		m[fmt.Sprint(k)] = v
	}
	return m, nil
}
