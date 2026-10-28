import { randomBytes, randomUUID } from 'node:crypto';
import { createServer, type Server } from 'node:http';
import type { AddressInfo } from 'node:net';
import { afterAll, beforeAll, describe, expect, it } from 'vitest';
import type { FastifyInstance } from 'fastify';
import { keccak256, parseTransaction, recoverTransactionAddress } from 'viem';
import { startPostgres, type EphemeralPostgres } from './support/postgres.js';
import { newAgentKey, startIdentityProvider, type IdentityProvider } from './support/identity.js';

let idp: IdentityProvider;
let pg: EphemeralPostgres;
let app: FastifyInstance;
let rpcServer: Server;
let latestCount = 7;
let poolModule: typeof import('../src/db/pool.js');
const broadcasts: `0x${string}`[] = [];

beforeAll(async () => {
  rpcServer = createServer((req, res) => {
    const chunks: Buffer[] = [];
    req.on('data', (c: Buffer) => chunks.push(c));
    req.on('end', () => {
      const body = JSON.parse(Buffer.concat(chunks).toString('utf8')) as {
        id: number; method: string; params: [`0x${string}`];
      };
      if (body.method === 'eth_sendRawTransaction') {
        broadcasts.push(body.params[0]);
        res.writeHead(503, { 'content-type': 'text/plain' });
        res.end('broadcast outcome unavailable');
        return;
      }
      const result = body.method === 'eth_getTransactionCount' ? `0x${latestCount.toString(16)}`
        : body.method === 'eth_blockNumber' ? '0x64'
          : body.method === 'eth_call' ? '0x'
            : body.method === 'eth_getTransactionByHash' ? null : undefined;
      res.writeHead(200, { 'content-type': 'application/json' });
      res.end(JSON.stringify(result === undefined
        ? { jsonrpc: '2.0', id: body.id, error: { code: -32601, message: 'method not found' } }
        : { jsonrpc: '2.0', id: body.id, result }));
    });
  });
  await new Promise<void>((r) => rpcServer.listen(0, '127.0.0.1', () => r()));
  idp = await startIdentityProvider();
  process.env.SUPABASE_URL = idp.url;
  process.env.AGENT_JWT_SECRET = randomBytes(32).toString('hex');
  process.env.LOG_LEVEL = 'error';
  process.env.RPC_URLS = `http://127.0.0.1:${(rpcServer.address() as AddressInfo).port}`;
  process.env.SEQUENCER_RPC_URL = process.env.RPC_URLS;
  pg = await startPostgres();
  const index = await import('../src/index.js');
  poolModule = await import('../src/db/pool.js');
  app = await index.buildApp();
  await app.ready();
}, 120_000);

afterAll(async () => {
  await app?.close();
  await poolModule?.closePool();
  await pg?.stop();
  await idp?.stop();
  await new Promise<void>((r) => (rpcServer ? rpcServer.close(() => r()) : r()));
}, 120_000);

async function asUser(userId: string, url: string, payload: Record<string, unknown>) {
  return app.inject({ method: 'POST', url, headers: { authorization: `Bearer ${await idp.mintUserToken(userId)}` }, payload });
}

const tx = (nonce?: number) => ({
  to: '0x00000000000000000000000000000000000000e1', value: '1', gas: '21000', maxFeePerGas: '2000000000',
  maxPriorityFeePerGas: '1000000000', ...(nonce === undefined ? {} : { nonce }),
});

