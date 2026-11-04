import { createECDH, randomBytes } from 'node:crypto';
import { createRequire } from 'node:module';
import { readFile, rm } from 'node:fs/promises';
import { createServer } from 'node:https';
import type { ServerResponse as HttpServerResponse, IncomingHttpHeaders } from 'node:http';
import path from 'node:path';
import webpush from 'web-push';
import { expect, test } from 'vitest';
import { createPushService, type PushServiceOptions } from './push-service';
import { loopbackAgent } from './fixtures/repair-22-33-167/tls-agent';
import {
  CAMPAIGN_MAX_ATTEMPTS,
  INACTIVITY_COOLDOWN_MS,
  createPushStore,
  type StoredSubscription,
} from './push-store';
import {
  scratchDirectory,
  worker,
  store_corruption_reopen_refuses_empty_writes_and_verifies_backup,
  store_migration_and_insertion_serialize_in_separate_processes,
  store_publication_crash_boundaries_reopen_with_verified_generations,
  store_live_holder_dead_takeover_and_late_release_keep_ownership,
} from './repair-22-33-167.store-fixture';

const require = createRequire(import.meta.url);
const ece = createRequire(require.resolve('web-push'))('http_ece') as {
  decrypt(
    body: Buffer,
    options: { version: string; privateKey: ReturnType<typeof createECDH>; authSecret: string }
  ): Buffer;
};
interface ReceivedPush {
  route: string;
  headers: IncomingHttpHeaders;
  payload: Record<string, unknown>;
}

async function pushEndpoint(
  handler: (received: ReceivedPush, response: HttpServerResponse) => void
) {
  const fixtureDirectory = path.join(__dirname, 'fixtures/repair-22-33-167');
  const cert = await readFile(path.join(fixtureDirectory, 'tls-cert.pem'), 'utf8');
  const key = await readFile(path.join(fixtureDirectory, 'tls-key.pem'), 'utf8');
  const recipients = new Map<string, { curve: ReturnType<typeof createECDH>; auth: string }>();
  const received: ReceivedPush[] = [];
  const errors: unknown[] = [];
  const disconnects = new Set<() => void>();
  const server = createServer({ cert, key }, (request, response) => {
    const chunks: Buffer[] = [];
    request.on('data', (chunk) => chunks.push(Buffer.from(chunk)));
    request.on('end', () => {
      try {
        const route = request.url!;
        const recipient = recipients.get(route);
        if (!recipient) throw new Error('Unknown push recipient');
        const plaintext = ece.decrypt(Buffer.concat(chunks), {
          version: 'aes128gcm',
          privateKey: recipient.curve,
          authSecret: recipient.auth,
        });
        const message = {
          route,
          headers: request.headers,
          payload: JSON.parse(plaintext.toString('utf8')),
        };
        received.push(message);
        handler(message, response);
      } catch (error) {
        errors.push(error);
        response.writeHead(500).end();
      }
    });
  });
  server.on('connection', (socket) => {
    const disconnect = () => {
      socket.destroy();
    };
    disconnects.add(disconnect);
    socket.on('close', () => disconnects.delete(disconnect));
  });
  await new Promise<void>((resolve) => server.listen(0, '127.0.0.1', resolve));
  const address = server.address();
  if (!address || typeof address === 'string') throw new Error('Missing loopback listener');
  const options: PushServiceOptions = {
    vapidDetails: { subject: 'mailto:fixture@example.test', ...webpush.generateVAPIDKeys() },
    ca: cert,
    networkTimeoutMs: 200,
    routingBudgetMs: 700,
    createAgent: (agentOptions) => loopbackAgent(address.port, agentOptions),
  };
  return {
    options,
    received,
    errors,
    port: address.port,
    subscription(route: string): StoredSubscription {
      const curve = createECDH('prime256v1');
      const publicKey = curve.generateKeys();
      const auth = randomBytes(16).toString('base64url');
      recipients.set(route, { curve, auth });
      return {
        endpoint: `https://fcm.googleapis.com${route}`,
        keys: { p256dh: publicKey.toString('base64url'), auth },
        walletAddress: `0x${'1'.repeat(40)}`,
        createdAt: 1,
        lastActiveAt: 1,
      };
    },
    async close() {
      const closed = new Promise<void>((resolve, reject) =>
        server.close((error) => (error ? reject(error) : resolve()))
      );
      for (const disconnect of disconnects) disconnect();
      await closed;
      expect(errors).toEqual([]);
    },
  };
}

