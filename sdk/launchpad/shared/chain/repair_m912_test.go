package chain

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/http/httputil"
	"net/url"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/rpc"
)

func repairM912ObservedPeer(t *testing.T, peer *httptest.Server) (string, func()) {
	t.Helper()
	target, err := url.Parse(peer.URL)
	if err != nil {
		t.Fatal(err)
	}
	proxy := httputil.NewSingleHostReverseProxy(target)
	var mu sync.Mutex
	var ids []uint64
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Error(err)
			http.Error(w, "read request", http.StatusBadRequest)
			return
		}
		var request struct {
			ID json.RawMessage `json:"id"`
		}
		if err := json.Unmarshal(body, &request); err != nil {
			t.Error(err)
			http.Error(w, "decode request", http.StatusBadRequest)
			return
		}
		id, err := strconv.ParseUint(string(request.ID), 10, 64)
		if err != nil {
			t.Error(err)
			http.Error(w, "decode request ID", http.StatusBadRequest)
			return
		}
		mu.Lock()
		ids = append(ids, id)
		mu.Unlock()
		r.Body = io.NopCloser(bytes.NewReader(body))
		proxy.ServeHTTP(w, r)
	}))
	t.Cleanup(server.Close)
	return server.URL, func() {
		t.Helper()
		mu.Lock()
		defer mu.Unlock()
		if len(ids) < 2 {
			t.Fatalf("observed request IDs = %v, want consecutive requests", ids)
		}
		for i, id := range ids {
			if id == 0 || (i > 0 && id != ids[i-1]+1) {
				t.Fatalf("request identities are not consecutive: %v", ids)
			}
		}
	}
}

func TestRepairM_912_RPCFixtureCorrelation(t *testing.T) {
	for _, failPrimary := range []bool{false, true} {
		t.Run(strconv.FormatBool(failPrimary), func(t *testing.T) {
			results := map[string]string{"eth_chainId": "0x7d", "eth_blockNumber": "0x1a4"}
			primary := rpcServer(t, results, failPrimary)
			primaryURL, checkPrimary := repairM912ObservedPeer(t, primary)
			fallbackURL := ""
			var checkFallback func()
			if failPrimary {
				fallback := rpcServer(t, results, false)
				fallbackURL, checkFallback = repairM912ObservedPeer(t, fallback)
			}
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			client, err := NewClient(ctx, primaryURL, fallbackURL)
			if err != nil {
				t.Fatal(err)
			}
			defer client.Close()
			for i := 0; i < 2; i++ {
				id, err := client.ChainID(ctx)
				if err != nil || id == nil || id.Int64() != 125 {
					t.Fatalf("ChainID request %d = %v, error %v", i, id, err)
				}
				height, err := client.BlockNumber(ctx)
				if err != nil || height != 420 {
					t.Fatalf("BlockNumber request %d = %d, error %v", i, height, err)
				}
			}
			checkPrimary()
			if checkFallback != nil {
				checkFallback()
			}
		})
	}
	t.Run("method-error-and-recovery", func(t *testing.T) {
		peer := rpcServer(t, map[string]string{"eth_chainId": "0x7d"}, false)
		peerURL, checkIDs := repairM912ObservedPeer(t, peer)
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		client, err := NewClient(ctx, peerURL, "")
		if err != nil {
			t.Fatal(err)
		}
		defer client.Close()
		for i := 0; i < 2; i++ {
			var result string
			err := client.Eth().Client().CallContext(ctx, &result, "missing_method", "eth_chainId")
			rpcErr, ok := err.(rpc.Error)
			if !ok || rpcErr.ErrorCode() != -32601 || err.Error() != "method not found" || result != "" {
				t.Fatalf("unknown method = %q, error %v; want correlated -32601", result, err)
			}
			id, err := client.ChainID(ctx)
			if err != nil || id == nil || id.Int64() != 125 {
				t.Fatalf("success after method error = %v, error %v", id, err)
			}
		}
		checkIDs()
	})
	t.Run("existing-success", TestChainIDAndBlockNumber)
	t.Run("existing-failover", TestFailoverToSecondary)
	t.Run("existing-wrong-chain", TestWrongChainCannotServe)
}

func TestRepairM_912_RPCFixtureIDTypes(t *testing.T) {
	peer := rpcServer(t, map[string]string{"eth_chainId": "0x7d"}, false)
	client := &http.Client{Timeout: 5 * time.Second}
	for _, id := range []json.RawMessage{json.RawMessage(`41`), json.RawMessage(`42`), json.RawMessage(`9007199254740993`), json.RawMessage(`"request-43"`)} {
		for _, method := range []string{"eth_chainId", "missing_method"} {
			body, err := json.Marshal(map[string]any{"jsonrpc": "2.0", "id": id, "method": method, "params": []string{"eth_chainId"}})
			if err != nil {
				t.Fatal(err)
			}
			response, err := client.Post(peer.URL, "application/json", bytes.NewReader(body))
			if err != nil {
				t.Fatal(err)
			}
			var got struct {
				JSONRPC string          `json:"jsonrpc"`
				ID      json.RawMessage `json:"id"`
				Result  string          `json:"result"`
				Error   *struct {
					Code    int    `json:"code"`
					Message string `json:"message"`
				} `json:"error"`
			}
			err = json.NewDecoder(response.Body).Decode(&got)
			response.Body.Close()
			if err != nil || response.StatusCode != http.StatusOK || got.JSONRPC != "2.0" || !bytes.Equal(got.ID, id) {
				t.Fatalf("%s ID %s: response %+v, status %d, error %v", method, id, got, response.StatusCode, err)
			}
			if method == "eth_chainId" {
				if got.Result != "0x7d" || got.Error != nil {
					t.Fatalf("success response = %+v", got)
				}
			} else if got.Result != "" || got.Error == nil || got.Error.Code != -32601 || got.Error.Message != "method not found" {
				t.Fatalf("method error response = %+v", got)
			}
		}
	}
}

func TestRepairM_912_ContractFixtureCorrelation(t *testing.T) {
	t.Run("beacon-routing", TestCreateContractsRouting)
	t.Run("precompile-filter-and-separate-beacon", TestRolesBindToPrecompile)
}
