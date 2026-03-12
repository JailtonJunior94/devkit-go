# Especificação Técnica — `pkg/nullable`

## Resumo Executivo

O pacote `pkg/nullable` entrega seis value objects nulábeis (`String`, `Int`, `Int64`, `Float32`, `Float64`, `Time`) com API mínima e uniforme, compatíveis com `encoding/json` e `database/sql`. A representação interna usa ponteiro (`*T`), tornando o zero value da struct seguro (nil = ausente) sem necessidade de campo `valid bool` separado. O pacote é independente, sem dependências externas, e coexiste com os tipos existentes em `pkg/vos/` sem conflito ou migração obrigatória.

O caso especial é `Time`: suporta layout JSON configurável em construção (`RFC3339` por padrão), resolvendo a necessidade de flexibilidade sem quebrar a API mínima.

---

## Arquitetura do Sistema

### Visão Geral dos Componentes

| Componente | Arquivo | Responsabilidade |
|---|---|---|
| `String` | `pkg/nullable/string.go` | VO nulável para `string` |
| `Int` | `pkg/nullable/int.go` | VO nulável para `int` |
| `Int64` | `pkg/nullable/int64.go` | VO nulável para `int64` |
| `Float32` | `pkg/nullable/float32.go` | VO nulável para `float32` |
| `Float64` | `pkg/nullable/float64.go` | VO nulável para `float64` |
| `Time` | `pkg/nullable/time.go` | VO nulável para `time.Time` com layout JSON |
| Erros | `pkg/nullable/errors.go` | Erros sentinela do pacote |
| Testes | `pkg/nullable/*_test.go` | Cobertura unitária por tipo |

Não há dependência entre os arquivos de tipos. Cada um importa apenas `stdlib`. O pacote não importa nenhum outro pacote interno do módulo.

---

## Design de Implementação

### Estrutura interna (todos os tipos exceto `Time`)

```go
// Exemplo representativo — padrão replicado em Int, Int64, Float32, Float64
type String struct {
    val *string
}
```

O campo `val` não é exportado. O zero value (`String{}`) representa ausência. Nenhum campo `valid bool` é necessário: `val == nil` é a única fonte de verdade para presença/ausência.

### Estrutura de `Time` (layout configurável)

```go
type Time struct {
    val    *time.Time
    layout string // vazio → RFC3339 no marshal
}
```

O campo `layout` é opaco ao consumer. É definido apenas na construção via `OfTime` ou `OfTimeLayout`.

### Construtores

Para os cinco tipos numéricos/string (padrão uniforme):

```go
// Of retorna instância com valor presente.
func Of(v string) String { return String{val: &v} }

// Empty retorna instância com ausência de valor.
func Empty() String { return String{} }

// FromPtr cria a partir de ponteiro — nil resulta em Empty.
func FromPtr(v *string) String {
    if v == nil {
        return String{}
    }
    return String{val: v}
}
```

Para `Time` (dois construtores explícitos):

```go
// Of cria Time com RFC3339 como layout JSON padrão.
func Of(t time.Time) Time { return Time{val: &t} }

// OfWithLayout cria Time com layout JSON customizado.
func OfWithLayout(t time.Time, layout string) Time {
    return Time{val: &t, layout: layout}
}

// Empty retorna Time sem valor.
func Empty() Time { return Time{} }
```

> Nota: `Of` e `Empty` são funções de pacote — não há conflito entre `nullable.Of` para tipos distintos porque cada tipo tem suas próprias funções em seu arquivo, acessadas como `nullable.String` → construtor é `nullable.Of` via tipo inferido pelo compilador. Na prática, o consumer escreve `nullable.Of("foo")` e o compilador resolve pelo tipo do argumento. Porém, como Go não suporta sobrecarga, cada tipo terá funções nomeadas com sufixo de tipo:

```go
// pkg/nullable/string.go
func Of(v string) String   { ... }
func Empty() String        { ... }  // CONFLITO se mesmo pacote

// SOLUÇÃO: construtores com tipo explícito no nome
func StringOf(v string) String     { ... }
func StringEmpty() String          { ... }
func IntOf(v int) Int              { ... }
func IntEmpty() Int                { ... }
func Int64Of(v int64) Int64        { ... }
func Int64Empty() Int64            { ... }
func Float32Of(v float32) Float32  { ... }
func Float32Empty() Float32        { ... }
func Float64Of(v float64) Float64  { ... }
func Float64Empty() Float64        { ... }
func TimeOf(t time.Time) Time                          { ... }
func TimeOfWithLayout(t time.Time, layout string) Time { ... }
func TimeEmpty() Time                                  { ... }
```

