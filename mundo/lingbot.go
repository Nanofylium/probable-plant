package mundo

// Cliente do LingBot-World servido pelo SGLang (sglang serve --model-path
// robbyant/lingbot-world-fast-diffusers). Protocolo: WebSocket em
// /v1/realtime_video/generate com mensagens MessagePack; a primeira é "init"
// com a foto, o texto e as ações da câmera; o servidor responde com
// "frame_batch" (quadros em WebP) até completar max_chunks e fecha.

import (
	"context"
	"errors"
	"fmt"
	"time"
)

// Pedido descreve uma geração.
type Pedido struct {
	Servidor  string // host:porta, ws://… ou wss://…
	Foto      []byte // JPEG ou PNG
	Prompt    string
	Caminho   string // frente, esquerda, direita, redor
	Quadros   int    // total de quadros do caminho (padrão 81)
	PorBloco  int    // quadros por bloco do modelo (padrão 9)
	Largura   int    // padrão 832
	Altura    int    // padrão 480
	FPS       int    // padrão 16
	Semente   int64
	Progresso func(feitos, total int)
}

const limiteMensagem = 64 << 20

// Gerar pede ao LingBot-World os quadros de um caminho e devolve cada quadro
// já codificado (WebP ou JPEG) e a extensão.
func Gerar(ctx context.Context, p Pedido) ([][]byte, string, error) {
	if p.Quadros <= 0 {
		p.Quadros = 81
	}
	if p.PorBloco <= 0 {
		p.PorBloco = 9
	}
	if p.Largura <= 0 || p.Altura <= 0 {
		p.Largura, p.Altura = 832, 480
	}
	if p.FPS <= 0 {
		p.FPS = 16
	}
	if len(p.Foto) == 0 {
		return nil, "", errors.New("falta a foto")
	}
	if _, ok := ctx.Deadline(); !ok {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, 30*time.Minute)
		defer cancel()
	}
	ws, err := discar(ctx, enderecoWS(p.Servidor))
	if err != nil {
		return nil, "", fmt.Errorf("não consegui falar com o servidor do LingBot-World: %w", err)
	}
	defer ws.Fechar()
	go func() { <-ctx.Done(); ws.c.Close() }()

	blocos := (p.Quadros + p.PorBloco - 1) / p.PorBloco
	init := map[string]any{
		"type":                   "init",
		"prompt":                 p.Prompt,
		"first_frame":            p.Foto,
		"size":                   fmt.Sprintf("%dx%d", p.Largura, p.Altura),
		"fps":                    p.FPS,
		"num_frames":             p.PorBloco,
		"num_inference_steps":    4,
		"guidance_scale":         1.0,
		"max_chunks":             blocos,
		"realtime_output_format": "webp",
		"output_compression":     85,
		"condition_inputs":       map[string]any{"camera_actions": Acoes(p.Caminho, blocos*p.PorBloco)},
	}
	if p.Semente != 0 {
		init["seed"] = p.Semente
	}
	msg, err := Empacotar(init)
	if err != nil {
		return nil, "", err
	}
	if err := ws.Binario(msg); err != nil {
		return nil, "", err
	}
	var quadros [][]byte
	ext := "webp"
	var cabecalho map[string]any // frame_batch_header: o próximo envio é só o conteúdo
	for len(quadros) < p.Quadros {
		b, err := ws.Ler(limiteMensagem)
		if err != nil {
			var f *ErrFechado
			if errors.As(err, &f) && f.Codigo == 1000 && len(quadros) > 0 {
				break // o servidor terminou os blocos
			}
			if ctx.Err() != nil {
				return nil, "", fmt.Errorf("o LingBot-World demorou demais: %w", ctx.Err())
			}
			return nil, "", fmt.Errorf("conexão com o LingBot-World caiu: %w", err)
		}
		if cabecalho != nil {
			qs, e, err := separar(cabecalho, b)
			if err != nil {
				return nil, "", err
			}
			quadros, ext, cabecalho = append(quadros, qs...), e, nil
			if p.Progresso != nil {
				p.Progresso(min(len(quadros), p.Quadros), p.Quadros)
			}
			continue
		}
		v, err := Desempacotar(b)
		if err != nil {
			return nil, "", err
		}
		m, _ := v.(map[string]any)
		switch m["type"] {
		case "error":
			return nil, "", fmt.Errorf("o LingBot-World respondeu: %v", m["content"])
		case "frame_batch_header":
			cabecalho = m
		case "frame_batch":
			pl, _ := m["payload"].([]byte)
			qs, e, err := separar(m, pl)
			if err != nil {
				return nil, "", err
			}
			quadros, ext = append(quadros, qs...), e
			if p.Progresso != nil {
				p.Progresso(min(len(quadros), p.Quadros), p.Quadros)
			}
		}
	}
	if len(quadros) == 0 {
		return nil, "", errors.New("o LingBot-World não mandou nenhum quadro")
	}
	if len(quadros) > p.Quadros {
		quadros = quadros[:p.Quadros]
	}
	return quadros, ext, nil
}

// separar corta o conteúdo de um frame_batch em quadros (payload_lengths).
func separar(cab map[string]any, payload []byte) ([][]byte, string, error) {
	ext := "webp"
	switch fmt.Sprint(cab["encoding"]) {
	case "jpeg", "jpg":
		ext = "jpg"
	case "webp":
	default:
		if ct := fmt.Sprint(cab["content_type"]); ct == "image/jpeg" {
			ext = "jpg"
		} else if ct != "image/webp" {
			return nil, "", fmt.Errorf("o servidor mandou quadros em %q; peça webp ou jpeg", ct)
		}
	}
	tams, _ := cab["payload_lengths"].([]any)
	if len(tams) == 0 {
		return [][]byte{payload}, ext, nil
	}
	var out [][]byte
	i := 0
	for _, t := range tams {
		n, _ := t.(int64)
		if n < 0 || i+int(n) > len(payload) {
			return nil, "", errors.New("quadros cortados na resposta do LingBot-World")
		}
		out = append(out, payload[i:i+int(n)])
		i += int(n)
	}
	return out, ext, nil
}
