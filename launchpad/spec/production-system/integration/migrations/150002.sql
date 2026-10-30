CREATE SCHEMA IF NOT EXISTS api_stream;
CREATE TABLE api_stream.stream_generation(singleton boolean PRIMARY KEY DEFAULT true CHECK(singleton),generation uuid NOT NULL DEFAULT gen_random_uuid());
INSERT INTO api_stream.stream_generation(singleton) VALUES(true);
CREATE TABLE api_stream.stream_projection(topic text PRIMARY KEY CHECK(length(topic) BETWEEN 1 AND 200),partition_id bigint GENERATED ALWAYS AS IDENTITY UNIQUE,sequence bigint NOT NULL CHECK(sequence>=0),frame bytea,updated_at timestamptz NOT NULL DEFAULT clock_timestamp(),CHECK((sequence=0)=(frame IS NULL)),CHECK(frame IS NULL OR octet_length(frame)<=262144));
CREATE TABLE api_stream.stream_delivery_outbox(partition_id bigint NOT NULL REFERENCES api_stream.stream_projection(partition_id),sequence bigint NOT NULL CHECK(sequence>0),event_id text NOT NULL UNIQUE,frame bytea NOT NULL CHECK(octet_length(frame)<=262144),created_at timestamptz NOT NULL DEFAULT clock_timestamp(),PRIMARY KEY(partition_id,sequence));
CREATE INDEX stream_delivery_retention ON api_stream.stream_delivery_outbox(created_at);
CREATE FUNCTION api_stream.stream_immutable() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN RAISE EXCEPTION 'immutable stream envelope'; END $$;
CREATE TRIGGER stream_delivery_immutable BEFORE UPDATE ON api_stream.stream_delivery_outbox FOR EACH ROW EXECUTE FUNCTION api_stream.stream_immutable();
CREATE TABLE api_stream.stream_snapshots(snapshot_id uuid PRIMARY KEY,account_id uuid NOT NULL,session_id uuid NOT NULL,auth_epoch bigint NOT NULL CHECK(auth_epoch>=0),filter_digest text NOT NULL CHECK(filter_digest ~ '^[0-9a-f]{64}$'),generation uuid NOT NULL,scope jsonb NOT NULL,initial_offsets jsonb NOT NULL,created_at timestamptz NOT NULL DEFAULT clock_timestamp(),expires_at timestamptz NOT NULL,CHECK(expires_at>created_at AND expires_at<=created_at+interval '72 hours'));
CREATE INDEX stream_snapshot_expiry ON api_stream.stream_snapshots(expires_at);
REVOKE ALL ON ALL TABLES IN SCHEMA api_stream FROM PUBLIC;
REVOKE ALL ON ALL FUNCTIONS IN SCHEMA api_stream FROM PUBLIC;

