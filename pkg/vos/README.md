# Value Objects (VOs)

## Índice

1. [Introdução](#introdução)
2. [O que são Value Objects](#o-que-são-value-objects)
3. [Convenções e Princípios](#convenções-e-princípios)
4. [Value Objects Disponíveis](#value-objects-disponíveis)
   - [UUID](#uuid)
   - [ULID](#ulid)
   - [Currency](#currency)
   - [Money](#money)
   - [Percentage](#percentage)
   - [NullableInt](#nullableint)
   - [NullableString](#nullablestring)
   - [NullableBool](#nullablebool)
   - [NullableFloat](#nullablefloat)
   - [NullableTime](#nullabletime)
5. [Boas Práticas Gerais](#boas-práticas-gerais)
6. [Erros Comuns](#erros-comuns)
7. [Uso com Banco de Dados](#uso-com-banco-de-dados)
8. [Uso em APIs](#uso-em-apis)

---

## Introdução

Este pacote contém a implementação de **Value Objects** (VOs) seguindo os princípios de **Domain-Driven Design (DDD)** e **Clean Architecture**. Os Value Objects são componentes fundamentais do domínio da aplicação que representam conceitos de negócio com regras e validações específicas.

## O que são Value Objects

**Value Objects** são objetos imutáveis que representam conceitos do domínio através de seus valores, não de sua identidade. Diferentemente de Entidades, dois Value Objects são considerados iguais se todos os seus atributos forem iguais.

### Por que utilizá-los?

1. **Expressividade**: `Money` é mais expressivo que `float64`
2. **Segurança de Tipo**: Evita misturar conceitos diferentes (ex: somar `Percentage` com `Money` por engano)
3. **Validação Centralizada**: Regras de negócio ficam encapsuladas no próprio objeto
4. **Imutabilidade**: Uma vez criados, não podem ser modificados, evitando efeitos colaterais
5. **Precisão**: Evita problemas de arredondamento com tipos primitivos (ex: `float64` para dinheiro)

### Benefícios

- **Precisão matemática**: Operações financeiras sem perda de precisão
- **Imutabilidade garantida**: Thread-safe por padrão
- **Validação em tempo de construção**: Impossível criar objetos inválidos
- **Expressividade no domínio**: Código autodocumentado e alinhado com o negócio
- **Integração nativa**: Suporte completo para database/sql e encoding/json

---

## Convenções e Princípios

Todos os Value Objects deste pacote seguem os seguintes princípios:

### 1. Imutabilidade
- Uma vez criado, o valor não pode ser modificado
- Operações retornam novos Value Objects ao invés de modificar o existente

### 2. Validação
- Validação ocorre no construtor
- Impossível criar um Value Object inválido
- Construtores retornam `(VO, error)` ao invés de usar `panic`

### 3. Go Idiomático
- Compatível com `go vet`, `go fmt` e `golangci-lint`
- Não usa `panic` para controle de fluxo
- Thread-safe e nil-safe

### 4. Integração com Infraestrutura
- Implementa `driver.Valuer` para escrita no banco de dados
- Implementa `sql.Scanner` para leitura do banco de dados
- Implementa `json.Marshaler` e `json.Unmarshaler` para APIs JSON

### 5. Precisão Numérica
- Value Objects financeiros (`Money`, `Percentage`) usam inteiros internamente
- Evita problemas de arredondamento de ponto flutuante
- Representação decimal precisa para cálculos financeiros

---

## Value Objects Disponíveis

### UUID

#### Descrição
Representa um **Universally Unique Identifier (UUID)** versão 7. UUIDs são identificadores únicos globalmente que não requerem coordenação central. Este Value Object garante que apenas UUIDs válidos sejam utilizados no domínio.

**Por que não usar `string`?**
- Validação automática
- Impossível criar UUIDs inválidos
- Garante formato correto
- Type safety: evita misturar UUIDs com outras strings

#### Estrutura Interna
```go
type UUID struct {
    Value uuid.UUID  // github.com/google/uuid
}
```

**Garantias:**
- Imutável
- Validação automática (não aceita UUID nil)
- Thread-safe

#### Criação / Construtor

```go
// Gerar novo UUID V7
uuid, err := vos.NewUUID()
if err != nil {
    log.Fatal(err)
}

// A partir de string
uuid, err := vos.NewUUIDFromString("0191d8e8-7f3f-7000-8000-123456789012")
if err != nil {
    log.Fatal(err)
}

// A partir de uuid.UUID existente
existingUUID := uuid.New()
uuid, err := vos.NewFromUUID(existingUUID)
if err != nil {
    log.Fatal(err)
}
```

#### Operações Disponíveis

```go
// Verificar se está vazio
if uuid.IsEmpty() {
    // UUID é nil
}

// Converter para string
str := uuid.String()  // "0191d8e8-7f3f-7000-8000-123456789012"

// Obter UUID subjacente
rawUUID := uuid.UUID()

// Obter ponteiro seguro (nil-safe)
ptr := uuid.SafeUUID()  // Retorna nil se UUID for inválido
```

#### Uso com Banco de Dados

O Value Object UUID **implementa** `sql.Scanner` e pode ser usado diretamente em consultas, mas não implementa `driver.Valuer`. Para armazenar no banco, utilize o método `String()` ou `UUID()`:

```go
// INSERT
_, err := db.Exec(
    "INSERT INTO users (id, name) VALUES ($1, $2)",
    uuid.String(),  // ou uuid.UUID()
    "John Doe",
)

// SELECT
var id string
var name string
err := db.QueryRow("SELECT id, name FROM users WHERE id = $1", uuid.String()).
    Scan(&id, &name)

// Converter de volta
userUUID, err := vos.NewUUIDFromString(id)
```

#### Uso em APIs (Input / Output)

```go
// Estrutura de Request/Response
type CreateUserRequest struct {
    ID   vos.UUID `json:"id"`
    Name string   `json:"name"`
}

// Serialização JSON
user := CreateUserRequest{
    ID:   uuid,
    Name: "John Doe",
}
jsonData, _ := json.Marshal(user)
// {"id":"0191d8e8-7f3f-7000-8000-123456789012","name":"John Doe"}

// Deserialização JSON
var req CreateUserRequest
json.Unmarshal(jsonData, &req)
```

#### Boas Práticas
- Use UUIDs para identificadores de entidades que precisam ser únicos globalmente
- Prefira UUIDv7 para melhor performance em índices de banco de dados (ordenáveis)
- Sempre trate erros de criação de UUID

#### Erros Comuns
- ❌ Ignorar erro de criação: `uuid, _ := vos.NewUUID()`
- ❌ Usar string diretamente no domínio: `userID string`
- ✅ Sempre usar o Value Object: `userID vos.UUID`

---

### ULID

#### Descrição
Representa um **Universally Unique Lexicographically Sortable Identifier**. ULIDs combinam as vantagens de UUIDs (unicidade global) com ordenação lexicográfica baseada em timestamp, sendo ideais para chaves primárias ordenáveis por tempo.

**Por que não usar `string`?**
- Garante formato válido
- Ordenação temporal automática
- Thread-safe com crypto/rand
- Type safety

#### Estrutura Interna
```go
type ULID struct {
    Value ulid.ULID  // github.com/oklog/ulid/v2
}
```

**Garantias:**
- Imutável
- Thread-safe (usa crypto/rand)
- Lexicograficamente ordenável
- Não aceita zero value

#### Criação / Construtor

```go
// Gerar novo ULID
ulid, err := vos.NewULID()
if err != nil {
    log.Fatal(err)
}

// A partir de string
ulid, err := vos.NewULIDFromString("01ARZ3NDEKTSV4RRFFQ69G5FAV")
if err != nil {
    log.Fatal(err)
}
```

#### Operações Disponíveis

```go
// Converter para string
str := ulid.String()  // "01ARZ3NDEKTSV4RRFFQ69G5FAV"

// Validar
err := ulid.Validate()
```

#### Uso com Banco de Dados

ULID **não implementa** `driver.Valuer` nem `sql.Scanner`. Use o método `String()` para persistência:

```go
// INSERT
_, err := db.Exec(
    "INSERT INTO orders (id, amount) VALUES ($1, $2)",
    ulid.String(),
    1000,
)

// SELECT
var id string
err := db.QueryRow("SELECT id FROM orders WHERE id = $1", ulid.String()).
    Scan(&id)

// Converter de volta
orderULID, err := vos.NewULIDFromString(id)
```

#### Uso em APIs (Input / Output)

```go
type Order struct {
    ID     vos.ULID `json:"id"`
    Amount int64    `json:"amount"`
}

// Serialização - usar String() explicitamente ou implementar MarshalJSON
type OrderResponse struct {
    ID     string `json:"id"`
    Amount int64  `json:"amount"`
}

func toResponse(order Order) OrderResponse {
    return OrderResponse{
        ID:     order.ID.String(),
        Amount: order.Amount,
    }
}
```

#### Boas Práticas
- Use ULIDs quando precisar de IDs ordenáveis por tempo de criação
- Ideal para chaves primárias em bancos de dados (melhor performance que UUIDv4)
- Sempre valide ULIDs recebidos de fontes externas

#### Erros Comuns
- ❌ Usar ULIDs de fontes não thread-safe em ambiente concorrente
- ❌ Confundir ULID com UUID
- ✅ Preferir ULID para novos projetos que precisam de ordenação temporal

---

### Currency

#### Descrição
Representa um **código de moeda ISO 4217**. Este Value Object garante que apenas códigos de moeda válidos sejam utilizados, evitando erros de digitação e garantindo consistência em operações monetárias.

**Por que não usar `string`?**
- Validação automática contra ISO 4217
- Evita typos ("BLR" ao invés de "BRL")
- Type safety: evita passar moedas onde deveria ser outras strings
- Conjunto limitado e conhecido de valores

#### Estrutura Interna
```go
type Currency string

const (
    CurrencyBRL Currency = "BRL"  // Real Brasileiro
    CurrencyUSD Currency = "USD"  // Dólar Americano
    CurrencyEUR Currency = "EUR"  // Euro
    CurrencyGBP Currency = "GBP"  // Libra Esterlina
    CurrencyJPY Currency = "JPY"  // Iene Japonês
    // ... outras moedas
)
```

**Moedas suportadas:**
- BRL (Brazilian Real)
- USD (United States Dollar)
- EUR (Euro)
- GBP (British Pound Sterling)
- JPY (Japanese Yen)
- CAD (Canadian Dollar)
- AUD (Australian Dollar)
- CHF (Swiss Franc)
- CNY (Chinese Yuan)
- INR (Indian Rupee)
- MXN (Mexican Peso)
- ARS (Argentine Peso)

#### Criação / Construtor

```go
// A partir de string
currency, err := vos.NewCurrency("BRL")
if err != nil {
    log.Fatal(err)  // ErrInvalidCurrency
}

// Usar constantes (recomendado)
currency := vos.CurrencyBRL

// Case insensitive
currency, err := vos.NewCurrency("brl")  // OK, converte para "BRL"
```

#### Operações Disponíveis

```go
// Validar
if currency.IsValid() {
    // Moeda válida
}

// Converter para string
str := currency.String()  // "BRL"
```

#### Uso com Banco de Dados

**Implementa** `driver.Valuer` e `sql.Scanner`:

```go
// INSERT
currency := vos.CurrencyBRL
_, err := db.Exec(
    "INSERT INTO accounts (id, currency) VALUES ($1, $2)",
    1,
    currency,  // Armazena como "BRL"
)

// SELECT
var currency vos.Currency
err := db.QueryRow("SELECT currency FROM accounts WHERE id = $1", 1).
    Scan(&currency)
// currency é validado automaticamente
```

**Schema recomendado:**
```sql
CREATE TABLE accounts (
    id SERIAL PRIMARY KEY,
    currency VARCHAR(3) NOT NULL CHECK (currency IN ('BRL', 'USD', 'EUR', ...))
);
```

#### Uso em APIs (Input / Output)

**Implementa** `json.Marshaler` e `json.Unmarshaler`:

```go
type Account struct {
    ID       int          `json:"id"`
    Currency vos.Currency `json:"currency"`
}

// Serialização JSON
acc := Account{
    ID:       1,
    Currency: vos.CurrencyBRL,
}
jsonData, _ := json.Marshal(acc)
// {"id":1,"currency":"BRL"}

// Deserialização JSON (com validação)
var acc Account
err := json.Unmarshal(jsonData, &acc)
if err != nil {
    // Moeda inválida retorna erro
}
```

#### Boas Práticas
- Use constantes ao invés de strings literais
- Sempre valide moedas vindas de entrada de usuário
- Use Currency em conjunto com Money para garantir consistência

#### Erros Comuns
- ❌ Usar strings no domínio: `currency := "BRL"`
- ❌ Ignorar validação: `currency, _ := vos.NewCurrency(input)`
- ❌ Criar moedas personalizadas sem adicionar à validação
- ✅ Sempre usar o construtor ou constantes: `currency := vos.CurrencyBRL`

---

### Money

#### Descrição
Representa um **valor monetário** com precisão fixa e moeda associada. Este é um dos Value Objects mais críticos em sistemas financeiros, garantindo cálculos precisos sem erros de arredondamento de ponto flutuante.

**Por que não usar `float64`?**
- `float64` tem problemas de precisão: `0.1 + 0.2 != 0.3`
- Impossível representar valores decimais com precisão exata
- Operações financeiras exigem precisão matemática absoluta
- Money usa inteiros internamente (centavos) para precisão total

#### Estrutura Interna
```go
type Money struct {
    cents    int64     // Valor em centavos (imutável)
    currency Currency  // Moeda ISO 4217 (imutável)
}
```

**Características:**
- Escala fixa: 2 casas decimais (100 = R$ 1,00)
- Limite seguro: ~90 trilhões (int64)
- Imutável e thread-safe
- Validação de moeda obrigatória

#### Criação / Construtor

```go
// RECOMENDADO: A partir de centavos (máxima precisão)
money, err := vos.NewMoney(1050, vos.CurrencyBRL)  // R$ 10,50
if err != nil {
    log.Fatal(err)
}

// A partir de float (menos preciso, evite se possível)
money, err := vos.NewMoneyFromFloat(10.50, vos.CurrencyUSD)  // $10.50
if err != nil {
    log.Fatal(err)
}

// A partir de string
money, err := vos.NewMoneyFromString("10.50", vos.CurrencyEUR)  // €10.50
// Aceita: "10.50", "10,50", "10.5", "10"
```

#### Operações Disponíveis

**Aritméticas:**
```go
m1, _ := vos.NewMoney(1000, vos.CurrencyBRL)  // R$ 10,00
m2, _ := vos.NewMoney(500, vos.CurrencyBRL)   // R$ 5,00

// Adição
total, err := m1.Add(m2)  // R$ 15,00
if err != nil {
    // ErrCurrencyMismatch ou ErrOverflow
}

// Subtração
diff, err := m1.Subtract(m2)  // R$ 5,00

// Multiplicação
doubled, err := m1.Multiply(2)  // R$ 20,00

// Divisão (trunca)
half, err := m1.Divide(2)  // R$ 5,00
```

**Comparações:**
```go
if m1.GreaterThan(m2) {
    // m1 > m2
}

if m1.Equals(m2) {
    // m1 == m2 (mesmo valor E mesma moeda)
}

if m1.LessThanOrEqual(m2) {
    // m1 <= m2
}
```

**Utilitários:**
```go
cents := money.Cents()         // 1050
currency := money.Currency()   // CurrencyBRL
floatVal := money.Float()      // 10.50 (apenas para display!)

isZero := money.IsZero()       // false
isPositive := money.IsPositive()  // true
isNegative := money.IsNegative()  // false

absolute := money.Abs()        // Valor absoluto
negated := money.Negate()      // Inverte sinal
```

#### Uso com Banco de Dados

**Implementa** `driver.Valuer` e `sql.Scanner`:

**Formato de armazenamento:** `"cents:currency"` (ex: `"1050:BRL"`)

```go
// INSERT
money, _ := vos.NewMoney(1050, vos.CurrencyBRL)
_, err := db.Exec(
    "INSERT INTO transactions (id, amount) VALUES ($1, $2)",
    1,
    money,  // Salva como "1050:BRL"
)

// SELECT
var amount vos.Money
err := db.QueryRow("SELECT amount FROM transactions WHERE id = $1", 1).
    Scan(&amount)
// amount é reconstruído automaticamente
```

**Schema recomendado:**
```sql
-- Opção 1: Coluna única (TEXT)
CREATE TABLE transactions (
    id SERIAL PRIMARY KEY,
    amount TEXT NOT NULL  -- Formato: "cents:currency"
);

-- Opção 2: Colunas separadas (mais comum)
CREATE TABLE transactions (
    id SERIAL PRIMARY KEY,
    amount_cents BIGINT NOT NULL,
    currency VARCHAR(3) NOT NULL
);

-- Para opção 2, você precisa scanear manualmente:
var cents int64
var currency string
db.QueryRow("SELECT amount_cents, currency FROM transactions WHERE id = $1", 1).
    Scan(&cents, &currency)

curr, _ := vos.NewCurrency(currency)
money, _ := vos.NewMoney(cents, curr)
```

#### Uso em APIs (Input / Output)

**Implementa** `json.Marshaler` e `json.Unmarshaler`:

**Formato JSON:** `{"amount": "10.50", "currency": "BRL"}`

```go
type Transaction struct {
    ID     int       `json:"id"`
    Amount vos.Money `json:"amount"`
}

// Serialização
tx := Transaction{
    ID:     1,
    Amount: money,
}
jsonData, _ := json.Marshal(tx)
// {"id":1,"amount":{"amount":"10.50","currency":"BRL"}}

// Deserialização (com validação)
var tx Transaction
err := json.Unmarshal(jsonData, &tx)
if err != nil {
    // Moeda inválida ou formato incorreto
}
```

**Para APIs que precisam de formato simples:**
```go
type TransactionResponse struct {
    ID       int    `json:"id"`
    Amount   string `json:"amount"`    // "10.50"
    Currency string `json:"currency"`  // "BRL"
}

func toResponse(tx Transaction) TransactionResponse {
    return TransactionResponse{
        ID:       tx.ID,
        Amount:   fmt.Sprintf("%.2f", tx.Amount.Float()),
        Currency: tx.Amount.Currency().String(),
    }
}
```

#### Boas Práticas
- **SEMPRE** use `NewMoney(cents, currency)` para valores precisos
- **NUNCA** use `Float()` para cálculos, apenas para exibição
- Sempre trate erros de operações aritméticas (overflow, moedas diferentes)
- Use constantes de Currency para evitar erros
- Valide moedas em boundaries (API, input de usuário)

#### Erros Comuns
- ❌ Usar `float64` para dinheiro: `amount := 10.50`
- ❌ Ignorar erros: `total, _ := m1.Add(m2)`
- ❌ Somar moedas diferentes sem validação
- ❌ Usar `Float()` em cálculos: `result := money.Float() * 1.1`
- ✅ Usar centavos: `money, err := vos.NewMoney(1050, vos.CurrencyBRL)`
- ✅ Validar operações: `if err != nil { return err }`

---

### Percentage

#### Descrição
Representa um **valor percentual** com precisão fixa de 3 casas decimais. Essencial para cálculos financeiros envolvendo juros, taxas, descontos e comissões.

**Por que não usar `float64`?**
- Precisão fixa garante cálculos exatos
- Evita erros de arredondamento
- Type safety: evita confundir porcentagem com outros valores numéricos
- Semântica clara no domínio

#### Estrutura Interna
```go
type Percentage struct {
    value int64  // Valor escalado por 1000 (3 casas decimais)
}
```

**Características:**
- Escala: 1000 (12.345% = 12345)
- Precisão: 3 casas decimais
- Imutável e thread-safe
- Suporta valores negativos

#### Criação / Construtor

```go
// RECOMENDADO: A partir de valor escalado (máxima precisão)
pct, err := vos.NewPercentage(12345)  // 12.345%
if err != nil {
    log.Fatal(err)
}

// A partir de float
pct, err := vos.NewPercentageFromFloat(12.345)  // 12.345%

// A partir de string
pct, err := vos.NewPercentageFromString("12.345")  // 12.345%
// Aceita: "12.345", "12,345", "12.3%", "12"
```

#### Operações Disponíveis

**Aritméticas:**
```go
p1, _ := vos.NewPercentageFromFloat(10.0)   // 10.000%
p2, _ := vos.NewPercentageFromFloat(5.5)    // 5.500%

// Adição
total, err := p1.Add(p2)  // 15.500%

// Subtração
diff, err := p1.Subtract(p2)  // 4.500%

// Multiplicação
doubled, err := p1.Multiply(2)  // 20.000%

// Divisão
half, err := p1.Divide(2)  // 5.000%
```

**Aplicar a Money:**
```go
pct, _ := vos.NewPercentageFromFloat(10.0)      // 10%
money, _ := vos.NewMoney(10000, vos.CurrencyBRL)  // R$ 100,00

result, err := pct.Apply(money)  // R$ 10,00 (10% de R$ 100,00)
```

**Comparações:**
```go
if p1.GreaterThan(p2) { }
if p1.Equals(p2) { }
if p1.LessThanOrEqual(p2) { }
```

**Utilitários:**
```go
value := pct.Value()         // 12345 (escalado)
floatVal := pct.Float()      // 12.345 (apenas para display!)

isZero := pct.IsZero()       // false
isPositive := pct.IsPositive()  // true
isNegative := pct.IsNegative()  // false

absolute := pct.Abs()        // Valor absoluto
negated := pct.Negate()      // Inverte sinal
```

#### Uso com Banco de Dados

**Implementa** `sql.Scanner` e método `ValuerValue()`:

**Nota:** O método é `ValuerValue()` ao invés de `Value()` (que é usado para obter o valor escalado).

```go
// INSERT - use ValuerValue()
pct, _ := vos.NewPercentageFromFloat(12.345)
_, err := db.Exec(
    "INSERT INTO rates (id, interest_rate) VALUES ($1, $2)",
    1,
    pct,  // Salva como int64: 12345
)

// SELECT
var rate vos.Percentage
err := db.QueryRow("SELECT interest_rate FROM rates WHERE id = $1", 1).
    Scan(&rate)
// rate é reconstruído automaticamente
```

**Schema recomendado:**
```sql
CREATE TABLE rates (
    id SERIAL PRIMARY KEY,
    interest_rate BIGINT NOT NULL  -- Armazena valor escalado por 1000
);

-- Alternativa: NUMERIC se preferir float
CREATE TABLE rates (
    id SERIAL PRIMARY KEY,
    interest_rate NUMERIC(10, 3) NOT NULL  -- Scanner suporta float64
);
```

#### Uso em APIs (Input / Output)

**Implementa** `json.Marshaler` e `json.Unmarshaler`:

**Formato JSON:** String com 3 casas decimais: `"12.345"`

```go
type InterestRate struct {
    ID   int            `json:"id"`
    Rate vos.Percentage `json:"rate"`
}

// Serialização
rate := InterestRate{
    ID:   1,
    Rate: pct,
}
jsonData, _ := json.Marshal(rate)
// {"id":1,"rate":"12.345"}

// Deserialização (aceita string ou número)
var rate InterestRate
err := json.Unmarshal([]byte(`{"id":1,"rate":"12.345"}`), &rate)  // OK
err = json.Unmarshal([]byte(`{"id":1,"rate":12.345}`), &rate)     // OK
```

#### Boas Práticas
- Use `NewPercentage(value)` com valor escalado para máxima precisão
- Sempre valide resultados de operações aritméticas
- Use `Apply()` para calcular porcentagem de Money de forma segura
- Evite usar `Float()` em cálculos

#### Erros Comuns
- ❌ Usar `float64`: `discount := 10.5`
- ❌ Confundir valor escalado com valor real: `NewPercentage(10)` é 0.010%, não 10%
- ❌ Usar `Float()` em cálculos: `result := pct.Float() * amount`
- ✅ Usar construtor apropriado: `vos.NewPercentageFromFloat(10.0)` para 10%
- ✅ Usar `Apply()`: `result, err := discount.Apply(price)`

---

### NullableInt

#### Descrição
Representa um **int64 que pode ser nulo**. Diferentemente de ponteiros (`*int64`), oferece uma API mais segura e expressiva para lidar com valores opcionais.

**Por que não usar `*int64`?**
- API mais expressiva e segura
- Métodos utilitários (ValueOr, Get, etc.)
- Integração nativa com JSON e SQL
- Zero value seguro (representa nulo)
- Evita nil pointer dereference

#### Estrutura Interna
```go
type NullableInt struct {
    value *int64  // nil = valor nulo
}
```

**Características:**
- Zero value é seguro: representa valor nulo
- Imutável
- Nil-safe

#### Criação / Construtor

```go
// Valor válido
num := vos.NewNullableInt(42)

// A partir de ponteiro
var ptr *int64 = nil
num := vos.NewNullableIntFromPointer(ptr)  // Nulo

value := int64(42)
num := vos.NewNullableIntFromPointer(&value)  // Válido

// A partir de sql.NullInt64
sqlInt := sql.NullInt64{Int64: 42, Valid: true}
num := vos.NewNullableIntFromSQL(sqlInt)

// Valor nulo (zero value)
var num vos.NullableInt  // Nulo
```

#### Operações Disponíveis

```go
// Verificar validade
if num.IsValid() {
    // Tem valor
}

// Obter valor (idiomático em Go)
value, ok := num.Get()
if ok {
    fmt.Println(value)  // 42
}

// Valor ou padrão
value := num.ValueOr(0)  // Retorna 0 se nulo

// Obter ponteiro
ptr := num.Ptr()  // *int64 ou nil

// Conversões
sqlInt := num.ToSQL()       // sql.NullInt64
intVal := num.Int()         // int (0 se nulo)
intVal := num.IntOr(99)     // int (99 se nulo)
str := num.String()         // string ("" se nulo)
str := num.StringOr("N/A")  // string ("N/A" se nulo)
```

**Funções utilitárias globais:**
```go
// Conversão de/para ponteiro
nullable := vos.IntToNullable(&value)
ptr := vos.NullableToInt(nullable)

// Conversão de/para SQL
nullable := vos.SQLIntToNullable(sqlInt)
sqlInt := vos.NullableToSQLInt(nullable)

// String segura de ponteiro
str := vos.SafeIntToString(&value)        // "" se nil
str := vos.SafeIntToStringOr(&value, "0") // "0" se nil
```

#### Uso com Banco de Dados

**Implementa** `driver.Valuer` e `sql.Scanner`:

```go
// INSERT
age := vos.NewNullableInt(25)
_, err := db.Exec(
    "INSERT INTO users (id, age) VALUES ($1, $2)",
    1,
    age,  // Salva 25
)

// INSERT com valor nulo
var age vos.NullableInt  // Nulo
_, err := db.Exec(
    "INSERT INTO users (id, age) VALUES ($1, $2)",
    2,
    age,  // Salva NULL
)

// SELECT
var age vos.NullableInt
err := db.QueryRow("SELECT age FROM users WHERE id = $1", 1).
    Scan(&age)

if age.IsValid() {
    fmt.Printf("Age: %d\n", age.Int())
} else {
    fmt.Println("Age not provided")
}
```

#### Uso em APIs (Input / Output)

**Implementa** `json.Marshaler` e `json.Unmarshaler`:

```go
type User struct {
    ID   int              `json:"id"`
    Age  vos.NullableInt  `json:"age"`
}

// Valor válido
user := User{
    ID:  1,
    Age: vos.NewNullableInt(25),
}
json.Marshal(user)  // {"id":1,"age":25}

// Valor nulo
user = User{
    ID:  2,
    Age: vos.NullableInt{},  // Nulo
}
json.Marshal(user)  // {"id":2,"age":null}

// Deserialização
json.Unmarshal([]byte(`{"id":1,"age":25}`), &user)    // age válido
json.Unmarshal([]byte(`{"id":2,"age":null}`), &user)  // age nulo
```

#### Boas Práticas
- Use `Get()` quando precisar distinguir entre 0 e nulo
- Use `ValueOr()` quando tiver um valor padrão razoável
- Prefira NullableInt a `*int64` em structs de domínio
- Sempre verifique `IsValid()` antes de usar o valor

#### Erros Comuns
- ❌ Não verificar validade: `value := num.Int()` (retorna 0 se nulo!)
- ❌ Usar ponteiros no domínio: `Age *int64`
- ✅ Usar `Get()`: `if value, ok := num.Get(); ok { }`
- ✅ Usar Value Object: `Age vos.NullableInt`

---

### NullableString

#### Descrição
Representa uma **string que pode ser nula**. Oferece API rica para manipulação de strings opcionais com segurança e expressividade.

**Por que não usar `*string`?**
- API rica com métodos utilitários
- Distinção clara entre string vazia e nula
- Integração nativa com JSON e SQL
- Métodos de string convenientes (ToUpper, TrimSpace, etc.)

#### Estrutura Interna
```go
type NullableString struct {
    value *string  // nil = valor nulo
}
```

#### Criação / Construtor

```go
// Valor válido
str := vos.NewNullableString("Hello")

// A partir de ponteiro
var ptr *string = nil
str := vos.NewNullableStringFromPointer(ptr)  // Nulo

// A partir de sql.NullString
sqlStr := sql.NullString{String: "Hello", Valid: true}
str := vos.NewNullableStringFromSQL(sqlStr)

// Valor nulo (zero value)
var str vos.NullableString  // Nulo
```

#### Operações Disponíveis

```go
// Verificar validade
if str.IsValid() { }

// Verificar se é vazio OU nulo
if str.IsEmpty() { }  // true se nulo ou ""

// Obter valor
value, ok := str.Get()  // ("Hello", true) ou ("", false)
value := str.ValueOr("default")
value := str.String()   // "" se nulo
value := str.StringOr("N/A")

// Obter ponteiro
ptr := str.Ptr()  // *string ou nil

// Operações de string
upper := str.ToUpper()      // "HELLO" ou ""
lower := str.ToLower()      // "hello" ou ""
trimmed := str.TrimSpace()  // Remove espaços ou ""
length := str.Len()         // 5 ou 0

// Verificações
hasSubstr := str.Contains("ell")  // true ou false

// Conversões
sqlStr := str.ToSQL()  // sql.NullString
```

**Funções utilitárias globais:**
```go
nullable := vos.StringToNullable(&value)
ptr := vos.NullableToString(nullable)

nullable := vos.SQLStringToNullable(sqlStr)
sqlStr := vos.NullableToSQLString(nullable)

str := vos.SafeStringValue(&value)           // "" se nil
str := vos.SafeStringValueOr(&value, "N/A")  // "N/A" se nil
```

#### Uso com Banco de Dados

**Implementa** `driver.Valuer` e `sql.Scanner`:

```go
// INSERT
name := vos.NewNullableString("John Doe")
_, err := db.Exec(
    "INSERT INTO users (id, middle_name) VALUES ($1, $2)",
    1,
    name,
)

// INSERT nulo
var middleName vos.NullableString
_, err := db.Exec(
    "INSERT INTO users (id, middle_name) VALUES ($1, $2)",
    2,
    middleName,  // Salva NULL
)

// SELECT
var middleName vos.NullableString
err := db.QueryRow("SELECT middle_name FROM users WHERE id = $1", 1).
    Scan(&middleName)

fmt.Println(middleName.StringOr("(not provided)"))
```

#### Uso em APIs (Input / Output)

**Implementa** `json.Marshaler` e `json.Unmarshaler`:

```go
type User struct {
    ID         int                 `json:"id"`
    MiddleName vos.NullableString  `json:"middle_name"`
}

// Valor válido
user := User{
    ID:         1,
    MiddleName: vos.NewNullableString("Robert"),
}
json.Marshal(user)  // {"id":1,"middle_name":"Robert"}

// Valor nulo
user = User{
    ID:         2,
    MiddleName: vos.NullableString{},
}
json.Marshal(user)  // {"id":2,"middle_name":null}

// Deserialização
json.Unmarshal([]byte(`{"id":1,"middle_name":"Robert"}`), &user)  // válido
json.Unmarshal([]byte(`{"id":2,"middle_name":null}`), &user)      // nulo
```

#### Boas Práticas
- Use `IsEmpty()` quando quiser tratar nulo e string vazia da mesma forma
- Use `IsValid()` quando precisar distinguir entre nulo e vazio
- Use métodos utilitários (ToUpper, TrimSpace) para evitar nil pointer
- Prefira NullableString a `*string` em structs de domínio

#### Erros Comuns
- ❌ Confundir nulo com string vazia
- ❌ Não verificar validade antes de usar o valor
- ❌ Usar `*string` no domínio
- ✅ Usar `IsEmpty()` quando apropriado
- ✅ Usar `Get()` ou `StringOr()` para obter valor

---

### NullableBool

#### Descrição
Representa um **bool que pode ser nulo**. Essencial para campos opcionais booleanos onde é necessário distinguir entre `true`, `false` e "não definido".

**Por que não usar `*bool`?**
- Distinção clara entre false e nulo
- API expressiva (IsTrue, IsFalse)
- Integração nativa com JSON e SQL
- Semântica mais clara

#### Estrutura Interna
```go
type NullableBool struct {
    value *bool  // nil = valor nulo
}
```

#### Criação / Construtor

```go
// Valor válido
active := vos.NewNullableBool(true)
inactive := vos.NewNullableBool(false)

// A partir de ponteiro
var ptr *bool = nil
b := vos.NewNullableBoolFromPointer(ptr)  // Nulo

// A partir de sql.NullBool
sqlBool := sql.NullBool{Bool: true, Valid: true}
b := vos.NewNullableBoolFromSQL(sqlBool)

// Valor nulo
var b vos.NullableBool  // Nulo
```

#### Operações Disponíveis

```go
// Verificar validade
if b.IsValid() { }

// Obter valor
value, ok := b.Get()  // (true, true) ou (false, false)
value := b.ValueOr(false)
value := b.Bool()  // false se nulo (cuidado!)

// Verificações específicas
if b.IsTrue() {      // true se válido E true
    // Ativo
}

if b.IsFalse() {     // true se válido E false
    // Inativo
}

// Obter ponteiro
ptr := b.Ptr()  // *bool ou nil

// Conversões
sqlBool := b.ToSQL()        // sql.NullBool
str := b.String()           // "true", "false" ou ""
str := b.StringOr("N/A")    // "true", "false" ou "N/A"
```

**Funções utilitárias globais:**
```go
nullable := vos.BoolToNullable(&value)
ptr := vos.NullableToBool(nullable)

nullable := vos.SQLBoolToNullable(sqlBool)
sqlBool := vos.NullableToSQLBool(nullable)

str := vos.SafeBoolToString(&value)           // "" se nil
str := vos.SafeBoolToStringOr(&value, "N/A")  // "N/A" se nil
```

#### Uso com Banco de Dados

**Implementa** `driver.Valuer` e `sql.Scanner`:

```go
// INSERT
isVerified := vos.NewNullableBool(true)
_, err := db.Exec(
    "INSERT INTO users (id, is_verified) VALUES ($1, $2)",
    1,
    isVerified,
)

// INSERT nulo
var emailVerified vos.NullableBool
_, err := db.Exec(
    "INSERT INTO users (id, email_verified) VALUES ($1, $2)",
    2,
    emailVerified,  // Salva NULL
)

// SELECT
var isVerified vos.NullableBool
err := db.QueryRow("SELECT is_verified FROM users WHERE id = $1", 1).
    Scan(&isVerified)

if isVerified.IsTrue() {
    fmt.Println("User is verified")
} else if isVerified.IsFalse() {
    fmt.Println("User is not verified")
} else {
    fmt.Println("Verification status unknown")
}
```

#### Uso em APIs (Input / Output)

**Implementa** `json.Marshaler` e `json.Unmarshaler`:

```go
type User struct {
    ID         int               `json:"id"`
    IsVerified vos.NullableBool  `json:"is_verified"`
}

// Verdadeiro
user := User{
    ID:         1,
    IsVerified: vos.NewNullableBool(true),
}
json.Marshal(user)  // {"id":1,"is_verified":true}

// Falso
user = User{
    ID:         2,
    IsVerified: vos.NewNullableBool(false),
}
json.Marshal(user)  // {"id":2,"is_verified":false}

// Nulo
user = User{
    ID:         3,
    IsVerified: vos.NullableBool{},
}
json.Marshal(user)  // {"id":3,"is_verified":null}

// Deserialização
json.Unmarshal([]byte(`{"id":1,"is_verified":true}`), &user)   // true
json.Unmarshal([]byte(`{"id":2,"is_verified":false}`), &user)  // false
json.Unmarshal([]byte(`{"id":3,"is_verified":null}`), &user)   // nulo
```

#### Boas Práticas
- Use `IsTrue()` e `IsFalse()` ao invés de `Bool()` para evitar confusão
- Use `Get()` quando precisar de lógica ternária (true/false/null)
- Sempre documente o significado de null no seu domínio
- Prefira NullableBool a `*bool` em structs de domínio

#### Erros Comuns
- ❌ Usar `Bool()` e assumir que false significa "não definido"
- ❌ Não distinguir entre false e null
- ❌ Usar `*bool` no domínio
- ✅ Usar `IsTrue()` / `IsFalse()`: claros e explícitos
- ✅ Usar `Get()` quando precisar de 3 estados

---

### NullableFloat

#### Descrição
Representa um **float64 que pode ser nulo**. Útil para valores numéricos opcionais não-financeiros (medições, scores, coordenadas, etc.).

**Por que não usar `*float64`?**
- API rica com métodos utilitários
- Métodos matemáticos convenientes (Abs, Round, etc.)
- Verificações de validez (IsNaN, IsInf)
- Integração nativa com JSON e SQL

**Nota:** Para valores financeiros, use `Money` ao invés de NullableFloat.

#### Estrutura Interna
```go
type NullableFloat struct {
    value *float64  // nil = valor nulo
}
```

#### Criação / Construtor

```go
// Valor válido
rating := vos.NewNullableFloat(4.5)

// A partir de ponteiro
var ptr *float64 = nil
num := vos.NewNullableFloatFromPointer(ptr)  // Nulo

// A partir de sql.NullFloat64
sqlFloat := sql.NullFloat64{Float64: 4.5, Valid: true}
num := vos.NewNullableFloatFromSQL(sqlFloat)

// Valor nulo
var num vos.NullableFloat  // Nulo
```

#### Operações Disponíveis

```go
// Verificar validade
if num.IsValid() { }

// Obter valor
value, ok := num.Get()  // (4.5, true) ou (0.0, false)
value := num.ValueOr(0.0)
value := num.Float64()       // 0 se nulo
value := num.Float64Or(1.0)  // 1.0 se nulo

// Obter ponteiro
ptr := num.Ptr()  // *float64 ou nil

// Formatação
str := num.String()             // "" se nulo
str := num.StringOr("N/A")      // "N/A" se nulo
str := num.Format(2)            // "4.50" ou ""
str := num.FormatOr(2, "N/A")   // "4.50" ou "N/A"

// Verificações
if num.IsZero() { }      // true se válido e == 0
if num.IsPositive() { }  // true se válido e > 0
if num.IsNegative() { }  // true se válido e < 0
if num.IsNaN() { }       // true se válido e NaN
if num.IsInf() { }       // true se válido e infinito

// Operações matemáticas
abs := num.Abs()          // Valor absoluto (0 se nulo)
rounded := num.Round(2)   // Arredonda para 2 casas (0 se nulo)

// Formatação especial
currency := num.FormatCurrency("R$")  // "R$4.50" ou ""
percent := num.FormatPercentage()     // "4.50%" ou ""

// Conversões
sqlFloat := num.ToSQL()  // sql.NullFloat64
```

**Funções utilitárias globais:**
```go
nullable := vos.FloatToNullable(&value)
ptr := vos.NullableToFloat(nullable)

nullable := vos.SQLFloatToNullable(sqlFloat)
sqlFloat := vos.NullableToSQLFloat(nullable)

str := vos.SafeFloatToString(&value, 2)            // "" se nil
str := vos.SafeFloatToStringOr(&value, 2, "N/A")   // "N/A" se nil
```

#### Uso com Banco de Dados

**Implementa** `driver.Valuer` e `sql.Scanner`:

```go
// INSERT
rating := vos.NewNullableFloat(4.5)
_, err := db.Exec(
    "INSERT INTO products (id, rating) VALUES ($1, $2)",
    1,
    rating,
)

// INSERT nulo
var discount vos.NullableFloat
_, err := db.Exec(
    "INSERT INTO products (id, discount) VALUES ($1, $2)",
    2,
    discount,  // Salva NULL
)

// SELECT
var rating vos.NullableFloat
err := db.QueryRow("SELECT rating FROM products WHERE id = $1", 1).
    Scan(&rating)

fmt.Printf("Rating: %s\n", rating.Format(1))
```

#### Uso em APIs (Input / Output)

**Implementa** `json.Marshaler` e `json.Unmarshaler`:

```go
type Product struct {
    ID     int                `json:"id"`
    Rating vos.NullableFloat  `json:"rating"`
}

// Valor válido
product := Product{
    ID:     1,
    Rating: vos.NewNullableFloat(4.5),
}
json.Marshal(product)  // {"id":1,"rating":4.5}

// Valor nulo
product = Product{
    ID:     2,
    Rating: vos.NullableFloat{},
}
json.Marshal(product)  // {"id":2,"rating":null}

// Deserialização
json.Unmarshal([]byte(`{"id":1,"rating":4.5}`), &product)    // válido
json.Unmarshal([]byte(`{"id":2,"rating":null}`), &product)   // nulo
```

#### Boas Práticas
- Use para valores opcionais não-financeiros
- Sempre verifique `IsNaN()` e `IsInf()` se processar valores de fontes não confiáveis
- Use `Get()` quando precisar distinguir entre 0.0 e nulo
- Use métodos de formatação ao invés de manipular float diretamente

#### Erros Comuns
- ❌ Usar para valores financeiros (use `Money`)
- ❌ Não verificar `IsNaN()` e `IsInf()`
- ❌ Usar `Float64()` sem verificar validade
- ✅ Usar `Money` para finanças
- ✅ Usar `Get()` ou `ValueOr()` para obter valores

---

### NullableTime

#### Descrição
Representa um **time.Time que pode ser nulo**. Essencial para campos de data/hora opcionais como "deleted_at", "last_login", etc.

**Por que não usar `*time.Time`?**
- API expressiva para formatação
- Integração nativa com JSON (RFC3339) e SQL
- Zero value seguro
- Métodos de formatação convenientes

#### Estrutura Interna
```go
type NullableTime struct {
    time *time.Time  // nil = valor nulo
}
```

#### Criação / Construtor

```go
// Valor válido
now := vos.NewNullableTime(time.Now())

// A partir de ponteiro
var ptr *time.Time = nil
t := vos.NewNullableTimeFromPointer(ptr)  // Nulo

// A partir de sql.NullTime
sqlTime := sql.NullTime{Time: time.Now(), Valid: true}
t := vos.NewNullableTimeFromSQL(sqlTime)

// Valor nulo
var t vos.NullableTime  // Nulo
```

#### Operações Disponíveis

```go
// Verificar validade
if t.IsValid() { }

// Obter valor
value, ok := t.Get()  // (time.Time, true) ou (time.Time{}, false)
value := t.ValueOr(time.Now())

// Obter ponteiro
ptr := t.Ptr()  // *time.Time ou nil

// Formatação
str := t.Format(time.RFC3339)              // "" se nulo
str := t.FormatOr(time.RFC3339, "N/A")     // "N/A" se nulo
str := t.RFC3339()                         // "" se nulo

// Exemplos de formatação
str := t.Format("2006-01-02")              // "2024-01-15"
str := t.Format("02/01/2006 15:04:05")     // "15/01/2024 14:30:00"

// Conversões
sqlTime := t.ToSQL()  // sql.NullTime
```

**Funções utilitárias globais:**
```go
nullable := vos.TimeToNullable(&value)
ptr := vos.NullableToTime(nullable)

nullable := vos.SQLTimeToNullable(sqlTime)
sqlTime := vos.NullableToSQL(nullable)

str := vos.SafeFormatTime(&value, time.RFC3339)            // "" se nil
str := vos.SafeFormatTimeOr(&value, time.RFC3339, "N/A")   // "N/A" se nil
```

#### Uso com Banco de Dados

**Implementa** `driver.Valuer` e `sql.Scanner`:

```go
// INSERT
createdAt := vos.NewNullableTime(time.Now())
_, err := db.Exec(
    "INSERT INTO posts (id, created_at) VALUES ($1, $2)",
    1,
    createdAt,
)

// INSERT nulo (ex: deleted_at)
var deletedAt vos.NullableTime
_, err := db.Exec(
    "INSERT INTO posts (id, deleted_at) VALUES ($1, $2)",
    2,
    deletedAt,  // Salva NULL
)

// SELECT
var lastLogin vos.NullableTime
err := db.QueryRow("SELECT last_login FROM users WHERE id = $1", 1).
    Scan(&lastLogin)

if lastLogin.IsValid() {
    fmt.Printf("Last login: %s\n", lastLogin.Format("2006-01-02 15:04"))
} else {
    fmt.Println("Never logged in")
}
```

**Schema recomendado:**
```sql
CREATE TABLE users (
    id SERIAL PRIMARY KEY,
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    last_login TIMESTAMP NULL,
    deleted_at TIMESTAMP NULL
);
```

#### Uso em APIs (Input / Output)

**Implementa** `json.Marshaler` e `json.Unmarshaler`:

**Formato JSON:** RFC3339 (ISO 8601)

```go
type Post struct {
    ID        int               `json:"id"`
    CreatedAt vos.NullableTime  `json:"created_at"`
    DeletedAt vos.NullableTime  `json:"deleted_at,omitempty"`
}

// Valor válido
post := Post{
    ID:        1,
    CreatedAt: vos.NewNullableTime(time.Now()),
}
json.Marshal(post)
// {"id":1,"created_at":"2024-01-15T14:30:00Z","deleted_at":null}

// Valor nulo (com omitempty)
post = Post{
    ID:        2,
    CreatedAt: vos.NewNullableTime(time.Now()),
    DeletedAt: vos.NullableTime{},  // Será omitido no JSON
}
json.Marshal(post)
// {"id":2,"created_at":"2024-01-15T14:30:00Z"}

// Deserialização
json.Unmarshal([]byte(`{"id":1,"created_at":"2024-01-15T14:30:00Z"}`), &post)
json.Unmarshal([]byte(`{"id":2,"deleted_at":null}`), &post)
```

#### Boas Práticas
- Use para timestamps opcionais (deleted_at, last_login, etc.)
- Sempre use RFC3339 em APIs para compatibilidade internacional
- Use `omitempty` em JSON para timestamps nulos
- Armazene timestamps em UTC no banco de dados

#### Erros Comuns
- ❌ Não usar UTC ao armazenar no banco
- ❌ Usar formatos de data não padronizados em APIs
- ❌ Usar `*time.Time` no domínio
- ✅ Sempre converter para UTC: `time.Now().UTC()`
- ✅ Usar RFC3339 em APIs
- ✅ Usar NullableTime para campos opcionais

---

## Boas Práticas Gerais

### 1. Sempre Use Construtores
```go
// ❌ ERRADO: Instanciação direta
money := vos.Money{cents: 1000, currency: vos.CurrencyBRL}

// ✅ CORRETO: Usar construtor
money, err := vos.NewMoney(1000, vos.CurrencyBRL)
if err != nil {
    return err
}
```

### 2. Nunca Ignore Erros
```go
// ❌ ERRADO
money, _ := vos.NewMoney(cents, currency)

// ✅ CORRETO
money, err := vos.NewMoney(cents, currency)
if err != nil {
    return fmt.Errorf("failed to create money: %w", err)
}
```

### 3. Use Value Objects nas Camadas de Domínio
```go
// ❌ ERRADO: Tipos primitivos no domínio
type Order struct {
    ID          string
    TotalAmount float64
    Currency    string
}

// ✅ CORRETO: Value Objects
type Order struct {
    ID          vos.UUID
    TotalAmount vos.Money
}
```

### 4. Converta Apenas nos Boundaries
```go
// API Handler (boundary)
func CreateOrder(w http.ResponseWriter, r *http.Request) {
    var req struct {
        Amount   string `json:"amount"`
        Currency string `json:"currency"`
    }
    json.NewDecoder(r.Body).Decode(&req)

    // Converter para Value Objects no boundary
    currency, err := vos.NewCurrency(req.Currency)
    if err != nil {
        http.Error(w, "invalid currency", http.StatusBadRequest)
        return
    }

    money, err := vos.NewMoneyFromString(req.Amount, currency)
    if err != nil {
        http.Error(w, "invalid amount", http.StatusBadRequest)
        return
    }

    // Passar Value Objects para o domínio
    order, err := domain.CreateOrder(money)
    // ...
}
```

### 5. Imutabilidade: Crie Novos Objetos
```go
// Operações retornam NOVOS objetos
original, _ := vos.NewMoney(1000, vos.CurrencyBRL)
tax, _ := vos.NewMoney(100, vos.CurrencyBRL)

total, err := original.Add(tax)  // original NÃO é modificado
```

### 6. Use Métodos de Comparação
```go
// ❌ ERRADO: Comparação direta de structs
if money1 == money2 { }  // Não compila para a maioria dos VOs

// ✅ CORRETO: Usar métodos
if money1.Equals(money2) { }
if money1.GreaterThan(money2) { }
```

### 7. Prefira Precisão em Valores Financeiros
```go
// ❌ EVITE: Float para criação
money, _ := vos.NewMoneyFromFloat(10.50, vos.CurrencyBRL)

// ✅ MELHOR: Centavos para máxima precisão
money, _ := vos.NewMoney(1050, vos.CurrencyBRL)
```

### 8. Valide Entrada de Usuário nos Boundaries
```go
// No handler/controller
func CreateProduct(req CreateProductRequest) error {
    // Validar ANTES de criar Value Objects
    if req.Price <= 0 {
        return errors.New("price must be positive")
    }

    price, err := vos.NewMoney(req.Price, vos.CurrencyBRL)
    if err != nil {
        return fmt.Errorf("invalid price: %w", err)
    }

    // Usar no domínio
    product := domain.NewProduct(price)
    return productRepo.Save(product)
}
```

---

## Erros Comuns

### 1. Usar Tipos Primitivos no Domínio
```go
// ❌ ANTI-PATTERN
type Product struct {
    Price    float64
    Currency string
}

// ✅ CORRETO
type Product struct {
    Price vos.Money
}
```

### 2. Ignorar Erros de Validação
```go
// ❌ PERIGOSO
uuid, _ := vos.NewUUIDFromString(input)  // Pode falhar!

// ✅ SEGURO
uuid, err := vos.NewUUIDFromString(input)
if err != nil {
    return fmt.Errorf("invalid UUID: %w", err)
}
```

### 3. Quebrar Imutabilidade
```go
// ❌ ERRADO: Tentar modificar (não compila)
money.cents = 2000  // Campo privado!

// ✅ CORRETO: Criar novo objeto
newMoney, err := vos.NewMoney(2000, money.Currency())
```

### 4. Usar Float para Cálculos Financeiros
```go
// ❌ PERIGOSO: Perda de precisão
price := money.Float() * 1.1  // NUNCA FAÇA ISSO!

// ✅ CORRETO: Usar operações do Value Object
tax, _ := vos.NewPercentageFromFloat(10.0)
priceWithTax, err := tax.Apply(money)
```

### 5. Misturar Moedas sem Validação
```go
// ❌ PERIGOSO
total := brl.Add(usd)  // Retorna erro!

// ✅ CORRETO: Sempre tratar erro
total, err := brl.Add(usd)
if err != nil {
    return fmt.Errorf("cannot add different currencies: %w", err)
}
```

### 6. Não Distinguir Null de Zero/Empty
```go
// ❌ CONFUSO
age := nullableInt.Int()  // Retorna 0 se nulo OU se realmente for 0

// ✅ CLARO
if age, ok := nullableInt.Get(); ok {
    fmt.Printf("Age: %d\n", age)
} else {
    fmt.Println("Age not provided")
}
```

### 7. Usar Construtores Sem Validação
```go
// ❌ PERIGOSO: Criar diretamente
money := vos.Money{}  // Campo privado, não compila

// ✅ SEGURO: Sempre usar construtor
money, err := vos.NewMoney(1000, vos.CurrencyBRL)
```

---

## Uso com Banco de Dados

### Princípios Gerais

1. **Sempre armazene com precisão**: Use tipos numéricos apropriados (BIGINT, NUMERIC)
2. **Valide na leitura**: O scanner deve validar dados vindos do banco
3. **Normalize na escrita**: O valuer deve garantir formato consistente
4. **Use constraints**: Adicione CHECK constraints no banco quando possível

### Estratégias de Armazenamento

#### Money
```sql
-- Opção 1: Coluna única (formato "cents:currency")
CREATE TABLE transactions (
    id SERIAL PRIMARY KEY,
    amount TEXT NOT NULL
);

-- Opção 2: Colunas separadas (recomendado)
CREATE TABLE transactions (
    id SERIAL PRIMARY KEY,
    amount_cents BIGINT NOT NULL,
    currency VARCHAR(3) NOT NULL,
    CONSTRAINT valid_currency CHECK (currency IN ('BRL', 'USD', 'EUR', ...))
);
```

#### Percentage
```sql
CREATE TABLE tax_rates (
    id SERIAL PRIMARY KEY,
    rate BIGINT NOT NULL  -- Armazena valor * 1000
);
```

#### Nullable Types
```sql
CREATE TABLE users (
    id SERIAL PRIMARY KEY,
    age INT NULL,
    middle_name VARCHAR(100) NULL,
    is_verified BOOLEAN NULL,
    last_login TIMESTAMP NULL
);
```

#### UUID/ULID
```sql
-- UUID
CREATE TABLE orders (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid()
);

-- ULID (armazenar como TEXT)
CREATE TABLE orders (
    id TEXT PRIMARY KEY
);
```

### Migrations de Dados Legados

```sql
-- Migrar de float para Money (cents)
ALTER TABLE products ADD COLUMN price_cents BIGINT;
UPDATE products SET price_cents = ROUND(price * 100);
ALTER TABLE products DROP COLUMN price;
ALTER TABLE products RENAME COLUMN price_cents TO price;

-- Migrar de string para ULID
-- Validar dados antes da migração!
```

---

## Uso em APIs

### Serialização JSON

Todos os Value Objects implementam `json.Marshaler` e `json.Unmarshaler`:

```go
type CreateOrderRequest struct {
    Amount   vos.Money      `json:"amount"`
    Discount vos.Percentage `json:"discount"`
}

// Deserialização automática com validação
var req CreateOrderRequest
if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
    http.Error(w, "invalid request", http.StatusBadRequest)
    return
}
// req.Amount e req.Discount já estão validados!
```

### Validação em Boundaries

```go
// Handler HTTP
func CreateProductHandler(w http.ResponseWriter, r *http.Request) {
    var input struct {
        PriceCents int64  `json:"price_cents"`
        Currency   string `json:"currency"`
    }

    json.NewDecoder(r.Body).Decode(&input)

    // Validar e converter no boundary
    currency, err := vos.NewCurrency(input.Currency)
    if err != nil {
        http.Error(w, "invalid currency", http.StatusBadRequest)
        return
    }

    price, err := vos.NewMoney(input.PriceCents, currency)
    if err != nil {
        http.Error(w, "invalid price", http.StatusBadRequest)
        return
    }

    // Passar para camada de domínio
    product, err := service.CreateProduct(price)
    // ...
}
```

### Formatos de Resposta

```go
// Opção 1: Serialização automática
type ProductResponse struct {
    ID    vos.UUID  `json:"id"`
    Price vos.Money `json:"price"`
}
// JSON: {"id":"...","price":{"amount":"10.50","currency":"BRL"}}

// Opção 2: Formato customizado
type ProductResponse struct {
    ID       string `json:"id"`
    Price    string `json:"price"`
    Currency string `json:"currency"`
}

func toResponse(p Product) ProductResponse {
    return ProductResponse{
        ID:       p.ID.String(),
        Price:    fmt.Sprintf("%.2f", p.Price.Float()),
        Currency: p.Price.Currency().String(),
    }
}
// JSON: {"id":"...","price":"10.50","currency":"BRL"}
```

### Documentação OpenAPI/Swagger

```yaml
components:
  schemas:
    Money:
      type: object
      required:
        - amount
        - currency
      properties:
        amount:
          type: string
          example: "10.50"
        currency:
          type: string
          enum: [BRL, USD, EUR, GBP, JPY]
          example: "BRL"

    Percentage:
      type: string
      example: "12.345"
      description: "Percentage with 3 decimal places"

    UUID:
      type: string
      format: uuid
      example: "0191d8e8-7f3f-7000-8000-123456789012"
