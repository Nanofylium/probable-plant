# Especificação da linguagem Nepenthes — versão 1 (rascunho 0.1)

Este documento define o que um programa Nepenthes significa e como um arquivo `.nep.html` vira um site (seção 10). Qualquer implementação (o executável `nep` em Go, o runtime web antigo em PHP) deve seguir estas regras. Onde uma implementação ainda diverge, isso está marcado como **[divergência]**.

## 1. Arquivos e codificação

- Código-fonte em UTF-8, extensão `.nep`.
- Um arquivo é um **módulo**. O arquivo passado ao `nep` é o módulo principal.
- Fim de linha não é obrigatório entre instruções, mas uma chamada `f(...)` ou um índice `x[...]` precisa começar na mesma linha do valor à esquerda (assim `a` numa linha e `(b)` na próxima são duas instruções).

## 2. Léxico

**Comentários**: de `#` ou `--` até o fim da linha.

**Identificadores**: letra (inclusive acentuada) ou `_`, seguida de letras, dígitos ou `_`. `preço` e `ação` são nomes válidos.

**Palavras reservadas**:
`let fn do end if elif else for in while return true false nil and or not break continue try catch import as`

**Números**: inteiros `42`, `1_000_000`; reais `3.14`, `1e3`, `2.5e-4`. Um literal com ponto ou expoente é `float`.

**Textos**: entre `"` ou `'`, podendo ocupar várias linhas. Escapes: `\n \t \r \0 \\ \" \' \uXXXX`.

**Textos com valores** (interpolação): entre crases, cada `{expressão}` é trocada pelo valor mostrado como em `str`: `` `Olá, {nome}! Total: {itens |> sum |> money}` ``. Use `{{` e `}}` para escrever chaves. Escapes: `` \n \t \` \\ \{ \} ``. Um texto com crases equivale a `"" + "Olá, " + str(nome) + "!"` e pode ocupar várias linhas.

**Operadores e pontuação**: `+ - * / // % == != < > <= >= = += -= *= /= |> . , : ( ) [ ] { } ...`

## 3. Tipos

| Tipo | Exemplos | Observação |
|---|---|---|
| `nil` | `nil` | ausência de valor |
| `bool` | `true`, `false` | |
| `int` | `42`, `-7` | inteiro de 64 bits |
| `float` | `3.14`, `2.0` | IEEE 754 de 64 bits |
| `string` | `"ação"` | imutável; `len` conta caracteres, não bytes |
| `list` | `[1, "a", nil]` | sequência mutável, índice a partir de 0 |
| `table` | `{nome: "Ana", "com espaço": 1}` | dicionário mutável que **preserva a ordem de inserção** |
| `fn` | `fn(x) do x * 2 end`, `print` | funções são valores |
| `module` | resultado de `import` | só leitura |
| `html` | `raw("<b>oi</b>")` | texto marcado como HTML seguro; só existe em sites (seção 10); `html + html` é `html` |

**Referência**: listas e tabelas são passadas por referência. `let b = a` faz `b` e `a` apontarem para a mesma lista. Use `copy(a)` para uma cópia rasa. **[divergência]** o runtime web em PHP 0.1 copia por valor.

**Chaves de tabela**: texto, número ou bool. Um `float` com valor inteiro vira `int` (`t[1.0]` é o mesmo que `t[1]`).

**Verdade**: são falsos `nil`, `false`, `0`, `0.0`, `""`, `[]` e `{}`. Todo o resto é verdadeiro.

## 4. Expressões

### 4.1 Precedência (da menor para a maior)

1. `|>` (pipe)
2. `or`
3. `and`
4. `not`
5. `== != < > <= >=`
6. `+ -`
7. `* / // %`
8. `-` unário
9. chamada `f(x)`, campo `t.nome`, índice `l[0]`

Operadores binários de mesmo nível associam à esquerda.

### 4.2 Aritmética

- `int op int` dá `int`, exceto `/`: dá `int` quando a divisão é exata (`6 / 3 == 2`) e `float` caso contrário (`7 / 2 == 3.5`).
- Se um dos lados é `float`, o resultado é `float`.
- `//` é divisão arredondada para baixo (`-7 // 2 == -4`); `%` tem o sinal do divisor (`-7 % 3 == 2`), como em Python.
- Divisão ou resto por zero é erro.
- `+` concatena quando um dos lados é texto (`"a" + 1 == "a1"`) e junta duas listas (`[1] + [2] == [1, 2]`).
- `texto * int` repete o texto.

### 4.3 Comparação

- `==` e `!=` comparam por **valor**: `1 == 1.0`, `[1, [2]] == [1, [2]]`, `{a: 1} == {a: 1}`. Funções e módulos comparam por identidade. Tipos diferentes (fora números) nunca são iguais.
- `< > <= >=` funcionam entre números ou entre textos (ordem de bytes UTF-8). Outras combinações são erro.

### 4.4 Lógica

`and` e `or` param assim que o resultado é conhecido e devolvem um dos operandos (não necessariamente um bool): `nil or "padrão"` vale `"padrão"`. `not` sempre devolve bool.

### 4.5 Pipe

`x |> f(a, b)` é o mesmo que `f(x, a, b)`. `x |> f` é o mesmo que `f(x)`. Encadeia da esquerda para a direita e pode quebrar linhas antes do `|>`.

### 4.6 Acesso

