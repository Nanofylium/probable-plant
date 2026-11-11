package web

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"html"
	"math"
	"net"
	"net/http"
	"net/mail"
	"net/url"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/Nanofylium/probable-plant/lang"
)

// Quantas instruções da VM um pedido pode executar (protege contra laço infinito).
const maxSteps = 40_000_000

func native(name string, f func(a []lang.Value) (lang.Value, error)) *lang.Native {
	return &lang.Native{Name: name, Fn: func(_ *lang.VM, a []lang.Value) (lang.Value, error) { return f(a) }}
}

func argv(a []lang.Value, i int) lang.Value {
	if i < len(a) {
		return a[i]
	}
	return nil
}

func toInt(v lang.Value) (int64, bool) {
	switch x := v.(type) {
	case int64:
		return x, true
	case float64:
		if x == math.Trunc(x) {
			return int64(x), true
		}
	case string:
		n, err := strconv.ParseInt(strings.TrimSpace(x), 10, 64)
		return n, err == nil
	}
	return 0, false
}

func stop(h halt) error { return &lang.Halt{Reason: h} }

func (rq *request) mod() *lang.Module { return rq.module }

// setup cria a VM do pedido com tudo que o código do site enxerga.
func (s *Server) setup(rq *request, page int, params []string) error {
	vm := lang.NewVM()
	vm.MaxSteps = maxSteps
	vm.RemoveStdModules("fs", "os", "http")
	vm.In = strings.NewReader("")
	rq.vm = vm
	rq.out = &strings.Builder{}
	rq.outs = nil
	vm.Out = rq.out
	rq.caps = map[int]string{}
	rq.formSt = map[int]*formState{}
	rq.formN = map[int]int{}
	rq.liveAt = map[int][2]int{}
	rq.module = lang.NewModule("site", s.file)
	mod := rq.module
	site := rq.site

	for _, d := range site.Data {
		v, err := lang.FromJSON(d.JSON)
		if err != nil {
			return &TemplateError{File: site.File, Line: d.Line, Msg: fmt.Sprintf("JSON inválido em <data name=\"%s\">: %v", d.Name, err)}
		}
		mod.Define(d.Name, v)
	}

	b := vm.Builtins
	delete(b, "input")
	def := func(name string, f func(a []lang.Value) (lang.Value, error)) { b[name] = native(name, f) }

	// ---------- saída do HTML ----------
	def("__w", func(a []lang.Value) (lang.Value, error) {
		i, _ := a[0].(int64)
		rq.out.WriteString(site.texts[i])
		return nil, nil
	})
	def("__e", func(a []lang.Value) (lang.Value, error) {
		writeVal(rq.out, argv(a, 0))
		return nil, nil
	})
	def("print", func(a []lang.Value) (lang.Value, error) {
		for i, v := range a {
			if i > 0 {
				rq.out.WriteByte(' ')
			}
			writeVal(rq.out, v)
		}
		rq.out.WriteByte('\n')
		return nil, nil
	})
	push := func() {
		rq.outs = append(rq.outs, rq.out)
		rq.out = &strings.Builder{}
		vm.Out = rq.out
	}
	pop := func() string {
		t := rq.out.String()
		rq.out = rq.outs[len(rq.outs)-1]
		rq.outs = rq.outs[:len(rq.outs)-1]
		vm.Out = rq.out
		return t
	}
	def("__cap", func(a []lang.Value) (lang.Value, error) { push(); return nil, nil })
	def("__endcap", func(a []lang.Value) (lang.Value, error) {
		i, _ := a[0].(int64)
		rq.caps[int(i)] = pop()
		return nil, nil
	})
	def("__wcap", func(a []lang.Value) (lang.Value, error) {
		i, _ := a[0].(int64)
		rq.out.WriteString(rq.caps[int(i)])
		return nil, nil
	})
	def("__vals", func(a []lang.Value) (lang.Value, error) {
		switch x := argv(a, 0).(type) {
		case nil:
			return lang.NewList(), nil
		case *lang.List:
			return x, nil
		case *lang.Table:
			return lang.NewList(x.Values()...), nil
		case string:
			l := lang.NewList()
			for _, r := range x {
				l.Items = append(l.Items, string(r))
			}
			return l, nil
		}
		return nil, fmt.Errorf("<each> precisa de uma lista ou tabela (veio %s)", lang.TypeName(argv(a, 0)))
	})
	def("__pairs", func(a []lang.Value) (lang.Value, error) {
		l := lang.NewList()
		switch x := argv(a, 0).(type) {
		case nil:
		case *lang.List:
			for i, v := range x.Items {
				l.Items = append(l.Items, lang.NewList(int64(i), v))
			}
		case *lang.Table:
			for i := 0; i < x.Len(); i++ {
				k, v := x.Entry(i)
				l.Items = append(l.Items, lang.NewList(k, v))
			}
		default:
			return nil, fmt.Errorf("<each> precisa de uma lista ou tabela (veio %s)", lang.TypeName(x))
		}
		return l, nil
	})
	def("__loop", func(a []lang.Value) (lang.Value, error) {
		i, _ := a[0].(int64)
		n := int64(0)
		if l, ok := a[1].(*lang.List); ok {
			n = int64(len(l.Items))
		}
		t := lang.NewTable()
		t.SetStr("index", i+1)
		t.SetStr("first", i == 0)
		t.SetStr("last", i == n-1)
		t.SetStr("count", n)
		return t, nil
	})
	def("__fo", func(a []lang.Value) (lang.Value, error) {
		i, _ := a[0].(int64)
		rq.forms = append(rq.forms, int(i))
		push()
		return nil, nil
	})
	def("__fa", func(a []lang.Value) (lang.Value, error) {
		rq.split = append(rq.split, rq.out.Len())
		return nil, nil
	})
	def("__fe", func(a []lang.Value) (lang.Value, error) {
		id := rq.forms[len(rq.forms)-1]
		rq.forms = rq.forms[:len(rq.forms)-1]
		sp := rq.split[len(rq.split)-1]
		rq.split = rq.split[:len(rq.split)-1]
		t := pop()
		rq.formN[id]++
		rq.out.WriteString(s.formHTML(rq, id, rq.formN[id]-1, t[:sp], t[sp:]))
		return nil, nil
	})
	def("__lo", func(a []lang.Value) (lang.Value, error) {
		i, _ := a[0].(int64)
		lv := site.Lives[i]
		rq.lives = true
		fmt.Fprintf(rq.out, `<div nep-live="%d"`, i)
		if lv.Every > 0 {
			fmt.Fprintf(rq.out, ` data-every="%d"`, lv.Every)
		}
		rq.out.WriteString(">")
		if _, done := rq.liveAt[int(i)]; !done && len(rq.outs) == 0 {
			rq.liveAt[int(i)] = [2]int{rq.out.Len(), -1}
		}
		return nil, nil
	})
	def("__lc", func(a []lang.Value) (lang.Value, error) {
		i, _ := a[0].(int64)
		if at, ok := rq.liveAt[int(i)]; ok && at[1] < 0 && len(rq.outs) == 0 {
			rq.liveAt[int(i)] = [2]int{at[0], rq.out.Len()}
		}
		rq.out.WriteString("</div>")
		return nil, nil
	})

	// ---------- funções para páginas ----------
	def("raw", func(a []lang.Value) (lang.Value, error) { return lang.HTML(lang.Str(argv(a, 0))), nil })
	def("nl2br", func(a []lang.Value) (lang.Value, error) {
		return lang.HTML(strings.ReplaceAll(html.EscapeString(lang.Str(argv(a, 0))), "\n", "<br>\n")), nil
	})
	// campo vazio (nil) não vira o texto "nil" na página
	def("markdown", func(a []lang.Value) (lang.Value, error) { return lang.HTML(Markdown(strOrEmpty(argv(a, 0)))), nil })
	def("comentarios", func(a []lang.Value) (lang.Value, error) { return s.commentsHTML(rq) })
	def("buscar", func(a []lang.Value) (lang.Value, error) {
		var only []string
		switch x := argv(a, 1).(type) {
		case string:
			only = []string{x}
		case *lang.List:
			for _, it := range x.Items {
				only = append(only, lang.Str(it))
			}
		}
		limit := 50
		if n, ok := toInt(argv(a, 2)); ok && n > 0 && n < 500 {
			limit = int(n)
		}
		return s.search(rq.site, strOrEmpty(argv(a, 0)), only, limit), nil
	})
	def("enviar_email", s.emailBuiltin(rq))
	def("send_email", s.emailBuiltin(rq))
	def("visual", func(a []lang.Value) (lang.Value, error) { return Visual(strOrEmpty(argv(a, 0))), nil })
	// textos editáveis (editavel="chave")
	def("__edhas", func(a []lang.Value) (lang.Value, error) {
		_, ok := s.edGet(lang.Str(argv(a, 0)))
		return ok, nil
	})
	def("__ed", func(a []lang.Value) (lang.Value, error) {
		k := lang.Str(argv(a, 0))
		v, _ := s.edGet(k)
		kind := "texto"
		if e := site.Editable(k); e != nil {
			kind = e.Kind
			if kind == "menu" {
				return s.menuHTML(e, rq.path), nil
			}
			if kind == "visual" {
				return Visual(v), nil
			}
		}
		return edHTML(kind, v), nil
	})
	def("menu", func(a []lang.Value) (lang.Value, error) { // menu("principal") no código: [{texto, url, ativo}]
		l := lang.NewList()
		e := site.Editable(lang.Str(argv(a, 0)))
		if e == nil || e.Kind != "menu" {
			return l, nil
		}
		for _, it := range s.menuItems(e) {
			t := lang.NewTable()
			t.SetStr("texto", it.Text)
			t.SetStr("url", s.menuHref(it.URL))
			t.SetStr("ativo", samePath(it.URL, rq.path))
			l.Items = append(l.Items, t)
		}
		return l, nil
	})
	def("__edsrc", func(a []lang.Value) (lang.Value, error) {
		k := lang.Str(argv(a, 0))
		if v, ok := s.edGet(k); ok && v != "" {
			return v, nil
		}
		if e := site.Editable(k); e != nil {
			return e.Default, nil
		}
		return "", nil
	})
	def("texto", func(a []lang.Value) (lang.Value, error) { // texto("chave", "padrão") no código
		if v, ok := s.edGet(lang.Str(argv(a, 0))); ok {
			return v, nil
		}
		return argv(a, 1), nil
	})
	def("urlencode", func(a []lang.Value) (lang.Value, error) { return url.QueryEscape(lang.Str(argv(a, 0))), nil })
	def("json", func(a []lang.Value) (lang.Value, error) {
		j, err := lang.ToJSON(argv(a, 0), "")
		if err != nil {
			return nil, err
		}
		j = strings.NewReplacer("<", `<`, ">", `>`, "&", `&`, "'", `'`).Replace(j)
		return lang.HTML(j), nil
	})
	def("url", func(a []lang.Value) (lang.Value, error) { return s.url(lang.Str(argv(a, 0))), nil })
	def("not_found", func(a []lang.Value) (lang.Value, error) { return nil, stop(halt{kind: "not_found"}) })
	def("redirect", func(a []lang.Value) (lang.Value, error) {
		to := lang.Str(argv(a, 0))
		if !localPath(to) {
			to = "/"
		}
		return nil, stop(halt{kind: "redirect", to: s.url(to)})
	})
	def("abort", func(a []lang.Value) (lang.Value, error) {
		code, ok := toInt(argv(a, 0))
		if !ok || code < 400 || code > 599 {
			code = 400
		}
		msg := lang.Str(argv(a, 1))
		if argv(a, 1) == nil {
			msg = http.StatusText(int(code))
		}
		return nil, stop(halt{kind: "abort", status: int(code), msg: msg})
	})
	def("reject", func(a []lang.Value) (lang.Value, error) {
		return nil, stop(halt{kind: "reject", msg: lang.Str(argv(a, 0))})
	})
	b["recusar"] = b["reject"]
	if orig, ok := b["load_csv"].(*lang.Native); ok {
		b["load_csv"] = &lang.Native{Name: "load_csv", Fn: func(vm *lang.VM, a []lang.Value) (lang.Value, error) {
			p, ok := argv(a, 0).(string)
			if !ok {
				return nil, fmt.Errorf("load_csv: informe o caminho do arquivo")
			}
			full, ok := s.inside(p)
			if !ok {
				return nil, fmt.Errorf("load_csv: só arquivos dentro da pasta do site")
			}
			return orig.Fn(vm, []lang.Value{full})
		}}
	}

	// ---------- variáveis prontas ----------
	mod.Define("db", s.dbTable(rq))
	pt := lang.NewTable()
	if page >= 0 {
		pg := site.Pages[page]
		for i, n := range pg.params {
			if i < len(params) {
				pt.SetStr(n, params[i])
			}
		}
		pv := lang.NewTable()
		pv.SetStr("route", pg.Route)
		pv.SetStr("title", pg.Title)
		pv.SetStr("description", pg.Description)
		pv.SetStr("image", pg.Image)
		pv.SetStr("path", rq.path)
		mod.Define("page", pv)
		if pg.Conteudo != "" { // <page conteudo="artigos">: o item vem pelo endereço
			c := site.Content(pg.Conteudo)
			var key lang.Value
			if len(params) > 0 {
				key = params[0]
			}
			it := s.findItem(c.Name, key)
			if it == nil || (!published(it) && !rq.sess.isAdmin()) {
				return stop(halt{kind: "not_found"})
			}
			mod.Define("item", it)
			rq.item, rq.itemCol = it, c.Name
			if pg.Title == "" {
				pv.SetStr("title", itemTitle(c, it))
			}
			if d := itemDescription(c, it); d != "" && pg.Description == "" {
				pv.SetStr("description", d)
			}
			if im := itemImage(c, it); im != "" && pg.Image == "" {
				pv.SetStr("image", im)
			}
		}
	}
	mod.Define("params", pt)
	req := lang.NewTable()
	req.SetStr("method", rq.r.Method)
	req.SetStr("path", rq.path)
	req.SetStr("live", rq.r.URL.Query().Get("_nep_live") != "")
	q := lang.NewTable()
	for k, v := range rq.r.URL.Query() {
		if k != "_nep_live" && len(v) > 0 {
			q.SetStr(k, v[0])
		}
	}
	req.SetStr("query", q)
	mod.Define("request", req)
	mod.Define("admin", rq.sess.isAdmin())

	var sv lang.Value = lang.NewTable()
	if len(rq.sess.Data) > 0 {
		if v, err := lang.FromJSON(string(rq.sess.Data)); err == nil {
			sv = v
		}
	}
	rq.sessVal = sv
	rq.sessRaw, _ = lang.ToJSON(sv, "")
	mod.Define("session", sv)
	return nil
}

