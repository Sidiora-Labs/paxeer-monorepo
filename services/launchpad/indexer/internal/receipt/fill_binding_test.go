package receipt

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/Sidiora-Technologies/KindleLaunch/core/indexer/internal/decode"
	"github.com/ethereum/go-ethereum/common"
)

func TestFillRequiresCanonicalSwapArguments(t *testing.T) {
	input := repair53InputsFromProducer(t)
	ctx := context.Background()
	authority, closeClient := repair53Open(t, ctx, input)
	defer closeClient()
	if err := authority.Rehydrate(ctx, input.Markets); err != nil {
		t.Fatal(err)
	}
	authority = authority.current()
	b, err := authority.Bundle(ctx, input.NativeSwap)
	if err != nil {
		t.Fatal(err)
	}
	sender, err := coordinates(b)
	if err != nil {
		t.Fatal(err)
	}
	receiptHash, err := digest(b.Receipt)
	if err != nil {
		t.Fatal(err)
	}
	headerHash, err := provenanceHeaderDigest(b.Header)
	if err != nil {
		t.Fatal(err)
	}
	for _, log := range b.Receipt.Logs {
		original, err := authority.Registry().Decode(*log)
		if err != nil {
			t.Fatal(err)
		}
		if !qualifiedSwapFamily(original, log) {
			continue
		}
		valid, err := makeFill(ctx, authority, b, original, log, sender, receiptHash, headerHash)
		if err != nil || valid.SourceFillID == "" || valid.BlockTimestampSeconds == "" {
			t.Fatalf("source fill %+v %v", valid, err)
		}
		for _, tc := range []struct {
			name  string
			key   string
			value any
		}{
			{"time", "timestamp", "0"}, {"height", "blockNumber", "0"},
			{"pool type", "poolId", 1}, {"pool length", "poolId", "0x"},
			{"pool encoding", "poolId", "0x" + strings.Repeat("z", 64)},
			{"pool padding", "poolId", "0x01" + strings.Repeat("0", 62)},
			{"zero pool", "poolId", common.Hash{}.Hex()},
			{"emitter pool", "poolId", common.BytesToHash(log.Address.Bytes()).Hex()},
			{"missing market", "poolId", common.HexToHash("0xbad").Hex()},
			{"side", "isBuy", "true"}, {"input", "amountIn", 1}, {"output", "amountOut", 1}, {"fee", "fee", 1},
		} {
			t.Run(tc.name, func(t *testing.T) {
				env, err := clone(original)
				if err != nil {
					t.Fatal(err)
				}
				env.Event.Args[tc.key] = tc.value
				if _, err = makeFill(ctx, authority, b, env, log, sender, receiptHash, headerHash); !errors.Is(err, ErrQualification) {
					t.Fatalf("changed %s accepted: %v", tc.key, err)
				}
			})
		}
		changed, err := clone(original)
		if err != nil {
			t.Fatal(err)
		}
		changed.ABIRevision = strings.Repeat("0", 64)
		if _, err = makeFill(ctx, authority, b, changed, log, sender, receiptHash, headerHash); !errors.Is(err, ErrQualification) {
			t.Fatalf("changed ABI accepted: %v", err)
		}
		return
	}
	t.Fatal("recorded swap absent")
}

func TestTenantActorsRequireUniqueAdmittedMarket(t *testing.T) {
	input := repair53InputsFromProducer(t)
	ctx := context.Background()
	authority, closeClient := repair53Open(t, ctx, input)
	defer closeClient()
	if err := authority.Rehydrate(ctx, input.Markets); err != nil {
		t.Fatal(err)
	}
	authority = authority.current()
	b, err := authority.Bundle(ctx, input.TenantSwap)
	if err != nil {
		t.Fatal(err)
	}
	q, err := Qualify(ctx, authority.Registry(), authority, b)
	if err != nil {
		t.Fatal(err)
	}
	fills, err := q.Fills(ctx)
	if err != nil || len(fills) != 1 {
		t.Fatalf("tenant fills %v %v", fills, err)
	}
	f := fills[0]
	market := decode.ActorMarketBinding{Pool: common.HexToAddress(f.PoolAddress), Token: common.HexToAddress(f.Token.Address), Settlement: common.HexToAddress(f.Settlement.Address)}
	decoded := make([]*decode.DecodedEnvelope, len(b.Receipt.Logs))
	swapIndex := -1
	for i, l := range b.Receipt.Logs {
		decoded[i], err = authority.Registry().Decode(*l)
		if err != nil {
			t.Fatal(err)
		}
		if qualifiedSwapFamily(decoded[i], l) {
			swapIndex = i
		}
	}
	if swapIndex < 0 {
		t.Fatal("tenant swap absent")
	}
	out, err := tenantReceiptActors(ctx, authority, b, []decode.ActorMarketBinding{market}, decoded)
	if err != nil || len(out) != len(decoded) || out[swapIndex].Actors == nil || out[swapIndex].Actors.TransactionOrigin == nil {
		t.Fatalf("tenant actors %v %v", out, err)
	}
	for _, tc := range []struct {
		name    string
		markets []decode.ActorMarketBinding
	}{
		{"missing", nil}, {"duplicate", []decode.ActorMarketBinding{market, market}},
		{"unbounded", make([]decode.ActorMarketBinding, 65)},
		{"zero pool", []decode.ActorMarketBinding{{Token: market.Token, Settlement: market.Settlement}}},
		{"zero token", []decode.ActorMarketBinding{{Pool: market.Pool, Settlement: market.Settlement}}},
		{"equal assets", []decode.ActorMarketBinding{{Pool: market.Pool, Token: market.Token, Settlement: market.Token}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := tenantReceiptActors(ctx, authority, b, tc.markets, decoded); !errors.Is(err, ErrQualification) {
				t.Fatalf("changed market admitted: %v", err)
			}
		})
	}
	for _, edit := range []func([]*decode.DecodedEnvelope){
		func(e []*decode.DecodedEnvelope) { e[0] = nil },
		func(e []*decode.DecodedEnvelope) { e[swapIndex].Event.Args["poolId"] = true },
		func(e []*decode.DecodedEnvelope) {
			e[swapIndex].Event.Args["poolId"] = "0x01" + strings.Repeat("0", 62)
		},
		func(e []*decode.DecodedEnvelope) { e[swapIndex].Event.Args["isBuy"] = "false" },
	} {
		changed, err := clone(decoded)
		if err != nil {
			t.Fatal(err)
		}
		edit(changed)
		if _, err = tenantReceiptActors(ctx, authority, b, []decode.ActorMarketBinding{market}, changed); !errors.Is(err, ErrQualification) {
			t.Fatalf("changed envelope admitted: %v", err)
		}
	}
	if _, err = tenantReceiptActors(ctx, authority, b, []decode.ActorMarketBinding{market}, decoded[1:]); !errors.Is(err, ErrQualification) {
		t.Fatalf("missing envelope admitted: %v", err)
	}
}
