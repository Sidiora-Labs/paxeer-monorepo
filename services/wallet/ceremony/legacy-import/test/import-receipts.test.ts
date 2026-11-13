import { randomBytes, randomUUID } from 'node:crypto';
import { existsSync, mkdtempSync, readFileSync, rmSync } from 'node:fs';
import { createRequire } from 'node:module';
import { tmpdir } from 'node:os';
import { join } from 'node:path';
import { afterAll, beforeAll, expect, test } from 'vitest';
import type { FastifyInstance } from '../../../gateway/node_modules/fastify/fastify.js';
import { startPostgres, type EphemeralPostgres } from '../../../gateway/test/support/postgres.js';
import {
  Journal,
  applyPlans,
  planAll,
  type LegacyWallet,
  type WalletPlan,
} from '../src/importer.js';

const requireGateway = createRequire(new URL('../../../gateway/package.json', import.meta.url));
const Fastify: typeof import('../../../gateway/node_modules/fastify/fastify.js').default =
  requireGateway('fastify');
const token = randomBytes(32).toString('hex');
const receipts: { status: number; body: string }[] = [];
let damage: 'none' | 'missing' | 'wallet' | 'custody' | 'hash' | 'tag' | 'json' = 'none';
let database: EphemeralPostgres;
let pool: typeof import('../../../gateway/src/db/pool.js');
let app: FastifyInstance;
let url: string;
let scratch: string;

beforeAll(async () => {
  process.env.CORS_ORIGINS = 'http://127.0.0.1';
  process.env.SUPABASE_URL = 'http://127.0.0.1';
  process.env.WALLET_MASTER_KEY = randomBytes(32).toString('base64');
  process.env.GATEWAY_ADMIN_TOKEN = token;
  process.env.ATTESTOR_ENDPOINTS = '';
  database = await startPostgres();
  pool = await import('../../../gateway/src/db/pool.js');
  const { adminRoutes } = await import('../../../gateway/src/routes/admin.js');
  app = Fastify({ logger: false });
  app.addHook('onSend', async (_request, reply, payload) => {
    if (typeof payload !== 'string') throw new Error('expected the real JSON route response');
    receipts.push({ status: reply.statusCode, body: payload });
    if (damage === 'none') return payload;
    if (damage === 'json') return payload.slice(0, -1);
    const body = JSON.parse(payload);
    if (damage === 'missing') delete body.record_sha256;
    if (damage === 'wallet') body.wallet_id = randomUUID();
    if (damage === 'custody') body.custody = body.custody === 'live' ? 'archived' : 'live';
    if (damage === 'hash') body.record_sha256 = '0'.repeat(64);
    if (damage === 'tag') body.error = 'already_imported';
    return JSON.stringify(body);
  });
  await app.register(adminRoutes);
  url = await app.listen({ host: '127.0.0.1', port: 0 });
  scratch = mkdtempSync(join(tmpdir(), 'repair-22-31-369-'));
});

afterAll(async () => {
  await app?.close();
  await pool?.closePool();
  await database?.stop();
  if (scratch) rmSync(scratch, { recursive: true, force: true });
});

function legacyWallet(kind: 'standard' | 'funded' = 'standard'): LegacyWallet {
  return {
    id: randomUUID(),
    user_id: randomUUID(),
    address: `0x${randomBytes(20).toString('hex').toUpperCase()}`,
    encrypted_private_key: randomBytes(48).toString('base64'),
    key_version: 1,
    chain_id: 125,
    kind,
    funded: kind === 'funded',
    is_disabled: false,
    disabled_reason: null,
    created_at: '2025-01-02T03:04:05.000Z',
  };
}

function plans(wallets: LegacyWallet[]): WalletPlan[] {
  const result = planAll(wallets, 125, new Set());
  expect(result.every((plan) => plan.record !== null)).toBe(true);
  return result;
}

function newJournal(): Journal {
  return new Journal(join(scratch, `${randomUUID()}.jsonl`));
}

function apply(wallets: LegacyWallet[], journal: Journal) {
  return applyPlans(plans(wallets), { url, token }, journal);
}

async function stored(walletId: string) {
  return (await pool.query('select * from wallets where id = $1', [walletId])).rows;
}

async function legacy_conflict_success_insert_and_identical_replay() {
  const standard = legacyWallet();
  const funded = legacyWallet('funded');
  standard.encrypted_private_key = ` ${standard.encrypted_private_key} `;
  funded.is_disabled = true;
  funded.disabled_reason = 'archived by operator';
  const wallets = [standard, funded];
  const journal = newJournal();
  expect((await apply(wallets, journal)).map((result) => result.outcome)).toEqual([
    'imported',
    'archived',
  ]);
  for (const wallet of wallets) {
    const [row] = await stored(wallet.id);
    expect(row).toMatchObject({
      id: wallet.id,
      user_id: wallet.user_id,
      address: wallet.address.toLowerCase(),
      encrypted_private_key: wallet.encrypted_private_key,
      key_version: wallet.key_version,
      chain_id: wallet.chain_id,
      kind: wallet.kind,
      binding_state: 'unbound',
      attestor_key_id: `wallet:${wallet.id}:secp256k1`,
      is_disabled: wallet.is_disabled,
      disabled_reason: wallet.disabled_reason,
      created_at: new Date(wallet.created_at),
    });
    expect(row.archived_at !== null).toBe(wallet.funded);
    expect(journal.ids.has(wallet.id)).toBe(true);
  }
  const inserted = receipts.slice(-2);
  expect(inserted.map((receipt) => receipt.status)).toEqual([201, 201]);
  for (const receipt of inserted) {
    expect(JSON.parse(receipt.body)).toMatchObject({
      status: 'inserted',
      record_sha256: expect.stringMatching(/^[0-9a-f]{64}$/),
    });
    expect(receipt.body).not.toContain(standard.encrypted_private_key!);
    expect(receipt.body).not.toContain(funded.encrypted_private_key!);
  }
  const replayJournal = newJournal();
  expect((await apply(wallets, replayJournal)).map((result) => result.outcome)).toEqual([
    'already_imported',
    'already_imported',
  ]);
  expect(receipts.slice(-2).map((receipt) => receipt.status)).toEqual([409, 409]);
  for (const [index, receipt] of receipts.slice(-2).entries()) {
    const original = JSON.parse(inserted[index]!.body);
    expect(JSON.parse(receipt.body)).toEqual({
      error: 'identical_replay',
      wallet_id: original.wallet_id,
      custody: original.custody,
      record_sha256: original.record_sha256,
    });
  }
  const reopened = new Journal(replayJournal.path);
  expect([...reopened.ids]).toEqual(wallets.map((wallet) => wallet.id));
  const posts = receipts.length;
  expect(
    (await applyPlans(planAll(wallets, 125, reopened.ids), { url, token }, reopened)).map(
      (result) => result.outcome
    )
  ).toEqual(['already_imported', 'already_imported']);
  expect(receipts).toHaveLength(posts);
  expect(readFileSync(replayJournal.path, 'utf8')).not.toContain(standard.encrypted_private_key!);
}

