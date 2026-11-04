import { afterAll, beforeAll, expect, test } from 'vitest';
import {
  createServer,
  type IncomingMessage,
  type Server,
  type ServerResponse,
} from 'node:http';
import {
  mkdtemp,
  readFile,
  writeFile,
  rename,
  rm,
  readdir,
} from 'node:fs/promises';
import { tmpdir } from 'node:os';
import { join } from 'node:path';
import {
  randomUUID,
  randomBytes,
  createHash,
  generateKeyPairSync,
  sign,
  verify,
} from 'node:crypto';
import { privateKeyToAccount } from 'viem/accounts';
import { verifyMessage, type Hex } from 'viem';
import {
  MobileServices,
  decodeCurrentIdentity,
  type CurrentIdentity,
  type PublicConfiguration,
} from '../../src/runtime/service-client.ts';
import {
  SnapshotCache,
  type PublicSnapshotStorage,
} from '../../src/runtime/offline-cache.ts';
import {
  SdkClient,
  decodeV2Contract,
} from '../../../../../sdk/launchpad/packages/sdk/src/transport.ts';
import { decodeContract } from '../../../../../sdk/launchpad/packages/sdk/src/validate.ts';
import type { ContractTypes } from '../../../../../sdk/launchpad/packages/sdk/src/types.gen.ts';

const pool = '0x2222222222222222222222222222222222222222';
const signal = new AbortController().signal;
// Published deterministic fixture key; no account or endpoint outside this test is used.
const privateKey: Hex = `0x${'11'.repeat(32)}`;
const account = privateKeyToAccount(privateKey);
const installation = '88888888-8888-4888-8888-888888888888';
const proofKeys = generateKeyPairSync('ed25519');
let deviceBody: ContractTypes['DeviceRegisterBody'];
let directory: string;
let services: MobileServices;
let gateway: {
  origin: string;
  privateKey: Hex;
  cookie: string;
  refreshCookie: string;
  csrf: string;
  identity: CurrentIdentity;
};
const user = {
  installation,
  get deviceBody() {
    return deviceBody;
  },
};
let live: { origin: string };
let signatures = 0;
let refreshes = 0;
const servers: Server[] = [];
const fixtureErrors: unknown[] = [];
const delivered: {
  path: string;
  method: string;
  body: string;
  operation: string;
  signature: string;
  id: string;
  requestId: string;
  legacySignature: string;
}[] = [];
let loseNextResponse: string | undefined;
let watchRevision = '1';
let watchEntries: ContractTypes['WatchlistEntry'][] = [];
let profile: ContractTypes['Profile'];
let preferences: ContractTypes['Preferences'];
let device: ContractTypes['PushDevice'] | undefined;
type FixtureReply = {
  contract: keyof ContractTypes;
  value: unknown;
  status: number;
};
const ledger = new Map<string, { fingerprint: string; reply: FixtureReply }>();
const nonces = new Set<string>();

function envelope(data: unknown, revision = '1') {
  return {
    requestId: randomUUID(),
    data,
    watermark: null,
    revision,
    quality: 'complete',
    nextCursor: null,
    generatedAt: new Date().toISOString(),
  };
}
function sendV1(response: ServerResponse, reply: FixtureReply) {
  const value = decodeContract(reply.contract, reply.value);
  response.writeHead(reply.status, { 'Content-Type': 'application/json' });
  response.end(reply.status === 204 ? undefined : JSON.stringify(value));
}
function sendV2(
  response: ServerResponse,
  name: Parameters<typeof decodeV2Contract>[0],
  value: unknown,
  status = 200,
  headers: Record<string, string> = {},
) {
  const validated = decodeV2Contract(name, value);
  response.writeHead(status, {
    'Content-Type': 'application/json',
    ...headers,
  });
  response.end(JSON.stringify(validated));
}
class FixtureRefusal extends Error {}
function requireFixture(
  condition: unknown,
  message: string,
): asserts condition {
  if (!condition) throw new FixtureRefusal(message);
}
async function requestBody(request: IncomingMessage) {
  const chunks: Buffer[] = [];
  for await (const chunk of request) chunks.push(Buffer.from(chunk));
  return Buffer.concat(chunks).toString('utf8');
}
async function listen(
  handler: (
    request: IncomingMessage,
    response: ServerResponse,
  ) => Promise<void>,
) {
  const server = createServer((request, response) => {
    void handler(request, response).catch((error: unknown) => {
      if (!(error instanceof FixtureRefusal)) fixtureErrors.push(error);
      sendV2(
        response,
        'Error',
        {
          code: 'FORBIDDEN',
          message: error instanceof Error ? error.message : 'fixture failure',
          requestId: randomUUID(),
          retryable: false,
          retryAfterMs: null,
          fieldErrors: [],
          details: null,
        },
        error instanceof FixtureRefusal ? 403 : 500,
      );
    });
  });
  servers.push(server);
  await new Promise<void>((resolve, reject) => {
    server.once('error', reject);
    server.listen(0, '127.0.0.1', resolve);
  });
  const address = server.address();
  if (!address || typeof address === 'string')
    throw new Error('Loopback fixture not listening');
  return `http://127.0.0.1:${address.port}`;
}

