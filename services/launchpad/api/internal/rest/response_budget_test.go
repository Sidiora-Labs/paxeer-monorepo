package rest

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestResponseBudget(t *testing.T) {
	for _, tc := range []struct {
		name   string
		budget time.Duration
		wrap   func(http.Handler) http.Handler
		status int
	}{
		{"discovery runtime", 3 * time.Second, func(next http.Handler) http.Handler {
			return ResponseMetadataWithBudget(next, 3*time.Second)
		}, http.StatusOK},
		{"default", 400 * time.Millisecond, ResponseMetadata, http.StatusGatewayTimeout},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var contextError error
			started := time.Now()
			handler := tc.wrap(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				deadline, ok := r.Context().Deadline()
				if !ok || deadline.Before(started.Add(tc.budget)) || deadline.After(time.Now().Add(tc.budget)) {
					t.Fatalf("request deadline = %v, want budget %v", deadline, tc.budget)
				}
				timer := time.NewTimer(600 * time.Millisecond)
				defer timer.Stop()
				select {
				case <-r.Context().Done():
					contextError = r.Context().Err()
					w.WriteHeader(http.StatusGatewayTimeout)
				case <-timer.C:
					w.WriteHeader(http.StatusOK)
				}
			}))
			response := httptest.NewRecorder()
			handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/v2/discovery/feed", nil))
			if response.Code != tc.status {
				t.Fatalf("status = %d, want %d; context = %v", response.Code, tc.status, contextError)
			}
			if tc.status == http.StatusGatewayTimeout && !errors.Is(contextError, context.DeadlineExceeded) {
				t.Fatalf("context = %v, want deadline exceeded", contextError)
			}
			if response.Header().Get("X-Schema-Version") != "2" || response.Header().Get("X-Request-ID") == "" {
				t.Fatalf("response metadata = %v", response.Header())
			}
		})
	}
}

func TestResponseBudgetPreservesParentDeadline(t *testing.T) {
	ctx, cancel := context.WithDeadline(context.Background(), time.Now().Add(-time.Second))
	defer cancel()
	deadline, _ := ctx.Deadline()
	handler := ResponseMetadataWithBudget(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		actual, ok := r.Context().Deadline()
		if !ok || !actual.Equal(deadline) || !errors.Is(r.Context().Err(), context.DeadlineExceeded) {
			t.Fatalf("parent deadline = %v, context = %v", actual, r.Context().Err())
		}
		w.WriteHeader(http.StatusGatewayTimeout)
	}), 3*time.Second)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/v2/discovery/feed", nil).WithContext(ctx))
	if response.Code != http.StatusGatewayTimeout {
		t.Fatalf("status = %d", response.Code)
	}
}
