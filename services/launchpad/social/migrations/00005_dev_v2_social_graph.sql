-- +goose Up

CREATE TABLE IF NOT EXISTS chat.interaction_follows(follower text,followee text,created_at timestamptz NOT NULL DEFAULT now(),PRIMARY KEY(follower,followee));

CREATE INDEX IF NOT EXISTS interaction_follows_followee_created_at_follower_idx ON chat.interaction_follows(followee,created_at DESC,follower DESC);

CREATE INDEX IF NOT EXISTS interaction_follows_follower_created_at_followee_idx ON chat.interaction_follows(follower,created_at DESC,followee DESC);

CREATE TABLE IF NOT EXISTS chat.interaction_callout_refs(id text,revision bigint,actor text,metadata bytea,PRIMARY KEY(id,revision));

CREATE TABLE IF NOT EXISTS chat.interaction_notification_intents(target text,revision bigint,recipient text,PRIMARY KEY(target,revision,recipient));

CREATE TABLE IF NOT EXISTS chat.interaction_preferences(wallet text PRIMARY KEY,inbox text NOT NULL CHECK(inbox IN ('everyone','followed_only','nobody')),revision bigint NOT NULL DEFAULT 1 CHECK(revision>0));

CREATE TABLE IF NOT EXISTS chat.interaction_blocks(owner text NOT NULL,target text NOT NULL,PRIMARY KEY(owner,target),CHECK(owner<>target));

CREATE TABLE IF NOT EXISTS chat.interaction_mutes(owner text NOT NULL,target text NOT NULL,PRIMARY KEY(owner,target));

CREATE TABLE IF NOT EXISTS chat.interaction_erasure_jobs(deletion_case_id uuid PRIMARY KEY,wallet text NOT NULL,policy_version text NOT NULL,stage integer NOT NULL DEFAULT 0,state text NOT NULL DEFAULT 'running',rows_processed bigint NOT NULL DEFAULT 0,completed_at timestamptz,hold_until timestamptz);

CREATE TABLE IF NOT EXISTS chat.interaction_erasure_receipts(deletion_case_id uuid PRIMARY KEY,wallet text NOT NULL,policy_version text NOT NULL,completed_at timestamptz NOT NULL,receipt_hash text NOT NULL);

CREATE TABLE IF NOT EXISTS chat.interaction_public_events(event_id text PRIMARY KEY,aggregate_id text NOT NULL,revision bigint NOT NULL,payload bytea NOT NULL,payload_hash text NOT NULL,created_at timestamptz NOT NULL DEFAULT now(),expires_at timestamptz NOT NULL DEFAULT now()+interval '72 hours',UNIQUE(aggregate_id,revision));

CREATE INDEX IF NOT EXISTS interaction_public_events_expires_at_event_id_idx ON chat.interaction_public_events(expires_at,event_id);

CREATE TABLE IF NOT EXISTS chat.interaction_feed_projection(generation text NOT NULL,aggregate_id text NOT NULL,revision bigint NOT NULL,actor text NOT NULL,removed boolean NOT NULL,payload bytea NOT NULL,PRIMARY KEY(generation,aggregate_id));

CREATE INDEX IF NOT EXISTS interaction_feed_projection_actor_generation_idx ON chat.interaction_feed_projection(actor,generation);

CREATE TABLE IF NOT EXISTS chat.interaction_callout_authority(id text NOT NULL,revision bigint NOT NULL,actor text NOT NULL,removed boolean NOT NULL,PRIMARY KEY(id,revision));

CREATE TABLE IF NOT EXISTS chat.chat_broker_inbox(event_id text PRIMARY KEY,payload_hash text NOT NULL,broker_stream text NOT NULL,broker_sequence bigint NOT NULL CHECK(broker_sequence>0));

CREATE TABLE IF NOT EXISTS chat.chat_live_outbox(event_id text NOT NULL REFERENCES chat.chat_broker_inbox(event_id),channel text NOT NULL,payload jsonb NOT NULL,created_at timestamptz NOT NULL DEFAULT now(),delivered_at timestamptz,PRIMARY KEY(event_id,channel));

