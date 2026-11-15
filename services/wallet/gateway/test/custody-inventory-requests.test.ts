import { generateKeyPairSync, randomBytes, randomUUID, sign } from 'node:crypto';
import { mkdtempSync, rmSync, writeFileSync } from 'node:fs';
import { join } from 'node:path';
import { tmpdir } from 'node:os';
import { afterAll, beforeAll, describe, expect, it } from 'vitest';
import type { FastifyInstance } from 'fastify';
import { startIdentityProvider, type IdentityProvider } from './support/identity.js';
import { startPostgres, type EphemeralPostgres } from './support/postgres.js';

const ADMIN_TOKEN = randomBytes(32).toString('hex');
const USER_ID = randomUUID();
const KEY_ID = `wallet:${USER_ID}:standard:secp256k1:0`;
const PUBLIC_KEY = `04${'ab'.repeat(64)}`;

let idp: IdentityProvider;
let pg: EphemeralPostgres;
let app: FastifyInstance;
let poolModule: typeof import('../src/db/pool.js');
let inventoryDir: string;
let inventoryFile: string;
let inventoryPublicKeyFile: string;
let authorityPrivateKey: ReturnType<typeof generateKeyPairSync>['privateKey'];
let sequence = 0;

function writeInventory(keys: unknown[]): void {
  sequence += 1;
  const header = Buffer.from(
    JSON.stringify({ alg: 'ES256', typ: 'wallet-custody-inventory+jwt' })
  ).toString('base64url');
  const payload = Buffer.from(
    JSON.stringify({
      version: 1,
      iss: `${idp.url}/auth/v1`,
      aud: 'wallet-custody-inventory',
      tenant: 'custody-inventory-requests-test',
      sequence: String(sequence),
      iat: Math.floor(Date.now() / 1000),
      exp: Math.floor(Date.now() / 1000) + 3600,
      protocol: 'wallet',
      threshold: 3,
      members: Array.from({ length: 5 }, (_, i) => ({
        id: `node-${i + 1}`,
        spki_sha256: `${String(i + 1).repeat(64)}`,
      })),
      keys,
    })
  ).toString('base64url');
  const input = `${header}.${payload}`;
  const signature = sign('sha256', Buffer.from(input), {
    key: authorityPrivateKey,
    dsaEncoding: 'ieee-p1363',
  });
  writeFileSync(inventoryFile, `${input}.${signature.toString('base64url')}`);
}

beforeAll(async () => {
  idp = await startIdentityProvider();
  inventoryDir = mkdtempSync(join(tmpdir(), 'custody-inventory-requests-'));
  inventoryFile = join(inventoryDir, 'inventory.jwt');
  inventoryPublicKeyFile = join(inventoryDir, 'inventory-public.pem');
  const authority = generateKeyPairSync('ec', { namedCurve: 'prime256v1' });
  authorityPrivateKey = authority.privateKey;
  const identityPrivateKeyFile = join(inventoryDir, 'identity-private.pem');
  writeFileSync(
    identityPrivateKeyFile,
    authority.privateKey.export({ type: 'pkcs8', format: 'pem' }),
    {
      mode: 0o600,
    }
  );
  writeFileSync(
    inventoryPublicKeyFile,
    authority.publicKey.export({ type: 'spki', format: 'pem' })
  );
  writeInventory([]);

  Object.assign(process.env, {
    SUPABASE_URL: idp.url,
    GATEWAY_ADMIN_TOKEN: ADMIN_TOKEN,
    WALLET_IDENTITY_BINDING_TENANT: 'custody-inventory-requests-test',
    WALLET_IDENTITY_BINDING_PRIVATE_KEY_FILE: identityPrivateKeyFile,
    WALLET_CUSTODY_INVENTORY_FILE: inventoryFile,
    WALLET_CUSTODY_INVENTORY_PUBLIC_KEY_FILE: inventoryPublicKeyFile,
  });
  pg = await startPostgres();

  const index = await import('../src/index.js');
  poolModule = await import('../src/db/pool.js');
  app = await index.buildApp();
}, 120_000);

afterAll(async () => {
  await app?.close();
  await poolModule?.closePool();
  await pg?.stop();
  await idp?.stop();
  if (inventoryDir) rmSync(inventoryDir, { recursive: true, force: true });
}, 120_000);

describe('GET /v1/admin/custody-inventory/requests', () => {
  it('refuses a request without the administrator bearer token', async () => {
    const response = await app.inject({
      method: 'GET',
      url: '/v1/admin/custody-inventory/requests',
    });
    expect(response.statusCode).toBe(401);
  });

  it('lists generation, signing, and admitted custody requests exactly once', async () => {
    await poolModule.query(
      `insert into account_provisioning (user_id, kind, evm_key_id, evm_key_owner)
       values ($1, 'standard', $2, $3)`,
      [USER_ID, KEY_ID, USER_ID]
    );

    const get = () =>
      app.inject({
        method: 'GET',
        url: '/v1/admin/custody-inventory/requests',
        headers: { authorization: `Bearer ${ADMIN_TOKEN}` },
      });

    let response = await get();
    expect(response.statusCode).toBe(200);
    expect(response.json()).toMatchObject({
      inventory_sequence: '1',
      count: 1,
      inventory_requests: [
        {
          key_id: KEY_ID,
          epoch: 0,
          curve: 'secp256k1',
          public_key: '',
          owner: USER_ID,
          operations: ['generate'],
        },
      ],
    });

    writeInventory([
      {
        key_id: KEY_ID,
        epoch: 0,
        curve: 'secp256k1',
        public_key: '',
        owner: USER_ID,
        operations: ['generate'],
      },
    ]);
    await poolModule.query(
      `update account_provisioning
          set evm_public_key = $2, evm_key_epoch = 7
        where user_id = $1 and kind = 'standard'`,
      [USER_ID, PUBLIC_KEY]
    );

    response = await get();
    expect(response.json()).toMatchObject({
      inventory_sequence: '2',
      count: 1,
      inventory_requests: [
        {
          key_id: KEY_ID,
          epoch: 7,
          curve: 'secp256k1',
          public_key: PUBLIC_KEY.toLowerCase(),
          owner: USER_ID,
          operations: ['sign', 'refresh'],
        },
      ],
    });

    writeInventory([
      {
        key_id: KEY_ID,
        epoch: 7,
        curve: 'secp256k1',
        public_key: PUBLIC_KEY.toLowerCase(),
        owner: USER_ID,
        operations: ['sign', 'refresh'],
      },
    ]);
    response = await get();
    expect(response.json()).toMatchObject({
      inventory_sequence: '3',
      count: 0,
      inventory_requests: [],
    });
  });
});
