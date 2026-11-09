package lang

import (
	"fmt"
	"strings"
)

// Opcodes da máquina virtual. Operandos são de 16 bits (big-endian), salvo indicação.
const (
	OpConst          byte = iota // k
	OpNil                        //
	OpTrue                       //
	OpFalse                      //
	OpPop                        //
	OpDup                        //
	OpDup2                       //
	OpGetLocal                   // slot
	OpSetLocal                   // slot  (não remove da pilha)
	OpGetUpval                   // i
	OpSetUpval                   // i     (não remove da pilha)
	OpGetGlobal                  // k
	OpSetGlobal                  // k     (não remove da pilha)
	OpDefGlobal                  // k     (remove da pilha)
	OpGetField                   // k
	OpSetField                   // k     obj val -> (remove os dois)
	OpGetIndex                   //
	OpSetIndex                   //       obj idx val -> (remove os três)
	OpAdd                        //
	OpSub                        //
	OpMul                        //
	OpDiv                        //
	OpIDiv                       //
	OpMod                        //
	OpNeg                        //
	OpNot                        //
	OpEq                         //
	OpNe                         //
	OpLt                         //
	OpGt                         //
	OpLe                         //
	OpGe                         //
	OpJump                       // off
	OpJumpIfFalse                // off  (não remove)
	OpJumpIfTrue                 // off  (não remove)
	OpPopJumpIfFalse             // off
	OpLoop                       // off (para trás)
	OpCall                       // argc (1 byte)
	OpClosure                    // k, depois n pares (isLocal 1 byte, index 2 bytes)
	OpCloseUpval                 //
	OpReturn                     //
	OpList                       // n
	OpTable                      // n pares
	OpIterPrep                   //
	OpIterNext                   // slot, nvars (1 byte), off
	OpTry                        // off
	OpEndTry                     //
	OpImport                     // k (nome/caminho), 1 byte isPath
)

var opNames = []string{"CONST", "NIL", "TRUE", "FALSE", "POP", "DUP", "DUP2", "GET_LOCAL", "SET_LOCAL", "GET_UPVAL", "SET_UPVAL",
	"GET_GLOBAL", "SET_GLOBAL", "DEF_GLOBAL", "GET_FIELD", "SET_FIELD", "GET_INDEX", "SET_INDEX", "ADD", "SUB", "MUL", "DIV", "IDIV",
	"MOD", "NEG", "NOT", "EQ", "NE", "LT", "GT", "LE", "GE", "JUMP", "JUMP_IF_FALSE", "JUMP_IF_TRUE", "POP_JUMP_IF_FALSE", "LOOP",
	"CALL", "CLOSURE", "CLOSE_UPVAL", "RETURN", "LIST", "TABLE", "ITER_PREP", "ITER_NEXT", "TRY", "END_TRY", "IMPORT"}

// Proto é uma função compilada.
type Proto struct {
	Name      string
	Arity     int
	Variadic  bool
	Code      []byte
	Lines     []int
	Consts    []Value
	NUpvalues int
	Module    *Module
	File      string
	IsTop     bool
}

type local struct {
	name     string
	depth    int
	captured bool
}

type upvalDesc struct {
	isLocal bool
	index   uint16
}

type loopInfo struct {
	start      int
	breaks     []int
	continues  []int
	scopeDepth int
	tryDepth   int
	isFor      bool
}

type compiler struct {
	enclosing *compiler
	proto     *Proto
	locals    []local
	upvals    []upvalDesc
	depth     int
	loops     []*loopInfo
	tryDepth  int
	line      int
	file      string
}

type compileBail struct{ err error }

// Compile transforma um programa em uma função de nível superior.
func Compile(prog []Stmt, mod *Module, file string) (p *Proto, err error) {
	c := &compiler{proto: &Proto{Name: "", Module: mod, File: file, IsTop: true}, file: file}
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
	for _, s := range prog {
		c.stmt(s)
	}
	c.emit(OpNil)
	c.emit(OpReturn)
	return c.proto, nil
}

func (c *compiler) fail(line int, msg string) {
	panic(compileBail{&SyntaxError{File: c.file, Line: line, Msg: msg}})
}

