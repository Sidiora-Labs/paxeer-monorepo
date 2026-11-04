SET LOCAL search_path = "pnl", pg_catalog;
CREATE TABLE inventory_l16_wallet_ranges_v3 (
 snapshot_id uuid PRIMARY KEY,wallet text NOT NULL,generation text NOT NULL,start_at timestamptz NOT NULL,end_at timestamptz NOT NULL,release_digest text NOT NULL,input_digest text NOT NULL,watermark jsonb NOT NULL);
CREATE INDEX inventory_l16_wallet_ranges_v3_latest ON inventory_l16_wallet_ranges_v3(wallet,end_at DESC);
CREATE TABLE inventory_l16_wallet_intervals_v3 (snapshot_id uuid NOT NULL REFERENCES inventory_l16_wallet_ranges_v3(snapshot_id),interval text NOT NULL CHECK(interval IN ('1h','1d')),bucket timestamptz NOT NULL,payload jsonb NOT NULL,PRIMARY KEY(snapshot_id,interval,bucket));
CREATE TRIGGER inventory_l16_range_immutable_v3 BEFORE UPDATE OR DELETE ON inventory_l16_wallet_ranges_v3 FOR EACH ROW EXECUTE FUNCTION inventory_l16_append_only_v2();
CREATE TRIGGER inventory_l16_interval_immutable_v3 BEFORE UPDATE OR DELETE ON inventory_l16_wallet_intervals_v3 FOR EACH ROW EXECUTE FUNCTION inventory_l16_append_only_v2();
CREATE VIEW "pnl"."inventory_l16_wallet_ranges_read_v3" WITH (security_barrier=true) AS SELECT "snapshot_id", "wallet", "generation", "start_at", "end_at", "release_digest", "input_digest", "watermark" FROM "pnl"."inventory_l16_wallet_ranges_v3";
REVOKE ALL ON "pnl"."inventory_l16_wallet_ranges_v3" FROM PUBLIC;
REVOKE ALL ON "pnl"."inventory_l16_wallet_ranges_read_v3" FROM PUBLIC;
CREATE VIEW "pnl"."inventory_l16_wallet_intervals_read_v3" WITH (security_barrier=true) AS SELECT "snapshot_id", "interval", "bucket", "payload" FROM "pnl"."inventory_l16_wallet_intervals_v3";
REVOKE ALL ON "pnl"."inventory_l16_wallet_intervals_v3" FROM PUBLIC;
REVOKE ALL ON "pnl"."inventory_l16_wallet_intervals_read_v3" FROM PUBLIC;
