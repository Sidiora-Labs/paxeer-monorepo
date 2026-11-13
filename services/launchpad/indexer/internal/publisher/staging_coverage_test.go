package publisher

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Sidiora-Technologies/KindleLaunch/core/indexer/internal/store"
	"github.com/Sidiora-Technologies/KindleLaunch/core/indexer/internal/store/canonicaltest"
	"github.com/Sidiora-Technologies/KindleLaunch/shared/accounting"
	"github.com/Sidiora-Technologies/KindleLaunch/shared/delivery"
	"github.com/Sidiora-Technologies/KindleLaunch/shared/queue"
	"github.com/Sidiora-Technologies/KindleLaunch/shared/secrets"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

func stagingProviders(t *testing.T) (*store.Store, *delivery.Store, *pgxpool.Pool) {
	t.Helper()
	ctx := context.Background()
	_, pool := canonicaltest.New(t)
	for _, build := range []func(string) (string, error){delivery.SchemaDDL, delivery.PartitionSchemaDDL, delivery.WebhookDDL, delivery.ReplayDDL} {
		ddl, err := build("indexer")
		if err != nil {
			t.Fatal(err)
		}
		if _, err = pool.Exec(ctx, ddl); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := pool.Exec(ctx, `CREATE SCHEMA recovery`); err != nil {
		t.Fatal(err)
	}
	for _, id := range []string{"60001", "190001", "440000"} {
		body, err := os.ReadFile(filepath.Join("../../../../../launchpad/spec/production-system/integration/migrations", id+".sql"))
		if err != nil {
			t.Fatal(err)
		}
		if _, err = pool.Exec(ctx, string(body)); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := pool.Exec(ctx, `INSERT INTO recovery.recovery_control VALUES(true,1,'primary',1,repeat('0',64))`); err != nil {
		t.Fatal(err)
	}
	guard, err := queue.NewEpochGuard("recovery", 1, "primary")
	if err != nil {
		t.Fatal(err)
	}
	canonical, err := store.NewLifecycleStore(pool, guard.Assert)
	if err != nil {
		t.Fatal(err)
	}
	policy := delivery.Policy{Subjects: map[string]bool{accounting.MarketSubject: true, accounting.InventorySubject: true, CanonicalCreationSubject: true, "chain.swap": true}, EventTypes: map[string]bool{accounting.MarketSubject: true, accounting.InventorySubject: true, CanonicalCreationEventType: true, "Swap": true}}
	durable, err := delivery.NewStoreWithEpochGuard(pool, "indexer", policy, guard.Assert)
	if err != nil {
		t.Fatal(err)
	}
	return canonical, durable, pool
}

func stagingPublication(t *testing.T) accounting.PublicationV1 {
	t.Helper()
	f := stagingFill()
	digest, err := accounting.FillDigest(f)
	if err != nil {
		t.Fatal(err)
	}
	key := "125:" + f.PoolAddress
	positions := []accounting.PartitionPosition{{ProjectionVersion: "market-v1", PartitionKind: "market", PartitionKey: key, Sequence: "1", SourceEventID: f.SourceFillID, EffectIdentity: accounting.EffectIdentity(f.SourceFillID, "market", "market-v1", "", "market", key), PayloadDigest: digest, SourceOrder: f.Position}}
	p, err := accounting.NewMarketPublication(f, positions)
	if err != nil {
		t.Fatal(err)
	}
	return p
}
func stagingBridge(t *testing.T, durable *delivery.Store) *delivery.WebhookBridge {
	t.Helper()
	provider, err := secrets.NewChecked(secrets.Options{Provider: "env"})
	if err != nil {
		t.Fatal(err)
	}
	bridge, err := delivery.NewWebhookBridge(durable, []delivery.WebhookTarget{{ID: "charts", URL: "https://charts.example/events", SecretReference: "WEBHOOK_HMAC_SECRET", Subjects: map[string]bool{accounting.MarketSubject: true}}}, &http.Client{Transport: http.DefaultTransport.(*http.Transport).Clone()}, provider, func(context.Context, string) error { return nil }, "indexer", 10*time.Second, time.Second, time.Second, 3)
	if err != nil {
		t.Fatal(err)
	}
	return bridge
}
func TestDurableStagingPreservesCanonicalPublicationsAndWebhookPositions(t *testing.T) {
	ctx := context.Background()
	canonical, durable, pool := stagingProviders(t)
	subjects := map[string]string{accounting.MarketSubject: accounting.MarketSubject, accounting.InventorySubject: accounting.InventorySubject, CanonicalCreationEventType: CanonicalCreationSubject, "Swap": "chain.swap"}
	stager, err := NewDurableStager(durable, "private-125", subjects)
	if err != nil {
		t.Fatal(err)
	}
	publication := stagingPublication(t)
	f := *publication.Fill
	raw := store.RawRecord{Height: 101, TransactionHash: f.Position.TransactionHash, BlockHash: f.Position.BlockHash, Emitter: f.ActualEmitter, Topics: []string{}, Data: []byte{1}}
	fence, err := canonical.AcquireLease(ctx, 125, "staging", "worker", 30*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = pool.Exec(ctx, `INSERT INTO indexer.indexer_ingest_watermarks VALUES(125,'staging',100,100)`); err != nil {
		t.Fatal(err)
	}
	batch := store.CanonicalBatch{ChainID: 125, StreamID: "staging", Owner: "worker", BatchID: "source-fill", Fence: fence, PreviousHeight: 100, Blocks: []store.BlockCompleteness{{Height: 101, Hash: raw.BlockHash, RawCount: 1, FetchComplete: true, RequiredDecodeComplete: true}}, Records: []store.RawRecord{raw}}
	if err = canonical.CommitProjectedCanonicalTx(ctx, batch, func(ctx context.Context, tx store.CanonicalTx) ([]store.OutboxIntent, error) {
		if _, err := tx.InsertAccountingFill(ctx, f); err != nil {
			return nil, err
		}
		return nil, stager.StagePublication(ctx, tx.Tx, accounting.MarketSubject, publication)
	}); err != nil {
		t.Fatal(err)
	}
	subject, envelope, err := readStagedEnvelope(t, pool, durable, publication.PublicationID)
	if err != nil || subject != accounting.MarketSubject || envelope.EventID != publication.PublicationID || envelope.EmittedAt.Unix() != 1700000000 {
		t.Fatalf("canonical envelope %+v %v", envelope, err)
	}
	wire, err := accounting.CanonicalPublicationV1(publication)
	if err != nil || !bytes.Equal(wire, envelope.Payload) || envelope.PayloadHash != delivery.Hash(wire) {
		t.Fatalf("canonical publication changed %v", err)
	}
	bridge := stagingBridge(t, durable)
	integrated, err := NewDurableStagerWithWebhook(durable, "private-125", subjects, bridge, "g1")
	if err != nil {
		t.Fatal(err)
	}
	wrapped, err := NewDurableWebhookStager(stager, bridge, "g2")
	if err != nil {
		t.Fatal(err)
	}
	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(ctx)
	if err = integrated.StagePublication(ctx, tx, accounting.MarketSubject, publication); err != nil {
		t.Fatal(err)
	}
	if err = wrapped.StagePublication(ctx, tx, accounting.MarketSubject, publication); err != nil {
		t.Fatal(err)
	}
	if err = wrapped.StagePublication(ctx, tx, "unapproved", publication); err == nil {
		t.Fatal("unapproved webhook publication accepted")
	}
	var rows int
	if err = tx.QueryRow(ctx, `SELECT count(*) FROM indexer.delivery_webhooks`).Scan(&rows); err != nil || rows != 2 {
		t.Fatalf("webhook positions %d %v", rows, err)
	}
	event := RawCanonicalEvent{EventType: "Swap", TxHash: f.Position.TransactionHash, TraceID: "canonical", SourceOrder: delivery.SourceOrder{Height: "101", TransactionIndex: "0", LogIndex: "0"}, EmittedAt: time.Unix(1700000000, 0).UTC(), Payload: json.RawMessage(`{"amountRaw":"5000000"}`)}
	if err = stager.Stage(ctx, tx, event); err != nil {
		t.Fatal(err)
	}
	bad := event
	bad.SourceOrder.LogIndex = "-1"
	if err = stager.Stage(ctx, tx, bad); err == nil {
		t.Fatal("negative source index accepted")
	}
	shadow, err := NewShadowStager(stager, durable)
	if err != nil {
		t.Fatal(err)
	}
	rangeDigest := strings.Repeat("a", 64)
	if _, err = tx.Exec(ctx, `INSERT INTO indexer.delivery_archive_manifests(manifest_digest,source_stream,from_height,to_height,raw_count,final_block_hash,manifest) VALUES($1,'staging',101,101,1,$2,$3)`, rangeDigest, raw.BlockHash, wire); err != nil {
		t.Fatal(err)
	}
	if err = shadow.ObserveCanonicalTx(ctx, tx, rangeDigest, "legacy", event); err != nil {
		t.Fatal(err)
	}
	shadowSubject, shadowEnvelope, err := stager.Envelope(event)
	if err != nil {
		t.Fatal(err)
	}
	if err = shadow.ObservePublicationTx(ctx, tx, rangeDigest, "durable", shadowSubject, shadowEnvelope); err != nil {
		t.Fatal(err)
	}
	if err = shadow.ObserveCanonicalTx(ctx, tx, rangeDigest, "legacy", bad); err == nil {
		t.Fatal("invalid shadow source accepted")
	}
	if err = tx.QueryRow(ctx, `SELECT count(*) FROM indexer.delivery_shadow_observations`).Scan(&rows); err != nil || rows != 2 {
		t.Fatalf("shadow observations %d %v", rows, err)
	}
	if err = tx.Commit(ctx); err != nil {
		t.Fatal(err)
	}
}

func TestDurableStagerAdmissionAndCanonicalCreation(t *testing.T) {
	ctx := context.Background()
	canonical, durable, pool := stagingProviders(t)
	p := stagingPublication(t)
	f := *p.Fill
	for _, tc := range []struct {
		store      *delivery.Store
		deployment string
		subjects   map[string]string
	}{{nil, "deployment", map[string]string{"Swap": "chain.swap"}}, {durable, "", map[string]string{"Swap": "chain.swap"}}, {durable, "deployment", nil}, {durable, "deployment", map[string]string{"": "chain.swap"}}, {durable, "deployment", map[string]string{"Swap": ""}}} {
		if _, err := NewDurableStager(tc.store, tc.deployment, tc.subjects); err == nil {
			t.Fatal("invalid durable stager admitted")
		}
	}
	subjects := map[string]string{accounting.MarketSubject: accounting.MarketSubject, CanonicalCreationEventType: CanonicalCreationSubject}
	stager, err := NewDurableStager(durable, "private-125", subjects)
	if err != nil {
		t.Fatal(err)
	}
	for _, s := range []*DurableStager{nil, {}, stager} {
		if err = s.StagePublication(ctx, nil, accounting.MarketSubject, p); err == nil {
			t.Fatal("transactionless accounting staging accepted")
		}
	}
	for _, generation := range []string{"", "bad generation"} {
		if _, err = NewDurableStagerWithWebhook(durable, "private-125", subjects, new(delivery.WebhookBridge), generation); err == nil {
			t.Fatal("invalid webhook generation admitted")
		}
		if _, err = NewDurableWebhookStager(stager, new(delivery.WebhookBridge), generation); err == nil {
			t.Fatal("invalid webhook wrapper admitted")
		}
	}
	if _, err = NewShadowStager(nil, durable); err == nil {
		t.Fatal("missing shadow stager admitted")
	}
	if _, err = NewShadowStager(stager, nil); err == nil {
		t.Fatal("missing shadow store admitted")
	}
	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(ctx)
	for _, subject := range []string{"unapproved", accounting.InventorySubject} {
		if err = stager.StagePublication(ctx, tx, subject, p); err == nil {
			t.Fatal("wrong facet subject accepted")
		}
	}
	invalid := p
	invalid.PublicationID = "changed"
	if err = stager.StagePublication(ctx, tx, accounting.MarketSubject, invalid); err == nil {
		t.Fatal("altered publication accepted")
	}
	other, err := NewDurableStager(durable, "another-deployment", subjects)
	if err != nil {
		t.Fatal(err)
	}
	if err = other.StagePublication(ctx, tx, accounting.MarketSubject, p); err == nil {
		t.Fatal("foreign deployment accepted")
	}
	creation := store.CreationFact{Token: f.Token.Address, Pool: f.PoolAddress, Creator: f.Attribution.TransactionSender, Factory: f.ActualEmitter, Optical: "0x" + strings.Repeat("0", 40), NFTID: "1", Position: f.Position, BlockTimestampSeconds: f.BlockTimestampSeconds, Provenance: f.Provenance}
	if err = stager.StageCanonicalCreationTx(ctx, tx, creation); !errors.Is(err, pgx.ErrNoRows) {
		t.Fatalf("absent creation staged: %v", err)
	}
	if err = tx.Rollback(ctx); err != nil {
		t.Fatal(err)
	}
	fence, err := canonical.AcquireLease(ctx, 125, "creation", "worker", 30*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = pool.Exec(ctx, `INSERT INTO indexer.indexer_ingest_watermarks VALUES(125,'creation',100,100)`); err != nil {
		t.Fatal(err)
	}
	batch := store.CanonicalBatch{ChainID: 125, StreamID: "creation", Owner: "worker", BatchID: "creation", Fence: fence, PreviousHeight: 100, Blocks: []store.BlockCompleteness{{Height: 101, Hash: f.Position.BlockHash, RawCount: 1, FetchComplete: true, RequiredDecodeComplete: true}}, Records: []store.RawRecord{{Height: 101, TransactionHash: f.Position.TransactionHash, BlockHash: f.Position.BlockHash, Emitter: f.ActualEmitter, Topics: []string{}, Data: []byte{1}}}}
	if err = canonical.CommitProjectedCanonicalTx(ctx, batch, func(ctx context.Context, tx store.CanonicalTx) ([]store.OutboxIntent, error) {
		if err := tx.InsertCreation(ctx, creation); err != nil {
			return nil, err
		}
		return nil, stager.StageCanonicalCreationTx(ctx, tx.Tx, creation)
	}); err != nil {
		t.Fatal(err)
	}
	id := accounting.HashTuple("canonical-creation-v1", "125", "private-125", f.Position.BlockHash, f.Position.TransactionHash, f.Position.LogIndex)
	subject, e, err := readStagedEnvelope(t, pool, durable, id)
	if err != nil || subject != CanonicalCreationSubject || e.EventType != CanonicalCreationEventType {
		t.Fatalf("creation envelope %+v %v", e, err)
	}
	var decoded store.CreationFact
	if err = json.Unmarshal(e.Payload, &decoded); err != nil || decoded.Token != creation.Token || decoded.Position != creation.Position {
		t.Fatalf("creation source changed %+v %v", decoded, err)
	}
	tx, err = pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(ctx)
	for _, tc := range []struct {
		name string
		edit func(*store.CreationFact)
	}{
		{"deployment", func(c *store.CreationFact) { c.Provenance.DeploymentID = "foreign" }},
		{"position", func(c *store.CreationFact) { c.Position.ChainID = "1" }},
		{"timestamp", func(c *store.CreationFact) { c.BlockTimestampSeconds = "0" }},
		{"payload", func(c *store.CreationFact) { c.Creator = c.Pool }},
		{"size", func(c *store.CreationFact) {
			c.Metadata = &store.TokenCreationMetadata{Name: strings.Repeat("x", delivery.MaxEnvelopeBytes)}
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			changed := creation
			tc.edit(&changed)
			if err := stager.StageCanonicalCreationTx(ctx, tx, changed); err == nil {
				t.Fatal("altered creation accepted")
			}
		})
	}
}

func stagingFill() accounting.FillV1 {
	f := accounting.FillV1{Version: accounting.FillVersion, Position: accounting.SourcePosition{ChainID: "125", Height: "101", TransactionIndex: "0", LogIndex: "0", TransactionHash: "0x" + strings.Repeat("1", 64), BlockHash: "0x" + strings.Repeat("2", 64)}, ActualEmitter: "0x" + strings.Repeat("3", 40), PoolAddress: "0x" + strings.Repeat("4", 40), Token: accounting.Asset{Address: "0x" + strings.Repeat("5", 40), Decimals: 18}, Settlement: accounting.Asset{Address: "0x" + strings.Repeat("6", 40), Decimals: 6}, Side: "buy", TokenAmountRaw: "1000000000000000000", USDLAmountRaw: "5000000", PaidInputRaw: "5000000", FeeRaw: "15000", InputDerivationQuality: "receipt_incomplete", FeeDisposition: "distributed_usdl", Attribution: accounting.Attribution{TransactionSender: "0x" + strings.Repeat("7", 40), Quality: "unavailable", Evidence: []accounting.EvidenceRef{}}, Execution: accounting.ExecutionPolicy{Quality: "unavailable", Evidence: []accounting.EvidenceRef{}}, Provenance: accounting.Provenance{DeploymentID: "private-125", ABIRevision: "sidiora-v1", EmitterCodeEpoch: "epoch-1", PoolCodeEpoch: "epoch-1", ReceiptSHA256: strings.Repeat("8", 64), HeaderSHA256: strings.Repeat("9", 64)}, BlockTimestampSeconds: "1700000000"}
	f.SourceFillID = accounting.SourceFillIdentity(f.Position, f.Provenance.DeploymentID)
	return f
}

func readStagedEnvelope(t *testing.T, pool *pgxpool.Pool, durable *delivery.Store, id string) (string, delivery.Envelope, error) {
	t.Helper()
	ctx := context.Background()
	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(ctx)
	return durable.OriginalEnvelopeTx(ctx, tx, id)
}
