import assert from "node:assert/strict";
import { test } from "node:test";

import { PrecompileAbiError } from "../src/exchange.js";
import {
  perpsCancelOrderCall,
  perpsOpenPositionCall,
  type PerpsOpenPosition,
} from "../src/perps.js";

const marketId = `0x${"11".repeat(32)}`;
const accountId = `0x${"22".repeat(32)}`;
const orderId = `0x${"0".repeat(63)}9`;
const word = (value: bigint): string => value.toString(16).padStart(64, "0");

function perps_open_position_call_uses_the_v2_selector(): void {
  const order: PerpsOpenPosition = { marketId, accountId, side: 1, size: 4n, leverage: 2n };
  for (const candidate of [
    order,
    { ...order, side: 2 as const, size: (1n << 128n) + 7n, leverage: (1n << 256n) - 1n },
  ]) {
    assert.deepEqual(perpsOpenPositionCall(candidate), {
      to: "0x0000000000000000000000000000000000001015",
      data: "0xb14bfbe6" + marketId.slice(2) + accountId.slice(2)
        + word(BigInt(candidate.side)) + word(candidate.size) + word(candidate.leverage),
      value: 0n,
    });
  }
  for (const invalid of [
    { ...order, marketId: "0x11" },
    { ...order, accountId: "0x22" },
    { ...order, size: -1n },
    { ...order, size: 1n << 256n },
    { ...order, leverage: -1n },
    { ...order, leverage: 1n << 256n },
  ]) {
    assert.throws(() => perpsOpenPositionCall(invalid), PrecompileAbiError);
  }
}

function perps_cancel_order_call_carries_the_account_id(): void {
  for (const account of [accountId, `0x${"33".repeat(32)}`]) {
    assert.deepEqual(perpsCancelOrderCall(orderId, account), {
      to: "0x0000000000000000000000000000000000001015",
      data: "0xa47d9d33" + orderId.slice(2) + account.slice(2),
      value: 0n,
    });
  }
  assert.throws(() => perpsCancelOrderCall("0x09", accountId), PrecompileAbiError);
  assert.throws(() => perpsCancelOrderCall(orderId, "0x22"), PrecompileAbiError);
  assert.throws(() => perpsCancelOrderCall(orderId.slice(2), accountId), PrecompileAbiError);
  assert.throws(() => perpsCancelOrderCall(orderId, accountId.slice(2)), PrecompileAbiError);
}

test(perps_open_position_call_uses_the_v2_selector);
test(perps_cancel_order_call_carries_the_account_id);
