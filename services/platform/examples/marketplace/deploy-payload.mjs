import {
  decodeNativeProgramDeploy,
  encodeNativeProgramDeploy,
} from "@sidiora/layerx-sdk";
import {
  LayerXApplicationStateError,
  exactObject,
  hex32,
  secureBaseUrl,
} from "../support/runtime.mjs";

export const IMMUTABLE_POLICY = 0;
export const LIFECYCLE_WINDOW_MS = 300_000n;

const MAX_U64 = 0xffffffffffffffffn;

export function canonicalNativeProgramDeploy({
  programId,
  abiVersion,
  codeHash,
  wasm,
}) {
  if (![1, 2, 3].includes(abiVersion)) {
    throw new LayerXApplicationStateError(
      "refused",
      "program_build_reported_unsupported_guest_abi",
    );
  }
  const value = {
    programId: hex32(programId),
    guestAbi: abiVersion,
    policy: IMMUTABLE_POLICY,
    authority: new Uint8Array(32),
    newHash: hex32(codeHash),
    wasm: Uint8Array.from(wasm),
  };
  let payload;
  let decoded;
  try {
    payload = Uint8Array.from(encodeNativeProgramDeploy(value));
    decoded = decodeNativeProgramDeploy(payload);
  } catch {
    throw new LayerXApplicationStateError(
      "refused",
      "program_deployment_is_not_canonical",
    );
  }
  return Object.freeze({ payload, value: decoded });
}

export function accountNameForDid(did) {
  if (
    typeof did !== "string" ||
    !/^did:[a-z0-9]+:[A-Za-z0-9._-]{1,128}$/u.test(did)
  ) {
    throw new LayerXApplicationStateError("refused", "invalid_signing_key_did");
  }
  return `agent:${did}:main`;
}

export const IDENTITY_SEQUENCE_SELECTOR = "pending";

export async function readLifecycleAnchor({
  endpoint,
  token,
  did,
  evmAddress,
  fetchImplementation = fetch,
}) {
  accountNameForDid(did);
  if (
    typeof evmAddress !== "string" ||
    !/^0x[0-9a-fA-F]{40}$/u.test(evmAddress)
  ) {
    throw new LayerXApplicationStateError(
      "refused",
      "invalid_signing_key_evm_address",
    );
  }
  const block = await callRpc(
    endpoint,
    token,
    "eth_getBlockByNumber",
    ["latest", false],
    fetchImplementation,
  );
  if (
    block === null ||
    typeof block !== "object" ||
    Array.isArray(block) ||
    typeof block.stateRoot !== "string" ||
    !/^0x[0-9a-fA-F]{64}$/u.test(block.stateRoot)
  ) {
    throw new LayerXApplicationStateError("unknown", "invalid_program_state_block");
  }
  const notBefore =
    rpcUnsigned64(block.timestamp, "program_state_timestamp") * 1000n;
  const expiresAt = notBefore + LIFECYCLE_WINDOW_MS;
  if (expiresAt > MAX_U64) {
    throw new LayerXApplicationStateError(
      "unknown",
      "invalid_program_state_timestamp_ms",
    );
  }
  const nonce = await callRpc(
    endpoint,
    token,
    "eth_getTransactionCount",
    [evmAddress, IDENTITY_SEQUENCE_SELECTOR],
    fetchImplementation,
  );
  return Object.freeze({
    identitySequence: rpcUnsigned64(nonce, "identity_next_sequence"),
    notBefore,
    expiresAt,
    previousStateRoot: Buffer.from(hex32(block.stateRoot)).toString("hex"),
  });
}

export const LIST_OPERATION = 1;
export const BUY_OPERATION = 2;

