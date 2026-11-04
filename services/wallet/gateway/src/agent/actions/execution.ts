import { AsyncLocalStorage } from 'node:async_hooks';
import { performance } from 'node:perf_hooks';

const execution = new AsyncLocalStorage<ActionExecution>();

export class ActionExecution {
  private readonly controller = new AbortController();
  private readonly deadline: number;
  private expires: number;

  constructor(started: number, leaseMs: number, operationMs: number) {
    this.expires = started + leaseMs;
    this.deadline = performance.now() + operationMs;
  }

  get signal(): AbortSignal {
    return this.controller.signal;
  }

  renew(started: number, leaseMs: number): void {
    this.assertActive();
    this.expires = started + leaseMs;
    this.assertActive();
  }

  cancel(reason: Error): void {
    this.controller.abort(reason);
  }

  assertActive(): void {
    if (performance.now() >= this.deadline)
      this.cancel(new Error('action operation deadline exceeded'));
    if (performance.now() >= this.expires) this.cancel(new Error('action lease expired'));
    this.signal.throwIfAborted();
  }

  run<T>(fn: () => Promise<T>): Promise<T> {
    this.assertActive();
    return execution.run(this, fn);
  }
}

export function assertActionActive(): void {
  execution.getStore()?.assertActive();
}

/** Read/sign operations can be abandoned; started durable writes must drain. */
export function actionOperation<A extends unknown[], R>(
  operation: (...args: A) => Promise<R>,
  interruptible = true
): (...args: A) => Promise<R> {
  return async (...args) => {
    const current = execution.getStore();
    if (!current) return operation(...args);
    current.assertActive();
    const work = operation(...args);
    let result: R;
    if (interruptible) {
      result = await new Promise<R>((resolve, reject) => {
        const abort = () => reject(current.signal.reason);
        current.signal.addEventListener('abort', abort, { once: true });
        work
          .then(resolve, reject)
          .finally(() => current.signal.removeEventListener('abort', abort));
        if (current.signal.aborted) abort();
      });
    } else result = await work;
    current.assertActive();
    return result;
  };
}
