import { generateKeyPairSync, randomUUID } from 'node:crypto';
import { createServer, type Server } from 'node:http';
import Fastify, { type FastifyInstance } from 'fastify';
import { decodeFunctionData, encodeFunctionResult, parseAbi, type Hex } from 'viem';
import { afterAll, beforeAll, expect, test } from 'vitest';
import { startIdentityProvider, type IdentityProvider } from './support/identity.js';
import { startPostgres, type EphemeralPostgres } from './support/postgres.js';

const identityAbi = parseAbi([
  'function getLayerXDid(address addr) view returns (bytes32 didPublicKey, string did)',
]);
const sponsor = `0x${'22'.repeat(20)}`;
const relayerSignature = `0x${'33'.repeat(65)}`;
const requests: { path: string; body: unknown }[] = [];
const endpointErrors: unknown[] = [];
const identities = new Map<string, { key: Hex; did: string }>();
let quoteReply = { status: 429, body: { error: 'budget_exhausted' } as Record<string, unknown> };
let statusReply: Record<string, unknown>;
let identity: IdentityProvider;
let postgres: EphemeralPostgres;
let server: Server;
let app: FastifyInstance;
let database: typeof import('../src/db/pool.js');
let wallets: typeof import('../src/db/wallets.js');
let binding: typeof import('../src/provision/bind.js');

beforeAll(async () => {
  identity = await startIdentityProvider();
  process.env.SUPABASE_URL = identity.url;
  postgres = await startPostgres();
  database = await import('../src/db/pool.js');
  wallets = await import('../src/db/wallets.js');
  binding = await import('../src/provision/bind.js');
  const { env } = await import('../src/env.js');
  const { RpcPool } = await import('../src/rpc/pool.js');
  const { walletSponsorRoutes } = await import('../src/routes/wallet.js');
  server = createServer(async (req, res) => {
    try {
      const chunks: Buffer[] = [];
      for await (const chunk of req) chunks.push(Buffer.from(chunk));
      const body = JSON.parse(Buffer.concat(chunks).toString('utf8'));
      expect(req.method).toBe('POST');
      expect(req.headers['content-type']).toBe('application/json');
      res.setHeader('content-type', 'application/json');
      if (req.url === '/rpc') {
        expect(body.method).toBe('eth_call');
        expect(body.params[0].to).toBe(binding.ADDR_PRECOMPILE);
        expect(body.params[1]).toBe('latest');
        const call = decodeFunctionData({ abi: identityAbi, data: body.params[0].data });
        expect(call.functionName).toBe('getLayerXDid');
        const bound = identities.get(call.args[0].toLowerCase());
        if (!bound) throw new Error('identity read for an unknown wallet');
        const result = encodeFunctionResult({
          abi: identityAbi,
          functionName: 'getLayerXDid',
          result: [bound.key, bound.did],
        });
        res.end(JSON.stringify({ jsonrpc: '2.0', id: body.id, result }));
        return;
      }
      requests.push({ path: req.url!, body });
      if (req.url === '/station/quote') {
        res.writeHead(quoteReply.status);
        res.end(JSON.stringify(quoteReply.body));
        return;
      }
      expect(req.url).toBe('/station/status');
      res.end(JSON.stringify(statusReply));
    } catch (error) {
      endpointErrors.push(error);
      res.writeHead(500, { 'content-type': 'application/json' });
      res.end(JSON.stringify({ error: 'station_contract_violation' }));
    }
  });
  await new Promise<void>((resolve, reject) => {
    server.once('error', reject);
    server.listen(0, '127.0.0.1', resolve);
  });
  const address = server.address();
  if (!address || typeof address === 'string') throw new Error('station did not bind');
  const url = `http://127.0.0.1:${address.port}`;
  const rpc = new RpcPool({
    urls: [`${url}/rpc`],
    chainId: env.HYPERPAXEER_CHAIN_ID,
    timeoutMs: 2_000,
    healthIntervalMs: 60_000,
    lagThresholdBlocks: 2,
  });
  app = Fastify();
  await app.register(walletSponsorRoutes, { stationUrl: `${url}/station`, rpc });
  await app.ready();
}, 120_000);

afterAll(async () => {
  try {
    await app?.close();
  } finally {
    try {
      if (server)
        await new Promise<void>((resolve, reject) =>
          server.close((error) => (error ? reject(error) : resolve()))
        );
    } finally {
      try {
        await database?.closePool();
      } finally {
        try {
          await postgres?.stop();
        } finally {
          await identity?.stop();
        }
      }
    }
  }
}, 120_000);

async function caller() {
  const userId = randomUUID();
  const { wallet } = await wallets.provisionWalletForUser(userId);
  const publicKey = generateKeyPairSync('ed25519').publicKey.export({
    format: 'der',
    type: 'spki',
  });
  const key = `0x${publicKey.subarray(-32).toString('hex')}` as Hex;
  const did = binding.didFromPublicKey(key);
  identities.set(wallet.address.toLowerCase(), { key, did });
  return { wallet, did, token: await identity.mintUserToken(userId) };
}

