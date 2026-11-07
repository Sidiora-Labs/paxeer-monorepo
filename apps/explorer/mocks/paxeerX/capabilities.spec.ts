import type { PaxeerXCapabilities } from 'types/api/paxeerX';

import { expect, test } from 'vitest';

import * as capabilitiesMock from './capabilities';

function capabilities_mocks_have_no_retired_keys() {
  const expectedCapabilities = {
    addr: true,
    exchange: true,
    bridge: true,
    launchpad: true,
  } satisfies Required<Omit<PaxeerXCapabilities, 'price_feed'>>;
  const expectedKeys = Object.keys(expectedCapabilities).sort();
  const mocks = Object.values(capabilitiesMock);

  expect(mocks.length).toBeGreaterThan(0);

  for (const mock of mocks) {
    expect(Object.keys(mock).sort()).toEqual(expectedKeys);
    expect(mock).not.toHaveProperty('custody');
    expect(mock).not.toHaveProperty('anchor');
  }
}

test(capabilities_mocks_have_no_retired_keys);
