import { createHash } from 'node:crypto';
import { setTimeout } from 'node:timers/promises';
import { afterAll, beforeAll, expect, test } from 'vitest';
import type { AgentCtx } from '../src/routes/agentExec.js';
import { TestChain } from './support/chain.js';
import { newAgentKey } from './support/identity.js';
import { startPostgres, type EphemeralPostgres } from './support/postgres.js';

const chain = new TestChain(125);
const recipient = '0x00000000000000000000000000000000000000e2';
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

async function agentRateLimitReservation(): Promise<void> {
  const agents = await import('../src/db/agents.js');
  const wallets = await import('../src/db/wallets.js');
  const agent = newAgentKey('rate-limit-reservation');
  await agents.upsertPrincipalOnVerify({
    did: agent.did,
    label: 'rate-limit-reservation',
    keyFingerprint: agent.publicKeyHex.slice(0, 16),
    publicKey: agent.publicKeyHex,
  });
  await agents.setPrincipalFrozen(agent.did, false);
  await database.query(
    `update agent_policies
        set mode = 'full', allow_native_transfer = true, withdrawal_allowlist_only = false,
            max_tx_value_wei = $2, max_daily_value_wei = 0, rate_limit_per_min = 1
      where did = $1`,
    [agent.did, value.toString()],
  );
  const budget = await agents.addBudget({
    did: agent.did,
    targetContract: recipient,
    capWei: dailyCap,
    expiresAt: new Date(Date.now() + 10 * 60_000),
  });
  const { wallet } = await wallets.provisionWalletForUser(
    wallets.agentWalletUserId(agent.did),
    'agent',
  );
  await agents.setPrincipalWallet(agent.did, wallet.id);
  const principal = await agents.findPrincipal(agent.did);
  expect(principal).not.toBeNull();
  if (!principal) throw new Error('provisioned agent principal is missing');
  const ctx: AgentCtx = {
    did: agent.did,
    ownerUserId: principal.owner_user_id,
    principal,
  };
  chain.balances.set(wallet.address.toLowerCase(), 10n ** 20n);
  const startedAt = new Date().toISOString();

  async function send(index: number) {
    return execution.executeAgentTransaction({
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
        request_hash: createHash('sha256')
          .update(`${agent.did}:${index}`)
          .digest('hex'),
        ip: null,
        user_agent: null,
      },
    });
  }

  const first = await send(1);
  expect(first.status, JSON.stringify(first.body)).toBe(200);
  expect(chain.sent).toHaveLength(1);
  expect(await agents.countAgentSignaturesLastMinute(agent.did)).toBe(1);
  expect(await agents.listActiveBudgets(agent.did)).toMatchObject([
    { id: budget.id, cap_wei: dailyCap, spent_wei: value },
  ]);

  const second = await send(2);
  expect(second.status).toBe(403);
  expect(second.body).toMatchObject({ error: 'RATE_LIMIT' });
  expect(chain.sent).toHaveLength(1);
  expect(await agents.listActiveBudgets(agent.did)).toMatchObject([
    { id: budget.id, cap_wei: dailyCap, spent_wei: value },
  ]);
  expect(await agents.sumAgentDailyValue(agent.did, startedAt)).toBe(value);
  expect(await agents.countAgentSignaturesLastMinute(agent.did)).toBe(1);

  await setTimeout(61_000);
  expect(await agents.countAgentSignaturesLastMinute(agent.did)).toBe(0);
  expect(await agents.listActiveBudgets(agent.did)).toMatchObject([
    { id: budget.id, cap_wei: dailyCap, spent_wei: value },
  ]);

  const third = await send(3);
  expect(third.status, JSON.stringify(third.body)).toBe(200);
  expect(chain.sent).toHaveLength(2);
  expect(chain.balances.get(recipient)).toBe(dailyCap);
  for (const sent of chain.sent) {
    expect(sent.from).toBe(wallet.address.toLowerCase());
    expect(sent.to).toBe(recipient);
    expect(sent.value).toBe(value);
    expect(chain.receipts.get(sent.hash)?.status).toBe('0x1');
  }
  expect([first.body.tx_hash, third.body.tx_hash]).toEqual(
    chain.sent.map((sent) => sent.hash),
  );
  expect(await agents.listActiveBudgets(agent.did)).toMatchObject([
    { id: budget.id, cap_wei: dailyCap, spent_wei: dailyCap },
  ]);
  expect(await agents.sumAgentDailyValue(agent.did, startedAt)).toBe(dailyCap);
}

test(agentRateLimitReservation, agentRateLimitReservation, 90_000);
