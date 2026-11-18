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
    operations.call('IF_AUTOMATION_STATUS', {
      path: { campaignId: randomUUID() },
    }),
    error => {
      assert.ok(error instanceof CapabilityUnavailableError);
      assert.equal(error.code, 'capability_unavailable');
      assert.equal(error.status, 501);
      assert.equal(error.message, 'Operation IF_AUTOMATION_STATUS has no served route');
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

async function retiredOperationsRefusedBeforeNetwork() {
  const calls = [];
  const client = new SdkClient({
    ...transportConfig('http://127.0.0.1:1'),
    session: { mode: 'bearer', token: () => 'test-session' },
  }, (input, init) => {
    calls.push({ input, init });
    return globalThis.fetch(input, init);
  });
  const operations = new Operations(client);
  const assetId = randomUUID();
  const roomId = randomUUID();
  const sha256 = '0x' + 'a'.repeat(64);
  const inputs = {
    IF_ASSET_GET: { path: { assetId } },
    IF_UPLOAD_CREATE: { body: { mediaType: 'image/png', bytes: 1, sha256 } },
    IF_UPLOAD_COMPLETE: { path: { assetId }, body: { sha256 } },
    IF_MESSAGES_LIST: { path: { roomId }, query: { limit: 50 } },
    IF_MESSAGE_SEND: { path: { roomId }, body: { body: 'Hello', clientMessageId: randomUUID() } },
  };
  for (const [name, input] of Object.entries(inputs)) {
    await assert.rejects(
      operations.call(name, input, { headers: { 'Idempotency-Key': randomUUID() } }),
      error => {
        assert.ok(error instanceof CapabilityUnavailableError);
        assert.equal(error.code, 'capability_unavailable');
        assert.equal(error.status, 501);
        assert.equal(error.message, `Operation ${name} has no served route`);
        return true;
      },
    );
    assert.deepEqual(calls, [], name);
  }
}

test(unroutedOperationRefusedBeforeNetwork);
test(retiredOperationsRefusedBeforeNetwork);
test(servedOperationStillReachesNetwork);
