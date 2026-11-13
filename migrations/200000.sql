-- Proposed serial migration, not a registered migration. No production writes.
CREATE SCHEMA IF NOT EXISTS stats_search;

CREATE TABLE stats_search.generation (singleton boolean PRIMARY KEY CHECK(singleton), generation text NOT NULL, watermark bigint NOT NULL CHECK(watermark>0));
CREATE TABLE stats_search.documents (
 generation text NOT NULL, chain_id text NOT NULL CHECK(chain_id='125'),
 pool_address text NOT NULL CHECK(pool_address ~ '^0x[0-9a-f]{40}$'),
 token_address text NOT NULL CHECK(token_address ~ '^0x[0-9a-f]{40}$'),
 name text NOT NULL, name_key text NOT NULL, ticker_key text NOT NULL,
 creation_event_id text NOT NULL, revision bigint NOT NULL CHECK(revision>0),
 metadata_revision bigint NOT NULL CHECK(metadata_revision>0), visibility_revision bigint NOT NULL CHECK(visibility_revision>0),
 pool_status text NOT NULL CHECK(pool_status IN ('active','paused')),
 canonical boolean NOT NULL, quarantined boolean NOT NULL,
 PRIMARY KEY(generation,pool_address));
-- Current visibility is generation-independent, so rollback cannot resurrect tombstones.
CREATE TABLE stats_search.visibility (pool_address text PRIMARY KEY, revision bigint NOT NULL CHECK(revision>0), public boolean NOT NULL, tombstone boolean NOT NULL);
CREATE INDEX search_token_exact ON stats_search.documents(generation,token_address,pool_address);
CREATE INDEX search_name_trgm ON stats_search.documents USING gin(name_key public.gin_trgm_ops);
CREATE INDEX search_ticker_trgm ON stats_search.documents USING gin(ticker_key public.gin_trgm_ops);
CREATE INDEX search_pool_prefix ON stats_search.documents(pool_address text_pattern_ops);
CREATE INDEX search_token_prefix ON stats_search.documents(token_address text_pattern_ops);
