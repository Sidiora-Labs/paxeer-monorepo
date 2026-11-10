import assert from "node:assert/strict";
import { execFile } from "node:child_process";
import { createHash, createPublicKey, randomBytes, verify } from "node:crypto";
import { once } from "node:events";
import {
  chmod,
  mkdtemp,
  readFile,
  readdir,
  rm,
  stat,
  symlink,
  writeFile,
} from "node:fs/promises";
import { createServer } from "node:http";
import { tmpdir, totalmem } from "node:os";
import { join } from "node:path";
import test from "node:test";
import { promisify } from "node:util";
import {
  encodeManifestV2,
  helloSigningBytes,
  manifestDigest,
  measureManifest,
  PeerIdentity,
  signHello,
} from "../src/identity.mjs";
import { decodeSignalFrame, encodeSignalFrame } from "../src/signal.mjs";
import { SignalClient } from "../src/transport.mjs";
import { RoomHost, vector } from "./host-signal.mjs";

// Exact manifest_v1() fixture from task 22.38.2:
// programs/crates/layerx-programs-ai-market/tests/manifest_v2_capabilities.rs.
const V1_FIXTURE = Buffer.from(
  [
    "00010a0a0a0a0a0a0a0a0a0a0a0a0a0a0a0a0a0a0a0a0a0a0a0a0a0a0a0a0a0a0a0a0b0b0b0b0b0b0b0b0b0b0b0b0b0b0b0b",
    "0b0b0b0b0b0b0b0b0b0b0b0b0b0b0b0b0c0c0c0c0c0c0c0c0c0c0c0c0c0c0c0c0c0c0c0c0c0c0c0c0c0c0c0c0c0c0c0c0000",
    "00000000000200000000000000030000000000000005000000000000012c00000000000001ac0d0d0d0d0d0d0d0d0d0d0d0d",
    "0d0d0d0d0d0d0d0d0d0d0d0d0d0d0d0d0d0d0d0d000101011414141414141414141414141414141414141414141414141414",
    "1414141414141515151515151515151515151515151515151515151515151515151515151515161616161616161616161616",
    "1616161616161616161616161616161616161616171717171717171717171717171717171717171717171717171717171717",
    "1717181818181818181818181818181818181818181818181818181818181818181800000400000008000000100000002000",
    "010000ea600004010001010000001f68747470733a2f2f776f726b65722e6578616d706c652f70617861692f76311e1e1e1e",
    "1e1e1e1e1e1e1e1e1e1e1e1e1e1e1e1e1e1e1e1e1e1e1e1e1e1e1e1e01000100011f1f1f1f1f1f1f1f1f1f1f1f1f1f1f1f1f",
    "1f1f1f1f1f1f1f1f1f1f1f1f1f1f1f2020202020202020202020202020202020202020202020202020202020202020000000",
    "00",
  ].join(""),
  "hex",
);

const CAPABILITIES = Object.freeze({
  acceleratorMemoryMiB: 24576,
  systemMemoryMiB: 65536,
  bandwidthClass: 4,
  regionCode: "US",
  attestationFamily: 255,
  runtimeKind: 2,
});

async function directory(t) {
  const path = await mkdtemp(join(tmpdir(), "inference-identity-"));
  t.after(() => rm(path, { recursive: true, force: true }));
  return path;
}

function publicKey(peerKey) {
  return createPublicKey({
    key: Buffer.concat([
      Buffer.from("302a300506032b6570032100", "hex"),
      Buffer.from(peerKey, "hex"),
    ]),
    type: "spki",
    format: "der",
  });
}

function metadataHash(manifest) {
  return createHash("sha256")
    .update("PAXAI/participant-metadata/v2\0")
    .update(manifest)
    .digest();
}

function independentlyFramedHello(roomId, nonce) {
  return Buffer.concat([
    Buffer.from("PAXAI/inference-signal/hello/v1\0"),
    Buffer.from(roomId, "hex"),
    Buffer.from(nonce, "hex"),
  ]);
}

async function listen(t, handler) {
  const server = createServer(handler);
  server.listen(0, "127.0.0.1");
  await once(server, "listening");
  t.after(
    () =>
      new Promise((resolve, reject) => {
        server.closeAllConnections();
        server.close((error) => (error ? reject(error) : resolve()));
      }),
  );
  return `http://127.0.0.1:${server.address().port}`;
}

