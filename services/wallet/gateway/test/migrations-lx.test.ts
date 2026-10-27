import { randomUUID } from 'node:crypto';
import { readFileSync } from 'node:fs';
import { expect, test } from 'vitest';
import { startPostgres } from './support/postgres.js';

const wallet_lx_approvals = 'wallet_lx_approvals';

async function lxActivityOpenApprovalsRemovedByMigration018(): Promise<void> {
  const postgres = await startPostgres();
  const { getPool, closePool } = await import('../src/db/pool.js');
  const { runMigrations } = await import('../src/db/migrate.js');
  const pool = getPool();
  try {
    await pool.query(`drop table ${wallet_lx_approvals}`);
    await pool.query(
      readFileSync(
        new URL('../migrations/012_wallet_lx_approvals.sql', import.meta.url),
        'utf8',
      ),
    );
    const migration = '018_wallet_lx_approvals_send_only.sql';
    await pool.query('delete from _migrations where filename = $1', [
      migration,
    ]);

    const walletId = randomUUID();
    const principal = randomUUID();
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
    const insert = `insert into wallet_lx_approvals
      (id, wallet_id, principal, session_id, state, artifact, review_evidence,
       approval, evidence, expires_at, approved_at, attempted_at, completed_at)
      values ($1, $2, $3, $4, $5, $6::jsonb, '[{},{},{}]'::jsonb,
       case when $5 = 'reviewed' then null else '{"consent":true}'::jsonb end,
       case when $5 = 'signed' then '{"signature":"retained"}'::jsonb else null end,
       now() + interval '600 seconds',
       case when $5 = 'reviewed' then null else now() end,
       case when $5 in ('signing_unknown','signed') then now() else null end,
       case when $5 = 'signed' then now() else null end)`;
    const states = [
      'reviewed',
      'approved',
      'signing_unknown',
      'signed',
    ] as const;
    for (const kind of ['lx_activity', 'lx_send_authorization']) {
      for (const state of states) {
        await pool.query(insert, [
          randomUUID(),
          walletId,
          principal,
          randomUUID(),
          state,
          { kind },
        ]);
      }
    }
    const before = await pool.query(
      `select * from ${wallet_lx_approvals} order by id`,
    );
    expect(before.rowCount).toBe(8);

    await runMigrations();

    const applied = await pool.query(
      'select filename from _migrations where filename = $1',
      [migration],
    );
    expect(applied.rows).toEqual([{ filename: migration }]);
    const after = await pool.query(
      `select * from ${wallet_lx_approvals} order by id`,
    );
    expect(after.rows).toEqual(
      before.rows.filter(
        (row) =>
          row.artifact.kind === 'lx_send_authorization' ||
          row.state === 'signed' ||
          row.state === 'signing_unknown',
      ),
    );
    expect(after.rowCount).toBe(6);
    const openActivities = await pool.query(
      `select id from wallet_lx_approvals
       where state in ('reviewed','approved') and artifact->>'kind' = 'lx_activity'`,
    );
    expect(openActivities.rows).toEqual([]);
    const constraint = await pool.query(
      `select convalidated from pg_constraint
       where conrelid = $1::regclass and conname = $2`,
      [wallet_lx_approvals, 'wallet_lx_approvals_send_only'],
    );
    expect(constraint.rows).toEqual([{ convalidated: true }]);

    for (const state of ['reviewed', 'approved']) {
      const id = randomUUID();
      await expect(
        pool.query(insert, [
          id,
          walletId,
          principal,
          randomUUID(),
          state,
          { kind: 'lx_activity' },
        ]),
      ).rejects.toMatchObject({
        code: '23514',
        constraint: 'wallet_lx_approvals_send_only',
      });
      const rejected = await pool.query(
        'select id from wallet_lx_approvals where id = $1',
        [id],
      );
      expect(rejected.rows).toEqual([]);
    }
    for (const state of states) {
      const id = randomUUID();
      await pool.query(insert, [
        id,
        walletId,
        principal,
        randomUUID(),
        state,
        { kind: 'lx_send_authorization' },
      ]);
      const saved = await pool.query(
        'select state, artifact from wallet_lx_approvals where id = $1',
        [id],
      );
      expect(saved.rows).toEqual([
        { state, artifact: { kind: 'lx_send_authorization' } },
      ]);
    }
  } finally {
    try {
      await closePool();
    } finally {
      await postgres.stop();
    }
  }
}

test(
  lxActivityOpenApprovalsRemovedByMigration018,
  lxActivityOpenApprovalsRemovedByMigration018,
  120_000,
);
