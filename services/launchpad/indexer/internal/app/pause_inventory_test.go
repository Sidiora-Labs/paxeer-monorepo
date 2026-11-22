package app

import (
	"math/big"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Sidiora-Technologies/KindleLaunch/core/indexer/internal/processor"
	"github.com/Sidiora-Technologies/KindleLaunch/core/indexer/internal/receipt"
	"github.com/Sidiora-Technologies/KindleLaunch/core/indexer/internal/source"
	"github.com/ethereum/go-ethereum/core/types"
)

func pauseInventoryBlock(t *testing.T, c *pauseChain) *types.Receipt {
	t.Helper()
	if err := c.client.Client().CallContext(c.ctx, nil, "evm_mine"); err != nil {
		t.Fatal(err)
	}
	header, err := c.client.HeaderByNumber(c.ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	block := &types.Receipt{BlockNumber: header.Number, BlockHash: header.Hash()}
	c.retain(t, block)
	return block
}

func TestPauseObservationInventoryCoveragePeriodicRuns(t *testing.T) {
	c := newPauseChain(t)
	address, _, created := c.create(t, "Inventory pause")
	c.retain(t, created)
	c.coverRaw()
	c.observation.inventoryCovered = true
	c.observation.discoveredEpochs = func() []receipt.RPCCodeEpoch { return nil }
	calls := pauseBoundedClient(t, c)
	c.refresh(t)
	base := c.observation.snapshot.Load().base
	workers := newRuntimeWorkers(nil)
	workers.AddPeriodic("pause_observations", time.Second, false, c.observation.refresh)
	if err := workers.Start(c.ctx); err != nil {
		t.Fatal(err)
	}
	defer workers.Close()
	server := httptest.NewServer(c.observation)
	defer server.Close()
	var blocks []*types.Receipt
	for range 3 {
		block := pauseInventoryBlock(t, c)
		blocks = append(blocks, block)
		deadline := time.NewTimer(5 * time.Second)
		tick := time.NewTicker(time.Millisecond)
		for c.observation.snapshot.Load().head.height < block.BlockNumber.Int64() {
			select {
			case <-deadline.C:
				t.Fatal("periodic observation did not reach the complete head")
			case <-tick.C:
			}
		}
		tick.Stop()
		deadline.Stop()
		if c.observation.snapshot.Load().base != base || time.Since(base.built) >= pauseBaseCycle || calls() != 1 {
			t.Fatal("inventory coverage must retain the base inside its cycle without pool logs")
		}
	}
	workers.Close()
	snapshot := c.observation.snapshot.Load()
	if len(snapshot.events) != 0 || len(c.observation.discoveredEpochs()) != 0 || len(snapshot.blocks) != int(snapshot.head.height-base.block.height+1) {
		t.Fatal("inventory snapshot must retain every complete block without discoveries or events")
	}
	for height := base.block.height; height <= snapshot.head.height; height++ {
		header, err := c.client.HeaderByNumber(c.ctx, big.NewInt(height))
		if err != nil {
			t.Fatal(err)
		}
		if snapshot.blocks[height] != strings.ToLower(header.Hash().Hex()) {
			t.Fatalf("complete block %d missing from the snapshot", height)
		}
	}
	assertPauseState(t, readPauseResponse(t, server, blocks[1]), address, false)
	if c.observation.snapshot.Load() != snapshot {
		t.Fatal("a request inside the snapshot must use its stored pool state")
	}
	above := pauseInventoryBlock(t, c)
	assertPauseState(t, readPauseResponse(t, server, above), address, false)
	if extended := c.observation.snapshot.Load(); extended.base != base || extended.head.height != above.BlockNumber.Int64() || calls() != 1 {
		t.Fatal("a request above the snapshot must extend inventory coverage without a periodic run")
	}
}

func TestPauseObservationInventoryCoverageEventsAndCreations(t *testing.T) {
	c := newPauseChain(t)
	address, pool, created := c.create(t, "Inventory history")
	c.retain(t, created)
	c.coverRaw()
	c.observation.inventoryCovered = true
	c.observation.discoveredEpochs = func() []receipt.RPCCodeEpoch { return nil }
	calls := pauseBoundedClient(t, c)
	c.refresh(t)
	base := c.observation.snapshot.Load().base
	beforePause := pauseInventoryBlock(t, c)
	paused := c.transact(t, pool, "pause")
	c.retain(t, paused)
	c.refresh(t)
	server := httptest.NewServer(c.observation)
	defer server.Close()
	assertPauseState(t, readPauseResponse(t, server, beforePause), address, false)
	assertPauseState(t, readPauseResponse(t, server, paused), address, true)
	if snapshot := c.observation.snapshot.Load(); snapshot.base != base || len(snapshot.events) != 1 || snapshot.events[0].BlockNumber != paused.BlockNumber.Uint64() {
		t.Fatal("pool pauses above the base must use the retained event at its block")
	}
	later, laterPool, laterCreated := c.create(t, "Inventory creation")
	laterPaused := c.transact(t, laterPool, "pause")
	c.retain(t, laterPaused)
	c.refresh(t)
	assertPauseState(t, readPauseResponse(t, server, laterCreated), later, false)
	assertPauseState(t, readPauseResponse(t, server, laterPaused), later, true)
	assertPauseState(t, readPauseResponse(t, server, beforePause), address, false)
	if _, ok := readPauseResponse(t, server, paused).Pools[strings.ToLower(later.Hex())]; ok {
		t.Fatal("inventory pool must be absent before its creation")
	}
	if c.observation.snapshot.Load().base != base || calls() != 1 || len(c.observation.discoveredEpochs()) != 0 {
		t.Fatal("new canonical pools must retain the base without discoveries or new pinned reads")
	}
}

func TestPauseObservationInventoryCoverageRequiresBeacon(t *testing.T) {
	c := newPauseChain(t)
	address, _, created := c.create(t, "Inventory beacon coverage")
	pools := []pausePool{{address: strings.ToLower(address.Hex()), height: created.BlockNumber.Int64()}}
	from, to := pools[0].height+1, pools[0].height+10
	retired := uint64(to)
	for _, test := range []struct {
		name   string
		epochs []source.AddressEpoch
		want   bool
	}{
		{"Missing", nil, false},
		{"WrongRole", []source.AddressEpoch{{Role: "SidioraPool"}}, false},
		{"StartsAfterRange", []source.AddressEpoch{{Role: "PoolBeacon", ActivationHeight: uint64(from + 1)}}, false},
		{"RetiresAtHead", []source.AddressEpoch{{Role: "PoolBeacon", RetirementHeight: &retired}}, false},
		{"CompleteRange", []source.AddressEpoch{{Role: "PoolBeacon", ActivationHeight: uint64(from)}}, true},
	} {
		t.Run(test.name, func(t *testing.T) {
			observer := &pauseObservations{inventoryCovered: true, epochs: test.epochs}
			if got := observer.rawCoverage(pools, nil, from, to); got != test.want {
				t.Fatalf("inventory coverage=%t want=%t", got, test.want)
			}
		})
	}
}

func TestPauseObservationInventoryCoverageRealm(t *testing.T) {
	for _, test := range []struct {
		realm string
		want  bool
	}{
		{valueProduction, true}, {valuePrivateFixture, false}, {"", false}, {"unreviewed", false},
	} {
		t.Run(test.realm, func(t *testing.T) {
			observer := &pauseObservations{}
			observer.configureInventoryCoverage(test.realm, &recoveringQualifiedSource{})
			if observer.inventoryCovered != test.want {
				t.Fatalf("realm %q inventory coverage=%t want=%t", test.realm, observer.inventoryCovered, test.want)
			}
			for _, batchSource := range []processor.QualifiedBatchSource{
				&source.LiveQualifiedSource{}, &source.HistoricalQualifiedSource{}, (*recoveringQualifiedSource)(nil), nil,
			} {
				observer := &pauseObservations{inventoryCovered: true}
				observer.configureInventoryCoverage(test.realm, batchSource)
				if observer.inventoryCovered {
					t.Fatalf("realm %q source %T must require explicit pool epochs", test.realm, batchSource)
				}
			}
		})
	}
}
