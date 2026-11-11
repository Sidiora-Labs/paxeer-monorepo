import { MAX_FRAME_BYTES } from "./wire.mjs";
import { signalHex, validateSignalFrame } from "./signal.mjs";

const encoder = new TextEncoder();
const decoder = new TextDecoder("utf-8", { fatal: true });

function hex(bytes) {
  return Array.from(bytes, (b) => b.toString(16).padStart(2, "0")).join("");
}

function digestBytes(value) {
  if (typeof value === "string" && /^[0-9a-f]{64}$/.test(value))
    return Uint8Array.from(value.match(/../g), (b) => parseInt(b, 16));
  if (
    (Array.isArray(value) || value instanceof Uint8Array) &&
    value.length === 32 &&
    Array.from(value).every((b) => Number.isInteger(b) && b >= 0 && b <= 255)
  )
    return Uint8Array.from(value);
  throw new TypeError("Expected a 32-byte digest or lowercase hex");
}

function integer(value, max, name) {
  if (!Number.isSafeInteger(value) || value < 0 || value > max)
    throw new RangeError(`Invalid ${name}`);
  return value;
}

function concat(...parts) {
  const out = new Uint8Array(parts.reduce((n, part) => n + part.length, 0));
  let offset = 0;
  for (const part of parts) {
    out.set(part, offset);
    offset += part.length;
  }
  return out;
}

function unsigned(value, size) {
  if (typeof value === "number" && !Number.isSafeInteger(value))
    throw new RangeError("Unsafe integer in deal");
  if (!/^(0|[1-9][0-9]*)$/.test(String(value)))
    throw new TypeError("Noncanonical deal integer");
  let n = BigInt(value);
  if (n >= 1n << BigInt(size * 8))
    throw new RangeError("Deal integer overflow");
  const out = new Uint8Array(size);
  for (let i = size - 1; i >= 0; i--) {
    out[i] = Number(n & 255n);
    n >>= 8n;
  }
  return out;
}

async function sha256(bytes) {
  return new Uint8Array(await crypto.subtle.digest("SHA-256", bytes));
}

function validatePlan(plan) {
  if (
    !plan ||
    hex(digestBytes(plan.embedding_host)) !== hex(digestBytes(plan.head_host))
  )
    throw new Error("Embedding and head must stay on the host");
  integer(plan.layer_count, 65535, "layer count");
  if (
    !plan.layer_count ||
    !Array.isArray(plan.layers) ||
    !plan.layers.length ||
    plan.layers.length > 64
  )
    throw new Error("Invalid layer plan");
  const peers = new Set(),
    owners = new Set(),
    bids = new Set();
  let end = 0;
  for (const split of plan.layers) {
    for (const name of ["peer_key", "owner", "bid_id", "manifest_digest"])
      if (!digestBytes(split[name]).some(Boolean))
        throw new Error("Zero plan identity");
    const peer = hex(digestBytes(split.peer_key)),
      owner = hex(digestBytes(split.owner)),
      bid = hex(digestBytes(split.bid_id));
    if (
      peers.has(peer) ||
      owners.has(owner) ||
      bids.has(bid) ||
      owner === hex(digestBytes(plan.embedding_host))
    )
      throw new Error("Duplicate or host-owned layer assignment");
    peers.add(peer);
    owners.add(owner);
    bids.add(bid);
    integer(split.end_layer, plan.layer_count, "end layer");
    if (split.first_layer !== end || split.end_layer <= end)
      throw new Error("Layer plan must be contiguous");
    end = split.end_layer;
  }
  if (end !== plan.layer_count) throw new Error("Layer plan is incomplete");
}

