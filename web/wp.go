package web

// O que faltava para um site .nep.html cuidar de si como um WordPress:
// histórico de versões, lixeira, publicação agendada, busca, feed RSS,
// comentários com moderação, biblioteca de mídia e cópia de segurança.
// Tudo no mesmo banco (coleções internas começam com "_" e o código do site
// não alcança) e no mesmo painel.

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"html"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/Nanofylium/probable-plant/lang"
)

const (
	colVersions = "_versoes"
	colTrash    = "_lixeira"
	colComments = "_comentarios"
	colMedia    = "_midia"
	maxVersions = 20
	trashDays   = 30
)

// ---------- banco: apagar sem levar os arquivos e devolver com o mesmo número ----------

// DeleteKeep apaga o registro, mas deixa os arquivos (eles vão para a lixeira junto).
func (s *Store) DeleteKeep(col string, id int64) (bool, error) {
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
	return true, s.write(&logLine{Op: "del", Col: col, ID: id})
}

// Put grava um registro com um número escolhido (só se o número estiver livre).
func (s *Store) Put(col string, id int64, created string, data *lang.Table) error {
	j, err := dataJSON(data)
	if err != nil {
		return err
	}
	unlock, err := s.lock()
	if err != nil {
		return err
	}
	defer unlock()
	_ = s.reload(false)
	if _, used := s.recs[id]; used {
		return fmt.Errorf("o número %d já está em uso", id)
	}
	if created == "" {
		created = stamp()
	}
	return s.write(&logLine{Op: "put", Col: col, ID: id, C: created, U: stamp(), D: j})
}

func recTable(data string) *lang.Table {
	v, err := lang.FromJSON(data)
	if err != nil {
		return lang.NewTable()
	}
	if t, ok := v.(*lang.Table); ok {
		return t
	}
	return lang.NewTable()
}

func getS(t *lang.Table, k string) string {
	if v, ok := t.Get(k); ok && v != nil {
		return lang.Str(v)
	}
	return ""
}

func getI(t *lang.Table, k string) int64 {
	if v, ok := t.Get(k); ok {
		if n, ok := toInt(v); ok {
			return n
		}
	}
	return 0
}

// who diz quem está no painel, para o histórico.
func (a *adminPage) who() string {
	if a.rq.sess.U == "" {
		return "dono"
	}
	if u, ok := a.s.users()[a.rq.sess.U]; ok && u.Nome != "" {
		return u.Nome
	}
	return a.rq.sess.U
}

// ---------- publicação agendada ----------

const schedField = "publicar_em"

var schedLayouts = []string{"2006-01-02T15:04", "2006-01-02 15:04", "2006-01-02T15:04:05", "2006-01-02 15:04:05"}

func parseSched(v string) (time.Time, bool) {
	for _, l := range schedLayouts {
		if t, err := time.ParseInLocation(l, strings.TrimSpace(v), time.Local); err == nil {
			return t, true
		}
	}
	return time.Time{}, false
}

// scheduledFuture diz se o item está agendado para o futuro (e quando).
func scheduledFuture(t *lang.Table) (time.Time, bool) {
	if when, ok := parseSched(getS(t, schedField)); ok && time.Now().Before(when) {
		return when, true
	}
	return time.Time{}, false
}

// ---------- histórico de versões ----------

// saveVersion guarda como o item estava antes de ser alterado.
func (s *Server) saveVersion(col string, rec *Record, who string) {
	when := rec.Updated
	if when == "" {
		when = rec.Created
	}
	t := lang.NewTable()
	t.SetStr("col", col)
	t.SetStr("item", rec.ID)
	t.SetStr("dados", rec.Data)
	t.SetStr("quando", when)
	t.SetStr("por", who)
	if _, err := s.Store.Insert(colVersions, t); err != nil {
		return
	}
	vs := s.versions(col, rec.ID)
	for _, v := range vs[min(len(vs), maxVersions):] {
		_, _ = s.Store.DeleteKeep(colVersions, v.ID)
	}
}

// versions devolve as versões antigas de um item, a mais nova primeiro.
func (s *Server) versions(col string, id int64) []*Record {
	var out []*Record
	for _, r := range s.Store.All(colVersions, 0) {
		t := r.ToValue()
		if getS(t, "col") == col && getI(t, "item") == id {
			out = append(out, r)
		}
	}
	return out
}