CREATE INDEX IF NOT EXISTS chat_live_pending ON chat.chat_live_outbox(created_at,event_id) WHERE delivered_at IS NULL;

CREATE TABLE IF NOT EXISTS chat.chat_notification_intents(event_id text NOT NULL, account_id uuid NOT NULL, conversation_id text NOT NULL, message_id text NOT NULL REFERENCES chat.chat_messages(id), created_at timestamptz NOT NULL DEFAULT now(), delivered_at timestamptz, PRIMARY KEY(event_id,account_id));

CREATE INDEX IF NOT EXISTS chat_notification_pending ON chat.chat_notification_intents(created_at,event_id) WHERE delivered_at IS NULL;

CREATE TABLE IF NOT EXISTS chat.interaction_graph_revisions(wallet text PRIMARY KEY,revision bigint NOT NULL CHECK(revision>0));

INSERT INTO chat.interaction_graph_revisions(wallet,revision) SELECT wallet,1 FROM chat.interaction_preferences ON CONFLICT (wallet) DO NOTHING;

-- +goose StatementBegin
CREATE FUNCTION chat.social_asset_state(p_id uuid) RETURNS TABLE(owner_wallet text,state text,blocked bool) LANGUAGE sql SECURITY DEFINER SET search_path=pg_catalog,pg_temp AS $guard$ SELECT owner_wallet,state,blocked FROM metadata.account_assets_authority_v2 WHERE id=p_id $guard$;
-- +goose StatementEnd

REVOKE ALL ON FUNCTION chat.social_asset_state(uuid) FROM PUBLIC;

-- +goose StatementBegin
CREATE FUNCTION chat.social_canonical_pools() RETURNS TABLE(pool text,token text,creator text) LANGUAGE sql SECURITY DEFINER SET search_path=pg_catalog,pg_temp AS $guard$ SELECT pool,token,creator FROM indexer.indexer_canonical_creations_v1 $guard$;
-- +goose StatementEnd

REVOKE ALL ON FUNCTION chat.social_canonical_pools() FROM PUBLIC;

-- +goose StatementBegin
CREATE FUNCTION chat.social_graph_revision() RETURNS trigger LANGUAGE plpgsql SET search_path=pg_catalog,pg_temp AS $guard$ DECLARE a text; b text; BEGIN
IF TG_TABLE_NAME='interaction_follows' THEN a=COALESCE(NEW.follower,OLD.follower); b=COALESCE(NEW.followee,OLD.followee);
ELSIF TG_TABLE_NAME='interaction_blocks' THEN a=COALESCE(NEW.owner,OLD.owner);b=COALESCE(NEW.target,OLD.target);
ELSE a=COALESCE(NEW.wallet,OLD.wallet); END IF;
INSERT INTO chat.interaction_graph_revisions(wallet,revision)VALUES(a,1) ON CONFLICT(wallet) DO UPDATE SET revision=chat.interaction_graph_revisions.revision+1;
IF b IS NOT NULL AND b<>a THEN INSERT INTO chat.interaction_graph_revisions(wallet,revision)VALUES(b,1) ON CONFLICT(wallet) DO UPDATE SET revision=chat.interaction_graph_revisions.revision+1; END IF; RETURN NULL; END $guard$;
-- +goose StatementEnd

REVOKE ALL ON FUNCTION chat.social_graph_revision() FROM PUBLIC;

CREATE TRIGGER social_follow_revision AFTER INSERT OR DELETE ON chat.interaction_follows FOR EACH ROW EXECUTE FUNCTION chat.social_graph_revision();

CREATE TRIGGER social_block_revision AFTER INSERT OR DELETE ON chat.interaction_blocks FOR EACH ROW EXECUTE FUNCTION chat.social_graph_revision();

CREATE TRIGGER social_preference_revision AFTER INSERT OR UPDATE ON chat.interaction_preferences FOR EACH ROW EXECUTE FUNCTION chat.social_graph_revision();

CREATE TRIGGER social_erasure_revision AFTER INSERT ON chat.interaction_erased_wallets FOR EACH ROW EXECUTE FUNCTION chat.social_graph_revision();