- `t.nome` lê o campo de uma tabela (ou `nil` se não existe) ou um nome exportado de um módulo (erro se não existe). Ler campo de `nil` é erro.
- **Métodos**: em listas, textos e números, `v.f(a, b)` é o mesmo que `f(v, a, b)`, onde `f` é uma função do módulo atual ou da biblioteca padrão: `lista.sort().map(fn(x) do x * 2 end)`, `"ana".upper()`, `(19.9).money()`. Em tabelas, o ponto continua lendo campos (use `|>` ou chame a função).
- `l[i]` em lista ou texto: `i` inteiro; negativo conta do fim (`l[-1]` é o último). Fora dos limites devolve `nil`.
- `t[k]` em tabela: qualquer chave válida.

### 4.7 Literais compostos

- Lista: `[a, b, c]` (vírgula final permitida).
- Tabela: `{chave: valor, "outra chave": v}`; chaves são nomes ou textos.
- Função anônima: `fn(a, b) do ... end`.

## 5. Instruções

### 5.1 Declaração e atribuição

```
let nome = expressão      # cria uma variável no escopo atual
nome = expressão          # altera uma variável que já existe
nome += 1                 # também -= *= /=
t.campo = v               # cria ou altera o campo de uma tabela
l[i] = v                  # i == len(l) acrescenta no fim
```
Atribuir a um nome que não foi declarado com `let` é erro em tempo de execução. No nível do arquivo, `let` pode redeclarar um nome; dentro de um bloco, declarar duas vezes o mesmo nome no mesmo bloco é erro de compilação.

### 5.2 Escopo

- Variáveis do nível do arquivo são **globais do módulo**.
- Cada bloco (`do ... end` de `if`, `while`, `for`, `try`, `catch` e funções) cria um escopo novo. `let` dentro dele não é visível fora.
- Funções capturam variáveis do escopo onde foram criadas (closures). Em `for`, cada volta tem variáveis novas, então funções criadas no laço lembram o valor daquela volta.

### 5.3 Condicional

```
if cond do
  ...
elif outra do
  ...
else
  ...
end
```
`else do` e `else if` também são aceitos (`else if` equivale a `elif` e compartilha o mesmo `end`).

### 5.4 Laços

```
while cond do ... end

for item in lista do ... end            # valores
for i, item in lista do ... end         # índice e valor
for chave, valor in tabela do ... end   # tabela: um nome recebe o valor; dois recebem chave e valor
for i, letra in "texto" do ... end      # caracteres
```
`for` sobre `nil` não executa nenhuma vez. Alterar uma lista durante o `for` não muda as voltas daquele laço (o laço percorre o conteúdo do início). `break` sai do laço mais interno; `continue` vai para a próxima volta.

### 5.5 Funções

```
fn soma(a, b) do
  return a + b
end

fn dobro(x) do x * 2 end          # a última expressão é o valor devolvido
fn todos(primeiro, ...resto) do    # variádica: resto é uma lista
  resto
end
```
- `fn nome(...)` no nível do arquivo cria uma global; dentro de outra função, cria uma variável local (que pode ser recursiva).
- Faltando argumentos, os parâmetros recebem `nil`. Argumentos a mais são erro (exceto em funções variádicas).
- Sem `return`, a função devolve o valor da última instrução se ela for uma expressão; senão, `nil`.
- Profundidade máxima de chamadas: 4000.

### 5.6 Erros

```
try do
  arriscado()
catch e do
  print(e.message, e.line)
end
```
- `error(valor)` lança um erro. Com texto, o valor capturado é `{message, line, file}`; com outro valor (por exemplo uma tabela), o próprio valor.
- Erros da linguagem (divisão por zero, variável inexistente…) chegam ao `catch` como `{message, line, file}`.
- `return`, `break` e `continue` podem sair de dentro de um `try`.
- Erro não capturado encerra o programa com a mensagem e o rastro de chamadas, e código de saída 1.

### 5.7 Módulos

```
import math                    # módulo da biblioteca padrão
import "lib/util"              # arquivo lib/util.nep, relativo ao arquivo atual; nome: util
import "lib/util" as u         # escolhe o nome
```
- Um módulo é executado uma só vez; importações seguintes recebem o mesmo módulo.
- Tudo que o módulo define no nível do arquivo (com `let`, `fn` ou `import`) fica acessível como `modulo.nome`.
- Um nome sem aspas procura primeiro na biblioteca padrão e depois um arquivo `nome.nep` na pasta do arquivo atual.
- Importação circular é erro.

## 6. Biblioteca padrão

### 6.1 Funções globais (sem import)

| Grupo | Funções |
|---|---|
| Básico | `print(...)`, `input(prompt)`, `type(v)`, `str(v)`, `repr(v)`, `int(v)`, `float(v)`, `bool(v)`, `len(v)`, `copy(v)`, `default(v, padrão)`, `error(v)`, `assert(cond, msg)` |
| Texto | `upper lower trim capitalize replace(s, a, b) split(s, sep) join(lista, sep) starts_with ends_with contains find_index substr(s, início, n) truncate(s, n) pad(s, n, ch) slug format(modelo, ...)` |
| Números | `round(x, casas) floor ceil abs money number(x, casas) sum avg min max summarize` |
| Listas | `range map filter find reduce any all sort sort_by group_by count_by where pluck take skip slice first last reverse push pop insert remove unique flatten zip enumerate` |
| Tabelas | `keys values has merge remove` |
| Datas | `now()`, `date(valor, "d/m/Y")` |
| Dados | `load_csv(caminho)` |
| Do Lua | `setmetatable getmetatable rawget rawset match gmatch gsub coroutine.{create resume yield wrap status running isyieldable close}` |

