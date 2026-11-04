import { fork } from 'node:child_process';
import { once } from 'node:events';
import { mkdtemp, readFile, readdir, rename, rm, stat, utimes, writeFile } from 'node:fs/promises';
import os from 'node:os';
import path from 'node:path';
import { expect } from 'vitest';
import {
  AtomicJsonStore,
  DurableStoreCorruptError,
  type StorePublicationPhase,
} from './atomic-json-store';

export interface RecordState {
  version: 1;
  records: string[];
}
export function parseRecords(input: unknown): RecordState {
  if (
    typeof input !== 'object' ||
    input === null ||
    !('version' in input) ||
    input.version !== 1 ||
    !('records' in input) ||
    !Array.isArray(input.records) ||
    !input.records.every((item: unknown) => typeof item === 'string')
  )
    throw new TypeError('Invalid records');
  return { version: 1, records: input.records };
}
export function openRecords(filePath: string) {
  return new AtomicJsonStore<RecordState>({
    filePath,
    empty: () => ({ version: 1, records: [] }),
    parse: parseRecords,
  });
}

export async function scratchDirectory() {
  return mkdtemp(path.join(os.tmpdir(), 'repair-22-33-167-'));
}
export const workerPath = path.join(__dirname, 'fixtures/repair-22-33-167/worker.mjs');

export function worker(mode: string, filePath: string, parameter = '') {
  const child = fork(workerPath, [mode, filePath, parameter], { execArgv: [], silent: true });
  const messages: unknown[] = [];
  let stderr = '';
  child.stderr?.on('data', (chunk) => {
    stderr += chunk.toString();
  });
  child.on('message', (message) => messages.push(message));
  const exited = once(child, 'exit').then(([code, signal]) => ({ code, signal }));
  return {
    child,
    messages,
    exited,
    async message(expected: unknown) {
      const deadline = Date.now() + 8_000;
      while (!messages.some((message) => JSON.stringify(message) === JSON.stringify(expected))) {
        if (child.exitCode !== null || child.signalCode !== null)
          throw new Error(`Worker exited: ${stderr}; ${JSON.stringify(messages)}`);
        if (Date.now() >= deadline)
          throw new Error(
            `Worker message timeout: ${JSON.stringify(expected)}; ${stderr}; ${JSON.stringify(messages)}`
          );
        await new Promise((resolve) => setTimeout(resolve, 10));
      }
    },
    async stop() {
      if (child.exitCode === null && child.signalCode === null) child.kill('SIGKILL');
      await exited;
    },
  };
}

async function seed(filePath: string) {
  await openRecords(filePath).update(() => ({ version: 1, records: ['A'] }));
  await openRecords(filePath).update((current) => ({
    ...current,
    records: [...current.records, 'B'],
  }));
}

export async function store_corruption_reopen_refuses_empty_writes_and_verifies_backup() {
  const directory = await scratchDirectory();
  const filePath = path.join(directory, 'records.json');
  try {
    await seed(filePath);
    const backup = await readFile(`${filePath}.bak`, 'utf8');
    await writeFile(filePath, '{broken');
    await expect(openRecords(filePath).read()).rejects.toBeInstanceOf(DurableStoreCorruptError);
    let mutated = false;
    await expect(
      openRecords(filePath).update((current) => {
        mutated = true;
        return current;
      })
    ).rejects.toBeInstanceOf(DurableStoreCorruptError);
    expect(mutated).toBe(false);
    await expect(openRecords(filePath).read()).rejects.toBeInstanceOf(DurableStoreCorruptError);
    await writeFile(`${filePath}.bak`, '{"version":1,"records":[42]}');
    await expect(openRecords(filePath).recoverFromBackup()).rejects.toBeInstanceOf(
      DurableStoreCorruptError
    );
    await expect(openRecords(filePath).read()).rejects.toBeInstanceOf(DurableStoreCorruptError);
    await writeFile(`${filePath}.bak`, backup);
    expect(await openRecords(filePath).recoverFromBackup()).toEqual({ version: 1, records: ['A'] });
    expect(await openRecords(filePath).read()).toEqual({ version: 1, records: ['A'] });
    expect(await readFile(`${filePath}.bak`, 'utf8')).toBe(backup);
    await openRecords(filePath).update((current) => ({
      ...current,
      records: [...current.records, 'reopened'],
    }));
    expect((await openRecords(filePath).read()).records).toEqual(['A', 'reopened']);
  } finally {
    await rm(directory, { recursive: true, force: true });
  }
}

