package web

// O dono cuida do site pelo WhatsApp: manda "chegou bolo de cenoura, R$ 12"
// com uma foto e o item aparece no site; "acabou o bolo" tira; "amanhã fecho
// às 14h" põe um aviso no topo do site; "desfazer" volta o último passo.
//
// As mensagens chegam pela API oficial do WhatsApp (Cloud API da Meta), no
// endereço /_nep/whatsapp, com a assinatura conferida; só os números do dono
// são aceitos. No painel (/_admin/whatsapp) há um simulador para testar sem a
// Meta. Quem entende a mensagem são regras do Nepenthes; o que elas não
// entendem vai para a IA (do servidor ou a local), que só escolhe uma das
// mesmas ações.

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"html"
	"io"
	"math"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/Nanofylium/probable-plant/ialocal"
	"github.com/Nanofylium/probable-plant/lang"
)

const (
	metaZapConfig   = "whatsapp:config"
	metaZapConversa = "whatsapp:conversa"
	metaZapDesfazer = "whatsapp:desfazer"
	metaZapUltimo   = "whatsapp:ultimo"
	metaZapAviso    = "whatsapp:aviso"
	metaZapVistas   = "whatsapp:vistas"
)

// graphURL é o endereço da API da Meta (trocável nos testes).
var graphURL = "https://graph.facebook.com/v21.0"

type zapConfig struct {
	PhoneID   string   `json:"phone_id"`
	Token     string   `json:"token"`
	AppSecret string   `json:"app_secret"`
	Verify    string   `json:"verify"`
	Donos     []string `json:"donos"`
}

func (s *Server) zapConfig() zapConfig {
	var c zapConfig
	_ = json.Unmarshal([]byte(s.Store.Meta(metaZapConfig)), &c)
	return c
}

func soDigitos(x string) string {
	var b strings.Builder
	for _, r := range x {
		if r >= '0' && r <= '9' {
			b.WriteRune(r)
		}
	}
	return b.String()
}

// ehDono confere o número (com ou sem o 9 a mais dos celulares brasileiros).
func (c zapConfig) ehDono(de string) bool {
	d := soDigitos(de)
	for _, n := range c.Donos {
		n = soDigitos(n)
		if n == "" {
			continue
		}
		if n == d || semNove(n) == semNove(d) {
			return true
		}
	}
	return false
}

// semNove tira o nono dígito de celular brasileiro (55 + DDD + 9 + 8 dígitos):
// a Meta às vezes manda o número sem ele.
func semNove(n string) string {
	if len(n) == 13 && strings.HasPrefix(n, "55") && n[4] == '9' {
		return n[:4] + n[5:]
	}
	return n
}

// ---------- conversa (aparece no painel) ----------

type zapLinha struct {
	Quem  string `json:"quem"` // dono | site
	Texto string `json:"texto"`
	Foto  string `json:"foto,omitempty"`
	Em    string `json:"em"`
	Via   string `json:"via,omitempty"` // whatsapp | simulador
}

func (s *Server) zapConversa() []zapLinha {
	var ls []zapLinha
	_ = json.Unmarshal([]byte(s.Store.Meta(metaZapConversa)), &ls)
	return ls
}

func (s *Server) zapAnotar(l zapLinha) {
	ls := append(s.zapConversa(), l)
	if len(ls) > 60 {
		ls = ls[len(ls)-60:]
	}
	b, _ := json.Marshal(ls)
	_ = s.Store.SetMeta(metaZapConversa, string(b))
}

// ---------- entender a mensagem ----------

type acaoZap struct {
	Acao  string   `json:"acao"` // novo preco acabou voltou aviso tirar_aviso horario lista campo desfazer ajuda
	Item  string   `json:"item,omitempty"`
	Preco *float64 `json:"preco,omitempty"`
	Texto string   `json:"texto,omitempty"`
	Campo string   `json:"campo,omitempty"`
	Tipo  string   `json:"tipo,omitempty"`
}

var (
	rePrecoRS    = regexp.MustCompile(`(?i)r\$\s*(\d{1,7}(?:[.,]\d{3})*(?:[.,]\d{1,2})?)`)
	rePrecoReais = regexp.MustCompile(`(?i)\b(\d{1,7}(?:[.,]\d{1,2})?)\s*(?:reais|real|contos?)\b`)
	rePrecoFim   = regexp.MustCompile(`(?i)\b(?:por|a|agora|custa|custando|pre[çc]o|valor|de)\s*:?\s*(\d{1,7}(?:[.,]\d{1,2})?)\s*$`)
	reNovo       = regexp.MustCompile(`(?i)^(chegou|chegaram|novo|nova|novos|novas|novidade|temos|lan[çc]amento|hoje tem|tem hoje|adicionar|adiciona|cadastrar|cadastra|incluir|inclui|colocar|coloca|p[oõ]e)\b[\s:,-]*(.*)$`)
	reAcabou     = regexp.MustCompile(`(?i)^(acabou|acabaram|esgotou|esgotaram|esgotado|esgotada|tirar|tira|remover|remove|apagar|apaga|sem|n[aã]o tem mais)\b[\s:,-]*(.*)$`)
	reVoltou     = regexp.MustCompile(`(?i)^(voltou|voltaram|voltar|volta|tem de novo|repor|repus|reabasteci)\b[\s:,-]*(.*)$`)
	reAviso      = regexp.MustCompile(`(?i)^(aviso|avisar|avisa|recado|comunicado)\b[\s:,-]*(.*)$`)
	reTirarAviso = regexp.MustCompile(`(?i)^(tirar|tira|apagar|apaga|sem|remover)\s+(o\s+)?aviso\b`)
	reHorario    = regexp.MustCompile(`(?i)^hor[aá]rio\b[\s:,-]*(.*)$`)
	reFuncionam  = regexp.MustCompile(`(?i)\b(fech[oa]|fechad[oa]s?|fechamos|abr[oe]|abrimos|abriremos|fecharemos|feriado|folga|n[aã]o (vamos )?abrir|n[aã]o abre|n[aã]o abrimos|funcionamento|expediente)\b`)
	reCampo      = regexp.MustCompile(`^([\pL][\pL\d _-]{0,30}):\s*(.+)$`)
	reFimSolto   = regexp.MustCompile(`(?i)(\s+|,\s*)(agora|hoje|por|a|custa|custando|sai|saindo|cada|o|a|hoje cedo|fresquinho|fresquinhos|fresquinha|fresquinhas)$`)
	reMudaPreco  = regexp.MustCompile(`(?i)\b(subiu|subir|baixou|baixar|abaixou|passou|passa|mudou|vai custar|vai pra|vai para|t[aá] custando|est[aá] custando|reajust\w*)\b`)
	reMudaResto  = regexp.MustCompile(`(?i)\s+(subiu|subir|baixou|baixar|abaixou|passou|passa|mudou|vai|t[aá]|est[aá]|reajust\w*)(\s+(pra|para|a|custando|custar|o pre[çc]o))*\s*$`)
	reVerboIni   = regexp.MustCompile(`(?i)^(fiz|fizemos|tenho|temos|saiu|sa[ií]ram|acabei de fazer|preparei|preparamos|assei|assamos|hoje tem|tem)\s+`)
	reArtigoIni  = regexp.MustCompile(`(?i)^(o|a|os|as|um|uma|uns|umas|do|da|dos|das|de)\s+`)
	reSujeira    = regexp.MustCompile(`(?i)\s*(r\$\s*\d[\d.,]*|\b\d[\d.,]*\s*(reais|real|contos?)\b|\b(por|a|custa|custando|pre[çc]o|valor)\s*:?\s*\d[\d.,]*\s*$)`)
)

