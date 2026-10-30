// @vitest-environment jsdom

import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { cleanup, render, waitFor } from '@testing-library/react';
import { generateKeyPairSync, sign } from 'node:crypto';
import { createServer } from 'node:http';
import type { AddressInfo } from 'node:net';
import React from 'react';

import type { PaxeerXTxSignature } from 'types/api/paxeerXSignature';

import type { Provider as ChakraProvider } from 'toolkit/chakra/provider';
import { afterAll, afterEach, beforeAll, expect, test } from 'vitest';

import type TxDetailsSequencerSignature from './TxDetailsSequencerSignature';

const hash = `0x${ '12'.repeat(32) }`;
const missingHash = `0x${ '34'.repeat(32) }`;
const receipt = Buffer.from('sequencer receipt');
const { privateKey, publicKey } = generateKeyPairSync('ed25519');
const answer: PaxeerXTxSignature = {
  rung: 'receipted',
  receipt: `0x${ receipt.toString('hex') }`,
  receipt_key: {
    key_id: `0x${ '56'.repeat(32) }`,
    public_key: `0x${ publicKey.export({ format: 'der', type: 'spki' }).subarray(-32).toString('hex') }`,
  },
  signature: `0x${ sign(null, new Uint8Array(receipt), privateKey).toString('hex') }`,
};
const requests: Array<string | undefined> = [];
const server = createServer((request, response) => {
  requests.push(request.url);
  response.setHeader('Content-Type', 'application/json');
  if (request.url === `/api/v2/transactions/${ hash }/signature`) {
    response.writeHead(200);
    response.end(JSON.stringify(answer));
  } else {
    response.writeHead(404);
    response.end(JSON.stringify({ message: 'Not found' }));
  }
});
const originalEnvs = window.__envs;
let Component: typeof TxDetailsSequencerSignature;
let Provider: typeof ChakraProvider;
let queryClient: QueryClient;

beforeAll(async() => {
  fetchMock.disableMocks();
  await new Promise<void>((resolve) => server.listen(0, '127.0.0.1', resolve));
  window.__envs = {
    ...originalEnvs,
    NEXT_PUBLIC_PAXEER_X_ENABLED: 'true',
    NEXT_PUBLIC_API_PROTOCOL: 'http',
    NEXT_PUBLIC_API_HOST: '127.0.0.1',
    NEXT_PUBLIC_API_PORT: String((server.address() as AddressInfo).port),
    NEXT_PUBLIC_API_BASE_PATH: '/',
    NEXT_PUBLIC_USE_NEXT_JS_PROXY: 'false',
    NEXT_PUBLIC_APP_ENV: 'testing',
  };
  await import('ui/paxeerX/account/testWrapper');
  ({ Provider } = await import('toolkit/chakra/provider'));
  ({ 'default': Component } = await import('./TxDetailsSequencerSignature'));
});

afterEach(() => {
  cleanup();
  queryClient.clear();
  requests.length = 0;
});

afterAll(async() => {
  window.__envs = originalEnvs;
  fetchMock.enableMocks();
  await new Promise<void>((resolve, reject) => server.close((error) => error ? reject(error) : resolve()));
});

function renderSignature(txHash: string) {
  queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  const view = render(
    <Provider>
      <QueryClientProvider client={ queryClient }>
        <div data-testid="signature-row"><Component hash={ txHash }/></div>
      </QueryClientProvider>
    </Provider>,
  );
  return { ...view, container: view.getByTestId('signature-row') };
}

async function signature_row_renders_key_and_signature() {
  const { container, findByText } = renderSignature(hash);

  expect(await findByText(`Key id: ${ answer.receipt_key.key_id }`)).toBeDefined();
  expect(await findByText(answer.signature)).toBeDefined();
  expect(answer.signature).toMatch(/^0x[0-9a-f]{128}$/);
  expect(container.querySelector('[data-rung="receipted"]')?.textContent).toBe('Receipted');
  expect(container.textContent).toContain('Sequencer signature');
  expect(requests).toEqual([ `/api/v2/transactions/${ hash }/signature` ]);
}

async function signature_row_hidden_on_404() {
  const { container } = renderSignature(missingHash);
  const queryKey = [ 'general:paxeer_x_tx_signature', { hash: missingHash } ];

  await waitFor(() => expect(queryClient.getQueryState(queryKey)?.status).toBe('error'));

  expect(queryClient.getQueryState(queryKey)?.error).toMatchObject({ status: 404 });
  expect(requests).toEqual([ `/api/v2/transactions/${ missingHash }/signature` ]);
  expect(container.innerHTML).toBe('');
}

test(signature_row_renders_key_and_signature);
test(signature_row_hidden_on_404);