async function waitUntil(predicate: () => boolean) {
  const deadline = Date.now() + 8_000;
  while (!predicate()) {
    if (Date.now() >= deadline) throw new Error('Loopback condition timed out');
    await new Promise((resolve) => setTimeout(resolve, 10));
  }
}

async function push_recovery_preserves_provider_validation_and_subscription_ownership() {
  const directory = await scratchDirectory();
  const peer = await pushEndpoint((_, response) => response.writeHead(201).end());
  try {
    const store = createPushStore(directory);
    const subscription = peer.subscription('/ownership');
    await store.upsertSubscription({
      ...subscription,
      lastNotifiedAt: Number.MAX_SAFE_INTEGER,
      tags: ['subscriber-controlled'],
    });
    const saved = (await store.getSubscription(subscription.endpoint))!;
    expect(saved.createdAt).toBeGreaterThan(subscription.createdAt);
    expect(saved.lastNotifiedAt).toBeUndefined();
    expect(saved.tags).toBeUndefined();
    await store.addTagToWallet(subscription.walletAddress, 'server-managed');
    await store.updateLastNotified([subscription.endpoint]);
    const notified = (await store.getSubscription(subscription.endpoint))!;
    await store.upsertSubscription({ ...subscription, tags: ['replacement'] });
    expect(await createPushStore(directory).getSubscription(subscription.endpoint)).toMatchObject({
      createdAt: saved.createdAt,
      lastNotifiedAt: notified.lastNotifiedAt,
      tags: ['server-managed'],
    });
    await expect(
      store.upsertSubscription({ ...subscription, walletAddress: `0x${'2'.repeat(40)}` })
    ).rejects.toThrow('Subscription owner cannot change');
    const service = createPushService(store, peer.options);
    const invalid = { ...subscription, endpoint: `https://127.0.0.1:${peer.port}/ownership` };
    await expect(store.upsertSubscription(invalid)).rejects.toThrow('endpoint is invalid');
    await expect(
      service.sendToSubscription(invalid, { title: 'Validation', body: 'No untrusted egress' })
    ).rejects.toThrow('endpoint is invalid');
    expect(peer.received).toEqual([]);
    expect(
      await service.sendToSubscription(subscription, {
        title: 'Validation',
        body: 'Valid provider',
      })
    ).toEqual({ success: true, removed: false, retryable: false, uncertain: false });
    expect(peer.received).toHaveLength(1);
    expect(peer.received[0].payload.body).toBe('Valid provider');
  } finally {
    await peer.close();
    await rm(directory, { recursive: true, force: true });
  }
}

async function push_deadlines_bound_stalled_tls_and_trickling_batches() {
  const directory = await scratchDirectory();
  const peer = await pushEndpoint((message, response) => {
    if (message.route === '/stalled') return;
    response.writeHead(200);
    response.write('.');
    const timer = setInterval(() => response.write('.'), 25);
    response.once('close', () => clearInterval(timer));
  });
  try {
    const store = createPushStore(directory);
    const service = createPushService(store, {
      ...peer.options,
      networkTimeoutMs: 100,
      routingBudgetMs: 450,
    });
    const stalled = peer.subscription('/stalled');
    await store.upsertSubscription(stalled);
    const start = performance.now();
    const outcome = await service.sendToSubscription(stalled, {
      title: 'Deadline',
      body: 'Stalled socket',
    });
    expect(outcome).toEqual({ success: false, removed: false, retryable: false, uncertain: true });
    expect(performance.now() - start).toBeLessThan(400);
    expect(await store.getSubscription(stalled.endpoint)).not.toBeNull();
    await store.removeSubscription(stalled.endpoint);
    for (let index = 0; index < 51; index++)
      await store.upsertSubscription(peer.subscription(`/trickle-${index}`));
    const batchStart = performance.now();
    expect(await service.sendToAll({ title: 'Deadline', body: 'Trickling response' })).toEqual({
      total: 51,
      sent: 0,
      failed: 51,
      removed: 0,
    });
    expect(performance.now() - batchStart).toBeLessThan(1_200);
    expect(peer.received.filter((message) => message.route.startsWith('/trickle-'))).toHaveLength(
      50
    );
    expect(peer.received[0].headers['content-encoding']).toBe('aes128gcm');
    expect(peer.received[0].headers.authorization).toMatch(/^vapid /);
    expect(peer.received[0].payload).toMatchObject({ title: 'Deadline', body: 'Stalled socket' });
  } finally {
    await peer.close();
    await rm(directory, { recursive: true, force: true });
  }
}

