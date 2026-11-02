CREATE TABLE "recovery".recovery_broker_fences (
 proposal_id text PRIMARY KEY REFERENCES "recovery".recovery_proposals(proposal_id),
 expected_epoch bigint NOT NULL CHECK(expected_epoch>0),
 region text NOT NULL CHECK(region ~ '^[a-z][a-z0-9-]{1,62}$'),
 revision bigint NOT NULL CHECK(revision>0),
 actor text NOT NULL CHECK(length(actor) BETWEEN 1 AND 128),
 configuration_sha256 text NOT NULL CHECK(configuration_sha256 ~ '^[0-9a-f]{64}$'),
 state text NOT NULL CHECK(state IN ('applying','verified','failed')),
 proof_sha256 text CHECK(proof_sha256 ~ '^[0-9a-f]{64}$'),
 evidence jsonb,
 failure_code text,
 created_at timestamptz NOT NULL DEFAULT clock_timestamp(),
 verified_at timestamptz,
 CHECK(state<>'verified' OR (proof_sha256 IS NOT NULL AND evidence IS NOT NULL AND verified_at IS NOT NULL))
);
CREATE FUNCTION "recovery".activate_fenced_v2(proposal text, authenticated_actor text) RETURNS bigint
LANGUAGE plpgsql SECURITY DEFINER SET search_path = pg_catalog AS $$
DECLARE c "recovery".recovery_control%ROWTYPE; p "recovery".recovery_proposals%ROWTYPE;
BEGIN
 SELECT * INTO c FROM "recovery".recovery_control WHERE singleton FOR UPDATE;
 IF NOT FOUND THEN RAISE EXCEPTION 'missing restoration authority' USING ERRCODE='55000'; END IF;
 SELECT * INTO p FROM "recovery".recovery_proposals WHERE proposal_id=proposal FOR UPDATE;
 IF NOT FOUND OR NOT EXISTS(SELECT 1 FROM "recovery".recovery_broker_fences f
     WHERE f.proposal_id=p.proposal_id AND f.expected_epoch=c.epoch AND f.region=c.region
      AND f.revision=p.revision AND f.actor=authenticated_actor AND f.state='verified'
      AND f.proof_sha256=p.fence_proof_sha256
      AND f.verified_at >= clock_timestamp()-interval '5 minutes') THEN
  RAISE EXCEPTION 'actual broker epoch fence required' USING ERRCODE='55000';
 END IF;
 RETURN "recovery".activate(proposal,authenticated_actor);
END $$;
REVOKE ALL ON TABLE "recovery".recovery_broker_fences FROM PUBLIC;
REVOKE ALL ON FUNCTION "recovery".activate_fenced_v2(text,text) FROM PUBLIC;
