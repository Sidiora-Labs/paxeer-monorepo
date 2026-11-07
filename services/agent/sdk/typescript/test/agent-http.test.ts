import { once } from "node:events";
import * as http from "node:http";

import {
  AgentHttpTransport,
  LayerXKeyCredential,
  ProgramOperations,
  ProgramTrustContext,
  ProductionClient,
  SecretBytes,
} from "../src/index.js";
import { assertFreshSimulationObservation, decodeSignedProgramCall } from "../src/program-wire.js";

function assert(condition: boolean, message: string): asserts condition {
  if (!condition) throw new Error(message);
}

async function rejectsWith(action: () => Promise<unknown>, expected: string, message: string): Promise<void> {
  let failure: unknown;
  try { await action(); } catch (error) { failure = error; }
  assert(failure instanceof Error && failure.message.includes(expected), message);
}

const programId = "11".repeat(32);
const bodies: Buffer[] = [];
const server = http.createServer((request, response) => {
  const chunks: Buffer[] = [];
  request.on("data", (chunk: Buffer) => chunks.push(Buffer.from(chunk)));
  request.on("end", () => {
    bodies.push(Buffer.concat(chunks));
    assert(request.method === "GET", "program discovery did not use GET");
    assert(request.url === `/v1/programs/registry/${programId}`, "program path was not exact");
    assert(request.headers.authorization === `LayerX-Key key_1:lxp_live_${"22".repeat(32)}`, "LayerX-Key authentication changed");
    response.writeHead(200, { "Content-Type": "application/json" });
    response.end(JSON.stringify({
      request_id: "1",
      value: { program_id: programId },
      verification_status: { state: "Unverified", requested: "SequencerSigned", achieved: "Unverified", reason: "server_side_receipt_verification_only" },
    }));
  });
});
server.listen(0, "127.0.0.1");
await once(server, "listening");
const address = server.address();
assert(address !== null && typeof address === "object", "test listener missing");

try {
  const credential = new LayerXKeyCredential(
    "key_1",
    new SecretBytes(Buffer.from(`lxp_live_${"22".repeat(32)}`, "ascii")),
  );
  const client = new ProductionClient(new AgentHttpTransport({
    endpoint: `http://127.0.0.1:${address.port}`,
    credential,
  }));
  const value = await client.agent<{ program_id: string; requested_verification_level: string }, { program_id: string }>(
    "program.discover",
    { program_id: programId, requested_verification_level: "sequencer-signed" },
  );
  assert(value.program_id === programId, "success envelope value changed");
  assert(
    JSON.parse(bodies[0]?.toString("utf8") ?? "null").requested_verification_level === "sequencer-signed",
    "GET verification request body was discarded",
  );
} finally {
  server.close();
  await once(server, "close");
}

const callIdempotency = "33".repeat(32);
const signedActivity = await canonicalProgramCall(programId, callIdempotency);
const signedBinding = await decodeSignedProgramCall({
  programId,
  calldata: new Uint8Array([0xaa]),
  budget: { fuel: 1n, feeLimit: 0n },
  capabilities: [],
  signedActivity,
});
assert(signedBinding.notBefore === 10n && signedBinding.notAfter === 20n, "signed Programs validity window was discarded");
const mismatchedLegacyEnvelope = new Uint8Array(signedActivity);
mismatchedLegacyEnvelope[1] = 1;
await rejectsWith(
  () => decodeSignedProgramCall({
    programId,
    calldata: new Uint8Array([0xaa]),
    budget: { fuel: 1n, feeLimit: 0n },
    capabilities: [],
    signedActivity: mismatchedLegacyEnvelope,
  }),
  "protocol",
  "legacy outer envelope accepted a protocol v2 Programs activity",
);
assertFreshSimulationObservation(15n, signedBinding, 15n, 5n);
for (const [observedAt, now, maximumAge] of [[9n, 15n, 10n], [21n, 21n, 10n], [15n, 14n, 10n], [15n, 21n, 5n]] as const) {
  let rejected = false;
  try { assertFreshSimulationObservation(observedAt, signedBinding, now, maximumAge); } catch { rejected = true; }
  assert(rejected, "out-of-window Programs simulation observation was accepted");
}
let zeroKeyRejected = false;
try { new ProgramTrustContext(new Uint8Array(32)); } catch { zeroKeyRejected = true; }
assert(zeroKeyRejected, "all-zero Programs sequencer key was accepted");

