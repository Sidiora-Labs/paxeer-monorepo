import { randomBytes, randomUUID } from 'node:crypto';
import { afterAll, beforeAll, expect, test } from 'vitest';
import { keccak256, type Hex } from 'viem';
import { generatePrivateKey, privateKeyToAccount } from 'viem/accounts';
import type {
  ActionLeg,
  ActionSignedTransaction,
  ActionTransactionDraft,
} from '../src/db/actions.js';
import { TestChain } from './support/chain.js';
import { startPostgres, type EphemeralPostgres } from './support/postgres.js';

const node = new TestChain(125);
const token = '0x000000000000000000000000000000000000c0de';
const contract = '0x000000000000000000000000000000000000beef';
let postgres: EphemeralPostgres;
let database: typeof import('../src/db/pool.js');
let actions: typeof import('../src/db/actions.js');
let orchestrator: typeof import('../src/agent/actions/orchestrator.js');
let rpc: ReturnType<typeof import('../src/rpc/pool.js').sharedRpcPool>;

beforeAll(async () => {
  await node.start();
  process.env.HYPERPAXEER_RPC_URL = node.url;
  process.env.RPC_URLS = node.url;
  process.env.SEQUENCER_RPC_URL = node.url;
  process.env.HYPERPAXEER_CHAIN_ID = '125';
  process.env.ACTION_CONFIRMATIONS = '1';
  process.env.ACTION_RECEIPT_TIMEOUT_MS = '1';
  process.env.LOG_LEVEL = 'error';
  postgres = await startPostgres();
  database = await import('../src/db/pool.js');
  actions = await import('../src/db/actions.js');
  orchestrator = await import('../src/agent/actions/orchestrator.js');
  rpc = (await import('../src/rpc/pool.js')).sharedRpcPool();
}, 120_000);

afterAll(async () => {
  rpc?.stop();
  await database?.closePool();
  await postgres?.stop();
  await node.stop();
}, 120_000);

async function pendingAction(leg: ActionLeg = 'approval') {
  const account = privateKeyToAccount(generatePrivateKey());
  const agents = await import('../src/db/agents.js');
  const publicKey = randomBytes(32).toString('hex');
  const did = `did:matrix:nonce:${publicKey.slice(0, 16)}`;
  await agents.upsertPrincipalOnVerify({
    did,
    label: 'nonce',
    keyFingerprint: publicKey.slice(0, 16),
    publicKey,
  });
  const { row } = await actions.createOrGetAction({
    did,
    walletId: null,
    walletAddress: account.address,
    kind: 'allowance_and_call',
    idempotencyKey: randomUUID(),
    request: {},
  });
  const { buildAllowanceAndCallPlan, approvalCalldata } =
    await import('../src/agent/actions/plan.js');
  const plan = buildAllowanceAndCallPlan({
    token,
    spender: contract,
    contract,
    amount: '10',
    method: 'deposit(uint256)',
    args: ['10'],
  });
  await actions.updateAction(row.id, { plan: { ...plan } });
  const budget = await agents.addBudget({
    did,
    capWei: 100n,
    expiresAt: new Date(Date.now() + 60_000),
  });
  expect(
    await actions.reserveActionBudget(row.id, {
      did,
      targetContract: contract,
      token,
      valueWei: 10n,
    }),
  ).toBe(budget.id);
  node.balances.set(account.address.toLowerCase(), 10n ** 18n);
  node.tokens.set(token, { balances: new Map(), allowances: new Map() });
  const draft: ActionTransactionDraft = {
    chainId: 125,
    to: leg === 'approval' ? token : contract,
    data: leg === 'approval' ? approvalCalldata(plan) : plan.callData,
    value: '0',
    nonce: 0,
    gas: '60000',
    maxFeePerGas: '2000000000',
    maxPriorityFeePerGas: '1000000000',
    replaces: null,
  };
  async function sign(next: ActionTransactionDraft): Promise<ActionSignedTransaction> {
    const raw = await account.signTransaction({
      type: 'eip1559',
      chainId: next.chainId,
      to: next.to,
      data: next.data,
      nonce: next.nonce,
      value: BigInt(next.value),
      gas: BigInt(next.gas),
      maxFeePerGas: BigInt(next.maxFeePerGas),
      maxPriorityFeePerGas: BigInt(next.maxPriorityFeePerGas),
    });
    return { draft: next, raw, hash: keccak256(raw) };
  }
  async function retain(next: ActionTransactionDraft) {
    await actions.retainActionTransactionDraft(row.id, leg, next);
    const signed = await sign(next);
    await actions.retainActionSignedTransaction(row.id, leg, signed);
    return signed;
  }
  const signed = await retain(draft);
  const { sharedNonceStore } = await import('../src/nonce/store.js');
  node.automine = false;
  const retained = (await actions.getAction(row.id))!;
  expect(
    await sharedNonceStore().withLock(
      account.address,
      (lease) => orchestrator.submitRetained(retained, signed, lease),
      row.id,
    ),
  ).toBeNull();
  expect(await rpc.request('eth_getTransactionReceipt', [signed.hash])).toBeNull();
  expect(node.pendingTransactions.has(signed.hash)).toBe(true);
  return { row: retained, signed, draft, sign, retain, account, budget, did, leg };
}

