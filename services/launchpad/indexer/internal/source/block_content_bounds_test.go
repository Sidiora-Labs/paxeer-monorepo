package source

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/Sidiora-Technologies/KindleLaunch/core/indexer/internal/decode"
	"github.com/Sidiora-Technologies/KindleLaunch/core/indexer/internal/internaltest/chainfixture"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	ethtypes "github.com/ethereum/go-ethereum/core/types"
)

func contentSource(t *testing.T, b *chainfixture.ShellBlock, fallback bool) *CommittedSource {
	t.Helper()
	manifest := strings.Repeat("a", 64)
	height := b.Header.Number.Uint64()
	address := b.Receipts[0].Logs[0].Address
	s, err := NewCommittedSource(context.Background(), CommittedOptions{
		Endpoints:     map[string]string{"primary": b.ServeContent(t, fallback)},
		Anchor:        &CommittedAnchor{Height: height - 1, Hash: b.AnchorHash},
		BlockIdentity: valueReported, ManifestDigest: manifest,
		Epochs: []AddressEpoch{{Address: address.Hex(), Role: "EventEmitter", ABIRevision: "recorded", ManifestDigest: manifest, ActivationHeight: height}},
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := s.Close(); err != nil {
			t.Error(err)
		}
	})
	return s
}

func checkContentSource(t *testing.T, transactions, logs, dataBytes int) {
	t.Helper()
	b := chainfixture.NewContentBlock(t, transactions, logs, make([]byte, dataBytes))
	height := b.Header.Number.Uint64()
	hashes := make([]common.Hash, len(b.Transactions))
	for i, tx := range b.Transactions {
		hashes[i] = tx.Hash()
	}
	regular, err := separateShellReceipts(b.Receipts, hashes, b.Hash, height, hexutil.Encode(b.Header.Bloom[:]), b.Header.ReceiptHash.Hex(), false)
	if err != nil || len(regular) != transactions || len(regular[0].Logs) != logs {
		t.Fatalf("source shell content: receipts=%d: %v", len(regular), err)
	}
	ctx := context.Background()
	for _, fallback := range []bool{false, true} {
		s := contentSource(t, b, fallback)
		endpoint := s.endpoints[0]
		wire, record, err := s.block(ctx, endpoint, fmtHeight(height), false)
		if err != nil || len(wire.Transactions) != transactions {
			t.Fatalf("source block content: %v", err)
		}
		address := b.Receipts[0].Logs[0].Address
		archive, err := s.receiptLogs(ctx, endpoint, wire, record, height, []common.Address{address}, false)
		if err != nil || len(archive) != logs || len(archive[0].Data) != dataBytes {
			t.Fatalf("archive content: logs=%d: %v", len(archive), err)
		}
		got, err := s.logs(ctx, endpoint, int64(height), int64(height), []string{strings.ToLower(address.Hex())}, false, map[uint64]batchBlock{height: {wire, record}}, func() error { return nil })
		if err != nil || len(got) != logs || len(got[0].Data) != dataBytes {
			t.Fatalf("committed content fallback=%t: logs=%d: %v", fallback, len(got), err)
		}
	}
}

func TestBlockContentBoundsSource(t *testing.T) {
	for _, tc := range []struct {
		name                          string
		transactions, logs, dataBytes int
	}{{"transactions", 4097, 1, (64 << 10) + 1}, {"receipt logs", 1, 4097, 0}, {"log data", 1, 1, (64 << 10) + 1}} {
		t.Run(tc.name, func(t *testing.T) { checkContentSource(t, tc.transactions, tc.logs, tc.dataBytes) })
	}
}

func TestBlockContentBoundsSourceBytes(t *testing.T) {
	b := chainfixture.NewContentBlock(t, 1, 1, nil)
	shared := make([]byte, 1<<20)
	base := *b.Receipts[0].Logs[0]
	b.Receipts[0].Logs = make([]*ethtypes.Log, decode.MaxBlockLogBytes/(256+len(shared))+1)
	for i := range b.Receipts[0].Logs {
		log := base
		log.Index, log.Data = uint(i), shared
		b.Receipts[0].Logs[i] = &log
	}
	_, err := separateShellReceipts(b.Receipts, []common.Hash{b.Transactions[0].Hash()}, b.Hash, b.Header.Number.Uint64(), hexutil.Encode(b.Header.Bloom[:]), b.Header.ReceiptHash.Hex(), true)
	var se *SourceError
	if !errors.As(err, &se) || se.Kind != OversizeError {
		t.Fatalf("source block byte bound: %v", err)
	}
}

func TestBlockContentBoundsHistoricalMerge(t *testing.T) {
	for _, tc := range []struct {
		name            string
		logs, dataBytes int
	}{{"logs", 131073, 0}, {"bytes", 129, 1 << 20}} {
		t.Run(tc.name, func(t *testing.T) {
			b := chainfixture.NewContentBlock(t, 1, 1, nil)
			height := b.Header.Number.Uint64()
			record := BlockRecord{ChainID: 125, Height: height, Hash: b.Hash, ParentHash: b.Header.ParentHash, TimestampSeconds: int64(b.Header.Time), TransactionCount: 1}
			page := FetchResult{ManifestDigest: "fixture", Complete: true, CoveredRanges: []CoveredRange{{height, height}}, Blocks: map[uint64]BlockRecord{height: record}, BlockTimestamps: map[uint64]int64{height: record.TimestampSeconds}, TxFromMap: map[string]string{}, Logs: make([]ethtypes.Log, tc.logs)}
			shared := make([]byte, tc.dataBytes)
			for i := range page.Logs {
				page.Logs[i] = *b.Receipts[0].Logs[0]
				page.Logs[i].Index, page.Logs[i].Data = uint(i), shared
			}
			out := FetchResult{ManifestDigest: "fixture", Blocks: map[uint64]BlockRecord{}, BlockTimestamps: map[uint64]int64{}, TxFromMap: map[string]string{}}
			if err := mergeFetch(&out, page); err != nil || len(out.Logs) != tc.logs {
				t.Fatalf("single block merge: logs=%d: %v", len(out.Logs), err)
			}
			input := ParityInput{SourceID: "rpc", TimestampUnit: "seconds", Result: page}
			if err := validateParityInput(input, height, height); err != nil {
				t.Fatalf("single block parity: %v", err)
			}
			next := record
			next.Height, next.ParentHash = height+1, b.Hash
			next.Hash[0] ^= 1
			second := FetchResult{ManifestDigest: "fixture", Complete: true, CoveredRanges: []CoveredRange{{height + 1, height + 1}}, Blocks: map[uint64]BlockRecord{height + 1: next}, BlockTimestamps: map[uint64]int64{height + 1: next.TimestampSeconds}, TxFromMap: map[string]string{}}
			var se *SourceError
			if err := mergeFetch(&out, second); !errors.As(err, &se) || se.Kind != OversizeError {
				t.Fatalf("multi-block merge bound: %v", err)
			}
		})
	}
}
