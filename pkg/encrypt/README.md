# encrypt

Secure password hashing using bcrypt with configurable cost.

## Quick Start

```go
hasher := encrypt.NewHashAdapter()

// Hash password
hash, err := hasher.GenerateHash("mypassword")

// Verify password
isValid := hasher.CheckHash(hash, "mypassword")  // true
```

## API

```go
type HashAdapter interface {
    GenerateHash(str string) (string, error)
    CheckHash(hash, str string) bool
}

NewHashAdapter() HashAdapter  // Cost 10 (production default)
NewHashAdapterWithCost(cost int) HashAdapter

// Constants
DefaultBcryptCost = 10  // ~100ms
MinBcryptCost = 4       // Tests only
MaxBcryptCost = 31      // Very slow
```

## Examples

```go
// Production use
hasher := encrypt.NewHashAdapter()
hash, _ := hasher.GenerateHash("user_password_123")
// Store hash in database

// Later: verify
if hasher.CheckHash(hash, inputPassword) {
    // Password matches
}

// High security (slow)
secureHasher := encrypt.NewHashAdapterWithCost(14)  // ~1.6s

// Tests (fast)
testHasher := encrypt.NewHashAdapterWithCost(encrypt.MinBcryptCost)
```

## Best Practices

- Use cost 10-12 for production
- Use MinCost only in tests
- Never store plain passwords
- Hash includes salt automatically