func (a *adminPage) versionsHTML(c *Content, id int64) string {
	vs := a.s.versions(c.Name, id)
	if len(vs) == 0 {
		return ""
	}
	var b strings.Builder
	b.WriteString(`<details class="card"><summary>Histórico: ` + map[bool]string{true: "1 versão anterior", false: strconv.Itoa(len(vs)) + " versões anteriores"}[len(vs) == 1] + `</summary><div class="scroll"><table><tr><th>Salva em</th><th>Por</th><th>Título</th><th></th></tr>`)
	for _, v := range vs {
		t := v.ToValue()
		old := recTable(getS(t, "dados"))
		title := itemTitle(c, old)
		b.WriteString(`<tr><td>` + esc(fmtWhen(getS(t, "quando"))) + `</td><td>` + esc(getS(t, "por")) + `</td><td>` + esc(title) + `</td><td>` +
			`<form method="post" action="` + a.base + `/conteudo/versao" onsubmit="return confirm('Voltar o item para esta versão? A versão atual fica guardada no histórico.')">` + a.tok() +
			`<input type="hidden" name="c" value="` + esc(c.Name) + `"><input type="hidden" name="id" value="` + strconv.FormatInt(id, 10) + `"><input type="hidden" name="v" value="` + strconv.FormatInt(v.ID, 10) + `"><button class="ghost">Restaurar</button></form></td></tr>`)
	}
	b.WriteString(`</table></div><p class="muted">Arquivos (imagens e anexos) não voltam com a versão: ficam os atuais.</p></details>`)
	return b.String()
}

// restoreVersion é POST /_admin/conteudo/versao.
func (a *adminPage) restoreVersion() error {
	r := a.rq.r
	c := a.site().Content(r.PostForm.Get("c"))
	id, _ := strconv.ParseInt(r.PostForm.Get("id"), 10, 64)
	vid, _ := strconv.ParseInt(r.PostForm.Get("v"), 10, 64)
	if c == nil || r.Method != http.MethodPost {
		a.s.simple(a.rq, 400, "Pedido inválido", "")
		return nil
	}
	cur := a.s.Store.Find(c.Name, id)
	v := a.s.Store.Find(colVersions, vid)
	if cur == nil || v == nil || getI(v.ToValue(), "item") != id || getS(v.ToValue(), "col") != c.Name {
		a.s.simple(a.rq, 404, "Versão não encontrada", "")
		return nil
	}
	old := recTable(getS(v.ToValue(), "dados"))
	data := lang.NewTable()
	for _, f := range c.Fields {
		if f.Type == "file" {
			continue // arquivos ficam os atuais
		}
		val, _ := old.Get(f.Name)
		data.SetStr(f.Name, val)
	}
	for _, k := range []string{"slug", "publicado", schedField} {
		val, _ := old.Get(k)
		data.SetStr(k, val)
	}
	if s := getS(old, "slug"); s != "" {
		data.SetStr("slug", a.s.uniqueSlug(c.Name, s, id))
	}
	a.s.saveVersion(c.Name, cur, a.who())
	if _, err := a.s.Store.Update(c.Name, id, data); err != nil {
		return err
	}
	a.s.redirect(a.rq, a.base+"/conteudo/editar?c="+url.QueryEscape(c.Name)+"&id="+strconv.FormatInt(id, 10)+"&restaurado=1", http.StatusSeeOther)
	return nil
}

// ---------- lixeira ----------

// trash manda um item para a lixeira (com os arquivos, para poder voltar).
func (s *Server) trash(col string, id int64, who string) error {
	rec := s.Store.Find(col, id)
	if rec == nil {
		return nil
	}
	t := lang.NewTable()
	t.SetStr("col", col)
	t.SetStr("item", id)
	t.SetStr("dados", rec.Data)
	t.SetStr("criado", rec.Created)
	t.SetStr("apagado", stamp())
	t.SetStr("por", who)
	if _, err := s.Store.Insert(colTrash, t); err != nil {
		return err
	}
	_, err := s.Store.DeleteKeep(col, id)
	return err
}

// purgeTrash apaga de vez o que está na lixeira há mais de 30 dias.
func (s *Server) purgeTrash() {
	limit := time.Now().AddDate(0, 0, -trashDays)
	for _, r := range s.Store.All(colTrash, 0) {
		if when, err := time.ParseInLocation("2006-01-02 15:04:05", getS(r.ToValue(), "apagado"), time.Local); err == nil && when.Before(limit) {
			s.dropTrash(r)
		}
	}
}

func (s *Server) dropTrash(r *Record) {
	t := r.ToValue()
	files := filesIn(getS(t, "dados"))
	_, _ = s.Store.DeleteKeep(colTrash, r.ID)
	// só apaga o arquivo se nenhum item vivo ainda o usa
	inUse := map[string]bool{}
	for _, f := range files {
		for _, x := range s.Store.Search(f) {
			if !strings.HasPrefix(x.Col, "_") || x.Col == colMedia {
				inUse[f] = true
			}
		}
	}
	var gone []string
	for _, f := range files {
		if !inUse[f] {
			gone = append(gone, f)
		}
	}
	s.Store.removeFiles(gone)
}

