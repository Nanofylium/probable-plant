package main

import (
	"bytes"
	"context"
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/Nanofylium/probable-plant/ialocal"
)

// A cada uso, os dois cliques abrem /comecar: "Criar um site" ou "Programar".
// Programar é um editor com o botão Rodar e a IA do Nepenthes (Granite, no
// próprio computador) que escreve e conserta código Nepenthes ou Lua.

//go:embed comecar.html
var comecarHTML string

//go:embed programar.html
var programarHTML string

const (
	maxPrograma = 256 << 10
	maxSaida    = 128 << 10
	tempoRodar  = 20 * time.Second
)

// iaChat fala com a IA do Nepenthes (trocado nos testes).
var iaChat = func(ctx context.Context, msgs []map[string]string, maxTokens int) (string, error) {
	u, err := ialocal.URL(ctx)
	if err != nil {
		return "", err
	}
	corpo, _ := json.Marshal(map[string]any{"model": "granite", "messages": msgs, "max_tokens": maxTokens, "temperature": 0.2})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, u+"/chat/completions", bytes.NewReader(corpo))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	r, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer r.Body.Close()
	var out struct {
		Choices []struct {
			Message struct{ Content string } `json:"message"`
		} `json:"choices"`
	}
	b, _ := io.ReadAll(io.LimitReader(r.Body, 1<<20))
	if r.StatusCode != 200 || json.Unmarshal(b, &out) != nil || len(out.Choices) == 0 {
		return "", fmt.Errorf("a IA respondeu %d", r.StatusCode)
	}
	return out.Choices[0].Message.Content, nil
}

// pastaProgramas: "Programas do Nepenthes", ao lado da pasta dos sites.
func (c *central) pastaProgramas() string {
	return filepath.Join(filepath.Dir(c.base), "Programas do Nepenthes")
}

var reNomePrograma = regexp.MustCompile(`^[\p{L}\p{N}_\- ]{1,60}\.(nep|lua)$`)

// arquivoPrograma valida o nome (sem pastas) e devolve o caminho.
func (c *central) arquivoPrograma(nome string) (string, error) {
	nome = strings.TrimSpace(nome)
	if !reNomePrograma.MatchString(nome) || strings.Contains(nome, "..") {
		return "", errors.New("nome inválido: use letras, números e termine em .nep ou .lua")
	}
	return filepath.Join(c.pastaProgramas(), nome), nil
}

type programa struct {
	Nome      string `json:"nome"`
	Alterado  string `json:"alterado"`
	alteradoT time.Time
}

func (c *central) programas() []programa {
	out := []programa{}
	ents, _ := os.ReadDir(c.pastaProgramas())
	for _, e := range ents {
		if e.IsDir() || !reNomePrograma.MatchString(e.Name()) {
			continue
		}
		if st, err := e.Info(); err == nil {
			out = append(out, programa{Nome: e.Name(), Alterado: st.ModTime().Format("02/01/2006 15:04"), alteradoT: st.ModTime()})
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].alteradoT.After(out[j].alteradoT) })
	return out
}

// salvarPrograma grava o código na pasta dos programas (criando-a).
func (c *central) salvarPrograma(nome, codigo string) (string, error) {
	f, err := c.arquivoPrograma(nome)
	if err != nil {
		return "", err
	}
	if len(codigo) > maxPrograma {
		return "", errors.New("programa grande demais")
	}
	if err := os.MkdirAll(c.pastaProgramas(), 0o755); err != nil {
		return "", err
	}
	return f, os.WriteFile(f, []byte(codigo), 0o644)
}

type saidaPrograma struct {
	Saida  string `json:"saida"`
	Erro   string `json:"erro,omitempty"`
	Codigo int    `json:"codigo"`
	MS     int64  `json:"ms"`
	Cortou bool   `json:"cortou,omitempty"`
}

type limitado struct {
	b     bytes.Buffer
	cheio bool
}

func (l *limitado) Write(p []byte) (int, error) {
	if falta := maxSaida - l.b.Len(); falta < len(p) {
		l.cheio = true
		if falta > 0 {
			l.b.Write(p[:falta])
		}
		return len(p), nil
	}
	return l.b.Write(p)
}

