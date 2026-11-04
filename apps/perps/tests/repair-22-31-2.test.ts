import assert from "node:assert/strict";
import { spawn, spawnSync } from "node:child_process";
import { once } from "node:events";
import { readFileSync } from "node:fs";
import { createServer } from "node:http";
import type { AddressInfo } from "node:net";
import { test } from "node:test";
import React from "react";
import { renderToStaticMarkup } from "react-dom/server";
import {
  createPublicClient, createWalletClient, decodeFunctionData, erc20Abi,
  http, parseTransaction, recoverTransactionAddress, type Address, type Hex, type TransactionSerialized,
} from "viem";
import { mnemonicToAccount } from "viem/accounts";
import { createConnector, type CreateConnectorFn } from "wagmi";
import { connect, disconnect, getAccount, switchAccount } from "wagmi/actions";

function deferred<T>() {
  let resolve!: (value: T) => void;
  const promise = new Promise<T>((done) => { resolve = done; });
  return { promise, resolve };
}

type RpcRequest = { id: number; method: string; params?: unknown[] };

async function runCollateralAccountBinding() {
  const node = spawn(process.env.ANVIL_BIN ?? "anvil", [
    "--host", "127.0.0.1", "--port", "0", "--chain-id", "125",
  ], { stdio: ["ignore", "pipe", "pipe"] });
  const ready = deferred<string>();
  let output = "";
  node.stdout.on("data", (chunk: Buffer) => {
    output += chunk.toString();
    const match = output.match(/Listening on (?:http:\/\/)?127\.0\.0\.1:(\d+)/);
    if (match) ready.resolve(`http://127.0.0.1:${match[1]}`);
  });
  const startupError = new Promise<never>((_, reject) => {
    node.once("error", reject);
    node.once("exit", (code) => reject(new Error(`Anvil exited with ${code}`)));
  });
  const startupTimer = setTimeout(() => node.kill("SIGTERM"), 15_000);
  let server: ReturnType<typeof createServer> | undefined;
  let releasePending: (() => void) | undefined;
  try {
    const upstream = await Promise.race([ready.promise, startupError]);
    clearTimeout(startupTimer);
    const submitted: { from: Address; to: Address | null | undefined; data: Hex | undefined; hash: Hex }[] = [];
    let pause: {
      matches: (request: RpcRequest) => boolean;
      entered: ReturnType<typeof deferred<void>>;
      released: ReturnType<typeof deferred<void>>;
    } | undefined;
    const transportFailures: unknown[] = [];
    server = createServer((request, response) => {
      void (async () => {
        const chunks: Buffer[] = [];
        for await (const chunk of request) chunks.push(Buffer.from(chunk));
        const body = Buffer.concat(chunks).toString();
        const rpc = JSON.parse(body) as RpcRequest;
        const forwarded = await fetch(upstream, {
          method: "POST", headers: { "content-type": "application/json" }, body,
        });
        const result = await forwarded.text();
        if (rpc.method === "eth_sendRawTransaction") {
          const raw = rpc.params![0];
          assert.ok(typeof raw === "string" && /^0x(?:0[1-4]|[c-f][0-9a-f])[0-9a-f]+$/i.test(raw));
          const serializedTransaction = raw as TransactionSerialized;
          const tx = parseTransaction(serializedTransaction);
          const from = await recoverTransactionAddress({ serializedTransaction });
          const hash = (JSON.parse(result) as { result: Hex }).result;
          assert.ok(hash, "the real node accepted the signed transaction");
          submitted.push({ from, to: tx.to, data: tx.data, hash });
        }
        const pending = pause;
        if (pending?.matches(rpc)) {
          pause = undefined;
          pending.entered.resolve();
          await pending.released.promise;
        }
        response.writeHead(forwarded.status, { "content-type": "application/json" });
        response.end(result);
      })().catch((error) => {
        transportFailures.push(error);
        response.destroy(error instanceof Error ? error : new Error(String(error)));
      });
    });
    server.listen(0, "127.0.0.1");
    await once(server, "listening");
    const origin = `http://127.0.0.1:${(server.address() as AddressInfo).port}`;
    process.env.NEXT_PUBLIC_PAXEER_RPC_URLS = origin;
    process.env.NEXT_PUBLIC_PERPS_GATEWAY_URL = origin;
    process.env.NEXT_PUBLIC_PAXSCAN_API_URL = origin;
    process.env.NEXT_PUBLIC_PAXSCAN_WS_URL = origin.replace("http:", "ws:");
    process.env.NEXT_PUBLIC_PAXSCAN_BASE_URL = origin;

    const { CollateralOperationIdentity } = await import("../src/app/modals/CollateralOperationIdentity");
    const { paxeerChain } = await import("../src/core/chain");
    const { wagmiConfig } = await import("../src/core/wagmi");
    const accountHook = await import("../src/hooks/perps/useAccount");
    const { COLLATERAL_TOKENS } = await import("../src/core/perps/config");
    const { PERPS_SPENDER } = await import("../src/core/network/approvals");
    const token = COLLATERAL_TOKENS.usdl.address;
    const mnemonic = "test test test test test test test test test test test junk";
    const alice = mnemonicToAccount(mnemonic, { addressIndex: 0 });
    const bob = mnemonicToAccount(mnemonic, { addressIndex: 1 });
    const client = createPublicClient({ chain: paxeerChain, transport: http(upstream) });
    const aliceWallet = createWalletClient({ account: alice, chain: paxeerChain, transport: http(upstream) });

    const sources = Object.fromEntries([
      "core/SidioraERC20.sol", "base/ERC20Base.sol",
    ].map((path) => [path, {
      content: readFileSync(new URL(`../../../contracts/launchpad/contracts/${path}`, import.meta.url), "utf8"),
    }]));
    const compiled = spawnSync(process.env.SOLC_BIN ?? "solcjs", ["--standard-json"], {
      input: JSON.stringify({
        language: "Solidity", sources,
        settings: { optimizer: { enabled: true, runs: 200 }, outputSelection: { "*": { "*": ["abi", "evm.bytecode.object"] } } },
      }), encoding: "utf8", timeout: 20_000, maxBuffer: 4 * 1024 * 1024,
    });
    assert.equal(compiled.error, undefined);
    assert.equal(compiled.status, 0, compiled.stderr);
    const artifact = JSON.parse(compiled.stdout.slice(compiled.stdout.indexOf("{")));
    assert.deepEqual((artifact.errors ?? []).filter((error: { severity: string }) => error.severity === "error"), []);
    const contract = artifact.contracts["core/SidioraERC20.sol"].SidioraERC20;
    const deployment = await aliceWallet.deployContract({
      abi: contract.abi, bytecode: `0x${contract.evm.bytecode.object}`,
      args: ["Liquidity USD", "USDL", 1_000_000_000n, alice.address],
    });
    const deployed = await client.waitForTransactionReceipt({ hash: deployment });
    assert.equal(deployed.status, "success");
    const code = await client.getCode({ address: deployed.contractAddress! });
    assert.ok(code && code.length > 2);
    // Use the production token bytecode at the application's fixed collateral address.
    await client.request({ method: "anvil_setCode", params: [token, code] } as never);

    function localSigner(account: typeof alice, id: string): CreateConnectorFn {
      const wallet = createWalletClient({ account, chain: paxeerChain, transport: http(origin) });
      return createConnector(({ emitter }) => {
        let authorized = false;
        return {
          id, name: id, type: "localSigner",
          async connect() {
            assert.equal(await wallet.getChainId(), paxeerChain.id);
            authorized = true;
            return { accounts: [account.address], chainId: paxeerChain.id } as never;
          },
          async disconnect() { authorized = false; },
          async getAccounts() { return authorized ? [account.address] : []; },
          async getChainId() { return wallet.getChainId(); },
          async getProvider() { return wallet.transport; },
          async getClient() { return wallet; },
          async isAuthorized() { return authorized; },
          onAccountsChanged(accounts) { emitter.emit("change", { accounts: accounts as Address[] }); },
          onChainChanged(chain) { emitter.emit("change", { chainId: Number(chain) }); },
          onDisconnect() { authorized = false; emitter.emit("disconnect"); },
        };
      });
    }

    await connect(wagmiConfig, { connector: localSigner(alice, "Alice local signer") });
    const aliceConnector = getAccount(wagmiConfig).connector!;
    await connect(wagmiConfig, { connector: localSigner(bob, "Bob local signer") });
    const bobConnector = getAccount(wagmiConfig).connector!;
    const hook = accountHook.useAccount();
    const amount = 290_000n;
    const identity = { address: alice.address, chainId: paxeerChain.id };

    async function proveSwitch(
      matches: (request: RpcRequest) => boolean,
      switchSession: () => Promise<void>,
      expectedSignatures: number,
    ) {
      await switchAccount(wagmiConfig, { connector: aliceConnector });
      const entered = deferred<void>();
      const released = deferred<void>();
      pause = { matches, entered, released };
      releasePending = () => released.resolve();
      const start = submitted.length;
      const steps: string[] = [];
      const outcome = hook.deposit(alice.address, amount, (step) => steps.push(step))
        .then((receipt) => ({ receipt, error: undefined }), (error: unknown) => ({ receipt: undefined, error }));
      const reached = await Promise.race([
        entered.promise.then(() => true), outcome.then(() => false),
      ]);
      assert.equal(reached, true, "operation reached the real delayed RPC boundary");
      await switchSession();
      released.resolve();
      releasePending = undefined;
      const result = await outcome;
      assert.ok(result.error instanceof accountHook.CollateralSessionChangedError);
      assert.deepEqual(result.error.identity, identity);
      assert.equal(result.receipt, undefined);
      assert.equal(submitted.length - start, expectedSignatures);
      assert.deepEqual(steps, expectedSignatures === 1 ? ["approving"] : []);
      for (const transaction of submitted.slice(start)) {
        assert.equal(transaction.from.toLowerCase(), alice.address.toLowerCase());
        assert.equal(transaction.to?.toLowerCase(), token.toLowerCase());
        const decoded = decodeFunctionData({ abi: erc20Abi, data: transaction.data! });
        assert.equal(decoded.functionName, "approve");
        assert.deepEqual(decoded.args, [PERPS_SPENDER, amount]);
        assert.equal((await client.getTransactionReceipt({ hash: transaction.hash })).status, "success");
      }
      assert.equal(await client.getTransactionCount({ address: bob.address }), 0);
      assert.deepEqual(transportFailures, []);
    }

    const allowanceRead = (rpc: RpcRequest) => rpc.method === "eth_call" &&
      (rpc.params?.[0] as { data?: string }).data?.startsWith("0xdd62ed3e") === true;
    await proveSwitch(allowanceRead, () => switchAccount(wagmiConfig, { connector: bobConnector }).then(() => {}), 0);
    await proveSwitch(allowanceRead, async () => {
      await switchAccount(wagmiConfig, { connector: bobConnector });
      await switchAccount(wagmiConfig, { connector: aliceConnector });
    }, 0);
    await proveSwitch(allowanceRead, async () => {
      aliceConnector.onChainChanged("0x1");
    }, 0);
    aliceConnector.onChainChanged("0x7d");
    await proveSwitch((rpc) => rpc.method === "eth_getTransactionReceipt", async () => {
      await switchAccount(wagmiConfig, { connector: bobConnector });
    }, 1);
    assert.equal(await hook.getAllowance(alice.address), amount);
    assert.equal(await hook.getAllowance(bob.address), 0n);

    const count = submitted.length;
    await assert.rejects(hook.withdrawAll(alice.address), accountHook.CollateralSessionChangedError);
    await assert.rejects(hook.withdraw(amount, alice.address), accountHook.CollateralSessionChangedError);
    await assert.rejects(hook.deposit(alice.address, amount, undefined, 1), accountHook.CollateralSessionChangedError);
    assert.equal(submitted.length, count);
    const markup = renderToStaticMarkup(React.createElement(CollateralOperationIdentity, { identity }));
    assert.ok(markup.includes(alice.address));
    assert.match(markup, /Chain 125/);
    assert.ok(!markup.includes(bob.address));
    await disconnect(wagmiConfig, { connector: aliceConnector });
    await disconnect(wagmiConfig, { connector: bobConnector });
    console.log("collateral-account-binding: real allowance, approval, signer switch and bound identity assertions passed");
  } finally {
    clearTimeout(startupTimer);
    releasePending?.();
    server?.closeAllConnections();
    if (server) await new Promise<void>((resolve) => server!.close(() => resolve()));
    const stopped = once(node, "exit");
    node.kill("SIGTERM");
    await stopped;
  }
}

function collateralAccountBinding() {
  const child = spawnSync(process.execPath, ["--import", "tsx", process.argv[1], "--collateral-binding-worker"], {
    cwd: process.cwd(), encoding: "utf8", timeout: 60_000,
  });
  assert.equal(child.error, undefined);
  assert.equal(child.status, 0, `${child.stdout}\n${child.stderr}`);
  assert.match(child.stdout, /collateral-account-binding: real allowance, approval, signer switch and bound identity assertions passed/);
}

if (process.argv.includes("--collateral-binding-worker")) {
  runCollateralAccountBinding().then(() => process.exit(0), (error) => {
    console.error(error);
    process.exit(1);
  });
} else {
  test(collateralAccountBinding);
}
