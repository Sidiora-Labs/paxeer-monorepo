/**
 * Proves: config.ts/readConfig -> chains.ts/buildBridgeChains, explorer.ts/paxeerAddressUrlFrom,paxeerTxUrlFrom -> proof_config.spec.ts.
 * Environment inputs also cross config.ts/bridgeConfig into BRIDGE_CHAINS,
 * bridgeChainByKey/bridgeChainById and paxeerAddressUrl/paxeerTxUrl.
 * Repairs chains.ts/buildBridgeChains trusting a malformed program id and
 * explorer.ts/paxeerAddressUrlFrom,paxeerTxUrlFrom emitting links for unsafe or
 * malformed bases (including paths appended inside a query or fragment).
 * No persistent state: each observation reloads the real modules, without doubles.
 */
import type { Address, Hex } from 'viem';
import { afterEach, expect, test, vi } from 'vitest';

const address: Address = '0x0000000000000000000000000000000000000001';
const hash: Hex = '0x0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef';
const programId = 'So11111111111111111111111111111111111111112';
const environment = {
  NEXT_PUBLIC_PAXEER_RPC_URL: ' https://paxeer.example/rpc/// ',
  NEXT_PUBLIC_PAXEER_EXPLORER_URL: ' https://explorer.example/scan/// ',
  NEXT_PUBLIC_BRIDGE_API_URL: ' https://bridge.example/api/ ',
  NEXT_PUBLIC_BRIDGE_SOLANA_PROGRAM_ID: ` ${ programId } `,
  NEXT_PUBLIC_BRIDGE_RPC_SOLANA: ' https://solana.example/rpc/// ',
  NEXT_PUBLIC_BRIDGE_VAULT_ETHEREUM: ` ${ address } `,
  NEXT_PUBLIC_BRIDGE_VAULT_BASE: undefined,
  NEXT_PUBLIC_BRIDGE_VAULT_ARBITRUM: undefined,
  NEXT_PUBLIC_BRIDGE_VAULT_OPTIMISM: undefined,
  NEXT_PUBLIC_BRIDGE_VAULT_BNB: undefined,
  NEXT_PUBLIC_BRIDGE_VAULT_POLYGON: undefined,
  NEXT_PUBLIC_BRIDGE_VAULT_AVALANCHE: undefined,
  NEXT_PUBLIC_BRIDGE_VAULT_HYPEREVM: undefined,
  NEXT_PUBLIC_BRIDGE_RPC_ETHEREUM: undefined,
  NEXT_PUBLIC_BRIDGE_RPC_BASE: undefined,
  NEXT_PUBLIC_BRIDGE_RPC_ARBITRUM: undefined,
  NEXT_PUBLIC_BRIDGE_RPC_OPTIMISM: undefined,
  NEXT_PUBLIC_BRIDGE_RPC_BNB: undefined,
  NEXT_PUBLIC_BRIDGE_RPC_POLYGON: undefined,
  NEXT_PUBLIC_BRIDGE_RPC_AVALANCHE: undefined,
  NEXT_PUBLIC_BRIDGE_RPC_HYPEREVM: undefined,
};
const originalEnvironment = { ...process.env };

afterEach(() => {
  for (const key of Object.keys(environment)) {
    const value = originalEnvironment[key];
    if (value === undefined) delete process.env[key];
    else process.env[key] = value;
  }
  vi.resetModules();
});

async function loadConfigConsumers(env: Record<string, string | undefined>) {
  for (const key of Object.keys(environment)) {
    const value = env[key];
    if (value === undefined) delete process.env[key];
    else process.env[key] = value;
  }
  vi.resetModules();
  const config = await import('../lib/bridge/config');
  const chains = await import('../lib/bridge/chains');
  const explorer = await import('../lib/bridge/explorer');
  return { ...config, ...chains, ...explorer };
}

let previousConsumer: Awaited<ReturnType<typeof loadConfigConsumers>> | undefined;

