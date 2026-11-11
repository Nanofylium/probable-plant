package lang

import (
	"bufio"
	"fmt"
	"io"
	"math"
	"math/rand"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"
)

// Biblioteca padrão do Lua 5.4 escrita em Go: funções básicas, string, table,
// math, os, io, utf8, coroutine e package/require.

type luaState struct {
	G          *Table // _G / _ENV
	mod        *Module
	stringLib  *Table
	stringMeta *Table
	loaded     *Table
	main       *Coroutine
	stdin      *bufio.Reader
	rng        *rand.Rand
	start      time.Time
}

func (vm *VM) lua() *luaState {
	if vm.L == nil {
		vm.L = newLuaState(vm)
	}
	return vm.L
}

// LuaGlobals devolve a tabela de variáveis globais do Lua (_G).
func (vm *VM) LuaGlobals() *Table { return vm.lua().G }

// fsAllowed diz se o código pode mexer em arquivos (no servidor web não pode).
func (vm *VM) fsAllowed() bool {
	_, ok := vm.stdmods["fs"]
	return ok
}

func luaArgErr(fn string, i int, want string, got Value) error {
	g := luaTypeName(got)
	if got == nil {
		g = "nada"
	}
	return fmt.Errorf("argumento #%d inválido para '%s' (esperava %s, recebeu %s)", i+1, fn, want, g)
}

func luaCheckStr(fn string, a []Value, i int) (string, error) {
	switch x := arg(a, i).(type) {
	case string:
		return x, nil
	case HTML:
		return string(x), nil
	case int64, float64:
		s, _ := luaNumStr(x)
		return s, nil
	}
	return "", luaArgErr(fn, i, "texto", arg(a, i))
}

func luaOptStr(fn string, a []Value, i int, def string) (string, error) {
	if arg(a, i) == nil {
		return def, nil
	}
	return luaCheckStr(fn, a, i)
}

func luaCheckInt(fn string, a []Value, i int) (int64, error) {
	v := arg(a, i)
	if n, ok := luaToInteger(v); ok {
		return n, nil
	}
	if _, ok := luaToNumber(v); ok {
		return 0, fmt.Errorf("argumento #%d inválido para '%s' (número sem representação inteira)", i+1, fn)
	}
	return 0, luaArgErr(fn, i, "número", v)
}

func luaOptInt(fn string, a []Value, i int, def int64) (int64, error) {
	if arg(a, i) == nil {
		return def, nil
	}
	return luaCheckInt(fn, a, i)
}

func luaCheckNum(fn string, a []Value, i int) (Value, error) {
	if n, ok := luaToNumber(arg(a, i)); ok {
		return n, nil
	}
	return nil, luaArgErr(fn, i, "número", arg(a, i))
}

func luaCheckFloat(fn string, a []Value, i int) (float64, error) {
	n, err := luaCheckNum(fn, a, i)
	if err != nil {
		return 0, err
	}
	f, _ := toFloat(n)
	return f, nil
}

func luaCheckTable(fn string, a []Value, i int) (*Table, error) {
	if t, ok := arg(a, i).(*Table); ok {
		return t, nil
	}
	return nil, luaArgErr(fn, i, "tabela", arg(a, i))
}

// luaErrValue converte um erro no valor que pcall entrega (texto com posição).
func (vm *VM) luaErrValue(err error) Value {
	switch e := err.(type) {
	case *thrown:
		return e.val
	case *RuntimeError:
		if s, ok := e.Value.(string); ok && e.Pos != "" {
			return e.Pos + ": " + s
		}
		return e.Value
	}
	return err.Error()
}

// where devolve "arquivo:linha" da função no nível dado (1 = quem chamou).
// Vazio quando nesse nível está uma função do Go (como pcall).
func (vm *VM) where(level int) string {
	i := len(vm.frames) - 1
	if level <= 0 || i < 0 {
		return ""
	}
	for l := 1; l < level; l++ {
		if vm.frames[i].goEntry || i == 0 {
			return ""
		}
		i--
	}
	f := vm.frames[i]
	ip := f.ip - 1
	if ip < 0 {
		ip = 0
	}
	line := 0
	if ip < len(f.cl.Proto.Lines) {
		line = f.cl.Proto.Lines[ip]
	}
	return fmt.Sprintf("%s:%d", filepath.Base(f.cl.Proto.File), line)
}

