package store

import (
	"context"
	"encoding/json"
	"errors"
	"math"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/Sidiora-Technologies/KindleLaunch/shared/accounting"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/puddle/v2"
)

func TestStoredIntegerBoundsPreserveRawCoordinates(t *testing.T) {
	ctx := context.Background()
	st, pool := lifecycleStore(t)
	for _, coordinates := range []struct {
		height int64
		index  uint32
	}{{0, 0}, {math.MaxInt64, math.MaxUint32}} {
		tx, err := pool.Begin(ctx)
		if err != nil {
			t.Fatal(err)
		}
		raw := lifecycleRaw()
		raw.Height, raw.TransactionIndex, raw.LogIndex = coordinates.height, coordinates.index, coordinates.index
		if err := stageImmutableRaw(ctx, tx, 125, raw); err != nil {
			t.Fatal(err)
		}
		var height, transactionIndex, logIndex int64
		if err := tx.QueryRow(ctx, `SELECT height,transaction_index,log_index FROM indexer.indexer_raw_logs WHERE transaction_hash=$1`, raw.TransactionHash).Scan(&height, &transactionIndex, &logIndex); err != nil {
			t.Fatal(err)
		}
		if height != coordinates.height || transactionIndex != int64(coordinates.index) || logIndex != int64(coordinates.index) {
			t.Fatalf("stored coordinates = %d/%d/%d; want %+v", height, transactionIndex, logIndex, coordinates)
		}
		if err := tx.Rollback(ctx); err != nil {
			t.Fatal(err)
		}
		canonical := CanonicalTx{Tx: tx, Store: st}
		for _, tc := range []struct {
			name string
			call func() error
		}{
			{"raw log", func() error { return stageImmutableRaw(ctx, tx, 125, raw) }},
			{"source poison", func() error {
				return canonical.ResolveSourceQualifiedPoisonTx(ctx, CanonicalBatch{ChainID: 125, StreamID: "integer", Records: []RawRecord{raw}})
			}},
			{"inventory intent", func() error {
				return canonical.InsertCommittedOutboxIntent(ctx, CanonicalBatch{ChainID: 125, PreviousHeight: coordinates.height}, OutboxIntent{TransactionHash: raw.TransactionHash, LogIndex: coordinates.index, Kind: accounting.InventorySubject, Payload: json.RawMessage(`{}`)})
			}},
		} {
			if err := tc.call(); !errors.Is(err, pgx.ErrTxClosed) {
				t.Fatalf("%s after rollback = %v; want %v", tc.name, err, pgx.ErrTxClosed)
			}
		}
	}
}

func TestStoredIntegerRangesRejectUnavailableDatabase(t *testing.T) {
	ctx := context.Background()
	st, pool := lifecycleStore(t)
	raw := lifecycleRaw()
	block := BlockCompleteness{Height: raw.Height, Hash: raw.BlockHash, FetchComplete: true, RequiredDecodeComplete: true}
	segment := ReplaySegment{FormatVersion: "raw-replay-v2", ChainID: 125, SourceStream: "integer", FromHeight: raw.Height, ToHeight: raw.Height, Blocks: []BlockCompleteness{block}}
	request := ReplayRequest{JobID: "integer", SourceStream: "integer", Target: "market", Generation: "g1", ProjectionRevision: "v1", Mode: "replay", Requester: "operator", AuthorizationID: "authorization", FromHeight: raw.Height, ToHeight: raw.Height, CheckpointHeight: raw.Height, CheckpointHash: raw.BlockHash}
	pool.Close()
	for _, tc := range []struct {
		name string
		call func() error
	}{
		{"archive source", func() error { return st.CheckArchiveSource(ctx, segment) }},
		{"archive lookup", func() error {
			m, err := st.FindArchive(ctx, "integer", 0, math.MaxInt64)
			if !reflect.DeepEqual(m, ArchiveManifest{}) {
				t.Fatalf("partial manifest: %+v", m)
			}
			return err
		}},
		{"retention", func() error {
			eligible, err := st.RetentionEligible(ctx, OperationScope{Action: "retention", From: 0, To: math.MaxInt64})
			if eligible {
				t.Fatal("unavailable retention source admitted")
			}
			return err
		}},
		{"poison", func() error {
			return st.RetainPoison(ctx, PoisonRecord{ChainID: 125, StreamID: "integer", Raw: raw, ErrorCode: "decode", ABIRevision: "v1", Requirement: "required"})
		}},
		{"replay admission", func() error { return st.AdmitReplay(ctx, request) }},
		{"replay lease", func() error {
			job, err := st.AcquireReplay(ctx, "integer", "worker", time.Second)
			if !reflect.DeepEqual(job, ReplayJob{}) {
				t.Fatalf("partial job: %+v", job)
			}
			return err
		}},
		{"replay conflict", func() error { return st.RecordReplayConflict(ctx, request, segment, "source identity differs") }},
		{"operation authorization", func() error { return st.CheckOperationAuthorization(ctx, request.Scope()) }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if err := tc.call(); !errors.Is(err, puddle.ErrClosedPool) {
				t.Fatalf("closed database = %v; want %v", err, puddle.ErrClosedPool)
			}
		})
	}
}

