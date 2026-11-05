import assert from "node:assert/strict";
import { existsSync, readFileSync } from "node:fs";
import { createServer } from "node:https";
import { registerHooks } from "node:module";
import test, { type TestContext } from "node:test";
import { getCACertificates, setDefaultCACertificates } from "node:tls";
import { fileURLToPath } from "node:url";
import ts from "typescript";
import type { PrecompileSendOutcome } from "../src/api/wallet.ts";

registerHooks({
  resolve(specifier, context, nextResolve) {
    if (specifier.startsWith(".") && context.parentURL !== undefined) {
      const candidate = new URL(specifier, context.parentURL);
      for (const suffix of [".ts", ".tsx", "/index.ts"]) {
        const source = candidate.href.endsWith(".js")
          ? new URL(candidate.href.slice(0, -3) + suffix)
          : new URL(candidate.href + suffix);
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
        format: "module",
        shortCircuit: true,
        source: ts.transpileModule(readFileSync(new URL(url), "utf8"), {
          fileName: fileURLToPath(url),
          compilerOptions: {
            target: ts.ScriptTarget.ES2022,
            module: ts.ModuleKind.ESNext,
            jsx: ts.JsxEmit.ReactJSX,
          },
        }).outputText,
      };
    }
    const loaded = nextLoad(url, context);
    return loaded.format === "commonjs" ? { ...loaded, source: undefined } : loaded;
  },
});

const { gatewayTransactionReceipt } = await import("../src/api/gateway-actions.ts");
const { GatewayUnavailableError } = await import("../src/api/gateway.ts");
const { isPrecompileSendOutcome } = await import("../src/api/wallet.ts");
const { preflightPerpsWrite } = await import("../src/app/exchange/actions.ts");
const { perpsCancelOrderCall } = await import("../src/api/sdk.ts");
const { SendOutcome } = await import("../src/app/_markets/send-outcome.tsx");
const { createElement } = await import("react");
const { renderToStaticMarkup } = await import("react-dom/server");

const WEB_DEPLOYMENT = readFileSync(
  new URL("../../../services/platform/hosted/human/web-deployment.yaml", import.meta.url),
  "utf8",
);

function deploymentGateway(): URL {
  const web = WEB_DEPLOYMENT.split("\n---\n")[0];
  assert.ok(web);
  const origin = /- name: LAYERX_GATEWAY_URL\n\s+value: (\S+)/u.exec(web)?.[1];
  assert.ok(origin, "receipt-deployment-wiring: the web container needs its runtime gateway origin");
  const endpoint = new URL(origin);
  assert.equal(endpoint.href, "https://layerx-gateway.layerx-testnet.svc.cluster.local/");
  return endpoint;
}

function receipt_deployment_wiring(): void {
  deploymentGateway();
  const documents = WEB_DEPLOYMENT.split("\n---\n");
  const web = documents[0];
  assert.ok(web);
  assert.match(web, /- name: NODE_EXTRA_CA_CERTS\n\s+value: \/run\/layerx\/trust\/ca\.crt/u);
  assert.match(web, /- name: trust\n\s+mountPath: \/run\/layerx\/trust\n\s+readOnly: true/u);
  assert.match(web, /- name: trust\n\s+secret:\n\s+secretName: layerx-internal-ca/u);
  assert.doesNotMatch(web, /NODE_TLS_REJECT_UNAUTHORIZED|rejectUnauthorized|--use-insecure/u);

  const egress = documents.find((document) =>
    document.includes("kind: NetworkPolicy\nmetadata:\n  name: layerx-human-web\n"),
  );
  assert.ok(egress);
  assert.match(egress, /namespace: layerx-testnet/u);
  assert.match(egress, /podSelector:\n\s+matchLabels:\n\s+app: layerx-human-web/u);
  assert.match(egress, /policyTypes:\n  - Ingress\n  - Egress/u);
  assert.match(egress, /egress:[\s\S]*- to:\n    - podSelector:\n        matchLabels:\n          app: layerx-gateway\n    ports:\n    - protocol: TCP\n      port: 9443(?:\n|$)/u);

  const ingress = documents.find((document) =>
    document.includes("kind: NetworkPolicy\nmetadata:\n  name: layerx-human-web-gateway\n"),
  );
  assert.ok(ingress, "receipt-deployment-wiring: the gateway must admit the web pod");
  assert.match(ingress, /namespace: layerx-testnet/u);
  assert.match(ingress, /podSelector:\n    matchLabels:\n      app: layerx-gateway\n/u);
  assert.match(ingress, /policyTypes:\n  - Ingress\n  ingress:\n  - from:\n    - podSelector:\n        matchLabels:\n          app: layerx-human-web\n    ports:\n    - protocol: TCP\n      port: 9443(?:\n|$)/u);

  // Check the other side of the deployment interface: Service 443 -> pod 9443.
  const gateway = readFileSync(
    new URL("../../../services/platform/hosted/gateway/deployment.yaml", import.meta.url), "utf8",
  );
  assert.match(gateway, /ports: \[\{name: https, containerPort: 9443\}\]/u);
  assert.match(gateway, /kind: Service\nmetadata: \{name: layerx-gateway, namespace: layerx-testnet\}\nspec: \{selector: \{app: layerx-gateway\}, ports: \[\{name: https, port: 443, targetPort: https\}\]\}/u);
}

