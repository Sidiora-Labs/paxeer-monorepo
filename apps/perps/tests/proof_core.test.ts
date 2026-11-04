/**
 * Proves: core/net/venueStream.isVenueSymbol, core/perps/pnlHelpers.computePositionPnl,
 * core/perps/orderPlan.buildOrderPlan, core/errors/classify.PRECOMPILE_ERROR_CODES,
 * core/perps/calls.buildGet{FeeTier,MarginTiers,MarketConfig,MarkPriceDetails}
 * -> exported TypeScript modules, useQuoter.quoterMarketReads, viem ABI codecs
 * -> MobileTradeHeader.mobileTradeMarket, analytics.analyticsMarketId,
 * perps.tradingMarket, AlertsPanel.alertErrorText, MarketSnapshotTable.snapshotPrice,
 * orderPlan.buildConfirmation and PortfolioFeeTierPanel.
 *
 * Repairs: venueStream.registerVenueSymbols could partially replace its registry;
 * calls accepted malformed accounts and negative/out-of-range market identifiers;
 * useQuoter cast malformed open quotes; orderPlan trusted mismatched quotes;
 * pnlHelpers accepted negative position inputs; classify discarded already
 * classified errors; analytics silently selected a different market for an unknown
 * selection; the mobile header accepted disabled markets; snapshot prices could
 * display NaN and lost integer precision before formatting. Consumer selection
 * and error formatting are exported from, and used by, their actual UI modules.
 *
 * All producers run in this process. The crossing is the application's exported
 * TypeScript surface, not an operator's RPC or a replacement transport. The read
 * builders produce real ABI descriptors, which the quoter consumes unchanged;
 * the ABI codecs below consume those descriptors, including malformed refusals.
 * No persistent state is claimed. Fresh producer calls and consumer observations
 * run twice, then with changed input, including recovery after refused input.
 * No workspace dist entry, environment-provisioned service, mock or skip is used.
 */
import assert from "node:assert/strict";
import { test } from "node:test";
import { createElement } from "react";
import { renderToStaticMarkup } from "react-dom/server";
import {
  decodeFunctionData,
  decodeFunctionResult,
  encodeErrorResult,
  encodeFunctionData,
  encodeFunctionResult,
  zeroAddress,
  type Address,
} from "viem";
import type { OrderFormState } from "../src/core/perps/orderPlan";
import type { OpenPositionQuote } from "../src/hooks/perps/useQuoter";
import type { ParsedPosition } from "../src/core/perps/pnlHelpers";
import { auditMarketConfig, auditMarginTiers } from "./fixtures/audit-market-config";
import type { GatewayMarketRow } from "../src/core/perps/marketCatalog";

// The production gateway module validates configuration during import. These
// pure module paths never make a request; no URL is obtained from the environment.
process.env.NEXT_PUBLIC_PERPS_GATEWAY_URL = "http://127.0.0.1";

const WAD = 10n ** 18n;
const ACCOUNT: Address = "0x1234567890123456789012345678901234567890";

