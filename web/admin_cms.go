package web

// Telas do painel para o lado CMS: tipos de conteúdo (/_admin/conteudo) e
// textos editáveis do site (/_admin/textos).

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/Nanofylium/probable-plant/lang"
)

const cmsCSS = `<style>.cms label{display:grid;gap:4px;margin:12px 0;font-weight:600}.cms input:not([type=checkbox]):not([type=radio]),.cms textarea,.cms select{font:inherit;font-weight:400;padding:8px 10px;border:1px solid #c9cfc5;border-radius:8px;width:100%;box-sizing:border-box}` +
	`.cms textarea{min-height:90px}.cms textarea[data-editor]{min-height:260px;font-family:ui-monospace,Menlo,Consolas,monospace;font-size:14px}.cms .check{display:flex;gap:8px;align-items:center;font-weight:600}` +
	`.tag{display:inline-block;font-size:12px;padding:1px 8px;border-radius:99px;background:#e9f5ec;color:#2b6b45}.tag.off{background:#f3eee2;color:#7a5b12}` +
	`.card{border:1px solid #e5e8e2;border-radius:10px;padding:12px 14px;margin:12px 0}.card h2{font-size:1rem;margin:0 0 6px}.card img{max-height:120px;border-radius:8px;display:block;margin:6px 0}` +
	`a.btn{display:inline-block;background:#2b6b45;color:#fff;padding:8px 12px;border-radius:8px;text-decoration:none}</style>`

const mdHint = `<p class="muted">Nos textos longos você pode usar: <code>**negrito**</code>, <code>*itálico*</code>, <code># Título</code>, <code>- item de lista</code>, <code>[link](https://...)</code> e <code>&gt; citação</code>. Uma linha em branco separa parágrafos.</p>`

// navCMS acrescenta ao menu do painel os tipos de conteúdo e os textos.
func (a *adminPage) navCMS() string {
	site := a.site()
	if site == nil {
		return ""
	}
	n := ""
	for _, c := range site.Contents {
		n += `<a href="` + a.base + `/conteudo?c=` + url.QueryEscape(c.Name) + `">` + esc(c.Title) + `</a>`
	}
	if len(site.Editables) > 0 {
		n += `<a href="` + a.base + `/textos">Textos e menus</a>`
	}
	n += `<a href="` + a.base + `/midia">Mídia</a>`
	if p := a.pendingComments(); p > 0 {
		n += `<a href="` + a.base + `/comentarios">Comentários (` + strconv.Itoa(p) + `)</a>`
	} else if a.s.Store.Count(colComments) > 0 || site.usesComments() {
		n += `<a href="` + a.base + `/comentarios">Comentários</a>`
	}
	if a.s.Store.Count(colTrash) > 0 {
		n += `<a href="` + a.base + `/lixeira">Lixeira</a>`
	}
	return n
}

func (a *adminPage) site() *Site {
	if a.st == nil {
		a.st, _ = a.s.load()
	}
	return a.st
}

func (a *adminPage) content() *Content {
	site := a.site()
	if site == nil {
		return nil
	}
	return site.Content(a.rq.r.URL.Query().Get("c"))
}

func fmtWhen(s string) string {
	if tm, err := time.ParseInLocation("2006-01-02 15:04:05", s, time.Local); err == nil {
		return tm.Format("02/01/2006 15:04")
	}
	return s
}