func (rq *request) pageID() int {
	if rq.page == nil {
		return -1
	}
	return rq.page.ID
}

func writeVal(b *strings.Builder, v lang.Value) {
	switch x := v.(type) {
	case nil:
	case lang.HTML:
		b.WriteString(string(x))
	case string:
		b.WriteString(html.EscapeString(x))
	default:
		b.WriteString(html.EscapeString(lang.Str(v)))
	}
}

// inside resolve um caminho relativo à pasta do site, recusando a pasta de dados e o que fica fora.
func (s *Server) inside(p string) (string, bool) {
	full := filepath.Join(s.dir, filepath.FromSlash(p))
	rel, err := filepath.Rel(s.dir, full)
	if err != nil || strings.HasPrefix(rel, "..") {
		return "", false
	}
	if dd, err := filepath.Abs(s.opt.DataDir); err == nil {
		if r2, err := filepath.Rel(dd, full); err == nil && !strings.HasPrefix(r2, "..") {
			return "", false
		}
	}
	return full, true
}

// dbTable é o objeto `db` que o código do site usa.
func (s *Server) dbTable(rq *request) *lang.Table {
	t := lang.NewTable()
	// rascunhos de <conteudo> só aparecem para quem está logado no painel
	hide := func(c string) bool {
		return rq != nil && rq.site != nil && rq.site.Content(c) != nil && (rq.sess == nil || !rq.sess.isAdmin())
	}
	visible := func(c string, limit int) []*Record {
		if !hide(c) {
			return s.Store.All(c, limit)
		}
		var out []*Record
		for _, r := range s.Store.All(c, 0) {
			if published(r.ToValue()) {
				out = append(out, r)
				if limit > 0 && len(out) == limit {
					break
				}
			}
		}
		return out
	}
	col := func(fn string, v lang.Value) (string, error) {
		c, _ := v.(string)
		if !ValidCol(c) {
			return "", fmt.Errorf("db.%s: nome de coleção inválido: %s", fn, lang.Repr(v))
		}
		return c, nil
	}
	list := func(rs []*Record) *lang.List {
		l := lang.NewList()
		for _, r := range rs {
			l.Items = append(l.Items, r.ToValue())
		}
		return l
	}
	def := func(name string, f func(a []lang.Value) (lang.Value, error)) { t.SetStr(name, native("db."+name, f)) }
	def("all", func(a []lang.Value) (lang.Value, error) {
		c, err := col("all", argv(a, 0))
		if err != nil {
			return nil, err
		}
		n, _ := toInt(argv(a, 1))
		return list(visible(c, int(n))), nil
	})
	def("find", func(a []lang.Value) (lang.Value, error) {
		c, err := col("find", argv(a, 0))
		if err != nil {
			return nil, err
		}
		it := s.findItem(c, argv(a, 1)) // número ou endereço (slug)
		if it == nil || (hide(c) && !published(it)) {
			return nil, nil
		}
		return it, nil
	})
	def("where", func(a []lang.Value) (lang.Value, error) {
		c, err := col("where", argv(a, 0))
		if err != nil {
			return nil, err
		}
		field, want := argv(a, 1), argv(a, 2)
		l := lang.NewList()
		for _, r := range visible(c, 0) {
			v := r.ToValue()
			if fv, _ := v.Get(field); lang.Equal(fv, want) {
				l.Items = append(l.Items, v)
			}
		}
		return l, nil
	})
	def("count", func(a []lang.Value) (lang.Value, error) {
		c, err := col("count", argv(a, 0))
		if err != nil {
			return nil, err
		}
		if hide(c) {
			return int64(len(visible(c, 0))), nil
		}
		return int64(s.Store.Count(c)), nil
	})
	def("insert", func(a []lang.Value) (lang.Value, error) {
		c, err := col("insert", argv(a, 0))
		if err != nil {
			return nil, err
		}
		d, ok := argv(a, 1).(*lang.Table)
		if !ok {
			return nil, fmt.Errorf("db.insert: o segundo argumento precisa ser uma tabela")
		}
		id, err := s.Store.Insert(c, d)
		return id, err
	})
	def("update", func(a []lang.Value) (lang.Value, error) {
		c, err := col("update", argv(a, 0))
		if err != nil {
			return nil, err
		}
		id, _ := toInt(argv(a, 1))
		d, ok := argv(a, 2).(*lang.Table)
		if !ok {
			return nil, fmt.Errorf("db.update: o terceiro argumento precisa ser uma tabela")
		}
		return s.Store.Update(c, id, d)
	})
	def("incr", func(a []lang.Value) (lang.Value, error) {
		c, err := col("incr", argv(a, 0))
		if err != nil {
			return nil, err
		}
		id, _ := toInt(argv(a, 1))
		field, ok := argv(a, 2).(string)
		if !ok {
			return nil, fmt.Errorf("db.incr: informe o nome do campo")
		}
		n := 1.0
		switch x := argv(a, 3).(type) {
		case int64:
			n = float64(x)
		case float64:
			n = x
		}
		return s.Store.Incr(c, id, field, n)
	})
	def("delete", func(a []lang.Value) (lang.Value, error) {
		c, err := col("delete", argv(a, 0))
		if err != nil {
			return nil, err
		}
		id, _ := toInt(argv(a, 1))
		return s.Store.Delete(c, id)
	})
	return t
}

