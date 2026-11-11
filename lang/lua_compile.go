package lang

import "fmt"

// Compilador de Lua para o mesmo bytecode do Nepenthes.

type luaLabel struct{ pos, nlocals int }

type luaGoto struct {
	name           string
	line           int
	jumpAt, topAt  int
	nlocals, depth int
}

type luaBlock struct {
	labels  map[string]luaLabel
	pending []luaGoto
	nlocals int
}

// CompileLua transforma um programa Lua em uma função de nível superior.
// env é a tabela de variáveis globais (_ENV) que o código vai usar.
func CompileLua(prog []luaStmt, mod *Module, env *Table, file string) (p *Proto, err error) {
	c := &compiler{proto: &Proto{Module: mod, File: file, IsTop: true, Lua: true, Env: env, Arity: 1, Variadic: true}, file: file, lua: true}
	c.locals = append(c.locals, local{name: "", depth: 0})
	defer func() {
		if r := recover(); r != nil {
			if b, ok := r.(compileBail); ok {
				p, err = nil, b.err
				return
			}
			panic(r)
		}
	}()
	c.begin()
	c.declare("...", 1)
	c.luaBody(prog)
	c.emit(OpPack, 0)
	c.emit(OpReturn)
	return c.proto, nil
}

func (c *compiler) luaBody(stmts []luaStmt) {
	b := &luaBlock{labels: map[string]luaLabel{}, nlocals: len(c.locals)}
	c.lblocks = append(c.lblocks, b)
	for i, s := range stmts {
		if l, ok := s.(*lLabel); ok {
			c.luaDefineLabel(b, l, stmts[i+1:])
			continue
		}
		c.luaStmt(s)
	}
	c.lblocks = c.lblocks[:len(c.lblocks)-1]
	if len(b.pending) > 0 {
		if len(c.lblocks) == 0 {
			g := b.pending[0]
			c.fail(g.line, "não há rótulo visível '"+g.name+"' para o goto")
		}
		parent := c.lblocks[len(c.lblocks)-1]
		for _, g := range b.pending {
			// ao sair do bloco, as locais dele deixam de existir
			g.nlocals = min(g.nlocals, b.nlocals)
			parent.pending = append(parent.pending, g)
		}
	}
}

// luaBlock compila um bloco com escopo próprio.
func (c *compiler) luaBlock(stmts []luaStmt) {
	c.begin()
	c.luaBody(stmts)
	c.end()
}

func (c *compiler) luaDefineLabel(b *luaBlock, l *lLabel, rest []luaStmt) {
	for _, ob := range c.lblocks {
		if _, dup := ob.labels[l.Name]; dup {
			c.fail(l.Line, "o rótulo '"+l.Name+"' já existe")
		}
	}
	n := len(c.locals)
	void := true
	for _, s := range rest {
		if _, ok := s.(*lLabel); !ok {
			void = false
			break
		}
	}
	if void { // rótulo no fim do bloco: fora do alcance das locais do bloco
		n = b.nlocals
	}
	lb := luaLabel{pos: len(c.proto.Code), nlocals: n}
	b.labels[l.Name] = lb
	keep := b.pending[:0]
	for _, g := range b.pending {
		if g.name != l.Name {
			keep = append(keep, g)
			continue
		}
		if g.nlocals < lb.nlocals {
			c.fail(g.line, "o goto "+g.name+" pula para dentro do alcance de uma variável local")
		}
		c.proto.Code[g.topAt] = byte(lb.nlocals)
		c.patch(g.jumpAt)
	}
	b.pending = keep
}

func (c *compiler) luaGotoStmt(s *lGoto) {
	c.line = s.Line
	for i := len(c.lblocks) - 1; i >= 0; i-- {
		if l, ok := c.lblocks[i].labels[s.Label]; ok {
			c.emit(OpSetTop, byte(l.nlocals))
			c.emitLoop(l.pos)
			return
		}
	}
	c.emit(OpSetTop, 0)
	g := luaGoto{name: s.Label, line: s.Line, topAt: len(c.proto.Code) - 1, nlocals: len(c.locals), depth: c.depth}
	g.jumpAt = c.emitJump(OpJump)
	b := c.lblocks[len(c.lblocks)-1]
	b.pending = append(b.pending, g)
}