test(
  legacy_conflict_success_insert_and_identical_replay.name,
  legacy_conflict_success_insert_and_identical_replay
);

async function legacy_conflict_success_retains_identity_owner_and_material_conflicts() {
  const original = legacyWallet();
  expect((await apply([original], newJournal()))[0]!.outcome).toBe('imported');
  const before = await stored(original.id);
  const ownerCollision = { ...original, id: randomUUID(), address: legacyWallet().address };
  const collisions: LegacyWallet[] = [
    { ...original, id: randomUUID(), user_id: randomUUID() },
    ownerCollision,
    { ...original, address: legacyWallet().address },
    { ...original, user_id: randomUUID() },
    { ...original, encrypted_private_key: randomBytes(48).toString('base64') },
    { ...original, key_version: 2 },
    { ...original, kind: 'funded', funded: true },
    { ...original, created_at: '2025-01-02T03:04:06.000Z' },
    { ...original, is_disabled: true },
    { ...original, disabled_reason: 'operator hold' },
  ];
  const journals: Journal[] = [];
  for (const wallet of collisions) {
    const journal = newJournal();
    journals.push(journal);
    const [result] = await apply([wallet], journal);
    expect(result!.outcome).toBe('failed');
    expect(result!.reason).toContain('409');
    expect(receipts.at(-1)).toEqual({
      status: 409,
      body: JSON.stringify({ error: 'wallet_conflict' }),
    });
    expect(journal.ids.has(wallet.id)).toBe(false);
    expect(existsSync(journal.path)).toBe(false);
    const reopened = new Journal(journal.path);
    expect(planAll([wallet], 125, reopened.ids)[0]!.action).toBe(
      wallet.funded ? 'archive' : 'import'
    );
    if (wallet.id !== original.id) expect(await stored(wallet.id)).toHaveLength(0);
    expect(await stored(original.id)).toEqual(before);
  }
  for (const mutation of [
    "binding_state = 'pending'",
    "attestor_key_id = 'different-key'",
    'archived_at = now()',
    'chain_id = 1',
  ]) {
    await pool.query(`update wallets set ${mutation} where id = $1`, [original.id]);
    const journal = newJournal();
    expect((await apply([original], journal))[0]!.outcome).toBe('failed');
    expect(journal.ids.size).toBe(0);
    expect(existsSync(journal.path)).toBe(false);
    await pool.query(
      "update wallets set binding_state = 'unbound', attestor_key_id = $2, archived_at = null, chain_id = 125 where id = $1",
      [original.id, `wallet:${original.id}:secp256k1`]
    );
  }
  await pool.query('delete from wallets where id = $1', [original.id]);
  const retry = new Journal(journals[1]!.path);
  expect((await apply([ownerCollision], retry))[0]!.outcome).toBe('imported');
  expect(new Journal(retry.path).ids.has(ownerCollision.id)).toBe(true);
  expect(await stored(ownerCollision.id)).toHaveLength(1);
}

test(
  legacy_conflict_success_retains_identity_owner_and_material_conflicts.name,
  legacy_conflict_success_retains_identity_owner_and_material_conflicts
);

async function legacy_conflict_success_requires_verified_receipts() {
  const wallet = legacyWallet();
  const journal = newJournal();
  try {
    damage = 'missing';
    expect((await apply([wallet], journal))[0]!.outcome).toBe('failed');
    expect(receipts.at(-1)!.status).toBe(201);
    expect(await stored(wallet.id)).toHaveLength(1);
    expect(existsSync(journal.path)).toBe(false);
    for (const corruption of ['missing', 'wallet', 'custody', 'hash', 'tag', 'json'] as const) {
      damage = corruption;
      expect((await apply([wallet], journal))[0]!.outcome).toBe('failed');
      expect(receipts.at(-1)!.status).toBe(409);
      expect(journal.ids.size).toBe(0);
      expect(existsSync(journal.path)).toBe(false);
    }
  } finally {
    damage = 'none';
  }
  expect((await apply([wallet], new Journal(journal.path)))[0]!.outcome).toBe('already_imported');
  expect(new Journal(journal.path).ids.has(wallet.id)).toBe(true);
}

test(
  legacy_conflict_success_requires_verified_receipts.name,
  legacy_conflict_success_requires_verified_receipts
);
