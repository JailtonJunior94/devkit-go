package httpserver

import (
	"context"
	"net/http"

	"github.com/JailtonJunior94/devkit-go/pkg/vos"
)

type ContextKey string

const ContextKeyRequestID ContextKey = "request-id"

func RequestID(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id, err := vos.NewUUID()
		if err != nil {
			next.ServeHTTP(w, r)
			return
		}
		ctx := context.WithValue(r.Context(), ContextKeyRequestID, id.String())
		r = r.WithContext(ctx)
		next.ServeHTTP(w, r)
	})
}
