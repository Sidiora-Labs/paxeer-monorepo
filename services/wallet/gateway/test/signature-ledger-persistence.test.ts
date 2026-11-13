import { mkdtemp, rm } from 'node:fs/promises';
import { tmpdir } from 'node:os';
import { join } from 'node:path';
import { afterAll, beforeAll, expect, test } from 'vitest';
import {
  hashMessage,
  keccak256,
  parseTransaction,
  recoverMessageAddress,
  recoverTransactionAddress,
  recoverTypedDataAddress,
  type TransactionSerializableEIP1559,
} from 'viem';
import { PGlite, pgcrypto, PGLiteSocketServer } from '../src/db/testDatabase.js';

let directory: string;
let db: PGlite;
let server: PGLiteSocketServer;
let database: typeof import('../src/db/pool.js');
let wallets: typeof import('../src/db/wallets.js');
let agents: typeof import('../src/db/agents.js');
let policy: typeof import('../src/policy/index.js');
let agentPolicy: typeof import('../src/policy/agent.js');
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
  directory = await mkdtemp(join(tmpdir(), 'signature-ledger-'));
  process.env.POLICY_MAX_TX_VALUE_WEI = '10';
  process.env.POLICY_MAX_DAILY_VALUE_WEI = '10';
  process.env.POLICY_RATE_LIMIT_PER_MINUTE = '100';
  await openDatabase();
  configuration = await import('../src/env.js');
  database = await import('../src/db/pool.js');
  migration = await import('../src/db/migrate.js');
  wallets = await import('../src/db/wallets.js');
  agents = await import('../src/db/agents.js');
  policy = await import('../src/policy/index.js');
  agentPolicy = await import('../src/policy/agent.js');
  await migration.runMigrations();
}, 30_000);

afterAll(async () => {
  await stopDatabase();
  if (directory) await rm(directory, { recursive: true, force: true });
});

const recipient = '0x00000000000000000000000000000000000000e2';
const transaction: TransactionSerializableEIP1559 = {
  type: 'eip1559',
  chainId: 125,
  nonce: 0,
  to: recipient,
  value: 10n,
  gas: 21_000n,
  maxFeePerGas: 2_000_000_000n,
  maxPriorityFeePerGas: 2_000_000n,
};

