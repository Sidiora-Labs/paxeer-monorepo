import { test } from "node:test";
import { strict as assert } from "node:assert";
import type { GatewayMarketRow } from "../src/core/perps/marketCatalog";

const ENV_NAMES = ["LOCAL_NODE_RPC", "NEXT_PUBLIC_PERPS_GATEWAY_URL"] as const;
type EnvName = (typeof ENV_NAMES)[number];

function requireEnv(): Record<EnvName, string> {
  const env = {} as Record<EnvName, string>;
  for (const name of ENV_NAMES) {
    const value = process.env[name];
    if (!value) throw new Error(`${name} is not set`);
    env[name] = value;
  }
  return env;
}

async function perpsMarketsLoadFromGateway() {
  const env = requireEnv();
  process.env.NEXT_PUBLIC_PAXEER_RPC_URLS = env.LOCAL_NODE_RPC;
  const { loadPerpsMarkets } = await import("../src/hooks/perps/usePerpsMarkets");
  const { MARKETS_QUERY } = await import("../src/hooks/perps/useSubgraph");
  const { isVenueSymbol } = await import("../src/core/net/venueStream");
  const { catalogFromGateway } = await import("../src/core/perps/marketCatalog");

  const loaded = await loadPerpsMarkets();
  const response = await fetch(
    new URL("/graphql", env.NEXT_PUBLIC_PERPS_GATEWAY_URL),
    {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ query: MARKETS_QUERY }),
      signal: AbortSignal.timeout(8_000),
    },
  );
  assert.equal(response.status, 200);
  const answer = (await response.json()) as {
    data: { markets: GatewayMarketRow[] };
    errors?: { message: string }[];
  };
  assert.equal(answer.errors?.length ?? 0, 0, JSON.stringify(answer.errors));
  const rows = answer.data.markets;
  assert.ok(rows.length > 0, "the live gateway returns markets");
  assert.equal(
    loaded.catalog.length,
    rows.length,
    "one catalog entry per gateway market",
  );
  assert.equal(
    new Set(loaded.catalog.map((entry) => entry.marketId)).size,
    rows.length,
  );
  const mapped = catalogFromGateway(answer.data);
  for (const row of rows) {
    const entry = loaded.catalog.find((market) => market.marketId === row.marketId);
    assert.ok(entry, `market ${row.marketId} is loaded`);
    assert.ok(
      Number.isInteger(entry.szDecimals) &&
        entry.szDecimals !== null &&
        entry.szDecimals >= 0,
      `${row.symbol} has chain size decimals, including zero`,
    );
    assert.ok(
      entry.tickSize !== null && BigInt(entry.tickSize) > 0n,
      `${row.symbol} has a positive tick size`,
    );
    assert.deepEqual(entry, {
      ...mapped.catalog.find((market) => market.marketId === row.marketId),
      baseToken: row.symbol,
      oracleSymbol: row.oracleSymbol,
      tickSize: row.tickSize,
      lotSize: row.lotSize,
      tierClass: row.tierClass,
    });
    assert.equal(entry.szDecimals, row.szDecimals);
    assert.equal(entry.pxDecimals, row.pxDecimals);
    assert.ok(
      isVenueSymbol(row.symbol),
      `${row.symbol} is registered with the venue stream`,
    );
  }
  assert.deepEqual(loaded.tradeable, loaded.catalog.filter((entry) => entry.enabled));
  assert.ok(
    loaded.tradeable.some((entry) => entry.symbol === "PAX"),
    "the PAX default is available",
  );
}

test(perpsMarketsLoadFromGateway);
