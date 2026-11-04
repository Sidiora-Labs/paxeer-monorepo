import { createHook } from 'node:async_hooks';
import { once } from 'node:events';
import type { ServerResponse } from 'node:http';
import { createServer } from 'node:http';
import type { AddressInfo } from 'node:net';
import { afterAll, beforeAll, expect, test } from 'vitest';

import { EXPLORER_REQUEST_TIMEOUT_MS, getExplorerUrls, run } from './index';

// The shared explorer setup intercepts fetch. These proofs use real HTTP transport.
beforeAll(() => {
  fetchMock.disableMocks();
});
afterAll(() => {
  fetchMock.enableMocks();
});

async function withAggregator(
  respond: (response: ServerResponse) => void,
  consume: (host: string, requests: Array<string>) => Promise<void>,
) {
  const requests: Array<string> = [];
  const server = createServer((request, response) => {
    requests.push(request.url || '');
    respond(response);
  });
  server.listen(0, '127.0.0.1');
  await once(server, 'listening');
  const host = `http://127.0.0.1:${ (server.address() as AddressInfo).port }`;

  try {
    await consume(host, requests);
  } finally {
    const closed = new Promise<void>((resolve, reject) => {
      server.close((error) => error ? reject(error) : resolve());
    });
    server.closeAllConnections();
    await closed;
  }
}

async function withoutWorkers(action: () => Promise<void>) {
  const workers: Array<number> = [];
  const hook = createHook({
    init(id, type) {
      if (type === 'WORKER') {
        workers.push(id);
      }
    },
  }).enable();

  try {
    await action();
    expect(workers).toEqual([]);
  } finally {
    hook.disable();
  }
}

async function repairMultichainFetchDeadlineBeforeHeaders() {
  let disconnected: Promise<unknown> | undefined;
  await withAggregator((response) => {
    disconnected = once(response, 'close');
    // Keep the connection open without sending response headers.
  }, async(host, requests) => {
    const started = performance.now();
    await withoutWorkers(async() => {
      await expect(run({ aggregatorHost: host, cluster: 'recorded-cluster', requestTimeoutMs: 500 }))
        .rejects.toThrow('Request to multichain aggregator API timed out after 500ms');
    });
    expect(performance.now() - started).toBeLessThan(2_000);
    expect(requests).toEqual([ '/api/v1/clusters/recorded-cluster/chains' ]);
    expect(disconnected).toBeDefined();
    await disconnected;
  });
}

async function repairMultichainFetchDeadlineIncludesBody() {
  let disconnected: Promise<unknown> | undefined;
  await withAggregator((response) => {
    disconnected = once(response, 'close');
    response.writeHead(200, { 'Content-Type': 'application/json' });
    response.write('{"items":[');
  }, async(host, requests) => {
    const started = performance.now();
    await withoutWorkers(async() => {
      await expect(run({ aggregatorHost: host, cluster: 'recorded-cluster', requestTimeoutMs: 500 }))
        .rejects.toThrow('Request to multichain aggregator API timed out after 500ms');
    });
    expect(performance.now() - started).toBeLessThan(2_000);
    expect(requests).toEqual([ '/api/v1/clusters/recorded-cluster/chains' ]);
    expect(disconnected).toBeDefined();
    await disconnected;
  });
}

async function repairMultichainFetchRejectsHttpErrors() {
  let disconnected: Promise<unknown> | undefined;
  await withAggregator((response) => {
    disconnected = once(response, 'close');
    response.writeHead(503, { 'Content-Type': 'application/json' });
    response.write(JSON.stringify({ items: [ { explorer_url: 'https://explorer.example' } ] }));
  }, async(host, requests) => {
    await withoutWorkers(async() => {
      await expect(run({ aggregatorHost: host, cluster: 'recorded-cluster' }))
        .rejects.toThrow('Failed to fetch chains from multichain aggregator API: HTTP 503');
    });
    expect(requests).toEqual([ '/api/v1/clusters/recorded-cluster/chains' ]);
    expect(disconnected).toBeDefined();
    await disconnected;
  });
}

