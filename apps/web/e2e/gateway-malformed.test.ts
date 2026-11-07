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

const { ethBlockNumber, surfaceCapability, GatewayUnavailableError } = await import("../src/api/gateway.ts");

interface RpcRequest {
  readonly jsonrpc: string;
  readonly id: string;
  readonly method: string;
  readonly params: readonly unknown[];
}

async function withGateway(body: string, contentType: string, run: (requests: readonly RpcRequest[]) => Promise<void>): Promise<void> {
  const requests: RpcRequest[] = [];
  const server = createServer(async (request, response) => {
    let requestBody = "";
    for await (const chunk of request) requestBody += String(chunk);
    const document: RpcRequest = JSON.parse(requestBody);
    requests.push(document);
    response.writeHead(200, { "content-type": contentType });
    response.end(body);
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

const MALFORMED_ANSWERS = [
  { body: "<!doctype html><html><body>Bad Gateway</body></html>", contentType: "text/html" },
  { body: '{"jsonrpc":"2.0","result":', contentType: "application/json" },
] as const;

async function gateway_non_json_answer_is_gateway_unavailable(): Promise<void> {
  for (const { body, contentType } of MALFORMED_ANSWERS) {
    await withGateway(body, contentType, async (requests) => {
      await assert.rejects(ethBlockNumber(), (error: unknown) => {
        assert.ok(error instanceof GatewayUnavailableError);
        assert.equal(error.message, "The network gateway is unavailable: eth_blockNumber answered malformed JSON");
        return true;
      });
      assert.equal(requests.length, 1);
      assert.equal(requests[0]?.jsonrpc, "2.0");
      assert.equal(requests[0]?.method, "eth_blockNumber");
      assert.deepEqual(requests[0]?.params, []);
    });
  }
}

async function surface_capability_degrades_on_a_non_json_answer(): Promise<void> {
  for (const { body, contentType } of MALFORMED_ANSWERS) {
    await withGateway(body, contentType, async (requests) => {
      assert.deepEqual(await surfaceCapability("launchpad"), {
        live: false,
        detail: "The network gateway is unavailable: px_getCapabilities answered malformed JSON",
      });
      assert.equal(requests.length, 1);
      assert.equal(requests[0]?.jsonrpc, "2.0");
      assert.equal(requests[0]?.method, "px_getCapabilities");
      assert.deepEqual(requests[0]?.params, []);
    });
  }
}

test(gateway_non_json_answer_is_gateway_unavailable);
test(surface_capability_degrades_on_a_non_json_answer);
