package lang

import (
	"bufio"
	"encoding/csv"
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"
)

type nativeFn = func(vm *VM, a []Value) (Value, error)

func nat(name string, f nativeFn) *Native { return &Native{Name: name, Fn: f} }

func arg(a []Value, i int) Value {
	if i < len(a) {
		return a[i]
	}
	return nil
}

func argErr(fn string, i int, want string, got Value) error {
	return fmt.Errorf("%s: o argumento %d precisa ser %s (recebeu %s)", fn, i+1, want, TypeName(got))
}

func argStr(fn string, a []Value, i int) (string, error) {
	if s, ok := arg(a, i).(string); ok {
		return s, nil
	}
	return "", argErr(fn, i, "texto", arg(a, i))
}

func argInt(fn string, a []Value, i int, def int64) (int64, error) {
	switch x := arg(a, i).(type) {
	case nil:
		return def, nil
	case int64:
		return x, nil
	case float64:
		if x == math.Trunc(x) {
			return int64(x), nil
		}
	}
	return 0, argErr(fn, i, "um inteiro", arg(a, i))
}

func argNum(fn string, a []Value, i int) (float64, error) {
	if f, ok := toFloat(arg(a, i)); ok {
		return f, nil
	}
	return 0, argErr(fn, i, "um número", arg(a, i))
}

func argList(fn string, a []Value, i int) ([]Value, error) {
	switch x := arg(a, i).(type) {
	case *List:
		return x.Items, nil
	case nil:
		return nil, nil
	case *Table:
		return x.Values(), nil
	}
	return nil, argErr(fn, i, "uma lista", arg(a, i))
}

func argFn(fn string, a []Value, i int) (Value, error) {
	switch arg(a, i).(type) {
	case *Closure, *Native:
		return arg(a, i), nil
	}
	return nil, argErr(fn, i, "uma função", arg(a, i))
}

// keyFn aceita uma função ou um nome de campo ("preco").
func keyFn(vm *VM, fname string, a []Value, i int) (func(Value) (Value, error), error) {
	switch k := arg(a, i).(type) {
	case string:
		return func(v Value) (Value, error) {
			if t, ok := v.(*Table); ok {
				r, _ := t.Get(k)
				return r, nil
			}
			return nil, nil
		}, nil
	case *Closure, *Native:
		return func(v Value) (Value, error) { return vm.Call(k, []Value{v}) }, nil
	}
	return nil, argErr(fname, i, "uma função ou nome de campo", arg(a, i))
}

func numVal(v Value) (Value, bool) {
	switch x := v.(type) {
	case int64, float64:
		return x, true
	case string:
		s := strings.TrimSpace(x)
		if n, err := strconv.ParseInt(s, 10, 64); err == nil {
			return n, true
		}
		if f, err := strconv.ParseFloat(s, 64); err == nil {
			return f, true
		}
	}
	return nil, false
}

func numbers(items []Value) []float64 {
	var r []float64
	for _, it := range items {
		if n, ok := numVal(it); ok {
			f, _ := toFloat(n)
			r = append(r, f)
		}
	}
	return r
}

func allInts(items []Value) bool {
	for _, it := range items {
		if _, ok := it.(int64); !ok {
			return false
		}
	}
	return true
}

func brNumber(f float64, dec int) string {
	s := strconv.FormatFloat(math.Abs(f), 'f', dec, 64)
	intPart, frac := s, ""
	if i := strings.IndexByte(s, '.'); i >= 0 {
		intPart, frac = s[:i], s[i+1:]
	}
	var b strings.Builder
	for i, c := range intPart {
		if i > 0 && (len(intPart)-i)%3 == 0 {
			b.WriteByte('.')
		}
		b.WriteRune(c)
	}
	out := b.String()
	if frac != "" {
		out += "," + frac
	}
	if f < 0 && strings.Trim(out, "0.,") != "" {
		out = "-" + out
	}
	return out
}

var accents = strings.NewReplacer("á", "a", "à", "a", "â", "a", "ã", "a", "ä", "a", "é", "e", "ê", "e", "è", "e", "ë", "e",
	"í", "i", "ì", "i", "î", "i", "ï", "i", "ó", "o", "ò", "o", "ô", "o", "õ", "o", "ö", "o", "ú", "u", "ù", "u", "û", "u", "ü", "u", "ç", "c", "ñ", "n")

