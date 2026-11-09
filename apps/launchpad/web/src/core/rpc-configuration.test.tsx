// @vitest-environment jsdom
import { channel } from 'node:diagnostics_channel';
import { createElement } from 'react';
import { renderToStaticMarkup } from 'react-dom/server';
import { afterEach, expect, test as registerTest, vi } from 'vitest';
import { BackendConfigurationError } from '@/ui/shared/backend-configuration-error';
import { plainCopy } from '@/core/user-facing-error';

const RPC_VARIABLES = [
  'NEXT_PUBLIC_RPC_URL',
  'NEXT_PUBLIC_RPC_URL_FALLBACK',
  'NEXT_PUBLIC_PAXEER_RPC_URL',
] as const;

const INVALID_RPC_VALUES = [
  '',
  'malformed',
  'https://rpc.example.test/rpc',
  'https://rpc.example.test/evm',
  'https://rpc.example.test/rpc/..',
  'http://rpc.example.test',
  'http://127.0.0.1:18080',
  'https://operator:secret@rpc.example.test',
  'https://rpc.example.test?key=secret',
  'https://rpc.example.test#secret',
  'wss://rpc.example.test',
  'https://node.hyperpaxeer.com',
  'https://node.hyperpaxeer.com.',
  'https://public-mainnet.rpcpaxeer.online',
] as const;

function configureRpc(overrides: Partial<Record<(typeof RPC_VARIABLES)[number], string>>) {
  vi.stubEnv('NODE_ENV', 'production');
  for (const variable of RPC_VARIABLES) vi.stubEnv(variable, overrides[variable]);
  vi.stubEnv('NEXT_PUBLIC_DATA_API_URL', 'https://data.example.test');
  vi.stubEnv('NEXT_PUBLIC_GATEWAY_URL', 'https://gateway.example.test');
  vi.stubEnv('NEXT_PUBLIC_METADATA_API_URL', 'https://metadata.example.test');
  vi.stubEnv('NEXT_PUBLIC_PNL_API_URL', 'https://pnl.example.test');
  for (const variable of [
    'NEXT_PUBLIC_STATS_API_URL',
    'NEXT_PUBLIC_LIVESTREAM_API_URL',
    'NEXT_PUBLIC_STREAM_ORIGIN',
    'NEXT_PUBLIC_PRIVATE_API_ORIGIN',
    'NEXT_PUBLIC_PRIVATE_STREAM_ORIGIN',
    'NEXT_PUBLIC_WALLET_IFRAME_ORIGIN',
    'NEXT_PUBLIC_WALLETCONNECT_PROJECT_ID',
    'NEXT_PUBLIC_SUPABASE_URL',
    'NEXT_PUBLIC_SUPABASE_PUBLISHABLE_KEY',
  ]) {
    vi.stubEnv(variable, undefined);
  }
  vi.resetModules();
}

async function importRpcConfiguration() {
  const rpc = await import('./rpc');
  const origins = await import('./backend-origins');
  const wagmi = await import('./wagmi-config');
  const wallet = await import('./wallet-sdk/client');
  return { rpc, origins, wagmi, wallet };
}

afterEach(() => {
  vi.unstubAllEnvs();
  vi.resetModules();
  window.localStorage.clear();
});

async function rpcColdImportReportsInvalidOverrides() {
  const requests: unknown[] = [];
  const recordRequest = (request: unknown) => {
    requests.push(request);
  };
  const channels = [channel('undici:request:create'), channel('http.client.request.start')];
  for (const requestsChannel of channels) requestsChannel.subscribe(recordRequest);
  try {
    for (const variable of RPC_VARIABLES) {
      for (const invalid of INVALID_RPC_VALUES) {
        configureRpc({ [variable]: invalid });
        vi.stubEnv('NEXT_PUBLIC_SUPABASE_URL', 'https://auth.example.test');
        vi.stubEnv('NEXT_PUBLIC_SUPABASE_PUBLISHABLE_KEY', 'sb_publishable_example');
        vi.resetModules();
        const { rpc, origins, wagmi, wallet } = await importRpcConfiguration();
        expect(rpc.getPaxeerRpcUrls).toBeTypeOf('function');
        expect(wagmi.createWagmiConfig).toBeTypeOf('function');
        expect(wallet.getPublicClient).toBeTypeOf('function');
        const problems = origins.backendConfigurationProblems();
        expect(problems, `${variable}=${invalid}`).toHaveLength(1);
        expect(problems[0]).toBeInstanceOf(origins.BackendOriginError);
        expect(problems[0].variable).toBe(variable);
        expect(problems[0].message).toContain(new rpc.RpcConfigurationError(variable).message);
        expect(origins.backendConfiguration()).toEqual({ origins: null, problems });
        const html = renderToStaticMarkup(createElement(BackendConfigurationError, { problems }));
        expect(html).toContain('role="alert"');
        expect(html).toContain('data-state="backend-configuration-error"');
        expect(html).toContain(plainCopy.configuration);
        expect(html).not.toContain(variable);
        expect(window.localStorage.length).toBe(0);
        expect(requests).toEqual([]);
      }
    }
  } finally {
    for (const requestsChannel of channels) requestsChannel.unsubscribe(recordRequest);
  }
}