async function coreCatalogReachesMobileTradingAndAnalyticsConsumers() {
  const { catalogFromGateway } = await import("../src/core/perps/marketCatalog");
  const { registerVenueSymbols, isVenueSymbol } = await import("../src/core/net/venueStream");
  const { mobileTradeMarket } = await import("../src/app/shell/MobileTradeHeader");
  const { analyticsMarketId } = await import("../src/page-modules/analytics");
  const { tradingMarket } = await import("../src/page-modules/perps");
  const { PERPS_MARKETS } = await import("../src/core/perps/config");
  const row: GatewayMarketRow = {
    marketId: 81, symbol: "PROOF", name: "Proof market", oracleSymbol: "PROOF",
    szDecimals: 4, pxDecimals: 2, tickSize: String(WAD / 100n), lotSize: String(WAD),
    tierClass: 0, maxLeverage: "20", enabled: true, createdAt: null,
    latestPrice: null, fundingRate: null,
  };
  try {
    for (const enabled of [true, false, true]) {
      for (let pass = 0; pass < 2; pass++) {
        const produced = catalogFromGateway({ markets: [{ ...row, enabled }] }, 0);
        registerVenueSymbols(produced.catalog.map((market) => market.symbol));
        assert.equal(isVenueSymbol("proof"), true);
        assert.equal(isVenueSymbol("UNKNOWN"), false);
        assert.equal(isVenueSymbol(null), false);
        assert.equal(isVenueSymbol(undefined), false);
        assert.equal(mobileTradeMarket("PROOF/USD", produced.catalog), enabled ? produced.catalog[0] : undefined);
        assert.equal(tradingMarket("proof", produced.catalog), enabled ? produced.catalog[0] : undefined);
        assert.equal(analyticsMarketId(81, produced.catalog), 81);
        assert.equal(analyticsMarketId(undefined, produced.catalog), 81);
        assert.equal(mobileTradeMarket("UNKNOWN/USD", produced.catalog), undefined);
        assert.equal(tradingMarket("UNKNOWN", produced.catalog), undefined);
        assert.equal(analyticsMarketId(999, produced.catalog), undefined,
          "an unknown selection never silently selects another market");
      }
    }
    assert.throws(() => registerVenueSymbols(["REPLACEMENT", " bad "]), /Invalid venue symbol/);
    assert.equal(isVenueSymbol("PROOF"), true, "a refused update preserves the complete prior registry");
    assert.equal(isVenueSymbol("REPLACEMENT"), false);
    registerVenueSymbols(["NEXT"]);
    assert.equal(isVenueSymbol("PROOF"), false, "registration does not retain a cached old symbol");
    assert.equal(isVenueSymbol("NEXT"), true);
  } finally {
    registerVenueSymbols(PERPS_MARKETS.map((market) => market.symbol));
  }
}

async function readBuildersReachQuoterAndFeeTierConsumer() {
  const calls = await import("../src/core/perps/calls");
  const { quoterMarketReads } = await import("../src/hooks/perps/useQuoter");
  const { PerpsPrecompileABI } = await import("../src/core/network/abi/PerpsPrecompile");
  const { PRECOMPILES } = await import("../src/core/network/precompiles");
  const { feeTierView } = await import("../src/core/perps/feeTier");
  const { PortfolioFeeTierPanel } = await import("../src/widgets/portfolio/PortfolioFeeTierPanel");
  let previous: ReturnType<typeof quoterMarketReads> | undefined;
  for (const marketId of [81n, 82n]) {
    for (let pass = 0; pass < 2; pass++) {
      const observed = quoterMarketReads(marketId);
      assert.notEqual(observed, previous);
      if (previous) assert.notEqual(observed.config, previous.config);
      previous = observed;
      const expected = [calls.buildGetMarketConfig(marketId), calls.buildGetMarginTiers(marketId), calls.buildGetMarkPriceDetails(marketId)];
      for (const [index, descriptor] of Object.values(observed).entries()) {
        assert.deepEqual(descriptor, expected[index]);
        assert.equal(descriptor.address, PRECOMPILES.perps);
        assert.equal(descriptor.abi, PerpsPrecompileABI);
        const encoded = encodeFunctionData(descriptor);
        const decoded = decodeFunctionData({ abi: descriptor.abi, data: encoded });
        assert.equal(decoded.functionName, descriptor.functionName);
        assert.deepEqual(decoded.args, [marketId]);
        assert.throws(() => decodeFunctionData({ abi: descriptor.abi, data: encoded.slice(0, 12) as `0x${string}` }));
      }
    }
  }
  for (const invalid of [-1n, 1n << 256n]) {
    for (const build of [quoterMarketReads, calls.buildGetMarketConfig, calls.buildGetMarginTiers, calls.buildGetMarkPriceDetails]) {
      assert.throws(() => build(invalid), (error: unknown) => {
        assert.equal((error as { code: string }).code, "VALIDATION_AMOUNT_INVALID");
        return true;
      });
    }
  }
  for (let pass = 0; pass < 2; pass++) {
    const descriptor = calls.buildGetFeeTier(ACCOUNT);
    assert.equal(descriptor.address, PRECOMPILES.perps);
    const request = decodeFunctionData({ abi: descriptor.abi, data: encodeFunctionData(descriptor) });
    assert.equal(request.functionName, "getFeeTier");
    assert.deepEqual(request.args, [ACCOUNT]);
  }
  // This result is emitted by viem's real ABI encoder, and is decoded exactly
  // like useFeeTier before the actual portfolio consumer renders its view.
  for (const tier of [2, 3]) {
    for (let pass = 0; pass < 2; pass++) {
      const descriptor = calls.buildGetFeeTier(ACCOUNT);
      const data = encodeFunctionResult({ abi: descriptor.abi, functionName: descriptor.functionName,
        result: [tier, 250n, -50n, 123n * WAD] });
      const decoded = decodeFunctionResult({ abi: descriptor.abi, functionName: descriptor.functionName, data });
      const view = feeTierView(decoded);
      const html = renderToStaticMarkup(createElement(PortfolioFeeTierPanel, { address: undefined, view }));
      assert.ok(html.includes(`Tier ${tier} · Taker 2.5 bps / Maker -0.5 bps`));
      assert.equal(view.volume14dUsd, 123n * WAD);
      assert.throws(() => decodeFunctionResult({ abi: descriptor.abi, functionName: descriptor.functionName, data: "0x01" }));
    }
  }
  for (const [account, code] of [[zeroAddress, "WALLET_NOT_CONNECTED"], ["0xbroken", "VALIDATION_AMOUNT_INVALID"]] as const) {
    assert.throws(() => calls.buildGetFeeTier(account as Address), (error: unknown) => {
      assert.equal((error as { code: string }).code, code);
      return true;
    });
  }
}