async function proofConfigReachesExportedConsumersTwice({ explorerBase }: { explorerBase: string }) {
  const env = { ...environment, NEXT_PUBLIC_PAXEER_EXPLORER_URL: ` ${ explorerBase }/// ` };
  const consumer = await loadConfigConsumers(env);
  const config = consumer.readConfig(env);
  expect(consumer.bridgeConfig).toEqual(config);
  expect(consumer.bridgeConfig).not.toBe(previousConsumer?.bridgeConfig);
  expect(consumer.BRIDGE_CHAINS).not.toBe(previousConsumer?.BRIDGE_CHAINS);
  expect(config).toMatchObject({
    paxeerRpcUrl: 'https://paxeer.example/rpc',
    paxeerExplorerUrl: explorerBase,
    attestationApiUrl: 'https://bridge.example/api',
    solanaProgramId: programId,
    foreignRpcUrls: { solana: 'https://solana.example/rpc' },
    vaults: { ethereum: address, base: undefined },
  });

  const producedChains = consumer.buildBridgeChains(config);
  expect(producedChains).toEqual(consumer.BRIDGE_CHAINS);
  expect(producedChains).not.toBe(consumer.BRIDGE_CHAINS);
  expect(producedChains.map((chain) => chain.key)).toEqual([
    'ethereum', 'base', 'arbitrum', 'optimism', 'bnb', 'polygon', 'avalanche', 'hyperevm', 'solana',
  ]);
  expect(consumer.bridgeChainByKey('ethereum')).toMatchObject({ chainId: 1n, vault: address, state: 'configured' });
  expect(consumer.bridgeChainByKey('base')).toMatchObject({ chainId: 8453n, vault: undefined, state: 'not-deployed' });
  expect(consumer.bridgeChainByKey('solana')).toMatchObject({ chainId: 91600046870081n, state: 'configured' });
  expect(consumer.bridgeChainById(91600046870081n)).toBe(consumer.bridgeChainByKey('solana'));
  expect(consumer.bridgeChainByKey('unknown')).toBeUndefined();
  expect(consumer.bridgeChainById(-1n)).toBeUndefined();
  expect(consumer.paxeer.rpcUrls.default.http).toEqual([ 'https://paxeer.example/rpc' ]);
  expect(consumer.paxeer.blockExplorers?.default.url).toBe(explorerBase);

  expect(consumer.paxeerTxUrlFrom(config.paxeerExplorerUrl, hash)).toBe(`${ explorerBase }/tx/${ hash }`);
  expect(consumer.paxeerAddressUrlFrom(config.paxeerExplorerUrl, address)).toBe(`${ explorerBase }/address/${ address }`);
  expect(consumer.paxeerTxUrl(hash)).toBe(`${ explorerBase }/tx/${ hash }`);
  expect(consumer.paxeerAddressUrl(address)).toBe(`${ explorerBase }/address/${ address }`);
  expect(consumer.txUrlByRouteKey('paxeer', hash)).toBe(`${ explorerBase }/tx/${ hash }`);
  expect(consumer.txUrlByRouteKey('unknown', hash)).toBeUndefined();
  previousConsumer = consumer;
}

async function proofConfigRefusesMalformedAndMissingEnvironment({ env }: { env: Record<string, string | undefined> }) {
  const consumer = await loadConfigConsumers(env);
  expect(consumer.bridgeConfig).toMatchObject({
    paxeerRpcUrl: undefined,
    paxeerExplorerUrl: undefined,
    solanaProgramId: undefined,
    foreignRpcUrls: { solana: undefined },
    vaults: { ethereum: undefined },
  });
  expect(consumer.buildBridgeChains(consumer.readConfig(env))).toEqual(consumer.BRIDGE_CHAINS);
  expect(consumer.bridgeChainByKey('solana')?.state).toBe('awaiting-backend');
  expect(consumer.bridgeChainByKey('ethereum')?.state).toBe('not-deployed');
  expect(consumer.paxeer.rpcUrls.default.http).toEqual([]);
  expect(consumer.paxeer.blockExplorers).toBeUndefined();
  expect(consumer.paxeerAddressUrl(address)).toBeUndefined();
  expect(consumer.paxeerTxUrl(hash)).toBeUndefined();
  expect(consumer.txUrlByRouteKey('paxeer', hash)).toBeUndefined();
}

