SET LOCAL search_path = "pnl", pg_catalog;
CREATE TABLE inventory_l16_archive_v2 (
 generation text NOT NULL,partition text NOT NULL,identity text NOT NULL,payload_hash text NOT NULL,payload bytea NOT NULL,
 PRIMARY KEY(generation,partition,identity));
CREATE TABLE inventory_l16_snapshots_v2 (
 generation text NOT NULL,partition text NOT NULL,snapshot_digest text NOT NULL,payload bytea NOT NULL,
 height bigint NOT NULL CHECK(height>0),block_hash text NOT NULL,release_digest text NOT NULL,
 PRIMARY KEY(generation,partition,snapshot_digest));
CREATE TABLE inventory_l16_active_v2 (
 partition text PRIMARY KEY,generation text NOT NULL,snapshot_digest text NOT NULL,height bigint NOT NULL,
 FOREIGN KEY(generation,partition,snapshot_digest) REFERENCES inventory_l16_snapshots_v2);
CREATE TABLE inventory_l16_checkpoints_v2 (
 generation text NOT NULL,partition text NOT NULL,day date NOT NULL,snapshot_digest text NOT NULL,checkpoint_digest text NOT NULL,payload bytea NOT NULL,
 PRIMARY KEY(generation,partition,snapshot_digest,day),
 FOREIGN KEY(generation,partition,snapshot_digest) REFERENCES inventory_l16_snapshots_v2);
CREATE TABLE inventory_l16_quarantine_v2 (
 generation text NOT NULL,partition text NOT NULL,height bigint NOT NULL,block_hash text NOT NULL,reason text NOT NULL,payload bytea NOT NULL,
 observed_at timestamptz NOT NULL DEFAULT clock_timestamp(),PRIMARY KEY(generation,partition,height,block_hash));
CREATE FUNCTION inventory_l16_append_only_v2() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN RAISE EXCEPTION 'immutable inventory evidence'; END; $$;
CREATE TRIGGER inventory_l16_archive_immutable_v2 BEFORE UPDATE OR DELETE ON inventory_l16_archive_v2 FOR EACH ROW EXECUTE FUNCTION inventory_l16_append_only_v2();
CREATE TRIGGER inventory_l16_snapshot_immutable_v2 BEFORE UPDATE OR DELETE ON inventory_l16_snapshots_v2 FOR EACH ROW EXECUTE FUNCTION inventory_l16_append_only_v2();
CREATE TRIGGER inventory_l16_checkpoint_immutable_v2 BEFORE UPDATE OR DELETE ON inventory_l16_checkpoints_v2 FOR EACH ROW EXECUTE FUNCTION inventory_l16_append_only_v2();
REVOKE ALL ON "pnl"."inventory_l16_archive_v2" FROM PUBLIC;
REVOKE ALL ON "pnl"."inventory_l16_snapshots_v2" FROM PUBLIC;
REVOKE ALL ON "pnl"."inventory_l16_active_v2" FROM PUBLIC;
REVOKE ALL ON "pnl"."inventory_l16_checkpoints_v2" FROM PUBLIC;
REVOKE ALL ON "pnl"."inventory_l16_quarantine_v2" FROM PUBLIC;
REVOKE ALL ON FUNCTION "pnl"."inventory_l16_append_only_v2"() FROM PUBLIC;
