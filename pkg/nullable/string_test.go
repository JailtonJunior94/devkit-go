package nullable_test

import (
	"encoding/json"
	"testing"

	"github.com/JailtonJunior94/devkit-go/pkg/nullable"
	"github.com/stretchr/testify/require"
)

func TestString_ZeroValueIsNull(t *testing.T) {
	var s nullable.String
	require.True(t, s.IsNull())
}

func TestString_ZeroValueIsNotNull(t *testing.T) {
	s := nullable.StringOf("")
	require.False(t, s.IsNull())
}

func TestString_Of(t *testing.T) {
	s := nullable.StringOf("hello")
	require.False(t, s.IsNull())
	v, ok := s.Get()
	require.True(t, ok)
	require.Equal(t, "hello", v)
}

func TestString_Empty(t *testing.T) {
	s := nullable.StringEmpty()
	require.True(t, s.IsNull())
	require.Nil(t, s.Ptr())
}

func TestString_FromPtr_nil(t *testing.T) {
	s := nullable.StringFromPtr(nil)
	require.True(t, s.IsNull())
}

func TestString_FromPtr_value(t *testing.T) {
	v := "world"
	s := nullable.StringFromPtr(&v)
	require.False(t, s.IsNull())
	got, ok := s.Get()
	require.True(t, ok)
	require.Equal(t, "world", got)
}

func TestString_ValueOr_present(t *testing.T) {
	s := nullable.StringOf("foo")
	require.Equal(t, "foo", s.ValueOr("bar"))
}

func TestString_ValueOr_absent(t *testing.T) {
	s := nullable.StringEmpty()
	require.Equal(t, "bar", s.ValueOr("bar"))
}

func TestString_Ptr_nil(t *testing.T) {
	require.Nil(t, nullable.StringEmpty().Ptr())
}

func TestString_Ptr_value(t *testing.T) {
	s := nullable.StringOf("x")
	require.NotNil(t, s.Ptr())
	require.Equal(t, "x", *s.Ptr())
}

func TestString_Equal_twoNull(t *testing.T) {
	require.True(t, nullable.StringEmpty().Equal(nullable.StringEmpty()))
}

func TestString_Equal_sameValue(t *testing.T) {
	a := nullable.StringOf("abc")
	b := nullable.StringOf("abc")
	require.True(t, a.Equal(b))
}

func TestString_Equal_differentValues(t *testing.T) {
	a := nullable.StringOf("abc")
	b := nullable.StringOf("xyz")
	require.False(t, a.Equal(b))
}

func TestString_Equal_nullVsPresent(t *testing.T) {
	require.False(t, nullable.StringEmpty().Equal(nullable.StringOf("x")))
	require.False(t, nullable.StringOf("x").Equal(nullable.StringEmpty()))
}

func TestString_Equal_reflexive(t *testing.T) {
	a := nullable.StringOf("hello")
	require.True(t, a.Equal(a))
}

func TestString_Equal_symmetric(t *testing.T) {
	a := nullable.StringOf("hello")
	b := nullable.StringOf("hello")
	require.Equal(t, a.Equal(b), b.Equal(a))
}

func TestString_String_null(t *testing.T) {
	require.Equal(t, "<null>", nullable.StringEmpty().String())
}

func TestString_String_present(t *testing.T) {
	require.Equal(t, "hello", nullable.StringOf("hello").String())
}

func TestString_MarshalJSON_null(t *testing.T) {
	data, err := json.Marshal(nullable.StringEmpty())
	require.NoError(t, err)
	require.Equal(t, "null", string(data))
}

func TestString_MarshalJSON_present(t *testing.T) {
	data, err := json.Marshal(nullable.StringOf("hello"))
	require.NoError(t, err)
	require.Equal(t, `"hello"`, string(data))
}

func TestString_UnmarshalJSON_null(t *testing.T) {
	var s nullable.String
	require.NoError(t, json.Unmarshal([]byte("null"), &s))
	require.True(t, s.IsNull())
}

func TestString_UnmarshalJSON_value(t *testing.T) {
	var s nullable.String
	require.NoError(t, json.Unmarshal([]byte(`"world"`), &s))
	require.False(t, s.IsNull())
	require.Equal(t, "world", s.ValueOr(""))
}

func TestString_UnmarshalJSON_wrongType(t *testing.T) {
	var s nullable.String
	require.Error(t, json.Unmarshal([]byte(`42`), &s))
}

func TestString_JSONRoundTrip(t *testing.T) {
	original := nullable.StringOf("round-trip")
	data, err := json.Marshal(original)
	require.NoError(t, err)

	var restored nullable.String
	require.NoError(t, json.Unmarshal(data, &restored))
	require.Equal(t, original.ValueOr(""), restored.ValueOr(""))
}

func TestString_JSONRoundTrip_empty(t *testing.T) {
	original := nullable.StringOf("")
	data, err := json.Marshal(original)
	require.NoError(t, err)

	var restored nullable.String
	require.NoError(t, json.Unmarshal(data, &restored))
	require.False(t, restored.IsNull())
	require.Equal(t, "", restored.ValueOr("x"))
}

func TestString_Scan_nil(t *testing.T) {
	var s nullable.String
	require.NoError(t, s.Scan(nil))
	require.True(t, s.IsNull())
}

func TestString_Scan_value(t *testing.T) {
	var s nullable.String
	require.NoError(t, s.Scan("scanned"))
	require.False(t, s.IsNull())
	require.Equal(t, "scanned", s.ValueOr(""))
}

func TestString_Value_null(t *testing.T) {
	v, err := nullable.StringEmpty().Value()
	require.NoError(t, err)
	require.Nil(t, v)
}

func TestString_Value_present(t *testing.T) {
	v, err := nullable.StringOf("stored").Value()
	require.NoError(t, err)
	require.Equal(t, "stored", v)
}

func TestString_FromPtr_immutable(t *testing.T) {
	v := "original"
	n := nullable.StringFromPtr(&v)
	// mutating source must not affect n
	v = "mutated"
	require.Equal(t, "original", n.ValueOr(""), "StringFromPtr must copy the value")
}
