package receipt

import (
	"context"
	"encoding/json"
	"errors"
	"math"
	"math/big"
	"reflect"
	"testing"
	"time"

	"github.com/Sidiora-Technologies/KindleLaunch/core/indexer/internal/decode"
	"github.com/Sidiora-Technologies/KindleLaunch/core/indexer/internal/store"
	"github.com/Sidiora-Technologies/KindleLaunch/shared/config/deployment"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
)

func TestQualifiedBatchRechecksRecordedIdentitiesAndPrefetch(t *testing.T) {
	input := repair53InputsFromProducer(t)
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	authority, closeClient := repair53Open(t, ctx, input)
	defer closeClient()
	if err := authority.Rehydrate(ctx, input.Markets); err != nil {
		t.Fatal(err)
	}
	authority = authority.current()
	bundle, err := authority.Bundle(ctx, input.NativeSwap)
	if err != nil {
		t.Fatal(err)
	}
	height := bundle.Header.Number.Uint64()
	if err = authority.ConfigureArchiveReads(2, 2); err != nil {
		t.Fatal(err)
	}
	authority.ReadAhead(height, height)
	authority.ReadAhead(height, height)
	batch, receipts, err := authority.FetchQualifiedBatch(ctx, height, height)
	if err != nil || len(receipts) == 0 || authority.ArchiveReadMetrics().ReadaheadHits != 1 {
		t.Fatalf("prefetched receipts %d %+v %v", len(receipts), authority.ArchiveReadMetrics(), err)
	}
	checked, err := authority.QualifiedBatch(ctx, batch)
	if err != nil || len(checked) != len(receipts) {
		t.Fatalf("source-bound batch %d/%d %v", len(checked), len(receipts), err)
	}
	for i, q := range checked {
		before, e := receipts[i].SourceLogs(ctx)
		if e != nil {
			t.Fatal(e)
		}
		after, e := q.SourceLogs(ctx)
		if e != nil || !reflect.DeepEqual(before, after) {
			t.Fatalf("original receipt logs changed %v", e)
		}
		if len(after) > 0 {
			after[0].Data = append(after[0].Data, 0xff)
			again, e := q.SourceLogs(ctx)
			if e != nil || !reflect.DeepEqual(before, again) {
				t.Fatalf("source logs share mutable storage %v", e)
			}
		}
	}
	for _, field := range []string{"chain", "empty", "bounds", "block", "count", "raw", "position", "duplicate"} {
		t.Run(field, func(t *testing.T) {
			changed, e := clone(batch)
			if e != nil {
				t.Fatal(e)
			}
			switch field {
			case "chain":
				changed.ChainID = 1
			case "empty":
				changed.Blocks = nil
			case "bounds":
				changed.Blocks[0].Height = -1
			case "block":
				changed.Blocks[0].Hash = common.HexToHash("0xaaaa").Hex()
			case "count":
				changed.Records = changed.Records[1:]
			case "raw":
				changed.Records[0].Data = append(changed.Records[0].Data, 0xff)
			case "position":
				changed.Records[0].LogIndex = 9999
			case "duplicate":
				changed.Records[1] = changed.Records[0]
			}
			if _, e = authority.QualifiedBatch(ctx, changed); !errors.Is(e, ErrBinding) {
				t.Fatalf("changed %s source accepted %v", field, e)
			}
		})
	}
	epochs := authority.DiscoveredEpochs()
	if len(epochs) < 2 {
		t.Fatalf("restored market epochs %d", len(epochs))
	}
	for _, epoch := range epochs {
		if authority.current().manifestAddress(epoch.Deployment.Address) || epoch.Deployment.Role != "SidioraPool" && epoch.Deployment.Role != "SidioraERC20" {
			t.Fatalf("non-market discovery %+v", epoch)
		}
	}
	if (*RPCQualification)(nil).DiscoveredEpochs() != nil {
		t.Fatal("absent authority has discoveries")
	}
	authority.ReadAhead(height, height)
	pin, ok := authority.reads.takeAhead(ctx, height)
	if !ok || pin.header.Number.Uint64() != height {
		t.Fatal("source pin absent")
	}
	authority.ReadAhead(height, height)
	authority.reads.mu.Lock()
	entry := authority.reads.ahead[height]
	authority.reads.mu.Unlock()
	select {
	case <-entry.done:
	case <-ctx.Done():
		t.Fatal(ctx.Err())
	}
	authority.reads.dropAheadBelow(height + 1)
	if _, ok = authority.reads.takeAhead(ctx, height); ok {
		t.Fatal("expired read-ahead pin retained")
	}
	if err = authority.ConfigureArchiveReads(0, -1); err != nil {
		t.Fatal(err)
	}
	authority.ReadAhead(height, height)
	if _, ok = authority.reads.takeAhead(ctx, height); ok {
		t.Fatal("disabled prefetch issued reads")
	}
	if metrics := authority.ArchiveReadMetrics(); metrics.ReadaheadHits < 2 || metrics.ReceiptReads == 0 {
		t.Fatalf("archive counters %+v", metrics)
	}
}