export async function dealDigest(deal) {
  validatePlan(deal.plan);
  if (BigInt(deal.price) <= 0n)
    throw new Error("Deal price must be positive");
  const fields = [encoder.encode("PAXAI/inference-deal/v1\0")];
  for (const name of [
    "chain",
    "program",
    "market",
    "room_id",
    "model_digest",
    "runtime_digest",
    "deal_id",
    "ask_id",
    "roster_digest",
  ])
    fields.push(digestBytes(deal[name]));
  for (const name of ["epoch", "config", "round"])
    fields.push(unsigned(deal[name], 8));
  fields.push(
    unsigned(deal.price, 16),
    digestBytes(deal.plan.embedding_host),
    digestBytes(deal.plan.head_host),
    unsigned(deal.plan.layer_count, 2),
    unsigned(deal.plan.layers.length, 2),
  );
  for (const split of deal.plan.layers) {
    for (const name of ["peer_key", "owner", "bid_id", "manifest_digest"])
      fields.push(digestBytes(split[name]));
    fields.push(unsigned(split.first_layer, 2), unsigned(split.end_layer, 2));
  }
  return sha256(concat(...fields));
}

function sourceURL(value, base) {
  const url = new URL(value, base);
  if (
    url.username ||
    url.password ||
    url.hash ||
    !(
      url.protocol === "https:" ||
      (url.protocol === "http:" &&
        ["127.0.0.1", "[::1]"].includes(url.hostname))
    )
  )
    throw new Error("Model source requires HTTPS or HTTP loopback");
  return url;
}

async function download(url, limit, signal) {
  const response = await fetch(url, {
    redirect: "error",
    credentials: "omit",
    cache: "no-store",
    signal,
  });
  if (!response.ok) {
    await response.body?.cancel();
    throw new Error(`Model download refused (${response.status})`);
  }
  const reader = response.body.getReader(),
    chunks = [];
  let size = 0;
  try {
    for (;;) {
      const { value, done } = await reader.read();
      if (done) break;
      size += value.length;
      if (size > limit)
        throw new Error("Model download exceeds declared size");
      chunks.push(value);
    }
  } finally {
    await reader.cancel();
    reader.releaseLock();
  }
  return concat(...chunks);
}

export async function loadLayerRange({
  offer,
  firstLayer,
  endLayer,
  runtime,
  signal,
  modelSource,
}) {
  if (!modelSource?.manifest_url)
    throw new Error(
      "Model source must be configured locally; the host offer does not serve it",
    );
  const manifestURL = sourceURL(modelSource.manifest_url);
  const bytes = await download(manifestURL, 1048576, signal);
  if (hex(await sha256(bytes)) !== hex(digestBytes(offer.model_digest)))
    throw new Error("Model manifest digest mismatch");
  const manifest = JSON.parse(decoder.decode(bytes));
  integer(manifest.layer_count, 65535, "manifest layer count");
  integer(firstLayer, manifest.layer_count, "first layer");
  integer(endLayer, manifest.layer_count, "end layer");
  integer(manifest.hidden_size, 16384, "hidden size");
  if (
    manifest.version !== 1 ||
    !manifest.hidden_size ||
    firstLayer >= endLayer ||
    !Array.isArray(manifest.shards)
  )
    throw new Error("Invalid model digest manifest");
  let end = 0;
  const paths = new Set(),
    selected = [];
  for (const shard of manifest.shards) {
    integer(shard.end_layer, manifest.layer_count, "shard end layer");
    integer(shard.size, 0x7fffffff, "shard size");
    digestBytes(shard.sha256);
    if (
      shard.first_layer !== end ||
      shard.end_layer <= end ||
      !shard.size ||
      typeof shard.path !== "string" ||
      !shard.path.length
    )
      throw new Error("Shard manifest must cover every layer exactly once");
    end = shard.end_layer;
    const url = sourceURL(shard.path, manifestURL);
    if (url.origin !== manifestURL.origin || paths.has(url.href))
      throw new Error("Invalid shard source");
    paths.add(url.href);
    if (shard.end_layer <= firstLayer || shard.first_layer >= endLayer)
      continue;
    if (shard.first_layer < firstLayer || shard.end_layer > endLayer)
      throw new Error("Shard crosses assigned layer range");
    selected.push({ ...shard, url });
  }
  if (
    end !== manifest.layer_count ||
    selected[0]?.first_layer !== firstLayer ||
    selected.at(-1)?.end_layer !== endLayer
  )
    throw new Error("Incomplete shard manifest");
  const shards = [];
  for (const shard of selected) {
    const data = await download(shard.url, shard.size, signal);
    if (
      data.length !== shard.size ||
      hex(await sha256(data)) !== hex(digestBytes(shard.sha256))
    )
      throw new Error(`Shard digest mismatch: ${shard.path}`);
    shards.push({ ...shard, bytes: data });
  }
  const model = await runtime.loadLayerRange({
    manifest,
    shards,
    firstLayer,
    endLayer,
  });
  for (const name of ["createCache", "runActivation", "dropCache", "close"])
    if (typeof model?.[name] !== "function")
      throw new TypeError(`Runtime requires ${name}`);
  return { model, manifest };
}

