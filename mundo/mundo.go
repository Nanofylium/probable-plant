// Pacote mundo faz o "passeio vivo" dos sites: a partir de uma foto, gera os
// quadros de quem anda por aquele lugar. Com GPU, usa o LingBot-World
// (Robbyant, Apache 2.0) servido pelo SGLang; sem GPU, simula o passeio
// aproximando e virando a câmera sobre a própria foto.
//
// O resultado fica numa pasta do site: mundo.json + uma pasta por caminho,
// com os quadros numerados (001.webp, 002.webp…). O site só mostra imagens:
// não precisa de GPU para quem visita.
package mundo

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// Caminho é um trajeto que o visitante pode fazer a partir da foto.
type Caminho struct {
	ID      string `json:"id"`
	Nome    string `json:"nome"`
	Quadros int    `json:"quadros"`
	Ext     string `json:"ext"`
}

// Mundo é o mundo.json que o site lê.
type Mundo struct {
	Nome     string    `json:"nome"`
	Titulo   string    `json:"titulo,omitempty"`
	Prompt   string    `json:"prompt,omitempty"`
	Largura  int       `json:"largura"`
	Altura   int       `json:"altura"`
	FPS      int       `json:"fps"`
	Fonte    string    `json:"fonte"` // lingbot-world, depth-anything, estimado ou simulado
	Caminhos []Caminho `json:"caminhos,omitempty"`
	// passeio 3D (sem GPU): a foto e a profundidade; o navegador monta a cena
	Tipo         string `json:"tipo,omitempty"` // "3d" ou vazio (quadros)
	Foto         string `json:"foto,omitempty"`
	Profundidade string `json:"profundidade,omitempty"`
}

// Os caminhos que o Nepenthes sabe pedir. Ações do LingBot-World:
// w a s d andam; i j k l olham (cima, esquerda, baixo, direita).
var Caminhos = []struct{ ID, Nome string }{
	{"frente", "Entrar"},
	{"esquerda", "Pela esquerda"},
	{"direita", "Pela direita"},
	{"redor", "Olhar ao redor"},
}

// CaminhoValido diz se o Nepenthes conhece o caminho.
func CaminhoValido(id string) bool {
	for _, c := range Caminhos {
		if c.ID == id {
			return true
		}
	}
	return false
}

// NomeDoCaminho é o rótulo do botão no site.
func NomeDoCaminho(id string) string {
	for _, c := range Caminhos {
		if c.ID == id {
			return c.Nome
		}
	}
	return id
}

// Acoes devolve a ação de cada quadro do caminho, no formato do LingBot-World.
func Acoes(id string, n int) [][]string {
	out := make([][]string, n)
	virar := n * 3 / 10
	for i := range out {
		switch id {
		case "esquerda":
			if i < virar {
				out[i] = []string{"j"}
			} else {
				out[i] = []string{"w"}
			}
		case "direita":
			if i < virar {
				out[i] = []string{"l"}
			} else {
				out[i] = []string{"w"}
			}
		case "redor":
			out[i] = []string{"l"}
		default:
			out[i] = []string{"w"}
		}
	}
	return out
}

var reNome = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{0,39}$`)

// NomeValido: o nome vira pasta e endereço, então só letras minúsculas, números e hífen.
func NomeValido(nome string) bool { return reNome.MatchString(nome) }

// Salvar grava os quadros e o mundo.json em pasta/nome.
func Salvar(pasta string, m *Mundo, quadros map[string][][]byte) error {
	if !NomeValido(m.Nome) {
		return fmt.Errorf("nome %q: use só letras minúsculas, números e hífen", m.Nome)
	}
	dir := filepath.Join(pasta, m.Nome)
	for i := range m.Caminhos {
		c := &m.Caminhos[i]
		qs := quadros[c.ID]
		if len(qs) == 0 {
			return fmt.Errorf("o caminho %s ficou sem quadros", c.ID)
		}
		cd := filepath.Join(dir, c.ID)
		if err := os.MkdirAll(cd, 0o755); err != nil {
			return err
		}
		// apaga quadros antigos de uma geração anterior
		if velhos, _ := filepath.Glob(filepath.Join(cd, "[0-9][0-9][0-9].*")); len(velhos) > 0 {
			for _, v := range velhos {
				os.Remove(v)
			}
		}
		for j, q := range qs {
			if err := os.WriteFile(filepath.Join(cd, fmt.Sprintf("%03d.%s", j+1, c.Ext)), q, 0o644); err != nil {
				return err
			}
		}
		c.Quadros = len(qs)
	}
	return salvarJSON(dir, m)
}

func salvarJSON(dir string, m *Mundo) error {
	j, _ := json.MarshalIndent(m, "", "  ")
	return os.WriteFile(filepath.Join(dir, "mundo.json"), j, 0o644)
}

// Abrir lê o mundo.json de pasta/nome.
func Abrir(pasta, nome string) (*Mundo, error) {
	if !NomeValido(nome) {
		return nil, errors.New("nome de mundo inválido")
	}
	b, err := os.ReadFile(filepath.Join(pasta, nome, "mundo.json"))
	if err != nil {
		return nil, err
	}
	var m Mundo
	if err := json.Unmarshal(b, &m); err != nil {
		return nil, err
	}
	return &m, nil
}

// Prompt monta o texto para o modelo a partir da descrição da pessoa.
func Prompt(descricao string) string {
	d := strings.TrimSpace(descricao)
	if d == "" {
		d = "A welcoming place"
	}
	return d + ". First-person walk through this place, smooth and stable camera motion, realistic lighting, consistent details."
}
