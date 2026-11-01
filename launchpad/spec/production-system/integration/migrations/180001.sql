SET LOCAL search_path = "pnl", pg_catalog;
CREATE TABLE l18_requests (
 actor text NOT NULL, request_key uuid NOT NULL, digest text NOT NULL,
 method text NOT NULL, path text NOT NULL, response bytea NOT NULL,
 expires_at timestamptz NOT NULL, release_digest text NOT NULL, interface_digest text NOT NULL,
 PRIMARY KEY(actor,request_key)
);
CREATE TABLE l18_audit (
 identity text PRIMARY KEY, actor text NOT NULL, action text NOT NULL,
 reason text NOT NULL CHECK(length(reason) BETWEEN 1 AND 512), payload jsonb NOT NULL,
 created_at timestamptz NOT NULL DEFAULT clock_timestamp()
);
CREATE TABLE l18_payout_attempts (
 award_id text NOT NULL REFERENCES l18_awards(identity), attempt integer NOT NULL CHECK(attempt>0),
 tx_hash text NOT NULL UNIQUE, state text NOT NULL CHECK(state IN ('prepared','pending_investigation','included_success','included_reverted')),
 raw numeric(78,0) NOT NULL CHECK(raw>0), beneficiary text NOT NULL, actor text NOT NULL,
 reason text NOT NULL, receipt_identity text UNIQUE, created_at timestamptz NOT NULL DEFAULT clock_timestamp(),
 PRIMARY KEY(award_id,attempt)
);
CREATE TABLE l18_corrections (
 identity uuid PRIMARY KEY, award_id text NOT NULL REFERENCES l18_awards(identity),
 proposer text NOT NULL, approver text, reason text NOT NULL, approval_reason text,
 state text NOT NULL CHECK(state IN ('proposed','approved')),
 CHECK(approver IS NULL OR approver<>proposer), created_at timestamptz NOT NULL DEFAULT clock_timestamp()
);
CREATE INDEX l18_awards_wallet_page ON l18_awards(beneficiary,identity);
CREATE INDEX l18_pending_payouts ON l18_payout_attempts(state,created_at) WHERE state IN ('prepared','pending_investigation');
CREATE TABLE l18_analytics (
 identity uuid PRIMARY KEY, campaign text NOT NULL, actor text NOT NULL,
 day date NOT NULL, short_code text NOT NULL, kind text NOT NULL CHECK(kind IN ('view','click')),
 created_at timestamptz NOT NULL DEFAULT clock_timestamp()
);
CREATE INDEX l18_analytics_day ON l18_analytics(campaign,actor,day);
CREATE TABLE l18_community_history (
 revision bigserial PRIMARY KEY, request_id uuid NOT NULL UNIQUE,
 nft_id numeric(78,0) NOT NULL, state text NOT NULL CHECK(state IN ('proposed','proof_pending','active','revoked')),
 owner text NOT NULL, policy_digest text NOT NULL, service_identity text NOT NULL, block_number bigint NOT NULL,
 block_hash text NOT NULL, payload jsonb NOT NULL, created_at timestamptz NOT NULL DEFAULT clock_timestamp()
);
CREATE INDEX l18_community_nft_history ON l18_community_history(nft_id,revision);
REVOKE ALL ON "pnl"."l18_requests" FROM PUBLIC;
REVOKE ALL ON "pnl"."l18_audit" FROM PUBLIC;
REVOKE ALL ON "pnl"."l18_payout_attempts" FROM PUBLIC;
REVOKE ALL ON "pnl"."l18_corrections" FROM PUBLIC;
REVOKE ALL ON "pnl"."l18_analytics" FROM PUBLIC;
REVOKE ALL ON "pnl"."l18_community_history" FROM PUBLIC;
CREATE TABLE l18_referral_codes (
 campaign text NOT NULL, beneficiary text NOT NULL, short_code varchar(16) NOT NULL UNIQUE,
 created_at bigint NOT NULL, PRIMARY KEY(campaign,beneficiary)
);
CREATE TABLE l18_referral_consents (
 revision bigserial PRIMARY KEY, campaign text NOT NULL, beneficiary text NOT NULL,
 consent boolean NOT NULL, recorded_at bigint NOT NULL, request_id text NOT NULL UNIQUE
);
CREATE INDEX l18_referral_consents_current ON l18_referral_consents(campaign,beneficiary,revision DESC);
CREATE TABLE l18_referral_requests (
 actor text NOT NULL, request_id uuid NOT NULL, operation text NOT NULL, digest text NOT NULL,
 response jsonb NOT NULL, created_at timestamptz NOT NULL DEFAULT clock_timestamp(), PRIMARY KEY(actor,request_id)
);

REVOKE ALL ON pnl.l18_referral_codes,pnl.l18_referral_consents,pnl.l18_referral_requests FROM PUBLIC;
CREATE TABLE l18_community_scan (
 nft_id numeric(78,0) PRIMARY KEY, checked_at timestamptz NOT NULL DEFAULT clock_timestamp()
);

REVOKE ALL ON pnl.l18_community_scan FROM PUBLIC;
