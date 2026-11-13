package source

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Sidiora-Technologies/KindleLaunch/core/indexer/internal/store"
	"github.com/Sidiora-Technologies/KindleLaunch/core/indexer/internal/store/canonicaltest"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	ethtypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type canonicalCapacityWorkload struct {
	source *CommittedSource
	store  *store.Store
	fence  int64
	stream string
}

func (w *canonicalCapacityWorkload) ProcessBatch(ctx context.Context, from, to int64) error {
	result, err := w.source.FetchCommitted(ctx, FetchOptions{FromBlock: from, ToBlock: to}, false, nil)
	if err != nil {
		return err
	}
	batch := store.CanonicalBatch{ChainID: 125, StreamID: w.stream, Owner: "capacity", Fence: w.fence, PreviousHeight: from - 1, BatchID: fmt.Sprintf("capacity-%d-%d", from, to)}
	for height := from; height <= to; height++ {
		record := result.Blocks[uint64(height)]
		count := 0
		for _, log := range result.Logs {
			if log.BlockNumber == uint64(height) {
				count++
			}
		}
		batch.Blocks = append(batch.Blocks, store.BlockCompleteness{Height: height, Hash: strings.ToLower(record.Hash.Hex()), RawCount: count, FetchComplete: true, RequiredDecodeComplete: true})
	}
	for _, log := range result.Logs {
		raw := store.RawRecord{Height: int64(log.BlockNumber), BlockHash: strings.ToLower(log.BlockHash.Hex()), TransactionHash: strings.ToLower(log.TxHash.Hex()), Emitter: strings.ToLower(log.Address.Hex()), TransactionIndex: uint32(log.TxIndex), LogIndex: uint32(log.Index), Data: log.Data, Topics: []string{}}
		for _, topic := range log.Topics {
			raw.Topics = append(raw.Topics, strings.ToLower(topic.Hex()))
		}
		batch.Records = append(batch.Records, raw)
	}
	return w.store.CommitCanonical(ctx, batch, nil)
}

type capacityChainFixture struct {
	head          atomic.Uint64
	advanceOnHead atomic.Bool
	logs          []ethtypes.Log
	server        *httptest.Server
}

func newCapacityChain(t *testing.T) *capacityChainFixture {
	t.Helper()
	node := new(capacityChainFixture)
	node.head.Store(100)
	address := common.HexToAddress("0x0000000000000000000000000000000000000001")
	node.logs = []ethtypes.Log{{Address: address, BlockNumber: 101, BlockHash: committedFixtureHash(101), TxHash: common.HexToHash(fh), Topics: []common.Hash{common.HexToHash(fh)}, Data: []byte{1}}}
	node.server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var q struct {
			ID     uint64
			Method string
			Params []json.RawMessage
		}
		if err := json.NewDecoder(r.Body).Decode(&q); err != nil {
			http.Error(w, "invalid request", 400)
			return
		}
		var result any
		switch q.Method {
		case "eth_chainId":
			result = "0x7d"
		case "eth_getBlockByNumber":
			var tag string
			if err := json.Unmarshal(q.Params[0], &tag); err != nil {
				http.Error(w, "invalid height", 400)
				return
			}
			if tag == "latest" {
				tag = hexutil.EncodeUint64(node.head.Load())
				if node.advanceOnHead.Swap(false) {
					node.head.Add(1)
				}
			}
			transactions := []any{}
			height, err := hexutil.DecodeUint64(tag)
			if err != nil {
				http.Error(w, "invalid height", 400)
				return
			}
			seen := map[common.Hash]bool{}
			for _, log := range node.logs {
				if log.BlockNumber == height && !seen[log.TxHash] {
					transactions = append(transactions, map[string]any{"hash": log.TxHash.Hex(), "from": log.Address.Hex()})
					seen[log.TxHash] = true
				}
			}
			block := committedFixtureBlock(tag, transactions)
			block["timestamp"] = hexutil.EncodeUint64(100 + height)
			result = block
		case "eth_getLogs":
			var filter struct {
				From string `json:"fromBlock"`
				To   string `json:"toBlock"`
			}
			if err := json.Unmarshal(q.Params[0], &filter); err != nil {
				http.Error(w, "invalid range", 400)
				return
			}
			from, err := hexutil.DecodeUint64(filter.From)
			if err != nil {
				http.Error(w, "invalid range", 400)
				return
			}
			to, err := hexutil.DecodeUint64(filter.To)
			if err != nil {
				http.Error(w, "invalid range", 400)
				return
			}
			logs := []ethtypes.Log{}
			for _, log := range node.logs {
				if log.BlockNumber >= from && log.BlockNumber <= to {
					logs = append(logs, log)
				}
			}
			result = logs
		default:
			http.Error(w, "unsupported method", 400)
			return
		}
		if err := json.NewEncoder(w).Encode(map[string]any{"jsonrpc": "2.0", "id": q.ID, "result": result}); err != nil {
			t.Error(err)
		}
	}))
	t.Cleanup(node.server.Close)
	return node
}

