-- Wiring request: domain 3, services/platform/perps-data/migrations:
-- install this additive migration before deploying the board writer/readers.
-- An absent generation means no fold has been published yet; do not backfill
-- a timestamp or an omission count from ranks that lack that information.
CREATE TABLE perps.spot_board_omissions (
    account bytea PRIMARY KEY CHECK (octet_length(account) = 20),
    updated_block bigint NOT NULL
);
CREATE TABLE perps.spot_board_generation (
    singleton boolean PRIMARY KEY CHECK (singleton),
    generated_at timestamptz NOT NULL,
    omitted_accounts bigint NOT NULL CHECK (omitted_accounts >= 0)
);
