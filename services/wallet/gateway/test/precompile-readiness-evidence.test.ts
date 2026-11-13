import { randomBytes } from 'node:crypto';
import { createServer, type Server } from 'node:http';
import type { AddressInfo } from 'node:net';
import Fastify, { type FastifyInstance } from 'fastify';
import { Pool } from 'pg';
import { decodeFunctionData, encodeFunctionResult, zeroAddress, type Hex } from 'viem';
import { afterAll, beforeAll, expect, test } from 'vitest';
import { PGlite, pgcrypto, PGLiteSocketServer } from '../src/db/testDatabase.js';

type ReadOutcome = 'success' | 'missing' | 'rpc-error' | 'malformed' | 'transport-error';
type IdentityOutcome = 'success' | 'empty' | 'malformed' | 'http-error';
let outcome: ReadOutcome = 'success';
let identityOutcome: IdentityOutcome = 'success';
let db: PGlite;
let socket: PGLiteSocketServer;
let server: Server;
let app: FastifyInstance;
let baseUrl: string;
let authorization: string;
let walletAddress: Hex;
let database: typeof import('../src/db/pool.js');
let configuration: typeof import('../src/env.js');
let precompiles: typeof import('../src/precompiles.js');
let readiness: typeof import('../src/routes/readiness.js');
let rpcModule: typeof import('../src/rpc/pool.js');
let attestorModule: typeof import('../src/attestor/client.js');
const fixtureErrors: unknown[] = [];
const calls: Array<{
  method: string;
  address?: string;
  functionName?: string;
  args?: readonly unknown[];
}> = [];
const adminToken = randomBytes(32).toString('hex');
const privateError = 'fixture-database-password-must-stay-private';
const urlUser = 'fixture-private-user';
const urlPassword = 'fixture-private-password';

function rpcResult(call: { method: string; params?: Array<{ to: string; data: Hex }> }): unknown {
  if (call.method === 'eth_chainId') return '0x7d';
  if (call.method === 'eth_blockNumber') return '0x64';
  if (call.method !== 'eth_call' || !call.params?.[0])
    throw new Error(`unexpected RPC ${call.method}`);
  const request = call.params[0];
  if (request.data === '0xffffffff') {
    calls.push({ method: call.method, address: request.to, functionName: 'capability-probe' });
    throw new Error('LXP_PRECOMPILE_UNKNOWN_METHOD');
  }
  if (request.to.toLowerCase() === precompiles.PRECOMPILE_ADDRESSES.scheduler) {
    const decoded = decodeFunctionData({ abi: precompiles.SCHEDULER_ABI, data: request.data });
    calls.push({ method: call.method, address: request.to, ...decoded });
    if (outcome === 'rpc-error') throw new Error('scheduler temporarily unavailable');
    if (outcome === 'malformed') return '0x1234';
    if (decoded.functionName === 'pending') {
      return encodeFunctionResult({
        abi: precompiles.SCHEDULER_ABI,
        functionName: 'pending',
        result: outcome === 'missing' ? [] : [11n, 29n],
      });
    }
    if (decoded.functionName === 'getJob') {
      return encodeFunctionResult({
        abi: precompiles.SCHEDULER_ABI,
        functionName: 'getJob',
        result: [
          outcome === 'missing' ? 0n : 11n,
          walletAddress,
          zeroAddress,
          '0x',
          100n,
          21000n,
          2n,
          outcome !== 'missing',
        ],
      });
    }
  } else if (request.to.toLowerCase() === precompiles.PRECOMPILE_ADDRESSES.streams) {
    const decoded = decodeFunctionData({ abi: precompiles.STREAMS_ABI, data: request.data });
    calls.push({ method: call.method, address: request.to, ...decoded });
    if (outcome === 'rpc-error') throw new Error('streams temporarily unavailable');
    if (outcome === 'malformed') return '0x1234';
    if (decoded.functionName === 'getStream') {
      return encodeFunctionResult({
        abi: precompiles.STREAMS_ABI,
        functionName: 'getStream',
        result: {
          id: outcome === 'missing' ? 0n : 11n,
          payer: walletAddress,
          payee: zeroAddress,
          token: zeroAddress,
          ratePerSecond: 2n,
          cap: 100n,
          startTime: 1n,
          stopTime: 50n,
          settled: 4n,
          active: outcome !== 'missing',
        },
      });
    }
    if (decoded.functionName === 'accrued') {
      return encodeFunctionResult({
        abi: precompiles.STREAMS_ABI,
        functionName: 'accrued',
        result: outcome === 'missing' ? 0n : 12n,
      });
    }
  }
  throw new Error(`unexpected precompile call ${request.to} ${request.data}`);
}

