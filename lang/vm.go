package lang

import (
	"fmt"
	"io"
	"math"
	"os"
	"path/filepath"
	"strings"
	"unicode/utf8"
)

const (
	stackMax  = 1 << 16
	framesMax = 4000
)

// RuntimeError é um erro durante a execução, com o valor lançado e o rastro de chamadas.
type RuntimeError struct {
	Value Value
	Trace []string
}

func (e *RuntimeError) Error() string {
	msg := Str(e.Value)
	if t, ok := e.Value.(*Table); ok {
		if m, ok := t.Get("message"); ok {
			msg = Str(m)
		}
	}
	var b strings.Builder
	b.WriteString("Erro: " + msg)
	for _, t := range e.Trace {
		b.WriteString("\n  " + t)
	}
	return b.String()
}

// thrown é um erro lançado por error() ou por uma função nativa com valor próprio.
type thrown struct{ val Value }

func (t *thrown) Error() string { return Str(t.val) }

// Throw cria um erro Nepenthes com qualquer valor (usado por error()).
func Throw(v Value) error { return &thrown{v} }

type frame struct {
	cl   *Closure
	ip   int
	base int
}

type handler struct {
	frame int
	ip    int
	sp    int
}

type iterator struct {
	list  []Value
	table *Table
	runes []rune
	i     int
	kind  int // 0 lista, 1 tabela, 2 texto
}

// VM executa bytecode Nepenthes.
type VM struct {
	stack    [stackMax]Value
	sp       int
	frames   []frame
	handlers []handler
	open     []*Upvalue // upvalues abertos, ordenados por slot
	Builtins map[string]Value
	modules  map[string]*Module
	stdmods  map[string]func(vm *VM) *Module
	Out      io.Writer
	In       io.Reader
	Args     []string
	loading  map[string]bool
}

func NewVM() *VM {
	vm := &VM{Out: os.Stdout, In: os.Stdin, modules: map[string]*Module{}, loading: map[string]bool{}}
	vm.frames = make([]frame, 0, framesMax+1) // capacidade fixa: ponteiros para frames nunca ficam inválidos
	vm.Builtins = coreBuiltins()
	vm.stdmods = stdModules()
	return vm
}

// RunFile executa um arquivo .nep como programa principal.
func (vm *VM) RunFile(path string) error {
	src, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("não consegui abrir %s: %v", path, err)
	}
	abs, _ := filepath.Abs(path)
	mod := NewModule(baseName(path), abs)
	vm.modules[abs] = mod
	_, err = vm.RunSource(string(src), path, mod)
	return err
}

// RunSource compila e executa código em um módulo; devolve o valor de retorno do módulo.
func (vm *VM) RunSource(src, file string, mod *Module) (Value, error) {
	prog, err := Parse(src, file)
	if err != nil {
		return nil, err
	}
	proto, err := Compile(prog, mod, file)
	if err != nil {
		return nil, err
	}
	return vm.Call(&Closure{Proto: proto}, nil)
}

// Call chama uma função Nepenthes ou nativa e devolve o resultado.
func (vm *VM) Call(fn Value, args []Value) (Value, error) {
	if n, ok := fn.(*Native); ok {
		return n.Fn(vm, args)
	}
	if vm.sp+len(args)+1 >= stackMax {
		return nil, vm.wrap(fmt.Errorf("pilha cheia"))
	}
	base := len(vm.frames)
	baseSP := vm.sp
	vm.stack[vm.sp] = fn
	vm.sp++
	for _, a := range args {
		vm.stack[vm.sp] = a
		vm.sp++
	}
	if err := vm.callValue(fn, len(args)); err != nil {
		vm.sp = baseSP
		return nil, vm.wrap(err)
	}
	if len(vm.frames) == base { // nativa já resolveu
		vm.sp--
		return vm.stack[vm.sp], nil
	}
	res, err := vm.run(base)
	if err != nil {
		vm.closeUpvals(baseSP)
		vm.sp = baseSP
		vm.frames = vm.frames[:base]
		return nil, err
	}
	return res, nil
}

func (vm *VM) push(v Value)     { vm.stack[vm.sp] = v; vm.sp++ }
func (vm *VM) pop() Value       { vm.sp--; v := vm.stack[vm.sp]; vm.stack[vm.sp] = nil; return v }
func (vm *VM) peek(d int) Value { return vm.stack[vm.sp-1-d] }

