-- +goose Up
CREATE TABLE IF NOT EXISTS "candles"."development_state" (
    "key" text PRIMARY KEY,
    "payload" bytea NOT NULL
);

-- +goose Down
DROP TABLE "candles"."development_state";
