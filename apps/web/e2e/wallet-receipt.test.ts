import assert from "node:assert/strict";
import { existsSync, readFileSync } from "node:fs";
import { createServer } from "node:http";
import { registerHooks } from "node:module";
import test from "node:test";
import { fileURLToPath } from "node:url";
import ts from "typescript";
import type { Eip1193Provider } from "../src/api/provider.ts";
import type { PrecompileSendOutcome } from "../src/api/wallet.ts";

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

const wallet = await import("../src/api/wallet.ts");
const { perpsDepositCall, precompileTransactionRequest } = await import("../src/api/sdk.ts");
const { createElement } = await import("react");
const { renderToStaticMarkup } = await import("react-dom/server");
const { SendOutcome } = await import("../src/app/_markets/send-outcome.tsx");
const ACCOUNT = `0x${"11".repeat(20)}`;
const HASH = `0x${"ab".repeat(32)}`;
const CALL = perpsDepositCall(`0x${"22".repeat(20)}`, 123n);
const LOGS = [
  {
    address: CALL.to,
    topics: [`0x${"cd".repeat(32)}`, `0x${"00".repeat(12)}${ACCOUNT.slice(2)}`],
    data: `0x${"00".repeat(31)}7b`,
    transactionHash: HASH,
    blockNumber: "0x20000000000001",
    logIndex: "0xa",
  },
  {
    address: CALL.to,
    topics: [],
    data: "0x",
    transactionHash: HASH,
    blockNumber: "0x20000000000001",
    logIndex: "0xb",
  },
];

async function withWalletReceipt(
  status: "0x1" | "0x0",
  pendingPolls: number,
  run: (state: { readonly requests: unknown[]; readonly sends: unknown[]; readonly sleeps: number[] }) => Promise<void>,
  options: Readonly<{ receiptFault?: "http" | "rpc" | "malformed"; sendError?: Error }> = {},
): Promise<void> {
  const requests: unknown[] = [];
  const sends: unknown[] = [];
  const sleeps: number[] = [];
  const ethereum: Eip1193Provider = {
    async request(request) {
      sends.push(request);
      assert.deepEqual(request, precompileTransactionRequest(ACCOUNT, CALL));
      if (options.sendError !== undefined) throw options.sendError;
      return HASH;
    },
  };
  const server = createServer(async (request, response) => {
    let body = "";
    for await (const chunk of request) body += String(chunk);
    const document = JSON.parse(body) as { id: string };
    requests.push(document);
    const fault = requests.length > pendingPolls ? options.receiptFault : undefined;
    response.writeHead(fault === "http" ? 503 : 200, { "content-type": "application/json" });
    if (fault === "rpc") {
      response.end(JSON.stringify({ jsonrpc: "2.0", id: document.id, error: { code: -32000, message: "Receipt unavailable" } }));
      return;
    }
    response.end(JSON.stringify({
      jsonrpc: "2.0", id: document.id,
      result: requests.length <= pendingPolls ? null : {
        transactionHash: HASH, blockNumber: "0x20000000000001", status: fault === "malformed" ? "unknown" : status, logs: LOGS,
      },
    }));
  });
  const previousWindow = Object.getOwnPropertyDescriptor(globalThis, "window");
  const previousGateway = process.env.LAYERX_GATEWAY_URL;
  try {
    await new Promise<void>((resolve, reject) => {
      server.once("error", reject);
      server.listen(0, "127.0.0.1", resolve);
    });
    const address = server.address();
    assert.ok(address !== null && typeof address !== "string");
    process.env.LAYERX_GATEWAY_URL = `http://127.0.0.1:${address.port}`;
    Object.defineProperty(globalThis, "window", {
      configurable: true,
      value: { ethereum, sessionStorage: { getItem: () => "paxeer" } },
    });
    await run({ requests, sends, sleeps });
    assert.equal(sends.length, 1);
    for (const request of requests) {
      assert.equal(typeof request, "object");
      assert.ok(request !== null);
      assert.deepEqual(request, {
        jsonrpc: "2.0", id: (request as { id: string }).id,
        method: "eth_getTransactionReceipt", params: [HASH],
      });
    }
  } finally {
    if (previousWindow === undefined) Reflect.deleteProperty(globalThis, "window");
    else Object.defineProperty(globalThis, "window", previousWindow);
    if (previousGateway === undefined) delete process.env.LAYERX_GATEWAY_URL;
    else process.env.LAYERX_GATEWAY_URL = previousGateway;
    await new Promise<void>((resolve, reject) => {
      server.close(error => error === undefined ? resolve() : reject(error));
      server.closeAllConnections();
    });
  }
}

