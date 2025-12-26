package vos

import (
	"encoding/json"
	"testing"
)

func TestNewPercentage(t *testing.T) {
	t.Run("creates percentage from float value", func(t *testing.T) {
		p := NewPercentage(10.50)
		if p.BasisPoints() != 105000 {
			t.Errorf("NewPercentage(10.50).BasisPoints() = %d, want 105000", p.BasisPoints())
		}
	})

	t.Run("handles zero value", func(t *testing.T) {
		p := NewPercentage(0)
		if !p.IsZero() {
			t.Error("NewPercentage(0).IsZero() = false, want true")
		}
	})

	t.Run("handles negative values", func(t *testing.T) {
		p := NewPercentage(-5.75)
		if p.BasisPoints() != -57500 {
			t.Errorf("NewPercentage(-5.75).BasisPoints() = %d, want -57500", p.BasisPoints())
		}
	})

	t.Run("handles very precise values", func(t *testing.T) {
		p := NewPercentage(0.0001)
		if p.BasisPoints() != 1 {
			t.Errorf("NewPercentage(0.0001).BasisPoints() = %d, want 1", p.BasisPoints())
		}
	})
}

func TestNewPercentageFromBasisPoints(t *testing.T) {
	t.Run("creates percentage from basis points", func(t *testing.T) {
		p := NewPercentageFromBasisPoints(105000)
		if p.Float() != 10.50 {
			t.Errorf("NewPercentageFromBasisPoints(105000).Float() = %f, want 10.50", p.Float())
		}
	})

	t.Run("is equivalent to NewPercentage", func(t *testing.T) {
		p1 := NewPercentage(10.50)
		p2 := NewPercentageFromBasisPoints(105000)

		if !p1.Equals(p2) {
			t.Errorf("NewPercentage(10.50) != NewPercentageFromBasisPoints(105000)")
		}
	})
}

func TestPercentage_Add(t *testing.T) {
	t.Run("adds two positive values", func(t *testing.T) {
		p1 := NewPercentage(10.50)
		p2 := NewPercentage(5.25)
		result := p1.Add(p2)

		expected := NewPercentage(15.75)
		if !result.Equals(expected) {
			t.Errorf("Add() = %v, want %v", result.Float(), expected.Float())
		}
	})

	t.Run("adds positive and negative", func(t *testing.T) {
		p1 := NewPercentage(10.00)
		p2 := NewPercentage(-3.50)
		result := p1.Add(p2)

		expected := NewPercentage(6.50)
		if !result.Equals(expected) {
			t.Errorf("Add() = %v, want %v", result.Float(), expected.Float())
		}
	})

	t.Run("maintains precision", func(t *testing.T) {
		// Este teste falha com float64 tradicional
		p1 := NewPercentage(0.1)
		p2 := NewPercentage(0.2)
		result := p1.Add(p2)

		// Com int64 basis points, 0.1 + 0.2 = 0.3 exatamente
		expected := NewPercentage(0.3)
		if !result.Equals(expected) {
			t.Errorf("Add() precision issue: 0.1 + 0.2 = %v, want 0.3", result.Float())
		}
	})
}

func TestPercentage_Sub(t *testing.T) {
	t.Run("subtracts values", func(t *testing.T) {
		p1 := NewPercentage(10.50)
		p2 := NewPercentage(3.25)
		result := p1.Sub(p2)

		expected := NewPercentage(7.25)
		if !result.Equals(expected) {
			t.Errorf("Sub() = %v, want %v", result.Float(), expected.Float())
		}
	})

	t.Run("handles negative results", func(t *testing.T) {
		p1 := NewPercentage(5.00)
		p2 := NewPercentage(10.00)
		result := p1.Sub(p2)

		if !result.IsNegative() {
			t.Error("Sub() result should be negative")
		}

		expected := NewPercentage(-5.00)
		if !result.Equals(expected) {
			t.Errorf("Sub() = %v, want %v", result.Float(), expected.Float())
		}
	})
}

