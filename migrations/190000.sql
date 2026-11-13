-- Proposal only: serial coordinator registers migration 190000 for ranking.
-- Replace ranking with the approved service-owned schema at integration.

CREATE TABLE ranking.snapshots (
 run_id text NOT NULL,
 category text NOT NULL CHECK(category IN ('trending','breakout','volume','movers','unusual','new')),
 input_digest char(64) NOT NULL,
 output_digest char(64) NOT NULL,
 artifact jsonb NOT NULL,
 completed_at timestamptz NOT NULL DEFAULT clock_timestamp(),
 PRIMARY KEY(run_id,category)
);
CREATE TABLE ranking.publication_outbox (
 sequence bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
 run_id text NOT NULL,
 category text NOT NULL,
 artifact jsonb NOT NULL,
 created_at timestamptz NOT NULL DEFAULT clock_timestamp(),
 published_at timestamptz,
 UNIQUE(run_id,category),
 FOREIGN KEY(run_id,category) REFERENCES ranking.snapshots(run_id,category)
);
CREATE INDEX ranking_outbox_pending ON ranking.publication_outbox(sequence) WHERE published_at IS NULL;
-- Runtime grant: INSERT,SELECT on snapshots; SELECT,INSERT,UPDATE(published_at)
-- on publication_outbox, USAGE on its sequence. No snapshot UPDATE/DELETE.
