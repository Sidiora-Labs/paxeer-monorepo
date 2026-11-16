SET LOCAL search_path = stats, pg_catalog;
CREATE VIEW stats.pool_market_context_v1 WITH (security_barrier=true) AS
 SELECT pool_address,price,market_cap FROM stats.pool_stats;
REVOKE ALL ON stats.pool_market_context_v1 FROM PUBLIC;