export function marketplaceCallRequest({
  action,
  listingId,
  asset,
  seller,
  price,
  receiptDigest,
}) {
  const listing = Buffer.from(hex32(listingId));
  if (action === "list") {
    return Object.freeze({
      calldata: Buffer.concat([
        Buffer.from([LIST_OPERATION]),
        listing,
        Buffer.from(hex32(asset)),
        Buffer.from(hex32(seller)),
        Buffer.from(u128(price)),
      ]),
      capabilities: Object.freeze([
        "shared-storage-read",
        "shared-storage-write",
        "emit-event",
      ]),
    });
  }
  if (action !== "buy")
    throw new LayerXApplicationStateError(
      "refused",
      "unsupported_marketplace_action",
    );
  const digest = Buffer.from(hex32(receiptDigest)).toString("hex");
  u128(price);
  return Object.freeze({
    calldata: Buffer.concat([
      Buffer.from([BUY_OPERATION]),
      listing,
      Buffer.from(hex32(receiptDigest)),
    ]),
    capabilities: Object.freeze([
      "shared-storage-read",
      "shared-storage-write",
      "emit-event",
      `receipt-read:${digest}`,
      `transfer402:${Buffer.from(hex32(asset)).toString("hex")}:${Buffer.from(hex32(seller)).toString("hex")}:${price}`,
    ]),
  });
}

export function u128(value) {
  if (typeof value !== "string" || !/^(0|[1-9][0-9]{0,38})$/u.test(value)) {
    throw new LayerXApplicationStateError(
      "refused",
      "invalid_marketplace_price",
    );
  }
  let number = BigInt(value);
  if (number <= 0n || number > 0xffffffffffffffffffffffffffffffffn) {
    throw new LayerXApplicationStateError(
      "refused",
      "invalid_marketplace_price",
    );
  }
  const bytes = new Uint8Array(16);
  for (let index = 15; index >= 0; index -= 1) {
    bytes[index] = Number(number & 0xffn);
    number >>= 8n;
  }
  return bytes;
}

async function callRpc(endpoint, token, method, params, fetchImplementation) {
  let response;
  try {
    response = await fetchImplementation(
      new URL("/", secureBaseUrl(endpoint)),
      {
        method: "POST",
        headers: {
          accept: "application/json",
          "content-type": "application/json",
          authorization: `Bearer ${token}`,
        },
        body: JSON.stringify({ jsonrpc: "2.0", id: 1, method, params }),
      },
    );
  } catch {
    throw new LayerXApplicationStateError(
      "unknown",
      "identity_sequence_unreachable",
    );
  }
  const body = await response.json().catch(() => undefined);
  if (
    body?.jsonrpc === "2.0" &&
    body.id === 1 &&
    !("result" in body) &&
    body.error?.code === -32601 &&
    typeof body.error.message === "string"
  ) {
    throw new LayerXApplicationStateError(
      "refused",
      `${method}: JSON-RPC error -32601`,
    );
  }
  if (!response.ok) throw httpFailure(response.status, "identity_sequence");
  const envelope = exactObject(body);
  if (
    envelope.jsonrpc !== "2.0" ||
    envelope.id !== 1 ||
    "error" in envelope ||
    envelope.result === undefined
  ) {
    throw new LayerXApplicationStateError(
      "unknown",
      "identity_sequence_rpc_refused",
    );
  }
  return envelope.result;
}

function httpFailure(status, source) {
  if (
    status === 408 ||
    status === 409 ||
    status === 425 ||
    status === 429 ||
    status === 503
  ) {
    return new LayerXApplicationStateError(
      "pending",
      `${source}_http_${status}`,
    );
  }
  if (
    status === 400 ||
    status === 401 ||
    status === 403 ||
    status === 404 ||
    status === 410 ||
    status === 422
  ) {
    return new LayerXApplicationStateError(
      "refused",
      `${source}_http_${status}`,
    );
  }
  return new LayerXApplicationStateError("unknown", `${source}_http_${status}`);
}

function rpcUnsigned64(value, name) {
  if (
    typeof value !== "string" ||
    !/^0x(?:0|[1-9a-f][0-9a-f]{0,15})$/u.test(value)
  ) {
    throw new LayerXApplicationStateError("unknown", `invalid_${name}`);
  }
  return BigInt(value);
}
