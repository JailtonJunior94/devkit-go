# Tarefa 2.0: nullable.String

<critical>Ler prd.md e techspec.md desta pasta — sua tarefa será invalidada se você pular</critical>

## Visão Geral

Implementar o tipo `String` no pacote `nullable` com API completa: construtores, métodos de acesso, serialização JSON e integração com `database/sql`. Esta tarefa estabelece o padrão que os demais tipos numéricos seguirão.

<requirements>
- Implementar `pkg/nullable/string.go` com o tipo `String`
- Implementar `pkg/nullable/string_test.go` cobrindo todos os cenários da matriz RF (techspec.md)
- Representação interna via `*string` (campo `val` não exportado)
- Zero value seguro: `String{}` representa ausência
- Todos os métodos definidos na seção "API pública por tipo" da techspec.md
- Nenhuma dependência externa além de `database/sql`, `database/sql/driver`, `encoding/json` da stdlib
</requirements>

## Subtarefas

- [ ] 2.1 Criar `pkg/nullable/string.go` com a struct `String { val *string }`
- [ ] 2.2 Implementar construtores: `StringOf(v string) String`, `StringEmpty() String`, `StringFromPtr(v *string) String`
- [ ] 2.3 Implementar métodos de acesso: `IsNull()`, `Get()`, `ValueOr()`, `Ptr()`
- [ ] 2.4 Implementar `Equal(other String) bool`
- [ ] 2.5 Implementar `String() string` (fmt.Stringer) — retorna `"<null>"` ou o valor
- [ ] 2.6 Implementar `MarshalJSON()` e `UnmarshalJSON()`
- [ ] 2.7 Implementar `Value()` (driver.Valuer) e `Scan()` (sql.Scanner) — delegar para `sql.NullString`
- [ ] 2.8 Criar `pkg/nullable/string_test.go` cobrindo todos os cenários da tabela de testes da techspec.md
- [ ] 2.9 Executar `go test ./pkg/nullable/... -race` e confirmar passa

## Detalhes de Implementação

Ver seções em `techspec.md`:
- **Estrutura interna** — `*string`, zero value seguro
- **Construtores** — `StringOf`, `StringEmpty`, `StringFromPtr`
- **API pública por tipo** — contrato completo de métodos
- **Implementação de Scan** — delegação para `sql.NullString`
- **Matriz Requisito → Decisão → Teste** — cenários RF-01 a RF-17

**Cenários obrigatórios de teste:**

| Cenário | Verificação |
|---|---|
| `String{}` → IsNull | `true` |
| `StringOf("")` → IsNull | `false` (zero-value presente ≠ null) |
| `StringEmpty()` → IsNull | `true` |
| `StringFromPtr(nil)` → IsNull | `true` |
| `StringFromPtr(&v)` → Get | `(v, true)` |
| `ValueOr` com presente | valor da string |
| `ValueOr` com ausente | fallback |
| `Equal` dois null | `true` |
| `Equal` dois iguais | `true` |
| `Equal` null vs presente | `false` |
| `Equal` reflexivo | `a.Equal(a)` |
| `Equal` simétrico | `a.Equal(b) == b.Equal(a)` |
| Marshal null | `[]byte("null")` |
| Marshal presente | bytes JSON da string |
| Unmarshal `null` | IsNull `true` |
| Unmarshal string | Get correto |
| Unmarshal número | error |
| Round-trip JSON | `StringOf(v) → Marshal → Unmarshal → ValueOr == v` |
| Scan nil | IsNull `true` |
| Scan string | Get correto |
| Value() null | `nil, nil` |
| Value() presente | string, nil |
| String() null | `"<null>"` |
| String() presente | o valor da string |

## Critérios de Sucesso

- `go test ./pkg/nullable/... -race` passa sem falhas
- `go vet ./pkg/nullable/...` passa
- Todos os cenários da tabela acima cobertos
- `StringOf("") != StringEmpty()` verificado explicitamente
- Nenhuma dependência externa

## Testes da Tarefa

- [ ] Testes unitários: `go test ./pkg/nullable/... -run TestString -race -v`
- [ ] Sem testes de integração (sem I/O externo)

<critical>SEMPRE CRIAR E EXECUTAR TESTES DA TAREFA ANTES DE CONSIDERAR A TAREFA COMO DONE</critical>

## Arquivos Relevantes
- `pkg/nullable/string.go` — a criar
- `pkg/nullable/string_test.go` — a criar
- `pkg/nullable/errors.go` — dependência (Tarefa 1.0)
- `pkg/vos/nullable_string.go` — referência de padrão existente