func numeroBR(x string) (float64, bool) {
	x = strings.TrimSpace(x)
	if strings.Count(x, ",") == 1 { // 1.234,50 ou 12,5
		x = strings.ReplaceAll(x, ".", "")
		x = strings.Replace(x, ",", ".", 1)
	} else if strings.Count(x, ".") > 1 {
		x = strings.ReplaceAll(x, ".", "")
	}
	f, err := strconv.ParseFloat(x, 64)
	return f, err == nil && !math.IsNaN(f) && f >= 0
}

var rePrecoNu = regexp.MustCompile(`\s(\d{1,7}(?:[.,]\d{1,2})?)\s*$`)

// achaPreco acha "R$ 12", "12 reais" ou "por 12"; comNu aceita também um
// número solto no fim ("coxinha 6,50"), quando já se sabe que é produto.
func achaPrecoNu(t string) *float64 {
	if p := achaPreco(t); p != nil {
		return p
	}
	if m := rePrecoNu.FindStringSubmatch(" " + t); m != nil {
		if f, ok := numeroBR(m[1]); ok {
			return &f
		}
	}
	return nil
}

func achaPreco(t string) *float64 {
	for _, re := range []*regexp.Regexp{rePrecoRS, rePrecoReais, rePrecoFim} {
		if m := re.FindStringSubmatch(t); m != nil {
			if f, ok := numeroBR(m[1]); ok {
				return &f
			}
		}
	}
	return nil
}

// limparNome tira preço, artigos e pontuação: "o bolo de cenoura, R$ 12" → "Bolo de cenoura".
func limparNome(t string) string {
	t = reSujeira.ReplaceAllString(t, "")
	t = rePrecoNu.ReplaceAllString(" "+t, "")
	t = strings.Trim(strings.TrimSpace(t), " ,.;:!-–—")
	t = reFimSolto.ReplaceAllString(t, "")
	t = reMudaResto.ReplaceAllString(t, "")
	t = strings.Trim(strings.TrimSpace(t), " ,.;:!-–—")
	for i := 0; i < 3; i++ {
		t = reVerboIni.ReplaceAllString(t, "")
		t = reArtigoIni.ReplaceAllString(t, "")
		t = strings.Trim(reFimSolto.ReplaceAllString(t, ""), " ,.;:!-–—")
	}
	t = strings.TrimSpace(t)
	if utf8.RuneCountInString(t) > 80 {
		t = string([]rune(t)[:80])
	}
	return maiuscula(t)
}

// entenderZap são as regras: rápidas, sem internet, previsíveis.
func entenderZap(texto string, temFoto bool) acaoZap {
	t := strings.TrimSpace(texto)
	l := strings.ToLower(strings.Trim(t, " .!?"))
	switch l {
	case "", "ajuda", "menu", "oi", "olá", "ola", "bom dia", "boa tarde", "boa noite", "comandos", "?":
		if temFoto {
			break
		}
		return acaoZap{Acao: "ajuda"}
	case "desfazer", "desfaz", "desfaça", "volta", "voltar atrás", "voltar atras", "cancela", "cancelar":
		return acaoZap{Acao: "desfazer"}
	case "lista", "listar", "o que tem", "o que tem no site", "itens", "produtos", "cardápio", "cardapio":
		return acaoZap{Acao: "lista"}
	}
	if reTirarAviso.MatchString(l) {
		return acaoZap{Acao: "tirar_aviso"}
	}
	if m := reAviso.FindStringSubmatch(t); m != nil {
		return acaoZap{Acao: "aviso", Texto: strings.TrimSpace(m[2])}
	}
	if m := reHorario.FindStringSubmatch(t); m != nil && strings.TrimSpace(m[1]) != "" {
		return acaoZap{Acao: "horario", Texto: strings.TrimSpace(m[1])}
	}
	if m := reAcabou.FindStringSubmatch(t); m != nil && strings.TrimSpace(m[2]) != "" {
		return acaoZap{Acao: "acabou", Item: limparNome(m[2])}
	}
	if m := reVoltou.FindStringSubmatch(t); m != nil && strings.TrimSpace(m[2]) != "" {
		return acaoZap{Acao: "voltou", Item: limparNome(m[2])}
	}
	if regexp.MustCompile(`(?i)\bpromo[çc](ão|ões|ao|oes)\b|\bdesconto\b`).MatchString(l) && !temFoto {
		return acaoZap{Acao: "aviso", Texto: t}
	}
	if m := reNovo.FindStringSubmatch(t); m != nil && strings.TrimSpace(m[2]) != "" {
		return acaoZap{Acao: "novo", Item: limparNome(m[2]), Preco: achaPrecoNu(m[2]), Texto: t}
	}
	if m := reCampo.FindStringSubmatch(t); m != nil && !temFoto {
		return acaoZap{Acao: "campo", Campo: strings.TrimSpace(m[1]), Texto: strings.TrimSpace(m[2])}
	}
	if reFuncionam.MatchString(l) && !temFoto {
		return acaoZap{Acao: "aviso", Texto: t}
	}
	p := achaPreco(t)
	if temFoto {
		return acaoZap{Acao: "novo", Item: limparNome(t), Preco: achaPrecoNu(t), Texto: t}
	}
	if p != nil && limparNome(t) != "" && reMudaPreco.MatchString(t) {
		return acaoZap{Acao: "preco", Item: limparNome(t), Preco: p, Texto: t}
	}
	if p != nil && limparNome(t) != "" {
		// com preço: muda o preço se o item existe; senão, cria (decidido ao executar)
		return acaoZap{Acao: "preco_ou_novo", Item: limparNome(t), Preco: p, Texto: t}
	}
	return acaoZap{}
}

var schemaZap = map[string]any{"type": "object", "required": []string{"acao"}, "properties": map[string]any{
	"acao":  map[string]any{"type": "string", "enum": []string{"novo", "preco", "acabou", "voltou", "aviso", "tirar_aviso", "horario", "lista", "desfazer", "ajuda"}},
	"item":  map[string]any{"type": "string"},
	"preco": map[string]any{"type": "number"},
	"texto": map[string]any{"type": "string"},
}}

const promptZap = `Você traduz mensagens de WhatsApp do dono de um pequeno negócio em UMA ação no site dele. Responda só com JSON: {"acao":..., "item":..., "preco":..., "texto":...}.
Ações:
- novo: produto/serviço novo (item = nome curto, preco = número se houver)
- preco: mudou o preço de um item (item, preco)
- acabou: item esgotou ou saiu (item)
- voltou: item voltou (item)
- aviso: recado para os clientes no topo do site, como horário especial, feriado, promoção do dia (texto = o recado, escrito para o cliente, curto)
- tirar_aviso, horario (texto = horário de funcionamento novo), lista, desfazer
- ajuda: quando não der para saber
Exemplos: "fiz pão de queijo hoje, 3 reais cada" -> {"acao":"novo","item":"Pão de queijo","preco":3}
"o brigadeiro subiu pra 5" -> {"acao":"preco","item":"Brigadeiro","preco":5}
"segunda é feriado, só abro terça" -> {"acao":"aviso","texto":"Segunda é feriado: voltamos na terça."}`

