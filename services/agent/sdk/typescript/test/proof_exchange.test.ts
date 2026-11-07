/**
 * Proves: src/exchange.ts exchangePlaceOrderCall/exchangeRequestSettlementCall/sendExchangeRequestSettlement -> src/index.ts exports, precompileTransactionRequest/sendPrecompileCall -> test/support/exchange_consumers.ts consumeExchangeCall/consumeExchangeSettlementRequest (used by precompile-calls.test.ts and xweb-api.test.ts)
 *
 * Repairs src/exchange.ts precompileTransactionRequest forwarding malformed call
 * bytes, destinations and values, and exchangePlaceOrderCall leaking RangeError
 * for fractional uint8 inputs instead of PrecompileAbiError.
 * Shared consumers have no module-level assertions; ABI inventory checks stay
 * in precompile-calls.test.ts, and the published order ABI is pinned explicitly below.
 * The anchor has no persistent state. Each observation uses a fresh producer
 * call and request; changed inputs must reach both consumers on the next call.
 * The settlement request consumer returns the real request it observes. That
 * result is deliberately refused by sendPrecompileCall's hash decoder; this
 * proof does not fabricate a wallet hash or simulate chain execution.
 */
import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import { test } from "node:test";

import {
  PERPS_WRITES,
  PrecompileAbiError,
  exchangePlaceOrderCall,
  exchangeRequestSettlementCall,
  sendExchangeRequestSettlement,
  type PrecompileCall,
  type WalletRequest,
} from "../src/index.js";
import {
  consumeExchangeCall,
  consumeExchangeSettlementRequest,
  consumeExchangePlaceOrderCall,
} from "./support/exchange_consumers.js";

const sender = `0x${"12".repeat(20)}`;
const id = (byte: string): string => `0x${byte.repeat(32)}`;
const word = (value: bigint | number): string => BigInt(value).toString(16).padStart(64, "0");
const invalidValue = (error: unknown): boolean =>
  error instanceof PrecompileAbiError && error.code === "invalid_value";

function proof_exchange_catalogue_matches_published_order_abi(): void {
  // src/generated/precompile-writes.ts PERPS_WRITES exports the canonical
  // tuple key, not the legacy exchange signature. Keep that interface visible
  // without substituting its incompatible order shape for exchangePlaceOrderCall.
  const key = "placeOrder((uint256,bool,uint8,uint8,uint256,uint256,uint256,uint256,uint256,uint256,bool,uint256,uint256,uint64,bytes16))";
  assert.deepEqual(PERPS_WRITES[key], {
    types: ["(uint256,bool,uint8,uint8,uint256,uint256,uint256,uint256,uint256,uint256,bool,uint256,uint256,uint64,bytes16)"],
    payable: false,
  });
  assert.equal(Object.hasOwn(PERPS_WRITES, "placeOrder(bytes32,uint8,uint256,uint256,uint8)"), false);

  const entries = JSON.parse(readFileSync(
    new URL("../../../../../../precompiles/layerxexchange/abi.json", import.meta.url), "utf8",
  )) as readonly { type: string; name?: string; inputs?: readonly { type: string }[] }[];
  const entry = entries.find((candidate) => candidate.type === "function" && candidate.name === "placeOrder");
  assert.ok(entry, "precompiles/layerxexchange/abi.json must declare placeOrder");
  assert.deepEqual(entry.inputs?.map((input) => input.type), ["bytes32", "uint8", "uint256", "uint256", "uint8"]);
  const produced = exchangePlaceOrderCall({
    marketId: id("11"), side: 2, price: 5n, quantity: 2n, timeInForce: 0,
  });
  assert.equal(produced.data.length, 10 + 5 * 64, "the exchange producer matches the published five-input ABI");
}

