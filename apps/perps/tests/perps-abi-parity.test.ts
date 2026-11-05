import { strict as assert } from "node:assert";
import { readFileSync } from "node:fs";
import { test } from "node:test";

import type { Abi, AbiParameter } from "viem";

import { PerpsPrecompileABI as appPerpsPrecompileABI } from "../src/core/network/abi/PerpsPrecompile";

const PerpsPrecompileABI = appPerpsPrecompileABI;

type ComparedEntry = Extract<Abi[number], { type: "function" | "event" | "error" }>;

function parameterType(parameter: AbiParameter): string {
  if ("components" in parameter) {
    return `(${parameter.components.map(parameterType).join(",")})${parameter.type.slice(5)}`;
  }
  return parameter.type;
}

function indexEntries(abi: Abi, source: string): Map<string, ComparedEntry> {
  const entries = new Map<string, ComparedEntry>();
  for (const entry of abi) {
    if (entry.type !== "function" && entry.type !== "event" && entry.type !== "error") continue;
    const signature = `${entry.type} ${entry.name}(${entry.inputs.map(parameterType).join(",")})`;
    assert.ok(!entries.has(signature), `${source} has duplicate entry ${signature}`);
    entries.set(signature, entry);
  }
  assert.ok(entries.size > 0, `${source} has no functions, events or errors`);
  return entries;
}

function normalizeKeys(value: unknown): unknown {
  if (Array.isArray(value)) return value.map(normalizeKeys);
  if (value !== null && typeof value === "object") {
    return Object.fromEntries(
      Object.entries(value)
        .sort(([left], [right]) => (left < right ? -1 : left > right ? 1 : 0))
        .map(([key, child]) => [key, normalizeKeys(child)]),
    );
  }
  return value;
}

function perpsAbiMatchesChain() {
  const chainAbi: Abi = JSON.parse(
    readFileSync(new URL("../../../precompiles/perps/abi/abi.json", import.meta.url), "utf8"),
  );
  const app = indexEntries(PerpsPrecompileABI, "PerpsPrecompileABI");
  const chain = indexEntries(chainAbi, "precompiles/perps/abi/abi.json");

  for (const signature of [...new Set([...app.keys(), ...chain.keys()])].sort()) {
    assert.ok(app.has(signature), `PerpsPrecompileABI is missing ${signature}`);
    assert.ok(chain.has(signature), `PerpsPrecompileABI has extra entry ${signature}`);
    assert.deepEqual(
      normalizeKeys(app.get(signature)),
      normalizeKeys(chain.get(signature)),
      `PerpsPrecompileABI differs from precompiles/perps/abi/abi.json at ${signature}`,
    );
  }
}

test(perpsAbiMatchesChain);
