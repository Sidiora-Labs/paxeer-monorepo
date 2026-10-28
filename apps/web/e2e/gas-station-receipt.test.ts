import assert from "node:assert/strict";
import { existsSync, readFileSync } from "node:fs";
import { createServer, type RequestListener, type Server } from "node:http";
import { createServer as createSecureServer } from "node:https";
import { registerHooks } from "node:module";
import test from "node:test";
import { getCACertificates, setDefaultCACertificates } from "node:tls";
import { fileURLToPath } from "node:url";
import ts from "typescript";
import type { GasQuoteRequest, GasStationConfig } from "../../../services/agent/sdk/typescript/src/gas-station.ts";

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

const station = await import("../src/api/gas-station.ts");
const sdk = await import("../../../services/agent/sdk/typescript/src/gas-station.ts");
const { privateKeyToAccount } = await import("viem/accounts");

const account = privateKeyToAccount(`0x${"0b".repeat(32)}`);
const sponsor = privateKeyToAccount(`0x${"0c".repeat(32)}`);
const config: GasStationConfig = {
  chainId: 1325n,
  sponsor: sponsor.address,
  paymaster: `0x${"44".repeat(20)}`,
  token: sdk.SIDIORA_TOKEN,
  decimals: sdk.SIDIORA_DECIMALS,
};
const quoteRequest: GasQuoteRequest = {
  account: account.address,
  did: `did:layerx:${"55".repeat(32)}`,
  nonce: 7n,
  calls: [{ to: `0x${"33".repeat(20)}`, value: 0n, data: "0x1234" }],
  maxTokenAmount: 2_100_000n,
  gasCost: 10n ** 18n,
};
const HASH = `0x${"aB".repeat(32)}`;
const LOG = {
  address: config.paymaster,
  topics: [`0x${"66".repeat(32)}`],
  data: "0x1234",
  transactionHash: HASH,
  blockNumber: "0x20000000000001",
  logIndex: "0x2",
};

function wire(value: unknown): unknown {
  return JSON.parse(JSON.stringify(value, (_key, item: unknown) => typeof item === "bigint" ? item.toString() : item));
}

interface ReceivedRequest {
  readonly method: string | undefined;
  readonly url: string | undefined;
  readonly body: unknown;
}

async function listen(server: Server): Promise<number> {
  await new Promise<void>((resolve, reject) => {
    server.once("error", reject);
    server.listen(0, "127.0.0.1", resolve);
  });
  const address = server.address();
  assert.ok(address !== null && typeof address !== "string");
  return address.port;
}

async function withStation(
  quoteStatus: number,
  quoteAnswer: unknown,
  receipt: unknown,
  run: (requests: Readonly<{ quote: ReceivedRequest[]; submit: ReceivedRequest[]; gateway: ReceivedRequest[] }>) => Promise<void>,
  options: Readonly<{ receiptHttpStatus?: number }> = {},
): Promise<void> {
  const key = readFileSync(new URL("./fixtures/localhost-key.pem", import.meta.url), "utf8");
  const cert = readFileSync(new URL("./fixtures/localhost-cert.pem", import.meta.url), "utf8");
  const requests = { quote: [] as ReceivedRequest[], submit: [] as ReceivedRequest[], gateway: [] as ReceivedRequest[] };
  function handler(kind: keyof typeof requests): RequestListener {
    return async (request, response) => {
      let body = "";
      for await (const chunk of request) body += String(chunk);
      const document: unknown = JSON.parse(body);
      requests[kind].push({ method: request.method, url: request.url, body: document });
      response.writeHead(kind === "quote" ? quoteStatus : kind === "gateway" ? options.receiptHttpStatus ?? 200 : 200, { "content-type": "application/json" });
      const answer = kind === "quote" ? quoteAnswer : kind === "submit" ? { transactionHash: HASH } : {
        jsonrpc: "2.0", id: (document as { id: string }).id, result: receipt,
      };
      response.end(JSON.stringify(wire(answer)));
    };
  }
  const quote = createSecureServer({ key, cert }, handler("quote"));
  const submit = createSecureServer({ key, cert }, handler("submit"));
  const gateway = createServer(handler("gateway"));
  const environment = {
    PAXEER_GAS_STATION_QUOTE_URL: "",
    PAXEER_GAS_STATION_SUBMIT_URL: "",
    PAXEER_GAS_STATION_CHAIN_ID: config.chainId.toString(),
    PAXEER_GAS_STATION_SPONSOR: config.sponsor,
    PAXEER_GAS_STATION_PAYMASTER: config.paymaster,
    LAYERX_GATEWAY_URL: "",
  };
  const previous = Object.fromEntries(Object.keys(environment).map((name) => [name, process.env[name]]));
  const previousCertificates = getCACertificates("default");
  try {
    environment.PAXEER_GAS_STATION_QUOTE_URL = `https://127.0.0.1:${await listen(quote)}/quote`;
    environment.PAXEER_GAS_STATION_SUBMIT_URL = `https://127.0.0.1:${await listen(submit)}/submit`;
    environment.LAYERX_GATEWAY_URL = `http://127.0.0.1:${await listen(gateway)}`;
    setDefaultCACertificates([...previousCertificates, cert]);
    Object.assign(process.env, environment);
    await run(requests);
  } finally {
    for (const [name, value] of Object.entries(previous)) {
      if (value === undefined) delete process.env[name];
      else process.env[name] = value;
    }
    setDefaultCACertificates(previousCertificates);
    await Promise.all([quote, submit, gateway].map((server) => new Promise<void>((resolve, reject) => {
      if (!server.listening) return resolve();
      server.close((error) => error === undefined ? resolve() : reject(error));
      server.closeAllConnections();
    })));
  }
}

