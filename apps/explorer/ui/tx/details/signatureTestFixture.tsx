import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { cleanup, render as baseRender } from '@testing-library/react';
import { createServer } from 'node:http';
import type { AddressInfo } from 'node:net';
import React from 'react';

import type { PaxeerXTxSignature } from 'types/api/paxeerXSignature';

import { afterAll, afterEach, beforeEach } from 'vitest';

import vector from './fixtures/vector-v4-committed.json';

export const settlement = vector.settlement;
export const hash = settlement.receipt.txHash;
export const missingHash = `0x${ '34'.repeat(32) }`;
export const answer: PaxeerXTxSignature = {
  rung: 'receipted',
  receipt: settlement.receipt.receipt,
  receipt_key: {
    key_id: settlement.receipt.keyId,
    public_key: settlement.receipt.signingKey,
  },
  signature: `0x${ settlement.receipt.receipt.slice(-128) }`,
};
export const committedAnswer: PaxeerXTxSignature = {
  ...answer,
  rung: 'committed',
  committed_height: Number(settlement.execution_header.height),
};

type SignatureResponse = { status: 200; body: PaxeerXTxSignature } | { status: 404; body: { message: 'Not found' } };

const refusal: SignatureResponse = { status: 404, body: { message: 'Not found' } };
let signatureResponse: SignatureResponse = { status: 200, body: answer };
export const requests: Array<string | undefined> = [];
export const exchanges: Array<{ path: string | undefined; status: number; body: SignatureResponse['body'] }> = [];
const server = createServer((request, response) => {
  requests.push(request.url);
  const result = request.url === `/api/v2/transactions/${ hash }/signature` ? signatureResponse : refusal;
  exchanges.push({ path: request.url, ...result });
  response.writeHead(result.status, { 'content-type': 'application/json' });
  response.end(JSON.stringify(result.body));
});

await new Promise<void>((resolve) => server.listen(0, '127.0.0.1', resolve));
server.unref();
const originalEnvs = window.__envs;
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

const { RouterContext } = await import('next/dist/shared/lib/router-context.shared-runtime');
const { 'default': Router } = await import('next/dist/shared/lib/router/router');
const { 'default': PageLoader } = await import('next/dist/client/page-loader');
const { 'default': App } = await import('next/app');
const { Provider } = await import('toolkit/chakra/provider');
const { AppContextProvider } = await import('lib/contexts/app');
const { GrowthBookProvider } = await import('@growthbook/growthbook-react');
const { SocketProvider } = await import('lib/socket/context');
const { base } = await import('mocks/txs/tx');
export const transaction = { ...base, hash };
export const { 'default': TxInfo } = await import('./TxInfo');
export const { 'default': Signature } = await import('./TxDetailsSequencerSignature');
const { getResourceKey } = await import('lib/api/useApiQuery');
await import('ui/shared/layout/testWrapper');

const clients: Array<QueryClient> = [];
const routers: Array<InstanceType<typeof Router>> = [];

beforeEach(() => {
  fetchMock.disableMocks();
  requests.length = 0;
  exchanges.length = 0;
  signatureResponse = { status: 200, body: answer };
});

afterEach(async() => {
  cleanup();
  routers.splice(0).forEach((router) => window.removeEventListener('popstate', router.onPopState));
  await Promise.all(clients.splice(0).map(async(client) => {
    await client.cancelQueries();
    client.clear();
  }));
});

afterAll(async() => {
  server.closeAllConnections();
  await new Promise<void>((resolve, reject) => server.close((error) => error ? reject(error) : resolve()));
  window.__envs = originalEnvs;
  fetchMock.enableMocks();
});

export function serveSignature(body: PaxeerXTxSignature) {
  signatureResponse = { status: 200, body };
}

export function refuseSignature() {
  signatureResponse = refusal;
}

export function signatureQueryKey(txHash = hash) {
  return getResourceKey('general:paxeer_x_tx_signature', { pathParams: { hash: txHash } });
}

function SignatureTransactionPage() {
  return <TxInfo data={ transaction } isLoading={ false }/>;
}

export function render(ui: React.ReactElement, txHash = hash) {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false, refetchOnWindowFocus: false } } });
  clients.push(client);
  window.__NEXT_DATA__ = { props: {}, page: '/tx/[hash]', query: { hash: txHash }, buildId: 'repair-22-33-520' };
  window.__DEV_MIDDLEWARE_MATCHERS = [];
  const router = new Router('/tx/[hash]', { hash: txHash }, `/tx/${ txHash }`, {
    initialProps: {},
    pageLoader: new PageLoader('repair-22-33-520', ''),
    App,
    Component: SignatureTransactionPage,
    wrapApp: (component) => component,
    subscription: async() => {},
    isFallback: false,
  });
  routers.push(router);
  const wrapper = ({ children }: { children: React.ReactNode }) => (
    <RouterContext.Provider value={ router }>
      <QueryClientProvider client={ client }>
        <AppContextProvider pageProps={{ cookies: '', referrer: '', query: { hash: txHash }, adBannerProvider: null, apiData: null, uuid: 'signature' }}>
          <GrowthBookProvider>
            <SocketProvider>
              <Provider>{ children }</Provider>
            </SocketProvider>
          </GrowthBookProvider>
        </AppContextProvider>
      </QueryClientProvider>
    </RouterContext.Provider>
  );
  return { ...baseRender(ui, { wrapper }), client };
}

export function renderSignature(txHash = hash) {
  const view = render(<div data-testid="signature-row"><Signature hash={ txHash }/></div>, txHash);
  return { ...view, container: view.getByTestId('signature-row') };
}
