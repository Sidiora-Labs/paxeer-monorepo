const text = new TextEncoder();
const METADATA_DOMAIN = text.encode("PAXAI/participant-metadata/v2\0");
const HELLO_DOMAIN = text.encode("PAXAI/peer-hello/v1\0");
const MIB = 1024 * 1024;
export const MAX_MANIFEST_BYTES = 8192;

function bytes(value) {
  if (!(value instanceof Uint8Array)) throw new TypeError("Expected Uint8Array bytes");
  return value;
}

function concat(...parts) {
  const result = new Uint8Array(parts.reduce((n, p) => n + p.length, 0));
  let offset = 0;
  for (const part of parts) {
    result.set(part, offset);
    offset += part.length;
  }
  return result;
}

function hex(value) {
  return Array.from(value, (b) => b.toString(16).padStart(2, "0")).join("");
}

function unhex(value, length) {
  if (typeof value !== "string" || !/^(?:[0-9a-f]{2})+$/.test(value))
    throw new TypeError("Expected lowercase hex bytes");
  const result = Uint8Array.from(value.match(/../g), (b) => parseInt(b, 16));
  if (length !== undefined && result.length !== length)
    throw new RangeError("Wrong hex byte length");
  return result;
}

function uint(value, max, name) {
  if (!Number.isInteger(value) || value < 0 || value > max) throw new RangeError(`Invalid ${name}`);
  return value;
}

function runtime(mode) {
  mode ??= globalThis.process?.versions?.node ? "native" : "browser";
  if (mode !== "native" && mode !== "browser")
    throw new TypeError("Runtime must be browser or native");
  return mode;
}