// phpDate formata datas com as letras do PHP (d/m/Y H:i:s), iguais às do Nepenthes para web.
func phpDate(t time.Time, f string) string {
	var b strings.Builder
	dias := []string{"dom", "seg", "ter", "qua", "qui", "sex", "sáb"}
	for _, c := range f {
		switch c {
		case 'd':
			fmt.Fprintf(&b, "%02d", t.Day())
		case 'j':
			fmt.Fprintf(&b, "%d", t.Day())
		case 'm':
			fmt.Fprintf(&b, "%02d", int(t.Month()))
		case 'n':
			fmt.Fprintf(&b, "%d", int(t.Month()))
		case 'Y':
			fmt.Fprintf(&b, "%04d", t.Year())
		case 'y':
			fmt.Fprintf(&b, "%02d", t.Year()%100)
		case 'H':
			fmt.Fprintf(&b, "%02d", t.Hour())
		case 'G':
			fmt.Fprintf(&b, "%d", t.Hour())
		case 'i':
			fmt.Fprintf(&b, "%02d", t.Minute())
		case 's':
			fmt.Fprintf(&b, "%02d", t.Second())
		case 'D':
			b.WriteString(dias[t.Weekday()])
		case 'w':
			fmt.Fprintf(&b, "%d", int(t.Weekday()))
		case 'U':
			fmt.Fprintf(&b, "%d", t.Unix())
		default:
			b.WriteRune(c)
		}
	}
	return b.String()
}

func parseTime(v Value) (time.Time, error) {
	switch x := v.(type) {
	case nil:
		return time.Now(), nil
	case int64:
		return time.Unix(x, 0), nil
	case float64:
		return time.Unix(int64(x), int64((x-math.Floor(x))*1e9)), nil
	case string:
		for _, layout := range []string{"2006-01-02 15:04:05", "2006-01-02T15:04:05Z07:00", "2006-01-02T15:04:05", "2006-01-02", "02/01/2006 15:04", "02/01/2006"} {
			if t, err := time.ParseInLocation(layout, strings.TrimSpace(x), time.Local); err == nil {
				return t, nil
			}
		}
		return time.Time{}, fmt.Errorf("data inválida: %q (use 2026-11-08 ou 08/11/2026)", x)
	}
	return time.Time{}, fmt.Errorf("data inválida: %s", TypeName(v))
}