// contentList é /_admin/conteudo?c=artigos.
func (a *adminPage) contentList() error {
	c := a.content()
	if c == nil {
		a.s.simple(a.rq, 404, "Conteúdo não encontrado", "")
		return nil
	}
	site := a.site()
	var b strings.Builder
	b.WriteString(cmsCSS)
	if a.rq.r.URL.Query().Get("ok") != "" {
		b.WriteString(`<p class="ok">Salvo.</p>`)
	}
	ec := url.QueryEscape(c.Name)
	b.WriteString(`<p class="row"><a class="btn" href="` + a.base + `/conteudo/editar?c=` + ec + `">+ Novo ` + esc(c.Item) + `</a></p>`)
	recs := a.s.Store.All(c.Name, 0)
	if len(recs) == 0 {
		b.WriteString(`<p class="muted">Nada aqui ainda. Crie o primeiro ` + esc(c.Item) + `.</p>`)
		return a.out(c.Title, b.String(), true)
	}
	b.WriteString(`<div class="scroll"><table><tr><th>Título</th><th>Situação</th><th>Atualizado</th><th></th></tr>`)
	for _, r := range recs {
		t := r.ToValue()
		title := itemTitle(c, t)
		if title == "" {
			title = "(sem título)"
		}
		st := `<span class="tag">Publicado</span>`
		pv, hasPub := t.Get("publicado")
		if w, future := scheduledFuture(t); future && (!hasPub || pv != false) {
			st = `<span class="tag off">Agendado: ` + esc(w.Format("02/01/2006 15:04")) + `</span>`
		} else if !published(t) {
			st = `<span class="tag off">Rascunho</span>`
		}
		when := r.Updated
		if when == "" {
			when = r.Created
		}
		id := strconv.FormatInt(r.ID, 10)
		b.WriteString(`<tr><td><a href="` + a.base + `/conteudo/editar?c=` + ec + `&id=` + id + `">` + esc(title) + `</a>`)
		if u := site.itemURL(c.Name, t); u != "" {
			b.WriteString(`<div class="muted">` + esc(u) + `</div>`)
		}
		b.WriteString(`</td><td>` + st + `</td><td>` + esc(fmtWhen(when)) + `</td><td class="row">`)
		if u := site.itemURL(c.Name, t); u != "" {
			b.WriteString(`<a href="` + esc(a.s.url(u)) + `" target="_blank">Ver</a>`)
		}
		b.WriteString(`<form method="post" action="` + a.base + `/delete" onsubmit="return confirm('Mandar este ` + esc(c.Item) + ` para a lixeira? Dá para restaurar por 30 dias.')">` + a.tok() +
			`<input type="hidden" name="c" value="` + esc(c.Name) + `"><input type="hidden" name="id" value="` + id + `"><input type="hidden" name="volta" value="conteudo?c=` + esc(ec) + `"><button class="ghost">Apagar</button></form></td></tr>`)
	}
	b.WriteString(`</table></div>`)
	return a.out(c.Title, b.String(), true)
}

// oldValues transforma um registro nos valores que o formulário mostra.
func oldValues(c *Content, t *lang.Table) map[string][]string {
	old := map[string][]string{}
	for _, f := range c.Fields {
		v, ok := t.Get(f.Name)
		if !ok || v == nil {
			continue
		}
		switch x := v.(type) {
		case bool:
			if x {
				old[f.Name] = []string{checkedMark}
			} else {
				old[f.Name] = []string{}
			}
		case *lang.List:
			for _, it := range x.Items {
				old[f.Name] = append(old[f.Name], lang.Str(it))
			}
		case *lang.Table:
		default:
			old[f.Name] = []string{lang.Str(v)}
		}
	}
	return old
}

