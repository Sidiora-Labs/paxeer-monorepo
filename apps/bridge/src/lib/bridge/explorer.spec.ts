import type { Address, Hex } from 'viem';
import { expect, test } from 'vitest';

import { bridgeConfig, readConfig } from './config';
import { paxeerAddressUrl, paxeerAddressUrlFrom, paxeerTxUrl, paxeerTxUrlFrom } from './explorer';

const hash: Hex = '0x0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef';
const address: Address = '0x1234567890123456789012345678901234567890';

function paxeerLinksAreUndefinedWithoutAConfiguredExplorer() {
  const config = readConfig({});

  expect(config.paxeerExplorerUrl).toBeUndefined();
  expect(paxeerTxUrlFrom(undefined, hash)).toBeUndefined();
  expect(paxeerAddressUrlFrom(undefined, address)).toBeUndefined();
  expect(paxeerTxUrlFrom(config.paxeerExplorerUrl, hash)).toBeUndefined();
  expect(paxeerAddressUrlFrom(config.paxeerExplorerUrl, address)).toBeUndefined();
}

function paxeerLinksUseTheConfiguredExplorer() {
  const base = 'https://explorer.example';
  const config = readConfig({ NEXT_PUBLIC_PAXEER_EXPLORER_URL: `${ base }///` });

  expect(config.paxeerExplorerUrl).toBe(base);
  for (const explorer of [ base, `${ base }/`, `${ base }///`, config.paxeerExplorerUrl ]) {
    expect(paxeerTxUrlFrom(explorer, hash)).toBe(`${ base }/tx/${ hash }`);
    expect(paxeerAddressUrlFrom(explorer, address)).toBe(`${ base }/address/${ address }`);
  }
  expect(paxeerTxUrl(hash)).toBe(paxeerTxUrlFrom(bridgeConfig.paxeerExplorerUrl, hash));
  expect(paxeerAddressUrl(address)).toBe(paxeerAddressUrlFrom(bridgeConfig.paxeerExplorerUrl, address));
}

test(paxeerLinksAreUndefinedWithoutAConfiguredExplorer.name, paxeerLinksAreUndefinedWithoutAConfiguredExplorer);
test(paxeerLinksUseTheConfiguredExplorer.name, paxeerLinksUseTheConfiguredExplorer);
