import assert from "node:assert/strict";
import { test } from "node:test";
import { decodeFunctionResult, encodeFunctionResult } from "viem";

import { PerpsPrecompileABI } from "../src/core/network/abi/PerpsPrecompile";
import { feeTierView } from "../src/core/perps/feeTier";

function feeTierViewFromAbi() {
  const cases = [
    {
      result: [2, 250n, 0n, 123_456n * 10n ** 18n],
      expected: { tier: 2, takerBps: 2.5, makerBps: 0, volume14dUsd: 123_456n * 10n ** 18n },
    },
    {
      result: [3, 125n, -51n, (1n << 256n) - 1n],
      expected: { tier: 3, takerBps: 1.25, makerBps: -0.51, volume14dUsd: (1n << 256n) - 1n },
    },
    {
      result: [0, 1n, 51n, 0n],
      expected: { tier: 0, takerBps: 0.01, makerBps: 0.51, volume14dUsd: 0n },
    },
  ] as const;

  for (const { result, expected } of cases) {
    const data = encodeFunctionResult({
      abi: PerpsPrecompileABI,
      functionName: "getFeeTier",
      result,
    });
    const decoded = decodeFunctionResult({
      abi: PerpsPrecompileABI,
      functionName: "getFeeTier",
      data,
    });

    assert.deepEqual(feeTierView(decoded), expected);
  }
}

test(feeTierViewFromAbi);
