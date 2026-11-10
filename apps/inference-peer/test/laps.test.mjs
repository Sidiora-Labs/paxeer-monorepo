import assert from "node:assert/strict";
import { createHash, createPublicKey, verify } from "node:crypto";
import { once } from "node:events";
import { readFileSync } from "node:fs";
import { createServer as httpServer } from "node:http";
import { connect, createServer as tcpServer } from "node:net";
import test from "node:test";
import { DealClient, decodeActivation } from "../src/deal.mjs";
import {
  countersignLap,
  LapTracker,
  lapCountersignDigest,
  lapReceiptBytes,
  lapReceiptDigest,
  lapSigningDigest,
} from "../src/laps.mjs";
import {
  decodeSignalFrame,
  encodeSignalFrame,
  signalHex,
} from "../src/signal.mjs";
import { SignalClient } from "../src/transport.mjs";
import { decodeFrame, encodeFrame, MAX_FRAME_BYTES } from "../src/wire.mjs";
import {
  ScriptedRuntime,
  scriptedModelSource,
} from "../tests/scripted_model.mjs";
import {
  fixedIdentity,
  manifest,
  RoomHost,
  roomId,
  vector as signalVector,
} from "./host-signal.mjs";

const fixtures = JSON.parse(
  readFileSync(new URL("fixtures/host-lap-frames.json", import.meta.url)),
);
function vectorRecord(name) {
  const result = [...fixtures.vectors, ...fixtures.session].find(
    (item) => item.name === name,
  );
  assert.ok(result, `missing host vector ${name}`);
  return result;
}
function vector(name) {
  const item = vectorRecord(name);
  return decodeSignalFrame(item.json, item.direction);
}
function bytes(hex) {
  return new Uint8Array(Buffer.from(hex, "hex"));
}
function hex(value) {
  return Buffer.from(value).toString("hex");
}
function publicKey(raw) {
  return createPublicKey({
    key: Buffer.concat([
      Buffer.from("302a300506032b6570032100", "hex"),
      Buffer.from(raw),
    ]),
    format: "der",
    type: "spki",
  });
}
function verifyAcknowledgement(ack, receiptName) {
  const expected = vector(receiptName).lap;
  // Independent implementation of LapFrame::countersign_digest, over fixed
  // program-encoded receipt bytes rather than the peer's digest function.
  const receipt = expected.receipt;
  const u64 = (n) => {
    const result = Buffer.alloc(8);
    result.writeBigUInt64BE(BigInt(n));
    return result;
  };
  const bitmap = Buffer.alloc(4);
  bitmap.writeUInt32BE(receipt.peer_countersignatures);
  const encoded = Buffer.concat([
    Buffer.from(receipt.deal_id),
    ...[
      "lap_index",
      "prompt_tokens",
      "generated_tokens",
      "cumulative_tokens",
    ].map((name) => u64(receipt[name])),
    Buffer.from([receipt.layer_range_digests.length]),
    ...receipt.layer_range_digests.map((digest) => Buffer.from(digest)),
    Buffer.from(receipt.host_signature),
    bitmap,
  ]);
  const receiptDigest = createHash("sha256")
    .update("PAXAI/lap-receipt/v1\0")
    .update(encoded)
    .digest();
  const digest = createHash("sha256")
    .update("PAXAI/lap-countersign/v1\0")
    .update(Buffer.from(expected.deal_digest))
    .update(Buffer.from(expected.host_meter_key))
    .update(u64(expected.deadline_ms))
    .update(receiptDigest)
    .digest();
  assert.deepEqual(Buffer.from(ack.digest), digest);
  assert.ok(
    verify(
      null,
      digest,
      publicKey(Buffer.from(fixtures.peer_key, "hex")),
      Buffer.from(ack.signature),
    ),
  );
  assert.equal(
    verify(
      null,
      receiptDigest,
      publicKey(Buffer.from(fixtures.peer_key, "hex")),
      Buffer.from(ack.signature),
    ),
    false,
    "host requires the bound countersign digest, not the bare receipt digest",
  );
}
async function tracker() {
  return new LapTracker({
    identity: await fixedIdentity(),
    deal: vector("deal").deal,
    hostMeterKey: fixtures.host_meter_key,
  });
}
async function record(tracker, from = 0, to = 4) {
  for (const activation of fixtures.activations.slice(from, to))
    await tracker.recordActivation(bytes(activation.output_hex));
}

