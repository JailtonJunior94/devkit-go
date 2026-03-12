package nullable

import (
	"database/sql"
	"database/sql/driver"
	"encoding/json"
	"fmt"
	"time"
)

// Time is a nullable time.Time value object.
// The zero value is safe and represents absence (null).
//
// JSON layout: TimeOf uses RFC3339 by default. Use TimeOfWithLayout to specify
// a custom layout. When unmarshaling into a Time that was created with a custom
// layout, the same layout must be set on the receiver before calling
// json.Unmarshal, for example:
//
//	receiver := nullable.TimeOfWithLayout(time.Time{}, myLayout)
//	_ = json.Unmarshal(data, &receiver)
type Time struct {
	val    *time.Time
	layout string // empty → time.RFC3339
}

// TimeOf returns a Time with the given value present, using RFC3339 as the
// default JSON layout.
func TimeOf(t time.Time) Time { return Time{val: &t} }

// TimeOfWithLayout returns a Time with the given value present and a custom
// JSON serialization layout.
func TimeOfWithLayout(t time.Time, layout string) Time {
	return Time{val: &t, layout: layout}
}

// TimeEmpty returns a Time with no value (null).
func TimeEmpty() Time { return Time{} }

// TimeFromPtr creates a Time from a pointer. Nil pointer results in null.
// The resulting Time uses RFC3339 as the default JSON layout.
// The pointed-to value is copied; mutations to the original pointer do not
// affect the returned Time.
func TimeFromPtr(t *time.Time) Time {
	if t == nil {
		return Time{}
	}
	copied := *t
	return Time{val: &copied}
}

// IsNull returns true when no value is present.
func (n Time) IsNull() bool { return n.val == nil }

// Get returns the value and whether it is present.
func (n Time) Get() (time.Time, bool) {
	if n.val == nil {
		return time.Time{}, false
	}
	return *n.val, true
}

// ValueOr returns the value if present, or fallback otherwise.
func (n Time) ValueOr(fallback time.Time) time.Time {
	if n.val == nil {
		return fallback
	}
	return *n.val
}

// Ptr returns a pointer to the value, or nil if absent.
func (n Time) Ptr() *time.Time { return n.val }

// Equal returns true when both are null or both represent the same instant.
// Uses time.Time.Equal which is timezone-independent.
func (n Time) Equal(other Time) bool {
	if n.val == nil && other.val == nil {
		return true
	}
	if n.val == nil || other.val == nil {
		return false
	}
	return n.val.Equal(*other.val)
}

// String implements fmt.Stringer. Returns "<null>" when absent, or the value
// formatted as RFC3339.
func (n Time) String() string {
	if n.val == nil {
		return "<null>"
	}
	return n.val.Format(time.RFC3339)
}

// MarshalJSON implements json.Marshaler.
// Absent values are encoded as JSON null.
// Present values are encoded as a JSON string using the configured layout
// (RFC3339 when no layout was specified).
func (n Time) MarshalJSON() ([]byte, error) {
	if n.val == nil {
		return []byte("null"), nil
	}
	layout := n.layout
	if layout == "" {
		layout = time.RFC3339
	}
	return json.Marshal(n.val.Format(layout))
}

// UnmarshalJSON implements json.Unmarshaler.
// JSON null sets the value to absent.
// Present values are parsed using the configured layout (RFC3339 by default).
func (n *Time) UnmarshalJSON(data []byte) error {
	if string(data) == "null" {
		n.val = nil
		return nil
	}
	layout := n.layout
	if layout == "" {
		layout = time.RFC3339
	}
	var s string
	if err := json.Unmarshal(data, &s); err != nil {
		return err
	}
	t, err := time.Parse(layout, s)
	if err != nil {
		return fmt.Errorf("nullable.Time: parse %q with layout %q: %w", s, layout, err)
	}
	n.val = &t
	return nil
}

// Value implements driver.Valuer for database writes.
func (n Time) Value() (driver.Value, error) {
	if n.val == nil {
		return nil, nil
	}
	return *n.val, nil
}

// Scan implements sql.Scanner for database reads.
// Delegates to sql.NullTime.
func (n *Time) Scan(value any) error {
	var s sql.NullTime
	if err := s.Scan(value); err != nil {
		return err
	}
	if !s.Valid {
		n.val = nil
		return nil
	}
	n.val = &s.Time
	return nil
}
