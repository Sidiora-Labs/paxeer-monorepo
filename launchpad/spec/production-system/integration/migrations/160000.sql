SET LOCAL search_path = "pnl", pg_catalog;
CREATE TABLE inventory_l16 (partition text PRIMARY KEY, revision bigint NOT NULL CHECK(revision>0), state jsonb NOT NULL, last_identity text NOT NULL, height numeric(20,0) NOT NULL, tx_index bigint NOT NULL, log_index bigint NOT NULL);
CREATE TABLE inventory_l16_inbox (consumer_version text NOT NULL, identity text NOT NULL, partition text NOT NULL, payload_hash text NOT NULL, payload jsonb NOT NULL, PRIMARY KEY(consumer_version,identity,partition));
CREATE TABLE inventory_l16_outbox (consumer_version text NOT NULL, identity text NOT NULL, partition text NOT NULL, payload jsonb NOT NULL, PRIMARY KEY(consumer_version,identity,partition));
CREATE TABLE inventory_l16_transfers (consumer_version text NOT NULL, identity text NOT NULL, source_partition text NOT NULL, destination_partition text NOT NULL, payload_hash text NOT NULL, payload jsonb NOT NULL, PRIMARY KEY(consumer_version,identity,source_partition,destination_partition));
CREATE TABLE inventory_l16_effects (
 projection_generation text NOT NULL, partition text NOT NULL, effect_identity text NOT NULL,
 leg_identity text NOT NULL, mapping_digest text NOT NULL CHECK(mapping_digest ~ '^[0-9a-f]{64}$'),
 position bytea NOT NULL,
 PRIMARY KEY(projection_generation,partition,effect_identity),
 UNIQUE(projection_generation,partition,leg_identity)
);