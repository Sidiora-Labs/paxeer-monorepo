import { createServer } from "node:http";
import { AsyncLocalStorage } from "node:async_hooks";
import { createHash, timingSafeEqual } from "node:crypto";
import { FileCompareAndSet } from "./file-state.mjs";
import { keccak256 } from "@sidiora/layerx-sdk";
import { link, mkdir, open, readFile, unlink } from "node:fs/promises";
import { basename, dirname, join } from "node:path";
import { MerchantError, MerchantMiddleware, MerchantSettlementWebhooks } from "@sidiora/layerx-merchant-middleware";
import {
  decodePaymentCommitmentEvidence,
  encodePaymentCommitmentEvidence,
  MiddlewareError,
  SellerMiddleware,
  VerifiedWebhookConsumer,
  encodeSettlementHeader,
} from "@sidiora/layerx-seller-middleware";
import {
  LayerXApplicationStateError,
  SequencerReceiptClient,
  receiptKeysFromEnvironment,
  applicationStatePath,
  exactObject,
  boundedJsonRequest,
  hex32,
  loadApplicationConfig,
  requiredEnvironment,
  secureUrl,
} from "./runtime.mjs";

export function platform_ref_merchant() {
  return "merchant-middleware-receipt-backed-orders-and-webhooks";
}

const safeName = (value) => {
  if (!/^[A-Za-z0-9._-]{1,255}$/u.test(value) || basename(value) !== value) throw new Error("invalid_identifier");
  return value;
};

const transactionHash = (raw) => `0x${Buffer.from(keccak256(raw)).toString("hex")}`;

class TransactionReceipts {
  constructor(rpcUrl, receiptKeys, transportLimits = {}) {
    this.client = new SequencerReceiptClient(rpcUrl, receiptKeys, transportLimits);
  }

  async resolve(transaction) {
    if (typeof transaction !== "string" || !/^0x[0-9a-f]{64}$/u.test(transaction)) {
      throw new LayerXApplicationStateError("refused", "invalid_transaction_hash");
    }
    let response;
    let body;
    try {
      response = await boundedJsonRequest(this.client.rpcUrl, {
        method: "POST",
        headers: { "content-type": "application/json" },
        body: JSON.stringify({ jsonrpc: "2.0", id: 1, method: "eth_getRawTransactionByHash", params: [transaction] }),
      }, this.client.transportLimits);
      body = response.body;
    } catch {
      throw new LayerXApplicationStateError("unknown", "transaction_unreachable");
    }
    if (!response.ok || body?.jsonrpc !== "2.0" || body.id !== 1 || body.error !== undefined) {
      throw new LayerXApplicationStateError("unknown", "invalid_transaction_response");
    }
    if (body.result === null) throw new LayerXApplicationStateError("pending", "transaction_unavailable");
    if (typeof body.result !== "string" || !/^0x(?:[0-9a-f]{2})+$/u.test(body.result)
      || body.result.length > 2097154) {
      throw new LayerXApplicationStateError("unknown", "invalid_raw_transaction");
    }
    const raw = Uint8Array.from(Buffer.from(body.result.slice(2), "hex"));
    if (transactionHash(raw) !== transaction) {
      throw new LayerXApplicationStateError("refused", "transaction_hash_mismatch");
    }
    return this.client.resolve(raw);
  }
}

export class MerchantCheckout extends MerchantMiddleware {
  async quote(checkoutKey, lines) {
    const quote = await super.quote(checkoutKey, lines);
    const requirements = quote.paymentRequired.accepts[0];
    requirements.extra = {
      ...requirements.extra,
      layerx: {
        ...requirements.extra.layerx,
        purposeHash: transactionHash(new TextEncoder().encode(quote.paymentRequired.resource.url)).slice(2),
      },
    };
    return quote;
  }
}

const syncDirectory = async (path) => {
  const directory = await open(dirname(path), "r");
  try {
    await directory.sync();
  } finally {
    await directory.close();
  }
};

const claimJson = async (path, value) => {
  const temporary = `${path}.${process.pid}.${crypto.randomUUID()}.tmp`;
  const file = await open(temporary, "wx", 0o600);
  try {
    try {
      await file.writeFile(JSON.stringify(value), "utf8");
      await file.sync();
    } finally {
      await file.close();
    }
    try {
      await link(temporary, path);
    } catch (error) {
      if (error.code !== "EEXIST") throw error;
    }
    await syncDirectory(path);
  } finally {
    await unlink(temporary);
  }
};