func capacityExplorer(t *testing.T, dsn string, pool *pgxpool.Pool, live *CommittedSource, blocks uint64, logs ...ethtypes.Log) *CommittedPaxscanSource {
	t.Helper()
	ctx := context.Background()
	role := "capacity_" + strings.ReplaceAll(uuid.NewString(), "-", "")
	password := uuid.NewString()
	schema := strings.Split(canonicalExplorerSchema, "CREATE ROLE")[0]
	if _, err := pool.Exec(ctx, schema); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, "CREATE ROLE "+pgx.Identifier{role}.Sanitize()+" LOGIN PASSWORD '"+password+"' NOSUPERUSER NOCREATEDB NOCREATEROLE NOREPLICATION NOBYPASSRLS"); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if _, err := pool.Exec(context.Background(), "DROP OWNED BY "+pgx.Identifier{role}.Sanitize()+"; DROP ROLE "+pgx.Identifier{role}.Sanitize()); err != nil {
			t.Error(err)
		}
	})
	if _, err := pool.Exec(ctx, "GRANT USAGE ON SCHEMA public TO "+pgx.Identifier{role}.Sanitize()+"; GRANT SELECT ON ALL TABLES IN SCHEMA public TO "+pgx.Identifier{role}.Sanitize()); err != nil {
		t.Fatal(err)
	}
	rows := [][]any{}
	for height := uint64(0); height <= blocks; height++ {
		rows = append(rows, []any{int64(height), committedFixtureHash(height).Bytes(), committedFixtureHash(height - 1).Bytes(), time.Unix(100+int64(height), 0), true})
	}
	if _, err := pool.CopyFrom(ctx, pgx.Identifier{"public", "blocks"}, []string{"number", "hash", "parent_hash", "timestamp", "consensus"}, pgx.CopyFromRows(rows)); err != nil {
		t.Fatal(err)
	}

	transactions := map[common.Hash]ethtypes.Log{}
	for _, log := range logs {
		if prior, exists := transactions[log.TxHash]; exists {
			if prior.BlockHash != log.BlockHash || prior.TxIndex != log.TxIndex || prior.Address != log.Address {
				t.Fatal("transaction coordinates disagree across source logs")
			}
		} else {
			if _, err := pool.Exec(ctx, `INSERT INTO public.transactions(hash,block_hash,"index",from_address_hash) VALUES($1,$2,$3,$4)`, log.TxHash.Bytes(), log.BlockHash.Bytes(), int64(log.TxIndex), log.Address.Bytes()); err != nil {
				t.Fatal(err)
			}
			transactions[log.TxHash] = log
		}
		topics := make([][]byte, 4)
		for i, topic := range log.Topics {
			topics[i] = topic.Bytes()
		}
		if _, err := pool.Exec(ctx, `INSERT INTO public.logs(address_hash,first_topic,second_topic,third_topic,fourth_topic,data,transaction_hash,block_hash,block_number,"index",transaction_index) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11)`, log.Address.Bytes(), topics[0], topics[1], topics[2], topics[3], log.Data, log.TxHash.Bytes(), log.BlockHash.Bytes(), int64(log.BlockNumber), int64(log.Index), int64(log.TxIndex)); err != nil {
			t.Fatal(err)
		}
	}
	parsed, err := url.Parse(dsn)
	if err != nil {
		t.Fatal(err)
	}
	parsed.User = url.UserPassword(role, password)
	explorer, err := NewCommittedPaxscan(ctx, parsed.String(), CommittedPaxscanOptions{SourceID: "capacity-explorer", SchemaVersion: ExplorerSchemaV1, ExpectedRole: role, ManifestDigest: live.opts.ManifestDigest, Epochs: live.opts.Epochs, RPC: live})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := explorer.Close(); err != nil {
			t.Error(err)
		}
	})
	return explorer
}