async function sponsoredQuoteAnswersBudgetExhausted(): Promise<void> {
  const owner = await caller();
  const body = {
    chainId: String(owner.wallet.chain_id),
    account: owner.wallet.address,
    nonce: '7',
    calls: [{ to: `0x${'44'.repeat(20)}`, value: '0', data: '0x1234' }],
    maxTokenAmount: '1000000',
    gasCost: '200000000000000',
    token: binding.SIDIORA_TOKEN,
    decimals: 6,
  };
  requests.length = 0;
  const send = (payload: unknown, token: string | null = owner.token) =>
    app.inject({
      method: 'POST',
      url: '/v1/wallet/sponsored/quote',
      headers: token === null ? {} : { authorization: `Bearer ${token}` },
      payload,
    });
  expect((await send(body, null)).statusCode).toBe(401);
  expect((await send({ ...body, calls: [] })).statusCode).toBe(400);
  expect((await send({ ...body, did: owner.did })).statusCode).toBe(400);
  expect((await send({ ...body, chainId: String(owner.wallet.chain_id + 1) })).statusCode).toBe(
    403
  );
  const other = await caller();
  expect((await send(body, other.token)).statusCode).toBe(403);
  expect(requests).toEqual([]);

  for (const status of [429, 422]) {
    quoteReply = { status, body: { error: 'budget_exhausted' } };
    const response = await send(body);
    expect(response.statusCode, response.body).toBe(429);
    expect(response.headers['cache-control']).toBe('no-store');
    expect(response.json()).toStrictEqual({
      error: 'budget_exhausted',
      detail: 'budget_exhausted',
    });
  }
  expect(requests).toStrictEqual([
    { path: '/station/quote', body },
    { path: '/station/quote', body },
  ]);
  quoteReply = { status: 429, body: { error: 'rate_limited' } };
  const limited = await send(body);
  expect(limited.statusCode).toBe(429);
  expect(limited.json()).toStrictEqual({ error: 'rate_limited' });

  const answer = {
    quote: {
      sponsor,
      token: body.token,
      maxTokenAmount: body.maxTokenAmount,
      tokenAmount: '100',
      deadline: '2000000000',
      quoteNonce: '8',
      gasCost: body.gasCost,
      decimals: 6,
    },
    relayerSignature,
  };
  quoteReply = { status: 200, body: answer };
  const accepted = await send(body);
  expect(accepted.statusCode, accepted.body).toBe(200);
  expect(accepted.json()).toStrictEqual(answer);
  expect(requests).toHaveLength(4);
  expect(requests[3]).toStrictEqual({ path: '/station/quote', body });
  expect(endpointErrors).toEqual([]);
}

async function sponsoredStatusReportsDidBudget(): Promise<void> {
  const owner = await caller();
  const body = {
    account: owner.wallet.address,
    sponsor,
    quoteNonce: '8',
    relayerSignature,
  };
  const budget = {
    key: owner.did,
    gasLimit: '5000000',
    usedGas: '3000000',
    remainingGas: '2000000',
    dayEnds: '2000073600',
  };
  const report = {
    state: 'quoted',
    deadline: '2000000000',
    submission: null,
    replacement: null,
    completion: null,
    budget,
  };
  statusReply = report;
  requests.length = 0;
  const send = (payload: unknown = body, token: string | null = owner.token) =>
    app.inject({
      method: 'POST',
      url: '/v1/wallet/sponsored/status',
      headers: token === null ? {} : { authorization: `Bearer ${token}` },
      payload,
    });
  expect((await send(body, null)).statusCode).toBe(401);
  const other = await caller();
  expect((await send(body, other.token)).statusCode).toBe(403);
  expect(requests).toEqual([]);
  const response = await send();
  expect(response.statusCode, response.body).toBe(200);
  expect(response.headers['cache-control']).toBe('no-store');
  expect(response.json()).toStrictEqual({
    tx_hash: null,
    status: 'pending',
    station: report,
    did: owner.did,
    budget,
  });
  expect(response.json().budget.remainingGas).toBe('2000000');
  expect(requests).toStrictEqual([{ path: '/station/status', body }]);

  const updatedBudget = {
    ...budget,
    usedGas: '4000000',
    remainingGas: '1000000',
  };
  statusReply = { ...report, budget: updatedBudget };
  const updated = await send();
  expect(updated.statusCode, updated.body).toBe(200);
  expect(updated.json().budget).toStrictEqual(updatedBudget);
  for (const invalidBudget of [
    { ...budget, key: other.did },
    { ...budget, gasLimit: '-1' },
    { ...budget, usedGas: 3000000 },
    { ...budget, remainingGas: undefined },
    { ...budget, extra: 'unexpected' },
    { ...budget, remainingGas: '1000000' },
  ]) {
    statusReply = { ...report, budget: invalidBudget };
    const invalid = await send();
    expect(invalid.statusCode, invalid.body).toBe(502);
    expect(invalid.json()).toStrictEqual({
      error: 'gas_station_invalid_response',
      status: 'pending',
    });
  }
  expect(requests).toHaveLength(8);
  expect(endpointErrors).toEqual([]);
}

test(sponsoredQuoteAnswersBudgetExhausted, sponsoredQuoteAnswersBudgetExhausted);
test(sponsoredStatusReportsDidBudget, sponsoredStatusReportsDidBudget);
