import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import { test } from "node:test";

import { abiSelector, PrecompileAbiError } from "../src/exchange.js";
import {
  perpsCancelOrderCall,
  perpsOpenPositionCall,
  type PerpsOpenPosition,
} from "../src/perps.js";

interface AbiEntry {
  readonly type: string;
  readonly name?: string;
  readonly inputs?: readonly { readonly type: string }[];
}

const abi = JSON.parse(readFileSync(
  new URL("../../../../../../precompiles/perps/abi/abi.json", import.meta.url),
  "utf8",
)) as readonly AbiEntry[];

function perpsSelector(name: string): string {
  const entry = abi.find((candidate) => candidate.type === "function" && candidate.name === name);
  assert.ok(entry);
  assert.ok(entry.inputs);
  return abiSelector(`${entry.name}(${entry.inputs.map((input) => input.type).join(",")})`);
}

const collateralToken = "0x1234567890abcdef1234567890abcdef12345678";
const word = (value: bigint): string => value.toString(16).padStart(64, "0");

function perps_open_position_call_uses_the_precompile_selector(): void {
  const order: PerpsOpenPosition = {
    marketId: 17n,
    collateralToken,
    collateralAmount: 4n,
    leverage: 2n,
    isLong: true,
  };
  const selector = perpsSelector("openPosition");
  for (const candidate of [
    order,
    {
      ...order,
      marketId: (1n << 256n) - 1n,
      collateralAmount: (1n << 128n) + 7n,
      leverage: (1n << 256n) - 1n,
      isLong: false,
    },
  ]) {
    assert.deepEqual(perpsOpenPositionCall(candidate), {
      to: "0x0000000000000000000000000000000000001015",
      data: selector + word(candidate.marketId) + collateralToken.slice(2).padStart(64, "0")
        + word(candidate.collateralAmount) + word(candidate.leverage) + word(candidate.isLong ? 1n : 0n),
      value: 0n,
    });
  }
  for (const invalid of [
    { ...order, marketId: -1n },
    { ...order, marketId: 1n << 256n },
    { ...order, collateralToken: "0x22" },
    { ...order, collateralToken: `0x${"22".repeat(32)}` },
    { ...order, collateralToken: collateralToken.slice(2) },
    { ...order, collateralAmount: -1n },
    { ...order, collateralAmount: 1n << 256n },
    { ...order, leverage: -1n },
    { ...order, leverage: 1n << 256n },
  ]) {
    assert.throws(() => perpsOpenPositionCall(invalid), PrecompileAbiError);
  }
}

function perps_cancel_order_call_carries_the_order_id(): void {
  const selector = perpsSelector("cancelOrder");
  for (const orderId of [0n, 9n, (1n << 128n) + 7n, (1n << 256n) - 1n]) {
    assert.deepEqual(perpsCancelOrderCall(orderId), {
      to: "0x0000000000000000000000000000000000001015",
      data: selector + word(orderId),
      value: 0n,
    });
  }
  assert.throws(() => perpsCancelOrderCall(-1n), PrecompileAbiError);
  assert.throws(() => perpsCancelOrderCall(1n << 256n), PrecompileAbiError);
}

test(perps_open_position_call_uses_the_precompile_selector);
test(perps_cancel_order_call_carries_the_order_id);
