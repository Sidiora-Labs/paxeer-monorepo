SET LOCAL search_path = "api_stream", pg_catalog;
CREATE TABLE api_stream.stream_delivery_offsets(
 partition_id bigint PRIMARY KEY REFERENCES api_stream.stream_projection(partition_id),
 next_sequence bigint NOT NULL DEFAULT 1 CHECK(next_sequence>0),broker_stream text,broker_sequence bigint,
 updated_at timestamptz NOT NULL DEFAULT clock_timestamp());
CREATE TABLE api_stream.stream_delivery_publications(
 partition_id bigint NOT NULL,sequence bigint NOT NULL,event_id text NOT NULL,frame_sha256 text NOT NULL,
 state text NOT NULL DEFAULT 'pending' CHECK(state IN ('pending','published','quarantined')),
 owner text,epoch bigint NOT NULL DEFAULT 0,expires_at timestamptz,attempts integer NOT NULL DEFAULT 0,
 available_at timestamptz NOT NULL DEFAULT clock_timestamp(),claimed_at timestamptz,
 lease_expiry_count bigint NOT NULL DEFAULT 0,acknowledgement_latency_ms bigint,
 failure_class text,broker_stream text,broker_sequence bigint,published_at timestamptz,
 PRIMARY KEY(partition_id,sequence),FOREIGN KEY(partition_id,sequence) REFERENCES api_stream.stream_delivery_outbox(partition_id,sequence) ON DELETE CASCADE,
 CHECK((state='published')=(published_at IS NOT NULL)),CHECK(broker_sequence IS NULL OR broker_sequence>0));
CREATE INDEX stream_publication_pending ON api_stream.stream_delivery_publications(available_at,partition_id) WHERE state='pending';
CREATE FUNCTION api_stream.stream_publication_identity() RETURNS trigger LANGUAGE plpgsql SET search_path=pg_catalog AS $$ BEGIN
 IF ROW(NEW.partition_id,NEW.sequence,NEW.event_id,NEW.frame_sha256) IS DISTINCT FROM ROW(OLD.partition_id,OLD.sequence,OLD.event_id,OLD.frame_sha256) THEN RAISE EXCEPTION 'immutable stream publication identity';END IF;RETURN NEW;END $$;
