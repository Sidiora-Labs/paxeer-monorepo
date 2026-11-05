"use client";

import { exchange as perpsVenue } from "@paxeer/wallet";
import { useRouter } from "next/navigation";
import { useEffect, useRef, useState, useTransition } from "react";

import {
  perpsCall,
  perpsCancelOrderCall,
  perpsClosePositionCall,
  perpsDepositCall,
  perpsOpenPositionCall,
  perpsWithdrawCall,
  type PrecompileCall,
  type PerpsOpenPosition,
  type PerpsWrite,
} from "../../api/sdk";
import { isPrecompileSendOutcome, sendWalletPrecompileCall, walletFeePreference, walletSendFailure, retainedWalletNativeSubmission, resumeWalletNativeSubmission, type NativeSubmission, type PrecompileSendOutcome } from "../../api/wallet";
import { windowWalletProvider } from "../../api/provider";
import { SegmentedControl } from "../../kit/collection";
import { KitButton } from "../../kit/control";
import { ExplorerPanel } from "../../kit/explorer";
import { TextField } from "../../kit/field";
import { availability } from "../_markets/availability";
import { WEI_DECIMALS, formatUnits, parseUnits } from "../_markets/format";
import { ChoiceField } from "../_markets/select";
import { SendOutcome } from "../_markets/send-outcome";
import { SPONSORED_APPROVE_UNSUPPORTED } from "../launchpad/launchpad-client";
import { preflightPerpsWrite } from "./actions";

export type PerpsCollateral = Readonly<{ token: string; decimals: number; symbol: string }>;
type Choice = Readonly<{ id: string; label: string }>;

export type PerpsSendOutcome = PrecompileSendOutcome & Readonly<{ stage?: "approval" }>;

export async function sendPerpsWrite(
  from: string,
  collateral: PerpsCollateral,
  pull: bigint,
  call: PrecompileCall,
  onApproving: (active: boolean) => void,
  operation?: string,
  onApproval: (outcome: PrecompileSendOutcome) => void = () => {},
  isCurrent: () => boolean = () => true,
): Promise<PerpsSendOutcome> {
  const wallet = windowWalletProvider();
  if (wallet === undefined) return { outcome: "unavailable" };
  try {
    if (!isCurrent()) return { outcome: "rejected" };
    if (operation !== undefined) {
      const retained = retainedWalletNativeSubmission(from);
      if (retained !== undefined) {
        const recovered = await resumeWalletNativeSubmission(from);
        if (recovered !== undefined) {
          if (retained.stage === "write") return recovered;
          onApproval(recovered);
          if (!(recovered.outcome === "sent" && recovered.receipt !== undefined))
            return { ...recovered, stage: "approval" };
        }
      }
    }
    const approve = pull > 0n ? await perpsVenue(wallet).approval(from, collateral.token, pull) : undefined;
    if (approve !== undefined) {
      if (walletFeePreference().currency === "sidiora")
        return { outcome: "failed", detail: SPONSORED_APPROVE_UNSUPPORTED };
      if (!isCurrent()) return { outcome: "rejected" };
      onApproving(true);
      const approved = await sendWalletPrecompileCall(from, () => approve, undefined,
        operation === undefined ? undefined : { operation, stage: "approval" });
      onApproval(approved);
      if (!isPrecompileSendOutcome(approved) || approved.outcome !== "sent" || approved.receipt === undefined)
        return { ...approved, stage: "approval" };
    }
    if (!isCurrent()) return { outcome: "rejected" };
    const preflight = await preflightPerpsWrite(from, call.data);
    if (!preflight.ok) return { outcome: "failed", detail: preflight.detail };
    if (!isCurrent()) return { outcome: "rejected" };
    return await sendWalletPrecompileCall(from, () => call, undefined,
      operation === undefined ? undefined : { operation, stage: "write" });
  } catch (error) {
    return walletSendFailure(error);
  } finally {
    onApproving(false);
  }
}