CREATE FUNCTION api_stream.append_market_trade(u jsonb,proof jsonb) RETURNS void LANGUAGE plpgsql SECURITY DEFINER SET search_path=pg_catalog AS $$
DECLARE source_fill jsonb; expected_trade jsonb; refs jsonb; fee_asset jsonb; part bigint; last_sequence bigint; prior bytea; wire bytea; existing bytea; event_identity text; topic_name text; revision_value text;
BEGIN
 IF proof->>'kind' IS DISTINCT FROM 'market_commit' OR jsonb_typeof(u) IS DISTINCT FROM 'object' OR octet_length(u::text)>262144 THEN RAISE EXCEPTION 'invalid public stream source'; END IF;
 SELECT fill INTO STRICT source_fill FROM stats.accounting_market_commits_v1 WHERE pool=proof->>'pool' AND projection_version=proof->>'projectionVersion' AND sequence::text=proof->>'sequence' AND source_fill_id=proof->>'sourceFillId' AND source_fact_digest=proof->>'sourceFactDigest';
 event_identity:=u->>'eventId'; topic_name:=u->>'topic'; revision_value:=u->>'revision';
 IF event_identity IS DISTINCT FROM source_fill->>'sourceFillId' OR topic_name IS DISTINCT FROM 'market.trades.'||(source_fill->>'poolAddress') OR revision_value IS DISTINCT FROM proof->>'sequence' OR revision_value !~ '^[1-9][0-9]*$' THEN RAISE EXCEPTION 'stream source identity mismatch'; END IF;
 IF (u->>'occurredAt')::timestamptz IS DISTINCT FROM to_timestamp((source_fill->>'blockTimestampSeconds')::double precision) OR u->'watermark'->>'chainId' IS DISTINCT FROM '125' OR u->'watermark'->>'height' IS DISTINCT FROM source_fill->'position'->>'height' OR u->'watermark'->>'blockHash' IS DISTINCT FROM source_fill->'position'->>'blockHash' OR u->'watermark'->>'projectionRevision' IS DISTINCT FROM revision_value OR u->'watermark'->>'integrity' IS DISTINCT FROM 'healthy' THEN RAISE EXCEPTION 'stream watermark mismatch'; END IF;
 fee_asset:=source_fill->'token';IF source_fill->>'side'='buy' THEN fee_asset:=source_fill->'settlement';END IF;
 refs:='[]'::jsonb;
 IF source_fill->'attribution'->'recipient' <> 'null'::jsonb THEN SELECT COALESCE(jsonb_agg(e->'artifactSHA256'),'[]'::jsonb) INTO refs FROM jsonb_array_elements(source_fill->'attribution'->'evidence') e;END IF;
 expected_trade:=jsonb_build_object('eventId',source_fill->'sourceFillId','poolAddress',source_fill->'poolAddress','sender',source_fill->'attribution'->'transactionSender','recipient',source_fill->'attribution'->'recipient','side',source_fill->'side','tokenAmountRaw',source_fill->'tokenAmountRaw','usdlAmountRaw',source_fill->'usdlAmountRaw','position',source_fill->'position','feeRaw',source_fill->'feeRaw','feeAssetAddress',fee_asset->'address','feeAssetDecimals',fee_asset->'decimals','feeDisposition',source_fill->'feeDisposition','paidInputRaw',source_fill->'paidInputRaw','pricedInputRaw',source_fill->'pricedInputRaw','ammInputAfterFeeRaw',source_fill->'ammInputAfterFeeRaw','inputDerivationQuality',source_fill->'inputDerivationQuality','recipientAttribution',CASE WHEN source_fill->'attribution'->'recipient'='null'::jsonb THEN 'unavailable' ELSE source_fill->'attribution'->>'quality' END,'recipientEvidenceRefs',refs,'emittingContract',source_fill->'actualEmitter','codeEpoch',source_fill->'provenance'->'emitterCodeEpoch');
 IF u->'payload' IS DISTINCT FROM expected_trade THEN RAISE EXCEPTION 'public trade differs from committed source';END IF;
 INSERT INTO api_stream.stream_projection(topic,sequence) VALUES(topic_name,0) ON CONFLICT DO NOTHING;
 SELECT partition_id,sequence,frame INTO STRICT part,last_sequence,prior FROM api_stream.stream_projection WHERE topic=topic_name FOR UPDATE;
 SELECT frame INTO existing FROM api_stream.stream_delivery_outbox WHERE event_id=event_identity;
 IF FOUND THEN
  IF convert_from(existing,'UTF8')::jsonb-'sequence'-'coalesced'-'sourceSequenceFirst'-'sourceSequenceLast' IS DISTINCT FROM u THEN RAISE EXCEPTION 'immutable public stream identity conflict';END IF;
  RETURN;
 END IF;
 IF last_sequence=9223372036854775807 OR (prior IS NOT NULL AND (convert_from(prior,'UTF8')::jsonb->>'revision')::numeric>=revision_value::numeric) THEN RAISE EXCEPTION 'stream source order regression';END IF;
 wire:=convert_to((u||jsonb_build_object('sequence',(last_sequence+1)::text,'coalesced',false,'sourceSequenceFirst',NULL,'sourceSequenceLast',NULL))::text,'UTF8');
 IF octet_length(wire)>262144 THEN RAISE EXCEPTION 'public stream envelope too large';END IF;
 INSERT INTO api_stream.stream_delivery_outbox(partition_id,sequence,event_id,frame) VALUES(part,last_sequence+1,event_identity,wire);
 UPDATE api_stream.stream_projection SET sequence=last_sequence+1,frame=wire,updated_at=clock_timestamp() WHERE partition_id=part;
