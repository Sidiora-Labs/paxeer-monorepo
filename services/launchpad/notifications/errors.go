package notifications

import (
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"

	v2 "github.com/Sidiora-Technologies/KindleLaunch/shared/interfaces/v2"
)

func writeError(w http.ResponseWriter, r *http.Request, status int, code string) {
	requestID := notificationRequestID(r)
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "private, no-store")
	var retryAfterMs *v2.SafeInteger
	if status == http.StatusServiceUnavailable {
		delay := v2.SafeInteger(1000)
		retryAfterMs = &delay
		w.Header().Set("Retry-After", "1")
	}
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v2.Error{
		Code:         code,
		Message:      http.StatusText(status),
		RequestId:    requestID,
		Retryable:    status == http.StatusServiceUnavailable,
		RetryAfterMs: retryAfterMs,
		FieldErrors:  []v2.FieldError{},
		Details:      nil,
	})
}

func errorStatus(e error) (int, string) {
	switch {
	case errors.Is(e, sql.ErrNoRows):
		return http.StatusNotFound, "NOT_FOUND"
	case errors.Is(e, ErrInvalid):
		return http.StatusBadRequest, "INVALID_REQUEST"
	case errors.Is(e, ErrForbidden):
		return http.StatusForbidden, "SCOPE_DENIED"
	default:
		return http.StatusServiceUnavailable, "DEPENDENCY_UNAVAILABLE"
	}
}
