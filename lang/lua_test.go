package lang

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Cada testdata/lua/X.lua roda no nep e a saída é comparada com X.out, que foi
// gerado pelo Lua 5.4 oficial. Assim o comportamento fica igual ao do Lua.
func TestLuaGolden(t *testing.T) {
	files, _ := filepath.Glob("../testdata/lua/*.lua")
	if len(files) == 0 {
		t.Fatal("nenhum programa em testdata/lua")
	}
	for _, f := range files {
		want, err := os.ReadFile(strings.TrimSuffix(f, ".lua") + ".out")
		if err != nil {
			continue // módulos usados por outros testes
		}
		name := strings.TrimSuffix(filepath.Base(f), ".lua")
		t.Run(name, func(t *testing.T) {
			var out bytes.Buffer
			vm := NewVM()
			vm.Out = &out
			wd, _ := os.Getwd()
			os.Chdir(filepath.Dir(f))
			err := vm.RunLuaFile(filepath.Base(f), nil)
			os.Chdir(wd)
			if err != nil {
				out.WriteString(err.Error() + "\n")
			}
			if got := out.String(); got != string(want) {
				t.Errorf("saída diferente do Lua 5.4\n--- esperado\n%s\n--- obtido\n%s", want, got)
			}
		})
	}
}

func runLua(t *testing.T, src string) string {
	t.Helper()
	var out bytes.Buffer
	vm := NewVM()
	vm.Out = &out
	if _, err := vm.RunLua(src, "t.lua"); err != nil {
		return "ERRO: " + err.Error()
	}
	return strings.TrimSpace(out.String())
}

func TestLuaErros(t *testing.T) {
	casos := []struct{ src, want string }{
		{"local t = nil; print(t.x)", "tentou indexar um valor nil"},
		{"print(1 + {})", "aritmética"},
		{"print(#5)", "tamanho"},
		{"print({} < {})", "comparar"},
		{"local x = 1 x()", "chamar um valor number"},
		{"goto fim", "rótulo visível"},
		{"do local a goto f end local b ::f:: print(b)", ""},
		{"for i = 1, 10, 0 do end", "passo do 'for' é zero"},
		{"print(('x'):rep(1e10))", "grande demais"},
		{"error('alto')", "t.lua:1: alto"},
		{"local function f() return ... end", "'...'"},
		{"x = = 1", "erro de sintaxe"},
		{"print(coroutine.yield())", "fora de uma corrotina"},
	}
	for _, c := range casos {
		got := runLua(t, c.src)
		if !strings.Contains(got, c.want) {
			t.Errorf("%s\n  esperava conter %q\n  veio %q", c.src, c.want, got)
		}
	}
}