func TestPercentage_Mul(t *testing.T) {
	t.Run("multiplies by positive factor", func(t *testing.T) {
		p := NewPercentage(10.50)
		result := p.Mul(2)

		expected := NewPercentage(21.00)
		if !result.Equals(expected) {
			t.Errorf("Mul(2) = %v, want %v", result.Float(), expected.Float())
		}
	})

	t.Run("multiplies by fractional factor", func(t *testing.T) {
		p := NewPercentage(10.00)
		result := p.Mul(0.5)

		expected := NewPercentage(5.00)
		if !result.Equals(expected) {
			t.Errorf("Mul(0.5) = %v, want %v", result.Float(), expected.Float())
		}
	})

	t.Run("handles zero multiplication", func(t *testing.T) {
		p := NewPercentage(10.50)
		result := p.Mul(0)

		if !result.IsZero() {
			t.Errorf("Mul(0) = %v, want 0", result.Float())
		}
	})
}

func TestPercentage_Div(t *testing.T) {
	t.Run("divides by positive divisor", func(t *testing.T) {
		p := NewPercentage(10.00)
		result, err := p.Div(2)

		if err != nil {
			t.Fatalf("Div(2) error = %v, want nil", err)
		}

		expected := NewPercentage(5.00)
		if !result.Equals(expected) {
			t.Errorf("Div(2) = %v, want %v", result.Float(), expected.Float())
		}
	})

	t.Run("returns error on division by zero", func(t *testing.T) {
		p := NewPercentage(10.00)
		_, err := p.Div(0)

		if err != ErrDivisionByZeroPercentage {
			t.Errorf("Div(0) error = %v, want %v", err, ErrDivisionByZeroPercentage)
		}
	})

	t.Run("handles fractional divisor", func(t *testing.T) {
		p := NewPercentage(10.00)
		result, err := p.Div(0.5)

		if err != nil {
			t.Fatalf("Div(0.5) error = %v, want nil", err)
		}

		expected := NewPercentage(20.00)
		if !result.Equals(expected) {
			t.Errorf("Div(0.5) = %v, want %v", result.Float(), expected.Float())
		}
	})
}

func TestPercentage_Equals(t *testing.T) {
	t.Run("equal values return true", func(t *testing.T) {
		p1 := NewPercentage(10.50)
		p2 := NewPercentage(10.50)

		if !p1.Equals(p2) {
			t.Error("Equals() = false, want true for equal values")
		}
	})

	t.Run("different values return false", func(t *testing.T) {
		p1 := NewPercentage(10.50)
		p2 := NewPercentage(10.51)

		if p1.Equals(p2) {
			t.Error("Equals() = true, want false for different values")
		}
	})

	t.Run("precision test - no float comparison issues", func(t *testing.T) {
		// Com float64, 0.1 + 0.2 != 0.3
		// Com int64 basis points, deve ser exatamente igual
		p1 := NewPercentage(0.1).Add(NewPercentage(0.2))
		p2 := NewPercentage(0.3)

		if !p1.Equals(p2) {
			t.Errorf("Equals() precision issue: 0.1 + 0.2 != 0.3 (got %v)", p1.Float())
		}
	})
}

func TestPercentage_Comparisons(t *testing.T) {
	t.Run("LessThan", func(t *testing.T) {
		p1 := NewPercentage(5.00)
		p2 := NewPercentage(10.00)

		if !p1.LessThan(p2) {
			t.Error("LessThan() = false, want true")
		}

		if p2.LessThan(p1) {
			t.Error("LessThan() = true, want false")
		}
	})

	t.Run("GreaterThan", func(t *testing.T) {
		p1 := NewPercentage(10.00)
		p2 := NewPercentage(5.00)

		if !p1.GreaterThan(p2) {
			t.Error("GreaterThan() = false, want true")
		}

		if p2.GreaterThan(p1) {
			t.Error("GreaterThan() = true, want false")
		}
	})

	t.Run("LessThanOrEqual", func(t *testing.T) {
		p1 := NewPercentage(5.00)
		p2 := NewPercentage(10.00)
		p3 := NewPercentage(5.00)

		if !p1.LessThanOrEqual(p2) {
			t.Error("LessThanOrEqual() = false, want true for less than")
		}

		if !p1.LessThanOrEqual(p3) {
			t.Error("LessThanOrEqual() = false, want true for equal")
		}
	})

	t.Run("GreaterThanOrEqual", func(t *testing.T) {
		p1 := NewPercentage(10.00)
		p2 := NewPercentage(5.00)
		p3 := NewPercentage(10.00)

		if !p1.GreaterThanOrEqual(p2) {
			t.Error("GreaterThanOrEqual() = false, want true for greater than")
		}

		if !p1.GreaterThanOrEqual(p3) {
			t.Error("GreaterThanOrEqual() = false, want true for equal")
		}
	})
}

