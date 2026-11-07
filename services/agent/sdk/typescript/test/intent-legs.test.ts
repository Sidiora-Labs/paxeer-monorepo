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
    legs: [0, 1].map((index) => ({
      index,
      mechanism: "transfer",
      domain: "paxeer",
      source: { kind: "paxeer-wallet" },
      destination: { kind: "agent", account: "agent:did:layerx:bob:main" },
      money: { amount: "500000", currency: "LXP" },
      fee: { amount: "1", currency: "LXP" },
      to: call.to,
      value: call.value.toString(),
      calldata: call.data,
      nonce: Number.MAX_SAFE_INTEGER - 2 + index,
      chain_id: 125,
      gas_limit: 100000,
      max_fee_per_gas: "30",
      max_priority_fee_per_gas: "2",
    })),
    signing_requirements: [],
  };
}

function intent_plan_legs_carry_an_EVM_call_and_nonce(): void {
  const document = planDocument();
  const nativeCall = { to: `0x${"11".repeat(20)}`, calldata: "0x", value: ((1n << 256n) - 1n).toString() };
  Object.assign(document.legs[1]!, nativeCall);
  const plan = decodeIntentPlan(JSON.parse(JSON.stringify(document)));
  assert.equal(plan.sponsored, false);
  assert.equal(plan.legs.length, 2);
  for (const [index, leg] of plan.legs.entries()) {
    const encoded = document.legs[index]!;
    assert.deepEqual(leg.call, { to: encoded.to, data: encoded.calldata, value: BigInt(encoded.value) });
    assert.equal(leg.nonce, BigInt(Number.MAX_SAFE_INTEGER - 2) + BigInt(index));
    assert.equal(leg.chainId, 125n);
    assert.equal(leg.gasLimit, 100000n);
    assert.equal(leg.maxFeePerGas, 30n);
    assert.equal(leg.maxPriorityFeePerGas, 2n);
    assert(Object.isFrozen(leg.call));
  }

  for (const start of [0, Number.MAX_SAFE_INTEGER - 1]) {
    const decoded = decodeIntentPlan({
      ...document,
      legs: document.legs.map((leg, index) => ({ ...leg, nonce: start + index })),
    });
    assert.deepEqual(decoded.legs.map((leg) => leg.nonce), [BigInt(start), BigInt(start) + 1n]);
  }

  for (const nonce of [null, undefined]) {
    const sponsored = decodeIntentPlan(JSON.parse(JSON.stringify({
      ...document,
      legs: document.legs.map((leg) => ({ ...leg, nonce })),
    })));
    assert.equal(sponsored.sponsored, true);
    assert.deepEqual(sponsored.legs.map((leg) => leg.nonce), [null, null]);
    assert.deepEqual(sponsored.legs.map((leg) => leg.call), plan.legs.map((leg) => leg.call));
  }

  for (const call of [
    { ...nativeCall, to: "0x1234" },
    { ...nativeCall, calldata: "0x1" },
    { ...nativeCall, value: (1n << 256n).toString() },
    { ...nativeCall, value: "-1" },
    { ...nativeCall, value: "01" },
    { ...nativeCall, extra: true },
  ]) {
    assert.throws(() => decodeIntentPlan({
      ...document,
      legs: document.legs.map((leg) => ({ ...leg, ...call })),
    }), (error: unknown) => error instanceof PlatformSdkError && error.code === "decode-failure");
  }
  for (const sponsored of [false, true, "false"]) {
    assert.throws(() => decodeIntentPlan({ ...document, sponsored }));
  }
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
    assert.equal(typeof nonce, "number");
    refusesNonce({
      ...document,
      legs: document.legs.map((leg, position) => position === index ? missingNonce : leg),
    }, index);
    refusesNonce({
      ...document,
      legs: document.legs.map((leg, position) => position === index ? { ...leg, nonce: null } : leg),
    }, index);
  }
  const firstNonce = document.legs[0]!.nonce;
  for (const nonce of [firstNonce, firstNonce - 1, firstNonce + 2]) {
    refusesNonce({ ...document, legs: [document.legs[0], { ...document.legs[1], nonce }] }, 1);
  }
  for (const nonce of [
    "-1", "01", "1.5", "18446744073709551616",
    "9007199254740993", "9007199254740992", "9007199254740995",
    -1, 1.5, 9007199254740992,
  ]) {
    assert.throws(() => decodeIntentPlan({
      ...document,
      legs: document.legs.map((leg) => ({ ...leg, nonce })),
    }), (error: unknown) => error instanceof PlatformSdkError && error.code === "decode-failure");
  }
}

test(intent_plan_legs_carry_an_EVM_call_and_nonce);
test(a_plan_leg_without_a_nonce_is_refused);
