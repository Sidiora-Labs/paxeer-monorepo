import { expect, test } from 'vitest';
import {
  createServer,
  type IncomingMessage,
  type ServerResponse,
} from 'node:http';
import {
  createHash,
  generateKeyPairSync,
  randomBytes,
  randomUUID,
  sign,
  verify,
} from 'node:crypto';
import { privateKeyToAccount } from 'viem/accounts';
import { verifyMessage, type Hex } from 'viem';
import {
  MobileServices,
  type CurrentIdentity,
  type PublicConfiguration,
} from '../../src/runtime/service-client.ts';
import {
  SessionController,
  type CredentialStore,
} from '../../src/runtime/session.ts';
import { SnapshotCache } from '../../src/runtime/offline-cache.ts';
import { createApplicationPush } from '../../src/runtime/push-bootstrap.ts';
import type {
  NativePushProvider,
  PushController,
} from '../../src/runtime/push.ts';
import {
  decodeContract,
  decodeV2Contract,
} from '../../../../../sdk/launchpad/packages/sdk/src/index.ts';

const owner = privateKeyToAccount(`0x${'11'.repeat(32)}`);
const installation = '88888888-8888-4888-8888-888888888888';
const otherWallet = '0x2222222222222222222222222222222222222222';
const origin = 'https://kindle.test';
type StorageKey = Parameters<CredentialStore['get']>[0];
class InstallationStore implements CredentialStore {
  private closed = false;
  constructor(readonly values = new Map<StorageKey, string>()) {}
  private check() {
    if (this.closed) throw new Error('Installation store closed');
  }
  async get(key: StorageKey) {
    this.check();
    return this.values.get(key) ?? null;
  }
  async set(key: StorageKey, value: string) {
    this.check();
    this.values.set(key, value);
  }
  async remove(key: StorageKey) {
    this.check();
    this.values.delete(key);
  }
  async clear() {
    this.check();
    this.values.clear();
  }
  close() {
    this.closed = true;
  }
  reopen() {
    return new InstallationStore(this.values);
  }
}
function envelope(data: unknown) {
  return {
    requestId: randomUUID(),
    data,
    watermark: null,
    revision: '1',
    quality: 'complete',
    nextCursor: null,
    generatedAt: new Date().toISOString(),
  };
}
async function bodyOf(request: IncomingMessage) {
  const parts: Buffer[] = [];
  for await (const part of request) parts.push(Buffer.from(part));
  return Buffer.concat(parts).toString('utf8');
}
function reply(
  response: ServerResponse,
  value: unknown,
  status = 200,
  headers: Record<string, string> = {},
) {
  response.writeHead(status, {
    'Content-Type': 'application/json',
    ...headers,
  });
  response.end(status === 204 ? undefined : JSON.stringify(value));
}
class PushInstallationFixture {
  readonly keys = generateKeyPairSync('ed25519');
  readonly csrf = 'c'.repeat(48);
  readonly listeners = new Set<() => void>();
  readonly errors: unknown[] = [];
  readonly deliveries: {
    method: string;
    token?: string;
    operation: Record<string, unknown>;
  }[] = [];
  readonly signedMessages: string[] = [];
  readonly events: string[] = [];
  readonly nonces = new Set<string>();
  identity: CurrentIdentity = {
    accountId: '11111111-1111-4111-8111-111111111111',
    sessionId: '33333333-3333-4333-8333-333333333333',
    wallet: owner.address.toLowerCase(),
    authEpoch: 1,
    proofAt: new Date().toISOString(),
    accessExpiresAt: new Date(Date.now() + 3600_000).toISOString(),
  };
  token = 'provider-token-initial';
  permission: 'granted' | 'denied' = 'granted';
  permissionRequests = 0;
  proofRequests = 0;
  sessionRevoked = false;
  deviceState: 'active' | 'revoked' | undefined;
  deviceToken: string | undefined;
  proofMode:
    | 'valid'
    | 'account'
    | 'token'
    | 'expired'
    | 'platform'
    | 'malformed'
    | 'unavailable'
    | 'signature' = 'valid';
  refuseRevocation = false;
  platform: 'ios' | 'android' = 'ios';
  portOrigin = '';
  readonly server = createServer((request, response) => {
    void this.handle(request, response).catch((error) => {
      this.errors.push(error);
      reply(response, { error: String(error) }, 500);
    });
  });
  get provider() {
    return this.platform === 'ios' ? ('apns' as const) : ('fcm' as const);
  }
  get session() {
    return decodeV2Contract('Session', {
      sessionId: this.identity.sessionId,
      wallet: this.identity.wallet,
      deviceLabel: 'Installed fixture',
      scopes: ['user'],
      accessExpiresAt: this.identity.accessExpiresAt,
      refreshExpiresAt: new Date(Date.now() + 86400_000).toISOString(),
      revokedAt: null,
    });
  }
  async open() {
    await new Promise<void>((resolve) =>
      this.server.listen(0, '127.0.0.1', resolve),
    );
    const address = this.server.address();
    if (!address || typeof address === 'string')
      throw new Error('Fixture must listen');
    this.portOrigin = `http://127.0.0.1:${address.port}`;
    return this;
  }
  async close() {
    await new Promise<void>((resolve, reject) => {
      this.server.close((error) => (error ? reject(error) : resolve()));
      this.server.closeAllConnections();
    });
    expect(this.errors).toEqual([]);
  }
  rotate(token: string) {
    this.token = token;
    for (const listener of this.listeners) listener();
  }
  proof(input: {
    accountId: string;
    sessionId: string;
    installationId: string;
    token: string;
  }) {
    const wire = Buffer.alloc(107);
    wire[0] = 2;
    Buffer.from(
      (this.proofMode === 'account' ? randomUUID() : input.accountId).replace(
        /-/g,
        '',
      ),
      'hex',
    ).copy(wire, 1);
    Buffer.from(input.sessionId.replace(/-/g, ''), 'hex').copy(wire, 17);
    Buffer.from(input.installationId.replace(/-/g, ''), 'hex').copy(wire, 33);
    createHash('sha256')
      .update(this.proofMode === 'token' ? 'different-token' : input.token)
      .digest()
      .copy(wire, 49);
    randomBytes(16).copy(wire, 81);
    wire.writeBigUInt64BE(
      BigInt(
        Math.floor(Date.now() / 1000) +
          (this.proofMode === 'expired' ? -1 : 240),
      ),
      97,
    );
    wire[105] =
      this.proofMode === 'platform' ? 1 : this.platform === 'ios' ? 2 : 3;
    wire[106] = this.platform === 'ios' ? 2 : 3;
    const signature = sign(null, wire, this.keys.privateKey);
    if (this.proofMode === 'signature') signature[0] ^= 1;
    return Buffer.concat([wire, signature]).toString('base64url');
  }
  async authorize(request: IncomingMessage, text: string) {
    const encoded = request.headers['x-kindle-operation'];
    const signature = request.headers['x-kindle-operation-signature'];
    expect(typeof encoded).toBe('string');
    expect(typeof signature).toBe('string');
    const operation = JSON.parse(
      Buffer.from(String(encoded), 'base64url').toString('utf8'),
    ) as Record<string, unknown>;
    expect(operation).toMatchObject({
      version: 2,
      chainId: '125',
      environment: 'production',
      actor: this.identity.wallet,
      action: request.method === 'PUT' ? 'device.register' : 'device.revoke',
      method: request.method,
      path: request.url,
      bodyHash: createHash('sha256').update(text).digest('hex'),
    });
    expect(operation.nonce).toMatch(/^[0-9a-f]{64}$/);
    expect(this.nonces.has(String(operation.nonce))).toBe(false);
    expect(Number(operation.expiresAt)).toBeGreaterThan(Date.now() / 1000);
    expect(
      Number(operation.expiresAt) - Number(operation.issuedAt),
    ).toBeLessThanOrEqual(300);
    expect(request.headers['idempotency-key']).toMatch(/^[0-9a-f-]{36}$/);
    expect(request.headers['x-kindle-signature']).toBeUndefined();
    const statement = `KindleLaunch signed operation v2\n${JSON.stringify(operation)}`;
    expect(
      await verifyMessage({
        address: owner.address,
        message: statement,
        signature: String(signature) as Hex,
      }),
    ).toBe(true);
    this.nonces.add(String(operation.nonce));
    return operation;
  }
  async handle(request: IncomingMessage, response: ServerResponse) {
    const text = await bodyOf(request);
    if (request.url === '/native/permission') {
      if (request.method === 'POST') this.permissionRequests++;
      return reply(response, { permission: this.permission });
    }
    if (request.url === '/native/registration')
      return reply(response, {
        installationId: installation,
        token: this.token,
        platform: this.platform,
        provider: this.provider,
      });
    if (request.url === '/wallet/personal_sign') {
      const input = JSON.parse(text) as { message: string };
      expect(
        input.message.startsWith('KindleLaunch signed operation v2\n'),
      ).toBe(true);
      this.signedMessages.push(input.message);
      return reply(response, {
        signature: await owner.signMessage({ message: input.message }),
      });
    }
    expect(request.headers.cookie).toBe('fixture-session=protected');
    expect(request.headers.origin).toBe(origin);
    if (this.sessionRevoked) return reply(response, { error: 'revoked' }, 401);
    if (request.url === '/v2/auth/session')
      return reply(response, { data: this.identity }, 200, {
        'X-CSRF-Token': this.csrf,
      });
    if (request.url === '/v2/auth/sessions?limit=50')
      return reply(
        response,
        decodeV2Contract(
          'SessionsResponse',
          envelope({ items: [this.session] }),
        ),
      );
    expect(request.headers['x-csrf-token']).toBe(this.csrf);
    if (
      request.url === `/v2/auth/sessions/${this.identity.sessionId}` &&
      request.method === 'DELETE'
    ) {
      this.events.push('session.logout');
      this.sessionRevoked = true;
      return reply(response, null, 204);
    }
    if (
      request.url === `/v1/me/devices/${installation}/proof` &&
      request.method === 'POST'
    ) {
      this.proofRequests++;
      const input = JSON.parse(text);
      expect(input).toEqual({
        installationId: installation,
        accountId: this.identity.accountId,
        sessionId: this.identity.sessionId,
        token: this.token,
        platform: this.platform,
        provider: this.provider,
      });
      if (this.proofMode === 'unavailable')
        return reply(response, { error: 'issuer unavailable' }, 503);
      return reply(response, {
        data: {
          proof:
            this.proofMode === 'malformed' ? 'bad-proof' : this.proof(input),
        },
      });
    }
    expect(request.url).toBe(`/v1/me/devices/${installation}`);
    const operation = await this.authorize(request, text);
    if (request.method === 'DELETE') {
      expect(text).toBe('');
      if (this.refuseRevocation)
        return reply(response, { error: 'reconnect required' }, 503);
      this.events.push('device.revoke');
      this.deliveries.push({ method: 'DELETE', operation });
      this.deviceState = 'revoked';
      return reply(response, null, 204);
    }
    expect(request.method).toBe('PUT');
    const input = decodeContract('DeviceRegisterBody', JSON.parse(text));
    const proof = Buffer.from(input.deviceProof, 'base64url');
    if (
      !verify(
        null,
        proof.subarray(0, 107),
        this.keys.publicKey,
        proof.subarray(107),
      )
    )
      return reply(response, { error: 'invalid provider signature' }, 403);
    expect(proof.length).toBe(171);
    expect(proof[0]).toBe(2);
    expect(proof.subarray(1, 17).toString('hex')).toBe(
      this.identity.accountId.replace(/-/g, ''),
    );
    expect(proof.subarray(17, 33).toString('hex')).toBe(
      this.identity.sessionId.replace(/-/g, ''),
    );
    expect(proof.subarray(33, 49).toString('hex')).toBe(
      installation.replace(/-/g, ''),
    );
    expect(proof.subarray(49, 81)).toEqual(
      createHash('sha256').update(input.token).digest(),
    );
    expect(proof.subarray(81, 97).some((byte) => byte !== 0)).toBe(true);
    expect(proof.readBigUInt64BE(97)).toBeGreaterThan(
      BigInt(Math.floor(Date.now() / 1000)),
    );
    expect(proof[105]).toBe(this.platform === 'ios' ? 2 : 3);
    expect(proof[106]).toBe(this.platform === 'ios' ? 2 : 3);
    expect(input.platform).toBe(this.platform);
    expect(input.provider).toBe(this.provider);
    expect(input.token).toBe(this.token);
    this.events.push('device.register');
    this.deliveries.push({ method: 'PUT', token: input.token, operation });
    this.deviceState = 'active';
    this.deviceToken = input.token;
    reply(
      response,
      decodeContract(
        'DeviceRegisterResponse',
        envelope({ deviceId: installation, revision: '1', state: 'active' }),
      ),
    );
  }
  configuration(): PublicConfiguration {
    return {
      apiOrigin: origin,
      identityOrigin: origin,
      mediaOrigin: origin,
      canonicalOrigin: origin,
      identityURI: origin,
      streamOrigin: 'wss://kindle.test',
      approvedStreamOrigins: ['wss://kindle.test'],
      production: true,
      manifestDigest: 'a'.repeat(64),
      manifestURL: `${origin}/manifest.json`,
      optionalFeatures: { chat: false, live: false },
      approvedPlaybackOrigins: [],
    };
  }
  services() {
    return new MobileServices(
      this.configuration(),
      undefined,
      async (url, init = {}) => {
        const target = new URL(url);
        expect(target.origin).toBe(origin);
        const headers = new Headers(init.headers);
        headers.set('Cookie', 'fixture-session=protected');
        return fetch(`${this.portOrigin}${target.pathname}${target.search}`, {
          ...init,
          headers,
        });
      },
      async () => randomUUID(),
    );
  }
}
class NativeInstallationPeer implements NativePushProvider {
  constructor(readonly fixture: PushInstallationFixture) {}
  async permission() {
    return (
      (await (
        await fetch(`${this.fixture.portOrigin}/native/permission`)
      ).json()) as { permission: 'granted' | 'denied' }
    ).permission;
  }
  async requestPermission() {
    return (
      (await (
        await fetch(`${this.fixture.portOrigin}/native/permission`, {
          method: 'POST',
        })
      ).json()) as { permission: 'granted' | 'denied' }
    ).permission;
  }
  async registration() {
    return (await (
      await fetch(`${this.fixture.portOrigin}/native/registration`)
    ).json()) as Awaited<ReturnType<NativePushProvider['registration']>>;
  }
  listenTokenChanged(listener: () => void) {
    this.fixture.listeners.add(listener);
    return () => {
      this.fixture.listeners.delete(listener);
    };
  }
}
class ExternalWalletPeer {
  readonly supportedChain = 125 as const;
  connectedAccount: string | undefined = owner.address;
  afterSign: (() => void) | undefined;
  constructor(readonly fixture: PushInstallationFixture) {}
  async signMessage(message: string, signal: AbortSignal) {
    const response = await fetch(
      `${this.fixture.portOrigin}/wallet/personal_sign`,
      {
        method: 'POST',
        signal,
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ message }),
      },
    );
    const result = (await response.json()) as { signature: string };
    this.afterSign?.();
    return result.signature;
  }
}
async function openApplication(
  fixture: PushInstallationFixture,
  storage = new InstallationStore(),
) {
  const services = fixture.services();
  const session = new SessionController(storage, new SnapshotCache());
  await session.restore(services);
  const wallet = new ExternalWalletPeer(fixture);
  let unlocked = true;
  let changes = 0;
  session.onInvalidated = () => {
    unlocked = false;
  };
  const push = createApplicationPush({
    session,
    services,
    wallet,
    native: new NativeInstallationPeer(fixture),
    unlocked: () => unlocked,
    changed: () => {
      changes++;
    },
  });
  return {
    services,
    session,
    wallet,
    push,
    storage,
    lock: () => {
      unlocked = false;
    },
    changes: () => changes,
  };
}
function closeApplication(app: Awaited<ReturnType<typeof openApplication>>) {
  app.push.dispose();
  app.session.cancel();
  app.services.clearIdentity();
  app.storage.close();
}

