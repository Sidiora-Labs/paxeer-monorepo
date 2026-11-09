import { createDecipheriv, createECDH, hkdfSync, randomBytes } from 'node:crypto';
import { once } from 'node:events';
import { mkdtemp, readFile, rm } from 'node:fs/promises';
import https from 'node:https';
import os from 'node:os';
import path from 'node:path';
import webpush from 'web-push';
import { afterEach, expect, test, vi } from 'vitest';
import type { StoredSubscription } from './push-store';

const cleanup: (() => Promise<void>)[] = [];

afterEach(async () => {
  for (const dispose of cleanup.splice(0).reverse()) await dispose();
  vi.unstubAllEnvs();
  vi.resetModules();
});

function browserSubscription(endpoint = 'https://fcm.googleapis.com/fcm/send/device') {
  const browser = createECDH('prime256v1');
  const publicKey = browser.generateKeys();
  const auth = randomBytes(16);
  const subscription: StoredSubscription = {
    endpoint,
    keys: { p256dh: publicKey.toString('base64url'), auth: auth.toString('base64url') },
    walletAddress: `0x${'a'.repeat(40)}`,
    userAgent: 'original-browser',
    createdAt: 1,
    lastActiveAt: 1,
  };
  function consume(encrypted: Buffer): unknown {
    const salt = encrypted.subarray(0, 16);
    const recordSize = encrypted.readUInt32BE(16);
    const keyLength = encrypted[20];
    expect(keyLength).toBe(65);
    const serverKey = encrypted.subarray(21, 21 + keyLength);
    const context = Buffer.concat([Buffer.from('WebPush: info\0'), publicKey, serverKey]);
    const ikm = Buffer.from(
      hkdfSync('sha256', browser.computeSecret(serverKey), auth, context, 32)
    );
    const key = Buffer.from(
      hkdfSync('sha256', ikm, salt, Buffer.from('Content-Encoding: aes128gcm\0'), 16)
    );
    const nonce = Buffer.from(
      hkdfSync('sha256', ikm, salt, Buffer.from('Content-Encoding: nonce\0'), 12)
    );
    const ciphertext = encrypted.subarray(21 + keyLength);
    expect(ciphertext.length).toBeLessThanOrEqual(recordSize);
    const decipher = createDecipheriv('aes-128-gcm', key, nonce);
    decipher.setAuthTag(ciphertext.subarray(-16));
    const plaintext = Buffer.concat([
      decipher.update(ciphertext.subarray(0, -16)),
      decipher.final(),
    ]);
    let delimiter = plaintext.length - 1;
    while (plaintext[delimiter] === 0) delimiter--;
    expect(plaintext[delimiter]).toBe(2);
    return JSON.parse(plaintext.subarray(0, delimiter).toString('utf8'));
  }
  return { subscription, consume };
}

async function openComponents() {
  const directory = await mkdtemp(path.join(os.tmpdir(), 'repair-22-33-168-'));
  cleanup.push(() => rm(directory, { recursive: true, force: true }));
  vi.stubEnv('PUSH_DATA_DIR', directory);
  const vapid = webpush.generateVAPIDKeys();
  vi.stubEnv('NEXT_PUBLIC_VAPID_PUBLIC_KEY', vapid.publicKey);
  vi.stubEnv('VAPID_PRIVATE_KEY', vapid.privateKey);
  vi.stubEnv('VAPID_SUBJECT', 'mailto:push-test@example.test');
  vi.resetModules();
  const store = await import('./push-store');
  const { createPushService } = await import('./push-service');
  return {
    directory,
    store,
    createService(provider: Awaited<ReturnType<typeof pushProvider>>) {
      return createPushService(store.pushStore, {
        vapidDetails: { subject: 'mailto:push-test@example.test', ...vapid },
        createAgent: provider.createAgent,
      });
    },
  };
}

async function pushProvider(browser: ReturnType<typeof browserSubscription>) {
  const fixture = path.join(__dirname, 'fixtures/repair-22-33-168');
  const [key, cert] = await Promise.all([
    readFile(path.join(fixture, 'push-provider.key')),
    readFile(path.join(fixture, 'push-provider.crt')),
  ]);
  const requests: { url: string; payload: unknown; authorization: string | undefined }[] = [];
  const errors: unknown[] = [];
  let redirect: string | undefined;
  let status = 201;
  const server = https.createServer({ key, cert }, (request, response) => {
    const chunks: Buffer[] = [];
    request.on('data', (chunk: Buffer) => chunks.push(chunk));
    request.on('end', () => {
      try {
        requests.push({
          url: request.url ?? '',
          payload: browser.consume(Buffer.concat(chunks)),
          authorization: request.headers.authorization,
        });
        if (redirect !== undefined) response.setHeader('Location', redirect);
        response.writeHead(status).end();
      } catch (error) {
        errors.push(error);
        response.writeHead(400).end();
      }
    });
  });
  server.listen(0, '127.0.0.1');
  await once(server, 'listening');
  const address = server.address();
  if (!address || typeof address === 'string') throw new Error('Missing fixture address');
  const port = address.port;
  // Route real TLS sockets to the isolated provider; hostname verification stays enabled.
  function createAgent(options: https.AgentOptions): https.Agent {
    return new https.Agent({
      ...options,
      ca: cert,
      port,
      lookup: (_hostname, options, callback) => {
        if (options.all) callback(null, [{ address: '127.0.0.1', family: 4 }]);
        else callback(null, '127.0.0.1', 4);
      },
    });
  }
  cleanup.push(async () => {
    server.closeAllConnections();
    await new Promise<void>((resolve, reject) =>
      server.close((error) => (error ? reject(error) : resolve()))
    );
  });
  return {
    requests,
    errors,
    createAgent,
    redirectTo(location: string, code: number) {
      redirect = location;
      status = code;
    },
  };
}

