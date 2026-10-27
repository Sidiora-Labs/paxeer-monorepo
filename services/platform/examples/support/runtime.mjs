import { readFile } from "node:fs/promises";
import { dirname, resolve } from "node:path";
import { fileURLToPath } from "node:url";
import { keccak256, verifySequencerReceipt } from "@sidiora/layerx-sdk";

const LOOPBACK = new Set(["127.0.0.1", "localhost", "::1"]);

export class LayerXApplicationStateError extends Error {
  constructor(state, detail) {
    super(detail);
    this.name = "LayerXApplicationStateError";
    this.state = state;
  }
}

export const requiredEnvironment = (name) => {
  if (!/^[A-Z][A-Z0-9_]{0,127}$/u.test(name)) throw new Error("invalid_environment_binding");
  const value = process.env[name];
  if (value === undefined || value.length === 0) throw new Error(`missing_${name.toLowerCase()}`);
  return value;
};

export const optionalEnvironment = (name) => {
  if (name === undefined) return undefined;
  if (!/^[A-Z][A-Z0-9_]{0,127}$/u.test(name)) throw new Error("invalid_environment_binding");
  const value = process.env[name];
  return value === undefined || value.length === 0 ? undefined : value;
};

export async function loadApplicationConfig(moduleUrl, application) {
  const directory = dirname(fileURLToPath(moduleUrl));
  const document = exactObject(JSON.parse(await readFile(resolve(directory, optionalEnvironment("LAYERX_EXAMPLE_CONFIG") ?? "layerx.example.json"), "utf8")));
  if (document.version !== 1 || document.application !== application) throw new Error("invalid_application_config");
  const selected = parseArguments();
  const environments = exactObject(document.environments);
  const config = applyEndpointOverride(selected.environment, exactObject(environments[selected.environment]));
  return Object.freeze({
    name: selected.environment,
    application,
    action: selected.action,
    directory,
    ...config,
  });
}

export function applicationStatePath(config, relative) {
  if (typeof relative !== "string" || relative.length === 0 || relative.startsWith("/")
    || relative.split(/[\\/]/u).some((part) => part === "..")) {
    throw new Error("invalid_application_state_path");
  }
  const stateRoot = optionalEnvironment("LAYERX_EXAMPLE_STATE_ROOT");
  if (stateRoot === undefined) return resolve(config.directory, relative);
  if (typeof config.application !== "string" || !/^[a-z][a-z0-9-]{0,63}$/u.test(config.application)) {
    throw new Error("invalid_application_state_identity");
  }
  return resolve(stateRoot, config.application, relative);
}

const ENDPOINT_FIELDS = Object.freeze(["humanUrl", "rpcUrl", "settlementUrl", "endpoint"]);

export function applyEndpointOverride(environment, config) {
  const override = optionalEnvironment("LAYERX_EXAMPLE_ENDPOINT");
  if (override === undefined || environment !== "beta") return config;
  const base = secureBaseUrl(override);
  const rebased = { ...config };
  for (const field of ENDPOINT_FIELDS) {
    const current = config[field];
    if (current === undefined) continue;
    if (typeof current !== "string") throw new Error("invalid_application_endpoint");
    const path = secureUrl(current).pathname.replace(/^\//u, "");
    rebased[field] = new URL(path, base).toString();
  }
  return rebased;
}

export function receiptKeysFromEnvironment(name) {
  const entries = JSON.parse(requiredEnvironment(name));
  if (!Array.isArray(entries) || entries.length === 0) throw new Error("invalid_receipt_keys");
  return Object.freeze(entries.map((entry) => {
    const key = exactObject(entry);
    const firstHeight = receiptKeyHeight(key.firstHeight);
    const lastHeight = receiptKeyHeight(key.lastHeight);
    if (firstHeight > lastHeight) throw new Error("invalid_receipt_key_range");
    return Object.freeze({
      keyId: hex32(key.keyId),
      first_height: firstHeight,
      last_height: lastHeight,
      publicKey: hex32(key.publicKey),
    });
  }));
}

export class SequencerReceiptClient {
  #receiptKeys;

  constructor(rpcUrl, receiptKeys) {
    this.rpcUrl = secureUrl(rpcUrl).toString();
    if (!Array.isArray(receiptKeys) || receiptKeys.length === 0) throw new Error("invalid_receipt_keys");
    this.#receiptKeys = receiptKeys.map((key) => {
      if (!(key.keyId instanceof Uint8Array) || key.keyId.length !== 32
        || !(key.publicKey instanceof Uint8Array) || key.publicKey.length !== 32
        || typeof key.first_height !== "bigint" || typeof key.last_height !== "bigint"
        || key.first_height < 0n || key.last_height > 0xffff_ffff_ffff_ffffn
        || key.first_height > key.last_height) throw new Error("invalid_receipt_keys");
      return Object.freeze({ ...key, keyId: new Uint8Array(key.keyId), publicKey: new Uint8Array(key.publicKey) });
    });
  }

  async resolve(rawTransaction) {
    const raw = rawTransaction instanceof Uint8Array
      ? new Uint8Array(rawTransaction) : receiptBytes(rawTransaction);
    if (raw.length === 0) throw new LayerXApplicationStateError("refused", "invalid_raw_transaction");
    const txHash = receiptHex(keccak256(raw));
    let response;
    let body;
    try {
      response = await fetch(this.rpcUrl, {
        method: "POST",
        headers: { accept: "application/json", "content-type": "application/json" },
        body: JSON.stringify({ jsonrpc: "2.0", id: 1, method: "pax_sequencerReceipt", params: [txHash] }),
      });
      body = await response.json();
    } catch {
      throw new LayerXApplicationStateError("unknown", "sequencer_receipt_unreachable");
    }
    if (body === null || typeof body !== "object" || Array.isArray(body)
      || body.jsonrpc !== "2.0" || body.id !== 1) {
      throw new LayerXApplicationStateError("unknown", "invalid_sequencer_response");
    }
    if (body.error !== undefined) throw new LayerXApplicationStateError("refused", "sequencer_receipt_refused");
    if (!response.ok) throw new LayerXApplicationStateError("unknown", `sequencer_receipt_http_${response.status}`);
    const result = body.result;
    if (result === null || typeof result !== "object" || Array.isArray(result) || result.txHash !== txHash) {
      throw new LayerXApplicationStateError("unknown", "sequencer_receipt_mismatch");
    }
    if (result.rung === "submitted") throw new LayerXApplicationStateError("pending", "sequencer_receipt_submitted");
    if (result.rung !== "receipted" && result.rung !== "committed") {
      throw new LayerXApplicationStateError("unknown", "invalid_sequencer_receipt_rung");
    }
    const signedReceipt = receiptBytes(result.receipt);
    for (const key of this.#receiptKeys) {
      try {
        await verifySequencerReceipt(signedReceipt, raw, [key]);
      } catch {
        continue;
      }
      return {
        format: "layerx-receipt-proof-v2",
        verificationLevel: "receipted",
        rawTransaction: receiptHex(raw),
        signedReceipt: receiptHex(signedReceipt),
        receiptDigest: receiptHex(keccak256(signedReceipt)),
        receiptKey: {
          keyId: receiptHex(key.keyId),
          publicKey: receiptHex(key.publicKey),
          firstHeight: `0x${key.first_height.toString(16)}`,
          lastHeight: `0x${key.last_height.toString(16)}`,
        },
      };
    }
    throw new LayerXApplicationStateError("refused", "sequencer_receipt_verification_failed");
  }
}

