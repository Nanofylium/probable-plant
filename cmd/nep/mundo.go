package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/Nanofylium/probable-plant/mundo"
	"github.com/Nanofylium/probable-plant/rede"
)

// mundo gera o passeio vivo de um site a partir de uma foto:
//
//	nep mundo fachada.jpg "padaria artesanal com balcão de madeira" --nome padaria
//	nep mundo sala.jpg "apartamento claro" --servidor gpu.exemplo:30000
//
// Sem GPU (padrão), faz o passeio 3D: profundidade da foto calculada no
// processador e cena montada no navegador. Com --servidor (ou
// NEP_MUNDO_SERVIDOR), usa o LingBot-World servido pelo SGLang. --simular faz
// quadros 2D. Grava em mundos/NOME/.
func mundoCmd(args []string) int {
	var pos []string
	nome, servidor, titulo, pasta := "", os.Getenv("NEP_MUNDO_SERVIDOR"), "", "mundos"
	caminhos := []string{"frente", "esquerda", "direita"}
	quadros, semente, simular, semModelo, lado := 0, int64(0), false, false, 518
	valor := func(i *int, a string) string {
		if k := strings.IndexByte(a, '='); k > 0 {
			return a[k+1:]
		}
		if *i+1 < len(args) {
			*i++
			return args[*i]
		}
		return ""
	}
	for i := 0; i < len(args); i++ {
		a := args[i]
		chave := a
		if k := strings.IndexByte(a, '='); k > 0 {
			chave = a[:k]
		}
		switch chave {
		case "--nome":
			nome = valor(&i, a)
		case "--servidor":
			servidor = valor(&i, a)
		case "--titulo":
			titulo = valor(&i, a)
		case "--pasta":
			pasta = valor(&i, a)
		case "--caminhos":
			caminhos = strings.Split(valor(&i, a), ",")
		case "--quadros":
			quadros, _ = strconv.Atoi(valor(&i, a))
		case "--semente":
			semente, _ = strconv.ParseInt(valor(&i, a), 10, 64)
		case "--ajuda-gpu":
			fmt.Print(ajudaGPU)
			return 0
		case "--simular", "--quadros-2d":
			simular = true
		case "--sem-modelo":
			semModelo = true
		case "--lado":
			lado, _ = strconv.Atoi(valor(&i, a))
		default:
			if strings.HasPrefix(a, "--") {
				fmt.Fprintln(os.Stderr, "opção desconhecida:", a)
				return 2
			}
			pos = append(pos, a)
		}
	}
	if len(pos) == 0 {
		fmt.Fprintln(os.Stderr, `uso: nep mundo foto.jpg "descrição do lugar" [--nome loja] [--servidor host:porta] [--sem-modelo] [--simular]
          [--caminhos frente,esquerda,direita,redor] [--quadros N] [--titulo "Conheça a loja"]`)
		return 2
	}
	foto, err := os.ReadFile(pos[0])
	if err != nil {
		fmt.Fprintln(os.Stderr, "não consegui ler a foto:", err)
		return 1
	}
	descricao := strings.Join(pos[1:], " ")
	if nome == "" {
		nome = slugMundo(strings.TrimSuffix(filepath.Base(pos[0]), filepath.Ext(pos[0])))
	}
	if !mundo.NomeValido(nome) {
		fmt.Fprintf(os.Stderr, "nome %q: use só letras minúsculas, números e hífen (--nome loja)\n", nome)
		return 2
	}
	for _, c := range caminhos {
		if !mundo.CaminhoValido(c) {
			fmt.Fprintf(os.Stderr, "caminho %q não existe; use frente, esquerda, direita ou redor\n", c)
			return 2
		}
	}
	if simular {
		servidor = ""
	}
	if servidor == "" && !simular {
		return mundo3D(foto, nome, titulo, descricao, pasta, semModelo, lado)
	}
	ctx, parar := signal.NotifyContext(context.Background(), os.Interrupt)
	defer parar()
	m := &mundo.Mundo{Nome: nome, Titulo: titulo, Prompt: mundo.Prompt(descricao), Largura: 832, Altura: 480, FPS: 16, Fonte: "lingbot-world"}
	gerados := map[string][][]byte{}
	if servidor == "" {
		m.Fonte, m.FPS = "simulado", 20
		if quadros <= 0 {
			quadros = 48
		}
		fmt.Println("Passeio em quadros simulados a partir da foto.")
	} else if quadros <= 0 {
		quadros = 81
	}
	for _, c := range caminhos {
		var qs [][]byte
		ext := "jpg"
		if servidor == "" {
			qs, err = mundo.Simular(foto, c, quadros, m.Largura, m.Altura)
		} else {
			fmt.Printf("LingBot-World: caminho %q…\n", c)
			qs, ext, err = mundo.Gerar(ctx, mundo.Pedido{
				Servidor: servidor, Foto: foto, Prompt: m.Prompt, Caminho: c, Quadros: quadros, Semente: semente,
				Largura: m.Largura, Altura: m.Altura, FPS: m.FPS,
				Progresso: func(f, t int) { fmt.Printf("\r  %d de %d quadros", f, t) },
			})
			fmt.Println()
		}
		if err != nil {
			fmt.Fprintln(os.Stderr, "erro:", err)
			return 1
		}
		gerados[c] = qs
		m.Caminhos = append(m.Caminhos, mundo.Caminho{ID: c, Nome: mundo.NomeDoCaminho(c), Ext: ext})
	}
	if err := mundo.Salvar(pasta, m, gerados); err != nil {
		fmt.Fprintln(os.Stderr, "erro:", err)
		return 1
	}
	fmt.Printf("Pronto: %s\n", filepath.Join(pasta, nome))
	fmt.Printf("No site (o .nep.html na pasta acima de %s), escreva:\n  {{ mundo(%q) }}\n", pasta, nome)
	fmt.Printf("ou, para andar rolando a página:\n  {{ mundo(%q, {rolar: true, titulo: \"Entre e conheça\"}) }}\n", nome)
	return 0
}

