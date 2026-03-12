package nullable_test

import (
	"encoding/json"
	"testing"

	"github.com/JailtonJunior94/devkit-go/pkg/nullable"
	"github.com/stretchr/testify/require"
)

func TestInt64_ZeroValueIsNull(t *testing.T) {
	var n nullable.Int64
	require.True(t, n.IsNull())
}

func TestInt64_ZeroValueIsNotNull(t *testing.T) {
	n := nullable.Int64Of(0)
	require.False(t, n.IsNull())
}

func TestInt64_Of(t *testing.T) {
	n := nullable.Int64Of(100)
	require.False(t, n.IsNull())
	v, ok := n.Get()
	require.True(t, ok)
	require.Equal(t, int64(100), v)
}

func TestInt64_Empty(t *testing.T) {
	n := nullable.Int64Empty()
	require.True(t, n.IsNull())
	require.Nil(t, n.Ptr())
}

func TestInt64_FromPtr_nil(t *testing.T) {
	require.True(t, nullable.Int64FromPtr(nil).IsNull())
}

func TestInt64_FromPtr_value(t *testing.T) {
	v := int64(99)
	n := nullable.Int64FromPtr(&v)
	require.False(t, n.IsNull())
	got, ok := n.Get()
	require.True(t, ok)
	require.Equal(t, int64(99), got)
}

func TestInt64_ValueOr_present(t *testing.T) {
	require.Equal(t, int64(10), nullable.Int64Of(10).ValueOr(99))
}

func TestInt64_ValueOr_absent(t *testing.T) {
	require.Equal(t, int64(99), nullable.Int64Empty().ValueOr(99))
}

func TestInt64_Equal_twoNull(t *testing.T) {
	require.True(t, nullable.Int64Empty().Equal(nullable.Int64Empty()))
}

func TestInt64_Equal_sameValue(t *testing.T) {
	require.True(t, nullable.Int64Of(5).Equal(nullable.Int64Of(5)))
}

func TestInt64_Equal_differentValues(t *testing.T) {
	require.False(t, nullable.Int64Of(5).Equal(nullable.Int64Of(6)))
}

func TestInt64_Equal_nullVsPresent(t *testing.T) {
	require.False(t, nullable.Int64Empty().Equal(nullable.Int64Of(1)))
	require.False(t, nullable.Int64Of(1).Equal(nullable.Int64Empty()))
}

func TestInt64_Equal_reflexive(t *testing.T) {
	a := nullable.Int64Of(3)
	require.True(t, a.Equal(a))
}

func TestInt64_Equal_symmetric(t *testing.T) {
	a := nullable.Int64Of(3)
	b := nullable.Int64Of(3)
	require.Equal(t, a.Equal(b), b.Equal(a))
}

func TestInt64_String_null(t *testing.T) {
	require.Equal(t, "<null>", nullable.Int64Empty().String())
}

func TestInt64_String_present(t *testing.T) {
	require.Equal(t, "100", nullable.Int64Of(100).String())
}

func TestInt64_MarshalJSON_null(t *testing.T) {
	data, err := json.Marshal(nullable.Int64Empty())
	require.NoError(t, err)
	require.Equal(t, "null", string(data))
}

func TestInt64_MarshalJSON_present(t *testing.T) {
	data, err := json.Marshal(nullable.Int64Of(7))
	require.NoError(t, err)
	require.Equal(t, "7", string(data))
}

func TestInt64_UnmarshalJSON_null(t *testing.T) {
	var n nullable.Int64
	require.NoError(t, json.Unmarshal([]byte("null"), &n))
	require.True(t, n.IsNull())
}

func TestInt64_UnmarshalJSON_value(t *testing.T) {
	var n nullable.Int64
	require.NoError(t, json.Unmarshal([]byte("42"), &n))
	require.False(t, n.IsNull())
	require.Equal(t, int64(42), n.ValueOr(0))
}

func TestInt64_UnmarshalJSON_wrongType(t *testing.T) {
	var n nullable.Int64
	require.Error(t, json.Unmarshal([]byte(`"string"`), &n))
}

func TestInt64_JSONRoundTrip(t *testing.T) {
	original := nullable.Int64Of(9876543210)
	data, err := json.Marshal(original)
	require.NoError(t, err)

	var restored nullable.Int64
	require.NoError(t, json.Unmarshal(data, &restored))
	require.Equal(t, original.ValueOr(0), restored.ValueOr(0))
}

func TestInt64_Scan_nil(t *testing.T) {
	var n nullable.Int64
	require.NoError(t, n.Scan(nil))
	require.True(t, n.IsNull())
}

func TestInt64_Scan_value(t *testing.T) {
	var n nullable.Int64
	require.NoError(t, n.Scan(int64(200)))
	require.False(t, n.IsNull())
	require.Equal(t, int64(200), n.ValueOr(0))
}

func TestInt64_Value_null(t *testing.T) {
	v, err := nullable.Int64Empty().Value()
	require.NoError(t, err)
	require.Nil(t, v)
}

func TestInt64_Value_present(t *testing.T) {
	v, err := nullable.Int64Of(55).Value()
	require.NoError(t, err)
	require.Equal(t, int64(55), v)
}
