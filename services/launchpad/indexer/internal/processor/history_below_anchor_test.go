package processor

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/Sidiora-Technologies/KindleLaunch/core/indexer/internal/decode"
	"github.com/Sidiora-Technologies/KindleLaunch/core/indexer/internal/publisher"
	"github.com/Sidiora-Technologies/KindleLaunch/core/indexer/internal/receipt"
	"github.com/Sidiora-Technologies/KindleLaunch/core/indexer/internal/source"
	"github.com/Sidiora-Technologies/KindleLaunch/core/indexer/internal/store"
	"github.com/Sidiora-Technologies/KindleLaunch/shared/delivery"
	"github.com/Sidiora-Technologies/KindleLaunch/shared/queue"
	"github.com/ethereum/go-ethereum/common"
	"github.com/jackc/pgx/v5/pgxpool"
)

type historyBelowAnchorDocuments struct {
	environment map[string]string
	sql         []byte
	lifecycle   struct {
		Committed               source.CommittedOptions
		BackfillRequests        []store.ReplayRequest
		InitialCheckpointHeight int64
		InitialCheckpointHash   string
	}
	receipt struct{ Production receipt.RPCOptions }
}

func composeHistoryBelowAnchor(t *testing.T) historyBelowAnchorDocuments {
	t.Helper()
	root := repositoryRoot()
	values := t.TempDir()
	write := func(name string, body []byte) {
		t.Helper()
		if err := os.WriteFile(filepath.Join(values, name), body, 0600); err != nil {
			t.Fatal(err)
		}
	}
	read := func(name string) []byte {
		t.Helper()
		raw, err := os.ReadFile(filepath.Join(root, "deploy/launchpad/documents/values", name))
		if err != nil {
			t.Fatal(err)
		}
		return raw
	}
	write("common.json", read("common.example.json"))
	if err := os.Mkdir(filepath.Join(values, "expected-schema"), 0700); err != nil {
		t.Fatal(err)
	}
	write("expected-schema/indexer.json", []byte(`{"service":"core/indexer"}`))
	var data map[string]any
	if err := json.Unmarshal(read("indexer.example.json"), &data); err != nil {
		t.Fatal(err)
	}
	inputs := nativeProjectionReceiptInputs(t)
	options := nativeAccountingReceiptOptions(t, inputs)
	options.Anchor = &receipt.BlockAnchor{Height: 33963733, Hash: common.HexToHash(recordedHistoryBlockHash(t, 33963733))}
	raw, err := json.Marshal(struct {
		Realm      string
		Production receipt.RPCOptions
	}{"production", options})
	if err != nil {
		t.Fatal(err)
	}
	write("receipt.json", raw)
	data["receipt_rendered_file"] = "receipt.json"
	data["receipt_runtime_path"] = "/run/launchpad/indexer/receipt.json"
	data["anchor_height"] = options.Anchor.Height
	data["anchor_hash"] = strings.ToLower(options.Anchor.Hash.Hex())
	data["manifest_digest"] = options.Manifest.SourceManifestSHA256
	data["deployment_id"] = options.Manifest.DeploymentVersion
	data["history_from_height"] = 33963716
	data["history_operator"] = "launchpad_operator"
	data["history_expires_at"] = "2099-01-01T00:00:00Z"
	data["backfill_batch_size"] = 8
	data["region"] = "railway-prod"
	raw, err = json.Marshal(data)
	if err != nil {
		t.Fatal(err)
	}
	write("indexer.json", raw)
	out := t.TempDir()
	command := exec.Command("python3", filepath.Join(root, "deploy/launchpad/documents/compose.py"), "--values", values, "--out", out, "--service", "indexer")
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("compose history documents: %s: %v", output, err)
	}
	envBytes, err := os.ReadFile(filepath.Join(out, "indexer.json"))
	if err != nil {
		t.Fatal(err)
	}
	var docs historyBelowAnchorDocuments
	if err = json.Unmarshal(envBytes, &docs.environment); err != nil {
		t.Fatal(err)
	}
	if docs.environment["INDEXER_MODE"] != "backfill" || docs.environment["BACKFILL_ONLY"] != "false" {
		t.Fatal("composed mode must run history before live")
	}
	life, err := base64.StdEncoding.DecodeString(docs.environment["INDEXER_LIFECYCLE_CONFIG_DOCUMENT"])
	if err != nil {
		t.Fatal(err)
	}
	if delivery.Hash(life) != docs.environment["INDEXER_LIFECYCLE_CONFIG_SHA256"] {
		t.Fatal("lifecycle digest mismatch")
	}
	if err = json.Unmarshal(life, &docs.lifecycle); err != nil {
		t.Fatal(err)
	}
	if delivery.Hash(rawReceipt(t, values)) != docs.environment["INDEXER_RECEIPT_CONFIG_SHA256"] {
		t.Fatal("receipt digest mismatch")
	}
	if err = json.Unmarshal(rawReceipt(t, values), &docs.receipt); err != nil {
		t.Fatal(err)
	}
	docs.sql, err = os.ReadFile(filepath.Join(out, "indexer-history-authorizations.sql"))
	if err != nil {
		t.Fatal(err)
	}
	if len(docs.lifecycle.BackfillRequests) != 1 || docs.lifecycle.BackfillRequests[0].FromHeight != 33963716 || docs.lifecycle.BackfillRequests[0].ToHeight != 33963733 {
		t.Fatal("composed history must include the recorded market below the anchor")
	}
	return docs
}