async function authorizeFixture(
  request: IncomingMessage,
  body: string,
  action: string,
) {
  requireFixture(
    request.headers['x-fixture-account'] === gateway.identity.accountId &&
      request.headers['x-fixture-session'] === gateway.identity.sessionId &&
      request.headers['x-fixture-epoch'] === String(gateway.identity.authEpoch),
    'current session binding required',
  );
  const encoded = request.headers['x-kindle-operation'];
  const signature = request.headers['x-kindle-operation-signature'];
  const id = request.headers['idempotency-key'];
  requireFixture(
    typeof encoded === 'string' &&
      typeof signature === 'string' &&
      /^0x[0-9a-f]{130}$/i.test(signature) &&
      typeof id === 'string' &&
      /^[0-9a-f-]{36}$/i.test(id),
    'operation headers required',
  );
  const operation = JSON.parse(
    Buffer.from(encoded, 'base64url').toString('utf8'),
  ) as Record<string, unknown>;
  requireFixture(
    operation.version === 2 &&
      operation.chainId === '125' &&
      operation.environment === 'production' &&
      operation.actor === gateway.identity.wallet &&
      operation.action === action &&
      operation.method === request.method &&
      operation.path === request.url &&
      operation.bodyHash ===
        createHash('sha256').update(body, 'utf8').digest('hex') &&
      typeof operation.nonce === 'string' &&
      /^[0-9a-f]{64}$/.test(operation.nonce) &&
      typeof operation.issuedAt === 'number' &&
      typeof operation.expiresAt === 'number' &&
      operation.issuedAt <= Date.now() / 1000 &&
      operation.expiresAt > Date.now() / 1000 &&
      operation.expiresAt - operation.issuedAt <= 300,
    'operation scope required',
  );
  // Independently reconstruct the declaration order and escaping the authorizer signs.
  const canonical = JSON.stringify({
    version: operation.version,
    chainId: operation.chainId,
    environment: operation.environment,
    actor: operation.actor,
    action: operation.action,
    method: operation.method,
    path: operation.path,
    bodyHash: operation.bodyHash,
    nonce: operation.nonce,
    issuedAt: operation.issuedAt,
    expiresAt: operation.expiresAt,
  }).replace(
    /[<>&\u2028\u2029]/g,
    (character) =>
      `\\u${character.charCodeAt(0).toString(16).padStart(4, '0')}`,
  );
  requireFixture(
    await verifyMessage({
      address: account.address,
      message: `KindleLaunch signed operation v2\n${canonical}`,
      signature: signature as Hex,
    }),
    'wallet signature required',
  );
  const fingerprint = JSON.stringify([
    gateway.identity.accountId,
    gateway.identity.sessionId,
    gateway.identity.authEpoch,
    request.method,
    request.url,
    body,
    encoded,
    signature,
  ]);
  const recorded = ledger.get(id);
  if (recorded) {
    requireFixture(
      recorded.fingerprint === fingerprint,
      'idempotency conflict',
    );
    return {
      id,
      fingerprint,
      nonce: operation.nonce,
      recorded: recorded.reply,
    };
  }
  requireFixture(
    !nonces.has(operation.nonce),
    'operation replay with a different identity',
  );
  return { id, fingerprint, nonce: operation.nonce, recorded: undefined };
}

