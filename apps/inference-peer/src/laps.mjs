import { dealDigest, decodeActivation } from "./deal.mjs";
import {
  signalBytes,
  signalHex,
  validateLapReceipt,
  validateSignalFrame,
} from "./signal.mjs";

const encoder = new TextEncoder();
const U64_MAX = (1n << 64n) - 1n;
const MAX_DEAL_PEERS = 32;
const MAX_PENDING_BYTES = 64 * 1024 * 1024;

function concat(...parts) {
  return concatenate(parts);
}
function concatenate(parts) {
  const result = new Uint8Array(
    parts.reduce((n, part) => n + part.length, 0),
  );
  let offset = 0;
  for (const part of parts) {
    result.set(part, offset);
    offset += part.length;
  }
  return result;
}
function unsigned(value, length) {
  const result = new Uint8Array(length);
  let n = BigInt(value);
  if (n < 0n || n >= 1n << BigInt(length * 8))
    throw new RangeError("Lap integer overflow");
  for (let i = length - 1; i >= 0; i--) {
    result[i] = Number(n & 255n);
    n >>= 8n;
  }
  return result;
}
async function hash(...parts) {
  return new Uint8Array(
    await crypto.subtle.digest("SHA-256", concat(...parts)),
  );
}
function nonzero(value) {
  return value.some((byte) => byte !== 0);
}
function canonicalReceipt(receipt) {
  const ranges = receipt.layer_range_digests;
  if (
    !nonzero(receipt.deal_id) ||
    !ranges.length ||
    ranges.length > MAX_DEAL_PEERS ||
    ranges.some((digest) => !nonzero(digest)) ||
    BigInt(receipt.prompt_tokens) + BigInt(receipt.generated_tokens) >
      BigInt(receipt.cumulative_tokens) ||
    !nonzero(receipt.host_signature) ||
    (ranges.length < 32 &&
      BigInt(receipt.peer_countersignatures) >> BigInt(ranges.length))
  )
    throw new Error("noncanonical_receipt");
}

// LapReceipt::encode: big-endian counters, u8 range count, signature, u32 bitmap.
export function lapReceiptBytes(receipt, { signing = false } = {}) {
  validateLapReceipt(receipt);
  canonicalReceipt(receipt);
  return concat(
    Uint8Array.from(receipt.deal_id),
    ...[
      "lap_index",
      "prompt_tokens",
      "generated_tokens",
      "cumulative_tokens",
    ].map((key) => unsigned(receipt[key], 8)),
    Uint8Array.of(receipt.layer_range_digests.length),
    ...receipt.layer_range_digests.map((digest) => Uint8Array.from(digest)),
    signing ? new Uint8Array(64) : Uint8Array.from(receipt.host_signature),
    unsigned(receipt.peer_countersignatures, 4),
  );
}
export function lapSigningDigest(receipt) {
  return hash(
    encoder.encode("PAXAI/lap-signing/v1\0"),
    lapReceiptBytes(receipt, { signing: true }),
  );
}
export function lapReceiptDigest(receipt) {
  return hash(
    encoder.encode("PAXAI/lap-receipt/v1\0"),
    lapReceiptBytes(receipt),
  );
}
export async function lapCountersignDigest(lap) {
  validateSignalFrame({ type: "lap_receipt", lap });
  canonicalReceipt(lap.receipt);
  if (lap.receipt.peer_countersignatures !== 0)
    throw new Error("issued_bitmap_not_empty");
  const key = await crypto.subtle.importKey(
    "raw",
    Uint8Array.from(lap.host_meter_key),
    "Ed25519",
    false,
    ["verify"],
  );
  if (
    !(await crypto.subtle.verify(
      "Ed25519",
      key,
      Uint8Array.from(lap.receipt.host_signature),
      await lapSigningDigest(lap.receipt),
    ))
  )
    throw new Error("host_signature_invalid");
  return hash(
    encoder.encode("PAXAI/lap-countersign/v1\0"),
    Uint8Array.from(lap.deal_digest),
    Uint8Array.from(lap.host_meter_key),
    unsigned(lap.deadline_ms, 8),
    await lapReceiptDigest(lap.receipt),
  );
}

// Refusals are local results. The host defines no refusal frame on the socket.
function refusal(reason) {
  return { type: "refusal", reason };
}

export class LapTracker {
  #identity;
  #deal;
  #host;
  #index;
  #binding;
  #nextLap = 0n;
  #cumulative = 0n;
  #pending = [];
  #pendingBytes = 0;
  #positions = new Map();
  #last;
  #queue = Promise.resolve();

