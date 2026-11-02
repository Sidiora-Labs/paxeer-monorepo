import { expect, test } from 'vitest';
import { PAXEER_CONFIG } from './constants';
import {
  VIRTUAL_CHAIN_ID_BASE,
  isVirtualChainId,
  networkForChainId,
  rootNetwork,
  virtualNetwork,
} from './networks';

function networks_derive_urls_and_validate_ids() {
  const first = virtualNetwork(125000001, 'First virtual chain');
  expect(first).toEqual({
    chainId: 125000001,
    name: 'First virtual chain',
    kind: 'virtual',
    rpcUrl: `${PAXEER_CONFIG.rpcUrl.replace(/\/+$/, '')}/chain/1`,
    explorerUrl: `${PAXEER_CONFIG.explorerBase.replace(/\/+$/, '')}/chains/1`,
  });
  const maximum = VIRTUAL_CHAIN_ID_BASE + 4294967295;
  expect(isVirtualChainId(VIRTUAL_CHAIN_ID_BASE)).toBe(false);
  expect(isVirtualChainId(VIRTUAL_CHAIN_ID_BASE + 1)).toBe(true);
  expect(isVirtualChainId(maximum)).toBe(true);
  expect(virtualNetwork(maximum, 'Last chain').rpcUrl).toBe(
    `${PAXEER_CONFIG.rpcUrl.replace(/\/+$/, '')}/chain/4294967295`
  );
  for (const id of [
    PAXEER_CONFIG.chainId,
    VIRTUAL_CHAIN_ID_BASE,
    maximum + 1,
    125000001.5,
    NaN,
    Infinity,
    -1,
  ]) {
    expect(isVirtualChainId(id)).toBe(false);
    expect(() => virtualNetwork(id, 'Invalid chain')).toThrow(TypeError);
  }
  expect(() => virtualNetwork(first.chainId, '   ')).toThrow(TypeError);
  expect(() => virtualNetwork(first.chainId, 'x'.repeat(81))).toThrow(TypeError);
  expect(rootNetwork()).toEqual({
    chainId: PAXEER_CONFIG.chainId,
    name: 'Paxeer',
    rpcUrl: PAXEER_CONFIG.rpcUrl,
    explorerUrl: PAXEER_CONFIG.explorerBase,
    kind: 'root',
  });
  expect(networkForChainId(first.chainId, [first])).toEqual(first);
  expect(networkForChainId(first.chainId + 1, [first])).toEqual(rootNetwork());
  expect(networkForChainId(PAXEER_CONFIG.chainId, [first])).toEqual(rootNetwork());
  expect(networkForChainId(null, [first])).toEqual(rootNetwork());
  expect(networkForChainId(VIRTUAL_CHAIN_ID_BASE, [first])).toEqual(rootNetwork());
}

test(networks_derive_urls_and_validate_ids);
