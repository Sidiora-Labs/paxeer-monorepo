import assert from "node:assert/strict";
import { once } from "node:events";
import { mkdtemp, readFile, rm, writeFile } from "node:fs/promises";
import { createServer } from "node:http";
import { tmpdir } from "node:os";
import { join } from "node:path";
import { decodeRawEip1559, keccak256 } from "@sidiora/layerx-sdk";
import { createMerchantApplicationServer } from "../support/merchant-app.mjs";
import { loadMerchantShopConfig } from "./config.mjs";
import {
  ConformanceSequencer,
  buildSignedReceipt,
  fixedBytes,
  toHex,
} from "../../middleware/conformance/src/receipts.ts";

export const document = JSON.parse(await readFile(new URL("./layerx.example.json", import.meta.url), "utf8"));
export const beta = { name: "beta", ...document.environments.beta };
export const bytes = (value) => Buffer.from(value.slice(2), "hex");
export const hex = (value) => `0x${Buffer.from(value).toString("hex")}`;

export function bindEnvironment(context, bindings) {
  const previous = Object.fromEntries(Object.keys(bindings).map((name) => [name, process.env[name]]));
  context.after(() => {
    for (const [name, value] of Object.entries(previous)) {
      if (value === undefined) delete process.env[name];
      else process.env[name] = value;
    }
  });
  for (const [name, value] of Object.entries(bindings)) {
    if (value === undefined) delete process.env[name];
    else process.env[name] = value;
  }
}

async function listen(server) {
  server.listen(0, "127.0.0.1");
  await once(server, "listening");
  return `http://127.0.0.1:${server.address().port}`;
}

async function close(server) {
  if (!server?.listening) return;
  await new Promise((resolve, reject) => {
    server.close((error) => error ? reject(error) : resolve());
    server.closeAllConnections();
  });
}

async function readMessage(request) {
  const chunks = [];
  for await (const chunk of request) chunks.push(chunk);
  return JSON.parse(Buffer.concat(chunks).toString("utf8"));
}