// contentEdit é /_admin/conteudo/editar?c=artigos[&id=3].
func (a *adminPage) contentEdit(post bool) error {
	c := a.content()
	if c == nil {
		a.s.simple(a.rq, 404, "Conteúdo não encontrado", "")
		return nil
	}
	r := a.rq.r
	id, _ := strconv.ParseInt(r.URL.Query().Get("id"), 10, 64)
	var cur *lang.Table
	var curRec *Record
	if id > 0 {
		curRec = a.s.Store.Find(c.Name, id)
		if curRec == nil {
			a.s.simple(a.rq, 404, "Item não encontrado", "<p>Ele pode ter sido apagado.</p>")
			return nil
		}
		cur = curRec.ToValue()
	}
	var errs []string
	old := map[string][]string{}
	slug, pub, sched := "", true, ""
	if cur != nil {
		old = oldValues(c, cur)
		if v, _ := cur.Get("slug"); v != nil {
			slug = lang.Str(v)
		}
		v, has := cur.Get("publicado")
		pub = !has || v != false
		if w, ok := parseSched(getS(cur, schedField)); ok {
			sched = w.Format("2006-01-02T15:04")
		}
	}
	if post {
		// arquivo obrigatório já enviado antes não precisa ser reenviado
		fields := make([]*Field, len(c.Fields))
		for i, f := range c.Fields {
			cp := *f
			if f.Type == "file" && cur != nil {
				if v, ok := cur.Get(f.Name); ok && v != nil {
					cp.Required = false
				}
			}
			fields[i] = &cp
		}
		data, verrs := validate(fields, r.PostForm)
		files, ferrs := checkFiles(fields, r)
		errs = append(verrs, ferrs...)
		for _, f := range c.Fields {
			old[f.Name] = r.PostForm[f.Name+map[bool]string{true: "[]", false: ""}[f.Multi]]
			if f.Type == "checkbox" && r.PostForm.Get(f.Name) != "" {
				old[f.Name] = []string{checkedMark}
			}
		}
		slug = strings.TrimSpace(r.PostForm.Get("_slug"))
		pub = r.PostForm.Get("_publicado") != ""
		sched = strings.TrimSpace(r.PostForm.Get("_publicar_em"))
		if slug != "" && !slugRe.MatchString(slugify(slug)) {
			errs = append(errs, "O endereço precisa ter letras ou números.")
		}
		if _, ok := parseSched(sched); sched != "" && !ok {
			errs = append(errs, "Data de publicação inválida.")
		}
		if len(errs) == 0 {
			want := slugify(slug)
			if want == "" {
				want = slugify(itemTitle(c, data))
			}
			data.SetStr("slug", a.s.uniqueSlug(c.Name, want, id))
			data.SetStr("publicado", pub)
			if w, ok := parseSched(sched); ok {
				data.SetStr(schedField, w.Format("2006-01-02T15:04"))
			} else {
				data.SetStr(schedField, nil)
			}
			for _, f := range c.Fields {
				if f.Type == "file" && r.PostForm.Get("_remover_"+f.Name) != "" {
					data.SetStr(f.Name, nil)
				}
			}
			saved, err := a.s.saveFiles(files, data)
			if err != nil {
				return err
			}
			if id > 0 {
				a.s.saveVersion(c.Name, curRec, a.who())
				before := filesIn(curRec.Data)
				if _, err := a.s.Store.Update(c.Name, id, data); err != nil {
					a.s.Store.removeFiles(saved)
					return err
				}
				if now := a.s.Store.Find(c.Name, id); now != nil {
					still := map[string]bool{}
					for _, f := range filesIn(now.Data) {
						still[f] = true
					}
					var gone []string
					for _, f := range before {
						if !still[f] {
							gone = append(gone, f)
						}
					}
					a.s.Store.removeFiles(gone) // arquivo trocado ou removido não fica para trás
				}
			} else if _, err := a.s.Store.Insert(c.Name, data); err != nil {
				a.s.Store.removeFiles(saved)
				return err
			}
			a.s.redirect(a.rq, a.base+"/conteudo?c="+url.QueryEscape(c.Name)+"&ok=1", http.StatusSeeOther)
			return nil
		}
	}

	var b strings.Builder
	b.WriteString(cmsCSS)
	if r.URL.Query().Get("restaurado") != "" {
		b.WriteString(`<p class="ok">Versão restaurada. A anterior ficou no histórico.</p>`)
	}
	for _, e := range errs {
		b.WriteString(errP(e))
	}
	q := "c=" + url.QueryEscape(c.Name)
	if id > 0 {
		q += "&id=" + strconv.FormatInt(id, 10)
	}
	b.WriteString(`<form class="cms" method="post" enctype="multipart/form-data" action="` + a.base + `/conteudo/editar?` + esc(q) + `">` + a.tok())
	b.WriteString(refill(c.HTML, old))
	hasText := false
	for _, f := range c.Fields {
		if f.Type == "textarea" {
			hasText = true
		}
		if f.Type != "file" || cur == nil {
			continue
		}
		if v, ok := cur.Get(f.Name); ok {
			if ft, ok := v.(*lang.Table); ok {
				u, _ := ft.Get("url")
				nm, _ := ft.Get("nome")
				tp, _ := ft.Get("tipo")
				b.WriteString(`<div class="card"><div class="muted">Arquivo atual em “` + esc(f.Label) + `” (envie outro para trocar)</div>`)
				if strings.HasPrefix(lang.Str(tp), "image/") {
					b.WriteString(`<img src="` + esc(lang.Str(u)) + `" alt="">`)
				}
				b.WriteString(`<a href="` + esc(lang.Str(u)) + `" target="_blank">` + esc(lang.Str(nm)) + `</a>`)
				if !f.Required {
					b.WriteString(`<label class="check"><input type="checkbox" name="_remover_` + esc(f.Name) + `"> Remover este arquivo</label>`)
				}
				b.WriteString(`</div>`)
			}
		}
	}
	builder := false
	for _, f := range c.Fields {
		if f.Builder {
			builder = true
		}
	}
	if hasText && !builder {
		b.WriteString(mdHint)
	}
	chk := ""
	if pub {
		chk = " checked"
	}
	b.WriteString(`<label>Endereço no site <input name="_slug" value="` + esc(slug) + `" placeholder="criado a partir do título" maxlength="80"></label>` +
		`<label class="check"><input type="checkbox" name="_publicado"` + chk + `> Publicado (desmarque para deixar como rascunho)</label>` +
		`<label>Publicar em (opcional: agenda para esta data e hora) <input type="datetime-local" name="_publicar_em" value="` + esc(sched) + `"></label>` +
		`<p class="row"><button>Salvar</button><a href="` + a.base + `/conteudo?` + esc("c="+url.QueryEscape(c.Name)) + `">Cancelar</a>`)
	if cur != nil {
		if u := a.site().itemURL(c.Name, cur); u != "" {
			b.WriteString(`<a href="` + esc(a.s.url(u)) + `" target="_blank">Ver no site</a>`)
		}
	}
	b.WriteString(`</p></form>`)
	if cur != nil {
		b.WriteString(a.versionsHTML(c, id))
	}
	if builder {
		b.WriteString(a.builderAssets())
	}
	title := "Novo " + c.Item
	if cur != nil {
		title = "Editar " + c.Item
	}
	return a.out(title, b.String(), true)
}