export function encodeActivation({
  requestId,
  digest,
  position,
  layer,
  values,
}) {
  const id = encoder.encode(requestId);
  if (
    typeof requestId !== "string" ||
    !id.length ||
    id.length > 256 ||
    decoder.decode(id) !== requestId
  )
    throw new TypeError("Invalid activation request ID");
  integer(position, 0xffffffff, "token position");
  integer(layer, 65535, "activation layer");
  if (
    !(values instanceof Float32Array) ||
    !values.length ||
    !values.every(Number.isFinite)
  )
    throw new TypeError("Activation requires finite float32 values");
  const out = new Uint8Array(45 + id.length + values.length * 4);
  if (out.length > MAX_FRAME_BYTES - 12)
    throw new RangeError("Activation exceeds frame limit");
  const view = new DataView(out.buffer);
  view.setUint8(0, 1);
  view.setUint16(1, id.length);
  out.set(id, 3);
  let offset = 3 + id.length;
  out.set(digestBytes(digest), offset);
  offset += 32;
  view.setUint32(offset, position);
  offset += 4;
  view.setUint16(offset, layer);
  offset += 2;
  view.setUint32(offset, values.length);
  offset += 4;
  for (const value of values) {
    view.setFloat32(offset, value);
    offset += 4;
  }
  return out;
}

export function decodeActivation(bytes) {
  if (!(bytes instanceof Uint8Array) || bytes.length < 45)
    throw new Error("Truncated activation");
  const view = new DataView(bytes.buffer, bytes.byteOffset, bytes.byteLength);
  const length = view.getUint16(1);
  if (
    view.getUint8(0) !== 1 ||
    !length ||
    length > 256 ||
    bytes.length < 45 + length
  )
    throw new Error("Invalid activation header");
  const requestId = decoder.decode(bytes.subarray(3, 3 + length));
  let offset = 3 + length;
  const digest = bytes.slice(offset, offset + 32);
  offset += 32;
  const position = view.getUint32(offset);
  offset += 4;
  const layer = view.getUint16(offset);
  offset += 2;
  const count = view.getUint32(offset);
  offset += 4;
  if (!count || offset + count * 4 !== bytes.length)
    throw new Error("Invalid activation dimensions");
  const values = new Float32Array(count);
  for (let i = 0; i < count; i++) values[i] = view.getFloat32(offset + i * 4);
  if (!values.every(Number.isFinite)) throw new Error("Nonfinite activation");
  return { requestId, digest, position, layer, values };
}

export async function runActivation({ model, cache, values, position }) {
  const output = await model.runActivation(values, position, cache);
  if (
    !(output instanceof Float32Array) ||
    output.length !== values.length ||
    !output.every(Number.isFinite)
  )
    throw new Error("Runtime returned invalid activations");
  return output;
}

export class DealClient extends EventTarget {
  #requests = new Map();
  #ended = new Set();
  #queue = Promise.resolve();
  #abort = new AbortController();
  #listener;
  #closedListener;
  #model;
  #digest;
  #dealId;
  #range;
  #next;
  #previous;
  #state = "idle";