func (vm *VM) callValue(callee Value, argc int) error {
	switch f := callee.(type) {
	case *Closure:
		p := f.Proto
		if p.Variadic {
			fixed := p.Arity - 1
			var rest []Value
			if argc > fixed {
				rest = append(rest, vm.stack[vm.sp-(argc-fixed):vm.sp]...)
				for i := 0; i < argc-fixed; i++ {
					vm.pop()
				}
				argc = fixed
			}
			for argc < fixed {
				vm.push(nil)
				argc++
			}
			vm.push(NewList(rest...))
			argc++
		} else {
			if argc > p.Arity {
				name := p.Name
				if name == "" {
					name = "anônima"
				}
				return fmt.Errorf("a função %s recebe %d argumento(s), mas recebeu %d", name, p.Arity, argc)
			}
			for argc < p.Arity {
				vm.push(nil)
				argc++
			}
		}
		if len(vm.frames) >= framesMax {
			return fmt.Errorf("recursão profunda demais (mais de %d chamadas)", framesMax)
		}
		if vm.sp+256 >= stackMax {
			return fmt.Errorf("pilha cheia")
		}
		vm.frames = append(vm.frames, frame{cl: f, ip: 0, base: vm.sp - argc - 1})
		return nil
	case *Native:
		args := make([]Value, argc)
		copy(args, vm.stack[vm.sp-argc:vm.sp])
		res, err := f.Fn(vm, args)
		if err != nil {
			return err
		}
		vm.sp -= argc + 1
		for i := vm.sp; i < vm.sp+argc+1; i++ {
			vm.stack[i] = nil
		}
		vm.push(res)
		return nil
	}
	return fmt.Errorf("tentou chamar um valor %s, que não é uma função", TypeName(callee))
}

func (vm *VM) capture(slot int) *Upvalue {
	i := len(vm.open) - 1
	for ; i >= 0 && vm.open[i].slot > slot; i-- {
	}
	if i >= 0 && vm.open[i].slot == slot {
		return vm.open[i]
	}
	u := &Upvalue{loc: &vm.stack[slot], slot: slot}
	vm.open = append(vm.open, nil)
	copy(vm.open[i+2:], vm.open[i+1:])
	vm.open[i+1] = u
	return u
}

func (vm *VM) closeUpvals(from int) {
	for len(vm.open) > 0 && vm.open[len(vm.open)-1].slot >= from {
		u := vm.open[len(vm.open)-1]
		u.closed = *u.loc
		u.loc = &u.closed
		vm.open = vm.open[:len(vm.open)-1]
	}
}

// wrap transforma qualquer erro em RuntimeError com rastro.
func (vm *VM) wrap(err error) *RuntimeError {
	if re, ok := err.(*RuntimeError); ok {
		return re
	}
	var val Value = err.Error()
	if t, ok := err.(*thrown); ok {
		val = t.val
	}
	re := &RuntimeError{Value: val}
	for i := len(vm.frames) - 1; i >= 0 && len(re.Trace) < 12; i-- {
		f := vm.frames[i]
		p := f.cl.Proto
		ip := f.ip - 1
		if ip < 0 {
			ip = 0
		}
		line := 0
		if ip < len(p.Lines) {
			line = p.Lines[ip]
		}
		name := p.Name
		if p.IsTop {
			name = "<principal>"
		} else if name == "" {
			name = "<fn anônima>"
		}
		re.Trace = append(re.Trace, fmt.Sprintf("em %s (%s:%d)", name, filepath.Base(p.File), line))
	}
	return re
}

// errValue converte um erro Go no valor que o catch recebe.
func errValue(err error) Value {
	if t, ok := err.(*thrown); ok {
		return t.val
	}
	if re, ok := err.(*RuntimeError); ok {
		return re.Value
	}
	return err.Error()
}

