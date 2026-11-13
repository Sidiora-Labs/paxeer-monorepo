SET LOCAL search_path = "pnl", pg_catalog;
CREATE TABLE inventory_l16_baselines (
 baseline_digest text PRIMARY KEY CHECK(baseline_digest ~ '^[0-9a-f]{64}$'), consumer_id text NOT NULL,
 projection_generation text NOT NULL, partition text NOT NULL, wallet text NOT NULL, token text NOT NULL,
 creation_height bigint NOT NULL CHECK(creation_height>0), creation_block_hash text NOT NULL,
 source_stream text NOT NULL, source_owner text NOT NULL, source_epoch bigint NOT NULL CHECK(source_epoch>0),
 movement_digest text NOT NULL, observed_balance_raw text NOT NULL CHECK(observed_balance_raw='0'),
 attestation bytea NOT NULL, captured_at timestamptz NOT NULL DEFAULT clock_timestamp(),
 UNIQUE(consumer_id,projection_generation,partition,source_stream,source_owner,source_epoch)
);
CREATE VIEW inventory_l16_baselines_v1 AS SELECT baseline_digest,consumer_id,projection_generation,partition,wallet,token,creation_height,creation_block_hash,source_stream,source_owner,source_epoch,movement_digest,observed_balance_raw FROM inventory_l16_baselines;
CREATE TABLE inventory_l16_baseline_uses (
 projection_generation text NOT NULL, partition text NOT NULL, effect_identity text NOT NULL,
 owner_epoch bigint NOT NULL CHECK(owner_epoch>0), baseline_digest text NOT NULL REFERENCES inventory_l16_baselines,
 checkpoint_height bigint NOT NULL, checkpoint_hash text NOT NULL, movement_digest text NOT NULL,
 replayed_balance_raw text NOT NULL, attestation_digest text NOT NULL, attestation bytea NOT NULL,
 PRIMARY KEY(projection_generation,partition,effect_identity,owner_epoch)
);
REVOKE ALL ON inventory_l16_baselines,inventory_l16_baseline_uses,inventory_l16_baselines_v1 FROM PUBLIC;
CREATE VIEW inventory_l16_stream_state_v1 AS SELECT partition,revision,state,last_identity,height,tx_index,log_index FROM inventory_l16;
CREATE VIEW inventory_l16_stream_proof_v1 AS
 SELECT e.projection_generation,e.partition,e.effect_identity,e.leg_identity,e.mapping_digest,e.position,
 b.owner_epoch,b.baseline_digest,b.attestation_digest,b.checkpoint_height,b.checkpoint_hash,b.movement_digest,b.replayed_balance_raw,o.payload
 FROM inventory_l16_effects e
 JOIN inventory_l16_baseline_uses b ON b.projection_generation=e.projection_generation AND b.partition=e.partition AND b.effect_identity=e.effect_identity
 JOIN inventory_l16_outbox o ON o.consumer_version='proportional_floor_v1:'||e.projection_generation AND o.identity=e.leg_identity AND o.partition=e.partition;
REVOKE ALL ON inventory_l16_stream_state_v1,inventory_l16_stream_proof_v1 FROM PUBLIC;
