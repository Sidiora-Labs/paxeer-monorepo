import assert from "node:assert/strict";
import { createHash } from "node:crypto";
import { once } from "node:events";
import { createServer, type IncomingHttpHeaders, type ServerResponse } from "node:http";
import { createRequire } from "node:module";
import type { AddressInfo } from "node:net";
import type { Duplex } from "node:stream";
import { after, before, test } from "node:test";
import { act, createElement } from "react";
import type { Root } from "react-dom/client";

const A = "0x00000000000000000000000000000000000000ab";
const B = "0x00000000000000000000000000000000000000cd";
const earned = "123456789012345678901234567890";
// The fixture peer owns its in-memory account/enrollment tables. Only selected
// GraphQL fields cross the real HTTP transport into the production hooks.
const accounts = new Map([
  [A, { balance: "111", referralCode: "ALPHA", totalEarned: earned }],
  [B, { balance: "222", referralCode: "BETA", totalEarned: null }],
]);
const enrollments = new Map([["1", [A, B]], ["2", [B]]]);
const queryLog: string[] = [];
let authSequence = 0;
let origin: string;

interface RequestBody {
  query?: string;
  variables?: { address?: string; id?: string; };
  address?: string;
  code?: string;
  token?: string;
}
class PendingRequest {
  constructor(readonly path: string, readonly method: string, readonly body: RequestBody, readonly headers: IncomingHttpHeaders, private readonly response: ServerResponse) { }
  release(failure = false) {
    const { query = "", variables = {} } = this.body;
    if (failure) {
      this.response.writeHead(this.path === "/graphql" ? 200 : 401, { "Content-Type": "application/json" }).end(JSON.stringify({ errors: [{ message: "fixture query failed" }] }));
      return;
    }
    let result: unknown;
    if (this.path === "/v2/auth/challenge") result = { nonce: String(++authSequence), message: `Sign in ${this.body.address} ${authSequence}` };
    else if (this.path === "/v2/auth/verify" || this.path === "/v2/auth/refresh") result = { token: `verified-${++authSequence}-${this.body.address ?? this.body.token}`, expires_in: 3600 };
    else if (this.path === "/v2/referrals/generate") result = { referralCode: "GENERATED" };
    else if (this.path === "/v2/referrals/apply" || /^\/v2\/competitions\/[^/]+\/join$/.test(this.path) || /^\/v2\/alerts(?:\/[^/]+)?$/.test(this.path)) result = { ok: true };
    else if (this.path !== "/graphql") {
      this.response.writeHead(404, { "Content-Type": "application/json" }).end(JSON.stringify({ error: `Unrecorded fixture route: ${this.method} ${this.path}` }));
      return;
    }
    else {
      const row = accounts.get(variables.address ?? "");
      if (query.includes("userBalances")) result = { data: { userBalances: row ? [{ token: "USDL", balance: row.balance, totalDeposited: row.balance, totalWithdrawn: "0", depositCount: 1, withdrawCount: 0 }] : [] } };
      else if (query.includes("referralTiers")) {
        const user = row ? { referralCode: row.referralCode, tier: null, referralCount: 2, ...(/\btotalEarned\b/.test(query) ? { totalEarned: row.totalEarned } : {}) } : null;
        result = { data: { userReferrals: user, referralTiers: [{ tier: "bronze", minReferrals: 3, minVolume: "1000", rewardBps: 500, feeDiscountBps: 100 }] } };
      } else if (query.includes("userAlerts")) result = { data: { userAlerts: [] } };
      else {
        const competition = (id: string) => ({ id, name: `Competition ${id}`, description: null, startAt: "2020-01-01T00:00:00Z", endAt: "2099-01-01T00:00:00Z", metric: "volume", prizePool: "1000", active: true, ...(/\brankings\b/.test(query) ? { rankings: (enrollments.get(id) ?? []).map((address, index) => ({ rank: index + 1, address, score: String(100 - index) })) } : {}) });
        result = { data: variables.id ? { competition: competition(variables.id) } : { competitions: [...enrollments.keys()].map(competition) } };
      }
    }
    this.response.writeHead(200, { "Content-Type": "application/json" }).end(JSON.stringify(result));
  }
}
const pending: PendingRequest[] = [];
const server = createServer(async (request, response) => {
  let body = "";
  for await (const chunk of request) body += chunk;
  const parsed: RequestBody = body ? JSON.parse(body) : {};
  if (parsed.query) queryLog.push(parsed.query);
  pending.push(new PendingRequest(request.url!, request.method!, parsed, request.headers, response));
});