async function quoteAndPlanReachConfirmationAndSnapshotConsumers() {
  const { decodeOpenPositionQuote } = await import("../src/hooks/perps/useQuoter");
  const { buildOrderPlan, buildConfirmation } = await import("../src/core/perps/orderPlan");
  const { planOrder } = await import("../src/core/perps/positionWrites");
  const { computePositionPnl } = await import("../src/core/perps/pnlHelpers");
  const { snapshotPrice } = await import("../src/widgets/analytics/MarketSnapshotTable");
  const { loadPerpsOrderForm } = await import("../src/page-modules/perps");
  const { PerpsOrderForm, orderLimitsFromConfig } = await import("../src/widgets/trading/PerpsOrderForm");
  const { decodeMarketConfig } = await import("../src/core/perps/marketConfig");
  const calls = await import("../src/core/perps/calls");
  const configRead = calls.buildGetMarketConfig(81n);
  const tierRead = calls.buildGetMarginTiers(81n);
  const config = decodeMarketConfig(decodeFunctionResult({ ...configRead,
    data: encodeFunctionResult({ ...configRead, result: { ...auditMarketConfig, marketId: 81n, symbol: "PROOF" } }),
  }));
  const tiers = decodeFunctionResult({ ...tierRead,
    data: encodeFunctionResult({ ...tierRead, result: auditMarginTiers }),
  });
  assert.equal(tiers[0].maxLeverage, 20n, "ABI tier leverage is an unscaled integer");
  assert.equal((await loadPerpsOrderForm()).default, PerpsOrderForm,
    "the page's actual Next loader reaches the form that imports buildOrderPlan");
  const state: OrderFormState = {
    orderType: "market", marketId: 81n, isLong: true, collateralAmount: "100", leverage: "10",
    triggerPrice: "", limitPrice: "", sizeUsd: "", scaledStartPrice: "", scaledEndPrice: "",
    scaledTotalOrders: "2", scaledDistribution: "flat", tpslEnabled: false, tpPrice: "", slPrice: "",
    reduceOnly: false, postOnly: false, ioc: false, fok: false, ladderId: 1n,
  };
  const limits = orderLimitsFromConfig({ config, tiers }, 1000n * WAD);
  assert.ok(limits);
  assert.equal(limits.maxLeverage, 20n * WAD);
  assert.equal(orderLimitsFromConfig({ config: { ...config, enabled: false }, tiers }, 1000n * WAD), null);
  const quoteState: OpenPositionQuote = {
    entryPrice: 100n * WAD, sizeUsd: 1000n * WAD, collateralUsd: 100n * WAD, leverage: 10n * WAD,
    tradingFee: 1_000_000n, tradingFeeUsd: WAD, priceImpact: 0n, liquidationPrice: 91n * WAD,
    estimatedFunding24h: 0n, maintenanceMarginBps: 100n,
  };
  const position: ParsedPosition = {
    positionId: 1n, marketId: 81, symbol: "PROOF", isLong: true,
    sizeUsd: quoteState.sizeUsd, collateralUsd: quoteState.collateralUsd, leverage: quoteState.leverage,
    entryPrice: quoteState.entryPrice, collateralToken: ACCOUNT, collateralAmount: 100_000_000n,
    fundingPaidUsd: 0n, feesUsd: WAD, openedAt: undefined, openTxHash: undefined,
  };
  const boundaryState: OrderFormState = { ...state, orderType: "limit", leverage: "20", sizeUsd: "1000", triggerPrice: "100" };
  const boundary = buildOrderPlan(boundaryState, null, limits);
  assert.equal(boundary.error, null);
  const boundaryCall = boundary.calls[0];
  assert.ok(boundaryCall);
  const boundaryWire = decodeFunctionData({ abi: boundaryCall.abi, data: encodeFunctionData(boundaryCall) });
  assert.equal(boundaryWire.functionName, "placeOrder");
  if (boundaryWire.functionName !== "placeOrder") throw new Error("Expected placeOrder");
  assert.equal(boundaryWire.args[0].leverage, 20n * WAD);
  const above = buildOrderPlan({ ...boundaryState, leverage: "20.000000000000000001" }, null, limits);
  assert.deepEqual(above.calls, []);
  assert.equal(above.error?.code, "VALIDATION_ABOVE_MAX");
  const higherTierLimits = orderLimitsFromConfig({ config, tiers }, 10_000n * WAD);
  assert.ok(higherTierLimits);
  assert.equal(higherTierLimits.maxLeverage, 10n * WAD);
  const marketCapLimits = orderLimitsFromConfig({ config: { ...config, maxLeverage: 5n }, tiers }, 1000n * WAD);
  assert.ok(marketCapLimits);
  assert.equal(marketCapLimits.maxLeverage, 5n * WAD);
  for (const entryPrice of [100n * WAD, 120n * WAD]) {
    for (let pass = 0; pass < 2; pass++) {
      const quote = decodeOpenPositionQuote({ ...quoteState, entryPrice });
      const plan = buildOrderPlan(state, quote, limits);
      assert.equal(plan.error, null);
      assert.equal(plan.calls.length, 1);
      const dispatched = await planOrder(plan.calls);
      assert.equal(dispatched, plan.calls, "the submit planner consumes the actual produced calls");
      const call = dispatched[0];
      const wire = encodeFunctionData(call);
      const decoded = decodeFunctionData({ abi: call.abi, data: wire });
      assert.equal(decoded.functionName, "placeOrder");
      assert.deepEqual(decoded.args, call.args);
      const confirmation = buildConfirmation(quote, plan);
      assert.ok(confirmation);
      assert.equal(confirmation.display.size, "1000.00 USD");
      assert.equal(confirmation.display.fee, "1.00 USD");
      assert.equal(confirmation.display.acceptablePrice, entryPrice === 100n * WAD ? "100.5" : "120.6");
      assert.equal(confirmation.side, "long");
      const pnl = computePositionPnl(position, entryPrice, tiers);
      assert.equal(pnl.unrealizedPnl, entryPrice === 100n * WAD ? 0n : 200n * WAD);
      assert.equal(pnl.remainingCollateral, 99n * WAD + pnl.unrealizedPnl);
      assert.equal(snapshotPrice(pnl.markPrice.toString()), entryPrice === 100n * WAD ? "$100.0000" : "$120.0000");
    }
  }
  for (const malformed of [{ ...quoteState, entryPrice: "100" }, { ...quoteState, sizeUsd: -1n }, { ...quoteState, maintenanceMarginBps: 10_001n }, {}]) {
    assert.throws(() => decodeOpenPositionQuote(malformed), { name: "ZodError" });
    const refused = buildOrderPlan(state, malformed as OpenPositionQuote, limits);
    assert.deepEqual(refused.calls, []);
    assert.equal(refused.error?.code, "API_PAYLOAD_INVALID");
    assert.equal(buildConfirmation(null, refused), null);
  }
  const mismatch = buildOrderPlan(state, { ...quoteState, leverage: 11n * WAD }, limits);
  assert.deepEqual(mismatch.calls, []);
  assert.equal(mismatch.error?.code, "VALIDATION_AMOUNT_INVALID");
  const missing = buildOrderPlan(state, null, limits);
  assert.deepEqual(missing.calls, []);
  assert.equal(missing.error?.code, "VALIDATION_REQUIRED_FIELD");
  assert.throws(() => computePositionPnl({ ...position, sizeUsd: -1n }, WAD, tiers), /Invalid position PnL input/);
  assert.throws(() => computePositionPnl(position, WAD, []), /No margin tier/);
  for (const malformed of ["NaN", "1e18", "-1", "0x10"]) {
    assert.throws(() => snapshotPrice(malformed), /Invalid snapshot price/);
  }
  assert.equal(snapshotPrice("9007199254740993123450000000000000"), "$9,007,199,254,740,993.12");
  assert.equal(snapshotPrice(null), "—");
  assert.equal(snapshotPrice(computePositionPnl(position, 130n * WAD, tiers).markPrice.toString()), "$130.0000");
  assert.equal(buildOrderPlan(state, decodeOpenPositionQuote(quoteState), limits).error, null);
}

