import assert from "node:assert/strict";
import { test } from "node:test";
import { createElement } from "react";
import { renderToStaticMarkup } from "react-dom/server";
import { decodeFunctionResult, encodeFunctionResult } from "viem";

import { PerpsPrecompileABI } from "../src/core/network/abi/PerpsPrecompile";
import { feeTierView } from "../src/core/perps/feeTier";

async function feeTierPanelShowsChainTier() {
  process.env.NEXT_PUBLIC_PERPS_GATEWAY_URL ??= "https://gateway.test";
  const { PortfolioFeeTierPanel } = await import(
    "../src/widgets/portfolio/PortfolioFeeTierPanel"
  );
  const cases = [
    {
      result: [2, 250n, 0n, 123_456n * 10n ** 18n],
      expected: "Tier 2 · Taker 2.5 bps / Maker 0 bps",
    },
    {
      result: [3, 125n, -51n, 0n],
      expected: "Tier 3 · Taker 1.25 bps / Maker -0.51 bps",
    },
  ] as const;

  for (const { result, expected } of cases) {
    const data = encodeFunctionResult({
      abi: PerpsPrecompileABI,
      functionName: "getFeeTier",
      result,
    });
    const view = feeTierView(
      decodeFunctionResult({
        abi: PerpsPrecompileABI,
        functionName: "getFeeTier",
        data,
      }),
    );
    const html = renderToStaticMarkup(
      createElement(PortfolioFeeTierPanel, { address: undefined, view }),
    );

    assert.ok(html.includes(expected), `renders the chain fee tier: ${expected}`);
    assert.ok(!html.includes("Tier 1 · Taker 3 bps / Maker 0.5 bps rebate"));
    assert.ok(!html.includes("30d volume"));
    assert.ok(!html.includes("PoFQ"));
    assert.ok(!html.includes("rolls out"));
  }

  const disconnected = renderToStaticMarkup(
    createElement(PortfolioFeeTierPanel, { address: undefined }),
  );
  assert.ok(disconnected.includes("Connect your wallet to view your fee tier"));
  assert.ok(!disconnected.includes("Tier 1"));
}

test(feeTierPanelShowsChainTier);
