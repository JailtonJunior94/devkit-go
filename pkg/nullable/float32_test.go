package nullable_test

import (
	"encoding/json"
	"testing"

	"github.com/JailtonJunior94/devkit-go/pkg/nullable"
	"github.com/stretchr/testify/require"
)

func TestFloat32_ZeroValueIsNull(t *testing.T) {
	var n nullable.Float32
	require.True(t, n.IsNull())
}

func TestFloat32_ZeroValueIsNotNull(t *testing.T) {
	n := nullable.Float32Of(0.0)
	require.False(t, n.IsNull())
}

func TestFloat32_Of(t *testing.T) {
	n := nullable.Float32Of(1.5)
	require.False(t, n.IsNull())
	v, ok := n.Get()
	require.True(t, ok)
	require.Equal(t, float32(1.5), v)
}

func TestFloat32_Empty(t *testing.T) {
	n := nullable.Float32Empty()
	require.True(t, n.IsNull())
	require.Nil(t, n.Ptr())
}

func TestFloat32_FromPtr_nil(t *testing.T) {
	require.True(t, nullable.Float32FromPtr(nil).IsNull())
}

func TestFloat32_FromPtr_value(t *testing.T) {
	v := float32(2.5)
	n := nullable.Float32FromPtr(&v)
	require.False(t, n.IsNull())
	got, ok := n.Get()
	require.True(t, ok)
	require.Equal(t, float32(2.5), got)
}

func TestFloat32_ValueOr_present(t *testing.T) {
	require.Equal(t, float32(1.5), nullable.Float32Of(1.5).ValueOr(9.9))
}

func TestFloat32_ValueOr_absent(t *testing.T) {
	require.Equal(t, float32(9.9), nullable.Float32Empty().ValueOr(9.9))
}

func TestFloat32_Equal_twoNull(t *testing.T) {
	require.True(t, nullable.Float32Empty().Equal(nullable.Float32Empty()))
}

func TestFloat32_Equal_sameValue(t *testing.T) {
	require.True(t, nullable.Float32Of(1.1).Equal(nullable.Float32Of(1.1)))
}

func TestFloat32_Equal_differentValues(t *testing.T) {
	require.False(t, nullable.Float32Of(1.1).Equal(nullable.Float32Of(2.2)))
}

func TestFloat32_Equal_nullVsPresent(t *testing.T) {
	require.False(t, nullable.Float32Empty().Equal(nullable.Float32Of(1.0)))
	require.False(t, nullable.Float32Of(1.0).Equal(nullable.Float32Empty()))
}

func TestFloat32_Equal_reflexive(t *testing.T) {
	a := nullable.Float32Of(3.14)
	require.True(t, a.Equal(a))
}

func TestFloat32_Equal_symmetric(t *testing.T) {
	a := nullable.Float32Of(3.14)
	b := nullable.Float32Of(3.14)
	require.Equal(t, a.Equal(b), b.Equal(a))
}

func TestFloat32_String_null(t *testing.T) {
	require.Equal(t, "<null>", nullable.Float32Empty().String())
}

func TestFloat32_String_present(t *testing.T) {
	require.Equal(t, "1.5", nullable.Float32Of(1.5).String())
}

func TestFloat32_MarshalJSON_null(t *testing.T) {
	data, err := json.Marshal(nullable.Float32Empty())
	require.NoError(t, err)
	require.Equal(t, "null", string(data))
}

func TestFloat32_MarshalJSON_present(t *testing.T) {
	data, err := json.Marshal(nullable.Float32Of(1.5))
	require.NoError(t, err)
	require.Equal(t, "1.5", string(data))
}

func TestFloat32_UnmarshalJSON_null(t *testing.T) {
	var n nullable.Float32
	require.NoError(t, json.Unmarshal([]byte("null"), &n))
	require.True(t, n.IsNull())
}

func TestFloat32_UnmarshalJSON_value(t *testing.T) {
	var n nullable.Float32
	require.NoError(t, json.Unmarshal([]byte("1.5"), &n))
	require.False(t, n.IsNull())
	require.Equal(t, float32(1.5), n.ValueOr(0))
}

func TestFloat32_UnmarshalJSON_wrongType(t *testing.T) {
	var n nullable.Float32
	require.Error(t, json.Unmarshal([]byte(`"string"`), &n))
}

func TestFloat32_JSONRoundTrip(t *testing.T) {
	original := nullable.Float32Of(3.14)
	data, err := json.Marshal(original)
	require.NoError(t, err)

	var restored nullable.Float32
	require.NoError(t, json.Unmarshal(data, &restored))
	require.InDelta(t, original.ValueOr(0), restored.ValueOr(0), 1e-5)
}

func TestFloat32_Scan_nil(t *testing.T) {
	var n nullable.Float32
	require.NoError(t, n.Scan(nil))
	require.True(t, n.IsNull())
}

func TestFloat32_Scan_value(t *testing.T) {
	// sql drivers return float64; Scan converts to float32
	var n nullable.Float32
	require.NoError(t, n.Scan(float64(1.5)))
	require.False(t, n.IsNull())
	require.Equal(t, float32(1.5), n.ValueOr(0))
}

func TestFloat32_Scan_precisionNote(t *testing.T) {
	// float64 → float32 cast: values representable in float32 survive without loss
	var n nullable.Float32
	require.NoError(t, n.Scan(float64(0.5)))
	require.Equal(t, float32(0.5), n.ValueOr(0))
}

func TestFloat32_Value_null(t *testing.T) {
	v, err := nullable.Float32Empty().Value()
	require.NoError(t, err)
	require.Nil(t, v)
}

func TestFloat32_Value_returnsFloat64(t *testing.T) {
	// driver.Value requires float64 for float types
	v, err := nullable.Float32Of(1.5).Value()
	require.NoError(t, err)
	_, ok := v.(float64)
	require.True(t, ok, "Value() must return float64 for driver compatibility")
}

func TestFloat32_Value_present(t *testing.T) {
	v, err := nullable.Float32Of(1.5).Value()
	require.NoError(t, err)
	require.InDelta(t, float64(float32(1.5)), v.(float64), 1e-6)
}
