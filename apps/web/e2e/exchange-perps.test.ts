import assert from "node:assert/strict";
import { existsSync, readFileSync } from "node:fs";
import { createServer, type IncomingMessage } from "node:http";
import { registerHooks } from "node:module";
import test from "node:test";
import { fileURLToPath } from "node:url";
import ts from "typescript";
import { isValidElement, type ReactElement, type ReactNode } from "react";
import type { PerpsOrderInputs } from "../src/app/exchange/exchange-client.tsx";
import type { PerpsOpenPosition } from "../src/api/sdk.ts";
import { decodeFunctionData, encodeErrorResult, encodeFunctionData, encodeFunctionResult, isHex, parseAbi, type Abi, type Hex } from "viem";

registerHooks({
  resolve(specifier, context, nextResolve) {
    if (specifier.startsWith(".") && context.parentURL !== undefined) {
      const candidate = new URL(specifier, context.parentURL);
      for (const suffix of [".ts", ".tsx", "/index.ts"]) {
        const source = candidate.href.endsWith(".js") ? new URL(candidate.href.slice(0, -3) + suffix) : new URL(candidate.href + suffix);
        if (existsSync(source)) return nextResolve(source.href, context);
      }
    }
    if (specifier === "server-only") return { url: "data:text/javascript,", shortCircuit: true };
    if (specifier === "next/navigation") return nextResolve("next/navigation.js", context);
    if (specifier === "next/link") return nextResolve("next/link.js", context);
    return nextResolve(specifier, context);
  },
  load(url, context, nextLoad) {
    if (/\.(ts|tsx)$/u.test(url) && !url.includes("/node_modules/")) {
      return {
        format: "module",
        shortCircuit: true,
        source: ts.transpileModule(readFileSync(new URL(url), "utf8"), {
          fileName: fileURLToPath(url),
          compilerOptions: { target: ts.ScriptTarget.ES2022, module: ts.ModuleKind.ESNext, jsx: ts.JsxEmit.ReactJSX },
        }).outputText,
      };
    }
    const loaded = nextLoad(url, context);
    return loaded.format === "commonjs" ? { ...loaded, source: undefined } : loaded;
  },
});

const precompiles = await import("../src/api/precompiles.ts");
const sdk = await import("../src/api/sdk.ts");
const { preflightPerpsWrite } = await import("../src/app/exchange/actions.ts");
const client = await import("../src/app/exchange/exchange-client.tsx");
const wallet = await import("../src/api/wallet.ts");

// The ABI the chain publishes for 0x1015; every answer below is encoded from it.
const PERPS_ABI = JSON.parse(readFileSync(new URL("../../../precompiles/perps/abi/abi.json", import.meta.url), "utf8")) as Abi;
const PERPS = "0x0000000000000000000000000000000000001015";
const USDL = precompiles.PERPS_COLLATERAL_TOKEN;
const TRADER = `0x${"11".repeat(20)}`;
function word(id: bigint): Hex {
  return `0x${id.toString(16).padStart(64, "0")}`;
}
const ZERO_WORD = `0x${"0".repeat(64)}`;

const ANSWERS: Record<string, (args: readonly unknown[]) => unknown> = {
  totalMarkets: () => 2n,
  getMarket: ([id]) => (id === 0n ? ["Bitcoin", "BTC", 1000n, 5n, 10n ** 27n, true] : ["Ether", "ETH", 500n, 10n, 2n * 10n ** 26n, false]),
  getAccountSummary: ([account, token]) => {
    assert.equal(String(account).toLowerCase(), TRADER);
    assert.equal(String(token).toLowerCase(), USDL);
    return { idle: 7_000_000n, totalLocked: 3_000_000n, positionCount: 1n };
  },
  getUserPositionIds: () => [4n, 5n],
  getPosition: ([id]) => [TRADER, 0n, true, 30n * 10n ** 18n, 3n * 10n ** 18n, USDL, 3_000_000n, 65_000n * 10n ** 18n, 1n, id === 4n],
  getUserOrderIds: () => [9n],
  getOrder: () => [TRADER, 1n, false, 0, 0n, 2_000n * 10n ** 18n, 10n ** 19n, 5n * 10n ** 18n, USDL, 2_000_000n, true],
};

