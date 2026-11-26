package mundo

import (
	"bufio"
	"bytes"
	"context"
	"crypto/sha1"
	"encoding/base64"
	"encoding/binary"
	"image"
	"image/color"
	"image/jpeg"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestMsgpackIdaEVolta(t *testing.T) {
	v := map[string]any{
		"type": "init", "n": 300, "neg": -5, "grande": int64(1) << 40, "f": 1.5, "ok": true, "nada": nil,
		"bytes": bytes.Repeat([]byte{7}, 300), "texto": strings.Repeat("á", 200),
		"lista": []any{"a", int64(1), []any{}}, "acoes": [][]string{{"w"}, {"j", "w"}},
	}
	b, err := Empacotar(v)
	if err != nil {
		t.Fatal(err)
	}
	got, err := Desempacotar(b)
	if err != nil {
		t.Fatal(err)
	}
	m := got.(map[string]any)
	if m["n"] != int64(300) || m["neg"] != int64(-5) || m["grande"] != int64(1)<<40 || m["f"] != 1.5 || m["ok"] != true || m["nada"] != nil {
		t.Fatalf("números: %v", m)
	}
	if !bytes.Equal(m["bytes"].([]byte), v["bytes"].([]byte)) || m["texto"] != v["texto"] {
		t.Fatal("bytes/texto")
	}
	if !reflect.DeepEqual(m["acoes"], []any{[]any{"w"}, []any{"j", "w"}}) {
		t.Fatalf("acoes: %#v", m["acoes"])
	}
	if _, err := Desempacotar(b[:len(b)-3]); err == nil {
		t.Fatal("mensagem cortada devia dar erro")
	}
}

func TestAcoes(t *testing.T) {
	a := Acoes("esquerda", 10)
	if a[0][0] != "j" || a[9][0] != "w" || len(a) != 10 {
		t.Fatalf("%v", a)
	}
	if Acoes("frente", 3)[2][0] != "w" || Acoes("redor", 2)[1][0] != "l" {
		t.Fatal("frente/redor")
	}
}

func fotoTeste(t *testing.T) []byte {
	img := image.NewRGBA(image.Rect(0, 0, 400, 300))
	for y := 0; y < 300; y++ {
		for x := 0; x < 400; x++ {
			img.Set(x, y, color.RGBA{uint8(x * 255 / 400), uint8(y * 255 / 300), 90, 255})
		}
	}
	var b bytes.Buffer
	jpeg.Encode(&b, img, nil)
	return b.Bytes()
}

func TestSimular(t *testing.T) {
	foto := fotoTeste(t)
	for _, c := range Caminhos {
		qs, err := Simular(foto, c.ID, 12, 320, 184)
		if err != nil || len(qs) != 12 {
			t.Fatalf("%s: %v", c.ID, err)
		}
		a, _ := jpeg.Decode(bytes.NewReader(qs[0]))
		z, _ := jpeg.Decode(bytes.NewReader(qs[11]))
		if a.Bounds().Dx() != 320 || a.Bounds().Dy() != 184 {
			t.Fatalf("tamanho %v", a.Bounds())
		}
		if bytes.Equal(qs[0], qs[11]) {
			t.Fatalf("%s: a câmera não se mexeu", c.ID)
		}
		_ = z
	}
	// indo pela esquerda, o fim mostra a parte esquerda (menos vermelho)
	qs, _ := Simular(foto, "esquerda", 6, 320, 184)
	fim, _ := jpeg.Decode(bytes.NewReader(qs[5]))
	r, _, _, _ := fim.At(160, 92).RGBA()
	if r>>8 > 120 {
		t.Fatalf("esquerda devia ir para o lado escuro, r=%d", r>>8)
	}
	if _, err := Simular([]byte("não é foto"), "frente", 3, 0, 0); err == nil {
		t.Fatal("foto inválida devia dar erro")
	}
}

// servidor falso com o protocolo do SGLang
func servidorFalso(t *testing.T, modo string, pedidos chan map[string]any) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/realtime_video/generate" {
			http.NotFound(w, r)
			return
		}
		h := sha1.Sum([]byte(r.Header.Get("Sec-WebSocket-Key") + "258EAFA5-E914-47DA-95CA-C5AB0DC85B11"))
		nc, rw, _ := w.(http.Hijacker).Hijack()
		defer nc.Close()
		rw.WriteString("HTTP/1.1 101 Switching Protocols\r\nUpgrade: websocket\r\nConnection: Upgrade\r\nSec-WebSocket-Accept: " + base64.StdEncoding.EncodeToString(h[:]) + "\r\n\r\n")
		rw.Flush()
		c := &conexao{c: nc, r: bufio.NewReader(rw)}
		b, err := c.Ler(1 << 20)
		if err != nil {
			t.Error(err)
			return
		}
		v, _ := Desempacotar(b)
		init := v.(map[string]any)
		pedidos <- init
		manda := func(op byte, p []byte) {
			f := []byte{0x80 | op}
			if len(p) < 126 {
				f = append(f, byte(len(p)))
			} else {
				f = binary.BigEndian.AppendUint16(append(f, 126), uint16(len(p)))
			}
			rw.Write(append(f, p...))
			rw.Flush()
		}
		if modo == "erro" {
			m, _ := Empacotar(map[string]any{"type": "error", "content": "another realtime session is already active"})
			manda(2, m)
			return
		}
		blocos := int(init["max_chunks"].(int64))
		por := int(init["num_frames"].(int64))
		for k := 0; k < blocos; k++ {
			var pl []byte
			var tams []any
			for q := 0; q < por; q++ {
				f := []byte("RIFFquadro" + string(rune('a'+k)) + string(rune('0'+q)))
				pl = append(pl, f...)
				tams = append(tams, int64(len(f)))
			}
			cab := map[string]any{"chunk_index": k, "content_type": "image/webp", "encoding": "webp", "payload_lengths": tams, "num_frames": por}
			if modo == "separado" {
				cab["type"] = "frame_batch_header"
				m, _ := Empacotar(cab)
				manda(2, m)
				manda(2, pl)
			} else {
				cab["type"] = "frame_batch"
				cab["payload"] = pl
				m, _ := Empacotar(cab)
				manda(2, m)
			}
			m, _ := Empacotar(map[string]any{"type": "chunk_stats", "chunk_total_ms": 10.0})
			manda(2, m)
		}
		manda(8, []byte{0x03, 0xe8})
		time.Sleep(50 * time.Millisecond)
	}))
}

