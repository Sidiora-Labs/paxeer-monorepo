/* mobile-live-restore-version -> mobileLiveRestoreVersion.
 * Baseline verification was not run: the bounded gate order requires complete implementation first.
 * No out-of-path wiring is required. Go overlays provide a private harness for the real producer codecs.
 */
import { execFile } from 'node:child_process';
import {
  mkdtemp,
  open,
  readFile,
  writeFile,
  rm,
  type FileHandle,
} from 'node:fs/promises';
import { createServer } from 'node:http';
import { createRequire } from 'node:module';
import { tmpdir } from 'node:os';
import { dirname, join, resolve } from 'node:path';
import { promisify } from 'node:util';
import { expect, test } from 'vitest';
import {
  SdkClient,
  type V2ContractTypes,
} from '../../../../../sdk/launchpad/packages/sdk/src/index.ts';
import { activityRoutes } from '../../src/runtime/routes/activity.ts';
import {
  SnapshotCache,
  validatePublicSnapshot,
  type PublicSnapshotStorage,
} from '../../src/runtime/offline-cache.ts';
import {
  SessionController,
  type CredentialStore,
} from '../../src/runtime/session.ts';
import type * as ResourceModule from '../../src/runtime/resource.ts';
import type * as React from 'react';
import type { Root } from '../../../web/node_modules/@types/react-dom/client.d.ts';

class DiskStorage implements PublicSnapshotStorage, CredentialStore {
  private constructor(
    readonly directory: string,
    private readonly handle: FileHandle,
  ) {}
  static async open(directory: string) {
    return new DiskStorage(directory, await open(directory, 'r'));
  }
  private async path(key: string) {
    await this.handle.stat();
    if (!/^[a-zA-Z0-9_-]+$/.test(key)) throw new Error('Invalid disk key');
    return join(this.directory, key);
  }
  async read(key: string) {
    try {
      return await readFile(await this.path(key), 'utf8');
    } catch (error) {
      if (
        error &&
        typeof error === 'object' &&
        'code' in error &&
        error.code === 'ENOENT'
      )
        return null;
      throw error;
    }
  }
  async write(key: string, value: string) {
    await writeFile(await this.path(key), value, { mode: 0o600 });
  }
  get(key: Parameters<CredentialStore['get']>[0]) {
    return this.read(key);
  }
  set(key: Parameters<CredentialStore['set']>[0], value: string) {
    return this.write(key, value);
  }
  async remove(key: Parameters<CredentialStore['remove']>[0]) {
    await rm(await this.path(key), { force: true });
  }
  async clear() {
    for (const key of [
      'live',
      'session',
      'pushToken',
      'pending',
      'installation',
      'privacy',
      'approval',
    ])
      await rm(await this.path(key), { force: true });
  }
  close() {
    return this.handle.close();
  }
}

async function producerResponse(directory: string) {
  const repository = resolve(process.cwd(), '../../..');
  const source = join(directory, 'producer.go');
  await writeFile(
    source,
    `package main
import (
  "fmt"
  "time"
  streams "github.com/Sidiora-Technologies/KindleLaunch/media/livestream/internal/streams"
)
func main() {
  at := time.Now().UTC()
  operation := streams.Operation{
    ID: "11111111-1111-4111-8111-111111111111",
    Pool: "0x1111111111111111111111111111111111111111",
    Wallet: "0x2222222222222222222222222222222222222222",
    Title: "Unavailable viewers survive restart", State: "LIVE",
    ProviderID: "provider_1", ProviderTime: &at, Revision: 9,
  }
  item, err := streams.AdaptV2Stream(operation, "https://livepeercdn.studio/hls/one/index.m3u8", nil, "unavailable")
  if err != nil { panic(err) }
  raw, err := streams.AdaptV2StreamList("33333333-3333-4333-8333-333333333333", []streams.V2Stream{item}, nil, at)
  if err != nil { panic(err) }
  fmt.Print(string(raw))
}
`,
  );
  const overlay = join(directory, 'overlay.json');
  await writeFile(
    overlay,
    JSON.stringify({
      Replace: {
        [join(
          repository,
          'services/launchpad/livestream/cmd/livestreamd/main.go',
        )]: source,
      },
    }),
  );
  const result = await promisify(execFile)(
    'go',
    ['run', '-overlay', overlay, './cmd/livestreamd'],
    {
      cwd: join(repository, 'services/launchpad/livestream'),
      timeout: 120000,
      maxBuffer: 4 * 1024 * 1024,
    },
  );
  return result.stdout;
}