func (c *compiler) emit(b ...byte) {
	for _, x := range b {
		c.proto.Code = append(c.proto.Code, x)
		c.proto.Lines = append(c.proto.Lines, c.line)
	}
}
func (c *compiler) emit16(op byte, v int) {
	if v > 0xFFFF {
		c.fail(c.line, "função grande demais")
	}
	c.emit(op, byte(v>>8), byte(v))
}
func (c *compiler) konst(v Value) int {
	for i, k := range c.proto.Consts {
		if k == v {
			if _, ok := k.(string); ok || isNum(k) {
				if TypeName(k) == TypeName(v) {
					return i
				}
			}
		}
	}
	c.proto.Consts = append(c.proto.Consts, v)
	return len(c.proto.Consts) - 1
}
func (c *compiler) emitJump(op byte) int {
	c.emit(op, 0xFF, 0xFF)
	return len(c.proto.Code) - 2
}
func (c *compiler) patch(at int) {
	off := len(c.proto.Code) - at - 2
	if off > 0xFFFF {
		c.fail(c.line, "bloco grande demais")
	}
	c.proto.Code[at] = byte(off >> 8)
	c.proto.Code[at+1] = byte(off)
}
func (c *compiler) emitLoop(start int) {
	off := len(c.proto.Code) - start + 3
	c.emit16(OpLoop, off)
}

// ---------- escopo ----------

func (c *compiler) begin() { c.depth++ }
func (c *compiler) end() {
	c.depth--
	for len(c.locals) > 0 && c.locals[len(c.locals)-1].depth > c.depth {
		if c.locals[len(c.locals)-1].captured {
			c.emit(OpCloseUpval)
		} else {
			c.emit(OpPop)
		}
		c.locals = c.locals[:len(c.locals)-1]
	}
}

// popTo emite pops/fechamentos para sair até a profundidade d sem mexer no estado do compilador.
func (c *compiler) popTo(d int) {
	for i := len(c.locals) - 1; i >= 0 && c.locals[i].depth > d; i-- {
		if c.locals[i].captured {
			c.emit(OpCloseUpval)
		} else {
			c.emit(OpPop)
		}
	}
}

func (c *compiler) declare(name string, line int) int {
	for i := len(c.locals) - 1; i >= 0 && c.locals[i].depth == c.depth; i-- {
		if c.locals[i].name == name && name != "" && name[0] != '(' {
			c.fail(line, "a variável '"+name+"' já foi declarada neste bloco")
		}
	}
	if len(c.locals) >= 250 {
		c.fail(line, "variáveis locais demais em uma função (máximo 250)")
	}
	c.locals = append(c.locals, local{name: name, depth: c.depth})
	return len(c.locals) - 1
}

func (c *compiler) resolveLocal(name string) int {
	for i := len(c.locals) - 1; i >= 0; i-- {
		if c.locals[i].name == name {
			return i
		}
	}
	return -1
}

func (c *compiler) addUpval(isLocal bool, idx int) int {
	for i, u := range c.upvals {
		if u.isLocal == isLocal && int(u.index) == idx {
			return i
		}
	}
	c.upvals = append(c.upvals, upvalDesc{isLocal, uint16(idx)})
	c.proto.NUpvalues = len(c.upvals)
	return len(c.upvals) - 1
}

func (c *compiler) resolveUpval(name string) int {
	if c.enclosing == nil {
		return -1
	}
	if l := c.enclosing.resolveLocal(name); l >= 0 {
		if c.enclosing.enclosing == nil && c.enclosing.locals[l].depth == 0 {
			return -1 // slot 0 do script não é capturável
		}
		c.enclosing.locals[l].captured = true
		return c.addUpval(true, l)
	}
	if u := c.enclosing.resolveUpval(name); u >= 0 {
		return c.addUpval(false, u)
	}
	return -1
}

func (c *compiler) isGlobalScope() bool { return c.enclosing == nil && c.depth == 0 }

// ---------- instruções ----------

func (c *compiler) block(stmts []Stmt) {
	c.begin()
	for _, s := range stmts {
		c.stmt(s)
	}
	c.end()
}

