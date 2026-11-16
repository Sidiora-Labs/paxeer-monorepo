SET LOCAL search_path = users, pg_catalog;
CREATE VIEW users.account_search_events_v2 AS
 SELECT o.event_id,o.kind,o.payload,o.revision AS source_revision,a.wallet,
 COALESCE(a.profile->>'displayName','') AS display_name,a.revision,a.deleted,a.updated_at,v.public
 FROM users.account_outbox o JOIN users.account_state a USING(account_id)
 JOIN users.account_public_search_v2 v ON v.wallet=a.wallet;
REVOKE ALL ON users.account_search_events_v2 FROM PUBLIC;
