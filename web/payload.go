package web

// O Nepenthes com as ideias do Payload CMS (MIT), escritas de novo em Go e na
// linguagem Nepenthes:
//
//   - cada <conteudo> ganha uma API REST em /api/<colecao> (listar com
//     filtros, ordem e páginas; ler; criar; editar; apagar para a lixeira);
//   - chaves de API (Authorization: Bearer nep_...) feitas em /_admin/api;
//   - relações: <select name="autor" data-relacao="autores"> guarda o número
//     de um item de outro tipo, e a API/página trazem o item inteiro;
//   - ganchos: <conteudo antes="fn" depois="fn"> chamam funções do site;
//   - globais: <global nome="rodape"> é um formulário único do site inteiro,
//     lido com global("rodape") e em /api/globais/rodape.

import (
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"html"
	"math"
	"net/http"
	"net/url"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/Nanofylium/probable-plant/lang"
)

// ---------- chaves de API ----------

const metaChaves = "api:chaves"

type chaveAPI struct {
	ID      string `json:"id"`
	Nome    string `json:"nome"`
	Hash    string `json:"hash"`
	Prefixo string `json:"prefixo"`
	Escrita bool   `json:"escrita"`
	Em      string `json:"em"`
	Por     string `json:"por,omitempty"`
	Uso     string `json:"uso,omitempty"`
}

func hashChave(k string) string {
	h := sha256.Sum256([]byte("nep-api:" + k))
	return hex.EncodeToString(h[:])
}

func (s *Server) chavesAPI() []chaveAPI {
	var ks []chaveAPI
	_ = json.Unmarshal([]byte(s.Store.Meta(metaChaves)), &ks)
	return ks
}

func (s *Server) salvarChaves(ks []chaveAPI) error {
	b, _ := json.Marshal(ks)
	return s.Store.SetMeta(metaChaves, string(b))
}

// acessoAPI diz o que o pedido pode fazer: ler rascunhos e arquivos privados
// (chave ou sessão do painel) e escrever (só chave com escrita).
func (s *Server) acessoAPI(rq *request) (ler, escrever bool, chave string) {
	a := strings.TrimSpace(rq.r.Header.Get("Authorization"))
	k := ""
	for _, p := range []string{"Bearer ", "bearer ", "users API-Key ", "API-Key "} {
		if strings.HasPrefix(a, p) {
			k = strings.TrimSpace(a[len(p):])
		}
	}
	if k != "" {
		h := hashChave(k)
		ks := s.chavesAPI()
		for i, c := range ks {
			if subtle.ConstantTimeCompare([]byte(c.Hash), []byte(h)) == 1 {
				if hoje := time.Now().Format("2006-01-02"); c.Uso != hoje { // anota o uso uma vez por dia
					ks[i].Uso = hoje
					_ = s.salvarChaves(ks)
				}
				return true, c.Escrita, c.Nome
			}
		}
		return false, false, "?"
	}
	return rq.sess != nil && rq.sess.isAdmin(), false, ""
}

// ---------- API REST ----------

func apiErros(msgs ...string) lang.Value {
	t := lang.NewTable()
	l := lang.NewList()
	for _, m := range msgs {
		e := lang.NewTable()
		e.SetStr("message", m)
		l.Items = append(l.Items, e)
	}
	t.SetStr("errors", l)
	if len(msgs) > 0 {
		t.SetStr("erro", msgs[0])
	}
	return t
}

