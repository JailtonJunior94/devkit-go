package nullable_test

import (
	"encoding/json"
	"testing"

	"github.com/JailtonJunior94/devkit-go/pkg/nullable"
	"github.com/stretchr/testify/require"
)

func TestFloat64_ZeroValueIsNull(t *testing.T) {
	var n nullable.Float64
	require.True(t, n.IsNull())
}

func TestFloat64_ZeroValueIsNotNull(t *testing.T) {
	n := nullable.Float64Of(0.0)
	require.False(t, n.IsNull())
}

func TestFloat64_Of(t *testing.T) {
	n := nullable.Float64Of(3.14)
	require.False(t, n.IsNull())
	v, ok := n.Get()
	require.True(t, ok)
	require.Equal(t, 3.14, v)
}

func TestFloat64_Empty(t *testing.T) {
	n := nullable.Float64Empty()
	require.True(t, n.IsNull())
	require.Nil(t, n.Ptr())
}

func TestFloat64_FromPtr_nil(t *testing.T) {
	require.True(t, nullable.Float64FromPtr(nil).IsNull())
}

func TestFloat64_FromPtr_value(t *testing.T) {
	v := 2.71
	n := nullable.Float64FromPtr(&v)
	require.False(t, n.IsNull())
	got, ok := n.Get()
	require.True(t, ok)
	require.Equal(t, 2.71, got)
}

func TestFloat64_ValueOr_present(t *testing.T) {
	require.Equal(t, 1.5, nullable.Float64Of(1.5).ValueOr(9.9))
}

func TestFloat64_ValueOr_absent(t *testing.T) {
	require.Equal(t, 9.9, nullable.Float64Empty().ValueOr(9.9))
}

func TestFloat64_Equal_twoNull(t *testing.T) {
	require.True(t, nullable.Float64Empty().Equal(nullable.Float64Empty()))
}

func TestFloat64_Equal_sameValue(t *testing.T) {
	require.True(t, nullable.Float64Of(1.1).Equal(nullable.Float64Of(1.1)))
}

func TestFloat64_Equal_differentValues(t *testing.T) {
	require.False(t, nullable.Float64Of(1.1).Equal(nullable.Float64Of(2.2)))
}

func TestFloat64_Equal_nullVsPresent(t *testing.T) {
	require.False(t, nullable.Float64Empty().Equal(nullable.Float64Of(1.0)))
	require.False(t, nullable.Float64Of(1.0).Equal(nullable.Float64Empty()))
}

func TestFloat64_Equal_reflexive(t *testing.T) {
	a := nullable.Float64Of(3.14)
	require.True(t, a.Equal(a))
}

func TestFloat64_Equal_symmetric(t *testing.T) {
	a := nullable.Float64Of(3.14)
	b := nullable.Float64Of(3.14)
	require.Equal(t, a.Equal(b), b.Equal(a))
}

func TestFloat64_String_null(t *testing.T) {
	require.Equal(t, "<null>", nullable.Float64Empty().String())
}

func TestFloat64_String_present(t *testing.T) {
	require.Equal(t, "3.14", nullable.Float64Of(3.14).String())
}

func TestFloat64_MarshalJSON_null(t *testing.T) {
	data, err := json.Marshal(nullable.Float64Empty())
	require.NoError(t, err)
	require.Equal(t, "null", string(data))
}

func TestFloat64_MarshalJSON_present(t *testing.T) {
	data, err := json.Marshal(nullable.Float64Of(1.5))
	require.NoError(t, err)
	require.Equal(t, "1.5", string(data))
}

func TestFloat64_UnmarshalJSON_null(t *testing.T) {
	var n nullable.Float64
	require.NoError(t, json.Unmarshal([]byte("null"), &n))
	require.True(t, n.IsNull())
}

func TestFloat64_UnmarshalJSON_value(t *testing.T) {
	var n nullable.Float64
	require.NoError(t, json.Unmarshal([]byte("3.14"), &n))
	require.False(t, n.IsNull())
	require.InDelta(t, 3.14, n.ValueOr(0), 1e-9)
}

func TestFloat64_UnmarshalJSON_wrongType(t *testing.T) {
	var n nullable.Float64
	require.Error(t, json.Unmarshal([]byte(`"string"`), &n))
}

func TestFloat64_JSONRoundTrip(t *testing.T) {
	original := nullable.Float64Of(3.14159)
	data, err := json.Marshal(original)
	require.NoError(t, err)

	var restored nullable.Float64
	require.NoError(t, json.Unmarshal(data, &restored))
	require.InDelta(t, original.ValueOr(0), restored.ValueOr(0), 1e-9)
}

func TestFloat64_JSONRoundTrip_zero(t *testing.T) {
	original := nullable.Float64Of(0.0)
	data, err := json.Marshal(original)
	require.NoError(t, err)

	var restored nullable.Float64
	require.NoError(t, json.Unmarshal(data, &restored))
	require.False(t, restored.IsNull())
	require.Equal(t, 0.0, restored.ValueOr(9.9))
}

func TestFloat64_Scan_nil(t *testing.T) {
	var n nullable.Float64
	require.NoError(t, n.Scan(nil))
	require.True(t, n.IsNull())
}

func TestFloat64_Scan_value(t *testing.T) {
	var n nullable.Float64
	require.NoError(t, n.Scan(float64(2.5)))
	require.False(t, n.IsNull())
	require.Equal(t, 2.5, n.ValueOr(0))
}

func TestFloat64_Value_null(t *testing.T) {
	v, err := nullable.Float64Empty().Value()
	require.NoError(t, err)
	require.Nil(t, v)
}

func TestFloat64_Value_present(t *testing.T) {
	v, err := nullable.Float64Of(1.5).Value()
	require.NoError(t, err)
	require.Equal(t, 1.5, v)
}
