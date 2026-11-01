import { test } from 'node:test';
import assert from 'node:assert/strict';
import { createServer } from 'node:http';
import { once } from 'node:events';
import { randomUUID } from 'node:crypto';
import { SdkClient, Operations, routeStatus } from '../dist/index.js';
import { ROUTE_CORRECTIONS } from '../dist/route-corrections.js';

async function rewardClaimUsesServedRoute() {
  const seen = [];
  const body = {
    recipient: '0x1111111111111111111111111111111111111111',
    rewardIds: ['reward-1', 'reward-2'],
  };
  const idempotencyKey = randomUUID();
  const response = {
    requestId: randomUUID(),
    data: {
      estimatedGasPaxRaw: '21000',
      transactionRequest: {
        chainId: '125',
        from: body.recipient,
        to: '0x2222222222222222222222222222222222222222',
        data: '0x1234',
        valueRaw: '0',
        gasLimitRaw: null,
        maxFeePerGasRaw: null,
        maxPriorityFeePerGasRaw: null,
      },
    },
    watermark: null,
    revision: null,
    quality: 'complete',
    nextCursor: null,
    generatedAt: new Date().toISOString(),
  };
  const server = createServer((req, res) => {
    let receivedBody = '';
    req.setEncoding('utf8');
    req.on('data', chunk => { receivedBody += chunk; });
    req.on('end', () => {
      seen.push({
        method: req.method,
        path: req.url,
        authorization: req.headers.authorization,
        idempotencyKey: req.headers['idempotency-key'],
        contentType: req.headers['content-type'],
        body: receivedBody,
      });
      if (req.method !== 'POST' || req.url !== '/v2/me/rewards/claim') {
        res.writeHead(404);
        res.end();
        return;
      }
      res.writeHead(200, { 'Content-Type': 'application/json' });
      res.end(JSON.stringify(response));
    });
  });
  server.listen(0, '127.0.0.1');
  await once(server, 'listening');
  try {
    const origin = `http://127.0.0.1:${server.address().port}`;
    const streamOrigin = origin.replace(/^http/, 'ws');
    const operations = new Operations(new SdkClient({
      apiOrigin: origin,
      identityOrigin: origin,
      mediaOrigin: origin,
      streamOrigin,
      approvedStreamOrigins: [streamOrigin],
      production: false,
      session: { mode: 'bearer', token: () => 'test-session' },
    }));
    const result = await operations.call('IF_REWARD_CLAIM', { body }, {
      headers: { 'Idempotency-Key': idempotencyKey },
    });

    assert.equal(ROUTE_CORRECTIONS.IF_REWARD_CLAIM, '/v2/me/rewards/claim');
    assert.equal(routeStatus('IF_REWARD_CLAIM'), 'served');
    assert.deepEqual(seen, [{
      method: 'POST',
      path: '/v2/me/rewards/claim',
      authorization: 'Bearer test-session',
      idempotencyKey,
      contentType: 'application/json',
      body: JSON.stringify(body),
    }]);
    assert.deepEqual(result, response);
  } finally {
    const closed = new Promise((resolve, reject) => server.close(error => error ? reject(error) : resolve()));
    server.closeAllConnections();
    await closed;
  }
}

test(rewardClaimUsesServedRoute);
