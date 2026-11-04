import assert from 'node:assert/strict';
import {
  mkdtemp,
  open,
  readFile,
  readdir,
  rm,
  writeFile,
  type FileHandle,
} from 'node:fs/promises';
import { createServer } from 'node:http';
import { tmpdir } from 'node:os';
import { join } from 'node:path';
import { fileURLToPath } from 'node:url';
import {
  SdkClient,
  decodeV2Contract,
} from '../../../../../sdk/launchpad/packages/sdk/src/transport.ts';
import {
  SnapshotCache,
  validatePublicSnapshot,
  type PublicSnapshotStorage,
} from '../../src/runtime/offline-cache.ts';
import { activityRoutes } from '../../src/runtime/routes/activity.ts';

class PublicSnapshotDisk implements PublicSnapshotStorage {
  private constructor(
    readonly directory: string,
    private readonly handle: FileHandle,
  ) {}

  static async open(directory: string) {
    return new PublicSnapshotDisk(directory, await open(directory, 'r'));
  }

  private async path(key: string) {
    await this.handle.stat();
    return join(this.directory, `${encodeURIComponent(key)}.json`);
  }

  async read(key: string) {
    const path = await this.path(key);
    try {
      return await readFile(path, 'utf8');
    } catch (error) {
      if (error instanceof Error && 'code' in error && error.code === 'ENOENT')
        return null;
      throw error;
    }
  }

  async write(key: string, value: string) {
    await writeFile(await this.path(key), value, { mode: 0o600 });
  }

  async clear() {
    await this.handle.stat();
    for (const file of await readdir(this.directory))
      await rm(join(this.directory, file));
  }

  close() {
    return this.handle.close();
  }
}

async function withProducerResponse(
  run: (context: {
    sdk: SdkClient;
    value: ReturnType<typeof decodeV2Contract<'StreamListResponse'>>;
    disk: PublicSnapshotDisk;
    directory: string;
    send(value: unknown): void;
    requests: string[];
  }) => Promise<void>,
) {
  const transcript = await readFile(
    fileURLToPath(
      new URL('../fixtures/live-v2-response.json', import.meta.url),
    ),
    'utf8',
  );
  const value = decodeV2Contract('StreamListResponse', JSON.parse(transcript));
  let wire = transcript;
  const requests: string[] = [];
  const server = createServer((request, response) => {
    requests.push(`${request.method} ${request.url}`);
    response.writeHead(200, { 'Content-Type': 'application/json' });
    response.end(wire);
  });
  const directory = await mkdtemp(join(tmpdir(), 'mobile-cache-proof-'));
  const disk = await PublicSnapshotDisk.open(directory);
  try {
    await new Promise<void>((resolve, reject) => {
      server.once('error', reject);
      server.listen(0, '127.0.0.1', resolve);
    });
    const bound = server.address();
    assert.ok(bound && typeof bound !== 'string');
    const origin = `http://127.0.0.1:${bound.port}`;
    const streamOrigin = origin.replace('http:', 'ws:');
    const sdk = new SdkClient({
      apiOrigin: origin,
      identityOrigin: origin,
      mediaOrigin: origin,
      streamOrigin,
      approvedStreamOrigins: [streamOrigin],
      production: false,
    });
    await run({
      sdk,
      value,
      disk,
      directory,
      requests,
      send: (body) => {
        wire = JSON.stringify(body);
      },
    });
  } finally {
    await new Promise<void>((resolve, reject) => {
      server.close((error) => (error ? reject(error) : resolve()));
      server.closeAllConnections();
    });
    await disk.close();
    await rm(directory, { recursive: true, force: true });
  }
}

export async function mobileCachePositiveProducerRestore() {
  await withProducerResponse(
    async ({ sdk, value, disk, directory, requests }) => {
      const produced = await activityRoutes.streams(
        sdk,
        new AbortController().signal,
      );
      assert.deepEqual(produced, value);
      assert.deepEqual(requests, ['GET /v2/streams?status=live&limit=50']);
      assert.equal(produced.data.items[0].viewerCount, null);
      assert.equal(produced.data.items[0].viewerQuality, 'unavailable');
      assert.equal(produced.data.items[1].viewerCount, 0);
      assert.equal(produced.data.items[1].viewerQuality, 'verified');
      const storedAt = Date.parse(value.generatedAt);
      const expiresAt = storedAt + 300_000;
      let cache: SnapshotCache | undefined = new SnapshotCache(
        undefined,
        undefined,
        disk,
      );
      cache.put('live', produced, null, expiresAt, storedAt);
      cache.put(
        'portfolio',
        { quantity: '13' },
        'account-a',
        expiresAt,
        storedAt,
      );
      await cache.flush();
      const saved = JSON.parse((await disk.read('live'))!);
      assert.deepEqual(saved, {
        formatVersion: 2,
        contractVersion: 2,
        value: produced,
        storedAt,
        expiresAt,
        account: null,
        bytes: Buffer.byteLength(JSON.stringify(produced)),
      });
      assert.deepEqual(await readdir(directory), ['live.json']);
      await disk.close();
      cache = undefined;
      await assert.rejects(disk.read('live'));
      const reopened = await PublicSnapshotDisk.open(directory);
      try {
        const consumer = new SnapshotCache(undefined, undefined, reopened);
        assert.equal(consumer.get('live', null), undefined);
        const restored = await consumer.read<typeof produced>(
          'live',
          null,
          expiresAt + 1,
        );
        assert.ok(restored);
        assert.deepEqual(restored.value, produced);
        assert.equal(restored.storedAt, storedAt);
        assert.equal(restored.expiresAt, expiresAt);
        assert.equal(restored.account, null);
        assert.equal(restored.bytes, saved.bytes);
        assert.equal(
          await consumer.read('portfolio', 'account-a', storedAt),
          undefined,
        );
        restored.value.data.items[0].title = 'Consumer mutation';
        assert.deepEqual(
          consumer.get<typeof produced>('live', null)?.value,
          produced,
        );
        assert.deepEqual(
          JSON.parse((await reopened.read('live'))!).value,
          produced,
        );
        assert.equal(requests.length, 1);
      } finally {
        await reopened.close();
      }
    },
  );
}

