import assert from 'node:assert/strict';
import { createServer } from 'node:http';
import { once } from 'node:events';
import test from 'node:test';
import { RankingHttpClient, RankingHttpError, httpClient } from '../src/index.ts';

async function rankingRoundTrip() {
  const computedAt = '2026-01-01T00:00:00.000Z';
  const watermark = {
    chainId: '125', height: '100', blockHash: `0x${'ab'.repeat(32)}`,
    projectionRevision: '1', observedAt: computedAt, integrity: 'healthy',
  };
  const items = [1, 2].map(rank => ({
    poolAddress: `0x${String(rank).repeat(40)}`, category: 'trending',
    score: rank === 1 ? '0.000000000000000001' : '0.000000000000000000',
    rank, modelVersion: 'v1', computedAt,
  }));
  const firstPage = {
    data: { snapshot: {
      snapshotId: '00000000-0000-4000-8000-000000000001', category: 'trending',
      modelVersion: 'v1', computedAt, watermark, items: [items[0]],
      totalEligible: '2', truncated: true, replace: true,
    } },
    requestId: '00000000-0000-4000-8000-000000000002', generatedAt: computedAt,
    nextCursor: 'snapshot-page-2', revision: '1', watermark, quality: 'complete',
    freshness: {
      cutoffHeight: '100', sourceObservedAt: computedAt, generatedAt: computedAt,
      ageSeconds: 0, stale: false, staleReason: null, retryAfterSeconds: 10,
    },
  };
  const secondPage = {
    ...firstPage,
    data: { snapshot: { ...firstPage.data.snapshot, items: [items[1]] } },
    requestId: '00000000-0000-4000-8000-000000000003', nextCursor: null,
  };
  const fullPage = {
    ...firstPage,
    data: { snapshot: { ...firstPage.data.snapshot, items, truncated: false } },
    nextCursor: null,
  };
  const requests = [];
  const server = createServer((req, res) => {
    requests.push({ url: req.url, authorization: req.headers.authorization, accept: req.headers.accept });
    if (req.url.startsWith('/rankings/unusual')) {
      res.writeHead(503);
      res.end('unavailable');
      return;
    }
    if (req.url.startsWith('/rankings/movers')) {
      res.end('{"category":"movers","items":"invalid"}');
      return;
    }
    res.setHeader('Content-Type', 'application/json');
    const url = new URL(req.url, 'http://localhost');
    const page = url.searchParams.get('cursor') === firstPage.nextCursor
      ? secondPage : url.searchParams.has('limit') ? firstPage : fullPage;
    res.end(JSON.stringify(page));
  });
  server.listen(0, '127.0.0.1');
  await once(server, 'listening');
  try {
    const client = new RankingHttpClient();
    client.configure({
      baseUrl: `http://127.0.0.1:${server.address().port}/`,
      headers: { Authorization: 'Bearer local-test' },
    });
    const result = await client.getRankings('trending', { limit: 1, cursor: '' });
    assert.equal(result.data.snapshot.items[0].score, '0.000000000000000001');
    assert.deepEqual(result, firstPage);
    assert.equal(result.nextCursor, 'snapshot-page-2');
    assert.deepEqual(requests[0], {
      url: '/rankings/trending?limit=1&cursor=', authorization: 'Bearer local-test', accept: 'application/json',
    });
    const next = await client.getRankings('trending', { limit: 1, cursor: result.nextCursor });
    assert.deepEqual(next, secondPage);
    assert.equal(next.nextCursor, null);
    assert.equal(next.data.snapshot.snapshotId, result.data.snapshot.snapshotId);
    assert.equal(next.data.snapshot.items[0].rank, 2);
    assert.notEqual(next.data.snapshot.items[0].poolAddress, result.data.snapshot.items[0].poolAddress);
    assert.deepEqual(requests[1], {
      url: '/rankings/trending?limit=1&cursor=snapshot-page-2', authorization: 'Bearer local-test', accept: 'application/json',
    });
    for (const query of [undefined, {}, { limit: 1 }, { limit: 1, cursor: null }, { limit: 1, cursor: undefined }]) {
      assert.deepEqual(await client.getRankings('trending', query), query?.limit === undefined ? fullPage : firstPage);
      assert.equal(new URL(requests.at(-1).url, 'http://localhost').searchParams.has('cursor'), false);
    }
    await assert.rejects(client.getRankings('unusual'), e => e instanceof RankingHttpError && e.status === 503 && e.body === 'unavailable');
    await assert.rejects(client.getRankings('movers'), /Invalid rankings response/);
    await assert.rejects(client.getRankings('trending', { offset: -1 }), RangeError);
    const received = requests.length;
    for (const offset of [-1, 0, 2, undefined, null]) {
      await assert.rejects(client.getRankings('trending', { offset }), { name: 'RangeError', message: /offset.*cursor pagination/ });
    }
    for (const limit of [0, -1, 101, 1.5, NaN, Infinity]) {
      await assert.rejects(client.getRankings('trending', { limit }), { name: 'RangeError', message: /limit/ });
    }
    for (const cursor of [42, 'a'.repeat(2049)]) {
      await assert.rejects(client.getRankings('trending', { cursor }), { name: 'TypeError', message: /cursor/ });
    }
    await assert.rejects(client.getRankings('unknown'), TypeError);
    const controller = new AbortController();
    controller.abort();
    await assert.rejects(client.getRankings('trending', { signal: controller.signal }), { name: 'AbortError' });
    assert.equal(requests.length, received);
  } finally {
    server.close();
    await once(server, 'close');
  }
}

test('real HTTP round trip preserves decimal strings, pagination, headers, and errors', rankingRoundTrip);

test('configuration fails explicitly and accepts query-provider configure interface', async () => {
  await assert.rejects(new RankingHttpClient().getRankings('trending'), /not configured/);
  assert.throws(() => httpClient.configure({ baseUrl: 'file:///tmp' }), TypeError);
  httpClient.configure({ baseUrl: 'https://example.test' });
});
