package source

import (
	"context"
	"errors"
	"fmt"
	"github.com/ethereum/go-ethereum/common"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestHistoricalHTTPBackoffAndResponseBoundaries(t *testing.T) {
	for _, tc := range []struct {
		name      string
		status    int
		retry     string
		timeout   time.Duration
		body      string
		truncated bool
		want      ErrorKind
	}{
		{name: "gateway", status: 502, want: ThrottleError},
		{name: "gateway cancellation", status: 503, timeout: 30 * time.Millisecond, want: TimeoutError},
		{name: "rate date", status: 429, retry: time.Now().Add(-time.Hour).UTC().Format(http.TimeFormat), want: ThrottleError},
		{name: "rate capped cancellation", status: 429, retry: "100", timeout: 30 * time.Millisecond, want: TimeoutError},
		{name: "truncated body", status: 200, truncated: true, want: TimeoutError},
		{name: "result shape", status: 200, body: `{"jsonrpc":"2.0","id":1,"result":{}}`, want: InvalidError},
	} {
		t.Run(tc.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if tc.retry != "" {
					w.Header().Set("Retry-After", tc.retry)
				}
				if tc.truncated {
					w.Header().Set("Content-Length", "100")
				}
				w.WriteHeader(tc.status)
				if tc.body != "" {
					if _, err := fmt.Fprint(w, tc.body); err != nil {
						t.Error(err)
					}
				} else if tc.truncated {
					if _, err := fmt.Fprint(w, "{"); err != nil {
						t.Error(err)
					}
				}
			}))
			defer server.Close()
			s := &CommittedSource{opts: CommittedOptions{MaxResponseBytes: 1024, LiveDeadline: time.Second, BulkDeadline: time.Second}, client: server.Client()}
			endpoint := &committedEndpoint{id: "archive", url: server.URL, historical: true, permits: make(chan struct{}, 1)}
			timeout := time.Second
			if tc.timeout != 0 {
				timeout = tc.timeout
			}
			ctx, cancel := context.WithTimeout(context.Background(), timeout)
			defer cancel()
			var result string
			err := s.rpcOnce(ctx, endpoint, false, "eth_chainId", nil, &result, 0)
			var typed *SourceError
			if !errors.As(err, &typed) || typed.Kind != tc.want {
				t.Fatalf("response admission %v", err)
			}
			if s.transport.inFlight.Load() != 0 || len(endpoint.permits) != 0 {
				t.Fatal("transport retained active request")
			}
			if tc.want == ThrottleError && typed.RetryAfter <= 0 {
				t.Fatal("backoff absent")
			}
		})
	}
	server := fixture(t, "0x7d")
	s := repair997Open(t, fopts(server.URL))
	endpoint := s.endpoints[0]
	endpoint.url = "http://invalid\nurl"
	var result string
	err := s.rpcOnce(context.Background(), endpoint, false, "eth_chainId", nil, &result, 0)
	repair997Kind(t, err, InvalidError)
}

func TestRejoinPrunedAnchorRequiresIndependentVerifiedPeer(t *testing.T) {
	ctx := context.Background()
	primary, archive := repair997Serve(t), repair997Serve(t)
	primary.pruned.Store(true)
	opts := fopts(primary.server.URL)
	opts.GenesisHash = [32]byte{}
	opts.Anchor = &CommittedAnchor{Height: 1, Hash: committedFixtureHash(1)}
	opts.HistoricalEndpoints = map[string]string{"archive": archive.server.URL}
	s := repair997Open(t, opts)
	// Recent accepted identities allow recovery while the old anchor is pruned.
	primary.pruned.Store(false)
	result, err := s.FetchCommitted(ctx, FetchOptions{FromBlock: 256, ToBlock: 383}, false, nil)
	if err != nil || !result.Complete {
		t.Fatalf("recent coverage %v", err)
	}
	primary.pruned.Store(true)
	var endpoint *committedEndpoint
	s.mu.Lock()
	for _, e := range s.endpoints {
		if !e.historical {
			endpoint = e
			e.quarantined = true
			e.health.QuarantineReason = string(TimeoutError)
		}
	}
	s.mu.Unlock()
	if endpoint == nil {
		t.Fatal("primary absent")
	}
	if err := s.RejoinEndpoint(ctx, endpoint.id); err != nil {
		t.Fatal(err)
	}
	if health := s.EndpointHealth(); len(health) != 2 {
		t.Fatalf("endpoint inventory %+v", health)
	}
	s.mu.Lock()
	endpoint.quarantined = true
	s.accepted = map[uint64]common.Hash{}
	s.mu.Unlock()
	if err := s.RejoinEndpoint(ctx, endpoint.id); err == nil {
		t.Fatal("recovery without recent identities admitted")
	}
}
