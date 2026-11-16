package ialocal

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

func shaDe(b []byte) string { h := sha256.Sum256(b); return hex.EncodeToString(h[:]) }

// Baixa da primeira fonte que der certo, retoma o .parte e recusa arquivo
// com SHA-256 diferente.
func TestBuscar(t *testing.T) {
	dados := bytes.Repeat([]byte("granite"), 5000)
	ruim := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.Write([]byte("adulterado")) }))
	defer ruim.Close()
	bom := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.ServeContent(w, r, "m.gguf", testTime, bytes.NewReader(dados))
	}))
	defer bom.Close()
	velho := fontes
	defer func() { fontes = velho }()
	fontes = func(n string) []string { return []string{ruim.URL + "/" + n, bom.URL + "/" + n} }

	dir := t.TempDir()
	dest := filepath.Join(dir, "m.gguf")
	os.WriteFile(dest+".parte", dados[:1000], 0o644) // download interrompido
	var total int64
	if err := buscar(arquivo{"m.gguf", shaDe(dados)}, dest, func(n int64) { total += n }); err != nil {
		t.Fatal(err)
	}
	if b, _ := os.ReadFile(dest); !bytes.Equal(b, dados) {
		t.Fatal("arquivo diferente")
	}
	if err := buscar(arquivo{"x.gguf", strings.Repeat("0", 64)}, filepath.Join(dir, "x"), func(int64) {}); err == nil {
		t.Fatal("aceitou SHA-256 errado")
	}
}

func tgz(t *testing.T, add func(*tar.Writer)) string {
	var b bytes.Buffer
	gz := gzip.NewWriter(&b)
	tw := tar.NewWriter(gz)
	add(tw)
	tw.Close()
	gz.Close()
	f := filepath.Join(t.TempDir(), "p.tar.gz")
	os.WriteFile(f, b.Bytes(), 0o644)
	return f
}

func TestExtrair(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("links")
	}
	t.Setenv("NEP_IA_LOCAL", t.TempDir())
	arq := func(tw *tar.Writer, nome, conteudo string, modo int64) {
		tw.WriteHeader(&tar.Header{Name: nome, Mode: modo, Size: int64(len(conteudo)), Typeflag: tar.TypeReg})
		tw.Write([]byte(conteudo))
	}
	pac := tgz(t, func(tw *tar.Writer) {
		arq(tw, "llama-"+Versao+"/llama-server", "#!/bin/sh\n", 0o755)
		arq(tw, "llama-"+Versao+"/libggml.so.1.0.0", "lib", 0o644)
		tw.WriteHeader(&tar.Header{Name: "llama-" + Versao + "/libggml.so.1", Linkname: "libggml.so.1.0.0", Typeflag: tar.TypeSymlink})
	})
	if err := extrair(pac, Pasta()); err != nil {
		t.Fatal(err)
	}
	if fi, err := os.Stat(exe()); err != nil || fi.Mode()&0o100 == 0 {
		t.Fatalf("llama-server: %v %v", fi, err)
	}
	if b, err := os.ReadFile(filepath.Join(dirMotor(), "libggml.so.1")); err != nil || string(b) != "lib" {
		t.Fatalf("link: %q %v", b, err)
	}
	for _, mau := range []func(*tar.Writer){
		func(tw *tar.Writer) { arq(tw, "llama-"+Versao+"/../../fora", "x", 0o644) },
		func(tw *tar.Writer) {
			tw.WriteHeader(&tar.Header{Name: "llama-" + Versao + "/l", Linkname: "/etc/passwd", Typeflag: tar.TypeSymlink})
		},
	} {
		if err := extrair(tgz(t, mau), Pasta()); err == nil {
			t.Error("aceitou pacote que escreve fora da pasta")
		}
	}
}

func TestEstado(t *testing.T) {
	t.Setenv("NEP_IA_LOCAL", t.TempDir())
	if e := Ver(); Suportado() && e.Fase != "ausente" {
		t.Fatalf("estado: %+v", e)
	}
}

var testTime = time.Unix(1700000000, 0)
