package store_test

import (
	"context"
	"errors"
	"os"
	"testing"

	"github.com/Sidiora-Technologies/KindleLaunch/core/api/internal/store"
)

func TestChainRegistryAndRevenuePages(t *testing.T) {
	st, db, _ := newStore(t)
	ctx := context.Background()
	schema, err := os.ReadFile("../../../indexer/internal/migrate/chainlaunch.sql")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(ctx, string(schema)); err != nil {
		t.Fatal(err)
	}
	const pool = "0x1111111111111111111111111111111111111111"
	const token = "0x2222222222222222222222222222222222222222"
	const creator = "0x3333333333333333333333333333333333333333"
	const holder = "0x4444444444444444444444444444444444444444"
	const txHash = "0xaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	if _, err := db.Exec(ctx, `INSERT INTO indexer.pools(pool_address,token_address,creator,optical,pool_id,created_at,created_block,tx_hash,home_chain)
		VALUES($1,$2,$3,$3,$4,100,40,$4,7)`, pool, token, creator, txHash); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(ctx, `INSERT INTO indexer.launched_chains(chain,chain_id,pool,token,fee_right,creator,block_number,tx_hash,status)
		VALUES(7,125,$1,$2,'9',$3,42,$4,'active')`, pool, token, creator, txHash); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(ctx, `INSERT INTO indexer.indexer_canonical_creations(token,pool,creator,factory,optical,nft_id,height,block_hash,transaction_hash,transaction_index,log_index,block_timestamp,provenance,payload)
		VALUES($1,$2,$3,$3,$3,9,42,$4,$5,0,0,1000,'{}','{}')`, token, pool, creator, txHash, txHash); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(ctx, `INSERT INTO indexer.pool_state_snapshots(id,pool_id,virtual_reserve,real_reserve,token_reserve,price,block_number,block_timestamp,tx_hash,log_index)
		VALUES('state-1',$1,'100','80','50','2',42,1000,$2,0)`, txHash, txHash); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(ctx, `INSERT INTO indexer.chain_revenue_epochs(chain,epoch,amount,snapshot_block,eligible_supply,opened_block)
		VALUES(7,3,'100',42,'1000',42)`); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(ctx, `INSERT INTO indexer.chain_revenue_claims(chain,epoch,holder,amount,block_number,tx_hash)
		VALUES(7,3,$1,'25',43,$2)`, holder, txHash); err != nil {
		t.Fatal(err)
	}
	chains, err := st.LaunchedChains(ctx, 10, 0)
	if err != nil || chains.Total != 1 || chains.SnapshotID != "7:1" || len(chains.Items) != 1 || chains.Items[0].Pool != pool || chains.Items[0].NFTID != "9" {
		t.Fatalf("launched chains: %+v, %v", chains, err)
	}
	chain, err := st.LaunchedChain(ctx, 7)
	if err != nil || chain == nil || chain.ChainID != "125" || chain.Creator != creator {
		t.Fatalf("launched chain: %+v, %v", chain, err)
	}
	missing, err := st.LaunchedChain(ctx, 8)
	if err != nil || missing != nil {
		t.Fatalf("missing chain: %+v, %v", missing, err)
	}
	empty, err := st.LaunchedChains(ctx, 10, 2)
	if err != nil || empty.Total != 1 || len(empty.Items) != 0 || empty.SnapshotID != chains.SnapshotID {
		t.Fatalf("chain page beyond end: %+v, %v", empty, err)
	}
	if _, err := st.LaunchedChains(ctx, 0, 0); !errors.Is(err, store.ErrDataIntegrity) {
		t.Fatalf("invalid chain page: %v", err)
	}
	page, err := st.ChainRevenue(ctx, 7, holder, 10, 0)
	if err != nil || page.Total != 1 || page.SnapshotID != "3:1" || len(page.Epochs) != 1 || page.Epochs[0].EpochAmount != "100" || len(page.Claims) != 1 || page.Claims[0].Amount != "25" {
		t.Fatalf("chain revenue: %+v, %v", page, err)
	}
	emptyRevenue, err := st.ChainRevenue(ctx, 7, holder, 10, 2)
	if err != nil || emptyRevenue.Total != 1 || len(emptyRevenue.Epochs) != 0 || len(emptyRevenue.Claims) != 0 {
		t.Fatalf("revenue page beyond end: %+v, %v", emptyRevenue, err)
	}
	if _, err := st.ChainRevenue(ctx, 7, holder, 101, 0); !errors.Is(err, store.ErrDataIntegrity) {
		t.Fatalf("invalid revenue page: %v", err)
	}
	homes, err := st.PoolHomeChains(ctx, []string{pool})
	if err != nil || homes[pool] != "7" {
		t.Fatalf("pool home chains: %+v, %v", homes, err)
	}
	if _, err := st.PoolHomeChains(ctx, nil); !errors.Is(err, store.ErrDataIntegrity) {
		t.Fatalf("empty pool home selection: %v", err)
	}
	if _, err := st.PoolHomeChains(ctx, []string{token}); !errors.Is(err, store.ErrDataIntegrity) {
		t.Fatalf("unregistered pool home: %v", err)
	}
	registry, err := st.PoolRegistry(ctx, []string{pool})
	if err != nil || registry[pool].Token != token || registry[pool].Position.Height != "42" || registry[pool].Position.BlockHash != txHash {
		t.Fatalf("canonical pool registry: %+v, %v", registry, err)
	}
	reserves, err := st.PoolReserves(ctx, []string{pool})
	if err != nil || reserves[pool].VirtualUsdl != "100" || reserves[pool].RealUsdl != "80" || reserves[pool].Token != "50" || reserves[pool].Height != 42 {
		t.Fatalf("pool reserves: %+v, %v", reserves, err)
	}
	if _, err := st.PoolRegistry(ctx, nil); !errors.Is(err, store.ErrDataIntegrity) {
		t.Fatalf("empty pool registry selection: %v", err)
	}
	if _, err := st.PoolReserves(ctx, nil); !errors.Is(err, store.ErrDataIntegrity) {
		t.Fatalf("empty reserve selection: %v", err)
	}
	for _, check := range []struct {
		name, change, restore string
		read                  func() error
	}{
		{"chain fee right", `UPDATE indexer.launched_chains SET fee_right='invalid' WHERE chain=7`, `UPDATE indexer.launched_chains SET fee_right='9' WHERE chain=7`, func() error { _, err := st.LaunchedChains(ctx, 10, 0); return err }},
		{"chain creator", `UPDATE indexer.launched_chains SET creator='invalid' WHERE chain=7`, `UPDATE indexer.launched_chains SET creator='` + creator + `' WHERE chain=7`, func() error { _, err := st.LaunchedChain(ctx, 7); return err }},
		{"chain identifier", `UPDATE indexer.launched_chains SET chain_id=0 WHERE chain=7`, `UPDATE indexer.launched_chains SET chain_id=125 WHERE chain=7`, func() error { _, err := st.LaunchedChains(ctx, 10, 0); return err }},
		{"epoch amount", `UPDATE indexer.chain_revenue_epochs SET amount='invalid' WHERE chain=7`, `UPDATE indexer.chain_revenue_epochs SET amount='100' WHERE chain=7`, func() error { _, err := st.ChainRevenue(ctx, 7, holder, 10, 0); return err }},
		{"eligible supply", `UPDATE indexer.chain_revenue_epochs SET eligible_supply='invalid' WHERE chain=7`, `UPDATE indexer.chain_revenue_epochs SET eligible_supply='1000' WHERE chain=7`, func() error { _, err := st.ChainRevenue(ctx, 7, holder, 10, 0); return err }},
		{"claim amount", `UPDATE indexer.chain_revenue_claims SET amount='invalid' WHERE chain=7`, `UPDATE indexer.chain_revenue_claims SET amount='25' WHERE chain=7`, func() error { _, err := st.ChainRevenue(ctx, 7, holder, 10, 0); return err }},
		{"claim block", `UPDATE indexer.chain_revenue_claims SET block_number=-1 WHERE chain=7`, `UPDATE indexer.chain_revenue_claims SET block_number=43 WHERE chain=7`, func() error { _, err := st.ChainRevenue(ctx, 7, holder, 10, 0); return err }},
		{"pool home", `UPDATE indexer.pools SET home_chain=-1 WHERE pool_address='` + pool + `'`, `UPDATE indexer.pools SET home_chain=7 WHERE pool_address='` + pool + `'`, func() error { _, err := st.PoolHomeChains(ctx, []string{pool}); return err }},
		{"virtual reserve", `UPDATE indexer.pool_state_snapshots SET virtual_reserve='-1' WHERE id='state-1'`, `UPDATE indexer.pool_state_snapshots SET virtual_reserve='100' WHERE id='state-1'`, func() error { _, err := st.PoolReserves(ctx, []string{pool}); return err }},
		{"token reserve", `UPDATE indexer.pool_state_snapshots SET token_reserve='-1' WHERE id='state-1'`, `UPDATE indexer.pool_state_snapshots SET token_reserve='50' WHERE id='state-1'`, func() error { _, err := st.PoolReserves(ctx, []string{pool}); return err }},
		{"registry position", `UPDATE indexer.indexer_canonical_creations SET block_hash='invalid'`, `UPDATE indexer.indexer_canonical_creations SET block_hash='` + txHash + `'`, func() error { _, err := st.PoolRegistry(ctx, []string{pool}); return err }},
		{"registry creator", `UPDATE indexer.indexer_canonical_creations SET creator='invalid'`, `UPDATE indexer.indexer_canonical_creations SET creator='` + creator + `'`, func() error { _, err := st.PoolRegistry(ctx, []string{pool}); return err }},
		{"registry metadata", `UPDATE indexer.indexer_canonical_creations SET payload='{"metadata":{"totalSupply":"invalid"}}'`, `UPDATE indexer.indexer_canonical_creations SET payload='{}'`, func() error { _, err := st.PoolRegistry(ctx, []string{pool}); return err }},
		{"pool reserve", `UPDATE indexer.pool_state_snapshots SET real_reserve='-1' WHERE id='state-1'`, `UPDATE indexer.pool_state_snapshots SET real_reserve='80' WHERE id='state-1'`, func() error { _, err := st.PoolReserves(ctx, []string{pool}); return err }},
		{"chain number", `UPDATE indexer.launched_chains SET chain=0 WHERE chain=7`, `UPDATE indexer.launched_chains SET chain=7 WHERE chain=0`, func() error { _, err := st.LaunchedChains(ctx, 10, 0); return err }},
		{"chain block", `UPDATE indexer.launched_chains SET block_number=-1 WHERE chain=7`, `UPDATE indexer.launched_chains SET block_number=42 WHERE chain=7`, func() error { _, err := st.LaunchedChains(ctx, 10, 0); return err }},
		{"chain status", `ALTER TABLE indexer.launched_chains DROP CONSTRAINT launched_chains_status_check; UPDATE indexer.launched_chains SET status='unknown' WHERE chain=7`, `UPDATE indexer.launched_chains SET status='active' WHERE chain=7`, func() error { _, err := st.LaunchedChains(ctx, 10, 0); return err }},
		{"chain token", `UPDATE indexer.launched_chains SET token='invalid' WHERE chain=7`, `UPDATE indexer.launched_chains SET token='` + token + `' WHERE chain=7`, func() error { _, err := st.LaunchedChains(ctx, 10, 0); return err }},
		{"chain pool", `UPDATE indexer.launched_chains SET pool='invalid' WHERE chain=7`, `UPDATE indexer.launched_chains SET pool='` + pool + `' WHERE chain=7`, func() error { _, err := st.LaunchedChains(ctx, 10, 0); return err }},
		{"epoch number", `UPDATE indexer.chain_revenue_epochs SET epoch=-1 WHERE chain=7`, `UPDATE indexer.chain_revenue_epochs SET epoch=3 WHERE chain=7`, func() error { _, err := st.ChainRevenue(ctx, 7, holder, 10, 0); return err }},
	} {
		t.Run(check.name, func(t *testing.T) {
			if _, err := db.Exec(ctx, check.change); err != nil {
				t.Fatal(err)
			}
			if err := check.read(); !errors.Is(err, store.ErrDataIntegrity) {
				t.Fatalf("invalid source accepted: %v", err)
			}
			if _, err := db.Exec(ctx, check.restore); err != nil {
				t.Fatal(err)
			}
		})
	}
	if _, err := db.Exec(ctx, `DROP TABLE indexer.chain_revenue_claims`); err != nil {
		t.Fatal(err)
	}
	if _, err := st.ChainRevenue(ctx, 7, holder, 10, 0); err == nil {
		t.Fatal("revenue page ignored unavailable claims")
	}
	if _, err := db.Exec(ctx, `DROP TABLE indexer.chain_revenue_epochs`); err != nil {
		t.Fatal(err)
	}
	if _, err := st.ChainRevenue(ctx, 7, holder, 10, 0); err == nil {
		t.Fatal("revenue page ignored unavailable epochs")
	}
	if _, err := db.Exec(ctx, `DROP TABLE indexer.launched_chains`); err != nil {
		t.Fatal(err)
	}
	if _, err := st.LaunchedChains(ctx, 10, 0); err == nil {
		t.Fatal("chain page ignored unavailable registry")
	}
	if _, err := st.LaunchedChain(ctx, 7); err == nil {
		t.Fatal("chain detail ignored unavailable registry")
	}
}