func TestArchiveIntegerBoundsRefuseInvalidCounts(t *testing.T) {
	base := ArchiveManifest{FormatVersion: "raw-archive-v2", ChainID: 125, SourceStream: "integer", FromHeight: 0, ToHeight: 0, ByteLength: 1, SHA256: strings.Repeat("a", 64), Bucket: "archive", ObjectKey: "indexer/raw/v2/integer/" + strings.Repeat("a", 64) + ".json", Blocks: []BlockCompleteness{{Height: 0, Hash: lifecycleRaw().BlockHash, FetchComplete: true, RequiredDecodeComplete: true}}}
	if err := base.Validate(); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name string
		edit func(*ArchiveManifest)
	}{
		{"negative height", func(m *ArchiveManifest) { m.FromHeight = -1 }},
		{"maximum signed height", func(m *ArchiveManifest) { m.ToHeight = math.MaxInt64 }},
		{"negative event count", func(m *ArchiveManifest) { m.EventCount = -1 }},
		{"event count above bound", func(m *ArchiveManifest) { m.EventCount = 65537 }},
		{"negative typed count", func(m *ArchiveManifest) { m.TypedCount = -1 }},
		{"typed count above bound", func(m *ArchiveManifest) { m.TypedCount = 65537 }},
		{"empty bytes", func(m *ArchiveManifest) { m.ByteLength = 0 }},
		{"bytes above bound", func(m *ArchiveManifest) { m.ByteLength = 64<<20 + 1 }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			m := base
			tc.edit(&m)
			if err := m.Validate(); err == nil || err.Error() != "invalid bounded archive manifest" {
				t.Fatalf("Validate = %v", err)
			}
		})
	}
	raw := lifecycleRaw()
	raw.Topics = []string{"invalid"}
	if digest, err := RawDigest(raw); digest != "" || err == nil || err.Error() != "invalid raw topic" {
		t.Fatalf("RawDigest = %s, %v", digest, err)
	}
}

