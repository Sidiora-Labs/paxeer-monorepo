import assert from "node:assert/strict";
import { spawn, execFile } from "node:child_process";
import { createECDH, createHash, createPublicKey, verify } from "node:crypto";
import { once } from "node:events";
import { mkdtemp, rm, writeFile } from "node:fs/promises";
import { createServer } from "node:http";
import { tmpdir } from "node:os";
import { join } from "node:path";
import { fileURLToPath } from "node:url";
import { promisify } from "node:util";
import test from "node:test";
import {
  checkAttestation, createCPURuntime, createNativeRuntime, joinCommand,
  keccak256, parseCLI, serveCommand, statusCommand,
} from "../src/cli.mjs";
import { encodeManifestV2, helloSigningBytes, PeerIdentity } from "../src/identity.mjs";
import { decodeFrame, encodeFrame, MAX_FRAME_BYTES } from "../src/wire.mjs";

const executable = fileURLToPath(new URL("../bin/inference-peer.mjs", import.meta.url));
const room = "45".repeat(32);
const modelDigest = "23".repeat(32);
const runtimeDigest = "67".repeat(32);
// Exact bytes emitted by manifest_v1() in the Rust manifest_v2_capabilities test.
const manifestV1 = Buffer.from([
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
].join(""), "hex");

async function inputs(t) {
  const dir = await mkdtemp(join(tmpdir(), "inference-cli-"));
  t.after(() => rm(dir, { recursive: true, force: true }));
  const manifest = join(dir, "manifest.bin"), identity = join(dir, "identity.pkcs8");
  const bytes = encodeManifestV2(manifestV1, { acceleratorMemoryMiB: 0, systemMemoryMiB: 1024,
    bandwidthClass: 1, regionCode: "US", attestationFamily: 255, runtimeKind: 1 });
  await writeFile(manifest, bytes);
  const peer = await PeerIdentity.open({ mode: "native", path: identity });
  return { dir, manifest, identity, bytes, peer };
}

async function listen(t, server, sockets = new Set()) {
  server.listen(0, "127.0.0.1");
  await once(server, "listening");
  t.after(async () => {
    for (const socket of sockets) socket.destroy();
    server.closeAllConnections();
    await new Promise((resolve) => server.close(resolve));
  });
  return `http://127.0.0.1:${server.address().port}`;
}

async function jsonBody(request) {
  const chunks = [];
  for await (const chunk of request) chunks.push(chunk);
  return JSON.parse(Buffer.concat(chunks));
}