async function accountFixture(
  request: IncomingMessage,
  response: ServerResponse,
) {
  const target = new URL(request.url!, gateway.origin);
  const path = target.pathname,
    method = request.method;
  requireFixture(
    request.headers['x-fixture-account'] === gateway.identity.accountId,
    'authenticated account required',
  );
  if (method === 'GET') {
    if (path === '/v1/me/watchlist') {
      requireFixture(
        target.searchParams.get('limit') === '50' &&
          [...target.searchParams].length === 1,
        'recorded watchlist query required',
      );
      return sendV1(response, {
        contract: 'WatchlistGetResponse',
        value: envelope({ items: watchEntries }, watchRevision),
        status: 200,
      });
    }
    if (path === `/v1/profiles/${gateway.identity.wallet}`)
      return sendV1(response, {
        contract: 'ProfileGetResponse',
        value: envelope({ profile }, profile.revision),
        status: 200,
      });
    if (path === '/v1/me/preferences')
      return sendV1(response, {
        contract: 'PreferencesReadResponse',
        value: envelope({ preferences }, preferences.revision),
        status: 200,
      });
    throw new FixtureRefusal('unrecorded fixture read');
  }
  const body = await requestBody(request);
  const action =
    path === `/v1/me/watchlist/${pool}`
      ? method === 'PUT'
        ? 'watchlist.update'
        : 'watchlist.remove'
      : path === `/v1/profiles/${gateway.identity.wallet}`
        ? 'profile.update'
        : path === '/v1/me/preferences'
          ? 'preferences.update'
          : path === `/v1/me/devices/${installation}`
            ? method === 'PUT'
              ? 'device.register'
              : 'device.revoke'
            : undefined;
  requireFixture(action, 'unrecorded fixture write');
  const authority = await authorizeFixture(request, body, action);
  if (authority.recorded) return sendV1(response, authority.recorded);
  let reply: FixtureReply;
  if (action === 'watchlist.update') {
    requireFixture(method === 'PUT', 'watch method');
    const input = decodeContract('WatchlistPutBody', JSON.parse(body));
    const entry = decodeContract('WatchlistEntry', {
      poolAddress: pool,
      wallet: gateway.identity.wallet,
      alertEnabled: input.alertEnabled,
      addedAt: new Date().toISOString(),
      order: 0,
    });
    watchEntries = [entry];
    watchRevision = String(Number(watchRevision) + 1);
    reply = {
      contract: 'WatchlistPutResponse',
      value: envelope({ entry }, watchRevision),
      status: 200,
    };
  } else if (action === 'watchlist.remove') {
    requireFixture(
      method === 'DELETE' && body === '',
      'empty DELETE body required',
    );
    watchEntries = [];
    watchRevision = String(Number(watchRevision) + 1);
    reply = { contract: 'WatchlistDeleteResponse', value: null, status: 204 };
  } else if (action === 'profile.update') {
    requireFixture(method === 'PATCH', 'profile method');
    const { expectedRevision, ...patch } = decodeContract(
      'ProfilePatchBody',
      JSON.parse(body),
    );
    requireFixture(
      expectedRevision === profile.revision,
      'profile revision conflict',
    );
    profile = decodeContract('Profile', {
      ...profile,
      ...patch,
      revision: String(Number(profile.revision) + 1),
    });
    reply = {
      contract: 'ProfilePatchResponse',
      value: envelope({ profile }, profile.revision),
      status: 200,
    };
  } else if (action === 'preferences.update') {
    requireFixture(method === 'PATCH', 'preferences method');
    const input = decodeContract('PreferencesUpdateBody', JSON.parse(body));
    requireFixture(
      input.expectedRevision === preferences.revision,
      'preferences revision conflict',
    );
    preferences = decodeContract('Preferences', {
      ...preferences,
      locale: input.locale ?? preferences.locale,
      timezone: input.timezone ?? preferences.timezone,
      revision: String(Number(preferences.revision) + 1),
    });
    reply = {
      contract: 'PreferencesUpdateResponse',
      value: envelope({ preferences }, preferences.revision),
      status: 200,
    };
  } else if (action === 'device.register') {
    requireFixture(method === 'PUT', 'device method');
    const input = decodeContract('DeviceRegisterBody', JSON.parse(body));
    const proof = Buffer.from(input.deviceProof, 'base64url');
    requireFixture(
      proof.length === 171 &&
        proof[0] === 2 &&
        proof[105] === 2 &&
        proof[106] === 2 &&
        proof
          .subarray(1, 17)
          .equals(
            Buffer.from(gateway.identity.accountId.replace(/-/g, ''), 'hex'),
          ) &&
        proof
          .subarray(17, 33)
          .equals(
            Buffer.from(gateway.identity.sessionId.replace(/-/g, ''), 'hex'),
          ) &&
        proof
          .subarray(33, 49)
          .equals(Buffer.from(installation.replace(/-/g, ''), 'hex')) &&
        proof
          .subarray(49, 81)
          .equals(createHash('sha256').update(input.token).digest()) &&
        proof.readBigUInt64BE(97) > BigInt(Math.floor(Date.now() / 1000)) &&
        verify(
          null,
          proof.subarray(0, 107),
          proofKeys.publicKey,
          proof.subarray(107),
        ),
      'installation-bound provider signature required',
    );
    device = decodeContract('PushDevice', {
      deviceId: installation,
      encryptedToken: 'recorded-encrypted-provider-token',
      lastSeenAt: new Date().toISOString(),
      platform: input.platform,
      provider: input.provider,
      sessionId: gateway.identity.sessionId,
      state: 'active',
    });
    reply = {
      contract: 'DeviceRegisterResponse',
      value: envelope({
        deviceId: installation,
        revision: '1',
        state: 'active',
      }),
      status: 200,
    };
  } else {
    requireFixture(
      method === 'DELETE' && body === '' && device,
      'registered device required',
    );
    device = decodeContract('PushDevice', { ...device, state: 'revoked' });
    reply = { contract: 'DeviceRevokeResponse', value: null, status: 204 };
  }
  decodeContract(reply.contract, reply.value);
  ledger.set(authority.id, { fingerprint: authority.fingerprint, reply });
  nonces.add(authority.nonce);
  sendV1(response, reply);
}
function configuration(origin = 'https://kindle.test'): PublicConfiguration {
  return {
    apiOrigin: origin,
    identityOrigin: origin,
    mediaOrigin: origin,
    canonicalOrigin: origin,
    identityURI: origin,
    streamOrigin: origin.replace(/^http/, 'ws'),
    approvedStreamOrigins: [origin.replace(/^http/, 'ws')],
    production: origin.startsWith('https:'),
    manifestDigest: 'a'.repeat(64),
    manifestURL: `${origin}/manifest.json`,
    optionalFeatures: { chat: false, live: true },
    approvedPlaybackOrigins: ['https://livepeercdn.studio'],
  };
}
async function cookieTransport(
  url: string,
  init: RequestInit = {},
): Promise<Response> {
  const target = new URL(url);
  if (target.origin !== 'https://kindle.test')
    throw new Error('Unrecorded fixture origin');
  const headers = new Headers(init.headers);
  // Preserve native cookie-provider headers on an owned loopback connection.
  headers.set(
    'Cookie',
    `__Host-kindle_access=${gateway.cookie}; __Host-kindle_refresh=${gateway.refreshCookie}; __Host-kindle_csrf=${gateway.csrf}`,
  );
  const method = init.method ?? 'GET';
  if (method !== 'GET' && !headers.has('X-CSRF-Token'))
    headers.set('X-CSRF-Token', gateway.csrf);
  if (method !== 'GET' && target.pathname.startsWith('/v1/')) {
    delivered.push({
      path: target.pathname,
      method,
      body: typeof init.body === 'string' ? init.body : '',
      operation: headers.get('X-Kindle-Operation') ?? '',
      signature: headers.get('X-Kindle-Operation-Signature') ?? '',
      id: headers.get('Idempotency-Key') ?? '',
      requestId: headers.get('X-Request-ID') ?? '',
      legacySignature: headers.get('X-Kindle-Signature') ?? '',
    });
  }
  const response = await fetch(
    `${gateway.origin}${target.pathname}${target.search}`,
    { ...init, headers },
  );
  for (const value of response.headers.getSetCookie()) {
    const pair = value.split(';', 1)[0],
      index = pair.indexOf('=');
    const name = pair.slice(0, index),
      cookie = pair.slice(index + 1);
    if (name === '__Host-kindle_access') gateway.cookie = cookie;
    if (name === '__Host-kindle_refresh') gateway.refreshCookie = cookie;
    if (name === '__Host-kindle_csrf') gateway.csrf = cookie;
  }
  if (loseNextResponse === `${method} ${target.pathname}` && response.ok) {
    loseNextResponse = undefined;
    await response.arrayBuffer();
    throw new Error('Response lost after the loopback fixture committed');
  }
  return response;
}
function createServices() {
  const result = new MobileServices(
    configuration(),
    undefined,
    cookieTransport,
    async () => randomUUID(),
  );
  const account = privateKeyToAccount(gateway.privateKey);
  result.setOperationWallet({
    supportedChain: 125,
    connectedAccount: account.address,
    async signMessage(message, cancellation) {
      cancellation.throwIfAborted();
      signatures++;
      return account.signMessage({ message });
    },
  });
  return result;
}
beforeAll(async () => {
  directory = await mkdtemp(join(tmpdir(), 'mobile-repair-22-31-30-'));
  const identity = decodeCurrentIdentity({
    accountId: '33333333-3333-4333-8333-333333333333',
    sessionId: '44444444-4444-4444-8444-444444444444',
    wallet: account.address.toLowerCase(),
    authEpoch: 1,
    proofAt: new Date().toISOString(),
    accessExpiresAt: new Date(Date.now() + 3600_000).toISOString(),
  });
  const proof = Buffer.alloc(107);
  proof[0] = 2;
  Buffer.from(identity.accountId.replace(/-/g, ''), 'hex').copy(proof, 1);
  Buffer.from(identity.sessionId.replace(/-/g, ''), 'hex').copy(proof, 17);
  Buffer.from(installation.replace(/-/g, ''), 'hex').copy(proof, 33);
  const token = 'recorded-installation-provider-token';
  createHash('sha256').update(token).digest().copy(proof, 49);
  randomBytes(16).copy(proof, 81);
  proof.writeBigUInt64BE(BigInt(Math.floor(Date.now() / 1000) + 300), 97);
  proof[105] = 2;
  proof[106] = 2;
  deviceBody = decodeContract('DeviceRegisterBody', {
    platform: 'ios',
    provider: 'apns',
    token,
    deviceProof: Buffer.concat([
      proof,
      sign(null, proof, proofKeys.privateKey),
    ]).toString('base64url'),
  });
  profile = decodeContract('Profile', {
    wallet: identity.wallet,
    displayName: 'Mobile owner',
    bio: '',
    revision: '1',
    avatarAssetId: null,
    bannerAssetId: null,
    website: null,
  });
  preferences = decodeContract('Preferences', {
    analyticsConsent: false,
    channels: ['inapp', 'push'],
    locale: 'en',
    timezone: 'UTC',
    revision: '1',
    notificationCategories: {
      trade: true,
      priceAlert: true,
      community: true,
      mention: true,
      moderation: true,
      reward: true,
    },
    quietHours: { start: '00:00', end: '00:00', timezone: 'UTC' },
  });
  const upstream = await listen(accountFixture);
  gateway = {
    origin: '',
    privateKey,
    cookie: 'recorded-access-cookie',
    refreshCookie: 'recorded-refresh-cookie',
    csrf: 'c'.repeat(64),
    identity,
  };
  gateway.origin = await listen(async (request, response) => {
    const target = new URL(request.url!, gateway.origin);
    if (request.method === 'GET' && target.pathname === '/v2/streams') {
      requireFixture(
        target.searchParams.get('status') === 'live' &&
          target.searchParams.get('limit') === '50' &&
          [...target.searchParams].length === 2,
        'recorded stream query required',
      );
      // Recorded wire shape of AdaptV2Stream/AdaptV2StreamList, validated by its generated codec.
      return sendV2(
        response,
        'StreamListResponse',
        envelope({
          items: [null, 0].map((count, index) => ({
            streamId:
              index === 0
                ? '55555555-5555-4555-8555-555555555555'
                : '66666666-6666-4666-8666-666666666666',
            poolAddress: pool,
            creatorWallet: identity.wallet,
            title: 'Real stream producer',
            status: 'live',
            playbackUrl: `https://livepeercdn.studio/${index === 0 ? 'live' : 'zero'}.m3u8`,
            viewerCount: count,
            viewerQuality: count === null ? 'unavailable' : 'verified',
            revision: '1',
            generatedAt: new Date().toISOString(),
          })),
        }),
      );
    }
    requireFixture(
      request.headers.origin === 'https://kindle.test' &&
        request.headers.cookie ===
          `__Host-kindle_access=${gateway.cookie}; __Host-kindle_refresh=${gateway.refreshCookie}; __Host-kindle_csrf=${gateway.csrf}`,
      'cookie session required',
    );
    if (request.method === 'GET' && request.url === '/v2/auth/session') {
      const principal = decodeCurrentIdentity(gateway.identity);
      // This internal principal contract uses MobileServices' strict identity decoder.
      response.writeHead(200, { 'Content-Type': 'application/json' });
      response.end(JSON.stringify({ data: principal }));
      return;
    }
    requireFixture(
      request.method === 'GET' ||
        request.headers['x-csrf-token'] === gateway.csrf,
      'CSRF proof required',
    );
    if (
      request.method === 'POST' &&
      request.url === '/v2/auth/sessions:refresh'
    ) {
      refreshes++;
      return sendV2(
        response,
        'RefreshResponse',
        envelope({
          accessToken: 'recorded-access-token',
          session: {
            sessionId: identity.sessionId,
            wallet: identity.wallet,
            deviceLabel: 'Mobile fixture',
            scopes: ['user'],
            accessExpiresAt: identity.accessExpiresAt,
            refreshExpiresAt: new Date(Date.now() + 86400_000).toISOString(),
            revokedAt: null,
          },
        }),
        200,
        { 'X-CSRF-Token': gateway.csrf },
      );
    }
    requireFixture(request.url?.startsWith('/v1/'), 'unrecorded fixture route');
    const body = await requestBody(request);
    const headers = new Headers();
    for (const [name, value] of Object.entries(request.headers)) {
      if (
        typeof value === 'string' &&
        !['host', 'connection', 'content-length'].includes(name)
      )
        headers.set(name, value);
    }
    headers.set('X-Fixture-Account', gateway.identity.accountId);
    headers.set('X-Fixture-Session', gateway.identity.sessionId);
    headers.set('X-Fixture-Epoch', String(gateway.identity.authEpoch));
    const forwarded = await fetch(`${upstream}${request.url}`, {
      method: request.method,
      headers,
      ...(body ? { body } : {}),
    });
    response.writeHead(forwarded.status, {
      'Content-Type': 'application/json',
    });
    response.end(Buffer.from(await forwarded.arrayBuffer()));
  });
  live = { origin: gateway.origin };
  services = createServices();
  const restored = await services.currentSession(signal);
  expect(restored).toEqual(identity);
  expect(Date.parse(restored.accessExpiresAt)).toBeGreaterThan(Date.now());
  expect(refreshes).toBe(1);
});
afterAll(async () => {
  try {
    await Promise.all(
      servers.map(
        (server) =>
          new Promise<void>((resolve, reject) => {
            server.close((error) => (error ? reject(error) : resolve()));
            server.closeAllConnections();
          }),
      ),
    );
    expect(fixtureErrors).toEqual([]);
    expect(ledger.size).toBe(6);
    expect(device?.state).toBe('revoked');
  } finally {
    if (directory) await rm(directory, { recursive: true, force: true });
  }
});

