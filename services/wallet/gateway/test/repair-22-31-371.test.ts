import { randomBytes, randomUUID } from 'node:crypto';
import { readFileSync, readdirSync } from 'node:fs';
import { createServer, type Server } from 'node:http';
import type { AddressInfo } from 'node:net';
import Fastify, { type FastifyInstance } from 'fastify';
import { Pool } from 'pg';
import {
  decodeFunctionData,
  encodeFunctionData,
  erc20Abi,
  keccak256,
  parseTransaction,
  recoverTransactionAddress,
  type Abi,
  type Hex,
} from 'viem';
import { afterAll, beforeAll, expect, test } from 'vitest';
import { newAgentKey, startIdentityProvider, type AgentKey, type IdentityProvider } from './support/identity.js';
import { startPostgres, type EphemeralPostgres } from './support/postgres.js';

const recipient = '0x00000000000000000000000000000000000000a1';
const token = '0x00000000000000000000000000000000000000b2';
const blocked = '0x00000000000000000000000000000000000000c3';
const broadcasts: Hex[] = [];
const simulations: unknown[] = [];
const rpcErrors: unknown[] = [];
const apps: FastifyInstance[] = [];
const restrictedPools: Pool[] = [];
let postgres: EphemeralPostgres;
let identity: IdentityProvider;
let server: Server;
let database: typeof import('../src/db/pool.js');
let wallets: typeof import('../src/db/wallets.js');
let agents: typeof import('../src/db/agents.js');
let policy: typeof import('../src/policy/agent.js');
let humanPolicy: typeof import('../src/policy/index.js');
let precompiles: typeof import('../src/precompiles.js');
let requests: typeof import('../src/agent/verify.js');
let rpc: typeof import('../src/rpc/pool.js');
let app: FastifyInstance;

async function mount(pool?: Pool): Promise<FastifyInstance> {
  const instance = Fastify();
  apps.push(instance);
  requests.installRawBodyParser(instance);
  const { signRoutes } = await import('../src/routes/sign.js');
  const { agentRoutes } = await import('../src/routes/agent.js');
  const { agentPrecompileRoutes } = await import('../src/routes/agentPrecompiles.js');
  await signRoutes(instance, { pool, attestors: null });
  await agentRoutes(instance);
  await agentPrecompileRoutes(instance);
  await instance.ready();
  return instance;
}

beforeAll(async () => {
  // Real PostgreSQL, private to this test, with the repository's actual migrations.
  const versions = readdirSync('/usr/lib/postgresql').filter((v) => /^\d+$/.test(v));
  versions.sort((a, b) => Number(b) - Number(a));
  if (!versions[0]) throw new Error('PostgreSQL installation is required for the real reservation proof');
  process.env.PG_BIN_DIR = `/usr/lib/postgresql/${versions[0]}/bin`;
  identity = await startIdentityProvider();
  process.env.SUPABASE_URL = identity.url;
  process.env.AGENT_JWT_SECRET = randomBytes(32).toString('hex');
  process.env.POLICY_MAX_TX_VALUE_WEI = '100';
  process.env.POLICY_MAX_DAILY_VALUE_WEI = '100';
  process.env.POLICY_RATE_LIMIT_PER_MINUTE = '1000';
  process.env.RATE_LIMIT_CLIENT_PER_MINUTE = '1000';
  process.env.RATE_LIMIT_ACCOUNT_PER_MINUTE = '1000';
  process.env.LOG_LEVEL = 'error';

  // HTTP fault boundary: real RpcPool/viem clients send their actual requests.
  // A send loses its response after capturing the real envelope; no transaction
  // hash, signature, provider, database or policy function is substituted.
  server = createServer((req, res) => {
    const chunks: Buffer[] = [];
    req.on('data', (chunk: Buffer) => chunks.push(chunk));
    req.on('end', () => {
      try {
        const body = JSON.parse(Buffer.concat(chunks).toString('utf8'));
        const answer = (call: { id: number; method: string; params: any[] }) => {
          const base = { jsonrpc: '2.0', id: call.id };
          switch (call.method) {
            case 'eth_chainId': return { ...base, result: '0x7d' };
            case 'eth_blockNumber': return { ...base, result: '0x64' };
            case 'eth_getTransactionCount': return { ...base, result: '0x0' };
            case 'eth_getTransactionByHash':
              return { ...base, error: { code: -32002, message: 'transaction visibility unavailable' } };
            case 'eth_call':
              if (call.params[0].data === '0xffffffff') {
                return { ...base, error: { code: -32000, message: 'LXP_PRECOMPILE_UNKNOWN_METHOD' } };
              }
              simulations.push(call.params[0]);
              return { ...base, result: '0x' };
            case 'eth_sendRawTransaction':
              broadcasts.push(call.params[0] as Hex);
              return null;
            default: throw new Error(`unexpected RPC method ${call.method}`);
          }
        };
        const response = Array.isArray(body) ? body.map(answer) : answer(body);
        if (response === null) {
          res.writeHead(503);
          res.end('response lost after submission');
          return;
        }
        res.writeHead(200, { 'content-type': 'application/json' });
        res.end(JSON.stringify(response));
      } catch (error) {
        rpcErrors.push(error);
        res.writeHead(500);
        res.end();
      }
    });
  });
  await new Promise<void>((resolve) => server.listen(0, '127.0.0.1', resolve));
  const url = `http://127.0.0.1:${(server.address() as AddressInfo).port}`;
  process.env.HYPERPAXEER_RPC_URL = url;
  process.env.RPC_URLS = url;
  process.env.SEQUENCER_RPC_URL = url;
  postgres = await startPostgres();
  database = await import('../src/db/pool.js');
  wallets = await import('../src/db/wallets.js');
  agents = await import('../src/db/agents.js');
  policy = await import('../src/policy/agent.js');
  humanPolicy = await import('../src/policy/index.js');
  precompiles = await import('../src/precompiles.js');
  requests = await import('../src/agent/verify.js');
  rpc = await import('../src/rpc/pool.js');
  app = await mount();
}, 120_000);

