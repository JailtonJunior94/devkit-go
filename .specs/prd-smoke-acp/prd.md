# PRD — strutil.CountVowels (smoke test runtime acp)

> **Status:** Aprovado
> **Data:** 2026-05-23
> **Objetivo:** Validar ponta-a-ponta o task-loop no runtime **acp** após a migração do root SDD
> para `.specs/`. Feature trivial e isolada, sem acoplamento ao código existente.

## 0. Objetivo

Adicionar a função pura `CountVowels` ao pacote `pkg/strutil`, com testes table-driven. Serve de carga
mínima para provar que o harness resolve bundles em `.specs/prd-*/` e executa tarefas no runtime ACP.

## 1. Escopo

- Novo arquivo `pkg/strutil/countvowels.go` (stdlib apenas).
- Uma função pura + teste table-driven.
- Nenhuma alteração em código existente do devkit-go.

## 2. Restrições

- Apenas stdlib Go; sem novas dependências em `go.mod`.
- Função pura (sem IO, sem estado global).
- Cobertura table-driven para caminho feliz e bordas (string vazia, maiúsculas/minúsculas, sem vogais).

## 3. Requisitos Funcionais

- **RF-01** — `CountVowels(s string) int`: conta as vogais ASCII (`a, e, i, o, u`) em `s`,
  case-insensitive. `CountVowels("") == 0`.

## 4. Critérios de Aceitação

- `go test ./pkg/strutil/...` verde, incluindo `CountVowels`.
- `go vet` sem warnings; nenhum arquivo fora de `pkg/strutil/` alterado.
