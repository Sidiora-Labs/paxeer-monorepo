import { createServer } from "node:http";
import { once } from "node:events";
import { Readable, Transform } from "node:stream";
import { pipeline } from "node:stream/promises";
import { parseArgs } from "node:util";
import { PeerIdentity, manifestDigest } from "./identity.mjs";
import { SignalClient, PeerMesh } from "./transport.mjs";
import { DealClient, createDenseRuntime, dealDigest } from "./deal.mjs";

const MASK64 = (1n << 64n) - 1n;
const KECCAK_ROUNDS = [
  0x1n,
  0x8082n,
  0x800000000000808an,
  0x8000000080008000n,
  0x808bn,
  0x80000001n,
  0x8000000080008081n,
  0x8000000000008009n,
  0x8an,
  0x88n,
  0x80008009n,
  0x8000000an,
  0x8000808bn,
  0x800000000000008bn,
  0x8000000000008089n,
  0x8000000000008003n,
  0x8000000000008002n,
  0x8000000000000080n,
  0x800an,
  0x800000008000000an,
  0x8000000080008081n,
  0x8000000000008080n,
  0x80000001n,
  0x8000000080008008n,
];
const KECCAK_ROTATIONS = [
  0, 1, 62, 28, 27, 36, 44, 6, 55, 20, 3, 10, 43, 25, 39, 41, 45, 15, 21, 8, 18,
  2, 61, 56, 14,
];

// Ethereum uses Keccak's 0x01 padding, not SHA3's 0x06 padding.
export function keccak256(input) {
  const padded = Buffer.alloc(Math.ceil((input.length + 1) / 136) * 136);
  padded.set(input);
  padded[input.length] = 1;
  padded[padded.length - 1] |= 128;
  const state = Array(25).fill(0n);
  const rotate = (n, shift) => {
    const s = BigInt(shift);
    return ((n << s) | (n >> (64n - s))) & MASK64;
  };
  for (let offset = 0; offset < padded.length; offset += 136) {
    for (let i = 0; i < 17; i++)
      state[i] ^= padded.readBigUInt64LE(offset + i * 8);
    for (const rc of KECCAK_ROUNDS) {
      const c = Array.from(
        { length: 5 },
        (_, x) =>
          state[x] ^
          state[x + 5] ^
          state[x + 10] ^
          state[x + 15] ^
          state[x + 20],
      );
      const b = Array(25).fill(0n);
      for (let x = 0; x < 5; x++) {
        const d = c[(x + 4) % 5] ^ rotate(c[(x + 1) % 5], 1);
        for (let y = 0; y < 5; y++) {
          const i = x + y * 5;
          b[y + ((2 * x + 3 * y) % 5) * 5] = rotate(
            state[i] ^ d,
            KECCAK_ROTATIONS[i],
          );
        }
      }
      for (let x = 0; x < 5; x++)
        for (let y = 0; y < 5; y++)
          state[x + 5 * y] =
            b[x + 5 * y] ^
            (~b[((x + 1) % 5) + 5 * y] & b[((x + 2) % 5) + 5 * y]);
      state[0] ^= rc;
    }
  }
  const out = Buffer.alloc(32);
  for (let i = 0; i < 4; i++) out.writeBigUInt64LE(state[i], i * 8);
  return out;
}

function digest(value, name) {
  if (
    Array.isArray(value) &&
    value.length === 32 &&
    value.every((b) => Number.isInteger(b) && b >= 0 && b <= 255)
  )
    return Buffer.from(value).toString("hex");
  if (typeof value !== "string" || !/^[0-9a-f]{64}$/.test(value))
    throw new TypeError(`${name} requires 32-byte lowercase hex`);
  return value;
}

