# Nepenthes 0.1 — a linguagem

Nepenthes é uma linguagem de programação de uso geral, com sintaxe clara (`let`, `fn`, `do ... end`), o operador pipe `|>` para trabalhar com dados e uma biblioteca padrão pensada para o Brasil (`money`, datas `d/m/Y`, CSV com `;`).

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

## Instalar

Baixe o executável do seu sistema na página de Releases (ou compile com `go build ./cmd/nep`) e coloque numa pasta do PATH:

| Sistema | Arquivo |
|---|---|
| Windows | `nep-windows-amd64.exe` (renomeie para `nep.exe`) |
| Linux | `nep-linux-amd64` ou `nep-linux-arm64` |
| Mac (Apple Silicon) | `nep-macos-arm64` |
| Mac (Intel) | `nep-macos-amd64` |

No Linux/Mac: `chmod +x nep-*` e, no Mac, na primeira vez, clique com o botão direito > Abrir (o executável ainda não é assinado).

## Usar

```
nep                       modo interativo
nep programa.nep          executa
nep check programa.nep    só verifica a sintaxe
nep test pasta            roda os testes (*_test.nep, funções test_*)
nep disasm programa.nep   mostra o bytecode
```

## O que tem nesta pasta

- `SPEC.md`: a especificação da linguagem (versão 1, rascunho). É o documento que define o que é Nepenthes.
- `lang/`: a implementação de referência em Go: lexer, parser, compilador para bytecode, máquina virtual e biblioteca padrão.
- `lang/std/cerebro.nep`: módulo padrão escrito na própria Nepenthes (os modelos cognitivos do projeto original).
- `cmd/nep/`: o comando `nep`.
- `testdata/`: programas com a saída esperada, usados nos testes automáticos (`go test ./...`).
- `exemplos/`: programas de exemplo, incluindo os quatro `.nep` originais do projeto, que rodam sem mudanças na lógica.

## Compilar a partir do código

Precisa do Go 1.22 ou mais novo:

```
go test ./...
go build -o nep ./cmd/nep
```

## Desempenho

Na mesma máquina, `fib(30)` recursivo leva cerca de 0,2 s, na mesma faixa do Python 3.

## Próximos passos (fase 2)

Interpolação de texto, métodos (`lista.map(...)`), gerenciador de pacotes, `nep fmt`, extensão para VS Code, site de documentação e o módulo `mosca` (simulação do conectoma da Drosophila com dados públicos do hemibrain da Janelia, CC BY 4.0; sem dados do FlyWire).
