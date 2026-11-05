import { randomUUID } from 'node:crypto';
import { mkdir, open, readFile, readdir, rename, rmdir, stat, unlink } from 'node:fs/promises';
import path from 'node:path';

const queues = new Map<string, Promise<unknown>>();

export class DurableStoreCorruptError extends Error {
  readonly storePath: string;
  constructor(storePath: string) {
    super('Durable store is corrupt');
    this.name = 'DurableStoreCorruptError';
    this.storePath = storePath;
  }
}

export type StorePublicationPhase = 'temporary-synced' | 'backup-published' | 'primary-published';

export interface AtomicJsonStoreOptions<T> {
  readonly filePath: string;
  readonly empty: () => T;
  readonly parse: (input: unknown) => T;
  readonly migrate?: (input: unknown) => T;
  readonly maxBytes?: number;
  readonly onPublication?: (phase: StorePublicationPhase) => Promise<void>;
}

function enqueue<T>(key: string, operation: () => Promise<T>): Promise<T> {
  const previous = queues.get(key) ?? Promise.resolve();
  const current = previous.then(operation, operation);
  queues.set(key, current);
  const cleanup = () => {
    if (queues.get(key) === current) queues.delete(key);
  };
  void current.then(cleanup, cleanup);
  return current;
}

async function wait(milliseconds: number): Promise<void> {
  await new Promise<void>((resolve) => setTimeout(resolve, milliseconds));
}

export class AtomicJsonStore<T> {
  #ownerName?: string;
  readonly #filePath: string;
  readonly #lockPath: string;
  readonly #corruptionPath: string;
  readonly #empty: () => T;
  readonly #parse: (input: unknown) => T;
  readonly #migrate?: (input: unknown) => T;
  readonly #maxBytes: number;
  readonly #onPublication?: AtomicJsonStoreOptions<T>['onPublication'];

  constructor(options: AtomicJsonStoreOptions<T>) {
    this.#filePath = options.filePath;
    this.#lockPath = `${options.filePath}.lock`;
    this.#corruptionPath = `${options.filePath}.corrupt`;
    this.#empty = options.empty;
    this.#parse = options.parse;
    this.#migrate = options.migrate;
    this.#maxBytes = options.maxBytes ?? 5_242_880;
    this.#onPublication = options.onPublication;
  }

