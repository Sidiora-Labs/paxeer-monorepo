// @vitest-environment jsdom
import { once } from 'node:events';
import { createServer, type Server } from 'node:http';
import { Wallet, verifyMessage } from 'ethers';
import { afterEach, beforeEach, expect, test, vi } from 'vitest';
import { pushUnsubscribeMessage } from '@/app/api/push/subscribe/unsubscribe-intent';

const owner = new Wallet('0x' + '17'.repeat(32));
const other = new Wallet('0x' + '18'.repeat(32));
const endpoint = 'https://push.example.test/subscriptions/17';
const nativeFetch = globalThis.fetch;
const serviceWorkerDescriptor = Object.getOwnPropertyDescriptor(navigator, 'serviceWorker');
let server: Server;
let origin: string;
let push: typeof import('./push');
let deletions: typeof import('./push-deletions');
let current: BrowserSubscription | null;
let mode: 'ok' | 'unavailable' | 'disconnect' | 'invalid-ack' | 'lost-ack';
let postReply: 'ok' | 'unavailable' | 'invalid-ack';
let browserRemovalAllowed: boolean;
let removalObservations: boolean[];
let requests: { method: string; body: Record<string, unknown> }[];
let associations: Map<string, string>;
let postBarrier: Promise<void> | null;

class BrowserSubscription implements PushSubscription {
  readonly expirationTime = null;
  readonly options: PushSubscriptionOptions = {
    applicationServerKey: null,
    userVisibleOnly: true,
  };
  constructor(readonly endpoint: string) {}
  getKey(): ArrayBuffer | null {
    return null;
  }
  toJSON(): PushSubscriptionJSON & { keys: { p256dh: string; auth: string } } {
    return {
      endpoint: this.endpoint,
      keys: { p256dh: 'recorded-public-key', auth: 'recorded-auth' },
    };
  }
  async unsubscribe(): Promise<boolean> {
    removalObservations.push(
      deletions.pushDeletionRepository.read().some((item) => item.endpoint === this.endpoint)
    );
    if (!browserRemovalAllowed) return false;
    if (current === this) current = null;
    return true;
  }
}

async function reopen(): Promise<void> {
  push?.clearWalletAddressForPush();
  vi.resetModules();
  push = await import('./push');
  deletions = await import('./push-deletions');
}

