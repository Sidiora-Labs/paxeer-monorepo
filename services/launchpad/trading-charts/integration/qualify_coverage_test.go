package integration

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/Sidiora-Technologies/KindleLaunch/core/trading-charts/internal/internaltest"
	"github.com/Sidiora-Technologies/KindleLaunch/shared/accounting"
	"github.com/Sidiora-Technologies/KindleLaunch/shared/delivery"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/nats-io/nats.go/jetstream"
)

func chartFaultDigest(t *testing.T, value any) string {
	t.Helper()
	wire, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	digest := sha256.Sum256(wire)
	return hex.EncodeToString(digest[:])
}

func chartFaultFill(t *testing.T, index int, chain internaltest.MarketChain) accounting.FillV1 {
	t.Helper()
	receipt := chain.Swaps[index]
	log := receipt.Logs[0]
	poolAddress := strings.ToLower(chain.Pool.Hex())
	fill := accounting.FillV1{
		Version:       accounting.FillVersion,
		Position:      accounting.SourcePosition{ChainID: "125", Height: strconv.FormatUint(log.BlockNumber, 10), TransactionIndex: strconv.FormatUint(uint64(log.TxIndex), 10), LogIndex: strconv.FormatUint(uint64(log.Index), 10), TransactionHash: log.TxHash.Hex(), BlockHash: log.BlockHash.Hex()},
		ActualEmitter: strings.ToLower(log.Address.Hex()), PoolAddress: poolAddress,
		Token: accounting.Asset{Address: strings.ToLower(chain.Token.Hex()), Decimals: 18}, Settlement: accounting.Asset{Address: strings.ToLower(chain.Settlement.Hex()), Decimals: 6},
		Side: "buy", TokenAmountRaw: "2000000", USDLAmountRaw: "1000000", PaidInputRaw: "1000000", FeeRaw: "1000", FeeDisposition: "distributed_usdl", InputDerivationQuality: "receipt_incomplete", BlockTimestampSeconds: strconv.FormatInt(chain.Headers[int64(log.BlockNumber)].Time, 10),
		Attribution: accounting.Attribution{TransactionSender: strings.ToLower(chain.Sender.Hex()), Quality: "unavailable", Evidence: []accounting.EvidenceRef{}},
		Execution:   accounting.ExecutionPolicy{Quality: "unavailable", Evidence: []accounting.EvidenceRef{}},
		Provenance:  accounting.Provenance{DeploymentID: "chart-fault-local", ABIRevision: "sidiora-v1", EmitterCodeEpoch: "epoch-1", PoolCodeEpoch: "epoch-1", ReceiptSHA256: chartFaultDigest(t, receipt), HeaderSHA256: chartFaultDigest(t, chain.Headers[int64(log.BlockNumber)])},
	}
	fill.SourceFillID = accounting.SourceFillIdentity(fill.Position, fill.Provenance.DeploymentID)
	return fill
}