async function push_destination_egress_rejects_unsafe_origins_before_storage_and_delivery() {
  const { store, createService, directory } = await openComponents();
  const browser = browserSubscription();
  const provider = await pushProvider(browser);
  const service = createService(provider);
  for (const endpoint of [
    'https://127.0.0.1/private',
    'https://[::1]/private',
    'https://169.254.169.254/metadata',
    'https://10.0.0.1/private',
    'https://localhost/private',
    'https://rebinding.attacker.test/push',
    'https://fcm.googleapis.com.attacker.test/push',
    'https://attacker.test/?https://fcm.googleapis.com',
    'https://user:password@fcm.googleapis.com/push',
    'https://fcm.googleapis.com:8443/push',
    'http://fcm.googleapis.com/push',
    'https://fcm.googleapis.com/push#fragment',
    'https://fcm.googleapis.com\\@attacker.test/push',
    'https://fcm.googleapis.com\n/push',
  ]) {
    const unsafe = { ...browser.subscription, endpoint };
    await expect(store.upsertSubscription(unsafe)).rejects.toThrow();
    await expect(
      service.sendToSubscription(unsafe, { title: 'Blocked', body: 'Blocked' })
    ).rejects.toThrow();
  }
  expect(await store.getAllSubscriptions()).toEqual([]);
  await expect(readFile(path.join(directory, 'subscriptions.json'))).rejects.toMatchObject({
    code: 'ENOENT',
  });
  expect(provider.requests).toEqual([]);
  expect(provider.errors).toEqual([]);
  for (const origin of [
    'https://fcm.googleapis.com',
    'https://updates.push.services.mozilla.com',
    'https://web.push.apple.com',
  ]) {
    expect(
      store.parseStoredSubscription({ ...browser.subscription, endpoint: `${origin}/push` })
    ).toMatchObject({ endpoint: `${origin}/push`, keys: browser.subscription.keys });
  }
}

async function push_destination_egress_validates_browser_key_encodings_before_persistence() {
  const { store, createService } = await openComponents();
  const browser = browserSubscription();
  const provider = await pushProvider(browser);
  const service = createService(provider);
  const offCurve = Buffer.alloc(65);
  offCurve[0] = 4;
  const nonCanonicalAuth = browser.subscription.keys.auth.slice(0, -1) + 'B';
  for (const keys of [
    { ...browser.subscription.keys, p256dh: 'a'.repeat(32) },
    { ...browser.subscription.keys, p256dh: '!'.repeat(87) },
    { ...browser.subscription.keys, p256dh: offCurve.toString('base64url') },
    { ...browser.subscription.keys, p256dh: ` ${browser.subscription.keys.p256dh}` },
    { ...browser.subscription.keys, auth: 'b'.repeat(16) },
    { ...browser.subscription.keys, auth: '!'.repeat(22) },
    { ...browser.subscription.keys, auth: nonCanonicalAuth },
    { ...browser.subscription.keys, auth: `${browser.subscription.keys.auth}==` },
  ]) {
    const invalid = { ...browser.subscription, keys };
    await expect(store.upsertSubscription(invalid)).rejects.toThrow();
    await expect(
      service.sendToSubscription(invalid, { title: 'Blocked', body: 'Invalid keys' })
    ).rejects.toThrow();
  }
  expect(await store.getAllSubscriptions()).toEqual([]);
  expect(provider.requests).toEqual([]);
}

async function push_destination_egress_never_follows_provider_redirects() {
  const { store, createService } = await openComponents();
  const browser = browserSubscription();
  const provider = await pushProvider(browser);
  const service = createService(provider);
  await store.upsertSubscription(browser.subscription);
  for (const [index, location] of [
    'https://127.0.0.1/private',
    'https://rebinding.attacker.test/push',
    'https://fcm.googleapis.com/redirected',
    '/redirected',
  ].entries()) {
    provider.redirectTo(location, [301, 302, 307, 308][index]);
    expect(
      await service.sendToWallet(browser.subscription.walletAddress, {
        title: 'Redirect',
        body: 'Do not forward this payload',
      })
    ).toEqual({ total: 1, sent: 0, failed: 1, removed: 0 });
    expect(provider.requests).toHaveLength(index + 1);
    expect(provider.requests[index].url).toBe('/fcm/send/device');
    expect(provider.requests[index].payload).toMatchObject({ title: 'Redirect' });
    expect(provider.requests[index].authorization).toMatch(/^vapid t=.+, k=.+$/);
  }
  expect(
    (await store.getSubscription(browser.subscription.endpoint))?.lastNotifiedAt
  ).toBeUndefined();
  expect(provider.errors).toEqual([]);
}

