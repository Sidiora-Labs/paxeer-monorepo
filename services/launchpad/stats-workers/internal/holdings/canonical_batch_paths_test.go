package holdings

import (
	"encoding/json"
	"errors"
	"math/big"
	"strings"
	"testing"

	"github.com/ethereum/go-ethereum/crypto"
)

func TestCanonicalCatchupPagesCommittedTransfersBeforeCompleteCoverage(t *testing.T) {
	ctx, pool, mint := canonicalLocalFixture(t)
	const schema = "holdings_batch_pages"
	ddl, err := SchemaSQL(schema)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, ddl); err != nil {
		t.Fatal(err)
	}
	repo, err := NewRepository(pool, schema)
	if err != nil {
		t.Fatal(err)
	}
	transfer := event(mint.Token, mint.To, addr(86), "40", 2)
	signature := crypto.Keccak256Hash([]byte("Transfer(address,address,uint256)"))
	topics, err := json.Marshal([]string{signature.Hex(), "0x" + strings.Repeat("0", 24) + transfer.From[2:], "0x" + strings.Repeat("0", 24) + transfer.To[2:]})
	if err != nil {
		t.Fatal(err)
	}
	data := big.NewInt(40).FillBytes(make([]byte, 32))
	payload, err := json.Marshal(transfer)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO indexer.indexer_raw_logs(chain_id,transaction_hash,log_index,height,transaction_index,block_hash,emitter,topics,data) VALUES(125,$1,2,10,0,$2,$3,$4,$5)`, transfer.TransactionHash, transfer.BlockHash, transfer.Token, topics, data); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO indexer.indexer_token_transfers(chain_id,transaction_hash,log_index,token,from_address,to_address,amount_raw,height,transaction_index,block_hash,provenance,payload) VALUES(125,$1,2,$2,$3,$4,40,10,0,$5,'{}',$6)`, transfer.TransactionHash, transfer.Token, transfer.From, transfer.To, transfer.BlockHash, payload); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `UPDATE indexer.indexer_block_completeness SET raw_count=3 WHERE chain_id=125 AND stream_id='canonical' AND height=10`); err != nil {
		t.Fatal(err)
	}
	if err := repo.DiscoverCanonical(ctx, mint.Token, "canonical"); err != nil {
		t.Fatal(err)
	}
	first, err := repo.CatchUpCanonicalAt(ctx, mint.Token, "canonical", "10", 1)
	if err != nil || first.Applied != 1 || !first.More || first.Coverage == "complete" {
		t.Fatalf("first page: %+v %v", first, err)
	}
	second, err := repo.CatchUpCanonicalAt(ctx, mint.Token, "canonical", "10", 1)
	if err != nil || second.Applied != 1 || second.More || second.Coverage != "complete" {
		t.Fatalf("second page: %+v %v", second, err)
	}
	snapshot, err := repo.Snapshot(ctx, mint.Token, 10)
	if err != nil || snapshot.Supply != "100" || len(snapshot.Balances) != 2 || snapshot.Balances[0].Raw != "60" || snapshot.Balances[1].Raw != "40" {
		t.Fatalf("committed balances: %+v %v", snapshot, err)
	}
	if _, err := repo.CatchUpCanonicalAt(ctx, mint.Token, "canonical", "11", 1); !errors.Is(err, ErrOrder) {
		t.Fatalf("future bound accepted: %v", err)
	}
}
