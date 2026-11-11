package lang

// ---------- AST ----------

type Expr interface{ exprNode() }
type Stmt interface{ stmtNode() }

type (
	LitExpr   struct{ Val Value }
	IdentExpr struct {
		Name string
		Line int
	}
	BinaryExpr struct {
		Op   string
		L, R Expr
		Line int
	}
	LogicalExpr struct {
		Op   string // and | or
		L, R Expr
	}
	UnaryExpr struct {
		Op   string // - | not
		X    Expr
		Line int
	}
	CallExpr struct {
		Fn   Expr
		Args []Expr
		Line int
	}
	GetExpr struct {
		Obj  Expr
		Name string
		Line int
	}
	// MethodExpr é obj:metodo(args): chama metodo passando obj como primeiro argumento.
	MethodExpr struct {
		Obj  Expr
		Name string
		Args []Expr
		Line int
	}
	IndexExpr struct {
		Obj, Idx Expr
		Line     int
	}
	ListExpr  struct{ Items []Expr }
	TableExpr struct {
		Keys []string
		Vals []Expr
	}
	FnExpr struct {
		Name     string
		Params   []string
		Variadic bool
		Body     []Stmt
		Line     int
	}
)

func (*LitExpr) exprNode()     {}
func (*IdentExpr) exprNode()   {}
func (*BinaryExpr) exprNode()  {}
func (*LogicalExpr) exprNode() {}
func (*UnaryExpr) exprNode()   {}
func (*CallExpr) exprNode()    {}
func (*GetExpr) exprNode()     {}
func (*MethodExpr) exprNode()  {}
func (*IndexExpr) exprNode()   {}
func (*ListExpr) exprNode()    {}
func (*TableExpr) exprNode()   {}
func (*FnExpr) exprNode()      {}

type (
	LetStmt struct {
		Name string
		Val  Expr
		Line int
	}
	AssignStmt struct {
		Target Expr
		Op     string // "=" ou "+=", "-=", "*=", "/="
		Val    Expr
		Line   int
	}
	ExprStmt struct {
		X    Expr
		Line int
	}
	IfStmt struct {
		Cond       Expr
		Then, Else []Stmt
		Line       int
	}
	WhileStmt struct {
		Cond Expr
		Body []Stmt
		Line int
	}
	ForStmt struct {
		A, B string // B vazio quando há uma só variável
		Coll Expr
		Body []Stmt
		Line int
	}
	ReturnStmt struct {
		X    Expr
		Line int
	}
	BreakStmt    struct{ Line int }
	ContinueStmt struct{ Line int }
	TryStmt      struct {
		Body    []Stmt
		ErrName string
		Catch   []Stmt
		Line    int
	}
	ImportStmt struct {
		Path   string
		IsPath bool
		Alias  string
		Line   int
	}
)

func (*LetStmt) stmtNode()      {}
func (*AssignStmt) stmtNode()   {}
func (*ExprStmt) stmtNode()     {}
func (*IfStmt) stmtNode()       {}
func (*WhileStmt) stmtNode()    {}
func (*ForStmt) stmtNode()      {}
func (*ReturnStmt) stmtNode()   {}
func (*BreakStmt) stmtNode()    {}
func (*ContinueStmt) stmtNode() {}
func (*TryStmt) stmtNode()      {}
func (*ImportStmt) stmtNode()   {}

// ---------- Parser ----------

type parser struct {
	toks []Token
	p    int
	file string
}

type parseBail struct{ err *SyntaxError }

// Parse lê um programa inteiro.
func Parse(src, file string) (prog []Stmt, err error) {
	toks, lerr := Lex(src, file)
	if lerr != nil {
		return nil, lerr
	}
	ps := &parser{toks: toks, file: file}
	defer func() {
		if r := recover(); r != nil {
			if b, ok := r.(parseBail); ok {
				prog, err = nil, b.err
				return
			}
			panic(r)
		}
	}()
	prog = ps.block("EOF")
	return prog, nil
}

func (ps *parser) peek() Token { return ps.toks[ps.p] }
func (ps *parser) peekN(n int) Token {
	if ps.p+n < len(ps.toks) {
		return ps.toks[ps.p+n]
	}
	return ps.toks[len(ps.toks)-1]
}
func (ps *parser) next() Token {
	t := ps.toks[ps.p]
	if ps.p < len(ps.toks)-1 {
		ps.p++
	}
	return t
}
func (ps *parser) prevLine() int {
	if ps.p == 0 {
		return 1
	}
	return ps.toks[ps.p-1].Line
}