async function refusesOutstandingClientNonceOnSendAndTracksTheHashOfAFailedSend() {
  const user = randomUUID();
  const provisioned = await asUser(user, '/v1/wallet/provision', {});
  expect(provisioned.statusCode).toBe(200);
  const { id, address } = provisioned.json().wallet as { id: string; address: string };
  const key = address.toLowerCase();
  latestCount = 7;
  broadcasts.length = 0;

  const signed = await asUser(user, '/v1/wallet/sign', { tx: tx(9) });
  expect(signed.statusCode).toBe(200);
  const signedTx = (signed.json() as { signed_tx: `0x${string}` }).signed_tx;
  const signaturesBefore = await poolModule.query(
    'select id from wallet_signatures where wallet_id = $1 order by id', [id],
  );
  expect(signaturesBefore.rows).toHaveLength(1);
  const blocked = await asUser(user, '/v1/wallet/send', { tx: tx(9) });
  expect(blocked.statusCode).toBe(409);
  expect(blocked.json()).toEqual({ error: 'nonce_outstanding', nonce: 9, request_id: expect.any(String) });
  expect(broadcasts).toEqual([]);
  const signaturesAfter = await poolModule.query(
    'select id from wallet_signatures where wallet_id = $1 order by id', [id],
  );
  expect(signaturesAfter.rows).toEqual(signaturesBefore.rows);
  const reservation = await poolModule.query(
    'select nonce::text, tx_hash from nonce_outstanding where address = $1', [key],
  );
  expect(reservation.rows).toEqual([{ nonce: '9', tx_hash: keccak256(signedTx) }]);

  const below = await asUser(user, '/v1/wallet/send', { tx: tx(6) });
  expect(below.statusCode).toBe(409);
  expect(below.json()).toEqual({ error: 'nonce_below_latest', nonce: 6, latest: 7, request_id: expect.any(String) });
  expect(broadcasts).toEqual([]);

  for (const [clientNonce, expectedNonce] of [[10, 10], [undefined, 7]] as const) {
    const countBefore = broadcasts.length;
    const failed = await asUser(user, '/v1/wallet/send', { tx: tx(clientNonce) });
    expect(failed.statusCode).toBe(500);
    expect(failed.json()).toEqual({ error: 'send_failed', detail: 'HTTP 503' });
    expect(broadcasts).toHaveLength(countBefore + 1);
    const raw = broadcasts[countBefore]!;
    expect(parseTransaction(raw).nonce).toBe(expectedNonce);
    expect((await recoverTransactionAddress({ serializedTransaction: raw })).toLowerCase()).toBe(key);
    const outstanding = await poolModule.query(
      'select nonce::text, tx_hash, member from nonce_outstanding where address = $1 and nonce = $2',
      [key, expectedNonce],
    );
    expect(outstanding.rows).toEqual([{ nonce: String(expectedNonce), tx_hash: keccak256(raw), member: null }]);
    const retry = await asUser(user, '/v1/wallet/send', { tx: tx(expectedNonce) });
    expect(retry.statusCode).toBe(409);
    expect(retry.json()).toEqual({ error: 'nonce_outstanding', nonce: expectedNonce, request_id: expect.any(String) });
    expect(broadcasts).toHaveLength(countBefore + 1);
  }
}