async function native_key_persists_across_instances_and_process_restart(t) {
  const path = join(await directory(t), "identity.pkcs8");
  const first = await PeerIdentity.open({ path });
  const saved = await readFile(path);
  const second = await PeerIdentity.open({ path });
  assert.equal(first.peerKey, second.peerKey);
  assert.match(first.peerKey, /^[a-f0-9]{64}$/);
  assert.equal((await stat(path)).mode & 0o777, 0o600);
  const input = randomBytes(32);
  assert.equal(await first.sign(input), await second.sign(input));
  assert.ok(
    verify(
      null,
      input,
      publicKey(first.peerKey),
      Buffer.from(await second.sign(input), "hex"),
    ),
  );
  const moduleUrl = new URL("../src/identity.mjs", import.meta.url).href;
  const script = `
    const { PeerIdentity } = await import(process.argv[2]);
    const identity = await PeerIdentity.open({ path: process.argv[1] });
    console.log(JSON.stringify({ peerKey: identity.peerKey,
      signature: await identity.sign(new TextEncoder().encode("after restart")) }));
  `;
  const { stdout } = await promisify(execFile)(process.execPath, [
    "--input-type=module",
    "-e",
    script,
    path,
    moduleUrl,
  ]);
  const restarted = JSON.parse(stdout);
  assert.equal(restarted.peerKey, first.peerKey);
  assert.ok(
    verify(
      null,
      Buffer.from("after restart"),
      publicKey(first.peerKey),
      Buffer.from(restarted.signature, "hex"),
    ),
  );
  assert.deepEqual(await readFile(path), saved);
}

async function simultaneous_native_instances_publish_one_complete_key(t) {
  const dir = await directory(t);
  const identities = await Promise.all(
    Array.from({ length: 8 }, () =>
      PeerIdentity.open({ path: join(dir, "identity.pkcs8") }),
    ),
  );
  assert.equal(new Set(identities.map((identity) => identity.peerKey)).size, 1);
  assert.deepEqual(await readdir(dir), ["identity.pkcs8"]);
}

async function invalid_or_exposed_native_keys_are_refused_without_replacement(
  t,
) {
  const dir = await directory(t);
  const path = join(dir, "identity.pkcs8");
  await writeFile(path, "damaged key", { mode: 0o600 });
  await assert.rejects(PeerIdentity.open({ path }));
  assert.equal(await readFile(path, "utf8"), "damaged key");
  await rm(path);
  await PeerIdentity.open({ path });
  const original = await readFile(path);
  await chmod(path, 0o644);
  await assert.rejects(PeerIdentity.open({ path }), /owner-only/);
  assert.deepEqual(await readFile(path), original);
  await chmod(path, 0o600);
  const alias = join(dir, "alias.pkcs8");
  await symlink(path, alias);
  await assert.rejects(PeerIdentity.open({ path: alias }), { code: "ELOOP" });
}

