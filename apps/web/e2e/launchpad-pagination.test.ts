// Task 22.33.350: launchpad-first-page-only is proved by
// launchpad_pagination_reaches_later_pools_and_quotes_the_selected_market and
// launchpad_pagination_bounds_invalid_and_terminal_pages.
// Wiring requests: none; the existing registry interface supports bounded offsets.
// Baseline verification was not run: the task's gate order requires completed
// implementation before hf-build/hf-done and forbids a manual verify run.
import assert from "node:assert/strict";
import { existsSync, readFileSync } from "node:fs";
import { createServer } from "node:http";
import { registerHooks } from "node:module";
import test from "node:test";
import { fileURLToPath } from "node:url";
import { Window } from "happy-dom";
import type { ReactElement, ReactNode } from "react";
import ts from "typescript";
import { decodeFunctionData, encodeFunctionResult, type Abi, type Hex } from "viem";
import type { MarketSearchParams } from "../src/app/_markets/format.ts";

registerHooks({
  resolve(specifier, context, nextResolve) {
    if (specifier === "server-only") return { url: "data:text/javascript,", shortCircuit: true };
    if (specifier.startsWith(".") && context.parentURL !== undefined) {
      const candidate = new URL(specifier, context.parentURL);
      for (const suffix of [".ts", ".tsx", "/index.ts"]) {
        const source = candidate.href.endsWith(".js")
          ? new URL(candidate.href.slice(0, -3) + suffix)
          : new URL(candidate.href + suffix);
        if (existsSync(source)) return nextResolve(source.href, context);
      }
    }
    if (specifier === "next/navigation") return nextResolve("next/navigation.js", context);
    if (specifier === "next/link") return nextResolve("next/link.js", context);
    return nextResolve(specifier, context);
  },
  load(url, context, nextLoad) {
    if (/\.(ts|tsx)$/u.test(url) && !url.includes("/node_modules/")) {
      return {
        format: "module", shortCircuit: true,
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

const browser = new Window({ url: "http://localhost/launchpad" });
Object.assign(globalThis, {
  window: browser, document: browser.document, HTMLElement: browser.HTMLElement,
  HTMLInputElement: browser.HTMLInputElement, HTMLButtonElement: browser.HTMLButtonElement,
  HTMLFormElement: browser.HTMLFormElement,
  Element: browser.Element, Node: browser.Node, NodeFilter: browser.NodeFilter,
  DocumentFragment: browser.DocumentFragment, Event: browser.Event, EventTarget: browser.EventTarget,
  FocusEvent: browser.FocusEvent, MouseEvent: browser.MouseEvent, PointerEvent: browser.PointerEvent,
  KeyboardEvent: browser.KeyboardEvent, MutationObserver: browser.MutationObserver,
  ResizeObserver: browser.ResizeObserver, CustomEvent: browser.CustomEvent,
  getComputedStyle: browser.getComputedStyle.bind(browser),
  requestAnimationFrame: browser.requestAnimationFrame.bind(browser),
  cancelAnimationFrame: browser.cancelAnimationFrame.bind(browser), IS_REACT_ACT_ENVIRONMENT: true,
});
const { createElement, isValidElement, act } = await import("react");
const { createRoot } = await import("react-dom/client");
const { renderToStaticMarkup } = await import("react-dom/server");
const { AppRouterContext } = await import("next/dist/shared/lib/app-router-context.shared-runtime.js");
const { default: LaunchpadPage } = await import("../src/app/launchpad/page.tsx");
const { LaunchpadActions } = await import("../src/app/launchpad/launchpad-client.tsx");
const { ExplorerTable } = await import("../src/kit/explorer.tsx");

function abi(path: string): Abi {
  return JSON.parse(readFileSync(new URL(`../../../precompiles/launchpad/${path}`, import.meta.url), "utf8")) as Abi;
}
const registryAbi = abi("abi.json");
const poolAbi = abi("abi/SidioraPool.json");
const tokenAbi = abi("abi/SidioraERC20.json");
const nftAbi = abi("abi/SidioraNFT.json");
const registry = "0x0000000000000000000000000000000000001017";
const nft = "0x00000000000000000000000000000000000000a1";
const quoteToken = "0x00000000000000000000000000000000000000a2";
const account = "0x00000000000000000000000000000000000000a3";
const zero = `0x${"0".repeat(40)}`;

// Committed in-memory registry peer. Its ordered store serves the published
// precompile/pool/token ABIs over HTTP; production readers and codecs run intact.
class LaunchpadRegistry {
  readonly pools;
  readonly pages: { offset: bigint; limit: bigint }[] = [];
  readonly quotes: { pool: string; amount: bigint; buy: boolean }[] = [];
  readonly failures: string[] = [];

  constructor(size: number) {
    this.pools = Array.from({ length: size }, (_, index) => ({
      pool: `0x${(0x2000 + index).toString(16).padStart(40, "0")}`,
      token: `0x${(0x3000 + index).toString(16).padStart(40, "0")}`,
      name: `Market ${index + 1}`, symbol: `M${index + 1}`, nftId: BigInt(index + 1),
    }));
  }

  read(to: string, data: Hex): Hex {
    const pool = this.pools.find((market) => market.pool === to);
    const token = this.pools.find((market) => market.token === to);
    const contract = to === registry ? registryAbi : to === nft ? nftAbi
      : to === quoteToken || token !== undefined ? tokenAbi : pool !== undefined ? poolAbi : undefined;
    assert.ok(contract, `Unknown contract ${to}`);
    const decoded = decodeFunctionData({ abi: contract, data });
    const args = decoded.args ?? [];
    let result: unknown;
    if (to === registry) {
      switch (decoded.functionName) {
        case "getPoolCount": result = BigInt(this.pools.length); break;
        case "getAllPools": {
          const [offset, limit] = args;
          assert.equal(typeof offset, "bigint");
          assert.equal(limit, 50n, "every registry request remains bounded");
          assert.ok((offset as bigint) >= 0n);
          this.pages.push({ offset: offset as bigint, limit: limit as bigint });
          result = this.pools.slice(Number(offset), Number((offset as bigint) + (limit as bigint))).map((market) => market.pool);
          break;
        }
        case "nftContract": result = nft; break;
        case "usdlAddress": result = quoteToken; break;
        case "creationFee": result = 1_000_000n; break;
        case "minFeeBps": result = 30n; break;
        case "maxFeeBps": result = 300n; break;
        case "baseFeeBps": result = 100n; break;
        case "getPoolMetadata": {
          const market = this.pools.find((entry) => entry.pool === String(args[0]).toLowerCase());
          assert.ok(market);
          result = { creator: account, token: market.token, optical: zero, nftId: market.nftId, createdAt: 1n, createdBlock: 1n };
          break;
        }
        case "getPoolStats": {
          assert.ok(this.pools.some((entry) => entry.pool === String(args[0]).toLowerCase()));
          result = { virtualUsdl: 1n, realUsdl: 2n, tokenReserve: 3n, cumulativeVolume: 4n,
            currentFeeBps: 100n, poolAge: 1n, marketCap: 6n, price: 10n ** 18n };
          break;
        }
        case "getPoolByToken": {
          result = this.pools.find((entry) => entry.token === String(args[0]).toLowerCase())?.pool ?? zero;
          break;
        }
        case "quoteExactInput": {
          const market = this.pools.find((entry) => entry.pool === String(args[0]).toLowerCase());
          assert.ok(market);
          assert.equal(typeof args[1], "bigint");
          assert.equal(typeof args[2], "boolean");
          const amount = args[1] as bigint;
          this.quotes.push({ pool: market.pool, amount, buy: args[2] as boolean });
          result = { amountOut: amount * 2n, feeAmount: amount / 100n, priceImpactBps: 10n };
          break;
        }
        default: throw new Error(`Unexpected registry method ${decoded.functionName}`);
      }
    } else if (to === nft && decoded.functionName === "getFeeStrategy") {
      assert.ok(this.pools.some((entry) => entry.nftId === args[0]));
      result = 0;
    } else if (pool !== undefined && decoded.functionName === "paused") {
      result = false;
    } else if (decoded.functionName === "symbol" && (token !== undefined || to === quoteToken)) {
      result = token?.symbol ?? "USDL";
    } else if (token !== undefined && decoded.functionName === "name") {
      result = token.name;
    } else if (token !== undefined && decoded.functionName === "totalSupply") {
      result = 1_000_000_000n;
    } else {
      throw new Error(`Unexpected read ${to}.${decoded.functionName}`);
    }
    return encodeFunctionResult({ abi: contract, functionName: decoded.functionName, result } as never);
  }
}

async function serving(size: number, run: (store: LaunchpadRegistry) => Promise<void>) {
  const store = new LaunchpadRegistry(size);
  const server = createServer((request, response) => {
    void (async () => {
      let body = "";
      for await (const chunk of request) body += String(chunk);
      const call = JSON.parse(body) as { id: string; method: string; params: [{ to: string; data: Hex }, string] };
      response.setHeader("Content-Type", "application/json");
      try {
        assert.equal(request.method, "POST");
        let result: unknown;
        if (call.method === "px_getCapabilities") {
          result = { exchange: false, bridge: false, launchpad: true, probed_at: 0, rpc_height: "1" };
        } else if (call.method === "eth_blockNumber") {
          result = "0x1";
        } else {
          assert.equal(call.method, "eth_call");
          assert.ok(["latest", "0x1"].includes(call.params[1]));
          result = store.read(call.params[0].to.toLowerCase(), call.params[0].data);
        }
        response.end(JSON.stringify({ jsonrpc: "2.0", id: call.id, result }));
      } catch (error) {
        store.failures.push(String(error));
        response.end(JSON.stringify({ jsonrpc: "2.0", id: call.id, error: { code: 3, message: String(error) } }));
      }
    })().catch((error: unknown) => {
      store.failures.push(String(error));
      response.writeHead(500).end();
    });
  });
  await new Promise<void>((resolve) => server.listen(0, "127.0.0.1", resolve));
  const address = server.address();
  assert.ok(address !== null && typeof address !== "string");
  const previous = process.env.LAYERX_GATEWAY_URL;
  process.env.LAYERX_GATEWAY_URL = `http://127.0.0.1:${address.port}`;
  try {
    await run(store);
    assert.deepEqual(store.failures, [], "every call must match the published producer ABI");
  } finally {
    if (previous === undefined) delete process.env.LAYERX_GATEWAY_URL;
    else process.env.LAYERX_GATEWAY_URL = previous;
    await new Promise<void>((resolve) => { server.close(() => resolve()); server.closeAllConnections(); });
  }
}

type ElementProps = { children?: ReactNode; href?: string; "aria-label"?: string };
function elements(node: ReactNode): ReactElement<ElementProps>[] {
  if (Array.isArray(node)) return node.flatMap(elements);
  if (!isValidElement<ElementProps>(node)) return [];
  return [node, ...elements(node.props.children)];
}

async function load(parameters: MarketSearchParams) {
  const page = await LaunchpadPage({ searchParams: Promise.resolve(parameters) });
  const nodes = elements(page);
  const actions = nodes.find((node) => node.type === LaunchpadActions);
  assert.ok(isValidElement<Parameters<typeof LaunchpadActions>[0]>(actions));
  const table = nodes.find((node) => node.type === ExplorerTable);
  function link(label: string): string | undefined {
    return nodes.find((node) => node.props.children === label)?.props.href;
  }
  return { actions: actions.props, nodes, table, link };
}

function follow(href: string): MarketSearchParams {
  const params: Record<string, string | string[]> = {};
  for (const [name, value] of new URL(href, "http://localhost").searchParams) {
    const previous = params[name];
    params[name] = previous === undefined ? value : [...(typeof previous === "string" ? [previous] : previous), value];
  }
  return params;
}

async function mount(initial: Parameters<typeof LaunchpadActions>[0]) {
  const container = browser.document.createElement("div");
  browser.document.body.appendChild(container);
  const root = createRoot(container as unknown as HTMLElement);
  const router = {
    bfcacheId: "launchpad-fixture", back() { browser.history.back(); }, forward() { browser.history.forward(); },
    refresh() { throw new Error("A quote must not refresh or submit a trade"); },
    push(href: string) { browser.history.pushState(null, "", href); },
    replace(href: string) { browser.history.replaceState(null, "", href); }, prefetch() {},
  };
  async function render(props: Parameters<typeof LaunchpadActions>[0]) {
    await act(async () => {
      root.render(createElement(AppRouterContext.Provider, { value: router }, createElement(LaunchpadActions, props)));
    });
  }
  await render(initial);
  return { container, render, async stop() { await act(async () => root.unmount()); container.remove(); } };
}

async function launchpad_pagination_reaches_later_pools_and_quotes_the_selected_market() {
  await serving(103, async (store) => {
    const query = { account, q: "Moon + stars & pools", side: "sell", cursor: "activity+cursor/=", kind: "swap", tag: ["one", "two"] };
    const first = await load(query);
    assert.equal(first.actions.account, account);
    assert.deepEqual(first.actions.markets.map((market) => market.pool), store.pools.slice(0, 50).map((market) => market.pool));
    assert.equal(first.link("Previous markets"), undefined);
    const next = first.link("Next markets");
    assert.ok(next, "markets beyond fifty must be reachable from the real page");
    assert.deepEqual(follow(next), { ...query, marketPage: "2" });
    const second = await load(follow(next));
    assert.deepEqual(second.actions.markets.map((market) => market.pool), store.pools.slice(50, 100).map((market) => market.pool));
    assert.equal(second.actions.account, account);
    assert.ok(second.table);
    const html = renderToStaticMarkup(second.table);
    assert.match(html, /Market 74 \(M74\)/u);
    assert.doesNotMatch(html, /Market 1 \(M1\)/u);
    const previous = second.link("Previous markets");
    assert.ok(previous);
    assert.deepEqual(follow(previous), { ...query, marketPage: "1" });
    const lastHref = second.link("Next markets");
    assert.ok(lastHref);
    const last = await load(follow(lastHref));
    assert.deepEqual(last.actions.markets.map((market) => market.pool), store.pools.slice(100).map((market) => market.pool));
    assert.equal(last.link("Next markets"), undefined);
    const lastPrevious = last.link("Previous markets");
    assert.ok(lastPrevious);
    assert.deepEqual(follow(lastPrevious), { ...query, marketPage: "2" });
    assert.deepEqual(store.pages, [0n, 50n, 100n].map((offset) => ({ offset, limit: 50n })));

    const mounted = await mount(first.actions);
    try {
      // Next preserves client instances between server navigations. Replace the
      // actual page props while keeping the production hook state alive.
      await mounted.render(second.actions);
      const selected = mounted.container.querySelector('[aria-label="Market"] [aria-checked="true"]');
      assert.equal(selected?.getAttribute("value"), store.pools[50]!.token);
      const target = store.pools[73]!;
      const choice = mounted.container.querySelector(`[aria-label="Market"] [value="${target.token}"]`);
      assert.ok(choice instanceof browser.HTMLButtonElement);
      await act(async () => choice.click());
      assert.equal(choice.getAttribute("aria-checked"), "true");
      const label = [...mounted.container.querySelectorAll("label")].find((node) => node.textContent === "You pay (USDL)");
      assert.ok(label);
      const input = browser.document.getElementById(label.htmlFor);
      assert.ok(input instanceof browser.HTMLInputElement);
      await act(async () => {
        const setter = Object.getOwnPropertyDescriptor(browser.HTMLInputElement.prototype, "value")?.set;
        assert.ok(setter);
        setter.call(input, "2");
        input.dispatchEvent(new browser.Event("input", { bubbles: true }));
      });
      const preview = [...mounted.container.querySelectorAll("button")].find((node) => node.textContent === "Preview quote");
      assert.ok(preview);
      assert.equal(preview.disabled, false);
      await act(async () => preview.click());
      for (let attempt = 0; attempt < 100 && !mounted.container.textContent.includes("Minimum received"); attempt++) {
        await act(async () => { await new Promise((resolve) => setTimeout(resolve, 10)); });
      }
      assert.deepEqual(store.quotes, [{ pool: target.pool, amount: 2_000_000n, buy: true }]);
      assert.match(mounted.container.textContent, /You receive \(token\)4/u);
      assert.match(mounted.container.textContent, /Minimum received3\.96/u);
      await mounted.render(last.actions);
      assert.equal(mounted.container.querySelector('[aria-label="Market"] [aria-checked="true"]')?.getAttribute("value"), store.pools[100]!.token);
      assert.doesNotMatch(mounted.container.textContent, /Minimum received/u, "a prior page's quote cannot be traded");
    } finally { await mounted.stop(); }
  });
}

async function launchpad_pagination_bounds_invalid_and_terminal_pages() {
  await serving(51, async (store) => {
    for (const marketPage of ["0", "-1", "1.5", "junk", "9".repeat(79), ["2", "3"]]) {
      const page = await load({ account, marketPage });
      assert.equal(page.actions.markets.length, 50);
      assert.equal(page.actions.markets[0]?.pool, store.pools[0]!.pool);
      assert.equal(page.link("Previous markets"), undefined);
    }
    for (const marketPage of ["2", "999999999999999999999999999999999999999999999999999999999999999999999999999999"]) {
      const page = await load({ account, marketPage });
      assert.deepEqual(page.actions.markets.map((market) => market.pool), [store.pools[50]!.pool]);
      assert.equal(page.link("Next markets"), undefined);
      const nav = page.nodes.find((node) => node.props["aria-label"] === "Market pages");
      assert.ok(nav);
      assert.match(renderToStaticMarkup(nav), /Page 2 of 2/u);
    }
    assert.deepEqual(store.pages.map((page) => page.offset), [0n, 0n, 0n, 0n, 0n, 0n, 50n, 50n]);
  });
  for (const size of [0, 50]) {
    await serving(size, async () => {
      const page = await load({ marketPage: "2" });
      assert.equal(page.actions.markets.length, size);
      assert.equal(page.link("Next markets"), undefined);
      assert.equal(page.link("Previous markets"), undefined);
      assert.equal(page.nodes.some((node) => node.props["aria-label"] === "Market pages"), size > 0);
    });
  }
}

test(launchpad_pagination_reaches_later_pools_and_quotes_the_selected_market);
test(launchpad_pagination_bounds_invalid_and_terminal_pages);
