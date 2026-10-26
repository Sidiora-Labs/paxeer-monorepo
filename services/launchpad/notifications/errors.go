package notifications

import (
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"

	v2 "github.com/Sidiora-Technologies/KindleLaunch/shared/interfaces/v2"
	"github.com/google/uuid"
)

func writeError(w http.ResponseWriter, r *http.Request, status int, code string) {
	var requestID uuid.UUID
	var err error
	if r != nil {
		requestID, err = uuid.Parse(r.Header.Get("X-Request-Id"))
	}
	if r == nil || err != nil {
		requestID = uuid.New()
	}
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "private, no-store")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v2.Error{
		Code:         code,
		Message:      http.StatusText(status),
		RequestId:    requestID.String(),
		Retryable:    status == http.StatusServiceUnavailable,
		RetryAfterMs: nil,
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
