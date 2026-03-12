package nullable

import (
	"database/sql"
	"database/sql/driver"
	"encoding/json"
	"strconv"
)

// Int is a nullable int value object.
// The zero value is safe and represents absence (null).
type Int struct {
	val *int
}

// IntOf returns an Int with the given value present.
func IntOf(v int) Int { return Int{val: &v} }

// IntEmpty returns an Int with no value (null).
func IntEmpty() Int { return Int{} }

// IntFromPtr creates an Int from a pointer. Nil pointer results in null.
// The pointed-to value is copied; mutations to the original pointer do not
// affect the returned Int.
func IntFromPtr(v *int) Int {
	if v == nil {
		return Int{}
	}
	copied := *v
	return Int{val: &copied}
}

// IsNull returns true when no value is present.
func (n Int) IsNull() bool { return n.val == nil }

// Get returns the value and whether it is present.
func (n Int) Get() (int, bool) {
	if n.val == nil {
		return 0, false
	}
	return *n.val, true
}

// ValueOr returns the value if present, or fallback otherwise.
func (n Int) ValueOr(fallback int) int {
	if n.val == nil {
		return fallback
	}
	return *n.val
}

// Ptr returns a pointer to the value, or nil if absent.
func (n Int) Ptr() *int { return n.val }

// Equal returns true when both are null or both hold the same value.
func (n Int) Equal(other Int) bool {
	if n.val == nil && other.val == nil {
		return true
	}
	if n.val == nil || other.val == nil {
		return false
	}
	return *n.val == *other.val
}

// String implements fmt.Stringer. Returns "<null>" when absent.
func (n Int) String() string {
	if n.val == nil {
		return "<null>"
	}
	return strconv.Itoa(*n.val)
}

// MarshalJSON implements json.Marshaler.
// Absent values are encoded as JSON null.
func (n Int) MarshalJSON() ([]byte, error) {
	if n.val == nil {
		return []byte("null"), nil
	}
	return json.Marshal(*n.val)
}

// UnmarshalJSON implements json.Unmarshaler.
// JSON null sets the value to absent.
func (n *Int) UnmarshalJSON(data []byte) error {
	if string(data) == "null" {
		n.val = nil
		return nil
	}
	var v int
	if err := json.Unmarshal(data, &v); err != nil {
		return err
	}
	n.val = &v
	return nil
}

// Value implements driver.Valuer for database writes.
// Returns int64 as required by the driver.Value interface.
func (n Int) Value() (driver.Value, error) {
	if n.val == nil {
		return nil, nil
	}
	return int64(*n.val), nil
}

// Scan implements sql.Scanner for database reads.
// Delegates to sql.NullInt64 and converts to int, preserving the full 64-bit
// range of int on 64-bit platforms.
func (n *Int) Scan(value any) error {
	var s sql.NullInt64
	if err := s.Scan(value); err != nil {
		return err
	}
	if !s.Valid {
		n.val = nil
		return nil
	}
	v := int(s.Int64)
	n.val = &v
	return nil
}