Essa convenção é idiomática em Go (ver `context.WithCancel`, `context.WithTimeout`) e evita nomes verbosos no ponto de uso: `nullable.IntOf(42)`, `nullable.StringEmpty()`.

### API pública por tipo (contrato uniforme)

Todos os tipos expõem o mesmo contrato de métodos:

```go
// IsNull retorna true quando o valor está ausente.
func (n String) IsNull() bool

// Get retorna (valor, true) quando presente, ("", false) quando ausente.
func (n String) Get() (string, bool)

// ValueOr retorna o valor ou o fallback fornecido.
func (n String) ValueOr(fallback string) string

// Ptr retorna *string ou nil.
func (n String) Ptr() *string

// Equal retorna true se ambos ausentes ou ambos com mesmo valor.
func (n String) Equal(other String) bool

// String implementa fmt.Stringer — "<null>" ou o valor formatado.
func (n String) String() string

// MarshalJSON — null ou valor serializado.
func (n String) MarshalJSON() ([]byte, error)

// UnmarshalJSON — null → ausente; valor → presente.
func (n *String) UnmarshalJSON(data []byte) error

// Value implementa driver.Valuer — nil ou valor concreto.
func (n String) Value() (driver.Value, error)

// Scan implementa sql.Scanner — nil → ausente; valor → presente.
func (n *String) Scan(value any) error
```

### Implementação de `Scan` — estratégia por tipo

Os tipos numéricos delegam para os correspondentes `sql.NullXxx` da stdlib (menor superfície de código, melhor cobertura de edge cases):

```go
// Int
func (n *Int) Scan(value any) error {
    var s sql.NullInt32
    if err := s.Scan(value); err != nil {
        return err
    }
    if !s.Valid {
        n.val = nil
        return nil
    }
    v := int(s.Int32)
    n.val = &v
    return nil
}

// Int64 — delega para sql.NullInt64
// Float32 — delega para sql.NullFloat64 com cast para float32
// Float64 — delega para sql.NullFloat64
// String  — delega para sql.NullString
// Time    — delega para sql.NullTime
```

> `Float32`: `sql.NullFloat64` é o único tipo float da stdlib. O `Scan` delega para ele e converte com `float32(v.Float64)`. O `Value()` converte de volta para `float64` (único tipo float aceito por `driver.Value`).

### Implementação de `MarshalJSON` / `UnmarshalJSON` para `Time`

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

> Consequência: ao deserializar `Time`, o `layout` deve ser o mesmo usado na serialização. Se o consumer usa layout customizado, ele deve construir o `Time` com `TimeOfWithLayout` antes de chamar `json.Unmarshal` (passando o ponteiro para o struct que já tem o layout definido). Isso é documentado em godoc.

### Erros sentinela

```go
// pkg/nullable/errors.go
var (
    ErrInvalidScan = errors.New("nullable: unsupported scan source type")
)
```

Erros de parse são wrappados com `fmt.Errorf(...%w...)` para preservar rastreabilidade.

### Modelos de Dados

Nenhum schema de banco de dados é criado. Os tipos são value objects puros — sem estado persistido por conta própria. A integração SQL ocorre via interfaces `sql.Scanner` / `driver.Valuer` diretamente nas structs do consumer (entidade de domínio ou DTO de repositório).

---

## Pontos de Integração

### `encoding/json`

Implementação via `json.Marshaler` / `json.Unmarshaler`. Nenhuma dependência de biblioteca JSON externa. Compatível com `encoding/json` da stdlib e com bibliotecas alternativas que respeitam as mesmas interfaces (ex.: `json-iterator`).

### `database/sql`

Implementação via `sql.Scanner` / `driver.Valuer`. Compatível com qualquer driver Go que respeite a interface `database/sql` (`lib/pq`, `pgx` via adaptador, `go-sqlite3`, `mysql`, etc.).

---

## Abordagem de Testes

### Testes Unitários

