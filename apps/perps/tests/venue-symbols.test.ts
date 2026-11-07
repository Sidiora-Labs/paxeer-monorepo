import { test } from 'node:test';
import assert from 'node:assert/strict';

process.env.NEXT_PUBLIC_PERPS_GATEWAY_URL ??= 'https://perps-gateway.invalid';

async function registeredSymbolIsVenueSymbol() {
  const { registerVenueSymbols, isVenueSymbol, isSpotVenueSymbol } = await import('../src/core/net/venueStream');
  const { PERPS_MARKETS } = await import('../src/core/perps/config');

  for (const market of PERPS_MARKETS) {
    assert.equal(isVenueSymbol(market.symbol.toLowerCase()), true);
  }
  assert.equal(isVenueSymbol('DOGE'), false);
  assert.equal(isVenueSymbol('doge'), false);
  assert.equal(isVenueSymbol(null), false);
  assert.equal(isVenueSymbol(undefined), false);
  assert.equal(isVenueSymbol(''), false);

  try {
    registerVenueSymbols(['BTC', 'DOGE']);
    assert.equal(isVenueSymbol('doge'), true);
    assert.equal(isVenueSymbol('DOGE'), true);
    assert.equal(isVenueSymbol('btc'), true);
    assert.equal(isVenueSymbol('ETH'), false);
    assert.equal(isSpotVenueSymbol('pax'), true);
    assert.equal(isSpotVenueSymbol('sid'), true);
    assert.equal(isSpotVenueSymbol('DOGE'), false);
    assert.equal(isSpotVenueSymbol('BTC'), false);

    const updatedSymbols = ['eth', 'DoGe'] as const;
    registerVenueSymbols(updatedSymbols);
    assert.equal(isVenueSymbol('ETH'), true);
    assert.equal(isVenueSymbol('doge'), true);
    assert.equal(isVenueSymbol('BTC'), false);
    assert.deepEqual(updatedSymbols, ['eth', 'DoGe']);

    registerVenueSymbols([]);
    assert.equal(isVenueSymbol('DOGE'), false);
    for (const market of PERPS_MARKETS) {
      assert.equal(isVenueSymbol(market.symbol), false);
    }
    assert.equal(isSpotVenueSymbol('PAX'), true);
    assert.equal(isSpotVenueSymbol('SID'), true);
  } finally {
    registerVenueSymbols(PERPS_MARKETS.map(m => m.symbol));
  }
}

test(registeredSymbolIsVenueSymbol);