// rodarPrograma roda o arquivo num processo à parte (o próprio nep), na pasta
// dos programas, com tempo limite: um laço infinito não trava a Central.
func rodarPrograma(ctx context.Context, arquivo, entrada string) saidaPrograma {
	exe, err := os.Executable()
	if err != nil {
		return saidaPrograma{Erro: err.Error(), Codigo: -1}
	}
	ctx, cancel := context.WithTimeout(ctx, tempoRodar)
	defer cancel()
	cmd := exec.CommandContext(ctx, exe, "run", filepath.Base(arquivo))
	cmd.Dir = filepath.Dir(arquivo)
	cmd.Stdin = strings.NewReader(entrada)
	var saida limitado
	cmd.Stdout, cmd.Stderr = &saida, &saida
	cmd.WaitDelay = 2 * time.Second
	escondeJanela(cmd)
	t := time.Now()
	err = cmd.Run()
	r := saidaPrograma{Saida: saida.b.String(), MS: time.Since(t).Milliseconds(), Cortou: saida.cheio}
	if ctx.Err() == context.DeadlineExceeded {
		r.Erro, r.Codigo = fmt.Sprintf("parei o programa: passou de %d segundos", int(tempoRodar.Seconds())), -1
	} else if ee, ok := err.(*exec.ExitError); ok {
		r.Codigo = ee.ExitCode()
	} else if err != nil {
		r.Erro, r.Codigo = err.Error(), -1
	}
	return r
}

// promptProgramar ensina a IA a escrever Nepenthes (ou Lua).
func promptProgramar(lua bool) string {
	if lua {
		return "Você é a IA do Nepenthes e escreve programas em Lua 5.4 que rodam no Nepenthes (biblioteca padrão do Lua: string, table, math, os.time/os.date, io.write, io.read). " +
			"Responda em português do Brasil: uma frase curta explicando e depois o programa inteiro num bloco ```lua. Não use bibliotecas externas."
	}
	return `Você é a IA do Nepenthes e escreve programas na linguagem Nepenthes. Responda em português do Brasil: uma frase curta explicando e depois o programa INTEIRO num bloco ` + "```nep" + `.

A linguagem Nepenthes (parecida com Python e Lua, palavras em inglês, nomes podem ter acento):
- Comentário: # até o fim da linha.
- Variável nova: let x = 10. Alterar: x = 11, x += 1. Alterar sem let antes é erro.
- Blocos sempre com do ... end:
  if x > 3 do ... elif x == 3 do ... else ... end
  while cond do ... end
  for item in lista do ... end      for i, item in lista do ... end      for chave, valor in tabela do ... end
  fn soma(a, b) do return a + b end     fn dobro(x) do x * 2 end   (a última expressão é devolvida)
  função anônima: fn(x) do x * 2 end
  try do ... catch e do print(e.message) end
- Tipos: nil, true/false, int, float, "texto" ou 'texto', lista [1, 2, 3] (índice começa em 0, l[-1] é o último), tabela {nome: "Ana", idade: 30} (t.nome lê o campo).
- Texto com valores: ` + "`Olá, {nome}! Total: {money(total)}`" + `
- Lógica: and, or, not. Comparação: == != < > <= >=. Divisão inteira: //. Resto: %.
- Pipe: x |> f(a) é f(x, a). Ex.: produtos |> filter(fn(p) do p.qtd > 0 end) |> pluck("nome")
- Métodos em listas e textos: lista.sort(), "ana".upper(), (19.9).money()
- Funções globais: print(...), input(pergunta), str, int, float, len, type, copy, default(v, padrao), format("{} e {}", a, b), upper, lower, trim, replace(s, a, b), split(s, sep), join(lista, sep), contains, starts_with, slug, pad, round(x, casas), floor, ceil, abs, money(x) dá "R$ 19,90", number(x, casas), sum, avg, min, max, range(n) ou range(a, b), map, filter, find, reduce(lista, fn(acc, x), inicial), any, all, sort, sort_by(lista, "campo"), group_by, count_by, where, pluck, take, skip, first, last, reverse, push(lista, v), pop, remove, unique, zip, enumerate, keys, values, has(tabela, chave), merge, now(), date(valor, "d/m/Y"), load_csv(caminho), error(msg), assert(cond, msg).
- Módulos (import nome no topo): math (pi sqrt pow random random_int(a, b) choice shuffle), fs (read write append lines exists list mkdir), json (encode pretty decode), time (unix sleep(ms) format add_days diff_days), http (get post → {status, ok, body}), os (args env).
- Não existe classe nem ponto e vírgula. Uma instrução por linha. Feche todo do com end.

Exemplo:
` + "```nep" + `
import json
let produtos = [{nome: "Pão", preco: 6.5, qtd: 40}, {nome: "Bolo", preco: 30, qtd: 0}]
fn valor(p) do p.preco * p.qtd end
let em_falta = produtos |> filter(fn(p) do p.qtd == 0 end) |> pluck("nome")
print("Estoque vale", produtos |> map(valor) |> sum |> money)
for p in produtos do
  if p.qtd == 0 do
    print(` + "`Falta {p.nome}`" + `)
  end
end
print(json.encode(em_falta))
` + "```"
}