Regras gerais: funções que recebem "chave" (`sort_by`, `group_by`, `count_by`) aceitam um nome de campo ou uma função. `push`, `pop`, `insert` e `remove` alteram a lista recebida; as demais devolvem listas novas. `format("Olá, {}!", nome)` usa `{}` em ordem, `{0}` por posição ou `{campo}` com uma tabela. `money(19.9)` dá `"R$ 19,90"`.

Formato de datas (letras do PHP, iguais no runtime web): `d j m n Y y H G i s D w U`.

### 6.2 Módulos

| Módulo | Conteúdo |
|---|---|
| `math` | `pi e inf sqrt pow sin cos tan log log10 exp random random_int(a, b) choice shuffle seed` |
| `fs` | `read write append lines exists is_dir list mkdir remove join` |
| `json` | `encode pretty decode` (preserva a ordem das chaves) |
| `os` | `args platform env(nome, padrão) exit(código) cwd` |
| `time` | `unix sleep(ms) format(data, formato) diff_days add_days` |
| `http` | `get post put delete` → `{status, ok, body, headers}` |
| `cerebro` | `WorkingMemory AttentionGate DecisionMaker MetaCognition` (escrito em Nepenthes) |

## 7. Exibição de valores

`print` e `str` mostram textos sem aspas. Dentro de listas e tabelas, textos aparecem entre aspas. `float` sempre mostra o ponto (`2.0`) e usa a menor representação exata (`0.1 + 0.2` mostra `0.30000000000000004`). Tabelas: `{nome: "Ana", "com espaço": 1}`.

**[divergência]** o runtime web mostra `2.0` como `2` e arredonda floats em 10 casas, para exibição em páginas.

## 8. Implementação de referência (`nep`)

- Escrita em Go, sem dependências externas: um único executável.
- O código é lido, transformado em árvore, compilado para **bytecode** e executado numa **máquina virtual de pilha** com closures e upvalues (o mesmo modelo da Lua). `nep disasm arquivo.nep` mostra o bytecode.
- Comandos: `nep` (interativo), `nep arquivo.nep`, `nep check`, `nep test`, `nep disasm`, `nep version`, e para sites `nep novo`, `nep dev`, `nep serve`, `nep cgi`.

## 10. Nepenthes Web: um arquivo `.nep.html` é um site

Um arquivo `.nep.html` é HTML comum com algumas tags a mais. O `nep` lê o arquivo, traduz o HTML para código Nepenthes e executa na mesma máquina virtual da linguagem. Não há PHP, Node nem banco de dados para instalar: o executável `nep` é o servidor, o banco e o painel.

```
nep novo meu-site              cria meu-site/site.nep.html
nep dev meu-site/site.nep.html desenvolvimento: http://localhost:8000, erros com a linha do arquivo
nep serve site.nep.html        produção (VPS), atrás de um proxy com HTTPS
nep.cgi                        hospedagem compartilhada (Apache + CGI), ver hospedagem/.htaccess
```

### 10.1 Estrutura

Tudo fora de `<page>` é o **layout** (cabeçalho, rodapé), comum a todas as páginas. A `<page>` cuja rota bate com o endereço aparece no lugar onde está escrita; as outras somem.

| Tag | Significado |
|---|---|
| `<page route="/x/:id" title="T" auth>` | uma página. `:nome` captura um trecho (`params.nome`); `*` captura o resto (`params.resto`). `route="404"` é a página de não encontrado. `auth` exige o login do painel. Em português: `<pagina rota titulo restrita>` |
| `{{ expressão }}` | mostra o valor, **sempre escapado** (proteção contra XSS). Vale em texto e atributos. Valores `html` (de `raw`, `nl2br`, `json`) não são escapados |
| `<nep> código </nep>` | código Nepenthes. Fora de `<each>`/`<if>`/`<live>` roda **antes** do HTML, na ordem do arquivo (o global e depois o da página pedida), por isso uma página pode mudar `page.title`. Dentro de `<each>` roda a cada volta. `print` escreve no HTML (escapado) |
| `<each x in="lista">` / `<each k, v in="tabela">` | repete o conteúdo. `loop.index` (1, 2…), `loop.first`, `loop.last`, `loop.count`. Um `<else>` dentro mostra o conteúdo quando a lista está vazia. Em português: `<cada x em="...">` |
| `<if test="cond"> … <else> … </if>` | condição. Em português: `<se teste>` e `<senao>` |
| `<data name="nome">JSON</data>` | dados fixos, viram a variável `nome` |
| `<nep-config ...>` | `movimento="true\|suave\|forte"` (anima a página, seção 10.6), `progresso`, `cor`, `csp="..."` |
| `<form save="col">` / `<form run="funcao">` | formulário que valida e salva sozinho (10.3) |
| `<live>` / `<live every="10">` | região que se atualiza sozinha no navegador (10.4). Em português: `<vivo cada="10">` |
| `<api route="/api/x" method="GET">código</api>` | rota que devolve JSON (10.5) |
| `<conteudo nome="artigos">campos</conteudo>` | tipo de conteúdo que quem cuida do site cria e edita no painel (10.11) |
| `editavel="chave"` em qualquer tag | texto ou imagem que quem cuida do site troca no painel ou direto na página (10.11) |
| `menu-editavel="nome"` | menu de links que quem cuida do site reorganiza no painel (10.11) |
| `construtor="chave"` | área que quem cuida do site monta arrastando blocos no construtor visual (10.11) |

