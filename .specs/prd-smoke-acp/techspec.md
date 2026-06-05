# Especificação Técnica — strutil.CountVowels

> PRD: `.specs/prd-smoke-acp/prd.md`. Stack: Go, stdlib apenas. Convenções: `AGENTS.md`.

## Resumo

Função pura nova `CountVowels` em `pkg/strutil`. Sem domínio complexo, sem IO, sem dependências.
Objetivo: carga de validação do task-loop no runtime ACP com root SDD em `.specs/`.

## Design

- `countvowels.go` — `func CountVowels(s string) int`. Iterar sobre `s`, normalizar com
  `unicode.ToLower`, incrementar quando o rune for `a/e/i/o/u`.

Arquivo acompanha `countvowels_test.go` table-driven cobrindo: caso normal, string vazia,
maiúsculas, sem vogais.

## Mapeamento Requisito → Componente → Teste

| Req | Componente | Teste |
|---|---|---|
| RF-01 | `pkg/strutil/countvowels.go` | `countvowels_test.go`: "hello"→2, ""→0, "AEIOU"→5, "xyz"→0 |

## Abordagem de Testes

Testes unitários table-driven (`R-TEST-001`), determinísticos, sem rede, sem mocks (função pura).

## Considerações

- Sem alteração em `go.mod` (stdlib only).
- Função pura: imutabilidade e ausência de efeitos colaterais.
