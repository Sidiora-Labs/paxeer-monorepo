import assert from "node:assert/strict";
import { existsSync, readFileSync, readdirSync } from "node:fs";
import { mkdtemp, rm } from "node:fs/promises";
import { createServer } from "node:https";
import { registerHooks } from "node:module";
import { tmpdir } from "node:os";
import { dirname, join, relative } from "node:path";
import test, { type TestContext } from "node:test";
import { getCACertificates, setDefaultCACertificates } from "node:tls";
import { fileURLToPath } from "node:url";
import ts from "typescript";
import { Window } from "happy-dom";

registerHooks({
  resolve(specifier, context, nextResolve) {
    if (specifier === "server-only") return nextResolve("next/dist/compiled/server-only/empty.js", context);
    if (specifier.startsWith("@layerx/ui/")) {
      const path = specifier === "@layerx/ui/cn" ? "lib/utils" : specifier.slice("@layerx/ui/".length);
      const source = new URL(`../packages/layerx-ui/src/${path}`, import.meta.url);
      for (const suffix of [".ts", ".tsx"]) {
        const candidate = new URL(source.href + suffix);
        if (existsSync(candidate)) return nextResolve(candidate.href, context);
      }
    }
    if (specifier.startsWith(".") && context.parentURL !== undefined) {
      const candidate = new URL(specifier, context.parentURL);
      for (const suffix of [".ts", ".tsx", "/index.ts"]) {
        const source = candidate.href.endsWith(".js")
          ? new URL(candidate.href.slice(0, -3) + suffix) : new URL(candidate.href + suffix);
        if (existsSync(source)) return nextResolve(source.href, context);
      }
    }
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

const { createElement, act, StrictMode } = await import("react");
const { createRoot } = await import("react-dom/client");
const { renderToStaticMarkup } = await import("react-dom/server");
const { CURRENT_ROUTES, CURRENT_SHELLS, REQUIRED_SCREEN_STATES, StateMatrix, humanStateMatrix } = await import("../src/states/matrix.ts");
const { MatrixStateSurface } = await import("../src/states/matrix-surface.tsx");
const { errorPresentation, ErrorSurface } = await import("../src/states/error.tsx");
const { SUPPORT_REPORT_ROUTES, supportReportRoute } = await import("../src/states/report-routes.ts");
const { BrowserSupportReportOutbox, SupportReportPrincipalError, requireSupportReportPrincipal, SUPPORT_REPORT_PRINCIPAL_HEADER, redactErrorReport } = await import("../src/states/report.ts");
const { SupportReportSessionBoundary, useSupportReportOutbox } = await import("../src/states/report-session.tsx");
const { encodeAccountBalance, encodeSessionList } = await import("../src/api/generated/index.ts");
const { parseSupportReportRequest } = await import("../src/states/report-schema.ts");
const { verifiedWebSession } = await import("../src/auth/server-session.ts");
const { SupportReportRepository } = await import("../src/server/support-reports.ts");
const { POST } = await import("../src/app/api/support/reports/route.ts");

type Session = NonNullable<Awaited<ReturnType<typeof verifiedWebSession>>>;
type Outbox = InstanceType<typeof BrowserSupportReportOutbox>;
const appRoot = fileURLToPath(new URL("../src/app/", import.meta.url));
const draft = Object.freeze({ consented: true, traceId: "trc_principal_a", machineCode: "UI_UNHANDLED", route: "/app/agents/private-agent", shell: "desktop" as const });

function livePageRoutes(directory = appRoot): string[] {
  return readdirSync(directory, { withFileTypes: true }).flatMap((entry) => {
    if (entry.isDirectory() && !entry.name.startsWith("_") && !entry.name.startsWith("@")) {
      return livePageRoutes(join(directory, entry.name));
    }
    if (!entry.isFile() || entry.name !== "page.tsx") return [];
    const segments = relative(appRoot, directory).split("/").filter((segment) => segment !== "" && !segment.startsWith("("));
    return ["/" + segments.join("/")];
  });
}

function state_matrix_stale_inventory_matches_live_routes(): void {
  assert.deepEqual([...CURRENT_ROUTES].sort(), livePageRoutes().sort());
  assert.equal(SUPPORT_REPORT_ROUTES, CURRENT_ROUTES);
  const matrix = humanStateMatrix();
  matrix.assertComplete(livePageRoutes(), CURRENT_SHELLS);
  for (const entry of matrix.enumerate().filter((target) => target.kind === "route")) {
    assert.throws(() => new StateMatrix(matrix.enumerate().filter((target) => target !== entry)).assertComplete(livePageRoutes()), /no state-matrix registration/u);
  }
  for (const retired of ["/explorer/checkpoints", "/explorer/checkpoints/[checkpointId]", "/explorer/batches", "/explorer/batches/[batchNumber]"]) {
    const exemplar = matrix.enumerate()[0];
    assert.ok(exemplar);
    assert.throws(() => new StateMatrix([...matrix.enumerate(), { ...exemplar, id: "retired", route: retired }]).assertComplete(livePageRoutes()), /not a current route/u);
    assert.throws(() => supportReportRoute(retired), /Invalid report route/u);
  }
  const exemplar = matrix.enumerate()[0];
  assert.ok(exemplar);
  assert.throws(() => new StateMatrix([...matrix.enumerate(), { ...exemplar, id: "duplicate" }]).assertComplete(), /duplicate state registrations/u);
  assert.throws(() => new StateMatrix([{ ...exemplar, shells: ["desktop"] }]).assertComplete(), /both shells/u);
}

async function state_matrix_real_surfaces_both_shells(): Promise<void> {
  const error = errorPresentation({ code: "UI_UNHANDLED" }, "trc_state_matrix");
  const callback = () => { };
  for (const route of livePageRoutes()) {
    for (const shell of ["mobile", "desktop"] as const) {
      const states = [
        { state: "loading" as const },
        { state: "empty" as const },
        { state: "error" as const, error, onRetry: callback, onReload: callback },
        { state: "offline" as const, onRetry: callback },
        { state: "degraded" as const, age: "2 minutes", referencePoint: "recorded head 42", onReload: callback },
        { state: "still-checking" as const, lookupOutcome: async () => "pending" as const, onResolved: callback },
      ];
      assert.deepEqual(states.map((state) => state.state), [...REQUIRED_SCREEN_STATES]);
      for (const state of states) {
        const markup = renderToStaticMarkup(createElement(MatrixStateSurface, { route, shell, ...state }));
        assert.ok(markup.length > 0, `${route} ${shell} ${state.state}`);
        if (state.state === "error") {
          assert.ok(markup.includes('role="alert"'));
          assert.ok(markup.includes(error.traceId));
        } else if (state.state !== "empty") assert.ok(markup.includes('role="status"'));
        if (state.state === "still-checking") {
          assert.ok(!markup.includes("<button"), "pending outcomes cannot submit duplicate actions");
          assert.ok(markup.includes('aria-busy="true"'));
        }
        if (state.state === "degraded") assert.ok(markup.includes("recorded head 42"));
      }
    }
    // Drive the actual Next loading/error boundaries inherited by every live page.
    for (const boundary of ["loading", "error"] as const) {
      let directory = join(appRoot, route.slice(1));
      while (!existsSync(join(directory, `${boundary}.tsx`))) {
        assert.notEqual(directory, dirname(appRoot), `${route} has no ${boundary} boundary`);
        directory = dirname(directory);
      }
      const { default: Component } = await import(new URL(`file://${join(directory, `${boundary}.tsx`)}`).href);
      const element = boundary === "error"
        ? createElement(Component, { error: new Error("private failure"), reset: callback })
        : createElement(Component);
      const markup = renderToStaticMarkup(element);
      assert.ok(markup.includes(boundary === "error" ? 'role="alert"' : 'role="status"'));
      assert.ok(!markup.includes("private failure"));
    }
  }
  assert.throws(() => renderToStaticMarkup(createElement(MatrixStateSurface, { route: "/explorer/batches", shell: "mobile", state: "loading" })), /Invalid report route/u);
}

function preserveEnvironment(context: TestContext, names: readonly string[]): void {
  const previous = names.map((name) => [name, process.env[name]] as const);
  context.after(() => {
    for (const [name, value] of previous) {
      if (value === undefined) delete process.env[name]; else process.env[name] = value;
    }
  });
}

async function reportPeer(context: TestContext) {
  const directory = await mkdtemp(join(tmpdir(), "repair-22-33-355-"));
  context.after(() => rm(directory, { recursive: true, force: true }));
  preserveEnvironment(context, ["LAYERX_SUPPORT_REPORT_DIR", "LAYERX_SUPPORT_REPORT_RETENTION", "LAYERX_HUMAN_SERVICE_URL", "LAYERX_HUMAN_WEB_ORIGIN"]);
  const cert = readFileSync(new URL("./fixtures/localhost-cert.pem", import.meta.url), "utf8");
  const key = readFileSync(new URL("./fixtures/localhost-key.pem", import.meta.url), "utf8");
  const previousCertificates = getCACertificates("default");
  setDefaultCACertificates([...previousCertificates, cert]);
  context.after(() => setDefaultCACertificates(previousCertificates));
  const cookies = {
    a: "__Host-layerx-session=recorded-session-a; __Host-layerx_csrf=recorded-csrf-token-a-355",
    b: "__Host-layerx-session=recorded-session-b; __Host-layerx_csrf=recorded-csrf-token-b-355",
  };
  const received: { body: string; principal: string }[] = [];
  const failures: unknown[] = [];
  let origin = "";
  let pause: (() => Promise<void>) | undefined;
  const server = createServer({ cert, key }, async (incoming, outgoing) => {
    outgoing.setHeader("connection", "close");
    try {
      const path = incoming.url ?? "/";
      const cookie = incoming.headers.cookie ?? "";
      const actor = cookie === cookies.a ? "a" : cookie === cookies.b ? "b" : undefined;
      if (path === "/v1/account/balance" || path === "/v1/sessions") {
        if (actor === undefined) { outgoing.writeHead(401).end(); return; }
        assert.equal(incoming.method, "GET");
        const result = path === "/v1/account/balance"
          ? encodeAccountBalance({ account_id: `recorded-account-${actor}`, money: { amount: 0n, currency: "USD" }, verification: "unverified", freshness: { observed_at: "2026-11-01T00:00:00Z", age_seconds: 0, source_head: "1", within_bound: true }, evidence: [] })
          : encodeSessionList({ sessions: [{ session_id: `recorded-session-${actor}`, current: true, device: { device_id: "recorded-device", label: "Fixture browser", platform: "desktop" }, opened_at: "2026-11-01T00:00:00Z", last_active_at: "2026-11-01T00:00:00Z" }] });
        outgoing.writeHead(200, { "content-type": "application/json" }).end(JSON.stringify({ ok: true, trace: "trc_session_fixture", result }));
        return;
      }
      const headers = new Headers();
      for (const [name, value] of Object.entries(incoming.headers)) {
        if (value !== undefined) headers.set(name, Array.isArray(value) ? value.join(", ") : value);
      }
      let body = "";
      for await (const chunk of incoming) body += String(chunk);
      const request = new Request(`${origin}${path}`, { method: incoming.method ?? "GET", headers, ...(incoming.method === "POST" ? { body } : {}) });
      assert.equal(path, "/api/support/reports");
      await pause?.();
      const session = await verifiedWebSession(cookie);
      let response: Response;
      if (session === undefined) response = await POST(request);
      else {
        // Exercise the exported production guard at the out-of-scope POST wiring boundary.
        try { requireSupportReportPrincipal(request, session); }
        catch (error) {
          if (!(error instanceof SupportReportPrincipalError)) throw error;
          outgoing.writeHead(403).end(); return;
        }
        response = await POST(request);
        if (response.ok) received.push({ body, principal: session.principalScope });
      }
      outgoing.writeHead(response.status, Object.fromEntries(response.headers));
      outgoing.end(Buffer.from(await response.arrayBuffer()));
    } catch (error) { failures.push(error); outgoing.writeHead(500).end(); }
  });
  async function stop(): Promise<void> {
    if (!server.listening) return;
    await new Promise<void>((resolve, reject) => {
      server.close((error) => error === undefined ? resolve() : reject(error)); server.closeAllConnections();
    });
  }
  context.after(stop);
  async function reopen(port = 0): Promise<void> {
    await new Promise<void>((resolve, reject) => { server.once("error", reject); server.listen(port, "127.0.0.1", resolve); });
    const address = server.address(); assert.ok(address !== null && typeof address !== "string");
    origin = `https://127.0.0.1:${address.port}`;
  }
  await reopen();
  Object.assign(process.env, { LAYERX_SUPPORT_REPORT_DIR: directory, LAYERX_SUPPORT_REPORT_RETENTION: "50", LAYERX_HUMAN_SERVICE_URL: origin, LAYERX_HUMAN_WEB_ORIGIN: origin });
  const sessionA = await verifiedWebSession(cookies.a); const sessionB = await verifiedWebSession(cookies.b);
  assert.ok(sessionA); assert.ok(sessionB); assert.notEqual(sessionA.principalScope, sessionB.principalScope);
  return { directory, origin, cookies, sessionA, sessionB, received, failures, stop, reopen, setPause(value: (() => Promise<void>) | undefined) { pause = value; } };
}

function browserPeer(context: TestContext, origin: string, cookie: string) {
  const browser = new Window({ url: origin });
  const nativeFetch = globalThis.fetch;
  const globals = { window: browser, document: browser.document, navigator: browser.navigator, HTMLElement: browser.HTMLElement, IS_REACT_ACT_ENVIRONMENT: true };
  const previous = [...Object.keys(globals), "fetch"].map((name) => [name, Object.getOwnPropertyDescriptor(globalThis, name)] as const);
  context.after(async () => {
    await browser.happyDOM.close();
    for (const [name, descriptor] of previous) {
      if (descriptor === undefined) Reflect.deleteProperty(globalThis, name); else Object.defineProperty(globalThis, name, descriptor);
    }
  });
  for (const [name, value] of Object.entries(globals)) Object.defineProperty(globalThis, name, { configurable: true, writable: true, value });
  const switchCookie = (next: string) => {
    for (const entry of browser.document.cookie.split(";")) browser.document.cookie = `${entry.trim().split("=")[0]}=; Max-Age=0; Secure; Path=/`;
    for (const entry of next.split(";")) browser.document.cookie = `${entry.trim()}; Secure; Path=/`;
  };
  switchCookie(cookie);
  Object.defineProperty(globalThis, "fetch", {
    configurable: true, writable: true,
    value: ((input, init) => {
      if (typeof input !== "string" || !input.startsWith("/")) return nativeFetch(input, init);
      assert.equal(init?.credentials, "same-origin");
      const headers = new Headers(init?.headers); headers.set("cookie", browser.document.cookie);
      return nativeFetch(new URL(input, origin), { ...init, headers });
    }) satisfies typeof fetch,
  });
  return { browser, storage: browser.localStorage, switchCookie, nativeFetch };
}

async function support_report_principal_replay_offline_switch_reopen(context: TestContext): Promise<void> {
  const peer = await reportPeer(context);
  const { storage, switchCookie } = browserPeer(context, peer.origin, peer.cookies.a);
  storage.setItem("layerx.human.pending-support-reports.v1", JSON.stringify([redactErrorReport(draft)]));
  let outboxA = new BrowserSupportReportOutbox(peer.sessionA);
  assert.equal(storage.getItem("layerx.human.pending-support-reports.v1"), null, "legacy unowned reports must never be adopted");
  const attempt = outboxA.prepare(draft);
  assert.ok(Object.isFrozen(attempt)); assert.ok(Object.isFrozen(attempt.report));
  assert.equal(Reflect.set(attempt.report, "route", "/bridge"), false);
  assert.throws(() => outboxA.prepare({ ...draft, consented: false }), /consent/u);
  await peer.stop();
  assert.deepEqual(await outboxA.submitAttempt(attempt), { traceId: draft.traceId, status: "pending" });
  const keyA = storage.key(0); assert.ok(keyA); assert.ok(keyA.endsWith(peer.sessionA.principalScope));
  const queuedA = storage.getItem(keyA); assert.ok(queuedA);
  assert.deepEqual(JSON.parse(queuedA), [attempt]);
  assert.equal((await outboxA.submitAttempt(attempt)).status, "pending");
  assert.equal(storage.getItem(keyA), queuedA, "the same immutable attempt is queued only once");
  await assert.rejects(outboxA.submit({ ...draft, route: "/bridge" }), /attempt conflict/u);
  storage.setItem(keyA, JSON.stringify([{ ...attempt, principalScope: peer.sessionB.principalScope }]));
  await assert.rejects(outboxA.flushPending(), SupportReportPrincipalError);
  assert.equal(peer.received.length, 0);
  storage.setItem(keyA, queuedA);
  assert.ok(!queuedA.includes("private-agent")); assert.ok(!queuedA.includes("csrf"));
  outboxA.close();
  switchCookie(peer.cookies.b);
  let outboxB = new BrowserSupportReportOutbox(peer.sessionB);
  await assert.rejects(outboxB.submitAttempt(attempt), SupportReportPrincipalError);
  const draftB = { ...draft, traceId: "trc_principal_b", shell: "mobile" as const };
  assert.equal((await outboxB.submit(draftB)).status, "pending");
  const keyB = [...Array(storage.length).keys()].map((index) => storage.key(index)).find((key) => key?.endsWith(peer.sessionB.principalScope));
  assert.ok(keyB); const queuedB = storage.getItem(keyB);
  await peer.reopen(Number(new URL(peer.origin).port));
  outboxB.close(); outboxB = new BrowserSupportReportOutbox(peer.sessionB);
  await Promise.all([outboxB.flushPending(), outboxB.flushPending()]);
  assert.equal(storage.getItem(keyB), null); assert.equal(storage.getItem(keyA), queuedA);
  assert.deepEqual(peer.received.map((entry) => entry.principal), [peer.sessionB.principalScope]);
  assert.equal(await new SupportReportRepository(peer.directory, 50).findByTrace(draft.traceId, peer.sessionB.principalScope), undefined);
  outboxB.close(); switchCookie(peer.cookies.a); outboxA = new BrowserSupportReportOutbox(peer.sessionA);
  await outboxA.flushPending();
  assert.equal(storage.length, 0);
  const reopened = new SupportReportRepository(peer.directory, 50);
  const recoveredA = await reopened.findByTrace(draft.traceId, peer.sessionA.principalScope);
  assert.ok(recoveredA); assert.equal(recoveredA.route, "/app/agents/[agentId]");
  assert.equal(await reopened.findByTrace(draftB.traceId, peer.sessionA.principalScope), undefined);
  assert.deepEqual(peer.received.map((entry) => parseSupportReportRequest(JSON.parse(entry.body))), [redactErrorReport(draftB), attempt.report]);
  assert.ok(queuedB);
  storage.setItem(keyB, queuedB); storage.setItem("unrelated-preference", "keep");
  outboxA.logout();
  assert.equal(storage.getItem(keyB), null); assert.equal(storage.getItem("unrelated-preference"), "keep");
  await assert.rejects(outboxA.submitAttempt(attempt), SupportReportPrincipalError);
  assert.throws(() => outboxA.connect(), SupportReportPrincipalError);
  await assert.rejects(new BrowserSupportReportOutbox().submit(draft), SupportReportPrincipalError);
  assert.deepEqual(peer.failures, []);
}

async function support_report_principal_replay_session_guard_and_authorization(context: TestContext): Promise<void> {
  const peer = await reportPeer(context);
  const { storage, switchCookie, nativeFetch } = browserPeer(context, peer.origin, peer.cookies.a);
  const outbox = new BrowserSupportReportOutbox(peer.sessionA);
  const attempt = outbox.prepare(draft);
  switchCookie(peer.cookies.b);
  await assert.rejects(outbox.submitAttempt(attempt), SupportReportPrincipalError);
  assert.equal(peer.received.length, 0); assert.equal(storage.length, 0);
  for (const [cookie, principal, csrf, expected] of [
    [peer.cookies.b, peer.sessionA.principalScope, "recorded-csrf-token-b-355", 403],
    [peer.cookies.a, peer.sessionA.principalScope, "wrong-csrf-token-355", 403],
    ["__Host-layerx-session=forged", peer.sessionA.principalScope, "recorded-csrf-token-a-355", 401],
    ["", peer.sessionA.principalScope, "recorded-csrf-token-a-355", 401],
    [peer.cookies.a, "", "recorded-csrf-token-a-355", 403],
  ] as const) {
    const response = await nativeFetch(`${peer.origin}/api/support/reports`, { method: "POST", headers: { cookie, "content-type": "application/json", "x-layerx-csrf": csrf, [SUPPORT_REPORT_PRINCIPAL_HEADER]: principal }, body: JSON.stringify(attempt.report) });
    assert.equal(response.status, expected);
  }
  assert.equal(peer.received.length, 0);
  switchCookie(peer.cookies.a);
  const saved = await outbox.submitAttempt(attempt);
  assert.deepEqual(saved, { traceId: draft.traceId, status: "saved" });
  assert.equal(peer.received.length, 1);
  const conflict = { ...draft, route: "/bridge" };
  await assert.rejects(outbox.submit(conflict), /server rejected/u);
  assert.equal(storage.length, 0, "server refusals must not enter the offline queue");
  const repository = new SupportReportRepository(peer.directory, 50);
  assert.equal((await repository.findByTrace(draft.traceId, peer.sessionA.principalScope))?.route, "/app/agents/[agentId]");
  assert.equal(await repository.findByTrace(draft.traceId, peer.sessionB.principalScope), undefined);
  assert.deepEqual(peer.failures, []);
}

async function support_report_principal_replay_authenticated_boundary_logout(context: TestContext): Promise<void> {
  const peer = await reportPeer(context);
  const { browser, storage, switchCookie } = browserPeer(context, peer.origin, peer.cookies.a);
  const container = browser.document.createElement("div"); browser.document.body.append(container);
  const root = createRoot(container as unknown as HTMLElement);
  const captured: Outbox[] = [];
  function ReporterConsumer() {
    const outbox = useSupportReportOutbox();
    if (captured.at(-1) !== outbox) captured.push(outbox);
    return createElement(ErrorSurface, { error: errorPresentation({ code: "UI_UNHANDLED" }, draft.traceId), route: "/app", platform: "desktop" });
  }
  const render = async (session: Session | undefined) => {
    await act(async () => { root.render(createElement(StrictMode, null, createElement(SupportReportSessionBoundary, { session, children: createElement(ReporterConsumer) }))); });
  };
  try {
    await render(peer.sessionA);
    const outboxA = captured.at(-1); assert.ok(outboxA);
    const attempt = outboxA.prepare(draft);
    await peer.stop();
    assert.equal((await outboxA.submitAttempt(attempt)).status, "pending");
    switchCookie(peer.cookies.b); await render(peer.sessionB);
    const outboxB = captured.at(-1); assert.ok(outboxB); assert.notEqual(outboxB, outboxA);
    await assert.rejects(outboxA.submit(draft), SupportReportPrincipalError);
    await assert.rejects(outboxB.submitAttempt(attempt), SupportReportPrincipalError);
    assert.equal(storage.length, 1);
    switchCookie(""); await render(undefined);
    assert.equal(storage.length, 0);
    await assert.rejects(outboxB.submit(draft), SupportReportPrincipalError);
    assert.deepEqual(peer.received, []); assert.deepEqual(peer.failures, []);
  } finally { await act(async () => { root.unmount(); }); }
}

async function support_report_principal_replay_in_flight_logout(context: TestContext): Promise<void> {
  const peer = await reportPeer(context);
  const { storage, switchCookie } = browserPeer(context, peer.origin, peer.cookies.a);
  const outbox = new BrowserSupportReportOutbox(peer.sessionA);
  const arrived = Promise.withResolvers<void>();
  const resume = Promise.withResolvers<void>();
  peer.setPause(async () => { arrived.resolve(); await resume.promise; });
  const pending = outbox.submit(draft);
  const rejected = assert.rejects(pending, SupportReportPrincipalError);
  await arrived.promise;
  switchCookie(peer.cookies.b);
  outbox.logout();
  resume.resolve();
  await rejected;
  assert.equal(storage.length, 0, "an in-flight completion cannot recreate a queue after logout");
  assert.deepEqual(peer.received.map((entry) => entry.principal), [peer.sessionA.principalScope]);
  await peer.stop();
  const recovered = new SupportReportRepository(peer.directory, 50);
  assert.equal((await recovered.findByTrace(draft.traceId, peer.sessionA.principalScope))?.route, "/app/agents/[agentId]");
  assert.equal(await recovered.findByTrace(draft.traceId, peer.sessionB.principalScope), undefined);
  assert.deepEqual(peer.failures, []);
}

test(state_matrix_stale_inventory_matches_live_routes);
test(state_matrix_real_surfaces_both_shells);
test(support_report_principal_replay_offline_switch_reopen);
test(support_report_principal_replay_session_guard_and_authorization);
test(support_report_principal_replay_authenticated_boundary_logout);

test(support_report_principal_replay_in_flight_logout);