Cada linha de código gerada fica na mesma linha do `.nep.html`: um erro aponta a linha do arquivo do site.

### 10.2 Variáveis e funções disponíveis no site

- `page` (`route`, `title`, `path`), `params`, `request` (`method`, `path`, `query`, `live`; nas APIs também `json` e `form`), `admin` (bool), `session` (tabela do visitante, 10.7).
- `db`: `all(col, limite)` (mais novos primeiro), `find(col, id)` (ou `find(col, "endereco")` nos tipos de `<conteudo>`), `where(col, campo, valor)`, `count(col)`, `insert(col, tabela)` → id, `update(col, id, tabela)` (mistura os campos), `incr(col, id, campo, n)` (soma sem condição de corrida), `delete(col, id)`. Cada registro ganha `id`, `created_at` e `updated_at`.
- `url("/caminho")` (funciona em subpasta), `raw(texto)`, `nl2br(texto)`, `markdown(texto)`, `visual(texto)` e `menu("nome")` (10.11), `json(valor)`, `urlencode(texto)`, `texto("chave", "padrão")` (um texto editável usado no código).
- `not_found()`, `redirect("/caminho")` (só caminhos do próprio site), `abort(código, mensagem)`, `recusar(mensagem)` (ou `reject`). Essas quatro **param** o pedido e não são capturadas por `try`.
- Módulos `fs`, `os` e `http` e a função `input` não existem no site. `load_csv` só lê arquivos dentro da pasta do site, nunca de `data/`.
- Cada pedido pode executar no máximo 40 milhões de instruções; um laço infinito vira erro 500, não trava o servidor.

### 10.3 Formulários

```html
<form save="contatos" success="Mensagem enviada!" redirect="/obrigado" limit="5">
  <label>Nome <input name="nome" required maxlength="80"></label>
  <label>E-mail <input name="email" type="email" required></label>
  <button>Enviar</button>
</form>
```
- O servidor valida as regras escritas no próprio HTML: `required`, `type=email|url|number|range|date|tel|checkbox`, `min`, `max`, `maxlength` (padrão 500, ou 5000 em `textarea`), opções de `<select>` e de radio/checkbox. Campos `nome[]` viram listas. `number` vira `int` ou `float`; checkbox vira `bool`.
- Só os campos que existem no formulário são guardados; o resto é ignorado.
- Proteções automáticas: token CSRF, armadilha anti-robô, limite de envios aceitos por visitante (`limit`, padrão 5 a cada 10 minutos; o IP é guardado só como hash com sal).
- Com erro, a página volta com status 422, as mensagens em português e o que a pessoa digitou. Com sucesso, redireciona (303) e mostra `success` uma vez (`success=""` não mostra nada).
- `run="funcao"` chama uma função do site com a tabela de dados já validada. `recusar("motivo")` devolve o motivo como erro do formulário; se a função devolver um caminho (`"/obrigado"`), o visitante vai para lá. Com `save` e `run`, a função roda antes e pode alterar os dados que serão salvos.
- Um formulário só é aceito na página onde aparece (ou no layout).

### 10.4 Regiões ao vivo

O conteúdo de `<live>` é renderizado no servidor como o resto da página, e o navegador o atualiza sozinho, sem recarregar:
- `<live>` atualiza quando qualquer dado do site muda (no `nep serve`, por Server-Sent Events; no CGI, consultando `/_nep/versao` a cada 4 s);
- `<live every="10">` atualiza a cada 10 segundos (mínimo 2);
- formulários dentro de um `<live>` são enviados sem recarregar a página, e a região volta já com a mensagem ou os erros (`nep-recarregar` no `<form>` desliga isso);
- sem JavaScript, tudo continua funcionando com recarga normal.

O script `/_nep/vivo.js` é colocado na página automaticamente quando ela tem `<live>`.

### 10.5 APIs JSON

```html
<api route="/api/produtos/:id" method="GET">
  let p = db.find("produtos", params.id)
  if not p do abort(404, "produto não existe") end
  return p
</api>
```
O valor devolvido vira JSON (status 200). `abort(código, msg)` responde `{"erro": msg}`. O código global do site (os `<nep>` fora das páginas) roda antes, então as funções dele estão disponíveis. Em `POST`, `PUT` e `DELETE`, `request.json` traz o corpo JSON e `request.form` os campos de formulário; pedidos de outro site com corpo de formulário são recusados. `method="*"` aceita qualquer método.

### 10.6 Movimento

`<nep-config movimento="true">` coloca o script `/_nep/movimento.js` (o mesmo `nepenthes.js` que funciona sozinho em qualquer HTML): elementos surgem ao rolar, transição entre páginas e os atributos `nep-anim`, `nep-contar`, `nep-digitar`, `nep-paralaxe`, `nep-inclinar`, `nep-magnetico`, `nep-letreiro`, entre outros.

### 10.7 Sessão, painel e dados

