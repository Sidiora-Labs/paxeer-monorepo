import { describe, expect, it } from 'vitest';

import getTokenInventoryTitle from './getTokenInventoryTitle';

describe('getTokenInventoryTitle', () => {
  it('keeps a lower bound while another cursor page follows', () => {
    expect(getTokenInventoryTitle({ itemsNum: 50, hasNextPage: true }))
      .toBe('More than 50 tokens found');
  });

  it('does not turn the final item of a 51-instance collection into the collection total', () => {
    expect(getTokenInventoryTitle({ itemsNum: 50, hasNextPage: true }))
      .toBe('More than 50 tokens found');
    expect(getTokenInventoryTitle({ itemsNum: 1, hasNextPage: false }))
      .toBe('Showing 1 tokens on this page');
  });

  it('keeps an empty final page scoped to the page rather than an empty collection', () => {
    expect(getTokenInventoryTitle({ itemsNum: 0, hasNextPage: false }))
      .toBe('Showing 0 tokens on this page');
  });

  it('describes only displayed items when the final response is capped', () => {
    expect(getTokenInventoryTitle({ itemsNum: 25, hasNextPage: false }))
      .toBe('Showing 25 tokens on this page');
    expect(getTokenInventoryTitle({ itemsNum: 25, hasNextPage: true }))
      .toBe('More than 25 tokens found');
  });

  it('keeps an explicitly supplied collection count independent of the final page size', () => {
    expect(getTokenInventoryTitle({ itemsNum: 1, hasNextPage: false, inventoryCount: 51 }))
      .toBe('Latest 1 from a total of 51 tokens');
  });

  it('retains an explicitly supplied complete or empty inventory count', () => {
    expect(getTokenInventoryTitle({ itemsNum: 2, hasNextPage: false, inventoryCount: 2 }))
      .toBe('A total of 2 tokens found');
    expect(getTokenInventoryTitle({ itemsNum: 0, hasNextPage: false, inventoryCount: 0 }))
      .toBe('A total of 0 tokens found');
  });

  it('does not carry a count across collection or owner-filter changes', () => {
    expect(getTokenInventoryTitle({ itemsNum: 50, hasNextPage: true, inventoryCount: 105 }))
      .toBe('Latest 50 from a total of 105 tokens');
    expect(getTokenInventoryTitle({ itemsNum: 2, hasNextPage: false }))
      .toBe('Showing 2 tokens on this page');
    expect(getTokenInventoryTitle({ itemsNum: 0, hasNextPage: false }))
      .toBe('Showing 0 tokens on this page');
    expect(getTokenInventoryTitle({ itemsNum: 50, hasNextPage: true }))
      .toBe('More than 50 tokens found');
  });
});
