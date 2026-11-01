import { strict as assert } from "node:assert";
import { test } from "node:test";

import {
  spotTokenState,
  type VaultTokenInfo,
} from "../src/core/spot/tokenState";

process.env.NEXT_PUBLIC_PERPS_GATEWAY_URL ??= "https://perps-gateway.invalid";

async function pausedTokenBlocksSubmit() {
  const { spotSubmitBlocked } =
    await import("../src/widgets/spot/SpotOrderForm");

  for (const registered of [true, false]) {
    const info: VaultTokenInfo = [
      registered,
      false,
      6,
      10_000_000n,
      12_000_000n,
      2_000_000n,
    ];
    const paused = spotTokenState(true, info);
    assert.equal(paused.tradeable, false);
    assert.equal(spotSubmitBlocked(paused), "TRADE_MARKET_HALTED");

    const resumed = spotTokenState(false, info);
    assert.equal(
      spotSubmitBlocked(resumed),
      registered ? null : "TokenNotRegistered",
    );
    assert.equal(
      spotSubmitBlocked(spotTokenState(true, info)),
      "TRADE_MARKET_HALTED",
    );
  }

  assert.equal(spotSubmitBlocked(null), "TokenStateUnavailable");
}

test(pausedTokenBlocksSubmit);
