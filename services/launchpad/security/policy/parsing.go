package policy

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net"
	"net/http"
	"strings"
	"time"
)

type MutationSchema struct {
	BodyBytes int64
	Arrays    map[string]int
}

func DecodeMutationSchema(r *http.Request, dst any, schema MutationSchema) error {
	if r == nil || r.Body == nil || schema.BodyBytes <= 0 || schema.BodyBytes > 8<<20 || len(schema.Arrays) > 128 {
		return ErrDenied
	}
	body, err := io.ReadAll(io.LimitReader(r.Body, schema.BodyBytes+1))
	if err != nil {
		return err
	}
	if int64(len(body)) > schema.BodyBytes {
		return &http.MaxBytesError{Limit: schema.BodyBytes}
	}
	clone := r.Clone(r.Context())
	clone.Body = io.NopCloser(bytes.NewReader(body))
	var parsed any
	if err = DecodeMutation(clone, &parsed, schema.BodyBytes); err != nil {
		return err
	}
	var value any
	decoder := json.NewDecoder(bytes.NewReader(body))
	decoder.UseNumber()
	if err = decoder.Decode(&value); err != nil {
		return err
	}
	var visit func(any, string) error
	visit = func(v any, path string) error {
		switch x := v.(type) {
		case []any:
			bound, ok := schema.Arrays[path]
			if !ok || bound < 0 || bound > 1000 || len(x) > bound {
				return ErrDenied
			}
			for _, item := range x {
				if err := visit(item, path+"/*"); err != nil {
					return err
				}
			}
		case map[string]any:
			for key, item := range x {
				if err := visit(item, path+"/"+strings.ReplaceAll(strings.ReplaceAll(key, "~", "~0"), "/", "~1")); err != nil {
					return err
				}
			}
		}
		return nil
	}
	if err = visit(value, ""); err != nil {
		return err
	}
	decoder = json.NewDecoder(bytes.NewReader(body))
	decoder.DisallowUnknownFields()
	return decodeDestination(decoder, dst)
}
func MutationStatus(err error) int {
	var timeout net.Error
	if errors.Is(err, context.DeadlineExceeded) || errors.As(err, &timeout) && timeout.Timeout() {
		return http.StatusRequestTimeout
	}
	var tooLarge *http.MaxBytesError
	if errors.As(err, &tooLarge) {
		return http.StatusRequestEntityTooLarge
	}
	return http.StatusBadRequest
}
func RequestBounds(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if len(r.URL.RawQuery) > 8192 {
			http.Error(w, "query_too_large", http.StatusRequestURITooLong)
			return
		}
		total := 0
		for key, values := range r.Header {
			for _, value := range values {
				total += len(key) + len(value) + 4
			}
		}
		if total > 16384 {
			http.Error(w, "headers_too_large", http.StatusRequestHeaderFieldsTooLarge)
			return
		}
		if r.Method == http.MethodGet || r.Method == http.MethodHead || r.Method == http.MethodOptions {
			// Read streams use their own heartbeat and cancellation policy.
			next.ServeHTTP(w, r)
			return
		}
		deadline := time.Now().Add(MutationRequestTimeout)
		controller := http.NewResponseController(w)
		if err := controller.SetReadDeadline(deadline); err != nil {
			http.Error(w, "request_deadline_unavailable", http.StatusServiceUnavailable)
			return
		}
		if err := controller.SetWriteDeadline(deadline.Add(time.Second)); err != nil {
			http.Error(w, "response_deadline_unavailable", http.StatusServiceUnavailable)
			return
		}
		defer func() {
			if err := controller.SetReadDeadline(time.Time{}); err != nil {
				slog.Warn("request deadline reset", "error", err)
			}
		}()
		defer func() {
			if err := controller.SetWriteDeadline(time.Time{}); err != nil {
				slog.Warn("response deadline reset", "error", err)
			}
		}()
		defer func() {
			if err := r.Body.Close(); err != nil {
				slog.Warn("request body close", "error", err)
			}
		}()
		ctx, cancel := context.WithDeadline(r.Context(), deadline)
		defer cancel()
		http.TimeoutHandler(next, MutationRequestTimeout, "request_timeout").ServeHTTP(w, r.WithContext(ctx))
	})
}

const MutationRequestTimeout = 10 * time.Second

func ProductionServer(address string, handler http.Handler) *http.Server {
	return &http.Server{Addr: address, Handler: RequestBounds(handler), ReadHeaderTimeout: 5 * time.Second, IdleTimeout: 60 * time.Second, MaxHeaderBytes: 16384}
}
