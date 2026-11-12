package receipt

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"
)

func exerciseExecutionPolicyStateFaults(t *testing.T, n *privateNodeFixture, a *RPCQualification, b Bundle, index uint32) {
	t.Helper()
	var mode atomic.Value
	mode.Store("")
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var q struct {
			ID     json.RawMessage   `json:"id"`
			Method string            `json:"method"`
			Params []json.RawMessage `json:"params"`
		}
		if err := json.NewDecoder(r.Body).Decode(&q); err != nil {
			t.Error(err)
			w.WriteHeader(400)
			return
		}
		arguments := make([]any, len(q.Params))
		for i, p := range q.Params {
			arguments[i] = p
		}
		var result json.RawMessage
		err := n.client.Client().CallContext(r.Context(), &result, q.Method, arguments...)
		current := mode.Load().(string)
		failure := false
		if current == "parent unavailable" && q.Method == "eth_getBlockByHash" && len(q.Params) > 0 && strings.Contains(strings.ToLower(string(q.Params[0])), strings.ToLower(b.Header.ParentHash.Hex())) {
			failure = true
		}
		if q.Method == "eth_call" && len(q.Params) > 1 && strings.Contains(strings.ToLower(string(q.Params[1])), strings.ToLower(b.Header.ParentHash.Hex())) {
			var call struct {
				Data  string `json:"data"`
				Input string `json:"input"`
			}
			if e := json.Unmarshal(q.Params[0], &call); e != nil {
				t.Error(e)
				w.WriteHeader(400)
				return
			}
			data := call.Data
			if data == "" {
				data = call.Input
			}
			matches := func(signature string) bool {
				return strings.HasPrefix(strings.ToLower(data), "0x"+common.Bytes2Hex(crypto.Keccak256([]byte(signature))[:4]))
			}
			switch {
			case current == "metadata unavailable" && matches("tokenAddress()"), current == "optical unavailable" && matches("opticalAddress()"), current == "fee unavailable" && matches("baseFeeBps()"):
				failure = true
			case current == "optical enabled" && matches("opticalAddress()"):
				result = json.RawMessage(`"0x` + common.Bytes2Hex(common.LeftPadBytes(n.owner.Bytes(), 32)) + `"`)
			case current == "base too wide" && matches("baseFeeBps()"), current == "minimum too wide" && matches("minFeeBps()"), current == "maximum too wide" && matches("maxFeeBps()"):
				result = json.RawMessage(fmt.Sprintf(`"0x%064x"`, 65536))
			case current == "adaptive fee" && matches("feeDecayRate()"):
				result = json.RawMessage(fmt.Sprintf(`"0x%064x"`, 1))
			}
		}
		response := map[string]any{"jsonrpc": "2.0", "id": q.ID}
		if failure {
			response["error"] = map[string]any{"code": -32000, "message": "parent state unavailable"}
		} else if err != nil {
			response["error"] = map[string]any{"code": -32000, "message": err.Error()}
		} else {
			response["result"] = result
		}
		if err := json.NewEncoder(w).Encode(response); err != nil {
			t.Error(err)
		}
	}))
	defer server.Close()
	client, err := ethclient.DialContext(n.ctx, server.URL)
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()
	original := a.current()
	qualified := &RPCQualification{client: client, options: original.options, registry: original.registry, manifestHash: original.manifestHash, privateFixture: original.privateFixture, identityVerified: original.identityVerified, reads: original.reads}
	policy, err := qualified.ExecutionPolicy(n.ctx, b, index)
	if err != nil || policy.Quality != "qualified_execution" {
		t.Fatalf("original execution policy: %+v %v", policy, err)
	}
	for _, tc := range []struct {
		name   string
		reject bool
	}{
		{"parent unavailable", true}, {"metadata unavailable", true}, {"optical unavailable", true}, {"optical enabled", false}, {"fee unavailable", true}, {"base too wide", true}, {"minimum too wide", true}, {"maximum too wide", true}, {"adaptive fee", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			mode.Store(tc.name)
			result, err := qualified.ExecutionPolicy(n.ctx, b, index)
			if (err != nil) != tc.reject || result.Quality != "unavailable" || result.FeeBPS != nil || result.ConfigDigest != nil {
				t.Fatalf("unbound execution policy: %+v %v", result, err)
			}
		})
	}
}