func newLuaState(vm *VM) *luaState {
	L := &luaState{G: NewTable(), loaded: NewTable(), start: time.Now(),
		rng: rand.New(rand.NewSource(time.Now().UnixNano()))}
	L.mod = NewModule("lua", "<lua>")
	L.main = &Coroutine{st: &coState{status: "running"}}
	G := L.G
	ipairsIter := ipairsIterFn()
	def := func(t *Table, name string, f nativeFn) { t.SetStr(name, nat(name, f)) }

	// ---------- básicas ----------
	def(G, "print", func(vm *VM, a []Value) (Value, error) {
		parts := make([]string, len(a))
		for i, v := range a {
			s, err := vm.luaToString(v)
			if err != nil {
				return nil, err
			}
			parts[i] = s
		}
		fmt.Fprintln(vm.Out, strings.Join(parts, "\t"))
		return Multi(), nil
	})
	def(G, "type", func(vm *VM, a []Value) (Value, error) {
		if len(a) == 0 {
			return nil, fmt.Errorf("argumento #1 inválido para 'type' (esperava um valor)")
		}
		return luaTypeName(a[0]), nil
	})
	def(G, "tostring", func(vm *VM, a []Value) (Value, error) { return vm.luaToString(arg(a, 0)) })
	def(G, "tonumber", func(vm *VM, a []Value) (Value, error) {
		v := arg(a, 0)
		if arg(a, 1) == nil {
			if isNum(v) {
				return v, nil
			}
			if s, ok := v.(string); ok {
				if n, ok := parseLuaNumber(strings.TrimSpace(s)); ok {
					return n, nil
				}
			}
			return nil, nil
		}
		base, err := luaCheckInt("tonumber", a, 1)
		if err != nil {
			return nil, err
		}
		if base < 2 || base > 36 {
			return nil, fmt.Errorf("argumento #2 inválido para 'tonumber' (base fora do intervalo)")
		}
		s, ok := v.(string)
		if !ok {
			return nil, luaArgErr("tonumber", 0, "texto", v)
		}
		s = strings.ToLower(strings.TrimSpace(s))
		neg := false
		if strings.HasPrefix(s, "-") {
			neg, s = true, s[1:]
		}
		if s == "" {
			return nil, nil
		}
		var n int64
		for _, c := range s {
			var d int64
			switch {
			case c >= '0' && c <= '9':
				d = int64(c - '0')
			case c >= 'a' && c <= 'z':
				d = int64(c-'a') + 10
			default:
				return nil, nil
			}
			if d >= base {
				return nil, nil
			}
			n = n*base + d
		}
		if neg {
			n = -n
		}
		return n, nil
	})
	def(G, "ipairs", func(vm *VM, a []Value) (Value, error) {
		if len(a) == 0 {
			return nil, luaArgErr("ipairs", 0, "tabela", nil)
		}
		return Multi(ipairsIter, a[0], int64(0)), nil
	})
	nextFn := nat("next", func(vm *VM, a []Value) (Value, error) {
		switch t := arg(a, 0).(type) {
		case *Table:
			k, v, err := t.luaNext(arg(a, 1))
			if err != nil {
				return nil, err
			}
			if k == nil {
				return nil, nil
			}
			return Multi(k, v), nil
		case *List:
			i := int64(0)
			if k := arg(a, 1); k != nil {
				i, _ = luaIntKey(k)
			}
			if i < int64(len(t.Items)) {
				return Multi(i+1, t.Items[i]), nil
			}
			return nil, nil
		}
		return nil, luaArgErr("next", 0, "tabela", arg(a, 0))
	})
	G.SetStr("next", nextFn)
	def(G, "pairs", func(vm *VM, a []Value) (Value, error) {
		v := arg(a, 0)
		if h := vm.metamethod(v, "__pairs"); h != nil {
			r, err := vm.CallMulti(h, []Value{v})
			if err != nil {
				return nil, err
			}
			vals := Values(r)
			for len(vals) < 3 {
				vals = append(vals, nil)
			}
			return Multi(vals[:3]...), nil
		}
		switch x := v.(type) {
		case *Table, *List:
			return Multi(nextFn, x, nil), nil
		case *Module:
			t := NewTable()
			for _, k := range x.order {
				t.SetStr(k, x.Globals[k])
			}
			return Multi(nextFn, t, nil), nil
		}
		return nil, luaArgErr("pairs", 0, "tabela", v)
	})
	def(G, "rawget", func(vm *VM, a []Value) (Value, error) {
		t, err := luaCheckTable("rawget", a, 0)
		if err != nil {
			return nil, err
		}
		v, _ := t.Get(arg(a, 1))
		return v, nil
	})
	def(G, "rawset", func(vm *VM, a []Value) (Value, error) {
		t, err := luaCheckTable("rawset", a, 0)
		if err != nil {
			return nil, err
		}
		return t, t.luaSet(arg(a, 1), arg(a, 2))
	})
	def(G, "rawequal", func(vm *VM, a []Value) (Value, error) { return luaRawEq(arg(a, 0), arg(a, 1)), nil })
	def(G, "rawlen", func(vm *VM, a []Value) (Value, error) {
		switch x := arg(a, 0).(type) {
		case *Table:
			return x.luaLen(), nil
		case string:
			return int64(len(x)), nil
		case *List:
			return int64(len(x.Items)), nil
		}
		return nil, fmt.Errorf("argumento #1 inválido para 'rawlen' (esperava tabela ou texto)")
	})
	def(G, "setmetatable", setMetatable)
	def(G, "getmetatable", getMetatable)
	def(G, "assert", func(vm *VM, a []Value) (Value, error) {
		if len(a) > 0 && luaTruthy(a[0]) {
			return Multi(a...), nil
		}
		if len(a) > 1 {
			return nil, Throw(a[1])
		}
		return nil, Throw("assertion failed!")
	})
	def(G, "error", func(vm *VM, a []Value) (Value, error) {
		v := arg(a, 0)
		level, _ := luaOptInt("error", a, 1, 1)
		if s, ok := v.(string); ok && level > 0 && !vm.fromGo {
			if w := vm.where(int(level)); w != "" {
				v = w + ": " + s
			}
		}
		return nil, Throw(v)
	})
	def(G, "pcall", func(vm *VM, a []Value) (Value, error) {
		if len(a) == 0 {
			return nil, luaArgErr("pcall", 0, "um valor", nil)
		}
		r, err := vm.CallMulti(a[0], a[1:])
		if err != nil {
			if isHalt(err) {
				return nil, err
			}
			return Multi(false, vm.luaErrValue(err)), nil
		}
		return Multi(append([]Value{true}, Values(r)...)...), nil
	})
	def(G, "xpcall", func(vm *VM, a []Value) (Value, error) {
		r, err := vm.CallMulti(arg(a, 0), argsFrom(a, 2))
		if err != nil {
			if isHalt(err) {
				return nil, err
			}
			h, herr := vm.CallMulti(arg(a, 1), []Value{vm.luaErrValue(err)})
			if herr != nil {
				return Multi(false, vm.luaErrValue(herr)), nil
			}
			return Multi(append([]Value{false}, Values(h)...)...), nil
		}
		return Multi(append([]Value{true}, Values(r)...)...), nil
	})
	def(G, "select", func(vm *VM, a []Value) (Value, error) {
		if s, ok := arg(a, 0).(string); ok && s == "#" {
			return int64(len(a) - 1), nil
		}
		n, err := luaCheckInt("select", a, 0)
		if err != nil {
			return nil, err
		}
		rest := argsFrom(a, 1)
		if n < 0 {
			n = int64(len(rest)) + n
			if n < 0 {
				return nil, fmt.Errorf("argumento #1 inválido para 'select' (índice fora do intervalo)")
			}
			return Multi(rest[n:]...), nil
		}
		if n == 0 {
			return nil, fmt.Errorf("argumento #1 inválido para 'select' (índice fora do intervalo)")
		}
		if n > int64(len(rest)) {
			return Multi(), nil
		}
		return Multi(rest[n-1:]...), nil
	})
	def(G, "load", func(vm *VM, a []Value) (Value, error) {
		var src string
		switch c := arg(a, 0).(type) {
		case string:
			src = c
		case *Closure, *Native:
			var b strings.Builder
			for {
				piece, err := vm.Call(c, nil)
				if err != nil {
					return Multi(nil, vm.luaErrValue(err)), nil
				}
				s, ok := piece.(string)
				if !ok || s == "" {
					break
				}
				b.WriteString(s)
			}
			src = b.String()
		default:
			return nil, luaArgErr("load", 0, "texto ou função", arg(a, 0))
		}
		name, _ := luaOptStr("load", a, 1, src)
		name = chunkName(name)
		env := vm.lua().G
		if len(a) > 3 {
			if t, ok := a[3].(*Table); ok {
				env = t
			}
		}
		fn, err := vm.compileLua(src, name, env)
		if err != nil {
			return Multi(nil, err.Error()), nil
		}
		return fn, nil
	})
	def(G, "loadfile", func(vm *VM, a []Value) (Value, error) {
		if !vm.fsAllowed() {
			return nil, fmt.Errorf("loadfile não está disponível aqui")
		}
		name, err := luaCheckStr("loadfile", a, 0)
		if err != nil {
			return nil, err
		}
		src, rerr := os.ReadFile(name)
		if rerr != nil {
			return Multi(nil, "não consegui abrir "+name), nil
		}
		fn, cerr := vm.compileLua(string(src), name, vm.lua().G)
		if cerr != nil {
			return Multi(nil, cerr.Error()), nil
		}
		return fn, nil
	})
	def(G, "dofile", func(vm *VM, a []Value) (Value, error) {
		if !vm.fsAllowed() {
			return nil, fmt.Errorf("dofile não está disponível aqui")
		}
		name, err := luaCheckStr("dofile", a, 0)
		if err != nil {
			return nil, err
		}
		src, rerr := os.ReadFile(name)
		if rerr != nil {
			return nil, fmt.Errorf("não consegui abrir %s", name)
		}
		fn, cerr := vm.compileLua(string(src), name, vm.lua().G)
		if cerr != nil {
			return nil, cerr
		}
		return vm.CallMulti(fn, nil)
	})
	def(G, "collectgarbage", func(vm *VM, a []Value) (Value, error) {
		opt, _ := luaOptStr("collectgarbage", a, 0, "collect")
		switch opt {
		case "count":
			var ms runtime.MemStats
			runtime.ReadMemStats(&ms)
			kb := float64(ms.HeapAlloc) / 1024
			return Multi(kb, int64(ms.HeapAlloc%1024)), nil
		case "collect":
			runtime.GC()
		case "isrunning":
			return true, nil
		}
		return int64(0), nil
	})
	G.SetStr("_G", G)
	G.SetStr("_VERSION", "Lua 5.4")

	// ---------- bibliotecas ----------
	L.stringLib = luaStringLib()
	L.stringMeta = NewTable()
	L.stringMeta.SetStr("__index", L.stringLib)
	G.SetStr("string", L.stringLib)
	G.SetStr("table", luaTableLib())
	G.SetStr("math", luaMathLib(L))
	G.SetStr("os", luaOSLib())
	G.SetStr("io", luaIOLib())
	G.SetStr("utf8", luaUTF8Lib())
	G.SetStr("coroutine", coroutineLib())

	pkg := NewTable()
	pkg.SetStr("loaded", L.loaded)
	pkg.SetStr("preload", NewTable())
	pkg.SetStr("path", "./?.lua;./?/init.lua")
	pkg.SetStr("config", "/\n;\n?\n!\n-\n")
	G.SetStr("package", pkg)
	def(G, "require", luaRequire)
	for _, n := range []string{"string", "table", "math", "os", "io", "utf8", "coroutine", "package"} {
		v, _ := G.Get(n)
		L.loaded.SetStr(n, v)
	}
	L.loaded.SetStr("_G", G)

	// ponte com o Nepenthes: nep.upper("x"), nep.import("json")
	nep := NewTable()
	for name, f := range vm.Builtins {
		if !strings.HasPrefix(name, "__") {
			nep.SetStr(name, f)
		}
	}
	def(nep, "import", func(vm *VM, a []Value) (Value, error) {
		name, err := luaCheckStr("nep.import", a, 0)
		if err != nil {
			return nil, err
		}
		file := ""
		if len(vm.frames) > 0 {
			file = vm.frames[len(vm.frames)-1].cl.Proto.File
		}
		isPath := strings.ContainsAny(name, "/\\") || strings.HasSuffix(name, ".nep")
		return vm.importAny(name, isPath, file)
	})
	G.SetStr("nep", nep)
	return L
}

func ipairsIterFn() *Native {
	return nat("ipairs_iter", func(vm *VM, a []Value) (Value, error) {
		i, _ := luaIntKey(arg(a, 1))
		i++
		var v Value
		var err error
		if t, ok := a[0].(*Table); ok && t.meta == nil {
			v = t.getInt(i)
		} else {
			v, err = vm.luaIndex(a[0], i)
			if err != nil {
				return nil, err
			}
		}
		if v == nil {
			return nil, nil
		}
		return Multi(i, v), nil
	})
}

func argsFrom(a []Value, i int) []Value {
	if i >= len(a) {
		return nil
	}
	return a[i:]
}

func chunkName(name string) string {
	if strings.HasPrefix(name, "=") || strings.HasPrefix(name, "@") {
		return name[1:]
	}
	line, _, _ := strings.Cut(name, "\n")
	if len(line) > 40 {
		line = line[:40] + "..."
	}
	return `[string "` + line + `"]`
}

func setMetatable(vm *VM, a []Value) (Value, error) {
	t, ok := arg(a, 0).(*Table)
	if !ok {
		return nil, luaArgErr("setmetatable", 0, "tabela", arg(a, 0))
	}
	var mt *Table
	switch m := arg(a, 1).(type) {
	case nil:
	case *Table:
		mt = m
	default:
		return nil, luaArgErr("setmetatable", 1, "nil ou tabela", m)
	}
	if t.meta != nil && t.meta.rawStr("__metatable") != nil {
		return nil, fmt.Errorf("não dá para trocar uma metatabela protegida")
	}
	t.meta = mt
	return t, nil
}