func rawReceipt(t *testing.T, values string) []byte {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(values, "receipt.json"))
	if err != nil {
		t.Fatal(err)
	}
	return raw
}

func recordedHistoryBlockHash(t *testing.T, height uint64) string {
	t.Helper()
	raw := nativeProjectionRecordedResult(t, `eth_getBlockByNumber ["`+"0x"+strconv.FormatUint(height, 16)+`",true]`)
	var block struct{ Hash string }
	if err := json.Unmarshal(raw, &block); err != nil || len(block.Hash) != 66 {
		t.Fatalf("recorded block %d: %v", height, err)
	}
	return block.Hash
}

func historyBelowAnchorProcessor(t *testing.T, ctx context.Context, pool *pgxpool.Pool, docs historyBelowAnchorDocuments) *Processor {
	t.Helper()
	endpoint := nativeAccountingRecordedRPC(t, 33963749)
	client, closeClient, err := receipt.NewBoundedRPCClient(ctx, endpoint, "")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(closeClient)
	authority, err := receipt.NewRPCQualification(ctx, client, docs.receipt.Production)
	if err != nil {
		t.Fatal(err)
	}
	// Recover factory-issued epochs from the recorded creation before qualifying
	// the anchor's trades, as the runtime's historical market recovery does.
	creationHeight := uint64(docs.lifecycle.BackfillRequests[0].FromHeight)
	if _, _, err = authority.FetchQualifiedBatch(ctx, creationHeight, creationHeight); err != nil {
		t.Fatal(err)
	}
	options := docs.lifecycle.Committed
	options.Endpoints = map[string]string{"paxeer-primary": endpoint}
	rpc, err := source.NewCommittedSource(ctx, options)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := rpc.Close(); err != nil {
			t.Error(err)
		}
	})
	guard, err := queue.NewEpochGuard("recovery", 1, "railway-prod")
	if err != nil {
		t.Fatal(err)
	}
	st, err := store.NewLifecycleStore(pool, guard.Assert)
	if err != nil {
		t.Fatal(err)
	}
	policy := delivery.Policy{Subjects: map[string]bool{publisher.CanonicalCreationSubject: true, "accounting.market.v1": true, "accounting.inventory.v1": true, ReplaySubjectV2: true}, EventTypes: map[string]bool{publisher.CanonicalCreationEventType: true, "accounting.market.v1": true, "accounting.inventory.v1": true, ReplaySubjectV2: true}}
	outbox, err := delivery.NewStoreWithEpochGuard(pool, "indexer", policy, guard.Assert)
	if err != nil {
		t.Fatal(err)
	}
	partitions, err := delivery.NewPartitionStore(pool, "indexer")
	if err != nil {
		t.Fatal(err)
	}
	durable, err := publisher.NewDurableStager(outbox, docs.receipt.Production.Manifest.DeploymentVersion, map[string]string{publisher.CanonicalCreationEventType: publisher.CanonicalCreationSubject, "accounting.market.v1": "accounting.market.v1", "accounting.inventory.v1": "accounting.inventory.v1"})
	if err != nil {
		t.Fatal(err)
	}
	durableHeight := func() uint64 {
		height, e := st.GetCanonicalWatermark(ctx, 125, realStream)
		if e != nil {
			t.Error(e)
			return 0
		}
		return uint64(height)
	}
	historical := &source.HistoricalQualifiedSource{History: &source.HistoricalSource{RPC: rpc}, Authority: authority, DurableHeight: durableHeight}
	live := &source.LiveQualifiedSource{RPC: rpc, Authority: authority, DurableHeight: durableHeight, PriorHash: func(c context.Context, height uint64) (common.Hash, error) {
		if height == uint64(docs.lifecycle.InitialCheckpointHeight) {
			return common.HexToHash(docs.lifecycle.InitialCheckpointHash), nil
		}
		var hash string
		e := pool.QueryRow(c, `SELECT block_hash FROM indexer.indexer_block_completeness WHERE chain_id=125 AND stream_id=$1 AND height=$2`, realStream, int64(height)).Scan(&hash)
		return common.HexToHash(hash), e
	}}
	replay := &ReplayRunner{Store: st, Source: historical, Deliveries: outbox, Owner: inventoryTestOwner, DeliverySchema: "indexer", BatchSize: 8}
	return New(Deps{Store: st, Decoder: decode.NewDecoder(), ChainID: 125, EventEmitterAddress: docs.receipt.Production.Manifest.Roles["EventEmitter"].Address, QualifiedBatchSource: live, CanonicalStreamID: realStream, CanonicalOwner: inventoryTestOwner, AccountingPartitions: partitions, Durable: durable, AccountingProjectionVersion: "accounting-market-v1", InventoryGeneration: pnlGeneration, InventoryProjectionVersion: pnlInventoryVersion, LegacyProjection: true, Reserves: authority, HistoricalReplay: replay, HistoricalRequests: docs.lifecycle.BackfillRequests})
}

