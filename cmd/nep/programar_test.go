package main

import (
	"context"
	"encoding/json"
	"os"
	"strings"
	"testing"
)

// O teste se faz de nep: com NEP_TESTE_RUN=1, "run arquivo" roda o programa.
func TestMain(m *testing.M) {
	if os.Getenv("NEP_TESTE_RUN") == "1" && len(os.Args) >= 3 && os.Args[1] == "run" {
		os.Exit(run(os.Args[2], os.Args[3:]))
	}
	os.Setenv("NEP_TESTE_RUN", "1")
	os.Exit(m.Run())
}

func TestProgramar(t *testing.T) {
	c := novaCentral(t.TempDir() + "/sites")
	c.extras = nil
	api := func(caminho, corpo string) map[string]any {
		t.Helper()
		w := pedidoCentral(c, "POST", caminho, []byte(corpo), "application/json", true)
		var r map[string]any
		json.Unmarshal(w.Body.Bytes(), &r)
		if w.Code != 200 {
			t.Fatalf("%s: %d %s", caminho, w.Code, w.Body)
		}
		return r
	}
	if w := pedidoCentral(c, "GET", "/programar", nil, "", false); w.Code != 200 || !strings.Contains(w.Body.String(), c.token) {
		t.Fatal("a página do editor devia abrir com a chave")
	}
	r := api("/api/programa/rodar", `{"nome":"olá.nep","codigo":"let n = input(\"nome? \")\nprint(`+"`Olá, {n}!`"+`)\nprint([1,2,3] |> sum)\n","entrada":"Ana\n"}`)
	if !strings.Contains(r["saida"].(string), "Olá, Ana!") || !strings.Contains(r["saida"].(string), "6") || r["codigo"].(float64) != 0 {
		t.Fatalf("%v", r)
	}
	r = api("/api/programa/rodar", `{"nome":"erro.nep","codigo":"print(1 // 0)\n"}`)
	if r["codigo"].(float64) == 0 || !strings.Contains(r["saida"].(string), "zero") {
		t.Fatalf("o erro devia aparecer: %v", r)
	}
	r = api("/api/programa/rodar", `{"nome":"x.lua","codigo":"print(string.rep('ab', 3))\n"}`)
	if !strings.Contains(r["saida"].(string), "ababab") {
		t.Fatalf("lua: %v", r)
	}
	l := api("/api/programa/lista", `{}`)["programas"].([]any)
	if len(l) != 3 {
		t.Fatalf("lista: %v", l)
	}
	if r := api("/api/programa/abrir", `{"nome":"olá.nep"}`); !strings.Contains(r["codigo"].(string), "input") {
		t.Fatal(r)
	}
	for _, ruim := range []string{"../fora.nep", "a/b.nep", "x.exe", ".nep", "..nep"} {
		w := pedidoCentral(c, "POST", "/api/programa/salvar", []byte(`{"nome":"`+ruim+`","codigo":"print(1)"}`), "application/json", true)
		if w.Code != 400 {
			t.Fatalf("nome %q devia ser recusado", ruim)
		}
	}
	if w := pedidoCentral(c, "POST", "/api/programa/rodar", []byte(`{"nome":"a.nep","codigo":"print(1)"}`), "application/json", false); w.Code != 403 {
		t.Fatal("sem a chave não roda")
	}
}

func TestProgramarIA(t *testing.T) {
	velho := iaChat
	defer func() { iaChat = velho }()
	var respostas = []string{
		"Aqui está:\n```nep\nif x do\nprint(x)\n```", // não compila
		"Corrigido:\n```nep\nlet x = [1, 2, 3] |> map(fn(v) do v * 2 end)\nprint(x)\n```",
	}
	var vistas [][]map[string]string
	iaChat = func(ctx context.Context, msgs []map[string]string, n int) (string, error) {
		vistas = append(vistas, msgs)
		r := respostas[0]
		respostas = respostas[1:]
		return r, nil
	}
	r, err := iaPrograma(context.Background(), pedidoIA{Pedido: "dobre a lista"})
	if err != nil || r["ok"] != true || r["tentativas"] != 2 || !strings.Contains(r["codigo"].(string), "map(") || r["explicacao"] != "Corrigido:" {
		t.Fatalf("%v %v", r, err)
	}
	if !strings.Contains(vistas[0][0]["content"], "do ... end") || !strings.Contains(vistas[1][3]["content"], "não compila") {
		t.Fatal("a IA devia receber a sintaxe e o erro de compilação")
	}
	if _, c := separarCodigo("só texto"); c != "só texto" {
		t.Fatal(c)
	}
}