func coreBuiltins() map[string]Value {
	b := map[string]Value{}
	def := func(name string, f nativeFn) { b[name] = nat(name, f) }

	// ---------- básico ----------
	def("print", func(vm *VM, a []Value) (Value, error) {
		parts := make([]string, len(a))
		for i, v := range a {
			parts[i] = Str(v)
		}
		fmt.Fprintln(vm.Out, strings.Join(parts, " "))
		return nil, nil
	})
	def("input", func(vm *VM, a []Value) (Value, error) {
		if len(a) > 0 {
			fmt.Fprint(vm.Out, Str(a[0]))
		}
		r := bufio.NewReader(vm.In)
		line, err := r.ReadString('\n')
		if err != nil && line == "" {
			return nil, nil
		}
		return strings.TrimRight(line, "\r\n"), nil
	})
	def("type", func(vm *VM, a []Value) (Value, error) { return TypeName(arg(a, 0)), nil })
	def("str", func(vm *VM, a []Value) (Value, error) { return Str(arg(a, 0)), nil })
	def("__str", func(vm *VM, a []Value) (Value, error) { return Str(arg(a, 0)), nil }) // usado pelos textos com crases
	def("repr", func(vm *VM, a []Value) (Value, error) { return Repr(arg(a, 0)), nil })
	def("int", func(vm *VM, a []Value) (Value, error) {
		v := arg(a, 0)
		if n, ok := numVal(v); ok {
			if f, isF := n.(float64); isF {
				return int64(f), nil
			}
			return n, nil
		}
		if bl, ok := v.(bool); ok {
			if bl {
				return int64(1), nil
			}
			return int64(0), nil
		}
		return nil, nil
	})
	def("float", func(vm *VM, a []Value) (Value, error) {
		if n, ok := numVal(arg(a, 0)); ok {
			f, _ := toFloat(n)
			return f, nil
		}
		return nil, nil
	})
	def("bool", func(vm *VM, a []Value) (Value, error) { return Truthy(arg(a, 0)), nil })
	def("len", func(vm *VM, a []Value) (Value, error) {
		switch x := arg(a, 0).(type) {
		case string:
			return int64(utf8.RuneCountInString(x)), nil
		case *List:
			return int64(len(x.Items)), nil
		case *Table:
			return int64(x.Len()), nil
		case nil:
			return int64(0), nil
		}
		return nil, argErr("len", 0, "texto, lista ou tabela", arg(a, 0))
	})
	def("error", func(vm *VM, a []Value) (Value, error) {
		v := arg(a, 0)
		if s, ok := v.(string); ok {
			return nil, Throw(vm.errorTable(s))
		}
		return nil, Throw(v)
	})
	def("assert", func(vm *VM, a []Value) (Value, error) {
		if Truthy(arg(a, 0)) {
			return arg(a, 0), nil
		}
		msg := "asserção falhou"
		if len(a) > 1 {
			msg = Str(a[1])
		}
		return nil, Throw(vm.errorTable(msg))
	})
	def("default", func(vm *VM, a []Value) (Value, error) {
		if Truthy(arg(a, 0)) {
			return arg(a, 0), nil
		}
		return arg(a, 1), nil
	})
	def("copy", func(vm *VM, a []Value) (Value, error) {
		switch x := arg(a, 0).(type) {
		case *List:
			return NewList(append([]Value(nil), x.Items...)...), nil
		case *Table:
			t := NewTable()
			for i := 0; i < x.Len(); i++ {
				k, v := x.Entry(i)
				_ = t.Set(k, v)
			}
			return t, nil
		}
		return arg(a, 0), nil
	})

	// ---------- texto ----------
	strFn := func(name string, f func(string) string) {
		def(name, func(vm *VM, a []Value) (Value, error) { return f(Str(arg(a, 0))), nil })
	}
	strFn("upper", strings.ToUpper)
	strFn("lower", strings.ToLower)
	strFn("trim", strings.TrimSpace)
	strFn("capitalize", func(s string) string {
		r, n := utf8.DecodeRuneInString(s)
		if n == 0 {
			return s
		}
		return string(unicode.ToUpper(r)) + s[n:]
	})
	strFn("slug", func(s string) string {
		s = accents.Replace(strings.ToLower(s))
		var b strings.Builder
		dash := false
		for _, r := range s {
			if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
				b.WriteRune(r)
				dash = false
			} else if !dash && b.Len() > 0 {
				b.WriteByte('-')
				dash = true
			}
		}
		return strings.TrimRight(b.String(), "-")
	})
	def("replace", func(vm *VM, a []Value) (Value, error) {
		return strings.ReplaceAll(Str(arg(a, 0)), Str(arg(a, 1)), Str(arg(a, 2))), nil
	})
	def("split", func(vm *VM, a []Value) (Value, error) {
		s := Str(arg(a, 0))
		sep := ","
		if len(a) > 1 {
			sep = Str(a[1])
		}
		var parts []string
		if sep == "" {
			for _, r := range s {
				parts = append(parts, string(r))
			}
		} else {
			parts = strings.Split(s, sep)
		}
		l := make([]Value, len(parts))
		for i, p := range parts {
			l[i] = p
		}
		return NewList(l...), nil
	})
	def("join", func(vm *VM, a []Value) (Value, error) {
		items, err := argList("join", a, 0)
		if err != nil {
			return nil, err
		}
		sep := ", "
		if len(a) > 1 {
			sep = Str(a[1])
		}
		parts := make([]string, len(items))
		for i, it := range items {
			parts[i] = Str(it)
		}
		return strings.Join(parts, sep), nil
	})
	def("starts_with", func(vm *VM, a []Value) (Value, error) { return strings.HasPrefix(Str(arg(a, 0)), Str(arg(a, 1))), nil })
	def("ends_with", func(vm *VM, a []Value) (Value, error) { return strings.HasSuffix(Str(arg(a, 0)), Str(arg(a, 1))), nil })
	def("contains", func(vm *VM, a []Value) (Value, error) {
		switch h := arg(a, 0).(type) {
		case *List:
			for _, it := range h.Items {
				if Equal(it, arg(a, 1)) {
					return true, nil
				}
			}
			return false, nil
		case *Table:
			_, ok := h.Get(arg(a, 1))
			return ok, nil
		}
		return strings.Contains(Str(arg(a, 0)), Str(arg(a, 1))), nil
	})
	def("find_index", func(vm *VM, a []Value) (Value, error) {
		switch h := arg(a, 0).(type) {
		case *List:
			for i, it := range h.Items {
				if Equal(it, arg(a, 1)) {
					return int64(i), nil
				}
			}
			return int64(-1), nil
		}
		s, sub := Str(arg(a, 0)), Str(arg(a, 1))
		i := strings.Index(s, sub)
		if i < 0 {
			return int64(-1), nil
		}
		return int64(utf8.RuneCountInString(s[:i])), nil
	})
	def("substr", func(vm *VM, a []Value) (Value, error) {
		rs := []rune(Str(arg(a, 0)))
		start, err := argInt("substr", a, 1, 0)
		if err != nil {
			return nil, err
		}
		n, err := argInt("substr", a, 2, int64(len(rs)))
		if err != nil {
			return nil, err
		}
		if start < 0 {
			start += int64(len(rs))
		}
		if start < 0 {
			start = 0
		}
		if start > int64(len(rs)) {
			return "", nil
		}
		end := start + n
		if end > int64(len(rs)) || n < 0 {
			end = int64(len(rs))
		}
		return string(rs[start:end]), nil
	})
	def("truncate", func(vm *VM, a []Value) (Value, error) {
		rs := []rune(Str(arg(a, 0)))
		n, err := argInt("truncate", a, 1, 100)
		if err != nil {
			return nil, err
		}
		suf := "…"
		if len(a) > 2 {
			suf = Str(a[2])
		}
		if int64(len(rs)) <= n {
			return string(rs), nil
		}
		return strings.TrimRight(string(rs[:n]), " ") + suf, nil
	})
	def("pad", func(vm *VM, a []Value) (Value, error) {
		s := Str(arg(a, 0))
		n, err := argInt("pad", a, 1, 0)
		if err != nil {
			return nil, err
		}
		ch := " "
		if len(a) > 2 && Str(a[2]) != "" {
			ch = Str(a[2])
		}
		for int64(utf8.RuneCountInString(s)) < n {
			s = ch + s
		}
		return s, nil
	})
	def("format", func(vm *VM, a []Value) (Value, error) {
		// format("Olá, {}! Você tem {} anos", nome, idade)  ou  format("{nome}", tabela)
		tpl := Str(arg(a, 0))
		var b strings.Builder
		next := 1
		var named *Table
		if len(a) == 2 {
			named, _ = a[1].(*Table)
		}
		for i := 0; i < len(tpl); i++ {
			if tpl[i] == '{' {
				if i+1 < len(tpl) && tpl[i+1] == '{' {
					b.WriteByte('{')
					i++
					continue
				}
				j := strings.IndexByte(tpl[i:], '}')
				if j < 0 {
					b.WriteString(tpl[i:])
					break
				}
				key := tpl[i+1 : i+j]
				if key == "" {
					b.WriteString(Str(arg(a, next)))
					next++
				} else if named != nil {
					v, _ := named.Get(key)
					b.WriteString(Str(v))
				} else if k, err := strconv.Atoi(key); err == nil {
					b.WriteString(Str(arg(a, k+1)))
				}
				i += j
				continue
			}
			if tpl[i] == '}' && i+1 < len(tpl) && tpl[i+1] == '}' {
				i++
			}
			b.WriteByte(tpl[i])
		}
		return b.String(), nil
	})

	// ---------- números ----------
	def("round", func(vm *VM, a []Value) (Value, error) {
		f, err := argNum("round", a, 0)
		if err != nil {
			return nil, err
		}
		d, err := argInt("round", a, 1, 0)
		if err != nil {
			return nil, err
		}
		p := math.Pow(10, float64(d))
		r := math.Round(f*p) / p
		if d == 0 {
			return int64(r), nil
		}
		return r, nil
	})
	def("floor", func(vm *VM, a []Value) (Value, error) {
		f, err := argNum("floor", a, 0)
		return int64(math.Floor(f)), err
	})
	def("ceil", func(vm *VM, a []Value) (Value, error) {
		f, err := argNum("ceil", a, 0)
		return int64(math.Ceil(f)), err
	})
	def("abs", func(vm *VM, a []Value) (Value, error) {
		switch x := arg(a, 0).(type) {
		case int64:
			if x < 0 {
				return -x, nil
			}
			return x, nil
		case float64:
			return math.Abs(x), nil
		}
		return nil, argErr("abs", 0, "um número", arg(a, 0))
	})
	def("money", func(vm *VM, a []Value) (Value, error) {
		n, _ := numVal(arg(a, 0))
		f, _ := toFloat(n)
		return "R$ " + brNumber(f, 2), nil
	})
	def("number", func(vm *VM, a []Value) (Value, error) {
		n, _ := numVal(arg(a, 0))
		f, _ := toFloat(n)
		d, err := argInt("number", a, 1, 0)
		return brNumber(f, int(d)), err
	})
	aggregate := func(name string, f func(items []Value, nums []float64) Value) {
		def(name, func(vm *VM, a []Value) (Value, error) {
			items := a
			if len(a) == 1 {
				if l, ok := a[0].(*List); ok {
					items = l.Items
				} else if t, ok := a[0].(*Table); ok {
					items = t.Values()
				}
			}
			return f(items, numbers(items)), nil
		})
	}
	aggregate("sum", func(items []Value, n []float64) Value {
		if allInts(items) {
			var s int64
			for _, it := range items {
				s += it.(int64)
			}
			return s
		}
		s := 0.0
		for _, x := range n {
			s += x
		}
		return s
	})
	aggregate("avg", func(items []Value, n []float64) Value {
		if len(n) == 0 {
			return nil
		}
		s := 0.0
		for _, x := range n {
			s += x
		}
		return s / float64(len(n))
	})
	minmax := func(name string, better func(a, b float64) bool) {
		aggregate(name, func(items []Value, n []float64) Value {
			var best Value
			bf := 0.0
			for _, it := range items {
				nv, ok := numVal(it)
				if !ok {
					continue
				}
				f, _ := toFloat(nv)
				if best == nil || better(f, bf) {
					best, bf = nv, f
				}
			}
			return best
		})
	}
	minmax("min", func(a, b float64) bool { return a < b })
	minmax("max", func(a, b float64) bool { return a > b })
	aggregate("summarize", func(items []Value, n []float64) Value {
		t := NewTable()
		t.SetStr("count", int64(len(n)))
		if len(n) == 0 {
			t.SetStr("sum", int64(0))
			t.SetStr("mean", nil)
			t.SetStr("min", nil)
			t.SetStr("max", nil)
			return t
		}
		s, mn, mx := 0.0, n[0], n[0]
		for _, x := range n {
			s += x
			mn = math.Min(mn, x)
			mx = math.Max(mx, x)
		}
		t.SetStr("sum", s)
		t.SetStr("mean", s/float64(len(n)))
		t.SetStr("min", mn)
		t.SetStr("max", mx)
		return t
	})

	// ---------- listas e tabelas ----------
	def("range", func(vm *VM, a []Value) (Value, error) {
		start, err := argInt("range", a, 0, 0)
		if err != nil {
			return nil, err
		}
		var end, step int64 = 0, 1
		if len(a) >= 2 {
			if end, err = argInt("range", a, 1, 0); err != nil {
				return nil, err
			}
		} else {
			start, end = 0, start
		}
		if len(a) >= 3 {
			if step, err = argInt("range", a, 2, 1); err != nil || step == 0 {
				return nil, fmt.Errorf("range: passo inválido")
			}
		}
		if (end-start)/step > 10_000_000 {
			return nil, fmt.Errorf("range: grande demais")
		}
		var l []Value
		for i := start; (step > 0 && i < end) || (step < 0 && i > end); i += step {
			l = append(l, i)
		}
		return NewList(l...), nil
	})
	def("map", func(vm *VM, a []Value) (Value, error) {
		items, err := argList("map", a, 0)
		if err != nil {
			return nil, err
		}
		f, err := argFn("map", a, 1)
		if err != nil {
			return nil, err
		}
		out := make([]Value, len(items))
		for i, it := range items {
			if out[i], err = vm.Call(f, []Value{it}); err != nil {
				return nil, err
			}
		}
		return NewList(out...), nil
	})
	def("filter", func(vm *VM, a []Value) (Value, error) {
		items, err := argList("filter", a, 0)
		if err != nil {
			return nil, err
		}
		f, err := argFn("filter", a, 1)
		if err != nil {
			return nil, err
		}
		out := []Value{}
		for _, it := range items {
			r, err := vm.Call(f, []Value{it})
			if err != nil {
				return nil, err
			}
			if Truthy(r) {
				out = append(out, it)
			}
		}
		return NewList(out...), nil
	})
	def("find", func(vm *VM, a []Value) (Value, error) {
		items, err := argList("find", a, 0)
		if err != nil {
			return nil, err
		}
		f, err := argFn("find", a, 1)
		if err != nil {
			return nil, err
		}
		for _, it := range items {
			r, err := vm.Call(f, []Value{it})
			if err != nil {
				return nil, err
			}
			if Truthy(r) {
				return it, nil
			}
		}
		return nil, nil
	})
	def("reduce", func(vm *VM, a []Value) (Value, error) {
		items, err := argList("reduce", a, 0)
		if err != nil {
			return nil, err
		}
		f, err := argFn("reduce", a, 1)
		if err != nil {
			return nil, err
		}
		acc := arg(a, 2)
		for _, it := range items {
			if acc, err = vm.Call(f, []Value{acc, it}); err != nil {
				return nil, err
			}
		}
		return acc, nil
	})
	def("any", func(vm *VM, a []Value) (Value, error) {
		items, err := argList("any", a, 0)
		if err != nil {
			return nil, err
		}
		for _, it := range items {
			v := it
			if len(a) > 1 {
				if v, err = vm.Call(a[1], []Value{it}); err != nil {
					return nil, err
				}
			}
			if Truthy(v) {
				return true, nil
			}
		}
		return false, nil
	})
	def("all", func(vm *VM, a []Value) (Value, error) {
		items, err := argList("all", a, 0)
		if err != nil {
			return nil, err
		}
		for _, it := range items {
			v := it
			if len(a) > 1 {
				if v, err = vm.Call(a[1], []Value{it}); err != nil {
					return nil, err
				}
			}
			if !Truthy(v) {
				return false, nil
			}
		}
		return true, nil
	})
	sortImpl := func(vm *VM, name string, a []Value, keyIdx, dirIdx int) (Value, error) {
		items, err := argList(name, a, 0)
		if err != nil {
			return nil, err
		}
		out := append([]Value(nil), items...)
		key := func(v Value) (Value, error) { return v, nil }
		if keyIdx >= 0 {
			if key, err = keyFn(vm, name, a, keyIdx); err != nil {
				return nil, err
			}
		}
		keys := make(map[int]Value, len(out))
		idx := make([]Value, len(out))
		for i, it := range out {
			k, err := key(it)
			if err != nil {
				return nil, err
			}
			if n, ok := numVal(k); ok {
				if _, isStr := k.(string); !isStr {
					k = n
				}
			}
			keys[i] = k
			idx[i] = int64(i)
		}
		err = SortValues(idx, func(x, y Value) (bool, error) {
			c, err := Compare(keys[int(x.(int64))], keys[int(y.(int64))])
			return c < 0, err
		})
		if err != nil {
			return nil, fmt.Errorf("%s: %v", name, err)
		}
		res := make([]Value, len(out))
		for i, ix := range idx {
			res[i] = out[int(ix.(int64))]
		}
		if dirIdx >= 0 && strings.ToLower(Str(arg(a, dirIdx))) == "desc" {
			for i, j := 0, len(res)-1; i < j; i, j = i+1, j-1 {
				res[i], res[j] = res[j], res[i]
			}
		}
		return NewList(res...), nil
	}
	def("sort", func(vm *VM, a []Value) (Value, error) { return sortImpl(vm, "sort", a, -1, 1) })
	def("sort_by", func(vm *VM, a []Value) (Value, error) { return sortImpl(vm, "sort_by", a, 1, 2) })
	def("group_by", func(vm *VM, a []Value) (Value, error) {
		items, err := argList("group_by", a, 0)
		if err != nil {
			return nil, err
		}
		key, err := keyFn(vm, "group_by", a, 1)
		if err != nil {
			return nil, err
		}
		t := NewTable()
		for _, it := range items {
			k, err := key(it)
			if err != nil {
				return nil, err
			}
			ks := Str(k)
			cur, ok := t.Get(ks)
			if !ok {
				cur = NewList()
				t.SetStr(ks, cur)
			}
			cur.(*List).Items = append(cur.(*List).Items, it)
		}
		return t, nil
	})
	def("count_by", func(vm *VM, a []Value) (Value, error) {
		items, err := argList("count_by", a, 0)
		if err != nil {
			return nil, err
		}
		key, err := keyFn(vm, "count_by", a, 1)
		if err != nil {
			return nil, err
		}
		t := NewTable()
		for _, it := range items {
			k, err := key(it)
			if err != nil {
				return nil, err
			}
			ks := Str(k)
			cur, _ := t.Get(ks)
			n, _ := cur.(int64)
			t.SetStr(ks, n+1)
		}
		return t, nil
	})
	def("where", func(vm *VM, a []Value) (Value, error) {
		items, err := argList("where", a, 0)
		if err != nil {
			return nil, err
		}
		field := Str(arg(a, 1))
		out := []Value{}
		for _, it := range items {
			if t, ok := it.(*Table); ok {
				v, _ := t.Get(field)
				if Equal(v, arg(a, 2)) {
					out = append(out, it)
				}
			}
		}
		return NewList(out...), nil
	})
	def("pluck", func(vm *VM, a []Value) (Value, error) {
		items, err := argList("pluck", a, 0)
		if err != nil {
			return nil, err
		}
		field := Str(arg(a, 1))
		out := make([]Value, len(items))
		for i, it := range items {
			if t, ok := it.(*Table); ok {
				out[i], _ = t.Get(field)
			}
		}
		return NewList(out...), nil
	})
	slice := func(name string, f func(items []Value, n int) []Value) {
		def(name, func(vm *VM, a []Value) (Value, error) {
			items, err := argList(name, a, 0)
			if err != nil {
				return nil, err
			}
			n, err := argInt(name, a, 1, 1)
			if err != nil {
				return nil, err
			}
			if n < 0 {
				n = 0
			}
			if int(n) > len(items) {
				n = int64(len(items))
			}
			return NewList(append([]Value(nil), f(items, int(n))...)...), nil
		})
	}
	slice("take", func(items []Value, n int) []Value { return items[:n] })
	slice("skip", func(items []Value, n int) []Value { return items[n:] })
	def("slice", func(vm *VM, a []Value) (Value, error) {
		items, err := argList("slice", a, 0)
		if err != nil {
			return nil, err
		}
		n := int64(len(items))
		s, err := argInt("slice", a, 1, 0)
		if err != nil {
			return nil, err
		}
		e, err := argInt("slice", a, 2, n)
		if err != nil {
			return nil, err
		}
		if s < 0 {
			s += n
		}
		if e < 0 {
			e += n
		}
		s = int64(math.Max(0, math.Min(float64(s), float64(n))))
		e = int64(math.Max(float64(s), math.Min(float64(e), float64(n))))
		return NewList(append([]Value(nil), items[s:e]...)...), nil
	})
	def("first", func(vm *VM, a []Value) (Value, error) {
		items, err := argList("first", a, 0)
		if err != nil || len(items) == 0 {
			return nil, err
		}
		return items[0], nil
	})
	def("last", func(vm *VM, a []Value) (Value, error) {
		items, err := argList("last", a, 0)
		if err != nil || len(items) == 0 {
			return nil, err
		}
		return items[len(items)-1], nil
	})
	def("reverse", func(vm *VM, a []Value) (Value, error) {
		if s, ok := arg(a, 0).(string); ok {
			rs := []rune(s)
			for i, j := 0, len(rs)-1; i < j; i, j = i+1, j-1 {
				rs[i], rs[j] = rs[j], rs[i]
			}
			return string(rs), nil
		}
		items, err := argList("reverse", a, 0)
		if err != nil {
			return nil, err
		}
		out := make([]Value, len(items))
		for i, it := range items {
			out[len(items)-1-i] = it
		}
		return NewList(out...), nil
	})
	def("push", func(vm *VM, a []Value) (Value, error) {
		l, ok := arg(a, 0).(*List)
		if !ok {
			return nil, argErr("push", 0, "uma lista", arg(a, 0))
		}
		l.Items = append(l.Items, a[1:]...)
		return l, nil
	})
	def("pop", func(vm *VM, a []Value) (Value, error) {
		l, ok := arg(a, 0).(*List)
		if !ok {
			return nil, argErr("pop", 0, "uma lista", arg(a, 0))
		}
		if len(l.Items) == 0 {
			return nil, nil
		}
		v := l.Items[len(l.Items)-1]
		l.Items = l.Items[:len(l.Items)-1]
		return v, nil
	})
	def("insert", func(vm *VM, a []Value) (Value, error) {
		l, ok := arg(a, 0).(*List)
		if !ok {
			return nil, argErr("insert", 0, "uma lista", arg(a, 0))
		}
		i, err := argInt("insert", a, 1, 0)
		if err != nil {
			return nil, err
		}
		if i < 0 {
			i += int64(len(l.Items))
		}
		if i < 0 || i > int64(len(l.Items)) {
			return nil, fmt.Errorf("insert: posição %d fora da lista", i)
		}
		l.Items = append(l.Items, nil)
		copy(l.Items[i+1:], l.Items[i:])
		l.Items[i] = arg(a, 2)
		return l, nil
	})
	def("remove", func(vm *VM, a []Value) (Value, error) {
		switch c := arg(a, 0).(type) {
		case *List:
			i, ok := listIndex(len(c.Items), arg(a, 1))
			if !ok || i < 0 || i >= len(c.Items) {
				return nil, nil
			}
			v := c.Items[i]
			c.Items = append(c.Items[:i], c.Items[i+1:]...)
			return v, nil
		case *Table:
			v, _ := c.Get(arg(a, 1))
			c.Delete(arg(a, 1))
			return v, nil
		}
		return nil, argErr("remove", 0, "uma lista ou tabela", arg(a, 0))
	})
	def("unique", func(vm *VM, a []Value) (Value, error) {
		items, err := argList("unique", a, 0)
		if err != nil {
			return nil, err
		}
		out := []Value{}
		for _, it := range items {
			dup := false
			for _, o := range out {
				if Equal(o, it) {
					dup = true
					break
				}
			}
			if !dup {
				out = append(out, it)
			}
		}
		return NewList(out...), nil
	})
	def("flatten", func(vm *VM, a []Value) (Value, error) {
		items, err := argList("flatten", a, 0)
		if err != nil {
			return nil, err
		}
		out := []Value{}
		for _, it := range items {
			if l, ok := it.(*List); ok {
				out = append(out, l.Items...)
			} else {
				out = append(out, it)
			}
		}
		return NewList(out...), nil
	})
	def("zip", func(vm *VM, a []Value) (Value, error) {
		x, err := argList("zip", a, 0)
		if err != nil {
			return nil, err
		}
		y, err := argList("zip", a, 1)
		if err != nil {
			return nil, err
		}
		n := len(x)
		if len(y) < n {
			n = len(y)
		}
		out := make([]Value, n)
		for i := 0; i < n; i++ {
			out[i] = NewList(x[i], y[i])
		}
		return NewList(out...), nil
	})
	def("enumerate", func(vm *VM, a []Value) (Value, error) {
		items, err := argList("enumerate", a, 0)
		if err != nil {
			return nil, err
		}
		out := make([]Value, len(items))
		for i, it := range items {
			out[i] = NewList(int64(i), it)
		}
		return NewList(out...), nil
	})
	def("keys", func(vm *VM, a []Value) (Value, error) {
		switch t := arg(a, 0).(type) {
		case *Table:
			return NewList(t.Keys()...), nil
		case *Module:
			out := []Value{}
			for _, k := range t.order {
				out = append(out, k)
			}
			return NewList(out...), nil
		}
		return NewList(), nil
	})
	def("values", func(vm *VM, a []Value) (Value, error) {
		if t, ok := arg(a, 0).(*Table); ok {
			return NewList(t.Values()...), nil
		}
		return NewList(), nil
	})
	def("has", func(vm *VM, a []Value) (Value, error) {
		switch t := arg(a, 0).(type) {
		case *Table:
			_, ok := t.Get(arg(a, 1))
			return ok, nil
		case *Module:
			_, ok := t.Globals[Str(arg(a, 1))]
			return ok, nil
		}
		return false, nil
	})
	def("merge", func(vm *VM, a []Value) (Value, error) {
		t := NewTable()
		for i := range a {
			if s, ok := a[i].(*Table); ok {
				for j := 0; j < s.Len(); j++ {
					k, v := s.Entry(j)
					_ = t.Set(k, v)
				}
			}
		}
		return t, nil
	})

	// ---------- datas ----------
	def("now", func(vm *VM, a []Value) (Value, error) { return time.Now().Format("2006-01-02 15:04:05"), nil })
	def("date", func(vm *VM, a []Value) (Value, error) {
		t, err := parseTime(arg(a, 0))
		if err != nil {
			return nil, err
		}
		f := "d/m/Y"
		if len(a) > 1 {
			f = Str(a[1])
		}
		return phpDate(t, f), nil
	})

	// ---------- dados ----------
	def("load_csv", func(vm *VM, a []Value) (Value, error) {
		path, err := argStr("load_csv", a, 0)
		if err != nil {
			return nil, err
		}
		data, err := readFile(path)
		if err != nil {
			return nil, err
		}
		first := strings.SplitN(data, "\n", 2)[0]
		r := csv.NewReader(strings.NewReader(strings.TrimPrefix(data, "\ufeff")))
		if strings.Count(first, ";") > strings.Count(first, ",") {
			r.Comma = ';'
		}
		r.FieldsPerRecord = -1
		rows, err := r.ReadAll()
		if err != nil {
			return nil, fmt.Errorf("load_csv: %v", err)
		}
		out := []Value{}
		if len(rows) == 0 {
			return NewList(), nil
		}
		head := rows[0]
		for _, row := range rows[1:] {
			t := NewTable()
			for i, h := range head {
				var cell Value = ""
				if i < len(row) {
					s := strings.TrimSpace(row[i])
					cell = s
					if n, ok := numVal(s); ok && s != "" {
						cell = n
					}
				}
				t.SetStr(strings.TrimSpace(h), cell)
			}
			out = append(out, t)
		}
		return NewList(out...), nil
	})
	return b
}
