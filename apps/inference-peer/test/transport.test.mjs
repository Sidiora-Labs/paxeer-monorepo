import assert from "node:assert/strict";
import { createHash } from "node:crypto";
import { EventEmitter, once } from "node:events";
import { createServer } from "node:http";
import test from "node:test";
import { PeerMesh, SignalClient } from "../src/transport.mjs";
import { decodeFrame, encodeFrame, MAX_FRAME_BYTES } from "../src/wire.mjs";

const roomId = "room-1";
const hello = {
  peerKey: "peer-a",
  signature: "signed-room-and-nonce",
  nonce: "nonce-1",
  manifest: { version: 2, runtimeKind: "browser", acceleratorMemory: 1024 },
};

// A real loopback HTTP upgrade and RFC 6455 binary exchange. This stand-in
// exercises the platform WebSocket client without implementing chain admission.
class RoomHost extends EventEmitter {
  server = createServer();
  connections = new Set();

  async listen(t, handler) {
    this.server.on("upgrade", (request, socket, head) => {
      const accept = createHash("sha1")
        .update(
          request.headers["sec-websocket-key"] +
            "258EAFA5-E914-47DA-95CA-C5AB0DC85B11",
        )
        .digest("base64");
      socket.write(
        "HTTP/1.1 101 Switching Protocols\r\nUpgrade: websocket\r\nConnection: Upgrade\r\nSec-WebSocket-Accept: " +
          accept +
          "\r\n\r\n",
      );
      this.connections.add(socket);
      socket.on("error", () => {});
      socket.on("close", () => this.connections.delete(socket));
      let buffer = Buffer.from(head);
      const connection = {
        socket,
        frames: [],
        sequence: 0,
        send: (kind, payload, overrides = {}) => {
          const frame = {
            kind,
            roomId,
            sequence: connection.sequence++,
            payload,
            ...overrides,
          };
          this.sendBytes(socket, encodeFrame(frame));
          return frame;
        },
      };
      const consume = () => {
        while (buffer.length >= 2) {
          const opcode = buffer[0] & 15;
          assert.equal(
            buffer[0] & 128,
            128,
            "stand-in expects complete WebSocket messages",
          );
          assert.equal(buffer[1] & 128, 128, "client messages must be masked");
          let length = buffer[1] & 127;
          let offset = 2;
          if (length === 126) {
            if (buffer.length < 4) return;
            length = buffer.readUInt16BE(2);
            offset = 4;
          } else if (length === 127) {
            if (buffer.length < 10) return;
            length = Number(buffer.readBigUInt64BE(2));
            offset = 10;
          }
          assert.ok(length <= MAX_FRAME_BYTES);
          if (buffer.length < offset + 4 + length) return;
          const mask = buffer.subarray(offset, offset + 4);
          offset += 4;
          const body = Buffer.from(buffer.subarray(offset, offset + length));
          for (let i = 0; i < length; i++) body[i] ^= mask[i % 4];
          buffer = buffer.subarray(offset + length);
          if (opcode === 8) {
            socket.end(Buffer.from([0x88, 0]));
            return;
          }
          assert.equal(opcode, 2, "room frames use binary WebSocket messages");
          const frame = decodeFrame(body);
          connection.frames.push(frame);
          handler(connection, frame);
          this.emit("frame", connection, frame);
        }
      };
      socket.on("data", (data) => {
        buffer = Buffer.concat([buffer, data]);
        consume();
      });
      consume();
    });
    this.server.listen(0, "127.0.0.1");
    await once(this.server, "listening");
    t.after(async () => {
      for (const socket of this.connections) socket.destroy();
      await new Promise((resolve) => this.server.close(resolve));
    });
    return `ws://127.0.0.1:${this.server.address().port}/room`;
  }

  sendBytes(socket, bytes) {
    let header;
    if (bytes.length < 126) header = Buffer.from([0x82, bytes.length]);
    else if (bytes.length <= 65535) {
      header = Buffer.alloc(4);
      header[0] = 0x82;
      header[1] = 126;
      header.writeUInt16BE(bytes.length, 2);
    } else {
      header = Buffer.alloc(10);
      header[0] = 0x82;
      header[1] = 127;
      header.writeBigUInt64BE(BigInt(bytes.length), 2);
    }
    socket.write(Buffer.concat([header, bytes]));
  }
}

