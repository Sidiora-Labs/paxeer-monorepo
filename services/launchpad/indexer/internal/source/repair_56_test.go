package source_test

import (
	"errors"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/Sidiora-Technologies/KindleLaunch/core/indexer/internal/receipt"
	"github.com/Sidiora-Technologies/KindleLaunch/core/indexer/internal/source"
	"github.com/Sidiora-Technologies/KindleLaunch/core/indexer/internal/store"
	"github.com/ethereum/go-ethereum/common"
	ethtypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/ethclient"
)

func (f *repair56Fixture) openRPC(t *testing.T) (*source.CommittedSource, *receipt.RPCQualification, func()) {
	t.Helper()
	client, closeClient, err := receipt.NewBoundedRPCClient(f.ctx, f.endpoint, "")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(closeClient)
	authority, err := receipt.NewRPCQualification(f.ctx, client, f.options)
	if err != nil {
		t.Fatal(err)
	}
	rpc, err := source.NewCommittedSource(f.ctx, f.committed)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = rpc.Close() })
	return rpc, authority, func() {
		if err := rpc.Close(); err != nil {
			t.Fatal(err)
		}
		closeClient()
	}
}

func (f *repair56Fixture) explorerOptions(rpc *source.CommittedSource) source.CommittedPaxscanOptions {
	return source.CommittedPaxscanOptions{SourceID: "repair56-explorer", SchemaVersion: source.ExplorerSchemaV1, ManifestDigest: f.committed.ManifestDigest, Epochs: f.committed.Epochs, RPC: rpc}
}

// Project the real recorded receipts through the exact address plan and row
// predicate used by explorer SQL. This does not emulate a database or return
// manufactured explorer responses. PostgreSQL transport coverage is retained
// separately in TestRepair56Postgres_*.
func repair56Project(t *testing.T, f *repair56Fixture, rpc *source.CommittedSource, from, to uint64, discoveries []source.AddressEpoch) source.FetchResult {
	t.Helper()
	o := source.FetchOptions{FromBlock: int64(from), ToBlock: int64(to)}
	addresses, active, err := source.Repair56ExplorerFilter(f.explorerOptions(rpc), o, discoveries)
	if err != nil {
		t.Fatal(err)
	}
	if !slices.IsSorted(addresses) {
		t.Fatal("explorer SQL addresses must remain sorted")
	}
	for _, epoch := range discoveries {
		if epoch.ActivationHeight <= to && (epoch.RetirementHeight == nil || *epoch.RetirementHeight > from) && !slices.Contains(addresses, strings.ToLower(epoch.Address)) {
			t.Fatalf("discovered emitter missing from explorer SQL plan: %s", epoch.Address)
		}
	}
	client, err := ethclient.DialContext(f.ctx, f.endpoint)
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()
	result := source.FetchResult{ManifestDigest: f.committed.ManifestDigest, Blocks: map[uint64]source.BlockRecord{}, Complete: true}
	for height := from; height <= to; height++ {
		block, err := rpc.BlockAt(f.ctx, height)
		if err != nil {
			t.Fatal(err)
		}
		result.Blocks[height] = block
		var receipts []*ethtypes.Receipt
		if err := client.Client().CallContext(f.ctx, &receipts, "eth_getBlockReceipts", map[string]any{"blockHash": block.Hash.Hex(), "requireCanonical": true}); err != nil {
			t.Fatal(err)
		}
		for _, receipt := range receipts {
			for _, log := range receipt.Logs {
				if slices.Contains(addresses, strings.ToLower(log.Address.Hex())) && active(*log) {
					result.Logs = append(result.Logs, *log)
				}
			}
		}
	}
	return result
}

func repair56QualifyProjection(t *testing.T, f *repair56Fixture, rpc *source.CommittedSource, authority *receipt.RPCQualification, from, to uint64) (store.CanonicalBatch, []*receipt.QualifiedReceipt, int) {
	t.Helper()
	attempts := 0
	beforeLogs := f.logReads.Load()
	batch, qualified, err := source.Repair56QualifyWithDiscoveries(f.ctx, authority, f.committed.ManifestDigest, from, to, "historical-", func(discoveries []source.AddressEpoch) (source.FetchResult, error) {
		attempts++
		return repair56Project(t, f, rpc, from, to, discoveries), nil
	})
	if err != nil {
		t.Fatalf("qualify explorer epoch projection %d..%d: %v", from, to, err)
	}
	if f.logReads.Load() != beforeLogs {
		t.Fatal("explorer epoch projection unexpectedly fetched RPC logs")
	}
	if err := receipt.ValidateBindings(f.ctx, batch, qualified); err != nil {
		t.Fatal(err)
	}
	return batch, qualified, attempts
}