func (ps *parser) fail(t Token, msg string) {
	panic(parseBail{&SyntaxError{File: ps.file, Line: t.Line, Msg: msg, Incomplete: t.Kind == TEOF}})
}

func (ps *parser) isKw(w string) bool { return ps.peek().Is(TKeyword, w) }
func (ps *parser) isOp(o string) bool { return ps.peek().Is(TOp, o) }
func (ps *parser) acceptKw(w string) bool {
	if ps.isKw(w) {
		ps.next()
		return true
	}
	return false
}
func (ps *parser) acceptOp(o string) bool {
	if ps.isOp(o) {
		ps.next()
		return true
	}
	return false
}
func (ps *parser) expectKw(w, ctx string) Token {
	if !ps.isKw(w) {
		ps.fail(ps.peek(), "esperava '"+w+"'"+ctx+" mas encontrei "+ps.peek().String())
	}
	return ps.next()
}
func (ps *parser) expectOp(o, ctx string) Token {
	if !ps.isOp(o) {
		ps.fail(ps.peek(), "esperava '"+o+"'"+ctx+" mas encontrei "+ps.peek().String())
	}
	return ps.next()
}
func (ps *parser) expectIdent(ctx string) Token {
	if ps.peek().Kind != TIdent {
		ps.fail(ps.peek(), "esperava "+ctx+" mas encontrei "+ps.peek().String())
	}
	return ps.next()
}

// block lê instruções até uma das palavras de parada (sem consumi-la).
func (ps *parser) block(stops ...string) []Stmt {
	var out []Stmt
	for {
		t := ps.peek()
		if t.Kind == TEOF {
			for _, s := range stops {
				if s == "EOF" {
					return out
				}
			}
			ps.fail(t, "faltou fechar um bloco com 'end'")
		}
		if t.Kind == TKeyword {
			for _, s := range stops {
				if t.Lit == s {
					return out
				}
			}
		}
		out = append(out, ps.statement())
	}
}

func (ps *parser) statement() Stmt {
	t := ps.peek()
	if t.Kind == TKeyword {
		switch t.Lit {
		case "let":
			ps.next()
			name := ps.expectIdent("um nome depois de 'let'")
			ps.expectOp("=", " depois do nome")
			return &LetStmt{Name: name.Lit, Val: ps.expr(), Line: t.Line}
		case "fn":
			if ps.peekN(1).Kind == TIdent {
				ps.next()
				name := ps.next()
				return &LetStmt{Name: name.Lit, Val: ps.fnRest(name.Lit, t.Line), Line: t.Line}
			}
		case "if":
			ps.next()
			return ps.ifRest(t.Line)
		case "while":
			ps.next()
			cond := ps.expr()
			ps.expectKw("do", " depois da condição do while")
			body := ps.block("end")
			ps.next()
			return &WhileStmt{Cond: cond, Body: body, Line: t.Line}
		case "for":
			ps.next()
			a := ps.expectIdent("um nome de variável depois de 'for'").Lit
			b := ""
			if ps.acceptOp(",") {
				b = ps.expectIdent("um segundo nome de variável").Lit
			}
			ps.expectKw("in", " no for")
			coll := ps.expr()
			ps.expectKw("do", " no for")
			body := ps.block("end")
			ps.next()
			return &ForStmt{A: a, B: b, Coll: coll, Body: body, Line: t.Line}
		case "return":
			ps.next()
			nx := ps.peek()
			if nx.Kind == TEOF || nx.Is(TKeyword, "end") || nx.Is(TKeyword, "else") || nx.Is(TKeyword, "elif") || nx.Is(TKeyword, "catch") || nx.Line != t.Line {
				return &ReturnStmt{Line: t.Line}
			}
			return &ReturnStmt{X: ps.expr(), Line: t.Line}
		case "break":
			ps.next()
			return &BreakStmt{Line: t.Line}
		case "continue":
			ps.next()
			return &ContinueStmt{Line: t.Line}
		case "try":
			ps.next()
			ps.expectKw("do", " depois de 'try'")
			body := ps.block("catch")
			ps.next()
			name := ps.expectIdent("um nome para o erro depois de 'catch'").Lit
			ps.expectKw("do", " depois de 'catch "+name+"'")
			c := ps.block("end")
			ps.next()
			return &TryStmt{Body: body, ErrName: name, Catch: c, Line: t.Line}
		case "import":
			ps.next()
			st := &ImportStmt{Line: t.Line}
			nx := ps.next()
			switch nx.Kind {
			case TIdent:
				st.Path, st.Alias = nx.Lit, nx.Lit
			case TString:
				st.Path, st.IsPath = nx.Lit, true
				st.Alias = baseName(nx.Lit)
			default:
				ps.fail(nx, "esperava um nome de módulo ou um caminho entre aspas depois de 'import'")
			}
			if ps.acceptKw("as") {
				st.Alias = ps.expectIdent("um nome depois de 'as'").Lit
			}
			if !validIdent(st.Alias) {
				ps.fail(nx, "use 'import \"caminho\" as nome' para dar um nome válido ao módulo")
			}
			return st
		}
	}
	e := ps.expr()
	if t2 := ps.peek(); t2.Kind == TOp && (t2.Lit == "=" || t2.Lit == "+=" || t2.Lit == "-=" || t2.Lit == "*=" || t2.Lit == "/=") {
		ps.next()
		switch e.(type) {
		case *IdentExpr, *GetExpr, *IndexExpr:
		default:
			ps.fail(t2, "o lado esquerdo de '"+t2.Lit+"' precisa ser uma variável, um campo ou um índice")
		}
		return &AssignStmt{Target: e, Op: t2.Lit, Val: ps.expr(), Line: t2.Line}
	}
	return &ExprStmt{X: e, Line: t.Line}
}

