package web

// Estatísticas de visitas embutidas, sem cookies e sem guardar dados pessoais
// (no espírito da LGPD): cada visita grava dia, página, site de origem e tipo
// de aparelho. Visitantes únicos são estimados com um hash de IP + navegador
// misturado a um sal aleatório que é trocado todo dia e nunca guardado depois,
// então não dá para reconstruir quem visitou nem seguir alguém entre dias.

import (
	"bufio"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"html"
	"net"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"
)

type visit struct {
	D string `json:"d"`           // dia AAAA-MM-DD
	P string `json:"p"`           // caminho
	R string `json:"r,omitempty"` // site de origem (só o domínio)
	M bool   `json:"m,omitempty"` // celular
	V string `json:"v"`           // visitante do dia (hash curto, sal trocado diariamente)
}

var botWords = []string{"bot", "crawl", "spider", "slurp", "preview", "facebookexternalhit", "headless", "lighthouse", "curl", "wget", "python-requests", "go-http-client"}

func isBot(ua string) bool {
	if ua == "" {
		return true
	}
	l := strings.ToLower(ua)
	for _, w := range botWords {
		if strings.Contains(l, w) {
			return true
		}
	}
	return false
}

func (s *Server) statsOn(site *Site) bool {
	v := strings.ToLower(strings.TrimSpace(site.Config["estatisticas"]))
	return v != "false" && v != "0" && v != "nao" && v != "não"
}

// countVisit registra uma visita a uma página (só GET, HTML completo, sem robôs).
func (s *Server) countVisit(rq *request) {
	r := rq.r
	ua := r.Header.Get("User-Agent")
	if r.Method != "GET" || isBot(ua) || r.Header.Get("DNT") == "1" || r.Header.Get("Sec-GPC") == "1" || rq.sess.isAdmin() {
		return
	}
	now := time.Now()
	day := now.Format("2006-01-02")
	salt, err := s.daySalt(day)
	if err != nil {
		return
	}
	ip := r.RemoteAddr
	if h, _, err := net.SplitHostPort(ip); err == nil {
		ip = h
	}
	m := hmac.New(sha256.New, []byte(salt))
	m.Write([]byte(ip + "|" + ua))
	v := visit{D: day, P: rq.path, V: hex.EncodeToString(m.Sum(nil))[:10]}
	if ref, err := url.Parse(r.Referer()); err == nil && ref.Host != "" && !strings.EqualFold(ref.Host, r.Host) {
		v.R = strings.TrimPrefix(strings.ToLower(ref.Hostname()), "www.")
	}
	l := strings.ToLower(ua)
	v.M = strings.Contains(l, "mobi") || strings.Contains(l, "android") || strings.Contains(l, "iphone")
	j, _ := json.Marshal(v)
	f, err := os.OpenFile(filepath.Join(s.opt.DataDir, "visitas-"+now.Format("2006-01")+".log"), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o640)
	if err != nil {
		return
	}
	f.Write(append(j, '\n'))
	f.Close()
}

// daySalt devolve o sal do dia; ao virar o dia, o sal anterior é substituído (e some).
func (s *Server) daySalt(day string) (string, error) {
	cur := s.Store.Meta("sal_visitas")
	if strings.HasPrefix(cur, day+":") {
		return cur, nil
	}
	v := day + ":" + randHex(16)
	return v, s.Store.SetMeta("sal_visitas", v)
}

type statsSummary struct {
	Days                  []string
	Views, Uniq           map[string]int
	Pages, Refs           map[string]int
	Mobile, Total, Total2 int
}

func (s *Server) loadStats(days int) statsSummary {
	st := statsSummary{Views: map[string]int{}, Uniq: map[string]int{}, Pages: map[string]int{}, Refs: map[string]int{}}
	now := time.Now()
	first := now.AddDate(0, 0, -(days - 1)).Format("2006-01-02")
	for i := days - 1; i >= 0; i-- {
		st.Days = append(st.Days, now.AddDate(0, 0, -i).Format("2006-01-02"))
	}
	seen := map[string]bool{}
	months := map[string]bool{}
	for _, d := range st.Days {
		months[d[:7]] = true
	}
	for mo := range months {
		f, err := os.Open(filepath.Join(s.opt.DataDir, "visitas-"+mo+".log"))
		if err != nil {
			continue
		}
		sc := bufio.NewScanner(f)
		for sc.Scan() {
			var v visit
			if json.Unmarshal(sc.Bytes(), &v) != nil || v.D < first {
				continue
			}
			st.Views[v.D]++
			st.Total++
			if k := v.D + v.V; !seen[k] {
				seen[k] = true
				st.Uniq[v.D]++
				st.Total2++
			}
			st.Pages[v.P]++
			if v.R != "" {
				st.Refs[v.R]++
			}
			if v.M {
				st.Mobile++
			}
		}
		f.Close()
	}
	return st
}

