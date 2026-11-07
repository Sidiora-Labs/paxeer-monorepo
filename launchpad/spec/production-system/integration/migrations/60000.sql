-- Coordinator proposal: register under indexer isolated migration history.
CREATE TABLE indexer.indexer_ingest_leases (
 chain_id integer NOT NULL, stream_id text NOT NULL, owner_instance text NOT NULL,
 fence_token bigint NOT NULL CHECK(fence_token>0), expires_at timestamptz NOT NULL,
 last_heartbeat_at timestamptz NOT NULL DEFAULT clock_timestamp(), PRIMARY KEY(chain_id,stream_id));
CREATE TABLE indexer.indexer_ingest_watermarks (
 chain_id integer NOT NULL, stream_id text NOT NULL, raw_complete_height bigint NOT NULL,
 typed_complete_height bigint NOT NULL, PRIMARY KEY(chain_id,stream_id));
CREATE TABLE indexer.indexer_raw_logs (
 chain_id integer NOT NULL, transaction_hash text NOT NULL, log_index bigint NOT NULL CHECK(log_index BETWEEN 0 AND 4294967295),
 height bigint NOT NULL CHECK(height>=0), transaction_index bigint NOT NULL CHECK(transaction_index BETWEEN 0 AND 4294967295),
 block_hash text NOT NULL, emitter text NOT NULL, topics jsonb NOT NULL, data bytea NOT NULL,
 PRIMARY KEY(chain_id,transaction_hash,log_index));
CREATE INDEX indexer_raw_canonical_order ON indexer.indexer_raw_logs(chain_id,height,transaction_index,log_index);
CREATE TABLE indexer.indexer_block_completeness (
 chain_id integer NOT NULL, stream_id text NOT NULL, height bigint NOT NULL,
 block_hash text NOT NULL, raw_count integer NOT NULL, fetch_complete boolean NOT NULL,
 required_decode_complete boolean NOT NULL, completed_at timestamptz NOT NULL DEFAULT clock_timestamp(),
 PRIMARY KEY(chain_id,stream_id,height));
CREATE TABLE indexer.indexer_ingest_outbox (
 chain_id integer NOT NULL, transaction_hash text NOT NULL, log_index bigint NOT NULL,
 event_kind text NOT NULL, payload jsonb NOT NULL, created_at timestamptz NOT NULL DEFAULT clock_timestamp(),
 PRIMARY KEY(chain_id,transaction_hash,log_index,event_kind),
 FOREIGN KEY(chain_id,transaction_hash,log_index) REFERENCES indexer.indexer_raw_logs(chain_id,transaction_hash,log_index));
CREATE TABLE indexer.indexer_ingest_commits (
 chain_id integer NOT NULL, stream_id text NOT NULL, batch_id text NOT NULL,
 previous_height bigint NOT NULL,last_complete_height bigint NOT NULL,block_hash text NOT NULL,
 record_count integer NOT NULL,outbox_count integer NOT NULL,commit_time timestamptz NOT NULL DEFAULT clock_timestamp(),
 PRIMARY KEY(chain_id,stream_id,batch_id));
-- Runtime role must have SELECT/INSERT only on immutable raw and completeness,
-- no UPDATE/DELETE. Coordinator assigns roles and shared outbox adapter.

CREATE TABLE indexer.indexer_accounting_fills(source_fill_id text PRIMARY KEY,source_fact_digest text NOT NULL,chain_id integer NOT NULL,transaction_hash text NOT NULL,log_index bigint NOT NULL CHECK(log_index BETWEEN 0 AND 4294967295),pool_address text NOT NULL,payload jsonb NOT NULL,UNIQUE(chain_id,transaction_hash,log_index),FOREIGN KEY(chain_id,transaction_hash,log_index) REFERENCES indexer.indexer_raw_logs(chain_id,transaction_hash,log_index));
CREATE VIEW indexer.indexer_accounting_fills_v1 AS SELECT source_fill_id,source_fact_digest,chain_id,transaction_hash,log_index,pool_address,payload FROM indexer.indexer_accounting_fills;
CREATE VIEW indexer.indexer_accounting_publications_v1 AS SELECT payload->>'publicationId' AS publication_id,payload FROM indexer.indexer_ingest_outbox WHERE event_kind IN ('accounting.market.v1','accounting.inventory.v1');
