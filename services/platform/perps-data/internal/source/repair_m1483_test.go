package source_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Sidiora-Labs/Paxeer-X-Network/services/platform/perps-data/internal/decode"
	. "github.com/Sidiora-Labs/Paxeer-X-Network/services/platform/perps-data/internal/source"
)

type repair1483Request struct {
	Method string `json:"method"`
	ID     int64  `json:"id"`
}

var repair1483Secrets = []string{"sentinelUser1483", "sentinelPass1483", "sentinelPath1483", "sentinelQuery1483", "sentinelFragment1483"}

type repair1483Capture struct {
	mu     sync.Mutex
	buffer bytes.Buffer
}

func (c *repair1483Capture) Write(p []byte) (int, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.buffer.Write(p)
}
func (c *repair1483Capture) String() string {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.buffer.String()
}
func repair1483URL(t *testing.T, base string) string {
	t.Helper()
	u, err := url.Parse(base)
	if err != nil {
		t.Fatal(err)
	}
	u.User = url.UserPassword(repair1483Secrets[0], repair1483Secrets[1])
	u.Path = "/" + repair1483Secrets[2]
	u.RawQuery = "key=" + repair1483Secrets[3]
	u.Fragment = repair1483Secrets[4]
	return u.String()
}
func repair1483Private(t *testing.T, value string) {
	t.Helper()
	for _, secret := range repair1483Secrets {
		if strings.Contains(value, secret) {
			t.Errorf("endpoint secret %q escaped into diagnostics", secret)
		}
	}
}
func repair1483Contains(t *testing.T, value string, want ...string) {
	t.Helper()
	for _, part := range want {
		if !strings.Contains(value, part) {
			t.Errorf("diagnostic missing %q: %s", part, value)
		}
	}
}

