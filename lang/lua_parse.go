package lang

// Árvore e leitor de código Lua 5.4.

type luaExpr interface{}
type luaStmt interface{}

type (
	lConst  struct{ Val Value }
	lVararg struct{ Line int }
	lName   struct {
		Name string
		Line int
	}
	lIndex struct {
		Obj, Key luaExpr
		Line     int
	}
	lCall struct {
		Fn   luaExpr
		Args []luaExpr
		Line int
	}
	lMethod struct {
		Obj  luaExpr
		Name string
		Args []luaExpr
		Line int
	}
	lFunc struct {
		Name   string
		Params []string
		Vararg bool
		Body   []luaStmt
		Line   int
	}
	lBin struct {
		Op   string
		L, R luaExpr
		Line int
	}
	lUn struct {
		Op   string
		X    luaExpr
		Line int
	}
	lLogic struct {
		Op   string
		L, R luaExpr
	}
	lField struct {
		Key luaExpr // nil = posicional
		Val luaExpr
	}
	lTable struct {
		Items []lField
		Line  int
	}
	lParen struct{ X luaExpr }
)

type (
	lLocal struct {
		Names []string
		Vals  []luaExpr
		Line  int
	}
	lAssign struct {
		Targets []luaExpr
		Vals    []luaExpr
		Line    int
	}
	lCallStmt struct {
		Call luaExpr
		Line int
	}
	lDo    struct{ Body []luaStmt }
	lWhile struct {
		Cond luaExpr
		Body []luaStmt
		Line int
	}
	lRepeat struct {
		Body []luaStmt
		Cond luaExpr
		Line int
	}
	lIf struct {
		Conds  []luaExpr
		Blocks [][]luaStmt
		Else   []luaStmt
		Line   int
	}
	lNumFor struct {
		Var                string
		Start, Limit, Step luaExpr
		Body               []luaStmt
		Line               int
	}
	lGenFor struct {
		Names []string
		Exprs []luaExpr
		Body  []luaStmt
		Line  int
	}
	lLocalFunc struct {
		Name string
		Fn   *lFunc
		Line int
	}
	lReturn struct {
		Vals []luaExpr
		Line int
	}
	lBreak struct{ Line int }
	lGoto  struct {
		Label string
		Line  int
	}
	lLabel struct {
		Name string
		Line int
	}
)

type luaParser struct{ parser }

// ParseLua lê um programa Lua inteiro.
func ParseLua(src, file string) (prog []luaStmt, err error) {
	toks, lerr := LexLua(src, file)
	if lerr != nil {
		return nil, lerr
	}
	ps := &luaParser{parser{toks: toks, file: file}}
	defer func() {
		if r := recover(); r != nil {
			if b, ok := r.(parseBail); ok {
				prog, err = nil, b.err
				return
			}
			panic(r)
		}
	}()
	prog = ps.block()
	if t := ps.peek(); t.Kind != TEOF {
		ps.fail(t, "não esperava "+t.String()+" aqui")
	}
	return prog, nil
}

func (ps *luaParser) blockEnd() bool {
	t := ps.peek()
	if t.Kind == TEOF {
		return true
	}
	if t.Kind == TKeyword {
		switch t.Lit {
		case "end", "else", "elseif", "until":
			return true
		}
	}
	return false
}

func (ps *luaParser) block() []luaStmt {
	var out []luaStmt
	for !ps.blockEnd() {
		if ps.isKw("return") {
			t := ps.next()
			r := &lReturn{Line: t.Line}
			if !ps.blockEnd() && !ps.isOp(";") {
				r.Vals = ps.exprList()
			}
			ps.acceptOp(";")
			out = append(out, r)
			if !ps.blockEnd() {
				ps.fail(ps.peek(), "'return' precisa ser a última instrução do bloco")
			}
			return out
		}
		if s := ps.statement(); s != nil {
			out = append(out, s)
		}
	}
	return out
}

func (ps *luaParser) endBlock(ctx string) { ps.expectKw("end", ctx) }

