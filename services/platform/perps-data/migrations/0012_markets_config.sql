ALTER TABLE perps.markets ADD COLUMN tick_size perps.uint256;
ALTER TABLE perps.markets ADD COLUMN lot_size perps.uint256;
ALTER TABLE perps.markets ADD COLUMN tier_class smallint CHECK (tier_class >= 0);
ALTER TABLE perps.markets ADD COLUMN impact_notional_usd perps.uint256;
ALTER TABLE perps.markets ADD COLUMN oi_cap_usd perps.uint256;
ALTER TABLE perps.markets ADD COLUMN funding_cap_ppm_per_hour perps.uint256;
ALTER TABLE perps.markets ADD COLUMN listed_at timestamptz;
