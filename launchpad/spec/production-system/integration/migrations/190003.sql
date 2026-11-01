-- Proposal: core/api (api_stream) owns this function; core/ranking-algo calls it.
-- Commits one ranking category head as a public stream frame in the SAME
-- transaction that marks the ranking publication_outbox row published, so
-- every market.rankings.* NATS message has an api_stream outbox row before
-- the api_stream relay publishes it. Predecessors: 150003 (api_stream
-- delivery), 190002 (ranking heads/entries/outbox sequence), 190004 (ranking
-- owner-interface views read here instead of the ranking base tables).
SET LOCAL search_path = "api_stream", pg_catalog;
CREATE FUNCTION api_stream.append_ranking(u jsonb,proof jsonb) RETURNS void LANGUAGE plpgsql SECURITY DEFINER SET search_path=pg_catalog AS $$
DECLARE committed record; topic_name text; revision_value text; expected_items jsonb; part bigint; last_sequence bigint; prior bytea; existing bytea; wire bytea;
BEGIN
 IF proof->>'kind' IS DISTINCT FROM 'ranking_commit' OR jsonb_typeof(u) IS DISTINCT FROM 'object' OR octet_length(u::text)>262144 THEN RAISE EXCEPTION 'invalid committed ranking source';END IF;
 SELECT s.run_id,s.category,s.output_digest,s.public_header,s.completed_at,o.category_sequence,o.published_at INTO STRICT committed FROM ranking.snapshots_v1 s JOIN ranking.publication_outbox_v1 o USING(run_id,category) WHERE s.run_id=proof->>'sourceId' AND o.category_sequence::text=proof->>'sequence' AND s.output_digest=proof->>'sourceDigest';
 topic_name:=u->>'topic';revision_value:=u->>'revision';
 IF committed.published_at IS NOT NULL OR topic_name IS DISTINCT FROM 'market.rankings.'||committed.category OR revision_value IS DISTINCT FROM committed.category_sequence::text OR u->'payload'->>'snapshotId' IS DISTINCT FROM committed.run_id OR u->'payload'->>'category' IS DISTINCT FROM committed.category OR u->'payload'->>'totalEligible' IS DISTINCT FROM committed.public_header->>'total' OR u->'payload'->>'modelVersion' IS DISTINCT FROM committed.public_header->>'algorithmVersion' OR u->'payload'->'watermark' IS DISTINCT FROM committed.public_header->'provenance'->'watermark' OR u->'watermark' IS DISTINCT FROM committed.public_header->'provenance'->'watermark' OR u->'payload'->>'replace' IS DISTINCT FROM 'true' THEN RAISE EXCEPTION 'ranking source identity mismatch';END IF;
 SELECT COALESCE(jsonb_agg(jsonb_build_object('poolAddress',e.pool_address,'score',e.score,'rank',e.rank) ORDER BY e.rank),'[]'::jsonb) INTO expected_items FROM (SELECT pool_address,score,rank FROM ranking.snapshot_entries_v1 WHERE run_id=committed.run_id AND category=committed.category ORDER BY rank LIMIT 100) e;
 IF (SELECT COALESCE(jsonb_agg(jsonb_build_object('poolAddress',i->>'poolAddress','score',i->>'score','rank',(i->>'rank')::bigint) ORDER BY ord),'[]'::jsonb) FROM jsonb_array_elements(u->'payload'->'items') WITH ORDINALITY AS t(i,ord)) IS DISTINCT FROM expected_items OR EXISTS(SELECT 1 FROM jsonb_array_elements(u->'payload'->'items') i WHERE i->>'category' IS DISTINCT FROM committed.category OR i->>'modelVersion' IS DISTINCT FROM committed.public_header->>'algorithmVersion') THEN RAISE EXCEPTION 'ranking committed entries mismatch';END IF;
 INSERT INTO api_stream.stream_projection(topic,sequence) VALUES(topic_name,0) ON CONFLICT DO NOTHING;
 SELECT partition_id,sequence,frame INTO STRICT part,last_sequence,prior FROM api_stream.stream_projection WHERE topic=topic_name FOR UPDATE;
 SELECT frame INTO existing FROM api_stream.stream_delivery_outbox WHERE event_id=u->>'eventId';
 IF FOUND THEN IF convert_from(existing,'UTF8')::jsonb-'sequence'-'coalesced'-'sourceSequenceFirst'-'sourceSequenceLast' IS DISTINCT FROM u THEN RAISE EXCEPTION 'immutable ranking stream identity conflict';END IF;RETURN;END IF;
 IF last_sequence=9223372036854775807 OR (prior IS NOT NULL AND (convert_from(prior,'UTF8')::jsonb->>'revision')::numeric>=revision_value::numeric) THEN RAISE EXCEPTION 'ranking source order regression';END IF;
 wire:=convert_to((u||jsonb_build_object('sequence',(last_sequence+1)::text,'coalesced',false,'sourceSequenceFirst',NULL,'sourceSequenceLast',NULL))::text,'UTF8');
 IF octet_length(wire)>262144 THEN RAISE EXCEPTION 'ranking stream envelope too large';END IF;
 INSERT INTO api_stream.stream_delivery_outbox(partition_id,sequence,event_id,frame) VALUES(part,last_sequence+1,u->>'eventId',wire);
 UPDATE api_stream.stream_projection SET sequence=last_sequence+1,frame=wire,updated_at=clock_timestamp() WHERE partition_id=part;
END $$;
REVOKE ALL ON FUNCTION api_stream.append_ranking(jsonb,jsonb) FROM PUBLIC;
