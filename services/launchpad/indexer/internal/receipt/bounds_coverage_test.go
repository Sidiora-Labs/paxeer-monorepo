package receipt

import (
	"context"
	"errors"
	"math/big"
	"strings"
	"testing"
	"time"

	"github.com/Sidiora-Technologies/KindleLaunch/core/indexer/internal/decode"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
)

func TestReceiptAuthorityRejectsInvalidRuntimeEpochs(t *testing.T) {
	input := repair53InputsFromProducer(t)
	ctx := context.Background()
	client, closeClient, err := NewBoundedRPCClient(ctx, input.Endpoint, "")
	if err != nil {
		t.Fatal(err)
	}
	defer closeClient()
	for _, tc := range []struct {
		name string
		edit func(*RPCOptions)
	}{
		{"identity mode", func(o *RPCOptions) { o.BlockIdentity = "unreviewed" }},
		{"missing identity", func(o *RPCOptions) { o.Anchor = nil; o.GenesisHash = common.Hash{} }},
		{"dual identity", func(o *RPCOptions) { o.GenesisHash = common.HexToHash("0x1") }},
		{"empty anchor", func(o *RPCOptions) { o.Anchor = &BlockAnchor{Height: 1} }},
		{"empty epochs", func(o *RPCOptions) { o.Epochs = nil }},
		{"timeout", func(o *RPCOptions) { o.Timeout = -time.Second }},
		{"long timeout", func(o *RPCOptions) { o.Timeout = time.Minute }},
		{"discovery role", func(o *RPCOptions) { o.Discovery[0].Role = "Unreviewed" }},
		{"discovery kind", func(o *RPCOptions) { o.Discovery[0].Kind = "unreviewed" }},
		{"discovery digest", func(o *RPCOptions) { o.Discovery[0].RuntimeHash = "invalid" }},
		{"epoch identity", func(o *RPCOptions) { o.Epochs[0].Identity.Address = "invalid" }},
		{"epoch address", func(o *RPCOptions) { o.Epochs[0].Deployment.Address = common.HexToAddress("0xbad") }},
		{"epoch height", func(o *RPCOptions) { o.Epochs[0].Deployment.From.Block = 0 }},
		{"epoch source", func(o *RPCOptions) { o.Epochs[0].SourceActivationHeight = 1 }},
		{"epoch role", func(o *RPCOptions) { o.Epochs[0].Deployment.Role = "Unreviewed" }},
		{"epoch abi", func(o *RPCOptions) { o.Epochs[0].Deployment.ABIRevision = strings.Repeat("0", 64) }},
		{"empty catalog", func(o *RPCOptions) { o.CatalogJSON = nil }},
		{"history count", func(o *RPCOptions) { o.History = make([]RPCGeneration, 17) }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			options, err := clone(input.Options)
			if err != nil {
				t.Fatal(err)
			}
			tc.edit(&options)
			if _, err = newRPCQualification(ctx, client, options, "bound", false, nil); err == nil {
				t.Fatal("unbound runtime authority accepted")
			}
		})
	}
}
func TestSourcePoolReservesMatchArchivedABIWords(t *testing.T) {
	input := repair53InputsFromProducer(t)
	ctx := context.Background()
	authority, closeClient := repair53Open(t, ctx, input)
	defer closeClient()
	if err := authority.Rehydrate(ctx, input.Markets); err != nil {
		t.Fatal(err)
	}
	for _, sample := range []struct{ block, real, token, price string }{
		{"0x1cd6b450aa87745be16a8760a928a2e7ddd1097e872bdcd99b31ff66f77c2945", "49721088", "995196593047440", "10098226981692"},
		{"0x50f4e42d7247aa1665d1aaebc33039c8fc8e493727a864085ba77910dda56f6b", "97000000", "990393186094880", "10194940899999"},
	} {
		reserves, err := authority.PoolReserves(ctx, input.Markets[0].Pool, common.HexToHash(sample.block))
		if err != nil {
			t.Fatal(err)
		}
		if reserves.VirtualUSDL.String() != "10000000000" || reserves.RealUSDL.String() != sample.real || reserves.Token.String() != sample.token || reserves.Price.String() != sample.price {
			t.Fatalf("pinned reserves %+v", reserves)
		}
	}
	for _, tc := range []struct {
		a     *RPCQualification
		pool  common.Address
		block common.Hash
	}{{nil, input.Markets[0].Pool, input.NativeSwap}, {authority, common.Address{}, input.NativeSwap}, {authority, input.Markets[0].Pool, common.Hash{}}} {
		if _, err := tc.a.PoolReserves(ctx, tc.pool, tc.block); !errors.Is(err, ErrQualification) {
			t.Fatalf("unbound reserve read %v", err)
		}
	}
}
func TestReceiptCoordinatesAndEnvelopeBoundsRejectChangedInputs(t *testing.T) {
	input := repair53InputsFromProducer(t)
	authority, closeClient := repair53Open(t, context.Background(), input)
	defer closeClient()
	bundle, err := authority.Bundle(context.Background(), input.NativeSwap)
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name string
		edit func(*Bundle)
	}{
		{"header", func(b *Bundle) { b.Header = nil }}, {"number", func(b *Bundle) { b.Header.Number = nil }}, {"timestamp", func(b *Bundle) { b.Header.Time = 0 }},
		{"log address", func(b *Bundle) { b.Receipt.Logs[0].Address = common.Address{} }}, {"removed log", func(b *Bundle) { b.Receipt.Logs[0].Removed = true }},
		{"prepay nil", func(b *Bundle) { b.PrepayReceipts = []*types.Receipt{nil} }}, {"duplicate prepay", func(b *Bundle) { b.PrepayReceipts = []*types.Receipt{b.Receipt} }},
		{"prepay order", func(b *Bundle) {
			p, _ := clone(b.Receipt)
			p.TxHash = common.HexToHash("0x1")
			b.PrepayReceipts = []*types.Receipt{p}
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			changed, err := clone(bundle)
			if err != nil {
				t.Fatal(err)
			}
			tc.edit(&changed)
			if _, err = coordinates(changed); !errors.Is(err, ErrQualification) {
				t.Fatalf("invalid coordinates %v", err)
			}
		})
	}
	for _, edit := range []func(*Bundle){func(b *Bundle) { b.PrepayReceipts = []*types.Receipt{nil} }, func(b *Bundle) { b.Header.Extra = make([]byte, decode.MaxFieldBytes+1) }, func(b *Bundle) { b.Receipt.Logs[0].Topics = make([]common.Hash, 5) }, func(b *Bundle) { b.Receipt.Logs[0].Data = make([]byte, decode.MaxFieldBytes+1) }, func(b *Bundle) { b.Receipt.Logs = make([]*types.Log, 4097) }} {
		changed, err := clone(bundle)
		if err != nil {
			t.Fatal(err)
		}
		edit(&changed)
		if err = boundedBundle(changed); !errors.Is(err, ErrQualification) {
			t.Fatalf("unbounded bundle %v", err)
		}
	}
	if _, err := coordinates(bundle); err != nil {
		t.Fatal(err)
	}
	unsupported := types.NewTx(&types.LegacyTx{V: big.NewInt(37), R: big.NewInt(1), S: big.NewInt(1)})
	if err := authority.ValidateSignedDomain(context.Background(), Bundle{Transaction: unsupported}); !errors.Is(err, ErrQualification) {
		t.Fatalf("unsupported domain %v", err)
	}
	key, err := crypto.GenerateKey()
	if err != nil {
		t.Fatal(err)
	}
	tx, err := types.SignTx(types.NewTx(&types.LegacyTx{}), types.LatestSignerForChainID(new(big.Int).SetUint64(virtualChainIDBase+1)), key)
	if err != nil {
		t.Fatal(err)
	}
	if err := authority.ValidateSignedDomain(context.Background(), Bundle{Transaction: tx}); !errors.Is(err, ErrQualification) {
		t.Fatalf("missing tenant context %v", err)
	}
}
