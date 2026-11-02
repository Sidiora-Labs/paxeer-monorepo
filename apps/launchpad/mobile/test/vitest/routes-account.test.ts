import { createServer } from 'node:http';
import { expect, test } from 'vitest';
import { SdkClient } from '../../../../../sdk/launchpad/packages/sdk/src/index.ts';
import {
  accountRoutes,
  type PortfolioResponse,
} from '../../src/runtime/routes/account.ts';

async function portfolioReadUsesV2Route() {
  const wallet = `0x${'ab'.repeat(20)}`;
  const tokenAddress = `0x${'cd'.repeat(20)}`;
  const id = '123e4567-e89b-42d3-a456-426614174000';
  const time = '2026-01-01T00:00:00.000Z';
  const watermark = {
    blockHash: `0x${'ef'.repeat(32)}`,
    chainId: '125',
    height: '100',
    integrity: 'healthy',
    observedAt: time,
    projectionRevision: '2',
  } as const;
  const document = {
    generatedAt: time,
    nextCursor: null,
    quality: 'partial',
    requestId: id,
    revision: '2',
    watermark,
    data: {
      summary: {
        basisQuality: 'partial',
        completeValueUsdlRaw: null,
        knownRealizedSubtotalUsdlRaw: '-12345678901234567890',
        positionCount: 1,
        pricedSubtotalUsdlRaw: '900719925474099312345',
        pricingQuality: 'unpriced',
        realizedPnlRaw: null,
        snapshotId: id,
        unpricedAssetCount: 1,
        wallet,
        watermark,
      },
      items: [
        {
          balanceRaw: '900719925474099312345',
          basisCoverage: { numerator: '1', denominator: '2' },
          basisQuality: 'partial',
          chainId: '125',
          knownQuantityRaw: '1',
          knownRealizedSubtotalUsdlRaw: '-12345678901234567890',
          knownRemainingCostUsdlRaw: '100',
          markPrice: null,
          positionId: id,
          pricingQuality: 'unpriced',
          realizedBasisQuality: 'partial',
          realizedPnlRaw: null,
          remainingCostUsdlRaw: null,
          sourceWatermark: watermark,
          tokenAddress,
          unknownQuantityRaw: '900719925474099312344',
          unrealizedPnlRaw: null,
          wallet,
        },
      ],
    },
  } satisfies PortfolioResponse;
  const requests: { method?: string; path?: string; body: string }[] = [];
  const credentials: (RequestCredentials | undefined)[] = [];
  const server = createServer((request, response) => {
    let body = '';
    request.setEncoding('utf8');
    request.on('data', (chunk: string) => {
      body += chunk;
    });
    request.on('end', () => {
      requests.push({ method: request.method, path: request.url, body });
      if (request.method !== 'GET' || request.url !== '/v2/me/portfolio') {
        response.writeHead(404);
        response.end();
        return;
      }
      response.writeHead(200, { 'Content-Type': 'application/json' });
      response.end(JSON.stringify(document));
    });
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
    const config = {
      apiOrigin: origin,
      identityOrigin: origin,
      mediaOrigin: origin,
      streamOrigin,
      approvedStreamOrigins: [streamOrigin],
      production: false,
    };
    const sdk = new SdkClient(
      {
        ...config,
        session: {
          mode: 'cookie',
          applicationOrigin: origin,
          csrfToken: () => '',
        },
      },
      (input, init) => {
        credentials.push(init?.credentials);
        return fetch(input, init);
      },
    );
    const result = await accountRoutes.portfolio(
      sdk,
      new AbortController().signal,
    );
    expect(result).toEqual(document);
    expect(result.data.summary).toMatchObject({
      pricingQuality: 'unpriced',
      basisQuality: 'partial',
      pricedSubtotalUsdlRaw: '900719925474099312345',
      realizedPnlRaw: null,
      unpricedAssetCount: 1,
    });
    expect(result.data.items[0]).toMatchObject({
      positionId: id,
      tokenAddress,
      balanceRaw: '900719925474099312345',
      pricingQuality: 'unpriced',
      basisQuality: 'partial',
      unrealizedPnlRaw: null,
    });
    expect(credentials).toEqual(['same-origin']);
    await expect(
      accountRoutes.portfolio(
        new SdkClient(config),
        new AbortController().signal,
      ),
    ).rejects.toThrow('Session required');
    await expect(
      accountRoutes.portfolio(sdk, AbortSignal.abort()),
    ).rejects.toThrow();
    expect(requests).toEqual([
      { method: 'GET', path: '/v2/me/portfolio', body: '' },
    ]);
  } finally {
    await new Promise<void>((resolve, reject) => {
      server.close((error) => (error ? reject(error) : resolve()));
      server.closeAllConnections();
    });
  }
}

test(portfolioReadUsesV2Route, portfolioReadUsesV2Route);
