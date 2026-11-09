package lang

import (
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"
)

// Value é qualquer valor Nepenthes:
// nil, bool, int64, float64, string, *List, *Table, *Closure, *Native, *Module.
type Value = any

type List struct{ Items []Value }

// Table é um dicionário que preserva a ordem de inserção.
type Table struct {
	keys  []Value
	vals  []Value
	index map[Value]int
}

func NewTable() *Table { return &Table{index: map[Value]int{}} }

func NewList(items ...Value) *List {
	if items == nil {
		items = []Value{}
	}
	return &List{Items: items}
}

// normKey converte uma chave para a forma canônica (float inteiro vira int).
func normKey(k Value) (Value, error) {
	switch v := k.(type) {
	case string, int64, bool:
		return v, nil
	case float64:
		if v == math.Trunc(v) && math.Abs(v) < 1<<53 {
			return int64(v), nil
		}
		return v, nil
	}
	return nil, fmt.Errorf("chave de tabela inválida: %s (use texto, número ou bool)", TypeName(k))
}

func (t *Table) Get(k Value) (Value, bool) {
	nk, err := normKey(k)
	if err != nil {
		return nil, false
	}
	i, ok := t.index[nk]
	if !ok {
		return nil, false
	}
	return t.vals[i], true
}

func (t *Table) Set(k, v Value) error {
	nk, err := normKey(k)
	if err != nil {
		return err
	}
	if i, ok := t.index[nk]; ok {
		t.vals[i] = v
		return nil
	}
	t.index[nk] = len(t.keys)
	t.keys = append(t.keys, nk)
	t.vals = append(t.vals, v)
	return nil
}

func (t *Table) SetStr(k string, v Value) { _ = t.Set(k, v) }

func (t *Table) Delete(k Value) bool {
	nk, err := normKey(k)
	if err != nil {
		return false
	}
	i, ok := t.index[nk]
	if !ok {
		return false
	}
	t.keys = append(t.keys[:i], t.keys[i+1:]...)
	t.vals = append(t.vals[:i], t.vals[i+1:]...)
	delete(t.index, nk)
	for j := i; j < len(t.keys); j++ {
		t.index[t.keys[j]] = j
	}
	return true
}

func (t *Table) Len() int                   { return len(t.keys) }
func (t *Table) Keys() []Value              { return append([]Value(nil), t.keys...) }
func (t *Table) Values() []Value            { return append([]Value(nil), t.vals...) }
func (t *Table) Entry(i int) (Value, Value) { return t.keys[i], t.vals[i] }

// Closure é uma função Nepenthes com as variáveis que ela capturou.
type Closure struct {
	Proto    *Proto
	Upvalues []*Upvalue
}

type Upvalue struct {
	loc    *Value
	closed Value
	slot   int
}

// Native é uma função escrita em Go.
type Native struct {
	Name string
	Fn   func(vm *VM, args []Value) (Value, error)
}

// Module é um arquivo .nep carregado ou um módulo da biblioteca padrão.
type Module struct {
	Name    string
	Path    string
	Globals map[string]Value
	order   []string
}

func NewModule(name, path string) *Module {
	return &Module{Name: name, Path: path, Globals: map[string]Value{}}
}

func (m *Module) Define(name string, v Value) {
	if _, ok := m.Globals[name]; !ok {
		m.order = append(m.order, name)
	}
	m.Globals[name] = v
}

// ---------- operações sobre valores ----------

func TypeName(v Value) string {
	switch x := v.(type) {
	case nil:
		return "nil"
	case bool:
		return "bool"
	case int64:
		return "int"
	case float64:
		return "float"
	case string:
		return "string"
	case *List:
		return "list"
	case *Table:
		return "table"
	case *Closure, *Native:
		return "fn"
	case *Module:
		return "module"
	default:
		return fmt.Sprintf("%T", x)
	}
}

func Truthy(v Value) bool {
	switch x := v.(type) {
	case nil:
		return false
	case bool:
		return x
	case int64:
		return x != 0
	case float64:
		return x != 0
	case string:
		return x != ""
	case *List:
		return len(x.Items) > 0
	case *Table:
		return x.Len() > 0
	}
	return true
}

func toFloat(v Value) (float64, bool) {
	switch x := v.(type) {
	case int64:
		return float64(x), true
	case float64:
		return x, true
	}
	return 0, false
}

func isNum(v Value) bool {
	switch v.(type) {
	case int64, float64:
		return true
	}
	return false
}

func Equal(a, b Value) bool { return equalDepth(a, b, 0) }