// restAPI atende /api, /api/<colecao>[/<id|slug>] e /api/globais/<nome>.
// Devolve false quando o endereço não é dele (aí segue para as páginas).
func (s *Server) restAPI(rq *request) (bool, error) {
	if rq.path != "/api" && !strings.HasPrefix(rq.path, "/api/") {
		return false, nil
	}
	site := rq.site
	partes := strings.Split(strings.Trim(strings.TrimPrefix(rq.path, "/api"), "/"), "/")
	metodo := rq.r.Method
	if metodo == http.MethodHead {
		metodo = http.MethodGet
	}
	ler, escrever, chave := s.acessoAPI(rq)
	if chave == "?" {
		rq.w.Header().Set("Cache-Control", "no-store")
		return true, s.apiJSON(rq, 401, apiErros("chave de API inválida"))
	}
	switch {
	case partes[0] == "":
		if metodo != http.MethodGet {
			return false, nil
		}
		rq.w.Header().Set("Cache-Control", "no-store")
		return true, s.apiJSON(rq, 200, s.apiIndice(site))
	case partes[0] == "globais" || partes[0] == "globals":
		if len(partes) != 2 || site.Global(partes[1]) == nil || metodo != http.MethodGet {
			return false, nil
		}
		rq.w.Header().Set("Cache-Control", "no-store")
		return true, s.apiJSON(rq, 200, s.globalValor(site.Global(partes[1]), ler))
	}
	c := site.Content(partes[0])
	if c == nil || c.SemAPI || len(partes) > 2 {
		return false, nil
	}
	rq.w.Header().Set("Cache-Control", "no-store")
	depth := 1
	if d, err := strconv.Atoi(rq.r.URL.Query().Get("depth")); err == nil {
		depth = max(0, min(d, 3))
	}
	if len(partes) == 1 {
		switch metodo {
		case http.MethodGet:
			return true, s.apiJSON(rq, 200, s.apiLista(rq, c, ler, depth))
		case http.MethodPost:
			if !escrever {
				return true, s.apiJSON(rq, 401, apiErros("para criar, envie Authorization: Bearer <chave com escrita> (faça a chave no painel, em API)"))
			}
			return true, s.apiSalvar(rq, c, nil, chave, depth)
		}
		rq.w.Header().Set("Allow", "GET, POST")
		return true, s.apiJSON(rq, 405, apiErros("use GET ou POST"))
	}
	it := s.findItem(c.Name, partes[1])
	if it == nil || (!ler && !published(it)) {
		return true, s.apiJSON(rq, 404, apiErros("não encontrado"))
	}
	switch metodo {
	case http.MethodGet:
		return true, s.apiJSON(rq, 200, s.itemSaida(site, c, it, depth, ler))
	case http.MethodPatch, http.MethodPut:
		if !escrever {
			return true, s.apiJSON(rq, 401, apiErros("para editar, envie Authorization: Bearer <chave com escrita>"))
		}
		return true, s.apiSalvar(rq, c, it, chave, depth)
	case http.MethodDelete:
		if !escrever {
			return true, s.apiJSON(rq, 401, apiErros("para apagar, envie Authorization: Bearer <chave com escrita>"))
		}
		id, _ := toInt(getV(it, "id"))
		if c.Antes != "" {
			if _, err := s.gancho(rq, site, c.Antes, it, infoGancho("apagar", id, it)); err != nil {
				if msg, ok := recusado(err); ok {
					return true, s.apiJSON(rq, 400, apiErros(msg))
				}
				return true, s.apiErr(rq, err)
			}
		}
		if err := s.trash(c.Name, id, "api:"+chave); err != nil {
			return true, err
		}
		s.depois(rq, site, c, "apagar", id, it)
		t := lang.NewTable()
		t.SetStr("doc", s.itemSaida(site, c, it, 0, true))
		t.SetStr("message", "Foi para a lixeira (dá para restaurar no painel por 30 dias).")
		return true, s.apiJSON(rq, 200, t)
	}
	rq.w.Header().Set("Allow", "GET, PATCH, DELETE")
	return true, s.apiJSON(rq, 405, apiErros("use GET, PATCH ou DELETE"))
}

func getV(t *lang.Table, k string) lang.Value {
	v, _ := t.Get(k)
	return v
}

// apiIndice descreve os tipos e globais (útil para IAs e outros programas).
func (s *Server) apiIndice(site *Site) lang.Value {
	out := lang.NewTable()
	cols := lang.NewList()
	for _, c := range site.Contents {
		if c.SemAPI {
			continue
		}
		t := lang.NewTable()
		t.SetStr("nome", c.Name)
		t.SetStr("titulo", c.Title)
		t.SetStr("url", s.url("/api/"+c.Name))
		t.SetStr("campos", camposJSON(c.Fields))
		t.SetStr("total", int64(s.Store.Count(c.Name)))
		cols.Items = append(cols.Items, t)
	}
	out.SetStr("colecoes", cols)
	gs := lang.NewList()
	for _, g := range site.Globals {
		t := lang.NewTable()
		t.SetStr("nome", g.Name)
		t.SetStr("titulo", g.Title)
		t.SetStr("url", s.url("/api/globais/"+g.Name))
		t.SetStr("campos", camposJSON(g.Fields))
		gs.Items = append(gs.Items, t)
	}
	out.SetStr("globais", gs)
	return out
}

func camposJSON(fs []*Field) *lang.List {
	l := lang.NewList()
	for _, f := range fs {
		t := lang.NewTable()
		t.SetStr("nome", f.Name)
		t.SetStr("rotulo", f.Label)
		tp := f.Type
		if f.Rel != "" {
			tp = "relacao"
			t.SetStr("relacao", f.Rel)
		}
		t.SetStr("tipo", tp)
		t.SetStr("obrigatorio", f.Required)
		if f.Multi {
			t.SetStr("varios", true)
		}
		if f.Options != nil {
			ol := lang.NewList()
			for _, o := range f.Options {
				if o != "" {
					ol.Items = append(ol.Items, o)
				}
			}
			t.SetStr("opcoes", ol)
		}
		l.Items = append(l.Items, t)
	}
	return l
}

// itemSaida prepara um item para fora: tira arquivos privados de quem não
// tem acesso e troca o número das relações pelo item (depth níveis).
func (s *Server) itemSaida(site *Site, c *Content, t *lang.Table, depth int, ler bool) *lang.Table {
	out := lang.NewTable()
	for i := 0; i < t.Len(); i++ {
		k, v := t.Entry(i)
		_ = out.Set(k, v)
	}
	for _, f := range c.Fields {
		v, ok := out.Get(f.Name)
		if !ok || v == nil {
			continue
		}
		if f.Type == "file" && !f.Public && !ler {
			out.Delete(f.Name)
			continue
		}
		if f.Rel == "" || depth <= 0 {
			continue
		}
		rc := site.Content(f.Rel)
		if rc == nil {
			continue
		}
		pega := func(v lang.Value) lang.Value {
			id, ok := toInt(v)
			if !ok {
				return v
			}
			r := s.Store.Find(rc.Name, id)
			if r == nil {
				return nil
			}
			rt := r.ToValue()
			if !ler && !published(rt) {
				return id
			}
			return s.itemSaida(site, rc, rt, depth-1, ler)
		}
		if l, isL := v.(*lang.List); isL {
			nl := lang.NewList()
			for _, x := range l.Items {
				if y := pega(x); y != nil {
					nl.Items = append(nl.Items, y)
				}
			}
			out.SetStr(f.Name, nl)
		} else {
			out.SetStr(f.Name, pega(v))
		}
	}
	return out
}