afterAll(async () => {
  for (const instance of apps) await instance.close();
  for (const pool of restrictedPools) await pool.end();
  rpc?.sharedRpcPool().stop();
  await database?.closePool();
  await postgres?.stop();
  await identity?.stop();
  if (server) await new Promise<void>((resolve) => server.close(() => resolve()));
  expect(rpcErrors).toEqual([]);
}, 120_000);

function transaction(value: string) {
  return { to: recipient, value, gas: '21000', maxFeePerGas: '2000000000', maxPriorityFeePerGas: '1000000000' };
}

async function humanRequest(instance: FastifyInstance, user: string, route: 'sign' | 'send', value: string) {
  return instance.inject({
    method: 'POST', url: `/v1/wallet/${route}`,
    headers: { authorization: `Bearer ${await identity.mintUserToken(user)}` },
    payload: { tx: transaction(value) },
  });
}

async function humanDailyCapParallelLegacySigning() {
  const user = randomUUID();
  const { row } = await wallets.provisionWalletForUser(user);
  expect(row.encrypted_private_key).toEqual(expect.any(String));
  const custody = await database.query('select migrated_at from wallets where id=$1', [row.id]);
  expect(custody.rows).toEqual([{ migrated_at: null }]);
  const otherInstance = await mount();
  const replies = await Promise.all(Array.from({ length: 8 }, (_, n) =>
    humanRequest(n % 2 === 0 ? app : otherInstance, user, 'sign', '50')));
  const signed = replies.filter((reply) => reply.statusCode === 200);
  expect(signed.map((reply) => reply.json()), JSON.stringify(replies.map((reply) => reply.json()))).toHaveLength(2);
  expect(replies.filter((reply) => reply.statusCode === 403)).toHaveLength(6);
  for (const reply of replies.filter((r) => r.statusCode !== 200)) {
    expect(reply.json()).toMatchObject({ error: 'DAILY_VALUE_CAP' });
  }
  const hashes: Hex[] = [];
  const nonces: number[] = [];
  for (const reply of signed) {
    const raw = reply.json().signed_tx as Hex;
    expect((await recoverTransactionAddress({ serializedTransaction: raw })).toLowerCase()).toBe(row.address.toLowerCase());
    expect(parseTransaction(raw).value).toBe(50n);
    nonces.push(parseTransaction(raw).nonce!);
    hashes.push(keccak256(raw));
  }
  expect(new Set(nonces).size).toBe(2);
  const charged = await database.query('select value_wei::text, tx_hash from wallet_signatures where wallet_id=$1', [row.id]);
  expect(charged.rows.map((entry) => entry.value_wei)).toEqual(['50', '50']);
  expect(charged.rows.map((entry) => entry.tx_hash).sort()).toEqual(hashes.sort());
  const audit = await database.query("select path,decision from signing_audit where wallet_id=$1 and decision='signed'", [row.id]);
  expect(audit.rows).toEqual([{ path: 'envelope', decision: 'signed' }, { path: 'envelope', decision: 'signed' }]);
}

