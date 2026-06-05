# Especificação Técnica — strutil.Truncate

> PRD: `.specs/prd-smoke-legacy/prd.md`. Stack: Go, stdlib apenas. Convenções: `AGENTS.md`.

## Resumo

Função pura nova `Truncate` em `pkg/strutil`. Sem domínio complexo, sem IO, sem dependências.
Objetivo: carga de validação do task-loop no runtime legacy com root SDD em `.specs/`.

## Design

- `truncate.go` — `func Truncate(s string, n int) string`. Converter `s` para `[]rune`; se `n <= 0`
  retornar `""`; se `len(runes) <= n` retornar `s`; caso contrário `string(runes[:n])`.

Arquivo acompanha `truncate_test.go` table-driven cobrindo: caso normal, string vazia, `n<=0`,
`n>=len`, unicode.

## Mapeamento Requisito → Componente → Teste

| Req | Componente | Teste |
|---|---|---|
| RF-01 | `pkg/strutil/truncate.go` | `truncate_test.go`: ("abcd",2)→"ab", ("",3)→"", ("ab",0)→"", ("ab",5)→"ab", ("áéí",2)→"áé" |

## Abordagem de Testes

Testes unitários table-driven (`R-TEST-001`), determinísticos, sem rede, sem mocks (função pura).

## Considerações

- Sem alteração em `go.mod` (stdlib only).
- Função pura: imutabilidade e ausência de efeitos colaterais.
