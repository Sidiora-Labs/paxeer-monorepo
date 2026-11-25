SET LOCAL search_path = "api_stream", pg_catalog;

CREATE INDEX stream_publication_quarantined ON api_stream.stream_delivery_publications(partition_id,sequence) WHERE state='quarantined';
CREATE INDEX stream_publication_expiries ON api_stream.stream_delivery_publications(lease_expiry_count) WHERE lease_expiry_count>0;
CREATE INDEX stream_publication_latency ON api_stream.stream_delivery_publications(acknowledgement_latency_ms DESC) WHERE acknowledgement_latency_ms IS NOT NULL;

ALTER TABLE api_stream.stream_delivery_offsets
 ADD COLUMN retention_owner text,
 ADD COLUMN retention_epoch bigint NOT NULL DEFAULT 0 CHECK(retention_epoch>=0),
 ADD COLUMN retention_expires_at timestamptz,
 ADD COLUMN retention_next_at timestamptz,
 ADD COLUMN retention_cutoff timestamptz,
 ADD COLUMN retention_removed bigint NOT NULL DEFAULT 0 CHECK(retention_removed BETWEEN 0 AND 5000);

CREATE FUNCTION api_stream.prune_published_frames() RETURNS trigger
LANGUAGE plpgsql SECURITY DEFINER SET search_path=pg_catalog AS $$
BEGIN
 IF NEW.retention_owner IS NULL OR NEW.retention_epoch<1 OR NEW.retention_expires_at IS NULL OR NEW.retention_expires_at<=clock_timestamp()
    OR NEW.retention_cutoff IS NULL OR NEW.retention_cutoff>clock_timestamp()-interval '72 hours' THEN
  RAISE EXCEPTION 'active retention lease and reader replay window required';
 END IF;
 WITH picked AS MATERIALIZED (
  SELECT o.partition_id,o.sequence
  FROM api_stream.stream_delivery_outbox o
  JOIN api_stream.stream_delivery_publications p USING(partition_id,sequence)
  JOIN api_stream.stream_delivery_offsets c USING(partition_id)
  WHERE o.created_at<NEW.retention_cutoff AND p.state='published' AND o.sequence<c.next_sequence-1
  ORDER BY o.created_at,o.partition_id,o.sequence LIMIT 5000
  FOR UPDATE OF o,p SKIP LOCKED
 )
 DELETE FROM api_stream.stream_delivery_outbox o USING picked q
 WHERE o.partition_id=q.partition_id AND o.sequence=q.sequence;
 GET DIAGNOSTICS NEW.retention_removed = ROW_COUNT;
 RETURN NEW;
END $$;
REVOKE ALL ON FUNCTION api_stream.prune_published_frames() FROM PUBLIC;
CREATE TRIGGER stream_delivery_prune BEFORE UPDATE OF retention_cutoff ON api_stream.stream_delivery_offsets
 FOR EACH ROW EXECUTE FUNCTION api_stream.prune_published_frames();
