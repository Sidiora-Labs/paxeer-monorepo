import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import { test } from "node:test";
import { createHash, createPublicKey, verify } from "node:crypto";
import { encodeProgramMutationBody, decodeProgramBoundaryError, decodeEnvelope } from "../src/agent-http.js";
import { bindSignedProgramLifecycle } from "../src/program-wire.js";
import { resolveLifecycleFailure } from "../src/programs.js";
import { decodeNativeProgramDeploy, decodeNativeProgramUpgrade, decodeNativeProgramWindDown,
  admitNativeProgramUpgrade, encodeNativeProgramDeploy, encodeNativeProgramUpgrade, encodeNativeProgramWindDown, NativeProgramLifecycleRequest, type NativeProgramGuestAbi } from "../src/program-lifecycle.js";

async function lifecycle_signature_payload_and_framing_refusals(): Promise<void> {
  for (const name of ["deploy", "upgrade", "wind-down-route", "wind-down-deprecate", "wind-down-tombstone", "wind-down-exit"]) {
    const fixture = JSON.parse(readFileSync(new URL(`../../../../../../sdk/platform/conformance/fixtures/native-program-${name}-v3.json`, import.meta.url), "utf8")) as Record<string, string>;
    const payload = Buffer.from(fixture.payload_hex!, "hex"), signed = Buffer.from(fixture.signed_activity_hex!, "hex");
    assert.deepEqual(encodeProgramMutationBody(fixture.signed_activity_hex), signed);
    assert.throws(() => encodeProgramMutationBody({ activity: fixture.signed_activity_hex }));
    assert.throws(() => encodeProgramMutationBody(fixture.signed_activity_hex!.toUpperCase()));
    const unsigned = Buffer.from(signed.subarray(0, -69)); unsigned[4] = 11;
    const preimage = createHash("sha256").update(Buffer.from("LXP/v1/signature-preimage\0")).update(unsigned).digest();
    const publicKey = createPublicKey({ key: Buffer.concat([Buffer.from("302a300506032b6570032100", "hex"), Buffer.from(fixture.public_key_hex!, "hex")]), format: "der", type: "spki" });
    assert.equal(verify(null, preimage, publicKey, signed.subarray(-64)), true);
    const decode = name === "deploy" ? (bytes: Uint8Array) => encodeNativeProgramDeploy(decodeNativeProgramDeploy(bytes))
      : name === "upgrade" ? (bytes: Uint8Array) => encodeNativeProgramUpgrade(decodeNativeProgramUpgrade(bytes))
      : (bytes: Uint8Array) => encodeNativeProgramWindDown(decodeNativeProgramWindDown(bytes));
    assert.deepEqual(Buffer.from(decode(payload)), payload);
    const request = name === "deploy" ? NativeProgramLifecycleRequest.deploy(decodeNativeProgramDeploy(payload), signed)
      : name === "upgrade" ? NativeProgramLifecycleRequest.upgrade(decodeNativeProgramUpgrade(payload), signed)
      : NativeProgramLifecycleRequest.windDown(decodeNativeProgramWindDown(payload), signed);
    assert.equal((await request.bind(fixture.idempotency_key_hex)).activityId, fixture.activity_id_hex);
    const ordinal = name === "deploy" ? 1 : name === "upgrade" ? 2 : 7;
    const binding = await request.bind();
    const expectedUnknown = { state: "unknown", activity_id: fixture.activity_id_hex, idempotency_key: fixture.idempotency_key_hex, retained_signed_activity: fixture.signed_activity_hex };
    for (const encoded of [Buffer.from('{"result":'), Buffer.from(JSON.stringify({ result: {}, extra: true }))]) {
      let caught: unknown;
      try { decodeEnvelope(200, encoded, "program.deploy"); } catch (error) { caught = error; }
      assert.notEqual(caught, undefined);
      assert.deepEqual(resolveLifecycleFailure(caught, binding), expectedUnknown);
    }
    const refusalError = decodeProgramBoundaryError(400, { code: "invalid_program_payload", retry: "never" });
    assert.throws(() => resolveLifecycleFailure(refusalError, binding), (error) => error === refusalError);
    const localError = new TypeError("invalid signed payload");
    assert.throws(() => resolveLifecycleFailure(localError, binding), (error) => error === localError);
    const hashOffset = signed.length - 69 - payload.length - 5 - 32;
    const badHash = Buffer.from(signed); badHash[hashOffset] = badHash[hashOffset]! ^ 1;
    await assert.rejects(bindSignedProgramLifecycle(badHash, payload, ordinal));
    if (name === "deploy") {
      for (const size of [524_288, 524_289]) {
        const original = decodeNativeProgramDeploy(payload);
        const largeWasm = Buffer.concat([original.wasm, Buffer.alloc(size - payload.length)]);
        const registration = { ...original, wasm: largeWasm, newHash: createHash("sha256").update(largeWasm).digest() };
        let largePayload: Buffer;
        if (size === 524_288) {
          largePayload = Buffer.from(encodeNativeProgramDeploy(registration));
        } else {
          assert.throws(() => encodeNativeProgramDeploy(registration), /program registration payload exceeds network limit/u);
          // Mutate a producer-issued boundary payload to exercise the signed consumer's independent size refusal.
          const boundaryWasm = largeWasm.subarray(0, -1);
          const boundary = encodeNativeProgramDeploy({ ...registration, wasm: boundaryWasm,
            newHash: createHash("sha256").update(boundaryWasm).digest() });
          largePayload = Buffer.concat([boundary, largeWasm.subarray(-1)]);
          largePayload.writeUInt32BE(largeWasm.length, 100);
          largePayload.set(registration.newHash, 68);
        }
        assert.equal(largePayload.length, size);
        const length = Buffer.alloc(4); length.writeUInt32BE(size);
        const mutated = Buffer.concat([signed.subarray(0, hashOffset), createHash("sha256").update(Buffer.from("LXP/v1/payload-hash\0")).update(largePayload).digest(), Buffer.from([11]), length, largePayload, signed.subarray(-69)]);
        if (size === 524_288) assert.equal((await bindSignedProgramLifecycle(mutated, undefined, ordinal)).idempotencyKey, fixture.idempotency_key_hex);
        else await assert.rejects(bindSignedProgramLifecycle(mutated, undefined, ordinal));
      }
    }
    assert.equal((await bindSignedProgramLifecycle(signed, undefined, ordinal, fixture.idempotency_key_hex)).activityId, fixture.activity_id_hex);
    await assert.rejects(bindSignedProgramLifecycle(signed, undefined, ordinal, "00".repeat(32)));
    await assert.rejects(bindSignedProgramLifecycle(signed, undefined, ordinal === 1 ? 2 : 1, fixture.idempotency_key_hex));
    const beforeMutation = Buffer.from(signed); signed[0] = signed[0]! ^ 1;
    assert.equal((await request.bind()).activityId, fixture.activity_id_hex);
    signed.set(beforeMutation);
    const mutable = Buffer.from(signed), pending = bindSignedProgramLifecycle(mutable, undefined, ordinal);
    mutable.fill(0);
    assert.equal((await pending).activityId, fixture.activity_id_hex);
    await assert.rejects(request.bind("00".repeat(32)));
    for (let length = 0; length < payload.length; length++) assert.throws(() => decode(payload.subarray(0, length)));
    assert.throws(() => decode(Buffer.concat([payload, Buffer.from([0])])));
    if (name === "deploy" || name === "upgrade") {
      for (const offset of [35, 68]) { const changed = Buffer.from(payload); changed[offset] = changed[offset]! ^ 1; assert.throws(() => decode(changed)); }
    }
    const changed = Buffer.from(signed); changed[1] = 2;
    const wrong = name === "deploy" ? NativeProgramLifecycleRequest.deploy(decodeNativeProgramDeploy(payload), changed)
      : name === "upgrade" ? NativeProgramLifecycleRequest.upgrade(decodeNativeProgramUpgrade(payload), changed)
      : NativeProgramLifecycleRequest.windDown(decodeNativeProgramWindDown(payload), changed);
    await assert.rejects(wrong.bind());
  }
  const wasm = Buffer.from([0, 97, 115, 109, 1, 0, 0, 0]), programId = Buffer.alloc(32, 1), newHash = createHash("sha256").update(wasm).digest();
  const refusal = decodeProgramBoundaryError(400, { code: "invalid_program_payload", retry: "never" });
  assert.equal(refusal.boundaryCode, "invalid_program_payload");
  assert.equal(refusal.retry, "never");
  assert.equal(decodeProgramBoundaryError(503, { code: "node_unavailable", retry: "after", retry_after_seconds: 2 }).retryAfterMs, 2000);
  assert.throws(() => decodeProgramBoundaryError(200, { code: "invalid_program_payload", retry: "never" }));
  assert.throws(() => decodeProgramBoundaryError(400, { code: "invalid_program_payload", retry: "never", extra: 0 }));
  assert.throws(() => decodeProgramBoundaryError(503, { code: "node_unavailable", retry: "after", retry_after_seconds: true }));
  assert.throws(() => encodeNativeProgramDeploy({ programId, guestAbi: 2, policy: 0, authority: programId, newHash, wasm }));
  assert.throws(() => encodeNativeProgramDeploy({ programId, guestAbi: 2, policy: 0, authority: Buffer.alloc(32), newHash, wasm, interface: Buffer.alloc(953) }));
  assert.throws(() => encodeNativeProgramUpgrade({ programId, guestAbi: 2, oldHash: newHash, newHash, wasm, migrationHook: Buffer.alloc(0), clearInterface: true }));
  assert.throws(() => encodeNativeProgramWindDown({ programId, operation: "route", account: programId, asset: programId, destination: programId, seed: Buffer.alloc(129) }));

  const nativeCodec = await import("../src/native-program-call.js");
  const decodeNativeProgramCallV1 = nativeCodec.decodeNativeProgramCallV1;
  const supportsProgramGuestAbi = (await import("../src/generated/receipt.js")).supportsProgramGuestAbi;
  let firstUnsupportedGuestAbi = 1;
  while (firstUnsupportedGuestAbi <= 65535 && supportsProgramGuestAbi(firstUnsupportedGuestAbi)) firstUnsupportedGuestAbi++;
  assert.equal(firstUnsupportedGuestAbi, 6);
  for (const name of ["native-program-call-v3", "native-program-call-v4"]) {
    const fixture = JSON.parse(readFileSync(new URL(`../../../../../../sdk/platform/conformance/fixtures/${name}.json`, import.meta.url), "utf8"));
    const payload = Buffer.from(fixture.payload_hex, "hex");
    const decoded = decodeNativeProgramCallV1(payload);
    assert.deepEqual(Buffer.from(nativeCodec.encodeNativeProgramCallV1(decoded)), payload);
    const programs = await import("../src/programs.js");
    const wire = await import("../src/program-wire.js");
    const request = new programs.NativeProgramRequestV1(decoded, BigInt(fixture.fee_limit), Buffer.from(fixture.signed_activity_hex, "hex"));
    assert.equal((await wire.decodeSignedProgramCall(request, fixture.idempotency_key_hex)).activityId, fixture.activity_id_hex);
    await assert.rejects(wire.decodeSignedProgramCall(request, "00".repeat(32)));
    await assert.rejects(programs.verifyGatewayProgramDiscovery(null, request.programId,
      new programs.ProgramTrustContext(Buffer.from(fixture.public_key_hex, "hex"), () => 0n, 300_000n, 3)));
    await assert.rejects(programs.verifyGatewayProgramInterface(null, request.programId,
      new programs.ProgramTrustContext(Buffer.from(fixture.public_key_hex, "hex"), () => 0n, 300_000n, 3),
      null as unknown as Parameters<typeof programs.verifyGatewayProgramInterface>[3]));
    assert.deepEqual(Buffer.from(nativeCodec.encodeNativeProgramCall(nativeCodec.decodeNativeProgramCall(payload))), payload);
    for (const guestAbi of [3, 4, 5] as const) {
      assert.equal(supportsProgramGuestAbi(guestAbi), true);
      const call = { ...decoded, guestAbi, programId: new Uint8Array(decoded.programId),
        calldata: new Uint8Array(decoded.calldata), capabilities: new Uint8Array(decoded.capabilities),
        accessDeclaration: new Uint8Array(decoded.accessDeclaration) };
      const canonical = nativeCodec.encodeNativeProgramCallV1(call);
      assert.equal(new DataView(canonical.buffer, canonical.byteOffset).getUint16(32), guestAbi);
      assert.deepEqual(decodeNativeProgramCallV1(canonical), call);
      assert.throws(() => nativeCodec.decodeNativeProgramCall(canonical));
      for (const boundary of [0, 32, 105, canonical.length - 1]) assert.throws(() => decodeNativeProgramCallV1(canonical.subarray(0, boundary)));
      assert.throws(() => decodeNativeProgramCallV1(Buffer.concat([canonical, Buffer.from([0])])));
    }
    for (const guestAbi of [0, firstUnsupportedGuestAbi, 65535]) {
      assert.equal(supportsProgramGuestAbi(guestAbi), false);
      const invalid = Buffer.from(payload); invalid.writeUInt16BE(guestAbi, 32);
      assert.throws(() => decodeNativeProgramCallV1(invalid));
    }
  }
  console.log("native-v1 call encoding uses the canonical ABI policy; legacy ABI1/2 refusals retained");
}