Um arquivo `_test.go` por tipo. Estratégia: table-driven com `testify/require` para asserções.

**Cenários obrigatórios por tipo:**

| Cenário | Método(s) verificado(s) |
|---|---|
| Zero value é null | `IsNull()`, `Get()`, `Ptr()` |
| `Of(zero-value)` não é null | `IsNull()`, `ValueOr()` |
| `Empty()` é null | `IsNull()`, `Ptr() == nil` |
| `FromPtr(nil)` → null | `IsNull()` |
| `FromPtr(&v)` → presente | `Get()`, `Equal()` |
| `ValueOr` com presente | valor retornado |
| `ValueOr` com ausente | fallback retornado |
| `Equal` reflexivo | `a.Equal(a)` |
| `Equal` simétrico | `a.Equal(b) == b.Equal(a)` |
| `Equal` dois null | `true` |
| `Equal` null vs presente | `false` |
| JSON marshal null | `[]byte("null")` |
| JSON marshal presente | bytes do valor |
| JSON unmarshal `null` | `IsNull() == true` |
| JSON unmarshal valor | `Get()` correto |
| JSON unmarshal tipo errado | `error != nil` |
| JSON round-trip | `Of(v) → Marshal → Unmarshal → ValueOr == v` |
| SQL Scan nil | `IsNull() == true` |
| SQL Scan valor | `Get()` correto |
| SQL Value() null | `nil, nil` |
| SQL Value() presente | valor correto |
| `String()` null | `"<null>"` |
| `String()` presente | valor formatado |

**Cenário adicional para `Time`:**
- Marshal/unmarshal com layout customizado round-trip
- Marshal RFC3339 default
- Parse inválido retorna `error`

### Testes de Integração

Não requeridos: os tipos não têm dependência de infraestrutura externa. O `sql.Scanner` é testado via `sql.NullXxx` da stdlib (não precisa de banco real).

### Testes E2E

Não aplicável — pacote biblioteca sem transporte HTTP ou banco de dados.

---

## Sequenciamento de Desenvolvimento

### Ordem de Build

1. **`pkg/nullable/errors.go`** — erros sentinela; sem dependências, base para os demais.
2. **`pkg/nullable/string.go` + `string_test.go`** — tipo mais simples; valida o padrão da API antes de replicar.
3. **`pkg/nullable/int.go` + `int_test.go`** — adiciona numérico; valida delegação para `sql.NullInt32`.
4. **`pkg/nullable/int64.go` + `int64_test.go`** — variante de int; mínima diferença de implementação.
5. **`pkg/nullable/float64.go` + `float64_test.go`** — valida delegação `sql.NullFloat64`.
6. **`pkg/nullable/float32.go` + `float32_test.go`** — adiciona cast `float32 ↔ float64`; valida precisão.
7. **`pkg/nullable/time.go` + `time_test.go`** — caso mais complexo (layout JSON); implementado por último.

### Dependências Técnicas

Nenhuma dependência de infraestrutura. Requer apenas Go 1.21+ (já disponível — módulo usa Go 1.26).

---

## Monitoramento e Observabilidade

Não aplicável. O pacote é uma biblioteca de value objects sem estado runtime, goroutines ou I/O. Nenhuma métrica, log ou trace é emitido — responsabilidade do consumer conforme `o11y.md`.

---

## Considerações Técnicas

### Decisões Chave

| Decisão | Escolha | Justificativa |
|---|---|---|
| Representação interna | `*T` (ponteiro) | Zero value seguro sem bool extra; alinhado com `pkg/vos/` existente |
| Construtores | `XxxOf` / `XxxEmpty` / `XxxFromPtr` | Go não permite sobrecarga; prefixo de tipo é idiomático e claro |
| `Scan` numérico | Delegação para `sql.NullXxx` | Reutiliza conversões testadas da stdlib; reduz código proprietário |
| `Time` JSON | Layout em campo da struct | Evita global mutável; permite round-trip seguro em contextos multi-layout |
| Pacote separado de `pkg/vos/` | `pkg/nullable/` dedicado | API diferente (`IsNull` vs `IsValid`, `Of` vs `NewNullable`); evita breaking change em consumers existentes |
| `Float32.Scan` | Cast de `float64` | Único tipo float em `driver.Value`; perda de precisão documentada em godoc |
| `Equal` por valor | Comparação `*a == *b` | Value objects são imutáveis por design; comparação por valor é correta |

