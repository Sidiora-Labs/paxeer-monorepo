SET LOCAL search_path = security, pg_catalog;
CREATE VIEW security.operator_session_authority_v1 WITH (security_barrier=true) AS
 SELECT session_id,wallet,actor,audience,issued_at,expires_at,mfa_expires_at,scopes,revoked
 FROM security.operator_sessions;
REVOKE ALL ON security.operator_session_authority_v1 FROM PUBLIC;
CREATE FUNCTION security.record_authorization_decision_v1(
 decision_actor text, decision_scope text, decision_request text, decision_policy text, decision_allowed boolean
) RETURNS void LANGUAGE sql SECURITY DEFINER SET search_path = pg_catalog, pg_temp AS $$
 INSERT INTO security.authorization_decisions(actor,scope,request_id,policy_version,allowed)
 VALUES(decision_actor,decision_scope,decision_request,decision_policy,decision_allowed)
$$;
REVOKE ALL ON FUNCTION security.record_authorization_decision_v1(text,text,text,text,boolean) FROM PUBLIC;