// This host uses real HTTP and RFC 6455 sockets; the CLI's transport is unchanged.
async function roomHost(t, { binding = true } = {}) {
  const events = [], sockets = new Set();
  let enrolled, receivedHello, upgradePath;
  const server = createServer(async (request, response) => {
    events.push(request.url);
    const body = await jsonBody(request);
    response.setHeader("content-type", "application/json");
    if (request.url === "/bind_peer") {
      assert.equal(body.family, 0);
      assert.equal(body.modelDigest, modelDigest);
      assert.equal(body.runtimeDigest, runtimeDigest);
      assert.equal(Buffer.from(body.envelope, "hex").length, 161);
      response.end(JSON.stringify({ bound: binding }));
    } else if (request.url === "/enrol_peer") {
      enrolled = body;
      response.end(JSON.stringify({ enrolled: true }));
    } else response.writeHead(404).end("{}");
  });
  server.on("upgrade", (request, socket, head) => {
    upgradePath = request.url;
    events.push("upgrade");
    sockets.add(socket);
    socket.on("close", () => sockets.delete(socket));
    const accept = createHash("sha1").update(request.headers["sec-websocket-key"] +
      "258EAFA5-E914-47DA-95CA-C5AB0DC85B11").digest("base64");
    socket.write("HTTP/1.1 101 Switching Protocols\r\nUpgrade: websocket\r\nConnection: Upgrade\r\nSec-WebSocket-Accept: " + accept + "\r\n\r\n");
    let buffer = Buffer.from(head);
    socket.on("error", () => {});
    socket.on("data", (chunk) => {
      buffer = Buffer.concat([buffer, chunk]);
      while (buffer.length >= 2) {
        const opcode = buffer[0] & 15;
        let size = buffer[1] & 127, offset = 2;
        if (size === 126) { if (buffer.length < 4) return; size = buffer.readUInt16BE(2); offset = 4; }
        if (size === 127) { if (buffer.length < 10) return; size = Number(buffer.readBigUInt64BE(2)); offset = 10; }
        assert.ok(size <= MAX_FRAME_BYTES);
        assert.equal(buffer[1] & 128, 128);
        if (buffer.length < offset + 4 + size) return;
        const mask = buffer.subarray(offset, offset + 4);
        offset += 4;
        const body = Buffer.from(buffer.subarray(offset, offset + size));
        for (let i = 0; i < body.length; i++) body[i] ^= mask[i % 4];
        buffer = buffer.subarray(offset + size);
        if (opcode === 8) { socket.end(Buffer.from([0x88, 0])); return; }
        assert.equal(opcode, 2);
        const hello = decodeFrame(body);
        assert.equal(hello.kind, "hello");
        assert.equal(hello.sequence, 0);
        receivedHello = hello;
        events.push("hello");
        const bytes = Buffer.from(encodeFrame({ kind: "offer", roomId: room, sequence: 0,
          payload: { model_digest: modelDigest, runtime_digest: runtimeDigest } }));
        const header = Buffer.alloc(4);
        header[0] = 0x82; header[1] = 126; header.writeUInt16BE(bytes.length, 2);
        socket.write(Buffer.concat([header, bytes]));
      }
    });
  });
  const host = await listen(t, server, sockets);
  return { host, events, get enrolled() { return enrolled; }, get hello() { return receivedHello; },
    get upgradePath() { return upgradePath; } };
}

function verifyManifestAndHello(host, peer, bytes) {
  const hello = host.hello;
  assert.equal(hello.roomId, room);
  assert.equal(hello.payload.peerKey, peer.peerKey);
  assert.equal(hello.payload.nonce, "challenge-123");
  assert.deepEqual(host.enrolled.manifest, hello.payload.manifest);
  assert.equal(hello.payload.manifest.bytes, Buffer.from(bytes).toString("hex"));
  const key = createPublicKey({ key: Buffer.concat([Buffer.from("302a300506032b6570032100", "hex"),
    Buffer.from(peer.peerKey, "hex")]), format: "der", type: "spki" });
  const metadata = createHash("sha256").update("PAXAI/participant-metadata/v2\0").update(bytes).digest();
  assert.equal(hello.payload.manifest.metadataDigest, metadata.toString("hex"));
  assert.ok(verify(null, metadata, key, Buffer.from(hello.payload.manifest.signature, "hex")));
  assert.ok(verify(null, helloSigningBytes(room, hello.payload.nonce, metadata.toString("hex")), key,
    Buffer.from(hello.payload.signature, "hex")));
}

