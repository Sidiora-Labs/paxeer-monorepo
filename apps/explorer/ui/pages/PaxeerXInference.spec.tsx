import { ChakraProvider } from '@chakra-ui/react';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { apiResolver } from 'next/dist/server/api-utils/node/api-resolver';
import fetch from 'node-fetch';
import { readFileSync } from 'node:fs';
import { createServer } from 'node:http';
import { env } from 'node:process';
import React from 'react';
import { renderToStaticMarkup } from 'react-dom/server';

import type { InferenceDeal, InferenceRoom, InferenceReceipt } from 'lib/api/services/general/paxeerXInference';
import {
  GENERAL_API_PAXEER_X_INFERENCE_RESOURCES,
  inferenceQueryKey,
  parseInferenceResult,
} from 'lib/api/services/general/paxeerXInference';
import inference, { config } from 'pages/api/inference';
import theme from 'toolkit/theme/theme';
import { expect, test } from 'vitest';

import PaxeerXInference, { InferenceDealPanel, InferenceReceiptsTable } from './PaxeerXInference';

const id = (byte: string) => byte.repeat(32);

// Recorded decoder answers asserted in rpc/px_node_root_inference_test.go, TestPxNodeRootInferenceDecoders.
const room: InferenceRoom = {
  room_id: id('01'), host_key: id('02'), model_digest: id('03'), runtime_digest: id('04'),
  profile: {
    minimum_bond: '100', canary_ratio_bps: 100, lap_size_tokens: '32', challenge_window_blocks: '10',
    slash_fraction_bps: 500, host_share_bps: 1000, protocol_share_bps: 100, settlement_account: id('05'),
  },
  state: 0, opened_at_epoch: '6', open_asks: [ id('07') ], open_bids: [ id('08') ],
  ask_streams: [ { ask_id: id('07'), stream_id: id('00'), stream: null } ],
};

const deal: InferenceDeal = {
  room_id: id('01'), deal_id: id('09'), ask_id: id('07'), agreed_price_per_thousand_tokens: '2500',
  roster_digest: id('0a'), state: 1,
  layers: [ { bid_id: id('08'), first_layer: 0, end_layer: 16 }, { bid_id: id('0b'), first_layer: 16, end_layer: 32 } ],
  settled_tokens: '9007199254740993', settled_amount_owed: '1329227995784915872903807060280344583',
};

const receipt: InferenceReceipt = {
  deal_id: id('09'), first_lap_index: '4', last_lap_index: '7', cumulative_tokens: '9007199254740993',
  batch_digest: id('10'), countersign_quorum: 2,
};

function renderMarkup(children: React.ReactNode) {
  return renderToStaticMarkup(<ChakraProvider value={ theme }>{ children }</ChakraProvider>);
}

function inferencePageRendersProducerRoomList() {
  const client = new QueryClient();
  const rooms = parseInferenceResult('px_getInferenceRooms', [ room ]);
  client.setQueryData(inferenceQueryKey('px_getInferenceRooms'), rooms);
  try {
    const html = renderMarkup(<QueryClientProvider client={ client }><PaxeerXInference/></QueryClientProvider>);
    expect(html).toContain('>Inference</h1>');
    expect(html).toContain('1 inference rooms');
    expect(html).toContain(`data-room="${ room.room_id }"`);
    expect(html).toContain(`title="${ room.model_digest }"`);
    expect(html).toContain(`title="${ room.host_key }"`);
    expect(html).toContain('>Peers</th>');
    expect(html).toContain('>Open asks</th>');
    expect(html).toContain('>Settled tokens</th>');
    expect(html).toMatch(/<td[^>]*>1<\/td>/);
    expect(html).toContain('Peer count is not included in the room view');
    expect(html).toContain('>Look up deal</button>');
  } finally {
    client.clear();
  }
}

function inferencePageRendersEmptyRoomList() {
  const client = new QueryClient();
  client.setQueryData(inferenceQueryKey('px_getInferenceRooms'), parseInferenceResult('px_getInferenceRooms', []));
  try {
    const html = renderMarkup(<QueryClientProvider client={ client }><PaxeerXInference/></QueryClientProvider>);
    expect(html).toContain('There are no inference rooms.');
    expect(html).not.toContain('Loading inference rooms');
  } finally {
    client.clear();
  }
}

function inferenceDealPanelRendersLayerAndPayoutSplit() {
  const html = renderMarkup(<InferenceDealPanel deal={ parseInferenceResult('px_getInferenceDeal', deal) } room={ room }/>);
  expect(html).toContain(`data-deal="${ deal.deal_id }"`);
  expect(html).toContain('Payout split');
  expect(html).toContain('Host 10%');
  expect(html).toContain('Protocol 1%');
  expect(html).toContain('Peer pool 89%');
  expect(html).toContain(`data-bid="${ id('08') }"`);
  expect(html).toContain(`data-bid="${ id('0b') }"`);
  expect(html.match(/16 \/ 32/g)).toHaveLength(2);
  expect(html).toContain('End layer (exclusive)');
  expect(html).toContain('Settled tokens: 9,007,199,254,740,993');
  expect(html).toContain('Amount owed: 1,329,227,995,784,915,872,903,807,060,280,344,583');
  expect(renderMarkup(<InferenceDealPanel deal={ deal }/>)).toContain('Payout shares are unavailable');
}

