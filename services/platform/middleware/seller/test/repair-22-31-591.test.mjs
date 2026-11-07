import assert from "node:assert/strict";
import { once } from "node:events";
import { createServer } from "node:http";
import test from "node:test";
import { HostedGatewayRpc } from "../dist/index.js";

async function hostedGatewayPostsToRootAndRefusesNonPxMethods(context) {
  const requests = [];
  const server = createServer(async (request, response) => {
    try {
      const chunks = [];
      for await (const chunk of request) chunks.push(Buffer.from(chunk));
      const body = JSON.parse(Buffer.concat(chunks).toString("utf8"));
      requests.push({ method: request.method, path: request.url, headers: request.headers, body });
      if (request.method !== "POST" || request.url !== "/") {
        response.writeHead(404).end();
        return;
      }
      response.writeHead(200, { "content-type": "application/json" });
      response.end(JSON.stringify({ jsonrpc: "2.0", id: body.id, result: body.params }));
    } catch (error) {
      response.writeHead(500).end(String(error));
    }
  });
  context.after(async () => {
    const closed = new Promise((resolve, reject) => {
      server.close((error) => error ? reject(error) : resolve());
    });
    server.closeAllConnections();
    await closed;
  });
  server.listen(0, "127.0.0.1");
  await once(server, "listening");
  const origin = `http://127.0.0.1:${server.address().port}`;
  const params = [{ address: "0x1111111111111111111111111111111111111111" }];
  for (const endpoint of [origin, `${origin}/`]) {
    const client = new HostedGatewayRpc(endpoint, { "x-payment-client": "seller" });
    for (const method of ["px_register", "px_requestFunds"]) {
      assert.deepEqual(await client.call(method, params), params);
    }
    for (const method of ["eth_chainId", "pax_sequencerReceipt", "lx_getAccount", "px_", "px_register/"]) {
      assert.throws(() => client.call(method, []), /invalid-gateway-method/);
    }
  }
  assert.equal(requests.length, 4);
  for (const [index, request] of requests.entries()) {
    assert.equal(request.method, "POST");
    assert.equal(request.path, "/");
    assert.equal(request.headers["content-type"], "application/json");
    assert.equal(request.headers["x-payment-client"], "seller");
    assert.deepEqual(request.body, {
      jsonrpc: "2.0", id: index % 2 + 1,
      method: index % 2 === 0 ? "px_register" : "px_requestFunds", params,
    });
  }
  assert.throws(() => new HostedGatewayRpc(`${origin}/rpc`), /invalid-rpc-endpoint/);
}

test(hostedGatewayPostsToRootAndRefusesNonPxMethods);