// mundo3D: passeio 3D sem GPU. A profundidade sai do Depth Anything V2 Small
// rodando no processador; sem o modelo, usa uma estimativa simples.
func mundo3D(foto []byte, nome, titulo, descricao, pasta string, semModelo bool, lado int) int {
	m := &mundo.Mundo{Nome: nome, Titulo: titulo, Prompt: descricao, Fonte: "depth-anything"}
	var mapa *mundo.Mapa
	var err error
	if !semModelo && !mundo.ModeloProfBaixado() {
		fmt.Printf("Baixando o %s (%d MB, só desta vez) para %s…\n", mundo.ModeloProfNome, mundo.ModeloProfMB, filepath.Dir(mundo.CaminhoModeloProf()))
		ultimo := -1
		err = mundo.BaixarModeloProf(func(f, t int64) {
			if p := int(f * 100 / t); p != ultimo {
				ultimo = p
				fmt.Printf("\r  %3d%%", p)
			}
		})
		fmt.Println()
		if err != nil {
			fmt.Fprintln(os.Stderr, "aviso:", err)
			fmt.Fprintln(os.Stderr, "Sigo com a profundidade estimada (sem o modelo). Rode de novo com internet para a versão completa.")
			semModelo = true
		}
	}
	if !semModelo {
		fmt.Printf("Calculando a profundidade da foto com o %s, no processador…\n", mundo.ModeloProfNome)
		inicio := time.Now()
		rd, e := rede.Abrir(mundo.CaminhoModeloProf())
		if e == nil {
			mapa, e = mundo.CalcularProfundidade(rd, foto, lado)
		}
		if e != nil {
			fmt.Fprintln(os.Stderr, "aviso:", e, "(sigo com a profundidade estimada)")
			semModelo = true
		} else {
			fmt.Printf("  pronto em %.1f s\n", time.Since(inicio).Seconds())
		}
	}
	if semModelo {
		m.Fonte = "estimado"
		if mapa, err = mundo.EstimarProfundidade(foto); err != nil {
			fmt.Fprintln(os.Stderr, "erro:", err)
			return 1
		}
	}
	if err := mundo.Salvar3D(pasta, m, foto, mapa); err != nil {
		fmt.Fprintln(os.Stderr, "erro:", err)
		return 1
	}
	fmt.Printf("Pronto: passeio 3D em %s (foto + profundidade; o navegador de quem visita monta a cena)\n", filepath.Join(pasta, nome))
	fmt.Printf("No site (o .nep.html na pasta acima de %s), escreva:\n  {{ mundo(%q) }}\n", pasta, nome)
	fmt.Printf("ou, para andar rolando a página:\n  {{ mundo(%q, {rolar: true, titulo: \"Entre e conheça\"}) }}\n", nome)
	return 0
}

func slugMundo(s string) string {
	var b strings.Builder
	for _, r := range strings.ToLower(s) {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			b.WriteRune(r)
		case strings.ContainsRune("áàâã", r):
			b.WriteByte('a')
		case strings.ContainsRune("éê", r):
			b.WriteByte('e')
		case r == 'í':
			b.WriteByte('i')
		case strings.ContainsRune("óôõ", r):
			b.WriteByte('o')
		case r == 'ú':
			b.WriteByte('u')
		case r == 'ç':
			b.WriteByte('c')
		default:
			if b.Len() > 0 && !strings.HasSuffix(b.String(), "-") {
				b.WriteByte('-')
			}
		}
	}
	out := strings.Trim(b.String(), "-")
	if len(out) > 40 {
		out = strings.Trim(out[:40], "-")
	}
	if out == "" {
		out = "passeio"
	}
	return out
}

const ajudaGPU = `Passeio de verdade com o LingBot-World (Robbyant, Apache 2.0)

O modelo precisa de GPU NVIDIA grande. Num servidor com GPU (alugado por hora,
por exemplo), rode o SGLang:

  pip install "sglang[diffusion]"
  sglang serve --model-path robbyant/lingbot-world-fast-diffusers \
    --pipeline-class-name LingBotWorldCausalDMDPipeline --num-gpus 4 --ulysses-degree 4

Depois, no seu computador:

  nep mundo foto.jpg "descrição do lugar" --nome loja --servidor IP-DO-SERVIDOR:30000

O passeio é gerado uma vez só e fica no site como imagens: quem visita não
precisa de GPU. Pode desligar o servidor depois.
`
