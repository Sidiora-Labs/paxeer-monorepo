package app

import (
	"context"
	"encoding/json"
	"fmt"
	"math/big"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Sidiora-Technologies/KindleLaunch/core/stats-workers/internal/holdings"
	"github.com/Sidiora-Technologies/KindleLaunch/core/stats-workers/internal/internaltest"
	"github.com/Sidiora-Technologies/KindleLaunch/shared/delivery"
	"github.com/ethereum/go-ethereum/crypto"
)

func TestCanonicalHolderJobAdvancesCommittedTransfers(t *testing.T) {
	ctx := context.Background()
	db := internaltest.NewPostgres(t)
	if _, err := db.Exec(ctx, "CREATE SCHEMA indexer"); err != nil {
		t.Fatal(err)
	}
	root, err := filepath.Abs("../../../../..")
	if err != nil {
		t.Fatal(err)
	}
	for _, id := range []string{"60000", "190001"} {
		if id == "190001" {
			ddl, err := delivery.PartitionSchemaDDL("indexer")
			if err != nil {
				t.Fatal(err)
			}
			if _, err := db.Exec(ctx, ddl); err != nil {
				t.Fatal(err)
			}
		}
		ddl, err := os.ReadFile(filepath.Join(root, "migrations", id+".sql"))
		if err != nil {
			t.Fatal(err)
		}
		if _, err := db.Exec(ctx, string(ddl)); err != nil {
			t.Fatal(err)
		}
	}
	const schema = "job_holders"
	holderDDL, err := holdings.SchemaSQL(schema)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(ctx, holderDDL); err != nil {
		t.Fatal(err)
	}
	repository, err := holdings.NewRepository(db, schema)
	if err != nil {
		t.Fatal(err)
	}
	address := func(n int) string { return fmt.Sprintf("0x%040x", n) }
	token, wallet, pool := address(81), address(82), address(83)
	blockHash, transactionHash := "0x"+strings.Repeat("a", 64), "0x"+strings.Repeat("b", 64)
	metadata := `{"metadata":{"decimals":6,"totalSupply":"100"}}`
	if _, err := db.Exec(ctx, `INSERT INTO indexer.indexer_canonical_creations(token,pool,creator,factory,optical,nft_id,height,block_hash,transaction_hash,transaction_index,log_index,block_timestamp,provenance,payload) VALUES($1,$2,$3,$4,$5,1,10,$6,$7,0,0,100,'{}',$8)`, token, pool, wallet, address(84), address(85), blockHash, transactionHash, metadata); err != nil {
		t.Fatal(err)
	}
	for _, sql := range []string{
		`INSERT INTO indexer.indexer_ingest_leases(chain_id,stream_id,owner_instance,fence_token,expires_at) VALUES(125,'canonical','indexer',1,clock_timestamp()+interval '1 hour')`,
		`INSERT INTO indexer.indexer_ingest_watermarks(chain_id,stream_id,raw_complete_height,typed_complete_height) VALUES(125,'canonical',10,10)`,
	} {
		if _, err := db.Exec(ctx, sql); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := db.Exec(ctx, `INSERT INTO indexer.indexer_block_completeness(chain_id,stream_id,height,block_hash,raw_count,fetch_complete,required_decode_complete) VALUES(125,'canonical',10,$1,2,true,true)`, blockHash); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(ctx, `INSERT INTO indexer.indexer_ingest_commits(chain_id,stream_id,batch_id,previous_height,last_complete_height,block_hash,record_count,outbox_count) VALUES(125,'canonical','first',9,10,$1,2,0)`, blockHash); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(ctx, `INSERT INTO indexer.indexer_raw_logs(chain_id,transaction_hash,log_index,height,transaction_index,block_hash,emitter,topics,data) VALUES(125,$1,0,10,0,$2,$3,'[]','\x')`, transactionHash, blockHash, address(84)); err != nil {
		t.Fatal(err)
	}
	signature := crypto.Keccak256Hash([]byte("Transfer(address,address,uint256)"))
	topics, err := json.Marshal([]string{signature.Hex(), "0x" + strings.Repeat("0", 64), "0x" + strings.Repeat("0", 24) + wallet[2:]})
	if err != nil {
		t.Fatal(err)
	}
	amount := big.NewInt(100).FillBytes(make([]byte, 32))
	if _, err := db.Exec(ctx, `INSERT INTO indexer.indexer_raw_logs(chain_id,transaction_hash,log_index,height,transaction_index,block_hash,emitter,topics,data) VALUES(125,$1,1,10,0,$2,$3,$4,$5)`, transactionHash, blockHash, token, topics, amount); err != nil {
		t.Fatal(err)
	}
	event := holdings.Transfer{ChainID: "125", Token: token, From: holdings.Zero, To: wallet, Amount: "100", Height: "10", TransactionIndex: "0", LogIndex: "1", TransactionHash: transactionHash, BlockHash: blockHash}
	payload, err := json.Marshal(event)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(ctx, `INSERT INTO indexer.indexer_token_transfers(chain_id,transaction_hash,log_index,token,from_address,to_address,amount_raw,height,transaction_index,block_hash,provenance,payload) VALUES(125,$1,1,$2,$3,$4,100,10,0,$5,'{}',$6)`, transactionHash, token, holdings.Zero, wallet, blockHash, payload); err != nil {
		t.Fatal(err)
	}
	after := ""
	refreshed := []string{}
	if err := catchUpStatsHolders(ctx, db, repository, "canonical", 2, 10, &after, func(_ context.Context, token string) error { refreshed = append(refreshed, token); return nil }); err != nil {
		t.Fatal(err)
	}
	if after != token || len(refreshed) != 1 || refreshed[0] != token {
		t.Fatalf("holder cursor=%q refresh=%v", after, refreshed)
	}
	snapshot, err := repository.Snapshot(ctx, token, 10)
	if err != nil || snapshot.Supply != "100" || snapshot.Coverage != "complete" || len(snapshot.Balances) != 1 || snapshot.Balances[0].Address != wallet {
		t.Fatalf("canonical holders: %+v %v", snapshot, err)
	}
	if err := catchUpStatsHolders(ctx, db, repository, "canonical", 2, 10, &after); err != nil || after != "" {
		t.Fatalf("completed scan: %q %v", after, err)
	}
	if err := catchUpStatsHolders(ctx, nil, repository, "canonical", 2, 10, &after); err == nil {
		t.Fatal("missing source pool accepted")
	}
}