// ---------- textos do site ----------

var brRe = regexp.MustCompile(`(?i)<br\s*/?>|</p>|</li>|</h[1-6]>`)

// plainDefault tira o HTML do texto padrão para mostrar na caixa de edição.
func plainDefault(e *Editable) string {
	if e.Kind == "imagem" {
		return e.Default
	}
	t := brRe.ReplaceAllString(e.Default, "\n")
	t = stripTags(t)
	lines := strings.Split(t, "\n")
	var out []string
	for _, l := range lines {
		l = strings.Join(strings.Fields(l), " ")
		if l != "" || (len(out) > 0 && out[len(out)-1] != "") {
			out = append(out, l)
		}
	}
	sep := "\n"
	if e.Kind == "rico" {
		sep = "\n\n"
		var keep []string
		for _, l := range out {
			if l != "" {
				keep = append(keep, l)
			}
		}
		out = keep
	}
	return strings.TrimSpace(htmlUnescape(strings.Join(out, sep)))
}

func htmlUnescape(s string) string {
	r := strings.NewReplacer("&amp;", "&", "&lt;", "<", "&gt;", ">", "&quot;", `"`, "&#39;", "'", "&nbsp;", " ")
	return r.Replace(s)
}

var imgField = &Field{Name: "imagem", Type: "file", Label: "Imagem", Accept: "image/*", MaxMB: 8, Public: true}

