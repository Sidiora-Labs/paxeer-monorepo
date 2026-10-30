import Fastify, { type FastifyInstance } from 'fastify';
import { randomBytes } from 'node:crypto';
import { afterAll, beforeAll, expect, test } from 'vitest';
import { newAgentKey } from './support/identity.js';
import { startPostgres, type EphemeralPostgres } from './support/postgres.js';

let pg: EphemeralPostgres;
let app: FastifyInstance;
let pool: typeof import('../src/db/pool.js');
let precompiles: typeof import('../src/precompiles.js');
let authorization: string;

beforeAll(async () => {
  process.env.AGENT_JWT_SECRET = randomBytes(32).toString('hex');
  pg = await startPostgres();
  pool = await import('../src/db/pool.js');
  precompiles = await import('../src/precompiles.js');
  const { upsertPrincipalOnVerify } = await import('../src/db/agents.js');
  const { mintAgentToken } = await import('../src/auth/agentToken.js');
  const { agentPrecompileRoutes } = await import('../src/routes/agentPrecompiles.js');
  const agent = newAgentKey('precompile-list');
  await upsertPrincipalOnVerify({
    did: agent.did,
    label: 'Precompile list',
    keyFingerprint: agent.did,
    publicKey: agent.publicKeyHex,
  });
  const { token } = await mintAgentToken({ did: agent.did, ownerUserId: null });
  authorization = `Bearer ${token}`;
  app = Fastify();
  await app.register(agentPrecompileRoutes);
  await app.ready();
}, 120_000);

afterAll(async () => {
  await app?.close();
  await pool?.closePool();
  await pg?.stop();
}, 120_000);

async function precompileListReportsServedTable(): Promise<void> {
  const UNSERVED_PRECOMPILES = precompiles.UNSERVED_PRECOMPILES;
  const { PRECOMPILE_ADDRESSES, TEE_FAMILIES } = precompiles;
  const routes = {
    orob: [],
    clearing: [],
    oracle: [],
    pofq: [],
    scheduler: [
      ['POST', 'scheduler/schedule'],
      ['POST', 'scheduler/cancel'],
      ['POST', 'scheduler/reschedule'],
      ['GET', 'scheduler/job/:id'],
      ['GET', 'scheduler/pending'],
    ],
    streams: [
      ['POST', 'streams/open'],
      ['POST', 'streams/settle'],
      ['POST', 'streams/close'],
      ['POST', 'streams/update-rate'],
      ['GET', 'streams/stream/:id'],
      ['GET', 'streams/accrued/:id'],
    ],
    teeAttestor: [
      ['POST', 'tee/verify'],
      ['POST', 'tee/root-count'],
      ['POST', 'tee/root-of'],
    ],
    eip712: [
      ['POST', 'eip712/domain-separator'],
      ['POST', 'eip712/hash-typed-data'],
      ['POST', 'eip712/recover'],
    ],
    staking: [],
    bech32: [],
    p256: [],
  } as const satisfies Record<
    keyof typeof PRECOMPILE_ADDRESSES,
    readonly (readonly ['GET' | 'POST', string])[]
  >;
  const keys = Object.keys(PRECOMPILE_ADDRESSES) as (keyof typeof PRECOMPILE_ADDRESSES)[];
  const expectedServed = Object.fromEntries(
    keys
      .filter((key) => !UNSERVED_PRECOMPILES.has(key))
      .map((key) => [key, routes[key].map(([, name]) => `/v1/agent/precompiles/${name}`)])
  );

  const unauthenticated = await app.inject({ method: 'GET', url: '/v1/agent/precompiles' });
  expect(unauthenticated.statusCode).toBe(401);
  const response = await app.inject({
    method: 'GET',
    url: '/v1/agent/precompiles',
    headers: { authorization },
  });
  expect(response.statusCode).toBe(200);
  expect(response.json()).toEqual({
    addresses: PRECOMPILE_ADDRESSES,
    served: expectedServed,
    tee_families: TEE_FAMILIES,
  });
  for (const key of keys) {
    for (const [method, name] of routes[key]) {
      expect(app.hasRoute({ method, url: `/v1/agent/precompiles/${name}` }), name).toBe(true);
    }
  }
}

test(precompileListReportsServedTable, precompileListReportsServedTable);
