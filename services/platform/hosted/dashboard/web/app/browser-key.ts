const DATABASE_NAME = "layerx-developer";
const STORE_NAME = "keys";
const KEY_NAME = "developer";

type HeldKey = { publicKey: CryptoKey; privateKey: CryptoKey };
export type BrowserKeyState = { ed25519: boolean; hasKey: boolean; publicKey?: string };

export function lowercaseHex(bytes: Uint8Array): string {
  return Array.from(bytes, (byte) => byte.toString(16).padStart(2, "0")).join("");
}

function publicKeyBytes(key: string): Uint8Array<ArrayBuffer> {
  if (!/^[0-9a-f]{64}$/.test(key)) throw new Error("invalid_public_key");
  return Uint8Array.from(key.match(/../g)!, (byte) => Number.parseInt(byte, 16));
}

function tenantBytes(tenant: string): Uint8Array<ArrayBuffer> {
  if (!/^[a-z0-9._-]{1,95}$/.test(tenant)) throw new Error("invalid_tenant");
  return new TextEncoder().encode(tenant);
}

async function taggedDigest(domain: string, parts: Uint8Array<ArrayBuffer>[]): Promise<Uint8Array<ArrayBuffer>> {
  const all = [new TextEncoder().encode(domain), ...parts];
  const bytes = new Uint8Array(all.reduce((size, part) => size + 8 + part.byteLength, 0));
  const view = new DataView(bytes.buffer);
  let offset = 0;
  for (const part of all) {
    view.setBigUint64(offset, BigInt(part.byteLength), false);
    bytes.set(part, offset + 8);
    offset += 8 + part.byteLength;
  }
  return new Uint8Array(await crypto.subtle.digest("SHA-256", bytes));
}

export function registrationBinding(tenant: string, publicKey: string): Promise<Uint8Array<ArrayBuffer>> {
  return taggedDigest("layerx-register-binding-v1", [tenantBytes(tenant), publicKeyBytes(publicKey)]);
}

export function sessionDigest(tenant: string, publicKey: string, time: string): Promise<Uint8Array<ArrayBuffer>> {
  if (!/^(0|[1-9][0-9]{0,19})$/.test(time) || BigInt(time) > 18446744073709551615n) throw new Error("invalid_proof_time");
  return taggedDigest("layerx-session-open-v1", [tenantBytes(tenant), publicKeyBytes(publicKey), new TextEncoder().encode(time)]);
}

async function database(): Promise<IDBDatabase> {
  return new Promise((resolve, reject) => {
    const request = indexedDB.open(DATABASE_NAME, 1);
    request.onupgradeneeded = () => request.result.createObjectStore(STORE_NAME);
    request.onsuccess = () => resolve(request.result);
    request.onerror = () => reject(new Error("browser_key_storage_unavailable"));
    request.onblocked = () => reject(new Error("browser_key_storage_unavailable"));
  });
}

function completed(transaction: IDBTransaction): Promise<void> {
  return new Promise((resolve, reject) => {
    transaction.oncomplete = () => resolve();
    transaction.onabort = transaction.onerror = () => reject(new Error("browser_key_storage_unavailable"));
  });
}

async function heldKey(): Promise<HeldKey | undefined> {
  const db = await database();
  try {
    const transaction = db.transaction(STORE_NAME, "readonly");
    const done = completed(transaction);
    const request = transaction.objectStore(STORE_NAME).get(KEY_NAME);
    await done;
    const pair = request.result as HeldKey | undefined;
    if (pair && (pair.privateKey.extractable || pair.privateKey.type !== "private" || pair.privateKey.algorithm.name !== "Ed25519"
      || pair.publicKey.type !== "public" || pair.publicKey.algorithm.name !== "Ed25519")) {
      throw new Error("invalid_browser_key");
    }
    return pair;
  } finally {
    db.close();
  }
}

async function publicHex(pair: HeldKey): Promise<string> {
  return lowercaseHex(new Uint8Array(await crypto.subtle.exportKey("raw", pair.publicKey)));
}

export async function generateDeveloperKey(): Promise<CryptoKeyPair> {
  const pair = await crypto.subtle.generateKey("Ed25519", false, ["sign", "verify"]) as CryptoKeyPair;
  if (pair.privateKey.extractable) throw new Error("invalid_browser_key");
  return pair;
}

export async function browserKeyState(): Promise<BrowserKeyState> {
  try {
    if (!globalThis.crypto?.subtle) return { ed25519: false, hasKey: false };
    await generateDeveloperKey();
  } catch {
    return { ed25519: false, hasKey: false };
  }
  const pair = await heldKey();
  return { ed25519: true, hasKey: pair !== undefined, publicKey: pair ? await publicHex(pair) : undefined };
}

export async function createDeveloperKey(): Promise<string> {
  const existing = await heldKey();
  if (existing) return publicHex(existing);
  const pair = await generateDeveloperKey();
  const db = await database();
  try {
    const transaction = db.transaction(STORE_NAME, "readwrite");
    const done = completed(transaction);
    transaction.objectStore(STORE_NAME).add({ publicKey: pair.publicKey, privateKey: pair.privateKey }, KEY_NAME);
    await done;
  } finally {
    db.close();
  }
  return publicHex(pair);
}

export async function registrationProof(tenant: string): Promise<{ signer_public_key: string; registration_signature: string }> {
  const pair = await heldKey();
  if (!pair) throw new Error("browser_key_missing");
  const publicKey = await publicHex(pair);
  const signature = await crypto.subtle.sign("Ed25519", pair.privateKey, await registrationBinding(tenant, publicKey));
  return { signer_public_key: publicKey, registration_signature: lowercaseHex(new Uint8Array(signature)) };
}

export async function sessionProof(tenant: string): Promise<{ signer_public_key: string; proof_time: string; session_signature: string }> {
  const pair = await heldKey();
  if (!pair) throw new Error("browser_key_missing");
  const publicKey = await publicHex(pair);
  const time = Math.floor(Date.now() / 1000).toString();
  const signature = await crypto.subtle.sign("Ed25519", pair.privateKey, await sessionDigest(tenant, publicKey, time));
  return { signer_public_key: publicKey, proof_time: time, session_signature: lowercaseHex(new Uint8Array(signature)) };
}
