// @vitest-environment jsdom
import { EventEmitter } from 'node:events';
import { readFileSync } from 'node:fs';
import { act, createElement, type ReactNode } from 'react';
import { createRoot, type Root } from 'react-dom/client';
import { afterEach, beforeAll, expect, test as registerTest } from 'vitest';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { createConfig, http, WagmiProvider } from 'wagmi';
import { connect, disconnect } from 'wagmi/actions';
import { injected } from 'wagmi/connectors';
import {
  createPublicClient,
  createWalletClient,
  UserRejectedRequestError,
  type Address,
  type EIP1193Provider,
  type PublicClient,
  type WalletClient,
} from 'viem';
import { AppRouterContext } from 'next/dist/shared/lib/app-router-context.shared-runtime';
import { PathParamsContext, SearchParamsContext } from 'next/dist/shared/lib/hooks-client-context.shared-runtime';
import { adaptForAppRouterInstance } from 'next/dist/shared/lib/router/adapters';
import singletonRouter from 'next/router';
import TokenMetadataPage from '@/app/tokens/[tokenAddress]/metadata/page';
import { fetchMetadataV2, fetchMetadataMedia } from '@/core/clients/metadata';
import { RuntimeAuthorityProvider } from '@/core/runtime-authority';
import CreateWizard from '@/widgets/create/create-wizard';
import {
  ChainCreationAdapter,
  KnownPurchaseNotSubmittedError,
  purchaseSubmissionError,
  type LaunchDeployment,
} from '@/widgets/create/creation-adapters';
import NewModule from '@/page-modules/new';

declare const jsdom: { reconfigure(options: { url: string }): void };
const mounts: { root: Root; host: HTMLElement; queries: QueryClient }[] = [];
function test(testFunction: () => void | Promise<void>, options?: { timeout: number }) {
  registerTest(testFunction, options ?? {}, testFunction);
}
const chain = {
  id: 125,
  name: 'Paxeer',
  nativeCurrency: { name: 'PAX', symbol: 'PAX', decimals: 18 },
  rpcUrls: { default: { http: ['http://127.0.0.1:1'] } },
} as const;
const router = adaptForAppRouterInstance(singletonRouter);
const unavailableConfig = createConfig({
  chains: [chain],
  transports: { 125: http('http://127.0.0.1:1', { retryCount: 0 }) },
  multiInjectedProviderDiscovery: false,
});

beforeAll(() => {
  Object.assign(globalThis, { IS_REACT_ACT_ENVIRONMENT: true });
  jsdom.reconfigure({ url: 'http://127.0.0.1:1' });
});

afterEach(async () => {
  for (const { root, host, queries } of mounts.splice(0)) {
    await act(async () => root.unmount());
    queries.clear();
    host.remove();
  }
});

async function mount(node: ReactNode, tokenAddress = '', config = unavailableConfig) {
  const host = document.createElement('div');
  document.body.appendChild(host);
  const root = createRoot(host);
  const queries = new QueryClient({ defaultOptions: { queries: { retry: false, gcTime: 0 } } });
  mounts.push({ root, host, queries });
  await act(async () => root.render(
    createElement(WagmiProvider, { config, reconnectOnMount: false },
      createElement(QueryClientProvider, { client: queries },
        createElement(AppRouterContext.Provider, { value: router },
          createElement(SearchParamsContext.Provider, { value: new URLSearchParams() },
            createElement(PathParamsContext.Provider, { value: { tokenAddress } }, node))))),
  ));
  return host;
}

async function waitFor(check: () => boolean) {
  const deadline = Date.now() + 5000;
  while (!check() && Date.now() < deadline) {
    await act(async () => { await new Promise(resolve => setTimeout(resolve, 10)); });
  }
  expect(check()).toBe(true);
}

function creationFixture() {
  const file = process.env.L27_PHASE2_FIXTURE;
  if (!file) throw new Error('Creation deployment is required');
  return JSON.parse(readFileSync(file, 'utf8')).phase2 as {
    ownedFixture: boolean;
    rpcUrl: string;
    creator: Address;
    deployment: LaunchDeployment;
  };
}

async function metadataInvalidAddressOffersWayBack() {
  const host = await mount(createElement(TokenMetadataPage), 'invalid');
  expect(host.querySelector('[role="alert"]')?.textContent).toBe('This is not a valid coin address');
  expect(host.querySelector('[role="status"]')).toBeNull();
  expect(host.querySelector('a[href="/explore"]')?.textContent).toBe('Back to coins');
  expect(host.textContent).not.toContain('Try again');
}

async function metadataReadFailureOffersRetryAndWayBack() {
  const previous = process.env.NEXT_PUBLIC_METADATA_API_URL;
  process.env.NEXT_PUBLIC_METADATA_API_URL = 'http://127.0.0.1:1';
  try {
    const host = await mount(createElement(TokenMetadataPage), creationFixture().creator);
    await waitFor(() => host.querySelector('[role="alert"]') !== null);
    expect(host.querySelector('[role="status"]')).toBeNull();
    expect(host.textContent).toContain("Can't load coin details right now");
    const retry = [...host.querySelectorAll('button')].find(button => button.textContent === 'Try again');
    expect(retry?.disabled).toBe(false);
    await act(async () => retry!.click());
    await waitFor(() => host.querySelector('[role="alert"]') !== null);
    expect(host.querySelector('a[href="/explore"]')).not.toBeNull();
  } finally {
    if (previous === undefined) delete process.env.NEXT_PUBLIC_METADATA_API_URL;
    else process.env.NEXT_PUBLIC_METADATA_API_URL = previous;
  }
}

