import type { SequencerReceiptKey } from "@sidiora/layerx-sdk";
import type { ReceiptKeyEntry } from "./receipts.js";
import { MiddlewareError } from "@sidiora/layerx-seller-middleware";
import type {
  FulfillmentRepository,
  StoredFulfillment,
  WebhookClaimResult,
  WebhookDeliveryClaim,
  WebhookDeliveryStore,
} from "@sidiora/layerx-seller-middleware";

export class FixedReceiptKeys {
  readonly #keys: readonly SequencerReceiptKey[];

  public constructor(entries: readonly ReceiptKeyEntry[]) {
    this.#keys = entries.map((entry) => ({
      keyId: entry.keyId.slice(),
      publicKey: entry.publicKey.slice(),
      first_height: entry.firstHeight,
      last_height: entry.lastHeight,
    }));
  }

  public async resolve(): Promise<readonly SequencerReceiptKey[]> {
    return this.#keys.map((key) => ({ ...key, keyId: key.keyId.slice(), publicKey: key.publicKey.slice() }));
  }
}

/**
 * An in-memory, first-writer-wins fulfilment ledger. The release callback runs
 * at most once per idempotency key; matching callers share its result and a
 * digest mismatch is a conflict. Failed releases retain their claim because
 * their outcome may be uncertain. Restart checks must use durable storage.
 */
export class InMemoryFulfillmentRepository<T> implements FulfillmentRepository<T> {
  readonly #records = new Map<string, StoredFulfillment<T>>();
  readonly #claims = new Map<string, {
    requestDigest: string;
    pending: Promise<StoredFulfillment<T>> | undefined;
  }>();

  public releaseCount = 0;

  public async get(idempotencyKey: string): Promise<StoredFulfillment<T> | undefined> {
    return this.#records.get(idempotencyKey);
  }

  public async fulfill(
    proposed: Omit<StoredFulfillment<T>, "resource">,
    release: () => Promise<T>,
  ): Promise<StoredFulfillment<T>> {
    const existing = this.#records.get(proposed.idempotencyKey);
    if (existing !== undefined) {
      if (existing.requestDigest !== proposed.requestDigest) {
        throw new MiddlewareError("fulfillment-conflict");
      }
      return existing;
    }
    const existingClaim = this.#claims.get(proposed.idempotencyKey);
    if (existingClaim !== undefined) {
      if (existingClaim.requestDigest !== proposed.requestDigest) {
        throw new MiddlewareError("fulfillment-conflict");
      }
      if (existingClaim.pending === undefined) {
        throw new MiddlewareError("fulfillment-outcome-unknown");
      }
      return existingClaim.pending;
    }
    const claim = {
      requestDigest: proposed.requestDigest,
      pending: undefined as Promise<StoredFulfillment<T>> | undefined,
    };
    this.#claims.set(proposed.idempotencyKey, claim);
    claim.pending = Promise.resolve().then(async () => {
      try {
        const resource = await release();
        this.releaseCount += 1;
        const stored: StoredFulfillment<T> = { ...proposed, resource };
        this.#records.set(proposed.idempotencyKey, stored);
        this.#claims.delete(proposed.idempotencyKey);
        return stored;
      } catch (error) {
        claim.pending = undefined;
        throw error;
      }
    });
    return claim.pending;
  }
}

/** In-memory webhook delivery ledger enforcing single-flight, replay and conflict semantics. */
export class InMemoryDeliveryStore implements WebhookDeliveryStore {
  readonly #records = new Map<string, { digest: string; done: boolean }>();

  public async claim(value: WebhookDeliveryClaim): Promise<WebhookClaimResult> {
    const existing = this.#records.get(value.deliveryId);
    if (existing === undefined) {
      this.#records.set(value.deliveryId, { digest: value.payloadDigest, done: false });
      return "claimed";
    }
    if (existing.digest !== value.payloadDigest) {
      return "conflict";
    }
    return existing.done ? "completed" : "processing";
  }

  public async complete(deliveryId: string, payloadDigest: string): Promise<void> {
    const record = this.#records.get(deliveryId);
    if (record !== undefined && record.digest === payloadDigest) {
      record.done = true;
    }
  }

  public async release(deliveryId: string, payloadDigest: string): Promise<void> {
    const record = this.#records.get(deliveryId);
    if (record !== undefined && record.digest === payloadDigest && !record.done) {
      this.#records.delete(deliveryId);
    }
  }
}

export interface CheckResult {
  readonly name: string;
  readonly ok: boolean;
  readonly detail?: string;
}

/** A minimal assertion harness that records outcomes rather than throwing eagerly, so a full run reports every failure. */
export class Suite {
  readonly #results: CheckResult[] = [];

  public async check(name: string, body: () => Promise<void> | void): Promise<void> {
    try {
      await body();
      this.#results.push({ name, ok: true });
    } catch (error) {
      this.#results.push({ name, ok: false, detail: error instanceof Error ? error.message : String(error) });
    }
  }

  public results(): readonly CheckResult[] {
    return this.#results;
  }
}

export async function expectThrows(
  body: () => Promise<unknown> | unknown,
  predicate: (error: unknown) => boolean,
  description: string,
): Promise<void> {
  try {
    await body();
  } catch (error) {
    if (!predicate(error)) {
      throw new Error(`${description}: threw the wrong error: ${error instanceof Error ? error.message : String(error)}`);
    }
    return;
  }
  throw new Error(`${description}: expected a rejection but the call resolved`);
}

export function assert(condition: boolean, message: string): void {
  if (!condition) {
    throw new Error(message);
  }
}
