import { mkdtemp, readFile, rm } from 'node:fs/promises';
import { tmpdir } from 'node:os';
import { join } from 'node:path';
import { setTimeout as delay } from 'node:timers/promises';
import { afterAll, beforeAll, expect, test } from 'vitest';
import { keccak256, recoverTransactionAddress } from 'viem';
import type { PoolClient } from 'pg';
import { PGlite, pgcrypto, PGLiteSocketServer } from '../src/db/testDatabase.js';
import type { ActionTransactionDraft } from '../src/db/actions.js';

let directory: string;
let db: PGlite;
let server: PGLiteSocketServer;
let database: typeof import('../src/db/pool.js');
let actions: typeof import('../src/db/actions.js');
let agents: typeof import('../src/db/agents.js');
let wallets: typeof import('../src/db/wallets.js');
let migration: typeof import('../src/db/migrate.js');
let configuration: typeof import('../src/env.js');

async function openDatabase(): Promise<void> {
  db = await PGlite.create({ dataDir: directory, extensions: { pgcrypto } });
  server = new PGLiteSocketServer({ db, port: 0, host: '127.0.0.1', maxConnections: 10 });
  await server.start();
  process.env.DATABASE_URL = `postgres://postgres:postgres@${server.getServerConn()}/postgres`;
  if (configuration) configuration.env.DATABASE_URL = process.env.DATABASE_URL;
}

async function stopDatabase(): Promise<void> {
  await database?.closePool();
  await server?.stop();
  await db?.close();
}

beforeAll(async () => {
  directory = await mkdtemp(join(tmpdir(), 'repair-22-33-180-'));
  await openDatabase();
  configuration = await import('../src/env.js');
  database = await import('../src/db/pool.js');
  actions = await import('../src/db/actions.js');
  agents = await import('../src/db/agents.js');
  wallets = await import('../src/db/wallets.js');
  migration = await import('../src/db/migrate.js');
  await migration.runMigrations();
}, 30_000);

afterAll(async () => {
  await stopDatabase();
  if (directory) await rm(directory, { recursive: true, force: true });
});

const did = 'did:matrix:lease:0123456789abcdef';
const recipient = '0x0000000000000000000000000000000000000180';
const draft: ActionTransactionDraft = {
  chainId: 125,
  to: recipient,
  data: '0x',
  value: '6',
  nonce: 0,
  gas: '21000',
  maxFeePerGas: '2000000000',
  maxPriorityFeePerGas: '2000000',
  replaces: null,
};

async function provisionPrincipal(subject: string): Promise<void> {
  await agents.upsertPrincipalOnVerify({
    did: subject,
    label: 'repair',
    keyFingerprint: '0123456789abcdef',
    publicKey: '01'.repeat(32),
  });
}

