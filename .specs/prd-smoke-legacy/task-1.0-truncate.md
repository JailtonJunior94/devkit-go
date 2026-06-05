# Tarefa 1.0: Truncate(s string, n int) string + testes (RF-01)

> **Status:** done

<critical>Ler prd.md e techspec.md desta pasta antes de implementar.</critical>

## Visão Geral

Adicionar ao pacote `pkg/strutil` a função `Truncate`, que retorna os primeiros `n` runes de `s`,
respeitando unicode.

<requirements>
- `pkg/strutil/truncate.go`, package `strutil`: `func Truncate(s string, n int) string`.
- `n <= 0` → `""`; `len(runes) <= n` → `s`; senão `string(runes[:n])`. Operar sobre `[]rune`.
- Apenas stdlib. Função pura, sem IO.
</requirements>

## Subtarefas
- [x] 1.1 Criar `pkg/strutil/truncate.go` com `Truncate`.
- [x] 1.2 Criar `pkg/strutil/truncate_test.go` table-driven: ("abcd",2)→"ab", ("",3)→"", ("ab",0)→"", ("ab",5)→"ab", ("áéí",2)→"áé".

## Critérios de Sucesso
- `go test ./pkg/strutil/...` verde para Truncate.
- `go vet` sem warnings; nenhum arquivo fora de `pkg/strutil/` alterado.

## Skills Necessárias

Nenhuma além das auto-carregadas (governance + linguagem).

## Testes da Tarefa
- [x] Testes unitários

<critical>SEMPRE CRIAR E EXECUTAR TESTES DA TAREFA ANTES DE CONSIDERAR A TAREFA COMO `done`</critical>

## Arquivos Relevantes
- `pkg/strutil/truncate.go`
- `pkg/strutil/truncate_test.go`
