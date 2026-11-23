package main

import (
	"bytes"
	"image"
	"image/jpeg"
	"os"
	"path/filepath"
	"testing"

	"github.com/Nanofylium/probable-plant/mundo"
)

func TestNepMundoSimulado(t *testing.T) {
	dir := t.TempDir()
	var b bytes.Buffer
	jpeg.Encode(&b, image.NewRGBA(image.Rect(0, 0, 300, 200)), nil)
	foto := filepath.Join(dir, "Fachada da Padaria.jpg")
	os.WriteFile(foto, b.Bytes(), 0o644)
	pasta := filepath.Join(dir, "mundos")
	if c := mundoCmd([]string{foto, "padaria", "--simular", "--quadros", "5", "--caminhos", "frente,redor", "--pasta", pasta}); c != 0 {
		t.Fatalf("saída %d", c)
	}
	m, err := mundo.Abrir(pasta, "fachada-da-padaria")
	if err != nil || m.Fonte != "simulado" || len(m.Caminhos) != 2 || m.Caminhos[1].Quadros != 5 || m.Caminhos[1].Nome != "Olhar ao redor" {
		t.Fatalf("%v %+v", err, m)
	}
	if _, err := os.Stat(filepath.Join(pasta, "fachada-da-padaria", "redor", "005.jpg")); err != nil {
		t.Fatal(err)
	}
	if c := mundoCmd([]string{foto, "--caminhos", "voar", "--simular"}); c != 2 {
		t.Fatal("caminho desconhecido devia recusar")
	}
	if slugMundo("Café São João!") != "cafe-sao-joao" {
		t.Fatal(slugMundo("Café São João!"))
	}
}
