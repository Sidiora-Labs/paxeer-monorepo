import { OFFLINE_URL, PRECACHE_URLS, cacheNames, staleCaches, strategyFor } from './caching';
import { MANIFEST_ICONS } from './manifest';
import { parsePendingPushRenewal } from './push-renewal';

declare const __PAXEER_SW__: { readonly version: string; readonly networkOnly: readonly string[] };

interface WorkerExtendableEvent extends Event {
  waitUntil(promise: Promise<unknown>): void;
}

interface WorkerFetchEvent extends WorkerExtendableEvent {
  readonly request: Request;
  respondWith(response: Promise<Response>): void;
}

interface WorkerMessageEvent extends WorkerExtendableEvent {
  readonly data: unknown;
  readonly ports?: readonly MessagePort[];
  readonly source?: WorkerWindowClient | null;
}

interface WorkerPushEvent extends WorkerExtendableEvent {
  readonly data: { json(): unknown; text(): string } | null;
}

interface WorkerNotificationEvent extends WorkerExtendableEvent {
  readonly notification: Notification;
}

interface WorkerSubscriptionChangeEvent extends WorkerExtendableEvent {
  readonly oldSubscription: PushSubscription | null;
  readonly newSubscription?: PushSubscription | null;
}

interface WorkerWindowClient {
  readonly url: string;
  postMessage(message: unknown): void;
  focus(): Promise<unknown>;
}

interface WorkerScope {
  readonly location: Location;
  readonly registration: ServiceWorkerRegistration;
  readonly clients: {
    claim(): Promise<void>;
    matchAll(options?: {
      type?: string;
      includeUncontrolled?: boolean;
    }): Promise<WorkerWindowClient[]>;
    openWindow(url: string): Promise<unknown>;
  };
  skipWaiting(): Promise<void>;
  addEventListener(
    type: 'install' | 'activate',
    listener: (event: WorkerExtendableEvent) => void
  ): void;
  addEventListener(type: 'fetch', listener: (event: WorkerFetchEvent) => void): void;
  addEventListener(type: 'message', listener: (event: WorkerMessageEvent) => void): void;
  addEventListener(type: 'push', listener: (event: WorkerPushEvent) => void): void;
  addEventListener(
    type: 'notificationclick',
    listener: (event: WorkerNotificationEvent) => void
  ): void;
  addEventListener(
    type: 'pushsubscriptionchange',
    listener: (event: WorkerSubscriptionChangeEvent) => void
  ): void;
}

const scope = self as unknown as WorkerScope;
const names = cacheNames(__PAXEER_SW__.version);
const networkOnly = __PAXEER_SW__.networkOnly;
const notificationIcon = MANIFEST_ICONS[0].src;

async function precache(): Promise<void> {
  const cache = await caches.open(names.shell);
  await cache.addAll(PRECACHE_URLS.map((url) => new Request(url, { cache: 'reload' })));
}

async function purge(): Promise<void> {
  const existing = await caches.keys();
  await Promise.all(staleCaches(existing, names).map((name) => caches.delete(name)));
  await scope.clients.claim();
}

async function navigate(request: Request): Promise<Response> {
  try {
    return await fetch(request);
  } catch (error) {
    const shell = await caches.open(names.shell);
    const offline = await shell.match(OFFLINE_URL);
    if (offline) return offline;
    throw error;
  }
}

async function cacheFirst(request: Request): Promise<Response> {
  const cache = await caches.open(names.static);
  const cached =
    (await cache.match(request)) ??
    (await caches.open(names.shell).then((shell) => shell.match(request)));
  if (cached) return cached;
  const response = await fetch(request);
  if (response.ok && response.type === 'basic') {
    await cache.put(request, response.clone());
  }
  return response;
}

function notificationTarget(raw: unknown): string {
  if (typeof raw !== 'string' || raw.length > 2048) return '/wallet/';
  try {
    const candidate = new URL(raw, scope.location.origin);
    if (
      candidate.origin === scope.location.origin &&
      candidate.pathname === '/wallet/' &&
      !candidate.username &&
      !candidate.password
    ) {
      return `${candidate.pathname}${candidate.search}`;
    }
  } catch {
    return '/wallet/';
  }
  return '/wallet/';
}

scope.addEventListener('install', (event) => {
  event.waitUntil(precache());
});

scope.addEventListener('activate', (event) => {
  event.waitUntil(purge());
});

scope.addEventListener('fetch', (event) => {
  const request = event.request;
  const strategy = strategyFor(
    { url: request.url, method: request.method, mode: request.mode },
    { origin: scope.location.origin, networkOnly }
  );
  if (strategy === 'navigation') {
    event.respondWith(navigate(request));
  } else if (strategy === 'cache-first') {
    event.respondWith(cacheFirst(request));
  }
});

