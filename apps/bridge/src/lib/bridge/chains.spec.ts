import { expect, test } from 'vitest';

import { buildBridgeChains } from './chains';
import { readConfig } from './config';

function buildBridgeChainsMarksSolanaConfiguredWithProgramAndApi() {
  const config = readConfig({
    NEXT_PUBLIC_BRIDGE_SOLANA_PROGRAM_ID: 'So11111111111111111111111111111111111111112',
    NEXT_PUBLIC_BRIDGE_API_URL: 'https://bridge.example/api',
    NEXT_PUBLIC_BRIDGE_VAULT_ETHEREUM: '0x0000000000000000000000000000000000000001',
  });
  const chains = buildBridgeChains(config);

  expect(chains.find((chain) => chain.key === 'solana')).toMatchObject({
    kind: 'solana',
    state: 'configured',
  });
  expect(chains.find((chain) => chain.key === 'ethereum')).toMatchObject({
    vault: config.vaults.ethereum,
    state: 'configured',
  });
  expect(buildBridgeChains(readConfig({
    NEXT_PUBLIC_BRIDGE_SOLANA_PROGRAM_ID: config.solanaProgramId,
  })).find((chain) => chain.key === 'solana')).toMatchObject({
    state: 'awaiting-backend',
  });
}

function buildBridgeChainsMarksSolanaAwaitingWithoutAProgramId() {
  const config = readConfig({
    NEXT_PUBLIC_BRIDGE_API_URL: 'https://bridge.example/api',
  });
  const chains = buildBridgeChains(config);

  expect(config.solanaProgramId).toBeUndefined();
  expect(chains.find((chain) => chain.key === 'solana')).toMatchObject({
    kind: 'solana',
    state: 'awaiting-backend',
  });
  for (const chain of chains.filter((chain) => chain.kind === 'evm')) {
    expect(chain.vault).toBeUndefined();
    expect(chain.state).toBe('not-deployed');
  }
}

test(buildBridgeChainsMarksSolanaConfiguredWithProgramAndApi.name, buildBridgeChainsMarksSolanaConfiguredWithProgramAndApi);
test(buildBridgeChainsMarksSolanaAwaitingWithoutAProgramId.name, buildBridgeChainsMarksSolanaAwaitingWithoutAProgramId);
