import { mkdtemp, readFile, readdir, rm, stat, unlink, writeFile } from 'node:fs/promises';
import os from 'node:os';
import path from 'node:path';
import { afterEach, expect, test, vi } from 'vitest';
import { AtomicJsonStore, DurableStoreCorruptError } from './atomic-json-store';

const directories: string[] = [];
const originalDataDirectory = process.env.PUSH_DATA_DIR;

afterEach(async () => {
  if (originalDataDirectory === undefined) delete process.env.PUSH_DATA_DIR;
  else process.env.PUSH_DATA_DIR = originalDataDirectory;
  vi.resetModules();
  await Promise.all(directories.splice(0).map((directory) => rm(directory, { recursive: true })));
});

async function temporaryDirectory(): Promise<string> {
  const directory = await mkdtemp(path.join(os.tmpdir(), 'repair-22-31-246-'));
  directories.push(directory);
  return directory;
}

interface State {
  version: 1;
  records: string[];
}

function parseState(input: unknown): State {
  if (
    typeof input !== 'object' ||
    input === null ||
    !('version' in input) ||
    input.version !== 1 ||
    !('records' in input) ||
    !Array.isArray(input.records) ||
    !input.records.every((record: unknown) => typeof record === 'string')
  ) {
    throw new TypeError('Invalid state');
  }
  return { version: 1, records: input.records };
}

function openStore(filePath: string): AtomicJsonStore<State> {
  return new AtomicJsonStore({
    filePath,
    empty: () => ({ version: 1, records: [] }),
    parse: parseState,
    maxBytes: 256,
  });
}

async function seedStore(filePath: string): Promise<State> {
  const store = openStore(filePath);
  const backedUp = await store.update((current) => ({
    ...current,
    records: [...current.records, 'durable-record'],
  }));
  await store.update((current) => ({ ...current, records: [...current.records, 'latest-record'] }));
  return backedUp;
}

async function push_corruption_reset_remains_unavailable_after_push_store_reopen() {
  const directory = await temporaryDirectory();
  process.env.PUSH_DATA_DIR = directory;
  const producer = await import('./push-store');
  const subscription = {
    endpoint: 'https://push.example.test/subscription',
    keys: { p256dh: 'a'.repeat(32), auth: 'b'.repeat(16) },
    walletAddress: `0x${'1'.repeat(40)}`,
    createdAt: 1,
    lastActiveAt: 1,
  };
  await producer.upsertSubscription(subscription);
  await producer.upsertSubscription({ ...subscription, tags: ['latest'] });
  await producer.addCampaign({ id: 'durable-campaign', title: 'Title', body: 'Body' });
  await producer.addCampaign({ id: 'latest-campaign', title: 'Latest', body: 'Body' });

  const cases = [
    {
      file: 'subscriptions.json',
      read: () => producer.getAllSubscriptions(),
      update: () => producer.upsertSubscription(subscription),
    },
    {
      file: 'campaigns.json',
      read: () => producer.getAllCampaigns(),
      update: () => producer.addCampaign({ id: 'retry', title: 'Retry', body: 'Body' }),
    },
  ];
  const backups = new Map<string, string>();
  for (const scenario of cases) {
    const filePath = path.join(directory, scenario.file);
    backups.set(scenario.file, await readFile(`${filePath}.bak`, 'utf8'));
    await writeFile(filePath, '{corrupted-durable-store');
    await expect(scenario.read()).rejects.toMatchObject({ name: 'DurableStoreCorruptError' });
    await expect(scenario.read()).rejects.toMatchObject({ name: 'DurableStoreCorruptError' });
    await expect(scenario.update()).rejects.toMatchObject({ name: 'DurableStoreCorruptError' });
    await expect(stat(filePath)).rejects.toMatchObject({ code: 'ENOENT' });
    expect((await stat(`${filePath}.corrupt`)).mode & 0o777).toBe(0o600);
    const quarantines = (await readdir(directory)).filter((name) =>
      name.startsWith(`${scenario.file}.corrupt-`)
    );
    expect(quarantines).toHaveLength(1);
    expect(await readFile(path.join(directory, quarantines[0]), 'utf8')).toBe(
      '{corrupted-durable-store'
    );
  }

  // Unload both consumers and the store module so no in-memory corruption flag can survive.
  vi.resetModules();
  const reopened = await import('./push-store');
  await expect(reopened.getAllSubscriptions()).rejects.toMatchObject({
    name: 'DurableStoreCorruptError',
  });
  await expect(reopened.upsertSubscription(subscription)).rejects.toMatchObject({
    name: 'DurableStoreCorruptError',
  });
  await expect(reopened.getAllCampaigns()).rejects.toMatchObject({
    name: 'DurableStoreCorruptError',
  });
  await expect(
    reopened.addCampaign({ id: 'after-restart', title: 'Restart', body: 'Body' })
  ).rejects.toMatchObject({ name: 'DurableStoreCorruptError' });
  for (const scenario of cases) {
    const filePath = path.join(directory, scenario.file);
    await expect(stat(filePath)).rejects.toMatchObject({ code: 'ENOENT' });
    expect(await readFile(`${filePath}.bak`, 'utf8')).toBe(backups.get(scenario.file));
  }
}