var reBlocoCodigo = regexp.MustCompile("(?s)```[a-zA-Z]*[ \t]*\r?\n(.*?)```")

// separarCodigo tira o programa da resposta da IA (o maior bloco ```).
func separarCodigo(txt string) (explicacao, codigo string) {
	ms := reBlocoCodigo.FindAllStringSubmatchIndex(txt, -1)
	if len(ms) == 0 {
		return "", strings.TrimSpace(txt)
	}
	melhor := ms[0]
	for _, m := range ms[1:] {
		if m[3]-m[2] > melhor[3]-melhor[2] {
			melhor = m
		}
	}
	explicacao = strings.TrimSpace(txt[:melhor[0]])
	if len(explicacao) > 600 {
		explicacao = explicacao[:600] + "…"
	}
	return explicacao, strings.TrimRight(txt[melhor[2]:melhor[3]], " \t\r\n") + "\n"
}

type pedidoIA struct {
	Pedido, Codigo, Erro string
	Lua                  bool
}

// iaPrograma pede o código à IA; se não compilar, mostra o erro e pede de
// novo (até 2 consertos).
func iaPrograma(ctx context.Context, in pedidoIA) (map[string]any, error) {
	pedido := strings.TrimSpace(in.Pedido)
	if pedido == "" && in.Erro == "" {
		return nil, errors.New("diga o que o programa deve fazer")
	}
	ext := ".nep"
	if in.Lua {
		ext = ".lua"
	}
	user := pedido
	if strings.TrimSpace(in.Codigo) != "" {
		user = "Programa atual:\n```\n" + in.Codigo + "\n```\n"
		if in.Erro != "" {
			user += "Ao rodar deu este erro:\n" + in.Erro + "\nConserte o programa. "
		}
		if pedido != "" {
			user += "Pedido: " + pedido
		}
	}
	msgs := []map[string]string{{"role": "system", "content": promptProgramar(in.Lua)}, {"role": "user", "content": user}}
	var explicacao, codigo string
	tentativas := 0
	for tentativas < 3 {
		tentativas++
		txt, err := iaChat(ctx, msgs, 1500)
		if err != nil {
			return nil, err
		}
		explicacao, codigo = separarCodigo(txt)
		_, errC := compileAny(codigo, "programa"+ext)
		if errC == nil {
			return map[string]any{"codigo": codigo, "explicacao": explicacao, "tentativas": tentativas, "ok": true}, nil
		}
		msgs = append(msgs, map[string]string{"role": "assistant", "content": txt},
			map[string]string{"role": "user", "content": "Esse programa não compila: " + errC.Error() + "\nMande o programa inteiro corrigido."})
	}
	return map[string]any{"codigo": codigo, "explicacao": explicacao, "tentativas": tentativas, "ok": false,
		"aviso": "A IA tentou 3 vezes e o programa ainda tem erro. Veja a mensagem ao rodar e peça de novo, ou corrija à mão."}, nil
}