// The v1 body is retained byte for byte. Validate its variable-length fields
// before appending capabilities, using workers::decode_manifest's bounds.
function validateManifest(input, version) {
  bytes(input);
  if (input.length > MAX_MANIFEST_BYTES) throw new RangeError("Manifest exceeds 8192 bytes");
  const view = new DataView(input.buffer, input.byteOffset, input.byteLength);
  let offset = 0;
  function take(n) {
    if (offset + n > input.length) throw new RangeError("Truncated manifest");
    const start = offset;
    offset += n;
    return input.subarray(start, offset);
  }
  function number(n) {
    const start = offset;
    take(n);
    if (n === 8) return view.getBigUint64(start);
    if (n === 4) return view.getUint32(start);
    if (n === 2) return view.getUint16(start);
    return view.getUint8(start);
  }
  function range(n, min, max) {
    const value = number(n);
    if (value < min || value > max) throw new RangeError("Invalid manifest field");
    return value;
  }
  function digest() {
    const value = take(32);
    if (!value.some((b) => b !== 0)) throw new RangeError("Zero manifest identity");
  }
  if (number(2) !== version) throw new RangeError("Wrong manifest version");
  for (let i = 0; i < 3; i++) digest();
  for (let i = 0; i < 3; i++) range(8, 1n, 0xffffffffffffffffn);
  const validFrom = number(8);
  const expiry = number(8);
  if (expiry <= validFrom || expiry - validFrom > 256n)
    throw new RangeError("Invalid metadata window");
  digest();
  const capabilities = range(2, 1, 8);
  let previous = "";
  for (let i = 0; i < capabilities; i++) {
    const kind = range(1, 1, 3);
    range(1, 1, 2);
    const model = hex(take(32));
    take(64);
    const schema = hex(take(32));
    take(32);
    for (let j = 0; j < 4; j++) range(4, 1, 1048576);
    range(1, 1, 3);
    range(4, 1, 3600000);
    range(2, 1, 32);
    range(1, 1, 2);
    const key = `${kind}${model}${schema}`;
    if (key <= previous) throw new RangeError("Unordered manifest capabilities");
    previous = key;
  }
  const endpoints = range(2, 1, 2);
  let previousId = 0;
  let previousUri;
  for (let i = 0; i < endpoints; i++) {
    const id = range(1, 1, 2);
    if (id <= previousId) throw new RangeError("Unordered manifest endpoints");
    previousId = id;
    const uriBytes = take(range(4, 1, 2048));
    if (uriBytes.some((b) => b > 127)) throw new RangeError("Non-ASCII endpoint");
    const uri = new TextDecoder().decode(uriBytes);
    const match = /^https:\/\/([^/:?#@]+)(?::([1-9][0-9]{0,4}))?\/paxai\/v1$/.exec(uri);
    if (!match || uri === previousUri || (match[2] && Number(match[2]) > 65535))
      throw new RangeError("Noncanonical manifest endpoint");
    const host = match[1];
    if (
      host.length > 253 ||
      /^[0-9]+$/.test(host.split(".").at(-1)) ||
      host.split(".").some((label) => !/^[a-z0-9](?:[a-z0-9-]{0,61}[a-z0-9])?$/.test(label))
    )
      throw new RangeError("Noncanonical manifest hostname");
    previousUri = uri;
    digest();
    range(1, 1, 1);
    range(2, 1, 1);
    range(2, 1, 1);
  }
  digest();
  digest();
  range(4, 0, 0);
  const baseLength = offset;
  if (version === 2) {
    number(4);
    number(4);
    range(1, 0, 4);
    const region = take(2);
    if (!region.every((b) => (b >= 65 && b <= 90) || (b >= 97 && b <= 122)))
      throw new RangeError("Region requires two ASCII letters");
    number(1);
    range(1, 0, 2);
  }
  if (offset !== input.length) throw new RangeError("Trailing manifest bytes");
  return baseLength;
}

export function encodeManifestV2(manifestV1, capabilities) {
  validateManifest(manifestV1, 1);
  if (manifestV1.length + 13 > MAX_MANIFEST_BYTES)
    throw new RangeError("Manifest exceeds 8192 bytes");
  if (!/^[a-zA-Z]{2}$/.test(capabilities.regionCode))
    throw new RangeError("Region requires two ASCII letters");
  const result = new Uint8Array(manifestV1.length + 13);
  result.set(manifestV1);
  const view = new DataView(result.buffer);
  view.setUint16(0, 2);
  const offset = manifestV1.length;
  view.setUint32(offset, uint(capabilities.acceleratorMemoryMiB, 0xffffffff, "accelerator memory"));
  view.setUint32(offset + 4, uint(capabilities.systemMemoryMiB, 0xffffffff, "system memory"));
  view.setUint8(offset + 8, uint(capabilities.bandwidthClass, 4, "bandwidth class"));
  result.set(text.encode(capabilities.regionCode), offset + 9);
  view.setUint8(offset + 11, uint(capabilities.attestationFamily, 255, "attestation family"));
  view.setUint8(offset + 12, uint(capabilities.runtimeKind, 2, "runtime kind"));
  return result;
}

function endpoint(value) {
  const url = new URL(value);
  if (
    url.username ||
    url.password ||
    url.hash ||
    !(
      url.protocol === "https:" ||
      (url.protocol === "http:" && ["127.0.0.1", "[::1]"].includes(url.hostname))
    )
  )
    throw new TypeError("Endpoint requires HTTPS or HTTP loopback without credentials");
  return url;
}

function deadline(timeoutMs) {
  uint(timeoutMs, 2000, "timeout (maximum 2000ms)");
  if (!timeoutMs) throw new RangeError("Timeout must be positive");
  return AbortSignal.timeout(timeoutMs);
}

async function probeBandwidth(probeUrl, timeoutMs) {
  const start = performance.now();
  const response = await fetch(endpoint(probeUrl), {
    cache: "no-store",
    credentials: "omit",
    redirect: "error",
    signal: deadline(timeoutMs),
  });
  const lengthText = response.headers.get("content-length");
  const length = Number(lengthText);
  if (
    response.status !== 200 ||
    !/^\d+$/.test(lengthText ?? "") ||
    length < 65536 ||
    length > 1048576 ||
    response.headers.has("content-encoding")
  ) {
    await response.body?.cancel();
    throw new Error("Bandwidth probe requires 65536..1048576 unencoded bytes");
  }
  const reader = response.body.getReader();
  let received = 0;
  try {
    for (;;) {
      const { done, value } = await reader.read();
      if (done) break;
      received += value.length;
      if (received > length) throw new RangeError("Bandwidth probe exceeds its length");
    }
    if (received !== length) throw new Error("Incomplete bandwidth probe");
  } finally {
    await reader.cancel();
    reader.releaseLock();
  }
  const bps = (received * 8000) / Math.max(performance.now() - start, 0.001);
  return bps < 1e7 ? 1 : bps < 1e8 ? 2 : bps < 1e9 ? 3 : 4;
}

export async function measureManifest({
  manifestV1,
  regionCode,
  attestationFamily = 255,
  probeUrl,
  mode,
  timeoutMs = 2000,
}) {
  mode = runtime(mode);
  validateManifest(manifestV1, 1);
  const navigator = globalThis.navigator;
  let acceleratorMemoryMiB = 0;
  if (navigator?.gpu) {
    const adapter = await navigator.gpu.requestAdapter();
    if (!adapter) throw new Error("WebGPU adapter is unavailable");
    acceleratorMemoryMiB = Math.floor(adapter.limits.maxBufferSize / MIB);
    uint(acceleratorMemoryMiB, 0xffffffff, "WebGPU maxBufferSize");
  }
  let readSystemBytes;
  if (mode === "native") {
    readSystemBytes = (await import("node:os")).totalmem;
  } else {
    const gib = navigator?.deviceMemory;
    if (!Number.isFinite(gib) || gib <= 0) throw new Error("navigator.deviceMemory is unavailable");
    readSystemBytes = () => gib * 1024 * MIB;
  }
  const bandwidthClass = await probeBandwidth(probeUrl, timeoutMs);
  // Native memory can be hot-plugged during the probe. Sample it at publication.
  const capabilities = {
    acceleratorMemoryMiB,
    systemMemoryMiB: Math.floor(readSystemBytes() / MIB),
    bandwidthClass,
    regionCode,
    attestationFamily,
    runtimeKind: mode === "browser" ? 0 : 1,
  };
  return encodeManifestV2(manifestV1, capabilities);
}

async function browserKeys(databaseName) {
  if (!globalThis.indexedDB) throw new Error("IndexedDB is unavailable");
  const db = await new Promise((resolve, reject) => {
    const request = indexedDB.open(databaseName, 1);
    request.onupgradeneeded = () => request.result.createObjectStore("keys");
    request.onsuccess = () => resolve(request.result);
    request.onerror = () => reject(request.error);
    request.onblocked = () => reject(new Error("Identity database upgrade is blocked"));
  });
  try {
    // Generate outside the transaction: awaiting crypto would close an IDB transaction.
    const candidate = await crypto.subtle.generateKey("Ed25519", false, ["sign", "verify"]);
    return await new Promise((resolve, reject) => {
      const transaction = db.transaction("keys", "readwrite");
      const store = transaction.objectStore("keys");
      const request = store.get("peer");
      let pair;
      request.onsuccess = () => {
        pair = request.result;
        if (pair === undefined) {
          pair = candidate;
          store.add(pair, "peer");
        }
      };
      transaction.oncomplete = () => resolve(pair);
      transaction.onabort = () =>
        reject(transaction.error ?? new Error("Identity transaction aborted"));
      transaction.onerror = () => reject(transaction.error);
    });
  } finally {
    db.close();
  }
}

async function nativeKeys(path) {
  const fs = await import("node:fs/promises");
  const { constants } = await import("node:fs");
  const { dirname, basename, join } = await import("node:path");
  const { createPrivateKey, createPublicKey } = await import("node:crypto");
  path ??= join((await import("node:os")).homedir(), ".paxeer", "inference-peer", "identity.pkcs8");
  async function read() {
    const handle = await fs.open(path, constants.O_RDONLY | constants.O_NOFOLLOW);
    try {
      const stat = await handle.stat();
      if (!stat.isFile() || stat.mode & 0o077 || stat.size > 4096 || stat.uid !== process.getuid())
        throw new Error("Identity key requires an owner-only regular file");
      const der = await handle.readFile();
      const privateKey = await crypto.subtle.importKey("pkcs8", der, "Ed25519", false, ["sign"]);
      const key = createPrivateKey({ key: der, format: "der", type: "pkcs8" });
      const publicDer = createPublicKey(key).export({ format: "der", type: "spki" });
      const publicKey = await crypto.subtle.importKey("spki", publicDer, "Ed25519", true, [
        "verify",
      ]);
      return { privateKey, publicKey };
    } finally {
      await handle.close();
    }
  }
  try {
    return await read();
  } catch (error) {
    if (error.code !== "ENOENT") throw error;
  }
  await fs.mkdir(dirname(path), { recursive: true, mode: 0o700 });
  const pair = await crypto.subtle.generateKey("Ed25519", true, ["sign", "verify"]);
  const der = new Uint8Array(await crypto.subtle.exportKey("pkcs8", pair.privateKey));
  const temporary = join(dirname(path), `.${basename(path)}.${crypto.randomUUID()}.tmp`);
  try {
    const handle = await fs.open(temporary, "wx", 0o600);
    try {
      await handle.writeFile(der);
      await handle.sync();
    } finally {
      await handle.close();
    }
    try {
      await fs.link(temporary, path);
    } catch (error) {
      if (error.code !== "EEXIST") throw error;
    }
  } finally {
    der.fill(0);
    await fs.unlink(temporary);
  }
  return read();
}

export async function manifestDigest(manifest) {
  validateManifest(manifest, 2);
  return new Uint8Array(await crypto.subtle.digest("SHA-256", concat(METADATA_DOMAIN, manifest)));
}

export function helloSigningBytes(roomId, nonce, metadataDigest) {
  function field(value) {
    if (typeof value !== "string" || !value.length)
      throw new TypeError("Room and nonce must be nonempty strings");
    const encoded = text.encode(value);
    if (
      encoded.length > 4096 ||
      new TextDecoder("utf-8", { fatal: true }).decode(encoded) !== value
    )
      throw new RangeError("Invalid hello text");
    const size = new Uint8Array(4);
    new DataView(size.buffer).setUint32(0, encoded.length);
    return concat(size, encoded);
  }
  return concat(HELLO_DOMAIN, field(roomId), field(nonce), unhex(metadataDigest, 32));
}

export class PeerIdentity {
  #privateKey;
  #peerKey;

  constructor(privateKey, peerKey) {
    this.#privateKey = privateKey;
    this.#peerKey = peerKey;
  }

  static async open({ mode, path, databaseName = "paxeer-inference-peer" } = {}) {
    mode = runtime(mode);
    const pair = mode === "browser" ? await browserKeys(databaseName) : await nativeKeys(path);
    const publicBytes = new Uint8Array(await crypto.subtle.exportKey("raw", pair.publicKey));
    const challenge = crypto.getRandomValues(new Uint8Array(32));
    const signature = await crypto.subtle.sign("Ed25519", pair.privateKey, challenge);
    if (!(await crypto.subtle.verify("Ed25519", pair.publicKey, signature, challenge)))
      throw new Error("Stored identity key pair does not match");
    return new PeerIdentity(pair.privateKey, hex(publicBytes));
  }

  get peerKey() {
    return this.#peerKey;
  }

  async sign(input) {
    return hex(new Uint8Array(await crypto.subtle.sign("Ed25519", this.#privateKey, bytes(input))));
  }

  async signManifest(input) {
    const manifest = bytes(input).slice();
    const digest = await manifestDigest(manifest);
    return {
      version: 2,
      bytes: hex(manifest),
      metadataDigest: hex(digest),
      peerKey: this.peerKey,
      signature: await this.sign(digest),
    };
  }

  async enrolPeer(enrolUrl, { manifest, quote, headers, timeoutMs = 2000 }) {
    const url = endpoint(enrolUrl);
    if (!url.pathname.endsWith("/enrol_peer"))
      throw new TypeError("Expected the host enrol_peer route");
    const signed = await this.signManifest(manifest);
    const body = JSON.stringify({ manifest: signed, ...(quote === undefined ? {} : { quote }) });
    if (text.encode(body).length > 262144) throw new RangeError("Enrolment exceeds 262144 bytes");
    const requestHeaders = new Headers(headers);
    requestHeaders.set("content-type", "application/json");
    const response = await fetch(url, {
      method: "POST",
      headers: requestHeaders,
      body,
      credentials: "omit",
      redirect: "error",
      signal: deadline(timeoutMs),
    });
    if (!response.ok) {
      const reason = (await response.text()).slice(0, 4096);
      throw new Error(`enrol_peer refused (${response.status}): ${reason}`);
    }
    return response.json();
  }
}

export async function signHello(identity, { roomId, nonce = crypto.randomUUID(), manifest }) {
  const signed = await identity.signManifest(manifest);
  const signature = await identity.sign(helloSigningBytes(roomId, nonce, signed.metadataDigest));
  return { peerKey: identity.peerKey, signature, nonce, manifest: signed };
}
