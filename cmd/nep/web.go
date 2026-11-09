package main

import (
	"flag"
	"fmt"
	"net"
	"net/http"
	"net/http/cgi"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/Nanofylium/probable-plant/web"
)

// isSite diz se o arquivo é um site (.nep.html / .html) e não um programa .nep.
func isSite(f string) bool {
	l := strings.ToLower(f)
	return strings.HasSuffix(l, ".html") || strings.HasSuffix(l, ".htm")
}

// serve sobe o servidor web embutido: nep serve [--porta 8000] [--dev] site.nep.html
func serve(args []string, dev bool) int {
	fs := flag.NewFlagSet("serve", flag.ExitOnError)
	port := fs.String("porta", "", "porta (padrão 8000, ou a variável PORT)")
	fs.StringVar(port, "port", "", "o mesmo que --porta")
	host := fs.String("host", "", "endereço (padrão: 127.0.0.1 no modo dev, todos em produção)")
	data := fs.String("dados", "", "pasta de dados (padrão: data/ ao lado do site)")
	base := fs.String("base", "", "prefixo da URL, se o site fica numa subpasta (ex.: /loja)")
	fs.BoolVar(&dev, "dev", dev, "modo de desenvolvimento: erros detalhados")
	_ = fs.Parse(reorder(args))
	file := "site.nep.html"
	if fs.NArg() > 0 {
		file = fs.Arg(0)
	}
	if *port == "" {
		*port = os.Getenv("PORT")
	}
	if *port == "" {
		*port = "8000"
	}
	if *host == "" && dev {
		*host = "127.0.0.1"
	}
	srv, err := web.New(web.Options{SiteFile: file, DataDir: *data, Dev: dev, Base: *base})
	if srv == nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "atenção:", err)
	}
	addr := net.JoinHostPort(*host, *port)
	shown := addr
	if *host == "" {
		shown = "localhost:" + *port
	}
	modo := "produção"
	if dev {
		modo = "desenvolvimento"
	}
	fmt.Printf("Nepenthes servindo %s em http://%s%s/ (%s)\n", file, shown, *base, modo)
	fmt.Printf("Painel: http://%s%s/_admin   ·   Ctrl+C para parar\n", shown, *base)
	hs := &http.Server{Addr: addr, Handler: srv, ReadHeaderTimeout: 10 * time.Second, ReadTimeout: 30 * time.Second, IdleTimeout: 2 * time.Minute}
	if err := hs.ListenAndServe(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	return 0
}

// reorder deixa as opções (--x) antes do nome do arquivo, para aceitar as duas ordens.
func reorder(args []string) []string {
	var flags, rest []string
	for i := 0; i < len(args); i++ {
		a := args[i]
		if strings.HasPrefix(a, "-") {
			flags = append(flags, a)
			if !strings.Contains(a, "=") && i+1 < len(args) && !strings.HasPrefix(args[i+1], "-") && a != "--dev" && a != "-dev" {
				flags = append(flags, args[i+1])
				i++
			}
		} else {
			rest = append(rest, a)
		}
	}
	return append(flags, rest...)
}

// cgiMode atende um pedido CGI (hospedagem compartilhada com Apache).
// O site é o arquivo da variável NEP_SITE ou site.nep.html ao lado do executável.
func cgiMode(file string) int {
	if file == "" {
		file = os.Getenv("NEP_SITE")
	}
	if file == "" {
		exe, _ := os.Executable()
		file = filepath.Join(filepath.Dir(exe), "site.nep.html")
	}
	base := strings.TrimRight(path.Dir(os.Getenv("SCRIPT_NAME")), "/.")
	dev := os.Getenv("NEP_DEV") == "1"
	srv, err := web.New(web.Options{SiteFile: file, Dev: dev, CGI: true, Base: base, DataDir: os.Getenv("NEP_DATA")})
	if srv == nil {
		fmt.Print("Status: 500\r\nContent-Type: text/plain; charset=utf-8\r\n\r\nNepenthes: " + err.Error() + "\n")
		return 0
	}
	if err := cgi.Serve(srv); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	return 0
}

func checkSite(f string) int {
	site, err := web.Check(f)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	fmt.Println("ok:", f)
	for _, p := range site.Pages {
		extra := ""
		if p.Auth {
			extra = "  (restrita)"
		}
		fmt.Printf("  página  %-24s linha %d%s\n", p.Route, p.Line, extra)
	}
	for _, a := range site.APIs {
		fmt.Printf("  api     %-6s %-17s linha %d\n", a.Method, a.Route, a.Line)
	}
	for _, fm := range site.Forms {
		var names []string
		for _, fd := range fm.Fields {
			names = append(names, fd.Name)
		}
		dest := "salva em '" + fm.Save + "'"
		if fm.Save == "" {
			dest = "chama " + fm.Run + "()"
		} else if fm.Run != "" {
			dest += " e chama " + fm.Run + "()"
		}
		fmt.Printf("  form    %s · campos: %s\n", dest, strings.Join(names, ", "))
	}
	for _, d := range site.Data {
		fmt.Printf("  dados   %s\n", d.Name)
	}
	if n := len(site.Lives); n > 0 {
		fmt.Printf("  vivo    %d região(ões) <live>\n", n)
	}
	return 0
}