async function wallet_send_reports_receipt_and_logs(): Promise<void> {
  assert.equal(wallet.RECEIPT_POLL_LIMIT, 20);
  assert.equal(wallet.RECEIPT_POLL_INTERVAL_MS, 500);
  for (const pendingPolls of [0, 2, wallet.RECEIPT_POLL_LIMIT - 1, wallet.RECEIPT_POLL_LIMIT]) {
    await withWalletReceipt("0x1", pendingPolls, async ({ requests, sleeps }) => {
      const outcome = await wallet.sendWalletPrecompileCall(ACCOUNT, () => CALL, async milliseconds => {
        sleeps.push(milliseconds);
        assert.equal(requests.length, sleeps.length, "sleep only between receipt requests");
      });
      const polls = Math.min(pendingPolls + 1, wallet.RECEIPT_POLL_LIMIT);
      assert.equal(requests.length, polls);
      assert.deepEqual(sleeps, Array.from({ length: polls - 1 }, () => 500));
      if (pendingPolls === wallet.RECEIPT_POLL_LIMIT) {
        assert.deepEqual(outcome, { outcome: "sent", transactionHash: HASH });
      } else {
        assert.deepEqual(outcome, {
          outcome: "sent", transactionHash: HASH,
          receipt: {
            transactionHash: HASH, blockNumber: 9_007_199_254_740_993n, status: "success",
            logs: LOGS.map(log => ({ ...log, blockNumber: 9_007_199_254_740_993n, logIndex: BigInt(log.logIndex) })),
          },
        });
      }
    });
  }
}

async function wallet_send_reports_reverted_receipt(): Promise<void> {
  for (const pendingPolls of [0, 2]) {
    await withWalletReceipt("0x0", pendingPolls, async ({ requests, sleeps }) => {
      const outcome = await wallet.sendWalletPrecompileCall(ACCOUNT, () => CALL, async milliseconds => {
        sleeps.push(milliseconds);
        assert.equal(requests.length, sleeps.length, "sleep only between receipt requests");
      });
      assert.deepEqual(outcome, {
        outcome: "reverted", transactionHash: HASH,
        receipt: {
          transactionHash: HASH, blockNumber: 9_007_199_254_740_993n, status: "reverted",
          logs: LOGS.map(log => ({ ...log, blockNumber: 9_007_199_254_740_993n, logIndex: BigInt(log.logIndex) })),
        },
      });
      assert.equal(requests.length, pendingPolls + 1);
      assert.deepEqual(sleeps, Array.from({ length: pendingPolls }, () => 500));
    });
  }
}

async function wallet_send_preserves_hash_when_confirmation_fails(): Promise<void> {
  for (const receiptFault of ["http", "rpc", "malformed"] as const) {
    for (const pendingPolls of [0, 2]) {
      await withWalletReceipt("0x1", pendingPolls, async ({ requests, sleeps }) => {
        const outcome = await wallet.sendWalletPrecompileCall(ACCOUNT, () => CALL, async milliseconds => {
          sleeps.push(milliseconds);
        });
        assert.deepEqual(outcome, { outcome: "unconfirmed", transactionHash: HASH });
        assert.equal(requests.length, pendingPolls + 1);
        assert.deepEqual(sleeps, Array.from({ length: pendingPolls }, () => 500));
      }, { receiptFault });
    }
  }
  await withWalletReceipt("0x1", 1, async ({ requests }) => {
    assert.deepEqual(await wallet.sendWalletPrecompileCall(ACCOUNT, () => CALL, async () => {
      throw new Error("Polling interrupted");
    }), { outcome: "unconfirmed", transactionHash: HASH });
    assert.equal(requests.length, 1);
  });
}

async function wallet_send_keeps_pre_submission_failures_separate(): Promise<void> {
  for (const [sendError, expected] of [
    [new Error("Submission failed"), { outcome: "failed", detail: "Submission failed" }],
    [Object.assign(new Error("Cancelled"), { code: 4001 }), { outcome: "cancelled" }],
  ] as const) {
    await withWalletReceipt("0x1", 0, async ({ requests }) => {
      assert.deepEqual(await wallet.sendWalletPrecompileCall(ACCOUNT, () => CALL), expected);
      assert.deepEqual(requests, []);
    }, { sendError });
  }
}

function wallet_send_renders_confirmation_states_truthfully(): void {
  const render = (outcome: PrecompileSendOutcome) => renderToStaticMarkup(createElement(SendOutcome, { outcome }));
  const pending = render({ outcome: "sent", transactionHash: HASH });
  const unknown = render({ outcome: "unconfirmed", transactionHash: HASH });
  assert.match(pending, /Awaiting confirmation/u);
  assert.match(unknown, /confirmation is unavailable/u);
  for (const html of [pending, unknown]) {
    assert.ok(html.includes(HASH));
    assert.match(html, /Check its status before sending again/u);
    assert.match(html, /border-warning/u);
    assert.doesNotMatch(html, /not sent|Sent\.|border-success/u);
  }
  const receipt = { transactionHash: HASH, status: "success" as const, blockNumber: 1n, logs: [] };
  assert.match(render({ outcome: "sent", transactionHash: HASH, receipt }), /Sent\./u);
  assert.match(render({ outcome: "reverted", transactionHash: HASH, receipt: { ...receipt, status: "reverted" } }), /reverted/u);
  assert.match(render({ outcome: "failed" }), /The transaction was not sent/u);
}

test(wallet_send_reports_receipt_and_logs);
test(wallet_send_reports_reverted_receipt);
test(wallet_send_preserves_hash_when_confirmation_fails);
test(wallet_send_keeps_pre_submission_failures_separate);
test(wallet_send_renders_confirmation_states_truthfully);
