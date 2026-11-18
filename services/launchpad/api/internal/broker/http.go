package broker

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/google/uuid"
)

// WriteStreamHTTPError answers a rejected stream request before streaming starts.
func WriteStreamHTTPError(w http.ResponseWriter, r *http.Request, status int, code string) {
	id := w.Header().Get("X-Request-ID")
	if _, err := uuid.Parse(id); err != nil {
		id = uuid.NewString()
	}
	w.Header().Set("X-Request-ID", id)
	w.Header().Set("Content-Type", "application/json")
	if strings.Contains(w.Header().Get("Cache-Control"), "private") || r.Header.Get("Authorization") != "" || r.Header.Get("Cookie") != "" {
		w.Header().Set("Cache-Control", "private, no-store")
	} else {
		w.Header().Set("Cache-Control", "no-store")
	}
	w.Header().Set("X-Schema-Version", "2")
	w.Header().Del("Retry-After")
	var retryAfter *int
	if status == http.StatusServiceUnavailable {
		milliseconds := 1000
		retryAfter = &milliseconds
		w.Header().Set("Retry-After", "1")
	}
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(struct {
		Code         string `json:"code"`
		Message      string `json:"message"`
		RequestID    string `json:"requestId"`
		Retryable    bool   `json:"retryable"`
		RetryAfterMS *int   `json:"retryAfterMs"`
		FieldErrors  []any  `json:"fieldErrors"`
		Details      any    `json:"details"`
	}{code, code, id, retryAfter != nil, retryAfter, []any{}, nil})
}
