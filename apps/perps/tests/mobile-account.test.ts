import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import { test } from "node:test";
import * as React from "react";
import { createElement } from "react";
import { renderToStaticMarkup } from "react-dom/server";
import { decodeFunctionResult, encodeFunctionResult } from "viem";

import type { AccountState } from "../src/core/perps/calls";
import { portfolioFromAccount } from "../src/core/perps/portfolio";

const E18 = 10n ** 18n;
const ACCOUNT: AccountState = {
  collateral: 1_000_000_000n,
  equityUsd: 1_000n * E18,
  initialMarginUsd: 200n * E18,
  maintenanceMarginUsd: 0n,
  withdrawable: 800_000_000n,
  feeTier: 0,
  volume14dUsd: 0n,
  positionCount: 0n,
  openOrderCount: 1n,
};

function metrics(html: string): Record<string, string> {
  return Object.fromEntries(
    [...html.matchAll(/<span[^>]*>([^<]*)<\/span><span[^>]*>([^<]*)<\/span>/g)]
      .map((match) => [match[1], match[2]]),
  );
}

test("mobileAccountRendersNativeMaintenanceMargin", async () => {
  process.env.NEXT_PUBLIC_PERPS_GATEWAY_URL ??= "https://gateway.test";
  const { buildGetAccount } = await import("../src/core/perps/calls");
  const { MobileAccountSummary } = await import("../src/widgets/mobile/MobileAccountView");
  const call = buildGetAccount("0x00000000000000000000000000000000000a11ce");
  const reactGlobal = Object.getOwnPropertyDescriptor(globalThis, "React");
  Object.defineProperty(globalThis, "React", { value: React, configurable: true });

  function render(account: AccountState, available = true) {
    const native = decodeFunctionResult({
      abi: call.abi,
      functionName: call.functionName,
      data: encodeFunctionResult({
        abi: call.abi,
        functionName: call.functionName,
        result: account,
      }),
    });
    return metrics(renderToStaticMarkup(createElement(MobileAccountSummary, {
      ...portfolioFromAccount(native, []),
      maintenanceMarginUsd: available ? native.maintenanceMarginUsd : null,
    })));
  }

  try {
    const restingOrderOnly = render(ACCOUNT);
    assert.deepEqual(restingOrderOnly, {
      Spot: "$0.00",
      Perps: "$1,000.00",
      Balance: "$1,000.00",
      "Unrealized PNL": "+$0.00",
      "Cross Margin Ratio": "20.00%",
      "Maintenance Margin": "$0.00",
      "Cross Account Leverage": "—",
    });

    for (const [amount, expected] of [
      [30n * E18, "$30.00"],
      [12_345n * 10n ** 15n, "$12.35"],
      [1_234_567_890_000_000_000_000_000n, "$1.23M"],
    ] as const) {
      const actual = render({ ...ACCOUNT, maintenanceMarginUsd: amount, positionCount: 1n });
      assert.deepEqual(actual, { ...restingOrderOnly, "Maintenance Margin": expected });
    }

    assert.equal(render({
      ...ACCOUNT,
      collateral: 0n,
      equityUsd: -5n * E18,
      withdrawable: 0n,
      maintenanceMarginUsd: 20n * E18,
      positionCount: 1n,
    })["Maintenance Margin"], "$20.00");

    assert.deepEqual(render(ACCOUNT, false), {
      ...restingOrderOnly,
      "Maintenance Margin": "Unavailable",
    });
    assert.equal(render(ACCOUNT)["Maintenance Margin"], "$0.00");
  } finally {
    if (reactGlobal) Object.defineProperty(globalThis, "React", reactGlobal);
    else Reflect.deleteProperty(globalThis, "React");
  }
});

test("portfolioHookForwardsNativeMaintenanceAndClearsUnavailableReads", () => {
  const source = readFileSync(new URL("../src/hooks/perps/usePortfolioStats.ts", import.meta.url), "utf8");
  const mobile = readFileSync(new URL("../src/widgets/mobile/MobileAccountView.tsx", import.meta.url), "utf8");
  assert.match(source, /readContract\(buildGetAccount\(/);
  assert.match(source, /_setRaw\(\{ account, accountAddress: addr, maintenanceMarginUsd: account\.maintenanceMarginUsd \}\)/);
  assert.match(source, /catch \{\s*if \(_activeAddress === addr\) _setRaw\(\{ maintenanceMarginUsd: null \}\)/);
  assert.match(source, /_setRaw\(\{ isLoading: true, accountAddress: null, maintenanceMarginUsd: null \}\)/);
  assert.match(source, /maintenanceMarginUsd: wantedAddress !== null && raw\.accountAddress === wantedAddress\s*\? raw\.maintenanceMarginUsd\s*: null/);
  assert.match(mobile, /return <MobileAccountSummary \{\.\.\.stats\} \/>/);
  assert.doesNotMatch(mobile, /lockedMargin\s*\*\s*0\.05/);
});
