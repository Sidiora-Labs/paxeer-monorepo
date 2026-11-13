SET LOCAL search_path = "indexer", pg_catalog;
CREATE TABLE "indexer".delivery_webhook_cursors (
 target_id text NOT NULL, generation text NOT NULL, partition text NOT NULL,
 enqueued_sequence bigint NOT NULL DEFAULT 0, enqueued_event text NOT NULL DEFAULT '',
 committed_sequence bigint NOT NULL DEFAULT 0, committed_event text NOT NULL DEFAULT '',
 PRIMARY KEY(target_id,generation,partition), CHECK(committed_sequence>=0 AND enqueued_sequence>=committed_sequence)
);
CREATE TABLE "indexer".delivery_webhooks (
 target_id text NOT NULL, generation text NOT NULL, partition text NOT NULL, sequence bigint NOT NULL CHECK(sequence>0),
 position_identity text NOT NULL, event_id text NOT NULL REFERENCES "indexer".delivery_outbox(event_id), predecessor text NOT NULL,
 payload_hash text NOT NULL, wire_hash text NOT NULL, idempotency_key text NOT NULL,
 created_at timestamptz NOT NULL DEFAULT clock_timestamp(), state text NOT NULL DEFAULT 'pending' CHECK(state IN ('pending','delivered','quarantined')),
 attempts integer NOT NULL DEFAULT 0, allowed_attempts integer NOT NULL CHECK(allowed_attempts>0), available_at timestamptz NOT NULL DEFAULT clock_timestamp(),
 lease_expiry_count bigint NOT NULL DEFAULT 0, claimed_at timestamptz, acknowledgement_latency_ms bigint, owner text, fence bigint NOT NULL DEFAULT 0, lease_until timestamptz,
 response_status integer, failure_class text, delivered_at timestamptz,
 PRIMARY KEY(target_id,generation,partition,sequence), UNIQUE(target_id,generation,partition,event_id),
 FOREIGN KEY(target_id,generation,partition) REFERENCES "indexer".delivery_webhook_cursors
);
CREATE INDEX delivery_webhook_pending ON "indexer".delivery_webhooks(target_id,available_at) WHERE state='pending';
CREATE FUNCTION "indexer".delivery_webhook_immutable() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
 IF ROW(NEW.target_id,NEW.generation,NEW.partition,NEW.sequence,NEW.event_id,NEW.position_identity,NEW.predecessor,NEW.payload_hash,NEW.wire_hash,NEW.idempotency_key,NEW.created_at)
 IS DISTINCT FROM ROW(OLD.target_id,OLD.generation,OLD.partition,OLD.sequence,OLD.event_id,OLD.position_identity,OLD.predecessor,OLD.payload_hash,OLD.wire_hash,OLD.idempotency_key,OLD.created_at)
 THEN RAISE EXCEPTION 'immutable webhook identity'; END IF;
 RETURN NEW;
END $$;
CREATE TRIGGER delivery_webhook_immutable BEFORE UPDATE ON "indexer".delivery_webhooks FOR EACH ROW EXECUTE FUNCTION "indexer".delivery_webhook_immutable();
REVOKE ALL ON "indexer".delivery_webhooks,"indexer".delivery_webhook_cursors FROM PUBLIC;

CREATE TABLE "indexer".delivery_webhook_retry_audit (target_id text NOT NULL,generation text NOT NULL,partition text NOT NULL,sequence bigint NOT NULL,attempts_before integer NOT NULL,allowed_attempts integer NOT NULL,authorized_at timestamptz NOT NULL DEFAULT clock_timestamp());
REVOKE ALL ON "indexer".delivery_webhook_retry_audit FROM PUBLIC;