  constructor({ identity, deal, hostMeterKey }) {
    if (typeof identity?.sign !== "function")
      throw new TypeError("Lap tracker requires an identity signer");
    validateSignalFrame({ type: "deal", deal });
    this.#identity = identity;
    this.#deal = structuredClone(deal);
    this.#host = signalHex(hostMeterKey);
    if (!nonzero(signalBytes(this.#host)))
      throw new TypeError("Lap tracker requires a nonzero host meter key");
    this.#index = deal.plan.layers.findIndex(
      (split) => signalHex(split.peer_key) === signalHex(identity.peerKey),
    );
    if (this.#index < 0 || deal.plan.layers.length > MAX_DEAL_PEERS)
      throw new Error("Peer is not assigned to the lap deal");
    this.#binding = dealDigest(this.#deal);
  }
  get nextLap() {
    return this.#nextLap;
  }
  get cumulativeTokens() {
    return this.#cumulative;
  }
  get pendingActivations() {
    return this.#pending.length;
  }

  #enqueue(work) {
    const result = this.#queue.then(work);
    this.#queue = result.catch(() => {});
    return result;
  }
  async validateDeal(deal) {
    if (signalHex(await dealDigest(deal)) !== signalHex(await this.#binding))
      throw new Error("Lap tracker deal differs");
  }
  recordActivation(payload) {
    if (!(payload instanceof Uint8Array))
      throw new TypeError("Lap activation must be encoded bytes");
    const bytes = Uint8Array.from(payload);
    return this.#enqueue(async () => {
      const activation = decodeActivation(bytes);
      if (
        signalHex(activation.digest) !== signalHex(await this.#binding) ||
        activation.layer !== this.#deal.plan.layers[this.#index].end_layer
      )
        throw new Error("Activation does not match lap range");
      const expected = this.#positions.get(activation.requestId) ?? 0;
      if (activation.position !== expected)
        throw new Error("Lap activation position is not continuous");
      if (
        (!this.#positions.has(activation.requestId) &&
          this.#positions.size >= 64) ||
        this.#pendingBytes + bytes.length > MAX_PENDING_BYTES
      )
        throw new Error("Lap activation buffer limit reached");
      this.#positions.set(activation.requestId, expected + 1);
      this.#pending.push(bytes);
      this.#pendingBytes += bytes.length;
    });
  }
  countersign(frame) {
    const copy = structuredClone(frame);
    return this.#enqueue(() => this.#countersign(copy));
  }
  async #countersign(frame) {
    try {
      validateSignalFrame(frame);
    } catch (error) {
      return refusal(`malformed_receipt: ${error.message}`);
    }
    if (frame.type !== "lap_receipt") return refusal("expected_lap_receipt");
    const { lap } = frame,
      receipt = lap.receipt;
    if (
      signalHex(lap.deal_digest) !== signalHex(await this.#binding) ||
      signalHex(receipt.deal_id) !== signalHex(this.#deal.deal_id)
    )
      return refusal("deal_mismatch");
    if (signalHex(lap.host_meter_key) !== this.#host)
      return refusal("host_meter_key_mismatch");
    if (BigInt(lap.deadline_ms) <= BigInt(Date.now()))
      return refusal("deadline_expired");
    let digest;
    try {
      digest = await lapCountersignDigest(lap);
    } catch (error) {
      return refusal(error.message);
    }
    const index = BigInt(receipt.lap_index);
    if (this.#last && index === this.#nextLap - 1n) {
      if (signalHex(digest) !== this.#last.digest)
        return refusal("receipt_replay_conflict");
      return structuredClone(this.#last.frame);
    }
    if (index !== this.#nextLap) return refusal("lap_index_gap");
    if (BigInt(receipt.generated_tokens) === 0n)
      return refusal("generated_tokens_invalid");
    if (index !== 0n && BigInt(receipt.prompt_tokens) !== 0n)
      return refusal("prompt_tokens_invalid");
    const count =
      BigInt(receipt.prompt_tokens) + BigInt(receipt.generated_tokens);
    const cumulative = this.#cumulative + count;
    if (
      cumulative > U64_MAX ||
      BigInt(receipt.cumulative_tokens) !== cumulative
    )
      return refusal("cumulative_tokens_mismatch");
    if (receipt.layer_range_digests.length !== this.#deal.plan.layers.length)
      return refusal("range_count_mismatch");
    if (count > BigInt(this.#pending.length))
      return refusal("unprocessed_activations");
    const processed = this.#pending.slice(0, Number(count));
    const rangeDigest = await hash(concatenate(processed));
    if (
      signalHex(rangeDigest) !==
      signalHex(receipt.layer_range_digests[this.#index])
    )
      return refusal("range_digest_mismatch");
    const signature = signalBytes(await this.#identity.sign(digest), 64);
    // The deadline can pass while the signer is working; do not consume evidence.
    if (BigInt(lap.deadline_ms) <= BigInt(Date.now()))
      return refusal("deadline_expired");
    const ack = validateSignalFrame(
      {
        type: "lap_countersign",
        deal_id: receipt.deal_id.slice(),
        lap_index: receipt.lap_index,
        digest: Array.from(digest),
        signature,
      },
      "client",
    );
    this.#pending.splice(0, Number(count));
    this.#pendingBytes -= processed.reduce((n, bytes) => n + bytes.length, 0);
    this.#nextLap++;
    this.#cumulative = cumulative;
    this.#last = { digest: signalHex(digest), frame: structuredClone(ack) };
    return ack;
  }
}

export function countersignLap(frame, tracker) {
  if (!(tracker instanceof LapTracker))
    throw new TypeError("A LapTracker is required");
  return tracker.countersign(frame);
}
