-- core/stats-workers is the committed producer of the market.stats.<pool> and
-- market.price.<pool> topics of the core-api stream projection. Both append
-- functions bind every frame to committed canonical stats/indexer facts before
-- advancing the topic sequence, exactly like append_market_trade/append_candle.
-- Every foreign read goes through a column-scoped owner-interface view granted to
-- api_stream_owner in grants.json owner_read_grants: stats.trade_facts_v1
-- (150005), stats.accounting_market_commits_v1 (130000) and
-- indexer.indexer_canonical_creations_v1 / indexer.indexer_raw_logs_v1 (190001).
CREATE FUNCTION api_stream.market_projection_head(topic_name text) RETURNS bytea LANGUAGE plpgsql STABLE SECURITY DEFINER SET search_path=pg_catalog AS $$
DECLARE result bytea;
BEGIN
 IF topic_name !~ '^market\.(stats|price)\.0x[0-9a-f]{40}$' THEN RAISE EXCEPTION 'market projection head topic not admitted';END IF;
 SELECT frame INTO result FROM api_stream.stream_projection WHERE topic=topic_name;
 RETURN result;
END $$;
REVOKE ALL ON FUNCTION api_stream.market_projection_head(text) FROM PUBLIC;

CREATE FUNCTION api_stream.append_market_stats(u jsonb,proof jsonb) RETURNS void LANGUAGE plpgsql SECURITY DEFINER SET search_path=pg_catalog AS $$
DECLARE pool_value text; topic_name text; upper jsonb; wm jsonb; anchor_ts bigint; window_end bigint; volume numeric; buys bigint; sells bigint; booked numeric; part bigint; last_sequence bigint; prior bytea; prior_frame jsonb; existing bytea; wire bytea;
BEGIN
 IF proof->>'kind' IS DISTINCT FROM 'stats_commit' OR jsonb_typeof(u) IS DISTINCT FROM 'object' OR octet_length(u::text)>262144 THEN RAISE EXCEPTION 'invalid committed stats source';END IF;
 pool_value:=u->'payload'->>'poolAddress';topic_name:=u->>'topic';upper:=u->'payload'->'sourceUpperPosition';wm:=u->'watermark';
 IF pool_value !~ '^0x[0-9a-f]{40}$' OR topic_name IS DISTINCT FROM 'market.stats.'||pool_value OR proof->>'pool' IS DISTINCT FROM pool_value OR coalesce(proof->>'projectionVersion','') !~ '^[A-Za-z0-9_.:-]{1,64}$' OR u->>'eventId' IS DISTINCT FROM 'stats:'||pool_value||':'||(u->>'revision') THEN RAISE EXCEPTION 'stats source identity mismatch';END IF;
 IF u->'payload'->'watermark' IS DISTINCT FROM wm OR wm->>'integrity' IS DISTINCT FROM 'healthy' OR wm->>'chainId' IS DISTINCT FROM '125' OR u->>'revision' !~ '^[1-9][0-9]{0,77}$' OR u->>'revision' IS DISTINCT FROM wm->>'projectionRevision' OR upper->>'chainId' IS DISTINCT FROM '125' OR upper->>'height' IS DISTINCT FROM wm->>'height' OR upper->>'blockHash' IS DISTINCT FROM wm->>'blockHash' THEN RAISE EXCEPTION 'stats watermark binding mismatch';END IF;
 SELECT ts INTO anchor_ts FROM (
  SELECT (c.fill->>'blockTimestampSeconds')::bigint AS ts FROM stats.accounting_market_commits_v1 c WHERE c.projection_version=proof->>'projectionVersion' AND c.fill->'position'=upper
  UNION ALL
  SELECT k.block_timestamp FROM indexer.indexer_canonical_creations_v1 k WHERE k.transaction_hash=upper->>'transactionHash' AND k.log_index::text=upper->>'logIndex' AND k.height::text=upper->>'height' AND k.transaction_index::text=upper->>'transactionIndex' AND k.block_hash=upper->>'blockHash') a LIMIT 1;
 IF anchor_ts IS NULL THEN RAISE EXCEPTION 'stats anchor is not a committed canonical event';END IF;
 IF NOT EXISTS(SELECT 1 FROM indexer.indexer_canonical_creations_v1 k WHERE k.pool=pool_value AND k.height<=(upper->>'height')::bigint) THEN RAISE EXCEPTION 'stats pool is not canonical at anchor';END IF;
 window_end:=anchor_ts+1;
 IF u->'payload'->>'windowEnd' IS DISTINCT FROM to_char(to_timestamp(window_end) AT TIME ZONE 'UTC','YYYY-MM-DD"T"HH24:MI:SS.MS"Z"') OR u->'payload'->>'windowStart' IS DISTINCT FROM to_char(to_timestamp(window_end-86400) AT TIME ZONE 'UTC','YYYY-MM-DD"T"HH24:MI:SS.MS"Z"') OR u->>'occurredAt' IS DISTINCT FROM to_char(to_timestamp(anchor_ts) AT TIME ZONE 'UTC','YYYY-MM-DD"T"HH24:MI:SS.MS"Z"') THEN RAISE EXCEPTION 'stats window binding mismatch';END IF;
 SELECT coalesce(sum(t.volume),0),count(*) FILTER(WHERE t.side='buy'),count(*) FILTER(WHERE t.side='sell') INTO volume,buys,sells FROM stats.trade_facts_v1 t JOIN stats.accounting_market_commits_v1 c ON c.projection_version=t.projection_version AND c.pool=t.pool AND c.fill->'position'->>'transactionHash'=t.payload->'position'->>'transactionHash' AND (c.fill->'position'->>'logIndex')::numeric=t.log_index
  WHERE t.projection_version=proof->>'projectionVersion' AND t.pool=pool_value AND (t.height,t.transaction_index,t.log_index)<=((upper->>'height')::numeric,(upper->>'transactionIndex')::numeric,(upper->>'logIndex')::numeric) AND t.block_timestamp>=window_end-86400 AND t.block_timestamp<window_end;
 IF u->'payload'->>'volume24hRaw' IS DISTINCT FROM volume::text OR u->'payload'->>'buys24h' IS DISTINCT FROM buys::text OR u->'payload'->>'sells24h' IS DISTINCT FROM sells::text THEN RAISE EXCEPTION 'stats window aggregate differs from committed fills';END IF;
 -- Without a syncReserves (PoolStateUpdated) reset the booked real USDL is the
 -- exact fold of committed fills: buys add paid input minus fee, sells remove output.
 IF NOT EXISTS(SELECT 1 FROM indexer.indexer_raw_logs_v1 r WHERE r.chain_id=125 AND lower(r.topics->>0)='0x8155a4b38a877d212b271811786986f8f7d78074c82fce4526c58defcc6d7ebd' AND lower(r.topics->>1)='0x000000000000000000000000'||substr(pool_value,3) AND (r.height,r.transaction_index,r.log_index)<=((upper->>'height')::bigint,(upper->>'transactionIndex')::bigint,(upper->>'logIndex')::bigint)) THEN
  SELECT coalesce(sum(CASE WHEN c.fill->>'side'='buy' THEN (c.fill->>'paidInputRaw')::numeric-(c.fill->>'feeRaw')::numeric ELSE -(c.fill->>'usdlAmountRaw')::numeric END),0) INTO booked FROM stats.accounting_market_commits_v1 c WHERE c.projection_version=proof->>'projectionVersion' AND c.pool=pool_value AND ((c.fill->'position'->>'height')::numeric,(c.fill->'position'->>'transactionIndex')::numeric,(c.fill->'position'->>'logIndex')::numeric)<=((upper->>'height')::numeric,(upper->>'transactionIndex')::numeric,(upper->>'logIndex')::numeric);
  IF u->'payload'->>'realUsdlBalanceRaw' IS DISTINCT FROM booked::text THEN RAISE EXCEPTION 'stats booked real USDL differs from committed fills';END IF;
 END IF;
 INSERT INTO api_stream.stream_projection(topic,sequence) VALUES(topic_name,0) ON CONFLICT DO NOTHING;
 SELECT partition_id,sequence,frame INTO STRICT part,last_sequence,prior FROM api_stream.stream_projection WHERE topic=topic_name FOR UPDATE;
 SELECT frame INTO existing FROM api_stream.stream_delivery_outbox WHERE event_id=u->>'eventId';
 IF FOUND THEN IF convert_from(existing,'UTF8')::jsonb-'sequence'-'coalesced'-'sourceSequenceFirst'-'sourceSequenceLast' IS DISTINCT FROM u THEN RAISE EXCEPTION 'immutable stats stream identity conflict';END IF;RETURN;END IF;
 IF prior IS NOT NULL THEN
  prior_frame:=convert_from(prior,'UTF8')::jsonb;
  IF (prior_frame->>'revision')::numeric>=(u->>'revision')::numeric OR ((prior_frame->'payload'->'sourceUpperPosition'->>'height')::numeric,(prior_frame->'payload'->'sourceUpperPosition'->>'transactionIndex')::numeric,(prior_frame->'payload'->'sourceUpperPosition'->>'logIndex')::numeric)>((upper->>'height')::numeric,(upper->>'transactionIndex')::numeric,(upper->>'logIndex')::numeric) THEN RAISE EXCEPTION 'stats source order regression';END IF;
 END IF;
 IF last_sequence=9223372036854775807 THEN RAISE EXCEPTION 'stats stream sequence exhausted';END IF;
 wire:=convert_to((u||jsonb_build_object('sequence',(last_sequence+1)::text,'coalesced',false,'sourceSequenceFirst',NULL,'sourceSequenceLast',NULL))::text,'UTF8');
 IF octet_length(wire)>262144 THEN RAISE EXCEPTION 'stats stream envelope too large';END IF;
 INSERT INTO api_stream.stream_delivery_outbox(partition_id,sequence,event_id,frame) VALUES(part,last_sequence+1,u->>'eventId',wire);
 UPDATE api_stream.stream_projection SET sequence=last_sequence+1,frame=wire,updated_at=clock_timestamp() WHERE partition_id=part;