func (s *Server) iaDisponivel() string {
	if s.iaConf().ready() {
		return "servidor"
	}
	if !s.opt.CGI {
		switch ialocal.Ver().Fase {
		case "pronto", "ligado":
			return "local"
		}
	}
	return ""
}

// zapIA pede à IA para escolher a ação (só quando as regras não entenderam).
func (s *Server) zapIA(rq *request, texto string) (acaoZap, bool) {
	var txt string
	var err error
	switch s.iaDisponivel() {
	case "servidor":
		txt, err = s.ask("whatsapp", promptZap, texto, 200, 0.1)
	case "local":
		txt, err = s.askLocal(rq, schemaZap, promptZap, texto, 200, 0.1)
	default:
		return acaoZap{}, false
	}
	if err != nil {
		return acaoZap{}, false
	}
	var a acaoZap
	m := reJSONObj.FindString(txt)
	if m == "" || json.Unmarshal([]byte(m), &a) != nil || a.Acao == "" {
		return acaoZap{}, false
	}
	a.Item = limparNome(a.Item)
	a.Texto = strings.TrimSpace(a.Texto)
	return a, true
}

// ---------- executar ----------

type zapDesfazer struct {
	Op    string `json:"op"` // criar | editar | aviso | texto
	Col   string `json:"col,omitempty"`
	ID    int64  `json:"id,omitempty"`
	Antes string `json:"antes,omitempty"` // JSON dos campos mudados (editar) ou aviso/texto anterior
	Chave string `json:"chave,omitempty"`
	Diz   string `json:"diz"`
}

func (s *Server) zapEmpilhar(d zapDesfazer) {
	var ds []zapDesfazer
	_ = json.Unmarshal([]byte(s.Store.Meta(metaZapDesfazer)), &ds)
	ds = append(ds, d)
	if len(ds) > 20 {
		ds = ds[len(ds)-20:]
	}
	b, _ := json.Marshal(ds)
	_ = s.Store.SetMeta(metaZapDesfazer, string(b))
}

// campos do tipo que o WhatsApp usa
type camposZap struct {
	titulo, preco, foto, texto string
}

func camposDoTipo(c *Content) camposZap {
	var z camposZap
	z.titulo = campoTitulo(c)
	for _, f := range c.Fields {
		n := strings.ToLower(f.Name + " " + f.Label)
		switch {
		case z.preco == "" && (f.Type == "number" || f.Type == "range") && regexp.MustCompile(`pre[çc]o|valor|price`).MatchString(n):
			z.preco = f.Name
		case z.foto == "" && f.Type == "file" && (f.Accept == "" || strings.Contains(f.Accept, "image")):
			z.foto = f.Name
		case z.texto == "" && f.Type == "textarea" && regexp.MustCompile(`resumo|descri|chamada|corpo|texto`).MatchString(n):
			z.texto = f.Name
		}
	}
	return z
}

// tipoPadrao: o tipo pedido pelo nome; senão o último usado; senão o que tem preço e foto.
func (s *Server) tipoZap(site *Site, pedido string) *Content {
	if pedido != "" {
		p := slugify(pedido)
		for _, c := range site.Contents {
			if c.Name == pedido || slugify(c.Title) == p || slugify(c.Item) == p || slugify(plural(c.Item)) == p {
				return c
			}
		}
	}
	var u struct{ Col string }
	_ = json.Unmarshal([]byte(s.Store.Meta(metaZapUltimo)), &u)
	if c := site.Content(u.Col); c != nil {
		return c
	}
	var melhor *Content
	nota := -1
	for _, c := range site.Contents {
		z := camposDoTipo(c)
		n := 0
		if z.preco != "" {
			n += 2
		}
		if z.foto != "" {
			n++
		}
		if n > nota {
			melhor, nota = c, n
		}
	}
	return melhor
}

// acharItem procura pelo nome em todos os tipos (o nome mais parecido ganha).
func (s *Server) acharItem(site *Site, nome string) (*Content, *lang.Table) {
	c, t, _ := s.acharItemNota(site, nome)
	return c, t
}

// acharItemNota diz também se o nome bateu bem: exato, ou cobrindo pelo
// menos metade do nome do item ("leite" não basta para "Pudim de leite").
func (s *Server) acharItemNota(site *Site, nome string) (*Content, *lang.Table, bool) {
	q := slugify(nome)
	if q == "" {
		return nil, nil, false
	}
	var bc *Content
	var bt *lang.Table
	best := 0
	for _, c := range site.Contents {
		for _, r := range s.Store.All(c.Name, 0) {
			t := r.ToValue()
			ts := slugify(itemTitle(c, t))
			if ts == "" {
				continue
			}
			n := 0
			switch {
			case ts == q:
				n = 1000
			case strings.Contains(ts, q):
				n = 500 + len(q)
			case strings.Contains(q, ts):
				n = 400 + len(ts)
			default:
				comuns := 0
				for _, w := range strings.Split(q, "-") {
					if len(w) > 2 && strings.Contains("-"+ts+"-", "-"+w+"-") {
						comuns += len(w)
					}
				}
				if comuns >= 4 {
					n = comuns
				}
			}
			if n > best {
				best, bc, bt = n, c, t
			}
		}
	}
	certo := false
	if bt != nil {
		ts := slugify(itemTitle(bc, bt))
		certo = best == 1000 || (strings.Contains(q, ts)) || float64(len(q)) >= 0.5*float64(len(ts))
	}
	return bc, bt, certo
}

func brl(f float64) string {
	s := strconv.FormatFloat(f, 'f', 2, 64)
	inteiro, cent := s[:len(s)-3], s[len(s)-2:]
	var b strings.Builder
	for i, r := range inteiro {
		if i > 0 && (len(inteiro)-i)%3 == 0 {
			b.WriteByte('.')
		}
		b.WriteRune(r)
	}
	return "R$ " + b.String() + "," + cent
}

func (s *Server) linkItem(rq *request, site *Site, c *Content, t *lang.Table) string {
	u := site.itemURL(c.Name, t)
	if u == "" {
		return ""
	}
	if rq.site == nil {
		rq.site = site
	}
	return s.siteURL(rq) + u
}

const ajudaZap = `Olá! Eu cuido do seu site. Você pode mandar:
• *chegou bolo de cenoura, R$ 12* (com foto, se quiser): coloco no site
• *brigadeiro agora R$ 5*: mudo o preço
• *acabou o bolo de cenoura* / *voltou o bolo de cenoura*
• *amanhã fecho às 14h* ou *aviso: promoção de café em dobro*: aparece no topo do site
• *tirar aviso*, *horário: seg a sáb, 7h às 19h*, *lista*
• *desfazer*: volto o último passo`