var whereRe = regexp.MustCompile(`^where\[([^\]]+)\](?:\[([^\]]+)\])?$`)

type filtroAPI struct{ campo, op, valor string }

func numeroDe(v lang.Value) (float64, bool) {
	switch x := v.(type) {
	case int64:
		return float64(x), true
	case float64:
		return x, true
	case string:
		f, err := strconv.ParseFloat(strings.Replace(strings.TrimSpace(x), ",", ".", 1), 64)
		return f, err == nil && !math.IsNaN(f)
	}
	return 0, false
}

// compararValores ordena números como números e o resto como texto; vazio fica por último.
func compararValores(a, b lang.Value) int {
	va, vb := a == nil || lang.Str(a) == "", b == nil || lang.Str(b) == ""
	switch {
	case va && vb:
		return 0
	case va:
		return 1
	case vb:
		return -1
	}
	if x, ok := numeroDe(a); ok {
		if y, ok := numeroDe(b); ok {
			switch {
			case x < y:
				return -1
			case x > y:
				return 1
			}
			return 0
		}
	}
	return strings.Compare(strings.ToLower(lang.Str(a)), strings.ToLower(lang.Str(b)))
}

func (f filtroAPI) passa(t *lang.Table) bool {
	v, _ := t.Get(f.campo)
	if l, ok := v.(*lang.List); ok { // lista (vários): basta um bater
		for _, x := range l.Items {
			tt := lang.NewTable()
			tt.SetStr(f.campo, x)
			if f.passa(tt) {
				return f.op != "diferente"
			}
		}
		return f.op == "diferente"
	}
	if rt, ok := v.(*lang.Table); ok { // arquivo: compara o nome
		v = getV(rt, "nome")
	}
	sv := ""
	if v != nil {
		sv = lang.Str(v)
	}
	if b, ok := v.(bool); ok {
		sv = strconv.FormatBool(b)
	}
	cmp := func() (int, bool) {
		a, ok1 := numeroDe(v)
		b, ok2 := numeroDe(f.valor)
		if ok1 && ok2 {
			switch {
			case a < b:
				return -1, true
			case a > b:
				return 1, true
			}
			return 0, true
		}
		if v == nil || sv == "" {
			return 0, false
		}
		return strings.Compare(sv, f.valor), true
	}
	switch f.op {
	case "igual":
		if a, ok := numeroDe(v); ok {
			if b, ok := numeroDe(f.valor); ok {
				return a == b
			}
		}
		return strings.EqualFold(sv, f.valor)
	case "diferente":
		return !strings.EqualFold(sv, f.valor)
	case "contem":
		return strings.Contains(strings.ToLower(sv), strings.ToLower(f.valor))
	case "em":
		for _, x := range strings.Split(f.valor, ",") {
			if strings.EqualFold(sv, strings.TrimSpace(x)) {
				return true
			}
		}
		return false
	case "existe":
		tem := v != nil && sv != ""
		return tem == (f.valor != "false" && f.valor != "nao" && f.valor != "0")
	}
	n, ok := cmp()
	if !ok {
		return false
	}
	switch f.op {
	case "maior":
		return n > 0
	case "menor":
		return n < 0
	case "maior_igual":
		return n >= 0
	case "menor_igual":
		return n <= 0
	}
	return false
}

var opsAPI = map[string]string{"": "igual", "equals": "igual", "igual": "igual", "not_equals": "diferente", "diferente": "diferente",
	"contains": "contem", "like": "contem", "contem": "contem", "contém": "contem", "in": "em", "em": "em", "exists": "existe", "existe": "existe",
	"greater_than": "maior", "maior": "maior", "less_than": "menor", "menor": "menor",
	"greater_than_equal": "maior_igual", "maior_igual": "maior_igual", "less_than_equal": "menor_igual", "menor_igual": "menor_igual"}

func camposDe(c *Content) map[string]bool {
	ok := map[string]bool{"id": true, "slug": true, "publicado": true, "created_at": true, "updated_at": true, schedField: true}
	for _, f := range c.Fields {
		ok[f.Name] = true
	}
	return ok
}