beforeAll(async () => {
  db = await PGlite.create({ extensions: { pgcrypto } });
  socket = new PGLiteSocketServer({ db, port: 0, host: '127.0.0.1', maxConnections: 10 });
  await socket.start();
  process.env.DATABASE_URL = `postgres://postgres:postgres@${socket.getServerConn()}/postgres`;
  process.env.AGENT_JWT_SECRET = randomBytes(32).toString('hex');
  process.env.GATEWAY_ADMIN_TOKEN = adminToken;
  server = createServer(async (req, res) => {
    try {
      if (req.method === 'GET') {
        res.setHeader('content-type', 'application/json');
        if (identityOutcome === 'http-error') {
          res.writeHead(503);
          res.end(privateError);
        } else if (identityOutcome === 'malformed') {
          res.end(privateError);
        } else {
          res.end(
            JSON.stringify({
              keys: identityOutcome === 'empty' ? [] : [{ kty: 'RSA', kid: 'fixture' }],
            })
          );
        }
        return;
      }
      const chunks: Buffer[] = [];
      for await (const chunk of req) chunks.push(Buffer.from(chunk));
      const body = JSON.parse(Buffer.concat(chunks).toString('utf8'));
      if (outcome === 'transport-error') {
        res.writeHead(503);
        res.end('fixture RPC transport unavailable');
        return;
      }
      const respond = (call: {
        id: number;
        method: string;
        params?: Array<{ to: string; data: Hex }>;
      }) => {
        try {
          return { jsonrpc: '2.0', id: call.id, result: rpcResult(call) };
        } catch (error) {
          if (
            !(error instanceof Error) ||
            !/LXP_PRECOMPILE_UNKNOWN_METHOD|temporarily unavailable/.test(error.message)
          ) {
            fixtureErrors.push(error);
          }
          return { jsonrpc: '2.0', id: call.id, error: { code: -32000, message: String(error) } };
        }
      };
      res.setHeader('content-type', 'application/json');
      res.end(JSON.stringify(Array.isArray(body) ? body.map(respond) : respond(body)));
    } catch (error) {
      fixtureErrors.push(error);
      res.writeHead(500);
      res.end();
    }
  });
  await new Promise<void>((resolve, reject) => {
    server.once('error', reject);
    server.listen(0, '127.0.0.1', resolve);
  });
  baseUrl = `http://127.0.0.1:${(server.address() as AddressInfo).port}`;
  process.env.HYPERPAXEER_RPC_URL = baseUrl;
  configuration = await import('../src/env.js');
  database = await import('../src/db/pool.js');
  await (await import('../src/db/migrate.js')).runMigrations();
  const agents = await import('../src/db/agents.js');
  const wallets = await import('../src/db/wallets.js');
  const did = 'did:matrix:repair-read-evidence';
  await agents.upsertPrincipalOnVerify({
    did,
    label: 'read-evidence',
    publicKey: '12'.repeat(32),
    keyFingerprint: '12'.repeat(8),
  });
  const { wallet } = await wallets.provisionWalletForUser(wallets.agentWalletUserId(did), 'agent');
  walletAddress = wallet.address;
  await agents.setPrincipalWallet(did, wallet.id);
  authorization = `Bearer ${(await (await import('../src/auth/agentToken.js')).mintAgentToken({ did, ownerUserId: null })).token}`;
  precompiles = await import('../src/precompiles.js');
  readiness = await import('../src/routes/readiness.js');
  rpcModule = await import('../src/rpc/pool.js');
  attestorModule = await import('../src/attestor/client.js');
  app = Fastify();
  await app.register((await import('../src/routes/agentPrecompiles.js')).agentPrecompileRoutes);
  await app.ready();
}, 30_000);

afterAll(async () => {
  await app?.close();
  await database?.closePool();
  await socket?.stop();
  await db?.close();
  if (server) {
    server.closeAllConnections();
    await new Promise<void>((resolve) => server.close(() => resolve()));
  }
  expect(fixtureErrors).toEqual([]);
});

function read(path: string) {
  return app.inject({
    method: 'GET',
    url: `/v1/agent/precompiles/${path}`,
    headers: { authorization },
  });
}

async function precompileCapabilityOutageIsReadFailed() {
  outcome = 'transport-error';
  for (const path of ['scheduler/pending', 'streams/stream/11']) {
    const reply = await read(path);
    expect(reply.statusCode, reply.body).toBe(502);
    expect(reply.json()).toEqual({ error: 'read_failed' });
  }
}

