SET LOCAL search_path = "chat", pg_catalog;
CREATE TABLE safety_action_bans (
 action_id uuid NOT NULL REFERENCES safety_actions(id), ban_id uuid NOT NULL REFERENCES safety_bans(id),
 PRIMARY KEY(action_id,ban_id)
);
CREATE TABLE voice_recordings (
 id uuid PRIMARY KEY,room_id uuid NOT NULL REFERENCES voice_rooms(id),
 state text NOT NULL CHECK(state IN ('consent_pending','starting','recording','stopping','stopped')),
 purpose text NOT NULL CHECK(length(purpose) BETWEEN 1 AND 1000),
 policy_version text NOT NULL CHECK(length(policy_version) BETWEEN 1 AND 160),
 revision bigint NOT NULL DEFAULT 1 CHECK(revision>0),provider_id text NOT NULL DEFAULT '',
 deleted_at timestamptz,delete_at timestamptz NOT NULL,created_at timestamptz NOT NULL DEFAULT now()
);
CREATE UNIQUE INDEX voice_recordings_active ON voice_recordings(room_id) WHERE state IN ('consent_pending','starting','recording','stopping');
CREATE TABLE voice_consents (
 recording_id uuid NOT NULL REFERENCES voice_recordings(id),wallet text NOT NULL,agreed boolean NOT NULL,
 purpose text NOT NULL,policy_version text NOT NULL,session_id text NOT NULL,created_at timestamptz NOT NULL DEFAULT now(),
 PRIMARY KEY(recording_id,wallet)
);
CREATE INDEX voice_reconciliation_pending ON voice_rooms(updated_at,id) WHERE state IN ('draft','live','ending');
CREATE INDEX voice_revocations_pending ON voice_participants(room_id,wallet) WHERE grant_state='revoking';
CREATE INDEX voice_recording_deletion ON voice_recordings(delete_at,id) WHERE state='stopped';
CREATE INDEX safety_bans_active_lookup ON safety_bans(wallet,scope,resource) WHERE revoked_at IS NULL;
CREATE TABLE safety_moderator_sessions (
 session_id uuid PRIMARY KEY,account_id uuid NOT NULL,wallet text NOT NULL,
 role text NOT NULL CHECK(role IN ('reviewer','room_moderator','global_administrator')),
 expires_at timestamptz NOT NULL,step_up_at timestamptz
);
CREATE TABLE safety_moderator_scopes (
 session_id uuid NOT NULL REFERENCES safety_moderator_sessions(session_id),pool_id text NOT NULL DEFAULT '',case_id uuid REFERENCES safety_cases(id),
 CHECK(pool_id<>'' OR case_id IS NOT NULL)
);
CREATE INDEX safety_moderator_scope_session ON safety_moderator_scopes(session_id);
CREATE TABLE voice_ejections (
 room_id uuid NOT NULL REFERENCES voice_rooms(id),wallet text NOT NULL,actor text NOT NULL,
 session_id text NOT NULL,created_at timestamptz NOT NULL DEFAULT now(),PRIMARY KEY(room_id,wallet)
);
CREATE TABLE voice_recording_events (
 id bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,recording_id uuid NOT NULL REFERENCES voice_recordings(id),
 room_id uuid NOT NULL REFERENCES voice_rooms(id),actor text NOT NULL,session_id text NOT NULL,
 kind text NOT NULL,payload jsonb NOT NULL,created_at timestamptz NOT NULL DEFAULT now()
);
CREATE TRIGGER voice_recording_events_immutable BEFORE UPDATE OR DELETE ON voice_recording_events
 FOR EACH ROW EXECUTE FUNCTION safety_append_only();
REVOKE ALL ON "chat"."safety_action_bans" FROM PUBLIC;
REVOKE ALL ON "chat"."voice_recordings" FROM PUBLIC;
REVOKE ALL ON "chat"."voice_consents" FROM PUBLIC;
REVOKE ALL ON "chat"."safety_moderator_sessions" FROM PUBLIC;
REVOKE ALL ON "chat"."safety_moderator_scopes" FROM PUBLIC;
REVOKE ALL ON "chat"."voice_ejections" FROM PUBLIC;
REVOKE ALL ON "chat"."voice_recording_events" FROM PUBLIC;
CREATE VIEW chat.safety_outbox_v2 AS SELECT id,kind,entity_id,revision,payload,created_at FROM chat.safety_outbox WHERE kind='visibility_changed';
REVOKE ALL ON chat.safety_outbox_v2 FROM PUBLIC;