func (ps *luaParser) statement() luaStmt {
	t := ps.peek()
	if t.Is(TOp, ";") {
		ps.next()
		return nil
	}
	if t.Is(TOp, "::") {
		ps.next()
		name := ps.expectIdent("o nome do rótulo").Lit
		ps.expectOp("::", " para fechar o rótulo")
		return &lLabel{Name: name, Line: t.Line}
	}
	if t.Kind == TKeyword {
		switch t.Lit {
		case "if":
			ps.next()
			st := &lIf{Line: t.Line}
			st.Conds = append(st.Conds, ps.expr())
			ps.expectKw("then", " depois da condição do if")
			st.Blocks = append(st.Blocks, ps.block())
			for ps.isKw("elseif") {
				ps.next()
				st.Conds = append(st.Conds, ps.expr())
				ps.expectKw("then", " depois da condição do elseif")
				st.Blocks = append(st.Blocks, ps.block())
			}
			if ps.acceptKw("else") {
				st.Else = ps.block()
				if st.Else == nil {
					st.Else = []luaStmt{}
				}
			}
			ps.endBlock(" para fechar o if")
			return st
		case "while":
			ps.next()
			cond := ps.expr()
			ps.expectKw("do", " depois da condição do while")
			body := ps.block()
			ps.endBlock(" para fechar o while")
			return &lWhile{Cond: cond, Body: body, Line: t.Line}
		case "do":
			ps.next()
			body := ps.block()
			ps.endBlock(" para fechar o do")
			return &lDo{Body: body}
		case "for":
			ps.next()
			n1 := ps.expectIdent("um nome de variável depois de 'for'").Lit
			if ps.acceptOp("=") {
				st := &lNumFor{Var: n1, Line: t.Line}
				st.Start = ps.expr()
				ps.expectOp(",", " no for numérico")
				st.Limit = ps.expr()
				if ps.acceptOp(",") {
					st.Step = ps.expr()
				}
				ps.expectKw("do", " no for")
				st.Body = ps.block()
				ps.endBlock(" para fechar o for")
				return st
			}
			st := &lGenFor{Names: []string{n1}, Line: t.Line}
			for ps.acceptOp(",") {
				st.Names = append(st.Names, ps.expectIdent("um nome de variável").Lit)
			}
			ps.expectKw("in", " no for")
			st.Exprs = ps.exprList()
			ps.expectKw("do", " no for")
			st.Body = ps.block()
			ps.endBlock(" para fechar o for")
			return st
		case "repeat":
			ps.next()
			body := ps.block()
			ps.expectKw("until", " para fechar o repeat")
			return &lRepeat{Body: body, Cond: ps.expr(), Line: t.Line}
		case "function":
			ps.next()
			n := ps.expectIdent("o nome da função")
			var target luaExpr = &lName{Name: n.Lit, Line: n.Line}
			full := n.Lit
			method := false
			for ps.isOp(".") || ps.isOp(":") {
				colon := ps.next().Lit == ":"
				k := ps.expectIdent("um nome depois de '.' ou ':'")
				target = &lIndex{Obj: target, Key: &lConst{k.Lit}, Line: k.Line}
				if colon {
					full += ":" + k.Lit
					method = true
					break
				}
				full += "." + k.Lit
			}
			fn := ps.funcBody(full, t.Line, method)
			return &lAssign{Targets: []luaExpr{target}, Vals: []luaExpr{fn}, Line: t.Line}
		case "local":
			ps.next()
			if ps.acceptKw("function") {
				n := ps.expectIdent("o nome da função").Lit
				return &lLocalFunc{Name: n, Fn: ps.funcBody(n, t.Line, false), Line: t.Line}
			}
			st := &lLocal{Line: t.Line}
			for {
				st.Names = append(st.Names, ps.expectIdent("um nome de variável depois de 'local'").Lit)
				if ps.acceptOp("<") { // <const> ou <close>
					a := ps.expectIdent("const ou close").Lit
					if a != "const" && a != "close" {
						ps.fail(ps.peek(), "atributo desconhecido '"+a+"' (use <const> ou <close>)")
					}
					ps.expectOp(">", " depois do atributo")
				}
				if !ps.acceptOp(",") {
					break
				}
			}
			if ps.acceptOp("=") {
				st.Vals = ps.exprList()
			}
			return st
		case "break":
			ps.next()
			return &lBreak{Line: t.Line}
		case "goto":
			ps.next()
			return &lGoto{Label: ps.expectIdent("o nome do rótulo depois de 'goto'").Lit, Line: t.Line}
		}
	}
	e := ps.suffixed()
	if ps.isOp("=") || ps.isOp(",") {
		targets := []luaExpr{e}
		for ps.acceptOp(",") {
			targets = append(targets, ps.suffixed())
		}
		eq := ps.expectOp("=", " na atribuição")
		for _, tg := range targets {
			switch tg.(type) {
			case *lName, *lIndex:
			default:
				ps.fail(eq, "só dá para atribuir a variáveis e campos")
			}
		}
		return &lAssign{Targets: targets, Vals: ps.exprList(), Line: eq.Line}
	}
	switch e.(type) {
	case *lCall, *lMethod:
		return &lCallStmt{Call: e, Line: t.Line}
	}
	ps.fail(ps.peek(), "esperava uma atribuição ou uma chamada de função")
	return nil
}

