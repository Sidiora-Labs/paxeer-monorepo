-- Service-owned schema template. Serial migration coordinator substitutes "stats".
CREATE TABLE "stats".delivery_outbox (
 event_id text PRIMARY KEY,
 schema_version integer NOT NULL,
 subject text NOT NULL,
 payload_hash text NOT NULL,
 envelope bytea NOT NULL,
 created_at timestamptz NOT NULL DEFAULT clock_timestamp(),
 state text NOT NULL DEFAULT 'pending' CHECK(state IN ('pending','published','quarantined')),
 attempts integer NOT NULL DEFAULT 0 CHECK(attempts>=0),
 available_at timestamptz NOT NULL DEFAULT clock_timestamp(),
 lease_owner text,
 lease_until timestamptz,
 fence bigint NOT NULL DEFAULT 0,
 lease_expiry_count bigint NOT NULL DEFAULT 0,
 claimed_at timestamptz,
 acknowledgement_latency_ms bigint,
 broker_stream text,
 broker_sequence bigint,
 published_at timestamptz,
 failure_class text,
 CHECK ((state='published') = (published_at IS NOT NULL)),
 CHECK (broker_sequence IS NULL OR broker_sequence>0)
);
CREATE INDEX delivery_pending ON "stats".delivery_outbox(available_at,created_at) WHERE state='pending';
CREATE FUNCTION "stats".delivery_immutable() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
 IF NEW.event_id IS DISTINCT FROM OLD.event_id OR NEW.schema_version IS DISTINCT FROM OLD.schema_version
 OR NEW.subject IS DISTINCT FROM OLD.subject OR NEW.payload_hash IS DISTINCT FROM OLD.payload_hash
 OR NEW.envelope IS DISTINCT FROM OLD.envelope OR NEW.created_at IS DISTINCT FROM OLD.created_at THEN
 RAISE EXCEPTION 'immutable delivery envelope'; END IF;
 RETURN NEW;
END $$;
CREATE TRIGGER delivery_immutable BEFORE UPDATE ON "stats".delivery_outbox FOR EACH ROW EXECUTE FUNCTION "stats".delivery_immutable();
CREATE TABLE "stats".delivery_inbox (
 consumer_id text NOT NULL,
 projection_generation text NOT NULL,
 event_id text NOT NULL,
 envelope_hash text NOT NULL,
 committed_at timestamptz NOT NULL DEFAULT clock_timestamp(),
 PRIMARY KEY(consumer_id,projection_generation,event_id)
);
CREATE TABLE "stats".delivery_quarantine (
 consumer_id text NOT NULL,
 projection_generation text NOT NULL,
 envelope_hash text NOT NULL,
 subject text NOT NULL,
 envelope bytea NOT NULL,
 failure_class text NOT NULL,
 created_at timestamptz NOT NULL DEFAULT clock_timestamp(),
 PRIMARY KEY(consumer_id,projection_generation,envelope_hash)
);

CREATE TABLE "stats".delivery_partition_producers (
 projection_version text NOT NULL, partition_kind text NOT NULL, partition_key text NOT NULL,
 last_sequence bigint NOT NULL DEFAULT 0 CHECK(last_sequence>=0), last_effect_identity text NOT NULL DEFAULT '',
 last_source_order bytea,
 PRIMARY KEY(projection_version,partition_kind,partition_key)
);
CREATE TABLE "stats".delivery_partition_positions (
 projection_version text NOT NULL, partition_kind text NOT NULL, partition_key text NOT NULL,
 sequence bigint NOT NULL CHECK(sequence>0), effect_identity text NOT NULL, position bytea NOT NULL,
 PRIMARY KEY(projection_version,partition_kind,partition_key,sequence),
 UNIQUE(projection_version,partition_kind,partition_key,effect_identity)
);
CREATE TABLE "stats".delivery_partition_leases (
 consumer_id text NOT NULL, projection_generation text NOT NULL, partition_kind text NOT NULL, partition_key text NOT NULL,
 owner text NOT NULL, epoch bigint NOT NULL CHECK(epoch>0), expires_at timestamptz NOT NULL,
 PRIMARY KEY(consumer_id,projection_generation,partition_kind,partition_key)
);
CREATE TABLE "stats".delivery_partition_offsets (
 consumer_id text NOT NULL, projection_generation text NOT NULL, partition_kind text NOT NULL, partition_key text NOT NULL,
 checkpoint_digest text NOT NULL CHECK(checkpoint_digest ~ '^[0-9a-f]{64}$'),
 initial_sequence bigint NOT NULL CHECK(initial_sequence>0), initial_predecessor text NOT NULL,
 next_sequence bigint NOT NULL CHECK(next_sequence>0), previous_effect_identity text NOT NULL,
 PRIMARY KEY(consumer_id,projection_generation,partition_kind,partition_key),
 FOREIGN KEY(consumer_id,projection_generation,partition_kind,partition_key) REFERENCES "stats".delivery_partition_leases
);
CREATE TABLE "stats".delivery_partition_commits (
 consumer_id text NOT NULL, projection_generation text NOT NULL, partition_kind text NOT NULL, partition_key text NOT NULL,
 sequence bigint NOT NULL CHECK(sequence>0), effect_identity text NOT NULL, position bytea NOT NULL,
 PRIMARY KEY(consumer_id,projection_generation,partition_kind,partition_key,sequence),
 UNIQUE(consumer_id,projection_generation,partition_kind,partition_key,effect_identity),
 FOREIGN KEY(consumer_id,projection_generation,partition_kind,partition_key) REFERENCES "stats".delivery_partition_offsets
);
CREATE TABLE "stats".delivery_ordered_quarantine (
 consumer_id text NOT NULL, projection_generation text NOT NULL, envelope_hash text NOT NULL,
 failure_class text NOT NULL CHECK(failure_class IN ('inner_schema','authority','identity_conflict','partition_binding')),
 subject text NOT NULL, publication_id text NOT NULL, envelope bytea NOT NULL CHECK(octet_length(envelope)<=1048576),
 affected_pools text[] NOT NULL, evidence_digest text NOT NULL,
 created_at timestamptz NOT NULL DEFAULT clock_timestamp(),
 PRIMARY KEY(consumer_id,projection_generation,envelope_hash,failure_class)
);
REVOKE ALL ON "stats".delivery_ordered_quarantine FROM PUBLIC;

CREATE VIEW "stats"."delivery_partition_offsets_v1" AS SELECT consumer_id,projection_generation,partition_kind,partition_key,checkpoint_digest,next_sequence,previous_effect_identity FROM "stats"."delivery_partition_offsets";

CREATE VIEW "stats"."delivery_partition_commits_v1" AS SELECT consumer_id,projection_generation,partition_kind,partition_key,sequence,effect_identity,position FROM "stats"."delivery_partition_commits";

CREATE VIEW "stats"."delivery_ordered_quarantine_v1" AS SELECT consumer_id,projection_generation,affected_pools FROM "stats"."delivery_ordered_quarantine";
