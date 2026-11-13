import { test } from 'node:test';
import assert from 'node:assert/strict';
import { createServer } from 'node:http';
import { once } from 'node:events';
import { Chains, SdkClient, CHAIN_ROUTE_STATUS, routeStatus } from '../dist/index.js';
import { validateChainsResponse, validateChainDetailResponse, validateChainRevenueResponse } from '../dist/validate.js';

const holder = '0xaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa';
const chain = {
  chain: '7', chainId: '1007',
  pool: '0x1111111111111111111111111111111111111111',
  token: '0x3333333333333333333333333333333333333333',
  nftId: '77', creator: '0x2222222222222222222222222222222222222222',
  blockNumber: '101', status: 'active',
};
const epoch = { epoch: '2', epochAmount: '2000', snapshotBlock: '96', eligibleSupply: '10000' };
const claim = { epoch: '2', holder, amount: '200', blockNumber: '110' };

function envelope(data, nextCursor = null) {
  return { requestId: '00000000-0000-4000-8000-000000000001', data, revision: null, watermark: null, quality: 'complete', nextCursor, generatedAt: '2026-11-11T00:00:00.000Z' };
}

async function withChainServer(routes, run) {
  const requests = [];
  const server = createServer((request, response) => {
    requests.push({ method: request.method, path: request.url });
    const body = routes[request.url];
    response.writeHead(body === undefined ? 404 : 200, { 'Content-Type': 'application/json' });
    response.end(JSON.stringify(body ?? { code: 'NOT_FOUND' }));
  });
  server.listen(0, '127.0.0.1');
  await once(server, 'listening');
  const origin = `http://127.0.0.1:${server.address().port}`;
  const stream = origin.replace('http:', 'ws:');
  const client = new SdkClient({ apiOrigin: origin, identityOrigin: origin, mediaOrigin: origin, streamOrigin: stream, approvedStreamOrigins: [stream], production: false });
  try {
    await run(new Chains(client), requests);
  } finally {
    server.close();
    await once(server, 'close');
  }
}

async function chainsListRequestsAndValidatesPage() {
  const sample = envelope({ items: [chain] });
  await withChainServer({ '/v2/chains?limit=2': sample }, async (chains, requests) => {
    const response = await chains.list({ limit: 2 });
    assert.deepEqual(response, sample);
    assert.equal(response.data.items[0].chainId, '1007');
    assert.deepEqual(requests, [{ method: 'GET', path: '/v2/chains?limit=2' }]);
  });
  assert.deepEqual(validateChainsResponse(sample), sample);
}

async function chainDetailRequestsAndValidatesRow() {
  const sample = envelope({ chain });
  await withChainServer({ '/v2/chains/7': sample }, async (chains, requests) => {
    const response = await chains.get('7');
    assert.deepEqual(response, sample);
    assert.equal(response.data.chain.nftId, '77');
    assert.deepEqual(requests, [{ method: 'GET', path: '/v2/chains/7' }]);
  });
  assert.deepEqual(validateChainDetailResponse(sample), sample);
}

async function chainRevenueRequestsAndValidatesEpochsAndClaims() {
  const plain = envelope({ chain: '7', epochs: [epoch] });
  const withHolder = envelope({ chain: '7', epochs: [epoch], holder, claims: [claim] });
  await withChainServer({
    '/v2/chains/7/revenue': plain,
    [`/v2/chains/7/revenue?limit=1&holder=${holder}`]: withHolder,
  }, async (chains, requests) => {
    assert.deepEqual(await chains.revenue('7'), plain);
    const response = await chains.revenue('7', { limit: 1, holder });
    assert.deepEqual(response, withHolder);
    assert.equal(response.data.claims[0].amount, '200');
    assert.deepEqual(requests, [
      { method: 'GET', path: '/v2/chains/7/revenue' },
      { method: 'GET', path: `/v2/chains/7/revenue?limit=1&holder=${holder}` },
    ]);
  });
  assert.deepEqual(validateChainRevenueResponse(withHolder), withHolder);
}

async function chainResponsesRequireChainId() {
  const missing = { ...chain };
  delete missing.chainId;
  const page = envelope({ items: [missing] });
  assert.throws(() => validateChainsResponse(page), TypeError);
  assert.throws(() => validateChainDetailResponse(envelope({ chain: missing })), TypeError);
  await withChainServer({ '/v2/chains': page }, async chains => {
    await assert.rejects(chains.list(), TypeError);
  });
}

function chainRoutesAreServed() {
  assert.deepEqual(CHAIN_ROUTE_STATUS, { IF_CHAINS: 'served', IF_CHAIN_DETAIL: 'served', IF_CHAIN_REVENUE: 'served' });
  for (const name of Object.keys(CHAIN_ROUTE_STATUS)) assert.equal(routeStatus(name), 'served');
}

test(chainsListRequestsAndValidatesPage);
test(chainDetailRequestsAndValidatesRow);
test(chainRevenueRequestsAndValidatesEpochsAndClaims);
test(chainResponsesRequireChainId);
test(chainRoutesAreServed);
