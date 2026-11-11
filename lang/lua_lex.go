package lang

// Leitura de código Lua (5.4). Os tokens são os mesmos do Nepenthes; muda o
// conjunto de palavras-chave, os operadores, os comentários e os textos longos.
//
// O suporte a Lua segue a linguagem Lua 5.4 (https://www.lua.org), de Roberto
// Ierusalimschy, Waldemar Celes e Luiz Henrique de Figueiredo (PUC-Rio),
// distribuída sob a licença MIT. Esta é uma implementação própria em Go: nenhum
// código C do Lua foi incorporado. Veja o aviso completo em LICENCA-LUA.txt.

import (
	"fmt"
	"math"
	"strconv"
	"strings"
	"unicode/utf8"
)

var luaKeywords = map[string]bool{
	"and": true, "break": true, "do": true, "else": true, "elseif": true, "end": true, "false": true,
	"for": true, "function": true, "goto": true, "if": true, "in": true, "local": true, "nil": true,
	"not": true, "or": true, "repeat": true, "return": true, "then": true, "true": true, "until": true, "while": true,
}

var luaOps = []string{"...", "..", "==", "~=", "<=", ">=", "<<", ">>", "//", "::"}

// LexLua divide um código Lua em tokens.
func LexLua(src, file string) ([]Token, error) {
	var toks []Token
	line := 1
	i, n := 0, len(src)
	errAt := func(l int, msg string, inc bool) error {
		return &SyntaxError{File: file, Line: l, Msg: msg, Incomplete: inc}
	}
	if strings.HasPrefix(src, "#") { // linha #!/usr/bin/env ...
		for i < n && src[i] != '\n' {
			i++
		}
	}
	for i < n {
		c := src[i]
		switch {
		case c == '\n':
			line++
			i++
			continue
		case c == ' ' || c == '\t' || c == '\r' || c == '\f' || c == '\v':
			i++
			continue
		case c == '-' && i+1 < n && src[i+1] == '-':
			i += 2
			if lvl, ok := longOpen(src, i); ok {
				start := line
				body, j, nl, found := longBody(src, i+lvl+2, lvl, line)
				_ = body
				if !found {
					return nil, errAt(start, "comentário longo sem fechamento", true)
				}
				i, line = j, nl
				continue
			}
			for i < n && src[i] != '\n' {
				i++
			}
			continue
		}
		if isDigit(c) || (c == '.' && i+1 < n && isDigit(src[i+1])) {
			tok, j, err := luaNumber(src, i)
			if err != nil {
				return nil, errAt(line, err.Error(), false)
			}
			tok.Line = line
			toks = append(toks, tok)
			i = j
			continue
		}
		if c == '_' || (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || c >= 0x80 {
			j := i
			for j < n {
				d := src[j]
				if d == '_' || isDigit(d) || (d >= 'a' && d <= 'z') || (d >= 'A' && d <= 'Z') {
					j++
					continue
				}
				if d >= 0x80 { // letras acentuadas também valem em nomes
					_, size := utf8.DecodeRuneInString(src[j:])
					j += size
					continue
				}
				break
			}
			w := src[i:j]
			if luaKeywords[w] {
				toks = append(toks, Token{Kind: TKeyword, Lit: w, Line: line})
			} else {
				toks = append(toks, Token{Kind: TIdent, Lit: w, Line: line})
			}
			i = j
			continue
		}
		if c == '[' {
			if lvl, ok := longOpen(src, i); ok {
				start := line
				body, j, nl, found := longBody(src, i+lvl+2, lvl, line)
				if !found {
					return nil, errAt(start, "texto longo sem fechamento", true)
				}
				toks = append(toks, Token{Kind: TString, Lit: body, Line: start})
				i, line = j, nl
				continue
			}
		}
		if c == '"' || c == '\'' {
			s, j, nl, err := luaQuoted(src, i, line)
			if err != nil {
				return nil, errAt(line, err.Error(), j >= n)
			}
			toks = append(toks, Token{Kind: TString, Lit: s, Line: line})
			i, line = j, nl
			continue
		}
		matched := false
		for _, op := range luaOps {
			if strings.HasPrefix(src[i:], op) {
				toks = append(toks, Token{Kind: TOp, Lit: op, Line: line})
				i += len(op)
				matched = true
				break
			}
		}
		if matched {
			continue
		}
		if strings.IndexByte("+-*/%^#&~|<>=(){}[];:,.", c) >= 0 {
			toks = append(toks, Token{Kind: TOp, Lit: string(c), Line: line})
			i++
			continue
		}
		r, _ := utf8.DecodeRuneInString(src[i:])
		return nil, errAt(line, fmt.Sprintf("caractere inesperado %q", r), false)
	}
	toks = append(toks, Token{Kind: TEOF, Line: line})
	return toks, nil
}

// longOpen reconhece [[ ou [==[ em i e devolve o nível (número de =).
func longOpen(src string, i int) (int, bool) {
	if i >= len(src) || src[i] != '[' {
		return 0, false
	}
	j := i + 1
	for j < len(src) && src[j] == '=' {
		j++
	}
	if j < len(src) && src[j] == '[' {
		return j - i - 1, true
	}
	return 0, false
}

func longBody(src string, i, lvl, line int) (string, int, int, bool) {
	closer := "]" + strings.Repeat("=", lvl) + "]"
	// a primeira quebra de linha logo após a abertura é ignorada
	if i < len(src) && src[i] == '\r' {
		i++
	}
	if i < len(src) && src[i] == '\n' {
		i++
		line++
	}
	k := strings.Index(src[i:], closer)
	if k < 0 {
		return "", len(src), line, false
	}
	body := src[i : i+k]
	line += strings.Count(body, "\n")
	return body, i + k + len(closer), line, true
}

func luaQuoted(src string, i, line int) (string, int, int, error) {
	q := src[i]
	n := len(src)
	var b strings.Builder
	j := i + 1
	for {
		if j >= n {
			return "", j, line, fmt.Errorf("texto sem aspas de fechamento")
		}
		d := src[j]
		if d == q {
			return b.String(), j + 1, line, nil
		}
		if d == '\n' {
			return "", j, line, fmt.Errorf("texto sem aspas de fechamento")
		}
		if d != '\\' {
			b.WriteByte(d)
			j++
			continue
		}
		if j+1 >= n {
			return "", n, line, fmt.Errorf("texto sem aspas de fechamento")
		}
		e := src[j+1]
		j += 2
		switch e {
		case 'n':
			b.WriteByte('\n')
		case 't':
			b.WriteByte('\t')
		case 'r':
			b.WriteByte('\r')
		case 'a':
			b.WriteByte('\a')
		case 'b':
			b.WriteByte('\b')
		case 'f':
			b.WriteByte('\f')
		case 'v':
			b.WriteByte('\v')
		case '\\', '"', '\'':
			b.WriteByte(e)
		case '\n':
			b.WriteByte('\n')
			line++
		case 'z':
			for j < n && (src[j] == ' ' || src[j] == '\n' || src[j] == '\t' || src[j] == '\r') {
				if src[j] == '\n' {
					line++
				}
				j++
			}
		case 'x':
			if j+2 > n {
				return "", j, line, fmt.Errorf("escape \\x inválido")
			}
			v, err := strconv.ParseUint(src[j:j+2], 16, 8)
			if err != nil {
				return "", j, line, fmt.Errorf("escape \\x inválido")
			}
			b.WriteByte(byte(v))
			j += 2
		case 'u':
			if j >= n || src[j] != '{' {
				return "", j, line, fmt.Errorf("escape \\u inválido (use \\u{e9})")
			}
			k := strings.IndexByte(src[j:], '}')
			if k < 0 {
				return "", j, line, fmt.Errorf("escape \\u inválido (use \\u{e9})")
			}
			v, err := strconv.ParseUint(src[j+1:j+k], 16, 32)
			if err != nil {
				return "", j, line, fmt.Errorf("escape \\u inválido (use \\u{e9})")
			}
			b.WriteString(string(rune(v)))
			j += k + 1
		default:
			if isDigit(e) {
				k := j - 1
				end := k
				for end < n && end < k+3 && isDigit(src[end]) {
					end++
				}
				v, _ := strconv.Atoi(src[k:end])
				if v > 255 {
					return "", j, line, fmt.Errorf("escape decimal grande demais")
				}
				b.WriteByte(byte(v))
				j = end
				continue
			}
			return "", j, line, fmt.Errorf("escape inválido \\%c", e)
		}
	}
}

func isHex(c byte) bool {
	return isDigit(c) || (c >= 'a' && c <= 'f') || (c >= 'A' && c <= 'F')
}

func luaNumber(src string, i int) (Token, int, error) {
	n := len(src)
	j := i
	if src[j] == '0' && j+1 < n && (src[j+1] == 'x' || src[j+1] == 'X') {
		j += 2
		for j < n && (isHex(src[j]) || src[j] == '.' || ((src[j] == 'p' || src[j] == 'P') && j+1 < n) ||
			((src[j] == '+' || src[j] == '-') && (src[j-1] == 'p' || src[j-1] == 'P'))) {
			j++
		}
	} else {
		for j < n && (isDigit(src[j]) || src[j] == '.' || src[j] == 'e' || src[j] == 'E' ||
			((src[j] == '+' || src[j] == '-') && (src[j-1] == 'e' || src[j-1] == 'E'))) {
			j++
		}
	}
	txt := src[i:j]
	v, ok := parseLuaNumber(txt)
	if !ok {
		return Token{}, j, fmt.Errorf("número inválido: %s", txt)
	}
	if f, isF := v.(float64); isF {
		return Token{Kind: TFloat, Lit: txt, Flt: f}, j, nil
	}
	return Token{Kind: TInt, Lit: txt, Int: v.(int64)}, j, nil
}

// parseLuaNumber converte um numeral Lua (decimal ou hexadecimal) em int64 ou float64.
func parseLuaNumber(s string) (Value, bool) {
	if s == "" {
		return nil, false
	}
	neg := false
	t := s
	if t[0] == '-' || t[0] == '+' {
		neg = t[0] == '-'
		t = t[1:]
	}
	if t == "" {
		return nil, false
	}
	low := strings.ToLower(t)
	if strings.HasPrefix(low, "0x") {
		h := low[2:]
		if h == "" {
			return nil, false
		}
		if !strings.ContainsAny(h, ".p") {
			var u uint64
			for k := 0; k < len(h); k++ {
				if !isHex(h[k]) {
					return nil, false
				}
				d, _ := strconv.ParseUint(h[k:k+1], 16, 8)
				u = u<<4 | d // inteiros hexadecimais dão a volta, como no Lua
			}
			v := int64(u)
			if neg {
				v = -v
			}
			return v, true
		}
		if !strings.Contains(h, "p") {
			h += "p0"
		}
		f, err := strconv.ParseFloat("0x"+h, 64)
		if err != nil {
			return nil, false
		}
		if neg {
			f = -f
		}
		return f, true
	}
	for k := 0; k < len(low); k++ {
		c := low[k]
		if !(isDigit(c) || c == '.' || c == 'e' || c == '+' || c == '-') {
			return nil, false
		}
	}
	if low == "inf" || low == "nan" || strings.HasPrefix(low, "e") || strings.HasPrefix(low, ".e") || low == "." {
		return nil, false
	}
	if !strings.ContainsAny(low, ".e") {
		if v, err := strconv.ParseInt(low, 10, 64); err == nil {
			if neg {
				v = -v
			}
			return v, true
		}
	}
	f, err := strconv.ParseFloat(low, 64)
	if err != nil && !(math.IsInf(f, 0)) {
		return nil, false
	}
	if neg {
		f = -f
	}
	return f, true
}