export function checkAttestation(
  envelope,
  { peerKey, modelDigest, runtimeDigest },
) {
  if (
    envelope.length < 96 ||
    envelope.subarray(0, 4).toString() !== "PXVE" ||
    envelope[4] !== 1 ||
    envelope[5] > 3 ||
    envelope[6] > 1 ||
    !envelope[95] ||
    envelope.length !== 96 + envelope[95] * 65
  )
    throw new Error("Invalid PXVE attestation envelope");
  const expected = keccak256(
    Buffer.from(
      digest(peerKey, "peer key") +
        digest(modelDigest, "model digest") +
        digest(runtimeDigest, "runtime digest"),
      "hex",
    ),
  );
  if (!envelope.subarray(39, 71).equals(expected))
    throw new Error(
      "Attestation report data does not match peer identity, model and runtime",
    );
  return {
    family: envelope[5],
    envelope: envelope.toString("hex"),
    peerKey,
    modelDigest,
    runtimeDigest,
  };
}

function hostURL(value) {
  const url = new URL(value);
  if (
    url.username ||
    url.password ||
    url.hash ||
    url.search ||
    !(
      url.protocol === "https:" ||
      (url.protocol === "http:" &&
        ["127.0.0.1", "[::1]"].includes(url.hostname))
    )
  )
    throw new Error(
      "Host requires HTTPS or HTTP loopback without credentials, query or fragment",
    );
  if (!url.pathname.endsWith("/")) url.pathname += "/";
  return url;
}

function route(host, path) {
  return new URL(path, host).href;
}

function port(value, name) {
  if (!/^\d+$/.test(String(value)) || Number(value) > 65535)
    throw new RangeError(`Invalid ${name}`);
  return Number(value);
}

export function parseCLI(args) {
  const command = args[0];
  const allowed = {
    join: [
      "manifest",
      "identity",
      "attestation",
      "model-digest",
      "runtime-digest",
      "model-manifest-url",
      "deal",
      "host-key",
      "status-port",
    ],
    serve: ["host", "ask-bearer", "port"],
    status: ["port"],
  };
  if (!Object.hasOwn(allowed, command))
    throw new Error(
      "Usage: inference-peer join <host> <room> --manifest <v2-file> | serve --host <host> | status",
    );
  const { values, positionals } = parseArgs({
    args: args.slice(1),
    allowPositionals: true,
    strict: true,
    options: Object.fromEntries(
      allowed[command].map((key) => [key, { type: "string" }]),
    ),
  });
  if (command === "join") {
    if (positionals.length !== 2 || !values.manifest)
      throw new Error(
        "Usage: join <host> <room> --manifest <v2-file> [--attestation <PXVE-file>]",
      );
    digest(positionals[1], "room");
    return {
      command,
      host: hostURL(positionals[0]).href,
      room: positionals[1],
      ...values,
      statusPort: port(values["status-port"] ?? 8788, "status port"),
    };
  }
  if (positionals.length) throw new Error(`Unexpected argument for ${command}`);
  if (command === "serve" && !values.host)
    throw new Error("serve requires --host");
  return {
    command,
    ...values,
    port: port(values.port ?? (command === "serve" ? 8787 : 8788), "port"),
  };
}

async function fileBytes(path, limit) {
  const handle = await import("node:fs/promises").then((fs) =>
    fs.open(path, "r"),
  );
  try {
    const info = await handle.stat();
    if (!info.isFile() || info.size > limit)
      throw new Error(`File exceeds ${limit} bytes or is not regular`);
    const bytes = Buffer.alloc(limit + 1);
    let size = 0;
    while (size <= limit) {
      const { bytesRead } = await handle.read(
        bytes,
        size,
        bytes.length - size,
        null,
      );
      if (!bytesRead) break;
      size += bytesRead;
    }
    if (size > limit) throw new Error(`File exceeds ${limit} bytes`);
    return bytes.subarray(0, size);
  } finally {
    await handle.close();
  }
}

async function postJSON(url, body) {
  const response = await fetch(url, {
    method: "POST",
    headers: { "content-type": "application/json" },
    body: JSON.stringify(body),
    redirect: "error",
    signal: AbortSignal.timeout(10000),
  });
  if (!response.ok) {
    await response.body?.cancel();
    throw new Error(`Host request refused (${response.status})`);
  }
  return response.json();
}