// zapExecutar faz a ação e devolve a resposta para o dono.
func (s *Server) zapExecutar(rq *request, site *Site, a acaoZap, foto []byte, nomeFoto string) string {
	switch a.Acao {
	case "ajuda", "":
		return ajudaZap
	case "desfazer":
		return s.zapDesfazer(site)
	case "lista":
		c := s.tipoZap(site, a.Tipo)
		if c == nil {
			return "O site ainda não tem lista de produtos ou serviços."
		}
		z := camposDoTipo(c)
		var b strings.Builder
		b.WriteString("*" + c.Title + "*")
		n := 0
		for _, r := range s.Store.All(c.Name, 15) {
			t := r.ToValue()
			b.WriteString("\n• " + itemTitle(c, t))
			if z.preco != "" {
				if f, ok := numeroDe(getV(t, z.preco)); ok {
					b.WriteString(" — " + brl(f))
				}
			}
			if !published(t) {
				b.WriteString(" (fora do site)")
			}
			n++
		}
		if n == 0 {
			b.WriteString("\nNada ainda. Mande *chegou* + o nome.")
		}
		return b.String()
	case "aviso":
		if a.Texto == "" {
			return "Qual é o aviso? Ex.: *aviso: hoje fechamos às 14h*"
		}
		antes := s.Store.Meta(metaZapAviso)
		ate := fimDoDia(time.Now().AddDate(0, 0, 1))
		l := strings.ToLower(a.Texto)
		if strings.Contains(l, "hoje") {
			ate = fimDoDia(time.Now())
		} else if strings.Contains(l, "semana") {
			ate = fimDoDia(time.Now().AddDate(0, 0, 7))
		} else if !strings.Contains(l, "amanh") {
			ate = fimDoDia(time.Now().AddDate(0, 0, 2))
		}
		txt := maiuscula(strings.TrimSpace(a.Texto))
		if utf8.RuneCountInString(txt) > 200 {
			txt = string([]rune(txt)[:200])
		}
		b, _ := json.Marshal(map[string]string{"texto": txt, "ate": ate.Format(time.RFC3339)})
		if err := s.Store.SetMeta(metaZapAviso, string(b)); err != nil {
			return "Não consegui gravar o aviso: " + err.Error()
		}
		s.zapEmpilhar(zapDesfazer{Op: "aviso", Antes: antes, Diz: "o aviso"})
		return "📢 Aviso no topo do site até " + ate.Format("02/01 15:04") + ":\n“" + txt + "”\nPara tirar antes: *tirar aviso*"
	case "tirar_aviso":
		antes := s.Store.Meta(metaZapAviso)
		if antes == "" {
			return "O site não tem aviso agora."
		}
		_ = s.Store.SetMeta(metaZapAviso, "")
		s.zapEmpilhar(zapDesfazer{Op: "aviso", Antes: antes, Diz: "a retirada do aviso"})
		return "Aviso retirado do site."
	case "horario":
		return s.zapHorario(site, a.Texto)
	case "campo":
		return s.zapCampo(rq, site, a.Campo, a.Texto)
	case "acabou", "voltou":
		c, t, certo := s.acharItemNota(site, a.Item)
		if t == nil {
			return "Não achei “" + a.Item + "” no site. Mande *lista* para ver os nomes."
		}
		if !certo {
			return "Você quis dizer *" + itemTitle(c, t) + "*? Se for, mande: *" + a.Acao + " " + strings.ToLower(itemTitle(c, t)) + "*"
		}
		id, _ := toInt(getV(t, "id"))
		pub := a.Acao == "voltou"
		antes, _ := json.Marshal(map[string]any{"publicado": getV(t, "publicado") != false})
		d := lang.NewTable()
		d.SetStr("publicado", pub)
		if _, err := s.Store.Update(c.Name, id, d); err != nil {
			return "Não consegui mudar: " + err.Error()
		}
		s.zapEmpilhar(zapDesfazer{Op: "editar", Col: c.Name, ID: id, Antes: string(antes), Diz: itemTitle(c, t)})
		if pub {
			return "✅ *" + itemTitle(c, t) + "* voltou para o site."
		}
		return "Tirei *" + itemTitle(c, t) + "* do site (fica guardado: mande *voltou " + strings.ToLower(itemTitle(c, t)) + "* quando tiver de novo)."
	case "preco", "preco_ou_novo":
		c, t, certo := s.acharItemNota(site, a.Item)
		if t != nil && !certo {
			if a.Acao == "preco" && a.Preco != nil {
				return "Você quis dizer *" + itemTitle(c, t) + "*? Se for, mande: *" + strings.ToLower(itemTitle(c, t)) + " agora " + brl(*a.Preco) + "*"
			}
			t = nil
		}
		if t == nil || a.Preco == nil {
			if a.Acao == "preco_ou_novo" {
				a.Acao = "novo"
				return s.zapExecutar(rq, site, a, foto, nomeFoto)
			}
			return "Não achei “" + a.Item + "” no site. Mande *lista* para ver os nomes."
		}
		z := camposDoTipo(c)
		if z.preco == "" {
			return "“" + c.Title + "” não tem campo de preço."
		}
		id, _ := toInt(getV(t, "id"))
		antes, _ := json.Marshal(map[string]any{z.preco: getV(t, z.preco)})
		d := lang.NewTable()
		d.SetStr(z.preco, numeroZap(*a.Preco))
		if _, err := s.Store.Update(c.Name, id, d); err != nil {
			return "Não consegui mudar: " + err.Error()
		}
		s.zapEmpilhar(zapDesfazer{Op: "editar", Col: c.Name, ID: id, Antes: string(antes), Diz: "o preço de " + itemTitle(c, t)})
		return "✅ *" + itemTitle(c, t) + "* agora custa " + brl(*a.Preco) + "."
	case "novo":
		return s.zapNovo(rq, site, a, foto, nomeFoto)
	}
	return ajudaZap
}

func numeroZap(f float64) lang.Value {
	if f == math.Trunc(f) {
		return int64(f)
	}
	return f
}

func fimDoDia(t time.Time) time.Time {
	y, m, d := t.Date()
	return time.Date(y, m, d, 23, 59, 0, 0, t.Location())
}

