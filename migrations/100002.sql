SET LOCAL search_path = "candles", pg_catalog;

CREATE TABLE candles.chart_rebuild_stream_sources(
    event_id text PRIMARY KEY REFERENCES candles.chart_stream_sources(event_id),
    run_id text NOT NULL REFERENCES candles.chart_rebuild_runs,
    manifest_digest text NOT NULL CHECK(manifest_digest ~ '^[0-9a-f]{64}$'),
    output_digest text NOT NULL CHECK(output_digest ~ '^[0-9a-f]{64}$'),
    owner_epoch bigint NOT NULL CHECK(owner_epoch>0)
);
CREATE TRIGGER chart_rebuild_stream_source_immutable
    BEFORE UPDATE OR DELETE ON candles.chart_rebuild_stream_sources
    FOR EACH ROW EXECUTE FUNCTION candles.chart_stream_source_immutable();
REVOKE ALL ON candles.chart_rebuild_stream_sources FROM PUBLIC;
