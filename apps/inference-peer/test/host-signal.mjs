import assert from "node:assert/strict";
import { createHash, createPublicKey, verify } from "node:crypto";
import { EventEmitter, once } from "node:events";
import { readFileSync } from "node:fs";
import { createServer } from "node:http";
import { PeerIdentity, manifestDigest } from "../src/identity.mjs";
import {
  decodeSignalFrame,
  encodeSignalFrame,
  MAX_SIGNAL_FRAME_BYTES,
  signalHex,
} from "../src/signal.mjs";

export const fixtures = JSON.parse(
  readFileSync(new URL("fixtures/host-signal-frames.json", import.meta.url)),
);
export function vector(name) {
  return JSON.parse(fixtures.vectors.find((v) => v.name === name).json);
}
export const roomId = signalHex(vector("hello").room_id);
export const manifest = Uint8Array.from(vector("hello").manifest);
export async function fixedIdentity() {
  const key = await crypto.subtle.importKey(
    "pkcs8",
    Buffer.from(fixtures.key.pkcs8_hex, "hex"),
    "Ed25519",
    false,
    ["sign"],
  );
  return new PeerIdentity(key, fixtures.key.peer_key_hex);
}
export function hostHelloBytes(room, nonce) {
  return Buffer.concat([
    Buffer.from("PAXAI/inference-signal/hello/v1\0"),
    Buffer.from(room),
    Buffer.from(nonce),
  ]);
}
export function verifyHostHello(hello, nonce, room) {
  assert.equal(hello.type, "hello");
  assert.deepEqual(hello.room_id, room);
  assert.deepEqual(hello.nonce, nonce);
  const key = createPublicKey({
    key: Buffer.concat([
      Buffer.from("302a300506032b6570032100", "hex"),
      Buffer.from(hello.peer_key),
    ]),
    format: "der",
    type: "spki",
  });
  assert.ok(
    verify(
      null,
      hostHelloBytes(hello.room_id, hello.nonce),
      key,
      Buffer.from(hello.signature),
    ),
  );
}

// Real HTTP upgrade and masked RFC 6455 text exchange, with the host's schemas
// and independently constructed hello verification bytes. No chain admission.
export class RoomHost extends EventEmitter {
  server = createServer();
  connections = new Set();
  failures = [];
  async listen(
    t,
    handler,
    { room = vector("hello").room_id, challenge = vector("challenge") } = {},
  ) {
    this.server.on("upgrade", (request, socket, head) => {
      assert.equal(request.url, "/");
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
      let buffer = Buffer.from(head),
        queue = Promise.resolve();
      const connection = {
        socket,
        frames: [],
        send: (frame) => {
          this.sendText(socket, encodeSignalFrame(frame, "server"));
          return frame;
        },
      };
      const consume = () => {
        while (buffer.length >= 2) {
          const opcode = buffer[0] & 15;
          assert.equal(buffer[0] & 0xf0, 0x80);
          assert.equal(buffer[1] & 128, 128, "client messages must be masked");
          let size = buffer[1] & 127,
            offset = 2;
          if (size === 126) {
            if (buffer.length < 4) return;
            size = buffer.readUInt16BE(2);
            offset = 4;
          } else if (size === 127) {
            if (buffer.length < 10) return;
            size = Number(buffer.readBigUInt64BE(2));
            offset = 10;
          }
          assert.ok(size <= MAX_SIGNAL_FRAME_BYTES);
          if (buffer.length < offset + 4 + size) return;
          const mask = buffer.subarray(offset, offset + 4);
          offset += 4;
          const body = Buffer.from(buffer.subarray(offset, offset + size));
          for (let i = 0; i < size; i++) body[i] ^= mask[i % 4];
          buffer = buffer.subarray(offset + size);
          if (opcode === 8) {
            socket.end(Buffer.from([0x88, 0]));
            return;
          }
          assert.equal(opcode, 1, "host accepts only JSON text messages");
          queue = queue
            .then(async () => {
              const frame = decodeSignalFrame(
                new TextDecoder("utf-8", { fatal: true }).decode(body),
                "client",
              );
              if (!connection.frames.length) {
                verifyHostHello(frame, challenge.nonce, room);
                await manifestDigest(Uint8Array.from(frame.manifest));
              } else
                assert.notEqual(
                  frame.type,
                  "hello",
                  "host refuses a second hello",
                );
              connection.frames.push(frame);
              await handler(connection, frame);
              this.emit("frame", connection, frame);
            })
            .catch((error) => {
              this.failures.push(error);
              this.emit("failure", error);
              socket.destroy();
            });
        }
      };
      socket.on("data", (data) => {
        buffer = Buffer.concat([buffer, data]);
        consume();
      });
      if (challenge) connection.send(challenge);
      consume();
    });
    this.server.listen(0, "127.0.0.1");
    await once(this.server, "listening");
    t.after(async () => {
      for (const socket of this.connections) socket.destroy();
      await new Promise((resolve) => this.server.close(resolve));
      assert.deepEqual(this.failures, [], "stand-in host protocol failures");
    });
    return `ws://127.0.0.1:${this.server.address().port}/`;
  }
  sendText(socket, text, opcode = 1) {
    const bytes = Buffer.from(text);
    let header;
    if (bytes.length < 126) header = Buffer.from([0x80 | opcode, bytes.length]);
    else if (bytes.length <= 65535) {
      header = Buffer.alloc(4);
      header[0] = 0x80 | opcode;
      header[1] = 126;
      header.writeUInt16BE(bytes.length, 2);
    } else {
      header = Buffer.alloc(10);
      header[0] = 0x80 | opcode;
      header[1] = 127;
      header.writeBigUInt64BE(BigInt(bytes.length), 2);
    }
    socket.write(Buffer.concat([header, bytes]));
  }
}
