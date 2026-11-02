import { strict as assert } from "node:assert";
import { test } from "node:test";
import {
  decodeFunctionData,
  decodeFunctionResult,
  encodeFunctionData,
  encodeFunctionResult,
  type Address,
} from "viem";

import { PRECOMPILES } from "../src/core/network/precompiles";
import { pecorVaultAbi } from "../src/core/network/spot_abi/PECORVault";
import { buildGetTokenInfo, buildIsTokenPaused } from "../src/core/spot/calls";
import { spotTokenState, type VaultTokenInfo } from "../src/core/spot/tokenState";

function pausedTokenNotTradeable() {
  for (const isRegistered of [true, false]) {
    const info: VaultTokenInfo = [isRegistered, false, 6, 10_000_000n, 12_000_000n, 2_000_000n];
    const infoData = encodeFunctionResult({ abi: pecorVaultAbi, functionName: "getTokenInfo", result: info });
    const decodedInfo = decodeFunctionResult({ abi: pecorVaultAbi, functionName: "getTokenInfo", data: infoData });
    assert.deepEqual(decodedInfo, info);

    for (const paused of [true, false]) {
      const pauseData = encodeFunctionResult({ abi: pecorVaultAbi, functionName: "isTokenPaused", result: paused });
      const decodedPaused = decodeFunctionResult({
        abi: pecorVaultAbi,
        functionName: "isTokenPaused",
        data: pauseData,
      });
      assert.deepEqual(spotTokenState(decodedPaused, decodedInfo), {
        tradeable: isRegistered && !paused,
        paused,
        reason: paused ? "TokenPaused" : isRegistered ? null : "TokenNotRegistered",
      });
    }
  }
}

function tokenStateCallsTargetVault() {
  const tokens: readonly Address[] = [
    "0x0000000000000000000000000000000000000011",
    "0x0000000000000000000000000000000000000022",
  ];
  for (const token of tokens) {
    const pauseCall = buildIsTokenPaused(token);
    const infoCall = buildGetTokenInfo(token);
    assert.equal(pauseCall.functionName, "isTokenPaused");
    assert.equal(infoCall.functionName, "getTokenInfo");

    for (const call of [pauseCall, infoCall]) {
      assert.equal(call.address, PRECOMPILES.spot.PECORVault);
      assert.equal(call.abi, pecorVaultAbi);
      assert.deepEqual(call.args, [token]);
      assert.deepEqual(decodeFunctionData({ abi: pecorVaultAbi, data: encodeFunctionData(call) }), {
        functionName: call.functionName,
        args: [token],
      });
    }
  }
}

test(pausedTokenNotTradeable);
test(tokenStateCallsTargetVault);
