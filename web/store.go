package web

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/Nanofylium/probable-plant/lang"
)

// Store é o banco de dados próprio do Nepenthes: coleções de documentos JSON
// guardadas num único arquivo de log (data/nep.db). Cada escrita é uma linha
// acrescentada; o arquivo é compactado de vez em quando. Funciona com vários
// processos ao mesmo tempo (modo CGI) graças a um arquivo de trava.
type Store struct {
	dir     string
	file    string
	mu      sync.Mutex
	size    int64
	modTime time.Time
	recs    map[int64]*Record
	meta    map[string]string
	hits    map[string][]int64
	nextID  int64
	lines   int
}

type Record struct {
	ID      int64  `json:"id"`
	Col     string `json:"col"`
	Created string `json:"c"`
	Updated string `json:"u,omitempty"`
	Data    string `json:"-"`
}

type logLine struct {
	Op  string          `json:"op"`
	Col string          `json:"col,omitempty"`
	ID  int64           `json:"id,omitempty"`
	C   string          `json:"c,omitempty"`
	U   string          `json:"u,omitempty"`
	D   json.RawMessage `json:"d,omitempty"`
	K   string          `json:"k,omitempty"`
	V   string          `json:"v,omitempty"`
	T   int64           `json:"t,omitempty"`
}

var colRe = regexp.MustCompile(`^[A-Za-z][\w\-]{0,63}$`)

// ValidCol diz se o nome pode ser usado como coleção pelo código do site.
func ValidCol(c string) bool { return colRe.MatchString(c) }

func OpenStore(dir string) (*Store, error) {
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return nil, fmt.Errorf("não consegui criar a pasta de dados %s: %v", dir, err)
	}
	protect(dir)
	s := &Store{dir: dir, file: filepath.Join(dir, "nep.db")}
	if err := s.reload(true); err != nil {
		return nil, err
	}
	return s, nil
}

// protect impede que um servidor Apache entregue a pasta de dados.
func protect(dir string) {
	ht := filepath.Join(dir, ".htaccess")
	if _, err := os.Stat(ht); err != nil {
		_ = os.WriteFile(ht, []byte("Require all denied\nDeny from all\n"), 0o640)
	}
	idx := filepath.Join(dir, "index.html")
	if _, err := os.Stat(idx); err != nil {
		_ = os.WriteFile(idx, nil, 0o640)
	}
}

// lock usa um arquivo de trava para coordenar processos (CGI).
func (s *Store) lock() (func(), error) {
	s.mu.Lock()
	lf := filepath.Join(s.dir, "nep.lock")
	deadline := time.Now().Add(8 * time.Second)
	for {
		f, err := os.OpenFile(lf, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
		if err == nil {
			f.Close()
			return func() { os.Remove(lf); s.mu.Unlock() }, nil
		}
		if st, e := os.Stat(lf); e == nil && time.Since(st.ModTime()) > 15*time.Second {
			os.Remove(lf) // trava abandonada
			continue
		}
		if time.Now().After(deadline) {
			s.mu.Unlock()
			return nil, fmt.Errorf("banco de dados ocupado, tente de novo")
		}
		time.Sleep(15 * time.Millisecond)
	}
}

// reload relê o arquivo se outro processo o alterou.
func (s *Store) reload(force bool) error {
	st, err := os.Stat(s.file)
	if os.IsNotExist(err) {
		s.recs, s.meta, s.hits, s.nextID, s.lines, s.size = map[int64]*Record{}, map[string]string{}, map[string][]int64{}, 1, 0, 0
		return nil
	}
	if err != nil {
		return err
	}
	if !force && st.Size() == s.size && st.ModTime().Equal(s.modTime) {
		return nil
	}
	f, err := os.Open(s.file)
	if err != nil {
		return err
	}
	defer f.Close()
	s.recs, s.meta, s.hits, s.nextID, s.lines = map[int64]*Record{}, map[string]string{}, map[string][]int64{}, 1, 0
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 1<<20), 64<<20)
	for sc.Scan() {
		b := sc.Bytes()
		if len(b) == 0 {
			continue
		}
		var l logLine
		if json.Unmarshal(b, &l) != nil {
			continue // linha incompleta (queda no meio da escrita): ignora
		}
		s.apply(&l)
		s.lines++
	}
	s.size, s.modTime = st.Size(), st.ModTime()
	return sc.Err()
}

func (s *Store) apply(l *logLine) {
	switch l.Op {
	case "put":
		s.recs[l.ID] = &Record{ID: l.ID, Col: l.Col, Created: l.C, Updated: l.U, Data: string(l.D)}
		if l.ID >= s.nextID {
			s.nextID = l.ID + 1
		}
	case "del":
		delete(s.recs, l.ID)
	case "next": // gravado na compactação: ids de registros apagados nunca são reutilizados
		if l.ID > s.nextID {
			s.nextID = l.ID
		}
	case "meta":
		s.meta[l.K] = l.V
	case "hit":
		s.hits[l.K] = append(s.hits[l.K], l.T)
	}
}