// Loopback TCP carries real room frames; this adapter implements the mesh's
// send/event contract and does not emulate WebRTC APIs or model execution.
class SocketMesh extends EventTarget {
  sockets = new Map();
  sequence = 0;
  constructor(key) {
    super();
    this.key = key;
  }
  attach(peer, socket) {
    this.sockets.set(peer, socket);
    let pending = Buffer.alloc(0);
    socket.on("data", (data) => {
      pending = Buffer.concat([pending, data]);
      while (pending.length >= 6) {
        const size = pending.readUInt32BE(2) + 6;
        assert.ok(size <= MAX_FRAME_BYTES);
        if (pending.length < size) break;
        const frame = decodeFrame(pending.subarray(0, size));
        pending = pending.subarray(size);
        this.dispatchEvent(
          new CustomEvent("frame", { detail: { peerKey: peer, frame } }),
        );
      }
    });
    socket.on("error", (error) =>
      this.dispatchEvent(new CustomEvent("error", { detail: error })),
    );
  }
  send(peer, kind, payload) {
    const socket = this.sockets.get(peer);
    assert.ok(socket && !socket.destroyed, "live mesh socket required");
    const frame = encodeFrame({
      kind,
      roomId,
      sequence: this.sequence++,
      payload,
    });
    return new Promise((resolve, reject) =>
      socket.write(frame, (error) => (error ? reject(error) : resolve())),
    );
  }
  close() {
    for (const socket of this.sockets.values()) socket.destroy();
    this.sockets.clear();
    this.dispatchEvent(new Event("close"));
  }
}
async function link(t, left, right) {
  const server = tcpServer();
  server.listen(0, "127.0.0.1");
  await once(server, "listening");
  const accepted = once(server, "connection");
  const outbound = connect(server.address().port, "127.0.0.1");
  await once(outbound, "connect");
  const [inbound] = await accepted;
  left.attach(right.key, outbound);
  right.attach(left.key, inbound);
  t.after(() => {
    outbound.destroy();
    inbound.destroy();
    server.close();
  });
}
async function socketSession(t) {
  const identity = await fixedIdentity(),
    host = new RoomHost();
  const received = [],
    failures = [];
  let connection;
  function send(name) {
    const item = vectorRecord(name);
    assert.equal(item.direction, "server");
    host.sendText(connection.socket, item.json);
  }
  const url = await host.listen(t, (client, frame) => {
    connection = client;
    if (frame.type === "hello") {
      assert.deepEqual(frame, signalVector("hello"));
      send("offer");
    } else if (frame.type === "deal_ack") {
      assert.deepEqual(frame, vector("deal_ack"));
      send("deal_live");
    } else {
      const expected =
        received.length === 0 ? "lap_countersign" : "lap_countersign_next";
      // A retransmitted first acknowledgement is explicitly allowed.
      const name =
        BigInt(frame.lap_index) === 0n ? "lap_countersign" : expected;
      assert.deepEqual(frame, vector(name));
      verifyAcknowledgement(
        frame,
        BigInt(frame.lap_index) === 0n ? "lap_receipt" : "lap_receipt_next",
      );
      received.push(frame);
    }
  });
  const signal = new SignalClient(url, { roomId, identity, manifest });
  t.after(() => signal.close());
  await signal.connect();
  const source = await scriptedModelSource();
  assert.deepEqual(
    Array.from(source.digest),
    vector("deal").deal.model_digest,
  );
  const downloads = [];
  const server = httpServer((request, response) => {
    downloads.push(request.url);
    const data = source.resources.get(request.url);
    response.writeHead(data ? 200 : 404);
    response.end(data);
  });
  server.listen(0, "127.0.0.1");
  await once(server, "listening");
  t.after(
    () =>
      new Promise((resolve) => {
        server.closeAllConnections();
        server.close(resolve);
      }),
  );
  const peer = new SocketMesh(identity.peerKey),
    previous = new SocketMesh(fixtures.previous_peer_key),
    hostMesh = new SocketMesh(fixtures.host_meter_key);
  for (const mesh of [peer, previous, hostMesh]) t.after(() => mesh.close());
  await link(t, previous, peer);
  await link(t, peer, hostMesh);
  const laps = await tracker();
  const client = new DealClient({
    roomId,
    identity,
    offer: vector("offer").offer,
    runtime: new ScriptedRuntime(),
    mesh: peer,
    hostPeerKey: fixtures.host_meter_key,
    validateDeal: async (deal) => {
      assert.deepEqual(deal, vector("deal").deal);
      return true;
    },
    acknowledge: (ack) => signal.acknowledgeDeal(ack),
    modelSource: {
      manifest_url: `http://127.0.0.1:${server.address().port}/manifest.json`,
    },
    lapTracker: laps,
    countersign: (ack) => signal.countersignLap(ack),
  });
  t.after(() => client.close());
  signal.addEventListener("frame", ({ detail }) => {
    if (["deal", "deal_live", "lap_receipt"].includes(detail.type))
      client.handleFrame(detail).catch((error) => {
        failures.push(error);
        client.dispatchEvent(new CustomEvent("error", { detail: error }));
      });
  });
  client.addEventListener("error", ({ detail }) => failures.push(detail));
  t.after(() => assert.deepEqual(failures, []));
  const ready = once(client, "ready");
  send("deal");
  await ready;
  assert.equal(signal.state, "ready");
  assert.deepEqual(downloads, [
    "/manifest.json",
    "/layer-2.json",
    "/layer-3.json",
  ]);
  async function process(from = 0, to = 4) {
    for (const activation of fixtures.activations.slice(from, to)) {
      const output = once(hostMesh, "frame"),
        processed = once(client, "activation");
      await previous.send(
        identity.peerKey,
        "activation",
        bytes(activation.input_hex),
      );
      const [{ detail }] = await output;
      assert.equal(detail.peerKey, identity.peerKey);
      assert.equal(detail.frame.kind, "activation");
      assert.equal(hex(detail.frame.payload), activation.output_hex);
      await processed;
    }
  }
  async function countersign(name = "lap_receipt") {
    const ack = once(host, "frame");
    send(name);
    const [, frame] = await ack;
    assert.equal(frame.type, "lap_countersign");
    return frame;
  }
  return { client, signal, laps, host, send, process, countersign, received };
}

