package nullable

import (
	"database/sql"
	"database/sql/driver"
	"encoding/json"
)

// String is a nullable string value object.
// The zero value is safe and represents absence (null).
type String struct {
	val *string
}

// StringOf returns a String with the given value present.
func StringOf(v string) String { return String{val: &v} }

// StringEmpty returns a String with no value (null).
func StringEmpty() String { return String{} }

// StringFromPtr creates a String from a pointer. Nil pointer results in null.
// The pointed-to value is copied; mutations to the original pointer do not
// affect the returned String.
func StringFromPtr(v *string) String {
	if v == nil {
		return String{}
	}
	copied := *v
	return String{val: &copied}
}

// IsNull returns true when no value is present.
func (n String) IsNull() bool { return n.val == nil }

// Get returns the value and whether it is present.
func (n String) Get() (string, bool) {
	if n.val == nil {
		return "", false
	}
	return *n.val, true
}

// ValueOr returns the value if present, or fallback otherwise.
func (n String) ValueOr(fallback string) string {
	if n.val == nil {
		return fallback
	}
	return *n.val
}

// Ptr returns a pointer to the value, or nil if absent.
func (n String) Ptr() *string { return n.val }

// Equal returns true when both are null or both hold the same value.
func (n String) Equal(other String) bool {
	if n.val == nil && other.val == nil {
		return true
	}
	if n.val == nil || other.val == nil {
		return false
	}
	return *n.val == *other.val
}

// String implements fmt.Stringer. Returns "<null>" when absent.
func (n String) String() string {
	if n.val == nil {
		return "<null>"
	}
	return *n.val
}

// MarshalJSON implements json.Marshaler.
// Absent values are encoded as JSON null.
func (n String) MarshalJSON() ([]byte, error) {
	if n.val == nil {
		return []byte("null"), nil
	}
	return json.Marshal(*n.val)
}

// UnmarshalJSON implements json.Unmarshaler.
// JSON null sets the value to absent.
func (n *String) UnmarshalJSON(data []byte) error {
	if string(data) == "null" {
		n.val = nil
		return nil
	}
	var s string
	if err := json.Unmarshal(data, &s); err != nil {
		return err
	}
	n.val = &s
	return nil
}

// Value implements driver.Valuer for database writes.
func (n String) Value() (driver.Value, error) {
	if n.val == nil {
		return nil, nil
	}
	return *n.val, nil
}

// Scan implements sql.Scanner for database reads.
func (n *String) Scan(value any) error {
	var ns sql.NullString
	if err := ns.Scan(value); err != nil {
		return err
	}
	if !ns.Valid {
		n.val = nil
		return nil
	}
	n.val = &ns.String
	return nil
}
