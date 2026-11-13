import { createRequire } from 'node:module';
import { expect, test } from 'vitest';
import { custom, type Address } from 'viem';
import { CORE_DEPLOYMENT } from '@/core/network/deployment';
import { creatorWalletFixture } from './creator-wallet.fixture';

const require = createRequire(import.meta.url);
const { JSDOM } = require('jsdom') as {
  JSDOM: new (html: string, options: { url: string }) => {
    window: Window & typeof globalThis & { close(): void };
  };
};

async function creator_wallet_lifecycle_cancels_obsolete_metadata_and_selects_current_pools() {
  const originalEnvironment = { ...process.env };
  const globals = ['window', 'document', 'self', 'IS_REACT_ACT_ENVIRONMENT'] as const;
  const descriptors = globals.map((key) =>
    Object.getOwnPropertyDescriptor(globalThis, key),
  );
  const fixture = await creatorWalletFixture();
  let closeView: (() => Promise<void>) | undefined;
  let closeDom: (() => void) | undefined;
  try {
    const [alpha, beta] = fixture.coins;
    const [walletA, walletB] = fixture.wallets;
    expect(fixture.pools.get(walletA.toLowerCase())).toEqual([alpha.Pool]);
    expect(fixture.pools.get(walletB.toLowerCase())).toEqual([beta.Pool]);
    expect(alpha.Pool).not.toBe(beta.Pool);
    expect(new URL(fixture.origin).hostname).toBe('127.0.0.1');
    for (const key of [
      'NEXT_PUBLIC_METADATA_API_URL', 'NEXT_PUBLIC_DATA_API_URL',
      'NEXT_PUBLIC_GATEWAY_URL', 'NEXT_PUBLIC_STATS_API_URL',
    ]) process.env[key] = fixture.origin;
    const dom = new JSDOM('<html><body><main id="root"></main></body></html>', {
      url: 'http://localhost:3050',
    });
    closeDom = () => dom.window.close();
    for (const [key, value] of Object.entries({
      window: dom.window,
      document: dom.window.document,
      self: dom.window,
      IS_REACT_ACT_ENVIRONMENT: true,
    })) {
      Object.defineProperty(globalThis, key, {
        value, configurable: true, writable: true,
      });
    }
    const { act, createElement } = await import('react');
    const { createRoot } = await import('react-dom/client');
    const { QueryClient, QueryClientProvider } = await import('@tanstack/react-query');
    const { createConfig, WagmiProvider } = await import('wagmi');
    const { injected } = await import('wagmi/connectors');
    const { reconnect, disconnect } = await import('wagmi/actions');
    const { default: MyTokenView } = await import('./my-token-view');
    const config = createConfig({
      storage: null,
      batch: { multicall: false },
      chains: [{
        id: 125,
        name: 'In-memory creator registry',
        nativeCurrency: { name: 'PAX', symbol: 'PAX', decimals: 18 },
        rpcUrls: { default: { http: [fixture.origin] } },
      }],
      connectors: [
        injected({
          target: {
            id: 'creator-wallet-lifecycle',
            name: 'Creator wallet store',
            provider: fixture.provider,
          },
        }),
      ],
      multiInjectedProviderDiscovery: false,
      transports: { 125: custom(fixture.provider, { retryCount: 0 }) },
    });
    const query = new QueryClient({
      defaultOptions: { queries: { retry: false, gcTime: Infinity } },
    });
    const container = document.getElementById('root')!;
    const root = createRoot(container);
    closeView = async () => {
      await act(async () => root.unmount());
      query.clear();
      await disconnect(config);
    };
    async function waitFor(predicate: () => boolean) {
      for (let attempt = 0; attempt < 250; attempt++) {
        if (predicate()) return;
        await act(async () => {
          await new Promise((resolve) => setTimeout(resolve, 20));
        });
      }
      throw new Error(`Wallet lifecycle did not settle: ${container.textContent}`);
    }
    function coinInfo() {
      return [...container.querySelectorAll('h3')].find(
        (heading) => heading.textContent === 'Coin info',
      )?.parentElement;
    }
    function expectCoin(coin: typeof alpha) {
      expect(coinInfo()?.textContent?.toLowerCase()).toContain(coin.Pool.toLowerCase());
      expect(coinInfo()?.textContent?.toLowerCase()).toContain(coin.Token.toLowerCase());
      expect(coinInfo()?.textContent).toContain(coin.Name);
      expect(
        [...container.querySelectorAll('button')].find(
          (button) => button.textContent === coin.Symbol,
        )?.className,
      ).toContain('bg-info-soft');
    }
    async function changePools(wallet: Address, list: readonly Address[]) {
      fixture.pools.set(wallet.toLowerCase(), list);
      await act(async () => {
        await query.invalidateQueries({ queryKey: ['readContract'] });
      });
    }
    const pendingA = fixture.holdMetadata(alpha.Token);
    await act(async () => {
      await reconnect(config, { connectors: config.connectors });
    });
    expect(config.state.status).toBe('connected');
    await act(async () => {
      root.render(
        createElement(
          WagmiProvider,
          { config, reconnectOnMount: false },
          createElement(QueryClientProvider, { client: query }, createElement(MyTokenView)),
        ),
      );
    });
    await waitFor(() => pendingA.started);
    await act(async () => fixture.switchWallet(walletB));
    await waitFor(() => coinInfo()?.textContent?.includes(beta.Name) === true);
    expectCoin(beta);
    expect(container.textContent).not.toContain(alpha.Name);
    expect(container.textContent?.toLowerCase()).not.toContain(alpha.Pool.toLowerCase());
    await waitFor(() => pendingA.cancelled);
    pendingA.release();
    await waitFor(() => pendingA.finished);
    expectCoin(beta);
    const oldWalletQueries = query.getQueriesData({
      queryKey: ['creator-pool-metadata', CORE_DEPLOYMENT.chainId, walletA],
    });
    expect(oldWalletQueries.length).toBeGreaterThan(0);
    for (const [, data] of oldWalletQueries) expect(data).toBeUndefined();

    // Same wallet, same count, different pool identity must also refresh selection.
    await changePools(walletB, [alpha.Pool as Address]);
    await waitFor(() => coinInfo()?.textContent?.includes(alpha.Name) === true);
    expectCoin(alpha);
    expect(container.textContent).not.toContain(beta.Name);
    expect(fixture.requests.filter(
      (path) => path === `/metadata/${alpha.Token.toLowerCase()}`,
    ).length).toBe(2);

    const obsoletePool = fixture.holdMetadata(alpha.Token);
    await act(async () => {
      void query.invalidateQueries({ queryKey: ['creator-pool-metadata', 125, walletB] });
    });
    await waitFor(() => obsoletePool.started);
    await changePools(walletB, [beta.Pool as Address]);
    await waitFor(() => coinInfo()?.textContent?.includes(beta.Name) === true);
    expectCoin(beta);
    await waitFor(() => obsoletePool.cancelled);
    obsoletePool.release();
    await waitFor(() => obsoletePool.finished);
    expectCoin(beta);
    expect(container.textContent).not.toContain(alpha.Name);

    await changePools(walletB, []);
    await waitFor(() => container.textContent?.includes('No coins created yet') === true);
    expect(coinInfo()).toBeUndefined();
    await changePools(walletB, [alpha.Pool as Address]);
    await waitFor(() => coinInfo()?.textContent?.includes(alpha.Name) === true);
    expectCoin(alpha);

    const pendingLogout = fixture.holdMetadata(alpha.Token);
    await act(async () => {
      void query.invalidateQueries({ queryKey: ['creator-pool-metadata', 125, walletB] });
    });
    await waitFor(() => pendingLogout.started);
    await act(async () => {
      await disconnect(config);
    });
    await waitFor(() => container.textContent?.includes('Sign in to see your coins') === true);
    expect(coinInfo()).toBeUndefined();
    await waitFor(() => pendingLogout.cancelled);
    pendingLogout.release();
    await waitFor(() => pendingLogout.finished);
    expect(container.textContent).not.toContain(alpha.Name);
    expect(fixture.errors).toEqual([]);
    expect(fixture.provider.errors).toEqual([]);
  } finally {
    try {
      if (closeView) await closeView();
    } finally {
      closeDom?.();
      globals.forEach((key, index) => {
        const descriptor = descriptors[index];
        if (descriptor) Object.defineProperty(globalThis, key, descriptor);
        else Reflect.deleteProperty(globalThis, key);
      });
      for (const key of Object.keys(process.env))
        if (!(key in originalEnvironment)) delete process.env[key];
      Object.assign(process.env, originalEnvironment);
      await fixture.close();
    }
  }
}

test(
  creator_wallet_lifecycle_cancels_obsolete_metadata_and_selects_current_pools,
  creator_wallet_lifecycle_cancels_obsolete_metadata_and_selects_current_pools,
  30000,
);
