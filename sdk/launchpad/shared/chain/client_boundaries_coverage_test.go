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
	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
)

func TestReportedBlockRequiresNodeIdentityAndHeight(t *testing.T) {
	for _, tc := range []struct {
		name string
		raw  string
	}{
		{"missing", "null"},
		{"malformed", `{`},
		{"missing hash", `{"number":"0x1","difficulty":"0x0","gasLimit":"0x1","timestamp":"0x1"}`},
		{"zero hash", `{"number":"0x1","difficulty":"0x0","gasLimit":"0x1","timestamp":"0x1","hash":"0x0000000000000000000000000000000000000000000000000000000000000000"}`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if block, err := decodeReportedBlock(json.RawMessage(tc.raw)); err == nil || block.Header != nil {
				t.Fatalf("block without reported identity accepted: %+v, %v", block, err)
			}
		})
	}
	if _, err := NewVerifiedClient(context.Background(), "", "", VerifiedConfig{}); err == nil {
		t.Fatal("empty anchor accepted")
	}
	if _, err := NewClient(context.Background(), "file:///tmp/rpc", ""); err == nil {
		t.Fatal("unbounded transport accepted")
	}
	header := testHeader(100)
	var calls atomic.Int64
	server := pinnedServer(t, header, false, &calls)
	client, err := NewClient(context.Background(), server.URL, "")
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()
	if _, err := client.Pin(context.Background(), 100); err == nil {
		t.Fatal("unverified client issued a pin")
	}
	if _, err := client.LogsBounded(context.Background(), ethereum.FilterQuery{FromBlock: big.NewInt(100), ToBlock: big.NewInt(100)}, 1); err == nil {
		t.Fatal("unverified client served committed logs")
	}
	if attestations := client.EndpointAttestations(); attestations[valuePrimary].State != "configured" {
		t.Fatalf("initial endpoint state: %+v", attestations)
	}
}

func TestPinnedReadRejectsUnboundRequestsAndCommittedConflicts(t *testing.T) {
	header := testHeader(100)
	var calls atomic.Int64
	server := pinnedServer(t, header, false, &calls)
	client, err := NewVerifiedClient(context.Background(), server.URL, "", VerifiedConfig{profile.Paxeer("e1", "d1"), 100, consensusHash(header)})
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()
	if err := client.checkCommittedHeader(context.Background(), client.primary, reportedBlock{}); err == nil {
		t.Fatal("empty committed header accepted")
	}
	if _, err := client.ReadPinned(context.Background(), BlockPin{}, nil); err == nil {
		t.Fatal("empty snapshot accepted")
	}
	pin, err := client.Pin(context.Background(), 100)
	if err != nil {
		t.Fatal(err)
	}
	to := common.HexToAddress("0x1111111111111111111111111111111111111111")
	unknown := pin
	unknown.Hash = common.HexToHash("0x77")
	if values, err := client.ReadPinned(context.Background(), unknown, []ethereum.CallMsg{{To: &to}}); err == nil || values != nil {
		t.Fatalf("unissued block pin served a read: %x, %v", values, err)
	}
	changed := testHeader(100)
	changed.Extra = []byte{9}
	if err := client.checkCommittedHeader(context.Background(), client.primary, reportedBlock{Header: changed, Hash: consensusHash(changed)}); err == nil {
		t.Fatal("committed height conflict accepted")
	}
	if client.Eth() != nil || client.EndpointAttestations()[valuePrimary].State != "quarantined" {
		t.Fatal("conflicting endpoint remained available")
	}
	if err := client.serving(); err == nil {
		t.Fatal("quarantined endpoint served a read")
	}
}