- `session` é uma tabela guardada num cookie assinado (HMAC-SHA256, `HttpOnly`, `SameSite=Lax`): o visitante não consegue alterá-la. Só valores que viram JSON; máximo de 3,8 KB.
- `/_admin`: no primeiro acesso pede o código do arquivo `data/SETUP_TOKEN.txt` e cria a senha (PBKDF2-SHA256). Depois: coleções, busca, exclusão, exportação CSV para o Excel brasileiro (`;`, BOM, proteção contra fórmulas) e a página **LGPD**, que acha e apaga todos os dados de uma pessoa. Login com no máximo 5 erros a cada 15 minutos; a sessão do painel expira após 8 horas parada.
- Os dados ficam em `data/nep.db` (um registro JSON por linha, compactado de tempos em tempos), com trava de arquivo para vários processos ao mesmo tempo. A pasta `data/` nunca é entregue pela web.
- Cabeçalhos de segurança em toda resposta: `Content-Security-Policy`, `X-Frame-Options`, `X-Content-Type-Options`, `Referrer-Policy`, `Permissions-Policy` e HSTS em HTTPS.

### 10.8 Upload de arquivos

`<input type="file" name="foto" accept="image/*">` dentro de um `<form save>` ou `<form run>` funciona sem configurar nada: o formulário passa a enviar em `multipart/form-data` e o servidor confere cada arquivo.
- Tipos aceitos: JPEG, PNG, GIF, WebP e PDF, conferidos **pelo conteúdo**, não pelo nome (um HTML disfarçado de `.png` é recusado). `accept` restringe mais (`image/*`, `.pdf`, `application/pdf`).
- Tamanho: `data-max-mb="2"` (padrão 5, máximo 20; o envio inteiro tem até 25 MB). `required` vale para arquivos.
- O valor salvo é uma tabela `{nome, tipo, tamanho, url}`. Os arquivos ficam em `data/arquivos/` com nome aleatório e são entregues por `/_nep/arquivo/...` com `Content-Security-Policy: sandbox`.
- Por padrão o arquivo é **restrito**: só quem está logado no painel abre (currículos, documentos). Com `data-publico`, qualquer um abre (fotos do site).
- Apagar o registro (no painel, na página LGPD ou com `db.delete`) apaga o arquivo junto.

### 10.9 Busca e redes sociais (SEO)

- `<page description="..." image="/capa.jpg">`, ou `page.description` e `page.image` no código, viram `<meta name="description">`, a prévia em redes sociais (Open Graph e Twitter) e o endereço canônico. Etiquetas que o HTML já tem não são repetidas.
- `/sitemap.xml` lista as páginas públicas sem parâmetros; `/robots.txt` esconde `/_admin` e aponta o sitemap. Uma `<page>` com essas rotas, ou um arquivo de verdade, tem prioridade.
- `<page noindex>` pede aos buscadores para não indexar (páginas `auth` também) e fica fora do sitemap.
- `<nep-config url="https://www.exemplo.com.br">` fixa o endereço público usado nos links absolutos.

### 10.10 Visitas sem cookies

Toda página vista é contada em `data/visitas-AAAA-MM.log` com dia, caminho, site de origem (só o domínio) e se foi no celular. Visitantes únicos são estimados com um hash de IP e navegador misturado a um sal aleatório **trocado todo dia** e nunca guardado depois: não há cookie, IP nem identificação, e não dá para seguir alguém entre dias. Robôs, quem envia Do Not Track ou GPC e o dono logado no painel não contam. O painel mostra em `/_admin/visitas` o gráfico de 7, 30 ou 90 dias, as páginas mais vistas e de onde vieram os visitantes. `<nep-config estatisticas="false">` desliga a contagem.

### 10.11 Site gerenciável: conteúdo e textos editáveis (CMS)

Quem programa escreve o tema à mão, no próprio `.nep.html`. Quem cuida do site (sem saber código) publica e troca conteúdo em `/_admin`. Não há construtor de arrastar e soltar, e o layout continua sendo só o que o programador escreveu.

**Tipos de conteúdo.** Os campos são declarados como num formulário comum, com as mesmas regras de validação (10.3) e de upload (10.8):

```html
<conteudo nome="artigos" titulo="Artigos" item="artigo">
  <label>Título <input name="titulo" required maxlength="120"></label>
  <label>Resumo <textarea name="resumo" maxlength="300"></textarea></label>
  <label>Texto <textarea name="corpo" data-editor></textarea></label>
  <label>Capa <input type="file" name="capa" accept="image/*" data-publico></label>
</conteudo>

<page route="/artigos/:slug" conteudo="artigos">
  <h1>{{ item.titulo }}</h1>
  {{ markdown(item.corpo) }}
</page>
```
- `<conteudo>` fica fora das outras tags e não aparece na página. O painel ganha um menu com o `titulo`, com lista, **criar**, **editar** (com os valores já preenchidos, trocar ou remover arquivos), **apagar**, **rascunho/publicado** e o **endereço** (`slug`), criado a partir do título (`"Pão de Queijo!"` → `pao-de-queijo`), sem repetir. Em inglês: `<content name>`.
- Cada item guarda os campos mais `slug` e `publicado` (esses nomes, `id`, `created_at` e `updated_at` são reservados).
- **Rascunhos não aparecem para visitantes**: `db.all`, `db.find`, `db.where` e `db.count` os escondem, e a página do item responde 404. Quem está logado no painel vê tudo, para conferir antes de publicar.
- `<page conteudo="artigos">` precisa de uma rota com parte variável (`:slug`). A variável `item` chega pronta (achada pelo endereço ou pelo número); se não existir, a página 404 aparece sozinha. O título da página vem do campo `titulo` (ou `nome`), a descrição do `resumo` (ou do primeiro texto longo), a imagem de prévia do primeiro arquivo público de imagem, e cada item publicado entra no `/sitemap.xml`. `title`, `description` e `image` escritos na `<page>` têm prioridade.
- `data-editor` num `textarea` mostra a caixa grande, com a ajuda de formatação. Mostre o texto com `markdown(...)`.

