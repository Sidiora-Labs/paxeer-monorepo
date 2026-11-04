-- An absent generation means no fold has been published yet. Do not infer
-- metadata from existing ranks or publish an empty generation during migration.
CREATE TABLE perps.spot_board_omissions (
    account bytea PRIMARY KEY CHECK (octet_length(account) = 20),
    updated_block bigint NOT NULL
);

CREATE TABLE perps.spot_board_generation (
    singleton boolean PRIMARY KEY CHECK (singleton),
    generated_at timestamptz NOT NULL,
    omitted_accounts bigint NOT NULL CHECK (omitted_accounts >= 0)
);
