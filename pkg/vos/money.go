package vos

import (
	"database/sql/driver"
	"encoding/json"
	"fmt"
	"math"
	"strconv"
	"strings"
)

// Money represents a monetary value with precision and currency.
// It is a DDD Value Object that is immutable, thread-safe, and safe for financial calculations.
//
// Internal representation uses int64 for cents (smallest unit) to avoid floating-point precision issues.
// Scale is fixed at 2 decimal places (e.g., 1000 = 10.00).
//
// Example:
//
//	m, _ := NewMoney(1050, CurrencyBRL)  // 10.50 BRL
//	m2, _ := NewMoneyFromFloat(10.50, CurrencyUSD)  // 10.50 USD (less precise, not recommended)
type Money struct {
	cents    int64    // Value in smallest unit (cents) - immutable
	currency Currency // ISO 4217 currency code - immutable
}

const (
	moneyScaleFactor = 100     // 10^scale = 100 for 2 decimal places
	maxMoneyCents    = 1 << 53 // Safe integer limit for int64 operations (~90 trillion)
)

// NewMoney creates a new Money value object from cents (smallest unit) and currency.
// This is the recommended constructor for precision-critical operations.
//
// Parameters:
//   - cents: The amount in smallest unit (e.g., cents for USD/BRL, yen for JPY)
//   - currency: ISO 4217 currency code (must be valid)
//
// Returns ErrInvalidCurrency if currency is invalid.
//
// Example:
//
//	m, err := NewMoney(1050, CurrencyBRL)  // 10.50 BRL
//	if err != nil {
//	    log.Fatal(err)
//	}
func NewMoney(cents int64, currency Currency) (Money, error) {
	if !currency.IsValid() {
		return Money{}, ErrInvalidCurrency
	}

	// Check for potential overflow in operations
	if cents > maxMoneyCents || cents < -maxMoneyCents {
		return Money{}, ErrOverflow
	}

	return Money{
		cents:    cents,
		currency: currency,
	}, nil
}

// NewMoneyFromFloat creates Money from a float64 value.
// This is less precise due to floating-point representation and should be avoided when possible.
// Use NewMoney with cents for precision-critical operations.
//
// Example:
//
//	m, err := NewMoneyFromFloat(10.50, CurrencyUSD)  // 10.50 USD
func NewMoneyFromFloat(value float64, currency Currency) (Money, error) {
	if !currency.IsValid() {
		return Money{}, ErrInvalidCurrency
	}

	if math.IsNaN(value) || math.IsInf(value, 0) {
		return Money{}, ErrInvalidValue
	}

	cents := int64(math.Round(value * moneyScaleFactor))

	if cents > maxMoneyCents || cents < -maxMoneyCents {
		return Money{}, ErrOverflow
	}

	return Money{
		cents:    cents,
		currency: currency,
	}, nil
}

// NewMoneyFromString creates Money from a string representation.
// Accepts formats: "10.50", "10,50", "10.5", "10"
//
// Example:
//
//	m, err := NewMoneyFromString("10.50", CurrencyBRL)
func NewMoneyFromString(value string, currency Currency) (Money, error) {
	if !currency.IsValid() {
		return Money{}, ErrInvalidCurrency
	}

	// Normalize decimal separator
	value = strings.TrimSpace(value)
	value = strings.ReplaceAll(value, ",", ".")

	// Parse as float64
	floatValue, err := strconv.ParseFloat(value, 64)
	if err != nil {
		return Money{}, ErrInvalidFormat
	}

	return NewMoneyFromFloat(floatValue, currency)
}

// Cents returns the raw value in smallest unit (cents).
// Useful for database storage and precise calculations.
func (m Money) Cents() int64 {
	return m.cents
}

// Currency returns the ISO 4217 currency code.
func (m Money) Currency() Currency {
	return m.currency
}

// Float returns the value as float64.
// WARNING: Use only for display purposes, not for calculations due to precision loss.
func (m Money) Float() float64 {
	return float64(m.cents) / moneyScaleFactor
}

// Add adds two Money values.
// Returns ErrCurrencyMismatch if currencies don't match.
// Returns ErrOverflow if result would overflow.
func (m Money) Add(other Money) (Money, error) {
	if m.currency != other.currency {
		return Money{}, ErrCurrencyMismatch
	}

	// Check for overflow
	if (other.cents > 0 && m.cents > maxMoneyCents-other.cents) ||
		(other.cents < 0 && m.cents < -maxMoneyCents-other.cents) {
		return Money{}, ErrOverflow
	}

	return Money{
		cents:    m.cents + other.cents,
		currency: m.currency,
	}, nil
}

// Subtract subtracts another Money value from this one.
// Returns ErrCurrencyMismatch if currencies don't match.
// Returns ErrOverflow if result would overflow.
func (m Money) Subtract(other Money) (Money, error) {
	if m.currency != other.currency {
		return Money{}, ErrCurrencyMismatch
	}

	// Check for overflow
	if (other.cents < 0 && m.cents > maxMoneyCents+other.cents) ||
		(other.cents > 0 && m.cents < -maxMoneyCents+other.cents) {
		return Money{}, ErrOverflow
	}

	return Money{
		cents:    m.cents - other.cents,
		currency: m.currency,
	}, nil
}

// Multiply multiplies Money by an integer factor.
// Returns ErrOverflow if result would overflow.
func (m Money) Multiply(factor int64) (Money, error) {
	// Check for overflow
	if factor != 0 && (m.cents > maxMoneyCents/factor || m.cents < -maxMoneyCents/factor) {
		return Money{}, ErrOverflow
	}

	return Money{
		cents:    m.cents * factor,
		currency: m.currency,
	}, nil
}