async function inactivity_atomic_cooldown_survives_concurrent_scans_and_reopen() {
  const directory = await scratchDirectory();
  const peer = await pushEndpoint((_, response) => response.writeHead(201).end());
  try {
    const store = createPushStore(directory);
    const subscription = peer.subscription('/inactive');
    await store.upsertSubscription(subscription);
    await new Promise((resolve) => setTimeout(resolve, 10));
    const first = createPushService(store, peer.options);
    const second = createPushService(createPushStore(directory), peer.options);
    const results = await Promise.all([
      first.sendInactivityNudges(0),
      second.sendInactivityNudges(0),
    ]);
    expect(results.reduce((count, result) => count + result.sent, 0)).toBe(1);
    expect(peer.received).toHaveLength(1);
    expect(peer.received[0].payload.tag).toBe('inactivity-nudge');
    const reopened = createPushStore(directory);
    expect(await reopened.getInactiveSubscriptions(0)).toEqual([]);
    expect((await createPushService(reopened, peer.options).sendInactivityNudges(0)).total).toBe(0);
    const reservedAt = (await reopened.getSubscription(subscription.endpoint))!.lastNotifiedAt!;
    await reopened.upsertSubscription(subscription);
    expect(
      (await createPushStore(directory).getSubscription(subscription.endpoint))!.lastNotifiedAt
    ).toBe(reservedAt);
    expect(
      await reopened.reserveInactiveSubscriptions(0, reservedAt + INACTIVITY_COOLDOWN_MS - 1)
    ).toEqual([]);
    expect(
      await reopened.reserveInactiveSubscriptions(0, reservedAt + INACTIVITY_COOLDOWN_MS)
    ).toHaveLength(1);
    // A reservation survives a stop before the transport is started, too.
    expect(
      await createPushStore(directory).reserveInactiveSubscriptions(
        0,
        reservedAt + INACTIVITY_COOLDOWN_MS
      )
    ).toEqual([]);
    expect(peer.received).toHaveLength(1);
  } finally {
    await peer.close();
    await rm(directory, { recursive: true, force: true });
  }
}

async function campaign_concurrent_processes_recover_interrupted_send_without_blind_redelivery() {
  const directory = await scratchDirectory();
  const peer = await pushEndpoint((message, response) => {
    if (message.route !== '/uncertain') response.writeHead(201).end();
  });
  const children: ReturnType<typeof worker>[] = [];
  try {
    const store = createPushStore(directory);
    for (const route of ['/accepted', '/uncertain', '/pending'])
      await store.upsertSubscription(peer.subscription(route));
    await store.addCampaign({ id: 'interrupted', title: 'Campaign', body: 'Restart evidence' });
    const options = {
      ...peer.options,
      ca: undefined,
      networkTimeoutMs: 5_000,
      routingBudgetMs: 10_000,
      loopbackPort: peer.port,
    };
    children.push(
      worker('campaign', directory, JSON.stringify(options)),
      worker('campaign', directory, JSON.stringify(options))
    );
    await Promise.all(children.map((child) => child.message('ready')));
    children.forEach((child) => child.child.send('start'));
    await waitUntil(() => peer.received.some((message) => message.route === '/uncertain'));
    await new Promise((resolve) => setTimeout(resolve, 100));
    expect(peer.received.map((message) => message.route)).toEqual(['/accepted', '/uncertain']);
    await Promise.all(children.map((child) => child.stop()));
    const reopened = createPushStore(directory);
    const interrupted = (await reopened.getCampaign('interrupted'))!;
    expect(interrupted.recipients?.map((recipient) => recipient.status)).toEqual([
      'sent',
      'in_flight',
      'pending',
    ]);
    expect(interrupted.sentAt).toBeUndefined();
    expect(
      await reopened.claimCampaign(interrupted.id, [], interrupted.claim!.expiresAt - 1)
    ).toBeNull();
    const recovered = (await reopened.claimCampaign(
      interrupted.id,
      [],
      interrupted.claim!.expiresAt
    ))!;
    expect(recovered.recipients?.map((recipient) => recipient.status)).toEqual([
      'sent',
      'uncertain',
      'pending',
    ]);
    await expect(
      reopened.recordCampaignAttempt(
        interrupted.id,
        interrupted.claim!.token,
        interrupted.recipients![1].endpoint,
        'sent',
        interrupted.claim!.expiresAt
      )
    ).rejects.toThrow('Campaign claim lost');
    await reopened.finishCampaignClaim(
      interrupted.id,
      recovered.claim!.token,
      interrupted.claim!.expiresAt
    );
    const service = createPushService(createPushStore(directory), peer.options);
    await service.processPendingCampaigns();
    expect(peer.received.map((message) => message.route)).toEqual([
      '/accepted',
      '/uncertain',
      '/pending',
    ]);
    expect(
      (await createPushStore(directory).getCampaign(interrupted.id))!.recipients?.map(
        (recipient) => recipient.status
      )
    ).toEqual(['sent', 'uncertain', 'sent']);
    expect((await createPushStore(directory).getCampaign(interrupted.id))!.sentAt).toBeUndefined();
    await service.processPendingCampaigns();
    expect(peer.received).toHaveLength(3);
    // The receiver has an actual decrypted receipt; explicitly reconcile that
    // evidence instead of retrying an unknown delivery automatically.
    expect(peer.received[1].payload).toMatchObject({
      title: 'Campaign',
      tag: 'campaign-interrupted',
    });
    await reopened.reconcileCampaignRecipient(
      interrupted.id,
      interrupted.recipients![1].endpoint,
      'sent'
    );
    await service.processPendingCampaigns();
    const completed = (await createPushStore(directory).getCampaign(interrupted.id))!;
    expect(completed.sentAt).toBeTypeOf('number');
    expect(completed.sentCount).toBe(3);
    expect(await reopened.getPendingCampaigns()).toEqual([]);
    expect(peer.received).toHaveLength(3);
  } finally {
    await Promise.all(children.map((child) => child.stop()));
    await peer.close();
    await rm(directory, { recursive: true, force: true });
  }
}

