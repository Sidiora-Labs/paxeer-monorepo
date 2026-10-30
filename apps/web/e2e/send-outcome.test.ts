import assert from "node:assert/strict";
import { existsSync, readFileSync } from "node:fs";
import { registerHooks } from "node:module";
import test from "node:test";
import { fileURLToPath } from "node:url";
import ts from "typescript";

import { copyEntries } from "../copy/catalog.ts";
import { copyEntry } from "../copy/runtime.ts";
import type { GatewayReceipt } from "../src/api/gateway.ts";
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

const { createElement } = await import("react");
const { renderToStaticMarkup } = await import("react-dom/server");
const { SendOutcome } = await import("../src/app/_markets/send-outcome.tsx");

function send_outcome_renders_receipt_block() {
  const transactionHash = `0x${"12".repeat(32)}`;
  const blockNumber = 9007199254740993n;
  for (const logCount of [0, 2]) {
    const receipt: GatewayReceipt = {
      transactionHash,
      blockNumber,
      status: "success",
      logs: Array.from({ length: logCount }, (_, index) => ({
        address: `0x${"34".repeat(20)}`,
        topics: [`0x${"56".repeat(32)}`],
        data: "0x",
        blockNumber,
        transactionHash,
        logIndex: BigInt(index),
      })),
    };
    const outcome: PrecompileSendOutcome = { outcome: "sent", transactionHash, receipt };
    const html = renderToStaticMarkup(createElement(SendOutcome, { outcome }));
    const message = copyEntry("markets.send.receipt").message;
    assert.equal(message, copyEntries.find((entry) => entry.key === "markets.send.receipt")?.message);
    assert.ok(html.includes(message.replace("{blockNumber}", blockNumber.toString()).replace("{logCount}", logCount.toString())));
    assert.ok(html.includes(`<p>Receipt: block 9007199254740993, logs ${logCount}.</p>`));
    assert.ok(html.includes(`Sent. Transaction ${transactionHash}`));
    assert.match(html, /\bbg-success-soft\b/u);
  }
  const pending = renderToStaticMarkup(createElement(SendOutcome, { outcome: { outcome: "sent", transactionHash } }));
  assert.ok(pending.includes("Submitted. Awaiting confirmation."));
  assert.doesNotMatch(pending, /Receipt:|Sent\.|\bbg-success-soft\b/u);
  assert.equal(renderToStaticMarkup(createElement(SendOutcome, { outcome: undefined })), "");
}

function send_outcome_renders_reverted() {
  const transactionHash = `0x${"78".repeat(32)}`;
  const receipt: GatewayReceipt = { transactionHash, blockNumber: 314159n, status: "reverted", logs: [] };
  const outcome: PrecompileSendOutcome = { outcome: "reverted", transactionHash, receipt };
  const html = renderToStaticMarkup(createElement(SendOutcome, { outcome }));
  const message = copyEntry("markets.send.reverted").message;
  assert.equal(message, copyEntries.find((entry) => entry.key === "markets.send.reverted")?.message);
  assert.ok(html.includes(message.replace("{transactionHash}", transactionHash)));
  assert.ok(html.includes(`Transaction ${transactionHash} reverted.`));
  assert.ok(html.includes("Receipt: block 314159, logs 0."));
  assert.match(html, /\bbg-destructive-soft\b/u);
  assert.doesNotMatch(html, /Sent\.|success|Awaiting confirmation/u);
}

test(send_outcome_renders_receipt_block);
test(send_outcome_renders_reverted);