async function restrictedPool(permission: 'read' | 'write'): Promise<Pool> {
  const role = `repair_cap_${permission}`;
  await database.query(`create role ${role} login`);
  await database.query(`grant usage on schema public to ${role}`);
  await database.query(`grant all privileges on all tables in schema public to ${role}`);
  await database.query(`grant usage, select on all sequences in schema public to ${role}`);
  if (permission === 'read') {
    await database.query(`revoke select on wallet_signatures from ${role}`);
    await database.query(`grant select(id,user_id,created_at) on wallet_signatures to ${role}`);
  } else {
    await database.query(`revoke insert on wallet_signatures from ${role}`);
  }
  const url = new URL(postgres.url);
  url.username = role;
  const pool = new Pool({ connectionString: url.toString() });
  restrictedPools.push(pool);
  return pool;
}

async function humanDailyCapDatabaseRefusalBeforeLegacyAuthority() {
  for (const permission of ['read', 'write'] as const) {
    const user = randomUUID();
    const { row } = await wallets.provisionWalletForUser(user);
    const pool = await restrictedPool(permission);
    const instance = await mount(pool);
    const simulationsBefore = simulations.length;
    const broadcastsBefore = broadcasts.length;
    const reply = await humanRequest(instance, user, 'sign', '50');
    expect(reply.statusCode, reply.body).toBe(503);
    expect(reply.json()).toMatchObject({ error: 'POLICY_UNAVAILABLE' });
    expect(reply.json()).not.toHaveProperty('signed_tx');
    expect(simulations).toHaveLength(simulationsBefore);
    expect(broadcasts).toHaveLength(broadcastsBefore);
    expect((await database.query('select id from wallet_signatures where wallet_id=$1', [row.id])).rows).toEqual([]);
    expect((await database.query('select nonce from nonce_outstanding where address=$1', [row.address.toLowerCase()])).rows).toEqual([]);
    if (permission === 'read') {
      expect(await humanPolicy.evaluate({ user_id: user, value_wei: 1n }, pool.query.bind(pool)))
        .toMatchObject({ allow: false, code: 'POLICY_UNAVAILABLE' });
    }
  }
}

async function humanDailyCapRetainsUncertainSendAcrossReopen() {
  const user = randomUUID();
  const { row } = await wallets.provisionWalletForUser(user);
  const before = broadcasts.length;
  const first = await humanRequest(app, user, 'send', '100');
  expect(first.statusCode, first.body).toBe(500);
  expect(first.json()).toMatchObject({ error: 'send_failed' });
  expect(broadcasts).toHaveLength(before + 1);
  const raw = broadcasts[before]!;
  expect((await recoverTransactionAddress({ serializedTransaction: raw })).toLowerCase()).toBe(row.address.toLowerCase());
  expect(parseTransaction(raw).value).toBe(100n);
  const hash = keccak256(raw);
  const reservations = await database.query('select value_wei::text,tx_hash from wallet_signatures where wallet_id=$1', [row.id]);
  expect(reservations.rows).toEqual([{ value_wei: '100', tx_hash: hash }]);
  expect((await database.query('select tx_hash from nonce_outstanding where address=$1', [row.address.toLowerCase()])).rows).toEqual([{ tx_hash: hash }]);
  await app.close();
  const reopenedPool = new Pool({ connectionString: postgres.url });
  restrictedPools.push(reopenedPool);
  app = await mount(reopenedPool);
  for (const route of ['sign', 'send'] as const) {
    const retry = await humanRequest(app, user, route, '1');
    expect(retry.statusCode, retry.body).toBe(403);
    expect(retry.json()).toMatchObject({ error: 'DAILY_VALUE_CAP' });
  }
  expect(broadcasts).toHaveLength(before + 1);
  expect((await database.query('select value_wei::text,tx_hash from wallet_signatures where wallet_id=$1', [row.id])).rows).toEqual(reservations.rows);
}

