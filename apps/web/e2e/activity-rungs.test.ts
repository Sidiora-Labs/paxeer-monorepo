import assert from "node:assert/strict";
import { existsSync, readFileSync } from "node:fs";
import { registerHooks } from "node:module";
import test from "node:test";
import { fileURLToPath } from "node:url";
import ts from "typescript";

import type { EvidenceRef, VerificationLevel } from "../src/api/index.ts";

registerHooks({
  resolve(specifier, context, nextResolve) {
    if (specifier.startsWith(".") && context.parentURL !== undefined) {
      const candidate = new URL(specifier, context.parentURL);
      for (const suffix of [".ts", ".tsx", "/index.ts"]) {
        const source = candidate.href.endsWith(".js") ? new URL(candidate.href.slice(0, -3) + suffix) : new URL(candidate.href + suffix);
        if (existsSync(source)) return nextResolve(source.href, context);
      }
    }
    return nextResolve(specifier, context);
  },
  load(url, context, nextLoad) {
    if (/\.(ts|tsx)$/u.test(url) && !url.includes("/node_modules/")) {
      return {
        format: "module", shortCircuit: true,
        source: ts.transpileModule(readFileSync(new URL(url), "utf8"), {
          fileName: fileURLToPath(url),
          compilerOptions: { target: ts.ScriptTarget.ES2022, module: ts.ModuleKind.ESNext, jsx: ts.JsxEmit.ReactJSX },
        }).outputText,
      };
    }
    const loaded = nextLoad(url, context);
    return loaded.format === "commonjs" ? { ...loaded, source: undefined } : loaded;
  },
});

const { copyEntry } = await import("../copy/runtime.ts");
const { entryVerification, receiptBacked, verificationLabel } = await import("../src/journeys/activity/model.ts");

function receiptEvidence(verification: VerificationLevel): EvidenceRef {
  return { class: "layerx-receipt", evidence_id: "rcpt_activity_rungs", verification };
}

function activity_entry_verification_ranks_three_rungs() {
  const rungs = [
    ["unverified", "activity.verification.unverified"],
    ["receipt-verified", "activity.verification.receipt_verified"],
    ["checkpoint-finalised", "activity.verification.checkpoint_finalised"],
  ] as const;
  assert.equal(entryVerification([]), "unverified");
  for (const [level, key] of rungs) {
    assert.equal(entryVerification([receiptEvidence(level)]), level);
    assert.equal(verificationLabel(level), copyEntry(key).message);
  }

  const receipt = receiptEvidence("receipt-verified");
  const checkpoint: EvidenceRef = {
    class: "checkpoint-proof", evidence_id: "chkpt_activity_rungs", verification: "checkpoint-finalised",
  };
  assert.equal(entryVerification([receipt, checkpoint]), "checkpoint-finalised");
  assert.equal(entryVerification([checkpoint, receipt]), "checkpoint-finalised");

  const retired = ["paxeer-finalised", "settlement-anchored"] as const;
  for (const level of retired) {
    const evidence = receiptEvidence(level);
    assert.equal(entryVerification([evidence]), "unverified");
    assert.equal(verificationLabel(level), copyEntry("activity.verification.unverified").message);
    assert.equal(entryVerification([evidence, receipt]), "receipt-verified");
    assert.equal(entryVerification([receipt, evidence]), "receipt-verified");
    assert.equal(entryVerification([evidence, receipt, checkpoint]), "checkpoint-finalised");
    assert.equal(entryVerification([checkpoint, receipt, evidence]), "checkpoint-finalised");
  }
  assert.equal(entryVerification(retired.map(receiptEvidence)), "unverified");
}

function activity_receipt_backed_refuses_retired_levels() {
  for (const level of ["unverified", "paxeer-finalised", "settlement-anchored"] as const) {
    assert.equal(receiptBacked(receiptEvidence(level)), false, level);
  }
  for (const level of ["receipt-verified", "checkpoint-finalised"] as const) {
    assert.equal(receiptBacked(receiptEvidence(level)), true, level);
    for (const evidenceClass of ["checkpoint-proof", "paxeer-finality"] as const) {
      assert.equal(receiptBacked({ ...receiptEvidence(level), class: evidenceClass }), false, evidenceClass);
    }
  }
}

test(activity_entry_verification_ranks_three_rungs);
test(activity_receipt_backed_refuses_retired_levels);
