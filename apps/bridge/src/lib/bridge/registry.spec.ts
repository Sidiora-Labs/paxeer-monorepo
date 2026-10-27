import { readFileSync } from 'node:fs';
import { resolve } from 'node:path';

import { expect, test } from 'vitest';

import * as registry from './chains';

const BRIDGE_CHAINS = registry.BRIDGE_CHAINS;
const bridgeChainByKey = registry.bridgeChainByKey;

type ChainConfig = {
  chain_id: number;
  finality_depth: number;
};

function evmChainsMatchTheCommittedConfigs() {
  const evmChains = BRIDGE_CHAINS.filter((chain) => chain.kind === 'evm');

  expect(evmChains).toHaveLength(8);
  for (const chain of evmChains) {
    const config = JSON.parse(readFileSync(
      resolve(__dirname, '../../../../../contracts/bridge/evm/chains', chain.key, 'config.json'),
      'utf8',
    )) as ChainConfig;

    expect(chain.chainId, `${ chain.key } chain id`).toBe(BigInt(config.chain_id));
    expect(chain.finalityDepth, `${ chain.key } finality depth`).toBe(config.finality_depth);
  }
}

function solanaChainMatchesTheCommittedConfig() {
  const config = JSON.parse(readFileSync(
    resolve(__dirname, '../../../../../contracts/bridge/solana/chains/solana/config.json'),
    'utf8',
  )) as ChainConfig;
  const chain = bridgeChainByKey('solana');

  expect(config.chain_id).toBe(91600046870081);
  expect(config.finality_depth).toBe(32);
  expect(chain).toMatchObject({
    key: 'solana',
    kind: 'solana',
    chainId: BigInt(config.chain_id),
    finalityDepth: config.finality_depth,
  });
}

test(evmChainsMatchTheCommittedConfigs.name, evmChainsMatchTheCommittedConfigs);
test(solanaChainMatchesTheCommittedConfig.name, solanaChainMatchesTheCommittedConfig);
