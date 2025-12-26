package encrypt

import (
	"strings"
	"testing"

	"golang.org/x/crypto/bcrypt"
)

func TestNewHashAdapter(t *testing.T) {
	t.Run("creates adapter with default cost", func(t *testing.T) {
		adapter := NewHashAdapter()

		if adapter == nil {
			t.Fatal("NewHashAdapter() returned nil")
		}

		// Verifica que pode gerar hash
		hash, err := adapter.GenerateHash("password")
		if err != nil {
			t.Errorf("GenerateHash() error = %v, want nil", err)
		}

		if hash == "" {
			t.Error("GenerateHash() returned empty hash")
		}
	})
}

func TestNewHashAdapterWithCost(t *testing.T) {
	testCases := []struct {
		name          string
		inputCost     int
		testHash      bool // Se deve testar geração de hash
		expectedValid bool
	}{
		{"minimum cost", MinBcryptCost, true, true},
		{"default cost", DefaultBcryptCost, true, true},
		{"high cost", 12, true, true},
		{"below minimum (should clamp to min)", 2, true, true},
		{"above maximum (should clamp to max)", 50, false, true}, // Não testa hash (muito lento)
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			adapter := NewHashAdapterWithCost(tc.inputCost)

			if adapter == nil {
				t.Fatal("NewHashAdapterWithCost() returned nil")
			}

			// Apenas testa geração de hash para costs razoáveis
			if tc.testHash {
				hash, err := adapter.GenerateHash("password")
				if tc.expectedValid && err != nil {
					t.Errorf("GenerateHash() error = %v, want nil", err)
				}

				if tc.expectedValid && hash == "" {
					t.Error("GenerateHash() returned empty hash")
				}
			}
		})
	}
}

func TestHashAdapter_GenerateHash(t *testing.T) {
	adapter := NewHashAdapter()

	t.Run("generates valid bcrypt hash", func(t *testing.T) {
		password := "mySecurePassword123"
		hash, err := adapter.GenerateHash(password)

		if err != nil {
			t.Fatalf("GenerateHash() error = %v, want nil", err)
		}

		if hash == "" {
			t.Fatal("GenerateHash() returned empty hash")
		}

		// Bcrypt hashes começam com $2a$, $2b$, ou $2y$
		if !strings.HasPrefix(hash, "$2") {
			t.Errorf("GenerateHash() hash format invalid: %s", hash)
		}
	})

	t.Run("generates different hashes for same password (salt)", func(t *testing.T) {
		password := "testPassword"

		hash1, err1 := adapter.GenerateHash(password)
		hash2, err2 := adapter.GenerateHash(password)

		if err1 != nil || err2 != nil {
			t.Fatalf("GenerateHash() errors: %v, %v", err1, err2)
		}

		// Os hashes devem ser diferentes devido ao salt
		if hash1 == hash2 {
			t.Error("GenerateHash() produced identical hashes for same password (salt issue)")
		}
	})

	t.Run("handles empty string", func(t *testing.T) {
		hash, err := adapter.GenerateHash("")

		if err != nil {
			t.Errorf("GenerateHash(\"\") error = %v, want nil", err)
		}

		if hash == "" {
			t.Error("GenerateHash(\"\") returned empty hash")
		}
	})

	t.Run("handles long password within bcrypt limit", func(t *testing.T) {
		// bcrypt tem limite de 72 bytes
		longPassword := strings.Repeat("a", 72)
		hash, err := adapter.GenerateHash(longPassword)

		if err != nil {
			t.Errorf("GenerateHash(72 bytes) error = %v, want nil", err)
		}

		if hash == "" {
			t.Error("GenerateHash(72 bytes) returned empty hash")
		}
	})

	t.Run("returns error for password exceeding 72 bytes", func(t *testing.T) {
		// Senhas > 72 bytes excedem o limite do bcrypt
		veryLongPassword := strings.Repeat("a", 100)
		_, err := adapter.GenerateHash(veryLongPassword)

		if err == nil {
			t.Error("GenerateHash(>72 bytes) error = nil, want error (bcrypt limit exceeded)")
		}
	})

	t.Run("handles special characters", func(t *testing.T) {
		password := "P@ssw0rd!@#$%^&*()_+-=[]{}|;:',.<>?/~`"
		hash, err := adapter.GenerateHash(password)

		if err != nil {
			t.Errorf("GenerateHash(special chars) error = %v, want nil", err)
		}

		if hash == "" {
			t.Error("GenerateHash(special chars) returned empty hash")
		}
	})

	t.Run("handles unicode characters", func(t *testing.T) {
		password := "パスワード密码пароль"
		hash, err := adapter.GenerateHash(password)

		if err != nil {
			t.Errorf("GenerateHash(unicode) error = %v, want nil", err)
		}

		if hash == "" {
			t.Error("GenerateHash(unicode) returned empty hash")
		}
	})
}

