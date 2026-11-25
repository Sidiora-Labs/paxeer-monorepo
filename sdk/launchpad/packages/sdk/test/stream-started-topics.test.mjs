import { test } from 'node:test';
import assert from 'node:assert/strict';
import { SnapshotStream, SnapshotStreamV2 } from '../dist/index.js';

const pool = `0x${'a'.repeat(40)}`;
const topics = ['trades', 'price', 'stats'].map(kind => `market.${kind}.${pool}`);
const at = '2026-01-02T12:00:00.000Z';
const position = { chainId: '125', height: '100', blockHash: `0x${'1'.repeat(64)}`, transactionHash: `0x${'2'.repeat(64)}`, transactionIndex: '0', logIndex: '0' };
const watermark = { chainId: '125', height: '100', blockHash: position.blockHash, integrity: 'healthy', observedAt: at, projectionRevision: '1' };
const stats = { poolAddress: pool, buys24h: '0', sells24h: '0', changeBaselineQuality: 'insufficient_history', circulatingMarketCapUsdlRaw: null, holderCount: '0', indicativeTvlUsdlRaw: null, priceChangeBps24h: null, realUsdlBalanceRaw: '0', sourceUpperPosition: position, supplyValuationQuality: 'unavailable', totalSupplyValuationUsdlRaw: null, valuationChangeUsdlRaw: null, valuationMethod: null, volume24hRaw: '0', watermark, windowStart: '2026-01-01T12:00:00.000Z', windowEnd: at };
const price = { numerator: '1', denominator: '100', baseAsset: { kind: 'erc20', chainId: '125', address: pool, currencyCode: null, decimals: 18 }, quoteAsset: { kind: 'erc20', chainId: '125', address: pool, currencyCode: null, decimals: 6 }, source: { kind: 'post_swap_spot', amountBasis: 'effective_reserves', sourceEventId: null, position, providerId: null, observedAt: at, sourceReference: null } };
const trade = { eventId: 'first-fill', poolAddress: pool, sender: pool, recipient: null, side: 'buy', tokenAmountRaw: '100', usdlAmountRaw: '1', position, feeRaw: '0', feeAssetAddress: pool, feeAssetDecimals: 6, feeDisposition: 'distributed_usdl', paidInputRaw: '1', pricedInputRaw: '1', ammInputAfterFeeRaw: '1', inputDerivationQuality: 'exact', recipientAttribution: 'unavailable', recipientEvidenceRefs: [], emittingContract: pool, codeEpoch: '1' };
const entries = [trade, price, stats].map((payload, index) => ({ topic: topics[index], sequence: '1', revision: '1', watermark, payload }));

function snapshot(selected = [entries[2]]) {
  return { snapshotId: '11111111-1111-4111-8111-111111111111', topics: structuredClone(selected), resumeCursor: 'started-topics.cursor', generatedAt: new Date().toISOString() };
}

function transportFor(Stream, initial) {
  const installed = [], applied = [], subscriptions = [], reads = [], tickets = [], channels = [];
  const stream = new Stream({
    transport: {
      snapshot: async requested => { reads.push([...requested]); return structuredClone(initial); },
      ticket: async requested => {
        tickets.push([...requested]);
        return { ticket: 'a'.repeat(64), subprotocol: Stream === SnapshotStream ? 'sidiora.v1' : 'sidiora.v2', expiresAt: new Date(Date.now() + 29000).toISOString() };
      },
      connect: async input => {
        channels.push(input);
        return { send: frame => subscriptions.push(frame), close() {} };
      },
    },
    installSnapshot: value => installed.push(value),
    applyEvent: value => applied.push(value),
  });
  const control = (op, cursor = null) => channels.at(-1).receive({ op, subscriptionId: subscriptions.at(-1).subscriptionId, cursor, code: null, snapshotUrl: null, watermark: null });
  return { stream, installed, applied, subscriptions, reads, tickets, channels, control };
}

