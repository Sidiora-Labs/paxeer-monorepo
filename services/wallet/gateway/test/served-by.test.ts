import { spawn, type ChildProcess } from 'node:child_process';
import { generateKeyPairSync, randomBytes, randomUUID, sign } from 'node:crypto';
import { mkdirSync, mkdtempSync, rmSync, writeFileSync } from 'node:fs';
import { createServer } from 'node:net';
import { tmpdir } from 'node:os';
import { join } from 'node:path';
import { fileURLToPath } from 'node:url';
import type { AddressInfo } from 'node:net';
import { afterAll, beforeAll, describe, expect, it, test } from 'vitest';
import type { FastifyInstance } from 'fastify';
import Fastify from 'fastify';
import { generatePrivateKey, privateKeyToAccount } from 'viem/accounts';
import { startPostgres, type EphemeralPostgres } from './support/postgres.js';
import { startIdentityProvider, type IdentityProvider } from './support/identity.js';
import { buildAttestor, makePki, type Pki } from './e2e/attestors.js';

let idp: IdentityProvider;
let pg: EphemeralPostgres;
let app: FastifyInstance;
let base: string;
let index: typeof import('../src/index.js');
let poolModule: typeof import('../src/db/pool.js');
let sponsorApp: FastifyInstance;

beforeAll(async () => {
  idp = await startIdentityProvider();
  process.env.SUPABASE_URL = idp.url;
  process.env.AGENT_JWT_SECRET = randomBytes(32).toString('hex');
  process.env.LOG_LEVEL = 'error';
  process.env.RPC_URLS = `${idp.url}/rpc`;
  process.env.SPONSOR_PRIVATE_KEY_FILE = '';
  delete process.env.WALLET_GAS_STATION_URL;
  pg = await startPostgres();
  index = await import('../src/index.js');
  poolModule = await import('../src/db/pool.js');
  app = await index.buildApp();
  await app.listen({ port: 0, host: '127.0.0.1' });
  const address = app.server.address() as AddressInfo;
  base = `http://127.0.0.1:${address.port}`;
  sponsorApp = Fastify();
  const { walletSponsorRoutes } = await import('../src/routes/wallet.js');
  const { RpcPool } = await import('../src/rpc/pool.js');
  await sponsorApp.register(walletSponsorRoutes, {
    stationUrl: idp.url,
    rpc: new RpcPool({
      urls: [`${idp.url}/rpc`],
      chainId: 125,
      lagThresholdBlocks: 0,
      timeoutMs: 1_000,
      healthIntervalMs: 1_000,
    }),
  });
  await sponsorApp.ready();
}, 120_000);

afterAll(async () => {
  await stopGatewayEntry();
  await app?.close();
  await sponsorApp?.close();
  await poolModule?.closePool();
  await pg?.stop();
  await idp?.stop();
}, 120_000);

