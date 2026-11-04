-- recovery-reconciliation-stream-v1
BEGIN ISOLATION LEVEL REPEATABLE READ READ ONLY;
SET LOCAL search_path=pg_catalog;
SET LOCAL statement_timeout='20s';
SET LOCAL lock_timeout='2s';
DO $inventory$
BEGIN
 IF EXISTS(SELECT 1 FROM pnl.inventory_l16 WHERE
  state->>'knownQuantityRaw' IS NULL OR state->>'knownQuantityRaw' !~ '^(0|[1-9][0-9]{0,77})$' OR
  state->>'unknownQuantityRaw' IS NULL OR state->>'unknownQuantityRaw' !~ '^(0|[1-9][0-9]{0,77})$' OR
  state->>'knownRemainingCostUsdlRaw' IS NULL OR state->>'knownRemainingCostUsdlRaw' !~ '^(0|[1-9][0-9]{0,77})$' OR
  state->>'knownRealizedSubtotalUsdlRaw' IS NULL OR state->>'knownRealizedSubtotalUsdlRaw' !~ '^(0|-?[1-9][0-9]{0,77})$') THEN
  RAISE EXCEPTION 'missing or invalid authoritative raw inventory amount';
 END IF;
END $inventory$;
SET LOCAL work_mem='8MB';
SET LOCAL max_parallel_workers_per_gather=0;
DO $bounds$
BEGIN
 IF pg_size_bytes(current_setting('temp_file_limit')) NOT BETWEEN 0 AND 268435456 THEN
  RAISE EXCEPTION 'reconciliation requires an admitted temp_file_limit at most 256 MiB';
 END IF;
END $bounds$;
SELECT json_build_object(
 'kind','summary',
 'database_snapshot',CAST(pg_current_snapshot() AS text),
 'epoch',(SELECT epoch FROM recovery.current_epoch()),
 'region',(SELECT region FROM recovery.current_epoch()),
 'raw_chain_log_count',(SELECT count(*) FROM indexer.indexer_raw_logs WHERE chain_id=125),
 'inventory_count',(SELECT count(*) FROM pnl.inventory_l16),
 'known_quantity_raw',(SELECT CAST(coalesce(sum(CAST(state->>'knownQuantityRaw' AS numeric)),0) AS text) FROM pnl.inventory_l16),
 'unknown_quantity_raw',(SELECT CAST(coalesce(sum(CAST(state->>'unknownQuantityRaw' AS numeric)),0) AS text) FROM pnl.inventory_l16),
 'remaining_cost_usdl_raw',(SELECT CAST(coalesce(sum(CAST(state->>'knownRemainingCostUsdlRaw' AS numeric)),0) AS text) FROM pnl.inventory_l16),
 'realized_subtotal_usdl_raw',(SELECT CAST(coalesce(sum(CAST(state->>'knownRealizedSubtotalUsdlRaw' AS numeric)),0) AS text) FROM pnl.inventory_l16),
 'indexer_outbox_rows',(SELECT count(*) FROM indexer.delivery_outbox),
 'indexer_pending_outbox_rows',(SELECT count(*) FROM indexer.delivery_outbox WHERE state='pending'),
 'raw_chain_identities_count',(SELECT count(*) FROM indexer.indexer_raw_logs WHERE chain_id=125),
 'ingest_watermarks_count',(SELECT count(*) FROM indexer.indexer_ingest_watermarks),
 'inventory_state_count',(SELECT count(*) FROM pnl.inventory_l16),
 'indexer_outbox_identities_count',(SELECT count(*) FROM indexer.delivery_outbox),
 'indexer_inbox_identities_count',(SELECT count(*) FROM indexer.delivery_inbox),
 'candle_inbox_identities_count',(SELECT count(*) FROM candles.delivery_inbox),
 'stats_inbox_identities_count',(SELECT count(*) FROM stats.delivery_inbox),
 'pnl_inbox_identities_count',(SELECT count(*) FROM pnl.delivery_inbox)
);
SELECT json_build_object('kind','identity','stream','raw_chain_identities','value',canonical_identity)
FROM (SELECT CAST(jsonb_build_array(chain_id,block_hash,transaction_hash,log_index) AS text) AS canonical_identity
 FROM indexer.indexer_raw_logs WHERE chain_id=125) AS identities
ORDER BY canonical_identity COLLATE "C";
SELECT json_build_object('kind','identity','stream','ingest_watermarks','value',canonical_identity)
FROM (SELECT CAST(jsonb_build_array(chain_id,stream_id,raw_complete_height,typed_complete_height) AS text) AS canonical_identity
 FROM indexer.indexer_ingest_watermarks) AS identities
ORDER BY canonical_identity COLLATE "C";
SELECT json_build_object('kind','identity','stream','inventory_state','value',canonical_identity)
FROM (SELECT CAST(jsonb_build_array(partition,revision,CAST(state AS text),last_identity,height,tx_index,log_index) AS text) AS canonical_identity
 FROM pnl.inventory_l16) AS identities
ORDER BY canonical_identity COLLATE "C";
SELECT json_build_object('kind','identity','stream','indexer_outbox_identities','value',canonical_identity)
FROM (SELECT CAST(jsonb_build_array(event_id,payload_hash) AS text) AS canonical_identity
 FROM indexer.delivery_outbox) AS identities
ORDER BY canonical_identity COLLATE "C";
SELECT json_build_object('kind','identity','stream','indexer_inbox_identities','value',canonical_identity)
FROM (SELECT CAST(jsonb_build_array(consumer_id,projection_generation,event_id,envelope_hash) AS text) AS canonical_identity
 FROM indexer.delivery_inbox) AS identities
ORDER BY canonical_identity COLLATE "C";
SELECT json_build_object('kind','identity','stream','candle_inbox_identities','value',canonical_identity)
FROM (SELECT CAST(jsonb_build_array(consumer_id,projection_generation,event_id,envelope_hash) AS text) AS canonical_identity
 FROM candles.delivery_inbox) AS identities
ORDER BY canonical_identity COLLATE "C";
SELECT json_build_object('kind','identity','stream','stats_inbox_identities','value',canonical_identity)
FROM (SELECT CAST(jsonb_build_array(consumer_id,projection_generation,event_id,envelope_hash) AS text) AS canonical_identity
 FROM stats.delivery_inbox) AS identities
ORDER BY canonical_identity COLLATE "C";
SELECT json_build_object('kind','identity','stream','pnl_inbox_identities','value',canonical_identity)
FROM (SELECT CAST(jsonb_build_array(consumer_id,projection_generation,event_id,envelope_hash) AS text) AS canonical_identity
 FROM pnl.delivery_inbox) AS identities
ORDER BY canonical_identity COLLATE "C";
COMMIT;
