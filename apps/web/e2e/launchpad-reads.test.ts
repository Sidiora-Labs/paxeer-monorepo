import assert from "node:assert/strict";
import { createServer, type IncomingMessage } from "node:http";
import { registerHooks } from "node:module";
import { existsSync, readFileSync } from "node:fs";
import test from "node:test";
import { fileURLToPath } from "node:url";
import type { ReactElement, ReactNode } from "react";
import ts from "typescript";
import { decodeFunctionData, encodeFunctionResult, type Abi, type Hex } from "viem";

registerHooks({
  resolve(specifier, context, nextResolve) {
    if (specifier === "server-only") return { url: "data:text/javascript,", shortCircuit: true };
    if (specifier.startsWith(".") && context.parentURL !== undefined) {
      const candidate = new URL(specifier, context.parentURL);
      for (const suffix of [".ts", ".tsx", "/index.ts"]) {
        const source = candidate.href.endsWith(".js") ? new URL(candidate.href.slice(0, -3) + suffix) : new URL(candidate.href + suffix);
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

function abi(path: string): Abi {
  return JSON.parse(readFileSync(new URL(`../../../precompiles/launchpad/${path}`, import.meta.url), "utf8")) as Abi;
}

const LAUNCHPAD = "0x0000000000000000000000000000000000001017";
const POOL = "0x00000000000000000000000000000000000000a1";
const TOKEN = "0x00000000000000000000000000000000000000b2";
const NFT = "0x00000000000000000000000000000000000000c3";
const USDL = "0x00000000000000000000000000000000000000d4";
const CREATOR = "0x00000000000000000000000000000000000000e5";
const ZERO = `0x${"00".repeat(20)}`;

// Each address the web reads is served by the ABI the chain publishes for it:
// the launchpad precompile's abi.json at 0x1017 and the generation 2 role ABIs
// at the pool surface, the pool token, the fee-right collection and USDL.
const CONTRACTS: Record<string, Abi> = {
  [LAUNCHPAD]: abi("abi.json"),
  [POOL]: abi("abi/SidioraPool.json"),
  [TOKEN]: abi("abi/SidioraERC20.json"),
  [NFT]: abi("abi/SidioraNFT.json"),
  [USDL]: abi("abi/SidioraERC20.json"),
};

const ANSWERS: Record<string, (args: readonly unknown[]) => unknown> = {
  [`${LAUNCHPAD}.getPoolCount`]: () => 1n,
  [`${LAUNCHPAD}.getAllPools`]: ([offset, limit]) => {
    assert.equal(offset, 0n);
    assert.equal(limit, 50n);
    return [POOL];
  },
  [`${LAUNCHPAD}.nftContract`]: () => NFT,
  [`${LAUNCHPAD}.usdlAddress`]: () => USDL,
  [`${LAUNCHPAD}.creationFee`]: () => 5n * 10n ** 18n,
  [`${LAUNCHPAD}.minFeeBps`]: () => 30n,
  [`${LAUNCHPAD}.maxFeeBps`]: () => 300n,
  [`${LAUNCHPAD}.baseFeeBps`]: () => 100n,
  [`${LAUNCHPAD}.getPoolStats`]: ([pool]) => {
    assert.equal(String(pool).toLowerCase(), POOL);
    return {
      virtualUsdl: 1n,
      realUsdl: 2n,
      tokenReserve: 3n,
      cumulativeVolume: 4n,
      currentFeeBps: 5n,
      poolAge: 6n,
      marketCap: 7n,
      price: 8n,
    };
  },
  [`${LAUNCHPAD}.getPoolMetadata`]: ([pool]) => {
    assert.equal(String(pool).toLowerCase(), POOL);
    return { creator: CREATOR, token: TOKEN, optical: ZERO, nftId: 9n, createdAt: 10n, createdBlock: 11n };
  },
  [`${LAUNCHPAD}.getPoolByToken`]: ([token]) => (String(token).toLowerCase() === TOKEN ? POOL : ZERO),
  [`${LAUNCHPAD}.quoteExactInput`]: ([pool, amountIn, isBuy]) => {
    assert.equal(String(pool).toLowerCase(), POOL);
    return { amountOut: (amountIn as bigint) * (isBuy === true ? 2n : 3n), feeAmount: 11n, priceImpactBps: 12n };
  },
  [`${POOL}.paused`]: () => true,
  [`${TOKEN}.name`]: () => "Moon",
  [`${TOKEN}.symbol`]: () => "MOON",
  [`${TOKEN}.totalSupply`]: () => 10n ** 27n,
  [`${NFT}.getFeeStrategy`]: ([id]) => {
    assert.equal(id, 9n);
    return 2;
  },
  [`${USDL}.symbol`]: () => "USDL",
};

async function body(request: IncomingMessage): Promise<{ id: string; method: string; params: { to: string; data: Hex }[] }> {
  let text = "";
  for await (const chunk of request) text += String(chunk);
  return JSON.parse(text) as { id: string; method: string; params: { to: string; data: Hex }[] };
}

/** Serves eth_call only for selectors the published ABI of the called address declares. */
async function serving(run: (called: string[]) => Promise<void>, answers = ANSWERS) {
  const called: string[] = [];
  const server = createServer((request, response) => {
    void body(request).then((call) => {
      response.setHeader("content-type", "application/json");
      if (call.method === "px_getCapabilities") {
        called.push(call.method);
        response.end(JSON.stringify({
          jsonrpc: "2.0", id: call.id,
          result: { exchange: false, bridge: false, launchpad: true, probed_at: 0, rpc_height: "1" },
        }));
        return;
      }
      const [target] = call.params;
      const to = target?.to.toLowerCase() ?? "";
      const contract = CONTRACTS[to];
      try {
        assert.equal(call.method, "eth_call");
        assert.ok(contract !== undefined, `no published ABI for ${to}`);
        const { functionName, args } = decodeFunctionData({ abi: contract, data: target!.data });
        const key = `${to}.${functionName}`;
        called.push(key);
        const answer = answers[key];
        assert.ok(answer !== undefined, `unexpected read ${key}`);
        const data = encodeFunctionResult({ abi: contract, functionName, result: answer(args ?? []) } as never);
        response.end(JSON.stringify({ jsonrpc: "2.0", id: call.id, result: data }));
      } catch (error) {
        called.push(`rejected ${to} ${target?.data.slice(0, 10)}: ${String(error)}`);
        response.end(JSON.stringify({ jsonrpc: "2.0", id: call.id, error: { code: 3, message: "execution reverted" } }));
      }
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

test("launchpad markets, count and config read only methods the current launchpad ABI declares", async () => {
  await serving(async (called) => {
    const [count, markets, config] = await Promise.all([
      precompiles.launchpadMarketCount(),
      precompiles.launchpadMarkets(0n, 50n),
      precompiles.launchpadConfig(),
    ]);
    assert.deepEqual(called.filter((entry) => entry.startsWith("rejected")), []);
    assert.equal(count, 1n);
    assert.deepEqual(markets, [
      {
        pool: POOL,
        token: TOKEN,
        name: "Moon",
        symbol: "MOON",
        creator: CREATOR,
        feeStrategy: 2n,
        paused: true,
        totalSupply: 10n ** 27n,
        tokenReserve: 3n,
        realQuoteBalance: 2n,
        cumulativeVolume: 4n,
        price: 8n,
      },
    ]);
    assert.deepEqual(config, {
      quoteDenom: "USDL",
      creationFee: 5n * 10n ** 18n,
      minFeeBps: 30n,
      maxFeeBps: 300n,
      baseFeeBps: 100n,
    });
  });
});

test("a launchpad quote resolves the token's pool and asks quoteExactInput with the trade direction", async () => {
  await serving(async (called) => {
    assert.deepEqual(await precompiles.launchpadQuote(TOKEN, "buy", 1000n), {
      amountOut: 2000n,
      feeAmount: 11n,
      priceImpactBps: 12n,
    });
    assert.equal((await precompiles.launchpadQuote(TOKEN, "sell", 1000n)).amountOut, 3000n);
    assert.deepEqual(called.filter((entry) => entry.startsWith("rejected")), []);
    await assert.rejects(precompiles.launchpadQuote(USDL, "buy", 1n), RangeError);
  });
});

test("launchpad page renders pool prices at 18 decimals and amounts at six decimals", async (t) => {
  const { isValidElement } = await import("react");
  const { renderToStaticMarkup } = await import("react-dom/server");
  const { default: LaunchpadPage } = await import("../src/app/launchpad/page.tsx");
  const { ExplorerTable } = await import("../src/kit/explorer.tsx");
  const { LabelValue } = await import("../src/kit/money.tsx");
  const { LAUNCHPAD_DECIMALS, parseUnits } = await import("../src/app/_markets/format.ts");

  function descendants(node: ReactNode): ReactElement<{ children?: ReactNode }>[] {
    if (Array.isArray(node)) return node.flatMap(descendants);
    if (!isValidElement<{ children?: ReactNode }>(node)) return [];
    return [node, ...descendants(node.props.children)];
  }

  const virtualUsdl = 10_000_000_000n;
  const tokenReserve = 1_000_000_000_000_000n;
  const nativePrice = (virtualUsdl * 10n ** 18n) / tokenReserve;
  assert.equal(nativePrice, 10_000_000_000_000n);

  for (const [price, expected] of [
    [nativePrice, "0.00001"],
    [0n, "0"],
    [1n, "0.000000000000000001"],
    [9_007_199_254_740_993_123_456_789_012_345_678n, "9007199254740993.123456789012345678"],
  ] as const) {
    await t.test(`price ${price.toString()} renders as ${expected}`, async () => {
      await serving(async (called) => {
        const page = await LaunchpadPage({ searchParams: Promise.resolve({}) });
        const elements = descendants(page);
        const table = elements.find((element) => element.type === ExplorerTable);
        assert.ok(table, "the live launchpad page must contain its Markets table");
        const html = renderToStaticMarkup(table);
        const cells = [...html.matchAll(/<td\b[^>]*>(.*?)<\/td>/gu)].map((match) => match[1]);
        assert.deepEqual(cells.slice(2, 6), [expected, "1000000000.000001", "1000000000", "123.456789"]);
        const labels = elements.filter((element) => element.type === LabelValue).map((element) => renderToStaticMarkup(element)).join("");
        assert.match(labels, /Creation fee<\/span><span[^>]*>5\.000001 USDL<\/span>/u);
        assert.ok(called.includes("px_getCapabilities"));
        assert.ok(called.includes(`${LAUNCHPAD}.getPoolStats`));
        assert.deepEqual(called.filter((entry) => entry.startsWith("rejected")), []);
      }, {
        ...ANSWERS,
        [`${LAUNCHPAD}.creationFee`]: () => 5_000_001n,
        [`${TOKEN}.totalSupply`]: () => 1_000_000_000_000_001n,
        [`${LAUNCHPAD}.getPoolStats`]: ([pool]) => {
          assert.equal(String(pool).toLowerCase(), POOL);
          return {
            virtualUsdl, realUsdl: 0n, tokenReserve, cumulativeVolume: 123_456_789n,
            currentFeeBps: 100n, poolAge: 6n, marketCap: 10_000_000_000n, price,
          };
        },
      });
    });
  }
  assert.equal(parseUnits("5.000001", LAUNCHPAD_DECIMALS), 5_000_001n);
  assert.equal(parseUnits("0.0000001", LAUNCHPAD_DECIMALS), undefined);
});
