package vos

import (
	"database/sql"
	"database/sql/driver"
	"encoding/json"
	"time"
)

// NullableTime representa um time.Time que pode ser nulo.
// Usa internamente um ponteiro para evitar redundância com o campo Valid.
// Zero value é seguro: representa um valor nulo.
type NullableTime struct {
	time *time.Time
}

// NewNullableTime cria um NullableTime com um valor válido.
func NewNullableTime(t time.Time) NullableTime {
	return NullableTime{time: &t}
}

// NewNullableTimeFromPointer cria um NullableTime a partir de um ponteiro.
// Se o ponteiro for nil, retorna um NullableTime inválido.
func NewNullableTimeFromPointer(t *time.Time) NullableTime {
	if t == nil {
		return NullableTime{}
	}
	return NullableTime{time: t}
}

// NewNullableTimeFromSQL cria um NullableTime a partir de sql.NullTime.
func NewNullableTimeFromSQL(nt sql.NullTime) NullableTime {
	if !nt.Valid {
		return NullableTime{}
	}
	return NewNullableTime(nt.Time)
}

// IsValid retorna true se o valor é válido (não nulo).
func (n NullableTime) IsValid() bool {
	return n.time != nil
}

// Get retorna o valor time.Time e um booleano indicando se é válido.
// Esta é a abordagem idiomática em Go para valores opcionais.
func (n NullableTime) Get() (time.Time, bool) {
	if n.time == nil {
		return time.Time{}, false
	}
	return *n.time, true
}

// ValueOr retorna o valor se válido, ou o valor padrão fornecido.
func (n NullableTime) ValueOr(defaultValue time.Time) time.Time {
	if n.time == nil {
		return defaultValue
	}
	return *n.time
}

// Ptr retorna um ponteiro para o valor, ou nil se inválido.
// Útil para interoperabilidade com código que usa *time.Time.
func (n NullableTime) Ptr() *time.Time {
	return n.time
}

// ToSQL converte para sql.NullTime.
func (n NullableTime) ToSQL() sql.NullTime {
	if n.time == nil {
		return sql.NullTime{Valid: false}
	}
	return sql.NullTime{Time: *n.time, Valid: true}
}

// Format retorna o tempo formatado ou string vazia se inválido.
func (n NullableTime) Format(layout string) string {
	if n.time == nil {
		return ""
	}
	return n.time.Format(layout)
}

// FormatOr retorna o tempo formatado ou o valor padrão se inválido.
func (n NullableTime) FormatOr(layout, defaultValue string) string {
	if n.time == nil {
		return defaultValue
	}
	return n.time.Format(layout)
}

// RFC3339 retorna o tempo em formato RFC3339 ou string vazia se inválido.
func (n NullableTime) RFC3339() string {
	return n.Format(time.RFC3339)
}

// Scan implementa sql.Scanner para leitura do banco de dados.
func (n *NullableTime) Scan(value any) error {
	var nt sql.NullTime
	if err := nt.Scan(value); err != nil {
		return err
	}
	*n = NewNullableTimeFromSQL(nt)
	return nil
}

// Value implementa driver.Valuer para escrita no banco de dados.
func (n NullableTime) Value() (driver.Value, error) {
	if n.time == nil {
		return nil, nil
	}
	return *n.time, nil
}

// MarshalJSON implementa json.Marshaler.
// Valores nulos são serializados como null.
func (n NullableTime) MarshalJSON() ([]byte, error) {
	if n.time == nil {
		return []byte("null"), nil
	}
	return json.Marshal(n.time)
}

// UnmarshalJSON implementa json.Unmarshaler.
func (n *NullableTime) UnmarshalJSON(data []byte) error {
	if string(data) == "null" {
		n.time = nil
		return nil
	}
	var t time.Time
	if err := json.Unmarshal(data, &t); err != nil {
		return err
	}
	n.time = &t
	return nil
}