func (ps *luaParser) funcBody(name string, line int, method bool) *lFunc {
	f := &lFunc{Name: name, Line: line}
	if method {
		f.Params = append(f.Params, "self")
	}
	ps.expectOp("(", " antes dos parâmetros")
	if !ps.isOp(")") {
		for {
			if ps.acceptOp("...") {
				f.Vararg = true
				break
			}
			f.Params = append(f.Params, ps.expectIdent("um nome de parâmetro").Lit)
			if !ps.acceptOp(",") {
				break
			}
		}
	}
	ps.expectOp(")", " para fechar os parâmetros")
	f.Body = ps.block()
	ps.endBlock(" para fechar a função")
	return f
}

func (ps *luaParser) exprList() []luaExpr {
	l := []luaExpr{ps.expr()}
	for ps.acceptOp(",") {
		l = append(l, ps.expr())
	}
	return l
}

var luaBinPrio = map[string][2]int{
	"or": {1, 1}, "and": {2, 2},
	"<": {3, 3}, ">": {3, 3}, "<=": {3, 3}, ">=": {3, 3}, "~=": {3, 3}, "==": {3, 3},
	"|": {4, 4}, "~": {5, 5}, "&": {6, 6}, "<<": {7, 7}, ">>": {7, 7},
	"..": {9, 8}, "+": {10, 10}, "-": {10, 10},
	"*": {11, 11}, "/": {11, 11}, "//": {11, 11}, "%": {11, 11},
	"^": {14, 13},
}

const luaUnaryPrio = 12

func (ps *luaParser) expr() luaExpr { return ps.subexpr(0) }

func (ps *luaParser) binop() (string, bool) {
	t := ps.peek()
	if t.Kind == TOp || (t.Kind == TKeyword && (t.Lit == "and" || t.Lit == "or")) {
		if _, ok := luaBinPrio[t.Lit]; ok {
			return t.Lit, true
		}
	}
	return "", false
}

func (ps *luaParser) subexpr(limit int) luaExpr {
	var e luaExpr
	t := ps.peek()
	if t.Is(TKeyword, "not") || t.Is(TOp, "-") || t.Is(TOp, "#") || t.Is(TOp, "~") {
		ps.next()
		x := ps.subexpr(luaUnaryPrio)
		// dobra constantes negativas: -1 vira o número -1
		if c, ok := x.(*lConst); ok && t.Lit == "-" {
			switch v := c.Val.(type) {
			case int64:
				e = &lConst{-v}
			case float64:
				e = &lConst{-v}
			}
		}
		if e == nil {
			e = &lUn{Op: t.Lit, X: x, Line: t.Line}
		}
	} else {
		e = ps.simple()
	}
	for {
		op, ok := ps.binop()
		if !ok || luaBinPrio[op][0] <= limit {
			return e
		}
		line := ps.next().Line
		r := ps.subexpr(luaBinPrio[op][1])
		if op == "and" || op == "or" {
			e = &lLogic{Op: op, L: e, R: r}
		} else {
			e = &lBin{Op: op, L: e, R: r, Line: line}
		}
	}
}