interface SubscriptionFrame {
  type: string;
  id?: string;
  payload?: { query: string; variables?: { marketId?: number; }; };
}
// RFC6455 loopback peer. It speaks the real graphql-transport-ws protocol;
// neither WebSocket nor subscribeStream is replaced by a test double.
class StreamPeer {
  private buffer = Buffer.alloc(0);
  readonly subscriptions = new Map<string, NonNullable<SubscriptionFrame["payload"]>>();
  constructor(readonly socket: Duplex) {
    socket.on("data", (chunk: Buffer) => { this.buffer = Buffer.concat([this.buffer, chunk]); this.drain(); });
  }
  private write(opcode: number, payload: Buffer) {
    assert.ok(payload.length < 65536);
    const header = payload.length < 126 ? Buffer.from([opcode, payload.length]) : Buffer.from([opcode, 126, payload.length >> 8, payload.length & 255]);
    this.socket.write(Buffer.concat([header, payload]));
  }
  private send(value: unknown) { this.write(0x81, Buffer.from(JSON.stringify(value))); }
  private drain() {
    for (; ;) {
      if (this.buffer.length < 2) return;
      const opcode = this.buffer[0]! & 15;
      assert.ok(this.buffer[1]! & 128);
      let length = this.buffer[1]! & 127;
      let offset = 2;
      if (length === 126) { if (this.buffer.length < 4) return; length = this.buffer.readUInt16BE(2); offset = 4; }
      assert.notEqual(length, 127);
      if (this.buffer.length < offset + 4 + length) return;
      const mask = this.buffer.subarray(offset, offset + 4);
      const payload = Buffer.from(this.buffer.subarray(offset + 4, offset + 4 + length));
      for (let i = 0; i < payload.length; i++) payload[i] = payload[i]! ^ mask[i % 4]!;
      this.buffer = this.buffer.subarray(offset + 4 + length);
      if (opcode === 8) { this.write(0x88, payload); this.socket.end(); return; }
      assert.equal(opcode, 1);
      const frame: SubscriptionFrame = JSON.parse(payload.toString());
      if (frame.type === "connection_init") this.send({ type: "connection_ack" });
      else if (frame.type === "subscribe") { assert.ok(frame.id); assert.ok(frame.payload); this.subscriptions.set(frame.id, frame.payload); }
      else if (frame.type === "complete") { assert.ok(frame.id); this.subscriptions.delete(frame.id); }
      else if (frame.type === "ping") this.send({ type: "pong" });
      else assert.equal(frame.type, "pong");
    }
  }
  publish(marketId: number) {
    for (const [id, { query, variables }] of this.subscriptions) {
      if (variables?.marketId !== marketId) continue;
      const data = query.includes("priceUpdated") ? { priceUpdated: { marketId, price: "1500", timestamp: 123 } } : { tradeExecuted: { tradeId: "trade-1", marketId, isLong: true, sizeUsd: "12", price: "1500", blockTimestamp: 123 } };
      this.send({ id, type: "next", payload: { data } });
    }
  }
  close() { this.write(0x88, Buffer.from([3, 232])); }
}
const peers: StreamPeer[] = [];
let modules: {
  auth: typeof import("../src/hooks/perps/usePerfdbAuth");
  balances: typeof import("../src/hooks/perps/usePerfdbBalances");
  referrals: typeof import("../src/hooks/perps/usePerfdbReferrals");
  competitions: typeof import("../src/hooks/perps/usePerfdbCompetitions");
  subscriptions: typeof import("../src/hooks/perps/usePerfdbSubscriptions");
  alerts: typeof import("../src/hooks/perps/usePerfdbAlerts");
  wallet: typeof import("../src/hooks/wallet/useWalletStore");
};
let createRoot: typeof import("react-dom/client")["createRoot"];
const savedGlobals = new Map<string, PropertyDescriptor | undefined>();
const savedEnv = new Map<string, string | undefined>();
const mounted = new Set<Root>();
let closeDom: () => void;

