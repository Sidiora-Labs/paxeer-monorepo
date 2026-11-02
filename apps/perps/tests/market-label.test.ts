import { test } from "node:test";
import { strict as assert } from "node:assert";

import { marketLabel } from "../src/core/perps/marketLabel";

function marketLabelFromCatalog() {
  const catalog = [
    { marketId: 17n, symbol: "BNB" },
    { marketId: 18n, symbol: "DOGE" },
  ] as const;

  assert.equal(marketLabel(18n, catalog), "DOGE");
  assert.equal(marketLabel(18n, []), "18");
  assert.equal(marketLabel(19n, catalog), "19");
  assert.equal(marketLabel(0n, []), "0");
  assert.equal(marketLabel(18n, [{ marketId: 18n, symbol: "UPDATED" }]), "UPDATED");

  const largeMarketId = 9_007_199_254_740_993n;
  const largeCatalog = [{ marketId: largeMarketId, symbol: "LARGE" }] as const;
  assert.equal(marketLabel(largeMarketId, largeCatalog), "LARGE");
  assert.equal(marketLabel(largeMarketId - 1n, largeCatalog), "9007199254740992");
  assert.equal(marketLabel(largeMarketId, []), "9007199254740993");
}

test(marketLabelFromCatalog);
