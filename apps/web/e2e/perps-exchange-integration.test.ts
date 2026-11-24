import assert from "node:assert/strict";
import { existsSync, readFileSync } from "node:fs";
import { createServer } from "node:http";
import { createRequire, registerHooks } from "node:module";
import test from "node:test";
import { fileURLToPath, pathToFileURL } from "node:url";
import ts from "typescript";
import { Window } from "happy-dom";
import {
  decodeFunctionData, encodeFunctionResult, http, keccak256, parseAbi,
  parseTransaction, recoverTransactionAddress, type Abi, type Hex,
} from "viem";
import { privateKeyToAccount } from "viem/accounts";
import type { Eip1193Provider } from "../src/api/provider.ts";
import type { PerpsCollateral } from "../src/app/exchange/exchange-client.tsx";

function happy_dom_resolves_from_web_manifest() {
  const manifestURL = new URL("../package.json", import.meta.url);
  const manifest = JSON.parse(readFileSync(manifestURL, "utf8"));
  assert.equal(manifest.devDependencies["happy-dom"], "21.0.4");
  const resolveFromWeb = createRequire(manifestURL).resolve;
  const dependencyManifestURL = pathToFileURL(resolveFromWeb("happy-dom/package.json"));
  const dependencyManifest = JSON.parse(readFileSync(dependencyManifestURL, "utf8"));
  assert.equal(dependencyManifest.name, "happy-dom");
  assert.equal(dependencyManifest.version, manifest.devDependencies["happy-dom"]);
  assert.equal(resolveFromWeb("happy-dom"), fileURLToPath(new URL(dependencyManifest.main, dependencyManifestURL)));
  assert.equal(fileURLToPath(import.meta.resolve("happy-dom")), resolveFromWeb("happy-dom"));
}
test(happy_dom_resolves_from_web_manifest);

