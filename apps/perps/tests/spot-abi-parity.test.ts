import { strict as assert } from "node:assert";
import { readFileSync } from "node:fs";
import { resolve } from "node:path";
import { test } from "node:test";
import type { Abi, AbiParameter } from "viem";

import { pecorAbi as appPecorAbi } from "../src/core/network/spot_abi/PECOR";
import { pecorOrdersAbi } from "../src/core/network/spot_abi/PECOROrders";
import { pecorQuoterAbi } from "../src/core/network/spot_abi/PECORQuoter";
import { pecorStopOrdersAbi } from "../src/core/network/spot_abi/PECORStopOrders";
import { pecorVaultAbi as appPecorVaultAbi } from "../src/core/network/spot_abi/PECORVault";

const pecorAbi = appPecorAbi;
const pecorVaultAbi = appPecorVaultAbi;

const RAILS: readonly { name: string; abi: Abi; file: string }[] = [
  { name: "pecorAbi", abi: pecorAbi, file: "pecor.json" },
  { name: "pecorOrdersAbi", abi: pecorOrdersAbi, file: "orders.json" },
  { name: "pecorQuoterAbi", abi: pecorQuoterAbi, file: "quoter.json" },
  { name: "pecorStopOrdersAbi", abi: pecorStopOrdersAbi, file: "stoporders.json" },
  { name: "pecorVaultAbi", abi: pecorVaultAbi, file: "vault.json" },
];

function parameterType(parameter: AbiParameter): string {
  if ("components" in parameter) {
    return `(${parameter.components.map(parameterType).join(",")})${parameter.type.slice("tuple".length)}`;
  }
  return parameter.type;
}

function entryName(entry: Abi[number] | undefined): string {
  if (entry === undefined) return "missing";
  return "name" in entry
    ? `${entry.type} ${entry.name}(${entry.inputs.map(parameterType).join(",")})`
    : entry.type;
}

function orderedEntries(abi: Abi): Abi {
  return [...abi].sort((left, right) => entryName(left).localeCompare(entryName(right)));
}

function spotAbisMatchChain() {
  for (const { name, abi, file } of RAILS) {
    const chainPath = resolve(__dirname, "../../../precompiles/spot/abi", file);
    const chainAbi: Abi = JSON.parse(readFileSync(chainPath, "utf8"));
    assert.ok(Array.isArray(chainAbi), `${name}: ${chainPath} must contain an ABI array`);
    assert.ok(chainAbi.length > 0, `${name}: ${chainPath} must not be empty`);
    const appEntries = orderedEntries(abi);
    const chainEntries = orderedEntries(chainAbi);

    for (let index = 0; index < Math.max(appEntries.length, chainEntries.length); index++) {
      assert.deepEqual(
        appEntries[index],
        chainEntries[index],
        `${name} / ${file} entry ${index}: app ${entryName(appEntries[index])}, chain ${entryName(chainEntries[index])}`,
      );
    }
  }
}

test(spotAbisMatchChain);