async function push_corruption_reset_recovers_only_an_explicitly_verified_backup() {
  const filePath = path.join(await temporaryDirectory(), 'records.json');
  const backedUp = await seedStore(filePath);
  const backup = await readFile(`${filePath}.bak`, 'utf8');
  await writeFile(filePath, '{broken');
  await expect(openStore(filePath).read()).rejects.toBeInstanceOf(DurableStoreCorruptError);

  const reopened = openStore(filePath);
  await expect(reopened.read()).rejects.toBeInstanceOf(DurableStoreCorruptError);
  let mutated = false;
  await expect(
    reopened.update((current) => {
      mutated = true;
      return { ...current, records: ['replacement'] };
    })
  ).rejects.toBeInstanceOf(DurableStoreCorruptError);
  expect(mutated).toBe(false);
  expect(await reopened.recoverFromBackup()).toEqual(backedUp);
  expect(await readFile(`${filePath}.bak`, 'utf8')).toBe(backup);
  await expect(stat(`${filePath}.corrupt`)).rejects.toMatchObject({ code: 'ENOENT' });
  expect(await openStore(filePath).read()).toEqual(backedUp);
  await openStore(filePath).update((current) => ({
    ...current,
    records: [...current.records, 'after-recovery'],
  }));
  expect(await openStore(filePath).read()).toEqual({
    version: 1,
    records: ['durable-record', 'after-recovery'],
  });
}

async function push_corruption_reset_rejects_invalid_or_missing_backups_across_reopen() {
  const filePath = path.join(await temporaryDirectory(), 'records.json');
  const backedUp = await seedStore(filePath);
  const verifiedBackup = await readFile(`${filePath}.bak`, 'utf8');
  await writeFile(filePath, '{broken');
  await expect(openStore(filePath).read()).rejects.toBeInstanceOf(DurableStoreCorruptError);

  for (const invalid of [
    undefined,
    '{broken-backup',
    '{"version":1,"records":[42]}',
    ' '.repeat(257),
  ]) {
    if (invalid === undefined) await unlink(`${filePath}.bak`);
    else await writeFile(`${filePath}.bak`, invalid);
    await expect(openStore(filePath).recoverFromBackup()).rejects.toBeInstanceOf(
      DurableStoreCorruptError
    );
    await expect(openStore(filePath).read()).rejects.toBeInstanceOf(DurableStoreCorruptError);
    await expect(openStore(filePath).update((current) => current)).rejects.toBeInstanceOf(
      DurableStoreCorruptError
    );
    await expect(stat(filePath)).rejects.toMatchObject({ code: 'ENOENT' });
    expect((await stat(`${filePath}.corrupt`)).isFile()).toBe(true);
    if (invalid !== undefined) expect(await readFile(`${filePath}.bak`, 'utf8')).toBe(invalid);
  }

  await writeFile(`${filePath}.bak`, verifiedBackup);
  expect(await openStore(filePath).recoverFromBackup()).toEqual(backedUp);
  expect(await openStore(filePath).read()).toEqual(backedUp);
}

async function push_corruption_reset_keeps_interrupted_recovery_unavailable() {
  const filePath = path.join(await temporaryDirectory(), 'records.json');
  const backedUp = await seedStore(filePath);
  const backup = await readFile(`${filePath}.bak`, 'utf8');
  await writeFile(filePath, '{broken');
  await expect(openStore(filePath).read()).rejects.toBeInstanceOf(DurableStoreCorruptError);

  // Emulate a stop after primary publication but before the recovery marker is removed.
  await writeFile(filePath, backup);
  await expect(openStore(filePath).read()).rejects.toBeInstanceOf(DurableStoreCorruptError);
  await expect(openStore(filePath).update((current) => current)).rejects.toBeInstanceOf(
    DurableStoreCorruptError
  );
  expect(await openStore(filePath).recoverFromBackup()).toEqual(backedUp);
  expect(await readFile(`${filePath}.bak`, 'utf8')).toBe(backup);
  expect(await openStore(filePath).read()).toEqual(backedUp);
}

async function push_corruption_reset_marks_schema_and_size_corruption() {
  const directory = await temporaryDirectory();
  for (const [index, invalid] of ['{"version":1,"records":[42]}', ' '.repeat(257)].entries()) {
    const filePath = path.join(directory, `records-${index}.json`);
    const backedUp = await seedStore(filePath);
    await writeFile(filePath, invalid);
    const store = openStore(filePath);
    await expect(store.read()).rejects.toBeInstanceOf(DurableStoreCorruptError);
    await expect(store.read()).rejects.toBeInstanceOf(DurableStoreCorruptError);
    await expect(openStore(filePath).read()).rejects.toBeInstanceOf(DurableStoreCorruptError);
    await expect(openStore(filePath).update((current) => current)).rejects.toBeInstanceOf(
      DurableStoreCorruptError
    );
    expect(await openStore(filePath).recoverFromBackup()).toEqual(backedUp);
  }
}

async function push_corruption_reset_preserves_fresh_store_initialization() {
  const filePath = path.join(await temporaryDirectory(), 'new', 'records.json');
  expect(await openStore(filePath).read()).toEqual({ version: 1, records: [] });
  await openStore(filePath).update((current) => ({
    ...current,
    records: [...current.records, 'first-record'],
  }));
  expect(await openStore(filePath).read()).toEqual({ version: 1, records: ['first-record'] });
}

test(
  push_corruption_reset_remains_unavailable_after_push_store_reopen,
  push_corruption_reset_remains_unavailable_after_push_store_reopen
);
test(
  push_corruption_reset_recovers_only_an_explicitly_verified_backup,
  push_corruption_reset_recovers_only_an_explicitly_verified_backup
);
test(
  push_corruption_reset_rejects_invalid_or_missing_backups_across_reopen,
  push_corruption_reset_rejects_invalid_or_missing_backups_across_reopen
);
test(
  push_corruption_reset_keeps_interrupted_recovery_unavailable,
  push_corruption_reset_keeps_interrupted_recovery_unavailable
);
test(
  push_corruption_reset_marks_schema_and_size_corruption,
  push_corruption_reset_marks_schema_and_size_corruption
);
test(
  push_corruption_reset_preserves_fresh_store_initialization,
  push_corruption_reset_preserves_fresh_store_initialization
);