type Reply = Readonly<{ result: string }> | Readonly<{ error: { code: number; message: string; data?: string } }>;

async function body(request: IncomingMessage): Promise<{ id: string; method: string; params: { from?: string; to: string; data: Hex }[] }> {
  let text = "";
  for await (const chunk of request) text += String(chunk);
  return JSON.parse(text) as { id: string; method: string; params: { from?: string; to: string; data: Hex }[] };
}

/** A gateway answering eth_call at 0x1015 only for functions the published perps ABI declares. */
async function gateway(
  answer: (call: { from?: string; to: string; data: Hex }, name: string, args: readonly unknown[]) => Reply,
  run: (called: string[]) => Promise<void>,
  receipts?: ReadonlySet<string>,
) {
  const called: string[] = [];
  const server = createServer((request, response) => {
    void body(request).then((envelope) => {
      response.setHeader("content-type", "application/json");
      if (envelope.method === "eth_getTransactionReceipt" && receipts !== undefined) {
        const hash = String(envelope.params[0]);
        assert.ok(receipts.has(hash), `receipt for unsent transaction ${hash}`);
        response.end(JSON.stringify({ jsonrpc: "2.0", id: envelope.id,
          result: { transactionHash: hash, status: "0x1", blockNumber: "0x1", logs: [] } }));
        return;
      }
      const [call] = envelope.params;
      let reply: Reply;
      try {
        assert.equal(envelope.method, "eth_call");
        assert.ok(call !== undefined && call.to.toLowerCase() === PERPS, `eth_call to ${String(call?.to)}`);
        const { functionName, args } = decodeFunctionData({ abi: PERPS_ABI, data: call.data });
        called.push(functionName);
        reply = answer(call, functionName, args ?? []);
      } catch (error) {
        called.push(`rejected: ${String(error)}`);
        reply = { error: { code: -32602, message: "unexpected call" } };
      }
      response.end(JSON.stringify({ jsonrpc: "2.0", id: envelope.id, ...reply }));
    });
  });
  await new Promise<void>((resolve) => server.listen(0, "127.0.0.1", resolve));
  const address = server.address();
  assert.ok(address !== null && typeof address === "object");
  const previous = process.env.LAYERX_GATEWAY_URL;
  process.env.LAYERX_GATEWAY_URL = `http://127.0.0.1:${address.port}`;
  try {
    await run(called);
  } finally {
    if (previous === undefined) delete process.env.LAYERX_GATEWAY_URL;
    else process.env.LAYERX_GATEWAY_URL = previous;
    await new Promise<void>((resolve) => server.close(() => resolve()));
  }
}

function served(_call: unknown, name: string, args: readonly unknown[]): Reply {
  const answer = ANSWERS[name];
  assert.ok(answer !== undefined, `unexpected read ${name}`);
  return { result: encodeFunctionResult({ abi: PERPS_ABI, functionName: name, result: answer(args) } as never) };
}

test("the web perps constants match the module default collateral and the precompile's custom errors", () => {
  const params = readFileSync(new URL("../../../modules/perps/types/params.go", import.meta.url), "utf8");
  const declared = /DefaultCollateralToken = "(0x[0-9a-fA-F]{40})"/u.exec(params)?.[1];
  assert.equal(declared?.toLowerCase(), USDL);
  const errors = PERPS_ABI.filter((entry) => entry.type === "error").map((entry) => (entry as { name: string }).name);
  assert.deepEqual([...precompiles.PERPS_ERRORS].sort(), errors.sort());
  assert.equal(sdk.PERPS_PRECOMPILE, PERPS);
});

