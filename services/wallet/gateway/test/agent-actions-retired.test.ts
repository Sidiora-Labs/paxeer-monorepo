import { randomBytes } from 'node:crypto';
import { readFileSync } from 'node:fs';
import type { FastifyInstance } from 'fastify';
import ts from 'typescript';
import { afterAll, beforeAll, expect, test } from 'vitest';
import { newAgentKey } from './support/identity.js';
import { startPostgres, type EphemeralPostgres } from './support/postgres.js';

const buildLayerxDepositPlan = 'buildLayerxDepositPlan';
const didClaimFor = 'didClaimFor';

let pg: EphemeralPostgres;
let app: FastifyInstance;
let pool: typeof import('../src/db/pool.js');

beforeAll(async () => {
  process.env.AGENT_JWT_SECRET = randomBytes(32).toString('hex');
  process.env.LOG_LEVEL = 'error';
  pg = await startPostgres();
  pool = await import('../src/db/pool.js');
  const { buildApp } = await import('../src/index.js');
  app = await buildApp();
  await app.ready();
}, 120_000);

afterAll(async () => {
  await app?.close();
  await pool?.closePool();
  await pg?.stop();
}, 120_000);

async function layerxDepositRouteIsNotServed(): Promise<void> {
  const plans = await import('../src/agent/actions/plan.js');
  expect(Object.getOwnPropertyDescriptor(plans, buildLayerxDepositPlan)).toBeUndefined();
  expect(Object.getOwnPropertyDescriptor(plans, didClaimFor)).toBeUndefined();

  const { env } = await import('../src/env.js');
  for (const key of ['LAYERX_VAULT_ADDRESS', 'LAYERX_USDL_ADDRESS', 'LAYERX_USDL_DECIMALS']) {
    expect(env).not.toHaveProperty(key);
  }

  const agent = newAgentKey('retired-deposit');
  const { upsertPrincipalOnVerify, setPrincipalFrozen } = await import('../src/db/agents.js');
  await upsertPrincipalOnVerify({
    did: agent.did,
    label: 'retired-deposit',
    keyFingerprint: agent.publicKeyHex.slice(0, 16),
    publicKey: agent.publicKeyHex,
  });
  await setPrincipalFrozen(agent.did, false);

  const url = '/v1/agent/actions/layerx/deposit';
  const body = JSON.stringify({
    asset: 'USDL',
    amount: '250',
    did_claim: '0x' + '11'.repeat(32),
    idempotency_key: 'retired-layerx-deposit',
  });
  const nonce = randomBytes(16);
  const expiry = BigInt(Math.floor(Date.now() / 1000) + 60);
  const { agentRequestDigest } = await import('../src/agent/verify.js');
  const signature = agent.sign(
    agentRequestDigest({
      method: `POST ${url}`,
      keyId: agent.did,
      nonce,
      expiry,
      body: Buffer.from(body),
    })
  );
  const signedHeaders = {
    'x-agent-key': agent.did,
    'x-agent-nonce': nonce.toString('hex'),
    'x-agent-expires': expiry.toString(),
    'x-agent-signature': signature,
  };

  const before = await pool.query<{ count: string }>(
    'select count(*)::text as count from agent_actions'
  );
  expect(before.rows[0]?.count).toBe('0');
  for (const headers of [{}, signedHeaders, signedHeaders]) {
    const response = await app.inject({
      method: 'POST',
      url,
      headers: { 'content-type': 'application/json', ...headers },
      payload: body,
    });
    expect(response.statusCode).toBe(404);
    expect(response.json()).toMatchObject({ statusCode: 404, error: 'Not Found' });
    const after = await pool.query<{ count: string }>(
      'select count(*)::text as count from agent_actions'
    );
    expect(after.rows[0]?.count).toBe('0');
  }

  const allowance = await app.inject({
    method: 'POST',
    url: '/v1/agent/actions/allowance-and-call',
    payload: {},
  });
  expect(allowance.statusCode).toBe(401);
  expect(allowance.json()).toMatchObject({ error: 'agent_signature_required' });
}

test(layerxDepositRouteIsNotServed);

async function actionKindsExcludeLayerxDeposit(): Promise<void> {
  const actions = await import('../src/db/actions.js');
  expect(actions.retainActionCustodyAuthorization).toBeTypeOf('function');

  const source = ts.createSourceFile(
    'actions.ts',
    readFileSync(new URL('../src/db/actions.ts', import.meta.url), 'utf8'),
    ts.ScriptTarget.Latest,
    true,
    ts.ScriptKind.TS
  );
  const kind = source.statements.find(
    (node): node is ts.TypeAliasDeclaration =>
      ts.isTypeAliasDeclaration(node) && node.name.text === 'ActionKind'
  );
  expect(kind).toBeDefined();
  expect(kind?.type).toMatchObject({
    kind: ts.SyntaxKind.LiteralType,
    literal: { kind: ts.SyntaxKind.StringLiteral, text: 'allowance_and_call' },
  });

  const agent = newAgentKey('allowance-action-store');
  const { upsertPrincipalOnVerify } = await import('../src/db/agents.js');
  await upsertPrincipalOnVerify({
    did: agent.did,
    label: 'allowance-action-store',
    keyFingerprint: agent.publicKeyHex.slice(0, 16),
    publicKey: agent.publicKeyHex,
  });

  try {
    const request = { spender: '0x' + '11'.repeat(20), amount: '250' };
    const args: Parameters<typeof actions.createOrGetAction>[0] = {
      did: agent.did,
      walletId: null,
      walletAddress: null,
      kind: 'allowance_and_call',
      idempotencyKey: 'allowance-action-store',
      request,
    };
    const created = await actions.createOrGetAction(args);
    expect(created.created).toBe(true);
    expect(created.row).toMatchObject({
      did: agent.did,
      kind: 'allowance_and_call',
      status: 'received',
      phase: 'intake',
      request,
    });
    expect(await actions.getAction(created.row.id)).toEqual(created.row);

    const replay = await actions.createOrGetAction(args);
    expect(replay.created).toBe(false);
    expect(replay.row).toEqual(created.row);

    const stored = await pool.query<{ kind: string }>(
      'select kind from agent_actions where did = $1',
      [agent.did]
    );
    expect(stored.rows).toEqual([{ kind: 'allowance_and_call' }]);
  } finally {
    await pool.query('delete from agent_principals where did = $1', [agent.did]);
  }
}

test(actionKindsExcludeLayerxDeposit);