func TestRepair_56_ExplorerDynamicDiscoveries(t *testing.T) {
	f := repair56RPCSetup(t)
	initialEpochs := append([]source.AddressEpoch(nil), f.committed.Epochs...)
	rpc, authority, stop := f.openRPC(t)
	if len(authority.DiscoveredEpochs()) != 0 {
		t.Fatal("cold authority already discovered markets")
	}
	creation, receipts, attempts := repair56QualifyProjection(t, f, rpc, authority, repair56Creation, repair56Creation)
	if attempts != 2 || len(authority.DiscoveredEpochs()) != 2 || len(receipts) != 1 {
		t.Fatal("market creation must grow discovery epochs and retry with a new explorer filter")
	}
	tokenLogs := 0
	for _, raw := range creation.Records {
		if raw.Emitter == repair56Token {
			tokenLogs++
		}
	}
	if tokenLogs != 2 {
		t.Fatalf("creation transaction must include both token transfers: %d", tokenLogs)
	}
	events, err := receipts[0].QualifiedEvents(f.ctx)
	if err != nil {
		t.Fatal(err)
	}
	market := false
	for _, event := range events {
		if event.EmitterRole == "SidioraFactory" && event.EventName == "MarketCreated" {
			market = true
		}
	}
	if !market {
		t.Fatal("qualified factory market creation missing")
	}
	buy, buyReceipts, attempts := repair56QualifyProjection(t, f, rpc, authority, repair56Creation+1, repair56Buy)
	if attempts != 1 {
		t.Fatal("known discoveries should qualify without an additional retry")
	}
	token, pool := false, false
	for _, raw := range buy.Records {
		token = token || raw.Emitter == repair56Token
		pool = pool || f.poolSwap(raw.Emitter, raw.Topics)
	}
	if !token || !pool || len(buyReceipts) != 1 {
		t.Fatal("subsequent history lost discovered token or pool events")
	}
	fills, err := buyReceipts[0].Fills(f.ctx)
	if err != nil || len(fills) != 1 || fills[0].Side != "buy" || fills[0].PoolAddress != repair56Pool || fills[0].Token.Address != repair56Token {
		t.Fatalf("real qualified consumer did not recover buy: %+v %v", fills, err)
	}
	for _, interval := range []struct {
		from, to uint64
		expected store.CanonicalBatch
	}{{repair56Creation, repair56Creation, creation}, {repair56Creation + 1, repair56Buy, buy}} {
		live := &source.LiveQualifiedSource{RPC: rpc, Authority: authority, DurableHeight: func() uint64 { return interval.from - 1 }}
		rpcBatch, _, err := live.FetchQualifiedBatch(f.ctx, interval.from, interval.to)
		if err != nil || !reflect.DeepEqual(interval.expected.Blocks, rpcBatch.Blocks) || !reflect.DeepEqual(interval.expected.Records, rpcBatch.Records) {
			t.Fatalf("RPC and explorer-filtered canonical history differ: %v", err)
		}
	}
	if !reflect.DeepEqual(initialEpochs, f.committed.Epochs) {
		t.Fatal("request discoveries mutated initial options")
	}
	stop()
	rpc, authority, _ = f.openRPC(t)
	if len(authority.DiscoveredEpochs()) != 0 {
		t.Fatal("restart retained process-local discoveries")
	}
	recoveredCreation, _, attempts := repair56QualifyProjection(t, f, rpc, authority, repair56Creation, repair56Creation)
	if attempts != 2 {
		t.Fatal("reopened qualifier must rediscover and retry")
	}
	recoveredBuy, _, _ := repair56QualifyProjection(t, f, rpc, authority, repair56Creation+1, repair56Buy)
	if !reflect.DeepEqual(creation, recoveredCreation) || !reflect.DeepEqual(buy, recoveredBuy) {
		t.Fatal("reopened source and qualifier did not recover identical canonical history")
	}
}

