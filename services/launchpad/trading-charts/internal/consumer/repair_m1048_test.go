package consumer

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Sidiora-Technologies/KindleLaunch/core/trading-charts/internal/engine"
	"github.com/Sidiora-Technologies/KindleLaunch/core/trading-charts/internal/store"
	"github.com/Sidiora-Technologies/KindleLaunch/shared/accounting"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/jackc/pgx/v5"
)

// The fixture is a transactional in-memory peer, persisted to a file for the
// restart proof. It stores source facts and applied publications, not answers
// or callbacks for the refresh component. Source authorization, checkpoint
// selection, RPC reads, scheduling and completeness calculation are real code.
type repair1048SourceFill struct {
	Fill                          accounting.FillV1
	Digest                        accounting.Digest
	Raw                           accounting.SourcePosition
	Emitter, CompleteHash         string
	FetchComplete, DecodeComplete bool
	PreviousHeight, LastHeight    int64
}
type repair1048Applied struct {
	Wire, Fill []byte
	Frozen     bool
	Clock      engine.SeriesCheckpoint
	Revision   uint64
}
type repair1048State struct {
	Watermark    sourceWatermark
	Fills        map[string]repair1048SourceFill
	Publications map[string][]byte
	Positions    map[string]map[int64][]byte
	Applied      map[string]repair1048Applied
	BlockHashes  map[int64]string
	Unresolved   bool
}
type repair1048Store struct {
	mu        sync.Mutex
	state     repair1048State
	path      string
	rollbacks atomic.Int64
}
type repair1048Transaction struct {
	owner  *repair1048Store
	state  repair1048State
	closed bool
}

func repair1048JSON(v any) []byte {
	wire, err := json.Marshal(v)
	if err != nil {
		panic(err)
	}
	return wire
}
func (s *repair1048Store) save() error { return os.WriteFile(s.path, repair1048JSON(s.state), 0600) }
func (s *repair1048Store) mutate(t *testing.T, f func(*repair1048State)) {
	t.Helper()
	s.mu.Lock()
	defer s.mu.Unlock()
	f(&s.state)
	if err := s.save(); err != nil {
		t.Fatal(err)
	}
}
func (s *repair1048Store) Publications(ctx context.Context, after string, limit int) ([]persistedPoolPublication, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	pools := []string{}
	for pool := range s.state.Applied {
		if pool > after {
			pools = append(pools, pool)
		}
	}
	sort.Strings(pools)
	if len(pools) > limit {
		pools = pools[:limit]
	}
	entries := []persistedPoolPublication{}
	for _, pool := range pools {
		entries = append(entries, persistedPoolPublication{Pool: pool, Wire: append([]byte(nil), s.state.Applied[pool].Wire...)})
	}
	return entries, nil
}
func (s *repair1048Store) Begin(ctx context.Context) (completenessTransaction, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	s.mu.Lock()
	var state repair1048State
	if err := json.Unmarshal(repair1048JSON(s.state), &state); err != nil {
		s.mu.Unlock()
		return nil, err
	}
	return &repair1048Transaction{owner: s, state: state}, nil
}
func (tx *repair1048Transaction) Watermark(context.Context) (sourceWatermark, error) {
	return tx.state.Watermark, nil
}
func (tx *repair1048Transaction) MatchesFill(_ context.Context, f accounting.FillV1, digest accounting.Digest, body []byte) (bool, error) {
	source, ok := tx.state.Fills[f.SourceFillID]
	if !ok {
		return false, pgx.ErrNoRows
	}
	height, err := strconv.ParseInt(f.Position.Height, 10, 64)
	if err != nil {
		return false, err
	}
	return source.Digest == digest && bytes.Equal(repair1048JSON(source.Fill), body) && source.Raw == f.Position && source.Emitter == f.ActualEmitter && source.CompleteHash == f.Position.BlockHash && source.FetchComplete && source.DecodeComplete && source.PreviousHeight < height && source.LastHeight >= height, nil
}
func (tx *repair1048Transaction) MatchesPublication(_ context.Context, p accounting.PublicationV1, body, position []byte) (bool, error) {
	raw, ok := tx.state.Publications[p.PublicationID]
	if !ok {
		return false, pgx.ErrNoRows
	}
	sequence, err := strconv.ParseInt(p.Positions[0].Sequence, 10, 64)
	if err != nil {
		return false, err
	}
	return bytes.Equal(raw, body) && bytes.Equal(tx.state.Positions[p.Fill.PoolAddress][sequence], position), nil
}
func (tx *repair1048Transaction) Head(_ context.Context, pool string) (int64, error) {
	var head int64
	for sequence := range tx.state.Positions[pool] {
		if sequence > head {
			head = sequence
		}
	}
	return head, nil
}
func (tx *repair1048Transaction) Clock(_ context.Context, pool string) (engine.SeriesCheckpoint, error) {
	applied, ok := tx.state.Applied[pool]
	if !ok {
		return engine.SeriesCheckpoint{}, pgx.ErrNoRows
	}
	return applied.Clock, nil
}
func (tx *repair1048Transaction) BlockHash(_ context.Context, height int64) (string, error) {
	hash, ok := tx.state.BlockHashes[height]
	if !ok {
		return "", pgx.ErrNoRows
	}
	return hash, nil
}
func (tx *repair1048Transaction) Applied(_ context.Context, p accounting.PublicationV1) error {
	applied, ok := tx.state.Applied[p.Fill.PoolAddress]
	if !ok || applied.Frozen || !bytes.Equal(applied.Wire, repair1048JSON(p)) || !bytes.Equal(applied.Fill, repair1048JSON(*p.Fill)) {
		return ErrAccountingAuthority
	}
	return nil
}
func (tx *repair1048Transaction) Unresolved(context.Context) (bool, error) {
	return tx.state.Unresolved, nil
}
func (tx *repair1048Transaction) Advance(_ context.Context, pool string, checkpoint engine.SeriesCheckpoint) error {
	applied := tx.state.Applied[pool]
	if checkpoint.Height < applied.Clock.Height || checkpoint.Timestamp < applied.Clock.Timestamp {
		return store.ErrSourceWatermarkRegression
	}
	applied.Clock = checkpoint
	applied.Revision++
	tx.state.Applied[pool] = applied
	return nil
}
func (tx *repair1048Transaction) Commit(context.Context) error {
	if tx.closed {
		return pgx.ErrTxClosed
	}
	tx.owner.state = tx.state
	err := tx.owner.save()
	tx.closed = true
	tx.owner.mu.Unlock()
	return err
}
func (tx *repair1048Transaction) Rollback(context.Context) error {
	if tx.closed {
		return pgx.ErrTxClosed
	}
	tx.closed = true
	tx.owner.rollbacks.Add(1)
	tx.owner.mu.Unlock()
	return nil
}

