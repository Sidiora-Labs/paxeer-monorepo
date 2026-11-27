package consumer

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"math/big"
	"net/http"
	"net/http/httptest"
	"os"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/Sidiora-Technologies/KindleLaunch/core/stats-workers/internal/internaltest"
	"github.com/Sidiora-Technologies/KindleLaunch/core/stats-workers/internal/projectionstats"
	"github.com/Sidiora-Technologies/KindleLaunch/shared/accounting"
	"github.com/Sidiora-Technologies/KindleLaunch/shared/delivery"
	"github.com/Sidiora-Technologies/KindleLaunch/shared/delivery/natsdriver"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/nats-io/nats.go/jetstream"
)

type statsHistoryNode struct {
	mu                                              sync.Mutex
	calls                                           map[string]int
	stateReads, logReads, pruned, historicalAnswers int
	pruning                                         bool
	creation                                        *types.Receipt
	fill                                            types.Log
	beforeFillBlock                                 func()
}

func (n *statsHistoryNode) serve(w http.ResponseWriter, r *http.Request) {
	var request struct {
		ID     json.RawMessage   `json:"id"`
		Method string            `json:"method"`
		Params []json.RawMessage `json:"params"`
	}
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	time.Sleep(150 * time.Millisecond)
	n.mu.Lock()
	defer n.mu.Unlock()
	n.calls[request.Method]++
	var result any
	var rpcError any
	height := int64(n.fill.BlockNumber)
	switch request.Method {
	case "eth_chainId":
		result = "0x7d"
	case "eth_getBlockByNumber":
		var number string
		if len(request.Params) > 0 {
			_ = json.Unmarshal(request.Params[0], &number)
		}
		parsed, err := strconv.ParseInt(strings.TrimPrefix(number, "0x"), 16, 64)
		if err != nil {
			rpcError = map[string]any{"code": -32602, "message": "block number required"}
			break
		}
		height = parsed
		if n.pruning && height < int64(n.fill.BlockNumber)-100000 {
			n.pruned++
			rpcError = map[string]any{"code": -32000, "message": fmt.Sprintf("height %d pruned; earliest height %d", height, int64(n.fill.BlockNumber)-100000)}
			break
		}
		hash := n.fill.BlockHash
		if height == n.creation.BlockNumber.Int64() {
			hash = n.creation.BlockHash
			n.historicalAnswers++
		}
		if height == int64(n.fill.BlockNumber) && n.beforeFillBlock != nil {
			n.beforeFillBlock()
			n.beforeFillBlock = nil
		}
		result = map[string]any{"number": fmt.Sprintf("0x%x", height), "hash": hash.Hex()}
	case "eth_getTransactionReceipt":
		height = n.creation.BlockNumber.Int64()
		if n.pruning && height < int64(n.fill.BlockNumber)-100000 {
			n.pruned++
			rpcError = map[string]any{"code": -32000, "message": fmt.Sprintf("height %d pruned; earliest height %d", height, int64(n.fill.BlockNumber)-100000)}
			break
		}
		n.historicalAnswers++
		result = n.creation
	case "eth_getLogs":
		n.logReads++
		rpcError = map[string]any{"code": -32000, "message": "log filtering is unavailable"}
	default:
		n.stateReads++
		rpcError = map[string]any{"code": -32000, "message": "historical state is unavailable"}
	}
	w.Header().Set("Content-Type", "application/json")
	response := map[string]any{"jsonrpc": "2.0", "id": request.ID}
	if rpcError != nil {
		response["error"] = rpcError
	} else {
		response["result"] = result
	}
	_ = json.NewEncoder(w).Encode(response)
}

func (n *statsHistoryNode) client(t *testing.T) *ethclient.Client {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(n.serve))
	t.Cleanup(server.Close)
	client, err := ethclient.Dial(server.URL)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(client.Close)
	return client
}

type statsLongSpanFixture struct {
	pool             *pgxpool.Pool
	config           SourceVerifiedAccountingConfig
	publication      accounting.PublicationV1
	primary, history *statsHistoryNode
	log              bytes.Buffer
}

