import { strict as assert } from "node:assert";
import { test } from "node:test";
import {
  BaseError,
  ContractFunctionRevertedError,
  TransactionReceiptNotFoundError,
  createPublicClient,
  createWalletClient,
  erc20Abi,
  formatUnits,
  getAddress,
  http,
  isHex,
  parseEventLogs,
  size,
  type Hex,
  type TransactionReceipt,
} from "viem";
import { privateKeyToAccount } from "viem/accounts";

import type { OrderFormState, OrderLimits } from "../src/core/perps/orderPlan";

const ENV_NAMES = ["LOCAL_NODE_RPC", "LOCAL_NODE_KEY", "NEXT_PUBLIC_PERPS_GATEWAY_URL"] as const;
type EnvName = (typeof ENV_NAMES)[number];

function requireEnv(): Record<EnvName, string> {
  const env = {} as Record<EnvName, string>;
  for (const name of ENV_NAMES) {
    const value = process.env[name];
    if (!value) throw new Error(`${name} is not set`);
    env[name] = value;
  }
  if (!isHex(env.LOCAL_NODE_KEY) || size(env.LOCAL_NODE_KEY) !== 32) {
    throw new Error("LOCAL_NODE_KEY is not a 32-byte hex private key");
  }
  return env;
}

async function buildOrderPlan(state: OrderFormState, limits: OrderLimits) {
  const planner = await import("../src/core/perps/orderPlan");
  return planner.buildOrderPlan(state, null, limits);
}