// ---------- formulários ----------

const honeypot = `<div style="position:absolute;left:-10000px;width:1px;height:1px;overflow:hidden" aria-hidden="true">` +
	`<label>Não preencha<input type="text" name="_nep_hp" tabindex="-1" autocomplete="off"></label></div>`

func (s *Server) formHTML(rq *request, id, inst int, attrs, inner string) string {
	var b strings.Builder
	if rq.site.Forms[id].HasFile && !strings.Contains(strings.ToLower(attrs), "enctype") {
		attrs += ` enctype="multipart/form-data"`
	}
	b.WriteString(`<form method="post"` + attrs + `>`)
	b.WriteString(`<input type="hidden" name="_nep_form" value="` + strconv.Itoa(id) + `">`)
	if inst > 0 {
		b.WriteString(`<input type="hidden" name="_nep_i" value="` + strconv.Itoa(inst) + `">`)
	}
	b.WriteString(`<input type="hidden" name="_nep_csrf" value="` + rq.sess.CSRF + `">`)
	b.WriteString(honeypot)
	if m := rq.sess.takeFlash(id, inst); m != "" {
		b.WriteString(`<div class="nep-ok" role="status">` + html.EscapeString(m) + `</div>`)
	}
	if st := rq.formSt[id]; st != nil && st.inst == inst {
		b.WriteString(`<div class="nep-erros" role="alert"><ul>`)
		for _, e := range st.errors {
			b.WriteString("<li>" + html.EscapeString(e) + "</li>")
		}
		b.WriteString("</ul></div>")
		inner = refill(inner, st.old)
	}
	b.WriteString(inner)
	b.WriteString("</form>")
	return b.String()
}