func getMetatable(vm *VM, a []Value) (Value, error) {
	mt := vm.metaOf(arg(a, 0))
	if mt == nil {
		return nil, nil
	}
	if p := mt.rawStr("__metatable"); p != nil {
		return p, nil
	}
	return mt, nil
}

// compileLua compila um trecho Lua como função (para load, dofile, require).
func (vm *VM) compileLua(src, name string, env *Table) (*Closure, error) {
	prog, err := ParseLua(src, name)
	if err != nil {
		return nil, err
	}
	proto, err := CompileLua(prog, vm.lua().mod, env, name)
	if err != nil {
		return nil, err
	}
	return &Closure{Proto: proto}, nil
}

func luaRequire(vm *VM, a []Value) (Value, error) {
	name, err := luaCheckStr("require", a, 0)
	if err != nil {
		return nil, err
	}
	L := vm.lua()
	if v, _ := L.loaded.Get(name); v != nil {
		return v, nil
	}
	pkg, _ := L.G.Get("package")
	if pt, ok := pkg.(*Table); ok {
		if pre, ok := pt.rawStr("preload").(*Table); ok {
			if loader := pre.rawStr(name); loader != nil {
				r, err := vm.Call(loader, []Value{name, ":preload:"})
				if err != nil {
					return nil, err
				}
				if r == nil {
					r = true
				}
				_ = L.loaded.luaSet(name, r)
				return r, nil
			}
		}
	}
	if !vm.fsAllowed() {
		return nil, fmt.Errorf("módulo '%s' não encontrado", name)
	}
	rel := strings.ReplaceAll(name, ".", string(filepath.Separator))
	var dirs []string
	if len(vm.frames) > 0 {
		if f := vm.frames[len(vm.frames)-1].cl.Proto.File; f != "" && !strings.HasPrefix(f, "[") {
			dirs = append(dirs, filepath.Dir(f))
		}
	}
	dirs = append(dirs, ".")
	var tried []string
	for _, d := range dirs {
		for _, cand := range []string{rel + ".lua", filepath.Join(rel, "init.lua")} {
			p := filepath.Join(d, cand)
			src, rerr := os.ReadFile(p)
			if rerr != nil {
				tried = append(tried, "\n\tsem o arquivo '"+p+"'")
				continue
			}
			fn, cerr := vm.compileLua(string(src), p, L.G)
			if cerr != nil {
				return nil, cerr
			}
			r, err := vm.Call(fn, []Value{name, p})
			if err != nil {
				return nil, err
			}
			if r == nil {
				if v, _ := L.loaded.Get(name); v != nil {
					return v, nil
				}
				r = true
			}
			_ = L.loaded.luaSet(name, r)
			return Multi(r, p), nil
		}
	}
	return nil, fmt.Errorf("módulo '%s' não encontrado:%s", name, strings.Join(tried, ""))
}

// ---------- string ----------

func luaStringLib() *Table {
	t := NewTable()
	def := func(name string, f nativeFn) { t.SetStr(name, nat("string."+name, f)) }
	def("len", func(vm *VM, a []Value) (Value, error) {
		s, err := luaCheckStr("len", a, 0)
		return int64(len(s)), err
	})
	def("sub", func(vm *VM, a []Value) (Value, error) {
		s, err := luaCheckStr("sub", a, 0)
		if err != nil {
			return nil, err
		}
		i, err := luaOptInt("sub", a, 1, 1)
		if err != nil {
			return nil, err
		}
		j, err := luaOptInt("sub", a, 2, -1)
		if err != nil {
			return nil, err
		}
		b, e := luaRange(i, j, len(s))
		if b > e {
			return "", nil
		}
		return s[b-1 : e], nil
	})
	def("upper", func(vm *VM, a []Value) (Value, error) {
		s, err := luaCheckStr("upper", a, 0)
		return strings.ToUpper(s), err
	})
	def("lower", func(vm *VM, a []Value) (Value, error) {
		s, err := luaCheckStr("lower", a, 0)
		return strings.ToLower(s), err
	})
	def("rep", func(vm *VM, a []Value) (Value, error) {
		s, err := luaCheckStr("rep", a, 0)
		if err != nil {
			return nil, err
		}
		n, err := luaCheckInt("rep", a, 1)
		if err != nil {
			return nil, err
		}
		sep, _ := luaOptStr("rep", a, 2, "")
		if n <= 0 {
			return "", nil
		}
		if (int64(len(s))+int64(len(sep)))*n > 1<<28 {
			return nil, fmt.Errorf("texto resultante grande demais")
		}
		if sep == "" {
			return strings.Repeat(s, int(n)), nil
		}
		return strings.Repeat(s+sep, int(n)-1) + s, nil
	})
	def("reverse", func(vm *VM, a []Value) (Value, error) {
		s, err := luaCheckStr("reverse", a, 0)
		if err != nil {
			return nil, err
		}
		b := []byte(s)
		for i, j := 0, len(b)-1; i < j; i, j = i+1, j-1 {
			b[i], b[j] = b[j], b[i]
		}
		return string(b), nil
	})
	def("byte", func(vm *VM, a []Value) (Value, error) {
		s, err := luaCheckStr("byte", a, 0)
		if err != nil {
			return nil, err
		}
		i, err := luaOptInt("byte", a, 1, 1)
		if err != nil {
			return nil, err
		}
		j, err := luaOptInt("byte", a, 2, i)
		if err != nil {
			return nil, err
		}
		b, e := luaRange(i, j, len(s))
		var out []Value
		for k := b; k <= e; k++ {
			out = append(out, int64(s[k-1]))
		}
		return Multi(out...), nil
	})
	def("char", func(vm *VM, a []Value) (Value, error) {
		b := make([]byte, len(a))
		for i := range a {
			n, err := luaCheckInt("char", a, i)
			if err != nil {
				return nil, err
			}
			if n < 0 || n > 255 {
				return nil, fmt.Errorf("argumento #%d inválido para 'char' (valor fora do intervalo)", i+1)
			}
			b[i] = byte(n)
		}
		return string(b), nil
	})
	def("format", func(vm *VM, a []Value) (Value, error) {
		f, err := luaCheckStr("format", a, 0)
		if err != nil {
			return nil, err
		}
		return vm.luaFormat(f, a[1:])
	})
	def("find", func(vm *VM, a []Value) (Value, error) { return vm.strFind(a, true) })
	def("match", func(vm *VM, a []Value) (Value, error) { return vm.strFind(a, false) })
	def("gmatch", func(vm *VM, a []Value) (Value, error) {
		s, err := luaCheckStr("gmatch", a, 0)
		if err != nil {
			return nil, err
		}
		p, err := luaCheckStr("gmatch", a, 1)
		if err != nil {
			return nil, err
		}
		init, _ := luaOptInt("gmatch", a, 2, 1)
		pos := strStart(init, len(s))
		lastmatch := -1
		return nat("gmatch_iter", func(vm *VM, _ []Value) (Value, error) {
			for pos <= len(s) {
				ms := &patState{src: s, pat: p}
				e := ms.match(pos, 0)
				if ms.err != nil {
					return nil, ms.err
				}
				if e != -1 && e != lastmatch {
					start := pos
					pos, lastmatch = e, e
					caps := ms.captures(start, e, true)
					return Multi(caps...), ms.err
				}
				pos++
			}
			return nil, nil
		}), nil
	})
	def("gsub", func(vm *VM, a []Value) (Value, error) {
		s, err := luaCheckStr("gsub", a, 0)
		if err != nil {
			return nil, err
		}
		p, err := luaCheckStr("gsub", a, 1)
		if err != nil {
			return nil, err
		}
		n, err := luaOptInt("gsub", a, 3, -1)
		if err != nil {
			return nil, err
		}
		if arg(a, 3) == nil {
			n = -1
		}
		r, cnt, err := vm.patGsub(s, p, arg(a, 2), n)
		if err != nil {
			return nil, err
		}
		return Multi(r, cnt), nil
	})
	return t
}

// luaRange converte i, j (estilo Lua) para posições 1..n.
func luaRange(i, j int64, n int) (int, int) {
	ln := int64(n)
	if i < 0 {
		i = max(ln+i+1, 1)
	} else if i == 0 {
		i = 1
	}
	if j < 0 {
		j = ln + j + 1
	} else if j > ln {
		j = ln
	}
	if i > j {
		return 1, 0
	}
	return int(i), int(j)
}

func (vm *VM) strFind(a []Value, find bool) (Value, error) {
	fn := "match"
	if find {
		fn = "find"
	}
	s, err := luaCheckStr(fn, a, 0)
	if err != nil {
		return nil, err
	}
	p, err := luaCheckStr(fn, a, 1)
	if err != nil {
		return nil, err
	}
	init, err := luaOptInt(fn, a, 2, 1)
	if err != nil {
		return nil, err
	}
	start := strStart(init, len(s))
	if start > len(s) {
		return nil, nil
	}
	if find && (luaTruthy(arg(a, 3)) || !hasSpecials(p)) {
		k := strings.Index(s[start:], p)
		if k < 0 {
			return nil, nil
		}
		return Multi(int64(start+k+1), int64(start+k+len(p))), nil
	}
	b, e, ms, err := patFind(s, p, start)
	if err != nil {
		return nil, err
	}
	if b < 0 {
		return nil, nil
	}
	if find {
		caps := ms.captures(b, e, false)
		return Multi(append([]Value{int64(b + 1), int64(e)}, caps...)...), ms.err
	}
	return Multi(ms.captures(b, e, true)...), ms.err
}

