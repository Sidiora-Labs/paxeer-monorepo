SET LOCAL search_path = "indexer", pg_catalog;
CREATE TABLE indexer.indexer_quarantine (
 chain_id integer NOT NULL CHECK(chain_id=125),stream_id text NOT NULL,
 transaction_hash text NOT NULL,log_index bigint NOT NULL,block_hash text NOT NULL,height bigint NOT NULL CHECK(height>=0),
 raw_digest text NOT NULL CHECK(raw_digest~'^[0-9a-f]{64}$'),raw_payload jsonb NOT NULL,error_code text NOT NULL,abi_revision text NOT NULL,requirement text NOT NULL,required boolean NOT NULL,
 first_seen timestamptz NOT NULL DEFAULT clock_timestamp(),last_attempt_at timestamptz NOT NULL DEFAULT clock_timestamp(),attempt_count integer NOT NULL CHECK(attempt_count BETWEEN 1 AND 10),next_attempt_at timestamptz NOT NULL,operator_required boolean NOT NULL DEFAULT false,
 resolution text NOT NULL DEFAULT 'unresolved' CHECK(resolution IN('unresolved','repaired')),resolved_at timestamptz,resolved_by text,
 PRIMARY KEY(chain_id,stream_id,transaction_hash,log_index)
);
CREATE INDEX indexer_quarantine_pending ON indexer.indexer_quarantine(stream_id,height) WHERE resolution='unresolved';
CREATE TABLE indexer.indexer_capability_degradation(chain_id integer NOT NULL,stream_id text NOT NULL,transaction_hash text NOT NULL,log_index bigint NOT NULL,requirement text NOT NULL,raw_digest text NOT NULL,created_at timestamptz NOT NULL DEFAULT clock_timestamp(),PRIMARY KEY(chain_id,stream_id,transaction_hash,log_index,requirement));
CREATE TABLE indexer.indexer_replay_authorizations(
 authorization_id text PRIMARY KEY,operator_identity text NOT NULL,authorization_evidence text NOT NULL CHECK(authorization_evidence~'^[0-9a-f]{64}$'),action text NOT NULL CHECK(action IN('replay','backfill','reconstruct','waive','repair','retention')),
 target text NOT NULL,generation text NOT NULL,projection_revision text NOT NULL,from_height bigint NOT NULL CHECK(from_height>=0),to_height bigint NOT NULL CHECK(to_height>=from_height),max_concurrent integer NOT NULL CHECK(max_concurrent BETWEEN 1 AND 16),expires_at timestamptz NOT NULL,revoked_at timestamptz
);
REVOKE ALL ON indexer.indexer_replay_authorizations FROM PUBLIC;
CREATE TABLE indexer.indexer_quarantine_waivers(raw_digest text NOT NULL,requirement text NOT NULL,operator_identity text NOT NULL,authorization_id text NOT NULL REFERENCES indexer.indexer_replay_authorizations,reason text NOT NULL,from_height bigint NOT NULL,to_height bigint NOT NULL,expires_at timestamptz NOT NULL,created_at timestamptz NOT NULL DEFAULT clock_timestamp(),PRIMARY KEY(raw_digest,requirement,authorization_id));
CREATE TABLE indexer.indexer_replay_jobs(
 job_id text PRIMARY KEY,request_digest text NOT NULL UNIQUE,request jsonb NOT NULL,chain_id integer NOT NULL CHECK(chain_id=125),source_stream text NOT NULL,target text NOT NULL,generation text NOT NULL,projection_revision text NOT NULL,mode text NOT NULL CHECK(mode IN('replay','backfill','reconstruct')),authorization_id text NOT NULL REFERENCES indexer.indexer_replay_authorizations,
 from_height bigint NOT NULL CHECK(from_height>=0),to_height bigint NOT NULL CHECK(to_height>=from_height AND to_height-from_height<1000000),progress_height bigint NOT NULL,
 status text NOT NULL CHECK(status IN('queued','running','paused','cancelled','failed','completed')),owner text,fence bigint NOT NULL DEFAULT 0,expires_at timestamptz,last_error text,unavailable_from bigint,unavailable_to bigint,updated_at timestamptz NOT NULL DEFAULT clock_timestamp(),CHECK(progress_height BETWEEN from_height-1 AND to_height)
);
CREATE INDEX indexer_replay_target ON indexer.indexer_replay_jobs(target,generation,projection_revision,status);
CREATE TABLE indexer.indexer_replay_effects(target text NOT NULL,generation text NOT NULL,projection_revision text NOT NULL,effect_id text NOT NULL,raw_digest text NOT NULL,height bigint NOT NULL,event_kind text NOT NULL,payload jsonb NOT NULL,PRIMARY KEY(target,generation,projection_revision,effect_id));
CREATE TABLE indexer.indexer_replay_batches(job_id text NOT NULL REFERENCES indexer.indexer_replay_jobs,from_height bigint NOT NULL,to_height bigint NOT NULL,raw_count bigint NOT NULL,intent_count bigint NOT NULL,identity_digest text NOT NULL,checkpoint_digest text NOT NULL,committed_at timestamptz NOT NULL DEFAULT clock_timestamp(),PRIMARY KEY(job_id,from_height));
CREATE TABLE indexer.indexer_archive_segments(segment_id text PRIMARY KEY,chain_id integer NOT NULL CHECK(chain_id=125),source_stream text NOT NULL,from_height bigint NOT NULL,to_height bigint NOT NULL,manifest jsonb NOT NULL,manifest_digest text NOT NULL,object_key text NOT NULL UNIQUE,verified_at timestamptz NOT NULL DEFAULT clock_timestamp(),CHECK(from_height>=0 AND to_height>=from_height));
REVOKE ALL ON indexer.indexer_quarantine,indexer.indexer_quarantine_waivers,indexer.indexer_replay_jobs,indexer.indexer_archive_segments FROM PUBLIC;
CREATE TABLE indexer.indexer_replay_source_blocks(source_stream text NOT NULL,height bigint NOT NULL,block_hash text NOT NULL,raw_count integer NOT NULL CHECK(raw_count>=0),PRIMARY KEY(source_stream,height));
CREATE TABLE indexer.indexer_replay_checkpoints(source_stream text NOT NULL,height bigint NOT NULL,block_hash text NOT NULL,PRIMARY KEY(source_stream,height));
CREATE TABLE indexer.indexer_generation_raw(target text NOT NULL,generation text NOT NULL,projection_revision text NOT NULL,height bigint NOT NULL,transaction_index bigint NOT NULL,log_index bigint NOT NULL,transaction_hash text NOT NULL,raw_digest text NOT NULL,payload jsonb NOT NULL,PRIMARY KEY(target,generation,projection_revision,height,transaction_index,log_index,transaction_hash));
CREATE TABLE indexer.indexer_replay_checkpoint_conflicts(job_id text NOT NULL REFERENCES indexer.indexer_replay_jobs,from_height bigint NOT NULL,to_height bigint NOT NULL,expected_checkpoint text NOT NULL,observed_digest text NOT NULL,observed jsonb NOT NULL,reason text NOT NULL,created_at timestamptz NOT NULL DEFAULT clock_timestamp(),PRIMARY KEY(job_id,from_height,to_height,observed_digest));

