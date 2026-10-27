import assert from "node:assert/strict";
import { existsSync, readFileSync } from "node:fs";
import { registerHooks } from "node:module";
import test from "node:test";
import { fileURLToPath } from "node:url";
import ts from "typescript";

import type { EvidenceRef, Journey } from "../src/api/index.ts";

registerHooks({
  resolve(specifier, context, nextResolve) {
    if (specifier.startsWith(".") && context.parentURL !== undefined) {
      const candidate = new URL(specifier, context.parentURL);
      for (const suffix of [".ts", ".tsx", "/index.ts"]) {
        const source = candidate.href.endsWith(".js")
          ? new URL(candidate.href.slice(0, -3) + suffix)
          : new URL(candidate.href + suffix);
        if (existsSync(source)) return nextResolve(source.href, context);
      }
    }
    return nextResolve(specifier, context);
  },
  load(url, context, nextLoad) {
    if (/\.(ts|tsx)$/u.test(url) && !url.includes("/node_modules/")) {
      return {
        format: "module",
        shortCircuit: true,
        source: ts.transpileModule(readFileSync(new URL(url), "utf8"), {
          fileName: fileURLToPath(url),
          compilerOptions: {
            target: ts.ScriptTarget.ES2022,
            module: ts.ModuleKind.ESNext,
            jsx: ts.JsxEmit.ReactJSX,
          },
        }).outputText,
      };
    }
    const loaded = nextLoad(url, context);
    return loaded.format === "commonjs" ? { ...loaded, source: undefined } : loaded;
  },
});

const { verifiedOutcomeEvidence, receiptBacked, receiptEvidence, moveResult } =
  await import("../src/journeys/move/model.ts");

function moveJourney(evidence: EvidenceRef[], state: "done" | "done-finalised"): Journey {
  return {
    journey_id: "jny_move_rungs_12345678",
    kind: "move",
    state,
    state_copy_key: `status.${state}`,
    stages: [{ stage_id: "stg_move_12345678", copy_key: "move.title", state, evidence }],
    evidence,
    started_at: "2026-01-01T00:00:00Z",
    updated_at: "2026-01-01T00:00:00Z",
  };
}

function move_outcome_verified_on_receipt_rungs() {
  for (const verification of ["receipt-verified", "checkpoint-finalised"] as const) {
    const receipt: EvidenceRef = {
      evidence_id: `evt_move_${verification}`,
      class: "layerx-receipt",
      verification,
    };
    assert.equal(verifiedOutcomeEvidence(receipt), true, verification);
    for (const state of ["done", "done-finalised"] as const) {
      const journey = moveJourney([receipt], state);
      assert.equal(receiptBacked(journey), true);
      assert.deepEqual(receiptEvidence(journey), [receipt]);
      const result = moveResult(journey);
      assert.equal(result.kind, "done");
      assert.equal(result.status, state === "done" ? "done" : "done_finalised");
      if (result.kind !== "done") assert.fail("Receipt-backed moves must be done");
      assert.deepEqual(result.receipts, [receipt]);
      assert.equal(result.stages[0]?.status, result.status);
    }
  }
}

function move_outcome_unverified_on_paxeer_finality() {
  const rejected: EvidenceRef[] = [
    ...([
      "unverified",
      "receipt-verified",
      "checkpoint-finalised",
      "paxeer-finalised",
      "settlement-anchored",
    ] as const).map((verification): EvidenceRef => ({
      evidence_id: `evt_paxeer_${verification}`,
      class: "paxeer-finality",
      verification,
    })),
    ...(["unverified", "paxeer-finalised", "settlement-anchored"] as const).map(
      (verification): EvidenceRef => ({
        evidence_id: `evt_receipt_${verification}`,
        class: "layerx-receipt",
        verification,
      }),
    ),
  ];
  for (const evidence of rejected) {
    assert.equal(verifiedOutcomeEvidence(evidence), false, evidence.evidence_id);
  }
  for (const evidence of [...rejected.map((item) => [item]), rejected]) {
    for (const state of ["done", "done-finalised"] as const) {
      const journey = moveJourney(evidence, state);
      assert.equal(receiptBacked(journey), false);
      assert.deepEqual(receiptEvidence(journey), []);
      const result = moveResult(journey);
      assert.equal(result.kind, "in-progress");
      assert.equal(result.status, "processing");
      assert.equal(result.stages[0]?.status, "processing");
    }
  }
}

test(move_outcome_verified_on_receipt_rungs);
test(move_outcome_unverified_on_paxeer_finality);