// siteNovo: a pasta com a tag <nepenthes> que ainda não virou site (ou uma
// nova), ligada; devolve o endereço da tela que pergunta e cria o site.
func (c *central) siteNovo() (string, error) {
	pasta := ""
	for _, s := range c.listar() {
		if s.Novo {
			pasta = s.Pasta
			break
		}
	}
	if pasta == "" {
		var err error
		if pasta, err = c.pastaNova("meu-site"); err != nil {
			return "", err
		}
		if err := os.WriteFile(filepath.Join(pasta, "index.html"), inicioHTML, 0o644); err != nil {
			return "", err
		}
	}
	return c.ligar(pasta)
}

// servirProgramar atende /comecar, /programar e /api/programa/*. Devolve
// false se o caminho não é dele.
func (c *central) servirProgramar(w http.ResponseWriter, r *http.Request) bool {
	switch r.URL.Path {
	case "/comecar", "/programar":
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("Content-Security-Policy", "default-src 'self'; style-src 'self' 'unsafe-inline'; script-src 'self' 'unsafe-inline'; img-src 'self' data:; frame-ancestors 'none'; base-uri 'none'; form-action 'none'")
		pag := comecarHTML
		if r.URL.Path == "/programar" {
			pag = programarHTML
		}
		io.WriteString(w, strings.Replace(pag, "{{token}}", c.token, 1))
		return true
	}
	if !strings.HasPrefix(r.URL.Path, "/api/programa/") && r.URL.Path != "/api/comecar-site" {
		return false
	}
	if r.Method != http.MethodPost || r.Header.Get("X-Central") != c.token {
		http.Error(w, "recarregue a página", http.StatusForbidden)
		return true
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	resp := func(v any, err error) {
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(map[string]string{"erro": err.Error()})
			return
		}
		json.NewEncoder(w).Encode(v)
	}
	var in struct {
		Nome, Codigo, Entrada, Pedido, Erro string
		Lua                                 bool
	}
	if err := json.NewDecoder(io.LimitReader(r.Body, maxPrograma+64<<10)).Decode(&in); err != nil && err != io.EOF {
		resp(nil, errors.New("pedido inválido"))
		return true
	}
	switch r.URL.Path {
	case "/api/comecar-site":
		u, err := c.siteNovo()
		resp(map[string]string{"url": u}, err)
	case "/api/programa/lista":
		resp(map[string]any{"programas": c.programas(), "pasta": c.pastaProgramas(), "ia": ialocal.Ver()}, nil)
	case "/api/programa/abrir":
		f, err := c.arquivoPrograma(in.Nome)
		if err != nil {
			resp(nil, err)
			return true
		}
		b, err := os.ReadFile(f)
		if err != nil {
			resp(nil, errors.New("programa não encontrado"))
			return true
		}
		resp(map[string]string{"nome": in.Nome, "codigo": string(b)}, nil)
	case "/api/programa/salvar":
		_, err := c.salvarPrograma(in.Nome, in.Codigo)
		resp(map[string]bool{"ok": err == nil}, err)
	case "/api/programa/rodar": // grava e roda
		f, err := c.salvarPrograma(in.Nome, in.Codigo)
		if err != nil {
			resp(nil, err)
			return true
		}
		resp(rodarPrograma(r.Context(), f, in.Entrada), nil)
	case "/api/programa/ia":
		ctx, cancel := context.WithTimeout(r.Context(), 10*time.Minute)
		defer cancel()
		resp(iaPrograma(ctx, pedidoIA{Pedido: in.Pedido, Codigo: in.Codigo, Erro: in.Erro, Lua: in.Lua}))
	case "/api/programa/pasta":
		os.MkdirAll(c.pastaProgramas(), 0o755)
		abrirPasta(c.pastaProgramas())
		resp(map[string]bool{"ok": true}, nil)
	default:
		http.NotFound(w, r)
	}
	return true
}