async function listen(server, listenPort) {
  server.listen(listenPort, "127.0.0.1");
  await once(server, "listening");
  return `http://127.0.0.1:${server.address().port}`;
}

async function closeServer(server) {
  server.closeAllConnections();
  await new Promise((resolve, reject) =>
    server.close((error) => (error ? reject(error) : resolve())),
  );
}

export async function joinCommand(options) {
  const host = hostURL(options.host);
  const room = digest(options.room, "room");
  const identity = await PeerIdentity.open({
    mode: "native",
    path: options.identity,
  });
  const manifest = await fileBytes(options.manifest, 8192);
  await manifestDigest(manifest);
  if (options.nonce !== undefined)
    throw new Error("The signalling nonce comes from the host challenge");
  if (manifest.at(-1) !== 1)
    throw new Error("join requires a native v2 manifest");
  const accepted = options.deal
    ? JSON.parse((await fileBytes(options.deal, 65536)).toString())
    : undefined;
  const acceptedDigest = accepted
    ? Buffer.from(await dealDigest(accepted)).toString("hex")
    : undefined;
  const hostPeerKey =
    options["host-key"] === undefined
      ? undefined
      : digest(options["host-key"], "host key");
  const binding = options.attestation
    ? checkAttestation(await fileBytes(options.attestation, 16671), {
        peerKey: identity.peerKey,
        modelDigest:
          options["model-digest"] ??
          (accepted && digest(accepted.model_digest, "model digest")),
        runtimeDigest:
          options["runtime-digest"] ??
          (accepted && digest(accepted.runtime_digest, "runtime digest")),
      })
    : undefined;
  if (accepted && digest(accepted.room_id, "deal room") !== room)
    throw new Error("Accepted deal belongs to another room");
  if (
    accepted &&
    binding &&
    (binding.modelDigest !== digest(accepted.model_digest, "model digest") ||
      binding.runtimeDigest !==
        digest(accepted.runtime_digest, "runtime digest"))
  )
    throw new Error("Attestation digests differ from accepted deal");
  // Every local validation above precedes binding, enrolment and the WebSocket hello.
  if (binding) {
    const result = await postJSON(route(host, "bind_peer"), binding);
    if (result.bound !== true) throw new Error("Host did not confirm bindPeer");
  }
  await identity.enrolPeer(route(host, "enrol_peer"), { manifest });
  const signalURL = new URL("/", host);
  signalURL.protocol = host.protocol === "https:" ? "wss:" : "ws:";
  const signal = new SignalClient(signalURL.href, {
    roomId: room,
    identity,
    manifest,
  });
  const session = new EventTarget();
  const state = {
    room,
    peerKey: identity.peerKey,
    host: host.href,
    state: "joining",
    backend: null,
    layerRange: null,
    activations: 0,
    error: null,
  };
  let mesh, client, runtime, server;
  let closed = false,
    closing;
  session.status = () => ({
    ...state,
    layerRange: state.layerRange?.slice() ?? null,
  });
  session.close = () => {
    if (closing) return closing;
    closed = true;
    state.state = "closed";
    closing = (async () => {
      client?.close();
      mesh?.close();
      signal.close();
      runtime?.close?.();
      if (server?.listening) await closeServer(server);
    })();
    return closing;
  };
  const report = (error) => {
    state.error = error.message;
    session.dispatchEvent(new CustomEvent("error", { detail: error }));
  };
  const fail = (error) => {
    void session.close().then(() => report(error), report);
  };
  signal.addEventListener("error", ({ detail }) => fail(detail));
  signal.addEventListener("close", () => {
    if (!closed) void session.close().catch(report);
  });
  let queued = Promise.resolve();
  signal.addEventListener("frame", ({ detail: frame }) => {
    if (frame.type === "deal_live") {
      queued = queued
        .then(async () => {
          if (!client) throw new Error("Live deal arrived before setup");
          await client.handleFrame(frame);
          if (!closed) state.state = "ready";
        })
        .catch(fail);
      return;
    }
    if (frame.type !== "deal") return;
    queued = queued
      .then(async () => {
        if (!acceptedDigest || !hostPeerKey)
          throw new Error(
            "Execution requires a chain-checked --deal file and --host-key",
          );
        const deal = frame.deal;
        if (
          Buffer.from(await dealDigest(deal)).toString("hex") !== acceptedDigest
        )
          throw new Error("Deal differs from accepted chain terms");
        if (closed) throw new Error("Join closed before deal setup");
        runtime = await createNativeRuntime();
        state.backend = runtime.backend;
        if (closed) {
          runtime.close?.();
          return;
        }
        mesh = await PeerMesh.create(signal, { mode: "native" });
        if (closed) {
          mesh.close();
          return;
        }
        mesh.addEventListener("error", ({ detail }) => fail(detail));
        client = new DealClient({
          roomId: room,
          identity,
          offer: signalOffer.offer,
          runtime,
          mesh,
          hostPeerKey,
          validateDeal: async (value) =>
            Buffer.from(await dealDigest(value)).toString("hex") ===
            acceptedDigest,
          modelSource: options["model-manifest-url"]
            ? { manifest_url: options["model-manifest-url"] }
            : undefined,
          acknowledge: (payload) => signal.acknowledgeDeal(payload),
        });
        client.addEventListener("error", ({ detail }) => fail(detail));
        client.addEventListener("activation", () => state.activations++);
        await client.handleFrame(frame);
        const split = deal.plan.layers.find(
          (part) => digest(part.peer_key, "peer key") === identity.peerKey,
        );
        state.layerRange = [split.first_layer, split.end_layer];
        await mesh.join({
          dealId: digest(deal.deal_id, "deal id"),
          peers: [
            hostPeerKey,
            ...deal.plan.layers.map((part) =>
              digest(part.peer_key, "peer key"),
            ),
          ],
        });
        if (!closed && signal.state !== "ready") state.state = "acknowledged";
      })
      .catch(fail);
  });
  let signalOffer;
  try {
    signalOffer = await signal.connect();
    state.state = "offered";
    if (closed) throw new Error(state.error ?? "Room closed during join");
    server = createServer((request, response) => {
      response.setHeader("content-type", "application/json");
      if (request.method !== "GET" || request.url !== "/status") {
        response.writeHead(404).end(JSON.stringify({ error: "Unknown route" }));
        return;
      }
      response.end(JSON.stringify(session.status()));
    });
    session.url = await listen(server, options.statusPort ?? 8788);
    if (closed) {
      if (server.listening) await closeServer(server);
      throw new Error(state.error ?? "Room closed during join");
    }
    return session;
  } catch (error) {
    await session.close();
    throw error;
  }
}

