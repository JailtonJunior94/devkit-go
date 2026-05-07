package common

import (
	"errors"
	"fmt"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/stretchr/testify/require"
)

type customError struct{ msg string }

func (e *customError) Error() string { return e.msg }

func TestProblemFromError_TableDriven(t *testing.T) {
	t.Parallel()

	sensitiveErr := errors.New("postgres: password authentication failed for user 'admin'")
	wrappedFiber := fmt.Errorf("decoding payload: %w", fiber.NewError(http.StatusBadRequest, "invalid json"))
	deepFiberWrap := fmt.Errorf("outer: %w", fmt.Errorf("middle: %w", fiber.NewError(http.StatusUnprocessableEntity, "validation failed")))

	tests := []struct {
		name           string
		err            error
		instance       string
		requestID      string
		wantStatus     int
		wantTitle      string
		wantDetail     string
		wantTypeSuffix string
		expectLeak     string // substring that must NOT appear in Detail
	}{
		{
			name:           "fiber error preserves code and message",
			err:            fiber.NewError(http.StatusNotFound, "resource missing"),
			instance:       "/v1/items/42",
			requestID:      "req-1",
			wantStatus:     http.StatusNotFound,
			wantTitle:      "Not Found",
			wantDetail:     "resource missing",
			wantTypeSuffix: "/404",
		},
		{
			name:           "errors.As traverses wrap chain",
			err:            wrappedFiber,
			instance:       "/v1/items",
			requestID:      "req-2",
			wantStatus:     http.StatusBadRequest,
			wantTitle:      "Bad Request",
			wantDetail:     "invalid json",
			wantTypeSuffix: "/400",
		},
		{
			name:           "deep wrap chain still resolves fiber error",
			err:            deepFiberWrap,
			instance:       "/v1/items",
			requestID:      "req-2b",
			wantStatus:     http.StatusUnprocessableEntity,
			wantTitle:      "Unprocessable Entity",
			wantDetail:     "validation failed",
			wantTypeSuffix: "/422",
		},
		{
			name:           "arbitrary error returns sanitized 500",
			err:            errors.New("database connection refused at 10.0.0.5:5432"),
			instance:       "/v1/items",
			requestID:      "req-3",
			wantStatus:     http.StatusInternalServerError,
			wantTitle:      "Internal Server Error",
			wantDetail:     "internal server error",
			wantTypeSuffix: "/500",
			expectLeak:     "10.0.0.5:5432",
		},
		{
			name:           "custom error type returns sanitized 500",
			err:            &customError{msg: "secret token leaked"},
			instance:       "/v1/items",
			requestID:      "req-4",
			wantStatus:     http.StatusInternalServerError,
			wantTitle:      "Internal Server Error",
			wantDetail:     "internal server error",
			wantTypeSuffix: "/500",
			expectLeak:     "secret token leaked",
		},
		{
			name:           "nil error returns 500 fallback",
			err:            nil,
			instance:       "/healthz",
			requestID:      "",
			wantStatus:     http.StatusInternalServerError,
			wantTitle:      "Internal Server Error",
			wantDetail:     "internal server error",
			wantTypeSuffix: "/500",
		},
		{
			name:           "sensitive infra error never appears in detail",
			err:            sensitiveErr,
			instance:       "/v1/items",
			requestID:      "req-5",
			wantStatus:     http.StatusInternalServerError,
			wantTitle:      "Internal Server Error",
			wantDetail:     "internal server error",
			wantTypeSuffix: "/500",
			expectLeak:     "password authentication",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			before := time.Now().UTC()
			got := ProblemFromError(tc.err, tc.instance, tc.requestID)
			after := time.Now().UTC()

			require.Equal(t, tc.wantStatus, got.Status)
			require.Equal(t, tc.wantTitle, got.Title)
			require.Equal(t, tc.wantDetail, got.Detail)
			require.Equal(t, tc.instance, got.Instance)
			require.Equal(t, tc.requestID, got.RequestID)
			require.True(t, strings.HasSuffix(got.Type, tc.wantTypeSuffix), "expected Type %q to end with %q", got.Type, tc.wantTypeSuffix)
			require.True(t, strings.HasPrefix(got.Type, "https://"), "expected Type to be an https URI, got %q", got.Type)

			require.False(t, got.Timestamp.Before(before), "timestamp older than call start")
			require.False(t, got.Timestamp.After(after), "timestamp newer than call end")
			require.Equal(t, time.UTC, got.Timestamp.Location(), "timestamp must be UTC")

			if tc.expectLeak != "" {
				require.NotContains(t, got.Detail, tc.expectLeak, "Detail must not leak original error text")
			}
		})
	}
}

func TestProblemFromError_FiberErrorWithEmptyMessage(t *testing.T) {
	t.Parallel()

	got := ProblemFromError(fiber.NewError(http.StatusTeapot, ""), "/teapot", "")
	require.Equal(t, http.StatusTeapot, got.Status)
	require.Equal(t, "", got.Detail, "fiber.Error message is preserved verbatim, even when empty")
}

func TestProblemTypeURI_UsesStatusCode(t *testing.T) {
	t.Parallel()

	require.Equal(t, "https://httpstatuses.com/400", problemTypeURI(http.StatusBadRequest))
	require.Equal(t, "https://httpstatuses.com/500", problemTypeURI(http.StatusInternalServerError))
}