async function signatureLedgerWriteLossSurvivesRestart(): Promise<void> {
  const did = 'did:matrix:ledger:0123456789abcdef';
  const userId = wallets.agentWalletUserId(did);
  const { row: wallet } = await wallets.provisionWalletForUser(userId, 'agent');
  await agents.upsertPrincipalOnVerify({
    did,
    label: 'ledger',
    keyFingerprint: '0123456789abcdef',
    publicKey: '01'.repeat(32),
  });
  await agents.setPrincipalWallet(did, wallet.id);
  await agents.setPrincipalFrozen(did, false);
  await database.query(
    `update agent_policies set mode = 'full', max_tx_value_wei = 10,
    max_daily_value_wei = 10, rate_limit_per_min = 100, allow_native_transfer = true,
    withdrawal_allowlist_only = false where did = $1`,
    [did],
  );

  const context = { subject: did, route: '/v1/agent/sign', requestHash: 'insert-failure' };
  const account = await wallets.getMigrationAwareSigningAccountForRow(
    wallet,
    undefined,
    null,
    undefined,
    context,
  );
  await database.query(`alter table wallet_signatures add constraint repair_insert_failure
    check (request_hash <> 'insert-failure') not valid`);
  await expect(account.signTransaction(transaction)).rejects.toMatchObject({ code: '23514' });
  expect((await database.query('select * from wallet_signature_reservations')).rowCount).toBe(0);
  expect(await agents.sumAgentDailyValue(did, '2000-01-01T00:00:00Z')).toBe(0n);
  await database.query('alter table wallet_signatures drop constraint repair_insert_failure');

  context.requestHash = 'completion-failure';
  await database.query(`create function repair_completion_failure() returns trigger language plpgsql as $$
    begin raise exception 'repair completion write failure'; end $$;
    create trigger repair_completion_failure before update on wallet_signature_reservations
    for each row execute function repair_completion_failure()`);
  await expect(account.signTransaction(transaction)).rejects.toThrow(
    'repair completion write failure',
  );
  const pending = await database.query(`select s.value_wei::text, r.signed_evidence, r.tx_hash
    from wallet_signatures s join wallet_signature_reservations r on r.signature_id = s.id`);
  expect(pending.rows).toEqual([{ value_wei: '10', signed_evidence: null, tx_hash: null }]);
  expect(await agents.countAgentSignaturesLastMinute(did)).toBe(1);

  await stopDatabase();
  await openDatabase();
  await migration.runMigrations();
  expect(await agents.sumAgentDailyValue(did, '2000-01-01T00:00:00Z')).toBe(10n);
  expect(await agents.countAgentSignaturesLastMinute(did)).toBe(1);
  expect(await policy.evaluate({ user_id: userId, value_wei: 1n })).toMatchObject({
    allow: false,
    code: 'DAILY_VALUE_CAP',
  });
  const principal = await agents.findPrincipal(did);
  expect(principal).not.toBeNull();
  if (!principal) throw new Error('provisioned agent missing');
  const decision = await agentPolicy.evaluateAgent({
    principal,
    policy: await agents.getPolicy(did),
    rules: await agents.listRules(did),
    intent: { kind: 'transaction', to: recipient, value: 1n },
  });
  expect(decision).toMatchObject({ allow: false, code: 'DAILY_VALUE_CAP' });

  await database.query(
    'drop trigger repair_completion_failure on wallet_signature_reservations; drop function repair_completion_failure()',
  );
  const recoveredWallet = await wallets.findWalletByUserId(userId, 'agent');
  expect(recoveredWallet).not.toBeNull();
  if (!recoveredWallet) throw new Error('wallet did not survive restart');
  const recoveredAccount = await wallets.getMigrationAwareSigningAccountForRow(
    recoveredWallet,
    undefined,
    null,
    undefined,
    context,
  );
  const raw = await recoveredAccount.signTransaction(transaction);
  expect(await recoverTransactionAddress({ serializedTransaction: raw })).toBe(wallet.address);
  expect(parseTransaction(raw)).toMatchObject({
    to: recipient,
    value: 10n,
    nonce: 0,
    chainId: 125,
  });
  const hash = keccak256(raw);
  const input = {
    user_id: userId,
    wallet_id: wallet.id,
    address: wallet.address,
    kind: 'transaction' as const,
    to_address: recipient,
    value_wei: 10n,
    chain_id: 125,
    request_hash: context.requestHash,
    tx_hash: hash,
    principal_did: did,
  };
  await wallets.logSignature(input);
  await wallets.logSignature(input);
  expect(await recoveredAccount.signTransaction(transaction)).toBe(raw);
  const alias = await wallets.getMigrationAwareSigningAccountForRow(
    recoveredWallet,
    undefined,
    null,
    undefined,
    { ...context, requestHash: 'same-transaction-new-request' },
  );
  expect(await alias.signTransaction(transaction)).toBe(raw);
  await wallets.logSignature({ ...input, request_hash: 'same-transaction-new-request' });
  expect(await agents.sumAgentDailyValue(did, '2000-01-01T00:00:00Z')).toBe(10n);
  expect(await agents.countAgentSignaturesLastMinute(did)).toBe(1);
  await expect(recoveredAccount.signTransaction({ ...transaction, value: 9n })).rejects.toThrow(
    'cannot be retargeted',
  );
  await expect(wallets.logSignature({ ...input, value_wei: 9n })).rejects.toThrow(
    'no matching durable signature',
  );
  await expect(wallets.logSignature({ ...input, tx_hash: `0x${'00'.repeat(32)}` })).rejects.toThrow(
    'no matching durable signature',
  );

  await database.query(
    `alter table wallet_signatures add constraint repair_metadata_failure check (ip is null) not valid`,
  );
  await expect(wallets.logSignature({ ...input, ip: '127.0.0.1' })).rejects.toMatchObject({
    code: '23514',
  });
  await database.query('alter table wallet_signatures drop constraint repair_metadata_failure');
  await stopDatabase();
  await openDatabase();
  expect(await policy.evaluate({ user_id: userId, value_wei: 1n })).toMatchObject({
    allow: false,
    code: 'DAILY_VALUE_CAP',
  });
  const retained = await database.query(
    'select signed_evidence, tx_hash from wallet_signature_reservations',
  );
  expect(retained.rows).toEqual([{ signed_evidence: raw, tx_hash: hash }]);
  await database.query(`create function repair_completion_failure() returns trigger language plpgsql as $$
    begin raise exception 'repair completion write failure'; end $$;
    create trigger repair_completion_failure before update on wallet_signature_reservations
    for each row execute function repair_completion_failure()`);
  const retainedAccount = await wallets.getMigrationAwareSigningAccountForRow(
    recoveredWallet,
    undefined,
    null,
    undefined,
    context,
  );
  expect(await retainedAccount.signTransaction(transaction)).toBe(raw);
  await database.query(
    'drop trigger repair_completion_failure on wallet_signature_reservations; drop function repair_completion_failure()',
  );
  await wallets.logSignature(input);
  expect(await agents.countAgentSignaturesLastMinute(did)).toBe(1);
}

