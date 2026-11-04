import assert from "node:assert/strict";
import { once } from "node:events";
import { readFileSync } from "node:fs";
import { createServer } from "node:http";
import { registerHooks } from "node:module";
import ts from "typescript";

import { abiSelector } from "../../src/exchange.js";
import { discoverSponsoredNonce, SIDIORA_TOKEN, type GasStationConfig } from "../../src/gas-station.js";
import { JsonRpcClient } from "../../src/rpc.js";

export async function gas_nonce_root_route_uses_pending(): Promise<void> {
  const walletRpc = new URL("../../../../../../../sdk/wallet/src/rpc.ts", import.meta.url);
  const loader = registerHooks({
    load(url, context, next) {
      if (url !== walletRpc.href) return next(url, context);
      return {
        format: "module", shortCircuit: true,
        source: ts.transpileModule(readFileSync(walletRpc, "utf8"), {
          compilerOptions: { module: ts.ModuleKind.ESNext, target: ts.ScriptTarget.ES2022 },
          fileName: walletRpc.pathname,
        }).outputText,
      };
    },
  });
  const account = `0x${"11".repeat(20)}`;
  const config: GasStationConfig = {
    chainId: 125n, sponsor: `0x${"22".repeat(20)}`, token: SIDIORA_TOKEN,
    decimals: 6, paymaster: `0x${"44".repeat(20)}`,
  };
  let nonce = 7n;
  let malformedNonce = false;
  const calls: { readonly method: string; readonly params: readonly unknown[] }[] = [];
  const failures: unknown[] = [];
  const server = createServer(async (request, response) => {
    if (request.url !== "/") {
      request.resume();
      response.writeHead(404);
      response.end();
      return;
    }
    try {
      assert.equal(request.method, "POST");
      assert.equal(request.headers["content-type"], "application/json");
      const chunks: Buffer[] = [];
      for await (const chunk of request) chunks.push(Buffer.from(chunk));
      const body = JSON.parse(Buffer.concat(chunks).toString("utf8")) as {
        readonly jsonrpc: string; readonly id: number | string;
        readonly method: string; readonly params: readonly unknown[];
      };
      assert.equal(body.jsonrpc, "2.0");
      calls.push({ method: body.method, params: body.params });
      let result: string;
      switch (body.method) {
        case "eth_chainId":
          assert.deepEqual(body.params, []);
          result = "0x7d";
          break;
        case "eth_getCode":
          assert.deepEqual(body.params, [account, "pending"]);
          result = `0xef0100${config.paymaster.slice(2)}`;
          break;
        case "eth_call":
          assert.deepEqual(body.params, [{ to: account, data: abiSelector("nonce()") }, "pending"]);
          result = malformedNonce ? "0x01" : `0x${nonce.toString(16).padStart(64, "0")}`;
          break;
        case "eth_getTransactionCount":
          assert.deepEqual(body.params, [account, "pending"]);
          result = `0x${nonce.toString(16)}`;
          break;
        default:
          throw new Error(`unexpected nonce producer method ${body.method}`);
      }
      response.writeHead(200, { "content-type": "application/json" });
      response.end(JSON.stringify({ jsonrpc: "2.0", id: body.id, result }));
    } catch (error) {
      failures.push(error);
      response.destroy();
    }
  });
  try {
    server.listen(0, "127.0.0.1");
    await once(server, "listening");
    const address = server.address();
    assert(address !== null && typeof address !== "string");
    const endpoint = `http://127.0.0.1:${address.port}`;
    const { JsonRpcClient: WalletJsonRpcClient } = await import(walletRpc.href);
    const wallet = new WalletJsonRpcClient({ url: endpoint });
    const request = (args: { readonly method: string; readonly params: readonly unknown[] }): Promise<unknown> =>
      wallet.call(args.method, args.params);
    for (const expected of [7n, 8n]) {
      nonce = expected;
      assert.deepEqual(await discoverSponsoredNonce(config, account, request), { ok: true, value: expected });
    }
    const triple = [
      { method: "eth_chainId", params: [] },
      { method: "eth_getCode", params: [account, "pending"] },
      { method: "eth_call", params: [{ to: account, data: abiSelector("nonce()") }, "pending"] },
    ];
    assert.deepEqual(calls, [...triple, ...triple]);
    assert.equal(await new JsonRpcClient(endpoint).getIdentitySequence(account), 8n);
    assert.deepEqual(calls.at(-1), { method: "eth_getTransactionCount", params: [account, "pending"] });
    malformedNonce = true;
    assert.deepEqual(await discoverSponsoredNonce(config, account, request), {
      ok: false, refusal: { code: "replay_state_unavailable", field: "nonce" },
    });
    for (const suffix of ["/rpc", "/rpc/ws"]) {
      const before: number = calls.length;
      const wrong = new WalletJsonRpcClient({ url: endpoint + suffix });
      await assert.rejects(wrong.call("eth_chainId", []));
      assert.equal(calls.length, before);
      await assert.rejects(new JsonRpcClient(endpoint + suffix).getIdentitySequence(account));
      assert.equal(calls.length, before);
    }
    assert.deepEqual(failures, []);
  } finally {
    server.closeAllConnections();
    if (server.listening) await new Promise<void>((resolve, reject) => server.close(error => error ? reject(error) : resolve()));
    loader.deregister();
  }
}