func (c *compiler) stmt(s Stmt) {
	switch s := s.(type) {
	case *LetStmt:
		c.line = s.Line
		if c.isGlobalScope() {
			c.expr(s.Val)
			c.line = s.Line
			c.emit16(OpDefGlobal, c.konst(s.Name))
			return
		}
		// declara antes para permitir funções recursivas locais
		if _, isFn := s.Val.(*FnExpr); isFn {
			c.declare(s.Name, s.Line)
			c.expr(s.Val)
			return
		}
		c.expr(s.Val)
		c.declare(s.Name, s.Line)
	case *ExprStmt:
		c.line = s.Line
		c.expr(s.X)
		c.emit(OpPop)
	case *AssignStmt:
		c.assign(s)
	case *IfStmt:
		c.line = s.Line
		c.expr(s.Cond)
		jElse := c.emitJump(OpPopJumpIfFalse)
		c.block(s.Then)
		if s.Else != nil {
			jEnd := c.emitJump(OpJump)
			c.patch(jElse)
			c.block(s.Else)
			c.patch(jEnd)
		} else {
			c.patch(jElse)
		}
	case *WhileStmt:
		c.line = s.Line
		lp := &loopInfo{start: len(c.proto.Code), scopeDepth: c.depth, tryDepth: c.tryDepth}
		c.loops = append(c.loops, lp)
		c.expr(s.Cond)
		jExit := c.emitJump(OpPopJumpIfFalse)
		c.block(s.Body)
		for _, at := range lp.continues {
			c.patch(at)
		}
		c.emitLoop(lp.start)
		c.patch(jExit)
		for _, at := range lp.breaks {
			c.patch(at)
		}
		c.loops = c.loops[:len(c.loops)-1]
	case *ForStmt:
		c.line = s.Line
		c.begin()
		c.expr(s.Coll)
		c.line = s.Line
		c.emit(OpIterPrep)
		iterSlot := c.declare("(iter)", s.Line)
		lp := &loopInfo{start: len(c.proto.Code), scopeDepth: c.depth, tryDepth: c.tryDepth, isFor: true}
		c.loops = append(c.loops, lp)
		nvars := 1
		if s.B != "" {
			nvars = 2
		}
		c.emit(OpIterNext, byte(iterSlot), byte(nvars), 0xFF, 0xFF)
		jExit := len(c.proto.Code) - 2
		c.begin()
		c.declare(s.A, s.Line)
		if s.B != "" {
			c.declare(s.B, s.Line)
		}
		for _, st := range s.Body {
			c.stmt(st)
		}
		for _, at := range lp.continues {
			c.patch(at)
		}
		c.end()
		c.emitLoop(lp.start)
		c.patch(jExit)
		for _, at := range lp.breaks {
			c.patch(at)
		}
		c.loops = c.loops[:len(c.loops)-1]
		c.end()
	case *BreakStmt, *ContinueStmt:
		line := 0
		isBreak := false
		if b, ok := s.(*BreakStmt); ok {
			line, isBreak = b.Line, true
		} else {
			line = s.(*ContinueStmt).Line
		}
		c.line = line
		if len(c.loops) == 0 {
			c.fail(line, "break/continue fora de um laço")
		}
		lp := c.loops[len(c.loops)-1]
		for i := c.tryDepth; i > lp.tryDepth; i-- {
			c.emit(OpEndTry)
		}
		if isBreak {
			c.popTo(lp.scopeDepth)
			lp.breaks = append(lp.breaks, c.emitJump(OpJump))
		} else if lp.isFor {
			// continue em for: limpa até as variáveis do laço e pula para o fim do corpo
			c.popTo(lp.scopeDepth + 1)
			lp.continues = append(lp.continues, c.emitJump(OpJump))
		} else {
			c.popTo(lp.scopeDepth)
			c.emitLoop(lp.start)
		}
	case *ReturnStmt:
		c.line = s.Line
		if c.enclosing == nil {
			// return no nível do arquivo encerra o módulo
		}
		if s.X == nil {
			c.emit(OpNil)
		} else {
			c.expr(s.X)
		}
		c.emit(OpReturn)
	case *TryStmt:
		c.line = s.Line
		jCatch := c.emitJump(OpTry)
		c.tryDepth++
		c.block(s.Body)
		c.tryDepth--
		c.emit(OpEndTry)
		jEnd := c.emitJump(OpJump)
		c.patch(jCatch)
		c.begin()
		c.declare(s.ErrName, s.Line) // a VM empurra o erro neste slot
		for _, st := range s.Catch {
			c.stmt(st)
		}
		c.end()
		c.patch(jEnd)
	case *ImportStmt:
		c.line = s.Line
		isPath := byte(0)
		if s.IsPath {
			isPath = 1
		}
		k := c.konst(s.Path)
		c.emit(OpImport, byte(k>>8), byte(k), isPath)
		if c.isGlobalScope() {
			c.emit16(OpDefGlobal, c.konst(s.Alias))
		} else {
			c.declare(s.Alias, s.Line)
		}
	default:
		c.fail(c.line, fmt.Sprintf("instrução desconhecida %T", s))
	}
}

