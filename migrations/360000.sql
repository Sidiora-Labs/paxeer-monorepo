SET LOCAL search_path = "chat", pg_catalog;
-- Proposed coordinator migration; not applied by service startup.
CREATE TABLE chat_conversations (id text PRIMARY KEY, kind text NOT NULL CHECK(kind IN ('dm','pool')), next_sequence bigint NOT NULL DEFAULT 0 CHECK(next_sequence>=0), history_version bigint NOT NULL DEFAULT 0, retained_from bigint NOT NULL DEFAULT 1);
CREATE TABLE chat_members (conversation_id text NOT NULL REFERENCES chat_conversations(id), wallet text NOT NULL, read_sequence bigint NOT NULL DEFAULT 0, PRIMARY KEY(conversation_id,wallet));
CREATE INDEX chat_members_wallet ON chat_members(wallet,conversation_id);
CREATE TABLE chat_messages (id text PRIMARY KEY, conversation_id text NOT NULL REFERENCES chat_conversations(id), sender text NOT NULL, client_id text NOT NULL, content text NOT NULL, reply_to text NOT NULL DEFAULT '', sequence bigint NOT NULL CHECK(sequence>0), revision bigint NOT NULL DEFAULT 1 CHECK(revision>0), created_at timestamptz NOT NULL, edited_at timestamptz, deleted_at timestamptz, UNIQUE(sender,client_id), UNIQUE(conversation_id,sequence));
CREATE TABLE chat_outbox (event_id text PRIMARY KEY, message_id text NOT NULL REFERENCES chat_messages(id), revision bigint NOT NULL, payload jsonb NOT NULL, channels jsonb NOT NULL, created_at timestamptz NOT NULL DEFAULT now(), delivered_at timestamptz, attempts integer NOT NULL DEFAULT 0, UNIQUE(message_id,revision));
CREATE INDEX chat_outbox_pending ON chat_outbox(created_at,event_id) WHERE delivered_at IS NULL;
