-- Coordinator renders "recovery" only after validating [a-z][a-z0-9_]{1,62}.
-- Rendered into the recovery-owned migration by the serial coordinator.
CREATE TABLE "recovery".recovery_control (
 singleton boolean PRIMARY KEY DEFAULT true CHECK(singleton),
 epoch bigint NOT NULL CHECK(epoch > 0), region text NOT NULL CHECK(region ~ '^[a-z][a-z0-9-]{1,62}$'),
 revision bigint NOT NULL CHECK(revision > 0),
 checkpoint_sha256 text NOT NULL CHECK(checkpoint_sha256 ~ '^[0-9a-f]{64}$')
);
CREATE TABLE "recovery".recovery_proposals (
 proposal_id text PRIMARY KEY CHECK(proposal_id ~ '^[a-z][a-z0-9-]{1,62}$'), expected_epoch bigint NOT NULL CHECK(expected_epoch > 0),
 next_region text NOT NULL CHECK(next_region ~ '^[a-z][a-z0-9-]{1,62}$'), revision bigint NOT NULL CHECK(revision > 0),
 checkpoint_sha256 text NOT NULL CHECK(checkpoint_sha256 ~ '^[0-9a-f]{64}$'),
 fence_proof_sha256 text CHECK(fence_proof_sha256 ~ '^[0-9a-f]{64}$'),
 state text NOT NULL CHECK(state IN ('proposed','validated','activated','rejected')),
 actor text NOT NULL CHECK(length(actor) BETWEEN 1 AND 128), created_at timestamptz NOT NULL DEFAULT clock_timestamp()
);
CREATE TABLE "recovery".recovery_audit (
 id bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
 proposal_id text NOT NULL, state text NOT NULL, actor text NOT NULL,
 revision bigint NOT NULL, epoch bigint NOT NULL, region text NOT NULL,
 checkpoint_sha256 text NOT NULL, fence_proof_sha256 text,
 at timestamptz NOT NULL DEFAULT clock_timestamp()
);
-- Called in SAME transaction as every authoritative write or durable inbox effect.
-- FOR SHARE blocks cutover until the guarded transaction commits/rolls back.
CREATE FUNCTION "recovery".assert_epoch(wanted bigint, wanted_region text) RETURNS void
LANGUAGE plpgsql SECURITY DEFINER SET search_path = pg_catalog AS $$
DECLARE current_epoch bigint; current_region text;
BEGIN
 SELECT epoch,region INTO current_epoch,current_region FROM "recovery".recovery_control WHERE singleton FOR SHARE;
 IF NOT FOUND OR wanted IS DISTINCT FROM current_epoch OR wanted_region IS DISTINCT FROM current_region THEN
  RAISE EXCEPTION 'stale restoration epoch or region' USING ERRCODE='55000';
 END IF;
END $$;
CREATE FUNCTION "recovery".current_epoch() RETURNS TABLE(epoch bigint,region text)
LANGUAGE sql SECURITY DEFINER SET search_path = pg_catalog AS $$
 SELECT epoch,region FROM "recovery".recovery_control WHERE singleton
$$;
-- Coordinator-only. Digest proves a pinned external fence record exists, NOT its validity.
-- The orchestrator must independently verify provider/network/credential revocations.
CREATE FUNCTION "recovery".activate(proposal text, authenticated_actor text) RETURNS bigint
LANGUAGE plpgsql SET search_path = pg_catalog AS $$
DECLARE c "recovery".recovery_control%ROWTYPE; p "recovery".recovery_proposals%ROWTYPE;
BEGIN
 SELECT * INTO c FROM "recovery".recovery_control WHERE singleton FOR UPDATE;
 IF NOT FOUND THEN RAISE EXCEPTION 'missing restoration authority'; END IF;
 SELECT * INTO p FROM "recovery".recovery_proposals WHERE proposal_id=proposal FOR UPDATE;
 IF NOT FOUND OR p.state <> 'validated' OR p.fence_proof_sha256 IS NULL OR
    p.actor IS DISTINCT FROM authenticated_actor OR length(authenticated_actor)=0 OR
    p.expected_epoch <> c.epoch OR p.revision <= c.revision OR p.next_region = c.region THEN
  RAISE EXCEPTION 'invalid recovery activation' USING ERRCODE='55000';
 END IF;
 UPDATE "recovery".recovery_control SET epoch=c.epoch+1,region=p.next_region,revision=p.revision,
   checkpoint_sha256=p.checkpoint_sha256 WHERE singleton;
 UPDATE "recovery".recovery_proposals SET state='activated' WHERE proposal_id=proposal;
 INSERT INTO "recovery".recovery_audit(proposal_id,state,actor,revision,epoch,region,checkpoint_sha256,fence_proof_sha256)
 VALUES(proposal,'activated',authenticated_actor,p.revision,c.epoch+1,p.next_region,p.checkpoint_sha256,p.fence_proof_sha256);
 RETURN c.epoch+1;
END $$;
REVOKE ALL ON ALL TABLES IN SCHEMA "recovery" FROM PUBLIC;
REVOKE ALL ON FUNCTION "recovery".activate(text,text) FROM PUBLIC;
REVOKE ALL ON FUNCTION "recovery".assert_epoch(bigint,text) FROM PUBLIC;

REVOKE ALL ON FUNCTION "recovery".current_epoch() FROM PUBLIC;