// luaFormat implementa string.format.
func (vm *VM) luaFormat(f string, args []Value) (Value, error) {
	var b strings.Builder
	ai := 0
	next := func() (Value, error) {
		if ai >= len(args) {
			return nil, fmt.Errorf("argumento #%d inválido para 'format' (sem valor)", ai+2)
		}
		v := args[ai]
		ai++
		return v, nil
	}
	for i := 0; i < len(f); i++ {
		c := f[i]
		if c != '%' {
			b.WriteByte(c)
			continue
		}
		i++
		if i >= len(f) {
			return nil, fmt.Errorf("formato inválido (termina com '%%')")
		}
		if f[i] == '%' {
			b.WriteByte('%')
			continue
		}
		j := i
		for j < len(f) && strings.IndexByte("-+ #0", f[j]) >= 0 {
			j++
		}
		for j < len(f) && isDigit(f[j]) {
			j++
		}
		if j < len(f) && f[j] == '.' {
			j++
			for j < len(f) && isDigit(f[j]) {
				j++
			}
		}
		if j >= len(f) {
			return nil, fmt.Errorf("formato inválido")
		}
		spec := f[i:j]
		conv := f[j]
		i = j
		v, err := next()
		if err != nil {
			return nil, err
		}
		switch conv {
		case 'd', 'i':
			n, ok := luaToInteger(v)
			if !ok {
				return nil, fmt.Errorf("argumento #%d inválido para 'format' (número sem representação inteira)", ai+1)
			}
			b.WriteString(fmt.Sprintf("%"+spec+"d", n))
		case 'u':
			n, ok := luaToInteger(v)
			if !ok {
				return nil, fmt.Errorf("argumento #%d inválido para 'format' (número sem representação inteira)", ai+1)
			}
			b.WriteString(fmt.Sprintf("%"+spec+"d", uint64(n)))
		case 'c':
			n, ok := luaToInteger(v)
			if !ok {
				return nil, luaArgErr("format", ai, "número", v)
			}
			b.WriteByte(byte(n))
		case 'x', 'X', 'o':
			n, ok := luaToInteger(v)
			if !ok {
				return nil, fmt.Errorf("argumento #%d inválido para 'format' (número sem representação inteira)", ai+1)
			}
			b.WriteString(fmt.Sprintf("%"+spec+string(conv), uint64(n)))
		case 'e', 'E', 'f', 'F', 'g', 'G':
			n, ok := luaToNumber(v)
			if !ok {
				return nil, luaArgErr("format", ai, "número", v)
			}
			x, _ := toFloat(n)
			sp := spec
			if !strings.Contains(sp, ".") {
				sp += ".6"
			}
			if conv == 'F' {
				conv = 'f'
			}
			if math.IsInf(x, 0) || math.IsNaN(x) {
				s := luaFmtFloat(x)
				if conv >= 'A' && conv <= 'Z' {
					s = strings.ToUpper(s)
				}
				b.WriteString(fmt.Sprintf("%"+strings.Split(spec, ".")[0]+"s", s))
				continue
			}
			b.WriteString(fixExp(fmt.Sprintf("%"+sp+string(conv), x)))
		case 'a', 'A':
			n, ok := luaToNumber(v)
			if !ok {
				return nil, luaArgErr("format", ai, "número", v)
			}
			x, _ := toFloat(n)
			s := hexFloat(x)
			if conv == 'A' {
				s = strings.ToUpper(s)
			}
			b.WriteString(fmt.Sprintf("%"+strings.TrimLeft(spec, "0")+"s", s))
		case 's':
			s, err := vm.luaToString(v)
			if err != nil {
				return nil, err
			}
			b.WriteString(fmt.Sprintf("%"+spec+"s", s))
		case 'q':
			b.WriteString(luaQuote(v))
		default:
			return nil, fmt.Errorf("conversão inválida '%%%c' no formato", conv)
		}
	}
	return b.String(), nil
}

// fixExp faz o expoente ter pelo menos 2 dígitos, como no C (o Go já faz isso).
func fixExp(s string) string { return s }

func hexFloat(x float64) string {
	if math.IsInf(x, 0) || math.IsNaN(x) {
		return luaFmtFloat(x)
	}
	s := strconv.FormatFloat(x, 'x', -1, 64)
	// Go: 0x1.8p+01 ; C: 0x1.8p+1
	if k := strings.IndexAny(s, "p"); k >= 0 {
		exp := s[k+1:]
		sign := exp[:1]
		digits := strings.TrimLeft(exp[1:], "0")
		if digits == "" {
			digits = "0"
		}
		s = s[:k+1] + sign + digits
	}
	return s
}

func luaQuote(v Value) string {
	switch x := v.(type) {
	case string:
		var b strings.Builder
		b.WriteByte('"')
		for i := 0; i < len(x); i++ {
			c := x[i]
			switch {
			case c == '"' || c == '\\':
				b.WriteByte('\\')
				b.WriteByte(c)
			case c == '\n':
				b.WriteString("\\\n")
			case c == '\r':
				b.WriteString("\\r")
			case c == 0:
				if i+1 < len(x) && isDigit(x[i+1]) {
					b.WriteString("\\000")
				} else {
					b.WriteString("\\0")
				}
			case c < 32 || c == 127:
				if i+1 < len(x) && isDigit(x[i+1]) {
					fmt.Fprintf(&b, "\\%03d", c)
				} else {
					fmt.Fprintf(&b, "\\%d", c)
				}
			default:
				b.WriteByte(c)
			}
		}
		b.WriteByte('"')
		return b.String()
	case int64:
		if x == math.MinInt64 {
			return "0x8000000000000000"
		}
		return strconv.FormatInt(x, 10)
	case float64:
		switch {
		case math.IsInf(x, 1):
			return "1e9999"
		case math.IsInf(x, -1):
			return "-1e9999"
		case math.IsNaN(x):
			return "(0/0)"
		}
		if x == math.Floor(x) {
			return fmt.Sprintf("%s", hexFloat(x))
		}
		return hexFloat(x)
	}
	s, _ := luaNumStr(v)
	return s
}

// ---------- table ----------

func tget(vm *VM, t Value, i int64) (Value, error) {
	if tb, ok := t.(*Table); ok && tb.meta == nil {
		return tb.getInt(i), nil
	}
	return vm.luaIndex(t, i)
}

func tset(vm *VM, t Value, i int64, v Value) error {
	if tb, ok := t.(*Table); ok && tb.meta == nil {
		return tb.luaSet(i, v)
	}
	return vm.luaSetIndex(t, i, v, true)
}

func tlen(vm *VM, t Value) (int64, error) {
	n, err := vm.luaLen(t)
	if err != nil {
		return 0, err
	}
	i, ok := luaToInteger(n)
	if !ok {
		return 0, fmt.Errorf("o tamanho do objeto não é um inteiro")
	}
	return i, nil
}

func luaCheckList(fn string, a []Value, i int) (Value, error) {
	switch arg(a, i).(type) {
	case *Table, *List:
		return a[i], nil
	}
	return nil, luaArgErr(fn, i, "tabela", arg(a, i))
}

