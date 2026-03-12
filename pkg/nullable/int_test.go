package nullable_test

import (
	"encoding/json"
	"testing"

	"github.com/JailtonJunior94/devkit-go/pkg/nullable"
	"github.com/stretchr/testify/require"
)

func TestInt_ZeroValueIsNull(t *testing.T) {
	var n nullable.Int
	require.True(t, n.IsNull())
}

func TestInt_ZeroValueIsNotNull(t *testing.T) {
	n := nullable.IntOf(0)
	require.False(t, n.IsNull())
}

func TestInt_Of(t *testing.T) {
	n := nullable.IntOf(42)
	require.False(t, n.IsNull())
	v, ok := n.Get()
	require.True(t, ok)
	require.Equal(t, 42, v)
}

func TestInt_Empty(t *testing.T) {
	n := nullable.IntEmpty()
	require.True(t, n.IsNull())
	require.Nil(t, n.Ptr())
}

func TestInt_FromPtr_nil(t *testing.T) {
	n := nullable.IntFromPtr(nil)
	require.True(t, n.IsNull())
}

func TestInt_FromPtr_value(t *testing.T) {
	v := 7
	n := nullable.IntFromPtr(&v)
	require.False(t, n.IsNull())
	got, ok := n.Get()
	require.True(t, ok)
	require.Equal(t, 7, got)
}

func TestInt_ValueOr_present(t *testing.T) {
	require.Equal(t, 10, nullable.IntOf(10).ValueOr(99))
}

func TestInt_ValueOr_absent(t *testing.T) {
	require.Equal(t, 99, nullable.IntEmpty().ValueOr(99))
}

func TestInt_Equal_twoNull(t *testing.T) {
	require.True(t, nullable.IntEmpty().Equal(nullable.IntEmpty()))
}

func TestInt_Equal_sameValue(t *testing.T) {
	require.True(t, nullable.IntOf(5).Equal(nullable.IntOf(5)))
}

func TestInt_Equal_differentValues(t *testing.T) {
	require.False(t, nullable.IntOf(5).Equal(nullable.IntOf(6)))
}

func TestInt_Equal_nullVsPresent(t *testing.T) {
	require.False(t, nullable.IntEmpty().Equal(nullable.IntOf(1)))
	require.False(t, nullable.IntOf(1).Equal(nullable.IntEmpty()))
}

func TestInt_Equal_reflexive(t *testing.T) {
	a := nullable.IntOf(3)
	require.True(t, a.Equal(a))
}

func TestInt_Equal_symmetric(t *testing.T) {
	a := nullable.IntOf(3)
	b := nullable.IntOf(3)
	require.Equal(t, a.Equal(b), b.Equal(a))
}

func TestInt_String_null(t *testing.T) {
	require.Equal(t, "<null>", nullable.IntEmpty().String())
}

func TestInt_String_present(t *testing.T) {
	require.Equal(t, "42", nullable.IntOf(42).String())
}

func TestInt_MarshalJSON_null(t *testing.T) {
	data, err := json.Marshal(nullable.IntEmpty())
	require.NoError(t, err)
	require.Equal(t, "null", string(data))
}

func TestInt_MarshalJSON_present(t *testing.T) {
	data, err := json.Marshal(nullable.IntOf(7))
	require.NoError(t, err)
	require.Equal(t, "7", string(data))
}

func TestInt_UnmarshalJSON_null(t *testing.T) {
	var n nullable.Int
	require.NoError(t, json.Unmarshal([]byte("null"), &n))
	require.True(t, n.IsNull())
}

func TestInt_UnmarshalJSON_value(t *testing.T) {
	var n nullable.Int
	require.NoError(t, json.Unmarshal([]byte("42"), &n))
	require.False(t, n.IsNull())
	require.Equal(t, 42, n.ValueOr(0))
}

func TestInt_UnmarshalJSON_wrongType(t *testing.T) {
	var n nullable.Int
	require.Error(t, json.Unmarshal([]byte(`"string"`), &n))
}

func TestInt_JSONRoundTrip(t *testing.T) {
	original := nullable.IntOf(42)
	data, err := json.Marshal(original)
	require.NoError(t, err)

	var restored nullable.Int
	require.NoError(t, json.Unmarshal(data, &restored))
	require.Equal(t, original.ValueOr(0), restored.ValueOr(0))
}

func TestInt_JSONRoundTrip_zero(t *testing.T) {
	original := nullable.IntOf(0)
	data, err := json.Marshal(original)
	require.NoError(t, err)

	var restored nullable.Int
	require.NoError(t, json.Unmarshal(data, &restored))
	require.False(t, restored.IsNull())
	require.Equal(t, 0, restored.ValueOr(99))
}

func TestInt_Scan_nil(t *testing.T) {
	var n nullable.Int
	require.NoError(t, n.Scan(nil))
	require.True(t, n.IsNull())
}

func TestInt_Scan_value(t *testing.T) {
	var n nullable.Int
	require.NoError(t, n.Scan(int64(100)))
	require.False(t, n.IsNull())
	require.Equal(t, 100, n.ValueOr(0))
}

func TestInt_Scan_largeValue(t *testing.T) {
	// Values beyond int32 range must scan correctly on 64-bit platforms.
	large := int64(3_000_000_000)
	var n nullable.Int
	require.NoError(t, n.Scan(large))
	require.False(t, n.IsNull())
	require.Equal(t, int(large), n.ValueOr(0))
}

func TestInt_FromPtr_immutable(t *testing.T) {
	v := 42
	n := nullable.IntFromPtr(&v)
	// mutating source must not affect n
	v = 99
	require.Equal(t, 42, n.ValueOr(0), "IntFromPtr must copy the value")
}

func TestInt_Value_null(t *testing.T) {
	v, err := nullable.IntEmpty().Value()
	require.NoError(t, err)
	require.Nil(t, v)
}

func TestInt_Value_present(t *testing.T) {
	v, err := nullable.IntOf(55).Value()
	require.NoError(t, err)
	require.Equal(t, int64(55), v)
}
