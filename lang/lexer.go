// Package lang implementa a linguagem Nepenthes: lexer, parser,
// compilador para bytecode e a máquina virtual.
package lang

import (
	"fmt"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"
)

type TokKind int

const (
	TEOF TokKind = iota
	TIdent
	TInt
	TFloat
	TString
	TKeyword
	TOp
)

type Token struct {
	Kind TokKind
	Lit  string // texto do operador, palavra-chave, identificador ou conteúdo da string
	Int  int64
	Flt  float64
	Line int
}

func (t Token) Is(kind TokKind, lit string) bool { return t.Kind == kind && t.Lit == lit }

func (t Token) String() string {
	switch t.Kind {
	case TEOF:
		return "o fim do arquivo"
	case TString:
		return strconv.Quote(t.Lit)
	default:
		return "'" + t.Lit + "'"
	}
}

var keywords = map[string]bool{
	"let": true, "fn": true, "do": true, "end": true, "if": true, "elif": true, "else": true,
	"for": true, "in": true, "while": true, "return": true, "true": true, "false": true, "nil": true,
	"and": true, "or": true, "not": true, "break": true, "continue": true, "try": true, "catch": true,
	"import": true, "as": true,
}

// SyntaxError é um erro de leitura do código. Incomplete indica que o código
// terminou no meio de uma construção (usado pelo modo interativo).
type SyntaxError struct {
	File       string
	Line       int
	Msg        string
	Incomplete bool
}

func (e *SyntaxError) Error() string {
	if e.File != "" {
		return fmt.Sprintf("%s:%d: erro de sintaxe: %s", e.File, e.Line, e.Msg)
	}
	return fmt.Sprintf("linha %d: erro de sintaxe: %s", e.Line, e.Msg)
}

var ops3 = []string{"..."}
var ops2 = []string{"|>", "==", "!=", "<=", ">=", "//", "+=", "-=", "*=", "/="}

func Lex(src, file string) ([]Token, error) {
	var toks []Token
	line := 1
	i := 0
	n := len(src)
	errAt := func(l int, msg string, inc bool) error {
		return &SyntaxError{File: file, Line: l, Msg: msg, Incomplete: inc}
	}
	for i < n {
		c := src[i]
		switch {
		case c == '\n':
			line++
			i++
			continue
		case c == ' ' || c == '\t' || c == '\r':
			i++
			continue
		case c == '#' || (c == '-' && i+1 < n && src[i+1] == '-'):
			for i < n && src[i] != '\n' {
				i++
			}
			continue
		}
		if c >= '0' && c <= '9' {
			j := i
			isFloat := false
			for j < n && (isDigit(src[j]) || src[j] == '_') {
				j++
			}
			if j+1 < n && src[j] == '.' && isDigit(src[j+1]) {
				isFloat = true
				j++
				for j < n && (isDigit(src[j]) || src[j] == '_') {
					j++
				}
			}
			if j < n && (src[j] == 'e' || src[j] == 'E') {
				k := j + 1
				if k < n && (src[k] == '+' || src[k] == '-') {
					k++
				}
				if k < n && isDigit(src[k]) {
					isFloat = true
					j = k
					for j < n && isDigit(src[j]) {
						j++
					}
				}
			}
			txt := strings.ReplaceAll(src[i:j], "_", "")
			if isFloat {
				f, err := strconv.ParseFloat(txt, 64)
				if err != nil {
					return nil, errAt(line, "número inválido: "+txt, false)
				}
				toks = append(toks, Token{Kind: TFloat, Lit: txt, Flt: f, Line: line})
			} else {
				v, err := strconv.ParseInt(txt, 10, 64)
				if err != nil {
					f, _ := strconv.ParseFloat(txt, 64)
					toks = append(toks, Token{Kind: TFloat, Lit: txt, Flt: f, Line: line})
				} else {
					toks = append(toks, Token{Kind: TInt, Lit: txt, Int: v, Line: line})
				}
			}
			i = j
			continue
		}
		if c == '_' || c >= 0x80 || unicode.IsLetter(rune(c)) {
			j := i
			for j < n {
				r, size := utf8.DecodeRuneInString(src[j:])
				if r == '_' || unicode.IsLetter(r) || unicode.IsDigit(r) {
					j += size
					continue
				}
				break
			}
			if j == i {
				r, _ := utf8.DecodeRuneInString(src[i:])
				return nil, errAt(line, fmt.Sprintf("caractere inesperado %q", r), false)
			}
			w := src[i:j]
			if keywords[w] {
				toks = append(toks, Token{Kind: TKeyword, Lit: w, Line: line})
			} else {
				toks = append(toks, Token{Kind: TIdent, Lit: w, Line: line})
			}
			i = j
			continue
		}
		if c == '"' || c == '\'' {
			q := c
			start := line
			var b strings.Builder
			j := i + 1
			for {
				if j >= n {
					return nil, errAt(start, "texto sem aspas de fechamento", true)
				}
				d := src[j]
				if d == q {
					j++
					break
				}
				if d == '\n' {
					line++
				}
				if d == '\\' && j+1 < n {
					e := src[j+1]
					j += 2
					switch e {
					case 'n':
						b.WriteByte('\n')
					case 't':
						b.WriteByte('\t')
					case 'r':
						b.WriteByte('\r')
					case '0':
						b.WriteByte(0)
					case '\\', '"', '\'':
						b.WriteByte(e)
					case 'u':
						if j+4 <= n {
							if v, err := strconv.ParseUint(src[j:j+4], 16, 32); err == nil {
								b.WriteRune(rune(v))
								j += 4
								continue
							}
						}
						return nil, errAt(line, "escape \\u inválido (use \\u00e9)", false)
					default:
						b.WriteByte('\\')
						b.WriteByte(e)
					}
					continue
				}
				b.WriteByte(d)
				j++
			}
			toks = append(toks, Token{Kind: TString, Lit: b.String(), Line: start})
			i = j
			continue
		}
		matched := false
		for _, group := range [][]string{ops3, ops2} {
			for _, op := range group {
				if strings.HasPrefix(src[i:], op) {
					toks = append(toks, Token{Kind: TOp, Lit: op, Line: line})
					i += len(op)
					matched = true
					break
				}
			}
			if matched {
				break
			}
		}
		if matched {
			continue
		}
		if strings.IndexByte("+-*/%<>=.,:()[]{}", c) >= 0 {
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

func isDigit(c byte) bool { return c >= '0' && c <= '9' }
