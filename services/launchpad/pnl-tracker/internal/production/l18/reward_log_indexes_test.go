package l18

import (
	"context"
	"encoding/json"
	"fmt"
	"math/big"
	"os"
	"strings"
	"sync"
	"testing"

	"github.com/Sidiora-Technologies/KindleLaunch/shared/db/production/coordinatortest"
	"github.com/ethereum/go-ethereum/common"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type rewardLogStatement struct {
	sql  string
	args []any
}

type rewardLogRecorder struct {
	mu         sync.Mutex
	statements []rewardLogStatement
}

func (r *rewardLogRecorder) TraceQueryStart(ctx context.Context, _ *pgx.Conn, data pgx.TraceQueryStartData) context.Context {
	if strings.Contains(data.SQL, "FROM indexer.indexer_raw_logs_v1") {
		r.mu.Lock()
		r.statements = append(r.statements, rewardLogStatement{data.SQL, append([]any(nil), data.Args...)})
		r.mu.Unlock()
	}
	return ctx
}
func (r *rewardLogRecorder) TraceQueryEnd(context.Context, *pgx.Conn, pgx.TraceQueryEndData) {}

type rewardLogPlan struct {
	NodeType string          `json:"Node Type"`
	Relation string          `json:"Relation Name"`
	Index    string          `json:"Index Name"`
	Plans    []rewardLogPlan `json:"Plans"`
}

func TestRewardLogIndexWalletReads(t *testing.T) {
	ctx := context.Background()
	f := rewardHistoryChain(t)
	f.db.mu.Lock()
	rewardCandidateMarkets(f.db, 4)
	f.db.peers.Transfers = []rewardHistoryTransfer{
		rewardCandidateTransfer("1", f.chain.Alice, 10),
		rewardCandidateTransfer("2", f.chain.Bob, 10),
		rewardCandidateTransfer("2", f.chain.Alice, 11),
		rewardCandidateTransfer("3", f.chain.Alice, 10),
		rewardCandidateTransfer("3", f.chain.Bob, 12),
		rewardCandidateTransfer("4", f.chain.Bob, 10),
	}
	f.db.peers.Claims = []rewardHistoryClaim{{Pool: f.db.peers.Markets[2].Pool, Wallet: f.chain.Alice, Height: 11, BlockHash: rewardHistoryHash(11), Data: "0x" + fmt.Sprintf("%064x", 7)}}
	peers := f.db.peers
	f.db.mu.Unlock()
	database := coordinatortest.Bootstrap(t, os.Getenv("PNL_TEST_POSTGRES_DSN"), "pnl_runtime")
	for _, id := range []int64{60007, 60008} {
		if database.Applied[id] == "" || f.db.catalog.Applied[id] != database.Applied[id] {
			t.Fatalf("raw log index migration %d absent from history catalog", id)
		}
	}
	admin, err := pgxpool.New(ctx, database.AdminDSN)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(admin.Close)
	execute := func(sql string, args ...any) {
		t.Helper()
		if _, err := admin.Exec(ctx, sql, args...); err != nil {
			t.Fatal(err)
		}
	}
	for _, m := range peers.Markets {
		execute(`INSERT INTO indexer.indexer_canonical_creations(token,pool,creator,factory,optical,nft_id,height,block_hash,transaction_hash,transaction_index,log_index,block_timestamp,provenance,payload)
   VALUES($1,$1,$2,$2,$2,$3::numeric,$4,$5,$6,0,$7,1,'{}','{}')`, m.Pool, m.Creator, m.ID, m.Height, m.BlockHash, m.TxHash, rewardHistoryUint(m.LogIndex))
	}
	execute(`INSERT INTO indexer.indexer_canonical_creations(token,pool,creator,factory,optical,nft_id,height,block_hash,transaction_hash,transaction_index,log_index,block_timestamp,provenance,payload)
 SELECT '0x'||lpad(to_hex(g+1000),40,'0'),'0x'||lpad(to_hex(g+1000),40,'0'),$1,$1,$1,g+4,10,$2,'0x'||lpad(to_hex(g+100000),64,'0'),0,0,1,'{}','{}' FROM generate_series(1,20000) g`, f.chain.Bob, rewardHistoryHash(10))
	execute(`INSERT INTO indexer.indexer_block_completeness(chain_id,stream_id,height,block_hash,raw_count,fetch_complete,required_decode_complete)
 SELECT 125,$1,g,'0x'||lpad(to_hex(g),64,'0'),CASE WHEN g=10 THEN 20000 ELSE 1 END,true,true FROM generate_series(10,15) g`, rewardHistoryStream)
	execute(`INSERT INTO indexer.indexer_ingest_watermarks VALUES(125,$1,15,15)`, rewardHistoryStream)
	insertLog := func(number int, height uint64, emitter string, topics []string, data []byte) {
		t.Helper()
		raw, err := json.Marshal(topics)
		if err != nil {
			t.Fatal(err)
		}
		execute(`INSERT INTO indexer.indexer_raw_logs(chain_id,transaction_hash,log_index,height,transaction_index,block_hash,emitter,topics,data)
   VALUES(125,$1,$2,$3,0,$4,$5,$6,$7)`, rewardHistoryHash(uint64(number+1000000)), number, height, rewardHistoryHash(height), emitter, raw, data)
	}
	for i, transfer := range peers.Transfers {
		id, ok := new(big.Int).SetString(transfer.NFTID, 10)
		if !ok {
			t.Fatal("invalid NFT identity")
		}
		insertLog(i, transfer.Height, NFT, []string{nft.Events["Transfer"].ID.Hex(), addressTopic(f.chain.Bob), addressTopic(transfer.Owner), common.BigToHash(id).Hex()}, []byte{})
	}
	insertLog(len(peers.Transfers), 11, Accumulator, []string{accumulator.Events["FeesClaimed"].ID.Hex(), addressTopic(peers.Markets[2].Pool), addressTopic(f.chain.Alice)}, common.LeftPadBytes(big.NewInt(7).Bytes(), 32))
	execute(`INSERT INTO indexer.indexer_raw_logs(chain_id,transaction_hash,log_index,height,transaction_index,block_hash,emitter,topics,data)
 SELECT 125,'0x'||lpad(to_hex(g+2000000),64,'0'),0,10,0,$1,
 CASE WHEN g%2=0 THEN $2 ELSE $3 END,
 CASE WHEN g%2=0 THEN jsonb_build_array($4::text,'0x'||lpad(to_hex(g+1000),64,'0'),$6::text)
 ELSE jsonb_build_array($5::text,$6::text,$6::text,'0x'||lpad(to_hex(g+4),64,'0')) END,
 decode(repeat('00',32),'hex') FROM generate_series(1,20000) g`, rewardHistoryHash(10), Accumulator, NFT, accumulator.Events["FeesClaimed"].ID.Hex(), nft.Events["Transfer"].ID.Hex(), addressTopic(f.chain.Bob))
	var unrelated, emitters int
	if err := admin.QueryRow(ctx, `SELECT count(*),count(DISTINCT emitter) FROM indexer.indexer_raw_logs WHERE topics->>2=$1`, addressTopic(f.chain.Bob)).Scan(&unrelated, &emitters); err != nil || unrelated < 20000 || emitters != 2 {
		t.Fatalf("unrelated logs=%d emitters=%d: %v", unrelated, emitters, err)
	}
	execute(`UPDATE indexer.indexer_block_completeness b SET raw_count=(SELECT count(*) FROM indexer.indexer_raw_logs l WHERE l.chain_id=b.chain_id AND l.height=b.height)`)
	execute(`ANALYZE indexer.indexer_raw_logs; ANALYZE indexer.indexer_canonical_creations; ANALYZE indexer.indexer_block_completeness`)
	recorder := &rewardLogRecorder{}
	config, err := pgxpool.ParseConfig(database.Runtimes["pnl_runtime"].DSN)
	if err != nil {
		t.Fatal(err)
	}
	config.ConnConfig.Tracer = recorder
	runtime, err := pgxpool.NewWithConfig(ctx, config)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(runtime.Close)
	f.ledger.pool = runtime
	page, err := f.journey.Rewards(ctx, Actor{Wallet: f.chain.Alice}, "", "", 100)
	if err != nil {
		t.Fatal(err)
	}
	if len(page.Items) != 3 || page.NextCursor != nil {
		t.Fatalf("wallet rewards=%+v", page)
	}
	for i, item := range page.Items {
		wantClaimed := "0"
		if i == 2 {
			wantClaimed = "7"
		}
		if item.RewardId != fmt.Sprintf("creator:%d", i+1) || item.Wallet != f.chain.Alice || string(item.ClaimedRaw) != wantClaimed {
			t.Fatalf("wallet reward %d=%+v", i, item)
		}
	}
	recorder.mu.Lock()
	statements := append([]rewardLogStatement(nil), recorder.statements...)
	recorder.mu.Unlock()
	tx, err := runtime.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(ctx)
	if _, err = tx.Exec(ctx, `SET LOCAL enable_seqscan=off`); err != nil {
		t.Fatal(err)
	}
	seen := map[string]int{}
	for _, statement := range statements {
		kind, index := "candidates", "indexer_raw_emitter_topic2_order"
		if strings.HasPrefix(statement.sql, "SELECT l.topics->>2") {
			kind, index = "owner", "indexer_raw_emitter_topic3_order"
		}
		if strings.HasPrefix(statement.sql, "SELECT l.height,") {
			kind = "claims"
		}
		var raw []byte
		if err = tx.QueryRow(ctx, "EXPLAIN (ANALYZE, FORMAT JSON) "+statement.sql, statement.args...).Scan(&raw); err != nil {
			t.Fatalf("%s explain: %v", kind, err)
		}
		var plans []struct {
			Plan rewardLogPlan `json:"Plan"`
		}
		if err = json.Unmarshal(raw, &plans); err != nil || len(plans) != 1 {
			t.Fatalf("%s plan: %s: %v", kind, raw, err)
		}
		scans, indexes := 0, 0
		var inspect func(rewardLogPlan)
		inspect = func(plan rewardLogPlan) {
			if plan.Relation == "indexer_raw_logs" {
				scans++
				if strings.Contains(plan.NodeType, "Seq Scan") {
					t.Fatalf("%s sequential raw read: %s", kind, raw)
				}
			}
			if plan.Index == index {
				indexes++
			}
			for _, child := range plan.Plans {
				inspect(child)
			}
		}
		inspect(plans[0].Plan)
		if scans == 0 || indexes < scans {
			t.Fatalf("%s missing %s: %s", kind, index, raw)
		}
		seen[kind]++
	}
	if seen["candidates"] != 1 || seen["owner"] != 3 || seen["claims"] != 3 {
		t.Fatalf("recorded reward reads=%v", seen)
	}
}
