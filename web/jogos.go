package web

// Jogos de promoção: roleta de prêmios, raspadinha e jogo da memória, feitos
// com o Hilo (Alibaba, licença MIT; ver LICENCA-HILO.txt). Na página:
//
//	{{ jogo("roleta", {premios: ["10% de desconto", "Café grátis", {nome: "Não foi dessa vez", cupom: false}]}) }}
//	{{ jogo("raspadinha", {premios: [{nome: "Frete grátis", chance: 3}, "5% de desconto"]}) }}
//	{{ jogo("memoria", {premio: "Brigadeiro de presente", imagens: [...]}) }}
//
// Quem decide o prêmio é o servidor (o navegador só anima), cada contato e
// cada aparelho jogam uma vez por dia, e o cupom vai para a coleção "cupons"
// do painel, com nome e contato de quem aceitou receber as ofertas.

import (
	"crypto/rand"
	"crypto/sha256"
	_ "embed"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"html"
	"io"
	"math/big"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/Nanofylium/probable-plant/icones"
	"github.com/Nanofylium/probable-plant/lang"
)

//go:embed assets/hilo.js
var hiloJS []byte

//go:embed assets/jogos.js
var jogosJS []byte

type premioJogo struct {
	Nome   string `json:"nome"`
	Chance int    `json:"chance"`
	Cupom  bool   `json:"cupom"`
}

type confJogo struct {
	Tipo     string       `json:"tipo"`
	Titulo   string       `json:"titulo"`
	Texto    string       `json:"texto"`
	Botao    string       `json:"botao"`
	Premios  []premioJogo `json:"premios"`
	Imagens  []string     `json:"imagens,omitempty"`
	Contato  bool         `json:"contato"`
	Validade int          `json:"validade"`
	Prefixo  string       `json:"prefixo"`
}

var reCodigo = regexp.MustCompile(`[^A-Z0-9]`)

func lerConfJogo(tipo string, op *lang.Table) (*confJogo, error) {
	c := &confJogo{Tipo: strings.ToLower(strings.TrimSpace(tipo)), Contato: true, Validade: 7}
	switch c.Tipo {
	case "roleta", "raspadinha", "memoria":
	case "memória":
		c.Tipo = "memoria"
	default:
		return nil, fmt.Errorf("jogo: tipo %q não existe; use roleta, raspadinha ou memoria", tipo)
	}
	opc := func(k string) lang.Value {
		if op == nil {
			return nil
		}
		v, _ := op.Get(k)
		return v
	}
	c.Titulo = clip(strOrEmpty(opc("titulo")), 120)
	c.Texto = clip(strOrEmpty(opc("texto")), 400)
	c.Botao = clip(strOrEmpty(opc("botao")), 40)
	if b, ok := opc("contato").(bool); ok {
		c.Contato = b
	}
	if n, ok := toInt(opc("validade")); ok && n > 0 && n <= 365 {
		c.Validade = int(n)
	}
	c.Prefixo = reCodigo.ReplaceAllString(strings.ToUpper(strOrEmpty(opc("prefixo"))), "")
	if len(c.Prefixo) > 8 {
		c.Prefixo = c.Prefixo[:8]
	}
	add := func(v lang.Value) {
		p := premioJogo{Chance: 1, Cupom: true}
		switch x := v.(type) {
		case string:
			p.Nome = x
		case *lang.Table:
			p.Nome = getS(x, "nome")
			if n, ok := toInt(func() lang.Value { v, _ := x.Get("chance"); return v }()); ok && n >= 0 {
				p.Chance = int(min(n, 1000))
			}
			if b, ok := func() lang.Value { v, _ := x.Get("cupom"); return v }().(bool); ok {
				p.Cupom = b
			}
		}
		if p.Nome = clip(strings.TrimSpace(p.Nome), 60); p.Nome != "" && len(c.Premios) < 12 {
			c.Premios = append(c.Premios, p)
		}
	}
	if l, ok := opc("premios").(*lang.List); ok {
		for _, it := range l.Items {
			add(it)
		}
	}
	if p := opc("premio"); p != nil {
		add(p)
	}
	if l, ok := opc("imagens").(*lang.List); ok {
		for _, it := range l.Items {
			u := strOrEmpty(it)
			if t, ok := it.(*lang.Table); ok { // registro com foto: {foto: {url: ...}}
				if f, ok := func() lang.Value { v, _ := t.Get("foto"); return v }().(*lang.Table); ok {
					u = getS(f, "url")
				} else {
					u = getS(t, "url")
				}
			}
			if (strings.HasPrefix(u, "/") || strings.HasPrefix(u, "https://")) && len(c.Imagens) < 6 {
				c.Imagens = append(c.Imagens, u)
			}
		}
	}
	switch {
	case len(c.Premios) == 0:
		return nil, fmt.Errorf("jogo: diga o prêmio, ex.: {premios: [\"10%% de desconto\", \"Café grátis\"]}")
	case c.Tipo == "roleta" && len(c.Premios) < 2:
		return nil, fmt.Errorf("jogo: a roleta precisa de pelo menos 2 prêmios")
	}
	total := 0
	for _, p := range c.Premios {
		total += p.Chance
	}
	if total == 0 {
		return nil, fmt.Errorf("jogo: todas as chances estão em zero")
	}
	return c, nil
}

