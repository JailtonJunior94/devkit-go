package nullable

import (
	"database/sql"
	"database/sql/driver"
	"encoding/json"
	"strconv"
)

// Float64 is a nullable float64 value object.
// The zero value is safe and represents absence (null).
type Float64 struct {
	val *float64
}

// Float64Of returns a Float64 with the given value present.
func Float64Of(v float64) Float64 { return Float64{val: &v} }

// Float64Empty returns a Float64 with no value (null).
func Float64Empty() Float64 { return Float64{} }

// Float64FromPtr creates a Float64 from a pointer. Nil pointer results in null.
// The pointed-to value is copied; mutations to the original pointer do not
// affect the returned Float64.
func Float64FromPtr(v *float64) Float64 {
	if v == nil {
		return Float64{}
	}
	copied := *v
	return Float64{val: &copied}
}

// IsNull returns true when no value is present.
func (n Float64) IsNull() bool { return n.val == nil }

// Get returns the value and whether it is present.
func (n Float64) Get() (float64, bool) {
	if n.val == nil {
		return 0, false
	}
	return *n.val, true
}

// ValueOr returns the value if present, or fallback otherwise.
func (n Float64) ValueOr(fallback float64) float64 {
	if n.val == nil {
		return fallback
	}
	return *n.val
}

// Ptr returns a pointer to the value, or nil if absent.
func (n Float64) Ptr() *float64 { return n.val }

// Equal returns true when both are null or both hold the same value.
func (n Float64) Equal(other Float64) bool {
	if n.val == nil && other.val == nil {
		return true
	}
	if n.val == nil || other.val == nil {
		return false
	}
	return *n.val == *other.val
}

// String implements fmt.Stringer. Returns "<null>" when absent.
func (n Float64) String() string {
	if n.val == nil {
		return "<null>"
	}
	return strconv.FormatFloat(*n.val, 'f', -1, 64)
}

// MarshalJSON implements json.Marshaler.
// Absent values are encoded as JSON null.
func (n Float64) MarshalJSON() ([]byte, error) {
	if n.val == nil {
		return []byte("null"), nil
	}
	return json.Marshal(*n.val)
}

// UnmarshalJSON implements json.Unmarshaler.
// JSON null sets the value to absent.
func (n *Float64) UnmarshalJSON(data []byte) error {
	if string(data) == "null" {
		n.val = nil
		return nil
	}
	var v float64
	if err := json.Unmarshal(data, &v); err != nil {
		return err
	}
	n.val = &v
	return nil
}

// Value implements driver.Valuer for database writes.
func (n Float64) Value() (driver.Value, error) {
	if n.val == nil {
		return nil, nil
	}
	return *n.val, nil
}

// Scan implements sql.Scanner for database reads.
// Delegates to sql.NullFloat64.
func (n *Float64) Scan(value any) error {
	var s sql.NullFloat64
	if err := s.Scan(value); err != nil {
		return err
	}
	if !s.Valid {
		n.val = nil
		return nil
	}
	n.val = &s.Float64
	return nil
}
