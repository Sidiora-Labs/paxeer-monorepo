import assert from "node:assert/strict";
import { randomBytes } from "node:crypto";
import { existsSync, readFileSync } from "node:fs";
import { mkdtemp, readdir, rm } from "node:fs/promises";
import { createServer } from "node:https";
import { registerHooks } from "node:module";
import { tmpdir } from "node:os";
import { join } from "node:path";
import test, { type TestContext } from "node:test";
import { getCACertificates, setDefaultCACertificates } from "node:tls";
import { fileURLToPath } from "node:url";
import ts from "typescript";

registerHooks({
  resolve(specifier, context, nextResolve) {
    if (specifier === "server-only") {
      return nextResolve("next/dist/compiled/server-only/empty.js", context);
    }
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
          ? new URL(candidate.href.slice(0, -3) + suffix)
          : new URL(candidate.href + suffix);
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
          compilerOptions: {
            target: ts.ScriptTarget.ES2022, module: ts.ModuleKind.ESNext,
            jsx: ts.JsxEmit.ReactJSX,
          },
        }).outputText,
      };
    }
    const loaded = nextLoad(url, context);
    return loaded.format === "commonjs" ? { ...loaded, source: undefined } : loaded;
  },
});

const { createElement } = await import("react");
const { renderToStaticMarkup } = await import("react-dom/server");
const { default: AgentDetailRouteError } = await import("../src/app/app/agents/[agentId]/error.tsx");
const { RouteErrorBoundary, ErrorSurface } = await import("../src/states/error.tsx");
const { CURRENT_ROUTES } = await import("../src/states/matrix.ts");
const { SUPPORT_REPORT_ROUTES, supportReportRoute, redactSupportReportRoute } = await import("../src/states/report-routes.ts");
const { BrowserSupportReportOutbox, redactErrorReport } = await import("../src/states/report.ts");
const { encodeAccountBalance, encodeSessionList } = await import("../src/api/generated/index.ts");
const { parseSupportReportRequest, parseStoredSupportReport, parseSavedReportReceipt } = await import("../src/states/report-schema.ts");
const { SupportReportRepository } = await import("../src/server/support-reports.ts");
const { POST } = await import("../src/app/api/support/reports/route.ts");
const { GET } = await import("../src/app/api/support/reports/[traceId]/route.ts");
const { csrfTokenFromCookie } = await import("../src/auth/session.ts");

function agentBoundaryDraft() {
  let resets = 0;
  const error = Object.assign(new Error("Private agent name and entered data"), {
    digest: `trc_${randomBytes(16).toString("hex")}`,
  });
  const boundary = AgentDetailRouteError({ error, reset: () => { resets += 1; } });
  assert.equal(boundary.type, RouteErrorBoundary);
  const surface = RouteErrorBoundary(boundary.props);
  assert.equal(surface.type, ErrorSurface);
  const markup = renderToStaticMarkup(createElement(ErrorSurface, surface.props));
  assert.ok(markup.includes(surface.props.error.traceId));
  assert.ok(!markup.includes(error.message));
  assert.equal(resets, 0);
  return {
    consented: true as const,
    route: surface.props.route,
    traceId: surface.props.error.traceId,
    machineCode: surface.props.error.machineCode,
    shell: "desktop" as const,
  };
}

function agentBoundaryReport() {
  return redactErrorReport(agentBoundaryDraft());
}

