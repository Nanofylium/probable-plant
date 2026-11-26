package rede

import (
	"encoding/base64"
	"encoding/json"
	"math"
	"os"
	"reflect"
	"testing"
)

// Os casos em testdata/casos.json foram gerados por testdata/gerar_casos.py:
// modelos ONNX pequenos (todas as operações suportadas) e a resposta do
// onnxruntime oficial para a mesma entrada.
func TestIgualAoOnnxruntime(t *testing.T) {
	b, err := os.ReadFile("testdata/casos.json")
	if err != nil {
		t.Fatal(err)
	}
	var casos []struct {
		Nome           string
		Modelo         string
		Forma, Forma_y []int
		X, Y           []float32
	}
	if err := json.Unmarshal(b, &casos); err != nil {
		t.Fatal(err)
	}
	for _, c := range casos {
		mb, _ := base64.StdEncoding.DecodeString(c.Modelo)
		m, err := Carregar(mb)
		if err != nil {
			t.Fatalf("%s: %v", c.Nome, err)
		}
		x := NovoTensor(c.Forma...)
		copy(x.F, c.X)
		y, err := m.Rodar(x)
		if err != nil {
			t.Fatalf("%s: %v", c.Nome, err)
		}
		y = paraFloat(y)
		if !reflect.DeepEqual(y.Forma, c.Forma_y) {
			t.Fatalf("%s: forma %v, esperada %v", c.Nome, y.Forma, c.Forma_y)
		}
		for i, v := range c.Y {
			if math.Abs(float64(y.F[i]-v)) > 1e-4*(1+math.Abs(float64(v))) {
				t.Fatalf("%s: posição %d deu %v, o onnxruntime dá %v", c.Nome, i, y.F[i], v)
			}
		}
	}
}

func TestOperacaoDesconhecida(t *testing.T) {
	if _, err := Carregar([]byte{0x3a, 0x0a, 0x0a, 0x08, 0x12, 0x01, 'y', 0x22, 0x03, 'F', 'o', 'o'}); err == nil {
		t.Fatal("operação desconhecida devia dar erro")
	}
	if _, err := Carregar([]byte{0xff, 0xff}); err == nil {
		t.Fatal("arquivo quebrado devia dar erro")
	}
}

func TestMMGrande(t *testing.T) {
	// confere o caminho paralelo da multiplicação (linhas e colunas)
	for _, d := range [][3]int{{64, 33, 70}, {1, 300, 900}} {
		M, K, N := d[0], d[1], d[2]
		a, b := make([]float32, M*K), make([]float32, K*N)
		for i := range a {
			a[i] = float32(i%7) - 3
		}
		for i := range b {
			b[i] = float32(i%5) - 2
		}
		c := make([]float32, M*N)
		mm(a, b, c, M, K, N)
		for i := 0; i < M; i += 13 {
			for j := 0; j < N; j += 17 {
				var s float32
				for k := 0; k < K; k++ {
					s += a[i*K+k] * b[k*N+j]
				}
				if c[i*N+j] != s {
					t.Fatalf("%v: c[%d,%d]=%v, esperado %v", d, i, j, c[i*N+j], s)
				}
			}
		}
	}
}

// Com o modelo de verdade (NEP_PROFUNDIDADE=caminho do .onnx), roda uma imagem.
func TestModeloDeProfundidade(t *testing.T) {
	c := os.Getenv("NEP_PROFUNDIDADE")
	if c == "" {
		t.Skip("defina NEP_PROFUNDIDADE para testar com o Depth Anything V2 Small")
	}
	m, err := Abrir(c)
	if err != nil {
		t.Fatal(err)
	}
	x := NovoTensor(1, 3, 140, 196)
	for i := range x.F {
		x.F[i] = float32(i%97)/48 - 1
	}
	y, err := m.Rodar(x)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(y.Forma, []int{1, 140, 196}) {
		t.Fatalf("forma %v", y.Forma)
	}
}
