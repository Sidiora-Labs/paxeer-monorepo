import { mkdir, chmod, lstat, open } from "node:fs/promises";
import { resolve, join } from "node:path";
import { constants } from "node:fs";
import { DatabaseSync } from "node:sqlite";
import {
  MiddlewareError,
  decodePaymentCommitmentEvidence,
  encodePaymentCommitmentEvidence,
} from "@sidiora/layerx-seller-middleware";

const queues = new Map();

export class FileFulfillmentRepository {
  constructor(directory) {
    this.directory = resolve(directory);
  }

  async get(idempotencyKey) {
    if (!/^[0-9a-f]{64}$/u.test(idempotencyKey)) {
      throw new MiddlewareError("fulfillment-conflict");
    }
    const database = await this.openDatabase();
    try {
      const stored = database.prepare("SELECT request_digest, receipt, resource, evidence FROM fulfillment WHERE id = ?").get(idempotencyKey);
      // Digest-only records need a verified proposal before evidence can be backfilled.
      if (stored === undefined || stored.evidence === null) return undefined;
      return this.restore(idempotencyKey, stored);
    } finally {
      database.close();
    }
  }

  restore(idempotencyKey, stored) {
    const evidence = JSON.parse(stored.evidence);
    if (evidence.idempotencyKey !== idempotencyKey
      || evidence.requestDigest !== stored.request_digest
      || evidence.receiptDigest !== stored.receipt) {
      throw new MiddlewareError("fulfillment-conflict");
    }
    return {
      ...evidence,
      ...(evidence.commitmentEvidence === undefined ? {} : {
        commitmentEvidence: decodePaymentCommitmentEvidence(evidence.commitmentEvidence),
      }),
      resource: JSON.parse(stored.resource),
    };
  }

  async fulfill(proposed, release) {
    return this.enqueue(proposed, release, false);
  }

  async reconcile(proposed, resolve) {
    return this.enqueue(proposed, resolve, true);
  }

  async enqueue(proposed, release, reconciliation) {
    const before = queues.get(this.directory) ?? Promise.resolve();
    const operation = before.then(() => this.commit(proposed, release, reconciliation));
    const barrier = operation.then(() => undefined, () => undefined);
    queues.set(this.directory, barrier);
    try {
      return await operation;
    } finally {
      if (queues.get(this.directory) === barrier) queues.delete(this.directory);
    }
  }

  async openDatabase() {
    await mkdir(this.directory, { recursive: true, mode: 0o700 });
    const directory = await lstat(this.directory);
    if (!directory.isDirectory() || directory.isSymbolicLink() || (directory.mode & 0o777) !== 0o700) {
      throw new MiddlewareError("fulfillment-conflict");
    }
    const path = join(this.directory, "fulfillments.sqlite");
    const file = await open(path, constants.O_RDWR | constants.O_CREAT | constants.O_NOFOLLOW, 0o600);
    await file.close();
    const metadata = await lstat(path);
    if (!metadata.isFile() || metadata.isSymbolicLink() || metadata.nlink !== 1) {
      throw new MiddlewareError("fulfillment-conflict");
    }
    await chmod(path, 0o600);
    const database = new DatabaseSync(path);
    try {
      database.exec("PRAGMA journal_mode=WAL; PRAGMA synchronous=FULL; PRAGMA busy_timeout=30000;");
      database.exec("CREATE TABLE IF NOT EXISTS fulfillment (id TEXT PRIMARY KEY, request_digest TEXT NOT NULL, receipt TEXT NOT NULL, resource TEXT NOT NULL, evidence TEXT)");
      if (!database.prepare("PRAGMA table_info(fulfillment)").all().some(column => column.name === "evidence")) {
        database.exec("ALTER TABLE fulfillment ADD COLUMN evidence TEXT");
      }
      database.exec("CREATE TABLE IF NOT EXISTS fulfillment_claim (id TEXT PRIMARY KEY, request_digest TEXT NOT NULL, receipt TEXT NOT NULL)");
      return database;
    } catch (error) {
      database.close();
      throw error;
    }
  }