async function proofConfigConsumersDecodeEachProducerResult() {
  const consumer = await loadConfigConsumers(environment);
  for (let observation = 0; observation < 2; observation += 1) {
    const config = consumer.readConfig(environment);
    const first = consumer.buildBridgeChains(config);
    first.find((chain) => chain.key === 'solana')!.state = 'awaiting-backend';
    expect(consumer.buildBridgeChains(config).find((chain) => chain.key === 'solana')?.state).toBe('configured');
    expect(consumer.buildBridgeChains({ ...config, solanaProgramId: 'abc' })
      .find((chain) => chain.key === 'solana')?.state).toBe('awaiting-backend');
    expect(consumer.buildBridgeChains({ ...config, attestationApiUrl: undefined })
      .find((chain) => chain.key === 'solana')?.state).toBe('awaiting-backend');

    for (const base of [
      'not a URL', 'javascript:alert(1)', 'https://user:password@explorer.example',
      'https://explorer.example/?redirect=', 'https://explorer.example/#fragment',
      'https://explorer.example/?', 'https://explorer.example/#',
    ]) {
      // config.ts/readConfig accepts HTTP URL queries/fragments; the explorer
      // consumers must refuse these bases rather than append a path inside them.
      const produced = consumer.readConfig({ NEXT_PUBLIC_PAXEER_EXPLORER_URL: base });
      if (base.includes('?') || base.includes('#')) expect(produced.paxeerExplorerUrl).toBe(base);
      expect(consumer.paxeerTxUrlFrom(produced.paxeerExplorerUrl, hash)).toBeUndefined();
      expect(consumer.paxeerAddressUrlFrom(produced.paxeerExplorerUrl, address)).toBeUndefined();
      expect(consumer.paxeerTxUrlFrom(base, hash)).toBeUndefined();
      expect(consumer.paxeerAddressUrlFrom(base, address)).toBeUndefined();
    }
    expect(consumer.paxeerTxUrlFrom(`${ config.paxeerExplorerUrl }///`, hash))
      .toBe(`https://explorer.example/scan/tx/${ hash }`);
    expect(consumer.paxeerAddressUrlFrom(`${ config.paxeerExplorerUrl }///`, address))
      .toBe(`https://explorer.example/scan/address/${ address }`);
  }
}

// Each independent module reload gets the runner's unchanged timeout. Both
// observations still execute for each input, and identity checks reject caching.
test.each([
  { explorerBase: 'https://explorer.example/scan', observation: 1 },
  { explorerBase: 'https://explorer.example/scan', observation: 2 },
  { explorerBase: 'http://other.example/explorer', observation: 1 },
  { explorerBase: 'http://other.example/explorer', observation: 2 },
])(`${ proofConfigReachesExportedConsumersTwice.name }: $explorerBase, observation $observation`, proofConfigReachesExportedConsumersTwice);

const refusedEnvironments = [
  { scenario: 'missing', env: {} },
  {
    scenario: 'malformed',
    env: {
      ...environment,
      NEXT_PUBLIC_PAXEER_RPC_URL: 'not a URL',
      NEXT_PUBLIC_PAXEER_EXPLORER_URL: 'javascript:alert(1)',
      NEXT_PUBLIC_BRIDGE_SOLANA_PROGRAM_ID: 'abc',
      NEXT_PUBLIC_BRIDGE_RPC_SOLANA: 'https://user:password@solana.example/rpc',
      NEXT_PUBLIC_BRIDGE_VAULT_ETHEREUM: '0x0000000000000000000000000000000000000000',
    },
  },
];
test.each(refusedEnvironments.flatMap((scenario) => [
  { ...scenario, observation: 1 },
  { ...scenario, observation: 2 },
]))(`${ proofConfigRefusesMalformedAndMissingEnvironment.name }: $scenario, observation $observation`, proofConfigRefusesMalformedAndMissingEnvironment);
test(proofConfigConsumersDecodeEachProducerResult.name, proofConfigConsumersDecodeEachProducerResult);
