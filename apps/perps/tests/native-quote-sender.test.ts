import assert from "node:assert/strict";
import { EventEmitter } from "node:events";
import { createRequire } from "node:module";
import test, { type TestContext } from "node:test";
import {
  isMainThread,
  parentPort,
  Worker,
  workerData,
} from "node:worker_threads";
import {
  decodeFunctionData,
  encodeErrorResult,
  encodeFunctionResult,
  toHex,
  type Address,
  type Hex,
} from "viem";

async function exerciseQuoteSender(scenario: string) {
  const chainOrigin = "https://api1.mainnet-beta.paxeer.network";
  process.env.NEXT_PUBLIC_PAXEER_RPC_URLS = chainOrigin;
  process.env.NEXT_PUBLIC_PERPS_GATEWAY_URL = chainOrigin;
  const { wagmiConfig } =
    await import("../packages/paxeer-backend/src/core/wagmi");
  const { paxeerChain } =
    await import("../packages/paxeer-backend/src/core/chain");
  const { PerpsPrecompileABI, OrderType } =
    await import("../packages/paxeer-backend/src/core/network/abi/PerpsPrecompile");
  const { COLLATERAL_TOKENS } =
    await import("../packages/paxeer-backend/src/core/perps/config");
  const { PRECOMPILES } =
    await import("../packages/paxeer-backend/src/core/network/precompiles");
  const { classifyError, makeError } =
    await import("../packages/paxeer-backend/src/core/errors/classify");
  const { useQuoter } =
    await import("../packages/paxeer-backend/src/hooks/perps/useQuoter");
  const { prepareNativeQuote } =
    await import("../packages/paxeer-backend/src/nativeTrading");
  const { updateNativeSnapshot } =
    await import("../packages/paxeer-backend/src/runtime");
  const { Window } = await import("happy-dom");
  const dom = new Window();
  for (const [name, value] of Object.entries({
    window: dom,
    document: dom.document,
    navigator: dom.navigator,
    Event: dom.Event,
  })) {
    Object.defineProperty(globalThis, name, { value, configurable: true });
  }
  const owner = COLLATERAL_TOKENS.usdl.address;
  const wad = 10n ** 18n;
  const hash = toHex(1n, { size: 32 });
  const quoteReads: { from?: Address }[] = [];
  const transactions: ReturnType<typeof decodeFunctionData>[] = [];
  const violations: unknown[] = [];
  const noLiquidity = encodeErrorResult({
    abi: PerpsPrecompileABI,
    errorName: "PerpsNoLiquidity",
  });
  const failQuote =
    scenario === "anOrderThatRestsGoesOnWhenTheQuoteFails" ||
    scenario === "aMarketOrderGetsThePlainMessageWhenTheQuoteFails";
  let quoteAnswered = false;
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
        hash,
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
    if (method === "eth_getTransactionByHash") return null;
    if (method === "eth_getTransactionReceipt") {
      assert.equal(params[0], hash);
      assert.equal(transactions.length, 1);
      return {
        transactionHash: hash,
        blockNumber: "0x1",
        blockHash: hash,
        transactionIndex: "0x0",
        from: owner,
        to: PRECOMPILES.perps,
        status: "0x1",
        type: "0x0",
        cumulativeGasUsed: "0x186a0",
        gasUsed: "0x186a0",
        effectiveGasPrice: "0xa",
        logs: [],
        logsBloom: toHex(0n, { size: 256 }),
        contractAddress: null,
      };
    }
    assert.ok(
      ["eth_call", "eth_estimateGas", "eth_sendTransaction"].includes(method),
      `Unexpected chain method ${method}`,
    );
    const raw = params[0] as { to: Address; data: Hex; from?: Address };
    assert.equal(raw.to.toLowerCase(), PRECOMPILES.perps.toLowerCase());
    const call = decodeFunctionData({
      abi: PerpsPrecompileABI,
      data: raw.data,
    });
    if (method === "eth_call" && call.functionName === "quoteOpenPosition") {
      quoteReads.push({ from: raw.from });
      quoteAnswered = true;
      if (failQuote)
        throw Object.assign(new Error("execution reverted"), {
          data: noLiquidity,
        });
      return encodeFunctionResult({
        abi: PerpsPrecompileABI,
        functionName: "quoteOpenPosition",
        result: {
          entryPrice: 3000n * wad,
          sizeUsd: 30n * wad,
          collateralUsd: call.args[2] * 10n ** 12n,
          leverage: call.args[3],
          tradingFee: 0n,
          tradingFeeUsd: 0n,
          priceImpact: 0n,
          liquidationPrice: 2000n * wad,
          estimatedFunding24h: 0n,
          maintenanceMarginBps: 500n,
        },
      });
    }
    assert.equal(call.functionName, "placeOrder");
    if (call.functionName !== "placeOrder")
      throw new Error("Expected a resting order");
    assert.equal(call.args[0].orderType, OrderType.LIMIT);
    assert.equal(call.args[0].marketId, 1n);
    assert.equal(call.args[0].sizeUsd, 30n * wad);
    assert.equal(call.args[0].reduceOnly, false);
    assert.equal(raw.from?.toLowerCase(), owner.toLowerCase());
    if (method === "eth_estimateGas") return "0x186a0";
    if (method === "eth_sendTransaction") {
      transactions.push(call);
      return hash;
    }
    return encodeFunctionResult({
      abi: PerpsPrecompileABI,
      functionName: "placeOrder",
      result: 1n,
    });
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
        error: {
          code: 3,
          message: error instanceof Error ? error.message : String(error),
          ...(error instanceof Error && "data" in error
            ? { data: error.data }
            : {}),
        },
      });
    }
  };
  const backendRequire = createRequire(
    new URL("../packages/paxeer-backend/package.json", import.meta.url),
  );
  const { connect, disconnect } = backendRequire(
    "wagmi/actions",
  ) as typeof import("../packages/paxeer-backend/node_modules/wagmi/actions");
  const { injected } = backendRequire(
    "wagmi/connectors",
  ) as typeof import("../packages/paxeer-backend/node_modules/wagmi/connectors");
  const connected = scenario !== "withoutAnAccountNoQuoteIsRead";
  if (connected) {
    await connect(wagmiConfig, {
      connector: injected({
        target: {
          id: "quote-sender",
          name: "Quote wallet",
          provider: Object.assign(new EventEmitter(), { request }),
        },
      }),
    });
  }
  try {
    updateNativeSnapshot({
      address: connected ? owner : null,
      chainId: paxeerChain.id,
      connected,
      freeCollateral: 1000,
      walletBalances: { USDC: "0" },
      markets: [{ symbol: "ETH", price: 3000 }],
    });
    const order = {
      symbol: "PERP_ETH_USDC",
      side: "BUY",
      order_type:
        scenario === "anOrderThatRestsGoesOnWhenTheQuoteFails"
          ? "LIMIT"
          : "MARKET",
      order_price: "3000",
      order_quantity: "0.01",
    };
    const params = {
      marketId: 1n,
      collateralToken: COLLATERAL_TOKENS.usdc.address,
      collateralAmount: 10_013_500n,
      leverage: 3n * wad,
      isLong: true,
    };
    if (scenario === "theQuoteReadNamesTheAccountAsSender") {
      await prepareNativeQuote(order, 3000, 3);
      assert.equal(quoteReads.length, 1);
      assert.equal(quoteReads[0].from?.toLowerCase(), owner.toLowerCase());
    } else if (scenario === "withoutAnAccountNoQuoteIsRead") {
      await assert.rejects(useQuoter().quoteOpenPosition(params), (error) => {
        assert.deepEqual(error, makeError("WALLET_NOT_CONNECTED"));
        return true;
      });
      assert.deepEqual(quoteReads, []);
    } else if (
      scenario === "aQuoteWithoutANamedAccountTakesTheConnectedAddress"
    ) {
      await useQuoter().quoteOpenPosition(params);
      assert.equal(quoteReads.length, 1);
      assert.equal(quoteReads[0].from?.toLowerCase(), owner.toLowerCase());
    } else {
      assert.ok(failQuote);
      const React = await import("react");
      const { act, create } = await import("react-test-renderer");
      const { useNativeOrderEntry } =
        await import("../packages/hooks/src/native/useNativeOrderEntry");
      let latest: ReturnType<typeof useNativeOrderEntry>;
      let renderedAfterQuote = false;
      function Probe() {
        latest = useNativeOrderEntry(order, 3, () => {});
        if (quoteAnswered) renderedAfterQuote = true;
        return null;
      }
      let renderer: ReturnType<typeof create>;
      await act(async () => {
        renderer = create(React.createElement(Probe));
      });
      try {
        await act(async () => {
          const deadline = Date.now() + 2000;
          while (!renderedAfterQuote && Date.now() < deadline)
            await new Promise((resolve) => setTimeout(resolve, 10));
          assert.equal(quoteAnswered, true);
          assert.equal(renderedAfterQuote, true);
        });
        assert.equal(quoteReads.length, 1);
        assert.ok(latest!.freeCollateral >= 1000);
        if (scenario === "anOrderThatRestsGoesOnWhenTheQuoteFails") {
          await act(async () => {
            const result = await latest!.submit();
            assert.equal(result.success, true);
            assert.equal(result.data.transaction_hash, hash);
          });
          assert.equal(transactions.length, 1);
          assert.equal(transactions[0].functionName, "placeOrder");
        } else {
          const expected = classifyError({ data: noLiquidity });
          assert.notEqual(expected.code, "UNKNOWN");
          await act(async () => {
            await assert.rejects(latest!.submit(), (error) => {
              assert.ok(error instanceof Error);
              assert.equal(error.message, expected.message);
              assert.doesNotMatch(error.message, /quoteOpenPosition|viem/i);
              return true;
            });
          });
          assert.deepEqual(transactions, []);
        }
      } finally {
        await act(async () => renderer!.unmount());
      }
    }
    assert.deepEqual(violations, []);
  } finally {
    if (connected) await disconnect(wagmiConfig);
    await dom.happyDOM.close();
  }
}

