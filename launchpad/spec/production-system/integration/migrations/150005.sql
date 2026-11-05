-- core/stats-workers owner interface for api_stream.append_market_stats (150004): a column-scoped versioned
-- view over stats.trade_facts, the same pattern as stats.accounting_market_commits_v1 (130000) and
-- candles.chart_*_v2 read by api_stream.append_candle. api_stream_owner reads it through grants.json
-- owner_read_grants (core/api manifest); it never reads stats base tables.
SET LOCAL search_path = "stats", pg_catalog;
CREATE VIEW stats.trade_facts_v1 AS SELECT "projection_version","pool","height","transaction_index","log_index","block_timestamp","volume","side","payload" FROM stats.trade_facts;
REVOKE ALL ON stats.trade_facts_v1 FROM PUBLIC;
