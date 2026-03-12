# Tarefa 4.0: nullable.Float64 e nullable.Float32

<critical>Ler prd.md e techspec.md desta pasta — sua tarefa será invalidada se você pular</critical>

## Visão Geral

Implementar os tipos `Float64` (wrapa `float64`) e `Float32` (wrapa `float32`). O caso de `Float32` tem complexidade adicional: `database/sql` não possui `sql.NullFloat32`, então `Scan` delega para `sql.NullFloat64` com cast, e `Value()` converte de volta para `float64`. Esse comportamento deve ser explicitamente testado.

<requirements>
- Implementar `pkg/nullable/float64.go` com tipo `Float64` wrappando `float64`
- Implementar `pkg/nullable/float32.go` com tipo `Float32` wrappando `float32`
- `Float64.Scan` delega para `sql.NullFloat64` sem cast
- `Float32.Scan` delega para `sql.NullFloat64` com cast `float64 → float32`
- `Float32.Value()` converte `float32 → float64` (driver.Value aceita apenas float64)
- Teste explícito de perda de precisão em `Float32` está documentado mas não é erro
- Testes correspondentes em `float64_test.go` e `float32_test.go`
</requirements>

## Subtarefas

### Float64
- [ ] 4.1 Criar `pkg/nullable/float64.go` com `Float64 { val *float64 }`
- [ ] 4.2 Implementar construtores: `Float64Of`, `Float64Empty`, `Float64FromPtr`
- [ ] 4.3 Implementar métodos: `IsNull()`, `Get()`, `ValueOr()`, `Ptr()`, `Equal()`, `String()`
- [ ] 4.4 Implementar `MarshalJSON()` e `UnmarshalJSON()`
- [ ] 4.5 Implementar `Scan()` delegando para `sql.NullFloat64`
- [ ] 4.6 Implementar `Value()` retornando `*n.val` diretamente
- [ ] 4.7 Criar `pkg/nullable/float64_test.go` com todos os cenários

### Float32
- [ ] 4.8 Criar `pkg/nullable/float32.go` com `Float32 { val *float32 }`
- [ ] 4.9 Implementar construtores: `Float32Of`, `Float32Empty`, `Float32FromPtr`
- [ ] 4.10 Implementar métodos: `IsNull()`, `Get()`, `ValueOr()`, `Ptr()`, `Equal()`, `String()`
- [ ] 4.11 Implementar `MarshalJSON()` e `UnmarshalJSON()`
- [ ] 4.12 Implementar `Scan()` delegando para `sql.NullFloat64` com cast `float64 → float32`
- [ ] 4.13 Implementar `Value()` retornando `float64(*n.val)` para compatibilidade com `driver.Value`
- [ ] 4.14 Adicionar godoc em `Float32` documentando comportamento de precisão no round-trip SQL
- [ ] 4.15 Criar `pkg/nullable/float32_test.go` com todos os cenários + teste de precisão
- [ ] 4.16 Executar `go test ./pkg/nullable/... -race` e confirmar passa

## Detalhes de Implementação

Ver seções em `techspec.md`:
- **Implementação de Scan — estratégia por tipo** — Float32 usa `sql.NullFloat64` com cast
- **Decisões Chave** — `Float32.Scan` com nota de precisão
- **Riscos Conhecidos** — `Float32` perde precisão; documentar em godoc

**Float32.String()** — usar `strconv.FormatFloat(float64(*n.val), 'f', -1, 32)` para representação precisa de 32 bits.

**Cenários obrigatórios de teste (replicar para Float64 e Float32):**

| Cenário | Verificação |
|---|---|
| Zero value → IsNull | `true` |
| `Float64Of(0.0)` → IsNull | `false` |
| `Float64Empty()` → IsNull | `true` |
| `Float64FromPtr(nil)` → IsNull | `true` |
| `ValueOr` presente | valor float |
| `ValueOr` ausente | fallback |
| `Equal` dois null | `true` |
| `Equal` mesmo valor | `true` |
| Marshal null | `[]byte("null")` |
| Marshal presente | bytes do número |
| Unmarshal `null` | IsNull `true` |
| Unmarshal float | Get correto |
| Unmarshal string | error |
| Round-trip JSON | `Float64Of(3.14) → Marshal → Unmarshal → ValueOr == 3.14` |
| Scan nil | IsNull `true` |
| Scan float64 | Get correto |
| Value() null | `nil, nil` |
| Value() presente | float64, nil |
| String() null | `"<null>"` |
| String() presente | representação decimal |

**Cenário adicional apenas para Float32:**

| Cenário | Verificação |
|---|---|
| `Scan(float64(1.5))` | `Get() == float32(1.5)` |
| `Value()` retorna float64 | `float64(float32Of(1.5).val)` |
| Round-trip SQL float32 | sem panic, precisão documentada |

## Critérios de Sucesso

- `go test ./pkg/nullable/... -run 'TestFloat' -race` passa
- `Float64Of(0.0).IsNull() == false` verificado
- `Float32.Value()` retorna `float64` (não `float32`)
- Godoc de `Float32` menciona comportamento de precisão SQL
- `go vet ./pkg/nullable/...` passa

## Testes da Tarefa

- [ ] Testes unitários: `go test ./pkg/nullable/... -run 'TestFloat' -race -v`
- [ ] Sem testes de integração

<critical>SEMPRE CRIAR E EXECUTAR TESTES DA TAREFA ANTES DE CONSIDERAR A TAREFA COMO DONE</critical>

## Arquivos Relevantes
- `pkg/nullable/float64.go` — a criar
- `pkg/nullable/float64_test.go` — a criar
- `pkg/nullable/float32.go` — a criar
- `pkg/nullable/float32_test.go` — a criar
- `pkg/nullable/errors.go` — dependência (Tarefa 1.0)
- `pkg/vos/nullable_float.go` — referência de padrão existente (wrapa float64)
