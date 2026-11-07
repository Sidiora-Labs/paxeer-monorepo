// @vitest-environment jsdom

import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { renderHook, waitFor } from '@testing-library/react';
import React from 'react';

import { TX_HASH } from 'stubs/tx';
import { afterAll, expect, test, vi } from 'vitest';

import { NATIVE_RUNG_ORDER, RUNG_ORDER } from './rungs';
import './rungs.spec';
import useTxStatusLadderQuery from './useTxStatusLadderQuery';

const { server, requests, transcript } = await vi.hoisted(async() => {
  const { createServer } = await import('node:http');
  const { 'default': transcript } = await import('./status-transcript.json');
  const requests: Array<{ method: string | undefined; url: string | undefined }> = [];
  const server = createServer((request, response) => {
    if (request.method !== 'GET' || !/^\/api\/v2\/transactions\/0x[0-9a-f]{64}\/status$/i.test(request.url || '')) {
      response.writeHead(404).end();
      return;
    }

    requests.push({ method: request.method, url: request.url });
    const item = transcript.cases[Math.min(requests.length - 1, transcript.cases.length - 1)];

    response.writeHead(200, { 'Content-Type': 'application/json' });
    response.end(JSON.stringify(item.response));
  });

  await new Promise<void>((resolve, reject) => {
    server.once('error', reject);
    server.listen(0, '127.0.0.1', () => {
      server.off('error', reject);
      resolve();
    });
  });
  const address = server.address();

  if (!address || typeof address === 'string') {
    throw new Error('The status transcript server did not bind a TCP port');
  }

  window.__envs = {
    ...window.__envs,
    NEXT_PUBLIC_PAXEER_X_ENABLED: 'true',
    NEXT_PUBLIC_API_PROTOCOL: 'http',
    NEXT_PUBLIC_API_HOST: '127.0.0.1',
    NEXT_PUBLIC_API_PORT: String(address.port),
    NEXT_PUBLIC_API_BASE_PATH: '/',
  };

  return { server, requests, transcript };
});

afterAll(async() => {
  server.closeAllConnections();
  await new Promise<void>((resolve, reject) => server.close((error) => error ? reject(error) : resolve()));
  fetchMock.enableMocks();
});

function ladder_retired_model() {
  const expectedDescriptions = [
    'The transaction has been submitted to the node and is awaiting a receipt and a committed block.',
    'A transaction receipt is available, and finality requires its block to be committed.',
    'The transaction receipt is final because its block is committed.',
  ];

  for (const family of [ RUNG_ORDER, NATIVE_RUNG_ORDER ]) {
    expect(family.map((item) => item.rung)).toEqual([ 'submitted', 'receipted', 'committed' ]);
    expect(family.map((item) => item.label)).toEqual([ 'Submitted', 'Receipted', 'Committed' ]);
    expect(family.map((item) => item.description)).toEqual(expectedDescriptions);
    for (const item of family) {
      expect(item.description).not.toMatch(/sequencer|pinned|receipt keys/i);
      expect(item.description.match(/\./g)).toHaveLength(1);
    }
  }
}

async function ladder_refresh() {
  fetchMock.disableMocks();
  const client = new QueryClient({
    defaultOptions: { queries: { retry: false, refetchOnWindowFocus: false } },
  });
  const wrapper = ({ children }: { children: React.ReactNode }) => React.createElement(QueryClientProvider, { client }, children);
  const { result, unmount } = renderHook(() => useTxStatusLadderQuery(TX_HASH), { wrapper });

  try {
    for (const { response } of transcript.cases) {
      await waitFor(() => {
        expect(result.current.isPlaceholderData).toBe(false);
        expect(result.current.isError).toBe(false);
        expect(result.current.data).toEqual(response);
      }, { timeout: 6_000 });
    }

    expect(requests).toEqual(transcript.cases.map(() => ({ method: 'GET', url: `/api/v2/transactions/${ TX_HASH }/status` })));
    await new Promise((resolve) => setTimeout(resolve, 5_500));
    expect(requests).toHaveLength(3);
    expect(result.current.data).toEqual(transcript.cases[2].response);
    expect(result.current.isFetching).toBe(false);
  } finally {
    unmount();
    client.clear();
  }
}

test(ladder_retired_model.name, ladder_retired_model);
test(ladder_refresh.name, ladder_refresh, 25_000);