func isMultiExpr(e luaExpr) bool {
	switch e.(type) {
	case *lCall, *lMethod, *lVararg:
		return true
	}
	return false
}

func (c *compiler) luaStmt(s luaStmt) {
	switch s := s.(type) {
	case *lLocal:
		c.line = s.Line
		c.luaExprList(s.Vals, len(s.Names))
		for _, n := range s.Names {
			c.declare(n, s.Line)
		}
	case *lLocalFunc:
		c.line = s.Line
		c.declare(s.Name, s.Line)
		c.luaFunction(s.Fn)
	case *lAssign:
		c.luaAssign(s)
	case *lCallStmt:
		c.line = s.Line
		c.luaCall(s.Call, false)
		c.emit(OpPop)
	case *lDo:
		c.luaBlock(s.Body)
	case *lWhile:
		c.line = s.Line
		lp := &loopInfo{start: len(c.proto.Code), scopeDepth: c.depth}
		c.loops = append(c.loops, lp)
		c.luaExpr(s.Cond)
		jExit := c.emitJump(OpPopJumpIfFalse)
		c.luaBlock(s.Body)
		c.emitLoop(lp.start)
		c.patch(jExit)
		for _, at := range lp.breaks {
			c.patch(at)
		}
		c.loops = c.loops[:len(c.loops)-1]
	case *lRepeat:
		c.line = s.Line
		lp := &loopInfo{start: len(c.proto.Code), scopeDepth: c.depth}
		c.loops = append(c.loops, lp)
		c.begin()
		c.luaBody(s.Body)
		c.luaExpr(s.Cond) // a condição enxerga as locais do corpo
		jFalse := c.emitJump(OpPopJumpIfFalse)
		c.popTo(c.depth - 1)
		jExit := c.emitJump(OpJump)
		c.patch(jFalse)
		c.popTo(c.depth - 1)
		c.emitLoop(lp.start)
		c.patch(jExit)
		c.depth--
		for len(c.locals) > 0 && c.locals[len(c.locals)-1].depth > c.depth {
			c.locals = c.locals[:len(c.locals)-1]
		}
		for _, at := range lp.breaks {
			c.patch(at)
		}
		c.loops = c.loops[:len(c.loops)-1]
	case *lIf:
		c.line = s.Line
		var ends []int
		for i, cond := range s.Conds {
			c.luaExpr(cond)
			jNext := c.emitJump(OpPopJumpIfFalse)
			c.luaBlock(s.Blocks[i])
			if i < len(s.Conds)-1 || s.Else != nil {
				ends = append(ends, c.emitJump(OpJump))
			}
			c.patch(jNext)
		}
		if s.Else != nil {
			c.luaBlock(s.Else)
		}
		for _, at := range ends {
			c.patch(at)
		}
	case *lNumFor:
		c.line = s.Line
		c.begin()
		c.luaExpr(s.Start)
		c.luaExpr(s.Limit)
		if s.Step != nil {
			c.luaExpr(s.Step)
		} else {
			c.emit16(OpConst, c.konst(int64(1)))
		}
		slot := c.declare("(for)", s.Line)
		c.declare("(limite)", s.Line)
		c.declare("(passo)", s.Line)
		c.line = s.Line
		c.emit(OpForPrep, byte(slot), 0xFF, 0xFF)
		jExit := len(c.proto.Code) - 2
		lp := &loopInfo{start: len(c.proto.Code), scopeDepth: c.depth}
		c.loops = append(c.loops, lp)
		c.begin()
		c.emit16(OpGetLocal, slot)
		c.declare(s.Var, s.Line)
		c.luaBlock(s.Body)
		c.end()
		c.line = s.Line
		off := len(c.proto.Code) + 4 - lp.start
		c.emit(OpForLoop, byte(slot), byte(off>>8), byte(off))
		c.patch(jExit)
		for _, at := range lp.breaks {
			c.patch(at)
		}
		c.loops = c.loops[:len(c.loops)-1]
		c.end()
	case *lGenFor:
		c.line = s.Line
		c.begin()
		c.luaExprList(s.Exprs, 3)
		slot := c.declare("(f)", s.Line)
		c.declare("(estado)", s.Line)
		c.declare("(controle)", s.Line)
		lp := &loopInfo{start: len(c.proto.Code), scopeDepth: c.depth}
		c.loops = append(c.loops, lp)
		if len(s.Names) > 200 {
			c.fail(s.Line, "variáveis demais no for")
		}
		c.line = s.Line
		c.emit(OpForGen, byte(slot), byte(len(s.Names)), 0xFF, 0xFF)
		jExit := len(c.proto.Code) - 2
		c.begin()
		for _, n := range s.Names {
			c.declare(n, s.Line)
		}
		c.luaBlock(s.Body)
		c.end()
		c.emitLoop(lp.start)
		c.patch(jExit)
		for _, at := range lp.breaks {
			c.patch(at)
		}
		c.loops = c.loops[:len(c.loops)-1]
		c.end()
	case *lReturn:
		c.line = s.Line
		if len(s.Vals) == 1 {
			switch e := s.Vals[0].(type) {
			case *lCall, *lMethod:
				c.luaCall(e, true)
				c.emit(OpReturn)
				return
			case *lVararg:
				c.luaMulti(e)
				c.emit(OpReturn)
				return
			}
			c.luaExpr(s.Vals[0])
			c.emit(OpReturn)
			return
		}
		c.luaPushAll(s.Vals)
		c.emit(OpPack, byte(len(s.Vals)))
		c.emit(OpReturn)
	case *lBreak:
		c.line = s.Line
		if len(c.loops) == 0 {
			c.fail(s.Line, "break fora de um laço")
		}
		lp := c.loops[len(c.loops)-1]
		c.popTo(lp.scopeDepth)
		lp.breaks = append(lp.breaks, c.emitJump(OpJump))
	case *lGoto:
		c.luaGotoStmt(s)
	case *lLabel:
		// tratado em luaBody
	default:
		c.fail(c.line, fmt.Sprintf("instrução Lua desconhecida %T", s))
	}
}

