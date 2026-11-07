// ── Push Notification Service ────────────────────────────────────────────────
// Server-side Web Push delivery via VAPID. Handles single, targeted, and
// broadcast push with retry, cleanup of expired subscriptions, and campaigns.

import webpush from 'web-push';
import { Agent, type AgentOptions } from 'node:https';
import {
  parseStoredSubscription,
  pushStore,
  CAMPAIGN_CLAIM_MS,
  type PushStore,
  type StoredSubscription,
} from './push-store';

export interface PushServiceOptions {
  vapidDetails: { subject: string; publicKey: string; privateKey: string };
  networkTimeoutMs?: number;
  routingBudgetMs?: number;
  ca?: AgentOptions['ca'];
  createAgent?: (options: AgentOptions) => Agent;
}

export interface DeliveryOutcome {
  success: boolean;
  removed: boolean;
  retryable: boolean;
  uncertain: boolean;
}

// ── Push Payload Builder ────────────────────────────────────────────────────

export interface PushPayload {
  title: string;
  body: string;
  icon?: string;
  badge?: string;
  image?: string;
  url?: string;
  tag?: string;
  actions?: { action: string; title: string }[];
  requireInteraction?: boolean;
}

function buildPayload(payload: PushPayload): string {
  return JSON.stringify({
    title: payload.title,
    body: payload.body,
    icon: payload.icon || '/wallet/icons/android/launchericon-192x192.png',
    badge: payload.badge || '/wallet/icons/android/launchericon-96x96.png',
    image: payload.image,
    url: payload.url || '/wallet/',
    tag: payload.tag || 'paxeer-push',
    actions: payload.actions || [],
    requireInteraction: payload.requireInteraction || false,
  });
}

// ── Send to a single subscription ───────────────────────────────────────────

export interface SendResult {
  total: number;
  sent: number;
  failed: number;
  removed: number;
}

