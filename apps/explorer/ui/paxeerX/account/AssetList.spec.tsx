// @vitest-environment jsdom

import React from 'react';

import * as paxeerXMock from 'mocks/paxeerX/unifiedAccount';
import { describe, expect, it } from 'vitest';
import { screen } from 'vitest/lib';

import AssetList from './AssetList';
import { render } from './testWrapper';

describe('AssetList', () => {
  it('shows an empty message when the account holds nothing', () => {
    render(<AssetList items={ [] }/>);

    expect(screen.getByText('No assets are held by this account.')).toBeTruthy();
  });

  it('renders one row with one total per asset', () => {
    const { container } = render(<AssetList items={ paxeerXMock.unifiedAccount.balances }/>);

    expect(container.querySelectorAll('[data-asset]')).toHaveLength(2);
    expect(container.querySelector(`[data-asset="${ paxeerXMock.nativeAsset.id }"] [data-label="total"]`)?.textContent).toBe('1');
    expect(container.querySelector(`[data-asset="${ paxeerXMock.tokenAsset.id }"] [data-label="total"]`)?.textContent).toBe('2.5');
    // an asset with a symbol is denominated in it, so the symbol and the denom columns agree
    expect(screen.getAllByText('USDX')).toHaveLength(2);
    expect(screen.getAllByText('HPX')).toHaveLength(2);
  });

  it('renders flat asset rows without breakdown controls or parts', () => {
    const { container } = render(<AssetList items={ paxeerXMock.unifiedAccount.balances }/>);

    expect(container.querySelectorAll('[data-parts-of]')).toHaveLength(0);

    expect(container.querySelectorAll('[data-part]')).toHaveLength(0);
    expect(screen.queryByLabelText(/breakdown/)).toBeNull();
    expect(Array.from(container.querySelectorAll('[data-asset]')).map((row) => row.querySelectorAll('td').length))
      .toEqual([ 3, 3 ]);
    expect(Array.from(container.querySelectorAll('thead th')).map((cell) => cell.textContent))
      .toEqual([ 'Asset', 'Denom', 'Total' ]);
  });

  it('renders the denom and unscaled total when metadata is absent', () => {
    const balance = {
      asset: { ...paxeerXMock.tokenAsset, symbol: null, decimals: null },
      total: '1200',
    };
    const { container } = render(<AssetList items={ [ balance ] }/>);

    expect(container.querySelectorAll('[data-asset]')).toHaveLength(1);
    expect(screen.getAllByText(paxeerXMock.tokenAsset.denom)).toHaveLength(2);
    expect(container.querySelector('[data-label="asset-id"]')?.textContent).toBe(paxeerXMock.tokenAsset.id);
    expect(container.querySelector('[data-label="total"]')?.textContent).toBe('1,200');
    expect(container.querySelectorAll('[data-parts-of]')).toHaveLength(0);
  });
  it('heads the scan table card with the asset count and its note', () => {
    const { container } = render(<AssetList items={ paxeerXMock.unifiedAccount.balances }/>);

    const card = container.querySelector('[data-scan-table-card]');

    expect(card?.querySelector('[data-title]')?.textContent).toBe('A total of 2 assets found');
    expect(card?.querySelector('[data-note]')?.textContent).toBe('Chain, custody and kernel balances of one asset add up to its total');
  });

  it('keeps the empty message inside the card', () => {
    const { container } = render(<AssetList items={ [] }/>);

    const card = container.querySelector('[data-scan-table-card]') as HTMLElement;

    expect(card.querySelector('[data-title]')?.textContent).toBe('A total of 0 assets found');
    expect(card.querySelector('[data-body]')?.textContent).toBe('No assets are held by this account.');
  });

});