async function principal() {
  const key = newAgentKey('repair-precompile');
  await agents.upsertPrincipalOnVerify({ did: key.did, label: 'repair-precompile', publicKey: key.publicKeyHex, keyFingerprint: key.publicKeyHex.slice(0, 16) });
  await agents.setPrincipalFrozen(key.did, false);
  const { wallet } = await wallets.provisionWalletForUser(wallets.agentWalletUserId(key.did), 'agent');
  await agents.setPrincipalWallet(key.did, wallet.id);
  await agents.upsertPolicy(key.did, { mode: 'full', withdrawal_allowlist_only: true, max_approve_wei: 100n }, null);
  return key;
}

async function agentRequest(key: AgentKey, url: string, payload: unknown) {
  const body = Buffer.from(JSON.stringify(payload));
  const nonce = randomBytes(16);
  const expiry = BigInt(Math.floor(Date.now() / 1000) + 60);
  const digest = requests.agentRequestDigest({ method: `POST ${url}`, keyId: key.did, nonce, expiry, body });
  return app.inject({
    method: 'POST', url, payload: body.toString(),
    headers: { 'content-type': 'application/json', 'x-agent-key': key.did,
      'x-agent-nonce': nonce.toString('hex'), 'x-agent-expires': expiry.toString(), 'x-agent-signature': key.sign(digest) },
  });
}

async function rule(key: AgentKey, effect: 'allow' | 'deny', subject: 'withdrawal' | 'token' | 'contract' | 'selector' | 'address', value: string, maxValueWei?: bigint) {
  await agents.addRule({ did: key.did, effect, subject, value, maxValueWei });
}

async function assertRawAndHelperDenial(key: AgentKey, call: { to: Hex; data: Hex }, helper: string, payload: unknown, code: string) {
  const before = broadcasts.length;
  for (const url of ['/v1/agent/sign', '/v1/agent/send']) {
    const reply = await agentRequest(key, url, { tx: { to: call.to, data: call.data, value: '0', gas: '100000', maxFeePerGas: '2000000000', maxPriorityFeePerGas: '1000000000' } });
    expect(reply.statusCode, `${url}: ${reply.body}`).toBe(403);
    expect(reply.json()).toMatchObject({ error: code });
  }
  const reply = await agentRequest(key, `/v1/agent/precompiles/${helper}`, payload);
  expect(reply.statusCode, reply.body).toBe(403);
  expect(reply.json()).toMatchObject({ error: code });
  expect(broadcasts).toHaveLength(before);
  const wallet = await wallets.findWalletByUserId(wallets.agentWalletUserId(key.did), 'agent');
  expect((await database.query('select id from wallet_signatures where wallet_id=$1', [wallet!.id])).rows).toEqual([]);
}

async function rawPrecompilePolicyMatchesHelperRecipientsAndCaps() {
  const streamPayload = { payee: blocked, token, rate_per_second: '1', cap: '101' };
  const streamCall = precompiles.streams.open({ payee: blocked, token, ratePerSecond: 1n, cap: 101n });
  const deniedRecipient = await principal();
  await assertRawAndHelperDenial(deniedRecipient, streamCall, 'streams/open', streamPayload, 'WITHDRAWAL_NOT_ALLOWLISTED');

  const deniedToken = await principal();
  await rule(deniedToken, 'allow', 'withdrawal', blocked);
  await rule(deniedToken, 'deny', 'token', token);
  await assertRawAndHelperDenial(deniedToken, streamCall, 'streams/open', streamPayload, 'TOKEN_DENYLISTED');

  const deniedCap = await principal();
  await rule(deniedCap, 'allow', 'withdrawal', blocked);
  await rule(deniedCap, 'allow', 'token', token, 100n);
  await assertRawAndHelperDenial(deniedCap, streamCall, 'streams/open', streamPayload, 'TOKEN_VALUE_CAP');

  const scheduled = precompiles.scheduler.schedule({ target: blocked, callData: '0x', executeAtBlock: 100n, gasLimit: 100000n });
  await assertRawAndHelperDenial(await principal(), scheduled, 'scheduler/schedule', {
    target: blocked, call_data: '0x', execute_at_block: '100', gas_limit: '100000',
  }, 'WITHDRAWAL_NOT_ALLOWLISTED');

  const p = await agents.findPrincipal(deniedCap.did);
  expect(p).not.toBeNull();
  const decision = await policy.evaluateAgent({ principal: p!, policy: await agents.getPolicy(deniedCap.did), rules: await agents.listRules(deniedCap.did), intent: {
    kind: 'transaction', ...streamCall, tokenContract: recipient, tokenRecipient: recipient, tokenAmount: 0n,
  } });
  expect(decision).toMatchObject({ allow: false, code: 'TOKEN_VALUE_CAP' });
}

