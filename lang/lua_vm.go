package lang

import (
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// Tuple carrega vários valores de uma vez (retorno múltiplo e ... do Lua).
// Só existe em trânsito: o compilador reduz a um valor onde só cabe um.
type Tuple struct{ V []Value }

func (t *Tuple) first() Value {
	if len(t.V) == 0 {
		return nil
	}
	return t.V[0]
}

// Multi cria um retorno com vários valores (para funções nativas).
func Multi(vals ...Value) Value {
	if len(vals) == 1 {
		return vals[0]
	}
	return &Tuple{V: vals}
}

// Values abre um retorno múltiplo em uma lista de valores.
func Values(v Value) []Value {
	if t, ok := v.(*Tuple); ok {
		return t.V
	}
	return []Value{v}
}

func single(v Value) Value {
	if t, ok := v.(*Tuple); ok {
		return t.first()
	}
	return v
}

func luaTruthy(v Value) bool {
	switch x := v.(type) {
	case nil:
		return false
	case bool:
		return x
	}
	return true
}

// ---------- metatabelas ----------

func (vm *VM) metaOf(v Value) *Table {
	switch x := v.(type) {
	case *Table:
		return x.meta
	case string:
		return vm.lua().stringMeta
	}
	return nil
}

func (vm *VM) metamethod(v Value, ev string) Value {
	if m := vm.metaOf(v); m != nil {
		return m.rawStr(ev)
	}
	return nil
}

func isCallable(v Value) bool {
	switch x := v.(type) {
	case *Closure, *Native:
		return true
	case *Table:
		return x.meta != nil && x.meta.rawStr("__call") != nil
	}
	return false
}

func luaTypeName(v Value) string {
	switch v.(type) {
	case nil:
		return "nil"
	case bool:
		return "boolean"
	case int64, float64:
		return "number"
	case string, HTML:
		return "string"
	case *Table, *List, *Module:
		return "table"
	case *Closure, *Native:
		return "function"
	case *Coroutine:
		return "thread"
	}
	return "userdata"
}

// luaIndex lê obj[key] seguindo __index.
func (vm *VM) luaIndex(obj, key Value) (Value, error) {
	for loop := 0; loop < 100; loop++ {
		var h Value
		switch o := obj.(type) {
		case *Table:
			v, _ := o.Get(key)
			if v != nil || o.meta == nil {
				return v, nil
			}
			h = o.meta.rawStr("__index")
			if h == nil {
				return nil, nil
			}
		case string, HTML:
			v, _ := vm.lua().stringLib.Get(key)
			return v, nil
		case *List:
			if i, ok := luaIntKey(key); ok {
				if i >= 1 && i <= int64(len(o.Items)) {
					return o.Items[i-1], nil
				}
				return nil, nil
			}
			if s, ok := key.(string); ok {
				if f, ok := vm.Builtins[s]; ok { // l:map(f) etc.
					return f, nil
				}
			}
			return nil, nil
		case *Module:
			if s, ok := key.(string); ok {
				return o.Globals[s], nil
			}
			return nil, nil
		default:
			return nil, fmt.Errorf("tentou indexar um valor %s", luaTypeName(obj))
		}
		if isFn(h) {
			r, err := vm.Call(h, []Value{obj, key})
			return r, err
		}
		obj = h
	}
	return nil, fmt.Errorf("laço de '__index' longo demais")
}

func isFn(v Value) bool {
	switch v.(type) {
	case *Closure, *Native:
		return true
	}
	return false
}

func luaIntKey(k Value) (int64, bool) {
	switch x := k.(type) {
	case int64:
		return x, true
	case float64:
		if x == math.Trunc(x) && math.Abs(x) < 1<<53 {
			return int64(x), true
		}
	}
	return 0, false
}

// luaSetIndex grava obj[key] = val seguindo __newindex. lua=false mantém
// as regras do Nepenthes para a gravação final (nil fica guardado).
func (vm *VM) luaSetIndex(obj, key, val Value, lua bool) error {
	for loop := 0; loop < 100; loop++ {
		switch o := obj.(type) {
		case *Table:
			if o.meta != nil {
				if h := o.meta.rawStr("__newindex"); h != nil {
					if cur, _ := o.Get(key); cur == nil {
						if isFn(h) {
							_, err := vm.Call(h, []Value{obj, key, val})
							return err
						}
						obj = h
						continue
					}
				}
			}
			if lua {
				return o.luaSet(key, val)
			}
			return o.Set(key, val)
		case *List:
			if !lua {
				return setIndex(obj, key, val)
			}
			i, ok := luaIntKey(key)
			if !ok || i < 1 || i > int64(len(o.Items))+1 {
				return fmt.Errorf("índice %s fora da lista (tamanho %d)", Str(key), len(o.Items))
			}
			if i == int64(len(o.Items))+1 {
				o.Items = append(o.Items, val)
			} else {
				o.Items[i-1] = val
			}
			return nil
		case *Module:
			return fmt.Errorf("não é possível alterar o módulo %s de fora", o.Name)
		default:
			if lua {
				return fmt.Errorf("tentou indexar um valor %s", luaTypeName(obj))
			}
			return setIndex(obj, key, val)
		}
	}
	return fmt.Errorf("laço de '__newindex' longo demais")
}

// ---------- números e textos ----------

// luaToNumber converte para número como na aritmética do Lua ("10" + 1 == 11).
func luaToNumber(v Value) (Value, bool) {
	switch x := v.(type) {
	case int64, float64:
		return x, true
	case string:
		return parseLuaNumber(strings.TrimSpace(x))
	}
	return nil, false
}

func luaToInteger(v Value) (int64, bool) {
	n, ok := luaToNumber(v)
	if !ok {
		return 0, false
	}
	switch x := n.(type) {
	case int64:
		return x, true
	case float64:
		if x == math.Floor(x) && x >= -9.223372036854775808e18 && x < 9.223372036854775808e18 {
			return int64(x), true
		}
	}
	return 0, false
}

func luaFmtFloat(f float64) string {
	switch {
	case math.IsInf(f, 1):
		return "inf"
	case math.IsInf(f, -1):
		return "-inf"
	case math.IsNaN(f):
		if math.Signbit(f) {
			return "-nan"
		}
		return "nan"
	}
	s := fmtG14(f)
	if !strings.ContainsAny(s, ".ne") {
		s += ".0"
	}
	return s
}

// fmtG14 formata como o "%.14g" do C.
func fmtG14(f float64) string {
	switch {
	case math.IsInf(f, 1):
		return "inf"
	case math.IsInf(f, -1):
		return "-inf"
	case math.IsNaN(f):
		if math.Signbit(f) {
			return "-nan"
		}
		return "nan"
	}
	s := strconv.FormatFloat(f, 'g', 14, 64)
	if strings.ContainsAny(s, "e") {
		// o Go escreve 1e+06; o C escreve 1e+06 também, mas com no mínimo 2 dígitos
		mant, exp, _ := strings.Cut(s, "e")
		sign := exp[0]
		digits := exp[1:]
		if len(digits) < 2 {
			digits = "0" + digits
		}
		return mant + "e" + string(sign) + digits
	}
	return s
}

func luaNumStr(v Value) (string, bool) {
	switch x := v.(type) {
	case int64:
		return strconv.FormatInt(x, 10), true
	case float64:
		return luaFmtFloat(x), true
	case string:
		return x, true
	case HTML:
		return string(x), true
	}
	return "", false
}

// luaToString é o tostring() do Lua.
func (vm *VM) luaToString(v Value) (string, error) {
	if h := vm.metamethod(v, "__tostring"); h != nil {
		r, err := vm.Call(h, []Value{v})
		if err != nil {
			return "", err
		}
		s, ok := luaNumStr(r)
		if !ok {
			return "", fmt.Errorf("'__tostring' precisa devolver um texto")
		}
		return s, nil
	}
	switch x := v.(type) {
	case nil:
		return "nil", nil
	case bool:
		if x {
			return "true", nil
		}
		return "false", nil
	case int64, float64, string, HTML:
		s, _ := luaNumStr(x)
		return s, nil
	case *Table:
		if x.meta != nil {
			if n, ok := x.meta.rawStr("__name").(string); ok {
				return fmt.Sprintf("%s: %p", n, x), nil
			}
		}
		return fmt.Sprintf("table: %p", x), nil
	case *Closure:
		return fmt.Sprintf("function: %p", x), nil
	case *Native:
		return fmt.Sprintf("function: builtin: %p", x), nil
	case *Coroutine:
		return fmt.Sprintf("thread: %p", x), nil
	case *List:
		return Repr(x), nil
	case *Module:
		return "module: " + x.Name, nil
	}
	return Str(v), nil
}

var arithEvents = map[byte]string{OpAdd: "__add", OpSub: "__sub", OpMul: "__mul", OpDiv: "__div", OpMod: "__mod",
	OpIDiv: "__idiv", OpPow: "__pow", OpBand: "__band", OpBor: "__bor", OpBxor: "__bxor", OpShl: "__shl", OpShr: "__shr",
	OpConcat: "__concat"}

var arithSym = map[byte]string{OpAdd: "+", OpSub: "-", OpMul: "*", OpDiv: "/", OpMod: "%", OpIDiv: "//", OpPow: "^",
	OpBand: "&", OpBor: "|", OpBxor: "~", OpShl: "<<", OpShr: ">>"}

// metaBinary tenta o metamétodo da operação; ok=false se nenhum dos lados tem.
func (vm *VM) metaBinary(ev string, a, b Value) (Value, bool, error) {
	h := vm.metamethod(a, ev)
	if h == nil {
		h = vm.metamethod(b, ev)
	}
	if h == nil {
		return nil, false, nil
	}
	r, err := vm.Call(h, []Value{a, b})
	return r, true, err
}

func shiftLeft(x, n int64) int64 {
	if n <= -64 || n >= 64 {
		return 0
	}
	if n >= 0 {
		return int64(uint64(x) << uint(n))
	}
	return int64(uint64(x) >> uint(-n))
}

func luaFloatMod(a, b float64) float64 {
	m := math.Mod(a, b)
	if m != 0 && (m > 0) != (b > 0) {
		m += b
	}
	return m
}

// luaArith faz as operações binárias com as regras do Lua.
func (vm *VM) luaArith(op byte, a, b Value) (Value, error) {
	if op == OpConcat {
		return vm.luaConcat(a, b)
	}
	x, xok := luaToNumber(a)
	y, yok := luaToNumber(b)
	if xok && yok {
		switch op {
		case OpBand, OpBor, OpBxor, OpShl, OpShr:
			i, iok := luaToInteger(x)
			j, jok := luaToInteger(y)
			if !iok || !jok {
				return nil, fmt.Errorf("número sem representação inteira")
			}
			switch op {
			case OpBand:
				return i & j, nil
			case OpBor:
				return i | j, nil
			case OpBxor:
				return i ^ j, nil
			case OpShl:
				return shiftLeft(i, j), nil
			default:
				return shiftLeft(i, -j), nil
			}
		}
		xi, xInt := x.(int64)
		yi, yInt := y.(int64)
		if xInt && yInt {
			switch op {
			case OpAdd:
				return xi + yi, nil
			case OpSub:
				return xi - yi, nil
			case OpMul:
				return xi * yi, nil
			case OpIDiv:
				if yi == 0 {
					return nil, fmt.Errorf("tentou fazer 'n//0'")
				}
				if yi == -1 {
					return -xi, nil
				}
				q := xi / yi
				if (xi%yi != 0) && ((xi < 0) != (yi < 0)) {
					q--
				}
				return q, nil
			case OpMod:
				if yi == 0 {
					return nil, fmt.Errorf("tentou fazer 'n%%%%0'")
				}
				if yi == -1 {
					return int64(0), nil
				}
				m := xi % yi
				if m != 0 && (m < 0) != (yi < 0) {
					m += yi
				}
				return m, nil
			}
		}
		f, _ := toFloat(x)
		g, _ := toFloat(y)
		switch op {
		case OpAdd:
			return f + g, nil
		case OpSub:
			return f - g, nil
		case OpMul:
			return f * g, nil
		case OpDiv:
			return f / g, nil
		case OpPow:
			if g == 2 {
				return f * f, nil
			}
			if g == 0.5 {
				return math.Sqrt(f), nil
			}
			return math.Pow(f, g), nil
		case OpIDiv:
			return math.Floor(f / g), nil
		case OpMod:
			if math.IsInf(g, 0) && !math.IsNaN(f) && !math.IsInf(f, 0) {
				if (f >= 0) == (g > 0) {
					return f, nil
				}
				return g, nil
			}
			return luaFloatMod(f, g), nil
		}
	}
	r, ok, err := vm.metaBinary(arithEvents[op], a, b)
	if ok {
		return single(r), err
	}
	switch op {
	case OpBand, OpBor, OpBxor, OpShl, OpShr:
		if xok && yok {
			return nil, fmt.Errorf("número sem representação inteira")
		}
		bad := a
		if _, isNum := a.(int64); isNum || xok {
			bad = b
		}
		return nil, fmt.Errorf("tentou fazer operação bit a bit com um valor %s", luaTypeName(bad))
	}
	bad := a
	if xok {
		bad = b
	}
	return nil, fmt.Errorf("tentou fazer aritmética ('%s') com um valor %s", arithSym[op], luaTypeName(bad))
}

func (vm *VM) luaConcat(a, b Value) (Value, error) {
	sa, aok := luaNumStr(a)
	sb, bok := luaNumStr(b)
	if aok && bok {
		return sa + sb, nil
	}
	r, ok, err := vm.metaBinary("__concat", a, b)
	if ok {
		return single(r), err
	}
	bad := a
	if aok {
		bad = b
	}
	return nil, fmt.Errorf("tentou concatenar um valor %s", luaTypeName(bad))
}

func (vm *VM) luaUnm(a Value) (Value, error) {
	if n, ok := luaToNumber(a); ok {
		switch x := n.(type) {
		case int64:
			return -x, nil
		case float64:
			return -x, nil
		}
	}
	if h := vm.metamethod(a, "__unm"); h != nil {
		r, err := vm.Call(h, []Value{a, a})
		return r, err
	}
	return nil, fmt.Errorf("tentou fazer aritmética ('-') com um valor %s", luaTypeName(a))
}

func (vm *VM) luaBnot(a Value) (Value, error) {
	if i, ok := luaToInteger(a); ok {
		return ^i, nil
	}
	if h := vm.metamethod(a, "__bnot"); h != nil {
		r, err := vm.Call(h, []Value{a, a})
		return r, err
	}
	if _, ok := luaToNumber(a); ok {
		return nil, fmt.Errorf("número sem representação inteira")
	}
	return nil, fmt.Errorf("tentou fazer operação bit a bit com um valor %s", luaTypeName(a))
}

func (vm *VM) luaLen(a Value) (Value, error) {
	switch x := a.(type) {
	case string:
		return int64(len(x)), nil
	case HTML:
		return int64(len(x)), nil
	}
	if h := vm.metamethod(a, "__len"); h != nil {
		r, err := vm.Call(h, []Value{a})
		return r, err
	}
	switch x := a.(type) {
	case *Table:
		return x.luaLen(), nil
	case *List:
		return int64(len(x.Items)), nil
	}
	return nil, fmt.Errorf("tentou medir o tamanho de um valor %s", luaTypeName(a))
}

// luaRawEq é a igualdade sem metamétodos.
func luaRawEq(a, b Value) bool {
	switch x := a.(type) {
	case int64:
		switch y := b.(type) {
		case int64:
			return x == y
		case float64:
			return float64(x) == y
		}
		return false
	case float64:
		switch y := b.(type) {
		case int64:
			return x == float64(y)
		case float64:
			return x == y
		}
		return false
	case HTML:
		if s, ok := b.(string); ok {
			return string(x) == s
		}
	case string:
		if h, ok := b.(HTML); ok {
			return x == string(h)
		}
	}
	return a == b
}

func (vm *VM) luaEq(a, b Value) (bool, error) {
	if luaRawEq(a, b) {
		return true, nil
	}
	ta, ok1 := a.(*Table)
	tb, ok2 := b.(*Table)
	if !ok1 || !ok2 {
		return false, nil
	}
	h := vm.metamethod(ta, "__eq")
	if h == nil {
		h = vm.metamethod(tb, "__eq")
	}
	if h == nil {
		return false, nil
	}
	r, err := vm.Call(h, []Value{a, b})
	return luaTruthy(r), err
}

func (vm *VM) luaLess(a, b Value, orEq bool) (bool, error) {
	if isNum(a) && isNum(b) {
		ai, aok := a.(int64)
		bi, bok := b.(int64)
		if aok && bok {
			if orEq {
				return ai <= bi, nil
			}
			return ai < bi, nil
		}
		x, _ := toFloat(a)
		y, _ := toFloat(b)
		if orEq {
			return x <= y, nil
		}
		return x < y, nil
	}
	sa, aok := a.(string)
	sb, bok := b.(string)
	if aok && bok {
		if orEq {
			return sa <= sb, nil
		}
		return sa < sb, nil
	}
	ev := "__lt"
	if orEq {
		ev = "__le"
	}
	r, ok, err := vm.metaBinary(ev, a, b)
	if ok {
		return luaTruthy(single(r)), err
	}
	ta, tb := luaTypeName(a), luaTypeName(b)
	if ta == tb {
		return false, fmt.Errorf("tentou comparar dois valores %s", ta)
	}
	return false, fmt.Errorf("tentou comparar %s com %s", ta, tb)
}

// luaOp executa as instruções acrescentadas para o Lua.
func (vm *VM) luaOp(op byte, fr *frame, code []byte, reload *bool) error {
	u16 := func() int {
		v := int(code[fr.ip])<<8 | int(code[fr.ip+1])
		fr.ip += 2
		return v
	}
	switch op {
	case OpPow, OpConcat, OpBand, OpBor, OpBxor, OpShl, OpShr:
		b := vm.pop()
		a := vm.pop()
		v, err := vm.luaArith(op, a, b)
		if err != nil {
			return err
		}
		vm.push(v)
	case OpLen, OpBnot:
		a := vm.pop()
		var v Value
		var err error
		if op == OpLen {
			v, err = vm.luaLen(a)
		} else {
			v, err = vm.luaBnot(a)
		}
		if err != nil {
			return err
		}
		vm.push(v)
	case OpSelf:
		name := fr.cl.Proto.Consts[u16()].(string)
		obj := vm.pop()
		var m Value
		var err error
		if fr.cl.Proto.Lua {
			m, err = vm.luaIndex(obj, name)
		} else {
			m, err = vm.nepSelf(obj, name, fr.cl.Proto.Module)
		}
		if err != nil {
			return err
		}
		vm.push(m)
		vm.push(obj)
	case OpCallM:
		argc := int(code[fr.ip])
		fr.ip++
		argc = vm.expandTop(argc)
		if argc < 0 {
			return fmt.Errorf("pilha cheia")
		}
		*reload = true
		return vm.callValue(vm.stack[vm.sp-argc-1], argc)
	case OpTailCall:
		argc := int(code[fr.ip])
		multi := code[fr.ip+1] == 1
		fr.ip += 2
		if multi {
			argc = vm.expandTop(argc)
			if argc < 0 {
				return fmt.Errorf("pilha cheia")
			}
		}
		callee := vm.stack[vm.sp-argc-1]
		*reload = true
		if cl, ok := callee.(*Closure); ok {
			// chamada final: reaproveita o lugar desta função na pilha
			vm.closeUpvals(fr.base)
			start := vm.sp - argc - 1
			copy(vm.stack[fr.base:], vm.stack[start:vm.sp])
			newSP := fr.base + argc + 1
			for i := newSP; i < vm.sp; i++ {
				vm.stack[i] = nil
			}
			vm.sp = newSP
			vm.frames = vm.frames[:len(vm.frames)-1]
			return vm.callValue(cl, argc)
		}
		return vm.callValue(callee, argc)
	case OpFirst:
		if t, ok := vm.stack[vm.sp-1].(*Tuple); ok {
			vm.stack[vm.sp-1] = t.first()
		}
	case OpSpread:
		n := int(code[fr.ip])
		fr.ip++
		vals := Values(vm.pop())
		for i := 0; i < n; i++ {
			if i < len(vals) {
				vm.push(vals[i])
			} else {
				vm.push(nil)
			}
		}
	case OpPack:
		n := int(code[fr.ip])
		fr.ip++
		var vals []Value
		if n > 0 {
			vals = append(vals, vm.stack[vm.sp-n:vm.sp-1]...)
			vals = append(vals, Values(vm.stack[vm.sp-1])...)
			for i := 0; i < n; i++ {
				vm.pop()
			}
		}
		if len(vals) == 1 {
			vm.push(vals[0])
		} else {
			vm.push(&Tuple{V: vals})
		}
	case OpNewTable:
		vm.push(NewTable())
	case OpTSet:
		v := vm.pop()
		k := vm.pop()
		return vm.peek(0).(*Table).luaSet(k, v)
	case OpTAppend:
		i := u16()
		v := vm.pop()
		return vm.peek(0).(*Table).luaSet(int64(i), v)
	case OpTAppendM:
		i := u16()
		t := vm.peek(1).(*Table)
		for j, x := range Values(vm.pop()) {
			if err := t.luaSet(int64(i+j), x); err != nil {
				return err
			}
		}
	case OpForPrep:
		slot := fr.base + int(code[fr.ip])
		fr.ip++
		off := u16()
		run, err := forPrep(vm.stack[slot : slot+3])
		if err != nil {
			return err
		}
		if !run {
			fr.ip += off
		}
	case OpForLoop:
		slot := fr.base + int(code[fr.ip])
		fr.ip++
		off := u16()
		s := vm.stack[slot : slot+3]
		if i, ok := s[0].(int64); ok {
			count := uint64(s[1].(int64))
			if count > 0 {
				s[1] = int64(count - 1)
				s[0] = i + s[2].(int64)
				fr.ip -= off
			}
			break
		}
		idx := s[0].(float64) + s[2].(float64)
		lim, step := s[1].(float64), s[2].(float64)
		if (step > 0 && idx <= lim) || (step < 0 && idx >= lim) {
			s[0] = idx
			fr.ip -= off
		}
	case OpForGen:
		slot := fr.base + int(code[fr.ip])
		nvars := int(code[fr.ip+1])
		fr.ip += 2
		off := u16()
		f, st, ctl := vm.stack[slot], vm.stack[slot+1], vm.stack[slot+2]
		r, err := vm.CallMulti(f, []Value{st, ctl})
		if err != nil {
			return err
		}
		vals := Values(r)
		var first Value
		if len(vals) > 0 {
			first = vals[0]
		}
		if first == nil {
			fr.ip += off
			break
		}
		vm.stack[slot+2] = first
		for i := 0; i < nvars; i++ {
			if i < len(vals) {
				vm.push(vals[i])
			} else {
				vm.push(nil)
			}
		}
	case OpSetTop:
		target := fr.base + int(code[fr.ip])
		fr.ip++
		vm.closeUpvals(target)
		for i := target; i < vm.sp; i++ {
			vm.stack[i] = nil
		}
		vm.sp = target
	default:
		return fmt.Errorf("instrução desconhecida %d", op)
	}
	return nil
}

// expandTop abre o último argumento se ele for vários valores; devolve o novo argc.
func (vm *VM) expandTop(argc int) int {
	if argc == 0 {
		return 0
	}
	t, ok := vm.stack[vm.sp-1].(*Tuple)
	if !ok {
		return argc
	}
	if vm.sp+len(t.V)+256 >= len(vm.stack) {
		return -1
	}
	vm.pop()
	for _, v := range t.V {
		vm.push(v)
	}
	return argc - 1 + len(t.V)
}

// forPrep prepara o for numérico (início, limite, passo); diz se o laço roda.
func forPrep(s []Value) (bool, error) {
	init, limit, step := s[0], s[1], s[2]
	if !isNum(init) {
		return false, fmt.Errorf("o valor inicial do 'for' precisa ser um número")
	}
	if !isNum(limit) {
		return false, fmt.Errorf("o limite do 'for' precisa ser um número")
	}
	if !isNum(step) {
		return false, fmt.Errorf("o passo do 'for' precisa ser um número")
	}
	i, iok := init.(int64)
	st, sok := step.(int64)
	if iok && sok {
		if st == 0 {
			return false, fmt.Errorf("o passo do 'for' é zero")
		}
		var lim int64
		switch l := limit.(type) {
		case int64:
			lim = l
		case float64:
			if math.IsNaN(l) {
				return false, nil
			}
			if st > 0 {
				l = math.Floor(l)
				if l >= 9.2233720368547758e18 {
					lim = math.MaxInt64
				} else if l < -9.2233720368547758e18 {
					return false, nil
				} else {
					lim = int64(l)
				}
			} else {
				l = math.Ceil(l)
				if l < -9.2233720368547758e18 {
					lim = math.MinInt64
				} else if l >= 9.2233720368547758e18 {
					return false, nil
				} else {
					lim = int64(l)
				}
			}
		}
		if (st > 0 && i > lim) || (st < 0 && i < lim) {
			return false, nil
		}
		var count uint64
		if st > 0 {
			count = (uint64(lim) - uint64(i)) / uint64(st)
		} else {
			count = (uint64(i) - uint64(lim)) / (uint64(-(st + 1)) + 1)
		}
		s[1] = int64(count)
		return true, nil
	}
	f, _ := toFloat(init)
	l, _ := toFloat(limit)
	g, _ := toFloat(step)
	if g == 0 {
		return false, fmt.Errorf("o passo do 'for' é zero")
	}
	s[0], s[1], s[2] = f, l, g
	if (g > 0 && f > l) || (g < 0 && f < l) {
		return false, nil
	}
	return true, nil
}

// nepSelf resolve obj:metodo() no Nepenthes: campo de tabela/módulo ou função
// com esse nome (do módulo ou da biblioteca) que recebe obj como primeiro argumento.
func (vm *VM) nepSelf(obj Value, name string, mod *Module) (Value, error) {
	switch o := obj.(type) {
	case *Table:
		return vm.luaIndex(o, name)
	case *Module:
		return getField(o, name)
	}
	if g, ok := mod.Globals[name]; ok && isFn(g) {
		return g, nil
	}
	if b, ok := vm.Builtins[name]; ok {
		return b, nil
	}
	return nil, fmt.Errorf("%s não tem o método '%s'", TypeName(obj), name)
}

// importAny carrega um módulo para o import do Nepenthes: arquivos .lua
// devolvem o valor que o arquivo retorna (normalmente uma tabela).
func (vm *VM) importAny(name string, isPath bool, fromFile string) (Value, error) {
	if !strings.HasSuffix(name, ".lua") {
		return vm.importModule(name, isPath, fromFile)
	}
	dir := "."
	if fromFile != "" && fromFile != "<repl>" && !strings.HasPrefix(fromFile, "[") {
		dir = filepath.Dir(fromFile)
	}
	p := name
	if !filepath.IsAbs(p) {
		p = filepath.Join(dir, p)
	}
	abs, _ := filepath.Abs(p)
	L := vm.lua()
	key := "@" + abs
	if v, _ := L.loaded.Get(key); v != nil {
		return v, nil
	}
	src, err := os.ReadFile(abs)
	if err != nil {
		return nil, fmt.Errorf("não consegui importar %s: arquivo não encontrado", name)
	}
	fn, err := vm.compileLua(string(src), abs, L.G)
	if err != nil {
		return nil, err
	}
	r, err := vm.Call(fn, []Value{baseName(abs), abs})
	if err != nil {
		return nil, err
	}
	if r == nil {
		r = L.G
	}
	_ = L.loaded.luaSet(key, r)
	return r, nil
}

// RunLuaFile executa um arquivo .lua como programa principal (arg[0] é o arquivo).
func (vm *VM) RunLuaFile(path string, args []string) error {
	src, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("não consegui abrir %s: %v", path, err)
	}
	L := vm.lua()
	argT := NewTable()
	_ = argT.luaSet(int64(0), path)
	vals := make([]Value, len(args))
	for i, a := range args {
		_ = argT.luaSet(int64(i+1), a)
		vals[i] = a
	}
	L.G.SetStr("arg", argT)
	fn, err := vm.compileLua(string(src), path, L.G)
	if err != nil {
		return err
	}
	_, err = vm.CallMulti(fn, vals)
	return err
}

// RunLua compila e executa um trecho de código Lua e devolve o que ele retorna.
func (vm *VM) RunLua(src, name string) (Value, error) {
	fn, err := vm.compileLua(src, name, vm.lua().G)
	if err != nil {
		return nil, err
	}
	return vm.CallMulti(fn, nil)
}
