SET LOCAL search_path = indexer, pg_catalog;
CREATE VIEW indexer.indexer_swap_outbox_v1 AS
 SELECT chain_id,transaction_hash,log_index,event_kind,payload
 FROM indexer.indexer_ingest_outbox WHERE event_kind IN ('Buy','Sell','MultihopSwap');
REVOKE ALL ON indexer.indexer_swap_outbox_v1 FROM PUBLIC;