func (ps *parser) ifRest(line int) Stmt {
	cond := ps.expr()
	ps.expectKw("do", " depois da condição do if")
	then := ps.block("elif", "else", "end")
	st := &IfStmt{Cond: cond, Then: then, Line: line}
	switch {
	case ps.isKw("elif"):
		l := ps.next().Line
		st.Else = []Stmt{ps.ifRest(l)}
		return st
	case ps.isKw("else"):
		ps.next()
		if ps.isKw("if") {
			l := ps.next().Line
			st.Else = []Stmt{ps.ifRest(l)}
			return st
		}
		ps.acceptKw("do")
		st.Else = ps.block("end")
	}
	ps.expectKw("end", " para fechar o if")
	return st
}

func (ps *parser) fnRest(name string, line int) *FnExpr {
	ps.expectOp("(", " depois de fn")
	f := &FnExpr{Name: name, Line: line}
	if !ps.isOp(")") {
		for {
			if ps.acceptOp("...") {
				f.Params = append(f.Params, ps.expectIdent("o nome do parâmetro variádico").Lit)
				f.Variadic = true
				break
			}
			f.Params = append(f.Params, ps.expectIdent("um nome de parâmetro").Lit)
			if !ps.acceptOp(",") {
				break
			}
		}
	}
	ps.expectOp(")", " para fechar os parâmetros")
	ps.expectKw("do", " antes do corpo da função")
	f.Body = ps.block("end")
	ps.next()
	return f
}

// Precedência (da menor para a maior): |>  or  and  not  comparação  + -  * / // %  unário  pós-fixo
func (ps *parser) expr() Expr {
	l := ps.or()
	for ps.isOp("|>") {
		line := ps.next().Line
		r := ps.or()
		if c, ok := r.(*CallExpr); ok {
			c.Args = append([]Expr{l}, c.Args...)
			l = c
		} else if m, ok := r.(*MethodExpr); ok {
			m.Args = append([]Expr{l}, m.Args...)
			l = m
		} else {
			l = &CallExpr{Fn: r, Args: []Expr{l}, Line: line}
		}
	}
	return l
}
func (ps *parser) or() Expr {
	l := ps.and()
	for ps.acceptKw("or") {
		l = &LogicalExpr{Op: "or", L: l, R: ps.and()}
	}
	return l
}
func (ps *parser) and() Expr {
	l := ps.not()
	for ps.acceptKw("and") {
		l = &LogicalExpr{Op: "and", L: l, R: ps.not()}
	}
	return l
}
func (ps *parser) not() Expr {
	if ps.isKw("not") {
		line := ps.next().Line
		return &UnaryExpr{Op: "not", X: ps.not(), Line: line}
	}
	return ps.cmp()
}
func (ps *parser) binLevel(next func() Expr, ops ...string) Expr {
	l := next()
	for {
		t := ps.peek()
		found := false
		if t.Kind == TOp {
			for _, o := range ops {
				if t.Lit == o {
					found = true
					break
				}
			}
		}
		if !found {
			return l
		}
		ps.next()
		l = &BinaryExpr{Op: t.Lit, L: l, R: next(), Line: t.Line}
	}
}
func (ps *parser) cmp() Expr { return ps.binLevel(ps.add, "==", "!=", "<", ">", "<=", ">=") }
func (ps *parser) add() Expr { return ps.binLevel(ps.mul, "+", "-") }
func (ps *parser) mul() Expr { return ps.binLevel(ps.unary, "*", "/", "//", "%") }
func (ps *parser) unary() Expr {
	if ps.isOp("-") {
		line := ps.next().Line
		return &UnaryExpr{Op: "-", X: ps.unary(), Line: line}
	}
	return ps.postfix()
}

