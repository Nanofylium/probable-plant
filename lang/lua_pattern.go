package lang

import (
	"fmt"
	"strings"
)

// Padrões de texto do Lua (string.find, match, gmatch, gsub), reescritos em Go
// seguindo as regras do manual do Lua 5.4.

const (
	patMaxCaptures = 32
	capUnfinished  = -1
	capPosition    = -2
	patMaxDepth    = 200
)

type patState struct {
	src, pat string
	level    int
	capture  [patMaxCaptures]struct{ init, len int }
	depth    int
	err      error
}

func (ms *patState) fail(format string, a ...any) int {
	if ms.err == nil {
		ms.err = fmt.Errorf(format, a...)
	}
	return -1
}

func classEnd(ms *patState, p int) int {
	if p >= len(ms.pat) {
		return ms.fail("padrão malformado (termina com '%%')")
	}
	c := ms.pat[p]
	p++
	if c == '%' {
		if p >= len(ms.pat) {
			return ms.fail("padrão malformado (termina com '%%')")
		}
		return p + 1
	}
	if c == '[' {
		if p < len(ms.pat) && ms.pat[p] == '^' {
			p++
		}
		for { // procura o ]
			if p >= len(ms.pat) {
				return ms.fail("padrão malformado (falta ']')")
			}
			cc := ms.pat[p]
			p++
			if cc == '%' {
				if p >= len(ms.pat) {
					return ms.fail("padrão malformado")
				}
				p++
			}
			if p < len(ms.pat) && ms.pat[p] == ']' {
				return p + 1
			}
			if p >= len(ms.pat) {
				return ms.fail("padrão malformado (falta ']')")
			}
		}
	}
	return p
}