**Textos editáveis.** `editavel="chave"` em qualquer tag deixa o conteúdo dela trocável, mantendo a tag, as classes e o layout do programador:

```html
<h1 editavel="capa.titulo">O que está acontecendo perto de você</h1>
<div editavel="sobre"><p>Texto padrão, com <strong>HTML</strong>.</p></div>
<img editavel="capa.imagem" src="/img/capa.jpg" alt="">
```
- O que está escrito no HTML é o padrão, e vale até alguém trocar. Em `/_admin/textos` cada trecho tem sua caixa e um botão para voltar ao original.
- Tags de texto (`h1`, `p`, `span`, `a`…) recebem texto simples (quebras de linha viram `<br>`). Tags de bloco (`div`, `section`, `article`, `aside`, `header`, `footer`, `blockquote`, `figure`…) recebem markdown. Em `img`, troca-se a imagem por upload.
- **Edição na própria página**: quem está logado no painel vê o botão "Editar textos" no canto do site; os trechos ficam contornados, e um clique num texto simples permite escrever ali mesmo (Enter salva, Esc cancela). Blocos e imagens abrem o painel no trecho certo. Visitantes não recebem nada disso.
- A chave usa letras minúsculas, números, `.`, `-` e `_`. A mesma chave pode aparecer em mais de um lugar (o texto é o mesmo). Um trecho editável não pode ter outro dentro, e `{{ }}` dentro dele vale só no texto padrão.
- No código, `texto("chave", "padrão")` devolve o texto trocado no painel (útil para atributos e títulos).

**Menus editáveis.** `menu-editavel="nome"` numa tag com links (normalmente `<nav>`) deixa quem cuida do site reorganizar o menu em `/_admin/textos`, uma linha por item no formato `Texto | /endereço`:

```html
<nav menu-editavel="principal">
  <a class="link" href="{{ url('/') }}">Início</a>
  <a class="link" href="/contato">Contato</a>
</nav>
```
- Os links escritos no HTML são o menu padrão (links gerados por `<each>`/`<if>` aparecem no site, mas não entram na lista do painel). Depois de trocado, o menu usa os atributos do primeiro link do programador (classes etc.), marca o item da página atual com `aria-current="page"` e aceita endereços do site, `https://`, `mailto:` e `tel:`. O painel mostra os endereços disponíveis, incluindo os itens publicados de cada `<conteudo>`.
- No código, `menu("principal")` devolve a lista `{texto, url, ativo}`, para montar menus com outro HTML.

**Construtor visual (arrastar e soltar).** O programador escolhe onde o cliente pode montar blocos visualmente; o resto do tema continua sendo código:

```html
<section construtor="home.faixa">conteúdo padrão</section>          <!-- área numa página -->
<textarea name="layout" data-construtor></textarea>                 <!-- campo num <conteudo> -->
{{ visual(item.layout) }}                                           <!-- mostra o campo -->
```
- No painel, essas caixas viram o **GrapesJS** (código aberto, licença BSD-3-Clause, embutido no `nep`, sem depender de serviço externo e com a telemetria desligada), em português: blocos de colunas, texto, link, imagem, vídeo e mapa, gerenciador de estilos, prévia em tablet e celular, e envio de imagens (as mesmas regras do upload, 10.8).
- O que é montado é guardado como `<style>css</style>html` e **limpo no servidor** ao mostrar: só tags e atributos de uma lista segura (sem `script`, eventos `on...`, formulários, `svg`, links `javascript:`), `iframe` só de YouTube, Vimeo e Google Maps (com `sandbox`), e o CSS fica preso dentro de `.nep-visual` (`body {}` vira o próprio bloco), então não estraga o tema. `@import` e `url()` perigosos são removidos.
- Um campo `data-construtor` aceita até 300 mil caracteres (sem `maxlength`).

**Usuários do painel.** Quem instalou o painel é o **dono** e vê tudo. Em `/_admin/usuarios` ele cria **editores** (usuário e senha, mín. 10 caracteres), troca a senha deles ou os remove. Editores entram com usuário e senha e só acessam o conteúdo, os textos e os menus: não veem coleções de formulários, arquivos restritos, visitas, a página LGPD, os usuários nem as páginas com `auth`. Trocar a senha ou remover um editor desconecta as sessões abertas dele. No site, `admin` é verdadeiro para o dono e para os editores (eles veem rascunhos).

**markdown(texto)** gera HTML seguro: `# título` (vira `<h2>`; `##` vira `<h3>`), `**negrito**`, `*itálico*`, `` `código` ``, `[texto](https://...)`, `![descrição](/imagem.jpg)`, listas com `-` ou `1.`, `> citação`, `---` e blocos com três crases. HTML digitado aparece como texto, e links só aceitam `http`, `https`, `mailto`, `tel` e endereços do próprio site.

### 10.12 E-mail

O site manda e-mail de verdade, sem biblioteca nem serviço de fora.

```html
<form save="pedidos" avisar="contato@empresa.com.br"
      confirmar="Recebemos seu pedido. Respondemos em até 1 dia útil."
      confirmar-assunto="Pedido recebido">
  <label>Nome <input name="nome" required></label>
  <label>E-mail <input name="email" type="email" required></label>
  <button>Enviar</button>
</form>
```