func newStatsLongSpanFixture(t *testing.T, original types.Log, receipt *types.Receipt, timestamp int64, span int64) *statsLongSpanFixture {
	t.Helper()
	const head = int64(2000000)
	creationHeight := head - span
	f := &statsLongSpanFixture{}
	pool, deliveries, partitions, repo, authority := localAccountingDatabase(t, original)
	f.pool = pool
	exec := func(sql string, args ...any) {
		t.Helper()
		if _, err := pool.Exec(t.Context(), sql, args...); err != nil {
			t.Fatal(err)
		}
	}
	creationHash := common.HexToHash("0x" + strings.Repeat("c", 64))
	creationTx := common.HexToHash("0x" + strings.Repeat("d", 64))
	created := *receipt.Logs[0]
	created.BlockNumber, created.BlockHash, created.TxHash = uint64(creationHeight), creationHash, creationTx
	creationReceipt := *receipt
	creationReceipt.BlockNumber, creationReceipt.BlockHash, creationReceipt.TxHash = big.NewInt(creationHeight), creationHash, creationTx
	creationReceipt.Logs = []*types.Log{&created}
	fill := original
	fill.BlockNumber = uint64(head)
	exec(`DELETE FROM indexer.indexer_ingest_commits; DELETE FROM indexer.indexer_block_completeness`)
	exec(`UPDATE indexer.indexer_raw_logs SET height=$1,block_hash=$2,transaction_hash=$3 WHERE log_index=0`, creationHeight, creationHash.Hex(), creationTx.Hex())
	exec(`UPDATE indexer.indexer_canonical_creations SET height=$1,block_hash=$2,transaction_hash=$3`, creationHeight, creationHash.Hex(), creationTx.Hex())
	exec(`UPDATE indexer.indexer_raw_logs SET height=$1 WHERE log_index=$2`, head, original.Index)
	exec(`UPDATE indexer.indexer_ingest_watermarks SET raw_complete_height=$1,typed_complete_height=$1`, head)
	exec(`INSERT INTO indexer.indexer_block_completeness(chain_id,stream_id,height,block_hash,raw_count,fetch_complete,required_decode_complete)
 SELECT 125,'canonical',h,CASE WHEN h=$1 THEN $2 ELSE $3 END,CASE WHEN h=$1 OR h=$4 THEN 1 ELSE 0 END,true,true FROM generate_series(1,$4::bigint) h`, creationHeight, creationHash.Hex(), fill.BlockHash.Hex(), head)
	exec(`INSERT INTO indexer.indexer_ingest_commits(chain_id,stream_id,batch_id,previous_height,last_complete_height,block_hash,record_count,outbox_count)
 VALUES(125,'canonical','history',0,$1,$2,0,0),(125,'canonical','creation',$1,$3,$4,1,0),(125,'canonical','first',$3,$5,$2,1,1)`, creationHeight-1, fill.BlockHash.Hex(), creationHeight, creationHash.Hex(), head)
	f.publication = localAccountingPublication(t, pool, fill, timestamp)
	f.primary = &statsHistoryNode{calls: map[string]int{}, pruning: true, creation: &creationReceipt, fill: fill}
	f.history = &statsHistoryNode{calls: map[string]int{}, creation: &creationReceipt, fill: fill}
	f.config = SourceVerifiedAccountingConfig{Pool: pool, Deliveries: deliveries, Partitions: partitions, Repository: repo, Authority: authority, ConsumerID: "stats", Generation: "g1", ProjectionVersion: "projection-v1", Owner: "stats-worker", SourceStream: "canonical", SourcePartitionSchema: "indexer", RPC: f.primary.client(t), RPCFallback: f.history.client(t), Logger: slog.New(slog.NewTextHandler(&f.log, nil)), LeaseDuration: 2 * time.Second, ProofTimeout: 5 * time.Second, MaxReplayBlocks: 4096, MaxReplayFills: 16}
	return f
}

func (f *statsLongSpanFixture) consumer(t *testing.T) *SourceVerifiedAccountingConsumer {
	t.Helper()
	consumer, err := NewSourceVerifiedAccountingConsumer(f.config)
	if err != nil {
		t.Fatal(err)
	}
	return consumer
}

