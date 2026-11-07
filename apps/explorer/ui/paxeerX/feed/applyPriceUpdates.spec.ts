import type { PriceUpdateItem } from 'types/api/paxeerXFeed';

import { markets, olderUpdateHash, oldestUpdateHash, priceUpdate, updateHash } from 'mocks/paxeerX/feed';
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
          accepted: true,
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

function keepsTheAcceptedPriceWhenANewerSuccessfulSubmissionCarriesARefusedObservation() {
  const observation = {
    accepted: false,
    market_id: 0,
    symbol: 'BTC-USD',
    index_price: '67500000000000000000000',
    mark_price: '67505000000000000000000',
    source_timestamp_ms: 1767323050000,
    sequence: 9003,
  };
  const update: PriceUpdateItem = {
    ...priceUpdate,
    hash: olderUpdateHash,
    block_number: 34800124,
    price_update: {
      ...priceUpdate.price_update,
      observation_count: 1,
      observations: [ observation ],
    },
  };
  const refused = applyPriceUpdates({ ...markets, chain_head: 34800123 }, [ update ]);

  expect(refused.items[0].latest).toBe(markets.items[0].latest);
  expect(refused.items[0].latest?.update_hash).toBe(updateHash);
  expect(refused.items[0].latest?.block_number).toBe(34800123);
  expect(refused.chain_head).toBe(34800124);

  const accepted = applyPriceUpdates(refused, [ {
    ...update,
    price_update: {
      ...update.price_update,
      observations: [ { ...observation, accepted: true } ],
    },
  } ]);

  expect(accepted.items[0].latest).toEqual({
    index: update.index,
    index_price: observation.index_price,
    mark_price: observation.mark_price,
    source_timestamp_ms: observation.source_timestamp_ms,
    sequence: observation.sequence,
    update_hash: update.hash,
    block_number: 34800124,
    timestamp: update.timestamp,
  });
  expect(accepted.chain_head).toBe(34800124);
}

test(
  'keeps the accepted price when a newer successful submission carries a refused observation',
  keepsTheAcceptedPriceWhenANewerSuccessfulSubmissionCarriesARefusedObservation,
);