func TestGerarComLingBot(t *testing.T) {
	for _, modo := range []string{"junto", "separado"} {
		pedidos := make(chan map[string]any, 1)
		s := servidorFalso(t, modo, pedidos)
		var ultimo int
		qs, ext, err := Gerar(context.Background(), Pedido{
			Servidor: s.URL, Foto: []byte("foto"), Prompt: "padaria", Caminho: "direita", Quadros: 20, PorBloco: 9,
			Progresso: func(f, total int) { ultimo = f },
		})
		s.Close()
		if err != nil {
			t.Fatalf("%s: %v", modo, err)
		}
		if len(qs) != 20 || ext != "webp" || string(qs[0]) != "RIFFquadroa0" || string(qs[19]) != "RIFFquadroc1" || ultimo != 20 {
			t.Fatalf("%s: %d quadros, %q, %q, %d", modo, len(qs), qs[0], qs[len(qs)-1], ultimo)
		}
		init := <-pedidos
		acoes := init["condition_inputs"].(map[string]any)["camera_actions"].([]any)
		if init["type"] != "init" || init["max_chunks"] != int64(3) || init["size"] != "832x480" || len(acoes) != 27 ||
			!bytes.Equal(init["first_frame"].([]byte), []byte("foto")) || acoes[0].([]any)[0] != "l" {
			t.Fatalf("init: %v", init)
		}
	}
}

