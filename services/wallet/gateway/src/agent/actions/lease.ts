import type { ActionRow } from '../../db/actions.js';
import { query } from '../../db/pool.js';

/** Each atomic claim increments attempts, fencing older holders without a schema change. */
export async function renewActionLease(row: ActionRow, leaseMs: number): Promise<boolean> {
  const result = await query(
    `update agent_actions
       set worker_locked_until = clock_timestamp() + ($3 || ' milliseconds')::interval
       where id = $1 and attempts = $2 and worker_locked_until > clock_timestamp()
       returning id`,
    [row.id, row.attempts, String(leaseMs)]
  );
  return result.rowCount === 1;
}

export async function releaseActionLease(row: ActionRow): Promise<void> {
  await query(
    `update agent_actions set worker_locked_until = null, updated_at = now()
       where id = $1 and attempts = $2`,
    [row.id, row.attempts]
  );
}
