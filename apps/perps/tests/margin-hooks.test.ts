import { describe, it } from "node:test";
import { strict as assert } from "node:assert";
import { existsSync, readFileSync } from "node:fs";
import { decodeFunctionData, encodeFunctionData, getAddress, type Address } from "viem";

import { buildGetAccountSummary, buildGetPositionMargin, buildTopUpPosition } from "../src/core/perps/calls";
import { PERPS_SPENDER, planApproval } from "../src/core/network/approvals";
import { COLLATERAL_TOKENS, erc20Abi } from "../src/core/perps/config";
import { toCollateralUnits } from "../src/core/perps/collateralInputs";
import { SPOT_NATIVE_TOKENS } from "../src/hooks/spot/useSpotApprovals";

const PERPS: Address = getAddress("0x0000000000000000000000000000000000001015");
const USDL: Address = getAddress("0x85FcD13735F4309833A503EE804ea32395851479");
const ACCOUNT: Address = getAddress("0x00000000000000000000000000000000000000a1");

const path = (file: string) => new URL("../" + file, import.meta.url);
const read = (file: string) => readFileSync(path(file), "utf8");
const HOOKS = "src/hooks/perps/useMarginMode.ts";

describe("margin hooks", () => {
  it("exportsOnlyTheThreeAccountHooks", async () => {
    const mod = await import("../src/hooks/perps/useMarginMode");
    assert.deepEqual(Object.keys(mod).sort(), ["useAccountSummary", "usePositionMargin", "useTopUpPosition"]);
    const exported = [...read(HOOKS).matchAll(/^export\s+(?:async\s+)?(?:function|const|let|class)\s+(\w+)/gm)].map(
      (m) => m[1],
    );
    assert.deepEqual(exported.sort(), ["useAccountSummary", "usePositionMargin", "useTopUpPosition"]);
  });

  it("vaultHookIsAbsent", () => {
    assert.equal(existsSync(path("src/hooks/perps/useVault.ts")), false);
    const text = read(HOOKS);
    for (const name of [
      "useVault",
      "getVaultAddress",
      "createVault",
      "TradingAccountABI",
      "useMarginMode",
      "useSetMarginMode",
      "useTransferMargin",
      "useEmergencyWithdraw",
      "transferMargin",
      "emergencyWithdraw",
    ]) {
      assert.equal(text.includes(name), false, `${HOOKS} names ${name}`);
    }
  });

  it("hooksSendThePerpsBuilders", () => {
    const text = read(HOOKS);
    assert.match(text, /writeContract\(buildTopUpPosition\(args\.positionId, args\.amount\)\)/);
    assert.match(text, /readContract\(buildGetPositionMargin\(positionId\)\)/);
    assert.match(text, /readContract\(\s*buildGetAccountSummary\(userAddress, tokenAddress\),?\s*\)/);
  });

  it("buildersTargetTheAccountAt0x1015", () => {
    const topUp = buildTopUpPosition(7n, 15_000_000n);
    assert.equal(topUp.address, PERPS);
    assert.equal(topUp.functionName, "topUpPosition");
    assert.deepEqual(topUp.args, [7n, 15_000_000n]);

    const margin = buildGetPositionMargin(7n);
    assert.equal(margin.address, PERPS);
    assert.equal(margin.functionName, "getPositionMargin");
    assert.deepEqual(margin.args, [7n]);

    const summary = buildGetAccountSummary(ACCOUNT, USDL);
    assert.equal(summary.address, PERPS);
    assert.equal(summary.functionName, "getAccountSummary");
    assert.deepEqual(summary.args, [ACCOUNT, USDL]);
  });

  it("refusesZeroTopUpAndZeroAccount", () => {
    assert.throws(
      () => buildTopUpPosition(7n, 0n),
      (err: unknown) => (err as { code?: unknown }).code === "VALIDATION_AMOUNT_INVALID",
    );
    assert.throws(
      () => buildGetAccountSummary(getAddress("0x0000000000000000000000000000000000000000"), USDL),
      (err: unknown) => (err as { code?: unknown }).code === "WALLET_NOT_CONNECTED",
    );
  });

  it("topUpApprovesExactUsdlOnlyWhenAllowanceIsShort", async () => {
    const amount = toCollateralUnits("1.000001");
    assert.equal(amount, 1_000_001n);
    assert.equal(COLLATERAL_TOKENS.usdl.address, USDL);
    assert.equal(PERPS_SPENDER, PERPS);
    const topUp = buildTopUpPosition(7n, amount);
    assert.deepEqual(
      decodeFunctionData({
        abi: topUp.abi,
        data: encodeFunctionData(topUp),
      }),
      { functionName: "topUpPosition", args: [7n, amount] },
    );

    for (const allowance of [0n, amount - 1n, amount, amount + 1n]) {
      const plan = await planApproval(
        COLLATERAL_TOKENS.usdl.address,
        PERPS_SPENDER,
        () => Promise.resolve(allowance),
        amount,
        SPOT_NATIVE_TOKENS,
      );
      assert.equal(plan.length, allowance < amount ? 1 : 0);
      if (allowance < amount) {
        assert.deepEqual(plan, [{ address: USDL, functionName: "approve", args: [PERPS, amount] }]);
        assert.deepEqual(
          decodeFunctionData({
            abi: erc20Abi,
            data: encodeFunctionData({ ...plan[0], abi: erc20Abi }),
          }),
          { functionName: "approve", args: [PERPS, amount] },
        );
      }
    }
  });

  it("topUpAwaitsAllowanceBeforeSubmittingAndGuardsConcurrentCalls", () => {
    const text = read(HOOKS);
    const start = text.indexOf("export function useTopUpPosition");
    const end = text.indexOf("export function useAccountSummary", start);
    const topUp = text.slice(start, end);
    assert.match(topUp, /if \(pendingRef\.current\) throw new Error\("Position top-up already pending"\)/);
    assert.ok(topUp.indexOf("pendingRef.current = true") < topUp.indexOf("await ensureAllowance("));
    assert.match(topUp, /args = \{ \.\.\.args \}/);
    assert.match(
      topUp,
      /try \{\s*buildTopUpPosition\(args\.positionId, args\.amount\);\s*await ensureAllowance\(\{\s*token: COLLATERAL_TOKENS\.usdl\.address,\s*owner: userAddress,\s*spender: PERPS_SPENDER,\s*amount: args\.amount,\s*\}\);\s*const hash = await writeContract\(buildTopUpPosition\(args\.positionId, args\.amount\)\);\s*await waitForTx\(hash\)/,
    );
    assert.equal((topUp.match(/await writeContract\(/g) ?? []).length, 1);
    assert.match(topUp, /catch \(e\) \{[\s\S]*setError\(err\);\s*throw err;/);
    assert.match(topUp, /finally \{\s*pendingRef\.current = false;\s*setIsPending\(false\)/);

    const approvals = read("src/hooks/spot/useSpotApprovals.ts");
    assert.match(approvals, /receipt = await approveExact\(\{/);
    assert.match(approvals, /return waitForTx\(hash\)/);
  });
});
