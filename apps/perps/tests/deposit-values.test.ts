import assert from "node:assert/strict";
import { createRequire } from "node:module";
import { test, type TestContext } from "node:test";
import { isMainThread, parentPort, Worker } from "node:worker_threads";
import type { Address, Hex } from "viem";
import type { ContextType } from "react";

async function exerciseDepositValues() {
  const chainOrigin = "https://api1.mainnet-beta.paxeer.network";
  Object.assign(process.env, {
    NEXT_PUBLIC_PAXEER_RPC_URLS: chainOrigin,
    NEXT_PUBLIC_PERPS_GATEWAY_URL: chainOrigin,
    NEXT_PUBLIC_PAXSCAN_BASE_URL: chainOrigin,
    NEXT_PUBLIC_PAXSCAN_API_URL: `${chainOrigin}/api/v2`,
    NEXT_PUBLIC_PAXSCAN_WS_URL: chainOrigin.replace("https:", "wss:"),
  });
  const { Window } = await import("happy-dom");
  const dom = new Window();
  for (const [name, value] of Object.entries({
    window: dom,
    document: dom.document,
    navigator: dom.navigator,
    localStorage: dom.localStorage,
    Event: dom.Event,
    CustomEvent: dom.CustomEvent,
    HTMLElement: dom.HTMLElement,
    HTMLFormElement: dom.HTMLFormElement,
    HTMLInputElement: dom.HTMLInputElement,
    HTMLSelectElement: dom.HTMLSelectElement,
    Element: dom.Element,
    Node: dom.Node,
    MutationObserver: dom.MutationObserver,
    ResizeObserver: dom.ResizeObserver,
    getComputedStyle: dom.getComputedStyle.bind(dom),
    requestAnimationFrame: dom.requestAnimationFrame.bind(dom),
    cancelAnimationFrame: dom.cancelAnimationFrame.bind(dom),
  }))
    Object.defineProperty(globalThis, name, { value, configurable: true });
  for (const name of Object.getOwnPropertyNames(dom)) {
    if (/^(HTML|SVG|Document|Node|Element|DOM)|Event$|Observer$/.test(name)) {
      Object.defineProperty(globalThis, name, {
        value: (dom as unknown as Record<string, unknown>)[name],
        configurable: true,
        writable: true,
      });
    }
  }
  await import("fake-indexeddb/auto");
  const { indexedDB, IDBKeyRange } = await import("fake-indexeddb");
  Object.assign(globalThis, { indexedDB, IDBKeyRange });
  const React = await import("react");
  Object.assign(globalThis, { React });
  const { createElement } = React;
  const { act, render, fireEvent } = await import("@testing-library/react");
  const { decodeFunctionData, encodeFunctionResult, toHex } =
    await import("viem");
  const { erc20Abi, COLLATERAL_TOKENS } =
    await import("../packages/paxeer-backend/src/core/perps/config");
  const { PerpsPrecompileABI } =
    await import("../packages/paxeer-backend/src/core/network/abi/PerpsPrecompile");
  const { PERPS_SPENDER } =
    await import("../packages/paxeer-backend/src/core/network/approvals");
  const { paxeerChain } =
    await import("../packages/paxeer-backend/src/core/chain");
  const token = COLLATERAL_TOKENS.usdl;
  const owner = token.address;
  function deferred() {
    let resolve!: () => void;
    const promise = new Promise<void>((done) => {
      resolve = done;
    });
    return { promise, resolve };
  }
  const balance = deferred();
  const allowance = deferred();
  const fee = deferred();
  const reads = { balance: 0, allowance: 0, fee: 0 };
  const violations: unknown[] = [];
  const requests: string[] = [];
  let delayReads = false;
  const gasPrice = 10n ** 13n;

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
      return [
        {
          parentCapability: "eth_accounts",
          caveats: [{ type: "restrictReturnedAccounts", value: [owner] }],
        },
      ];
    if (method === "net_listening") return true;
    if (method === "eth_blockNumber") return "0x1";
    if (method === "eth_getBalance") return toHex(10n ** 18n);
    if (method === "eth_getCode") return "0x";
    if (method === "eth_gasPrice" || method === "eth_maxPriorityFeePerGas")
      return toHex(gasPrice);
    const call = params[0] as { to: Address; data: Hex };
    const abi =
      call.to.toLowerCase() === PERPS_SPENDER.toLowerCase()
        ? PerpsPrecompileABI
        : erc20Abi;
    const decoded = decodeFunctionData({ abi, data: call.data });
    if (
      method === "eth_call" &&
      (decoded.functionName === "getBalance" ||
        decoded.functionName === "getLockedBalance")
    ) {
      assert.equal(call.to.toLowerCase(), PERPS_SPENDER.toLowerCase());
      return encodeFunctionResult({
        abi: PerpsPrecompileABI,
        functionName: decoded.functionName,
        args: decoded.args,
        result: 0n,
      });
    }
    if (method === "eth_call" && decoded.functionName === "balanceOf") {
      reads.balance++;
      if (delayReads) await balance.promise;
      return encodeFunctionResult({
        abi: erc20Abi,
        functionName: "balanceOf",
        result: 100_000_000n,
      });
    }
    if (method === "eth_call" && decoded.functionName === "allowance") {
      reads.allowance++;
      if (delayReads) await allowance.promise;
      assert.deepEqual(decoded.args, [owner, PERPS_SPENDER]);
      return encodeFunctionResult({
        abi: erc20Abi,
        functionName: "allowance",
        result: 20_000_000n,
      });
    }
    if (method === "eth_estimateGas") {
      reads.fee++;
      assert.ok(["approve", "deposit"].includes(decoded.functionName));
      if (delayReads) await fee.promise;
      return decoded.functionName === "approve" ? "0x64" : "0xc8";
    }
    throw new Error(
      `Unexpected chain method ${method}: ${decoded.functionName}`,
    );
  }
  globalThis.fetch = async (input, init) => {
    const url = input instanceof Request ? input.url : String(input);
    requests.push(url);
    assert.equal(new URL(url).origin, chainOrigin);
    const body = JSON.parse(String(init?.body));
    try {
      return Response.json({
        jsonrpc: "2.0",
        id: body.id,
        result: await request(body),
      });
    } catch (error) {
      violations.push(error);
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
  const { connect } = backendRequire(
    "wagmi/actions",
  ) as typeof import("../packages/paxeer-backend/node_modules/wagmi/actions");
  const { injected } = backendRequire(
    "wagmi/connectors",
  ) as typeof import("../packages/paxeer-backend/node_modules/wagmi/connectors");
  const { wagmiConfig } =
    await import("../packages/paxeer-backend/src/core/wagmi");
  await connect(wagmiConfig, {
    connector: injected({
      target: { id: "deposit-values", name: "Deposit wallet", provider },
    }),
  });
  const { updateNativeSnapshot } =
    await import("../packages/paxeer-backend/src/runtime");
  updateNativeSnapshot({
    address: owner,
    chainId: paxeerChain.id,
    connected: true,
    markets: [],
  });
  const { nativeChains, nativeTokens } =
    await import("../packages/paxeer-backend/src/nativeMetadata");
  const chains = nativeChains();
  assert.equal(nativeTokens()[0].collateral_ratio, 1);
  const { Account, DefaultConfigStore, LocalStorageStore, SimpleDI } =
    await import("../packages/core/src/index");
  const { DefaultEVMWalletAdapter } =
    await import("../packages/default-evm-adapter/src/index");
  const { EthersProvider } =
    await import("../packages/web3-provider-ethers/src/index");
  const configStore = new DefaultConfigStore({ networkId: "mainnet" });
  const keyStore = new LocalStorageStore();
  const adapters = [new DefaultEVMWalletAdapter(new EthersProvider())];
  const account = new Account(configStore, keyStore, adapters);
  await account.setAddress(owner, {
    provider,
    chain: { id: paxeerChain.id, namespace: "EVM" },
    wallet: { name: "Deposit wallet" },
  });
  SimpleDI.registerByName("account", account);
  const { PaxeerNetworkContext } =
    await import("../packages/hooks/src/paxeerNetworkContext");
  const { WalletConnectorContext } =
    await import("../packages/hooks/src/walletConnectorContext");
  const { useMainTokenStore } =
    await import("../packages/hooks/src/provider/store/mainTokenStore");
  const { useTestTokenStore } =
    await import("../packages/hooks/src/provider/store/testTokenStore");
  const { useMainnetChainsStore, useTestnetChainsStore } =
    await import("../packages/hooks/src/provider/store/index");
  useMainTokenStore.setState({ data: [] });
  useTestTokenStore.setState({ data: [] });
  useMainnetChainsStore.setState({ data: [] });
  useTestnetChainsStore.setState({ data: [] });
  const hooksRequire = createRequire(
    new URL("../packages/hooks/package.json", import.meta.url),
  );
  const { SWRConfig } = hooksRequire(
    "swr",
  ) as typeof import("../packages/hooks/node_modules/swr");
  const { useDepositFormScript } =
    await import("../packages/ui-transfer/src/components/depositForm/depositForm.script");
  const { QuantityInput } =
    await import("../packages/ui-transfer/src/components/quantityInput/index");
  const { CollateralContribution } =
    await import("../packages/ui-transfer/src/components/collateralContribution/index");
  const { Fee } =
    await import("../packages/ui-transfer/src/components/fee/index");
  const { DepositProvider } =
    await import("../packages/ui-transfer/src/components/provider/DepositProvider");
  const { DepositAction } = await import("../packages/ui-transfer/src/types");
  const { useDepositFee } =
    await import("../packages/ui-transfer/src/components/depositForm/hooks/useDepositFee");
  const { useCollateralValue } =
    await import("../packages/ui-transfer/src/components/depositForm/hooks/useCollateralValue");
  let latest!: ReturnType<typeof useDepositFormScript>;
  let referenceFee!: ReturnType<typeof useDepositFee>;
  let referenceCollateral!: ReturnType<typeof useCollateralValue>;
  function Probe() {
    latest = useDepositFormScript({});
    referenceFee = useDepositFee({
      chainId: 1,
      nativeSymbol: "ETH",
      depositFee: 2n * 10n ** 15n,
      networkId: "mainnet",
      getIndexPrice: () => 2000,
    });
    referenceCollateral = useCollateralValue({
      chainId: 1,
      sourceToken: { symbol: "ETH", user_max_qty: -1 },
      targetToken: { base_weight: 0.9, discount_factor: 0 },
      quantity: "10",
      indexPrice: 2000,
    });
    return createElement(
      DepositProvider,
      { collateralUnit: latest.collateralUnit },
      createElement(QuantityInput, {
        token: latest.sourceToken,
        tokens: latest.sourceTokens,
        value: latest.quantity,
        onValueChange: latest.onQuantityChange,
      }),
      createElement(
        "div",
        { "data-testid": "contribution" },
        createElement(CollateralContribution, {
          value: latest.collateralContributionQuantity,
        }),
      ),
      createElement(
        "div",
        { "data-testid": "fee" },
        createElement(Fee, {
          ...latest.fee,
          nativeSymbol: latest.nativeSymbol,
        }),
      ),
    );
  }
  const context = {
    configStore,
    keyStore,
    walletAdapters: adapters,
    networkId: "mainnet" as const,
    customChains: { mainnet: chains, testnet: [] },
  };
  const cache = new Map();
  const connector = {
    connectedChain: { id: paxeerChain.id, namespace: "EVM" as const },
  };
  function tree() {
    return createElement(
      SWRConfig,
      { value: { provider: () => cache, shouldRetryOnError: false } },
      createElement(
        PaxeerNetworkContext.Provider,
        { value: context },
        createElement(
          WalletConnectorContext.Provider,
          { value: connector as ContextType<typeof WalletConnectorContext> },
          createElement(Probe),
        ),
      ),
    );
  }
  delayReads = true;
  let renderer!: ReturnType<typeof render>;
  await act(async () => {
    renderer = render(tree());
  });
  async function settle(condition: () => boolean) {
    for (let i = 0; i < 100 && !condition(); i++)
      await act(async () => {
        await new Promise((resolve) => setTimeout(resolve, 20));
      });
    assert.ok(condition(), "deposit values settle");
  }
  const input = () => {
    const element = renderer.container.querySelector<HTMLInputElement>("input");
    assert.ok(element);
    return element;
  };
  await settle(
    () => !!latest.sourceToken && reads.balance > 0 && reads.allowance > 0,
  );
  assert.equal(latest.sourceToken?.symbol, "USDL");
  await act(async () => {
    fireEvent.change(input(), { target: { value: "10", selectionStart: 2 } });
  });
  assert.equal(latest.loading, true);
  const quantityInput = input();
  assert.equal(input().value, "10");
  assert.equal(input(), quantityInput);
  assert.equal(latest.collateralRatio, 1);
  assert.equal(latest.collateralContributionQuantity, 10);
  assert.equal(latest.collateralUnit?.symbol, "USDL");
  assert.equal(latest.quantityNotional, 10);
  assert.equal(latest.currentLTV, 0);
  assert.equal(latest.nextLTV, 0);
  await act(async () => {
    balance.resolve();
  });
  await settle(
    () => latest.maxQuantity === "100" && !latest.balanceRevalidating,
  );
  assert.equal(input().value, "10");
  assert.equal(input(), quantityInput);
  await act(async () => {
    allowance.resolve();
  });
  await settle(
    () => latest.actionType === DepositAction.Deposit && reads.fee > 0,
  );
  assert.equal(input().value, "10");
  assert.equal(input(), quantityInput);
  await act(async () => {
    fee.resolve();
  });
  await settle(() => !latest.loading && latest.fee.dstGasFee === "0.002");
  assert.equal(input().value, "10");
  assert.equal(input(), quantityInput);
  assert.equal(latest.fee.feeAmount, undefined);
  const feeText = () => renderer.getByTestId("fee").textContent!;
  assert.ok(feeText().includes("PAX"));
  assert.ok(feeText().includes("0.002"));
  assert.ok(!feeText().includes("$"));
  const contributionText = renderer.getByTestId("contribution").textContent!;
  assert.ok(contributionText.includes("10"));
  assert.ok(contributionText.includes("USDL"));
  assert.ok(!contributionText.includes("USDC"));
  await act(async () => {
    updateNativeSnapshot({
      markets: [{ symbol: "PAX", index_price: 4, price: 7 }],
    });
  });
  assert.equal(latest.fee.feeAmount, "0.008");
  assert.ok(feeText().includes("$"));
  assert.equal(input().value, "10");
  assert.equal(input(), quantityInput);
  await act(async () => {
    updateNativeSnapshot({ markets: [{ symbol: "PAX", price: 7 }] });
  });
  assert.equal(latest.fee.feeAmount, undefined);
  assert.ok(!feeText().includes("$"));
  assert.equal(referenceFee.feeAmount, "4");
  assert.equal(referenceCollateral.collateralRatio, 0.9);
  assert.equal(referenceCollateral.collateralContributionQuantity, 18000);
  const selectedToken = latest.sourceToken;
  await act(async () => {
    context.customChains = { mainnet: nativeChains(), testnet: [] };
    renderer.rerender(tree());
  });
  assert.equal(input().value, "10");
  assert.equal(input(), quantityInput);
  assert.notEqual(latest.sourceToken, selectedToken);
  assert.ok(requests.length > 0);
  assert.deepEqual(violations, []);
  await act(async () => {
    renderer.unmount();
  });
}

async function nativeDepositValues(t: TestContext) {
  const worker = new Worker(new URL(import.meta.url), {
    execArgv: ["--import", "tsx"],
  });
  t.after(async () => {
    await worker.terminate();
  });
  await new Promise<void>((resolve, reject) => {
    worker.once("error", reject);
    worker.once("exit", (code) =>
      reject(new Error(`Deposit values exited before completion: ${code}`)),
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

if (isMainThread) test(nativeDepositValues);
else
  void exerciseDepositValues().then(() => {
    assert.ok(parentPort);
    parentPort.postMessage("assertions-passed");
  });
