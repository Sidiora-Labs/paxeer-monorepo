package source

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"reflect"
	"sync/atomic"
	"testing"
	"time"
)

func TestEVMHealthRankingUsesFailuresLatencyAndStableTies(t *testing.T) {
	ctx := context.Background()
	node := jsonRPC(t, 100, nil, nil)
	s, err := NewEVM(ctx, EVMOptions{RPCURLs: []string{node.URL, node.URL, node.URL, node.URL, node.URL}, Timeout: time.Second})
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	// Independent observations retain their measured latency when failures clear.
	s.mu.Lock()
	for _, slot := range s.slots {
		slot.recentLatencyMs = 0
		slot.consecutiveFailures = 0
	}
	s.mu.Unlock()
	s.markSuccess(0, 30)
	s.markSuccess(1, 10)
	s.markSuccess(2, 10)
	s.markFailure(3, errors.New("connection closed"))
	if got := s.rankedIndices(); !reflect.DeepEqual(got, []int{1, 2, 0, 4, 3}) {
		t.Fatalf("endpoint ranking %v", got)
	}
	s.mu.Lock()
	s.slots[4].isStale = true
	s.mu.Unlock()
	if got := s.rankedIndices(); !reflect.DeepEqual(got, []int{1, 2, 0, 3}) {
		t.Fatalf("quarantined endpoint ranking %v", got)
	}
	s.runHealthCheck(ctx)
	s.mu.Lock()
	recovered := !s.slots[4].isStale && s.slots[3].consecutiveFailures == 0 && s.slots[0].lastBlock == 100
	s.mu.Unlock()
	if !recovered {
		t.Fatal("observed healthy endpoints did not recover")
	}
}

func TestEVMResponseAdmissionRejectsMalformedHeadsLogsAndBlocks(t *testing.T) {
	ctx := context.Background()
	var mode atomic.Value
	mode.Store("head shape")
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var call rpcReq
		if e := json.NewDecoder(r.Body).Decode(&call); e != nil {
			t.Error(e)
			return
		}
		result := json.RawMessage(`"0x64"`)
		switch call.Method {
		case "eth_blockNumber":
			if mode.Load().(string) == "head shape" {
				result = json.RawMessage(`{}`)
			} else if mode.Load().(string) == "head value" {
				result = json.RawMessage(`"zz"`)
			}
		case "eth_getLogs":
			result = json.RawMessage(`{}`)
		case "eth_getBlockByNumber":
			if mode.Load().(string) == "null block" {
				result = json.RawMessage(`null`)
			} else {
				result = json.RawMessage(`{"timestamp":true}`)
			}
		}
		if e := json.NewEncoder(w).Encode(map[string]any{"jsonrpc": "2.0", "id": call.ID, "result": result}); e != nil {
			t.Error(e)
		}
	}))
	defer server.Close()
	s, err := NewEVM(ctx, EVMOptions{RPCURLs: []string{server.URL}, Timeout: time.Second, ReceiptConcurrency: 1})
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	for _, name := range []string{"head shape", "head value", "log shape", "null block", "block shape"} {
		t.Run(name, func(t *testing.T) {
			mode.Store(name)
			var e error
			switch name {
			case "head shape", "head value":
				_, e = s.Head(ctx)
			case "log shape":
				_, e = s.FetchLogs(ctx, FetchOptions{FromBlock: 1, ToBlock: 1})
			default:
				_, e = s.getBlock(ctx, 1)
			}
			if e == nil {
				t.Fatal("malformed endpoint response admitted")
			}
		})
	}
	if _, err = s.FetchLogs(ctx, FetchOptions{FromBlock: -1, ToBlock: 1}); err == nil {
		t.Fatal("negative source range admitted")
	}
	s.runHealthCheck(ctx)
	mode.Store("head shape")
	s.runHealthCheck(ctx)
	s.mu.Lock()
	for _, slot := range s.slots {
		slot.isStale = true
	}
	s.mu.Unlock()
	if _, err = s.Head(ctx); err == nil {
		t.Fatal("head selected a quarantined endpoint")
	}
	if _, err = NewEVM(ctx, EVMOptions{RPCURLs: []string{server.URL}, ReceiptConcurrency: 129}); err == nil {
		t.Fatal("unbounded receipt concurrency admitted")
	}
	s.Close()
	if _, err = s.FetchLogs(ctx, FetchOptions{FromBlock: 1, ToBlock: 1}); err == nil {
		t.Fatal("closed source accepted range")
	}
	if err = s.Close(); err != nil {
		t.Fatal(err)
	}
}
