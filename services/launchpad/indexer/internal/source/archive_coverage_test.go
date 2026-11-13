package source

import (
	"context"
	"math/big"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
)

func TestArchiveReceiptsMatchGenuineChainAndReadAhead(t *testing.T) {
	ctx, client, endpoint := genuineHistoricalNode(t)
	key, err := crypto.GenerateKey()
	if err != nil {
		t.Fatal(err)
	}
	sender := crypto.PubkeyToAddress(key.PublicKey)
	if err = client.Client().CallContext(ctx, nil, "anvil_setBalance", sender.Hex(), "0x3635c9adc5dea00000"); err != nil {
		t.Fatal(err)
	}
	parsed, bytecode, revision, manifest := historicalArtifact(t)
	auth, err := bind.NewKeyedTransactorWithChainID(key, big.NewInt(125))
	if err != nil {
		t.Fatal(err)
	}
	auth.Context = ctx
	token, tx, _, err := bind.DeployContract(auth, parsed, bytecode, client, "Archive", "ARC", big.NewInt(1000), sender)
	if err != nil {
		t.Fatal(err)
	}
	mined, err := bind.WaitMined(ctx, client, tx)
	if err != nil || len(mined.Logs) != 1 {
		t.Fatalf("token deployment %v %v", mined, err)
	}
	if err = client.Client().CallContext(ctx, nil, "anvil_mine", "0x2"); err != nil {
		t.Fatal(err)
	}
	genesis, err := client.HeaderByNumber(ctx, big.NewInt(0))
	if err != nil {
		t.Fatal(err)
	}
	s, err := NewCommittedSource(ctx, CommittedOptions{Endpoints: map[string]string{"primary": endpoint}, HistoricalEndpoints: map[string]string{"archive": endpoint}, GenesisHash: genesis.Hash(), ManifestDigest: manifest, Epochs: []AddressEpoch{{Address: token.Hex(), Role: "SidioraERC20", ABIRevision: revision, ManifestDigest: manifest, ActivationHeight: 1}}, HistoricalReadahead: 3})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := s.Close(); err != nil {
			t.Error(err)
		}
	})
	var primary, archive *committedEndpoint
	for _, e := range s.endpoints {
		if e.historical {
			archive = e
		} else {
			primary = e
		}
	}
	if primary == nil || archive == nil {
		t.Fatal("archive endpoints not admitted")
	}
	expected, err := s.fetch(ctx, primary, FetchOptions{FromBlock: 1, ToBlock: 3}, true, s.opts.Epochs)
	if err != nil {
		t.Fatal(err)
	}
	s.ReadAhead(1, 3)
	s.ReadAhead(1, 3)
	actual, err := s.fetch(ctx, archive, FetchOptions{FromBlock: 1, ToBlock: 3}, true, s.opts.Epochs)
	if err != nil || !actual.Complete || len(actual.Logs) != 1 || !reflect.DeepEqual(actual.Logs, expected.Logs) || s.Metrics().ReadaheadHits != 3 || s.Metrics().BloomSkipped < 2 {
		t.Fatalf("archive parity logs=%#v expected=%#v metrics=%+v err=%v", actual.Logs, expected.Logs, s.Metrics(), err)
	}
	wire, record, err := s.block(ctx, archive, "0x1", true)
	if err != nil {
		t.Fatal(err)
	}
	for _, mode := range []string{"keccak", "reported"} {
		s.opts.BlockIdentity = mode
		logs, err := s.receiptLogs(ctx, archive, wire, record, 1, []common.Address{token}, true)
		if err != nil || len(logs) != 1 || logs[0].TxHash != tx.Hash() || logs[0].Address != token {
			t.Fatalf("receipt identity %s: %+v %v", mode, logs, err)
		}
	}
	for _, field := range []string{"root", "transaction", "count", "height", "bloom"} {
		t.Run(field, func(t *testing.T) {
			changed := wire
			changed.Transactions = append([]ethTxWire(nil), wire.Transactions...)
			changedRecord := record
			switch field {
			case "root":
				changed.ReceiptsRoot = "invalid"
			case "transaction":
				changed.Transactions[0].Hash = "invalid"
			case "count":
				changed.Transactions = nil
			case "height":
				changedRecord.Hash = common.HexToHash("0xbeef")
			case "bloom":
				changed.LogsBloom = "0x" + strings.Repeat("0", 512)
			}
			if _, err := s.receiptLogs(ctx, archive, changed, changedRecord, 1, []common.Address{token}, true); err == nil {
				t.Fatal("unbound receipt input accepted")
			}
		})
	}
	s.ahead.mu.Lock()
	s.ahead.entries[1] = &aheadEntry{endpoint: archive.id, done: make(chan struct{})}
	s.ahead.mu.Unlock()
	cancelled, cancel := context.WithCancel(ctx)
	cancel()
	if _, ok := s.takeAhead(cancelled, archive, 1); ok {
		t.Fatal("cancelled prefetch accepted")
	}
	finished := make(chan struct{})
	close(finished)
	s.ahead.mu.Lock()
	s.ahead.entries[1] = &aheadEntry{endpoint: archive.id, done: finished}
	s.ahead.mu.Unlock()
	s.dropAheadBelow(2)
	s.ahead.mu.Lock()
	remaining := len(s.ahead.entries)
	s.ahead.mu.Unlock()
	if remaining != 0 {
		t.Fatalf("completed old prefetch retained: %d", remaining)
	}
	if _, ok := s.takeAhead(ctx, archive, 1); ok {
		t.Fatal("absent prefetch accepted")
	}
	s.ReadAhead(3, 2)
	primary.floor = 3
	if floor := s.ArchiveFloor(); floor != 3 {
		t.Fatalf("archive floor %d", floor)
	}
	primary.floor = 0
	if floor := s.ArchiveFloor(); floor != 0 {
		t.Fatalf("unreported floor %d", floor)
	}
	s.opts.HistoricalReadahead = -1
	if s.ReadaheadDepth() != 0 {
		t.Fatal("disabled prefetch enabled")
	}
	s.ReadAhead(1, 3)
}

func TestArchiveReadaheadStopsWithSourceLifecycle(t *testing.T) {
	node := repair997Serve(t)
	opts := fopts(node.server.URL)
	opts.HistoricalEndpoints = map[string]string{"archive": node.server.URL}
	opts.HistoricalReadahead = 2
	s := repair997Open(t, opts)
	s.ReadAhead(1, 2)
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
	s.ReadAhead(3, 4)
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	for _, e := range s.endpoints {
		if e.historical {
			if _, ok := s.takeAhead(ctx, e, 3); ok {
				t.Fatal("closed source prefetched a block")
			}
		}
	}
}
