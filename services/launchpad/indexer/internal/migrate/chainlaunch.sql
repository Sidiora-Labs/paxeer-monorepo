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