describe('/v1/wallet/sign after migration 019', () => {
  it('refuses an outstanding client nonce on /v1/wallet/send and tracks the hash of a failed send',
    refusesOutstandingClientNonceOnSendAndTracksTheHashOfAFailedSend);

  it('reserves the client nonce with the signed hash, refuses it while outstanding and below latest, and returns the broadcast deadline', async () => {
    const migration = await poolModule.query(
      'select filename from _migrations where filename = $1',
      ['019_drop_retired_layerx_custody.sql'],
    );
    expect(migration.rows).toEqual([{ filename: '019_drop_retired_layerx_custody.sql' }]);
    const tables = await poolModule.query(
      `select to_regclass('public.wallet_custody_submissions')::text as retired,
              to_regclass('public.custody_signing_reservations')::text as retained`,
    );
    expect(tables.rows).toEqual([{ retired: null, retained: 'custody_signing_reservations' }]);
    const user = randomUUID();
    const provisioned = await asUser(user, '/v1/wallet/provision', {});
    expect(provisioned.statusCode).toBe(200);
    const { address } = (provisioned.json() as { wallet: { address: string } }).wallet;
    latestCount = 7;
    const before = Date.now();
    const signed = await asUser(user, '/v1/wallet/sign', { tx: tx(9) });
    const after = Date.now();
    expect(signed.statusCode).toBe(200);
    const body = signed.json() as { signed_tx: `0x${string}`; nonce: number; broadcast_deadline: string };
    expect(body.nonce).toBe(9);
    expect(parseTransaction(body.signed_tx).nonce).toBe(9);
    expect((await recoverTransactionAddress({ serializedTransaction: body.signed_tx })).toLowerCase()).toBe(address.toLowerCase());
    const { BROADCAST_GRACE_MS } = await import('../src/chain.js');
    expect(Date.parse(body.broadcast_deadline)).toBeGreaterThanOrEqual(before + BROADCAST_GRACE_MS);
    expect(Date.parse(body.broadcast_deadline)).toBeLessThanOrEqual(after + BROADCAST_GRACE_MS + 1_000);
    const { rows } = await poolModule.query<{ nonce: string; tx_hash: string }>(
      'select nonce::text as nonce, tx_hash from nonce_outstanding where address = $1', [address.toLowerCase()]);
    expect(rows).toEqual([{ nonce: '9', tx_hash: keccak256(body.signed_tx) }]);
    const again = await asUser(user, '/v1/wallet/sign', { tx: tx(9) });
    expect(again.statusCode).toBe(409);
    expect(again.json()).toEqual({ error: 'nonce_outstanding', nonce: 9, request_id: expect.any(String) });
    const below = await asUser(user, '/v1/wallet/sign', { tx: tx(6) });
    expect(below.statusCode).toBe(409);
    expect(below.json()).toEqual({ error: 'nonce_below_latest', nonce: 6, latest: 7, request_id: expect.any(String) });
    const gateway = await asUser(user, '/v1/wallet/sign', { tx: tx() });
    expect(gateway.statusCode).toBe(200);
    expect((gateway.json() as { nonce: number }).nonce).toBe(7);
  });

  it('keeps custody reservations exclusive to their action and chain before signing', async () => {
    const user = randomUUID();
    const provisioned = await asUser(user, '/v1/wallet/provision', {});
    expect(provisioned.statusCode).toBe(200);
    const { id, address, chain_id: chainId } = provisioned.json().wallet as {
      id: string; address: string; chain_id: number;
    };
    const key = address.toLowerCase();
    const agent = newAgentKey('nonce-reservation');
    const actionId = randomUUID();
    await poolModule.query(
      `insert into agent_principals (did, label, key_fingerprint, public_key)
       values ($1, 'nonce-reservation', $2, $3)`,
      [agent.did, agent.publicKeyHex.slice(0, 16), agent.publicKeyHex],
    );
    await poolModule.query(
      `insert into agent_actions (id, did, wallet_id, wallet_address, kind, idempotency_key, request)
       values ($1, $2, $3, $4, 'allowance_and_call', $1, '{}'::jsonb)`,
      [actionId, agent.did, id, address],
    );
    await poolModule.query(
      `insert into custody_signing_reservations (address, chain_id, action_id, nonce)
       values ($1, $2, $3, 7), ($1, $4, $3, 7)`,
      [key, chainId, actionId, chainId + 1],
    );
    const { sharedNonceStore } = await import('../src/nonce/store.js');
    const store = sharedNonceStore();
    const refusal = 'wallet nonce is reserved by an action awaiting custody authorization or broadcast';
    let callbacks = 0;
    for (const owner of [undefined, randomUUID()]) {
      await expect(store.withLock(address, async (lease) => {
        callbacks += 1;
        return lease.peek();
      }, owner)).rejects.toThrow(refusal);
    }
    expect(callbacks).toBe(0);
    const allocations = await poolModule.query('select address from nonce_allocations where address = $1', [key]);
    expect(allocations.rows).toEqual([]);

    const blocked = await asUser(user, '/v1/wallet/sign', { tx: tx() });
    expect(blocked.statusCode).toBe(500);
    expect(blocked.json()).toEqual({ error: 'sign_failed', detail: refusal, request_id: expect.any(String) });
    const signatures = await poolModule.query('select id from wallet_signatures where wallet_id = $1', [id]);
    expect(signatures.rows).toEqual([]);

    latestCount = 7;
    expect(await store.withLock(address, async (lease) => {
      const nonce = await lease.next();
      await lease.release(nonce);
      return nonce;
    }, actionId)).toBe(7);
    const reservations = await poolModule.query(
      `select chain_id::text, action_id, nonce::text, signed_hash from custody_signing_reservations
       where address = $1 order by chain_id`,
      [key],
    );
    expect(reservations.rows).toEqual([
      { chain_id: String(chainId), action_id: actionId, nonce: '7', signed_hash: null },
      { chain_id: String(chainId + 1), action_id: actionId, nonce: '7', signed_hash: null },
    ]);
    await poolModule.query('delete from custody_signing_reservations where address = $1 and chain_id = $2', [key, chainId]);
    const signed = await asUser(user, '/v1/wallet/sign', { tx: tx() });
    expect(signed.statusCode).toBe(200);
    const body = signed.json() as { signed_tx: `0x${string}`; nonce: number };
    expect(body.nonce).toBe(7);
    const outstanding = await poolModule.query(
      'select nonce::text, tx_hash from nonce_outstanding where address = $1', [key],
    );
    expect(outstanding.rows).toEqual([{ nonce: '7', tx_hash: keccak256(body.signed_tx) }]);
  });

  it('rolls back incomplete leases and gives concurrent sign requests distinct tracked nonces', async () => {
    const user = randomUUID();
    const provisioned = await asUser(user, '/v1/wallet/provision', {});
    expect(provisioned.statusCode).toBe(200);
    const { address } = provisioned.json().wallet as { address: string };
    const key = address.toLowerCase();
    const { sharedNonceStore } = await import('../src/nonce/store.js');
    const store = sharedNonceStore();
    latestCount = 7;
    const failure = new Error('signing interrupted');
    await expect(store.withLock(address, async (lease) => {
      expect(await lease.next()).toBe(7);
      throw failure;
    })).rejects.toBe(failure);
    await expect(store.withLock(address, (lease) => lease.next())).rejects.toThrow(
      'reserved nonce 7 was neither tracked with a signed transaction hash nor released',
    );
    const allocations = await poolModule.query('select address from nonce_allocations where address = $1', [key]);
    expect(allocations.rows).toEqual([]);
    const rolledBack = await poolModule.query('select nonce from nonce_outstanding where address = $1', [key]);
    expect(rolledBack.rows).toEqual([]);

    const responses = await Promise.all([
      asUser(user, '/v1/wallet/sign', { tx: tx() }),
      asUser(user, '/v1/wallet/sign', { tx: tx() }),
    ]);
    const signed = responses.map((response) => {
      expect(response.statusCode).toBe(200);
      return response.json() as { signed_tx: `0x${string}`; nonce: number };
    }).sort((a, b) => a.nonce - b.nonce);
    expect(signed.map((body) => body.nonce)).toEqual([7, 8]);
    for (const body of signed) {
      expect(parseTransaction(body.signed_tx).nonce).toBe(body.nonce);
      expect((await recoverTransactionAddress({ serializedTransaction: body.signed_tx })).toLowerCase()).toBe(key);
    }
    const outstanding = await poolModule.query(
      'select nonce::text, tx_hash from nonce_outstanding where address = $1 order by nonce_outstanding.nonce',
      [key],
    );
    expect(outstanding.rows).toEqual(signed.map((body) => ({ nonce: String(body.nonce), tx_hash: keccak256(body.signed_tx) })));
  });
});
