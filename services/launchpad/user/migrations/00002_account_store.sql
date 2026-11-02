-- +goose Up
CREATE TABLE account_state (
    account_id uuid PRIMARY KEY,
    wallet varchar(42) NOT NULL UNIQUE,
    revision bigint NOT NULL DEFAULT 0 CHECK (revision >= 0),
    deleted boolean NOT NULL DEFAULT false,
    profile jsonb NOT NULL,
    preferences jsonb NOT NULL,
    updated_at timestamptz NOT NULL DEFAULT clock_timestamp()
);

CREATE TABLE account_watch (
    account_id uuid NOT NULL REFERENCES account_state(account_id),
    chain_id integer NOT NULL CHECK (chain_id = 125),
    pool varchar(42) NOT NULL,
    added_at timestamptz NOT NULL,
    revision bigint NOT NULL CHECK (revision >= 0),
    alert_enabled boolean NOT NULL DEFAULT false,
    PRIMARY KEY (account_id, chain_id, pool)
);
CREATE INDEX account_watch_page ON account_watch (account_id, added_at, pool);

CREATE TABLE account_consent (
    account_id uuid NOT NULL REFERENCES account_state(account_id),
    category text NOT NULL,
    channel text NOT NULL,
    enabled boolean NOT NULL,
    source text NOT NULL,
    policy_version text NOT NULL,
    revision bigint NOT NULL CHECK (revision >= 0),
    revoke_revision bigint NOT NULL DEFAULT 0 CHECK (revoke_revision >= 0 AND revoke_revision <= revision),
    changed_at timestamptz NOT NULL,
    PRIMARY KEY (account_id, category, channel)
);

CREATE TABLE account_outbox (
    event_id uuid PRIMARY KEY,
    account_id uuid NOT NULL REFERENCES account_state(account_id),
    revision bigint NOT NULL CHECK (revision >= 0),
    kind text NOT NULL,
    payload jsonb NOT NULL
);
CREATE INDEX account_outbox_account_revision ON account_outbox (account_id, revision);

CREATE TABLE account_idempotency (
    account_id uuid NOT NULL REFERENCES account_state(account_id),
    method varchar(8) NOT NULL,
    path varchar(512) NOT NULL,
    key uuid NOT NULL,
    digest bytea NOT NULL CHECK (octet_length(digest) = 32),
    result jsonb NOT NULL,
    expires_at timestamptz NOT NULL,
    PRIMARY KEY (account_id, method, path, key)
);
CREATE INDEX account_idempotency_expiry ON account_idempotency (expires_at);

CREATE TABLE account_endpoint_proof (
    proof_hash bytea PRIMARY KEY CHECK (octet_length(proof_hash) = 32),
    expires_at timestamptz NOT NULL
);
CREATE INDEX account_endpoint_proof_expiry ON account_endpoint_proof (expires_at);

CREATE TABLE account_device_installation (
    installation_id uuid PRIMARY KEY,
    account_id uuid NOT NULL REFERENCES account_state(account_id),
    session_id uuid NOT NULL,
    provider text NOT NULL,
    platform text NOT NULL,
    verified_at timestamptz NOT NULL,
    revoked boolean NOT NULL DEFAULT false
);
CREATE INDEX account_device_installation_account ON account_device_installation (account_id);

CREATE TABLE account_endpoint (
    fingerprint bytea PRIMARY KEY CHECK (octet_length(fingerprint) = 32),
    account_id uuid NOT NULL REFERENCES account_state(account_id),
    ciphertext bytea NOT NULL,
    platform text NOT NULL,
    consent_revision bigint NOT NULL CHECK (consent_revision >= 0),
    validated_at timestamptz NOT NULL,
    revoked boolean NOT NULL DEFAULT false,
    device_id uuid,
    provider text,
    session_id uuid
);
CREATE INDEX account_endpoint_account_device ON account_endpoint (account_id, device_id);

CREATE TABLE account_job (
    job_id uuid PRIMARY KEY,
    account_id uuid NOT NULL REFERENCES account_state(account_id),
    kind text NOT NULL CHECK (kind IN ('export', 'deletion')),
    state text NOT NULL CHECK (state IN ('requested', 'preparing', 'processing', 'failed', 'ready', 'expired', 'completed', 'blocked')),
    requested_at timestamptz NOT NULL,
    payload jsonb NOT NULL,
    expires_at timestamptz,
    download_hash bytea,
    consumed_at timestamptz,
    lease_owner text,
    lease_until timestamptz,
    fence bigint NOT NULL DEFAULT 0 CHECK (fence >= 0)
);
CREATE INDEX account_job_claim ON account_job (requested_at, job_id)
    WHERE state IN ('requested', 'preparing', 'processing', 'failed');
CREATE INDEX account_job_account_kind ON account_job (account_id, kind);
CREATE INDEX account_job_export_expiry ON account_job (expires_at)
    WHERE kind = 'export' AND state = 'ready';

CREATE TABLE account_backfill (
    source_key text PRIMARY KEY,
    source_hash bytea NOT NULL,
    account_id uuid,
    state text NOT NULL CHECK (state IN ('linked', 'quarantined')),
    reason text NOT NULL
);

CREATE TABLE account_backfill_range (
    range_id text NOT NULL,
    after_wallet text NOT NULL,
    batch_limit integer NOT NULL CHECK (batch_limit BETWEEN 1 AND 500),
    result jsonb NOT NULL,
    completed_at timestamptz NOT NULL,
    PRIMARY KEY (range_id, after_wallet, batch_limit)
);

CREATE TABLE account_read_switch (
    singleton boolean PRIMARY KEY CHECK (singleton),
    stable_reads boolean NOT NULL,
    changed_at timestamptz NOT NULL,
    generation bigint NOT NULL CHECK (generation >= 1)
);

CREATE TABLE account_revision_inbox (
    event_id text PRIMARY KEY,
    digest text NOT NULL,
    account_id uuid NOT NULL REFERENCES account_state(account_id),
    revision bigint NOT NULL CHECK (revision >= 1)
);
CREATE INDEX account_revision_inbox_account ON account_revision_inbox (account_id, revision);

-- +goose Down
DROP TABLE account_revision_inbox;
DROP TABLE account_read_switch;
DROP TABLE account_backfill_range;
DROP TABLE account_backfill;
DROP TABLE account_job;
DROP TABLE account_endpoint;
DROP TABLE account_device_installation;
DROP TABLE account_endpoint_proof;
DROP TABLE account_idempotency;
DROP TABLE account_outbox;
DROP TABLE account_consent;
DROP TABLE account_watch;
DROP TABLE account_state;
