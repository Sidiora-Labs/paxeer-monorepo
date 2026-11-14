import assert from "node:assert/strict";
import test from "node:test";

import {
  decodeJourney,
  encodeJourney,
  journeyStateVariants,
  evidenceClassVariants,
  type EvidenceRef,
  type Journey,
  type JourneyState,
  type JsonValue,
  type VerificationLevel,
} from "../src/api/generated/index.ts";
import { recoveryPresentation } from "../src/settings/security/model.ts";

function recoveryJourney(
  evidence: EvidenceRef[],
  state: JourneyState = "done",
): Journey {
  return {
    journey_id: "recovery-audit-22-33-356",
    kind: "onboarding",
    state: "done",
    state_copy_key: "status.done",
    stages: [{
      stage_id: "recovery",
      copy_key: "onboarding.stage.putting-recovery-in-place",
      state,
      evidence,
    }],
    evidence: [],
    started_at: "2026-01-01T00:00:00Z",
    updated_at: "2026-01-01T00:01:00Z",
  };
}

function roundTripJourney(journey: Journey): Journey {
  const wire = JSON.parse(JSON.stringify(encodeJourney(journey))) as JsonValue;
  const decoded = decodeJourney(wire, "recovery journey");
  assert.deepEqual(decoded, journey, "compatibility decoding must preserve evidence");
  return decoded;
}

function recovery_retired_evidence_rungs_are_not_ready() {
  const levels: readonly VerificationLevel[] = [
    "paxeer-finalised",
    "settlement-anchored",
    "unverified",
  ];
  for (const verification of levels) {
    for (const state of ["done", "done-finalised"] as const) {
      const journey = roundTripJourney(recoveryJourney([{
        evidence_id: `recovery-${verification}`,
        class: "layerx-receipt",
        verification,
      }], state));
      assert.equal(journey.stages[0]?.evidence[0]?.verification, verification);
      assert.deepEqual(recoveryPresentation(journey), { ready: false }, `${verification}/${state}`);
    }
  }
}

function recovery_current_receipts_require_a_completed_stage() {
  for (const verification of ["receipt-verified", "checkpoint-finalised"] as const) {
    const receipt: EvidenceRef = {
      evidence_id: `recovery-${verification}`,
      class: "layerx-receipt",
      verification,
    };
    for (const state of journeyStateVariants) {
      const journey = roundTripJourney(recoveryJourney([receipt], state));
      assert.deepEqual(recoveryPresentation(journey), {
        ready: state === "done" || state === "done-finalised",
        receipt,
      }, `${verification}/${state}`);
    }
  }
}

function recovery_selects_only_current_receipts_in_the_recovery_stage() {
  const receipt: EvidenceRef = {
    evidence_id: "current-recovery-receipt",
    class: "layerx-receipt",
    verification: "receipt-verified",
  };
  const retired: EvidenceRef[] = (["paxeer-finalised", "settlement-anchored"] as const).map(
    (verification) => ({
      ...receipt,
      evidence_id: `historical-${verification}`,
      verification,
    }),
  );
  assert.deepEqual(
    recoveryPresentation(roundTripJourney(recoveryJourney([...retired, receipt]))),
    { ready: true, receipt },
    "historical evidence must not mask the current recovery receipt",
  );

  for (const evidenceClass of evidenceClassVariants) {
    if (evidenceClass === "layerx-receipt") {
      continue;
    }
    assert.deepEqual(recoveryPresentation(roundTripJourney(recoveryJourney([{
      ...receipt,
      class: evidenceClass,
    }]))), { ready: false }, evidenceClass);
  }

  const journey = recoveryJourney([]);
  journey.evidence = [receipt];
  journey.stages.push({
    stage_id: "other-stage",
    copy_key: "onboarding.stage.creating-your-account",
    state: "done-finalised",
    evidence: [receipt],
  });
  assert.deepEqual(recoveryPresentation(roundTripJourney(journey)), { ready: false });
  journey.stages = [];
  assert.deepEqual(recoveryPresentation(roundTripJourney(journey)), { ready: false });
}

test(recovery_retired_evidence_rungs_are_not_ready);
test(recovery_current_receipts_require_a_completed_stage);
test(recovery_selects_only_current_receipts_in_the_recovery_stage);