beforeEach(async () => {
  localStorage.clear();
  current = new BrowserSubscription(endpoint);
  browserRemovalAllowed = true;
  removalObservations = [];
  requests = [];
  associations = new Map();
  mode = 'ok';
  postReply = 'ok';
  postBarrier = null;
  const registration = {
    active: {},
    pushManager: {
      async getSubscription() {
        return current;
      },
    },
  };
  Object.defineProperty(navigator, 'serviceWorker', {
    configurable: true,
    value: {
      async getRegistration() {
        return registration;
      },
    },
  });
  server = createServer(async (request, response) => {
    try {
      let raw = '';
      for await (const chunk of request) raw += chunk.toString();
      const body = JSON.parse(raw) as Record<string, unknown>;
      const method = request.method ?? '';
      requests.push({ method, body });
      if (request.url !== '/wallet/api/push/subscribe') {
        response.writeHead(404).end();
        return;
      }
      if (method === 'DELETE' && mode === 'disconnect') {
        request.socket.destroy();
        return;
      }
      if (method === 'DELETE' && mode === 'unavailable') {
        response.writeHead(503).end();
        return;
      }
      const now = Math.floor(Date.now() / 1000);
      if (
        typeof body.endpoint !== 'string' ||
        typeof body.walletAddress !== 'string' ||
        typeof body.issuedAt !== 'number' ||
        !Number.isSafeInteger(body.issuedAt) ||
        body.issuedAt < now - 300 ||
        body.issuedAt > now + 60 ||
        typeof body.signature !== 'string'
      ) {
        response.writeHead(401).end();
        return;
      }
      const message =
        method === 'DELETE'
          ? pushUnsubscribeMessage(body.endpoint, body.walletAddress, body.issuedAt)
          : push.pushOwnershipMessage(body.endpoint, body.walletAddress, body.issuedAt);
      const signer = verifyMessage(message, body.signature).toLowerCase();
      if (signer !== body.walletAddress.toLowerCase()) {
        response.writeHead(401).end();
        return;
      }
      if (method === 'POST') {
        if (postBarrier) await postBarrier;
        if (postReply === 'unavailable') {
          response.writeHead(503).end();
          return;
        }
        associations.set(body.endpoint, signer);
        if (postReply === 'invalid-ack') {
          response.writeHead(200, { 'Content-Type': 'application/json' }).end('{"ok":false}');
          return;
        }
      } else if (method === 'DELETE') {
        if (Object.keys(body).length !== 4 || associations.get(body.endpoint) !== signer) {
          response.writeHead(401).end();
          return;
        }
        if (mode === 'invalid-ack') {
          response.writeHead(200, { 'Content-Type': 'application/json' }).end('{"ok":false}');
          return;
        }
        associations.delete(body.endpoint);
        if (mode === 'lost-ack') {
          request.socket.destroy();
          return;
        }
      } else {
        response.writeHead(405).end();
        return;
      }
      response
        .writeHead(200, { 'Content-Type': 'application/json' })
        .end('{"ok":true,"removed":true}');
    } catch {
      response.writeHead(400).end();
    }
  });
  server.listen(0, '127.0.0.1');
  await once(server, 'listening');
  const address = server.address();
  if (!address || typeof address === 'string') throw new Error('Loopback address unavailable');
  origin = `http://127.0.0.1:${address.port}`;
  globalThis.fetch = (input, init) => {
    const url = new URL(input instanceof Request ? input.url : String(input), origin);
    return nativeFetch(new URL(url.pathname + url.search, origin), init);
  };
  await reopen();
  push.setWalletAddressForPush(owner.address, (message) => owner.signMessage(message));
  expect(await push.heartbeat()).toEqual({
    ok: true,
    endpoint,
    walletAddress: owner.address.toLowerCase(),
  });
  expect(push.isPushSubscriptionEnabled(endpoint)).toBe(true);
  expect(associations.get(endpoint)).toBe(owner.address.toLowerCase());
  expect(requests.map((item) => item.method)).toEqual(['POST']);
});

afterEach(async () => {
  push.clearWalletAddressForPush();
  await push.retryPendingPushDeletions();
  globalThis.fetch = nativeFetch;
  if (serviceWorkerDescriptor) {
    Object.defineProperty(navigator, 'serviceWorker', serviceWorkerDescriptor);
  } else {
    Reflect.deleteProperty(navigator, 'serviceWorker');
  }
  localStorage.clear();
  server.closeAllConnections();
  await new Promise<void>((resolve, reject) =>
    server.close((error) => (error ? reject(error) : resolve()))
  );
});

async function finding17_recovers_http_failure_after_component_restart(): Promise<void> {
  mode = 'unavailable';
  expect(await push.unsubscribeFromPush()).toBe(false);
  expect(removalObservations).toEqual([true]);
  expect(current).toBeNull();
  expect(associations.has(endpoint)).toBe(true);
  expect(deletions.pushDeletionRepository.read()).toEqual([
    { endpoint, walletAddress: owner.address.toLowerCase() },
  ]);
  const persisted = localStorage.getItem(deletions.pushDeletionRepository.registration.key);
  expect(persisted).not.toContain('signature');
  expect(persisted).not.toContain('recorded-auth');
  await reopen();
  mode = 'ok';
  // Reopening alone cannot sign; the intent survives until its owner reconnects.
  expect(await push.getCurrentSubscription()).toBeNull();
  expect(associations.has(endpoint)).toBe(true);
  push.setWalletAddressForPush(owner.address, (message) => owner.signMessage(message));
  await expect.poll(() => associations.has(endpoint)).toBe(false);
  expect(await push.retryPendingPushDeletions()).toBe(true);
  expect(deletions.pushDeletionRepository.read()).toEqual([]);
  expect(requests.map((item) => item.method)).toEqual(['POST', 'DELETE', 'DELETE']);
}