// luaPushAll empurra todos os valores; o último fica "aberto" se for chamada ou ...
func (c *compiler) luaPushAll(exprs []luaExpr) {
	if len(exprs) > 250 {
		c.fail(c.line, "valores demais")
	}
	for i, e := range exprs {
		if i == len(exprs)-1 && isMultiExpr(e) {
			c.luaMulti(e)
		} else {
			c.luaExpr(e)
		}
	}
}

// luaExprList empurra exatamente want valores, ajustando como o Lua faz.
func (c *compiler) luaExprList(exprs []luaExpr, want int) {
	if want > 250 {
		c.fail(c.line, "variáveis demais")
	}
	for i, e := range exprs {
		last := i == len(exprs)-1
		if !last {
			c.luaExpr(e)
			if i >= want {
				c.emit(OpPop)
			}
			continue
		}
		remaining := want - i
		switch {
		case remaining <= 0:
			if isMultiExpr(e) {
				c.luaMulti(e)
			} else {
				c.luaExpr(e)
			}
			c.emit(OpPop)
		case isMultiExpr(e):
			c.luaMulti(e)
			c.emit(OpSpread, byte(remaining))
		default:
			c.luaExpr(e)
			for k := 1; k < remaining; k++ {
				c.emit(OpNil)
			}
		}
	}
	if len(exprs) == 0 {
		for k := 0; k < want; k++ {
			c.emit(OpNil)
		}
	}
}

