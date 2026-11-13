package httpapi

import (
	"bytes"
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestPoolMetadataClientRejectsUnverifiedUpstreamResponses(t *testing.T) {
	for _, tc := range []struct {
		name, want string
		status     int
		body       []byte
	}{
		{"unknown pool", "unknown verified pool", http.StatusNotFound, nil},
		{"unavailable upstream", "upstream unavailable", http.StatusServiceUnavailable, nil},
		{"invalid frozen response", "invalid frozen chart upstream response", http.StatusOK, []byte(`{"pool":"unbound"}`)},
		{"oversized response", "response too large", http.StatusOK, bytes.Repeat([]byte("x"), 16*1024*1024+1)},
		{"redirect", "redirect refused", http.StatusFound, nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if !strings.HasSuffix(r.URL.Path, "/v2/pools/"+udfPool) {
					t.Errorf("unbound metadata route: %s", r.URL.Path)
				}
				if tc.status == http.StatusFound {
					w.Header().Set("Location", "/another-pool")
				}
				w.WriteHeader(tc.status)
				if _, err := w.Write(tc.body); err != nil {
					t.Error(err)
				}
			}))
			defer server.Close()
			resolved, err := (PoolMetadataClient{Endpoint: server.URL}).Resolve(context.Background(), udfPool)
			if err == nil || !strings.Contains(err.Error(), tc.want) || resolved.Data.Pool.PoolAddress != "" {
				t.Fatalf("unverified metadata accepted: %+v, %v", resolved, err)
			}
			if tc.status == http.StatusNotFound && !errors.Is(err, ErrUnknownPool) {
				t.Fatalf("unknown pool classification: %v", err)
			}
		})
	}
	var response map[string]any
	if err := boundedChartJSON(context.Background(), nil, "http://invalid.local/\n", &response, "PoolDetailResponse"); err == nil {
		t.Fatal("malformed upstream URL accepted")
	}
}
