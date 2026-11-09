# Nepenthes

Nepenthes é uma linguagem de programação de uso geral, com sintaxe clara (`let`, `fn`, `do ... end`), o operador pipe `|>` para trabalhar com dados e uma biblioteca padrão pensada para o Brasil (`money`, datas `d/m/Y`, CSV com `;`).

E tem uma coisa que nenhuma outra linguagem faz sozinha: **um arquivo HTML vira um site completo**, com páginas, banco de dados, formulários que salvam, painel administrativo, API JSON e partes da página que se atualizam ao vivo, tudo servido pelo próprio executável `nep`. Sem PHP, sem Node, sem MySQL, sem build.

```html
<page route="/" title="Mural">
  <form save="recados" success="Publicado!">
    <label>Nome <input name="autor" required maxlength="40"></label>
    <label>Recado <textarea name="texto" required maxlength="280"></textarea></label>
    <button>Publicar</button>
  </form>

  <live>   <!-- atualiza sozinho em todas as abas abertas quando alguém publica -->
    <each r in="db.all('recados', 30)">
      <p><strong>{{ r.autor }}</strong>: {{ r.texto }}</p>
    <else>
      <p>Nenhum recado ainda.</p>
    </each>
  </live>
</page>
```

```
nep dev site.nep.html
```

Isso já é um site no ar: o formulário valida no servidor as regras do próprio HTML (`required`, `type=email`, `maxlength`…), tem proteção CSRF, anti-robô e limite de envios, salva no banco embutido, aparece no painel `/_admin` (com exportação CSV e ferramenta LGPD) e a lista se atualiza ao vivo nas outras abas.

## O que o diferencia

| | PHP + WordPress | Node/Next.js | htmx / Alpine | Phoenix LiveView | **Nepenthes** |
|---|---|---|---|---|---|
| Site inteiro em 1 arquivo HTML | não | não | não (só a parte do navegador) | não | **sim** |
| Linguagem própria, servidor e banco no mesmo executável | não | não | não | não | **sim** |
| Formulário valida e salva a partir do próprio HTML | plugin | não | não | não | **sim** |
| Partes da página ao vivo sem escrever JavaScript | não | não | parcial | sim | **sim (`<live>`)** |
| Painel admin, CSV e LGPD embutidos | plugins | não | não | não | **sim** |
| Roda em hospedagem compartilhada barata | sim | não | sim | não | **sim (CGI)** |

## Instalar

Os executáveis de cada sistema são gerados pela CI a cada mudança (aba **Actions** do GitHub, artefatos da última execução). Para compilar você mesmo, precisa do Go 1.24 ou mais novo:

```
go test ./...
go build -o nep ./cmd/nep
```

| Sistema | Arquivo |
|---|---|
| Windows | `nep-windows-amd64.exe` (renomeie para `nep.exe`) |
| Linux | `nep-linux-amd64` ou `nep-linux-arm64` |
| Mac (Apple Silicon) | `nep-macos-arm64` |
| Mac (Intel) | `nep-macos-amd64` |

No Linux/Mac: `chmod +x nep-*` e, no Mac, na primeira vez, clique com o botão direito > Abrir (o executável ainda não é assinado).

## Usar

```
nep                         modo interativo
nep programa.nep            executa um programa
nep check arquivo           verifica um programa .nep ou um site .nep.html
nep test pasta              roda os testes (*_test.nep, funções test_*)
nep disasm programa.nep     mostra o bytecode

nep novo meu-site           cria meu-site/site.nep.html
nep dev site.nep.html       roda o site no computador (http://localhost:8000), erros com a linha exata
nep serve site.nep.html     roda em produção num VPS: --porta 8000 --dados /var/lib/site --base /loja
```

## Colocar um site no ar

**VPS (recomendado):** copie o `nep` e o `site.nep.html` para o servidor, rode `nep serve site.nep.html --porta 8000` como serviço (systemd) e coloque um proxy com HTTPS na frente (Caddy ou Nginx). As regiões `<live>` usam Server-Sent Events, em tempo real.

**Hospedagem compartilhada (cPanel, Apache):** envie para a pasta do site o executável `nep-linux-amd64` renomeado para `nep.cgi` (permissão 755), o arquivo [`hospedagem/.htaccess`](hospedagem/.htaccess) e o seu `site.nep.html`. A hospedagem precisa permitir CGI; as regiões `<live>` consultam o servidor a cada 4 segundos.

Nos dois casos, a pasta `data/` é criada sozinha, e o painel fica em `/_admin`. No primeiro acesso, ele pede o código que está em `data/SETUP_TOKEN.txt` (só quem tem acesso aos arquivos do servidor consegue ver).

## A linguagem em um exemplo

```nep
import json
import fs

let vendas = json.decode(fs.read("vendas.json"))
let por_mes = vendas
  |> filter(fn(v) do v.status == "pago" end)
  |> group_by("mes")

for mes, itens in por_mes do
  print(mes, itens |> pluck("valor") |> sum |> money)
end
```

A definição completa está em [`SPEC.md`](SPEC.md). A seção 10 descreve as tags de site: `<page>`, `{{ }}`, `<nep>`, `<each>`, `<if>`, `<data>`, `<form save|run>`, `<live>`, `<api>` e `<nep-config>`.

## O que tem nesta pasta

- `SPEC.md`: a especificação da linguagem e do formato `.nep.html`.
- `lang/`: a implementação de referência em Go: lexer, parser, compilador para bytecode, máquina virtual e biblioteca padrão. `lang/std/cerebro.nep` é um módulo escrito na própria Nepenthes.
- `web/`: o lado site: leitura do `.nep.html` e tradução para Nepenthes, servidor HTTP, banco de dados embutido, sessões, formulários, painel e os scripts `movimento.js` e `vivo.js`.
- `cmd/nep/`: o comando `nep`.
- `exemplos/`: programas `.nep` e dois sites completos: `exemplos/padaria` (loja com encomendas) e `exemplos/mural` (mural ao vivo com curtidas e API).
- `hospedagem/.htaccess`: configuração para hospedagem compartilhada.
- `testdata/`: programas com a saída esperada, usados nos testes automáticos.

## Desempenho

Na mesma máquina, `fib(30)` recursivo leva cerca de 0,2 s, na mesma faixa do Python 3. Uma página típica de site é gerada em poucos milissegundos.

## Próximos passos

Interpolação de texto, métodos (`lista.map(...)`), upload de arquivos em formulários, login de visitantes, e-mail de aviso de formulário, gerenciador de pacotes, `nep fmt`, extensão para VS Code e o módulo `mosca` (simulação do conectoma da Drosophila com dados públicos do hemibrain da Janelia, CC BY 4.0; sem dados do FlyWire).