test(lifecycle_signature_payload_and_framing_refusals);

export async function lifecycle_abi_4_5_registration_codecs(): Promise<void> {
  const deployFixture = JSON.parse(readFileSync(new URL("../../../../../../sdk/platform/conformance/fixtures/native-program-deploy-v3.json", import.meta.url), "utf8")) as { payload_hex: string; signed_activity_hex: string };
  const upgradeFixture = JSON.parse(readFileSync(new URL("../../../../../../sdk/platform/conformance/fixtures/native-program-upgrade-v3.json", import.meta.url), "utf8")) as { payload_hex: string; signed_activity_hex: string };
  const deploy = decodeNativeProgramDeploy(Buffer.from(deployFixture.payload_hex, "hex"));
  const upgrade = decodeNativeProgramUpgrade(Buffer.from(upgradeFixture.payload_hex, "hex"));
  for (const guestAbi of [4, 5] as const) {
    const registration = { ...deploy, guestAbi };
    const deploymentBytes = encodeNativeProgramDeploy(registration);
    assert.equal(new DataView(deploymentBytes.buffer, deploymentBytes.byteOffset).getUint16(32), guestAbi);
    assert.deepEqual(decodeNativeProgramDeploy(deploymentBytes), registration);
    const deployRequest = NativeProgramLifecycleRequest.deploy(registration, Buffer.from(deployFixture.signed_activity_hex, "hex"));
    assert.equal(deployRequest.ordinal, 1);
    await assert.rejects(deployRequest.bind(), /lifecycle binding/u);
    const replacement = { ...upgrade, guestAbi };
    const upgradeBytes = encodeNativeProgramUpgrade(replacement);
    assert.equal(new DataView(upgradeBytes.buffer, upgradeBytes.byteOffset).getUint16(32), guestAbi);
    assert.deepEqual(decodeNativeProgramUpgrade(upgradeBytes), replacement);
    for (const currentAbi of [1, 2, 3, 4, 5] as const) {
      if (currentAbi <= guestAbi) {
        admitNativeProgramUpgrade(currentAbi, guestAbi);
        const upgradeRequest = NativeProgramLifecycleRequest.upgrade(replacement, Buffer.from(upgradeFixture.signed_activity_hex, "hex"), currentAbi);
        assert.equal(upgradeRequest.ordinal, 2);
        await assert.rejects(upgradeRequest.bind(), /lifecycle binding/u);
      } else {
        assert.throws(() => admitNativeProgramUpgrade(currentAbi, guestAbi), /unsupported program ABI transition/u);
        assert.throws(() => NativeProgramLifecycleRequest.upgrade(replacement, Buffer.from(upgradeFixture.signed_activity_hex, "hex"), currentAbi), /unsupported program ABI transition/u);
      }
    }
    for (const unsupported of [0, 6, 65535]) {
      assert.throws(() => encodeNativeProgramDeploy({ ...registration, guestAbi: unsupported as NativeProgramGuestAbi }), /invalid program code/u);
      assert.throws(() => encodeNativeProgramUpgrade({ ...replacement, guestAbi: unsupported as NativeProgramGuestAbi }), /invalid program code/u);
      const badDeploy = Buffer.from(deploymentBytes); badDeploy.writeUInt16BE(unsupported, 32);
      const badUpgrade = Buffer.from(upgradeBytes); badUpgrade.writeUInt16BE(unsupported, 32);
      assert.throws(() => decodeNativeProgramDeploy(badDeploy), /invalid program code/u);
      assert.throws(() => decodeNativeProgramUpgrade(badUpgrade), /invalid program code/u);
      assert.throws(() => admitNativeProgramUpgrade(guestAbi, unsupported), /unsupported program ABI transition/u);
      assert.throws(() => admitNativeProgramUpgrade(unsupported, guestAbi), /unsupported program ABI transition/u);
    }
  }
}

test(lifecycle_abi_4_5_registration_codecs);