func luaTableLib() *Table {
	t := NewTable()
	def := func(name string, f nativeFn) { t.SetStr(name, nat("table."+name, f)) }
	def("insert", func(vm *VM, a []Value) (Value, error) {
		tb, err := luaCheckList("insert", a, 0)
		if err != nil {
			return nil, err
		}
		n, err := tlen(vm, tb)
		if err != nil {
			return nil, err
		}
		switch len(a) {
		case 2:
			if l, ok := tb.(*List); ok {
				l.Items = append(l.Items, a[1])
				return Multi(), nil
			}
			return Multi(), tset(vm, tb, n+1, a[1])
		case 3:
			pos, err := luaCheckInt("insert", a, 1)
			if err != nil {
				return nil, err
			}
			if pos < 1 || pos > n+1 {
				return nil, fmt.Errorf("argumento #2 inválido para 'insert' (posição fora dos limites)")
			}
			for i := n; i >= pos; i-- {
				v, err := tget(vm, tb, i)
				if err != nil {
					return nil, err
				}
				if err := tset(vm, tb, i+1, v); err != nil {
					return nil, err
				}
			}
			return Multi(), tset(vm, tb, pos, a[2])
		}
		return nil, fmt.Errorf("número errado de argumentos para 'insert'")
	})
	def("remove", func(vm *VM, a []Value) (Value, error) {
		tb, err := luaCheckList("remove", a, 0)
		if err != nil {
			return nil, err
		}
		n, err := tlen(vm, tb)
		if err != nil {
			return nil, err
		}
		pos, err := luaOptInt("remove", a, 1, n)
		if err != nil {
			return nil, err
		}
		if len(a) > 1 && n+1 != pos && (pos < 1 || pos > n+1) {
			if !(n == 0 && pos == 0) {
				return nil, fmt.Errorf("argumento #2 inválido para 'remove' (posição fora dos limites)")
			}
		}
		if l, ok := tb.(*List); ok {
			if pos < 1 || pos > int64(len(l.Items)) {
				return nil, nil
			}
			v := l.Items[pos-1]
			l.Items = append(l.Items[:pos-1], l.Items[pos:]...)
			return v, nil
		}
		v, err := tget(vm, tb, pos)
		if err != nil {
			return nil, err
		}
		for ; pos < n; pos++ {
			nx, err := tget(vm, tb, pos+1)
			if err != nil {
				return nil, err
			}
			if err := tset(vm, tb, pos, nx); err != nil {
				return nil, err
			}
		}
		if pos <= n || len(a) > 1 {
			if err := tset(vm, tb, pos, nil); err != nil {
				return nil, err
			}
		}
		return v, nil
	})
	def("concat", func(vm *VM, a []Value) (Value, error) {
		tb, err := luaCheckList("concat", a, 0)
		if err != nil {
			return nil, err
		}
		sep, err := luaOptStr("concat", a, 1, "")
		if err != nil {
			return nil, err
		}
		i, err := luaOptInt("concat", a, 2, 1)
		if err != nil {
			return nil, err
		}
		var j int64
		if arg(a, 3) == nil {
			if j, err = tlen(vm, tb); err != nil {
				return nil, err
			}
		} else if j, err = luaCheckInt("concat", a, 3); err != nil {
			return nil, err
		}
		var b strings.Builder
		for k := i; k <= j; k++ {
			v, err := tget(vm, tb, k)
			if err != nil {
				return nil, err
			}
			s, ok := luaNumStr(v)
			if !ok {
				return nil, fmt.Errorf("valor inválido (na posição %d) para 'concat': %s", k, luaTypeName(v))
			}
			b.WriteString(s)
			if k < j {
				b.WriteString(sep)
			}
		}
		return b.String(), nil
	})
	def("unpack", func(vm *VM, a []Value) (Value, error) {
		i, err := luaOptInt("unpack", a, 1, 1)
		if err != nil {
			return nil, err
		}
		var j int64
		if arg(a, 2) == nil {
			if j, err = tlen(vm, arg(a, 0)); err != nil {
				return nil, err
			}
		} else if j, err = luaCheckInt("unpack", a, 2); err != nil {
			return nil, err
		}
		if i > j {
			return Multi(), nil
		}
		if j-i >= 1<<20 {
			return nil, fmt.Errorf("resultados demais para desempacotar")
		}
		out := make([]Value, 0, j-i+1)
		for k := i; k <= j; k++ {
			v, err := tget(vm, arg(a, 0), k)
			if err != nil {
				return nil, err
			}
			out = append(out, v)
		}
		return Multi(out...), nil
	})
	def("pack", func(vm *VM, a []Value) (Value, error) {
		t := NewTable()
		for i, v := range a {
			_ = t.luaSet(int64(i+1), v)
		}
		t.SetStr("n", int64(len(a)))
		return t, nil
	})
	def("move", func(vm *VM, a []Value) (Value, error) {
		a1, err := luaCheckList("move", a, 0)
		if err != nil {
			return nil, err
		}
		f, err := luaCheckInt("move", a, 1)
		if err != nil {
			return nil, err
		}
		e, err := luaCheckInt("move", a, 2)
		if err != nil {
			return nil, err
		}
		tpos, err := luaCheckInt("move", a, 3)
		if err != nil {
			return nil, err
		}
		a2 := a1
		if arg(a, 4) != nil {
			if a2, err = luaCheckList("move", a, 4); err != nil {
				return nil, err
			}
		}
		if e >= f {
			if tpos > e || tpos <= f || a1 != a2 {
				for i := int64(0); i <= e-f; i++ {
					v, err := tget(vm, a1, f+i)
					if err != nil {
						return nil, err
					}
					if err := tset(vm, a2, tpos+i, v); err != nil {
						return nil, err
					}
				}
			} else {
				for i := e - f; i >= 0; i-- {
					v, err := tget(vm, a1, f+i)
					if err != nil {
						return nil, err
					}
					if err := tset(vm, a2, tpos+i, v); err != nil {
						return nil, err
					}
				}
			}
		}
		return a2, nil
	})
	def("sort", func(vm *VM, a []Value) (Value, error) {
		tb, err := luaCheckList("sort", a, 0)
		if err != nil {
			return nil, err
		}
		n, err := tlen(vm, tb)
		if err != nil {
			return nil, err
		}
		items := make([]Value, n)
		for i := int64(1); i <= n; i++ {
			if items[i-1], err = tget(vm, tb, i); err != nil {
				return nil, err
			}
		}
		comp := arg(a, 1)
		var serr error
		sort.Slice(items, func(i, j int) bool {
			if serr != nil {
				return false
			}
			if comp != nil {
				r, err := vm.Call(comp, []Value{items[i], items[j]})
				if err != nil {
					serr = err
					return false
				}
				return luaTruthy(r)
			}
			r, err := vm.luaLess(items[i], items[j], false)
			if err != nil {
				serr = err
			}
			return r
		})
		if serr != nil {
			return nil, serr
		}
		for i := int64(1); i <= n; i++ {
			if err := tset(vm, tb, i, items[i-1]); err != nil {
				return nil, err
			}
		}
		return Multi(), nil
	})
	return t
}

// ---------- math ----------

func luaMathLib(L *luaState) *Table {
	t := NewTable()
	def := func(name string, f nativeFn) { t.SetStr(name, nat("math."+name, f)) }
	f1 := func(name string, fn func(float64) float64) {
		def(name, func(vm *VM, a []Value) (Value, error) {
			x, err := luaCheckFloat(name, a, 0)
			if err != nil {
				return nil, err
			}
			return fn(x), nil
		})
	}
	f1("sqrt", math.Sqrt)
	f1("sin", math.Sin)
	f1("cos", math.Cos)
	f1("tan", math.Tan)
	f1("asin", math.Asin)
	f1("acos", math.Acos)
	f1("exp", math.Exp)
	t.SetStr("pi", math.Pi)
	t.SetStr("huge", math.Inf(1))
	t.SetStr("maxinteger", int64(math.MaxInt64))
	t.SetStr("mininteger", int64(math.MinInt64))
	def("atan", func(vm *VM, a []Value) (Value, error) {
		y, err := luaCheckFloat("atan", a, 0)
		if err != nil {
			return nil, err
		}
		x := 1.0
		if arg(a, 1) != nil {
			if x, err = luaCheckFloat("atan", a, 1); err != nil {
				return nil, err
			}
		}
		return math.Atan2(y, x), nil
	})
	def("log", func(vm *VM, a []Value) (Value, error) {
		x, err := luaCheckFloat("log", a, 0)
		if err != nil {
			return nil, err
		}
		if arg(a, 1) == nil {
			return math.Log(x), nil
		}
		b, err := luaCheckFloat("log", a, 1)
		if err != nil {
			return nil, err
		}
		switch b {
		case 2:
			return math.Log2(x), nil
		case 10:
			return math.Log10(x), nil
		}
		return math.Log(x) / math.Log(b), nil
	})
	rounding := func(name string, fn func(float64) float64) {
		def(name, func(vm *VM, a []Value) (Value, error) {
			n, err := luaCheckNum(name, a, 0)
			if err != nil {
				return nil, err
			}
			if i, ok := n.(int64); ok {
				return i, nil
			}
			f := fn(n.(float64))
			if f >= -9.2233720368547758e18 && f < 9.2233720368547758e18 {
				return int64(f), nil
			}
			return f, nil
		})
	}
	rounding("floor", math.Floor)
	rounding("ceil", math.Ceil)
	def("abs", func(vm *VM, a []Value) (Value, error) {
		n, err := luaCheckNum("abs", a, 0)
		if err != nil {
			return nil, err
		}
		if i, ok := n.(int64); ok {
			if i < 0 {
				return -i, nil
			}
			return i, nil
		}
		return math.Abs(n.(float64)), nil
	})
	minmax := func(name string, less bool) {
		def(name, func(vm *VM, a []Value) (Value, error) {
			best, err := luaCheckNum(name, a, 0)
			if err != nil {
				return nil, err
			}
			for i := 1; i < len(a); i++ {
				v, err := luaCheckNum(name, a, i)
				if err != nil {
					return nil, err
				}
				lt, _ := vm.luaLess(v, best, false)
				gt, _ := vm.luaLess(best, v, false)
				if (less && lt) || (!less && gt) {
					best = v
				}
			}
			return best, nil
		})
	}
	minmax("min", true)
	minmax("max", false)
	def("fmod", func(vm *VM, a []Value) (Value, error) {
		x, err := luaCheckNum("fmod", a, 0)
		if err != nil {
			return nil, err
		}
		y, err := luaCheckNum("fmod", a, 1)
		if err != nil {
			return nil, err
		}
		xi, xok := x.(int64)
		yi, yok := y.(int64)
		if xok && yok {
			if yi == 0 {
				return nil, fmt.Errorf("argumento #2 inválido para 'fmod' (zero)")
			}
			if yi == -1 {
				return int64(0), nil
			}
			return xi % yi, nil
		}
		fx, _ := toFloat(x)
		fy, _ := toFloat(y)
		return math.Mod(fx, fy), nil
	})
	def("modf", func(vm *VM, a []Value) (Value, error) {
		x, err := luaCheckFloat("modf", a, 0)
		if err != nil {
			return nil, err
		}
		if math.IsInf(x, 0) {
			return Multi(x, 0.0), nil
		}
		ip, frac := math.Modf(x)
		var ipv Value = ip
		if ip >= -9.2233720368547758e18 && ip < 9.2233720368547758e18 {
			ipv = int64(ip)
		}
		return Multi(ipv, frac), nil
	})
	def("tointeger", func(vm *VM, a []Value) (Value, error) {
		switch x := arg(a, 0).(type) {
		case int64:
			return x, nil
		case float64:
			if i, ok := luaToInteger(x); ok {
				return i, nil
			}
		case string:
			if i, ok := luaToInteger(x); ok {
				return i, nil
			}
		}
		return nil, nil
	})
	def("type", func(vm *VM, a []Value) (Value, error) {
		if len(a) == 0 {
			return nil, luaArgErr("type", 0, "um valor", nil)
		}
		switch a[0].(type) {
		case int64:
			return "integer", nil
		case float64:
			return "float", nil
		}
		return nil, nil
	})
	def("ult", func(vm *VM, a []Value) (Value, error) {
		x, err := luaCheckInt("ult", a, 0)
		if err != nil {
			return nil, err
		}
		y, err := luaCheckInt("ult", a, 1)
		if err != nil {
			return nil, err
		}
		return uint64(x) < uint64(y), nil
	})
	def("random", func(vm *VM, a []Value) (Value, error) {
		r := vm.lua().rng
		switch len(a) {
		case 0:
			return r.Float64(), nil
		}
		lo, hi := int64(1), int64(0)
		var err error
		if len(a) == 1 {
			if hi, err = luaCheckInt("random", a, 0); err != nil {
				return nil, err
			}
			if hi == 0 {
				return int64(r.Uint64()), nil
			}
		} else {
			if lo, err = luaCheckInt("random", a, 0); err != nil {
				return nil, err
			}
			if hi, err = luaCheckInt("random", a, 1); err != nil {
				return nil, err
			}
		}
		if lo > hi {
			return nil, fmt.Errorf("argumento #%d inválido para 'random' (intervalo vazio)", len(a))
		}
		span := uint64(hi - lo)
		if span == math.MaxUint64 {
			return int64(r.Uint64()), nil
		}
		return lo + int64(r.Uint64()%(span+1)), nil
	})
	def("randomseed", func(vm *VM, a []Value) (Value, error) {
		seed := time.Now().UnixNano()
		if len(a) > 0 {
			n, err := luaCheckNum("randomseed", a, 0)
			if err != nil {
				return nil, err
			}
			if i, ok := n.(int64); ok {
				seed = i
			} else {
				seed = int64(math.Float64bits(n.(float64)))
			}
		}
		vm.lua().rng = rand.New(rand.NewSource(seed))
		return Multi(), nil
	})
	_ = L
	return t
}

