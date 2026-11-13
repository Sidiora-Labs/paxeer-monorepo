SET LOCAL search_path = "chat", pg_catalog;

ALTER TABLE chat_members ADD COLUMN muted boolean NOT NULL DEFAULT false;
ALTER TABLE chat_members ADD COLUMN notification_enabled boolean NOT NULL DEFAULT false;
CREATE TABLE chat_broker_inbox(event_id text PRIMARY KEY,payload_hash text NOT NULL,broker_stream text NOT NULL,broker_sequence bigint NOT NULL CHECK(broker_sequence>0));
CREATE TABLE chat_live_outbox(event_id text NOT NULL REFERENCES chat_broker_inbox(event_id),channel text NOT NULL,payload jsonb NOT NULL,created_at timestamptz NOT NULL DEFAULT now(),delivered_at timestamptz,PRIMARY KEY(event_id,channel));
CREATE INDEX chat_live_pending ON chat_live_outbox(created_at,event_id) WHERE delivered_at IS NULL;
CREATE TABLE chat_control_outbox(event_id text PRIMARY KEY, wallet text NOT NULL, payload jsonb NOT NULL, created_at timestamptz NOT NULL DEFAULT now(), delivered_at timestamptz);
CREATE INDEX chat_control_pending ON chat_control_outbox(created_at,event_id) WHERE delivered_at IS NULL;
CREATE TABLE chat_notification_intents(event_id text NOT NULL, account_id uuid NOT NULL, conversation_id text NOT NULL, message_id text NOT NULL REFERENCES chat_messages(id), created_at timestamptz NOT NULL DEFAULT now(), delivered_at timestamptz, PRIMARY KEY(event_id,account_id));
CREATE INDEX chat_notification_pending ON chat_notification_intents(created_at,event_id) WHERE delivered_at IS NULL;


