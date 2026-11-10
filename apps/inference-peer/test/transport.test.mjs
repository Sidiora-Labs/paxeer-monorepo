import assert from "node:assert/strict";
import { createPublicKey, verify } from "node:crypto";
import test from "node:test";
import { PeerMesh, SignalClient } from "../src/transport.mjs";
import {
  MAX_SIGNAL_FRAME_BYTES,
  signalBytes,
  signalHex,
} from "../src/signal.mjs";
import {
  fixedIdentity,
  fixtures,
  manifest,
  roomId,
  RoomHost,
  vector,
} from "./host-signal.mjs";

async function client(t, url, options = {}) {
  const signal = new SignalClient(url, {
    roomId,
    identity: await fixedIdentity(),
    manifest,
    ...options,
  });
  t.after(() => signal.close());
  return signal;
}
function eventDetail(target, name) {
  return new Promise((resolve) =>
    target.addEventListener(name, ({ detail }) => resolve(detail), {
      once: true,
    }),
  );
}
async function helloThenOfferThenDealAckInOrder(t) {
  const host = new RoomHost(),
    observed = [];
  const acknowledged = new Promise((resolve) =>
    host.on("frame", (connection, frame) => {
      if (frame.type === "deal_ack") resolve(frame);
    }),
  );
  const url = await host.listen(t, (connection, frame) => {
    observed.push(frame.type);
    if (frame.type === "hello") {
      assert.deepEqual(frame, vector("hello"));
      connection.send(vector("offer"));
      observed.push("offer");
      connection.send(vector("deal"));
    } else if (frame.type === "deal_ack") {
      assert.deepEqual(frame, vector("deal_ack"));
      const key = createPublicKey({
        key: Buffer.concat([
          Buffer.from("302a300506032b6570032100", "hex"),
          Buffer.from(fixtures.key.peer_key_hex, "hex"),
        ]),
        format: "der",
        type: "spki",
      });
      assert.ok(
        verify(
          null,
          Buffer.concat([
            Buffer.from("PAXAI/inference-deal-ack/v1\0"),
            Buffer.from(frame.digest),
          ]),
          key,
          Buffer.from(frame.signature),
        ),
      );
      connection.send(vector("deal_live"));
    }
  });
  const signal = await client(t, url);
  assert.throws(
    () => signal.acknowledgeDeal(vector("deal_ack")),
    /No matching deal/,
  );
  const received = [],
    ready = eventDetail(signal, "frame");
  const live = new Promise((resolve, reject) => {
    signal.addEventListener("error", ({ detail }) => reject(detail));
    signal.addEventListener("frame", ({ detail: frame }) => {
      received.push(frame.type);
      if (frame.type === "deal") {
        assert.throws(
          () =>
            signal.acknowledgeDeal({
              ...vector("deal_ack"),
              deal_id: Array(32).fill(99),
            }),
          /No matching deal/,
        );
        assert.throws(
          () =>
            signal.acknowledgeDeal({
              ...vector("deal_ack"),
              digest: Array(32).fill(99),
            }),
          /digest differs/,
        );
        signal.acknowledgeDeal(vector("deal_ack"));
        assert.equal(signal.state, "acknowledged");
      }
      if (frame.type === "deal_live") resolve();
    });
  });
  const offer = await signal.connect();
  await ready;
  assert.deepEqual(offer.offer, vector("offer").offer);
  const frame = await acknowledged;
  await live;
  assert.deepEqual(observed, ["hello", "offer", "deal_ack"]);
  assert.deepEqual(received, ["offer", "deal", "deal_live"]);
  assert.deepEqual(frame, vector("deal_ack"));
  assert.equal(signal.state, "ready");
  assert.throws(() => signal.acknowledgeDeal(frame), /No matching deal/);
  await assert.rejects(signal.connect(), /already started/);
}
async function descriptionsAndCandidatesRelayToNamedPeer(t) {
  const host = new RoomHost(),
    peers = new Map(),
    routed = [];
  const url = await host.listen(t, (connection, frame) => {
    if (frame.type === "hello") {
      connection.key = frame.peer_key;
      peers.set(signalHex(frame.peer_key), connection);
      connection.send(vector("offer"));
    } else {
      assert.ok(["description", "candidate"].includes(frame.type));
      const to = signalHex(frame.to);
      routed.push(to);
      peers
        .get(to)
        .send({
          type: frame.type,
          from: connection.key,
          [frame.type]: frame[frame.type],
        });
    }
  });
  const identity = await fixedIdentity();
  const pair = await crypto.subtle.generateKey("Ed25519", false, [
    "sign",
    "verify",
  ]);
  const { PeerIdentity } = await import("../src/identity.mjs");
  const second = new PeerIdentity(
    pair.privateKey,
    Buffer.from(await crypto.subtle.exportKey("raw", pair.publicKey)).toString(
      "hex",
    ),
  );
  const a = await client(t, url, { identity }),
    b = await client(t, url, { identity: second });
  await Promise.all([a.connect(), b.connect()]);
  assert.throws(
    () => a.relay(second.peerKey, {}),
    /one description or candidate/,
  );
  let unexpected = 0;
  a.addEventListener("frame", () => unexpected++);
  const description = vector("description_to").description;
  const incomingDescription = eventDetail(b, "frame");
  a.relay(second.peerKey, { description });
  assert.deepEqual(await incomingDescription, {
    type: "description",
    from: signalBytes(identity.peerKey),
    description,
  });
  const candidate = {
    candidate: vector("candidate_to").candidate.candidate,
    sdpMid: "0",
    sdpMLineIndex: 0,
  };
  const incomingCandidate = eventDetail(b, "frame");
  a.relay(second.peerKey, { candidate });
  assert.deepEqual(
    (await incomingCandidate).candidate,
    vector("candidate_from").candidate,
  );
  assert.deepEqual(routed, [second.peerKey, second.peerKey]);
  assert.equal(unexpected, 0);
}
async function invalidSignallingFramesCloseTheClient(t) {
  const cases = [
    [
      "deal before offer",
      (host, connection) => connection.send(vector("deal")),
      /Expected room offer/,
    ],
    [
      "wrong room",
      (host, connection) =>
        connection.send({
          type: "offer",
          offer: { ...vector("offer").offer, room_id: Array(32).fill(99) },
        }),
      /Wrong signalling room/,
    ],
    [
      "unknown type",
      (host, connection) =>
        host.sendText(connection.socket, '{"type":"future"}'),
      /Unknown signalling type/,
    ],
    [
      "oversize",
      (host, connection) =>
        host.sendText(
          connection.socket,
          " ".repeat(MAX_SIGNAL_FRAME_BYTES + 1),
        ),
      /exceeds 65536/,
    ],
    [
      "error code",
      (host, connection) => connection.send(vector("error")),
      /degraded/,
    ],
    [
      "binary",
      (host, connection) =>
        host.sendText(connection.socket, JSON.stringify(vector("offer")), 2),
      /binary frame refused/,
    ],
  ];
  for (const [name, respond, pattern] of cases)
    await t.test(name, async (t) => {
      const host = new RoomHost();
      const url = await host.listen(t, (connection) =>
        respond(host, connection),
      );
      const signal = await client(t, url),
        errorEvent = eventDetail(signal, "error");
      await assert.rejects(signal.connect(), pattern);
      assert.match((await errorEvent).message, pattern);
      assert.equal(signal.state, "closed");
    });
}
async function replayedChallengeAndDuplicateOfferRefuse(t) {
  for (const duplicate of [false, true])
    await t.test(
      duplicate ? "duplicate offer" : "replayed challenge",
      async (t) => {
        const host = new RoomHost();
        const url = await host.listen(t, (connection) => {
          connection.send(vector("offer"));
          connection.send(vector(duplicate ? "offer" : "challenge"));
        });
        const signal = await client(t, url),
          error = eventDetail(signal, "error");
        await signal.connect();
        assert.match(
          (await error).message,
          duplicate ? /Duplicate room offer/ : /Unexpected signalling/,
        );
        assert.equal(signal.state, "closed");
      },
    );
}
async function timeoutAndRemoteDisconnectRejectAdmission(t) {
  for (const disconnect of [false, true])
    await t.test(
      disconnect ? "remote disconnect" : "offer timeout",
      async (t) => {
        const host = new RoomHost();
        const url = await host.listen(t, (connection) => {
          if (disconnect) connection.socket.destroy();
        });
        const signal = await client(t, url, { timeoutMs: 100 });
        await assert.rejects(
          signal.connect(),
          disconnect ? /WebSocket closed/ : /timed out/,
        );
        assert.equal(signal.state, "closed");
      },
    );
}
async function outgoingOversizeRefusesBeforeSending(t) {
  const host = new RoomHost();
  let received = 0;
  const url = await host.listen(t, (connection) => {
    received++;
    connection.send(vector("offer"));
  });
  const signal = await client(t, url);
  await signal.connect();
  assert.throws(
    () =>
      signal.relay("16".repeat(32), {
        description: { type: "offer", sdp: "x".repeat(MAX_SIGNAL_FRAME_BYTES) },
      }),
    /exceeds 65536/,
  );
  assert.equal(signal.state, "offered");
  assert.equal(received, 1);
}
async function errorClosesTheSignallingSocket(t) {
  const host = new RoomHost();
  const url = await host.listen(t, (connection) => {
    connection.send(vector("offer"));
    connection.send(vector("error"));
  });
  const signal = await client(t, url),
    closed = eventDetail(signal, "close"),
    error = eventDetail(signal, "error");
  const observed = [];
  signal.addEventListener("frame", ({ detail }) => observed.push(detail.type));
  await signal.connect();
  await closed;
  assert.deepEqual(observed, ["offer"]);
  assert.equal((await error).code, "degraded");
  assert.equal(signal.state, "closed");
  signal.close();
  assert.throws(
    () =>
      signal.relay("16".repeat(32), {
        candidate: { candidate: "c", sdpMid: "0" },
      }),
    /Room offer is required/,
  );
}
async function chainIdentityAndRuntimeRequirementsRefuseInvalidInputs() {
  const options = { roomId, identity: await fixedIdentity(), manifest };
  for (const invalid of [
    { identity: null },
    { identity: { sign() {}, peerKey: "" } },
    { roomId: "" },
  ])
    assert.throws(
      () => new SignalClient("ws://localhost/", { ...options, ...invalid }),
      /required|byte array/,
    );
  assert.throws(
    () =>
      new SignalClient("ws://localhost/", {
        ...options,
        manifest: Uint8Array.from([0, 1]),
      }),
    /v2 manifest/,
  );
  assert.throws(
    () => new SignalClient("https://localhost/", options),
    /WebSocket URL/,
  );
  assert.throws(
    () => new SignalClient("ws://localhost/rooms/x/signal", options),
    /request target/,
  );
  assert.throws(
    () => new SignalClient("ws://localhost/", { ...options, timeoutMs: 0 }),
    /timeoutMs/,
  );
  assert.throws(
    () => new SignalClient("ws://localhost/", { ...options, WebSocket: null }),
    /unavailable/,
  );
  const signal = new SignalClient("ws://localhost/", options);
  assert.throws(
    () =>
      signal.relay("16".repeat(32), {
        description: { type: "offer", sdp: "v=0" },
      }),
    /Room offer is required/,
  );
  assert.throws(
    () => new PeerMesh(signal, { mode: "unknown" }),
    /Unknown mesh mode/,
  );
}

test(helloThenOfferThenDealAckInOrder);
test(descriptionsAndCandidatesRelayToNamedPeer);
test(invalidSignallingFramesCloseTheClient);
test(replayedChallengeAndDuplicateOfferRefuse);
test(timeoutAndRemoteDisconnectRejectAdmission);
test(outgoingOversizeRefusesBeforeSending);
test(errorClosesTheSignallingSocket);
test(chainIdentityAndRuntimeRequirementsRefuseInvalidInputs);
