import { formatUnits, parseUnits } from "viem";
import {
  nativeSymbols,
  nativeMarketRows,
  nativeChains,
  nativeTokens,
  toNativeSymbol,
} from "./nativeMetadata";
import { getNativeSnapshot, updateNativeSnapshot } from "./runtime";
import { executeNativeOrder, cancelNativeOrder } from "./nativeTrading";
import { COLLATERAL_TOKENS, PERPS_MARKETS } from "./core/perps/config";
import { highestPerpsFeeTier } from "./core/perps/feeTier";
import {
  positionMarginRate,
  rowFundingFee,
} from "./core/perps/positionDetails";
import {
  subgraphQuery,
  ORDERS_QUERY,
  TRADES_QUERY,
  VAULT_EVENTS_QUERY,
} from "./hooks/perps/useSubgraph";
import { candleFeedUrl, MARKET_DATA_URL } from "./core/feeds";
import { fetchJsonWithPolicy } from "./core/net/fetch";
import {
  getVaultAddress,
  getBalance,
  getLockedBalance,
  getWalletBalance,
  createVault,
  deposit,
  withdraw,
} from "./hooks/perps/useVault";
const ZERO = "0x0000000000000000000000000000000000000000";
export class NativeCapabilityError extends Error {
  constructor(path: string) {
    super(
      `This operation is not available from the configured Paxeer service: ${path}`,
    );
    this.name = "NativeCapabilityError";
  }
}
export function nativePositionRows() {
  const snapshot = getNativeSnapshot();
  const rows = snapshot.positions.map((position) => {
    const market = PERPS_MARKETS.find(
      (row) => row.marketId === Number(position.marketId),
    );
    const info = nativeSymbols().find(
      (row) => toNativeSymbol(row.symbol) === market?.symbol,
    );
    const size = Number(formatUnits(BigInt(position.sizeUsd), 18));
    const entry = Number(formatUnits(BigInt(position.entryPrice), 18));
    const collateral = Number(formatUnits(BigInt(position.collateralUsd), 18));
    const marketRow = snapshot.markets.find(
      (row) => row.symbol === toNativeSymbol(info?.symbol ?? ""),
    );
    const mark = marketRow?.price;
    const index = marketRow?.index_price;
    const quantity =
      entry > 0 ? (size / entry) * (position.isLong ? 1 : -1) : 0;
    const leverage = Number(formatUnits(BigInt(position.leverage), 18));
    const rate = positionMarginRate(leverage, collateral, size);
    const roi = (pnl: number) =>
      Number.isFinite(pnl) &&
      pnl !== 0 &&
      Number.isFinite(quantity) &&
      quantity !== 0 &&
      Number.isFinite(entry) &&
      entry !== 0 &&
      rate > 0
        ? pnl / (Math.abs(quantity) * entry * rate)
        : 0;
    const pnl =
      Number.isFinite(mark) && mark > 0 ? quantity * (mark - entry) : undefined;
    const indexPnl =
      Number.isFinite(index) && index > 0 ? quantity * (index - entry) : 0;
    return {
      ...position,
      symbol: info?.symbol,
      position_id: position.positionId,
      position_qty: quantity,
      average_open_price: entry,
      mark_price: mark,
      notional:
        Number.isFinite(mark) && mark > 0
          ? Math.abs(quantity * mark)
          : undefined,
      unrealized_pnl: pnl,
      unrealized_pnl_ROI: pnl === undefined ? undefined : roi(pnl),
      unrealized_pnl_index: indexPnl,
      unrealized_pnl_ROI_index: roi(indexPnl),
      unsettled_pnl: mark > 0 ? quantity * (mark - entry) : undefined,
      est_liq_price: position.liquidationPrice
        ? Number(formatUnits(BigInt(position.liquidationPrice), 18))
        : undefined,
      accrued_funding_fee: rowFundingFee(
        position.fundingOwedUsd === undefined
          ? undefined
          : Number(formatUnits(BigInt(position.fundingOwedUsd), 18)),
      ),
      cost_position: size,
      imr: rate > 0 ? rate : undefined,
      mmr: undefined,
      pending_long_qty: 0,
      pending_short_qty: 0,
      leverage: Number(formatUnits(BigInt(position.leverage), 18)),
      timestamp: Number(position.openedAt) * 1000,
      updated_time: Number(position.openedAt) * 1000,
    };
  });
  const totalNotional = rows.reduce(
    (sum, row) =>
      sum +
      (typeof row.notional === "number" && Number.isFinite(row.notional)
        ? row.notional
        : 0),
    0,
  );
  return rows.map((row) => ({
    ...row,
    mm:
      snapshot.maintenanceMarginUsd !== null &&
      Number.isFinite(snapshot.maintenanceMarginUsd) &&
      row.notional !== undefined &&
      Number.isFinite(row.notional) &&
      totalNotional > 0
        ? (snapshot.maintenanceMarginUsd * row.notional) / totalNotional
        : null,
  }));
}
export function nativePositionTotals(
  rows: readonly {
    notional?: unknown;
    unrealized_pnl?: unknown;
    unrealized_pnl_index?: unknown;
  }[],
  totalValue: number | null,
) {
  let notional = 0;
  let pnl = 0;
  let indexPnl = 0;
  for (const row of rows) {
    if (typeof row.notional === "number" && Number.isFinite(row.notional))
      notional += row.notional;
    if (
      typeof row.unrealized_pnl === "number" &&
      Number.isFinite(row.unrealized_pnl)
    )
      pnl += row.unrealized_pnl;
    if (
      typeof row.unrealized_pnl_index === "number" &&
      Number.isFinite(row.unrealized_pnl_index)
    )
      indexPnl += row.unrealized_pnl_index;
  }
  const roi = (result: number) =>
    totalValue !== null &&
    Number.isFinite(totalValue) &&
    totalValue !== 0 &&
    totalValue - result !== 0
      ? result / (totalValue - result)
      : 0;
  return {
    notional,
    unrealPnL: pnl,
    total_unreal_pnl: pnl,
    total_unreal_pnl_index: indexPnl,
    unrealPnlROI: roi(pnl),
    unrealPnlROI_index: roi(indexPnl),
  };
}
export function nativeAccountInfo() {
  const state = getNativeSnapshot();
  return {
    account_id: state.address,
    address: state.address,
    max_leverage: 1000,
    account_mode: "FUTURES",
    margin_mode: state.marginMode,
    imr_factor: Object.fromEntries(
      nativeSymbols().map((symbol) => [symbol.symbol, symbol.imr_factor]),
    ),
    account_group: "PAXEER",
    futures_taker_fee_rate:
      state.feeTier?.takerBps ?? highestPerpsFeeTier.takerBps,
    futures_maker_fee_rate:
      state.feeTier?.makerBps ?? highestPerpsFeeTier.makerBps,
    taker_fee_rate: 0,
    maker_fee_rate: 0,
  };
}
export async function nativeRequest(
  url: string,
  init: RequestInit = {},
): Promise<any> {
  const location = new URL(url, "http://paxeer-network.native");
  const path = location.pathname;
  const state = getNativeSnapshot();
  const method = (init.method ?? "GET").toUpperCase();
  const body =
    typeof init.body === "string" && init.body ? JSON.parse(init.body) : {};
  const success = (data: any) => ({
    success: true,
    data,
    timestamp: Date.now(),
  });
  if (method !== "GET") {
    if (/\/algo\/order$/.test(path) && method !== "DELETE")
      throw new NativeCapabilityError(path);
    if (/\/(?:batch-order|batch_order)$/.test(path)) {
      const orders = body.orders ?? body;
      const result: any[] = [];
      if (!Array.isArray(orders)) throw new Error("Orders must be a list.");
      for (const order of orders) result.push(await executeNativeOrder(order));
      return success({ rows: result });
    }
    if (/\/order$/.test(path) && method === "DELETE")
      return cancelNativeOrder(
        String(
          location.searchParams.get("order_id") ??
            location.searchParams.get("algo_order_id") ??
            body.order_id ??
            body.algo_order_id,
        ),
      );
    if (/\/order$/.test(path) && method === "POST")
      return executeNativeOrder(body);
    if (/\/leverage$/.test(path)) {
      const symbol = body.symbol;
      const value = Number(body.leverage);
      if (!symbol || !Number.isFinite(value) || value < 1)
        throw new Error("Choose a valid leverage.");
      updateNativeSnapshot({
        leverage: { ...state.leverage, [symbol]: value },
      });
      return success({ leverage: value });
    }
    throw new NativeCapabilityError(`${method} ${path}`);
  }
  if (path.endsWith("/public/info")) return success({ rows: nativeSymbols() });
  if (/\/public\/futures(?:\/[^/]+)?$/.test(path)) {
    const rows = nativeMarketRows();
    const symbol = path.split("/futures/")[1];
    return success(
      symbol ? rows.find((row) => row.symbol === symbol) : { rows },
    );
  }
  if (path.endsWith("/public/token")) return success({ rows: nativeTokens() });
  if (path.endsWith("/public/chain_info"))
    return success({ rows: nativeChains() });
  if (path.endsWith("/public/system_info")) return success({});
  if (path.includes("/public/funding_rate")) {
    const rows = nativeMarketRows()
      .filter((row) => Number.isFinite(row.est_funding_rate))
      .map((row) => ({
        symbol: row.symbol,
        est_funding_rate: row.est_funding_rate,
        last_funding_rate: row.last_funding_rate,
        next_funding_time: row.next_funding_time,
      }));
    const symbol = path.split("/funding_rate/")[1];
    return success(
      symbol ? (rows.find((row) => row.symbol === symbol) ?? {}) : { rows },
    );
  }
  if (path.endsWith("/public/rwa/symbols"))
    return success({ rows: nativeSymbols().filter((row) => row.is_rwa) });
  if (path.includes("/public/orderbook/")) {
    const symbol = toNativeSymbol(path.split("/").at(-1)!);
    const result = await fetchJsonWithPolicy<any>(
      `${MARKET_DATA_URL}/orderbook/${symbol}?depth=100`,
    );
    if (!result.success || !result.data)
      throw new Error("Paxeer order book is unavailable.");
    return success({ ...result.data, symbol: `PERP_${symbol}_USDC` });
  }
  if (path.endsWith("/public/kline")) {
    const symbol = toNativeSymbol(location.searchParams.get("symbol") ?? "ETH");
    const interval =
      location.searchParams.get("type")?.replace(/^kline_/, "") ?? "15m";
    const resolution =
      (
        {
          "1m": "1",
          "3m": "3",
          "5m": "5",
          "15m": "15",
          "30m": "30",
          "1h": "60",
          "4h": "240",
          "1d": "1D",
          "1w": "1W",
        } as Record<string, string>
      )[interval] ?? "15";
    const end = Math.floor(Date.now() / 1000);
    const query = new URLSearchParams({
      resolution,
      from: String(end - 7 * 86400),
      to: String(end),
    });
    const result = await fetchJsonWithPolicy<any>(
      candleFeedUrl("history", symbol, query),
    );
    if (result.s !== "ok" || !Array.isArray(result.t))
      throw new Error("Paxeer candle history is unavailable.");
    return success({
      rows: result.t.map((time: number, index: number) => ({
        start_timestamp: time * 1000,
        end_timestamp: time * 1000,
        open: result.o[index],
        high: result.h[index],
        low: result.l[index],
        close: result.c[index],
        volume: result.v?.[index] ?? 0,
      })),
    });
  }
  if (!state.address && !path.includes("/public/"))
    throw new Error("Connect your Paxeer wallet to view account data.");
  if (/\/(?:client\/info|client\/account|account)$/.test(path))
    return success(nativeAccountInfo());
  if (path.endsWith("/positions"))
    return success({ rows: nativePositionRows() });
  if (path.endsWith("/client/holding")) {
    return success({
      holding: await Promise.all(
        Object.values(COLLATERAL_TOKENS).map(async (token) => {
          const vault = await getVaultAddress(state.address!);
          const amount =
            vault === ZERO ? 0n : await getBalance(vault, token.address);
          const locked =
            vault === ZERO ? 0n : await getLockedBalance(vault, token.address);
          return {
            token: token.symbol,
            holding: Number(formatUnits(amount, token.decimals)),
            frozen: Number(formatUnits(locked, token.decimals)),
            pending_short: 0,
            updated_time: Date.now(),
          };
        }),
      ),
    });
  }
  if (path.endsWith("/client/margin_modes"))
    return success(
      state.marginMode
        ? nativeSymbols().map((row) => ({
            symbol: row.symbol,
            default_margin_mode: state.marginMode,
          }))
        : [],
    );
  if (path.endsWith("/client/leverages"))
    return success(
      Object.entries(state.leverage).map(([symbol, leverage]) => ({
        symbol,
        leverage,
        margin_mode: "CROSS",
      })),
    );
  if (/\/(?:algo\/)?orders$/.test(path)) {
    const result = await subgraphQuery<{ orders: any[] }>(ORDERS_QUERY, {
      userAddress: state.address!.toLowerCase(),
    });
    const wanted = location.searchParams.get("status");
    const wantedSymbol = location.searchParams.get("symbol");
    const rows = result.orders
      .map((row) => {
        const symbol = nativeSymbols().find(
          (info) =>
            toNativeSymbol(info.symbol) ===
            PERPS_MARKETS.find((m) => m.marketId === Number(row.marketId))
              ?.symbol,
        )?.symbol;
        const price = Number(formatUnits(BigInt(row.triggerPrice), 18));
        const size = Number(formatUnits(BigInt(row.sizeUsd), 18));
        const type = String(row.orderTypeName)
          .replace(/([a-z])([A-Z])/g, "$1_$2")
          .replace(/ /g, "_")
          .toUpperCase();
        const rawStatus = String(row.status).toUpperCase();
        const status =
          rawStatus === "PENDING" || rawStatus === "ACTIVE"
            ? "NEW"
            : rawStatus === "EXECUTED"
              ? "FILLED"
              : rawStatus === "CANCELED"
                ? "CANCELLED"
                : rawStatus;
        return {
          ...row,
          symbol,
          order_id: row.orderId,
          algo_order_id: row.orderId,
          order_type: type,
          type,
          side: row.isLong ? "BUY" : "SELL",
          order_price: price,
          price,
          order_quantity: price > 0 ? size / price : undefined,
          quantity: price > 0 ? size / price : undefined,
          created_time: Number(row.placedAt) * 1000,
          updated_time: Number(row.resolvedAt ?? row.placedAt) * 1000,
          status,
        };
      })
      .filter(
        (row) =>
          (!wantedSymbol || row.symbol === wantedSymbol) &&
          (!wanted ||
            wanted === "ALL" ||
            (wanted === "INCOMPLETE"
              ? ["NEW", "PARTIAL_FILLED"].includes(row.status)
              : row.status === wanted)),
      );
    const algo = path.includes("/algo/");
    const filtered = rows.filter((row) =>
      algo
        ? ["TAKE_PROFIT", "STOP_LOSS", "STOP_LIMIT"].includes(row.type)
        : !["TAKE_PROFIT", "STOP_LOSS"].includes(row.type),
    );
    const size = Math.max(1, Number(location.searchParams.get("size") ?? 50));
    const page = Math.max(1, Number(location.searchParams.get("page") ?? 1));
    return success({
      rows: filtered.slice((page - 1) * size, page * size),
      meta: {
        current_page: page,
        records_per_page: size,
        total: filtered.length,
      },
    });
  }
  if (/\/(?:trades|client\/trades)$/.test(path)) {
    const result = await subgraphQuery<{ trades: any[] }>(TRADES_QUERY, {
      userAddress: state.address!.toLowerCase(),
      limit: 100,
      offset: 0,
    });
    return success({
      rows: result.trades.map((row) => ({
        ...row,
        id: row.id,
        transaction_hash: row.txHash,
        realized_pnl: Number(formatUnits(BigInt(row.realizedPnl ?? 0), 18)),
        fee: Number(formatUnits(BigInt(row.feeUsd ?? 0), 18)),
        executed_price: Number(formatUnits(BigInt(row.price), 18)),
        created_time: Number(row.blockTimestamp) * 1000,
      })),
    });
  }
  if (/\/asset\/history$/.test(path)) {
    const result = await subgraphQuery<{ vaultEvents: any[] }>(
      VAULT_EVENTS_QUERY,
      { userAddress: state.address!.toLowerCase(), limit: 100, offset: 0 },
    );
    return success({
      rows: result.vaultEvents.map((row) => {
        const token = nativeToken(row.tokenAddress);
        return {
          ...row,
          token: token.symbol,
          amount: Number(formatUnits(BigInt(row.amount), token.decimals)),
          side: String(row.eventType).toLowerCase().includes("withdraw")
            ? "WITHDRAW"
            : "DEPOSIT",
          trans_status: "COMPLETED",
          tx_id: row.txHash,
          created_time: Number(row.blockTimestamp) * 1000,
        };
      }),
    });
  }
  if (path.endsWith("/client/statistics/daily") && state.dailyPnl)
    return success({ rows: state.dailyPnl });
  if (path.endsWith("/client/leverage"))
    return success({
      leverage: state.leverage[location.searchParams.get("symbol") ?? ""] ?? 2,
    });
  throw new NativeCapabilityError(path);
}
export async function getNativeVault(create = false) {
  const address = getNativeSnapshot().address;
  if (!address) throw new Error("Connect your wallet first.");
  let vault = await getVaultAddress(address);
  if (vault === ZERO && create) {
    await createVault();
    vault = await getVaultAddress(address);
  }
  if (vault === ZERO)
    throw new Error("Create your Paxeer vault before continuing.");
  return vault;
}
export function nativeToken(symbolOrAddress: string) {
  const token = Object.values(COLLATERAL_TOKENS).find(
    (token) =>
      token.symbol === symbolOrAddress.toUpperCase() ||
      token.address.toLowerCase() === symbolOrAddress.toLowerCase(),
  );
  if (!token) throw new Error("Choose a supported Paxeer collateral token.");
  return token;
}
export async function nativeDeposit(inputs: {
  address?: string;
  token?: string;
  amount: string;
  decimals: number;
}) {
  const token = nativeToken(inputs.address ?? inputs.token ?? "USDL");
  const vault = await getNativeVault(true);
  const amount = parseUnits(inputs.amount, token.decimals);
  if (amount <= 0n) throw new Error("Enter a positive deposit amount.");
  const receipt = await deposit(vault, token.address, amount);
  return { ...receipt, hash: receipt.transactionHash };
}
export async function nativeWithdraw(inputs: {
  token: string;
  amount: string | number;
  receiver?: string;
  chainId?: number;
}) {
  const address = getNativeSnapshot().address;
  if (
    inputs.receiver &&
    inputs.receiver.toLowerCase() !== address?.toLowerCase()
  )
    throw new Error("Paxeer vault withdrawals return to the connected wallet.");
  const token = nativeToken(inputs.token);
  const amount = parseUnits(String(inputs.amount), token.decimals);
  if (amount <= 0n) throw new Error("Enter a positive withdrawal amount.");
  const receipt = await withdraw(await getNativeVault(), token.address, amount);
  return { ...receipt, hash: receipt.transactionHash };
}
