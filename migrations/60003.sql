SET LOCAL search_path = indexer, pg_catalog;
ALTER TABLE indexer.pools ADD COLUMN IF NOT EXISTS home_chain bigint NOT NULL DEFAULT 0;

CREATE TABLE IF NOT EXISTS indexer.launched_chains (
    chain bigint PRIMARY KEY,
    chain_id numeric(20,0) NOT NULL,
    pool varchar(42) NOT NULL UNIQUE,
    token varchar(42) NOT NULL,
    fee_right text NOT NULL,
    creator varchar(42) NOT NULL,
    block_number bigint NOT NULL,
    tx_hash varchar(66) NOT NULL,
    status varchar(16) NOT NULL CHECK (status IN ('active', 'retired')),
    retired_block bigint
);

CREATE TABLE IF NOT EXISTS indexer.chain_revenue_epochs (
    chain bigint NOT NULL,
    epoch numeric(20,0) NOT NULL,
    amount text NOT NULL,
    snapshot_block numeric(20,0) NOT NULL,
    eligible_supply text NOT NULL,
    opened_block bigint NOT NULL,
    PRIMARY KEY (chain, epoch)
);

CREATE TABLE IF NOT EXISTS indexer.chain_revenue_claims (
    chain bigint NOT NULL,
    epoch numeric(20,0) NOT NULL,
    holder varchar(42) NOT NULL,
    amount text NOT NULL,
    block_number bigint NOT NULL,
    tx_hash varchar(66) NOT NULL,
    PRIMARY KEY (chain, epoch, holder)
);

CREATE VIEW indexer.launched_chains_v1 WITH (security_barrier=true) AS
 SELECT chain,chain_id,pool,token,fee_right,creator,block_number,status FROM indexer.launched_chains;
REVOKE ALL ON indexer.launched_chains_v1 FROM PUBLIC;

CREATE VIEW indexer.chain_revenue_epochs_v1 WITH (security_barrier=true) AS
 SELECT chain,epoch,amount,snapshot_block,eligible_supply FROM indexer.chain_revenue_epochs;
REVOKE ALL ON indexer.chain_revenue_epochs_v1 FROM PUBLIC;

CREATE VIEW indexer.chain_revenue_claims_v1 WITH (security_barrier=true) AS
 SELECT chain,epoch,holder,amount,block_number FROM indexer.chain_revenue_claims;
REVOKE ALL ON indexer.chain_revenue_claims_v1 FROM PUBLIC;

CREATE VIEW indexer.pools_read_v1 WITH (security_barrier=true) AS
 SELECT pool_address,pool_id,home_chain,token_address,created_at FROM indexer.pools;
REVOKE ALL ON indexer.pools_read_v1 FROM PUBLIC;

CREATE VIEW indexer.pool_state_snapshots_v1 WITH (security_barrier=true) AS
 SELECT pool_id,virtual_reserve,real_reserve,token_reserve,block_number,log_index FROM indexer.pool_state_snapshots;
REVOKE ALL ON indexer.pool_state_snapshots_v1 FROM PUBLIC;

CREATE VIEW indexer.fee_distributions_v1 WITH (security_barrier=true) AS
 SELECT strategy,amount,pool_id,block_number,log_index FROM indexer.fee_distributions;
REVOKE ALL ON indexer.fee_distributions_v1 FROM PUBLIC;

CREATE VIEW indexer.airdrop_claims_v1 WITH (security_barrier=true) AS
 SELECT holder,pool_id FROM indexer.airdrop_claims;
REVOKE ALL ON indexer.airdrop_claims_v1 FROM PUBLIC;

CREATE VIEW indexer.swap_progress_v1 WITH (security_barrier=true) AS
 SELECT block_number FROM indexer.swaps;
REVOKE ALL ON indexer.swap_progress_v1 FROM PUBLIC;

CREATE FUNCTION indexer.lock_ingest_lease_v1(wanted_stream text)
RETURNS TABLE(owner_instance text, fence_token bigint)
LANGUAGE sql SECURITY DEFINER SET search_path = pg_catalog, pg_temp AS $$
 SELECT l.owner_instance,l.fence_token FROM indexer.indexer_ingest_leases l
 WHERE l.chain_id=125 AND l.stream_id=wanted_stream AND l.expires_at>clock_timestamp()
 FOR SHARE
$$;
REVOKE ALL ON FUNCTION indexer.lock_ingest_lease_v1(text) FROM PUBLIC;
REVOKE ALL ON indexer.launched_chains, indexer.chain_revenue_epochs, indexer.chain_revenue_claims FROM PUBLIC;

CREATE VIEW indexer.fee_strategy_changes_v1 WITH (security_barrier=true) AS
 SELECT pool_id,block_number,log_index,new_strategy FROM indexer.fee_strategy_changes;
REVOKE ALL ON indexer.fee_strategy_changes_v1 FROM PUBLIC;