async function mobileDefaultPushAuthority() {
  for (const platform of ['ios', 'android'] as const) {
    const fixture = await new PushInstallationFixture().open();
    fixture.platform = platform;
    let app: Awaited<ReturnType<typeof openApplication>> | undefined;
    try {
      app = await openApplication(fixture);
      expect(fixture.permissionRequests).toBe(0);
      expect(fixture.proofRequests).toBe(0);
      expect(app.services.deviceAuthorization).toBeTypeOf('function');
      expect(await app.push.enableByUserAction()).toBe('granted');
      expect(fixture.deviceState).toBe('active');
      expect(fixture.deviceToken).toBe('provider-token-initial');
      expect(await app.storage.get('installation')).toBe(installation);
      expect(await app.storage.get('pushToken')).toBe(fixture.deviceToken);
      fixture.rotate('provider-token-rotated');
      expect(app.push.tokenChanged).toBe(true);
      expect(app.changes()).toBe(1);
      expect(await app.push.enableByUserAction()).toBe('granted');
      expect(app.push.tokenChanged).toBe(false);
      expect(fixture.deviceToken).toBe('provider-token-rotated');
      expect(await app.storage.get('pushToken')).toBe(fixture.deviceToken);
      const storage = app.storage;
      closeApplication(app);
      expect(fixture.listeners.size).toBe(0);
      await expect(storage.get('installation')).rejects.toThrow('closed');
      app = await openApplication(fixture, storage.reopen());
      expect(app.session.account).toBe(fixture.identity.wallet);
      expect(await app.storage.get('installation')).toBe(installation);
      expect(await app.storage.get('pushToken')).toBe('provider-token-rotated');
      expect(fixture.permissionRequests).toBe(2);
      // Sign-out after restart must have authority before push is enabled again.
      app.wallet.afterSign = () => {
        expect(app?.session.token).toBeUndefined();
      };
      await app.session.logout(app.services);
      expect(fixture.deviceState).toBe('revoked');
      expect(fixture.sessionRevoked).toBe(true);
      expect(fixture.events).toEqual([
        'device.register',
        'device.register',
        'device.revoke',
        'session.logout',
      ]);
      expect(fixture.deliveries.map((value) => value.operation.action)).toEqual(
        ['device.register', 'device.register', 'device.revoke'],
      );
      expect(fixture.deliveries.map((value) => value.token)).toEqual([
        'provider-token-initial',
        'provider-token-rotated',
        undefined,
      ]);
      expect(fixture.signedMessages).toHaveLength(3);
      expect(fixture.proofRequests).toBe(2);
      expect(app.session.current).toBeUndefined();
      expect(app.storage.values.size).toBe(0);
      const deniedWrites = fixture.deliveries.length;
      await expect(app.push.enableByUserAction()).rejects.toThrow('Sign in');
      expect(fixture.deliveries).toHaveLength(deniedWrites);
    } finally {
      if (app) closeApplication(app);
      await fixture.close();
    }
  }
}
async function mobilePushProofFailClosed() {
  const fixture = await new PushInstallationFixture().open();
  const app = await openApplication(fixture);
  try {
    fixture.permission = 'denied';
    expect(await app.push.enableByUserAction()).toBe('denied');
    expect(fixture.proofRequests).toBe(0);
    fixture.permission = 'granted';
    for (const mode of [
      'account',
      'token',
      'expired',
      'platform',
      'malformed',
      'unavailable',
      'signature',
    ] as const) {
      fixture.proofMode = mode;
      await expect(app.push.enableByUserAction()).rejects.toThrow();
      expect(fixture.deviceState).toBeUndefined();
      expect(await app.storage.get('installation')).toBeNull();
      expect(await app.storage.get('pushToken')).toBeNull();
    }
    fixture.proofMode = 'valid';
    app.lock();
    const signatures = fixture.signedMessages.length;
    await expect(app.push.enableByUserAction()).rejects.toThrow('local unlock');
    expect(fixture.signedMessages).toHaveLength(signatures);
    expect(fixture.deliveries).toEqual([]);
  } finally {
    closeApplication(app);
    await fixture.close();
  }
}
async function mobilePushWalletAuthorityChanges() {
  const fixture = await new PushInstallationFixture().open();
  const app = await openApplication(fixture);
  let missingWalletPush: PushController | undefined;
  try {
    missingWalletPush = createApplicationPush({
      session: app.session,
      services: app.services,
      wallet: undefined,
      native: new NativeInstallationPeer(fixture),
      unlocked: () => true,
      changed: () => {},
    });
    await expect(missingWalletPush.enableByUserAction()).rejects.toThrow(
      'signer unavailable',
    );
    expect(fixture.deliveries).toEqual([]);
    app.wallet.afterSign = () => {
      app.wallet.connectedAccount = otherWallet;
    };
    await expect(app.push.enableByUserAction()).rejects.toThrow('wallet');
    expect(fixture.deliveries).toEqual([]);
    app.wallet.connectedAccount = owner.address;
    app.wallet.afterSign = () => {
      fixture.identity = {
        ...fixture.identity,
        authEpoch: fixture.identity.authEpoch + 1,
      };
    };
    await expect(app.push.enableByUserAction()).rejects.toThrow('session');
    expect(fixture.deliveries).toEqual([]);
    expect(await app.storage.get('installation')).toBeNull();
  } finally {
    missingWalletPush?.dispose();
    closeApplication(app);
    await fixture.close();
  }
}
async function mobilePushRevocationFailurePurgesLocally() {
  const fixture = await new PushInstallationFixture().open();
  const app = await openApplication(fixture);
  try {
    await app.push.enableByUserAction();
    fixture.refuseRevocation = true;
    await expect(app.session.logout(app.services)).rejects.toThrow(
      'server device revocation requires reconnect',
    );
    expect(app.session.current).toBeUndefined();
    expect(app.storage.values.size).toBe(0);
    expect(fixture.sessionRevoked).toBe(true);
    expect(fixture.deviceState).toBe('active');
    expect(fixture.events).toEqual(['device.register', 'session.logout']);
  } finally {
    closeApplication(app);
    await fixture.close();
  }
}

test(mobileDefaultPushAuthority.name, mobileDefaultPushAuthority);
test(mobilePushProofFailClosed.name, mobilePushProofFailClosed);
test(mobilePushWalletAuthorityChanges.name, mobilePushWalletAuthorityChanges);
test(
  mobilePushRevocationFailurePurgesLocally.name,
  mobilePushRevocationFailurePurgesLocally,
);