// trashPage é /_admin/lixeira.
func (a *adminPage) trashPage(post bool) error {
	r := a.rq.r
	a.s.purgeTrash()
	site := a.site()
	if post {
		id, _ := strconv.ParseInt(r.PostForm.Get("id"), 10, 64)
		msg := ""
		switch r.PostForm.Get("acao") {
		case "restaurar":
			if rec := a.s.Store.Find(colTrash, id); rec != nil {
				t := rec.ToValue()
				col, item := getS(t, "col"), getI(t, "item")
				data := recTable(getS(t, "dados"))
				if site != nil && site.Content(col) != nil {
					data.SetStr("slug", a.s.uniqueSlug(col, getS(data, "slug"), item))
				}
				if err := a.s.Store.Put(col, item, getS(t, "criado"), data); err != nil {
					if _, err2 := a.s.Store.Insert(col, data); err2 != nil {
						return err2
					}
				}
				_, _ = a.s.Store.DeleteKeep(colTrash, id)
				msg = "Item restaurado."
			}
		case "apagar":
			if rec := a.s.Store.Find(colTrash, id); rec != nil {
				a.s.dropTrash(rec)
				msg = "Apagado de vez."
			}
		case "esvaziar":
			for _, rec := range a.s.Store.All(colTrash, 0) {
				a.s.dropTrash(rec)
			}
			msg = "Lixeira esvaziada."
		}
		a.s.redirect(a.rq, a.base+"/lixeira?ok="+urlQ(msg), http.StatusSeeOther)
		return nil
	}
	var b strings.Builder
	b.WriteString(cmsCSS)
	if ok := r.URL.Query().Get("ok"); ok != "" {
		b.WriteString(`<p class="ok">` + esc(ok) + `</p>`)
	}
	b.WriteString(`<p class="muted">O que é apagado no painel fica aqui por ` + strconv.Itoa(trashDays) + ` dias, com imagens e anexos. Depois sai sozinho.</p>`)
	recs := a.s.Store.All(colTrash, 0)
	if len(recs) == 0 {
		b.WriteString(`<p class="muted">A lixeira está vazia.</p>`)
		return a.out("Lixeira", b.String(), true)
	}
	b.WriteString(`<div class="scroll"><table><tr><th>Item</th><th>Tipo</th><th>Apagado em</th><th>Por</th><th></th></tr>`)
	for _, rec := range recs {
		t := rec.ToValue()
		col := getS(t, "col")
		data := recTable(getS(t, "dados"))
		title, kind := "", col
		if site != nil {
			if c := site.Content(col); c != nil {
				title, kind = itemTitle(c, data), c.Title
			}
		}
		if title == "" {
			title = "(sem título)"
		}
		id := strconv.FormatInt(rec.ID, 10)
		b.WriteString(`<tr><td>` + esc(title) + `</td><td>` + esc(kind) + `</td><td>` + esc(fmtWhen(getS(t, "apagado"))) + `</td><td>` + esc(getS(t, "por")) + `</td><td class="row">` +
			`<form method="post">` + a.tok() + `<input type="hidden" name="acao" value="restaurar"><input type="hidden" name="id" value="` + id + `"><button>Restaurar</button></form>` +
			`<form method="post" onsubmit="return confirm('Apagar de vez? Não dá para desfazer.')">` + a.tok() + `<input type="hidden" name="acao" value="apagar"><input type="hidden" name="id" value="` + id + `"><button class="ghost">Apagar de vez</button></form></td></tr>`)
	}
	b.WriteString(`</table></div><form method="post" onsubmit="return confirm('Esvaziar a lixeira? Não dá para desfazer.')">` + a.tok() + `<input type="hidden" name="acao" value="esvaziar"><button class="ghost">Esvaziar lixeira</button></form>`)
	return a.out("Lixeira", b.String(), true)
}

// ---------- busca ----------

// fold deixa o texto sem acento e minúsculo, para a busca achar "pao" em "Pão".
func fold(s string) string {
	var b strings.Builder
	for _, r := range strings.ToLower(s) {
		if a, ok := accents[r]; ok {
			r = a
		}
		b.WriteRune(r)
	}
	return b.String()
}

// search procura nos tipos de conteúdo publicados que têm página no site.
func (s *Server) search(site *Site, term string, only []string, limit int) *lang.List {
	out := lang.NewList()
	words := strings.Fields(fold(term))
	if len(words) == 0 {
		return out
	}
	want := map[string]bool{}
	for _, o := range only {
		want[o] = true
	}
	type hit struct {
		t     *lang.Table
		score int
		when  string
	}
	var hits []hit
	for _, c := range site.Contents {
		if len(want) > 0 && !want[c.Name] {
			continue
		}
		for _, r := range s.Store.All(c.Name, 0) {
			t := r.ToValue()
			if !published(t) {
				continue
			}
			title := itemTitle(c, t)
			var hay strings.Builder
			for _, f := range c.Fields {
				if f.Type == "file" {
					continue
				}
				hay.WriteString(getS(t, f.Name) + " ")
			}
			text, ft := fold(hay.String()), fold(title)
			score := 0
			for _, w := range words {
				n := strings.Count(text, w)
				if n == 0 {
					score = 0
					break
				}
				score += n
				if strings.Contains(ft, w) {
					score += 10
				}
			}
			if score == 0 {
				continue
			}
			res := lang.NewTable()
			res.SetStr("titulo", title)
			res.SetStr("tipo", c.Name)
			res.SetStr("resumo", itemDescription(c, t))
			if u := site.itemURL(c.Name, t); u != "" {
				res.SetStr("url", s.url(u))
			}
			res.SetStr("imagem", itemImage(c, t))
			res.SetStr("item", t)
			hits = append(hits, hit{res, score, r.Created})
		}
	}
	sort.SliceStable(hits, func(i, j int) bool {
		if hits[i].score != hits[j].score {
			return hits[i].score > hits[j].score
		}
		return hits[i].when > hits[j].when
	})
	for i, h := range hits {
		if i >= limit {
			break
		}
		out.Items = append(out.Items, h.t)
	}
	return out
}

