import { randomBytes, randomUUID } from 'node:crypto';
import { existsSync, mkdtempSync, readFileSync, rmSync } from 'node:fs';
import { createRequire } from 'node:module';
import { tmpdir } from 'node:os';
import { join } from 'node:path';
import pg from 'pg';
import { afterAll, beforeAll, describe, expect, it, test } from 'vitest';
import type { FastifyInstance } from '../../../gateway/node_modules/fastify/fastify.js';
import { startPostgres, type EphemeralPostgres } from '../../../gateway/test/support/postgres.js';
import {
  Journal,
  applyPlans,
  planAll,
  readLegacyWallets,
  reportLine,
  summary,
  type LegacyWallet,
} from '../src/importer.js';

const W1 = '00000000-0000-4000-8000-000000000001';
const W2 = '00000000-0000-4000-8000-000000000002';
const W3 = '00000000-0000-4000-8000-000000000003';
const TOKEN = randomBytes(32).toString('hex');
const requireGateway = createRequire(new URL('../../../gateway/package.json', import.meta.url));
const Fastify: typeof import('../../../gateway/node_modules/fastify/fastify.js').default =
  requireGateway('fastify');
const receipts: { status: number; body: string }[] = [];
let database: EphemeralPostgres;
let destination: typeof import('../../../gateway/src/db/pool.js');
let pool: pg.Pool;
let gateway: FastifyInstance;
let gatewayUrl: string;
let scratch: string;

beforeAll(async () => {
  process.env.CORS_ORIGINS = 'http://127.0.0.1';
  process.env.SUPABASE_URL = 'http://127.0.0.1';
  process.env.WALLET_MASTER_KEY = randomBytes(32).toString('base64');
  process.env.GATEWAY_ADMIN_TOKEN = TOKEN;
  process.env.HYPERPAXEER_CHAIN_ID = '125';
  process.env.ATTESTOR_ENDPOINTS = '';
  database = await startPostgres();
  destination = await import('../../../gateway/src/db/pool.js');
  await destination.query('create database legacy_source');
  const sourceUrl = new URL(database.url);
  sourceUrl.pathname = '/legacy_source';
  pool = new pg.Pool({ connectionString: sourceUrl.toString() });
  await pool.query(readFileSync(new URL('legacy-schema.sql', import.meta.url), 'utf8'));

  const { adminRoutes } = await import('../../../gateway/src/routes/admin.js');
  gateway = Fastify({ logger: false });
  gateway.addHook('onSend', async (_request, reply, payload) => {
    if (typeof payload !== 'string') throw new Error('expected the real JSON route response');
    receipts.push({ status: reply.statusCode, body: payload });
    return payload;
  });
  await gateway.register(adminRoutes);
  gatewayUrl = await gateway.listen({ host: '127.0.0.1', port: 0 });
  scratch = mkdtempSync(join(tmpdir(), 'legacy-import-journal-'));
});

afterAll(async () => {
  await gateway?.close();
  await pool?.end();
  await destination?.closePool();
  await database?.stop();
  if (scratch) rmSync(scratch, { recursive: true, force: true });
});

async function storedWallets() {
  return (await destination.query('select * from wallets order by id')).rows;
}

async function replaceSourceWallet(currentId: string, wallet: LegacyWallet) {
  const result = await pool.query(
    `update wallets set id = $2, user_id = $3, address = $4, encrypted_private_key = $5
      where id = $1`,
    [currentId, wallet.id, wallet.user_id, wallet.address, wallet.encrypted_private_key]
  );
  expect(result.rowCount).toBe(1);
}