func TestRepairM_1483_RPCURLCredentials(t *testing.T) {
	for _, format := range []string{"text", "json"} {
		t.Run(format, func(t *testing.T) {
			var capture repair1483Capture
			var handler slog.Handler = slog.NewTextHandler(&capture, nil)
			if format == "json" {
				handler = slog.NewJSONHandler(&capture, nil)
			}
			var mode atomic.Int32
			var requests atomic.Int32
			node := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				requests.Add(1)
				user, pass, ok := r.BasicAuth()
				if !ok || user != repair1483Secrets[0] || pass != repair1483Secrets[1] || r.URL.Path != "/"+repair1483Secrets[2] || r.URL.Query().Get("key") != repair1483Secrets[3] {
					t.Error("transport changed the authenticated endpoint")
				}
				var request repair1483Request
				if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
					t.Error(err)
					return
				}
				switch mode.Load() {
				case 1:
					http.Error(w, strings.Join(repair1483Secrets, "/"), http.StatusServiceUnavailable)
				case 2:
					_ = json.NewEncoder(w).Encode(map[string]any{"jsonrpc": "2.0", "id": request.ID, "error": map[string]any{"code": -32601, "message": strings.Join(repair1483Secrets, "/")}})
				case 3:
					conn, _, err := w.(http.Hijacker).Hijack()
					if err != nil {
						t.Error(err)
						return
					}
					_ = conn.Close()
				case 4:
					_ = json.NewEncoder(w).Encode(map[string]any{"jsonrpc": "2.0", "id": request.ID, "error": map[string]any{"code": -32000, "message": "too many results " + strings.Join(repair1483Secrets, "/")}})
				case 5:
					_ = json.NewEncoder(w).Encode(map[string]any{"jsonrpc": "2.0", "id": request.ID, "error": map[string]any{"code": -32000, "message": "missing trie node " + strings.Join(repair1483Secrets, "/")}})
				case 6:
					_ = json.NewEncoder(w).Encode(map[string]any{"jsonrpc": "2.0", "id": request.ID, "error": map[string]any{"code": 3, "message": strings.Join(repair1483Secrets, "/"), "data": "0x1234"}})
				default:
					_ = json.NewEncoder(w).Encode(map[string]any{"jsonrpc": "2.0", "id": request.ID, "result": "0x100"})
				}
			}))
			defer node.Close()
			pool, err := NewRPC(context.Background(), RPCOptions{URLs: []string{repair1483URL(t, node.URL)}, Logger: slog.New(handler), HealthInterval: time.Hour})
			if err != nil {
				t.Fatal(err)
			}
			defer pool.Close()
			if head, err := pool.Head(context.Background()); err != nil || head != 256 {
				t.Fatalf("authenticated head=%d err=%v", head, err)
			}
			for _, failure := range []struct {
				mode  int32
				class string
			}{{1, "http_503"}, {2, "rpc_-32601"}, {3, "transport"}} {
				mode.Store(failure.mode)
				_, err := pool.Head(context.Background())
				if err == nil {
					t.Fatal("failed endpoint returned success")
				}
				repair1483Private(t, err.Error())
				repair1483Contains(t, err.Error(), "endpoint 0", "eth_blockNumber", failure.class)
			}
			// The consumer receives the same safe error through the pool's real
			// HTTP transport, including failures carrying a net/url.Error.
			client := &http.Client{Transport: pool.Transport()}
			response, err := client.Post("http://pool.invalid", "application/json", strings.NewReader(`{"jsonrpc":"2.0","method":"eth_blockNumber","id":17,"params":[]}`))
			if err != nil {
				t.Fatal(err)
			}
			wire, err := io.ReadAll(response.Body)
			_ = response.Body.Close()
			if err != nil {
				t.Fatal(err)
			}
			repair1483Private(t, string(wire))
			repair1483Contains(t, string(wire), `"id":17`, "eth_blockNumber", "transport")
			for _, refusal := range []struct {
				mode   int32
				target error
			}{{4, ErrResponseSize}, {5, decode.ErrStatePruned}} {
				mode.Store(refusal.mode)
				before := Repair1483Failures(pool, 0)
				_, err := pool.Head(context.Background())
				if !errors.Is(err, refusal.target) {
					t.Fatalf("refusal identity lost: %v", err)
				}
				repair1483Private(t, err.Error())
				if got := Repair1483Failures(pool, 0); got != before {
					t.Fatal("request refusal penalized endpoint health")
				}
			}
			mode.Store(6)
			_, err = pool.Call(context.Background(), PerpsModule, []byte{0})
			var revert *decode.RevertError
			if !errors.As(err, &revert) || !bytes.Equal(revert.Data, []byte{0x12, 0x34}) {
				t.Fatalf("revert classification/data lost: %v", err)
			}
			repair1483Private(t, err.Error())
			mode.Store(0)
			if _, err := pool.Head(context.Background()); err != nil {
				t.Fatal(err)
			}
			if Repair1483Failures(pool, 0) != 0 {
				t.Fatal("successful request did not restore health")
			}
			if requests.Load() < 10 {
				t.Fatalf("producer/transport path received only %d requests", requests.Load())
			}
			logged := capture.String()
			repair1483Private(t, logged)
			repair1483Contains(t, logged, "node request failed", "endpoint", "eth_blockNumber", "failure_class", "http_503", "rpc_-32601", "transport")
		})
	}
}

func TestRepairM_1483_StartupAndHealthTransitions(t *testing.T) {
	var capture repair1483Capture
	var healthy atomic.Bool
	var high atomic.Bool
	node := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !healthy.Load() {
			http.Error(w, strings.Join(repair1483Secrets, "/"), http.StatusServiceUnavailable)
			return
		}
		head := "0x1"
		if high.Load() {
			head = "0x100"
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"jsonrpc": "2.0", "id": 1, "result": head})
	}))
	defer node.Close()
	peer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, `{"jsonrpc":"2.0","id":1,"result":"0x100"}`)
	}))
	defer peer.Close()
	pool, err := NewRPC(context.Background(), RPCOptions{URLs: []string{repair1483URL(t, node.URL), repair1483URL(t, peer.URL)}, Logger: slog.New(slog.NewJSONHandler(&capture, nil)), HealthInterval: time.Hour})
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	repair1483Contains(t, capture.String(), "node health check failed", "http_503", `"endpoint":0`, `"method":"eth_blockNumber"`)
	healthy.Store(true)
	Repair1483HealthCheck(pool, context.Background())
	repair1483Contains(t, capture.String(), "node quarantined as stale")
	high.Store(true)
	Repair1483HealthCheck(pool, context.Background())
	repair1483Contains(t, capture.String(), "node recovered")
	repair1483Private(t, capture.String())
	if got := Repair1483RankedIndices(pool); len(got) != 2 {
		t.Fatalf("recovered pool has %d healthy endpoints", len(got))
	}
}

