# Tarefa 3.0: nullable.Int e nullable.Int64

<critical>Ler prd.md e techspec.md desta pasta — sua tarefa será invalidada se você pular</critical>

## Visão Geral

Implementar os tipos `Int` (wrapa `int`) e `Int64` (wrapa `int64`) no pacote `nullable`. Ambos seguem o mesmo padrão da Tarefa 2.0 com a distinção que `Int.Scan` delega para `sql.NullInt32` e `Int64.Scan` delega para `sql.NullInt64`.

<requirements>
- Implementar `pkg/nullable/int.go` com tipo `Int` wrappando `int` nativo
- Implementar `pkg/nullable/int64.go` com tipo `Int64` wrappando `int64`
- Implementar testes correspondentes em `int_test.go` e `int64_test.go`
- `Int.Scan` delega para `sql.NullInt32` com cast para `int`
- `Int64.Scan` delega para `sql.NullInt64`
- `Int.Value()` retorna `int64` (único tipo inteiro aceito por `driver.Value`)
- `Int64.Value()` retorna `int64` diretamente
- Zero value seguro para ambos os tipos
</requirements>

## Subtarafas

### Int
- [ ] 3.1 Criar `pkg/nullable/int.go` com `Int { val *int }`
- [ ] 3.2 Implementar construtores: `IntOf(v int) Int`, `IntEmpty() Int`, `IntFromPtr(v *int) Int`
- [ ] 3.3 Implementar métodos: `IsNull()`, `Get()`, `ValueOr()`, `Ptr()`, `Equal()`, `String()`
- [ ] 3.4 Implementar `MarshalJSON()` e `UnmarshalJSON()`
- [ ] 3.5 Implementar `Scan()` delegando para `sql.NullInt32` com cast `int32 → int`
- [ ] 3.6 Implementar `Value()` retornando `int64(*n.val)` para compatibilidade com `driver.Value`
- [ ] 3.7 Criar `pkg/nullable/int_test.go` com todos os cenários

### Int64
- [ ] 3.8 Criar `pkg/nullable/int64.go` com `Int64 { val *int64 }`
- [ ] 3.9 Implementar construtores: `Int64Of`, `Int64Empty`, `Int64FromPtr`
- [ ] 3.10 Implementar métodos: `IsNull()`, `Get()`, `ValueOr()`, `Ptr()`, `Equal()`, `String()`
- [ ] 3.11 Implementar `MarshalJSON()` e `UnmarshalJSON()`
- [ ] 3.12 Implementar `Scan()` delegando para `sql.NullInt64`
- [ ] 3.13 Implementar `Value()` retornando `*n.val` diretamente
- [ ] 3.14 Criar `pkg/nullable/int64_test.go` com todos os cenários
- [ ] 3.15 Executar `go test ./pkg/nullable/... -race` e confirmar passa

## Detalhes de Implementação

Ver seções em `techspec.md`:
- **Estrutura interna** — `*T`, zero value seguro
- **Implementação de Scan — estratégia por tipo** — Int usa `sql.NullInt32`, Int64 usa `sql.NullInt64`
- **Matriz Requisito → Decisão → Teste** — cenários aplicáveis a todos os tipos

**Atenção — `Int.Value()`:**
`driver.Value` aceita apenas `int64` para inteiros (não `int`). O método deve converter: `return int64(*n.val), nil`.

**Cenários obrigatórios de teste (replicar para Int e Int64):**

| Cenário | Verificação |
|---|---|
| Zero value → IsNull | `true` |
| `IntOf(0)` → IsNull | `false` (zero presente ≠ null) |
| `IntEmpty()` → IsNull | `true` |
| `IntFromPtr(nil)` → IsNull | `true` |
| `ValueOr` presente | valor |
| `ValueOr` ausente | fallback |
| `Equal` dois null | `true` |
| `Equal` mesmo valor | `true` |
| `Equal` valores diferentes | `false` |
| Marshal null | `[]byte("null")` |
| Marshal presente | bytes do número |
| Unmarshal `null` | IsNull `true` |
| Unmarshal número | Get correto |
| Unmarshal string | error |
| Round-trip JSON | `IntOf(42) → Marshal → Unmarshal → ValueOr == 42` |
| Scan nil | IsNull `true` |
| Scan valor inteiro | Get correto |
| Value() null | `nil, nil` |
| Value() presente | `int64`, nil |
| String() null | `"<null>"` |
| String() presente | representação decimal |

## Critérios de Sucesso

- `go test ./pkg/nullable/... -run 'TestInt' -race` passa
- `IntOf(0).IsNull() == false` verificado explicitamente
- `Int.Value()` retorna `int64` (não `int`)
- `go vet ./pkg/nullable/...` passa

## Testes da Tarefa

- [ ] Testes unitários: `go test ./pkg/nullable/... -run 'TestInt' -race -v`
- [ ] Sem testes de integração

<critical>SEMPRE CRIAR E EXECUTAR TESTES DA TAREFA ANTES DE CONSIDERAR A TAREFA COMO DONE</critical>

## Arquivos Relevantes
- `pkg/nullable/int.go` — a criar
- `pkg/nullable/int_test.go` — a criar
- `pkg/nullable/int64.go` — a criar
- `pkg/nullable/int64_test.go` — a criar
- `pkg/nullable/errors.go` — dependência (Tarefa 1.0)
- `pkg/vos/nullable_int.go` — referência (wrapa int64; padrão diferente do novo Int)