func TestPercentage_Predicates(t *testing.T) {
	t.Run("IsZero", func(t *testing.T) {
		if !NewPercentage(0).IsZero() {
			t.Error("IsZero() = false, want true for zero value")
		}

		if NewPercentage(1.00).IsZero() {
			t.Error("IsZero() = true, want false for non-zero value")
		}
	})

	t.Run("IsNegative", func(t *testing.T) {
		if !NewPercentage(-5.00).IsNegative() {
			t.Error("IsNegative() = false, want true for negative value")
		}

		if NewPercentage(5.00).IsNegative() {
			t.Error("IsNegative() = true, want false for positive value")
		}

		if NewPercentage(0).IsNegative() {
			t.Error("IsNegative() = true, want false for zero")
		}
	})

	t.Run("IsPositive", func(t *testing.T) {
		if !NewPercentage(5.00).IsPositive() {
			t.Error("IsPositive() = false, want true for positive value")
		}

		if NewPercentage(-5.00).IsPositive() {
			t.Error("IsPositive() = true, want false for negative value")
		}

		if NewPercentage(0).IsPositive() {
			t.Error("IsPositive() = true, want false for zero")
		}
	})
}

func TestPercentage_Abs(t *testing.T) {
	t.Run("returns absolute value of negative", func(t *testing.T) {
		p := NewPercentage(-10.50)
		abs := p.Abs()

		expected := NewPercentage(10.50)
		if !abs.Equals(expected) {
			t.Errorf("Abs() = %v, want %v", abs.Float(), expected.Float())
		}
	})

	t.Run("returns same value for positive", func(t *testing.T) {
		p := NewPercentage(10.50)
		abs := p.Abs()

		if !abs.Equals(p) {
			t.Errorf("Abs() = %v, want %v", abs.Float(), p.Float())
		}
	})
}

func TestPercentage_Negate(t *testing.T) {
	t.Run("negates positive value", func(t *testing.T) {
		p := NewPercentage(10.50)
		neg := p.Negate()

		expected := NewPercentage(-10.50)
		if !neg.Equals(expected) {
			t.Errorf("Negate() = %v, want %v", neg.Float(), expected.Float())
		}
	})

	t.Run("negates negative value", func(t *testing.T) {
		p := NewPercentage(-10.50)
		neg := p.Negate()

		expected := NewPercentage(10.50)
		if !neg.Equals(expected) {
			t.Errorf("Negate() = %v, want %v", neg.Float(), expected.Float())
		}
	})
}

func TestPercentage_Apply(t *testing.T) {
	t.Run("applies percentage to value", func(t *testing.T) {
		p := NewPercentage(10.0) // 10%
		result := p.Apply(100.0)

		expected := 10.0
		if result != expected {
			t.Errorf("Apply(100) = %v, want %v", result, expected)
		}
	})

	t.Run("applies fractional percentage", func(t *testing.T) {
		p := NewPercentage(2.5) // 2.5%
		result := p.Apply(200.0)

		expected := 5.0
		if result != expected {
			t.Errorf("Apply(200) = %v, want %v", result, expected)
		}
	})

	t.Run("handles zero percentage", func(t *testing.T) {
		p := NewPercentage(0)
		result := p.Apply(100.0)

		if result != 0 {
			t.Errorf("Apply(100) with 0%% = %v, want 0", result)
		}
	})
}