func TestStoredRangeCallersPreserveIntegerRefusals(t *testing.T) {
	ctx := context.Background()
	st, pool := lifecycleStore(t)
	raw := lifecycleRaw()
	block := BlockCompleteness{Height: raw.Height, Hash: raw.BlockHash, FetchComplete: true, RequiredDecodeComplete: true}
	segment := ReplaySegment{FormatVersion: "raw-replay-v2", ChainID: 125, SourceStream: "integer", FromHeight: raw.Height, ToHeight: raw.Height, Blocks: []BlockCompleteness{block}}
	invalidSegment := segment
	invalidSegment.FromHeight = math.MinInt64
	manifest := ArchiveManifest{FormatVersion: "raw-archive-v2", ChainID: 125, SourceStream: "integer", FromHeight: raw.Height, ToHeight: raw.Height, ByteLength: 1, SHA256: strings.Repeat("a", 64), Bucket: "archive", ObjectKey: "indexer/raw/v2/integer/" + strings.Repeat("a", 64) + ".json", Blocks: []BlockCompleteness{block}}
	request := ReplayRequest{JobID: "integer", SourceStream: "integer", Target: "market", Generation: "g1", ProjectionRevision: "v1", Mode: "replay", Requester: "operator", AuthorizationID: "authorization", FromHeight: raw.Height, ToHeight: raw.Height, CheckpointHeight: raw.Height, CheckpointHash: raw.BlockHash}
	invalidRequest := request
	invalidRequest.FromHeight = math.MinInt64
	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(ctx)
	for _, tc := range []struct {
		name, want string
		call       func() error
	}{
		{"archive source range", "invalid bounded replay segment", func() error { return st.CheckArchiveSource(ctx, invalidSegment) }},
		{"archive completion manifest", "invalid bounded archive manifest", func() error { bad := manifest; bad.ByteLength = -1; return st.CompleteArchive(ctx, bad, segment) }},
		{"archive completion range", "invalid bounded replay segment", func() error { return st.CompleteArchive(ctx, manifest, invalidSegment) }},
		{"reconstruction range", "invalid bounded replay segment", func() error { _, _, err := st.CompareReconstructionSegment(ctx, request, invalidSegment); return err }},
		{"reconstruction interval", ErrIntegrity.Error(), func() error {
			other := request
			other.FromHeight = raw.Height + 1
			_, _, err := st.CompareReconstructionSegment(ctx, other, segment)
			return err
		}},
		{"replay admission range", "invalid replay bounds, mode or source checkpoint", func() error { return st.AdmitReplay(ctx, invalidRequest) }},
		{"replay conflict range", "invalid replay bounds, mode or source checkpoint", func() error { return st.RecordReplayConflict(ctx, invalidRequest, segment, "source identity differs") }},
		{"zero lease duration", "bounded replay lease required", func() error { _, err := st.AcquireReplay(ctx, "integer", "worker", 0); return err }},
		{"lease duration above bound", "bounded replay lease required", func() error {
			_, err := st.AcquireReplay(ctx, "integer", "worker", 30*time.Second+time.Nanosecond)
			return err
		}},
		{"checkpoint height", ErrIntegrity.Error(), func() error {
			other := block
			other.Height = math.MaxInt64
			return st.BindReplayCheckpoint(ctx, request, other)
		}},
		{"source repair transaction", "source repair requires actual canonical transaction", func() error {
			return (CanonicalTx{}).ResolveSourceQualifiedPoisonTx(ctx, CanonicalBatch{Records: []RawRecord{raw}})
		}},
		{"source repair height", "invalid bounded raw identity", func() error {
			bad := raw
			bad.Height = -1
			return (CanonicalTx{Tx: tx}).ResolveSourceQualifiedPoisonTx(ctx, CanonicalBatch{Records: []RawRecord{bad}})
		}},
		{"raw chain", ErrIntegrity.Error(), func() error { return stageImmutableRaw(ctx, tx, -1, raw) }},
		{"raw height", "invalid bounded raw identity", func() error { bad := raw; bad.Height = math.MinInt64; return stageImmutableRaw(ctx, tx, 125, bad) }},
		{"poison classification", "complete quarantine classification required", func() error { return st.RetainPoison(ctx, PoisonRecord{ChainID: -1}) }},
		{"poison raw height", "invalid bounded raw identity", func() error {
			bad := raw
			bad.Height = -1
			return st.RetainPoison(ctx, PoisonRecord{ChainID: 125, StreamID: "integer", Raw: bad, ErrorCode: "decode", ABIRevision: "v1", Requirement: "required"})
		}},
		{"poison repair scope", ErrIntegrity.Error(), func() error { return st.ResolvePoison(ctx, OperationScope{}, "", raw.BlockHash) }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if err := tc.call(); err == nil || err.Error() != tc.want {
				t.Fatalf("refusal = %v; want %q", err, tc.want)
			}
		})
	}
}
