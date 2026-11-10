package web

// Upload de arquivos em formulários: <input type="file" name="foto" accept="image/*">.
// Só imagens (JPEG, PNG, GIF, WebP) e PDF são aceitos, conferidos pelo conteúdo
// do arquivo e não pelo nome. Os arquivos ficam em data/arquivos/ e são
// entregues por /_nep/arquivo/...: restritos ao painel por padrão, públicos
// com data-publico. Apagar o registro (no painel ou na página LGPD) apaga o arquivo.

import (
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"unicode/utf8"

	"github.com/Nanofylium/probable-plant/lang"
)

const maxUpload = 25 << 20

var fileKinds = map[string]string{
	"image/jpeg":      ".jpg",
	"image/png":       ".png",
	"image/gif":       ".gif",
	"image/webp":      ".webp",
	"application/pdf": ".pdf",
}

var (
	fileNameRe = regexp.MustCompile(`^[pr]-[0-9a-f]{32}\.(jpg|png|gif|webp|pdf)$`)
	fileRefRe  = regexp.MustCompile(`/_nep/arquivo/([pr]-[0-9a-f]{32}\.(?:jpg|png|gif|webp|pdf))`)
)

type pendingFile struct {
	field *Field
	fh    *multipart.FileHeader
	kind  string
}

func accepts(accept, kind string) bool {
	if strings.TrimSpace(accept) == "" {
		return true
	}
	ext := fileKinds[kind]
	for _, a := range strings.Split(accept, ",") {
		a = strings.TrimSpace(a)
		switch {
		case a == kind, a == ext, a == ".jpeg" && ext == ".jpg":
			return true
		case strings.HasSuffix(a, "/*") && strings.HasPrefix(kind, strings.TrimSuffix(a, "*")):
			return true
		}
	}
	return false
}

func sniff(fh *multipart.FileHeader) (string, error) {
	f, err := fh.Open()
	if err != nil {
		return "", err
	}
	defer f.Close()
	head := make([]byte, 512)
	n, _ := io.ReadFull(f, head)
	return http.DetectContentType(head[:n]), nil
}

// checkFiles confere os arquivos enviados contra as regras do HTML.
func checkFiles(fields []*Field, r *http.Request) ([]pendingFile, []string) {
	var out []pendingFile
	var errs []string
	for _, f := range fields {
		if f.Type != "file" {
			continue
		}
		var fh *multipart.FileHeader
		if r.MultipartForm != nil && len(r.MultipartForm.File[f.Name]) > 0 {
			fh = r.MultipartForm.File[f.Name][0]
		}
		if fh == nil || fh.Size == 0 {
			if f.Required {
				errs = append(errs, fmt.Sprintf("Escolha um arquivo em \"%s\".", f.Label))
			}
			continue
		}
		if float64(fh.Size) > f.MaxMB*(1<<20) {
			errs = append(errs, fmt.Sprintf("O arquivo em \"%s\" passa de %s MB.", f.Label, fmtNum(f.MaxMB)))
			continue
		}
		kind, err := sniff(fh)
		if err != nil {
			errs = append(errs, fmt.Sprintf("Não consegui ler o arquivo em \"%s\".", f.Label))
			continue
		}
		if _, ok := fileKinds[kind]; !ok || !accepts(f.Accept, kind) {
			errs = append(errs, fmt.Sprintf("Tipo de arquivo não aceito em \"%s\" (envie imagem JPG, PNG, GIF, WebP ou PDF).", f.Label))
			continue
		}
		out = append(out, pendingFile{field: f, fh: fh, kind: kind})
	}
	return out, errs
}

func cleanName(n string) string {
	n = filepath.Base(strings.ReplaceAll(n, "\\", "/"))
	n = strings.Map(func(r rune) rune {
		if r < 32 || strings.ContainsRune(`<>:"/\|?*`, r) {
			return '_'
		}
		return r
	}, n)
	if utf8.RuneCountInString(n) > 100 {
		n = string([]rune(n)[:100])
	}
	return n
}

// saveFiles grava os arquivos e coloca na tabela de dados {nome, tipo, tamanho, url}.
func (s *Server) saveFiles(files []pendingFile, data *lang.Table) ([]string, error) {
	if len(files) == 0 {
		return nil, nil
	}
	dir := filepath.Join(s.opt.DataDir, "arquivos")
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return nil, err
	}
	var saved []string
	for _, p := range files {
		vis := "r"
		if p.field.Public {
			vis = "p"
		}
		name := vis + "-" + randHex(16) + fileKinds[p.kind]
		src, err := p.fh.Open()
		if err != nil {
			s.Store.removeFiles(saved)
			return nil, err
		}
		dst, err := os.OpenFile(filepath.Join(dir, name), os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o640)
		if err == nil {
			_, err = io.Copy(dst, src)
			dst.Close()
		}
		src.Close()
		if err != nil {
			s.Store.removeFiles(append(saved, name))
			return nil, fmt.Errorf("não consegui gravar o arquivo: %v", err)
		}
		saved = append(saved, name)
		t := lang.NewTable()
		t.SetStr("nome", cleanName(p.fh.Filename))
		t.SetStr("tipo", p.kind)
		t.SetStr("tamanho", p.fh.Size)
		t.SetStr("url", s.opt.Base+"/_nep/arquivo/"+name)
		data.SetStr(p.field.Name, t)
	}
	return saved, nil
}

// removeFiles apaga arquivos enviados (pelo nome dentro de data/arquivos).
func (s *Store) removeFiles(names []string) {
	for _, n := range names {
		if fileNameRe.MatchString(n) {
			os.Remove(filepath.Join(s.dir, "arquivos", n))
		}
	}
}

// filesIn acha os arquivos citados nos dados de um registro.
func filesIn(data string) []string {
	var out []string
	for _, m := range fileRefRe.FindAllStringSubmatch(data, -1) {
		out = append(out, m[1])
	}
	return out
}

// serveFile entrega /_nep/arquivo/NOME. Arquivos restritos só para quem está no painel.
func (s *Server) serveFile(w http.ResponseWriter, r *http.Request, name string) {
	if !fileNameRe.MatchString(name) {
		http.NotFound(w, r)
		return
	}
	if name[0] == 'r' {
		if !loadSession(r, s.secret).isOwner() { // dados de visitantes: só o dono
			http.Error(w, "Arquivo restrito: entre no painel para ver.", http.StatusForbidden)
			return
		}
		w.Header().Set("Cache-Control", "private, no-store")
	} else {
		w.Header().Set("Cache-Control", "public, max-age=86400")
	}
	full := filepath.Join(s.opt.DataDir, "arquivos", name)
	f, err := os.Open(full)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	defer f.Close()
	st, _ := f.Stat()
	for kind, ext := range fileKinds {
		if strings.HasSuffix(name, ext) {
			w.Header().Set("Content-Type", kind)
		}
	}
	w.Header().Set("Content-Security-Policy", "sandbox; default-src 'none'; img-src 'self'; style-src 'unsafe-inline'")
	w.Header().Set("Content-Disposition", "inline")
	http.ServeContent(w, r, "", st.ModTime(), f)
}