func TestRepairM_1483_CometURLCredentials(t *testing.T) {
	var capture repair1483Capture
	logger := slog.New(slog.NewTextHandler(&capture, nil))
	bad := "http://" + repair1483Secrets[0] + ":" + repair1483Secrets[1] + "@bad%host/" + repair1483Secrets[2] + "?key=" + repair1483Secrets[3]
	if client, err := NewComet([]string{bad}, time.Second); err == nil || client != nil {
		t.Fatal("malformed comet endpoint admitted")
	} else {
		logger.Error("startup failed", slog.Any("err", err))
		repair1483Private(t, err.Error())
		repair1483Contains(t, err.Error(), "endpoint 0", "client_init")
	}
	node := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, _, err := w.(http.Hijacker).Hijack()
		if err != nil {
			t.Error(err)
			return
		}
		_ = conn.Close()
	}))
	defer node.Close()
	comet, err := NewComet([]string{repair1483URL(t, node.URL)}, time.Second)
	if err != nil {
		t.Fatal(err)
	}
	for _, call := range []struct {
		method string
		run    func() error
	}{
		{"header", func() error { _, err := comet.Header(context.Background(), 1); return err }},
		{"block_results", func() error { _, err := comet.BlockResults(context.Background(), 1); return err }},
	} {
		err := call.run()
		if err == nil {
			t.Fatal("broken comet connection returned success")
		}
		logger.Error("consensus request failed", slog.Any("err", fmt.Errorf("consumer: %w", err)))
		repair1483Private(t, err.Error())
		repair1483Contains(t, err.Error(), "endpoint 0", call.method, "transport")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := comet.Header(ctx, 1); !errors.Is(err, context.Canceled) {
		t.Fatalf("comet cancellation identity lost: %v", err)
	}
	repair1483Private(t, capture.String())
}

func TestRepairM_1483_RPCStartupFailure(t *testing.T) {
	var capture repair1483Capture
	bad := "http://" + repair1483Secrets[0] + ":" + repair1483Secrets[1] + "@bad%host/" + repair1483Secrets[2] + "?key=" + repair1483Secrets[3]
	pool, err := NewRPC(context.Background(), RPCOptions{URLs: []string{bad}, Logger: slog.New(slog.NewJSONHandler(&capture, nil)), HealthInterval: time.Hour})
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	if _, err := pool.Head(context.Background()); err == nil {
		t.Fatal("malformed RPC endpoint returned success")
	} else {
		repair1483Private(t, err.Error())
	}
	repair1483Contains(t, capture.String(), "node health check failed", `"endpoint":0`, "eth_blockNumber", "failure_class", "invalid_url")
	repair1483Private(t, capture.String())
}

func TestRepairM_1483_FollowerRecovery(t *testing.T) {
	repair1483FollowerRecovery(t)
}

func TestRepairM_1483_HeadDecodeCredentials(t *testing.T) {
	var capture repair1483Capture
	var malformed atomic.Bool
	var requests atomic.Int32
	node := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var request repair1483Request
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			t.Error(err)
			return
		}
		requests.Add(1)
		if request.Method != "eth_blockNumber" {
			t.Errorf("state read reached node without a proven head: %s", request.Method)
		}
		result := "0x1"
		if malformed.Load() {
			result = "0x" + strings.Join(repair1483Secrets, "/")
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"jsonrpc": "2.0", "id": request.ID, "result": result})
	}))
	defer node.Close()
	pool, err := NewRPC(context.Background(), RPCOptions{URLs: []string{repair1483URL(t, node.URL)}, Logger: slog.New(slog.NewJSONHandler(&capture, nil)), HealthInterval: time.Hour})
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	malformed.Store(true)
	for _, call := range []func() error{
		func() error { _, err := pool.Head(context.Background()); return err },
		func() error { _, err := pool.CallAt(context.Background(), PerpsModule, []byte{0}, 2); return err },
	} {
		err := call()
		if err == nil {
			t.Fatal("malformed upstream head returned success")
		}
		repair1483Private(t, err.Error())
		repair1483Contains(t, err.Error(), "endpoint 0", "eth_blockNumber", "request")
	}
	Repair1483HealthCheck(pool, context.Background())
	if requests.Load() != 4 {
		t.Fatalf("head producer received %d requests, want 4", requests.Load())
	}
	repair1483Private(t, capture.String())
	repair1483Contains(t, capture.String(), "node request failed", "node health check failed", `"endpoint":0`, `"method":"eth_blockNumber"`, `"failure_class":"request"`)
}