CREATE FUNCTION chat.social_asset_state(p_id uuid) RETURNS TABLE(owner_wallet text,state text,blocked bool) LANGUAGE sql SECURITY DEFINER SET search_path=pg_catalog,pg_temp AS $guard$ SELECT owner_wallet,state,blocked FROM metadata.account_assets_authority_v2 WHERE id=p_id $guard$;
REVOKE ALL ON FUNCTION chat.social_asset_state(uuid) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION chat.social_asset_state(uuid) TO chat_runtime;
CREATE FUNCTION chat.social_canonical_pools() RETURNS TABLE(pool text,token text,creator text) LANGUAGE sql SECURITY DEFINER SET search_path=pg_catalog,pg_temp AS $guard$ SELECT pool,token,creator FROM indexer.indexer_canonical_creations_v1 $guard$;
REVOKE ALL ON FUNCTION chat.social_canonical_pools() FROM PUBLIC;
GRANT EXECUTE ON FUNCTION chat.social_canonical_pools() TO chat_runtime;
CREATE TABLE chat.interaction_graph_revisions(wallet text PRIMARY KEY,revision bigint NOT NULL CHECK(revision>0));
INSERT INTO chat.interaction_graph_revisions(wallet,revision) SELECT wallet,1 FROM chat.interaction_preferences;
CREATE FUNCTION chat.social_graph_revision() RETURNS trigger LANGUAGE plpgsql SET search_path=pg_catalog,pg_temp AS $guard$ DECLARE a text; b text; BEGIN
IF TG_TABLE_NAME='interaction_follows' THEN a=COALESCE(NEW.follower,OLD.follower); b=COALESCE(NEW.followee,OLD.followee);
ELSIF TG_TABLE_NAME='interaction_blocks' THEN a=COALESCE(NEW.owner,OLD.owner);b=COALESCE(NEW.target,OLD.target);
ELSE a=COALESCE(NEW.wallet,OLD.wallet); END IF;
INSERT INTO chat.interaction_graph_revisions(wallet,revision)VALUES(a,1) ON CONFLICT(wallet) DO UPDATE SET revision=chat.interaction_graph_revisions.revision+1;
IF b IS NOT NULL AND b<>a THEN INSERT INTO chat.interaction_graph_revisions(wallet,revision)VALUES(b,1) ON CONFLICT(wallet) DO UPDATE SET revision=chat.interaction_graph_revisions.revision+1; END IF; RETURN NULL; END $guard$;
CREATE TRIGGER social_follow_revision AFTER INSERT OR DELETE ON chat.interaction_follows FOR EACH ROW EXECUTE FUNCTION chat.social_graph_revision();
CREATE TRIGGER social_block_revision AFTER INSERT OR DELETE ON chat.interaction_blocks FOR EACH ROW EXECUTE FUNCTION chat.social_graph_revision();
CREATE TRIGGER social_preference_revision AFTER INSERT OR UPDATE ON chat.interaction_preferences FOR EACH ROW EXECUTE FUNCTION chat.social_graph_revision();
CREATE TRIGGER social_erasure_revision AFTER INSERT ON chat.interaction_erased_wallets FOR EACH ROW EXECUTE FUNCTION chat.social_graph_revision();
CREATE FUNCTION chat.interaction_session_valid(p_session uuid,p_actor text) RETURNS boolean LANGUAGE plpgsql SECURITY DEFINER SET search_path=pg_catalog,pg_temp AS $guard$ DECLARE live boolean; BEGIN SELECT true INTO live FROM users.identity_sessions_v2 s JOIN users.identity_accounts_v2 a USING(account_id) JOIN users.identity_wallet_links_v2 w ON w.account_id=s.account_id AND w.wallet=s.wallet AND w.chain_id=125 AND w.state='verified' WHERE s.session_id=p_session AND s.wallet=p_actor AND s.revoked_at IS NULL AND s.access_expires_at>clock_timestamp() AND s.auth_epoch=a.auth_epoch; RETURN COALESCE(live,false); END $guard$;
CREATE FUNCTION chat.interaction_account_active(p_actor text) RETURNS boolean LANGUAGE sql SECURITY DEFINER SET search_path=pg_catalog,pg_temp AS $guard$ SELECT COALESCE((SELECT NOT deleted FROM users.account_state_v2 WHERE wallet=p_actor),false) $guard$;
CREATE FUNCTION chat.interaction_pool_creator(p_pool text) RETURNS text LANGUAGE sql SECURITY DEFINER SET search_path=pg_catalog,pg_temp AS $guard$ SELECT creator FROM indexer.indexer_canonical_creations_v1 WHERE pool=p_pool $guard$;
CREATE FUNCTION chat.social_account_for_wallet(p_actor text) RETURNS uuid LANGUAGE sql SECURITY DEFINER SET search_path=pg_catalog,pg_temp AS $guard$ SELECT DISTINCT w.account_id FROM users.identity_wallet_links_v2 w JOIN users.account_state_v2 a USING(account_id) WHERE w.wallet=p_actor AND w.chain_id=125 AND w.state='verified' AND NOT a.deleted $guard$;
CREATE FUNCTION chat.social_wallet_for_account(p_account uuid) RETURNS text LANGUAGE sql SECURITY DEFINER SET search_path=pg_catalog,pg_temp AS $guard$ SELECT a.wallet FROM users.account_state_v2 a JOIN users.identity_wallet_links_v2 w ON w.account_id=a.account_id AND w.wallet=a.wallet AND w.chain_id=125 AND w.state='verified' WHERE a.account_id=p_account AND NOT a.deleted $guard$;
CREATE FUNCTION chat.social_current_source_order() RETURNS TABLE(height text,transaction_index text,log_index text) LANGUAGE sql SECURITY DEFINER SET search_path=pg_catalog,pg_temp AS $guard$ SELECT l.height::text,l.transaction_index::text,l.log_index::text FROM indexer.indexer_raw_logs_v1 l JOIN indexer.indexer_block_completeness_v1 b ON b.chain_id=l.chain_id AND b.height=l.height AND b.block_hash=l.block_hash WHERE l.chain_id=125 AND b.fetch_complete AND b.required_decode_complete ORDER BY l.height DESC,l.transaction_index DESC,l.log_index DESC LIMIT 1 $guard$;
CREATE FUNCTION chat.social_receipt_committed(p_hash text) RETURNS boolean LANGUAGE sql SECURITY DEFINER SET search_path=pg_catalog,pg_temp AS $guard$ SELECT EXISTS(SELECT 1 FROM indexer.indexer_raw_logs_v1 l JOIN indexer.indexer_block_completeness_v1 b ON b.chain_id=l.chain_id AND b.height=l.height AND b.block_hash=l.block_hash WHERE l.chain_id=125 AND l.transaction_hash=p_hash AND b.fetch_complete AND b.required_decode_complete) $guard$;
REVOKE ALL ON FUNCTION chat.interaction_session_valid(uuid,text),chat.interaction_account_active(text),chat.interaction_pool_creator(text),chat.social_account_for_wallet(text),chat.social_wallet_for_account(uuid),chat.social_current_source_order(),chat.social_receipt_committed(text) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION chat.interaction_session_valid(uuid,text),chat.interaction_account_active(text),chat.interaction_pool_creator(text),chat.social_account_for_wallet(text),chat.social_wallet_for_account(uuid),chat.social_current_source_order(),chat.social_receipt_committed(text) TO chat_runtime;

REVOKE ALL ON chat.interaction_graph_revisions FROM PUBLIC;
REVOKE ALL ON chat.chat_broker_inbox FROM PUBLIC;
REVOKE ALL ON chat.chat_live_outbox FROM PUBLIC;
REVOKE ALL ON chat.chat_control_outbox FROM PUBLIC;
REVOKE ALL ON chat.chat_notification_intents FROM PUBLIC;