func repair1048Publication(t *testing.T, fill accounting.FillV1, sequence int64, predecessor string) accounting.PublicationV1 {
	t.Helper()
	digest, err := accounting.FillDigest(fill)
	if err != nil {
		t.Fatal(err)
	}
	position := accounting.PartitionPosition{ProjectionVersion: "market-v1", PartitionKind: "market", PartitionKey: "125:" + fill.PoolAddress, Sequence: strconv.FormatInt(sequence, 10), PreviousEffectIdentity: predecessor, SourceEventID: fill.SourceFillID, EffectIdentity: accounting.EffectIdentity(fill.SourceFillID, "market", "market-v1", "", "market", "125:"+fill.PoolAddress), PayloadDigest: digest, SourceOrder: fill.Position}
	p, err := accounting.NewMarketPublication(fill, []accounting.PartitionPosition{position})
	if err != nil {
		t.Fatal(err)
	}
	return p
}
func repair1048Record(t *testing.T, state *repair1048State, p accounting.PublicationV1, applied bool) {
	t.Helper()
	wire, err := accounting.CanonicalPublicationV1(p)
	if err != nil {
		t.Fatal(err)
	}
	// The same real producer wire crosses the persisted transport and decoder.
	decoded, err := accounting.DecodePublicationV1(wire)
	if err != nil {
		t.Fatal(err)
	}
	f := *decoded.Fill
	height, _ := strconv.ParseInt(f.Position.Height, 10, 64)
	state.Fills[f.SourceFillID] = repair1048SourceFill{Fill: f, Digest: p.SourceFactDigest, Raw: f.Position, Emitter: f.ActualEmitter, CompleteHash: f.Position.BlockHash, FetchComplete: true, DecodeComplete: true, PreviousHeight: height - 1, LastHeight: height}
	state.Publications[p.PublicationID] = wire
	if state.Positions[f.PoolAddress] == nil {
		state.Positions[f.PoolAddress] = map[int64][]byte{}
	}
	sequence, _ := strconv.ParseInt(p.Positions[0].Sequence, 10, 64)
	state.Positions[f.PoolAddress][sequence] = repair1048JSON(p.Positions[0])
	if applied {
		timestamp, _ := strconv.ParseInt(f.BlockTimestampSeconds, 10, 64)
		state.Applied[f.PoolAddress] = repair1048Applied{Wire: wire, Fill: repair1048JSON(f), Clock: engine.SeriesCheckpoint{Height: uint64(height), Timestamp: timestamp}, Revision: 1}
	}
}
func repair1048Fixture(t *testing.T) (*repair1048Store, *ethclient.Client, accounting.PublicationV1, *atomic.Int64) {
	t.Helper()
	calls := &atomic.Int64{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var request struct {
			JSONRPC string            `json:"jsonrpc"`
			ID      json.RawMessage   `json:"id"`
			Method  string            `json:"method"`
			Params  []json.RawMessage `json:"params"`
		}
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			t.Error(err)
			w.WriteHeader(400)
			return
		}
		var height string
		if len(request.Params) != 2 || json.Unmarshal(request.Params[0], &height) != nil || request.Method != "eth_getBlockByNumber" {
			t.Error("unexpected chain read", request.Method)
			w.WriteHeader(400)
			return
		}
		headers := map[string]any{
			"0x64": map[string]string{"number": "0x64", "hash": "0x" + strings.Repeat("b", 64), "timestamp": "0x6553f100"},
			"0x66": map[string]string{"number": "0x66", "hash": "0x" + strings.Repeat("e", 64), "timestamp": "0x6553f178"},
			"0x67": map[string]string{"number": "0x67", "hash": "0x" + strings.Repeat("f", 64), "timestamp": "0x6553f1b4"},
		}
		header, ok := headers[height]
		if !ok {
			t.Error("unrecorded height", height)
			w.WriteHeader(400)
			return
		}
		calls.Add(1)
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"jsonrpc": "2.0", "id": request.ID, "result": header})
	}))
	t.Cleanup(server.Close)
	rpc, err := ethclient.Dial(server.URL)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(rpc.Close)
	fill := accountingFixture()
	publication := repair1048Publication(t, fill, 1, "")
	peer := &repair1048Store{path: filepath.Join(t.TempDir(), "state.json"), state: repair1048State{Watermark: sourceWatermark{Raw: 100, Typed: 100, Fence: 1, Owner: "source-owner", Alive: true}, Fills: map[string]repair1048SourceFill{}, Publications: map[string][]byte{}, Positions: map[string]map[int64][]byte{}, Applied: map[string]repair1048Applied{}, BlockHashes: map[int64]string{100: fill.Position.BlockHash, 102: "0x" + strings.Repeat("e", 64), 103: "0x" + strings.Repeat("f", 64)}}}
	repair1048Record(t, &peer.state, publication, true)
	if err = peer.save(); err != nil {
		t.Fatal(err)
	}
	return peer, rpc, publication, calls
}
func repair1048Consumer(peer *repair1048Store, rpc *ethclient.Client) *AccountingConsumer {
	return &AccountingConsumer{completeness: &completenessRuntime{store: peer, rpc: rpc, projection: "market-v1"}}
}
func repair1048AdvanceSource(t *testing.T, peer *repair1048Store, height int64) {
	peer.mutate(t, func(s *repair1048State) { s.Watermark.Raw = height; s.Watermark.Typed = height })
}
func repair1048Bar(t *testing.T, peer *repair1048Store, p accounting.PublicationV1) (engine.SeriesCandle, uint64) {
	t.Helper()
	peer.mu.Lock()
	applied := peer.state.Applied[p.Fill.PoolAddress]
	peer.mu.Unlock()
	candle, err := engine.FoldQualified(nil, *p.Fill, "1m", false)
	if err != nil {
		t.Fatal(err)
	}
	bar, err := engine.DescribeSeries(candle, applied.Clock, time.Unix(1700001000, 0))
	if err != nil {
		t.Fatal(err)
	}
	return bar, applied.Revision
}
func TestRepairM_1048_IdlePoolCompletion(t *testing.T) {
	peer, rpc, p, calls := repair1048Fixture(t)
	c := repair1048Consumer(peer, rpc)
	before, revision := repair1048Bar(t, peer, p)
	if before.CompletenessStatus != "interval_elapsed" || revision != 1 {
		t.Fatal(before, revision)
	}
	repair1048AdvanceSource(t, peer, 102)
	next, err := c.refreshCompletenessBatch(context.Background(), "", completenessBatchSize)
	if err != nil || next != "" {
		t.Fatal(next, err)
	}
	after, revision := repair1048Bar(t, peer, p)
	if after.CompletenessStatus != "complete_through_watermark" || after.SourceWatermarkHeight != 102 || after.SourceWatermarkTimestamp != 1700000120 || revision != 2 || calls.Load() != 1 {
		t.Fatal(after, revision, calls.Load())
	}
	if _, err = c.refreshCompletenessBatch(context.Background(), "", completenessBatchSize); err != nil {
		t.Fatal(err)
	}
	_, revision = repair1048Bar(t, peer, p)
	if revision != 2 {
		t.Fatal("unchanged checkpoint rewrote clock", revision)
	}
}
func TestRepairM_1048_GapRefusal(t *testing.T) {
	for _, kind := range []string{"unresolved_delivery", "unapplied_partition_head"} {
		t.Run(kind, func(t *testing.T) {
			peer, rpc, p, _ := repair1048Fixture(t)
			c := repair1048Consumer(peer, rpc)
			repair1048AdvanceSource(t, peer, 102)
			peer.mutate(t, func(s *repair1048State) {
				if kind == "unresolved_delivery" {
					s.Unresolved = true
					return
				}
				fill := *p.Fill
				fill.Position.Height = "101"
				fill.Position.LogIndex = "8"
				fill.Position.TransactionHash = "0x" + strings.Repeat("d", 64)
				fill.SourceFillID = accounting.SourceFillIdentity(fill.Position, fill.Provenance.DeploymentID)
				later := repair1048Publication(t, fill, 2, p.Positions[0].EffectIdentity)
				repair1048Record(t, s, later, false)
			})
			if _, err := c.refreshCompletenessBatch(context.Background(), "", completenessBatchSize); err != nil {
				t.Fatal(err)
			}
			bar, _ := repair1048Bar(t, peer, p)
			if bar.CompletenessStatus != "backfill_incomplete" {
				t.Fatal("gap advertised as complete", bar)
			}
			if kind == "unapplied_partition_head" && bar.SourceWatermarkHeight != 100 {
				t.Fatal("crossed undelivered partition head", bar)
			}
			if kind == "unresolved_delivery" {
				peer.mutate(t, func(s *repair1048State) { s.Unresolved = false })
				if _, err := c.refreshCompletenessBatch(context.Background(), "", completenessBatchSize); err != nil {
					t.Fatal(err)
				}
				bar, _ = repair1048Bar(t, peer, p)
				if bar.CompletenessStatus != "complete_through_watermark" {
					t.Fatal("resolved delivery did not recover", bar)
				}
			}
		})
	}
}
func TestRepairM_1048_SourceAuthorization(t *testing.T) {
	for _, kind := range []string{"expired_lease", "raw_typed_gap", "missing_publication", "changed_publication", "changed_position", "changed_fill", "raw_block_conflict", "incomplete_decode", "missing_commit", "watermark_hash_conflict", "frozen_pool", "unapplied_fill", "malformed_persisted_wire", "projection_mismatch", "regressing_checkpoint"} {
		t.Run(kind, func(t *testing.T) {
			peer, rpc, p, _ := repair1048Fixture(t)
			repair1048AdvanceSource(t, peer, 102)
			peer.mutate(t, func(s *repair1048State) {
				source := s.Fills[p.SourceFillID]
				applied := s.Applied[p.Fill.PoolAddress]
				switch kind {
				case "expired_lease":
					s.Watermark.Alive = false
				case "raw_typed_gap":
					s.Watermark.Raw = 103
				case "missing_publication":
					delete(s.Publications, p.PublicationID)
				case "changed_publication":
					s.Publications[p.PublicationID] = []byte(`{}`)
				case "changed_position":
					s.Positions[p.Fill.PoolAddress][1] = []byte(`{}`)
				case "changed_fill":
					source.Fill.USDLAmountRaw = "6000000000"
				case "raw_block_conflict":
					source.Raw.BlockHash = "0x" + strings.Repeat("f", 64)
				case "incomplete_decode":
					source.DecodeComplete = false
				case "missing_commit":
					source.LastHeight = 99
				case "watermark_hash_conflict":
					s.BlockHashes[102] = "0x" + strings.Repeat("f", 64)
				case "frozen_pool":
					applied.Frozen = true
				case "unapplied_fill":
					applied.Fill = []byte(`{}`)
				case "malformed_persisted_wire":
					applied.Wire = []byte(`{}`)
				case "projection_mismatch":
					other := p
					other.Positions = append([]accounting.PartitionPosition(nil), p.Positions...)
					other.Positions[0].ProjectionVersion = "market-v2"
					other.Positions[0].EffectIdentity = accounting.EffectIdentity(p.SourceFillID, "market", "market-v2", "", "market", other.Positions[0].PartitionKey)
					other.PublicationID = accounting.PublicationIdentity(p.SourceFillID, p.SourceFactDigest, "market", "market-v2", "initial", "")
					wire, err := accounting.CanonicalPublicationV1(other)
					if err != nil {
						t.Fatal(err)
					}
					applied.Wire = wire
				case "regressing_checkpoint":
					applied.Clock = engine.SeriesCheckpoint{Height: 103, Timestamp: 1700000180}
				}
				s.Fills[p.SourceFillID] = source
				s.Applied[p.Fill.PoolAddress] = applied
			})
			peer.mu.Lock()
			before := repair1048JSON(peer.state.Applied)
			peer.mu.Unlock()
			c := repair1048Consumer(peer, rpc)
			_, err := c.refreshCompletenessBatch(context.Background(), "", completenessBatchSize)
			if err == nil {
				t.Fatal("source authorization accepted", kind)
			}
			peer.mu.Lock()
			after := repair1048JSON(peer.state.Applied)
			peer.mu.Unlock()
			if !bytes.Equal(before, after) {
				t.Fatal("refused refresh mutated persisted clock", kind)
			}
		})
	}
}
func TestRepairM_1048_RestartRecovery(t *testing.T) {
	peer, rpc, p, _ := repair1048Fixture(t)
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	run := func(c *AccountingConsumer, want uint64) {
		t.Helper()
		ctx, cancel := context.WithCancel(context.Background())
		done := make(chan error, 1)
		go func() { done <- c.RunCompleteness(ctx, logger) }()
		defer cancel()
		deadline := time.NewTimer(2 * time.Second)
		defer deadline.Stop()
		poll := time.NewTicker(time.Millisecond)
		defer poll.Stop()
		for {
			bar, _ := repair1048Bar(t, peer, p)
			if bar.SourceWatermarkHeight == want {
				break
			}
			select {
			case <-deadline.C:
				t.Fatal("refresh loop did not reach source checkpoint", bar)
			case <-poll.C:
			}
		}
		cancel()
		select {
		case err := <-done:
			if !errors.Is(err, context.Canceled) {
				t.Fatal(err)
			}
		case <-time.After(time.Second):
			t.Fatal("refresh loop did not stop")
		}
	}
	repair1048AdvanceSource(t, peer, 102)
	run(repair1048Consumer(peer, rpc), 102)
	raw, err := os.ReadFile(peer.path)
	if err != nil {
		t.Fatal(err)
	}
	reopened := &repair1048Store{path: peer.path}
	if err = json.Unmarshal(raw, &reopened.state); err != nil {
		t.Fatal(err)
	}
	peer = reopened
	repair1048AdvanceSource(t, peer, 103)
	run(repair1048Consumer(peer, rpc), 103)
	bar, revision := repair1048Bar(t, peer, p)
	if bar.CompletenessStatus != "complete_through_watermark" || revision != 3 || bar.SourceWatermarkTimestamp != 1700000180 {
		t.Fatal(bar, revision)
	}
	peer.mu.Lock()
	fills := len(peer.state.Fills)
	applied := len(peer.state.Applied)
	peer.mu.Unlock()
	if fills != 1 || applied != 1 {
		t.Fatal("restart needed a new fill", fills, applied)
	}
}
func TestRepairM_1048_BoundedFairSweep(t *testing.T) {
	peer, rpc, first, _ := repair1048Fixture(t)
	peer.mutate(t, func(s *repair1048State) {
		delete(s.Applied, first.Fill.PoolAddress)
		for i := 1; i <= completenessBatchSize+2; i++ {
			fill := *first.Fill
			fill.PoolAddress = fmt.Sprintf("0x%040x", i)
			fill.Position.TransactionHash = fmt.Sprintf("0x%064x", i)
			fill.SourceFillID = accounting.SourceFillIdentity(fill.Position, fill.Provenance.DeploymentID)
			p := repair1048Publication(t, fill, 1, "")
			repair1048Record(t, s, p, true)
		}
		broken := s.Applied[fmt.Sprintf("0x%040x", 1)]
		broken.Wire = []byte(`{}`)
		s.Applied[fmt.Sprintf("0x%040x", 1)] = broken
	})
	repair1048AdvanceSource(t, peer, 102)
	c := repair1048Consumer(peer, rpc)
	next, err := c.refreshCompletenessBatch(context.Background(), "", completenessBatchSize)
	if !errors.Is(err, ErrAccountingAuthority) || next != fmt.Sprintf("0x%040x", completenessBatchSize) {
		t.Fatal("page cursor", next, err)
	}
	peer.mu.Lock()
	refreshed := 0
	for _, applied := range peer.state.Applied {
		if applied.Clock.Height == 102 {
			refreshed++
		}
	}
	peer.mu.Unlock()
	if refreshed != completenessBatchSize-1 {
		t.Fatal("page did not respect bound or refused pool starved peers", refreshed)
	}
	next, err = c.refreshCompletenessBatch(context.Background(), next, completenessBatchSize)
	if err != nil || next != "" {
		t.Fatal("sweep did not wrap", next, err)
	}
	peer.mu.Lock()
	last := peer.state.Applied[fmt.Sprintf("0x%040x", completenessBatchSize+2)]
	peer.mu.Unlock()
	if last.Clock.Height != 102 {
		t.Fatal("tail pool starved", last)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err = c.refreshCompletenessBatch(ctx, "", completenessBatchSize); !errors.Is(err, context.Canceled) {
		t.Fatal("cancel ignored", err)
	}
	for _, limit := range []int{0, completenessBatchSize + 1} {
		if _, err = c.refreshCompletenessBatch(context.Background(), "", limit); !errors.Is(err, ErrAccountingAuthority) {
			t.Fatal("unbounded page accepted", limit, err)
		}
	}
}

func TestRepairM_1048_RefreshLoopRetries(t *testing.T) {
	peer, rpc, p, _ := repair1048Fixture(t)
	peer.mutate(t, func(s *repair1048State) { s.Watermark.Alive = false })
	c := repair1048Consumer(peer, rpc)
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- c.runCompleteness(ctx, logger, 5*time.Millisecond) }()
	defer func() {
		cancel()
		select {
		case err := <-done:
			if !errors.Is(err, context.Canceled) {
				t.Error(err)
			}
		case <-time.After(time.Second):
			t.Error("refresh loop did not stop")
		}
	}()
	deadline := time.NewTimer(2 * time.Second)
	defer deadline.Stop()
	poll := time.NewTicker(time.Millisecond)
	defer poll.Stop()
	for peer.rollbacks.Load() == 0 {
		select {
		case <-deadline.C:
			t.Fatal("initial refused sweep did not finish")
		case <-poll.C:
		}
	}
	initial, revision := repair1048Bar(t, peer, p)
	if initial.CompletenessStatus != "interval_elapsed" || revision != 1 {
		t.Fatal("expired lease advanced the clock", initial, revision)
	}
	// A later tick must recover when the persisted source lease and watermark
	// become available, after the first refused sweep has rolled back.
	peer.mutate(t, func(s *repair1048State) { s.Watermark.Alive = true; s.Watermark.Raw = 102; s.Watermark.Typed = 102 })
	for {
		bar, _ := repair1048Bar(t, peer, p)
		if bar.SourceWatermarkHeight == 102 {
			if bar.CompletenessStatus != "complete_through_watermark" {
				t.Fatal(bar)
			}
			return
		}
		select {
		case <-deadline.C:
			t.Fatal("periodic sweep did not recover", bar)
		case <-poll.C:
		}
	}
}
