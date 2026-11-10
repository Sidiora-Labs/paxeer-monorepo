package app

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/Sidiora-Technologies/KindleLaunch/shared/accounting"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Each reopen connects to the same real, isolated PostgreSQL database. The
// registered migrations, constraints, transactions and stream function govern
// persistence across the application restarts exercised by the tests.
func repairM934OpenPool(t *testing.T, dsn string) *pgxpool.Pool {
	t.Helper()
	pool, err := pgxpool.New(context.Background(), dsn)
	if err != nil {
		t.Fatal(err)
	}
	return pool
}

func repairM934Seed(t *testing.T, dsn string, cutoff time.Time) {
	t.Helper()
	ctx := context.Background()
	pool := repairM934OpenPool(t, dsn)
	defer pool.Close()
	exec := func(sql string, args ...any) {
		t.Helper()
		if _, err := pool.Exec(ctx, sql, args...); err != nil {
			t.Fatal(err)
		}
	}
	exec(`INSERT INTO indexer.indexer_ingest_leases(chain_id,stream_id,owner_instance,fence_token,expires_at) VALUES(125,$1,'indexer-1',1,clock_timestamp()+interval '10 minutes')`, sourceStream)
	exec(`INSERT INTO indexer.indexer_ingest_watermarks(chain_id,stream_id,raw_complete_height,typed_complete_height) VALUES(125,$1,200,200)`, sourceStream)
	exec(`INSERT INTO indexer.indexer_block_completeness(chain_id,stream_id,height,block_hash,raw_count,fetch_complete,required_decode_complete,completed_at) VALUES(125,$1,200,$2,122,true,true,$3)`, sourceStream, "0x"+strings.Repeat("c8", 32), cutoff)
	for i := 1; i <= 61; i++ {
		address, token := fmt.Sprintf("0x%040x", i), fmt.Sprintf("0x%040x", i+1000)
		creator := "0x" + strings.Repeat("1", 40)
		creationHash := fmt.Sprintf("0x%064x", i+10000)
		exec(`INSERT INTO indexer.indexer_canonical_creations(token,pool,creator,factory,optical,nft_id,height,block_hash,transaction_hash,transaction_index,log_index,block_timestamp,provenance,payload) VALUES($1,$2,$3,$3,$3,$4::bigint,$4::bigint,$5,$5,0,0,$6,'{}','{}')`, token, address, creator, i, creationHash, cutoff.Unix()-600)
		// Two committed buys establish price, positive depth, supply and traders.
		// Alternating final prices preserve positive and negative changes; with
		// sixty visible pools, movers still has candidates beyond trending's 50.
		for j := 0; j < 2; j++ {
			sequence := i*2 + j
			txHash := fmt.Sprintf("0x%064x", sequence)
			height := 100 + j
			blockHash := fmt.Sprintf("0x%064x", height)
			amount := "1000000000"
			if j == 1 {
				amount = "1250000000"
				if i%2 == 0 {
					amount = "750000000"
				}
			}
			wallet := fmt.Sprintf("0x%040x", j+2000)
			timestamp := cutoff.Unix() - 600 + int64(j)*540
			fill := accounting.FillV1{
				Version: accounting.FillVersion, SourceFillID: txHash + ":0",
				Position:      accounting.SourcePosition{ChainID: "125", Height: strconv.Itoa(height), TransactionIndex: "0", LogIndex: "0", TransactionHash: txHash, BlockHash: blockHash},
				ActualEmitter: address, PoolAddress: address,
				Token: accounting.Asset{Address: token, Decimals: 18}, Settlement: accounting.Asset{Address: "0x85fcd13735f4309833a503ee804ea32395851479", Decimals: 6},
				Side: "buy", TokenAmountRaw: "1000" + strings.Repeat("0", 18), USDLAmountRaw: amount, PaidInputRaw: amount, FeeRaw: "0",
				InputDerivationQuality: "legacy_unavailable", FeeDisposition: "distributed_usdl",
				Attribution: accounting.Attribution{TransactionSender: wallet, Trader: &wallet, Payer: &wallet, Recipient: &wallet, Quality: "exact_receipt", Evidence: []accounting.EvidenceRef{}},
				Execution:   accounting.ExecutionPolicy{Quality: "unavailable", Evidence: []accounting.EvidenceRef{}}, BlockTimestampSeconds: strconv.FormatInt(timestamp, 10),
			}
			payload, err := json.Marshal(fill)
			if err != nil {
				t.Fatal(err)
			}
			digest := fmt.Sprintf("%x", sha256.Sum256(payload))
			exec(`INSERT INTO indexer.indexer_raw_logs(chain_id,transaction_hash,log_index,height,transaction_index,block_hash,emitter,topics,data) VALUES(125,$1,0,$2,0,$3,$4,'[]','\x00')`, txHash, height, blockHash, address)
			exec(`INSERT INTO indexer.indexer_accounting_fills(source_fill_id,source_fact_digest,chain_id,transaction_hash,log_index,pool_address,payload) VALUES($1,$2,125,$3,0,$4,$5)`, fill.SourceFillID, digest, txHash, address, payload)
			exec(`INSERT INTO stats.accounting_market_commits(publication_id,source_fill_id,source_fact_digest,projection_version,pool,bucket_start,position,fill,sequence,revision) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$9)`, uuid.NewString(), fill.SourceFillID, digest, projection, address, timestamp-timestamp%60, `{}`, payload, sequence)
		}
	}
}
