import {
  SnapshotStreamV2,
  decodeV2Contract,
  browserStreamTransportV2,
  type StreamState,
  type V2StreamEvent,
  type V2TopicSnapshot,
} from '../../../../../sdk/launchpad/packages/sdk/src/index.ts';
import { retryDelay } from '../contracts.ts';
import type { MobileServices } from './service-client.ts';
export class MarketSession {
  private stream: SnapshotStreamV2;
  private topics: string[] = [];
  private timer: ReturnType<typeof setTimeout> | undefined;
  private attempts = 0;
  private active = false;
  private revision = 0;
  constructor(
    services: Pick<MobileServices, 'sdk' | 'config'>,
    callbacks: {
      snapshot(value: V2TopicSnapshot): void;
      event(value: V2StreamEvent): void;
      state(state: StreamState): void;
    },
  ) {
    this.stream = new SnapshotStreamV2({
      transport: browserStreamTransportV2(
        services.sdk,
        {
          applicationOrigin: services.config.canonicalOrigin,
          snapshotPath: '/v2/snapshots',
        },
        false,
      ),
      installSnapshot: callbacks.snapshot,
      applyEvent: callbacks.event,
      maxBuffer: 1000,
      onState: (state) => {
        callbacks.state(state);
        if (state === 'live') this.attempts = 0;
        if (this.active && ['stale', 'resync-required'].includes(state))
          this.retry();
      },
    });
  }
  async foreground(topics: string[]) {
    const uniqueTopics = [...new Set(topics)];
    decodeV2Contract('Subscribe', {
      op: 'subscribe',
      subscriptionId: 'mobile-market',
      topics: uniqueTopics,
      afterCursor: null,
    });
    // TopicName bounds strings but does not encode the public topic families.
    // Reject malformed input before closing a working session or scheduling retries.
    const addressTopic = /^market\.(?:price|stats|trades)\.0x[0-9a-fA-F]{40}$/;
    const candleTopic =
      /^market\.candles\.0x[0-9a-fA-F]{40}\.(?:1m|5m|15m|1h|4h|1d|1w)$/;
    const rankingTopic =
      /^market\.rankings\.(?:trending|breakout|volume|gainers|losers|movers|unusual|new)$/;
    if (
      uniqueTopics.some(
        (topic) =>
          !addressTopic.test(topic) &&
          !candleTopic.test(topic) &&
          !rankingTopic.test(topic),
      )
    )
      throw new TypeError('Invalid public market topic');
    this.background();
    this.active = true;
    const revision = this.revision;
    this.topics = uniqueTopics;
    try {
      await this.stream.start(this.topics);
    } catch {
      if (this.active && revision === this.revision) this.retry();
    }
  }
  private retry() {
    if (this.timer || !this.active) return;
    this.timer = setTimeout(
      () => {
        this.timer = undefined;
        if (this.active) {
          const revision = this.revision;
          void this.stream.start(this.topics).catch(() => {
            if (this.active && revision === this.revision) this.retry();
          });
        }
      },
      retryDelay(this.attempts++, Math.random()),
    );
  }
  background() {
    this.revision++;
    this.active = false;
    if (this.timer) clearTimeout(this.timer);
    this.timer = undefined;
    this.stream.close();
  }
}
