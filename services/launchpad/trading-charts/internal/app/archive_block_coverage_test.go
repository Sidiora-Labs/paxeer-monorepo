package app

import (
	"context"
	"strings"
	"testing"

	"github.com/Sidiora-Technologies/KindleLaunch/core/trading-charts/internal/internaltest"
)

func TestArchiveBlockTimeUsesCanonicalChainHeader(t *testing.T) {
	chain := internaltest.NewMarketChain(t)
	ctx := context.Background()
	header := chain.Headers[1]
	for _, tc := range []struct {
		name     string
		height   int64
		expected string
		wantTime int64
		wantErr  string
	}{
		{"canonical header", 1, strings.ToUpper(header.Hash), header.Time, ""},
		{"negative height", -1, header.Hash, 0, "invalid archive height"},
		{"missing height", 100000, header.Hash, 0, "archive block timestamp unavailable"},
		{"different block", 1, "0x" + strings.Repeat("0", 64), 0, "archive block authority mismatch"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			stamp, err := archiveBlockTime(ctx, chain.RPC, tc.height, tc.expected)
			if stamp != tc.wantTime || (tc.wantErr == "" && err != nil) || (tc.wantErr != "" && (err == nil || !strings.Contains(err.Error(), tc.wantErr))) {
				t.Fatalf("archive timestamp: %d, %v", stamp, err)
			}
		})
	}
}

func TestDevelopmentChainClientRequiresHTTPAuthority(t *testing.T) {
	for _, raw := range []string{"", "ftp://127.0.0.1/chain", "https://user:pass@127.0.0.1/chain", "http://127.0.0.1/#fragment"} {
		client, err := developmentChainClient(context.Background(), raw)
		if client != nil || err == nil || !strings.Contains(err.Error(), "development chain RPC URL invalid") {
			t.Fatalf("development RPC %q: client=%v err=%v", raw, client, err)
		}
	}
	chain := internaltest.NewMarketChain(t)
	client, err := developmentChainClient(context.Background(), chain.RPCURL)
	if err != nil || client == nil {
		t.Fatalf("owned development RPC: %v", err)
	}
	defer client.Close()
	var chainID string
	if err := client.CallContext(context.Background(), &chainID, "eth_chainId"); err != nil || chainID != "0x7d" {
		t.Fatalf("development chain ID: %q, %v", chainID, err)
	}
}
