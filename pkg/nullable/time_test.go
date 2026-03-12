package nullable_test

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/JailtonJunior94/devkit-go/pkg/nullable"
	"github.com/stretchr/testify/require"
)

// newTestTime returns a fixed time for use within a single test.
// Returned as a value to avoid shared mutable package state.
func newTestTime() time.Time {
	return time.Date(2024, 6, 15, 12, 0, 0, 0, time.UTC)
}

func newTestTimeOther() time.Time {
	return time.Date(2024, 6, 16, 12, 0, 0, 0, time.UTC)
}

func TestTime_ZeroValueIsNull(t *testing.T) {
	var n nullable.Time
	require.True(t, n.IsNull())
}

func TestTime_ZeroTimeIsNotNull(t *testing.T) {
	// time.Time{} is the zero value of time.Time — it must not be null
	n := nullable.TimeOf(time.Time{})
	require.False(t, n.IsNull())
}

func TestTime_Of(t *testing.T) {
	ts := newTestTime()
	n := nullable.TimeOf(ts)
	require.False(t, n.IsNull())
	v, ok := n.Get()
	require.True(t, ok)
	require.True(t, ts.Equal(v))
}

func TestTime_Empty(t *testing.T) {
	n := nullable.TimeEmpty()
	require.True(t, n.IsNull())
	require.Nil(t, n.Ptr())
}

func TestTime_FromPtr_nil(t *testing.T) {
	require.True(t, nullable.TimeFromPtr(nil).IsNull())
}

func TestTime_FromPtr_value(t *testing.T) {
	ts := newTestTime()
	n := nullable.TimeFromPtr(&ts)
	require.False(t, n.IsNull())
	v, ok := n.Get()
	require.True(t, ok)
	require.True(t, ts.Equal(v))
}

func TestTime_FromPtr_immutable(t *testing.T) {
	ts := newTestTime()
	n := nullable.TimeFromPtr(&ts)
	// mutating source must not affect n
	ts = newTestTimeOther()
	v, _ := n.Get()
	require.True(t, newTestTime().Equal(v), "FromPtr must copy the value")
}

func TestTime_ValueOr_present(t *testing.T) {
	ts := newTestTime()
	other := newTestTimeOther()
	n := nullable.TimeOf(ts)
	require.True(t, ts.Equal(n.ValueOr(other)))
}

func TestTime_ValueOr_absent(t *testing.T) {
	other := newTestTimeOther()
	n := nullable.TimeEmpty()
	require.True(t, other.Equal(n.ValueOr(other)))
}

func TestTime_Equal_twoNull(t *testing.T) {
	require.True(t, nullable.TimeEmpty().Equal(nullable.TimeEmpty()))
}

func TestTime_Equal_sameInstant(t *testing.T) {
	ts := newTestTime()
	a := nullable.TimeOf(ts)
	b := nullable.TimeOf(ts)
	require.True(t, a.Equal(b))
}

func TestTime_Equal_sameInstantDifferentTimezone(t *testing.T) {
	loc := time.FixedZone("BRT", -3*60*60)
	t1 := newTestTime().In(time.UTC)
	t2 := newTestTime().In(loc)
	require.True(t, nullable.TimeOf(t1).Equal(nullable.TimeOf(t2)))
}

func TestTime_Equal_differentInstants(t *testing.T) {
	require.False(t, nullable.TimeOf(newTestTime()).Equal(nullable.TimeOf(newTestTimeOther())))
}

func TestTime_Equal_nullVsPresent(t *testing.T) {
	require.False(t, nullable.TimeEmpty().Equal(nullable.TimeOf(newTestTime())))
	require.False(t, nullable.TimeOf(newTestTime()).Equal(nullable.TimeEmpty()))
}

func TestTime_Equal_reflexive(t *testing.T) {
	a := nullable.TimeOf(newTestTime())
	require.True(t, a.Equal(a))
}

func TestTime_Equal_symmetric(t *testing.T) {
	a := nullable.TimeOf(newTestTime())
	b := nullable.TimeOf(newTestTime())
	require.Equal(t, a.Equal(b), b.Equal(a))
}

func TestTime_String_null(t *testing.T) {
	require.Equal(t, "<null>", nullable.TimeEmpty().String())
}

func TestTime_String_present(t *testing.T) {
	ts := newTestTime()
	n := nullable.TimeOf(ts)
	require.Equal(t, ts.Format(time.RFC3339), n.String())
}