func isAlpha(c byte) bool { return (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') }

func singleClass(c, cl byte) bool {
	var res bool
	switch cl | 0x20 {
	case 'a':
		res = isAlpha(c)
	case 'c':
		res = c < 32 || c == 127
	case 'd':
		res = c >= '0' && c <= '9'
	case 'g':
		res = c > 32 && c < 127
	case 'l':
		res = c >= 'a' && c <= 'z'
	case 'p':
		res = (c >= 33 && c <= 47) || (c >= 58 && c <= 64) || (c >= 91 && c <= 96) || (c >= 123 && c <= 126)
	case 's':
		res = c == ' ' || (c >= '\t' && c <= '\r')
	case 'u':
		res = c >= 'A' && c <= 'Z'
	case 'w':
		res = isAlpha(c) || (c >= '0' && c <= '9')
	case 'x':
		res = isHex(c)
	default:
		return cl == c
	}
	if cl >= 'A' && cl <= 'Z' {
		return !res
	}
	return res
}

func matchBracketClass(ms *patState, c byte, p, ec int) bool {
	sig := true
	if ms.pat[p+1] == '^' {
		sig = false
		p++
	}
	for p++; p < ec; p++ {
		if ms.pat[p] == '%' {
			p++
			if singleClass(c, ms.pat[p]) {
				return sig
			}
		} else if p+2 < ec && ms.pat[p+1] == '-' {
			if ms.pat[p] <= c && c <= ms.pat[p+2] {
				return sig
			}
			p += 2
		} else if ms.pat[p] == c {
			return sig
		}
	}
	return !sig
}

func singleMatch(ms *patState, s, p, ep int) bool {
	if s >= len(ms.src) {
		return false
	}
	c := ms.src[s]
	switch ms.pat[p] {
	case '.':
		return true
	case '%':
		return singleClass(c, ms.pat[p+1])
	case '[':
		return matchBracketClass(ms, c, p, ep-1)
	}
	return ms.pat[p] == c
}

func (ms *patState) match(s, p int) int {
	ms.depth++
	defer func() { ms.depth-- }()
	if ms.depth > patMaxDepth {
		return ms.fail("padrão complexo demais")
	}
	for {
		if ms.err != nil {
			return -1
		}
		if p >= len(ms.pat) {
			return s
		}
		switch ms.pat[p] {
		case '(':
			if p+1 < len(ms.pat) && ms.pat[p+1] == ')' {
				return ms.startCapture(s, p+2, capPosition)
			}
			return ms.startCapture(s, p+1, capUnfinished)
		case ')':
			return ms.endCapture(s, p+1)
		case '$':
			if p+1 == len(ms.pat) {
				if s == len(ms.src) {
					return s
				}
				return -1
			}
		case '%':
			if p+1 < len(ms.pat) {
				switch ms.pat[p+1] {
				case 'b':
					s = ms.matchBalance(s, p+2)
					if s != -1 {
						p += 4
						continue
					}
					return -1
				case 'f':
					p += 2
					if p >= len(ms.pat) || ms.pat[p] != '[' {
						return ms.fail("falta '[' depois de '%%f' no padrão")
					}
					ep := classEnd(ms, p)
					if ep < 0 {
						return -1
					}
					var prev, cur byte
					if s > 0 {
						prev = ms.src[s-1]
					}
					if s < len(ms.src) {
						cur = ms.src[s]
					}
					if !matchBracketClass(ms, prev, p, ep-1) && matchBracketClass(ms, cur, p, ep-1) {
						p = ep
						continue
					}
					return -1
				}
				if c := ms.pat[p+1]; c >= '0' && c <= '9' {
					s = ms.matchCapture(s, c)
					if s != -1 {
						p += 2
						continue
					}
					return -1
				}
			}
		}
		ep := classEnd(ms, p)
		if ep < 0 {
			return -1
		}
		var epc byte
		if ep < len(ms.pat) {
			epc = ms.pat[ep]
		}
		if !singleMatch(ms, s, p, ep) {
			if epc == '*' || epc == '?' || epc == '-' {
				p = ep + 1
				continue
			}
			return -1
		}
		switch epc {
		case '?':
			if res := ms.match(s+1, ep+1); res != -1 {
				return res
			}
			p = ep + 1
			continue
		case '+':
			return ms.maxExpand(s+1, p, ep)
		case '*':
			return ms.maxExpand(s, p, ep)
		case '-':
			return ms.minExpand(s, p, ep)
		}
		s++
		p = ep
	}
}

func (ms *patState) maxExpand(s, p, ep int) int {
	i := 0
	for singleMatch(ms, s+i, p, ep) {
		i++
	}
	for i >= 0 {
		if res := ms.match(s+i, ep+1); res != -1 {
			return res
		}
		if ms.err != nil {
			return -1
		}
		i--
	}
	return -1
}

func (ms *patState) minExpand(s, p, ep int) int {
	for {
		if res := ms.match(s, ep+1); res != -1 {
			return res
		}
		if ms.err != nil {
			return -1
		}
		if singleMatch(ms, s, p, ep) {
			s++
		} else {
			return -1
		}
	}
}

func (ms *patState) startCapture(s, p, what int) int {
	if ms.level >= patMaxCaptures {
		return ms.fail("capturas demais no padrão")
	}
	ms.capture[ms.level].init = s
	ms.capture[ms.level].len = what
	ms.level++
	res := ms.match(s, p)
	if res == -1 {
		ms.level--
	}
	return res
}

func (ms *patState) endCapture(s, p int) int {
	l := -1
	for i := ms.level - 1; i >= 0; i-- {
		if ms.capture[i].len == capUnfinished {
			l = i
			break
		}
	}
	if l < 0 {
		return ms.fail("padrão com ')' sem '('")
	}
	ms.capture[l].len = s - ms.capture[l].init
	res := ms.match(s, p)
	if res == -1 {
		ms.capture[l].len = capUnfinished
	}
	return res
}

func (ms *patState) matchBalance(s, p int) int {
	if p+1 >= len(ms.pat) {
		return ms.fail("faltam argumentos para '%%b' no padrão")
	}
	if s >= len(ms.src) || ms.src[s] != ms.pat[p] {
		return -1
	}
	b, e := ms.pat[p], ms.pat[p+1]
	cont := 1
	for i := s + 1; i < len(ms.src); i++ {
		switch ms.src[i] {
		case e:
			cont--
			if cont == 0 {
				return i + 1
			}
		case b:
			cont++
		}
	}
	return -1
}

func (ms *patState) matchCapture(s int, c byte) int {
	l := int(c - '1')
	if l < 0 || l >= ms.level || ms.capture[l].len == capUnfinished {
		return ms.fail("referência de captura inválida %%%d", l+1)
	}
	cap := ms.src[ms.capture[l].init : ms.capture[l].init+ms.capture[l].len]
	if len(ms.src)-s >= len(cap) && ms.src[s:s+len(cap)] == cap {
		return s + len(cap)
	}
	return -1
}

// getCapture devolve a captura i (ou o trecho inteiro se não há capturas).
func (ms *patState) getCapture(i, s, e int) Value {
	if i >= ms.level {
		if i == 0 {
			return ms.src[s:e]
		}
		ms.fail("índice de captura inválido %%%d", i+1)
		return nil
	}
	c := ms.capture[i]
	if c.len == capPosition {
		return int64(c.init + 1)
	}
	if c.len == capUnfinished {
		ms.fail("captura não terminada")
		return nil
	}
	return ms.src[c.init : c.init+c.len]
}

func (ms *patState) captures(s, e int, wholeIfNone bool) []Value {
	n := ms.level
	if n == 0 && wholeIfNone {
		n = 1
	}
	out := make([]Value, n)
	for i := 0; i < n; i++ {
		out[i] = ms.getCapture(i, s, e)
	}
	return out
}

// patFind procura pat em src a partir do byte init (0-based). Devolve início,
// fim e o estado (para ler as capturas); início -1 se não achou.
func patFind(src, pat string, init int) (int, int, *patState, error) {
	ms := &patState{src: src, pat: pat}
	anchor := strings.HasPrefix(pat, "^")
	p := 0
	if anchor {
		p = 1
	}
	s := init
	for {
		ms.level = 0
		ms.depth = 0
		if e := ms.match(s, p); e != -1 {
			return s, e, ms, nil
		}
		if ms.err != nil {
			return -1, -1, ms, ms.err
		}
		s++
		if anchor || s > len(src) {
			return -1, -1, ms, nil
		}
	}
}

func hasSpecials(p string) bool { return strings.ContainsAny(p, "^$*+?.([%-") }

// strStart converte o índice Lua (1-based, negativo conta do fim) em 0-based.
func strStart(i int64, n int) int {
	switch {
	case i > 0:
		return int(min(i-1, int64(n)+1))
	case i == 0:
		return 0
	case i < -int64(n):
		return 0
	}
	return n + int(i)
}

// patGsub troca as ocorrências; repl pode ser texto, tabela ou função.
func (vm *VM) patGsub(src, pat string, repl Value, maxN int64) (string, int64, error) {
	switch repl.(type) {
	case string, int64, float64, *Table, *Closure, *Native:
	default:
		return "", 0, fmt.Errorf("gsub: o argumento 3 precisa ser texto, tabela ou função (recebeu %s)", luaTypeName(repl))
	}
	anchor := strings.HasPrefix(pat, "^")
	p := 0
	if anchor {
		p = 1
	}
	var b strings.Builder
	s, lastmatch := 0, -1
	var n int64
	ms := &patState{src: src, pat: pat}
	for maxN < 0 || n < maxN {
		ms.level, ms.depth = 0, 0
		e := ms.match(s, p)
		if ms.err != nil {
			return "", 0, ms.err
		}
		if e != -1 && e != lastmatch {
			n++
			if err := vm.gsubValue(&b, ms, s, e, repl); err != nil {
				return "", 0, err
			}
			s, lastmatch = e, e
		} else if s < len(src) {
			b.WriteByte(src[s])
			s++
		} else {
			break
		}
		if anchor {
			break
		}
	}
	b.WriteString(src[s:])
	return b.String(), n, nil
}

func (vm *VM) gsubValue(b *strings.Builder, ms *patState, s, e int, repl Value) error {
	whole := ms.src[s:e]
	var rv Value
	switch r := repl.(type) {
	case string, int64, float64:
		rs, _ := luaNumStr(r)
		for i := 0; i < len(rs); i++ {
			c := rs[i]
			if c != '%' {
				b.WriteByte(c)
				continue
			}
			i++
			if i >= len(rs) {
				return fmt.Errorf("uso inválido de '%%' no texto de troca")
			}
			d := rs[i]
			switch {
			case d == '%':
				b.WriteByte('%')
			case d == '0':
				b.WriteString(whole)
			case d >= '1' && d <= '9':
				v := ms.getCapture(int(d-'1'), s, e)
				if ms.err != nil {
					return ms.err
				}
				vs, _ := luaNumStr(v)
				b.WriteString(vs)
			default:
				return fmt.Errorf("uso inválido de '%%' no texto de troca")
			}
		}
		return nil
	case *Table:
		k := ms.getCapture(0, s, e)
		if ms.err != nil {
			return ms.err
		}
		v, err := vm.luaIndex(r, k)
		if err != nil {
			return err
		}
		rv = v
	default:
		caps := ms.captures(s, e, true)
		if ms.err != nil {
			return ms.err
		}
		v, err := vm.Call(r, caps)
		if err != nil {
			return err
		}
		rv = v
	}
	if !luaTruthy(rv) {
		b.WriteString(whole)
		return nil
	}
	rs, ok := luaNumStr(rv)
	if !ok {
		return fmt.Errorf("valor de troca inválido (um %s)", luaTypeName(rv))
	}
	b.WriteString(rs)
	return nil
}
