import { setTimeout } from 'node:timers/promises';
import type { Pool } from 'pg';

export async function waitForRateLimitWindow(pool: Pool): Promise<void> {
  for (;;) {
    const startedAt = performance.now();
    const { rows } = await pool.query<{ remaining_ms: number }>(
      `select (extract(epoch from (
         date_trunc('minute', now()) + interval '1 minute' - now()
       )) * 1000)::double precision as remaining_ms`
    );
    const remainingMs = rows[0]!.remaining_ms;
    if (remainingMs - (performance.now() - startedAt) >= 15_000) return;
    await setTimeout(Math.ceil(remainingMs));
  }
}