function inferenceReceiptBatchesRenderExactCounters() {
  const receipts = parseInferenceResult('px_getInferenceReceipts', [ receipt ]);
  const html = renderMarkup(<InferenceReceiptsTable receipts={ receipts }/>);
  expect(html).toContain('1 receipt batches');
  expect(html).toContain(`data-receipt="${ receipt.batch_digest }"`);
  expect(html).toMatch(/<td[^>]*>4<\/td>/);
  expect(html).toMatch(/<td[^>]*>7<\/td>/);
  expect(html).toContain('9,007,199,254,740,993');
  expect(html).toContain('Countersign quorum');
  expect(html).toMatch(/<td[^>]*>2<\/td>/);
  expect(renderMarkup(<InferenceReceiptsTable receipts={ [] }/>)).toContain('There are no receipt batches for this deal.');
}

function inferenceResourcesUseTheFiveProducerNames() {
  const resources = Object.values(GENERAL_API_PAXEER_X_INFERENCE_RESOURCES);
  expect(resources.map((resource) => resource.rpcMethod)).toEqual([
    'px_getInferenceRooms', 'px_getInferenceRoom', 'px_getInferenceDeal', 'px_getInferencePeer', 'px_getInferenceReceipts',
  ]);
  const producer = readFileSync(new URL('../../../../rpc/px_inference.go', import.meta.url), 'utf8');
  for (const resource of resources) {
    expect(producer).toContain(`func (a *pxInferenceAPI) ${ resource.rpcMethod.slice(3).replace(/^g/, 'G') }(`);
    expect(resource.path).toBe('/api/inference');
  }
}

function inferenceDecodingRejectsMalformedDataAndStripsPrivateFields() {
  expect(parseInferenceResult('px_getInferenceRoom', { ...room, node_url: 'http://private.invalid/' })).toEqual(room);
  expect(() => parseInferenceResult('px_getInferenceRooms', { items: [ room ] })).toThrow('Invalid inference response');
  expect(() => parseInferenceResult('px_getInferenceRoom', { ...room, host_key: 'http://private.invalid/' })).toThrow('Invalid inference response');
  expect(() => parseInferenceResult('px_getInferenceDeal', { ...deal, settled_tokens: 9007199254740992 })).toThrow('Invalid inference response');
  expect(() => parseInferenceResult('px_getInferenceDeal', { ...deal, layers: [] })).toThrow('Invalid inference response');
  expect(() => parseInferenceResult('px_getInferenceReceipts', [ { ...receipt, last_lap_index: '3' } ])).toThrow('Invalid inference response');
}

async function inferenceRouteRefusesMethodsOutsideTheFive() {
  const previousRoot = env.EXPLORER_INFERENCE_NODE_URL;
  delete env.EXPLORER_INFERENCE_NODE_URL;
  const server = createServer((req, res) => {
    void apiResolver(req, res, {}, { 'default': inference, config }, {
      dev: false, previewModeId: 'inference-test', previewModeEncryptionKey: 'inference-test', previewModeSigningKey: 'inference-test',
    }, false);
  });
  await new Promise<void>((resolve) => server.listen(0, '127.0.0.1', resolve));
  const address = server.address();
  if (!address || typeof address === 'string') throw new Error('Expected a local TCP listener');
  const origin = `http://127.0.0.1:${ address.port }`;
  try {
    for (const method of [ 'eth_sendRawTransaction', 'px_getAccount', 'toString', '__proto__' ]) {
      const response = await fetch(`${ origin }/api/inference`, {
        method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify({ method, params: [] }),
      });
      expect(response.status).toBe(400);
      expect(response.headers.get('cache-control')).toBe('no-store');
      expect(await response.json()).toEqual({ error: 'Unsupported inference method' });
    }
    for (const { rpcMethod } of Object.values(GENERAL_API_PAXEER_X_INFERENCE_RESOURCES)) {
      const response = await fetch(`${ origin }/api/inference`, {
        method: 'POST', headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ method: rpcMethod, params: rpcMethod === 'px_getInferenceRooms' ? [] : [ id('01') ] }),
      });
      expect(response.status).toBe(503);
      expect(await response.json()).toEqual({ error: 'Inference is not configured' });
    }
    for (const body of [
      { method: 'px_getInferenceRooms', params: [ id('01') ] },
      { method: 'px_getInferenceRoom', params: [] },
      { method: 'px_getInferenceRoom', params: [ id('00') ] },
      { method: 'px_getInferenceDeal', params: [ 'http://private.invalid/' ] },
      { method: 'px_getInferencePeer', params: [ id('01'), id('02') ] },
      { method: 'px_getInferenceReceipts', params: [ null ] },
      null,
    ]) {
      const response = await fetch(`${ origin }/api/inference`, {
        method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify(body),
      });
      expect(response.status).toBe(400);
    }
    const get = await fetch(`${ origin }/api/inference`);
    expect(get.status).toBe(405);
    expect(get.headers.get('allow')).toBe('POST');
    expect(await get.json()).toEqual({ error: 'Method not allowed' });
  } finally {
    await new Promise<void>((resolve, reject) => server.close((error) => error ? reject(error) : resolve()));
    if (previousRoot === undefined) delete env.EXPLORER_INFERENCE_NODE_URL;
    else env.EXPLORER_INFERENCE_NODE_URL = previousRoot;
  }
}

test(inferencePageRendersProducerRoomList);
test(inferencePageRendersEmptyRoomList);
test(inferenceDealPanelRendersLayerAndPayoutSplit);
test(inferenceReceiptBatchesRenderExactCounters);
test(inferenceResourcesUseTheFiveProducerNames);
test(inferenceDecodingRejectsMalformedDataAndStripsPrivateFields);
test(inferenceRouteRefusesMethodsOutsideTheFive);