// Divide divides Money by an integer divisor.
// Returns ErrDivisionByZero if divisor is zero.
// Result is truncated towards zero.
func (m Money) Divide(divisor int64) (Money, error) {
	if divisor == 0 {
		return Money{}, ErrDivisionByZero
	}

	return Money{
		cents:    m.cents / divisor,
		currency: m.currency,
	}, nil
}

// Equals checks if two Money values are equal (same amount and currency).
func (m Money) Equals(other Money) bool {
	return m.cents == other.cents && m.currency == other.currency
}

// GreaterThan checks if this Money is greater than another.
// Returns false if currencies don't match.
func (m Money) GreaterThan(other Money) bool {
	if m.currency != other.currency {
		return false
	}
	return m.cents > other.cents
}

// LessThan checks if this Money is less than another.
// Returns false if currencies don't match.
func (m Money) LessThan(other Money) bool {
	if m.currency != other.currency {
		return false
	}
	return m.cents < other.cents
}

// GreaterThanOrEqual checks if this Money is greater than or equal to another.
// Returns false if currencies don't match.
func (m Money) GreaterThanOrEqual(other Money) bool {
	if m.currency != other.currency {
		return false
	}
	return m.cents >= other.cents
}

// LessThanOrEqual checks if this Money is less than or equal to another.
// Returns false if currencies don't match.
func (m Money) LessThanOrEqual(other Money) bool {
	if m.currency != other.currency {
		return false
	}
	return m.cents <= other.cents
}

// IsZero checks if the value is zero.
func (m Money) IsZero() bool {
	return m.cents == 0
}

// IsPositive checks if the value is positive (> 0).
func (m Money) IsPositive() bool {
	return m.cents > 0
}

// IsNegative checks if the value is negative (< 0).
func (m Money) IsNegative() bool {
	return m.cents < 0
}

// Abs returns the absolute value.
func (m Money) Abs() Money {
	if m.cents < 0 {
		return Money{
			cents:    -m.cents,
			currency: m.currency,
		}
	}
	return m
}

// Negate returns the negated value.
func (m Money) Negate() Money {
	return Money{
		cents:    -m.cents,
		currency: m.currency,
	}
}

// String returns a human-readable string representation.
// Format: "10.50 BRL".
func (m Money) String() string {
	return fmt.Sprintf("%.2f %s", m.Float(), m.currency)
}

// MarshalJSON implements json.Marshaler.
// Serializes as JSON object: {"amount": "10.50", "currency": "BRL"}.
func (m Money) MarshalJSON() ([]byte, error) {
	type moneyJSON struct {
		Amount   string   `json:"amount"`
		Currency Currency `json:"currency"`
	}

	return json.Marshal(moneyJSON{
		Amount:   fmt.Sprintf("%.2f", m.Float()),
		Currency: m.currency,
	})
}

// UnmarshalJSON implements json.Unmarshaler.
// Accepts JSON object: {"amount": "10.50", "currency": "BRL"}
// Also accepts string amount: "10.50" (requires currency to be set elsewhere).
func (m *Money) UnmarshalJSON(data []byte) error {
	// Try to unmarshal as object first
	type moneyJSON struct {
		Amount   string   `json:"amount"`
		Currency Currency `json:"currency"`
	}

	var obj moneyJSON
	if err := json.Unmarshal(data, &obj); err == nil && obj.Amount != "" {
		money, err := NewMoneyFromString(obj.Amount, obj.Currency)
		if err != nil {
			return err
		}
		*m = money
		return nil
	}

	// Try to unmarshal as string (amount only)
	var amountStr string
	if err := json.Unmarshal(data, &amountStr); err != nil {
		return ErrInvalidFormat
	}

	// If we only have amount, we need currency to be already set or use a default
	// For safety, we require the full object format
	return ErrInvalidFormat
}

// Value implements driver.Valuer for database persistence.
// Stores as NUMERIC/DECIMAL in database with full precision.
// Format: Store cents as-is for precision, or as decimal string.
func (m Money) Value() (driver.Value, error) {
	// Store as string in format: "cents:currency" for atomic storage
	// Example: "1050:BRL" for 10.50 BRL
	return fmt.Sprintf("%d:%s", m.cents, m.currency), nil
}

// Scan implements sql.Scanner for database retrieval.
// Reads from NUMERIC/DECIMAL database column.
func (m *Money) Scan(value any) error {
	if value == nil {
		return ErrNullValue
	}

	var str string
	switch v := value.(type) {
	case string:
		str = v
	case []byte:
		str = string(v)
	case int64:
		// If stored as integer (cents only), assume BRL as default
		// This is a fallback - proper storage should include currency
		money, err := NewMoney(v, CurrencyBRL)
		if err != nil {
			return err
		}
		*m = money
		return nil
	case float64:
		// If stored as float (not recommended), assume BRL as default
		money, err := NewMoneyFromFloat(v, CurrencyBRL)
		if err != nil {
			return err
		}
		*m = money
		return nil
	default:
		return ErrInvalidFormat
	}

	// Parse format: "cents:currency"
	parts := strings.Split(str, ":")
	if len(parts) != 2 {
		return ErrInvalidFormat
	}

	cents, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil {
		return ErrInvalidFormat
	}

	currency, err := NewCurrency(parts[1])
	if err != nil {
		return err
	}

	money, err := NewMoney(cents, currency)
	if err != nil {
		return err
	}

	*m = money
	return nil
}