// texts é /_admin/textos.
func (a *adminPage) texts(post bool) error {
	site := a.site()
	if site == nil {
		a.s.simple(a.rq, 500, "Erro no site", "<p>O arquivo do site tem um erro. Rode nep check.</p>")
		return nil
	}
	r := a.rq.r
	ajax := r.PostForm.Get("ajax") != ""
	msg := ""
	if post {
		key := r.PostForm.Get("chave")
		e := site.Editable(key)
		if e == nil {
			a.s.simple(a.rq, 400, "Texto não encontrado", "")
			return nil
		}
		prev, _ := a.s.edGet(key)
		var err error
		switch {
		case r.PostForm.Get("restaurar") != "":
			err = a.s.edReset(key)
		case e.Kind == "imagem":
			files, errs := checkFiles([]*Field{imgField}, r)
			if len(errs) > 0 {
				msg = errs[0]
				break
			}
			if len(files) == 0 {
				msg = "Escolha uma imagem."
				break
			}
			t := lang.NewTable()
			var saved []string
			saved, err = a.s.saveFiles(files, t)
			if err == nil {
				img, _ := t.Get("imagem")
				u, _ := img.(*lang.Table).Get("url")
				if err = a.s.edSet(key, lang.Str(u)); err != nil {
					a.s.Store.removeFiles(saved)
				}
			}
		case e.Kind == "visual":
			v := strings.ReplaceAll(r.PostForm.Get("valor"), "\x00", "")
			if !utf8.ValidString(v) || len(v) > 300000 {
				msg = "O bloco ficou grande demais."
				break
			}
			err = a.s.edSet(key, v)
		case e.Kind == "menu":
			items, bad := parseMenu(strings.ReplaceAll(r.PostForm.Get("valor"), "\r\n", "\n"))
			if bad != "" {
				msg = bad
				break
			}
			err = a.s.edSet(key, formatMenu(items))
		default:
			v := strings.ReplaceAll(strings.ReplaceAll(r.PostForm.Get("valor"), "\r\n", "\n"), "\x00", "")
			max := 2000
			if e.Kind == "rico" {
				max = 20000
			}
			if !utf8.ValidString(v) || utf8.RuneCountInString(v) > max {
				msg = fmt.Sprintf("O texto pode ter no máximo %d caracteres.", max)
				break
			}
			if e.Kind == "texto" {
				v = strings.TrimSpace(v)
			}
			err = a.s.edSet(key, v)
		}
		if err != nil {
			return err
		}
		if msg == "" {
			if now, _ := a.s.edGet(key); now != prev && fileRefRe.MatchString(prev) {
				a.s.Store.removeFiles(filesIn(prev)) // imagem trocada: a antiga sai
			}
		}
		if ajax {
			w := a.rq.w
			w.Header().Set("Content-Type", "application/json; charset=utf-8")
			if msg != "" {
				w.WriteHeader(http.StatusUnprocessableEntity)
				json.NewEncoder(w).Encode(map[string]string{"erro": msg})
				return nil
			}
			v, ok := a.s.edGet(key)
			out := map[string]string{}
			if ok && e.Kind != "menu" && e.Kind != "visual" {
				out["html"] = string(edHTML(e.Kind, v))
			}
			json.NewEncoder(w).Encode(out)
			return nil
		}
		if msg == "" {
			a.s.redirect(a.rq, a.base+"/textos?ok=1#ed-"+url.QueryEscape(key), http.StatusSeeOther)
			return nil
		}
	}
	var b strings.Builder
	b.WriteString(cmsCSS)
	if msg != "" {
		b.WriteString(errP(msg))
	} else if r.URL.Query().Get("ok") != "" {
		b.WriteString(`<p class="ok">Salvo. Já aparece no site.</p>`)
	}
	b.WriteString(`<p class="muted">Estes são os trechos que o programador deixou editáveis. Você também pode editar os textos direto no site: com o painel aberto, use o botão “Editar textos” no canto da página.</p>`)
	seen := map[string]bool{}
	builder := false
	for _, e := range site.Editables {
		if seen[e.Key] {
			continue
		}
		seen[e.Key] = true
		v, changed := a.s.edGet(e.Key)
		if !changed {
			v = plainDefault(e)
			if e.Kind == "menu" {
				items, _ := menuDefaults(e)
				v = formatMenu(items)
			}
			if e.Kind == "visual" {
				v = e.Default
			}
		}
		kind := map[string]string{"texto": "texto curto", "rico": "texto longo", "imagem": "imagem", "menu": "menu", "visual": "construtor visual"}[e.Kind]
		b.WriteString(`<div class="card" id="ed-` + esc(e.Key) + `"><h2>` + esc(e.Key) + ` <span class="muted">(` + kind + `)</span>`)
		if changed {
			b.WriteString(` <span class="tag">alterado</span>`)
		}
		b.WriteString(`</h2>`)
		b.WriteString(`<form class="cms" method="post" action="` + a.base + `/textos"`)
		if e.Kind == "imagem" {
			b.WriteString(` enctype="multipart/form-data"`)
		}
		b.WriteString(`>` + a.tok() + `<input type="hidden" name="chave" value="` + esc(e.Key) + `">`)
		switch e.Kind {
		case "imagem":
			b.WriteString(`<img src="` + esc(a.s.url(v)) + `" alt=""><label>Nova imagem <input type="file" name="imagem" accept="image/*" required></label>`)
		case "rico":
			b.WriteString(`<textarea name="valor" data-editor rows="8">` + esc(v) + `</textarea>` + mdHint)
		case "visual":
			builder = true
			b.WriteString(`<textarea name="valor" data-construtor>` + esc(v) + `</textarea>`)
		case "menu":
			b.WriteString(`<textarea name="valor" data-editor rows="6" placeholder="Início | /">` + esc(v) + `</textarea>` +
				`<p class="muted">Um item por linha, no formato <code>Texto | /endereço</code>. A ordem das linhas é a ordem do menu. Links de fora usam <code>https://</code>.</p>`)
			if !changed && dynMenu(e) {
				b.WriteString(`<p class="muted">Hoje este menu também mostra links automáticos (criados pelo código, como as páginas publicadas). Ao salvar aqui, vale só a sua lista: inclua os que quiser manter, copiando da lista de endereços abaixo.</p>`)
			}
			b.WriteString(a.addresses())
		default:
			rows := 2
			if strings.Contains(v, "\n") || utf8.RuneCountInString(v) > 120 {
				rows = 4
			}
			b.WriteString(`<textarea name="valor" rows="` + strconv.Itoa(rows) + `">` + esc(v) + `</textarea>`)
		}
		b.WriteString(`<p class="row"><button>Salvar</button>`)
		if changed {
			b.WriteString(`<button class="ghost" name="restaurar" value="1" formnovalidate>Voltar ao texto original</button>`)
		}
		b.WriteString(`</p></form></div>`)
	}
	if builder {
		b.WriteString(a.builderAssets())
	}
	if len(seen) == 0 {
		b.WriteString(`<p class="muted">O site ainda não tem trechos editáveis. O programador marca com editavel="nome" na tag.</p>`)
	}
	return a.out("Textos e menus", b.String(), true)
}

