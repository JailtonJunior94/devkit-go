package vos

import (
	"database/sql/driver"
	"encoding/json"
	"fmt"
	"math"
	"strconv"
	"strings"
)

// Percentage represents a percentage value with fixed precision.
// It is a DDD Value Object that is immutable, thread-safe, and safe for calculations.
//
// Internal representation uses int64 with scale 3 (e.g., 12.345% = 12345).
// This avoids floating-point precision issues in financial calculations.
//
// Example:
//
//	p, _ := NewPercentage(12345)  // 12.345%
//	p2, _ := NewPercentageFromFloat(12.345)  // 12.345% (less precise, not recommended)
type Percentage struct {
	value int64 // Value scaled by 1000 (3 decimal places) - immutable
}

const (
	percentageScaleFactor = 1000 // 10^scale = 1000 for 3 decimal places
	maxPercentageValue    = 1 << 53
	minPercentageValue    = -maxPercentageValue
)

// NewPercentage creates a new Percentage value object from a scaled integer.
// This is the recommended constructor for precision-critical operations.
//
// The value should be scaled by 1000. For example:
//   - 12345 represents 12.345%
//   - 1000 represents 1.000%
//   - 100000 represents 100.000%
//
// Returns ErrInvalidValue for invalid inputs.
// Returns ErrOverflow if value exceeds safe limits.
//
// Example:
//
//	p, err := NewPercentage(12345)  // 12.345%
//	if err != nil {
//	    log.Fatal(err)
//	}
func NewPercentage(value int64) (Percentage, error) {
	if value > maxPercentageValue || value < minPercentageValue {
		return Percentage{}, ErrOverflow
	}

	return Percentage{value: value}, nil
}

// NewPercentageFromFloat creates Percentage from a float64 value.
// This is less precise due to floating-point representation.
// Use NewPercentage with scaled integer for precision-critical operations.
//
// Example:
//
//	p, err := NewPercentageFromFloat(12.345)  // 12.345%
func NewPercentageFromFloat(value float64) (Percentage, error) {
	if math.IsNaN(value) || math.IsInf(value, 0) {
		return Percentage{}, ErrInvalidValue
	}

	scaled := int64(math.Round(value * percentageScaleFactor))

	if scaled > maxPercentageValue || scaled < minPercentageValue {
		return Percentage{}, ErrOverflow
	}

	return Percentage{value: scaled}, nil
}

// NewPercentageFromString creates Percentage from a string representation.
// Accepts formats: "12.345", "12,345", "12.3", "12"
//
// Example:
//
//	p, err := NewPercentageFromString("12.345")  // 12.345%
func NewPercentageFromString(value string) (Percentage, error) {
	// Normalize decimal separator
	value = strings.TrimSpace(value)
	value = strings.ReplaceAll(value, ",", ".")
	value = strings.TrimSuffix(value, "%")
	value = strings.TrimSpace(value)

	// Parse as float64
	floatValue, err := strconv.ParseFloat(value, 64)
	if err != nil {
		return Percentage{}, ErrInvalidFormat
	}

	return NewPercentageFromFloat(floatValue)
}

// Value returns the raw scaled value (value * 1000).
// Useful for database storage and precise calculations.
func (p Percentage) Value() int64 {
	return p.value
}

// Float returns the value as float64.
// WARNING: Use only for display purposes, not for calculations due to precision loss.
func (p Percentage) Float() float64 {
	return float64(p.value) / percentageScaleFactor
}

// Add adds two Percentage values.
// Returns ErrOverflow if result would overflow.
func (p Percentage) Add(other Percentage) (Percentage, error) {
	// Check for overflow
	if (other.value > 0 && p.value > maxPercentageValue-other.value) ||
		(other.value < 0 && p.value < minPercentageValue-other.value) {
		return Percentage{}, ErrOverflow
	}

	return Percentage{value: p.value + other.value}, nil
}

// Subtract subtracts another Percentage value from this one.
// Returns ErrOverflow if result would overflow.
func (p Percentage) Subtract(other Percentage) (Percentage, error) {
	// Check for overflow
	if (other.value < 0 && p.value > maxPercentageValue+other.value) ||
		(other.value > 0 && p.value < minPercentageValue+other.value) {
		return Percentage{}, ErrOverflow
	}

	return Percentage{value: p.value - other.value}, nil
}

// Multiply multiplies Percentage by an integer factor.
// Returns ErrOverflow if result would overflow.
func (p Percentage) Multiply(factor int64) (Percentage, error) {
	// Check for overflow
	if factor != 0 && (p.value > maxPercentageValue/factor || p.value < minPercentageValue/factor) {
		return Percentage{}, ErrOverflow
	}

	return Percentage{value: p.value * factor}, nil
}

// Divide divides Percentage by an integer divisor.
// Returns ErrDivisionByZero if divisor is zero.
// Result is truncated towards zero.
func (p Percentage) Divide(divisor int64) (Percentage, error) {
	if divisor == 0 {
		return Percentage{}, ErrDivisionByZero
	}

	return Percentage{value: p.value / divisor}, nil
}