export async function createMerchantRepairFixture(context) {
  const directory = await mkdtemp(join(tmpdir(), "merchant-shop-origins-"));
  const sequencer = await ConformanceSequencer.generate();
  const key = sequencer.receiptKey();
  const receipts = new Map();
  const chainRequests = [];
  const settlementRequests = [];
  const failures = [];
  const token = "merchant-shop-loopback-token";
  const service = { settlementResponse: undefined, settlementStatus: 200 };
  const chain = createServer(async (request, response) => {
    try {
      assert.equal(request.method, "POST");
      assert.equal(request.url, "/", "chain JSON-RPC must use the root origin");
      assert.equal(request.headers.authorization, undefined, "gateway bearer must stay at the gateway");
      const message = await readMessage(request);
      chainRequests.push(message);
      assert.equal(message.jsonrpc, "2.0");
      assert.equal(message.id, 1);
      let result;
      if (message.method === "eth_chainId") {
        assert.deepEqual(message.params, []);
        result = "0x7d";
      } else {
        const receipt = receipts.get(message.params[0]);
        assert.ok(receipt, "RPC request must name a produced transaction");
        assert.deepEqual(message.params, [receipt.txHash]);
        if (message.method === "eth_getRawTransactionByHash") result = receipt.rawTransaction;
        else {
          assert.equal(message.method, "pax_sequencerReceipt");
          result = { txHash: receipt.txHash, rung: "receipted", receipt: receipt.signedReceipt };
        }
      }
      response.writeHead(200, { "content-type": "application/json" })
        .end(JSON.stringify({ jsonrpc: "2.0", id: message.id, result }));
    } catch (error) {
      failures.push(error);
      response.writeHead(500).end(JSON.stringify({ error: error.message }));
    }
  });
  const gateway = createServer(async (request, response) => {
    try {
      assert.equal(request.method, "POST");
      assert.equal(request.url, "/v1/settle");
      if (request.headers.authorization !== `Bearer ${token}`) {
        response.writeHead(401, { "content-type": "application/json" }).end(JSON.stringify({ state: "refused" }));
        return;
      }
      assert.equal(request.headers["content-type"], "application/json");
      const message = await readMessage(request);
      settlementRequests.push(message);
      assert.equal(message.principal, "merchant-customer");
      assert.equal(message.payload.x402Version, 2);
      assert.equal(message.requirements.network, "eip155:125");
      assert.deepEqual(message.payload.accepted, message.requirements);
      assert.match(message.requestDigest, /^[0-9a-f]{64}$/u);
      assert.match(message.idempotencyKey, /^[0-9a-f]{64}$/u);
      const transaction = message.payload.payload.transaction;
      assert.ok(receipts.has(transaction), "settlement must reference a produced payment");
      response.writeHead(service.settlementStatus, { "content-type": "application/json" })
        .end(JSON.stringify(service.settlementResponse ?? { state: "settled", transaction }));
    } catch (error) {
      failures.push(error);
      response.writeHead(500).end(JSON.stringify({ error: error.message }));
    }
  });
  let application;
  let applicationUrl;
  context.after(async () => {
    await close(application);
    await close(gateway);
    await close(chain);
    await rm(directory, { recursive: true, force: true });
    assert.deepEqual(failures, [], "all peer contract assertions must pass");
  });
  const rpcUrl = await listen(chain);
  const settlementUrl = `${await listen(gateway)}/v1/settle`;
  bindEnvironment(context, {
    LAYERX_EXAMPLE_ENDPOINT: undefined,
    LAYERX_EXAMPLE_STATE_ROOT: undefined,
    LAYERX_EXAMPLE_CONFIG: join(directory, "layerx.example.json"),
    [beta.settlementUrlEnvironment]: settlementUrl,
    [beta.tokenEnvironment]: token,
    [beta.receiptKeysEnvironment]: JSON.stringify([{
      keyId: hex(key.keyId), publicKey: hex(key.publicKey),
      firstHeight: key.firstHeight.toString(), lastHeight: key.lastHeight.toString(),
    }]),
    [beta.webhookKeysEnvironment]: JSON.stringify({ sequencer: hex(sequencer.publicKey) }),
    [beta.assetEnvironment]: toHex(fixedBytes(0x11)),
    [beta.payToEnvironment]: toHex(fixedBytes(0x22)),
    [beta.accountEnvironment]: "agent:merchant:main",
    [beta.currencyEnvironment]: "USD",
    [beta.priceEnvironment]: "25",
  });
  await writeFile(join(directory, "layerx.example.json"), JSON.stringify({
    ...document,
    environments: { ...document.environments, beta: { ...document.environments.beta, rpcUrl } },
  }));
  const previousArguments = process.argv;
  context.after(() => { process.argv = previousArguments; });
  process.argv = [process.execPath, "merchant-shop", "--environment", "beta"];
  const config = await loadMerchantShopConfig(new URL("./index.mjs", import.meta.url).href);
  // The fixture substitutes only the chain origin's authority; its root route
  // and the independently bound hosted settlement URL follow the shipped profile.
  assert.equal(new URL(beta.rpcUrl).pathname, new URL(config.rpcUrl).pathname);
  assert.equal(config.settlementUrl, settlementUrl);
  assert.notEqual(new URL(config.rpcUrl).origin, new URL(config.settlementUrl).origin);

  async function reopen() {
    await close(application);
    application = createMerchantApplicationServer({ ...config, directory });
    applicationUrl = await listen(application);
  }
  await reopen();

  async function checkout(checkoutKey, payment) {
    return fetch(`${applicationUrl}/checkout`, {
      method: "POST",
      headers: { "content-type": "application/json", ...(payment === undefined ? {} : { "payment-signature": payment }) },
      body: JSON.stringify({ principal: "merchant-customer", checkout_key: checkoutKey,
        lines: [{ sku: "metered-report", quantity: 1 }] }),
      signal: AbortSignal.timeout(8000),
    });
  }

  async function produce(purposeHash) {
    const receipt = await buildSignedReceipt(sequencer, {
      asset: fixedBytes(0x11), payTo: fixedBytes(0x22), amount: 25n,
      from: fixedBytes(0x33), batchId: fixedBytes(0x44),
      previousStateRoot: fixedBytes(0x55), resultingStateRoot: fixedBytes(0x66),
      ...(purposeHash === undefined ? {} : { purposeHash }),
    });
    assert.equal(decodeRawEip1559(bytes(receipt.rawTransaction)).chainId, 125n);
    assert.equal(hex(keccak256(bytes(receipt.rawTransaction))), receipt.txHash);
    receipts.set(receipt.txHash, receipt);
    return receipt;
  }

  return { config, directory, key, chainRequests, settlementRequests, service,
    checkout, produce, reopen,
    order: async (id) => {
      const response = await fetch(`${applicationUrl}/orders/${encodeURIComponent(id)}`);
      assert.equal(response.status, 200);
      return response.json();
    },
  };
}
