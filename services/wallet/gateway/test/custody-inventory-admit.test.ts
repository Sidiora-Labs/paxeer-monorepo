import { createPrivateKey, generateKeyPairSync, randomBytes, sign } from 'node:crypto';
import { readFileSync } from 'node:fs';
import { Agent, request } from 'node:https';
import Fastify, { type FastifyInstance } from 'fastify';
import { afterAll, beforeAll, expect, test, vi } from 'vitest';
import { startCustodyAuthority } from './support/custody.js';
import { startIdentityProvider, type IdentityProvider } from './support/identity.js';
import { startPostgres, type EphemeralPostgres } from './support/postgres.js';
import { TestChain } from './support/chain.js';
import type { WalletInventory, WalletInventoryKey } from '../src/agent/authority.js';
import type { AttestorClient } from '../src/attestor/client.js';

const adminToken = randomBytes(32).toString('hex');
const chain = new TestChain(125);
const clients = new Set<AttestorClient>();
const pools: Array<typeof import('../src/db/pool.js')> = [];
const newerKey: WalletInventoryKey = {
  key_id: 'custody-inventory-admitted-key',
  epoch: 0,
  curve: 'ed25519',
  public_key: generateKeyPairSync('ed25519')
    .publicKey.export({ type: 'spki', format: 'der' })
    .subarray(-32)
    .toString('hex'),
  owner: 'custody-inventory-owner',
  operations: ['sign', 'refresh'],
};
const followingKey: WalletInventoryKey = { ...newerKey, key_id: 'custody-inventory-following-key' };
let identity: IdentityProvider;
let postgres: EphemeralPostgres;
let custody: Awaited<ReturnType<typeof startCustodyAuthority>>;
let app: FastifyInstance;
let database: typeof import('../src/db/pool.js');
let authority: typeof import('../src/agent/authority.js');
let follower: typeof import('../src/agent/authority.js');
let tls: Agent;
let originalToken: string;
let admittedToken: string;
let followingToken: string;
let originalInventory: WalletInventory;
let endpoints: string[];

function signedInventory(sequence: string, keys: WalletInventoryKey[]): string {
  const header = originalToken.split('.')[0]!;
  const body = Buffer.from(JSON.stringify({ ...originalInventory, sequence, keys })).toString(
    'base64url'
  );
  const signature = sign('sha256', Buffer.from(`${header}.${body}`), {
    key: createPrivateKey(readFileSync(custody.env.WALLET_IDENTITY_BINDING_PRIVATE_KEY_FILE)),
    dsaEncoding: 'ieee-p1363',
  });
  return `${header}.${body}.${signature.toString('base64url')}`;
}

function admit(token: string, authenticated = true) {
  return app.inject({
    method: 'POST',
    url: '/v1/admin/custody-inventory',
    headers: authenticated ? { authorization: `Bearer ${adminToken}` } : {},
    payload: { token },
  });
}

function peerInventory(
  endpoint: string,
  token: string
): Promise<{ status: number; body: Record<string, unknown> }> {
  return new Promise((resolve, reject) => {
    const body = JSON.stringify({ token });
    const req = request(
      `${endpoint}/v1/inventory`,
      {
        agent: tls,
        method: 'POST',
        timeout: 10_000,
        headers: { 'content-type': 'application/json', 'content-length': Buffer.byteLength(body) },
      },
      (res) => {
        const chunks: Buffer[] = [];
        res.on('data', (chunk: Buffer) => chunks.push(chunk));
        res.on('error', reject);
        res.on('end', () => {
          try {
            resolve({
              status: res.statusCode ?? 0,
              body: JSON.parse(Buffer.concat(chunks).toString('utf8')),
            });
          } catch (error) {
            reject(error);
          }
        });
      }
    );
    req.on('error', reject);
    req.on('timeout', () => req.destroy(new Error('inventory request timed out')));
    req.end(body);
  });
}

async function retainedInventory() {
  const { rows } = await database.query<{
    inventory_sequence: string;
    inventory_token: string | null;
  }>(
    'select inventory_sequence::text, inventory_token from wallet_custody_authority where singleton = true'
  );
  return rows[0];
}

async function trackConfiguredClient(): Promise<void> {
  const { defaultWalletAttestors } = await import('../src/db/wallets.js');
  const client = defaultWalletAttestors();
  expect(client).not.toBeNull();
  if (client) clients.add(client);
}

