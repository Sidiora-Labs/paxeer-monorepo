package consumer

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"math/big"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/Sidiora-Technologies/KindleLaunch/core/trading-charts/internal/engine"
	"github.com/Sidiora-Technologies/KindleLaunch/core/trading-charts/internal/internaltest"
	"github.com/Sidiora-Technologies/KindleLaunch/core/trading-charts/internal/store"
	"github.com/Sidiora-Technologies/KindleLaunch/shared/accounting"
	"github.com/Sidiora-Technologies/KindleLaunch/shared/accounting/sourcebootstrap"
	"github.com/Sidiora-Technologies/KindleLaunch/shared/delivery"
	"github.com/Sidiora-Technologies/KindleLaunch/shared/delivery/natsdriver"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/ethereum/go-ethereum/rpc"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type longSpanNode struct {
	mu                                            sync.Mutex
	creation                                      *types.Receipt
	fill                                          accounting.FillV1
	pruned                                        bool
	chain                                         int64
	calls, filters, oldAnswered, creationAnswered int
}

func (n *longSpanNode) wait(ctx context.Context, height int64) error {
	n.mu.Lock()
	n.calls++
	n.mu.Unlock()
	timer := time.NewTimer(150 * time.Millisecond)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
	}
	head, _ := strconv.ParseInt(n.fill.Position.Height, 10, 64)
	earliest := head + 100000
	if height >= 0 && n.pruned && height < earliest {
		return fmt.Errorf("requested height %d is pruned; earliest height is %d", height, earliest)
	}
	n.mu.Lock()
	defer n.mu.Unlock()
	if height >= 0 && height < earliest {
		n.oldAnswered++
	}
	if height == n.creation.BlockNumber.Int64() {
		n.creationAnswered++
	}
	return nil
}
func (n *longSpanNode) ChainId(ctx context.Context) (*hexutil.Big, error) {
	if err := n.wait(ctx, -1); err != nil {
		return nil, err
	}
	return (*hexutil.Big)(big.NewInt(n.chain)), nil
}
func (n *longSpanNode) GetBlockByNumber(ctx context.Context, height string, _ bool) (map[string]any, error) {
	h, err := hexutil.DecodeUint64(height)
	if err != nil {
		return nil, err
	}
	if err = n.wait(ctx, int64(h)); err != nil {
		return nil, err
	}
	hash := longSpanHash(int64(h))
	timestamp, _ := strconv.ParseInt(n.fill.BlockTimestampSeconds, 10, 64)
	return map[string]any{"number": height, "hash": hash, "timestamp": hexutil.EncodeBig(big.NewInt(timestamp))}, nil
}
func (n *longSpanNode) GetTransactionReceipt(ctx context.Context, hash common.Hash) (*types.Receipt, error) {
	if err := n.wait(ctx, n.creation.BlockNumber.Int64()); err != nil {
		return nil, err
	}
	if hash != n.creation.TxHash {
		return nil, nil
	}
	return n.creation, nil
}
func (n *longSpanNode) GetLogs(ctx context.Context, _ map[string]any) ([]types.Log, error) {
	n.mu.Lock()
	n.filters++
	n.mu.Unlock()
	return nil, errors.New("indexed bootstrap does not require a node log scan")
}
func (n *longSpanNode) client(t *testing.T) *ethclient.Client {
	t.Helper()
	server := rpc.NewServer()
	if err := server.RegisterName("eth", n); err != nil {
		t.Fatal(err)
	}
	httpServer := httptest.NewServer(server)
	t.Cleanup(httpServer.Close)
	client, err := ethclient.Dial(httpServer.URL)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(client.Close)
	return client
}
func longSpanHash(height int64) string { return fmt.Sprintf("0x%064x", height+1) }

type longSpanQueryKey struct{}
type longSpanQueries struct {
	mu       sync.Mutex
	delay    time.Duration
	finished map[int64]int
	acquired func()
}

func (q *longSpanQueries) TraceQueryStart(ctx context.Context, _ *pgx.Conn, data pgx.TraceQueryStartData) context.Context {
	if strings.Contains(data.SQL, "delivery_partition_leases") && strings.HasPrefix(data.SQL, "INSERT INTO") {
		q.mu.Lock()
		action := q.acquired
		q.acquired = nil
		q.mu.Unlock()
		if action != nil {
			action()
		}
	}
	if data.SQL != bootstrapSliceSQL {
		return ctx
	}
	from := data.Args[1].(int64)
	if q.delay > 0 {
		timer := time.NewTimer(q.delay)
		select {
		case <-ctx.Done():
		case <-timer.C:
		}
		timer.Stop()
	}
	return context.WithValue(ctx, longSpanQueryKey{}, from)
}
func (q *longSpanQueries) TraceQueryEnd(ctx context.Context, _ *pgx.Conn, data pgx.TraceQueryEndData) {
	if from, ok := ctx.Value(longSpanQueryKey{}).(int64); ok && data.Err == nil {
		q.mu.Lock()
		q.finished[from]++
		q.mu.Unlock()
	}
}