describe('served-by header', () => {
  it('names the gateway as paxeer-wallet-gateway', () => {
    expect(index.SERVED_BY_HEADER).toBe('x-served-by');
    expect(index.SERVED_BY).toBe('paxeer-wallet-gateway');
  });

  it('is set on a liveness answer over a real connection', async () => {
    const res = await fetch(`${base}/healthz`);
    expect(res.status).toBe(200);
    expect(res.headers.get('x-served-by')).toBe('paxeer-wallet-gateway');
    expect(await res.json()).toMatchObject({ ok: true });
  });

  it('is set on the readiness answer whatever its status', async () => {
    const res = await fetch(`${base}/readyz`);
    expect([200, 503]).toContain(res.status);
    expect(res.headers.get('x-served-by')).toBe('paxeer-wallet-gateway');
  });

  it('is set on an authenticated wallet answer', async () => {
    const token = await idp.mintUserToken(randomUUID());
    const res = await fetch(`${base}/v1/wallet/provision`, {
      method: 'POST',
      headers: { authorization: `Bearer ${token}` },
    });
    expect(res.status).toBe(200);
    const provisioned = (await res.json()) as { wallet: { id: string; address: string } };
    expect(res.headers.get('x-served-by')).toBe('paxeer-wallet-gateway');
    const me = await fetch(`${base}/v1/wallet/me`, {
      headers: { authorization: `Bearer ${token}` },
    });
    expect(me.status).toBe(200);
    expect(await me.json()).toMatchObject({
      wallet: { id: provisioned.wallet.id, address: provisioned.wallet.address },
    });
    expect(me.headers.get('x-served-by')).toBe('paxeer-wallet-gateway');
  });

  it('is set on a refused request without a token', async () => {
    const res = await fetch(`${base}/v1/wallet/me`);
    expect(res.status).toBe(401);
    expect(res.headers.get('x-served-by')).toBe('paxeer-wallet-gateway');
  });

  it('is set on an unknown route', async () => {
    const res = await fetch(`${base}/no-such-route`);
    expect(res.status).toBe(404);
    expect(res.headers.get('x-served-by')).toBe('paxeer-wallet-gateway');
  });

  it('is set on a malformed request body', async () => {
    const token = await idp.mintUserToken(randomUUID());
    const res = await fetch(`${base}/v1/wallet/send`, {
      method: 'POST',
      headers: { authorization: `Bearer ${token}`, 'content-type': 'application/json' },
      body: '{not json',
    });
    expect(res.status).toBe(400);
    expect(res.headers.get('x-served-by')).toBe('paxeer-wallet-gateway');
  });

  it('is set on a CORS preflight answer', async () => {
    const res = await fetch(`${base}/v1/wallet/me`, {
      method: 'OPTIONS',
      headers: {
        origin: 'http://localhost:3000',
        'access-control-request-method': 'GET',
        'access-control-request-headers': 'authorization',
      },
    });
    expect(res.status).toBeLessThan(300);
    expect(res.headers.get('access-control-allow-origin')).toBe('http://localhost:3000');
    expect(res.headers.get('x-served-by')).toBe('paxeer-wallet-gateway');
  });
});

describe('sponsorship identity boundary', () => {
  async function provisionCaller() {
    const userId = randomUUID();
    const token = await idp.mintUserToken(userId);
    const response = await app.inject({
      method: 'POST',
      url: '/v1/wallet/provision',
      headers: { authorization: `Bearer ${token}` },
    });
    expect(response.statusCode).toBe(200);
    const wallet = response.json().wallet as { id: string; address: string };
    return { userId, token, wallet };
  }

  async function statusRequest(account: string) {
    const sponsor = privateKeyToAccount(generatePrivateKey());
    return {
      account,
      sponsor: sponsor.address,
      quoteNonce: '0',
      relayerSignature: await sponsor.signMessage({ message: account }),
    };
  }

  it('starts with the retired sponsor input discarded', async () => {
    const { env } = await import('../src/env.js');
    expect(Object.hasOwn(env, 'SPONSOR_PRIVATE_KEY_FILE')).toBe(false);
    expect(Object.hasOwn(env, 'WALLET_GAS_STATION_URL')).toBe(false);
    const { wallet, token } = await provisionCaller();
    const response = await app.inject({
      method: 'POST',
      url: '/v1/wallet/sponsored/status',
      headers: { authorization: `Bearer ${token}` },
      payload: await statusRequest(wallet.address),
    });
    expect(response.statusCode).toBe(503);
    expect(response.json()).toEqual({
      error: 'gas_station_unconfigured',
      status: 'pending',
    });
    expect(response.headers['x-served-by']).toBe('paxeer-wallet-gateway');
  });

  it('keeps provisioning idempotent when RPC and sponsorship are unavailable', async () => {
    const { userId, wallet, token } = await provisionCaller();
    const response = await app.inject({
      method: 'POST',
      url: '/v1/wallet/provision',
      headers: { authorization: `Bearer ${token}` },
    });
    expect(response.statusCode).toBe(200);
    expect(response.json().wallet).toMatchObject(wallet);
    const { rows } = await poolModule
      .getPool()
      .query<{ count: string }>(
        "select count(*)::text as count from wallets where user_id = $1 and kind = 'standard'",
        [userId]
      );
    expect(rows).toEqual([{ count: '1' }]);
  });

  it('refuses a caller requesting another wallet identity', async () => {
    const caller = await provisionCaller();
    const other = await provisionCaller();
    const response = await app.inject({
      method: 'POST',
      url: '/v1/wallet/sponsored/status',
      headers: { authorization: `Bearer ${caller.token}` },
      payload: await statusRequest(other.wallet.address),
    });
    expect(response.statusCode).toBe(403);
    expect(response.json()).toEqual({
      error: 'wallet_owner_mismatch',
      status: 'pending',
    });
  });

  it('refuses sponsorship status when the chain identity cannot be read', async () => {
    const { wallet, token } = await provisionCaller();
    const response = await sponsorApp.inject({
      method: 'POST',
      url: '/v1/wallet/sponsored/status',
      headers: { authorization: `Bearer ${token}` },
      payload: await statusRequest(wallet.address),
    });
    expect(response.statusCode).toBe(503);
    expect(response.json()).toEqual({
      error: 'wallet_identity_unavailable',
      status: 'pending',
    });
  });

  it('requires authentication before processing sponsored submissions', async () => {
    const response = await app.inject({
      method: 'POST',
      url: '/v1/wallet/sponsored/submit',
      payload: {},
    });
    expect(response.statusCode).toBe(401);
    expect(response.json()).toEqual({
      error: 'missing or malformed Authorization header',
    });
  });

  it('refuses an authenticated malformed sponsored submission', async () => {
    const { token } = await provisionCaller();
    const response = await app.inject({
      method: 'POST',
      url: '/v1/wallet/sponsored/submit',
      headers: { authorization: `Bearer ${token}` },
      payload: {},
    });
    expect(response.statusCode).toBe(400);
    expect(response.json().error).toBe('invalid_body');
    expect(response.json().issues.map((issue: { path: string[] }) => issue.path)).toEqual([
      ['chain_id'],
      ['account'],
      ['to'],
      ['data'],
      ['value'],
      ['construction'],
      ['account_signature'],
      ['relayer_signature'],
      ['authorization'],
      ['quote_decimals'],
    ]);
  });
});

