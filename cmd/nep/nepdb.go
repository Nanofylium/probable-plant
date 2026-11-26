package main

// Nepenthes DB sem Docker: um PostgreSQL 16 de verdade, baixado uma vez e
// ligado com um botão na Central. Usa o embedded-postgres (Fergus Strange,
// MIT), que traz os binários oficiais empacotados pelo projeto zonky (Apache
// 2.0). Os dados ficam na pasta "Nepenthes DB", junto dos sites.

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	ep "github.com/fergusstrange/embedded-postgres"
	_ "github.com/lib/pq"
)

const versaoPG = "16" // não mudar: os dados de uma versão não abrem em outra

type configDB struct {
	Porta   uint32 `json:"porta"`
	Usuario string `json:"usuario"`
	Senha   string `json:"senha"`
	Banco   string `json:"banco"`
	Ligar   bool   `json:"ligar_ao_abrir"`
}

type painel struct {
	ID     string `json:"id"`
	Titulo string `json:"titulo"`
	SQL    string `json:"sql"`
	Tipo   string `json:"tipo"` // barra | linha | pizza | tabela
	X      string `json:"x"`
	Y      string `json:"y"`
}

type nepDB struct {
	pasta string // <base>/Nepenthes DB
	cache string // binários baixados
	mu    sync.Mutex
	fase  string // desligado | ligando | ligado | erro
	erro  string
	cfg   configDB
	pg    *ep.EmbeddedPostgres
	db    *sql.DB
}

func novoNepDB(base string) *nepDB {
	cache, err := os.UserCacheDir()
	if err != nil {
		cache = os.TempDir()
	}
	d := &nepDB{pasta: filepath.Join(base, "Nepenthes DB"), cache: filepath.Join(cache, "nepenthes", "postgres"), fase: "desligado"}
	d.lerConfig()
	return d
}

func (d *nepDB) lerConfig() {
	if b, err := os.ReadFile(filepath.Join(d.pasta, "config.json")); err == nil {
		json.Unmarshal(b, &d.cfg)
	}
	if d.cfg.Usuario == "" {
		d.cfg.Usuario = "nepenthes"
	}
	if d.cfg.Banco == "" {
		d.cfg.Banco = "nepenthes"
	}
	if d.cfg.Senha == "" {
		b := make([]byte, 12)
		rand.Read(b)
		d.cfg.Senha = hex.EncodeToString(b)
	}
}

func (d *nepDB) gravarConfig() error {
	if err := os.MkdirAll(d.pasta, 0o755); err != nil {
		return err
	}
	b, _ := json.MarshalIndent(d.cfg, "", "  ")
	return os.WriteFile(filepath.Join(d.pasta, "config.json"), b, 0o600)
}

func (d *nepDB) url() string {
	return fmt.Sprintf("postgres://%s:%s@localhost:%d/%s?sslmode=disable", d.cfg.Usuario, d.cfg.Senha, d.cfg.Porta, d.cfg.Banco)
}

// Estado é o que a Central mostra.
func (d *nepDB) Estado() map[string]any {
	d.mu.Lock()
	defer d.mu.Unlock()
	e := map[string]any{"fase": d.fase, "pasta": d.pasta}
	if d.erro != "" {
		e["erro"] = d.erro
	}
	if d.fase == "ligado" {
		e["url"], e["porta"], e["usuario"], e["senha"], e["banco"] = d.url(), d.cfg.Porta, d.cfg.Usuario, d.cfg.Senha, d.cfg.Banco
	}
	return e
}

func portaPG() uint32 {
	for p := 5432; p < 5532; p++ {
		l, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", p))
		if err == nil {
			l.Close()
			return uint32(p)
		}
	}
	return 0
}

// Ligar baixa (na primeira vez) e liga o PostgreSQL. Demora: chame numa goroutine.
func (d *nepDB) Ligar() error {
	d.mu.Lock()
	if d.fase == "ligando" || d.fase == "ligado" {
		d.mu.Unlock()
		return nil
	}
	d.fase, d.erro = "ligando", ""
	d.mu.Unlock()
	err := d.ligar()
	d.mu.Lock()
	defer d.mu.Unlock()
	if err != nil {
		d.fase, d.erro = "erro", err.Error()
		return err
	}
	d.fase = "ligado"
	return nil
}