func (s *Server) zapNovo(rq *request, site *Site, a acaoZap, foto []byte, nomeFoto string) string {
	c := s.tipoZap(site, a.Tipo)
	if c == nil {
		return "O site ainda não tem onde guardar produtos ou serviços. Crie um tipo no painel (Tipos com IA) e mande de novo."
	}
	if a.Item == "" {
		if len(foto) > 0 {
			return "Recebi a foto. Qual é o nome? Mande a foto de novo com o nome na legenda, ex.: *Bolo de cenoura R$ 12*"
		}
		return "Qual é o nome? Ex.: *chegou bolo de cenoura, R$ 12*"
	}
	z := camposDoTipo(c)
	data := lang.NewTable()
	data.SetStr(z.titulo, a.Item)
	if a.Preco != nil && z.preco != "" {
		data.SetStr(z.preco, numeroZap(*a.Preco))
	}
	var aviso []string
	if len(foto) > 0 {
		if z.foto == "" {
			aviso = append(aviso, "este tipo não tem foto, então a foto ficou de fora")
		} else if img, err := lerImagem(bytes.NewReader(foto)); err != nil {
			aviso = append(aviso, "não consegui ler a foto ("+err.Error()+")")
		} else if f, err := s.gravarImagem(img, false, nomeFoto); err != nil {
			aviso = append(aviso, "não consegui gravar a foto")
		} else {
			ft := lang.NewTable()
			ft.SetStr("nome", f.Nome)
			ft.SetStr("tipo", f.Tipo)
			ft.SetStr("tamanho", f.Tamanho)
			ft.SetStr("url", s.opt.Base+f.URL)
			data.SetStr(z.foto, ft)
		}
	}
	// campos obrigatórios que faltam: o item fica fora do site até o dono responder
	var faltam []string
	for _, f := range c.Fields {
		if !f.Required || f.Type == "checkbox" {
			continue
		}
		if v := getV(data, f.Name); v != nil && lang.Str(v) != "" {
			continue
		}
		if f.Options != nil && len(f.Options) > 0 && f.Type != "file" {
			continue // escolha: o dono responde com uma das opções
		}
		faltam = append(faltam, f.Label)
	}
	for _, f := range c.Fields {
		if f.Required && f.Options != nil && getV(data, f.Name) == nil {
			faltam = append(faltam, f.Label+" ("+strings.Join(opcoesSemVazio(f.Options), ", ")+")")
		}
	}
	data.SetStr("slug", s.uniqueSlug(c.Name, slugify(a.Item), 0))
	data.SetStr("publicado", len(faltam) == 0)
	id, err := s.Store.Insert(c.Name, data)
	if err != nil {
		return "Não consegui guardar: " + err.Error()
	}
	u, _ := json.Marshal(map[string]any{"col": c.Name, "id": id})
	_ = s.Store.SetMeta(metaZapUltimo, string(u))
	s.zapEmpilhar(zapDesfazer{Op: "criar", Col: c.Name, ID: id, Diz: a.Item})
	var b strings.Builder
	if len(faltam) > 0 {
		b.WriteString("Guardei *" + a.Item + "*, mas ainda fora do site: falta " + strings.Join(faltam, " e ") + ".\nResponda, por exemplo: *" + strings.ToLower(strings.SplitN(faltam[0], " (", 2)[0]) + ": …*")
	} else {
		b.WriteString("✅ Coloquei *" + a.Item + "*")
		if a.Preco != nil && z.preco != "" {
			b.WriteString(" (" + brl(*a.Preco) + ")")
		}
		b.WriteString(" em " + c.Title + ".")
		if r := s.Store.Find(c.Name, id); r != nil {
			if l := s.linkItem(rq, site, c, r.ToValue()); l != "" {
				b.WriteString("\n" + l)
			}
		}
	}
	if a.Preco == nil && z.preco != "" {
		b.WriteString("\nSem preço. Para pôr: *preço: 12*")
	}
	for _, x := range aviso {
		b.WriteString("\n(" + x + ")")
	}
	b.WriteString("\nErrou? Mande *desfazer*.")
	return b.String()
}

func opcoesSemVazio(o []string) []string {
	var out []string
	for _, x := range o {
		if x != "" {
			out = append(out, x)
		}
	}
	return out
}

// zapCampo: "categoria: Bolos" muda um campo do último item criado.
func (s *Server) zapCampo(rq *request, site *Site, campo, valor string) string {
	var u struct {
		Col string
		ID  int64
	}
	_ = json.Unmarshal([]byte(s.Store.Meta(metaZapUltimo)), &u)
	c := site.Content(u.Col)
	var rec *Record
	if c != nil {
		rec = s.Store.Find(c.Name, u.ID)
	}
	if rec == nil {
		return ajudaZap
	}
	cs := slugify(campo)
	var f *Field
	for _, x := range c.Fields {
		if slugify(x.Name) == cs || slugify(x.Label) == cs || strings.HasPrefix(slugify(x.Label), cs) {
			f = x
			break
		}
	}
	if cs == "preco" || cs == "valor" {
		if p := camposDoTipo(c).preco; p != "" {
			for _, x := range c.Fields {
				if x.Name == p {
					f = x
				}
			}
		}
	}
	if f == nil || f.Type == "file" {
		return "“" + campo + "” não é um campo de " + c.Title + ". " + "Campos: " + nomesCampos(c) + "."
	}
	t := rec.ToValue()
	var v lang.Value = strings.TrimSpace(valor)
	switch f.Type {
	case "number", "range":
		p := achaPreco(valor)
		if p == nil {
			if x, ok := numeroBR(regexp.MustCompile(`[\d.,]+`).FindString(valor)); ok {
				p = &x
			}
		}
		if p == nil {
			return "“" + f.Label + "” precisa de um número."
		}
		v = numeroZap(*p)
	case "checkbox":
		l := strings.ToLower(valor)
		v = l == "sim" || l == "s" || l == "yes" || l == "1"
	}
	if f.Options != nil {
		achou := ""
		for _, o := range f.Options {
			if o != "" && slugify(o) == slugify(valor) {
				achou = o
			}
		}
		if achou == "" {
			return "Em “" + f.Label + "” vale: " + strings.Join(opcoesSemVazio(f.Options), ", ") + "."
		}
		v = achou
	}
	antes, _ := json.Marshal(map[string]any{f.Name: getV(t, f.Name), "publicado": getV(t, "publicado")})
	d := lang.NewTable()
	d.SetStr(f.Name, v)
	_ = t.Set(f.Name, v)
	pronto := true
	for _, x := range c.Fields {
		if x.Required && x.Type != "checkbox" && (getV(t, x.Name) == nil || lang.Str(getV(t, x.Name)) == "") {
			pronto = false
		}
	}
	if pronto && getV(t, "publicado") == false {
		d.SetStr("publicado", true)
	}
	if _, err := s.Store.Update(c.Name, u.ID, d); err != nil {
		return "Não consegui mudar: " + err.Error()
	}
	s.zapEmpilhar(zapDesfazer{Op: "editar", Col: c.Name, ID: u.ID, Antes: string(antes), Diz: f.Label + " de " + itemTitle(c, t)})
	msg := "✅ " + f.Label + " de *" + itemTitle(c, t) + "*: " + lang.Str(v) + "."
	if d.Len() > 1 {
		msg += " Agora está no site."
		if l := s.linkItem(rq, site, c, t); l != "" {
			msg += "\n" + l
		}
	} else if !pronto {
		msg += " Ainda falta preencher algo para ir ao site."
	}
	return msg
}

func nomesCampos(c *Content) string {
	var n []string
	for _, f := range c.Fields {
		if f.Type != "file" {
			n = append(n, strings.ToLower(f.Label))
		}
	}
	return strings.Join(n, ", ")
}

// zapHorario muda o horário onde o site guarda (texto editável ou global).
func (s *Server) zapHorario(site *Site, txt string) string {
	if txt == "" {
		return "Qual é o horário? Ex.: *horário: seg a sáb, 7h às 19h*"
	}
	mudou := 0
	for _, e := range site.Editables {
		if strings.Contains(e.Key, "horario") || strings.Contains(e.Key, "horário") {
			antes, _ := s.edGet(e.Key)
			if err := s.edSet(e.Key, txt); err == nil {
				s.zapEmpilhar(zapDesfazer{Op: "texto", Chave: e.Key, Antes: antes, Diz: "o horário"})
				mudou++
			}
		}
	}
	for _, g := range site.Globals {
		for _, f := range g.Fields {
			if strings.Contains(f.Name, "horario") {
				cur := s.globalValor(g, true)
				antes := s.Store.Meta(metaGlobal(g.Name))
				cur.SetStr(f.Name, txt)
				cur.SetStr("updated_at", stamp())
				j, _ := lang.ToJSON(cur, "")
				if s.Store.SetMeta(metaGlobal(g.Name), j) == nil {
					s.zapEmpilhar(zapDesfazer{Op: "global", Chave: g.Name, Antes: antes, Diz: "o horário"})
					mudou++
				}
			}
		}
	}
	if mudou == 0 {
		return "Este site não tem um lugar para horário. Posso pôr como aviso: mande *aviso: " + txt + "*"
	}
	return "✅ Horário atualizado no site: " + txt
}

