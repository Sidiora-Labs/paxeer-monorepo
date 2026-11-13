SET LOCAL search_path = "chat", pg_catalog;
-- v2 social resources of the media/social platform runtime (RUNTIME_PROFILE=platform): callouts,
-- bounties (+ submissions, acceptances) and reports, read and written by internal/devsocial as
-- chat_runtime. Column-for-column the DDL of media/social/migrations/00002_dev_v2_social.sql section 3,
-- which only the non-production runtime applies; in production the coordinator owns these tables.
CREATE TABLE v2_callouts (
 callout_id uuid PRIMARY KEY,
 author text NOT NULL CHECK (author ~ '^0x[0-9a-f]{40}$'),
 client_callout_id uuid NOT NULL,
 pool text NOT NULL CHECK (pool ~ '^0x[0-9a-f]{40}$'),
 body text NOT NULL CHECK (char_length(body) BETWEEN 1 AND 2000),
 direction text NOT NULL CHECK (direction IN ('bullish', 'bearish', 'neutral')),
 disclosure text NOT NULL CHECK (disclosure IN ('organic', 'sponsored', 'creator')),
 revision bigint NOT NULL DEFAULT 1 CHECK (revision > 0),
 removed boolean NOT NULL DEFAULT false,
 created_at timestamptz NOT NULL DEFAULT date_trunc('milliseconds', clock_timestamp()),
 UNIQUE (author, client_callout_id)
);
CREATE INDEX v2_callouts_pool ON v2_callouts (pool, created_at DESC, callout_id DESC);
CREATE TABLE v2_bounties (
 bounty_id uuid PRIMARY KEY,
 creator text NOT NULL CHECK (creator ~ '^0x[0-9a-f]{40}$'),
 idempotency_key text NOT NULL CHECK (char_length(idempotency_key) BETWEEN 1 AND 128),
 request_hash text NOT NULL,
 pool text CHECK (pool IS NULL OR pool ~ '^0x[0-9a-f]{40}$'),
 title text NOT NULL CHECK (char_length(title) BETWEEN 1 AND 120),
 description text NOT NULL CHECK (char_length(description) BETWEEN 1 AND 10000),
 asset text NOT NULL CHECK (asset ~ '^0x[0-9a-f]{40}$'),
 budget_raw numeric(78, 0) NOT NULL CHECK (budget_raw > 0),
 state text NOT NULL DEFAULT 'draft' CHECK (state IN ('draft', 'funding', 'open', 'reviewing', 'paid', 'cancelled', 'expired')),
 revision bigint NOT NULL DEFAULT 1 CHECK (revision > 0),
 deadline_at timestamptz,
 created_at timestamptz NOT NULL DEFAULT clock_timestamp(),
 UNIQUE (creator, idempotency_key)
);
CREATE INDEX v2_bounties_list ON v2_bounties (created_at DESC, bounty_id DESC);
CREATE INDEX v2_bounties_state ON v2_bounties (state, created_at DESC, bounty_id DESC);
CREATE INDEX v2_bounties_pool ON v2_bounties (pool, created_at DESC, bounty_id DESC);
CREATE TABLE v2_bounty_submissions (
 submission_id uuid PRIMARY KEY,
 bounty_id uuid NOT NULL REFERENCES v2_bounties (bounty_id),
 author text NOT NULL CHECK (author ~ '^0x[0-9a-f]{40}$'),
 client_submission_id uuid NOT NULL,
 body text NOT NULL CHECK (char_length(body) BETWEEN 1 AND 5000),
 evidence_urls jsonb NOT NULL CHECK (jsonb_typeof(evidence_urls) = 'array'),
 state text NOT NULL DEFAULT 'submitted' CHECK (state IN ('submitted', 'accepted', 'rejected', 'withdrawn')),
 award_raw numeric(78, 0) CHECK (award_raw IS NULL OR award_raw > 0),
 created_at timestamptz NOT NULL DEFAULT clock_timestamp(),
 UNIQUE (author, client_submission_id)
);
CREATE INDEX v2_bounty_submissions_bounty ON v2_bounty_submissions (bounty_id, created_at DESC, submission_id DESC);
CREATE TABLE v2_bounty_acceptances (
 creator text NOT NULL,
 idempotency_key text NOT NULL,
 request_hash text NOT NULL,
 submission_id uuid NOT NULL REFERENCES v2_bounty_submissions (submission_id),
 created_at timestamptz NOT NULL DEFAULT clock_timestamp(),
 PRIMARY KEY (creator, idempotency_key)
);
CREATE TABLE v2_reports (
 report_id uuid PRIMARY KEY,
 reporter text NOT NULL CHECK (reporter ~ '^0x[0-9a-f]{40}$'),
 idempotency_key text NOT NULL CHECK (char_length(idempotency_key) BETWEEN 1 AND 128),
 request_hash text NOT NULL,
 target_type text NOT NULL CHECK (target_type IN ('message', 'comment', 'stream', 'profile', 'token')),
 target_id text NOT NULL CHECK (char_length(target_id) BETWEEN 1 AND 160),
 reason text NOT NULL CHECK (reason IN ('spam', 'abuse', 'fraud', 'copyright', 'other')),
 reason_text text NOT NULL CHECK (char_length(reason_text) BETWEEN 1 AND 256),
 evidence text NOT NULL CHECK (char_length(evidence) <= 4000),
 state text NOT NULL DEFAULT 'open' CHECK (state IN ('open', 'reviewing', 'resolved', 'dismissed')),
 created_at timestamptz NOT NULL DEFAULT clock_timestamp(),
 UNIQUE (reporter, idempotency_key)
);
CREATE INDEX v2_reports_state ON v2_reports (state, created_at);
REVOKE ALL ON "chat"."v2_callouts" FROM PUBLIC;
REVOKE ALL ON "chat"."v2_bounties" FROM PUBLIC;
REVOKE ALL ON "chat"."v2_bounty_submissions" FROM PUBLIC;
REVOKE ALL ON "chat"."v2_bounty_acceptances" FROM PUBLIC;
REVOKE ALL ON "chat"."v2_reports" FROM PUBLIC;