export function perpsMarketPositionCall(marketId: bigint, collateral: PerpsCollateral, amount: bigint, leverage: bigint, isLong: boolean): PrecompileCall {
  const order: PerpsOpenPosition = {
    marketId,
    collateralToken: collateral.token,
    collateralAmount: amount,
    leverage,
    isLong,
  };
  return perpsOpenPositionCall(order);
}

export function perpsOrderCancelCall(orderId: bigint): PrecompileCall {
  return perpsCancelOrderCall(orderId);
}

export type PerpsOrderInputs = Readonly<{
  market: string;
  side: string;
  orderType: string;
  triggerPrice: string;
  limitPrice: string;
  size: string;
  leverage: string;
  collateral: string;
  closeSize: string;
  takeProfit: string;
  stopLoss: string;
  positionCollateral: string;
}>;

function perpsId(text: string): bigint | undefined {
  if (!/^(0|[1-9][0-9]*)$/u.test(text)) return undefined;
  const id = BigInt(text);
  return id < 1n << 256n ? id : undefined;
}

function perpsAmount(text: string, decimals: number): bigint | undefined {
  const amount = parseUnits(text, decimals);
  return amount !== undefined && amount > 0n && amount < 1n << 256n ? amount : undefined;
}

export function ExchangeOrderControls({
  mode,
  inputs,
  onChange,
  collateral,
  markets,
  positionId,
  pending,
  walletReason,
  send,
}: Readonly<{
  mode: "entry" | "position";
  inputs: PerpsOrderInputs;
  onChange: (key: keyof PerpsOrderInputs, value: string) => void;
  collateral: PerpsCollateral;
  markets: readonly Choice[];
  positionId: string;
  pending: boolean;
  walletReason: string | undefined;
  send: (build: () => PrecompileCall, pull: bigint) => void;
}>) {
  const field = (label: string, key: keyof PerpsOrderInputs) => (
    <TextField
      label={label}
      inputMode="decimal"
      value={inputs[key]}
      onChange={(event) => onChange(key, event.target.value)}
    />
  );
  const write = (
    label: string,
    method: PerpsWrite,
    args: Parameters<typeof perpsCall>[1] | undefined,
    pull = 0n,
  ) => (
    <KitButton
      loading={pending}
      onClick={() => {
        if (args === undefined || walletReason !== undefined) return;
        send(() => perpsCall(method, args), pull);
      }}
      {...availability(walletReason, args === undefined && "Choose a market or position and enter valid positive amounts.")}
    >
      {label}
    </KitButton>
  );

  if (mode === "entry") {
    const market = perpsId(inputs.market);
    const price = perpsAmount(inputs.triggerPrice, WEI_DECIMALS);
    const limit = perpsAmount(inputs.limitPrice, WEI_DECIMALS);
    const size = perpsAmount(inputs.size, WEI_DECIMALS);
    const leverage = perpsAmount(inputs.leverage, WEI_DECIMALS);
    const amount = perpsAmount(inputs.collateral, collateral.decimals);
    const valid =
      market !== undefined && markets.some((choice) => choice.id === inputs.market) &&
      price !== undefined && size !== undefined && leverage !== undefined && amount !== undefined &&
      (inputs.side === "long" || inputs.side === "short");
    return (
      <div className="flex flex-col gap-3">
        <ChoiceField
          label="Order market"
          value={inputs.market}
          onChange={(value) => onChange("market", value)}
          options={markets.map((market) => ({ value: market.id, label: market.label }))}
        />
        <ChoiceField
          label="Order type"
          value={inputs.orderType}
          onChange={(value) => onChange("orderType", value)}
          options={[{ value: "limit", label: "Limit" }, { value: "stop-limit", label: "Stop-limit" }]}
        />
        <SegmentedControl
          aria-label="Order side"
          value={inputs.side}
          onValueChange={(value) => onChange("side", value)}
          options={[{ value: "long", label: "Long" }, { value: "short", label: "Short" }]}
        />
        {field("Trigger price", "triggerPrice")}
        {inputs.orderType === "stop-limit" ? field("Limit price", "limitPrice") : null}
        {field("Order size (USD)", "size")}
        <p>Size is USD notional, not a base-asset quantity or {collateral.symbol} collateral.</p>
        {size !== undefined ? <p role="note">Review: order notional {formatUnits(size, WEI_DECIMALS)} USD.</p> : null}
        {field("Order leverage (x)", "leverage")}
        {field(`Order collateral (${collateral.symbol})`, "collateral")}
        {inputs.orderType === "limit"
          ? write(
              "Place limit order",
              "placeLimitOrder(uint256,bool,uint256,uint256,uint256,address,uint256)",
              valid ? [market, inputs.side === "long", price, size, leverage, collateral.token, amount] : undefined,
              amount,
            )
          : write(
              "Place stop-limit order",
              "placeStopLimitOrder(uint256,bool,uint256,uint256,uint256,uint256,address,uint256)",
              valid && inputs.orderType === "stop-limit" && limit !== undefined
                ? [market, inputs.side === "long", price, limit, size, leverage, collateral.token, amount]
                : undefined,
              amount,
            )}
      </div>
    );
  }

  const position = perpsId(positionId);
  const closeSize = perpsAmount(inputs.closeSize, WEI_DECIMALS);
  const takeProfit = perpsAmount(inputs.takeProfit, WEI_DECIMALS);
  const stopLoss = perpsAmount(inputs.stopLoss, WEI_DECIMALS);
  const amount = perpsAmount(inputs.positionCollateral, collateral.decimals);
  return (
    <>
      {field("Close size (USD)", "closeSize")}
      <p>Close size is USD notional, separate from {collateral.symbol} collateral.</p>
      {closeSize !== undefined ? <p role="note">Review: close notional {formatUnits(closeSize, WEI_DECIMALS)} USD.</p> : null}
      {write("Partially close position", "partialClose(uint256,uint256)",
        position !== undefined && closeSize !== undefined ? [position, closeSize] : undefined)}
      {field("Take-profit price", "takeProfit")}
      {write("Place take-profit order", "placeTakeProfitOrder(uint256,uint256)",
        position !== undefined && takeProfit !== undefined ? [position, takeProfit] : undefined)}
      {field("Stop-loss price", "stopLoss")}
      {write("Place stop-loss order", "placeStopLossOrder(uint256,uint256)",
        position !== undefined && stopLoss !== undefined ? [position, stopLoss] : undefined)}
      {write("Set position TP/SL", "setPositionTpSl(uint256,uint256,uint256)",
        position !== undefined && takeProfit !== undefined && stopLoss !== undefined ? [position, takeProfit, stopLoss] : undefined)}
      {field(`Position collateral (${collateral.symbol})`, "positionCollateral")}
      {write("Add collateral", "addCollateral(uint256,uint256)",
        position !== undefined && amount !== undefined ? [position, amount] : undefined, amount)}
      {write("Remove collateral", "removeCollateral(uint256,uint256)",
        position !== undefined && amount !== undefined ? [position, amount] : undefined)}
    </>
  );
}