func equalDepth(a, b Value, d int) bool {
	if d > 100 {
		return false
	}
	if isNum(a) && isNum(b) {
		ai, aok := a.(int64)
		bi, bok := b.(int64)
		if aok && bok {
			return ai == bi
		}
		x, _ := toFloat(a)
		y, _ := toFloat(b)
		return x == y
	}
	switch x := a.(type) {
	case nil:
		return b == nil
	case bool:
		y, ok := b.(bool)
		return ok && x == y
	case string:
		y, ok := b.(string)
		return ok && x == y
	case *List:
		y, ok := b.(*List)
		if !ok || len(x.Items) != len(y.Items) {
			return false
		}
		if x == y {
			return true
		}
		for i := range x.Items {
			if !equalDepth(x.Items[i], y.Items[i], d+1) {
				return false
			}
		}
		return true
	case *Table:
		y, ok := b.(*Table)
		if !ok || x.Len() != y.Len() {
			return false
		}
		if x == y {
			return true
		}
		for i, k := range x.keys {
			w, ok := y.Get(k)
			if !ok || !equalDepth(x.vals[i], w, d+1) {
				return false
			}
		}
		return true
	}
	return a == b
}

// Compare devolve -1, 0 ou 1 para números ou textos.
func Compare(a, b Value) (int, error) {
	if isNum(a) && isNum(b) {
		ai, aok := a.(int64)
		bi, bok := b.(int64)
		if aok && bok {
			switch {
			case ai < bi:
				return -1, nil
			case ai > bi:
				return 1, nil
			}
			return 0, nil
		}
		x, _ := toFloat(a)
		y, _ := toFloat(b)
		switch {
		case x < y:
			return -1, nil
		case x > y:
			return 1, nil
		}
		return 0, nil
	}
	if x, ok := a.(string); ok {
		if y, ok := b.(string); ok {
			return strings.Compare(x, y), nil
		}
	}
	return 0, fmt.Errorf("não sei comparar %s com %s", TypeName(a), TypeName(b))
}

// Str é a forma de exibir um valor (print, str()).
func Str(v Value) string { return show(v, false, 0) }

// Repr é a forma "de código": textos entre aspas.
func Repr(v Value) string { return show(v, true, 0) }

func FormatFloat(f float64) string {
	switch {
	case math.IsInf(f, 1):
		return "inf"
	case math.IsInf(f, -1):
		return "-inf"
	case math.IsNaN(f):
		return "nan"
	}
	s := strconv.FormatFloat(f, 'g', -1, 64)
	if strings.ContainsAny(s, "e") {
		if math.Abs(f) < 1e16 && math.Abs(f) >= 1e-4 {
			s = strconv.FormatFloat(f, 'f', -1, 64)
		} else {
			return s
		}
	}
	if !strings.Contains(s, ".") {
		s += ".0"
	}
	return s
}

func quote(s string) string {
	var b strings.Builder
	b.WriteByte('"')
	for _, r := range s {
		switch r {
		case '"':
			b.WriteString(`\"`)
		case '\\':
			b.WriteString(`\\`)
		case '\n':
			b.WriteString(`\n`)
		case '\t':
			b.WriteString(`\t`)
		case '\r':
			b.WriteString(`\r`)
		default:
			b.WriteRune(r)
		}
	}
	b.WriteByte('"')
	return b.String()
}

func show(v Value, repr bool, depth int) string {
	if depth > 20 {
		return "…"
	}
	switch x := v.(type) {
	case nil:
		return "nil"
	case bool:
		if x {
			return "true"
		}
		return "false"
	case int64:
		return strconv.FormatInt(x, 10)
	case float64:
		return FormatFloat(x)
	case string:
		if repr {
			return quote(x)
		}
		return x
	case *List:
		parts := make([]string, len(x.Items))
		for i, it := range x.Items {
			parts[i] = show(it, true, depth+1)
		}
		return "[" + strings.Join(parts, ", ") + "]"
	case *Table:
		parts := make([]string, x.Len())
		for i, k := range x.keys {
			ks := ""
			if s, ok := k.(string); ok && validIdent(s) {
				ks = s
			} else {
				ks = show(k, true, depth+1)
			}
			parts[i] = ks + ": " + show(x.vals[i], true, depth+1)
		}
		return "{" + strings.Join(parts, ", ") + "}"
	case *Closure:
		if x.Proto.Name != "" {
			return "<fn " + x.Proto.Name + ">"
		}
		return "<fn anônima>"
	case *Native:
		return "<fn nativa " + x.Name + ">"
	case *Module:
		return "<módulo " + x.Name + ">"
	}
	return fmt.Sprint(v)
}

// SortValues ordena uma lista de valores comparáveis (números ou textos).
func SortValues(items []Value, less func(a, b Value) (bool, error)) error {
	var err error
	sort.SliceStable(items, func(i, j int) bool {
		if err != nil {
			return false
		}
		r, e := less(items[i], items[j])
		if e != nil {
			err = e
		}
		return r
	})
	return err
}
