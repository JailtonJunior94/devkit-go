# Tarefa 5.0: nullable.Time

<critical>Ler prd.md e techspec.md desta pasta — sua tarefa será invalidada se você pular</critical>

## Visão Geral

Implementar o tipo `Time` no pacote `nullable`. Este é o tipo mais complexo: além do contrato padrão, suporta layout JSON configurável em construção (`RFC3339` por padrão via `TimeOf`, layout customizado via `TimeOfWithLayout`). O `UnmarshalJSON` usa o layout armazenado na struct, então o receptor deve ser construído com o layout correto antes de deserializar.

<requirements>
- Implementar `pkg/nullable/time.go` com tipo `Time { val *time.Time; layout string }`
- `TimeOf(t time.Time) Time` — cria com RFC3339 como layout padrão (campo layout vazio)
- `TimeOfWithLayout(t time.Time, layout string) Time` — cria com layout customizado
- `TimeEmpty() Time` — ausência, sem layout
- `MarshalJSON` usa `time.RFC3339` quando layout é vazio
- `UnmarshalJSON` usa layout armazenado na struct (ou RFC3339 se vazio)
- `Scan` delega para `sql.NullTime`
- `Value()` retorna `time.Time` (tipo aceito por `driver.Value`)
- Godoc documenta o comportamento de `UnmarshalJSON` com layout customizado
- Testes em `time_test.go` cobrindo RFC3339 padrão, layout customizado e edge cases
</requirements>

## Subtarefas

- [ ] 5.1 Criar `pkg/nullable/time.go` com `Time { val *time.Time; layout string }`
- [ ] 5.2 Implementar `TimeOf(t time.Time) Time` — layout vazio (RFC3339 implícito)
- [ ] 5.3 Implementar `TimeOfWithLayout(t time.Time, layout string) Time`
- [ ] 5.4 Implementar `TimeEmpty() Time` e `TimeFromPtr(t *time.Time) Time`
- [ ] 5.5 Implementar métodos padrão: `IsNull()`, `Get()`, `ValueOr()`, `Ptr()`, `Equal()`, `String()`
- [ ] 5.6 Implementar `MarshalJSON()` — `null` quando ausente; `time.RFC3339` quando layout vazio; layout customizado quando definido
- [ ] 5.7 Implementar `UnmarshalJSON()` — detectar `"null"` → ausente; parse com layout (ou RFC3339); retornar erro descritivo em falha de parse
- [ ] 5.8 Implementar `Scan()` delegando para `sql.NullTime`
- [ ] 5.9 Implementar `Value()` retornando `*n.val` (time.Time é aceito por driver.Value)
- [ ] 5.10 Adicionar godoc explicando comportamento de layout em `UnmarshalJSON`
- [ ] 5.11 Criar `pkg/nullable/time_test.go` com todos os cenários obrigatórios
- [ ] 5.12 Executar `go test ./pkg/nullable/... -race` e confirmar passa

## Detalhes de Implementação

Ver seções em `techspec.md`:
- **Estrutura de `Time`** — campos `val` e `layout`
- **Implementação de `MarshalJSON`/`UnmarshalJSON` para `Time`** — lógica detalhada com código de referência
- **Riscos Conhecidos** — `UnmarshalJSON` requer layout pré-configurado no receptor

**Fragmento de referência para `MarshalJSON`:**
```go
func (n Time) MarshalJSON() ([]byte, error) {
    if n.val == nil {
        return []byte("null"), nil
    }
    layout := n.layout
    if layout == "" {
        layout = time.RFC3339
    }
    return json.Marshal(n.val.Format(layout))
}
```

**Fragmento de referência para `UnmarshalJSON`:**
```go
func (n *Time) UnmarshalJSON(data []byte) error {
    if string(data) == "null" {
        n.val = nil
        return nil
    }
    layout := n.layout
    if layout == "" {
        layout = time.RFC3339
    }
    var s string
    if err := json.Unmarshal(data, &s); err != nil {
        return err
    }
    t, err := time.Parse(layout, s)
    if err != nil {
        return fmt.Errorf("nullable.Time: parse %q with layout %q: %w", s, layout, err)
    }
    n.val = &t
    return nil
}
```

**`String()` para Time:** retornar `"<null>"` quando ausente ou `t.Format(time.RFC3339)` quando presente (independente do layout JSON).

**`Equal()` para Time:** usar `t1.Equal(t2)` da stdlib (compara instante no tempo, independente de timezone).

**Cenários obrigatórios de teste:**

| Cenário | Verificação |
|---|---|
| `Time{}` → IsNull | `true` |
| `TimeOf(t)` → IsNull | `false` |
| `TimeEmpty()` → IsNull | `true` |
| `TimeFromPtr(nil)` → IsNull | `true` |
| `ValueOr` presente | valor time.Time |
| `ValueOr` ausente | fallback |
| `Equal` dois null | `true` |
| `Equal` mesmo instante (timezone diferente) | `true` (via `time.Equal`) |
| `Equal` instantes diferentes | `false` |
| Marshal null | `[]byte("null")` |
| Marshal presente (RFC3339 padrão) | string RFC3339 entre aspas |
| Marshal presente com layout customizado | string no layout entre aspas |
| Unmarshal `null` | IsNull `true` |
| Unmarshal RFC3339 válido | Get correto |
| Unmarshal com layout customizado (receptor pré-configurado) | Get correto |
| Unmarshal layout mismatch | `error != nil` com mensagem descritiva |
| Unmarshal não-string JSON | `error != nil` |
| Round-trip JSON RFC3339 | `TimeOf(t) → Marshal → Unmarshal → Equal(t)` |
| Round-trip JSON layout customizado | `TimeOfWithLayout(t, l) → Marshal → Unmarshal(receptor com l) → Equal(t)` |
| Scan nil | IsNull `true` |
| Scan time.Time | Get correto |
| Value() null | `nil, nil` |
| Value() presente | time.Time, nil |
| String() null | `"<null>"` |
| String() presente | formato RFC3339 |

## Critérios de Sucesso

- `go test ./pkg/nullable/... -run 'TestTime' -race` passa
- Round-trip JSON RFC3339 passa
- Round-trip JSON com layout customizado passa
- `TimeOf(time.Time{}).IsNull() == false` verificado (zero time ≠ null)
- `UnmarshalJSON` retorna erro descritivo em falha de parse
- `go vet ./pkg/nullable/...` passa

## Testes da Tarefa

- [ ] Testes unitários: `go test ./pkg/nullable/... -run 'TestTime' -race -v`
- [ ] Sem testes de integração

<critical>SEMPRE CRIAR E EXECUTAR TESTES DA TAREFA ANTES DE CONSIDERAR A TAREFA COMO DONE</critical>

## Arquivos Relevantes
- `pkg/nullable/time.go` — a criar
- `pkg/nullable/time_test.go` — a criar
- `pkg/nullable/errors.go` — dependência (Tarefa 1.0)
- `pkg/vos/nulable_time.go` — referência de padrão existente (sem layout configurável)