CREATE TABLE "indexer".delivery_archive_manifests (
 manifest_digest text PRIMARY KEY, source_stream text NOT NULL, from_height bigint NOT NULL,
 to_height bigint NOT NULL, raw_count bigint NOT NULL, final_block_hash text NOT NULL,
 manifest bytea NOT NULL, created_at timestamptz NOT NULL DEFAULT clock_timestamp(), CHECK(to_height>=from_height)
);
CREATE TABLE "indexer".delivery_rebuilds (
 consumer_id text NOT NULL, generation text NOT NULL, manifest_digest text NOT NULL REFERENCES "indexer".delivery_archive_manifests,
 next_height bigint NOT NULL, raw_count bigint NOT NULL DEFAULT 0, revision bigint NOT NULL DEFAULT 0,
 status text NOT NULL CHECK(status IN ('building','verified','active','retired')),
 projection_digest text, final_watermark jsonb, owner text NOT NULL, epoch bigint NOT NULL CHECK(epoch>0),
 lease_until timestamptz NOT NULL, PRIMARY KEY(consumer_id,generation)
);
CREATE TABLE "indexer".delivery_read_aliases (
 consumer_id text PRIMARY KEY, generation text NOT NULL, prior_generation text,
 revision bigint NOT NULL CHECK(revision>0), FOREIGN KEY(consumer_id,generation) REFERENCES "indexer".delivery_rebuilds
);
CREATE TABLE "indexer".delivery_cutover_history (
 consumer_id text NOT NULL, revision bigint NOT NULL, generation text NOT NULL, prior_generation text,
 changed_at timestamptz NOT NULL DEFAULT clock_timestamp(), PRIMARY KEY(consumer_id,revision)
);
CREATE TABLE "indexer".delivery_generation_paths (
 consumer_id text NOT NULL, generation text NOT NULL, authoritative_path text NOT NULL CHECK(authoritative_path IN ('legacy','durable')),
 PRIMARY KEY(consumer_id,generation)
);
REVOKE ALL ON "indexer".delivery_archive_manifests,"indexer".delivery_rebuilds,"indexer".delivery_read_aliases,"indexer".delivery_cutover_history,"indexer".delivery_generation_paths FROM PUBLIC;
CREATE TABLE "indexer".delivery_shadow_observations (
 range_digest text NOT NULL REFERENCES "indexer".delivery_archive_manifests(manifest_digest),
 path text NOT NULL CHECK(path IN ('legacy','durable')), event_id text NOT NULL,
 payload_hash text NOT NULL, source_order jsonb NOT NULL,
 PRIMARY KEY(range_digest,path,event_id)
);
CREATE TABLE "indexer".delivery_shadow_comparisons (
 manifest_digest text PRIMARY KEY REFERENCES "indexer".delivery_archive_manifests(manifest_digest),
 event_digest text NOT NULL, event_count bigint NOT NULL CHECK(event_count>0),
 compared_at timestamptz NOT NULL DEFAULT clock_timestamp()
);
REVOKE ALL ON "indexer".delivery_shadow_observations,"indexer".delivery_shadow_comparisons FROM PUBLIC;
CREATE TABLE "indexer".delivery_consumer_watermarks (
 consumer_id text NOT NULL,generation text NOT NULL,partition_kind text NOT NULL,partition_key text NOT NULL,
 broker_stream text NOT NULL,broker_sequence bigint NOT NULL CHECK(broker_sequence>0),
 source_order jsonb NOT NULL,position_sequence bigint NOT NULL CHECK(position_sequence>0),
 event_id text NOT NULL,envelope_hash text NOT NULL,committed_at timestamptz NOT NULL DEFAULT clock_timestamp(),
 PRIMARY KEY(consumer_id,generation,partition_kind,partition_key)
);
REVOKE ALL ON "indexer".delivery_consumer_watermarks FROM PUBLIC;
REVOKE ALL ON "indexer"."delivery_webhook_cursors" FROM PUBLIC;
REVOKE ALL ON "indexer"."delivery_webhooks" FROM PUBLIC;
REVOKE ALL ON "indexer"."delivery_webhook_retry_audit" FROM PUBLIC;
REVOKE ALL ON "indexer"."delivery_archive_manifests" FROM PUBLIC;
REVOKE ALL ON "indexer"."delivery_rebuilds" FROM PUBLIC;
REVOKE ALL ON "indexer"."delivery_read_aliases" FROM PUBLIC;
REVOKE ALL ON "indexer"."delivery_cutover_history" FROM PUBLIC;
REVOKE ALL ON "indexer"."delivery_generation_paths" FROM PUBLIC;
REVOKE ALL ON "indexer"."delivery_shadow_observations" FROM PUBLIC;
REVOKE ALL ON "indexer"."delivery_shadow_comparisons" FROM PUBLIC;
REVOKE ALL ON "indexer"."delivery_consumer_watermarks" FROM PUBLIC;
REVOKE ALL ON FUNCTION "indexer"."delivery_webhook_immutable"() FROM PUBLIC;
