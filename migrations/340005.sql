SET LOCAL search_path = "chat", pg_catalog;
CREATE TABLE security_audit_delivery_receipts(audit_id bigint PRIMARY KEY REFERENCES security_audit_outbox(id),event_id text NOT NULL UNIQUE,enqueued_at timestamptz NOT NULL DEFAULT clock_timestamp());
CREATE TABLE security_audit_sink(request_id uuid NOT NULL,action text NOT NULL,payload_hash text NOT NULL,payload bytea NOT NULL,revision bigint NOT NULL CHECK(revision>0),PRIMARY KEY(request_id,action));
REVOKE ALL ON "chat"."security_audit_delivery_receipts" FROM PUBLIC;
REVOKE ALL ON "chat"."security_audit_sink" FROM PUBLIC;
