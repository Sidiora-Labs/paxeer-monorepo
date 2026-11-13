package telemetry

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestReadMetricsExposeEligibleLatencyBuckets(t *testing.T) {
	metrics, err := New("userd", "qualification", "zone-a")
	if err != nil {
		t.Fatal(err)
	}
	for _, sample := range []struct {
		status   int
		duration time.Duration
	}{
		{http.StatusOK, 100 * time.Millisecond},
		{http.StatusServiceUnavailable, 300 * time.Millisecond},
		{http.StatusOK, 600 * time.Millisecond},
	} {
		if err := metrics.RecordRead("browse", sample.status, sample.duration, ""); err != nil {
			t.Fatal(err)
		}
	}
	for _, exclusion := range []string{"synthetic", "health", "client_cancel", "invalid_request"} {
		if err := metrics.RecordRead("browse", http.StatusOK, time.Millisecond, exclusion); err != nil {
			t.Fatalf("excluded %s read: %v", exclusion, err)
		}
	}
	for _, invalid := range []struct {
		route     string
		status    int
		duration  time.Duration
		exclusion string
	}{
		{"unknown", http.StatusOK, time.Millisecond, ""},
		{"browse", 99, time.Millisecond, ""},
		{"browse", http.StatusOK, -time.Millisecond, ""},
		{"browse", http.StatusOK, time.Millisecond, "private"},
	} {
		if err := metrics.RecordRead(invalid.route, invalid.status, invalid.duration, invalid.exclusion); err == nil {
			t.Fatalf("invalid read admitted: %+v", invalid)
		}
	}
	var output bytes.Buffer
	if err := metrics.WritePrometheus(&output); err != nil {
		t.Fatal(err)
	}
	for _, metric := range []struct {
		name string
		want string
	}{
		{"sli_read_eligible_requests_total", "3"},
		{"sli_read_good_requests_total", "2"},
		{"sli_read_fast150_requests_total", "1"},
		{"sli_read_fast400_requests_total", "2"},
	} {
		line := metric.name + `{service="userd",environment="qualification",zone="zone-a",route="browse"} ` + metric.want
		if !strings.Contains(output.String(), line+"\n") {
			t.Fatalf("read metric %s = %q", metric.name, output.String())
		}
	}
	get := httptest.NewRecorder()
	metrics.ServeHTTP(get, httptest.NewRequest(http.MethodGet, "/metrics", nil))
	if get.Code != http.StatusOK || !bytes.Equal(get.Body.Bytes(), output.Bytes()) {
		t.Fatalf("prometheus response: status=%d body=%q", get.Code, get.Body.String())
	}
	post := httptest.NewRecorder()
	metrics.ServeHTTP(post, httptest.NewRequest(http.MethodPost, "/metrics", nil))
	if post.Code != http.StatusMethodNotAllowed {
		t.Fatalf("metric write method: %d", post.Code)
	}
	missing := httptest.NewRecorder()
	(*Metrics)(nil).ServeHTTP(missing, httptest.NewRequest(http.MethodGet, "/metrics", nil))
	if missing.Code != http.StatusServiceUnavailable {
		t.Fatalf("uninitialized metrics response: %d", missing.Code)
	}
}