var compoundOp = map[string]byte{"+=": OpAdd, "-=": OpSub, "*=": OpMul, "/=": OpDiv}

func (c *compiler) assign(s *AssignStmt) {
	c.line = s.Line
	op, compound := compoundOp[s.Op]
	switch t := s.Target.(type) {
	case *IdentExpr:
		if compound {
			c.expr(t)
			c.expr(s.Val)
			c.line = s.Line
			c.emit(op)
		} else {
			c.expr(s.Val)
		}
		c.line = s.Line
		c.setVar(t.Name, s.Line)
		c.emit(OpPop)
	case *GetExpr:
		c.expr(t.Obj)
		k := c.konst(t.Name)
		if compound {
			c.emit(OpDup)
			c.emit16(OpGetField, k)
			c.expr(s.Val)
			c.line = s.Line
			c.emit(op)
		} else {
			c.expr(s.Val)
		}
		c.line = s.Line
		c.emit16(OpSetField, k)
	case *IndexExpr:
		c.expr(t.Obj)
		c.expr(t.Idx)
		if compound {
			c.emit(OpDup2)
			c.emit(OpGetIndex)
			c.expr(s.Val)
			c.line = s.Line
			c.emit(op)
		} else {
			c.expr(s.Val)
		}
		c.line = s.Line
		c.emit(OpSetIndex)
	}
}

func (c *compiler) setVar(name string, line int) {
	if l := c.resolveLocal(name); l >= 0 && !(c.enclosing == nil && l == 0) {
		c.emit16(OpSetLocal, l)
		return
	}
	if u := c.resolveUpval(name); u >= 0 {
		c.emit16(OpSetUpval, u)
		return
	}
	c.emit16(OpSetGlobal, c.konst(name))
}

// ---------- expressões ----------

var binOps = map[string]byte{"+": OpAdd, "-": OpSub, "*": OpMul, "/": OpDiv, "//": OpIDiv, "%": OpMod,
	"==": OpEq, "!=": OpNe, "<": OpLt, ">": OpGt, "<=": OpLe, ">=": OpGe}

func (c *compiler) expr(e Expr) {
	switch e := e.(type) {
	case *LitExpr:
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
	case *IdentExpr:
		c.line = e.Line
		if l := c.resolveLocal(e.Name); l >= 0 && !(c.enclosing == nil && l == 0) {
			c.emit16(OpGetLocal, l)
		} else if u := c.resolveUpval(e.Name); u >= 0 {
			c.emit16(OpGetUpval, u)
		} else {
			c.emit16(OpGetGlobal, c.konst(e.Name))
		}
	case *BinaryExpr:
		c.expr(e.L)
		c.expr(e.R)
		c.line = e.Line
		c.emit(binOps[e.Op])
	case *LogicalExpr:
		c.expr(e.L)
		var j int
		if e.Op == "and" {
			j = c.emitJump(OpJumpIfFalse)
		} else {
			j = c.emitJump(OpJumpIfTrue)
		}
		c.emit(OpPop)
		c.expr(e.R)
		c.patch(j)
	case *UnaryExpr:
		c.expr(e.X)
		c.line = e.Line
		if e.Op == "-" {
			c.emit(OpNeg)
		} else {
			c.emit(OpNot)
		}
	case *CallExpr:
		c.expr(e.Fn)
		if len(e.Args) > 255 {
			c.fail(e.Line, "argumentos demais (máximo 255)")
		}
		for _, a := range e.Args {
			c.expr(a)
		}
		c.line = e.Line
		c.emit(OpCall, byte(len(e.Args)))
	case *GetExpr:
		c.expr(e.Obj)
		c.line = e.Line
		c.emit16(OpGetField, c.konst(e.Name))
	case *IndexExpr:
		c.expr(e.Obj)
		c.expr(e.Idx)
		c.line = e.Line
		c.emit(OpGetIndex)
	case *ListExpr:
		for _, it := range e.Items {
			c.expr(it)
		}
		c.emit16(OpList, len(e.Items))
	case *TableExpr:
		for i, k := range e.Keys {
			c.emit16(OpConst, c.konst(k))
			c.expr(e.Vals[i])
		}
		c.emit16(OpTable, len(e.Keys))
	case *FnExpr:
		c.function(e)
	default:
		c.fail(c.line, fmt.Sprintf("expressão desconhecida %T", e))
	}
}

