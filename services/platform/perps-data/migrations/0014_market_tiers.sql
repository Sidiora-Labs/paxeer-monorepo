CREATE TABLE perps.market_tiers (
    market_id perps.uint256 NOT NULL REFERENCES perps.markets (market_id),
    tier smallint NOT NULL CHECK (tier >= 0),
    lower_bound_usd perps.uint256 NOT NULL,
    max_leverage perps.uint256 NOT NULL,
    maintenance_bps perps.uint256 NOT NULL,
    deduction_usd perps.uint256 NOT NULL,
    refreshed_block bigint NOT NULL CHECK (refreshed_block >= 0),
    PRIMARY KEY (market_id, tier)
);
