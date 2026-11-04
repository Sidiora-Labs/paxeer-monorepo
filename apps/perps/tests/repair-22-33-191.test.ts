import assert from "node:assert/strict";
import { createServer } from "node:http";
import type { AddressInfo } from "node:net";
import { test } from "node:test";
import { BroadcastChannel } from "node:worker_threads";
import { isValidElement, type ReactElement, type ReactNode } from "react";
import { renderToStaticMarkup } from "react-dom/server";

process.env.NEXT_PUBLIC_PERPS_GATEWAY_URL = "https://gateway.fixture.test";
process.env.NEXT_PUBLIC_PAXSCAN_BASE_URL = "https://explorer.fixture.test";
process.env.NEXT_PUBLIC_PAXSCAN_API_URL = "https://explorer.fixture.test/api";
process.env.NEXT_PUBLIC_PAXSCAN_WS_URL = "wss://explorer.fixture.test/socket";

class SessionStorage implements Storage {
  private readonly values = new Map<string, string>();
  readonly reads: string[] = [];
  readFailure: { key: string; value: unknown; beforeThrow?: () => void } | null = null;
  failSessionAfterWrite = false;
  onSessionReadFailure?: () => void;

  constructor(readonly sessionKey: string) {}

  get length(): number {
    return this.values.size;
  }

  clear(): void {
    this.values.clear();
  }

  key(index: number): string | null {
    return [...this.values.keys()][index] ?? null;
  }

  getItem(key: string): string | null {
    this.reads.push(key);
    if (this.readFailure?.key === key) {
      this.readFailure.beforeThrow?.();
      throw this.readFailure.value;
    }
    return this.values.get(key) ?? null;
  }

  removeItem(key: string): void {
    this.values.delete(key);
  }

  setItem(key: string, value: string): void {
    this.values.set(key, value);
    if (this.failSessionAfterWrite && key === this.sessionKey) {
      this.readFailure = {
        key,
        value: new Error("Session storage became unavailable"),
        beforeThrow: this.onSessionReadFailure,
      };
    }
  }
}

function deferred() {
  let resolve!: () => void;
  const promise = new Promise<void>((done) => {
    resolve = done;
  });
  return { promise, resolve };
}

function elements(node: ReactNode): ReactElement<Record<string, unknown>>[] {
  if (Array.isArray(node)) return node.flatMap(elements);
  if (!isValidElement<Record<string, unknown>>(node)) return [];
  return [node, ...elements(node.props.children as ReactNode)];
}

