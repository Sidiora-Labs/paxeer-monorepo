CREATE TABLE stats.canonical_multihop_checkpoints (
 source_stream text NOT NULL CHECK(length(source_stream) BETWEEN 1 AND 128),
 transaction_hash text NOT NULL CHECK(transaction_hash ~ '^0x[0-9a-f]{64}$'),
 log_index bigint NOT NULL CHECK(log_index BETWEEN 0 AND 4294967295),
 height bigint NOT NULL CHECK(height>=0),
 transaction_index bigint NOT NULL CHECK(transaction_index BETWEEN 0 AND 4294967295),
 block_hash text NOT NULL CHECK(block_hash ~ '^0x[0-9a-f]{64}$'),
 source_sha256 text NOT NULL CHECK(source_sha256 ~ '^[0-9a-f]{64}$'),
 swap_id text NOT NULL REFERENCES stats.cross_token_swaps(id),
 committed_at timestamptz NOT NULL DEFAULT clock_timestamp(),
 PRIMARY KEY(source_stream,transaction_hash,log_index),
 CHECK(swap_id=transaction_hash||'-'||log_index::text)
);
