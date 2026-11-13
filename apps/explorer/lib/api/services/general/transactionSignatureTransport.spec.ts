import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { createHash, createPublicKey, verify } from 'node:crypto';
import { once } from 'node:events';
import { readFileSync } from 'node:fs';
import { createServer } from 'node:http';
import path from 'node:path';
import React from 'react';
import { renderToString } from 'react-dom/server';
import { array, literal, number, object, optional, parse, string, union } from 'valibot';

import type { PaxeerXTxSignature } from 'types/api/paxeerXSignature';
import type { ClusterChainConfig } from 'types/multichain';

import config from 'configs/app';
import buildUrl from 'lib/api/buildUrl';
import useApiFetch from 'lib/api/useApiFetch';
import { afterAll, beforeAll, expect, expectTypeOf, test } from 'vitest';

import type { GeneralApiPaxeerXListsResourcePayload } from './paxeerXLists';
import { GENERAL_API_PAXEER_X_LISTS_RESOURCES } from './paxeerXLists';

const root = path.resolve(__dirname, '../../../../../..');
const gatewaySource = readFileSync(path.join(root, 'services/platform/hosted/gateway/src/explorer_proxy.rs'), 'utf8');
const backend = path.join(root, 'services/explorer/backend/apps');
const vectors = parse(object({
  cases: array(object({
    id: string(),
    receiptJson: object({ txHash: string(), receipt: string() }),
    pinnedKey: object({ keyId: string(), publicKey: string() }),
    eth_receipt: object({ blockNumber: string() }),
  })),
}), JSON.parse(readFileSync(path.join(backend, 'explorer/test/support/fixture/paxeer_x/committed_vectors.json'), 'utf8')));
const committed = vectors.cases.find(vector => vector.id === 'committed-ok');
if (!committed) {
  throw new Error('Missing backend committed-ok conformance fixture');
}
const hash = committed.receiptJson.txHash;
const receipt = Buffer.from(committed.receiptJson.receipt.slice(2), 'hex');
const signature = receipt.subarray(-64);
const committedResponse: PaxeerXTxSignature = {
  rung: 'committed',
  receipt: committed.receiptJson.receipt,
  receipt_key: { key_id: committed.pinnedKey.keyId, public_key: committed.pinnedKey.publicKey },
  signature: `0x${ signature.toString('hex') }`,
  committed_height: Number.parseInt(committed.eth_receipt.blockNumber, 16),
};
const { committed_height: committedHeight, ...receiptedResponse } = { ...committedResponse, rung: 'receipted' as const };

beforeAll(() => {
  fetchMock.disableMocks();
});
afterAll(() => {
  fetchMock.enableMocks();
});