async function joinParsesArgumentsAndSendsSignedHello(t) {
  const files = await inputs(t);
  const host = await roomHost(t);
  const args = ["join", host.host, room, "--manifest", files.manifest, "--identity", files.identity,
    "--nonce", "challenge-123", "--status-port", "0"];
  assert.equal(parseCLI(args).statusPort, 0);
  const child = spawn(process.execPath, [executable, ...args], { stdio: ["ignore", "pipe", "pipe"] });
  const exited = once(child, "exit");
  t.after(async () => { if (child.exitCode === null && child.signalCode === null) child.kill("SIGTERM"); await exited; });
  let stderr = "";
  child.stderr.on("data", (chunk) => stderr += chunk);
  const started = new Promise((resolve, reject) => {
    let text = "";
    child.stdout.on("data", (chunk) => {
      text += chunk;
      if (text.includes("\n")) resolve(JSON.parse(text.split("\n")[0]));
    });
    child.on("error", reject);
    child.on("exit", (code) => reject(new Error(`CLI exited ${code}: ${stderr}`)));
  });
  const status = await started;
  assert.equal(status.state, "offered");
  assert.equal(status.room, room);
  assert.deepEqual(host.events, ["/enrol_peer", "upgrade", "hello"]);
  assert.equal(host.upgradePath, `/rooms/${room}/signal`);
  verifyManifestAndHello(host, files.peer, files.bytes);
  const statusPort = new URL(status.statusURL).port;
  const { stdout } = await promisify(execFile)(process.execPath, [executable, "status", "--port", statusPort]);
  const queried = JSON.parse(stdout);
  assert.equal(queried.peerKey, files.peer.peerKey);
  assert.equal(queried.activations, 0);
  assert.equal(queried.layerRange, null);
  assert.equal(queried.error, null);
}

// Produce a signed PXVE development envelope, using secp256k1 scalar multiplication
// from OpenSSL. Chain trust/quorum remains the host's responsibility.
function signedEnvelope(report) {
  const order = 0xfffffffffffffffffffffffffffffffebaaedce6af48a03bbfd25e8cd0364141n;
  const header = Buffer.alloc(96);
  header.write("PXVE"); header[4] = 1; header[5] = 0;
  header.fill(0x11, 7, 39); report.copy(header, 39);
  header.writeBigUInt64BE(1000n, 71); header.writeBigUInt64BE(125n, 79);
  header.writeBigUInt64BE(1n, 87); header[95] = 1;
  const point = createECDH("secp256k1");
  point.setPrivateKey(Buffer.from("00".repeat(31) + "02", "hex"));
  const publicPoint = point.getPublicKey(undefined, "uncompressed");
  const x = BigInt("0x" + publicPoint.subarray(1, 33).toString("hex"));
  const r = x % order;
  const hash = BigInt("0x" + keccak256(header.subarray(0, 95)).toString("hex"));
  let s = ((hash + r) * ((order + 1n) / 2n)) % order;
  let recovery = (publicPoint[64] & 1) | (x >= order ? 2 : 0);
  if (s > order / 2n) { s = order - s; recovery ^= 1; }
  const signature = Buffer.from(r.toString(16).padStart(64, "0") + s.toString(16).padStart(64, "0") +
    recovery.toString(16).padStart(2, "0"), "hex");
  return Buffer.concat([header, signature]);
}

async function attestationBindingPrecedesEnrollmentAndHello(t) {
  const files = await inputs(t), host = await roomHost(t);
  const report = keccak256(Buffer.from(files.peer.peerKey + modelDigest + runtimeDigest, "hex"));
  const envelope = signedEnvelope(report), attestation = join(files.dir, "quote.pxve");
  await writeFile(attestation, envelope);
  assert.equal(checkAttestation(envelope, { peerKey: files.peer.peerKey, modelDigest, runtimeDigest }).family, 0);
  const session = await joinCommand({ host: host.host, room, manifest: files.manifest, identity: files.identity,
    attestation, "model-digest": modelDigest, "runtime-digest": runtimeDigest,
    nonce: "challenge-123", statusPort: 0 });
  t.after(() => session.close());
  assert.deepEqual(host.events, ["/bind_peer", "/enrol_peer", "upgrade", "hello"]);
  verifyManifestAndHello(host, files.peer, files.bytes);
  assert.equal((await statusCommand({ port: new URL(session.url).port })).state, "offered");
}

