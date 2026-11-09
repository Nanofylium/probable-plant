package web

import (
	"sync"
	"testing"

	"github.com/Nanofylium/probable-plant/lang"
)

// Dois Store no mesmo diretório imitam dois processos CGI escrevendo ao mesmo tempo.
func TestStoreConcorrente(t *testing.T) {
	dir := t.TempDir()
	a, err := OpenStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	b, _ := OpenStore(dir)
	var wg sync.WaitGroup
	for _, s := range []*Store{a, b} {
		wg.Add(1)
		go func(s *Store) {
			defer wg.Done()
			for i := 0; i < 100; i++ {
				tb := lang.NewTable()
				tb.SetStr("n", int64(i))
				if _, err := s.Insert("itens", tb); err != nil {
					t.Error(err)
				}
			}
		}(s)
	}
	wg.Wait()
	if n := a.Count("itens"); n != 200 {
		t.Fatalf("esperava 200 registros, veio %d", n)
	}
	ids := map[int64]bool{}
	for _, r := range b.All("itens", 0) {
		if ids[r.ID] {
			t.Fatalf("id repetido %d", r.ID)
		}
		ids[r.ID] = true
	}
	// reabrir lê o mesmo conteúdo
	c, _ := OpenStore(dir)
	if n := c.Count("itens"); n != 200 {
		t.Fatalf("depois de reabrir: %d", n)
	}
	r := c.All("itens", 1)[0]
	if ok, _ := c.Delete("itens", r.ID); !ok || a.Count("itens") != 199 {
		t.Fatal("delete não apareceu no outro processo")
	}
	if v, _ := a.Incr("itens", c.All("itens", 1)[0].ID, "n", 5); v == nil {
		t.Fatal("incr")
	}
}

func TestStoreCompacta(t *testing.T) {
	dir := t.TempDir()
	s, _ := OpenStore(dir)
	var last int64
	for i := 0; i < 2600; i++ {
		id, _ := s.Insert("x", lang.NewTable())
		s.Delete("x", id)
		last = id
	}
	if s.lines > 3000 {
		t.Fatalf("não compactou: %d linhas", s.lines)
	}
	s2, _ := OpenStore(dir)
	id, _ := s2.Insert("x", lang.NewTable())
	if id <= last {
		t.Fatalf("id reutilizado: %d <= %d", id, last)
	}
}
