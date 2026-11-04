import assert from "node:assert/strict";
import { getEventListeners } from "node:events";
import { test, type TestContext } from "node:test";

process.env.NEXT_PUBLIC_PERPS_GATEWAY_URL = "https://gateway.fixture.test";
process.env.NEXT_PUBLIC_PAXSCAN_BASE_URL = "https://explorer.fixture.test";
process.env.NEXT_PUBLIC_PAXSCAN_API_URL = "https://explorer.fixture.test/api/v2";
process.env.NEXT_PUBLIC_PAXSCAN_WS_URL = "wss://explorer.fixture.test";
process.env.NEXT_PUBLIC_WALLET_IFRAME_ORIGIN = "https://wallet.fixture.test";

const walletOrigin = "https://wallet.fixture.test";
const frameOrigin = "https://trading.fixture.test";
const accounts = ["0x1111111111111111111111111111111111111111"] as const;
type WalletModule = typeof import("../src/core/wallet/postMessageEip1193");
type RequestMessage = { type: "SIDIORA_EIP1193_REQUEST"; id: string; method: string; params?: unknown[] | object };

// Committed in-memory peers use native structured cloning and asynchronous
// MessagePort transport. Each connection fixes the sender identity and origin;
// message data cannot choose either value.
class WindowMessagePeer {
  readonly source: MessagePort;
  readonly target: MessagePort;
  readonly requests: RequestMessage[] = [];
  private readonly queued: RequestMessage[] = [];
  private readonly waiting: Array<(request: RequestMessage) => void> = [];

  constructor(
    frame: EventTarget,
    readonly origin: string,
  ) {
    const channel = new MessageChannel();
    this.source = channel.port1;
    this.target = channel.port2;
    const toPeer = this.source.postMessage.bind(this.source);
    const toFrame = this.target.postMessage.bind(this.target);
    // Window target-origin routing over the native loopback ports.
    Object.defineProperty(this.source, "postMessage", {
      value: (data: unknown, targetOrigin: string) => {
        if (targetOrigin === this.origin) toPeer(data);
      },
    });
    Object.defineProperty(this.target, "postMessage", {
      value: (data: unknown, targetOrigin: string) => {
        if (targetOrigin === frameOrigin) toFrame(data);
      },
    });
    this.target.addEventListener("message", ({ data: request }: MessageEvent<RequestMessage>) => {
      assert.equal(request.type, "SIDIORA_EIP1193_REQUEST");
      assert.equal(typeof request.id, "string");
      this.requests.push(request);
      const receive = this.waiting.shift();
      if (receive) receive(request);
      else this.queued.push(request);
    });
    this.source.addEventListener("message", ({ data }: MessageEvent<unknown>) => {
      frame.dispatchEvent(new MessageEvent("message", { data, origin: this.origin, source: this.source }));
    });
    this.source.start();
    this.target.start();
  }

  nextRequest(): Promise<RequestMessage> {
    const queued = this.queued.shift();
    if (queued) return Promise.resolve(queued);
    return new Promise((resolve, reject) => {
      const timeout = setTimeout(() => reject(new Error("Parent did not receive the provider request")), 2000);
      this.waiting.push((request) => {
        clearTimeout(timeout);
        resolve(request);
      });
    });
  }

  async deliver(data: unknown): Promise<void> {
    const delivered = new Promise<void>((resolve) => this.source.addEventListener("message", () => resolve(), { once: true }));
    // The target is a native MessagePort with Window's targetOrigin convention.
    (this.target as unknown as Window).postMessage(data, frameOrigin);
    await delivered;
  }

  async respond(request: RequestMessage, result: unknown): Promise<void> {
    await this.deliver({ type: "SIDIORA_EIP1193_RESPONSE", id: request.id, result });
  }

  async event(event: string, payload: unknown): Promise<void> {
    await this.deliver({ type: "SIDIORA_EIP1193_EVENT", event, payload });
  }

  close(): void {
    this.source.close();
    this.target.close();
  }
}

async function mountedFixture(t: TestContext) {
  const module = await import("../src/core/wallet/postMessageEip1193");
  const frame = new EventTarget();
  const parent = new WindowMessagePeer(frame, walletOrigin);
  const sibling = new WindowMessagePeer(frame, walletOrigin);
  const foreign = new WindowMessagePeer(frame, "https://other.fixture.test");
  const windowDescriptor = Object.getOwnPropertyDescriptor(globalThis, "window");
  const frameWindow = Object.assign(frame, { parent: parent.source, ethereum: undefined });
  Object.defineProperty(globalThis, "window", { configurable: true, value: frameWindow });
  const provider = module.installPostMessageEip1193Provider();
  assert.ok(provider);
  assert.equal(frameWindow.ethereum, provider);
  t.after(() => {
    provider.unmount();
    parent.close();
    sibling.close();
    foreign.close();
    if (windowDescriptor) Object.defineProperty(globalThis, "window", windowDescriptor);
    else Reflect.deleteProperty(globalThis, "window");
  });
  return { module, provider, frame, frameWindow, parent, sibling, foreign };
}