function receiptKeyHeight(value) {
  if ((typeof value !== "string" || !/^(?:0|[1-9][0-9]*|0x(?:0|[1-9a-f][0-9a-f]{0,15}))$/u.test(value))
    && (typeof value !== "number" || !Number.isSafeInteger(value) || value < 0)) {
    throw new Error("invalid_receipt_key_height");
  }
  const height = BigInt(value);
  if (height > 0xffff_ffff_ffff_ffffn) throw new Error("invalid_receipt_key_height");
  return height;
}

function receiptBytes(value) {
  if (typeof value !== "string" || !/^0x(?:[0-9a-f]{2})+$/u.test(value) || value.length > 2097154) {
    throw new LayerXApplicationStateError("refused", "invalid_sequencer_receipt_data");
  }
  return Uint8Array.from(Buffer.from(value.slice(2), "hex"));
}

function receiptHex(value) {
  return `0x${Buffer.from(value).toString("hex")}`;
}

const DIAGNOSTIC_LIMIT = 1024;

export function diagnosticText(value, secrets = [], limit = DIAGNOSTIC_LIMIT) {
  if (!Number.isSafeInteger(limit) || limit < 32) throw new Error("invalid_diagnostic_limit");
  let text = typeof value === "string" ? value : Buffer.from(value ?? []).toString("utf8");
  for (const secret of secrets) {
    if (typeof secret !== "string" || secret.length < 8) continue;
    text = text.split(secret).join("[redacted]");
  }
  text = text.replaceAll(/[^\t\n\r\u0020-\u007e]+/gu, " ").replaceAll(/\s+/gu, " ").trim();
  return text.length > limit ? `${text.slice(0, limit)} [truncated]` : text;
}

export function commandPath(arguments_) {
  const path = [];
  for (const part of arguments_) {
    if (typeof part !== "string" || part.startsWith("-")) break;
    path.push(part);
  }
  return path.length === 0 ? "layerx" : path.join(" ");
}

export function exactObject(value) {
  if (value === null || typeof value !== "object" || Array.isArray(value)) throw new Error("invalid_application_data");
  return value;
}

export function secureBaseUrl(value) {
  const url = secureUrl(value.endsWith("/") ? value : `${value}/`);
  return url;
}

export function secureUrl(value) {
  const url = new URL(value);
  if (url.username !== "" || url.password !== "" || url.hash !== "" || url.search !== "") {
    throw new Error("invalid_service_url");
  }
  if (url.protocol !== "https:" && !(url.protocol === "http:" && LOOPBACK.has(url.hostname))) {
    throw new Error("insecure_service_url");
  }
  return url;
}

export function hex32(value) {
  const digits = typeof value === "string" && value.startsWith("0x") ? value.slice(2) : value;
  if (typeof digits !== "string" || !/^[0-9a-fA-F]{64}$/u.test(digits)) throw new Error("invalid_receipt_authority");
  return Uint8Array.from({ length: 32 }, (_, index) => Number.parseInt(digits.slice(index * 2, index * 2 + 2), 16));
}

export function equalBytes(left, right) {
  if (!(left instanceof Uint8Array) || !(right instanceof Uint8Array) || left.length !== right.length) return false;
  let difference = 0;
  for (let index = 0; index < left.length; index += 1) difference |= left[index] ^ right[index];
  return difference === 0;
}

function parseArguments() {
  const arguments_ = process.argv.slice(2);
  if ((arguments_.length !== 2 && arguments_.length !== 4) || arguments_[0] !== "--environment" || !["emulator", "beta"].includes(arguments_[1])) {
    throw new Error("usage_--environment_emulator_or_beta");
  }
  if (arguments_.length === 4 && (arguments_[2] !== "--action" || !["deploy", "list", "buy"].includes(arguments_[3]))) {
    throw new Error("usage_--action_deploy_list_or_buy");
  }
  return { environment: arguments_[1], action: arguments_[3] ?? "run" };
}
