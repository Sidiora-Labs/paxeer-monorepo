import type { Metadata } from 'lib/metadata/types';

import config from 'configs/app';
import generate from 'lib/metadata/generate';
import { parseChainsResponse } from 'ui/paxeerX/chains/model';
import { expect, test } from 'vitest';

// Exercise the real runtime boundary while nextjs/nextjs-routes.d.ts awaits its owning domain's regeneration.
function chainMetadata(
  route: { pathname: '/chains'; query?: never } | { pathname: '/chains/[id]'; query: { id: string } },
  apiData: { chain_name: string } | null = null,
): Metadata {
  return Reflect.apply(generate, undefined, [ route, apiData ]);
}

test('chainDirectoryMetadataIncludesNetwork', function chainDirectoryMetadataIncludesNetwork() {
  const metadata = chainMetadata({ pathname: '/chains' });
  const postfix = config.meta.promoteBlockscoutInTitle ? ' | Blockscout' : '';

  expect(config.chain.name).toBeTruthy();
  expect(metadata.title).toBe(`${ config.chain.name } chains${ postfix }`);
  expect(metadata.opengraph.title).toBe(metadata.title);
  expect(metadata.title).not.toContain('undefined');
  expect(metadata.title).not.toMatch(/%\w+%/);
});

test('chainDetailMetadataIncludesNetworkAndRouteId', function chainDetailMetadataIncludesNetworkAndRouteId() {
  const postfix = config.meta.promoteBlockscoutInTitle ? ' | Blockscout' : '';

  for (const id of [ '1', '4294967295' ]) {
    const metadata = chainMetadata({ pathname: '/chains/[id]', query: { id } });

    expect(metadata.title).toBe(`${ config.chain.name } chain ${ id }${ postfix }`);
    expect(metadata.opengraph.title).toBe(metadata.title);
    expect(metadata.title).not.toContain('undefined');
    expect(metadata.title).not.toMatch(/%\w+%/);
  }
});

test('chainDetailMetadataUsesSelectedChainName', function chainDetailMetadataUsesSelectedChainName() {
  // Committed /v1/chains response fixture matching the production chain codec's existing model/detail fixtures.
  const chains = parseChainsResponse(JSON.parse(`{
    "chains": [{
      "id": 1, "name": "demo", "operator": "0x1111111111111111111111111111111111111111",
      "status": "active", "chain_id": 125000001, "created_height": 42,
      "bond": "100000000000000000000", "gas_allowance": 500000, "byte_allowance": 65536, "fee_share_bps": 100
    }, {
      "id": 4, "name": "four", "operator": "0x1111111111111111111111111111111111111111",
      "status": "active", "chain_id": 125000004, "created_height": 42,
      "bond": "100000000000000000000", "gas_allowance": 500000, "byte_allowance": 65536, "fee_share_bps": 100
    }]
  }`));
  const postfix = config.meta.promoteBlockscoutInTitle ? ' | Blockscout' : '';

  for (const chain of chains) {
    const metadata = chainMetadata(
      { pathname: '/chains/[id]', query: { id: String(chain.id) } },
      { chain_name: chain.name },
    );

    expect(metadata.title).toBe(`${ config.chain.name } chain ${ chain.name }${ postfix }`);
    expect(metadata.opengraph.title).toBe(metadata.title);
    expect(metadata.title).not.toContain('undefined');
    expect(metadata.title).not.toMatch(/%\w+%/);
  }
});