before(async () => {
  server.on("upgrade", (request, socket) => {
    const accept = createHash("sha1").update(`${request.headers["sec-websocket-key"]}258EAFA5-E914-47DA-95CA-C5AB0DC85B11`).digest("base64");
    socket.write(`HTTP/1.1 101 Switching Protocols\r\nUpgrade: websocket\r\nConnection: Upgrade\r\nSec-WebSocket-Accept: ${accept}\r\nSec-WebSocket-Protocol: graphql-transport-ws\r\n\r\n`);
    peers.push(new StreamPeer(socket));
  });
  server.listen(0, "127.0.0.1");
  await once(server, "listening");
  origin = `http://127.0.0.1:${(server.address() as AddressInfo).port}`;
  for (const [key, value] of Object.entries({ NEXT_PUBLIC_PERPS_GATEWAY_URL: origin, NEXT_PUBLIC_PAXSCAN_BASE_URL: origin, NEXT_PUBLIC_PAXSCAN_API_URL: `${origin}/api`, NEXT_PUBLIC_PAXSCAN_WS_URL: origin.replace("http", "ws") })) {
    savedEnv.set(key, process.env[key]); process.env[key] = value;
  }
  // jsdom is a committed wallet-demo dependency installed by the declared gate.
  const fixtureRequire = createRequire(new URL("../../../services/wallet/demo/package.json", import.meta.url));
  const { JSDOM } = fixtureRequire("jsdom");
  const dom = new JSDOM("<!doctype html><html><body></body></html>", { url: origin, pretendToBeVisual: true });
  assert.equal(dom.window.document.hidden, false);
  closeDom = () => dom.window.close();
  for (const [key, value] of Object.entries({ window: dom.window, document: dom.window.document, IS_REACT_ACT_ENVIRONMENT: true })) {
    savedGlobals.set(key, Object.getOwnPropertyDescriptor(globalThis, key));
    Object.defineProperty(globalThis, key, { value, configurable: true });
  }
  ({ createRoot } = await import("react-dom/client"));
  modules = {
    auth: await import("../src/hooks/perps/usePerfdbAuth"),
    balances: await import("../src/hooks/perps/usePerfdbBalances"),
    referrals: await import("../src/hooks/perps/usePerfdbReferrals"),
    competitions: await import("../src/hooks/perps/usePerfdbCompetitions"),
    subscriptions: await import("../src/hooks/perps/usePerfdbSubscriptions"),
    alerts: await import("../src/hooks/perps/usePerfdbAlerts"),
    wallet: await import("../src/hooks/wallet/useWalletStore"),
  };
  const { PERFDB_GRAPHQL, PERFDB_REST_URL } = await import("../src/core/perfdb/client");
  assert.equal(PERFDB_REST_URL, origin);
  assert.equal(PERFDB_GRAPHQL.wsUrl, `${origin.replace("http", "ws")}/graphql`);
});

after(async () => {
  await act(async () => { for (const root of mounted) root.unmount(); mounted.clear(); });
  for (const request of pending.splice(0)) request.release();
  for (const peer of peers) peer.socket.destroy();
  server.closeAllConnections();
  await new Promise<void>(resolve => server.close(() => resolve()));
  closeDom?.();
  for (const [key, descriptor] of savedGlobals) { if (descriptor) Object.defineProperty(globalThis, key, descriptor); else Reflect.deleteProperty(globalThis, key); }
  for (const [key, value] of savedEnv) { if (value === undefined) delete process.env[key]; else process.env[key] = value; }
});

