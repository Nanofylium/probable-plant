package web

import (
	"os"
	"strings"
	"testing"
)

// Um index.html só com a tag <nepenthes tipo=...> vira o site inteiro, sem
// tela e sem reescrever o arquivo; vale também no CGI (hospedagem).
func TestTagComMoldeMontaOSite(t *testing.T) {
	casos := []struct {
		tag     string
		paginas map[string]string // rota -> texto que tem de aparecer
	}{
		{`<nepenthes tipo="portal" nome="Folha do Vale" cidade="Aveiro" whatsapp="(93) 99123-4567" email="redacao@folha.com.br">Portal de notícias com agenda.</nepenthes>`,
			map[string]string{"/": "Folha do Vale", "/noticias": "Notícias", "/agenda": "Agenda", "/sobre": "Redação", "/contato": "wa.me/5593991234567", "/perguntas": "pauta"}},
		{`<nepenthes tipo="blog" nome="Caderno da Vó">Blog de receitas de família.</nepenthes>`,
			map[string]string{"/": "Caderno da Vó", "/sobre": "Sobre", "/contato": "Mensagem", "/busca": "Buscar"}},
		{`<nepenthes tipo="wiki" nome="Wiki da Horta">Base de conhecimento da horta.</nepenthes>`,
			map[string]string{"/": "Wiki da Horta", "/indice": "Glossário", "/wiki/como-contribuir": "Sugerir edição", "/wiki/nao-existe": "ainda não existe", "/recentes": "Mudanças recentes"}},
		{`<nepenthes tipo="estatico" nome="Studio Bela" instagram="@bela">Salão de beleza com cortes.</nepenthes>`,
			map[string]string{"/": "Studio Bela"}},
		{`<nepenthes tipo="auto" nome="Docs do Projeto">Documentação e manual do projeto.</nepenthes>`,
			map[string]string{"/indice": "Índice A–Z"}},
	}
	for _, c := range casos {
		src := "<!doctype html>\n<html><body>\n" + c.tag + "\n</body></html>\n"
		for _, cgi := range []bool{false, true} {
			s, f := assistenteSrv(t, src)
			s.opt.CGI = cgi
			for rota, quer := range c.paginas {
				w := pedir(s, "GET", rota, "", "203.0.113.9")
				if w.Code != 200 || !strings.Contains(w.Body.String(), quer) {
					t.Fatalf("%s %s (cgi=%v): %d, sem %q\n%.600s", c.tag[:30], rota, cgi, w.Code, quer, w.Body.String())
				}
				if strings.Contains(w.Body.String(), "{{") || strings.Contains(w.Body.String(), "⟦") {
					t.Fatalf("%s %s: sobrou marcação de modelo", c.tag[:30], rota)
				}
			}
			if b, _ := os.ReadFile(f); string(b) != src {
				t.Fatal("o arquivo da pessoa não pode mudar")
			}
		}
	}
	s, _ := assistenteSrv(t, `<nepenthes tipo="blog">x</nepenthes>`)
	for _, a := range []string{"/_nep/molde/blog.css", "/_nep/molde/bootstrap.bundle.min.js", "/_nep/molde/img/mesa.jpg"} {
		if w := pedir(s, "GET", a, "", "127.0.0.1"); w.Code != 200 {
			t.Fatalf("%s: %d", a, w.Code)
		}
	}
	for _, a := range []string{"/_nep/molde/../moldes.go", "/_nep/molde/img/../../modelos/blog.tmpl"} {
		if w := pedir(s, "GET", a, "", "127.0.0.1"); w.Code == 200 {
			t.Fatalf("%s não deveria sair", a)
		}
	}
}

func TestTagSemAtributosAindaAbreOAssistente(t *testing.T) {
	s, _ := assistenteSrv(t, `<nepenthes>uma padaria</nepenthes>`)
	if w := pedir(s, "GET", "/", "", "127.0.0.1"); !strings.Contains(w.Body.String(), "assistente") {
		t.Fatalf("sem atributos a tag continua chamando o assistente: %.300s", w.Body.String())
	}
}

func TestTagComTipoErradoExplica(t *testing.T) {
	s, _ := assistenteSrv(t, `<nepenthes tipo="loja-virtual">x</nepenthes>`)
	s.opt.Dev = true
	w := pedir(s, "GET", "/", "", "127.0.0.1")
	if !strings.Contains(w.Body.String(), "blog, portal, wiki, estatico") {
		t.Fatalf("o erro deveria listar os tipos: %.400s", w.Body.String())
	}
}