async function campaign_transient_rejections_remain_retryable_after_reopen_and_complete_only_terminal_recipients() {
  const directory = await scratchDirectory();
  let recovered = false;
  const peer = await pushEndpoint((message, response) => {
    response
      .writeHead(
        message.route === '/expired'
          ? 410
          : message.route === '/transient' && !recovered
            ? 503
            : 201
      )
      .end();
  });
  try {
    const store = createPushStore(directory);
    for (const route of ['/accepted', '/expired', '/transient'])
      await store.upsertSubscription(peer.subscription(route));
    await store.addCampaign({
      id: 'retry',
      title: 'Retry campaign',
      body: 'Keep rejected attempts',
    });
    await createPushService(store, peer.options).processPendingCampaigns();
    const reopened = createPushStore(directory);
    const campaign = (await reopened.getCampaign('retry'))!;
    expect(campaign.sentAt).toBeUndefined();
    expect(campaign.recipients?.map((recipient) => recipient.status)).toEqual([
      'sent',
      'removed',
      'retry',
    ]);
    const rejected = campaign.recipients![2];
    expect(rejected.attempts).toBe(1);
    expect(rejected.retryAt).toBeGreaterThan(Date.now());
    expect(await reopened.getSubscription(campaign.recipients![1].endpoint)).toBeNull();
    expect(await reopened.getSubscription(rejected.endpoint)).not.toBeNull();
    await expect(reopened.markCampaignSent('retry', 1, 2)).rejects.toThrow(
      'Campaign has unsettled recipients'
    );
    await createPushService(reopened, peer.options).processPendingCampaigns();
    expect(peer.received).toHaveLength(3);
    expect((await reopened.getPendingCampaigns()).map((item) => item.id)).toEqual(['retry']);
    recovered = true;
    // Drive the production scheduler at the persisted retry boundary. The
    // real transport's result is consumed by the same durable attempt API.
    const claimed = (await reopened.claimCampaign('retry', [], rejected.retryAt!))!;
    await expect(
      reopened.beginCampaignAttempt(
        'retry',
        claimed.claim!.token,
        rejected.endpoint,
        rejected.retryAt! - 1
      )
    ).rejects.toThrow('Campaign recipient is not eligible');
    await reopened.beginCampaignAttempt(
      'retry',
      claimed.claim!.token,
      rejected.endpoint,
      rejected.retryAt!
    );
    const result = await createPushService(reopened, peer.options).sendToSubscription(
      (await reopened.getSubscription(rejected.endpoint))!,
      { title: claimed.title, body: claimed.body }
    );
    expect(result.success).toBe(true);
    await reopened.recordCampaignAttempt(
      'retry',
      claimed.claim!.token,
      rejected.endpoint,
      'sent',
      rejected.retryAt!
    );
    await reopened.finishCampaignClaim('retry', claimed.claim!.token, rejected.retryAt!);
    const completed = (await createPushStore(directory).getCampaign('retry'))!;
    expect(completed.sentAt).toBe(rejected.retryAt);
    expect(completed.sentCount).toBe(2);
    expect(completed.failedCount).toBe(1);
    expect(completed.recipients?.map((recipient) => recipient.attempts)).toEqual([1, 1, 2]);
    expect(peer.received.map((message) => message.route)).toEqual([
      '/accepted',
      '/expired',
      '/transient',
      '/transient',
    ]);
    expect(await createPushStore(directory).getPendingCampaigns()).toEqual([]);
  } finally {
    await peer.close();
    await rm(directory, { recursive: true, force: true });
  }
}