async function schedulerReadsPreserveProducerEvidence() {
  outcome = 'success';
  const populated = await read('scheduler/pending');
  expect(populated.statusCode, populated.body).toBe(200);
  expect(populated.json()).toEqual({ creator: walletAddress, job_ids: ['11', '29'] });
  expect(calls).toContainEqual({
    method: 'eth_call',
    address: precompiles.PRECOMPILE_ADDRESSES.scheduler,
    functionName: 'pending',
    args: [walletAddress],
  });
  const job = await read('scheduler/job/11');
  expect(job.statusCode, job.body).toBe(200);
  expect(job.json().job).toMatchObject({ id: '11', creator: walletAddress, active: true });
  outcome = 'missing';
  const empty = await read('scheduler/pending');
  expect(empty.statusCode, empty.body).toBe(200);
  expect(empty.json()).toEqual({ creator: walletAddress, job_ids: [] });
  const missing = await read('scheduler/job/11');
  expect(missing.statusCode).toBe(404);
  expect(missing.json()).toEqual({ error: 'not_found' });
  for (const failure of ['rpc-error', 'malformed', 'transport-error'] as const) {
    outcome = failure;
    for (const path of ['scheduler/pending', 'scheduler/job/11']) {
      const reply = await read(path);
      expect(reply.statusCode, `${failure}: ${reply.body}`).toBe(502);
      expect(reply.json()).toEqual({ error: 'read_failed' });
    }
  }
}

async function streamReadsPreserveProducerEvidence() {
  outcome = 'success';
  const found = await read('streams/stream/11');
  expect(found.statusCode, found.body).toBe(200);
  expect(found.json()).toEqual({
    stream: {
      id: '11',
      payer: walletAddress,
      payee: zeroAddress,
      token: zeroAddress,
      ratePerSecond: '2',
      cap: '100',
      startTime: '1',
      stopTime: '50',
      settled: '4',
      active: true,
    },
  });
  expect(calls).toContainEqual({
    method: 'eth_call',
    address: precompiles.PRECOMPILE_ADDRESSES.streams,
    functionName: 'getStream',
    args: [11n],
  });
  const accrued = await read('streams/accrued/11');
  expect(accrued.statusCode).toBe(200);
  expect(accrued.json()).toEqual({ stream_id: '11', accrued: '12' });
  outcome = 'missing';
  const missing = await read('streams/stream/11');
  expect(missing.statusCode).toBe(404);
  expect(missing.json()).toEqual({ error: 'not_found' });
  const zero = await read('streams/accrued/11');
  expect(zero.statusCode).toBe(200);
  expect(zero.json()).toEqual({ stream_id: '11', accrued: '0' });
  for (const failure of ['rpc-error', 'malformed', 'transport-error'] as const) {
    outcome = failure;
    for (const path of ['streams/stream/11', 'streams/accrued/11']) {
      const reply = await read(path);
      expect(reply.statusCode, `${failure}: ${reply.body}`).toBe(502);
      expect(reply.json()).toEqual({ error: 'read_failed' });
    }
  }
}

function rpcPool(url: string) {
  return new rpcModule.RpcPool({
    urls: [url],
    chainId: 125,
    lagThresholdBlocks: 20,
    timeoutMs: 500,
    healthIntervalMs: 60_000,
  });
}

function assertNoSecrets(body: string) {
  for (const secret of [
    urlUser,
    urlPassword,
    privateError,
    'private-path-token',
    'private-query-token',
    adminToken,
  ]) {
    expect(body).not.toContain(secret);
  }
}

