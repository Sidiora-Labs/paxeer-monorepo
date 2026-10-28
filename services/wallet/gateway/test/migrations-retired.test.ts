import { randomUUID } from 'node:crypto';
import { readFileSync } from 'node:fs';
import { expect, test } from 'vitest';
import { startPostgres } from './support/postgres.js';

const layerx_accounts = 'layerx_accounts';
const wallet_custody_submissions = 'wallet_custody_submissions';
const wallet_custody_authorizations = 'wallet_custody_authorizations';

async function retiredTablesDroppedByMigration019(): Promise<void> {
  const postgres = await startPostgres();
  const { getPool, closePool } = await import('../src/db/pool.js');
  const { runMigrations } = await import('../src/db/migrate.js');
  const pool = getPool();
  const retiredTables = [
    layerx_accounts,
    wallet_custody_submissions,
    wallet_custody_authorizations,
  ].sort();
  const migration = '019_drop_retired_layerx_custody.sql';
  const constraint = 'agent_actions_no_layerx_deposit';
  const tableCatalog = `select name, to_regclass('public.' || name)::text as relation
    from unnest($1::text[]) as name order by name`;
  try {
    const freshTables = await pool.query(tableCatalog, [retiredTables]);
    expect(freshTables.rows).toEqual(
      retiredTables.map((name) => ({ name, relation: null })),
    );

    await pool.query(
      'alter table agent_actions drop constraint agent_actions_no_layerx_deposit',
    );
    for (const filename of [
      '005_agent_actions.sql',
      '011_wallet_custody_submissions.sql',
      '013_wallet_custody_authorizations.sql',
    ]) {
      await pool.query(
        readFileSync(
          new URL(`../migrations/${filename}`, import.meta.url),
          'utf8',
        ),
      );
    }
    await pool.query('delete from _migrations where filename = $1', [
      migration,
    ]);
    const beforeTables = await pool.query(tableCatalog, [retiredTables]);
    expect(beforeTables.rows).toEqual(
      retiredTables.map((name) => ({ name, relation: name })),
    );

    const did = `did:matrix:retired:${randomUUID()}`;
    await pool.query(
      `insert into agent_principals (did, label, key_fingerprint, public_key)
       values ($1, 'retired', $2, $3)`,
      [did, '11'.repeat(8), '11'.repeat(32)],
    );
    await pool.query('insert into layerx_accounts (did) values ($1)', [did]);
    const insert = `insert into agent_actions
      (id, did, kind, status, idempotency_key, request, terminal_at)
      values ($1, $2, $3, $4, $1, '{"retirement":true}'::jsonb, $5)`;
    const terminalAt = new Date('2026-01-01T00:00:00.000Z');
    const terminalStates = [
      'confirmed',
      'policy_denied',
      'invalid_request',
      'simulation_reverted',
      'transaction_reverted',
      'insufficient_balance',
      'infrastructure_unavailable',
      'failed_terminal',
    ];
    for (const status of terminalStates) {
      for (const kind of ['layerx_deposit', 'allowance_and_call']) {
        await pool.query(insert, [randomUUID(), did, kind, status, terminalAt]);
      }
    }
    const activeDepositId = randomUUID();
    const activeAllowanceId = randomUUID();
    await pool.query(insert, [
      activeDepositId,
      did,
      'layerx_deposit',
      'received',
      null,
    ]);
    await pool.query(insert, [
      activeAllowanceId,
      did,
      'allowance_and_call',
      'received',
      null,
    ]);
    const beforeActions = await pool.query(
      'select * from agent_actions order by id',
    );
    expect(beforeActions.rowCount).toBe(18);

    await expect(runMigrations()).rejects.toMatchObject({
      code: '23514',
      constraint,
    });
    const blockedActions = await pool.query(
      'select * from agent_actions order by id',
    );
    expect(blockedActions.rows).toEqual(beforeActions.rows);
    const blockedTables = await pool.query(tableCatalog, [retiredTables]);
    expect(blockedTables.rows).toEqual(beforeTables.rows);
    const mirror = await pool.query('select did from layerx_accounts');
    expect(mirror.rows).toEqual([{ did }]);
    const unapplied = await pool.query(
      'select filename from _migrations where filename = $1',
      [migration],
    );
    expect(unapplied.rows).toEqual([]);

    await pool.query(
      `update agent_actions set status = 'confirmed', terminal_at = $2 where id = $1`,
      [activeDepositId, terminalAt],
    );
    await runMigrations();

    const afterTables = await pool.query(tableCatalog, [retiredTables]);
    expect(afterTables.rows).toEqual(freshTables.rows);
    const afterActions = await pool.query(
      'select * from agent_actions order by id',
    );
    expect(afterActions.rows).toEqual(
      beforeActions.rows.filter((row) => row.kind === 'allowance_and_call'),
    );
    expect(afterActions.rowCount).toBe(9);
    const validated = await pool.query(
      `select convalidated from pg_constraint
       where conrelid = 'agent_actions'::regclass and conname = $1`,
      [constraint],
    );
    expect(validated.rows).toEqual([{ convalidated: true }]);

    for (const [status, terminal] of [
      ['received', null],
      ['confirmed', terminalAt],
    ] as const) {
      const id = randomUUID();
      await expect(
        pool.query(insert, [id, did, 'layerx_deposit', status, terminal]),
      ).rejects.toMatchObject({
        code: '23514',
        constraint,
      });
      const rejected = await pool.query(
        'select id from agent_actions where id = $1',
        [id],
      );
      expect(rejected.rows).toEqual([]);
    }
    await expect(
      pool.query(
        `update agent_actions set kind = 'layerx_deposit' where id = $1`,
        [activeAllowanceId],
      ),
    ).rejects.toMatchObject({ code: '23514', constraint });

    const allowedId = randomUUID();
    await pool.query(insert, [
      allowedId,
      did,
      'allowance_and_call',
      'received',
      null,
    ]);
    const allowed = await pool.query(
      'select kind from agent_actions where id = $1',
      [allowedId],
    );
    expect(allowed.rows).toEqual([{ kind: 'allowance_and_call' }]);

    await runMigrations();
    const applied = await pool.query(
      'select filename from _migrations where filename = $1',
      [migration],
    );
    expect(applied.rows).toEqual([{ filename: migration }]);
  } finally {
    try {
      await closePool();
    } finally {
      await postgres.stop();
    }
  }
}

test(
  retiredTablesDroppedByMigration019,
  retiredTablesDroppedByMigration019,
  120_000,
);