  constructor({
    roomId,
    identity,
    offer,
    runtime,
    mesh,
    hostPeerKey,
    acknowledge,
    validateDeal,
    modelSource,
    lapTracker,
    countersign,
  }) {
    super();
    if (
      typeof roomId !== "string" ||
      !roomId.length ||
      typeof identity?.sign !== "function" ||
      typeof runtime?.loadLayerRange !== "function" ||
      typeof mesh?.send !== "function" ||
      typeof acknowledge !== "function" ||
      typeof validateDeal !== "function"
    )
      throw new TypeError(
        "Deal client requires identity, offer, runtime, mesh and deal validation/acknowledgement callbacks",
      );
    digestBytes(identity.peerKey);
    digestBytes(hostPeerKey);
    if (
      lapTracker !== undefined &&
      (typeof lapTracker?.recordActivation !== "function" ||
        typeof lapTracker?.validateDeal !== "function" ||
        typeof lapTracker?.countersign !== "function" ||
        typeof countersign !== "function")
    )
      throw new TypeError(
        "Lap tracking requires a tracker and countersign callback",
      );
    this.roomId = roomId;
    this.identity = identity;
    this.offer = structuredClone(offer);
    this.runtime = runtime;
    this.modelSource = structuredClone(modelSource);
    this.mesh = mesh;
    this.hostPeerKey = hostPeerKey;
    this.lapTracker = lapTracker;
    this.countersign = countersign;
    this.acknowledge = acknowledge;
    this.validateDeal = validateDeal;
    this.#listener = ({ detail: { peerKey, frame } }) => {
      this.handleFrame(frame, peerKey).catch((error) => {
        this.close();
        this.dispatchEvent(new CustomEvent("error", { detail: error }));
      });
    };
    this.#closedListener = () => this.close();
    mesh.addEventListener("frame", this.#listener);
    mesh.addEventListener("close", this.#closedListener);
  }

  get state() {
    return this.#state;
  }
  get cacheCount() {
    return this.#requests.size;
  }
  get digest() {
    return this.#digest?.slice();
  }

  handleFrame(frame, peerKey = this.hostPeerKey) {
    const work = this.#queue.then(() => this.#handle(frame, peerKey));
    this.#queue = work.catch(() => {});
    return work;
  }

  async #handle(frame, peerKey) {
    if (this.#state === "closed") throw new Error("Deal client is closed");
    if (frame.type === "deal") {
      validateSignalFrame(frame);
      if (peerKey !== this.hostPeerKey || this.#state !== "idle")
        throw new Error("Unexpected deal");
      this.#state = "loading";
      try {
        const deal = structuredClone(frame.deal);
        const digest = await dealDigest(deal);
        await this.lapTracker?.validateDeal(deal);
        for (const name of ["model_digest", "runtime_digest"])
          if (
            hex(digestBytes(deal[name])) !==
            hex(digestBytes(this.offer[name]))
          )
            throw new Error("Deal differs from offer");
        if (hex(digestBytes(deal.room_id)) !== this.roomId)
          throw new Error("Deal room differs");
        if ((await this.validateDeal(deal)) !== true)
          throw new Error("Deal validation refused");
        const index = deal.plan.layers.findIndex(
          (split) =>
            hex(digestBytes(split.peer_key)) === this.identity.peerKey,
        );
        if (index < 0) throw new Error("Peer is not assigned to this deal");
        const split = deal.plan.layers[index];
        const loaded = await loadLayerRange({
          offer: this.offer,
          modelSource: this.modelSource,
          firstLayer: split.first_layer,
          endLayer: split.end_layer,
          runtime: this.runtime,
          signal: this.#abort.signal,
        });
        if (this.#abort.signal.aborted) {
          loaded.model.close();
          throw new Error("Deal client closed during loading");
        }
        this.#model = loaded.model;
        if (loaded.manifest.layer_count !== deal.plan.layer_count)
          throw new Error("Model layer count differs from deal");
        if (this.#abort.signal.aborted)
          throw new Error("Deal client closed during loading");
        const signature = await this.identity.sign(
          concat(encoder.encode("PAXAI/inference-deal-ack/v1\0"), digest),
        );
        if (this.#abort.signal.aborted)
          throw new Error("Deal client closed during signing");
        if (!/^[0-9a-f]{128}$/.test(signature))
          throw new Error("Invalid acknowledgement signature");
        await this.acknowledge({
          type: "deal_ack",
          deal_id: Array.from(digestBytes(deal.deal_id)),
          digest: Array.from(digest),
          signature: Array.from(
            Uint8Array.from(signature.match(/../g), (b) => parseInt(b, 16)),
          ),
        });
        if (this.#abort.signal.aborted)
          throw new Error("Deal client closed during acknowledgement");
        this.#digest = digest;
        this.#dealId = signalHex(deal.deal_id);
        this.#range = {
          first: split.first_layer,
          end: split.end_layer,
          width: loaded.manifest.hidden_size,
        };
        this.#previous =
          index === 0
            ? this.hostPeerKey
            : hex(digestBytes(deal.plan.layers[index - 1].peer_key));
        this.#next =
          index === deal.plan.layers.length - 1
            ? this.hostPeerKey
            : hex(digestBytes(deal.plan.layers[index + 1].peer_key));
        this.#state = "acknowledged";
      } catch (error) {
        this.close();
        throw error;
      }
      return;
    }
    if (frame.type === "deal_live") {
      validateSignalFrame(frame);
      if (
        peerKey !== this.hostPeerKey ||
        this.#state !== "acknowledged" ||
        signalHex(frame.deal_id) !== this.#dealId
      )
        throw new Error("Unexpected live deal");
      this.#state = "ready";
      this.dispatchEvent(
        new CustomEvent("ready", {
          detail: { digest: this.#digest.slice() },
        }),
      );
      return;
    }
    if (frame.type === "lap_receipt") {
      validateSignalFrame(frame);
      if (
        peerKey !== this.hostPeerKey ||
        this.#state !== "ready" ||
        !this.lapTracker
      )
        throw new Error("Unexpected lap receipt");
      const result = await this.lapTracker.countersign(frame);
      if (result.type === "refusal") {
        this.dispatchEvent(
          new CustomEvent("lap-refusal", { detail: result }),
        );
      } else {
        await this.countersign(result);
        this.dispatchEvent(new CustomEvent("lap", { detail: result }));
      }
      return result;
    }
    if (frame.roomId !== this.roomId) throw new Error("Wrong deal room");
    if (frame.kind === "bye") {
      if (peerKey !== this.hostPeerKey)
        throw new Error("Only host may end a request");
      const id = frame.payload.requestId;
      if (id === undefined) this.close();
      else {
        if (typeof id !== "string" || !id.length)
          throw new Error("Invalid bye request ID");
        const request = this.#requests.get(id);
        if (request) this.#model.dropCache(request.cache);
        this.#requests.delete(id);
        this.#ended.add(id);
      }
      this.dispatchEvent(
        new CustomEvent("bye", { detail: { requestId: id } }),
      );
      return;
    }
    if (frame.kind !== "activation")
      throw new Error("Unexpected deal client frame");
    if (this.#state !== "ready") throw new Error("Deal is not ready");
    const activation = decodeActivation(frame.payload);
    if (
      peerKey !== this.#previous ||
      hex(activation.digest) !== hex(this.#digest) ||
      activation.layer !== this.#range.first ||
      activation.values.length !== this.#range.width
    )
      throw new Error("Activation does not match assigned deal route");
    const { requestId, position } = activation;
    if (this.#ended.has(requestId)) throw new Error("Request already ended");
    let request = this.#requests.get(requestId);
    if (!request) {
      if (this.#requests.size >= 64)
        throw new Error("Too many active requests");
      request = {
        cache: this.#model.createCache(requestId),
        position: 0,
        pending: new Map(),
      };
      this.#requests.set(requestId, request);
    }
    if (position < request.position || request.pending.has(position))
      throw new Error("Replayed activation");
    if (position - request.position > 32 || request.pending.size >= 32)
      throw new Error("Activation reorder limit exceeded");
    request.pending.set(position, activation);
    while (request.pending.has(request.position)) {
      const next = request.pending.get(request.position);
      const values = await runActivation({
        model: this.#model,
        cache: request.cache,
        values: next.values,
        position: next.position,
      });
      if (this.#abort.signal.aborted)
        throw new Error("Deal client closed during activation");
      const output = encodeActivation({
        ...next,
        values,
        layer: this.#range.end,
      });
      await this.mesh.send(this.#next, "activation", output);
      await this.lapTracker?.recordActivation(output);
      request.pending.delete(request.position++);
      this.dispatchEvent(
        new CustomEvent("activation", {
          detail: { requestId, position: next.position },
        }),
      );
    }
  }

  close() {
    if (this.#state === "closed") return;
    this.#state = "closed";
    this.#abort.abort();
    this.mesh.removeEventListener("frame", this.#listener);
    this.mesh.removeEventListener("close", this.#closedListener);
    for (const request of this.#requests.values())
      this.#model.dropCache(request.cache);
    this.#requests.clear();
    this.#ended.clear();
    this.#model?.close();
  }
}

export function createDenseRuntime({ device, maxSeq = 512 }) {
  return {
    async loadLayerRange({ manifest, shards, firstLayer, endLayer }) {
      if (
        manifest.format !== "safetensors-dense-v1" ||
        manifest.config?.num_hidden_layers !== manifest.layer_count ||
        manifest.config?.hidden_size !== manifest.hidden_size
      )
        throw new Error("Unsupported dense model manifest");
      const { DenseEngine } = await import("../engine/engine/dense.js");
      const { parseSafetensors } =
        await import("../engine/engine/safetensors.js");
      const tensors = {};
      for (const shard of shards) {
        for (const [name, tensor] of Object.entries(
          parseSafetensors(shard.bytes.slice().buffer),
        )) {
          const match = /^model\.layers\.(\d+)\./.exec(name);
          if (
            !match ||
            +match[1] < firstLayer ||
            +match[1] >= endLayer ||
            Object.hasOwn(tensors, name)
          )
            throw new Error(
              "Shard contains tensors outside assigned layers or duplicate tensors",
            );
          tensors[name] = tensor;
        }
      }
      const engine = await DenseEngine.create({
        device,
        cfg: manifest.config,
        tensors,
        layerRange: [firstLayer, endLayer],
        hasEmbed: false,
        hasHead: false,
        maxSeq,
      });
      const caches = new Set();
      return {
        createCache(id) {
          const cache = { id, position: 0 };
          caches.add(cache);
          return cache;
        },
        async runActivation(values, position, cache) {
          if (
            !caches.has(cache) ||
            cache.position !== position ||
            position >= maxSeq
          )
            throw new Error("Invalid dense cache position");
          if (position) engine.loadSlot(cache.id);
          else engine.reset();
          const result = await engine.runHidden(values, position);
          engine.saveSlot(cache.id);
          cache.position++;
          return result;
        },
        dropCache(cache) {
          engine.dropSlot(cache.id);
          caches.delete(cache);
        },
        close() {
          engine.dropAllSlots();
          caches.clear();
          const seen = new Set();
          function release(value) {
            if (
              !value ||
              typeof value !== "object" ||
              seen.has(value) ||
              value === device ||
              ArrayBuffer.isView(value) ||
              value instanceof ArrayBuffer
            )
              return;
            seen.add(value);
            if (
              typeof GPUBuffer !== "undefined" &&
              value instanceof GPUBuffer
            )
              value.destroy();
            else for (const child of Object.values(value)) release(child);
          }
          release(engine);
        },
      };
    },
  };
}
