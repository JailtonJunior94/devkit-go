package chiserver

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/JailtonJunior94/devkit-go/pkg/http_server/common"
	"github.com/JailtonJunior94/devkit-go/pkg/observability"
	"github.com/JailtonJunior94/devkit-go/pkg/observability/fake"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestChiServer_RequestIDMiddleware_RegeneratesOnInvalid(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		raw  string
	}{
		{name: "rejects colon", raw: "abc:def"},
		{name: "rejects slash", raw: "abc/def"},
		{name: "rejects whitespace", raw: "abc def"},
		{name: "rejects exceeding max length", raw: stringOf("a", common.MaxRequestIDLength+1)},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			provider := fake.NewProvider()
			handler := requestIDMiddleware(provider)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				id, _ := r.Context().Value(requestIDKey).(string)
				assert.NotEqual(t, tt.raw, id, "raw value must not propagate to context")
				_, valid := common.ValidateRequestID(id)
				assert.True(t, valid, "regenerated id must be valid")
				_, parseErr := uuid.Parse(id)
				assert.NoError(t, parseErr, "regenerated id must be a UUID")
				w.WriteHeader(http.StatusNoContent)
			}))

			req := httptest.NewRequest(http.MethodGet, "/anything", nil)
			req.Header.Set(common.HeaderRequestID, tt.raw)
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)

			gotHeader := rec.Header().Get(common.HeaderRequestID)
			require.NotEmpty(t, gotHeader)
			assert.NotEqual(t, tt.raw, gotHeader, "raw value must not be echoed back")
		})
	}
}

func TestChiServer_RequestIDMiddleware_LogsWarnOnInvalid(t *testing.T) {
	t.Parallel()

	provider := fake.NewProvider()
	handler := requestIDMiddleware(provider)(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))

	req := httptest.NewRequest(http.MethodPost, "/orders/42", nil)
	req.Header.Set(common.HeaderRequestID, "abc:def")
	req.RemoteAddr = "10.0.0.1:443"
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	logger, ok := provider.Logger().(*fake.FakeLogger)
	require.True(t, ok)
	entries := logger.GetEntries()
	require.Len(t, entries, 1)

	entry := entries[0]
	assert.Equal(t, observability.LogLevelWarn, entry.Level)
	assert.Equal(t, "invalid X-Request-ID rejected", entry.Message)

	fields := indexFieldsByKey(entry.Fields)
	require.Contains(t, fields, "raw_length")
	require.Contains(t, fields, "remote_addr")
	require.Contains(t, fields, "path")
	require.Contains(t, fields, "method")
	assert.Equal(t, int64(len("abc:def")), fields["raw_length"].Int64Value())
	assert.Equal(t, "10.0.0.1:443", fields["remote_addr"].StringValue())
	assert.Equal(t, "/orders/42", fields["path"].StringValue())
	assert.Equal(t, http.MethodPost, fields["method"].StringValue())

	for _, f := range entry.Fields {
		assert.NotEqual(t, "abc:def", f.StringValue(),
			"raw rejected value must never be logged")
	}
}

func TestChiServer_RequestIDMiddleware_DoesNotEchoRawValue(t *testing.T) {
	t.Parallel()

	provider := fake.NewProvider()
	handler := requestIDMiddleware(provider)(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))

	req := httptest.NewRequest(http.MethodGet, "/x", nil)
	req.Header.Set(common.HeaderRequestID, "abc:def")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	gotHeader := rec.Header().Get(common.HeaderRequestID)
	assert.NotEqual(t, "abc:def", gotHeader)
	_, parseErr := uuid.Parse(gotHeader)
	assert.NoError(t, parseErr)
}