async function finding17_retries_transport_failure_on_connectivity_recovery(): Promise<void> {
  mode = 'disconnect';
  expect(await push.unsubscribeFromPush()).toBe(false);
  expect(current).toBeNull();
  expect(deletions.pushDeletionRepository.read()).toHaveLength(1);
  mode = 'ok';
  window.dispatchEvent(new Event('online'));
  await expect.poll(() => associations.has(endpoint)).toBe(false);
  expect(await push.retryPendingPushDeletions()).toBe(true);
  expect(deletions.pushDeletionRepository.read()).toEqual([]);
}

async function finding17_reopens_intent_persisted_before_browser_removal(): Promise<void> {
  browserRemovalAllowed = false;
  expect(await push.unsubscribeFromPush()).toBe(false);
  expect(current?.endpoint).toBe(endpoint);
  expect(requests.map((item) => item.method)).toEqual(['POST']);
  expect(removalObservations).toEqual([true]);
  await reopen();
  browserRemovalAllowed = true;
  push.setWalletAddressForPush(owner.address, (message) => owner.signMessage(message));
  expect(await push.retryPendingPushDeletions()).toBe(true);
  expect(current).toBeNull();
  expect(removalObservations).toEqual([true, true]);
  expect(associations.has(endpoint)).toBe(false);
}

async function finding17_storage_failure_preserves_browser_and_server(): Promise<void> {
  localStorage.setItem('quota-fixture', 'x'.repeat(4_999_900));
  expect(await push.unsubscribeFromPush()).toBe(false);
  expect(current?.endpoint).toBe(endpoint);
  expect(removalObservations).toEqual([]);
  expect(associations.has(endpoint)).toBe(true);
  expect(requests.map((item) => item.method)).toEqual(['POST']);
}

async function finding17_bounds_intents_without_evicting_pending_deletions(): Promise<void> {
  const pending = Array.from({ length: deletions.MAX_PUSH_DELETIONS }, (_, index) => ({
    endpoint: `${endpoint}-${index}`,
    walletAddress: other.address.toLowerCase(),
  }));
  deletions.pushDeletionRepository.write(pending);
  expect(await push.unsubscribeFromPush()).toBe(false);
  expect(deletions.pushDeletionRepository.read()).toEqual(pending);
  expect(current?.endpoint).toBe(endpoint);
  expect(removalObservations).toEqual([]);
  expect(associations.has(endpoint)).toBe(true);
}

async function finding17_keeps_failed_acknowledgements_and_wrong_owner_pending(): Promise<void> {
  mode = 'invalid-ack';
  expect(await push.unsubscribeFromPush()).toBe(false);
  expect(deletions.pushDeletionRepository.read()).toHaveLength(1);
  await reopen();
  push.setWalletAddressForPush(other.address, (message) => other.signMessage(message));
  expect(await push.retryPendingPushDeletions()).toBe(false);
  expect(requests.filter((item) => item.method === 'DELETE')).toHaveLength(1);
  mode = 'ok';
  push.setWalletAddressForPush(owner.address, (message) => owner.signMessage(message));
  expect(await push.retryPendingPushDeletions()).toBe(true);
  expect(associations.has(endpoint)).toBe(false);
}

async function finding17_refreshes_proof_after_restart_and_never_reregisters_pending_endpoint(): Promise<void> {
  mode = 'unavailable';
  expect(await push.unsubscribeFromPush()).toBe(false);
  await reopen();
  // A recovered browser may have a newer subscription, which is not the deletion target.
  const replacement = new BrowserSubscription(`${endpoint}-replacement`);
  current = replacement;
  push.setWalletAddressForPush(owner.address, (message) => owner.signMessage(message));
  await push.heartbeat();
  expect(requests.filter((item) => item.method === 'POST')).toHaveLength(1);
  mode = 'ok';
  expect(await push.retryPendingPushDeletions()).toBe(true);
  expect(current).toBe(replacement);
  const last = requests[requests.length - 1].body;
  expect(last.endpoint).toBe(endpoint);
  expect(last.issuedAt).toBeGreaterThanOrEqual(Math.floor(Date.now() / 1000) - 1);
  expect(
    verifyMessage(
      pushUnsubscribeMessage(endpoint, owner.address, Number(last.issuedAt)),
      String(last.signature)
    )
  ).toBe(owner.address);
}

