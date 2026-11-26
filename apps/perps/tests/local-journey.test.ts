import assert from "node:assert/strict";
import { test, type TestContext } from "node:test";
import {
  isMainThread,
  parentPort,
  Worker,
  workerData,
} from "node:worker_threads";
import {
  BaseError,
  ContractFunctionRevertedError,
  TransactionReceiptNotFoundError,
  createPublicClient,
  createWalletClient,
  encodeFunctionData,
  erc20Abi,
  formatUnits,
  getAddress,
  http,
  isHex,
  parseEventLogs,
  size,
  type Address,
  type Hex,
  type TransactionReceipt,
} from "viem";
import { privateKeyToAccount } from "viem/accounts";
import WebSocket from "ws";

import { PerpsPrecompileABI } from "@paxeer-network/paxeer-backend/core/network/abi/PerpsPrecompile";
import type { DepositPlan } from "@paxeer-network/paxeer-backend/core/perps/calls";
import type {
  OrderFormState,
  OrderLimits,
  OrderPlan,
} from "@paxeer-network/paxeer-backend/core/perps/orderPlan";
import type { WritePlan } from "@paxeer-network/paxeer-backend/core/perps/positionWrites";
import type { OpenPositionQuote } from "@paxeer-network/paxeer-backend/hooks/perps/useQuoter";

const WAD = 10n ** 18n;
const USDL_UNIT = 10n ** 6n;
const USD_PER_TOKEN_UNIT = WAD / USDL_UNIT;

interface GatewayPosition {
  positionId: string;
  userAddress: string;
  marketId: number;
  isLong: boolean;
  sizeUsd: string;
  entryPrice: string;
  collateralUsd: string;
  collateralAmount: string;
  leverage: string;
  status: string;
  openTxHash: string;
  closeTxHash: string | null;
  exitPrice: string | null;
}

interface GatewayOrder {
  orderId: string;
  userAddress: string;
  marketId: number;
  isLong: boolean;
  sizeUsd: string;
  triggerPrice: string;
  status: string;
  placedTxHash: string;
  resolvedTxHash: string | null;
}

interface GatewayEvent {
  eventName: string;
  eventData: string;
  txHash: string;
  logIndex: number;
  blockNumber: number;
}

function environment(name: string): string {
  const value = process.env[name];
  assert.ok(value, `${name} is required`);
  return value;
}

function fixtureKey(name: string): Hex {
  const value = environment(name);
  assert.ok(isHex(value) && size(value) === 32, `${name} must be 32 bytes`);
  return value;
}

function loopback(raw: string): URL {
  const url = new URL(raw);
  assert.equal(url.protocol, "http:");
  assert.ok(["127.0.0.1", "localhost", "[::1]"].includes(url.hostname));
  assert.notEqual(url.port, "0");
  assert.equal(url.username, "");
  assert.equal(url.password, "");
  return url;
}

function json(value: unknown): string {
  return JSON.stringify(value, (_, item: unknown) =>
    typeof item === "bigint" ? item.toString() : item,
  );
}

async function formPlan(
  state: OrderFormState,
  quote: OpenPositionQuote | null,
  limits: OrderLimits,
): Promise<OrderPlan> {
  const worker = new Worker(
    `const { workerData } = require("node:worker_threads");
     require("tsx/cjs/api").register();
     import("tsx/esm/api").then(({ register }) => {
       register();
       return import(workerData.moduleUrl);
     });`,
    {
      eval: true,
      execArgv: [],
      workerData: { moduleUrl: import.meta.url, state, quote, limits },
    },
  );
  try {
    return await new Promise<OrderPlan>((resolve, reject) => {
      const timeout = setTimeout(
        () => reject(new Error("Order planning timed out")),
        30_000,
      );
      worker.once("message", (plan: OrderPlan) => {
        clearTimeout(timeout);
        resolve(plan);
      });
      worker.once("error", (error) => {
        clearTimeout(timeout);
        reject(error);
      });
      worker.once("exit", (code) => {
        clearTimeout(timeout);
        reject(
          new Error(`Order planner exited before returning its plan: ${code}`),
        );
      });
    });
  } finally {
    await worker.terminate();
  }
}

