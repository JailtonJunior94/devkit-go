# Tarefa 1.0: CountVowels(s string) int + testes (RF-01)

> **Status:** done

<critical>Ler prd.md e techspec.md desta pasta antes de implementar.</critical>

## Visão Geral

Adicionar ao pacote `pkg/strutil` a função `CountVowels`, que conta vogais ASCII case-insensitive.

<requirements>
- `pkg/strutil/countvowels.go`, package `strutil`: `func CountVowels(s string) int`.
- Conta `a, e, i, o, u` case-insensitive. `CountVowels("") == 0`.
- Apenas stdlib. Função pura, sem IO.
</requirements>

## Subtarefas
- [x] 1.1 Criar `pkg/strutil/countvowels.go` com `CountVowels`.
- [x] 1.2 Criar `pkg/strutil/countvowels_test.go` table-driven: "hello"→2, ""→0, "AEIOU"→5, "xyz"→0.

## Critérios de Sucesso
- `go test ./pkg/strutil/...` verde para CountVowels.
- `go vet` sem warnings; nenhum arquivo fora de `pkg/strutil/` alterado.

## Skills Necessárias

Nenhuma além das auto-carregadas (governance + linguagem).

## Testes da Tarefa
- [x] Testes unitários

<critical>SEMPRE CRIAR E EXECUTAR TESTES DA TAREFA ANTES DE CONSIDERAR A TAREFA COMO `done`</critical>

## Arquivos Relevantes
- `pkg/strutil/countvowels.go`
- `pkg/strutil/countvowels_test.go`
