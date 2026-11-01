import { strict as assert } from "node:assert";
import { test } from "node:test";
import { decodeFunctionResult, encodeFunctionResult } from "viem";

import { PerpsPrecompileABI } from "../src/core/network/abi/PerpsPrecompile";
import { markStatus, type MarkPriceDetails } from "../src/core/perps/markStatus";

function markStatusStaleAndDisabled() {
  for (const fresh of [false, true]) {
    const data = encodeFunctionResult({
      abi: PerpsPrecompileABI,
      functionName: "getMarkPriceDetails",
      result: [50_001n * 10n ** 18n, 50_000n * 10n ** 18n, 50_002n * 10n ** 18n, fresh],
    });
    const [markPrice, indexPrice, midPrice, decodedFresh] = decodeFunctionResult({
      abi: PerpsPrecompileABI,
      functionName: "getMarkPriceDetails",
      data,
    });
    const details: MarkPriceDetails = { markPrice, indexPrice, midPrice, fresh: decodedFresh };

    assert.equal(markStatus(details, true), fresh ? "open" : "stale");
    assert.equal(markStatus(details, false), "disabled");
  }
}

test(markStatusStaleAndDisabled);