func (ps *luaParser) simple() luaExpr {
	t := ps.peek()
	switch t.Kind {
	case TInt:
		ps.next()
		return &lConst{t.Int}
	case TFloat:
		ps.next()
		return &lConst{t.Flt}
	case TString:
		ps.next()
		return &lConst{t.Lit}
	case TKeyword:
		switch t.Lit {
		case "nil":
			ps.next()
			return &lConst{nil}
		case "true":
			ps.next()
			return &lConst{true}
		case "false":
			ps.next()
			return &lConst{false}
		case "function":
			ps.next()
			return ps.funcBody("", t.Line, false)
		}
	case TOp:
		switch t.Lit {
		case "...":
			ps.next()
			return &lVararg{Line: t.Line}
		case "{":
			return ps.table()
		}
	}
	return ps.suffixed()
}

func (ps *luaParser) table() *lTable {
	t := ps.expectOp("{", "")
	tb := &lTable{Line: t.Line}
	for !ps.isOp("}") {
		switch {
		case ps.isOp("["):
			ps.next()
			k := ps.expr()
			ps.expectOp("]", " para fechar a chave")
			ps.expectOp("=", " depois da chave")
			tb.Items = append(tb.Items, lField{Key: k, Val: ps.expr()})
		case ps.peek().Kind == TIdent && ps.peekN(1).Is(TOp, "="):
			k := ps.next()
			ps.next()
			tb.Items = append(tb.Items, lField{Key: &lConst{k.Lit}, Val: ps.expr()})
		default:
			tb.Items = append(tb.Items, lField{Val: ps.expr()})
		}
		if !ps.acceptOp(",") && !ps.acceptOp(";") {
			break
		}
	}
	ps.expectOp("}", " para fechar a tabela")
	return tb
}

func (ps *luaParser) primaryExpr() luaExpr {
	t := ps.next()
	switch {
	case t.Kind == TIdent:
		return &lName{Name: t.Lit, Line: t.Line}
	case t.Is(TOp, "("):
		e := ps.expr()
		ps.expectOp(")", " para fechar o parêntese")
		return &lParen{X: e}
	}
	ps.fail(t, "não esperava "+t.String()+" aqui")
	return nil
}

func (ps *luaParser) callArgs() []luaExpr {
	t := ps.peek()
	switch {
	case t.Kind == TString:
		ps.next()
		return []luaExpr{&lConst{t.Lit}}
	case t.Is(TOp, "{"):
		return []luaExpr{ps.table()}
	}
	ps.expectOp("(", " na chamada")
	var args []luaExpr
	if !ps.isOp(")") {
		args = ps.exprList()
	}
	ps.expectOp(")", " para fechar a chamada")
	return args
}

func (ps *luaParser) suffixed() luaExpr {
	e := ps.primaryExpr()
	for {
		t := ps.peek()
		switch {
		case t.Is(TOp, "."):
			ps.next()
			k := ps.next()
			if k.Kind != TIdent {
				ps.fail(k, "esperava um nome de campo depois de '.'")
			}
			e = &lIndex{Obj: e, Key: &lConst{k.Lit}, Line: k.Line}
		case t.Is(TOp, "["):
			ps.next()
			k := ps.expr()
			ps.expectOp("]", " para fechar o índice")
			e = &lIndex{Obj: e, Key: k, Line: t.Line}
		case t.Is(TOp, ":"):
			ps.next()
			name := ps.expectIdent("o nome do método depois de ':'").Lit
			e = &lMethod{Obj: e, Name: name, Args: ps.callArgs(), Line: t.Line}
		case t.Is(TOp, "("), t.Is(TOp, "{"), t.Kind == TString:
			e = &lCall{Fn: e, Args: ps.callArgs(), Line: t.Line}
		default:
			return e
		}
	}
}
