import type { PriceUpdateItem } from 'types/api/paxeerXFeed';

import { markets, olderUpdateHash, oldestUpdateHash, priceUpdate } from 'mocks/paxeerX/feed';
import { expect, test } from 'vitest';

import applyPriceUpdates from './applyPriceUpdates';

function keepsTheSameBlockSubmissionWithTheHigherUpdateIndexWhateverItsFeederSequence() {
  expect.assertions(4);

  const lowerIndex: PriceUpdateItem = {
    ...priceUpdate,
    hash: olderUpdateHash,
    index: 2,
    price_update: {
      ...priceUpdate.price_update,
      observation_count: 1,
      observations: [
        {
          market_id: 0,
          symbol: 'BTC-USD',
          index_price: '67100000000000000000000',
          mark_price: '67105000000000000000000',
          source_timestamp_ms: 1767323044400,
          sequence: 9,
        },
      ],
    },
  };
  const higherIndex: PriceUpdateItem = {
    ...priceUpdate,
    hash: oldestUpdateHash,
    index: 3,
    price_update: {
      ...priceUpdate.price_update,
      observation_count: 1,
      observations: priceUpdate.price_update.observations
        .filter((observation) => observation.market_id === 0)
        .map((observation) => ({ ...observation, sequence: 1 })),
    },
  };
  const observation = higherIndex.price_update.observations[0];
  const expectedLatest = {
    index: 3,
    index_price: observation.index_price,
    mark_price: observation.mark_price,
    source_timestamp_ms: observation.source_timestamp_ms,
    sequence: 1,
    update_hash: higherIndex.hash,
    block_number: higherIndex.block_number,
    timestamp: higherIndex.timestamp,
  };

  for (const updates of [ [ lowerIndex, higherIndex ], [ higherIndex, lowerIndex ] ]) {
    const batched = applyPriceUpdates(markets, updates);
    const successive = updates.reduce((data, update) => applyPriceUpdates(data, [ update ]), markets);

    expect(batched.items[0].latest).toEqual(expectedLatest);
    expect(successive.items[0].latest).toEqual(expectedLatest);
  }
}

test(
  'keeps the same-block submission with the higher update index whatever its feeder sequence',
  keepsTheSameBlockSubmissionWithTheHigherUpdateIndexWhateverItsFeederSequence,
);