async function staleActionLeaseCannotOverwriteRecovery(): Promise<void> {
  await provisionPrincipal(did);
  const { row: wallet } = await wallets.provisionWalletForUser(wallets.agentWalletUserId(did), 'agent');
  await agents.setPrincipalWallet(did, wallet.id);
  const signer = await wallets.getSigningAccountForRow(wallet, undefined, {
    subject: did, requestHash: 'lease-recovery-signature',
  });
  const created = await actions.createOrGetAction({
    did,
    walletId: wallet.id,
    walletAddress: signer.address,
    kind: 'allowance_and_call',
    idempotencyKey: 'lease-recovery',
    request: { contract: recipient },
  });
  await actions.updateAction(created.row.id, { plan: { contract: recipient } });
  const original = await actions.claimNextAction(60_000);
  expect(original?.id).toBe(created.row.id);
  if (!original) throw new Error('original claim missing');
  expect(original.attempts).toBe(1);
  expect(await actions.claimNextAction(60_000)).toBeNull();
  await actions.retainActionTransactionDraft(original.id, 'call', draft, original.attempts);

  // A second wire connection observes and expires the durable lease, without
  // releasing it. The persisted draft must survive a complete database restart.
  const observer = await database.getPool().connect();
  try {
    expect((await observer.query('select attempts from agent_actions where id = $1', [original.id])).rows)
      .toEqual([{ attempts: original.attempts }]);
    await observer.query(`update agent_actions set worker_locked_until = clock_timestamp() + interval '20 milliseconds' where id = $1`, [original.id]);
    await delay(40);
    await expect(actions.updateAction(original.id, { phase: 'expired' }, original.attempts))
      .rejects.toBeInstanceOf(actions.ActionLeaseLostError);
    await expect(actions.releaseLease(original.id, original.attempts))
      .rejects.toBeInstanceOf(actions.ActionLeaseLostError);
  } finally {
    observer.release();
  }
  await stopDatabase();
  await openDatabase();
  await migration.runMigrations();
  const replacement = await actions.claimNextAction(60_000);
  if (!replacement) throw new Error('replacement claim missing');
  expect(replacement.id).toBe(original.id);
  expect(replacement.attempts).toBe(original.attempts + 1);
  expect(actions.actionLegExecution(replacement, 'call').draft).toEqual(draft);
  await actions.updateAction(replacement.id, { phase: 'recovered' }, replacement.attempts);

  const raw = await signer.signTransaction({
    type: 'eip1559', chainId: draft.chainId, nonce: draft.nonce, to: draft.to,
    data: draft.data, value: BigInt(draft.value), gas: BigInt(draft.gas),
    maxFeePerGas: BigInt(draft.maxFeePerGas), maxPriorityFeePerGas: BigInt(draft.maxPriorityFeePerGas),
  });
  const signed = { draft, raw, hash: keccak256(raw) };
  await expect(actions.retainActionSignedTransaction(replacement.id, 'call', {
    ...signed, draft: { ...draft, value: '7' },
  }, replacement.attempts)).rejects.toThrow('signed transaction differs from the retained draft');
  await actions.retainActionSignedTransaction(replacement.id, 'call', signed, replacement.attempts);
  expect(await recoverTransactionAddress({ serializedTransaction: raw })).toBe(signer.address);

  const staleMutations = [
    () => actions.updateAction(original.id, { status: 'failed_terminal', call_tx_hash: null }, original.attempts),
    () => actions.updateAction(original.id, { phase: 'unowned' }),
    () => actions.releaseLease(original.id, original.attempts),
    () => actions.releaseLease(original.id),
    () => actions.retainActionTransactionDraft(original.id, 'call', draft, original.attempts),
    () => actions.retainActionSignedTransaction(original.id, 'call', signed, original.attempts),
    () => actions.releaseActionNonce(original.id, draft.nonce, original.attempts),
    () => actions.releaseUnsignedActionNonce(original.id, original.attempts),
    () => actions.releaseActionBudget(original.id, original.attempts),
    () => actions.reserveActionBudget(original.id, { did, targetContract: recipient, token: null, valueWei: 6n }, original.attempts),
    () => actions.markCreditVerified(original.id, { stale: true }, original.attempts),
  ];
  for (const mutate of staleMutations) {
    await expect(mutate()).rejects.toBeInstanceOf(actions.ActionLeaseLostError);
  }
  const retained = await actions.getAction(original.id);
  expect(retained).toMatchObject({ attempts: replacement.attempts, phase: 'call_confirmation', status: 'call_pending', call_tx_hash: signed.hash });
  expect(actions.actionLegExecution(retained!, 'call').current).toEqual(signed);
  expect((await database.query('select signed_hash from custody_signing_reservations where action_id = $1', [original.id])).rows)
    .toEqual([{ signed_hash: signed.hash }]);
  await actions.releaseLease(replacement.id, replacement.attempts);
  await expect(actions.updateAction(replacement.id, { phase: 'after-release' }, replacement.attempts))
    .rejects.toBeInstanceOf(actions.ActionLeaseLostError);
  await stopDatabase();
  await openDatabase();
  const recovered = await actions.getAction(original.id);
  expect(recovered?.call_tx_hash).toBe(signed.hash);
  expect(actions.actionLegExecution(recovered!, 'call').current).toEqual(signed);
  const next = await actions.claimNextAction(60_000);
  expect(next?.attempts).toBe(replacement.attempts + 1);
  if (!next) throw new Error('next claim missing');
  await actions.updateAction(next.id, { status: 'confirmed' }, next.attempts);
  await actions.releaseLease(next.id, next.attempts);
}

interface QueryPlan {
  Alias?: string;
  Filter?: string;
  Plans?: QueryPlan[];
}

function budgetUpdateFilter(plan: QueryPlan): string {
  return [
    plan.Alias === 'b' ? plan.Filter ?? '' : '',
    ...(plan.Plans ?? []).map(budgetUpdateFilter),
  ].join(' ');
}