async function consumeExternally(action: Awaited<ReturnType<typeof pendingAction>>) {
  const external = await action.sign({
    ...action.draft,
    to: contract,
    data: '0x',
    value: '1',
    maxFeePerGas: '5000000000',
  });
  node.automine = true;
  expect(await rpc.sendRawTransaction(external.raw)).toBe(external.hash);
  expect(await rpc.getTransactionCount(action.account.address, 'latest')).toBe(1);
  return external;
}

async function signingRecords(did: string) {
  return (
    await database.query(
      'select * from wallet_signatures where principal_did = $1 order by id',
      [did],
    )
  ).rows;
}

async function agentActionEndsWhenItsNonceIsSpent(): Promise<void> {
  const action = await pendingAction();
  await consumeExternally(action);
  await database.query(
    "update agent_actions set attempts = 3, worker_locked_until = now() + interval '1 minute' where id = $1",
    [action.row.id],
  );
  const start = node.requests.length;
  const signatures = await signingRecords(action.did);
  const ended = await orchestrator.advanceById(action.row.id);
  expect(ended).toMatchObject({
    status: 'failed_terminal',
    error_code: 'NONCE_CONSUMED',
    phase: 'approval_confirmation',
    reserved_budget_id: action.budget.id,
    reserved_value_wei: '10',
    error: {
      cause: { leg: 'approval', nonce: 0, hashes: [action.signed.hash] },
      retry: { strategy: 'NEVER', allowed: false },
    },
  });
  expect(ended!.terminal_at).not.toBeNull();
  expect(ended!.request).toEqual(action.row.request);
  const reads = node.requests.slice(start);
  expect(reads[0]).toEqual({
    method: 'eth_getTransactionCount',
    params: [action.account.address, 'latest'],
  });
  expect(reads.filter((r) => r.method === 'eth_sendRawTransaction')).toEqual([]);
  expect(
    reads.filter((r) => r.method === 'eth_getTransactionReceipt').map((r) => r.params[0]),
  ).toContain(action.signed.hash);
  expect(await signingRecords(action.did)).toEqual(signatures);
  expect(
    (
      await database.query('select spent_wei::text from agent_budgets where id = $1', [
        action.budget.id,
      ])
    ).rows,
  ).toEqual([{ spent_wei: '10' }]);
  expect(
    (
      await database.query('select * from custody_signing_reservations where action_id = $1', [
        action.row.id,
      ])
    ).rows,
  ).toEqual([]);
  const after = node.requests.length;
  expect(await orchestrator.advanceById(action.row.id)).toEqual(ended);
  expect(node.requests.slice(after)).toEqual([]);
}

