import { createServer } from 'node:http';
import { expect, test } from 'vitest';
import {
  SdkClient,
  type V2ContractTypes,
  type V2TopicSnapshot,
} from '../../../../../sdk/launchpad/packages/sdk/src/index.ts';
import { MarketSession } from '../../src/runtime/market-session.ts';
import type { PublicConfiguration } from '../../src/runtime/service-client.ts';

async function marketSessionUsesV2Snapshots() {
  const address = `0x${'ab'.repeat(20)}`;
  const hash = `0x${'cd'.repeat(32)}`;
  const topic = `market.pool.${address}`;
  const statsTopic = `market.stats.${address}`;
  const time = new Date().toISOString();
  const watermark = {
    blockHash: hash,
    chainId: '125',
    height: '1',
    integrity: 'healthy',
    observedAt: time,
    projectionRevision: '1',
  } satisfies V2ContractTypes['Watermark'];
  const snapshot: V2TopicSnapshot = {
    generatedAt: time,
    resumeCursor: 'snapshot-cursor',
    snapshotId: '123e4567-e89b-42d3-a456-426614174000',
    topics: [
      {
        topic: statsTopic,
        sequence: '1',
        revision: '1',
        watermark,
        payload: {
          buys24h: '0',
          changeBaselineQuality: 'unavailable',
          circulatingMarketCapUsdlRaw: null,
          holderCount: null,
          indicativeTvlUsdlRaw: null,
          poolAddress: address,
          priceChangeBps24h: null,
          realUsdlBalanceRaw: '1',
          sells24h: '0',
          sourceUpperPosition: {
            blockHash: hash,
            chainId: '125',
            height: '1',
            logIndex: '0',
            transactionHash: hash,
            transactionIndex: '0',
          },
          supplyValuationQuality: 'unavailable',
          totalSupplyValuationUsdlRaw: null,
          valuationChangeUsdlRaw: null,
          valuationMethod: null,
          volume24hRaw: '0',
          watermark,
          windowEnd: time,
          windowStart: new Date(Date.parse(time) - 86400000).toISOString(),
        },
      },
    ],
  };
  const envelope = {
    data: { snapshot },
    generatedAt: time,
    nextCursor: null,
    quality: 'complete',
    requestId: snapshot.snapshotId,
    revision: '1',
    watermark,
  } satisfies V2ContractTypes['TopicSnapshotReadResponse'];
  const unsupportedTopic = {
    code: 'INVALID_TOPIC',
    message: 'Use market.stats for pool statistics',
    requestId: snapshot.snapshotId,
    retryable: false,
    retryAfterMs: null,
    fieldErrors: [],
    details: null,
  } satisfies V2ContractTypes['Error'];
  const requests: { method: string | undefined; url: URL }[] = [];
  const server = createServer((request, response) => {
    const url = new URL(request.url ?? '/', 'http://127.0.0.1');
    requests.push({ method: request.method, url });
    if (request.method !== 'GET' || url.pathname !== '/v2/snapshots') {
      response.writeHead(404);
      response.end();
      return;
    }
    if (url.searchParams.get('topics') === topic) {
      response.writeHead(400, { 'Content-Type': 'application/json' });
      response.end(JSON.stringify(unsupportedTopic));
      return;
    }
    response.writeHead(200, { 'Content-Type': 'application/json' });
    response.end(JSON.stringify(envelope));
  });
  await new Promise<void>((resolve, reject) => {
    server.once('error', reject);
    server.listen(0, '127.0.0.1', resolve);
  });
  let session: MarketSession | undefined;
  try {
    const bound = server.address();
    if (!bound || typeof bound === 'string')
      throw new Error('Loopback port missing');
    const origin = `http://127.0.0.1:${bound.port}`;
    const streamOrigin = `ws://127.0.0.1:${bound.port}`;
    const config: PublicConfiguration = {
      apiOrigin: origin,
      identityOrigin: origin,
      mediaOrigin: origin,
      streamOrigin,
      approvedStreamOrigins: [streamOrigin],
      production: false,
      canonicalOrigin: origin,
      identityURI: `${origin}/identity`,
      manifestDigest: 'ab'.repeat(32),
      manifestURL: `${origin}/manifest.json`,
      optionalFeatures: { chat: false, live: false },
      approvedPlaybackOrigins: [],
    };
    const snapshots: V2TopicSnapshot[] = [];
    session = new MarketSession(
      { sdk: new SdkClient(config), config },
      {
        snapshot(value) {
          snapshots.push(value);
          session?.background();
        },
        event() {
          throw new Error(
            'Unexpected event before snapshot transport completes',
          );
        },
        state() {},
      },
    );
    await session.foreground([topic]);
    expect(requests[0]?.method).toBe('GET');
    expect(requests[0]?.url.pathname).toBe('/v2/snapshots');
    expect(requests[0]?.url.searchParams.get('topics')).toBe(topic);
    expect(requests).toHaveLength(1);
    expect(snapshots).toEqual([]);
    await session.foreground([statsTopic]);
    expect(requests).toHaveLength(2);
    expect(requests[1]?.method).toBe('GET');
    expect(requests[1]?.url.pathname).toBe('/v2/snapshots');
    expect(requests[1]?.url.searchParams.get('topics')).toBe(statsTopic);
    expect(snapshots).toEqual([snapshot]);
  } finally {
    session?.background();
    await new Promise<void>((resolve, reject) => {
      server.close((error) => (error ? reject(error) : resolve()));
      server.closeAllConnections();
    });
  }
}

test(marketSessionUsesV2Snapshots, marketSessionUsesV2Snapshots);
