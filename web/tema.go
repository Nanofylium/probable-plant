package web

import (
	"crypto/sha256"
	_ "embed"
	"encoding/hex"
	"html"
	"net/http"
	"regexp"
	"strings"
	"time"
)

// Tema Nepenthes: a aparência padrão, para todo site sair com cara profissional
// mesmo sem uma linha de CSS. Ver assets/tema.css.

//go:embed assets/tema.css
var temaCSS []byte

var temaVersao = func() string {
	h := sha256.Sum256(temaCSS)
	return hex.EncodeToString(h[:4])
}()

func serveTema(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/css; charset=utf-8")
	w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
	http.ServeContent(w, r, "", time.Time{}, strings.NewReader(string(temaCSS)))
}

// Peças que o nep gera (avisos de formulário, comentários): um mínimo para não
// aparecerem cruas num site que tem CSS próprio e não as estilizou.
const pecasCSS = `<style>:where(.nep-ok){background:#e7f5ec;color:#145c34;border:1px solid #c4e6d0;padding:12px 16px;border-radius:12px;font-weight:600}` +
	`:where(.nep-erros){background:#fdeeec;color:#8c2219;border:1px solid #f5cdc8;padding:10px 16px;border-radius:12px}:where(.nep-erros ul){margin:0;padding-left:1.1em}` +
	`:where(.nep-comentarios){margin-top:40px}:where(.nep-comentario){border:1px solid rgba(0,0,0,.1);border-radius:12px;padding:12px 16px;margin:0 0 12px}` +
	`:where(.nep-comentario-autor){font-weight:700}:where(.nep-comentar){display:grid;gap:12px;max-width:640px}:where(.nep-comentar label){display:grid;gap:6px;font-weight:600}` +
	`:where(.nep-comentar input,.nep-comentar textarea){font:inherit;padding:10px 12px;border:1px solid rgba(0,0,0,.2);border-radius:10px}` +
	`:where(.nep-comentar button){justify-self:start;font:inherit;font-weight:600;padding:10px 18px;border-radius:999px;border:0;cursor:pointer}:where(.nep-comentar-nota){opacity:.75;font-size:.9em}</style>`

var corRe = regexp.MustCompile(`^#[0-9a-fA-F]{3}([0-9a-fA-F]{3})?$`)

var fontes = map[string]string{
	"serifa":        `Georgia, "Iowan Old Style", "Times New Roman", serif`,
	"sem-serifa":    "",
	"arredondada":   `ui-rounded, "SF Pro Rounded", "Nunito", system-ui, sans-serif`,
	"mono":          `ui-monospace, SFMono-Regular, Menlo, Consolas, monospace`,
	"classica":      `"Palatino Linotype", Palatino, "Book Antiqua", Georgia, serif`,
	"geometrica":    `"Avenir Next", Avenir, Montserrat, "Segoe UI", system-ui, sans-serif`,
	"humanista":     `"Gill Sans", "Gill Sans MT", Seravek, Calibri, "Trebuchet MS", sans-serif`,
	"titulo-serifa": `Georgia, "Iowan Old Style", serif`,
}

// temaTags devolve o que entra no começo do <head> (antes do CSS do site, que
// assim manda) e o que entra no fim.
func (s *Server) temaTags(cfg map[string]string, out string) (inicio []string, fim []string) {
	low := strings.ToLower(out)
	if !strings.Contains(low, `name="viewport"`) && !strings.Contains(low, `name=viewport`) {
		inicio = append(inicio, `<meta name="viewport" content="width=device-width, initial-scale=1">`)
	}
	modo := strings.ToLower(strings.TrimSpace(cfg["tema"]))
	usar := false
	switch modo {
	case "nenhum", "false", "0", "nao", "não", "none":
	case "":
		// sem CSS próprio: o site ganha o tema sozinho
		usar = !strings.Contains(low, "<style") && !strings.Contains(low, `rel="stylesheet"`) && !strings.Contains(low, "rel=stylesheet")
	default:
		usar = true
	}
	if usar {
		inicio = append(inicio, `<link rel="stylesheet" href="`+html.EscapeString(s.opt.Base)+`/_nep/tema.css?v=`+temaVersao+`">`)
		var vars []string
		if c := strings.TrimSpace(cfg["cor"]); corRe.MatchString(c) {
			vars = append(vars, "--nep-cor:"+c)
		}
		if f, ok := fontes[strings.ToLower(strings.TrimSpace(cfg["fonte"]))]; ok && f != "" {
			if strings.ToLower(strings.TrimSpace(cfg["fonte"])) == "titulo-serifa" {
				vars = append(vars, "--nep-fonte-titulo:"+f)
			} else {
				vars = append(vars, "--nep-fonte:"+f)
			}
		}
		if len(vars) > 0 {
			inicio = append(inicio, `<style>:root{`+strings.Join(vars, ";")+`}</style>`)
		}
	} else if strings.Contains(out, `class="nep-ok"`) || strings.Contains(out, `class="nep-erros"`) || strings.Contains(out, `class="nep-comentarios"`) {
		inicio = append(inicio, pecasCSS)
	}
	return inicio, fim
}

// noComecoDoHead põe tags logo depois de <head ...> (ou no começo da página).
func noComecoDoHead(out string, tags []string) string {
	if len(tags) == 0 {
		return out
	}
	t := strings.Join(tags, "\n")
	low := strings.ToLower(out)
	if i := tagFora(low, "<head"); i >= 0 {
		if j := strings.Index(low[i:], ">"); j >= 0 {
			k := i + j + 1
			return out[:k] + "\n" + t + out[k:]
		}
	}
	if i := tagFora(low, "<html"); i >= 0 {
		if j := strings.Index(low[i:], ">"); j >= 0 {
			k := i + j + 1
			return out[:k] + "<head>" + t + "</head>" + out[k:]
		}
	}
	return t + "\n" + out
}

// tagFora acha a tag (ex.: "<head") fora de comentários <!-- -->, seguida de
// ">" ou espaço (para "<head" não casar com "<header").
func tagFora(low, tag string) int {
	pos := 0
	for pos < len(low) {
		i := strings.Index(low[pos:], tag)
		if i < 0 {
			return -1
		}
		i += pos
		if c := strings.LastIndex(low[:i], "<!--"); c >= 0 && !strings.Contains(low[c:i], "-->") {
			end := strings.Index(low[i:], "-->")
			if end < 0 {
				return -1
			}
			pos = i + end + 3
			continue
		}
		if n := i + len(tag); n < len(low) && (low[n] == '>' || low[n] == ' ' || low[n] == '\t' || low[n] == '\n' || low[n] == '\r') {
			return i
		}
		pos = i + len(tag)
	}
	return -1
}
