package rest

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"

	"github.com/Sidiora-Technologies/KindleLaunch/core/api/internal/store"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

type developmentReadErrorKey struct{}
type developmentReadError struct {
	cause     error
	operation string
}

// Only fixed classes and SQLSTATE codes are logged: errors can contain SQL
// arguments, URLs and other request values and must never be rendered here.
func developmentErrorClass(err error) string {
	var pg *pgconn.PgError
	switch {
	case errors.As(err, &pg):
		return "postgres_" + pg.Code
	case errors.Is(err, context.DeadlineExceeded):
		return "deadline_exceeded"
	case errors.Is(err, context.Canceled):
		return "canceled"
	case errors.Is(err, store.ErrDataIntegrity):
		return "data_integrity"
	case errors.Is(err, store.ErrAccountingNotReady):
		return "projection_unavailable"
	case errors.Is(err, errChainReferenceUnavailable):
		return "chain_reference_unavailable"
	case errors.Is(err, ErrDependencyNotConfigured):
		return "dependency_not_configured"
	case errors.Is(err, pgx.ErrNoRows):
		return "projection_row_missing"
	case errors.Is(err, pgx.ErrTxClosed):
		return "transaction_closed"
	case errors.Is(err, pgx.ErrTxCommitRollback):
		return "transaction_rolled_back"
	default:
		return "dependency_error"
	}
}

func noteDevelopmentReadOperation(r *http.Request, operation string) {
	if state, ok := r.Context().Value(developmentReadErrorKey{}).(*developmentReadError); ok {
		state.operation = operation
	}
}

func noteDevelopmentReadError(r *http.Request, err error) {
	if state, ok := r.Context().Value(developmentReadErrorKey{}).(*developmentReadError); ok {
		state.cause = err
	}
}

type developmentReadWriter struct {
	http.ResponseWriter
	status int
	code   string
}

func (w *developmentReadWriter) WriteHeader(status int) {
	if w.status == 0 {
		w.status = status
	}
	w.ResponseWriter.WriteHeader(status)
}
func (w *developmentReadWriter) Write(raw []byte) (int, error) {
	if w.status == 0 {
		w.status = http.StatusOK
	}
	if w.status == 503 {
		var failure struct {
			Code string `json:"code"`
		}
		if json.Unmarshal(raw, &failure) == nil {
			switch failure.Code {
			case literalDEPENDENCYUNAVAILABLE, literalINTEGRITYQUARANTINED:
				w.code = failure.Code
			}
		}
	}
	return w.ResponseWriter.Write(raw)
}

func logDevelopmentReadFailures(route string, handler http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		state := &developmentReadError{}
		r = r.WithContext(context.WithValue(r.Context(), developmentReadErrorKey{}, state))
		capture := &developmentReadWriter{ResponseWriter: w}
		handler.ServeHTTP(capture, r)
		if capture.status == 503 {
			class := capture.code
			if state.cause != nil {
				class = developmentErrorClass(state.cause)
			}
			if class == "" {
				class = "dependency_error"
			}
			slog.ErrorContext(r.Context(), "development read failed", "route", route, "error_class", class, "operation", state.operation)
		}
	})
}
