package main

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// Liga um PostgreSQL de verdade (baixa uns 30 MB na primeira vez). O initdb
// não roda como root: nesse caso o teste é pulado.
func TestNepenthesDB(t *testing.T) {
	if runtime.GOOS != "windows" && os.Geteuid() == 0 {
		t.Skip("o PostgreSQL não roda como root")
	}
	if testing.Short() {
		t.Skip("baixa o PostgreSQL")
	}
	base := t.TempDir()
	d := novoNepDB(base)
	d.cache = filepath.Join(os.TempDir(), "nepenthes-teste-pg") // binários reaproveitados entre testes
	if err := d.Ligar(); err != nil {
		t.Fatal(err)
	}
	defer d.Desligar()
	e := d.Estado()
	if e["fase"] != "ligado" || !strings.HasPrefix(e["url"].(string), "postgres://nepenthes:") {
		t.Fatalf("%v", e)
	}
	ctx := context.Background()
	if _, err := d.Consultar(ctx, "CREATE TABLE x (n int)", false); err == nil || !strings.Contains(err.Error(), "Permitir alterar") {
		t.Fatalf("leitura devia barrar escrita: %v", err)
	}
	if err := d.Exemplo(ctx); err != nil {
		t.Fatal(err)
	}
	r, err := d.Consultar(ctx, "SELECT produto, sum(quantidade) AS n, min(dia) AS d FROM vendas_exemplo GROUP BY 1 ORDER BY 1", false)
	if err != nil || len(r.Linhas) != 4 || r.Colunas[1] != "n" || r.Linhas[0][0] != "Bolo de cenoura" {
		t.Fatalf("%v %+v", err, r)
	}
	if s, ok := r.Linhas[0][2].(string); !ok || len(s) != 10 {
		t.Fatalf("data: %#v", r.Linhas[0][2])
	}
	if ps := d.Paineis(); len(ps) != 2 {
		t.Fatalf("painéis de exemplo: %d", len(ps))
	}
	p, err := d.SalvarPainel(painel{Titulo: "Total", SQL: "SELECT 1 AS um", Tipo: "x"})
	if err != nil || p.ID == "" || p.Tipo != "barra" || len(d.Paineis()) != 3 {
		t.Fatalf("%v %+v", err, p)
	}
	d.ApagarPainel(p.ID)
	if len(d.Paineis()) != 2 {
		t.Fatal("apagar painel")
	}
	// desliga e liga de novo: os dados continuam
	if err := d.Desligar(); err != nil {
		t.Fatal(err)
	}
	d2 := novoNepDB(base)
	d2.cache = d.cache
	if err := d2.Ligar(); err != nil {
		t.Fatal(err)
	}
	defer d2.Desligar()
	if d2.cfg.Senha != d.cfg.Senha {
		t.Fatal("a senha devia ficar guardada")
	}
	if r, err := d2.Consultar(ctx, "SELECT count(*) FROM vendas_exemplo", false); err != nil || r.Linhas[0][0].(int64) < 300 {
		t.Fatalf("dados perdidos: %v %v", err, r)
	}
}

func TestNepenthesDBNaoApagaOutraVersao(t *testing.T) {
	base := t.TempDir()
	d := novoNepDB(base)
	os.MkdirAll(filepath.Join(d.pasta, "dados"), 0o755)
	os.WriteFile(filepath.Join(d.pasta, "dados", "PG_VERSION"), []byte("15\n"), 0o644)
	if err := d.Ligar(); err == nil || !strings.Contains(err.Error(), "não vai mexer") {
		t.Fatalf("%v", err)
	}
	if _, err := os.Stat(filepath.Join(d.pasta, "dados", "PG_VERSION")); err != nil {
		t.Fatal("os dados de outra versão sumiram")
	}
	if d.Estado()["fase"] != "erro" {
		t.Fatal("fase")
	}
}
