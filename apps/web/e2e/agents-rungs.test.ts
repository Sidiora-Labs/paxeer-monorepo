import assert from "node:assert/strict";
import { existsSync, readFileSync } from "node:fs";
import { registerHooks } from "node:module";
import test from "node:test";
import { fileURLToPath } from "node:url";
import ts from "typescript";

import { copyEntry } from "../copy/runtime.ts";
import type {
  Agent,
  EvidenceRef,
  JourneyStage,
  VerificationLevel,
} from "../src/api/generated/index.ts";

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

const {
  agentListItems,
  evidenceReceiptVerified,
  spendPresentation,
  stageReceiptVerified,
} = await import("../src/journeys/agents/model.ts");

function agent_evidence_verified_on_receipt_rungs_only() {
  const cases: readonly (readonly [EvidenceRef["class"], VerificationLevel, boolean])[] = [
    ["layerx-receipt", "receipt-verified", true],
    ["layerx-receipt", "checkpoint-finalised", true],
    ["checkpoint-proof", "receipt-verified", true],
    ["checkpoint-proof", "checkpoint-finalised", true],
    ["layerx-receipt", "unverified", false],
    ["checkpoint-proof", "unverified", false],
    ["layerx-receipt", "paxeer-finalised", false],
    ["layerx-receipt", "settlement-anchored", false],
    ["checkpoint-proof", "paxeer-finalised", false],
    ["checkpoint-proof", "settlement-anchored", false],
    ["paxeer-finality", "receipt-verified", false],
    ["paxeer-finality", "checkpoint-finalised", false],
    ["paxeer-finality", "paxeer-finalised", false],
    ["paxeer-finality", "settlement-anchored", false],
    ["submission-record", "receipt-verified", false],
    ["local-journey-state", "checkpoint-finalised", false],
  ];
  assert.equal(evidenceReceiptVerified([]), false);
  for (const [evidenceClass, verification, expected] of cases) {
    const reference: EvidenceRef = {
      evidence_id: "agent-evidence",
      class: evidenceClass,
      verification,
    };
    const message = `${evidenceClass}: ${verification}`;
    assert.equal(evidenceReceiptVerified([reference]), expected, message);
    for (const state of ["done", "done-finalised"] as const) {
      const stage: JourneyStage = {
        stage_id: "agent-receipt",
        copy_key: "agent.create.stage.setting-up",
        state,
        evidence: [reference],
      };
      assert.equal(stageReceiptVerified(stage), expected, `${state}: ${message}`);
      assert.equal(stageReceiptVerified({ ...stage, state: "processing" }), false);
    }
  }
  assert.equal(evidenceReceiptVerified([
    { evidence_id: "retired", class: "layerx-receipt", verification: "settlement-anchored" },
    { evidence_id: "receipt", class: "layerx-receipt", verification: "receipt-verified" },
  ]), true);
}

function agent_spend_sentence_for_retired_level_is_unverified() {
  const agent: Agent = {
    agent_id: "agent-spend",
    name: "Purchase Agent",
    purpose: "Process purchases",
    state: "active",
    state_copy_key: "agent.state.active",
    created_at: "2026-08-01T00:00:00Z",
    updated_at: "2026-08-15T00:00:00Z",
    spend: {
      period_start: "2026-08-01T00:00:00Z",
      period_end: "2026-08-31T23:59:59Z",
      spent: { amount: 25000n, currency: "LXP" },
      remaining: { amount: 75000n, currency: "LXP" },
      verification: "settlement-anchored",
    },
    limit: {
      monthly: { amount: 100000n, currency: "LXP" },
      enforcement: "protocol",
      enforcement_copy_key: "agent.limit.protocol-backed",
    },
    evidence: [
      { evidence_id: "agent-state", class: "layerx-receipt", verification: "receipt-verified" },
    ],
  };
  const cases: readonly (readonly [VerificationLevel, string])[] = [
    ["settlement-anchored", "verification.unverified"],
    ["paxeer-finalised", "verification.unverified"],
    ["unverified", "verification.unverified"],
    ["receipt-verified", "verification.receipt_verified"],
    ["checkpoint-finalised", "verification.checkpoint_finalised"],
  ];
  for (const [verification, copyKey] of cases) {
    const spendingAgent: Agent = { ...agent, spend: { ...agent.spend, verification } };
    const expected = copyEntry(copyKey).message;
    assert.equal(spendPresentation(spendingAgent, "en-GB").verificationSentence, expected, verification);
    const rows = agentListItems({ agents: [spendingAgent], next_cursor: "" }, "en-GB");
    assert.equal(rows.length, 1);
    assert.equal(rows[0]?.verificationSentence, expected, verification);
  }
}

test(agent_evidence_verified_on_receipt_rungs_only);
test(agent_spend_sentence_for_retired_level_is_unverified);