// ---------- feed RSS (/feed.xml) ----------

func (s *Server) feed(rq *request) {
	site := rq.site
	base := s.siteURL(rq)
	type item struct {
		title, link, desc, when string
	}
	var items []item
	for _, c := range site.Contents {
		for _, r := range s.Store.All(c.Name, 0) {
			t := r.ToValue()
			u := site.itemURL(c.Name, t)
			if u == "" || !published(t) {
				continue
			}
			when := r.Created
			if w, ok := parseSched(getS(t, schedField)); ok {
				when = w.Format("2006-01-02 15:04:05")
			}
			items = append(items, item{itemTitle(c, t), base + u, itemDescription(c, t), when})
		}
	}
	sort.SliceStable(items, func(i, j int) bool { return items[i].when > items[j].when })
	if len(items) > 20 {
		items = items[:20]
	}
	name := site.Config["nome"]
	if name == "" {
		name = site.titleName()
	}
	if name == "" {
		name = rq.r.Host
	}
	var b bytes.Buffer
	b.WriteString(xml.Header + `<rss version="2.0" xmlns:atom="http://www.w3.org/2005/Atom"><channel>`)
	x := func(tag, v string) {
		b.WriteString("<" + tag + ">")
		_ = xml.EscapeText(&b, []byte(v))
		b.WriteString("</" + tag + ">")
	}
	x("title", name)
	x("link", base+"/")
	x("description", name)
	if l := site.Config["lang"]; l != "" {
		x("language", l)
	}
	b.WriteString(`<atom:link href="` + html.EscapeString(base+"/feed.xml") + `" rel="self" type="application/rss+xml"/>`)
	for _, it := range items {
		b.WriteString("<item>")
		x("title", it.title)
		x("link", it.link)
		x("guid", it.link)
		x("description", it.desc)
		if tm, err := time.ParseInLocation("2006-01-02 15:04:05", it.when, time.Local); err == nil {
			x("pubDate", tm.Format(time.RFC1123Z))
		}
		b.WriteString("</item>")
	}
	b.WriteString("</channel></rss>")
	rq.w.Header().Set("Content-Type", "application/rss+xml; charset=utf-8")
	rq.w.Header().Set("Cache-Control", "public, max-age=300")
	_, _ = rq.w.Write(b.Bytes())
}

// hasFeed diz se o site tem algum tipo de conteúdo com página própria.
func (site *Site) hasFeed() bool {
	for _, p := range site.Pages {
		if p.Conteudo != "" && len(p.params) > 0 {
			return true
		}
	}
	return false
}

// ---------- comentários ----------

const commentFlash = -7

// commentsHTML é o que comentarios() desenha: os aprovados e o formulário.
func (s *Server) commentsHTML(rq *request) (lang.Value, error) {
	if rq.item == nil {
		return nil, fmt.Errorf("comentarios() só funciona numa <page conteudo=\"...\">")
	}
	id := getI(rq.item, "id")
	var list []*lang.Table
	all := s.Store.All(colComments, 0)
	for i := len(all) - 1; i >= 0; i-- { // mais antigos primeiro
		t := all[i].ToValue()
		if getS(t, "col") == rq.itemCol && getI(t, "item") == id && getS(t, "status") == "aprovado" {
			list = append(list, t)
		}
	}
	var b strings.Builder
	n := len(list)
	b.WriteString(`<section class="nep-comentarios" id="comentarios"><h2>` + map[bool]string{true: "1 comentário", false: strconv.Itoa(n) + " comentários"}[n == 1] + `</h2>`)
	for _, t := range list {
		b.WriteString(`<article class="nep-comentario"><p class="nep-comentario-autor"><strong>` + html.EscapeString(getS(t, "nome")) + `</strong> <time>` + html.EscapeString(fmtWhen(getS(t, "created_at"))) + `</time></p>` +
			`<p>` + strings.ReplaceAll(html.EscapeString(getS(t, "texto")), "\n", "<br>") + `</p></article>`)
	}
	b.WriteString(`<form class="nep-comentar" method="post" action="` + html.EscapeString(s.url("/_nep/comentar")) + `">`)
	if m := rq.sess.takeFlash(commentFlash, 0); m != "" {
		b.WriteString(`<div class="nep-ok" role="status">` + html.EscapeString(m) + `</div>`)
	}
	b.WriteString(`<input type="hidden" name="_nep_csrf" value="` + rq.sess.CSRF + `">` +
		`<input type="hidden" name="col" value="` + html.EscapeString(rq.itemCol) + `"><input type="hidden" name="item" value="` + strconv.FormatInt(id, 10) + `">` +
		`<input type="hidden" name="volta" value="` + html.EscapeString(rq.path) + `">` + honeypot +
		`<h3>Deixe um comentário</h3>` +
		`<label>Nome <input name="nome" required maxlength="60"></label>` +
		`<label>E-mail (não aparece no site) <input name="email" type="email" maxlength="120"></label>` +
		`<label>Comentário <textarea name="texto" required maxlength="3000" rows="4"></textarea></label>` +
		`<button>Enviar comentário</button><p class="nep-comentar-nota">Os comentários aparecem depois de aprovados.</p></form></section>`)
	return lang.HTML(b.String()), nil
}