const trustNow = 15n;
let headSequence = 10n;
let headRoot = "55".repeat(32);
const trustServer = http.createServer((request, response) => {
  assert(request.url === `/v1/programs/registry/${programId}`, "unexpected Programs trust route");
  response.writeHead(200, { "Content-Type": "application/json" });
  response.end(programEnvelope({
    program_id: programId, lifecycle: "active", version: 1, code_hash: "22".repeat(32), abi_version: 2,
    receipt_digest: "33".repeat(32), state_root: headRoot, observed_sequence: headSequence.toString(),
    observed_at: "10", valid_through: "20", verification: "registry-receipt-and-current-head-verified",
  }, false));
});
trustServer.listen(0, "127.0.0.1");
await once(trustServer, "listening");
const trustAddress = trustServer.address();
assert(trustAddress !== null && typeof trustAddress === "object", "Programs trust listener missing");
try {
  const trustPrograms = new ProgramOperations(
    new ProductionClient(new AgentHttpTransport({ endpoint: `http://127.0.0.1:${trustAddress.port}` })),
    new ProgramTrustContext(Uint8Array.from({ length: 32 }, () => 0x44), () => trustNow, 5n),
  );
  await trustPrograms.discover(programId);
  headSequence = 9n;
  await rejectsWith(() => trustPrograms.discover(programId), "rollback or conflict", "Programs cache accepted a sequence rollback");
  headSequence = 10n;
  headRoot = "66".repeat(32);
  await rejectsWith(() => trustPrograms.discover(programId), "rollback or conflict", "Programs cache accepted a conflicting root");
  headRoot = "55".repeat(32);
} finally {
  trustServer.close();
  await once(trustServer, "close");
}

function programEnvelope(value: Readonly<Record<string, unknown>>, achieved: boolean): string {
  return JSON.stringify({
    request_id: "1",
    value,
    verification_status: achieved
      ? { state: "Achieved", level: "SequencerSigned" }
      : { state: "Unverified", requested: "SequencerSigned", achieved: "Unverified", reason: "server_side_receipt_verification_only" },
  });
}

async function canonicalProgramCall(callee: string, idempotency: string): Promise<Uint8Array> {
  const payload = join(
    Buffer.from("LayerX/programs/call/v1\0", "utf8"),
    Buffer.from(callee, "hex"),
    integer(1n, 8), integer(0n, 16), integer(0n, 2),
    sized(Buffer.from([0xaa])),
  );
  const payloadHash = await hash(join(Buffer.from("LXP/v1/payload-hash\0", "utf8"), payload));
  return join(
    integer(2n, 2), integer(0x1001n, 2), Buffer.from([12]),
    Buffer.from([1]), integer(2n, 2),
    Buffer.from([2]), integer(1n, 4),
    Buffer.from([3]), integer(0x0009_0003n, 4),
    Buffer.from([4]), sized(Buffer.from("did:lxp:test", "utf8")),
    Buffer.from([5]), sized(Buffer.from([1])),
    Buffer.from([6]), integer(0n, 8),
    Buffer.from([7]), integer(10n, 8), integer(20n, 8),
    Buffer.from([8]), sized(Buffer.from(idempotency, "hex")),
    Buffer.from([9]), integer(0n, 16),
    Buffer.from([10]), sized(payloadHash),
    Buffer.from([11]), sized(payload),
    Buffer.from([12]), sized(Buffer.from([2])),
  );
}

async function hash(value: Uint8Array): Promise<Uint8Array> {
  const encoded = new ArrayBuffer(value.length);
  new Uint8Array(encoded).set(value);
  return new Uint8Array(await globalThis.crypto.subtle.digest("SHA-256", encoded));
}

function sized(value: Uint8Array): Uint8Array { return join(integer(BigInt(value.length), 4), value); }
function join(...values: readonly Uint8Array[]): Uint8Array { return Buffer.concat(values); }
function integer(value: bigint, length: number): Uint8Array {
  const result = new Uint8Array(length);
  let remaining = value;
  for (let index = length - 1; index >= 0; index -= 1) { result[index] = Number(remaining & 0xffn); remaining >>= 8n; }
  return result;
}
