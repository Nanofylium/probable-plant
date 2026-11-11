package lang

import (
	"fmt"
	"math"
)

// Table é um dicionário que preserva a ordem de inserção.
//
// As chaves 1, 2, 3... inseridas em ordem logo no começo (o caso das listas do
// Lua) ficam numa parte de array, sem passar pelo mapa. A ordem continua a de
// inserção: primeiro a parte de array, depois as demais chaves.
type Table struct {
	arr   []Value // valores das chaves 1..len(arr)
	keys  []Value
	vals  []Value
	index map[Value]int
	meta  *Table // metatabela (setmetatable)
	nils  int    // chaves apagadas pelo Lua que ainda ocupam lugar
	udata any    // dado do Go por trás da tabela (ex.: arquivo do io.open)
}

func NewTable() *Table { return &Table{} }

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
	case *Table, *List, *Closure, *Native, *Module, *Coroutine:
		return v, nil
	case HTML:
		return string(v), nil
	}
	return nil, fmt.Errorf("chave de tabela inválida: %s (use texto, número ou bool)", TypeName(k))
}

func (t *Table) arrIdx(nk Value) int {
	if i, ok := nk.(int64); ok && i >= 1 && i <= int64(len(t.arr)) {
		return int(i - 1)
	}
	return -1
}

// canAppend diz se a chave nk entra no fim da parte de array.
func (t *Table) canAppend(nk Value) bool {
	i, ok := nk.(int64)
	return ok && len(t.keys) == 0 && i == int64(len(t.arr))+1
}

func (t *Table) Get(k Value) (Value, bool) {
	nk, err := normKey(k)
	if err != nil {
		return nil, false
	}
	if j := t.arrIdx(nk); j >= 0 {
		return t.arr[j], true
	}
	i, ok := t.index[nk]
	if !ok {
		return nil, false
	}
	return t.vals[i], true
}

func (t *Table) hashInsert(nk, v Value) {
	if t.index == nil {
		t.index = map[Value]int{}
	}
	t.index[nk] = len(t.keys)
	t.keys = append(t.keys, nk)
	t.vals = append(t.vals, v)
}

func (t *Table) Set(k, v Value) error {
	nk, err := normKey(k)
	if err != nil {
		return err
	}
	if j := t.arrIdx(nk); j >= 0 {
		t.arr[j] = v
		return nil
	}
	if i, ok := t.index[nk]; ok {
		t.vals[i] = v
		return nil
	}
	if t.canAppend(nk) {
		t.arr = append(t.arr, v)
		return nil
	}
	t.hashInsert(nk, v)
	return nil
}

func (t *Table) SetStr(k string, v Value) { _ = t.Set(k, v) }

// flatten passa a parte de array para o mapa (antes de apagar uma chave dela).
func (t *Table) flatten() {
	if len(t.arr) == 0 {
		return
	}
	n := len(t.arr)
	keys := make([]Value, 0, n+len(t.keys))
	vals := make([]Value, 0, n+len(t.keys))
	for i, v := range t.arr {
		keys = append(keys, int64(i+1))
		vals = append(vals, v)
	}
	t.keys = append(keys, t.keys...)
	t.vals = append(vals, t.vals...)
	t.arr = nil
	t.index = make(map[Value]int, len(t.keys))
	for i, k := range t.keys {
		t.index[k] = i
	}
}