async function streamStartedTopicsInstallsSubsetAndSubscribesEveryTopic() {
  for (const Stream of [SnapshotStream, SnapshotStreamV2]) {
    const initial = snapshot();
    const h = transportFor(Stream, initial);
    try {
      await h.stream.start(topics);
      assert.deepEqual(h.installed, [initial]);
      assert.deepEqual(h.reads, [topics]);
      assert.deepEqual(h.tickets, [topics]);
      assert.deepEqual(h.subscriptions[0].topics, topics);
      assert.equal(h.subscriptions[0].afterCursor, initial.resumeCursor);
      assert.deepEqual(h.stream.waitingTopics, topics.slice(0, 2));
      h.control('subscribed');
      h.control('checkpoint', 'checkpoint.cursor');
      assert.equal(h.stream.state, 'live');
      h.channels.at(-1).receive({ ...entries[2], sequence: '2', revision: '2', eventId: 'stats-update', occurredAt: at, coalesced: false, sourceSequenceFirst: null, sourceSequenceLast: null });
      assert.equal(h.applied.length, 1);
      assert.equal(h.applied[0].topic, topics[2]);
      const reported = h.stream.waitingTopics;
      reported.pop();
      assert.deepEqual(h.stream.waitingTopics, topics.slice(0, 2));
    } finally { h.stream.close(); }
  }
}
test(streamStartedTopicsInstallsSubsetAndSubscribesEveryTopic);

async function streamStartedTopicsRefusesForeignDuplicateAndEmptyEntries() {
  for (const Stream of [SnapshotStream, SnapshotStreamV2]) {
    for (const selected of [[], [entries[2], entries[2]], [{ ...entries[2], topic: `market.stats.0x${'b'.repeat(40)}` }]]) {
      const h = transportFor(Stream, snapshot(selected));
      try {
        await h.stream.start(topics);
        assert.equal(h.stream.state, Stream === SnapshotStream ? 'resync-required' : 'stale');
        assert.deepEqual(h.installed, []);
        assert.deepEqual(h.subscriptions, []);
        assert.deepEqual(h.tickets, []);
      } finally { h.stream.close(); }
    }
  }
}
test(streamStartedTopicsRefusesForeignDuplicateAndEmptyEntries);

async function streamStartedTopicsPreservesCompleteSnapshots() {
  for (const Stream of [SnapshotStream, SnapshotStreamV2]) {
    const initial = snapshot(entries);
    const h = transportFor(Stream, initial);
    try {
      await h.stream.start(topics);
      assert.deepEqual(h.installed, [initial]);
      assert.deepEqual(h.stream.waitingTopics, []);
      assert.deepEqual(h.subscriptions[0].topics, topics);
      assert.equal(h.subscriptions[0].afterCursor, initial.resumeCursor);
      h.control('subscribed');
      h.control('checkpoint', 'all-topics.cursor');
      assert.equal(h.stream.state, 'live');
    } finally { h.stream.close(); }
  }
}
test(streamStartedTopicsPreservesCompleteSnapshots);

async function streamStartedTopicsKeepsWaitingTopicsOnResume() {
  for (const Stream of [SnapshotStream, SnapshotStreamV2]) {
    const h = transportFor(Stream, snapshot());
    try {
      await h.stream.start(topics);
      h.control('subscribed');
      h.control('checkpoint', 'resume-started.cursor');
      if (Stream === SnapshotStreamV2) {
        await h.stream.reconnect();
        assert.equal(h.reads.length, 1);
        assert.equal(h.subscriptions.at(-1).afterCursor, 'resume-started.cursor');
      } else {
        await h.stream.resumeVisible();
        assert.deepEqual(h.reads, [topics, topics]);
      }
      assert.deepEqual(h.subscriptions.at(-1).topics, topics);
      assert.deepEqual(h.tickets, [topics, topics]);
      assert.deepEqual(h.stream.waitingTopics, topics.slice(0, 2));
      h.control('subscribed');
      h.control('checkpoint', 'resumed.cursor');
      assert.equal(h.stream.state, 'live');
      h.channels.at(-1).receive({ ...entries[0], eventId: 'first-trade-frame', occurredAt: at, coalesced: false, sourceSequenceFirst: null, sourceSequenceLast: null });
      assert.equal(h.stream.state, 'resync-required');
      assert.deepEqual(h.applied, []);
    } finally { h.stream.close(); }
    assert.deepEqual(h.stream.waitingTopics, []);
  }
}
test(streamStartedTopicsKeepsWaitingTopicsOnResume);