type longSpanFixture struct {
	ctx                         context.Context
	pool, source                *pgxpool.Pool
	config                      SourceVerifiedAccountingConfig
	fill                        accounting.FillV1
	primary, archive            *longSpanNode
	queries                     *longSpanQueries
	logs                        bytes.Buffer
	message                     *natsdriver.Message
	creationHeight, firstHeight int64
}

func newLongSpanFixture(t *testing.T, span int64) *longSpanFixture {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	t.Cleanup(cancel)
	source := internaltest.OpenPool(t, internaltest.NewDatabase(t))
	productionMigrations(t, ctx, source)
	queries := &longSpanQueries{finished: map[int64]int{}}
	poolConfig := source.Config()
	poolConfig.ConnConfig.Tracer = queries
	pool, err := pgxpool.NewWithConfig(ctx, poolConfig)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	f := &longSpanFixture{ctx: ctx, pool: pool, source: source, queries: queries, creationHeight: 100, firstHeight: 100 + span}
	fill := accountingFixture()
	fill.Position.Height = strconv.FormatInt(f.firstHeight, 10)
	fill.Position.BlockHash = longSpanHash(f.firstHeight)
	fill.Position.TransactionIndex = "0"
	fill.Position.LogIndex = "0"
	fill.BlockTimestampSeconds = "1700000000"
	fill.SourceFillID = accounting.SourceFillIdentity(fill.Position, fill.Provenance.DeploymentID)
	f.fill = fill
	created := &types.Log{Address: common.HexToAddress("0x" + strings.Repeat("a", 40)), Topics: []common.Hash{marketCreatedTopic, common.BytesToHash(common.HexToAddress(fill.PoolAddress).Bytes())}, Data: []byte{}, BlockNumber: uint64(f.creationHeight), BlockHash: common.HexToHash(longSpanHash(f.creationHeight)), TxHash: common.HexToHash(longSpanHash(1)), TxIndex: 0, Index: 0}
	receipt := &types.Receipt{Type: types.DynamicFeeTxType, Status: types.ReceiptStatusSuccessful, CumulativeGasUsed: 21000, Logs: []*types.Log{created}, TxHash: created.TxHash, GasUsed: 21000, BlockHash: created.BlockHash, BlockNumber: big.NewInt(f.creationHeight), TransactionIndex: 0}
	f.primary = &longSpanNode{creation: receipt, fill: fill, pruned: true, chain: 125}
	f.archive = &longSpanNode{creation: receipt, fill: fill, chain: 125}
	topics, _ := json.Marshal([]string{created.Topics[0].Hex(), created.Topics[1].Hex()})
	f.exec(t, `INSERT INTO indexer.indexer_raw_logs(chain_id,transaction_hash,log_index,height,transaction_index,block_hash,emitter,topics,data) VALUES(125,$1,0,$2,0,$3,$4,$5,$6)`, created.TxHash.Hex(), f.creationHeight, created.BlockHash.Hex(), strings.ToLower(created.Address.Hex()), topics, created.Data)
	data := []byte{}
	for _, value := range []string{"1", fill.USDLAmountRaw, fill.TokenAmountRaw, fill.FeeRaw, "1", fill.BlockTimestampSeconds, fill.Position.Height} {
		n, ok := new(big.Int).SetString(value, 10)
		if !ok {
			t.Fatal(value)
		}
		data = append(data, common.LeftPadBytes(n.Bytes(), 32)...)
	}
	swapTopics, _ := json.Marshal([]string{swapEventTopic.Hex(), common.BytesToHash(common.HexToAddress(fill.PoolAddress).Bytes()).Hex()})
	f.exec(t, `INSERT INTO indexer.indexer_raw_logs(chain_id,transaction_hash,log_index,height,transaction_index,block_hash,emitter,topics,data) VALUES(125,$1,0,$2,0,$3,$4,$5,$6)`, fill.Position.TransactionHash, f.firstHeight, fill.Position.BlockHash, fill.ActualEmitter, swapTopics, data)
	f.exec(t, `INSERT INTO indexer.indexer_block_completeness(chain_id,stream_id,height,block_hash,raw_count,fetch_complete,required_decode_complete) SELECT 125,'canonical',h,'0x'||lpad(to_hex(h+1),64,'0'),CASE WHEN h IN ($1,$2) THEN 1 ELSE 0 END,true,true FROM generate_series($1::bigint,$2::bigint) h`, f.creationHeight, f.firstHeight)
	f.exec(t, `INSERT INTO indexer.indexer_ingest_commits(chain_id,stream_id,batch_id,previous_height,last_complete_height,block_hash,record_count,outbox_count) SELECT 125,'canonical','span_'||h::text,h-1,least(h+6999,$2),'0x'||lpad(to_hex(least(h+6999,$2)+1),64,'0'),(CASE WHEN h=$1 THEN 1 ELSE 0 END)+(CASE WHEN h+6999>=$2 THEN 1 ELSE 0 END),0 FROM generate_series($1::bigint,$2::bigint,7000) h`, f.creationHeight, f.firstHeight)
	f.exec(t, `INSERT INTO indexer.indexer_ingest_watermarks VALUES(125,'canonical',$1,$1)`, f.firstHeight)
	f.exec(t, `INSERT INTO indexer.indexer_ingest_leases(chain_id,stream_id,owner_instance,fence_token,expires_at) VALUES(125,'canonical','long-span',1,clock_timestamp()+interval '1 hour')`)
	f.exec(t, `INSERT INTO indexer.indexer_canonical_creations(token,pool,creator,factory,optical,nft_id,height,block_hash,transaction_hash,transaction_index,log_index,block_timestamp,provenance,payload) VALUES($1,$2,$3,$4,$1,'1',$5,$6,$7,0,0,1700000000,'{}','{}')`, fill.Token.Address, fill.PoolAddress, fill.Attribution.TransactionSender, strings.ToLower(created.Address.Hex()), f.creationHeight, created.BlockHash.Hex(), created.TxHash.Hex())
	digest, err := accounting.FillDigest(fill)
	if err != nil {
		t.Fatal(err)
	}
	body, err := json.Marshal(fill)
	if err != nil {
		t.Fatal(err)
	}
	f.exec(t, `INSERT INTO indexer.indexer_accounting_fills VALUES($1,$2,125,$3,0,$4,$5)`, fill.SourceFillID, string(digest), fill.Position.TransactionHash, fill.PoolAddress, body)
	producer, err := delivery.NewPartitionStore(pool, "indexer")
	if err != nil {
		t.Fatal(err)
	}
	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	position, err := producer.AppendTx(ctx, tx, delivery.AppendInput{ProjectionVersion: "market-v1", PartitionKind: "market", PartitionKey: "125:" + fill.PoolAddress, SourceEventID: fill.SourceFillID, EffectIdentity: accounting.EffectIdentity(fill.SourceFillID, "market", "market-v1", "", "market", "125:"+fill.PoolAddress), PayloadDigest: digest, SourceOrder: fill.Position})
	if err != nil {
		_ = tx.Rollback(ctx)
		t.Fatal(err)
	}
	if err = tx.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	publication, err := accounting.NewMarketPublication(fill, []accounting.PartitionPosition{position})
	if err != nil {
		t.Fatal(err)
	}
	payload, err := accounting.CanonicalPublicationV1(publication)
	if err != nil {
		t.Fatal(err)
	}
	f.exec(t, `INSERT INTO indexer.indexer_ingest_outbox(chain_id,transaction_hash,log_index,event_kind,payload) VALUES(125,$1,0,'accounting.market.v1',$2)`, fill.Position.TransactionHash, payload)
	policy := delivery.Policy{Subjects: map[string]bool{accounting.MarketSubject: true}, EventTypes: map[string]bool{accounting.MarketSubject: true}}
	envelope := delivery.Envelope{EventID: publication.PublicationID, EventType: accounting.MarketSubject, SchemaVersion: 1, ChainID: 125, SourceOrder: delivery.SourceOrder{Height: fill.Position.Height, TransactionIndex: "0", LogIndex: "0"}, EmittedAt: time.Now().UTC(), TraceID: "long-span", PayloadHash: delivery.Hash(payload), Payload: payload}
	wire, err := envelope.Encode(policy, accounting.MarketSubject)
	if err != nil {
		t.Fatal(err)
	}
	deliveries, err := delivery.NewStore(pool, "candles", policy)
	if err != nil {
		t.Fatal(err)
	}
	partitions, err := delivery.NewPartitionStore(pool, "candles")
	if err != nil {
		t.Fatal(err)
	}
	exact, err := store.NewExact(pool, "candles")
	if err != nil {
		t.Fatal(err)
	}
	f.config = SourceVerifiedAccountingConfig{Pool: pool, Deliveries: deliveries, Partitions: partitions, Exact: exact, ConsumerID: "charts", Generation: "long_span", Owner: "long-span", SourceStream: "canonical", SourcePartitionSchema: "indexer", ProjectionVersion: "market-v1", RPC: f.primary.client(t), RPCFallback: f.archive.client(t), MaxReplayBlocks: 4096, MaxReplayFills: 10000, ProofTimeout: 15 * time.Second, Logger: slog.New(slog.NewTextHandler(&f.logs, nil))}
	nc, stream := internaltest.NewBroker(t, "CHART_LONG_SPAN", []string{accounting.MarketSubject})
	brokerConfig := natsdriver.ConsumerConfig{Stream: "CHART_LONG_SPAN", Durable: "long_span", Subject: accounting.MarketSubject, Batch: 1, MaxAckPending: 16, MaxWaiting: 1, MaxDeliver: 20, FetchWait: 100 * time.Millisecond, AckWait: 30 * time.Second}
	admitted, err := brokerConfig.BrokerConsumerConfig()
	if err != nil {
		t.Fatal(err)
	}
	if _, err = stream.CreateConsumer(ctx, admitted); err != nil {
		t.Fatal(err)
	}
	publisher, err := natsdriver.NewPublisher(nc, brokerConfig.Stream)
	if err != nil {
		t.Fatal(err)
	}
	reader, err := natsdriver.NewConsumer(ctx, nc, brokerConfig)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(reader.Close)
	if _, err = publisher.Publish(ctx, accounting.MarketSubject, publication.PublicationID, wire); err != nil {
		t.Fatal(err)
	}
	messages := faultFetch(t, ctx, reader)
	if len(messages) != 1 {
		t.Fatalf("market deliveries: %d", len(messages))
	}
	f.message = messages[0]
	return f
}
func (f *longSpanFixture) exec(t *testing.T, sql string, args ...any) {
	t.Helper()
	if _, err := f.source.Exec(f.ctx, sql, args...); err != nil {
		t.Fatal(err)
	}
}
func (f *longSpanFixture) consumer(t *testing.T) *AccountingConsumer {
	t.Helper()
	c, err := NewSourceVerifiedAccountingConsumer(f.config)
	if err != nil {
		t.Fatal(err)
	}
	return c
}
func (f *longSpanFixture) empty(t *testing.T) {
	t.Helper()
	var count int
	if err := f.pool.QueryRow(f.ctx, `SELECT (SELECT count(*) FROM candles.delivery_partition_offsets)+(SELECT count(*) FROM candles.chart_inbox)+(SELECT count(*) FROM candles.chart_exact_v1)`).Scan(&count); err != nil || count != 0 {
		t.Fatalf("unproved bootstrap wrote rows: %d %v", count, err)
	}
}
func (f *longSpanFixture) committed(t *testing.T) {
	t.Helper()
	var sequence int64
	var digest string
	if err := f.pool.QueryRow(f.ctx, `SELECT next_sequence,checkpoint_digest FROM candles.delivery_partition_offsets WHERE partition_key=$1`, "125:"+f.fill.PoolAddress).Scan(&sequence, &digest); err != nil || sequence != 2 {
		t.Fatalf("bootstrap offset: %d %v", sequence, err)
	}
	evidence, err := sourcebootstrap.LoadCharts(f.ctx, f.pool, "indexer", "canonical", "market-v1", f.fill.PoolAddress)
	if err != nil || digest != string(evidence.Checkpoint()) {
		t.Fatalf("charts checkpoint: %s %v", digest, err)
	}
	var wire []byte
	if err := f.pool.QueryRow(f.ctx, `SELECT snapshot FROM candles.chart_exact_v1 WHERE pool=$1 AND interval='1m'`, f.fill.PoolAddress).Scan(&wire); err != nil {
		t.Fatal(err)
	}
	var candle engine.QualifiedCandle
	if err := json.Unmarshal(wire, &candle); err != nil || candle.Trades != 1 || candle.VolumeUsdlRaw != f.fill.USDLAmountRaw {
		t.Fatalf("first fill candle: %+v %v", candle, err)
	}
}
func (f *longSpanFixture) finish(t *testing.T, c *AccountingConsumer) {
	t.Helper()
	for i := 0; i < 20; i++ {
		err := c.Consume(f.ctx, f.message)
		if err == nil {
			f.committed(t)
			return
		}
		if !errors.Is(err, ErrAccountingSourceUnavailable) && !errors.Is(err, context.DeadlineExceeded) {
			t.Fatal(err)
		}
	}
	t.Fatal("bounded deliveries did not complete the indexed proof")
}