func (vm *VM) run(base int) (Value, error) {
	fr := &vm.frames[len(vm.frames)-1]
	code := fr.cl.Proto.Code
	consts := fr.cl.Proto.Consts

	reload := func() {
		fr = &vm.frames[len(vm.frames)-1]
		code = fr.cl.Proto.Code
		consts = fr.cl.Proto.Consts
	}
	u16 := func() int {
		v := int(code[fr.ip])<<8 | int(code[fr.ip+1])
		fr.ip += 2
		return v
	}

	for {
		var err error
		op := code[fr.ip]
		fr.ip++
		switch op {
		case OpConst:
			vm.push(consts[u16()])
		case OpNil:
			vm.push(nil)
		case OpTrue:
			vm.push(true)
		case OpFalse:
			vm.push(false)
		case OpPop:
			vm.pop()
		case OpDup:
			vm.push(vm.peek(0))
		case OpDup2:
			a, b := vm.peek(1), vm.peek(0)
			vm.push(a)
			vm.push(b)
		case OpGetLocal:
			vm.push(vm.stack[fr.base+u16()])
		case OpSetLocal:
			vm.stack[fr.base+u16()] = vm.peek(0)
		case OpGetUpval:
			vm.push(*fr.cl.Upvalues[u16()].loc)
		case OpSetUpval:
			*fr.cl.Upvalues[u16()].loc = vm.peek(0)
		case OpGetGlobal:
			name := consts[u16()].(string)
			if v, ok := fr.cl.Proto.Module.Globals[name]; ok {
				vm.push(v)
			} else if v, ok := vm.Builtins[name]; ok {
				vm.push(v)
			} else {
				err = fmt.Errorf("variável não definida: %s", name)
			}
		case OpSetGlobal:
			name := consts[u16()].(string)
			if _, ok := fr.cl.Proto.Module.Globals[name]; ok {
				fr.cl.Proto.Module.Globals[name] = vm.peek(0)
			} else {
				err = fmt.Errorf("variável não declarada: %s (use 'let %s = ...' para criar)", name, name)
			}
		case OpDefGlobal:
			fr.cl.Proto.Module.Define(consts[u16()].(string), vm.pop())
		case OpGetField:
			name := consts[u16()].(string)
			var v Value
			v, err = getField(vm.pop(), name)
			if err == nil {
				vm.push(v)
			}
		case OpSetField:
			name := consts[u16()].(string)
			val := vm.pop()
			obj := vm.pop()
			switch o := obj.(type) {
			case *Table:
				o.SetStr(name, val)
			case *Module:
				err = fmt.Errorf("não é possível alterar o módulo %s de fora", o.Name)
			default:
				err = fmt.Errorf("não dá para definir o campo '%s' em %s (só em tabelas)", name, TypeName(obj))
			}
		case OpGetIndex:
			idx := vm.pop()
			obj := vm.pop()
			var v Value
			v, err = getIndex(obj, idx)
			if err == nil {
				vm.push(v)
			}
		case OpSetIndex:
			val := vm.pop()
			idx := vm.pop()
			obj := vm.pop()
			err = setIndex(obj, idx, val)
		case OpAdd, OpSub, OpMul, OpDiv, OpIDiv, OpMod:
			b := vm.pop()
			a := vm.pop()
			var v Value
			v, err = arith(op, a, b)
			if err == nil {
				vm.push(v)
			}
		case OpNeg:
			switch x := vm.pop().(type) {
			case int64:
				vm.push(-x)
			case float64:
				vm.push(-x)
			default:
				err = fmt.Errorf("sinal de menos em %s, que não é número", TypeName(x))
			}
		case OpNot:
			vm.push(!Truthy(vm.pop()))
		case OpEq:
			b := vm.pop()
			vm.push(Equal(vm.pop(), b))
		case OpNe:
			b := vm.pop()
			vm.push(!Equal(vm.pop(), b))
		case OpLt, OpGt, OpLe, OpGe:
			b := vm.pop()
			a := vm.pop()
			var c int
			c, err = Compare(a, b)
			if err == nil {
				switch op {
				case OpLt:
					vm.push(c < 0)
				case OpGt:
					vm.push(c > 0)
				case OpLe:
					vm.push(c <= 0)
				default:
					vm.push(c >= 0)
				}
			}
		case OpJump:
			off := u16()
			fr.ip += off
		case OpJumpIfFalse:
			off := u16()
			if !Truthy(vm.peek(0)) {
				fr.ip += off
			}
		case OpJumpIfTrue:
			off := u16()
			if Truthy(vm.peek(0)) {
				fr.ip += off
			}
		case OpPopJumpIfFalse:
			off := u16()
			if !Truthy(vm.pop()) {
				fr.ip += off
			}
		case OpLoop:
			off := u16()
			fr.ip -= off
		case OpCall:
			argc := int(code[fr.ip])
			fr.ip++
			callee := vm.stack[vm.sp-argc-1]
			if err = vm.callValue(callee, argc); err == nil {
				reload()
			}
		case OpClosure:
			p := consts[u16()].(*Proto)
			cl := &Closure{Proto: p, Upvalues: make([]*Upvalue, p.NUpvalues)}
			for i := 0; i < p.NUpvalues; i++ {
				isLocal := code[fr.ip]
				idx := int(code[fr.ip+1])<<8 | int(code[fr.ip+2])
				fr.ip += 3
				if isLocal == 1 {
					cl.Upvalues[i] = vm.capture(fr.base + idx)
				} else {
					cl.Upvalues[i] = fr.cl.Upvalues[idx]
				}
			}
			vm.push(cl)
		case OpCloseUpval:
			vm.closeUpvals(vm.sp - 1)
			vm.pop()
		case OpReturn:
			res := vm.pop()
			vm.closeUpvals(fr.base)
			for len(vm.handlers) > 0 && vm.handlers[len(vm.handlers)-1].frame >= len(vm.frames)-1 {
				vm.handlers = vm.handlers[:len(vm.handlers)-1]
			}
			for i := fr.base; i < vm.sp; i++ {
				vm.stack[i] = nil
			}
			vm.sp = fr.base
			vm.frames = vm.frames[:len(vm.frames)-1]
			if len(vm.frames) == base {
				return res, nil
			}
			vm.push(res)
			reload()
		case OpList:
			n := u16()
			items := make([]Value, n)
			copy(items, vm.stack[vm.sp-n:vm.sp])
			for i := 0; i < n; i++ {
				vm.pop()
			}
			vm.push(&List{Items: items})
		case OpTable:
			n := u16()
			t := NewTable()
			start := vm.sp - 2*n
			for i := 0; i < n; i++ {
				_ = t.Set(vm.stack[start+2*i], vm.stack[start+2*i+1])
			}
			for i := 0; i < 2*n; i++ {
				vm.pop()
			}
			vm.push(t)
		case OpIterPrep:
			coll := vm.pop()
			it := &iterator{}
			switch c := coll.(type) {
			case nil:
				it.kind = 0
			case *List:
				it.list = append([]Value(nil), c.Items...)
			case *Table:
				it.kind, it.table = 1, c
			case string:
				it.kind, it.runes = 2, []rune(c)
			default:
				err = fmt.Errorf("for ... in precisa de uma lista, tabela ou texto (recebeu %s)", TypeName(coll))
			}
			if err == nil {
				vm.push(it)
			}
		case OpIterNext:
			slot := int(code[fr.ip])
			nvars := int(code[fr.ip+1])
			fr.ip += 2
			off := u16()
			it := vm.stack[fr.base+slot].(*iterator)
			var k, v Value
			ok := false
			switch it.kind {
			case 0:
				if it.i < len(it.list) {
					k, v, ok = int64(it.i), it.list[it.i], true
				}
			case 1:
				if it.i < it.table.Len() {
					k, v = it.table.Entry(it.i)
					ok = true
				}
			case 2:
				if it.i < len(it.runes) {
					k, v, ok = int64(it.i), string(it.runes[it.i]), true
				}
			}
			if !ok {
				fr.ip += off
				break
			}
			it.i++
			if nvars == 2 {
				vm.push(k)
			}
			vm.push(v)
		case OpTry:
			off := u16()
			vm.handlers = append(vm.handlers, handler{frame: len(vm.frames) - 1, ip: fr.ip + off, sp: vm.sp})
		case OpEndTry:
			vm.handlers = vm.handlers[:len(vm.handlers)-1]
		case OpImport:
			k := u16()
			isPath := code[fr.ip] == 1
			fr.ip++
			var m *Module
			m, err = vm.importModule(consts[k].(string), isPath, fr.cl.Proto.File)
			if err == nil {
				vm.push(m)
			}
		default:
			err = fmt.Errorf("instrução desconhecida %d", op)
		}

		if err != nil {
			// procura um try/catch dentro desta execução
			if len(vm.handlers) > 0 && vm.handlers[len(vm.handlers)-1].frame >= base {
				h := vm.handlers[len(vm.handlers)-1]
				vm.handlers = vm.handlers[:len(vm.handlers)-1]
				val := errValue(err)
				if _, isRE := err.(*RuntimeError); !isRE {
					if _, isThrown := err.(*thrown); !isThrown {
						val = vm.errorTable(err.Error())
					}
				} else if s, ok := val.(string); ok {
					val = vm.errorTable(s)
				}
				vm.closeUpvals(h.sp)
				for i := h.sp; i < vm.sp; i++ {
					vm.stack[i] = nil
				}
				vm.sp = h.sp
				vm.frames = vm.frames[:h.frame+1]
				reload()
				fr.ip = h.ip
				vm.push(val)
				continue
			}
			return nil, vm.wrap(err)
		}
	}
}