func (f *statsLongSpanFixture) message(t *testing.T) *natsdriver.Message {
	t.Helper()
	transport, publisher := accountingBatchBroker(t)
	payload, err := accounting.CanonicalPublicationV1(f.publication)
	if err != nil {
		t.Fatal(err)
	}
	fill := f.publication.Fill
	envelope := delivery.Envelope{EventID: f.publication.PublicationID, EventType: "accounting.market.v1", SchemaVersion: 1, ChainID: 125, SourceOrder: delivery.SourceOrder{Height: fill.Position.Height, TransactionIndex: fill.Position.TransactionIndex, LogIndex: fill.Position.LogIndex}, EmittedAt: time.Now().UTC(), TraceID: "stats_history", PayloadHash: delivery.Hash(payload), Payload: payload}
	wire, err := envelope.Encode(delivery.Policy{Subjects: map[string]bool{accounting.MarketSubject: true}, EventTypes: map[string]bool{"accounting.market.v1": true}}, accounting.MarketSubject)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = publisher.Publish(t.Context(), accounting.MarketSubject, envelope.EventID, wire); err != nil {
		t.Fatal(err)
	}
	messages, err := transport.Fetch(t.Context())
	if err != nil || len(messages) != 1 {
		t.Fatalf("accounting delivery: %d %v", len(messages), err)
	}
	return messages[0]
}

func (f *statsLongSpanFixture) empty(t *testing.T) {
	t.Helper()
	var checkpoints, trades, commits int
	if err := f.pool.QueryRow(t.Context(), `SELECT (SELECT count(*) FROM stats_delivery.delivery_partition_offsets WHERE checkpoint_digest IS NOT NULL),(SELECT count(*) FROM stats_projection.trade_facts),(SELECT count(*) FROM stats_projection.accounting_market_commits)`).Scan(&checkpoints, &trades, &commits); err != nil || checkpoints != 0 || trades != 0 || commits != 0 {
		t.Fatalf("unadmitted pool: checkpoints=%d trades=%d commits=%d err=%v", checkpoints, trades, commits, err)
	}
}

func (f *statsLongSpanFixture) replay(t *testing.T, original *natsdriver.Message) *natsdriver.Message {
	t.Helper()
	nc := internaltest.ConnectNATS(t, os.Getenv("L20_NATS_URL"))
	js, err := jetstream.New(nc)
	if err != nil {
		t.Fatal(err)
	}
	cfg := natsdriver.ConsumerConfig{Stream: original.Stream, Durable: "stats_replay", Subject: accounting.MarketSubject, Batch: 64, MaxAckPending: 64, MaxWaiting: 1, MaxDeliver: 2, FetchWait: time.Second, AckWait: time.Second}
	native, err := cfg.BrokerConsumerConfig()
	if err != nil {
		t.Fatal(err)
	}
	if _, err = js.CreateConsumer(t.Context(), cfg.Stream, native); err != nil {
		t.Fatal(err)
	}
	transport, err := natsdriver.NewConsumer(t.Context(), nc, cfg)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(transport.Close)
	messages, err := transport.Fetch(t.Context())
	if err != nil || len(messages) != 1 {
		t.Fatalf("accounting replay: %d %v", len(messages), err)
	}
	message := messages[0]
	if message.Stream != original.Stream || message.Sequence != original.Sequence || message.EventID != original.EventID || !bytes.Equal(message.Wire, original.Wire) {
		t.Fatal("accounting replay changed the retained delivery")
	}
	return message
}

func (f *statsLongSpanFixture) assertNodes(t *testing.T) int {
	t.Helper()
	calls := 0
	for _, node := range []*statsHistoryNode{f.primary, f.history} {
		node.mu.Lock()
		if node.stateReads != 0 || node.logReads != 0 {
			t.Errorf("bootstrap node reads: state=%d logs=%d", node.stateReads, node.logReads)
		}
		for _, count := range node.calls {
			calls += count
		}
		node.mu.Unlock()
	}
	return calls
}