### Riscos Conhecidos

| Risco | Mitigação |
|---|---|
| `Float32` perde precisão no round-trip SQL (`float64` → `float32`) | Documentado em godoc; usuários que precisam de precisão máxima devem usar `Float64` |
| `Time.UnmarshalJSON` requer layout pré-configurado no receptor | Documentado em godoc com exemplo explícito |
| `NullableInt` em `pkg/vos/` wrapa `int64`, gerando confusão com novo `Int` (wrapa `int`) | Os pacotes são distintos; `pkg/nullable/Int` wrapa `int` nativamente; sem conflito de import |

### Conformidade com Padrões

| Regra | Aplicação |
|---|---|
| `R-CODE-001` | Nomes exportados em `PascalCase`; construtores começam com verbo implícito no sufixo; sem abreviações obscuras |
| `R-ARCH-001` | Pacote sem dependências de camada de aplicação ou infraestrutura; value objects autovalidáveis |
| `R-ERR-001` | Erros sentinela em `errors.go` com prefixo `Err`; wrapping com `%w`; nenhum panic |
| `R-TEST-001` | Table-driven; sem estado compartilhado; sem I/O real; asserções com `testify/require` |
| `R-SEC-001` | Nenhum dado sensível; nenhum log; nenhuma rede |

### Arquivos Relevantes e Dependentes

**Criados:**
- `pkg/nullable/errors.go`
- `pkg/nullable/string.go` + `pkg/nullable/string_test.go`
- `pkg/nullable/int.go` + `pkg/nullable/int_test.go`
- `pkg/nullable/int64.go` + `pkg/nullable/int64_test.go`
- `pkg/nullable/float32.go` + `pkg/nullable/float32_test.go`
- `pkg/nullable/float64.go` + `pkg/nullable/float64_test.go`
- `pkg/nullable/time.go` + `pkg/nullable/time_test.go`

**Referenciados (não modificados):**
- `pkg/vos/nullable_string.go` — padrão de implementação de referência
- `pkg/vos/nullable_int.go` — padrão de delegação SQL
- `pkg/vos/nullable_float.go` — padrão float
- `pkg/vos/nulable_time.go` — padrão Time base

---

## Matriz Requisito → Decisão → Teste

| RF | Decisão técnica | Teste |
|---|---|---|
| RF-01 `Of` | `XxxOf(v T)` copia valor para heap via `&v` | `TestXxxOf_present` |
| RF-02 `Empty` | `XxxEmpty()` retorna zero struct | `TestXxxEmpty_isNull` |
| RF-03 `IsNull` | `return n.val == nil` | `TestIsNull_*` |
| RF-04 `ValueOr` | guard clause em `val == nil` | `TestValueOr_present`, `TestValueOr_absent` |
| RF-05 `Ptr` | retorna `n.val` diretamente | `TestPtr_nil`, `TestPtr_nonNil` |
| RF-06 marshal null | `return []byte("null"), nil` | `TestMarshalJSON_null` |
| RF-07 marshal presente | `json.Marshal(*n.val)` | `TestMarshalJSON_present` |
| RF-08 unmarshal null | detect `"null"` → nil | `TestUnmarshalJSON_null` |
| RF-09 unmarshal válido | `json.Unmarshal` no tipo T | `TestUnmarshalJSON_valid` |
| RF-10 unmarshal tipo errado | erro de `json.Unmarshal` propagado | `TestUnmarshalJSON_wrongType` |
| RF-11 Scan nil | `val = nil` quando source nil | `TestScan_nil` |
| RF-12 Scan valor | delegação `sql.NullXxx.Scan` | `TestScan_value` |
| RF-13 Scan tipo errado | erro de `sql.NullXxx.Scan` | `TestScan_unsupportedType` |
| RF-14 Value() null | `return nil, nil` | `TestValue_null` |
| RF-15 Value() presente | retorna valor como `driver.Value` | `TestValue_present` |
| RF-16 Equal | compare nil-nil e `*a == *b` | `TestEqual_*` |
| RF-17 String() | `"<null>"` ou `fmt.Sprint(*val)` | `TestString_null`, `TestString_present` |