class ReportBrowserStorage implements Storage {
  readonly #entries = new Map<string, string>();
  get length(): number { return this.#entries.size; }
  clear(): void { this.#entries.clear(); }
  getItem(key: string): string | null { return this.#entries.get(key) ?? null; }
  key(index: number): string | null { return [...this.#entries.keys()][index] ?? null; }
  removeItem(key: string): void { this.#entries.delete(key); }
  setItem(key: string, value: string): void { this.#entries.set(key, value); }
}

async function reportHttpsFixture(context: TestContext) {
  const directory = await mkdtemp(join(tmpdir(), "repair-22-31-387-post-"));
  context.after(() => rm(directory, { recursive: true, force: true }));
  preserveEnvironment(context, [
    "LAYERX_SUPPORT_REPORT_DIR", "LAYERX_SUPPORT_REPORT_BEARER_TOKEN",
    "LAYERX_SUPPORT_REPORT_RETENTION", "LAYERX_HUMAN_SERVICE_URL", "LAYERX_HUMAN_WEB_ORIGIN",
  ]);
  const credential = randomBytes(32).toString("hex");
  const csrf = randomBytes(32).toString("hex");
  const cookie = `__Host-layerx-session=${randomBytes(32).toString("hex")}; __Host-layerx_csrf=${csrf}`;
  const cert = readFileSync(new URL("./fixtures/localhost-cert.pem", import.meta.url), "utf8");
  const key = readFileSync(new URL("./fixtures/localhost-key.pem", import.meta.url), "utf8");
  const previousCertificates = getCACertificates("default");
  setDefaultCACertificates([...previousCertificates, cert]);
  context.after(() => setDefaultCACertificates(previousCertificates));
  const sessionRequests: string[] = [];
  const received: string[] = [];
  const failures: unknown[] = [];
  let origin = "";
  const server = createServer({ cert, key }, async (incoming, outgoing) => {
    outgoing.setHeader("connection", "close");
    try {
      const path = incoming.url ?? "/";
      if (path === "/v1/account/balance" || path === "/v1/sessions") {
        sessionRequests.push(path);
        if (incoming.headers.cookie !== cookie) {
          outgoing.writeHead(401).end();
          return;
        }
        assert.equal(incoming.method, "GET");
        assert.equal(incoming.headers.origin, origin);
        const result = path === "/v1/account/balance"
          ? encodeAccountBalance({
            account_id: "support-report-fixture-account",
            money: { amount: 0n, currency: "USD" },
            verification: "unverified",
            freshness: { observed_at: "2026-11-01T00:00:00Z", age_seconds: 0, source_head: "1", within_bound: true },
            evidence: [],
          })
          : encodeSessionList({ sessions: [{
            session_id: "support-report-fixture-session", current: true,
            device: { device_id: "fixture-device", label: "Fixture browser", platform: "desktop" },
            opened_at: "2026-11-01T00:00:00Z", last_active_at: "2026-11-01T00:00:00Z",
          }] });
        outgoing.writeHead(200, { "content-type": "application/json" });
        outgoing.end(JSON.stringify({ ok: true, trace: "trc_session_fixture", result }));
        return;
      }
      const headers = new Headers();
      for (const [name, value] of Object.entries(incoming.headers)) {
        if (value !== undefined) headers.set(name, Array.isArray(value) ? value.join(", ") : value);
      }
      let body = "";
      for await (const chunk of incoming) body += String(chunk);
      const request = new Request(`${origin}${path}`, {
        method: incoming.method ?? "GET", headers,
        ...(incoming.method === "POST" ? { body } : {}),
      });
      let response: Response;
      if (path === "/api/support/reports" && incoming.method === "POST") {
        received.push(body);
        response = await POST(request);
      } else if (path.startsWith("/api/support/reports/") && incoming.method === "GET") {
        response = await GET(request, { params: Promise.resolve({ traceId: path.slice("/api/support/reports/".length) }) });
      } else {
        response = new Response(null, { status: 404 });
      }
      outgoing.writeHead(response.status, Object.fromEntries(response.headers));
      outgoing.end(Buffer.from(await response.arrayBuffer()));
    } catch (error) {
      failures.push(error);
      outgoing.writeHead(500).end();
    }
  });
  async function stop(): Promise<void> {
    if (!server.listening) return;
    await new Promise<void>((resolve, reject) => {
      server.close((error) => error === undefined ? resolve() : reject(error));
      server.closeAllConnections();
    });
  }
  context.after(stop);
  async function reopen(port = 0): Promise<void> {
    await new Promise<void>((resolve, reject) => {
      server.once("error", reject);
      server.listen(port, "127.0.0.1", resolve);
    });
    const address = server.address();
    assert.ok(address !== null && typeof address !== "string");
    origin = `https://127.0.0.1:${address.port}`;
  }
  await reopen();
  Object.assign(process.env, {
    LAYERX_SUPPORT_REPORT_DIR: directory,
    LAYERX_SUPPORT_REPORT_BEARER_TOKEN: credential,
    LAYERX_SUPPORT_REPORT_RETENTION: "10",
    LAYERX_HUMAN_SERVICE_URL: origin,
    LAYERX_HUMAN_WEB_ORIGIN: origin,
  });
  return { directory, credential, csrf, cookie, origin, received, sessionRequests, failures, stop, reopen };
}

function reportBrowserFixture(context: TestContext, origin: string, cookie: string) {
  const storage = new ReportBrowserStorage();
  const nativeFetch = globalThis.fetch;
  const previous = ["window", "document", "navigator", "fetch"].map((name) => [
    name, Object.getOwnPropertyDescriptor(globalThis, name),
  ] as const);
  context.after(() => {
    for (const [name, descriptor] of previous) {
      if (descriptor === undefined) Reflect.deleteProperty(globalThis, name);
      else Object.defineProperty(globalThis, name, descriptor);
    }
  });
  Object.defineProperties(globalThis, {
    window: { configurable: true, value: { localStorage: storage } },
    document: { configurable: true, value: { cookie } },
    navigator: { configurable: true, value: { onLine: true } },
    fetch: {
      configurable: true, writable: true,
      value: ((input, init) => {
        if (typeof input !== "string" || !input.startsWith("/")) return nativeFetch(input, init);
        // Supply browser URL resolution and cookies; every response comes over real HTTPS.
        assert.equal(init?.credentials, "same-origin");
        const headers = new Headers(init?.headers);
        headers.set("cookie", cookie);
        return nativeFetch(new URL(input, origin), { ...init, headers });
      }) satisfies typeof fetch,
    },
  });
  return storage;
}

function preserveEnvironment(context: TestContext, names: readonly string[]): void {
  const previous = names.map((name) => [name, process.env[name]] as const);
  context.after(() => {
    for (const [name, value] of previous) {
      if (value === undefined) delete process.env[name];
      else process.env[name] = value;
    }
  });
}

async function support_report_route_template_durable_retrieval(context: TestContext): Promise<void> {
  const report = agentBoundaryReport();
  assert.equal(report.route, "/app/agents/[agentId]");
  assert.deepEqual(Object.keys(report).sort(), ["machineCode", "route", "shell", "traceId"]);
  const request = new Request("https://app.example/api/support/reports", {
    method: "POST",
    headers: { "content-type": "application/json" },
    body: JSON.stringify(report),
  });
  const received = parseSupportReportRequest(await request.json());
  assert.deepEqual(received, report);

  const directory = await mkdtemp(join(tmpdir(), "repair-22-31-387-"));
  context.after(() => rm(directory, { recursive: true, force: true }));
  preserveEnvironment(context, ["LAYERX_SUPPORT_REPORT_DIR", "LAYERX_SUPPORT_REPORT_BEARER_TOKEN"]);
  process.env.LAYERX_SUPPORT_REPORT_DIR = directory;
  const credential = randomBytes(32).toString("hex");
  process.env.LAYERX_SUPPORT_REPORT_BEARER_TOKEN = credential;
  const principal = "route-template-qualified-principal";
  const saved = await new SupportReportRepository(directory, 10).save(principal, received);
  assert.equal(saved.created, true);
  const reopened = new SupportReportRepository(directory, 10);
  assert.deepEqual(await reopened.findByTrace(report.traceId, principal), saved.record);
  const getRequest = new Request(`https://app.example/api/support/reports/${report.traceId}`, {
    headers: { authorization: `Bearer ${credential}` },
  });
  const response = await GET(getRequest, { params: Promise.resolve({ traceId: report.traceId }) });
  assert.equal(response.status, 200);
  assert.deepEqual(parseStoredSupportReport(await response.json()), saved.record);
  assert.equal(response.headers.get("cache-control"), "no-store");
  const unauthorized = await GET(new Request(getRequest.url), { params: Promise.resolve({ traceId: report.traceId }) });
  assert.equal(unauthorized.status, 401);
  assert.equal((await reopened.save(principal, received)).created, false);
}

function support_report_route_template_inventory(): void {
  for (const route of [...CURRENT_ROUTES, ...SUPPORT_REPORT_ROUTES]) {
    const report = redactErrorReport({
      consented: true, traceId: "trc_inventory", machineCode: "UI_UNHANDLED", route, shell: "mobile",
    });
    assert.equal(report.route, route);
    assert.deepEqual(parseSupportReportRequest(JSON.parse(JSON.stringify(report))), report);
  }
  for (const route of ["/app/agents/new", "/app/agents", "/app/settings/security"]) {
    assert.equal(redactSupportReportRoute(route), route);
  }
}

function support_report_route_template_redacts_identifiers(): void {
  for (const [path, template] of [
    ["/app/agents/private-agent", "/app/agents/[agentId]"],
    ["/app/agents/did%3Aexample%3Aprivate", "/app/agents/[agentId]"],
    ["/app/activity/private-entry", "/app/activity/[entryId]"],
    ["/app/approvals/private-approval", "/app/approvals/[approvalId]"],
    ["/explorer/receipts/private-receipt", "/explorer/receipts/[receiptId]"],
    ["/explorer/accounts/private-account", "/explorer/accounts/[accountId]"],
    ["/explorer/programs/private-program", "/explorer/programs/[programId]"],
  ]) {
    assert.ok(path !== undefined && template !== undefined);
    const report = redactErrorReport({
      consented: true, traceId: "trc_redacted", machineCode: "UI_UNHANDLED", route: path, shell: "desktop",
    });
    assert.equal(report.route, template);
    assert.ok(!JSON.stringify(report).includes("private"));
    assert.throws(() => parseSupportReportRequest({ ...report, route: path }), /Invalid report route/u);
  }
}

function support_report_route_template_refuses_unsafe_paths(): void {
  const report = agentBoundaryReport();
  for (const route of [
    "/app/agents/[unknown]", "/app/agents/[...agentId]", "/app/agents/[]", "/other/[agentId]",
    "/app/agents/..", "/app/agents/.", "/app/agents/%2e%2e", "/app/agents/%252e%252e",
    "/app/agents/../support", "/app/agents/%2fprivate", "/app/agents/%5cprivate",
    "/app/agents/private?secret=value", "/app/agents/private#secret", "/app/agents/[agentId]?secret=value",
    "/app/agents/%00", "/app/agents/%", "/app//agents/private", "//app/agents/private",
    "https://app.example/app/agents/private", "/app/agents/", `/app/agents/${"a".repeat(161)}`,
  ]) {
    assert.throws(() => supportReportRoute(route), /Invalid report route/u, route);
    assert.throws(() => redactSupportReportRoute(route), /Invalid report route/u, route);
    assert.throws(() => parseSupportReportRequest({ ...report, route }), /Invalid report route/u, route);
  }
  assert.throws(() => redactErrorReport({ ...report, consented: false }), /consent/u);
  assert.throws(() => parseSupportReportRequest({ ...report, enteredValue: "private" }), /fields/u);
}

async function support_report_route_template_authenticated_post(context: TestContext): Promise<void> {
  const report = agentBoundaryReport();
  const fixture = await reportHttpsFixture(context);
  const { cookie, directory, credential, origin } = fixture;
  const csrf = csrfTokenFromCookie(cookie);
  assert.ok(csrf, "POST proof requires the session's real CSRF cookie");
  assert.equal(csrf, fixture.csrf);
  const response = await fetch(`${origin}/api/support/reports`, {
    method: "POST",
    headers: { "content-type": "application/json", cookie, "x-layerx-csrf": csrf },
    body: JSON.stringify(report),
  });
  assert.equal(response.status, 201);
  assert.deepEqual(parseSavedReportReceipt(await response.json()), { traceId: report.traceId, status: "saved" });
  assert.equal((await readdir(directory)).length, 1);
  const reopened = new SupportReportRepository(directory, 10);
  const stored = await reopened.findByTrace(report.traceId);
  assert.ok(stored);
  assert.equal(stored.route, report.route);
  await fixture.stop();
  await fixture.reopen(Number(new URL(origin).port));
  const retrieved = await fetch(`${origin}/api/support/reports/${report.traceId}`, {
    headers: { authorization: `Bearer ${credential}` },
  });
  assert.equal(retrieved.status, 200);
  assert.deepEqual(parseStoredSupportReport(await retrieved.json()), stored);
  assert.deepEqual(fixture.received, [JSON.stringify(report)]);
  assert.deepEqual(fixture.sessionRequests.sort(), ["/v1/account/balance", "/v1/sessions"]);
  const rejectedRequests: readonly [HeadersInit, number][] = [
    [{ "content-type": "application/json", "x-layerx-csrf": csrf }, 401],
    [{ "content-type": "application/json", cookie: "__Host-layerx-session=forged", "x-layerx-csrf": csrf }, 401],
    [{ "content-type": "application/json", cookie }, 403],
  ];
  for (const [headers, status] of rejectedRequests) {
    const rejected = await fetch(`${origin}/api/support/reports`, {
      method: "POST", headers, body: JSON.stringify(report),
    });
    assert.equal(rejected.status, status);
  }
  const unauthorized = await fetch(`${origin}/api/support/reports/${report.traceId}`);
  assert.equal(unauthorized.status, 401);
  assert.deepEqual(fixture.failures, []);
}

async function support_report_route_template_outbox_reopens(context: TestContext): Promise<void> {
  const draft = agentBoundaryDraft();
  const report = redactErrorReport(draft);
  const fixture = await reportHttpsFixture(context);
  const storage = reportBrowserFixture(context, fixture.origin, fixture.cookie);
  await fixture.stop();
  let outbox = new BrowserSupportReportOutbox();
  assert.deepEqual(await outbox.submit(draft), { traceId: report.traceId, status: "pending" });
  assert.equal(storage.length, 1);
  const queueKey = storage.key(0);
  assert.ok(queueKey);
  const queued = storage.getItem(queueKey);
  assert.ok(queued);
  assert.deepEqual(JSON.parse(queued), [report]);
  assert.ok(!queued.includes("Private agent name"));
  assert.deepEqual(fixture.received, []);

  // Recreate the production outbox with the browser's retained storage, then reopen transport.
  outbox = new BrowserSupportReportOutbox();
  await fixture.reopen(Number(new URL(fixture.origin).port));
  await outbox.flushPending();
  assert.equal(storage.length, 0);
  assert.deepEqual(fixture.received, [JSON.stringify(report)]);
  const reopened = new SupportReportRepository(fixture.directory, 10);
  const stored = await reopened.findByTrace(report.traceId);
  assert.ok(stored);
  assert.deepEqual(parseSupportReportRequest({
    traceId: stored.traceId, machineCode: stored.machineCode, route: stored.route, shell: stored.shell,
  }), report);
  const receipt = await outbox.submit(draft);
  assert.deepEqual(receipt, { traceId: report.traceId, status: "saved" });
  assert.deepEqual(await reopened.findByTrace(report.traceId), stored);
  assert.equal((await readdir(fixture.directory)).length, 1);
  await fixture.stop();
  await fixture.reopen(Number(new URL(fixture.origin).port));
  const retrieved = await fetch(`${fixture.origin}/api/support/reports/${report.traceId}`, {
    headers: { authorization: `Bearer ${fixture.credential}` },
  });
  assert.equal(retrieved.status, 200);
  assert.deepEqual(parseStoredSupportReport(await retrieved.json()), stored);
  assert.deepEqual(fixture.failures, []);
}

test(support_report_route_template_durable_retrieval);
test(support_report_route_template_inventory);
test(support_report_route_template_redacts_identifiers);
test(support_report_route_template_refuses_unsafe_paths);
test(support_report_route_template_authenticated_post);
test(support_report_route_template_outbox_reopens);
