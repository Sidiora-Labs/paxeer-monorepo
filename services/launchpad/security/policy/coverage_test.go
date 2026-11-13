package policy

import (
	"bytes"
	"errors"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"
)

func TestSecurityTelemetryRejectsInvalidEventsAndClosedWrites(t *testing.T) {
	key := []byte(strings.Repeat("k", 32))
	file, err := os.CreateTemp(t.TempDir(), "security-event-*")
	if err != nil {
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
	logger := &SecurityLog{Writer: file, Key: key}
	if err := logger.Write("request", "session", "operator", "allow", "v1", http.Header{}); !errors.Is(err, os.ErrClosed) {
		t.Fatalf("closed security write: %v", err)
	}
	logger.Writer = &bytes.Buffer{}
	for _, event := range []struct{ request, class, outcome, version string }{
		{"bad request", "operator", "allow", "v1"},
		{"request", "unclassified", "allow", "v1"},
		{"request", "operator", "other", "v1"},
		{"request", "operator", "allow", "V1"},
	} {
		if err := logger.Write(event.request, "session", event.class, event.outcome, event.version, http.Header{}); !errors.Is(err, ErrDenied) {
			t.Fatalf("invalid security event %+v: %v", event, err)
		}
	}
	request, err := http.NewRequest(http.MethodGet, "/", nil)
	if err != nil {
		t.Fatal(err)
	}
	request.Header.Set("X-Request-ID", "request")
	capture := &MiddlewareCapture{Writer: file, Key: key, RunID: "run", ReplicaID: "replica", CandidateSHA256: strings.Repeat("a", 64), PolicyVersion: "v1"}
	if err := capture.Record(request, "authorization", time.Now().Add(-time.Second), true, false); !errors.Is(err, os.ErrClosed) {
		t.Fatalf("closed capture write: %v", err)
	}
	capture.Writer = &bytes.Buffer{}
	if err := capture.Record(request, "unknown", time.Now().Add(-time.Second), true, false); !errors.Is(err, ErrDenied) {
		t.Fatalf("unknown capture stage: %v", err)
	}
	request.Header.Add("X-Request-ID", "second")
	if err := capture.Record(request, "limiter", time.Now().Add(-time.Second), false, true); !errors.Is(err, ErrDenied) {
		t.Fatalf("ambiguous request ID: %v", err)
	}
}
