package web

// /_admin/sites: o dono cria um site novo na própria hospedagem, pelo
// navegador, sem zip, sem .exe e sem API do cPanel. O nep que já está no ar
// (por exemplo /mural/nep.cgi) se copia para uma pasta nova ao lado, grava o
// .htaccess e um index.html com a tag <nepenthes tipo=...>, e o site novo já
// abre com a mesma senha do painel.

import (
	_ "embed"
	"fmt"
	"io"

	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/Nanofylium/probable-plant/moldes"
)

//go:embed assets/htaccess.txt
var htaccessCGI string

var rePasta = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{1,39}$`)

// pastasReservadas nunca viram site (são do cPanel ou do próprio servidor).
var pastasReservadas = map[string]bool{"cgi-bin": true, "data": true, "well-known": true, "mail": true, "webmail": true, "cpanel": true, "ftp": true, "tmp": true, "logs": true, "etc": true}

// raizWeb: a pasta que o domínio serve ("public_html"). No CGI vem do Apache.
func (s *Server) raizWeb() string {
	if d := os.Getenv("DOCUMENT_ROOT"); s.opt.CGI && d != "" {
		if abs, err := filepath.Abs(d); err == nil {
			return abs
		}
	}
	return filepath.Dir(s.dir)
}

type siteNaPasta struct{ Pasta, Arquivo string }

// sitesNaRaiz: pastas da raiz com um nep.cgi e um site dentro.
func (s *Server) sitesNaRaiz() []siteNaPasta {
	raiz := s.raizWeb()
	es, _ := os.ReadDir(raiz)
	out := []siteNaPasta{}
	for _, e := range es {
		if !e.IsDir() {
			continue
		}
		d := filepath.Join(raiz, e.Name())
		if _, err := os.Stat(filepath.Join(d, "nep.cgi")); err != nil {
			continue
		}
		for _, n := range []string{"site.nep.html", "index.html"} {
			if _, err := os.Stat(filepath.Join(d, n)); err == nil {
				out = append(out, siteNaPasta{e.Name(), n})
				break
			}
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Pasta < out[j].Pasta })
	return out
}

func attr(s string) string { return strings.ReplaceAll(esc(strings.TrimSpace(s)), "\n", " ") }

func (a *adminPage) sitesPage(post bool) error {
	s := a.s
	r := a.rq.r
	msg, feito := "", ""
	v := func(k string) string { return strings.TrimSpace(r.PostForm.Get(k)) }
	if post {
		pasta := strings.ToLower(v("pasta"))
		raiz := s.raizWeb()
		dest := filepath.Join(raiz, pasta)
		_, tipoOK := moldes.Achar(v("tipo"))
		switch {
		case !rePasta.MatchString(pasta) || pastasReservadas[pasta]:
			msg = "O nome da pasta usa só letras minúsculas, números e hífen (ex.: folha-do-vale)."
		case !tipoOK && v("tipo") != "auto":
			msg = "Escolha o tipo de site."
		case v("nome") == "":
			msg = "Dê um nome ao site."
		case filepath.Dir(dest) != raiz:
			msg = "Pasta inválida."
		default:
			if _, err := os.Stat(dest); err == nil {
				msg = "Já existe uma pasta " + pasta + " na hospedagem. Escolha outro nome."
				break
			}
			if err := s.criarSiteNaPasta(dest, v); err != nil {
				_ = os.RemoveAll(dest) // não deixa site pela metade
				msg = "Não consegui criar o site: " + err.Error()
				break
			}
			if !s.opt.CGI { // no computador: não há Apache para servir a pasta nova
				feito = `<p class="ok">Pronto! A pasta <code>` + esc(dest) + `</code> foi criada. Para ver, rode <code>nep dev ` + esc(filepath.Join(dest, "index.html")) + `</code>. Na hospedagem, o site já sai no ar sozinho.</p>`
				break
			}
			link := "/" + pasta + "/"
			feito = `<p class="ok">Pronto! O site está no ar em <a href="` + esc(link) + `" target="_blank"><b>` + esc(link) + `</b></a>. ` +
				`O painel dele é <a href="` + esc(link) + `_admin" target="_blank">` + esc(link) + `_admin</a>, com a mesma senha deste painel.</p>`
		}
	}
	var lista strings.Builder
	for _, x := range s.sitesNaRaiz() {
		link := "/" + x.Pasta + "/"
		fmt.Fprintf(&lista, `<li><a href="%s" target="_blank">%s</a> <small>(%s)</small></li>`, esc(link), esc(link), esc(x.Arquivo))
	}
	opc := ""
	for _, m := range moldes.Lista {
		sel := ""
		if v("tipo") == m.ID || (v("tipo") == "" && m.ID == "portal") {
			sel = " selected"
		}
		opc += `<option value="` + m.ID + `"` + sel + `>` + esc(m.Nome+": "+m.Para) + `</option>`
	}
	opc += `<option value="auto">Deixar o Nepenthes escolher pela descrição</option>`
	campo := func(nome, rot, ph string) string {
		return `<label>` + rot + `<br><input name="` + nome + `" value="` + attr(v(nome)) + `" placeholder="` + ph + `" style="width:100%"></label>`
	}
	corpo := errP(msg) + feito +
		`<p>Cria um site novo nesta hospedagem, numa pasta ao lado deste. Não precisa enviar arquivo nenhum: o Nepenthes que já está no ar se copia para a pasta nova e monta o site pela tag <code>&lt;nepenthes&gt;</code>.</p>` +
		`<form method="post" class="row" style="flex-direction:column;align-items:stretch;max-width:560px">` + a.tok() +
		`<label>Tipo de site<br><select name="tipo" style="width:100%">` + opc + `</select></label>` +
		campo("nome", "Nome do site", "Folha do Vale") +
		`<label>Endereço (nome da pasta)<br><input name="pasta" required pattern="[a-z0-9][a-z0-9\-]{1,39}" value="` + attr(v("pasta")) + `" placeholder="folha-do-vale" style="width:100%"></label>` +
		`<label>Descrição (o que é o site, para quem, o que tem)<br><textarea name="descricao" rows="3" style="width:100%">` + esc(v("descricao")) + `</textarea></label>` +
		campo("cidade", "Cidade", "Aveiro") + campo("whatsapp", "WhatsApp", "(93) 90000-0000") + campo("email", "E-mail que recebe as mensagens", "contato@seusite.com.br") +
		campo("endereco", "Endereço", "Rua das Flores, 120") + campo("horario", "Horário", "Seg a sex, 8h às 18h") + campo("instagram", "Instagram", "@seuperfil") +
		campo("pix", "Chave Pix (associação: recebe as mensalidades)", "tesouraria@seusite.org.br") + campo("mensalidade", "Mensalidade (associação)", "25,00") +
		`<button>Criar o site</button></form>`
	if lista.Len() > 0 {
		corpo += `<h3>Sites nesta hospedagem</h3><ul>` + lista.String() + `</ul>`
	}
	return a.out("Criar site na hospedagem", corpo, true)
}

// criarSiteNaPasta grava nep.cgi (cópia deste programa), .htaccess, index.html
// com a tag e a pasta de dados com a senha do painel atual.
func (s *Server) criarSiteNaPasta(dest string, v func(string) string) error {
	exe, err := os.Executable()
	if err != nil {
		return err
	}
	if err := os.Mkdir(dest, 0o755); err != nil {
		return err
	}
	if err := copiarArquivo(exe, filepath.Join(dest, "nep.cgi"), 0o755); err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(dest, ".htaccess"), []byte(htaccessCGI), 0o644); err != nil {
		return err
	}
	var tag strings.Builder
	tag.WriteString(`<nepenthes tipo="` + attr(v("tipo")) + `"`)
	for _, k := range []string{"nome", "cidade", "whatsapp", "email", "endereco", "horario", "instagram", "pix", "mensalidade"} {
		if x := v(k); x != "" {
			tag.WriteString(` ` + k + `="` + attr(x) + `"`)
		}
	}
	tag.WriteString(">\n  " + esc(v("descricao")) + "\n</nepenthes>")
	idx := "<!doctype html>\n<html lang=\"pt-BR\">\n<head><meta charset=\"utf-8\"><title>" + esc(v("nome")) + "</title></head>\n<body>\n" +
		"<!-- Este arquivo é o site inteiro. Troque o tipo (blog, portal, wiki, estatico) ou os dados na tag e salve. -->\n" +
		tag.String() + "\n</body>\n</html>\n"
	if _, _, err := moldes.Expandir([]byte(idx)); err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(dest, "index.html"), []byte(idx), 0o644); err != nil {
		return err
	}
	st, err := OpenStore(filepath.Join(dest, "data"))
	if err != nil {
		return err
	}
	return st.SetMeta("admin_hash", s.Store.Meta("admin_hash"))
}

func copiarArquivo(de, para string, modo os.FileMode) error {
	in, err := os.Open(de)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.OpenFile(para, os.O_CREATE|os.O_EXCL|os.O_WRONLY, modo)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		out.Close()
		return err
	}
	if err := out.Close(); err != nil {
		return err
	}
	return os.Chmod(para, modo)
}