test("markets come from totalMarkets and getMarket, and the account from its position and order ids", async () => {
  await gateway(served, async (called) => {
    const markets = await precompiles.perpsMarkets();
    assert.deepEqual(markets, [
      {
        marketId: 0n,
        name: "Bitcoin",
        symbol: "BTC",
        maxLeverage: 1000n,
        maintenanceMarginBps: 5n,
        maxOpenInterest: 10n ** 27n,
        enabled: true,
      },
      {
        marketId: 1n,
        name: "Ether",
        symbol: "ETH",
        maxLeverage: 500n,
        maintenanceMarginBps: 10n,
        maxOpenInterest: 2n * 10n ** 26n,
        enabled: false,
      },
    ]);
    const account = await precompiles.perpsAccount(TRADER);
    assert.equal(account.idle, 7_000_000n);
    assert.equal(account.locked, 3_000_000n);
    assert.deepEqual(account.positions, [
      {
        positionId: 4n,
        marketId: 0n,
        isLong: true,
        sizeUsd: 30n * 10n ** 18n,
        collateralUsd: 3n * 10n ** 18n,
        collateralAmount: 3_000_000n,
        entryPrice: 65_000n * 10n ** 18n,
      },
    ]);
    assert.deepEqual(account.orders, [
      {
        orderId: 9n,
        marketId: 1n,
        isLong: false,
        orderType: 0n,
        triggerPrice: 0n,
        limitPrice: 2_000n * 10n ** 18n,
        sizeUsd: 10n ** 19n,
        collateralAmount: 2_000_000n,
      },
    ]);
    assert.ok(
      called.every((name) => !name.startsWith("rejected")),
      called.join("\n"),
    );
  });
});

test("a perps refusal surfaces the precompile's custom error or reason string", async () => {
  const close = encodeFunctionData({ abi: PERPS_ABI, functionName: "closePosition", args: [4n] });
  assert.equal(close, sdk.perpsClosePositionCall(4n).data);
  const replies: Reply[] = [
    { error: { code: 3, message: "execution reverted", data: encodeErrorResult({ abi: PERPS_ABI, errorName: "PerpsNotPositionHolder" }) } },
    {
      error: {
        code: 3,
        message: "execution reverted",
        data: encodeErrorResult({
          abi: [{ type: "error", name: "Error", inputs: [{ type: "string", name: "reason" }] }],
          errorName: "Error",
          args: ["perps: the perps keeper is not wired"],
        }),
      },
    },
    { result: "0x" },
  ];
  const senders: (string | undefined)[] = [];
  await gateway(
    (call, name) => {
      assert.equal(name, "closePosition");
      senders.push(call.from);
      return replies.shift()!;
    },
    async () => {
      assert.deepEqual(await preflightPerpsWrite(TRADER, close), {
        ok: false,
        detail: "The perps precompile refused: PerpsNotPositionHolder",
      });
      assert.deepEqual(await preflightPerpsWrite(TRADER, close), {
        ok: false,
        detail: "The perps precompile refused: perps: the perps keeper is not wired",
      });
      assert.deepEqual(await preflightPerpsWrite(TRADER, close), { ok: true });
      assert.deepEqual(await preflightPerpsWrite("0x12", close), { ok: false, detail: "The perps write is malformed." });
    },
  );
  assert.deepEqual(senders, [TRADER, TRADER, TRADER]);
});

test("a market read refusal names the custom error", async () => {
  await gateway(
    () => ({ error: { code: 3, message: "execution reverted", data: encodeErrorResult({ abi: PERPS_ABI, errorName: "PerpsPaused" }) } }),
    async () => {
      await assert.rejects(precompiles.perpsMarkets(), /The perps precompile refused: PerpsPaused/u);
    },
  );
});

class MemoryStorage {
  readonly #items = new Map<string, string>();
  getItem(key: string) {
    return this.#items.get(key) ?? null;
  }
  setItem(key: string, value: string) {
    this.#items.set(key, value);
  }
  removeItem(key: string) {
    this.#items.delete(key);
  }
}

