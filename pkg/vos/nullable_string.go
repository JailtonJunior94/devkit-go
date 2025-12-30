package vos

import (
	"database/sql"
	"database/sql/driver"
	"encoding/json"
	"strings"
)

// NullableString representa uma string que pode ser nula.
// Usa internamente um ponteiro para evitar redundância com o campo Valid.
// Zero value é seguro: representa um valor nulo.
type NullableString struct {
	value *string
}

// NewNullableString cria um NullableString com um valor válido.
func NewNullableString(s string) NullableString {
	return NullableString{value: &s}
}

// NewNullableStringFromPointer cria um NullableString a partir de um ponteiro.
// Se o ponteiro for nil, retorna um NullableString inválido.
func NewNullableStringFromPointer(s *string) NullableString {
	if s == nil {
		return NullableString{}
	}
	return NullableString{value: s}
}

// NewNullableStringFromSQL cria um NullableString a partir de sql.NullString.
func NewNullableStringFromSQL(ns sql.NullString) NullableString {
	if !ns.Valid {
		return NullableString{}
	}
	return NewNullableString(ns.String)
}

// IsValid retorna true se o valor é válido (não nulo).
func (n NullableString) IsValid() bool {
	return n.value != nil
}

// Get retorna o valor string e um booleano indicando se é válido.
// Esta é a abordagem idiomática em Go para valores opcionais.
func (n NullableString) Get() (string, bool) {
	if n.value == nil {
		return "", false
	}
	return *n.value, true
}

// ValueOr retorna o valor se válido, ou o valor padrão fornecido.
func (n NullableString) ValueOr(defaultValue string) string {
	if n.value == nil {
		return defaultValue
	}
	return *n.value
}

// Ptr retorna um ponteiro para o valor, ou nil se inválido.
// Útil para interoperabilidade com código que usa *string.
func (n NullableString) Ptr() *string {
	return n.value
}

// ToSQL converte para sql.NullString.
func (n NullableString) ToSQL() sql.NullString {
	if n.value == nil {
		return sql.NullString{Valid: false}
	}
	return sql.NullString{String: *n.value, Valid: true}
}

// String retorna o valor como string ou string vazia se inválido.
// Implementa fmt.Stringer.
func (n NullableString) String() string {
	if n.value == nil {
		return ""
	}
	return *n.value
}

// StringOr retorna o valor ou o padrão fornecido se inválido.
func (n NullableString) StringOr(defaultValue string) string {
	if n.value == nil {
		return defaultValue
	}
	return *n.value
}

// IsEmpty retorna true se o valor é inválido OU é uma string vazia.
func (n NullableString) IsEmpty() bool {
	if n.value == nil {
		return true
	}
	return *n.value == ""
}

// Len retorna o comprimento da string, ou 0 se inválido.
func (n NullableString) Len() int {
	if n.value == nil {
		return 0
	}
	return len(*n.value)
}

// ToUpper retorna o valor em maiúsculas, ou string vazia se inválido.
func (n NullableString) ToUpper() string {
	if n.value == nil {
		return ""
	}
	return strings.ToUpper(*n.value)
}

// ToLower retorna o valor em minúsculas, ou string vazia se inválido.
func (n NullableString) ToLower() string {
	if n.value == nil {
		return ""
	}
	return strings.ToLower(*n.value)
}

// TrimSpace retorna o valor sem espaços nas extremidades, ou string vazia se inválido.
func (n NullableString) TrimSpace() string {
	if n.value == nil {
		return ""
	}
	return strings.TrimSpace(*n.value)
}

// Contains verifica se o valor contém a substring fornecida.
// Retorna false se o valor é inválido.
func (n NullableString) Contains(substr string) bool {
	if n.value == nil {
		return false
	}
	return strings.Contains(*n.value, substr)
}

// Scan implementa sql.Scanner para leitura do banco de dados.
func (n *NullableString) Scan(value interface{}) error {
	var ns sql.NullString
	if err := ns.Scan(value); err != nil {
		return err
	}
	*n = NewNullableStringFromSQL(ns)
	return nil
}

// Value implementa driver.Valuer para escrita no banco de dados.
func (n NullableString) Value() (driver.Value, error) {
	if n.value == nil {
		return nil, nil
	}
	return *n.value, nil
}

// MarshalJSON implementa json.Marshaler.
// Valores nulos são serializados como null.
func (n NullableString) MarshalJSON() ([]byte, error) {
	if n.value == nil {
		return []byte("null"), nil
	}
	return json.Marshal(*n.value)
}

// UnmarshalJSON implementa json.Unmarshaler.
func (n *NullableString) UnmarshalJSON(data []byte) error {
	if string(data) == "null" {
		n.value = nil
		return nil
	}
	var s string
	if err := json.Unmarshal(data, &s); err != nil {
		return err
	}
	n.value = &s
	return nil
}

// --- Funções Utilitárias Globais ---

// StringToNullable converte *string para NullableString de forma segura.
func StringToNullable(s *string) NullableString {
	return NewNullableStringFromPointer(s)
}

// SQLStringToNullable converte sql.NullString para NullableString de forma segura.
func SQLStringToNullable(ns sql.NullString) NullableString {
	return NewNullableStringFromSQL(ns)
}

// NullableToString converte NullableString para *string de forma segura.
func NullableToString(n NullableString) *string {
	return n.Ptr()
}

// NullableToSQLString converte NullableString para sql.NullString de forma segura.
func NullableToSQLString(n NullableString) sql.NullString {
	return n.ToSQL()
}

// SafeStringValue retorna o valor da string ou string vazia se nil.
func SafeStringValue(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

// SafeStringValueOr retorna o valor da string ou defaultValue se nil.
func SafeStringValueOr(s *string, defaultValue string) string {
	if s == nil {
		return defaultValue
	}
	return *s
}
