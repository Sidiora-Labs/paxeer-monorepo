import React from 'react';
import { renderToStaticMarkup } from 'react-dom/server';
import { expect, test } from 'vitest';
import { ActionRow, PaletteStatus } from './global-search-parts';

function headerSearchHeadlineAgreesWithTheWalletRow() {
  const markup = renderToStaticMarkup(
    <>
      <PaletteStatus loading={false} error={null} coinCount={0} userCount={0}
        onRetry={() => {}} query={'0x' + 'c3'.repeat(20)} walletAvailable />
      <ActionRow id="wallet" active={false} icon="wallet" title="Open wallet"
        detail="View profile" onSelect={() => {}} />
    </>,
  );
  expect(markup).toContain('No coins or named users match; open this wallet');
  expect(markup).not.toContain('No coins or users match');
  expect(markup).toContain('Open wallet');
  expect(markup).toContain('View profile');
  expect(markup).not.toMatch(/1 coin|1 user/);

  const wordMarkup = renderToStaticMarkup(
    <PaletteStatus loading={false} error={null} coinCount={0} userCount={0}
      onRetry={() => {}} query="missingcoin" />,
  );
  expect(wordMarkup).toContain('No coins or users match &quot;missingcoin&quot;');
  expect(wordMarkup).not.toContain('open this wallet');
}

function headerSearchCountsCoinsAndNamedUsers() {
  for (const walletAvailable of [false, true]) {
    for (const [coinCount, userCount, count] of [
      [1, 0, '1 coin, 0 users'],
      [0, 1, '0 coins, 1 user'],
      [2, 2, '2 coins, 2 users'],
    ] as const) {
      const markup = renderToStaticMarkup(
        <PaletteStatus loading={false} error={null} coinCount={coinCount} userCount={userCount}
          onRetry={() => {}} query={walletAvailable ? '0x' + 'c3'.repeat(20) : 'pax'}
          walletAvailable={walletAvailable} />,
      );
      expect(markup).toContain(count);
      expect(markup).not.toContain('No coins');
      expect(markup).not.toContain('open this wallet');
    }
  }
}

test(headerSearchHeadlineAgreesWithTheWalletRow.name, headerSearchHeadlineAgreesWithTheWalletRow);
test(headerSearchCountsCoinsAndNamedUsers.name, headerSearchCountsCoinsAndNamedUsers);