async function hello_signature_verifies_after_wire_encoding_and_binds_room_nonce_and_manifest(
  t,
) {
  const identity = await PeerIdentity.open({
    path: join(await directory(t), "key"),
  });
  const manifest = encodeManifestV2(V1_FIXTURE, {
    ...CAPABILITIES,
    runtimeKind: 1,
  });
  const roomId = "04".repeat(32);
  const nonce = randomBytes(32).toString("hex");
  const host = new RoomHost();
  let hello;
  const url = await host.listen(
    t,
    (connection, frame) => {
      hello = frame;
      connection.send(vector("offer"));
    },
    {
      challenge: {
        type: "challenge",
        nonce: Array.from(Buffer.from(nonce, "hex")),
      },
    },
  );
  const client = new SignalClient(url, { roomId, identity, manifest });
  t.after(() => client.close());
  await client.connect();
  assert.deepEqual(
    decodeSignalFrame(encodeSignalFrame(hello), "client"),
    hello,
  );
  assert.deepEqual(Buffer.from(hello.manifest), Buffer.from(manifest));
  const signed = await identity.signManifest(manifest);
  const digest = metadataHash(Buffer.from(hello.manifest));
  assert.equal(signed.metadataDigest, digest.toString("hex"));
  assert.equal(signed.peerKey, Buffer.from(hello.peer_key).toString("hex"));
  assert.equal(signed.version, 2);
  const key = publicKey(identity.peerKey);
  assert.ok(verify(null, digest, key, Buffer.from(signed.signature, "hex")));
  const signature = Buffer.from(hello.signature);
  const preimage = independentlyFramedHello(roomId, nonce);
  assert.deepEqual(Buffer.from(helloSigningBytes(roomId, nonce)), preimage);
  assert.ok(verify(null, preimage, key, signature));
  assert.equal(
    verify(
      null,
      independentlyFramedHello("05".repeat(32), nonce),
      key,
      signature,
    ),
    false,
  );
  assert.equal(
    verify(
      null,
      independentlyFramedHello(roomId, "06".repeat(32)),
      key,
      signature,
    ),
    false,
  );
  const changed = Buffer.from(manifest);
  changed[changed.length - 5] = 3;
  assert.equal(
    verify(
      null,
      metadataHash(changed),
      key,
      Buffer.from(signed.signature, "hex"),
    ),
    false,
  );
  await assert.rejects(
    signHello(identity, { roomId, manifest }),
    /32 byte array/,
  );
  await assert.rejects(
    signHello(identity, { roomId: "", nonce, manifest }),
    /32 byte array/,
  );
  await assert.rejects(
    signHello(identity, { roomId, nonce: "", manifest }),
    /32 byte array/,
  );
  assert.throws(() => helloSigningBytes("\ud800", nonce), /32 byte array/);
}

async function manifest_matches_chain_v2_fixture_and_metadata_domain() {
  assert.equal(V1_FIXTURE.length, 501);
  const manifest = encodeManifestV2(V1_FIXTURE, CAPABILITIES);
  assert.equal(manifest.length, 514);
  assert.deepEqual(Buffer.from(manifest.subarray(0, 2)), Buffer.from([0, 2]));
  assert.deepEqual(
    Buffer.from(manifest.subarray(2, 501)),
    V1_FIXTURE.subarray(2),
  );
  // v2_decodes_all_inference_capability_fields asserts these same 13 bytes.
  assert.deepEqual(
    Array.from(manifest.subarray(501)),
    [0, 0, 0x60, 0, 0, 1, 0, 0, 4, 85, 83, 255, 2],
  );
  assert.deepEqual(
    Buffer.from(await manifestDigest(manifest)),
    metadataHash(manifest),
  );
  assert.notDeepEqual(
    metadataHash(manifest),
    createHash("sha256")
      .update("PAXAI/worker-metadata/v1\0")
      .update(manifest)
      .digest(),
  );
  for (const runtimeKind of [0, 1, 2]) {
    for (const bandwidthClass of [0, 1, 2, 3, 4]) {
      const boundary = encodeManifestV2(V1_FIXTURE, {
        ...CAPABILITIES,
        acceleratorMemoryMiB: 0xffffffff,
        systemMemoryMiB: 0,
        regionCode: "gb",
        runtimeKind,
        bandwidthClass,
      });
      assert.equal(new DataView(boundary.buffer).getUint32(501), 0xffffffff);
      assert.equal(boundary.at(-1), runtimeKind);
      assert.equal(boundary.at(-5), bandwidthClass);
      assert.equal((await manifestDigest(boundary)).length, 32);
    }
  }
}

