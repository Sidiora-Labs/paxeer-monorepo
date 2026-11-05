import assert from "node:assert/strict";
import { existsSync, readFileSync } from "node:fs";
import { createServer } from "node:http";
import { registerHooks } from "node:module";
import test from "node:test";
import { fileURLToPath } from "node:url";
import ts from "typescript";

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

const { ethTransactionReceipt, ethGetLogs, GatewayUnavailableError } = await import("../src/api/gateway.ts");

interface RpcRequest {
  readonly jsonrpc: string;
  readonly id: string;
  readonly method: string;
  readonly params: readonly unknown[];
}

async function withGateway(result: unknown, run: (requests: readonly RpcRequest[]) => Promise<void>): Promise<void> {
  const requests: RpcRequest[] = [];
  const server = createServer(async (request, response) => {
    let body = "";
    for await (const chunk of request) body += String(chunk);
    const document: RpcRequest = JSON.parse(body);
    requests.push(document);
    response.writeHead(200, { "content-type": "application/json" });
    response.end(JSON.stringify({ jsonrpc: "2.0", id: document.id, result }));
  });
  const previous = process.env.LAYERX_GATEWAY_URL;
  try {
    await new Promise<void>((resolve, reject) => {
      server.once("error", reject);
      server.listen(0, "127.0.0.1", resolve);
    });
    const address = server.address();
    assert.ok(address !== null && typeof address !== "string");
    process.env.LAYERX_GATEWAY_URL = `http://127.0.0.1:${address.port}`;
    await run(requests);
  } finally {
    if (previous === undefined) delete process.env.LAYERX_GATEWAY_URL;
    else process.env.LAYERX_GATEWAY_URL = previous;
    await new Promise<void>((resolve, reject) => {
      server.close((error) => error === undefined ? resolve() : reject(error));
      server.closeAllConnections();
    });
  }
}

const HASH = `0x${"aB".repeat(32)}`;
const BLOCK_NUMBER = "0x20000000000001";
const LOGS = [
  {
    address: `0x${"cD".repeat(20)}`,
    topics: [`0x${"eF".repeat(32)}`, `0x${"12".repeat(32)}`],
    data: "0x00aBff",
    blockNumber: BLOCK_NUMBER,
    transactionHash: HASH,
    logIndex: "0xa",
  },
  {
    address: `0x${"34".repeat(20)}`,
    topics: [],
    data: "0x",
    blockNumber: BLOCK_NUMBER,
    transactionHash: HASH,
    logIndex: "0xb",
  },
];
const RECEIPT = { transactionHash: HASH, blockNumber: BLOCK_NUMBER, status: "0x1", logs: LOGS };

async function eth_transaction_receipt_decodes_status_and_logs(): Promise<void> {
  const expectedLogs = LOGS.map((entry) => ({
    ...entry,
    blockNumber: BigInt(entry.blockNumber),
    logIndex: BigInt(entry.logIndex),
  }));
  for (const [status, decodedStatus] of [["0x1", "success"], ["0x0", "reverted"]] as const) {
    await withGateway({ ...RECEIPT, status }, async (requests) => {
      assert.deepEqual(await ethTransactionReceipt(HASH), {
        transactionHash: HASH,
        blockNumber: 9_007_199_254_740_993n,
        status: decodedStatus,
        logs: expectedLogs,
      });
      assert.equal(requests.length, 1);
      const request = requests[0];
      assert.ok(request);
      assert.equal(request.jsonrpc, "2.0");
      assert.equal(request.method, "eth_getTransactionReceipt");
      assert.deepEqual(request.params, [HASH]);
    });
  }
  await withGateway(LOGS, async () => {
    assert.deepEqual(await ethGetLogs({ address: LOGS[0]!.address, topics: [], fromBlock: 0n }), expectedLogs);
  });
  for (const result of [
    { ...RECEIPT, status: "0x2" },
    { ...RECEIPT, status: null },
    ...["42", "0x", "0xgg", "-0x1", 42, null].map((blockNumber) => ({ ...RECEIPT, blockNumber })),
    { ...RECEIPT, transactionHash: "0x1234" },
    { ...RECEIPT, logs: null },
    { ...RECEIPT, logs: [{ ...LOGS[0], blockNumber: "no quantity" }] },
    { ...RECEIPT, logs: [{ ...LOGS[0], topics: [123] }] },
  ]) {
    await withGateway(result, async () => {
      await assert.rejects(ethTransactionReceipt(HASH), GatewayUnavailableError);
    });
  }
}

async function eth_transaction_receipt_is_undefined_while_pending(): Promise<void> {
  await withGateway(null, async (requests) => {
    for (const hash of ["", "0x", "ab".repeat(32), `0x${"ab".repeat(31)}`, `0x${"ab".repeat(33)}`, `0x${"gg".repeat(32)}`]) {
      await assert.rejects(ethTransactionReceipt(hash), TypeError);
    }
    assert.equal(requests.length, 0, "invalid hashes must fail before making a request");
    assert.equal(await ethTransactionReceipt(HASH), undefined);
    assert.equal(requests.length, 1);
    assert.equal(requests[0]?.method, "eth_getTransactionReceipt");
    assert.deepEqual(requests[0]?.params, [HASH]);
  });
}

test(eth_transaction_receipt_decodes_status_and_logs);
test(eth_transaction_receipt_is_undefined_while_pending);