END $$;
REVOKE ALL ON FUNCTION api_stream.append_market_trade(jsonb,jsonb) FROM PUBLIC;

CREATE FUNCTION api_stream.append_portfolio(u jsonb,proof jsonb) RETURNS void LANGUAGE plpgsql SECURITY DEFINER SET search_path=pg_catalog AS $$
DECLARE source_proof record; row_state record; row_proof record; change jsonb; position_value jsonb; expected_position jsonb; summary_value jsonb; logical text; prefix text; wallet_value text; known numeric; unknown numeric; balance numeric; known_total numeric:=0; unknown_total numeric:=0; realized_total numeric:=0; revision_total numeric:=0; incomplete boolean:=false; rows_count integer:=0; unpriced_count integer:=0; basis text; realized_quality text; latest_height numeric:=-1; latest_tx bigint:=-1; latest_log bigint:=-1; latest_hash text; row_watermark jsonb; part bigint; last_sequence bigint; prior bytea; existing bytea; wire bytea;
BEGIN
 IF proof->>'kind' IS DISTINCT FROM 'portfolio_commit' OR jsonb_typeof(u) IS DISTINCT FROM 'object' OR octet_length(u::text)>262144 THEN RAISE EXCEPTION 'invalid committed portfolio source';END IF;
 wallet_value:=proof->>'wallet';logical:='125:'||wallet_value||':'||(proof->>'token');prefix:='proportional_floor_v1:'||(proof->>'projectionVersion')||':125:'||wallet_value||':';
 IF wallet_value !~ '^0x[0-9a-f]{40}$' OR proof->>'projectionVersion' !~ '^[a-zA-Z0-9_-]{1,128}$' OR u->>'topic' IS DISTINCT FROM 'private.portfolio.'||wallet_value OR u->>'eventId' IS DISTINCT FROM (proof->>'sourceId')||'_'||wallet_value THEN RAISE EXCEPTION 'portfolio source identity mismatch';END IF;
 SELECT * INTO STRICT source_proof FROM pnl.inventory_l16_stream_proof_v1 WHERE projection_generation=proof->>'projectionVersion' AND partition=logical AND effect_identity=proof->>'sourceId' AND leg_identity=proof->>'sourceFillId' AND mapping_digest=proof->>'sourceFactDigest' AND owner_epoch::text=proof->>'ownerEpoch' AND attestation_digest=proof->>'sourceDigest';
 IF NOT EXISTS(SELECT 1 FROM pnl.inventory_l16_stream_state_v1 WHERE partition=prefix||(proof->>'token') AND last_identity=source_proof.leg_identity AND state=source_proof.payload) THEN RAISE EXCEPTION 'portfolio source is not committed current state';END IF;
 IF u->'payload'->'completeBatch' IS DISTINCT FROM 'true'::jsonb OR u->'payload'->>'snapshotId' IS DISTINCT FROM u->'payload'->'summary'->>'snapshotId' THEN RAISE EXCEPTION 'incomplete portfolio checkpoint';END IF;
 FOR row_state IN SELECT * FROM pnl.inventory_l16_stream_state_v1 WHERE starts_with(partition,prefix) ORDER BY partition LOOP
  rows_count:=rows_count+1;IF rows_count>100 THEN RAISE EXCEPTION 'portfolio resnapshot capacity exceeded';END IF;
  logical:=substring(row_state.partition FROM length('proportional_floor_v1:'||(proof->>'projectionVersion')||':')+1);
  SELECT * INTO STRICT row_proof FROM pnl.inventory_l16_stream_proof_v1 WHERE projection_generation=proof->>'projectionVersion' AND partition=logical AND leg_identity=row_state.last_identity AND payload=row_state.state ORDER BY owner_epoch DESC LIMIT 1;
  SELECT e INTO STRICT change FROM jsonb_array_elements(u->'payload'->'changes') e WHERE e->>'positionId'=logical;
  position_value:=change->'position';row_watermark:=position_value->'sourceWatermark';
  IF change->>'operation' IS DISTINCT FROM 'upsert' OR change->>'revision' IS DISTINCT FROM row_state.revision::text OR row_watermark->>'chainId' IS DISTINCT FROM '125' OR row_watermark->>'height' IS DISTINCT FROM row_state.height::text OR row_watermark->>'blockHash' IS DISTINCT FROM convert_from(row_proof.position,'UTF8')::jsonb->'sourceOrder'->>'blockHash' OR row_watermark->>'projectionRevision' IS DISTINCT FROM row_state.revision::text OR row_watermark->>'integrity' IS DISTINCT FROM 'healthy' THEN RAISE EXCEPTION 'portfolio source position mismatch';END IF;
  known:=(row_state.state->>'knownQuantityRaw')::numeric;unknown:=(row_state.state->>'unknownQuantityRaw')::numeric;balance:=known+unknown;
  basis:=CASE WHEN unknown=0 THEN 'complete' WHEN known>0 THEN 'partial' ELSE 'unknown' END;
  realized_quality:=CASE WHEN (row_state.state->>'incompleteRealized')::boolean THEN 'partial' ELSE 'complete' END;
  expected_position:=jsonb_build_object('positionId',logical,'chainId','125','wallet',wallet_value,'tokenAddress',split_part(logical,':',3),'balanceRaw',balance::text,'knownQuantityRaw',known::text,'unknownQuantityRaw',unknown::text,'knownRemainingCostUsdlRaw',row_state.state->'knownRemainingCostUsdlRaw','basisCoverage',jsonb_build_object('numerator',CASE WHEN balance=0 THEN '1' ELSE known::text END,'denominator',CASE WHEN balance=0 THEN '1' ELSE balance::text END),'remainingCostUsdlRaw',CASE WHEN unknown=0 THEN row_state.state->'knownRemainingCostUsdlRaw' ELSE 'null'::jsonb END,'realizedPnlRaw',CASE WHEN (row_state.state->>'incompleteRealized')::boolean THEN 'null'::jsonb ELSE row_state.state->'knownRealizedSubtotalUsdlRaw' END,'knownRealizedSubtotalUsdlRaw',row_state.state->'knownRealizedSubtotalUsdlRaw','unrealizedPnlRaw',NULL,'basisQuality',basis,'realizedBasisQuality',realized_quality,'pricingQuality','unpriced','markPrice',NULL,'sourceWatermark',row_watermark);
  IF position_value IS DISTINCT FROM expected_position THEN RAISE EXCEPTION 'public portfolio differs from computed inventory';END IF;
  known_total:=known_total+known;unknown_total:=unknown_total+unknown;realized_total:=realized_total+(row_state.state->>'knownRealizedSubtotalUsdlRaw')::numeric;revision_total:=revision_total+row_state.revision;incomplete:=incomplete OR (row_state.state->>'incompleteRealized')::boolean;
  IF balance>0 THEN unpriced_count:=unpriced_count+1;END IF;
  IF (row_state.height,row_state.tx_index,row_state.log_index)>(latest_height,latest_tx,latest_log) THEN latest_height:=row_state.height;latest_tx:=row_state.tx_index;latest_log:=row_state.log_index;latest_hash:=row_watermark->>'blockHash';END IF;
 END LOOP;
 IF rows_count<1 OR jsonb_array_length(u->'payload'->'changes')<>rows_count OR u->>'revision' IS DISTINCT FROM revision_total::text OR proof->>'sequence' IS DISTINCT FROM revision_total::text OR u->'payload'->>'summaryRevision' IS DISTINCT FROM revision_total::text THEN RAISE EXCEPTION 'portfolio checkpoint revision mismatch';END IF;
 IF u->'watermark'->>'chainId' IS DISTINCT FROM '125' OR u->'watermark'->>'height' IS DISTINCT FROM latest_height::text OR u->'watermark'->>'blockHash' IS DISTINCT FROM latest_hash OR u->'watermark'->>'projectionRevision' IS DISTINCT FROM revision_total::text OR u->'watermark'->>'integrity' IS DISTINCT FROM 'healthy' THEN RAISE EXCEPTION 'portfolio summary watermark mismatch';END IF;
 summary_value:=jsonb_build_object('wallet',wallet_value,'pricedSubtotalUsdlRaw','0','completeValueUsdlRaw',CASE WHEN unpriced_count=0 THEN to_jsonb('0'::text) ELSE 'null'::jsonb END,'realizedPnlRaw',CASE WHEN incomplete THEN 'null'::jsonb ELSE to_jsonb(realized_total::text) END,'knownRealizedSubtotalUsdlRaw',realized_total::text,'basisQuality',CASE WHEN unknown_total=0 THEN 'complete' WHEN known_total>0 THEN 'partial' ELSE 'unknown' END,'pricingQuality',CASE WHEN unpriced_count=0 THEN 'complete' ELSE 'unpriced' END,'unpricedAssetCount',unpriced_count,'positionCount',rows_count,'snapshotId',u->'payload'->>'snapshotId','watermark',u->'watermark');
 IF u->'payload'->'summary' IS DISTINCT FROM summary_value THEN RAISE EXCEPTION 'portfolio summary differs from computed inventory';END IF;
 INSERT INTO api_stream.stream_projection(topic,sequence) VALUES(u->>'topic',0) ON CONFLICT DO NOTHING;
 SELECT partition_id,sequence,frame INTO STRICT part,last_sequence,prior FROM api_stream.stream_projection WHERE topic=u->>'topic' FOR UPDATE;
 SELECT frame INTO existing FROM api_stream.stream_delivery_outbox WHERE event_id=u->>'eventId';
 IF FOUND THEN IF convert_from(existing,'UTF8')::jsonb-'sequence'-'coalesced'-'sourceSequenceFirst'-'sourceSequenceLast' IS DISTINCT FROM u THEN RAISE EXCEPTION 'immutable portfolio identity conflict';END IF;RETURN;END IF;
 IF last_sequence=9223372036854775807 OR (prior IS NOT NULL AND (convert_from(prior,'UTF8')::jsonb->>'revision')::numeric>=revision_total) THEN RAISE EXCEPTION 'portfolio source order regression';END IF;
 wire:=convert_to((u||jsonb_build_object('sequence',(last_sequence+1)::text,'coalesced',false,'sourceSequenceFirst',NULL,'sourceSequenceLast',NULL))::text,'UTF8');IF octet_length(wire)>262144 THEN RAISE EXCEPTION 'portfolio envelope capacity exceeded';END IF;
 INSERT INTO api_stream.stream_delivery_outbox(partition_id,sequence,event_id,frame) VALUES(part,last_sequence+1,u->>'eventId',wire);
 UPDATE api_stream.stream_projection SET sequence=last_sequence+1,frame=wire,updated_at=clock_timestamp() WHERE partition_id=part;
END $$;
REVOKE ALL ON FUNCTION api_stream.append_portfolio(jsonb,jsonb) FROM PUBLIC;