func top(m map[string]int, n int) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool {
		if m[keys[i]] != m[keys[j]] {
			return m[keys[i]] > m[keys[j]]
		}
		return keys[i] < keys[j]
	})
	if len(keys) > n {
		keys = keys[:n]
	}
	return keys
}

// statsPage é a página /_admin/visitas.
func (a *adminPage) statsPage() error {
	days := 30
	if d, err := strconv.Atoi(a.rq.r.URL.Query().Get("dias")); err == nil && (d == 7 || d == 90) {
		days = d
	}
	st := a.s.loadStats(days)
	var b strings.Builder
	fmt.Fprintf(&b, `<p class="row">Período: `)
	for _, d := range []int{7, 30, 90} {
		if d == days {
			fmt.Fprintf(&b, `<strong>%d dias</strong> `, d)
		} else {
			fmt.Fprintf(&b, `<a href="%s/visitas?dias=%d">%d dias</a> `, a.base, d, d)
		}
	}
	b.WriteString(`</p>`)
	mob := 0
	if st.Total > 0 {
		mob = st.Mobile * 100 / st.Total
	}
	fmt.Fprintf(&b, `<div class="row" style="gap:24px;margin:8px 0 16px"><div><div class="muted">Visualizações</div><strong style="font-size:1.6rem">%d</strong></div>`+
		`<div><div class="muted">Visitantes (soma diária)</div><strong style="font-size:1.6rem">%d</strong></div>`+
		`<div><div class="muted">No celular</div><strong style="font-size:1.6rem">%d%%</strong></div></div>`, st.Total, st.Total2, mob)

	max := 1
	for _, d := range st.Days {
		if st.Views[d] > max {
			max = st.Views[d]
		}
	}
	b.WriteString(`<div style="display:flex;align-items:flex-end;gap:2px;height:140px;border-bottom:1px solid #dfe3dc;margin-bottom:4px" role="img" aria-label="Visualizações por dia">`)
	for _, d := range st.Days {
		h := st.Views[d] * 130 / max
		u := st.Uniq[d] * 130 / max
		t, _ := time.Parse("2006-01-02", d)
		fmt.Fprintf(&b, `<div title="%s: %d visualizações, %d visitantes" style="flex:1;position:relative;height:%dpx;background:#cfe3d6;border-radius:3px 3px 0 0">`+
			`<div style="position:absolute;bottom:0;left:0;right:0;height:%dpx;background:#2b6b45;border-radius:3px 3px 0 0"></div></div>`,
			t.Format("02/01"), st.Views[d], st.Uniq[d], max1(h), u)
	}
	b.WriteString(`</div><p class="muted">Barra clara: visualizações. Barra escura: visitantes. Passe o mouse para ver o dia.</p>`)

	table := func(title string, m map[string]int, empty string) {
		fmt.Fprintf(&b, `<h2 style="font-size:1.1rem;margin:20px 0 8px">%s</h2>`, title)
		keys := top(m, 15)
		if len(keys) == 0 {
			b.WriteString(`<p class="muted">` + empty + `</p>`)
			return
		}
		b.WriteString(`<table>`)
		for _, k := range keys {
			fmt.Fprintf(&b, `<tr><td>%s</td><td style="text-align:right">%d</td></tr>`, html.EscapeString(k), m[k])
		}
		b.WriteString(`</table>`)
	}
	table("Páginas mais vistas", st.Pages, "Nenhuma visita ainda.")
	table("De onde vieram", st.Refs, "Nenhum visitante veio de outro site ainda.")
	b.WriteString(`<p class="muted" style="margin-top:20px">Sem cookies e sem dados pessoais: não guardamos IP nem identificamos ninguém. Robôs, quem pede para não ser rastreado (Do Not Track / GPC) e você, logado no painel, não entram na conta.</p>`)
	return a.out("Visitas", b.String(), true)
}

func max1(n int) int {
	if n < 1 {
		return 1
	}
	return n
}
