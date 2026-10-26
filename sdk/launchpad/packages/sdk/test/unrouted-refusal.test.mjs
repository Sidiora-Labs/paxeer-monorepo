import { test } from 'node:test';
import assert from 'node:assert/strict';
import { createServer } from 'node:http';
import { once } from 'node:events';
import { randomUUID } from 'node:crypto';
import { SdkClient, Operations, CapabilityUnavailableError } from '../dist/index.js';

function transportConfig(origin) {
  const streamOrigin = origin.replace('http:', 'ws:');
  return {
    apiOrigin: origin,
    identityOrigin: origin,
    mediaOrigin: origin,
    streamOrigin,
    approvedStreamOrigins: [streamOrigin],
    production: false,
  };
}

async function unroutedOperationRefusedBeforeNetwork() {
  const calls = [];
  const client = new SdkClient({
    ...transportConfig('http://127.0.0.1:1'),
    session: { mode: 'bearer', token: () => 'test-session' },
  }, (input, init) => {
    calls.push({ input, init });
    return globalThis.fetch(input, init);
  });
  const operations = new Operations(client);

  await assert.rejects(
    operations.call('IF_VOICE_END', {
      path: { roomId: randomUUID() },
      body: { expectedRevision: '1' },
    }),
    error => {
      assert.ok(error instanceof CapabilityUnavailableError);
      assert.equal(error.code, 'capability_unavailable');
      assert.equal(error.status, 501);
      assert.equal(error.message, 'Operation IF_VOICE_END has no served route');
      return true;
    },
  );
  assert.deepEqual(calls, []);
}

async function servedOperationStillReachesNetwork() {
  const seen = [];
  const response = {
    requestId: randomUUID(),
    data: { items: [] },
    watermark: null,
    revision: null,
    quality: 'complete',
    nextCursor: null,
    generatedAt: new Date().toISOString(),
  };
  const server = createServer((req, res) => {
    seen.push({ method: req.method, path: req.url });
    res.writeHead(200, { 'Content-Type': 'application/json' });
    res.end(JSON.stringify(response));
  });
  server.listen(0, '127.0.0.1');
  await once(server, 'listening');
  try {
    const origin = `http://127.0.0.1:${server.address().port}`;
    const operations = new Operations(new SdkClient(transportConfig(origin)));
    const result = await operations.call('IF_POOL_LIST', {
      query: { sort: 'trending', cursor: 'page/2+next', limit: 7 },
    });

    assert.deepEqual(seen, [{ method: 'GET', path: '/v2/pools?sort=trending&cursor=page%2F2%2Bnext&limit=7' }]);
    assert.deepEqual(result, response);
  } finally {
    const closed = new Promise((resolve, reject) => server.close(error => error ? reject(error) : resolve()));
    server.closeAllConnections();
    await closed;
  }
}

test(unroutedOperationRefusedBeforeNetwork);
test(servedOperationStillReachesNetwork);