func TestPercentage_ApplyToMoney(t *testing.T) {
	t.Run("applies percentage to money", func(t *testing.T) {
		p := NewPercentage(10.0)        // 10%
		m := NewMoney(100.0)            // R$ 100.00
		result := p.ApplyToMoney(m)     // 10% de R$ 100 = R$ 10

		expected := NewMoney(10.0)
		if !result.Equals(expected) {
			t.Errorf("ApplyToMoney(100) = %v, want %v", result.Float(), expected.Float())
		}
	})

	t.Run("applies tax percentage", func(t *testing.T) {
		p := NewPercentage(15.0)        // 15% tax
		m := NewMoney(50.00)            // R$ 50.00
		result := p.ApplyToMoney(m)     // 15% de R$ 50 = R$ 7.50

		expected := NewMoney(7.50)
		if !result.Equals(expected) {
			t.Errorf("ApplyToMoney(50) = %v, want %v", result.Float(), expected.Float())
		}
	})
}

func TestPercentage_JSON(t *testing.T) {
	t.Run("marshals to JSON", func(t *testing.T) {
		p := NewPercentage(10.50)
		data, err := json.Marshal(p)

		if err != nil {
			t.Fatalf("MarshalJSON() error = %v, want nil", err)
		}

		expected := "10.5000"
		if string(data) != expected {
			t.Errorf("MarshalJSON() = %s, want %s", string(data), expected)
		}
	})

	t.Run("unmarshals from JSON", func(t *testing.T) {
		jsonData := []byte("10.50")
		var p Percentage

		err := json.Unmarshal(jsonData, &p)
		if err != nil {
			t.Fatalf("UnmarshalJSON() error = %v, want nil", err)
		}

		expected := NewPercentage(10.50)
		if !p.Equals(expected) {
			t.Errorf("UnmarshalJSON() = %v, want %v", p.Float(), expected.Float())
		}
	})

	t.Run("roundtrip JSON", func(t *testing.T) {
		original := NewPercentage(12.3456)

		data, err := json.Marshal(original)
		if err != nil {
			t.Fatalf("MarshalJSON() error = %v", err)
		}

		var decoded Percentage
		err = json.Unmarshal(data, &decoded)
		if err != nil {
			t.Fatalf("UnmarshalJSON() error = %v", err)
		}

		if !original.Equals(decoded) {
			t.Errorf("JSON roundtrip failed: got %v, want %v", decoded.Float(), original.Float())
		}
	})
}

func TestPercentage_String(t *testing.T) {
	t.Run("formats with percentage symbol", func(t *testing.T) {
		p := NewPercentage(10.50)
		str := p.String()

		expected := "10.5000%"
		if str != expected {
			t.Errorf("String() = %s, want %s", str, expected)
		}
	})

	t.Run("formats negative percentage", func(t *testing.T) {
		p := NewPercentage(-5.25)
		str := p.String()

		expected := "-5.2500%"
		if str != expected {
			t.Errorf("String() = %s, want %s", str, expected)
		}
	})
}

// Benchmarks
func BenchmarkNewPercentage(b *testing.B) {
	for i := 0; i < b.N; i++ {
		NewPercentage(10.50)
	}
}

func BenchmarkPercentage_Add(b *testing.B) {
	p1 := NewPercentage(10.50)
	p2 := NewPercentage(5.25)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		p1.Add(p2)
	}
}

func BenchmarkPercentage_Apply(b *testing.B) {
	p := NewPercentage(10.0)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		p.Apply(100.0)
	}
}

func BenchmarkPercentage_ApplyToMoney(b *testing.B) {
	p := NewPercentage(10.0)
	m := NewMoney(100.0)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		p.ApplyToMoney(m)
	}
}

func BenchmarkPercentage_JSON_Marshal(b *testing.B) {
	p := NewPercentage(12.3456)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		json.Marshal(p)
	}
}

func BenchmarkPercentage_JSON_Unmarshal(b *testing.B) {
	data := []byte("12.3456")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		var p Percentage
		json.Unmarshal(data, &p)
	}
}