func (ps *parser) postfix() Expr {
	e := ps.primary()
	for {
		prev := ps.prevLine()
		t := ps.peek()
		switch {
		case t.Is(TOp, "(") && t.Line == prev:
			ps.next()
			var args []Expr
			for !ps.isOp(")") {
				args = append(args, ps.expr())
				if !ps.acceptOp(",") {
					break
				}
			}
			ps.expectOp(")", " para fechar a chamada")
			e = &CallExpr{Fn: e, Args: args, Line: t.Line}
		case t.Is(TOp, "."):
			ps.next()
			nt := ps.next()
			if nt.Kind != TIdent && nt.Kind != TKeyword {
				ps.fail(nt, "esperava um nome de campo depois de '.'")
			}
			e = &GetExpr{Obj: e, Name: nt.Lit, Line: nt.Line}
		case t.Is(TOp, ":") && ps.peekN(1).Kind == TIdent && ps.peekN(2).Is(TOp, "(") && ps.peekN(2).Line == ps.peekN(1).Line:
			ps.next()
			name := ps.next()
			ps.next()
			var args []Expr
			for !ps.isOp(")") {
				args = append(args, ps.expr())
				if !ps.acceptOp(",") {
					break
				}
			}
			ps.expectOp(")", " para fechar a chamada")
			e = &MethodExpr{Obj: e, Name: name.Lit, Args: args, Line: name.Line}
		case t.Is(TOp, "[") && t.Line == prev:
			ps.next()
			idx := ps.expr()
			ps.expectOp("]", " para fechar o índice")
			e = &IndexExpr{Obj: e, Idx: idx, Line: t.Line}
		default:
			return e
		}
	}
}

func (ps *parser) primary() Expr {
	t := ps.next()
	switch t.Kind {
	case TInt:
		return &LitExpr{Val: t.Int}
	case TFloat:
		return &LitExpr{Val: t.Flt}
	case TString:
		return &LitExpr{Val: t.Lit}
	case TIdent:
		return &IdentExpr{Name: t.Lit, Line: t.Line}
	case TKeyword:
		switch t.Lit {
		case "true":
			return &LitExpr{Val: true}
		case "false":
			return &LitExpr{Val: false}
		case "nil":
			return &LitExpr{Val: nil}
		case "fn":
			return ps.fnRest("", t.Line)
		}
	case TOp:
		switch t.Lit {
		case "(":
			e := ps.expr()
			ps.expectOp(")", " para fechar o parêntese")
			return e
		case "[":
			l := &ListExpr{}
			for !ps.isOp("]") {
				l.Items = append(l.Items, ps.expr())
				if !ps.acceptOp(",") {
					break
				}
			}
			ps.expectOp("]", " para fechar a lista")
			return l
		case "{":
			tb := &TableExpr{}
			for !ps.isOp("}") {
				k := ps.next()
				switch k.Kind {
				case TIdent, TString, TKeyword, TInt:
				default:
					ps.fail(k, "chave de tabela inválida: "+k.String())
				}
				ps.expectOp(":", " depois da chave da tabela")
				tb.Keys = append(tb.Keys, k.Lit)
				tb.Vals = append(tb.Vals, ps.expr())
				if !ps.acceptOp(",") {
					break
				}
			}
			ps.expectOp("}", " para fechar a tabela")
			return tb
		}
	}
	ps.fail(t, "não esperava "+t.String()+" aqui")
	return nil
}

func validIdent(s string) bool {
	toks, err := Lex(s, "")
	return err == nil && len(toks) == 2 && toks[0].Kind == TIdent
}

func baseName(p string) string {
	for i := len(p) - 1; i >= 0; i-- {
		if p[i] == '/' || p[i] == '\\' {
			p = p[i+1:]
			break
		}
	}
	if len(p) > 4 && (p[len(p)-4:] == ".nep" || p[len(p)-4:] == ".lua") {
		p = p[:len(p)-4]
	}
	return p
}