func TestGerarErroDoServidor(t *testing.T) {
	pedidos := make(chan map[string]any, 1)
	s := servidorFalso(t, "erro", pedidos)
	defer s.Close()
	_, _, err := Gerar(context.Background(), Pedido{Servidor: strings.TrimPrefix(s.URL, "http://"), Foto: []byte("x")})
	if err == nil || !strings.Contains(err.Error(), "already active") {
		t.Fatalf("%v", err)
	}
	if _, _, err := Gerar(context.Background(), Pedido{Servidor: "127.0.0.1:1", Foto: []byte("x")}); err == nil {
		t.Fatal("servidor fora do ar devia dar erro")
	}
}

func TestSalvarEAbrir(t *testing.T) {
	dir := t.TempDir()
	m := &Mundo{Nome: "loja", Largura: 832, Altura: 480, FPS: 16, Fonte: "simulado",
		Caminhos: []Caminho{{ID: "frente", Nome: "Entrar", Ext: "jpg"}}}
	if err := Salvar(dir, m, map[string][][]byte{"frente": {[]byte("a"), []byte("b")}}); err != nil {
		t.Fatal(err)
	}
	m2, err := Abrir(dir, "loja")
	if err != nil || m2.Caminhos[0].Quadros != 2 {
		t.Fatalf("%v %+v", err, m2)
	}
	if err := Salvar(dir, &Mundo{Nome: "../x"}, nil); err == nil {
		t.Fatal("nome com .. devia dar erro")
	}
	if enderecoWS("gpu.exemplo:30000") != "ws://gpu.exemplo:30000/v1/realtime_video/generate" ||
		enderecoWS("https://gpu.exemplo/") != "wss://gpu.exemplo/v1/realtime_video/generate" {
		t.Fatal(enderecoWS("gpu.exemplo:30000"))
	}
}

func TestPasseio3DSemModelo(t *testing.T) {
	foto := fotoTeste(t)
	mapa, err := EstimarProfundidade(foto)
	if err != nil {
		t.Fatal(err)
	}
	if mapa.Disp[0] >= mapa.Disp[len(mapa.Disp)-1] {
		t.Fatal("embaixo devia estar mais perto que em cima")
	}
	p, err := mapa.PNG()
	if err != nil {
		t.Fatal(err)
	}
	img, _, err := image.Decode(bytes.NewReader(p))
	if err != nil || img.Bounds().Dx() != mapa.Largura {
		t.Fatalf("png: %v", err)
	}
	// 16 bits em vermelho (alto) e verde (baixo)
	r, g, _, _ := img.At(mapa.Largura/2, mapa.Altura-1).RGBA()
	v := float32((r>>8)*256+(g>>8)) / 65535
	if d := v - mapa.Disp[len(mapa.Disp)-mapa.Largura/2-1+0]; d > .01 || d < -.01 {
		t.Fatalf("profundidade no PNG %v, no mapa %v", v, mapa.Disp[len(mapa.Disp)-mapa.Largura/2-1])
	}
	dir := t.TempDir()
	m := &Mundo{Nome: "sala", Fonte: "estimado"}
	if err := Salvar3D(dir, m, foto, mapa); err != nil {
		t.Fatal(err)
	}
	m2, err := Abrir(dir, "sala")
	if err != nil || m2.Tipo != "3d" || m2.Foto != "foto.jpg" || m2.Largura != 400 || m2.Altura != 300 {
		t.Fatalf("%v %+v", err, m2)
	}
}

func TestNormalizar(t *testing.T) {
	m := normalizar(&Mapa{Largura: 4, Altura: 1, Disp: []float32{2, 4, 6, 8}})
	if m.Disp[0] != 0 || m.Disp[3] != 1 || m.Disp[1] <= 0 || m.Disp[1] >= m.Disp[2] {
		t.Fatalf("%v", m.Disp)
	}
}
