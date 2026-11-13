SET LOCAL search_path = "stats", pg_catalog;
CREATE INDEX ON "stats".trade_facts(height,transaction_index,log_index);CREATE TABLE "stats".phase2_read_snapshots(id uuid PRIMARY KEY,pool text NOT NULL,side text NOT NULL,projection_version text NOT NULL,watermark jsonb NOT NULL,expires_at timestamptz NOT NULL);CREATE INDEX ON "stats".phase2_read_snapshots(expires_at);CREATE TABLE "stats".phase2_windows(pool text NOT NULL,seconds bigint NOT NULL,projection_version text NOT NULL,observed_at bigint NOT NULL,payload jsonb NOT NULL,PRIMARY KEY(pool,seconds,projection_version));
REVOKE ALL ON "stats"."phase2_read_snapshots" FROM PUBLIC;
REVOKE ALL ON "stats"."phase2_windows" FROM PUBLIC;
CREATE VIEW stats.accounting_pool_fences_v2 AS SELECT frozen FROM stats.accounting_pool_fences;
REVOKE ALL ON stats.accounting_pool_fences_v2 FROM PUBLIC;
CREATE VIEW stats.accounting_market_commits_v2 AS SELECT projection_version,source_fill_id,source_fact_digest,fill,pool FROM stats.accounting_market_commits;
REVOKE ALL ON stats.accounting_market_commits_v2 FROM PUBLIC;