async function localTraderJourney(t: TestContext) {
  const rpc = loopback(environment("LOCAL_NODE_RPC")).href;
  const gateway = loopback(environment("NEXT_PUBLIC_PERPS_GATEWAY_URL"));
  process.env.NEXT_PUBLIC_PAXEER_RPC_URLS = rpc;
  const trader = privateKeyToAccount(fixtureKey("LOCAL_NODE_KEY"));
  const maker = privateKeyToAccount(fixtureKey("LOCAL_NODE_MAKER_KEY"));
  assert.notEqual(trader.address, maker.address);

  const { paxeerChain } =
    await import("@paxeer-network/paxeer-backend/core/chain");
  const calls = await import("@paxeer-network/paxeer-backend/core/perps/calls");
  const writes =
    await import("@paxeer-network/paxeer-backend/core/perps/positionWrites");
  const actions =
    await import("@paxeer-network/paxeer-backend/core/perps/positionActions");
  const { COLLATERAL_TOKENS, INDEXER_GRAPHQL } =
    await import("@paxeer-network/paxeer-backend/core/perps/config");
  const { gqlQuery } =
    await import("@paxeer-network/paxeer-backend/core/graphql/client");
  assert.equal(INDEXER_GRAPHQL.url, new URL("/graphql", gateway).href);
  const publicClient = createPublicClient({
    chain: paxeerChain,
    transport: http(rpc),
  });
  const wallet = createWalletClient({
    account: trader,
    chain: paxeerChain,
    transport: http(rpc),
  });
  const makerWallet = createWalletClient({
    account: maker,
    chain: paxeerChain,
    transport: http(rpc),
  });
  assert.equal(await publicClient.getChainId(), 125);
  const token = COLLATERAL_TOKENS.usdl.address;
  const perps = calls.buildDeposit(1n).address;
  const marketId = 11n;
  const config = await publicClient.readContract(
    calls.buildGetMarketConfig(marketId),
  );
  assert.equal(config.symbol, "PAX");
  assert.ok(config.enabled);
  // Base size uses 18-decimal units.
  assert.equal(config.lotSize, WAD);
  const leverage = 2n * WAD;
  // The chain cap is a plain integer; requested leverage uses 18-decimal units.
  assert.ok(config.maxLeverage * WAD >= leverage);
  const limits: OrderLimits = {
    tickSize: config.tickSize,
    lotSize: config.lotSize,
    maxLeverage: config.maxLeverage * WAD,
  };
  const native = new Set<Address>();
  const btcMarketId = 0n;
  const btcConfig = await publicClient.readContract(
    calls.buildGetMarketConfig(btcMarketId),
  );
  assert.equal(btcConfig.symbol, "BTC");
  assert.ok(btcConfig.enabled);

  const query = <T>(document: string, variables?: Record<string, unknown>) =>
    gqlQuery<T>(INDEXER_GRAPHQL, document, variables);
  const accountState = () =>
    publicClient.readContract(calls.buildGetAccount(trader.address));
  const tokenBalance = (owner = trader.address) =>
    publicClient.readContract({
      address: token,
      abi: erc20Abi,
      functionName: "balanceOf",
      args: [owner],
    });
  const allowance = (owner = trader.address) =>
    publicClient.readContract({
      address: token,
      abi: erc20Abi,
      functionName: "allowance",
      args: [owner, perps],
    });
  const initialWallet = await tokenBalance();
  const initialAccount = await accountState();
  // Position and order counts are plain integers.
  assert.equal(initialAccount.positionCount, 0n);
  assert.equal(initialAccount.openOrderCount, 0n);
  const startingWealth = initialWallet + initialAccount.collateral;
  let longId: bigint;
  let shortId: bigint;
  let limitId: bigint;
  let longOpenHash: Hex;
  let shortOpenHash: Hex;
  let limitHash: Hex;
  let secondId: bigint;
  let secondOpenHash: Hex;
  let feesUsd = 0n;
  let feeTokens = 0n;
  let grossPnlUsd = 0n;
  let fundingUsd = 0n;
  let settlementDelta = 0n;
  const traderHashes: Hex[] = [];
  let lastReceipt: TransactionReceipt | undefined;

  const positionFields = `positionId userAddress marketId isLong sizeUsd entryPrice collateralUsd collateralAmount
    leverage status openTxHash closeTxHash exitPrice`;
  const streams = new Map<
    Address,
    { socket: WebSocket; positions?: GatewayPosition[]; error?: Error }
  >();
  for (const owner of [trader.address, maker.address]) {
    const url = new URL("/graphql-ws", gateway);
    url.protocol = "ws:";
    const socket = new WebSocket(url, "graphql-transport-ws");
    const stream: {
      socket: WebSocket;
      positions?: GatewayPosition[];
      error?: Error;
    } = { socket };
    streams.set(owner, stream);
    t.after(() => socket.close());
    await new Promise<void>((resolve, reject) => {
      const timeout = setTimeout(
        () => reject(new Error("Position subscription timed out")),
        10_000,
      );
      socket.on("error", (error) => {
        stream.error = error;
        clearTimeout(timeout);
        reject(error);
      });
      socket.on("open", () => socket.send(json({ type: "connection_init" })));
      socket.on("message", (raw) => {
        const message = JSON.parse(raw.toString());
        if (message.type === "connection_ack") {
          socket.send(
            json({
              id: "positions",
              type: "subscribe",
              payload: {
                query: `subscription JourneyPositions($user: String!) { positionChanged(userAddress: $user) { ${positionFields} } }`,
                variables: { user: owner },
              },
            }),
          );
          socket.send(json({ type: "ping" }));
        } else if (message.type === "pong") {
          clearTimeout(timeout);
          resolve();
        } else if (message.type === "next" && !message.payload.errors) {
          stream.positions = message.payload.data.positionChanged;
        } else if (message.type === "error" || message.payload?.errors) {
          stream.error = new Error(json(message.payload));
          clearTimeout(timeout);
          reject(stream.error);
        }
      });
    });
  }

  async function accountReads(
    context: TestContext,
    receipt: TransactionReceipt,
  ) {
    for (const owner of [trader.address, maker.address]) {
      const chain = await publicClient.readContract(
        calls.buildGetAccount(owner),
      );
      const ids = await publicClient.readContract(
        calls.buildGetUserPositionIds(owner),
      );
      const positions = await Promise.all(
        ids.map(async (id) => ({
          id,
          position: writes.positionFromTuple(
            await publicClient.readContract(calls.buildGetPosition(id)),
          ),
          margin: await publicClient.readContract(
            calls.buildGetPositionMargin(id),
          ),
        })),
      );
      const data = await query<{
        userBalances: { token: string; balance: string; locked: string }[];
        positions: GatewayPosition[];
      }>(
        `query JourneyAccount($user: String!) {
        userBalances(address: $user) { token balance locked }
        positions(userAddress: $user, status: "open", limit: 500) { ${positionFields} }
      }`,
        { user: owner },
      );
      const balance = data.userBalances.find(
        (row) => getAddress(row.token) === getAddress(token),
      );
      const locked = positions.reduce(
        (sum, item) => sum + item.margin.lockedAmount,
        0n,
      );
      context.diagnostic(
        json({
          step: context.name,
          transaction: receipt.transactionHash,
          owner,
          chain: { ...chain, locked },
          gateway: data,
        }),
      );
      if (balance === undefined) {
        assert.equal(chain.collateral, 0n);
        assert.equal(locked, 0n);
        assert.equal(chain.positionCount, 0n);
      } else {
        assert.equal(
          BigInt(balance.balance),
          chain.collateral,
          "account collateral agrees",
        );
        assert.equal(
          BigInt(balance.locked),
          locked,
          "account position margin agrees",
        );
      }
      assert.equal(data.positions.length, ids.length);
      for (const item of positions) {
        const row = data.positions.find(
          (position) => BigInt(position.positionId) === item.id,
        );
        assert.ok(row);
        const { position } = await query<{ position: GatewayPosition }>(
          `query JourneyAccountPosition($id: String!) { position(positionId: $id) { ${positionFields} } }`,
          { id: item.id.toString() },
        );
        assert.deepEqual(
          position,
          row,
          "position list and individual read agree",
        );
        assert.equal(
          BigInt(row.sizeUsd),
          item.position.sizeUsd,
          "listed position size agrees",
        );
        assert.equal(
          BigInt(row.entryPrice),
          item.position.entryPrice,
          "listed entry price agrees",
        );
        assert.equal(
          BigInt(row.collateralUsd),
          item.position.collateralUsd,
          "listed position margin agrees",
        );
        assert.equal(
          BigInt(row.collateralAmount),
          item.position.collateralAmount,
          "listed position collateral agrees",
        );
      }
      const stream = streams.get(owner)!;
      const expected = data.positions
        .map((row) => ({
          id: row.positionId,
          size: row.sizeUsd,
          collateral: row.collateralAmount,
          margin: row.collateralUsd,
        }))
        .sort((a, b) => a.id.localeCompare(b.id));
      const observed = () =>
        stream.positions
          ?.filter((row) => row.status === "open")
          .map((row) => ({
            id: row.positionId,
            size: row.sizeUsd,
            collateral: row.collateralAmount,
            margin: row.collateralUsd,
          }))
          .sort((a, b) => a.id.localeCompare(b.id));
      if (positions.length > 0 || stream.positions !== undefined) {
        const deadline = Date.now() + 30_000;
        while (
          !stream.error &&
          json(observed()) !== json(expected) &&
          Date.now() < deadline
        ) {
          await new Promise((resolve) => setTimeout(resolve, 100));
        }
        assert.ifError(stream.error);
        assert.deepEqual(
          observed(),
          expected,
          "position stream and account list agree",
        );
      }
    }
  }

  async function waitReceipt(hash: Hex) {
    const deadline = Date.now() + 30_000;
    while (Date.now() < deadline) {
      try {
        return await publicClient.getTransactionReceipt({ hash });
      } catch (error) {
        if (
          !(error instanceof TransactionReceiptNotFoundError) &&
          !(
            error instanceof Error &&
            /not\s*(be\s*)?found|does not exist|indexing|not indexed/i.test(
              error.message,
            )
          )
        )
          throw error;
      }
      await new Promise((resolve) => setTimeout(resolve, 400));
    }
    throw new Error(`Receipt timed out: ${hash}`);
  }

  async function indexed(receipt: TransactionReceipt) {
    const deadline = Date.now() + 30_000;
    let height = -1;
    do {
      const data = await query<{
        indexerStatus: { lastIndexedBlock: number } | null;
      }>("query JourneyCursor { indexerStatus { lastIndexedBlock } }");
      if (data.indexerStatus) height = data.indexerStatus.lastIndexedBlock;
      if (BigInt(height) >= receipt.blockNumber) return;
      await new Promise((resolve) => setTimeout(resolve, 400));
    } while (Date.now() < deadline);
    assert.fail(
      `Gateway cursor ${height} did not reach transaction block ${receipt.blockNumber}`,
    );
  }

  type Write =
    | DepositPlan[number]
    | WritePlan[number]
    | ReturnType<typeof calls.buildCancelOrder>
    | ReturnType<typeof calls.buildSetPositionTpSl>
    | ReturnType<typeof calls.buildWithdrawAll>;

  async function submit(context: TestContext, call: Write, sender = wallet) {
    const abi = call.functionName === "approve" ? erc20Abi : PerpsPrecompileABI;
    const data =
      call.functionName === "approve"
        ? encodeFunctionData({
            abi: erc20Abi,
            functionName: call.functionName,
            args: call.args,
          })
        : encodeFunctionData({
            abi: PerpsPrecompileABI,
            functionName: call.functionName,
            args: call.args,
          });
    const hash = await sender.sendTransaction({
      to: call.address,
      data,
      gas: 2_000_000n,
      gasPrice: 1_000_000_000_000n,
      nonce: await publicClient.getTransactionCount({
        address: sender.account.address,
        blockTag: "latest",
      }),
    });
    const receipt = await waitReceipt(hash);
    context.diagnostic(
      json({
        step: context.name,
        call: call.functionName,
        args: call.args,
        transaction: hash,
        gas: receipt.gasUsed,
        status: receipt.status,
        block: receipt.blockNumber,
        events: parseEventLogs({ abi, logs: receipt.logs }),
      }),
    );
    if (receipt.status !== "success") {
      await publicClient.simulateContract({
        ...call,
        abi,
        account: sender.account,
        blockNumber: receipt.blockNumber,
      });
      assert.fail(`${call.functionName} reverted`);
    }
    // Gas is a plain integer.
    assert.ok(receipt.gasUsed > 0n && receipt.gasUsed <= 2_000_000n);
    await indexed(receipt);
    if (sender.account.address === trader.address) {
      lastReceipt = receipt;
      traderHashes.push(hash);
      for (const event of parseEventLogs({
        abi: PerpsPrecompileABI,
        logs: receipt.logs,
        eventName: "FeeCollected",
      })) {
        if (getAddress(event.args.user) !== trader.address) continue;
        feesUsd += event.args.feeUsd;
        feeTokens += event.args.feeTokens;
      }
      for (const event of parseEventLogs({
        abi: PerpsPrecompileABI,
        logs: receipt.logs,
        eventName: "TradeSettled",
      })) {
        if (getAddress(event.args.user) !== trader.address) continue;
        grossPnlUsd += event.args.grossPnl;
        fundingUsd += event.args.fundingPaidUsd;
        settlementDelta +=
          (event.args.grossPnl + event.args.fundingPaidUsd) /
          USD_PER_TOKEN_UNIT;
      }
    }
    await context.test(async function accountsAgreeThroughGateway() {
      await accountReads(context, receipt);
    });
    return receipt;
  }

  async function gatewayEvents(receipt: TransactionReceipt, name: string) {
    const result = await query<{ protocolEvents: GatewayEvent[] }>(
      `query JourneyEvents($name: String!) {
        protocolEvents(eventName: $name, limit: 500) { eventName eventData txHash logIndex blockNumber }
      }`,
      { name },
    );
    const events = result.protocolEvents.filter(
      (event) =>
        event.txHash.toLowerCase() === receipt.transactionHash.toLowerCase(),
    );
    const chainEvents = parseEventLogs({
      abi: PerpsPrecompileABI,
      logs: receipt.logs,
    }).filter((event) => event.eventName === name);
    assert.ok(chainEvents.length > 0, `${name} must be emitted`);
    assert.equal(
      events.length,
      chainEvents.length,
      `${name} count through the gateway`,
    );
    for (const event of events) {
      // Block heights and log indices are plain integers.
      assert.equal(BigInt(event.blockNumber), receipt.blockNumber);
      const chainEvent = chainEvents.find(
        (item) => item.logIndex === event.logIndex,
      );
      assert.ok(chainEvent);
      const decoded: Record<string, unknown> = JSON.parse(event.eventData);
      for (const [key, value] of Object.entries(chainEvent.args)) {
        if (typeof value === "bigint")
          assert.equal(BigInt(String(decoded[key])), value, `${name}.${key}`);
        else if (typeof value === "string")
          assert.equal(
            String(decoded[key]).toLowerCase(),
            value.toLowerCase(),
            `${name}.${key}`,
          );
        else if (typeof value === "number")
          assert.equal(Number(decoded[key]), value, `${name}.${key}`);
        else assert.equal(decoded[key], value, `${name}.${key}`);
      }
    }
    return events;
  }

  async function collateralRead(
    context: TestContext,
    receipt: TransactionReceipt,
    amount: bigint,
    deposit: boolean,
  ) {
    const name = deposit ? "CollateralDeposited" : "CollateralWithdrawn";
    const event = parseEventLogs({
      abi: PerpsPrecompileABI,
      logs: receipt.logs,
      eventName: name,
    }).find((item) => getAddress(item.args.account) === trader.address);
    assert.ok(event);
    // Collateral amounts and balances use 6-decimal token units.
    assert.equal(event.args.amount, amount);
    const chain = await accountState();
    assert.equal(event.args.newBalance, chain.collateral);
    const events = await gatewayEvents(receipt, name);
    context.diagnostic(
      json({
        step: context.name,
        chain,
        gateway: events,
        wallet: await tokenBalance(),
      }),
    );
  }

  async function positionRead(
    context: TestContext,
    id: bigint,
    openHash: Hex,
    expectedMarket = marketId,
  ) {
    const chain = writes.positionFromTuple(
      await publicClient.readContract(calls.buildGetPosition(id)),
    );
    const details = await publicClient.readContract(
      calls.buildGetPositionDetails(id),
    );
    const margin = await publicClient.readContract(
      calls.buildGetPositionMargin(id),
    );
    const { position } = await query<{ position: GatewayPosition | null }>(
      `query JourneyPosition($id: String!) { position(positionId: $id) {
        positionId userAddress marketId isLong sizeUsd entryPrice collateralUsd collateralAmount
        leverage status openTxHash closeTxHash exitPrice
      } }`,
      { id: id.toString() },
    );
    context.diagnostic(
      json({ step: context.name, chain, details, margin, gateway: position }),
    );
    assert.equal(chain.user, trader.address);
    // Market and position identifiers are plain integers.
    assert.equal(chain.marketId, expectedMarket);
    assert.equal(chain.active, true);
    // USD notional and requested leverage use 18-decimal units.
    assert.equal(chain.sizeUsd, details.sizeUsdAtMark);
    assert.equal(details.leverage, leverage);
    // Convert 18-decimal USD initial margin to 6-decimal collateral token units, rounding up.
    assert.equal(
      margin.lockedAmount,
      (chain.sizeUsd * WAD + leverage * USD_PER_TOKEN_UNIT - 1n) /
        (leverage * USD_PER_TOKEN_UNIT),
    );
    assert.ok(
      (
        await publicClient.readContract(
          calls.buildGetUserPositionIds(trader.address),
        )
      ).includes(id),
    );
    const tiers = await publicClient.readContract(
      calls.buildGetMarginTiers(expectedMarket),
    );
    // Tier lower bounds use 18-decimal USD; tier leverage caps are plain integers.
    const tier = tiers.findLast((item) => item.lowerBoundUsd <= chain.sizeUsd);
    assert.ok(tier);
    assert.ok(tier.maxLeverage * WAD >= details.leverage);
    await context.test(function positionAgreesThroughGateway() {
      assert.ok(position);
      // Position and market identifiers are plain integers.
      assert.equal(BigInt(position.positionId), id);
      assert.equal(getAddress(position.userAddress), chain.user);
      assert.equal(BigInt(position.marketId), chain.marketId);
      assert.equal(position.isLong, chain.isLong);
      assert.equal(position.status, "open");
      assert.equal(position.openTxHash.toLowerCase(), openHash.toLowerCase());
      // Gateway and chain notional use 18-decimal USD, price uses 18-decimal price units, and leverage uses 18 decimals.
      assert.equal(
        BigInt(position.sizeUsd),
        chain.sizeUsd,
        "position size agrees",
      );
      assert.equal(
        BigInt(position.entryPrice),
        chain.entryPrice,
        "entry price agrees",
      );
      assert.equal(
        BigInt(position.leverage),
        details.leverage,
        "leverage agrees",
      );
      assert.equal(
        BigInt(position.collateralUsd),
        chain.collateralUsd,
        "position margin agrees",
      );
      // Collateral uses 6-decimal token units on chain and through the gateway.
      assert.equal(
        BigInt(position.collateralAmount),
        chain.collateralAmount,
        "position collateral agrees",
      );
    });
    return { chain, details };
  }

  async function orderRead(id: bigint) {
    const result = await query<{ order: GatewayOrder | null }>(
      `query JourneyOrder($id: String!) { order(orderId: $id) {
        orderId userAddress marketId isLong sizeUsd triggerPrice status placedTxHash resolvedTxHash
      } }`,
      { id: id.toString() },
    );
    assert.ok(result.order);
    return result.order;
  }

  function state(
    orderType: "market" | "limit",
    isLong: boolean,
  ): OrderFormState {
    return {
      orderType,
      marketId,
      isLong,
      collateralAmount: "100",
      leverage: formatUnits(leverage, 18),
      triggerPrice: "0.5",
      limitPrice: "",
      sizeUsd: "20",
      scaledStartPrice: "",
      scaledEndPrice: "",
      scaledTotalOrders: "5",
      scaledDistribution: "flat",
      tpslEnabled: false,
      tpPrice: "",
      slPrice: "",
      reduceOnly: false,
      postOnly: orderType === "limit",
      ioc: false,
      fok: false,
      ladderId: BigInt(Date.now()),
    };
  }

  async function provideLiquidity(
    context: TestContext,
    buy: boolean,
    targetMarket = marketId,
  ) {
    for (const id of await publicClient.readContract(
      calls.buildGetUserOrderIds(maker.address),
    )) {
      const cancelled = await submit(
        context,
        calls.buildCancelOrder(id),
        makerWallet,
      );
      await gatewayEvents(cancelled, "OrderCancelled");
    }
    const [mark, , , fresh] = await publicClient.readContract(
      calls.buildGetMarkPriceDetails(targetMarket),
    );
    assert.ok(fresh);
    // Mark price uses 18-decimal price units.
    assert.equal(mark, targetMarket === btcMarketId ? 60_000n * WAD : WAD);
    const order = calls.buildPlaceLimitOrder({
      marketId: targetMarket,
      isLong: buy,
      limitPrice: mark,
      sizeUsd: 1_000n * WAD,
      leverage: WAD,
      collateralAmount: 1_000n * USDL_UNIT,
      timeInForce: 3,
      reduceOnly: false,
      cloid: calls.ladderCloid(BigInt(Date.now()), 0),
    });
    const receipt = await submit(
      context,
      (await writes.planOrder([order]))[0],
      makerWallet,
    );
    await gatewayEvents(receipt, "OrderPlaced");
  }

  async function open(context: TestContext, isLong: boolean) {
    await provideLiquidity(context, !isLong);
    const quote = await publicClient.readContract({
      address: perps,
      abi: PerpsPrecompileABI,
      functionName: "quoteOpenPosition",
      args: [marketId, token, 100n * USDL_UNIT, leverage, isLong],
      account: trader,
    });
    const plan = await formPlan(state("market", isLong), quote, limits);
    assert.equal(plan.error, null);
    assert.equal(plan.calls.length, 1);
    const positionPlan = await writes.planOpen({
      marketId,
      isLong,
      collateralAmount: 100n * USDL_UNIT,
      leverage,
      quote,
    });
    assert.equal(plan.calls[0].functionName, positionPlan[0].functionName);
    const receipt = await submit(context, plan.calls[0]);
    const opened = parseEventLogs({
      abi: PerpsPrecompileABI,
      logs: receipt.logs,
      eventName: "PositionOpened",
    }).find((event) => getAddress(event.args.user) === trader.address);
    assert.ok(opened);
    if (isLong) {
      longId = opened.args.positionId;
      longOpenHash = receipt.transactionHash;
    } else {
      shortId = opened.args.positionId;
      shortOpenHash = receipt.transactionHash;
    }
    // Market identifiers are plain integers.
    assert.equal(opened.args.marketId, marketId);
    assert.equal(opened.args.isLong, isLong);
    // Size uses 18-decimal USD, entry price uses 18-decimal price units, and leverage uses 18 decimals.
    assert.equal(opened.args.sizeUsd, quote.sizeUsd);
    assert.equal(opened.args.entryPrice, quote.entryPrice);
    assert.equal(opened.args.leverage, leverage);
    await gatewayEvents(receipt, "PositionOpened");
    await gatewayEvents(receipt, "TradeSettled");
    const fees = parseEventLogs({
      abi: PerpsPrecompileABI,
      logs: receipt.logs,
      eventName: "FeeCollected",
    }).filter((event) => getAddress(event.args.user) === trader.address);
    // Trading fees use 18-decimal USD.
    assert.equal(
      fees.reduce((sum, event) => sum + event.args.feeUsd, 0n),
      quote.tradingFeeUsd,
    );
    await positionRead(
      context,
      opened.args.positionId,
      receipt.transactionHash,
    );
  }

  async function close(context: TestContext, id: bigint, isLong: boolean) {
    await provideLiquidity(context, isLong);
    const before = await publicClient.readContract(
      calls.buildGetPositionDetails(id),
    );
    const quote = await publicClient.readContract({
      address: perps,
      abi: PerpsPrecompileABI,
      functionName: "quoteClosePosition",
      args: [id],
    });
    const plan = writes.planClose({
      positionId: id,
      isLong,
      exitPrice: quote.exitPrice,
    });
    const receipt = await submit(context, plan[0]);
    const closed = parseEventLogs({
      abi: PerpsPrecompileABI,
      logs: receipt.logs,
      eventName: "PositionClosed",
    }).find((event) => event.args.positionId === id);
    assert.ok(closed);
    assert.equal(closed.args.isFullClose, true);
    // 18-decimal base size times 18-decimal price gives 18-decimal USD after division by WAD.
    assert.equal(
      closed.args.closedSizeUsd,
      (before.sizeBase * quote.exitPrice) / WAD,
    );
    assert.equal(closed.args.exitPrice, quote.exitPrice);
    await gatewayEvents(receipt, "PositionClosed");
    await gatewayEvents(receipt, "TradeSettled");
    const ids = await publicClient.readContract(
      calls.buildGetUserPositionIds(trader.address),
    );
    assert.ok(!ids.includes(id));
    await assert.rejects(
      publicClient.readContract(calls.buildGetPosition(id)),
      (error: unknown) => {
        assert.ok(error instanceof BaseError);
        const revert = error.walk(
          (cause) => cause instanceof ContractFunctionRevertedError,
        );
        assert.ok(revert instanceof ContractFunctionRevertedError);
        assert.equal(revert.data?.errorName, "PerpsUnknownPosition");
        return true;
      },
    );
    const { position } = await query<{ position: GatewayPosition | null }>(
      `query JourneyClosed($id: String!) { position(positionId: $id) {
        positionId status closeTxHash exitPrice
      } }`,
      { id: id.toString() },
    );
    context.diagnostic(
      json({
        step: context.name,
        chainPositionIds: ids,
        gateway: position,
        account: await accountState(),
      }),
    );
    assert.ok(position);
    assert.equal(position.status, "closed");
    assert.equal(
      position.closeTxHash?.toLowerCase(),
      receipt.transactionHash.toLowerCase(),
    );
    // Exit prices use 18-decimal price units.
    assert.equal(BigInt(position.exitPrice!), closed.args.exitPrice);
  }

  async function fundCounterparty(context: TestContext) {
    const amount = 10_000n * USDL_UNIT;
    const before = await publicClient.readContract(
      calls.buildGetAccount(maker.address),
    );
    const plan = await calls.buildDepositPlan(
      amount,
      () => allowance(maker.address),
      native,
    );
    for (const call of plan) {
      const receipt = await submit(context, call, makerWallet);
      if (call.functionName === "approve") {
        const [event] = parseEventLogs({
          abi: erc20Abi,
          logs: receipt.logs,
          eventName: "Approval",
        });
        assert.ok(event);
        // Approval amounts use 6-decimal collateral token units.
        assert.equal(event.args.value, amount);
        assert.equal(await allowance(maker.address), amount);
      } else await gatewayEvents(receipt, "CollateralDeposited");
    }
    // Deposited collateral uses 6-decimal token units.
    assert.equal(
      (await publicClient.readContract(calls.buildGetAccount(maker.address)))
        .collateral,
      before.collateral + amount,
    );
  }

  async function approveAndDeposit(context: TestContext) {
    const amount = 500n * USDL_UNIT;
    const plan = await calls.buildDepositPlan(amount, allowance, native);
    assert.equal(plan.length, 2);
    const approval = plan[0];
    assert.equal(approval.functionName, "approve");
    const approved = await submit(context, approval);
    const events = parseEventLogs({
      abi: erc20Abi,
      logs: approved.logs,
      eventName: "Approval",
    });
    assert.equal(events.length, 1);
    assert.equal(getAddress(events[0].args.owner), trader.address);
    assert.equal(getAddress(events[0].args.spender), perps);
    // Approval, allowance, wallet balance and collateral use 6-decimal token units.
    assert.equal(events[0].args.value, amount);
    assert.equal(await allowance(), amount);
    const deposited = await submit(context, plan[1]);
    assert.equal(await tokenBalance(), initialWallet - amount);
    assert.equal(
      (await accountState()).collateral,
      initialAccount.collateral + amount,
    );
    assert.equal(await allowance(), 0n);
    await collateralRead(context, deposited, amount, true);
  }

  async function openLong(context: TestContext) {
    await open(context, true);
  }

  async function openSecondMarket(context: TestContext) {
    const [mark, , , fresh] = await publicClient.readContract(
      calls.buildGetMarkPriceDetails(btcMarketId),
    );
    assert.ok(fresh);
    assert.equal(mark, 60_000n * WAD);
    const liquidity = calls.buildPlaceLimitOrder({
      marketId: btcMarketId,
      isLong: true,
      limitPrice: mark,
      sizeUsd: 1_000n * WAD,
      leverage: WAD,
      collateralAmount: 1_000n * USDL_UNIT,
      timeInForce: 3,
      reduceOnly: false,
      cloid: calls.ladderCloid(BigInt(Date.now()), 0),
    });
    const placed = await submit(
      context,
      (await writes.planOrder([liquidity]))[0],
      makerWallet,
    );
    await gatewayEvents(placed, "OrderPlaced");
    const quote = await publicClient.readContract({
      address: perps,
      abi: PerpsPrecompileABI,
      functionName: "quoteOpenPosition",
      args: [btcMarketId, token, 100n * USDL_UNIT, leverage, false],
      account: trader,
    });
    const form = { ...state("market", false), marketId: btcMarketId };
    const plan = await formPlan(form, quote, {
      tickSize: btcConfig.tickSize,
      lotSize: btcConfig.lotSize,
      maxLeverage: btcConfig.maxLeverage * WAD,
    });
    assert.equal(plan.error, null);
    assert.equal(plan.calls.length, 1);
    const opened = await submit(
      context,
      (await writes.planOrder(plan.calls))[0],
    );
    const event = parseEventLogs({
      abi: PerpsPrecompileABI,
      logs: opened.logs,
      eventName: "PositionOpened",
    }).find((item) => getAddress(item.args.user) === trader.address);
    assert.ok(event);
    secondId = event.args.positionId;
    secondOpenHash = opened.transactionHash;
    assert.equal(event.args.marketId, btcMarketId);
    assert.equal(event.args.isLong, false);
    assert.equal(event.args.sizeUsd, quote.sizeUsd);
    assert.equal((await accountState()).positionCount, 2n);
    await gatewayEvents(opened, "PositionOpened");
    await gatewayEvents(opened, "TradeSettled");
    await positionRead(context, longId, longOpenHash);
    await positionRead(context, secondId, secondOpenHash, btcMarketId);
  }

  async function closeSecondMarket(context: TestContext) {
    await provideLiquidity(context, false, btcMarketId);
    const quote = await publicClient.readContract({
      address: perps,
      abi: PerpsPrecompileABI,
      functionName: "quoteClosePosition",
      args: [secondId],
    });
    const receipt = await submit(
      context,
      writes.planClose({
        positionId: secondId,
        isLong: false,
        exitPrice: quote.exitPrice,
      })[0],
    );
    const event = parseEventLogs({
      abi: PerpsPrecompileABI,
      logs: receipt.logs,
      eventName: "PositionClosed",
    }).find((item) => item.args.positionId === secondId);
    assert.ok(event);
    assert.equal(event.args.isFullClose, true);
    assert.equal(event.args.exitPrice, quote.exitPrice);
    await gatewayEvents(receipt, "PositionClosed");
    await gatewayEvents(receipt, "TradeSettled");
    const ids = await publicClient.readContract(
      calls.buildGetUserPositionIds(trader.address),
    );
    assert.equal(ids.length, 1);
    assert.ok(ids.includes(longId));
    assert.ok(!ids.includes(secondId));
    const { position } = await query<{ position: GatewayPosition }>(
      `query JourneySecondClose($id: String!) { position(positionId: $id) { ${positionFields} } }`,
      { id: secondId.toString() },
    );
    assert.equal(position.status, "closed");
    assert.equal(
      position.closeTxHash?.toLowerCase(),
      receipt.transactionHash.toLowerCase(),
    );
    await positionRead(context, longId, longOpenHash);
  }

  async function setTakeProfitAndStopLoss(context: TestContext) {
    const call = actions.setTpSlAction(longId, true, "1.2", "0.8");
    const receipt = await submit(context, call);
    const [event] = parseEventLogs({
      abi: PerpsPrecompileABI,
      logs: receipt.logs,
      eventName: "PositionTpSlSet",
    });
    assert.ok(event);
    // Position identifiers are plain integers.
    assert.equal(event.args.positionId, longId);
    // TP and SL use 18-decimal price units.
    assert.equal(event.args.takeProfitPrice, (12n * WAD) / 10n);
    assert.equal(event.args.stopLossPrice, (8n * WAD) / 10n);
    const details = await publicClient.readContract(
      calls.buildGetPositionDetails(longId),
    );
    assert.equal(details.takeProfitPrice, event.args.takeProfitPrice);
    assert.equal(details.stopLossPrice, event.args.stopLossPrice);
    const events = await gatewayEvents(receipt, "PositionTpSlSet");
    context.diagnostic(
      json({ step: context.name, chain: details, gateway: events }),
    );
    await positionRead(context, longId, longOpenHash);
  }

  async function addCollateral(context: TestContext) {
    const amount = 25n * USDL_UNIT;
    const before = await accountState();
    const balance = await tokenBalance();
    const position = await publicClient.readContract(
      calls.buildGetPositionDetails(longId),
    );
    const plan = await writes.planCollateral(
      { action: "add", positionId: longId, amount },
      { allowance, native },
    );
    assert.equal(plan.length, 2);
    for (const call of plan) {
      const receipt = await submit(context, call);
      if (call.functionName === "approve") {
        const [event] = parseEventLogs({
          abi: erc20Abi,
          logs: receipt.logs,
          eventName: "Approval",
        });
        assert.ok(event);
        // Approval amounts use 6-decimal collateral token units.
        assert.equal(event.args.value, amount);
        assert.equal(await allowance(), amount);
      } else await collateralRead(context, receipt, amount, true);
    }
    // Account and wallet deltas use 6-decimal collateral token units.
    assert.equal((await accountState()).collateral, before.collateral + amount);
    assert.equal(await tokenBalance(), balance - amount);
    const after = await publicClient.readContract(
      calls.buildGetPositionDetails(longId),
    );
    assert.equal(after.sizeBase, position.sizeBase);
    assert.equal(after.entryPrice, position.entryPrice);
    await positionRead(context, longId, longOpenHash);
  }

  async function removeCollateral(context: TestContext) {
    const amount = 25n * USDL_UNIT;
    const before = await accountState();
    const balance = await tokenBalance();
    // Withdrawable collateral uses 6-decimal token units.
    assert.ok(before.withdrawable >= amount);
    const plan = await writes.planCollateral(
      { action: "remove", positionId: longId, amount },
      { allowance, native },
    );
    assert.equal(plan.length, 1);
    const receipt = await submit(context, plan[0]);
    // Account and wallet deltas use 6-decimal collateral token units.
    assert.equal((await accountState()).collateral, before.collateral - amount);
    assert.equal(await tokenBalance(), balance + amount);
    await collateralRead(context, receipt, amount, false);
    await positionRead(context, longId, longOpenHash);
  }

  async function partiallyCloseLong(context: TestContext) {
    await provideLiquidity(context, true);
    const before = await publicClient.readContract(
      calls.buildGetPositionDetails(longId),
    );
    const action = actions.partialCloseAction({
      positionId: longId,
      isLong: true,
      sizeUsd: before.sizeUsdAtMark,
      bps: 5_000,
    });
    const quote = await publicClient.readContract({
      address: perps,
      abi: PerpsPrecompileABI,
      functionName: "quotePartialClose",
      args: [longId, action.closeSizeUsd],
    });
    const plan = writes.planPartialClose({
      positionId: longId,
      isLong: true,
      closeSizeUsd: action.closeSizeUsd,
      exitPrice: quote[0],
    });
    assert.equal(plan[0].functionName, action.call(quote[0]).functionName);
    assert.equal(plan[0].args[1], action.closeSizeUsd);
    const receipt = await submit(context, plan[0]);
    const event = parseEventLogs({
      abi: PerpsPrecompileABI,
      logs: receipt.logs,
      eventName: "PositionClosed",
    }).find((item) => item.args.positionId === longId);
    assert.ok(event);
    assert.equal(event.args.isFullClose, false);
    const mark = (before.sizeUsdAtMark * WAD) / before.sizeBase;
    const closedBase =
      ((action.closeSizeUsd * WAD) / mark / config.lotSize) * config.lotSize;
    // Closed USD notional uses 18 decimals; base size and price each use 18 decimals.
    assert.equal(event.args.closedSizeUsd, (closedBase * quote[0]) / WAD);
    const after = await publicClient.readContract(
      calls.buildGetPositionDetails(longId),
    );
    // Remaining base size uses 18-decimal units aligned to the base lot size.
    assert.equal(after.sizeBase, before.sizeBase - closedBase);
    assert.ok(after.sizeBase > 0n && after.sizeBase < before.sizeBase);
    await gatewayEvents(receipt, "PositionClosed");
    await gatewayEvents(receipt, "PositionModified");
    await gatewayEvents(receipt, "TradeSettled");
    await positionRead(context, longId, longOpenHash);
  }

  async function placeRestingLimit(context: TestContext) {
    const plan = await formPlan(state("limit", true), null, limits);
    assert.equal(plan.error, null);
    assert.equal(plan.calls.length, 1);
    const receipt = await submit(
      context,
      (await writes.planOrder(plan.calls))[0],
    );
    const [event] = parseEventLogs({
      abi: PerpsPrecompileABI,
      logs: receipt.logs,
      eventName: "OrderPlaced",
    });
    assert.ok(event);
    limitId = event.args.orderId;
    limitHash = receipt.transactionHash;
    assert.equal(getAddress(event.args.user), trader.address);
    // Market identifiers are plain integers.
    assert.equal(event.args.marketId, marketId);
    // Trigger and limit prices use 18-decimal price units.
    assert.equal(event.args.triggerPrice, WAD / 2n);
    const chain = await publicClient.readContract(calls.buildGetOrder(limitId));
    const ids = await publicClient.readContract(
      calls.buildGetUserOrderIds(trader.address),
    );
    assert.ok(ids.includes(limitId));
    assert.equal(chain[2], true);
    // Order notional uses 18-decimal USD, prices use 18-decimal price units, and leverage uses 18 decimals.
    assert.equal(chain[6], 20n * WAD);
    assert.equal(chain[4], WAD / 2n);
    assert.equal(chain[5], WAD / 2n);
    assert.equal(chain[7], leverage);
    const order = await orderRead(limitId);
    const active = await query<{ orders: GatewayOrder[] }>(
      `query JourneyOpenOrders($user: String!) { orders(userAddress: $user, status: "active") { orderId } }`,
      { user: trader.address },
    );
    assert.ok(active.orders.some((item) => BigInt(item.orderId) === limitId));
    assert.equal(BigInt(order.sizeUsd), chain[6]);
    assert.equal(BigInt(order.triggerPrice), chain[4]);
    assert.equal(order.status, "active");
    assert.equal(order.placedTxHash.toLowerCase(), limitHash.toLowerCase());
    await gatewayEvents(receipt, "OrderPlaced");
    context.diagnostic(
      json({ step: context.name, chain, chainOrderIds: ids, gateway: order }),
    );
  }

  async function cancelRestingLimit(context: TestContext) {
    const receipt = await submit(context, calls.buildCancelOrder(limitId));
    const [event] = parseEventLogs({
      abi: PerpsPrecompileABI,
      logs: receipt.logs,
      eventName: "OrderCancelled",
    });
    assert.ok(event);
    // Order identifiers are plain integers.
    assert.equal(event.args.orderId, limitId);
    const ids = await publicClient.readContract(
      calls.buildGetUserOrderIds(trader.address),
    );
    assert.ok(!ids.includes(limitId));
    const chain = await publicClient.readContract(calls.buildGetOrder(limitId));
    assert.equal(chain[10], false);
    const order = await orderRead(limitId);
    assert.equal(order.status, "cancelled");
    assert.equal(
      order.resolvedTxHash?.toLowerCase(),
      receipt.transactionHash.toLowerCase(),
    );
    await gatewayEvents(receipt, "OrderCancelled");
    context.diagnostic(
      json({ step: context.name, chain, chainOrderIds: ids, gateway: order }),
    );
  }

  async function closeLong(context: TestContext) {
    await close(context, longId, true);
  }
  async function openShort(context: TestContext) {
    await open(context, false);
  }
  async function closeShort(context: TestContext) {
    await close(context, shortId, false);
  }

  async function withdrawAllAndReconcile(context: TestContext) {
    assert.ok(lastReceipt);
    const before = await accountState();
    // Position and order counts are plain integers.
    assert.equal(before.positionCount, 0n);
    assert.equal(before.openOrderCount, 0n);
    // Withdrawable collateral and wallet deltas use 6-decimal token units.
    assert.equal(before.withdrawable, before.collateral);
    const walletBefore = await tokenBalance();
    const receipt = await submit(context, calls.buildWithdrawAll(before));
    const after = await accountState();
    const walletAfter = await tokenBalance();
    assert.equal(walletAfter - walletBefore, before.withdrawable);
    assert.equal(after.collateral, 0n);
    assert.equal(after.withdrawable, 0n);
    await collateralRead(context, receipt, before.withdrawable, false);
    const { tradeSettlements, fees } = await query<{
      tradeSettlements: {
        grossPnl: string;
        totalFeesUsd: string;
        fundingPaidUsd: string;
        txHash: string;
      }[];
      fees: { feeUsd: string; feeTokens: string; txHash: string }[];
    }>(
      `query JourneySettlement($user: String!) {
      tradeSettlements(userAddress: $user, limit: 500) { grossPnl totalFeesUsd fundingPaidUsd txHash }
      fees(userAddress: $user, limit: 500) { feeUsd feeTokens txHash }
    }`,
      { user: trader.address },
    );
    const hashes = new Set(traderHashes.map((hash) => hash.toLowerCase()));
    const trades = tradeSettlements.filter((item) =>
      hashes.has(item.txHash.toLowerCase()),
    );
    const paid = fees.filter((item) => hashes.has(item.txHash.toLowerCase()));
    // Three opens, a partial close and three full closes give seven settlements.
    assert.equal(trades.length, 7);
    // Settlement PnL, funding and USD fees use 18-decimal USD.
    assert.equal(
      trades.reduce((sum, item) => sum + BigInt(item.grossPnl), 0n),
      grossPnlUsd,
    );
    assert.equal(
      trades.reduce((sum, item) => sum + BigInt(item.fundingPaidUsd), 0n),
      fundingUsd,
    );
    assert.equal(
      trades.reduce((sum, item) => sum + BigInt(item.totalFeesUsd), 0n),
      feesUsd,
    );
    assert.equal(
      paid.reduce((sum, item) => sum + BigInt(item.feeUsd), 0n),
      feesUsd,
    );
    // Paid collateral fees and the reconciled wallet balance use 6-decimal token units.
    assert.equal(
      paid.reduce((sum, item) => sum + BigInt(item.feeTokens), 0n),
      feeTokens,
    );
    assert.ok(feeTokens > 0n);
    assert.equal(walletAfter, startingWealth + settlementDelta - feeTokens);
    assert.equal(
      walletAfter - startingWealth,
      (grossPnlUsd + fundingUsd) / USD_PER_TOKEN_UNIT - feeTokens,
    );
    context.diagnostic(
      json({
        step: context.name,
        initialWallet,
        initialCollateral: initialAccount.collateral,
        walletBefore,
        walletAfter,
        grossPnlUsd,
        fundingUsd,
        feesUsd,
        feeTokens,
        chain: after,
        gateway: { tradeSettlements: trades, fees: paid },
      }),
    );
  }

  await t.test(fundCounterparty);
  await t.test(approveAndDeposit);
  await t.test(openLong);
  await t.test(openSecondMarket);
  await t.test(setTakeProfitAndStopLoss);
  await t.test(addCollateral);
  await t.test(removeCollateral);
  await t.test(partiallyCloseLong);
  await t.test(placeRestingLimit);
  await t.test(cancelRestingLimit);
  await t.test(closeSecondMarket);
  await t.test(closeLong);
  await t.test(openShort);
  await t.test(closeShort);
  await t.test(withdrawAllAndReconcile);
}

if (isMainThread) {
  test(localTraderJourney);
} else {
  assert.ok(parentPort);
  const port = parentPort;
  void import("@paxeer-network/paxeer-backend/core/perps/orderPlan").then(
    ({ buildOrderPlan }) => {
      port.postMessage(
        buildOrderPlan(workerData.state, workerData.quote, workerData.limits),
      );
    },
  );
}