// errorTable cria o valor de erro padrão: {message, line, file}.
func (vm *VM) errorTable(msg string) *Table {
	t := NewTable()
	t.SetStr("message", msg)
	if len(vm.frames) > 0 {
		f := vm.frames[len(vm.frames)-1]
		ip := f.ip - 1
		if ip >= 0 && ip < len(f.cl.Proto.Lines) {
			t.SetStr("line", int64(f.cl.Proto.Lines[ip]))
		}
		t.SetStr("file", filepath.Base(f.cl.Proto.File))
	}
	return t
}

func getField(obj Value, name string) (Value, error) {
	switch o := obj.(type) {
	case *Table:
		v, _ := o.Get(name)
		return v, nil
	case *Module:
		v, ok := o.Globals[name]
		if !ok {
			return nil, fmt.Errorf("o módulo %s não tem '%s'", o.Name, name)
		}
		return v, nil
	case nil:
		return nil, fmt.Errorf("tentou ler o campo '%s' de nil", name)
	}
	return nil, fmt.Errorf("%s não tem campos (tentou ler '%s')", TypeName(obj), name)
}

func listIndex(n int, idx Value) (int, bool) {
	var i int64
	switch x := idx.(type) {
	case int64:
		i = x
	case float64:
		if x != math.Trunc(x) {
			return 0, false
		}
		i = int64(x)
	default:
		return 0, false
	}
	if i < 0 {
		i += int64(n)
	}
	return int(i), true
}