// apiLista: ?limit=10&page=1&sort=-preco&where[bairro]=Centro&where[preco][menor]=500000
func (s *Server) apiLista(rq *request, c *Content, ler bool, depth int) lang.Value {
	q := rq.r.URL.Query()
	validos := camposDe(c)
	var filtros []filtroAPI
	var erros []string
	for k, vs := range q {
		m := whereRe.FindStringSubmatch(k)
		if m == nil {
			continue
		}
		op, ok := opsAPI[strings.ToLower(m[2])]
		if !ok {
			erros = append(erros, "filtro desconhecido: "+m[2]+" (use igual, diferente, contem, em, existe, maior, menor, maior_igual, menor_igual)")
			continue
		}
		if !validos[m[1]] {
			erros = append(erros, "campo desconhecido: "+m[1])
			continue
		}
		for _, v := range vs {
			filtros = append(filtros, filtroAPI{m[1], op, v})
		}
	}
	if len(erros) > 0 {
		return apiErros(erros...)
	}
	var docs []*lang.Table
	for _, r := range s.Store.All(c.Name, 0) {
		t := r.ToValue()
		if !ler && !published(t) {
			continue
		}
		ok := true
		for _, f := range filtros {
			if !f.passa(t) {
				ok = false
				break
			}
		}
		if ok {
			docs = append(docs, t)
		}
	}
	if so := strings.TrimSpace(q.Get("sort")); so != "" {
		desc := strings.HasPrefix(so, "-")
		campo := strings.TrimPrefix(so, "-")
		if validos[campo] {
			sort.SliceStable(docs, func(i, j int) bool {
				n := compararValores(getV(docs[i], campo), getV(docs[j], campo))
				if desc {
					return n > 0
				}
				return n < 0
			})
		}
	}
	limit := 10
	if n, err := strconv.Atoi(q.Get("limit")); err == nil && n > 0 {
		limit = min(n, 100)
	}
	page := 1
	if n, err := strconv.Atoi(q.Get("page")); err == nil && n > 0 {
		page = n
	}
	total := len(docs)
	pages := (total + limit - 1) / limit
	ini := min((page-1)*limit, total)
	fim := min(ini+limit, total)
	l := lang.NewList()
	for _, d := range docs[ini:fim] {
		l.Items = append(l.Items, s.itemSaida(rq.site, c, d, depth, ler))
	}
	out := lang.NewTable()
	out.SetStr("docs", l)
	out.SetStr("totalDocs", int64(total))
	out.SetStr("limit", int64(limit))
	out.SetStr("totalPages", int64(pages))
	out.SetStr("page", int64(page))
	out.SetStr("pagingCounter", int64(ini+1))
	out.SetStr("hasPrevPage", page > 1)
	out.SetStr("hasNextPage", page < pages)
	if page > 1 {
		out.SetStr("prevPage", int64(page-1))
	} else {
		out.SetStr("prevPage", nil)
	}
	if page < pages {
		out.SetStr("nextPage", int64(page+1))
	} else {
		out.SetStr("nextPage", nil)
	}
	return out
}

// jsonParaForm transforma o JSON enviado nos valores de formulário que
// validate() já sabe conferir (assim a API segue as mesmas regras do HTML).
func jsonParaForm(fields []*Field, obj *lang.Table, base url.Values) (url.Values, []string) {
	in := url.Values{}
	for k, v := range base {
		in[k] = v
	}
	por := map[string]*Field{}
	for _, f := range fields {
		por[f.Name] = f
	}
	var erros []string
	for i := 0; i < obj.Len(); i++ {
		kv, v := obj.Entry(i)
		k := lang.Str(kv)
		switch k {
		case "id", "created_at", "updated_at", "slug", "publicado", schedField, "createdAt", "updatedAt", "_status":
			continue
		}
		f := por[k]
		if f == nil {
			erros = append(erros, "campo desconhecido: "+k)
			continue
		}
		if f.Type == "file" {
			erros = append(erros, "o arquivo de \""+f.Label+"\" se envia pelo painel (a API guarda textos, números e relações)")
			continue
		}
		str := func(x lang.Value) string {
			switch y := x.(type) {
			case nil:
				return ""
			case bool:
				if y {
					return "on"
				}
				return ""
			case float64:
				return fmtNum(y)
			case *lang.Table: // relação populada: {id: 3, ...}
				return lang.Str(getV(y, "id"))
			}
			return lang.Str(x)
		}
		if f.Multi {
			delete(in, k+"[]")
			if l, ok := v.(*lang.List); ok {
				for _, x := range l.Items {
					in.Add(k+"[]", str(x))
				}
			} else if v != nil {
				in.Add(k+"[]", str(v))
			}
			continue
		}
		if sv := str(v); sv != "" {
			in.Set(k, sv)
		} else {
			delete(in, k)
		}
	}
	return in, erros
}

// valoresForm devolve o item salvo como valores de formulário (para o PATCH
// mudar só o que veio).
func valoresForm(c *Content, t *lang.Table) url.Values {
	in := url.Values{}
	for k, vs := range oldValues(c, t) {
		name := k
		for _, f := range c.Fields {
			if f.Name == k && f.Multi {
				name = k + "[]"
			}
		}
		for _, v := range vs {
			if v == checkedMark {
				v = "on"
			}
			in.Add(name, v)
		}
	}
	return in
}

