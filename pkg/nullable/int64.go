package nullable

import (
	"database/sql"
	"database/sql/driver"
	"encoding/json"
	"strconv"
)

// Int64 is a nullable int64 value object.
// The zero value is safe and represents absence (null).
type Int64 struct {
	val *int64
}

// Int64Of returns an Int64 with the given value present.
func Int64Of(v int64) Int64 { return Int64{val: &v} }

// Int64Empty returns an Int64 with no value (null).
func Int64Empty() Int64 { return Int64{} }

// Int64FromPtr creates an Int64 from a pointer. Nil pointer results in null.
// The pointed-to value is copied; mutations to the original pointer do not
// affect the returned Int64.
func Int64FromPtr(v *int64) Int64 {
	if v == nil {
		return Int64{}
	}
	copied := *v
	return Int64{val: &copied}
}

// IsNull returns true when no value is present.
func (n Int64) IsNull() bool { return n.val == nil }

// Get returns the value and whether it is present.
func (n Int64) Get() (int64, bool) {
	if n.val == nil {
		return 0, false
	}
	return *n.val, true
}

// ValueOr returns the value if present, or fallback otherwise.
func (n Int64) ValueOr(fallback int64) int64 {
	if n.val == nil {
		return fallback
	}
	return *n.val
}

// Ptr returns a pointer to the value, or nil if absent.
func (n Int64) Ptr() *int64 { return n.val }

// Equal returns true when both are null or both hold the same value.
func (n Int64) Equal(other Int64) bool {
	if n.val == nil && other.val == nil {
		return true
	}
	if n.val == nil || other.val == nil {
		return false
	}
	return *n.val == *other.val
}

// String implements fmt.Stringer. Returns "<null>" when absent.
func (n Int64) String() string {
	if n.val == nil {
		return "<null>"
	}
	return strconv.FormatInt(*n.val, 10)
}

// MarshalJSON implements json.Marshaler.
// Absent values are encoded as JSON null.
func (n Int64) MarshalJSON() ([]byte, error) {
	if n.val == nil {
		return []byte("null"), nil
	}
	return json.Marshal(*n.val)
}

// UnmarshalJSON implements json.Unmarshaler.
// JSON null sets the value to absent.
func (n *Int64) UnmarshalJSON(data []byte) error {
	if string(data) == "null" {
		n.val = nil
		return nil
	}
	var v int64
	if err := json.Unmarshal(data, &v); err != nil {
		return err
	}
	n.val = &v
	return nil
}

// Value implements driver.Valuer for database writes.
func (n Int64) Value() (driver.Value, error) {
	if n.val == nil {
		return nil, nil
	}
	return *n.val, nil
}

// Scan implements sql.Scanner for database reads.
// Delegates to sql.NullInt64.
func (n *Int64) Scan(value any) error {
	var s sql.NullInt64
	if err := s.Scan(value); err != nil {
		return err
	}
	if !s.Valid {
		n.val = nil
		return nil
	}
	n.val = &s.Int64
	return nil
}
