import { randomBytes, randomUUID } from 'node:crypto';
import { createServer, type Server } from 'node:http';
import type { AddressInfo } from 'node:net';
import type { FastifyInstance } from 'fastify';
import { keccak256, parseTransaction, recoverTransactionAddress, type Hex } from 'viem';
import { afterAll, beforeAll, expect, test } from 'vitest';
import { newAgentKey, startIdentityProvider, type IdentityProvider } from './support/identity.js';
import { startPostgres, type EphemeralPostgres } from './support/postgres.js';

let idp: IdentityProvider;
let pg: EphemeralPostgres;
let app: FastifyInstance;
let rpcServer: Server;
let database: typeof import('../src/db/pool.js');
let rpc: typeof import('../src/rpc/pool.js');
const latestCounts = new Map<string, number>();
const broadcasts: Hex[] = [];
const lookups: Hex[] = [];
let broadcastError = 'nonce too low';

beforeAll(async () => {
  rpcServer = createServer(async (req, res) => {
    const chunks: Buffer[] = [];
    for await (const chunk of req) chunks.push(Buffer.from(chunk));
    const body = JSON.parse(Buffer.concat(chunks).toString('utf8')) as {
      id: number; method: string; params: Hex[];
    };
    let result: unknown;
    let error: { code: number; message: string } | undefined;
    switch (body.method) {
      case 'eth_blockNumber': result = '0x64'; break;
      case 'eth_getTransactionCount':
        result = `0x${(latestCounts.get(body.params[0]!.toLowerCase()) ?? 0).toString(16)}`;
        break;
      case 'eth_estimateGas': result = '0x5208'; break;
      case 'eth_getBlockByNumber': result = { number: '0x64', baseFeePerGas: '0x3b9aca00' }; break;
      case 'eth_maxPriorityFeePerGas': result = '0x3b9aca00'; break;
      case 'eth_sendRawTransaction': {
        const raw = body.params[0]!;
        broadcasts.push(raw);
        if (broadcastError === 'nonce too low') {
          const sender = await recoverTransactionAddress({ serializedTransaction: raw });
          latestCounts.set(sender.toLowerCase(), (parseTransaction(raw).nonce ?? 0) + 1);
        }
        error = { code: -32000, message: broadcastError };
        break;
      }
      case 'eth_getTransactionByHash':
        lookups.push(body.params[0]!);
        result = null;
        break;
      default: error = { code: -32601, message: 'method not found' };
    }
    res.writeHead(200, { 'content-type': 'application/json' });
    res.end(JSON.stringify({ jsonrpc: '2.0', id: body.id, ...(error ? { error } : { result }) }));
  });
  await new Promise<void>((resolve) => rpcServer.listen(0, '127.0.0.1', resolve));
  const rpcUrl = `http://127.0.0.1:${(rpcServer.address() as AddressInfo).port}`;
  process.env.RPC_URLS = rpcUrl;
  process.env.HYPERPAXEER_RPC_URL = rpcUrl;
  process.env.SEQUENCER_RPC_URL = rpcUrl;
  process.env.AGENT_JWT_SECRET = randomBytes(32).toString('hex');
  process.env.LOG_LEVEL = 'error';
  idp = await startIdentityProvider();
  process.env.SUPABASE_URL = idp.url;
  pg = await startPostgres();
  database = await import('../src/db/pool.js');
  rpc = await import('../src/rpc/pool.js');
  const { buildApp } = await import('../src/index.js');
  app = await buildApp();
  await app.ready();
}, 120_000);

afterAll(async () => {
  await app?.close();
  rpc?.sharedRpcPool().stop();
  await database?.closePool();
  await pg?.stop();
  await idp?.stop();
  await new Promise<void>((resolve, reject) => {
    if (!rpcServer) return resolve();
    rpcServer.close((error) => error ? reject(error) : resolve());
  });
}, 120_000);

async function fundAndSweepAnswerAnUnknownBroadcastOutcomeWith202PollReceipt(): Promise<void> {
  const ownerId = randomUUID();
  const agent = newAgentKey('owner-broadcast');
  const agents = await import('../src/db/agents.js');
  const wallets = await import('../src/db/wallets.js');
  await agents.upsertPrincipalOnVerify({
    did: agent.did, label: 'owner-broadcast', keyFingerprint: agent.publicKeyHex.slice(0, 16),
    publicKey: agent.publicKeyHex,
  });
  expect(await agents.claimPrincipal(agent.did, ownerId)).toBe(true);
  const { wallet: ownerWallet } = await wallets.provisionWalletForUser(ownerId);
  const { wallet: agentWallet } = await wallets.provisionWalletForUser(wallets.agentWalletUserId(agent.did), 'agent');
  await agents.setPrincipalWallet(agent.did, agentWallet.id);
  const authorization = `Bearer ${await idp.mintUserToken(ownerId)}`;

  for (const route of ['fund', 'sweep'] as const) {
    const sender = route === 'fund' ? ownerWallet : agentWallet;
    const recipient = route === 'fund' ? agentWallet : ownerWallet;
    const countBefore = broadcasts.length;
    const response = await app.inject({
      method: 'POST', url: `/v1/agents/${encodeURIComponent(agent.did)}/${route}`,
      headers: { authorization }, payload: { amount: '1' },
    });
    expect(response.statusCode, response.body).toBe(202);
    expect(broadcasts).toHaveLength(countBefore + 1);
    const raw = broadcasts[countBefore]!;
    const hash = keccak256(raw);
    expect(response.json()).toMatchObject({
      error: 'outcome_unknown', tx_hash: hash, nonce: 0, retry: 'poll_receipt',
      poll: 'eth_getTransactionReceipt', must_not_resubmit: true, members: 1,
    });
    expect(lookups).toContain(hash);
    expect(parseTransaction(raw)).toMatchObject({ to: recipient.address.toLowerCase(), value: 1n, nonce: 0 });
    expect((await recoverTransactionAddress({ serializedTransaction: raw })).toLowerCase()).toBe(sender.address.toLowerCase());
    const outstanding = await database.query(
      'select nonce::text, tx_hash from nonce_outstanding where address = $1', [sender.address.toLowerCase()],
    );
    expect(outstanding.rows).toEqual([{ nonce: '0', tx_hash: hash }]);
  }

  broadcastError = 'replacement transaction underpriced';
  for (const route of ['fund', 'sweep']) {
    const countBefore = broadcasts.length;
    const response = await app.inject({
      method: 'POST', url: `/v1/agents/${encodeURIComponent(agent.did)}/${route}`,
      headers: { authorization }, payload: { amount: '1' },
    });
    expect(response.statusCode, response.body).toBe(409);
    expect(response.json()).toMatchObject({ error: 'nonce_conflict', retry: 'prepare' });
    expect(broadcasts).toHaveLength(countBefore + 1);
  }
}

test('fund and sweep answer an unknown broadcast outcome with 202 poll_receipt',
  fundAndSweepAnswerAnUnknownBroadcastOutcomeWith202PollReceipt);
