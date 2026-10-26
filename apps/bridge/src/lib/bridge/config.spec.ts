import { expect, test } from 'vitest';

import { readConfig } from './config';

function readConfigReadsTheSolanaProgramId() {
  const config = readConfig({
    NEXT_PUBLIC_BRIDGE_SOLANA_PROGRAM_ID: 'So11111111111111111111111111111111111111112',
    NEXT_PUBLIC_BRIDGE_RPC_SOLANA: ' https://solana.example/rpc/ ',
  });

  expect(config.solanaProgramId).toBe('So11111111111111111111111111111111111111112');
  expect(config.foreignRpcUrls.solana).toBe('https://solana.example/rpc');
  expect(readConfig({}).solanaProgramId).toBeUndefined();
  expect(readConfig({}).foreignRpcUrls.solana).toBeUndefined();
}

function readConfigRefusesAShortProgramId() {
  const config = readConfig({
    NEXT_PUBLIC_BRIDGE_SOLANA_PROGRAM_ID: '1111111111111111111111111111111',
  });

  expect(config.solanaProgramId).toBeUndefined();
}

function readConfigDropsAZeroVault() {
  const config = readConfig({
    NEXT_PUBLIC_BRIDGE_VAULT_ETHEREUM: '0x0000000000000000000000000000000000000000',
    NEXT_PUBLIC_BRIDGE_VAULT_BASE: '0x0000000000000000000000000000000000000000',
    NEXT_PUBLIC_BRIDGE_VAULT_ARBITRUM: '0x0000000000000000000000000000000000000000',
    NEXT_PUBLIC_BRIDGE_VAULT_OPTIMISM: '0x0000000000000000000000000000000000000000',
    NEXT_PUBLIC_BRIDGE_VAULT_BNB: '0x0000000000000000000000000000000000000000',
    NEXT_PUBLIC_BRIDGE_VAULT_POLYGON: '0x0000000000000000000000000000000000000000',
    NEXT_PUBLIC_BRIDGE_VAULT_AVALANCHE: '0x0000000000000000000000000000000000000000',
    NEXT_PUBLIC_BRIDGE_VAULT_HYPEREVM: ' 0x0000000000000000000000000000000000000000 ',
  });

  for (const vault of Object.values(config.vaults)) {
    expect(vault).toBeUndefined();
  }
  expect(readConfig({
    NEXT_PUBLIC_BRIDGE_VAULT_ETHEREUM: '0x0000000000000000000000000000000000000001',
  }).vaults.ethereum).toBe('0x0000000000000000000000000000000000000001');
}

test(readConfigReadsTheSolanaProgramId);
test(readConfigRefusesAShortProgramId);
test(readConfigDropsAZeroVault);