  async read(): Promise<T> {
    return enqueue(this.#filePath, () => this.#withFileLock(() => this.#readUnlocked()));
  }

  async update(mutator: (current: T) => T | Promise<T>): Promise<T> {
    return enqueue(this.#filePath, () =>
      this.#withFileLock(async () => {
        const current = await this.#readUnlocked();
        const next = await mutator(current);
        this.#parse(next);
        await this.#writeUnlocked(next);
        return next;
      })
    );
  }

  async recoverFromBackup(): Promise<T> {
    return enqueue(this.#filePath, () =>
      this.#withFileLock(async () => {
        await stat(this.#corruptionPath);
        let recovered: T;
        try {
          const backupPath = `${this.#filePath}.bak`;
          const metadata = await stat(backupPath);
          if (metadata.size > this.#maxBytes) throw new RangeError('Backup exceeds its size limit');
          const raw = await readFile(backupPath);
          if (raw.byteLength > this.#maxBytes)
            throw new RangeError('Backup exceeds its size limit');
          recovered = this.#parse(JSON.parse(raw.toString('utf8')));
        } catch {
          throw new DurableStoreCorruptError(this.#filePath);
        }
        // Keep both the marker and the verified backup until the primary is published.
        await this.#writeUnlocked(recovered, true);
        await unlink(this.#corruptionPath);
        await this.#syncDirectory();
        return recovered;
      })
    );
  }

  async #withFileLock<R>(operation: () => Promise<R>): Promise<R> {
    await mkdir(path.dirname(this.#filePath), { recursive: true });
    const release = await this.#acquireFileLock();
    try {
      return await operation();
    } finally {
      await release();
    }
  }

  async #assertAvailable(): Promise<void> {
    try {
      await stat(this.#corruptionPath);
    } catch (error) {
      if (error instanceof Error && 'code' in error && error.code === 'ENOENT') return;
      throw error;
    }
    throw new DurableStoreCorruptError(this.#filePath);
  }

  async #readUnlocked(): Promise<T> {
    await this.#assertAvailable();
    try {
      const metadata = await stat(this.#filePath);
      if (metadata.size > this.#maxBytes) {
        await this.#quarantine();
        throw new DurableStoreCorruptError(this.#filePath);
      }
      const raw = await readFile(this.#filePath, 'utf8');
      const input: unknown = JSON.parse(raw);
      try {
        return this.#parse(input);
      } catch {
        if (!this.#migrate) throw new Error('No migration');
        const migrated = this.#migrate(input);
        this.#parse(migrated);
        await this.#writeUnlocked(migrated);
        return migrated;
      }
    } catch (error) {
      if (error instanceof Error && 'code' in error && error.code === 'ENOENT') {
        return this.#empty();
      }
      if (error instanceof DurableStoreCorruptError) throw error;
      await this.#quarantine();
      throw new DurableStoreCorruptError(this.#filePath);
    }
  }

  async #writeUnlocked(value: T, recovering = false): Promise<void> {
    await this.#assertOwnership();
    if (!recovering) await this.#assertAvailable();
    const serialized = JSON.stringify(value);
    if (Buffer.byteLength(serialized) > this.#maxBytes) {
      throw new RangeError('Durable store exceeds its size limit');
    }
    await mkdir(path.dirname(this.#filePath), { recursive: true });
    // Publish a synced previous generation before replacing the primary. Recovery
    // never rotates the backup: the marker remains until the restored primary is durable.
    const temporary = await this.#syncedTemporary(serialized);
    try {
      await this.#onPublication?.('temporary-synced');
      if (!recovering) {
        let previous: string | undefined;
        try {
          const input: unknown = JSON.parse(await readFile(this.#filePath, 'utf8'));
          try {
            previous = JSON.stringify(this.#parse(input));
          } catch {
            if (!this.#migrate) throw new Error('No backup migration');
            previous = JSON.stringify(this.#parse(this.#migrate(input)));
          }
        } catch (error) {
          if (!hasCode(error, 'ENOENT')) throw error;
        }
        if (previous !== undefined) {
          const backupTemporary = await this.#syncedTemporary(previous);
          try {
            await this.#assertOwnership();
            await rename(backupTemporary, `${this.#filePath}.bak`);
            await this.#syncDirectory();
          } finally {
            await unlink(backupTemporary).catch((error) => {
              if (!hasCode(error, 'ENOENT')) throw error;
            });
          }
        }
      }
      await this.#onPublication?.('backup-published');
      await this.#assertOwnership();
      await rename(temporary, this.#filePath);
      await this.#syncDirectory();
      await this.#onPublication?.('primary-published');
    } finally {
      await unlink(temporary).catch((error) => {
        if (!hasCode(error, 'ENOENT')) throw error;
      });
    }
  }

  async #syncedTemporary(contents: string): Promise<string> {
    const temporary = `${this.#filePath}.${process.pid}.${randomUUID()}.tmp`;
    const handle = await open(temporary, 'wx', 0o600);
    try {
      await handle.writeFile(contents, 'utf8');
      await handle.sync();
    } finally {
      await handle.close();
    }
    return temporary;
  }

  async #syncDirectory(): Promise<void> {
    const directory = await open(path.dirname(this.#filePath), 'r');
    try {
      await directory.sync();
    } finally {
      await directory.close();
    }
  }

  async #assertOwnership(): Promise<void> {
    if (!this.#ownerName) throw new Error('Durable store lock lost');
    try {
      await stat(path.join(this.#lockPath, this.#ownerName));
    } catch {
      throw new Error('Durable store lock lost');
    }
  }

  async #quarantine(): Promise<void> {
    await this.#assertOwnership();
    const marker = await open(this.#corruptionPath, 'wx', 0o600);
    try {
      await marker.writeFile('Durable store requires verified backup recovery\n');
      await marker.sync();
      await this.#syncDirectory();
    } finally {
      await marker.close();
    }
    const target = `${this.#filePath}.corrupt-${randomUUID()}`;
    await rename(this.#filePath, target);
    await this.#syncDirectory();
  }

  async #acquireFileLock(): Promise<() => Promise<void>> {
    // A prepared, nonempty directory is published atomically. Reapers and late
    // releasers remove only their observed owner entry; rmdir cannot remove a
    // nonempty successor, even if it was published between unlink and rmdir.
    const token = randomUUID();
    const candidate = `${this.#lockPath}.${token}`;
    const ownerName = `owner-${token}`;
    await mkdir(candidate, { mode: 0o700 });
    const owner = await open(path.join(candidate, ownerName), 'wx', 0o600);
    try {
      await owner.writeFile(
        JSON.stringify({ pid: process.pid, start: await processIdentity(process.pid) })
      );
      await owner.sync();
    } finally {
      await owner.close();
    }
    const prepared = await open(candidate, 'r');
    try {
      await prepared.sync();
    } finally {
      await prepared.close();
    }
    try {
      for (let attempt = 0; attempt < 100; attempt += 1) {
        try {
          await rename(candidate, this.#lockPath);
          this.#ownerName = ownerName;
          return async () => {
            this.#ownerName = undefined;
            await unlink(path.join(this.#lockPath, ownerName)).catch((error) => {
              if (!hasCode(error, 'ENOENT')) throw error;
            });
            await removeEmptyLock(this.#lockPath);
          };
        } catch (error) {
          if (!['EEXIST', 'ENOTEMPTY', 'ENOTDIR'].some((code) => hasCode(error, code))) throw error;
        }
        let names: string[];
        try {
          names = await readdir(this.#lockPath);
        } catch (error) {
          if (hasCode(error, 'ENOENT')) continue;
          if (hasCode(error, 'ENOTDIR'))
            throw new Error('Unowned legacy durable store lock requires operator recovery');
          throw error;
        }
        for (const name of names) {
          if (!name.startsWith('owner-')) throw new Error('Invalid durable store lock');
          const ownerPath = path.join(this.#lockPath, name);
          let owner: { pid: number; start: string };
          try {
            owner = JSON.parse(await readFile(ownerPath, 'utf8'));
          } catch (error) {
            if (hasCode(error, 'ENOENT')) continue;
            throw error;
          }
          if (
            !Number.isSafeInteger(owner.pid) ||
            owner.pid <= 0 ||
            typeof owner.start !== 'string'
          ) {
            throw new Error('Invalid durable store lock owner');
          }
          if (!(await ownerIsAlive(owner))) {
            await unlink(ownerPath).catch((error) => {
              if (!hasCode(error, 'ENOENT')) throw error;
            });
          }
        }
        await removeEmptyLock(this.#lockPath);
        await wait(10 + attempt * 2);
      }
      throw new Error('Timed out acquiring durable store lock');
    } finally {
      await unlink(path.join(candidate, ownerName)).catch((error) => {
        if (!hasCode(error, 'ENOENT')) throw error;
      });
      await removeEmptyLock(candidate);
    }
  }
}

function hasCode(error: unknown, code: string): boolean {
  return error instanceof Error && 'code' in error && error.code === code;
}

async function removeEmptyLock(lockPath: string): Promise<void> {
  await rmdir(lockPath).catch((error) => {
    if (!['ENOENT', 'ENOTEMPTY', 'EEXIST'].some((code) => hasCode(error, code))) throw error;
  });
}

async function processIdentity(pid: number): Promise<string> {
  // Linux start ticks prevent PID reuse from preserving an abandoned lock.
  const status = await readFile(`/proc/${pid}/stat`, 'utf8');
  return status.slice(status.lastIndexOf(')') + 2).split(' ')[19];
}

async function ownerIsAlive(owner: { pid: number; start: string }): Promise<boolean> {
  try {
    return (await processIdentity(owner.pid)) === owner.start;
  } catch (error) {
    if (hasCode(error, 'ENOENT') || hasCode(error, 'ESRCH')) return false;
    throw error;
  }
}
