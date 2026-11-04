// @vitest-environment jsdom

import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { act, cleanup, render, waitFor } from '@testing-library/react';
import { createServer } from 'node:http';
import type { ServerResponse } from 'node:http';
import type { AddressInfo } from 'node:net';
import { setTimeout as delay } from 'node:timers/promises';
import React from 'react';
import type { WebSocket } from 'ws';
import { WebSocketServer } from 'ws';

import type { TokenInstance } from 'types/api/token';

import type { getResourceKey as resourceKey } from 'lib/api/useApiQuery';
import { SocketProvider } from 'lib/socket/context';
import { base } from 'mocks/tokens/tokenInstance';
import type { toaster as toastStore } from 'toolkit/chakra/toaster';
import { afterAll, afterEach, beforeAll, beforeEach, expect, test } from 'vitest';

import { MetadataUpdateProvider, useMetadataUpdateContext } from './contexts/metadataUpdate';
import transcript from './fixtures/metadata-refresh.json';
import type useTokenInstanceMetadataRefresh from './useTokenInstanceMetadataRefresh';

const HASH = base.token.address_hash;
const ID = transcript.cooldown.token_id;
const TOPIC = `token_instances:${ HASH.toLowerCase() }`;
const INSTANCE_PATH = `/api/v2/tokens/${ HASH }/instances/${ ID }`;
const socketOptions = { reconnectAfterMs: () => 20, rejoinAfterMs: () => 20 };
const originalEnvs = window.__envs;
const initialInstance: TokenInstance = { ...base, id: ID };
let canonicalInstance: TokenInstance;
let media = transcript.media[0];
let patches: number;
let gets: number;
let joins: number;
let acknowledged: boolean;
let holdJoin: boolean;
let holdResponse: boolean;
let publishOnPatch: boolean;
let patchResponse: ServerResponse | undefined;
let acknowledgeJoin: (() => void) | undefined;
let subscription: { client: WebSocket; joinRef: string; topic: string } | undefined;
let queryClient: QueryClient;
let context: ReturnType<typeof useMetadataUpdateContext>;
let changeVisibility: React.Dispatch<React.SetStateAction<boolean>>;
let useRefresh: typeof useTokenInstanceMetadataRefresh;
let toaster: typeof toastStore;
let getResourceKey: typeof resourceKey;
const order: Array<string> = [];

function publish(event: string, payload: unknown) {
  if (!subscription) {
    throw new Error('The consumer has not joined the producer topic');
  }
  const { client, joinRef, topic } = subscription;
  client.send(JSON.stringify([ joinRef, null, topic, event, payload ]));
}

const httpServer = createServer((request, response) => {
  response.setHeader('Content-Type', 'application/json');
  if (request.method === 'PATCH' && request.url === `${ INSTANCE_PATH }/refetch-metadata`) {
    patches++;
    order.push(acknowledged ? 'patch-after-ack' : 'patch-before-ack');
    patchResponse = response;
    if (publishOnPatch) {
      canonicalInstance = { ...initialInstance, image_url: media.image_url, animation_url: media.animation_url, metadata: media.metadata };
      publish('fetched_token_instance_metadata', media.fetched_event);
    }
    if (!holdResponse) {
      response.end('{}');
    }
  } else if (request.method === 'GET' && request.url === INSTANCE_PATH) {
    gets++;
    order.push('get');
    response.end(JSON.stringify(canonicalInstance));
  } else {
    response.writeHead(404);
    response.end(JSON.stringify({ message: 'Unexpected resource' }));
  }
});
const wsServer = new WebSocketServer({ noServer: true });
httpServer.on('upgrade', (request, socket, head) => {
  wsServer.handleUpgrade(request, socket, head, (client) => wsServer.emit('connection', client, request));
});
wsServer.on('connection', (client) => {
  client.on('message', (raw) => {
    const [ joinRef, ref, topic, event ] = JSON.parse(raw.toString()) as [ string, string, string, string ];
    if (event === 'heartbeat' || event === 'phx_leave') {
      client.send(JSON.stringify([ joinRef, ref, topic, 'phx_reply', { status: 'ok', response: {} } ]));
      return;
    }
    if (event !== 'phx_join') {
      throw new Error(`Unexpected consumer event: ${ event }`);
    }
    joins++;
    order.push('join');
    acknowledgeJoin = () => {
      subscription = { client, joinRef, topic };
      acknowledged = true;
      order.push('ack');
      client.send(JSON.stringify([ joinRef, ref, topic, 'phx_reply', { status: 'ok', response: {} } ]));
    };
    if (!holdJoin) {
      acknowledgeJoin();
    }
  });
});