func TestRepair_56_DiscoveryEpochBounds(t *testing.T) {
	f := repair56RPCSetup(t)
	rpc, authority, _ := f.openRPC(t)
	creation, _, _ := repair56QualifyProjection(t, f, rpc, authority, repair56Creation, repair56Creation)
	discoveries := source.DiscoveredAddressEpochs(authority, f.committed.ManifestDigest)
	var creationToken ethtypes.Log
	for _, log := range repair56Project(t, f, rpc, repair56Creation, repair56Creation, discoveries).Logs {
		if log.Address == common.HexToAddress(repair56Token) {
			creationToken = log
			break
		}
	}
	var buyToken ethtypes.Log
	for _, log := range repair56Project(t, f, rpc, repair56Buy, repair56Buy, discoveries).Logs {
		if log.Address == common.HexToAddress(repair56Token) {
			buyToken = log
			break
		}
	}
	if creationToken.BlockNumber != repair56Creation || buyToken.BlockNumber != repair56Buy || len(creation.Records) == 0 {
		t.Fatal("authentic creation and subsequent token logs required")
	}
	for _, discovery := range discoveries {
		if discovery.ActivationHeight != repair56Creation || discovery.CreationTxIndex != 0 {
			t.Fatal("authority discovery lost its creation bounds")
		}
	}
	options := f.explorerOptions(rpc)
	creationRange := source.FetchOptions{FromBlock: int64(repair56Creation), ToBlock: int64(repair56Creation)}
	buyRange := source.FetchOptions{FromBlock: int64(repair56Buy), ToBlock: int64(repair56Buy)}
	t.Run("creation_transaction", func(t *testing.T) {
		bounded := append([]source.AddressEpoch(nil), discoveries...)
		for i := range bounded {
			bounded[i].CreationTxIndex = 1
		}
		addresses, active, err := source.Repair56ExplorerFilter(options, creationRange, bounded)
		if err != nil || !slices.Contains(addresses, repair56Token) || active(creationToken) || !active(buyToken) {
			t.Fatalf("creation transaction bound must exclude earlier transactions only in the activation block: %v", err)
		}
		_, active, err = source.Repair56ExplorerFilter(options, creationRange, discoveries)
		if err != nil || !active(creationToken) {
			t.Fatalf("events in the creation transaction must be admitted: %v", err)
		}
	})
	for _, kind := range []string{"activation", "retirement"} {
		t.Run(kind, func(t *testing.T) {
			bounded := append([]source.AddressEpoch(nil), discoveries...)
			interval, inactive, admitted := creationRange, creationToken, buyToken
			for i := range bounded {
				if kind == "activation" {
					bounded[i].ActivationHeight = repair56Creation + 1
				} else {
					retired := repair56Buy
					bounded[i].RetirementHeight = &retired
					interval, inactive, admitted = buyRange, buyToken, creationToken
				}
			}
			addresses, active, err := source.Repair56ExplorerFilter(options, interval, bounded)
			if err != nil || slices.Contains(addresses, repair56Token) || slices.Contains(addresses, repair56Pool) || active(inactive) || !active(admitted) {
				t.Fatalf("inactive epochs must be absent from both SQL addresses and row admission: %v", err)
			}
			if kind == "retirement" {
				for i := range bounded {
					*bounded[i].RetirementHeight = repair56Buy + 1
					bounded[i].Address = "0x0000000000000000000000000000000000000000"
				}
				if active(buyToken) || !active(creationToken) {
					t.Fatal("caller mutation changed the request-local epoch snapshot")
				}
			}
		})
	}
	t.Run("invalid_authority_and_historical_wiring", func(t *testing.T) {
		invalid := append([]source.AddressEpoch(nil), discoveries...)
		invalid[0].ManifestDigest = "unapproved"
		for _, epochs := range [][]source.AddressEpoch{invalid, {f.committed.Epochs[0]}, {discoveries[0], discoveries[0]}} {
			_, _, err := source.Repair56ExplorerFilter(options, creationRange, epochs)
			var refused *source.SourceError
			if !errors.As(err, &refused) || refused.Kind != source.IdentityError {
				t.Fatalf("invalid epoch must fail before explorer SQL: %v", err)
			}
			for _, explorer := range []bool{true, false} {
				result, err := source.Repair56HistoricalPage(f.ctx, options, creationRange, epochs, explorer)
				if !errors.As(err, &refused) || refused.Kind != source.IdentityError || result.Complete {
					t.Fatalf("historical page must forward discoveries to transport (explorer=%t): %v", explorer, err)
				}
			}
		}
	})
	addresses, active, err := source.Repair56ExplorerFilter(options, creationRange, nil)
	if err != nil || slices.Contains(addresses, repair56Token) || slices.Contains(addresses, repair56Pool) || active(creationToken) {
		t.Fatalf("request discoveries mutated initial explorer options: %v", err)
	}
}