async function budgetReservationsRecheckEligibility(): Promise<void> {
  const subject = 'did:matrix:budget:0123456789abcdef';
  await provisionPrincipal(subject);
  const budget = await agents.addBudget({ did: subject, capWei: 10n, expiresAt: new Date(Date.now() + 60_000) });
  const first = await database.getPool().connect();
  const second = await database.getPool().connect();
  expect(first).not.toBe(second);
  const request = { did: subject, targetContract: recipient, token: null, valueWei: 6n };
  try {
    const reservations = await Promise.all([
      agents.reserveBudget(request, first), agents.reserveBudget(request, second),
    ]);
    expect(reservations.filter((id) => id === budget.id)).toHaveLength(1);
    expect(reservations.filter((id) => id === null)).toHaveLength(1);
    expect((await agents.listActiveBudgets(subject))[0]?.spent_wei).toBe(6n);

    // PGlite serializes backend execution. Inspect PostgreSQL's actual plan as
    // well: a row-lock wait in multi-backend PostgreSQL must recheck predicates
    // on the UPDATE target, rather than only in the snapshot-based selector.
    const explained = await first.query('explain (format json) ' + agents.BUDGET_RESERVATION_SQL,
      [subject, recipient, null, '6']);
    const filter = budgetUpdateFilter(explained.rows[0]['QUERY PLAN'][0].Plan);
    for (const predicate of ['spent_wei', 'cap_wei', 'active', 'expires_at', 'clock_timestamp()', 'did', 'target_contract']) {
      expect(filter).toContain(predicate);
    }
    const tokenPlan = await first.query('explain (format json) ' + agents.BUDGET_RESERVATION_SQL,
      [subject, recipient, recipient, '6']);
    expect(budgetUpdateFilter(tokenPlan.rows[0]['QUERY PLAN'][0].Plan)).toContain('token');

    await first.query('BEGIN');
    try {
      // Expiry is checked at the update clock, even in a long-lived transaction.
      await first.query(`update agent_budgets set spent_wei = 0, expires_at = clock_timestamp() + interval '20 milliseconds' where id = $1`, [budget.id]);
      await delay(40);
      expect(await agents.reserveBudget(request, first)).toBeNull();
    } finally {
      await first.query('ROLLBACK');
    }
    await first.query('update agent_budgets set active = false where id = $1', [budget.id]);
    expect(await agents.reserveBudget(request, second)).toBeNull();
    expect((await first.query('select spent_wei::text from agent_budgets where id = $1', [budget.id])).rows)
      .toEqual([{ spent_wei: '6' }]);
  } finally {
    first.release(); second.release();
  }
}

async function supportedBootstrapAcceptsAgentAndMigratedHuman(): Promise<void> {
  const retired = await readFile(new URL('../src/db/schema.sql', import.meta.url), 'utf8');
  const blank = await PGlite.create();
  try {
    await blank.exec(retired);
    expect((await blank.query("select tablename from pg_tables where schemaname = 'public'")).rows).toEqual([]);
  } finally {
    await blank.close();
  }
  const subject = 'did:matrix:bootstrap:0123456789abcdef';
  const agent = await wallets.provisionWalletForUser(wallets.agentWalletUserId(subject), 'agent');
  expect(await wallets.findWalletByUserId(wallets.agentWalletUserId(subject), 'agent')).toMatchObject({ id: agent.row.id, kind: 'agent' });
  const userId = '00000000-0000-4000-8000-000000000180';
  const inserted = await database.query<{ id: string }>(`insert into wallets
    (user_id, address, encrypted_private_key, kind, migrated_at, attestor_key_id)
    values ($1, $2, null, 'standard', clock_timestamp(), $3) returning id`,
    [userId, recipient, 'repair-22-33-180-human']);
  await database.withTransaction(async (client: PoolClient) => {
    const migrated = await wallets.loadWalletForSigning(client, userId);
    expect(migrated).toMatchObject({
      row: { id: inserted.rows[0]!.id, kind: 'standard', encrypted_private_key: null },
      attestorKeyId: 'repair-22-33-180-human',
    });
    expect(migrated?.migratedAt).not.toBeNull();
  });
  await migration.runMigrations();
  expect(await wallets.findWalletByUserId(userId)).toMatchObject({ id: inserted.rows[0]!.id, encrypted_private_key: null });
}

test(staleActionLeaseCannotOverwriteRecovery.name, staleActionLeaseCannotOverwriteRecovery, 30_000);
test(budgetReservationsRecheckEligibility.name, budgetReservationsRecheckEligibility, 30_000);
test(supportedBootstrapAcceptsAgentAndMigratedHuman.name, supportedBootstrapAcceptsAgentAndMigratedHuman, 30_000);
