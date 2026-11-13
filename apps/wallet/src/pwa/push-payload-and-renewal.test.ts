import { randomUUID } from 'node:crypto';
import { readFile, mkdtemp, rm, writeFile } from 'node:fs/promises';
import { createServer, type Server } from 'node:http';
import { tmpdir } from 'node:os';
import path from 'node:path';
import vm from 'node:vm';
import { Wallet } from 'ethers';
import { NextRequest } from 'next/server';
import { transpileModule, ModuleKind, ScriptTarget } from 'typescript';
import { afterAll, beforeAll, expect, test } from 'vitest';
import * as caching from './caching';
import * as manifest from './manifest';
import * as renewal from './push-renewal';

// Browser platform fixtures retain response bytes across worker instances.
class BrowserCacheStorage {
  private readonly stores = new Map<string, Map<string, Response>>();
  async open(name: string) {
    let records = this.stores.get(name);
    if (!records) {
      records = new Map();
      this.stores.set(name, records);
    }
    const entries = records;
    return {
      async match(key: string) {
        return entries.get(key)?.clone();
      },
      async put(key: string, response: Response) {
        entries.set(key, response.clone());
      },
      async delete(key: string) {
        return entries.delete(key);
      },
    };
  }
  async keys() {
    return [...this.stores.keys()];
  }
  async delete(name: string) {
    return this.stores.delete(name);
  }
}

class BrowserSubscription {
  readonly options = { userVisibleOnly: true, applicationServerKey: new Uint8Array([1, 2, 3]) };
  constructor(readonly endpoint: string) {}
  toJSON() {
    return { endpoint: this.endpoint, keys: { p256dh: 'a'.repeat(87), auth: 'b'.repeat(22) } };
  }
}

type WorkerEvent = Record<string, unknown> & { waitUntil(promise: Promise<unknown>): void };
class BrowserWorker extends EventTarget implements renewal.RenewalWorker {
  private readonly listeners = new Map<string, (event: WorkerEvent) => void>();
  readonly notifications: { title: string; options: NotificationOptions }[] = [];
  readonly broadcasts: unknown[] = [];
  readonly subscriptions: unknown[] = [];
  constructor(
    readonly origin: string,
    storage: BrowserCacheStorage,
    source: string,
    readonly nextSubscription = new BrowserSubscription(
      'https://push.example/subscriptions/renewed'
    ),
    readonly windows = false
  ) {
    super();
    const scope = {
      location: new URL(`${origin}/wallet/sw.js`),
      addEventListener: (name: string, listener: (event: WorkerEvent) => void) => {
        this.listeners.set(name, listener);
      },
      registration: {
        showNotification: async (title: string, options: NotificationOptions) => {
          this.notifications.push({ title, options });
        },
        pushManager: {
          subscribe: async (options: unknown) => {
            this.subscriptions.push(options);
            return this.nextSubscription;
          },
        },
      },
      clients: {
        claim: async () => {},
        matchAll: async () =>
          windows
            ? [
                {
                  url: `${origin}/wallet/`,
                  postMessage: (message: unknown) => {
                    this.broadcasts.push(message);
                  },
                },
              ]
            : [],
      },
    };
    const dependencies: Record<string, unknown> = {
      './caching': caching,
      './manifest': manifest,
      './push-renewal': renewal,
    };
    vm.runInNewContext(source, {
      self: scope,
      caches: storage,
      URL,
      Request,
      Response,
      crypto: { randomUUID },
      exports: {},
      require: (name: string) => {
        if (!(name in dependencies)) throw new Error(`Unexpected worker import: ${name}`);
        return dependencies[name];
      },
      __PAXEER_SW__: { version: 'repair-22-33-153', networkOnly: [] },
    });
  }
  async dispatch(type: string, input: Record<string, unknown> = {}) {
    const waits: Promise<unknown>[] = [];
    const listener = this.listeners.get(type);
    if (!listener) throw new Error(`Worker is stopped or has no ${type} listener`);
    listener({
      ...input,
      waitUntil: (promise) => {
        waits.push(promise);
      },
    });
    await Promise.all(waits);
  }
  async push(text: string | null) {
    await this.dispatch('push', {
      data: text === null ? null : { json: () => JSON.parse(text), text: () => text },
    });
  }
  postMessage(message: unknown, transfer: Transferable[]) {
    void this.dispatch('message', {
      data: message,
      ports: transfer,
      source: { url: `${this.origin}/wallet/` },
    });
  }
  stop() {
    this.listeners.clear();
  }
}