-- +goose StatementBegin
CREATE FUNCTION chat.interaction_session_valid(p_session uuid,p_actor text) RETURNS boolean LANGUAGE plpgsql SECURITY DEFINER SET search_path=pg_catalog,pg_temp AS $guard$ DECLARE live boolean; BEGIN SELECT true INTO live FROM users.identity_sessions_v2 s JOIN users.identity_accounts_v2 a USING(account_id) JOIN users.identity_wallet_links_v2 w ON w.account_id=s.account_id AND w.wallet=s.wallet AND w.chain_id=125 AND w.state='verified' WHERE s.session_id=p_session AND s.wallet=p_actor AND s.revoked_at IS NULL AND s.access_expires_at>clock_timestamp() AND s.auth_epoch=a.auth_epoch; RETURN COALESCE(live,false); END $guard$;
-- +goose StatementEnd

REVOKE ALL ON FUNCTION chat.interaction_session_valid(uuid,text) FROM PUBLIC;

-- +goose StatementBegin
CREATE FUNCTION chat.interaction_account_active(p_actor text) RETURNS boolean LANGUAGE sql SECURITY DEFINER SET search_path=pg_catalog,pg_temp AS $guard$ SELECT COALESCE((SELECT NOT deleted FROM users.account_state_v2 WHERE wallet=p_actor),false) $guard$;
-- +goose StatementEnd

REVOKE ALL ON FUNCTION chat.interaction_account_active(text) FROM PUBLIC;

-- +goose StatementBegin
CREATE FUNCTION chat.interaction_pool_creator(p_pool text) RETURNS text LANGUAGE sql SECURITY DEFINER SET search_path=pg_catalog,pg_temp AS $guard$ SELECT creator FROM indexer.indexer_canonical_creations_v1 WHERE pool=p_pool $guard$;
-- +goose StatementEnd

REVOKE ALL ON FUNCTION chat.interaction_pool_creator(text) FROM PUBLIC;

-- +goose StatementBegin
CREATE FUNCTION chat.social_account_for_wallet(p_actor text) RETURNS uuid LANGUAGE sql SECURITY DEFINER SET search_path=pg_catalog,pg_temp AS $guard$ SELECT DISTINCT w.account_id FROM users.identity_wallet_links_v2 w JOIN users.account_state_v2 a USING(account_id) WHERE w.wallet=p_actor AND w.chain_id=125 AND w.state='verified' AND NOT a.deleted $guard$;
-- +goose StatementEnd

REVOKE ALL ON FUNCTION chat.social_account_for_wallet(text) FROM PUBLIC;

-- +goose StatementBegin
CREATE FUNCTION chat.social_wallet_for_account(p_account uuid) RETURNS text LANGUAGE sql SECURITY DEFINER SET search_path=pg_catalog,pg_temp AS $guard$ SELECT a.wallet FROM users.account_state_v2 a JOIN users.identity_wallet_links_v2 w ON w.account_id=a.account_id AND w.wallet=a.wallet AND w.chain_id=125 AND w.state='verified' WHERE a.account_id=p_account AND NOT a.deleted $guard$;
-- +goose StatementEnd

REVOKE ALL ON FUNCTION chat.social_wallet_for_account(uuid) FROM PUBLIC;

-- +goose StatementBegin
CREATE FUNCTION chat.social_current_source_order() RETURNS TABLE(height text,transaction_index text,log_index text) LANGUAGE sql SECURITY DEFINER SET search_path=pg_catalog,pg_temp AS $guard$ SELECT l.height::text,l.transaction_index::text,l.log_index::text FROM indexer.indexer_raw_logs_v1 l JOIN indexer.indexer_block_completeness_v1 b ON b.chain_id=l.chain_id AND b.height=l.height AND b.block_hash=l.block_hash WHERE l.chain_id=125 AND b.fetch_complete AND b.required_decode_complete ORDER BY l.height DESC,l.transaction_index DESC,l.log_index DESC LIMIT 1 $guard$;
-- +goose StatementEnd

REVOKE ALL ON FUNCTION chat.social_current_source_order() FROM PUBLIC;