async function agentActionFollowsTheReceiptOfAHeldHash(): Promise<void> {
  for (const leg of ['approval', 'call'] as const) {
    const action = await pendingAction(leg);
    const replacement = await action.retain({
      ...action.draft,
      maxFeePerGas: '3000000000',
      replaces: action.signed.hash,
    });
    node.pendingTransactions.delete(action.signed.hash);
    node.automine = true;
    expect(await rpc.sendRawTransaction(action.signed.raw)).toBe(action.signed.hash);
    const start = node.requests.length;
    const advanced = await orchestrator.advanceById(action.row.id);
    expect(advanced).toMatchObject({
      status: leg === 'approval' ? 'approval_confirmed' : 'confirmed',
      error_code: null,
      error: null,
    });
    const receipts = node.requests
      .slice(start)
      .filter((r) => r.method === 'eth_getTransactionReceipt')
      .map((r) => r.params[0]);
    expect(receipts).toContain(replacement.hash);
    expect(receipts).toContain(action.signed.hash);
    expect(
      node.requests
        .slice(start)
        .filter((r) => r.method === 'eth_sendRawTransaction')
        .map((r) => r.params[0]),
    ).toEqual([replacement.raw]);
    expect(
      (
        await database.query(
          'select * from custody_signing_reservations where action_id = $1',
          [action.row.id],
        )
      ).rows,
    ).toEqual([]);
  }
}

async function agentActionWaitsOnAPooledConflict(): Promise<void> {
  const action = await pendingAction();
  const external = await action.sign({
    ...action.draft,
    maxFeePerGas: '5000000000',
    value: '1',
    data: '0x',
    to: contract,
  });
  expect(await rpc.sendRawTransaction(external.raw)).toBe(external.hash);
  const start = node.requests.length;
  const advanced = await orchestrator.advanceById(action.row.id);
  expect(advanced).toMatchObject({
    status: 'approval_pending',
    terminal_at: null,
    error_code: 'RPC_UNAVAILABLE',
    error: { retry: { strategy: 'RETRY_SAME_ACTION' } },
  });
  expect(
    node.requests
      .slice(start)
      .filter((r) => r.method === 'eth_sendRawTransaction')
      .map((r) => r.params[0]),
  ).toEqual([action.signed.raw]);
  expect(await rpc.getTransactionCount(action.account.address, 'latest')).toBe(0);
}

async function agentActionEndsOnAConsumedResend(): Promise<void> {
  const action = await pendingAction();
  await consumeExternally(action);
  const start = node.requests.length;
  expect(await orchestrator.advanceById(action.row.id)).toMatchObject({
    status: 'failed_terminal',
    error_code: 'NONCE_CONSUMED',
  });
  expect(
    node.requests
      .slice(start)
      .filter((r) => r.method === 'eth_sendRawTransaction')
      .map((r) => r.params[0]),
  ).toEqual([action.signed.raw]);
}

async function agentActionKeepsARevertedHeldReceipt(): Promise<void> {
  const action = await pendingAction();
  const reverted = await action.retain({
    ...action.draft,
    gas: '21000',
    maxFeePerGas: '3000000000',
    replaces: action.signed.hash,
  });
  node.automine = true;
  expect(await rpc.sendRawTransaction(reverted.raw)).toBe(reverted.hash);
  const { sharedNonceStore } = await import('../src/nonce/store.js');
  expect(
    await sharedNonceStore().withLock(
      action.account.address,
      (lease) => orchestrator.submitRetained(action.row, reverted, lease),
      action.row.id,
    ),
  ).toBeNull();
  const newer = await action.retain({
    ...action.draft,
    maxFeePerGas: '4000000000',
    replaces: reverted.hash,
  });
  expect(await orchestrator.advanceById(action.row.id)).toMatchObject({
    status: 'transaction_reverted',
    error_code: 'TRANSACTION_REVERTED',
    error: { cause: { tx_hash: reverted.hash, nonce: 0 } },
  });
  expect(newer.hash).not.toBe(reverted.hash);
}

test(agentActionEndsWhenItsNonceIsSpent.name, agentActionEndsWhenItsNonceIsSpent);
test(agentActionFollowsTheReceiptOfAHeldHash.name, agentActionFollowsTheReceiptOfAHeldHash);
test(agentActionWaitsOnAPooledConflict.name, agentActionWaitsOnAPooledConflict);
test(agentActionEndsOnAConsumedResend.name, agentActionEndsOnAConsumedResend);
test(agentActionKeepsARevertedHeldReceipt.name, agentActionKeepsARevertedHeldReceipt);