type Harness = typeof ResourceModule & {
  createElement: typeof React.createElement;
  createRoot(container: Element): Root;
};
async function resourceHarness(directory: string): Promise<Harness> {
  const require = createRequire(import.meta.url);
  const web = createRequire(resolve(process.cwd(), '../web/package.json'));
  const fromVite = createRequire(require.resolve('vite'));
  const esbuild = fromVite('esbuild') as {
    build(
      options: Record<string, unknown>,
    ): Promise<{ outputFiles: { text: string }[] }>;
  };
  const validators = resolve(
    process.cwd(),
    '../../../sdk/launchpad/packages/sdk/dist',
  );
  const bundle = await esbuild.build({
    stdin: {
      contents: `export * from ${JSON.stringify(resolve(process.cwd(), 'src/runtime/resource.ts'))}; export {createElement} from 'react'; export {createRoot} from 'react-dom/client';`,
      resolveDir: process.cwd(),
      loader: 'ts',
    },
    bundle: true,
    write: false,
    platform: 'node',
    format: 'cjs',
    alias: {
      react: dirname(web.resolve('react/package.json')),
      'react-dom': dirname(web.resolve('react-dom/package.json')),
    },
    plugins: [
      {
        name: 'generated-validators',
        setup(build: {
          onResolve(
            options: { filter: RegExp },
            callback: (args: { path: string }) => { path: string },
          ): void;
        }) {
          build.onResolve(
            { filter: /^\.\/validators-(?:v1|v2|operations)\.gen\.js$/ },
            (args) => ({ path: join(validators, args.path) }),
          );
        },
      },
    ],
  });
  const output = join(directory, 'resource.cjs');
  // The test renderer uses the installed web React and DOM implementation, with no native-module substitute.
  await writeFile(output, bundle.outputFiles[0].text);
  return require(output) as Harness;
}

