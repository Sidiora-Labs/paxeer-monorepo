package source

import (
	"context"
	"fmt"
	"math"
	"reflect"
	"testing"
	"time"

	"github.com/Sidiora-Technologies/KindleLaunch/core/indexer/internal/internaltest"
	"github.com/Sidiora-Technologies/KindleLaunch/core/indexer/internal/store"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
)

func TestSourceRangeIntegerRefusals(t *testing.T) {
	ctx := context.Background()
	options := fopts("")
	for _, tc := range []struct {
		name     string
		from, to int64
		value    int64
	}{
		{"negative first height", -1, 0, -1},
		{"minimum first height", math.MinInt64, 0, math.MinInt64},
		{"negative last height", 0, -1, -1},
		{"minimum last height", 0, math.MinInt64, math.MinInt64},
	} {
		t.Run(tc.name, func(t *testing.T) {
			want := fmt.Sprintf("integer %d is outside the range of uint64", tc.value)
			fetch := FetchOptions{FromBlock: tc.from, ToBlock: tc.to}
			lifecycle := newSourceLifecycle()
			got, done, err := lifecycle.beginRange(ctx, fetch, true, "fixture")
			if got != nil || done != nil || err == nil || err.Error() != want || len(lifecycle.active) != 0 {
				t.Fatalf("beginRange = %v, %v; want %q without active work", got, err, want)
			}
			filter, err := newExplorerEpochFilter(CommittedPaxscanOptions{ManifestDigest: options.ManifestDigest, Epochs: options.Epochs}, fetch, nil)
			if err == nil || err.Error() != want || !reflect.DeepEqual(filter, explorerEpochFilter{}) {
				t.Fatalf("newExplorerEpochFilter = %+v, %v; want %q", filter, err, want)
			}
		})
	}
}

func TestHistoricalIntegerRefusals(t *testing.T) {
	for _, tc := range []struct {
		name     string
		from, to uint64
		result   FetchResult
		want     string
	}{
		{"first height above signed bound", math.MaxInt64 + 1, math.MaxInt64 + 1, FetchResult{Complete: true}, "integer 9223372036854775808 is outside the range of int64"},
		{"first height unsigned bound", math.MaxUint64, math.MaxUint64, FetchResult{Complete: true}, "integer 18446744073709551615 is outside the range of int64"},
		{"log height above signed bound", 0, 0, FetchResult{Complete: true, Logs: []types.Log{{BlockNumber: math.MaxInt64 + 1}}}, "integer 9223372036854775808 is outside the range of int64"},
		{"log height unsigned bound", 0, 0, FetchResult{Complete: true, Logs: []types.Log{{BlockNumber: math.MaxUint64}}}, "integer 18446744073709551615 is outside the range of int64"},
		{"transaction index above bound", 0, 0, FetchResult{Complete: true, Logs: []types.Log{{TxIndex: math.MaxUint32 + 1}}}, "source historical-qualified: invalid_response"},
		{"log index above bound", 0, 0, FetchResult{Complete: true, Logs: []types.Log{{Index: math.MaxUint32 + 1}}}, "source historical-qualified: invalid_response"},
		{"incomplete range", 0, 0, FetchResult{}, "source historical-qualified: incomplete"},
		{"missing block", 0, 0, FetchResult{Complete: true}, "source historical-qualified: incomplete"},
		{"block height above signed bound", math.MaxInt64, math.MaxInt64 + 1, FetchResult{Complete: true, Blocks: map[uint64]BlockRecord{math.MaxInt64: {}, math.MaxInt64 + 1: {}}}, "integer 9223372036854775808 is outside the range of int64"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			batch, receipts, err := qualifyFetch(context.Background(), tc.result, nil, tc.from, tc.to, "historical-")
			if err == nil || err.Error() != tc.want || receipts != nil || !reflect.DeepEqual(batch, store.CanonicalBatch{}) {
				t.Fatalf("qualifyFetch = %+v, %v, %v; want %q", batch, receipts, err, tc.want)
			}
		})
	}
}

func TestParityIntegerRefusals(t *testing.T) {
	for _, to := range []uint64{math.MaxInt64, math.MaxUint64 - 1} {
		t.Run(fmt.Sprintf("range ending %d", to), func(t *testing.T) {
			want := fmt.Sprintf("integer %d is outside the range of int", to+1)
			if err := validateParityInput(ParityInput{}, 0, to); err == nil || err.Error() != want {
				t.Fatalf("validateParityInput = %v; want %q", err, want)
			}
			if err := ValidateParityAdmission(&ParityReport{ToHeight: to}, "fixture", "explorer"); err == nil || err.Error() != want {
				t.Fatalf("ValidateParityAdmission = %v; want %q", err, want)
			}
		})
	}
	input := ParityInput{SourceID: "explorer", TimestampUnit: "seconds", Result: FetchResult{
		Complete: true, ManifestDigest: "fixture", CoveredRanges: []CoveredRange{{FromHeight: 0, ToHeight: 0}},
		Blocks:          map[uint64]BlockRecord{0: {ChainID: 125, Height: 0, Hash: common.HexToHash("0x01"), TimestampSeconds: 1}},
		BlockTimestamps: map[uint64]int64{0: 1},
	}}
	for _, count := range []int{-1, math.MinInt} {
		input.Result.Blocks[0] = BlockRecord{ChainID: 125, Height: 0, Hash: common.HexToHash("0x01"), TimestampSeconds: 1, TransactionCount: count}
		if err := validateParityInput(input, 0, 0); err == nil || err.Error() != "source parity: incomplete" {
			t.Fatalf("negative block transaction count = %v", err)
		}
	}
}