async function mismatchedAttestationIsRefusedBeforeAnySend(t) {
  const files = await inputs(t), host = await roomHost(t);
  for (const changed of ["peer", "model", "runtime"]) {
    const report = keccak256(Buffer.from(
      (changed === "peer" ? "89".repeat(32) : files.peer.peerKey) +
      (changed === "model" ? "89".repeat(32) : modelDigest) +
      (changed === "runtime" ? "89".repeat(32) : runtimeDigest), "hex"));
    const attestation = join(files.dir, `${changed}.pxve`);
    await writeFile(attestation, signedEnvelope(report));
    await assert.rejects(joinCommand({ host: host.host, room, identity: files.identity, manifest: files.manifest,
      attestation, "model-digest": modelDigest, "runtime-digest": runtimeDigest, statusPort: 0 }),
    /report data does not match peer identity, model and runtime/);
    assert.deepEqual(host.events, []);
  }
}

async function unconfirmedBindingStopsBeforeEnrollment(t) {
  const files = await inputs(t), host = await roomHost(t, { binding: false });
  const attestation = join(files.dir, "quote.pxve");
  await writeFile(attestation, signedEnvelope(keccak256(Buffer.from(files.peer.peerKey + modelDigest + runtimeDigest, "hex"))));
  await assert.rejects(joinCommand({ host: host.host, room, identity: files.identity, manifest: files.manifest,
    attestation, "model-digest": modelDigest, "runtime-digest": runtimeDigest, statusPort: 0 }), /did not confirm bindPeer/);
  assert.deepEqual(host.events, ["/bind_peer"]);
}

async function serveForwardsAskBearerAndStreamsBeforeHostFinishes(t) {
  let release, finished = false, received;
  const gate = new Promise((resolve) => release = resolve);
  t.after(() => release());
  const host = await listen(t, createServer(async (request, response) => {
    received = { path: request.url, authorization: request.headers.authorization,
      contentType: request.headers["content-type"], body: await jsonBody(request) };
    response.writeHead(200, { "content-type": "text/event-stream", "cache-control": "no-cache" });
    response.write('data: {"choices":[{"delta":{"content":"hello"}}]}\n\n');
    await gate;
    finished = true;
    response.end('data: {"choices":[{"delta":{"content":" world"}}]}\n\ndata: [DONE]\n\n');
  }));
  const bridge = await serveCommand({ host, "ask-bearer": "ask-stream-secret", port: 0 });
  t.after(() => bridge.close());
  const body = { model: "room-model", messages: [{ role: "user", content: "Hi" }], stream: true };
  const response = await fetch(bridge.url + "/v1/chat/completions", { method: "POST",
    headers: { "content-type": "application/json", authorization: "Bearer caller-other-token" }, body: JSON.stringify(body) });
  assert.equal(response.status, 200);
  assert.equal(response.headers.get("content-type"), "text/event-stream");
  assert.equal(response.headers.get("cache-control"), "no-cache");
  const reader = response.body.getReader();
  const first = await reader.read();
  assert.match(Buffer.from(first.value).toString(), /hello/);
  assert.equal(finished, false, "first bytes arrive while the host is still running");
  assert.deepEqual(received, { path: "/v1/chat/completions", authorization: "Bearer ask-stream-secret",
    contentType: "application/json", body });
  release();
  let rest = "";
  for (;;) { const { done, value } = await reader.read(); if (done) break; rest += Buffer.from(value); }
  assert.equal(rest, 'data: {"choices":[{"delta":{"content":" world"}}]}\n\ndata: [DONE]\n\n');
}