async function metadataReadsHonorCancellation() {
  const controller = new AbortController();
  controller.abort(new DOMException('Cancelled', 'AbortError'));
  const token = creationFixture().creator;
  await expect(fetchMetadataV2(token, controller.signal)).rejects.toMatchObject({ name: 'AbortError' });
  await expect(fetchMetadataMedia(token, controller.signal)).rejects.toMatchObject({ name: 'AbortError' });
}

async function creationAuthorityFailureKeepsDraftAndWayBack() {
  const fixture = creationFixture();
  expect(fixture.ownedFixture).toBe(true);
  jsdom.reconfigure({ url: 'https://127.0.0.1:65534' });
  const events = new EventEmitter();
  const client = createPublicClient({ transport: http(fixture.rpcUrl, { retryCount: 0 }) });
  const provider = {
    on: events.on.bind(events),
    removeListener: events.removeListener.bind(events),
    request: (request: { method: string; params?: unknown }) => client.request({
      ...request,
      method: request.method === 'eth_requestAccounts' ? 'eth_accounts' : request.method,
    } as Parameters<typeof client.request>[0]),
  } as EIP1193Provider;
  const config = createConfig({
    storage: null,
    chains: [chain],
    connectors: [injected({ target: { id: 'creation-wallet', name: 'Creator wallet', provider } })],
    transports: { 125: http(fixture.rpcUrl, { retryCount: 0 }) },
    multiInjectedProviderDiscovery: false,
  });
  try {
    await connect(config, { connector: config.connectors[0]! });
    const host = await mount(createElement(RuntimeAuthorityProvider, null, createElement(CreateWizard)), '', config);
    expect(config.state.status).toBe('connected');
    expect(config.state.connections.size).toBe(1);
    const name = host.querySelector<HTMLInputElement>('#create-name')!;
    await act(async () => {
      Object.getOwnPropertyDescriptor(HTMLInputElement.prototype, 'value')!.set!.call(name, 'Creator coin');
      name.dispatchEvent(new Event('input', { bubbles: true }));
    });
    await waitFor(() => host.textContent!.includes('Connection problem'));
    expect(name.value).toBe('Creator coin');
    expect(host.querySelector('a[href="/explore"]')?.textContent).toBe('Browse coins');
    expect([...host.querySelectorAll('button')].find(button => button.textContent === 'Create coin')?.disabled).toBe(true);
  } finally {
    for (const { root, host, queries } of mounts.splice(0)) {
      await act(async () => root.unmount());
      queries.clear();
      host.remove();
    }
    await disconnect(config);
    jsdom.reconfigure({ url: 'http://127.0.0.1:1' });
  }
}

async function creatorHistoryPagingEndsAtEmptyPage() {
  const fixture = creationFixture();
  const publicClient = createPublicClient({ chain, transport: http(fixture.rpcUrl) });
  const wallet = createWalletClient({ chain, account: fixture.creator, transport: http(fixture.rpcUrl) });
  const adapter = new ChainCreationAdapter(publicClient as PublicClient, wallet as WalletClient, fixture.deployment,
    () => ({ creator: fixture.creator, chainId: 125, epoch: 1 }));
  const page = await adapter.createdPools(fixture.creator);
  expect(page.length).toBeGreaterThan(0);
  expect(page.length).toBeLessThanOrEqual(25);
  expect(await adapter.createdPools(fixture.creator, 100n)).toEqual([]);
}

function purchaseWalletRefusalAllowsFreshReview() {
  const rejected = new UserRejectedRequestError(new Error('Request rejected'));
  const error = purchaseSubmissionError(rejected);
  expect(error).toBeInstanceOf(KnownPurchaseNotSubmittedError);
  expect(error).toMatchObject({ cause: rejected, message: 'Wallet rejected purchase. No transaction was submitted.' });
  const interrupted = new Error('Connection closed');
  expect(purchaseSubmissionError(interrupted)).toBe(interrupted);
}

function newCoinsRouteSelectsNewestFeed() {
  expect(NewModule).toThrow(expect.objectContaining({ digest: 'NEXT_REDIRECT;replace;/explore?preset=new;307;' }));
}

test(metadataInvalidAddressOffersWayBack);
test(metadataReadFailureOffersRetryAndWayBack);
test(metadataReadsHonorCancellation);
test(creationAuthorityFailureKeepsDraftAndWayBack);
test(creatorHistoryPagingEndsAtEmptyPage, { timeout: 30000 });
test(purchaseWalletRefusalAllowsFreshReview);
test(newCoinsRouteSelectsNewestFeed);
