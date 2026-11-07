import { pushOwnershipMessage } from '../lib/push';

export interface PendingPushRenewal {
  readonly id: string;
  readonly subscription: {
    readonly endpoint: string;
    readonly keys: { readonly p256dh: string; readonly auth: string };
  };
}

export function parsePendingPushRenewal(value: unknown): PendingPushRenewal {
  if (typeof value !== 'object' || value === null || Array.isArray(value)) {
    throw new TypeError('Invalid pending push renewal');
  }
  const pending = value as Record<string, unknown>;
  const subscription = pending.subscription as Record<string, unknown> | null;
  const keys = subscription?.keys as Record<string, unknown> | null;
  const bounded = (input: unknown, minimum: number, maximum: number): input is string =>
    typeof input === 'string' && input.length >= minimum && input.length <= maximum;
  if (
    !bounded(pending.id, 1, 128) ||
    !subscription ||
    Array.isArray(subscription) ||
    !bounded(subscription.endpoint, 12, 2048) ||
    !keys ||
    Array.isArray(keys) ||
    !bounded(keys.p256dh, 16, 256) ||
    !bounded(keys.auth, 8, 128)
  ) {
    throw new TypeError('Invalid pending push renewal');
  }
  const endpoint = new URL(subscription.endpoint);
  if (endpoint.protocol !== 'https:' || endpoint.username || endpoint.password) {
    throw new TypeError('Invalid pending push endpoint');
  }
  return {
    id: pending.id,
    subscription: {
      endpoint: subscription.endpoint,
      keys: { p256dh: keys.p256dh, auth: keys.auth },
    },
  };
}

export interface PushRenewalWallet {
  readonly address: string;
  signMessage(message: string): Promise<string>;
  isCurrent(): boolean;
}

export interface RenewalWorker {
  postMessage(message: unknown, transfer: Transferable[]): void;
}

async function workerRequest(worker: RenewalWorker, message: unknown): Promise<unknown> {
  const channel = new MessageChannel();
  return new Promise((resolve, reject) => {
    const finish = (error: Error | null, value?: unknown) => {
      clearTimeout(timer);
      channel.port1.close();
      channel.port2.close();
      if (error) reject(error);
      else resolve(value);
    };
    const timer = setTimeout(() => finish(new Error('Push renewal worker timed out')), 10_000);
    channel.port1.onmessage = (event: MessageEvent<unknown>) => finish(null, event.data);
    channel.port1.onmessageerror = () => finish(new Error('Invalid push renewal worker response'));
    try {
      worker.postMessage(message, [channel.port2]);
    } catch (error) {
      finish(error instanceof Error ? error : new Error('Push renewal worker unavailable'));
    }
  });
}

function responseRecord(value: unknown): Record<string, unknown> {
  if (typeof value !== 'object' || value === null || (value as { ok?: unknown }).ok !== true) {
    throw new Error('Push renewal worker refused the request');
  }
  return value as Record<string, unknown>;
}

export async function synchronizePushRenewal(
  worker: RenewalWorker,
  wallet: PushRenewalWallet,
  origin: string
): Promise<boolean> {
  if (!wallet.isCurrent()) return false;
  const reply = responseRecord(await workerRequest(worker, { type: 'PUSH_RENEWAL_READ' }));
  if (reply.pending === null) return false;
  const pending = parsePendingPushRenewal(reply.pending);
  const issuedAt = Math.floor(Date.now() / 1000);
  const signature = await wallet.signMessage(
    pushOwnershipMessage(pending.subscription.endpoint, wallet.address, issuedAt)
  );
  if (!wallet.isCurrent()) return false;
  const response = await fetch(new URL('/wallet/api/push/subscribe', origin), {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({
      ...pending.subscription,
      walletAddress: wallet.address,
      signature,
      issuedAt,
    }),
  });
  if (!response.ok) throw new Error(`Push renewal refused (${response.status})`);
  const acknowledgement: unknown = await response.json();
  responseRecord(acknowledgement);
  if (!wallet.isCurrent()) return false;
  const cleared = responseRecord(
    await workerRequest(worker, {
      type: 'PUSH_RENEWAL_ACK',
      id: pending.id,
    })
  );
  return cleared.cleared === true;
}

export interface PushRenewalContainer extends EventTarget {
  readonly controller: RenewalWorker | null;
  readonly ready: Promise<{ readonly active: RenewalWorker | null }>;
}

export function openPushRenewalSession(
  container: PushRenewalContainer,
  wallet: PushRenewalWallet,
  options: { origin: string; onError(error: unknown): void; lifecycle?: EventTarget }
): { ready: Promise<boolean>; retry(): Promise<boolean>; dispose(): void } {
  let disposed = false;
  let running: Promise<boolean> | null = null;
  const session: PushRenewalWallet = {
    address: wallet.address,
    signMessage: (message) => wallet.signMessage(message),
    isCurrent: () => !disposed && wallet.isCurrent(),
  };
  const retry = (): Promise<boolean> => {
    if (!session.isCurrent()) return Promise.resolve(false);
    if (running) return running;
    running = (async () => {
      const worker = container.controller ?? (await container.ready).active;
      if (!worker) throw new Error('Push renewal worker unavailable');
      return synchronizePushRenewal(worker, session, options.origin);
    })().finally(() => {
      running = null;
    });
    return running;
  };
  const onRetry = () => {
    void retry().catch(options.onError);
  };
  const onMessage: EventListener = (event) => {
    if (event instanceof MessageEvent && event.data?.type === 'PUSH_SUBSCRIPTION_CHANGED')
      onRetry();
  };
  container.addEventListener('message', onMessage);
  container.addEventListener('controllerchange', onRetry);
  options.lifecycle?.addEventListener('online', onRetry);
  options.lifecycle?.addEventListener('visibilitychange', onRetry);
  return {
    ready: retry(),
    retry,
    dispose() {
      disposed = true;
      container.removeEventListener('message', onMessage);
      container.removeEventListener('controllerchange', onRetry);
      options.lifecycle?.removeEventListener('online', onRetry);
      options.lifecycle?.removeEventListener('visibilitychange', onRetry);
    },
  };
}
