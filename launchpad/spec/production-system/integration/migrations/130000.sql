
CREATE TABLE "stats".canonical_blocks (height numeric(78,0) PRIMARY KEY,block_hash text NOT NULL UNIQUE,block_timestamp bigint NOT NULL);
CREATE TABLE "stats".canonical_transactions (transaction_hash text PRIMARY KEY,height numeric(78,0) NOT NULL REFERENCES "stats".canonical_blocks(height),transaction_index numeric(78,0) NOT NULL, UNIQUE(height,transaction_index));
CREATE TABLE "stats".canonical_positions (height numeric(78,0) NOT NULL,transaction_index numeric(78,0) NOT NULL,log_index numeric(78,0) NOT NULL,transaction_hash text NOT NULL REFERENCES "stats".canonical_transactions(transaction_hash),digest text NOT NULL,PRIMARY KEY(height,transaction_index,log_index),UNIQUE(transaction_hash,log_index));
CREATE TABLE "stats".canonical_pool_assets (pool text PRIMARY KEY,usdl text NOT NULL,token text NOT NULL,token_decimals integer NOT NULL);
CREATE TABLE "stats".inbox (
 chain_id text NOT NULL CHECK(chain_id='125'),transaction_hash text NOT NULL,log_index numeric(78,0) NOT NULL,
 consumer_version text NOT NULL,digest text NOT NULL,revision bigint,
 PRIMARY KEY(chain_id,transaction_hash,log_index,consumer_version));
CREATE TABLE "stats".trade_facts (
 event_id text NOT NULL,projection_version text NOT NULL,pool text NOT NULL,actor text NOT NULL,
 height numeric(78,0) NOT NULL,transaction_index numeric(78,0) NOT NULL,log_index numeric(78,0) NOT NULL,
 block_timestamp bigint NOT NULL CHECK(block_timestamp>=0),volume numeric(78,0) NOT NULL CHECK(volume>=0),
 fee numeric(78,0) NOT NULL CHECK(fee>=0),fee_asset text NOT NULL,fee_decimals integer NOT NULL,
 disposition text NOT NULL,side text NOT NULL,payload jsonb NOT NULL,revision bigint NOT NULL,
 PRIMARY KEY(event_id,projection_version));
CREATE INDEX ON "stats".trade_facts (projection_version,pool,height DESC,transaction_index DESC,log_index DESC);
CREATE TABLE "stats".minute_buckets (
 chain_id text NOT NULL CHECK(chain_id='125'),pool text NOT NULL,interval_seconds integer NOT NULL CHECK(interval_seconds=60),
 start bigint NOT NULL,projection_version text NOT NULL,volume numeric(156,0) NOT NULL CHECK(volume>=0),
 buys bigint NOT NULL,sells bigint NOT NULL,revision bigint NOT NULL,
 close_height numeric(78,0) NOT NULL,close_transaction_index numeric(78,0) NOT NULL,close_log_index numeric(78,0) NOT NULL,
 close_payload jsonb NOT NULL,
 PRIMARY KEY(chain_id,pool,interval_seconds,start,projection_version));
CREATE TABLE "stats".fee_groups (
 chain_id text NOT NULL CHECK(chain_id='125'),pool text NOT NULL,start bigint NOT NULL,projection_version text NOT NULL,
 asset text NOT NULL,decimals integer NOT NULL,fee_type text NOT NULL,disposition text NOT NULL,
 raw numeric(156,0) NOT NULL CHECK(raw>=0),revision bigint NOT NULL,
 PRIMARY KEY(chain_id,pool,start,projection_version,asset,decimals,fee_type,disposition));
CREATE TABLE "stats".outbox (
 event_id text NOT NULL,projection_version text NOT NULL,revision bigint NOT NULL,payload jsonb NOT NULL,
 attempts integer NOT NULL DEFAULT 0,published boolean NOT NULL DEFAULT false,
 PRIMARY KEY(event_id,projection_version));
CREATE TABLE "stats".accounting_pool_fences (pool text PRIMARY KEY,frozen boolean NOT NULL DEFAULT false,evidence_digest text);
CREATE TABLE "stats".accounting_market_commits (
 publication_id text PRIMARY KEY,source_fill_id text NOT NULL,source_fact_digest text NOT NULL,
 projection_version text NOT NULL,pool text NOT NULL,bucket_start bigint NOT NULL,
 position jsonb NOT NULL,fill jsonb NOT NULL,sequence bigint NOT NULL CHECK(sequence>0),revision bigint NOT NULL,
 UNIQUE(source_fill_id,projection_version),UNIQUE(projection_version,pool,sequence));
CREATE TABLE "stats".accounting_inventory_facets (
 publication_id text PRIMARY KEY,source_fill_id text NOT NULL,source_fact_digest text NOT NULL,
 projection_version text NOT NULL,partition_key text NOT NULL,position jsonb NOT NULL,payload jsonb NOT NULL);
CREATE VIEW "stats"."accounting_market_commits_v1" AS SELECT source_fill_id,source_fact_digest,projection_version,pool,fill,sequence FROM "stats"."accounting_market_commits";

CREATE VIEW "stats"."accounting_pool_fences_v1" AS SELECT pool,frozen FROM "stats"."accounting_pool_fences";