export async function mobileCacheMalformedPrivateRefusal() {
  await withProducerResponse(
    async ({ sdk, value, disk, directory, requests, send }) => {
      const produced = await activityRoutes.streams(
        sdk,
        new AbortController().signal,
      );
      const storedAt = Date.parse(value.generatedAt);
      const expiresAt = storedAt + 300_000;
      const cache = new SnapshotCache(undefined, undefined, disk);
      cache.put('live', produced, null, expiresAt, storedAt);
      await cache.flush();
      const original = (await disk.read('live'))!;
      const missingQuality = structuredClone(produced);
      Reflect.deleteProperty(missingQuality.data.items[0], 'viewerQuality');
      const malformed = [
        {
          ...produced,
          data: { items: [{ ...produced.data.items[0], viewerCount: '0' }] },
        },
        {
          ...produced,
          data: {
            items: [{ ...produced.data.items[0], viewerQuality: 'invented' }],
          },
        },
        missingQuality,
        { ...produced, revision: '0' },
        { ...produced, session: 'private-session' },
        { session: 'private-session' },
        { body: 'private message' },
        { transaction: { data: 'signed payload' } },
        { account: null, value: { pushToken: 'private-token' } },
      ];
      for (const payload of malformed) {
        send(payload);
        await assert.rejects(
          activityRoutes.streams(sdk, new AbortController().signal),
        );
        assert.throws(() => validatePublicSnapshot('live', payload));
        assert.throws(() =>
          cache.put('live', payload, null, expiresAt, storedAt),
        );
        await cache.flush();
        assert.equal(await disk.read('live'), original);
        assert.deepEqual(cache.get('live', null)?.value, produced);
        const stored = {
          ...JSON.parse(original),
          value: payload,
          bytes: Buffer.byteLength(JSON.stringify(payload)),
        };
        await disk.write('live', JSON.stringify(stored));
        const reopened = await PublicSnapshotDisk.open(directory);
        try {
          const consumer = new SnapshotCache(undefined, undefined, reopened);
          await assert.rejects(consumer.read('live', null, storedAt));
          assert.equal(consumer.get('live', null), undefined);
        } finally {
          await reopened.close();
        }
        await disk.write('live', original);
      }
      assert.equal(requests.length, malformed.length + 1);
      for (const altered of [
        { ...JSON.parse(original), account: 'account-a' },
        { ...JSON.parse(original), bytes: 1 },
        { ...JSON.parse(original), expiresAt: storedAt },
        { ...JSON.parse(original), storedAt: -1 },
        { ...JSON.parse(original), session: 'private-session' },
        { ...JSON.parse(original), formatVersion: 3 },
        { ...JSON.parse(original), contractVersion: 1 },
        { ...JSON.parse(original), contractVersion: 3 },
      ]) {
        await disk.write('live', JSON.stringify(altered));
        const consumer = new SnapshotCache(undefined, undefined, disk);
        await assert.rejects(consumer.read('live', null, storedAt));
        assert.equal(consumer.get('live', null), undefined);
      }
      const legacy = JSON.parse(original);
      delete legacy.formatVersion;
      delete legacy.contractVersion;
      await disk.write('live', JSON.stringify(legacy));
      await assert.rejects(
        new SnapshotCache(undefined, undefined, disk).read('live', null),
      );
      await disk.write('live', original);
      for (const key of [
        'session',
        'pushToken',
        'portfolio',
        'messages:private',
        '../session',
      ]) {
        assert.throws(() => validatePublicSnapshot(key, produced));
        cache.put(key, { secret: 'private' }, null, expiresAt, storedAt);
        assert.equal(
          await new SnapshotCache(undefined, undefined, disk).read(key, null),
          undefined,
        );
      }
      await cache.flush();
      assert.deepEqual(await readdir(directory), ['live.json']);
      assert.equal(await disk.read('live'), original);
      await disk.write('live', '{');
      const corrupt = new SnapshotCache(undefined, undefined, disk);
      await assert.rejects(corrupt.read('live', null));
      assert.equal(corrupt.get('live', null), undefined);
    },
  );
}