beforeAll(async () => {
  identity = await startIdentityProvider();
  await chain.start();
  custody = await startCustodyAuthority(identity, chain.url);
  Object.assign(process.env, custody.env, {
    SUPABASE_URL: identity.url,
    GATEWAY_ADMIN_TOKEN: adminToken,
    WALLET_CUSTODY_INVENTORY_REFRESH_INTERVAL_MS: '100',
    LOG_LEVEL: 'error',
  });
  endpoints = custody.env.ATTESTOR_ENDPOINTS.split(',');
  tls = new Agent({
    cert: readFileSync(custody.env.ATTESTOR_CLIENT_CERT_FILE),
    key: readFileSync(custody.env.ATTESTOR_CLIENT_KEY_FILE),
    ca: readFileSync(custody.env.ATTESTOR_CA_FILE),
    minVersion: 'TLSv1.3',
    rejectUnauthorized: true,
  });
  originalToken = readFileSync(custody.env.WALLET_CUSTODY_INVENTORY_FILE, 'utf8').trim();
  originalInventory = JSON.parse(
    Buffer.from(originalToken.split('.')[1]!, 'base64url').toString('utf8')
  );
  admittedToken = signedInventory('2', [newerKey]);
  followingToken = signedInventory('4', [newerKey, followingKey]);
  postgres = await startPostgres();
  database = await import('../src/db/pool.js');
  pools.push(database);
  authority = await import('../src/agent/authority.js');
  await trackConfiguredClient();
  const { adminRoutes } = await import('../src/routes/admin.js');
  app = Fastify();
  await app.register(adminRoutes);
  await app.ready();
}, 180_000);

afterAll(async () => {
  await app?.close();
  for (const client of clients) client.stop();
  tls?.destroy();
  await authority?.stopWalletInventoryRefresh();
  await follower?.stopWalletInventoryRefresh();
  for (const pool of pools) await pool.closePool();
  await postgres?.stop();
  await custody?.stop();
  await chain.stop();
  await identity?.stop();
}, 120_000);

async function custodyInventoryRequiresAdministratorBearer(): Promise<void> {
  const before = await retainedInventory();
  const response = await admit(admittedToken, false);
  expect(response.statusCode).toBe(401);
  expect(response.json()).toEqual({ error: 'unauthorized' });
  expect(await retainedInventory()).toEqual(before);
  expect(authority.loadWalletInventory().sequence).toBe('1');
}

async function custodyInventoryReachesFivePeersAndRetainsToken(): Promise<void> {
  expect(() => authority.requireInventoryKey(newerKey.key_id, 'sign', newerKey.owner)).toThrow();
  const response = await admit(admittedToken);
  expect(response.statusCode).toBe(200);
  expect(response.json()).toEqual({
    sequence: '2',
    members: originalInventory.members.map((member) => ({ node_id: member.id, sequence: '2' })),
  });
  expect(response.body).not.toContain(admittedToken);
  expect(response.json()).not.toHaveProperty('token');
  expect(await retainedInventory()).toEqual({
    inventory_sequence: '2',
    inventory_token: admittedToken,
  });
  expect(authority.requireInventoryKey(newerKey.key_id, 'sign', newerKey.owner)).toEqual(newerKey);
  for (const [index, endpoint] of endpoints.entries()) {
    const held = await peerInventory(endpoint, admittedToken);
    expect(held).toEqual({
      status: 200,
      body: { node_id: originalInventory.members[index]!.id, sequence: '2', status: 'held' },
    });
  }
  const repeat = await admit(admittedToken);
  expect(repeat.statusCode).toBe(200);
  expect(repeat.json()).toEqual(response.json());
  expect(await retainedInventory()).toEqual({
    inventory_sequence: '2',
    inventory_token: admittedToken,
  });
}

async function custodyInventoryRefusesLowerAndConflictingSequences(): Promise<void> {
  const lower = await admit(originalToken);
  expect(lower.statusCode).toBe(409);
  expect(lower.json()).toEqual({ error: 'custody_inventory_sequence_conflict' });
  const conflict = await admit(signedInventory('2', [followingKey]));
  expect(conflict.statusCode).toBe(409);
  expect(await retainedInventory()).toEqual({
    inventory_sequence: '2',
    inventory_token: admittedToken,
  });
  expect(authority.loadWalletInventory().sequence).toBe('2');
}