function walletWindow(answer: (method: string, params: readonly unknown[]) => unknown) {
  const requests: { method: string; params: readonly unknown[] }[] = [];
  const paxeer = {
    isPaxeer: true,
    request: async ({ method, params = [] }: { method: string; params?: readonly unknown[] }) => {
      requests.push({ method, params });
      return answer(method, params);
    },
  };
  Object.assign(globalThis, {
    window: { ethereum: paxeer, localStorage: new MemoryStorage(), sessionStorage: new MemoryStorage(), confirm: () => true },
  });
  return requests;
}

async function opening_a_position_sends_the_precompile_perps_write_without_an_approve() {
  const collateral = { token: USDL, decimals: precompiles.PERPS_COLLATERAL_DECIMALS, symbol: "USDL" };
  const order: PerpsOpenPosition = {
    marketId: 3n,
    collateralToken: USDL,
    collateralAmount: 2_000_000n,
    leverage: 5n * 10n ** 18n,
    isLong: false,
  };
  const call = client.perpsMarketPositionCall(order.marketId, collateral, order.collateralAmount, order.leverage, order.isLong);
  assert.deepEqual(call, sdk.perpsOpenPositionCall(order));
  assert.equal(call.to, PERPS);
  assert.equal(call.data.slice(0, 10), sdk.abiSelector("openPosition(uint256,address,uint256,uint256,bool)"));
  assert.equal(
    call.data,
    encodeFunctionData({
      abi: PERPS_ABI,
      functionName: "openPosition",
      args: [order.marketId, order.collateralToken, order.collateralAmount, order.leverage, order.isLong],
    }),
  );
  const writeHash = `0x${"bb".repeat(32)}`;
  const receipts = new Set<string>();
  const requests = walletWindow((method, params) => {
    switch (method) {
      case "eth_call":
        return ZERO_WORD;
      case "eth_sendTransaction":
        assert.equal((params[0] as { to: string }).to.toLowerCase(), PERPS);
        receipts.add(writeHash);
        return writeHash;
      case "eth_getTransactionReceipt":
        return { status: "0x1" };
      default:
        throw new Error(`unexpected ${method}`);
    }
  });
  const approving: boolean[] = [];
  try {
    await gateway(
      (preflight, name, args) => {
        assert.equal(name, "openPosition");
        assert.equal(preflight.from, TRADER);
        assert.deepEqual(args.map((arg) => typeof arg === "string" ? arg.toLowerCase() : arg),
          [order.marketId, order.collateralToken, order.collateralAmount, order.leverage, order.isLong]);
        return { result: encodeFunctionResult({ abi: PERPS_ABI, functionName: "openPosition", result: 4n }) };
      },
      async (called) => {
        const outcome = await client.sendPerpsWrite(TRADER, collateral, 0n, call, (active) => approving.push(active));
        assert.deepEqual(outcome, {
          outcome: "sent",
          transactionHash: writeHash,
          receipt: { transactionHash: writeHash, blockNumber: 1n, status: "success", logs: [] },
        });
        assert.deepEqual(called, ["openPosition"]);
      },
      receipts,
    );
    const sends = requests
      .filter((request) => request.method === "eth_sendTransaction")
      .map((request) => request.params[0] as { to: string; data: string });
    assert.equal(sends.length, 1);
    assert.equal(sends[0]!.to.toLowerCase(), PERPS);
    assert.equal(sends[0]!.data, call.data);
    assert.ok(requests.every((request) => request.method !== "eth_call"));
    assert.ok(sends.every((send) => !send.data.startsWith(sdk.abiSelector("approve(address,uint256)"))));
    assert.deepEqual(approving, [false]);
    const cancel = client.perpsOrderCancelCall(9n);
    assert.deepEqual(cancel, sdk.perpsCancelOrderCall(9n));
    assert.equal(cancel.to, PERPS);
    assert.equal(cancel.data.slice(0, 10), sdk.abiSelector("cancelOrder(uint256)"));
    assert.equal(cancel.data, encodeFunctionData({ abi: PERPS_ABI, functionName: "cancelOrder", args: [9n] }));
    await gateway(
      (preflight, name, args) => {
        assert.equal(name, "cancelOrder");
        assert.equal(preflight.from, TRADER);
        assert.deepEqual(args, [9n]);
        return { result: encodeFunctionResult({ abi: PERPS_ABI, functionName: "cancelOrder" }) };
      },
      async (called) => {
        const outcome = await client.sendPerpsWrite(TRADER, collateral, 0n, cancel, (active) => approving.push(active));
        assert.deepEqual(outcome, {
          outcome: "sent",
          transactionHash: writeHash,
          receipt: { transactionHash: writeHash, blockNumber: 1n, status: "success", logs: [] },
        });
        assert.deepEqual(called, ["cancelOrder"]);
      },
      receipts,
    );
    const allSends = requests.filter((request) => request.method === "eth_sendTransaction")
      .map((request) => request.params[0] as { to: string; data: string });
    assert.deepEqual(allSends.map((send) => send.data), [call.data, cancel.data]);
    assert.ok(allSends.every((send) => send.to.toLowerCase() === PERPS));
    assert.ok(requests.every((request) => request.method !== "eth_call"));
    assert.deepEqual(approving, [false, false]);
  } finally {
    delete (globalThis as { window?: unknown }).window;
  }
}