// id do jogo: o mesmo jogo na página dá sempre o mesmo id (e a mesma configuração guardada)
func (c *confJogo) id() string {
	b, _ := json.Marshal(c)
	h := sha256.Sum256(b)
	return hex.EncodeToString(h[:8])
}

// imagens padrão da memória: ícones do Lucide em SVG, na cor do site
func imagensPadrao() []string {
	var out []string
	for _, e := range []string{"☕", "🎁", "⭐", "❤️", "🌸", "🍰"} {
		s, _ := icones.SVG(e)
		s = strings.Replace(s, `stroke="currentColor"`, `stroke="#1d1d1d"`, 1)
		s = strings.Replace(s, `<svg `, `<svg xmlns="http://www.w3.org/2000/svg" width="96" height="96" `, 1)
		out = append(out, "data:image/svg+xml;charset=utf-8,"+strings.NewReplacer("#", "%23", `"`, "'", "<", "%3C", ">", "%3E").Replace(s))
	}
	return out
}

func (s *Server) jogoBuiltin(rq *request) func([]lang.Value) (lang.Value, error) {
	return func(a []lang.Value) (lang.Value, error) {
		op, _ := argv(a, 1).(*lang.Table)
		c, err := lerConfJogo(strOrEmpty(argv(a, 0)), op)
		if err != nil {
			return nil, err
		}
		id := c.id()
		b, _ := json.Marshal(c)
		if s.Store.Meta("jogo:"+id) != string(b) {
			_ = s.Store.SetMeta("jogo:"+id, string(b))
		}
		nomes := make([]string, len(c.Premios))
		for i, p := range c.Premios {
			nomes[i] = p.Nome
		}
		imgs := c.Imagens
		if c.Tipo == "memoria" && len(imgs) < 6 {
			imgs = append(imgs, imagensPadrao()[:6-len(imgs)]...)
		}
		pub := map[string]any{"id": id, "tipo": c.Tipo, "premios": nomes, "contato": c.Contato, "botao": c.Botao, "imagens": imgs}
		if c.Tipo != "roleta" {
			pub["premios"] = nil // a raspadinha e a memória não mostram a lista antes
		}
		j, _ := json.Marshal(pub)
		titulo := c.Titulo
		if titulo == "" {
			titulo = map[string]string{"roleta": "Gire a roleta e ganhe", "raspadinha": "Raspe e ganhe", "memoria": "Jogo da memória premiado"}[c.Tipo]
		}
		var h strings.Builder
		h.WriteString(`<div class="nep-jogo" data-jogo="` + html.EscapeString(string(j)) + `" data-csrf="` + html.EscapeString(rq.sess.CSRF) + `">`)
		h.WriteString(`<h3 class="nep-jogo-titulo">` + html.EscapeString(titulo) + `</h3>`)
		if c.Texto != "" {
			h.WriteString(`<p class="nep-jogo-texto">` + html.EscapeString(c.Texto) + `</p>`)
		}
		h.WriteString(`<noscript><p>Ative o JavaScript para jogar.</p></noscript></div>`)
		h.WriteString(`<script src="` + html.EscapeString(s.opt.Base) + `/_nep/jogos.js" data-hilo="` + html.EscapeString(s.opt.Base) + `/_nep/hilo.js" data-api="` + html.EscapeString(s.opt.Base) + `/_nep/jogo" defer></script>`)
		rq.saveSession()
		return lang.HTML(h.String()), nil
	}
}

func sortear(ps []premioJogo) int {
	total := 0
	for _, p := range ps {
		total += p.Chance
	}
	n, _ := rand.Int(rand.Reader, big.NewInt(int64(total)))
	x := int(n.Int64())
	for i, p := range ps {
		if x < p.Chance {
			return i
		}
		x -= p.Chance
	}
	return len(ps) - 1
}

func codigoCupom(prefixo string) string {
	const letras = "ABCDEFGHJKLMNPQRSTUVWXYZ23456789" // sem 0/O e 1/I
	b := make([]byte, 6)
	for i := range b {
		n, _ := rand.Int(rand.Reader, big.NewInt(int64(len(letras))))
		b[i] = letras[n.Int64()]
	}
	if prefixo == "" {
		prefixo = "NEP"
	}
	return prefixo + "-" + string(b)
}

var reContato = regexp.MustCompile(`^(\S+@\S+\.\S+|[\d\s()+-]{8,20})$`)