CREATE TRIGGER stream_publication_identity BEFORE UPDATE ON api_stream.stream_delivery_publications FOR EACH ROW EXECUTE FUNCTION api_stream.stream_publication_identity();
REVOKE ALL ON api_stream.stream_delivery_offsets,api_stream.stream_delivery_publications FROM PUBLIC;
CREATE FUNCTION api_stream.append_candle(u jsonb,proof jsonb) RETURNS void LANGUAGE plpgsql SECURITY DEFINER SET search_path=pg_catalog AS $$
DECLARE committed record; source_fill jsonb; topic_name text; revision_value text; part bigint; last_sequence bigint; prior bytea; existing bytea; wire bytea;
BEGIN
 IF proof->>'kind' IS DISTINCT FROM 'candle_commit' OR jsonb_typeof(u) IS DISTINCT FROM 'object' OR octet_length(u::text)>262144 THEN RAISE EXCEPTION 'invalid committed candle source';END IF;
 SELECT chain_id,pool,interval,start,revision,event_id,snapshot,payload,watermark,occurred_at,projection_version,source_sequence,source_fill_id,source_fact_digest INTO STRICT committed FROM candles.chart_stream_sources_v2 WHERE chain_id='125' AND pool=proof->>'pool' AND interval=proof->>'interval' AND start::text=proof->>'bucketStart' AND event_id=u->>'eventId' AND projection_version=proof->>'projectionVersion' AND source_sequence::text=proof->>'sequence' AND source_fill_id=proof->>'sourceFillId' AND source_fact_digest=proof->>'sourceFactDigest';
 topic_name:=u->>'topic';revision_value:=u->>'revision';
 IF topic_name IS DISTINCT FROM 'market.candles.'||committed.pool||'.'||committed.interval OR revision_value !~ '^[1-9][0-9]*$' OR revision_value IS DISTINCT FROM committed.revision::text OR u->'payload' IS DISTINCT FROM committed.payload OR u->'watermark' IS DISTINCT FROM committed.watermark OR u->>'occurredAt' IS DISTINCT FROM committed.occurred_at THEN RAISE EXCEPTION 'candle source identity mismatch';END IF;
 IF NOT EXISTS(SELECT 1 FROM candles.chart_exact_v2 WHERE chain_id=committed.chain_id AND pool=committed.pool AND interval=committed.interval AND start=committed.start AND snapshot=committed.snapshot) OR NOT EXISTS(SELECT 1 FROM candles.chart_outbox_v2 WHERE chain_id=committed.chain_id AND pool=committed.pool AND interval=committed.interval AND start=committed.start AND revision=committed.revision AND payload=committed.snapshot) THEN RAISE EXCEPTION 'candle source is not committed current projection';END IF;
 SELECT payload INTO STRICT source_fill FROM indexer.indexer_accounting_fills_v1 WHERE source_fill_id=committed.source_fill_id AND source_fact_digest=committed.source_fact_digest AND pool_address=committed.pool;
 IF NOT EXISTS(SELECT 1 FROM indexer.indexer_market_partition_positions_v1 p JOIN indexer.indexer_accounting_publications_v1 a ON a.payload->'positions'->0=convert_from(p.position,'UTF8')::jsonb WHERE p.projection_version=committed.projection_version AND p.partition_kind='market' AND p.partition_key='125:'||committed.pool AND p.sequence=committed.source_sequence AND a.payload->>'facet'='market' AND a.payload->>'sourceFillId'=committed.source_fill_id AND a.payload->>'sourceFactDigest'=committed.source_fact_digest AND a.payload->'fill'=source_fill AND convert_from(p.position,'UTF8')::jsonb->>'sourceEventId'=committed.source_fill_id AND convert_from(p.position,'UTF8')::jsonb->'sourceOrder'=source_fill->'position') THEN RAISE EXCEPTION 'candle canonical partition position mismatch';END IF;
 IF NOT EXISTS(SELECT 1 FROM candles.chart_inbox_v2 WHERE chain_id='125' AND pool=committed.pool AND emitter=source_fill->>'actualEmitter' AND code_epoch=source_fill->'provenance'->>'emitterCodeEpoch' AND tx_hash=source_fill->'position'->>'transactionHash' AND log_index=(source_fill->'position'->>'logIndex')::numeric AND fill=source_fill AND encode(digest,'hex')=committed.source_fact_digest) THEN RAISE EXCEPTION 'candle canonical source binding mismatch';END IF;
 IF committed.watermark='null'::jsonb THEN
  IF committed.snapshot->>'sourceWatermarkHeight' IS DISTINCT FROM '0' OR committed.snapshot->>'completenessStatus' IS DISTINCT FROM 'backfill_incomplete' THEN RAISE EXCEPTION 'candle missing complete watermark';END IF;
 ELSE
  IF committed.watermark->>'chainId' IS DISTINCT FROM '125' OR committed.watermark->>'integrity' IS DISTINCT FROM 'healthy' OR committed.watermark->>'projectionRevision' IS DISTINCT FROM committed.snapshot->>'displayRevision' OR committed.watermark->>'height' IS DISTINCT FROM committed.snapshot->>'sourceWatermarkHeight' THEN RAISE EXCEPTION 'candle source watermark mismatch';END IF;
 END IF;
 IF committed.payload->>'poolAddress' IS DISTINCT FROM committed.pool OR committed.payload->>'interval' IS DISTINCT FROM committed.interval OR committed.payload->>'revision' IS DISTINCT FROM revision_value OR committed.snapshot->>'revision' IS DISTINCT FROM revision_value OR committed.snapshot->>'synthetic' IS DISTINCT FROM 'false' OR (committed.snapshot->>'trades')::numeric<=0 THEN RAISE EXCEPTION 'candle committed state mismatch';END IF;
 INSERT INTO api_stream.stream_projection(topic,sequence) VALUES(topic_name,0) ON CONFLICT DO NOTHING;
 SELECT partition_id,sequence,frame INTO STRICT part,last_sequence,prior FROM api_stream.stream_projection WHERE topic=topic_name FOR UPDATE;
 SELECT frame INTO existing FROM api_stream.stream_delivery_outbox WHERE event_id=u->>'eventId';
 IF FOUND THEN IF convert_from(existing,'UTF8')::jsonb-'sequence'-'coalesced'-'sourceSequenceFirst'-'sourceSequenceLast' IS DISTINCT FROM u THEN RAISE EXCEPTION 'immutable candle stream identity conflict';END IF;RETURN;END IF;
 IF last_sequence=9223372036854775807 OR (prior IS NOT NULL AND convert_from(prior,'UTF8')::jsonb->'payload'->>'start'=committed.payload->>'start' AND (convert_from(prior,'UTF8')::jsonb->>'revision')::numeric>=revision_value::numeric) THEN RAISE EXCEPTION 'candle source order regression';END IF;
 wire:=convert_to((u||jsonb_build_object('sequence',(last_sequence+1)::text,'coalesced',false,'sourceSequenceFirst',NULL,'sourceSequenceLast',NULL))::text,'UTF8');
 IF octet_length(wire)>262144 THEN RAISE EXCEPTION 'candle stream envelope too large';END IF;
 INSERT INTO api_stream.stream_delivery_outbox(partition_id,sequence,event_id,frame) VALUES(part,last_sequence+1,u->>'eventId',wire);
 UPDATE api_stream.stream_projection SET sequence=last_sequence+1,frame=wire,updated_at=clock_timestamp() WHERE partition_id=part;
END $$;
REVOKE ALL ON FUNCTION api_stream.append_candle(jsonb,jsonb) FROM PUBLIC;