// comment é POST /_nep/comentar.
func (s *Server) comment(rq *request) {
	r := rq.r
	r.Body = http.MaxBytesReader(rq.w, r.Body, 64<<10)
	if r.Method != http.MethodPost || r.ParseForm() != nil {
		s.simple(rq, 400, "Pedido inválido", "")
		return
	}
	back := r.PostForm.Get("volta")
	if !localPath(back) {
		back = "/"
	}
	backURL := s.url(back) + "#comentarios"
	say := func(msg string) {
		rq.sess.setFlash(commentFlash, 0, msg)
		s.redirect(rq, backURL, http.StatusSeeOther)
	}
	if !rq.sess.csrfOK(r.PostForm.Get("_nep_csrf")) {
		say("Sua sessão expirou. Escreva de novo, por favor.")
		return
	}
	if r.PostForm.Get("_nep_hp") != "" {
		say("Obrigado! Seu comentário aparece depois de aprovado.")
		return
	}
	col := r.PostForm.Get("col")
	id, _ := strconv.ParseInt(r.PostForm.Get("item"), 10, 64)
	c := rq.site.Content(col)
	if c == nil {
		say("Não deu para comentar aqui.")
		return
	}
	it := s.Store.Find(col, id)
	if it == nil || !published(it.ToValue()) {
		say("Não deu para comentar aqui.")
		return
	}
	nome := strings.TrimSpace(r.PostForm.Get("nome"))
	texto := strings.TrimSpace(strings.ReplaceAll(r.PostForm.Get("texto"), "\r\n", "\n"))
	email := strings.TrimSpace(r.PostForm.Get("email"))
	switch {
	case nome == "" || texto == "":
		say("Preencha o nome e o comentário.")
		return
	case utf8.RuneCountInString(nome) > 60 || utf8.RuneCountInString(texto) > 3000:
		say("O comentário ficou grande demais.")
		return
	case email != "" && !emailRe.MatchString(email):
		say("Confira o e-mail.")
		return
	}
	rk := "comentar:" + s.ipKey(r)
	if ok, _ := s.Store.Rate(rk, 5, 600, true); !ok {
		say("Muitos comentários em pouco tempo. Tente de novo daqui a alguns minutos.")
		return
	}
	t := lang.NewTable()
	t.SetStr("col", col)
	t.SetStr("item", id)
	t.SetStr("titulo", itemTitle(c, it.ToValue()))
	t.SetStr("nome", nome)
	t.SetStr("email", email)
	t.SetStr("texto", texto)
	t.SetStr("pagina", back)
	t.SetStr("status", "pendente")
	if _, err := s.Store.Insert(colComments, t); err != nil {
		s.fail(rq, err)
		return
	}
	if to := splitAddrs(s.emailConf().Avisos); len(to) > 0 {
		if ok, _ := s.Store.Rate("email:comentario", 100, 3600, true); ok {
			link := s.siteURL(rq) + "/_admin/comentarios"
			_, _ = s.sendMail(mailMsg{To: to, Subject: "Novo comentário para aprovar: " + itemTitle(c, it.ToValue()),
				Text: nome + " comentou:\n\n" + texto + "\n\nAprovar ou apagar: " + link, ReplyTo: func() string {
					if a, err := cleanAddr(email); err == nil {
						return a
					}
					return ""
				}()}, r.Host)
		}
	}
	say("Obrigado! Seu comentário aparece depois de aprovado.")
}