async function manifest_refuses_noncanonical_capabilities_and_invalid_v1_templates() {
  for (const invalid of [
    { regionCode: "USA" },
    { regionCode: "U1" },
    { regionCode: "éU" },
    { runtimeKind: 3 },
    { bandwidthClass: 5 },
    { bandwidthClass: -1 },
    { acceleratorMemoryMiB: 0x100000000 },
    { systemMemoryMiB: -1 },
    { systemMemoryMiB: 1.5 },
    { attestationFamily: 256 },
  ])
    assert.throws(
      () => encodeManifestV2(V1_FIXTURE, { ...CAPABILITIES, ...invalid }),
      RangeError,
    );
  const version = Buffer.from(V1_FIXTURE);
  version.writeUInt16BE(2);
  const window = Buffer.from(V1_FIXTURE);
  window.writeBigUInt64BE(300n, 130);
  const endpoint = Buffer.from(V1_FIXTURE);
  endpoint[382] = 0;
  for (const invalid of [
    version,
    window,
    endpoint,
    V1_FIXTURE.subarray(0, 500),
    Buffer.concat([V1_FIXTURE, Buffer.from([0])]),
    new Uint8Array(8193),
  ])
    assert.throws(() => encodeManifestV2(invalid, CAPABILITIES), RangeError);
  const valid = encodeManifestV2(V1_FIXTURE, CAPABILITIES);
  for (const offset of [509, 513]) {
    const invalid = valid.slice();
    invalid[offset] = 255;
    await assert.rejects(manifestDigest(invalid), RangeError);
  }
  await assert.rejects(manifestDigest(valid.subarray(0, 513)), /Truncated/);
  await assert.rejects(
    manifestDigest(Buffer.concat([valid, Buffer.from([0])])),
    /Trailing/,
  );
}

async function measured_native_manifest_uses_platform_memory_and_a_bounded_real_probe(
  t,
) {
  const probeBody = randomBytes(65536);
  let requests = 0;
  const url = await listen(t, (request, response) => {
    requests++;
    assert.equal(request.method, "GET");
    assert.equal(request.url, "/probe");
    response.writeHead(200, { "content-length": probeBody.length });
    response.end(probeBody);
  });
  const manifest = await measureManifest({
    manifestV1: V1_FIXTURE,
    regionCode: "gb",
    probeUrl: `${url}/probe`,
    mode: "native",
  });
  assert.equal(requests, 1);
  const view = new DataView(manifest.buffer);
  assert.equal(view.getUint16(0), 2);
  assert.deepEqual(
    Buffer.from(manifest.subarray(2, 501)),
    V1_FIXTURE.subarray(2),
  );
  assert.equal(view.getUint32(505), Math.floor(totalmem() / (1024 * 1024)));
  if (!globalThis.navigator?.gpu) assert.equal(view.getUint32(501), 0);
  assert.ok(manifest[509] >= 1 && manifest[509] <= 4);
  assert.equal(Buffer.from(manifest.subarray(510, 512)).toString(), "gb");
  assert.equal(manifest[512], 255);
  assert.equal(manifest[513], 1);
  assert.equal((await manifestDigest(manifest)).length, 32);
}

async function probe_refusals_bad_lengths_redirects_and_deadlines_surface_errors(
  t,
) {
  const url = await listen(t, (request, response) => {
    if (request.url === "/slow") return;
    if (request.url === "/refused") {
      response.writeHead(503);
      response.end();
      return;
    }
    if (request.url === "/redirect") {
      response.writeHead(302, { location: "/probe" });
      response.end();
      return;
    }
    if (request.url === "/compressed") {
      response.writeHead(200, {
        "content-encoding": "gzip",
        "content-length": 65536,
      });
      response.end();
      return;
    }
    response.writeHead(200, {
      "content-length": request.url === "/small" ? 1 : 1048577,
    });
    response.end();
  });
  for (const path of [
    "/refused",
    "/small",
    "/large",
    "/compressed",
    "/redirect",
    "/slow",
  ]) {
    await assert.rejects(
      measureManifest({
        manifestV1: V1_FIXTURE,
        regionCode: "US",
        probeUrl: `${url}${path}`,
        timeoutMs: path === "/slow" ? 50 : 2000,
      }),
    );
  }
  await assert.rejects(
    measureManifest({
      manifestV1: V1_FIXTURE,
      regionCode: "US",
      probeUrl: "http://example.com/probe",
    }),
    /HTTPS/,
  );
  await assert.rejects(
    measureManifest({
      manifestV1: V1_FIXTURE,
      regionCode: "US",
      probeUrl: `${url}/slow`,
      timeoutMs: 2001,
    }),
    /timeout/,
  );
  await assert.rejects(PeerIdentity.open({ mode: "daemon" }), /Runtime/);
  if (!globalThis.indexedDB)
    await assert.rejects(PeerIdentity.open({ mode: "browser" }), /IndexedDB/);
  if (!globalThis.navigator?.deviceMemory)
    await assert.rejects(
      measureManifest({
        manifestV1: V1_FIXTURE,
        regionCode: "US",
        probeUrl: `${url}/probe`,
        mode: "browser",
      }),
      /deviceMemory/,
    );
}

