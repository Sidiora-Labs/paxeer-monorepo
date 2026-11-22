package app

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Sidiora-Technologies/KindleLaunch/shared/process"
)

func TestLeaseStandbyReadinessNamesStandbyIngestion(t *testing.T) {
	tracker := process.NewTracker()
	next := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"status":"ok","checks":{"database":"ok","redis":"ok","ingestion":"ok"}}`))
	})
	h := runtimeHandlerWithIngestionState(tracker, next, nil, func() string { return "standby" })
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/health/ready", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("standby readiness status = %d", rec.Code)
	}
	var body struct {
		Checks map[string]string `json:"checks"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body.Checks["ingestion"] != "standby" {
		t.Fatalf("ingestion readiness = %q, want standby", body.Checks["ingestion"])
	}
}
