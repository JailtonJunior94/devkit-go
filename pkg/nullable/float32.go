package nullable

import (
	"database/sql"
	"database/sql/driver"
	"encoding/json"
	"strconv"
)

// Float32 is a nullable float32 value object.
// The zero value is safe and represents absence (null).
//
// SQL round-trip note: database/sql has no NullFloat32. Scan delegates to
// sql.NullFloat64 and casts to float32, which may lose precision for values
// outside the float32 range. Use Float64 when full precision is required.
type Float32 struct {
	val *float32
}

// Float32Of returns a Float32 with the given value present.
func Float32Of(v float32) Float32 { return Float32{val: &v} }

// Float32Empty returns a Float32 with no value (null).
func Float32Empty() Float32 { return Float32{} }

// Float32FromPtr creates a Float32 from a pointer. Nil pointer results in null.
// The pointed-to value is copied; mutations to the original pointer do not
// affect the returned Float32.
func Float32FromPtr(v *float32) Float32 {
	if v == nil {
		return Float32{}
	}
	copied := *v
	return Float32{val: &copied}
}

// IsNull returns true when no value is present.
func (n Float32) IsNull() bool { return n.val == nil }

// Get returns the value and whether it is present.
func (n Float32) Get() (float32, bool) {
	if n.val == nil {
		return 0, false
	}
	return *n.val, true
}

// ValueOr returns the value if present, or fallback otherwise.
func (n Float32) ValueOr(fallback float32) float32 {
	if n.val == nil {
		return fallback
	}
	return *n.val
}

// Ptr returns a pointer to the value, or nil if absent.
func (n Float32) Ptr() *float32 { return n.val }

// Equal returns true when both are null or both hold the same value.
func (n Float32) Equal(other Float32) bool {
	if n.val == nil && other.val == nil {
		return true
	}
	if n.val == nil || other.val == nil {
		return false
	}
	return *n.val == *other.val
}

// String implements fmt.Stringer. Returns "<null>" when absent.
func (n Float32) String() string {
	if n.val == nil {
		return "<null>"
	}
	return strconv.FormatFloat(float64(*n.val), 'f', -1, 32)
}

// MarshalJSON implements json.Marshaler.
// Absent values are encoded as JSON null.
func (n Float32) MarshalJSON() ([]byte, error) {
	if n.val == nil {
		return []byte("null"), nil
	}
	return json.Marshal(*n.val)
}

// UnmarshalJSON implements json.Unmarshaler.
// JSON null sets the value to absent.
func (n *Float32) UnmarshalJSON(data []byte) error {
	if string(data) == "null" {
		n.val = nil
		return nil
	}
	var v float32
	if err := json.Unmarshal(data, &v); err != nil {
		return err
	}
	n.val = &v
	return nil
}

// Value implements driver.Valuer for database writes.
// Returns float64 as required by the driver.Value interface.
func (n Float32) Value() (driver.Value, error) {
	if n.val == nil {
		return nil, nil
	}
	return float64(*n.val), nil
}

// Scan implements sql.Scanner for database reads.
// Delegates to sql.NullFloat64 and converts to float32.
// See type-level documentation for precision considerations.
func (n *Float32) Scan(value any) error {
	var s sql.NullFloat64
	if err := s.Scan(value); err != nil {
		return err
	}
	if !s.Valid {
		n.val = nil
		return nil
	}
	v := float32(s.Float64)
	n.val = &v
	return nil
}
