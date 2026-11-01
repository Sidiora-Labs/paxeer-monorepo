import assert from "node:assert/strict";
import { readFileSync } from "node:fs";

import { PROGRAM_ABI_V5, supportsProgramGuestAbi } from "../src/generated/receipt.js";
import {
  decodeNativeProgramCall, decodeNativeProgramCallV1, encodeNativeProgramCall, encodeNativeProgramCallV1,
  type NativeProgramCall, type NativeProgramCallV1,
} from "../src/native-program-call.js";
import { NativeProgramRequestV1 } from "../src/programs.js";

const read = (path: string): Buffer => readFileSync(new URL(`../../../../../../${path}`, import.meta.url));
const fixture = <T>(name: string): T => JSON.parse(read(`sdk/platform/conformance/fixtures/${name}.json`).toString("utf8")) as T;

assert.equal(PROGRAM_ABI_V5, 5);
assert.deepEqual([0, 1, 2, 3, 4, 5, 6, 65_535].map(supportsProgramGuestAbi), [false, true, true, true, true, true, false, false]);

const call = fixture<{ payload_hex: string; fee_limit: string; signed_activity_hex: string }>("native-program-call-v4");
const payloadV4 = Buffer.from(call.payload_hex, "hex");
assert.equal(payloadV4.readUInt16BE(32), 2);
const payloadV5 = Buffer.from(payloadV4);
payloadV5.writeUInt16BE(5, 32);
const decoded = decodeNativeProgramCallV1(payloadV5);
assert.equal(decoded.guestAbi, 5);
assert.deepEqual(Buffer.from(encodeNativeProgramCallV1(decoded)), payloadV5);
const fromV4 = decodeNativeProgramCallV1(payloadV4);
const callV5: NativeProgramCallV1 = { ...fromV4, guestAbi: 5 };
assert.deepEqual(Buffer.from(encodeNativeProgramCallV1(callV5)), payloadV5);
assert.deepEqual({ ...decoded, guestAbi: 2 }, fromV4);
assert.throws(() => decodeNativeProgramCall(payloadV5), TypeError);
assert.throws(() => encodeNativeProgramCall({ ...fromV4, guestAbi: 5 } as unknown as NativeProgramCall), TypeError);
for (const abi of [0, 6, 65_535]) {
  const refused = Buffer.from(payloadV4);
  refused.writeUInt16BE(abi, 32);
  assert.throws(() => decodeNativeProgramCallV1(refused), TypeError);
  assert.throws(() => encodeNativeProgramCallV1({ ...fromV4, guestAbi: abi } as unknown as NativeProgramCallV1), TypeError);
}
const request = new NativeProgramRequestV1(callV5, BigInt(call.fee_limit), Buffer.from(call.signed_activity_hex, "hex"));
assert.equal(request.nativeCall.guestAbi, 5);
assert.equal(request.programId, Buffer.from(callV5.programId).toString("hex"));
assert.equal(request.budget.fuel, callV5.resources[0]);