func TestBootstrapLongSpanIndexedRecords(t *testing.T) {
	calls := []int{}
	for _, span := range []int64{10000, 1200000} {
		t.Run(strconv.FormatInt(span, 10), func(t *testing.T) {
			f := newLongSpanFixture(t, span)
			monitor, stop := context.WithCancel(f.ctx)
			done := make(chan error, 1)
			go func() {
				ticker := time.NewTicker(25 * time.Millisecond)
				defer ticker.Stop()
				for {
					var old bool
					err := f.source.QueryRow(monitor, `SELECT EXISTS(SELECT 1 FROM pg_stat_activity WHERE datname=current_database() AND xact_start<clock_timestamp()-interval '5 seconds')`).Scan(&old)
					if err != nil && monitor.Err() == nil {
						done <- err
						return
					}
					if old {
						done <- errors.New("database transaction exceeds five seconds during bootstrap")
						return
					}
					select {
					case <-monitor.Done():
						done <- nil
						return
					case <-ticker.C:
					}
				}
			}()
			f.finish(t, f.consumer(t))
			stop()
			if err := <-done; err != nil {
				t.Fatal(err)
			}
			if f.primary.oldAnswered != 0 || f.archive.creationAnswered != 2 || f.primary.filters != 0 || f.archive.filters != 0 {
				t.Fatalf("historical reads: primary=%+v archive=%+v", f.primary, f.archive)
			}
			calls = append(calls, f.primary.calls+f.archive.calls)
		})
	}
	if len(calls) != 2 || calls[0] != calls[1] || calls[0] != 10 {
		t.Fatalf("node calls by span: %v", calls)
	}
}