function assertSignedDelivery(action: string, previous: number) {
  const [first, retry] = delivered.slice(previous);
  expect(delivered.slice(previous)).toHaveLength(2);
  expect(retry).toEqual(first);
  const envelope = JSON.parse(
    Buffer.from(first.operation, 'base64url').toString('utf8'),
  );
  expect(envelope.action).toBe(action);
  expect(envelope.actor).toBe(gateway.identity.wallet);
  expect(envelope.method).toBe(first.method);
  expect(envelope.path).toBe(first.path);
  expect(envelope.version).toBe(2);
  expect(envelope.bodyHash).toBe(
    createHash('sha256').update(first.body, 'utf8').digest('hex'),
  );
  expect(first.signature).toMatch(/^0x[0-9a-f]{130}$/i);
  expect(first.id).toMatch(/^[0-9a-f-]{36}$/i);
  expect(first.requestId).toBe(first.id);
  expect(first.legacySignature).toBe('');
}
async function mobileWatchWrite() {
  const previous = delivered.length,
    signed = signatures;
  loseNextResponse = `PUT /v1/me/watchlist/${pool}`;
  await expect(services.watch(pool, true, signal)).rejects.toThrow();
  const committed = await services.watchlist(signal);
  expect(committed.data.items).toHaveLength(1);
  expect(committed.data.items[0].poolAddress).toBe(pool);
  expect(committed.data.items[0].alertEnabled).toBe(true);
  await services.watch(pool, true, signal);
  expect((await services.watchlist(signal)).revision).toBe(committed.revision);
  expect(signatures).toBe(signed + 1);
  assertSignedDelivery('watchlist.update', previous);
}
async function mobileUnwatchWrite() {
  const previous = delivered.length,
    signed = signatures;
  loseNextResponse = `DELETE /v1/me/watchlist/${pool}`;
  await expect(services.unwatch(pool, signal)).rejects.toThrow();
  const committed = await services.watchlist(signal);
  expect(committed.data.items).toEqual([]);
  expect(await services.unwatch(pool, signal)).toBeNull();
  expect((await services.watchlist(signal)).revision).toBe(committed.revision);
  expect(signatures).toBe(signed + 1);
  assertSignedDelivery('watchlist.remove', previous);
  expect(delivered[previous].body).toBe('');
}
function readOwnerProfile(wallet: string) {
  return services.sdk.request(`/v1/profiles/${wallet}`, 'ProfileGetResponse', {
    signal,
    authenticated: true,
  });
}
async function mobileProfileWrite() {
  const wallet = gateway.identity.wallet;
  const profile = (await readOwnerProfile(wallet)).data.profile;
  expect(profile.displayName).toBe('Mobile owner');
  const body = {
    expectedRevision: profile.revision,
    displayName: 'Mobile owner',
    bio: 'Exact <body> & UTF-8: é',
  };
  const previous = delivered.length,
    signed = signatures;
  loseNextResponse = `PATCH /v1/profiles/${wallet}`;
  await expect(services.editProfile(wallet, body, signal)).rejects.toThrow();
  const committed = (await readOwnerProfile(wallet)).data.profile;
  expect(committed.displayName).toBe(body.displayName);
  expect(committed.bio).toBe(body.bio);
  expect(
    (await services.editProfile(wallet, body, signal)).data.profile.revision,
  ).toBe(committed.revision);
  expect(signatures).toBe(signed + 1);
  assertSignedDelivery('profile.update', previous);
  expect(() => services.editProfile(pool, body, signal)).toThrow(
    'Current profile owner required',
  );
}
async function mobilePreferencesWrite() {
  const current = (await services.preferences(signal)).data.preferences;
  const body = {
    expectedRevision: current.revision,
    locale: 'fr-CA',
    timezone: 'America/Toronto',
  };
  const previous = delivered.length,
    signed = signatures;
  loseNextResponse = 'PATCH /v1/me/preferences';
  await expect(services.updatePreferences(body, signal)).rejects.toThrow();
  const committed = (await services.preferences(signal)).data.preferences;
  expect(committed.locale).toBe(body.locale);
  expect(committed.timezone).toBe(body.timezone);
  expect(
    (await services.updatePreferences(body, signal)).data.preferences.revision,
  ).toBe(committed.revision);
  expect(signatures).toBe(signed + 1);
  assertSignedDelivery('preferences.update', previous);
  const unsigned = new MobileServices(
    configuration(),
    undefined,
    cookieTransport,
  );
  await expect(unsigned.updatePreferences(body, signal)).rejects.toThrow(
    'External wallet operation signer required',
  );
}
async function mobileDeviceSignatureHeader() {
  const previous = delivered.length;
  const account = privateKeyToAccount(gateway.privateKey);
  const authorize: NonNullable<MobileServices['deviceAuthorization']> = async (
    input,
  ) => {
    const operation = {
      version: 2,
      chainId: '125',
      environment: 'production',
      actor: gateway.identity.wallet,
      action: input.method === 'PUT' ? 'device.register' : 'device.revoke',
      method: input.method,
      path: input.path,
      bodyHash: createHash('sha256').update(input.body).digest('hex'),
      nonce: (randomUUID() + randomUUID()).replace(/-/g, ''),
      issuedAt: Math.floor(Date.now() / 1000),
      expiresAt: Math.floor(Date.now() / 1000) + 300,
    };
    const encoded = JSON.stringify(operation);
    const signature = await account.signMessage({
      message: `KindleLaunch signed operation v2\n${encoded}`,
    });
    return {
      operation: Buffer.from(encoded).toString('base64url'),
      signature,
      idempotencyKey: randomUUID(),
    };
  };
  services.deviceAuthorization = authorize;
  const registered = await services.device(
    user.installation,
    user.deviceBody,
    signal,
  );
  expect(registered.data.deviceId).toBe(user.installation);
  // Close the mobile session, reopen the installation record, and revoke with the recovered ID.
  const path = join(directory, 'installation');
  await writeFile(path, user.installation, { mode: 0o600 });
  services.clearIdentity();
  const reopened = createServices();
  await reopened.currentSession(signal);
  reopened.deviceAuthorization = authorize;
  const recovered = await readFile(path, 'utf8');
  expect(recovered).toBe(registered.data.deviceId);
  await reopened.revokeDevice(recovered, signal);
  expect(delivered.slice(previous)).toHaveLength(2);
  for (const delivery of delivered.slice(previous)) {
    expect(delivery.signature).toMatch(/^0x[0-9a-f]{130}$/i);
    expect(delivery.operation).not.toBe('');
    expect(delivery.legacySignature).toBe('');
  }
  const registrations = {
    data: { devices: [decodeContract('PushDevice', device)] },
  };
  expect(registrations).toMatchObject({
    data: {
      devices: [
        {
          deviceId: recovered,
          state: 'revoked',
          platform: 'ios',
          provider: 'apns',
        },
      ],
    },
  });
}
class DiskPublicSnapshots implements PublicSnapshotStorage {
  private closed = false;
  constructor(readonly root: string) {}
  private path(key: string) {
    if (this.closed) throw new Error('storage closed');
    return join(this.root, `${encodeURIComponent(key)}.snapshot.json`);
  }
  async read(key: string) {
    try {
      return await readFile(this.path(key), 'utf8');
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
    const path = this.path(key);
    await writeFile(`${path}.next`, value);
    await rename(`${path}.next`, path);
  }
  async clear() {
    this.path('live');
    for (const file of await readdir(this.root))
      if (file.endsWith('.snapshot.json')) await rm(join(this.root, file));
  }
  close() {
    this.closed = true;
  }
}
async function TestRepair_30_MobileLive() {
  const network = new MobileServices(configuration(live.origin));
  const produced = await network.streams(signal);
  expect(produced.data.items[0].viewerCount).toBeNull();
  expect(produced.data.items[0].viewerQuality).toBe('unavailable');
  expect(produced.data.items[1].viewerCount).toBe(0);
  expect(produced.data.items[1].viewerQuality).toBe('verified');
  const disk = new DiskPublicSnapshots(directory),
    cache = new SnapshotCache(1024 * 1024, 1024, disk);
  const now = Date.now();
  cache.put('live', produced, null, now + 60_000, now);
  await cache.flush();
  const stored = JSON.parse((await disk.read('live'))!);
  expect(stored.formatVersion).toBe(2);
  expect(stored.contractVersion).toBe(2);
  disk.close();
  await expect(disk.read('live')).rejects.toThrow('storage closed');
  const reopenedDisk = new DiskPublicSnapshots(directory);
  const recovered = await new SnapshotCache(
    1024 * 1024,
    1024,
    reopenedDisk,
  ).read<typeof produced>('live', null);
  expect(recovered?.value).toEqual(produced);
  // Untagged v1 records and unknown stored versions are explicitly refused;
  // missing viewer evidence is never migrated to an invented zero count.
  delete stored.formatVersion;
  delete stored.contractVersion;
  await reopenedDisk.write('live', JSON.stringify(stored));
  await expect(
    new SnapshotCache(1024 * 1024, 1024, reopenedDisk).read('live', null),
  ).rejects.toThrow('Unsupported public snapshot version');
  stored.formatVersion = 2;
  stored.contractVersion = 3;
  await reopenedDisk.write('live', JSON.stringify(stored));
  await expect(
    new SnapshotCache(1024 * 1024, 1024, reopenedDisk).read('live', null),
  ).rejects.toThrow('Unsupported public snapshot version');
  const sdk = new SdkClient(configuration(live.origin));
  await expect(
    sdk.requestV2('/v2/streams', 'StreamListResponse', {
      headers: { 'X-Kindle-Signature': '0x00' },
    }),
  ).rejects.toThrow('Unsupported operation header');
}

async function mobileOperationBoundary() {
  const original = delivered.find(
    (delivery) =>
      delivery.method === 'PATCH' && delivery.path.startsWith('/v1/profiles/'),
  )!;
  const headers = new Headers({
    Origin: 'https://kindle.test',
    Cookie: `__Host-kindle_access=${gateway.cookie}; __Host-kindle_refresh=${gateway.refreshCookie}; __Host-kindle_csrf=${gateway.csrf}`,
    'X-CSRF-Token': gateway.csrf,
    'Content-Type': 'application/json',
    'Idempotency-Key': original.id,
    'X-Kindle-Operation': original.operation,
    'X-Kindle-Operation-Signature': original.signature,
  });
  const before = JSON.stringify({ profile, preferences, watchEntries, device });
  const submit = async (
    path: string,
    method: string,
    body: string,
    changed = headers,
  ) => {
    const response = await fetch(`${gateway.origin}${path}`, {
      method,
      headers: changed,
      ...(body ? { body } : {}),
    });
    expect(response.status).toBe(403);
    expect(decodeV2Contract('Error', await response.json()).retryable).toBe(
      false,
    );
  };
  await submit(
    original.path,
    original.method,
    original.body.replace('Mobile owner', 'Changed owner'),
  );
  await submit('/v1/me/preferences', original.method, original.body);
  await submit(original.path, 'PUT', original.body);
  const signedByAnotherWallet = new Headers(headers);
  signedByAnotherWallet.set(
    'X-Kindle-Operation-Signature',
    await privateKeyToAccount(`0x${'22'.repeat(32)}`).signMessage({
      message: `KindleLaunch signed operation v2\n${Buffer.from(original.operation, 'base64url').toString('utf8')}`,
    }),
  );
  await submit(
    original.path,
    original.method,
    original.body,
    signedByAnotherWallet,
  );
  const oldHeader = new Headers(headers);
  oldHeader.delete('X-Kindle-Operation-Signature');
  oldHeader.set('X-Kindle-Signature', original.signature);
  await submit(original.path, original.method, original.body, oldHeader);
  const changedId = new Headers(headers);
  changedId.set('Idempotency-Key', randomUUID());
  await submit(original.path, original.method, original.body, changedId);
  const missingCsrf = new Headers(headers);
  missingCsrf.delete('X-CSRF-Token');
  await submit(original.path, original.method, original.body, missingCsrf);
  const missingSession = new Headers(headers);
  missingSession.delete('Cookie');
  await submit(original.path, original.method, original.body, missingSession);
  const identity = gateway.identity;
  try {
    gateway.identity = {
      ...identity,
      sessionId: randomUUID(),
      authEpoch: identity.authEpoch + 1,
    };
    await submit(original.path, original.method, original.body);
  } finally {
    gateway.identity = identity;
  }
  expect(JSON.stringify({ profile, preferences, watchEntries, device })).toBe(
    before,
  );
  expect(ledger.size).toBe(6);
}
async function mobileOperationIdentityChange() {
  const client = createServices();
  const previous = delivered.length;
  client.setOperationWallet({
    supportedChain: 125,
    connectedAccount: account.address,
    async signMessage(message) {
      client.clearIdentity();
      return account.signMessage({ message });
    },
  });
  await expect(client.watch(pool, true, signal)).rejects.toThrow(
    'Operation account, session or local unlock changed',
  );
  expect(delivered).toHaveLength(previous);
  const locked = createServices();
  locked.setOperationWallet(
    {
      supportedChain: 125,
      connectedAccount: account.address,
      signMessage: (message) => account.signMessage({ message }),
    },
    () => false,
  );
  await expect(locked.watch(pool, true, signal)).rejects.toThrow(
    'Operation account, session or local unlock changed',
  );
  expect(delivered).toHaveLength(previous);
}

test(mobileWatchWrite, mobileWatchWrite);
test(mobileUnwatchWrite, mobileUnwatchWrite);
test(mobileProfileWrite, mobileProfileWrite);
test(mobilePreferencesWrite, mobilePreferencesWrite);
test(mobileDeviceSignatureHeader, mobileDeviceSignatureHeader);
test(TestRepair_30_MobileLive, TestRepair_30_MobileLive);
test(mobileOperationBoundary, mobileOperationBoundary);
test(mobileOperationIdentityChange, mobileOperationIdentityChange);
