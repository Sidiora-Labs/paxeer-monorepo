package accountingtest

import (
	"context"
	"encoding/json"
	"os"
	"strings"
	"testing"

	"github.com/Sidiora-Technologies/KindleLaunch/core/api/internal/internaltest"
	"github.com/Sidiora-Technologies/KindleLaunch/core/api/internal/store"
	"github.com/Sidiora-Technologies/KindleLaunch/shared/accounting"
	"github.com/Sidiora-Technologies/KindleLaunch/shared/accounting/sourcebootstrap"
	"github.com/Sidiora-Technologies/KindleLaunch/shared/delivery"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

const Pool = "0x1111111111111111111111111111111111111111"
const UnknownPool = "0x9999999999999999999999999999999999999999"
const Stream = "canonical"
const Version = "market-v1"
const Generation = "generation1"

const chartsSchema = "candles"
const statsSchema = "stats"
const sourceSchema = "indexer"
const marketPartition = "market"
const leaseOwner = "fixture"

var blockHash = "0x" + strings.Repeat("a", 64)
var creationTx = "0x" + strings.Repeat("b", 64)

type Fixture struct {
	DB     *pgxpool.Pool
	Model  *store.AccountingReadModel
	Config store.AccountingReadConfig
}

func (f Fixture) Exec(t *testing.T, sql string, args ...any) {
	t.Helper()
	if _, err := f.DB.Exec(t.Context(), sql, args...); err != nil {
		t.Fatal(err)
	}
}

func New(t *testing.T) Fixture {
	t.Helper()
	f := Fixture{DB: internaltest.NewPostgres(t)}
	internaltest.ApplyReadMigrations(t, f.DB, "80001", "80002", "100000", "130000", "60000", "80000")
	// The base read schema already contains canonical creations.
	root, err := os.OpenRoot(internaltest.RepositoryRoot(t))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = root.Close() }()
	raw, err := root.ReadFile("migrations/190001.sql")
	if err != nil {
		t.Fatal(err)
	}
	for _, statement := range strings.SplitAfter(string(raw), ";\n") {
		if strings.TrimSpace(statement) == "" || strings.HasPrefix(statement, "CREATE TABLE indexer.indexer_canonical_creations (") || strings.HasPrefix(statement, "CREATE VIEW indexer.indexer_canonical_creations_v1 ") {
			continue
		}
		f.Exec(t, statement)
	}
	f.Config = store.AccountingReadConfig{ChartsSchema: chartsSchema, StatsSchema: statsSchema, ChartsDeliverySchema: chartsSchema, StatsDeliverySchema: statsSchema, ProjectionVersion: Version, ProjectionGeneration: Generation, ChartsConsumerID: "charts", StatsConsumerID: statsSchema, SourceStream: Stream, SourcePartitionSchema: sourceSchema}
	f.Model, err = store.NewAccountingReadModel(f.DB, f.Config)
	if err != nil {
		t.Fatal(err)
	}
	factory := "0x" + strings.Repeat("f", 40)
	f.Exec(t, `INSERT INTO indexer.indexer_raw_logs(chain_id,transaction_hash,log_index,height,transaction_index,block_hash,emitter,topics,data) VALUES(125,$1,1,100,0,$2,$3,'[]',$4)`, creationTx, blockHash, factory, []byte{1})
	f.Exec(t, `INSERT INTO indexer.indexer_canonical_creations(token,pool,creator,factory,optical,nft_id,height,block_hash,transaction_hash,transaction_index,log_index,block_timestamp,provenance,payload) VALUES($1,$2,$3,$4,$3,1,100,$5,$6,0,1,1700000000,'{}','{}')`, "0x"+strings.Repeat("3", 40), Pool, "0x"+strings.Repeat("5", 40), factory, blockHash, creationTx)
	for _, height := range []int{100, 110} {
		f.Exec(t, `INSERT INTO indexer.indexer_block_completeness(chain_id,stream_id,height,block_hash,raw_count,fetch_complete,required_decode_complete) VALUES(125,$1,$2,$3,0,true,true)`, Stream, height, blockHash)
	}
	f.Exec(t, `INSERT INTO indexer.indexer_ingest_commits(chain_id,stream_id,batch_id,previous_height,last_complete_height,block_hash,record_count,outbox_count) VALUES(125,$1,'creation',99,100,$2,0,0),(125,$1,'market',100,110,$2,0,0)`, Stream, blockHash)
	return f
}