async function mobileLiveRestoreVersion() {
  const directory = await mkdtemp(join(tmpdir(), 'mobile-restore-'));
  let storage: DiskStorage | undefined;
  let root: Root | undefined;
  const server = createServer();
  let stopServer: (() => Promise<void>) | undefined;
  const web = createRequire(resolve(process.cwd(), '../web/package.json'));
  const { JSDOM } = web('jsdom') as {
    JSDOM: new (html: string) => {
      window: Window & typeof globalThis;
      windowClosed?: boolean;
    };
  };
  const dom = new JSDOM(
    '<!doctype html><html><body><div id="root"></div></body></html>',
  );
  const globals = new Map<string, PropertyDescriptor | undefined>();
  for (const key of ['window', 'document', 'navigator']) {
    globals.set(key, Object.getOwnPropertyDescriptor(globalThis, key));
    Object.defineProperty(globalThis, key, {
      configurable: true,
      value: dom.window[key as 'window' | 'document' | 'navigator'],
    });
  }
  try {
    const produced = await producerResponse(directory);
    let wire = produced;
    const observed: string[] = [];
    server.on('request', (request, response) => {
      observed.push(`${request.method} ${request.url}`);
      response.writeHead(200, { 'Content-Type': 'application/json' });
      response.end(wire);
    });
    await new Promise<void>((resolve) =>
      server.listen(0, '127.0.0.1', resolve),
    );
    stopServer = () =>
      new Promise<void>((resolve, reject) => {
        server.close((error) => (error ? reject(error) : resolve()));
        server.closeAllConnections();
      });
    const bound = server.address();
    if (!bound || typeof bound === 'string')
      throw new Error('HTTP listener unavailable');
    const origin = `http://127.0.0.1:${bound.port}`;
    const sdk = new SdkClient({
      apiOrigin: origin,
      identityOrigin: origin,
      mediaOrigin: origin,
      streamOrigin: origin.replace('http:', 'ws:'),
      approvedStreamOrigins: [origin.replace('http:', 'ws:')],
      production: false,
    });
    storage = await DiskStorage.open(directory);
    let cache = new SnapshotCache(undefined, undefined, storage);
    let runtime = { cache, session: new SessionController(storage, cache) };
    let networkLosses = 0;
    let disconnected = Promise.withResolvers<void>();
    const { useResource, offlineSnapshotLabel, createRoot, createElement } =
      await resourceHarness(directory);
    const container = dom.window.document.getElementById('root')!;
    let latest:
      | ReturnType<typeof useResource<V2ContractTypes['StreamListResponse']>>
      | undefined;
    function LiveProbe() {
      const state = useResource(
        runtime,
        'live',
        (signal) => activityRoutes.streams(sdk, signal),
        false,
        () => {
          networkLosses++;
          disconnected.resolve();
        },
      );
      latest = state;
      return createElement(
        'div',
        {},
        state.loading
          ? 'Loading'
          : state.data?.data.items.map((item) => item.title).join(', '),
        state.offline ? offlineSnapshotLabel(state.storedAt) : '',
        state.error,
      );
    }
    root = createRoot(container);
    root.render(createElement(LiveProbe));
    await expect
      .poll(
        () => latest?.loading === false && latest.data?.data.items[0]?.title,
      )
      .toBe('Unavailable viewers survive restart');
    expect(observed).toEqual(['GET /v2/streams?status=live&limit=50']);
    const value = latest!.data!;
    expect(value.data.items[0].viewerCount).toBeNull();
    expect(value.data.items[0].viewerQuality).toBe('unavailable');
    expect(latest!.offline).toBe(false);
    const storedAt = Date.now() - 10 * 60 * 1000;
    cache.put('live', value, null, storedAt + 5 * 60 * 1000, storedAt);
    await expect
      .poll(
        async () => JSON.parse((await storage!.read('live')) ?? '{}').storedAt,
      )
      .toBe(storedAt);

    // Both current transport and the shared recovery codec reject malformed versions of producer output.
    for (const malformed of [
      { ...value, unexpected: true },
      {
        ...value,
        data: { items: [{ ...value.data.items[0], viewerCount: '0' }] },
      },
      {
        ...value,
        data: {
          items: [{ ...value.data.items[0], viewerQuality: 'invented' }],
        },
      },
    ]) {
      wire = JSON.stringify(malformed);
      expect(() => validatePublicSnapshot('live', malformed)).toThrow();
      await expect(
        activityRoutes.streams(sdk, new AbortController().signal),
      ).rejects.toThrow();
    }
    expect(() => validatePublicSnapshot('unapproved', value)).toThrow();
    expect(() => validatePublicSnapshot('pools', value)).toThrow();

    root.unmount();
    root = undefined;
    await storage.close();
    await stopServer();
    stopServer = undefined;
    await expect(storage.read('live')).rejects.toThrow();
    storage = await DiskStorage.open(directory);
    cache = new SnapshotCache(undefined, undefined, storage);
    expect(cache.get('live', null)).toBeUndefined();
    runtime = { cache, session: new SessionController(storage, cache) };
    root = createRoot(container);
    root.render(createElement(LiveProbe));
    await disconnected.promise;
    await expect
      .poll(() => latest?.loading === false && latest.offline)
      .toBe(true);
    expect(networkLosses).toBe(1);
    expect(latest!.data).toEqual(value);
    expect(latest!.storedAt).toBe(storedAt);
    expect(container.textContent).toContain(value.data.items[0].title);
    expect(container.textContent).toContain(
      `Offline snapshot · ${new Date(storedAt).toLocaleString()}`,
    );
    expect(container.textContent).toContain(
      'Service unavailable. Retry when connected.',
    );
    expect(latest!.data!.data.items[0].viewerCount).toBeNull();
    expect(latest!.data!.data.items[0].viewerQuality).toBe('unavailable');

    // A corrupted persisted envelope is rejected on a second cold restart.
    root.unmount();
    root = undefined;
    const saved = JSON.parse((await storage.read('live'))!);
    saved.value.data.items[0].viewerCount = 'invalid';
    saved.bytes = Buffer.byteLength(JSON.stringify(saved.value));
    await storage.write('live', JSON.stringify(saved));
    await storage.close();
    storage = await DiskStorage.open(directory);
    cache = new SnapshotCache(undefined, undefined, storage);
    await expect(cache.read('live', null)).rejects.toThrow();
    runtime = { cache, session: new SessionController(storage, cache) };
    disconnected = Promise.withResolvers<void>();
    root = createRoot(container);
    root.render(createElement(LiveProbe));
    await disconnected.promise;
    await expect
      .poll(() => latest?.loading === false && latest.error)
      .toBe('Service unavailable. Retry when connected.');
    expect(latest!.data).toBeUndefined();
    expect(latest!.offline).toBe(false);
    expect(container.textContent).not.toContain(value.data.items[0].title);
    expect(container.textContent).not.toContain('Offline snapshot');

    // Memory recovery uses the same strict decoder even when disk admission was bypassed.
    root.unmount();
    root = undefined;
    cache = new SnapshotCache();
    cache.put(
      'live',
      saved.value,
      null,
      Date.now() + 300000,
      Date.now(),
      false,
    );
    runtime = { cache, session: new SessionController(storage, cache) };
    disconnected = Promise.withResolvers<void>();
    root = createRoot(container);
    root.render(createElement(LiveProbe));
    await disconnected.promise;
    await expect
      .poll(() => latest?.loading === false && latest.error)
      .toBe('Stored public snapshot is invalid. Reconnect to refresh it.');
    expect(latest!.data).toBeUndefined();
    expect(latest!.offline).toBe(false);
    expect(container.textContent).not.toContain(value.data.items[0].title);
  } finally {
    root?.unmount();
    await stopServer?.();
    await storage?.close();
    dom.window.close();
    for (const [key, descriptor] of globals) {
      if (descriptor) Object.defineProperty(globalThis, key, descriptor);
      else Reflect.deleteProperty(globalThis, key);
    }
    await rm(directory, { recursive: true, force: true });
  }
}

test(mobileLiveRestoreVersion, mobileLiveRestoreVersion, 180000);
