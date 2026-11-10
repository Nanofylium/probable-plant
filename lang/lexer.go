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
		if c == '`' {
			// texto com interpolação: `Olá, {nome}!` vira ("" + "Olá, " + __str(nome) + "!")
			start := line
			parts, j, nl, err := lexTemplate(src, i+1, line, file)
			if err != nil {
				return nil, err
			}
			toks = append(toks, Token{Kind: TOp, Lit: "(", Line: start}, Token{Kind: TString, Lit: "", Line: start})
			for _, p := range parts {
				toks = append(toks, Token{Kind: TOp, Lit: "+", Line: start})
				toks = append(toks, p...)
			}
			toks = append(toks, Token{Kind: TOp, Lit: ")", Line: start})
			line = nl
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

// lexTemplate lê um texto entre crases a partir de i (depois da crase de abertura).
// Devolve as partes já como tokens: textos fixos e __str(expressão).
func lexTemplate(src string, i, line int, file string) (parts [][]Token, end, endLine int, err error) {
	n := len(src)
	start := line
	var b strings.Builder
	flush := func() {
		if b.Len() > 0 {
			parts = append(parts, []Token{{Kind: TString, Lit: b.String(), Line: start}})
			b.Reset()
		}
	}
	for {
		if i >= n {
			return nil, 0, 0, &SyntaxError{File: file, Line: start, Msg: "texto com ` sem a crase de fechamento", Incomplete: true}
		}
		d := src[i]
		switch {
		case d == '`':
			flush()
			return parts, i + 1, line, nil
		case d == '\\' && i+1 < n:
			e := src[i+1]
			i += 2
			switch e {
			case 'n':
				b.WriteByte('\n')
			case 't':
				b.WriteByte('\t')
			case '`', '\\', '{', '}':
				b.WriteByte(e)
			default:
				b.WriteByte('\\')
				b.WriteByte(e)
			}
		case d == '{' && i+1 < n && src[i+1] == '{':
			b.WriteByte('{')
			i += 2
		case d == '}' && i+1 < n && src[i+1] == '}':
			b.WriteByte('}')
			i += 2
		case d == '{':
			flush()
			exprLine := line
			j, depth := i+1, 1
			var q byte
			for ; j < n; j++ {
				c := src[j]
				if c == '\n' {
					line++
				}
				if q != 0 {
					if c == '\\' {
						j++
					} else if c == q {
						q = 0
					}
					continue
				}
				if c == '"' || c == '\'' {
					q = c
				} else if c == '{' {
					depth++
				} else if c == '}' {
					depth--
					if depth == 0 {
						break
					}
				} else if c == '`' {
					return nil, 0, 0, &SyntaxError{File: file, Line: exprLine, Msg: "{ sem } dentro do texto com crases"}
				}
			}
			if j >= n {
				return nil, 0, 0, &SyntaxError{File: file, Line: exprLine, Msg: "{ sem } dentro do texto com crases", Incomplete: true}
			}
			expr := src[i+1 : j]
			if strings.TrimSpace(expr) == "" {
				return nil, 0, 0, &SyntaxError{File: file, Line: exprLine, Msg: "{} vazio dentro do texto com crases (use {{ para escrever {)"}
			}
			et, lerr := Lex(expr, file)
			if lerr != nil {
				if se, ok := lerr.(*SyntaxError); ok {
					se.Line += exprLine - 1
				}
				return nil, 0, 0, lerr
			}
			et = et[:len(et)-1] // sem o TEOF
			for k := range et {
				et[k].Line += exprLine - 1
			}
			p := []Token{{Kind: TIdent, Lit: "__str", Line: start}, {Kind: TOp, Lit: "(", Line: start}}
			p = append(p, et...)
			p = append(p, Token{Kind: TOp, Lit: ")", Line: start})
			parts = append(parts, p)
			i = j + 1
		default:
			if d == '\n' {
				line++
			}
			b.WriteByte(d)
			i++
		}
	}
}