func (d *nepDB) ligar() error {
	dados := filepath.Join(d.pasta, "dados")
	if b, err := os.ReadFile(filepath.Join(dados, "PG_VERSION")); err == nil && strings.TrimSpace(string(b)) != versaoPG {
		// o embedded-postgres apagaria a pasta para recriar: aqui nunca
		return fmt.Errorf("a pasta %s é do PostgreSQL %s; o Nepenthes usa o %s e não vai mexer nela", dados, strings.TrimSpace(string(b)), versaoPG)
	}
	rt := filepath.Join(d.cache, "rodando")
	// o Nepenthes foi fechado de repente na última vez: desliga o que ficou
	if _, err := os.Stat(filepath.Join(dados, "postmaster.pid")); err == nil {
		pgctl := filepath.Join(rt, "bin", "pg_ctl")
		if _, err := os.Stat(pgctl); err != nil {
			pgctl += ".exe"
		}
		c := exec.Command(pgctl, "stop", "-D", dados, "-m", "fast", "-w")
		escondeJanela(c)
		c.Run()
	}
	if d.cfg.Porta == 0 || !portaLivreTCP(d.cfg.Porta) {
		d.cfg.Porta = portaPG()
	}
	if d.cfg.Porta == 0 {
		return errors.New("não achei porta livre para o banco")
	}
	if err := d.gravarConfig(); err != nil {
		return err
	}
	log, err := os.OpenFile(filepath.Join(d.pasta, "registro.txt"), os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644)
	if err != nil {
		return err
	}
	cfg := ep.DefaultConfig().Version(ep.V16).Port(d.cfg.Porta).
		Username(d.cfg.Usuario).Password(d.cfg.Senha).Database(d.cfg.Banco).
		CachePath(filepath.Join(d.cache, "baixados")).RuntimePath(rt).DataPath(dados).
		Locale("C").Encoding("UTF8").StartTimeout(90 * time.Second).Logger(log).
		StartParameters(map[string]string{"listen_addresses": "localhost"})
	pg := ep.NewDatabase(cfg)
	if err := pg.Start(); err != nil {
		return fmt.Errorf("não consegui ligar o banco: %v (detalhes em %s)", err, filepath.Join(d.pasta, "registro.txt"))
	}
	db, err := sql.Open("postgres", d.url())
	if err != nil {
		pg.Stop()
		return err
	}
	db.SetMaxOpenConns(4)
	d.mu.Lock()
	d.pg, d.db = pg, db
	d.mu.Unlock()
	return nil
}

func portaLivreTCP(p uint32) bool {
	l, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", p))
	if err != nil {
		return false
	}
	l.Close()
	return true
}

// Desligar para o PostgreSQL (os dados ficam).
func (d *nepDB) Desligar() error {
	d.mu.Lock()
	pg, db := d.pg, d.db
	d.pg, d.db = nil, nil
	if d.fase == "ligado" {
		d.fase = "desligado"
	}
	d.mu.Unlock()
	if db != nil {
		db.Close()
	}
	if pg != nil {
		return pg.Stop()
	}
	return nil
}

func (d *nepDB) LembrarLigado(v bool) {
	d.mu.Lock()
	d.cfg.Ligar = v
	d.mu.Unlock()
	d.gravarConfig()
}

type resultado struct {
	Colunas []string `json:"colunas"`
	Linhas  [][]any  `json:"linhas"`
	Mais    bool     `json:"mais,omitempty"` // passou do limite
	Afetou  int64    `json:"afetou,omitempty"`
}

// Consultar roda SQL. Sem escrever, a transação é só de leitura (análises não
// estragam dados por engano).
func (d *nepDB) Consultar(ctx context.Context, q string, escrever bool) (*resultado, error) {
	d.mu.Lock()
	db := d.db
	d.mu.Unlock()
	if db == nil {
		return nil, errors.New("ligue o Nepenthes DB primeiro")
	}
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	tx, err := db.BeginTx(ctx, &sql.TxOptions{ReadOnly: !escrever})
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	rows, err := tx.QueryContext(ctx, q)
	if err != nil {
		return nil, limpaErroPG(err)
	}
	cols, _ := rows.Columns()
	res := &resultado{Colunas: cols, Linhas: [][]any{}}
	for rows.Next() {
		if len(res.Linhas) == 1000 {
			res.Mais = true
			break
		}
		v := make([]any, len(cols))
		p := make([]any, len(cols))
		for i := range v {
			p[i] = &v[i]
		}
		if err := rows.Scan(p...); err != nil {
			rows.Close()
			return nil, err
		}
		for i, x := range v {
			switch t := x.(type) {
			case []byte:
				v[i] = string(t)
			case time.Time:
				if t.Hour() == 0 && t.Minute() == 0 && t.Second() == 0 {
					v[i] = t.Format("2006-01-02")
				} else {
					v[i] = t.Format("2006-01-02 15:04:05")
				}
			}
		}
		res.Linhas = append(res.Linhas, v)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, limpaErroPG(err)
	}
	if escrever {
		if err := tx.Commit(); err != nil {
			return nil, limpaErroPG(err)
		}
	}
	return res, nil
}

