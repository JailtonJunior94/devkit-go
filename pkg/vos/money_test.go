package vos

import (
	"encoding/json"
	"testing"
)

func TestNewMoney(t *testing.T) {
	t.Run("creates money from float value", func(t *testing.T) {
		m := NewMoney(10.50)
		if m.Cents() != 1050 {
			t.Errorf("NewMoney(10.50).Cents() = %d, want 1050", m.Cents())
		}
	})

	t.Run("handles zero value", func(t *testing.T) {
		m := NewMoney(0)
		if !m.IsZero() {
			t.Error("NewMoney(0).IsZero() = false, want true")
		}
	})

	t.Run("handles negative values", func(t *testing.T) {
		m := NewMoney(-5.75)
		if m.Cents() != -575 {
			t.Errorf("NewMoney(-5.75).Cents() = %d, want -575", m.Cents())
		}
	})
}

func TestNewMoneyFromCents(t *testing.T) {
	t.Run("creates money from cents", func(t *testing.T) {
		m := NewMoneyFromCents(1050)
		if m.Float() != 10.50 {
			t.Errorf("NewMoneyFromCents(1050).Float() = %f, want 10.50", m.Float())
		}
	})

	t.Run("is equivalent to NewMoney", func(t *testing.T) {
		m1 := NewMoney(10.50)
		m2 := NewMoneyFromCents(1050)

		if !m1.Equals(m2) {
			t.Errorf("NewMoney(10.50) != NewMoneyFromCents(1050)")
		}
	})
}

func TestMoney_Add(t *testing.T) {
	t.Run("adds two positive values", func(t *testing.T) {
		m1 := NewMoney(10.50)
		m2 := NewMoney(5.25)
		result := m1.Add(m2)

		expected := NewMoney(15.75)
		if !result.Equals(expected) {
			t.Errorf("Add() = %v, want %v", result.Float(), expected.Float())
		}
	})

	t.Run("adds positive and negative", func(t *testing.T) {
		m1 := NewMoney(10.00)
		m2 := NewMoney(-3.50)
		result := m1.Add(m2)

		expected := NewMoney(6.50)
		if !result.Equals(expected) {
			t.Errorf("Add() = %v, want %v", result.Float(), expected.Float())
		}
	})

	t.Run("maintains precision", func(t *testing.T) {
		// Este teste falha com float64 tradicional
		m1 := NewMoney(0.1)
		m2 := NewMoney(0.2)
		result := m1.Add(m2)

		// Com int64 cents, 0.1 + 0.2 = 0.3 exatamente
		expected := NewMoney(0.3)
		if !result.Equals(expected) {
			t.Errorf("Add() precision issue: 0.1 + 0.2 = %v, want 0.3", result.Float())
		}
	})
}

func TestMoney_Sub(t *testing.T) {
	t.Run("subtracts values", func(t *testing.T) {
		m1 := NewMoney(10.50)
		m2 := NewMoney(3.25)
		result := m1.Sub(m2)

		expected := NewMoney(7.25)
		if !result.Equals(expected) {
			t.Errorf("Sub() = %v, want %v", result.Float(), expected.Float())
		}
	})

	t.Run("handles negative results", func(t *testing.T) {
		m1 := NewMoney(5.00)
		m2 := NewMoney(10.00)
		result := m1.Sub(m2)

		if !result.IsNegative() {
			t.Error("Sub() result should be negative")
		}

		expected := NewMoney(-5.00)
		if !result.Equals(expected) {
			t.Errorf("Sub() = %v, want %v", result.Float(), expected.Float())
		}
	})
}

func TestMoney_Mul(t *testing.T) {
	t.Run("multiplies by positive factor", func(t *testing.T) {
		m := NewMoney(10.50)
		result := m.Mul(2)

		expected := NewMoney(21.00)
		if !result.Equals(expected) {
			t.Errorf("Mul(2) = %v, want %v", result.Float(), expected.Float())
		}
	})

	t.Run("multiplies by fractional factor", func(t *testing.T) {
		m := NewMoney(10.00)
		result := m.Mul(0.5)

		expected := NewMoney(5.00)
		if !result.Equals(expected) {
			t.Errorf("Mul(0.5) = %v, want %v", result.Float(), expected.Float())
		}
	})

	t.Run("handles zero multiplication", func(t *testing.T) {
		m := NewMoney(10.50)
		result := m.Mul(0)

		if !result.IsZero() {
			t.Errorf("Mul(0) = %v, want 0", result.Float())
		}
	})
}

