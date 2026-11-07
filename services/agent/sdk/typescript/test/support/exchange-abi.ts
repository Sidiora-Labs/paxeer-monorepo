import assert from "node:assert/strict";
import { readFileSync } from "node:fs";

interface ExchangeAbiInput {
  readonly name: string;
  readonly type: string;
}

interface ExchangeAbiEntry {
  readonly type: string;
  readonly name?: string;
  readonly inputs?: readonly ExchangeAbiInput[];
}

export function exchangeOrderAbi(accountBearing: boolean): {
  readonly entry: ExchangeAbiEntry;
  readonly signature: string;
} {
  const entries = JSON.parse(readFileSync(
    new URL("../../../../../../../precompiles/layerxexchange/abi.json", import.meta.url),
    "utf8",
  )) as readonly ExchangeAbiEntry[];
  const matching = entries.filter(entry => entry.type === "function" && entry.name === "placeOrder" &&
    entry.inputs?.some(input => input.name === "accountId") === accountBearing);
  assert.equal(matching.length, 1, "the production ABI must unambiguously declare the order overload");
  const entry = matching[0]!;
  return { entry, signature: `${entry.name}(${entry.inputs!.map(input => input.type).join(",")})` };
}