async function servePreservesHostRefusalsAndRejectsUnknownRoutes(t) {
  let requests = 0;
  const host = await listen(t, createServer(async (request, response) => {
    requests++;
    await jsonBody(request);
    response.writeHead(402, { "content-type": "application/json", "retry-after": "5" });
    response.end('{"error":{"message":"ask capped"}}');
  }));
  const bridge = await serveCommand({ host, "ask-bearer": "ask-secret", port: 0 });
  t.after(() => bridge.close());
  const response = await fetch(bridge.url + "/v1/completions", { method: "POST", body: JSON.stringify({ prompt: "Hi" }) });
  assert.equal(response.status, 402);
  assert.equal(response.headers.get("retry-after"), "5");
  assert.deepEqual(await response.json(), { error: { message: "ask capped" } });
  assert.equal((await fetch(bridge.url + "/private", { method: "POST", body: "secret" })).status, 404);
  assert.equal(requests, 1);
  await assert.rejects(serveCommand({ host, "ask-bearer": "bad\nvalue", port: 0 }), /ask bearer/);
}

async function serveCancelsUpstreamWhenLocalClientDisconnects(t) {
  let closed;
  const upstreamClosed = new Promise((resolve) => closed = resolve);
  const host = await listen(t, createServer(async (request, response) => {
    await jsonBody(request);
    response.on("close", closed);
    response.writeHead(200, { "content-type": "text/event-stream" });
    response.write('data: {"choices":[]}\n\n');
  }));
  const bridge = await serveCommand({ host, "ask-bearer": "ask-secret", port: 0 });
  t.after(() => bridge.close());
  const response = await fetch(bridge.url + "/v1/chat/completions", { method: "POST", body: "{}" });
  const reader = response.body.getReader();
  assert.equal((await reader.read()).done, false);
  await reader.cancel();
  let timer;
  try {
    await Promise.race([upstreamClosed, new Promise((resolve, reject) => {
      timer = setTimeout(() => reject(new Error("Host stream was not cancelled")), 2000);
    })]);
  } finally { clearTimeout(timer); }
}

function parserAndKeccakMatchCanonicalInputs() {
  assert.equal(keccak256(Buffer.alloc(0)).toString("hex"), "c5d2460186f7233c927e7db2dcc703c0e500b653ca82273b7bfad8045d85a470");
  assert.equal(keccak256(Buffer.from("abc")).toString("hex"), "4e03657aea45a94fc7d47ba826c8d667c0d1e6e33a64a036ec44f58fa12d6c45");
  assert.throws(() => parseCLI(["join", "http://127.0.0.1", room]), /--manifest/);
  assert.throws(() => parseCLI(["join", "http://127.0.0.1", "wrong", "--manifest", "x"]), /room/);
  assert.throws(() => parseCLI(["serve", "--host", "https://host.invalid", "--port", "65536"]), /port/);
  assert.throws(() => parseCLI(["serve", "--host", "https://host.invalid", "--unknown", "x"]), /Unknown option/);
  assert.throws(() => parseCLI(["join", "http://public.invalid", room, "--manifest", "x"]), /HTTPS/);
  assert.throws(() => checkAttestation(Buffer.alloc(96), {}), /Invalid PXVE/);
  assert.deepEqual(parseCLI(["status"]), { command: "status", port: 8788 });
}

function denseShard() {
  const cfg = { num_hidden_layers: 1, hidden_size: 2, num_attention_heads: 1, num_key_value_heads: 1,
    intermediate_size: 2, rms_norm_eps: 1e-5, rope_theta: 10000, hidden_act: "silu" };
  const tensors = {
    "input_layernorm.weight": [[2], [1, 1]], "post_attention_layernorm.weight": [[2], [1, 1]],
    "self_attn.q_proj.weight": [[2, 2], [1, 0, 0, 1]],
    "self_attn.k_proj.weight": [[2, 2], [1, 0, 0, 1]],
    "self_attn.v_proj.weight": [[2, 2], [1, 0, 0, 1]],
    "self_attn.o_proj.weight": [[2, 2], [1, 0, 0, 1]],
    "mlp.gate_proj.weight": [[2, 2], [0, 0, 0, 0]],
    "mlp.up_proj.weight": [[2, 2], [0, 0, 0, 0]],
    "mlp.down_proj.weight": [[2, 2], [0, 0, 0, 0]],
  };
  const header = {}, data = [];
  let offset = 0;
  for (const [name, [shape, values]] of Object.entries(tensors)) {
    const bytes = Buffer.alloc(values.length * 4);
    values.forEach((value, i) => bytes.writeFloatLE(value, i * 4));
    header["model.layers.0." + name] = { dtype: "F32", shape, data_offsets: [offset, offset + bytes.length] };
    offset += bytes.length; data.push(bytes);
  }
  let text = JSON.stringify(header);
  text += " ".repeat((8 - Buffer.byteLength(text) % 8) % 8);
  const size = Buffer.alloc(8); size.writeBigUInt64LE(BigInt(Buffer.byteLength(text)));
  return { manifest: { format: "safetensors-dense-v1", config: cfg, layer_count: 1, hidden_size: 2 },
    shards: [{ bytes: Uint8Array.from(Buffer.concat([size, Buffer.from(text), ...data])) }], firstLayer: 0, endLayer: 1 };
}