async function rawPrecompilePolicyChecksNestedScheduledCalls() {
  const movement = encodeFunctionData({ abi: erc20Abi, functionName: 'transfer', args: [blocked, 101n] });
  const call = precompiles.scheduler.schedule({ target: token, callData: movement, executeAtBlock: 100n, gasLimit: 100000n });
  const payload = { target: token, call_data: movement, execute_at_block: '100', gas_limit: '100000' };
  const withdrawal = await principal();
  await rule(withdrawal, 'allow', 'withdrawal', token);
  await assertRawAndHelperDenial(withdrawal, call, 'scheduler/schedule', payload, 'WITHDRAWAL_NOT_ALLOWLISTED');

  const cap = await principal();
  await rule(cap, 'allow', 'withdrawal', token);
  await rule(cap, 'allow', 'withdrawal', blocked);
  await rule(cap, 'allow', 'token', token, 100n);
  await assertRawAndHelperDenial(cap, call, 'scheduler/schedule', payload, 'TOKEN_VALUE_CAP');

  const destination = await principal();
  await rule(destination, 'allow', 'withdrawal', token);
  await rule(destination, 'deny', 'contract', token);
  await assertRawAndHelperDenial(destination, call, 'scheduler/schedule', payload, 'CONTRACT_DENYLISTED');

  const selector = await principal();
  await rule(selector, 'allow', 'withdrawal', token);
  await rule(selector, 'deny', 'selector', movement.slice(0, 10));
  await assertRawAndHelperDenial(selector, call, 'scheduler/schedule', payload, 'SELECTOR_DENYLISTED');

  const nestedStream = precompiles.streams.open({ payee: blocked, token, ratePerSecond: 1n, cap: 101n });
  const middle = precompiles.scheduler.schedule({ target: nestedStream.to, callData: nestedStream.data, executeAtBlock: 100n, gasLimit: 100000n });
  const outer = precompiles.scheduler.schedule({ target: middle.to, callData: middle.data, executeAtBlock: 99n, gasLimit: 100000n });
  const nested = await principal();
  for (const address of [middle.to, nestedStream.to, blocked]) await rule(nested, 'allow', 'withdrawal', address);
  await rule(nested, 'allow', 'token', token, 100n);
  await assertRawAndHelperDenial(nested, outer, 'scheduler/schedule', {
    target: middle.to, call_data: middle.data, execute_at_block: '99', gas_limit: '100000',
  }, 'TOKEN_VALUE_CAP');
}

async function rawPrecompilePolicyRefusesUndecodableMovement() {
  const key = await principal();
  await agents.upsertPolicy(key.did, { withdrawal_allowlist_only: false }, null);
  const call = precompiles.streams.open({ payee: recipient, token, ratePerSecond: 1n, cap: 100n });
  for (const data of ['0x', '0xffffffff', call.data.slice(0, -2), `${call.data}00`] as Hex[]) {
    const reply = await agentRequest(key, '/v1/agent/send', { tx: { to: call.to, data, value: '0' } });
    expect(reply.statusCode, reply.body).toBe(403);
    expect(reply.json()).toMatchObject({ error: 'COUNTERPARTY_UNDECODED' });
  }
  for (const [call, helper, payload] of [
    [precompiles.streams.settle(1n), 'streams/settle', { stream_id: '1' }],
    [precompiles.streams.close(1n), 'streams/close', { stream_id: '1' }],
    [precompiles.streams.updateRate(1n, 1n), 'streams/update-rate', { stream_id: '1', new_rate: '1' }],
    [precompiles.scheduler.reschedule(1n, 100n), 'scheduler/reschedule', { job_id: '1', new_block: '100' }],
  ] as const) {
    await assertRawAndHelperDenial(key, call, helper, payload, 'COUNTERPARTY_UNDECODED');
  }
}

