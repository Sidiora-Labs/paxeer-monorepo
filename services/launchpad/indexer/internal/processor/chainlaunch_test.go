package processor

import (
	"context"
	"fmt"
	"math/big"
	"os"
	"strings"
	"testing"

	"github.com/Sidiora-Technologies/KindleLaunch/core/indexer/internal/store"
	"github.com/Sidiora-Technologies/KindleLaunch/core/indexer/internal/store/canonicaltest"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/jackc/pgx/v5/pgxpool"
)

func chainLaunchTestABI(t *testing.T) abi.ABI {
	t.Helper()
	data, err := os.ReadFile("../../../../../precompiles/launchpad/abi.json")
	if err != nil {
		t.Fatal(err)
	}
	parsed, err := abi.JSON(strings.NewReader(string(data)))
	if err != nil {
		t.Fatal(err)
	}
	return parsed
}

func chainLaunchRecord(t *testing.T, a abi.ABI, name string, height int64, index uint32, topics []common.Hash, values ...any) store.RawRecord {
	t.Helper()
	event := a.Events[name]
	data, err := event.Inputs.NonIndexed().Pack(values...)
	if err != nil {
		t.Fatal(err)
	}
	all := []string{strings.ToLower(event.ID.Hex())}
	for _, topic := range topics {
		all = append(all, strings.ToLower(topic.Hex()))
	}
	return store.RawRecord{TransactionHash: fmt.Sprintf("0x%064x", 500+index), BlockHash: fmt.Sprintf("0x%064x", height), Emitter: strings.ToLower(common.HexToAddress("0x1017").Hex()), Height: height, TransactionIndex: index, LogIndex: index, Topics: all, Data: data}
}

func chainLaunchBatch(t *testing.T, pool *pgxpool.Pool) store.CanonicalBatch {
	t.Helper()
	b := prepare(t, pool)
	a := chainLaunchTestABI(t)
	poolAddress := common.HexToAddress("0x" + strings.Repeat("2", 40))
	token := common.HexToAddress("0x" + strings.Repeat("3", 40))
	creator := common.HexToAddress("0x" + strings.Repeat("4", 40))
	chain := common.BigToHash(big.NewInt(7))
	epoch := common.BigToHash(big.NewInt(1))
	b.Records = append(b.Records,
		chainLaunchRecord(t, a, "ChainLaunched", 101, 4, []common.Hash{chain, common.BytesToHash(poolAddress.Bytes()), common.BytesToHash(token.Bytes())}, uint64(777), big.NewInt(9), creator, big.NewInt(123456), big.NewInt(101)),
		chainLaunchRecord(t, a, "ChainRevenueEpochOpened", 101, 5, []common.Hash{chain, epoch}, big.NewInt(300), uint64(100), big.NewInt(1000), big.NewInt(123456), big.NewInt(101)),
		chainLaunchRecord(t, a, "ChainRevenueClaimed", 101, 6, []common.Hash{chain, epoch, common.BytesToHash(creator.Bytes())}, big.NewInt(100), big.NewInt(123456), big.NewInt(101)),
	)
	for i := 4; i < len(b.Records); i++ {
		b.Records[i].BlockHash = b.Blocks[0].Hash
	}
	b.Blocks[0].RawCount = len(b.Records)
	return b
}

func chainLaunchState(t *testing.T, pool *pgxpool.Pool) string {
	t.Helper()
	var chains, epochs, claims, home int64
	var chainID, feeRight, status string
	err := pool.QueryRow(context.Background(), `SELECT (SELECT count(*) FROM indexer.launched_chains),(SELECT count(*) FROM indexer.chain_revenue_epochs),(SELECT count(*) FROM indexer.chain_revenue_claims),(SELECT home_chain FROM indexer.pools LIMIT 1),chain_id::text,fee_right,status FROM indexer.launched_chains WHERE chain=7`).Scan(&chains, &epochs, &claims, &home, &chainID, &feeRight, &status)
	if err != nil {
		t.Fatal(err)
	}
	return fmt.Sprintf("%d/%d/%d/%d/%s/%s/%s", chains, epochs, claims, home, chainID, feeRight, status)
}

