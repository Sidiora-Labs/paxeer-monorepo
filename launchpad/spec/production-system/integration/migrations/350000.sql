SET LOCAL search_path = "users", pg_catalog;
-- Serial integration proposal only; not a migration registry entry.
CREATE TABLE account_state (
 account_id uuid PRIMARY KEY, wallet text NOT NULL UNIQUE CHECK(wallet ~ '^0x[0-9a-f]{40}$'),
 revision bigint NOT NULL DEFAULT 0 CHECK(revision >= 0), deleted boolean NOT NULL DEFAULT false,
 profile jsonb NOT NULL, preferences jsonb NOT NULL, updated_at timestamptz NOT NULL DEFAULT now()
);
CREATE TABLE account_watch (
 account_id uuid NOT NULL REFERENCES account_state(account_id), chain_id integer NOT NULL CHECK(chain_id=125),
 pool text NOT NULL CHECK(pool ~ '^0x[0-9a-f]{40}$'), added_at timestamptz NOT NULL, revision bigint NOT NULL, alert_enabled boolean NOT NULL DEFAULT false,
 PRIMARY KEY(account_id,chain_id,pool)
);
CREATE INDEX account_watch_cursor ON account_watch(account_id,added_at,pool);
CREATE TABLE account_consent (
 account_id uuid NOT NULL REFERENCES account_state(account_id), category text NOT NULL, channel text NOT NULL,
 enabled boolean NOT NULL, source text NOT NULL, policy_version text NOT NULL, revision bigint NOT NULL,
 revoke_revision bigint NOT NULL, changed_at timestamptz NOT NULL, PRIMARY KEY(account_id,category,channel)
);
CREATE TABLE account_endpoint (
 fingerprint bytea PRIMARY KEY, account_id uuid NOT NULL REFERENCES account_state(account_id),
 ciphertext bytea NOT NULL, platform text NOT NULL, consent_revision bigint NOT NULL,
 validated_at timestamptz NOT NULL, revoked boolean NOT NULL DEFAULT false
);
CREATE TABLE account_job (
 job_id uuid PRIMARY KEY, account_id uuid NOT NULL REFERENCES account_state(account_id),
 kind text NOT NULL CHECK(kind IN ('export','deletion')), state text NOT NULL,
 requested_at timestamptz NOT NULL, expires_at timestamptz, payload jsonb NOT NULL,
 download_hash bytea, consumed_at timestamptz
);
CREATE TABLE account_idempotency (
 account_id uuid NOT NULL REFERENCES account_state(account_id), method text NOT NULL, path text NOT NULL,
 key uuid NOT NULL, digest bytea NOT NULL, result jsonb NOT NULL, expires_at timestamptz NOT NULL,
 PRIMARY KEY(account_id,method,path,key)
);
CREATE TABLE account_outbox (
 event_id uuid PRIMARY KEY, account_id uuid NOT NULL, revision bigint NOT NULL, kind text NOT NULL,
 payload jsonb NOT NULL, created_at timestamptz NOT NULL DEFAULT now()
);
CREATE TABLE account_backfill (
 source_key text PRIMARY KEY, source_hash bytea NOT NULL, account_id uuid,
 state text NOT NULL CHECK(state IN ('linked','quarantined')), reason text NOT NULL
);

CREATE TABLE account_endpoint_proof (digest bytea PRIMARY KEY, expires_at timestamptz NOT NULL);
