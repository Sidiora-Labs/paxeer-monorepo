CREATE TABLE perps.spot_leaderboard (
    kind text NOT NULL CHECK (kind IN ('pnl', 'volume', 'win-rate')),
    rank integer NOT NULL CHECK (rank >= 1),
    account bytea NOT NULL CHECK (octet_length(account) = 20),
    value numeric NOT NULL,
    updated_block bigint NOT NULL,
    PRIMARY KEY (kind, rank),
    UNIQUE (kind, account)
);

CREATE TABLE perps.spot_account_legs (
    account bytea PRIMARY KEY,
    realized_usd numeric NOT NULL,
    volume_usd numeric NOT NULL,
    trades bigint NOT NULL,
    updated_block bigint NOT NULL
);
