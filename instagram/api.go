package instagram

// Leitura pela API oficial do Instagram ("Instagram API with Instagram
// Login"), para contas profissionais (comercial ou criador). A pessoa entra
// com o Instagram uma vez (ou cola um token gerado no painel do app Meta) e o
// Nepenthes lê o perfil e os posts, sem raspar o site do Instagram.
//
//   autorizar: https://www.instagram.com/oauth/authorize
//   código -> token curto: POST https://api.instagram.com/oauth/access_token
//   token curto -> longo (60 dias): GET https://graph.instagram.com/access_token
//   perfil: GET https://graph.instagram.com/me
//   posts:  GET https://graph.instagram.com/me/media

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// Endereços (trocáveis nos testes).
var (
	URLAutorizar = "https://www.instagram.com/oauth/authorize"
	URLToken     = "https://api.instagram.com/oauth/access_token"
	URLGraph     = "https://graph.instagram.com"
)

// LinkEntrar monta o endereço do botão "Entrar com Instagram".
func LinkEntrar(appID, volta, estado string) string {
	q := url.Values{}
	q.Set("client_id", appID)
	q.Set("redirect_uri", volta)
	q.Set("response_type", "code")
	q.Set("scope", "instagram_business_basic")
	q.Set("state", estado)
	return URLAutorizar + "?" + q.Encode()
}

// Cliente faz os pedidos à API (trocável nos testes).
var Cliente = &http.Client{Timeout: 30 * time.Second}

func lerResposta(res *http.Response, v any) error {
	defer res.Body.Close()
	b, _ := io.ReadAll(io.LimitReader(res.Body, 4<<20))
	if res.StatusCode != 200 {
		var e struct {
			Error struct {
				Message string `json:"message"`
			} `json:"error"`
			ErrorMessage string `json:"error_message"`
		}
		json.Unmarshal(b, &e)
		msg := e.Error.Message
		if msg == "" {
			msg = e.ErrorMessage
		}
		if msg == "" {
			msg = strings.TrimSpace(string(b))
			if len(msg) > 200 {
				msg = msg[:200]
			}
		}
		return fmt.Errorf("o Instagram respondeu %d: %s", res.StatusCode, msg)
	}
	return json.Unmarshal(b, v)
}

// TrocarCodigo troca o código da volta do login por um token longo (60 dias).
func TrocarCodigo(ctx context.Context, appID, segredo, volta, codigo string) (string, error) {
	f := url.Values{"client_id": {appID}, "client_secret": {segredo}, "grant_type": {"authorization_code"},
		"redirect_uri": {volta}, "code": {strings.TrimSuffix(codigo, "#_")}}
	req, _ := http.NewRequestWithContext(ctx, http.MethodPost, URLToken, strings.NewReader(f.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	res, err := Cliente.Do(req)
	if err != nil {
		return "", err
	}
	var curto struct {
		AccessToken string `json:"access_token"`
	}
	if err := lerResposta(res, &curto); err != nil {
		return "", err
	}
	q := url.Values{"grant_type": {"ig_exchange_token"}, "client_secret": {segredo}, "access_token": {curto.AccessToken}}
	req, _ = http.NewRequestWithContext(ctx, http.MethodGet, URLGraph+"/access_token?"+q.Encode(), nil)
	if res, err = Cliente.Do(req); err != nil {
		return curto.AccessToken, nil // o curto vale 1 hora: basta para ler agora
	}
	var longo struct {
		AccessToken string `json:"access_token"`
	}
	if lerResposta(res, &longo) != nil || longo.AccessToken == "" {
		return curto.AccessToken, nil
	}
	return longo.AccessToken, nil
}

func pegar(ctx context.Context, token, caminho string, campos string, v any) error {
	q := url.Values{"fields": {campos}, "access_token": {token}}
	sep := "?"
	if strings.Contains(caminho, "?") {
		sep = "&"
	}
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, URLGraph+caminho+sep+q.Encode(), nil)
	res, err := Cliente.Do(req)
	if err != nil {
		return fmt.Errorf("não consegui falar com o Instagram: %v", err)
	}
	return lerResposta(res, v)
}

// LerPelaAPI lê o perfil e os posts mais recentes (até max) com o token.
// As fotos de cada Post vêm como endereços https das imagens.
func LerPelaAPI(ctx context.Context, token string, max int) (*Perfil, error) {
	var me struct {
		Username  string `json:"username"`
		Name      string `json:"name"`
		Biography string `json:"biography"`
		Website   string `json:"website"`
	}
	// bio e site nem sempre vêm com o login do Instagram: se recusar, pede o básico
	if err := pegar(ctx, token, "/me", "username,name,biography,website", &me); err != nil {
		if err2 := pegar(ctx, token, "/me", "username,name", &me); err2 != nil {
			return nil, err
		}
	}
	if me.Username == "" {
		return nil, errors.New("o Instagram não devolveu o perfil")
	}
	p := &Perfil{Nome: me.Name, Usuario: me.Username, Bio: me.Biography, Site: me.Website}
	type item struct {
		ID        string `json:"id"`
		Caption   string `json:"caption"`
		MediaType string `json:"media_type"`
		MediaURL  string `json:"media_url"`
		Permalink string `json:"permalink"`
		Timestamp string `json:"timestamp"`
		Children  struct {
			Data []item `json:"data"`
		} `json:"children"`
	}
	var pag struct {
		Data []item `json:"data"`
	}
	campos := "id,caption,media_type,media_url,permalink,timestamp,children{media_type,media_url}"
	if err := pegar(ctx, token, fmt.Sprintf("/me/media?limit=%d", max), campos, &pag); err != nil {
		return nil, err
	}
	for _, it := range pag.Data {
		po := Post{Legenda: strings.TrimSpace(it.Caption), Link: it.Permalink}
		po.Data, _ = time.Parse("2006-01-02T15:04:05-0700", it.Timestamp)
		add := func(x item) {
			if x.MediaType == "IMAGE" && strings.HasPrefix(x.MediaURL, "https://") {
				po.Fotos = append(po.Fotos, x.MediaURL)
			}
		}
		if it.MediaType == "CAROUSEL_ALBUM" {
			for _, c := range it.Children.Data {
				add(c)
			}
		} else {
			add(it)
		}
		if len(po.Fotos) > 0 || po.Legenda != "" {
			p.Posts = append(p.Posts, po)
		}
	}
	return p, nil
}
