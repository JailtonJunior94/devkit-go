package vos

import (
	"database/sql"
	"database/sql/driver"
	"encoding/json"
	"math"
	"strconv"
)

// NullableFloat representa um float64 que pode ser nulo.
// Usa internamente um ponteiro para evitar redundância com o campo Valid.
// Zero value é seguro: representa um valor nulo.
type NullableFloat struct {
	value *float64
}

// NewNullableFloat cria um NullableFloat com um valor válido.
func NewNullableFloat(f float64) NullableFloat {
	return NullableFloat{value: &f}
}

// NewNullableFloatFromPointer cria um NullableFloat a partir de um ponteiro.
// Se o ponteiro for nil, retorna um NullableFloat inválido.
func NewNullableFloatFromPointer(f *float64) NullableFloat {
	if f == nil {
		return NullableFloat{}
	}
	return NullableFloat{value: f}
}

// NewNullableFloatFromSQL cria um NullableFloat a partir de sql.NullFloat64.
func NewNullableFloatFromSQL(nf sql.NullFloat64) NullableFloat {
	if !nf.Valid {
		return NullableFloat{}
	}
	return NewNullableFloat(nf.Float64)
}

// IsValid retorna true se o valor é válido (não nulo).
func (n NullableFloat) IsValid() bool {
	return n.value != nil
}

// Get retorna o valor float64 e um booleano indicando se é válido.
// Esta é a abordagem idiomática em Go para valores opcionais.
func (n NullableFloat) Get() (float64, bool) {
	if n.value == nil {
		return 0, false
	}
	return *n.value, true
}

// ValueOr retorna o valor se válido, ou o valor padrão fornecido.
func (n NullableFloat) ValueOr(defaultValue float64) float64 {
	if n.value == nil {
		return defaultValue
	}
	return *n.value
}

// Ptr retorna um ponteiro para o valor, ou nil se inválido.
// Útil para interoperabilidade com código que usa *float64.
func (n NullableFloat) Ptr() *float64 {
	return n.value
}

// ToSQL converte para sql.NullFloat64.
func (n NullableFloat) ToSQL() sql.NullFloat64 {
	if n.value == nil {
		return sql.NullFloat64{Valid: false}
	}
	return sql.NullFloat64{Float64: *n.value, Valid: true}
}

// Float64 retorna o valor como float64, ou 0 se inválido.
// Use Get() se precisar distinguir entre 0 e inválido.
func (n NullableFloat) Float64() float64 {
	if n.value == nil {
		return 0
	}
	return *n.value
}

// Float64Or retorna o valor como float64, ou o valor padrão se inválido.
func (n NullableFloat) Float64Or(defaultValue float64) float64 {
	if n.value == nil {
		return defaultValue
	}
	return *n.value
}

// String retorna o valor formatado como string ou string vazia se inválido.
func (n NullableFloat) String() string {
	if n.value == nil {
		return ""
	}
	return strconv.FormatFloat(*n.value, 'f', -1, 64)
}

// StringOr retorna o valor formatado ou o padrão se inválido.
func (n NullableFloat) StringOr(defaultValue string) string {
	if n.value == nil {
		return defaultValue
	}
	return strconv.FormatFloat(*n.value, 'f', -1, 64)
}

// Format retorna o valor formatado com precisão específica.
// Retorna string vazia se inválido.
func (n NullableFloat) Format(precision int) string {
	if n.value == nil {
		return ""
	}
	return strconv.FormatFloat(*n.value, 'f', precision, 64)
}

// FormatOr retorna o valor formatado ou o padrão se inválido.
func (n NullableFloat) FormatOr(precision int, defaultValue string) string {
	if n.value == nil {
		return defaultValue
	}
	return strconv.FormatFloat(*n.value, 'f', precision, 64)
}

// IsZero retorna true se o valor é válido e igual a 0.
// Retorna false se o valor é inválido.
func (n NullableFloat) IsZero() bool {
	if n.value == nil {
		return false
	}
	return *n.value == 0
}

// IsPositive retorna true se o valor é válido e > 0.
func (n NullableFloat) IsPositive() bool {
	if n.value == nil {
		return false
	}
	return *n.value > 0
}

// IsNegative retorna true se o valor é válido e < 0.
func (n NullableFloat) IsNegative() bool {
	if n.value == nil {
		return false
	}
	return *n.value < 0
}

// IsNaN retorna true se o valor é válido e NaN.
func (n NullableFloat) IsNaN() bool {
	if n.value == nil {
		return false
	}
	return math.IsNaN(*n.value)
}

// IsInf retorna true se o valor é válido e infinito.
func (n NullableFloat) IsInf() bool {
	if n.value == nil {
		return false
	}
	return math.IsInf(*n.value, 0)
}

// Abs retorna o valor absoluto, ou 0 se inválido.
func (n NullableFloat) Abs() float64 {
	if n.value == nil {
		return 0
	}
	return math.Abs(*n.value)
}

// Round arredonda para N casas decimais, retorna 0 se inválido.
func (n NullableFloat) Round(decimals int) float64 {
	if n.value == nil {
		return 0
	}
	ratio := math.Pow(10, float64(decimals))
	return math.Round(*n.value*ratio) / ratio
}

// Scan implementa sql.Scanner para leitura do banco de dados.
func (n *NullableFloat) Scan(value any) error {
	var nf sql.NullFloat64
	if err := nf.Scan(value); err != nil {
		return err
	}
	*n = NewNullableFloatFromSQL(nf)
	return nil
}

// Value implementa driver.Valuer para escrita no banco de dados.
func (n NullableFloat) Value() (driver.Value, error) {
	if n.value == nil {
		return nil, nil
	}
	return *n.value, nil
}

// MarshalJSON implementa json.Marshaler.
// Valores nulos são serializados como null.
func (n NullableFloat) MarshalJSON() ([]byte, error) {
	if n.value == nil {
		return []byte("null"), nil
	}
	return json.Marshal(*n.value)
}

// UnmarshalJSON implementa json.Unmarshaler.
func (n *NullableFloat) UnmarshalJSON(data []byte) error {
	if string(data) == "null" {
		n.value = nil
		return nil
	}
	var f float64
	if err := json.Unmarshal(data, &f); err != nil {
		return err
	}
	n.value = &f
	return nil
}