export class DeclaredCatalog {
  constructor(item) {
    this.item = Object.freeze(item);
  }

  async get(sku) {
    return sku === this.item.sku ? this.item : undefined;
  }
}

export class FileOrders {
  constructor(directory, identity) {
    this.directory = directory;
    this.identity = identity === undefined ? undefined : Object.freeze({ ...identity });
    this.storage = new FileCompareAndSet(directory);
  }

  path(orderId) {
    return this.storage.path(orderId);
  }

  async open(request) {
    const orderId = request.checkoutKey;
    const order = { ...request, ...this.identity, orderId, state: "awaiting-payment" };
    await this.storage.compareAndSet(orderId, -1, order);
    const stored = await this.getRequired(orderId);
    if (stored.requestDigest !== request.requestDigest
      || JSON.stringify(stored.quote) !== JSON.stringify(request.quote)
      || (this.identity !== undefined
        && (stored.tenant !== this.identity.tenant || stored.principal !== this.identity.principal))) {
      throw new MerchantError("order-conflict");
    }
    return stored;
  }

  async releaseResource(orderId) {
    return this.getRequired(orderId);
  }

  async markPaid(orderId, requestDigest, receiptDigest, transaction) {
    return this.transition(orderId, requestDigest, { state: "paid-verified", receiptDigest, transaction });
  }

  async markRefused(orderId, requestDigest) {
    return this.transition(orderId, requestDigest, { state: "refused" });
  }

  async transition(orderId, requestDigest, terminal) {
    const current = await this.storage.read(orderId);
    if (current === undefined) throw new Error("order_missing");
    const order = current.value;
    if (order.requestDigest !== requestDigest) throw new MerchantError("order-conflict");
    if (order.state === "awaiting-payment") {
      await this.storage.compareAndSet(orderId, current.version, { ...order, ...terminal });
    }
    const winner = await this.getRequired(orderId);
    if (winner.state !== terminal.state || winner.receiptDigest !== terminal.receiptDigest
      || winner.transaction !== terminal.transaction) throw new MerchantError("order-conflict");
    return winner;
  }

  async get(orderId) {
    return (await this.storage.read(orderId))?.value;
  }

  async getRequired(orderId) {
    const order = await this.get(orderId);
    if (order === undefined) throw new Error("order_missing");
    return order;
  }
}

class SettlementAuthority {
  constructor(url, token, receipts, transportLimits = {}) {
    this.url = secureUrl(url);
    this.token = token;
    this.receipts = receipts;
    this.transportLimits = Object.freeze({ ...transportLimits });
  }

  async settle(request) {
    let response;
    try {
      response = await boundedJsonRequest(this.url.toString(), {
        method: "POST",
        headers: { authorization: `Bearer ${this.token}`, "content-type": "application/json" },
        body: JSON.stringify(request),
      }, this.transportLimits);
    } catch {
      throw new LayerXApplicationStateError("unknown", "settlement_authority_unreachable");
    }
    const body = response.body;
    if (response.status === 202) return { kind: "pending" };
    if (response.status === 408 || response.status === 409 || response.status === 425 || response.status >= 500) {
      throw new LayerXApplicationStateError("unknown", `settlement_authority_http_${response.status}`);
    }
    if (!response.ok) return { kind: "refused", reason: `settlement_authority_http_${response.status}` };
    const result = body?.result ?? body;
    if (result === null || typeof result !== "object" || Array.isArray(result)) {
      throw new LayerXApplicationStateError("unknown", "invalid_settlement_response");
    }
    if (result.state === "pending") return { kind: "pending" };
    if (result.state === "refused") return { kind: "refused", reason: String(result.reason ?? "refused") };
    if (result.state === "unknown") throw new LayerXApplicationStateError("unknown", "settlement_unknown");
    if (result.state !== "settled" || typeof result.transaction !== "string" || !/^0x[0-9a-f]{64}$/u.test(result.transaction)) {
      throw new LayerXApplicationStateError("unknown", "invalid_settlement_response");
    }
    return {
      kind: "settled",
      ...await this.receipts.resolve(result.transaction),
    };
  }
}

export class FileFulfillments {
  constructor(directory) {
    this.directory = directory;
  }

  async get(idempotencyKey) {
    const path = join(this.directory, `${safeName(idempotencyKey)}.json`);
    return this.read(path);
  }

