import { randomUUID } from 'node:crypto';
import Fastify, { type FastifyInstance } from 'fastify';
import { afterAll, beforeAll, expect, test } from 'vitest';
import { startIdentityProvider, type IdentityProvider } from './support/identity.js';
import { startPostgres, type EphemeralPostgres } from './support/postgres.js';

let idp: IdentityProvider;
let pg: EphemeralPostgres;
let app: FastifyInstance;
let pool: typeof import('../src/db/pool.js');
let wallets: typeof import('../src/db/wallets.js');
let authorization: string;
const userId = randomUUID();

beforeAll(async () => {
  idp = await startIdentityProvider();
  process.env.SUPABASE_URL = idp.url;
  pg = await startPostgres();
  pool = await import('../src/db/pool.js');
  wallets = await import('../src/db/wallets.js');
  const { env } = await import('../src/env.js');
  const { rpcPoolFromConfig } = await import('../src/rpc/pool.js');
  const { NonceStore } = await import('../src/nonce/store.js');
  const { signRoutes } = await import('../src/routes/sign.js');
  const rpc = rpcPoolFromConfig(env);
  const nonces = new NonceStore({
    pool: pool.getPool(),
    chainId: env.HYPERPAXEER_CHAIN_ID,
    pendingCount: (address) => rpc.getTransactionCount(address, 'latest'),
    lookupTransaction: (hash, member) => rpc.lookupTransaction(hash, member),
  });
  app = Fastify();
  await app.register(signRoutes, { rpc, nonces, attestors: null });
  await app.ready();
  await wallets.provisionWalletForUser(userId);
  authorization = `Bearer ${await idp.mintUserToken(userId)}`;
}, 120_000);

afterAll(async () => {
  await app?.close();
  await pool?.closePool();
  await pg?.stop();
  await idp?.stop();
}, 120_000);

async function custodyRoutesAreNotServed(): Promise<void> {
  const id = '0x' + '11'.repeat(32);
  for (const headers of [{}, { authorization }]) {
    for (const url of [
      `/v1/wallet/custody/${id}`,
      `/v1/wallet/custody-authorization/${id}`,
    ]) {
      const response = await app.inject({ method: 'GET', url, headers });
      expect(response.statusCode).toBe(404);
      expect(response.json()).toMatchObject({ statusCode: 404, error: 'Not Found' });
    }
    const response = await app.inject({
      method: 'POST',
      url: '/v1/wallet/sign-custody',
      headers,
      payload: { custody: '0x0102' },
    });
    expect(response.statusCode).toBe(404);
    expect(response.json()).toMatchObject({ statusCode: 404, error: 'Not Found' });
  }
}

async function walletSendRefusesCustodyProof(): Promise<void> {
  expect(await wallets.findWalletByUserId(userId)).toMatchObject({ user_id: userId });
  const tx = {
    to: '0x00000000000000000000000000000000000000e1',
    chainId: 125,
    nonce: 0,
    value: '1',
    data: '0x',
    gas: '21000',
    maxFeePerGas: '2000000000',
    maxPriorityFeePerGas: '1000000000',
  };
  const unauthenticated = await app.inject({
    method: 'POST',
    url: '/v1/wallet/send',
    payload: { tx },
  });
  expect(unauthenticated.statusCode).toBe(401);
  for (const custody of [
    { bytes: '0x0102', signature: '0x' + '11'.repeat(65) },
    null,
    {},
    '0x0102',
  ]) {
    const response = await app.inject({
      method: 'POST',
      url: '/v1/wallet/send',
      headers: { authorization },
      payload: { tx, custody },
    });
    expect(response.statusCode).toBe(400);
    expect(response.json()).toMatchObject({
      error: 'invalid_body',
      issues: [{ code: 'unrecognized_keys', keys: ['custody'], path: [] }],
    });
  }
}

test(custodyRoutesAreNotServed);
test(walletSendRefusesCustodyProof);
