SET LOCAL search_path = security, pg_catalog;
CREATE TABLE security.operator_enrolments (
 actor uuid PRIMARY KEY,
 wallet text NOT NULL,
 role text NOT NULL CHECK (role IN ('infrastructure','finance','moderator','support')),
 scopes text[] NOT NULL CHECK (cardinality(scopes) BETWEEN 1 AND 32),
 factor bytea NOT NULL,
 last_step bigint NOT NULL DEFAULT 0,
 enrolled_at timestamptz NOT NULL DEFAULT clock_timestamp(),
 revoked_at timestamptz
);
REVOKE ALL ON security.operator_enrolments FROM PUBLIC;