function client(t, url, options = {}) {
  const signal = new SignalClient(url, { roomId, hello, ...options });
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
  const host = new RoomHost();
  const observed = [];
  const offerPayload = {
    modelDigest: "model-digest",
    runtimeDigest: "runtime-digest",
    layerPlan: [0, 12],
    price: "10",
  };
  const acknowledged = new Promise((resolve) =>
    host.on("frame", (connection, frame) => {
      if (frame.kind === "deal-ack") resolve({ connection, frame });
    }),
  );
  const url = await host.listen(t, (connection, frame) => {
    observed.push(frame.kind);
    if (frame.kind === "hello") {
      assert.deepEqual(frame.payload, hello);
      assert.equal(frame.roomId, roomId);
      assert.equal(frame.sequence, 0);
      connection.send("offer", offerPayload);
      observed.push("offer");
      connection.send("deal", {
        dealId: "deal-1",
        peers: [hello.peerKey, "peer-b"],
      });
    }
  });
  const signal = client(t, url);
  assert.throws(() => signal.acknowledgeDeal("deal-1"), /No matching deal/);
  const received = [];
  signal.addEventListener("frame", ({ detail: frame }) => {
    received.push(frame.kind);
    if (frame.kind === "deal") {
      assert.throws(
        () => signal.acknowledgeDeal("wrong-deal"),
        /No matching deal/,
      );
      signal.acknowledgeDeal(frame.payload.dealId);
    }
  });
  const offer = await signal.connect();
  assert.deepEqual(offer.payload, offerPayload);
  const { frame } = await acknowledged;
  assert.deepEqual(observed, ["hello", "offer", "deal-ack"]);
  assert.deepEqual(received, ["offer", "deal"]);
  assert.deepEqual(frame.payload, { dealId: "deal-1" });
  assert.equal(frame.sequence, 1);
  assert.equal(signal.state, "ready");
  assert.throws(() => signal.acknowledgeDeal("deal-1"), /No matching deal/);
  await assert.rejects(signal.connect(), /already started/);
}

async function descriptionsAndCandidatesRelayToNamedPeer(t) {
  const host = new RoomHost();
  const peers = new Map();
  const routed = [];
  const url = await host.listen(t, (connection, frame) => {
    if (frame.kind === "hello") {
      peers.set(frame.payload.peerKey, connection);
      connection.key = frame.payload.peerKey;
      connection.send("offer", { roomId });
    } else {
      assert.equal(frame.kind, "offer");
      const { to, relay } = frame.payload;
      routed.push(to);
      peers.get(to).send("offer", { from: connection.key, to, relay });
    }
  });
  const a = client(t, url);
  const b = client(t, url, { hello: { ...hello, peerKey: "peer-b" } });
  await Promise.all([a.connect(), b.connect()]);
  assert.throws(() => a.relay("peer-b", {}), /one description or candidate/);
  let unexpected = 0;
  a.addEventListener("frame", () => unexpected++);
  const description = {
    description: { type: "offer", sdp: "v=0\r\ns=room\r\n" },
  };
  const incomingDescription = eventDetail(b, "frame");
  a.relay("peer-b", description);
  assert.deepEqual((await incomingDescription).payload, {
    from: hello.peerKey,
    to: "peer-b",
    relay: description,
  });
  const candidate = {
    candidate: {
      candidate: "candidate:1 1 UDP 1 127.0.0.1 1234 typ host",
      sdpMid: "0",
    },
  };
  const incomingCandidate = eventDetail(b, "frame");
  a.relay("peer-b", candidate);
  assert.deepEqual((await incomingCandidate).payload.relay, candidate);
  assert.deepEqual(routed, ["peer-b", "peer-b"]);
  assert.equal(unexpected, 0);
}

async function invalidSignallingFramesCloseTheClient(t) {
  const cases = [
    [
      "deal before offer",
      (host, connection) => connection.send("deal", { dealId: "d" }),
      /Expected room offer/,
    ],
    [
      "wrong room",
      (host, connection) => connection.send("offer", {}, { roomId: "other" }),
      /Wrong signalling room/,
    ],
    [
      "unknown kind",
      (host, connection) => {
        const bytes = encodeFrame({
          kind: "offer",
          roomId,
          sequence: 0,
          payload: {},
        });
        bytes[1] = 255;
        host.sendBytes(connection.socket, bytes);
      },
      /Unknown frame kind/,
    ],
    [
      "oversize",
      (host, connection) =>
        host.sendBytes(connection.socket, new Uint8Array(MAX_FRAME_BYTES + 1)),
      /exceeds 65536/,
    ],
  ];
  for (const [name, respond, pattern] of cases) {
    await t.test(name, async (t) => {
      const host = new RoomHost();
      const url = await host.listen(t, (connection) =>
        respond(host, connection),
      );
      const signal = client(t, url);
      const errorEvent = eventDetail(signal, "error");
      await assert.rejects(signal.connect(), pattern);
      assert.match((await errorEvent).message, pattern);
      assert.equal(signal.state, "closed");
    });
  }
}

