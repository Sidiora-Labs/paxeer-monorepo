import React from 'react';
import { renderToString } from 'react-dom/server';
import { QueryClient, QueryObserver } from '@tanstack/react-query';
import { expect, test, vi } from 'vitest';
import { decodeV2Contract, type V2ContractTypes } from '@kindle/sdk';
import { USDL_ADDRESS } from '@/core/network/contracts';
import { usePoolSnapshot, type PoolSnapshot } from '@/core/market/use-pool-snapshot';
import { v2Reads } from '@/core/test-fixtures/market-production/v2-reads';
import { useViewerWallet } from '@/widgets/social/use-follow';
import { usePrivateAccountQuery } from '@/widgets/profile/use-private-account-query';
import CreatorRewardsCard from './creator-rewards-card';

vi.mock('@/core/market/use-pool-snapshot', async original => ({
  ...await original<typeof import('@/core/market/use-pool-snapshot')>(),
  usePoolSnapshot: vi.fn(),
}));
vi.mock('@/widgets/social/use-follow', async original => ({
  ...await original<typeof import('@/widgets/social/use-follow')>(),
  useViewerWallet: vi.fn(),
}));
vi.mock('@/widgets/profile/use-private-account-query', async original => ({
  ...await original<typeof import('@/widgets/profile/use-private-account-query')>(),
  usePrivateAccountQuery: vi.fn(),
}));

function renderCreatorRewards(): string {
  const detail = decodeV2Contract('PoolDetailResponse', v2Reads.responses.poolDetailReal.body).data;
  const snapshot: PoolSnapshot = {
    pool: { chainId: '125', poolAddress: detail.pool.poolAddress, tokenAddress: detail.token.address },
    token: detail.token,
    metadata: null,
    stats: detail.stats,
    creator: detail.pool.creator,
    createdAt: null,
    generatedAt: '2026-11-27T00:00:00Z',
    resolvedBy: 'detail',
    unavailable: { stats: null, metadata: null, registry: null },
  };
  const rewards: V2ContractTypes['RewardsResponse'] = {
    data: { items: [{
      rewardId: 'creator:1',
      programId: 'creator_fees',
      wallet: detail.pool.creator,
      assetAddress: USDL_ADDRESS,
      claimableRaw: '135000',
      claimedRaw: '0',
      earnedRaw: '135000',
      sourcePosition: null,
    }] },
    generatedAt: '2026-11-27T00:00:00Z',
    nextCursor: null,
    quality: 'complete',
    requestId: 'creator-rewards',
    revision: '1',
    watermark: null,
  };
  const client = new QueryClient();
  const poolQuery = new QueryObserver<PoolSnapshot>(client, { queryKey: ['pool'], initialData: snapshot, enabled: false });
  const rewardsQuery = new QueryObserver<V2ContractTypes['RewardsResponse']>(client, { queryKey: ['rewards'], initialData: rewards, enabled: false });
  vi.mocked(usePoolSnapshot).mockReturnValue(poolQuery.getCurrentResult());
  vi.mocked(useViewerWallet).mockReturnValue(detail.pool.creator.toLowerCase());
  vi.mocked(usePrivateAccountQuery).mockReturnValue({ ...rewardsQuery.getCurrentResult(), owner: true, session: null });
  try {
    return renderToString(<CreatorRewardsCard poolAddress={detail.pool.poolAddress} />);
  } finally {
    client.clear();
    vi.clearAllMocks();
  }
}

function creator_card_names_the_program_plainly() {
  const markup = renderCreatorRewards();
  expect(markup).toMatch(/<dt\b[^>]*>Creator fees<\/dt>/);
  expect(markup).toMatch(/<dt\b[^>]*>Creator fees earned<\/dt>/);
  expect(markup.replace(/<[^>]*>/g, ' ')).not.toContain('creator_fees');
  expect(markup).toContain('0.14 USDL');
}
test(creator_card_names_the_program_plainly.name, creator_card_names_the_program_plainly);

function creator_card_notice_reads_in_the_plural() {
  const text = renderCreatorRewards().replace(/<[^>]*>/g, ' ');
  expect(text).toContain('Your rewards from this coin are not available yet');
  expect(text).not.toContain('Your rewards from this coin is not available yet');
}
test(creator_card_notice_reads_in_the_plural.name, creator_card_notice_reads_in_the_plural);
