CREATE TABLE IF NOT EXISTS livestream.stream_operations(
 id text PRIMARY KEY,wallet text NOT NULL,pool text NOT NULL,title text NOT NULL,idem_key text NOT NULL,request_hash text NOT NULL,
 state text NOT NULL CHECK(state IN ('REQUESTED','PROVISIONING','READY','LIVE','ENDING','ENDED','FAILED','SUSPENDED')),
 intent text NOT NULL DEFAULT 'create',provider_id text UNIQUE,revision bigint NOT NULL DEFAULT 1,
 provider_time timestamptz,created_at timestamptz NOT NULL DEFAULT clock_timestamp(),retry_at timestamptz,
 last_reason text NOT NULL DEFAULT '',UNIQUE(wallet,idem_key));
 CREATE TABLE IF NOT EXISTS livestream.stream_inbox(event_id text PRIMARY KEY,request_hash text NOT NULL,provider_id text NOT NULL,
 observed_at timestamptz NOT NULL,received_at timestamptz NOT NULL DEFAULT clock_timestamp(),outcome text NOT NULL);
 CREATE TABLE IF NOT EXISTS livestream.stream_audit(id bigserial PRIMARY KEY,operation_id text NOT NULL,actor text NOT NULL,reason text NOT NULL,
 revision bigint NOT NULL,created_at timestamptz NOT NULL DEFAULT clock_timestamp());
 CREATE TABLE IF NOT EXISTS livestream.stream_outbox(id bigserial PRIMARY KEY,operation_id text NOT NULL REFERENCES livestream.stream_operations(id),revision bigint NOT NULL,kind text NOT NULL CHECK(kind IN ('provision','start','end','suspend','provider_state')),payload jsonb NOT NULL,created_at timestamptz NOT NULL DEFAULT clock_timestamp(),UNIQUE(operation_id,revision,kind));