async function rpcColdImportPreservesValidatedSelection() {
  const primary = 'https://rpc-primary.example.test';
  const secondary = 'https://rpc-secondary.example.test';
  const legacy = 'https://rpc-legacy.example.test';
  for (const { overrides, expected } of [
    { overrides: {}, expected: ['https://api5.mainnet-beta.paxeer.network'] },
    { overrides: { NEXT_PUBLIC_RPC_URL: `${primary}/` }, expected: [primary] },
    { overrides: { NEXT_PUBLIC_RPC_URL_FALLBACK: `${secondary}/` }, expected: [secondary] },
    { overrides: { NEXT_PUBLIC_PAXEER_RPC_URL: `${legacy}/` }, expected: [legacy] },
    {
      overrides: {
        NEXT_PUBLIC_RPC_URL: `${primary}/`,
        NEXT_PUBLIC_RPC_URL_FALLBACK: secondary,
        NEXT_PUBLIC_PAXEER_RPC_URL: legacy,
      },
      expected: [primary, secondary, legacy],
    },
    {
      overrides: {
        NEXT_PUBLIC_RPC_URL: primary,
        NEXT_PUBLIC_RPC_URL_FALLBACK: `${primary}/`,
        NEXT_PUBLIC_PAXEER_RPC_URL: secondary,
      },
      expected: [primary, secondary],
    },
    {
      overrides: {
        NEXT_PUBLIC_RPC_URL: primary,
        NEXT_PUBLIC_RPC_URL_FALLBACK: secondary,
        NEXT_PUBLIC_PAXEER_RPC_URL: `${primary}/`,
      },
      expected: [primary, secondary],
    },
    {
      overrides: {
        NEXT_PUBLIC_RPC_URL: primary,
        NEXT_PUBLIC_RPC_URL_FALLBACK: `${primary}/`,
        NEXT_PUBLIC_PAXEER_RPC_URL: primary,
      },
      expected: [primary],
    },
  ]) {
    configureRpc(overrides);
    const { rpc, origins, wagmi, wallet } = await importRpcConfiguration();
    expect(rpc.getPaxeerRpcUrls()).toEqual(expected);
    expect(rpc.getPaxeerRpcUrl()).toBe(expected[0]);
    expect(wagmi.getPaxeerRpcUrl()).toBe(expected[0]);
    expect(origins.backendConfigurationProblems()).toEqual([]);
    expect(origins.backendConfiguration().origins).not.toBeNull();
    expect(wagmi.paxeer.rpcUrls.default.http).toEqual([expected[0]]);
    expect(wallet.paxeerWalletChain.rpcUrls.default.http).toEqual([expected[0]]);
    const config = wagmi.createWagmiConfig();
    expect(config.chains[0].rpcUrls.default.http).toEqual([expected[0]]);
    expect(wallet.getPublicClient().transport.url).toBe(expected[0]);
    config._internal.mipd?.destroy();
  }

  for (const variable of RPC_VARIABLES) {
    for (const invalid of INVALID_RPC_VALUES) {
      configureRpc({
        NEXT_PUBLIC_RPC_URL: primary,
        NEXT_PUBLIC_RPC_URL_FALLBACK: secondary,
        NEXT_PUBLIC_PAXEER_RPC_URL: legacy,
        [variable]: invalid,
      });
      const { rpc, wagmi, wallet } = await importRpcConfiguration();
      for (const consume of [
        rpc.getPaxeerRpcUrls,
        rpc.getPaxeerRpcUrl,
        wagmi.getPaxeerRpcUrl,
        wagmi.createWagmiConfig,
        wagmi.getWagmiConfig,
        () => wagmi.paxeer.rpcUrls.default.http,
        () => wallet.paxeerWalletChain.rpcUrls.default.http,
        () => wallet.getPublicClient(),
      ]) {
        expect(consume, `${variable}=${invalid}`).toThrow(rpc.RpcConfigurationError);
        expect(consume).toThrow(expect.objectContaining({ variable }));
      }
    }
  }
}

function test(testFunction: () => Promise<void>, timeout?: number) {
  registerTest(testFunction.name, testFunction, timeout);
}

test(rpcColdImportReportsInvalidOverrides, 30_000);
test(rpcColdImportPreservesValidatedSelection);
