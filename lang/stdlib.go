package lang

import (
	"bytes"
	"embed"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"math/rand"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"
)

//go:embed std/*.nep
var stdSources embed.FS

// stdSource devolve o código de um módulo padrão escrito em Nepenthes.
func stdSource(name string) (string, bool) {
	b, err := stdSources.ReadFile("std/" + name + ".nep")
	if err != nil {
		return "", false
	}
	return string(b), true
}

func readFile(path string) (string, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("não consegui ler %s: %v", path, simpleErr(err))
	}
	return string(b), nil
}

func simpleErr(err error) string {
	switch {
	case os.IsNotExist(err):
		return "arquivo não existe"
	case os.IsPermission(err):
		return "sem permissão"
	}
	return err.Error()
}

func modOf(name string, fns map[string]Value) *Module {
	m := NewModule(name, "<std>")
	keys := make([]string, 0, len(fns))
	for k := range fns {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		m.Define(k, fns[k])
	}
	return m
}

func stdModules() map[string]func(vm *VM) *Module {
	return map[string]func(vm *VM) *Module{
		"math": mathModule,
		"fs":   fsModule,
		"json": jsonModule,
		"os":   osModule,
		"time": timeModule,
		"http": httpModule,
	}
}

// ---------- math ----------
func mathModule(vm *VM) *Module {
	f1 := func(name string, f func(float64) float64) *Native {
		return nat("math."+name, func(vm *VM, a []Value) (Value, error) {
			x, err := argNum("math."+name, a, 0)
			if err != nil {
				return nil, err
			}
			return f(x), nil
		})
	}
	rng := rand.New(rand.NewSource(time.Now().UnixNano()))
	return modOf("math", map[string]Value{
		"pi": math.Pi, "e": math.E, "inf": math.Inf(1),
		"sqrt": f1("sqrt", math.Sqrt), "sin": f1("sin", math.Sin), "cos": f1("cos", math.Cos), "tan": f1("tan", math.Tan),
		"log": f1("log", math.Log), "log10": f1("log10", math.Log10), "exp": f1("exp", math.Exp),
		"pow": nat("math.pow", func(vm *VM, a []Value) (Value, error) {
			x, err := argNum("math.pow", a, 0)
			if err != nil {
				return nil, err
			}
			y, err := argNum("math.pow", a, 1)
			if err != nil {
				return nil, err
			}
			if xi, ok := a[0].(int64); ok {
				if yi, ok := a[1].(int64); ok && yi >= 0 && yi < 63 {
					r := int64(1)
					for i := int64(0); i < yi; i++ {
						r *= xi
					}
					return r, nil
				}
			}
			return math.Pow(x, y), nil
		}),
		"random": nat("math.random", func(vm *VM, a []Value) (Value, error) { return rng.Float64(), nil }),
		"random_int": nat("math.random_int", func(vm *VM, a []Value) (Value, error) {
			lo, err := argInt("math.random_int", a, 0, 0)
			if err != nil {
				return nil, err
			}
			hi, err := argInt("math.random_int", a, 1, 0)
			if err != nil {
				return nil, err
			}
			if hi < lo {
				return nil, fmt.Errorf("math.random_int: o máximo é menor que o mínimo")
			}
			return lo + rng.Int63n(hi-lo+1), nil
		}),
		"choice": nat("math.choice", func(vm *VM, a []Value) (Value, error) {
			items, err := argList("math.choice", a, 0)
			if err != nil || len(items) == 0 {
				return nil, err
			}
			return items[rng.Intn(len(items))], nil
		}),
		"shuffle": nat("math.shuffle", func(vm *VM, a []Value) (Value, error) {
			items, err := argList("math.shuffle", a, 0)
			if err != nil {
				return nil, err
			}
			out := append([]Value(nil), items...)
			rng.Shuffle(len(out), func(i, j int) { out[i], out[j] = out[j], out[i] })
			return NewList(out...), nil
		}),
		"seed": nat("math.seed", func(vm *VM, a []Value) (Value, error) {
			s, err := argInt("math.seed", a, 0, 0)
			rng.Seed(s)
			return nil, err
		}),
	})
}