func (s *Server) ipKey(r *http.Request) string {
	salt, _ := s.Store.MetaOrInit("salt", func() string { return randHex(16) })
	ip := r.RemoteAddr
	if h, _, err := net.SplitHostPort(ip); err == nil {
		ip = h
	}
	m := hmac.New(sha256.New, []byte(salt))
	m.Write([]byte(ip))
	return hex.EncodeToString(m.Sum(nil))[:32] // o IP nunca é gravado em claro (LGPD)
}

// handlePost processa um formulário. done=true quando a resposta (redirecionamento) já foi enviada.
func (s *Server) handlePost(rq *request) (bool, error) {
	r := rq.r
	if strings.HasPrefix(r.Header.Get("Content-Type"), "multipart/form-data") {
		r.Body = http.MaxBytesReader(rq.w, r.Body, maxUpload)
		if err := r.ParseMultipartForm(8 << 20); err != nil {
			return false, stop(halt{kind: "abort", status: 400, msg: "Envio inválido ou arquivos grandes demais (máximo 25 MB no total)."})
		}
		defer r.MultipartForm.RemoveAll()
	} else {
		r.Body = http.MaxBytesReader(rq.w, r.Body, 1<<20)
		if err := r.ParseForm(); err != nil {
			return false, stop(halt{kind: "abort", status: 400, msg: "Envio inválido ou grande demais."})
		}
	}
	id, err := strconv.Atoi(r.PostForm.Get("_nep_form"))
	if err != nil || id < 0 || id >= len(rq.site.Forms) || !rq.site.formOn(rq.page.ID, id) {
		return false, stop(halt{kind: "abort", status: 400, msg: "Envio inválido."})
	}
	f := rq.site.Forms[id]
	inst, _ := strconv.Atoi(r.PostForm.Get("_nep_i"))
	back := s.opt.Base + rq.path
	if r.URL.RawQuery != "" {
		back += "?" + r.URL.RawQuery
	}
	ok := back
	if f.Redirect != "" && localPath(f.Redirect) {
		ok = s.url(f.Redirect)
	}
	old := map[string][]string{}
	for _, fd := range f.Fields {
		old[fd.Name] = r.PostForm[fd.Name+map[bool]string{true: "[]", false: ""}[fd.Multi]]
	}
	fail := func(msgs ...string) (bool, error) {
		rq.formSt[id] = &formState{errors: msgs, old: old, inst: inst}
		return false, nil
	}
	if !rq.sess.csrfOK(r.PostForm.Get("_nep_csrf")) {
		return fail("Sua sessão expirou. Confira os dados e envie de novo.")
	}
	if r.PostForm.Get("_nep_hp") != "" { // robô caiu na armadilha: finge sucesso
		if f.Success != "" {
			rq.sess.setFlash(id, inst, f.Success)
		}
		s.redirect(rq, ok, http.StatusSeeOther)
		return true, nil
	}
	rk := fmt.Sprintf("form:%d:%s", id, s.ipKey(r))
	if allowed, _ := s.Store.Rate(rk, f.Limit, 600, false); !allowed {
		return fail("Muitos envios em pouco tempo. Tente de novo em alguns minutos.")
	}
	data, errs := validate(f.Fields, r.PostForm)
	files, ferrs := checkFiles(f.Fields, r)
	errs = append(errs, ferrs...)
	if len(errs) > 0 {
		return fail(errs...)
	}
	saved, err := s.saveFiles(files, data)
	if err != nil {
		return false, err
	}
	keep := false
	defer func() {
		if !keep {
			s.Store.removeFiles(saved)
		}
	}()
	if f.Run != "" {
		fn, has := rq.module.Globals[f.Run]
		if !has {
			return false, fmt.Errorf("o formulário da linha %d chama run=\"%s\", mas essa função não existe", f.Line, f.Run)
		}
		res, err := rq.vm.Call(fn, []lang.Value{data})
		if err != nil {
			if h, isH := haltOf(err); isH && h.kind == "reject" {
				return fail(h.msg)
			}
			return false, err
		}
		if to, isStr := res.(string); isStr && localPath(to) {
			ok = s.url(to)
		}
	}
	_, _ = s.Store.Rate(rk, math.MaxInt32, 600, true) // conta só envios aceitos
	if f.Save != "" {
		if _, err := s.Store.Insert(f.Save, data); err != nil {
			return false, err
		}
	}
	keep = f.Save != "" || f.Run != "" // com run=, a função decide o que fazer com os arquivos
	s.formEmails(rq, f, data)
	if f.Success != "" {
		rq.sess.setFlash(id, inst, f.Success)
	}
	s.redirect(rq, ok, http.StatusSeeOther)
	return true, nil
}

