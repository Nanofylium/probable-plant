# Especificação da linguagem Nepenthes — versão 1 (rascunho 0.1)

Este documento define o que um programa Nepenthes significa. Qualquer implementação (o executável `nep` em Go, o runtime web em PHP) deve seguir estas regras. Onde uma implementação ainda diverge, isso está marcado como **[divergência]**.

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
- Comandos: `nep` (interativo), `nep arquivo.nep`, `nep check`, `nep test`, `nep disasm`, `nep version`.

## 9. Fora da versão 1 (planejado)

Interpolação de texto, métodos em textos e listas (`lista.map(...)`), `match`, classes/protótipos, gerenciador de pacotes, formatador (`nep fmt`), servidor de linguagem para editores, e o módulo `mosca` para simulação do conectoma da Drosophila, usando dados de um repositório público com licença que permite uso comercial (candidato: hemibrain da Janelia, CC BY 4.0, com atribuição). Os dados do FlyWire não serão usados.