test(opening_a_position_sends_the_precompile_perps_write_without_an_approve);

test("a refused perps write is reported with its reason and never reaches the wallet", async () => {
  const collateral = { token: USDL, decimals: precompiles.PERPS_COLLATERAL_DECIMALS, symbol: "USDL" };
  const requests = walletWindow((method) => {
    throw new Error(`unexpected ${method}`);
  });
  try {
    await gateway(
      () => ({
        error: { code: 3, message: "execution reverted", data: encodeErrorResult({ abi: PERPS_ABI, errorName: "PerpsUnknownOrder" }) },
      }),
      async () => {
        const outcome = await client.sendPerpsWrite(TRADER, collateral, 0n, sdk.perpsCancelOrderCall(9n), () => undefined);
        assert.deepEqual(outcome, { outcome: "failed", detail: "The perps precompile refused: PerpsUnknownOrder" });
      },
    );
    assert.deepEqual(requests, []);
  } finally {
    delete (globalThis as { window?: unknown }).window;
  }
});

test("a short allowance under the sponsored SID fee is refused before any send", async () => {
  const collateral = { token: USDL, decimals: precompiles.PERPS_COLLATERAL_DECIMALS, symbol: "USDL" };
  const requests = walletWindow((method) => {
    if (method !== "eth_call") throw new Error(`unexpected ${method}`);
    return ZERO_WORD;
  });
  try {
    assert.equal(wallet.chooseWalletFee("sidiora", "2.1"), true);
    const outcome = await client.sendPerpsWrite(TRADER, collateral, 1_000_000n, sdk.perpsDepositCall(USDL, 1_000_000n), () => undefined);
    assert.equal(outcome.outcome, "failed");
    assert.ok(requests.every((request) => request.method === "eth_call"));
  } finally {
    wallet.chooseWalletFee("paxeer", "");
    delete (globalThis as { window?: unknown }).window;
  }
});