beforeAll(async() => {
  fetchMock.disableMocks();
  await new Promise<void>((resolve) => httpServer.listen(0, '127.0.0.1', resolve));
  window.__envs = {
    ...originalEnvs,
    NEXT_PUBLIC_API_HOST: '127.0.0.1',
    NEXT_PUBLIC_API_PORT: String((httpServer.address() as AddressInfo).port),
    NEXT_PUBLIC_API_PROTOCOL: 'http',
    NEXT_PUBLIC_API_BASE_PATH: '/',
    NEXT_PUBLIC_USE_NEXT_JS_PROXY: 'false',
    NEXT_PUBLIC_APP_ENV: 'testing',
  };
  ({ 'default': useRefresh } = await import('./useTokenInstanceMetadataRefresh'));
  ({ toaster } = await import('toolkit/chakra/toaster'));
  ({ getResourceKey } = await import('lib/api/useApiQuery'));
}, 30_000);

beforeEach(() => {
  canonicalInstance = initialInstance;
  patches = 0;
  gets = 0;
  joins = 0;
  acknowledged = false;
  holdJoin = false;
  holdResponse = false;
  publishOnPatch = true;
  patchResponse = undefined;
  acknowledgeJoin = undefined;
  subscription = undefined;
  media = transcript.media[0];
  order.length = 0;
  queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  queryClient.setQueryData(instanceKey(), initialInstance);
});

afterEach(async() => {
  cleanup();
  if (!patchResponse?.writableEnded) {
    patchResponse?.end('{}');
  }
  queryClient.clear();
  toaster.remove();
  const closed = Array.from(wsServer.clients, (client) => new Promise<void>((resolve) => {
    client.once('close', resolve);
    client.terminate();
  }));
  await Promise.all(closed);
});

afterAll(async() => {
  window.__envs = originalEnvs;
  fetchMock.enableMocks();
  await new Promise<void>((resolve, reject) => wsServer.close((error) => error ? reject(error) : resolve()));
  await new Promise<void>((resolve, reject) => httpServer.close((error) => error ? reject(error) : resolve()));
});

function instanceKey() {
  return getResourceKey('general:token_instance', { pathParams: { hash: HASH, id: ID } });
}

function RefreshConsumer() {
  useRefresh(HASH, ID);
  return null;
}

function Host() {
  const [ visible, setVisible ] = React.useState(true);
  context = useMetadataUpdateContext();
  changeVisibility = setVisible;
  return React.createElement(React.Fragment, null,
    visible && React.createElement(RefreshConsumer),
    React.createElement('output', null, context?.status));
}

async function openRefresh() {
  const port = (httpServer.address() as AddressInfo).port;
  render(React.createElement(QueryClientProvider, { client: queryClient },
    React.createElement(SocketProvider, {
      url: `ws://127.0.0.1:${ port }/socket`,
      options: socketOptions,
      children: React.createElement(MetadataUpdateProvider, null, React.createElement(Host)),
    })));
  await act(async() => context?.setStatus('MODAL_OPENED'));
}

async function metadataCompletionBeforeHttpResponse() {
  holdJoin = true;
  holdResponse = true;
  await openRefresh();
  await waitFor(() => expect(joins).toBe(1));
  expect(patches).toBe(0);
  expect(context?.status).toBe('MODAL_OPENED');

  await act(async() => acknowledgeJoin?.());
  await waitFor(() => expect(context?.status).toBe('SUCCESS'));
  expect(subscription?.topic).toBe(TOPIC);
  expect(patchResponse?.writableEnded).toBe(false);
  expect(order).toEqual([ 'join', 'ack', 'patch-after-ack', 'get' ]);
  expect(queryClient.getQueryData(instanceKey())).toEqual(canonicalInstance);

  await act(async() => {
    patchResponse?.writeHead(500);
    patchResponse?.end(JSON.stringify({ message: 'Response lost after completion' }));
    await delay(50);
  });
  expect(context?.status).toBe('SUCCESS');
  expect(patches).toBe(1);
}

async function metadataReconnectAfterCompletion() {
  publishOnPatch = false;
  await openRefresh();
  await waitFor(() => expect(patches).toBe(1));
  await waitFor(() => expect(context?.status).toBe('WAITING_FOR_RESPONSE'));
  expect(gets).toBe(0);

  await act(async() => {
    canonicalInstance = { ...initialInstance, image_url: media.image_url, animation_url: media.animation_url, metadata: media.metadata };
    subscription?.client.terminate();
  });
  await waitFor(() => expect(context?.status).toBe('SUCCESS'), { timeout: 5_000 });
  expect(joins).toBe(2);
  expect(patches).toBe(1);
  expect(gets).toBe(1);
  expect(queryClient.getQueryData(instanceKey())).toEqual(canonicalInstance);
}

async function metadataReopenAfterCompletion() {
  publishOnPatch = false;
  await openRefresh();
  await waitFor(() => expect(patches).toBe(1));
  await waitFor(() => expect(context?.status).toBe('WAITING_FOR_RESPONSE'));
  await act(async() => changeVisibility(false));
  canonicalInstance = { ...initialInstance, image_url: media.image_url, animation_url: media.animation_url, metadata: media.metadata };
  expect(queryClient.getQueryData(instanceKey())).toEqual(initialInstance);

  await act(async() => changeVisibility(true));
  await waitFor(() => expect(context?.status).toBe('SUCCESS'));
  expect(patches).toBe(1);
  expect(gets).toBe(1);
  expect(queryClient.getQueryData(instanceKey())).toEqual(canonicalInstance);
}

