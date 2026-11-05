import { pathToFileURL } from "node:url";
import { createServer } from "node:http";
import { randomUUID } from "node:crypto";
import { link, mkdir, open, readFile, rm } from "node:fs/promises";
import { dirname, join, resolve } from "node:path";
import {
  decodePaymentCommitmentEvidence,
  encodePaymentCommitmentEvidence,
  MiddlewareError,
  PAYMENT_SIGNATURE_HEADER,
  SellerMiddleware,
} from "@sidiora/layerx-seller-middleware";
import {
  LayerXApplicationStateError,
  SequencerReceiptClient,
  applicationStatePath,
  exactObject,
  loadApplicationConfig,
  receiptKeysFromEnvironment,
  requiredEnvironment,
} from "../support/runtime.mjs";

export function platform_ref_seller() {
  return "seller-middleware-live-receipt-authority-paid-api";
}

const safeName = (value) => {
  if (!/^[A-Za-z0-9._-]{1,255}$/u.test(value)) throw new Error("invalid_identifier");
  return value;
};

const syncDirectory = async (path) => {
  const directory = await open(dirname(path), "r");
  try {
    await directory.sync();
  } finally {
    await directory.close();
  }
};

const fulfillmentOperations = new Map();

export class FileFulfillmentRepository {
  constructor(directory) {
    this.directory = resolve(directory);
  }

  async serialize(idempotencyKey, operation) {
    const key = join(this.directory, safeName(idempotencyKey));
    const previous = fulfillmentOperations.get(key) ?? Promise.resolve();
    const current = previous.then(operation, operation);
    fulfillmentOperations.set(key, current);
    try {
      return await current;
    } finally {
      if (fulfillmentOperations.get(key) === current) fulfillmentOperations.delete(key);
    }
  }

  async get(idempotencyKey) {
    return this.serialize(idempotencyKey, () => this.recover(idempotencyKey));
  }

  async recover(idempotencyKey) {
    const path = join(this.directory, `${safeName(idempotencyKey)}.json`);
    const existing = await this.readOptional(path);
    if (existing !== undefined) {
      this.assertRecord(existing, idempotencyKey);
      return existing;
    }
    const claimDirectory = join(this.directory, `${safeName(idempotencyKey)}.claim`);
    const ready = await this.readOptional(join(claimDirectory, "ready.json"));
    if (ready === undefined) return undefined;
    const claim = await this.read(join(claimDirectory, "claim.json"));
    this.assertRecord(ready, idempotencyKey, claim.requestDigest);
    if (JSON.stringify(this.encode(claim)) !== JSON.stringify(this.encode({ ...ready, resource: undefined }))) {
      throw new MiddlewareError("fulfillment-conflict");
    }
    return this.publish(idempotencyKey, ready);
  }

  async fulfill(proposed, release) {
    return this.serialize(proposed.idempotencyKey, async () => {
      const existing = await this.recover(proposed.idempotencyKey);
      if (existing !== undefined) {
        this.assertRecord(existing, proposed.idempotencyKey, proposed.requestDigest);
        return existing;
      }
      await mkdir(this.directory, { recursive: true, mode: 0o700 });
      await syncDirectory(this.directory);
      const claimDirectory = join(this.directory, `${safeName(proposed.idempotencyKey)}.claim`);
      try {
        await mkdir(claimDirectory, { mode: 0o700 });
      } catch (error) {
        if (error.code !== "EEXIST") throw error;
        const claim = await this.readOptional(join(claimDirectory, "claim.json"));
        if (claim !== undefined && claim.requestDigest !== proposed.requestDigest) {
          throw new MiddlewareError("fulfillment-conflict");
        }
        throw new MiddlewareError("fulfillment-outcome-unknown");
      }
      await syncDirectory(claimDirectory);
      await this.writeAtomic(join(claimDirectory, "claim.json"), proposed);
      const resource = await release();
      const record = { ...proposed, resource };
      this.assertRecord(record, proposed.idempotencyKey, proposed.requestDigest);
      await this.writeAtomic(join(claimDirectory, "ready.json"), record);
      return this.publish(proposed.idempotencyKey, record);
    });
  }

  async reconcile(idempotencyKey, requestDigest, resource) {
    return this.serialize(idempotencyKey, async () => {
      const existing = await this.recover(idempotencyKey);
      if (existing !== undefined) {
        this.assertRecord(existing, idempotencyKey, requestDigest);
        return existing;
      }
      const claimDirectory = join(this.directory, `${safeName(idempotencyKey)}.claim`);
      const claim = await this.read(join(claimDirectory, "claim.json"));
      if (claim.idempotencyKey !== idempotencyKey || claim.requestDigest !== requestDigest) {
        throw new MiddlewareError("fulfillment-conflict");
      }
      const record = { ...claim, resource };
      this.assertRecord(record, idempotencyKey, requestDigest);
      await this.writeAtomic(join(claimDirectory, "ready.json"), record);
      return this.publish(idempotencyKey, record);
    });
  }

  async publish(idempotencyKey, record) {
    const claimDirectory = join(this.directory, `${safeName(idempotencyKey)}.claim`);
    const path = join(this.directory, `${safeName(idempotencyKey)}.json`);
    try {
      await link(join(claimDirectory, "ready.json"), path);
    } catch (error) {
      if (error.code !== "EEXIST") throw error;
      const existing = await this.read(path);
      if (JSON.stringify(this.encode(existing)) !== JSON.stringify(this.encode(record))) {
        throw new MiddlewareError("fulfillment-conflict");
      }
    }
    await syncDirectory(path);
    await rm(claimDirectory, { recursive: true, force: true });
    await syncDirectory(path);
    return structuredClone(record);
  }

