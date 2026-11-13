-- +goose Up
CREATE TABLE metadata.legacy_publications (
    operation uuid PRIMARY KEY,
    token text NOT NULL,
    state text NOT NULL CHECK (state IN ('staging', 'published')),
    staged jsonb NOT NULL,
    retired jsonb NOT NULL DEFAULT '[]',
    created_at timestamptz NOT NULL DEFAULT clock_timestamp()
);

-- +goose Down
DROP TABLE metadata.legacy_publications;
