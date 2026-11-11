package lang

import (
	"fmt"
	"runtime"
	"weak"
)

// Corrotinas (Lua e Nepenthes). Cada corrotina roda numa VM própria, numa
// goroutine; só uma delas anda de cada vez, passando a vez por canais.

const (
	coStack  = 8192
	coFrames = 600
)

// Coroutine é o valor que coroutine.create devolve.
type Coroutine struct{ st *coState }

type coState struct {
	fn      Value
	vm      *VM
	status  string // suspended, running, normal, dead
	started bool
	resume  chan coMsg
	yield   chan coMsg
	self    weak.Pointer[Coroutine]
}

type coMsg struct {
	vals []Value
	err  error
	done bool
	kill bool
}

type coKilled struct{}

func (coKilled) String() string { return "corrotina encerrada" }

// NewCoroutine cria uma corrotina que vai rodar fn.
func (vm *VM) NewCoroutine(fn Value) *Coroutine {
	st := &coState{fn: fn, status: "suspended", resume: make(chan coMsg, 1), yield: make(chan coMsg, 1)}
	co := &Coroutine{st: st}
	st.self = weak.Make(co)
	// corrotina esquecida no meio: libera a goroutine
	runtime.SetFinalizer(co, func(c *Coroutine) { c.st.kill() })
	return co
}

func (st *coState) kill() {
	if st.started && st.status == "suspended" {
		st.status = "dead"
		st.resume <- coMsg{kill: true}
	}
	st.status = "dead"
}

func (vm *VM) child() *VM {
	c := &VM{Out: vm.Out, In: vm.In, modules: vm.modules, loading: vm.loading, Builtins: vm.Builtins,
		stdmods: vm.stdmods, Args: vm.Args, MaxSteps: vm.MaxSteps, L: vm.lua()}
	c.stack = make([]Value, coStack)
	c.maxFrames = coFrames
	c.frames = make([]frame, 0, coFrames+1)
	return c
}

// Resume retoma a corrotina com args; devolve o que ela passou para yield ou devolveu.
func (vm *VM) Resume(co *Coroutine, args []Value) ([]Value, error) {
	st := co.st
	switch st.status {
	case "dead":
		return nil, fmt.Errorf("não dá para retomar uma corrotina que já terminou")
	case "running", "normal":
		return nil, fmt.Errorf("não dá para retomar uma corrotina que não está suspensa")
	}
	if vm.co != nil {
		vm.co.status = "normal"
	}
	st.status = "running"
	if !st.started {
		st.started = true
		st.vm = vm.child()
		st.vm.co = st
		go func(cvm *VM, fn Value, args []Value) {
			var msg coMsg
			defer func() {
				if r := recover(); r != nil {
					msg = coMsg{err: fmt.Errorf("erro interno na corrotina: %v", r)}
				}
				msg.done = true
				st.yield <- msg
			}()
			r, err := cvm.CallMulti(fn, args)
			if err == nil {
				msg.vals = Values(r)
				if r == nil {
					msg.vals = nil
				}
			}
			msg.err = err
		}(st.vm, st.fn, append([]Value(nil), args...))
	} else {
		st.resume <- coMsg{vals: append([]Value(nil), args...)}
	}
	msg := <-st.yield
	if vm.co != nil {
		vm.co.status = "running"
	}
	if msg.done {
		st.status = "dead"
		st.vm = nil
		st.fn = nil
	} else {
		st.status = "suspended"
	}
	return msg.vals, msg.err
}

// Yield suspende a corrotina atual e devolve os valores de quem a retomar.
func (vm *VM) Yield(vals []Value) ([]Value, error) {
	st := vm.co
	if st == nil {
		return nil, fmt.Errorf("yield fora de uma corrotina")
	}
	st.yield <- coMsg{vals: append([]Value(nil), vals...)}
	msg := <-st.resume
	if msg.kill {
		return nil, &Halt{Reason: coKilled{}}
	}
	return msg.vals, nil
}

func (vm *VM) coStatus(co *Coroutine) string {
	if co.st == vm.co {
		return "running"
	}
	return co.st.status
}

// nepValues entrega vários valores ao Nepenthes: nenhum vira nil, um vira ele
// mesmo, vários viram uma lista.
func nepValues(vals []Value) Value {
	switch len(vals) {
	case 0:
		return nil
	case 1:
		return vals[0]
	}
	return NewList(append([]Value(nil), vals...)...)
}

func argCo(fn string, a []Value) (*Coroutine, error) {
	if co, ok := arg(a, 0).(*Coroutine); ok {
		return co, nil
	}
	return nil, argErr(fn, 0, "uma corrotina", arg(a, 0))
}

// coroutineLib monta a tabela coroutine (igual no Lua e no Nepenthes; no
// Nepenthes, resume/yield devolvem um valor só e erros viram exceções).
func coroutineLib() *Table {
	t := NewTable()
	def := func(name string, f nativeFn) { t.SetStr(name, nat("coroutine."+name, f)) }
	def("create", func(vm *VM, a []Value) (Value, error) {
		if !isCallable(arg(a, 0)) {
			return nil, argErr("coroutine.create", 0, "uma função", arg(a, 0))
		}
		return vm.NewCoroutine(a[0]), nil
	})
	def("resume", func(vm *VM, a []Value) (Value, error) {
		co, err := argCo("coroutine.resume", a)
		if err != nil {
			return nil, err
		}
		lua := vm.inLua()
		vals, err := vm.Resume(co, a[1:])
		if err != nil {
			if isHalt(err) {
				return nil, err
			}
			if lua {
				return Multi(false, vm.luaErrValue(err)), nil
			}
			return nil, err
		}
		if lua {
			return Multi(append([]Value{true}, vals...)...), nil
		}
		return nepValues(vals), nil
	})
	def("yield", func(vm *VM, a []Value) (Value, error) {
		lua := vm.inLua()
		vals, err := vm.Yield(a)
		if err != nil {
			return nil, err
		}
		if lua {
			return Multi(vals...), nil
		}
		return nepValues(vals), nil
	})
	def("status", func(vm *VM, a []Value) (Value, error) {
		co, err := argCo("coroutine.status", a)
		if err != nil {
			return nil, err
		}
		return vm.coStatus(co), nil
	})
	def("isyieldable", func(vm *VM, a []Value) (Value, error) { return vm.co != nil, nil })
	def("running", func(vm *VM, a []Value) (Value, error) {
		if vm.co == nil {
			return Multi(vm.lua().main, true), nil
		}
		return Multi(vm.co.self.Value(), false), nil
	})
	def("wrap", func(vm *VM, a []Value) (Value, error) {
		if !isCallable(arg(a, 0)) {
			return nil, argErr("coroutine.wrap", 0, "uma função", arg(a, 0))
		}
		co := vm.NewCoroutine(a[0])
		return nat("coroutine.wrap", func(vm *VM, a []Value) (Value, error) {
			lua := vm.inLua()
			vals, err := vm.Resume(co, a)
			if err != nil {
				if isHalt(err) {
					return nil, err
				}
				return nil, Throw(vm.luaErrValue(err))
			}
			if lua {
				return Multi(vals...), nil
			}
			return nepValues(vals), nil
		}), nil
	})
	def("close", func(vm *VM, a []Value) (Value, error) {
		co, err := argCo("coroutine.close", a)
		if err != nil {
			return nil, err
		}
		switch vm.coStatus(co) {
		case "running", "normal":
			return nil, fmt.Errorf("não dá para fechar uma corrotina em execução")
		}
		co.st.kill()
		return true, nil
	})
	return t
}
