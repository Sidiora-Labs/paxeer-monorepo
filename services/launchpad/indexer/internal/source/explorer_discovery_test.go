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

func (f *explorerDiscoveryFixture) openRPC(t *testing.T) (*source.CommittedSource, *receipt.RPCQualification, func()) {
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

func (f *explorerDiscoveryFixture) explorerOptions(rpc *source.CommittedSource) source.CommittedPaxscanOptions {
	return source.CommittedPaxscanOptions{SourceID: "repair56-explorer", SchemaVersion: source.ExplorerSchemaV1, ManifestDigest: f.committed.ManifestDigest, Epochs: f.committed.Epochs, RPC: rpc}
}

// Project the real recorded receipts through the exact address plan and row
// predicate used by explorer SQL. This does not emulate a database or return
// manufactured explorer responses. PostgreSQL transport coverage is retained
// separately in TestExplorerDiscoveryPostgres*.
func explorerDiscoveryProject(t *testing.T, f *explorerDiscoveryFixture, rpc *source.CommittedSource, from, to uint64, discoveries []source.AddressEpoch) source.FetchResult {
	t.Helper()
	o := source.FetchOptions{FromBlock: int64(from), ToBlock: int64(to)}
	addresses, active, err := source.ExplorerDiscoveryFilter(f.explorerOptions(rpc), o, discoveries)
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

func explorerDiscoveryQualifyProjection(t *testing.T, f *explorerDiscoveryFixture, rpc *source.CommittedSource, authority *receipt.RPCQualification, from, to uint64) (store.CanonicalBatch, []*receipt.QualifiedReceipt, int) {
	t.Helper()
	attempts := 0
	beforeLogs := f.logReads.Load()
	batch, qualified, err := source.QualifyWithExplorerDiscoveries(f.ctx, authority, f.committed.ManifestDigest, from, to, "historical-", func(discoveries []source.AddressEpoch) (source.FetchResult, error) {
		attempts++
		return explorerDiscoveryProject(t, f, rpc, from, to, discoveries), nil
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

func TestExplorerDynamicDiscoveries(t *testing.T) {
	f := explorerDiscoveryRPCSetup(t)
	initialEpochs := append([]source.AddressEpoch(nil), f.committed.Epochs...)
	rpc, authority, stop := f.openRPC(t)
	if len(authority.DiscoveredEpochs()) != 0 {
		t.Fatal("cold authority already discovered markets")
	}
	creation, receipts, attempts := explorerDiscoveryQualifyProjection(t, f, rpc, authority, explorerDiscoveryCreation, explorerDiscoveryCreation)
	if attempts != 2 || len(authority.DiscoveredEpochs()) != 2 || len(receipts) != 1 {
		t.Fatal("market creation must grow discovery epochs and retry with a new explorer filter")
	}
	tokenLogs := 0
	for _, raw := range creation.Records {
		if raw.Emitter == explorerDiscoveryToken {
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
	buy, buyReceipts, attempts := explorerDiscoveryQualifyProjection(t, f, rpc, authority, explorerDiscoveryCreation+1, explorerDiscoveryBuy)
	if attempts != 1 {
		t.Fatal("known discoveries should qualify without an additional retry")
	}
	token, pool := false, false
	for _, raw := range buy.Records {
		token = token || raw.Emitter == explorerDiscoveryToken
		pool = pool || f.poolSwap(raw.Emitter, raw.Topics)
	}
	if !token || !pool || len(buyReceipts) != 1 {
		t.Fatal("subsequent history lost discovered token or pool events")
	}
	fills, err := buyReceipts[0].Fills(f.ctx)
	if err != nil || len(fills) != 1 || fills[0].Side != "buy" || fills[0].PoolAddress != explorerDiscoveryPool || fills[0].Token.Address != explorerDiscoveryToken {
		t.Fatalf("real qualified consumer did not recover buy: %+v %v", fills, err)
	}
	for _, interval := range []struct {
		from, to uint64
		expected store.CanonicalBatch
	}{{explorerDiscoveryCreation, explorerDiscoveryCreation, creation}, {explorerDiscoveryCreation + 1, explorerDiscoveryBuy, buy}} {
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
	recoveredCreation, _, attempts := explorerDiscoveryQualifyProjection(t, f, rpc, authority, explorerDiscoveryCreation, explorerDiscoveryCreation)
	if attempts != 2 {
		t.Fatal("reopened qualifier must rediscover and retry")
	}
	recoveredBuy, _, _ := explorerDiscoveryQualifyProjection(t, f, rpc, authority, explorerDiscoveryCreation+1, explorerDiscoveryBuy)
	if !reflect.DeepEqual(creation, recoveredCreation) || !reflect.DeepEqual(buy, recoveredBuy) {
		t.Fatal("reopened source and qualifier did not recover identical canonical history")
	}
}

func TestExplorerDiscoveryEpochBounds(t *testing.T) {
	f := explorerDiscoveryRPCSetup(t)
	rpc, authority, _ := f.openRPC(t)
	creation, _, _ := explorerDiscoveryQualifyProjection(t, f, rpc, authority, explorerDiscoveryCreation, explorerDiscoveryCreation)
	discoveries := source.DiscoveredAddressEpochs(authority, f.committed.ManifestDigest)
	var creationToken ethtypes.Log
	for _, log := range explorerDiscoveryProject(t, f, rpc, explorerDiscoveryCreation, explorerDiscoveryCreation, discoveries).Logs {
		if log.Address == common.HexToAddress(explorerDiscoveryToken) {
			creationToken = log
			break
		}
	}
	var buyToken ethtypes.Log
	for _, log := range explorerDiscoveryProject(t, f, rpc, explorerDiscoveryBuy, explorerDiscoveryBuy, discoveries).Logs {
		if log.Address == common.HexToAddress(explorerDiscoveryToken) {
			buyToken = log
			break
		}
	}
	if creationToken.BlockNumber != explorerDiscoveryCreation || buyToken.BlockNumber != explorerDiscoveryBuy || len(creation.Records) == 0 {
		t.Fatal("authentic creation and subsequent token logs required")
	}
	for _, discovery := range discoveries {
		if discovery.ActivationHeight != explorerDiscoveryCreation || discovery.CreationTxIndex != 0 {
			t.Fatal("authority discovery lost its creation bounds")
		}
	}
	options := f.explorerOptions(rpc)
	creationRange := source.FetchOptions{FromBlock: int64(explorerDiscoveryCreation), ToBlock: int64(explorerDiscoveryCreation)}
	buyRange := source.FetchOptions{FromBlock: int64(explorerDiscoveryBuy), ToBlock: int64(explorerDiscoveryBuy)}
	t.Run("creation_transaction", func(t *testing.T) {
		bounded := append([]source.AddressEpoch(nil), discoveries...)
		for i := range bounded {
			bounded[i].CreationTxIndex = 1
		}
		addresses, active, err := source.ExplorerDiscoveryFilter(options, creationRange, bounded)
		if err != nil || !slices.Contains(addresses, explorerDiscoveryToken) || active(creationToken) || !active(buyToken) {
			t.Fatalf("creation transaction bound must exclude earlier transactions only in the activation block: %v", err)
		}
		_, active, err = source.ExplorerDiscoveryFilter(options, creationRange, discoveries)
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
					bounded[i].ActivationHeight = explorerDiscoveryCreation + 1
				} else {
					retired := explorerDiscoveryBuy
					bounded[i].RetirementHeight = &retired
					interval, inactive, admitted = buyRange, buyToken, creationToken
				}
			}
			addresses, active, err := source.ExplorerDiscoveryFilter(options, interval, bounded)
			if err != nil || slices.Contains(addresses, explorerDiscoveryToken) || slices.Contains(addresses, explorerDiscoveryPool) || active(inactive) || !active(admitted) {
				t.Fatalf("inactive epochs must be absent from both SQL addresses and row admission: %v", err)
			}
			if kind == "retirement" {
				for i := range bounded {
					*bounded[i].RetirementHeight = explorerDiscoveryBuy + 1
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
			_, _, err := source.ExplorerDiscoveryFilter(options, creationRange, epochs)
			var refused *source.SourceError
			if !errors.As(err, &refused) || refused.Kind != source.IdentityError {
				t.Fatalf("invalid epoch must fail before explorer SQL: %v", err)
			}
			for _, explorer := range []bool{true, false} {
				result, err := source.HistoricalDiscoveryPage(f.ctx, options, creationRange, epochs, explorer)
				if !errors.As(err, &refused) || refused.Kind != source.IdentityError || result.Complete {
					t.Fatalf("historical page must forward discoveries to transport (explorer=%t): %v", explorer, err)
				}
			}
		}
	})
	addresses, active, err := source.ExplorerDiscoveryFilter(options, creationRange, nil)
	if err != nil || slices.Contains(addresses, explorerDiscoveryToken) || slices.Contains(addresses, explorerDiscoveryPool) || active(creationToken) {
		t.Fatalf("request discoveries mutated initial explorer options: %v", err)
	}
}