async function until(predicate: () => boolean) {
  const deadline = Date.now() + 5000;
  while (!predicate()) {
    assert.ok(Date.now() < deadline, "loopback operation must settle within five seconds");
    await act(async () => { await new Promise(resolve => setTimeout(resolve, 5)); });
  }
}
async function take(path: string, identity?: string) {
  const matches = (request: PendingRequest) => request.path === path && (!identity || request.body.variables?.address === identity || request.body.variables?.id === identity || request.body.query?.includes(identity) || request.body.address === identity);
  await until(() => pending.some(matches));
  return pending.splice(pending.findIndex(matches), 1)[0]!;
}
async function release(request: PendingRequest, failure = false) { await act(async () => { request.release(failure); await new Promise(resolve => setTimeout(resolve, 15)); }); }

async function mount<P, T>(hook: (props: P) => T, initial: P) {
  const container = document.createElement("div"); document.body.appendChild(container);
  const root = createRoot(container); mounted.add(root);
  let value: T;
  function Probe({ props }: { props: P; }) { value = hook(props); return createElement("span", null); }
  const render = async (props: P) => { await act(async () => root.render(createElement(Probe, { props }))); };
  await render(initial);
  return { read() { return value!; }, render, async close() { await act(async () => root.unmount()); mounted.delete(root); container.remove(); } };
}
async function select(address: typeof A | typeof B | null) {
  await act(async () => { if (address) modules.wallet.useWalletStore.getState().setConnected(address, 125); else modules.wallet.useWalletStore.getState().setDisconnected(); });
}
async function signIn(auth: ReturnType<typeof modules.auth.usePerfdbAuth>, address = A) {
  let completion!: Promise<void>;
  await act(async () => { completion = auth.authenticate(address, async () => "wallet-signature"); });
  await release(await take("/v2/auth/challenge", address));
  await release(await take("/v2/auth/verify", address));
  await act(async () => completion);
}

async function authLateCompletion() {
  await select(A);
  const hook = await mount(() => modules.auth.usePerfdbAuth(), undefined);
  let verify!: Promise<void>;
  await act(async () => { verify = hook.read().authenticate(A, async () => "signature"); });
  await release(await take("/v2/auth/challenge", A));
  const lateVerify = await take("/v2/auth/verify", A);
  await act(async () => hook.read().logout());
  await release(lateVerify); await act(async () => verify);
  assert.equal(window.localStorage.length, 0);
  assert.equal(hook.read().isAuthenticated, false);
  await signIn(hook.read());
  let refresh!: Promise<void>;
  await act(async () => { refresh = hook.read().refresh(); });
  const lateRefresh = await take("/v2/auth/refresh");
  await act(async () => hook.read().logout());
  await release(lateRefresh); await act(async () => refresh);
  assert.equal(window.localStorage.length, 0);
  assert.equal(hook.read().token, null);
  await act(async () => { verify = hook.read().authenticate(A, async () => "signature"); });
  await release(await take("/v2/auth/challenge", A));
  const unmountedVerify = await take("/v2/auth/verify", A);
  await hook.close();
  await release(unmountedVerify); await verify;
  assert.equal(window.localStorage.length, 0);
}
test(authLateCompletion);

