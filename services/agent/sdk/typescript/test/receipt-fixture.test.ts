import { readFileSync } from "node:fs";
import { fileURLToPath } from "node:url";
import { encodeNativeProgramCall, decodeNativeProgramCall } from "../src/index.js";

function assert(condition: boolean, message: string): asserts condition {
  if (!condition) throw new Error(message);
}

function hexBytes(value: string): Uint8Array {
  assert(value.length % 2 === 0, "odd hex length");
  const bytes = new Uint8Array(value.length / 2);
  for (let index = 0; index < bytes.length; index += 1) {
    const parsed = Number.parseInt(value.slice(2 * index, 2 * index + 2), 16);
    assert(Number.isInteger(parsed), "invalid hex byte");
    bytes[index] = parsed;
  }
  return bytes;
}

function hexEqual(actual: Uint8Array, expectedHex: string, field: string): void {
  const expected = hexBytes(expectedHex);
  assert(actual.length === expected.length, `${field} length diverged`);
  for (let index = 0; index < actual.length; index += 1) {
    assert(actual[index] === expected[index], `${field} diverged at byte ${index}`);
  }
}

const native = encodeNativeProgramCall({
  programId: new Uint8Array(32).fill(0x11), guestAbi: 1, entrypoint: "layerx_call", calldata: new Uint8Array(),
  capabilities: new Uint8Array([0, 0]), accessDeclaration: new TextEncoder().encode("LayerX/programs/access-declaration/v1\0\0"),
  responseCapacity: 16, resources: [1_000_000n, 16_777_216n, 1_048_576n, 1_048_576n, 64n, 1_048_576n, 4096n],
});
hexEqual(native.slice(32, 50), "0001000b0000000000020000002700000010", "native header");
assert(decodeNativeProgramCall(native).entrypoint === "layerx_call", "native decode");
for (let length = 0; length < native.length; length++) {
  let rejected = false;
  try { decodeNativeProgramCall(native.slice(0, length)); } catch { rejected = true; }
  assert(rejected, "truncated native call accepted");
}

const nativeFixture = JSON.parse(readFileSync(fileURLToPath(new URL("../../../../../../sdk/platform/conformance/fixtures/native-program-call-v3.json", import.meta.url)), "utf8")) as { payload_hex: string; signed_activity_hex: string; activity_id_hex: string };
const { NativeProgramRequest } = await import("../src/programs.js");
const { decodeSignedProgramCall } = await import("../src/program-wire.js");
const nativeModel = decodeNativeProgramCall(hexBytes(nativeFixture.payload_hex));
const nativeRequest = new NativeProgramRequest(nativeModel, 1000n, hexBytes(nativeFixture.signed_activity_hex));
assert((await decodeSignedProgramCall(nativeRequest)).activityId === nativeFixture.activity_id_hex, "native signed binding");
const changedProgramId = nativeModel.programId.slice();
const nonzeroIndex = changedProgramId.findIndex(value => value !== 0);
changedProgramId[nonzeroIndex] = changedProgramId[nonzeroIndex] === 1 ? 2 : 1;
const mutateBytes = (value: Uint8Array): Uint8Array => {
  const changed = value.length ? value.slice() : new Uint8Array(1);
  changed[0] = (changed[0] ?? 0) ^ 1;
  return changed;
};
const mismatches: { [K in keyof typeof nativeModel]: [K, (typeof nativeModel)[K]] }[keyof typeof nativeModel][] = [
  ["programId", changedProgramId], ["guestAbi", nativeModel.guestAbi === 1 ? 2 : 1],
  ["entrypoint", (nativeModel.entrypoint[0] === "a" ? "b" : "a") + nativeModel.entrypoint.slice(1)],
  ["calldata", mutateBytes(nativeModel.calldata)], ["capabilities", mutateBytes(nativeModel.capabilities)],
  ["accessDeclaration", mutateBytes(nativeModel.accessDeclaration)],
  ["responseCapacity", (nativeModel.responseCapacity + 1) % 1_048_577],
  ["resources", [nativeModel.resources[0] ^ 1n, nativeModel.resources[1], nativeModel.resources[2], nativeModel.resources[3], nativeModel.resources[4], nativeModel.resources[5], nativeModel.resources[6]]],
];
for (const [field, value] of mismatches) {
  const original = nativeModel[field];
  assert(value instanceof Uint8Array && original instanceof Uint8Array
    ? value.length !== original.length || value.some((byte, index) => byte !== original[index])
    : Array.isArray(value) && Array.isArray(original)
      ? value.some((item, index) => item !== original[index]) : value !== original,
  `native ${field} mutation is unchanged`);
  const changed = { ...nativeModel, [field]: value };
  encodeNativeProgramCall(changed);
  let rejected = false;
  try { await decodeSignedProgramCall(new NativeProgramRequest(changed, 1000n, nativeRequest.signedActivity)); } catch { rejected = true; }
  assert(rejected, "mismatched native field accepted");
}
let rejectedNativeFee = false;
try { await decodeSignedProgramCall(new NativeProgramRequest(nativeModel, nativeRequest.budget.feeLimit - 1n, nativeRequest.signedActivity)); } catch { rejectedNativeFee = true; }
assert(rejectedNativeFee, "native envelope fee mismatch accepted");
