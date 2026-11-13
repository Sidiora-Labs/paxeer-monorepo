import {
  signalBytes,
  signalHex,
  validateSignalFrame,
  validateCanaryPlan,
} from "./signal.mjs";

export { validateCanaryPlan } from "./signal.mjs";

const encoder = new TextEncoder();
const TOP_LOGITS = 8;
const FRACTION = 65536;
const MAX_POSITIONS = 8192;

function u64(value) {
  if (
    !(
      (typeof value === "number" && Number.isSafeInteger(value)) ||
      typeof value === "bigint"
    ) ||
    value < 0 ||
    BigInt(value) >= 1n << 64n
  )
    throw new RangeError("Invalid canary u64");
  return BigInt(value);
}
function integerBytes(value, size) {
  const bytes = new Uint8Array(size);
  let n = u64(value);
  for (let i = size - 1; i >= 0; i--) {
    bytes[i] = Number(n & 255n);
    n >>= 8n;
  }
  return bytes;
}
function concat(...parts) {
  const bytes = new Uint8Array(
    parts.reduce((n, part) => n + part.length, 0),
  );
  let offset = 0;
  for (const part of parts) {
    bytes.set(part, offset);
    offset += part.length;
  }
  return bytes;
}
async function domainHash(domain, ...parts) {
  return new Uint8Array(
    await crypto.subtle.digest(
      "SHA-256",
      concat(encoder.encode(domain + "\0"), ...parts),
    ),
  );
}
function equal(a, b) {
  return a.length === b.length && a.every((byte, i) => byte === b[i]);
}

// Program canary::encode_logits: eight descending Q16 i32s, big endian.
// Quantization rounds half away from zero; overflow is refused, never clamped.
export function encodeFixedPointLogits(logits) {
  if (
    !(
      Array.isArray(logits) ||
      logits instanceof Float32Array ||
      logits instanceof Float64Array
    ) ||
    logits.length < TOP_LOGITS
  )
    throw new TypeError("Canary requires at least eight logits");
  const top = [];
  for (const value of logits) {
    if (typeof value !== "number" || !Number.isFinite(value))
      throw new TypeError("Canary requires finite logits");
    let index = 0;
    while (index < top.length && top[index] >= value) index++;
    if (index < TOP_LOGITS) {
      top.splice(index, 0, value);
      if (top.length > TOP_LOGITS) top.pop();
    }
  }
  const bytes = new Uint8Array(32);
  const view = new DataView(bytes.buffer);
  for (const [i, value] of top.entries()) {
    const fixed =
      Math.sign(value) * Math.round(Math.abs(value) * FRACTION);
    if (
      !Number.isSafeInteger(fixed) ||
      fixed < -2147483648 ||
      fixed > 2147483647
    )
      throw new RangeError("Canary Q16 i32 overflow");
    view.setInt32(i * 4, fixed, false);
  }
  return bytes;
}

function refusal(reason) {
  return { type: "refusal", reason };
}

// A record must contain actual model-head logits. Hidden activations are not
// logits and cannot be recorded here as a substitute for the host's head.
export class CanaryTracker {
  #identity;
  #deal;
  #positions = new Map();
  #closed = false;

  constructor({ identity, deal }) {
    if (typeof identity?.sign !== "function")
      throw new TypeError("Canary tracker requires an identity signer");
    validateSignalFrame({ type: "deal", deal });
    const key = signalHex(identity.peerKey);
    if (
      !deal.plan.layers.some(
        (split) => signalHex(split.peer_key) === key,
      )
    )
      throw new Error("Peer is not assigned to the canary deal");
    this.#identity = identity;
    this.#deal = structuredClone(deal);
  }

  get peerKey() {
    return signalHex(this.#identity.peerKey);
  }

  recordLogits({ lapIndex, position, logits }) {
    if (this.#closed) throw new Error("Canary tracker is closed");
    const key = `${u64(lapIndex)}:${u64(position)}`;
    const encoded = encodeFixedPointLogits(logits);
    const previous = this.#positions.get(key);
    if (previous) {
      if (!equal(previous, encoded))
        throw new Error("Conflicting canary logits");
      return;
    }
    if (this.#positions.size >= MAX_POSITIONS)
      throw new RangeError("Canary position buffer limit reached");
    this.#positions.set(key, encoded);
  }

  async answer(plan) {
    if (this.#closed) return refusal("tracker_closed");
    try {
      plan = structuredClone(validateCanaryPlan(plan));
    } catch (error) {
      return refusal(`malformed_plan: ${error.message}`);
    }
    if (plan.position === null) return refusal("no_canary_position");
    if (signalHex(plan.deal_id) !== signalHex(this.#deal.deal_id))
      return refusal("deal_mismatch");
    if (signalHex(plan.peer_key) !== signalHex(this.#identity.peerKey))
      return refusal("peer_mismatch");
    const context = Uint8Array.from(plan.answer_context);
    const expected = concat(
      ...["chain", "program", "market"].map((key) =>
        Uint8Array.from(this.#deal[key]),
      ),
      integerBytes(this.#deal.epoch, 8),
      integerBytes(this.#deal.config, 8),
      Uint8Array.from(this.#deal.roster_digest),
      Uint8Array.from(plan.deal_id),
      integerBytes(plan.lap_index, 8),
    );
    if (!equal(context.subarray(0, 184), expected))
      return refusal("answer_context_mismatch");
    const promptDigest = await domainHash(
      "PAXAI/canary-prompt/v1",
      Uint8Array.from(this.#deal.model_digest),
      integerBytes(plan.position, 8),
      ...plan.prompt_tokens.map((token) => integerBytes(token, 4)),
    );
    if (!equal(context.subarray(216), promptDigest))
      return refusal("prompt_mismatch");
    const bytes = this.#positions.get(
      `${u64(plan.lap_index)}:${u64(plan.position)}`,
    );
    if (!bytes) return refusal("unknown_position");
    const logitsDigest = await domainHash(
      "PAXAI/canary-logits/q16/v1",
      bytes,
    );
    const digest = await domainHash(
      "PAXAI/canary-answer/v1",
      context,
      logitsDigest,
    );
    const signature = signalBytes(
      await this.#identity.sign(digest),
      64,
    );
    if (this.#closed) return refusal("tracker_closed");
    const view = new DataView(
      bytes.buffer,
      bytes.byteOffset,
      bytes.byteLength,
    );
    return {
      deal_id: plan.deal_id.slice(),
      lap_index: plan.lap_index,
      logits: Array.from({ length: TOP_LOGITS }, (_, i) =>
        view.getInt32(i * 4, false),
      ),
      signature,
    };
  }

  close() {
    this.#closed = true;
    this.#positions.clear();
  }
}

export function canaryAnswer(plan, tracker) {
  if (!(tracker instanceof CanaryTracker))
    throw new TypeError("A CanaryTracker is required");
  return tracker.answer(plan);
}