async function assertAccountsFromPeer(module: WalletModule, peer: WindowMessagePeer, values: readonly `0x${string}`[]): Promise<void> {
  const delivered = new Promise<void>((resolve) => peer.source.addEventListener("message", () => resolve(), { once: true }));
  module.postAccountsChangedEvent(peer.target as unknown as Window, frameOrigin, [...values]);
  await delivered;
}

async function iframePendingRequestsExpireAndRecover(t: TestContext) {
  const { module, provider, parent } = await mountedFixture(t);
  assert.equal(module.POST_MESSAGE_REQUEST_TIMEOUT_MS, 30_000);
  const started = Date.now();
  const lost = provider.request({ method: "eth_accounts" });
  const expired = assert.rejects(lost, { code: 4900, message: /timed out.*retry/ });
  const request = await parent.nextRequest();
  assert.equal(request.method, "eth_accounts");
  let watchdog: ReturnType<typeof setTimeout> | undefined;
  try {
    await Promise.race([
      expired,
      new Promise<never>((_, reject) => {
        watchdog = setTimeout(() => reject(new Error("Lost wallet response left the request unresolved")), module.POST_MESSAGE_REQUEST_TIMEOUT_MS + 1000);
      }),
    ]);
  } finally {
    clearTimeout(watchdog);
  }
  assert.ok(Date.now() - started >= module.POST_MESSAGE_REQUEST_TIMEOUT_MS, "deadline must not reject early");
  await parent.respond(request, ["late response"]);
  const retry = provider.request({ method: "eth_accounts" });
  const retried = await parent.nextRequest();
  assert.notEqual(retried.id, request.id);
  await parent.respond(request, ["stale response during retry"]);
  await parent.respond(retried, accounts);
  assert.deepEqual(await retry, accounts);
}

async function iframeParentReloadAndUnmountRecover(t: TestContext) {
  const { module, provider, frame, parent, sibling } = await mountedFixture(t);
  provider.mount();
  assert.equal(getEventListeners(frame, "message").length, 1);
  assert.equal(getEventListeners(frame, "pagehide").length, 1);
  const pending = provider.request({ method: "eth_sendTransaction", params: [{ from: accounts[0] }] });
  let interruptedSettled = false;
  void pending.then(
    () => (interruptedSettled = true),
    () => (interruptedSettled = true),
  );
  const pendingRejected = assert.rejects(pending, { code: 4900, message: /reconnected.*retry/ });
  const interrupted = await parent.nextRequest();
  await sibling.event("connect", { chainId: "0x7d" });
  assert.equal(interruptedSettled, false, "a sibling reconnect must not invalidate the parent's pending request");
  await parent.event("connect", { chainId: "0x7d" });
  await pendingRejected;
  const recovered = provider.request({ method: "eth_chainId" });
  const recoveredRequest = await parent.nextRequest();
  await parent.respond(interrupted, "stale signature");
  await parent.respond(recoveredRequest, "0x7d");
  assert.equal(await recovered, "0x7d");

  const disconnecting = provider.request({ method: "eth_accounts" });
  const disconnected = assert.rejects(disconnecting, { code: 4900, message: /disconnected.*retry/ });
  await parent.nextRequest();
  await parent.event("disconnect", { code: 4900, message: "Parent is reloading" });
  await disconnected;
  const sentBeforeDisconnectedRequest = parent.requests.length;
  await assert.rejects(provider.request({ method: "eth_accounts" }), { code: 4900 });
  assert.equal(parent.requests.length, sentBeforeDisconnectedRequest);
  await parent.event("connect", { chainId: "0x7d" });

  const unmounting = [provider.request({ method: "eth_accounts" }), provider.request({ method: "eth_chainId" })];
  const unmounted = unmounting.map((request) => assert.rejects(request, { code: 4900, message: /unmounted.*retry/ }));
  const beforeUnmount = [await parent.nextRequest(), await parent.nextRequest()];
  frame.dispatchEvent(new Event("pagehide"));
  await Promise.all(unmounted);
  assert.equal(getEventListeners(frame, "message").length, 0);
  assert.equal(getEventListeners(frame, "pagehide").length, 0);
  await assert.rejects(provider.request({ method: "eth_accounts" }), { code: 4900 });

  assert.equal(module.installPostMessageEip1193Provider(), provider, "installation remounts the retained provider after page restoration");
  const reopened = provider.request({ method: "eth_accounts" });
  const reopenedRequest = await parent.nextRequest();
  for (const old of beforeUnmount) await parent.respond(old, "stale result from the old session");
  await parent.respond(reopenedRequest, accounts);
  assert.deepEqual(await reopened, accounts);
  await assert.rejects(provider.request({ method: "eth_sendTransaction", params: [{ unserializable: () => undefined }] }), { name: "DataCloneError" });
  provider.unmount();
  provider.unmount();
  assert.equal(getEventListeners(frame, "message").length, 0);
}