class BrowserContainer extends EventTarget implements renewal.PushRenewalContainer {
  readonly ready: Promise<{ active: BrowserWorker }>;
  constructor(readonly controller: BrowserWorker) {
    super();
    this.ready = Promise.resolve({ active: controller });
  }
}

let source: string;
let directory: string;
let storeDirectory: string;
let origin: string;
let server: Server;
let post: typeof import('../app/api/push/subscribe/route').POST;
let getSubscription: typeof import('../server/push-store').getSubscription;
let registrations = 0;
const previousDataDirectory = process.env.PUSH_DATA_DIR;

beforeAll(async () => {
  source = transpileModule(await readFile(path.join(__dirname, 'sw.ts'), 'utf8'), {
    compilerOptions: { module: ModuleKind.CommonJS, target: ScriptTarget.ES2020 },
  }).outputText;
  directory = await mkdtemp(path.join(tmpdir(), 'wallet-push-repair-'));
  storeDirectory = path.join(directory, 'store');
  process.env.PUSH_DATA_DIR = storeDirectory;
  ({ POST: post } = await import('../app/api/push/subscribe/route'));
  ({ getSubscription } = await import('../server/push-store'));
  server = createServer((request, response) => {
    void (async () => {
      if (request.method !== 'POST' || request.url !== '/wallet/api/push/subscribe') {
        throw new Error('Unexpected registration request');
      }
      registrations++;
      const chunks: Buffer[] = [];
      for await (const chunk of request) chunks.push(Buffer.from(chunk));
      const result = await post(
        new NextRequest(new URL(request.url, origin), {
          method: 'POST',
          headers: { 'Content-Type': 'application/json' },
          body: Buffer.concat(chunks).toString('utf8'),
        })
      );
      response.writeHead(result.status, Object.fromEntries(result.headers));
      response.end(Buffer.from(await result.arrayBuffer()));
    })().catch((error: unknown) => {
      response.destroy(error instanceof Error ? error : undefined);
    });
  });
  await new Promise<void>((resolve) => server.listen(0, '127.0.0.1', resolve));
  const address = server.address();
  if (!address || typeof address === 'string') throw new Error('Missing loopback address');
  origin = `http://127.0.0.1:${address.port}`;
});

afterAll(async () => {
  if (server)
    await new Promise<void>((resolve, reject) =>
      server.close((error) => (error ? reject(error) : resolve()))
    );
  if (previousDataDirectory === undefined) delete process.env.PUSH_DATA_DIR;
  else process.env.PUSH_DATA_DIR = previousDataDirectory;
  if (directory) await rm(directory, { recursive: true, force: true });
});