async function authFixture() {
  const { finishAuthCallback, retryAuthCallback, AuthCallbackContent } = await import("../src/pages/auth/callback");
  const { PaxeerWallet } = await import("../src/core/wallet/embedded/sdk");
  const { useWalletStore } = await import("../src/hooks/wallet/useWalletStore");
  const requests: Array<{ path: string; body: unknown }> = [];
  const tokenReceived = deferred();
  const releaseToken = deferred();
  let holdToken = false;
  const server = createServer((request, response) => {
    void (async () => {
      let body = "";
      for await (const chunk of request) body += chunk;
      requests.push({ path: request.url ?? "", body: body ? JSON.parse(body) : null });
      if (request.method !== "POST" || request.url !== "/auth/v1/token?grant_type=pkce") {
        response.writeHead(404).end();
        return;
      }
      tokenReceived.resolve();
      if (holdToken) await releaseToken.promise;
      // Loopback Supabase token producer with a current session expiry.
      const expiresAt = Math.floor(Date.now() / 1000) + 3600;
      const token = [
        { alg: "HS256", typ: "JWT" },
        { sub: "00000000-0000-4000-8000-000000000001", aud: "authenticated", exp: expiresAt },
        "fixture-signature",
      ].map((part) => Buffer.from(JSON.stringify(part)).toString("base64url")).join(".");
      response.writeHead(200, { "Content-Type": "application/json" });
      response.end(JSON.stringify({
        access_token: token,
        refresh_token: "fixture-refresh-token",
        token_type: "bearer",
        expires_in: 3600,
        expires_at: expiresAt,
        user: {
          id: "00000000-0000-4000-8000-000000000001",
          aud: "authenticated",
          role: "authenticated",
          email: "trader@fixture.test",
          app_metadata: { provider: "github", providers: ["github"] },
          user_metadata: {},
          created_at: "2024-01-01T00:00:00Z",
        },
      }));
    })().catch((error: unknown) => {
      response.destroy(error instanceof Error ? error : new Error(String(error)));
    });
  });
  await new Promise<void>((resolve) => server.listen(0, "127.0.0.1", resolve));
  const origin = `http://127.0.0.1:${(server.address() as AddressInfo).port}`;
  const storage = new SessionStorage("sb-127-auth-token");
  const channels: BroadcastChannel[] = [];
  class LocalBroadcastChannel extends BroadcastChannel {
    constructor(name: string) {
      super(name);
      channels.push(this);
    }
  }
  const globals = {
    window: Object.assign(new EventTarget(), { location: new URL(origin), sessionStorage: storage }),
    document: Object.assign(new EventTarget(), { visibilityState: "hidden" }),
    localStorage: storage,
    BroadcastChannel: LocalBroadcastChannel,
  };
  const previous = new Map(Object.keys(globals).map((key) => [key, Object.getOwnPropertyDescriptor(globalThis, key)]));
  for (const [key, value] of Object.entries(globals)) {
    Object.defineProperty(globalThis, key, { configurable: true, writable: true, value });
  }
  const wallet = new PaxeerWallet({ apiUrl: origin, supabaseUrl: origin, supabaseAnonKey: "fixture-publishable-key" });
  await wallet.supabase.auth.initialize();
  assert.equal(await wallet.getSession(), null);
  await wallet.supabase.auth.stopAutoRefresh();
  const state = { error: null as string | null, redirects: 0, cancelled: false, path: "/auth/callback?code=old-code" };

  return {
    wallet,
    storage,
    state,
    requests,
    tokenReceived,
    releaseToken,
    retry: retryAuthCallback,
    walletStore: useWalletStore,
    holdExchange() { holdToken = true; },
    async startSignIn() {
      const { data, error } = await wallet.supabase.auth.signInWithOAuth({
        provider: "github",
        options: { skipBrowserRedirect: true, redirectTo: `${origin}/auth/callback` },
      });
      assert.equal(error, null);
      assert.ok(data.url);
      assert.ok(new URL(data.url).searchParams.get("code_challenge"));
      assert.ok(storage.getItem(`${storage.sessionKey}-code-verifier`));
    },
    finish(search = "?code=fixture-authorization-code") {
      return finishAuthCallback(wallet, {
        search,
        isCancelled: () => state.cancelled,
        onError: (message) => { state.error = message; },
        onSuccess: async () => { state.redirects += 1; },
      });
    },
    async assertRecoverable(message: string) {
      assert.equal(state.error, message);
      assert.equal(state.redirects, 0);
      const view = AuthCallbackContent({
        error: state.error,
        onRetry: () => retryAuthCallback(async () => {
          state.path = "/";
          return true;
        }),
      });
      const html = renderToStaticMarkup(view);
      assert.match(html, /Sign-in failed/);
      assert.match(html, /Sign in again/);
      assert.doesNotMatch(html, /Finishing sign-in|animate-spin/);
      const retry = elements(view).find((node) => node.type === "button");
      assert.ok(retry);
      assert.equal(retry.props.type, "button");
      assert.equal(typeof retry.props.onClick, "function");
      useWalletStore.getState().closeConnectModal();
      assert.equal(useWalletStore.getState().connectModalOpen, false);
      if (typeof retry.props.onClick === "function") await retry.props.onClick();
      assert.equal(state.path, "/");
      assert.equal(useWalletStore.getState().connectModalOpen, true);
      useWalletStore.getState().closeConnectModal();
    },
    async close() {
      useWalletStore.getState().closeConnectModal();
      storage.readFailure = null;
      await wallet.supabase.auth.stopAutoRefresh();
      for (const channel of channels) channel.close();
      for (const [key, descriptor] of previous) {
        if (descriptor) Object.defineProperty(globalThis, key, descriptor);
        else Reflect.deleteProperty(globalThis, key);
      }
      releaseToken.resolve();
      server.closeAllConnections();
      await new Promise<void>((resolve, reject) => server.close((error) => error ? reject(error) : resolve()));
    },
  };
}

async function f007ExchangeRejectionShowsRetry() {
  const fixture = await authFixture();
  try {
    await fixture.startSignIn();
    fixture.storage.readFailure = {
      key: `${fixture.storage.sessionKey}-code-verifier`,
      value: new Error("Authorization storage became unavailable"),
    };
    await fixture.finish();
    await fixture.assertRecoverable("Authorization storage became unavailable");
    assert.deepEqual(fixture.requests, []);
  } finally {
    await fixture.close();
  }
}

async function f007SessionRejectionShowsRetry() {
  const fixture = await authFixture();
  try {
    await fixture.startSignIn();
    fixture.storage.failSessionAfterWrite = true;
    await fixture.finish();
    await fixture.assertRecoverable("Session storage became unavailable");
    assert.equal(fixture.requests.length, 1);
    assert.equal(fixture.requests[0].path, "/auth/v1/token?grant_type=pkce");
    assert.ok(fixture.storage.reads.includes(fixture.storage.sessionKey));
  } finally {
    await fixture.close();
  }
}

