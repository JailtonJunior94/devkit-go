package chiserver

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/JailtonJunior94/devkit-go/pkg/http_server/common"
)

// writeErrorResponse writes an error response following RFC 7807.
func writeErrorResponse(w http.ResponseWriter, r *http.Request, code int, detail string) {
	requestID, _ := r.Context().Value(requestIDKey).(string)

	problem := common.ProblemDetail{
		Type:      fmt.Sprintf("https://httpstatuses.com/%d", code),
		Title:     common.GetStatusText(code),
		Status:    code,
		Detail:    detail,
		Instance:  r.URL.Path,
		Timestamp: time.Now(),
		RequestID: requestID,
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)

	_ = json.NewEncoder(w).Encode(problem)
}
