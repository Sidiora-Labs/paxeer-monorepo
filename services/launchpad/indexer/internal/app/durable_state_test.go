package app

import (
	"context"
	"errors"
	"github.com/jackc/pgx/v5/pgxpool"
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Sidiora-Technologies/KindleLaunch/core/indexer/internal/receipt"
	"github.com/Sidiora-Technologies/KindleLaunch/core/indexer/internal/store"
	"github.com/Sidiora-Technologies/KindleLaunch/core/indexer/internal/store/canonicaltest"
	"github.com/Sidiora-Technologies/KindleLaunch/shared/accounting"
	"github.com/Sidiora-Technologies/KindleLaunch/shared/delivery"
	"github.com/ethereum/go-ethereum/common"
	"github.com/jackc/pgx/v5"
)

func TestRuntimeDurableCheckpointAndMarketInventory(t *testing.T) {
	ctx := context.Background()
	_, pool := canonicaltest.New(t)
	for _, build := range []func(string) (string, error){delivery.PartitionSchemaDDL, delivery.ReplayDDL} {
		ddl, err := build("indexer")
		if err != nil {
			t.Fatal(err)
		}
		if _, err = pool.Exec(ctx, ddl); err != nil {
			t.Fatal(err)
		}
	}
	// Canonical creation and archive rows use the same production schema as ingestion.
	applyRuntimeCreationSchema(t, pool)
	st := store.New(pool)
	doc := lifecycleRuntimeDocument{InitialCheckpointHeight: 100, InitialCheckpointHash: "0x" + strings.Repeat("a", 64)}
	if err := seedCanonicalWatermark(ctx, pool, "runtime", doc); err != nil {
		t.Fatal(err)
	}
	if err := seedCanonicalWatermark(ctx, pool, "runtime", lifecycleRuntimeDocument{InitialCheckpointHeight: 99, InitialCheckpointHash: doc.InitialCheckpointHash}); err != nil {
		t.Fatal(err)
	}
	if h, err := st.GetCanonicalWatermark(ctx, 125, "runtime"); err != nil || h != 100 {
		t.Fatalf("seeded checkpoint %d %v", h, err)
	}
	if hash, err := durableBlockHash(ctx, pool, "runtime", 100, doc); err != nil || hash != common.HexToHash(doc.InitialCheckpointHash) {
		t.Fatalf("issued checkpoint %s %v", hash, err)
	}
	if _, err := durableBlockHash(ctx, pool, "runtime", 101, doc); err == nil {
		t.Fatal("incomplete block has durable hash")
	}
	anchor := receipt.BlockAnchor{Height: 100, Hash: common.HexToHash(doc.InitialCheckpointHash)}
	if err := attestDurableAnchor(ctx, pool, "runtime", anchor); err == nil {
		t.Fatal("unextended anchor attested")
	}
	raw := store.RawRecord{Height: 101, TransactionHash: "0x" + strings.Repeat("1", 64), BlockHash: "0x" + strings.Repeat("2", 64), Emitter: "0x" + strings.Repeat("3", 40), Topics: []string{}, Data: []byte{1}}
	fence, err := st.AcquireLease(ctx, 125, "runtime", "worker", 30*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	fact := store.CreationFact{Token: "0x" + strings.Repeat("4", 40), Pool: "0x" + strings.Repeat("5", 40), Creator: "0x" + strings.Repeat("6", 40), Factory: raw.Emitter, Optical: "0x" + strings.Repeat("0", 40), NFTID: "1", Position: accounting.SourcePosition{ChainID: "125", Height: "101", TransactionIndex: "0", LogIndex: "0", TransactionHash: raw.TransactionHash, BlockHash: raw.BlockHash}, BlockTimestampSeconds: "1700000000", Provenance: accounting.Provenance{DeploymentID: "private-125", ABIRevision: "v1", EmitterCodeEpoch: "emitter", PoolCodeEpoch: "pool", ReceiptSHA256: strings.Repeat("a", 64), HeaderSHA256: strings.Repeat("b", 64)}}
	batch := store.CanonicalBatch{ChainID: 125, StreamID: "runtime", Owner: "worker", BatchID: "creation", Fence: fence, PreviousHeight: 100, Blocks: []store.BlockCompleteness{{Height: 101, Hash: raw.BlockHash, RawCount: 1, FetchComplete: true, RequiredDecodeComplete: true}}, Records: []store.RawRecord{raw}}
	if err = st.CommitProjectedCanonicalTx(ctx, batch, func(ctx context.Context, tx store.CanonicalTx) ([]store.OutboxIntent, error) {
		return nil, tx.InsertCreation(ctx, fact)
	}); err != nil {
		t.Fatal(err)
	}
	if hash, err := durableBlockHash(ctx, pool, "runtime", 101, doc); err != nil || hash != common.HexToHash(raw.BlockHash) {
		t.Fatalf("canonical checkpoint %s %v", hash, err)
	}
	if err = attestDurableAnchor(ctx, pool, "runtime", anchor); err != nil {
		t.Fatal(err)
	}
	inventory := postgresMarketInventory{pool: pool}
	addresses := []common.Address{common.HexToAddress(fact.Pool), common.HexToAddress(fact.Token)}
	page, err := inventory.Page(ctx, addresses, 101, "")
	if err != nil || len(page) != 1 || page[0].Token != common.HexToAddress(fact.Token) || page[0].Pool != addresses[0] || page[0].Height != 101 {
		t.Fatalf("canonical market inventory %+v %v", page, err)
	}
	for _, tc := range []struct {
		through uint64
		after   string
	}{{100, ""}, {101, fact.Token}} {
		page, err = inventory.Page(ctx, addresses, tc.through, tc.after)
		if err != nil || len(page) != 0 {
			t.Fatalf("bounded inventory %+v %v", page, err)
		}
	}
	recovered, err := recoverBatchMarkets(ctx, inventory, addresses, 101)
	if err != nil || len(recovered) != 1 || recovered[0].Token != common.HexToAddress(fact.Token) {
		t.Fatalf("recovered inventory %+v %v", recovered, err)
	}
	for _, hints := range [][]common.Address{nil, make([]common.Address, 129)} {
		if _, err = inventory.Page(ctx, hints, 101, ""); !errors.Is(err, receipt.ErrQualification) {
			t.Fatalf("unbounded inventory %v", err)
		}
	}
	if _, err = inventory.Page(ctx, addresses, math.MaxUint64, ""); !errors.Is(err, receipt.ErrQualification) {
		t.Fatalf("inventory height %v", err)
	}
	canceled, cancel := context.WithCancel(ctx)
	cancel()
	if _, err = inventory.Page(canceled, addresses, 101, ""); !errors.Is(err, context.Canceled) {
		t.Fatalf("canceled inventory %v", err)
	}
	scope := store.OperationScope{AuthorizationID: "runtime-scope", Operator: "operator", Action: "retention", Target: "canonical", Generation: "g1", Revision: "v1", From: 101, To: 101}
	if err = authorizeRuntimeOperation(ctx, pool, scope); !errors.Is(err, pgx.ErrNoRows) {
		t.Fatalf("unissued scope %v", err)
	}
	if _, err = pool.Exec(ctx, `INSERT INTO indexer.indexer_replay_authorizations VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,1,clock_timestamp()+interval '1 hour',NULL)`, scope.AuthorizationID, scope.Operator, strings.Repeat("a", 64), scope.Action, scope.Target, scope.Generation, scope.Revision, scope.From, scope.To); err != nil {
		t.Fatal(err)
	}
	if err = authorizeRuntimeOperation(ctx, pool, scope); err != nil {
		t.Fatal(err)
	}
	denied := scope
	denied.To++
	if err = authorizeRuntimeOperation(ctx, pool, denied); err == nil {
		t.Fatal("scope expanded beyond issued range")
	}
	if err = captureArchiveBatch(ctx, pool, st, nil, nil, lifecycleArchiveDocument{FromHeight: 102, BatchBlocks: 32}, "runtime"); err != nil {
		t.Fatalf("archive beyond watermark %v", err)
	}
	if err = captureArchiveBatch(ctx, pool, st, nil, nil, lifecycleArchiveDocument{FromHeight: 101, BatchBlocks: 32}, "absent"); !errors.Is(err, pgx.ErrNoRows) {
		t.Fatalf("archive without complete blocks %v", err)
	}
	pool.Close()
	if _, err = inventory.Page(ctx, addresses, 101, ""); err == nil {
		t.Fatal("closed inventory database accepted")
	}
	if _, err = durableBlockHash(ctx, pool, "runtime", 101, doc); err == nil {
		t.Fatal("closed checkpoint database accepted")
	}
	if err = attestDurableAnchor(ctx, pool, "runtime", anchor); err == nil {
		t.Fatal("closed anchor database accepted")
	}
	if err = captureArchiveBatch(ctx, pool, st, nil, nil, lifecycleArchiveDocument{FromHeight: 101, BatchBlocks: 32}, "runtime"); err == nil {
		t.Fatal("closed archive database accepted")
	}
}

func applyRuntimeCreationSchema(t *testing.T, pool *pgxpool.Pool) {
	t.Helper()
	for _, id := range []string{"60001", "190001"} {
		body, err := os.ReadFile(filepath.Join("../../../../../migrations", id+".sql"))
		if err != nil {
			t.Fatal(err)
		}
		if _, err = pool.Exec(context.Background(), string(body)); err != nil {
			t.Fatal(err)
		}
	}
}
