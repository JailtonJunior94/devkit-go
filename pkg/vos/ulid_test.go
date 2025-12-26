package vos

import (
	"sync"
	"testing"

	"github.com/oklog/ulid/v2"
)

func TestNewULID(t *testing.T) {
	t.Run("creates valid ULID", func(t *testing.T) {
		id, err := NewULID()
		if err != nil {
			t.Fatalf("NewULID() error = %v, want nil", err)
		}

		if id.Value.Compare(ulid.ULID{}) == 0 {
			t.Error("NewULID() returned zero value")
		}
	})

	t.Run("creates unique ULIDs", func(t *testing.T) {
		id1, err1 := NewULID()
		id2, err2 := NewULID()

		if err1 != nil || err2 != nil {
			t.Fatalf("NewULID() errors: %v, %v", err1, err2)
		}

		if id1.String() == id2.String() {
			t.Error("NewULID() created duplicate ULIDs")
		}
	})

	t.Run("is thread-safe - no race conditions", func(t *testing.T) {
		const goroutines = 100
		var wg sync.WaitGroup
		ids := make(chan string, goroutines)

		for i := 0; i < goroutines; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				id, err := NewULID()
				if err != nil {
					t.Errorf("NewULID() error in goroutine: %v", err)
					return
				}
				ids <- id.String()
			}()
		}

		wg.Wait()
		close(ids)

		// Verificar que todos os IDs são únicos
		seen := make(map[string]bool)
		for id := range ids {
			if seen[id] {
				t.Errorf("Duplicate ULID found in concurrent execution: %s", id)
			}
			seen[id] = true
		}

		if len(seen) != goroutines {
			t.Errorf("Expected %d unique ULIDs, got %d", goroutines, len(seen))
		}
	})
}

func TestNewULIDFromString(t *testing.T) {
	t.Run("parses valid ULID string", func(t *testing.T) {
		original, err := NewULID()
		if err != nil {
			t.Fatalf("NewULID() error = %v", err)
		}

		parsed, err := NewULIDFromString(original.String())
		if err != nil {
			t.Fatalf("NewULIDFromString() error = %v, want nil", err)
		}

		if parsed.String() != original.String() {
			t.Errorf("NewULIDFromString() = %v, want %v", parsed.String(), original.String())
		}
	})

	t.Run("returns error for invalid string", func(t *testing.T) {
		_, err := NewULIDFromString("invalid-ulid")
		if err == nil {
			t.Error("NewULIDFromString() error = nil, want error")
		}
	})

	t.Run("returns error for empty string", func(t *testing.T) {
		_, err := NewULIDFromString("")
		if err == nil {
			t.Error("NewULIDFromString() error = nil, want error")
		}
	})
}

func TestULID_Validate(t *testing.T) {
	t.Run("valid ULID passes validation", func(t *testing.T) {
		id, _ := NewULID()
		if err := id.Validate(); err != nil {
			t.Errorf("Validate() error = %v, want nil", err)
		}
	})

	t.Run("zero value ULID fails validation", func(t *testing.T) {
		id := ULID{}
		if err := id.Validate(); err != ErrInvalidULID {
			t.Errorf("Validate() error = %v, want %v", err, ErrInvalidULID)
		}
	})
}

func TestULID_String(t *testing.T) {
	t.Run("returns valid string representation", func(t *testing.T) {
		id, err := NewULID()
		if err != nil {
			t.Fatalf("NewULID() error = %v", err)
		}

		str := id.String()
		if len(str) != 26 {
			t.Errorf("String() length = %d, want 26", len(str))
		}

		// Verificar que pode ser parseado de volta
		parsed, err := NewULIDFromString(str)
		if err != nil {
			t.Errorf("String() produced unparseable string: %v", err)
		}

		if parsed.String() != str {
			t.Errorf("String() roundtrip failed: got %v, want %v", parsed.String(), str)
		}
	})
}

// Benchmark para verificar performance
func BenchmarkNewULID(b *testing.B) {
	for i := 0; i < b.N; i++ {
		_, _ = NewULID()
	}
}

func BenchmarkNewULIDConcurrent(b *testing.B) {
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			_, _ = NewULID()
		}
	})
}

func BenchmarkNewULIDFromString(b *testing.B) {
	id, _ := NewULID()
	str := id.String()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = NewULIDFromString(str)
	}
}