async function push_upsert_metadata_survives_notification_resubscription_and_reopen() {
  const { store, createService, directory } = await openComponents();
  const browser = browserSubscription();
  const provider = await pushProvider(browser);
  const service = createService(provider);
  await store.upsertSubscription({
    ...browser.subscription,
    tags: ['browser-injected'],
    lastNotifiedAt: 123,
  });
  const initial = await store.getSubscription(browser.subscription.endpoint);
  expect(initial?.createdAt).toBeGreaterThan(1);
  expect(initial?.tags).toBeUndefined();
  expect(initial?.lastNotifiedAt).toBeUndefined();
  expect(await store.addTagToWallet(browser.subscription.walletAddress, 'administrator-tag')).toBe(
    1
  );
  expect(
    await service.sendToTag('administrator-tag', { title: 'Delivered', body: 'Real payload' })
  ).toEqual({ total: 1, sent: 1, failed: 0, removed: 0 });
  expect(provider.requests).toHaveLength(1);
  expect(provider.requests[0].payload).toMatchObject({ title: 'Delivered', body: 'Real payload' });
  const notified = await store.getSubscription(browser.subscription.endpoint);
  expect(notified?.lastNotifiedAt).toBeGreaterThan(0);
  const refreshed = browserSubscription(browser.subscription.endpoint).subscription;
  // Release module instances, then re-import both the consumer and its durable store.
  vi.resetModules();
  const restartedStore = await import('./push-store');
  await import('./push-service');
  expect(await restartedStore.getSubscription(browser.subscription.endpoint)).toEqual(notified);
  await restartedStore.upsertSubscription({
    ...refreshed,
    walletAddress: browser.subscription.walletAddress.toUpperCase().replace('0X', '0x'),
    userAgent: 'refreshed-browser',
  });
  await restartedStore.upsertSubscription({
    ...refreshed,
    userAgent: 'refreshed-browser',
    tags: ['replace-tag'],
    lastNotifiedAt: 0,
  });
  vi.resetModules();
  const reopened = await import('./push-store');
  const recovered = await reopened.getSubscription(browser.subscription.endpoint);
  expect(recovered).toEqual({
    ...notified,
    keys: refreshed.keys,
    userAgent: 'refreshed-browser',
    lastActiveAt: expect.any(Number),
  });
  expect(recovered?.lastActiveAt).toBeGreaterThanOrEqual(notified?.lastActiveAt ?? Infinity);
  expect(await reopened.getSubscriptionsByTag('administrator-tag')).toEqual([recovered]);
  expect(await reopened.getSubscriptionsByTag('replace-tag')).toEqual([]);
  const persisted = JSON.parse(await readFile(path.join(directory, 'subscriptions.json'), 'utf8'));
  expect(persisted.records).toEqual([recovered]);
  expect(provider.errors).toEqual([]);
}

async function push_upsert_metadata_rejects_owner_transfer_without_mutating_durable_state() {
  const { store, directory } = await openComponents();
  const browser = browserSubscription();
  await store.upsertSubscription(browser.subscription);
  await store.addTagToWallet(browser.subscription.walletAddress, 'owner-tag');
  const before = await readFile(path.join(directory, 'subscriptions.json'), 'utf8');
  await expect(
    store.upsertSubscription({
      ...browser.subscription,
      walletAddress: `0x${'b'.repeat(40)}`,
    })
  ).rejects.toThrow('Subscription owner cannot change');
  expect(await readFile(path.join(directory, 'subscriptions.json'), 'utf8')).toBe(before);
  vi.resetModules();
  const reopened = await import('./push-store');
  expect(await reopened.getSubscriptionsByWallet(`0x${'b'.repeat(40)}`)).toEqual([]);
  expect(await reopened.getSubscriptionsByWallet(browser.subscription.walletAddress)).toHaveLength(
    1
  );
  expect((await reopened.getSubscription(browser.subscription.endpoint))?.tags).toEqual([
    'owner-tag',
  ]);
}

test(
  push_destination_egress_rejects_unsafe_origins_before_storage_and_delivery.name,
  push_destination_egress_rejects_unsafe_origins_before_storage_and_delivery
);
test(
  push_destination_egress_validates_browser_key_encodings_before_persistence.name,
  push_destination_egress_validates_browser_key_encodings_before_persistence
);
test(
  push_destination_egress_never_follows_provider_redirects.name,
  push_destination_egress_never_follows_provider_redirects
);
test(
  push_upsert_metadata_survives_notification_resubscription_and_reopen.name,
  push_upsert_metadata_survives_notification_resubscription_and_reopen
);
test(
  push_upsert_metadata_rejects_owner_transfer_without_mutating_durable_state.name,
  push_upsert_metadata_rejects_owner_transfer_without_mutating_durable_state
);