// conferirRelacoes guarda o número do item relacionado e confere se ele existe.
func (s *Server) conferirRelacoes(site *Site, fields []*Field, data *lang.Table) []string {
	var erros []string
	for _, f := range fields {
		if f.Rel == "" {
			continue
		}
		if site.Content(f.Rel) == nil {
			erros = append(erros, fmt.Sprintf("\"%s\" aponta para o tipo %q, que não existe", f.Label, f.Rel))
			continue
		}
		um := func(v lang.Value) (lang.Value, bool) {
			if v == nil || lang.Str(v) == "" {
				return nil, true
			}
			id, ok := toInt(v)
			if !ok {
				if it := s.findItem(f.Rel, v); it != nil { // aceita o endereço (slug)
					id, ok = toInt(getV(it, "id"))
				}
			}
			if !ok || s.Store.Find(f.Rel, id) == nil {
				return nil, false
			}
			return id, true
		}
		v, _ := data.Get(f.Name)
		if l, ok := v.(*lang.List); ok {
			nl := lang.NewList()
			for _, x := range l.Items {
				y, ok := um(x)
				if !ok {
					erros = append(erros, fmt.Sprintf("Item inexistente em \"%s\".", f.Label))
					break
				}
				if y != nil {
					nl.Items = append(nl.Items, y)
				}
			}
			data.SetStr(f.Name, nl)
			continue
		}
		y, ok := um(v)
		if !ok {
			erros = append(erros, fmt.Sprintf("Item inexistente em \"%s\".", f.Label))
			continue
		}
		data.SetStr(f.Name, y)
	}
	return erros
}

// apiSalvar cria (cur == nil) ou edita um item a partir do JSON do pedido.
func (s *Server) apiSalvar(rq *request, c *Content, cur *lang.Table, chave string, depth int) error {
	site := rq.site
	if !strings.HasPrefix(rq.r.Header.Get("Content-Type"), "application/json") {
		return s.apiJSON(rq, 415, apiErros("envie Content-Type: application/json"))
	}
	body, err := readLimited(rq.r, 1<<20)
	if err != nil {
		return s.apiJSON(rq, 413, apiErros("o JSON passou de 1 MB"))
	}
	v, err := lang.FromJSON(body)
	obj, ok := v.(*lang.Table)
	if err != nil || !ok {
		return s.apiJSON(rq, 400, apiErros("o corpo precisa ser um objeto JSON"))
	}
	var base url.Values
	var id int64
	fields := c.Fields
	if cur != nil {
		base = valoresForm(c, cur)
		id, _ = toInt(getV(cur, "id"))
		fields = make([]*Field, len(c.Fields))
		for i, f := range c.Fields { // arquivo obrigatório já guardado continua valendo
			cp := *f
			if f.Type == "file" && getV(cur, f.Name) != nil {
				cp.Required = false
			}
			fields[i] = &cp
		}
	}
	in, erros := jsonParaForm(fields, obj, base)
	if cur == nil {
		for _, f := range fields {
			if f.Type == "file" && f.Required {
				erros = append(erros, "\""+f.Label+"\" é um arquivo obrigatório: crie este item pelo painel")
			}
		}
	}
	data, verrs := validate(fields, in)
	erros = append(erros, verrs...)
	for _, f := range fields {
		if f.Type == "file" {
			data.Delete(f.Name) // fica o arquivo que já estava
		}
	}
	if len(erros) == 0 {
		erros = s.conferirRelacoes(site, fields, data)
	}
	if len(erros) > 0 {
		return s.apiJSON(rq, 400, apiErros(erros...))
	}
	op := "criar"
	if cur != nil {
		op = "editar"
	}
	if c.Antes != "" {
		nd, err := s.gancho(rq, site, c.Antes, data, infoGancho(op, id, cur))
		if err != nil {
			if msg, ok := recusado(err); ok {
				return s.apiJSON(rq, 400, apiErros(msg))
			}
			return s.apiErr(rq, err)
		}
		data = nd
	}
	// endereço, publicação e agenda
	slugPedido, _ := obj.Get("slug")
	want := slugify(lang.Str(orNil(slugPedido)))
	if want == "" && cur != nil {
		want = lang.Str(orNil(getV(cur, "slug")))
	}
	if want == "" {
		want = slugify(itemTitle(c, data))
	}
	data.SetStr("slug", s.uniqueSlug(c.Name, want, id))
	if p, ok := obj.Get("publicado"); ok {
		data.SetStr("publicado", p != false && p != nil)
	} else if st, ok := obj.Get("_status"); ok { // como no Payload: "draft" ou "published"
		data.SetStr("publicado", lang.Str(st) != "draft")
	} else if cur == nil {
		data.SetStr("publicado", true)
	}
	if w, ok := obj.Get(schedField); ok {
		if tm, ok := parseSched(lang.Str(orNil(w))); ok {
			data.SetStr(schedField, tm.Format("2006-01-02T15:04"))
		} else {
			data.SetStr(schedField, nil)
		}
	}
	status := 200
	if cur != nil {
		if rec := s.Store.Find(c.Name, id); rec != nil {
			s.saveVersion(c.Name, rec, "api:"+chave)
		}
		if _, err := s.Store.Update(c.Name, id, data); err != nil {
			return err
		}
	} else {
		nid, err := s.Store.Insert(c.Name, data)
		if err != nil {
			return err
		}
		id, status = nid, 201
	}
	rec := s.Store.Find(c.Name, id)
	if rec == nil {
		return s.apiJSON(rq, 500, apiErros("o item sumiu depois de salvo"))
	}
	salvo := rec.ToValue()
	s.depois(rq, site, c, op, id, salvo)
	out := lang.NewTable()
	out.SetStr("doc", s.itemSaida(site, c, salvo, depth, true))
	if op == "criar" {
		out.SetStr("message", "Criado.")
	} else {
		out.SetStr("message", "Salvo.")
	}
	return s.apiJSON(rq, status, out)
}

func orNil(v lang.Value) lang.Value {
	if v == nil {
		return ""
	}
	return v
}