  async read(path) {
    try {
      const record = exactObject(JSON.parse(await readFile(path, "utf8")));
      return { ...record, ...(record.commitmentEvidence === undefined ? {} : {
        commitmentEvidence: decodePaymentCommitmentEvidence(record.commitmentEvidence),
      }) };
    } catch (error) {
      if (error.code === "ENOENT") return undefined;
      throw error;
    }
  }

  encode(record) {
    return { ...record, ...(record.commitmentEvidence === undefined ? {} : {
      commitmentEvidence: encodePaymentCommitmentEvidence(record.commitmentEvidence),
    }) };
  }

  requireMatching(stored, proposed) {
    if (stored.idempotencyKey !== proposed.idempotencyKey
      || stored.requestDigest !== proposed.requestDigest || stored.txHash !== proposed.txHash
      || stored.receiptDigest !== proposed.receiptDigest) throw new MiddlewareError("fulfillment-conflict");
  }

  async completed(proposed, path) {
    const record = await this.get(proposed.idempotencyKey);
    if (record === undefined) return undefined;
    this.requireMatching(record, proposed);
    try {
      await unlink(`${path}.pending`);
    } catch (error) {
      if (error.code !== "ENOENT") throw error;
    }
    await syncDirectory(path);
    return record;
  }

  async fulfill(proposed, release) {
    await mkdir(this.directory, { recursive: true, mode: 0o700 });
    await syncDirectory(this.directory);
    const path = join(this.directory, `${safeName(proposed.idempotencyKey)}.json`);
    const stored = await this.completed(proposed, path);
    if (stored !== undefined) {
      return stored;
    }
    const pendingPath = `${path}.pending`;
    await claimJson(pendingPath, { state: "pending", fulfillment: this.encode(proposed) });
    let pending;
    try {
      pending = exactObject(JSON.parse(await readFile(pendingPath, "utf8")));
    } catch (error) {
      if (error.code !== "ENOENT") throw error;
      const result = await this.completed(proposed, path);
      if (result === undefined) throw error;
      return result;
    }
    if (pending.state !== "pending") throw new MiddlewareError("fulfillment-conflict");
    const claimed = exactObject(pending.fulfillment);
    this.requireMatching(claimed, proposed);
    const completed = await this.completed(proposed, path);
    if (completed !== undefined) {
      return completed;
    }
    const resource = await release(claimed.idempotencyKey);
    await claimJson(path, { ...claimed, resource });
    return this.completed(proposed, path);
  }
}

class FileWebhookDeliveries {
  constructor(directory) {
    this.storage = new FileCompareAndSet(directory);
    this.ownership = new AsyncLocalStorage();
  }

  run(operation) {
    return this.ownership.run(new Map(), operation);
  }

  async claim(claim) {
    const ownership = this.ownership.getStore();
    if (ownership === undefined || ownership.has(claim.deliveryId)) throw new MiddlewareError("webhook-replay");
    if (!Number.isSafeInteger(claim.leaseUntilMs) || claim.leaseUntilMs <= Date.now()) {
      throw new MiddlewareError("invalid-webhook");
    }
    for (;;) {
      const current = await this.storage.read(claim.deliveryId);
      const stored = current?.value;
      if (stored !== undefined && stored.payloadDigest !== claim.payloadDigest) return "conflict";
      if (stored?.state === "completed") return "completed";
      if (stored?.state === "processing" && stored.leaseUntilMs > Date.now()) return "processing";
      if (claim.leaseUntilMs <= Date.now()) return "processing";
      const token = crypto.randomUUID();
      if (await this.storage.compareAndSet(claim.deliveryId, current?.version ?? -1,
        { ...claim, state: "processing", token }, claim.leaseUntilMs)) {
        ownership.set(claim.deliveryId, token);
        return "claimed";
      }
    }
  }

  async complete(deliveryId, payloadDigest) {
    await this.transition(deliveryId, payloadDigest, "completed");
  }

  async release(deliveryId, payloadDigest) {
    await this.transition(deliveryId, payloadDigest, "released");
  }

  async transition(deliveryId, payloadDigest, state) {
    const token = this.ownership.getStore()?.get(deliveryId);
    if (token === undefined) throw new MiddlewareError("webhook-replay");
    const current = await this.storage.read(deliveryId);
    const stored = current?.value;
    if (stored === undefined || stored.token !== token || stored.payloadDigest !== payloadDigest
      || stored.state !== "processing" || stored.leaseUntilMs <= Date.now()
      || !await this.storage.compareAndSet(deliveryId, current.version, { ...stored, state }, stored.leaseUntilMs)) {
      throw new MiddlewareError("webhook-replay");
    }
  }
}

