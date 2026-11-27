import type { QueryClient } from '@tanstack/react-query';
import { isTransientError, retryHint, transientRetryBudgetMs, transientRetryCapMs } from './transient-retry';
import { dataApiUrl } from './sdk-config';

const retryPaceHoldMs = 2;

interface ResourcePace {
  key: string;
  started: number;
  failures: number;
  next: number;
  error: unknown;
  immediate: boolean;
  touched: number;
}

export class ResourceRetryPace {
  private queries?: QueryClient;
  private queryClient?: Promise<QueryClient>;
  private readonly resources = new Map<string, ResourcePace>();
  private readonly errors = new WeakMap<object, ResourcePace>();
  private readonly references = new Map<string, { count: number }>();
  private readonly gates = new Map<string, Promise<void>>();

  constructor(private readonly now: () => number = () => Date.now()) {}

  state(error: unknown): ResourcePace | undefined {
    return typeof error === 'object' && error !== null ? this.errors.get(error) : undefined;
  }

  delay(error: unknown): number | false {
    const state = this.state(error);
    return state ? Math.max(1, state.next - this.now()) : false;
  }

  retryNow(error: unknown): void {
    const state = this.state(error);
    if (state && !this.references.has(state.key)) state.immediate = true;
  }

  resume(): void {
    for (const state of this.resources.values()) state.immediate = true;
  }

  async read<T>(key: string, load: (signal: AbortSignal) => Promise<T>, signal?: AbortSignal, paceKey = key): Promise<T> {
    const queries = this.queries ?? await (this.queryClient ??= import('@tanstack/react-query').then(({ QueryClient }) => this.queries = new QueryClient()));
    signal?.throwIfAborted();
    const queryKey = ['market-resource', key] as const;
    let active = queries.getQueryState(queryKey)?.fetchStatus === 'fetching';
    let state = this.resources.get(paceKey);
    while (!active && state && !state.immediate) {
      const remaining = state.next - this.now();
      if (remaining <= 0) break;
      if (remaining > retryPaceHoldMs) throw state.error;
      let gate = this.gates.get(paceKey);
      if (!gate) {
        gate = new Promise<void>(resolve => setTimeout(resolve, remaining)).finally(() => {
          if (this.gates.get(paceKey) === gate) this.gates.delete(paceKey);
        });
        this.gates.set(paceKey, gate);
      }
      await new Promise<void>((resolve, reject) => {
        const abort = () => {
          signal?.removeEventListener('abort', abort);
          reject(signal?.reason ?? new DOMException('Aborted', 'AbortError'));
        };
        signal?.addEventListener('abort', abort, { once: true });
        void gate.then(() => { signal?.removeEventListener('abort', abort); resolve(); });
        if (signal?.aborted) abort();
      });
      signal?.throwIfAborted();
      active = queries.getQueryState(queryKey)?.fetchStatus === 'fetching';
      state = this.resources.get(paceKey);
    }
    if (state) { state.immediate = false; state.touched = this.now(); }
    for (const [resource, pace] of this.resources) {
      if (this.now() - pace.touched > 5 * 60_000 && !this.references.has(resource)) this.resources.delete(resource);
    }
    if (!active) queries.getQueryCache().find({ queryKey, exact: true })?.invalidate();
    const promise = queries.fetchQuery({
      queryKey,
      queryFn: async ({ signal: sharedSignal }) => {
        try {
          const value = await load(sharedSignal);
          sharedSignal.throwIfAborted();
          this.resources.delete(paceKey);
          return value;
        } catch (error) {
          sharedSignal.throwIfAborted();
          if (isTransientError(error)) {
            const now = this.now();
            const pace = this.resources.get(paceKey) ?? { key, started: now, failures: 0, next: now, error, immediate: false, touched: now };
            pace.key = key;
            pace.failures++;
            pace.error = error;
            pace.touched = now;
            const serverDelay = Math.min(transientRetryCapMs, retryHint(error).delayMs ?? 1000);
            const delay = pace.failures < 5 ? Math.min(transientRetryCapMs, serverDelay * 2 ** (pace.failures - 1))
              : Math.min(60_000, 15_000 * 2 ** (pace.failures - 4));
            pace.next = now + Math.max(delay, serverDelay);
            this.resources.set(paceKey, pace);
            if (typeof error === 'object' && error !== null) this.errors.set(error, pace);
          } else this.resources.delete(paceKey);
          throw error;
        }
      },
      retry: false,
      networkMode: 'always',
      staleTime: 0,
      gcTime: 5 * 60_000,
    });
    const references = active ? this.references.get(key) ?? { count: 0 } : { count: 0 };
    references.count++;
    this.references.set(key, references);
    return new Promise<T>((resolve, reject) => {
      let released = false;
      const release = () => {
        if (released) return false;
        released = true;
        signal?.removeEventListener('abort', abort);
        if (--references.count === 0 && this.references.get(key) === references) {
          this.references.delete(key);
          queries.removeQueries({ queryKey, exact: true });
        }
        return true;
      };
      const abort = () => { if (release()) reject(signal?.reason ?? new DOMException('Aborted', 'AbortError')); };
      signal?.addEventListener('abort', abort, { once: true });
      if (signal?.aborted) abort();
      void promise.then(value => { if (release()) resolve(value); }, error => { if (release()) reject(error); });
    });
  }

  waiting(error: unknown): number {
    const state = this.state(error);
    return state ? Math.max(0, state.started + transientRetryBudgetMs - this.now()) : 0;
  }

  clear(): void {
    this.queries?.clear();
    this.resources.clear();
    this.references.clear();
    this.gates.clear();
  }
}

export const marketRetryPace = new ResourceRetryPace();

export function marketResourceKey(url: string | URL, body?: unknown, scope = ''): string {
  let canonical = new URL(url);
  if (typeof document !== 'undefined' && canonical.pathname.startsWith('/v2/pools/') && canonical.origin === document.location?.origin) {
    canonical = new URL(canonical.pathname + canonical.search, dataApiUrl('/'));
  }
  canonical.searchParams.sort();
  return JSON.stringify([canonical.href, body ?? null, scope]);
}

export function marketResourcePaceKey(url: string | URL, body?: unknown, scope = ''): string {
  const resource = new URL(url);
  if (/\/v2\/pools\/[^/]+\/candles$/.test(resource.pathname)) {
    for (const parameter of ['from', 'to', 'cursor', 'limit']) resource.searchParams.delete(parameter);
  }
  return marketResourceKey(resource, body, scope);
}
