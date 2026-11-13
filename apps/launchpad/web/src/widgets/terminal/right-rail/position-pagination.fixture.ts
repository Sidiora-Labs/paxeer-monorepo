import type { ConnectedPortfolioV3Response } from '@kindle/sdk-v2';

export const positionToken = '0x' + 'bb'.repeat(20);
export const positionPool = '0x' + 'cc'.repeat(20);
export const positionSnapshots = [
  '00000000-0000-4000-8000-000000000001',
  '00000000-0000-4000-8000-000000000002',
] as const;
const watermark = {
  chainId: '125', height: '4200', blockHash: '0x' + '6'.repeat(64),
  projectionRevision: '7', integrity: 'healthy', observedAt: '2026-10-13T12:00:00.000Z',
};
type Position = ConnectedPortfolioV3Response['data']['positions'][number];

function position(wallet: string, tokenAddress: string, generation: number): Position {
  return {
    positionId: `125:${wallet}:${tokenAddress}`, chainId: '125', wallet, tokenAddress,
    balanceRaw: '2000000000000000000', knownQuantityRaw: '2000000000000000000', unknownQuantityRaw: '0',
    basisCoverage: { numerator: '2000000000000000000', denominator: '2000000000000000000' },
    basisQuality: 'complete', realizedBasisQuality: 'complete', pricingQuality: 'priced',
    knownRemainingCostUsdlRaw: '2500000', remainingCostUsdlRaw: '2500000',
    knownRealizedSubtotalUsdlRaw: '400000', realizedPnlRaw: '400000', unrealizedPnlRaw: generation === 0 ? '500000' : '1500000',
    markPrice: {
      numerator: generation === 0 ? '3' : '4', denominator: '2',
      baseAsset: { kind: 'erc20', chainId: '125', address: tokenAddress, currencyCode: null, decimals: 18 },
      quoteAsset: { kind: 'erc20', chainId: '125', address: '0x85FcD13735F4309833A503EE804ea32395851479', currencyCode: 'USDL', decimals: 6 },
      source: { kind: 'post_swap_spot', amountBasis: 'effective_reserves', sourceEventId: 'swap-4200', observedAt: watermark.observedAt, providerId: null, sourceReference: null,
        position: { chainId: '125', height: '4200', transactionIndex: '0', logIndex: '1', blockHash: watermark.blockHash, transactionHash: '0x' + '7'.repeat(64) } },
    },
    sourceWatermark: watermark,
  };
}

/** Immutable v3 portfolio pages, served through the real account transport in the repair suite. */
export function positionPage(wallet: string, generation: number, continuation: boolean, nextAfterMatch = false): ConnectedPortfolioV3Response {
  const positions = continuation
    ? [position(wallet, positionToken, generation)]
    : Array.from({ length: 50 }, (_, index) => position(wallet, `0x${(index + 1).toString(16).padStart(40, '0')}`, generation));
  return {
    algorithm: 'proportional_floor_v1', decimals: 6, quoteAsset: 'USDL', version: 3, positionErrors: {},
    data: {
      positions,
      next: continuation ? (nextAfterMatch ? 'unneeded-page-three' : undefined) : `snapshot-${generation}-page-two`,
      summary: { wallet, basisQuality: 'complete', completeValueUsdlRaw: '153000000', pricedSubtotalUsdlRaw: '153000000',
        knownRealizedSubtotalUsdlRaw: '20400000', realizedPnlRaw: '20400000', positionCount: 51, pricingQuality: 'priced',
        snapshotId: positionSnapshots[generation], unpricedAssetCount: 0, watermark },
    },
  };
}