func (c *compiler) luaAssign(s *lAssign) {
	c.line = s.Line
	if len(s.Targets) == 1 {
		switch t := s.Targets[0].(type) {
		case *lName:
			c.luaExprList(s.Vals, 1)
			c.line = s.Line
			c.luaSetName(t.Name)
		case *lIndex:
			c.luaExpr(t.Obj)
			if k, ok := t.Key.(*lConst); ok {
				if ks, ok := k.Val.(string); ok {
					c.luaExprList(s.Vals, 1)
					c.line = t.Line
					c.emit16(OpSetField, c.konst(ks))
					return
				}
			}
			c.luaExpr(t.Key)
			c.luaExprList(s.Vals, 1)
			c.line = t.Line
			c.emit(OpSetIndex)
		}
		return
	}
	// várias: avalia tudo antes de atribuir
	c.begin()
	type tslot struct{ obj, key int }
	slots := make([]tslot, len(s.Targets))
	for i, tg := range s.Targets {
		if ix, ok := tg.(*lIndex); ok {
			c.luaExpr(ix.Obj)
			slots[i].obj = c.declare("(obj)", s.Line)
			c.luaExpr(ix.Key)
			slots[i].key = c.declare("(chave)", s.Line)
		}
	}
	c.luaExprList(s.Vals, len(s.Targets))
	first := len(c.locals)
	for range s.Targets {
		c.declare("(valor)", s.Line)
	}
	for i, tg := range s.Targets {
		c.line = s.Line
		switch t := tg.(type) {
		case *lName:
			c.emit16(OpGetLocal, first+i)
			c.luaSetName(t.Name)
		case *lIndex:
			c.emit16(OpGetLocal, slots[i].obj)
			c.emit16(OpGetLocal, slots[i].key)
			c.emit16(OpGetLocal, first+i)
			c.emit(OpSetIndex)
		}
	}
	c.end()
}

// luaSetName guarda o valor do topo na variável (e o tira da pilha).
func (c *compiler) luaSetName(name string) {
	if l := c.resolveLocal(name); l > 0 {
		c.emit16(OpSetLocal, l)
		c.emit(OpPop)
		return
	}
	if u := c.resolveUpval(name); u >= 0 {
		c.emit16(OpSetUpval, u)
		c.emit(OpPop)
		return
	}
	c.emit16(OpDefGlobal, c.konst(name))
}

var luaBinOps = map[string]byte{"+": OpAdd, "-": OpSub, "*": OpMul, "/": OpDiv, "//": OpIDiv, "%": OpMod, "^": OpPow,
	"..": OpConcat, "&": OpBand, "|": OpBor, "~": OpBxor, "<<": OpShl, ">>": OpShr,
	"==": OpEq, "~=": OpNe, "<": OpLt, ">": OpGt, "<=": OpLe, ">=": OpGe}

// luaExpr empurra exatamente um valor.
func (c *compiler) luaExpr(e luaExpr) {
	switch e := e.(type) {
	case *lConst:
		switch e.Val {
		case nil:
			c.emit(OpNil)
		case true:
			c.emit(OpTrue)
		case false:
			c.emit(OpFalse)
		default:
			c.emit16(OpConst, c.konst(e.Val))
		}
	case *lVararg, *lCall, *lMethod:
		c.luaMulti(e)
		c.emit(OpFirst)
	case *lName:
		c.line = e.Line
		if l := c.resolveLocal(e.Name); l > 0 {
			c.emit16(OpGetLocal, l)
		} else if u := c.resolveUpval(e.Name); u >= 0 {
			c.emit16(OpGetUpval, u)
		} else {
			c.emit16(OpGetGlobal, c.konst(e.Name))
		}
	case *lIndex:
		c.luaExpr(e.Obj)
		if k, ok := e.Key.(*lConst); ok {
			if ks, ok := k.Val.(string); ok {
				c.line = e.Line
				c.emit16(OpGetField, c.konst(ks))
				return
			}
		}
		c.luaExpr(e.Key)
		c.line = e.Line
		c.emit(OpGetIndex)
	case *lFunc:
		c.luaFunction(e)
	case *lBin:
		c.luaExpr(e.L)
		c.luaExpr(e.R)
		c.line = e.Line
		c.emit(luaBinOps[e.Op])
	case *lUn:
		c.luaExpr(e.X)
		c.line = e.Line
		switch e.Op {
		case "-":
			c.emit(OpNeg)
		case "not":
			c.emit(OpNot)
		case "#":
			c.emit(OpLen)
		case "~":
			c.emit(OpBnot)
		}
	case *lLogic:
		c.luaExpr(e.L)
		var j int
		if e.Op == "and" {
			j = c.emitJump(OpJumpIfFalse)
		} else {
			j = c.emitJump(OpJumpIfTrue)
		}
		c.emit(OpPop)
		c.luaExpr(e.R)
		c.patch(j)
	case *lParen:
		c.luaExpr(e.X)
	case *lTable:
		c.line = e.Line
		c.emit(OpNewTable)
		pos := 0
		for i, f := range e.Items {
			if f.Key != nil {
				c.luaExpr(f.Key)
				c.luaExpr(f.Val)
				c.emit(OpTSet)
				continue
			}
			pos++
			if pos > 0xFFFF {
				c.fail(e.Line, "tabela grande demais")
			}
			if i == len(e.Items)-1 && isMultiExpr(f.Val) {
				c.luaMulti(f.Val)
				c.emit16(OpTAppendM, pos)
			} else {
				c.luaExpr(f.Val)
				c.emit16(OpTAppend, pos)
			}
		}
	default:
		c.fail(c.line, fmt.Sprintf("expressão Lua desconhecida %T", e))
	}
}