async function the_order_panels_send_limit_stop_limit_TP_SL_partial_close_and_collateral_writes() {
  const { KitButton } = await import("../src/kit/control.tsx");
  const { ChoiceField } = await import("../src/app/_markets/select.tsx");
  const { SegmentedControl } = await import("../src/kit/collection.tsx");
  const chainAbi = JSON.parse(readFileSync(new URL("../../../precompiles/perps/abi/abi.json", import.meta.url), "utf8")) as Abi;
  const erc20 = parseAbi(["function allowance(address owner, address spender) view returns (uint256)", "function approve(address spender, uint256 value)"]);
  const collateral = { token: USDL, decimals: 6, symbol: "USDL" };
  const markets = [{ id: "3", label: "Bitcoin" }];
  const baseInputs: PerpsOrderInputs = {
    market: "3", side: "long", orderType: "limit", triggerPrice: "65000.25", limitPrice: "64900.5",
    size: "50.75", leverage: "5.5", collateral: "2.125001", closeSize: "12.25", takeProfit: "70000.5",
    stopLoss: "60000.75", positionCollateral: "1.500001",
  };
  let inputs = baseInputs;
  let positionId = "4";
  let walletReason: string | undefined;
  let submission: Promise<void> | undefined;
  let pullSent: bigint | undefined;
  let allowance = 0n;
  const receipts = new Set<string>();
  const approving: boolean[] = [];
  const requests = walletWindow((method, params) => {
    if (method === "eth_call") {
      const call = params[0] as { to: string; data: Hex };
      assert.equal(call.to.toLowerCase(), USDL);
      const decoded = decodeFunctionData({ abi: erc20, data: call.data });
      assert.equal(decoded.functionName, "allowance");
      assert.deepEqual(decoded.args.map((arg) => typeof arg === "string" ? arg.toLowerCase() : arg), [TRADER, PERPS]);
      return word(allowance);
    }
    if (method === "eth_sendTransaction") {
      const hash = word(BigInt(receipts.size + 1));
      receipts.add(hash);
      return hash;
    }
    if (method === "eth_getTransactionReceipt") {
      assert.ok(receipts.has(String(params[0])));
      return { status: "0x1" };
    }
    throw new Error(`unexpected wallet request ${method}`);
  });
  function descendants(node: ReactNode): ReactElement<{ children?: ReactNode }>[] {
    if (Array.isArray(node)) return node.flatMap(descendants);
    if (!isValidElement<{ children?: ReactNode }>(node)) return [];
    return [node, ...descendants(node.props.children)];
  }
  const controls = (mode: "entry" | "position") => descendants(client.ExchangeOrderControls({
    mode, inputs, onChange: (key, value) => { inputs = { ...inputs, [key]: value }; },
    collateral, markets, positionId, pending: false, walletReason,
    send: (build, pull) => {
      pullSent = pull;
      submission = client.sendPerpsWrite(TRADER, collateral, pull, build(), (active) => approving.push(active)).then((outcome) => {
        assert.equal(outcome.outcome, "sent", outcome.outcome);
      });
    },
  }));
  function button(mode: "entry" | "position", label: string) {
    const element = controls(mode).find((element) => element.type === KitButton && element.props.children === label);
    assert.ok(isValidElement<{ disabled?: boolean; disabledReason?: string; onClick: () => void }>(element), `missing ${label}`);
    return element.props;
  }
  const sendsSince = (start: number) => requests.slice(start).filter((request) => request.method === "eth_sendTransaction")
    .map((request) => request.params[0] as { to: string; data: Hex });
  const unit = 10n ** 18n;
  const price = 65_000n * unit + unit / 4n;
  const limit = 64_900n * unit + unit / 2n;
  const size = 50n * unit + 3n * unit / 4n;
  const leverage = 5n * unit + unit / 2n;
  const sentSelectors = new Set<string>();
  async function click(mode: "entry" | "position", label: string, functionName: string, args: readonly unknown[], pull = 0n) {
    const start = requests.length;
    const approvalStart = approving.length;
    const control = button(mode, label);
    assert.notEqual(control.disabled, true, label);
    submission = undefined;
    control.onClick();
    assert.ok(submission !== undefined, `${label} did not submit`);
    await submission;
    assert.equal(pullSent, pull, `${label} collateral pull`);
    const sends = sendsSince(start);
    const needsApprove = pull > allowance;
    assert.equal(sends.length, needsApprove ? 2 : 1, `${label} send count`);
    if (needsApprove) {
      const approval = sends[0]!;
      assert.equal(approval.to.toLowerCase(), USDL);
      const decoded = decodeFunctionData({ abi: erc20, data: approval.data });
      assert.equal(decoded.functionName, "approve");
      assert.deepEqual(decoded.args, [PERPS, pull]);
      const sequence = requests.slice(start).map((request) => request.method);
      assert.ok(sequence.indexOf("eth_getTransactionReceipt") < sequence.lastIndexOf("eth_sendTransaction"), "approve must be mined before the perps write");
      assert.deepEqual(approving.slice(approvalStart), [true, false]);
    } else {
      assert.deepEqual(approving.slice(approvalStart), [false]);
    }
    const write = sends.at(-1)!;
    assert.equal(write.to.toLowerCase(), PERPS);
    const decoded = decodeFunctionData({ abi: chainAbi, data: write.data });
    assert.equal(decoded.functionName, functionName);
    assert.deepEqual(decoded.args?.map((arg) => typeof arg === "string" ? arg.toLowerCase() : arg), args);
    assert.equal(write.data, encodeFunctionData({ abi: chainAbi, functionName, args }));
    sentSelectors.add(write.data.slice(0, 10));
  }
  try {
    wallet.chooseWalletFee("paxeer", "");
    await gateway((call, name) => {
      assert.equal(call.from, TRADER);
      assert.ok(["placeLimitOrder", "placeStopLimitOrder", "placeTakeProfitOrder", "placeStopLossOrder", "setPositionTpSl", "partialClose", "addCollateral", "removeCollateral"].includes(name));
      return { result: "0x" };
    }, async (called) => {
      await click("entry", "Place limit order", "placeLimitOrder", [3n, true, price, size, leverage, USDL, 2_125_001n], 2_125_001n);
      const orderType = controls("entry").find((element) => element.type === ChoiceField &&
        isValidElement<{ label: string }>(element) && element.props.label === "Order type");
      assert.ok(isValidElement<{ onChange: (value: string) => void }>(orderType));
      orderType.props.onChange("stop-limit");
      const side = controls("entry").find((element) => element.type === SegmentedControl);
      assert.ok(isValidElement<{ onValueChange: (value: string) => void }>(side));
      side.props.onValueChange("short");
      await click("entry", "Place stop-limit order", "placeStopLimitOrder", [3n, false, price, limit, size, leverage, USDL, 2_125_001n], 2_125_001n);
      await click("position", "Partially close position", "partialClose", [4n, 12n * unit + unit / 4n]);
      await click("position", "Place take-profit order", "placeTakeProfitOrder", [4n, 70_000n * unit + unit / 2n]);
      await click("position", "Place stop-loss order", "placeStopLossOrder", [4n, 60_000n * unit + 3n * unit / 4n]);
      await click("position", "Set position TP/SL", "setPositionTpSl", [4n, 70_000n * unit + unit / 2n, 60_000n * unit + 3n * unit / 4n]);
      await click("position", "Add collateral", "addCollateral", [4n, 1_500_001n], 1_500_001n);
      await click("position", "Remove collateral", "removeCollateral", [4n, 1_500_001n]);
      allowance = 2_125_001n;
      inputs = baseInputs;
      await click("entry", "Place limit order", "placeLimitOrder", [3n, true, price, size, leverage, USDL, allowance], allowance);
      assert.deepEqual(called, ["placeLimitOrder", "placeStopLimitOrder", "partialClose", "placeTakeProfitOrder", "placeStopLossOrder", "setPositionTpSl", "addCollateral", "removeCollateral", "placeLimitOrder"]);

      const invalid: readonly [keyof PerpsOrderInputs, "entry" | "position", string][] = [
        ["market", "entry", "Place limit order"], ["triggerPrice", "entry", "Place limit order"],
        ["size", "entry", "Place limit order"], ["leverage", "entry", "Place limit order"],
        ["collateral", "entry", "Place limit order"], ["limitPrice", "entry", "Place stop-limit order"],
        ["closeSize", "position", "Partially close position"], ["takeProfit", "position", "Place take-profit order"],
        ["stopLoss", "position", "Place stop-loss order"], ["takeProfit", "position", "Set position TP/SL"],
        ["stopLoss", "position", "Set position TP/SL"], ["positionCollateral", "position", "Add collateral"],
        ["positionCollateral", "position", "Remove collateral"],
      ];
      const requestCount = requests.length;
      for (const [key, mode, label] of invalid) {
        for (const value of ["", "bad", "-1", "0", "0.0000000000000000001", (1n << 256n).toString()]) {
          inputs = { ...baseInputs, orderType: label === "Place stop-limit order" ? "stop-limit" : "limit", [key]: value };
          const control = button(mode, label);
          assert.equal(control.disabled, true, `${key} accepted ${value}`);
          assert.ok(control.disabledReason);
          control.onClick();
        }
      }
      inputs = baseInputs;
      positionId = "";
      for (const label of ["Partially close position", "Place take-profit order", "Place stop-loss order", "Set position TP/SL", "Add collateral", "Remove collateral"]) {
        assert.equal(button("position", label).disabled, true);
      }
      positionId = "4";
      walletReason = "Connect a wallet first.";
      const disconnected = button("entry", "Place limit order");
      assert.equal(disconnected.disabled, true);
      disconnected.onClick();
      assert.equal(requests.length, requestCount, "invalid inputs must not send");
    }, receipts);

    const marketOpen = client.perpsMarketPositionCall(3n, collateral, 2_125_001n, leverage, true);
    assert.ok(isHex(marketOpen.data));
    const decodedMarketOpen = decodeFunctionData({ abi: chainAbi, data: marketOpen.data });
    assert.equal(decodedMarketOpen.functionName, "openPosition");
    assert.deepEqual(decodedMarketOpen.args?.map((arg) => typeof arg === "string" ? arg.toLowerCase() : arg),
      [3n, collateral.token, 2_125_001n, leverage, true]);
    const cancel = client.perpsOrderCancelCall(9n);
    assert.ok(isHex(cancel.data));
    assert.deepEqual(decodeFunctionData({ abi: chainAbi, data: cancel.data }), {
      functionName: "cancelOrder", args: [9n],
    });
    for (const call of [
      sdk.perpsDepositCall(USDL, 1n), sdk.perpsWithdrawCall(USDL, 1n), sdk.perpsClosePositionCall(4n),
      marketOpen, cancel,
    ]) sentSelectors.add(call.data.slice(0, 10));
    const policy = JSON.parse(readFileSync(new URL("../../../services/wallet/deploy/attestor-policy.json", import.meta.url), "utf8")) as {
      defaults: { selectors: Record<string, string[]> };
    };
    const allowed = policy.defaults.selectors[PERPS];
    assert.ok(allowed !== undefined);
    for (const selector of sentSelectors) assert.ok(allowed.includes(selector), `page selector ${selector} is absent from the attestor policy`);
  } finally {
    wallet.chooseWalletFee("paxeer", "");
    delete (globalThis as { window?: unknown }).window;
  }
}

Object.defineProperty(the_order_panels_send_limit_stop_limit_TP_SL_partial_close_and_collateral_writes, "name", {
  value: "the order panels send limit, stop-limit, TP/SL, partial close and collateral writes",
});
test(the_order_panels_send_limit_stop_limit_TP_SL_partial_close_and_collateral_writes);

function exchange_rejects_malformed_send_outcomes_before_refresh() {
  assert.equal(wallet.isPrecompileSendOutcome({ outcome: "sent", transactionHash: "0x1234" }), false);
  assert.equal(wallet.isPrecompileSendOutcome({ outcome: "cancelled" }), true);
  assert.equal(wallet.isPrecompileSendOutcome({ outcome: "reverted", transactionHash: `0x${"11".repeat(32)}` }), false);
}

test(exchange_rejects_malformed_send_outcomes_before_refresh);