var (
	emailRe = regexp.MustCompile(`^[^\s@<>()",;:]+@[^\s@<>()",;:]+\.[^\s@<>()",;:]{2,}$`)
	telRe   = regexp.MustCompile(`^[0-9+()\-.\s]{8,20}$`)
	dateRe  = regexp.MustCompile(`^\d{4}-\d{2}-\d{2}$`)
)

func fmtNum(f float64) string { return strconv.FormatFloat(f, 'f', -1, 64) }

// validate confere no servidor as mesmas regras escritas no HTML.
func validate(fields []*Field, in url.Values) (*lang.Table, []string) {
	data := lang.NewTable()
	var errs []string
	for _, f := range fields {
		label := f.Label
		if f.Type == "file" {
			continue // conferidos em checkFiles
		}
		if f.Multi {
			var vals []lang.Value
			bad := false
			for _, v := range in[f.Name+"[]"] {
				v = strings.TrimSpace(v)
				if v == "" {
					continue
				}
				if f.Options != nil && !hasOption(f.Options, v) {
					bad = true
				}
				vals = append(vals, v)
			}
			if f.Required && len(vals) == 0 {
				errs = append(errs, fmt.Sprintf("Escolha pelo menos uma opção em \"%s\".", label))
				continue
			}
			if bad {
				errs = append(errs, fmt.Sprintf("Opção inválida em \"%s\".", label))
				continue
			}
			data.SetStr(f.Name, lang.NewList(vals...))
			continue
		}
		if len(in[f.Name]) > 1 && f.Type != "checkbox" {
			errs = append(errs, fmt.Sprintf("Valor inválido em \"%s\".", label))
			continue
		}
		v := strings.TrimSpace(strings.ReplaceAll(in.Get(f.Name), "\x00", ""))
		max := f.MaxLen
		if max <= 0 {
			max = 500
			if f.Type == "textarea" {
				max = 5000
			}
			if f.Builder {
				max = 300000
			}
		}
		if v == "" {
			if f.Required {
				errs = append(errs, fmt.Sprintf("Preencha o campo \"%s\".", label))
			}
			if f.Type == "checkbox" {
				data.SetStr(f.Name, false)
			} else {
				data.SetStr(f.Name, "")
			}
			continue
		}
		if !utf8.ValidString(v) {
			errs = append(errs, fmt.Sprintf("Texto inválido em \"%s\".", label))
			continue
		}
		if utf8.RuneCountInString(v) > max {
			errs = append(errs, fmt.Sprintf("\"%s\" pode ter no máximo %d caracteres.", label, max))
			continue
		}
		var val lang.Value = v
		switch f.Type {
		case "email":
			if a, err := mail.ParseAddress(v); err != nil || a.Address != v || !emailRe.MatchString(v) {
				errs = append(errs, fmt.Sprintf("Informe um e-mail válido em \"%s\".", label))
				continue
			}
		case "url":
			u, err := url.Parse(v)
			if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
				errs = append(errs, fmt.Sprintf("Informe um endereço (http/https) válido em \"%s\".", label))
				continue
			}
		case "number", "range":
			n, err := strconv.ParseFloat(strings.Replace(v, ",", ".", 1), 64)
			if err != nil || math.IsNaN(n) || math.IsInf(n, 0) {
				errs = append(errs, fmt.Sprintf("\"%s\" precisa ser um número.", label))
				continue
			}
			if f.Min != nil && n < *f.Min {
				errs = append(errs, fmt.Sprintf("\"%s\" precisa ser no mínimo %s.", label, fmtNum(*f.Min)))
				continue
			}
			if f.Max != nil && n > *f.Max {
				errs = append(errs, fmt.Sprintf("\"%s\" precisa ser no máximo %s.", label, fmtNum(*f.Max)))
				continue
			}
			if n == math.Trunc(n) && math.Abs(n) < 1e15 {
				val = int64(n)
			} else {
				val = n
			}
		case "date":
			if _, err := time.Parse("2006-01-02", v); err != nil || !dateRe.MatchString(v) {
				errs = append(errs, fmt.Sprintf("Data inválida em \"%s\".", label))
				continue
			}
		case "tel":
			if !telRe.MatchString(v) {
				errs = append(errs, fmt.Sprintf("Telefone inválido em \"%s\".", label))
				continue
			}
		}
		if f.Options != nil && !hasOption(f.Options, v) {
			errs = append(errs, fmt.Sprintf("Opção inválida em \"%s\".", label))
			continue
		}
		if f.Type == "checkbox" {
			val = true
		}
		data.SetStr(f.Name, val)
	}
	return data, errs
}

