# entity

Base entity struct for DDD entities.

## Quick Start

```go
type User struct {
    entity.Base
    Name  string
    Email string
}

user := User{
    Base: entity.Base{
        ID:        vos.NewUUID(),
        CreatedAt: time.Now(),
    },
    Name:  "John Doe",
    Email: "john@example.com",
}
```

## API

```go
type Base struct {
    ID        vos.UUID
    CreatedAt time.Time
    UpdatedAt vos.NullableTime
    DeletedAt vos.NullableTime  // Soft delete
}

SetID(id vos.UUID)
```

## Usage

```go
// Create entity
user := User{
    Base: entity.Base{
        ID:        vos.NewUUID(),
        CreatedAt: time.Now(),
    },
    Name: "Alice",
}

// Update
user.UpdatedAt = vos.NewNullableTime(time.Now())

// Soft delete
user.DeletedAt = vos.NewNullableTime(time.Now())
```
