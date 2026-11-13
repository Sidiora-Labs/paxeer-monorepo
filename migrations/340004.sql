SET LOCAL search_path = "users", pg_catalog;
CREATE TABLE security_audit_delivery_receipts(audit_id bigint PRIMARY KEY REFERENCES security_audit_outbox(id),event_id text NOT NULL UNIQUE,enqueued_at timestamptz NOT NULL DEFAULT clock_timestamp());
CREATE TABLE security_audit_sink(request_id uuid NOT NULL,action text NOT NULL,payload_hash text NOT NULL,payload bytea NOT NULL,revision bigint NOT NULL CHECK(revision>0),PRIMARY KEY(request_id,action));
REVOKE ALL ON "users"."security_audit_delivery_receipts" FROM PUBLIC;
REVOKE ALL ON "users"."security_audit_sink" FROM PUBLIC;
CREATE VIEW users.auth_committed_blocks AS SELECT chain_id,height,block_hash,integrity FROM indexer.indexer_committed_heads_v2;
REVOKE ALL ON users.auth_committed_blocks FROM PUBLIC;
