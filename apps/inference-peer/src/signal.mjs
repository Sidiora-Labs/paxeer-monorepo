// JSON signalling types mirror inference_market/{signal,deals,laps}.rs.
export const MAX_SIGNAL_FRAME_BYTES = 65536;
const encoder = new TextEncoder();
const CLIENT_TYPES = new Set([
  "hello",
  "description",
  "candidate",
  "deal_ack",
  "lap_countersign",
]);
const SERVER_TYPES = new Set([
  "challenge",
  "offer",
  "description",
  "candidate",
  "deal",
  "deal_live",
  "lap_receipt",
  "error",
]);
const ERROR_CODES = new Set([
  "bad_signature",
  "wrong_challenge",
  "invalid_manifest",
  "invalid_offer",
  "chain_unavailable",
  "unenrolled",
  "manifest_mismatch",
  "offer_mismatch",
  "duplicate_peer",
  "unknown_peer",
  "invalid_frame",
  "frame_too_large",
  "degraded",
  "backpressure",
]);

function object(value, fields) {
  if (
    !value ||
    typeof value !== "object" ||
    Array.isArray(value) ||
    Object.keys(value).some((key) => !fields.includes(key))
  )
    throw new TypeError("Unknown member in signalling frame");
  for (const key of fields)
    if (!Object.hasOwn(value, key))
      throw new TypeError(`Missing member ${key}`);
}
function bytes(value, length) {
  if (
    !Array.isArray(value) ||
    (length !== undefined && value.length !== length) ||
    !Array.from(value).every((n) => Number.isInteger(n) && n >= 0 && n <= 255)
  )
    throw new TypeError(`Expected ${length ?? "variable-length"} byte array`);
}
function uint(value, bits) {
  if (
    (typeof value !== "number" ||
      !Number.isSafeInteger(value) ||
      value < 0) &&
    (bits !== 64 || typeof value !== "bigint" || value < 0n)
  )
    throw new TypeError(`Expected u${bits}`);
  if (BigInt(value) >= 1n << BigInt(bits))
    throw new RangeError(`Overflow in u${bits}`);
}
function string(value) {
  if (typeof value !== "string")
    throw new TypeError("Expected signalling string");
}
function price(value) {
  string(value);
  if (!/^(0|[1-9][0-9]*)$/.test(value) || BigInt(value) >= 1n << 128n)
    throw new TypeError("Invalid canonical u128 price");
}
function list(value, validate) {
  if (!Array.isArray(value)) throw new TypeError("Expected signalling list");
  value.forEach(validate);
}
function description(value) {
  object(value, ["type", "sdp"]);
  if (!["offer", "answer", "pranswer", "rollback"].includes(value.type))
    throw new TypeError("Invalid SDP type");
  string(value.sdp);
}
function candidate(value) {
  object(value, ["candidate", "sdp_mid", "sdp_m_line_index"]);
  string(value.candidate);
  if (value.sdp_mid !== null) string(value.sdp_mid);
  if (value.sdp_m_line_index !== null) uint(value.sdp_m_line_index, 16);
}
function assignment(value) {
  object(value, ["peer_key", "first_layer", "end_layer"]);
  bytes(value.peer_key, 32);
  uint(value.first_layer, 32);
  uint(value.end_layer, 32);
}
function offer(value) {
  object(value, [
    "room_id",
    "model_digest",
    "runtime_digest",
    "layer_plan",
    "price",
  ]);
  for (const key of ["room_id", "model_digest", "runtime_digest"])
    bytes(value[key], 32);
  list(value.layer_plan, assignment);
  price(value.price);
}
function plan(value) {
  object(value, ["embedding_host", "head_host", "layer_count", "layers"]);
  bytes(value.embedding_host, 32);
  bytes(value.head_host, 32);
  uint(value.layer_count, 16);
  list(value.layers, (split) => {
    object(split, [
      "peer_key",
      "owner",
      "bid_id",
      "manifest_digest",
      "first_layer",
      "end_layer",
    ]);
    for (const key of ["peer_key", "owner", "bid_id", "manifest_digest"])
      bytes(split[key], 32);
    uint(split.first_layer, 16);
    uint(split.end_layer, 16);
  });
}
function deal(value) {
  const digests = [
    "chain",
    "program",
    "market",
    "room_id",
    "model_digest",
    "runtime_digest",
    "deal_id",
    "ask_id",
    "roster_digest",
  ];
  object(value, [...digests, "epoch", "config", "round", "price", "plan"]);
  for (const key of digests) bytes(value[key], 32);
  for (const key of ["epoch", "config", "round"]) uint(value[key], 64);
  price(value.price);
  plan(value.plan);
}