async function authWalletBinding() {
  await select(A);
  const first = await mount(() => modules.auth.usePerfdbAuth(), undefined);
  const second = await mount(() => modules.auth.usePerfdbAuth(), undefined);
  await signIn(first.read());
  assert.equal(first.read().address, A);
  assert.equal(second.read().token, first.read().token);
  assert.equal(window.localStorage.getItem("perfdb_jwt_address"), A);
  const referral = await mount(address => modules.referrals.usePerfdbReferrals(address), B as string | undefined);
  await release(await take("/graphql", B)); await until(() => !referral.read().isLoading);
  const competition = await mount(id => modules.competitions.usePerfdbCompetitionDetail(id), "1" as string | null);
  await release(await take("/graphql", "1")); await until(() => !competition.read().isLoading);
  const alerts = await mount(address => modules.alerts.usePerfdbAlerts(address), B as string | undefined);
  await release(await take("/graphql", B));
  await assert.rejects(referral.read().generateCode(), /selected wallet/);
  await assert.rejects(referral.read().applyCode("CODE"), /selected wallet/);
  await assert.rejects(competition.read().join(B), /selected wallet/);
  await assert.rejects(alerts.read().createAlert("price_above", 1, "20"), /selected wallet/);
  await assert.rejects(alerts.read().deleteAlert("1"), /selected wallet/);
  await first.close(); await second.close();
  const reopened = await mount(() => modules.auth.usePerfdbAuth(), undefined);
  assert.equal(reopened.read().address, A);
  assert.equal(reopened.read().isAuthenticated, true);
  await select(B);
  assert.equal(reopened.read().isAuthenticated, false);
  assert.equal(window.localStorage.length, 0);
  await signIn(reopened.read(), B);
  assert.equal(reopened.read().address, B);
  await select(null);
  assert.equal(reopened.read().isAuthenticated, false);
  assert.equal(window.localStorage.length, 0);
  await referral.close(); await competition.close(); await alerts.close(); await reopened.close();
}
test(authWalletBinding);

async function authSupersedingSignIn() {
  await select(A);
  const hook = await mount(() => modules.auth.usePerfdbAuth(), undefined);
  let old!: Promise<void>;
  await act(async () => { old = hook.read().authenticate(A, async () => "signature"); });
  await release(await take("/v2/auth/challenge", A));
  const oldVerify = await take("/v2/auth/verify", A);
  await select(B); await signIn(hook.read(), B);
  const token = hook.read().token;
  await release(oldVerify); await old;
  assert.equal(hook.read().token, token);
  assert.equal(hook.read().address, B);
  await act(async () => hook.read().logout()); await hook.close();
}
test(authSupersedingSignIn);

async function balanceAccountRace() {
  for (const failure of [false, true]) {
    const hook = await mount(address => modules.balances.usePerfdbBalances(address), A as string | undefined);
    const old = await take("/graphql", A);
    await hook.render(B);
    assert.deepEqual(hook.read().balances, []); assert.equal(hook.read().isLoading, true);
    await release(old, failure);
    assert.deepEqual(hook.read().balances, []); assert.equal(hook.read().isLoading, true); assert.equal(hook.read().error, null);
    await release(await take("/graphql", B)); await until(() => !hook.read().isLoading);
    assert.equal(hook.read().balances[0]?.balance, "222");
    await hook.render(A); const disconnected = await take("/graphql", A);
    await hook.render(undefined); await release(disconnected, failure);
    assert.deepEqual(hook.read().balances, []); assert.equal(hook.read().isLoading, false); assert.equal(hook.read().error, null);
    await hook.close();
  }
  for (const failure of [false, true]) {
    const hook = await mount(address => modules.balances.usePerfdbBalances(address), A as string | undefined);
    const old = await take("/graphql", A); await hook.render(B);
    await release(await take("/graphql", B)); await until(() => !hook.read().isLoading);
    await release(old, failure); assert.equal(hook.read().balances[0]?.balance, "222");
    assert.equal(hook.read().error, null); assert.equal(hook.read().isLoading, false); await hook.close();
  }
}
test(balanceAccountRace);

