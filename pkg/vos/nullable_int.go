package vos

import (
	"database/sql"
	"database/sql/driver"
	"encoding/json"
	"strconv"
)

// NullableInt representa um int64 que pode ser nulo.
// Usa internamente um ponteiro para evitar redundância com o campo Valid.
// Zero value é seguro: representa um valor nulo.
type NullableInt struct {
	value *int64
}

// NewNullableInt cria um NullableInt com um valor válido.
func NewNullableInt(v int64) NullableInt {
	return NullableInt{value: &v}
}

// NewNullableIntFromPointer cria um NullableInt a partir de um ponteiro.
// Se o ponteiro for nil, retorna um NullableInt inválido.
func NewNullableIntFromPointer(v *int64) NullableInt {
	if v == nil {
		return NullableInt{}
	}
	return NullableInt{value: v}
}

// NewNullableIntFromSQL cria um NullableInt a partir de sql.NullInt64.
func NewNullableIntFromSQL(n sql.NullInt64) NullableInt {
	if !n.Valid {
		return NullableInt{}
	}
	return NewNullableInt(n.Int64)
}

// IsValid retorna true se o valor é válido (não nulo).
func (n NullableInt) IsValid() bool {
	return n.value != nil
}

// Get retorna o valor int64 e um booleano indicando se é válido.
// Esta é a abordagem idiomática em Go para valores opcionais.
func (n NullableInt) Get() (int64, bool) {
	if n.value == nil {
		return 0, false
	}
	return *n.value, true
}

// ValueOr retorna o valor se válido, ou o valor padrão fornecido.
func (n NullableInt) ValueOr(defaultValue int64) int64 {
	if n.value == nil {
		return defaultValue
	}
	return *n.value
}

// Ptr retorna um ponteiro para o valor, ou nil se inválido.
// Útil para interoperabilidade com código que usa *int64.
func (n NullableInt) Ptr() *int64 {
	return n.value
}

// ToSQL converte para sql.NullInt64.
func (n NullableInt) ToSQL() sql.NullInt64 {
	if n.value == nil {
		return sql.NullInt64{Valid: false}
	}
	return sql.NullInt64{Int64: *n.value, Valid: true}
}

// String retorna o valor como string ou string vazia se inválido.
func (n NullableInt) String() string {
	if n.value == nil {
		return ""
	}
	return strconv.FormatInt(*n.value, 10)
}

// StringOr retorna o valor como string ou o valor padrão se inválido.
func (n NullableInt) StringOr(defaultValue string) string {
	if n.value == nil {
		return defaultValue
	}
	return strconv.FormatInt(*n.value, 10)
}

// Int retorna o valor como int, ou zero se inválido.
// Use Get() se precisar distinguir entre zero e inválido.
func (n NullableInt) Int() int {
	if n.value == nil {
		return 0
	}
	return int(*n.value)
}

// IntOr retorna o valor como int, ou o valor padrão se inválido.
func (n NullableInt) IntOr(defaultValue int) int {
	if n.value == nil {
		return defaultValue
	}
	return int(*n.value)
}

// Scan implementa sql.Scanner para leitura do banco de dados.
func (n *NullableInt) Scan(value interface{}) error {
	var sqlInt sql.NullInt64
	if err := sqlInt.Scan(value); err != nil {
		return err
	}
	*n = NewNullableIntFromSQL(sqlInt)
	return nil
}

// Value implementa driver.Valuer para escrita no banco de dados.
func (n NullableInt) Value() (driver.Value, error) {
	if n.value == nil {
		return nil, nil
	}
	return *n.value, nil
}

// MarshalJSON implementa json.Marshaler.
// Valores nulos são serializados como null.
func (n NullableInt) MarshalJSON() ([]byte, error) {
	if n.value == nil {
		return []byte("null"), nil
	}
	return json.Marshal(*n.value)
}

// UnmarshalJSON implementa json.Unmarshaler.
func (n *NullableInt) UnmarshalJSON(data []byte) error {
	if string(data) == "null" {
		n.value = nil
		return nil
	}
	var v int64
	if err := json.Unmarshal(data, &v); err != nil {
		return err
	}
	n.value = &v
	return nil
}

// --- Funções Utilitárias Globais ---

// IntToNullable converte *int64 para NullableInt de forma segura.
func IntToNullable(v *int64) NullableInt {
	return NewNullableIntFromPointer(v)
}

// SQLIntToNullable converte sql.NullInt64 para NullableInt de forma segura.
func SQLIntToNullable(n sql.NullInt64) NullableInt {
	return NewNullableIntFromSQL(n)
}

// NullableToInt converte NullableInt para *int64 de forma segura.
func NullableToInt(n NullableInt) *int64 {
	return n.Ptr()
}

// NullableToSQLInt converte NullableInt para sql.NullInt64 de forma segura.
func NullableToSQLInt(n NullableInt) sql.NullInt64 {
	return n.ToSQL()
}

// SafeIntToString converte *int64 para string de forma segura, retornando string vazia se nil.
func SafeIntToString(v *int64) string {
	if v == nil {
		return ""
	}
	return strconv.FormatInt(*v, 10)
}

// SafeIntToStringOr converte *int64 para string de forma segura, retornando defaultValue se nil.
func SafeIntToStringOr(v *int64, defaultValue string) string {
	if v == nil {
		return defaultValue
	}
	return strconv.FormatInt(*v, 10)
}
