package receipt

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"net/http/httputil"
	"net/url"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/common"
)

func TestArchiveTransportRestoresRecordedQualificationAndLearnsRouting(t *testing.T) {
	input := repair53InputsFromProducer(t)
	recorded, err := url.Parse(input.Endpoint)
	if err != nil {
		t.Fatal(err)
	}
	proxy := httputil.NewSingleHostReverseProxy(recorded)
	var archived, primary atomic.Int64
	archive := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if archived.Add(1) == 1 {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		proxy.ServeHTTP(w, r)
	}))
	defer archive.Close()
	live := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		primary.Add(1)
		var request struct{ ID json.RawMessage }
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			t.Error(err)
			w.WriteHeader(400)
			return
		}
		if err := json.NewEncoder(w).Encode(map[string]any{"jsonrpc": "2.0", "id": request.ID, "error": map[string]any{"code": -32000, "message": "height pruned; earliest available is 999999999"}}); err != nil {
			t.Error(err)
		}
	}))
	defer live.Close()
	retries, overloads := ArchiveTransportCounters()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	client, closeClient, err := NewBoundedRPCClientWithArchive(ctx, live.URL, archive.URL, ArchiveOptions{Concurrency: 2, AttemptTimeout: time.Second})
	if err != nil {
		t.Fatal(err)
	}
	defer closeClient()
	authority, err := NewRPCQualification(ctx, client, input.Options)
	if err != nil {
		t.Fatal(err)
	}
	if err = authority.Rehydrate(ctx, input.Markets); err != nil {
		t.Fatal(err)
	}
	authority = authority.current()
	bundle, err := authority.Bundle(ctx, input.NativeSwap)
	if err != nil || bundle.Transaction.Hash() != input.NativeSwap {
		t.Fatalf("archived bundle: %+v %v", bundle, err)
	}
	if err := authority.ValidateReceipt(ctx, bundle); err != nil {
		t.Fatal(err)
	}
	before := primary.Load()
	if _, _, err := authority.headerByNumber(ctx, bundle.Header.Number); err != nil {
		t.Fatal(err)
	}
	if _, err := authority.headerByHash(ctx, bundle.BlockHash); err != nil {
		t.Fatal(err)
	}
	if primary.Load() != before || archived.Load() < 3 {
		t.Fatalf("archive routing primary=%d/%d archive=%d", before, primary.Load(), archived.Load())
	}
	nextRetries, nextOverloads := ArchiveTransportCounters()
	if nextRetries <= retries || nextOverloads <= overloads {
		t.Fatal("archive overload and retry were not counted")
	}
	closeClient()
	if _, err := client.ChainID(ctx); err == nil {
		t.Fatal("closed transport continued issuing calls")
	}
}

func TestTransportRefusesUnboundedRequestsAndResponses(t *testing.T) {
	ctx := context.Background()
	for _, endpoint := range []string{"", "ws://localhost", "http://", "http://localhost/#fragment", ":"} {
		if _, _, err := NewBoundedRPCClient(ctx, endpoint, ""); !errors.Is(err, ErrQualification) {
			t.Fatalf("invalid endpoint %q: %v", endpoint, err)
		}
		if _, _, err := NewBoundedRPCClient(ctx, "http://localhost", endpoint+"#fragment"); !errors.Is(err, ErrQualification) {
			t.Fatalf("invalid archive %q: %v", endpoint, err)
		}
	}
	transport := &boundedRPCTransport{lifetime: ctx, calls: make(chan struct{}, 1), upstream: http.DefaultTransport.(*http.Transport).Clone()}
	defer transport.upstream.CloseIdleConnections()
	for _, tc := range []struct{ method, body string }{
		{http.MethodGet, `{}`}, {http.MethodPost, `{`}, {http.MethodPost, strings.Repeat("x", 4097)},
		{http.MethodPost, `{"method":"eth_sendRawTransaction"}`},
		{http.MethodPost, `{"method":"eth_getBlockByNumber","params":[]}`},
		{http.MethodPost, `{"method":"eth_getBlockByNumber","params":["latest","true"]}`},
	} {
		request, err := http.NewRequestWithContext(ctx, tc.method, "http://localhost", strings.NewReader(tc.body))
		if err != nil {
			t.Fatal(err)
		}
		if _, err = transport.RoundTrip(request); !errors.Is(err, ErrQualification) {
			t.Fatalf("unbounded request accepted %s: %v", tc.body, err)
		}
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if _, err := io.WriteString(w, strings.Repeat("x", rpcScalarResponseBytes+1)); err != nil {
			t.Error(err)
		}
	}))
	defer server.Close()
	client, closeClient, err := NewBoundedRPCClient(ctx, server.URL, "")
	if err != nil {
		t.Fatal(err)
	}
	defer closeClient()
	if _, err = client.ChainID(ctx); !errors.Is(err, ErrQualification) {
		t.Fatalf("oversized scalar accepted %v", err)
	}
}

func TestArchiveRoutingBindsFloorAndPinnedHash(t *testing.T) {
	transport := new(boundedRPCTransport)
	transport.learnFloor([]byte(`{"error":{"message":"height pruned; earliest available is 100"}}`))
	transport.learnFloor([]byte(`{"error":{"message":"height pruned; earliest available is 10"}}`))
	hash := common.HexToHash("0xabc").Hex()
	transport.learn(valueEthGetBlockByNumber, []byte(`{"result":{"hash":"`+hash+`"}}`))
	for _, tc := range []struct {
		method, params string
		want           bool
	}{
		{valueEthGetBlockByNumber, `["0x63",true]`, true}, {valueEthGetBlockByNumber, `["0x64",true]`, false},
		{valueEthGetBlockByNumber, `["latest",true]`, false}, {valueEthGetBlockByNumber, `[1,true]`, false},
		{valueEthGetBlockByHash, `["` + hash + `",true]`, true}, {valueEthGetBlockReceipts, `["` + hash + `"]`, true},
		{valueEthGetCode, `["0x1",{"blockHash":"` + hash + `"}]`, true},
		{valueEthGetStorageAt, `["0x1","0x0",{"blockHash":"` + hash + `"}]`, true},
		{valueEthCall, `[{},"latest"]`, false}, {"eth_chainId", `[]`, false},
	} {
		var params []json.RawMessage
		if err := json.Unmarshal([]byte(tc.params), &params); err != nil {
			t.Fatal(err)
		}
		if got := transport.archived(tc.method, params); got != tc.want {
			t.Fatalf("routing %s %s: %v", tc.method, tc.params, got)
		}
	}
	for _, tc := range []struct {
		method, body string
		want         bool
	}{
		{valueEthGetBlockByNumber, `{"result":null}`, false}, {valueEthGetBlockByHash, `{"result":null}`, true},
		{valueEthGetCode, `{"error":{"message":"not found"}}`, true}, {valueEthCall, `{"error":{"message":"execution reverted"}}`, false},
		{valueEthGetBlockByNumber, `{"error":{"message":"pruned"}}`, true}, {valueEthGetBlockByHash, `{`, false},
	} {
		if got := historicalRetry(tc.method, []byte(tc.body)); got != tc.want {
			t.Fatalf("historical retry %s %s: %v", tc.method, tc.body, got)
		}
	}
}