CREATE TABLE indexer.indexer_committed_heads(
 chain_id integer NOT NULL CHECK(chain_id=125),stream_id text NOT NULL,
 height numeric(20,0) NOT NULL CHECK(height>=0),block_hash text NOT NULL CHECK(block_hash~'^0x[0-9a-f]{64}$'),
 integrity text NOT NULL CHECK(integrity IN('healthy','quarantined')),source_id text NOT NULL,
 observed_at timestamptz NOT NULL DEFAULT clock_timestamp(),PRIMARY KEY(chain_id,stream_id,height)
);
REVOKE ALL ON indexer.indexer_committed_heads FROM PUBLIC;

CREATE VIEW indexer.indexer_archive_segments_v2 AS SELECT manifest,manifest_digest,object_key,verified_at,source_stream,from_height,to_height FROM indexer.indexer_archive_segments;
REVOKE ALL ON "indexer"."indexer_quarantine" FROM PUBLIC;
REVOKE ALL ON "indexer"."indexer_capability_degradation" FROM PUBLIC;
REVOKE ALL ON "indexer"."indexer_replay_authorizations" FROM PUBLIC;
REVOKE ALL ON "indexer"."indexer_quarantine_waivers" FROM PUBLIC;
REVOKE ALL ON "indexer"."indexer_replay_jobs" FROM PUBLIC;
REVOKE ALL ON "indexer"."indexer_replay_effects" FROM PUBLIC;
REVOKE ALL ON "indexer"."indexer_replay_batches" FROM PUBLIC;
REVOKE ALL ON "indexer"."indexer_archive_segments" FROM PUBLIC;
REVOKE ALL ON "indexer"."indexer_replay_source_blocks" FROM PUBLIC;
REVOKE ALL ON "indexer"."indexer_replay_checkpoints" FROM PUBLIC;
REVOKE ALL ON "indexer"."indexer_generation_raw" FROM PUBLIC;
REVOKE ALL ON "indexer"."indexer_replay_checkpoint_conflicts" FROM PUBLIC;
REVOKE ALL ON "indexer"."indexer_committed_heads" FROM PUBLIC;
CREATE VIEW indexer.indexer_committed_heads_v2 AS
SELECT h.chain_id,h.height,max(h.block_hash) AS block_hash,
CASE WHEN count(DISTINCT h.block_hash)>1 OR bool_or(h.integrity='quarantined' OR EXISTS(
 SELECT 1 FROM indexer.indexer_quarantine q WHERE q.chain_id=h.chain_id AND q.stream_id=h.stream_id AND q.height<=h.height AND q.required AND q.resolution='unresolved'
 AND NOT EXISTS(SELECT 1 FROM indexer.indexer_quarantine_waivers w JOIN indexer.indexer_replay_authorizations a ON a.authorization_id=w.authorization_id WHERE w.raw_digest=q.raw_digest AND w.requirement=q.requirement AND q.height BETWEEN w.from_height AND w.to_height AND w.expires_at>clock_timestamp() AND a.expires_at>clock_timestamp() AND a.revoked_at IS NULL))) THEN 'quarantined' ELSE 'healthy' END AS integrity
FROM indexer.indexer_committed_heads h GROUP BY h.chain_id,h.height;
REVOKE ALL ON indexer.indexer_committed_heads_v2 FROM PUBLIC;
CREATE VIEW indexer.indexer_committed_heads_v1 AS SELECT h.chain_id,h.stream_id,h.height,h.block_hash,CASE WHEN h.integrity='healthy' THEN v.integrity ELSE 'quarantined' END AS integrity,h.source_id,h.observed_at FROM indexer.indexer_committed_heads h JOIN indexer.indexer_committed_heads_v2 v ON v.chain_id=h.chain_id AND v.height=h.height;
REVOKE ALL ON indexer.indexer_committed_heads_v1 FROM PUBLIC;
