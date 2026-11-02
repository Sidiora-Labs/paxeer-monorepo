CREATE TABLE indexer.indexer_canonical_creations (
 token text PRIMARY KEY, pool text NOT NULL UNIQUE, creator text NOT NULL,
 factory text NOT NULL, optical text NOT NULL, nft_id numeric(78,0) NOT NULL,
 height bigint NOT NULL CHECK(height>=0), block_hash text NOT NULL,
 transaction_hash text NOT NULL, transaction_index bigint NOT NULL CHECK(transaction_index BETWEEN 0 AND 4294967295),
 log_index bigint NOT NULL CHECK(log_index BETWEEN 0 AND 4294967295),
 block_timestamp bigint NOT NULL CHECK(block_timestamp>0), provenance jsonb NOT NULL, payload jsonb NOT NULL,
 UNIQUE(transaction_hash,log_index)
);
CREATE VIEW indexer.indexer_canonical_creations_v1 AS SELECT token,pool,creator,factory,optical,nft_id,height,block_hash,transaction_hash,transaction_index,log_index,block_timestamp,provenance,payload FROM indexer.indexer_canonical_creations;
CREATE TABLE indexer.indexer_token_transfers (
 chain_id integer NOT NULL CHECK(chain_id=125), transaction_hash text NOT NULL,
 log_index bigint NOT NULL CHECK(log_index BETWEEN 0 AND 4294967295), token text NOT NULL,
 from_address text NOT NULL, to_address text NOT NULL, amount_raw numeric(78,0) NOT NULL CHECK(amount_raw>=0),
 height bigint NOT NULL CHECK(height>=0), transaction_index bigint NOT NULL CHECK(transaction_index BETWEEN 0 AND 4294967295),
 block_hash text NOT NULL, provenance jsonb NOT NULL, payload jsonb NOT NULL,
 PRIMARY KEY(chain_id,transaction_hash,log_index), FOREIGN KEY(token) REFERENCES indexer.indexer_canonical_creations(token),
 FOREIGN KEY(chain_id,transaction_hash,log_index) REFERENCES indexer.indexer_raw_logs(chain_id,transaction_hash,log_index)
);
CREATE INDEX indexer_token_movements_order ON indexer.indexer_token_transfers(token,height,transaction_index,log_index);
CREATE VIEW indexer.indexer_token_transfers_v1 AS SELECT chain_id,transaction_hash,log_index,token,from_address,to_address,amount_raw,height,transaction_index,block_hash,provenance,payload FROM indexer.indexer_token_transfers;
CREATE VIEW indexer.indexer_raw_logs_v1 AS SELECT chain_id,transaction_hash,log_index,height,transaction_index,block_hash,emitter,topics,data FROM indexer.indexer_raw_logs;
CREATE VIEW indexer.indexer_block_completeness_v1 AS SELECT chain_id,stream_id,height,block_hash,raw_count,fetch_complete,required_decode_complete,completed_at FROM indexer.indexer_block_completeness;
CREATE VIEW indexer.indexer_ingest_leases_v1 AS SELECT chain_id,stream_id,owner_instance,fence_token,expires_at FROM indexer.indexer_ingest_leases;
CREATE VIEW indexer.indexer_ingest_watermarks_v1 AS SELECT chain_id,stream_id,raw_complete_height,typed_complete_height FROM indexer.indexer_ingest_watermarks;
CREATE VIEW indexer.indexer_ingest_commits_v1 AS SELECT chain_id,stream_id,batch_id,previous_height,last_complete_height,block_hash,record_count FROM indexer.indexer_ingest_commits;
CREATE VIEW indexer.indexer_market_partition_positions_v1 AS SELECT projection_version,partition_kind,partition_key,sequence,position FROM indexer.delivery_partition_positions WHERE partition_kind='market';