async function pushPayloadShape(): Promise<void> {
  const worker = new BrowserWorker(origin, new BrowserCacheStorage(), source);
  const generic = {
    title: 'Paxeer Wallet',
    options: {
      body: 'You have a new notification.',
      icon: new URL(manifest.MANIFEST_ICONS[0].src, origin).href,
      badge: new URL(manifest.MANIFEST_ICONS[0].src, origin).href,
      data: { url: '/wallet/' },
      tag: 'paxeer-default',
      requireInteraction: false,
    },
  };
  for (const payload of [
    null,
    [],
    [1],
    false,
    true,
    7,
    'scalar',
    {
      title: {},
      body: 3,
      icon: [],
      badge: true,
      url: {},
      tag: 5,
      requireInteraction: 'true',
    },
    {
      title: 'a'.repeat(121),
      body: 'b'.repeat(501),
      icon: 'c'.repeat(2049),
      badge: 'd'.repeat(2049),
      url: 'e'.repeat(2049),
      tag: 'f'.repeat(129),
      requireInteraction: {},
    },
  ]) {
    await worker.push(JSON.stringify(payload));
    expect(worker.notifications.at(-1)).toEqual(generic);
  }
  await worker.push(null);
  expect(worker.notifications.at(-1)).toEqual(generic);
  await worker.push('x'.repeat(501));
  expect(worker.notifications.at(-1)).toEqual(generic);
  await worker.push(
    JSON.stringify({
      title: 'a'.repeat(120),
      body: 'b'.repeat(500),
      tag: 't'.repeat(128),
      icon: '/wallet/icons/app/icon-192.png',
      badge: '/wallet/badge.png',
      url: '/wallet/?transfer=received',
      requireInteraction: true,
    })
  );
  expect(worker.notifications.at(-1)).toEqual({
    title: 'a'.repeat(120),
    options: {
      body: 'b'.repeat(500),
      tag: 't'.repeat(128),
      icon: `${origin}/wallet/icons/app/icon-192.png`,
      badge: `${origin}/wallet/badge.png`,
      data: { url: '/wallet/?transfer=received' },
      requireInteraction: true,
    },
  });
  for (const url of ['javascript:alert(1)', 'https://foreign.example/wallet/', '/other/']) {
    await worker.push(JSON.stringify({ icon: url, badge: url, url }));
    const notification = worker.notifications.at(-1);
    expect(notification?.options.data).toEqual({ url: '/wallet/' });
    if (url !== '/other/') {
      expect(notification?.options.icon).toBe(manifest.MANIFEST_ICONS[0].src);
      expect(notification?.options.badge).toBe(manifest.MANIFEST_ICONS[0].src);
    }
  }
  await worker.push('plain text');
  expect(worker.notifications.at(-1)?.options.body).toBe('plain text');
}

function sessionWallet(wallet: ReturnType<typeof Wallet.createRandom>): renewal.PushRenewalWallet {
  return {
    address: wallet.address,
    signMessage: (text) => wallet.signMessage(text),
    isCurrent: () => true,
  };
}

async function readPending(worker: BrowserWorker): Promise<renewal.PendingPushRenewal | null> {
  const channel = new MessageChannel();
  const response = new Promise<unknown>((resolve) => {
    channel.port1.onmessage = (event: MessageEvent<unknown>) => resolve(event.data);
  });
  worker.postMessage({ type: 'PUSH_RENEWAL_READ' }, [channel.port2]);
  try {
    const result = (await response) as { ok: boolean; pending: unknown };
    expect(result.ok).toBe(true);
    return result.pending === null ? null : renewal.parsePendingPushRenewal(result.pending);
  } finally {
    channel.port1.close();
    channel.port2.close();
  }
}