func TestBootstrapLongSpanShortLease(t *testing.T) {
	f := newLongSpanFixture(t, 10000)
	f.config.LeaseDuration = 2 * time.Second
	f.config.MaxReplayBlocks = 1024
	f.queries.delay = 100 * time.Millisecond
	start := time.Now()
	f.finish(t, f.consumer(t))
	if time.Since(start) <= f.config.LeaseDuration {
		t.Fatal("proof did not outlast the lease duration")
	}
}

func TestBootstrapLongSpanResumesFinishedSlices(t *testing.T) {
	f := newLongSpanFixture(t, 10000)
	f.config.MaxReplayBlocks = 512
	f.config.ProofTimeout = 2 * time.Second
	f.queries.delay = 100 * time.Millisecond
	c := f.consumer(t)
	if err := c.Consume(f.ctx, f.message); !errors.Is(err, ErrAccountingSourceUnavailable) && !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("bounded proof: %v", err)
	}
	f.empty(t)
	if len(f.queries.finished) == 0 {
		t.Fatal("first delivery completed no slice")
	}
	f.finish(t, c)
	for from, count := range f.queries.finished {
		if count != 1 {
			t.Fatalf("finished slice %d read %d times", from, count)
		}
	}
	if f.primary.calls+f.archive.calls != 10 {
		t.Fatal("resumption repeated canonical node reads")
	}
}