async function referralEarningsOmitted() {
  const hook = await mount(address => modules.referrals.usePerfdbReferrals(address), A as string | undefined);
  await release(await take("/graphql", A)); await until(() => !hook.read().isLoading);
  assert.equal(hook.read().data?.totalEarned, earned);
  assert.ok(queryLog.some(query => /userReferrals[\s\S]*totalEarned/.test(query)));
  await hook.render(B); await release(await take("/graphql", B)); await until(() => !hook.read().isLoading);
  assert.equal(hook.read().data?.totalEarned, null);
  await hook.close();
}
test(referralEarningsOmitted);

async function referralAccountRace() {
  for (const failure of [false, true]) {
    const hook = await mount(address => modules.referrals.usePerfdbReferrals(address), A as string | undefined);
    const old = await take("/graphql", A); await hook.render(B);
    assert.equal(hook.read().data, null); assert.deepEqual(hook.read().tiers, []);
    await release(old, failure); assert.equal(hook.read().isLoading, true); assert.equal(hook.read().error, null);
    await release(await take("/graphql", B)); await until(() => !hook.read().isLoading);
    assert.equal(hook.read().data?.referralCode, "BETA");
    await hook.render(A); const late = await take("/graphql", A);
    await hook.render(undefined); const tiers = await take("/graphql", "ReferralTiers");
    await release(tiers); await until(() => !hook.read().isLoading); await release(late, failure);
    assert.equal(hook.read().data, null); assert.equal(hook.read().error, null); assert.equal(hook.read().isLoading, false);
    await hook.close();
  }
  for (const failure of [false, true]) {
    const hook = await mount(address => modules.referrals.usePerfdbReferrals(address), A as string | undefined);
    const old = await take("/graphql", A); await hook.render(B);
    await release(await take("/graphql", B)); await until(() => !hook.read().isLoading);
    await release(old, failure); assert.equal(hook.read().data?.referralCode, "BETA");
    assert.equal(hook.read().error, null); assert.equal(hook.read().isLoading, false); await hook.close();
  }
}
test(referralAccountRace);

async function referralMutationAccountRace() {
  await select(A);
  const auth = await mount(() => modules.auth.usePerfdbAuth(), undefined); await signIn(auth.read());
  const hook = await mount(address => modules.referrals.usePerfdbReferrals(address), A as string | undefined);
  await release(await take("/graphql", A)); await until(() => !hook.read().isLoading);
  let generated!: Promise<string>;
  await act(async () => { generated = hook.read().generateCode(); });
  const mutation = await take("/v2/referrals/generate");
  assert.equal(mutation.method, "POST");
  assert.deepEqual(mutation.body, {});
  assert.equal(mutation.headers.authorization, `Bearer ${auth.read().token}`);
  await release(mutation); await release(await take("/graphql", A));
  assert.equal(await generated, "GENERATED");
  let applied!: Promise<void>;
  await act(async () => { applied = hook.read().applyCode("  CODE  "); });
  const apply = await take("/v2/referrals/apply"); assert.equal(apply.body.code, "CODE");
  assert.equal(apply.headers.authorization, `Bearer ${auth.read().token}`);
  // Attach the rejection assertion before releasing the superseded mutation.
  const rejected = assert.rejects(applied, /account changed/);
  await select(B); await hook.render(B);
  await release(await take("/graphql", B)); await until(() => !hook.read().isLoading);
  await release(apply); await rejected;
  assert.equal(hook.read().data?.referralCode, "BETA"); assert.equal(hook.read().error, null);
  await hook.close(); await auth.close();
}
test(referralMutationAccountRace);

async function competitionCountZero() {
  const list = await mount(() => modules.competitions.usePerfdbCompetitions(), undefined);
  assert.deepEqual(list.read().competitions, []); assert.equal(list.read().isLoading, true);
  const detail = await mount(id => modules.competitions.usePerfdbCompetitionDetail(id), "1" as string | null);
  await release(await take("/graphql", "Competitions")); await release(await take("/graphql", "1"));
  await until(() => !list.read().isLoading && !detail.read().isLoading);
  assert.equal(list.read().competitions[0]?.participantCount, enrollments.get("1")!.length);
  assert.equal(list.read().competitions[0]?.participantCount, detail.read().detail?.participantCount);
  assert.equal(detail.read().detail?.rankings.length, 2);
  await list.close(); await detail.close();
}
test(competitionCountZero);