function assertQuoteRequest(requests: readonly ReceivedRequest[]): void {
  assert.deepEqual(requests, [{
    method: "POST", url: "/quote",
    body: wire({ ...quoteRequest, chainId: config.chainId, token: config.token, decimals: config.decimals }),
  }]);
}

async function gas_quote_reports_budget_exhausted(): Promise<void> {
  for (const status of [403, 422, 429]) {
    await withStation(status, { error: "budget_exhausted" }, null, async (requests) => {
      assert.deepEqual(await station.requestSidioraGasQuote(quoteRequest, config.chainId), {
        ok: false, failure: { outcome: "rejected" }, refusal: "budget_exhausted",
      });
      assertQuoteRequest(requests.quote);
      assert.deepEqual(requests.submit, []);
      assert.deepEqual(requests.gateway, []);
    });
  }
}

async function sponsored_submit_returns_receipt(): Promise<void> {
  const quote = {
    sponsor: config.sponsor, token: config.token, decimals: config.decimals,
    maxTokenAmount: quoteRequest.maxTokenAmount, tokenAmount: 2_000_001n,
    deadline: BigInt(Math.floor(Date.now() / 1000)) + 300n, quoteNonce: 9n, gasCost: quoteRequest.gasCost,
  };
  const quoteDigest = sdk.gasQuoteDigest(config.chainId, account.address, quote);
  assert.ok(quoteDigest.ok);
  const relayerSignature = await sponsor.sign({ hash: quoteDigest.value as `0x${string}` });
  const authorizationNonce = 8n;
  const authorizationDigest = sdk.eip7702AuthorizationDigest({ chainId: config.chainId, address: config.paymaster, nonce: authorizationNonce });
  assert.ok(authorizationDigest.ok);
  const authorizationSignature = await account.sign({ hash: authorizationDigest.value as `0x${string}` });
  for (const [status, receiptHttpStatus] of [["0x1", 200], ["0x0", 200], [null, 200], ["0x1", 503], ["malformed", 200]] as const) {
    const receipt = status === null ? null : { transactionHash: HASH, status, blockNumber: LOG.blockNumber, logs: [LOG] };
    await withStation(200, { quote, relayerSignature, feeDenom: "sidiora" }, receipt, async (requests) => {
      const quoted = await station.requestSidioraGasQuote(quoteRequest, config.chainId);
      assert.ok(quoted.ok);
      assertQuoteRequest(requests.quote);
      const batchDigest = sdk.sponsoredBatchDigest(quoted.value.batch);
      assert.ok(batchDigest.ok);
      const accountSignature = await account.sign({ hash: batchDigest.value as `0x${string}` });
      const outcome = await station.sendSponsoredBatch(quoted.value, accountSignature, authorizationNonce, authorizationSignature);
      if (receiptHttpStatus === 503 || status === "malformed") {
        assert.deepEqual(outcome, { outcome: "unconfirmed", transactionHash: HASH });
      } else if (status === null) {
        assert.deepEqual(outcome, { outcome: "sent", transactionHash: HASH });
      } else {
        assert.deepEqual(outcome, {
          outcome: status === "0x0" ? "reverted" : "sent", transactionHash: HASH,
          receipt: {
            transactionHash: HASH, status: status === "0x0" ? "reverted" : "success", blockNumber: BigInt(LOG.blockNumber),
            logs: [{ ...LOG, blockNumber: BigInt(LOG.blockNumber), logIndex: 2n }],
          },
        });
      }
      const call = sdk.sponsoredBatchCall(config, quoted.value.batch, accountSignature, relayerSignature);
      const authorization = sdk.assembleEip7702Authorization(config, account.address, authorizationNonce, authorizationSignature);
      assert.ok(call.ok);
      assert.ok(authorization.ok);
      assert.deepEqual(requests.submit, [{
        method: "POST", url: "/submit",
        body: wire({ call: call.value, authorization: authorization.value, batch: quoted.value.batch, accountSignature, relayerSignature }),
      }]);
      assert.equal(requests.gateway.length, 1);
      const request = requests.gateway[0];
      assert.ok(request);
      assert.equal(request.method, "POST");
      assert.equal(request.url, "/");
      const document = request.body as { jsonrpc: string; id: string; method: string; params: unknown[] };
      assert.equal(document.jsonrpc, "2.0");
      assert.equal(typeof document.id, "string");
      assert.equal(document.method, "eth_getTransactionReceipt");
      assert.deepEqual(document.params, [HASH]);
    }, { receiptHttpStatus });
  }
}

test(gas_quote_reports_budget_exhausted);
test(sponsored_submit_returns_receipt);
