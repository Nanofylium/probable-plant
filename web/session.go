package web

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// Sessão guardada num cookie assinado (HMAC-SHA256): não precisa de arquivos
// no servidor, funciona igual no modo CGI e no servidor embutido.
type Session struct {
	CSRF    string            `json:"c"`
	AdminAt int64             `json:"a,omitempty"`
	U       string            `json:"u,omitempty"`  // editor logado ("" = o dono)
	UV      string            `json:"uv,omitempty"` // versão do editor (senha trocada derruba a sessão)
	Flash   map[string]string `json:"f,omitempty"`
	Data    json.RawMessage   `json:"d,omitempty"` // a tabela `session` do código do site
	changed bool
}

const (
	cookieName = "nep_s"
	adminIdle  = 8 * 3600
	maxCookie  = 3800
)

func randHex(n int) string {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		panic(err)
	}
	return hex.EncodeToString(b)
}

func sign(secret, msg string) string {
	m := hmac.New(sha256.New, []byte(secret))
	m.Write([]byte(msg))
	return base64.RawURLEncoding.EncodeToString(m.Sum(nil))
}

func loadSession(r *http.Request, secret string) *Session {
	s := &Session{}
	if c, err := r.Cookie(cookieName); err == nil {
		if i := strings.LastIndexByte(c.Value, '.'); i > 0 {
			body, mac := c.Value[:i], c.Value[i+1:]
			if subtle.ConstantTimeCompare([]byte(sign(secret, body)), []byte(mac)) == 1 {
				if raw, err := base64.RawURLEncoding.DecodeString(body); err == nil {
					_ = json.Unmarshal(raw, s)
				}
			}
		}
	}
	if len(s.CSRF) != 32 {
		*s = Session{CSRF: randHex(16), changed: true}
	}
	if s.AdminAt > 0 && time.Now().Unix()-s.AdminAt > adminIdle {
		s.AdminAt = 0
		s.changed = true
	}
	return s
}

func (s *Session) save(w http.ResponseWriter, secret, path string, secure bool) error {
	if !s.changed {
		return nil
	}
	raw, _ := json.Marshal(s)
	body := base64.RawURLEncoding.EncodeToString(raw)
	val := body + "." + sign(secret, body)
	if len(val) > maxCookie {
		return errTooBig
	}
	http.SetCookie(w, &http.Cookie{Name: cookieName, Value: val, Path: path, HttpOnly: true, Secure: secure, SameSite: http.SameSiteLaxMode})
	return nil
}

type sessErr string

func (e sessErr) Error() string { return string(e) }

const errTooBig = sessErr("a sessão passou de 3,8 KB (guarde menos coisas em session)")

func (s *Session) isAdmin() bool {
	if s.AdminAt == 0 {
		return false
	}
	if time.Now().Unix()-s.AdminAt > 60 { // renova sem regravar o cookie a cada pedido
		s.AdminAt = time.Now().Unix()
		s.changed = true
	}
	return true
}

// isOwner: logado como dono do site (não editor).
func (s *Session) isOwner() bool { return s.isAdmin() && s.U == "" }

func (s *Session) login(user, ver string) {
	s.CSRF = randHex(16) // token novo depois do login
	s.U, s.UV = user, ver
	s.AdminAt = time.Now().Unix()
	s.changed = true
}

func (s *Session) logout() {
	s.AdminAt = 0
	s.U, s.UV = "", ""
	s.CSRF = randHex(16)
	s.changed = true
}

func flashKey(form, inst int) string { return strconv.Itoa(form) + ":" + strconv.Itoa(inst) }

func (s *Session) setFlash(form, inst int, msg string) {
	if s.Flash == nil {
		s.Flash = map[string]string{}
	}
	s.Flash[flashKey(form, inst)] = msg
	s.changed = true
}

func (s *Session) takeFlash(form, inst int) string {
	k := flashKey(form, inst)
	m, ok := s.Flash[k]
	if ok {
		delete(s.Flash, k)
		s.changed = true
	}
	return m
}

func (s *Session) csrfOK(v string) bool {
	return v != "" && subtle.ConstantTimeCompare([]byte(v), []byte(s.CSRF)) == 1
}