// commentsPage é /_admin/comentarios (dono e editores moderam).
func (a *adminPage) commentsPage(post bool) error {
	r := a.rq.r
	if post {
		id, _ := strconv.ParseInt(r.PostForm.Get("id"), 10, 64)
		switch r.PostForm.Get("acao") {
		case "aprovar", "esconder":
			st := map[string]string{"aprovar": "aprovado", "esconder": "pendente"}[r.PostForm.Get("acao")]
			t := lang.NewTable()
			t.SetStr("status", st)
			if _, err := a.s.Store.Update(colComments, id, t); err != nil {
				return err
			}
		case "apagar":
			if _, err := a.s.Store.DeleteKeep(colComments, id); err != nil {
				return err
			}
		}
		a.s.redirect(a.rq, a.base+"/comentarios", http.StatusSeeOther)
		return nil
	}
	all := a.s.Store.All(colComments, 0)
	sort.SliceStable(all, func(i, j int) bool {
		pi := getS(all[i].ToValue(), "status") == "pendente"
		pj := getS(all[j].ToValue(), "status") == "pendente"
		return pi && !pj
	})
	var b strings.Builder
	b.WriteString(cmsCSS)
	pend := 0
	for _, rec := range all {
		if getS(rec.ToValue(), "status") == "pendente" {
			pend++
		}
	}
	b.WriteString(`<p class="muted">` + strconv.Itoa(pend) + ` esperando aprovação. Para receber um aviso por e-mail a cada comentário, preencha "Avisos" em E-mail.</p>`)
	if len(all) == 0 {
		b.WriteString(`<p class="muted">Nenhum comentário ainda. Para abrir comentários num tipo de conteúdo, o tema usa {{ comentarios() }} na página do item.</p>`)
		return a.out("Comentários", b.String(), true)
	}
	b.WriteString(`<div class="scroll"><table><tr><th>Comentário</th><th>Em</th><th>Situação</th><th></th></tr>`)
	for _, rec := range all {
		t := rec.ToValue()
		id := strconv.FormatInt(rec.ID, 10)
		st := getS(t, "status")
		tag := `<span class="tag">Aprovado</span>`
		act := `<form method="post">` + a.tok() + `<input type="hidden" name="acao" value="esconder"><input type="hidden" name="id" value="` + id + `"><button class="ghost">Esconder</button></form>`
		if st != "aprovado" {
			tag = `<span class="tag off">Esperando</span>`
			act = `<form method="post">` + a.tok() + `<input type="hidden" name="acao" value="aprovar"><input type="hidden" name="id" value="` + id + `"><button>Aprovar</button></form>`
		}
		who := esc(getS(t, "nome"))
		if e := getS(t, "email"); e != "" {
			who += ` <span class="muted">` + esc(e) + `</span>`
		}
		b.WriteString(`<tr><td><strong>` + who + `</strong><div>` + strings.ReplaceAll(esc(getS(t, "texto")), "\n", "<br>") + `</div><div class="muted">` + esc(fmtWhen(rec.Created)) + `</div></td>` +
			`<td><a href="` + esc(a.s.url(getS(t, "pagina"))) + `" target="_blank">` + esc(getS(t, "titulo")) + `</a></td><td>` + tag + `</td><td class="row">` + act +
			`<form method="post" onsubmit="return confirm('Apagar este comentário?')">` + a.tok() + `<input type="hidden" name="acao" value="apagar"><input type="hidden" name="id" value="` + id + `"><button class="ghost">Apagar</button></form></td></tr>`)
	}
	b.WriteString(`</table></div>`)
	return a.out("Comentários", b.String(), true)
}

// ---------- biblioteca de mídia ----------

var mediaField = &Field{Name: "arquivo", Type: "file", Label: "Arquivo", Accept: "image/*,application/pdf", MaxMB: 10, Public: true}

// addMedia registra um arquivo enviado na biblioteca de mídia.
func (s *Server) addMedia(file lang.Value, who string) {
	t := lang.NewTable()
	t.SetStr("arquivo", file)
	t.SetStr("por", who)
	_, _ = s.Store.Insert(colMedia, t)
}

// mediaURLs lista as imagens da biblioteca (para o construtor visual).
func (s *Server) mediaURLs() []string {
	var out []string
	for _, r := range s.Store.All(colMedia, 200) {
		if f, ok := r.ToValue().Get("arquivo"); ok {
			if ft, ok := f.(*lang.Table); ok && strings.HasPrefix(getS(ft, "tipo"), "image/") {
				out = append(out, getS(ft, "url"))
			}
		}
	}
	return out
}

