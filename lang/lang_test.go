package lang

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Cada testdata/X.nep é executado e a saída comparada com testdata/X.out.
func TestGolden(t *testing.T) {
	files, _ := filepath.Glob("../testdata/*.nep")
	if len(files) == 0 {
		t.Fatal("nenhum programa em testdata")
	}
	t.Setenv("NEP_TMP", t.TempDir())
	for _, f := range files {
		name := strings.TrimSuffix(filepath.Base(f), ".nep")
		t.Run(name, func(t *testing.T) {
			want, err := os.ReadFile(strings.TrimSuffix(f, ".nep") + ".out")
			if err != nil {
				t.Skip("sem .out")
			}
			var out bytes.Buffer
			vm := NewVM()
			vm.Out = &out
			if err := vm.RunFile(f); err != nil {
				out.WriteString(err.Error() + "\n")
			}
			if got := out.String(); got != string(want) {
				t.Errorf("saída diferente\n--- esperado\n%s\n--- obtido\n%s", want, got)
			}
		})
	}
}

func run(t *testing.T, src string) string {
	t.Helper()
	var out bytes.Buffer
	vm := NewVM()
	vm.Out = &out
	if _, err := vm.RunSource(src, "t.nep", NewModule("t", "t.nep")); err != nil {
		return "ERRO: " + err.Error()
	}
	return strings.TrimSpace(out.String())
}

func TestCasos(t *testing.T) {
	casos := []struct{ src, want string }{
		{`print(1 + 2 * 3)`, "7"},
		{`print((1 + 2) * 3)`, "9"},
		{`print(2 ** 3)`, "ERRO"},
		{`let a = [1, 2]
let b = a
push(b, 3)
print(a)`, "[1, 2, 3]"},
		{`fn f(x) do
  let g = fn(y) do x + y end
  g
end
print(f(10)(5))`, "15"},
		{`let n = 0
fn inc() do n += 1 end
inc()
inc()
print(n)`, "2"},
		{`x = 1`, "ERRO: Erro: variável não declarada: x"},
		{`let x = 1
let x = 2
print(x)`, "2"},
		{`do`, "ERRO"},
		{`print("a" + 1 + 2)`, "a12"},
		{`print(10 / 4, 10 // 4, -10 // 4, 10 % -3)`, "2.5 2 -3 -2"},
		{`print([3, 1, 2] |> sort |> reverse)`, "[3, 2, 1]"},
		{`let t = {}
t[1] = "um"
t[1.0] = "um de novo"
print(t, len(t))`, `{1: "um de novo"} 1`},
		{`for x in nil do print(x) end
print("ok")`, "ok"},
		{`let fs = []
let i = 0
while i < 3 do
  let j = i
  push(fs, fn() do j end)
  i += 1
end
print(map(fs, fn(f) do f() end))`, "[0, 1, 2]"},
		{`print(len("ação"), "ação"[1], upper("ação"))`, "4 ç AÇÃO"},
		{`print(1 < 2 and 2 < 3, not (1 == 1))`, "true false"},
		{`print(contains([1, 2], 2.0), contains("abc", "bc"), contains({a: 1}, "a"))`, "true true true"},
	}
	for _, c := range casos {
		got := run(t, c.src)
		if c.want == "ERRO" {
			if !strings.HasPrefix(got, "ERRO") {
				t.Errorf("%q: esperava erro, veio %q", c.src, got)
			}
			continue
		}
		if !strings.HasPrefix(got, c.want) {
			t.Errorf("%q:\n  esperado %q\n  obtido   %q", c.src, c.want, got)
		}
	}
}