func TestTime_MarshalJSON_null(t *testing.T) {
	data, err := json.Marshal(nullable.TimeEmpty())
	require.NoError(t, err)
	require.Equal(t, "null", string(data))
}

func TestTime_MarshalJSON_RFC3339Default(t *testing.T) {
	ts := newTestTime()
	data, err := json.Marshal(nullable.TimeOf(ts))
	require.NoError(t, err)
	expected, _ := json.Marshal(ts.Format(time.RFC3339))
	require.Equal(t, string(expected), string(data))
}

func TestTime_MarshalJSON_customLayout(t *testing.T) {
	ts := newTestTime()
	layout := "2006-01-02"
	n := nullable.TimeOfWithLayout(ts, layout)
	data, err := json.Marshal(n)
	require.NoError(t, err)
	require.Equal(t, `"`+ts.Format(layout)+`"`, string(data))
}

func TestTime_UnmarshalJSON_null(t *testing.T) {
	var n nullable.Time
	require.NoError(t, json.Unmarshal([]byte("null"), &n))
	require.True(t, n.IsNull())
}

func TestTime_UnmarshalJSON_RFC3339(t *testing.T) {
	ts := newTestTime()
	data, _ := json.Marshal(ts.Format(time.RFC3339))
	var n nullable.Time
	require.NoError(t, json.Unmarshal(data, &n))
	require.False(t, n.IsNull())
	v, _ := n.Get()
	require.True(t, ts.Equal(v))
}

func TestTime_UnmarshalJSON_customLayout(t *testing.T) {
	ts := newTestTime()
	layout := "2006-01-02"
	formatted := ts.Format(layout)
	data, _ := json.Marshal(formatted)

	// receiver must be pre-configured with the same layout
	receiver := nullable.TimeOfWithLayout(time.Time{}, layout)
	require.NoError(t, json.Unmarshal(data, &receiver))
	require.False(t, receiver.IsNull())

	v, _ := receiver.Get()
	require.Equal(t, ts.Format(layout), v.Format(layout))
}

func TestTime_UnmarshalJSON_layoutMismatch(t *testing.T) {
	// date-only string, but receiver expects RFC3339
	data, _ := json.Marshal("2024-06-15")
	var n nullable.Time
	err := json.Unmarshal(data, &n)
	require.Error(t, err)
	require.Contains(t, err.Error(), "nullable.Time")
}

func TestTime_UnmarshalJSON_nonString(t *testing.T) {
	var n nullable.Time
	require.Error(t, json.Unmarshal([]byte("42"), &n))
}

func TestTime_JSONRoundTrip_RFC3339(t *testing.T) {
	ts := newTestTime()
	original := nullable.TimeOf(ts)
	data, err := json.Marshal(original)
	require.NoError(t, err)

	var restored nullable.Time
	require.NoError(t, json.Unmarshal(data, &restored))
	v, _ := restored.Get()
	require.True(t, ts.Equal(v))
}

func TestTime_JSONRoundTrip_customLayout(t *testing.T) {
	ts := newTestTime()
	layout := "2006-01-02T15:04:05"
	original := nullable.TimeOfWithLayout(ts, layout)
	data, err := json.Marshal(original)
	require.NoError(t, err)

	receiver := nullable.TimeOfWithLayout(time.Time{}, layout)
	require.NoError(t, json.Unmarshal(data, &receiver))
	v, _ := receiver.Get()
	require.Equal(t, ts.Format(layout), v.Format(layout))
}

func TestTime_Scan_nil(t *testing.T) {
	var n nullable.Time
	require.NoError(t, n.Scan(nil))
	require.True(t, n.IsNull())
}

func TestTime_Scan_value(t *testing.T) {
	ts := newTestTime()
	var n nullable.Time
	require.NoError(t, n.Scan(ts))
	require.False(t, n.IsNull())
	v, _ := n.Get()
	require.True(t, ts.Equal(v))
}

func TestTime_Value_null(t *testing.T) {
	v, err := nullable.TimeEmpty().Value()
	require.NoError(t, err)
	require.Nil(t, v)
}

func TestTime_Value_present(t *testing.T) {
	ts := newTestTime()
	v, err := nullable.TimeOf(ts).Value()
	require.NoError(t, err)
	tv, ok := v.(time.Time)
	require.True(t, ok)
	require.True(t, ts.Equal(tv))
}