func readLimited(r *http.Request, n int64) (string, error) {
	var b strings.Builder
	buf := make([]byte, 32<<10)
	var total int64
	for {
		k, err := r.Body.Read(buf)
		total += int64(k)
		if total > n {
			return "", errors.New("grande demais")
		}
		b.Write(buf[:k])
		if err != nil {
			break
		}
	}
	return b.String(), nil
}

// ---------- ganchos antes/depois ----------

type recusa struct{ msg string }

func (r *recusa) Error() string { return r.msg }

func recusado(err error) (string, bool) {
	var r *recusa
	if errors.As(err, &r) {
		return r.msg, true
	}
	return "", false
}

func infoGancho(op string, id int64, anterior *lang.Table) *lang.Table {
	t := lang.NewTable()
	t.SetStr("operacao", op)
	if id > 0 {
		t.SetStr("id", id)
	} else {
		t.SetStr("id", nil)
	}
	if anterior != nil {
		t.SetStr("anterior", anterior)
	} else {
		t.SetStr("anterior", nil)
	}
	return t
}

// gancho chama fn(dados, info) do código do site. Se a função devolver uma
// tabela, ela vira os dados salvos; recusar("motivo") cancela.
func (s *Server) gancho(rq *request, site *Site, fn string, data, info *lang.Table) (*lang.Table, error) {
	sub := &request{s: s, site: site, w: rq.w, r: rq.r, path: rq.path, sess: rq.sess, https: rq.https}
	if err := s.setup(sub, -1, nil); err != nil {
		return nil, err
	}
	pre, _ := site.Sources(-1)
	if _, err := sub.vm.RunSource(pre, site.File, sub.mod()); err != nil {
		return nil, err
	}
	f, ok := sub.mod().Globals[fn]
	if !ok {
		return nil, fmt.Errorf("o gancho chama a função %q, que não existe no código do site", fn)
	}
	cp := lang.NewTable()
	for i := 0; i < data.Len(); i++ {
		k, v := data.Entry(i)
		_ = cp.Set(k, v)
	}
	res, err := sub.vm.Call(f, []lang.Value{cp, info})
	if err != nil {
		if h, isH := haltOf(err); isH && h.kind == "reject" {
			return nil, &recusa{h.msg}
		}
		return nil, err
	}
	if t, ok := res.(*lang.Table); ok {
		return t, nil
	}
	return cp, nil // a função pode mudar a tabela recebida
}

// depois roda o gancho depois="fn" (o erro só vai para o registro).
func (s *Server) depois(rq *request, site *Site, c *Content, op string, id int64, data *lang.Table) {
	if c.Depois == "" {
		return
	}
	if _, err := s.gancho(rq, site, c.Depois, data, infoGancho(op, id, nil)); err != nil {
		if _, isR := recusado(err); !isR {
			s.logErr(rq, fmt.Errorf("gancho depois=%q: %w", c.Depois, err))
		}
	}
}

// ---------- relações no formulário do painel ----------

// comOpcoesRelacao põe nos <select data-relacao> um <option> por item do
// outro tipo (o título aparece, o número é guardado).
func (s *Server) comOpcoesRelacao(site *Site, h string) string {
	return selRe.ReplaceAllStringFunc(h, func(tag string) string {
		m := selRe.FindStringSubmatch(tag)
		at := parseAttrs(m[2])
		rel := first(at, "data-relacao", "data-relation")
		rc := site.Content(rel)
		if rc == nil {
			return tag
		}
		var b strings.Builder
		if !strings.Contains(strings.ToLower(m[3]), "<option") {
			b.WriteString(`<option value="">— escolha —</option>`)
		}
		for _, r := range s.Store.All(rc.Name, 500) {
			t := r.ToValue()
			tit := itemTitle(rc, t)
			if tit == "" {
				tit = "#" + strconv.FormatInt(r.ID, 10)
			}
			if !published(t) {
				tit += " (rascunho)"
			}
			b.WriteString(`<option value="` + strconv.FormatInt(r.ID, 10) + `">` + html.EscapeString(tit) + `</option>`)
		}
		return m[1] + m[3] + b.String() + m[4]
	})
}

// ---------- globais ----------

func metaGlobal(nome string) string { return "global:" + nome }

// globalValor lê o global salvo; campo sem valor vem vazio.
func (s *Server) globalValor(g *Global, ler bool) *lang.Table {
	out := lang.NewTable()
	var salvo *lang.Table
	if v, err := lang.FromJSON(s.Store.Meta(metaGlobal(g.Name))); err == nil {
		salvo, _ = v.(*lang.Table)
	}
	for _, f := range g.Fields {
		var v lang.Value = ""
		if f.Type == "checkbox" {
			v = false
		}
		if f.Multi {
			v = lang.NewList()
		}
		if f.Type == "file" {
			v = nil
		}
		if salvo != nil {
			if x, ok := salvo.Get(f.Name); ok {
				v = x
			}
		}
		if f.Type == "file" && !f.Public && !ler {
			continue
		}
		out.SetStr(f.Name, v)
	}
	if salvo != nil {
		if v, ok := salvo.Get("updated_at"); ok {
			out.SetStr("updated_at", v)
		}
	}
	return out
}

