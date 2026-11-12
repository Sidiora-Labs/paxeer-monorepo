package consumer

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/big"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/Sidiora-Technologies/KindleLaunch/core/stats-workers/internal/internaltest"
	"github.com/Sidiora-Technologies/KindleLaunch/core/stats-workers/internal/projectionstats"
	"github.com/Sidiora-Technologies/KindleLaunch/shared/accounting"
	"github.com/Sidiora-Technologies/KindleLaunch/shared/delivery"
	"github.com/Sidiora-Technologies/KindleLaunch/shared/delivery/natsdriver"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/nats-io/nats.go/jetstream"
)

func localAccountingChain(t *testing.T) (*ethclient.Client, types.Log, int64) {
	t.Helper()
	port, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	address := port.Addr().(*net.TCPAddr).Port
	if err := port.Close(); err != nil {
		t.Fatal(err)
	}
	command := exec.Command("anvil", "--chain-id", "125", "--host", "127.0.0.1", "--port", strconv.Itoa(address), "--silent")
	var stderr bytes.Buffer
	command.Stdout, command.Stderr = io.Discard, &stderr
	if err := command.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = command.Process.Kill(); _ = command.Wait() })
	url := fmt.Sprintf("http://127.0.0.1:%d", address)
	client, err := ethclient.Dial(url)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(client.Close)
	ctx := context.Background()
	ready := false
	for i := 0; i < 100; i++ {
		id, err := client.ChainID(ctx)
		if err == nil && id.Cmp(big.NewInt(125)) == 0 {
			ready = true
			break
		}
		time.Sleep(25 * time.Millisecond)
	}
	if !ready {
		t.Fatalf("chain unavailable: %s", stderr.String())
	}
	emitter := common.HexToAddress("0x0000000000000000000000000000000000000099")
	pool := common.HexToAddress("0x0000000000000000000000000000000000000098")
	signature := crypto.Keccak256Hash([]byte("Swap(bytes32,address,bool,uint256,uint256,uint256,uint256,uint256,uint256)"))
	words := []int64{1, 1000, 2000, 3, 1, time.Now().Unix() + 5, 0}
	data := make([]byte, 0, 7*32)
	for _, value := range words {
		data = append(data, big.NewInt(value).FillBytes(make([]byte, 32))...)
	}
	code := []byte{0x61, 0, byte(len(data)), 0x61, 0, 82, 0x60, 0, 0x39, 0x7f}
	code = append(code, common.BytesToHash(pool.Bytes()).Bytes()...)
	code = append(code, 0x7f)
	code = append(code, signature.Bytes()...)
	code = append(code, 0x61, 0, byte(len(data)), 0x60, 0, 0xa2, 0x00)
	if len(code) != 82 {
		t.Fatalf("event program length=%d", len(code))
	}
	code = append(code, data...)
	if err := client.Client().CallContext(ctx, nil, "anvil_setCode", emitter.Hex(), "0x"+hex.EncodeToString(code)); err != nil {
		t.Fatal(err)
	}
	var accounts []common.Address
	if err := client.Client().CallContext(ctx, &accounts, "eth_accounts"); err != nil || len(accounts) == 0 {
		t.Fatalf("accounts: %v %v", accounts, err)
	}
	var hash common.Hash
	if err := client.Client().CallContext(ctx, &hash, "eth_sendTransaction", map[string]string{"from": accounts[0].Hex(), "to": emitter.Hex(), "gas": "0xf4240"}); err != nil {
		t.Fatal(err)
	}
	var receipt *types.Receipt
	for i := 0; i < 100; i++ {
		receipt, err = client.TransactionReceipt(ctx, hash)
		if err == nil {
			break
		}
		time.Sleep(25 * time.Millisecond)
	}
	if err != nil || receipt == nil || receipt.Status != types.ReceiptStatusSuccessful || len(receipt.Logs) != 1 {
		t.Fatalf("event receipt: %+v %v", receipt, err)
	}
	log := *receipt.Logs[0]
	if log.Address != emitter || len(log.Topics) != 2 || log.Topics[0] != signature || log.Topics[1] != common.BytesToHash(pool.Bytes()) || !bytes.Equal(log.Data, data) {
		t.Fatalf("chain event: %+v", log)
	}
	return client, log, words[5]
}

