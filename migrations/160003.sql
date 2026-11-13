SET LOCAL search_path = "pnl", pg_catalog;
CREATE TABLE inventory_l16_live_owners_v1 (
 wallet text PRIMARY KEY CHECK(wallet ~ '^0x[0-9a-f]{40}$'),account_id text NOT NULL,session_id text NOT NULL,auth_epoch bigint NOT NULL CHECK(auth_epoch>=0),proof_at timestamptz NOT NULL,expires_at timestamptz NOT NULL,
 accepted_height bigint NOT NULL DEFAULT 0 CHECK(accepted_height>=0),accepted_hash text,accepted_snapshot uuid REFERENCES portfolio_snapshots(id),source_digest text,
 updated_at timestamptz NOT NULL DEFAULT clock_timestamp(),last_attempt_at timestamptz NOT NULL DEFAULT 'epoch',cancelled_at timestamptz);
CREATE INDEX inventory_l16_live_owners_due_v1 ON inventory_l16_live_owners_v1(last_attempt_at,wallet) WHERE cancelled_at IS NULL;
CREATE TABLE inventory_l16_live_commits_v1 (
 generation text NOT NULL,wallet text NOT NULL,source_stream text NOT NULL,height bigint NOT NULL CHECK(height>0),block_hash text NOT NULL,source_owner text NOT NULL,source_epoch bigint NOT NULL CHECK(source_epoch>0),release_digest text NOT NULL,input_digest text NOT NULL,snapshot_id uuid NOT NULL REFERENCES portfolio_snapshots(id),
 PRIMARY KEY(generation,wallet,source_stream,height),UNIQUE(snapshot_id));
CREATE TRIGGER inventory_l16_live_commit_immutable_v1 BEFORE UPDATE OR DELETE ON inventory_l16_live_commits_v1 FOR EACH ROW EXECUTE FUNCTION inventory_l16_append_only_v2();
REVOKE ALL ON "pnl"."inventory_l16_live_owners_v1" FROM PUBLIC;
REVOKE ALL ON "pnl"."inventory_l16_live_commits_v1" FROM PUBLIC;