async function repairMultichainFetchValidatesEveryExplorerBeforeWorkers() {
  const validItem = { explorer_url: 'https://explorer.example' };
  const fixtures = [
    { body: 'null', error: 'expected an items array' },
    { body: '{}', error: 'expected an items array' },
    { body: '{"items":{}}', error: 'expected an items array' },
    ...[ null, {}, { explorer_url: 125 } ].map((item) => ({
      body: JSON.stringify({ items: [ validItem, item ] }),
      error: 'expected an explorer_url string for every chain',
    })),
    ...[ '', '/relative', 'not a URL' ].map((explorer_url) => ({
      body: JSON.stringify({ items: [ validItem, { explorer_url } ] }),
      error: 'Invalid URL',
    })),
    ...[ 'file:///etc/passwd', 'javascript:alert(1)', 'ftp://explorer.example' ].map((explorer_url) => ({
      body: JSON.stringify({ items: [ validItem, { explorer_url } ] }),
      error: 'explorer_url must use HTTP or HTTPS',
    })),
  ];

  for (const fixture of fixtures) {
    await withAggregator((response) => {
      response.writeHead(200, { 'Content-Type': 'application/json' });
      response.end(fixture.body);
    }, async(host, requests) => {
      await withoutWorkers(async() => {
        await expect(run({ aggregatorHost: host, cluster: 'recorded-cluster' })).rejects.toThrow(fixture.error);
      });
      expect(requests).toEqual([ '/api/v1/clusters/recorded-cluster/chains' ]);
    });
  }
}

async function repairMultichainFetchPropagatesInvalidJson() {
  await withAggregator((response) => {
    response.writeHead(200, { 'Content-Type': 'application/json' });
    response.end('{');
  }, async(host) => {
    await withoutWorkers(async() => {
      await expect(run({ aggregatorHost: host, cluster: 'recorded-cluster' })).rejects.toBeInstanceOf(SyntaxError);
    });
  });
}

async function repairMultichainFetchRejectsEmptyCluster() {
  await withAggregator((response) => {
    response.writeHead(200, { 'Content-Type': 'application/json' });
    response.end(JSON.stringify({ items: [] }));
  }, async(host) => {
    await withoutWorkers(async() => {
      await expect(run({ aggregatorHost: host, cluster: 'recorded-cluster' })).rejects.toThrow('No chains found in the cluster.');
    });
  });
}

async function repairMultichainFetchPreservesValidExplorerUrls() {
  const explorerUrls = [ 'https://explorer.example', 'http://127.0.0.1:1234/explorer' ];
  await withAggregator((response) => {
    response.writeHead(200, { 'Content-Type': 'application/json' });
    response.end(JSON.stringify({ items: explorerUrls.map((explorer_url) => ({ explorer_url })) }));
  }, async(host, requests) => {
    expect(EXPLORER_REQUEST_TIMEOUT_MS).toBe(30_000);
    await expect(getExplorerUrls(new URL('/api/v1/clusters/recorded-cluster/chains', host))).resolves.toEqual(explorerUrls);
    expect(requests).toEqual([ '/api/v1/clusters/recorded-cluster/chains' ]);
  });
}

test(repairMultichainFetchDeadlineBeforeHeaders.name, repairMultichainFetchDeadlineBeforeHeaders);
test(repairMultichainFetchDeadlineIncludesBody.name, repairMultichainFetchDeadlineIncludesBody);
test(repairMultichainFetchRejectsHttpErrors.name, repairMultichainFetchRejectsHttpErrors);
test(repairMultichainFetchValidatesEveryExplorerBeforeWorkers.name, repairMultichainFetchValidatesEveryExplorerBeforeWorkers);
test(repairMultichainFetchPropagatesInvalidJson.name, repairMultichainFetchPropagatesInvalidJson);
test(repairMultichainFetchRejectsEmptyCluster.name, repairMultichainFetchRejectsEmptyCluster);
test(repairMultichainFetchPreservesValidExplorerUrls.name, repairMultichainFetchPreservesValidExplorerUrls);
