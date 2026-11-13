SET LOCAL search_path = "users", pg_catalog;
SET LOCAL search_path = "users", pg_catalog;
CREATE TABLE identity_owner_requests(account_id uuid NOT NULL REFERENCES identity_accounts,request_id uuid NOT NULL,operation text NOT NULL,body_hash bytea NOT NULL,response jsonb NOT NULL,retain_until timestamptz NOT NULL,PRIMARY KEY(account_id,request_id,operation));
CREATE TABLE identity_revisions(account_id uuid PRIMARY KEY REFERENCES identity_accounts,revision bigint NOT NULL CHECK(revision>0));
CREATE TABLE identity_revision_inbox(event_id text PRIMARY KEY,account_id uuid NOT NULL,revision bigint NOT NULL CHECK(revision>0),payload_hash text NOT NULL);
CREATE TABLE identity_remote_revisions(account_id uuid PRIMARY KEY,revision bigint NOT NULL CHECK(revision>0));
CREATE FUNCTION identity_verified_at_immutable() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN IF OLD.linked_at IS NOT NULL AND OLD.linked_at IS DISTINCT FROM NEW.linked_at THEN RAISE EXCEPTION 'verified timestamp immutable' USING ERRCODE='23514'; END IF; RETURN NEW; END $$;
CREATE TRIGGER identity_verified_at_immutable BEFORE UPDATE OF linked_at ON identity_wallet_links FOR EACH ROW EXECUTE FUNCTION identity_verified_at_immutable();
REVOKE ALL ON FUNCTION identity_verified_at_immutable() FROM PUBLIC;
REVOKE ALL ON "users"."identity_owner_requests" FROM PUBLIC;
REVOKE ALL ON "users"."identity_revisions" FROM PUBLIC;
REVOKE ALL ON "users"."identity_revision_inbox" FROM PUBLIC;
REVOKE ALL ON "users"."identity_remote_revisions" FROM PUBLIC;
REVOKE ALL ON FUNCTION "users"."identity_verified_at_immutable"() FROM PUBLIC;
CREATE VIEW users.identity_sessions_v2 AS SELECT session_id,account_id,wallet,auth_epoch,access_hash,revoked_at,access_expires_at FROM users.identity_sessions;
REVOKE ALL ON users.identity_sessions_v2 FROM PUBLIC;
CREATE VIEW users.identity_accounts_v2 AS SELECT account_id,auth_epoch FROM users.identity_accounts;
REVOKE ALL ON users.identity_accounts_v2 FROM PUBLIC;
CREATE VIEW users.identity_wallet_links_v2 AS SELECT account_id,wallet,chain_id,state,visibility FROM users.identity_wallet_links;
REVOKE ALL ON users.identity_wallet_links_v2 FROM PUBLIC;
CREATE FUNCTION users.identity_guard_v2(p_session uuid,p_wallet text) RETURNS TABLE(account_id uuid,session_id uuid,wallet text,auth_epoch bigint,proof_at timestamptz,access_expires_at timestamptz) LANGUAGE sql SECURITY DEFINER SET search_path=pg_catalog SET lock_timeout='350ms' AS $guard$
SELECT a.account_id,s.session_id,s.wallet,a.auth_epoch,s.reauthenticated_at,s.access_expires_at
FROM users.identity_sessions s JOIN users.identity_accounts a USING(account_id)
JOIN users.identity_wallet_links w ON w.account_id=a.account_id AND w.wallet=s.wallet AND w.chain_id=125 AND w.state='verified'
WHERE s.session_id=p_session AND s.wallet=p_wallet AND p_wallet ~ '^0x[0-9a-f]{40}$' AND s.revoked_at IS NULL AND s.access_expires_at>clock_timestamp() AND s.auth_epoch=a.auth_epoch
FOR SHARE OF a,s,w $guard$;
REVOKE ALL ON FUNCTION users.identity_guard_v2(uuid,text) FROM PUBLIC;