// ---------- os ----------

func luaOSLib() *Table {
	t := NewTable()
	def := func(name string, f nativeFn) { t.SetStr(name, nat("os."+name, f)) }
	def("time", func(vm *VM, a []Value) (Value, error) {
		tb, ok := arg(a, 0).(*Table)
		if !ok {
			if arg(a, 0) != nil {
				return nil, luaArgErr("time", 0, "tabela", arg(a, 0))
			}
			return time.Now().Unix(), nil
		}
		field := func(k string, def int64, req bool) (int64, error) {
			v, _ := tb.Get(k)
			if v == nil {
				if req {
					return 0, fmt.Errorf("o campo '%s' está faltando na tabela de data", k)
				}
				return def, nil
			}
			i, ok := luaToInteger(v)
			if !ok {
				return 0, fmt.Errorf("o campo '%s' não é um inteiro", k)
			}
			return i, nil
		}
		y, err := field("year", 0, true)
		if err != nil {
			return nil, err
		}
		mo, err := field("month", 0, true)
		if err != nil {
			return nil, err
		}
		d, err := field("day", 0, true)
		if err != nil {
			return nil, err
		}
		h, _ := field("hour", 12, false)
		mi, _ := field("min", 0, false)
		s, _ := field("sec", 0, false)
		return time.Date(int(y), time.Month(mo), int(d), int(h), int(mi), int(s), 0, time.Local).Unix(), nil
	})
	def("clock", func(vm *VM, a []Value) (Value, error) {
		return time.Since(vm.lua().start).Seconds(), nil
	})
	def("difftime", func(vm *VM, a []Value) (Value, error) {
		x, err := luaCheckFloat("difftime", a, 0)
		if err != nil {
			return nil, err
		}
		y := 0.0
		if arg(a, 1) != nil {
			if y, err = luaCheckFloat("difftime", a, 1); err != nil {
				return nil, err
			}
		}
		return x - y, nil
	})
	def("date", func(vm *VM, a []Value) (Value, error) {
		f, err := luaOptStr("date", a, 0, "%c")
		if err != nil {
			return nil, err
		}
		tm := time.Now()
		if arg(a, 1) != nil {
			s, err := luaCheckInt("date", a, 1)
			if err != nil {
				return nil, err
			}
			tm = time.Unix(s, 0)
		}
		if strings.HasPrefix(f, "!") {
			tm = tm.UTC()
			f = f[1:]
		}
		if strings.HasPrefix(f, "*t") {
			r := NewTable()
			r.SetStr("year", int64(tm.Year()))
			r.SetStr("month", int64(tm.Month()))
			r.SetStr("day", int64(tm.Day()))
			r.SetStr("hour", int64(tm.Hour()))
			r.SetStr("min", int64(tm.Minute()))
			r.SetStr("sec", int64(tm.Second()))
			r.SetStr("wday", int64(tm.Weekday())+1)
			r.SetStr("yday", int64(tm.YearDay()))
			r.SetStr("isdst", tm.IsDST())
			return r, nil
		}
		return strftime(f, tm)
	})
	def("getenv", func(vm *VM, a []Value) (Value, error) {
		k, err := luaCheckStr("getenv", a, 0)
		if err != nil {
			return nil, err
		}
		if !vm.fsAllowed() {
			return nil, nil
		}
		v, ok := os.LookupEnv(k)
		if !ok {
			return nil, nil
		}
		return v, nil
	})
	def("exit", func(vm *VM, a []Value) (Value, error) {
		if !vm.fsAllowed() {
			return nil, fmt.Errorf("os.exit não está disponível aqui")
		}
		code := 0
		switch x := arg(a, 0).(type) {
		case bool:
			if !x {
				code = 1
			}
		case int64:
			code = int(x)
		}
		if f, ok := vm.Out.(interface{ Flush() error }); ok {
			f.Flush()
		}
		os.Exit(code)
		return nil, nil
	})
	fileOp := func(name string, fn func(a []Value) error) {
		def(name, func(vm *VM, a []Value) (Value, error) {
			if !vm.fsAllowed() {
				return nil, fmt.Errorf("os.%s não está disponível aqui", name)
			}
			if err := fn(a); err != nil {
				return Multi(nil, err.Error()), nil
			}
			return true, nil
		})
	}
	fileOp("remove", func(a []Value) error { return os.Remove(Str(arg(a, 0))) })
	fileOp("rename", func(a []Value) error { return os.Rename(Str(arg(a, 0)), Str(arg(a, 1))) })
	def("tmpname", func(vm *VM, a []Value) (Value, error) {
		if !vm.fsAllowed() {
			return nil, fmt.Errorf("os.tmpname não está disponível aqui")
		}
		f, err := os.CreateTemp("", "lua_")
		if err != nil {
			return nil, err
		}
		f.Close()
		return f.Name(), nil
	})
	return t
}

var (
	wdays   = []string{"Sunday", "Monday", "Tuesday", "Wednesday", "Thursday", "Friday", "Saturday"}
	monthsN = []string{"January", "February", "March", "April", "May", "June", "July", "August", "September", "October", "November", "December"}
)

