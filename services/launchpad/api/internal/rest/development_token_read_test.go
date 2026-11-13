package rest

import (
	"context"
	"strconv"
	"strings"
	"testing"

	"github.com/Sidiora-Technologies/KindleLaunch/core/api/internal/store"
)

func TestDevelopmentTokenReadPreservesPartialOnChainMetadata(t *testing.T) {
	chain := newRepair58Chain(t)
	ctx := context.Background()
	head, err := chain.backend.Client().HeaderByNumber(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	reader, err := newDevelopmentChain(ctx, chain.origin)
	if err != nil {
		t.Fatal(err)
	}
	defer reader.client.Close()
	token := strings.ToLower(chain.token.Hex())
	document, err := reader.tokenDocument(ctx, token, store.ChainWatermark{ChainID: "125", Height: strconv.FormatUint(head.Number.Uint64(), 10), BlockHash: head.Hash().Hex()})
	if err != nil {
		t.Fatal(err)
	}
	if document["address"] != token || document["name"] != "" || document["symbol"] != "" || document["sourcePosition"] == nil {
		t.Fatalf("partially implemented token metadata: %+v", document)
	}
	if document["decimals"] != 18 || document["totalSupplyRaw"] != "18" || document["decimalsQuality"] != "verified" || document["supplyQuality"] != "verified" {
		t.Fatalf("verified numeric token methods: %+v", document)
	}
}