export async function store_migration_and_insertion_serialize_in_separate_processes() {
  const directory = await scratchDirectory();
  const filePath = path.join(directory, 'records.json');
  const release = path.join(directory, 'release');
  await writeFile(filePath, JSON.stringify(['legacy']));
  const migration = worker('migrate', filePath, release);
  let insertion: ReturnType<typeof worker> | undefined;
  try {
    await migration.message('migration-entered');
    insertion = worker('insert', filePath, 'inserted');
    await new Promise((resolve) => setTimeout(resolve, 100));
    expect(insertion.messages).not.toContain('done');
    await writeFile(release, 'go');
    expect((await migration.exited).code).toBe(0);
    expect((await insertion.exited).code).toBe(0);
    expect((await openRecords(filePath).read()).records).toEqual(['legacy', 'inserted']);
    expect(JSON.parse(await readFile(`${filePath}.bak`, 'utf8')).records).toEqual(['legacy']);
  } finally {
    await migration.stop();
    await insertion?.stop();
    await rm(directory, { recursive: true, force: true });
  }
}

export async function store_publication_crash_boundaries_reopen_with_verified_generations() {
  for (const phase of [
    'temporary-synced',
    'backup-published',
    'primary-published',
  ] satisfies StorePublicationPhase[]) {
    const directory = await scratchDirectory();
    const filePath = path.join(directory, 'records.json');
    await seed(filePath);
    const publisher = worker('publish', filePath, phase);
    try {
      await publisher.message(phase);
      const expectedPrimary = phase === 'primary-published' ? ['A', 'B', 'C'] : ['A', 'B'];
      const expectedBackup = phase === 'temporary-synced' ? ['A'] : ['A', 'B'];
      expect(JSON.parse(await readFile(filePath, 'utf8')).records).toEqual(expectedPrimary);
      expect(JSON.parse(await readFile(`${filePath}.bak`, 'utf8')).records).toEqual(expectedBackup);
      await publisher.stop();
      expect((await openRecords(filePath).read()).records).toEqual(expectedPrimary);
      await writeFile(filePath, '{corrupt-after-restart');
      await expect(openRecords(filePath).read()).rejects.toBeInstanceOf(DurableStoreCorruptError);
      expect((await openRecords(filePath).recoverFromBackup()).records).toEqual(expectedBackup);
      expect((await openRecords(filePath).read()).records).toEqual(expectedBackup);
    } finally {
      await publisher.stop();
      await rm(directory, { recursive: true, force: true });
    }
  }
}

export async function store_live_holder_dead_takeover_and_late_release_keep_ownership() {
  const directory = await scratchDirectory();
  const filePath = path.join(directory, 'records.json');
  const children: ReturnType<typeof worker>[] = [];
  try {
    const holder = worker('hold', filePath, 'slow');
    children.push(holder);
    await holder.message('held');
    const old = new Date(Date.now() - 60_000);
    await utimes(`${filePath}.lock`, old, old);
    const contender = worker('hold', filePath, 'successor');
    children.push(contender);
    await new Promise((resolve) => setTimeout(resolve, 150));
    expect(contender.messages).not.toContain('held');
    holder.child.send('release');
    expect((await holder.exited).code).toBe(0);
    await contender.message('held');
    await contender.stop();
    const recovery = worker('hold', filePath, 'recovered');
    children.push(recovery);
    await recovery.message('held');
    // Simulate a lock-path replacement while a formerly owning process is
    // suspended. Its delayed publication and release must not affect the new owner.
    await rename(`${filePath}.lock`, `${filePath}.retired-lock`);
    const successor = worker('hold', filePath, 'final');
    children.push(successor);
    await successor.message('held');
    const successorOwners = await readdir(`${filePath}.lock`);
    recovery.child.send('release');
    expect((await recovery.exited).code).toBe(1);
    expect(recovery.messages).toContainEqual({ error: 'Durable store lock lost' });
    expect(await readdir(`${filePath}.lock`)).toEqual(successorOwners);
    const third = worker('insert', filePath, 'third');
    children.push(third);
    await new Promise((resolve) => setTimeout(resolve, 100));
    expect(third.messages).not.toContain('done');
    successor.child.send('release');
    expect((await successor.exited).code).toBe(0);
    expect((await third.exited).code).toBe(0);
    expect((await openRecords(filePath).read()).records).toEqual(['slow', 'final', 'third']);
    await expect(stat(`${filePath}.lock`)).rejects.toMatchObject({ code: 'ENOENT' });
  } finally {
    await Promise.all(children.map((child) => child.stop()));
    await rm(directory, { recursive: true, force: true });
  }
}