func TestHashAdapter_CheckHash(t *testing.T) {
	adapter := NewHashAdapter()

	t.Run("validates correct password", func(t *testing.T) {
		password := "myPassword123"
		hash, err := adapter.GenerateHash(password)

		if err != nil {
			t.Fatalf("GenerateHash() error = %v", err)
		}

		if !adapter.CheckHash(hash, password) {
			t.Error("CheckHash() = false, want true for correct password")
		}
	})

	t.Run("rejects incorrect password", func(t *testing.T) {
		password := "correctPassword"
		wrongPassword := "wrongPassword"

		hash, err := adapter.GenerateHash(password)
		if err != nil {
			t.Fatalf("GenerateHash() error = %v", err)
		}

		if adapter.CheckHash(hash, wrongPassword) {
			t.Error("CheckHash() = true, want false for incorrect password")
		}
	})

	t.Run("rejects empty password against valid hash", func(t *testing.T) {
		password := "password"
		hash, err := adapter.GenerateHash(password)

		if err != nil {
			t.Fatalf("GenerateHash() error = %v", err)
		}

		if adapter.CheckHash(hash, "") {
			t.Error("CheckHash() = true, want false for empty password")
		}
	})

	t.Run("handles empty hash", func(t *testing.T) {
		if adapter.CheckHash("", "password") {
			t.Error("CheckHash() = true, want false for empty hash")
		}
	})

	t.Run("handles invalid hash format", func(t *testing.T) {
		invalidHash := "not-a-valid-bcrypt-hash"

		if adapter.CheckHash(invalidHash, "password") {
			t.Error("CheckHash() = true, want false for invalid hash")
		}
	})

	t.Run("is case sensitive", func(t *testing.T) {
		password := "Password123"
		hash, err := adapter.GenerateHash(password)

		if err != nil {
			t.Fatalf("GenerateHash() error = %v", err)
		}

		if adapter.CheckHash(hash, "password123") {
			t.Error("CheckHash() = true, want false (case sensitive)")
		}
	})
}

func TestHashAdapter_DifferentCosts(t *testing.T) {
	testCases := []struct {
		name string
		cost int
	}{
		{"cost 4 (min)", MinBcryptCost},
		{"cost 10 (default)", DefaultBcryptCost},
		{"cost 12 (high security)", 12},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			adapter := NewHashAdapterWithCost(tc.cost)
			password := "testPassword"

			hash, err := adapter.GenerateHash(password)
			if err != nil {
				t.Fatalf("GenerateHash() error = %v", err)
			}

			// Verifica o custo do hash gerado
			cost, err := bcrypt.Cost([]byte(hash))
			if err != nil {
				t.Fatalf("bcrypt.Cost() error = %v", err)
			}

			// O custo deve estar dentro dos limites válidos
			if cost < MinBcryptCost || cost > MaxBcryptCost {
				t.Errorf("Hash cost %d outside valid range [%d, %d]", cost, MinBcryptCost, MaxBcryptCost)
			}

			// Verifica que o hash funciona
			if !adapter.CheckHash(hash, password) {
				t.Error("CheckHash() = false, want true")
			}
		})
	}
}

