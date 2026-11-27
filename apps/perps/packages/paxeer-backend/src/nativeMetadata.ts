import { formatUnits } from "viem";
import Decimal from "decimal.js";
import {
  COLLATERAL_TOKENS,
  PERPS_MARKETS,
  getMarketCategory,
} from "./core/perps/config";
import { paxeerChain } from "./core/chain";
import { PERPS_SPENDER } from "./core/network/approvals";
import { getNativeSnapshot } from "./runtime";
export const toNativeSymbol = (symbol: string) =>
  symbol
    .replace(/^PERP_/, "")
    .replace(/_USDC$/, "")
    .toUpperCase();
export const toTradingSymbol = (symbol: string) =>
  `PERP_${toNativeSymbol(symbol)}_USDC`;
export function nativeSymbols() {
  const snapshot = getNativeSnapshot();
  const catalog = new Map(
    snapshot.marketCatalog.map((market) => [
      toNativeSymbol(market.symbol),
      market,
    ]),
  );
  const builtIn = new Map(
    PERPS_MARKETS.map((market) => [toNativeSymbol(market.symbol), market]),
  );
  const symbols = new Set([...builtIn.keys(), ...catalog.keys()]);
  const prices = new Map(
    snapshot.markets.map((row) => [toNativeSymbol(row.symbol), row.price]),
  );
  return [...symbols].flatMap((symbol) => {
    const entry = catalog.get(symbol);
    const market = entry ?? builtIn.get(symbol)!;
    if (entry?.enabled === false) return [];
    const price = prices.get(toNativeSymbol(market.symbol)) ?? 0;
    const tick =
      entry?.tickSize != null && BigInt(entry.tickSize) > 0n
        ? new Decimal(formatUnits(BigInt(entry.tickSize), 18))
        : new Decimal(
            price >= 1 ? "0.01" : price >= 0.01 ? "0.0001" : "0.000001",
          );
    return [
      {
        marketId: market.marketId,
        symbol: toTradingSymbol(market.symbol),
        display_symbol_name: market.symbol,
        base_tick: 0.00000001,
        base_min: 0.00000001,
        base_max: Number.MAX_SAFE_INTEGER,
        quote_tick: tick.toNumber(),
        quote_dp: tick.decimalPlaces(),
        quote_min: tick.toNumber(),
        quote_max: Number.MAX_SAFE_INTEGER,
        min_notional: 0.000001,
        base_imr: 1 / (market.maxLeverage ?? 20),
        base_mmr: undefined,
        imr_factor: 0,
        interest_rate: undefined,
        cap_funding: undefined,
        floor_funding: undefined,
        liquidation_fee: undefined,
        max_leverage: market.maxLeverage ?? 20,
        funding_period: 8,
        category: getMarketCategory(market.marketId),
        is_rwa: ["stocks", "indices", "commodities"].includes(
          getMarketCategory(market.marketId),
        ),
      },
    ];
  });
}
export function nativeMarketRows() {
  const snapshots = new Map(
    getNativeSnapshot().markets.map((row) => [row.symbol.toUpperCase(), row]),
  );
  return nativeSymbols().map((info) => {
    const row = snapshots.get(toNativeSymbol(info.symbol));
    const price = row?.price ?? undefined;
    const open = row?.price_24h > 0 ? row.price_24h : undefined;
    return {
      ...info,
      mark_price: price,
      index_price: row?.index_price ?? undefined,
      last_price: price,
      "24h_open": open,
      "24h_close": price,
      "24h_high": row?.high_24h ?? undefined,
      "24h_low": row?.low_24h ?? undefined,
      "24h_volumn": row?.amount_24h ?? undefined,
      "24h_volume": row?.amount_24h ?? undefined,
      "24h_amount": row?.volume_24h ?? undefined,
      open_interest:
        row?.open_interest != null && row?.index_price > 0
          ? row.open_interest / row.index_price
          : undefined,
      est_funding_rate: row?.est_funding_rate ?? undefined,
      last_funding_rate: row?.funding_8h ?? undefined,
      last_funding_rate_hourly: row?.last_funding_rate_hourly ?? null,
      next_funding_time: row?.perp_next_funding_ms ?? null,
      timestamp: row?.price_updated_at ?? undefined,
    };
  });
}
export function nativeChains() {
  return [
    {
      chain_id: paxeerChain.id,
      name: paxeerChain.name,
      network_infos: {
        chain_id: paxeerChain.id,
        name: paxeerChain.name,
        shortName: "Paxeer",
        public_rpc_url: paxeerChain.rpcUrls.default.http[0],
        block_explorer_url: "https://paxscan.io",
        bridgeless: true,
        chain_namespace: "EVM",
        currency_symbol: "PAX",
        currency_decimal: 18,
        vault_address: PERPS_SPENDER,
      },
      token_infos: [COLLATERAL_TOKENS.usdl].map((token) => ({
        ...token,
        chain_id: paxeerChain.id,
        is_native: false,
        is_collateral: true,
        token_hash: token.address,
      })),
    },
  ];
}
export function nativeTokens() {
  return [COLLATERAL_TOKENS.usdl].map((token) => ({
    token: token.symbol,
    symbol: token.symbol,
    decimals: token.decimals,
    token_hash: token.address,
    is_collateral: true,
    collateral_ratio: 1,
    discount_factor: 1,
    minimum_withdraw_amount: 0.000001,
    chain_details: [
      {
        ...token,
        chain_id: paxeerChain.id,
        deposit_fee: undefined,
        withdraw_fee: undefined,
        allow_deposit: true,
        allow_withdraw: true,
      },
    ],
  }));
}