interface RpcRequest {
  readonly jsonrpc: string;
  readonly id: string;
  readonly method: string;
  readonly params: readonly unknown[];
}

interface RecordedReceipt {
  readonly transactionHash: string;
  readonly blockNumber: string;
  readonly status: string;
  readonly logs: readonly Readonly<{
    address: string;
    topics: readonly string[];
    data: string;
    transactionHash: string;
    blockNumber: string;
    logIndex: string;
  }>[];
}

async function receipt_deployment_reaches_market_consumer(context: TestContext): Promise<void> {
  const deployed = deploymentGateway();
  // Replay a committed node receipt, including its original log and block hash.
  // Only the JSON-RPC correlation id changes when the transcript is served.
  const exchanges: { request: { body: string }; response: { body: string } }[] = JSON.parse(
    readFileSync(new URL(
      "../../../services/explorer/backend/apps/explorer/test/support/fixture/vcr_cassettes/transaction_importer_import_1_receipt.json",
      import.meta.url,
    ), "utf8"),
  );
  const exchange = exchanges[0];
  assert.ok(exchange);
  const recordedRequest: { method: string; params: readonly string[] } = JSON.parse(exchange.request.body);
  const recorded: { jsonrpc: string; result: RecordedReceipt; id: number } = JSON.parse(exchange.response.body);
  const hash = recorded.result.transactionHash;
  assert.equal(recordedRequest.method, "eth_getTransactionReceipt");
  assert.deepEqual(recordedRequest.params, [hash]);
  assert.equal(recorded.result.status, "0x1");
  assert.equal(recorded.result.blockNumber, "0xc10ab");
  assert.equal(recorded.result.logs.length, 1);

  const previousGateway = process.env.LAYERX_GATEWAY_URL;
  const previousTls = process.env.NODE_TLS_REJECT_UNAUTHORIZED;
  const previousCertificates = getCACertificates("default");
  context.after(() => {
    if (previousGateway === undefined) delete process.env.LAYERX_GATEWAY_URL;
    else process.env.LAYERX_GATEWAY_URL = previousGateway;
    if (previousTls === undefined) delete process.env.NODE_TLS_REJECT_UNAUTHORIZED;
    else process.env.NODE_TLS_REJECT_UNAUTHORIZED = previousTls;
    setDefaultCACertificates(previousCertificates);
  });
  process.env.NODE_TLS_REJECT_UNAUTHORIZED = "1";
  const cert = readFileSync(new URL("fixtures/localhost-cert.pem", import.meta.url), "utf8");
  const key = readFileSync(new URL("fixtures/localhost-key.pem", import.meta.url), "utf8");
  const requests: RpcRequest[] = [];
  const failures: unknown[] = [];
  const preflightCall = perpsCancelOrderCall(1n);
  let pending = true;
  const server = createServer({ cert, key }, (request, response) => {
    void (async () => {
      assert.equal(request.method, "POST");
      assert.equal(request.url, deployed.pathname);
      assert.equal(request.headers["content-type"], "application/json");
      let body = "";
      for await (const chunk of request) body += String(chunk);
      const rpc: RpcRequest = JSON.parse(body);
      requests.push(rpc);
      assert.equal(rpc.jsonrpc, "2.0");
      assert.equal(typeof rpc.id, "string");
      let result: unknown;
      if (rpc.method === recordedRequest.method) {
        assert.deepEqual(rpc.params, recordedRequest.params);
        result = pending ? null : recorded.result;
        pending = false;
      } else {
        assert.equal(rpc.method, "eth_call");
        assert.deepEqual(rpc.params, [{ from: `0x${"11".repeat(20)}`, to: preflightCall.to, data: preflightCall.data }, "latest"]);
        result = "0x";
      }
      response.writeHead(200, { "content-type": "application/json" });
      response.end(JSON.stringify({ jsonrpc: recorded.jsonrpc, id: rpc.id, result }));
    })().catch((error: unknown) => {
      failures.push(error);
      response.writeHead(500);
      response.end();
    });
  });
  await new Promise<void>((resolve, reject) => {
    server.once("error", reject);
    server.listen(0, "127.0.0.1", resolve);
  });
  context.after(async () => {
    await new Promise<void>((resolve, reject) => {
      server.close((error) => error === undefined ? resolve() : reject(error));
      server.closeAllConnections();
    });
  });
  const address = server.address();
  assert.ok(address !== null && typeof address !== "string");
  // Relocate the declared HTTPS root onto the loopback transcript server.
  const local = new URL(deployed);
  local.hostname = "127.0.0.1";
  local.port = String(address.port);

  delete process.env.LAYERX_GATEWAY_URL;
  await assert.rejects(gatewayTransactionReceipt(hash), (error: unknown) =>
    error instanceof GatewayUnavailableError && error.message.includes("LAYERX_GATEWAY_URL is not set"),
  );
  process.env.LAYERX_GATEWAY_URL = local.href;
  setDefaultCACertificates(getCACertificates("bundled"));
  await assert.rejects(gatewayTransactionReceipt(hash), GatewayUnavailableError);
  assert.equal(requests.length, 0, "an untrusted certificate must prevent the RPC request");
  setDefaultCACertificates([...getCACertificates("bundled"), cert]);

  assert.equal(await gatewayTransactionReceipt(hash), undefined, "only a null receipt is pending");
  const receipt = await gatewayTransactionReceipt(hash);
  assert.ok(receipt);
  assert.deepEqual(receipt, {
    transactionHash: hash,
    blockNumber: BigInt(recorded.result.blockNumber),
    status: "success",
    logs: recorded.result.logs.map((log) => ({
      address: log.address, topics: log.topics, data: log.data,
      transactionHash: log.transactionHash, blockNumber: BigInt(log.blockNumber),
      logIndex: BigInt(log.logIndex),
    })),
  });
  const outcome: PrecompileSendOutcome = { outcome: "sent", transactionHash: hash, receipt };
  assert.ok(isPrecompileSendOutcome(outcome));
  const html = renderToStaticMarkup(createElement(SendOutcome, { outcome }));
  assert.ok(html.includes(`Sent. Transaction ${hash}`));
  assert.ok(html.includes(`Receipt: block ${BigInt(recorded.result.blockNumber)}, logs 1.`));
  assert.match(html, /\bbg-success-soft\b/u);
  assert.doesNotMatch(html, /Awaiting confirmation|confirmation is unavailable|malformed result/u);

  assert.deepEqual(await preflightPerpsWrite(`0x${"11".repeat(20)}`, preflightCall.data), { ok: true });
  assert.deepEqual(requests.map((request) => request.method), [
    "eth_getTransactionReceipt", "eth_getTransactionReceipt", "eth_call",
  ]);
  assert.equal(new Set(requests.map((request) => request.id)).size, requests.length);
  assert.deepEqual(failures, []);
}

test(receipt_deployment_wiring);
test(receipt_deployment_reaches_market_consumer);