func limpaErroPG(err error) error {
	m := strings.TrimPrefix(err.Error(), "pq: ")
	if strings.Contains(m, "read-only transaction") {
		return errors.New("esta consulta muda dados: marque \"Permitir alterar dados\" para rodar")
	}
	return errors.New(m)
}

// Exemplo cria uma tabela de vendas para ver os gráficos funcionando.
func (d *nepDB) Exemplo(ctx context.Context) error {
	_, err := d.Consultar(ctx, `
CREATE TABLE IF NOT EXISTS vendas_exemplo (
  id serial PRIMARY KEY, dia date NOT NULL, produto text NOT NULL, quantidade int NOT NULL, valor numeric(10,2) NOT NULL);
DELETE FROM vendas_exemplo;
INSERT INTO vendas_exemplo (dia, produto, quantidade, valor)
SELECT d::date, p.nome, q, round((q * p.preco)::numeric, 2)
FROM generate_series(date_trunc('month', now()) - interval '11 months', now(), interval '1 day') d
CROSS JOIN (VALUES ('Pão de fermentação natural', 18.0), ('Bolo de cenoura', 32.0), ('Café coado', 7.5), ('Croissant', 11.0)) p(nome, preco)
CROSS JOIN LATERAL (SELECT 3 + floor(random() * 20 * (1 + extract(month from d) / 12.0))::int AS q) r;`, true)
	if err != nil {
		return err
	}
	ps := d.Paineis()
	tem := map[string]bool{}
	for _, p := range ps {
		tem[p.ID] = true
	}
	for _, p := range []painel{
		{ID: "exemplo-mes", Titulo: "Vendas por mês (R$)", Tipo: "linha", X: "mes", Y: "total",
			SQL: "SELECT to_char(date_trunc('month', dia), 'YYYY-MM') AS mes, sum(valor) AS total\nFROM vendas_exemplo GROUP BY 1 ORDER BY 1"},
		{ID: "exemplo-produto", Titulo: "Mais vendidos", Tipo: "pizza", X: "produto", Y: "unidades",
			SQL: "SELECT produto, sum(quantidade) AS unidades\nFROM vendas_exemplo GROUP BY 1 ORDER BY 2 DESC"},
	} {
		if !tem[p.ID] {
			ps = append(ps, p)
		}
	}
	return d.gravarPaineis(ps)
}

func (d *nepDB) Paineis() []painel {
	ps := []painel{}
	if b, err := os.ReadFile(filepath.Join(d.pasta, "paineis.json")); err == nil {
		json.Unmarshal(b, &ps)
	}
	return ps
}

func (d *nepDB) gravarPaineis(ps []painel) error {
	if err := os.MkdirAll(d.pasta, 0o755); err != nil {
		return err
	}
	b, _ := json.MarshalIndent(ps, "", "  ")
	return os.WriteFile(filepath.Join(d.pasta, "paineis.json"), b, 0o644)
}

// SalvarPainel grava (ou troca, pelo id) um gráfico do painel de análises.
func (d *nepDB) SalvarPainel(p painel) (painel, error) {
	if strings.TrimSpace(p.SQL) == "" || strings.TrimSpace(p.Titulo) == "" {
		return p, errors.New("dê um título e escreva a consulta")
	}
	switch p.Tipo {
	case "barra", "linha", "pizza", "tabela":
	default:
		p.Tipo = "barra"
	}
	ps := d.Paineis()
	if p.ID == "" {
		b := make([]byte, 4)
		rand.Read(b)
		p.ID = hex.EncodeToString(b)
	}
	achou := false
	for i := range ps {
		if ps[i].ID == p.ID {
			ps[i], achou = p, true
		}
	}
	if !achou {
		ps = append(ps, p)
	}
	return p, d.gravarPaineis(ps)
}

func (d *nepDB) ApagarPainel(id string) error {
	ps := d.Paineis()
	out := ps[:0]
	for _, p := range ps {
		if p.ID != id {
			out = append(out, p)
		}
	}
	return d.gravarPaineis(out)
}