func TestReceiptBundleAndPolicyRejectChangedEvidence(t *testing.T) {
	input := repair53InputsFromProducer(t)
	ctx := context.Background()
	authority, closeClient := repair53Open(t, ctx, input)
	defer closeClient()
	if err := authority.Rehydrate(ctx, input.Markets); err != nil {
		t.Fatal(err)
	}
	authority = authority.current()
	bundle, err := authority.Bundle(ctx, input.NativeSwap)
	if err != nil {
		t.Fatal(err)
	}
	for _, field := range []string{"missing", "status", "block", "index", "gas", "logs", "header", "sender", "domain"} {
		t.Run(field, func(t *testing.T) {
			changed, e := clone(bundle)
			if e != nil {
				t.Fatal(e)
			}
			switch field {
			case "missing":
				changed.Receipt = nil
			case "status":
				changed.Receipt.Status = 0
			case "block":
				changed.BlockHash = common.HexToHash("0xbad")
			case "index":
				changed.Receipt.TransactionIndex = uint(len(bundle.Receipt.Logs) + 10000)
			case "gas":
				changed.Receipt.GasUsed++
			case "logs":
				changed.Receipt.Logs = changed.Receipt.Logs[:len(changed.Receipt.Logs)-1]
			case "header":
				changed.Header.Time++
			case "sender":
				changed.Transaction = types.NewTx(&types.LegacyTx{Nonce: 1, GasPrice: big.NewInt(1), Gas: 21000, To: bundle.Transaction.To(), Value: big.NewInt(0)})
			case "domain":
				changed.Transaction = types.NewTx(&types.LegacyTx{Nonce: 1, GasPrice: big.NewInt(1), Gas: 21000, To: bundle.Transaction.To(), Value: big.NewInt(0), V: big.NewInt(37), R: big.NewInt(1), S: big.NewInt(1)})
			}
			if e = authority.ValidateReceipt(ctx, changed); e == nil {
				t.Fatal("changed receipt evidence accepted")
			}
			if _, e = authority.SourceProvenance(ctx, changed, decode.LaunchpadPrecompileAddress); e == nil {
				t.Fatal("changed receipt provenance accepted")
			}
			if _, e = authority.ExecutionPolicy(ctx, changed, 0); e == nil {
				t.Fatal("changed receipt policy accepted")
			}
		})
	}
	if _, err = authority.SourceProvenance(ctx, bundle, common.HexToAddress("0xbad")); !errors.Is(err, ErrQualification) {
		t.Fatalf("unrelated emitter provenance %v", err)
	}
	if _, err = authority.ExecutionPolicy(ctx, bundle, math.MaxUint32); !errors.Is(err, ErrQualification) {
		t.Fatalf("absent swap policy %v", err)
	}
}

func TestPrivateFixtureAdmissionRequiresActualDeploymentBindings(t *testing.T) {
	ctx, client := realReceiptNode(t)
	input := repair53InputsFromProducer(t)
	fixture := PrivateFixtureRPCOptions{DeploymentVersion: "private-fixture-admission", ActivationHeight: input.Options.Manifest.ActivationHeight, Roles: input.Options.Manifest.Roles, CatalogJSON: input.Options.CatalogJSON, Epochs: input.Options.Epochs, GenesisHash: common.HexToHash("0xabc"), CreationTransactions: map[common.Address]common.Hash{common.HexToAddress("0x123"): common.HexToHash("0x456")}}
	for _, change := range []func(*PrivateFixtureRPCOptions){
		func(f *PrivateFixtureRPCOptions) { f.DeploymentVersion = "public" },
		func(f *PrivateFixtureRPCOptions) { f.ActivationHeight = 0 },
		func(f *PrivateFixtureRPCOptions) { f.CreationTransactions = nil },
		func(f *PrivateFixtureRPCOptions) { f.Roles = nil },
		func(f *PrivateFixtureRPCOptions) {
			f.Roles = map[string]deployment.Role{}
			for k, v := range fixture.Roles {
				f.Roles[k] = v
			}
			delete(f.Roles, "Router")
		},
		func(f *PrivateFixtureRPCOptions) {
			f.Roles = map[string]deployment.Role{}
			for k, v := range fixture.Roles {
				f.Roles[k] = v
			}
			f.Roles["Extra"] = f.Roles["Router"]
		},
	} {
		changed := fixture
		change(&changed)
		if _, err := NewPrivateFixtureRPCQualification(ctx, client, changed); !errors.Is(err, ErrQualification) {
			t.Fatalf("unbound private deployment admitted %v", err)
		}
	}
	if _, err := NewPrivateFixtureRPCQualification(ctx, nil, fixture); !errors.Is(err, ErrQualification) {
		t.Fatalf("missing private node %v", err)
	}
	if _, err := NewPrivateFixtureRPCQualification(ctx, client, fixture); err == nil {
		t.Fatal("public deployment relabeled as private fixture")
	}
	if _, err := (&RPCQualification{}).QualifiedBatch(ctx, store.CanonicalBatch{}); !errors.Is(err, ErrBinding) {
		t.Fatalf("absent batch %v", err)
	}
	for _, raw := range []json.RawMessage{nil, json.RawMessage(`null`), json.RawMessage(`{}`)} {
		if _, _, err := authorityHeader(raw); err == nil {
			t.Fatalf("invalid header %s", raw)
		}
	}
}

func authorityHeader(raw json.RawMessage) (*types.Header, common.Hash, error) {
	return (&RPCQualification{}).identifiedHeader(raw)
}