func TestCapacityWindowMeasuresCanonicalIdentitiesAndRejectsMissingFaults(t *testing.T) {
	ctx := context.Background()
	node := newCapacityChain(t)
	live := repair997Open(t, fopts(node.server.URL))
	dsn, pool := canonicaltest.New(t)
	explorer := capacityExplorer(t, dsn, pool, live, 100)
	st := store.New(pool)
	fence, err := st.AcquireLease(ctx, 125, "capacity", "capacity", 30*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = pool.Exec(ctx, `INSERT INTO indexer.indexer_ingest_watermarks VALUES(125,'capacity',100,100)`); err != nil {
		t.Fatal(err)
	}
	workload := &canonicalCapacityWorkload{source: live, store: st, fence: fence, stream: "capacity"}
	providers := CapacityProviders{Live: live, Historical: &HistoricalSource{RPC: live, Explorer: explorer}, Head: live, Processor: workload, Pool: pool}
	opts := capacityBoundOptions(t, live)
	opts.StreamID = "capacity"
	opts.OutputDirectory = t.TempDir()
	opts.Duration = 800 * time.Millisecond
	opts.TargetEventsPerSecond = 1
	opts.Profile = "harness"
	opts.HistoricalFrom = 1
	opts.HistoricalTo = 2
	opts.ParityFrom = 1
	opts.ParityTo = 10000
	opts.MaxCaptureBytes = 16 << 20
	opts.MaxHeapBytes = 1 << 30
	opts.MutationApproved = true
	opts.Workload = capacityOutputCommand(t, `{"Kind":"event","Value":{"chainId":125}}`)
	for _, profile := range []string{"unknown", "peak", "burst"} {
		changed := opts
		changed.Profile = profile
		report, err := RunSourceCapacity(ctx, changed, providers)
		if err == nil || report.HarnessPassed {
			t.Fatalf("invalid profile %s accepted", profile)
		}
	}
	changed := opts
	changed.TargetEventsPerSecond = 0
	if report, err := RunSourceCapacity(ctx, changed, providers); err == nil || report.HarnessPassed {
		t.Fatal("zero offered rate accepted")
	}
	prepareCapacityMeasurement(t, opts, providers)
	node.advanceOnHead.Store(true)
	report, err := RunSourceCapacity(ctx, opts, providers)
	if err == nil || !strings.Contains(err.Error(), "capacity-input-rate") && !strings.Contains(err.Error(), "capacity-adversarial-inputs") || report.HarnessPassed || report.CapacityTargetQualified || report.SharedSoakQualified || report.InputEvents != 1 || report.DurableEvents != 1 || report.Batches != 1 || report.HistoricalRanges == 0 || report.RawCaptureBytes == 0 || len(report.RawCaptureSHA256) != 64 || report.PeakHeapBytes == 0 {
		t.Fatalf("capacity events=%d durable=%d batches=%d history=%d capture=%d heap=%d: %v", report.InputEvents, report.DurableEvents, report.Batches, report.HistoricalRanges, report.RawCaptureBytes, report.PeakHeapBytes, err)
	}
	raw, readErr := os.ReadFile(filepath.Join(opts.OutputDirectory, "source-capacity.raw.jsonl"))
	if readErr != nil || !bytes.Contains(raw, []byte("durable_raw")) || !bytes.Contains(raw, []byte("resource_sample")) || !bytes.Contains(raw, []byte("batch_sample")) {
		t.Fatalf("capacity records %v", readErr)
	}
	if got, err := st.GetCanonicalWatermark(ctx, 125, "capacity"); err != nil || got != 101 {
		t.Fatalf("durable watermark %d: %v", got, err)
	}

	for _, tc := range []struct {
		name     string
		prepare  func(*CapacityOptions)
		expected string
	}{
		{"existing-output", func(o *CapacityOptions) {
			if err := os.WriteFile(filepath.Join(o.OutputDirectory, "source-capacity.raw.jsonl"), []byte("retained"), 0600); err != nil {
				t.Fatal(err)
			}
		}, "file exists"},
		{"provider-command", func(o *CapacityOptions) { o.Workload.Executable = "relative" }, "capacity-workload"},
		{"no-events", func(o *CapacityOptions) { o.Duration = 200 * time.Millisecond }, "capacity-identities"},
		{"watermark", func(o *CapacityOptions) {
			if _, err := pool.Exec(ctx, `UPDATE indexer.indexer_ingest_watermarks SET typed_complete_height=100 WHERE stream_id='capacity'`); err != nil {
				t.Fatal(err)
			}
		}, "capacity-watermark"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			next := opts
			next.OutputDirectory = t.TempDir()
			tc.prepare(&next)
			prepareCapacityMeasurement(t, next, providers)
			measured, err := RunSourceCapacity(ctx, next, providers)
			if err == nil || !strings.Contains(err.Error(), tc.expected) || measured.HarnessPassed || measured.CapacityTargetQualified {
				t.Fatalf("invalid capacity run accepted: %v", err)
			}
			if tc.name == "watermark" {
				if _, err := pool.Exec(ctx, `UPDATE indexer.indexer_ingest_watermarks SET typed_complete_height=101 WHERE stream_id='capacity'`); err != nil {
					t.Fatal(err)
				}
			}
		})
	}
	next := opts
	next.OutputDirectory = t.TempDir()
	prepareCapacityMeasurement(t, next, providers)
	cancelled, cancelRun := context.WithTimeout(ctx, 150*time.Millisecond)
	measured, cancelErr := RunSourceCapacity(cancelled, next, providers)
	cancelRun()
	if !errors.Is(cancelErr, context.DeadlineExceeded) || measured.HarnessPassed || measured.RawCaptureBytes == 0 {
		t.Fatalf("bounded shutdown %+v %v", measured, cancelErr)
	}
	expected, err := live.FetchCommitted(ctx, FetchOptions{FromBlock: 101, ToBlock: 101}, false, nil)
	if err != nil {
		t.Fatal(err)
	}
	for _, field := range []string{"identity", "payload", "topics", "blocks", "duplicate"} {
		t.Run(field, func(t *testing.T) {
			changed := expected
			changed.Logs = append([]ethtypes.Log(nil), expected.Logs...)
			changed.Blocks = map[uint64]BlockRecord{}
			for k, v := range expected.Blocks {
				changed.Blocks[k] = v
			}
			switch field {
			case "identity":
				changed.Logs[0].TxHash = common.HexToHash("0xbeef")
			case "payload":
				changed.Logs[0].Data = []byte{2}
			case "topics":
				changed.Logs[0].Topics = []common.Hash{common.HexToHash("0xbeef")}
			case "blocks":
				delete(changed.Blocks, 101)
			case "duplicate":
				changed.Logs = append(changed.Logs, changed.Logs[0])
			}
			capture, e := newCapacityCapture(filepath.Join(t.TempDir(), "durable.jsonl"), 1<<20)
			if e != nil {
				t.Fatal(e)
			}
			if _, _, e = capacityDurable(ctx, pool, "capacity", 101, 101, changed, map[uint64]time.Time{101: report.StartedAt}, -int64(time.Second), map[int64]uint64{}, capture); e == nil {
				t.Fatal("changed canonical observation accepted")
			}
			if _, _, e = capture.finish(); e != nil {
				t.Fatal(e)
			}
		})
	}
}