func TestHashAdapter_CompatibilityWithOldHashes(t *testing.T) {
	t.Run("validates hash created with old cost (5)", func(t *testing.T) {
		// Simula um hash antigo criado com cost 5
		oldAdapter := NewHashAdapterWithCost(5)
		password := "oldPassword"

		oldHash, err := oldAdapter.GenerateHash(password)
		if err != nil {
			t.Fatalf("GenerateHash() error = %v", err)
		}

		// Novo adapter com cost 10 deve conseguir validar hash antigo
		newAdapter := NewHashAdapter()
		if !newAdapter.CheckHash(oldHash, password) {
			t.Error("CheckHash() failed to validate old hash (backward compatibility issue)")
		}
	})
}

func TestHashAdapter_SecurityProperties(t *testing.T) {
	adapter := NewHashAdapter()

	t.Run("default cost is secure (>= 10)", func(t *testing.T) {
		if DefaultBcryptCost < 10 {
			t.Errorf("DefaultBcryptCost = %d, want >= 10 for security", DefaultBcryptCost)
		}
	})

	t.Run("hash contains salt", func(t *testing.T) {
		password := "testPassword"

		hash1, _ := adapter.GenerateHash(password)
		hash2, _ := adapter.GenerateHash(password)

		// Se os hashes são diferentes, significa que está usando salt
		if hash1 == hash2 {
			t.Error("Hashes are identical - salt not working")
		}
	})

	t.Run("timing attack resistance", func(t *testing.T) {
		// bcrypt.CompareHashAndPassword é resistente a timing attacks
		// Este teste apenas verifica que estamos usando a função correta

		password := "password"
		hash, _ := adapter.GenerateHash(password)

		// Mesmo com senhas de tamanhos diferentes, CheckHash deve ser usado
		// (a implementação interna de bcrypt é constant-time)
		result1 := adapter.CheckHash(hash, "a")
		result2 := adapter.CheckHash(hash, "aaaaaaaaaaaaaaaaaaa")

		// Ambos devem retornar false, mas isso não testa timing
		// O importante é que estamos usando bcrypt.CompareHashAndPassword
		if result1 || result2 {
			t.Error("CheckHash() returned true for wrong passwords")
		}
	})
}

func TestHashAdapter_ConcurrentUsage(t *testing.T) {
	t.Run("handles concurrent hash generation", func(t *testing.T) {
		adapter := NewHashAdapterWithCost(MinBcryptCost) // Use min cost for speed

		const goroutines = 50
		errors := make(chan error, goroutines)
		done := make(chan bool, goroutines)

		for i := 0; i < goroutines; i++ {
			go func(id int) {
				password := "password"
				hash, err := adapter.GenerateHash(password)

				if err != nil {
					errors <- err
					return
				}

				if !adapter.CheckHash(hash, password) {
					errors <- err
					return
				}

				done <- true
			}(i)
		}

		// Aguarda todas as goroutines
		for i := 0; i < goroutines; i++ {
			select {
			case err := <-errors:
				t.Errorf("Concurrent operation failed: %v", err)
			case <-done:
				// Success
			}
		}
	})
}

// Benchmarks
func BenchmarkHashAdapter_GenerateHash_Cost4(b *testing.B) {
	adapter := NewHashAdapterWithCost(MinBcryptCost)
	password := "benchmarkPassword"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		adapter.GenerateHash(password)
	}
}

func BenchmarkHashAdapter_GenerateHash_Cost10(b *testing.B) {
	adapter := NewHashAdapter()
	password := "benchmarkPassword"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		adapter.GenerateHash(password)
	}
}

func BenchmarkHashAdapter_CheckHash(b *testing.B) {
	adapter := NewHashAdapter()
	password := "benchmarkPassword"
	hash, _ := adapter.GenerateHash(password)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		adapter.CheckHash(hash, password)
	}
}