async function rawPrecompilePolicyAllowsDecodedCallsAtPolicyBoundaries() {
  const key = await principal();
  for (const address of [token, recipient, precompiles.PRECOMPILE_ADDRESSES.streams]) {
    await rule(key, 'allow', 'withdrawal', address);
  }
  await rule(key, 'allow', 'token', token, 100n);
  const principalRow = await agents.findPrincipal(key.did);
  const agentPolicy = await agents.getPolicy(key.did);
  const rules = await agents.listRules(key.did);
  const stream = precompiles.streams.open({ payee: recipient, token, ratePerSecond: 1n, cap: 100n });
  const transfer = encodeFunctionData({ abi: erc20Abi, functionName: 'transfer', args: [recipient, 100n] });
  const approve = encodeFunctionData({ abi: erc20Abi, functionName: 'approve', args: [recipient, 100n] });
  for (const call of [stream,
    precompiles.scheduler.schedule({ target: token, callData: transfer, executeAtBlock: 100n, gasLimit: 100000n }),
    precompiles.scheduler.schedule({ target: token, callData: approve, executeAtBlock: 100n, gasLimit: 100000n }),
    precompiles.scheduler.schedule({ target: stream.to, callData: stream.data, executeAtBlock: 100n, gasLimit: 100000n }),
    precompiles.scheduler.cancel(1n),
  ]) {
    expect(await policy.evaluateAgent({ principal: principalRow!, policy: agentPolicy, rules,
      intent: { kind: 'transaction', to: call.to, data: call.data, value: 0n } }))
      .toEqual({ allow: true, reservedBudgetId: null, reservedValueWei: 0n });
  }
}

async function rawPrecompilePolicyUsesServedAbiOnBothSides() {
  const source = readFileSync(new URL('../../attestor/internal/auth/agent/authority.go', import.meta.url), 'utf8');
  for (const [name, servedAbi, calls] of [
    ['agentSchedulerABI', precompiles.SCHEDULER_ABI, [precompiles.scheduler.schedule({ target: token, callData: '0x12345678', executeAtBlock: 123n, gasLimit: 100000n }), precompiles.scheduler.cancel(1n), precompiles.scheduler.reschedule(1n, 2n)]],
    ['agentStreamsABI', precompiles.STREAMS_ABI, [precompiles.streams.open({ payee: recipient, token, ratePerSecond: 2n, cap: 100n }), precompiles.streams.settle(1n), precompiles.streams.close(1n), precompiles.streams.updateRate(1n, 2n)]],
  ] as const) {
    const match = source.match(new RegExp(`const ${name} = \x60([\\s\\S]*?)\x60`));
    expect(match).not.toBeNull();
    const attestorAbi = JSON.parse(match![1]!) as Abi;
    for (const call of calls) {
      expect(decodeFunctionData({ abi: attestorAbi, data: call.data }))
        .toEqual(decodeFunctionData({ abi: servedAbi, data: call.data }));
    }
  }
}

test(humanDailyCapParallelLegacySigning.name, humanDailyCapParallelLegacySigning);
test(humanDailyCapDatabaseRefusalBeforeLegacyAuthority.name, humanDailyCapDatabaseRefusalBeforeLegacyAuthority);
test(humanDailyCapRetainsUncertainSendAcrossReopen.name, humanDailyCapRetainsUncertainSendAcrossReopen);
test(rawPrecompilePolicyMatchesHelperRecipientsAndCaps.name, rawPrecompilePolicyMatchesHelperRecipientsAndCaps);
test(rawPrecompilePolicyChecksNestedScheduledCalls.name, rawPrecompilePolicyChecksNestedScheduledCalls);
test(rawPrecompilePolicyRefusesUndecodableMovement.name, rawPrecompilePolicyRefusesUndecodableMovement);
test(rawPrecompilePolicyAllowsDecodedCallsAtPolicyBoundaries.name, rawPrecompilePolicyAllowsDecodedCallsAtPolicyBoundaries);
test(rawPrecompilePolicyUsesServedAbiOnBothSides.name, rawPrecompilePolicyUsesServedAbiOnBothSides);