func TestCommittedHistoryRejectsPeerOmissionAndDisagreement(t *testing.T) {
	header := testHeader(100)
	other := testHeader(100)
	other.Extra = []byte{8}
	for _, tc := range []struct {
		name     string
		response any
		latest   any
		valid    bool
	}{
		{"omitted", nil, reportedJSON(header), false},
		{"disagreed", reportedJSON(other), reportedJSON(header), false},
		{"lagging", nil, reportedJSON(testHeader(99)), true},
		{"missing latest", nil, nil, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			peer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				var request struct {
					ID     json.RawMessage   `json:"id"`
					Method string            `json:"method"`
					Params []json.RawMessage `json:"params"`
				}
				if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
					t.Error(err)
					return
				}
				var result any = "0x7d"
				if request.Method == "eth_getBlockByNumber" {
					result = tc.response
					if len(request.Params) > 0 && string(request.Params[0]) == `"latest"` {
						result = tc.latest
					}
				}
				if err := json.NewEncoder(w).Encode(map[string]any{"jsonrpc": "2.0", "id": request.ID, "result": result}); err != nil {
					t.Error(err)
				}
			}))
			defer peer.Close()
			var calls atomic.Int64
			primary := pinnedServer(t, header, false, &calls)
			client, err := NewClient(context.Background(), primary.URL, peer.URL)
			if err != nil {
				t.Fatal(err)
			}
			defer client.Close()
			err = client.checkCommittedHeader(context.Background(), client.primary, reportedBlock{Header: header, Hash: consensusHash(header)})
			if tc.valid {
				if err != nil || client.EndpointAttestations()[valueFallback].State != "lagging" {
					t.Fatalf("lagging peer authority: %+v %v", client.EndpointAttestations(), err)
				}
			} else if err == nil {
				t.Fatal("peer omitted or changed committed identity")
			}
			if tc.name == "disagreed" && client.EndpointAttestations()[valuePrimary].State != "quarantined" {
				t.Fatal("disagreeing endpoint was not quarantined")
			}
		})
	}
	if _, err := decodeReportedBlock(nil); !errors.Is(err, ethereum.NotFound) {
		t.Fatalf("missing block identity: %v", err)
	}
	_ = types.Header{}
}

func TestEndpointEligibilityAndPinRetentionAreBounded(t *testing.T) {
	ctx := context.Background()
	header := testHeader(100)
	var calls atomic.Int64
	primary := pinnedServer(t, header, false, &calls)
	peer := pinnedServer(t, header, false, &calls)
	client, err := NewVerifiedClient(ctx, primary.URL, peer.URL, VerifiedConfig{profile.Paxeer("e1", "d1"), 100, consensusHash(header)})
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()
	if client.Eth() != client.primary {
		t.Fatal("verified endpoint unavailable")
	}
	client.quarantineEndpoint(client.fallback)
	if err := client.checkCommittedHeader(ctx, client.primary, reportedBlock{Header: header, Hash: consensusHash(header)}); err == nil {
		t.Fatal("quarantined peer served history")
	}
	client.mu.Lock()
	delete(client.quarantine, client.fallback)
	client.mu.Unlock()
	cancelled, cancel := context.WithCancel(ctx)
	cancel()
	if err := client.checkCommittedHeader(cancelled, client.primary, reportedBlock{Header: header, Hash: consensusHash(header)}); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancelled peer read: %v", err)
	}
	peer.Close()
	if err := client.checkCommittedHeader(ctx, client.primary, reportedBlock{Header: header, Hash: consensusHash(header)}); err != nil {
		t.Fatalf("available committed endpoint with unavailable peer: %v", err)
	}
	primary.Close()
	if client.Eth() != nil {
		t.Fatal("unavailable endpoint exposed")
	}
	retained := BlockPin{Number: 1, Hash: common.Hash{1}, DeploymentVersion: "d1"}
	var singleCalls atomic.Int64
	single := pinnedServer(t, header, false, &singleCalls)
	bounded, err := NewVerifiedClient(ctx, single.URL, "", VerifiedConfig{profile.Paxeer("e1", "d1"), 100, consensusHash(header)})
	if err != nil {
		t.Fatal(err)
	}
	defer bounded.Close()
	bounded.mu.Lock()
	bounded.pinOrder = make([]BlockPin, 1024)
	for i := range bounded.pinOrder {
		pin := retained
		pin.Number = uint64(i + 1000)
		if i == 0 {
			pin = retained
		}
		bounded.pinOrder[i] = pin
		bounded.issuedPins[pin] = struct{}{}
	}
	bounded.mu.Unlock()
	if _, err := bounded.Pin(ctx, 100); err != nil {
		t.Fatal(err)
	}
	bounded.mu.Lock()
	_, present := bounded.issuedPins[retained]
	length := len(bounded.pinOrder)
	bounded.mu.Unlock()
	if present || length != 1024 {
		t.Fatalf("retained pin bound: %t %d", present, length)
	}
}
