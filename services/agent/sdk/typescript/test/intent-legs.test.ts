import assert from "node:assert/strict";
import { test } from "node:test";

import { decodeIntentPlan, HumanIntentError } from "../src/intent.js";
import { PlatformSdkError } from "../src/production.js";
import { openAccountCall } from "../src/x402/receive.js";

function planDocument() {
  const call = openAccountCall("99".repeat(32));
  return {
    plan_digest: "4b".repeat(32),
    journey_kind: "transfer",
    total_fee: { amount: "2", currency: "LXP" },
    sponsored: false,
    legs: [0, 1].map((index) => ({
      index,
      mechanism: "transfer",
      domain: "paxeer",
      source: { kind: "paxeer-wallet" },
      destination: { kind: "agent", account: "agent:did:layerx:bob:main" },
      money: { amount: "500000", currency: "LXP" },
      fee: { amount: "1", currency: "LXP" },
      call: { ...call, value: call.value.toString() },
      nonce: (9007199254740993n + BigInt(index)).toString(),
    })),
    signing_requirements: [],
  };
}

function intent_plan_legs_carry_an_EVM_call_and_nonce(): void {
  const document = planDocument();
  const nativeCall = { to: `0x${"11".repeat(20)}`, data: "0x", value: ((1n << 256n) - 1n).toString() };
  document.legs[1]!.call = nativeCall;
  const plan = decodeIntentPlan(JSON.parse(JSON.stringify(document)));
  assert.equal(plan.sponsored, false);
  assert.equal(plan.legs.length, 2);
  for (const [index, leg] of plan.legs.entries()) {
    const encoded = document.legs[index]!;
    assert.deepEqual(leg.call, { ...encoded.call, value: BigInt(encoded.call.value) });
    assert.equal(leg.nonce, 9007199254740993n + BigInt(index));
    assert(Object.isFrozen(leg.call));
  }

  for (const start of [0n, (1n << 64n) - 2n]) {
    const decoded = decodeIntentPlan({
      ...document,
      legs: document.legs.map((leg, index) => ({ ...leg, nonce: (start + BigInt(index)).toString() })),
    });
    assert.deepEqual(decoded.legs.map((leg) => leg.nonce), [start, start + 1n]);
  }

  const sponsored = decodeIntentPlan({
    ...document,
    sponsored: true,
    legs: document.legs.map((leg) => ({ ...leg, nonce: null })),
  });
  assert.equal(sponsored.sponsored, true);
  assert.deepEqual(sponsored.legs.map((leg) => leg.nonce), [null, null]);
  assert.deepEqual(sponsored.legs.map((leg) => leg.call), plan.legs.map((leg) => leg.call));

  for (const call of [
    { ...nativeCall, to: "0x1234" },
    { ...nativeCall, data: "0x1" },
    { ...nativeCall, value: (1n << 256n).toString() },
    { ...nativeCall, value: "-1" },
    { ...nativeCall, value: "01" },
    { ...nativeCall, extra: true },
  ]) {
    assert.throws(() => decodeIntentPlan({
      ...document,
      legs: document.legs.map((leg) => ({ ...leg, call })),
    }), (error: unknown) => error instanceof PlatformSdkError && error.code === "decode-failure");
  }
  assert.throws(() => decodeIntentPlan({ ...document, sponsored: "false" }));
}

function a_plan_leg_without_a_nonce_is_refused(): void {
  const document = planDocument();
  const refusesNonce = (value: unknown, index: number): void => {
    assert.throws(() => decodeIntentPlan(value), (error: unknown) => error instanceof HumanIntentError
      && error.humanCode === "refused-by-protocol"
      && error.field === `legs[${index}].nonce`
      && error.retriability === "final"
      && error.retry === "never");
  };
  for (const index of [0, 1]) {
    const { nonce, ...missingNonce } = document.legs[index]!;
    assert.equal(typeof nonce, "string");
    refusesNonce({
      ...document,
      legs: document.legs.map((leg, position) => position === index ? missingNonce : leg),
    }, index);
    refusesNonce({
      ...document,
      legs: document.legs.map((leg, position) => position === index ? { ...leg, nonce: null } : leg),
    }, index);
  }
  for (const nonce of ["9007199254740993", "9007199254740992", "9007199254740995"]) {
    refusesNonce({ ...document, legs: [document.legs[0], { ...document.legs[1], nonce }] }, 1);
  }
  for (const nonce of ["-1", "01", "1.5", "18446744073709551616", 9007199254740992]) {
    assert.throws(() => decodeIntentPlan({
      ...document,
      legs: document.legs.map((leg) => ({ ...leg, nonce })),
    }), (error: unknown) => error instanceof PlatformSdkError && error.code === "decode-failure");
  }
}

test(intent_plan_legs_carry_an_EVM_call_and_nonce);
test(a_plan_leg_without_a_nonce_is_refused);