export async function serveCommand(options) {
  const host = hostURL(options.host);
  const bearer = options["ask-bearer"] ?? process.env.PAXAI_ASK_BEARER;
  if (
    typeof bearer !== "string" ||
    !bearer.length ||
    /[\s\x00-\x1f\x7f]/.test(bearer)
  )
    throw new Error(
      "serve requires an ask bearer (--ask-bearer or PAXAI_ASK_BEARER)",
    );
  const controllers = new Set();
  const server = createServer(async (request, response) => {
    if (
      request.method !== "POST" ||
      !["/v1/chat/completions", "/v1/completions"].includes(request.url)
    ) {
      request.resume();
      response.writeHead(404).end();
      return;
    }
    const controller = new AbortController();
    controllers.add(controller);
    response.on("close", () => {
      if (!response.writableFinished) controller.abort();
    });
    let bytes = 0;
    const bounded = new Transform({
      transform(chunk, encoding, callback) {
        bytes += chunk.length;
        callback(
          bytes > 1048576
            ? new Error("Completion request exceeds 1048576 bytes")
            : null,
          chunk,
        );
      },
    });
    const input = pipeline(request, bounded).catch((error) => {
      controller.abort(error);
    });
    try {
      const upstream = await fetch(route(host, request.url.slice(1)), {
        method: "POST",
        headers: {
          "content-type": "application/json",
          authorization: `Bearer ${bearer}`,
        },
        body: bounded,
        duplex: "half",
        redirect: "error",
        signal: controller.signal,
      });
      response.statusCode = upstream.status;
      for (const name of [
        "content-type",
        "cache-control",
        "retry-after",
        "www-authenticate",
      ])
        if (upstream.headers.has(name))
          response.setHeader(name, upstream.headers.get(name));
      response.flushHeaders();
      if (upstream.body)
        await pipeline(Readable.fromWeb(upstream.body), response);
      else response.end();
    } catch {
      if (!response.headersSent && !response.destroyed) {
        response.writeHead(bytes > 1048576 ? 413 : 502, {
          "content-type": "application/json",
        });
        response.end(
          JSON.stringify({
            error: {
              message:
                bytes > 1048576
                  ? "Request too large"
                  : "Host completion unavailable",
              type: "upstream_error",
            },
          }),
        );
      } else response.destroy();
    } finally {
      controller.abort();
      bounded.destroy();
      await input;
      controllers.delete(controller);
    }
  });
  const url = await listen(server, options.port ?? 8787);
  return {
    url,
    async close() {
      for (const controller of controllers) controller.abort();
      await closeServer(server);
    },
  };
}

