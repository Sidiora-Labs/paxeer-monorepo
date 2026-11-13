package l18

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"math/big"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"
)

func rewardReceiptVariant(t *testing.T, origin string, mutate func(string, []json.RawMessage, map[string]any)) *Creator {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, err := io.ReadAll(r.Body)
		if err != nil {
			t.Error(err)
			return
		}
		var request struct {
			Method string
			Params []json.RawMessage
		}
		if err = json.Unmarshal(raw, &request); err != nil {
			t.Error(err)
			return
		}
		response, err := http.Post(origin, "application/json", bytes.NewReader(raw))
		if err != nil {
			t.Error(err)
			return
		}
		defer response.Body.Close()
		var envelope map[string]any
		if err = json.NewDecoder(response.Body).Decode(&envelope); err != nil {
			t.Error(err)
			return
		}
		mutate(request.Method, request.Params, envelope)
		if err = json.NewEncoder(w).Encode(envelope); err != nil {
			t.Error(err)
		}
	}))
	t.Cleanup(server.Close)
	rpc, err := newFixtureRPC(server.URL)
	if err != nil {
		t.Fatal(err)
	}
	return NewCreator(rpc, time.Now)
}

func TestRewardReceiptRejectsMalformedAndConflictingCanonicalEvidence(t *testing.T) {
	ctx := context.Background()
	raw, err := os.ReadFile(localChainFixture(t))
	if err != nil {
		t.Fatal(err)
	}
	var chain struct{ RPC, Pool, Alice string }
	if err = json.Unmarshal(raw, &chain); err != nil {
		t.Fatal(err)
	}
	client, err := newFixtureRPC(chain.RPC)
	if err != nil {
		t.Fatal(err)
	}
	hash := mustSend(t, client, chain.Alice, FeesRouter, router, "claimFees", big.NewInt(1))
	event := accumulator.Events["FeesClaimed"].ID.Hex()
	for _, tc := range []struct {
		name   string
		mutate func(map[string]any)
		want   error
	}{
		{"missing receipt", func(m map[string]any) { m["result"] = nil }, ErrUnavailable},
		{"invalid receipt schema", func(m map[string]any) { m["result"] = "invalid" }, nil},
		{"reverted receipt", func(m map[string]any) { m["result"].(map[string]any)["status"] = "0x0" }, ErrUnavailable},
		{"transaction binding", func(m map[string]any) {
			m["result"].(map[string]any)["transactionHash"] = "0x" + strings.Repeat("a", 64)
		}, ErrInvalid},
		{"frontier binding", func(m map[string]any) { m["result"].(map[string]any)["blockNumber"] = "0xfffffff" }, ErrInvalid},
		{"missing claim", func(m map[string]any) { m["result"].(map[string]any)["logs"] = []any{} }, ErrUnavailable},
		{"different pool", func(m map[string]any) {
			for _, v := range m["result"].(map[string]any)["logs"].([]any) {
				log := v.(map[string]any)
				topics := log["topics"].([]any)
				if topics[0] == event {
					topics[1] = addressTopic(actorA)
				}
			}
		}, ErrUnavailable},
		{"invalid claim hex", func(m map[string]any) {
			for _, v := range m["result"].(map[string]any)["logs"].([]any) {
				log := v.(map[string]any)
				if log["topics"].([]any)[0] == event {
					log["data"] = "0xgg"
				}
			}
		}, ErrInvalid},
		{"invalid claim payload", func(m map[string]any) {
			for _, v := range m["result"].(map[string]any)["logs"].([]any) {
				log := v.(map[string]any)
				if log["topics"].([]any)[0] == event {
					log["data"] = "0x00"
				}
			}
		}, ErrInvalid},
		{"duplicate claim", func(m map[string]any) {
			receipt := m["result"].(map[string]any)
			logs := receipt["logs"].([]any)
			for _, v := range logs {
				if v.(map[string]any)["topics"].([]any)[0] == event {
					receipt["logs"] = append(logs, v)
					break
				}
			}
		}, ErrConflict},
		{"transfer amount binding", func(m map[string]any) {
			for _, v := range m["result"].(map[string]any)["logs"].([]any) {
				log := v.(map[string]any)
				if strings.EqualFold(log["address"].(string), USDL) {
					log["data"] = "0x" + strings.Repeat("0", 63) + "1"
				}
			}
		}, ErrConflict},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c := rewardReceiptVariant(t, chain.RPC, func(method string, _ []json.RawMessage, m map[string]any) {
				if method == "eth_getTransactionReceipt" {
					tc.mutate(m)
				}
			})
			result, err := c.ClaimReceipt(ctx, hash, chain.Pool, chain.Alice)
			if err == nil || tc.want != nil && !errors.Is(err, tc.want) {
				t.Fatal(result, err, tc.want)
			}
		})
	}
	for _, tc := range []struct {
		name   string
		mutate func(string, []json.RawMessage, map[string]any)
		want   error
	}{
		{"wrong chain", func(method string, _ []json.RawMessage, m map[string]any) {
			if method == "eth_chainId" {
				m["result"] = "0x1"
			}
		}, ErrUnavailable},
		{"malformed frontier", func(method string, _ []json.RawMessage, m map[string]any) {
			if method == "eth_getBlockByNumber" {
				m["result"].(map[string]any)["hash"] = "invalid"
			}
		}, ErrUnavailable},
		{"canonical block binding", func(method string, params []json.RawMessage, m map[string]any) {
			if method == "eth_getBlockByNumber" && string(params[0]) != `"latest"` {
				m["result"].(map[string]any)["hash"] = "0x" + strings.Repeat("a", 64)
			}
		}, ErrUnavailable},
	} {
		t.Run(tc.name, func(t *testing.T) {
			result, err := rewardReceiptVariant(t, chain.RPC, tc.mutate).ClaimReceipt(ctx, hash, chain.Pool, chain.Alice)
			if !errors.Is(err, tc.want) {
				t.Fatal(result, err)
			}
		})
	}
	c := NewCreator(client, time.Now)
	for _, args := range [][3]string{{"invalid", chain.Pool, chain.Alice}, {hash, "invalid", chain.Alice}, {hash, chain.Pool, "invalid"}} {
		if _, err = c.ClaimReceipt(ctx, args[0], args[1], args[2]); !errors.Is(err, ErrInvalid) {
			t.Fatal(err)
		}
	}
	if _, err = c.read(ctx, Checkpoint{}, NFT, nft, "missing-method"); err == nil {
		t.Fatal("unknown ABI method accepted")
	}
}