async function finding17_retains_intent_when_server_commits_but_ack_is_lost(): Promise<void> {
  mode = 'lost-ack';
  expect(await push.unsubscribeFromPush()).toBe(false);
  expect(associations.has(endpoint)).toBe(false);
  await reopen();
  mode = 'ok';
  push.setWalletAddressForPush(owner.address, (message) => owner.signMessage(message));
  expect(await push.retryPendingPushDeletions()).toBe(false);
  // The current server rejects an absent endpoint with 401; retain evidence until
  // the owning domain implements the recorded authenticated idempotency request.
  expect(deletions.pushDeletionRepository.read()).toHaveLength(1);
}

async function finding17_orders_deletion_after_inflight_heartbeat(): Promise<void> {
  let releasePost!: () => void;
  postBarrier = new Promise<void>((resolve) => {
    releasePost = resolve;
  });
  const heartbeat = push.heartbeat();
  await expect.poll(() => requests.filter((item) => item.method === 'POST').length).toBe(2);
  const removal = push.unsubscribeFromPush();
  releasePost();
  await heartbeat;
  expect(await removal).toBe(true);
  expect(current).toBeNull();
  expect(associations.has(endpoint)).toBe(false);
  expect(requests.map((item) => item.method)).toEqual(['POST', 'POST', 'DELETE']);
  expect(deletions.pushDeletionRepository.read()).toEqual([]);
}

async function finding17_does_not_send_after_wallet_disconnect_during_signing(): Promise<void> {
  mode = 'unavailable';
  expect(await push.unsubscribeFromPush()).toBe(false);
  await reopen();
  let releaseSignature!: () => void;
  const signatureBarrier = new Promise<void>((resolve) => {
    releaseSignature = resolve;
  });
  let signing = false;
  push.setWalletAddressForPush(owner.address, async (message) => {
    signing = true;
    await signatureBarrier;
    return owner.signMessage(message);
  });
  await expect.poll(() => signing).toBe(true);
  push.clearWalletAddressForPush();
  releaseSignature();
  expect(await push.retryPendingPushDeletions()).toBe(false);
  expect(requests.filter((item) => item.method === 'DELETE')).toHaveLength(1);
  expect(deletions.pushDeletionRepository.read()).toHaveLength(1);
  mode = 'ok';
  push.setWalletAddressForPush(owner.address, (message) => owner.signMessage(message));
  expect(await push.retryPendingPushDeletions()).toBe(true);
  expect(associations.has(endpoint)).toBe(false);
}

async function finding17_preserves_registration_acknowledgement_contract(): Promise<void> {
  const subscription = current;
  if (!subscription) throw new Error('Browser subscription unavailable');
  const record = { endpoint, keys: subscription.toJSON().keys };
  postReply = 'unavailable';
  expect(await push.heartbeat(record)).toEqual({
    ok: false,
    code: 'SERVER_REFUSED',
    retryable: true,
    status: 503,
  });
  expect(push.isPushSubscriptionEnabled(endpoint)).toBe(false);
  await expect(push.confirmPushRegistration(subscription)).rejects.toBeInstanceOf(
    push.PushRegistrationError
  );
  postReply = 'invalid-ack';
  expect(await push.sendSubscriptionToServer(record)).toEqual({
    ok: false,
    code: 'ACK_INVALID',
    retryable: true,
  });
  expect(await push.getCurrentSubscription()).toBeNull();
  postReply = 'ok';
  expect(await push.confirmPushRegistration(subscription)).toBe(subscription);
  expect(await push.getCurrentSubscription()).toBe(subscription);
}