  async commit(proposed, release, reconciliation) {
    if (!/^[0-9a-f]{64}$/u.test(proposed.idempotencyKey)
      || !/^[0-9a-f]{64}$/u.test(proposed.requestDigest)
      || !/^0x[0-9a-f]{64}$/u.test(proposed.receiptDigest)) {
      throw new MiddlewareError("fulfillment-conflict");
    }
    const evidence = JSON.stringify({
      ...proposed,
      ...(proposed.commitmentEvidence === undefined ? {} : {
        commitmentEvidence: encodePaymentCommitmentEvidence(proposed.commitmentEvidence),
      }),
    });
    const database = await this.openDatabase();
    let transaction = false;
    try {
      database.exec("BEGIN IMMEDIATE");
      transaction = true;
      let stored = database.prepare("SELECT request_digest, receipt, resource, evidence FROM fulfillment WHERE id = ?").get(proposed.idempotencyKey);
      if (stored === undefined) {
        const legacyPath = join(this.directory, `${proposed.idempotencyKey}.json`);
        try {
          const legacyFile = await open(legacyPath, constants.O_RDONLY | constants.O_NOFOLLOW);
          let legacy;
          try {
            const metadata = await legacyFile.stat();
            if (!metadata.isFile() || metadata.nlink !== 1 || (metadata.mode & 0o777) !== 0o600) {
              throw new MiddlewareError("fulfillment-conflict");
            }
            legacy = JSON.parse(await legacyFile.readFile("utf8"));
          } finally {
            await legacyFile.close();
          }
          if (legacy.requestDigest !== proposed.requestDigest
            || legacy.receipt !== proposed.receiptDigest
            || legacy.resource === undefined) throw new MiddlewareError("fulfillment-conflict");
          stored = { request_digest: legacy.requestDigest, receipt: legacy.receipt, resource: JSON.stringify(legacy.resource), evidence };
          database.prepare("INSERT INTO fulfillment (id, request_digest, receipt, resource, evidence) VALUES (?, ?, ?, ?, ?)").run(
            proposed.idempotencyKey, stored.request_digest, stored.receipt, stored.resource, evidence,
          );
        } catch (error) {
          if (error.code !== "ENOENT") throw error;
        }
      }
      if (stored !== undefined) {
        if (stored.request_digest !== proposed.requestDigest
          || stored.receipt !== proposed.receiptDigest) {
          throw new MiddlewareError("fulfillment-conflict");
        }
        if (stored.evidence === null) {
          database.prepare("UPDATE fulfillment SET evidence = ? WHERE id = ?").run(evidence, proposed.idempotencyKey);
          stored.evidence = evidence;
        }
        const restored = this.restore(proposed.idempotencyKey, stored);
        if (restored.txHash !== proposed.txHash) throw new MiddlewareError("fulfillment-conflict");
        database.exec("COMMIT");
        transaction = false;
        return restored;
      }
      const receipt = proposed.receiptDigest;
      const claim = database.prepare("SELECT request_digest, receipt FROM fulfillment_claim WHERE id = ?").get(proposed.idempotencyKey);
      if (claim !== undefined) {
        if (claim.request_digest !== proposed.requestDigest || claim.receipt !== receipt) {
          throw new MiddlewareError("fulfillment-conflict");
        }
        if (!reconciliation) throw new MiddlewareError("fulfillment-outcome-unknown");
      } else {
        if (reconciliation) throw new MiddlewareError("fulfillment-conflict");
        database.prepare("INSERT INTO fulfillment_claim VALUES (?, ?, ?)").run(
          proposed.idempotencyKey, proposed.requestDigest, receipt,
        );
      }
      database.exec("COMMIT");
      transaction = false;
      const parent = await open(this.directory, constants.O_RDONLY | constants.O_DIRECTORY);
      try { await parent.sync(); } finally { await parent.close(); }
      const resource = await release();
      const encoded = JSON.stringify(resource);
      if (encoded === undefined) throw new MiddlewareError("fulfillment-outcome-unknown");
      database.exec("BEGIN IMMEDIATE");
      transaction = true;
      const completed = database.prepare("SELECT request_digest, receipt, resource FROM fulfillment WHERE id = ?").get(proposed.idempotencyKey);
      if (completed !== undefined) {
        if (completed.request_digest !== proposed.requestDigest || completed.receipt !== receipt
          || completed.resource !== encoded) throw new MiddlewareError("fulfillment-conflict");
      } else {
        database.prepare("INSERT INTO fulfillment (id, request_digest, receipt, resource, evidence) VALUES (?, ?, ?, ?, ?)").run(
          proposed.idempotencyKey, proposed.requestDigest, receipt, encoded, evidence,
        );
      }
      database.prepare("DELETE FROM fulfillment_claim WHERE id = ?").run(proposed.idempotencyKey);
      database.exec("COMMIT");
      transaction = false;
      return { ...proposed, resource };
    } catch (error) {
      if (transaction) database.exec("ROLLBACK");
      throw error;
    } finally {
      database.close();
    }
  }
}