function AccountExchangeActions({
  account,
  collateral,
  markets,
  positions,
  orders,
}: Readonly<{
  account: string | undefined;
  collateral: PerpsCollateral;
  markets: readonly Choice[];
  positions: readonly Choice[];
  orders: readonly Choice[];
}>) {
  const router = useRouter();
  const [pending, startTransition] = useTransition();
  const [approving, setApproving] = useState(false);
  const [marketSelection, setMarketId] = useState(markets[0]?.id ?? "");
  const [side, setSide] = useState("long");
  const [openCollateralText, setOpenCollateralText] = useState("");
  const [leverageText, setLeverageText] = useState("");
  const [openOutcome, setOpenOutcome] = useState<PrecompileSendOutcome | undefined>(undefined);
  const [positionSelection, setPositionId] = useState(positions[0]?.id ?? "");
  const [closeOutcome, setCloseOutcome] = useState<PrecompileSendOutcome | undefined>(undefined);
  const [orderSelection, setOrderId] = useState(orders[0]?.id ?? "");
  const [cancelOutcome, setCancelOutcome] = useState<PrecompileSendOutcome | undefined>(undefined);
  const [marginText, setMarginText] = useState("");
  const [marginOutcome, setMarginOutcome] = useState<PrecompileSendOutcome | undefined>(undefined);

  const [orderInputs, setOrderInputs] = useState<PerpsOrderInputs>({
    market: markets[0]?.id ?? "",
    side: "long",
    orderType: "limit",
    triggerPrice: "",
    limitPrice: "",
    size: "",
    leverage: "",
    collateral: "",
    closeSize: "",
    takeProfit: "",
    stopLoss: "",
    positionCollateral: "",
  });
  const [orderOutcome, setOrderOutcome] = useState<PrecompileSendOutcome | undefined>(undefined);
  const select = (value: string, choices: readonly Choice[]) =>
    choices.some((choice) => choice.id === value) ? value : choices[0]?.id ?? "";
  const marketId = select(marketSelection, markets);
  const positionId = select(positionSelection, positions);
  const orderId = select(orderSelection, orders);
  const selectedOrderMarket = select(orderInputs.market, markets);
  if (marketId !== marketSelection) setMarketId(marketId);
  if (positionId !== positionSelection) setPositionId(positionId);
  if (orderId !== orderSelection) setOrderId(orderId);
  if (selectedOrderMarket !== orderInputs.market)
    setOrderInputs({ ...orderInputs, market: selectedOrderMarket });

  const latest = useRef({ account, markets, positions, orders, mounted: true });
  latest.current = { account, markets, positions, orders, mounted: true };
  const sending = useRef(false);
  const [recovering, setRecovering] = useState(true);
  const [retained, setRetained] = useState<NativeSubmission | undefined>();
  const [recoveryError, setRecoveryError] = useState<string | undefined>();
  const [approvalOutcome, setApprovalOutcome] = useState<PrecompileSendOutcome | undefined>();

  useEffect(() => {
    let stopped = false;
    let polling = false;
    async function reconcile() {
      if (polling || sending.current || stopped) return;
      polling = true;
      try {
        if (account === undefined) return;
        const existing = retainedWalletNativeSubmission(account);
        if (existing === undefined) return;
        const outcome = await resumeWalletNativeSubmission(account);
        if (stopped) return;
        if (existing.stage === "approval") setApprovalOutcome(outcome);
        else {
          const records: Record<string, typeof setOpenOutcome> = { open: setOpenOutcome, close: setCloseOutcome, cancel: setCancelOutcome,
            margin: setMarginOutcome, order: setOrderOutcome };
          records[existing.operation]?.(outcome);
        }
        if (outcome !== undefined && "receipt" in outcome && outcome.receipt !== undefined)
          router.refresh();
      } catch (error) {
        if (!stopped) setRecoveryError(error instanceof Error ? error.message : "Native recovery failed.");
      } finally {
        if (!stopped) {
          try { setRetained(account === undefined ? undefined : retainedWalletNativeSubmission(account)); }
          catch (error) { setRecoveryError(error instanceof Error ? error.message : "Native recovery failed."); }
          setRecovering(false);
        }
        polling = false;
      }
    }
    void reconcile();
    const timer = setInterval(() => { void reconcile(); }, 1000);
    return () => { stopped = true; latest.current.mounted = false; clearInterval(timer); };
  }, [account, router]);

  const changeOrderInput = (key: keyof PerpsOrderInputs, value: string) => {
    setOrderInputs((previous) => ({ ...previous, [key]: value }));
  };

  const openCollateral = perpsAmount(openCollateralText, collateral.decimals);
  const leverage = perpsAmount(leverageText, WEI_DECIMALS);
  const openMarket = markets.some((choice) => choice.id === marketId) ? perpsId(marketId) : undefined;
  const cancelOrder = orders.some((choice) => choice.id === orderId) ? perpsId(orderId) : undefined;
  const margin = parseUnits(marginText, collateral.decimals);
  const walletReason = account === undefined ? "Connect a wallet first."
    : recoveryError !== undefined ? recoveryError
    : recovering ? "Checking retained transactions."
    : retained !== undefined ? "An existing transaction is pending reconciliation."
    : pending ? "A transaction is in progress." : undefined;

  const send = (build: () => PrecompileCall | Promise<PrecompileCall>, pull: bigint, record: (outcome: PrecompileSendOutcome) => void) => {
    if (account === undefined || walletReason !== undefined || sending.current) return;
    const operation = record === setOpenOutcome ? "open" : record === setCloseOutcome ? "close"
      : record === setCancelOutcome ? "cancel" : record === setOrderOutcome ? "order" : "margin";
    const kind = operation === "open" || operation === "order" ? "markets"
      : operation === "close" ? "positions" : operation === "cancel" ? "orders" : undefined;
    const id = operation === "open" ? marketId : operation === "order" ? orderInputs.market
      : operation === "close" ? positionId : orderId;
    const isCurrent = () => latest.current.mounted && latest.current.account === account &&
      (kind === undefined || latest.current[kind].some((choice) => choice.id === id));
    if (!isCurrent()) return;
    sending.current = true;
    startTransition(async () => {
      try {
        const call = await build();
        const outcome = await sendPerpsWrite(account, collateral, pull, call, setApproving,
          operation, setApprovalOutcome, isCurrent);
        if (!latest.current.mounted) return;
        if (outcome.stage !== "approval") record(outcome);
        if (isPrecompileSendOutcome(outcome) && outcome.outcome === "sent" &&
          outcome.stage !== "approval" && outcome.receipt !== undefined) router.refresh();
      } catch (error) {
        if (latest.current.mounted) record(walletSendFailure(error));
      } finally {
        sending.current = false;
        if (latest.current.mounted) {
          try { setRetained(retainedWalletNativeSubmission(account)); }
          catch (error) { setRecoveryError(error instanceof Error ? error.message : "Native recovery failed."); }
        }
      }
    });
  };

  return (
    <div className="grid gap-4 lg:grid-cols-2">
      {approvalOutcome !== undefined ? <section aria-label="Collateral approval">
        <h2>Collateral approval</h2>
        <SendOutcome outcome={approvalOutcome} />
      </section> : null}
      {retained !== undefined ? <p role="status">Pending {retained.stage} for {retained.operation}.
        {retained.transactionHash ?? " Awaiting the wallet transaction identity."}</p> : null}
      {recoveryError !== undefined ? <p role="alert">{recoveryError}</p> : null}
      <ExplorerPanel title="Open a position">
        <div className="flex flex-col gap-3">
          {markets.length === 0 ? (
            <p className="text-sm text-muted-foreground">No market is listed.</p>
          ) : (
            <ChoiceField
              label="Market"
              value={marketId}
              onChange={setMarketId}
              options={markets.map((market) => ({ value: market.id, label: market.label }))}
            />
          )}
          <SegmentedControl
            aria-label="Side"
            value={side}
            onValueChange={setSide}
            options={[
              { value: "long", label: "Long" },
              { value: "short", label: "Short" },
            ]}
          />
          <TextField
            label={`Collateral (${collateral.symbol})`}
            inputMode="decimal"
            value={openCollateralText}
            onChange={(event) => {
              setOpenCollateralText(event.target.value);
            }}
          />
          <TextField
            label="Leverage (x)"
            inputMode="decimal"
            value={leverageText}
            onChange={(event) => {
              setLeverageText(event.target.value);
            }}
          />
          <KitButton
            loading={pending}
            onClick={() => {
              if (walletReason !== undefined || openMarket === undefined || openCollateral === undefined || leverage === undefined) {
                return;
              }
              send(
                () => perpsMarketPositionCall(openMarket, collateral, openCollateral, leverage, side === "long"),
                openCollateral,
                setOpenOutcome,
              );
            }}
            {...availability(
              walletReason,
              (openMarket === undefined || openCollateral === undefined || leverage === undefined) &&
                "Choose a market and enter collateral and leverage.",
            )}
          >
            Open position
          </KitButton>
          <SendOutcome outcome={openOutcome} />
        </div>
      </ExplorerPanel>
      <ExplorerPanel title="Place an order">
        <ExchangeOrderControls
          mode="entry"
          inputs={orderInputs}
          onChange={changeOrderInput}
          collateral={collateral}
          markets={markets}
          positionId={positionId}
          pending={pending}
          walletReason={walletReason}
          send={(build, pull) => send(build, pull, setOrderOutcome)}
        />
        <SendOutcome outcome={orderOutcome} />
      </ExplorerPanel>
      <ExplorerPanel title="Close a position">
        <div className="flex flex-col gap-3">
          {positions.length === 0 ? (
            <p className="text-sm text-muted-foreground">No open position.</p>
          ) : (
            <ChoiceField
              label="Position"
              value={positionId}
              onChange={setPositionId}
              options={positions.map((position) => ({ value: position.id, label: position.label }))}
            />
          )}
          <p className="text-xs text-muted-foreground">
            Closes the whole position with a reduce-only market order at the venue slippage bound.
          </p>
          <KitButton
            variant="destructive"
            loading={pending}
            onClick={() => {
              if (walletReason !== undefined || !positions.some((choice) => choice.id === positionId) || perpsId(positionId) === undefined) return;
              send(() => perpsClosePositionCall(BigInt(positionId)), 0n, setCloseOutcome);
            }}
            {...availability(walletReason, (!positions.some((choice) => choice.id === positionId) || perpsId(positionId) === undefined) && "Choose a position.")}
          >
            Close position
          </KitButton>
          <ExchangeOrderControls
            mode="position"
            inputs={orderInputs}
            onChange={changeOrderInput}
            collateral={collateral}
            markets={markets}
            positionId={positionId}
            pending={pending}
            walletReason={walletReason}
            send={(build, pull) => send(build, pull, setCloseOutcome)}
          />
          <SendOutcome outcome={closeOutcome} />
        </div>
      </ExplorerPanel>
      <ExplorerPanel title="Cancel an order">
        <div className="flex flex-col gap-3">
          {orders.length === 0 ? (
            <p className="text-sm text-muted-foreground">No active order.</p>
          ) : (
            <ChoiceField
              label="Order"
              value={orderId}
              onChange={setOrderId}
              options={orders.map((order) => ({ value: order.id, label: order.label }))}
            />
          )}
          <KitButton
            variant="destructive"
            loading={pending}
            onClick={() => {
              if (walletReason !== undefined || cancelOrder === undefined) return;
              send(() => perpsOrderCancelCall(cancelOrder), 0n, setCancelOutcome);
            }}
            {...availability(walletReason, cancelOrder === undefined && "Choose an order.")}
          >
            Cancel order
          </KitButton>
          <SendOutcome outcome={cancelOutcome} />
        </div>
      </ExplorerPanel>
      <ExplorerPanel title="Margin">
        <div className="flex flex-col gap-3">
          <TextField
            label={`Amount (${collateral.symbol})`}
            inputMode="decimal"
            value={marginText}
            onChange={(event) => {
              setMarginText(event.target.value);
            }}
          />
          <p className="text-xs text-muted-foreground">
            {approving
              ? `Approving the exact ${collateral.symbol} amount for the perps venue.`
              : `If the deposit allowance is short, one approve for the exact ${collateral.symbol} amount is sent first.`}
          </p>
          <KitButton
            loading={pending}
            onClick={() => {
              if (margin === undefined) {
                return;
              }
              send(() => perpsDepositCall(collateral.token, margin), margin, setMarginOutcome);
            }}
            {...availability(walletReason, (margin === undefined || margin === 0n) && "Enter an amount.")}
          >
            Deposit
          </KitButton>
          <KitButton
            variant="secondary"
            loading={pending}
            onClick={() => {
              if (margin === undefined) {
                return;
              }
              send(() => perpsWithdrawCall(collateral.token, margin), 0n, setMarginOutcome);
            }}
            {...availability(walletReason, (margin === undefined || margin === 0n) && "Enter an amount.")}
          >
            Withdraw
          </KitButton>
          <SendOutcome outcome={marginOutcome} />
        </div>
      </ExplorerPanel>
    </div>
  );
}

/** Reset account-owned selections, outcomes and in-flight callbacks on a wallet switch. */
export function ExchangeActions(props: Parameters<typeof AccountExchangeActions>[0]) {
  return <AccountExchangeActions key={props.account?.toLowerCase() ?? "disconnected"} {...props} />;
}