func strftime(f string, t time.Time) (string, error) {
	var b strings.Builder
	for i := 0; i < len(f); i++ {
		if f[i] != '%' || i+1 >= len(f) {
			b.WriteByte(f[i])
			continue
		}
		i++
		switch f[i] {
		case 'Y':
			fmt.Fprintf(&b, "%d", t.Year())
		case 'y':
			fmt.Fprintf(&b, "%02d", t.Year()%100)
		case 'm':
			fmt.Fprintf(&b, "%02d", int(t.Month()))
		case 'd':
			fmt.Fprintf(&b, "%02d", t.Day())
		case 'e':
			fmt.Fprintf(&b, "%2d", t.Day())
		case 'H':
			fmt.Fprintf(&b, "%02d", t.Hour())
		case 'I':
			h := t.Hour() % 12
			if h == 0 {
				h = 12
			}
			fmt.Fprintf(&b, "%02d", h)
		case 'M':
			fmt.Fprintf(&b, "%02d", t.Minute())
		case 'S':
			fmt.Fprintf(&b, "%02d", t.Second())
		case 'p':
			if t.Hour() < 12 {
				b.WriteString("AM")
			} else {
				b.WriteString("PM")
			}
		case 'A':
			b.WriteString(wdays[t.Weekday()])
		case 'a':
			b.WriteString(wdays[t.Weekday()][:3])
		case 'B':
			b.WriteString(monthsN[t.Month()-1])
		case 'b', 'h':
			b.WriteString(monthsN[t.Month()-1][:3])
		case 'c':
			b.WriteString(t.Format("Mon Jan  2 15:04:05 2006"))
		case 'x':
			b.WriteString(t.Format("01/02/06"))
		case 'X', 'T':
			b.WriteString(t.Format("15:04:05"))
		case 'D':
			b.WriteString(t.Format("01/02/06"))
		case 'F':
			b.WriteString(t.Format("2006-01-02"))
		case 'R':
			b.WriteString(t.Format("15:04"))
		case 'j':
			fmt.Fprintf(&b, "%03d", t.YearDay())
		case 'w':
			fmt.Fprintf(&b, "%d", int(t.Weekday()))
		case 'u':
			w := int(t.Weekday())
			if w == 0 {
				w = 7
			}
			fmt.Fprintf(&b, "%d", w)
		case 'Z':
			z, _ := t.Zone()
			b.WriteString(z)
		case 'z':
			b.WriteString(t.Format("-0700"))
		case 's':
			fmt.Fprintf(&b, "%d", t.Unix())
		case 'n':
			b.WriteByte('\n')
		case 't':
			b.WriteByte('\t')
		case '%':
			b.WriteByte('%')
		default:
			return "", fmt.Errorf("conversão inválida '%%%c' no formato de data", f[i])
		}
	}
	return b.String(), nil
}

// ---------- io ----------

type luaFile struct {
	f      *os.File
	r      *bufio.Reader
	w      io.Writer
	closed bool
	std    bool
	stdout bool // escreve em vm.Out
	stdin  bool // lê de vm.In
}

func (vm *VM) stdinReader() *bufio.Reader {
	L := vm.lua()
	if L.stdin == nil {
		L.stdin = bufio.NewReader(vm.In)
	}
	return L.stdin
}

func readFormat(r *bufio.Reader, f Value) (Value, error) {
	if n, ok := luaToInteger(f); ok {
		buf := make([]byte, n)
		k, err := io.ReadFull(r, buf)
		if k == 0 && n > 0 {
			return nil, nil
		}
		if err != nil && err != io.ErrUnexpectedEOF && err != io.EOF {
			return nil, err
		}
		return string(buf[:k]), nil
	}
	s, _ := f.(string)
	s = strings.TrimPrefix(s, "*")
	switch {
	case strings.HasPrefix(s, "l"), strings.HasPrefix(s, "L"):
		line, err := r.ReadString('\n')
		if err != nil && line == "" {
			return nil, nil
		}
		if s[0] == 'l' {
			line = strings.TrimSuffix(line, "\n")
		}
		return line, nil
	case strings.HasPrefix(s, "a"):
		all, err := io.ReadAll(r)
		if err != nil {
			return nil, err
		}
		return string(all), nil
	case strings.HasPrefix(s, "n"):
		var tok strings.Builder
		for {
			c, err := r.ReadByte()
			if err != nil {
				break
			}
			if tok.Len() == 0 && (c == ' ' || c == '\n' || c == '\t' || c == '\r') {
				continue
			}
			if isHex(c) || c == '.' || c == '-' || c == '+' || c == 'x' || c == 'X' || c == 'p' || c == 'P' {
				tok.WriteByte(c)
				continue
			}
			r.UnreadByte()
			break
		}
		n, ok := parseLuaNumber(tok.String())
		if !ok {
			return nil, nil
		}
		return n, nil
	}
	return nil, fmt.Errorf("formato de leitura inválido")
}

func readAll(r *bufio.Reader, fmts []Value) (Value, error) {
	if len(fmts) == 0 {
		fmts = []Value{"l"}
	}
	var out []Value
	for _, f := range fmts {
		v, err := readFormat(r, f)
		if err != nil {
			return nil, err
		}
		out = append(out, v)
		if v == nil {
			break
		}
	}
	return Multi(out...), nil
}

func writeAll(vm *VM, w io.Writer, a []Value) error {
	for i, v := range a {
		s, ok := luaNumStr(v)
		if f, isF := v.(float64); isF {
			s = fmtG14(f) // io.write usa %.14g, sem o ".0"
		}
		if !ok {
			return luaArgErr("write", i, "texto", v)
		}
		if _, err := io.WriteString(w, s); err != nil {
			return err
		}
	}
	return nil
}

func luaIOLib() *Table {
	t := NewTable()
	def := func(name string, f nativeFn) { t.SetStr(name, nat("io."+name, f)) }
	fileMeta := NewTable()
	methods := NewTable()
	fileMeta.SetStr("__index", methods)
	fileMeta.SetStr("__name", "FILE*")
	getFile := func(fn string, a []Value) (*luaFile, *Table, error) {
		tb, ok := arg(a, 0).(*Table)
		if ok {
			if h, ok := tb.udata.(*luaFile); ok {
				if h.closed {
					return nil, nil, fmt.Errorf("tentou usar um arquivo fechado")
				}
				return h, tb, nil
			}
		}
		return nil, nil, luaArgErr(fn, 0, "um arquivo", arg(a, 0))
	}
	newFile := func(lf *luaFile) *Table {
		tb := NewTable()
		tb.meta = fileMeta
		tb.udata = lf
		return tb
	}
	mdef := func(name string, f nativeFn) { methods.SetStr(name, nat(name, f)) }
	mdef("write", func(vm *VM, a []Value) (Value, error) {
		h, tb, err := getFile("write", a)
		if err != nil {
			return nil, err
		}
		w := h.w
		if h.stdout {
			w = vm.Out
		}
		if w == nil {
			return Multi(nil, "arquivo aberto só para leitura"), nil
		}
		if err := writeAll(vm, w, a[1:]); err != nil {
			return Multi(nil, err.Error()), nil
		}
		return tb, nil
	})
	mdef("read", func(vm *VM, a []Value) (Value, error) {
		h, _, err := getFile("read", a)
		if err != nil {
			return nil, err
		}
		if h.stdin {
			return readAll(vm.stdinReader(), a[1:])
		}
		if h.r == nil {
			return Multi(nil, "arquivo aberto só para escrita"), nil
		}
		return readAll(h.r, a[1:])
	})
	mdef("lines", func(vm *VM, a []Value) (Value, error) {
		h, _, err := getFile("lines", a)
		if err != nil {
			return nil, err
		}
		fmts := append([]Value(nil), a[1:]...)
		return nat("lines_iter", func(vm *VM, _ []Value) (Value, error) {
			if h.stdin {
				return readAll(vm.stdinReader(), fmts)
			}
			if h.closed || h.r == nil {
				return nil, nil
			}
			return readAll(h.r, fmts)
		}), nil
	})
	mdef("close", func(vm *VM, a []Value) (Value, error) {
		h, _, err := getFile("close", a)
		if err != nil {
			return nil, err
		}
		if h.std {
			return Multi(nil, "não dá para fechar um arquivo padrão"), nil
		}
		h.closed = true
		if bw, ok := h.w.(*bufio.Writer); ok {
			bw.Flush()
		}
		return true, h.f.Close()
	})
	mdef("flush", func(vm *VM, a []Value) (Value, error) {
		h, tb, err := getFile("flush", a)
		if err != nil {
			return nil, err
		}
		w := h.w
		if h.stdout {
			w = vm.Out
		}
		if fl, ok := w.(interface{ Flush() error }); ok {
			fl.Flush()
		}
		return tb, nil
	})
	mdef("seek", func(vm *VM, a []Value) (Value, error) {
		h, _, err := getFile("seek", a)
		if err != nil {
			return nil, err
		}
		if h.f == nil {
			return Multi(nil, "não dá para usar seek aqui"), nil
		}
		whence, _ := luaOptStr("seek", a, 1, "cur")
		off, _ := luaOptInt("seek", a, 2, 0)
		if bw, ok := h.w.(*bufio.Writer); ok {
			bw.Flush()
		}
		w := map[string]int{"set": 0, "cur": 1, "end": 2}[whence]
		if w == 1 && h.r != nil {
			off -= int64(h.r.Buffered())
		}
		pos, err := h.f.Seek(off, w)
		if err != nil {
			return Multi(nil, err.Error()), nil
		}
		if h.r != nil {
			h.r.Reset(h.f)
		}
		return pos, nil
	})
	fileMeta.SetStr("__tostring", nat("tostring", func(vm *VM, a []Value) (Value, error) {
		if tb, ok := arg(a, 0).(*Table); ok {
			if h, ok := tb.udata.(*luaFile); ok && h.closed {
				return "file (closed)", nil
			}
		}
		return fmt.Sprintf("file (%p)", arg(a, 0)), nil
	}))

	def("write", func(vm *VM, a []Value) (Value, error) {
		if err := writeAll(vm, vm.Out, a); err != nil {
			return nil, err
		}
		out, _ := t.Get("stdout")
		return out, nil
	})
	def("read", func(vm *VM, a []Value) (Value, error) { return readAll(vm.stdinReader(), a) })
	def("lines", func(vm *VM, a []Value) (Value, error) {
		var r *bufio.Reader
		var f *os.File
		if name, ok := arg(a, 0).(string); ok {
			if !vm.fsAllowed() {
				return nil, fmt.Errorf("io.lines não está disponível aqui")
			}
			var err error
			f, err = os.Open(name)
			if err != nil {
				return nil, fmt.Errorf("%s: %v", name, err)
			}
			r = bufio.NewReader(f)
		} else {
			r = vm.stdinReader()
		}
		fmts := argsFrom(a, 1)
		return nat("lines_iter", func(vm *VM, _ []Value) (Value, error) {
			v, err := readAll(r, fmts)
			if (v == nil || err != nil) && f != nil {
				f.Close()
			}
			return v, err
		}), nil
	})
	def("open", func(vm *VM, a []Value) (Value, error) {
		if !vm.fsAllowed() {
			return nil, fmt.Errorf("io.open não está disponível aqui")
		}
		name, err := luaCheckStr("open", a, 0)
		if err != nil {
			return nil, err
		}
		mode, _ := luaOptStr("open", a, 1, "r")
		flag := os.O_RDONLY
		m := strings.TrimSuffix(mode, "b")
		switch m {
		case "r":
		case "w":
			flag = os.O_WRONLY | os.O_CREATE | os.O_TRUNC
		case "a":
			flag = os.O_WRONLY | os.O_CREATE | os.O_APPEND
		case "r+":
			flag = os.O_RDWR
		case "w+":
			flag = os.O_RDWR | os.O_CREATE | os.O_TRUNC
		case "a+":
			flag = os.O_RDWR | os.O_CREATE | os.O_APPEND
		default:
			return nil, fmt.Errorf("argumento #2 inválido para 'open' (modo inválido)")
		}
		f, oerr := os.OpenFile(name, flag, 0o644)
		if oerr != nil {
			return Multi(nil, name+": "+oerr.Error(), int64(2)), nil
		}
		lf := &luaFile{f: f}
		if flag == os.O_RDONLY || flag&os.O_RDWR != 0 {
			lf.r = bufio.NewReader(f)
		}
		if flag != os.O_RDONLY {
			lf.w = f
		}
		return newFile(lf), nil
	})
	def("close", func(vm *VM, a []Value) (Value, error) {
		if len(a) == 0 {
			return nil, nil
		}
		cl, _ := methods.Get("close")
		return vm.Call(cl, a)
	})
	def("type", func(vm *VM, a []Value) (Value, error) {
		if tb, ok := arg(a, 0).(*Table); ok {
			if h, ok := tb.udata.(*luaFile); ok {
				if h.closed {
					return "closed file", nil
				}
				return "file", nil
			}
		}
		return nil, nil
	})
	t.SetStr("stdout", newFile(&luaFile{stdout: true, std: true}))
	t.SetStr("stderr", newFile(&luaFile{w: os.Stderr, std: true}))
	t.SetStr("stdin", newFile(&luaFile{stdin: true, std: true}))
	return t
}

