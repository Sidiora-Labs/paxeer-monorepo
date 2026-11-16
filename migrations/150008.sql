SET LOCAL search_path = "stats", pg_catalog;

CREATE TABLE "stats"."analytics_progress"(
    pool text PRIMARY KEY,
    height numeric(78,0) NOT NULL,
    transaction_index numeric(78,0) NOT NULL,
    log_index numeric(78,0) NOT NULL,
    block_hash text NOT NULL,
    revision numeric(78,0) NOT NULL,
    observed_at text NOT NULL
);

CREATE TABLE "stats"."analytics_trades"(
    source_id text PRIMARY KEY,
    pool text NOT NULL,
    height numeric(78,0) NOT NULL,
    transaction_index numeric(78,0) NOT NULL,
    log_index numeric(78,0) NOT NULL,
    block_hash text NOT NULL,
    event_time bigint NOT NULL,
    side text NOT NULL CHECK(side IN ('buy','sell')),
    usdl_raw numeric(78,0) NOT NULL CHECK(usdl_raw>=0),
    digest text NOT NULL,
    UNIQUE(pool,height,transaction_index,log_index)
);
CREATE INDEX analytics_trades_pool_event_time_idx ON "stats"."analytics_trades"(pool,event_time);

CREATE TABLE "stats"."analytics_outbox"(
    id bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    pool text NOT NULL,
    model text NOT NULL,
    revision numeric(78,0) NOT NULL,
    payload jsonb NOT NULL,
    created_at timestamptz NOT NULL DEFAULT clock_timestamp(),
    UNIQUE(pool,model,revision)
);
CREATE INDEX analytics_outbox_pool_id_idx ON "stats"."analytics_outbox"(pool,id);

CREATE TABLE "stats"."analytics_risk_inputs"(
    pool text PRIMARY KEY,
    height numeric(78,0) NOT NULL,
    block_hash text NOT NULL,
    payload jsonb NOT NULL
);

CREATE TABLE "stats"."operation_ledger"(
    actor text NOT NULL,
    action text NOT NULL,
    nonce text NOT NULL,
    envelope_digest text NOT NULL,
    body_digest text NOT NULL,
    request_id uuid NOT NULL,
    response bytea NOT NULL,
    response_digest text NOT NULL,
    created_at timestamptz NOT NULL DEFAULT clock_timestamp(),
    retain_until timestamptz NOT NULL DEFAULT clock_timestamp()+interval '24 hours',
    PRIMARY KEY(actor,action,nonce),
    CHECK(retain_until>=created_at+interval '24 hours')
);
CREATE TABLE "stats"."security_audit_outbox"(
    id bigserial PRIMARY KEY,
    request_id uuid NOT NULL,
    actor_pseudonym text NOT NULL,
    action text NOT NULL,
    result text NOT NULL,
    policy_version text NOT NULL,
    request_digest text NOT NULL,
    created_at timestamptz NOT NULL DEFAULT clock_timestamp(),
    UNIQUE(request_id,action)
);

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

CREATE TABLE stats.public_stats_sync_baselines (
 projection_version text NOT NULL,pool text NOT NULL,checkpoint_key text NOT NULL,
 events jsonb NOT NULL CHECK(jsonb_typeof(events)='array' AND jsonb_array_length(events)<=256),
 PRIMARY KEY(projection_version,pool));

CREATE TABLE stats.delivery_consumer_watermarks (
 consumer_id text NOT NULL,generation text NOT NULL,partition_kind text NOT NULL,partition_key text NOT NULL,
 broker_stream text NOT NULL,broker_sequence bigint NOT NULL CHECK(broker_sequence>0),
 source_order jsonb NOT NULL,position_sequence bigint NOT NULL CHECK(position_sequence>0),
 event_id text NOT NULL,envelope_hash text NOT NULL,committed_at timestamptz NOT NULL DEFAULT clock_timestamp(),
 PRIMARY KEY(consumer_id,generation,partition_kind,partition_key)
);