func (s *Store) write(lines ...*logLine) error {
	f, err := os.OpenFile(s.file, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o640)
	if err != nil {
		return fmt.Errorf("não consegui gravar no banco: %v", err)
	}
	var b strings.Builder
	for _, l := range lines {
		j, _ := json.Marshal(l)
		b.Write(j)
		b.WriteByte('\n')
		s.apply(l)
		s.lines++
	}
	if _, err := f.WriteString(b.String()); err != nil {
		f.Close()
		return err
	}
	_ = f.Sync()
	f.Close()
	if st, err := os.Stat(s.file); err == nil {
		s.size, s.modTime = st.Size(), st.ModTime()
	}
	if s.lines > 2*(len(s.recs)+len(s.meta))+2000 {
		s.compact()
	}
	return nil
}

// compact reescreve o arquivo só com o estado atual.
func (s *Store) compact() {
	tmp := s.file + ".tmp"
	f, err := os.Create(tmp)
	if err != nil {
		return
	}
	w := bufio.NewWriter(f)
	n := 0
	enc := func(l *logLine) {
		j, _ := json.Marshal(l)
		w.Write(j)
		w.WriteByte('\n')
		n++
	}
	keys := make([]string, 0, len(s.meta))
	for k := range s.meta {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		enc(&logLine{Op: "meta", K: k, V: s.meta[k]})
	}
	enc(&logLine{Op: "next", ID: s.nextID})
	for _, r := range s.sorted("", false) {
		enc(&logLine{Op: "put", Col: r.Col, ID: r.ID, C: r.Created, U: r.Updated, D: json.RawMessage(r.Data)})
	}
	cut := time.Now().Unix() - 86400
	for k, ts := range s.hits {
		for _, t := range ts {
			if t > cut {
				enc(&logLine{Op: "hit", K: k, T: t})
			}
		}
	}
	if w.Flush() != nil || f.Sync() != nil {
		f.Close()
		os.Remove(tmp)
		return
	}
	f.Close()
	if os.Rename(tmp, s.file) == nil {
		s.lines = n
		_ = s.reload(true)
	}
}

func (s *Store) sorted(col string, desc bool) []*Record {
	out := make([]*Record, 0)
	for _, r := range s.recs {
		if col == "" || r.Col == col {
			out = append(out, r)
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if desc {
			return out[i].ID > out[j].ID
		}
		return out[i].ID < out[j].ID
	})
	return out
}

// ToValue converte um registro em tabela Nepenthes: {id, ...campos, created_at, updated_at}.
func (r *Record) ToValue() *lang.Table {
	t := lang.NewTable()
	t.SetStr("id", r.ID)
	if d, err := lang.FromJSON(r.Data); err == nil {
		if dt, ok := d.(*lang.Table); ok {
			for i := 0; i < dt.Len(); i++ {
				k, v := dt.Entry(i)
				if k == "id" || k == "created_at" || k == "updated_at" {
					continue
				}
				_ = t.Set(k, v)
			}
		}
	}
	t.SetStr("created_at", r.Created)
	if r.Updated != "" {
		t.SetStr("updated_at", r.Updated)
	} else {
		t.SetStr("updated_at", nil)
	}
	return t
}

func stamp() string { return time.Now().Format("2006-01-02 15:04:05") }

func dataJSON(data *lang.Table) (json.RawMessage, error) {
	clean := lang.NewTable()
	for i := 0; i < data.Len(); i++ {
		k, v := data.Entry(i)
		if k == "id" || k == "created_at" || k == "updated_at" {
			continue
		}
		_ = clean.Set(k, v)
	}
	j, err := lang.ToJSON(clean, "")
	return json.RawMessage(j), err
}

// ---------- API usada pelo servidor e pelo código do site ----------

func (s *Store) All(col string, limit int) []*Record {
	s.mu.Lock()
	defer s.mu.Unlock()
	_ = s.reload(false)
	rs := s.sorted(col, true)
	if limit > 0 && len(rs) > limit {
		rs = rs[:limit]
	}
	return rs
}

func (s *Store) Find(col string, id int64) *Record {
	s.mu.Lock()
	defer s.mu.Unlock()
	_ = s.reload(false)
	if r, ok := s.recs[id]; ok && r.Col == col {
		return r
	}
	return nil
}

func (s *Store) Insert(col string, data *lang.Table) (int64, error) {
	j, err := dataJSON(data)
	if err != nil {
		return 0, err
	}
	unlock, err := s.lock()
	if err != nil {
		return 0, err
	}
	defer unlock()
	if err := s.reload(false); err != nil {
		return 0, err
	}
	id := s.nextID
	return id, s.write(&logLine{Op: "put", Col: col, ID: id, C: stamp(), D: j})
}

func (s *Store) Update(col string, id int64, data *lang.Table) (bool, error) {
	unlock, err := s.lock()
	if err != nil {
		return false, err
	}
	defer unlock()
	_ = s.reload(false)
	r, ok := s.recs[id]
	if !ok || r.Col != col {
		return false, nil
	}
	cur := r.ToValue()
	for i := 0; i < data.Len(); i++ {
		k, v := data.Entry(i)
		_ = cur.Set(k, v)
	}
	j, err := dataJSON(cur)
	if err != nil {
		return false, err
	}
	return true, s.write(&logLine{Op: "put", Col: col, ID: id, C: r.Created, U: stamp(), D: j})
}