func (f Fixture) Publish(t *testing.T) (accounting.FillV1, accounting.PartitionPosition) {
	t.Helper()
	fill := accounting.FillV1{Version: accounting.FillVersion, Position: accounting.SourcePosition{ChainID: "125", Height: "110", TransactionIndex: "0", LogIndex: "0", TransactionHash: "0x" + strings.Repeat("c", 64), BlockHash: blockHash}, ActualEmitter: Pool, PoolAddress: Pool, Token: accounting.Asset{Address: "0x" + strings.Repeat("3", 40), Decimals: 18}, Settlement: accounting.Asset{Address: "0x" + strings.Repeat("4", 40), Decimals: 6}, Side: "buy", TokenAmountRaw: "1000000000000000000", USDLAmountRaw: "1000000", PaidInputRaw: "1000000", FeeRaw: "3000", InputDerivationQuality: "receipt_incomplete", FeeDisposition: "distributed_usdl", Attribution: accounting.Attribution{TransactionSender: "0x" + strings.Repeat("5", 40), Quality: "unavailable", Evidence: []accounting.EvidenceRef{}}, Execution: accounting.ExecutionPolicy{Quality: "unavailable", Evidence: []accounting.EvidenceRef{}}, Provenance: accounting.Provenance{DeploymentID: "fixture125", ABIRevision: "sidiora-v1", EmitterCodeEpoch: "epoch1", PoolCodeEpoch: "epoch1", ReceiptSHA256: strings.Repeat("c", 64), HeaderSHA256: strings.Repeat("d", 64)}, BlockTimestampSeconds: "1700000000"}
	fill.SourceFillID = accounting.SourceFillIdentity(fill.Position, fill.Provenance.DeploymentID)
	digest, err := accounting.FillDigest(fill)
	if err != nil {
		t.Fatal(err)
	}
	producer, err := delivery.NewPartitionStore(f.DB, sourceSchema)
	if err != nil {
		t.Fatal(err)
	}
	tx, err := f.DB.Begin(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = tx.Rollback(t.Context()) }()
	position, err := producer.AppendTx(t.Context(), tx, delivery.AppendInput{ProjectionVersion: Version, PartitionKind: marketPartition, PartitionKey: "125:" + Pool, SourceEventID: fill.SourceFillID, EffectIdentity: accounting.EffectIdentity(fill.SourceFillID, marketPartition, Version, "", marketPartition, "125:"+Pool), PayloadDigest: digest, SourceOrder: fill.Position})
	if err != nil {
		t.Fatal(err)
	}
	if err = tx.Commit(t.Context()); err != nil {
		t.Fatal(err)
	}
	publication, err := accounting.NewMarketPublication(fill, []accounting.PartitionPosition{position})
	if err != nil {
		t.Fatal(err)
	}
	wire, err := accounting.CanonicalPublicationV1(publication)
	if err != nil {
		t.Fatal(err)
	}
	f.Exec(t, `INSERT INTO indexer.indexer_raw_logs(chain_id,transaction_hash,log_index,height,transaction_index,block_hash,emitter,topics,data) VALUES(125,$1,0,110,0,$2,$3,'[]',$4)`, fill.Position.TransactionHash, blockHash, Pool, []byte{2})
	f.Exec(t, `INSERT INTO indexer.indexer_ingest_outbox(chain_id,transaction_hash,log_index,event_kind,payload) VALUES(125,$1,0,'accounting.market.v1',$2)`, fill.Position.TransactionHash, wire)
	return fill, position
}

