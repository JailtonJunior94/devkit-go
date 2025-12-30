package vos

import (
	"database/sql/driver"
	"encoding/json"
	"strings"
)

// Currency represents an ISO 4217 currency code.
// It provides type safety and validation for monetary operations.
type Currency string

// Standard ISO 4217 currency codes.
const (
	CurrencyBRL Currency = "BRL" // Brazilian Real
	CurrencyUSD Currency = "USD" // United States Dollar
	CurrencyEUR Currency = "EUR" // Euro
	CurrencyGBP Currency = "GBP" // British Pound Sterling
	CurrencyJPY Currency = "JPY" // Japanese Yen
	CurrencyCAD Currency = "CAD" // Canadian Dollar
	CurrencyAUD Currency = "AUD" // Australian Dollar
	CurrencyCHF Currency = "CHF" // Swiss Franc
	CurrencyCNY Currency = "CNY" // Chinese Yuan
	CurrencyINR Currency = "INR" // Indian Rupee
	CurrencyMXN Currency = "MXN" // Mexican Peso
	CurrencyARS Currency = "ARS" // Argentine Peso
)

// validCurrencies holds all supported currency codes for validation.
var validCurrencies = map[Currency]bool{
	CurrencyBRL: true,
	CurrencyUSD: true,
	CurrencyEUR: true,
	CurrencyGBP: true,
	CurrencyJPY: true,
	CurrencyCAD: true,
	CurrencyAUD: true,
	CurrencyCHF: true,
	CurrencyCNY: true,
	CurrencyINR: true,
	CurrencyMXN: true,
	CurrencyARS: true,
}

// NewCurrency creates a validated Currency from a string code.
// Returns ErrInvalidCurrency if the code is not recognized.
func NewCurrency(code string) (Currency, error) {
	c := Currency(strings.ToUpper(code))
	if !c.IsValid() {
		return "", ErrInvalidCurrency
	}
	return c, nil
}

// IsValid checks if the currency code is valid according to ISO 4217.
func (c Currency) IsValid() bool {
	return validCurrencies[c]
}

// String returns the currency code as a string.
func (c Currency) String() string {
	return string(c)
}

// MarshalJSON implements json.Marshaler.
func (c Currency) MarshalJSON() ([]byte, error) {
	return json.Marshal(string(c))
}

// UnmarshalJSON implements json.Unmarshaler.
func (c *Currency) UnmarshalJSON(data []byte) error {
	var code string
	if err := json.Unmarshal(data, &code); err != nil {
		return err
	}

	currency, err := NewCurrency(code)
	if err != nil {
		return err
	}

	*c = currency
	return nil
}

// Value implements driver.Valuer for database persistence.
func (c Currency) Value() (driver.Value, error) {
	if !c.IsValid() {
		return nil, ErrInvalidCurrency
	}
	return string(c), nil
}

// Scan implements sql.Scanner for database retrieval.
func (c *Currency) Scan(value any) error {
	if value == nil {
		return ErrNullValue
	}

	var code string
	switch v := value.(type) {
	case string:
		code = v
	case []byte:
		code = string(v)
	default:
		return ErrInvalidFormat
	}

	currency, err := NewCurrency(code)
	if err != nil {
		return err
	}

	*c = currency
	return nil
}