func (c *compiler) function(f *FnExpr) {
	fc := &compiler{enclosing: c, file: c.file, line: f.Line,
		proto: &Proto{Name: f.Name, Arity: len(f.Params), Variadic: f.Variadic, Module: c.proto.Module, File: c.file}}
	fc.locals = append(fc.locals, local{name: "", depth: 0})
	fc.depth = 1
	for _, p := range f.Params {
		fc.declare(p, f.Line)
	}
	n := len(f.Body)
	for i, s := range f.Body {
		if es, ok := s.(*ExprStmt); ok && i == n-1 {
			// a última expressão do corpo é o valor de retorno
			fc.line = es.Line
			fc.expr(es.X)
			fc.emit(OpReturn)
			continue
		}
		fc.stmt(s)
	}
	fc.emit(OpNil)
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

// Disassemble mostra o bytecode de uma função (nep disasm).
func Disassemble(p *Proto) string {
	var b strings.Builder
	var dis func(p *Proto, depth int)
	dis = func(p *Proto, depth int) {
		name := p.Name
		if name == "" {
			name = "<principal>"
		}
		pad := strings.Repeat("  ", depth)
		fmt.Fprintf(&b, "%s== %s (aridade %d, upvalues %d) ==\n", pad, name, p.Arity, p.NUpvalues)
		code := p.Code
		for i := 0; i < len(code); {
			op := code[i]
			fmt.Fprintf(&b, "%s%04d  L%-4d %-18s", pad, i, p.Lines[i], opNames[op])
			u16 := func(at int) int { return int(code[at])<<8 | int(code[at+1]) }
			switch op {
			case OpConst, OpGetGlobal, OpSetGlobal, OpDefGlobal, OpGetField, OpSetField:
				fmt.Fprintf(&b, " %d (%s)", u16(i+1), Repr(p.Consts[u16(i+1)]))
				i += 3
			case OpGetLocal, OpSetLocal, OpGetUpval, OpSetUpval, OpList, OpTable:
				fmt.Fprintf(&b, " %d", u16(i+1))
				i += 3
			case OpJump, OpJumpIfFalse, OpJumpIfTrue, OpPopJumpIfFalse, OpTry:
				fmt.Fprintf(&b, " -> %d", i+3+u16(i+1))
				i += 3
			case OpLoop:
				fmt.Fprintf(&b, " -> %d", i+3-u16(i+1))
				i += 3
			case OpCall:
				fmt.Fprintf(&b, " %d", code[i+1])
				i += 2
			case OpIterNext:
				fmt.Fprintf(&b, " slot %d vars %d -> %d", code[i+1], code[i+2], i+5+u16(i+3))
				i += 5
			case OpImport:
				fmt.Fprintf(&b, " %s", Repr(p.Consts[u16(i+1)]))
				i += 4
			case OpClosure:
				fp := p.Consts[u16(i+1)].(*Proto)
				fmt.Fprintf(&b, " <fn %s>", fp.Name)
				i += 3 + 3*fp.NUpvalues
			default:
				i++
			}
			b.WriteByte('\n')
		}
		for _, k := range p.Consts {
			if fp, ok := k.(*Proto); ok {
				dis(fp, depth+1)
			}
		}
	}
	dis(p, 0)
	return b.String()
}