async function finding17_blocks_explicit_registration_while_deletion_pending(): Promise<void> {
  const subscription = current;
  if (!subscription) throw new Error('Browser subscription unavailable');
  const record = { endpoint, keys: subscription.toJSON().keys };
  mode = 'unavailable';
  expect(await push.unsubscribeFromPush()).toBe(false);
  expect(push.isPushSubscriptionEnabled(endpoint)).toBe(false);
  expect(await push.heartbeat(record)).toMatchObject({ ok: false, retryable: true });
  expect(await push.sendSubscriptionToServer(record)).toMatchObject({ ok: false, retryable: true });
  await expect(push.confirmPushRegistration(subscription)).rejects.toBeInstanceOf(
    push.PushRegistrationError
  );
  expect(requests.filter((item) => item.method === 'POST')).toHaveLength(1);
  expect(deletions.pushDeletionRepository.read()).toHaveLength(1);
  mode = 'ok';
  expect(await push.heartbeat(record)).toMatchObject({ ok: false, retryable: true });
  expect(associations.has(endpoint)).toBe(false);
  expect(deletions.pushDeletionRepository.read()).toEqual([]);
  expect(requests.filter((item) => item.method === 'POST')).toHaveLength(1);
}

async function finding17_orders_deletion_after_explicit_registration(): Promise<void> {
  const subscription = current;
  if (!subscription) throw new Error('Browser subscription unavailable');
  let releasePost!: () => void;
  postBarrier = new Promise<void>((resolve) => {
    releasePost = resolve;
  });
  const registration = push.confirmPushRegistration(subscription);
  await expect.poll(() => requests.filter((item) => item.method === 'POST').length).toBe(2);
  const removal = push.unsubscribeFromPush();
  releasePost();
  expect(await registration).toBe(subscription);
  expect(await removal).toBe(true);
  expect(current).toBeNull();
  expect(associations.has(endpoint)).toBe(false);
  expect(push.isPushSubscriptionEnabled(endpoint)).toBe(false);
  expect(requests.map((item) => item.method)).toEqual(['POST', 'POST', 'DELETE']);
  expect(deletions.pushDeletionRepository.read()).toEqual([]);
}

async function finding17_confirms_existing_browser_subscription(): Promise<void> {
  const subscription = current;
  const pushManagerDescriptor = Object.getOwnPropertyDescriptor(window, 'PushManager');
  const notificationDescriptor = Object.getOwnPropertyDescriptor(window, 'Notification');
  Object.defineProperty(window, 'PushManager', { configurable: true, value: class PushManager {} });
  Object.defineProperty(window, 'Notification', {
    configurable: true,
    value: class Notification {},
  });
  try {
    expect(push.isPushSupported()).toBe(true);
    postReply = 'unavailable';
    await expect(push.subscribeToPush()).rejects.toBeInstanceOf(push.PushRegistrationError);
    expect(push.isPushSubscriptionEnabled(endpoint)).toBe(false);
    postReply = 'ok';
    expect(await push.subscribeToPush()).toBe(subscription);
    expect(push.isPushSubscriptionEnabled(endpoint)).toBe(true);
  } finally {
    if (pushManagerDescriptor) Object.defineProperty(window, 'PushManager', pushManagerDescriptor);
    else Reflect.deleteProperty(window, 'PushManager');
    if (notificationDescriptor)
      Object.defineProperty(window, 'Notification', notificationDescriptor);
    else Reflect.deleteProperty(window, 'Notification');
  }
}

for (const criterion of [
  finding17_recovers_http_failure_after_component_restart,
  finding17_retries_transport_failure_on_connectivity_recovery,
  finding17_reopens_intent_persisted_before_browser_removal,
  finding17_storage_failure_preserves_browser_and_server,
  finding17_bounds_intents_without_evicting_pending_deletions,
  finding17_keeps_failed_acknowledgements_and_wrong_owner_pending,
  finding17_refreshes_proof_after_restart_and_never_reregisters_pending_endpoint,
  finding17_retains_intent_when_server_commits_but_ack_is_lost,
  finding17_orders_deletion_after_inflight_heartbeat,
  finding17_does_not_send_after_wallet_disconnect_during_signing,
  finding17_preserves_registration_acknowledgement_contract,
  finding17_blocks_explicit_registration_while_deletion_pending,
  finding17_orders_deletion_after_explicit_registration,
  finding17_confirms_existing_browser_subscription,
]) {
  test(criterion.name, criterion);
}
