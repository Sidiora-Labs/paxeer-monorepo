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
        const source = candidate.href.endsWith(".js") ? new URL(candidate.href.slice(0, -3) + suffix) : new URL(candidate.href + suffix);
        if (existsSync(source)) return nextResolve(source.href, context);
      }
    }
    if (specifier === "server-only") return { url: "data:text/javascript,", shortCircuit: true };
    if (specifier === "next/navigation") return nextResolve("next/navigation.js", context);
    if (specifier === "next/link") return nextResolve("next/link.js", context);
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

const { accountActive, PROTOCOL_IDENTITY_STAGE_KEY } = await import("../src/journeys/onboarding/model.ts");

function protocolJourney(evidence: EvidenceRef[]): Journey {
  return {
    journey_id: "onboarding-rungs",
    kind: "onboarding",
    state: "done-finalised",
    state_copy_key: "status.done_finalised",
    stages: [{
      stage_id: "protocol-identity",
      copy_key: PROTOCOL_IDENTITY_STAGE_KEY,
      state: "done-finalised",
      evidence,
    }],
    evidence: [],
    started_at: "2026-01-01T00:00:00Z",
    updated_at: "2026-01-01T00:00:00Z",
  };
}

function onboarding_account_active_on_receipt_rung() {
  for (const verification of ["receipt-verified", "checkpoint-finalised"] as const) {
    const evidence: EvidenceRef = {
      evidence_id: `onboarding-${verification}`,
      class: "layerx-receipt",
      verification,
    };
    assert.equal(accountActive(protocolJourney([evidence])), true, verification);

    for (const evidenceClass of [
      "local-journey-state", "submission-record", "checkpoint-proof", "paxeer-finality",
      "typed-refusal", "approval-hold", "wallet-ack",
    ] as const) {
      assert.equal(accountActive(protocolJourney([{ ...evidence, class: evidenceClass }])), false, evidenceClass);
    }

    const outsideProtocolStage = protocolJourney([]);
    outsideProtocolStage.evidence = [evidence];
    outsideProtocolStage.stages.push({
      stage_id: "passkey",
      copy_key: "onboarding.stage.creating-your-passkey",
      state: "done-finalised",
      evidence: [evidence],
    });
    assert.equal(accountActive(outsideProtocolStage), false);
    outsideProtocolStage.stages = [];
    assert.equal(accountActive(outsideProtocolStage), false);
  }
}

function onboarding_account_inactive_on_retired_level() {
  for (const verification of ["paxeer-finalised", "settlement-anchored", "unverified"] as const) {
    assert.equal(accountActive(protocolJourney([{
      evidence_id: `onboarding-${verification}`,
      class: "layerx-receipt",
      verification,
    }])), false, verification);
  }
  assert.equal(accountActive(protocolJourney([])), false);
}

test(onboarding_account_active_on_receipt_rung);
test(onboarding_account_inactive_on_retired_level);