func (t *Table) Delete(k Value) bool {
	nk, err := normKey(k)
	if err != nil {
		return false
	}
	if t.arrIdx(nk) >= 0 {
		t.flatten()
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

func (t *Table) Len() int { return len(t.arr) + len(t.keys) }

func (t *Table) Keys() []Value {
	out := make([]Value, 0, t.Len())
	for i := range t.arr {
		out = append(out, int64(i+1))
	}
	return append(out, t.keys...)
}

func (t *Table) Values() []Value {
	out := make([]Value, 0, t.Len())
	out = append(out, t.arr...)
	return append(out, t.vals...)
}

func (t *Table) Entry(i int) (Value, Value) {
	if i < len(t.arr) {
		return int64(i + 1), t.arr[i]
	}
	i -= len(t.arr)
	return t.keys[i], t.vals[i]
}

// ---------- acesso no estilo Lua ----------

// SetMeta define a metatabela da tabela (nil remove).
func (t *Table) SetMeta(m *Table) { t.meta = m }

// Meta devolve a metatabela da tabela.
func (t *Table) Meta() *Table { return t.meta }

func (t *Table) rawStr(k string) Value {
	if t == nil {
		return nil
	}
	if i, ok := t.index[k]; ok {
		return t.vals[i]
	}
	return nil
}

func (t *Table) getInt(i int64) Value {
	if i >= 1 && i <= int64(len(t.arr)) {
		return t.arr[i-1]
	}
	if j, ok := t.index[i]; ok {
		return t.vals[j]
	}
	return nil
}

// luaSet grava como o Lua: nil apaga a chave (sem mexer na ordem durante um pairs).
func (t *Table) luaSet(k, v Value) error {
	if k == nil {
		return fmt.Errorf("índice da tabela é nil")
	}
	if f, ok := k.(float64); ok && math.IsNaN(f) {
		return fmt.Errorf("índice da tabela é NaN")
	}
	nk, err := normKey(k)
	if err != nil {
		return err
	}
	if j := t.arrIdx(nk); j >= 0 {
		old := t.arr[j]
		if old == nil && v != nil && t.nils > 0 {
			t.nils--
		} else if old != nil && v == nil {
			t.nils++
		}
		t.arr[j] = v
		return nil
	}
	if i, ok := t.index[nk]; ok {
		old := t.vals[i]
		if old == nil && v != nil && t.nils > 0 {
			t.nils--
		} else if old != nil && v == nil {
			t.nils++
		}
		t.vals[i] = v
		return nil
	}
	if v == nil {
		return nil
	}
	if t.canAppend(nk) {
		t.arr = append(t.arr, v)
		return nil
	}
	if t.nils > 16 && t.nils*2 > t.Len() {
		t.compact()
		if t.canAppend(nk) {
			t.arr = append(t.arr, v)
			return nil
		}
	}
	t.hashInsert(nk, v)
	return nil
}

// compact descarta as chaves apagadas (só chamado ao criar uma chave nova,
// o que o Lua não permite durante um pairs).
func (t *Table) compact() {
	for len(t.arr) > 0 && t.arr[len(t.arr)-1] == nil && len(t.keys) == 0 {
		t.arr = t.arr[:len(t.arr)-1]
	}
	keys, vals := t.keys[:0:0], t.vals[:0:0]
	for i, k := range t.keys {
		if t.vals[i] != nil {
			keys = append(keys, k)
			vals = append(vals, t.vals[i])
		}
	}
	t.keys, t.vals = keys, vals
	t.index = make(map[Value]int, len(keys))
	for i, k := range keys {
		t.index[k] = i
	}
	t.nils = 0
	for _, v := range t.arr {
		if v == nil {
			t.nils++
		}
	}
}

// border procura uma borda entre i (t[i] != nil ou i == 0) e j (t[j] == nil).
func (t *Table) border(i, j int64) int64 {
	for j-i > 1 {
		m := i + (j-i)/2
		if t.getInt(m) == nil {
			j = m
		} else {
			i = m
		}
	}
	return i
}

// luaLen devolve uma "borda" da parte de lista (o operador # do Lua).
func (t *Table) luaLen() int64 {
	if n := int64(len(t.arr)); n > 0 {
		if t.arr[n-1] == nil {
			return t.border(0, n)
		}
		if t.getInt(n+1) == nil {
			return n
		}
		return t.unbound(n)
	}
	if t.getInt(1) == nil {
		return 0
	}
	n := int64(len(t.keys) - t.nils)
	if n < 1 {
		n = 1
	}
	if t.getInt(n) != nil {
		if t.getInt(n+1) == nil {
			return n
		}
		return t.unbound(n)
	}
	return t.border(0, n)
}

func (t *Table) unbound(i int64) int64 {
	j := i + 1
	for t.getInt(j) != nil {
		i = j
		if j > math.MaxInt64/2 {
			return i
		}
		j *= 2
	}
	return t.border(i, j)
}

// luaNext implementa next(t, k): pula as chaves apagadas.
func (t *Table) luaNext(k Value) (Value, Value, error) {
	i := 0
	if k != nil {
		nk, err := normKey(k)
		if err != nil {
			return nil, nil, err
		}
		if j := t.arrIdx(nk); j >= 0 {
			i = j + 1
		} else if idx, ok := t.index[nk]; ok {
			i = len(t.arr) + idx + 1
		} else {
			return nil, nil, fmt.Errorf("chave inválida para 'next'")
		}
	}
	for ; i < len(t.arr); i++ {
		if t.arr[i] != nil {
			return int64(i + 1), t.arr[i], nil
		}
	}
	for h := i - len(t.arr); h < len(t.keys); h++ {
		if t.vals[h] != nil {
			return t.keys[h], t.vals[h], nil
		}
	}
	return nil, nil, nil
}