async function competitionDetailRace() {
  for (const failure of [false, true]) {
    const hook = await mount(id => modules.competitions.usePerfdbCompetitionDetail(id), "1" as string | null);
    const old = await take("/graphql", "1"); await hook.render("2");
    assert.equal(hook.read().detail, null); await release(old, failure);
    assert.equal(hook.read().isLoading, true); assert.equal(hook.read().error, null);
    await release(await take("/graphql", "2")); await until(() => !hook.read().isLoading);
    assert.equal(hook.read().detail?.id, "2");
    await hook.render("1"); const late = await take("/graphql", "1");
    await hook.render("2"); await release(await take("/graphql", "2")); await until(() => !hook.read().isLoading);
    await release(late, failure); assert.equal(hook.read().detail?.id, "2"); assert.equal(hook.read().error, null);
    await hook.render("1"); const cleared = await take("/graphql", "1");
    await hook.render(null); await release(cleared, failure);
    assert.equal(hook.read().detail, null); assert.equal(hook.read().isLoading, false); assert.equal(hook.read().error, null); await hook.close();
  }
  await select(A); const auth = await mount(() => modules.auth.usePerfdbAuth(), undefined); await signIn(auth.read());
  const hook = await mount(id => modules.competitions.usePerfdbCompetitionDetail(id), "1" as string | null);
  await release(await take("/graphql", "1")); await until(() => !hook.read().isLoading);
  let joined!: Promise<void>;
  await act(async () => { joined = hook.read().join(A); });
  await release(await take("/v2/competitions/1/join"));
  const refresh = await take("/graphql", "1"); await hook.render(null); await release(refresh); await joined;
  assert.equal(hook.read().detail, null); assert.equal(hook.read().error, null); assert.equal(hook.read().isLoading, false);
  await hook.close(); await act(async () => auth.read().logout()); await auth.close();
}
test(competitionDetailRace);

async function perfdbSelectionConnection() {
  const hook = await mount(marketId => ({ price: modules.subscriptions.usePriceSubscription(marketId), trade: modules.subscriptions.useTradeSubscription(marketId) }), 1);
  await until(() => peers.some(peer => peer.subscriptions.size === 2));
  const peer = peers.find(candidate => candidate.subscriptions.size === 2)!;
  peer.publish(1); await until(() => hook.read().price.latestUpdate !== null && hook.read().trade.latestTrade !== null);
  assert.equal(hook.read().price.latestUpdate?.marketId, 1);
  assert.equal(hook.read().trade.latestTrade?.marketId, 1);
  // Until core/graphql/client exposes onStatus, transport is explicitly
  // unavailable, rather than inventing connected=true from a data frame.
  assert.equal(hook.read().price.connectionStatus, "unavailable");
  assert.equal(hook.read().trade.connected, false);
  await hook.render(2);
  assert.equal(hook.read().price.latestUpdate, null); assert.equal(hook.read().trade.latestTrade, null);
  assert.equal(hook.read().price.connected, false);
  await until(() => peers.some(candidate => [...candidate.subscriptions.values()].filter(sub => sub.variables?.marketId === 2).length === 2));
  for (const candidate of peers) if (!candidate.socket.destroyed) candidate.publish(2);
  await until(() => hook.read().price.latestUpdate?.marketId === 2 && hook.read().trade.latestTrade?.marketId === 2);
  for (const candidate of peers) if (!candidate.socket.destroyed) candidate.close();
  await until(() => peers.every(candidate => candidate.socket.destroyed));
  assert.equal(hook.read().price.connected, false); assert.equal(hook.read().trade.connected, false);
  await hook.close();
}
test(perfdbSelectionConnection);
