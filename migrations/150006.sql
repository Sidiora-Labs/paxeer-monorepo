-- core/stats-workers interface for the media/livestream directory market-cap sort (streams.MarketCapReader):
-- a column-scoped versioned view over the stats.pool_stats projection, the same pattern as stats.trade_facts_v1
-- (150005). livestream_runtime reads it through its grants.json runtime grant; it never reads stats base tables.
SET LOCAL search_path = "stats", pg_catalog;
CREATE VIEW stats.pool_market_caps_v1 AS SELECT "pool_address","market_cap" FROM stats.pool_stats;
REVOKE ALL ON stats.pool_market_caps_v1 FROM PUBLIC;