  assertRecord(record, idempotencyKey, requestDigest = record.requestDigest) {
    if (record.idempotencyKey !== idempotencyKey || record.requestDigest !== requestDigest
      || !Object.hasOwn(record, "resource") || record.resource === undefined) {
      throw new MiddlewareError("fulfillment-conflict");
    }
  }

  encode(record) {
    return { ...record, ...(record.commitmentEvidence === undefined ? {} : {
      commitmentEvidence: encodePaymentCommitmentEvidence(record.commitmentEvidence),
    }) };
  }

  async writeAtomic(path, record) {
    const temporary = join(dirname(path), `${randomUUID()}.tmp`);
    const file = await open(temporary, "wx", 0o600);
    try {
      await file.writeFile(JSON.stringify(this.encode(record)), "utf8");
      await file.sync();
    } finally {
      await file.close();
    }
    try {
      await link(temporary, path);
      await syncDirectory(path);
    } finally {
      await rm(temporary, { force: true });
    }
  }

  async readOptional(path) {
    try {
      return await this.read(path);
    } catch (error) {
      if (error.code !== "ENOENT") throw error;
      return undefined;
    }
  }

  async read(path) {
    const record = exactObject(JSON.parse(await readFile(path, "utf8")));
    return { ...record, ...(record.commitmentEvidence === undefined ? {} : {
      commitmentEvidence: decodePaymentCommitmentEvidence(record.commitmentEvidence),
    }) };
  }
}

class StatePreservingReceiptAuthority {
  constructor(resolver) {
    this.resolver = resolver;
  }

  async settle(request) {
    try {
      const receipt = await this.resolver.resolve(request.payload.payload.rawTransaction);
      return { kind: "settled", ...receipt };
    } catch (error) {
      if (!(error instanceof LayerXApplicationStateError)) throw error;
      if (error.state === "pending") return { kind: "pending" };
      if (error.state === "refused") return { kind: "refused", reason: error.message };
      throw error;
    }
  }
}

export function createPaidApiServer(middleware, resourceBody) {
  return createServer(async (request, response) => {
    try {
      if (request.method !== "GET" || request.url !== "/paid") {
        response.writeHead(404).end();
        return;
      }
      const header = request.headers[PAYMENT_SIGNATURE_HEADER.toLowerCase()];
      const paymentHeader = Array.isArray(header) ? undefined : header;
      const decision = await middleware.handle("public-paid-api", paymentHeader, async () => resourceBody);
      const headers = decision.kind === "payment-required" || decision.kind === "refused" || decision.kind === "released"
        ? decision.headers
        : { "retry-after": "1" };
      response.writeHead(decision.status, { "content-type": "application/json", ...headers });
      response.end(JSON.stringify(decision.kind === "payment-required"
        ? decision.body
        : decision.kind === "released"
          ? decision.resource
          : { state: decision.kind }));
    } catch (error) {
      if (error instanceof LayerXApplicationStateError) {
        const status = error.state === "pending" ? 202 : error.state === "refused" ? 402 : 503;
        response.writeHead(status, {
          "content-type": "application/json",
          ...(error.state === "pending" || error.state === "unknown" ? { "retry-after": "1" } : {}),
        }).end(JSON.stringify({ state: error.state, detail: error.message }));
        return;
      }
      const state = error instanceof MiddlewareError
        ? error.code === "payment-pending"
          ? "pending"
          : (error.code === "fulfillment-conflict" || error.code === "fulfillment-outcome-unknown")
            ? "unknown"
            : "refused"
        : "unknown";
      const status = state === "pending" ? 202 : state === "refused" ? 400 : 503;
      response.writeHead(status, {
        "content-type": "application/json",
        ...(state === "pending" || state === "unknown" ? { "retry-after": "1" } : {}),
      }).end(JSON.stringify({ state, error: error instanceof MiddlewareError ? error.code : "internal-fault" }));
    }
  });
}

export async function runPaidApiApplication() {
  const config = await loadApplicationConfig(import.meta.url, "paid-api");
  const resourceBody = exactObject(JSON.parse(await readFile(resolve(config.directory, config.resourceFile), "utf8")));
  const receiptKeys = receiptKeysFromEnvironment(config.receiptKeysEnvironment);
  const resolver = new SequencerReceiptClient(config.rpcUrl, receiptKeys);
  const middleware = new SellerMiddleware({
    receiptKeys,
    paymentRequired: {
      x402Version: 2,
      resource: {
        url: config.resourceUrl,
        description: "Receipt-verified metered weather",
        mimeType: "application/json",
        serviceName: "LayerX paid API",
      },
      accepts: [{
        scheme: config.scheme,
        network: config.network,
        amount: requiredEnvironment(config.priceEnvironment),
        asset: requiredEnvironment(config.assetEnvironment),
        payTo: requiredEnvironment(config.payToEnvironment),
        maxTimeoutSeconds: 60,
        extra: {
          layerx: {
            commitment: "receipted",
            account: requiredEnvironment(config.accountEnvironment),
            currency: requiredEnvironment(config.currencyEnvironment),
          },
        },
      }],
    },
    authority: new StatePreservingReceiptAuthority(resolver),
    fulfillments: new FileFulfillmentRepository(applicationStatePath(config, config.fulfillmentDirectory)),
  });

  const port = Number(config.port);
  if (!Number.isSafeInteger(port) || port <= 0 || port > 65535) throw new Error("invalid_port");

  createPaidApiServer(middleware, resourceBody).listen(port, "127.0.0.1", () => {
    process.stdout.write(`${JSON.stringify({ application: "paid-api", environment: config.name, listening: port, path: "/paid" })}\n`);
  });
}

if (process.argv[1] !== undefined && import.meta.url === pathToFileURL(resolve(process.argv[1])).href) {
  await runPaidApiApplication();
}
