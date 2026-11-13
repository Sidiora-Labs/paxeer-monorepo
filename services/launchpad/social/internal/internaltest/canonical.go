package internaltest

import (
	"context"
	"encoding/json"
	"math/big"
	"os"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/ethereum/go-ethereum/rpc"
	"github.com/jackc/pgx/v5/pgxpool"
)

// SeedCanonicalSource records an actual chain-125 receipt through the indexer's
// registered raw-log and completeness interfaces, the inputs of social's owner
// functions. The constructor emits LOG0; no protocol event requires decoding.
func SeedCanonicalSource(t *testing.T, db *pgxpool.Pool) {
	t.Helper()
	endpoint := os.Getenv("L34_ANVIL_URL")
	if endpoint == "" {
		t.Fatal("L34_ANVIL_URL is required for mined canonical source evidence")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	transport, err := rpc.DialContext(ctx, endpoint)
	if err != nil {
		t.Fatal(err)
	}
	defer transport.Close()
	client := ethclient.NewClient(transport)
	chain, err := client.ChainID(ctx)
	if err != nil || chain.Cmp(big.NewInt(125)) != 0 {
		t.Fatalf("chain-125 source required: %v %v", chain, err)
	}
	var accounts []common.Address
	if err = transport.CallContext(ctx, &accounts, "eth_accounts"); err != nil || len(accounts) == 0 {
		t.Fatalf("unlocked source account required: %v", err)
	}
	var hash common.Hash
	if err = transport.CallContext(ctx, &hash, "eth_sendTransaction", map[string]string{"from": accounts[0].Hex(), "data": "0x60006000a000", "gas": "0x186a0"}); err != nil {
		t.Fatal(err)
	}
	var receipt *types.Receipt
	for ctx.Err() == nil {
		receipt, err = client.TransactionReceipt(ctx, hash)
		if err == nil {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if err != nil || receipt == nil || receipt.Status != types.ReceiptStatusSuccessful || len(receipt.Logs) != 1 {
		t.Fatalf("mined LOG0 receipt required: %+v %v", receipt, err)
	}
	for _, entry := range receipt.Logs {
		topics, err := json.Marshal(entry.Topics)
		if err != nil {
			t.Fatal(err)
		}
		if _, err = db.Exec(ctx, `INSERT INTO indexer.indexer_raw_logs(chain_id,transaction_hash,log_index,height,transaction_index,block_hash,emitter,topics,data) VALUES(125,$1,$2,$3,$4,$5,$6,$7,$8)`, entry.TxHash.Hex(), entry.Index, entry.BlockNumber, entry.TxIndex, entry.BlockHash.Hex(), entry.Address.Hex(), topics, entry.Data); err != nil {
			t.Fatal(err)
		}
	}
	if _, err = db.Exec(ctx, `INSERT INTO indexer.indexer_block_completeness(chain_id,stream_id,height,block_hash,raw_count,fetch_complete,required_decode_complete) VALUES(125,'social-fixture',$1,$2,$3,true,true)`, receipt.BlockNumber.Int64(), receipt.BlockHash.Hex(), len(receipt.Logs)); err != nil {
		t.Fatal(err)
	}
}
