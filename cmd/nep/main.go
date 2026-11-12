// Comando nep: executa programas Nepenthes, abre o modo interativo e roda testes.
package main

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/Nanofylium/probable-plant/lang"
)

const ajuda = `Nepenthes %s

Uso:
  nep                      abre o modo interativo
  nep arquivo.nep [args]   executa um programa
  nep arquivo.lua [args]   executa um programa em Lua 5.4 (na mesma máquina virtual)
  nep run arquivo.nep      o mesmo que acima
  nep check arquivo.nep    só verifica a sintaxe (também aceita um site .nep.html)
  nep novo pasta [--modelo empresa|blog]  cria um site pronto (pasta/site.nep.html)
  nep dev site.nep.html    roda o site no computador, com erros detalhados
  nep serve site.nep.html  roda o site em produção (VPS): --porta 8000 --dados pasta --base /sub
  nep cgi [site.nep.html]  atende um pedido CGI (hospedagem compartilhada; automático se o
                           executável for chamado pelo Apache)
  nep test [pasta]         roda os testes (arquivos *_test.nep, funções test_*)
  nep disasm arquivo.nep   mostra o bytecode gerado
  nep version              mostra a versão
`

func main() {
	args := os.Args[1:]
	if os.Getenv("GATEWAY_INTERFACE") != "" && (len(args) == 0 || args[0] != "cgi") {
		os.Exit(cgiMode(""))
	}
	if len(args) == 0 {
		os.Exit(repl())
	}
	switch args[0] {
	case "-h", "--help", "help", "ajuda":
		fmt.Printf(ajuda, lang.Version)
	case "-v", "--version", "version", "versao":
		fmt.Println("nepenthes", lang.Version)
	case "run":
		if len(args) < 2 {
			fail("informe o arquivo: nep run arquivo.nep")
		}
		os.Exit(run(args[1], args[2:]))
	case "check":
		if len(args) < 2 {
			fail("informe o arquivo: nep check arquivo.nep")
		}
		if isSite(args[1]) {
			os.Exit(checkSite(args[1]))
		}
		os.Exit(check(args[1:]))
	case "serve", "servir":
		os.Exit(serve(args[1:], false))
	case "novo", "new":
		os.Exit(novo(args[1:]))
	case "dev":
		os.Exit(serve(args[1:], true))
	case "cgi":
		f := ""
		if len(args) > 1 {
			f = args[1]
		}
		os.Exit(cgiMode(f))
	case "test":
		dir := "."
		if len(args) > 1 {
			dir = args[1]
		}
		os.Exit(test(dir))
	case "disasm":
		if len(args) < 2 {
			fail("informe o arquivo: nep disasm arquivo.nep")
		}
		os.Exit(disasm(args[1]))
	default:
		if isSite(args[0]) {
			os.Exit(serve(args, true))
		}
		os.Exit(run(args[0], args[1:]))
	}
}

func fail(msg string) {
	fmt.Fprintln(os.Stderr, msg)
	os.Exit(2)
}

func newVM(script string, args []string) (*lang.VM, *bufio.Writer) {
	vm := lang.NewVM()
	out := bufio.NewWriterSize(os.Stdout, 32<<10)
	vm.Out = out
	vm.Args = append([]string{script}, args...)
	return vm, out
}

func run(path string, args []string) int {
	vm, out := newVM(path, args)
	var err error
	if strings.HasSuffix(path, ".lua") {
		err = vm.RunLuaFile(path, args)
	} else {
		err = vm.RunFile(path)
	}
	out.Flush()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	return 0
}

func check(files []string) int {
	code := 0
	for _, f := range files {
		src, err := os.ReadFile(f)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			code = 1
			continue
		}
		_, err = compileAny(string(src), f)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			code = 1
			continue
		}
		fmt.Println("ok:", f)
	}
	return code
}

// compileAny compila um arquivo .nep ou .lua (pelo nome) sem executar.
func compileAny(src, path string) (*lang.Proto, error) {
	if strings.HasSuffix(path, ".lua") {
		prog, err := lang.ParseLua(src, path)
		if err != nil {
			return nil, err
		}
		return lang.CompileLua(prog, lang.NewModule("lua", path), lang.NewTable(), path)
	}
	prog, err := lang.Parse(src, path)
	if err != nil {
		return nil, err
	}
	return lang.Compile(prog, lang.NewModule("check", path), path)
}

