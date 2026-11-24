// @vitest-environment jsdom
import React from 'react';
import { renderToStaticMarkup } from 'react-dom/server';
import { expect, test } from 'vitest';
import { CoinCard } from '@/ui/primitives/coin-card';
import { CoinRow } from '@/shell/global-search-parts';
import { TrendingRow } from '@/widgets/home/trending-row';
import { toTrendingRow } from '@/widgets/home/trending-widget';
import { buildExploreRow } from '@/widgets/explore/use-explore-rows';
import { PairRow } from '@/widgets/terminal/left-rail/pair-row';
import { toRailPairRow } from '@/widgets/terminal/left-rail/rows';
import HolderRewardTokenCard from '@/widgets/holder-rewards/holder-reward-token-card';
import { tokenIdentity } from '@/widgets/profile/positions-open-closed';
import { displayCoinText } from './coin-text';
import { shortAddress } from './format';

const token = '0x1234567890abcdef1234567890abcdef12345678';
const pool = '0xabcdefabcdefabcdefabcdefabcdefabcdefabcd';

function coinViewsDisplayBlankControlAndLongLabels() {
  for (const raw of ['', '   ', '\u0000', 'Coin\u202eName', '🪙'.repeat(100), 'Ordinary Coin']) {
    const display = displayCoinText(raw, token);
    const metadata = { token_address: token, name: raw, symbol: raw, images: {} };
    const row = buildExploreRow({ poolAddress: pool, rank: 1, listing: { poolAddress: pool, tokenAddress: token, creator: pool, createdAtMs: null }, stats: null, metadata, now: 1000 });
    expect(row.name).toBe(display);
    expect(row.symbol).toBe(display);
    expect(tokenIdentity(token, metadata).name).toBe(display);
    expect(tokenIdentity(token, metadata).symbol).toBe(display);
    const nodes = [
      <CoinCard key="card" tokenAddress={token} image={null} name={raw} symbol={raw} mcapRaw={null} percentChange={null} createdAt={null} href={`/coin/${pool}`} />,
      <CoinRow key="search" id="coin" active={false} name={raw} symbol={raw} logo={null} mcapRaw={null} address={pool} tokenAddress={token} kind="pool" onSelect={() => {}} />,
      <TrendingRow key="trending" row={toTrendingRow(row, '24h', metadata)} />,
      <PairRow key="rail" row={toRailPairRow(row, metadata, '1h')} active={false} now={1000} />,
      <HolderRewardTokenCard key="rewards" name={raw} symbol={raw} image={null} tokenAddress={token} href={`/coin/${pool}`} createdAt={null} distributedRaw={null} mcapRaw={null} holdersPaid={null} />,
    ];
    for (const node of nodes) {
      const markup = renderToStaticMarkup(node);
      const doc = new DOMParser().parseFromString(markup, 'text/html');
      expect(doc.body.textContent).toContain(display);
      expect(markup).not.toContain('\u0000');
      expect(markup).not.toContain('\u202e');
      const labels = Array.from(doc.querySelectorAll<HTMLElement>('[title]')).filter(element => element.title === display);
      expect(labels.length).toBeGreaterThan(0);
      expect(labels.some(element => element.classList.contains('truncate') || element.querySelector('.truncate') !== null)).toBe(true);
    }
  }
  expect(tokenIdentity(token, null).name).toBe(shortAddress(token));
}
test(coinViewsDisplayBlankControlAndLongLabels.name, coinViewsDisplayBlankControlAndLongLabels);
