import assert from "node:assert/strict";
import test from "node:test";
import {
  decodeFrame,
  encodeFrame,
  FRAME_KINDS,
  MAX_FRAME_BYTES,
} from "../src/wire.mjs";

function everyFrameRoundTrips() {
  assert.equal(Object.keys(FRAME_KINDS).length, 9);
  assert.deepEqual(
    [
      ...encodeFrame({
        kind: "bye",
        roomId: "r",
        sequence: 0x01020304,
        payload: {},
      }),
    ],
    [0, 9, 0, 0, 0, 9, 0, 1, 114, 1, 2, 3, 4, 123, 125],
  );
  for (const [kind, number] of Object.entries(FRAME_KINDS)) {
    const frame = {
      kind,
      roomId: "room/東京",
      sequence: 0xffffffff,
      payload:
        kind === "activation"
          ? new Uint8Array([0, 255, 128, 1, 0])
          : {
              peerKey: "peer",
              nested: { tokens: [1, 2], signed: true },
              text: "héllo 😀",
            },
    };
    const encoded = encodeFrame(frame);
    const header = new DataView(encoded.buffer);
    assert.equal(header.getUint16(0), number);
    assert.equal(header.getUint32(2), encoded.byteLength - 6);
    assert.deepEqual(decodeFrame(encoded), frame, kind);
    assert.deepEqual(decodeFrame(encoded.buffer), frame, `${kind} ArrayBuffer`);
    const offset = new Uint8Array(encoded.length + 10);
    offset.set(encoded, 5);
    assert.deepEqual(
      decodeFrame(offset.subarray(5, -5)),
      frame,
      `${kind} offset view`,
    );
    assert.deepEqual(
      decodeFrame(Buffer.from(encoded)),
      frame,
      `${kind} Node Buffer`,
    );
  }
}

function maximumFrameBoundaryAndOversizeRefusal() {
  const frame = {
    kind: "activation",
    roomId: "r",
    sequence: 0,
    payload: new Uint8Array(MAX_FRAME_BYTES - 13),
  };
  const encoded = encodeFrame(frame);
  assert.equal(encoded.byteLength, MAX_FRAME_BYTES);
  assert.deepEqual(decodeFrame(encoded), frame);
  assert.throws(
    () =>
      encodeFrame({ ...frame, payload: new Uint8Array(MAX_FRAME_BYTES - 12) }),
    /exceeds 65536/,
  );
  assert.throws(
    () =>
      encodeFrame({
        ...frame,
        kind: "hello",
        payload: { text: "a".repeat(MAX_FRAME_BYTES) },
      }),
    /exceeds 65536/,
  );
  assert.throws(
    () => decodeFrame(new Uint8Array(MAX_FRAME_BYTES + 1)),
    /exceeds 65536/,
  );
}

function unknownFrameKindRefuses() {
  const frame = { kind: "hello", roomId: "room", sequence: 0, payload: {} };
  for (const kind of ["unknown", "toString", "__proto__", 1]) {
    assert.throws(() => encodeFrame({ ...frame, kind }), /Unknown frame kind/);
  }
  for (const number of [0, 10, 65535]) {
    const encoded = encodeFrame(frame);
    new DataView(encoded.buffer).setUint16(0, number);
    assert.throws(() => decodeFrame(encoded), /Unknown frame kind/);
  }
}

function malformedFramesRefuse() {
  const frame = { kind: "bye", roomId: "r", sequence: 0, payload: {} };
  const encoded = encodeFrame(frame);
  for (let n = 0; n < encoded.length; n++) {
    assert.throws(
      () => decodeFrame(encoded.subarray(0, n)),
      /Truncated|length mismatch/,
    );
  }
  const trailing = new Uint8Array(encoded.length + 1);
  trailing.set(encoded);
  assert.throws(() => decodeFrame(trailing), /length mismatch/);
  const badRoom = encoded.slice();
  new DataView(badRoom.buffer).setUint16(6, 65535);
  assert.throws(() => decodeFrame(badRoom), /Truncated room/);
  const invalidUtf8 = encoded.slice();
  invalidUtf8[8] = 255;
  assert.throws(() => decodeFrame(invalidUtf8), /encoded data/);
  const invalidJson = encoded.slice();
  invalidJson[invalidJson.length - 1] = 0;
  assert.throws(() => decodeFrame(invalidJson), SyntaxError);
  const arrayPayload = encoded.slice();
  arrayPayload.set(new TextEncoder().encode("[]"), arrayPayload.length - 2);
  assert.throws(() => decodeFrame(arrayPayload), /JSON object/);
  for (const sequence of [-1, 1.2, 0x100000000, NaN]) {
    assert.throws(() => encodeFrame({ ...frame, sequence }), /u32/);
  }
  for (const payload of [null, [], "hello", 3]) {
    assert.throws(() => encodeFrame({ ...frame, payload }), /JSON object/);
  }
  assert.throws(() => encodeFrame({ ...frame, roomId: "" }), /roomId/);
  assert.throws(() => encodeFrame({ ...frame, roomId: "\ud800" }), /Unicode/);
  assert.throws(
    () => encodeFrame({ ...frame, kind: "activation" }),
    /binary buffer/,
  );
  assert.throws(() => decodeFrame("hello"), /binary buffer/);
}

function activationPayloadUsesViewBytesAndOwnsDecodedMemory() {
  const source = new Uint16Array([0xabcd, 0x1234, 0xffff]);
  const payload = new DataView(source.buffer, 2, 2);
  const encoded = encodeFrame({
    kind: "activation",
    roomId: "r",
    sequence: 7,
    payload,
  });
  const decoded = decodeFrame(encoded);
  assert.deepEqual(decoded.payload, new Uint8Array(source.buffer.slice(2, 4)));
  encoded.fill(0);
  assert.deepEqual(decoded.payload, new Uint8Array(source.buffer.slice(2, 4)));
}

test(everyFrameRoundTrips);
test(maximumFrameBoundaryAndOversizeRefusal);
test(unknownFrameKindRefuses);
test(malformedFramesRefuse);
test(activationPayloadUsesViewBytesAndOwnsDecodedMemory);