END $$;
REVOKE ALL ON FUNCTION api_stream.append_market_stats(jsonb,jsonb) FROM PUBLIC;

CREATE FUNCTION api_stream.append_market_price(u jsonb,proof jsonb) RETURNS void LANGUAGE plpgsql SECURITY DEFINER SET search_path=pg_catalog AS $$
DECLARE committed record; pool_value text; topic_name text; src jsonb; token_amount numeric; usdl_amount numeric; part bigint; last_sequence bigint; prior bytea; existing bytea; wire bytea;
BEGIN
 IF proof->>'kind' IS DISTINCT FROM 'price_commit' OR jsonb_typeof(u) IS DISTINCT FROM 'object' OR octet_length(u::text)>262144 THEN RAISE EXCEPTION 'invalid committed price source';END IF;
 pool_value:=proof->>'pool';topic_name:=u->>'topic';src:=u->'payload'->'source';
 IF pool_value !~ '^0x[0-9a-f]{40}$' OR topic_name IS DISTINCT FROM 'market.price.'||pool_value OR coalesce(proof->>'sequence','') !~ '^[1-9][0-9]{0,18}$' THEN RAISE EXCEPTION 'price source identity mismatch';END IF;
 SELECT c.fill,c.sequence INTO STRICT committed FROM stats.accounting_market_commits_v1 c WHERE c.projection_version=proof->>'projectionVersion' AND c.pool=pool_value AND c.source_fill_id=proof->>'sourceFillId' AND c.source_fact_digest=proof->>'sourceFactDigest' AND c.sequence::text=proof->>'sequence';
 IF u->>'revision' IS DISTINCT FROM committed.sequence::text OR u->>'eventId' IS DISTINCT FROM 'price:125:'||(committed.fill->'position'->>'transactionHash')||':'||(committed.fill->'position'->>'logIndex') OR u->>'occurredAt' IS DISTINCT FROM to_char(to_timestamp((committed.fill->>'blockTimestampSeconds')::bigint) AT TIME ZONE 'UTC','YYYY-MM-DD"T"HH24:MI:SS.MS"Z"') THEN RAISE EXCEPTION 'price source binding mismatch';END IF;
 IF u->'watermark'->>'chainId' IS DISTINCT FROM '125' OR u->'watermark'->>'integrity' IS DISTINCT FROM 'healthy' OR u->'watermark'->>'height' IS DISTINCT FROM committed.fill->'position'->>'height' OR u->'watermark'->>'blockHash' IS DISTINCT FROM committed.fill->'position'->>'blockHash' OR u->'watermark'->>'projectionRevision' IS DISTINCT FROM u->>'revision' THEN RAISE EXCEPTION 'price watermark binding mismatch';END IF;
 IF src->>'kind' IS DISTINCT FROM 'gross_execution' OR src->>'amountBasis' IS DISTINCT FROM 'paid_input' OR src->'position' IS DISTINCT FROM committed.fill->'position' OR src->>'sourceEventId' IS DISTINCT FROM '125:'||(committed.fill->'position'->>'transactionHash')||':'||(committed.fill->'position'->>'logIndex') OR src->>'observedAt' IS DISTINCT FROM u->>'occurredAt' OR src->'providerId' IS DISTINCT FROM 'null'::jsonb OR src->'sourceReference' IS DISTINCT FROM 'null'::jsonb THEN RAISE EXCEPTION 'price source evidence mismatch';END IF;
 IF u->'payload'->'baseAsset' IS DISTINCT FROM jsonb_build_object('kind','erc20','chainId','125','address',committed.fill->'token'->>'address','currencyCode',NULL,'decimals',(committed.fill->'token'->>'decimals')::integer) OR u->'payload'->'quoteAsset' IS DISTINCT FROM jsonb_build_object('kind','erc20','chainId','125','address',committed.fill->'settlement'->>'address','currencyCode',NULL,'decimals',(committed.fill->'settlement'->>'decimals')::integer) THEN RAISE EXCEPTION 'price asset binding mismatch';END IF;
 token_amount:=(committed.fill->>'tokenAmountRaw')::numeric;usdl_amount:=(committed.fill->>'usdlAmountRaw')::numeric;
 IF u->'payload'->>'denominator' !~ '^[1-9][0-9]*$' OR u->'payload'->>'numerator' !~ '^(0|[1-9][0-9]*)$' OR length(u->'payload'->>'denominator')>512 OR length(u->'payload'->>'numerator')>512 OR token_amount<=0 OR (u->'payload'->>'numerator')::numeric*token_amount*power(10::numeric,(committed.fill->'settlement'->>'decimals')::integer)<>(u->'payload'->>'denominator')::numeric*usdl_amount*power(10::numeric,(committed.fill->'token'->>'decimals')::integer) THEN RAISE EXCEPTION 'price differs from committed gross execution';END IF;
 INSERT INTO api_stream.stream_projection(topic,sequence) VALUES(topic_name,0) ON CONFLICT DO NOTHING;
 SELECT partition_id,sequence,frame INTO STRICT part,last_sequence,prior FROM api_stream.stream_projection WHERE topic=topic_name FOR UPDATE;
 SELECT frame INTO existing FROM api_stream.stream_delivery_outbox WHERE event_id=u->>'eventId';
 IF FOUND THEN IF convert_from(existing,'UTF8')::jsonb-'sequence'-'coalesced'-'sourceSequenceFirst'-'sourceSequenceLast' IS DISTINCT FROM u THEN RAISE EXCEPTION 'immutable price stream identity conflict';END IF;RETURN;END IF;
 IF last_sequence=9223372036854775807 OR (prior IS NOT NULL AND (convert_from(prior,'UTF8')::jsonb->>'revision')::numeric>=committed.sequence) THEN RAISE EXCEPTION 'price source order regression';END IF;
 wire:=convert_to((u||jsonb_build_object('sequence',(last_sequence+1)::text,'coalesced',false,'sourceSequenceFirst',NULL,'sourceSequenceLast',NULL))::text,'UTF8');
 IF octet_length(wire)>262144 THEN RAISE EXCEPTION 'price stream envelope too large';END IF;
 INSERT INTO api_stream.stream_delivery_outbox(partition_id,sequence,event_id,frame) VALUES(part,last_sequence+1,u->>'eventId',wire);
 UPDATE api_stream.stream_projection SET sequence=last_sequence+1,frame=wire,updated_at=clock_timestamp() WHERE partition_id=part;
END $$;
REVOKE ALL ON FUNCTION api_stream.append_market_price(jsonb,jsonb) FROM PUBLIC;