// Apply applies this percentage to a Money value.
// Returns the calculated amount as Money with the same currency.
// Returns ErrCurrencyMismatch for operations between different currencies.
//
// Example:
//
//	p, _ := NewPercentageFromFloat(10.0)  // 10%
//	m, _ := NewMoney(10000, CurrencyBRL)  // 100.00 BRL
//	result, err := p.Apply(m)  // 10.00 BRL (10% of 100.00)
func (p Percentage) Apply(money Money) (Money, error) {
	// Calculate: (money.cents * percentage) / 100 / 1000
	// This is: (cents * p.value) / 100000
	// We use integer arithmetic to maintain precision

	cents := money.Cents()
	percentage := p.value
	divisor := int64(100 * percentageScaleFactor)

	// Check for overflow BEFORE multiplication
	// We need to ensure that cents * percentage doesn't overflow int64
	if percentage != 0 {
		// Use absolute values for overflow check
		absCents := cents
		if absCents < 0 {
			absCents = -absCents
		}

		absPercentage := percentage
		if absPercentage < 0 {
			absPercentage = -absPercentage
		}

		// Check if multiplication would overflow
		// We compare: absCents > maxPercentageValue / absPercentage
		if absPercentage > 0 && absCents > maxPercentageValue/absPercentage {
			return Money{}, ErrOverflow
		}
	}

	// Safe to perform multiplication now
	result := (cents * percentage) / divisor

	return NewMoney(result, money.Currency())
}

// Equals checks if two Percentage values are equal.
func (p Percentage) Equals(other Percentage) bool {
	return p.value == other.value
}

// GreaterThan checks if this Percentage is greater than another.
func (p Percentage) GreaterThan(other Percentage) bool {
	return p.value > other.value
}

// LessThan checks if this Percentage is less than another.
func (p Percentage) LessThan(other Percentage) bool {
	return p.value < other.value
}

// GreaterThanOrEqual checks if this Percentage is greater than or equal to another.
func (p Percentage) GreaterThanOrEqual(other Percentage) bool {
	return p.value >= other.value
}

// LessThanOrEqual checks if this Percentage is less than or equal to another.
func (p Percentage) LessThanOrEqual(other Percentage) bool {
	return p.value <= other.value
}

// IsZero checks if the value is zero.
func (p Percentage) IsZero() bool {
	return p.value == 0
}

// IsPositive checks if the value is positive (> 0).
func (p Percentage) IsPositive() bool {
	return p.value > 0
}

// IsNegative checks if the value is negative (< 0).
func (p Percentage) IsNegative() bool {
	return p.value < 0
}

// Abs returns the absolute value.
func (p Percentage) Abs() Percentage {
	if p.value < 0 {
		return Percentage{value: -p.value}
	}
	return p
}

// Negate returns the negated value.
func (p Percentage) Negate() Percentage {
	return Percentage{value: -p.value}
}

// String returns a human-readable string representation.
// Format: "12.345%".
func (p Percentage) String() string {
	return fmt.Sprintf("%.3f%%", p.Float())
}

// MarshalJSON implements json.Marshaler.
// Serializes as string with 3 decimal places: "12.345".
func (p Percentage) MarshalJSON() ([]byte, error) {
	return []byte(fmt.Sprintf(`"%.3f"`, p.Float())), nil
}

// UnmarshalJSON implements json.Unmarshaler.
// Accepts string or number: "12.345" or 12.345.
func (p *Percentage) UnmarshalJSON(data []byte) error {
	// Try to unmarshal as string first
	var str string
	if err := json.Unmarshal(data, &str); err == nil {
		percentage, err := NewPercentageFromString(str)
		if err != nil {
			return err
		}
		*p = percentage
		return nil
	}

	// Try to unmarshal as number
	var num float64
	if err := json.Unmarshal(data, &num); err != nil {
		return ErrInvalidFormat
	}

	percentage, err := NewPercentageFromFloat(num)
	if err != nil {
		return err
	}

	*p = percentage
	return nil
}

// Value implements driver.Valuer for database persistence.
// Stores as integer (scaled by 1000) for precision.
func (p Percentage) ValuerValue() (driver.Value, error) {
	return p.value, nil
}

// Scan implements sql.Scanner for database retrieval.
// Reads from INTEGER or NUMERIC database column.
func (p *Percentage) Scan(value interface{}) error {
	if value == nil {
		return ErrNullValue
	}

	switch v := value.(type) {
	case int64:
		// Stored as scaled integer (recommended)
		percentage, err := NewPercentage(v)
		if err != nil {
			return err
		}
		*p = percentage
		return nil
	case float64:
		// Stored as float (convert to scaled integer)
		percentage, err := NewPercentageFromFloat(v)
		if err != nil {
			return err
		}
		*p = percentage
		return nil
	case string:
		// Stored as string
		percentage, err := NewPercentageFromString(v)
		if err != nil {
			return err
		}
		*p = percentage
		return nil
	case []byte:
		// Stored as bytes
		percentage, err := NewPercentageFromString(string(v))
		if err != nil {
			return err
		}
		*p = percentage
		return nil
	default:
		return ErrInvalidFormat
	}
}