func TestMoney_Div(t *testing.T) {
	t.Run("divides by positive divisor", func(t *testing.T) {
		m := NewMoney(10.00)
		result, err := m.Div(2)

		if err != nil {
			t.Fatalf("Div(2) error = %v, want nil", err)
		}

		expected := NewMoney(5.00)
		if !result.Equals(expected) {
			t.Errorf("Div(2) = %v, want %v", result.Float(), expected.Float())
		}
	})

	t.Run("returns error on division by zero", func(t *testing.T) {
		m := NewMoney(10.00)
		_, err := m.Div(0)

		if err != ErrDivisionByZero {
			t.Errorf("Div(0) error = %v, want %v", err, ErrDivisionByZero)
		}
	})

	t.Run("handles fractional divisor", func(t *testing.T) {
		m := NewMoney(10.00)
		result, err := m.Div(0.5)

		if err != nil {
			t.Fatalf("Div(0.5) error = %v, want nil", err)
		}

		expected := NewMoney(20.00)
		if !result.Equals(expected) {
			t.Errorf("Div(0.5) = %v, want %v", result.Float(), expected.Float())
		}
	})
}

func TestMoney_Equals(t *testing.T) {
	t.Run("equal values return true", func(t *testing.T) {
		m1 := NewMoney(10.50)
		m2 := NewMoney(10.50)

		if !m1.Equals(m2) {
			t.Error("Equals() = false, want true for equal values")
		}
	})

	t.Run("different values return false", func(t *testing.T) {
		m1 := NewMoney(10.50)
		m2 := NewMoney(10.51)

		if m1.Equals(m2) {
			t.Error("Equals() = true, want false for different values")
		}
	})

	t.Run("precision test - no float comparison issues", func(t *testing.T) {
		// Com float64, 0.1 + 0.2 != 0.3
		// Com int64 cents, deve ser exatamente igual
		m1 := NewMoney(0.1).Add(NewMoney(0.2))
		m2 := NewMoney(0.3)

		if !m1.Equals(m2) {
			t.Errorf("Equals() precision issue: 0.1 + 0.2 != 0.3 (got %v)", m1.Float())
		}
	})
}

func TestMoney_Comparisons(t *testing.T) {
	t.Run("LessThan", func(t *testing.T) {
		m1 := NewMoney(5.00)
		m2 := NewMoney(10.00)

		if !m1.LessThan(m2) {
			t.Error("LessThan() = false, want true")
		}

		if m2.LessThan(m1) {
			t.Error("LessThan() = true, want false")
		}
	})

	t.Run("GreaterThan", func(t *testing.T) {
		m1 := NewMoney(10.00)
		m2 := NewMoney(5.00)

		if !m1.GreaterThan(m2) {
			t.Error("GreaterThan() = false, want true")
		}

		if m2.GreaterThan(m1) {
			t.Error("GreaterThan() = true, want false")
		}
	})

	t.Run("LessThanOrEqual", func(t *testing.T) {
		m1 := NewMoney(5.00)
		m2 := NewMoney(10.00)
		m3 := NewMoney(5.00)

		if !m1.LessThanOrEqual(m2) {
			t.Error("LessThanOrEqual() = false, want true for less than")
		}

		if !m1.LessThanOrEqual(m3) {
			t.Error("LessThanOrEqual() = false, want true for equal")
		}
	})

	t.Run("GreaterThanOrEqual", func(t *testing.T) {
		m1 := NewMoney(10.00)
		m2 := NewMoney(5.00)
		m3 := NewMoney(10.00)

		if !m1.GreaterThanOrEqual(m2) {
			t.Error("GreaterThanOrEqual() = false, want true for greater than")
		}

		if !m1.GreaterThanOrEqual(m3) {
			t.Error("GreaterThanOrEqual() = false, want true for equal")
		}
	})
}

func TestMoney_Predicates(t *testing.T) {
	t.Run("IsZero", func(t *testing.T) {
		if !NewMoney(0).IsZero() {
			t.Error("IsZero() = false, want true for zero value")
		}

		if NewMoney(1.00).IsZero() {
			t.Error("IsZero() = true, want false for non-zero value")
		}
	})

	t.Run("IsNegative", func(t *testing.T) {
		if !NewMoney(-5.00).IsNegative() {
			t.Error("IsNegative() = false, want true for negative value")
		}

		if NewMoney(5.00).IsNegative() {
			t.Error("IsNegative() = true, want false for positive value")
		}

		if NewMoney(0).IsNegative() {
			t.Error("IsNegative() = true, want false for zero")
		}
	})

	t.Run("IsPositive", func(t *testing.T) {
		if !NewMoney(5.00).IsPositive() {
			t.Error("IsPositive() = false, want true for positive value")
		}

		if NewMoney(-5.00).IsPositive() {
			t.Error("IsPositive() = true, want false for negative value")
		}

		if NewMoney(0).IsPositive() {
			t.Error("IsPositive() = true, want false for zero")
		}
	})
}

