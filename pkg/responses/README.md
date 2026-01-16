# responses

HTTP response helpers for JSON encoding.

## Quick Start

```go
responses.JSON(w, http.StatusOK, user)
responses.Error(w, http.StatusBadRequest, "Invalid input")
responses.ErrorWithDetails(w, http.StatusUnprocessableEntity, "Validation failed", validationErrors)
```

## API

```go
JSON(w http.ResponseWriter, statusCode int, data any)
Error(w http.ResponseWriter, statusCode int, message string)
ErrorWithDetails(w http.ResponseWriter, statusCode int, message string, details any)
```

## Examples

```go
// Success response
responses.JSON(w, http.StatusOK, map[string]string{
    "message": "User created",
    "id":      userID,
})

// Simple error
responses.Error(w, http.StatusNotFound, "User not found")

// Error with details
responses.ErrorWithDetails(w, http.StatusBadRequest, "Validation failed", map[string]string{
    "email": "Invalid format",
    "age":   "Must be positive",
})
```

## Features

- Sets Content-Type automatically
- Handles encoding errors gracefully
- Never panics
