package readpolicy

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestResponseTransportBoundsRealHTTPBodies(t *testing.T) {
	for _, size := range []int64{0, MaxResponseBytes, MaxResponseBytes + 1} {
		t.Run(string(rune('a'+size%10)), func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				_, _ = io.CopyN(w, strings.NewReader(strings.Repeat("x", int(size))), size)
			}))
			defer server.Close()
			req, err := http.NewRequest("GET", server.URL, nil)
			if err != nil {
				t.Fatal(err)
			}
			response, err := (ResponseTransport{}).RoundTrip(req)
			if err != nil {
				t.Fatal(err)
			}
			body, err := io.ReadAll(response.Body)
			if closeErr := response.Body.Close(); closeErr != nil {
				t.Fatal(closeErr)
			}
			if size <= MaxResponseBytes {
				if err != nil || int64(len(body)) != size {
					t.Fatalf("bounded body: %d %v", len(body), err)
				}
			} else if err == nil || int64(len(body)) != MaxResponseBytes {
				t.Fatalf("body overflow: %d %v", len(body), err)
			}
			cancelled, cancel := context.WithCancel(context.Background())
			cancel()
			if _, err := (ResponseTransport{}).RoundTrip(req.WithContext(cancelled)); !errors.Is(err, context.Canceled) {
				t.Fatalf("cancelled transport: %v", err)
			}
		})
	}
}

func TestCoalescerRejectsInvalidGroupsAndExpiredResults(t *testing.T) {
	read := func(context.Context) ([][]byte, error) { return [][]byte{[]byte("value")}, nil }
	for _, tc := range []struct {
		ctx  context.Context
		key  string
		read func(context.Context) ([][]byte, error)
	}{{nil, "key", read}, {context.Background(), "", read}, {context.Background(), strings.Repeat("x", 257), read}, {context.Background(), "key", nil}} {
		if _, err := NewCoalescer().Do(tc.ctx, tc.key, tc.read); err == nil {
			t.Fatal("invalid read group admitted")
		}
	}
	cancelled, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := NewCoalescer().Do(cancelled, "key", read); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
	c := NewCoalescer()
	c.cache["expired"] = cachedRead{value: [][]byte{[]byte("old")}, expires: time.Now().Add(-time.Second), bytes: 3}
	c.cacheBytes = 3
	value, err := c.Do(context.Background(), "expired", read)
	if err != nil || string(value[0]) != "value" || c.cacheBytes != 5 {
		t.Fatalf("expired read replacement: %q %d %v", value, c.cacheBytes, err)
	}
	for name, values := range map[string][][]byte{"bytes": {make([]byte, MaxResponseBytes+1)}, "batch": make([][]byte, MaxBatch+1)} {
		if _, err := c.Do(context.Background(), name, func(context.Context) ([][]byte, error) { return values, nil }); err == nil {
			t.Fatal("unbounded result admitted")
		}
	}
	c.mu.Lock()
	for i := 0; i < 128; i++ {
		c.groups[string(rune(i))] = &readGroup{}
	}
	c.mu.Unlock()
	if _, err := c.Do(context.Background(), "overflow", read); !errors.Is(err, ErrReadCapacity) {
		t.Fatal(err)
	}
	if c.Stats().CapacityRejections != 1 {
		t.Fatal("read capacity rejection not counted")
	}
}

func TestMeasurementStorageBoundsAndFinalization(t *testing.T) {
	binding := measuredTestBinding(t)
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "occupied"), []byte("occupied"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := NewMeasurementCapture(filepath.Join(root, "occupied", "capture"), binding, 1000, 1<<20); err == nil {
		t.Fatal("non-directory destination admitted")
	}
	for _, mode := range []string{"duplicate", "byte budget", "closed writer", "finalized output"} {
		t.Run(mode, func(t *testing.T) {
			dir := filepath.Join(t.TempDir(), "capture")
			capture, err := NewMeasurementCapture(dir, binding, 1000, 1<<20)
			if err != nil {
				t.Fatal(err)
			}
			sample := MeasurementSample{RequestID: "one", Scenario: "read", ChainID: "125", ObservedUTCNS: 1, FinishedUTCNS: 2, ElapsedNS: 1, Outcome: "success"}
			switch mode {
			case "duplicate":
				if err := capture.Append(sample); err != nil {
					t.Fatal(err)
				}
			case "byte budget":
				capture.maxBytes = 1
			case "closed writer":
				if err := capture.file.Close(); err != nil {
					t.Fatal(err)
				}
			case "finalized output":
				if err := capture.Append(sample); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(filepath.Join(dir, "measurement-result.json"), []byte("existing"), 0600); err != nil {
					t.Fatal(err)
				}
			}
			if mode != "finalized output" {
				if err := capture.Append(sample); err == nil {
					t.Fatal("invalid capture append admitted")
				}
			}
			result, err := capture.Finish(true, "")
			if mode == "closed writer" || mode == "finalized output" {
				if err == nil {
					t.Fatal("incomplete storage acknowledged")
				}
			} else if err != nil || result.LaneHarnessPassed {
				t.Fatalf("invalid capture qualified: %+v %v", result, err)
			}
			if _, err := capture.Finish(true, ""); err == nil {
				t.Fatal("duplicate finalization admitted")
			}
		})
	}
}