registerHooks({
  resolve(specifier, context, nextResolve) {
    if (specifier.startsWith(".") && context.parentURL !== undefined) {
      const candidate = new URL(specifier, context.parentURL);
      for (const suffix of [".ts", ".tsx", "/index.ts"]) {
        const source = candidate.href.endsWith(".js")
          ? new URL(candidate.href.slice(0, -3) + suffix) : new URL(candidate.href + suffix);
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
      return { format: "module", shortCircuit: true,
        source: ts.transpileModule(readFileSync(new URL(url), "utf8"), {
          fileName: fileURLToPath(url),
          compilerOptions: { target: ts.ScriptTarget.ES2022, module: ts.ModuleKind.ESNext, jsx: ts.JsxEmit.ReactJSX },
        }).outputText };
    }
    const loaded = nextLoad(url, context);
    return loaded.format === "commonjs" ? { ...loaded, source: undefined } : loaded;
  },
});

const browser = new Window({ url: "http://localhost/exchange" });
Object.assign(globalThis, {
  window: browser, document: browser.document, HTMLElement: browser.HTMLElement,
  HTMLInputElement: browser.HTMLInputElement, HTMLFormElement: browser.HTMLFormElement,
  HTMLButtonElement: browser.HTMLButtonElement, Element: browser.Element, Node: browser.Node,
  NodeFilter: browser.NodeFilter, DocumentFragment: browser.DocumentFragment,
  Event: browser.Event, EventTarget: browser.EventTarget, FocusEvent: browser.FocusEvent,
  MouseEvent: browser.MouseEvent, PointerEvent: browser.PointerEvent, KeyboardEvent: browser.KeyboardEvent,
  MutationObserver: browser.MutationObserver, ResizeObserver: browser.ResizeObserver,
  CustomEvent: browser.CustomEvent, getComputedStyle: browser.getComputedStyle.bind(browser),
  requestAnimationFrame: browser.requestAnimationFrame.bind(browser),
  cancelAnimationFrame: browser.cancelAnimationFrame.bind(browser),
  IS_REACT_ACT_ENVIRONMENT: true,
});
const { createElement, act } = await import("react");
const { createRoot } = await import("react-dom/client");
const { AppRouterContext } = await import("next/dist/shared/lib/app-router-context.shared-runtime.js");
const client = await import("../src/app/exchange/exchange-client.tsx");
const wallet = await import("../src/api/wallet.ts");
const sdk = await import("../src/api/sdk.ts");
const PERPS = "0x0000000000000000000000000000000000001015";
const collateral: PerpsCollateral = { token: "0x0000000000000000000000000000000000000801", decimals: 6, symbol: "USDL" };
const abi = JSON.parse(readFileSync(new URL("../../../precompiles/perps/abi/abi.json", import.meta.url), "utf8")) as Abi;
const tokenAbi = parseAbi([
  "function allowance(address owner, address spender) view returns (uint256)",
  "function approve(address spender, uint256 amount) returns (bool)",
]);
const signer = privateKeyToAccount(`0x${"17".repeat(32)}`);
const secondAccount = privateKeyToAccount(`0x${"18".repeat(32)}`).address;

type Transaction = ReturnType<typeof parseTransaction>;
type Receipt = { transactionHash: Hex; blockNumber: Hex; status: "0x1"; logs: [] };

/** Committed in-memory peer: actual signed bytes must debit funded state before a receipt exists. */
class ExchangeLedger {
  readonly transactions: Transaction[] = [];
  readonly receipts = new Map<Hex, Receipt>();
  readonly positions = new Set<string>();
  readonly orders = new Set<string>(["9"]);
  readonly preflights: string[] = [];
  nativeBalance = 10n ** 20n;
  collateralBalance = 100_000_000n;
  allowance = 0n;
  receiptTransportAvailable = true;
  interruptNext: "approval" | "write" | undefined;
  nextPosition = 4n;

  async rpc(method: string, params: readonly unknown[]): Promise<unknown> {
    if (method === "eth_chainId") return "0x7d";
    if (method === "eth_getTransactionReceipt") {
      if (!this.receiptTransportAvailable) throw new Error("Receipt transport interrupted");
      return this.receipts.get(params[0] as Hex) ?? null;
    }
    if (method === "eth_call") {
      const call = params[0] as { to: string; data: Hex; from?: string };
      if (call.to.toLowerCase() === collateral.token) {
        const decoded = decodeFunctionData({ abi: tokenAbi, data: call.data });
        assert.equal(decoded.functionName, "allowance");
        assert.deepEqual(decoded.args?.map((arg) => String(arg).toLowerCase()), [signer.address.toLowerCase(), PERPS]);
        return encodeFunctionResult({ abi: tokenAbi, functionName: "allowance", result: this.allowance });
      }
      assert.equal(call.to.toLowerCase(), PERPS);
      assert.equal(call.from?.toLowerCase(), signer.address.toLowerCase());
      const decoded = decodeFunctionData({ abi, data: call.data });
      this.preflights.push(decoded.functionName);
      return "0x";
    }
    assert.equal(method, "eth_sendRawTransaction");
    const serialized = params[0] as Hex;
    assert.ok(serialized.startsWith("0x02"), "the wallet must produce a type-2 envelope");
    const from = await recoverTransactionAddress({ serializedTransaction: serialized as `0x02${string}` });
    assert.equal(from, signer.address);
    const tx = parseTransaction(serialized);
    assert.equal(tx.chainId, 125);
    assert.equal(tx.type, "eip1559");
    assert.equal(tx.nonce, this.transactions.length);
    const fee = (tx.gas ?? 0n) * (tx.maxFeePerGas ?? 0n);
    assert.ok(fee > 0n && fee <= this.nativeBalance);
    this.nativeBalance -= fee;
    const approval = tx.to?.toLowerCase() === collateral.token;
    const decoded = decodeFunctionData({ abi: approval ? tokenAbi : abi, data: tx.data! });
    const args = decoded.args ?? [];
    if (approval) {
      assert.equal(decoded.functionName, "approve");
      assert.equal(String(args[0]).toLowerCase(), PERPS);
      this.allowance = args[1] as bigint;
    } else {
      assert.equal(tx.to?.toLowerCase(), PERPS);
      assert.equal(this.preflights.at(-1), decoded.functionName);
      if (decoded.functionName === "openPosition") {
        const amount = args[2] as bigint;
        assert.ok(this.allowance >= amount && this.collateralBalance >= amount);
        this.allowance -= amount;
        this.collateralBalance -= amount;
        this.positions.add((this.nextPosition++).toString());
      } else if (decoded.functionName === "closePosition") {
        assert.ok(this.positions.delete(String(args[0])), "only a current position may close");
      } else if (decoded.functionName === "cancelOrder") {
        assert.ok(this.orders.delete(String(args[0])), "only a current order may cancel");
      }
    }
    this.transactions.push(tx);
    const hash = keccak256(serialized);
    this.receipts.set(hash, { transactionHash: hash, blockNumber: `0x${this.transactions.length.toString(16)}`, status: "0x1", logs: [] });
    if (this.interruptNext === (approval ? "approval" : "write")) {
      this.interruptNext = undefined;
      this.receiptTransportAvailable = false;
    }
    return hash;
  }
}

/** Signing wallet uses the same HTTP peer as the server actions; no callback supplies wallet outcomes. */
class SigningWallet implements Eip1193Provider {
  readonly transport: ReturnType<ReturnType<typeof http>>;
  readonly ledger: ExchangeLedger;
  constructor(transport: ReturnType<ReturnType<typeof http>>, ledger: ExchangeLedger) {
    this.transport = transport;
    this.ledger = ledger;
  }
  async request(request: Readonly<{ method: string; params?: readonly unknown[] }>): Promise<unknown> {
    if (request.method !== "eth_sendTransaction")
      return this.transport.request(request as Parameters<typeof this.transport.request>[0]);
    const tx = request.params?.[0] as { from: string; to: Hex; data: Hex; value: Hex; chainId: string; type: string };
    assert.equal(tx.from.toLowerCase(), signer.address.toLowerCase());
    assert.equal(tx.chainId, "0x7d");
    assert.equal(tx.type, "0x2");
    const serialized = await signer.signTransaction({ chainId: 125, type: "eip1559", to: tx.to,
      data: tx.data, value: BigInt(tx.value), nonce: this.ledger.transactions.length,
      gas: 1_000_000n, maxFeePerGas: 1n, maxPriorityFeePerGas: 1n });
    return this.transport.request({ method: "eth_sendRawTransaction", params: [serialized] });
  }
}

async function fixture(run: (ledger: ExchangeLedger) => Promise<void>) {
  const ledger = new ExchangeLedger();
  const server = createServer((request, response) => {
    void (async () => {
      let body = "";
      for await (const chunk of request) body += String(chunk);
      const rpc = JSON.parse(body) as { id: string; method: string; params: readonly unknown[] };
      response.setHeader("content-type", "application/json");
      try {
        const result = await ledger.rpc(rpc.method, rpc.params);
        response.end(JSON.stringify({ jsonrpc: "2.0", id: rpc.id, result }));
      } catch (error) {
        response.end(JSON.stringify({ jsonrpc: "2.0", id: rpc.id,
          error: { code: -32000, message: error instanceof Error ? error.message : String(error) } }));
      }
    })();
  });
  await new Promise<void>((resolve) => server.listen(0, "127.0.0.1", resolve));
  const address = server.address();
  assert.ok(address !== null && typeof address === "object");
  const url = `http://127.0.0.1:${address.port}`;
  const previous = process.env.LAYERX_GATEWAY_URL;
  process.env.LAYERX_GATEWAY_URL = url;
  const provider = new SigningWallet(http(url, { retryCount: 0 })({}), ledger);
  Object.assign(browser, { ethereum: provider });
  browser.localStorage.clear();
  browser.sessionStorage.clear();
  wallet.chooseWalletFee("paxeer", "");
  try { await run(ledger); }
  finally {
    if (previous === undefined) delete process.env.LAYERX_GATEWAY_URL;
    else process.env.LAYERX_GATEWAY_URL = previous;
    await new Promise<void>((resolve) => server.close(() => resolve()));
  }
}

type Props = Parameters<typeof client.ExchangeActions>[0];
function props(ledger: ExchangeLedger): Props {
  return { account: signer.address, collateral, markets: [{ id: "3", label: "Bitcoin" }],
    positions: [...ledger.positions].map((id) => ({ id, label: `Position ${id}` })),
    orders: [...ledger.orders].map((id) => ({ id, label: `Order ${id}` })) };
}

async function mount(initial: Props) {
  const container = browser.document.createElement("div");
  browser.document.body.appendChild(container);
  const root = createRoot(container as unknown as HTMLElement);
  let refreshes = 0;
  const prefetched = new Set<string>();
  const router = { bfcacheId: "exchange-fixture", back() { browser.history.back(); }, forward() { browser.history.forward(); },
    refresh() { refreshes++; }, push(href: string) { browser.history.pushState(null, "", href); },
    replace(href: string) { browser.history.replaceState(null, "", href); }, prefetch(href: string) { prefetched.add(href); } };
  async function render(next: Props) {
    await act(async () => {
      root.render(createElement(AppRouterContext.Provider, { value: router }, createElement(client.ExchangeActions, next)));
    });
  }
  await render(initial);
  function button(label: string) {
    const found = [...container.querySelectorAll("button")].find((node) => node.textContent === label);
    assert.ok(found, `Missing mounted control: ${label}`);
    return found;
  }
  return {
    container, render, button, get refreshes() { return refreshes; },
    async click(label: string) { await act(async () => { button(label).click(); }); },
    async input(label: string, value: string) {
      const element = [...container.querySelectorAll("label")].find((node) => node.textContent === label);
      assert.ok(element, `Missing field: ${label}`);
      const input = browser.document.getElementById(element.htmlFor);
      assert.ok(input instanceof browser.HTMLInputElement);
      await act(async () => {
        const setter = Object.getOwnPropertyDescriptor(browser.HTMLInputElement.prototype, "value")?.set;
        assert.ok(setter);
        setter.call(input, value);
        input.dispatchEvent(new browser.Event("input", { bubbles: true }));
      });
    },
    async stop() { await act(async () => { root.unmount(); }); container.remove(); },
  };
}

async function until(check: () => boolean) {
  for (let i = 0; i < 150; i++) {
    if (check()) return;
    await act(async () => { await new Promise((resolve) => setTimeout(resolve, 20)); });
  }
  assert.ok(check(), "Mounted exchange did not reach its expected state");
}

async function perps_real_path_proof() {
  await fixture(async (ledger) => {
    const page = await mount(props(ledger));
    try {
      await page.input("Collateral (USDL)", "2.125001");
      await page.input("Leverage (x)", "5.5");
      assert.equal(page.button("Open position").disabled, false);
      await page.click("Open position");
      await until(() => page.refreshes === 1);
      assert.equal(ledger.transactions.length, 2, "production collateral branch must approve, then open");
      assert.deepEqual(decodeFunctionData({ abi: tokenAbi, data: ledger.transactions[0]!.data! }),
        { functionName: "approve", args: [PERPS, 2_125_001n] });
      const open = ledger.transactions[1]!;
      assert.equal(open.data, sdk.perpsOpenPositionCall({ marketId: 3n, collateralToken: collateral.token,
        collateralAmount: 2_125_001n, leverage: 5_500_000_000_000_000_000n, isLong: true }).data);
      assert.deepEqual([...ledger.positions], ["4"]);
      assert.equal(ledger.collateralBalance, 97_874_999n);
      for (const receipt of ledger.receipts.values()) assert.ok(page.container.textContent.includes(receipt.transactionHash));
      assert.equal(wallet.retainedWalletNativeSubmission(signer.address), undefined);
    } finally { await page.stop(); }
    const reopened = await mount(props(ledger));
    try {
      assert.equal(reopened.button("Close position").disabled, false);
      assert.equal(ledger.transactions.length, 2);
      await reopened.click("Close position");
      await until(() => reopened.refreshes === 1);
      assert.equal(ledger.positions.size, 0);
    } finally { await reopened.stop(); }
  });
}
test(perps_real_path_proof);

async function approval_stage_outcome() {
  await fixture(async (ledger) => {
    ledger.interruptNext = "approval";
    const page = await mount(props(ledger));
    try {
      await page.input("Collateral (USDL)", "2");
      await page.input("Leverage (x)", "5");
      await page.click("Open position");
      await until(() => page.container.textContent.includes("Pending approval for open"));
      assert.equal(ledger.transactions.length, 1);
      assert.equal(page.refreshes, 0, "an approval is not an opened position");
      const hash = [...ledger.receipts.keys()][0]!;
      const approval = page.container.querySelector('[aria-label="Collateral approval"]');
      assert.ok(approval !== null, "the approval has its own outcome panel");
      assert.ok(approval.textContent.includes(hash));
      assert.match(approval.textContent, /Submitted.*confirmation/u);
      assert.equal(page.button("Open position").disabled, true);
      assert.equal(wallet.retainedWalletNativeSubmission(signer.address)?.stage, "approval");
    } finally { await page.stop(); }
    const reopened = await mount(props(ledger));
    try {
      await until(() => reopened.container.textContent.includes("Pending approval for open"));
      assert.equal(reopened.button("Open position").disabled, true);
      assert.equal(ledger.transactions.length, 1);
      await reopened.input("Collateral (USDL)", "2");
      await reopened.input("Leverage (x)", "5");
      ledger.receiptTransportAvailable = true;
      await until(() => reopened.button("Open position").disabled === false);
      assert.equal(ledger.transactions.length, 1, "resuming approval must not send the intended write");
      await reopened.input("Collateral (USDL)", "2");
      await reopened.input("Leverage (x)", "5");
      await reopened.click("Open position");
      await until(() => ledger.transactions.length === 2 && !reopened.button("Open position").disabled);
      assert.equal(ledger.positions.size, 1);
    } finally { await reopened.stop(); }
  });
}
test(approval_stage_outcome);

async function perps_size_units() {
  await fixture(async (ledger) => {
    ledger.positions.add("4");
    const page = await mount(props(ledger));
    try {
      await page.input("Order size (USD)", "50.75");
      await page.input("Close size (USD)", "12.25");
      assert.match(page.container.textContent, /Size is USD notional/u);
      assert.match(page.container.textContent, /Review: order notional 50.75 USD/u);
      assert.match(page.container.textContent, /Review: close notional 12.25 USD/u);
      await page.click("Partially close position");
      await until(() => page.refreshes === 1);
      assert.deepEqual(decodeFunctionData({ abi, data: ledger.transactions[0]!.data! }),
        { functionName: "partialClose", args: [4n, 12_250_000_000_000_000_000n] });
      assert.ok(page.container.textContent.includes("12.25 USD"));
    } finally { await page.stop(); }
  });
}
test(perps_size_units);

async function exchange_selection_refresh() {
  await fixture(async (ledger) => {
    const page = await mount(props(ledger));
    try {
      assert.equal(page.button("Close position").disabled, true);
      await page.input("Collateral (USDL)", "2");
      await page.input("Leverage (x)", "5");
      await page.click("Open position");
      await until(() => page.refreshes === 1);
      await page.render(props(ledger));
      assert.equal(page.button("Close position").disabled, false);
      await page.click("Close position");
      await until(() => page.refreshes === 2);
      await page.render(props(ledger));
      assert.equal(page.button("Close position").disabled, true);
      await page.click("Cancel order");
      await until(() => page.refreshes === 3);
      await page.render(props(ledger));
      assert.equal(page.button("Cancel order").disabled, true);
      const before = ledger.transactions.length;
      await page.click("Cancel order");
      await page.click("Close position");
      assert.equal(ledger.transactions.length, before);
      await page.render({ ...props(ledger), markets: [] });
      assert.equal(page.button("Open position").disabled, true);
      assert.equal(page.button("Place limit order").disabled, true);
      await page.click("Open position");
      assert.equal(ledger.transactions.length, before);
      await page.render({ ...props(ledger), account: secondAccount, positions: [{ id: "88", label: "Position 88" }] });
      assert.doesNotMatch(page.container.textContent, /Sent\. Transaction/u);
      assert.equal(page.button("Cancel order").disabled, true);
      assert.ok(page.container.textContent.includes("Position 88"));
      assert.doesNotMatch(page.container.textContent, /Position 4/u);
      assert.equal(ledger.transactions.length, before);
    } finally { await page.stop(); }
  });
}
test(exchange_selection_refresh);

async function exchange_native_recovery() {
  await fixture(async (ledger) => {
    ledger.interruptNext = "write";
    ledger.positions.add("4");
    const page = await mount(props(ledger));
    let hash: string;
    try {
      await page.click("Close position");
      await until(() => page.container.textContent.includes("Pending write for close"));
      const retained = wallet.retainedWalletNativeSubmission(signer.address);
      assert.ok(retained?.transactionHash);
      hash = retained.transactionHash;
      assert.equal(retained.account, signer.address.toLowerCase());
      assert.equal(retained.chainId, "125");
      assert.equal(page.button("Close position").disabled, true);
    } finally { await page.stop(); }
    const reopened = await mount(props(ledger));
    try {
      await until(() => reopened.container.textContent.includes(hash));
      await reopened.input("Amount (USDL)", "1");
      assert.equal(reopened.button("Deposit").disabled, true);
      assert.equal(ledger.transactions.length, 1);
      ledger.receiptTransportAvailable = true;
      await until(() => reopened.refreshes === 1);
      assert.equal(wallet.retainedWalletNativeSubmission(signer.address), undefined);
      assert.ok(reopened.container.textContent.includes(hash));
      assert.match(reopened.container.textContent, /Sent\. Transaction/u);
      assert.equal(ledger.transactions.length, 1, "receipt recovery cannot broadcast again");
    } finally { await reopened.stop(); }
  });
}
test(exchange_native_recovery);

test.after(async () => { await browser.happyDOM.close(); });
