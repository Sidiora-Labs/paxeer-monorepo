import assert from "node:assert/strict";
import { execFile } from "node:child_process";
import { mkdtemp, rm } from "node:fs/promises";
import { createServer } from "node:http";
import { createRequire } from "node:module";
import type { AddressInfo } from "node:net";
import { tmpdir } from "node:os";
import { dirname, join } from "node:path";
import { test, type TestContext } from "node:test";
import { fileURLToPath } from "node:url";
import { promisify } from "node:util";
import { build } from "esbuild";
import type { Hex } from "viem";

async function iframeAccountEventReachesInjectedConnector(t: TestContext) {
  const directory = await mkdtemp(join(tmpdir(), "repair-22-31-5-"));
  t.after(() => rm(directory, { recursive: true, force: true }));
  const require = createRequire(import.meta.url);
  const wagmiRequire = createRequire(require.resolve("wagmi"));
  const emitterPath = wagmiRequire.resolve("@wagmi/core/internal");
  const cases: Hex[][] = [
    ["0x1111111111111111111111111111111111111111"],
    ["0x1111111111111111111111111111111111111111", "0x2222222222222222222222222222222222222222", "0x3333333333333333333333333333333333333333"],
    [],
  ];
  const bundle = await build({
    stdin: {
      resolveDir: dirname(fileURLToPath(import.meta.url)),
      sourcefile: "repair-22-31-5.browser.js",
      contents: `
import { injected } from "wagmi";
import { mainnet } from "wagmi/chains";
import { createEmitter } from ${JSON.stringify(emitterPath)};
import {
  installPostMessageEip1193Provider,
  postAccountsChangedEvent,
} from "../src/core/wallet/postMessageEip1193";

const origin = window.location.origin;
const diagnosticType = "repair-22-31-5";

function check(condition, message) {
  if (!condition) throw new Error(message);
}

async function runChild() {
  const provider = installPostMessageEip1193Provider();
  check(provider && window.ethereum === provider, "bridge installed as window.ethereum");
  check(installPostMessageEip1193Provider() === provider, "installation reuses the bridge");
  const calls = [];
  provider.on("accountsChanged", (...args) => calls.push(args));
  const emitter = createEmitter("repair-22-31-5");
  const report = (event, accounts) => window.parent.postMessage({
    type: diagnosticType, event, accounts, arguments: calls.shift(),
  }, origin);
  emitter.on("change", ({ accounts }) => report("change", accounts));
  emitter.on("disconnect", () => report("disconnect", []));
  const connector = injected({
    shimDisconnect: true,
    target: { id: "iframe", name: "Iframe", provider: () => window.ethereum },
  })({ chains: [mainnet], emitter, storage: null });
  check(await connector.getProvider() === provider, "connector discovers installed provider");
  await connector.setup();
  window.parent.postMessage({ type: diagnosticType, event: "ready" }, origin);
}

async function runParent() {
  const frame = document.createElement("iframe");
  frame.src = "/frame";
  function receive() {
    return new Promise((resolve, reject) => {
      const timeout = setTimeout(() => {
        window.removeEventListener("message", onMessage);
        reject(new Error("parent/iframe message timed out"));
      }, 3000);
      function onMessage(event) {
        if (event.source !== frame.contentWindow || event.origin !== origin ||
            event.data?.type !== diagnosticType) return;
        clearTimeout(timeout);
        window.removeEventListener("message", onMessage);
        if (event.data.error) reject(new Error(event.data.error));
        else resolve(event.data);
      }
      window.addEventListener("message", onMessage);
    });
  }
  const ready = receive();
  document.body.append(frame);
  check((await ready).event === "ready", "iframe initialized");
  for (const accounts of ${JSON.stringify(cases)}) {
    const delivered = receive();
    postAccountsChangedEvent(frame.contentWindow, origin, accounts);
    const result = await delivered;
    check(JSON.stringify(result.arguments) === JSON.stringify([accounts]),
      "accountsChanged must deliver exactly one Hex[] argument: " + JSON.stringify(result));
    check(JSON.stringify(result.accounts) === JSON.stringify(accounts),
      "injected connector must preserve all account addresses: " + JSON.stringify(result));
    check(result.event === (accounts.length ? "change" : "disconnect"),
      "empty accounts must disconnect the injected connector");
  }
  frame.remove();
  document.querySelector("pre").textContent = "PASS 3";
}

if (window.parent === window) {
  runParent().catch(error => {
    document.querySelector("pre").textContent = "FAIL " + error.stack;
  });
} else {
  runChild().catch(error => window.parent.postMessage({
    type: diagnosticType, error: error.stack,
  }, origin));
}
`,
    },
    bundle: true,
    platform: "browser",
    format: "iife",
    write: false,
    define: {
      "process.env.NEXT_PUBLIC_WALLET_IFRAME_ORIGIN": "window.location.origin",
      "process.env.NODE_ENV": '"test"',
    },
  });
  const server = createServer((request, response) => {
    if (request.url === "/browser.js") {
      response.setHeader("Content-Type", "text/javascript");
      response.end(bundle.outputFiles[0].contents);
    } else if (request.url === "/" || request.url === "/frame") {
      response.setHeader("Content-Type", "text/html");
      response.end('<!doctype html><html><body><pre id="repair-result">RUNNING</pre><script src="/browser.js"></script></body></html>');
    } else {
      response.writeHead(404).end();
    }
  });
  await new Promise<void>((resolve) => server.listen(0, "127.0.0.1", resolve));
  t.after(() => new Promise<void>((resolve, reject) => server.close((error) => (error ? reject(error) : resolve()))));
  const { port } = server.address() as AddressInfo;
  const { stdout, stderr } = await promisify(execFile)(
    process.env.CHROMIUM_BIN ?? "chromium",
    [
      "--headless",
      "--no-sandbox",
      "--disable-gpu",
      "--disable-dev-shm-usage",
      "--no-first-run",
      `--user-data-dir=${join(directory, "profile")}`,
      "--virtual-time-budget=15000",
      "--dump-dom",
      `http://127.0.0.1:${port}/`,
    ],
    { timeout: 25_000, maxBuffer: 4 * 1024 * 1024 },
  );
  assert.match(stdout, /<pre id="repair-result">PASS 3<\/pre>/, `${stdout}\n${stderr}`);
  t.diagnostic("Real parent-window messages preserved one, several and zero accounts through the injected connector.");
}

test(iframeAccountEventReachesInjectedConnector);