// addresses lista os endereços do site que dá para pôr num menu.
func (a *adminPage) addresses() string {
	site := a.site()
	var l []string
	for _, p := range site.Pages {
		if p.Route == "404" || p.Auth || len(p.params) > 0 {
			continue
		}
		t := p.Title
		if t == "" {
			t = p.Route
		}
		l = append(l, `<code>`+esc(t+" | "+"/"+strings.Trim(p.Route, "/"))+`</code>`)
	}
	for _, c := range site.Contents {
		n := 0
		for _, r := range a.s.Store.All(c.Name, 0) {
			t := r.ToValue()
			if u := site.itemURL(c.Name, t); u != "" && published(t) && n < 15 {
				l = append(l, `<code>`+esc(itemTitle(c, t)+" | "+u)+`</code>`)
				n++
			}
		}
	}
	if len(l) == 0 {
		return ""
	}
	return `<details><summary class="muted">Endereços do site (copie e cole)</summary><p>` + strings.Join(l, "<br>") + `</p></details>`
}

func dynMenu(e *Editable) bool {
	for _, re := range dynBlocks {
		if re.MatchString(e.Default) {
			return true
		}
	}
	return false
}

// builderAssets carrega o construtor visual (GrapesJS) na tela do painel.
func (a *adminPage) builderAssets() string {
	p := esc(a.s.opt.Base) + "/_nep/construtor/"
	return `<style>.nep-box{max-width:1320px}.nep-gjs{border:1px solid #c9cfc5;border-radius:8px;overflow:hidden;margin:6px 0 12px}</style>` +
		`<link rel="stylesheet" href="` + p + `grapes.min.css">` +
		`<script src="` + p + `grapes.min.js"></script><script src="` + p + `blocks-basic.js"></script><script src="` + p + `pt.js"></script>` +
		`<script src="` + p + `nep.js" data-base="` + esc(a.s.opt.Base) + `" data-csrf="` + esc(a.rq.sess.CSRF) + `" data-midia="` + esc(mediaJSON(a.s.mediaURLs())) + `"></script>` +
		`<p class="muted">Construtor visual: arraste os blocos da direita para a página, clique para editar o texto e use a aba de estilos para cores e espaços. Ele é de código aberto (GrapesJS, licença BSD).</p>`
}