describe('legacy wallet import', () => {
  it('maps the three legacy rows and reports without key ciphertext', async () => {
    const wallets = await readLegacyWallets(pool);
    expect(wallets.map((w) => w.id)).toEqual([W1, W2, W3]);
    const plans = planAll(wallets, 125, new Set());
    expect(plans.map((p) => p.action)).toEqual(['import', 'archive', 'refuse']);
    expect(plans[0]!.record).toMatchObject({
      legacy_wallet_id: W1,
      address: '0x1111111111111111111111111111111111111111',
      kind: 'standard',
      custody: 'live',
      binding_state: 'unbound',
      attestor_key_id: `wallet:${W1}:secp256k1`,
      identity_key_id: `wallet:${W1}:ed25519`,
      created_at: '2026-05-01T00:00:00.000Z',
    });
    expect(plans[1]!.record).toMatchObject({ kind: 'funded', custody: 'archived' });
    expect(plans[2]!.reason).toBe('chain 1 is not the target chain 125');
    expect(summary(plans)).toEqual({
      total: 3,
      import: 1,
      archive: 1,
      refuse: 1,
      already_imported: 0,
    });
    const report = plans.map(reportLine).join('\n');
    expect(report).not.toContain('synthetic-ciphertext');
    expect(report).toContain('"key_ciphertext":"present"');
  });
});