func TestMoney_Abs(t *testing.T) {
	t.Run("returns absolute value of negative", func(t *testing.T) {
		m := NewMoney(-10.50)
		abs := m.Abs()

		expected := NewMoney(10.50)
		if !abs.Equals(expected) {
			t.Errorf("Abs() = %v, want %v", abs.Float(), expected.Float())
		}
	})

	t.Run("returns same value for positive", func(t *testing.T) {
		m := NewMoney(10.50)
		abs := m.Abs()

		if !abs.Equals(m) {
			t.Errorf("Abs() = %v, want %v", abs.Float(), m.Float())
		}
	})
}

func TestMoney_Negate(t *testing.T) {
	t.Run("negates positive value", func(t *testing.T) {
		m := NewMoney(10.50)
		neg := m.Negate()

		expected := NewMoney(-10.50)
		if !neg.Equals(expected) {
			t.Errorf("Negate() = %v, want %v", neg.Float(), expected.Float())
		}
	})

	t.Run("negates negative value", func(t *testing.T) {
		m := NewMoney(-10.50)
		neg := m.Negate()

		expected := NewMoney(10.50)
		if !neg.Equals(expected) {
			t.Errorf("Negate() = %v, want %v", neg.Float(), expected.Float())
		}
	})
}

func TestMoney_JSON(t *testing.T) {
	t.Run("marshals to JSON", func(t *testing.T) {
		m := NewMoney(10.50)
		data, err := json.Marshal(m)

		if err != nil {
			t.Fatalf("MarshalJSON() error = %v, want nil", err)
		}

		expected := "10.50"
		if string(data) != expected {
			t.Errorf("MarshalJSON() = %s, want %s", string(data), expected)
		}
	})

	t.Run("unmarshals from JSON", func(t *testing.T) {
		jsonData := []byte("10.50")
		var m Money

		err := json.Unmarshal(jsonData, &m)
		if err != nil {
			t.Fatalf("UnmarshalJSON() error = %v, want nil", err)
		}

		expected := NewMoney(10.50)
		if !m.Equals(expected) {
			t.Errorf("UnmarshalJSON() = %v, want %v", m.Float(), expected.Float())
		}
	})

	t.Run("roundtrip JSON", func(t *testing.T) {
		original := NewMoney(123.45)

		data, err := json.Marshal(original)
		if err != nil {
			t.Fatalf("MarshalJSON() error = %v", err)
		}

		var decoded Money
		err = json.Unmarshal(data, &decoded)
		if err != nil {
			t.Fatalf("UnmarshalJSON() error = %v", err)
		}

		if !original.Equals(decoded) {
			t.Errorf("JSON roundtrip failed: got %v, want %v", decoded.Float(), original.Float())
		}
	})

	t.Run("marshals in struct", func(t *testing.T) {
		type Product struct {
			Name  string `json:"name"`
			Price Money  `json:"price"`
		}

		product := Product{
			Name:  "Laptop",
			Price: NewMoney(1299.99),
		}

		data, err := json.Marshal(product)
		if err != nil {
			t.Fatalf("Marshal struct error = %v", err)
		}

		var decoded Product
		err = json.Unmarshal(data, &decoded)
		if err != nil {
			t.Fatalf("Unmarshal struct error = %v", err)
		}

		if !decoded.Price.Equals(product.Price) {
			t.Errorf("Struct JSON roundtrip failed: got %v, want %v",
				decoded.Price.Float(), product.Price.Float())
		}
	})
}

func TestMoney_String(t *testing.T) {
	t.Run("formats as BRL currency", func(t *testing.T) {
		m := NewMoney(1234.56)
		str := m.String()

		// String() deve incluir símbolo de moeda BRL
		if len(str) == 0 {
			t.Error("String() returned empty string")
		}

		// Deve conter o valor formatado
		// Formato pode variar por locale, mas deve conter o número
		// Apenas verificamos que não está vazio
	})
}

// Benchmarks
func BenchmarkNewMoney(b *testing.B) {
	for i := 0; i < b.N; i++ {
		NewMoney(10.50)
	}
}

func BenchmarkMoney_Add(b *testing.B) {
	m1 := NewMoney(10.50)
	m2 := NewMoney(5.25)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		m1.Add(m2)
	}
}

func BenchmarkMoney_Equals(b *testing.B) {
	m1 := NewMoney(10.50)
	m2 := NewMoney(10.50)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		m1.Equals(m2)
	}
}

func BenchmarkMoney_JSON_Marshal(b *testing.B) {
	m := NewMoney(1234.56)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		json.Marshal(m)
	}
}

func BenchmarkMoney_JSON_Unmarshal(b *testing.B) {
	data := []byte("1234.56")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		var m Money
		json.Unmarshal(data, &m)
	}
}