async function campaign_retry_policy_persists_exhaustion_without_marking_transient_attempts_sent() {
  const directory = await scratchDirectory();
  const peer = await pushEndpoint((_, response) => response.writeHead(503).end());
  try {
    const store = createPushStore(directory);
    const subscription = peer.subscription('/outage');
    await store.upsertSubscription(subscription);
    await store.addCampaign({ id: 'exhaust', title: 'Outage', body: 'Bounded retry policy' });
    let now = Date.now();
    for (let index = 0; index < CAMPAIGN_MAX_ATTEMPTS; index++) {
      const reopened = createPushStore(directory);
      const claimed = (await reopened.claimCampaign('exhaust', [subscription], now))!;
      await reopened.beginCampaignAttempt(
        'exhaust',
        claimed.claim!.token,
        subscription.endpoint,
        now
      );
      const outcome = await createPushService(reopened, peer.options).sendToSubscription(
        subscription,
        { title: claimed.title, body: claimed.body }
      );
      expect(outcome).toEqual({
        success: false,
        removed: false,
        retryable: true,
        uncertain: false,
      });
      await reopened.recordCampaignAttempt(
        'exhaust',
        claimed.claim!.token,
        subscription.endpoint,
        'retry',
        now
      );
      await reopened.finishCampaignClaim('exhaust', claimed.claim!.token, now);
      const recorded = (await createPushStore(directory).getCampaign('exhaust'))!;
      if (index < CAMPAIGN_MAX_ATTEMPTS - 1) {
        expect(recorded.sentAt).toBeUndefined();
        expect(recorded.recipients![0].status).toBe('retry');
        expect(recorded.recipients![0].retryAt).toBeGreaterThan(now);
        now = recorded.recipients![0].retryAt!;
      } else {
        expect(recorded.recipients![0]).toMatchObject({
          status: 'exhausted',
          attempts: CAMPAIGN_MAX_ATTEMPTS,
        });
        expect(recorded.sentCount).toBe(0);
        expect(recorded.failedCount).toBe(1);
        expect(recorded.sentAt).toBe(now);
      }
    }
    expect(peer.received).toHaveLength(CAMPAIGN_MAX_ATTEMPTS);
    expect(await createPushStore(directory).getPendingCampaigns()).toEqual([]);
    expect(await createPushStore(directory).getSubscription(subscription.endpoint)).not.toBeNull();
  } finally {
    await peer.close();
    await rm(directory, { recursive: true, force: true });
  }
}

test(
  store_corruption_reopen_refuses_empty_writes_and_verifies_backup,
  store_corruption_reopen_refuses_empty_writes_and_verifies_backup
);
test(
  store_migration_and_insertion_serialize_in_separate_processes,
  store_migration_and_insertion_serialize_in_separate_processes,
  15_000
);
test(
  store_publication_crash_boundaries_reopen_with_verified_generations,
  store_publication_crash_boundaries_reopen_with_verified_generations,
  15_000
);
test(
  store_live_holder_dead_takeover_and_late_release_keep_ownership,
  store_live_holder_dead_takeover_and_late_release_keep_ownership,
  15_000
);
test(
  push_recovery_preserves_provider_validation_and_subscription_ownership,
  push_recovery_preserves_provider_validation_and_subscription_ownership
);
test(
  push_deadlines_bound_stalled_tls_and_trickling_batches,
  push_deadlines_bound_stalled_tls_and_trickling_batches,
  10_000
);
test(
  inactivity_atomic_cooldown_survives_concurrent_scans_and_reopen,
  inactivity_atomic_cooldown_survives_concurrent_scans_and_reopen
);
test(
  campaign_concurrent_processes_recover_interrupted_send_without_blind_redelivery,
  campaign_concurrent_processes_recover_interrupted_send_without_blind_redelivery,
  15_000
);
test(
  campaign_transient_rejections_remain_retryable_after_reopen_and_complete_only_terminal_recipients,
  campaign_transient_rejections_remain_retryable_after_reopen_and_complete_only_terminal_recipients
);
test(
  campaign_retry_policy_persists_exhaustion_without_marking_transient_attempts_sent,
  campaign_retry_policy_persists_exhaustion_without_marking_transient_attempts_sent
);
