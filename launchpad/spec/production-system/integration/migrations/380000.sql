SET LOCAL search_path = "chat", pg_catalog;
-- Coordinator-owned migration proposal, not auto-applied by application code.
CREATE TABLE safety_cases (
 id uuid PRIMARY KEY, target_type text NOT NULL, target_id text NOT NULL,
 pool_id text NOT NULL DEFAULT '', affected_wallet text NOT NULL DEFAULT '',
 reporter text NOT NULL, category text NOT NULL, description text NOT NULL,
 state text NOT NULL DEFAULT 'open', revision bigint NOT NULL DEFAULT 1,
 created_at timestamptz NOT NULL DEFAULT now(),
 UNIQUE(reporter,target_type,target_id), CHECK(revision>0),
 CHECK(length(target_id)<=160 AND length(description)<=4000)
);
CREATE TABLE safety_actions (
 id uuid PRIMARY KEY, case_id uuid REFERENCES safety_cases(id), actor text NOT NULL,
 session_id text NOT NULL, actor_role text NOT NULL, actor_scope jsonb NOT NULL, kind text NOT NULL, reason text NOT NULL,
 policy_version text NOT NULL, revision bigint NOT NULL, evidence_hash text NOT NULL,
 created_at timestamptz NOT NULL DEFAULT now(), CHECK(length(reason)<=1000)
);
CREATE TABLE safety_bans (
 id uuid PRIMARY KEY, wallet text NOT NULL, scope text NOT NULL,
 resource text NOT NULL, actor text NOT NULL, expires_at timestamptz,
 revoked_at timestamptz, revision bigint NOT NULL DEFAULT 1,
 CHECK(scope IN ('global','pool','dm','voice'))
);
CREATE TABLE safety_appeals (
 id uuid PRIMARY KEY, action_id uuid NOT NULL UNIQUE REFERENCES safety_actions(id),
 appellant text NOT NULL, reason text NOT NULL, state text NOT NULL DEFAULT 'submitted',
 revision bigint NOT NULL DEFAULT 1, reviewer text, decided_at timestamptz
);
CREATE TABLE safety_outbox (
 id uuid PRIMARY KEY, kind text NOT NULL, entity_id text NOT NULL,
 revision bigint NOT NULL, payload jsonb NOT NULL,
 created_at timestamptz NOT NULL DEFAULT now(), published_at timestamptz
);
CREATE TABLE voice_rooms (
 id uuid PRIMARY KEY, pool_id text NOT NULL, host_wallet text NOT NULL,
 state text NOT NULL DEFAULT 'draft', revision bigint NOT NULL DEFAULT 1,
 provider_session text NOT NULL DEFAULT '', start_key text NOT NULL UNIQUE,
 recording text NOT NULL DEFAULT 'off', updated_at timestamptz NOT NULL DEFAULT now(),
 CHECK(state IN ('draft','live','ending','ended')),
 CHECK(recording='off')
);
CREATE TABLE voice_participants (
 room_id uuid NOT NULL REFERENCES voice_rooms(id), wallet text NOT NULL,
 role text NOT NULL DEFAULT 'listener', muted boolean NOT NULL DEFAULT false,
 revoked boolean NOT NULL DEFAULT false, grant_state text NOT NULL DEFAULT 'active', revision bigint NOT NULL DEFAULT 1,
 PRIMARY KEY(room_id,wallet),
 CHECK(role IN ('listener','requested_speaker','speaker','moderator','host'))
);
-- Append-only even for ordinary UPDATE/DELETE attempts. Runtime role must also
-- have no TRUNCATE/DDL privilege; coordinator registers role grants separately.
CREATE FUNCTION safety_append_only() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN RAISE EXCEPTION 'safety actions are append only'; END $$;
CREATE TRIGGER safety_actions_immutable BEFORE UPDATE OR DELETE ON safety_actions
FOR EACH ROW EXECUTE FUNCTION safety_append_only();

CREATE TABLE safety_visibility (
 target_type text NOT NULL, target_id text NOT NULL, revision bigint NOT NULL,
 status text NOT NULL CHECK(status IN ('removed','restored')),
 case_id uuid NOT NULL REFERENCES safety_cases(id), PRIMARY KEY(target_type,target_id)
);
