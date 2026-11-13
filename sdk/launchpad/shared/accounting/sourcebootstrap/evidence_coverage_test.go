package sourcebootstrap_test

import (
	"context"
	"errors"
	"os"
	"strings"
	"testing"

	"github.com/Sidiora-Technologies/KindleLaunch/shared/accounting"
	"github.com/Sidiora-Technologies/KindleLaunch/shared/accounting/sourcebootstrap"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

func TestSourceBootstrapReadsCommittedIndexerEvidence(t *testing.T) {
	dsn := os.Getenv("COORDINATOR_TEST_POSTGRES_DSN")
	if dsn == "" {
		t.Fatal("coordinator PostgreSQL fixture is required")
	}
	ctx := context.Background()
	root, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(root.Close)
	name := "kd_source_" + strings.ReplaceAll(uuid.NewString(), "-", "")[:16]
	if _, err := root.Exec(ctx, "CREATE DATABASE "+pgx.Identifier{name}.Sanitize()); err != nil {
		t.Fatal(err)
	}
	config, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		t.Fatal(err)
	}
	config.ConnConfig.Database = name
	pool, err := pgxpool.NewWithConfig(ctx, config)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		pool.Close()
		if _, err := root.Exec(context.Background(), "DROP DATABASE "+pgx.Identifier{name}.Sanitize()+" WITH (FORCE)"); err != nil {
			t.Error(err)
		}
	})
	const schema = "source_partition"
	ddl := `CREATE SCHEMA source_partition; CREATE SCHEMA IF NOT EXISTS indexer;
CREATE TABLE source_partition.indexer_market_partition_positions_v1(projection_version text, partition_kind text, partition_key text, sequence bigint, position bytea);
CREATE TABLE indexer.indexer_accounting_publications_v1(payload jsonb);
CREATE TABLE indexer.indexer_ingest_commits_v1(chain_id integer,stream_id text,previous_height bigint,last_complete_height bigint,batch_id text,block_hash text);
CREATE TABLE indexer.indexer_canonical_creations_v1(height bigint,block_hash text,transaction_hash text,transaction_index bigint,log_index bigint,factory text,pool text);
CREATE TABLE indexer.indexer_raw_logs_v1(chain_id integer,height bigint,block_hash text,transaction_hash text,transaction_index bigint,log_index bigint,emitter text,topics bytea,data bytea);
CREATE TABLE indexer.indexer_block_completeness_v1(chain_id integer,stream_id text,height bigint,block_hash text,fetch_complete boolean,required_decode_complete boolean);`
	if _, err := pool.Exec(ctx, ddl); err != nil {
		t.Fatal(err)
	}
	fill := accounting.FillV1{
		Version:       accounting.FillVersion,
		Position:      accounting.SourcePosition{ChainID: "125", Height: "100", TransactionIndex: "0", LogIndex: "7", TransactionHash: "0x" + strings.Repeat("a", 64), BlockHash: "0x" + strings.Repeat("b", 64)},
		ActualEmitter: "0x" + strings.Repeat("1", 40), PoolAddress: "0x" + strings.Repeat("2", 40),
		Token: accounting.Asset{Address: "0x" + strings.Repeat("3", 40), Decimals: 18}, Settlement: accounting.Asset{Address: "0x" + strings.Repeat("4", 40), Decimals: 6},
		Side: "buy", TokenAmountRaw: "100", USDLAmountRaw: "50", PaidInputRaw: "50", FeeRaw: "1", InputDerivationQuality: "receipt_incomplete", FeeDisposition: "distributed_usdl",
		Attribution:           accounting.Attribution{TransactionSender: "0x" + strings.Repeat("5", 40), Quality: "unavailable", Evidence: []accounting.EvidenceRef{}},
		Execution:             accounting.ExecutionPolicy{Quality: "unavailable", Evidence: []accounting.EvidenceRef{}},
		Provenance:            accounting.Provenance{DeploymentID: "private-125", ABIRevision: "sidiora-v1", EmitterCodeEpoch: "epoch-1", PoolCodeEpoch: "epoch-1", ReceiptSHA256: strings.Repeat("c", 64), HeaderSHA256: strings.Repeat("d", 64)},
		BlockTimestampSeconds: "1700000000",
	}
	fill.SourceFillID = accounting.SourceFillIdentity(fill.Position, fill.Provenance.DeploymentID)
	digest, err := accounting.FillDigest(fill)
	if err != nil {
		t.Fatal(err)
	}
	position := accounting.PartitionPosition{ProjectionVersion: "market-v1", PartitionKind: "market", PartitionKey: "125:" + fill.PoolAddress, Sequence: "1", SourceEventID: fill.SourceFillID, EffectIdentity: accounting.EffectIdentity(fill.SourceFillID, "market", "market-v1", "", "market", "125:"+fill.PoolAddress), PayloadDigest: digest, SourceOrder: fill.Position}
	publication, err := accounting.NewMarketPublication(fill, []accounting.PartitionPosition{position})
	if err != nil {
		t.Fatal(err)
	}
	positionWire, err := accounting.CanonicalPublicationV1(publication)
	if err != nil {
		t.Fatal(err)
	}
	// The position column stores the position object, while the publication
	// column stores the complete canonical accounting publication.
	positionJSON := []byte(`{"projectionVersion":"market-v1","partitionKind":"market","partitionKey":"125:` + fill.PoolAddress + `","sequence":"1","previousEffectIdentity":"","sourceEventId":"` + fill.SourceFillID + `","effectIdentity":"` + position.EffectIdentity + `","payloadDigest":"` + string(digest) + `","sourceOrder":{"chainId":"125","height":"100","transactionIndex":"0","logIndex":"7","transactionHash":"` + fill.Position.TransactionHash + `","blockHash":"` + fill.Position.BlockHash + `"}}`)
	if _, err := pool.Exec(ctx, `INSERT INTO source_partition.indexer_market_partition_positions_v1 VALUES($1,'market',$2,1,$3)`, "market-v1", "125:"+fill.PoolAddress, positionJSON); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO indexer.indexer_accounting_publications_v1 VALUES($1)`, positionWire); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO indexer.indexer_ingest_commits_v1 VALUES(125,'canonical',98,99,'first',$1),(125,'canonical',99,101,'fill',$2)`, fill.Position.BlockHash, fill.Position.BlockHash); err != nil {
		t.Fatal(err)
	}
	creationHash := "0x" + strings.Repeat("e", 64)
	if _, err := pool.Exec(ctx, `INSERT INTO indexer.indexer_canonical_creations_v1 VALUES(99,$1,$2,0,2,$3,$4)`, creationHash, fill.Position.TransactionHash, fill.ActualEmitter, fill.PoolAddress); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO indexer.indexer_raw_logs_v1 VALUES(125,99,$1,$2,0,2,$3,$4,$5)`, creationHash, fill.Position.TransactionHash, fill.ActualEmitter, []byte{1}, []byte{2}); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO indexer.indexer_block_completeness_v1 VALUES(125,'canonical',99,$1,true,true)`, creationHash); err != nil {
		t.Fatal(err)
	}
	stats, err := sourcebootstrap.LoadStats(ctx, pool, schema, "canonical", "market-v1", fill.PoolAddress)
	if err != nil || stats.FirstCommitID != "fill" || stats.InitialPreviousHeight != 98 || stats.PayloadDigest != digest || stats.Checkpoint() == "" {
		t.Fatalf("stats source bootstrap: %+v, %v", stats, err)
	}
	charts, err := sourcebootstrap.LoadCharts(ctx, pool, schema, "canonical", "market-v1", fill.PoolAddress)
	if err != nil || charts.FirstCommitID != "fill" || charts.ReplayFromHeight != 98 || charts.CreationLogIndex != 2 || charts.Checkpoint() == "" {
		t.Fatalf("chart source bootstrap: %+v, %v", charts, err)
	}
	if _, err := sourcebootstrap.FirstMarketPosition(ctx, pool, schema, "market-v1", "0x"+strings.Repeat("9", 40)); !errors.Is(err, pgx.ErrNoRows) {
		t.Fatalf("unknown pool position: %v", err)
	}
	if _, err := sourcebootstrap.LoadStats(ctx, pool, schema, "canonical", "market-v1", "0x"+strings.Repeat("9", 40)); !errors.Is(err, sourcebootstrap.ErrUnavailable) {
		t.Fatalf("unknown pool bootstrap: %v", err)
	}
}