func (s *Server) zapDesfazer(site *Site) string {
	var ds []zapDesfazer
	_ = json.Unmarshal([]byte(s.Store.Meta(metaZapDesfazer)), &ds)
	if len(ds) == 0 {
		return "Não há nada para desfazer."
	}
	d := ds[len(ds)-1]
	ds = ds[:len(ds)-1]
	b, _ := json.Marshal(ds)
	_ = s.Store.SetMeta(metaZapDesfazer, string(b))
	switch d.Op {
	case "criar":
		if err := s.trash(d.Col, d.ID, "whatsapp"); err != nil {
			return "Não consegui desfazer: " + err.Error()
		}
		return "Desfeito: tirei *" + d.Diz + "* (está na lixeira do painel)."
	case "editar":
		v, err := lang.FromJSON(d.Antes)
		t, ok := v.(*lang.Table)
		if err != nil || !ok {
			return "Não consegui desfazer."
		}
		if _, err := s.Store.Update(d.Col, d.ID, t); err != nil {
			return "Não consegui desfazer: " + err.Error()
		}
		return "Desfeito: voltei " + d.Diz + " como estava."
	case "aviso":
		_ = s.Store.SetMeta(metaZapAviso, d.Antes)
		return "Desfeito: voltei " + d.Diz + "."
	case "texto":
		if d.Antes == "" {
			_ = s.edReset(d.Chave)
		} else {
			_ = s.edSet(d.Chave, d.Antes)
		}
		return "Desfeito: voltei " + d.Diz + "."
	case "global":
		_ = s.Store.SetMeta(metaGlobal(d.Chave), d.Antes)
		return "Desfeito: voltei " + d.Diz + "."
	}
	return "Não consegui desfazer."
}

// zapMensagem é o caminho de toda mensagem do dono (WhatsApp ou simulador).
func (s *Server) zapMensagem(rq *request, site *Site, via, texto string, foto []byte, nomeFoto string) string {
	s.zapAnotar(zapLinha{Quem: "dono", Texto: texto, Em: stamp(), Via: via, Foto: map[bool]string{true: "📷", false: ""}[len(foto) > 0]})
	a := entenderZap(texto, len(foto) > 0)
	if a.Acao == "" || (a.Acao == "preco_ou_novo" && len(strings.Fields(a.Item)) > 4) {
		if ia, ok := s.zapIA(rq, texto); ok {
			a = ia
		}
	}
	resp := s.zapExecutar(rq, site, a, foto, nomeFoto)
	s.zapAnotar(zapLinha{Quem: "site", Texto: resp, Em: stamp(), Via: via})
	return resp
}

// ---------- aviso no topo do site ----------

func (s *Server) avisoAtual() string {
	var av struct{ Texto, Ate string }
	if json.Unmarshal([]byte(s.Store.Meta(metaZapAviso)), &av) != nil || av.Texto == "" {
		return ""
	}
	if t, err := time.Parse(time.RFC3339, av.Ate); err == nil && time.Now().After(t) {
		return ""
	}
	return av.Texto
}

var reBodyAbre = regexp.MustCompile(`(?i)<body\b[^>]*>`)

func (s *Server) comAviso(out string) string {
	av := s.avisoAtual()
	if av == "" {
		return out
	}
	loc := reBodyAbre.FindStringIndex(out)
	if loc == nil {
		return out
	}
	barra := `<div class="nep-aviso" role="status" style="position:relative;z-index:60;background:#1f6f43;color:#fff;text-align:center;padding:10px 44px 10px 16px;font:600 15px/1.4 system-ui,-apple-system,'Segoe UI',Roboto,sans-serif">` +
		`📢 ` + html.EscapeString(av) +
		`<button type="button" aria-label="Fechar aviso" onclick="this.parentNode.remove()" style="all:unset;position:absolute;right:10px;top:50%;transform:translateY(-50%);background:none;border:0;color:#fff;font-size:20px;line-height:1;cursor:pointer">×</button></div>`
	return out[:loc[1]] + barra + out[loc[1]:]
}

// ---------- WhatsApp oficial (Cloud API da Meta) ----------

type zapEntrada struct {
	Entry []struct {
		Changes []struct {
			Value struct {
				Messages []struct {
					ID    string                `json:"id"`
					From  string                `json:"from"`
					Type  string                `json:"type"`
					Text  struct{ Body string } `json:"text"`
					Image struct {
						ID      string `json:"id"`
						Caption string `json:"caption"`
					} `json:"image"`
				} `json:"messages"`
			} `json:"value"`
		} `json:"changes"`
	} `json:"entry"`
}

// zapWebhook é /_nep/whatsapp: a Meta confere o endereço (GET) e manda as mensagens (POST).
func (s *Server) zapWebhook(rq *request) {
	w, r := rq.w, rq.r
	w.Header().Set("Cache-Control", "no-store")
	cfg := s.zapConfig()
	if r.Method == http.MethodGet {
		q := r.URL.Query()
		if cfg.Verify != "" && q.Get("hub.mode") == "subscribe" && hmac.Equal([]byte(q.Get("hub.verify_token")), []byte(cfg.Verify)) {
			io.WriteString(w, q.Get("hub.challenge"))
			return
		}
		http.Error(w, "não", http.StatusForbidden)
		return
	}
	if r.Method != http.MethodPost {
		http.Error(w, "use POST", http.StatusMethodNotAllowed)
		return
	}
	body, err := io.ReadAll(io.LimitReader(r.Body, 1<<20))
	if err != nil {
		http.Error(w, "erro", http.StatusBadRequest)
		return
	}
	if cfg.AppSecret == "" || !assinaturaOK(body, r.Header.Get("X-Hub-Signature-256"), cfg.AppSecret) {
		http.Error(w, "assinatura inválida", http.StatusUnauthorized)
		return
	}
	var in zapEntrada
	if json.Unmarshal(body, &in) != nil {
		w.WriteHeader(200)
		return
	}
	for _, e := range in.Entry {
		for _, ch := range e.Changes {
			for _, m := range ch.Value.Messages {
				if !cfg.ehDono(m.From) || s.zapJaVi(m.ID) {
					continue
				}
				texto, foto, nome := m.Text.Body, []byte(nil), ""
				if m.Type == "image" {
					texto = m.Image.Caption
					if b, err := s.zapBaixarMidia(r.Context(), cfg, m.Image.ID); err == nil {
						foto, nome = b, "whatsapp.jpg"
					}
				} else if m.Type != "text" {
					s.zapResponder(r.Context(), cfg, m.From, "Por enquanto eu entendo texto e foto. "+ajudaZap)
					continue
				}
				resp := s.zapMensagem(rq, rq.site, "whatsapp", texto, foto, nome)
				if err := s.zapResponder(r.Context(), cfg, m.From, resp); err != nil {
					s.logErr(rq, fmt.Errorf("whatsapp: resposta não enviada: %v", err))
				}
			}
		}
	}
	w.WriteHeader(200)
}

