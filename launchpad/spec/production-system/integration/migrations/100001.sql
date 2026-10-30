SET LOCAL search_path = "candles", pg_catalog;

CREATE TABLE "candles".chart_series_clock(chain_id text NOT NULL,pool text NOT NULL,revision bigint NOT NULL CHECK(revision>0),watermark_height bigint NOT NULL CHECK(watermark_height>=0),watermark_timestamp bigint NOT NULL CHECK(watermark_timestamp>=0),has_gaps boolean NOT NULL,PRIMARY KEY(chain_id,pool));
CREATE TABLE "candles".chart_rollup_pending(chain_id text NOT NULL,pool text NOT NULL,interval text NOT NULL,start bigint NOT NULL,child_revision bigint NOT NULL,PRIMARY KEY(chain_id,pool,interval,start));
CREATE TABLE "candles".chart_rollup_lineage(chain_id text NOT NULL,pool text NOT NULL,interval text NOT NULL,start bigint NOT NULL,child_digest text NOT NULL,source_watermark bigint NOT NULL,algorithm text NOT NULL,PRIMARY KEY(chain_id,pool,interval,start));
CREATE TABLE "candles".chart_delivery_attempts(consumer_id text NOT NULL,generation text NOT NULL,publication_id text NOT NULL,wire_digest text NOT NULL,source_sequence bigint NOT NULL,attempt_count bigint NOT NULL DEFAULT 1,first_seen timestamptz NOT NULL DEFAULT clock_timestamp(),last_attempt timestamptz NOT NULL DEFAULT clock_timestamp(),last_category text NOT NULL,resolved boolean NOT NULL DEFAULT false,PRIMARY KEY(consumer_id,generation,publication_id));
CREATE TABLE "candles".chart_reprocess_audit(id bigserial PRIMARY KEY,consumer_id text NOT NULL,generation text NOT NULL,publication_id text NOT NULL,previous_digest text NOT NULL,corrected_digest text NOT NULL,decoder_version text NOT NULL,operator_id text NOT NULL,created_at timestamptz NOT NULL DEFAULT clock_timestamp());
CREATE TABLE "candles".chart_stream_sources(chain_id text NOT NULL,pool text NOT NULL,interval text NOT NULL,start bigint NOT NULL,revision numeric(20,0) NOT NULL CHECK(revision>0),event_id text NOT NULL UNIQUE,snapshot jsonb NOT NULL,payload jsonb NOT NULL,watermark jsonb NOT NULL,occurred_at text NOT NULL,projection_version text NOT NULL,source_sequence bigint NOT NULL CHECK(source_sequence>0),source_fill_id text NOT NULL,source_fact_digest text NOT NULL CHECK(source_fact_digest ~ '^[0-9a-f]{64}$'),PRIMARY KEY(chain_id,pool,interval,start,revision));
CREATE FUNCTION "candles".chart_stream_source_immutable() RETURNS trigger LANGUAGE plpgsql SET search_path=pg_catalog AS $$ BEGIN RAISE EXCEPTION 'immutable canonical candle source'; END $$;
CREATE TRIGGER chart_stream_source_immutable BEFORE UPDATE OR DELETE ON "candles".chart_stream_sources FOR EACH ROW EXECUTE FUNCTION "candles".chart_stream_source_immutable();
CREATE TABLE "candles".chart_recoverability_incidents(id bigserial PRIMARY KEY,pool text NOT NULL,reason text NOT NULL,evidence_digest text NOT NULL,created_at timestamptz NOT NULL DEFAULT clock_timestamp());


CREATE TABLE "candles".chart_rebuild_runs(run_id text PRIMARY KEY,pool text NOT NULL,from_time bigint NOT NULL,to_time bigint NOT NULL,manifest_digest text NOT NULL,expected_digest text NOT NULL,output_digest text,status text NOT NULL CHECK(status IN('staging','validated','promoted')),base_revision bigint NOT NULL,promotion_epoch bigint,created_at timestamptz NOT NULL DEFAULT clock_timestamp(),promoted_at timestamptz,operator_name text NOT NULL DEFAULT current_user);
CREATE TABLE "candles".chart_rebuild_candles(run_id text NOT NULL REFERENCES "candles".chart_rebuild_runs,interval text NOT NULL,start bigint NOT NULL,snapshot jsonb NOT NULL,PRIMARY KEY(run_id,interval,start));
CREATE TABLE "candles".chart_rebuild_traders(run_id text NOT NULL REFERENCES "candles".chart_rebuild_runs,interval text NOT NULL,start bigint NOT NULL,actor text NOT NULL,PRIMARY KEY(run_id,interval,start,actor));
CREATE TABLE "candles".chart_rebuild_fills(run_id text NOT NULL REFERENCES "candles".chart_rebuild_runs,source_id text NOT NULL,digest text NOT NULL,payload jsonb NOT NULL,PRIMARY KEY(run_id,source_id));
CREATE TABLE "candles".chart_archive_checkpoints(pool text NOT NULL,from_time bigint NOT NULL,to_time bigint NOT NULL,manifest_digest text NOT NULL,verified_at timestamptz NOT NULL DEFAULT clock_timestamp(),PRIMARY KEY(pool,from_time,to_time));
CREATE TABLE "candles".chart_replay_references(reference_id text PRIMARY KEY,pool text NOT NULL,from_time bigint NOT NULL,to_time bigint NOT NULL,expires_at timestamptz NOT NULL);

REVOKE ALL ON candles."chart_series_clock" FROM PUBLIC;
REVOKE ALL ON candles."chart_rollup_pending" FROM PUBLIC;
REVOKE ALL ON candles."chart_rollup_lineage" FROM PUBLIC;
REVOKE ALL ON candles."chart_delivery_attempts" FROM PUBLIC;
REVOKE ALL ON candles."chart_reprocess_audit" FROM PUBLIC;
REVOKE ALL ON candles."chart_stream_sources" FROM PUBLIC;
REVOKE ALL ON candles."chart_recoverability_incidents" FROM PUBLIC;
REVOKE ALL ON candles."chart_rebuild_runs" FROM PUBLIC;
REVOKE ALL ON candles."chart_rebuild_candles" FROM PUBLIC;
REVOKE ALL ON candles."chart_rebuild_traders" FROM PUBLIC;
REVOKE ALL ON candles."chart_rebuild_fills" FROM PUBLIC;
REVOKE ALL ON candles."chart_archive_checkpoints" FROM PUBLIC;
REVOKE ALL ON candles."chart_replay_references" FROM PUBLIC;
CREATE VIEW candles.chart_stream_sources_v2 AS SELECT "chain_id","pool","interval","start","revision","event_id","snapshot","payload","watermark","occurred_at","projection_version","source_sequence","source_fill_id","source_fact_digest" FROM candles.chart_stream_sources;
REVOKE ALL ON candles.chart_stream_sources_v2 FROM PUBLIC;
CREATE VIEW candles.chart_exact_v2 AS SELECT "chain_id","pool","interval","start","snapshot" FROM candles.chart_exact;
REVOKE ALL ON candles.chart_exact_v2 FROM PUBLIC;
CREATE VIEW candles.chart_outbox_v2 AS SELECT "chain_id","pool","interval","start","revision","payload" FROM candles.chart_outbox;
REVOKE ALL ON candles.chart_outbox_v2 FROM PUBLIC;
CREATE VIEW candles.chart_inbox_v2 AS SELECT "chain_id","pool","emitter","code_epoch","tx_hash","log_index","fill","digest" FROM candles.chart_inbox;
REVOKE ALL ON candles.chart_inbox_v2 FROM PUBLIC;
