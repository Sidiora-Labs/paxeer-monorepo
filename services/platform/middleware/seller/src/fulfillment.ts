import { mkdir, open, readFile, rename } from "node:fs/promises";
import { join } from "node:path";
import { randomUUID } from "node:crypto";
import { MiddlewareError, type FulfillmentRepository, type StoredFulfillment } from "./index.js";
import { decodePaymentCommitmentEvidence, encodePaymentCommitmentEvidence } from "./commitment.js";

/** A durable claim precedes release. An interrupted release needs application reconciliation. */
export class DurableFulfillmentRepository<T> implements FulfillmentRepository<T> {
  public constructor(private readonly directory: string) {}
  private path(key: string): string {
    if (!/^[0-9a-f]{64}$/u.test(key)) throw new MiddlewareError("fulfillment-conflict");
    return join(this.directory, key);
  }
  public async get(key: string): Promise<StoredFulfillment<T> | undefined> {
    const directory = this.path(key);
    let text: string;
    try { text = await readFile(join(directory, "fulfilled.json"), "utf8"); }
    catch (error) {
      if ((error as NodeJS.ErrnoException).code !== "ENOENT") throw error;
      return undefined;
    }
    const record = JSON.parse(text) as StoredFulfillment<T>;
    if (record.idempotencyKey !== key) throw new MiddlewareError("fulfillment-conflict");
    return { ...record, ...(record.commitmentEvidence === undefined ? {} : {
      commitmentEvidence: decodePaymentCommitmentEvidence(record.commitmentEvidence),
    }) };
  }
  public async fulfill(proposed: Omit<StoredFulfillment<T>, "resource">, release: () => Promise<T>): Promise<StoredFulfillment<T>> {
    const directory = this.path(proposed.idempotencyKey);
    await mkdir(this.directory, { recursive: true, mode: 0o700 });
    try { await mkdir(directory, { mode: 0o700 }); }
    catch (error) {
      if ((error as NodeJS.ErrnoException).code !== "EEXIST") throw error;
      const existing = await this.get(proposed.idempotencyKey);
      if (existing !== undefined) {
        if (existing.requestDigest !== proposed.requestDigest) throw new MiddlewareError("fulfillment-conflict");
        return existing;
      }
      try {
        const claim = JSON.parse(await readFile(join(directory, "claim.json"), "utf8")) as { requestDigest: string };
        if (claim.requestDigest !== proposed.requestDigest) throw new MiddlewareError("fulfillment-conflict");
      } catch (error) {
        if (error instanceof MiddlewareError) throw error;
        if ((error as NodeJS.ErrnoException).code !== "ENOENT") throw error;
      }
      throw new MiddlewareError("fulfillment-outcome-unknown");
    }
    await this.write(join(directory, "claim.json"), JSON.stringify({ requestDigest: proposed.requestDigest }));
    await this.sync(directory);
    await this.sync(this.directory);
    const resource = await release();
    const record = { ...proposed, resource };
    const temporary = join(directory, `${randomUUID()}.tmp`);
    await this.write(temporary, JSON.stringify({ ...record, ...(record.commitmentEvidence === undefined ? {} : {
      commitmentEvidence: encodePaymentCommitmentEvidence(record.commitmentEvidence),
    }) }));
    await rename(temporary, join(directory, "fulfilled.json"));
    await this.sync(directory);
    return structuredClone(record);
  }
  private async write(path: string, text: string): Promise<void> {
    const file = await open(path, "wx", 0o600);
    try { await file.writeFile(text, "utf8"); await file.sync(); } finally { await file.close(); }
  }
  private async sync(path: string): Promise<void> {
    const directory = await open(path, "r");
    try { await directory.sync(); } finally { await directory.close(); }
  }
}
