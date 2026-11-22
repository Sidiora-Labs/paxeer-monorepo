import assert from "node:assert/strict";
import { createRequire } from "node:module";
import test, { type TestContext } from "node:test";
import { isMainThread, parentPort, Worker } from "node:worker_threads";
import {
  decodeFunctionData,
  encodeFunctionResult,
  formatUnits,
  parseUnits,
  toHex,
  type Address,
  type Hex,
} from "viem";

async function exerciseNativeOrderFunding() {
  const chainOrigin = "https://api1.mainnet-beta.paxeer.network";
  process.env.NEXT_PUBLIC_PAXEER_RPC_URLS = chainOrigin;
  process.env.NEXT_PUBLIC_PERPS_GATEWAY_URL = chainOrigin;
  const { wagmiConfig } =
    await import("../packages/paxeer-backend/src/core/wagmi");
  const { paxeerChain } =
    await import("../packages/paxeer-backend/src/core/chain");
  const { PerpsPrecompileABI, OrderType, TimeInForce } =
    await import("../packages/paxeer-backend/src/core/network/abi/PerpsPrecompile");
  const { erc20Abi, COLLATERAL_TOKENS } =
    await import("../packages/paxeer-backend/src/core/perps/config");
  const { PERPS_SPENDER } =
    await import("../packages/paxeer-backend/src/core/network/approvals");
  const { buildGetAccount, buildGetFeeTier } =
    await import("../packages/paxeer-backend/src/core/perps/calls");
  const { portfolioFromAccount } =
    await import("../packages/paxeer-backend/src/core/perps/portfolio");
  const { feeTierView } =
    await import("../packages/paxeer-backend/src/core/perps/feeTier");
  const { getPublicClient } =
    await import("../packages/paxeer-backend/src/core/viem/clients");
  const { getWalletBalance } =
    await import("../packages/paxeer-backend/src/hooks/perps/useVault");
  const { executeNativeOrder, prepareNativeQuote } =
    await import("../packages/paxeer-backend/src/nativeTrading");
  const {
    planNativeOrder,
    nativeOrderFunding,
    nativeOrderCapacity,
    NativeInsufficientBalanceError,
  } = await import("../packages/paxeer-backend/src/nativeOrderPlan");
  const { getNativeSnapshot, updateNativeSnapshot } =
    await import("../packages/paxeer-backend/src/runtime");
  const { Window } = await import("happy-dom");
  const dom = new Window();
  Object.defineProperty(globalThis, "window", {
    value: dom,
    configurable: true,
  });
  Object.defineProperty(globalThis, "document", {
    value: dom.document,
    configurable: true,
  });
  Object.defineProperty(globalThis, "navigator", {
    value: dom.navigator,
    configurable: true,
  });
  Object.defineProperty(globalThis, "Event", {
    value: dom.Event,
    configurable: true,
  });
  const owner = COLLATERAL_TOKENS.usdl.address;
  const wad = 10n ** 18n;
  const tokenScale = 10n ** 12n;
  let freeMargin = 0n;
  let walletBalance = 0n;
  let allowance = 0n;
  let takerPpm = 450n;
  let transactionCount = 0n;
  let rejectDeposit = false;
  const calls: ReturnType<typeof decode>[] = [];
  const events: string[] = [];
  const reads: string[] = [];
  const violations: unknown[] = [];
  const receipts = new Map<Hex, Record<string, unknown>>();
  function decode(call: { to: Address; data: Hex }) {
    return decodeFunctionData({
      abi:
        call.to.toLowerCase() === PERPS_SPENDER.toLowerCase()
          ? PerpsPrecompileABI
          : erc20Abi,
      data: call.data,
    });
  }
  function marginRequired(size: bigint, leverage: bigint) {
    const initial = (size * wad) / leverage;
    const fee = (size * takerPpm + 999_999n) / 1_000_000n;
    return (initial + fee + tokenScale - 1n) / tokenScale;
  }
  function validateWrite(call: ReturnType<typeof decode>) {
    if (call.functionName === "approve") {
      assert.equal(call.args[0], PERPS_SPENDER);
    } else if (call.functionName === "deposit") {
      assert.equal(call.args[0], owner);
      assert.ok(call.args[1] <= walletBalance);
      assert.ok(call.args[1] <= allowance);
      if (rejectDeposit) throw new Error("Deposit refused");
    } else if (call.functionName === "placeOrder") {
      const order = call.args[0];
      assert.ok(freeMargin >= marginRequired(order.sizeUsd, order.leverage));
      assert.equal(order.reduceOnly, false);
      assert.equal(order.marketId, 1n);
      assert.ok(order.sizeUsd > 0n);
    } else if (call.functionName === "openPosition") {
      assert.equal(call.args[1], owner);
      assert.ok(walletBalance >= call.args[2]);
      assert.ok(allowance >= call.args[2]);
    } else if (
      call.functionName === "placeLimitOrder" ||
      call.functionName === "placeStopLimitOrder"
    ) {
      const amount = call.args.at(-1) as bigint;
      assert.equal(call.args.at(-2), owner);
      assert.ok(walletBalance >= amount);
      assert.ok(allowance >= amount);
    } else throw new Error(`Unexpected write ${call.functionName}`);
  }
  async function request({
    method,
    params = [],
  }: {
    method: string;
    params?: readonly unknown[];
  }): Promise<unknown> {
    if (method === "eth_chainId") return toHex(paxeerChain.id);
    if (method === "eth_accounts" || method === "eth_requestAccounts")
      return [owner];
    if (method === "wallet_requestPermissions")
      return [{ parentCapability: "eth_accounts" }];
    if (method === "net_listening") return true;
    if (method === "eth_blockNumber") return "0x1";
    if (method === "eth_gasPrice" || method === "eth_maxPriorityFeePerGas")
      return "0xa";
    if (method === "eth_getBlockByNumber")
      return {
        number: "0x1",
        hash: toHex(1n, { size: 32 }),
        parentHash: toHex(0n, { size: 32 }),
        timestamp: "0x1",
        baseFeePerGas: "0xa",
        gasLimit: "0x1c9c380",
        gasUsed: "0x0",
        transactions: [],
        extraData: "0x",
        miner: owner,
        difficulty: "0x0",
        size: "0x0",
      };
    if (method === "eth_getTransactionReceipt") {
      const receipt = receipts.get(params[0] as Hex);
      assert.ok(receipt);
      return receipt;
    }
    if (method === "eth_getTransactionByHash")
      return {
        hash: params[0],
        from: owner,
        to: PERPS_SPENDER,
        nonce: "0x0",
        input: "0x",
        value: "0x0",
        gas: "0x186a0",
        gasPrice: "0xa",
        blockNumber: "0x1",
        blockHash: toHex(1n, { size: 32 }),
        transactionIndex: "0x0",
        type: "0x0",
        v: "0x1b",
        r: toHex(1n, { size: 32 }),
        s: toHex(1n, { size: 32 }),
      };
    const raw = params[0] as { to: Address; data: Hex };
    const call = decode(raw);
    if (method === "eth_call") {
      reads.push(call.functionName);
      if (call.functionName === "getAccount")
        return encodeFunctionResult({
          abi: PerpsPrecompileABI,
          functionName: "getAccount",
          result: {
            collateral: freeMargin + 100_000_000n,
            equityUsd: (freeMargin + 100_000_000n) * tokenScale,
            initialMarginUsd: 100_000_000n * tokenScale,
            maintenanceMarginUsd: 0n,
            withdrawable: freeMargin,
            feeTier: 0,
            volume14dUsd: 0n,
            positionCount: 1n,
            openOrderCount: 0n,
          },
        });
      if (call.functionName === "balanceOf")
        return encodeFunctionResult({
          abi: erc20Abi,
          functionName: "balanceOf",
          result: walletBalance,
        });
      if (call.functionName === "allowance")
        return encodeFunctionResult({
          abi: erc20Abi,
          functionName: "allowance",
          result: allowance,
        });
      if (call.functionName === "getFeeTier")
        return encodeFunctionResult({
          abi: PerpsPrecompileABI,
          functionName: "getFeeTier",
          result: [0, takerPpm, 150n, 0n],
        });
      if (call.functionName === "quoteOpenPosition") {
        const collateral = call.args[2];
        const leverage = call.args[3];
        return encodeFunctionResult({
          abi: PerpsPrecompileABI,
          functionName: "quoteOpenPosition",
          result: {
            entryPrice: 3000n * wad,
            sizeUsd:
              (collateral * tokenScale * leverage * 1_000_000n) /
              (wad * 1_000_000n + leverage * takerPpm),
            collateralUsd: collateral * tokenScale,
            leverage,
            tradingFee: 0n,
            tradingFeeUsd: 0n,
            priceImpact: 0n,
            liquidationPrice: 0n,
            estimatedFunding24h: 0n,
            maintenanceMarginBps: 500n,
          },
        });
      }
      validateWrite(call);
      if (call.functionName === "approve")
        return encodeFunctionResult({
          abi: erc20Abi,
          functionName: "approve",
          result: true,
        });
      if (call.functionName === "deposit") return "0x";
      return toHex(1n, { size: 32 });
    }
    if (method === "eth_estimateGas") {
      validateWrite(call);
      return "0x186a0";
    }
    if (method === "eth_sendTransaction") {
      validateWrite(call);
      calls.push(call);
      events.push(call.functionName);
      if (call.functionName === "approve") allowance = call.args[1];
      if (call.functionName === "deposit") {
        walletBalance -= call.args[1];
        freeMargin += call.args[1];
        allowance -= call.args[1];
      }
      const hash = toHex(++transactionCount, { size: 32 });
      receipts.set(hash, {
        transactionHash: hash,
        blockNumber: "0x1",
        blockHash: toHex(1n, { size: 32 }),
        transactionIndex: "0x0",
        from: owner,
        to: raw.to,
        status: "0x1",
        type: "0x0",
        cumulativeGasUsed: "0x186a0",
        gasUsed: "0x186a0",
        effectiveGasPrice: "0xa",
        logs: [],
        logsBloom: toHex(0n, { size: 256 }),
        contractAddress: null,
      });
      return hash;
    }
    throw new Error(`Unexpected chain method ${method}`);
  }
  globalThis.fetch = async (input, init) => {
    const url = input instanceof Request ? input.url : String(input);
    assert.equal(new URL(url).origin, chainOrigin);
    const body = JSON.parse(String(init?.body));
    try {
      return Response.json({
        jsonrpc: "2.0",
        id: body.id,
        result: await request(body),
      });
    } catch (error) {
      if (error instanceof assert.AssertionError) violations.push(error);
      return Response.json({
        jsonrpc: "2.0",
        id: body.id,
        error: { code: -32000, message: String(error) },
      });
    }
  };
  const { EventEmitter } = await import("node:events");
  const provider = Object.assign(new EventEmitter(), { request });
  const backendRequire = createRequire(
    new URL("../packages/paxeer-backend/package.json", import.meta.url),
  );
  const { connect, disconnect } = backendRequire(
    "wagmi/actions",
  ) as typeof import("../packages/paxeer-backend/node_modules/wagmi/actions");
  const { injected } = backendRequire(
    "wagmi/connectors",
  ) as typeof import("../packages/paxeer-backend/node_modules/wagmi/connectors");
  await connect(wagmiConfig, {
    connector: injected({
      target: { id: "funding", name: "Funding wallet", provider },
    }),
  });

  async function balances(margin: string, wallet: string, approved = true) {
    freeMargin = parseUnits(margin, 6);
    walletBalance = parseUnits(wallet, 6);
    allowance = approved ? parseUnits("1000000", 6) : 0n;
    calls.length = 0;
    events.length = 0;
    updateNativeSnapshot({
      address: owner,
      chainId: paxeerChain.id,
      connected: true,
    });
    const client = getPublicClient();
    const [account, walletUnits, tier] = await Promise.all([
      client.readContract(buildGetAccount(owner)),
      getWalletBalance(owner, owner),
      client.readContract(buildGetFeeTier(owner)),
    ]);
    const portfolio = portfolioFromAccount(account, []);
    assert.ok(portfolio.totalVaultUsd > portfolio.freeCollateral);
    updateNativeSnapshot({
      ...portfolio,
      positions: [],
      walletBalances: { USDC: formatUnits(walletUnits, 6) },
      feeTier: feeTierView(tier),
      markets: [{ symbol: "ETH", price: 3000 }],
    });
  }
  for (const type of ["MARKET", "LIMIT", "STOP_LIMIT"]) {
    for (const side of ["BUY", "SELL"]) {
      const order = {
        symbol: "PERP_ETH_USDC",
        side,
        order_type: type,
        order_quantity: "0.01",
        order_price: "3000",
        trigger_price: "3100",
        leverage: 3,
      };
      const plan = planNativeOrder(order, 3000, 3);
      for (const scenario of [
        { margin: "11", wallet: "0", source: "margin" },
        { margin: "11", wallet: "20", source: "margin" },
        { margin: "0", wallet: "20", source: "wallet" },
        { margin: "6", wallet: "20", source: "wallet" },
        { margin: "6", wallet: "6", source: "split" },
        { margin: "1", wallet: "1", source: "insufficient" },
      ]) {
        await balances(scenario.margin, scenario.wallet);
        const snapshot = getNativeSnapshot();
        const funding = nativeOrderFunding(
          plan,
          snapshot.freeCollateral,
          snapshot.walletBalances.USDC,
          3,
          4.5,
        );
        assert.equal(funding.source, scenario.source);
        assert.equal(funding.requiredAmount, 10_013_500n);
        if (scenario.source === "insufficient") {
          await assert.rejects(executeNativeOrder(order), {
            name: "NativeInsufficientBalanceError",
            message: "Insufficient balance to place this order",
          });
          assert.deepEqual(calls, []);
          continue;
        }
        const quote =
          type === "MARKET" ? await prepareNativeQuote(order, 3000, 3) : null;
        await executeNativeOrder(
          { ...order, native_quote_id: quote?.id },
          {
            beforeDeposit: async () => {
              assert.deepEqual(calls, []);
              events.push("two-transactions");
            },
          },
        );
        const names = calls.map((call) => call.functionName);
        if (scenario.source === "wallet") {
          assert.deepEqual(names, [
            type === "MARKET"
              ? "openPosition"
              : type === "LIMIT"
                ? "placeLimitOrder"
                : "placeStopLimitOrder",
          ]);
          assert.equal(
            calls[0].args[type === "MARKET" ? 2 : calls[0].args.length - 1],
            funding.requiredAmount,
          );
          assert.deepEqual(events, names);
        } else {
          assert.deepEqual(
            names,
            scenario.source === "split"
              ? ["deposit", "placeOrder"]
              : ["placeOrder"],
          );
          assert.deepEqual(
            events,
            scenario.source === "split"
              ? ["two-transactions", ...names]
              : names,
          );
          if (scenario.source === "split") {
            assert.deepEqual(calls[0].args, [owner, 4_013_500n]);
            assert.equal(funding.depositAmount, 4_013_500n);
          }
          const plain = calls.at(-1)!;
          assert.equal(plain.functionName, "placeOrder");
          if (plain.functionName !== "placeOrder")
            throw new Error("Expected plain order");
          const placed = plain.args[0];
          assert.equal(placed.sizeUsd, plan.sizeUsd);
          assert.equal(placed.leverage, plan.leverage);
          assert.equal(placed.isLong, side === "BUY");
          assert.equal(
            placed.orderType,
            type === "MARKET"
              ? OrderType.MARKET
              : type === "LIMIT"
                ? OrderType.LIMIT
                : OrderType.STOP_LIMIT,
          );
          assert.equal(
            placed.timeInForce,
            type === "MARKET" ? TimeInForce.IOC : TimeInForce.GTC,
          );
          assert.equal(
            placed.limitPrice,
            type === "MARKET" ? 0n : plan.limitPrice,
          );
          assert.equal(
            placed.triggerPrice,
            type === "STOP_LIMIT" ? plan.triggerPrice : 0n,
          );
          if (type !== "MARKET") assert.notEqual(BigInt(placed.cloid), 0n);
        }
      }
      for (const source of ["wallet", "split"]) {
        await balances(
          source === "wallet" ? "0" : "6",
          source === "wallet" ? "20" : "6",
          false,
        );
        const quote =
          type === "MARKET" ? await prepareNativeQuote(order, 3000, 3) : null;
        await executeNativeOrder(
          { ...order, native_quote_id: quote?.id },
          {
            beforeDeposit: async () => {
              events.push("two-transactions");
            },
          },
        );
        const finalCall =
          type === "MARKET"
            ? "openPosition"
            : type === "LIMIT"
              ? "placeLimitOrder"
              : "placeStopLimitOrder";
        assert.deepEqual(
          calls.map((call) => call.functionName),
          source === "wallet"
            ? ["approve", finalCall]
            : ["approve", "deposit", "placeOrder"],
        );
        assert.equal(
          calls[0].args[1],
          source === "wallet" ? 10_013_500n : 4_013_500n,
        );
        if (source === "split") assert.equal(events[0], "two-transactions");
      }
    }
  }

  const splitOrder = {
    symbol: "ETH",
    side: "BUY",
    order_type: "LIMIT",
    order_price: "3000",
    order_quantity: "0.01",
    leverage: 3,
  };
  await balances("6", "6");
  await assert.rejects(
    executeNativeOrder(splitOrder, {
      beforeDeposit: async () => {
        throw new Error("Order cancelled");
      },
    }),
    /Order cancelled/,
  );
  assert.deepEqual(calls, []);
  rejectDeposit = true;
  await assert.rejects(
    executeNativeOrder(splitOrder, {
      beforeDeposit: async () => {
        events.push("two-transactions");
      },
    }),
    /Deposit refused/,
  );
  assert.deepEqual(calls, []);
  assert.deepEqual(events, ["two-transactions"]);
  rejectDeposit = false;

  for (const takerBps of [0, 2.8, 4.5]) {
    takerPpm = BigInt(Math.round(takerBps * 100));
    for (const leverage of [1, 3, 20]) {
      for (const [margin, wallet] of [
        [1000, "0"],
        [0, "1000"],
        [600, "400"],
      ] as const) {
        const capacity = nativeOrderCapacity(
          margin,
          wallet,
          leverage,
          takerBps,
          3000,
        );
        assert.equal(capacity.freeCollateral, 1000);
        assert.ok(capacity.maxQty > 0);
        assert.ok(capacity.maxQty <= (1000 * leverage) / 3000);
        assert.ok(
          capacity.maxQty <=
            (1000 * leverage) / (3000 * (1 + (leverage * takerBps) / 10_000)),
        );
        const plan = planNativeOrder(
          {
            symbol: "ETH",
            side: "BUY",
            order_quantity: String(capacity.maxQty),
          },
          3000,
          leverage,
        );
        assert.ok(
          marginRequired(plan.sizeUsd, plan.leverage) <= 1_000_000_000n,
        );
        assert.notEqual(
          nativeOrderFunding(plan, margin, wallet, leverage, takerBps).source,
          "insufficient",
        );
      }
      assert.equal(
        nativeOrderCapacity(0, "0", leverage, takerBps, 3000).maxQty,
        0,
      );
    }
  }
  const tiny = planNativeOrder(
    { symbol: "ETH", side: "BUY", order_quantity: "0.00000001" },
    3000,
    3,
  );
  assert.equal(nativeOrderFunding(tiny, 0, "1", 3, 4.5).requiredAmount, 11n);
  assert.equal(
    nativeOrderFunding(tiny, 0.000011, "0", 3, 4.5).source,
    "margin",
  );
  assert.equal(
    nativeOrderFunding(tiny, 0.00001, "0", 3, 4.5).source,
    "insufficient",
  );

  const React = await import("react");
  const { act, create } = await import("react-test-renderer");
  const { useNativeOrderEntry } =
    await import("../packages/hooks/src/native/useNativeOrderEntry");
  const { useOrderEntry } =
    await import("../packages/hooks/src/next/useOrderEntry/useOrderEntry");
  let latest: ReturnType<typeof useNativeOrderEntry>;
  const formOrder = {
    symbol: "PERP_ETH_USDC",
    side: "BUY",
    order_type: "LIMIT",
    order_price: "3000",
    order_quantity: "0.01",
  };
  function Probe() {
    latest = useNativeOrderEntry(formOrder, 3, () => {});
    return null;
  }
  await balances("1000", "0");
  let renderer: ReturnType<typeof create>;
  await act(async () => {
    renderer = create(React.createElement(Probe));
  });
  assert.equal(latest!.freeCollateral, 1000);
  assert.ok(latest!.maxQty > 0 && latest!.maxQty <= 1);
  await act(async () => {
    updateNativeSnapshot({
      freeCollateral: null,
      walletBalances: { USDC: "5" },
    });
  });
  assert.equal(latest!.freeCollateral, 5);
  await act(async () => {
    updateNativeSnapshot({ freeCollateral: 6, walletBalances: {} });
  });
  assert.equal(latest!.freeCollateral, 6);
  await act(async () => {
    updateNativeSnapshot({ freeCollateral: 0, walletBalances: { USDC: "0" } });
  });
  assert.equal(latest!.maxQty, 0);
  await assert.rejects(
    latest!.helper.validate(),
    NativeInsufficientBalanceError,
  );
  assert.deepEqual(calls, []);
  await act(async () => {
    renderer!.unmount();
  });

  let form: ReturnType<typeof useOrderEntry>;
  function FormProbe() {
    form = useOrderEntry(formOrder.symbol, {
      initialOrder: { side: "BUY", order_type: "LIMIT" },
    });
    return null;
  }
  await act(async () => {
    renderer = create(React.createElement(FormProbe));
  });
  await act(async () => {
    form!.setValues({ order_quantity: "0.01", order_price: "3000" });
  });
  await act(async () => {
    await assert.rejects(form!.helper.validate(), (errors) => {
      assert.deepEqual(errors, {
        order_quantity: {
          type: "max",
          value: 0,
          message: "Insufficient balance to place this order",
        },
      });
      return true;
    });
  });
  assert.equal(
    form!.metaState.errors?.order_quantity?.message,
    "Insufficient balance to place this order",
  );
  assert.equal(form!.metaState.validated, true);
  assert.deepEqual(calls, []);
  await act(async () => {
    renderer!.unmount();
  });
  await disconnect(wagmiConfig);
  assert.ok(reads.includes("getAccount"));
  assert.ok(reads.includes("balanceOf"));
  assert.ok(reads.includes("getFeeTier"));
  assert.deepEqual(violations, []);
}

async function nativeOrdersUseDepositedMarginBeforeWallet(
  context: TestContext,
) {
  const worker = new Worker(new URL(import.meta.url), {
    execArgv: ["--import", "tsx"],
  });
  context.after(async () => {
    await worker.terminate();
  });
  await new Promise<void>((resolve, reject) => {
    worker.once("error", reject);
    worker.once("exit", (code) =>
      reject(new Error(`Funding assertions exited before completion: ${code}`)),
    );
    worker.once("message", (message) => {
      try {
        assert.equal(message, "assertions-passed");
        resolve();
      } catch (error) {
        reject(error);
      }
    });
  });
}

if (isMainThread) test(nativeOrdersUseDepositedMarginBeforeWallet);
else
  void exerciseNativeOrderFunding().then(() => {
    assert.ok(parentPort);
    parentPort.postMessage("assertions-passed");
  });
