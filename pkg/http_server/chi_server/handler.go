package chiserver

import (
	"context"
	"net/http"
)

// Handler is the idiomatic handler signature that returns an error.
// It mirrors the removed legacy httpserver Handler API to allow consumers
// to migrate without changing call sites.
type Handler func(w http.ResponseWriter, r *http.Request) error

// ErrorHandler handles errors returned by Handler. It receives the
// request context for correlation (request_id, trace) and the
// http.ResponseWriter to produce the response body.
type ErrorHandler func(ctx context.Context, w http.ResponseWriter, err error)

// Middleware is the idiomatic net/http middleware type.
type Middleware func(http.Handler) http.Handler

// requestPathContextKey carries the originating request URL path so
// ErrorHandlers (which only receive ctx) can build RFC 7807 instance.
type requestPathContextKey struct{}

// adaptHandler converts a Handler into an http.HandlerFunc, capturing
// any returned error and delegating to the configured ErrorHandler.
// It also stashes the request URL path in the context so the error
// handler can build a ProblemDetail without needing the *http.Request.
func adaptHandler(h Handler, eh ErrorHandler) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		err := h(w, r)
		if err == nil {
			return
		}
		ctx := context.WithValue(r.Context(), requestPathContextKey{}, r.URL.Path)
		eh(ctx, w, err)
	}
}

// requestPath returns the URL path stashed by adaptHandler, or the
// route pattern (when chi route context is present) as a fallback.
func requestPath(ctx context.Context) string {
	if v, ok := ctx.Value(requestPathContextKey{}).(string); ok {
		return v
	}
	return ""
}