export function createPushService(store: PushStore, options: PushServiceOptions) {
  const networkTimeoutMs = options.networkTimeoutMs ?? 3_000;
  const routingBudgetMs = options.routingBudgetMs ?? 10_000;
  if (
    ![networkTimeoutMs, routingBudgetMs].every(
      (value) => Number.isSafeInteger(value) && value > 0 && value < CAMPAIGN_CLAIM_MS / 2
    )
  )
    throw new RangeError('Push deadlines must fit within the campaign claim');
  const {
    getAllSubscriptions,
    getSubscriptionsByWallet,
    getSubscriptionsByTag,
    reserveInactiveSubscriptions,
    removeSubscription,
    updateLastNotified,
    getPendingCampaigns,
    claimCampaign,
    beginCampaignAttempt,
    recordCampaignAttempt,
    finishCampaignClaim,
  } = store;

  function ensureVapid(): void {
    if (!options.vapidDetails.publicKey || !options.vapidDetails.privateKey)
      throw new Error('VAPID keys not configured');
  }

  async function sendToSubscription(
    sub: StoredSubscription,
    payload: PushPayload,
    deadline = Date.now() + routingBudgetMs
  ): Promise<DeliveryOutcome> {
    return deliverToSubscription(sub, payload, deadline);
  }

  async function deliverToSubscription(
    sub: StoredSubscription,
    payload: PushPayload,
    deadline: number,
    connected?: () => void
  ): Promise<DeliveryOutcome> {
    sub = parseStoredSubscription(sub);
    ensureVapid();
    const remaining = deadline - Date.now();
    if (remaining <= 0)
      return { success: false, removed: false, retryable: true, uncertain: false };
    // Each request owns its agent so the absolute deadline can destroy even a
    // trickling response that never reaches the library's socket timeout.
    const agentOptions: AgentOptions = { keepAlive: false, ca: options.ca };
    const agent = options.createAgent ? options.createAgent(agentOptions) : new Agent(agentOptions);
    if (connected) {
      const createConnection = agent.createConnection.bind(agent);
      agent.createConnection = (connectionOptions, callback) => {
        const socket = createConnection(connectionOptions, callback);
        socket.once('secureConnect', connected);
        return socket;
      };
    }
    let timer: ReturnType<typeof setTimeout> | undefined;
    try {
      const expired = new Promise<never>((_, reject) => {
        timer = setTimeout(() => {
          agent.destroy();
          reject(new Error('Push routing deadline exceeded'));
        }, remaining);
      });
      await Promise.race([
        webpush.sendNotification(
          { endpoint: sub.endpoint, keys: sub.keys },
          buildPayload(payload),
          {
            TTL: 86400,
            urgency: 'normal',
            timeout: Math.min(networkTimeoutMs, remaining),
            vapidDetails: options.vapidDetails,
            agent,
          }
        ),
        expired,
      ]);
      return { success: true, removed: false, retryable: false, uncertain: false };
    } catch (error: unknown) {
      const statusCode =
        typeof error === 'object' &&
        error !== null &&
        'statusCode' in error &&
        typeof error.statusCode === 'number'
          ? error.statusCode
          : undefined;
      if (statusCode === 404 || statusCode === 410) {
        await removeSubscription(sub.endpoint);
        return { success: false, removed: true, retryable: false, uncertain: false };
      }
      return {
        success: false,
        removed: false,
        retryable: statusCode !== undefined,
        uncertain: statusCode === undefined,
      };
    } finally {
      clearTimeout(timer);
      agent.destroy();
    }
  }

  // ── Send to multiple subscriptions ──────────────────────────────────────────

  async function sendToMany(subs: StoredSubscription[], payload: PushPayload): Promise<SendResult> {
    const result: SendResult = { total: subs.length, sent: 0, failed: 0, removed: 0 };
    if (subs.length === 0) return result;

    // Send in batches of 50 to avoid overwhelming the push service
    const BATCH_SIZE = 50;
    const successEndpoints: string[] = [];

    const deadline = Date.now() + routingBudgetMs;
    for (let i = 0; i < subs.length; i += BATCH_SIZE) {
      if (Date.now() >= deadline) {
        result.failed += subs.length - i;
        break;
      }
      const batch = subs.slice(i, i + BATCH_SIZE);
      const deliveries: Promise<PromiseSettledResult<DeliveryOutcome>[]>[] = [];
      let connecting: Promise<void>[] = [];
      for (const sub of batch) {
        // Bound pending TLS handshakes separately from responses: stalled or
        // trickling recipients must not block the rest of this batch, and a
        // burst of encryption/handshakes must not starve existing socket timers.
        let connected!: () => void;
        connecting.push(
          new Promise<void>((resolve) => {
            connected = resolve;
          })
        );
        const delivery = deliverToSubscription(sub, payload, deadline, connected);
        deliveries.push(Promise.allSettled([delivery]));
        void delivery.then(connected, connected);
        if (connecting.length === 4) {
          await Promise.all(connecting);
          connecting = [];
        }
      }
      const results = (await Promise.all(deliveries)).flat();

      for (let j = 0; j < results.length; j++) {
        const r = results[j];
        if (r.status === 'fulfilled') {
          if (r.value.success) {
            result.sent++;
            successEndpoints.push(batch[j].endpoint);
          } else if (r.value.removed) {
            result.removed++;
          } else {
            result.failed++;
          }
        } else {
          result.failed++;
        }
      }
    }

    // Update lastNotifiedAt for successful sends
    if (successEndpoints.length > 0) {
      await updateLastNotified(successEndpoints);
    }

    return result;
  }

  // ── Public API ──────────────────────────────────────────────────────────────

  /** Send push to ALL subscribers. */
  async function sendToAll(payload: PushPayload): Promise<SendResult> {
    return sendToMany(await getAllSubscriptions(), payload);
  }

  /** Send push to a specific wallet address (all their devices). */
  async function sendToWallet(walletAddress: string, payload: PushPayload): Promise<SendResult> {
    return sendToMany(await getSubscriptionsByWallet(walletAddress), payload);
  }

  /** Send push to subscribers with a specific tag. */
  async function sendToTag(tag: string, payload: PushPayload): Promise<SendResult> {
    return sendToMany(await getSubscriptionsByTag(tag), payload);
  }

  /** Send push to specific wallet addresses. */
  async function sendToWallets(
    walletAddresses: string[],
    payload: PushPayload
  ): Promise<SendResult> {
    const normalized = new Set(walletAddresses.map((a) => a.toLowerCase()));
    const subs = (await getAllSubscriptions()).filter((s) =>
      normalized.has(s.walletAddress.toLowerCase())
    );
    return sendToMany(subs, payload);
  }

  // ── Transaction received notification ───────────────────────────────────────

  async function notifyTransactionReceived(
    walletAddress: string,
    txHash: string,
    fromAddress?: string,
    value?: string,
    symbol?: string
  ): Promise<SendResult> {
    const amountStr = value && symbol ? `${value} ${symbol}` : 'a transaction';
    const fromStr = fromAddress
      ? ` from ${fromAddress.slice(0, 6)}...${fromAddress.slice(-4)}`
      : '';

    return sendToWallet(walletAddress, {
      title: 'Transaction Received',
      body: `You received ${amountStr}${fromStr}.`,
      tag: `tx-${txHash.slice(0, 16)}`,
      url: '/',
      actions: [{ action: 'view', title: 'View' }],
    });
  }

  // ── Inactivity nudge ────────────────────────────────────────────────────────

  async function sendInactivityNudges(
    thresholdMs: number = 3 * 24 * 60 * 60 * 1000 // 3 days
  ): Promise<SendResult> {
    ensureVapid();
    const inactive = await reserveInactiveSubscriptions(thresholdMs);
    return sendToMany(inactive, {
      title: 'We miss you!',
      body: 'Your portfolio may have changed — check in to see how your tokens are doing.',
      tag: 'inactivity-nudge',
      url: '/',
    });
  }

  // ── Process scheduled campaigns ─────────────────────────────────────────────

  async function processPendingCampaigns(): Promise<{
    processed: number;
    results: { campaignId: string; result: SendResult }[];
  }> {
    ensureVapid();
    const pending = await getPendingCampaigns();
    const deadline = Date.now() + routingBudgetMs;
    const results: { campaignId: string; result: SendResult }[] = [];

    for (const candidate of pending) {
      if (Date.now() >= deadline) break;
      const campaign = candidate;

      let subs: StoredSubscription[];

      if (campaign.targetAddresses && campaign.targetAddresses.length > 0) {
        const normalized = new Set(campaign.targetAddresses.map((a) => a.toLowerCase()));
        subs = (await getAllSubscriptions()).filter((s) =>
          normalized.has(s.walletAddress.toLowerCase())
        );
      } else if (campaign.targetTags && campaign.targetTags.length > 0) {
        const tagSets = await Promise.all(campaign.targetTags.map((t) => getSubscriptionsByTag(t)));
        const seen = new Set<string>();
        subs = [];
        for (const set of tagSets) {
          for (const s of set) {
            if (!seen.has(s.endpoint)) {
              seen.add(s.endpoint);
              subs.push(s);
            }
          }
        }
      } else {
        subs = await getAllSubscriptions();
      }

      const claimed = await claimCampaign(campaign.id, subs);
      if (!claimed?.claim) continue;
      const token = claimed.claim.token;
      const payload: PushPayload = {
        title: campaign.title,
        body: campaign.body,
        url: campaign.url,
        icon: campaign.icon,
        image: campaign.image,
        tag: campaign.tag || `campaign-${campaign.id}`,
      };

      const recipients = claimed.recipients ?? [];
      const result: SendResult = { total: recipients.length, sent: 0, failed: 0, removed: 0 };
      for (const recipient of recipients) {
        if (
          !['pending', 'retry'].includes(recipient.status) ||
          (recipient.retryAt !== undefined && recipient.retryAt > Date.now())
        )
          continue;
        if (Date.now() >= deadline) break;
        await beginCampaignAttempt(campaign.id, token, recipient.endpoint);
        const sub =
          subs.find((item) => item.endpoint === recipient.endpoint) ??
          (await store.getSubscription(recipient.endpoint));
        const outcome = sub
          ? await sendToSubscription(sub, payload, deadline)
          : { success: false, removed: true, uncertain: false, retryable: false };
        const status = outcome.success
          ? 'sent'
          : outcome.removed
            ? 'removed'
            : outcome.uncertain
              ? 'uncertain'
              : 'retry';
        await recordCampaignAttempt(campaign.id, token, recipient.endpoint, status);
        if (outcome.success) {
          result.sent++;
          await updateLastNotified([recipient.endpoint]);
        } else if (outcome.removed) result.removed++;
        else result.failed++;
      }
      await finishCampaignClaim(campaign.id, token);
      results.push({ campaignId: campaign.id, result });
    }

    return { processed: results.length, results };
  }

  // ── Stats ───────────────────────────────────────────────────────────────────

  async function getPushStats() {
    const subs = await getAllSubscriptions();
    const now = Date.now();
    const day = 24 * 60 * 60 * 1000;

    return {
      totalSubscriptions: subs.length,
      activeLastDay: subs.filter((s) => now - s.lastActiveAt < day).length,
      activeLastWeek: subs.filter((s) => now - s.lastActiveAt < 7 * day).length,
      uniqueWallets: new Set(subs.map((s) => s.walletAddress.toLowerCase())).size,
    };
  }

  return {
    sendToSubscription,
    sendToAll,
    sendToWallet,
    sendToTag,
    sendToWallets,
    notifyTransactionReceived,
    sendInactivityNudges,
    processPendingCampaigns,
    getPushStats,
  };
}

const defaultService = createPushService(pushStore, {
  vapidDetails: {
    subject: process.env.VAPID_SUBJECT || 'mailto:admin@paxeer.app',
    publicKey: process.env.NEXT_PUBLIC_VAPID_PUBLIC_KEY || '',
    privateKey: process.env.VAPID_PRIVATE_KEY || '',
  },
});
export const {
  sendToSubscription,
  sendToAll,
  sendToWallet,
  sendToTag,
  sendToWallets,
  notifyTransactionReceived,
  sendInactivityNudges,
  processPendingCampaigns,
  getPushStats,
} = defaultService;