async function metadataReconnectBeforeCompletionKeepsWaiting() {
  publishOnPatch = false;
  await openRefresh();
  await waitFor(() => expect(context?.status).toBe('WAITING_FOR_RESPONSE'));
  await act(async() => subscription?.client.terminate());
  await waitFor(() => expect(gets).toBe(1), { timeout: 5_000 });
  await waitFor(() => expect(queryClient.isFetching({ queryKey: instanceKey() })).toBe(0));
  expect(context?.status).toBe('WAITING_FOR_RESPONSE');
  expect(queryClient.getQueryData(instanceKey())).toEqual(initialInstance);

  await act(async() => {
    canonicalInstance = { ...initialInstance, image_url: media.image_url, animation_url: media.animation_url, metadata: media.metadata };
    publish('fetched_token_instance_metadata', media.fetched_event);
  });
  await waitFor(() => expect(context?.status).toBe('SUCCESS'));
  expect(patches).toBe(1);
  expect(gets).toBe(2);
  expect(queryClient.getQueryData(instanceKey())).toEqual(canonicalInstance);
}

async function metadataRefreshPreservesCanonicalMedia() {
  await openRefresh();
  await waitFor(() => expect(context?.status).toBe('SUCCESS'));
  for (const [ index, fixture ] of transcript.media.entries()) {
    if (index > 0) {
      media = fixture;
      await act(async() => context?.setStatus('MODAL_OPENED'));
      await waitFor(() => expect(context?.status).toBe('SUCCESS'));
    }
    expect(gets).toBe(index + 1);
    const cached = queryClient.getQueryData<TokenInstance>(instanceKey());
    expect(cached).toEqual(canonicalInstance);
    expect(cached?.image_url).toBe(fixture.image_url);
    expect(cached?.animation_url).toBe(fixture.animation_url);
    expect(cached?.metadata).toEqual(fixture.metadata);
  }
  expect(patches).toBe(4);
}

async function metadataTimeoutReconcilesCanonicalInstance() {
  publishOnPatch = false;
  await openRefresh();
  await waitFor(() => expect(patches).toBe(1));
  canonicalInstance = { ...initialInstance, image_url: media.image_url, animation_url: media.animation_url, metadata: media.metadata };
  await waitFor(() => expect(context?.status).toBe('SUCCESS'), { timeout: 125_000, interval: 100 });
  expect(gets).toBe(1);
  expect(patches).toBe(1);
  expect(queryClient.getQueryData(instanceKey())).toEqual(canonicalInstance);
}

async function metadataFailureReasonsClearDeadline() {
  publishOnPatch = false;
  holdResponse = true;
  await openRefresh();
  await waitFor(() => expect(patches).toBe(1));

  await act(async() => {
    publish('not_fetched_token_instance_metadata', transcript.unrelated_failure);
    await delay(50);
  });
  expect(context?.status).toBe('MODAL_OPENED');
  expect(toaster.getVisibleToasts().find((toast) => toast.id === 'token-instance-metadata-fetcher')?.type).toBe('loading');

  await act(async() => publish('not_fetched_token_instance_metadata', transcript.cooldown));
  await waitFor(() => expect(context?.status).toBe('ERROR'));
  expect(toaster.getVisibleToasts().find((toast) => toast.id === 'token-instance-metadata-fetcher')?.description)
    .toBe('Metadata refresh is on cooldown. Please wait before trying again.');
  patchResponse?.end('{}');

  await act(async() => context?.setStatus('MODAL_OPENED'));
  await waitFor(() => expect(patches).toBe(2));
  await act(async() => publish('not_fetched_token_instance_metadata', transcript.failure));
  await waitFor(() => expect(context?.status).toBe('ERROR'));
  expect(toaster.getVisibleToasts().find((toast) => toast.id === 'token-instance-metadata-fetcher')?.description)
    .toBe('Unable to fetch token instance metadata. Please try again.');
  patchResponse?.end('{}');

  await act(async() => delay(121_000));
  expect(context?.status).toBe('ERROR');
  expect(gets).toBe(0);
  expect(queryClient.getQueryData(instanceKey())).toEqual(initialInstance);
}

test(metadataCompletionBeforeHttpResponse.name, metadataCompletionBeforeHttpResponse);
test(metadataReconnectAfterCompletion.name, { timeout: 10_000 }, metadataReconnectAfterCompletion);
test(metadataReopenAfterCompletion.name, metadataReopenAfterCompletion);
test(metadataReconnectBeforeCompletionKeepsWaiting.name, { timeout: 10_000 }, metadataReconnectBeforeCompletionKeepsWaiting);
test(metadataRefreshPreservesCanonicalMedia.name, metadataRefreshPreservesCanonicalMedia);
test(metadataTimeoutReconcilesCanonicalInstance.name, { timeout: 130_000 }, metadataTimeoutReconcilesCanonicalInstance);
test(metadataFailureReasonsClearDeadline.name, { timeout: 130_000 }, metadataFailureReasonsClearDeadline);
