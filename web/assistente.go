package web

// Assistente: quando o arquivo do site tem a tag <nepenthes>, o nep não mostra
// o site; abre uma tela que pergunta como a pessoa quer o site, conversa com a
// IA (Qwen) e reescreve o arquivo inteiro com o site pronto.
//
//	<nepenthes></nepenthes>                       abre a tela em branco
//	<nepenthes>uma padaria em Olinda</nepenthes>  já começa com essa ideia
//
// A IA pode rodar em quatro lugares, nesta ordem de preferência na tela:
//   - no servidor, se o painel (IA) tiver Qwen configurado (DashScope ou Ollama);
//   - a IA do Nepenthes (pacote ialocal): llama.cpp + Granite 3.3 2B rodando no
//     próprio computador, sem GPU e sem chave; baixa uma vez (1,5 GB);
//   - no navegador de quem está criando, com o Qwen do WebLLM: nada para
//     instalar, o modelo é baixado uma vez e fica guardado no navegador;
//   - sem IA, pelas regras do gerador (o mesmo `nep criar`).
// A IA só escreve o conteúdo (JSON); quem monta o código é o gerador, que já
// garante visual profissional e um site que funciona.
//
// Só quem está no próprio computador (localhost) ou entrou no painel como
// administrador vê a tela e pode criar. Os outros veem "site em construção".

import (
	"context"
	_ "embed"
	"encoding/json"
	"fmt"
	"html"
	"net"
	"net/http"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/Nanofylium/probable-plant/gerador"
	"github.com/Nanofylium/probable-plant/ialocal"
)

//go:embed assets/assistente.html
var assistenteHTML string

var reTagNepenthes = regexp.MustCompile(`(?is)<nepenthes(?:\s[^>]*)?>(.*?)(?:</nepenthes>|$)`)

// pedeAssistente diz se o arquivo do site pede o assistente e devolve a ideia
// escrita dentro da tag (pode ser vazia).
func (s *Server) pedeAssistente() (bool, string) {
	b, err := os.ReadFile(s.file)
	if err != nil {
		return false, ""
	}
	m := reTagNepenthes.FindSubmatch(b)
	if m == nil {
		return false, ""
	}
	ideia := strings.TrimSpace(regexp.MustCompile(`(?s)<[^>]*>`).ReplaceAllString(string(m[1]), " "))
	return true, strings.Join(strings.Fields(ideia), " ")
}