func TestBootstrapLongSpanRejectsEarlierSwap(t *testing.T) {
	f := newLongSpanFixture(t, 10000)
	f.exec(t, `INSERT INTO indexer.indexer_raw_logs SELECT chain_id,$1,log_index,$2,transaction_index,$3,emitter,topics,data FROM indexer.indexer_raw_logs WHERE transaction_hash=$4`, longSpanHash(2), f.creationHeight+1, longSpanHash(f.creationHeight+1), f.fill.Position.TransactionHash)
	f.exec(t, `UPDATE indexer.indexer_block_completeness SET raw_count=1 WHERE height=$1`, f.creationHeight+1)
	f.exec(t, `UPDATE indexer.indexer_ingest_commits SET record_count=record_count+1 WHERE previous_height<$1 AND last_complete_height>=$1`, f.creationHeight+1)
	if err := f.consumer(t).Consume(f.ctx, f.message); !errors.Is(err, ErrAccountingSourceUnavailable) {
		t.Fatalf("earlier indexed swap: %v", err)
	}
	f.empty(t)
}

func TestBootstrapLongSpanRejectsMissingCompleteness(t *testing.T) {
	f := newLongSpanFixture(t, 10000)
	f.exec(t, `DELETE FROM indexer.indexer_block_completeness WHERE height=$1`, f.creationHeight+1)
	if err := f.consumer(t).Consume(f.ctx, f.message); !errors.Is(err, ErrAccountingSourceUnavailable) {
		t.Fatalf("missing completeness: %v", err)
	}
	f.empty(t)
}

