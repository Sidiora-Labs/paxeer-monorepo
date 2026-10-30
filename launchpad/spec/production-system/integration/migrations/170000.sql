SET LOCAL search_path = "pnl", pg_catalog;
CREATE TABLE portfolio_snapshots (
 id uuid PRIMARY KEY, wallet text NOT NULL, source_event text NOT NULL UNIQUE,
 digest text NOT NULL, revision numeric(78,0) NOT NULL, expires_at timestamptz NOT NULL,
 summary jsonb NOT NULL, observed_at timestamptz NOT NULL, UNIQUE(wallet,revision));
 CREATE INDEX portfolio_snapshot_history_seek ON portfolio_snapshots(wallet,observed_at DESC,revision DESC);
 CREATE TABLE portfolio_snapshot_rows (
 snapshot_id uuid NOT NULL REFERENCES portfolio_snapshots(id), ordinal integer NOT NULL,
 payload jsonb NOT NULL, PRIMARY KEY(snapshot_id,ordinal));
 CREATE TABLE portfolio_outbox (
 event_id uuid PRIMARY KEY REFERENCES portfolio_snapshots(id), payload jsonb NOT NULL,
 published_at timestamptz);
 CREATE INDEX portfolio_outbox_pending ON portfolio_outbox(event_id) WHERE published_at IS NULL;
CREATE TABLE portfolio_history_snapshots (
 id uuid PRIMARY KEY REFERENCES portfolio_snapshots(id), source_event text NOT NULL UNIQUE,
 digest text NOT NULL, row_count integer NOT NULL CHECK(row_count>=0));
 CREATE TABLE portfolio_history_rows (
 snapshot_id uuid NOT NULL REFERENCES portfolio_history_snapshots(id), ordinal integer NOT NULL,
 payload jsonb NOT NULL, PRIMARY KEY(snapshot_id,ordinal));
 CREATE TABLE portfolio_history_outbox (
 event_id uuid PRIMARY KEY REFERENCES portfolio_history_snapshots(id), payload jsonb NOT NULL,
 published_at timestamptz);
CREATE TABLE portfolio_cards (
 id uuid PRIMARY KEY, source_event text NOT NULL UNIQUE, wallet text NOT NULL,
 snapshot_id uuid NOT NULL REFERENCES portfolio_snapshots(id), snapshot jsonb NOT NULL,
 state text NOT NULL CHECK(state IN ('pending','public','revoked')),
 image bytea, attempts integer NOT NULL DEFAULT 0 CHECK(attempts BETWEEN 0 AND 3),
 created_at timestamptz NOT NULL, purge_requested_at timestamptz, purged_at timestamptz);
 CREATE INDEX portfolio_cards_owner_created ON portfolio_cards(wallet,created_at);
 CREATE TABLE portfolio_card_outbox (
 event_id text PRIMARY KEY,card_id uuid NOT NULL REFERENCES portfolio_cards(id),
 kind text NOT NULL CHECK(kind IN ('render','purge')),published_at timestamptz);
ALTER TABLE portfolio_card_outbox ADD COLUMN fence bigint NOT NULL DEFAULT 0, ADD COLUMN lease_owner text, ADD COLUMN lease_until timestamptz, ADD COLUMN attempts integer NOT NULL DEFAULT 0, ADD COLUMN next_attempt timestamptz NOT NULL DEFAULT clock_timestamp(), ADD COLUMN last_error text;
 CREATE INDEX portfolio_card_effect_ready ON portfolio_card_outbox(next_attempt) WHERE published_at IS NULL;
