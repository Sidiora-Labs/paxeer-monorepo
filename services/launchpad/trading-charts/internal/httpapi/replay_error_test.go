package httpapi

import (
	"encoding/json"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"

	v2 "github.com/Sidiora-Technologies/KindleLaunch/shared/interfaces/v2"
	"github.com/go-chi/chi/v5"
)

func TestReplayGatewayOutageAnswersFrozenError(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	origin := "http://" + listener.Addr().String()
	if err := listener.Close(); err != nil {
		t.Fatal(err)
	}

	router := chi.NewRouter()
	if err := RegisterReplayGateway(router, origin); err != nil {
		t.Fatal(err)
	}
	response := httptest.NewRecorder()
	router.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/v2/snapshots", nil))

	if response.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want %d; body = %s", response.Code, http.StatusServiceUnavailable, response.Body.String())
	}
	if got := response.Header().Get("Retry-After"); got != "1" {
		t.Errorf("Retry-After = %q, want %q", got, "1")
	}
	body := response.Body.Bytes()
	if err := v2.ValidateJSON("Error", body); err != nil {
		t.Fatalf("response violates frozen Error schema: %v; body = %s", err, body)
	}
	var envelope struct {
		Code         string `json:"code"`
		Retryable    bool   `json:"retryable"`
		RetryAfterMs int    `json:"retryAfterMs"`
	}
	if err := json.Unmarshal(body, &envelope); err != nil {
		t.Fatal(err)
	}
	if envelope.Code != "DEPENDENCY_UNAVAILABLE" {
		t.Errorf("code = %q, want DEPENDENCY_UNAVAILABLE", envelope.Code)
	}
	if !envelope.Retryable || envelope.RetryAfterMs != 1000 {
		t.Errorf("retryable = %t, retryAfterMs = %d, want true and 1000", envelope.Retryable, envelope.RetryAfterMs)
	}
}