async function legacy_import_gateway_conflict_roundtrip() {
  const journalPath = join(scratch, 'journal.jsonl');
  const admin = { url: gatewayUrl, token: TOKEN };
  const journal = new Journal(journalPath);
  const wallets = await readLegacyWallets(pool);
  const plans = planAll(wallets, 125, journal.ids);
  expect(await storedWallets()).toEqual([]);
  const first = await applyPlans(plans, admin, journal);
  expect(first.map((r) => r.outcome)).toEqual(['imported', 'archived', 'refused']);
  expect(receipts).toHaveLength(2);
  const inserted = receipts.slice();
  expect(inserted.map((receipt) => receipt.status)).toEqual([201, 201]);
  const committed = await storedWallets();
  expect(committed.map((row) => row.id)).toEqual([W1, W2]);
  for (const [index, row] of committed.entries()) {
    const record = plans[index]!.record!;
    expect(row).toMatchObject({
      id: record.legacy_wallet_id,
      user_id: record.user_id,
      address: record.address,
      encrypted_private_key: record.encrypted_private_key,
      key_version: record.key_version,
      chain_id: record.chain_id,
      kind: record.kind,
      binding_state: 'unbound',
      attestor_key_id: record.attestor_key_id,
      is_disabled: record.is_disabled,
      disabled_reason: record.disabled_reason,
      created_at: new Date(record.created_at),
    });
    if (record.custody === 'archived') expect(row.archived_at).toBeInstanceOf(Date);
    else expect(row.archived_at).toBeNull();
    expect(JSON.parse(inserted[index]!.body)).toEqual({
      status: 'inserted',
      wallet_id: record.legacy_wallet_id,
      custody: record.custody,
      record_sha256: expect.stringMatching(/^[0-9a-f]{64}$/),
    });
    expect(inserted[index]!.body).not.toContain(record.encrypted_private_key);
  }
  expect(committed.some((row) => row.id === W3)).toBe(false);
  expect(committed[0]!.encrypted_private_key).toBe(
    'v1:synthetic-iv-1:synthetic-ciphertext-1:synthetic-tag-1'
  );

  const reopened = new Journal(journalPath);
  expect([...reopened.ids].sort()).toEqual([W1, W2]);
  const again = planAll(await readLegacyWallets(pool), 125, reopened.ids);
  expect(again.map((p) => p.action)).toEqual(['already_imported', 'already_imported', 'refuse']);
  const second = await applyPlans(again, admin, reopened);
  expect(second.map((r) => r.outcome)).toEqual(['already_imported', 'already_imported', 'refused']);
  expect(receipts).toHaveLength(2);

  const freshPath = join(scratch, 'fresh.jsonl');
  const third = await applyPlans(
    planAll(await readLegacyWallets(pool), 125, new Set()),
    admin,
    new Journal(freshPath)
  );
  expect(third.map((r) => r.outcome)).toEqual(['already_imported', 'already_imported', 'refused']);
  expect(receipts).toHaveLength(4);
  for (const [index, receipt] of receipts.slice(2).entries()) {
    const original = JSON.parse(inserted[index]!.body);
    expect(receipt).toEqual({
      status: 409,
      body: JSON.stringify({
        error: 'identical_replay',
        wallet_id: original.wallet_id,
        custody: original.custody,
        record_sha256: original.record_sha256,
      }),
    });
  }
  expect([...new Journal(freshPath).ids].sort()).toEqual([W1, W2]);
  expect(await storedWallets()).toEqual(committed);
  for (const path of [journalPath, freshPath]) {
    const entries = readFileSync(path, 'utf8')
      .trim()
      .split('\n')
      .map((line) => JSON.parse(line));
    expect(entries.map((entry) => [entry.wallet_id, entry.outcome])).toEqual([
      [W1, path === journalPath ? 'imported' : 'already_imported'],
      [W2, path === journalPath ? 'archived' : 'already_imported'],
    ]);
    expect(readFileSync(path, 'utf8')).not.toContain('synthetic-ciphertext');
  }

  const original = wallets[0]!;
  const addressCollision = { ...original, id: randomUUID(), user_id: randomUUID() };
  const ownerCollision = {
    ...original,
    id: randomUUID(),
    address: '0x4444444444444444444444444444444444444444',
  };
  const collisions = [
    addressCollision,
    ownerCollision,
    { ...original, encrypted_private_key: 'v1:changed-iv:changed-ciphertext:changed-tag' },
  ];
  for (const [index, collision] of collisions.entries()) {
    const conflictPath = join(scratch, `conflict-${index}.jsonl`);
    await replaceSourceWallet(W1, collision);
    try {
      for (let attempt = 0; attempt < 2; attempt++) {
        const retryJournal = new Journal(conflictPath);
        expect([...retryJournal.ids]).toEqual([]);
        const source = (await readLegacyWallets(pool)).filter(
          (wallet) => wallet.id === collision.id
        );
        expect(source).toEqual([collision]);
        const retryPlans = planAll(source, 125, retryJournal.ids);
        expect(retryPlans.map((plan) => plan.action)).toEqual(['import']);
        const posts = receipts.length;
        const results = await applyPlans(retryPlans, admin, retryJournal);
        expect(results).toHaveLength(1);
        expect(results[0]!.outcome).toBe('failed');
        expect(results[0]!.reason).toBe('gateway answered 409: {"error":"wallet_conflict"}');
        expect(receipts).toHaveLength(posts + 1);
        expect(receipts.at(-1)).toEqual({
          status: 409,
          body: JSON.stringify({ error: 'wallet_conflict' }),
        });
        expect([...retryJournal.ids]).toEqual([]);
        expect(existsSync(conflictPath)).toBe(false);
        expect([...new Journal(conflictPath).ids]).toEqual([]);
        expect(await storedWallets()).toEqual(committed);
      }
    } finally {
      await replaceSourceWallet(collision.id, original);
    }
  }

  const resolved = { ...addressCollision, address: '0x5555555555555555555555555555555555555555' };
  await replaceSourceWallet(W1, resolved);
  try {
    const retryJournal = new Journal(join(scratch, 'conflict-0.jsonl'));
    expect(retryJournal.ids.has(resolved.id)).toBe(false);
    const source = (await readLegacyWallets(pool)).filter((wallet) => wallet.id === resolved.id);
    expect(source).toEqual([resolved]);
    const posts = receipts.length;
    const results = await applyPlans(planAll(source, 125, retryJournal.ids), admin, retryJournal);
    expect(results.map((result) => result.outcome)).toEqual(['imported']);
    expect(receipts).toHaveLength(posts + 1);
    expect(receipts.at(-1)!.status).toBe(201);
    expect(JSON.parse(receipts.at(-1)!.body)).toEqual({
      status: 'inserted',
      wallet_id: resolved.id,
      custody: 'live',
      record_sha256: expect.stringMatching(/^[0-9a-f]{64}$/),
    });
    const after = await storedWallets();
    expect(after).toHaveLength(3);
    expect(after.filter((row) => row.id !== resolved.id)).toEqual(committed);
    expect(after.find((row) => row.id === resolved.id)).toMatchObject({
      user_id: resolved.user_id,
      address: resolved.address,
      encrypted_private_key: resolved.encrypted_private_key,
      archived_at: null,
    });
    expect([...new Journal(retryJournal.path).ids]).toEqual([resolved.id]);
    expect(JSON.parse(readFileSync(retryJournal.path, 'utf8').trim())).toMatchObject({
      wallet_id: resolved.id,
      outcome: 'imported',
    });
  } finally {
    await replaceSourceWallet(resolved.id, original);
  }
}

test(legacy_import_gateway_conflict_roundtrip.name, legacy_import_gateway_conflict_roundtrip);
