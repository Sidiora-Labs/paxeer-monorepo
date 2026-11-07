SET LOCAL search_path = "users", pg_catalog;
ALTER TABLE identity_tickets ADD COLUMN binding jsonb;
CREATE TABLE identity_challenge_requests(wallet text NOT NULL,request_id uuid NOT NULL,body_hash bytea NOT NULL,response jsonb NOT NULL,retain_until timestamptz NOT NULL,PRIMARY KEY(wallet,request_id));
ALTER TABLE users.identity_challenges DROP CONSTRAINT identity_challenges_purpose_check;
ALTER TABLE users.identity_challenges ADD CONSTRAINT identity_challenges_purpose_check CHECK(purpose IN ('login','link_wallet','step_up','step_up:account.delete'));
ALTER TABLE users.identity_reauth DROP CONSTRAINT identity_reauth_action_check;
ALTER TABLE users.identity_reauth ADD CONSTRAINT identity_reauth_action_check CHECK(action IN ('wallet_unlink','account.delete'));
CREATE FUNCTION users.consume_account_deletion_reauth(proof bytea, account uuid, session uuid) RETURNS void LANGUAGE plpgsql SECURITY DEFINER SET search_path = pg_catalog AS $$
DECLARE current_epoch bigint; consumed bytea;
BEGIN
 SELECT auth_epoch INTO current_epoch FROM users.identity_accounts WHERE account_id=account FOR UPDATE;
 IF NOT FOUND THEN RAISE EXCEPTION 'identity authority unavailable' USING ERRCODE='28000'; END IF;
 PERFORM 1 FROM users.identity_sessions WHERE session_id=session AND account_id=account AND auth_epoch=current_epoch AND revoked_at IS NULL AND access_expires_at>clock_timestamp() FOR UPDATE;
 IF NOT FOUND THEN RAISE EXCEPTION 'identity session unavailable' USING ERRCODE='28000'; END IF;
 UPDATE users.identity_reauth SET consumed_at=clock_timestamp() WHERE proof_hash=proof AND account_id=account AND session_id=session AND action='account.delete' AND consumed_at IS NULL AND issued_at<=clock_timestamp() AND expires_at>clock_timestamp() RETURNING proof_hash INTO consumed;
 IF NOT FOUND THEN RAISE EXCEPTION 'reauthentication unavailable' USING ERRCODE='28000'; END IF;
END $$;
CREATE FUNCTION users.revoke_account_sessions(account uuid) RETURNS void LANGUAGE plpgsql SECURITY DEFINER SET search_path = pg_catalog AS $$
DECLARE current_epoch bigint;
BEGIN
 SELECT auth_epoch INTO current_epoch FROM users.identity_accounts WHERE account_id=account FOR UPDATE;
 IF NOT FOUND OR current_epoch=9223372036854775807 THEN RAISE EXCEPTION 'identity epoch unavailable' USING ERRCODE='28000'; END IF;
 UPDATE users.identity_accounts SET auth_epoch=current_epoch+1 WHERE account_id=account;
 UPDATE users.identity_sessions SET revoked_at=clock_timestamp() WHERE account_id=account AND revoked_at IS NULL;
 INSERT INTO users.identity_outbox(event_id,account_id,session_id,action,created_at) VALUES(gen_random_uuid(),account,NULL,'account.delete',clock_timestamp());
END $$;
REVOKE ALL ON FUNCTION users.consume_account_deletion_reauth(bytea,uuid,uuid) FROM PUBLIC;
REVOKE ALL ON FUNCTION users.revoke_account_sessions(uuid) FROM PUBLIC;
