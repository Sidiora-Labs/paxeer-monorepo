package chain

import (
	"context"
	"encoding/json"
	"errors"
	"math/big"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/Sidiora-Technologies/KindleLaunch/shared/chain/profile"
	"github.com/ethereum/go-ethereum/core/types"
)

func TestPinnedReadRejectsIdentityChangesAndCancellation(t *testing.T) {
	for _, scenario := range []string{"before unavailable", "before changed", "during cancellation", "after unavailable", "after changed"} {
		t.Run(scenario, func(t *testing.T) {
			header := testHeader(100)
			changed := types.CopyHeader(header)
			changed.Extra = []byte{9}
			var state atomic.Int64
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				var req struct {
					ID     json.RawMessage
					Method string
				}
				if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
					t.Error(err)
					return
				}
				var result any = "0x7d"
				if req.Method == "eth_getBlockByNumber" {
					switch state.Load() {
					case 1:
						result = nil
					case 2:
						result = reportedJSON(changed)
					default:
						result = reportedJSON(header)
					}
				}
				if err := json.NewEncoder(w).Encode(map[string]any{"jsonrpc": "2.0", "id": req.ID, "result": result}); err != nil {
					t.Error(err)
				}
			}))
			defer server.Close()
			client, err := NewVerifiedClient(context.Background(), server.URL, "", VerifiedConfig{profile.Paxeer("e1", "d1"), 100, consensusHash(header)})
			if err != nil {
				t.Fatal(err)
			}
			defer client.Close()
			pin, err := client.Pin(context.Background(), 100)
			if err != nil {
				t.Fatal(err)
			}
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			if scenario == "before unavailable" {
				state.Store(1)
			}
			if scenario == "before changed" {
				state.Store(2)
			}
			reads := 0
			err = client.atPinned(ctx, client.primary, pin, func(height *big.Int, observed *types.Header) error {
				reads++
				if height.Uint64() != 100 || observed.Number.Uint64() != 100 {
					t.Fatal("read left pinned height")
				}
				switch scenario {
				case "during cancellation":
					cancel()
				case "after unavailable":
					state.Store(1)
				case "after changed":
					state.Store(2)
				}
				return nil
			})
			wantReads := 1
			if scenario == "before unavailable" || scenario == "before changed" {
				wantReads = 0
			}
			if err == nil || reads != wantReads {
				t.Fatalf("pinned read admission: reads=%d error=%v", reads, err)
			}
			if scenario == "during cancellation" && !errors.Is(err, context.Canceled) {
				t.Fatalf("pinned cancellation: %v", err)
			}
			if scenario == "before changed" || scenario == "after changed" {
				if client.EndpointAttestations()[valuePrimary].State != "quarantined" {
					t.Fatal("changed block identity remained eligible")
				}
			}
		})
	}
}

func TestCommittedPeerReadRetainsRPCFailureClassification(t *testing.T) {
	for _, retryable := range []bool{false, true} {
		t.Run(map[bool]string{true: "unavailable", false: "rejected"}[retryable], func(t *testing.T) {
			header := testHeader(100)
			var enabled atomic.Bool
			var headers atomic.Int64
			peer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				var req struct {
					ID     json.RawMessage
					Method string
				}
				if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
					t.Error(err)
					return
				}
				var result any = "0x7d"
				if req.Method == "eth_getBlockByNumber" {
					if enabled.Load() && headers.Add(1) == 2 {
						if retryable {
							w.WriteHeader(http.StatusServiceUnavailable)
							return
						}
						if err := json.NewEncoder(w).Encode(map[string]any{"jsonrpc": "2.0", "id": req.ID, "error": map[string]any{"code": -32000, "message": "committed read rejected"}}); err != nil {
							t.Error(err)
						}
						return
					}
					result = reportedJSON(header)
				}
				if err := json.NewEncoder(w).Encode(map[string]any{"jsonrpc": "2.0", "id": req.ID, "result": result}); err != nil {
					t.Error(err)
				}
			}))
			defer peer.Close()
			var calls atomic.Int64
			primary := pinnedServer(t, header, false, &calls)
			client, err := NewVerifiedClient(context.Background(), primary.URL, peer.URL, VerifiedConfig{profile.Paxeer("e1", "d1"), 100, consensusHash(header)})
			if err != nil {
				t.Fatal(err)
			}
			defer client.Close()
			enabled.Store(true)
			err = client.checkCommittedHeader(context.Background(), client.primary, reportedBlock{Header: header, Hash: consensusHash(header)})
			if headers.Load() != 2 || (err == nil) != retryable {
				t.Fatalf("committed peer classification: calls=%d error=%v", headers.Load(), err)
			}
		})
	}
}