func getIndex(obj, idx Value) (Value, error) {
	switch o := obj.(type) {
	case *List:
		i, ok := listIndex(len(o.Items), idx)
		if !ok {
			return nil, fmt.Errorf("índice de lista precisa ser inteiro (recebeu %s)", TypeName(idx))
		}
		if i < 0 || i >= len(o.Items) {
			return nil, nil
		}
		return o.Items[i], nil
	case *Table:
		v, _ := o.Get(idx)
		return v, nil
	case string:
		rs := []rune(o)
		i, ok := listIndex(len(rs), idx)
		if !ok {
			return nil, fmt.Errorf("índice de texto precisa ser inteiro")
		}
		if i < 0 || i >= len(rs) {
			return nil, nil
		}
		return string(rs[i]), nil
	case *Module:
		if s, ok := idx.(string); ok {
			return getField(o, s)
		}
	case nil:
		return nil, fmt.Errorf("tentou indexar nil")
	}
	return nil, fmt.Errorf("não dá para indexar %s", TypeName(obj))
}

func setIndex(obj, idx, val Value) error {
	switch o := obj.(type) {
	case *List:
		i, ok := listIndex(len(o.Items), idx)
		if !ok {
			return fmt.Errorf("índice de lista precisa ser inteiro (recebeu %s)", TypeName(idx))
		}
		if i == len(o.Items) {
			o.Items = append(o.Items, val)
			return nil
		}
		if i < 0 || i > len(o.Items) {
			return fmt.Errorf("índice %s fora da lista (tamanho %d)", Str(idx), len(o.Items))
		}
		o.Items[i] = val
		return nil
	case *Table:
		return o.Set(idx, val)
	}
	return fmt.Errorf("não dá para alterar um índice de %s", TypeName(obj))
}