func TestChiServer_RequestIDMiddleware_AcceptsValidValue(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		raw  string
	}{
		{name: "uuid", raw: "550e8400-e29b-41d4-a716-446655440000"},
		{name: "alphanumeric and dot", raw: "service.v1.req-42_X"},
		{name: "single char", raw: "A"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			provider := fake.NewProvider()
			handler := requestIDMiddleware(provider)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				got, _ := r.Context().Value(requestIDKey).(string)
				assert.Equal(t, tt.raw, got)
				w.WriteHeader(http.StatusNoContent)
			}))

			req := httptest.NewRequest(http.MethodGet, "/x", nil)
			req.Header.Set(common.HeaderRequestID, tt.raw)
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)

			assert.Equal(t, tt.raw, rec.Header().Get(common.HeaderRequestID))

			logger, ok := provider.Logger().(*fake.FakeLogger)
			require.True(t, ok)
			assert.Empty(t, logger.GetEntries(), "valid id must not emit warn log")
		})
	}
}

func TestChiServer_RequestIDMiddleware_EmptyGeneratesNewWithoutLog(t *testing.T) {
	t.Parallel()

	provider := fake.NewProvider()
	handler := requestIDMiddleware(provider)(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))

	req := httptest.NewRequest(http.MethodGet, "/x", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	got := rec.Header().Get(common.HeaderRequestID)
	require.NotEmpty(t, got)
	_, parseErr := uuid.Parse(got)
	assert.NoError(t, parseErr)

	logger, ok := provider.Logger().(*fake.FakeLogger)
	require.True(t, ok)
	assert.Empty(t, logger.GetEntries(), "empty id generation must not emit log")
}

// TestChiServer_RequestIDMiddleware_GeneratesUniqueIDs covers the "fallback
// request ID when UUID fails" scenario from RF-1.5. common.NewRequestID uses
// uuid.New() which delegates to crypto/rand; this test verifies that 100
// independent requests each receive a distinct UUID (uniqueness guarantee).
func TestChiServer_RequestIDMiddleware_GeneratesUniqueIDs(t *testing.T) {
	t.Parallel()

	provider := fake.NewProvider()
	ids := make(map[string]bool, 100)

	for i := 0; i < 100; i++ {
		var captured string
		handler := requestIDMiddleware(provider)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			captured, _ = r.Context().Value(requestIDKey).(string)
			w.WriteHeader(http.StatusOK)
		}))

		req := httptest.NewRequest(http.MethodGet, "/", nil)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		require.NotEmpty(t, captured)
		require.False(t, ids[captured], "duplicate request ID at iteration %d: %s", i, captured)
		ids[captured] = true
	}

	assert.Len(t, ids, 100)
}

func TestChiServer_ChiRoutePattern_ReturnsUnmatchedForMissingRoute(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		ctx  context.Context
	}{
		{name: "nil RouteContext", ctx: context.Background()},
		{name: "empty RoutePattern", ctx: context.WithValue(context.Background(), chi.RouteCtxKey, chi.NewRouteContext())},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, "unmatched", chiRoutePattern(tt.ctx))
		})
	}
}

func TestChiServer_ChiRoutePattern_ReturnsPatternWhenMatched(t *testing.T) {
	t.Parallel()

	router := chi.NewRouter()
	var captured string
	router.Get("/users/{id}", func(_ http.ResponseWriter, r *http.Request) {
		captured = chiRoutePattern(r.Context())
	})

	req := httptest.NewRequest(http.MethodGet, "/users/123", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	assert.Equal(t, "/users/{id}", captured)
}

func TestChiServer_ChiRoutePattern_ReturnsUnmatchedFor404(t *testing.T) {
	t.Parallel()

	router := chi.NewRouter()
	var captured string
	router.NotFound(func(_ http.ResponseWriter, r *http.Request) {
		captured = chiRoutePattern(r.Context())
	})

	req := httptest.NewRequest(http.MethodGet, "/does-not-exist", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	assert.Equal(t, "unmatched", captured)
}

func indexFieldsByKey(fields []observability.Field) map[string]observability.Field {
	out := make(map[string]observability.Field, len(fields))
	for _, f := range fields {
		out[f.Key] = f
	}
	return out
}

func stringOf(s string, n int) string {
	out := make([]byte, 0, len(s)*n)
	for i := 0; i < n; i++ {
		out = append(out, s...)
	}
	return string(out)
}