// mediaPage é /_admin/midia.
func (a *adminPage) mediaPage(post bool) error {
	r := a.rq.r
	msg := ""
	if post {
		switch r.PostForm.Get("acao") {
		case "enviar":
			if r.MultipartForm == nil {
				msg = "Escolha um arquivo."
				break
			}
			fhs := r.MultipartForm.File["arquivo"]
			if len(fhs) == 0 {
				msg = "Escolha um arquivo."
				break
			}
			if len(fhs) > 20 {
				fhs = fhs[:20]
			}
			for _, fh := range fhs {
				r.MultipartForm.File = map[string][]*multipart.FileHeader{"arquivo": {fh}}
				files, errs := checkFiles([]*Field{mediaField}, r)
				if len(errs) > 0 {
					msg = fh.Filename + ": " + errs[0]
					continue
				}
				t := lang.NewTable()
				if _, err := a.s.saveFiles(files, t); err != nil {
					return err
				}
				f, _ := t.Get("arquivo")
				a.s.addMedia(f, a.who())
			}
		case "apagar":
			id, _ := strconv.ParseInt(r.PostForm.Get("id"), 10, 64)
			if _, err := a.s.Store.Delete(colMedia, id); err != nil {
				return err
			}
		}
		if msg == "" {
			a.s.redirect(a.rq, a.base+"/midia", http.StatusSeeOther)
			return nil
		}
	}
	var b strings.Builder
	b.WriteString(cmsCSS + errP(msg))
	b.WriteString(`<form class="row" method="post" enctype="multipart/form-data">` + a.tok() + `<input type="hidden" name="acao" value="enviar">` +
		`<input type="file" name="arquivo" accept="image/*,application/pdf" multiple required><button>Enviar</button></form>` +
		`<p class="muted">Imagens e PDFs de até 10 MB. Copie o endereço para usar nos textos (![descrição](endereço)) ou escolha a imagem direto no construtor visual.</p>`)
	recs := a.s.Store.All(colMedia, 0)
	if len(recs) == 0 {
		b.WriteString(`<p class="muted">A biblioteca está vazia.</p>`)
		return a.out("Mídia", b.String(), true)
	}
	b.WriteString(`<div style="display:grid;grid-template-columns:repeat(auto-fill,minmax(180px,1fr));gap:14px">`)
	for _, rec := range recs {
		t := rec.ToValue()
		f, _ := t.Get("arquivo")
		ft, ok := f.(*lang.Table)
		if !ok {
			continue
		}
		u := getS(ft, "url")
		prev := `<div style="aspect-ratio:4/3;display:grid;place-items:center;background:#f1f3f2;border-radius:8px">PDF</div>`
		if strings.HasPrefix(getS(ft, "tipo"), "image/") {
			prev = `<img src="` + esc(u) + `" alt="" loading="lazy" style="width:100%;aspect-ratio:4/3;object-fit:cover;border-radius:8px">`
		}
		b.WriteString(`<div class="card" style="display:grid;gap:6px">` + prev + `<div style="font-size:.85rem;overflow-wrap:anywhere">` + esc(getS(ft, "nome")) + `</div>` +
			`<input readonly value="` + esc(u) + `" onclick="this.select()" style="font-size:.8rem">` +
			`<form method="post" onsubmit="return confirm('Apagar este arquivo? Onde ele estiver sendo usado, vai sumir.')">` + a.tok() + `<input type="hidden" name="acao" value="apagar"><input type="hidden" name="id" value="` + strconv.FormatInt(rec.ID, 10) + `"><button class="ghost">Apagar</button></form></div>`)
	}
	b.WriteString(`</div>`)
	return a.out("Mídia", b.String(), true)
}

// ---------- cópia de segurança ----------

// backupPage é /_admin/backup (só o dono): baixar e restaurar tudo.
func (a *adminPage) backupPage(post bool) error {
	r := a.rq.r
	if r.Method == http.MethodGet && r.URL.Query().Get("baixar") == "1" {
		return a.s.writeBackup(a.rq.w)
	}
	msg := ""
	if post && r.PostForm.Get("acao") == "restaurar" {
		var err error
		if msg, err = a.s.restoreBackup(r); err != nil {
			return err
		}
		if msg == "" {
			a.s.redirect(a.rq, a.base+"/backup?ok=1", http.StatusSeeOther)
			return nil
		}
	}
	var b strings.Builder
	b.WriteString(cmsCSS + errP(msg))
	if r.URL.Query().Get("ok") != "" {
		b.WriteString(`<p class="ok">Cópia restaurada. Se o login mudou, entre de novo com a senha da cópia.</p>`)
	}
	b.WriteString(`<p>Baixe uma cópia de tudo: conteúdo, textos, formulários recebidos, usuários, configurações e arquivos enviados. Guarde em lugar seguro: ela tem dados pessoais e as senhas (protegidas) do painel.</p>` +
		`<p><a class="btn" href="` + a.base + `/backup?baixar=1">Baixar cópia (.zip)</a></p>` +
		`<h2 style="font-size:1.1rem;margin-top:28px">Restaurar uma cópia</h2>` +
		`<p class="muted">Troca todos os dados atuais pelos da cópia. Os dados de agora ficam guardados em data/antes-da-restauracao-*.</p>` +
		`<form class="row" method="post" enctype="multipart/form-data" onsubmit="return confirm('Trocar todos os dados atuais pelos da cópia?')">` + a.tok() +
		`<input type="hidden" name="acao" value="restaurar"><input type="file" name="copia" accept=".zip,application/zip" required><button>Restaurar</button></form>`)
	return a.out("Cópia de segurança", b.String(), true)
}

func (s *Server) writeBackup(w http.ResponseWriter) error {
	unlock, err := s.Store.lock()
	if err != nil {
		return err
	}
	defer unlock()
	name := "copia-" + time.Now().Format("2006-01-02-1504") + ".zip"
	w.Header().Set("Content-Type", "application/zip")
	w.Header().Set("Content-Disposition", `attachment; filename="`+name+`"`)
	zw := zip.NewWriter(w)
	add := func(path, as string) error {
		f, err := os.Open(path)
		if err != nil {
			return nil // arquivo sumiu no meio do caminho: segue
		}
		defer f.Close()
		dst, err := zw.Create(as)
		if err != nil {
			return err
		}
		_, err = io.Copy(dst, f)
		return err
	}
	if err := add(filepath.Join(s.opt.DataDir, "nep.db"), "nep.db"); err != nil {
		return err
	}
	ents, _ := os.ReadDir(filepath.Join(s.opt.DataDir, "arquivos"))
	for _, e := range ents {
		if !e.IsDir() && fileNameRe.MatchString(e.Name()) {
			if err := add(filepath.Join(s.opt.DataDir, "arquivos", e.Name()), "arquivos/"+e.Name()); err != nil {
				return err
			}
		}
	}
	return zw.Close()
}