func assinaturaOK(body []byte, cab, segredo string) bool {
	if !strings.HasPrefix(cab, "sha256=") {
		return false
	}
	m := hmac.New(sha256.New, []byte(segredo))
	m.Write(body)
	return hmac.Equal([]byte(cab[7:]), []byte(hex.EncodeToString(m.Sum(nil))))
}

// zapJaVi evita repetir a mesma mensagem (a Meta reenvia quando demora).
func (s *Server) zapJaVi(id string) bool {
	if id == "" {
		return false
	}
	var vs []string
	_ = json.Unmarshal([]byte(s.Store.Meta(metaZapVistas)), &vs)
	for _, v := range vs {
		if v == id {
			return true
		}
	}
	vs = append(vs, id)
	if len(vs) > 200 {
		vs = vs[len(vs)-200:]
	}
	b, _ := json.Marshal(vs)
	_ = s.Store.SetMeta(metaZapVistas, string(b))
	return false
}

var clienteZap = &http.Client{Timeout: 30 * time.Second}

func (s *Server) zapBaixarMidia(ctx context.Context, cfg zapConfig, id string) ([]byte, error) {
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, graphURL+"/"+url.PathEscape(id), nil)
	req.Header.Set("Authorization", "Bearer "+cfg.Token)
	res, err := clienteZap.Do(req)
	if err != nil {
		return nil, err
	}
	var info struct{ URL string }
	err = json.NewDecoder(io.LimitReader(res.Body, 1<<16)).Decode(&info)
	res.Body.Close()
	if err != nil || info.URL == "" {
		return nil, fmt.Errorf("mídia sem endereço")
	}
	u, err := url.Parse(info.URL)
	if err != nil || u.Scheme != "https" {
		return nil, fmt.Errorf("endereço de mídia estranho")
	}
	req, _ = http.NewRequestWithContext(ctx, http.MethodGet, info.URL, nil)
	req.Header.Set("Authorization", "Bearer "+cfg.Token)
	res, err = clienteZap.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	if res.StatusCode != 200 {
		return nil, fmt.Errorf("mídia: %s", res.Status)
	}
	return io.ReadAll(io.LimitReader(res.Body, fotosMaxBytes))
}

func (s *Server) zapResponder(ctx context.Context, cfg zapConfig, para, texto string) error {
	if cfg.PhoneID == "" || cfg.Token == "" {
		return fmt.Errorf("WhatsApp não configurado")
	}
	if utf8.RuneCountInString(texto) > 4000 {
		texto = string([]rune(texto)[:4000])
	}
	b, _ := json.Marshal(map[string]any{"messaging_product": "whatsapp", "to": soDigitos(para), "type": "text", "text": map[string]any{"body": texto, "preview_url": true}})
	req, _ := http.NewRequestWithContext(ctx, http.MethodPost, graphURL+"/"+url.PathEscape(cfg.PhoneID)+"/messages", bytes.NewReader(b))
	req.Header.Set("Authorization", "Bearer "+cfg.Token)
	req.Header.Set("Content-Type", "application/json")
	res, err := clienteZap.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	if res.StatusCode/100 != 2 {
		msg, _ := io.ReadAll(io.LimitReader(res.Body, 400))
		return fmt.Errorf("%s: %s", res.Status, msg)
	}
	return nil
}

// ---------- painel: /_admin/whatsapp ----------

func (a *adminPage) whatsappEnviar() error {
	r := a.rq.r
	site := a.site()
	if site == nil {
		writeJSON(a.rq.w, map[string]string{"erro": "o site tem um erro; abra o painel"})
		return nil
	}
	if a.rq.site == nil {
		a.rq.site = site
	}
	texto := strings.TrimSpace(r.PostForm.Get("texto"))
	var foto []byte
	nome := ""
	if r.MultipartForm != nil {
		if fs := r.MultipartForm.File["foto"]; len(fs) > 0 {
			if f, err := fs[0].Open(); err == nil {
				foto, _ = io.ReadAll(io.LimitReader(f, fotosMaxBytes))
				nome = fs[0].Filename
				f.Close()
			}
		}
	}
	resp := a.s.zapMensagem(a.rq, site, "simulador", texto, foto, nome)
	writeJSON(a.rq.w, map[string]string{"resposta": resp})
	return nil
}

