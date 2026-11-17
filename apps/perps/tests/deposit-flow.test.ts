import assert from "node:assert/strict";
import { createRequire } from "node:module";
import { test, type TestContext } from "node:test";
import { isMainThread, parentPort, Worker } from "node:worker_threads";
import type { Address, Hex } from "viem";

async function exerciseDepositFlow() {
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
  }))
    Object.defineProperty(globalThis, name, { value, configurable: true });
  await import("fake-indexeddb/auto");
  const { indexedDB, IDBKeyRange } = await import("fake-indexeddb");
  Object.assign(globalThis, { indexedDB, IDBKeyRange });
  const React = await import("react");
  Object.assign(globalThis, { React });
  const { createElement } = React;
  const { act, create } = await import("react-test-renderer");
  const { decodeFunctionData, encodeFunctionResult, toHex, parseUnits } =
    await import("viem");
  const { erc20Abi, COLLATERAL_TOKENS } =
    await import("@paxeer-network/paxeer-backend/core/perps/config");
  const { PerpsPrecompileABI } =
    await import("@paxeer-network/paxeer-backend/core/network/abi/PerpsPrecompile");
  const { PERPS_SPENDER } =
    await import("@paxeer-network/paxeer-backend/core/network/approvals");
  const { paxeerChain } =
    await import("@paxeer-network/paxeer-backend/core/chain");
  const token = COLLATERAL_TOKENS.usdl;
  const owner = token.address;
  let allowance = 0n;
  let estimationFails = false;
  let depositRequiresApproval = false;
  const estimates: string[] = [];
  const sent: string[] = [];
  const requests: string[] = [];
  const forbidden: string[] = [];
  const receipts = new Map<Hex, Record<string, unknown>>();
  const violations: unknown[] = [];
  let expectedAmount = 1_000_000n;
  let transactionCount = 0n;

  function decode(call: { to: Address; data: Hex }) {
    return decodeFunctionData({
      abi:
        call.to.toLowerCase() === PERPS_SPENDER.toLowerCase()
          ? PerpsPrecompileABI
          : erc20Abi,
      data: call.data,
    });
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
      return [
        {
          parentCapability: "eth_accounts",
          caveats: [{ type: "restrictReturnedAccounts", value: [owner] }],
        },
      ];
    if (method === "net_listening") return true;
    if (method === "eth_blockNumber") return "0x1";
    if (method === "eth_getBalance") return toHex(10n ** 18n);
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
    const call = params[0] as { to: Address; data: Hex; from?: Address };
    const decoded = decode(call);
    if (method === "eth_call") {
      if (decoded.functionName === "balanceOf")
        return encodeFunctionResult({
          abi: erc20Abi,
          functionName: "balanceOf",
          result: 100_000_000n,
        });
      if (decoded.functionName === "allowance") {
        assert.deepEqual(decoded.args, [owner, PERPS_SPENDER]);
        return encodeFunctionResult({
          abi: erc20Abi,
          functionName: "allowance",
          result: allowance,
        });
      }
      assert.ok(["approve", "deposit"].includes(decoded.functionName));
      if (decoded.functionName === "approve") {
        return encodeFunctionResult({
          abi: erc20Abi,
          functionName: "approve",
          result: true,
        });
      }
      return "0x";
    }
    if (method === "eth_estimateGas") {
      estimates.push(decoded.functionName);
      assert.equal(call.from?.toLowerCase(), owner.toLowerCase());
      if (decoded.functionName === "approve")
        assert.deepEqual(decoded.args, [PERPS_SPENDER, expectedAmount]);
      else assert.deepEqual(decoded.args, [token.address, expectedAmount]);
      if (
        estimationFails ||
        (depositRequiresApproval &&
          decoded.functionName === "deposit" &&
          allowance < 1_000_000n)
      )
        throw new Error("Gas estimate unavailable");
      return decoded.functionName === "approve" ? "0x64" : "0xc8";
    }
    if (method === "eth_sendTransaction") {
      sent.push(decoded.functionName);
      if (decoded.functionName === "approve") {
        assert.deepEqual(decoded.args, [PERPS_SPENDER, 1_000_000n]);
        allowance = 1_000_000n;
      } else {
        assert.equal(decoded.functionName, "deposit");
        assert.deepEqual(decoded.args, [token.address, 1_000_000n]);
        assert.ok(allowance >= 1_000_000n);
        allowance -= 1_000_000n;
      }
      const hash = toHex(++transactionCount, { size: 32 });
      receipts.set(hash, {
        transactionHash: hash,
        blockNumber: "0x1",
        blockHash: toHex(1n, { size: 32 }),
        transactionIndex: "0x0",
        from: owner,
        to: call.to,
        status: "0x1",
        type: "0x0",
        cumulativeGasUsed: "0xc8",
        gasUsed: "0xc8",
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
    requests.push(url);
    if (new URL(url).origin !== chainOrigin) {
      forbidden.push(url);
      throw new Error("Only chain requests are permitted");
    }
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
  const { connect } = backendRequire(
    "wagmi/actions",
  ) as typeof import("../packages/paxeer-backend/node_modules/wagmi/actions");
  const { injected } = backendRequire(
    "wagmi/connectors",
  ) as typeof import("../packages/paxeer-backend/node_modules/wagmi/connectors");
  const { wagmiConfig } =
    await import("@paxeer-network/paxeer-backend/core/wagmi");
  await connect(wagmiConfig, {
    connector: injected({
      target: { id: "deposit", name: "Deposit wallet", provider },
    }),
  });
  const { updateNativeSnapshot } =
    await import("@paxeer-network/paxeer-backend/runtime");
  updateNativeSnapshot({
    address: owner,
    chainId: paxeerChain.id,
    connected: true,
  });
  const { nativeChains } =
    await import("@paxeer-network/paxeer-backend/nativeMetadata");
  const chains = nativeChains();
  assert.deepEqual(
    chains[0].token_infos.map((item) => item.symbol),
    ["USDL"],
  );
  assert.equal(chains[0].network_infos.vault_address, PERPS_SPENDER);
  const { Account, DefaultConfigStore, LocalStorageStore, SimpleDI } =
    await import("@paxeer-network/core");
  const { DefaultEVMWalletAdapter } =
    await import("../packages/default-evm-adapter/src/index");
  const { EthersProvider } =
    await import("../packages/web3-provider-ethers/src/index");
  const configStore = new DefaultConfigStore({ networkId: "testnet" });
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
  const { useDeposit } =
    await import("../packages/hooks/src/paxeer-network/useDeposit");
  const { useDepositValidation } =
    await import("../packages/ui-transfer/src/components/depositForm/hooks/useDepositValidation");
  const { useActionType } =
    await import("../packages/ui-transfer/src/components/depositForm/hooks/useActionType");
  const { useDepositAction } =
    await import("../packages/ui-transfer/src/components/depositForm/hooks/useDepositAction");
  const { useSwapTokens } =
    await import("../packages/ui-transfer/src/components/depositForm/swap/use1inchTokens");
  const { useSwapQuote } =
    await import("../packages/ui-transfer/src/components/depositForm/swap/useSwapQuote");
  const { DepositAction } = await import("../packages/ui-transfer/src/types");
  const context = {
    configStore,
    keyStore,
    walletAdapters: adapters,
    networkId: "testnet" as const,
    customChains: { mainnet: chains, testnet: [] },
  };

  async function scenario(
    initialAllowance: bigint,
    fails: boolean,
    partial: boolean,
  ) {
    allowance = initialAllowance;
    estimationFails = fails;
    depositRequiresApproval = partial;
    estimates.length = 0;
    sent.length = 0;
    const cache = new Map();
    let latest: ReturnType<typeof ProbeState>;
    let validationChain = paxeerChain.id;
    let cap: number | undefined;
    function ProbeState() {
      const deposit = useDeposit({
        address: token.address,
        decimals: 6,
        srcChainId: paxeerChain.id,
        srcToken: "USDL",
        dstToken: "USDL",
      });
      const sourceToken = { ...chains[0].token_infos[0], user_max_qty: cap };
      const validation = useDepositValidation({
        sourceToken,
        targetToken: sourceToken,
        quantity: deposit.quantity,
        targetQuantity: deposit.quantity,
        maxQuantity: deposit.balance || "0",
        isNativeToken: false,
        depositFee: deposit.depositFee,
        depositFeeRevalidating: deposit.depositFeeRevalidating,
        nativeBalanceRevalidating: false,
        dstGasFee: "0",
        nativeSymbol: "PAX",
        nativeBalance: "0",
        account,
        currentChain: {
          id: validationChain,
          namespace: "EVM",
          info: chains[0],
        },
        depositError: "",
        needSwap: false,
      });
      const actionType = useActionType({
        allowance: deposit.allowance,
        quantity: deposit.quantity,
        maxQuantity: deposit.balance || "0",
      });
      const action = useDepositAction({
        quantity: deposit.quantity,
        approve: deposit.approve,
        deposit: deposit.deposit,
      });
      const swapTokens = useSwapTokens(paxeerChain.id, true);
      const swapQuote = useSwapQuote({
        sourceToken: { ...sourceToken, swap_enable: true },
        targetToken: { ...sourceToken, address: PERPS_SPENDER },
        quantity: deposit.quantity,
        chainId: paxeerChain.id,
        slippage: 1,
      });
      return {
        ...deposit,
        validation,
        actionType,
        action,
        swapTokens,
        swapQuote,
      };
    }
    function Probe() {
      latest = ProbeState();
      return null;
    }
    let renderer: ReturnType<typeof create>;
    await act(async () => {
      renderer = create(
        createElement(
          SWRConfig,
          {
            value: {
              provider: () => cache,
              dedupingInterval: 0,
              shouldRetryOnError: false,
            },
          },
          createElement(
            PaxeerNetworkContext.Provider,
            { value: context },
            createElement(Probe),
          ),
        ),
      );
    });
    async function settle(condition: () => boolean) {
      for (let i = 0; i < 100 && !condition(); i++)
        await act(async () => {
          await new Promise((resolve) => setTimeout(resolve, 20));
        });
      assert.ok(condition(), "deposit reads settle");
    }
    async function quantity(value: string) {
      expectedAmount = value ? parseUnits(value, token.decimals) : 0n;
      await act(async () => {
        latest.setQuantity(value);
      });
      await settle(
        () => !latest.depositFeeRevalidating && !latest.balanceRevalidating,
      );
    }
    await settle(
      () => !latest.balanceRevalidating && !latest.allowanceRevalidating,
    );
    await quantity("1");
    const expectedFee = fails
      ? 0n
      : initialAllowance >= 1_000_000n
        ? 2000n
        : partial
          ? 1000n
          : 3000n;
    assert.equal(latest.depositFee, expectedFee);
    const keys = [...cache.values()]
      .map((entry) => entry._k)
      .filter(Array.isArray);
    assert.ok(
      keys.some(
        (key) =>
          key[0] === "allowance" &&
          key[1] === token.address &&
          key[2] === PERPS_SPENDER,
      ),
    );
    assert.ok(
      keys.some(
        (key) =>
          key[0] === "depositFee" &&
          key[1] === token.address &&
          key[2] === PERPS_SPENDER &&
          key[6] === "1",
      ),
    );
    assert.equal(latest.allowance, initialAllowance === 0n ? "0" : "2");
    assert.deepEqual(
      estimates.slice(0, initialAllowance === 0n ? 2 : 1),
      initialAllowance === 0n ? ["approve", "deposit"] : ["deposit"],
    );
    assert.equal(latest.validation.depositDisabled, false);
    assert.equal(latest.validation.validationMessage, undefined);
    assert.deepEqual(latest.swapTokens, []);
    assert.equal(latest.swapQuote.swapData, undefined);
    assert.equal(latest.swapQuote.swapPriceRevalidating, false);
    assert.equal(
      latest.actionType,
      initialAllowance === 0n
        ? DepositAction.ApproveAndDeposit
        : DepositAction.Deposit,
    );
    if (!fails && !partial) {
      await act(async () => {
        if (latest.actionType === DepositAction.ApproveAndDeposit)
          await latest.action.onApproveAndDeposit();
        else await latest.action.onDeposit();
      });
      assert.deepEqual(
        sent,
        initialAllowance === 0n ? ["approve", "deposit"] : ["deposit"],
      );
      assert.equal(latest.action.depositError, "");
      assert.equal(latest.action.isMutating, false);
      await settle(
        () =>
          latest.allowance === (initialAllowance === 0n ? "0" : "1") &&
          !latest.depositFeeRevalidating,
      );
    }
    await quantity("101");
    assert.equal(latest.validation.depositDisabled, true);
    assert.equal(latest.validation.inputStatus, "error");
    for (const invalid of ["", "0", "-1"]) {
      await quantity(invalid);
      assert.equal(latest.validation.depositDisabled, true);
    }
    cap = 0.5;
    await quantity("1");
    assert.equal(latest.validation.depositDisabled, true);
    cap = undefined;
    if (fails) {
      validationChain = 1;
      await act(async () => {
        renderer.update(
          createElement(
            SWRConfig,
            { value: { provider: () => cache } },
            createElement(
              PaxeerNetworkContext.Provider,
              { value: context },
              createElement(Probe),
            ),
          ),
        );
      });
      assert.equal(latest.validation.depositDisabled, true);
      assert.ok(latest.validation.validationMessage);
    }
    await act(async () => {
      renderer.unmount();
    });
  }
  await scenario(0n, false, false);
  await scenario(2_000_000n, false, false);
  await scenario(0n, false, true);
  await scenario(0n, true, false);
  assert.ok(requests.length > 0);
  assert.deepEqual(forbidden, []);
  assert.deepEqual(violations, []);
  assert.ok(requests.every((url) => new URL(url).origin === chainOrigin));
}

async function nativeDepositFlow(t: TestContext) {
  const worker = new Worker(new URL(import.meta.url), {
    execArgv: ["--import", "tsx"],
  });
  t.after(async () => {
    await worker.terminate();
  });
  await new Promise<void>((resolve, reject) => {
    worker.once("error", reject);
    worker.once("exit", (code) =>
      reject(new Error(`Deposit assertions exited before completion: ${code}`)),
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

if (isMainThread) test(nativeDepositFlow);
else
  void exerciseDepositFlow().then(() => {
    assert.ok(parentPort);
    parentPort.postMessage("assertions-passed");
  });
