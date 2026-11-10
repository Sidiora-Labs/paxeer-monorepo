-- +goose Up
CREATE TABLE "candles"."development_state" (
    "key" text PRIMARY KEY,
    "payload" bytea NOT NULL
);

-- +goose Down
DROP TABLE "candles"."development_state";