// globalPage é /_admin/global?g=rodape.
func (a *adminPage) globalPage(post bool) error {
	site := a.site()
	var g *Global
	if site != nil {
		g = site.Global(a.rq.r.URL.Query().Get("g"))
	}
	if g == nil {
		a.s.simple(a.rq, 404, "Global não encontrado", "")
		return nil
	}
	r := a.rq.r
	cur := a.s.globalValor(g, true)
	fake := &Content{Name: g.Name, Fields: g.Fields}
	old := oldValues(fake, cur)
	var errs []string
	if post {
		fields := make([]*Field, len(g.Fields))
		for i, f := range g.Fields {
			cp := *f
			if f.Type == "file" && getV(cur, f.Name) != nil {
				cp.Required = false
			}
			fields[i] = &cp
		}
		data, verrs := validate(fields, r.PostForm)
		files, ferrs := checkFiles(fields, r)
		errs = append(verrs, ferrs...)
		if len(errs) == 0 {
			errs = a.s.conferirRelacoes(site, fields, data)
		}
		for _, f := range g.Fields {
			old[f.Name] = r.PostForm[f.Name+map[bool]string{true: "[]", false: ""}[f.Multi]]
			if f.Type == "checkbox" && r.PostForm.Get(f.Name) != "" {
				old[f.Name] = []string{checkedMark}
			}
		}
		if len(errs) == 0 {
			var gone []string
			for _, f := range g.Fields {
				if f.Type != "file" {
					continue
				}
				prev := getV(cur, f.Name)
				novo := false
				for _, p := range files {
					if p.field.Name == f.Name {
						novo = true
					}
				}
				if novo || r.PostForm.Get("_remover_"+f.Name) != "" {
					if pt, ok := prev.(*lang.Table); ok {
						gone = append(gone, filesIn(mustJSON(pt))...)
					}
					data.SetStr(f.Name, nil)
				} else {
					data.SetStr(f.Name, prev)
				}
			}
			saved, err := a.s.saveFiles(files, data)
			if err != nil {
				return err
			}
			data.SetStr("updated_at", stamp())
			data.SetStr("por", a.who())
			j, _ := lang.ToJSON(data, "")
			if err := a.s.Store.SetMeta(metaGlobal(g.Name), j); err != nil {
				a.s.Store.removeFiles(saved)
				return err
			}
			a.s.Store.removeFiles(gone)
			a.s.redirect(a.rq, a.base+"/global?g="+url.QueryEscape(g.Name)+"&ok=1", http.StatusSeeOther)
			return nil
		}
	}
	var b strings.Builder
	b.WriteString(cmsCSS)
	if r.URL.Query().Get("ok") != "" {
		b.WriteString(`<p class="ok">Salvo. O site inteiro já usa os novos dados.</p>`)
	}
	for _, e := range errs {
		b.WriteString(errP(e))
	}
	b.WriteString(`<p class="muted">Isto vale para o site inteiro. No código: <code>{{ global("` + esc(g.Name) + `").campo }}</code>; para outros programas: <code>` + esc(a.s.url("/api/globais/"+g.Name)) + `</code>.</p>`)
	b.WriteString(`<form class="cms" method="post" enctype="multipart/form-data" action="` + a.base + `/global?g=` + esc(url.QueryEscape(g.Name)) + `">` + a.tok())
	b.WriteString(refill(a.s.comOpcoesRelacao(site, g.HTML), old))
	for _, f := range g.Fields {
		if f.Type != "file" {
			continue
		}
		if ft, ok := getV(cur, f.Name).(*lang.Table); ok {
			u := lang.Str(getV(ft, "url"))
			b.WriteString(`<div class="card"><div class="muted">Arquivo atual em “` + esc(f.Label) + `” (envie outro para trocar)</div>`)
			if strings.HasPrefix(lang.Str(getV(ft, "tipo")), "image/") {
				b.WriteString(`<img src="` + esc(u) + `" alt="">`)
			}
			b.WriteString(`<a href="` + esc(u) + `" target="_blank">` + esc(lang.Str(getV(ft, "nome"))) + `</a>`)
			if !f.Required {
				b.WriteString(`<label class="check"><input type="checkbox" name="_remover_` + esc(f.Name) + `"> Remover este arquivo</label>`)
			}
			b.WriteString(`</div>`)
		}
	}
	b.WriteString(`<p class="row"><button>Salvar</button></p></form>`)
	return a.out(g.Title, b.String(), true)
}

func mustJSON(t *lang.Table) string {
	j, _ := lang.ToJSON(t, "")
	return j
}

// ---------- tela de chaves de API ----------

