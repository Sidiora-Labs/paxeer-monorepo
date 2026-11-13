SET LOCAL search_path = "users", pg_catalog;
ALTER TABLE account_endpoint ADD COLUMN device_id uuid;
ALTER TABLE account_endpoint ADD COLUMN provider text CHECK(provider IN ('apns','fcm','webpush'));
ALTER TABLE account_endpoint ADD COLUMN session_id uuid;
CREATE UNIQUE INDEX account_endpoint_device_owner ON account_endpoint(account_id,device_id) WHERE device_id IS NOT NULL;
ALTER TABLE account_job ADD COLUMN lease_owner text;
ALTER TABLE account_job ADD COLUMN lease_until timestamptz;
ALTER TABLE account_job ADD COLUMN fence bigint NOT NULL DEFAULT 0 CHECK(fence>=0);
CREATE INDEX account_job_pending ON account_job(requested_at,job_id) WHERE state IN('requested','preparing','processing','failed');
CREATE TABLE account_revision_inbox(event_id text PRIMARY KEY,digest text NOT NULL,account_id uuid NOT NULL REFERENCES account_state,revision bigint NOT NULL CHECK(revision>0));
CREATE TABLE account_backfill_range(range_id text NOT NULL,after_wallet text NOT NULL,batch_limit integer NOT NULL CHECK(batch_limit BETWEEN 1 AND 500),result jsonb NOT NULL,completed_at timestamptz NOT NULL,PRIMARY KEY(range_id,after_wallet,batch_limit));
CREATE TABLE account_read_switch(singleton boolean PRIMARY KEY CHECK(singleton),stable_reads boolean NOT NULL,changed_at timestamptz NOT NULL,generation bigint NOT NULL DEFAULT 1 CHECK(generation>0));

CREATE VIEW users.account_public_search_v2 AS SELECT a.wallet,a.profile->>'displayName' AS display_name,a.revision,a.updated_at,EXISTS(SELECT 1 FROM users.identity_wallet_links l WHERE l.account_id=a.account_id AND l.wallet=a.wallet AND l.chain_id=125 AND l.state='verified' AND l.visibility='public_optin') AND NOT a.deleted AS public,a.deleted FROM users.account_state a;
REVOKE ALL ON "users"."account_revision_inbox" FROM PUBLIC;
REVOKE ALL ON "users"."account_backfill_range" FROM PUBLIC;
REVOKE ALL ON "users"."account_read_switch" FROM PUBLIC;
CREATE VIEW users.account_state_v2 AS SELECT account_id,wallet,revision,deleted FROM users.account_state;
REVOKE ALL ON users.account_state_v2 FROM PUBLIC;
CREATE TABLE account_device_installation(installation_id uuid PRIMARY KEY,account_id uuid NOT NULL REFERENCES account_state(account_id),session_id uuid NOT NULL,provider text NOT NULL CHECK(provider IN('apns','fcm','webpush')),platform text NOT NULL CHECK(platform IN('ios','android','web')),verified_at timestamptz NOT NULL,revoked boolean NOT NULL DEFAULT false);

REVOKE ALL ON users.account_device_installation FROM PUBLIC;