func arith(op byte, a, b Value) (Value, error) {
	ai, aInt := a.(int64)
	bi, bInt := b.(int64)
	if aInt && bInt {
		switch op {
		case OpAdd:
			return ai + bi, nil
		case OpSub:
			return ai - bi, nil
		case OpMul:
			return ai * bi, nil
		case OpDiv:
			if bi == 0 {
				return nil, fmt.Errorf("divisão por zero")
			}
			if ai%bi == 0 {
				return ai / bi, nil
			}
			return float64(ai) / float64(bi), nil
		case OpIDiv:
			if bi == 0 {
				return nil, fmt.Errorf("divisão por zero")
			}
			q := ai / bi
			if (ai%bi != 0) && ((ai < 0) != (bi < 0)) {
				q--
			}
			return q, nil
		case OpMod:
			if bi == 0 {
				return nil, fmt.Errorf("divisão por zero")
			}
			m := ai % bi
			if m != 0 && (m < 0) != (bi < 0) {
				m += bi
			}
			return m, nil
		}
	}
	x, xok := toFloat(a)
	y, yok := toFloat(b)
	if xok && yok {
		switch op {
		case OpAdd:
			return x + y, nil
		case OpSub:
			return x - y, nil
		case OpMul:
			return x * y, nil
		case OpDiv:
			if y == 0 {
				return nil, fmt.Errorf("divisão por zero")
			}
			return x / y, nil
		case OpIDiv:
			if y == 0 {
				return nil, fmt.Errorf("divisão por zero")
			}
			return math.Floor(x / y), nil
		case OpMod:
			if y == 0 {
				return nil, fmt.Errorf("divisão por zero")
			}
			return x - y*math.Floor(x/y), nil
		}
	}
	if op == OpAdd {
		if la, ok := a.(*List); ok {
			if lb, ok := b.(*List); ok {
				items := make([]Value, 0, len(la.Items)+len(lb.Items))
				items = append(append(items, la.Items...), lb.Items...)
				return &List{Items: items}, nil
			}
		}
		_, as := a.(string)
		_, bs := b.(string)
		if as || bs {
			return Str(a) + Str(b), nil
		}
	}
	if op == OpMul {
		if s, ok := a.(string); ok && bInt {
			if bi < 0 || bi > 1<<24 {
				return nil, fmt.Errorf("repetição de texto inválida")
			}
			return strings.Repeat(s, int(bi)), nil
		}
	}
	names := map[byte]string{OpAdd: "+", OpSub: "-", OpMul: "*", OpDiv: "/", OpIDiv: "//", OpMod: "%"}
	return nil, fmt.Errorf("a operação '%s' não funciona entre %s e %s", names[op], TypeName(a), TypeName(b))
}

// ---------- módulos ----------

func (vm *VM) importModule(name string, isPath bool, fromFile string) (*Module, error) {
	if !isPath {
		if m, ok := vm.modules["std:"+name]; ok {
			return m, nil
		}
		if mk, ok := vm.stdmods[name]; ok {
			m := mk(vm)
			vm.modules["std:"+name] = m
			return m, nil
		}
		if src, ok := stdSource(name); ok {
			m := NewModule(name, "<std>/"+name+".nep")
			vm.modules["std:"+name] = m
			prog, err := Parse(src, name+".nep")
			if err != nil {
				return nil, err
			}
			proto, err := Compile(prog, m, name+".nep")
			if err != nil {
				return nil, err
			}
			if _, err := vm.Call(&Closure{Proto: proto}, nil); err != nil {
				delete(vm.modules, "std:"+name)
				return nil, err
			}
			return m, nil
		}
	}
	dir := "."
	if fromFile != "" && fromFile != "<repl>" {
		dir = filepath.Dir(fromFile)
	}
	p := name
	if !strings.HasSuffix(p, ".nep") {
		p += ".nep"
	}
	if !filepath.IsAbs(p) {
		p = filepath.Join(dir, p)
	}
	abs, _ := filepath.Abs(p)
	if m, ok := vm.modules[abs]; ok {
		if vm.loading[abs] {
			return nil, fmt.Errorf("importação circular envolvendo %s", filepath.Base(abs))
		}
		return m, nil
	}
	src, err := os.ReadFile(abs)
	if err != nil {
		if !isPath {
			return nil, fmt.Errorf("módulo '%s' não encontrado (nem na biblioteca padrão, nem como %s)", name, filepath.Base(p))
		}
		return nil, fmt.Errorf("não consegui importar %s: arquivo não encontrado", name)
	}
	m := NewModule(baseName(abs), abs)
	vm.modules[abs] = m
	vm.loading[abs] = true
	defer delete(vm.loading, abs)
	prog, err := Parse(string(src), abs)
	if err != nil {
		delete(vm.modules, abs)
		return nil, err
	}
	proto, err := Compile(prog, m, abs)
	if err != nil {
		delete(vm.modules, abs)
		return nil, err
	}
	if _, err := vm.Call(&Closure{Proto: proto}, nil); err != nil {
		delete(vm.modules, abs)
		return nil, err
	}
	return m, nil
}

// RuneLen conta caracteres (não bytes).
func RuneLen(s string) int { return utf8.RuneCountInString(s) }
