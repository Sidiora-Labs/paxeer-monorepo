package store_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/Sidiora-Technologies/KindleLaunch/core/api/internal/store"
)

func TestDevelopmentRanksAndHolderRewardsUseIndexedEvents(t *testing.T) {
	st, db, _ := newStore(t)
	ctx := context.Background()
	const pool = "0x1111111111111111111111111111111111111111"
	const token = "0x2222222222222222222222222222222222222222"
	const creator = "0x3333333333333333333333333333333333333333"
	const trader = "0x4444444444444444444444444444444444444444"
	const poolID = "0xaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	const txHash = "0xbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
	for _, statement := range []string{
		`CREATE TABLE indexer.swaps(pool_id text,sender text,is_buy boolean,amount_in text,amount_out text,fee text,block_timestamp bigint)`,
		`INSERT INTO indexer.swaps VALUES('` + poolID + `','` + trader + `',true,'100','40','3',1000),('` + poolID + `','` + trader + `',false,'40','150','2',1001)`,
		`INSERT INTO indexer.pools(pool_address,token_address,creator,optical,pool_id,created_at,created_block,tx_hash) VALUES('` + pool + `','` + token + `','` + creator + `','` + creator + `','` + poolID + `',900,42,'` + txHash + `')`,
		`INSERT INTO stats.pool_stats(pool_address,token_address,market_cap,created_at,updated_at) VALUES('` + pool + `','` + token + `','9000',900,1001)`,
		`INSERT INTO indexer.fee_distributions VALUES('dist-1','` + poolID + `',1,2,'30','` + trader + `',43,1001,'` + txHash + `',0)`,
		`INSERT INTO indexer.airdrop_claims VALUES('claim-1','` + poolID + `','` + pool + `',1,'` + trader + `','30','1','Claimed',43,1001,'` + txHash + `',1)`,
	} {
		if _, err := db.Exec(ctx, statement); err != nil {
			t.Fatal(err)
		}
	}
	watermark, err := store.DevelopmentWatermark(store.DevelopmentHead{Height: 43, UpdatedAt: time.Unix(1001, 0)}, "0xaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa")
	if err != nil || watermark.Height != "43" || watermark.ProjectionRevision != "43" {
		t.Fatalf("development watermark: %+v, %v", watermark, err)
	}
	for _, tc := range []struct {
		category, wallet, metric string
	}{
		{"trader", trader, "50"},
		{"creator", creator, "3"},
		{"volume", trader, "250"},
	} {
		got, err := st.DevelopmentLeaderboard(ctx, tc.category, "24h", 1100, watermark)
		if err != nil || len(got.Entries) != 1 || got.Entries[0].Wallet != tc.wallet || got.Entries[0].MetricRaw != tc.metric || got.Revision != "43" {
			t.Fatalf("%s leaderboard: %+v, %v", tc.category, got, err)
		}
	}
	if _, err := st.DevelopmentLeaderboard(ctx, "unknown", "24h", 1100, watermark); !errors.Is(err, store.ErrAccountingNotReady) {
		t.Fatalf("unknown leaderboard category: %v", err)
	}
	stats, err := st.DevelopmentHolderRewardsStats(ctx, watermark)
	if err != nil || stats.CumulativeDistributedRaw != "30" || stats.WalletsRewarded != 1 || stats.Watermark.Height != "43" {
		t.Fatalf("holder reward stats: %+v, %v", stats, err)
	}
	for _, order := range []string{"rewards", "mcap", "holders", "age"} {
		got, err := st.DevelopmentHolderRewardPools(ctx, order, watermark)
		if err != nil || len(got.Items) != 1 || got.Items[0].Pool != pool || got.Items[0].DistributedRaw != "30" {
			t.Fatalf("%s reward pools: %+v, %v", order, got, err)
		}
	}
	if _, err := st.DevelopmentHolderRewardPools(ctx, "unknown", watermark); !errors.Is(err, store.ErrDataIntegrity) {
		t.Fatalf("unknown reward ordering: %v", err)
	}
	if _, err := db.Exec(ctx, `CREATE TABLE indexer.indexer_committed_heads_v2(chain_id integer,height bigint,block_hash text,integrity text);
		INSERT INTO indexer.indexer_committed_heads_v2 VALUES(125,43,'0xaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa','healthy')`); err != nil {
		t.Fatal(err)
	}
	committed, err := st.HolderRewardsStats(ctx)
	if err != nil || committed.CumulativeDistributedRaw != "30" || committed.WalletsRewarded != 1 || committed.Watermark.Height != "43" || committed.Watermark.Integrity != "healthy" {
		t.Fatalf("committed reward stats: %+v, %v", committed, err)
	}
	for _, order := range []string{"rewards", "mcap", "holders", "age"} {
		result, err := st.HolderRewardPools(ctx, order)
		if err != nil || len(result.Items) != 1 || result.Items[0].Pool != pool || result.Items[0].DistributedRaw != "30" || result.Watermark.Height != "43" {
			t.Fatalf("committed reward pools %s: %+v, %v", order, result, err)
		}
	}
	if _, err := db.Exec(ctx, `UPDATE indexer.indexer_committed_heads_v2 SET integrity='quarantined' WHERE chain_id=125`); err != nil {
		t.Fatal(err)
	}
	if _, err := st.HolderRewardsStats(ctx); !errors.Is(err, store.ErrWatermarkQuarantined) {
		t.Fatalf("quarantined head accepted: %v", err)
	}
	if _, err := db.Exec(ctx, `UPDATE indexer.swaps SET sender='invalid' WHERE is_buy=true`); err != nil {
		t.Fatal(err)
	}
	if _, err := st.DevelopmentLeaderboard(ctx, "trader", "24h", 1100, watermark); !errors.Is(err, store.ErrDataIntegrity) {
		t.Fatalf("invalid trader attribution accepted: %v", err)
	}
	if _, err := db.Exec(ctx, `UPDATE indexer.swaps SET sender=$1 WHERE is_buy=true`, trader); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(ctx, `UPDATE indexer.pools SET creator='invalid' WHERE pool_address=$1`, pool); err != nil {
		t.Fatal(err)
	}
	if _, err := st.DevelopmentLeaderboard(ctx, "creator", "24h", 1100, watermark); !errors.Is(err, store.ErrDataIntegrity) {
		t.Fatalf("invalid creator attribution accepted: %v", err)
	}
	if _, err := db.Exec(ctx, `UPDATE indexer.pools SET creator=$1 WHERE pool_address=$2`, creator, pool); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(ctx, `UPDATE indexer.swaps SET amount_in='invalid' WHERE is_buy=true`); err != nil {
		t.Fatal(err)
	}
	partial, err := st.DevelopmentLeaderboard(ctx, "volume", "24h", 1100, watermark)
	if err != nil || partial.Quality != "partial" || len(partial.Entries) != 1 || partial.Entries[0].Quality != "partial" {
		t.Fatalf("incomplete volume attribution: %+v, %v", partial, err)
	}
	clocked, err := st.DevelopmentLeaderboard(ctx, "volume", "24h", 0, watermark)
	if err != nil || clocked.WindowEnd <= 1100 || len(clocked.Entries) != 0 {
		t.Fatalf("database-clock leaderboard: %+v %v", clocked, err)
	}
	if _, err := db.Exec(ctx, `INSERT INTO indexer.swaps VALUES($1,$2,true,'100','40','3',1000),($1,$3,true,'100','40','3',1000)`, poolID, creator, pool); err != nil {
		t.Fatal(err)
	}
	tied, err := st.DevelopmentLeaderboard(ctx, "trader", "24h", 1100, watermark)
	if err != nil || len(tied.Entries) != 3 || tied.Entries[1].Wallet != pool || tied.Entries[2].Wallet != creator || tied.Entries[1].MetricSigned != "-100" {
		t.Fatalf("tied negative traders: %+v %v", tied, err)
	}
	if _, err := db.Exec(ctx, `DROP TABLE indexer.swaps`); err != nil {
		t.Fatal(err)
	}
	if _, err := st.DevelopmentLeaderboard(ctx, "volume", "24h", 1100, watermark); err == nil {
		t.Fatal("missing swaps source accepted")
	}

}