// podeCriar: no próprio computador ou administrador logado.
func (s *Server) podeCriar(rq *request) bool {
	if rq.sess != nil && rq.sess.isAdmin() {
		return true
	}
	if s.opt.CGI || rq.r.Header.Get("X-Forwarded-For") != "" || rq.r.Header.Get("Forwarded") != "" {
		return false
	}
	host, _, err := net.SplitHostPort(rq.r.RemoteAddr)
	if err != nil {
		host = rq.r.RemoteAddr
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

func (s *Server) assistente(rq *request, ideia string) {
	w := rq.w
	w.Header().Set("Cache-Control", "no-store")
	if !s.podeCriar(rq) {
		s.simple(rq, http.StatusServiceUnavailable, "Site em construção",
			`<p>Este site está sendo criado. Volte daqui a pouco.</p><p style="font-size:.9em;opacity:.7">É o dono? <a href="`+s.url("/_admin")+`">Entre no painel</a> e volte a esta página.</p>`)
		return
	}
	rq.saveSession()
	switch rq.path {
	case "/_nep/assistente/estado":
		c := s.iaConf()
		modelo := ""
		if c.ready() {
			modelo = c.Modelo
		}
		writeJSON(w, map[string]any{
			"servidor": modelo, "ideia": ideia, "arquivo": s.file,
			"prompt_perguntas": gerador.PromptPerguntas(), "prompt_site": gerador.PromptSite(),
			"local": ialocal.Ver(), "local_pasta": ialocal.Pasta(), "visuais": gerador.Visuais,
			"instagram": s.instaResposta(s.instaGuardado()), "instagram_app": s.instaAppEstado(rq.r),
		})
		return
	case "/_nep/assistente/instagram":
		s.assistenteInstagram(rq)
		return
	case "/_nep/assistente/instagram/app":
		s.assistenteInstagramApp(rq)
		return
	case "/_nep/assistente/instagram/entrar":
		s.assistenteInstagramEntrar(rq)
		return
	case "/_nep/assistente/instagram/volta":
		s.assistenteInstagramVolta(rq)
		return
	case "/_nep/assistente/local":
		if rq.r.Method == http.MethodPost {
			if o := rq.r.Header.Get("Origin"); o != "" && !sameOrigin(o, rq.r) {
				http.Error(w, "use a própria tela", http.StatusBadRequest)
				return
			}
			ialocal.Baixar()
		}
		writeJSON(w, ialocal.Ver())
		return
	case "/_nep/assistente/perguntas", "/_nep/assistente/criar":
		if rq.r.Method != http.MethodPost || !sameOrigin(rq.r.Header.Get("Origin"), rq.r) && rq.r.Header.Get("Origin") != "" {
			http.Error(w, "use POST pela própria tela", http.StatusBadRequest)
			return
		}
		var in struct {
			Descricao string      `json:"descricao"`
			Respostas [][2]string `json:"respostas"`
			Conteudo  string      `json:"conteudo"` // resposta da IA do navegador
			Motor     string      `json:"motor"`    // servidor | navegador | regras
			Visual    string      `json:"visual"`   // um de gerador.Visuais; vazio = sorteia
		}
		if err := json.NewDecoder(http.MaxBytesReader(w, rq.r.Body, 256<<10)).Decode(&in); err != nil || strings.TrimSpace(in.Descricao) == "" {
			http.Error(w, "conte como quer o site", http.StatusBadRequest)
			return
		}
		if rq.path == "/_nep/assistente/perguntas" {
			s.assistentePerguntas(rq, in.Descricao, in.Motor)
			return
		}
		s.assistenteCriar(rq, in.Descricao, in.Respostas, in.Conteudo, in.Motor, in.Visual)
		return
	}
	if strings.HasPrefix(rq.path, "/_nep/") || strings.Contains(rq.path, ".") {
		http.NotFound(w, rq.r)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	// o Qwen do navegador: biblioteca WebLLM (jsDelivr), modelo (Hugging Face) e
	// o programa do modelo em WebAssembly (GitHub)
	w.Header().Set("Content-Security-Policy", "default-src 'self'; img-src 'self' data:; style-src 'self' 'unsafe-inline'; "+
		"script-src 'self' 'unsafe-inline' 'wasm-unsafe-eval' https://cdn.jsdelivr.net; worker-src 'self' blob:; "+
		"connect-src 'self' https://cdn.jsdelivr.net https://huggingface.co https://*.huggingface.co https://*.hf.co https://raw.githubusercontent.com; "+
		"frame-ancestors 'none'; base-uri 'none'; form-action 'self'; object-src 'none'")
	pagina := strings.ReplaceAll(assistenteHTML, "{{BASE}}", html.EscapeString(s.opt.Base))
	fmt.Fprint(w, pagina)
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	_ = json.NewEncoder(w).Encode(v)
}

var schemaPerguntas = map[string]any{"type": "object", "required": []string{"perguntas"}, "properties": map[string]any{
	"perguntas": map[string]any{"type": "array", "minItems": 2, "maxItems": 4, "items": map[string]any{"type": "string"}}}}

// askLocal pergunta à IA do Nepenthes (llama.cpp + Granite), ligando-a se
// preciso. Sem GPU leva de 30 segundos a uns minutos.
func (s *Server) askLocal(rq *request, schema map[string]any, system, user string, maxTokens int, temp float64) (string, error) {
	ctx, cancel := context.WithTimeout(rq.r.Context(), 4*time.Minute)
	defer cancel()
	u, err := ialocal.URL(ctx)
	if err != nil {
		return "", err
	}
	return s.chatCom(iaConf{URL: u, Modelo: "granite"}, 15*time.Minute, schema, "assistente (IA do Nepenthes)",
		[]iaMsg{{"system", system}, {"user", user}}, maxTokens, temp)
}

func (s *Server) assistentePerguntas(rq *request, desc, motor string) {
	var txt string
	var err error
	switch {
	case motor == "servidor" && s.iaConf().ready():
		txt, err = s.ask("assistente", gerador.PromptPerguntas(), s.comInstagram(desc), 400, 0.7)
	case motor == "local":
		txt, err = s.askLocal(rq, schemaPerguntas, gerador.PromptPerguntas(), s.comInstagram(desc), 400, 0.7)
	default:
		err = errNoIA
	}
	if p := gerador.LerPerguntas(txt); err == nil && len(p) > 0 {
		writeJSON(rq.w, map[string]any{"perguntas": p, "motor": motor})
		return
	}
	writeJSON(rq.w, map[string]any{"perguntas": gerador.PerguntasPorRegras(desc), "motor": "regras"})
}

func (s *Server) assistenteCriar(rq *request, desc string, resp [][2]string, conteudo, motor, visual string) {
	completa := gerador.DescricaoComRespostas(desc, resp)
	semente := time.Now().UnixNano() // cada pedido pode sair com outro visual
	p := gerador.EntenderComSemente(completa, semente)
	insta := s.instaGuardado()
	if insta != nil && !p.RamoAchado && insta.Bio != "" { // a bio ajuda a achar o ramo
		if p2 := gerador.EntenderComSemente(completa+". "+insta.Bio, semente); p2.RamoAchado {
			p = p2
		}
	}
	if !gerador.TrocarVisual(p, visual) && !p.VisualPedido {
		// sem escolha na tela: sorteia de novo entre os que combinam com o ramo
		// (o ramo pode ter mudado com a bio do Instagram)
		gerador.TrocarVisual(p, gerador.SortearVisual(p.Setor.ID, completa, semente))
	}
	usado := "regras"
	var aviso string
	if conteudo == "" && (motor == "servidor" && s.iaConf().ready() || motor == "local") {
		var txt string
		var err error
		if motor == "local" {
			txt, err = s.askLocal(rq, gerador.SchemaSite(), gerador.PromptSite(), s.comInstagram(gerador.MensagemSite(desc, resp)), 2500, 0.9)
		} else {
			txt, err = s.ask("assistente", gerador.PromptSite(), s.comInstagram(gerador.MensagemSite(desc, resp)), 2500, 0.9)
		}
		if err != nil {
			aviso = "A IA não respondeu (" + err.Error() + "); montei pelas regras."
		} else {
			conteudo, usado = txt, motor
		}
	} else if conteudo != "" {
		usado = "navegador"
	}
	if conteudo != "" {
		c, err := gerador.LerConteudo(conteudo)
		if err != nil {
			aviso = "Não consegui ler a resposta da IA (" + err.Error() + "); montei pelas regras."
			usado = "regras"
		} else {
			gerador.Aplicar(p, c)
		}
	}
	if insta != nil {
		gerador.UsarInstagram(p, insta.Fotos, insta.Usuario, insta.Nome, insta.Site)
	}
	src := gerador.Montar(p)

	// guarda o arquivo anterior e grava o novo
	antes, _ := os.ReadFile(s.file)
	copia := s.file + ".antes-" + time.Now().Format("20060102-150405")
	if len(antes) > 0 {
		if err := os.WriteFile(copia, antes, 0o644); err != nil {
			http.Error(rq.w, "não consegui guardar o arquivo anterior: "+err.Error(), http.StatusInternalServerError)
			return
		}
	}
	if err := os.WriteFile(s.file, []byte(src), 0o644); err != nil {
		http.Error(rq.w, "não consegui gravar o site: "+err.Error(), http.StatusInternalServerError)
		return
	}
	if _, err := ParseSite("check", src); err != nil { // nunca deveria: o gerador tem testes
		_ = os.WriteFile(s.file, antes, 0o644)
		http.Error(rq.w, "o site gerado tem erro, voltei o anterior: "+err.Error(), http.StatusInternalServerError)
		return
	}
	arquivos := []string{s.file}
	if s.opt.AposCriar != nil {
		arquivos = append(arquivos, s.opt.AposCriar(s.file)...)
	}
	writeJSON(rq.w, map[string]any{
		"ok": true, "motor": usado, "aviso": aviso, "nome": p.Nome, "notas": p.Notas,
		"arquivos": arquivos, "copia": copia, "ir": s.url("/"),
	})
}

// comInstagram junta à mensagem para a IA o que veio do Instagram.
func (s *Server) comInstagram(msg string) string {
	if d := s.instaGuardado(); d != nil && d.Resumo != "" {
		return msg + "\nDo Instagram do negócio (use o jeito de falar e os produtos daqui):\n" + d.Resumo
	}
	return msg
}
