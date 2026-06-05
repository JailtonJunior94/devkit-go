# PRD — strutil.Truncate (smoke test runtime legacy)

> **Status:** Aprovado
> **Data:** 2026-05-23
> **Objetivo:** Validar ponta-a-ponta o task-loop no runtime **legacy** após a migração do root SDD
> para `.specs/`. Feature trivial e isolada, sem acoplamento ao código existente.

## 0. Objetivo

Adicionar a função pura `Truncate` ao pacote `pkg/strutil`, com testes table-driven. Serve de carga
mínima para provar que o harness resolve bundles em `.specs/prd-*/` e executa tarefas no runtime legacy.

## 1. Escopo

- Novo arquivo `pkg/strutil/truncate.go` (stdlib apenas).
- Uma função pura + teste table-driven.
- Nenhuma alteração em código existente do devkit-go.

## 2. Restrições

- Apenas stdlib Go; sem novas dependências em `go.mod`.
- Função pura (sem IO, sem estado global), rune-safe.
- Cobertura table-driven para caminho feliz e bordas (string vazia, n<=0, unicode, n>=len).

## 3. Requisitos Funcionais

- **RF-01** — `Truncate(s string, n int) string`: retorna os primeiros `n` runes de `s`.
  Se `n <= 0` retorna `""`. Se a contagem de runes de `s` for `<= n`, retorna `s` inalterado.
  Correto para unicode (operar sobre runes, não bytes).

## 4. Critérios de Aceitação

- `go test ./pkg/strutil/...` verde, incluindo `Truncate`.
- `go vet` sem warnings; nenhum arquivo fora de `pkg/strutil/` alterado.
