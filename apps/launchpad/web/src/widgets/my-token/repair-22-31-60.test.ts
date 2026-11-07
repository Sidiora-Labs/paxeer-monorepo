import { createRequire } from 'node:module';
import { test, expect } from 'vitest';
import * as viem from 'viem';
import { CORE_DEPLOYMENT } from '@/core/network/deployment';
import { creatorFixture, type CreatorFixture } from './creator.fixture';

const require = createRequire(import.meta.url);
const { JSDOM } = require('jsdom') as {
  JSDOM: new (
    html: string,
    options: { url: string },
  ) => { window: Window & typeof globalThis & { close(): void } };
};

async function metadata_token_identity_creator_discovery_through_gateway() {
  const originalEnvironment = { ...process.env };
  const originalWindow = Object.getOwnPropertyDescriptor(globalThis, 'window');
  const originalDocument = Object.getOwnPropertyDescriptor(
    globalThis,
    'document',
  );
  let chain: CreatorFixture | undefined;
  let closeView: (() => Promise<void>) | undefined;
  let closeDom: (() => void) | undefined;
  let failure: unknown;
  async function waitFor(
    predicate: () => boolean,
    act: (callback: () => Promise<void>) => Promise<void>,
  ) {
    for (let attempt = 0; attempt < 250; attempt++) {
      if (predicate()) return;
      await act(async () => {
        await new Promise((resolve) => setTimeout(resolve, 20));
      });
    }
    throw new Error(
      `Rendered creator coin did not settle: ${document.body.textContent}`,
    );
  }
  try {
    chain = await creatorFixture();
    expect(new URL(chain.origin).hostname).toBe('127.0.0.1');
    const [alpha, beta, unknown] = chain.coins;
    expect(
      new Set(
        chain.coins.flatMap((coin) => [
          coin.Pool.toLowerCase(),
          coin.Token.toLowerCase(),
        ]),
      ).size,
    ).toBe(6);
    const client = viem.createPublicClient({
      transport: viem.http(`${chain.origin}/rpc`, { retryCount: 0 }),
    });
    const creatorPools = await client.readContract({
      address: CORE_DEPLOYMENT.quoter,
      abi: viem.parseAbi([
        'function getPoolsByCreator(address) view returns (address[])',
      ]),
      functionName: 'getPoolsByCreator',
      args: [alpha.Creator],
    });
    expect(creatorPools.map((pool) => pool.toLowerCase())).toEqual(
      chain.coins.map((coin) => coin.Pool.toLowerCase()),
    );

    const gatewayOrigin = chain.origin;
    process.env.NEXT_PUBLIC_METADATA_API_URL = gatewayOrigin;
    process.env.NEXT_PUBLIC_DATA_API_URL = gatewayOrigin;
    process.env.NEXT_PUBLIC_GATEWAY_URL = gatewayOrigin;
    process.env.NEXT_PUBLIC_STATS_API_URL = gatewayOrigin;
    const knownResponse = await fetch(
      `${gatewayOrigin}/metadata/${alpha.Token}`,
    );
    expect(knownResponse.status).toBe(200);
    expect(await knownResponse.json()).toMatchObject({
      token_address: alpha.Token.toLowerCase(),
      pool_address: alpha.Pool.toLowerCase(),
      name: alpha.Name,
      symbol: alpha.Symbol,
    });
    const unknownResponse = await fetch(
      `${gatewayOrigin}/metadata/${unknown.Token}`,
    );
    expect(unknownResponse.status).toBe(200);
    expect(await unknownResponse.json()).toMatchObject({
      token_address: unknown.Token.toLowerCase(),
      pool_address: null,
      name: null,
      symbol: null,
    });
    const wrongIdentity = await fetch(
      `${gatewayOrigin}/metadata/${alpha.Pool}`,
    );
    expect(await wrongIdentity.json()).toMatchObject({
      pool_address: null,
      name: null,
      symbol: null,
    });

    const dom = new JSDOM('<html><body><main id="root"></main></body></html>', {
      url: 'http://localhost:3050',
    });
    closeDom = () => dom.window.close();
    Object.defineProperty(globalThis, 'window', {
      value: dom.window,
      configurable: true,
    });
    Object.defineProperty(globalThis, 'document', {
      value: dom.window.document,
      configurable: true,
    });
    Object.assign(globalThis, { IS_REACT_ACT_ENVIRONMENT: true });
    const { createElement, act } = await import('react');
    const { createRoot } = await import('react-dom/client');
    const { QueryClient, QueryClientProvider } = await import(
      '@tanstack/react-query'
    );
    const { createConfig, http, WagmiProvider } = await import('wagmi');
    const { injected } = await import('wagmi/connectors');
    const { reconnect, disconnect } = await import('wagmi/actions');
    const { default: MyTokenView, fetchCreatorPoolMetadata } = await import(
      './my-token-view'
    );
    const chainDefinition = {
      id: 125,
      name: 'Owned creator-discovery EVM',
      nativeCurrency: { name: 'PAX', symbol: 'PAX', decimals: 18 },
      rpcUrls: { default: { http: [`${chain.origin}/rpc`] } },
    } as const;
    const config = createConfig({
      storage: null,
      batch: { multicall: false },
      chains: [chainDefinition],
      connectors: [
        injected({
          target: {
            id: 'creator-proof',
            name: 'Owned EVM',
            provider: chain.provider,
          },
        }),
      ],
      multiInjectedProviderDiscovery: false,
      transports: { 125: http(`${chain.origin}/rpc`, { retryCount: 0 }) },
    });
    const query = new QueryClient({
      defaultOptions: { queries: { retry: false, gcTime: 0 } },
    });
    const container = document.getElementById('root')!;
    const root = createRoot(container);
    closeView = async () => {
      await act(async () => root.unmount());
      query.clear();
      await disconnect(config);
    };
    await act(async () => {
      await reconnect(config, { connectors: config.connectors });
    });
    expect(config.state.status).toBe('connected');
    expect(
      config.state.connections
        .get(config.state.current!)
        ?.accounts[0].toLowerCase(),
    ).toBe(alpha.Creator.toLowerCase());
    await act(async () => {
      root.render(
        createElement(
          WagmiProvider,
          { config, reconnectOnMount: false },
          createElement(
            QueryClientProvider,
            { client: query },
            createElement(MyTokenView),
          ),
        ),
      );
    });
    expect(container.textContent).not.toContain('Sign in');
    await waitFor(
      () =>
        container.textContent?.includes(alpha.Name) === true &&
        [...container.querySelectorAll('button')].some(
          (button) => button.textContent === beta.Symbol,
        ),
      act,
    );
    expect(container.textContent).toContain(alpha.Token);
    expect(container.textContent).toContain(alpha.Pool);
    expect(
      [...container.querySelectorAll('button')].map(
        (button) => button.textContent,
      ),
    ).toContain(alpha.Symbol);
    const select = async (label: string) => {
      const button = [...container.querySelectorAll('button')].find(
        (button) => button.textContent === label,
      );
      expect(button).toBeDefined();
      await act(async () => button!.click());
    };
    await select(beta.Symbol);
    await waitFor(
      () => container.textContent?.includes(beta.Name) === true,
      act,
    );
    expect(container.textContent).toContain(beta.Token);
    expect(container.textContent).toContain(beta.Pool);
    const unknownButton = [...container.querySelectorAll('button')].find(
      (button) => button.textContent?.includes(unknown.Pool.slice(0, 6)),
    );
    expect(unknownButton).toBeDefined();
    await act(async () => unknownButton!.click());
    await waitFor(
      () =>
        container.querySelector('[role="alert"]')?.textContent ===
        'Coin info unavailable',
      act,
    );
    expect(container.textContent).not.toContain(unknown.Name);
    expect(container.textContent).not.toContain(unknown.Symbol);
    await expect(
      fetchCreatorPoolMetadata(alpha.Pool, unknown.Token),
    ).rejects.toThrow('does not identify');
    await expect(
      fetchCreatorPoolMetadata(alpha.Pool, beta.Token),
    ).rejects.toThrow('does not identify');
    await expect(
      fetchCreatorPoolMetadata(alpha.Pool, viem.zeroAddress),
    ).rejects.toThrow('identity is unavailable');
    const requests = chain.requests;
    for (const coin of [alpha, beta, unknown])
      expect(requests).toContain(`/metadata/${coin.Token.toLowerCase()}`);
    expect(
      requests.some((path) =>
        path.startsWith(`/v2/pools/${alpha.Pool.toLowerCase()}`),
      ),
    ).toBe(true);
    expect(
      requests.some((path) =>
        path.startsWith(`/v2/pools/${beta.Pool.toLowerCase()}`),
      ),
    ).toBe(true);
    expect(
      requests.some((path) =>
        chain!.coins.some((coin) =>
          path.startsWith(`/v2/pools/${coin.Token.toLowerCase()}`),
        ),
      ),
    ).toBe(false);
    expect(chain.errors).toEqual([]);
    expect(chain.provider.errors).toEqual([]);
  } catch (error) {
    failure = error;
  } finally {
    const cleanupErrors: unknown[] = [];
    async function cleanup(action: () => void | Promise<void>) {
      try {
        await action();
      } catch (error) {
        cleanupErrors.push(error);
      }
    }
    if (closeView) await cleanup(closeView);
    if (closeDom) await cleanup(closeDom);
    if (originalWindow)
      Object.defineProperty(globalThis, 'window', originalWindow);
    else Reflect.deleteProperty(globalThis, 'window');
    if (originalDocument)
      Object.defineProperty(globalThis, 'document', originalDocument);
    else Reflect.deleteProperty(globalThis, 'document');
    for (const key of Object.keys(process.env))
      if (!(key in originalEnvironment)) delete process.env[key];
    Object.assign(process.env, originalEnvironment);
    if (chain) await cleanup(() => chain!.close());
    if (cleanupErrors.length)
      failure = new AggregateError(
        failure ? [failure, ...cleanupErrors] : cleanupErrors,
        'Creator discovery proof or dependency shutdown failed',
      );
  }
  if (failure) throw failure;
}

test(
  metadata_token_identity_creator_discovery_through_gateway,
  metadata_token_identity_creator_discovery_through_gateway,
  900000,
);