func TestCapacityIntegerRefusals(t *testing.T) {
	ctx := context.Background()
	_, pool := internaltest.NewPostgres(t)
	providers := CapacityProviders{Live: new(CommittedSource), Historical: &HistoricalSource{Explorer: new(CommittedPaxscanSource)}}
	for _, tc := range []struct {
		name     string
		from, to uint64
		value    uint64
	}{
		{"first height above signed bound", math.MaxInt64 + 1, math.MaxInt64 + 1, math.MaxInt64 + 1},
		{"last height above signed bound", math.MaxInt64, math.MaxInt64 + 1, math.MaxInt64 + 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			want := fmt.Sprintf("integer %d is outside the range of int64", tc.value)
			latest, count, err := capacityDurable(ctx, pool, "capacity", tc.from, tc.to, FetchResult{}, nil, 0, nil, nil)
			if !latest.IsZero() || count != 0 || err == nil || err.Error() != want {
				t.Fatalf("capacityDurable = %v, %d, %v; want %q", latest, count, err, want)
			}
			report, err := capacityParity(ctx, providers, CapacityOptions{ParityFrom: tc.from, ParityTo: tc.to})
			if err == nil || err.Error() != want || !reflect.DeepEqual(report, ParityReport{}) {
				t.Fatalf("capacityParity = %+v, %v; want %q", report, err, want)
			}
			if pool.Stat().AcquiredConns() != 0 {
				t.Fatal("range refusal retained a database connection")
			}
		})
	}
}

func TestPaxscanLogIndexIntegerBounds(t *testing.T) {
	ctx := context.Background()
	dsn, pool := internaltest.NewPostgres(t)
	if _, err := pool.Exec(ctx, blockscoutSchema); err != nil {
		t.Fatal(err)
	}
	emitter := common.HexToAddress("0xee")
	if _, err := pool.Exec(ctx, `INSERT INTO logs(address_hash,first_topic,data,transaction_hash,block_number,index) VALUES($1,$2,'', $3,1,0)`, emitter.Bytes(), common.HexToHash("0x01").Bytes(), common.HexToHash("0x02").Bytes()); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO blocks VALUES(1,to_timestamp(1700000000),true)`); err != nil {
		t.Fatal(err)
	}
	src, err := NewPaxscan(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer src.Close()
	for _, index := range []int32{math.MinInt32, -1, 0, math.MaxInt32} {
		t.Run(fmt.Sprintf("index %d", index), func(t *testing.T) {
			if _, err := pool.Exec(ctx, `UPDATE logs SET index=$1`, index); err != nil {
				t.Fatal(err)
			}
			result, err := src.FetchLogs(ctx, FetchOptions{MonitoredAddresses: []string{emitter.Hex()}, FromBlock: 1, ToBlock: 1})
			if index < 0 {
				want := fmt.Sprintf("integer %d is outside the range of uint", index)
				if err == nil || err.Error() != want || !reflect.DeepEqual(result, FetchResult{}) {
					t.Fatalf("FetchLogs = %+v, %v; want %q", result, err, want)
				}
			} else if err != nil || len(result.Logs) != 1 || result.Logs[0].Index != uint(index) || result.BlockTimestamps[1] != 1700000000 {
				t.Fatalf("FetchLogs index %d = %+v, %v", index, result, err)
			}
			if src.pool.Stat().AcquiredConns() != 0 {
				t.Fatal("log index refusal retained SQL rows")
			}
		})
	}
	closed, cancel := context.WithTimeout(ctx, time.Second)
	defer cancel()
	if err := src.pool.Ping(closed); err != nil {
		t.Fatal(err)
	}
}

func TestArchiveRangeIntegerRefusals(t *testing.T) {
	s := new(CommittedSource)
	for _, tc := range []struct {
		name     string
		from, to int64
		value    int64
	}{
		{"negative height", -1, 0, -1},
		{"minimum height", math.MinInt64, 0, math.MinInt64},
		{"reversed range", 2, 0, -1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			logs, err := s.archiveLogs(context.Background(), nil, tc.from, tc.to, []string{common.HexToAddress("0x01").Hex()}, true, nil)
			want := fmt.Sprintf("integer %d is outside the range of uint64", tc.value)
			if logs != nil || err == nil || err.Error() != want {
				t.Fatalf("archiveLogs = %v, %v; want %q", logs, err, want)
			}
		})
	}
	if logs, err := s.archiveLogs(context.Background(), nil, 0, 0, nil, true, nil); logs != nil || err != nil {
		t.Fatalf("archive without monitored addresses = %v, %v", logs, err)
	}
	endpoint := &committedEndpoint{id: "archive"}
	logs, err := s.receiptLogs(context.Background(), endpoint, blockWire{Transactions: make([]ethTxWire, 4097)}, BlockRecord{}, 0, nil, true)
	if logs != nil || err == nil || err.Error() != "source archive: incomplete" {
		t.Fatalf("oversized transaction count = %v, %v", logs, err)
	}
}
