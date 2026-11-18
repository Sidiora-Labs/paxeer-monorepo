SET LOCAL search_path = "indexer", pg_catalog;
CREATE OR REPLACE VIEW indexer.indexer_committed_heads_v2 AS
SELECT DISTINCT ON (h.chain_id,h.height) h.chain_id,h.height,h.block_hash,
CASE WHEN h.block_hash<>(
 SELECT conflicting.block_hash FROM indexer.indexer_committed_heads conflicting
 WHERE conflicting.chain_id=h.chain_id AND conflicting.height=h.height
 ORDER BY conflicting.block_hash LIMIT 1
) OR EXISTS (
 SELECT 1 FROM indexer.indexer_committed_heads quarantined
 WHERE quarantined.chain_id=h.chain_id AND quarantined.height=h.height AND quarantined.integrity='quarantined'
 OFFSET 0
) OR EXISTS (
 SELECT 1 FROM indexer.indexer_quarantine q
 WHERE q.chain_id=h.chain_id AND q.height<=h.height AND q.required AND q.resolution='unresolved'
 AND h.height=(
  SELECT source.height FROM indexer.indexer_committed_heads source
  WHERE source.chain_id=h.chain_id AND source.stream_id=q.stream_id AND source.height>=h.height
  ORDER BY source.height LIMIT 1
 )
 AND NOT EXISTS (
  SELECT 1 FROM indexer.indexer_quarantine_waivers w
  JOIN indexer.indexer_replay_authorizations a ON a.authorization_id=w.authorization_id
  WHERE w.raw_digest=q.raw_digest AND w.requirement=q.requirement
  AND q.height BETWEEN w.from_height AND w.to_height
  AND w.expires_at>clock_timestamp() AND a.expires_at>clock_timestamp() AND a.revoked_at IS NULL
 )
) THEN 'quarantined' ELSE 'healthy' END AS integrity
FROM indexer.indexer_committed_heads h
ORDER BY h.chain_id,h.height DESC,h.block_hash DESC;