async function program_lap_codec_fixture_and_signature_match() {
  const fixture = fixtures.program_codec,
    receipt = fixture.receipt;
  assert.equal(hex(lapReceiptBytes(receipt)), fixture.encoded_hex);
  assert.equal(
    hex(await lapSigningDigest(receipt)),
    fixture.signing_digest_hex,
  );
  assert.equal(
    hex(await lapReceiptDigest(receipt)),
    fixture.receipt_digest_hex,
  );
  assert.ok(
    verify(
      null,
      Buffer.from(fixture.signing_digest_hex, "hex"),
      publicKey(Buffer.from(fixtures.host_meter_key, "hex")),
      Buffer.from(receipt.host_signature),
    ),
  );
  const laps = await tracker();
  await record(laps);
  const ack = await countersignLap(vector("lap_receipt"), laps);
  assert.deepEqual(ack, vector("lap_countersign"));
  assert.equal(
    hex(await lapCountersignDigest(vector("lap_receipt").lap)),
    hex(ack.digest),
  );
  verifyAcknowledgement(ack, "lap_receipt");
  assert.equal(laps.nextLap, 1n);
  assert.equal(laps.cumulativeTokens, 4n);
}
async function matching_receipts_are_countersigned_over_text_websockets(t) {
  const session = await socketSession(t);
  await session.process(0, 6);
  assert.equal(session.laps.pendingActivations, 6);
  assert.deepEqual(await session.countersign(), vector("lap_countersign"));
  assert.equal(session.laps.pendingActivations, 2);
  assert.deepEqual(await session.countersign(), vector("lap_countersign"));
  assert.equal(session.laps.nextLap, 1n);
  assert.deepEqual(
    await session.countersign("lap_receipt_next"),
    vector("lap_countersign_next"),
  );
  assert.equal(session.laps.nextLap, 2n);
  assert.equal(session.laps.cumulativeTokens, 6n);
  assert.equal(session.laps.pendingActivations, 0);
  assert.equal(session.received.length, 3);
  const wrong = vector("lap_countersign_next");
  wrong.digest[0] ^= 1;
  assert.throws(
    () => session.signal.countersignLap(wrong),
    /No matching lap/,
  );
}
async function foreign_range_digest_is_refused_without_losing_evidence(t) {
  const session = await socketSession(t);
  await session.process();
  const refused = once(session.client, "lap-refusal");
  session.send("lap_receipt_foreign_range");
  const [{ detail }] = await refused;
  assert.deepEqual(detail, {
    type: "refusal",
    reason: "range_digest_mismatch",
  });
  assert.equal(session.received.length, 0);
  assert.equal(session.signal.state, "ready");
  assert.equal(session.laps.pendingActivations, 4);
  assert.equal(session.laps.nextLap, 0n);
  assert.deepEqual(await session.countersign(), vector("lap_countersign"));
}
async function lap_index_gap_is_refused_on_the_real_socket(t) {
  const session = await socketSession(t);
  await session.process();
  const refused = once(session.client, "lap-refusal");
  session.send("lap_receipt_gap");
  const [{ detail }] = await refused;
  assert.deepEqual(detail, { type: "refusal", reason: "lap_index_gap" });
  assert.equal(session.received.length, 0);
  assert.equal(session.laps.cumulativeTokens, 0n);
  assert.deepEqual(await session.countersign(), vector("lap_countersign"));
}
async function token_counts_host_binding_signature_and_deadline_are_checked() {
  const laps = await tracker();
  await record(laps);
  for (const [name, reason] of [
    ["bad_tokens", "cumulative_tokens_mismatch"],
    ["zero_generated", "generated_tokens_invalid"],
    ["bad_signature", "host_signature_invalid"],
    ["wrong_deal", "deal_mismatch"],
    ["wrong_binding", "deal_mismatch"],
    ["wrong_key", "host_meter_key_mismatch"],
    ["expired", "deadline_expired"],
    ["nonempty_bitmap", "issued_bitmap_not_empty"],
  ]) {
    assert.deepEqual(
      await countersignLap(vector("lap_receipt_" + name), laps),
      { type: "refusal", reason },
      name,
    );
    assert.equal(laps.nextLap, 0n);
    assert.equal(laps.cumulativeTokens, 0n);
    assert.equal(laps.pendingActivations, 4);
  }
  assert.deepEqual(
    await countersignLap(vector("lap_receipt"), laps),
    vector("lap_countersign"),
  );
  assert.deepEqual(
    await countersignLap(vector("lap_receipt_conflict"), laps),
    { type: "refusal", reason: "receipt_replay_conflict" },
  );
  await record(laps, 4, 6);
  assert.deepEqual(
    await countersignLap(vector("lap_receipt_later_prompt"), laps),
    { type: "refusal", reason: "prompt_tokens_invalid" },
  );
  assert.deepEqual(
    await countersignLap(vector("lap_receipt_next"), laps),
    vector("lap_countersign_next"),
  );
}
async function only_locally_processed_contiguous_activations_can_be_signed() {
  const laps = await tracker();
  assert.deepEqual(await countersignLap(vector("lap_receipt"), laps), {
    type: "refusal",
    reason: "unprocessed_activations",
  });
  await assert.rejects(
    laps.recordActivation(bytes(fixtures.activations[1].output_hex)),
    /not continuous/,
  );
  await assert.rejects(
    laps.recordActivation(bytes(fixtures.activations[0].input_hex)),
    /does not match lap range/,
  );
  const wrongDeal = bytes(fixtures.activations[0].output_hex);
  wrongDeal[3 + new TextEncoder().encode("lap-request").length] ^= 1;
  await assert.rejects(
    laps.recordActivation(wrongDeal),
    /does not match lap range/,
  );
  const payload = Buffer.from(fixtures.activations[0].output_hex, "hex");
  const pending = laps.recordActivation(payload);
  payload.fill(0);
  await pending;
  await assert.rejects(
    laps.recordActivation(bytes(fixtures.activations[0].output_hex)),
    /not continuous/,
  );
  await record(laps, 1, 4);
  const frame = vector("lap_receipt"),
    expected = vector("lap_countersign");
  const first = countersignLap(frame, laps);
  frame.lap.receipt.layer_range_digests[1].fill(0);
  const repeated = countersignLap(vector("lap_receipt"), laps);
  assert.deepEqual(await first, expected);
  assert.deepEqual(await repeated, expected);
  const ack = await countersignLap(vector("lap_receipt"), laps);
  ack.signature.fill(0);
  assert.deepEqual(
    await countersignLap(vector("lap_receipt"), laps),
    expected,
  );
  assert.equal(laps.nextLap, 1n);
  assert.equal(laps.pendingActivations, 0);
  assert.equal(
    decodeActivation(bytes(fixtures.activations[0].output_hex)).layer,
    4,
  );
}
function lap_vectors_enforce_exact_host_members_and_directions() {
  assert.deepEqual(
    new Set(fixtures.vectors.map((item) => JSON.parse(item.json).type)),
    new Set(["lap_receipt", "lap_countersign"]),
  );
  for (const item of [...fixtures.vectors, ...fixtures.session]) {
    const frame = decodeSignalFrame(item.json, item.direction);
    assert.equal(
      encodeSignalFrame(frame, item.direction),
      item.json,
      item.name,
    );
    assert.throws(
      () => encodeSignalFrame({ ...frame, future: true }, item.direction),
      /Unknown member/,
    );
    if (frame.type.startsWith("lap_")) {
      assert.throws(
        () =>
          encodeSignalFrame(
            frame,
            item.direction === "client" ? "server" : "client",
          ),
        /Unknown signalling type/,
      );
      for (const key of Object.keys(frame).filter(
        (name) => name !== "type",
      )) {
        const missing = structuredClone(frame);
        delete missing[key];
        assert.throws(
          () => encodeSignalFrame(missing, item.direction),
          /Missing member/,
        );
      }
    }
  }
  for (const member of ["lap", "receipt"]) {
    const frame = vector("lap_receipt");
    (member === "lap" ? frame.lap : frame.lap.receipt).future = true;
    assert.throws(() => encodeSignalFrame(frame, "server"), /Unknown member/);
  }
  for (const key of Object.keys(vector("lap_receipt").lap.receipt)) {
    const frame = vector("lap_receipt");
    delete frame.lap.receipt[key];
    assert.throws(() => encodeSignalFrame(frame, "server"), /Missing member/);
  }
  for (const value of [-1, 1.5, Number.MAX_SAFE_INTEGER + 1, 1n << 64n]) {
    const frame = vector("lap_receipt");
    frame.lap.receipt.lap_index = value;
    assert.throws(() => encodeSignalFrame(frame, "server"), /u64/);
  }
  const max = (1n << 64n) - 1n;
  const frame = vector("lap_receipt");
  frame.lap.deadline_ms = max;
  frame.lap.receipt.lap_index = max;
  const text = encodeSignalFrame(frame, "server");
  assert.ok(text.includes('"lap_index":18446744073709551615'));
  assert.equal(decodeSignalFrame(text).lap.receipt.lap_index, max);
  assert.equal(decodeSignalFrame(text).lap.deadline_ms, max);
  assert.throws(() => decodeSignalFrame(bytes("0102")), /text frame/);
  assert.throws(
    () => encodeSignalFrame({ type: "refusal", reason: "bad_receipt" }),
    /Unknown signalling type/,
  );
  const invalid = vector("lap_receipt");
  invalid.lap.receipt.layer_range_digests[1][0] = 256;
  assert.throws(() => encodeSignalFrame(invalid, "server"), /byte array/);
  assert.equal(
    signalHex(vector("lap_receipt").lap.host_meter_key),
    fixtures.host_meter_key,
  );
}

for (const criterion of [
  program_lap_codec_fixture_and_signature_match,
  matching_receipts_are_countersigned_over_text_websockets,
  foreign_range_digest_is_refused_without_losing_evidence,
  lap_index_gap_is_refused_on_the_real_socket,
  token_counts_host_binding_signature_and_deadline_are_checked,
  only_locally_processed_contiguous_activations_can_be_signed,
  lap_vectors_enforce_exact_host_members_and_directions,
])
  test(criterion.name, { timeout: 15000 }, criterion);