func (a *adminPage) whatsappPage(post bool) error {
	r := a.rq.r
	cfg := a.s.zapConfig()
	msg := ""
	if post && r.PostForm.Get("acao") == "config" {
		cfg.PhoneID = strings.TrimSpace(r.PostForm.Get("phone_id"))
		if t := strings.TrimSpace(r.PostForm.Get("token")); t != "" {
			cfg.Token = t
		}
		if t := strings.TrimSpace(r.PostForm.Get("app_secret")); t != "" {
			cfg.AppSecret = t
		}
		if cfg.Verify == "" {
			cfg.Verify = randHex(12)
		}
		cfg.Donos = nil
		for _, n := range strings.FieldsFunc(r.PostForm.Get("donos"), func(r rune) bool { return r == ',' || r == '\n' || r == ';' }) {
			if d := soDigitos(n); len(d) >= 10 {
				cfg.Donos = append(cfg.Donos, d)
			}
		}
		b, _ := json.Marshal(cfg)
		if err := a.s.Store.SetMeta(metaZapConfig, string(b)); err != nil {
			return err
		}
		msg = "Configuração salva."
	}
	if post && r.PostForm.Get("acao") == "limpar" {
		_ = a.s.Store.SetMeta(metaZapConversa, "")
	}
	if a.rq.site == nil {
		a.rq.site = a.site()
	}
	var b strings.Builder
	b.WriteString(cmsCSS + zapCSS)
	if msg != "" {
		b.WriteString(`<p class="ok">` + esc(msg) + `</p>`)
	}
	b.WriteString(`<p>Você cuida do site mandando mensagem: <b>chegou bolo de cenoura, R$ 12</b> com uma foto põe o produto no site; <b>acabou o bolo</b> tira; <b>amanhã fecho às 14h</b> vira um aviso no topo do site. Teste aqui antes de ligar o WhatsApp de verdade.</p>`)
	b.WriteString(`<div class="zap"><div class="zap-topo"><span class="zap-av">🌿</span><div><b>Seu site</b><small>simulador · responde como no WhatsApp</small></div></div><div class="zap-msgs" id="zap-msgs">`)
	for _, l := range a.s.zapConversa() {
		cls := "eu"
		if l.Quem == "site" {
			cls = "ele"
		}
		b.WriteString(`<div class="zap-b ` + cls + `">` + l.Foto + " " + zapFormatar(l.Texto) + `<small>` + esc(fmtWhen(l.Em)) + map[bool]string{true: " · WhatsApp", false: ""}[l.Via == "whatsapp"] + `</small></div>`)
	}
	b.WriteString(`</div><form class="zap-form" id="zap-form">` + a.tok() +
		`<label class="zap-clip" title="Mandar foto">📎<input type="file" name="foto" accept="image/*" hidden></label>` +
		`<input name="texto" placeholder="Ex.: chegou bolo de cenoura, R$ 12" autocomplete="off"><button aria-label="Enviar">➤</button></form>` +
		`<div class="zap-foto" id="zap-foto" hidden></div></div>`)
	b.WriteString(`<form method="post" action="` + a.base + `/whatsapp" style="margin:6px 0 18px">` + a.tok() + `<input type="hidden" name="acao" value="limpar"><button class="ghost">Limpar a conversa</button></form>`)

	webhook := a.s.siteURL(a.rq) + "/_nep/whatsapp"
	ligado := cfg.PhoneID != "" && cfg.Token != "" && cfg.AppSecret != "" && len(cfg.Donos) > 0
	estado := `<span class="tag off">não ligado</span>`
	if ligado {
		estado = `<span class="tag">ligado</span>`
	}
	b.WriteString(`<details class="card"` + map[bool]string{true: "", false: " open"}[ligado] + `><summary><b>WhatsApp de verdade</b> ` + estado + `</summary>`)
	b.WriteString(`<ol style="line-height:1.7"><li>Em <a href="https://developers.facebook.com/apps" target="_blank" rel="noopener">developers.facebook.com</a>, crie um app do tipo <b>Empresa</b> e adicione o produto <b>WhatsApp</b>.</li>` +
		`<li>Em WhatsApp › Configuração da API, copie o <b>ID do número de telefone</b> e gere um <b>token permanente</b> (usuário do sistema, permissão whatsapp_business_messaging).</li>` +
		`<li>Em Configurações do app › Básico, copie a <b>Chave secreta do app</b>.</li>` +
		`<li>Cole tudo abaixo e salve. Depois, em WhatsApp › Configuração › Webhook, use:<br>URL de retorno: <code>` + esc(webhook) + `</code><br>Token de verificação: <code>` + esc(map[bool]string{true: cfg.Verify, false: "(salve primeiro)"}[cfg.Verify != ""]) + `</code><br>e assine o campo <b>messages</b>.</li>` +
		`<li>Mande uma mensagem do seu celular para o número do WhatsApp do app.</li></ol>` +
		`<p class="muted">A Meta cobra por conversa iniciada pela empresa; respostas dentro de 24 h de uma mensagem sua não são cobradas. O endereço precisa ser https (no ar, não no computador).</p>`)
	tok, sec := "", ""
	if cfg.Token != "" {
		tok = "guardado (deixe em branco para manter)"
	}
	if cfg.AppSecret != "" {
		sec = "guardada (deixe em branco para manter)"
	}
	b.WriteString(`<form class="cms" method="post" action="` + a.base + `/whatsapp">` + a.tok() + `<input type="hidden" name="acao" value="config">` +
		`<label>ID do número de telefone <input name="phone_id" value="` + esc(cfg.PhoneID) + `" inputmode="numeric"></label>` +
		`<label>Token de acesso <input name="token" type="password" autocomplete="off" placeholder="` + esc(tok) + `"></label>` +
		`<label>Chave secreta do app <input name="app_secret" type="password" autocomplete="off" placeholder="` + esc(sec) + `"></label>` +
		`<label>Números do dono que podem mandar (com DDI e DDD, um por linha) <textarea name="donos" rows="2" placeholder="5571988881234">` + esc(strings.Join(cfg.Donos, "\n")) + `</textarea></label>` +
		`<p class="muted">Mensagens de outros números são ignoradas.</p><p><button>Salvar</button></p></form></details>`)
	b.WriteString(zapJS)
	return a.out("WhatsApp do site", b.String(), true)
}

// zapFormatar mostra *negrito* e quebras de linha como no WhatsApp.
func zapFormatar(t string) string {
	e := html.EscapeString(t)
	e = regexp.MustCompile(`\*([^*\n]+)\*`).ReplaceAllString(e, "<b>$1</b>")
	e = regexp.MustCompile(`(https?://[^\s<]+)`).ReplaceAllString(e, `<a href="$1" target="_blank">$1</a>`)
	return strings.ReplaceAll(e, "\n", "<br>")
}

const zapCSS = `<style>
.zap{max-width:560px;border-radius:14px;overflow:hidden;border:1px solid #d9dccf;background:#efeae2;box-shadow:0 6px 24px rgba(0,0,0,.06)}
.zap-topo{display:flex;gap:10px;align-items:center;background:#075e54;color:#fff;padding:10px 14px}
.zap-topo small{display:block;opacity:.8;font-size:12px}.zap-av{width:36px;height:36px;border-radius:50%;background:#fff;display:grid;place-items:center;font-size:20px}
.zap-msgs{height:420px;overflow:auto;padding:14px;display:flex;flex-direction:column;gap:6px}
.zap-b{max-width:82%;padding:7px 10px 4px;border-radius:9px;font-size:14.5px;line-height:1.4;box-shadow:0 1px .5px rgba(0,0,0,.13);word-wrap:break-word}
.zap-b small{display:block;text-align:right;font-size:11px;color:#667781;margin-top:2px}
.zap-b.eu{align-self:flex-end;background:#d9fdd3}.zap-b.ele{align-self:flex-start;background:#fff}
.zap-form{display:flex;gap:8px;align-items:center;padding:8px;background:#f0f2f5}
.zap-form input[name=texto]{flex:1;border:0;border-radius:20px;padding:10px 14px;font:inherit}
.zap-form button{border-radius:50%;width:42px;height:42px;padding:0;background:#00a884;border:0;color:#fff;font-size:18px}
.zap-clip{cursor:pointer;font-size:22px;padding:0 4px}
.zap-foto{padding:6px 10px;background:#f0f2f5;font-size:13px;color:#54656f}
.zap-digitando{font-style:italic;color:#667781}
</style>`

const zapJS = `<script>
(function(){
  var f=document.getElementById('zap-form'),m=document.getElementById('zap-msgs'),fi=f.querySelector('input[type=file]'),fp=document.getElementById('zap-foto');
  function fim(){m.scrollTop=m.scrollHeight}fim();
  function fmt(t){var d=document.createElement('div');d.textContent=t;var h=d.innerHTML;h=h.replace(/\*([^*\n]+)\*/g,'<b>$1</b>').replace(/(https?:\/\/[^\s<]+)/g,'<a href="$1" target="_blank">$1</a>');return h.replace(/\n/g,'<br>')}
  function bolha(c,h){var b=document.createElement('div');b.className='zap-b '+c;b.innerHTML=h+'<small>'+new Date().toLocaleTimeString('pt-BR',{hour:'2-digit',minute:'2-digit'})+'</small>';m.appendChild(b);fim();return b}
  fi.addEventListener('change',function(){fp.hidden=!fi.files.length;fp.textContent=fi.files.length?'📷 '+fi.files[0].name+' (escreva o nome e o preço na mensagem)':''});
  f.addEventListener('submit',function(e){e.preventDefault();var t=f.texto.value.trim();if(!t&&!fi.files.length)return;
    bolha('eu',(fi.files.length?'📷 ':'')+fmt(t));var fd=new FormData(f);f.texto.value='';
    var d=bolha('ele','<span class="zap-digitando">digitando…</span>');
    fetch(location.pathname.replace(/\/whatsapp$/,'/whatsapp/enviar'),{method:'POST',body:fd,credentials:'same-origin'}).then(function(r){return r.json()}).then(function(j){d.innerHTML=fmt(j.resposta||j.erro||'?')+'<small>agora</small>';fim()}).catch(function(){d.textContent='Não consegui enviar.'});
    fi.value='';fp.hidden=true});
})();
</script>`