async function runQuoteCase(context: TestContext, scenario: string) {
  const worker = new Worker(new URL(import.meta.url), {
    execArgv: ["--import", "tsx"],
    workerData: scenario,
  });
  context.after(async () => {
    await worker.terminate();
  });
  await new Promise<void>((resolve, reject) => {
    const timer = setTimeout(
      () => reject(new Error("Quote assertions timed out")),
      8000,
    );
    context.after(() => clearTimeout(timer));
    worker.once("error", reject);
    worker.once("exit", (code) =>
      reject(new Error(`Quote assertions exited before completion: ${code}`)),
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

async function theQuoteReadNamesTheAccountAsSender(context: TestContext) {
  await runQuoteCase(context, theQuoteReadNamesTheAccountAsSender.name);
}
async function withoutAnAccountNoQuoteIsRead(context: TestContext) {
  await runQuoteCase(context, withoutAnAccountNoQuoteIsRead.name);
}
async function aQuoteWithoutANamedAccountTakesTheConnectedAddress(
  context: TestContext,
) {
  await runQuoteCase(
    context,
    aQuoteWithoutANamedAccountTakesTheConnectedAddress.name,
  );
}
async function anOrderThatRestsGoesOnWhenTheQuoteFails(context: TestContext) {
  await runQuoteCase(context, anOrderThatRestsGoesOnWhenTheQuoteFails.name);
}
async function aMarketOrderGetsThePlainMessageWhenTheQuoteFails(
  context: TestContext,
) {
  await runQuoteCase(
    context,
    aMarketOrderGetsThePlainMessageWhenTheQuoteFails.name,
  );
}

if (isMainThread) {
  test(theQuoteReadNamesTheAccountAsSender);
  test(withoutAnAccountNoQuoteIsRead);
  test(aQuoteWithoutANamedAccountTakesTheConnectedAddress);
  test(anOrderThatRestsGoesOnWhenTheQuoteFails);
  test(aMarketOrderGetsThePlainMessageWhenTheQuoteFails);
} else {
  void exerciseQuoteSender(workerData).then(() => {
    assert.ok(parentPort);
    parentPort.postMessage("assertions-passed");
  });
}