const PUSH_RENEWAL_CACHE = 'paxeer-push-renewals-v1';
const renewalKey = new URL('/wallet/__push-renewal', scope.location.origin).href;
let renewalQueue: Promise<unknown> = Promise.resolve();

function withRenewalStore<T>(operation: (cache: Cache) => Promise<T>): Promise<T> {
  const result = renewalQueue.then(async () => operation(await caches.open(PUSH_RENEWAL_CACHE)));
  renewalQueue = result.then(
    () => undefined,
    () => undefined
  );
  return result;
}

scope.addEventListener('message', (event) => {
  const data = event.data as { type?: unknown; id?: unknown } | null;
  if (data?.type === 'SKIP_WAITING') {
    event.waitUntil(scope.skipWaiting());
  } else if (data?.type === 'PUSH_RENEWAL_READ' || data?.type === 'PUSH_RENEWAL_ACK') {
    const port = event.ports?.[0];
    if (!port) return;
    event.waitUntil(
      (async () => {
        try {
          const source = event.source && new URL(event.source.url);
          if (
            !source ||
            source.origin !== scope.location.origin ||
            !source.pathname.startsWith('/wallet/')
          ) {
            throw new Error('Push renewal client is outside the wallet');
          }
          const result = await withRenewalStore(async (cache) => {
            const stored = await cache.match(renewalKey);
            const pending = stored ? parsePendingPushRenewal(await stored.json()) : null;
            if (data.type === 'PUSH_RENEWAL_READ') return { ok: true, pending };
            const cleared = pending !== null && pending.id === data.id;
            if (cleared) await cache.delete(renewalKey);
            return { ok: true, cleared };
          });
          port.postMessage(result);
        } catch {
          port.postMessage({ ok: false });
        } finally {
          port.close();
        }
      })()
    );
  }
});

function presentationString(value: unknown, maximum: number, fallback: string): string {
  return typeof value === 'string' && value.length > 0 && value.length <= maximum
    ? value
    : fallback;
}

function notificationAsset(value: unknown): string {
  const bounded = presentationString(value, 2048, notificationIcon);
  try {
    const url = new URL(bounded, scope.location.origin);
    if (
      url.origin === scope.location.origin &&
      !url.username &&
      !url.password &&
      (url.protocol === 'https:' || url.protocol === 'http:')
    )
      return url.href;
  } catch {
    return notificationIcon;
  }
  return notificationIcon;
}

scope.addEventListener('push', (event) => {
  let input: unknown = null;
  if (event.data) {
    try {
      input = event.data.json();
    } catch {
      input = { body: event.data.text() };
    }
  }
  const payload =
    typeof input === 'object' && input !== null && !Array.isArray(input)
      ? (input as Record<string, unknown>)
      : {};
  event.waitUntil(
    scope.registration.showNotification(presentationString(payload.title, 120, 'Paxeer Wallet'), {
      body: presentationString(payload.body, 500, 'You have a new notification.'),
      icon: notificationAsset(payload.icon),
      badge: notificationAsset(payload.badge),
      data: { url: notificationTarget(payload.url) },
      tag: presentationString(payload.tag, 128, 'paxeer-default'),
      requireInteraction:
        typeof payload.requireInteraction === 'boolean' ? payload.requireInteraction : false,
    })
  );
});

scope.addEventListener('notificationclick', (event) => {
  event.notification.close();
  const target = notificationTarget((event.notification.data as { url?: unknown } | null)?.url);
  event.waitUntil(
    scope.clients.matchAll({ type: 'window', includeUncontrolled: true }).then((clients) => {
      for (const client of clients) {
        if (
          new URL(client.url).origin === scope.location.origin &&
          new URL(client.url).pathname.startsWith('/wallet/')
        ) {
          client.postMessage({ type: 'PAXPORT_NAVIGATE', route: target });
          return client.focus();
        }
      }
      return scope.clients.openWindow(target);
    })
  );
});

scope.addEventListener('pushsubscriptionchange', (event) => {
  const previous = event.oldSubscription;
  if (!previous && !event.newSubscription) return;
  event.waitUntil(
    (async () => {
      const subscription =
        event.newSubscription ??
        (await scope.registration.pushManager.subscribe(previous!.options));
      const pending = parsePendingPushRenewal({
        id: crypto.randomUUID(),
        subscription: subscription.toJSON(),
      });
      await withRenewalStore((cache) =>
        cache.put(
          renewalKey,
          new Response(JSON.stringify(pending), {
            headers: { 'Content-Type': 'application/json' },
          })
        )
      );
      const clients = await scope.clients.matchAll({ type: 'window', includeUncontrolled: true });
      for (const client of clients) {
        const url = new URL(client.url);
        if (url.origin === scope.location.origin && url.pathname.startsWith('/wallet/')) {
          client.postMessage({
            type: 'PUSH_SUBSCRIPTION_CHANGED',
            subscription: pending.subscription,
          });
        }
      }
    })()
  );
});