function proof_exchange_order_reaches_both_consumers_twice(): void {
  const order = {
    marketId: id("11"), side: 2,
    price: 18446744073709551618n, quantity: 5000n, timeInForce: 0,
  };
  const body = order.marketId.slice(2) + word(order.side)
    + word(order.price) + word(order.quantity) + word(order.timeInForce);
  const calls: PrecompileCall[] = [];
  const requests: WalletRequest[] = [];
  for (let observation = 0; observation < 2; observation += 1) {
    const call = exchangePlaceOrderCall(order);
    consumeExchangePlaceOrderCall(call, order);
    requests.push(consumeExchangeCall(call, "placeOrder(bytes32,uint8,uint256,uint256,uint8)", body, sender));
    calls.push(call);
  }
  assert.deepEqual(calls[0], calls[1]);
  assert.notEqual(calls[0], calls[1]);
  assert.deepEqual(requests[0], requests[1]);
  assert.notEqual(requests[0], requests[1]);
  assert.notEqual(requests[0]?.params[0], requests[1]?.params[0]);

  const changed = { ...order, marketId: id("33"), price: order.price + 1n };
  const changedCall = exchangePlaceOrderCall(changed);
  consumeExchangePlaceOrderCall(changedCall, changed);
  const changedBody = changed.marketId.slice(2) + word(changed.side)
    + word(changed.price) + word(changed.quantity) + word(changed.timeInForce);
  const changedRequest = consumeExchangeCall(changedCall, "placeOrder(bytes32,uint8,uint256,uint256,uint8)", changedBody, sender);
  assert.notDeepEqual(changedRequest, requests[1]);

  for (const malformed of [
    { ...order, side: 1.5 }, { ...order, side: NaN }, { ...order, side: 256 },
    { ...order, side: -1 }, { ...order, side: Infinity },
    { ...order, timeInForce: 0.5 }, { ...order, timeInForce: Infinity },
    { ...order, timeInForce: -1 }, { ...order, timeInForce: 256 },
    { ...order, marketId: "0x11" }, { ...order, quantity: -1n },
    { ...order, price: -1n }, { ...order, quantity: 1n << 256n },
  ]) {
    assert.throws(() => exchangePlaceOrderCall(malformed), invalidValue);
  }
}

function proof_exchange_settlement_reaches_consumer_twice(): void {
  const positionId = id("41");
  const firstCall = exchangeRequestSettlementCall(positionId);
  const secondCall = exchangeRequestSettlementCall(positionId);
  const first = consumeExchangeCall(firstCall, "requestSettlement(bytes32)", positionId.slice(2), sender);
  const second = consumeExchangeCall(secondCall, "requestSettlement(bytes32)", positionId.slice(2), sender);
  assert.deepEqual(first, second);
  assert.notEqual(first, second);
  assert.notEqual(firstCall, secondCall);
  assert.notEqual(first.params[0], second.params[0]);
  const nextPosition = id("42");
  const next = consumeExchangeCall(exchangeRequestSettlementCall(nextPosition), "requestSettlement(bytes32)", nextPosition.slice(2), sender);
  assert.notDeepEqual(next, second);
  assert.throws(() => exchangeRequestSettlementCall("0x41"), invalidValue);
}

async function proof_exchange_send_reaches_consumer_and_refuses_abi_result_twice(): Promise<void> {
  const positionId = id("51");
  const observed: Readonly<{ method: string; params?: readonly unknown[] }>[] = [];
  // This is the imported assertion consumer itself at the EIP-1193 boundary.
  // It returns the observed request for inspection by the proof.
  const consumer = {
    async request(request: Readonly<{ method: string; params?: readonly unknown[] }>) {
      const result = consumeExchangeSettlementRequest(request, sender, positionId);
      assert.equal(result, request);
      observed.push(result);
      return result;
    },
  };
  for (let observation = 0; observation < 2; observation += 1) {
    await assert.rejects(sendExchangeRequestSettlement(consumer, sender, positionId),
      (error: unknown) => error instanceof PrecompileAbiError && error.code === "malformed_wallet_answer");
    assert.equal(observed.length, observation + 1);
  }
  assert.deepEqual(observed[0], consumeExchangeCall(exchangeRequestSettlementCall(positionId), "requestSettlement(bytes32)", positionId.slice(2), sender));
  assert.deepEqual(observed[1], observed[0]);
  assert.notEqual(observed[0], observed[1]);
  await assert.rejects(async () => sendExchangeRequestSettlement(consumer, sender, "0x51"), invalidValue);
  await assert.rejects(async () => sendExchangeRequestSettlement(consumer, "0x12", positionId), invalidValue);
  assert.equal(observed.length, 2, "malformed identifiers and senders must be refused before the consumer runs");
}

function proof_exchange_consumer_refuses_malformed_producer_result(): void {
  const positionId = id("61");
  const produced = exchangeRequestSettlementCall(positionId);
  const consume = (call: PrecompileCall): WalletRequest =>
    consumeExchangeCall(call, "requestSettlement(bytes32)", positionId.slice(2), sender);
  for (const malformed of [
    { ...produced, data: produced.data.slice(0, -1) },
    { ...produced, data: `${produced.data.slice(0, -2)}zz` },
    { ...produced, data: "0x" },
    { ...produced, to: produced.to.slice(0, -2) },
    { ...produced, value: -1n },
    { ...produced, value: 1n << 256n },
  ]) {
    assert.throws(() => consume(malformed), invalidValue);
  }
  assert.deepEqual(consume(produced), consume(exchangeRequestSettlementCall(positionId)));
}

test(proof_exchange_catalogue_matches_published_order_abi);
test(proof_exchange_order_reaches_both_consumers_twice);
test(proof_exchange_settlement_reaches_consumer_twice);
test(proof_exchange_send_reaches_consumer_and_refuses_abi_result_twice);
test(proof_exchange_consumer_refuses_malformed_producer_result);