var (
	inputRe    = regexp.MustCompile(`(?is)<input\b((?:[^>"']|"[^"]*"|'[^']*')*)>`)
	textareaRe = regexp.MustCompile(`(?is)(<textarea\b((?:[^>"']|"[^"]*"|'[^']*')*)>)(.*?)(</textarea>)`)
	selRe      = regexp.MustCompile(`(?is)(<select\b((?:[^>"']|"[^"]*"|'[^']*')*)>)(.*?)(</select>)`)
	checkedRe  = regexp.MustCompile(`(?i)\schecked(\s*=\s*("[^"]*"|'[^']*'|[^\s>]+))?`)
	selectedRe = regexp.MustCompile(`(?i)\sselected(\s*=\s*("[^"]*"|'[^']*'|[^\s>]+))?`)
	valueRe    = regexp.MustCompile(`(?i)\svalue\s*=\s*("[^"]*"|'[^']*'|[^\s>]+)`)
)

func contains(list []string, v string) bool {
	for _, x := range list {
		if x == v {
			return true
		}
	}
	return false
}

// checkedMark marca uma caixa de seleção como marcada em refill, qualquer que seja seu value.
const checkedMark = "\x00marcado"

// refill recoloca no formulário o que a pessoa digitou quando o envio volta com erro.
func refill(h string, old map[string][]string) string {
	h = inputRe.ReplaceAllStringFunc(h, func(tag string) string {
		m := inputRe.FindStringSubmatch(tag)
		at := parseAttrs(m[1])
		name := strings.TrimSuffix(at["name"], "[]")
		typ := strings.ToLower(at["type"])
		vals, ok := old[name]
		if name == "" || !ok {
			return tag
		}
		switch typ {
		case "password", "hidden", "submit", "button", "file", "image", "reset":
			return tag
		}
		body := strings.TrimRight(m[1], " /")
		if typ == "checkbox" || typ == "radio" {
			val, has := at["value"]
			if !has {
				val = "on"
			}
			body = checkedRe.ReplaceAllString(body, "")
			if contains(vals, val) || (typ == "checkbox" && contains(vals, checkedMark)) {
				return "<input" + body + " checked>"
			}
			return "<input" + body + ">"
		}
		body = valueRe.ReplaceAllString(body, "")
		return "<input" + body + ` value="` + html.EscapeString(strings.Join(vals, ", ")) + `">`
	})
	h = textareaRe.ReplaceAllStringFunc(h, func(tag string) string {
		m := textareaRe.FindStringSubmatch(tag)
		vals, ok := old[parseAttrs(m[2])["name"]]
		if !ok {
			return tag
		}
		return m[1] + html.EscapeString(strings.Join(vals, "\n")) + m[4]
	})
	h = selRe.ReplaceAllStringFunc(h, func(tag string) string {
		m := selRe.FindStringSubmatch(tag)
		vals, ok := old[strings.TrimSuffix(parseAttrs(m[2])["name"], "[]")]
		if !ok {
			return tag
		}
		inner := m[3]
		var ob strings.Builder
		last := 0
		for _, loc := range optionRe.FindAllStringSubmatchIndex(inner, -1) {
			ob.WriteString(inner[last:loc[0]])
			last = loc[1]
			attrs := inner[loc[2]:loc[3]]
			at := parseAttrs(attrs)
			body := selectedRe.ReplaceAllString(attrs, "")
			v, has := at["value"]
			v = html.UnescapeString(v)
			if !has { // <option>Texto</option>: o valor é o próprio texto
				rest := inner[loc[1]:]
				if e := strings.IndexByte(rest, '<'); e >= 0 {
					rest = rest[:e]
				}
				v, has = strings.TrimSpace(html.UnescapeString(rest)), true
			}
			if has && contains(vals, v) {
				ob.WriteString("<option" + body + " selected>")
			} else {
				ob.WriteString("<option" + body + ">")
			}
		}
		ob.WriteString(inner[last:])
		opts := ob.String()
		return m[1] + opts + m[4]
	})
	return h
}

// strOrEmpty: como lang.Str, mas nil vira "" (campo opcional que ficou vazio).
func strOrEmpty(v lang.Value) string {
	if v == nil {
		return ""
	}
	return lang.Str(v)
}