// luaMulti empurra um valor que pode ser vários (chamada ou ...).
func (c *compiler) luaMulti(e luaExpr) {
	switch e := e.(type) {
	case *lVararg:
		c.line = e.Line
		l := c.resolveLocal("...")
		if l <= 0 || !c.proto.Variadic {
			c.fail(e.Line, "'...' só pode ser usado dentro de uma função com '...'")
		}
		c.emit16(OpGetLocal, l)
	case *lCall, *lMethod:
		c.luaCall(e, false)
	default:
		c.luaExpr(e)
	}
}

func (c *compiler) luaCall(e luaExpr, tail bool) {
	var args []luaExpr
	var line int
	extra := 0
	switch e := e.(type) {
	case *lCall:
		c.luaExpr(e.Fn)
		args, line = e.Args, e.Line
	case *lMethod:
		c.luaExpr(e.Obj)
		c.line = e.Line
		c.emit16(OpSelf, c.konst(e.Name))
		args, line, extra = e.Args, e.Line, 1
	default:
		c.luaExpr(e)
		return
	}
	if len(args)+extra > 250 {
		c.fail(line, "argumentos demais")
	}
	multi := len(args) > 0 && isMultiExpr(args[len(args)-1])
	c.luaPushAll(args)
	c.line = line
	argc := byte(len(args) + extra)
	switch {
	case tail:
		m := byte(0)
		if multi {
			m = 1
		}
		c.emit(OpTailCall, argc, m)
	case multi:
		c.emit(OpCallM, argc)
	default:
		c.emit(OpCall, argc)
	}
}

func (c *compiler) luaFunction(f *lFunc) {
	arity := len(f.Params)
	if f.Vararg {
		arity++
	}
	fc := &compiler{enclosing: c, file: c.file, line: f.Line, lua: true,
		proto: &Proto{Name: f.Name, Arity: arity, Variadic: f.Vararg, Module: c.proto.Module, File: c.file, Lua: true, Env: c.proto.Env}}
	fc.locals = append(fc.locals, local{name: "", depth: 0})
	fc.depth = 1
	for _, p := range f.Params {
		fc.declare(p, f.Line)
	}
	if f.Vararg {
		fc.declare("...", f.Line)
	}
	fc.luaBody(f.Body)
	fc.emit(OpPack, 0)
	fc.emit(OpReturn)
	c.line = f.Line
	k := c.konst(fc.proto)
	c.emit16(OpClosure, k)
	for _, u := range fc.upvals {
		b := byte(0)
		if u.isLocal {
			b = 1
		}
		c.emit(b, byte(u.index>>8), byte(u.index))
	}
}