func localAccountingDatabase(t *testing.T, log types.Log) (*pgxpool.Pool, *delivery.Store, *delivery.PartitionStore, *projectionstats.Repository, *projectionstats.AccountingSourceAuthority) {
	t.Helper()
	ctx := context.Background()
	pool := internaltest.NewPostgres(t)
	if _, err := pool.Exec(ctx, "CREATE SCHEMA indexer; CREATE SCHEMA stats_delivery; CREATE SCHEMA stats_projection"); err != nil {
		t.Fatal(err)
	}
	root, err := filepath.Abs("../../../../..")
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"60000", "190001"} {
		if name == "190001" {
			ddl, err := delivery.PartitionSchemaDDL("indexer")
			if err != nil {
				t.Fatal(err)
			}
			if _, err := pool.Exec(ctx, ddl); err != nil {
				t.Fatal(err)
			}
		}
		raw, err := os.ReadFile(filepath.Join(root, "launchpad/spec/production-system/integration/migrations", name+".sql"))
		if err != nil {
			t.Fatal(err)
		}
		if _, err := pool.Exec(ctx, string(raw)); err != nil {
			t.Fatal(err)
		}
	}
	ddl, err := delivery.SchemaDDL("stats_delivery")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, ddl); err != nil {
		t.Fatal(err)
	}
	ddl, err = delivery.PartitionSchemaDDL("stats_delivery")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, ddl); err != nil {
		t.Fatal(err)
	}
	ddl, err = delivery.ReplayDDL("stats_delivery")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, ddl); err != nil {
		t.Fatal(err)
	}
	repo, err := projectionstats.NewRepository(pool, "stats_projection")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, repo.MigrationSQL()); err != nil {
		t.Fatal(err)
	}
	streamDDL, err := os.ReadFile(filepath.Join(root, "launchpad/spec/production-system/integration/migrations/150002.sql"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, string(streamDDL)); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `CREATE VIEW stats.accounting_market_commits_v1 AS SELECT source_fill_id,source_fact_digest,projection_version,pool,fill,sequence FROM stats_projection.accounting_market_commits`); err != nil {
		t.Fatal(err)
	}
	policy := delivery.Policy{Subjects: map[string]bool{accounting.MarketSubject: true}, EventTypes: map[string]bool{"accounting.market.v1": true}}
	store, err := delivery.NewStore(pool, "stats_delivery", policy)
	if err != nil {
		t.Fatal(err)
	}
	partitions, err := delivery.NewPartitionStore(pool, "stats_delivery")
	if err != nil {
		t.Fatal(err)
	}
	authority, err := projectionstats.NewAccountingSourceAuthority("indexer")
	if err != nil {
		t.Fatal(err)
	}
	height := int64(log.BlockNumber)
	if height != 1 {
		t.Fatalf("first event block=%d", height)
	}
	topics := make([]string, len(log.Topics))
	for i, topic := range log.Topics {
		topics[i] = topic.Hex()
	}
	encoded, err := json.Marshal(topics)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO indexer.indexer_raw_logs(chain_id,transaction_hash,log_index,height,transaction_index,block_hash,emitter,topics,data) VALUES(125,$1,$2,$3,$4,$5,$6,$7,$8)`, strings.ToLower(log.TxHash.Hex()), log.Index, height, log.TxIndex, strings.ToLower(log.BlockHash.Hex()), strings.ToLower(log.Address.Hex()), encoded, log.Data); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO indexer.indexer_ingest_leases(chain_id,stream_id,owner_instance,fence_token,expires_at) VALUES(125,'canonical','indexer',1,clock_timestamp()+interval '1 hour')`); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO indexer.indexer_ingest_watermarks(chain_id,stream_id,raw_complete_height,typed_complete_height) VALUES(125,'canonical',1,1)`); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO indexer.indexer_block_completeness(chain_id,stream_id,height,block_hash,raw_count,fetch_complete,required_decode_complete) VALUES(125,'canonical',1,$1,1,true,true)`, strings.ToLower(log.BlockHash.Hex())); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO indexer.indexer_ingest_commits(chain_id,stream_id,batch_id,previous_height,last_complete_height,block_hash,record_count,outbox_count) VALUES(125,'canonical','first',0,1,$1,1,1)`, strings.ToLower(log.BlockHash.Hex())); err != nil {
		t.Fatal(err)
	}
	return pool, store, partitions, repo, authority
}

func localAccountingPublication(t *testing.T, pool *pgxpool.Pool, log types.Log, timestamp int64) accounting.PublicationV1 {
	t.Helper()
	ctx := context.Background()
	fill := accounting.FillV1{
		Version:       accounting.FillVersion,
		Position:      accounting.SourcePosition{ChainID: "125", Height: strconv.FormatUint(log.BlockNumber, 10), TransactionIndex: strconv.FormatUint(uint64(log.TxIndex), 10), LogIndex: strconv.FormatUint(uint64(log.Index), 10), TransactionHash: strings.ToLower(log.TxHash.Hex()), BlockHash: strings.ToLower(log.BlockHash.Hex())},
		ActualEmitter: strings.ToLower(log.Address.Hex()), PoolAddress: "0x0000000000000000000000000000000000000098",
		Token: accounting.Asset{Address: "0x0000000000000000000000000000000000000097", Decimals: 6}, Settlement: accounting.Asset{Address: "0x0000000000000000000000000000000000000096", Decimals: 6},
		Side: "buy", TokenAmountRaw: "2000", USDLAmountRaw: "1000", PaidInputRaw: "1000", FeeRaw: "3", FeeDisposition: "distributed_usdl", InputDerivationQuality: "receipt_incomplete",
		Attribution:           accounting.Attribution{TransactionSender: "0x0000000000000000000000000000000000000095", Quality: "unavailable", Evidence: []accounting.EvidenceRef{}},
		Execution:             accounting.ExecutionPolicy{Quality: "unavailable", Evidence: []accounting.EvidenceRef{}},
		Provenance:            accounting.Provenance{DeploymentID: "local_chain", ABIRevision: "swap-v1", EmitterCodeEpoch: "emitter-1", PoolCodeEpoch: "pool-1", ReceiptSHA256: strings.Repeat("a", 64), HeaderSHA256: strings.Repeat("b", 64)},
		BlockTimestampSeconds: strconv.FormatInt(timestamp, 10),
	}
	fill.SourceFillID = accounting.SourceFillIdentity(fill.Position, fill.Provenance.DeploymentID)
	digest, err := accounting.FillDigest(fill)
	if err != nil {
		t.Fatal(err)
	}
	bytes, err := json.Marshal(fill)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO indexer.indexer_accounting_fills(source_fill_id,source_fact_digest,chain_id,transaction_hash,log_index,pool_address,payload) VALUES($1,$2,125,$3,$4,$5,$6)`, fill.SourceFillID, string(digest), fill.Position.TransactionHash, log.Index, fill.PoolAddress, bytes); err != nil {
		t.Fatal(err)
	}
	key := "125:" + fill.PoolAddress
	effect := accounting.EffectIdentity(fill.SourceFillID, "market", "projection-v1", "", "market", key)
	producer, err := delivery.NewPartitionStore(pool, "indexer")
	if err != nil {
		t.Fatal(err)
	}
	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	position, err := producer.AppendTx(ctx, tx, delivery.AppendInput{ProjectionVersion: "projection-v1", PartitionKind: "market", PartitionKey: key, SourceEventID: fill.SourceFillID, EffectIdentity: effect, PayloadDigest: digest, SourceOrder: fill.Position})
	if err != nil {
		t.Fatal(err)
	}
	if err := tx.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	publication := accounting.PublicationV1{Version: accounting.PublicationVersion, Facet: "market", SourceFillID: fill.SourceFillID, SourceFactDigest: digest, Fill: &fill, Positions: []accounting.PartitionPosition{position}}
	publication.PublicationID = accounting.PublicationIdentity(fill.SourceFillID, digest, "market", "projection-v1", "initial", "")
	wire, err := accounting.CanonicalPublicationV1(publication)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO indexer.indexer_ingest_outbox(chain_id,transaction_hash,log_index,event_kind,payload) VALUES(125,$1,$2,'accounting.market.v1',$3)`, fill.Position.TransactionHash, log.Index, wire); err != nil {
		t.Fatal(err)
	}
	return publication
}

func TestSourceVerifiedAccountingFromLocalChainAndBroker(t *testing.T) {
	ctx := context.Background()
	chain, log, timestamp := localAccountingChain(t)
	pool, store, partitions, repo, authority := localAccountingDatabase(t, log)
	publication := localAccountingPublication(t, pool, log, timestamp)
	consumer, err := NewSourceVerifiedAccountingConsumer(SourceVerifiedAccountingConfig{Pool: pool, Deliveries: store, Partitions: partitions, Repository: repo, Authority: authority, ConsumerID: "stats", Generation: "g1", ProjectionVersion: "projection-v1", Owner: "stats-worker", SourceStream: "canonical", SourcePartitionSchema: "indexer", RPC: chain, MaxReplayBlocks: 16, MaxReplayFills: 16})
	if err != nil {
		t.Fatal(err)
	}
	brokerURL := os.Getenv("L20_NATS_URL")
	if brokerURL == "" {
		t.Fatal("L20_NATS_URL required")
	}
	nc := internaltest.ConnectNATS(t, brokerURL)
	js, err := jetstream.New(nc)
	if err != nil {
		t.Fatal(err)
	}
	stream := "STATSTEST_" + strings.ReplaceAll(uuid.NewString(), "-", "")
	if _, err := js.CreateStream(ctx, jetstream.StreamConfig{Name: stream, Subjects: []string{accounting.MarketSubject}, Storage: jetstream.FileStorage, MaxBytes: 1048576, MaxAge: time.Minute}); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		cleanup, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := js.DeleteStream(cleanup, stream); err != nil {
			t.Error(err)
		}
	})
	config := natsdriver.ConsumerConfig{Stream: stream, Durable: "stats", Subject: accounting.MarketSubject, Batch: 1, MaxAckPending: 1, MaxWaiting: 1, MaxDeliver: 2, FetchWait: time.Second, AckWait: time.Second}
	native, err := config.BrokerConsumerConfig()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := js.CreateOrUpdateConsumer(ctx, stream, native); err != nil {
		t.Fatal(err)
	}
	transport, err := natsdriver.NewConsumer(ctx, nc, config)
	if err != nil {
		t.Fatal(err)
	}
	defer transport.Close()
	publisher, err := natsdriver.NewPublisher(nc, stream)
	if err != nil {
		t.Fatal(err)
	}
	payload, err := accounting.CanonicalPublicationV1(publication)
	if err != nil {
		t.Fatal(err)
	}
	envelope := delivery.Envelope{EventID: publication.PublicationID, EventType: "accounting.market.v1", SchemaVersion: 1, ChainID: 125, SourceOrder: delivery.SourceOrder{Height: publication.Fill.Position.Height, TransactionIndex: publication.Fill.Position.TransactionIndex, LogIndex: publication.Fill.Position.LogIndex}, EmittedAt: time.Now().UTC(), TraceID: "stats_local", PayloadHash: delivery.Hash(payload), Payload: payload}
	wire, err := envelope.Encode(delivery.Policy{Subjects: map[string]bool{accounting.MarketSubject: true}, EventTypes: map[string]bool{"accounting.market.v1": true}}, accounting.MarketSubject)
	if err != nil {
		t.Fatal(err)
	}
	lease, err := consumer.lease(ctx, *publication.Fill)
	if err != nil {
		t.Fatal(err)
	}
	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	hooks := consumer.consumer.hooks(wire, lease)
	guarded, err := hooks.Guard(ctx, tx, envelope)
	if err != nil {
		_ = tx.Rollback(ctx)
		t.Fatal(err)
	}
	if err := hooks.Effect(ctx, tx, envelope, guarded); err != nil {
		_ = tx.Rollback(ctx)
		t.Fatal(err)
	}
	var provisional int
	if err := tx.QueryRow(ctx, `SELECT count(*) FROM stats_projection.trade_facts`).Scan(&provisional); err != nil || provisional != 1 {
		_ = tx.Rollback(ctx)
		t.Fatalf("provisional projection=%d %v", provisional, err)
	}
	if err := hooks.Advance(ctx, tx, envelope, guarded); err != nil {
		_ = tx.Rollback(ctx)
		t.Fatal(err)
	}
	if err := tx.Rollback(ctx); err != nil {
		t.Fatal(err)
	}
	if _, err := publisher.Publish(ctx, accounting.MarketSubject, publication.PublicationID, wire); err != nil {
		t.Fatal(err)
	}
	messages, err := transport.Fetch(ctx)
	if err != nil || len(messages) != 1 {
		t.Fatalf("broker messages: %d %v", len(messages), err)
	}
	if err := consumer.ConsumeBatch(ctx, messages); err != nil {
		t.Fatal(err)
	}
	if err := consumer.Consume(ctx, nil); err == nil {
		t.Fatal("missing broker message accepted")
	}
	bounded, stop := context.WithTimeout(ctx, 50*time.Millisecond)
	defer stop()
	if err := consumer.Run(bounded, transport); err == nil {
		t.Fatal("expired consumer continued fetching")
	}
	var trades, commits, offsets int
	if err := pool.QueryRow(ctx, `SELECT (SELECT count(*) FROM stats_projection.trade_facts),(SELECT count(*) FROM stats_projection.accounting_market_commits),(SELECT count(*) FROM stats_delivery.delivery_partition_commits)`).Scan(&trades, &commits, &offsets); err != nil || trades != 1 || commits != 1 || offsets != 1 {
		t.Fatalf("accounting rows=%d commits=%d offsets=%d err=%v", trades, commits, offsets, err)
	}
	if err := consumer.ConsumeBatch(ctx, nil); err == nil {
		t.Fatal("empty batch accepted")
	}
	if got, err := reportedBlockHash(ctx, chain, int64(log.BlockNumber)); err != nil || got != log.BlockHash {
		t.Fatalf("reported block=%s %v", got, err)
	}
	if h := sha256.Sum256(log.Data); h == ([32]byte{}) {
		t.Fatal("empty source log")
	}
}

func TestSourceVerifiedBootstrapRejectsChangedCanonicalEvidence(t *testing.T) {
	ctx := context.Background()
	chain, log, timestamp := localAccountingChain(t)
	for _, tc := range []struct {
		name, statement string
		want            error
	}{
		{"expired source lease", `UPDATE indexer.indexer_ingest_leases SET expires_at=clock_timestamp()-interval '1 second'`, ErrAccountingSourceUnavailable},
		{"incomplete source watermark", `UPDATE indexer.indexer_ingest_watermarks SET raw_complete_height=0`, ErrAccountingSourceUnavailable},
		{"missing market position", `DELETE FROM indexer.delivery_partition_positions`, ErrAccountingSourceUnavailable},
		{"incomplete source block", `UPDATE indexer.indexer_block_completeness SET required_decode_complete=false`, projectionstats.ErrIntegrity},
		{"incorrect commit count", `UPDATE indexer.indexer_ingest_commits SET record_count=2`, projectionstats.ErrIntegrity},
	} {
		t.Run(tc.name, func(t *testing.T) {
			pool, deliveries, partitions, repo, authority := localAccountingDatabase(t, log)
			publication := localAccountingPublication(t, pool, log, timestamp)
			if _, err := pool.Exec(ctx, tc.statement); err != nil {
				t.Fatal(err)
			}
			consumer, err := NewSourceVerifiedAccountingConsumer(SourceVerifiedAccountingConfig{Pool: pool, Deliveries: deliveries, Partitions: partitions, Repository: repo, Authority: authority, ConsumerID: "stats", Generation: "g1", ProjectionVersion: "projection-v1", Owner: "stats-worker", SourceStream: "canonical", SourcePartitionSchema: "indexer", RPC: chain, MaxReplayBlocks: 16, MaxReplayFills: 16})
			if err != nil {
				t.Fatal(err)
			}
			if _, err := consumer.lease(ctx, *publication.Fill); !errors.Is(err, tc.want) {
				t.Fatalf("source bootstrap: %v, want %v", err, tc.want)
			}
		})
	}
}
