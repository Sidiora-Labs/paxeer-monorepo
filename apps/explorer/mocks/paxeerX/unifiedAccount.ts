import type { PaxeerXAsset, PaxeerXUnifiedAccount } from 'types/api/paxeerX';

export const evmAddress = '0xd789a607CEac2f0E14867de4EB15b15C9FFB5859';
export const paxAddress = 'pax1005qwm6w5jj26zq8tsjs3eyp6my5d5fthrlqk3';
export const did = 'did:layerx:9f2c1b4d5e6a7b8c9d0e1f2a3b4c5d6e7f8091a2b3c4d5e6f708192a3b4c5d6e';
export const kernelAccount = 'agent:did:layerx:9f2c1b4d5e6a7b8c9d0e1f2a3b4c5d6e7f8091a2b3c4d5e6f708192a3b4c5d6e:main';
export const kernelAccountHash = '0x9f2c1b4d5e6a7b8c9d0e1f2a3b4c5d6e7f8091a2b3c4d5e6f708192a3b4c5d6e';

// The coin balance is reported under the `native` asset id, denominated in the coin symbol.
export const nativeAsset: PaxeerXAsset = {
  id: 'native',
  denom: 'HPX',
  symbol: 'HPX',
  decimals: 18,
};

// A token balance is reported under the checksummed contract address, with the token's own metadata.
export const tokenAsset: PaxeerXAsset = {
  id: '0x2170Ed0880ac9A755fd29B2688956BD959F933F8',
  denom: 'USDX',
  symbol: 'USDX',
  decimals: 6,
};

export const unifiedAccount: PaxeerXUnifiedAccount = {
  next_page_params: null,
  page_cursor: 'fixture-page',
  first_page_cursor: 'fixture-page',
  page_number: 1,
  activity_total: 2,
  identities: {
    evm: evmAddress,
    pax: paxAddress,
    did,
  },
  balances: [
    {
      asset: tokenAsset,
      total: '2500000',
    },
    {
      asset: nativeAsset,
      total: '1000000000000000000',
    },
  ],
  activity: [
    {
      kind: 'token_transfer',
      hash: '0x1f0e9c7a3b5d2e4f6081a2b3c4d5e6f708192a3b4c5d6e7f8091a2b3c4d5e6f7',
      block_number: 1_284_912,
      ordinal: 0,
      status: 'sealed',
      timestamp: '2024-04-02T11:03:11.000000Z',
      asset: tokenAsset,
      amount: '2500000',
      counterparty: '0x8Ba1f109551bD432803012645Ac136ddd64DBA72',
    },
    {
      kind: 'transaction',
      hash: '0x8a7b6c5d4e3f2a1b0c9d8e7f6a5b4c3d2e1f00112233445566778899aabbccdd',
      block_number: 1_284_017,
      ordinal: 0,
      status: 'final',
      timestamp: '2024-04-02T10:12:35.000000Z',
      asset: nativeAsset,
      amount: '1500000000000000000',
      counterparty: null,
    },
  ],
};

export const unifiedAccountEvmOnly: PaxeerXUnifiedAccount = {
  next_page_params: null,
  page_cursor: 'fixture-page',
  first_page_cursor: 'fixture-page',
  page_number: 1,
  activity_total: 0,
  identities: {
    evm: evmAddress,
    pax: null,
    did: null,
  },
  balances: [],
  activity: [],
};