async function signatureLedgerCountsAllSigningKinds(): Promise<void> {
  const userId = '00000000-0000-4000-8000-000000000372';
  const { row: wallet } = await wallets.provisionWalletForUser(userId);
  const missing = await wallets.getSigningAccountForRow(wallet);
  await expect(
    missing.signMessage({ message: 'must not sign without request context' }),
  ).rejects.toThrow('requires request context');
  const wrong = await wallets.getSigningAccountForRow(wallet, undefined, {
    subject: 'wrong-owner',
    requestHash: 'wrong',
  });
  await expect(wrong.signMessage({ message: 'wrong subject' })).rejects.toThrow(
    'differs from wallet owner',
  );

  const message = 'durable message accounting';
  const context = { subject: userId, requestHash: hashMessage(message) };
  const account = await wallets.getSigningAccountForUser(userId, 'standard', context);
  const signature = await account.signMessage({ message });
  expect(await recoverMessageAddress({ message, signature })).toBe(wallet.address);
  expect(await account.signMessage({ message })).toBe(signature);
  await wallets.logSignature({
    user_id: userId,
    wallet_id: wallet.id,
    address: wallet.address,
    kind: 'message',
    request_hash: context.requestHash,
  });

  const typedData = {
    domain: { name: 'Accounting', version: '1', chainId: 125 },
    types: { Note: [{ name: 'text', type: 'string' }] },
    primaryType: 'Note',
    message: { text: 'durable typed data accounting' },
  } as const;
  const typedAccount = await wallets.getSigningAccountForRow(wallet, undefined, {
    subject: userId,
    requestHash: 'typed-request',
  });
  const typedSignature = await typedAccount.signTypedData(typedData);
  expect(await recoverTypedDataAddress({ ...typedData, signature: typedSignature })).toBe(
    wallet.address,
  );
  await wallets.logSignature({
    user_id: userId,
    wallet_id: wallet.id,
    address: wallet.address,
    kind: 'typed_data',
    request_hash: 'typed-request',
  });
  await expect(
    typedAccount.signTypedData({ ...typedData, message: { text: 'retargeted' } }),
  ).rejects.toThrow('cannot be retargeted');
  const rows = await database.query(
    `select kind, value_wei::text from wallet_signatures where user_id = $1 order by id`,
    [userId],
  );
  expect(rows.rows).toEqual([
    { kind: 'message', value_wei: '0' },
    { kind: 'typed_data', value_wei: '0' },
  ]);
  await expect(
    wallets.logSignature({
      user_id: userId,
      wallet_id: wallet.id,
      address: wallet.address,
      kind: 'message',
      request_hash: 'missing-reservation',
    }),
  ).rejects.toThrow('no matching durable signature');
}

test(signatureLedgerWriteLossSurvivesRestart, signatureLedgerWriteLossSurvivesRestart, 30_000);
test(signatureLedgerCountsAllSigningKinds, signatureLedgerCountsAllSigningKinds, 30_000);
