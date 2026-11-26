package ialocal

import (
	"archive/zip"
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// O motor embutido vai para a pasta da IA sem internet e tira o pacote antigo.
func TestMotorEmbutido(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("NEP_IA_LOCAL", dir)
	velho := filepath.Join(dir, "llama-"+Versao)
	os.MkdirAll(velho, 0o755)
	var b bytes.Buffer
	z := zip.NewWriter(&b)
	nome := "llama-server"
	if runtime.GOOS == "windows" {
		nome += ".exe"
	}
	for _, n := range []string{nome, "ggml-cpu-haswell.dll", "LEIA-ME-MOTOR.txt"} {
		w, _ := z.Create(n)
		w.Write([]byte("x"))
	}
	z.Close()
	antes := motorEmbutido
	motorEmbutido = b.Bytes()
	defer func() { motorEmbutido = antes }()
	if !Suportado() || !Embutido() {
		t.Fatal("embutido não conta como suportado")
	}
	if err := prepararMotor(); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dir, "motor-"+Versao+"-nep", "ggml-cpu-haswell.dll")); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(velho); !os.IsNotExist(err) {
		t.Fatal("o pacote antigo (que pedia o Visual C++) ficou")
	}
	if _, err := os.Stat(filepath.Join(dir, "motor-embutido.zip")); !os.IsNotExist(err) {
		t.Fatal("o .zip temporário ficou")
	}
}

// Quando o motor fecha, a mensagem traz o motivo e o fim do registro.
func TestPorQueParou(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip()
	}
	dir := t.TempDir()
	t.Setenv("NEP_IA_LOCAL", dir)
	os.WriteFile(filepath.Join(dir, "llama-server.log"), []byte("carregando\nerro: modelo quebrado\n"), 0o644)
	cmd := exec.Command("sh", "-c", "exit 3")
	cmd.Run()
	err := porQueParou(cmd)
	if !strings.Contains(err.Error(), "código de saída 3") || !strings.Contains(err.Error(), "modelo quebrado") {
		t.Fatal(err)
	}
}