// ---------- fs ----------
func fsModule(vm *VM) *Module {
	return modOf("fs", map[string]Value{
		"read": nat("fs.read", func(vm *VM, a []Value) (Value, error) {
			p, err := argStr("fs.read", a, 0)
			if err != nil {
				return nil, err
			}
			return readFile(p)
		}),
		"write": nat("fs.write", func(vm *VM, a []Value) (Value, error) {
			p, err := argStr("fs.write", a, 0)
			if err != nil {
				return nil, err
			}
			if err := os.WriteFile(p, []byte(Str(arg(a, 1))), 0o644); err != nil {
				return nil, fmt.Errorf("não consegui gravar %s: %v", p, simpleErr(err))
			}
			return true, nil
		}),
		"append": nat("fs.append", func(vm *VM, a []Value) (Value, error) {
			p, err := argStr("fs.append", a, 0)
			if err != nil {
				return nil, err
			}
			f, err := os.OpenFile(p, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
			if err != nil {
				return nil, fmt.Errorf("não consegui abrir %s: %v", p, simpleErr(err))
			}
			defer f.Close()
			_, err = f.WriteString(Str(arg(a, 1)))
			return err == nil, err
		}),
		"exists": nat("fs.exists", func(vm *VM, a []Value) (Value, error) {
			_, err := os.Stat(Str(arg(a, 0)))
			return err == nil, nil
		}),
		"is_dir": nat("fs.is_dir", func(vm *VM, a []Value) (Value, error) {
			st, err := os.Stat(Str(arg(a, 0)))
			return err == nil && st.IsDir(), nil
		}),
		"list": nat("fs.list", func(vm *VM, a []Value) (Value, error) {
			dir := "."
			if len(a) > 0 {
				dir = Str(a[0])
			}
			ents, err := os.ReadDir(dir)
			if err != nil {
				return nil, fmt.Errorf("não consegui listar %s: %v", dir, simpleErr(err))
			}
			out := make([]Value, len(ents))
			for i, e := range ents {
				out[i] = e.Name()
			}
			return NewList(out...), nil
		}),
		"mkdir": nat("fs.mkdir", func(vm *VM, a []Value) (Value, error) {
			return true, os.MkdirAll(Str(arg(a, 0)), 0o755)
		}),
		"remove": nat("fs.remove", func(vm *VM, a []Value) (Value, error) {
			if err := os.Remove(Str(arg(a, 0))); err != nil {
				return false, nil
			}
			return true, nil
		}),
		"join": nat("fs.join", func(vm *VM, a []Value) (Value, error) {
			parts := make([]string, len(a))
			for i, x := range a {
				parts[i] = Str(x)
			}
			return filepath.Join(parts...), nil
		}),
		"lines": nat("fs.lines", func(vm *VM, a []Value) (Value, error) {
			s, err := readFile(Str(arg(a, 0)))
			if err != nil {
				return nil, err
			}
			s = strings.TrimSuffix(strings.ReplaceAll(s, "\r\n", "\n"), "\n")
			parts := strings.Split(s, "\n")
			out := make([]Value, len(parts))
			for i, p := range parts {
				out[i] = p
			}
			return NewList(out...), nil
		}),
	})
}

// ---------- json ----------
func ToJSON(v Value, indent string) (string, error) {
	var b strings.Builder
	var enc func(v Value, depth int) error
	nl := func(depth int) {
		if indent != "" {
			b.WriteByte('\n')
			b.WriteString(strings.Repeat(indent, depth))
		}
	}
	enc = func(v Value, depth int) error {
		if depth > 100 {
			return fmt.Errorf("json: estrutura profunda demais (referência circular?)")
		}
		switch x := v.(type) {
		case nil:
			b.WriteString("null")
		case bool:
			b.WriteString(strconv.FormatBool(x))
		case int64:
			b.WriteString(strconv.FormatInt(x, 10))
		case float64:
			if math.IsInf(x, 0) || math.IsNaN(x) {
				b.WriteString("null")
			} else {
				b.WriteString(strconv.FormatFloat(x, 'g', -1, 64))
			}
		case string:
			j, _ := json.Marshal(x)
			b.Write(j)
		case *List:
			if len(x.Items) == 0 {
				b.WriteString("[]")
				return nil
			}
			b.WriteByte('[')
			for i, it := range x.Items {
				if i > 0 {
					b.WriteByte(',')
				}
				nl(depth + 1)
				if err := enc(it, depth+1); err != nil {
					return err
				}
			}
			nl(depth)
			b.WriteByte(']')
		case *Table:
			if x.Len() == 0 {
				b.WriteString("{}")
				return nil
			}
			b.WriteByte('{')
			for i := 0; i < x.Len(); i++ {
				k, val := x.Entry(i)
				if i > 0 {
					b.WriteByte(',')
				}
				nl(depth + 1)
				j, _ := json.Marshal(Str(k))
				b.Write(j)
				b.WriteByte(':')
				if indent != "" {
					b.WriteByte(' ')
				}
				if err := enc(val, depth+1); err != nil {
					return err
				}
			}
			nl(depth)
			b.WriteByte('}')
		default:
			return fmt.Errorf("json: não sei converter %s", TypeName(v))
		}
		return nil
	}
	err := enc(v, 0)
	return b.String(), err
}

func FromJSON(s string) (Value, error) {
	dec := json.NewDecoder(strings.NewReader(s))
	dec.UseNumber()
	var read func() (Value, error)
	read = func() (Value, error) {
		tok, err := dec.Token()
		if err != nil {
			return nil, err
		}
		switch t := tok.(type) {
		case json.Delim:
			switch t {
			case '{':
				tb := NewTable()
				for dec.More() {
					kt, err := dec.Token()
					if err != nil {
						return nil, err
					}
					v, err := read()
					if err != nil {
						return nil, err
					}
					tb.SetStr(kt.(string), v)
				}
				_, err := dec.Token()
				return tb, err
			case '[':
				l := NewList()
				for dec.More() {
					v, err := read()
					if err != nil {
						return nil, err
					}
					l.Items = append(l.Items, v)
				}
				_, err := dec.Token()
				return l, err
			}
		case json.Number:
			if i, err := t.Int64(); err == nil {
				return i, nil
			}
			f, err := t.Float64()
			return f, err
		case string, bool, nil:
			return t, nil
		}
		return nil, fmt.Errorf("json inválido")
	}
	v, err := read()
	if err != nil {
		return nil, fmt.Errorf("json inválido: %v", err)
	}
	if _, err := dec.Token(); err != io.EOF {
		return nil, fmt.Errorf("json inválido: conteúdo sobrando depois do valor")
	}
	return v, nil
}

func jsonModule(vm *VM) *Module {
	return modOf("json", map[string]Value{
		"encode": nat("json.encode", func(vm *VM, a []Value) (Value, error) { return ToJSON(arg(a, 0), "") }),
		"pretty": nat("json.pretty", func(vm *VM, a []Value) (Value, error) { return ToJSON(arg(a, 0), "  ") }),
		"decode": nat("json.decode", func(vm *VM, a []Value) (Value, error) {
			s, err := argStr("json.decode", a, 0)
			if err != nil {
				return nil, err
			}
			return FromJSON(s)
		}),
	})
}

// ---------- os ----------
func osModule(vm *VM) *Module {
	args := make([]Value, len(vm.Args))
	for i, s := range vm.Args {
		args[i] = s
	}
	return modOf("os", map[string]Value{
		"args": NewList(args...),
		"platform": func() string {
			return strings.ToLower(goos())
		}(),
		"env": nat("os.env", func(vm *VM, a []Value) (Value, error) {
			v, ok := os.LookupEnv(Str(arg(a, 0)))
			if !ok {
				return arg(a, 1), nil
			}
			return v, nil
		}),
		"exit": nat("os.exit", func(vm *VM, a []Value) (Value, error) {
			code, _ := argInt("os.exit", a, 0, 0)
			if f, ok := vm.Out.(interface{ Flush() error }); ok {
				_ = f.Flush()
			}
			os.Exit(int(code))
			return nil, nil
		}),
		"cwd": nat("os.cwd", func(vm *VM, a []Value) (Value, error) {
			d, err := os.Getwd()
			return d, err
		}),
	})
}

// ---------- time ----------
func timeModule(vm *VM) *Module {
	return modOf("time", map[string]Value{
		"unix": nat("time.unix", func(vm *VM, a []Value) (Value, error) {
			return float64(time.Now().UnixNano()) / 1e9, nil
		}),
		"sleep": nat("time.sleep", func(vm *VM, a []Value) (Value, error) {
			ms, err := argNum("time.sleep", a, 0)
			if err != nil {
				return nil, err
			}
			time.Sleep(time.Duration(ms * float64(time.Millisecond)))
			return nil, nil
		}),
		"format": nat("time.format", func(vm *VM, a []Value) (Value, error) {
			t, err := parseTime(arg(a, 0))
			if err != nil {
				return nil, err
			}
			return phpDate(t, Str(arg(a, 1))), nil
		}),
		"diff_days": nat("time.diff_days", func(vm *VM, a []Value) (Value, error) {
			t1, err := parseTime(arg(a, 0))
			if err != nil {
				return nil, err
			}
			t2, err := parseTime(arg(a, 1))
			if err != nil {
				return nil, err
			}
			return int64(math.Round(t2.Sub(t1).Hours() / 24)), nil
		}),
		"add_days": nat("time.add_days", func(vm *VM, a []Value) (Value, error) {
			t, err := parseTime(arg(a, 0))
			if err != nil {
				return nil, err
			}
			n, err := argInt("time.add_days", a, 1, 0)
			if err != nil {
				return nil, err
			}
			return t.AddDate(0, 0, int(n)).Format("2006-01-02"), nil
		}),
	})
}

// ---------- http ----------
func httpModule(vm *VM) *Module {
	client := &http.Client{Timeout: 30 * time.Second}
	do := func(name, method string) *Native {
		return nat("http."+name, func(vm *VM, a []Value) (Value, error) {
			url, err := argStr("http."+name, a, 0)
			if err != nil {
				return nil, err
			}
			var body io.Reader
			hdrIdx := 1
			if method == "POST" || method == "PUT" {
				hdrIdx = 2
				switch b := arg(a, 1).(type) {
				case *Table, *List:
					j, err := ToJSON(b, "")
					if err != nil {
						return nil, err
					}
					body = bytes.NewBufferString(j)
				case nil:
				default:
					body = bytes.NewBufferString(Str(b))
				}
			}
			req, err := http.NewRequest(method, url, body)
			if err != nil {
				return nil, fmt.Errorf("http.%s: endereço inválido: %v", name, err)
			}
			req.Header.Set("User-Agent", "nepenthes/"+Version)
			if _, isJSON := arg(a, 1).(*Table); isJSON && body != nil {
				req.Header.Set("Content-Type", "application/json")
			}
			if h, ok := arg(a, hdrIdx).(*Table); ok {
				for i := 0; i < h.Len(); i++ {
					k, v := h.Entry(i)
					req.Header.Set(Str(k), Str(v))
				}
			}
			resp, err := client.Do(req)
			if err != nil {
				return nil, fmt.Errorf("http.%s: %v", name, err)
			}
			defer resp.Body.Close()
			data, err := io.ReadAll(io.LimitReader(resp.Body, 50<<20))
			if err != nil {
				return nil, err
			}
			t := NewTable()
			t.SetStr("status", int64(resp.StatusCode))
			t.SetStr("ok", resp.StatusCode >= 200 && resp.StatusCode < 300)
			t.SetStr("body", string(data))
			hd := NewTable()
			for k := range resp.Header {
				hd.SetStr(strings.ToLower(k), resp.Header.Get(k))
			}
			t.SetStr("headers", hd)
			return t, nil
		})
	}
	return modOf("http", map[string]Value{"get": do("get", "GET"), "post": do("post", "POST"), "put": do("put", "PUT"), "delete": do("delete", "DELETE")})
}