async function readinessPublicAndOperatorDisclosureBoundary() {
  outcome = 'success';
  const credentialUrl = `${baseUrl.replace('://', `://${urlUser}:${urlPassword}@`)}/private-path-token?key=private-query-token`;
  const rpc = rpcPool(credentialUrl);
  const attestors = new attestorModule.AttestorClient({
    endpoints: Array.from({ length: 5 }, (_, n) => `${credentialUrl}&node=${n}`),
    quorum: 3,
    tls: { cert: '', key: '', ca: '' },
    timeoutMs: 500,
    healthIntervalMs: 60_000,
  });
  const instance = Fastify();
  const unavailablePool = new Pool({ connectionString: configuration.env.DATABASE_URL, max: 1 });
  await unavailablePool.query('select 1 from nonce_allocations limit 1');
  await unavailablePool.end();
  await expect(unavailablePool.query('select 1 from nonce_allocations limit 1')).rejects.toThrow(
    'Cannot use a pool after calling end on the pool'
  );
  const options = {
    pool: unavailablePool,
    rpc,
    attestors,
    jwksUrl: credentialUrl,
    timeoutMs: 1000,
  };
  try {
    await instance.register(readiness.readinessRoutes, options);
    const reply = await instance.inject('/readyz');
    expect(reply.statusCode, reply.body).toBe(503);
    expect(reply.headers['cache-control']).toBe('no-store');
    expect(reply.json()).toEqual({
      error: 'not_ready',
      ready: false,
      components: {
        attestors: { state: 'down', reason: 'attestor_quorum_unavailable' },
        nonce_store: { state: 'down', reason: 'nonce_store_unavailable' },
        rpc_pool: { state: 'down', reason: 'rpc_pool_unavailable' },
        identity_provider: { state: 'down', reason: 'jwks_unavailable' },
      },
    });
    assertNoSecrets(reply.body);
    expect(reply.body).not.toContain('127.0.0.1');
    for (const token of [undefined, 'wrong-token']) {
      const denied = await instance.inject({
        url: '/v1/admin/readiness',
        headers: token ? { authorization: `Bearer ${token}` } : {},
      });
      expect(denied.statusCode).toBe(401);
      expect(denied.json()).toEqual({ error: 'unauthorized' });
      expect(denied.headers['cache-control']).toBe('no-store');
    }
    const operator = await instance.inject({
      url: '/v1/admin/readiness',
      headers: { authorization: `Bearer ${adminToken}` },
    });
    expect(operator.statusCode, operator.body).toBe(503);
    const report = operator.json();
    expect(report.components.attestors.nodes).toHaveLength(5);
    for (const node of report.components.attestors.nodes) {
      expect(node).toEqual({
        endpoint: baseUrl,
        node_id: null,
        healthy: false,
        error: 'attestor_health_failed',
      });
    }
    expect(report.components.rpc_pool.endpoints).toEqual([
      { url: baseUrl, state: 'down', head: null, error: 'rpc_endpoint_unavailable' },
    ]);
    assertNoSecrets(operator.body);
    const checked = await readiness.checkReadiness(options);
    assertNoSecrets(JSON.stringify(checked));
    const configuredToken = configuration.env.GATEWAY_ADMIN_TOKEN;
    try {
      configuration.env.GATEWAY_ADMIN_TOKEN = undefined;
      const disabled = await instance.inject({
        url: '/v1/admin/readiness',
        headers: { authorization: `Bearer ${adminToken}` },
      });
      expect(disabled.statusCode).toBe(401);
      expect(disabled.json()).toEqual({ error: 'unauthorized' });
    } finally {
      configuration.env.GATEWAY_ADMIN_TOKEN = configuredToken;
    }
  } finally {
    await instance.close();
    rpc.stop();
    attestors.stop();
  }
}

async function readinessUsesStableIdentityReasonCodes() {
  outcome = 'success';
  const rpc = rpcPool(baseUrl);
  const instance = Fastify();
  try {
    await instance.register(readiness.readinessRoutes, {
      attestors: null,
      pool: database.getPool(),
      rpc,
      jwksUrl: `${baseUrl}/jwks`,
      timeoutMs: 1000,
    });
    for (const [state, reason] of [
      ['success', null],
      ['empty', 'jwks_empty'],
      ['malformed', 'jwks_unavailable'],
      ['http-error', 'jwks_http_error'],
    ] as const) {
      identityOutcome = state;
      const reply = await instance.inject('/readyz');
      expect(reply.statusCode).toBe(503);
      expect(reply.json().components).toEqual({
        attestors: { state: 'down', reason: 'attestor_unconfigured' },
        nonce_store: { state: 'up', reason: null },
        rpc_pool: { state: 'up', reason: null },
        identity_provider: { state: state === 'success' ? 'up' : 'down', reason },
      });
      assertNoSecrets(reply.body);
      expect(reply.body).not.toContain(baseUrl);
    }
  } finally {
    identityOutcome = 'success';
    await instance.close();
    rpc.stop();
  }
}

test(precompileCapabilityOutageIsReadFailed.name, precompileCapabilityOutageIsReadFailed, 60_000);
test(schedulerReadsPreserveProducerEvidence.name, schedulerReadsPreserveProducerEvidence, 60_000);
test(streamReadsPreserveProducerEvidence.name, streamReadsPreserveProducerEvidence, 60_000);
test(
  readinessPublicAndOperatorDisclosureBoundary.name,
  readinessPublicAndOperatorDisclosureBoundary
);
test(readinessUsesStableIdentityReasonCodes.name, readinessUsesStableIdentityReasonCodes);