function lap(value) {
  object(value, ["deal_digest", "host_meter_key", "deadline_ms", "receipt"]);
  bytes(value.deal_digest, 32);
  bytes(value.host_meter_key, 32);
  uint(value.deadline_ms, 64);
  validateLapReceipt(value.receipt);
}
export function validateLapReceipt(receipt) {
  object(receipt, [
    "deal_id",
    "lap_index",
    "prompt_tokens",
    "generated_tokens",
    "cumulative_tokens",
    "layer_range_digests",
    "host_signature",
    "peer_countersignatures",
  ]);
  bytes(receipt.deal_id, 32);
  for (const key of [
    "lap_index",
    "prompt_tokens",
    "generated_tokens",
    "cumulative_tokens",
  ])
    uint(receipt[key], 64);
  list(receipt.layer_range_digests, (digest) => bytes(digest, 32));
  bytes(receipt.host_signature, 64);
  uint(receipt.peer_countersignatures, 32);
  return receipt;
}

export function validateSignalFrame(frame, direction = "server") {
  const types =
    direction === "client"
      ? CLIENT_TYPES
      : direction === "server"
        ? SERVER_TYPES
        : null;
  if (!types) throw new TypeError("Unknown signalling direction");
  if (!types.has(frame?.type)) throw new TypeError("Unknown signalling type");
  switch (frame.type) {
    case "hello":
      object(frame, [
        "type",
        "room_id",
        "peer_key",
        "nonce",
        "signature",
        "manifest",
      ]);
      for (const key of ["room_id", "peer_key", "nonce"])
        bytes(frame[key], 32);
      bytes(frame.signature, 64);
      bytes(frame.manifest);
      break;
    case "challenge":
      object(frame, ["type", "nonce"]);
      bytes(frame.nonce, 32);
      break;
    case "offer":
      object(frame, ["type", "offer"]);
      offer(frame.offer);
      break;
    case "description":
    case "candidate": {
      const peer = direction === "client" ? "to" : "from";
      object(frame, ["type", peer, frame.type]);
      bytes(frame[peer], 32);
      if (frame.type === "description") description(frame.description);
      else candidate(frame.candidate);
      break;
    }
    case "deal":
      object(frame, ["type", "deal"]);
      deal(frame.deal);
      break;
    case "deal_ack":
      object(frame, ["type", "deal_id", "digest", "signature"]);
      bytes(frame.deal_id, 32);
      bytes(frame.digest, 32);
      bytes(frame.signature, 64);
      break;
    case "deal_live":
      object(frame, ["type", "deal_id"]);
      bytes(frame.deal_id, 32);
      break;
    case "lap_receipt":
      object(frame, ["type", "lap"]);
      lap(frame.lap);
      break;
    case "lap_countersign":
      object(frame, ["type", "deal_id", "lap_index", "digest", "signature"]);
      bytes(frame.deal_id, 32);
      uint(frame.lap_index, 64);
      bytes(frame.digest, 32);
      bytes(frame.signature, 64);
      break;
    case "error":
      object(frame, ["type", "code"]);
      if (!ERROR_CODES.has(frame.code))
        throw new TypeError("Unknown signal error code");
      break;
  }
  return frame;
}

// Preserve the Rust u64 fields without rounding through a JavaScript number.
function json(value) {
  if (typeof value === "bigint") return value.toString();
  if (Array.isArray(value)) return `[${value.map(json).join(",")}]`;
  if (value && typeof value === "object")
    return `{${Object.entries(value)
      .map(([key, item]) => `${JSON.stringify(key)}:${json(item)}`)
      .join(",")}}`;
  return JSON.stringify(value);
}
function bounded(text) {
  if (typeof text !== "string")
    throw new TypeError(
      "Signalling requires a text frame; binary frame refused",
    );
  if (encoder.encode(text).length > MAX_SIGNAL_FRAME_BYTES)
    throw new RangeError("Signalling frame exceeds 65536 bytes");
  return text;
}
export function encodeSignalFrame(frame, direction = "client") {
  return bounded(json(validateSignalFrame(frame, direction)));
}
export function decodeSignalFrame(text, direction = "server") {
  bounded(text);
  const frame = JSON.parse(text, (key, value, context) => {
    if (
      typeof value === "number" &&
      !Number.isSafeInteger(value) &&
      /^\d+$/.test(context.source)
    )
      return BigInt(context.source);
    return value;
  });
  return validateSignalFrame(frame, direction);
}

// Hex is an application identity representation, never a signalling member.
export function signalBytes(value, length = 32) {
  if (
    typeof value === "string" &&
    new RegExp(`^[0-9a-f]{${length * 2}}$`).test(value)
  )
    return Array.from(value.match(/../g), (byte) => parseInt(byte, 16));
  const result = value instanceof Uint8Array ? Array.from(value) : value;
  bytes(result, length);
  return result.slice();
}
export function signalHex(value) {
  return signalBytes(value)
    .map((byte) => byte.toString(16).padStart(2, "0"))
    .join("");
}