func TestLagRecoveryPreservesPauseAndResumeIntervals(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 105*time.Second)
	defer cancel()
	node := newCapacityChain(t)
	live := repair997Open(t, fopts(node.server.URL))
	_, pool := canonicaltest.New(t)
	if _, err := pool.Exec(ctx, `INSERT INTO indexer.indexer_ingest_watermarks VALUES(125,'lag',1,1)`); err != nil {
		t.Fatal(err)
	}
	if _, err := live.BlockAt(ctx, 1); err != nil {
		t.Fatal(err)
	}
	if _, err := live.ObserveHead(ctx, 1); err != nil {
		t.Fatal(err)
	}
	providers := CapacityProviders{Live: live, Historical: &HistoricalSource{RPC: live}, Pool: pool}
	opts := capacityBoundOptions(t, live)
	opts.StreamID = "lag"
	opts.OutputDirectory = t.TempDir()
	opts.Duration = 98 * time.Second
	done := make(chan error, 1)
	go func() {
		done <- live.PollHeads(ctx, func() uint64 { return 1 }, func(_ HeadObservation, err error) error { return err })
	}()
	update := time.AfterFunc(33*time.Second, func() {
		if _, err := pool.Exec(ctx, `UPDATE indexer.indexer_ingest_watermarks SET raw_complete_height=100,typed_complete_height=100 WHERE stream_id='lag'`); err != nil {
			t.Error(err)
		}
	})
	defer update.Stop()
	report, err := MeasureSourceLagRecovery(ctx, opts, providers)
	if err != nil || !report.TransitionsValidated || !report.PauseObserved || !report.ResumeObserved || report.Samples == 0 || report.RawCaptureBytes == 0 || len(report.RawCaptureSHA256) != 64 || report.FinishedAt.Sub(report.StartedAt) < opts.Duration {
		t.Fatalf("lag recovery %+v %v", report, err)
	}
	cancel()
	if err := <-done; !errors.Is(err, context.Canceled) {
		t.Fatalf("head poll shutdown %v", err)
	}
}

