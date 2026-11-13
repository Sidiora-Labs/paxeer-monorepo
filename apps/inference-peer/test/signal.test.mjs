import assert from "node:assert/strict";
import { createPublicKey, verify } from "node:crypto";
import test from "node:test";
import { decodeSignalFrame, encodeSignalFrame } from "../src/signal.mjs";
import { helloSigningBytes, signHello } from "../src/identity.mjs";
import { dealDigest } from "../src/deal.mjs";
import { SignalClient } from "../src/transport.mjs";
import {
  fixedIdentity,
  fixtures,
  hostHelloBytes,
  manifest,
  RoomHost,
  roomId,
  vector,
} from "./host-signal.mjs";

async function every_vector_round_trips_through_the_codec() {
  assert.equal(fixtures.vectors.length, 11);
  assert.deepEqual(
    fixtures.vectors.map((v) => v.name),
    [
      "challenge",
      "hello",
      "offer",
      "description_to",
      "description_from",
      "candidate_to",
      "candidate_from",
      "deal",
      "deal_ack",
      "deal_live",
      "error",
    ],
  );
  const key = createPublicKey({
    key: Buffer.concat([
      Buffer.from("302a300506032b6570032100", "hex"),
      Buffer.from(fixtures.key.peer_key_hex, "hex"),
    ]),
    format: "der",
    type: "spki",
  });
  for (const v of fixtures.vectors) {
    const decoded = decodeSignalFrame(v.json, v.direction);
    assert.deepEqual(decoded, JSON.parse(v.json), v.name);
    assert.equal(encodeSignalFrame(decoded, v.direction), v.json, v.name);
    if (v.signing_bytes_hex) {
      assert.equal(
        Buffer.from(decoded.signature).toString("hex"),
        v.signature_hex,
      );
      assert.ok(
        verify(
          null,
          Buffer.from(v.signing_bytes_hex, "hex"),
          key,
          Buffer.from(decoded.signature),
        ),
      );
    }
  }
  const ack = fixtures.vectors.find((v) => v.name === "deal_ack");
  const digest = await dealDigest(vector("deal").deal);
  assert.deepEqual(Array.from(digest), vector("deal_ack").digest);
  assert.equal(
    Buffer.concat([
      Buffer.from("PAXAI/inference-deal-ack/v1\0"),
      digest,
    ]).toString("hex"),
    ack.signing_bytes_hex,
  );
}
async function hello_is_signed_over_the_challenge_of_the_host(t) {
  const identity = await fixedIdentity(),
    host = new RoomHost();
  let received;
  const url = await host.listen(t, (connection, frame) => {
    received = frame;
    assert.deepEqual(frame, vector("hello"));
    connection.send(vector("offer"));
  });
  const client = new SignalClient(url, { roomId, identity, manifest });
  t.after(() => client.close());
  await client.connect();
  assert.deepEqual(received.nonce, vector("challenge").nonce);
  const v = fixtures.vectors.find((v) => v.name === "hello");
  assert.equal(
    Buffer.from(helloSigningBytes(roomId, received.nonce)).toString("hex"),
    v.signing_bytes_hex,
  );
  assert.deepEqual(
    Buffer.from(helloSigningBytes(roomId, received.nonce)),
    hostHelloBytes(received.room_id, received.nonce),
  );
  assert.deepEqual(
    await signHello(identity, { roomId, nonce: received.nonce, manifest }),
    received,
  );
  const changed = received.nonce.slice();
  changed[0] ^= 1;
  const another = await signHello(identity, {
    roomId,
    nonce: changed,
    manifest,
  });
  assert.notDeepEqual(another.signature, received.signature);
  await assert.rejects(
    signHello(identity, { roomId, manifest }),
    /32 byte array/,
  );
}
function unknown_member_is_refused() {
  for (const v of fixtures.vectors) {
    const frame = JSON.parse(v.json);
    assert.throws(
      () =>
        decodeSignalFrame(JSON.stringify({ ...frame, future: 1 }), v.direction),
      /Unknown member/,
    );
    for (const key of Object.keys(frame).filter((key) => key !== "type")) {
      const missing = { ...frame };
      delete missing[key];
      assert.throws(
        () => encodeSignalFrame(missing, v.direction),
        /Missing member/,
      );
    }
    for (const key of ["offer", "deal", "description", "candidate"].filter(
      (key) => frame[key],
    )) {
      const nested = structuredClone(frame);
      nested[key].future = 1;
      assert.throws(
        () => encodeSignalFrame(nested, v.direction),
        /Unknown member/,
      );
    }
  }
  assert.throws(
    () => decodeSignalFrame('{"type":"bye"}'),
    /Unknown signalling type/,
  );
}
async function binary_frame_on_the_socket_is_refused(t) {
  const host = new RoomHost();
  const url = await host.listen(t, (connection) =>
    host.sendText(connection.socket, JSON.stringify(vector("offer")), 2),
  );
  const client = new SignalClient(url, {
    roomId,
    identity: await fixedIdentity(),
    manifest,
  });
  t.after(() => client.close());
  await assert.rejects(client.connect(), /binary frame refused/);
  assert.equal(client.state, "closed");
  for (const binary of [new Uint8Array(), new ArrayBuffer(0), new Blob(["{}"])])
    assert.throws(() => decodeSignalFrame(binary), /text frame/);
}
function byte_lengths_types_bounds_and_directions_are_enforced() {
  for (const key of ["room_id", "peer_key", "nonce", "signature"]) {
    for (const value of [
      [],
      Array(31).fill(0),
      Array(key === "signature" ? 64 : 32).fill(256),
      "00".repeat(32),
    ]) {
      assert.throws(
        () => encodeSignalFrame({ ...vector("hello"), [key]: value }),
        /byte array/,
      );
    }
  }
  for (const [key, value] of [
    ["nonce", Array(33).fill(0)],
    ["signature", Array(65).fill(0)],
  ])
    assert.throws(
      () => encodeSignalFrame({ ...vector("hello"), [key]: value }),
      /byte array/,
    );
  assert.throws(
    () => encodeSignalFrame(vector("offer")),
    /Unknown signalling type/,
  );
  assert.throws(
    () => decodeSignalFrame(JSON.stringify(vector("hello"))),
    /Unknown signalling type/,
  );
  assert.throws(
    () => decodeSignalFrame('{"type":"error","code":"invented"}'),
    /error code/,
  );
  const exact = {
    ...vector("description_to"),
    description: { type: "offer", sdp: "" },
  };
  exact.description.sdp = "x".repeat(
    65536 - Buffer.byteLength(encodeSignalFrame(exact)),
  );
  assert.equal(Buffer.byteLength(encodeSignalFrame(exact)), 65536);
  exact.description.sdp += "x";
  assert.throws(() => encodeSignalFrame(exact), /exceeds 65536/);
  assert.throws(() => decodeSignalFrame(" ".repeat(65537)), /exceeds 65536/);
  exact.description.sdp = "é".repeat(32768);
  assert.throws(() => encodeSignalFrame(exact), /exceeds 65536/);
  const u64 = vector("deal");
  u64.deal.epoch = 18446744073709551615n;
  const text = encodeSignalFrame(u64, "server");
  assert.match(text, /"epoch":18446744073709551615/);
  assert.equal(decodeSignalFrame(text).deal.epoch, 18446744073709551615n);
  u64.deal.epoch++;
  assert.throws(() => encodeSignalFrame(u64, "server"), /Overflow/);
}

test(every_vector_round_trips_through_the_codec);
test(hello_is_signed_over_the_challenge_of_the_host);
test(unknown_member_is_refused);
test(binary_frame_on_the_socket_is_refused);
test(byte_lengths_types_bounds_and_directions_are_enforced);