func TestStatsBootstrapLongSpanCommitsBeforeCoherentRead(t *testing.T) {
	chain, log, timestamp := localAccountingChain(t)
	receipt, err := chain.TransactionReceipt(t.Context(), log.TxHash)
	if err != nil {
		t.Fatal(err)
	}
	var longCalls int
	for _, span := range []int64{1100000, 10000} {
		t.Run(strconv.FormatInt(span, 10), func(t *testing.T) {
			f := newStatsLongSpanFixture(t, log, receipt, timestamp, span)
			consumer := f.consumer(t)
			message := f.message(t)
			// A source lock holds a proof statement while no partition lease exists.
			if span == 1100000 {
				f.primary.beforeFillBlock = func() {
					lock, err := f.pool.Begin(t.Context())
					if err != nil {
						t.Error(err)
						return
					}
					if _, err = lock.Exec(t.Context(), `LOCK TABLE indexer.indexer_block_completeness IN ACCESS EXCLUSIVE MODE`); err != nil {
						t.Error(err)
						_ = lock.Rollback(t.Context())
						return
					}
					go func() {
						time.Sleep(1200 * time.Millisecond)
						var leases int
						if err := f.pool.QueryRow(t.Context(), `SELECT count(*) FROM stats_delivery.delivery_partition_leases`).Scan(&leases); err != nil || leases != 0 {
							t.Errorf("proof acquired lease: %d %v", leases, err)
						}
						if err := lock.Rollback(context.Background()); err != nil {
							t.Error(err)
						}
					}()
				}
			}
			start := time.Now()
			for attempt := 0; ; attempt++ {
				err = consumer.Consume(t.Context(), message)
				if err == nil {
					break
				}
				if !errors.Is(err, ErrAccountingSourceUnavailable) || attempt >= 9 {
					t.Fatal(err)
				}
			}
			if span == 1100000 && time.Since(start) <= f.config.LeaseDuration {
				t.Fatal("proof did not exceed partition lease duration")
			}
			calls := f.assertNodes(t)
			if span == 1100000 {
				longCalls = calls
				if f.primary.pruned == 0 || f.history.historicalAnswers != 2 {
					t.Fatalf("creation history: pruned=%d answered=%d", f.primary.pruned, f.history.historicalAnswers)
				}
			} else if calls > longCalls {
				t.Fatalf("node calls grew for shorter span: %d > %d", calls, longCalls)
			}
			if _, err = f.pool.Exec(t.Context(), f.config.Repository.Phase2MigrationSQL()); err != nil {
				t.Fatal(err)
			}
			reader, err := projectionstats.NewReadService(f.config.Repository, projectionstats.ReadConfig{SourceSchema: "indexer", SourceStream: "canonical", ProjectionVersion: "projection-v1", CursorKey: bytes.Repeat([]byte{1}, 32), CursorTTL: time.Minute})
			if err != nil {
				t.Fatal(err)
			}
			page, err := reader.Trades(t.Context(), f.publication.Fill.PoolAddress, "", 10, "", timestamp+1)
			if err != nil || len(page.Trades) != 1 || page.Watermark.Height != "2000000" {
				t.Fatalf("coherent stats: %+v %v", page, err)
			}
			// A fresh process rechecks the durable checkpoint before redelivery.
			restarted := f.consumer(t)
			message = f.replay(t, message)
			for attempt := 0; ; attempt++ {
				err = restarted.Consume(t.Context(), message)
				if err == nil {
					break
				}
				if !errors.Is(err, ErrAccountingSourceUnavailable) || attempt >= 9 {
					t.Fatal(err)
				}
			}
			var trades, commits int
			if err = f.pool.QueryRow(t.Context(), `SELECT (SELECT count(*) FROM stats_projection.trade_facts),(SELECT count(*) FROM stats_projection.accounting_market_commits)`).Scan(&trades, &commits); err != nil || trades != 1 || commits != 1 {
				t.Fatalf("redelivery: %d %d %v", trades, commits, err)
			}
		})
	}
}