let work: string;
let pki: Pki;
let gateway: ChildProcess;
let gatewayBase: string;
let gatewayEnv: NodeJS.ProcessEnv;
const children: Array<{ process: ChildProcess; logs: string[] }> = [];
let gatewayLogs: string[];

function launch(command: string, args: string[], env: NodeJS.ProcessEnv) {
  const process = spawn(command, args, {
    env,
    cwd: fileURLToPath(new URL('..', import.meta.url)),
    stdio: ['ignore', 'pipe', 'pipe'],
  });
  const logs: string[] = [];
  process.stdout!.on('data', (chunk: Buffer) => logs.push(chunk.toString('utf8')));
  process.stderr!.on('data', (chunk: Buffer) => logs.push(chunk.toString('utf8')));
  process.on('error', (error) => logs.push(error.message));
  const child = { process, logs };
  children.push(child);
  return child;
}

beforeAll(async () => {
  work = mkdtempSync(join(tmpdir(), 'gateway-entry-'));
  const ids = Array.from({ length: 5 }, (_, i) => `node-${i + 1}`);
  pki = makePki(ids);
  const producer = generateKeyPairSync('ec', { namedCurve: 'prime256v1' });
  const inventorySigner = generateKeyPairSync('ec', { namedCurve: 'prime256v1' });
  const producerFile = join(work, 'producer.key');
  const producerPublic = join(work, 'producer.pub');
  const inventoryFile = join(work, 'inventory.jwt');
  const inventoryPublic = join(work, 'inventory.pub');
  writeFileSync(producerFile, producer.privateKey.export({ format: 'pem', type: 'pkcs8' }), {
    mode: 0o600,
  });
  writeFileSync(producerPublic, producer.publicKey.export({ format: 'pem', type: 'spki' }));
  writeFileSync(inventoryPublic, inventorySigner.publicKey.export({ format: 'pem', type: 'spki' }));
  const now = Math.floor(Date.now() / 1000);
  const header = Buffer.from(
    JSON.stringify({ alg: 'ES256', typ: 'wallet-custody-inventory+jwt' })
  ).toString('base64url');
  const payload = Buffer.from(
    JSON.stringify({
      version: 1,
      iss: `${idp.url}/auth/v1`,
      aud: 'wallet-custody-inventory',
      tenant: 'gateway-entry',
      sequence: '1',
      iat: now - 1,
      exp: now + 3600,
      protocol: 'wallet',
      threshold: 3,
      members: ids.map((id) => ({ id, spki_sha256: pki.node(id).pin })),
      keys: [],
    })
  ).toString('base64url');
  const input = `${header}.${payload}`;
  const signature = sign('sha256', Buffer.from(input), {
    key: inventorySigner.privateKey,
    dsaEncoding: 'ieee-p1363',
  });
  writeFileSync(inventoryFile, `${input}.${signature.toString('base64url')}`, {
    mode: 0o600,
  });
  const policyFile = join(work, 'policy.json');
  writeFileSync(
    policyFile,
    JSON.stringify({
      version: 1,
      defaults: {
        chain_id: 125,
        kinds: ['evm_tx'],
        rate_per_minute: 60,
        caps: { native: { per_transaction: '1000000', daily: '10000000' } },
      },
    })
  );

  const reservations = await Promise.all(
    Array.from({ length: 12 }, async () => {
      const server = createServer();
      await new Promise<void>((resolve, reject) => {
        server.once('error', reject);
        server.listen(0, '127.0.0.1', resolve);
      });
      return { server, port: (server.address() as AddressInfo).port };
    })
  );
  const ports = reservations.map((reservation) => reservation.port);
  await Promise.all(
    reservations.map(
      ({ server }) =>
        new Promise<void>((resolve, reject) => {
          server.close((error) => (error ? reject(error) : resolve()));
        })
    )
  );
  const rpcUrl = `http://127.0.0.1:${ports[10]}`;
  launch(
    process.env.ANVIL_BIN ?? 'anvil',
    ['--host', '127.0.0.1', '--port', String(ports[10]), '--chain-id', '125', '--silent'],
    process.env
  );
  const bin = buildAttestor(work);
  for (const [i, id] of ids.entries()) {
    const nodeKey = join(work, `${id}.key`);
    const nodeDir = join(work, id);
    mkdirSync(nodeDir);
    writeFileSync(nodeKey, randomBytes(32).toString('hex'), { mode: 0o600 });
    const peers = ids.filter((peer) => peer !== id);
    launch(bin, [], {
      PATH: process.env.PATH,
      HOME: process.env.HOME,
      ATTESTOR_NODE_ID: id,
      ATTESTOR_REGION: `region-${i + 1}`,
      ATTESTOR_LISTEN_ADDR: `127.0.0.1:${ports[i * 2]}`,
      ATTESTOR_PEER_LISTEN_ADDR: `127.0.0.1:${ports[i * 2 + 1]}`,
      ATTESTOR_PEERS: peers
        .map((peer) => `${peer}=127.0.0.1:${ports[ids.indexOf(peer) * 2 + 1]}`)
        .join(','),
      ATTESTOR_PEER_PINS: peers.map((peer) => `${peer}=${pki.node(peer).pin}`).join(','),
      ATTESTOR_NODE_KEY_FILE: nodeKey,
      ATTESTOR_DATA_DIR: nodeDir,
      ATTESTOR_CHAIN_ID: '125',
      ATTESTOR_JWKS_URL: `${idp.url}/auth/v1/.well-known/jwks.json`,
      ATTESTOR_JWT_ISSUER: `${idp.url}/auth/v1`,
      ATTESTOR_JWT_AUDIENCE: 'authenticated',
      ATTESTOR_POLICY_FILE: policyFile,
      ATTESTOR_RPC_URL: rpcUrl,
      ATTESTOR_TLS_CERT_FILE: pki.node(id).cert,
      ATTESTOR_TLS_KEY_FILE: pki.node(id).key,
      ATTESTOR_TLS_CA_FILE: pki.caFile,
      ATTESTOR_OPERATOR_CA_FILE: pki.operatorCaFile,
      ATTESTOR_AUTHORITY_PUBLIC_KEY_FILE: producerPublic,
      ATTESTOR_AUTHORITY_ISSUER: `${idp.url}/auth/v1`,
      ATTESTOR_AUTHORITY_TENANT: 'gateway-entry',
      ATTESTOR_INVENTORY_PUBLIC_KEY_FILE: inventoryPublic,
      ATTESTOR_INVENTORY_FILE: inventoryFile,
    });
  }
  gatewayEnv = {
    ...process.env,
    NODE_ENV: 'test',
    LOG_LEVEL: 'info',
    API_WORKERS: '1',
    PORT: String(ports[11]),
    DATABASE_URL: pg.url,
    RPC_URLS: rpcUrl,
    HYPERPAXEER_RPC_URL: rpcUrl,
    SEQUENCER_RPC_URL: rpcUrl,
    WALLET_IDENTITY_BINDING_PRIVATE_KEY_FILE: producerFile,
    WALLET_IDENTITY_BINDING_TENANT: 'gateway-entry',
    WALLET_CUSTODY_INVENTORY_FILE: inventoryFile,
    WALLET_CUSTODY_INVENTORY_PUBLIC_KEY_FILE: inventoryPublic,
    ATTESTOR_ENDPOINTS: ids.map((_, i) => `https://127.0.0.1:${ports[i * 2]}`).join(','),
    ATTESTOR_QUORUM: '3',
    ATTESTOR_CLIENT_CERT_FILE: pki.clientCert,
    ATTESTOR_CLIENT_KEY_FILE: pki.clientKey,
    ATTESTOR_CA_FILE: pki.caFile,
  };
  delete gatewayEnv.LAYER_X_DB_URI;
  delete gatewayEnv.LAYERX_SYNC_INTERVAL_MS;
  const started = launch(
    process.execPath,
    ['--import', 'tsx', fileURLToPath(new URL('../src/index.ts', import.meta.url))],
    gatewayEnv
  );
  gateway = started.process;
  gatewayLogs = started.logs;
  gatewayBase = `http://127.0.0.1:${ports[11]}`;
  const deadline = Date.now() + 60_000;
  for (;;) {
    const failed = children.find(
      (child) =>
        child.process.exitCode !== null || child.process.signalCode !== null || !child.process.pid
    );
    if (failed) throw new Error(`gateway dependency exited: ${failed.logs.join('')}`);
    const response = await fetch(`${gatewayBase}/readyz`, {
      signal: AbortSignal.timeout(5_000),
    }).catch(() => null);
    if (response) {
      const body = await response.text();
      if (response.status === 200) break;
      if (Date.now() >= deadline)
        throw new Error(
          `gateway not ready: ${body}\n${children.flatMap((child) => child.logs).join('')}`
        );
    }
    if (Date.now() >= deadline) throw new Error(`gateway never listened: ${gatewayLogs.join('')}`);
    await new Promise((resolve) => setTimeout(resolve, 100));
  }
}, 120_000);

