import { EventEmitter } from 'node:events';
import { createRequire } from 'node:module';
import React, { act, type ReactNode } from 'react';
import { QueryClient, QueryClientProvider, onlineManager } from '@tanstack/react-query';
import { createConfig, http, WagmiProvider } from 'wagmi';
import { connect, disconnect } from 'wagmi/actions';
import { injected } from 'wagmi/connectors';
import { createWalletClient, type EIP1193Provider } from 'viem';
import { expect } from 'vitest';
import { AppRouterContext } from 'next/dist/shared/lib/app-router-context.shared-runtime';
import { PathnameContext, SearchParamsContext } from 'next/dist/shared/lib/hooks-client-context.shared-runtime';
import { adaptForAppRouterInstance } from 'next/dist/shared/lib/router/adapters';
import singletonRouter from 'next/router';
import { paxeerWalletChain } from '@/core/wallet-sdk/client';
import { WalletRuntimeProvider } from '@/core/wallet-sdk/runtime';
import { applyBrowserSurface } from '@/core/app-foundation/browser-surface.test-support';
import { accountEpoch, clearAccountSession, installAccountSession } from '@/widgets/profile/account-transport';

const { JSDOM } = createRequire(import.meta.url)('jsdom') as typeof import('jsdom');
const dom = new JSDOM('<!doctype html><html><body></body></html>', { url: 'http://localhost:3050', pretendToBeVisual: true });
const globals = globalThis as unknown as Record<string, unknown>;
const installed: string[] = [];
for (const key of Object.getOwnPropertyNames(dom.window)) {
  if (key in globals || key.startsWith('_')) continue;
  const value = (dom.window as unknown as Record<string, unknown>)[key];
  globals[key] = typeof value === 'function' && !/^[A-Z]/.test(key) ? value.bind(dom.window) : value;
  installed.push(key);
}
globals.IS_REACT_ACT_ENVIRONMENT = true;
const closeBrowserSurface = applyBrowserSurface();
const { createRoot } = await import('react-dom/client');
const rpcUrl = process.env.WALLET_TEST_RPC_URL;
if (!rpcUrl) throw new Error('Wallet chain is required');
const rpc = createWalletClient({ chain: paxeerWalletChain, transport: http(rpcUrl) });
const provider = Object.assign(new EventEmitter(), { request: rpc.request }) as EIP1193Provider;
export const config = createConfig({ chains: [paxeerWalletChain], transports: { 125: http(rpcUrl) }, connectors: [injected({ target: { id: 'heading-wallet', name: 'Wallet', provider: () => provider } })], storage: null, multiInjectedProviderDiscovery: false });
const views: { root: ReturnType<typeof createRoot>; container: HTMLDivElement; queries: QueryClient }[] = [];

export async function connectWallet() {
  await connect(config, { connector: config.connectors[0]! });
  const [wallet] = await rpc.getAddresses();
  if (!wallet) throw new Error('Wallet account is required');
  return wallet;
}

export function signIn(wallet: `0x${string}`) {
  installAccountSession({ wallet, sessionId: crypto.randomUUID(), deviceLabel: 'Browser', accessExpiresAt: new Date(Date.now() + 3600000).toISOString(), refreshExpiresAt: new Date(Date.now() + 7200000).toISOString(), revokedAt: null, scopes: ['account'] }, crypto.randomUUID(), accountEpoch());
}

export function mount(node: ReactNode, path: string, search = '', queries = new QueryClient({ defaultOptions: { queries: { retry: false, refetchOnMount: false, gcTime: 0 } } })) {
  onlineManager.setOnline(false);
  const container = document.createElement('div');
  document.body.append(container);
  const root = createRoot(container);
  const render = (content: ReactNode) => act(() => root.render(
    <WagmiProvider config={config} reconnectOnMount={false}>
      <QueryClientProvider client={queries}>
        <WalletRuntimeProvider>
          <AppRouterContext.Provider value={adaptForAppRouterInstance(singletonRouter)}>
            <PathnameContext.Provider value={path}>
              <SearchParamsContext.Provider value={new URLSearchParams(search)}>{content}</SearchParamsContext.Provider>
            </PathnameContext.Provider>
          </AppRouterContext.Provider>
        </WalletRuntimeProvider>
      </QueryClientProvider>
    </WagmiProvider>
  ));
  views.push({ root, container, queries });
  render(node);
  return { container, queries, render };
}

export function expectHeading(container: ParentNode, text: string) {
  const headings = container.querySelectorAll('h1');
  expect(headings).toHaveLength(1);
  expect(headings[0]?.textContent).toBe(text);
  let previous = 1;
  for (const heading of container.querySelectorAll('h1,h2,h3,h4,h5,h6')) {
    const level = Number(heading.tagName.slice(1));
    expect(level).toBeLessThanOrEqual(previous + 1);
    previous = level;
  }
}

export async function cleanup() {
  for (const view of views.splice(0)) {
    await act(async () => view.root.unmount());
    await view.queries.cancelQueries();
    view.queries.clear();
    view.container.remove();
  }
  clearAccountSession(false);
  await disconnect(config);
  onlineManager.setOnline(true);
}

export async function close() {
  await closeBrowserSurface();
  dom.window.close();
  for (const key of installed) delete globals[key];
  delete globals.IS_REACT_ACT_ENVIRONMENT;
}