async function classifiedPrecompileErrorsReachAlertsConsumer() {
  const { PRECOMPILE_ERROR_CODES, classifyError, makeError } = await import("../src/core/errors/classify");
  const { PerpsPrecompileABI } = await import("../src/core/network/abi/PerpsPrecompile");
  const { alertErrorText } = await import("../src/widgets/alerts/AlertsPanel");
  for (const errorName of ["PerpsPaused", "PerpsMarketDisabled", "PerpsSlippageExceeded"] as const) {
    for (let pass = 0; pass < 2; pass++) {
      const data = encodeErrorResult({ abi: PerpsPrecompileABI, errorName });
      const classified = classifyError({ message: "execution reverted", cause: { data } }, "alerts/create");
      assert.equal(classified.code, PRECOMPILE_ERROR_CODES[errorName].code);
      assert.equal(classified.code, errorName === "PerpsSlippageExceeded" ? "TRADE_SLIPPAGE_EXCEEDED" : errorName === "PerpsMarketDisabled" ? "TRADE_MARKET_HALTED" : "RPC_REVERT");
      assert.equal(classified.context, "alerts/create");
      const text = alertErrorText(classified);
      assert.ok(text.startsWith(classified.message));
      if (classified.suggestedFix) assert.ok(text.includes(classified.suggestedFix));
      assert.ok(classified.fingerprint?.includes(errorName));
    }
  }
  const malformed = classifyError({ message: "execution reverted", data: "0x1234" }, "alerts/create");
  assert.equal(malformed.code, "RPC_REVERT", "malformed ABI data cannot claim a decoded contract error");
  assert.equal(malformed.fingerprint?.includes("PerpsPaused"), false);
  assert.ok(alertErrorText(malformed).length > 0);
  for (let pass = 0; pass < 2; pass++) {
    const produced = makeError("API_PAYLOAD_INVALID", "quoter/decode");
    const observed = classifyError(produced, "alerts/create");
    assert.equal(observed.code, "API_PAYLOAD_INVALID");
    assert.equal(observed.context, "alerts/create");
    assert.equal(alertErrorText(observed), `${produced.message} ${produced.suggestedFix}`);
    assert.equal(classifyError(observed), observed, "classification preserves an existing result");
  }
  assert.equal(classifyError({ code: "NOT_A_CATALOG_CODE", title: "invalid", message: "invalid" }).code, "UNKNOWN");
}

test(coreCatalogReachesMobileTradingAndAnalyticsConsumers);
test(readBuildersReachQuoterAndFeeTierConsumer);
test(quoteAndPlanReachConfirmationAndSnapshotConsumers);
test(classifiedPrecompileErrorsReachAlertsConsumer);