async function iframeOnlyParentCanDeliverValidatedResponsesAndEvents(t: TestContext) {
  const { module, provider, parent, sibling, foreign, frameWindow } = await mountedFixture(t);
  const events: unknown[][] = [];
  provider.on("accountsChanged", (...args) => events.push(args));
  await assertAccountsFromPeer(module, sibling, accounts);
  await assertAccountsFromPeer(module, foreign, accounts);
  assert.deepEqual(events, [], "same-origin sibling and other origins cannot alter accounts");
  await assertAccountsFromPeer(module, parent, accounts);
  await assertAccountsFromPeer(module, parent, []);
  assert.deepEqual(events, [[accounts], [[]]], "the intended parent preserves the full accounts array");

  let settlements = 0;
  const pending = provider.request({ method: "eth_accounts" });
  void pending.then(
    () => settlements++,
    () => settlements++,
  );
  const request = await parent.nextRequest();
  await sibling.respond(request, "forged sibling result");
  await foreign.respond(request, "forged foreign result");
  // Keep the real parent's source but change its origin to prove both checks.
  frameWindow.parent = foreign.source;
  await foreign.respond(request, "correct source, wrong origin");
  frameWindow.parent = parent.source;
  assert.equal(settlements, 0);

  const malformed: unknown[] = [
    null,
    [],
    "response",
    { type: "SIDIORA_EIP1193_RESPONSE", id: request.id },
    { type: "SIDIORA_EIP1193_RESPONSE", id: request.id, result: "ambiguous", error: { code: 4001, message: "refused" } },
    { type: "SIDIORA_EIP1193_RESPONSE", id: request.id, error: null },
    { type: "SIDIORA_EIP1193_RESPONSE", id: request.id, error: { code: "4001", message: "refused" } },
    { type: "SIDIORA_EIP1193_RESPONSE", id: request.id, error: { code: 4001 } },
    { type: "SIDIORA_EIP1193_RESPONSE", id: 42, result: "wrong id type" },
    { type: "SIDIORA_EIP1193_RESPONSE", id: "", result: "empty id" },
    { type: "SIDIORA_EIP1193_EVENT", event: "accountsChanged", payload: ["0x1234"] },
    { type: "SIDIORA_EIP1193_EVENT", event: "accountsChanged", payload: accounts[0] },
    { type: "SIDIORA_EIP1193_EVENT", event: "connect", payload: { chainId: 125 } },
    { type: "SIDIORA_EIP1193_EVENT", event: "disconnect", payload: { code: "4900", message: "wrong type" } },
    { type: "SIDIORA_EIP1193_EVENT", event: "chainChanged", payload: "125" },
    { type: "SIDIORA_EIP1193_EVENT", event: "message", payload: { type: "eth_subscription" } },
    { type: "SIDIORA_EIP1193_EVENT", event: "unknown", payload: [] },
  ];
  const otherEvents: unknown[][] = [];
  for (const name of ["connect", "disconnect", "chainChanged", "message", "unknown"]) {
    provider.on(name, (...args) => otherEvents.push([name, ...args]));
  }
  for (const message of malformed) await parent.deliver(message);
  assert.equal(settlements, 0, "malformed or unauthorized replies must leave the real request pending");
  assert.deepEqual(events, [[accounts], [[]]]);
  assert.deepEqual(otherEvents, []);
  await parent.respond(request, accounts);
  assert.deepEqual(await pending, accounts);
  assert.equal(settlements, 1);

  const errorData = { reason: "User declined", details: [1, 2] };
  const refused = provider.request({ method: "personal_sign", params: ["0xab", accounts[0]] });
  const refusal = assert.rejects(refused, { code: 4001, message: "User rejected request", data: errorData });
  const refusedRequest = await parent.nextRequest();
  assert.deepEqual(refusedRequest.params, ["0xab", accounts[0]]);
  await parent.deliver({ type: "SIDIORA_EIP1193_RESPONSE", id: refusedRequest.id, error: { code: 4001, message: "User rejected request", data: errorData } });
  await refusal;
  await parent.event("chainChanged", "0x7d");
  await parent.event("message", { type: "eth_subscription", data: { result: "0x1" } });
  await parent.event("connect", { chainId: "0x7d" });
  await parent.event("disconnect", { code: 4900, message: "Closed" });
  assert.deepEqual(otherEvents, [
    ["chainChanged", "0x7d"],
    ["message", { type: "eth_subscription", data: { result: "0x1" } }],
    ["connect", { chainId: "0x7d" }],
    ["disconnect", { code: 4900, message: "Closed" }],
  ]);
}

test(iframePendingRequestsExpireAndRecover);
test(iframeParentReloadAndUnmountRecover);
test(iframeOnlyParentCanDeliverValidatedResponsesAndEvents);