func TestStatsBootstrapLongSpanRejectsIncompleteEvidence(t *testing.T) {
	chain, log, timestamp := localAccountingChain(t)
	receipt, err := chain.TransactionReceipt(t.Context(), log.TxHash)
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct{ name, sql string }{
		{"earlier swap", `INSERT INTO indexer.indexer_raw_logs(chain_id,transaction_hash,log_index,height,transaction_index,block_hash,emitter,topics,data) SELECT chain_id,'0x'||repeat('e',64),log_index,1000000,transaction_index,block_hash,emitter,topics,data FROM indexer.indexer_raw_logs WHERE log_index=1; UPDATE indexer.indexer_block_completeness SET raw_count=1 WHERE height=1000000; UPDATE indexer.indexer_ingest_commits SET record_count=2 WHERE batch_id='first'`},
		{"missing completeness", `DELETE FROM indexer.indexer_block_completeness WHERE height=1000000`},
		{"evidence changed at lease", `CREATE FUNCTION indexer.change_origin() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN UPDATE indexer.indexer_ingest_commits SET batch_id='replacement' WHERE batch_id='first'; RETURN NEW; END $$; CREATE TRIGGER change_origin AFTER INSERT ON stats_delivery.delivery_partition_leases FOR EACH ROW EXECUTE FUNCTION indexer.change_origin()`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := newStatsLongSpanFixture(t, log, receipt, timestamp, 1100000)
			if _, err := f.pool.Exec(t.Context(), tc.sql); err != nil {
				t.Fatal(err)
			}
			consumer := f.consumer(t)
			message := f.message(t)
			for attempt := 0; ; attempt++ {
				err = consumer.Consume(t.Context(), message)
				if !errors.Is(err, ErrAccountingSourceUnavailable) {
					t.Fatalf("source refusal: %v", err)
				}
				var acquired bool
				if e := f.pool.QueryRow(t.Context(), `SELECT EXISTS(SELECT 1 FROM stats_delivery.delivery_partition_leases)`).Scan(&acquired); e != nil {
					t.Fatal(e)
				}
				if tc.name != "evidence changed at lease" || acquired {
					break
				}
				if attempt >= 9 {
					t.Fatal("proof did not reach evidence recheck")
				}
			}
			f.empty(t)
			f.assertNodes(t)
		})
	}
}

func TestStatsBootstrapLongSpanRequiresHistoryFallback(t *testing.T) {
	chain, log, timestamp := localAccountingChain(t)
	receipt, err := chain.TransactionReceipt(t.Context(), log.TxHash)
	if err != nil {
		t.Fatal(err)
	}
	f := newStatsLongSpanFixture(t, log, receipt, timestamp, 1100000)
	f.config.RPCFallback = nil
	err = f.consumer(t).Consume(t.Context(), f.message(t))
	if !errors.Is(err, ErrAccountingSourceUnavailable) || !strings.Contains(f.log.String(), "height=900000") || !strings.Contains(f.log.String(), "node with the full history") {
		t.Fatalf("historical delivery: %v log=%s", err, f.log.String())
	}
	f.empty(t)
	if f.primary.pruned != 1 {
		t.Fatalf("primary historical refusal: %d", f.primary.pruned)
	}
	f.assertNodes(t)
}

func TestStatsBootstrapLongSpanResumesProof(t *testing.T) {
	chain, log, timestamp := localAccountingChain(t)
	receipt, err := chain.TransactionReceipt(t.Context(), log.TxHash)
	if err != nil {
		t.Fatal(err)
	}
	f := newStatsLongSpanFixture(t, log, receipt, timestamp, 1100000)
	f.config.ProofTimeout = 2 * time.Second
	f.config.MaxReplayBlocks = 32
	consumer := f.consumer(t)
	message := f.message(t)
	previous := int64(899999)
	for attempt := 0; ; attempt++ {
		err = consumer.Consume(t.Context(), message)
		progress := consumer.bootstrap.pools[f.publication.Fill.PoolAddress]
		if progress.last < previous {
			t.Fatalf("proof repeated a completed slice: %d < %d", progress.last, previous)
		}
		if err == nil {
			if attempt == 0 {
				t.Fatal("bounded proof did not yield")
			}
			break
		}
		if !errors.Is(err, ErrAccountingSourceUnavailable) || attempt >= 99 {
			t.Fatal(err)
		}
		if progress.last == previous || progress.rpcStep != 4 {
			t.Fatalf("proof progress: height=%d previous=%d nodeStage=%d", progress.last, previous, progress.rpcStep)
		}
		previous = progress.last
		f.empty(t)
	}
	if calls := f.assertNodes(t); calls != 8 {
		t.Fatalf("completed node evidence was read again: %d", calls)
	}
}
