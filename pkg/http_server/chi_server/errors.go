package chiserver

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/JailtonJunior94/devkit-go/pkg/http_server/common"
)

// writeErrorResponse delegates to common.ProblemFromError to build an
// RFC 7807 application/problem+json response. The original err is never
// reflected to the client (R-SEC-001); callers are responsible for
// logging it via pkg/observability.
func writeErrorResponse(w http.ResponseWriter, r *http.Request, err error) {
	requestID, _ := r.Context().Value(requestIDKey).(string)
	problem := common.ProblemFromError(err, r.URL.Path, requestID)

	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(problem.Status)

	_ = json.NewEncoder(w).Encode(problem)
}

// writeStatusError writes a fixed status/detail RFC 7807 response.
// Used by middlewares that already know the canonical (code, detail)
// pair (e.g. body-limit exceeded, CORS rejection, panic recovery) and
// must not derive these from a raw error value.
func writeStatusError(w http.ResponseWriter, r *http.Request, code int, detail string) {
	requestID, _ := r.Context().Value(requestIDKey).(string)

	problem := common.ProblemDetail{
		Type:      fmt.Sprintf("https://httpstatuses.com/%d", code),
		Title:     common.GetStatusText(code),
		Status:    code,
		Detail:    detail,
		Instance:  r.URL.Path,
		Timestamp: time.Now().UTC(),
		RequestID: requestID,
	}

	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(code)

	_ = json.NewEncoder(w).Encode(problem)
}