func TestBootstrapLongSpanReprovesChangedEvidence(t *testing.T) {
	for _, change := range []string{"checkpoint", "commit_count", "swap_count"} {
		t.Run(change, func(t *testing.T) {
			f := newLongSpanFixture(t, 10000)
			f.queries.acquired = func() {
				switch change {
				case "checkpoint":
					f.exec(t, `UPDATE indexer.indexer_ingest_commits SET batch_id='first_fill' WHERE last_complete_height=$1`, f.firstHeight)
				case "commit_count":
					f.exec(t, `INSERT INTO indexer.indexer_ingest_commits(chain_id,stream_id,batch_id,previous_height,last_complete_height,block_hash,record_count,outbox_count) VALUES(125,'canonical','overlap',$1::bigint-1,$1::bigint,'0x'||lpad(to_hex($1::bigint+1),64,'0'),0,0)`, f.creationHeight+1)
				case "swap_count":
					f.exec(t, `INSERT INTO indexer.indexer_raw_logs SELECT chain_id,$1,log_index,$2,transaction_index,$3,emitter,topics,data FROM indexer.indexer_raw_logs WHERE transaction_hash=$4`, longSpanHash(3), f.creationHeight+1, longSpanHash(f.creationHeight+1), f.fill.Position.TransactionHash)
				}
			}
			c := f.consumer(t)
			if err := c.Consume(f.ctx, f.message); !errors.Is(err, ErrAccountingSourceUnavailable) {
				t.Fatalf("changed indexed evidence: %v", err)
			}
			f.empty(t)
			if change == "commit_count" {
				f.exec(t, `DELETE FROM indexer.indexer_ingest_commits WHERE batch_id='overlap'`)
			}
			if change == "swap_count" {
				f.exec(t, `DELETE FROM indexer.indexer_raw_logs WHERE transaction_hash=$1`, longSpanHash(3))
			}
			f.finish(t, c)
			if f.primary.calls+f.archive.calls != 20 {
				t.Fatal("changed evidence retained the previous proof")
			}
		})
	}
}

func TestBootstrapLongSpanBeyond4096Windows(t *testing.T) {
	f := newLongSpanFixture(t, 4100)
	f.config.MaxReplayBlocks = 1
	f.finish(t, f.consumer(t))
	if len(f.queries.finished) <= 4096 {
		t.Fatal("proof did not exceed 4096 indexed slices")
	}
}

func TestBootstrapLongSpanRequiresHistoricalNode(t *testing.T) {
	f := newLongSpanFixture(t, 1200000)
	f.config.RPCFallback = nil
	if err := f.consumer(t).Consume(f.ctx, f.message); !errors.Is(err, ErrAccountingSourceUnavailable) {
		t.Fatalf("pruned source: %v", err)
	}
	f.empty(t)
	if !strings.Contains(f.logs.String(), "height=100") || !strings.Contains(f.logs.String(), "a node with the full history is required") {
		t.Fatalf("historical node diagnostic: %s", f.logs.String())
	}
}

func TestBootstrapLongSpanChecksFallbackChain(t *testing.T) {
	f := newLongSpanFixture(t, 10000)
	f.archive.chain = 1
	if err := f.consumer(t).Consume(f.ctx, f.message); !errors.Is(err, ErrAccountingAuthority) {
		t.Fatalf("fallback chain identity: %v", err)
	}
	f.empty(t)
}

func TestBootstrapLongSpanRestartsProof(t *testing.T) {
	f := newLongSpanFixture(t, 10000)
	f.finish(t, f.consumer(t))
	if _, err := f.consumer(t).config.Lease(f.ctx, f.fill); err != nil {
		t.Fatal(err)
	}
	f.committed(t)
	if f.primary.calls+f.archive.calls != 20 {
		t.Fatal("process restart did not prove the source again")
	}
}
