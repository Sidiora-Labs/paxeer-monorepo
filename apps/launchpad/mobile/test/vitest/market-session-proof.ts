import { createServer } from 'node:http';
import { expect } from 'vitest';
import { readFileSync } from 'node:fs';
import {
  SdkClient,
  decodeV2Contract,
  type StreamState,
  type V2TopicSnapshot,
} from '../../../../../sdk/launchpad/packages/sdk/src/index.ts';
import { MarketSession } from '../../src/runtime/market-session.ts';
import type { PublicConfiguration } from '../../src/runtime/service-client.ts';

export async function marketSessionUsesV2Snapshots() {
  const recorded = JSON.parse(
    readFileSync(
      new URL('./fixtures/repair-22-33-929-v2.json', import.meta.url),
      'utf8',
    ),
  ) as { snapshot: unknown };
  const envelope = decodeV2Contract(
    'TopicSnapshotReadResponse',
    recorded.snapshot,
  );
  const snapshot = envelope.data.snapshot;
  const statsEntry = snapshot.topics.find((entry) =>
    entry.topic.startsWith('market.stats.'),
  );
  if (!statsEntry)
    throw new Error('Producer recording must include market.stats');
  const statsTopic = statsEntry.topic;
  const topic = statsTopic.replace('market.stats.', 'market.pool.');
  const supportedTopics = snapshot.topics.map((entry) => entry.topic);
  const requests: { method: string | undefined; url: URL }[] = [];
  const server = createServer((request, response) => {
    const url = new URL(request.url ?? '/', 'http://127.0.0.1');
    requests.push({ method: request.method, url });
    if (request.method !== 'GET' || url.pathname !== '/v2/snapshots') {
      response.writeHead(404);
      response.end();
      return;
    }
    if (
      url.searchParams.get('topics') !==
      [...supportedTopics].sort().join(',')
    ) {
      response.writeHead(400);
      response.end();
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
    const states: StreamState[] = [];
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
        state(state) {
          states.push(state);
        },
      },
    );
    await expect(session.foreground([topic])).rejects.toThrow(
      new TypeError('Invalid public market topic'),
    );
    expect(requests).toEqual([]);
    expect(states).toEqual([]);
    expect(snapshots).toEqual([]);
    await session.foreground(supportedTopics);
    expect(requests).toHaveLength(1);
    expect(requests[0]?.method).toBe('GET');
    expect(requests[0]?.url.pathname).toBe('/v2/snapshots');
    expect(requests[0]?.url.searchParams.get('topics')).toBe(
      [...supportedTopics].sort().join(','),
    );
    expect(snapshots).toEqual([snapshot]);
    expect(
      snapshots[0].topics.find((entry) => entry.topic === statsTopic),
    ).toEqual(statsEntry);
    expect(decodeV2Contract('PoolStats', statsEntry.payload)).toMatchObject(
      {
        poolAddress: statsTopic.slice('market.stats.'.length),
        buys24h: '1',
        sells24h: '1',
        volume24hRaw: '147278912',
        realUsdlBalanceRaw: '49721088',
        supplyValuationQuality: 'verified',
        totalSupplyValuationUsdlRaw: '10098226981',
      },
    );
    expect(states).toEqual(['closed', 'loading', 'closed']);
  } finally {
    session?.background();
    await new Promise<void>((resolve, reject) => {
      server.close((error) => (error ? reject(error) : resolve()));
      server.closeAllConnections();
    });
  }
}