async function cpuDensePathRunsCausalAttentionAndIsolatesCaches() {
  const source = denseShard();
  const runtime = await createNativeRuntime({ backend: "cpu", maxSeq: 2 });
  assert.equal(runtime.backend, "cpu");
  const model = await runtime.loadLayerRange(source);
  const a = model.createCache("a"), b = model.createCache("b");
  const input = new Float32Array([1, 2]);
  const first = await model.runActivation(input, 0, a);
  const scale = Math.sqrt(2.5 + 1e-5);
  for (let i = 0; i < 2; i++) assert.ok(Math.abs(first[i] - input[i] - input[i] / scale) < 1e-6);
  assert.deepEqual(await model.runActivation(input, 0, b), first);
  const next = new Float32Array([2, 1]);
  const second = await model.runActivation(next, 1, a);
  const q = [(2 * Math.cos(1) - Math.sin(1)) / scale, (2 * Math.sin(1) + Math.cos(1)) / scale];
  const oldScore = (q[0] + 2 * q[1]) / scale / Math.sqrt(2);
  const newScore = 5 / (scale * scale) / Math.sqrt(2);
  const oldProbability = 1 / (1 + Math.exp(newScore - oldScore));
  const expected = [2 + (oldProbability + 2 * (1 - oldProbability)) / scale,
    1 + (2 * oldProbability + 1 - oldProbability) / scale];
  for (let i = 0; i < 2; i++) assert.ok(Math.abs(second[i] - expected[i]) < 1e-6);
  await assert.rejects(model.runActivation(next, 0, a), /cache position/);
  await assert.rejects(model.runActivation(next, 2, a), /cache position/);
  model.dropCache(a);
  assert.equal(a.layers[0].keys.length, 0);
  await assert.rejects(model.runActivation(next, 1, a), /cache position/);
  model.close();
  assert.equal(b.layers[0].values.length, 0);
  assert.throws(() => model.createCache("closed"), /closed/);
  await assert.rejects(createCPURuntime().loadLayerRange({ ...source,
    manifest: { ...source.manifest, format: "unsupported" } }), /Unsupported CPU/);
}

test(joinParsesArgumentsAndSendsSignedHello, { timeout: 15000 });
test(attestationBindingPrecedesEnrollmentAndHello, { timeout: 10000 });
test(mismatchedAttestationIsRefusedBeforeAnySend, { timeout: 10000 });
test(unconfirmedBindingStopsBeforeEnrollment, { timeout: 10000 });
test(serveForwardsAskBearerAndStreamsBeforeHostFinishes, { timeout: 10000 });
test(servePreservesHostRefusalsAndRejectsUnknownRoutes, { timeout: 10000 });
test(serveCancelsUpstreamWhenLocalClientDisconnects, { timeout: 10000 });
test(parserAndKeccakMatchCanonicalInputs);
test(cpuDensePathRunsCausalAttentionAndIsolatesCaches);