export async function statusCommand({ port: statusPort = 8788 } = {}) {
  const response = await fetch(
    `http://127.0.0.1:${port(statusPort, "status port")}/status`,
    {
      redirect: "error",
      signal: AbortSignal.timeout(2000),
    },
  );
  if (!response.ok) throw new Error(`Status refused (${response.status})`);
  return response.json();
}

export async function runCLI(args) {
  const options = parseCLI(args);
  if (options.command === "status") {
    console.log(JSON.stringify(await statusCommand(options)));
    return;
  }
  const service = await (options.command === "join"
    ? joinCommand(options)
    : serveCommand(options));
  console.log(
    JSON.stringify(
      options.command === "join"
        ? { ...service.status(), statusURL: service.url }
        : { url: service.url },
    ),
  );
  const stop = () => {
    void service.close().catch((error) => {
      console.error(error.message);
      process.exitCode = 1;
    });
  };
  process.once("SIGINT", stop);
  process.once("SIGTERM", stop);
  service.addEventListener?.("error", ({ detail }) => {
    console.error(detail.message);
    process.exitCode = 1;
  });
}

export async function createNativeRuntime({
  backend = "auto",
  maxSeq = 512,
} = {}) {
  if (!["auto", "cpu"].includes(backend))
    throw new Error("Unknown native backend");
  if (backend === "auto") {
    let available = true;
    try {
      import.meta.resolve("webgpu");
    } catch (error) {
      if (error.code !== "ERR_MODULE_NOT_FOUND") throw error;
      available = false;
    }
    if (available) {
      const { create, globals } = await import("webgpu");
      Object.assign(globalThis, globals);
      const gpu = create([]);
      const adapter = await gpu.requestAdapter();
      if (!adapter) throw new Error("Installed webgpu has no usable adapter");
      const device = await adapter.requestDevice({
        requiredFeatures: adapter.features.has("shader-f16")
          ? ["shader-f16"]
          : [],
        requiredLimits: {
          maxBufferSize: adapter.limits.maxBufferSize,
          maxStorageBufferBindingSize:
            adapter.limits.maxStorageBufferBindingSize,
        },
      });
      return {
        ...createDenseRuntime({ device, maxSeq }),
        backend: "webgpu",
        gpu,
        close() {
          device.destroy();
          this.gpu = null;
        },
      };
    }
  }
  return { ...createCPURuntime({ maxSeq }), backend: "cpu" };
}

