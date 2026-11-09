package main

import (
	"fmt"
	"os"
	"path/filepath"
)

const siteInicial = `<!doctype html>
<!-- Seu site inteiro mora neste arquivo. Rode com: nep dev site.nep.html -->
<nep-config movimento="true">

<nep>
  let site = {nome: "Meu site"}
</nep>

<html lang="pt-BR">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1">
  <title>{{ page.title }} · {{ site.nome }}</title>
  <style>
    body { font: 17px/1.6 system-ui, sans-serif; max-width: 720px; margin: 0 auto; padding: 0 16px; }
    nav a { margin-right: 14px; }
    form { display: grid; gap: 10px; }
    input, textarea, button { font: inherit; padding: 8px; }
    .nep-ok { background: #e3f4ec; padding: 8px 12px; } .nep-erros { background: #fde9e7; padding: 4px 12px; }
  </style>
</head>
<body>
  <nav><a href="{{ url('/') }}">Início</a><a href="{{ url('/contato') }}">Contato</a></nav>

  <page route="/" title="Início">
    <h1>Olá! Este é o {{ site.nome }}.</h1>
    <p>Edite site.nep.html e recarregue a página.</p>
  </page>

  <page route="/contato" title="Contato">
    <h1>Contato</h1>
    <form save="contatos" success="Mensagem enviada!">
      <label>Nome <input name="nome" required maxlength="80"></label>
      <label>E-mail <input name="email" type="email" required></label>
      <label>Mensagem <textarea name="mensagem" required maxlength="2000"></textarea></label>
      <button>Enviar</button>
    </form>
    <p>As mensagens aparecem no painel: <a href="{{ url('/_admin') }}">/_admin</a></p>
  </page>

  <page route="404" title="Não encontrado">
    <h1>Página não encontrada</h1>
  </page>
</body>
</html>
`

// novo cria uma pasta com um site inicial: nep novo meu-site
func novo(dir string) int {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	f := filepath.Join(dir, "site.nep.html")
	if _, err := os.Stat(f); err == nil {
		fmt.Fprintln(os.Stderr, f, "já existe; nada foi alterado")
		return 1
	}
	if err := os.WriteFile(f, []byte(siteInicial), 0o644); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	fmt.Printf("Site criado em %s\nPara ver: nep dev %s\n", f, f)
	return 0
}