func TestHistoryBelowAnchorCanonicalCreationAndLiveContinuation(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	docs := composeHistoryBelowAnchor(t)
	setup := newInventoryHarness(t, ctx)
	pool := setup.pool
	if _, err := pool.Exec(ctx, `UPDATE indexer.indexer_ingest_watermarks SET raw_complete_height=$2,typed_complete_height=$2 WHERE chain_id=125 AND stream_id=$1`, realStream, docs.lifecycle.InitialCheckpointHeight); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 2; i++ {
		if _, err := pool.Exec(ctx, string(docs.sql)); err != nil {
			t.Fatalf("apply authorization statements: %v", err)
		}
	}
	var count int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM indexer.indexer_replay_authorizations`).Scan(&count); err != nil || count != len(docs.lifecycle.BackfillRequests) {
		t.Fatalf("repeat application must retain one row per request: %d %v", count, err)
	}
	p := historyBelowAnchorProcessor(t, ctx, pool, docs)
	first, err := strconv.ParseInt(docs.environment["BACKFILL_FROM_BLOCK"], 10, 64)
	if err != nil {
		t.Fatal(err)
	}
	size, err := strconv.Atoi(docs.environment["BACKFILL_BATCH_SIZE"])
	if err != nil {
		t.Fatal(err)
	}
	if err = NewBackfill(p, &first, size).Run(ctx); err != nil {
		t.Fatal(err)
	}
	if err = pool.QueryRow(ctx, `SELECT count(*) FROM indexer.indexer_canonical_creations WHERE pool=$1 AND height=33963716`, realMarketPool).Scan(&count); err != nil || count != 1 {
		t.Fatalf("canonical creation below anchor missing: %d %v", count, err)
	}
	if err = pool.QueryRow(ctx, `SELECT count(*) FROM indexer.pools WHERE pool_address=$1`, realMarketPool).Scan(&count); err != nil || count != 1 {
		t.Fatalf("market read projection missing: %d %v", count, err)
	}
	if height, e := p.deps.Store.GetCanonicalWatermark(ctx, 125, realStream); e != nil || height != 33963733 {
		t.Fatalf("history changed live anchor: %d %v", height, e)
	}
	if err = pool.QueryRow(ctx, `SELECT count(*) FROM indexer.indexer_accounting_fills_v1`).Scan(&count); err != nil || count != 1 {
		t.Fatalf("historical market event missing: %d %v", count, err)
	}
	if err = p.ProcessBatch(ctx, 33963734, 33963749); err != nil {
		t.Fatalf("live continuation from anchor: %v", err)
	}
	if height, e := p.deps.Store.GetCanonicalWatermark(ctx, 125, realStream); e != nil || height != 33963749 {
		t.Fatalf("live watermark: %d %v", height, e)
	}
	if err = pool.QueryRow(ctx, `SELECT count(*) FROM indexer.indexer_accounting_fills_v1`).Scan(&count); err != nil || count != 2 {
		t.Fatalf("live market event missing: %d %v", count, err)
	}
	freshPool, err := pgxpool.New(ctx, pool.Config().ConnString())
	if err != nil {
		t.Fatal(err)
	}
	defer freshPool.Close()
	restarted := historyBelowAnchorProcessor(t, ctx, freshPool, docs)
	if err = NewBackfill(restarted, &first, size).Run(ctx); err != nil {
		t.Fatalf("resume completed history from same documents: %v", err)
	}
	if err = freshPool.QueryRow(ctx, `SELECT count(*) FROM indexer.indexer_canonical_creations WHERE pool=$1`, realMarketPool).Scan(&count); err != nil || count != 1 {
		t.Fatalf("restart duplicated creation: %d %v", count, err)
	}
	if height, e := restarted.deps.Store.GetCanonicalWatermark(ctx, 125, realStream); e != nil || height != 33963749 {
		t.Fatalf("restart rewound live watermark: %d %v", height, e)
	}
}

func TestHistoryBelowAnchorMissingAuthorizationRefused(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	docs := composeHistoryBelowAnchor(t)
	h := newInventoryHarness(t, ctx)
	if _, err := h.pool.Exec(ctx, `UPDATE indexer.indexer_ingest_watermarks SET raw_complete_height=$2,typed_complete_height=$2 WHERE chain_id=125 AND stream_id=$1`, realStream, docs.lifecycle.InitialCheckpointHeight); err != nil {
		t.Fatal(err)
	}
	p := historyBelowAnchorProcessor(t, ctx, h.pool, docs)
	err := NewBackfill(p, nil, 8).Run(ctx)
	if err == nil || !strings.Contains(err.Error(), "authorization absent") {
		t.Fatalf("history without applied authorizations accepted: %v", err)
	}
	var count int
	if err = h.pool.QueryRow(ctx, `SELECT (SELECT count(*) FROM indexer.indexer_canonical_creations)+(SELECT count(*) FROM indexer.indexer_replay_jobs)+(SELECT count(*) FROM indexer.indexer_raw_logs)`).Scan(&count); err != nil || count != 0 {
		t.Fatalf("unauthorized history left effects: %d %v", count, err)
	}
	if height, e := p.deps.Store.GetCanonicalWatermark(ctx, 125, realStream); e != nil || height != 33963733 {
		t.Fatalf("unauthorized history moved anchor: %d %v", height, e)
	}
}

func TestHistoryBelowAnchorRequestRefusals(t *testing.T) {
	docs := composeHistoryBelowAnchor(t)
	base := docs.lifecycle.BackfillRequests[0]
	for _, tc := range []struct {
		name, want string
		edit       func(*store.ReplayRequest)
	}{
		{"range exceeds one request", "invalid replay bounds", func(r *store.ReplayRequest) { r.ToHeight = r.FromHeight + 1_000_000; r.CheckpointHeight = r.ToHeight }},
		{"other stream", "backfill source stream mismatch", func(r *store.ReplayRequest) { r.SourceStream = "other-stream" }},
		{"other projection", "canonical backfill source or projection mismatch", func(r *store.ReplayRequest) { r.ProjectionRevision = "other-projection" }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			request := base
			tc.edit(&request)
			p := New(Deps{CanonicalStreamID: realStream, InventoryGeneration: pnlGeneration, AccountingProjectionVersion: "accounting-market-v1", HistoricalReplay: &ReplayRunner{}, HistoricalRequests: []store.ReplayRequest{request}})
			if err := NewBackfill(p, nil, 8).Run(context.Background()); err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("history refusal: %v, want %s", err, tc.want)
			}
		})
	}
	if err := store.ValidateReplayRequest(base); err != nil {
		t.Fatal(err)
	}
	if err := NewBackfill(nil, nil, 8).Run(context.Background()); err == nil || errors.Is(err, context.Canceled) {
		t.Fatalf("missing historical runtime accepted: %v", err)
	}
}