// Dense safetensors on CPU, using the vendored parser and tensor conversion.
// Embeddings and the output head stay on the host, as in DenseEngine.runHidden.
export function createCPURuntime({ maxSeq = 512 } = {}) {
  if (!Number.isSafeInteger(maxSeq) || maxSeq < 1)
    throw new Error("Invalid maxSeq");
  return {
    async loadLayerRange({ manifest, shards, firstLayer, endLayer }) {
      const cfg = manifest.config;
      if (
        manifest.format !== "safetensors-dense-v1" ||
        !cfg ||
        cfg.num_hidden_layers !== manifest.layer_count ||
        cfg.hidden_size !== manifest.hidden_size ||
        cfg.rope_scaling ||
        (cfg.hidden_act !== undefined && cfg.hidden_act !== "silu")
      )
        throw new Error("Unsupported CPU dense model manifest");
      const width = cfg.hidden_size,
        heads = cfg.num_attention_heads,
        kvHeads = cfg.num_key_value_heads;
      const headDim = cfg.head_dim ?? width / heads,
        intermediate = cfg.intermediate_size;
      for (const value of [width, heads, kvHeads, headDim, intermediate])
        if (!Number.isSafeInteger(value) || value < 1)
          throw new Error("Invalid dense dimensions");
      if (
        heads % kvHeads ||
        headDim % 2 ||
        !Number.isFinite(cfg.rms_norm_eps) ||
        cfg.rms_norm_eps <= 0 ||
        !Number.isFinite(cfg.rope_theta) ||
        cfg.rope_theta <= 0 ||
        !Number.isInteger(firstLayer) ||
        !Number.isInteger(endLayer) ||
        firstLayer < 0 ||
        firstLayer >= endLayer ||
        endLayer > manifest.layer_count
      )
        throw new Error("Invalid dense attention configuration or range");
      const { parseSafetensors, tensorF32 } = await import(
        "../engine/engine/safetensors.js"
      );
      const tensors = new Map();
      for (const shard of shards) {
        for (const [name, tensor] of Object.entries(
          parseSafetensors(Uint8Array.from(shard.bytes).buffer),
        )) {
          const layer = /^model\.layers\.(\d+)\./.exec(name);
          if (
            !layer ||
            +layer[1] < firstLayer ||
            +layer[1] >= endLayer ||
            tensors.has(name)
          )
            throw new Error(
              "CPU shard contains duplicate or unassigned tensor",
            );
          tensors.set(name, tensor);
        }
      }
      function weight(name, shape) {
        const tensor = tensors.get(name);
        if (
          !tensor ||
          tensor.shape.length !== shape.length ||
          tensor.shape.some((n, i) => n !== shape[i])
        )
          throw new Error(`Invalid CPU tensor ${name}`);
        const data = tensorF32(tensor);
        if (!data.every(Number.isFinite))
          throw new Error(`Nonfinite CPU tensor ${name}`);
        tensors.delete(name);
        return data;
      }
      const qWidth = heads * headDim,
        kvWidth = kvHeads * headDim;
      const layers = [];
      for (let layer = firstLayer; layer < endLayer; layer++) {
        const prefix = `model.layers.${layer}.`;
        layers.push({
          norm: weight(prefix + "input_layernorm.weight", [width]),
          q: weight(prefix + "self_attn.q_proj.weight", [qWidth, width]),
          k: weight(prefix + "self_attn.k_proj.weight", [kvWidth, width]),
          v: weight(prefix + "self_attn.v_proj.weight", [kvWidth, width]),
          o: weight(prefix + "self_attn.o_proj.weight", [width, qWidth]),
          postNorm: weight(prefix + "post_attention_layernorm.weight", [width]),
          gate: weight(prefix + "mlp.gate_proj.weight", [intermediate, width]),
          up: weight(prefix + "mlp.up_proj.weight", [intermediate, width]),
          down: weight(prefix + "mlp.down_proj.weight", [width, intermediate]),
        });
      }
      if (tensors.size)
        throw new Error("Unsupported CPU tensor (bias or head normalisation)");
      function multiply(matrix, input) {
        return Float32Array.from(
          { length: matrix.length / input.length },
          (_, row) => {
            let sum = 0;
            for (let col = 0; col < input.length; col++)
              sum += matrix[row * input.length + col] * input[col];
            return sum;
          },
        );
      }
      function normalise(input, weights) {
        const rms = Math.sqrt(
          input.reduce((sum, n) => sum + n * n, 0) / input.length +
            cfg.rms_norm_eps,
        );
        return Float32Array.from(input, (n, i) => (n * weights[i]) / rms);
      }
      function rope(values, position) {
        for (let offset = 0; offset < values.length; offset += headDim)
          for (let i = 0; i < headDim / 2; i++) {
            const angle =
              position * Math.pow(cfg.rope_theta, (-2 * i) / headDim);
            const a = values[offset + i],
              b = values[offset + i + headDim / 2];
            values[offset + i] = a * Math.cos(angle) - b * Math.sin(angle);
            values[offset + i + headDim / 2] =
              a * Math.sin(angle) + b * Math.cos(angle);
          }
      }
      const caches = new Set();
      let closed = false;
      const model = {
        createCache(id) {
          if (closed) throw new Error("CPU model is closed");
          const cache = {
            id,
            position: 0,
            layers: layers.map(() => ({ keys: [], values: [] })),
          };
          caches.add(cache);
          return cache;
        },
        async runActivation(input, position, cache) {
          if (
            closed ||
            !caches.has(cache) ||
            position !== cache.position ||
            position >= maxSeq ||
            !(input instanceof Float32Array) ||
            input.length !== width ||
            !input.every(Number.isFinite)
          )
            throw new Error("Invalid CPU activation or cache position");
          let hidden = input.slice();
          for (const [index, layer] of layers.entries()) {
            const norm = normalise(hidden, layer.norm);
            const q = multiply(layer.q, norm),
              k = multiply(layer.k, norm),
              v = multiply(layer.v, norm);
            rope(q, position);
            rope(k, position);
            const history = cache.layers[index];
            history.keys.push(k);
            history.values.push(v);
            const context = new Float32Array(qWidth);
            for (let head = 0; head < heads; head++) {
              const kvOffset = Math.floor(head / (heads / kvHeads)) * headDim;
              const scores = history.keys.map((key) => {
                let score = 0;
                for (let i = 0; i < headDim; i++)
                  score += q[head * headDim + i] * key[kvOffset + i];
                return score / Math.sqrt(headDim);
              });
              const maximum = Math.max(...scores);
              const probabilities = scores.map((score) =>
                Math.exp(score - maximum),
              );
              const total = probabilities.reduce((sum, n) => sum + n, 0);
              for (let i = 0; i < headDim; i++)
                context[head * headDim + i] = history.values.reduce(
                  (sum, value, j) =>
                    sum + (probabilities[j] / total) * value[kvOffset + i],
                  0,
                );
            }
            const attended = multiply(layer.o, context);
            hidden = Float32Array.from(hidden, (n, i) => n + attended[i]);
            const post = normalise(hidden, layer.postNorm);
            const gate = multiply(layer.gate, post),
              up = multiply(layer.up, post);
            const gated = Float32Array.from(
              gate,
              (n, i) => (n / (1 + Math.exp(-n))) * up[i],
            );
            const down = multiply(layer.down, gated);
            hidden = Float32Array.from(hidden, (n, i) => n + down[i]);
          }
          cache.position++;
          return hidden;
        },
        dropCache(cache) {
          if (!caches.has(cache)) return;
          for (const history of cache.layers) {
            for (const values of [...history.keys, ...history.values])
              values.fill(0);
            history.keys.length = history.values.length = 0;
          }
          caches.delete(cache);
        },
        close() {
          for (const cache of caches) model.dropCache(cache);
          closed = true;
        },
      };
      return model;
    },
  };
}