func (f Fixture) Commit(t *testing.T, fill accounting.FillV1, position accounting.PartitionPosition) {
	t.Helper()
	ctx := t.Context()
	charts, err := sourcebootstrap.LoadCharts(ctx, f.DB, sourceSchema, Stream, Version, Pool)
	if err != nil {
		t.Fatal(err)
	}
	stats, err := sourcebootstrap.LoadStats(ctx, f.DB, sourceSchema, Stream, Version, Pool)
	if err != nil {
		t.Fatal(err)
	}
	for _, binding := range []struct {
		schema, consumer string
		checkpoint       accounting.Digest
	}{{chartsSchema, "charts", charts.Checkpoint()}, {statsSchema, statsSchema, stats.Checkpoint()}} {
		f.Exec(t, "INSERT INTO "+pgx.Identifier{binding.schema, "delivery_partition_leases"}.Sanitize()+" VALUES($1,$2,'market',$3,'fixture',1,now()+interval '1 hour')", binding.consumer, Generation, position.PartitionKey)
		partitions, err := delivery.NewPartitionStore(f.DB, binding.schema)
		if err != nil {
			t.Fatal(err)
		}
		tx, err := f.DB.Begin(ctx)
		if err != nil {
			t.Fatal(err)
		}
		lease := delivery.PartitionLease{Key: delivery.ConsumerKey{ConsumerID: binding.consumer, ProjectionGeneration: Generation, PartitionKind: marketPartition, PartitionKey: position.PartitionKey}, Owner: leaseOwner, Epoch: 1}
		if err = partitions.BootstrapTx(ctx, tx, delivery.BootstrapInput{Lease: lease, CheckpointDigest: binding.checkpoint, NextSequence: "1"}, func(ctx context.Context, tx pgx.Tx, in delivery.BootstrapInput) error {
			if binding.schema == chartsSchema {
				evidence, err := sourcebootstrap.LoadCharts(ctx, tx, sourceSchema, Stream, Version, Pool)
				if err != nil {
					return err
				}
				if evidence.Checkpoint() != in.CheckpointDigest {
					return delivery.ErrIntegrity
				}
			} else {
				evidence, err := sourcebootstrap.LoadStats(ctx, tx, sourceSchema, Stream, Version, Pool)
				if err != nil {
					return err
				}
				if evidence.Checkpoint() != in.CheckpointDigest {
					return delivery.ErrIntegrity
				}
			}
			return nil
		}); err != nil {
			_ = tx.Rollback(ctx)
			t.Fatal(err)
		}
		guard, err := partitions.CheckAndLockTx(ctx, tx, lease, position)
		if err != nil {
			_ = tx.Rollback(ctx)
			t.Fatal(err)
		}
		if err = partitions.AdvanceTx(ctx, tx, guard); err != nil {
			_ = tx.Rollback(ctx)
			t.Fatal(err)
		}
		if err = tx.Commit(ctx); err != nil {
			t.Fatal(err)
		}
	}
	publication, err := accounting.NewMarketPublication(fill, []accounting.PartitionPosition{position})
	if err != nil {
		t.Fatal(err)
	}
	fillRaw, err := json.Marshal(fill)
	if err != nil {
		t.Fatal(err)
	}
	positionRaw, err := json.Marshal(position)
	if err != nil {
		t.Fatal(err)
	}
	f.Exec(t, `INSERT INTO stats.accounting_market_commits(publication_id,source_fill_id,source_fact_digest,projection_version,pool,bucket_start,position,fill,sequence,revision) VALUES($1,$2,$3,$4,$5,1699999980,$6,$7,1,1)`, publication.PublicationID, fill.SourceFillID, string(position.PayloadDigest), Version, Pool, positionRaw, fillRaw)
	f.Exec(t, `INSERT INTO candles.chart_pool_fences VALUES('125',$1,1,false)`, Pool)
	f.Exec(t, `INSERT INTO stats.accounting_pool_fences(pool,frozen) VALUES($1,false)`, Pool)
	source := map[string]any{"sourceFillId": fill.SourceFillID, "position": fill.Position, "actualEmitter": fill.ActualEmitter, "provenance": fill.Provenance}
	price := map[string]string{"numerator": "1", "denominator": "1"}
	candle, err := json.Marshal(map[string]any{"poolAddress": Pool, "interval": "1m", "start": int64(1699999980), "open": price, "high": price, "low": price, "close": price, "openSource": source, "highSource": source, "lowSource": source, "closeSource": source, "volumeUsdlRaw": "1000000", "volumeTokenRaw": fill.TokenAmountRaw, "buyVolumeUsdlRaw": "1000000", "sellVolumeUsdlRaw": "0", "trades": 1, "revision": 1, "token": fill.Token, "settlement": fill.Settlement})
	if err != nil {
		t.Fatal(err)
	}
	f.Exec(t, `INSERT INTO candles.chart_exact VALUES('125',$1,'1m',1699999980,$2)`, Pool, candle)
}