// restoreBackup troca os dados pelos da cópia enviada. Devolve uma mensagem de erro para a pessoa.
func (s *Server) restoreBackup(r *http.Request) (string, error) {
	if r.MultipartForm == nil || len(r.MultipartForm.File["copia"]) == 0 {
		return "Escolha o arquivo .zip da cópia.", nil
	}
	fh := r.MultipartForm.File["copia"][0]
	f, err := fh.Open()
	if err != nil {
		return "", err
	}
	defer f.Close()
	zr, err := zip.NewReader(f, fh.Size)
	if err != nil {
		return "Esse arquivo não é uma cópia do Nepenthes (.zip).", nil
	}
	var db *zip.File
	var files []*zip.File
	var total uint64
	for _, zf := range zr.File {
		total += zf.UncompressedSize64
		switch {
		case zf.Name == "nep.db":
			db = zf
		case strings.HasPrefix(zf.Name, "arquivos/") && fileNameRe.MatchString(strings.TrimPrefix(zf.Name, "arquivos/")):
			files = append(files, zf)
		}
	}
	if db == nil {
		return "A cópia não tem o banco (nep.db).", nil
	}
	if total > 2<<30 {
		return "Cópia grande demais.", nil
	}
	tmp, err := os.MkdirTemp(s.opt.DataDir, "restaurando-")
	if err != nil {
		return "", err
	}
	defer os.RemoveAll(tmp)
	extract := func(zf *zip.File, to string) error {
		rc, err := zf.Open()
		if err != nil {
			return err
		}
		defer rc.Close()
		if err := os.MkdirAll(filepath.Dir(to), 0o750); err != nil {
			return err
		}
		out, err := os.OpenFile(to, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o640)
		if err != nil {
			return err
		}
		_, err = io.Copy(out, io.LimitReader(rc, 2<<30))
		out.Close()
		return err
	}
	if err := extract(db, filepath.Join(tmp, "nep.db")); err != nil {
		return "", err
	}
	// confere que o banco da cópia é lido sem erro antes de trocar
	if _, err := OpenStore(tmp); err != nil {
		return "O banco da cópia está danificado: " + err.Error(), nil
	}
	for _, zf := range files {
		if err := extract(zf, filepath.Join(tmp, zf.Name)); err != nil {
			return "", err
		}
	}
	unlock, err := s.Store.lock()
	if err != nil {
		return "", err
	}
	defer unlock()
	old := filepath.Join(s.opt.DataDir, "antes-da-restauracao-"+time.Now().Format("20060102-150405"))
	if err := os.MkdirAll(old, 0o750); err != nil {
		return "", err
	}
	data := s.opt.DataDir
	_ = os.Rename(filepath.Join(data, "nep.db"), filepath.Join(old, "nep.db"))
	_ = os.Rename(filepath.Join(data, "arquivos"), filepath.Join(old, "arquivos"))
	if err := os.Rename(filepath.Join(tmp, "nep.db"), filepath.Join(data, "nep.db")); err != nil {
		return "", err
	}
	if _, err := os.Stat(filepath.Join(tmp, "arquivos")); err == nil {
		if err := os.Rename(filepath.Join(tmp, "arquivos"), filepath.Join(data, "arquivos")); err != nil {
			return "", err
		}
	}
	return "", s.Store.reload(true)
}

func (a *adminPage) pendingComments() int {
	n := 0
	for _, r := range a.s.Store.All(colComments, 0) {
		if getS(r.ToValue(), "status") == "pendente" {
			n++
		}
	}
	return n
}

// usesComments diz se o tema chama comentarios() em algum lugar.
func (site *Site) usesComments() bool { return strings.Contains(site.Src, "comentarios(") }

func mediaJSON(urls []string) string {
	if urls == nil {
		urls = []string{}
	}
	b, _ := json.Marshal(urls)
	return string(b)
}

// titleName tira o nome do site do <title> do tema ("{{ page.title }} · Revista" → "Revista").
func (site *Site) titleName() string {
	src := site.Src
	i := strings.Index(strings.ToLower(src), "<title>")
	if i < 0 {
		return ""
	}
	j := strings.Index(strings.ToLower(src[i:]), "</title>")
	if j < 0 {
		return ""
	}
	t := src[i+7 : i+j]
	if k := strings.LastIndex(t, "}}"); k >= 0 {
		t = t[k+2:]
	}
	t = strings.Trim(strings.TrimSpace(t), "·|-–— ")
	if strings.Contains(t, "{{") {
		return ""
	}
	return htmlUnescape(strings.TrimSpace(t))
}
