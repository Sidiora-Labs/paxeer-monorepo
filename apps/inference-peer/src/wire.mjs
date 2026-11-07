export const MAX_FRAME_BYTES = 65536;
export const FRAME_KINDS = Object.freeze({
  hello: 1,
  offer: 2,
  deal: 3,
  "deal-ack": 4,
  activation: 5,
  "lap-receipt": 6,
  countersign: 7,
  "canary-answer": 8,
  bye: 9,
});

const kindNames = new Map(Object.entries(FRAME_KINDS).map(([k, v]) => [v, k]));
const encoder = new TextEncoder();
const decoder = new TextDecoder("utf-8", { fatal: true, ignoreBOM: true });
const HEADER_BYTES = 6;

function bytesOf(value) {
  if (value instanceof ArrayBuffer) return new Uint8Array(value);
  if (ArrayBuffer.isView(value)) {
    return new Uint8Array(value.buffer, value.byteOffset, value.byteLength);
  }
  throw new TypeError("Expected a binary buffer");
}

function validateRoom(roomId) {
  if (typeof roomId !== "string" || roomId.length === 0) {
    throw new TypeError("roomId must be a nonempty string");
  }
}

function validatePayload(payload) {
  if (
    payload === null ||
    typeof payload !== "object" ||
    Array.isArray(payload)
  ) {
    throw new TypeError("Control payload must be a JSON object");
  }
}

// Network byte order: u16 kind, u32 body length; body: u16 room length,
// UTF-8 room id, u32 sequence, then JSON control bytes or raw activation bytes.
export function encodeFrame({ kind, roomId, sequence, payload }) {
  if (!Object.hasOwn(FRAME_KINDS, kind))
    throw new RangeError("Unknown frame kind");
  validateRoom(roomId);
  if (!Number.isInteger(sequence) || sequence < 0 || sequence > 0xffffffff) {
    throw new RangeError("sequence must be a u32");
  }
  const room = encoder.encode(roomId);
  if (decoder.decode(room) !== roomId)
    throw new TypeError("Invalid room Unicode");
  let data;
  if (kind === "activation") {
    data = bytesOf(payload);
  } else {
    validatePayload(payload);
    data = encoder.encode(JSON.stringify(payload));
  }
  const length = 6 + room.byteLength + data.byteLength;
  if (HEADER_BYTES + length > MAX_FRAME_BYTES) {
    throw new RangeError("Frame exceeds 65536 bytes");
  }
  const out = new Uint8Array(HEADER_BYTES + length);
  const view = new DataView(out.buffer);
  view.setUint16(0, FRAME_KINDS[kind]);
  view.setUint32(2, length);
  view.setUint16(6, room.byteLength);
  out.set(room, 8);
  view.setUint32(8 + room.byteLength, sequence);
  out.set(data, 12 + room.byteLength);
  return out;
}

export function decodeFrame(input) {
  const bytes = bytesOf(input);
  if (bytes.byteLength > MAX_FRAME_BYTES)
    throw new RangeError("Frame exceeds 65536 bytes");
  if (bytes.byteLength < 12) throw new RangeError("Truncated frame");
  const view = new DataView(bytes.buffer, bytes.byteOffset, bytes.byteLength);
  const kind = kindNames.get(view.getUint16(0));
  if (!kind) throw new RangeError("Unknown frame kind");
  if (view.getUint32(2) !== bytes.byteLength - HEADER_BYTES) {
    throw new RangeError("Frame length mismatch");
  }
  const roomLength = view.getUint16(6);
  const payloadOffset = 12 + roomLength;
  if (payloadOffset > bytes.byteLength)
    throw new RangeError("Truncated room id");
  const roomId = decoder.decode(bytes.subarray(8, 8 + roomLength));
  validateRoom(roomId);
  const sequence = view.getUint32(8 + roomLength);
  const data = bytes.subarray(payloadOffset);
  const payload =
    kind === "activation" ? data.slice() : JSON.parse(decoder.decode(data));
  if (kind !== "activation") validatePayload(payload);
  return { kind, roomId, sequence, payload };
}