CREATE TABLE portfolio_export_jobs (
 id uuid PRIMARY KEY,source_event text NOT NULL UNIQUE,wallet text NOT NULL,
 snapshot_id uuid NOT NULL REFERENCES portfolio_snapshots(id),format text NOT NULL CHECK(format IN ('jsonl','csv')),
 state text NOT NULL CHECK(state IN ('queued','running','complete','cancelled','failed','expired')),
 manifest jsonb,created_at timestamptz NOT NULL DEFAULT clock_timestamp(),completed_at timestamptz,
 expires_at timestamptz,fence bigint NOT NULL DEFAULT 0,lease_owner text,lease_until timestamptz,
 attempts integer NOT NULL DEFAULT 0,last_error text,cleaned_at timestamptz,
 CHECK(state<>'complete' OR (manifest IS NOT NULL AND expires_at IS NOT NULL)));
 CREATE INDEX portfolio_export_ready ON portfolio_export_jobs(state,created_at);
 CREATE TABLE portfolio_export_outbox (event_id text PRIMARY KEY,job_id uuid NOT NULL REFERENCES portfolio_export_jobs(id),kind text NOT NULL,published_at timestamptz);
CREATE TABLE portfolio_snapshot_history_metadata (snapshot_id uuid PRIMARY KEY REFERENCES portfolio_snapshots(id),source_event text NOT NULL UNIQUE,payload jsonb NOT NULL);
REVOKE ALL ON "pnl"."portfolio_snapshots" FROM PUBLIC;
REVOKE ALL ON "pnl"."portfolio_snapshot_rows" FROM PUBLIC;
REVOKE ALL ON "pnl"."portfolio_outbox" FROM PUBLIC;
REVOKE ALL ON "pnl"."portfolio_history_snapshots" FROM PUBLIC;
REVOKE ALL ON "pnl"."portfolio_history_rows" FROM PUBLIC;
REVOKE ALL ON "pnl"."portfolio_history_outbox" FROM PUBLIC;
REVOKE ALL ON "pnl"."portfolio_cards" FROM PUBLIC;
REVOKE ALL ON "pnl"."portfolio_card_outbox" FROM PUBLIC;
REVOKE ALL ON "pnl"."portfolio_export_jobs" FROM PUBLIC;
REVOKE ALL ON "pnl"."portfolio_export_outbox" FROM PUBLIC;
REVOKE ALL ON "pnl"."portfolio_snapshot_history_metadata" FROM PUBLIC;
CREATE TABLE portfolio_shares_v3 (
 id uuid PRIMARY KEY, wallet text NOT NULL, account_id text NOT NULL, session_id text NOT NULL, auth_epoch bigint NOT NULL,
 realm text NOT NULL, release_digest text NOT NULL, interface_digest text NOT NULL,
 snapshot_id uuid NOT NULL REFERENCES portfolio_snapshots(id), public_snapshot jsonb NOT NULL, snapshot_digest text NOT NULL,
 intent text NOT NULL, preview_until timestamptz NOT NULL, expires_at timestamptz NOT NULL, publish_key text,
 state text NOT NULL CHECK(state IN ('draft','pending','public','revoked','expired')), image bytea,
 attempts integer NOT NULL DEFAULT 0 CHECK(attempts BETWEEN 0 AND 3), created_at timestamptz NOT NULL,
 purge_requested_at timestamptz, purged_at timestamptz,
 UNIQUE(wallet,publish_key));
 CREATE INDEX portfolio_shares_v3_owner ON portfolio_shares_v3(wallet,created_at,id);
 CREATE INDEX portfolio_shares_v3_expiry ON portfolio_shares_v3(expires_at) WHERE state IN ('draft','pending','public');
 CREATE TABLE portfolio_share_outbox_v3 (
 event_id text PRIMARY KEY, share_id uuid NOT NULL REFERENCES portfolio_shares_v3(id),
 kind text NOT NULL CHECK(kind IN ('render','purge')), fence bigint NOT NULL DEFAULT 0,
 lease_owner text, lease_until timestamptz, attempts integer NOT NULL DEFAULT 0,
 next_attempt timestamptz NOT NULL DEFAULT clock_timestamp(), last_error text, published_at timestamptz);
 CREATE INDEX portfolio_share_outbox_v3_ready ON portfolio_share_outbox_v3(next_attempt) WHERE published_at IS NULL;
REVOKE ALL ON pnl.portfolio_shares_v3,pnl.portfolio_share_outbox_v3 FROM PUBLIC;