async function f007SuccessfulExchangeUsesRealSession() {
  const fixture = await authFixture();
  try {
    await fixture.startSignIn();
    const verifier = JSON.parse(fixture.storage.getItem(`${fixture.storage.sessionKey}-code-verifier`)!);
    await fixture.finish();
    assert.equal(fixture.state.error, null);
    assert.equal(fixture.state.redirects, 1);
    assert.deepEqual(fixture.requests, [{
      path: "/auth/v1/token?grant_type=pkce",
      body: { auth_code: "fixture-authorization-code", code_verifier: verifier.split("/")[0] },
    }]);
    assert.equal((await fixture.wallet.getSession())?.user.email, "trader@fixture.test");
    assert.equal(fixture.storage.getItem(`${fixture.storage.sessionKey}-code-verifier`), null);
  } finally {
    await fixture.close();
  }
}

async function f007CancellationSuppressesLateSuccess() {
  const fixture = await authFixture();
  try {
    await fixture.startSignIn();
    fixture.holdExchange();
    const finishing = fixture.finish();
    await fixture.tokenReceived.promise;
    fixture.state.cancelled = true;
    fixture.releaseToken.resolve();
    await finishing;
    assert.equal(fixture.state.error, null);
    assert.equal(fixture.state.redirects, 0);
    assert.equal(fixture.requests.length, 1);
    assert.ok(await fixture.wallet.getSession(), "the exchange completed after cancellation");
  } finally {
    await fixture.close();
  }
}

async function f007CancellationSuppressesExchangeAndSessionRejections() {
  for (const phase of ["exchange", "session"] as const) {
    const fixture = await authFixture();
    try {
      await fixture.startSignIn();
      if (phase === "exchange") {
        fixture.storage.readFailure = {
          key: `${fixture.storage.sessionKey}-code-verifier`,
          value: new Error("Cancelled exchange failure"),
          beforeThrow: () => { fixture.state.cancelled = true; },
        };
      } else {
        fixture.storage.failSessionAfterWrite = true;
        fixture.storage.onSessionReadFailure = () => { fixture.state.cancelled = true; };
      }
      await fixture.finish();
      assert.equal(fixture.state.cancelled, true);
      assert.equal(fixture.state.error, null);
      assert.equal(fixture.state.redirects, 0);
      assert.equal(fixture.requests.length, phase === "session" ? 1 : 0);
    } finally {
      await fixture.close();
    }
  }
}

async function f007MissingCodeAndReturnedErrorsRecover() {
  const fixture = await authFixture();
  try {
    await fixture.finish("");
    await fixture.assertRecoverable("Sign-in link is missing its authorization code. Please sign in again.");
    await fixture.finish();
    await fixture.assertRecoverable(
      "PKCE code verifier not found in storage. " +
      "This can happen if the auth flow was initiated in a different browser or device, " +
      "or if the storage was cleared. For SSR frameworks (Next.js, SvelteKit, etc.), " +
      "use @supabase/ssr on both the server and client to store the code verifier in cookies.",
    );
    assert.deepEqual(fixture.requests, []);
  } finally {
    await fixture.close();
  }
}

async function f007NonErrorRejectionRecovers() {
  const fixture = await authFixture();
  try {
    await fixture.startSignIn();
    fixture.storage.readFailure = { key: `${fixture.storage.sessionKey}-code-verifier`, value: "storage denied" };
    await fixture.finish();
    await fixture.assertRecoverable("Sign-in did not complete. Please sign in again.");
  } finally {
    await fixture.close();
  }
}

async function f007RetryWaitsForHomeNavigation() {
  const fixture = await authFixture();
  try {
    const homeNavigation = deferred();
    const retrying = fixture.retry(async () => {
      await homeNavigation.promise;
      fixture.state.path = "/";
      return true;
    });
    assert.equal(fixture.walletStore.getState().connectModalOpen, false);
    assert.equal(fixture.state.path, "/auth/callback?code=old-code");
    homeNavigation.resolve();
    await retrying;
    assert.equal(fixture.state.path, "/");
    assert.equal(fixture.walletStore.getState().connectModalOpen, true);

    fixture.walletStore.getState().closeConnectModal();
    await fixture.retry(async () => false);
    assert.equal(fixture.walletStore.getState().connectModalOpen, false);
    await assert.rejects(fixture.retry(async () => { throw new Error("Navigation failed"); }), /Navigation failed/);
    assert.equal(fixture.walletStore.getState().connectModalOpen, false);
  } finally {
    await fixture.close();
  }
}

test(f007ExchangeRejectionShowsRetry);
test(f007SessionRejectionShowsRetry);
test(f007SuccessfulExchangeUsesRealSession);
test(f007CancellationSuppressesLateSuccess);
test(f007CancellationSuppressesExchangeAndSessionRejections);
test(f007MissingCodeAndReturnedErrorsRecover);
test(f007NonErrorRejectionRecovers);
test(f007RetryWaitsForHomeNavigation);