async function custodyInventoryChecksSignedTokenBeforePublication(): Promise<void> {
  const invalid = `${admittedToken.slice(0, admittedToken.lastIndexOf('.') + 1)}${Buffer.alloc(64).toString('base64url')}`;
  expect((await admit(invalid)).statusCode).toBe(400);
  expect((await admit(signedInventory('3', [{ ...newerKey, operations: [] }]))).statusCode).toBe(
    400
  );
  expect(await retainedInventory()).toEqual({
    inventory_sequence: '2',
    inventory_token: admittedToken,
  });
}

async function custodyInventoryFreshImportUsesRetainedTokenWithOlderFile(): Promise<void> {
  expect(readFileSync(custody.env.WALLET_CUSTODY_INVENTORY_FILE, 'utf8').trim()).toBe(
    originalToken
  );
  vi.resetModules();
  follower = await import('../src/agent/authority.js');
  const { runMigrations } = await import('../src/db/migrate.js');
  const followerDatabase = await import('../src/db/pool.js');
  pools.push(followerDatabase);
  await runMigrations();
  await trackConfiguredClient();
  expect(follower.loadWalletInventory().sequence).toBe('2');
  expect(follower.requireInventoryKey(newerKey.key_id, 'sign', newerKey.owner)).toEqual(newerKey);
}

async function custodyInventoryNamesRefusingMemberAndRetainsNothing(): Promise<void> {
  const advance = await peerInventory(endpoints[4]!, followingToken);
  expect(advance).toEqual({
    status: 200,
    body: { node_id: originalInventory.members[4]!.id, sequence: '4', status: 'admitted' },
  });
  const response = await admit(signedInventory('3', [newerKey]));
  expect(response.statusCode).toBe(502);
  expect(response.json()).toEqual({
    sequence: '3',
    members: [
      ...originalInventory.members
        .slice(0, 4)
        .map((member) => ({ node_id: member.id, sequence: '3' })),
      { node_id: originalInventory.members[4]!.id, sequence: null, error: 'inventory_refused' },
    ],
  });
  expect(response.json()).not.toHaveProperty('token');
  expect(await retainedInventory()).toEqual({
    inventory_sequence: '2',
    inventory_token: admittedToken,
  });
  expect(authority.loadWalletInventory().sequence).toBe('2');
  expect(follower.loadWalletInventory().sequence).toBe('2');
}

async function custodyInventoryRefreshesOtherGatewayWithoutRestart(): Promise<void> {
  expect(() =>
    follower.requireInventoryKey(followingKey.key_id, 'sign', followingKey.owner)
  ).toThrow();
  const response = await admit(followingToken);
  expect(response.statusCode).toBe(200);
  expect(response.json()).toEqual({
    sequence: '4',
    members: originalInventory.members.map((member) => ({ node_id: member.id, sequence: '4' })),
  });
  await expect.poll(() => follower.loadWalletInventory().sequence, { timeout: 5_000 }).toBe('4');
  expect(follower.requireInventoryKey(followingKey.key_id, 'sign', followingKey.owner)).toEqual(
    followingKey
  );
  expect(authority.requireInventoryKey(followingKey.key_id, 'sign', followingKey.owner)).toEqual(
    followingKey
  );
  expect(readFileSync(custody.env.WALLET_CUSTODY_INVENTORY_FILE, 'utf8').trim()).toBe(
    originalToken
  );
  expect(await retainedInventory()).toEqual({
    inventory_sequence: '4',
    inventory_token: followingToken,
  });
}

test(custodyInventoryRequiresAdministratorBearer.name, custodyInventoryRequiresAdministratorBearer);
test(
  custodyInventoryReachesFivePeersAndRetainsToken.name,
  custodyInventoryReachesFivePeersAndRetainsToken
);
test(
  custodyInventoryRefusesLowerAndConflictingSequences.name,
  custodyInventoryRefusesLowerAndConflictingSequences
);
test(
  custodyInventoryChecksSignedTokenBeforePublication.name,
  custodyInventoryChecksSignedTokenBeforePublication
);
test(
  custodyInventoryFreshImportUsesRetainedTokenWithOlderFile.name,
  custodyInventoryFreshImportUsesRetainedTokenWithOlderFile
);
test(
  custodyInventoryNamesRefusingMemberAndRetainsNothing.name,
  custodyInventoryNamesRefusingMemberAndRetainsNothing
);
test(
  custodyInventoryRefreshesOtherGatewayWithoutRestart.name,
  custodyInventoryRefreshesOtherGatewayWithoutRestart
);