func TestCapacityParityReportsCompleteHistoryWithoutInventingHighLogBlocks(t *testing.T) {
	ctx := context.Background()
	node := newCapacityChain(t)
	node.head.Store(10000)
	live := repair997Open(t, fopts(node.server.URL))
	dsn, pool := canonicaltest.New(t)
	explorer := capacityExplorer(t, dsn, pool, live, 10000, node.logs...)
	providers := CapacityProviders{Live: live, Historical: &HistoricalSource{RPC: live, Explorer: explorer}}
	opts := CapacityOptions{ParityFrom: 1, ParityTo: 10000}
	report, err := capacityParity(ctx, providers, opts)
	if err != nil || report.Certified || report.CompleteBlockCount != 10000 || report.EmptyBlockCount != 9999 || report.HighLogBlockCount != 0 || report.LeftRawSHA256 != report.RightRawSHA256 || report.LeftBlocksSHA256 != report.RightBlocksSHA256 || len(report.Missing) != 0 || len(report.Different) != 0 {
		t.Fatalf("complete parity %+v %v", report, err)
	}
	directory := t.TempDir()
	if err = WriteParityReport(directory, report); err != nil {
		t.Fatal(err)
	}
	if _, err = os.Stat(filepath.Join(directory, report.ReportID+".json")); err != nil {
		t.Fatal(err)
	}
	if err = WriteParityReport(directory, report); !os.IsExist(err) {
		t.Fatalf("parity report replaced %v", err)
	}
	changed := report
	changed.EmptyBlockCount--
	if err = WriteParityReport(t.TempDir(), changed); err == nil {
		t.Fatal("changed report identity accepted")
	}
	if err = ValidateParityAdmission(&report, live.opts.ManifestDigest, explorer.Name()); err == nil {
		t.Fatal("low log workload admitted as high-log parity")
	}
	providers.Historical = &HistoricalSource{RPC: live}
	if _, err = capacityParity(ctx, providers, opts); err == nil {
		t.Fatal("missing explorer parity accepted")
	}
}

func prepareCapacityMeasurement(t *testing.T, options CapacityOptions, providers CapacityProviders) {
	t.Helper()
	ctx := context.Background()
	if err := verifyCapacityBinding(ctx, options, providers); err != nil {
		t.Fatal(err)
	}
	var track string
	var recovery bool
	if err := providers.Pool.QueryRow(ctx, `SELECT current_setting('track_commit_timestamp'),pg_is_in_recovery()`).Scan(&track, &recovery); err != nil || track != "on" || recovery {
		t.Fatalf("capacity commit clock: track=%s recovery=%v err=%v", track, recovery, err)
	}
	var now time.Time
	if err := providers.Pool.QueryRow(ctx, `SELECT clock_timestamp()`).Scan(&now); err != nil || now.IsZero() {
		t.Fatalf("capacity database clock: %v %v", now, err)
	}
	runtime.GC()
}
