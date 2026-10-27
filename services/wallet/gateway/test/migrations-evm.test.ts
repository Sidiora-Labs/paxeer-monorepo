import { randomUUID } from 'node:crypto';
import { readdirSync } from 'node:fs';
import { expect, test } from 'vitest';
import { startPostgres } from './support/postgres.js';

const wallet_evm_approvals = 'wallet_evm_approvals';

async function evmApprovalsTableAppliesAfter016(): Promise<void> {
  const postgres = await startPostgres();
  const { getPool, closePool } = await import('../src/db/pool.js');
  const pool = getPool();
  try {
    const migrations = readdirSync(new URL('../migrations/', import.meta.url))
      .filter((filename) => filename.endsWith('.sql'))
      .sort();
    const applied = await pool.query<{ filename: string }>(
      'select filename from _migrations order by filename',
    );
    expect(applied.rows.map((row) => row.filename)).toEqual(migrations);
    expect(migrations.slice(-2)).toEqual([
      '016_nonce_outstanding_hash.sql',
      '017_wallet_evm_approvals.sql',
    ]);

    const columns = async (table: string) =>
      pool.query(
        `select column_name, data_type, is_nullable, column_default
       from information_schema.columns
       where table_schema = 'public' and table_name = $1 order by ordinal_position`,
        [table],
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
      ['public', wallet_evm_approvals],
    );
    expect(
      indexes.rows.some((row) => /\(principal, state\)/.test(row.indexdef)),
    ).toBe(true);

    const principal = randomUUID();
    const walletId = randomUUID();
    await pool.query(
      `insert into wallets (id, user_id, address, attestor_key_id)
       values ($1, $2, $3, $4)`,
      [
        walletId,
        principal,
        '0x0000000000000000000000000000000000000001',
        randomUUID(),
      ],
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
    await expect(
      pool.query(insert, [
        rejectedId,
        walletId,
        principal,
        randomUUID(),
        'signed',
        null,
      ]),
    ).rejects.toMatchObject({
      code: '23514',
      constraint: 'wallet_evm_approvals_signed_evidence',
    });
    const rejected = await pool.query(
      'select id from wallet_evm_approvals where id = $1',
      [rejectedId],
    );
    expect(rejected.rowCount).toBe(0);

    for (const state of ['reviewed', 'approved', 'signing_unknown', 'signed']) {
      const id = randomUUID();
      const evidence = state === 'signed' ? { sessionId: randomUUID() } : null;
      await pool.query(insert, [
        id,
        walletId,
        principal,
        randomUUID(),
        state,
        evidence,
      ]);
      const saved = await pool.query(
        'select state, evidence from wallet_evm_approvals where id = $1',
        [id],
      );
      expect(saved.rows).toEqual([{ state, evidence }]);
    }
    await expect(
      pool.query(insert, [
        randomUUID(),
        walletId,
        principal,
        randomUUID(),
        'invalid',
        null,
      ]),
    ).rejects.toMatchObject({ code: '23514' });
  } finally {
    try {
      await closePool();
    } finally {
      await postgres.stop();
    }
  }
}

test(
  evmApprovalsTableAppliesAfter016,
  evmApprovalsTableAppliesAfter016,
  120_000,
);