async function enrol_request_delivers_the_real_peer_signed_manifest_over_http(
  t,
) {
  const identity = await PeerIdentity.open({
    path: join(await directory(t), "key"),
  });
  let accepted;
  let failure;
  const url = await listen(t, async (request, response) => {
    try {
      assert.equal(request.method, "POST");
      assert.equal(request.url, "/enrol_peer");
      assert.equal(request.headers["content-type"], "application/json");
      assert.equal(request.headers.authorization, "Bearer local-test-session");
      const chunks = [];
      for await (const chunk of request) chunks.push(chunk);
      const document = JSON.parse(Buffer.concat(chunks));
      const manifest = document.manifest;
      const encoded = Buffer.from(manifest.bytes, "hex");
      const digest = metadataHash(encoded);
      assert.equal(manifest.version, 2);
      assert.equal(encoded.readUInt16BE(0), 2);
      assert.equal(manifest.metadataDigest, digest.toString("hex"));
      assert.ok(
        verify(
          null,
          digest,
          publicKey(manifest.peerKey),
          Buffer.from(manifest.signature, "hex"),
        ),
      );
      accepted = document;
      response.writeHead(200, { "content-type": "application/json" });
      response.end(
        JSON.stringify({
          peerKey: manifest.peerKey,
          metadataDigest: manifest.metadataDigest,
        }),
      );
    } catch (error) {
      failure = error;
      response.writeHead(400);
      response.end(String(error));
    }
  });
  const manifest = encodeManifestV2(V1_FIXTURE, {
    ...CAPABILITIES,
    runtimeKind: 1,
  });
  const result = await identity.enrolPeer(`${url}/enrol_peer`, {
    manifest,
    headers: { authorization: "Bearer local-test-session" },
  });
  assert.equal(failure, undefined);
  assert.equal(accepted.manifest.peerKey, identity.peerKey);
  assert.deepEqual(
    Buffer.from(accepted.manifest.bytes, "hex"),
    Buffer.from(manifest),
  );
  assert.deepEqual(result, {
    peerKey: identity.peerKey,
    metadataDigest: metadataHash(manifest).toString("hex"),
  });
  assert.equal(JSON.stringify(accepted).includes("privateKey"), false);
}

async function enrol_refusals_and_invalid_routes_do_not_report_success(t) {
  const identity = await PeerIdentity.open({
    path: join(await directory(t), "key"),
  });
  const url = await listen(t, (_request, response) => {
    response.writeHead(409, { "content-type": "application/json" });
    response.end(
      JSON.stringify({ error: "WRONG_ROSTER", operation: "LockBond" }),
    );
  });
  const manifest = encodeManifestV2(V1_FIXTURE, CAPABILITIES);
  await assert.rejects(
    identity.enrolPeer(`${url}/enrol_peer`, { manifest }),
    /enrol_peer refused \(409\).*WRONG_ROSTER.*LockBond/,
  );
  await assert.rejects(
    identity.enrolPeer(`${url}/other`, { manifest }),
    /enrol_peer route/,
  );
  await assert.rejects(
    identity.enrolPeer("http://example.com/enrol_peer", { manifest }),
    /HTTPS/,
  );
  await assert.rejects(
    identity.enrolPeer(`${url}/enrol_peer`, {
      manifest,
      quote: { oversized: "x".repeat(262144) },
    }),
    /exceeds/,
  );
}

test(native_key_persists_across_instances_and_process_restart);
test(simultaneous_native_instances_publish_one_complete_key);
test(invalid_or_exposed_native_keys_are_refused_without_replacement);
test(
  hello_signature_verifies_after_wire_encoding_and_binds_room_nonce_and_manifest,
);
test(manifest_matches_chain_v2_fixture_and_metadata_domain);
test(manifest_refuses_noncanonical_capabilities_and_invalid_v1_templates);
test(measured_native_manifest_uses_platform_memory_and_a_bounded_real_probe);
test(probe_refusals_bad_lengths_redirects_and_deadlines_surface_errors);
test(enrol_request_delivers_the_real_peer_signed_manifest_over_http);
test(enrol_refusals_and_invalid_routes_do_not_report_success);
