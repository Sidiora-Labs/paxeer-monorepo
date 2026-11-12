package store_test

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/Sidiora-Technologies/KindleLaunch/core/api/internal/store"
	"github.com/redis/go-redis/v9"
)

func TestDevelopmentStoreReadsIndexedMarket(t *testing.T) {
	st, pool, rdb := newStore(t)
	ctx := context.Background()
	const address = "0x1111111111111111111111111111111111111111"
	const token = "0x2222222222222222222222222222222222222222"
	const creator = "0x3333333333333333333333333333333333333333"
	const holder = "0x4444444444444444444444444444444444444444"
	const digest = "0xaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	if _, err := pool.Exec(ctx, `CREATE TABLE indexer.cursors(chain_id integer PRIMARY KEY,last_processed_block bigint,updated_at timestamptz DEFAULT now());
		CREATE TABLE indexer.swaps(pool_id varchar(66),pool_address varchar(42),amount_in text,fee text,is_buy boolean,block_number bigint,log_index integer);
		CREATE SCHEMA metadata;
		CREATE TABLE metadata.token_metadata(token_address text,name text,symbol text);
		CREATE SCHEMA users;
		CREATE TABLE users.user_profiles(wallet_address text,display_name text);
		INSERT INTO indexer.cursors(chain_id,last_processed_block) VALUES(125,42)`); err != nil {
		t.Fatal(err)
	}
	for _, statement := range []struct {
		query string
		args  []any
	}{
		{`INSERT INTO indexer.pools(pool_address,token_address,creator,optical,pool_id,created_at,created_block,tx_hash) VALUES($1,$2,$3,$4,$5,100,40,$5)`, []any{address, token, creator, "0x0000000000000000000000000000000000000000", digest}},
		{`INSERT INTO metadata.token_metadata VALUES($1,'Market Token','MKT')`, []any{token}},
		{`INSERT INTO users.user_profiles VALUES($1,'Wallet Holder')`, []any{holder}},
		{`INSERT INTO stats.pool_stats(pool_address,token_address,price_change_24h,created_at,updated_at) VALUES($1,$2,'125',100,200)`, []any{address, token}},
		{`INSERT INTO stats.pool_holders(pool_address,holder_address,balance,pct_of_supply,last_updated) VALUES($1,$2,'900','90',200)`, []any{address, holder}},
	} {
		if _, err := pool.Exec(ctx, statement.query, statement.args...); err != nil {
			t.Fatal(err)
		}
	}

	head, err := st.DevelopmentIndexerHead(ctx)
	if err != nil || head.Height != 42 {
		t.Fatalf("development head: %+v, %v", head, err)
	}
	indexed, err := st.DevelopmentPoolIndexed(ctx, address)
	if err != nil || !indexed {
		t.Fatalf("indexed pool: %v, %v", indexed, err)
	}
	indexedPools, err := st.DevelopmentPoolsIndexed(ctx, []string{address, "0x5555555555555555555555555555555555555555"})
	if err != nil || len(indexedPools) != 1 || !indexedPools[address] {
		t.Fatalf("indexed pools: %v, %v", indexedPools, err)
	}
	pools, err := st.DevelopmentPools(ctx, []string{address})
	if err != nil || len(pools) != 1 || pools[address].Token != token || pools[address].Creator != creator || pools[address].Reserves != nil {
		t.Fatalf("development pools: %v, %v", pools, err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO indexer.pool_state_snapshots(id,pool_id,virtual_reserve,real_reserve,token_reserve,price,block_number,block_timestamp,tx_hash,log_index) VALUES('state-1',$1,'1000000','500000','900000','1',41,100,$2,1)`, digest, digest); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO indexer.swaps(pool_id,pool_address,amount_in,fee,is_buy,block_number,log_index) VALUES($1,$2,'100000','300',true,42,1)`, digest, address); err != nil {
		t.Fatal(err)
	}
	withState, err := st.DevelopmentPools(ctx, []string{address})
	if err != nil || len(withState) != 1 || withState[address].Reserves == nil || withState[address].Reserves.VirtualUsdl != "1000000" || withState[address].Reserves.RealUsdl != "500000" || withState[address].StateEvent == nil || withState[address].StateEvent.Height != 41 || withState[address].FeeBps == nil || *withState[address].FeeBps != 30 {
		t.Fatalf("development pool state and fee: %+v, %v", withState, err)
	}
	if _, err := st.DevelopmentPools(ctx, nil); !errors.Is(err, store.ErrDataIntegrity) {
		t.Fatalf("empty pool selection: %v", err)
	}
	if _, err := st.DevelopmentPoolIndexed(ctx, strings.ToUpper(address)); !errors.Is(err, store.ErrDevelopmentScope) {
		t.Fatalf("noncanonical pool scope: %v", err)
	}

	holders, err := st.DevelopmentHolders(ctx, address, nil, "", "", 10)
	if err != nil || len(holders) != 1 || holders[0].Wallet != holder || holders[0].Balance != "900" {
		t.Fatalf("holders: %v, %v", holders, err)
	}
	holders, err = st.DevelopmentHolders(ctx, address, []string{holder}, "", "", 10)
	if err != nil || len(holders) != 0 {
		t.Fatalf("excluded holder: %v, %v", holders, err)
	}
	if _, err := st.DevelopmentHolders(ctx, address, nil, "01", "", 10); !errors.Is(err, store.ErrCursorInvalid) {
		t.Fatalf("invalid holder cursor: %v", err)
	}
	movers, total, err := st.DevelopmentPriceMovers(ctx, true, 10)
	if err != nil || total != 1 || len(movers) != 1 || movers[0].Pool != address || movers[0].ChangeBps != "125" {
		t.Fatalf("price movers: %v total=%d err=%v", movers, total, err)
	}
	losers, total, err := st.DevelopmentPriceMovers(ctx, false, 10)
	if err != nil || total != 0 || len(losers) != 0 {
		t.Fatalf("losers: %v total=%d err=%v", losers, total, err)
	}

	if err := rdb.ZAdd(ctx, "ranking:gainers", redis.Z{Score: 12, Member: address}).Err(); err != nil {
		t.Fatal(err)
	}
	ranked, total, err := st.RankingMembers(ctx, "gainers", 10)
	if err != nil || total != 1 || len(ranked) != 1 || ranked[0].PoolAddress != address || ranked[0].Rank != 1 {
		t.Fatalf("ranking members: %v total=%d err=%v", ranked, total, err)
	}
	if _, _, err := st.RankingMembers(ctx, "gainers", 0); !errors.Is(err, store.ErrDataIntegrity) {
		t.Fatalf("invalid ranking limit: %v", err)
	}

	for _, tc := range []struct {
		kind, match, key, want string
	}{
		{"pool", "exact_address", address, address},
		{"pool", "partial_address", "0x1111", address},
		{"token", "exact_address", token, address},
		{"profile", "exact_address", holder, holder},
		{"profile", "partial_address", "0x4444", holder},
		{"token", "text", "market", address},
		{"profile", "text", "wallet", holder},
	} {
		hits, err := st.DevelopmentSearch(ctx, tc.kind, tc.match, tc.key, "", 5)
		if err != nil || len(hits) != 1 || (hits[0].Pool != tc.want && hits[0].Wallet != tc.want) {
			t.Errorf("search %+v: %v, %v", tc, hits, err)
		}
	}
	if _, err := st.DevelopmentSearch(ctx, "pool", "text", "", "", 5); !errors.Is(err, store.ErrDevelopmentScope) {
		t.Fatalf("empty search: %v", err)
	}

	for _, args := range [][2]string{{"unknown", "text"}, {"pool", "unknown"}} {
		if _, err := st.DevelopmentSearch(ctx, args[0], args[1], "market", "", 5); !errors.Is(err, store.ErrDevelopmentScope) {
			t.Fatalf("invalid search mode: %v", err)
		}
	}
	continued, err := st.DevelopmentHolders(ctx, address, nil, "900", holder, 10)
	if err != nil || len(continued) != 0 {
		t.Fatalf("holder continuation: %v %v", continued, err)
	}
	if _, _, err := st.DevelopmentPriceMovers(ctx, true, 0); !errors.Is(err, store.ErrDataIntegrity) {
		t.Fatalf("mover limit: %v", err)
	}
	if _, err := st.DevelopmentHolders(ctx, address, nil, "", "", 0); !errors.Is(err, store.ErrDataIntegrity) {
		t.Fatalf("holder limit: %v", err)
	}
	if _, err := pool.Exec(ctx, `UPDATE stats.pool_holders SET balance='01' WHERE pool_address=$1`, address); err != nil {
		t.Fatal(err)
	}
	if _, err := st.DevelopmentHolders(ctx, address, nil, "", "", 10); !errors.Is(err, store.ErrDataIntegrity) {
		t.Fatalf("invalid balance: %v", err)
	}
	if _, err := pool.Exec(ctx, `UPDATE stats.pool_holders SET balance='900' WHERE pool_address=$1`, address); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `UPDATE users.user_profiles SET wallet_address='invalid'`); err != nil {
		t.Fatal(err)
	}
	if _, err := st.DevelopmentSearch(ctx, "profile", "text", "wallet", "", 5); !errors.Is(err, store.ErrDataIntegrity) {
		t.Fatalf("invalid profile address: %v", err)
	}
	if _, err := pool.Exec(ctx, `UPDATE users.user_profiles SET wallet_address=$1`, holder); err != nil {
		t.Fatal(err)
	}
	for _, check := range []struct {
		name, change, restore string
	}{
		{"registry token", `UPDATE indexer.pools SET token_address='invalid' WHERE pool_address='` + address + `'`, `UPDATE indexer.pools SET token_address='` + token + `' WHERE pool_address='` + address + `'`},
		{"registry optical", `UPDATE indexer.pools SET optical='invalid' WHERE pool_address='` + address + `'`, `UPDATE indexer.pools SET optical='0x0000000000000000000000000000000000000000' WHERE pool_address='` + address + `'`},
		{"registry identity", `UPDATE indexer.pools SET pool_id='invalid' WHERE pool_address='` + address + `'`, `UPDATE indexer.pools SET pool_id='` + digest + `' WHERE pool_address='` + address + `'`},
		{"registry time", `UPDATE indexer.pools SET created_at=0 WHERE pool_address='` + address + `'`, `UPDATE indexer.pools SET created_at=100 WHERE pool_address='` + address + `'`},
		{"virtual reserve", `UPDATE indexer.pool_state_snapshots SET virtual_reserve='invalid' WHERE id='state-1'`, `UPDATE indexer.pool_state_snapshots SET virtual_reserve='1000000' WHERE id='state-1'`},
		{"real reserve", `UPDATE indexer.pool_state_snapshots SET real_reserve='invalid' WHERE id='state-1'`, `UPDATE indexer.pool_state_snapshots SET real_reserve='500000' WHERE id='state-1'`},
		{"token reserve", `UPDATE indexer.pool_state_snapshots SET token_reserve='invalid' WHERE id='state-1'`, `UPDATE indexer.pool_state_snapshots SET token_reserve='900000' WHERE id='state-1'`},
		{"state position", `UPDATE indexer.pool_state_snapshots SET tx_hash='invalid' WHERE id='state-1'`, `UPDATE indexer.pool_state_snapshots SET tx_hash='` + digest + `' WHERE id='state-1'`},
	} {
		t.Run(check.name, func(t *testing.T) {
			if _, err := pool.Exec(ctx, check.change); err != nil {
				t.Fatal(err)
			}
			if _, err := st.DevelopmentPools(ctx, []string{address}); !errors.Is(err, store.ErrDataIntegrity) {
				t.Fatalf("invalid pool source accepted: %v", err)
			}
			if _, err := pool.Exec(ctx, check.restore); err != nil {
				t.Fatal(err)
			}
		})
	}
	if _, err := pool.Exec(ctx, `UPDATE stats.pool_holders SET holder_address='invalid' WHERE pool_address=$1`, address); err != nil {
		t.Fatal(err)
	}
	if _, err := st.DevelopmentHolders(ctx, address, nil, "", "", 10); !errors.Is(err, store.ErrDataIntegrity) {
		t.Fatalf("invalid holder source accepted: %v", err)
	}
	if _, err := pool.Exec(ctx, `UPDATE stats.pool_stats SET pool_address='invalid' WHERE pool_address=$1`, address); err != nil {
		t.Fatal(err)
	}
	if _, _, err := st.DevelopmentPriceMovers(ctx, true, 10); !errors.Is(err, store.ErrDataIntegrity) {
		t.Fatalf("invalid mover source accepted: %v", err)
	}
	if err := rdb.ZAdd(ctx, "ranking:gainers", redis.Z{Score: 20, Member: "invalid"}).Err(); err != nil {
		t.Fatal(err)
	}
	if _, _, err := st.RankingMembers(ctx, "gainers", 10); !errors.Is(err, store.ErrDataIntegrity) {
		t.Fatalf("invalid ranked member accepted: %v", err)
	}
}