function signatureGatewayRouteMatchesTheProducerAndExplorerResource() {
  const resource = GENERAL_API_PAXEER_X_LISTS_RESOURCES.paxeer_x_tx_signature;
  expect(resource.path).toBe('/api/v2/transactions/:hash/signature');
  expect(resource.pathParams).toEqual([ 'hash' ]);
  expectTypeOf<GeneralApiPaxeerXListsResourcePayload<'general:paxeer_x_tx_signature'>>().toEqualTypeOf<PaxeerXTxSignature>();

  const routes = gatewaySource.slice(gatewaySource.indexOf('static ROUTES:'));
  const declarations = Array.from(routes.matchAll(/Route\s*\{\s*method:\s*"([A-Z]+)",\s*path:\s*"([^"]+)",\s*session_csrf:\s*(true|false),\s*\}/g));
  const signatureRoutes = declarations.filter(route => route[2].endsWith('/signature'));
  expect(signatureRoutes.map(route => ({ method: route[1], path: route[2], sessionCsrf: route[3] }))).toEqual([ {
    method: 'GET',
    path: resource.path.replace(':hash', '{transaction_hash_param}'),
    sessionCsrf: 'false',
  } ]);
  const router = readFileSync(path.join(backend, 'block_scout_web/lib/block_scout_web/routers/api_router.ex'), 'utf8');
  expect(router).toContain('get("/:transaction_hash_param/signature", V2.PaxeerX.TransactionSignatureController, :signature)');
}

test('signatureReplayUsesTheBackendsCryptographicallyValidFixture', function signatureReplayUsesTheBackendsCryptographicallyValidFixture() {
  const expectedHeight = Number.parseInt(committed.eth_receipt.blockNumber, 16);
  expect(committedHeight).toBe(expectedHeight);
  // The canonical signed tail encodes the height as a 32-byte big-endian batch ID.
  expect(receipt.subarray(-424, -392).toString('hex')).toBe(expectedHeight.toString(16).padStart(64, '0'));
  expect(receipt.subarray(-69, -64).toString('hex')).toBe('0100000040');
  const digest = createHash('sha256')
    .update('LXP/v1/receipt\0')
    .update(Uint8Array.from(receipt.subarray(0, -69)))
    .update(new Uint8Array([ 0 ]))
    .digest();
  const key = createPublicKey({
    key: Buffer.from(`302a300506032b6570032100${ committedResponse.receipt_key.public_key.slice(2) }`, 'hex'),
    format: 'der',
    type: 'spki',
  });
  expect(verify(null, Uint8Array.from(digest), key, Uint8Array.from(signature))).toBe(true);
  const changed = Uint8Array.from(signature);
  changed[0] ^= 1;
  expect(verify(null, Uint8Array.from(digest), key, changed)).toBe(false);
});

async function openBackendResponseReplay(responseBody: PaxeerXTxSignature = committedResponse) {
  const requests: Array<{ method: string | undefined; target: string | undefined }> = [];
  const prefix = '/explorer/backend/api/v2/transactions';
  const responses = new Map<string, { status: number; body: PaxeerXTxSignature | { message: string } }>([
    [ `${ prefix }/${ hash }/signature`, { status: 200, body: responseBody } ],
    [ `${ prefix }/0x${ '00'.repeat(32) }/signature`, { status: 404, body: { message: 'Not found' } } ],
    [ `${ prefix }/0x01/signature`, { status: 422, body: { message: 'Invalid parameter(s)' } } ],
  ]);
  const server = createServer((request, response) => {
    requests.push({ method: request.method, target: request.url });
    const answer = responses.get(request.url ?? '');
    if (request.method !== 'GET' || !answer) {
      response.writeHead(500, { 'content-type': 'application/json' });
      response.end(JSON.stringify({ message: 'Unexpected replay request' }));
      return;
    }
    response.writeHead(answer.status, { 'content-type': 'application/json' });
    response.end(JSON.stringify(answer.body));
  });
  server.listen(0, '127.0.0.1');
  await once(server, 'listening');
  const address = server.address();
  if (!address || typeof address === 'string') {
    throw new Error('Expected a loopback TCP address');
  }
  const chain: ClusterChainConfig = {
    id: '125', name: 'Signature fixture', slug: 'signature-fixture',
    app_config: {
      ...config,
      apis: {
        ...config.apis,
        general: {
          endpoint: `http://127.0.0.1:${ address.port }`,
          host: '127.0.0.1',
          protocol: 'http',
          port: String(address.port),
          socketEndpoint: `ws://127.0.0.1:${ address.port }`,
          basePath: '/explorer/backend',
        },
      },
    },
  };
  return {
    chain,
    requests,
    close: () => new Promise<void>((resolve, reject) => {
      server.close(error => error ? reject(error) : resolve());
      server.closeAllConnections();
    }),
  };
}

function openSignatureConsumer() {
  const client = new QueryClient();
  const state: { read?: ReturnType<typeof useApiFetch> } = {};
  function SignatureConsumer() {
    state.read = useApiFetch();
    return null;
  }
  renderToString(React.createElement(QueryClientProvider, { client, children: React.createElement(SignatureConsumer) }));
  if (!state.read) {
    throw new Error('Signature consumer did not mount');
  }
  return { read: state.read, close: () => client.clear() };
}

test('signatureResourcePreservesTheReceiptKeySignatureAndCommittedHeight', async function signatureResourcePreservesTheReceiptKeySignatureAndCommittedHeight() {
  signatureGatewayRouteMatchesTheProducerAndExplorerResource();
  for (const response of [ committedResponse, receiptedResponse ]) {
    const replay = await openBackendResponseReplay(response);
    const consumer = openSignatureConsumer();
    try {
      const url = new URL(buildUrl('general:paxeer_x_tx_signature', { hash }, undefined, true, replay.chain));
      expect(url.pathname).toBe(`/explorer/backend/api/v2/transactions/${ hash }/signature`);
      const received = parse(object({
        rung: union([ literal('committed'), literal('receipted') ]),
        receipt: string(),
        receipt_key: object({ key_id: string(), public_key: string() }),
        signature: string(),
        committed_height: optional(number()),
      }), await consumer.read('general:paxeer_x_tx_signature', { chain: replay.chain, pathParams: { hash } }));
      expect(received).toEqual(response);
      const receivedReceipt = Buffer.from(received.receipt.slice(2), 'hex');
      const digest = createHash('sha256')
        .update('LXP/v1/receipt\0')
        .update(Uint8Array.from(receivedReceipt.subarray(0, -69)))
        .update(new Uint8Array([ 0 ]))
        .digest();
      const key = createPublicKey({
        key: Buffer.from(`302a300506032b6570032100${ received.receipt_key.public_key.slice(2) }`, 'hex'),
        format: 'der',
        type: 'spki',
      });
      expect(verify(null, Uint8Array.from(digest), key, Uint8Array.from(Buffer.from(received.signature.slice(2), 'hex')))).toBe(true);
      expect(receivedReceipt.subarray(-64).toString('hex')).toBe(received.signature.slice(2));
      expect(replay.requests).toEqual([ { method: 'GET', target: url.pathname } ]);
    } finally {
      consumer.close();
      await replay.close();
    }
  }
});

test('signatureResourcePreservesMissingAndInvalidHashRefusals', async function signatureResourcePreservesMissingAndInvalidHashRefusals() {
  signatureGatewayRouteMatchesTheProducerAndExplorerResource();
  const replay = await openBackendResponseReplay();
  const consumer = openSignatureConsumer();
  try {
    await expect(consumer.read('general:paxeer_x_tx_signature', {
      chain: replay.chain, pathParams: { hash: `0x${ '00'.repeat(32) }` },
    })).rejects.toMatchObject({ status: 404, payload: { message: 'Not found' } });
    await expect(consumer.read('general:paxeer_x_tx_signature', {
      chain: replay.chain, pathParams: { hash: '0x01' },
    })).rejects.toMatchObject({ status: 422, payload: { message: 'Invalid parameter(s)' } });
    expect(() => buildUrl('general:paxeer_x_tx_signature', { hash: undefined }, undefined, true, replay.chain))
      .toThrow('Missing parameters: hash');
    expect(replay.requests).toHaveLength(2);
  } finally {
    consumer.close();
    await replay.close();
  }
});

test('signatureGatewayRouteMatchesTheProducerAndExplorerResource', function signatureGatewayRouteMatchesTheProducerAndExplorerResource() {
  const resource = GENERAL_API_PAXEER_X_LISTS_RESOURCES.paxeer_x_tx_signature;
  expect(resource.path).toBe('/api/v2/transactions/:hash/signature');
  expect(resource.pathParams).toEqual([ 'hash' ]);
  expectTypeOf<GeneralApiPaxeerXListsResourcePayload<'general:paxeer_x_tx_signature'>>().toEqualTypeOf<PaxeerXTxSignature>();

  const routes = gatewaySource.slice(gatewaySource.indexOf('static ROUTES:'));
  const declarations = Array.from(routes.matchAll(/Route\s*\{\s*method:\s*"([A-Z]+)",\s*path:\s*"([^"]+)",\s*session_csrf:\s*(true|false),\s*\}/g));
  const signatureRoutes = declarations.filter(route => route[2].endsWith('/signature'));
  expect(signatureRoutes.map(route => ({ method: route[1], path: route[2], sessionCsrf: route[3] }))).toEqual([ {
    method: 'GET',
    path: resource.path.replace(':hash', '{transaction_hash_param}'),
    sessionCsrf: 'false',
  } ]);
  const router = readFileSync(path.join(backend, 'block_scout_web/lib/block_scout_web/routers/api_router.ex'), 'utf8');
  expect(router).toContain('get("/:transaction_hash_param/signature", V2.PaxeerX.TransactionSignatureController, :signature)');
});