export class MerchantCredentials {
  constructor(entries) {
    if (!Array.isArray(entries) || entries.length === 0) throw new Error("missing_merchant_credentials");
    const tokens = new Set();
    this.entries = entries.map((entry) => {
      const value = exactObject(entry);
      if (typeof value.token !== "string" || !/^[A-Za-z0-9._~-]{32,512}$/u.test(value.token)
        || tokens.has(value.token)
        || typeof value.tenant !== "string" || !/^[A-Za-z0-9._-]{1,128}$/u.test(value.tenant)
        || typeof value.principal !== "string" || !/^[A-Za-z0-9._-]{1,128}$/u.test(value.principal)) {
        throw new Error("invalid_merchant_credentials");
      }
      tokens.add(value.token);
      return Object.freeze({ digest: createHash("sha256").update(value.token).digest(),
        identity: Object.freeze({ tenant: value.tenant, principal: value.principal }) });
    });
  }

  authenticate(request) {
    const authorization = request.headers.authorization;
    if (typeof authorization !== "string" || !/^Bearer [A-Za-z0-9._~-]{32,512}$/u.test(authorization)) return undefined;
    const digest = createHash("sha256").update(authorization.slice(7)).digest();
    let identity;
    for (const entry of this.entries) {
      if (timingSafeEqual(digest, entry.digest)) identity = entry.identity;
    }
    return identity;
  }
}

export function scopedCheckoutKey(identity, key) {
  if (typeof key !== "string" || !/^[A-Za-z0-9._-]{1,255}$/u.test(key)) throw new MerchantError("invalid-cart");
  return createHash("sha256").update(JSON.stringify([identity.tenant, identity.principal, key])).digest("hex");
}

const parseWebhookKeys = (value) => {
  const input = exactObject(JSON.parse(value));
  const result = {};
  for (const [keyId, publicKey] of Object.entries(input)) {
    if (!/^[A-Za-z0-9._-]{1,64}$/u.test(keyId)) throw new Error("invalid_webhook_key_id");
    result[keyId] = hex32(publicKey);
  }
  if (Object.keys(result).length === 0) throw new Error("missing_webhook_key");
  return result;
};

const readBody = async (request) => {
  const chunks = [];
  let size = 0;
  for await (const chunk of request) {
    size += chunk.length;
    if (size > 64 * 1024) throw new Error("request_too_large");
    chunks.push(chunk);
  }
  return Uint8Array.from(Buffer.concat(chunks));
};

const singleHeader = (request, name) => {
  const value = request.headers[name];
  if (typeof value !== "string") throw new MiddlewareError("invalid-webhook");
  return value;
};