func TestChainLaunchRowsAreWrittenOnce(t *testing.T) {
	_, pool := canonicaltest.New(t)
	b := chainLaunchBatch(t, pool)
	if err := process(context.Background(), pool, b); err != nil {
		t.Fatal(err)
	}
	if got := chainLaunchState(t, pool); got != "1/1/1/7/777/9/active" {
		t.Fatalf("chain launch rows: %s", got)
	}
	launchedPool := "0x" + strings.Repeat("2", 40)
	market, err := store.New(pool).GetPoolByAddress(context.Background(), launchedPool)
	if err != nil || market == nil || market.HomeChain != 7 {
		t.Fatalf("launched market home chain: %#v, %v", market, err)
	}
	otherPool := "0x" + strings.Repeat("a", 40)
	_, err = pool.Exec(context.Background(), `INSERT INTO indexer.pools(pool_address,token_address,creator,optical,pool_id,created_at,created_block,tx_hash) VALUES($1,$2,$3,$4,$5,123456,101,$6)`, otherPool, "0x"+strings.Repeat("b", 40), "0x"+strings.Repeat("c", 40), otherPool, "0x"+strings.Repeat("d", 64), "0x"+strings.Repeat("e", 64))
	if err != nil {
		t.Fatal(err)
	}
	rootMarket, err := store.New(pool).GetPoolByAddress(context.Background(), otherPool)
	if err != nil || rootMarket == nil || rootMarket.HomeChain != 0 {
		t.Fatalf("root market home chain: %#v, %v", rootMarket, err)
	}
	var amount, supply, snapshot, claim string
	err = pool.QueryRow(context.Background(), `SELECT e.amount,e.eligible_supply,e.snapshot_block::text,c.amount FROM indexer.chain_revenue_epochs e JOIN indexer.chain_revenue_claims c USING(chain,epoch) WHERE e.chain=7`).Scan(&amount, &supply, &snapshot, &claim)
	if err != nil || amount != "300" || supply != "1000" || snapshot != "100" || claim != "100" {
		t.Fatalf("revenue fields: %s/%s/%s/%s: %v", amount, supply, snapshot, claim, err)
	}
}

func TestChainLaunchReplayOfABlockChangesNothing(t *testing.T) {
	_, pool := canonicaltest.New(t)
	b := chainLaunchBatch(t, pool)
	if err := process(context.Background(), pool, b); err != nil {
		t.Fatal(err)
	}
	before := chainLaunchState(t, pool)
	if err := process(context.Background(), pool, b); err == nil {
		t.Fatal("committed block was accepted twice")
	}
	if after := chainLaunchState(t, pool); after != before {
		t.Fatalf("replay changed rows: %s -> %s", before, after)
	}
}

func TestChainLaunchRetirementUpdatesTheRow(t *testing.T) {
	_, pool := canonicaltest.New(t)
	b := chainLaunchBatch(t, pool)
	ctx := context.Background()
	if err := process(ctx, pool, b); err != nil {
		t.Fatal(err)
	}
	a := chainLaunchTestABI(t)
	chain := common.BigToHash(big.NewInt(7))
	holder := common.HexToAddress("0x" + strings.Repeat("4", 40))
	record := chainLaunchRecord(t, a, "LaunchedChainRetired", 102, 0, []common.Hash{chain, common.BytesToHash(holder.Bytes())}, big.NewInt(5), big.NewInt(123457), big.NewInt(102))
	next := store.CanonicalBatch{ChainID: 125, StreamID: b.StreamID, Owner: b.Owner, Fence: b.Fence, BatchID: "batch102", PreviousHeight: 101, Blocks: []store.BlockCompleteness{{Height: 102, Hash: record.BlockHash, RawCount: 1, FetchComplete: true, RequiredDecodeComplete: true}}, Records: []store.RawRecord{record}}
	if err := processorFor(pool).ProcessCanonicalBatch(ctx, next, map[uint64]int64{102: 123457}, nil); err != nil {
		t.Fatal(err)
	}
	var status string
	var block int64
	if err := pool.QueryRow(ctx, `SELECT status,retired_block FROM indexer.launched_chains WHERE chain=7`).Scan(&status, &block); err != nil || status != "retired" || block != 102 {
		t.Fatalf("retirement: %s/%d: %v", status, block, err)
	}
}
