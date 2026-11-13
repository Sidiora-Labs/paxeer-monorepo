-- core/ranking-algo owner interface for api_stream.append_ranking (190003): column-scoped versioned views,
-- the same pattern as candles.chart_*_v2 read by api_stream.append_candle. api_stream_owner reads these
-- through grants.json owner_read_grants (core/api manifest); it never reads ranking base tables.
SET LOCAL search_path = "ranking", pg_catalog;
CREATE VIEW ranking.snapshots_v1 AS SELECT "run_id","category","output_digest","public_header","completed_at" FROM ranking.snapshots;
CREATE VIEW ranking.publication_outbox_v1 AS SELECT "run_id","category","category_sequence","published_at" FROM ranking.publication_outbox;
CREATE VIEW ranking.snapshot_entries_v1 AS SELECT "run_id","category","pool_address","score","rank" FROM ranking.snapshot_entries;
REVOKE ALL ON ranking.snapshots_v1, ranking.publication_outbox_v1, ranking.snapshot_entries_v1 FROM PUBLIC;