-- +goose StatementBegin
CREATE FUNCTION chat.social_receipt_committed(p_hash text) RETURNS boolean LANGUAGE sql SECURITY DEFINER SET search_path=pg_catalog,pg_temp AS $guard$ SELECT EXISTS(SELECT 1 FROM indexer.indexer_raw_logs_v1 l JOIN indexer.indexer_block_completeness_v1 b ON b.chain_id=l.chain_id AND b.height=l.height AND b.block_hash=l.block_hash WHERE l.chain_id=125 AND l.transaction_hash=p_hash AND b.fetch_complete AND b.required_decode_complete) $guard$;
-- +goose StatementEnd

REVOKE ALL ON FUNCTION chat.social_receipt_committed(text) FROM PUBLIC;

REVOKE ALL ON chat.interaction_follows FROM PUBLIC;

REVOKE ALL ON chat.interaction_callout_refs FROM PUBLIC;

REVOKE ALL ON chat.interaction_notification_intents FROM PUBLIC;

REVOKE ALL ON chat.interaction_preferences FROM PUBLIC;

REVOKE ALL ON chat.interaction_blocks FROM PUBLIC;

REVOKE ALL ON chat.interaction_mutes FROM PUBLIC;

REVOKE ALL ON chat.interaction_erasure_jobs FROM PUBLIC;

REVOKE ALL ON chat.interaction_erasure_receipts FROM PUBLIC;

REVOKE ALL ON chat.interaction_public_events FROM PUBLIC;

REVOKE ALL ON chat.interaction_feed_projection FROM PUBLIC;

REVOKE ALL ON chat.interaction_callout_authority FROM PUBLIC;

REVOKE ALL ON chat.chat_broker_inbox FROM PUBLIC;

REVOKE ALL ON chat.chat_live_outbox FROM PUBLIC;

REVOKE ALL ON chat.chat_notification_intents FROM PUBLIC;

REVOKE ALL ON chat.interaction_graph_revisions FROM PUBLIC;


-- +goose Down
DROP FUNCTION IF EXISTS chat.social_receipt_committed(text);
DROP FUNCTION IF EXISTS chat.social_current_source_order();
DROP FUNCTION IF EXISTS chat.social_wallet_for_account(uuid);
DROP FUNCTION IF EXISTS chat.social_account_for_wallet(text);
DROP FUNCTION IF EXISTS chat.interaction_pool_creator(text);
DROP FUNCTION IF EXISTS chat.interaction_account_active(text);
DROP FUNCTION IF EXISTS chat.interaction_session_valid(uuid,text);
DROP TRIGGER IF EXISTS social_erasure_revision ON chat.interaction_erased_wallets;
DROP TRIGGER IF EXISTS social_preference_revision ON chat.interaction_preferences;
DROP TRIGGER IF EXISTS social_block_revision ON chat.interaction_blocks;
DROP TRIGGER IF EXISTS social_follow_revision ON chat.interaction_follows;
DROP FUNCTION IF EXISTS chat.social_graph_revision();
DROP FUNCTION IF EXISTS chat.social_canonical_pools();
DROP FUNCTION IF EXISTS chat.social_asset_state(uuid);
DROP TABLE IF EXISTS chat.interaction_graph_revisions;
DROP TABLE IF EXISTS chat.chat_notification_intents;
DROP TABLE IF EXISTS chat.chat_live_outbox;
DROP TABLE IF EXISTS chat.chat_broker_inbox;
DROP TABLE IF EXISTS chat.interaction_callout_authority;
DROP TABLE IF EXISTS chat.interaction_feed_projection;
DROP TABLE IF EXISTS chat.interaction_public_events;
DROP TABLE IF EXISTS chat.interaction_erasure_receipts;
DROP TABLE IF EXISTS chat.interaction_erasure_jobs;
DROP TABLE IF EXISTS chat.interaction_mutes;
DROP TABLE IF EXISTS chat.interaction_blocks;
DROP TABLE IF EXISTS chat.interaction_preferences;
DROP TABLE IF EXISTS chat.interaction_notification_intents;
DROP TABLE IF EXISTS chat.interaction_callout_refs;
DROP TABLE IF EXISTS chat.interaction_follows;
