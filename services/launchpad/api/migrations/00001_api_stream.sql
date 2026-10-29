-- +goose Up
CREATE SCHEMA api_stream;

CREATE TABLE api_stream.stream_generation (
    singleton boolean PRIMARY KEY CHECK (singleton),
    generation uuid NOT NULL
);
INSERT INTO api_stream.stream_generation (singleton, generation) VALUES (true, gen_random_uuid());

CREATE TABLE api_stream.stream_projection (
    partition_id bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    topic text NOT NULL UNIQUE,
    sequence bigint NOT NULL DEFAULT 0 CHECK (sequence >= 0),
    frame bytea,
    updated_at timestamptz NOT NULL DEFAULT clock_timestamp(),
    CHECK (sequence = 0 OR frame IS NOT NULL)
);

CREATE TABLE api_stream.stream_delivery_outbox (
    partition_id bigint NOT NULL REFERENCES api_stream.stream_projection(partition_id),
    sequence bigint NOT NULL CHECK (sequence > 0),
    event_id text NOT NULL UNIQUE,
    frame bytea NOT NULL,
    created_at timestamptz NOT NULL DEFAULT clock_timestamp(),
    PRIMARY KEY (partition_id, sequence)
);
CREATE INDEX stream_delivery_outbox_retention ON api_stream.stream_delivery_outbox (created_at);

-- +goose StatementBegin
CREATE FUNCTION api_stream.reject_stream_delivery_update() RETURNS trigger
LANGUAGE plpgsql AS $$
BEGIN
    RAISE EXCEPTION 'stream delivery envelopes are immutable';
END;
$$;
-- +goose StatementEnd
CREATE TRIGGER stream_delivery_outbox_immutable
    BEFORE UPDATE ON api_stream.stream_delivery_outbox
    FOR EACH ROW EXECUTE FUNCTION api_stream.reject_stream_delivery_update();

CREATE TABLE api_stream.stream_snapshots (
    snapshot_id uuid PRIMARY KEY,
    account_id uuid NOT NULL,
    session_id uuid NOT NULL,
    auth_epoch bigint NOT NULL CHECK (auth_epoch >= 0),
    filter_digest text NOT NULL,
    generation uuid NOT NULL,
    scope jsonb NOT NULL,
    initial_offsets jsonb NOT NULL,
    created_at timestamptz NOT NULL,
    expires_at timestamptz NOT NULL CHECK (expires_at > created_at)
);
CREATE INDEX stream_snapshots_expiry ON api_stream.stream_snapshots (expires_at);
CREATE INDEX stream_snapshots_owner ON api_stream.stream_snapshots (account_id, session_id);

-- +goose Down
DROP TABLE api_stream.stream_snapshots;
DROP TABLE api_stream.stream_delivery_outbox;
DROP FUNCTION api_stream.reject_stream_delivery_update();
DROP TABLE api_stream.stream_projection;
DROP TABLE api_stream.stream_generation;
DROP SCHEMA api_stream;
