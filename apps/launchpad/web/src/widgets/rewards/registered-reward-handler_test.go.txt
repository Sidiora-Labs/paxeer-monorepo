// This test is added to the real app package through Go's read-only overlay.
// The server uses the production registrar and handler with unavailable backend
// dependencies, as in app/reward_routes_test.go; no operation is substituted.
package app

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/Sidiora-Technologies/KindleLaunch/core/pnl-tracker/internal/production/l18"
	iface "github.com/Sidiora-Technologies/KindleLaunch/shared/interfaces/v2"
	"github.com/go-chi/chi/v5"
)

func TestWebRewardClaimHandler(t *testing.T) {
	router := chi.NewRouter()
	registerRewardRoutes(router, l18.HTTP{})
	stop := make(chan struct{}, 1)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost && r.URL.Path == "/__reward_proof/stop" {
			w.WriteHeader(http.StatusNoContent)
			stop <- struct{}{}
			return
		}
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Error(err)
			return
		}
		r.Body = io.NopCloser(bytes.NewReader(body))
		request, decodeErr := iface.DecodeRewardClaimBody(body)
		response := httptest.NewRecorder()
		router.ServeHTTP(response, r)
		var envelope struct {
			Code string `json:"code"`
		}
		_ = json.Unmarshal(response.Body.Bytes(), &envelope)
		observation, err := json.Marshal(map[string]any{
			"method": r.Method, "path": r.URL.Path,
			"authorization": r.Header.Get("Authorization"),
			"contentType":   r.Header.Get("Content-Type"),
			"keys":          r.Header.Values("Idempotency-Key"),
			"body":          string(body), "validBody": decodeErr == nil,
			"request": request, "status": response.Code, "code": envelope.Code,
		})
		if err != nil {
			t.Error(err)
			return
		}
		fmt.Printf("REWARD_OBSERVATION %s\n", observation)
		for key, values := range response.Header() {
			w.Header()[key] = values
		}
		w.WriteHeader(response.Code)
		_, _ = w.Write(response.Body.Bytes())
	}))
	defer server.Close()
	fmt.Printf("REWARD_ORIGIN %s\n", server.URL)
	select {
	case <-stop:
	case <-time.After(time.Minute):
		t.Fatal("web consumer did not finish the registered-handler proof")
	}
}
