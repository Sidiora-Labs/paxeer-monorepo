import type { FastifyBaseLogger } from 'fastify';
import { performance } from 'node:perf_hooks';
import { env } from '../../env.js';
import { claimNextAction, type ActionRow } from '../../db/actions.js';
import { advanceAction } from './orchestrator.js';
import { ActionExecution } from './execution.js';
import { releaseActionLease, renewActionLease } from './lease.js';

/**
 * Background action worker + reconciler (one loop, both jobs).
 *
 * Because `agent_actions` is durable and `claimNextAction` selects ANY
 * non-terminal row whose lease has expired, this single loop provides BOTH:
 *   - forward progress for freshly-submitted actions, and
 *   - persistent reconciliation after a process restart / crash (on boot it
 *     simply finds the orphaned in-flight rows and resumes them — the tx hashes
 *     and nonces were persisted, so it never blind-resends).
 *
 * `SELECT ... FOR UPDATE SKIP LOCKED` + a per-row lease means N workers across
 * N processes cooperate safely: each grabs a different action, and a crashed
 * worker's lease expires so its action is picked up again.
 *
 * The loop is intentionally serial per tick (claim one, advance it, repeat a
 * bounded number of times) to keep RPC pressure predictable; scale by running
 * more workers, not by fanning out unbounded concurrency here.
 */

export interface WorkerHandle {
  stop: () => Promise<void>;
}

const MAX_PER_TICK = 8;

export function startActionWorker(log: FastifyBaseLogger): WorkerHandle {
  let stopped = false;
  let timer: NodeJS.Timeout | null = null;
  let running: Promise<void> = Promise.resolve();

  const leaseMs = Math.max(env.ACTION_RECEIPT_TIMEOUT_MS, env.ACTION_WORKER_INTERVAL_MS * 4);

  const advance = async (row: ActionRow, claimedAt: number): Promise<void> => {
    const execution = new ActionExecution(claimedAt, leaseMs, leaseMs * 4);
    let finished = false;
    let renewalTimer: NodeJS.Timeout | undefined;
    let renewing: Promise<void> = Promise.resolve();
    const fail = (err: unknown): void =>
      execution.cancel(err instanceof Error ? err : new Error(String(err)));
    const renew = async (): Promise<void> => {
      try {
        execution.assertActive();
        const started = performance.now();
        if (!(await renewActionLease(row, leaseMs)))
          throw new Error('action lease renewal lost ownership');
        execution.renew(started, leaseMs);
        if (!finished)
          renewalTimer = setTimeout(scheduleRenewal, Math.max(1, Math.floor(leaseMs / 3)));
      } catch (err) {
        fail(err);
      }
    };
    const scheduleRenewal = (): void => {
      renewing = renew();
    };
    const watchdog = setInterval(
      () => {
        try {
          execution.assertActive();
        } catch (err) {
          fail(err);
        }
      },
      Math.max(1, Math.floor(leaseMs / 10))
    );
    renewalTimer = setTimeout(scheduleRenewal, Math.max(1, Math.floor(leaseMs / 3)));
    try {
      await execution.run(() => advanceAction(row));
      execution.assertActive();
    } finally {
      finished = true;
      clearInterval(watchdog);
      clearTimeout(renewalTimer);
      await renewing;
      await releaseActionLease(row);
    }
  };

  const tick = async (): Promise<void> => {
    if (stopped) return;
    try {
      for (let i = 0; i < MAX_PER_TICK && !stopped; i++) {
        const claimedAt = performance.now();
        const row = await claimNextAction(leaseMs);
        if (!row) break;
        if (stopped) {
          await releaseActionLease(row);
          break;
        }
        await advance(row, claimedAt);
      }
    } catch (err) {
      log.warn({ err: (err as Error).message }, '[action-worker] tick error');
    } finally {
      if (!stopped) timer = setTimeout(startTick, env.ACTION_WORKER_INTERVAL_MS);
    }
  };

  const startTick = (): void => {
    running = tick();
  };

  log.info('[action-worker] started');
  timer = setTimeout(startTick, env.ACTION_WORKER_INTERVAL_MS);

  return {
    stop: async () => {
      stopped = true;
      if (timer) clearTimeout(timer);
      await running;
    },
  };
}