export function createMerchantApplicationServer(config) {
  const token = requiredEnvironment(config.tokenEnvironment);
  const credentials = new MerchantCredentials(JSON.parse(requiredEnvironment(config.customerCredentialsEnvironment)));
  const state = applicationStatePath(config, config.stateDirectory);
  const orders = new FileOrders(join(state, "orders"));
  const receiptKeys = receiptKeysFromEnvironment(config.receiptKeysEnvironment);
  const receipts = new TransactionReceipts(config.rpcUrl, receiptKeys, config.transportLimits);
  const authority = new SettlementAuthority(config.settlementUrl, token, receipts, config.transportLimits);
  const fulfillments = new FileFulfillments(join(state, "fulfillments"));
  const merchantFor = (identity) => new MerchantCheckout({
    catalog: new DeclaredCatalog({
      sku: "metered-report",
      title: "Receipt-backed market report",
      unitAmount: requiredEnvironment(config.priceEnvironment),
      asset: requiredEnvironment(config.assetEnvironment),
      payTo: requiredEnvironment(config.payToEnvironment),
      scheme: config.scheme,
      network: "eip155:125",
      maxTimeoutSeconds: 60,
      extra: {
        layerx: {
          commitment: "receipted",
          account: requiredEnvironment(config.accountEnvironment),
          currency: requiredEnvironment(config.currencyEnvironment),
        },
      },
    }),
    orders: new FileOrders(join(state, "orders"), identity),
    sellers: { create: (paymentRequired) => new SellerMiddleware({ paymentRequired, receiptKeys, authority, fulfillments }) },
    resourceUrl: (checkoutKey) => new URL(`/orders/${encodeURIComponent(checkoutKey)}`, config.publicUrl).toString(),
  });
  const deliveries = new FileWebhookDeliveries(join(state, "webhook-deliveries"));
  const webhook = new MerchantSettlementWebhooks({
    verifier: new VerifiedWebhookConsumer({
      publicKeys: parseWebhookKeys(requiredEnvironment(config.webhookKeysEnvironment)),
      deliveries,
      leaseMs: config.webhookLeaseMs,
    }),
    orders,
    receipts,
    receiptKeys,
  });
  return createServer(async (request, response) => {
    try {
      const protectedRoute = (request.method === "GET" && request.url?.startsWith("/orders/"))
        || (request.method === "POST" && request.url === "/checkout");
      const identity = protectedRoute ? credentials.authenticate(request) : undefined;
      if (protectedRoute && identity === undefined) {
        response.writeHead(401, { "content-type": "application/json", "www-authenticate": "Bearer" })
          .end(JSON.stringify({ state: "refused", error: "authentication_required" }));
        return;
      }
      if (request.method === "GET" && request.url?.startsWith("/orders/")) {
        const stored = await orders.get(decodeURIComponent(request.url.slice(8)));
        const order = stored?.tenant === identity.tenant && stored?.principal === identity.principal ? stored : undefined;
        response.writeHead(order === undefined ? 404 : 200, { "content-type": "application/json" })
          .end(JSON.stringify(order ?? { state: "missing" }));
        return;
      }
      if (request.method === "POST" && request.url === "/webhooks/settlement") {
        const raw = await readBody(request);
        const result = await deliveries.run(() => webhook.consume(raw, {
          id: singleHeader(request, "layerx-webhook-id"),
          timestamp: singleHeader(request, "layerx-webhook-timestamp"),
          keyId: singleHeader(request, "layerx-webhook-key-id"),
          signature: singleHeader(request, "layerx-webhook-signature"),
        }));
        response.writeHead(result === "processing" ? 202 : 200, { "content-type": "application/json" })
          .end(JSON.stringify({ state: result }));
        return;
      }
      if (request.method !== "POST" || request.url !== "/checkout") {
        response.writeHead(404).end();
        return;
      }
      const body = exactObject(JSON.parse(new TextDecoder("utf-8", { fatal: true }).decode(await readBody(request))));
      if ((body.principal !== undefined && body.principal !== identity.principal)
        || (body.tenant !== undefined && body.tenant !== identity.tenant)) {
        response.writeHead(403, { "content-type": "application/json" })
          .end(JSON.stringify({ state: "refused", error: "identity_mismatch" }));
        return;
      }
      const result = await merchantFor(identity).checkout(
        JSON.stringify([identity.tenant, identity.principal]),
        scopedCheckoutKey(identity, body.checkout_key),
        body.lines,
        typeof request.headers["payment-signature"] === "string" ? request.headers["payment-signature"] : undefined,
      );
      const status = result.kind === "payment-required" ? 402 : result.kind === "pending" ? 202 : result.kind === "paid" ? 200 : 402;
      const headers = result.kind === "payment-required"
        ? result.decision.headers
        : result.kind === "paid"
          ? { "payment-response": encodeSettlementHeader(result.settlement) }
          : result.kind === "pending" ? { "retry-after": "1" } : {};
      response.writeHead(status, { "content-type": "application/json", ...headers })
        .end(JSON.stringify({ state: result.kind, order: result.order }));
    } catch (error) {
      const state = error instanceof LayerXApplicationStateError
        ? error.state
        : error instanceof MiddlewareError && error.code === "payment-pending"
          ? "pending"
          : error instanceof MiddlewareError && error.code === "fulfillment-conflict"
            ? "unknown"
            : error instanceof MiddlewareError || error instanceof MerchantError || error instanceof SyntaxError
          ? "refused"
          : "unknown";
      const status = state === "pending" ? 202 : state === "refused" ? 400 : 503;
      response.writeHead(status, {
        "content-type": "application/json",
        ...(state === "pending" || state === "unknown" ? { "retry-after": "1" } : {}),
      }).end(JSON.stringify({ state, error: error.code ?? error.message }));
    }
  });
}

export async function runMerchantApplication(moduleUrl, application) {
  const config = await loadApplicationConfig(moduleUrl, application);
  const port = Number(config.port);
  if (!Number.isSafeInteger(port) || port <= 0 || port > 65535) throw new Error("invalid_port");
  return createMerchantApplicationServer(config).listen(port, "127.0.0.1", () => {
    process.stdout.write(`${JSON.stringify({ application, environment: config.name, listening: port, checkout: "/checkout", webhook: "/webhooks/settlement" })}\n`);
  });
}
