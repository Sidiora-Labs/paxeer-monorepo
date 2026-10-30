import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import { test } from "node:test";

import {
  LAUNCHPAD_PRECOMPILE,
  PrecompileAbiError,
  abiSelector,
  launchpadCall,
  precompileWriteCall,
} from "../src/index.js";

interface AbiEntry {
  readonly type: string;
  readonly name?: string;
  readonly stateMutability?: string;
  readonly inputs?: readonly AbiInput[];
}

interface AbiInput {
  readonly type: string;
  readonly indexed?: boolean;
  readonly name: string;
  readonly components?: readonly AbiInput[];
}

const canonical = (input: AbiInput): string =>
  input.type.startsWith("tuple")
    ? `(${(input.components ?? []).map(canonical).join(",")})${input.type.slice(5)}`
    : input.type;

function abi(directory: string): readonly AbiEntry[] {
  return JSON.parse(
    readFileSync(
      new URL(
        `../../../../../../precompiles/${directory === "perps" ? "perps/abi" : directory}/abi.json`,
        import.meta.url,
      ),
      "utf8",
    ),
  ) as readonly AbiEntry[];
}

const word = (value: bigint | number): string => BigInt(value).toString(16).padStart(64, "0");
const id = (byte: string): string => `0x${byte.repeat(32)}`;
const address = (byte: string): string => `0x${byte.repeat(20)}`;
const text = (value: string): string => {
  const body = Buffer.from(value, "utf8").toString("hex");
  return word(body.length / 2) + body.padEnd(Math.ceil(body.length / 64) * 64, "0");
};

function issue839_launchpad_permit_create_offsets_follow_the_full_static_head() {
  const signature = "createMarketWithPermit(string,string,uint8,address,(uint256,uint256,uint8,bytes32,bytes32))";
  const entry = abi("launchpad").find(
    (candidate) => candidate.type === "function" && candidate.name === "createMarketWithPermit",
  );
  assert.ok(entry);
  assert.equal(`${entry.name}(${(entry.inputs ?? []).map(canonical).join(",")})`, signature);
  const permit = [5n, 2000000000n, 27n, id("31"), id("41")] as const;
  const permitWords = word(5) + word(2000000000) + word(27) + "31".repeat(32) + "41".repeat(32);
  for (const [name, symbol, secondOffset, length] of [
    ["A", "B", 352, 416],
    ["", "", 320, 352],
    ["é".repeat(17), "B".repeat(33), 384, 480],
  ] as const) {
    const call = launchpadCall(signature, [name, symbol, 0n, address("00"), permit]);
    assert.deepEqual(call, {
      to: LAUNCHPAD_PRECOMPILE,
      value: 0n,
      data:
        abiSelector(signature) +
        word(288) +
        word(secondOffset) +
        word(0) +
        word(0) +
        permitWords +
        text(name) +
        text(symbol),
    });
    assert.equal((call.data.length - 10) / 2, length);
  }
  for (const invalidPermit of [
    permit.slice(0, -1),
    [-1n, 2000000000n, 27n, id("31"), id("41")],
    [1n << 256n, 2000000000n, 27n, id("31"), id("41")],
    [5n, 2000000000n, 256n, id("31"), id("41")],
    [5n, 2000000000n, 27n, "0x31", id("41")],
  ]) {
    assert.throws(
      () => launchpadCall(signature, ["A", "B", 0n, address("00"), invalidPermit]),
      (error: unknown) => error instanceof PrecompileAbiError && error.code === "invalid_value",
    );
  }
}

test(issue839_launchpad_permit_create_offsets_follow_the_full_static_head);

function issue839_nested_static_tuple_widths_and_dynamic_tuple_offsets() {
  const staticTypes = ["(uint256,(uint8,bytes32))", "string"];
  const staticSignature = `nested(${staticTypes.join(",")})`;
  const staticCall = precompileWriteCall(
    LAUNCHPAD_PRECOMPILE,
    staticSignature,
    { types: staticTypes, payable: false },
    [[7n, [8n, id("31")]], "A"],
  );
  assert.equal(
    staticCall.data,
    abiSelector(staticSignature) + word(7) + word(8) + "31".repeat(32) + word(128) + text("A"),
  );

  const dynamicTypes = ["(string,(uint256,uint8))", "bytes32"];
  const dynamicSignature = `nested(${dynamicTypes.join(",")})`;
  const dynamicCall = precompileWriteCall(
    LAUNCHPAD_PRECOMPILE,
    dynamicSignature,
    { types: dynamicTypes, payable: false },
    [["A", [7n, 8n]], id("31")],
  );
  assert.equal(
    dynamicCall.data,
    abiSelector(dynamicSignature) + word(64) + "31".repeat(32) + word(96) + word(7) + word(8) + text("A"),
  );
}

test(issue839_nested_static_tuple_widths_and_dynamic_tuple_offsets);