async function stopGatewayEntry() {
  for (const child of children.reverse()) {
    if (child.process.exitCode !== null || child.process.signalCode !== null || !child.process.pid)
      continue;
    const exited = new Promise<void>((resolve) => child.process.once('exit', () => resolve()));
    child.process.kill('SIGTERM');
    const timer = setTimeout(() => child.process.kill('SIGKILL'), 5_000);
    await exited;
    clearTimeout(timer);
  }
  if (work) rmSync(work, { recursive: true, force: true });
  if (pki) rmSync(pki.dir, { recursive: true, force: true });
}

async function gatewayStartsWithoutMirrorSyncWorker() {
  expect(gateway.exitCode).toBeNull();
  expect(gatewayEnv.LAYER_X_DB_URI).toBeUndefined();
  const response = await fetch(`${gatewayBase}/readyz`);
  expect(response.status).toBe(200);
  expect(await response.json()).toMatchObject({ ready: true });
  expect(response.headers.get('x-served-by')).toBe('paxeer-wallet-gateway');
  expect(gatewayLogs.join('')).toContain('listening on');
  expect(gatewayLogs.join('')).not.toContain('[layerx-sync]');
  const { EnvSchema } = await import('../src/env.js');
  const parsed = EnvSchema.parse({
    ...gatewayEnv,
    LAYER_X_DB_URI: 'retired',
    LAYERX_SYNC_INTERVAL_MS: 'retired',
  });
  expect(Object.hasOwn(parsed, 'LAYER_X_DB_URI')).toBe(false);
  expect(Object.hasOwn(parsed, 'LAYERX_SYNC_INTERVAL_MS')).toBe(false);
}

async function evmApprovalRoutesAreMounted() {
  for (const path of [
    '/v1/wallet/evm/review',
    '/v1/wallet/evm/approve',
    '/v1/wallet/evm/sign',
    `/v1/wallet/evm/approvals/${randomUUID()}`,
  ]) {
    const response = await fetch(`${gatewayBase}${path}`, {
      method: path.includes('/approvals/') ? 'GET' : 'POST',
      ...(path.includes('/approvals/')
        ? {}
        : { headers: { 'content-type': 'application/json' }, body: '{}' }),
    });
    expect(response.status).toBe(401);
    expect(await response.json()).toEqual({
      error: 'missing or malformed Authorization header',
    });
    expect(response.headers.get('x-served-by')).toBe('paxeer-wallet-gateway');
  }
}

test(gatewayStartsWithoutMirrorSyncWorker, gatewayStartsWithoutMirrorSyncWorker);
test(evmApprovalRoutesAreMounted, evmApprovalRoutesAreMounted);
