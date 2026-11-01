import { createServer } from 'node:http';
import { expect, test } from 'vitest';
import {
  SdkClient,
  type V2ContractTypes,
} from '../../../../../sdk/launchpad/packages/sdk/src/index.ts';
import { marketRoutes } from '../../src/runtime/routes/market.ts';

async function marketReadsUseV2PoolRoutes() {
  const address = `0x${'Ab'.repeat(20)}`;
  const tokenAddress = `0x${'12'.repeat(20)}`;
  const quoteAssetAddress = `0x${'34'.repeat(20)}`;
  const hash = `0x${'Cd'.repeat(32)}`;
  const time = '2026-01-01T00:00:00.000Z';
  const position = {
    blockHash: hash,
    chainId: '125',
    height: '123',
    logIndex: '0',
    transactionHash: hash,
    transactionIndex: '0',
  } satisfies V2ContractTypes['ChainPosition'];
  const watermark = {
    blockHash: hash,
    chainId: '125',
    height: '123',
    integrity: 'healthy',
    observedAt: time,
    projectionRevision: '7',
  } satisfies V2ContractTypes['Watermark'];
  const envelope = {
    generatedAt: time,
    nextCursor: null,
    quality: 'partial',
    requestId: '123e4567-e89b-42d3-a456-426614174000',
    revision: '7',
    watermark,
  } as const;
  const pool = {
    chainId: '125',
    createdAt: time,
    createdPosition: position,
    creator: address,
    effectivePricingUsdlReserveRaw: '3000000',
    feeBps: 30,
    poolAddress: address,
    quoteAssetAddress,
    quoteAssetDecimals: 6,
    realUsdlBalanceRaw: '2000000',
    tokenAddress,
    tokenReserveRaw: '4000000000000000000',
    virtualUsdlReserveRaw: '1000000',
  } satisfies V2ContractTypes['Pool'];
  const list = {
    ...envelope,
    nextCursor: 'next-page',
    data: { items: [pool] },
  } satisfies V2ContractTypes['PoolListResponse'];
  const detail = {
    ...envelope,
    data: {
      pool,
      token: {
        address: tokenAddress,
        decimals: null,
        decimalsQuality: 'unavailable',
        logoAssetId: null,
        metadataRevision: '7',
        name: 'Market token',
        sourcePosition: position,
        supplyQuality: 'unavailable',
        symbol: 'MARKET',
        totalSupplyRaw: null,
      },
      stats: {
        buys24h: '2',
        changeBaselineQuality: 'unavailable',
        circulatingMarketCapUsdlRaw: null,
        holderCount: null,
        indicativeTvlUsdlRaw: null,
        poolAddress: address,
        priceChangeBps24h: null,
        realUsdlBalanceRaw: pool.realUsdlBalanceRaw,
        sells24h: '1',
        sourceUpperPosition: position,
        supplyValuationQuality: 'unavailable',
        totalSupplyValuationUsdlRaw: null,
        valuationChangeUsdlRaw: null,
        valuationMethod: null,
        volume24hRaw: '5000000',
        watermark,
        windowEnd: time,
        windowStart: '2025-12-31T00:00:00.000Z',
      },
    },
  } satisfies V2ContractTypes['PoolDetailResponse'];
  const results = {
    ...envelope,
    data: {
      items: [
        {
          id: address,
          kind: 'pool',
          label: 'Market & token/USDL',
          poolAddress: address,
          score: '1',
          tokenAddress,
          wallet: null,
        },
      ],
    },
  } satisfies V2ContractTypes['SearchResponse'];
  const query = 'Market & token/USDL';
  const expectedPaths = [
    '/v2/pools?sort=new&limit=50',
    `/v2/pools/${address}`,
    '/v2/search?q=Market+%26+token%2FUSDL&type=pool&limit=25',
  ];
  const responses = [list, detail, results];
  const observed: {
    method: string | undefined;
    path: string | undefined;
    accept: string | undefined;
    authorization: string | undefined;
  }[] = [];
  const server = createServer((request, response) => {
    observed.push({
      method: request.method,
      path: request.url,
      accept: request.headers.accept,
      authorization: request.headers.authorization,
    });
    const index = expectedPaths.indexOf(request.url ?? '');
    if (request.method !== 'GET' || index < 0) {
      response.writeHead(404);
      response.end();
      return;
    }
    response.writeHead(200, { 'Content-Type': 'application/json' });
    response.end(JSON.stringify(responses[index]));
  });
  await new Promise<void>((resolve, reject) => {
    server.once('error', reject);
    server.listen(0, '127.0.0.1', resolve);
  });
  try {
    const bound = server.address();
    if (!bound || typeof bound === 'string')
      throw new Error('Loopback port missing');
    const origin = `http://127.0.0.1:${bound.port}`;
    const streamOrigin = `ws://127.0.0.1:${bound.port}`;
    const sdk = new SdkClient({
      apiOrigin: origin,
      identityOrigin: origin,
      mediaOrigin: origin,
      streamOrigin,
      approvedStreamOrigins: [streamOrigin],
      production: false,
    });
    const signal = new AbortController().signal;
    const pools = await marketRoutes.pools(sdk, signal);
    const token = await marketRoutes.pool(sdk, address, signal);
    const search = await marketRoutes.search(sdk, query, signal);
    expect(pools).toEqual(list);
    expect(token).toEqual(detail);
    expect(search).toEqual(results);
    expect(pools.data.items[0].poolAddress).toBe(address);
    expect(pools.data.items[0].tokenAddress).toBe(tokenAddress);
    expect(token.data.token).toMatchObject({
      address: tokenAddress,
      name: 'Market token',
      symbol: 'MARKET',
      decimals: null,
    });
    expect(token.data.pool.realUsdlBalanceRaw).toBe('2000000');
    expect(search.data.items[0]).toMatchObject({
      id: address,
      label: 'Market & token/USDL',
      poolAddress: address,
    });
    expect(observed).toEqual(
      expectedPaths.map((path) => ({
        method: 'GET',
        path,
        accept: 'application/json',
        authorization: undefined,
      })),
    );
    expect(() => marketRoutes.pool(sdk, '../invalid', signal)).toThrow(
      'Invalid wallet or pool address',
    );
    await expect(marketRoutes.search(sdk, 'x', signal)).rejects.toThrow(
      'Invalid IF_SEARCH query',
    );
    await expect(
      marketRoutes.search(sdk, 'x'.repeat(257), signal),
    ).rejects.toThrow('Invalid IF_SEARCH query');
    const cancelled = AbortSignal.abort();
    await expect(marketRoutes.pools(sdk, cancelled)).rejects.toThrow();
    await expect(marketRoutes.pool(sdk, address, cancelled)).rejects.toThrow();
    await expect(marketRoutes.search(sdk, query, cancelled)).rejects.toThrow();
    expect(observed).toHaveLength(3);
  } finally {
    await new Promise<void>((resolve, reject) => {
      server.close((error) => (error ? reject(error) : resolve()));
      server.closeAllConnections();
    });
  }
}

test(marketReadsUseV2PoolRoutes, marketReadsUseV2PoolRoutes);