async function pushRenewalDelivery(): Promise<void> {
  const storage = new BrowserCacheStorage();
  let worker = new BrowserWorker(origin, storage, source);
  const previous = new BrowserSubscription('https://push.example/subscriptions/expired');
  await worker.dispatch('pushsubscriptionchange', { oldSubscription: previous });
  expect(worker.subscriptions).toEqual([previous.options]);
  expect(worker.broadcasts).toEqual([]);
  const pending = await readPending(worker);
  expect(pending?.subscription).toEqual(worker.nextSubscription.toJSON());
  worker.stop();
  worker = new BrowserWorker(origin, storage, source);
  await worker.dispatch('activate');
  expect(await readPending(worker)).toEqual(pending);

  // A real store outage makes the real registration route return 500.
  await writeFile(storeDirectory, 'temporarily unavailable');
  const wallet = Wallet.createRandom();
  const errors: unknown[] = [];
  let session = renewal.openPushRenewalSession(
    new BrowserContainer(worker),
    sessionWallet(wallet),
    {
      origin,
      onError: (error) => {
        errors.push(error);
      },
    }
  );
  try {
    await expect(session.ready).rejects.toThrow('Push renewal refused (500)');
  } finally {
    session.dispose();
  }
  expect(registrations).toBe(1);
  expect(await readPending(worker)).toEqual(pending);
  worker.stop();
  await rm(storeDirectory);
  worker = new BrowserWorker(origin, storage, source);
  expect(await readPending(worker)).toEqual(pending);
  session = renewal.openPushRenewalSession(new BrowserContainer(worker), sessionWallet(wallet), {
    origin,
    onError: (error) => {
      errors.push(error);
    },
  });
  try {
    expect(await session.ready).toBe(true);
    expect(await session.retry()).toBe(false);
  } finally {
    session.dispose();
  }
  expect(errors).toEqual([]);
  expect(registrations).toBe(2);
  expect(await getSubscription(worker.nextSubscription.endpoint)).toMatchObject({
    ...worker.nextSubscription.toJSON(),
    walletAddress: wallet.address.toLowerCase(),
  });
  expect(await readPending(worker)).toBeNull();
  worker.stop();
  worker = new BrowserWorker(origin, storage, source);
  expect(await readPending(worker)).toBeNull();
}

async function renewalAcknowledgementRace(): Promise<void> {
  const storage = new BrowserCacheStorage();
  const newer = new BrowserSubscription('https://push.example/subscriptions/newer');
  const worker = new BrowserWorker(origin, storage, source, newer, true);
  await worker.dispatch('pushsubscriptionchange', {
    oldSubscription: null,
    newSubscription: new BrowserSubscription('https://push.example/subscriptions/first'),
  });
  const wallet = Wallet.createRandom();
  const current = sessionWallet(wallet);
  const signed: renewal.PushRenewalWallet = {
    ...current,
    async signMessage(message) {
      await worker.dispatch('pushsubscriptionchange', { oldSubscription: newer });
      return wallet.signMessage(message);
    },
  };
  expect(await renewal.synchronizePushRenewal(worker, signed, origin)).toBe(false);
  expect((await readPending(worker))?.subscription.endpoint).toBe(newer.endpoint);
  expect(worker.broadcasts).toHaveLength(2);
  expect(await renewal.synchronizePushRenewal(worker, current, origin)).toBe(true);
  expect(await readPending(worker)).toBeNull();
  expect(await getSubscription(newer.endpoint)).toMatchObject({
    walletAddress: wallet.address.toLowerCase(),
  });
}

async function renewalSessionEligibility(): Promise<void> {
  const worker = new BrowserWorker(origin, new BrowserCacheStorage(), source);
  await worker.dispatch('pushsubscriptionchange', {
    oldSubscription: new BrowserSubscription('https://push.example/old'),
  });
  const pending = await readPending(worker);
  const wallet = Wallet.createRandom();
  let eligible = false;
  const current = { ...sessionWallet(wallet), isCurrent: () => eligible };
  const before = registrations;
  expect(await renewal.synchronizePushRenewal(worker, current, origin)).toBe(false);
  eligible = true;
  const changesDuringSigning: renewal.PushRenewalWallet = {
    ...current,
    async signMessage(message) {
      eligible = false;
      return wallet.signMessage(message);
    },
  };
  expect(await renewal.synchronizePushRenewal(worker, changesDuringSigning, origin)).toBe(false);
  expect(registrations).toBe(before);
  expect(await readPending(worker)).toEqual(pending);
}

test(pushPayloadShape);
test(pushRenewalDelivery);
test(renewalAcknowledgementRace);
test(renewalSessionEligibility);