func chartFaultSource(t *testing.T, pool *pgxpool.Pool, chain internaltest.MarketChain) {
	t.Helper()
	ctx := context.Background()
	var err error
	if _, err := pool.Exec(ctx, `CREATE SCHEMA indexer; CREATE SCHEMA candles`); err != nil {
		t.Fatal(err)
	}
	migrationDirectory := filepath.Join("..", "..", "..", "..", "launchpad", "spec", "production-system", "integration", "migrations")
	for _, name := range []string{"60000.sql", "80000.sql", "80001.sql", "100000.sql", "100001.sql", "150002.sql", "150003.sql", "190001.sql"} {
		migration, err := os.ReadFile(filepath.Join(migrationDirectory, name))
		if err != nil {
			t.Fatal(err)
		}
		tx, err := pool.Begin(ctx)
		if err != nil {
			t.Fatal(err)
		}
		if _, err = tx.Exec(ctx, string(migration)); err != nil {
			_ = tx.Rollback(ctx)
			t.Fatalf("market schema %s: %v", name, err)
		}
		if err = tx.Commit(ctx); err != nil {
			t.Fatal(err)
		}
	}
	if _, err = pool.Exec(ctx, `CREATE SCHEMA recovery`); err != nil {
		t.Fatal(err)
	}
	recoverySQL, err := os.ReadFile(filepath.Join(migrationDirectory, "440000.sql"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err = pool.Exec(ctx, string(recoverySQL)); err != nil {
		t.Fatal(err)
	}
	if _, err = pool.Exec(ctx, `INSERT INTO recovery.recovery_control(singleton,epoch,region,revision,checkpoint_sha256) VALUES(true,1,'local',1,$1)`, strings.Repeat("a", 64)); err != nil {
		t.Fatal(err)
	}
	write := func(sql string, args ...any) {
		t.Helper()
		if _, err := pool.Exec(ctx, sql, args...); err != nil {
			t.Fatal(err)
		}
	}
	stream := "canonical"
	counts := map[int64]int{}
	for _, receipt := range append([]*types.Receipt{chain.Creation}, chain.Swaps...) {
		for _, log := range receipt.Logs {
			topics := make([]string, len(log.Topics))
			for i, topic := range log.Topics {
				topics[i] = topic.Hex()
			}
			encoded, err := json.Marshal(topics)
			if err != nil {
				t.Fatal(err)
			}
			write(`INSERT INTO indexer.indexer_raw_logs(chain_id,transaction_hash,log_index,height,transaction_index,block_hash,emitter,topics,data) VALUES(125,$1,$2,$3,$4,$5,$6,$7,$8)`, log.TxHash.Hex(), int64(log.Index), int64(log.BlockNumber), int64(log.TxIndex), log.BlockHash.Hex(), strings.ToLower(log.Address.Hex()), encoded, log.Data)
			counts[int64(log.BlockNumber)]++
		}
	}
	last := chain.Swaps[1].BlockNumber.Int64()
	for height := int64(1); height <= last; height++ {
		write(`INSERT INTO indexer.indexer_block_completeness(chain_id,stream_id,height,block_hash,raw_count,fetch_complete,required_decode_complete) VALUES(125,$1,$2,$3,$4,true,true)`, stream, height, chain.Headers[height].Hash, counts[height])
	}
	commit := func(id string, previous, height int64) {
		t.Helper()
		count := 0
		for current := previous + 1; current <= height; current++ {
			count += counts[current]
		}
		write(`INSERT INTO indexer.indexer_ingest_commits(chain_id,stream_id,batch_id,previous_height,last_complete_height,block_hash,record_count,outbox_count) VALUES(125,$1,$2,$3,$4,$5,$6,0)`, stream, id, previous, height, chain.Headers[height].Hash, count)
	}
	creationHeight := chain.Creation.BlockNumber.Int64()
	commit("activation", 0, creationHeight-1)
	commit("creation", creationHeight-1, creationHeight)
	commit("trades", creationHeight, last)
	write(`INSERT INTO indexer.indexer_ingest_watermarks(chain_id,stream_id,raw_complete_height,typed_complete_height) VALUES(125,$1,$2,$2)`, stream, last)
	write(`INSERT INTO indexer.indexer_ingest_leases(chain_id,stream_id,owner_instance,fence_token,expires_at) VALUES(125,$1,'local-market',1,clock_timestamp()+interval '1 hour')`, stream)
	created := chain.Creation.Logs[0]
	write(`INSERT INTO indexer.indexer_canonical_creations(token,pool,creator,factory,optical,nft_id,height,block_hash,transaction_hash,transaction_index,log_index,block_timestamp,provenance,payload) VALUES($1,$2,$3,$4,$5,'1',$6,$7,$8,$9,$10,$11,'{}','{}')`, strings.ToLower(chain.Token.Hex()), strings.ToLower(chain.Pool.Hex()), strings.ToLower(chain.Sender.Hex()), strings.ToLower(created.Address.Hex()), strings.ToLower(chain.Token.Hex()), creationHeight, created.BlockHash.Hex(), created.TxHash.Hex(), int64(created.TxIndex), int64(created.Index), chain.Headers[creationHeight].Time)
	partitions, err := delivery.NewPartitionStore(pool, "indexer")
	if err != nil {
		t.Fatal(err)
	}
	policy := delivery.Policy{Subjects: map[string]bool{accounting.MarketSubject: true}, EventTypes: map[string]bool{"market": true}}
	for index := 0; index < 2; index++ {
		fill := chartFaultFill(t, index, chain)
		if err = accounting.ValidateFillV1(fill); err != nil {
			t.Fatal(err)
		}
		digest, err := accounting.FillDigest(fill)
		if err != nil {
			t.Fatal(err)
		}
		body, err := json.Marshal(fill)
		if err != nil {
			t.Fatal(err)
		}
		write(`INSERT INTO indexer.indexer_accounting_fills(source_fill_id,source_fact_digest,chain_id,transaction_hash,log_index,pool_address,payload) VALUES($1,$2,125,$3,$4,$5,$6)`, fill.SourceFillID, string(digest), fill.Position.TransactionHash, int64(chain.Swaps[index].Logs[0].Index), fill.PoolAddress, body)
		tx, err := pool.Begin(ctx)
		if err != nil {
			t.Fatal(err)
		}
		position, err := partitions.AppendTx(ctx, tx, delivery.AppendInput{ProjectionVersion: "market-v1", PartitionKind: "market", PartitionKey: "125:" + fill.PoolAddress, SourceEventID: fill.SourceFillID, EffectIdentity: accounting.EffectIdentity(fill.SourceFillID, "market", "market-v1", "", "market", "125:"+fill.PoolAddress), PayloadDigest: digest, SourceOrder: fill.Position})
		if err != nil {
			_ = tx.Rollback(ctx)
			t.Fatal(err)
		}
		publication, err := accounting.NewMarketPublication(fill, []accounting.PartitionPosition{position})
		if err != nil {
			_ = tx.Rollback(ctx)
			t.Fatal(err)
		}
		payload, err := accounting.CanonicalPublicationV1(publication)
		if err != nil {
			_ = tx.Rollback(ctx)
			t.Fatal(err)
		}
		envelope := delivery.Envelope{EventID: publication.PublicationID, EventType: "market", SchemaVersion: 1, ChainID: 125, SourceOrder: delivery.SourceOrder{Height: fill.Position.Height, TransactionIndex: fill.Position.TransactionIndex, LogIndex: fill.Position.LogIndex}, EmittedAt: time.Now().UTC(), TraceID: "chart-fault-local", PayloadHash: delivery.Hash(payload), Payload: payload}
		wire, err := envelope.Encode(policy, accounting.MarketSubject)
		if err != nil {
			_ = tx.Rollback(ctx)
			t.Fatal(err)
		}
		if _, err = tx.Exec(ctx, `INSERT INTO indexer.indexer_ingest_outbox(chain_id,transaction_hash,log_index,event_kind,payload) VALUES(125,$1,$2,'accounting.market.v1',$3)`, fill.Position.TransactionHash, int64(chain.Swaps[index].Logs[0].Index), payload); err != nil {
			_ = tx.Rollback(ctx)
			t.Fatal(err)
		}
		if _, err = tx.Exec(ctx, `INSERT INTO indexer.delivery_outbox(event_id,schema_version,subject,payload_hash,envelope) VALUES($1,1,$2,$3,$4)`, publication.PublicationID, accounting.MarketSubject, string(delivery.Hash(payload)), wire); err != nil {
			_ = tx.Rollback(ctx)
			t.Fatal(err)
		}
		if err = tx.Commit(ctx); err != nil {
			t.Fatal(err)
		}
	}
}

func TestFaultScenariosPreserveOrderedAndPoisonedDelivery(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	pool := internaltest.OpenPool(t, internaltest.NewDatabase(t))
	chain := internaltest.NewMarketChain(t)
	poolAddress := strings.ToLower(chain.Pool.Hex())
	chartFaultSource(t, pool, chain)
	if chain.Swaps[1].BlockNumber.Int64() == 0 {
		t.Fatal("canonical RPC has no committed block")
	}
	nc, stream := internaltest.NewBroker(t, "CHART_FAULT", []string{accounting.MarketSubject})
	config := Config{Pool: pool, NATS: nc, RPC: chain.RPC, Stream: "CHART_FAULT", Schema: "kd_charts", SourceSchema: "indexer", SourceStream: "canonical", ConsumerID: "charts", Generation: "local", Owner: "local", ProjectionVersion: "market-v1", Region: "local", PoolAddress: poolAddress, Epoch: 1, MaxReplayBlocks: 4, MaxReplayFills: 10}
	for _, scenario := range []string{"stale-partition-owner", "ordered-gap", "poison-quarantine"} {
		if err := RunFaultScenario(ctx, scenario, config); err != nil {
			t.Fatalf("%s: %v", scenario, err)
		}
	}
	if err := RunFaultScenario(ctx, "unregistered", config); err == nil || !strings.Contains(err.Error(), "actual owned resources") {
		t.Fatalf("unregistered fault scenario: %v", err)
	}
	withoutProjection := config
	withoutProjection.ProjectionVersion = "other-projection"
	if err := RunFaultScenario(ctx, "ordered-gap", withoutProjection); err == nil || !strings.Contains(err.Error(), "contiguous committed source") {
		t.Fatalf("missing canonical projection: %v", err)
	}
	if err := RunFaultScenario(ctx, "ordered-gap", config); err == nil || !strings.Contains(err.Error(), "fresh exact owned schema") {
		t.Fatalf("owned fault schema reuse: %v", err)
	}
	rows, err := pool.Query(ctx, `SELECT event_id,envelope FROM indexer.delivery_outbox ORDER BY event_id`)
	if err != nil {
		t.Fatal(err)
	}
	type sourceWire struct {
		id   string
		wire []byte
	}
	wires := []sourceWire{}
	for rows.Next() {
		var value sourceWire
		if err = rows.Scan(&value.id, &value.wire); err != nil {
			rows.Close()
			t.Fatal(err)
		}
		wires = append(wires, value)
	}
	rows.Close()
	if err = rows.Err(); err != nil || len(wires) != 2 {
		t.Fatalf("source publication wires: %d, %v", len(wires), err)
	}
	for index, changed := range [][]byte{[]byte("invalid"), wires[1].wire} {
		name := "invalid envelope"
		if index == 1 {
			name = "unbound publication"
		}
		t.Run(name, func(t *testing.T) {
			if _, err := pool.Exec(ctx, `UPDATE indexer.delivery_outbox SET envelope=$2 WHERE event_id=$1`, wires[0].id, changed); err == nil || !strings.Contains(err.Error(), "immutable delivery envelope") {
				t.Fatalf("committed source envelope changed: %v", err)
			}
			var stored []byte
			if err := pool.QueryRow(ctx, `SELECT envelope FROM indexer.delivery_outbox WHERE event_id=$1`, wires[0].id).Scan(&stored); err != nil || string(stored) != string(wires[0].wire) {
				t.Fatalf("source envelope after immutable write refusal: %v", err)
			}
		})
	}
	longDurable := config
	longDurable.ConsumerID = strings.Repeat("c", 101)
	if err := RunFaultScenario(ctx, "ordered-gap", longDurable); err == nil || !strings.Contains(err.Error(), "bounded owned durable name required") {
		t.Fatalf("bounded broker identity: %v", err)
	}
	invalidRegion := config
	invalidRegion.Schema, invalidRegion.Region = "kd_invalid_region", "invalid region"
	if err := RunFaultScenario(ctx, "ordered-gap", invalidRegion); err == nil || !strings.Contains(err.Error(), "invalid recovery authority") {
		t.Fatalf("recovery region identity: %v", err)
	}
	unboundedStream := config
	unboundedStream.Schema, unboundedStream.SourceStream = "kd_unbounded_stream", strings.Repeat("s", 129)
	if err := RunFaultScenario(ctx, "ordered-gap", unboundedStream); err == nil || !strings.Contains(err.Error(), "bounded replay configuration") {
		t.Fatalf("canonical stream identity: %v", err)
	}
	if _, err := pool.Exec(ctx, `ALTER TABLE indexer.delivery_outbox DISABLE TRIGGER delivery_immutable`); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `UPDATE indexer.delivery_outbox SET envelope='invalid'::bytea WHERE event_id=$1`, wires[0].id); err != nil {
		t.Fatal(err)
	}
	badEnvelope := config
	badEnvelope.Schema = "kd_source_envelope"
	if err := RunFaultScenario(ctx, "ordered-gap", badEnvelope); err == nil || !strings.Contains(err.Error(), "invalid character") {
		t.Fatalf("unreadable canonical envelope: %v", err)
	}
	if _, err := pool.Exec(ctx, `UPDATE indexer.delivery_outbox SET envelope=$2 WHERE event_id=$1`, wires[0].id, wires[0].wire); err != nil {
		t.Fatal(err)
	}
	policy := delivery.Policy{Subjects: map[string]bool{accounting.MarketSubject: true}, EventTypes: map[string]bool{"market": true}}
	envelope, err := delivery.Decode(wires[0].wire, policy, accounting.MarketSubject)
	if err != nil {
		t.Fatal(err)
	}
	envelope.Payload = json.RawMessage(`{"version":"accounting-publication-v1"}`)
	envelope.PayloadHash = delivery.Hash(envelope.Payload)
	badWire, err := envelope.Encode(policy, accounting.MarketSubject)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `UPDATE indexer.delivery_outbox SET envelope=$2 WHERE event_id=$1`, wires[0].id, badWire); err != nil {
		t.Fatal(err)
	}
	badPublication := config
	badPublication.Schema = "kd_source_publication"
	if err := RunFaultScenario(ctx, "ordered-gap", badPublication); err == nil || !strings.Contains(err.Error(), "accounting") {
		t.Fatalf("unreadable canonical publication: %v", err)
	}
	if _, err := pool.Exec(ctx, `UPDATE indexer.delivery_outbox SET envelope=$2 WHERE event_id=$1`, wires[0].id, wires[0].wire); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `ALTER TABLE indexer.delivery_outbox ENABLE TRIGGER delivery_immutable`); err != nil {
		t.Fatal(err)
	}
	var positionWire []byte
	if err := pool.QueryRow(ctx, `SELECT position FROM indexer.delivery_partition_positions WHERE sequence=1 AND partition_kind='market'`).Scan(&positionWire); err != nil {
		t.Fatal(err)
	}
	var changedPosition accounting.PartitionPosition
	if err := json.Unmarshal(positionWire, &changedPosition); err != nil {
		t.Fatal(err)
	}
	var malformedPosition map[string]any
	if err := json.Unmarshal(positionWire, &malformedPosition); err != nil {
		t.Fatal(err)
	}
	malformedPosition["sequence"] = map[string]any{"value": "1"}
	malformedWire, err := json.Marshal(malformedPosition)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `UPDATE indexer.delivery_partition_positions SET position=$1 WHERE sequence=1 AND partition_kind='market'`, malformedWire); err != nil {
		t.Fatal(err)
	}
	malformed := config
	malformed.Schema = "kd_malformed_position"
	if err := RunFaultScenario(ctx, "ordered-gap", malformed); err == nil || !strings.Contains(err.Error(), "cannot unmarshal") {
		t.Fatalf("unreadable source position: %v", err)
	}
	if _, err := pool.Exec(ctx, `UPDATE indexer.delivery_partition_positions SET position=$1 WHERE sequence=1 AND partition_kind='market'`, positionWire); err != nil {
		t.Fatal(err)
	}
	changedPosition.Sequence = "2"
	changedWire, err := json.Marshal(changedPosition)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `UPDATE indexer.delivery_partition_positions SET position=$1 WHERE sequence=1 AND partition_kind='market'`, changedWire); err != nil {
		t.Fatal(err)
	}
	badPosition := config
	badPosition.Schema = "kd_source_position"
	if err := RunFaultScenario(ctx, "ordered-gap", badPosition); err == nil || !strings.Contains(err.Error(), "source outbox/publication/position disagreement") {
		t.Fatalf("changed canonical position: %v", err)
	}
	if _, err := pool.Exec(ctx, `UPDATE indexer.delivery_partition_positions SET position=$1 WHERE sequence=1 AND partition_kind='market'`, positionWire); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `ALTER VIEW indexer.indexer_market_partition_positions_v1 RENAME TO indexer_market_partition_positions_unavailable`); err != nil {
		t.Fatal(err)
	}
	missingSource := config
	missingSource.Schema = "kd_charts_unavailable"
	result := RunFaultScenario(ctx, "ordered-gap", missingSource)
	if _, err := pool.Exec(ctx, `ALTER VIEW indexer.indexer_market_partition_positions_unavailable RENAME TO indexer_market_partition_positions_v1`); err != nil {
		t.Fatal(err)
	}
	if result == nil || !strings.Contains(result.Error(), "indexer_market_partition_positions_v1") {
		t.Fatalf("unavailable canonical source: %v", result)
	}
	missingStream := config
	missingStream.Schema, missingStream.Stream = "kd_charts_missing", "CHART_MISSING"
	if err := RunFaultScenario(ctx, "ordered-gap", missingStream); err == nil {
		t.Fatal("missing retained broker stream admitted")
	}
	invalidDurable := config
	invalidDurable.Schema, invalidDurable.ConsumerID = "kd_invalid_durable", "invalid durable"
	if err := RunFaultScenario(ctx, "ordered-gap", invalidDurable); err == nil || !strings.Contains(err.Error(), "invalid bounded durable configuration") {
		t.Fatalf("broker durable identity: %v", err)
	}
	js, err := jetstream.New(nc)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := js.CreateStream(ctx, jetstream.StreamConfig{Name: "CHART_VOLATILE", Subjects: []string{"chart.volatile"}, Storage: jetstream.MemoryStorage}); err != nil {
		t.Fatal(err)
	}
	volatile := config
	volatile.Schema, volatile.Stream = "kd_charts_volatile", "CHART_VOLATILE"
	if err := RunFaultScenario(ctx, "ordered-gap", volatile); err == nil || !strings.Contains(err.Error(), "file-backed stream") {
		t.Fatalf("volatile broker stream: %v", err)
	}
	info, err := stream.Info(ctx)
	if err != nil || info.State.Msgs < 3 {
		t.Fatalf("retained broker fault messages: %+v, %v", info, err)
	}
	var sourceCount int
	if err = pool.QueryRow(ctx, `SELECT count(*) FROM indexer.indexer_accounting_publications_v1`).Scan(&sourceCount); err != nil || sourceCount != 2 {
		t.Fatalf("committed canonical source publications: %d, %v", sourceCount, err)
	}
}