// apiPage é /_admin/api (só o dono): cria e revoga chaves.
func (a *adminPage) apiPage(post bool) error {
	r := a.rq.r
	nova, msg := "", ""
	ks := a.s.chavesAPI()
	if post {
		switch r.PostForm.Get("acao") {
		case "criar":
			nome := strings.TrimSpace(r.PostForm.Get("nome"))
			if nome == "" {
				nome = "Chave " + strconv.Itoa(len(ks)+1)
			}
			if len([]rune(nome)) > 60 {
				nome = string([]rune(nome)[:60])
			}
			nova = "nep_" + randHex(24)
			ks = append(ks, chaveAPI{ID: randHex(6), Nome: nome, Hash: hashChave(nova), Prefixo: nova[:10], Escrita: r.PostForm.Get("escrita") != "", Em: stamp(), Por: a.who()})
			if err := a.s.salvarChaves(ks); err != nil {
				return err
			}
		case "revogar":
			var ficam []chaveAPI
			for _, k := range ks {
				if k.ID != r.PostForm.Get("id") {
					ficam = append(ficam, k)
				}
			}
			ks = ficam
			if err := a.s.salvarChaves(ks); err != nil {
				return err
			}
			msg = "Chave revogada: quem usava não entra mais."
		}
	}
	var b strings.Builder
	b.WriteString(cmsCSS)
	if msg != "" {
		b.WriteString(`<p class="ok">` + esc(msg) + `</p>`)
	}
	if nova != "" {
		b.WriteString(`<div class="card" style="border-color:#2b6b45"><h2>Sua chave nova</h2><p>Copie agora: ela não aparece de novo.</p><p><code style="font-size:15px;user-select:all">` + esc(nova) + `</code></p></div>`)
	}
	b.WriteString(`<p>Cada tipo de conteúdo do site tem uma API, como no Payload CMS. Ler o que está publicado é livre; rascunhos, arquivos privados e mudanças pedem uma chave.</p>`)
	if len(ks) > 0 {
		b.WriteString(`<div class="scroll"><table><tr><th>Nome</th><th>Começo</th><th>Pode</th><th>Criada</th><th>Último uso</th><th></th></tr>`)
		for _, k := range ks {
			pode := "ler"
			if k.Escrita {
				pode = "ler e mudar"
			}
			uso := k.Uso
			if uso == "" {
				uso = "nunca"
			}
			b.WriteString(`<tr><td>` + esc(k.Nome) + `</td><td><code>` + esc(k.Prefixo) + `…</code></td><td>` + pode + `</td><td>` + esc(fmtWhen(k.Em)) + `</td><td>` + esc(uso) + `</td><td>` +
				`<form method="post" action="` + a.base + `/api" onsubmit="return confirm('Revogar esta chave?')">` + a.tok() + `<input type="hidden" name="acao" value="revogar"><input type="hidden" name="id" value="` + esc(k.ID) + `"><button class="ghost">Revogar</button></form></td></tr>`)
		}
		b.WriteString(`</table></div>`)
	}
	b.WriteString(`<form class="cms card" method="post" action="` + a.base + `/api">` + a.tok() + `<input type="hidden" name="acao" value="criar"><h2>Nova chave</h2>` +
		`<label>Para quê (ex.: aplicativo, outro site) <input name="nome" maxlength="60"></label>` +
		`<label class="check"><input type="checkbox" name="escrita"> Pode criar, editar e apagar</label><p><button>Criar chave</button></p></form>`)
	site := a.site()
	if site != nil && len(site.Contents) > 0 {
		if a.rq.site == nil {
			a.rq.site = site
		}
		base := a.s.siteURL(a.rq)
		b.WriteString(`<h2 style="font-size:1.05rem">Endereços</h2><div class="scroll"><table><tr><th>Tipo</th><th>Endereço</th></tr>`)
		for _, c := range site.Contents {
			if c.SemAPI {
				continue
			}
			b.WriteString(`<tr><td>` + esc(c.Title) + `</td><td><a href="` + esc(a.s.url("/api/"+c.Name)) + `" target="_blank"><code>/api/` + esc(c.Name) + `</code></a></td></tr>`)
		}
		for _, g := range site.Globals {
			b.WriteString(`<tr><td>` + esc(g.Title) + ` (global)</td><td><a href="` + esc(a.s.url("/api/globais/"+g.Name)) + `" target="_blank"><code>/api/globais/` + esc(g.Name) + `</code></a></td></tr>`)
		}
		b.WriteString(`</table></div>`)
		c := site.Contents[0]
		b.WriteString(`<details class="card"><summary>Exemplos</summary><pre style="white-space:pre-wrap;font-size:13px">` + esc(
			"# listar (10 por página), mais novos primeiro\ncurl '"+base+"/api/"+c.Name+"?limit=10&page=1'\n\n"+
				"# filtrar e ordenar\ncurl '"+base+"/api/"+c.Name+"?where[campo][contem]=texto&sort=-created_at'\n\n"+
				"# um item (pelo número ou pelo endereço)\ncurl '"+base+"/api/"+c.Name+"/1'\n\n"+
				"# criar\ncurl -X POST '"+base+"/api/"+c.Name+"' -H 'Authorization: Bearer SUA_CHAVE' -H 'Content-Type: application/json' -d '{\""+c.Fields[0].Name+"\": \"...\"}'\n\n"+
				"# editar só um campo\ncurl -X PATCH '"+base+"/api/"+c.Name+"/1' -H 'Authorization: Bearer SUA_CHAVE' -H 'Content-Type: application/json' -d '{\"publicado\": false}'\n\n"+
				"# apagar (vai para a lixeira)\ncurl -X DELETE '"+base+"/api/"+c.Name+"/1' -H 'Authorization: Bearer SUA_CHAVE'") + `</pre>` +
			`<p class="muted">Filtros: igual, diferente, contem, em, existe, maior, menor, maior_igual, menor_igual (ou os nomes do Payload: equals, contains, greater_than…). <code>depth=0</code> não traz os itens relacionados.</p></details>`)
	}
	return a.out("API", b.String(), true)
}