// ---------- utf8 ----------

func luaUTF8Lib() *Table {
	t := NewTable()
	def := func(name string, f nativeFn) { t.SetStr(name, nat("utf8."+name, f)) }
	t.SetStr("charpattern", "[\x00-\x7F\xC2-\xFD][\x80-\xBF]*")
	def("char", func(vm *VM, a []Value) (Value, error) {
		var b strings.Builder
		for i := range a {
			n, err := luaCheckInt("char", a, i)
			if err != nil {
				return nil, err
			}
			b.WriteRune(rune(n))
		}
		return b.String(), nil
	})
	def("codepoint", func(vm *VM, a []Value) (Value, error) {
		s, err := luaCheckStr("codepoint", a, 0)
		if err != nil {
			return nil, err
		}
		i, _ := luaOptInt("codepoint", a, 1, 1)
		j, _ := luaOptInt("codepoint", a, 2, i)
		b, e := luaRange(i, j, len(s))
		var out []Value
		for k := b - 1; k < e; {
			r, size := utf8.DecodeRuneInString(s[k:])
			if r == utf8.RuneError && size <= 1 {
				return nil, fmt.Errorf("código UTF-8 inválido")
			}
			out = append(out, int64(r))
			k += size
		}
		return Multi(out...), nil
	})
	def("len", func(vm *VM, a []Value) (Value, error) {
		s, err := luaCheckStr("len", a, 0)
		if err != nil {
			return nil, err
		}
		i, _ := luaOptInt("len", a, 1, 1)
		j, _ := luaOptInt("len", a, 2, -1)
		b, e := luaRange(i, j, len(s))
		n := int64(0)
		for k := b - 1; k < e; {
			r, size := utf8.DecodeRuneInString(s[k:])
			if r == utf8.RuneError && size <= 1 {
				return Multi(nil, int64(k+1)), nil
			}
			k += size
			n++
		}
		return n, nil
	})
	def("offset", func(vm *VM, a []Value) (Value, error) {
		s, err := luaCheckStr("offset", a, 0)
		if err != nil {
			return nil, err
		}
		n, err := luaCheckInt("offset", a, 1)
		if err != nil {
			return nil, err
		}
		defI := int64(1)
		if n < 0 {
			defI = int64(len(s)) + 1
		}
		i, _ := luaOptInt("offset", a, 2, defI)
		pos := int(i - 1)
		cont := func(p int) bool { return p < len(s) && s[p]&0xC0 == 0x80 }
		if n == 0 {
			for pos > 0 && cont(pos) {
				pos--
			}
			return int64(pos + 1), nil
		}
		if cont(pos) {
			return nil, fmt.Errorf("posição inicial é uma continuação de caractere")
		}
		if n < 0 {
			for n < 0 && pos > 0 {
				pos--
				for pos > 0 && cont(pos) {
					pos--
				}
				n++
			}
		} else {
			n--
			for n > 0 && pos < len(s) {
				pos++
				for cont(pos) {
					pos++
				}
				n--
			}
		}
		if n == 0 {
			return int64(pos + 1), nil
		}
		return nil, nil
	})
	def("codes", func(vm *VM, a []Value) (Value, error) {
		s, err := luaCheckStr("codes", a, 0)
		if err != nil {
			return nil, err
		}
		iter := nat("codes_iter", func(vm *VM, b []Value) (Value, error) {
			i, _ := luaIntKey(arg(b, 1))
			k := int(i)
			if k > 0 {
				_, size := utf8.DecodeRuneInString(s[k-1:])
				k += size - 1
			}
			if k >= len(s) {
				return nil, nil
			}
			r, size := utf8.DecodeRuneInString(s[k:])
			if r == utf8.RuneError && size <= 1 {
				return nil, fmt.Errorf("código UTF-8 inválido")
			}
			return Multi(int64(k+1), int64(r)), nil
		})
		return Multi(iter, s, int64(0)), nil
	})
	return t
}

// addLuaBuiltins dá ao Nepenthes o que veio do Lua: metatabelas, corrotinas e
// padrões de texto (match, gmatch, gsub).
func addLuaBuiltins(b map[string]Value) {
	def := func(name string, f nativeFn) { b[name] = nat(name, f) }
	def("setmetatable", setMetatable)
	def("getmetatable", getMetatable)
	def("rawget", func(vm *VM, a []Value) (Value, error) {
		t, err := luaCheckTable("rawget", a, 0)
		if err != nil {
			return nil, err
		}
		v, _ := t.Get(arg(a, 1))
		return v, nil
	})
	def("rawset", func(vm *VM, a []Value) (Value, error) {
		t, err := luaCheckTable("rawset", a, 0)
		if err != nil {
			return nil, err
		}
		return t, t.Set(arg(a, 1), arg(a, 2))
	})
	b["coroutine"] = coroutineLib()
	pat := func(fn string, a []Value) (string, string, error) {
		s, err := luaCheckStr(fn, a, 0)
		if err != nil {
			return "", "", err
		}
		p, err := luaCheckStr(fn, a, 1)
		return s, p, err
	}
	def("match", func(vm *VM, a []Value) (Value, error) {
		s, p, err := pat("match", a)
		if err != nil {
			return nil, err
		}
		init, err := luaOptInt("match", a, 2, 1)
		if err != nil {
			return nil, err
		}
		st, e, ms, err := patFind(s, p, strStart(init, len(s)))
		if err != nil || st < 0 {
			return nil, err
		}
		return nepValues(ms.captures(st, e, true)), ms.err
	})
	def("gmatch", func(vm *VM, a []Value) (Value, error) {
		s, p, err := pat("gmatch", a)
		if err != nil {
			return nil, err
		}
		out := NewList()
		pos, lastmatch := 0, -1
		for pos <= len(s) {
			ms := &patState{src: s, pat: p}
			e := ms.match(pos, 0)
			if ms.err != nil {
				return nil, ms.err
			}
			if e != -1 && e != lastmatch {
				out.Items = append(out.Items, nepValues(ms.captures(pos, e, true)))
				pos, lastmatch = e, e
				continue
			}
			pos++
		}
		return out, nil
	})
	def("gsub", func(vm *VM, a []Value) (Value, error) {
		s, p, err := pat("gsub", a)
		if err != nil {
			return nil, err
		}
		n := int64(-1)
		if arg(a, 3) != nil {
			if n, err = luaCheckInt("gsub", a, 3); err != nil {
				return nil, err
			}
		}
		r, _, err := vm.patGsub(s, p, arg(a, 2), n)
		return r, err
	})
}
