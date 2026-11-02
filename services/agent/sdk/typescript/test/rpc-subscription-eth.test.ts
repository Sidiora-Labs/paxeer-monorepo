import assert from "node:assert/strict";
import { once } from "node:events";
import { test } from "node:test";
import type { TestContext } from "node:test";
import WebSocket, { WebSocketServer } from "ws";
import { subscribeRpc, type SubscriptionEvent } from "../src/rpc-subscription.js";

async function loopback(t: TestContext): Promise<WebSocketServer> {
  const server = new WebSocketServer({ host: "127.0.0.1", port: 0, path: "/rpc/ws" });
  t.after(async () => {
    for (const client of server.clients) client.terminate();
    await new Promise<void>((resolve, reject) => server.close(error => error ? reject(error) : resolve()));
  });
  await once(server, "listening");
  return server;
}

function endpoint(server: WebSocketServer): URL {
  const address = server.address();
  assert(address !== null && typeof address === "object");
  return new URL(`http://127.0.0.1:${address.port}/rpc`);
}

async function request(socket: WebSocket): Promise<unknown> {
  const [data, binary] = await once(socket, "message");
  assert.equal(binary, false);
  return JSON.parse(String(data));
}

function notify(socket: WebSocket, result: Record<string, unknown>): void {
  socket.send(JSON.stringify({ jsonrpc: "2.0", method: "eth_subscription", params: { subscription: "0x1", result } }));
}

async function unsubscribe(socket: WebSocket, stream: AsyncGenerator<SubscriptionEvent>, requestId: string): Promise<void> {
  const message = request(socket);
  const closing = stream.return(undefined);
  assert.deepEqual(await message, { jsonrpc: "2.0", id: `${requestId}:unsubscribe`, method: "eth_unsubscribe", params: ["0x1"] });
  socket.send(JSON.stringify({ jsonrpc: "2.0", id: `${requestId}:unsubscribe`, result: true }));
  assert.deepEqual(await closing, { done: true, value: undefined });
}

async function subscribeRpc_sends_eth_subscribe_newHeads(t: TestContext): Promise<void> {
  const server = await loopback(t);
  const connection = once(server, "connection");
  const stream = subscribeRpc(endpoint(server), undefined, "heads", "heads");
  const first = stream.next();
  const [socket, upgrade] = await connection;
  assert(socket instanceof WebSocket);
  assert.equal(upgrade.url, "/rpc/ws");
  assert.deepEqual(await request(socket), { jsonrpc: "2.0", id: "heads", method: "eth_subscribe", params: ["newHeads"] });
  socket.send(JSON.stringify({ jsonrpc: "2.0", id: "heads", result: "0x1" }));
  const head = { number: "0x29", hash: `0x${"11".repeat(32)}` };
  notify(socket, head);
  assert.deepEqual(await first, { done: false, value: { result: head, cursor: 41n } });
  await unsubscribe(socket, stream, "heads");
}

async function account_topic_subscribes_to_logs_for_the_account(t: TestContext): Promise<void> {
  const server = await loopback(t);
  const connection = once(server, "connection");
  const address = `0x${"ab".repeat(20)}`;
  const stream = subscribeRpc(endpoint(server), undefined, "account", "account", address);
  const first = stream.next();
  const [socket] = await connection;
  assert(socket instanceof WebSocket);
  assert.deepEqual(await request(socket), { jsonrpc: "2.0", id: "account", method: "eth_subscribe", params: ["logs", { address, topics: [] }] });
  socket.send(JSON.stringify({ jsonrpc: "2.0", id: "account", result: "0x1" }));
  const log = { address, topics: [`0x${"22".repeat(32)}`], data: "0x", blockNumber: "0x29", logIndex: "0x0", removed: false };
  notify(socket, log);
  assert.deepEqual(await first, { done: false, value: { result: log, cursor: 41n } });
  const second = stream.next();
  const nextLog = { ...log, logIndex: "0x1" };
  notify(socket, nextLog);
  assert.deepEqual(await second, { done: false, value: { result: nextLog, cursor: 41n } });
  await unsubscribe(socket, stream, "account");
}

async function block_height_continuity_refuses_a_gap(t: TestContext): Promise<void> {
  const server = await loopback(t);
  const connection = once(server, "connection");
  const stream = subscribeRpc(endpoint(server), undefined, "continuity", "heads", undefined, 40n);
  const first = stream.next();
  const [socket] = await connection;
  assert(socket instanceof WebSocket);
  assert.deepEqual(await request(socket), { jsonrpc: "2.0", id: "continuity", method: "eth_subscribe", params: ["newHeads"] });
  socket.send(JSON.stringify({ jsonrpc: "2.0", id: "continuity", result: "0x1" }));
  notify(socket, { number: "0x29" });
  assert.deepEqual(await first, { done: false, value: { result: { number: "0x29" }, cursor: 41n } });
  const consecutive = stream.next();
  notify(socket, { number: "0x2a" });
  assert.deepEqual(await consecutive, { done: false, value: { result: { number: "0x2a" }, cursor: 42n } });
  const closed = once(socket, "close");
  const gap = assert.rejects(stream.next(), /Subscription block height discontinuity; reconcile through reads/u);
  notify(socket, { number: "0x2c" });
  await gap;
  await closed;
  assert.deepEqual(await stream.next(), { done: true, value: undefined });
}

test(subscribeRpc_sends_eth_subscribe_newHeads);
test(account_topic_subscribes_to_logs_for_the_account);
test(block_height_continuity_refuses_a_gap);
