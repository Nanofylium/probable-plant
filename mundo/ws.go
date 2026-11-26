package mundo

// Cliente WebSocket mínimo (RFC 6455): o suficiente para falar com o
// servidor de tempo real do SGLang, sem dependências.

import (
	"bufio"
	"context"
	"crypto/rand"
	"crypto/sha1"
	"crypto/tls"
	"encoding/base64"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"
)

type conexao struct {
	c net.Conn
	r *bufio.Reader
}

// ErrFechado indica que o servidor fechou a conexão; Codigo diz por quê.
type ErrFechado struct {
	Codigo int
	Motivo string
}

func (e *ErrFechado) Error() string {
	return fmt.Sprintf("websocket fechado (%d %s)", e.Codigo, e.Motivo)
}

func discar(ctx context.Context, endereco string) (*conexao, error) {
	u, err := url.Parse(endereco)
	if err != nil {
		return nil, err
	}
	seguro := false
	switch u.Scheme {
	case "ws", "http":
	case "wss", "https":
		seguro = true
	default:
		return nil, fmt.Errorf("endereço %q: use ws:// ou wss://", endereco)
	}
	host := u.Host
	if u.Port() == "" {
		if seguro {
			host += ":443"
		} else {
			host += ":80"
		}
	}
	var d net.Dialer
	nc, err := d.DialContext(ctx, "tcp", host)
	if err != nil {
		return nil, err
	}
	if seguro {
		tc := tls.Client(nc, &tls.Config{ServerName: u.Hostname()})
		if err := tc.HandshakeContext(ctx); err != nil {
			nc.Close()
			return nil, err
		}
		nc = tc
	}
	if dl, ok := ctx.Deadline(); ok {
		nc.SetDeadline(dl)
	}
	k := make([]byte, 16)
	rand.Read(k)
	chave := base64.StdEncoding.EncodeToString(k)
	caminho := u.RequestURI()
	fmt.Fprintf(nc, "GET %s HTTP/1.1\r\nHost: %s\r\nUpgrade: websocket\r\nConnection: Upgrade\r\nSec-WebSocket-Key: %s\r\nSec-WebSocket-Version: 13\r\nUser-Agent: nepenthes\r\n\r\n", caminho, u.Host, chave)
	br := bufio.NewReaderSize(nc, 64<<10)
	resp, err := http.ReadResponse(br, &http.Request{Method: "GET"})
	if err != nil {
		nc.Close()
		return nil, err
	}
	if resp.StatusCode != http.StatusSwitchingProtocols {
		nc.Close()
		return nil, fmt.Errorf("o servidor recusou o websocket: %s", resp.Status)
	}
	h := sha1.Sum([]byte(chave + "258EAFA5-E914-47DA-95CA-C5AB0DC85B11"))
	if resp.Header.Get("Sec-WebSocket-Accept") != base64.StdEncoding.EncodeToString(h[:]) {
		nc.Close()
		return nil, errors.New("resposta do websocket inválida")
	}
	return &conexao{c: nc, r: br}, nil
}

// escrever manda um quadro (o cliente sempre mascara).
func (w *conexao) escrever(op byte, p []byte) error {
	b := []byte{0x80 | op}
	n := len(p)
	switch {
	case n < 126:
		b = append(b, 0x80|byte(n))
	case n < 1<<16:
		b = binary.BigEndian.AppendUint16(append(b, 0x80|126), uint16(n))
	default:
		b = binary.BigEndian.AppendUint64(append(b, 0x80|127), uint64(n))
	}
	m := make([]byte, 4)
	rand.Read(m)
	b = append(b, m...)
	for i, c := range p {
		b = append(b, c^m[i&3])
	}
	_, err := w.c.Write(b)
	return err
}

func (w *conexao) Binario(p []byte) error { return w.escrever(2, p) }

// Ler devolve a próxima mensagem inteira (texto ou binária).
func (w *conexao) Ler(limite int) ([]byte, error) {
	var msg []byte
	for {
		var h [2]byte
		if _, err := io.ReadFull(w.r, h[:]); err != nil {
			return nil, err
		}
		fim, op := h[0]&0x80 != 0, h[0]&0x0f
		n := uint64(h[1] & 0x7f)
		switch n {
		case 126:
			var e [2]byte
			if _, err := io.ReadFull(w.r, e[:]); err != nil {
				return nil, err
			}
			n = uint64(binary.BigEndian.Uint16(e[:]))
		case 127:
			var e [8]byte
			if _, err := io.ReadFull(w.r, e[:]); err != nil {
				return nil, err
			}
			n = binary.BigEndian.Uint64(e[:])
		}
		var mascara []byte
		if h[1]&0x80 != 0 {
			mascara = make([]byte, 4)
			if _, err := io.ReadFull(w.r, mascara); err != nil {
				return nil, err
			}
		}
		if n > uint64(limite) || uint64(len(msg))+n > uint64(limite) {
			return nil, fmt.Errorf("mensagem grande demais (%d bytes)", n)
		}
		p := make([]byte, n)
		if _, err := io.ReadFull(w.r, p); err != nil {
			return nil, err
		}
		if mascara != nil {
			for i := range p {
				p[i] ^= mascara[i&3]
			}
		}
		switch op {
		case 8: // fechar
			e := &ErrFechado{Codigo: 1005}
			if len(p) >= 2 {
				e.Codigo, e.Motivo = int(binary.BigEndian.Uint16(p)), string(p[2:])
			}
			w.escrever(8, p[:min(len(p), 2)])
			return nil, e
		case 9: // ping
			w.escrever(10, p)
			continue
		case 10:
			continue
		}
		msg = append(msg, p...)
		if fim {
			return msg, nil
		}
	}
}

func (w *conexao) Fechar() {
	w.c.SetDeadline(time.Now().Add(2 * time.Second))
	w.escrever(8, []byte{0x03, 0xe8})
	w.c.Close()
}

// endereço do websocket a partir do que a pessoa escreveu (host:porta, http://…)
func enderecoWS(s string) string {
	s = strings.TrimRight(strings.TrimSpace(s), "/")
	if !strings.Contains(s, "://") {
		s = "ws://" + s
	}
	s = strings.Replace(strings.Replace(s, "https://", "wss://", 1), "http://", "ws://", 1)
	if u, err := url.Parse(s); err == nil && (u.Path == "" || u.Path == "/") {
		s += "/v1/realtime_video/generate"
	}
	return s
}
