package production_test

import (
	"bytes"
	"context"
	"os"
	"testing"

	"github.com/Sidiora-Technologies/KindleLaunch/shared/db/production"
	"github.com/Sidiora-Technologies/KindleLaunch/shared/db/production/coordinatortest"
	"github.com/jackc/pgx/v5/pgxpool"
)

func TestRecoverySnapshotReadsCommittedDatabase(t *testing.T) {
	dsn := os.Getenv("COORDINATOR_TEST_POSTGRES_DSN")
	if dsn == "" {
		t.Fatal("private coordinator database required")
	}
	database := coordinatortest.Bootstrap(t, dsn, "recovery_runtime")
	schemaFile, schemaDigest := database.ExpectedSchemaFile(t, "recovery_runtime")
	schemaWire, err := os.ReadFile(schemaFile)
	if err != nil || !bytes.Equal(schemaWire, database.Runtimes["recovery_runtime"].ExpectedSchema) || production.Checksum(schemaWire) != schemaDigest {
		t.Fatalf("runtime schema identity: %v", err)
	}
	schemaInfo, err := os.Stat(schemaFile)
	if err != nil || schemaInfo.Mode().Perm() != 0600 {
		t.Fatalf("runtime schema permissions: %v", err)
	}

	pool, err := pgxpool.New(context.Background(), database.AdminDSN)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	if _, err := pool.Exec(context.Background(), `CREATE SCHEMA IF NOT EXISTS indexer;
CREATE SCHEMA IF NOT EXISTS stats;
CREATE TABLE indexer.indexer_raw_logs(chain_id bigint NOT NULL,height bigint NOT NULL,transaction_index bigint NOT NULL,log_index bigint NOT NULL,transaction_hash text NOT NULL,PRIMARY KEY(chain_id,transaction_hash,log_index));
CREATE TABLE indexer.indexer_ingest_outbox(chain_id bigint NOT NULL,transaction_hash text NOT NULL,log_index bigint NOT NULL);
CREATE TABLE indexer.delivery_outbox(event_id text PRIMARY KEY,state text NOT NULL,broker_sequence bigint);
CREATE TABLE stats.trade_facts(volume numeric NOT NULL,fee numeric NOT NULL);`); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(context.Background(), `INSERT INTO indexer.indexer_raw_logs VALUES(125,1,0,0,'transaction-one');
INSERT INTO indexer.indexer_ingest_outbox VALUES(125,'transaction-one',0);
INSERT INTO indexer.delivery_outbox VALUES('event-one','published',7);
INSERT INTO stats.trade_facts VALUES(100,3);`); err != nil {
		t.Fatal(err)
	}
	snapshot, err := production.CaptureRecoverySnapshot(context.Background(), pool)
	if err != nil {
		t.Fatal(err)
	}
	emptyDigest := production.Checksum([]byte("[]"))
	if snapshot.CanonicalCount != 1 || snapshot.TradeCount != 1 || snapshot.OutboxCount != 1 || snapshot.Volume != "100" || snapshot.Fees != "3" || snapshot.Published != 1 || snapshot.Pending != 0 || snapshot.BrokerSequence != 7 || snapshot.CanonicalSHA256 == emptyDigest || snapshot.OutboxSHA256 == emptyDigest {
		t.Fatalf("committed recovery database = %+v", snapshot)
	}
	if err := production.ReconcileRecoverySnapshot(snapshot, snapshot); err != nil {
		t.Fatalf("matching recovery snapshot: %v", err)
	}
	if _, err := pool.Exec(context.Background(), `INSERT INTO indexer.indexer_raw_logs VALUES(1,2,0,0,'wrong-chain')`); err != nil {
		t.Fatal(err)
	}
	if _, err := production.CaptureRecoverySnapshot(context.Background(), pool); err == nil {
		t.Fatal("other chain entered recovery snapshot")
	}
	if _, err := pool.Exec(context.Background(), `DELETE FROM indexer.indexer_raw_logs WHERE chain_id=1; INSERT INTO indexer.indexer_ingest_outbox VALUES(125,'orphan',0)`); err != nil {
		t.Fatal(err)
	}
	if _, err := production.CaptureRecoverySnapshot(context.Background(), pool); err == nil {
		t.Fatal("orphaned ingest event entered recovery snapshot")
	}
	if _, err := production.CaptureRecoverySnapshot(context.Background(), nil); err == nil {
		t.Fatal("missing recovery database admitted")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := production.CaptureRecoverySnapshot(ctx, pool); err == nil {
		t.Fatal("canceled recovery snapshot admitted")
	}
}