async function replayedSequenceAndDuplicateOfferRefuse(t) {
  for (const duplicate of [false, true]) {
    await t.test(
      duplicate ? "duplicate offer" : "replayed sequence",
      async (t) => {
        const host = new RoomHost();
        const url = await host.listen(t, (connection) => {
          connection.send("offer", {});
          if (duplicate) connection.send("offer", {});
          else connection.send("deal", { dealId: "d" }, { sequence: 0 });
        });
        const signal = client(t, url);
        const error = eventDetail(signal, "error");
        await signal.connect();
        assert.match(
          (await error).message,
          duplicate ? /Duplicate room offer/ : /Replayed signalling/,
        );
        assert.equal(signal.state, "closed");
      },
    );
  }
}

async function timeoutAndRemoteDisconnectRejectAdmission(t) {
  for (const disconnect of [false, true]) {
    await t.test(
      disconnect ? "remote disconnect" : "offer timeout",
      async (t) => {
        const host = new RoomHost();
        const url = await host.listen(t, (connection) => {
          if (disconnect) connection.socket.destroy();
        });
        const signal = client(t, url, { timeoutMs: 100 });
        await assert.rejects(
          signal.connect(),
          disconnect ? /WebSocket closed/ : /timed out/,
        );
        assert.equal(signal.state, "closed");
      },
    );
  }
}

async function outgoingOversizeRefusesBeforeSendingHello(t) {
  const host = new RoomHost();
  let received = 0;
  const url = await host.listen(t, () => received++);
  const signal = client(t, url, {
    hello: { ...hello, signature: "s".repeat(MAX_FRAME_BYTES) },
  });
  await assert.rejects(signal.connect(), /exceeds 65536/);
  assert.equal(signal.state, "closed");
  assert.equal(received, 0);
}

async function byeClosesTheSignallingSocket(t) {
  const host = new RoomHost();
  const url = await host.listen(t, (connection) => {
    connection.send("offer", {});
    connection.send("bye", { reason: "room closed" });
  });
  const signal = client(t, url);
  const closed = eventDetail(signal, "close");
  const observed = [];
  signal.addEventListener("frame", ({ detail }) => observed.push(detail.kind));
  await signal.connect();
  await closed;
  assert.deepEqual(observed, ["offer", "bye"]);
  assert.equal(signal.state, "closed");
  signal.close();
  assert.throws(
    () =>
      signal.relay("peer-b", { candidate: { candidate: "c", sdpMid: "0" } }),
    /Room offer is required/,
  );
}

function chainIdentityAndRuntimeRequirementsRefuseInvalidInputs() {
  for (const name of ["peerKey", "signature", "nonce"]) {
    assert.throws(
      () =>
        new SignalClient("ws://localhost/", {
          roomId,
          hello: { ...hello, [name]: "" },
        }),
      /required/,
    );
  }
  assert.throws(
    () =>
      new SignalClient("ws://localhost/", {
        roomId,
        hello: { ...hello, manifest: { version: 1 } },
      }),
    /v2 manifest/,
  );
  assert.throws(
    () => new SignalClient("https://localhost/", { roomId, hello }),
    /WebSocket URL/,
  );
  assert.throws(
    () => new SignalClient("ws://localhost/", { roomId, hello, timeoutMs: 0 }),
    /timeoutMs/,
  );
  assert.throws(
    () =>
      new SignalClient("ws://localhost/", { roomId, hello, WebSocket: null }),
    /unavailable/,
  );
  const signal = new SignalClient("ws://localhost/", { roomId, hello });
  assert.throws(
    () =>
      signal.relay("peer-b", { description: { type: "offer", sdp: "v=0" } }),
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
test(replayedSequenceAndDuplicateOfferRefuse);
test(timeoutAndRemoteDisconnectRejectAdmission);
test(outgoingOversizeRefusesBeforeSendingHello);
test(byeClosesTheSignallingSocket);
test(chainIdentityAndRuntimeRequirementsRefuseInvalidInputs);
