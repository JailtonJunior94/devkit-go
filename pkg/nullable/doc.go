// Package nullable provides immutable, zero-value-safe nullable value objects
// for common Go types (int, int64, float32, float64, string, time.Time).
//
// Each type follows a consistent API surface:
//   - Constructor: TypeOf(v) for present, TypeEmpty() for null, TypeFromPtr(*T)
//   - Access: Get() (T, bool), ValueOr(fallback T), Ptr() *T, IsNull() bool
//   - Equality: Equal(other) bool (null == null, value-based otherwise)
//   - Serialization: json.Marshaler/Unmarshaler (null ↔ JSON null)
//   - Database: driver.Valuer/sql.Scanner (null ↔ SQL NULL)
//
// The zero value of every nullable type is safe to use and represents absence.
// All types are immutable by design: constructors copy pointer values to prevent
// aliasing mutations.
package nullable
