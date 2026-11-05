import { createHash } from 'node:crypto';
import { afterAll, beforeAll, expect, test } from 'vitest';
import type { AgentCtx } from '../src/routes/agentExec.js';
import { TestChain } from './support/chain.js';
import { newAgentKey } from './support/identity.js';
import { startPostgres, type EphemeralPostgres } from './support/postgres.js';

const chain = new TestChain(125);
const recipient = '0x00000000000000000000000000000000000000e1';
const value = 1_000_000_000_000_000n;
const dailyCap = 2n * value;
let postgres: EphemeralPostgres;
let database: typeof import('../src/db/pool.js');
let rpc: typeof import('../src/rpc/pool.js');
let execution: typeof import('../src/routes/agentExec.js');

beforeAll(async () => {
  await chain.start();
  process.env.HYPERPAXEER_CHAIN_ID = String(chain.chainId);
  process.env.HYPERPAXEER_RPC_URL = chain.url;
  process.env.RPC_URLS = chain.url;
  process.env.SEQUENCER_RPC_URL = chain.url;
  process.env.LOG_LEVEL = 'error';
  postgres = await startPostgres();
  database = await import('../src/db/pool.js');
  rpc = await import('../src/rpc/pool.js');
  execution = await import('../src/routes/agentExec.js');
}, 120_000);

afterAll(async () => {
  rpc?.sharedRpcPool().stop();
  await database?.closePool();
  await postgres?.stop();
  await chain.stop();
}, 120_000);

async function agentDailyCapConcurrency(): Promise<void> {
  const agents = await import('../src/db/agents.js');
  const wallets = await import('../src/db/wallets.js');
  const agent = newAgentKey('daily-cap-concurrency');
  await agents.upsertPrincipalOnVerify({
    did: agent.did,
    label: 'daily-cap-concurrency',
    keyFingerprint: agent.publicKeyHex.slice(0, 16),
    publicKey: agent.publicKeyHex,
  });
  await agents.setPrincipalFrozen(agent.did, false);
  await database.query(
    `update agent_policies
        set mode = 'full', allow_native_transfer = true, withdrawal_allowlist_only = false,
            max_tx_value_wei = $2, max_daily_value_wei = $3, rate_limit_per_min = 60
      where did = $1`,
    [agent.did, value.toString(), dailyCap.toString()]
  );
  const { wallet } = await wallets.provisionWalletForUser(
    wallets.agentWalletUserId(agent.did),
    'agent'
  );
  await agents.setPrincipalWallet(agent.did, wallet.id);
  const principal = await agents.findPrincipal(agent.did);
  expect(principal).not.toBeNull();
  if (!principal) throw new Error('provisioned agent principal is missing');
  const ctx: AgentCtx = { did: agent.did, ownerUserId: principal.owner_user_id, principal };
  chain.balances.set(wallet.address.toLowerCase(), 10n ** 20n);
  const startedAt = new Date().toISOString();

  const responses = await Promise.all(
    Array.from({ length: 5 }, (_, index) =>
      execution.executeAgentTransaction({
        ctx,
        intent: { kind: 'transaction', to: recipient, value },
        tx: {
          to: recipient,
          value,
          gas: 21_000n,
          maxFeePerGas: 2_000_000_000n,
          maxPriorityFeePerGas: 2_000_000n,
        },
        broadcast: true,
        audit: {
          request_hash: createHash('sha256').update(`${agent.did}:${index}`).digest('hex'),
          ip: null,
          user_agent: null,
        },
      })
    )
  );

  const denied = responses.filter((response) => response.status === 403);
  expect(denied).toHaveLength(3);
  for (const response of denied) {
    expect(response.body).toMatchObject({
      error: 'DAILY_VALUE_CAP',
      used: dailyCap.toString(),
      request: value.toString(),
      cap: dailyCap.toString(),
    });
  }

  expect(chain.sent).toHaveLength(2);
  for (const sent of chain.sent) {
    expect(sent.from).toBe(wallet.address.toLowerCase());
    expect(sent.to).toBe(recipient);
    expect(sent.value).toBe(value);
    expect(chain.receipts.get(sent.hash)?.status).toBe('0x1');
  }
  expect(chain.balances.get(recipient)).toBe(dailyCap);
  const signatures = await database.query<{ tx_hash: string; value_wei: string }>(
    `select tx_hash, value_wei::text as value_wei from wallet_signatures
      where principal_did = $1 and kind = 'transaction'`,
    [agent.did]
  );
  expect(signatures.rows).toHaveLength(2);
  expect(signatures.rows.map((row) => row.tx_hash).sort()).toEqual(
    chain.sent.map((sent) => sent.hash).sort()
  );
  expect(signatures.rows.map((row) => row.value_wei)).toEqual([value.toString(), value.toString()]);
  expect(await agents.sumAgentDailyValue(agent.did, startedAt)).toBe(dailyCap);
  const budgets = await database.query('select id from agent_budgets where did = $1', [agent.did]);
  expect(budgets.rows).toEqual([]);

  const allowed = responses.filter((response) => response.status === 200);
  expect(allowed, JSON.stringify(responses)).toHaveLength(2);
  expect(allowed.map((response) => response.body.tx_hash).sort()).toEqual(
    chain.sent.map((sent) => sent.hash).sort()
  );
}

test('parallel agent sends stay within the daily native value cap', agentDailyCapConcurrency);
