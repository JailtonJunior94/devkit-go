# vos (Value Objects)

Domain-Driven Design value objects for Money, Percentage, identifiers (UUID, ULID), and nullable types with precision guarantees and type safety.

## Introduction

### Problem It Solves

**Primitive Obsession**: Using basic types (`string`, `int64`, `float64`) for domain concepts leads to:
- Lost business rules and invariants
- Precision errors in financial calculations
- Type confusion (is this value in cents or dollars?)
- Currency mismatch bugs (adding USD to BRL)
- Null handling boilerplate

Value Objects solve this by:
- **Encapsulating business logic** (Money knows how to add/subtract with currency validation)
- **Ensuring precision** (no floating-point errors in financial calculations)
- **Type safety** (can't accidentally pass a string where UUID is expected)
- **Immutability** (thread-safe by design)

### When to Use

✅ **Use Value Objects when:**
- Working with money or financial calculations
- Need precise percentage calculations
- Domain concepts have business rules (currency, scale, validation)
- Need nullable types with clear semantics
- Building DDD-based applications
- Require database persistence with type safety

❌ **Don't use when:**
- Simple data transfer without business logic
- Performance is absolutely critical (though overhead is minimal)
- Working with external APIs that require primitives (convert at boundary)

---

## Architecture

### Value Object Principles

All VOs in this package follow DDD principles:

1. **Immutable**: Once created, values cannot be changed
2. **Thread-Safe**: Can be safely shared across goroutines
3. **Self-Validating**: Invalid states are impossible to construct
4. **Equality by Value**: Two VOs with same value are equal
5. **No Identity**: VOs don't have IDs, only their value matters

### Package Structure

```
vos/
├── uuid.go           # UUID v7 identifiers
├── ulid.go           # ULID identifiers (lexicographically sortable)
├── money.go          # Money with currency (precision: 2 decimals)
├── percentage.go     # Percentage (precision: 3 decimals)
├── currency.go       # ISO 4217 currency codes
├── nullable_int.go   # Nullable int64
├── nullable_string.go  # Nullable string
├── nullable_bool.go    # Nullable bool
├── nullable_float.go   # Nullable float64
├── nulable_time.go     # Nullable time.Time (note: typo in filename)
└── errors.go         # Common errors
```

### Design Decisions

1. **Integer-Based Precision**: Money and Percentage use `int64` internally to avoid floating-point errors
2. **Scale Factors**: Money (scale 2 = cents), Percentage (scale 3 = thousandths)
3. **Pointer-Based Nullables**: Use `*T` internally for true three-state (null, false, true for bool)
4. **Immutable Operations**: All operations return new instances instead of mutating

---

## API Reference

### Money

```go
type Money struct {
    cents    int64    // Internal: scaled by 100
    currency Currency // ISO 4217 code
}

// Constructors
NewMoney(cents int64, currency Currency) (Money, error)
NewMoneyFromFloat(value float64, currency Currency) (Money, error)
NewMoneyFromString(value string, currency Currency) (Money, error)

// Accessors
Cents() int64
Currency() Currency
Float() float64  // ⚠️ For display only, not calculations

// Operations
Add(other Money) (Money, error)
Subtract(other Money) (Money, error)
Multiply(factor int64) (Money, error)
Divide(divisor int64) (Money, error)

// Comparisons
Equals(other Money) bool
GreaterThan(other Money) bool
LessThan(other Money) bool
IsZero() bool
IsPositive() bool
IsNegative() bool

// Utilities
Abs() Money
Negate() Money
String() string  // "10.50 BRL"

// Persistence
MarshalJSON() ([]byte, error)
UnmarshalJSON(data []byte) error
Value() (driver.Value, error)  // Database
Scan(value any) error           // Database
```

### Percentage

```go
type Percentage struct {
    value int64  // Internal: scaled by 1000
}

// Constructors
NewPercentage(value int64) (Percentage, error)  // value = 12345 means 12.345%
NewPercentageFromFloat(value float64) (Percentage, error)
NewPercentageFromString(value string) (Percentage, error)

// Accessors
ScaledValue() int64  // Returns raw scaled value
Float() float64      // ⚠️ For display only

// Operations
Add(other Percentage) (Percentage, error)
Subtract(other Percentage) (Percentage, error)
Multiply(factor int64) (Percentage, error)
Divide(divisor int64) (Percentage, error)
Apply(money Money) (Money, error)  // Calculate percentage of Money

// Comparisons (same as Money)

// Utilities
String() string  // "12.345%"
```

### Currency

```go
type Currency string

// Supported Currencies
const (
    CurrencyBRL Currency = "BRL"  // Brazilian Real
    CurrencyUSD Currency = "USD"  // US Dollar
    CurrencyEUR Currency = "EUR"  // Euro
    CurrencyGBP Currency = "GBP"  // British Pound
    CurrencyJPY Currency = "JPY"  // Japanese Yen
    // ... and more
)

// Constructor
NewCurrency(code string) (Currency, error)

// Methods
IsValid() bool
String() string
```

### UUID

```go
type UUID struct {
    Value uuid.UUID  // Uses google/uuid
}

// Constructors
NewUUID() (UUID, error)                      // Generates UUID v7
NewUUIDFromString(value string) (UUID, error)
NewFromUUID(value uuid.UUID) (UUID, error)

// Methods
Validate() error
String() string
IsEmpty() bool
UUID() uuid.UUID
SafeUUID() *uuid.UUID  // Returns nil if empty
```

### ULID

```go
type ULID struct {
    Value ulid.ULID  // Uses oklog/ulid
}

// Constructors
NewULID() (ULID, error)                      // Generates new ULID
NewULIDFromString(value string) (ULID, error)

// Methods
Validate() error
String() string
```

### Nullable Types

All nullable types share the same pattern:

```go
type NullableInt struct { value *int64 }

// Constructors
NewNullableInt(v int64) NullableInt
NewNullableIntFromPointer(v *int64) NullableInt
NewNullableIntFromSQL(n sql.NullInt64) NullableInt

// Methods
IsValid() bool
Get() (int64, bool)  // Idiomatic Go: value, ok
ValueOr(defaultValue int64) int64
Ptr() *int64
ToSQL() sql.NullInt64

// Conversion
String() string
Int() int  // For NullableInt

// Persistence
Scan(value any) error
Value() (driver.Value, error)
MarshalJSON() ([]byte, error)
UnmarshalJSON(data []byte) error
```

**Available nullable types:**
- `NullableInt` (int64)
- `NullableString`
- `NullableBool`
- `NullableFloat` (float64)
- `NullableTime` (time.Time)

---

## Examples

### Money: Basic Operations

```go
// Create money values
price, _ := vos.NewMoney(1050, vos.CurrencyBRL)  // 10.50 BRL
discount, _ := vos.NewMoney(200, vos.CurrencyBRL)  // 2.00 BRL

// Arithmetic operations
total, _ := price.Subtract(discount)  // 8.50 BRL
doubled, _ := price.Multiply(2)       // 21.00 BRL
half, _ := price.Divide(2)            // 5.25 BRL

// Comparisons
if price.GreaterThan(discount) {
    fmt.Println("Price is higher than discount")
}

// Display
fmt.Println(total.String())  // "8.50 BRL"
fmt.Printf("%.2f", total.Float())  // 8.50
```

### Money: Currency Safety

```go
brl, _ := vos.NewMoney(1000, vos.CurrencyBRL)
usd, _ := vos.NewMoney(1000, vos.CurrencyUSD)

// ❌ This returns ErrCurrencyMismatch
_, err := brl.Add(usd)
if err != nil {
    fmt.Println(err)  // "currency mismatch: cannot operate on different currencies"
}

// ✅ Only works with same currency
discount, _ := vos.NewMoney(100, vos.CurrencyBRL)
total, _ := brl.Subtract(discount)  // OK
```

### Money: Database Persistence

```go
type Order struct {
    ID    string
    Total vos.Money
}

// Saving
order := Order{
    ID:    "123",
    Total: vos.NewMoney(5000, vos.CurrencyBRL),
}

// Money is stored as "5000:BRL" string in database
db.Exec("INSERT INTO orders (id, total) VALUES ($1, $2)", order.ID, order.Total)

// Loading
var order Order
db.QueryRow("SELECT id, total FROM orders WHERE id = $1", "123").Scan(&order.ID, &order.Total)

// ✅ order.Total is correctly parsed back to Money{cents: 5000, currency: BRL}
```

### Percentage: Calculations

```go
// Create percentage
tax, _ := vos.NewPercentageFromFloat(10.0)  // 10%
discount, _ := vos.NewPercentageFromFloat(15.5)  // 15.5%

// Apply to money
price, _ := vos.NewMoney(10000, vos.CurrencyBRL)  // 100.00 BRL
taxAmount, _ := tax.Apply(price)  // 10.00 BRL
discountAmount, _ := discount.Apply(price)  // 15.50 BRL

finalPrice, _ := price.Add(taxAmount)
finalPrice, _ = finalPrice.Subtract(discountAmount)
// finalPrice = 94.50 BRL

// Percentage arithmetic
total, _ := tax.Add(discount)  // 25.5%
fmt.Println(total.String())  // "25.500%"
```

### Percentage: Precision

```go
// ✅ No precision loss with scaled integers
p1, _ := vos.NewPercentage(12345)  // 12.345%
p2, _ := vos.NewPercentage(1)      // 0.001%
sum, _ := p1.Add(p2)  // Exactly 12.346%

// ⚠️ Be careful with float constructor (precision loss)
p3, _ := vos.NewPercentageFromFloat(12.345)  // May have rounding
// Prefer: NewPercentage(12345) for exact values
```

### UUID and ULID

```go
// UUID v7 (timestamp-ordered)
userID, _ := vos.NewUUID()
fmt.Println(userID.String())  // "018d5e8b-3a3e-7890-abcd-1234567890ab"

// Parse existing UUID
id, _ := vos.NewUUIDFromString("018d5e8b-3a3e-7890-abcd-1234567890ab")
if !id.IsEmpty() {
    fmt.Println("Valid UUID")
}

// ULID (lexicographically sortable, timestamp-based)
orderID, _ := vos.NewULID()
fmt.Println(orderID.String())  // "01H4S7J9K0ABCDEFGHIJKLMNOP"

// ULID from string
ulid, _ := vos.NewULIDFromString("01H4S7J9K0ABCDEFGHIJKLMNOP")
```

### Nullable Types: Three-State Logic

```go
// Three states: null, false, true (for bool)
unknown := vos.NullableBool{}      // null
yes := vos.NewNullableBool(true)   // true
no := vos.NewNullableBool(false)   // false

// Check validity
if unknown.IsValid() {
    // Never executes, unknown is null
}

// Get with ok idiom
if value, ok := yes.Get(); ok {
    fmt.Println("Value is", value)  // "Value is true"
}

// Default value if null
value := unknown.ValueOr(false)  // Returns false

// Use cases
if yes.IsTrue() {
    fmt.Println("Definitely true")
}

if no.IsFalse() {
    fmt.Println("Definitely false")
}
```

### Nullable Types: Database Integration

```go
type User struct {
    ID    vos.UUID
    Name  vos.NullableString
    Age   vos.NullableInt
    Email vos.NullableString
}

// Insert with null values
user := User{
    ID:    vos.NewUUID(),
    Name:  vos.NewNullableString("John Doe"),
    Age:   vos.NullableInt{},  // NULL in database
    Email: vos.NewNullableString("john@example.com"),
}

db.Exec(`INSERT INTO users (id, name, age, email) VALUES ($1, $2, $3, $4)`,
    user.ID, user.Name, user.Age, user.Email)

// Query handles NULL correctly
var user User
row := db.QueryRow("SELECT id, name, age, email FROM users WHERE id = $1", id)
row.Scan(&user.ID, &user.Name, &user.Age, &user.Email)

// Check if age was NULL
if !user.Age.IsValid() {
    fmt.Println("Age not provided")
} else {
    fmt.Printf("Age: %d\n", user.Age.ValueOr(0))
}
```

### Nullable Types: JSON Serialization

```go
type UserProfile struct {
    Name  vos.NullableString `json:"name"`
    Age   vos.NullableInt    `json:"age"`
    Email vos.NullableString `json:"email"`
}

// Serialize
profile := UserProfile{
    Name:  vos.NewNullableString("John"),
    Age:   vos.NullableInt{},  // Will be null in JSON
    Email: vos.NewNullableString("john@example.com"),
}

data, _ := json.Marshal(profile)
// {"name":"John","age":null,"email":"john@example.com"}

// Deserialize
var loaded UserProfile
json.Unmarshal(data, &loaded)

// loaded.Age.IsValid() returns false
```

---

## Best Practices

### 1. Use Scaled Integer Constructors for Precision

```go
// ✅ Good: Exact precision
price, _ := vos.NewMoney(1099, vos.CurrencyBRL)  // Exactly 10.99 BRL
percent, _ := vos.NewPercentage(10500)  // Exactly 10.500%

// ⚠️ Acceptable: Float constructor (minor rounding possible)
price, _ := vos.NewMoneyFromFloat(10.99, vos.CurrencyBRL)
percent, _ := vos.NewPercentageFromFloat(10.5)
```

### 2. Always Check Errors

```go
// ✅ Good: Handle errors
money, err := vos.NewMoney(cents, currency)
if err != nil {
    return fmt.Errorf("invalid money: %w", err)
}

// ❌ Bad: Ignoring errors can cause panics or bugs
money, _ := vos.NewMoney(cents, currency)
```

### 3. Don't Use Float() for Calculations

```go
price, _ := vos.NewMoney(1050, vos.CurrencyBRL)

// ❌ Bad: Float arithmetic loses precision
floatValue := price.Float()  // 10.50
result := floatValue * 0.1   // May be 1.0499999999
finalMoney, _ := vos.NewMoneyFromFloat(result, vos.CurrencyBRL)

// ✅ Good: Use Money operations
percentage, _ := vos.NewPercentageFromFloat(10.0)
result, _ := percentage.Apply(price)  // Precise calculation
```

### 4. Nullable: Use Get() Idiom

```go
age := vos.NewNullableInt(30)

// ✅ Good: Idiomatic Go
if value, ok := age.Get(); ok {
    fmt.Printf("Age: %d\n", value)
} else {
    fmt.Println("Age not provided")
}

// ⚠️ Acceptable but less clear
if age.IsValid() {
    value := age.ValueOr(0)
    fmt.Printf("Age: %d\n", value)
}
```

### 5. Store Money as Structured Data

```go
// ✅ Good: Use Money's Value() method (stores as "cents:currency")
db.Exec("INSERT INTO orders (total) VALUES ($1)", order.Total)

// ❌ Bad: Storing cents only loses currency information
db.Exec("INSERT INTO orders (total_cents) VALUES ($1)", order.Total.Cents())
```

---

## Caveats and Limitations

### Money Overflow

**Limitation:** Money is limited to `±9 quadrillion` cents (~90 trillion dollars) due to int64 limits.

```go
maxMoney, _ := vos.NewMoney(1<<53, vos.CurrencyUSD)  // Max safe value
_, err := maxMoney.Multiply(2)  // Returns ErrOverflow
```

**Workaround:** For values exceeding this limit, use `math/big` package instead.

### Currency Validation

**Limitation:** Only predefined ISO 4217 currencies are supported. Adding a new currency requires code changes.

```go
// ✅ Supported
btc, err := vos.NewCurrency("BTC")  // Returns ErrInvalidCurrency

// Custom currencies require adding to validCurrencies map in currency.go
```

### Nullable Database NULL

**Caveat:** Nullable types handle NULL correctly, but zero values may be ambiguous.

```go
// NULL in database
age := vos.NullableInt{}  // Not valid

// Zero value in database
age := vos.NewNullableInt(0)  // Valid, value is 0

// To distinguish: check IsValid()
if !age.IsValid() {
    // Was NULL
} else if age.ValueOr(-1) == 0 {
    // Was explicitly 0
}
```

### Float Precision

**Caveat:** Float constructors may have minor rounding errors.

```go
// Exact: 10.995%
p1, _ := vos.NewPercentage(10995)

// May round to 10.995 or 10.994999...
p2, _ := vos.NewPercentageFromFloat(10.995)

// For critical precision, use scaled integers
```

### Immutability Performance

**Consideration:** All operations return new instances. For high-frequency operations:

```go
// Slight allocation overhead (negligible for most use cases)
total := vos.NewMoney(0, vos.CurrencyBRL)
for _, item := range items {
    total, _ = total.Add(item.Price)  // Creates new Money each iteration
}

// If profiling shows this as a bottleneck, accumulate in cents:
cents := int64(0)
for _, item := range items {
    cents += item.Price.Cents()
}
total, _ := vos.NewMoney(cents, vos.CurrencyBRL)
```

---

## Common Errors

```go
// Defined in errors.go
ErrInvalidCurrency      // Invalid or unsupported currency code
ErrCurrencyMismatch     // Operating on different currencies
ErrDivisionByZero       // Dividing by zero
ErrInvalidValue         // Invalid value (NaN, Inf, negative where not allowed)
ErrOverflow             // Numeric overflow
ErrInvalidFormat        // Parsing error (invalid string format)
ErrNullValue            // Scanning NULL into non-nullable type
```

---

## Thread Safety

All value objects are **immutable and thread-safe**. They can be safely shared across goroutines without synchronization.

```go
// Safe to share
money := vos.NewMoney(1000, vos.CurrencyBRL)

go func() {
    discounted, _ := money.Multiply(9)  // Creates new instance
}()

go func() {
    doubled, _ := money.Multiply(2)  // Original unchanged
}()
```

---

## Testing

Value Objects are easy to test due to immutability and predictable behavior:

```go
func TestMoneyAddition(t *testing.T) {
    m1, _ := vos.NewMoney(1000, vos.CurrencyBRL)
    m2, _ := vos.NewMoney(500, vos.CurrencyBRL)

    result, err := m1.Add(m2)

    assert.NoError(t, err)
    assert.Equal(t, int64(1500), result.Cents())
    assert.Equal(t, vos.CurrencyBRL, result.Currency())
}
```

---

## Related Packages

- `pkg/entity` - Uses vos.UUID for entity IDs
- `pkg/database` - Compatible with DBTX for persistence
- DDD patterns benefit greatly from Value Objects