- `avisar="a@x.com, b@y.com"`: a cada envio aceito, os endereços recebem um e-mail com todos os campos e o link do painel. O "Responder" vai direto para o e-mail que a pessoa digitou. `avisar` sem valor (ou `avisar="dono"`) usa o endereço configurado no painel, em **E-mail**. Endereço inválido é erro de `nep check`.
- `confirmar="texto"`: responde para o primeiro campo `type=email` preenchido. O texto é sempre o do autor do site, nunca o que foi digitado, para o formulário não virar caminho de spam. `confirmar-assunto` troca o assunto.
- No código: `enviar_email({para: "a@x.com", assunto: "Oi", texto: "…", html: "<p>…</p>", responder_para: "b@y.com"})` ou `enviar_email(para, assunto, texto)`. `para` aceita lista ou "a@x.com, b@y.com" (até 20). Devolve `true` se o e-mail saiu.
- Por onde sai: pelo SMTP configurado no painel (porta 587 com STARTTLS ou 465 com TLS; ou variáveis `NEP_SMTP_HOST`, `NEP_SMTP_PORTA`, `NEP_SMTP_USUARIO`, `NEP_SMTP_SENHA`, `NEP_EMAIL_DE`); sem SMTP, pelo `sendmail` da hospedagem (cPanel e a maioria das VPS); sem nenhum dos dois, como no computador com `nep dev`, o e-mail fica guardado em `data/emails/*.eml`.
- Remetente padrão: `nao-responda@<domínio do site>`. Para cair menos no spam, use um e-mail do próprio domínio.
- Segurança: quebras de linha em endereço ou assunto são recusadas; limites de 300 avisos, 60 confirmações e 200 envios pelo código por hora; o formulário continua com o limite por visitante. Falha no envio não perde o formulário: tudo fica no histórico **E-mail** do painel (só o dono vê).

### 10.13 O site cuida de si (nível WordPress)

Sem nenhuma linha a mais no tema, todo tipo de `<conteudo>` ganha no painel:

- **Histórico de versões**: cada vez que um item é salvo, a versão anterior fica guardada (as 20 últimas), com data e quem editou. "Restaurar" volta o texto; a versão atual vai para o histórico. Imagens e anexos ficam os atuais.
- **Lixeira**: apagar um item manda para a lixeira por 30 dias, com os arquivos. "Restaurar" devolve com o mesmo número e endereço.
- **Publicação agendada**: o campo "Publicar em" esconde o item do site, da busca, do feed e do sitemap até a data e hora escolhidas. Fica gravado em `publicar_em`.
- **Biblioteca de mídia** (`/_admin/midia`): imagens e PDFs para usar nos textos e no construtor visual (as imagens aparecem no seletor do construtor).
- **Cópia de segurança** (`/_admin/backup`, só o dono): baixa um .zip com o banco e os arquivos; restaurar troca tudo e guarda os dados anteriores em `data/antes-da-restauracao-*`.

No tema:

- `{{ comentarios() }}` numa `<page conteudo="...">` mostra os comentários aprovados e o formulário (nome, e-mail que não aparece, texto). Tudo passa por moderação em `/_admin/comentarios` (dono e editores); o dono recebe aviso por e-mail se configurou "Avisos" (10.12). Proteções: CSRF, armadilha para robôs, 5 comentários a cada 10 minutos por visitante, texto sempre escapado. Classes para estilizar: `.nep-comentarios`, `.nep-comentario`, `.nep-comentar`.
- `buscar(termo, tipos, limite)` devolve uma lista de `{titulo, url, resumo, imagem, tipo, item}` dos itens publicados que contêm todas as palavras, sem diferenciar acento nem maiúscula; título pesa mais. `tipos` é opcional (`"artigos"` ou uma lista).
- `/feed.xml` (RSS 2.0) com os 20 itens mais novos de todos os tipos que têm página própria; o `<head>` ganha o link do feed. Nome do feed: `<nep-config nome="...">`.

Coleções internas (`_versoes`, `_lixeira`, `_comentarios`, `_midia`) começam com `_`: o código do site não as alcança e o painel não as lista como coleções.

### 10.14 IA com Qwen: os frameworks Nepenthes

O site fala com o **Qwen** por uma API compatível com a da OpenAI. Pode ser a nuvem da Alibaba (Model Studio/DashScope, padrão `https://dashscope-intl.aliyuncs.com/compatible-mode/v1`, modelo `qwen-plus`, com chave) ou um Qwen no seu servidor (Ollama ou vLLM, sem chave). A configuração fica no painel, em **IA** (só o dono), ou nas variáveis `NEP_IA_URL`, `NEP_IA_CHAVE` e `NEP_IA_MODELO`. A chave nunca é mostrada de volta. Sem IA configurada, nada quebra: as funções devolvem `nil` (ou o texto original, no caso da tradução).

Cada framework tem o nome de uma espécie de *Nepenthes*:

