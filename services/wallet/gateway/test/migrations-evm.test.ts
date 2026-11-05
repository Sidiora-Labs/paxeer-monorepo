import { randomUUID } from 'node:crypto';
import { readdirSync, readFileSync } from 'node:fs';
import { Pool } from 'pg';
import { PGlite, pgcrypto, PGLiteSocketServer } from '../src/db/testDatabase.js';
import { expect, test } from 'vitest';

const wallet_evm_approvals = 'wallet_evm_approvals';

const migrationsDirectory = new URL('../migrations/', import.meta.url);
const migrationFiles = readdirSync(migrationsDirectory)
  .filter((name) => name.endsWith('.sql'))
  .sort();

async function qualifyEvmMigrations(files: string[]): Promise<void> {
  const db = await PGlite.create({ extensions: { pgcrypto } });
  const socket = new PGLiteSocketServer({ db, host: '127.0.0.1', port: 0 });
  await socket.start();
  const pool = new Pool({
    connectionString: `postgres://postgres:postgres@${socket.getServerConn()}/postgres`,
  });
  try {
    await pool.query('create table _migrations (filename text primary key)');
    for (const filename of files) {
      await pool.query(readFileSync(new URL(filename, migrationsDirectory), 'utf8'));
      await pool.query('insert into _migrations values ($1)', [filename]);
    }
    const applied = await pool.query<{ filename: string }>(
      'select filename from _migrations order by filename'
    );
    expect(applied.rows.map((row) => row.filename)).toEqual(files);
    const columns = async (table: string) =>
      pool.query(
        `select column_name, data_type, is_nullable, column_default
       from information_schema.columns
       where table_schema = 'public' and table_name = $1 order by ordinal_position`,
        [table]
      );
    const evmColumns = await columns(wallet_evm_approvals);
    const lxColumns = await columns('wallet_lx_approvals');
    expect(evmColumns.rows.map((row) => row.column_name)).toEqual([
      'id',
      'wallet_id',
      'principal',
      'session_id',
      'state',
      'artifact',
      'review_evidence',
      'approval',
      'evidence',
      'expires_at',
      'created_at',
      'approved_at',
      'attempted_at',
      'completed_at',
    ]);
    expect(evmColumns.rows).toEqual(lxColumns.rows);
    const indexes = await pool.query<{ indexdef: string }>(
      'select indexdef from pg_indexes where schemaname = $1 and tablename = $2',
      ['public', wallet_evm_approvals]
    );
    expect(indexes.rows.some((row) => /\(principal, state\)/.test(row.indexdef))).toBe(true);

    const principal = randomUUID();
    const walletId = randomUUID();
    await pool.query(
      `insert into wallets (id, user_id, address, attestor_key_id)
       values ($1, $2, $3, $4)`,
      [walletId, principal, '0x0000000000000000000000000000000000000001', randomUUID()]
    );
    const insert = `insert into wallet_evm_approvals
      (id, wallet_id, principal, session_id, state, artifact, review_evidence,
       approval, evidence, expires_at, approved_at, attempted_at, completed_at)
      values ($1, $2, $3, $4, $5, '{}'::jsonb, '[{},{},{}]'::jsonb,
       case when $5 = 'reviewed' then null else '{}'::jsonb end, $6::jsonb,
       now() + interval '1 hour',
       case when $5 = 'reviewed' then null else now() end,
       case when $5 in ('signing_unknown', 'signed') then now() else null end,
       case when $5 = 'signed' then now() else null end)`;
    const rejectedId = randomUUID();
    // Exercise rejection on the same PostgreSQL engine without retiring the
    // socket bridge's only connection after an expected query error.
    await expect(
      db.query(insert, [rejectedId, walletId, principal, randomUUID(), 'signed', null])
    ).rejects.toMatchObject({
      code: '23514',
      constraint: 'wallet_evm_approvals_signed_evidence',
    });
    const rejected = await pool.query('select id from wallet_evm_approvals where id = $1', [
      rejectedId,
    ]);
    expect(rejected.rowCount).toBe(0);

    for (const state of ['reviewed', 'approved', 'signing_unknown', 'signed']) {
      const id = randomUUID();
      const evidence = state === 'signed' ? { sessionId: randomUUID() } : null;
      await pool.query(insert, [id, walletId, principal, randomUUID(), state, evidence]);
      const saved = await pool.query(
        'select state, evidence from wallet_evm_approvals where id = $1',
        [id]
      );
      expect(saved.rows).toEqual([{ state, evidence }]);
    }
    await expect(
      db.query(insert, [randomUUID(), walletId, principal, randomUUID(), 'invalid', null])
    ).rejects.toMatchObject({ code: '23514' });
  } finally {
    await pool.end();
    await socket.stop();
    await db.close();
  }
}

export async function evmApprovalsTableAppliesAfter016(): Promise<void> {
  const prefix = migrationFiles.filter((name) => name <= '017_wallet_evm_approvals.sql');
  expect(prefix.slice(-2)).toEqual([
    '016_nonce_outstanding_hash.sql',
    '017_wallet_evm_approvals.sql',
  ]);
  expect(prefix).toEqual([
    '001_init.sql',
    '002_funded_accounts.sql',
    '003_dedupe_whitelist.sql',
    '004_agent_native.sql',
    '005_agent_actions.sql',
    '006_agent_request_nonces.sql',
    '006_archive_funded.sql',
    '006_attestor_signing.sql',
    '007_unified_account.sql',
    '008_custody_authority.sql',
    '009_custody_signing_reservations.sql',
    '010_custody_authorizations.sql',
    '011_wallet_custody_submissions.sql',
    '012_wallet_lx_approvals.sql',
    '013_wallet_custody_authorizations.sql',
    '014_nonce_outstanding.sql',
    '015_nonce_outstanding_seed.sql',
    '016_nonce_outstanding_hash.sql',
    '017_wallet_evm_approvals.sql',
  ]);
  await qualifyEvmMigrations(prefix);
}

export async function currentEvmApprovalsMigrationChain(): Promise<void> {
  expect(
    migrationFiles.filter((name) => name > '017_wallet_evm_approvals.sql').length
  ).toBeGreaterThan(0);
  await qualifyEvmMigrations(migrationFiles);
}

test(evmApprovalsTableAppliesAfter016.name, evmApprovalsTableAppliesAfter016, 30_000);
test(currentEvmApprovalsMigrationChain.name, currentEvmApprovalsMigrationChain, 30_000);