func disasm(path string) int {
	src, err := os.ReadFile(path)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	p, err := compileAny(string(src), path)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	fmt.Print(lang.Disassemble(p))
	return 0
}

// test procura arquivos *_test.nep e chama cada função test_* deles.
func test(dir string) int {
	var files []string
	_ = filepath.Walk(dir, func(p string, info os.FileInfo, err error) error {
		if err == nil && !info.IsDir() && strings.HasSuffix(p, "_test.nep") {
			files = append(files, p)
		}
		return nil
	})
	sort.Strings(files)
	if len(files) == 0 {
		fmt.Println("nenhum arquivo *_test.nep encontrado em", dir)
		return 1
	}
	passed, failed := 0, 0
	start := time.Now()
	for _, f := range files {
		vm, out := newVM(f, nil)
		src, err := os.ReadFile(f)
		if err != nil {
			fmt.Println("ERRO", f, err)
			failed++
			continue
		}
		abs, _ := filepath.Abs(f)
		mod := lang.NewModule(strings.TrimSuffix(filepath.Base(f), ".nep"), abs)
		_, err = vm.RunSource(string(src), abs, mod)
		out.Flush()
		if err != nil {
			fmt.Printf("ERRO  %s\n      %s\n", f, indent(err.Error()))
			failed++
			continue
		}
		var names []string
		for name := range mod.Globals {
			if strings.HasPrefix(name, "test_") {
				names = append(names, name)
			}
		}
		sort.Strings(names)
		for _, name := range names {
			_, err := vm.Call(mod.Globals[name], nil)
			out.Flush()
			if err != nil {
				fmt.Printf("FALHOU %s › %s\n      %s\n", filepath.Base(f), name, indent(err.Error()))
				failed++
			} else {
				fmt.Printf("ok     %s › %s\n", filepath.Base(f), name)
				passed++
			}
		}
	}
	fmt.Printf("\n%d passaram, %d falharam (%s)\n", passed, failed, time.Since(start).Round(time.Millisecond))
	if failed > 0 {
		return 1
	}
	return 0
}

func indent(s string) string { return strings.ReplaceAll(s, "\n", "\n      ") }

// repl é o modo interativo: digite código, veja o resultado.
func repl() int {
	fmt.Printf("Nepenthes %s — modo interativo. Digite 'sair' para encerrar.\n", lang.Version)
	vm := lang.NewVM()
	vm.Args = []string{"<repl>"}
	mod := lang.NewModule("repl", "<repl>")
	in := bufio.NewReader(os.Stdin)
	vm.In = in
	var buf strings.Builder
	for {
		if buf.Len() == 0 {
			fmt.Print(">> ")
		} else {
			fmt.Print(".. ")
		}
		line, err := in.ReadString('\n')
		if err != nil && line == "" {
			if err == io.EOF {
				fmt.Println()
				return 0
			}
			return 1
		}
		if buf.Len() == 0 && (strings.TrimSpace(line) == "sair" || strings.TrimSpace(line) == "exit") {
			return 0
		}
		buf.WriteString(line)
		src := buf.String()
		if strings.TrimSpace(src) == "" {
			buf.Reset()
			continue
		}
		prog, perr := lang.Parse(src, "<repl>")
		if perr != nil {
			if se, ok := perr.(*lang.SyntaxError); ok && se.Incomplete {
				continue // espera mais linhas
			}
			fmt.Println(perr)
			buf.Reset()
			continue
		}
		buf.Reset()
		// mostra o valor de uma expressão solta
		if len(prog) == 1 {
			if es, ok := prog[0].(*lang.ExprStmt); ok {
				if _, isCall := es.X.(*lang.CallExpr); !isCall || true {
					prog[0] = &lang.ReturnStmt{X: es.X, Line: es.Line}
				}
			}
		}
		p, cerr := lang.Compile(prog, mod, "<repl>")
		if cerr != nil {
			fmt.Println(cerr)
			continue
		}
		res, rerr := vm.Call(&lang.Closure{Proto: p}, nil)
		if rerr != nil {
			fmt.Println(rerr)
			continue
		}
		if res != nil {
			fmt.Println(lang.Repr(res))
		}
	}
}