| Framework | Espécie | Para quê | No código |
|---|---|---|---|
| **lowii** | *N. lowii* | atendente do site: responde visitantes só com o conteúdo do site e mostra os links | `{{ lowii("Tire suas dúvidas") }}`; instruções do dono em `<nep-config lowii="...">` |
| **rajah** | *N. rajah* | escreve: rascunho, resumo, descrição para buscadores; botões "✨" no editor do painel | `rajah.escrever(assunto, {tom:, tamanho:})`, `rajah.resumo(texto, 200)`, `rajah.seo(texto)` |
| **ampullaria** | *N. ampullaria* | organiza o que chega: classifica e extrai dados de envios | `ampullaria.classificar(texto, ["Orçamento", "Dúvida"])`, `ampullaria.extrair(texto, ["nome", "data"])` |
| **villosa** | *N. villosa* | traduz, com cache (cada texto é traduzido e pago uma vez) | `villosa.traduzir(texto, "en")` |
| **truncata** | *N. truncata* | modera: comentários suspeitos (spam, ofensa, golpe) ficam separados e não geram aviso | automático em `comentarios()`; `truncata.moderar(texto)` → `{ok, motivo, verificado}` |

E `ia(pergunta, {sistema:, max:})` dá acesso direto ao modelo; `ia_ligada()` diz se a IA está configurada (por exemplo, `<if test="ia_ligada()">{{ lowii() }}</if>`).

Limites: 600 pedidos de IA por hora no site, 20 perguntas a cada 10 minutos por visitante no lowii, respostas curtas. O lowii recebe só trechos do site achados pela busca e ignora pedidos para mudar as regras. O histórico de uso (com tokens e erros) fica no painel.

## 11. Lua 5.4 na mesma máquina virtual

O `nep` roda programas Lua 5.4: `nep jogo.lua [args]`. O código Lua é lido por um leitor próprio e compilado para o mesmo bytecode do Nepenthes, então os dois se misturam:

- No Nepenthes, `import "util.lua" as util` carrega um arquivo Lua e devolve o que ele retorna (normalmente uma tabela de funções). Funções Lua chamadas pelo Nepenthes devolvem só o primeiro valor.
- No Lua, a tabela `nep` traz as funções globais do Nepenthes (`nep.money(19.9)`, `nep.slug("Olá")`) e `nep.import("json")` carrega módulos do Nepenthes.
- `require("mod")` procura `mod.lua` e `mod/init.lua` ao lado do arquivo e na pasta atual.

O que vale do Lua 5.4: toda a sintaxe (incluindo `goto`, `repeat`, `local x <const>`, inteiros e decimais separados, operadores de bits, `//`, `..`, textos longos `[[ ]]`), vários retornos e `...`, chamadas finais sem crescer a pilha, metatabelas com todos os metamétodos (`__index __newindex __call __tostring __eq __lt __le __len __concat __unm` e os aritméticos e de bits, `__pairs`, `__metatable`, `__name`), corrotinas, `pcall`/`xpcall`/`error` com nível, `load` com ambiente, `_G` e `_ENV`, e as bibliotecas `string` (com padrões e `format`), `table`, `math`, `os`, `io`, `utf8`, `coroutine` e `package`.

Diferenças conhecidas: mensagens de erro em português; `string.upper/lower` também convertem letras acentuadas; `collectgarbage` não ajusta o coletor; não há `string.pack`, `string.dump`, `debug`, bibliotecas em C nem metatabelas fracas (`__mode`) ou `__gc`/`__close`; a recursão comum (que não é chamada final) vai até 4000 níveis. No servidor web, `io.open`, `os.remove`, `os.exit`, `dofile` e `require` de arquivos ficam desligados, como o `fs` do Nepenthes.

O que o Nepenthes ganhou do Lua:

- **Metatabelas**: `setmetatable(t, mt)` vale também no Nepenthes, com `__index`, `__newindex`, `__call`, operadores (`__add`, `__sub`, `__mul`, `__div`, `__mod`, `__idiv`, `__unm`) e comparações (`__lt`, `__le`). Com isso dá para escrever classes:

```
let Conta = {}
Conta.__index = Conta
Conta.depositar = fn(self, v) do self.saldo += v; self end
let c = setmetatable({saldo: 0}, Conta)
c:depositar(50):depositar(25)     # obj:metodo(args) passa obj como primeiro argumento
```

- **`obj:metodo(args)`**: em tabelas, busca o campo (seguindo `__index`); em textos e listas, usa a função com esse nome (`"oi":upper()`).
- **Corrotinas**: `coroutine.create/resume/yield/wrap/status`. No Nepenthes, `resume` devolve o valor passado ao `yield` (vários viram lista) e erros da corrotina viram exceção (pegue com `try`).
- **Padrões de texto do Lua**: `match(s, padrão)` devolve a captura (várias viram lista; `nil` se não achar), `gmatch(s, padrão)` devolve a lista de todas, `gsub(s, padrão, troca, n)` troca por texto (`%1`), tabela ou função. Classes: `%a %d %l %s %u %w %x %p %c %g`, conjuntos `[...]`, `%b()`, `%f[...]`, âncoras `^ $`, repetições `* + - ?`.

A implementação é própria, em Go; nenhum código C do Lua foi incorporado. Crédito e licença MIT do Lua (PUC-Rio) em `LICENCA-LUA.txt`. Os testes em `testdata/lua` comparam a saída do `nep` com a do Lua 5.4 oficial.

## 9. Fora da versão 1 (planejado)

Gerenciador de pacotes, formatador (`nep fmt`), servidor de linguagem para editores (já existe a extensão de cores para VS Code em `editor/vscode`), login de visitantes e o módulo `mosca` para simulação do conectoma da Drosophila, usando dados de um repositório público com licença que permite uso comercial (candidato: hemibrain da Janelia, CC BY 4.0, com atribuição). Os dados do FlyWire não serão usados.
