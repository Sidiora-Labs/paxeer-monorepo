// Proves: apps/web/src/kit/model.ts verificationRung, VERIFICATION_PRESENTATIONS, verificationWord -> exported TypeScript module imports -> apps/web/src/journeys/home/model.ts homeBalance, homeAgentRows
// Repairs homeBalance accepting retired or malformed verification as verified, and
// VERIFICATION_PRESENTATIONS allowing nested copy keys to change despite Object.freeze.
// The kit is stateless: repeated calls and changed inputs prove fresh consumption.
import assert from "node:assert/strict";
import test from "node:test";

import { copyEntry } from "../copy/runtime.ts";
import type { AccountBalance, Agent, VerificationLevel } from "../src/api/index.ts";
import { homeAgentRows, homeBalance } from "../src/journeys/home/model.ts";
import {
  VERIFICATION_PRESENTATIONS,
  verificationRung,
  verificationWord,
} from "../src/kit/model.ts";

function balanceInput(verification: VerificationLevel): AccountBalance {
  return {
    account_id: "account-kit-proof",
    money: { amount: 125n, currency: "LXP" },
    verification,
    freshness: {
      observed_at: "2026-01-01T00:00:00Z",
      age_seconds: 2,
      source_head: "42",
      within_bound: true,
    },
    evidence: [{ evidence_id: "receipt-kit-proof", class: "layerx-receipt", verification }],
  };
}

function agentInput(verification: VerificationLevel): Agent {
  return {
    agent_id: "agent-kit-proof",
    name: "Kit proof",
    purpose: "Show verified spend",
    state: "active",
    state_copy_key: "agent.state.active",
    limit: {
      monthly: { amount: 500n, currency: "LXP" },
      enforcement: "protocol",
      enforcement_copy_key: "agent.limit.enforced",
    },
    spend: {
      period_start: "2026-01-01T00:00:00Z",
      period_end: "2026-02-01T00:00:00Z",
      spent: { amount: 125n, currency: "LXP" },
      remaining: { amount: 375n, currency: "LXP" },
      verification,
    },
    evidence: [],
    created_at: "2026-01-01T00:00:00Z",
    updated_at: "2026-01-01T00:00:00Z",
  };
}

function kit_producers_reach_home_consumers_twice() {
  const cases = [
    ["unverified", "verification.unverified", "Not verified yet"],
    ["receipt-verified", "verification.receipt_verified", "Verified against its receipt"],
    ["checkpoint-finalised", "verification.checkpoint_finalised", "Finalised in a committed Paxeer block"],
  ] as const;
  assert.deepEqual(Object.keys(VERIFICATION_PRESENTATIONS), cases.map(([level]) => level));

  for (const [level, copyKey, expectedWord] of cases) {
    const rung = verificationRung(level);
    assert.equal(rung, level);
    assert.equal(VERIFICATION_PRESENTATIONS[rung].copyKey, copyKey);
    assert.equal(copyEntry(VERIFICATION_PRESENTATIONS[rung].copyKey).message, expectedWord);
    assert.equal(verificationWord(rung), expectedWord);
    const balance = balanceInput(rung);
    const agent = agentInput(rung);
    for (const amount of [125n, 250n]) {
      balance.money.amount = amount;
      agent.spend.spent.amount = amount;
      const firstRows = homeAgentRows([agent]);
      const secondRows = homeAgentRows([agent]);
      assert.deepEqual(secondRows, firstRows);
      assert.notStrictEqual(secondRows[0], firstRows[0]);
      assert.equal(secondRows[0]?.spendVerification, expectedWord);
      assert.equal(secondRows[0]?.spend, `Spent LXP\u00a0${String(amount)} of LXP\u00a0500 this month`);

      const firstBalance = homeBalance(balance);
      const secondBalance = homeBalance(balance);
      assert.deepEqual(secondBalance, firstBalance);
      assert.notStrictEqual(secondBalance, firstBalance);
      if (rung === "unverified") {
        assert.equal(secondBalance.kind, "unavailable");
      } else {
        assert.equal(secondBalance.kind, "verified");
        assert.equal(secondBalance.amount, Number(amount));
        assert.equal(secondBalance.currency, "LXP");
        assert.equal(secondBalance.verification, expectedWord);
        assert.equal(secondBalance.current, true);
      }
    }
  }
}

function kit_refuses_retired_and_malformed_verification_at_home_boundary() {
  const balance = balanceInput(verificationRung("receipt-verified"));
  const expected = {
    kind: "unavailable",
    label: "Verified balance",
    message: "Your balance can't be shown right now",
  };
  assert.deepEqual(homeBalance(), expected);
  assert.deepEqual(homeBalance({ ...balance, evidence: [] }), expected);
  for (const level of [
    "unverified", "paxeer-finalised", "settlement-anchored", "unknown", "",
    "__proto__", "constructor", "RECEIPT-VERIFIED", null, undefined, 1,
  ]) {
    // Deliberately malformed runtime input; the consumer must refuse to call it verified.
    // @ts-expect-error the runtime boundary may receive undeclared verification values
    const malformed: AccountBalance = { ...balance, verification: level };
    // @ts-expect-error evidence verification is independently untrusted at the same boundary
    const malformedEvidence: AccountBalance = { ...balance, evidence: [{ ...balance.evidence[0]!, verification: level }] };
    for (let observation = 0; observation < 2; observation += 1) {
      assert.deepEqual(homeBalance(malformed), expected, String(level));
      assert.deepEqual(homeBalance(malformedEvidence), expected, String(level));
    }
  }

  const agent = agentInput(verificationRung("receipt-verified"));
  for (const level of ["receipt-verified", "settlement-anchored", "checkpoint-finalised"] as const) {
    balance.verification = level;
    agent.spend.verification = level;
    assert.equal(homeBalance(balance).kind, level === "settlement-anchored" ? "unavailable" : "verified");
    assert.equal(homeAgentRows([agent])[0]?.spendVerification, verificationWord(level));
  }
}

function kit_presentation_producer_cannot_be_corrupted() {
  assert.equal(Object.isFrozen(VERIFICATION_PRESENTATIONS), true);
  for (const level of Object.keys(VERIFICATION_PRESENTATIONS)) {
    const rung = verificationRung(level);
    const presentation = VERIFICATION_PRESENTATIONS[rung];
    assert.equal(Object.isFrozen(presentation), true);
    assert.equal(Reflect.set(presentation, "copyKey", "verification.unverified"), false);
    assert.equal(Reflect.set(VERIFICATION_PRESENTATIONS, rung, { copyKey: "unknown" }), false);
    const expectedWord = copyEntry(presentation.copyKey).message;
    const agent = agentInput(rung);
    assert.equal(homeAgentRows([agent])[0]?.spendVerification, expectedWord);
    assert.equal(homeAgentRows([agent])[0]?.spendVerification, expectedWord);
  }
}

test(kit_producers_reach_home_consumers_twice);
test(kit_refuses_retired_and_malformed_verification_at_home_boundary);
test(kit_presentation_producer_cannot_be_corrupted);