// jogoAPI: POST /_nep/jogo {id, acao: "jogar"|"concluir", nome, contato, aceite, token}
func (s *Server) jogoAPI(rq *request) {
	w, r := rq.w, rq.r
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	reply := func(code int, v map[string]any) {
		rq.saveSession()
		w.WriteHeader(code)
		_ = json.NewEncoder(w).Encode(v)
	}
	if r.Method != http.MethodPost || !rq.sess.csrfOK(r.Header.Get("X-Nep-Csrf")) {
		reply(403, map[string]any{"erro": "Recarregue a página e tente de novo."})
		return
	}
	var in struct {
		ID      string `json:"id"`
		Acao    string `json:"acao"`
		Nome    string `json:"nome"`
		Contato string `json:"contato"`
		Aceite  bool   `json:"aceite"`
		Token   string `json:"token"`
	}
	if json.NewDecoder(io.LimitReader(r.Body, 8<<10)).Decode(&in) != nil {
		reply(400, map[string]any{"erro": "Pedido inválido."})
		return
	}
	var c confJogo
	if len(in.ID) != 16 || json.Unmarshal([]byte(s.Store.Meta("jogo:"+in.ID)), &c) != nil {
		reply(404, map[string]any{"erro": "Este jogo não existe mais. Recarregue a página."})
		return
	}
	nome, contato := clip(strings.TrimSpace(in.Nome), 80), clip(strings.TrimSpace(in.Contato), 80)
	if c.Contato {
		if nome == "" || !reContato.MatchString(contato) {
			reply(400, map[string]any{"erro": "Diga seu nome e um WhatsApp ou e-mail para receber o cupom."})
			return
		}
		if !in.Aceite {
			reply(400, map[string]any{"erro": "Para receber o cupom, marque que aceita receber as ofertas."})
			return
		}
	}
	dia := int64(24 * 3600)
	switch in.Acao {
	case "jogar":
		// um jogo por aparelho (IP com sal, nunca gravado em claro) e por contato a cada dia
		if ok, _ := s.Store.Rate("jogo:"+in.ID+":"+s.ipKey(r), 1, dia, false); !ok {
			reply(429, map[string]any{"erro": "Você já jogou hoje. Volte amanhã!"})
			return
		}
		if contato != "" {
			k := "jogo:" + in.ID + ":c:" + strings.ToLower(strings.Join(strings.Fields(contato), ""))
			if ok, _ := s.Store.Rate(k, 1, dia, true); !ok {
				reply(429, map[string]any{"erro": "Este contato já jogou hoje. Volte amanhã!"})
				return
			}
		}
		_, _ = s.Store.Rate("jogo:"+in.ID+":"+s.ipKey(r), 1, dia, true)
		if c.Tipo == "memoria" {
			tok := randHex(16)
			_ = s.Store.SetMeta("jogo:tok:"+tok, in.ID+"|"+fmt.Sprint(time.Now().Unix()))
			reply(200, map[string]any{"token": tok})
			return
		}
		i := sortear(c.Premios)
		reply(200, s.darCupom(&c, in.ID, i, nome, contato))
	case "concluir": // memória: o servidor confere o token e o tempo mínimo de jogo
		v := s.Store.Meta("jogo:tok:" + in.Token)
		var inicio int64
		if len(in.Token) != 32 || !strings.HasPrefix(v, in.ID+"|") {
			reply(400, map[string]any{"erro": "Jogo não encontrado. Comece de novo."})
			return
		}
		fmt.Sscan(strings.TrimPrefix(v, in.ID+"|"), &inicio)
		if time.Now().Unix()-inicio < 6 {
			reply(400, map[string]any{"erro": "Rápido demais! Jogue de novo."})
			return
		}
		_ = s.Store.SetMeta("jogo:tok:"+in.Token, "")
		reply(200, s.darCupom(&c, in.ID, sortear(c.Premios), nome, contato))
	default:
		reply(400, map[string]any{"erro": "Ação inválida."})
	}
}

// darCupom grava o cupom na coleção "cupons" (aparece no painel).
func (s *Server) darCupom(c *confJogo, id string, i int, nome, contato string) map[string]any {
	p := c.Premios[i]
	out := map[string]any{"indice": i, "premio": p.Nome, "cupom": p.Cupom}
	if !p.Cupom {
		return out
	}
	cod := codigoCupom(c.Prefixo)
	ate := time.Now().AddDate(0, 0, c.Validade).Format("2006-01-02")
	t := lang.NewTable()
	t.SetStr("titulo", cod)
	t.SetStr("premio", p.Nome)
	t.SetStr("jogo", c.Tipo)
	t.SetStr("jogo_id", id)
	t.SetStr("nome", nome)
	t.SetStr("contato", contato)
	t.SetStr("valido_ate", ate)
	t.SetStr("usado", false)
	t.SetStr("publicado", false)
	if _, err := s.Store.Insert("cupons", t); err != nil {
		return map[string]any{"indice": i, "premio": p.Nome, "cupom": false, "erro": "Não consegui guardar o cupom. Tente de novo."}
	}
	out["codigo"], out["valido_ate"] = cod, ate
	return out
}
