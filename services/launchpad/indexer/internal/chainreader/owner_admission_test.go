package chainreader

import (
	"context"
	"encoding/json"
	"math/big"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Sidiora-Technologies/KindleLaunch/core/indexer/internal/internaltest/chainfixture"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
)

func TestHistoricalOwnerRejectsZeroAndOversizedContractWords(t *testing.T) {
	c := actualAdversarialChain(t)
	reader, err := NewHistoricalReader(c.ctx, c.client)
	if err != nil {
		t.Fatal(err)
	}
	zero := []byte{0x60, 0, 0x60, 0, 0x52, 0x60, 0x20, 0x60, 0, 0xf3}
	wide := append([]byte{0x7f}, make([]byte, 32)...)
	for i := 1; i <= 32; i++ {
		wide[i] = 0xff
	}
	wide = append(wide, 0x60, 0, 0x52, 0x60, 0x20, 0x60, 0, 0xf3)
	for _, runtime := range [][]byte{zero, wide} {
		implementation, _ := deployRuntime(t, c, runtime)
		address, _, _, created := c.deploy(t, "ERC1967Proxy", implementation, []byte{})
		code, err := c.client.CodeAtHash(c.ctx, address, created.BlockHash)
		if err != nil {
			t.Fatal(err)
		}
		deployment := HistoricalDeployment{Address: address, Role: "SidioraNFT", DeploymentEpoch: created.TxHash.Hex(), Kind: "erc1967", CodeDigest: crypto.Keccak256Hash(code), Implementation: implementation, ImplementationDigest: crypto.Keccak256Hash(runtime)}
		out, err := reader.OwnerOf(c.ctx, deployment, created.BlockHash, big.NewInt(1))
		if err == nil || out.Status != "unavailable" || out.Value != nil || out.Failure != "invalid owner address" {
			t.Fatalf("invalid owner word: %+v %v", out, err)
		}
	}
	if out, err := (*HistoricalReader)(nil).Decimals(context.Background(), HistoricalDeployment{Role: "SidioraERC20"}, common.Hash{}); err == nil || out.Value != nil {
		t.Fatalf("unavailable decimal authority: %+v %v", out, err)
	}
}

func TestBoundedHistoricalConnectionRejectsForeignChainIdentity(t *testing.T) {
	fixture := chainfixture.New(t)
	fixture.Mutate = func(key string, body json.RawMessage) json.RawMessage {
		if strings.HasPrefix(key, "eth_chainId ") {
			return json.RawMessage(`"0x1"`)
		}
		return body
	}
	reader, err := NewBoundedHistoricalReader(context.Background(), fixture.Serve(t))
	if err == nil || reader != nil || !strings.Contains(err.Error(), "chain125") {
		t.Fatalf("foreign historical endpoint: %v", err)
	}
}

func TestHistoricalTransportRejectsTruncatedAndUnboundedBodies(t *testing.T) {
	for _, tc := range []struct{ name, want string }{{"truncated", "unexpected EOF"}, {"streamed", "exceeds byte cap"}, {"unavailable", "connect"}} {
		t.Run(tc.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				if tc.name == "truncated" {
					w.Header().Set("Content-Length", "128")
					_, _ = w.Write([]byte("{"))
					return
				}
				w.WriteHeader(200)
				w.(http.Flusher).Flush()
				_, _ = w.Write([]byte(strings.Repeat("x", MaxHistoricalResponseBytes+1)))
			}))
			defer server.Close()
			if tc.name == "unavailable" {
				server.Close()
			}
			upstream := &http.Transport{}
			defer upstream.CloseIdleConnections()
			request, err := http.NewRequestWithContext(context.Background(), http.MethodPost, server.URL, strings.NewReader(`{}`))
			if err != nil {
				t.Fatal(err)
			}
			response, err := (historicalTransport{upstream: upstream}).RoundTrip(request)
			if err == nil || response != nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("invalid historical transport: %+v %v", response, err)
			}
		})
	}
}