async function localOffTickOrderReverts() {
  const env = requireEnv();
  process.env.NEXT_PUBLIC_PAXEER_RPC_URLS = env.LOCAL_NODE_RPC;
  const { paxeerChain } = await import("../src/core/chain");
  const calls = await import("../src/core/perps/calls");
  const { classifyError } = await import("../src/core/errors/classify");
  const { COLLATERAL_TOKENS } = await import("../src/core/perps/config");
  const account = privateKeyToAccount(env.LOCAL_NODE_KEY as Hex);
  const publicClient = createPublicClient({ chain: paxeerChain, transport: http(env.LOCAL_NODE_RPC) });
  const wallet = createWalletClient({ account, chain: paxeerChain, transport: http(env.LOCAL_NODE_RPC) });
  assert.equal(await publicClient.getChainId(), paxeerChain.id);

  async function receipt(hash: Hex): Promise<TransactionReceipt> {
    const deadline = Date.now() + 30_000;
    while (Date.now() < deadline) {
      try {
        return await publicClient.getTransactionReceipt({ hash });
      } catch (error) {
        if (
          !(error instanceof TransactionReceiptNotFoundError) &&
          !(error instanceof Error && /not\s*(be\s*)?found|does not exist|indexing|not indexed/i.test(error.message))
        ) {
          throw error;
        }
      }
      await new Promise((resolve) => setTimeout(resolve, 400));
    }
    throw new Error(`Transaction receipt timed out: ${hash}`);
  }

  const marketId = 0n;
  const config = await publicClient.readContract(calls.buildGetMarketConfig(marketId));
  assert.equal(config.marketId, marketId);
  assert.equal(config.symbol, "BTC");
  assert.ok(config.tickSize > 1n, "BTC must have a representable off-tick price");
  assert.ok(config.lotSize > 0n);
  const [markPrice, , , fresh] = await publicClient.readContract(calls.buildGetMarkPriceDetails(marketId));
  assert.ok(fresh, "BTC mark price must be fresh");
  const alignedPrice = (markPrice / 2n / config.tickSize) * config.tickSize;
  assert.ok(alignedPrice > 0n && alignedPrice < markPrice);
  const sizeUsd = ((200n * 10n ** 18n + config.lotSize - 1n) / config.lotSize) * config.lotSize;
  const state: OrderFormState = {
    orderType: "limit",
    marketId,
    isLong: true,
    collateralAmount: "",
    leverage: "20",
    triggerPrice: formatUnits(alignedPrice, 18),
    limitPrice: "",
    sizeUsd: formatUnits(sizeUsd, 18),
    scaledStartPrice: "",
    scaledEndPrice: "",
    scaledTotalOrders: "5",
    scaledDistribution: "flat",
    tpslEnabled: false,
    tpPrice: "",
    slPrice: "",
    reduceOnly: false,
    postOnly: true,
    ioc: false,
    fok: false,
    ladderId: (await publicClient.getBlock()).timestamp,
  };
  const plan = await buildOrderPlan(state, {
    tickSize: config.tickSize,
    lotSize: config.lotSize,
    maxLeverage: config.maxLeverage * 10n ** 18n,
  });
  assert.equal(plan.error, null);
  assert.equal(plan.calls.length, 1);
  const aligned = plan.calls[0];
  assert.equal(aligned.functionName, "placeOrder");
  assert.equal(aligned.args[0].limitPrice, alignedPrice);
  assert.equal(aligned.args[0].limitPrice % config.tickSize, 0n);

  async function transactionOptions() {
    return {
      gas: 1_000_000n,
      gasPrice: 1_000_000_000_000n,
      nonce: await publicClient.getTransactionCount({ address: account.address, blockTag: "latest" }),
    };
  }

  const depositAmount = 100n * 10n ** 6n;
  const approved = await receipt(
    await wallet.writeContract({
      address: COLLATERAL_TOKENS.usdl.address,
      abi: erc20Abi,
      functionName: "approve",
      args: [aligned.address, depositAmount + 2n * aligned.args[0].collateralAmount],
      ...(await transactionOptions()),
    }),
  );
  assert.equal(approved.status, "success", "USDL approval must be mined");
  const deposited = await receipt(
    await wallet.writeContract({ ...calls.buildDeposit(depositAmount), ...(await transactionOptions()) }),
  );
  assert.equal(deposited.status, "success", "margin and fee reserve must be deposited");
  const accepted = await receipt(await wallet.writeContract({ ...aligned, ...(await transactionOptions()) }));
  assert.equal(accepted.status, "success", "aligned limit order must be mined");
  const placed = parseEventLogs({ abi: aligned.abi, logs: accepted.logs, eventName: "OrderPlaced" });
  assert.equal(placed.length, 1);
  assert.equal(getAddress(placed[0].args.user), account.address);
  assert.equal(placed[0].args.marketId, marketId);
  assert.equal(placed[0].args.triggerPrice, alignedPrice);
  const orderIds = await publicClient.readContract(calls.buildGetUserOrderIds(account.address));
  assert.ok(orderIds.includes(placed[0].args.orderId), "aligned order must rest on the book");

  const offTick = {
    ...aligned.args[0],
    limitPrice: alignedPrice + 1n,
    cloid: calls.ladderCloid(state.ladderId, 1),
  };
  assert.notEqual(offTick.limitPrice % config.tickSize, 0n);
  const raw = {
    address: aligned.address,
    abi: aligned.abi,
    functionName: "placeOrder",
    args: [offTick],
  } as const;
  const rejected = await receipt(await wallet.writeContract({ ...raw, ...(await transactionOptions()) }));
  assert.equal(rejected.status, "reverted", "raw off-tick order must revert on the node");
  assert.equal(parseEventLogs({ abi: aligned.abi, logs: rejected.logs, eventName: "OrderPlaced" }).length, 0);
  await assert.rejects(
    publicClient.simulateContract({ ...raw, account, blockNumber: rejected.blockNumber }),
    (error: unknown) => {
      assert.ok(error instanceof BaseError);
      const revert = error.walk((cause) => cause instanceof ContractFunctionRevertedError);
      assert.ok(revert instanceof ContractFunctionRevertedError);
      assert.equal(revert.data?.errorName, "PerpsTickSize");
      assert.equal(classifyError(error).code, "VALIDATION_PRECISION");
      return true;
    },
  );
  assert.deepEqual(await publicClient.readContract(calls.buildGetUserOrderIds(account.address)), orderIds);
}

test(localOffTickOrderReverts);