func (s *Store) Delete(col string, id int64) (bool, error) {
	unlock, err := s.lock()
	if err != nil {
		return false, err
	}
	defer unlock()
	_ = s.reload(false)
	r, ok := s.recs[id]
	if !ok || r.Col != col {
		return false, nil
	}
	if err := s.write(&logLine{Op: "del", Col: col, ID: id}); err != nil {
		return false, err
	}
	s.removeFiles(filesIn(r.Data)) // o arquivo enviado sai junto com o registro (LGPD)
	return true, nil
}

func (s *Store) Count(col string) int {
	s.mu.Lock()
	defer s.mu.Unlock()
	_ = s.reload(false)
	n := 0
	for _, r := range s.recs {
		if r.Col == col {
			n++
		}
	}
	return n
}

func (s *Store) Collections() map[string]int {
	s.mu.Lock()
	defer s.mu.Unlock()
	_ = s.reload(false)
	out := map[string]int{}
	for _, r := range s.recs {
		out[r.Col]++
	}
	return out
}

// Search procura um texto em todos os registros (atendimento LGPD).
func (s *Store) Search(term string) []*Record {
	s.mu.Lock()
	defer s.mu.Unlock()
	_ = s.reload(false)
	term = strings.ToLower(term)
	var out []*Record
	for _, r := range s.sorted("", false) {
		if strings.Contains(strings.ToLower(r.Data), term) || strings.Contains(strings.ToLower(jsonUnescape(r.Data)), term) {
			out = append(out, r)
		}
	}
	return out
}

func jsonUnescape(s string) string {
	var v any
	if json.Unmarshal([]byte(s), &v) != nil {
		return s
	}
	var b strings.Builder
	enc := json.NewEncoder(&b)
	enc.SetEscapeHTML(false)
	_ = enc.Encode(v)
	return strings.TrimSpace(b.String())
}

func (s *Store) Meta(k string) string {
	s.mu.Lock()
	defer s.mu.Unlock()
	_ = s.reload(false)
	return s.meta[k]
}

func (s *Store) SetMeta(k, v string) error {
	unlock, err := s.lock()
	if err != nil {
		return err
	}
	defer unlock()
	_ = s.reload(false)
	return s.write(&logLine{Op: "meta", K: k, V: v})
}

// MetaOrInit devolve o valor de k, criando-o com mk() na primeira vez.
func (s *Store) MetaOrInit(k string, mk func() string) (string, error) {
	if v := s.Meta(k); v != "" {
		return v, nil
	}
	unlock, err := s.lock()
	if err != nil {
		return "", err
	}
	defer unlock()
	_ = s.reload(false)
	if v := s.meta[k]; v != "" {
		return v, nil
	}
	v := mk()
	return v, s.write(&logLine{Op: "meta", K: k, V: v})
}

// Rate diz se a chave ainda está dentro do limite (max por janela de segundos).
// Com record=true, conta esta tentativa.
func (s *Store) Rate(key string, max int, window int64, record bool) (bool, error) {
	unlock, err := s.lock()
	if err != nil {
		return false, err
	}
	defer unlock()
	_ = s.reload(false)
	now := time.Now().Unix()
	n := 0
	for _, t := range s.hits[key] {
		if t > now-window {
			n++
		}
	}
	if n >= max {
		return false, nil
	}
	if record {
		return true, s.write(&logLine{Op: "hit", K: key, T: now})
	}
	return true, nil
}

// Version muda sempre que algum dado muda (usado pelas regiões <live>).
func (s *Store) Version() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	_ = s.reload(false)
	return fmt.Sprintf("%x-%x", s.size, s.nextID)
}

// Incr soma n ao campo numérico de um registro sem risco de dois pedidos se atropelarem.
func (s *Store) Incr(col string, id int64, field string, n float64) (lang.Value, error) {
	unlock, err := s.lock()
	if err != nil {
		return nil, err
	}
	defer unlock()
	_ = s.reload(false)
	r, ok := s.recs[id]
	if !ok || r.Col != col {
		return nil, nil
	}
	cur := r.ToValue()
	old, _ := cur.Get(field)
	var nv lang.Value
	switch x := old.(type) {
	case int64:
		if n == float64(int64(n)) {
			nv = x + int64(n)
		} else {
			nv = float64(x) + n
		}
	case float64:
		nv = x + n
	case nil:
		if n == float64(int64(n)) {
			nv = int64(n)
		} else {
			nv = n
		}
	default:
		return nil, fmt.Errorf("db.incr: o campo %s não é número", field)
	}
	cur.SetStr(field, nv)
	j, err := dataJSON(cur)
	if err != nil {
		return nil, err
	}
	return nv, s.write(&logLine{Op: "put", Col: col, ID: id, C: r.Created, U: stamp(), D: j})
}
