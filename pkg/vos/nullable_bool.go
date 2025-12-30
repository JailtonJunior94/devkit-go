package vos

import (
	"database/sql"
	"database/sql/driver"
	"encoding/json"
	"strconv"
)

// NullableBool representa um bool que pode ser nulo.
// Usa internamente um ponteiro para evitar redundância com o campo Valid.
// Zero value é seguro: representa um valor nulo.
type NullableBool struct {
	value *bool
}

// NewNullableBool cria um NullableBool com um valor válido.
func NewNullableBool(b bool) NullableBool {
	return NullableBool{value: &b}
}

// NewNullableBoolFromPointer cria um NullableBool a partir de um ponteiro.
// Se o ponteiro for nil, retorna um NullableBool inválido.
func NewNullableBoolFromPointer(b *bool) NullableBool {
	if b == nil {
		return NullableBool{}
	}
	return NullableBool{value: b}
}

// NewNullableBoolFromSQL cria um NullableBool a partir de sql.NullBool.
func NewNullableBoolFromSQL(nb sql.NullBool) NullableBool {
	if !nb.Valid {
		return NullableBool{}
	}
	return NewNullableBool(nb.Bool)
}

// IsValid retorna true se o valor é válido (não nulo).
func (n NullableBool) IsValid() bool {
	return n.value != nil
}

// Get retorna o valor bool e um booleano indicando se é válido.
// Esta é a abordagem idiomática em Go para valores opcionais.
func (n NullableBool) Get() (bool, bool) {
	if n.value == nil {
		return false, false
	}
	return *n.value, true
}

// ValueOr retorna o valor se válido, ou o valor padrão fornecido.
func (n NullableBool) ValueOr(defaultValue bool) bool {
	if n.value == nil {
		return defaultValue
	}
	return *n.value
}

// Ptr retorna um ponteiro para o valor, ou nil se inválido.
// Útil para interoperabilidade com código que usa *bool.
func (n NullableBool) Ptr() *bool {
	return n.value
}

// ToSQL converte para sql.NullBool.
func (n NullableBool) ToSQL() sql.NullBool {
	if n.value == nil {
		return sql.NullBool{Valid: false}
	}
	return sql.NullBool{Bool: *n.value, Valid: true}
}

// Bool retorna o valor como bool, ou false se inválido.
// IMPORTANTE: Use Get() se precisar distinguir entre false e inválido.
func (n NullableBool) Bool() bool {
	if n.value == nil {
		return false
	}
	return *n.value
}

// IsTrue retorna true se o valor é válido E true.
// Retorna false se o valor é inválido ou false.
func (n NullableBool) IsTrue() bool {
	if n.value == nil {
		return false
	}
	return *n.value
}

// IsFalse retorna true se o valor é válido E false.
// Retorna false se o valor é inválido.
func (n NullableBool) IsFalse() bool {
	if n.value == nil {
		return false
	}
	return !(*n.value)
}

// String retorna "true", "false" ou string vazia se inválido.
func (n NullableBool) String() string {
	if n.value == nil {
		return ""
	}
	return strconv.FormatBool(*n.value)
}

// StringOr retorna "true"/"false" ou o valor padrão se inválido.
func (n NullableBool) StringOr(defaultValue string) string {
	if n.value == nil {
		return defaultValue
	}
	return strconv.FormatBool(*n.value)
}

// Scan implementa sql.Scanner para leitura do banco de dados.
func (n *NullableBool) Scan(value interface{}) error {
	var nb sql.NullBool
	if err := nb.Scan(value); err != nil {
		return err
	}
	*n = NewNullableBoolFromSQL(nb)
	return nil
}

// Value implementa driver.Valuer para escrita no banco de dados.
func (n NullableBool) Value() (driver.Value, error) {
	if n.value == nil {
		return nil, nil
	}
	return *n.value, nil
}

// MarshalJSON implementa json.Marshaler.
// Valores nulos são serializados como null.
func (n NullableBool) MarshalJSON() ([]byte, error) {
	if n.value == nil {
		return []byte("null"), nil
	}
	return json.Marshal(*n.value)
}

// UnmarshalJSON implementa json.Unmarshaler.
func (n *NullableBool) UnmarshalJSON(data []byte) error {
	if string(data) == "null" {
		n.value = nil
		return nil
	}
	var b bool
	if err := json.Unmarshal(data, &b); err != nil {
		return err
	}
	n.value = &b
	return nil
}

// --- Funções Utilitárias Globais ---

// BoolToNullable converte *bool para NullableBool de forma segura.
func BoolToNullable(b *bool) NullableBool {
	return NewNullableBoolFromPointer(b)
}

// SQLBoolToNullable converte sql.NullBool para NullableBool de forma segura.
func SQLBoolToNullable(nb sql.NullBool) NullableBool {
	return NewNullableBoolFromSQL(nb)
}

// NullableToBool converte NullableBool para *bool de forma segura.
func NullableToBool(n NullableBool) *bool {
	return n.Ptr()
}

// NullableToSQLBool converte NullableBool para sql.NullBool de forma segura.
func NullableToSQLBool(n NullableBool) sql.NullBool {
	return n.ToSQL()
}

// SafeBoolToString converte *bool para string de forma segura, retornando string vazia se nil.
func SafeBoolToString(b *bool) string {
	if b == nil {
		return ""
	}
	return strconv.FormatBool(*b)
}

// SafeBoolToStringOr converte *bool para string de forma segura, retornando defaultValue se nil.
func SafeBoolToStringOr(b *bool, defaultValue string) string {
	if b == nil {
		return defaultValue
	}
	return strconv.FormatBool(*b)
}
