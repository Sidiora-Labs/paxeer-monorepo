import assert from "node:assert/strict";
import { createHash } from "node:crypto";
import { existsSync, readFileSync } from "node:fs";
import { createServer, type ServerResponse } from "node:http";
import { registerHooks } from "node:module";
import test, { type TestContext } from "node:test";
import { fileURLToPath } from "node:url";
import ts from "typescript";
import { Window } from "happy-dom";
import { keccak256, type Hex } from "viem";
import {
  createHumanApiClient,
  decodeAccountBalance,
  decodeActivityEntry,
  decodeActivityQueryRequest,
  decodeExportEvidenceRequest,
  decodeExportStatementRequest,
  encodeAccountBalance,
  encodeActivityPage,
  encodeEvidenceMaterial,
  encodeExportArtefact,
  type ActivityEntry,
  type ActivityFilter,
  type ActivityPage,
  type ActivityQueryRequest,
  type EvidenceMaterial,
  type EvidenceRef,
  type ExportArtefact,
  type JsonValue,
} from "../src/api/generated/index.ts";

registerHooks({
  resolve(specifier, context, nextResolve) {
    if (specifier.startsWith(".") && context.parentURL !== undefined) {
      const candidate = new URL(specifier, context.parentURL);
      for (const suffix of [".ts", ".tsx", "/index.ts"]) {
        const source = new URL(candidate.href + suffix);
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

const browser = new Window({ url: "http://localhost/app/activity" });
Object.assign(globalThis, {
  window: browser, document: browser.document, HTMLElement: browser.HTMLElement,
  Element: browser.Element, Node: browser.Node, Event: browser.Event,
  MutationObserver: browser.MutationObserver, IS_REACT_ACT_ENVIRONMENT: true,
});
const { createElement, act } = await import("react");
const { createRoot } = await import("react-dom/client");
const { renderToStaticMarkup } = await import("react-dom/server");
const { ActivityController } = await import("../src/journeys/activity/controller.ts");
const { ActivityFilterResults } = await import("../src/journeys/activity/activity.tsx");
const { ActivityEvidenceLink } = await import("../src/journeys/activity/detail.tsx");
const { PrivacyModeProvider } = await import("../src/settings/privacy.tsx");
const { explorerPath, filterEchoLines, kindLabel, plainSentence, resolveEvidencePath } = await import("../src/journeys/activity/model.ts");

type Controller = InstanceType<typeof ActivityController>;

function golden(operation: string): JsonValue {
  const document = JSON.parse(readFileSync(new URL(
    `../../../services/human/schema/human-api/golden/${operation}.response.json`, import.meta.url,
  ), "utf8")) as { body: { result: JsonValue } };
  return document.body.result;
}

function recordedEntries(): readonly ActivityEntry[] {
  const transcript = JSON.parse(readFileSync(new URL(
    "../src/journeys/activity/activity-detail-http.json", import.meta.url,
  ), "utf8")) as Record<string, string>;
  return Object.values(transcript).map((exchange) => {
    const envelope = JSON.parse(exchange.split("\r\n\r\n")[1]!) as { result: JsonValue };
    return decodeActivityEntry(envelope.result, "committed activity producer transcript");
  });
}

interface QueuedQuery {
  readonly request: ActivityQueryRequest;
  readonly principal: string;
  readonly response: ServerResponse;
}

interface ExportRow {
  readonly principal: string;
  readonly kind: ExportArtefact["kind"];
  readonly key: string;
  readonly payload: string;
  readonly artefact: ExportArtefact;
  deliveries: number;
}

class ActivityPeer {
  principal = "act_activityprincipala";
  readonly entries = recordedEntries();
  readonly queries: QueuedQuery[] = [];
  readonly exports = new Map<string, ExportRow>();
  readonly exportRequests: ExportRow[] = [];
  readonly evidence = new Map<string, EvidenceMaterial>();
  readonly evidenceRequests: string[] = [];
  readonly unexpected: string[] = [];
  dropNextExportResponse = false;
  conflicts = 0;

  page(request: ActivityQueryRequest): ActivityPage {
    const filter = request.filter ?? {};
    const matching = this.entries.filter((entry) => filter.kinds === undefined || filter.kinds.includes(entry.kind));
    const offset = request.cursor === undefined ? 0 : Number(request.cursor.slice("cur_".length));
    assert.ok(Number.isInteger(offset) && offset >= 0 && offset < matching.length);
    const selected = matching.slice(offset, offset + 1);
    return {
      groups: [{
        month: "2026-08", subtotal_in: { amount: 25n, currency: "LXP" },
        subtotal_out: { amount: 25n, currency: "LXP" }, entries: selected,
      }],
      next_cursor: offset + 1 < matching.length ? `cur_${offset + 1}` : "",
      filter,
    };
  }

  success(response: ServerResponse, result: JsonValue): void {
    response.writeHead(200, { "Content-Type": "application/json", Connection: "close" });
    response.end(JSON.stringify({ ok: true, result, trace: "trc_activity_loopback" }));
  }

  release(index: number, failed = false): void {
    const query = this.queries[index];
    assert.ok(query, `Missing HTTP activity query ${index}`);
    if (failed) {
      query.response.writeHead(503, { "Content-Type": "application/json", Connection: "close" });
      query.response.end(JSON.stringify({ ok: false, error: {
        code: "upstream-unavailable", copy_key: "state.error.body", retry: "retriable",
      }, trace: "trc_activity_loopback" }));
    } else {
      this.success(query.response, encodeActivityPage(this.page(query.request)));
    }
  }
}

async function peer(t: TestContext) {
  const store = new ActivityPeer();
  const server = createServer(async (incoming, response) => {
    try {
      assert.equal(incoming.headers.cookie, "human_session=activity-fixture-session");
      const chunks: Buffer[] = [];
      for await (const chunk of incoming) chunks.push(Buffer.from(chunk));
      const body: JsonValue = chunks.length === 0 ? {} : JSON.parse(Buffer.concat(chunks).toString("utf8"));
      const route = `${incoming.method} ${incoming.url}`;
      if (route === "GET /v1/account/balance") {
        const balance = decodeAccountBalance(golden("account.balance"), "recorded account");
        store.success(response, encodeAccountBalance({ ...balance, account_id: store.principal }));
      } else if (route === "GET /v1/agents") {
        store.success(response, golden("agent.list"));
      } else if (route === "POST /v1/activity/query") {
        store.queries.push({
          request: decodeActivityQueryRequest(body, "real client activity request"),
          principal: store.principal, response,
        });
      } else if (incoming.method === "POST" && ["/v1/activity/exports/statement", "/v1/activity/exports/evidence"].includes(incoming.url ?? "")) {
        const kind = incoming.url!.endsWith("statement") ? "statement" : "evidence-bundle";
        const request = kind === "statement"
          ? decodeExportStatementRequest(body, "real statement request")
          : decodeExportEvidenceRequest(body, "real evidence export request");
        const key = incoming.headers["idempotency-key"];
        assert.equal(typeof key, "string");
        assert.ok(typeof key === "string" && key.length > 0);
        const identity = JSON.stringify([store.principal, key]);
        const payload = JSON.stringify([kind, request]);
        let row = store.exports.get(identity);
        if (row !== undefined && row.payload !== payload) {
          store.conflicts++;
          response.writeHead(409).end("Idempotency key identifies a different immutable payload");
          return;
        }
        if (row === undefined) {
          const id = `exp_${createHash("sha256").update(identity + payload).digest("hex")}`;
          row = {
            principal: store.principal, key, payload, kind, deliveries: 0,
            artefact: {
              export_id: id, kind, download_path: `/v1/activity/exports/${id}/download`,
              content_type: kind === "statement" ? "text/csv; charset=utf-8" : "application/vnd.layerx.evidence-bundle",
              created_at: "2026-08-15T10:30:00Z", evidence: [],
            },
          };
          store.exports.set(identity, row);
        }
        row.deliveries++;
        store.exportRequests.push(row);
        if (store.dropNextExportResponse) {
          store.dropNextExportResponse = false;
          response.destroy();
          return;
        }
        store.success(response, encodeExportArtefact(row.artefact));
      } else if (incoming.method === "GET" && incoming.url?.startsWith("/v1/evidence/")) {
        const id = decodeURIComponent(incoming.url.slice("/v1/evidence/".length));
        store.evidenceRequests.push(id);
        const material = store.evidence.get(id);
        assert.ok(material, "only committed evidence fixtures are served");
        store.success(response, encodeEvidenceMaterial(material));
      } else {
        throw new Error(`Unrecorded route ${route}`);
      }
    } catch (error) {
      store.unexpected.push(String(error));
      response.writeHead(500).end(String(error));
    }
  });
  await new Promise<void>((resolve) => server.listen(0, "127.0.0.1", resolve));
  const address = server.address();
  assert.ok(address !== null && typeof address !== "string");
  const origin = `http://127.0.0.1:${address.port}`;
  const client = createHumanApiClient({
    baseUrl: origin, headers: { Cookie: "human_session=activity-fixture-session" },
  });
  t.after(async () => {
    await new Promise<void>((resolve) => {
      server.close(() => resolve());
      server.closeAllConnections();
    });
    assert.deepEqual(store.unexpected, []);
  });
  return { store, client, origin, controller: new ActivityController(client) };
}

async function until(check: () => boolean): Promise<void> {
  const deadline = Date.now() + 5_000;
  while (!check() && Date.now() < deadline) await new Promise((resolve) => setTimeout(resolve, 5));
  assert.ok(check(), "Activity HTTP path did not reach the expected state within five seconds");
}

async function refresh(controller: Controller, store: ActivityPeer, filter: ActivityFilter) {
  const index = store.queries.length;
  const loading = controller.refresh(filter);
  await until(() => store.queries.length === index + 1);
  store.release(index);
  await loading;
  assert.equal(controller.snapshot().load.kind, "loaded");
}

function renderedResults(controller: Controller, shell: "desktop" | "mobile"): string {
  const { load } = controller.snapshot();
  assert.equal(load.kind, "loaded");
  assert.ok(load.kind === "loaded");
  return renderToStaticMarkup(createElement(PrivacyModeProvider, {
    principalScope: load.principal,
    children: createElement(ActivityFilterResults, {
      page: load.page, agents: load.agents, shell, masked: false, onSelect() {},
    }),
  }));
}

function assertResults(controller: Controller, expected: readonly ActivityEntry[], filter: ActivityFilter) {
  const { load } = controller.snapshot();
  assert.ok(load.kind === "loaded");
  assert.deepEqual(load.page.filter, filter);
  assert.deepEqual(load.page.groups.flatMap((group) => group.entries.map((entry) => entry.entry_id)), expected.map((entry) => entry.entry_id));
  for (const shell of ["desktop", "mobile"] as const) {
    const html = renderedResults(controller, shell);
    const text = new browser.DOMParser().parseFromString(html, "text/html").body.textContent;
    for (const line of filterEchoLines(filter)) assert.ok(text.includes(line));
    for (const entry of expected) {
      assert.ok(text.includes(kindLabel(entry.kind)));
      assert.ok(text.includes(plainSentence(entry)));
    }
    assert.doesNotMatch(html, /activity-unavailable|connection-unavailable/u);
  }
}

async function activity_filter_response_race_pagination_and_rendered_results(t: TestContext) {
  const { controller, store } = await peer(t);
  await refresh(controller, store, {});
  const obsoletePage = controller.loadMore();
  await until(() => store.queries.length === 2);
  const movement: ActivityFilter = { kinds: ["movement"] };
  await refresh(controller, store, movement);
  const currentPage = controller.loadMore();
  await until(() => store.queries.length === 4);
  store.release(1);
  await obsoletePage;
  assert.equal(controller.snapshot().paging, true, "obsolete finally must not finish current pagination");
  const entries = store.entries.filter((entry) => entry.kind === "movement");
  assertResults(controller, entries.slice(0, 1), movement);
  store.release(3);
  await currentPage;
  assertResults(controller, entries, movement);
  assert.equal(controller.snapshot().paging, false);
  assert.equal(controller.snapshot().pagingFailure, undefined);
}

async function activity_filter_response_race_refresh_and_obsolete_failures(t: TestContext) {
  const { controller, store } = await peer(t);
  for (const failed of [false, true]) {
    const index = store.queries.length;
    const obsolete = controller.refresh({ kinds: ["security-event"] });
    await until(() => store.queries.length === index + 1);
    const deposit: ActivityFilter = { kinds: ["deposit"] };
    await refresh(controller, store, deposit);
    const current = controller.snapshot();
    store.release(index, failed);
    await obsolete;
    assert.equal(controller.snapshot(), current, "older refresh must not publish rows or failure");
    assertResults(controller, store.entries.filter((entry) => entry.kind === "deposit"), deposit);
  }
  await refresh(controller, store, {});
  const index = store.queries.length;
  const obsoletePage = controller.loadMore();
  await until(() => store.queries.length === index + 1);
  await refresh(controller, store, { kinds: ["withdrawal"] });
  const current = controller.snapshot();
  store.release(index, true);
  await obsoletePage;
  assert.equal(controller.snapshot(), current, "obsolete pagination failure and finally must be discarded");
}

async function activity_filter_response_race_account_and_unmount_invalidation(t: TestContext) {
  const { controller, store } = await peer(t);
  const obsolete = controller.refresh({ kinds: ["deposit"] });
  await until(() => store.queries.length === 1);
  controller.invalidateAccount();
  store.principal = "act_activityprincipalb";
  await refresh(controller, store, { kinds: ["withdrawal"] });
  const current = controller.snapshot();
  assert.ok(current.load.kind === "loaded");
  assert.equal(current.load.principal, store.principal);
  store.release(0);
  await obsolete;
  assert.equal(controller.snapshot(), current);
  assertResults(controller, store.entries.filter((entry) => entry.kind === "withdrawal"), { kinds: ["withdrawal"] });
  const index = store.queries.length;
  const unmounted = controller.refresh({});
  await until(() => store.queries.length === index + 1);
  controller.invalidateAccount();
  const stopped = controller.snapshot();
  store.release(index);
  await unmounted;
  assert.equal(controller.snapshot(), stopped, "effect cleanup invalidates outstanding work");
}

async function activity_export_payload_key_response_loss_filter_change_and_reconciliation(t: TestContext) {
  const { controller, store } = await peer(t);
  await refresh(controller, store, { kinds: ["deposit"] });
  store.dropNextExportResponse = true;
  await controller.requestExport("statement");
  assert.equal(controller.snapshot().exportState.kind, "failed");
  const original = store.exportRequests[0]!;
  assert.equal(store.exports.size, 1, "the service committed before its response was lost");
  const retained = controller.snapshot().pendingExports[0]!;
  assert.equal(retained.principal, store.principal);
  assert.deepEqual(retained.filter, { kinds: ["deposit"] });
  assert.ok(Object.isFrozen(retained) && Object.isFrozen(retained.filter) && Object.isFrozen(retained.filter.kinds));

  await refresh(controller, store, { kinds: ["movement"] });
  await controller.requestExport("statement");
  const changed = store.exportRequests[1]!;
  assert.notEqual(changed.key, original.key, "a distinct payload needs a new key");
  assert.notEqual(changed.payload, original.payload);
  assert.notEqual(changed.artefact.export_id, original.artefact.export_id);
  assert.equal(store.exports.size, 2);
  assert.deepEqual(controller.snapshot().pendingExports.map((attempt) => attempt.key), [original.key]);

  await controller.retryExport(original.key);
  const reconciled = store.exportRequests[2]!;
  assert.equal(reconciled, original, "retry resolves the stored original operation");
  assert.equal(original.deliveries, 2);
  assert.equal(store.exports.size, 2, "reconciliation does not create a second export");
  const state = controller.snapshot().exportState;
  assert.ok(state.kind === "ready");
  assert.equal(state.artefact.export_id, original.artefact.export_id);
  assert.deepEqual(state.attempt.filter, { kinds: ["deposit"] });
  assert.equal(controller.snapshot().pendingExports.length, 0);
  assert.equal(store.conflicts, 0);
}

async function activity_export_payload_key_kind_and_principal_isolation(t: TestContext) {
  const { controller, store } = await peer(t);
  await refresh(controller, store, {});
  for (const kind of ["statement", "evidence-bundle"] as const) {
    store.dropNextExportResponse = true;
    await controller.requestExport(kind);
    assert.equal(controller.snapshot().exportState.kind, "failed");
  }
  const attempts = controller.snapshot().pendingExports;
  assert.equal(attempts.length, 2);
  assert.notEqual(attempts[0]!.key, attempts[1]!.key);
  const oldPrincipal = store.principal;
  controller.invalidateAccount();
  store.principal = "act_activityprincipalb";
  await refresh(controller, store, {});
  assert.equal(controller.snapshot().pendingExports.length, 0);
  await controller.retryExport(attempts[0]!.key);
  assert.equal(store.exportRequests.length, 2, "a different principal cannot reconcile the old operation");
  await controller.requestExport("statement");
  assert.equal(store.exportRequests[2]!.principal, store.principal);
  assert.notEqual(store.exportRequests[2]!.key, attempts[0]!.key);
  controller.invalidateAccount();
  store.principal = oldPrincipal;
  await refresh(controller, store, {});
  assert.equal(controller.snapshot().pendingExports.length, 2);
  for (const attempt of attempts) await controller.retryExport(attempt.key);
  assert.equal(controller.snapshot().pendingExports.length, 0);
  assert.equal(store.exports.size, 3);
  assert.equal(store.conflicts, 0);
}

function nativeMaterial(): { reference: EvidenceRef; material: EvidenceMaterial; hash: string } {
  const fixture = JSON.parse(readFileSync(new URL(
    "../../../sdk/platform/conformance/fixtures/vector-v5-portable.json", import.meta.url,
  ), "utf8")) as { bundles: { rawTransaction: Hex; [key: string]: unknown }[] };
  const portable = fixture.bundles[0]!;
  const reference: EvidenceRef = {
    evidence_id: "evd_activitynative", class: "layerx-receipt", verification: "receipt-verified",
  };
  return {
    reference, hash: keccak256(portable.rawTransaction),
    material: {
      ...reference, content_type: "application/vnd.layerx.receipt-proof-v2+json",
      bytes_base64: Buffer.from(JSON.stringify(portable)).toString("base64"),
    },
  };
}

async function activity_evidence_dead_route_material_to_registered_native_url(t: TestContext) {
  const { store, client, origin } = await peer(t);
  const { reference, material, hash } = nativeMaterial();
  store.evidence.set(reference.evidence_id, material);
  assert.equal(explorerPath(reference), undefined, "an evidence id alone is never an explorer identifier");
  assert.equal(await resolveEvidencePath(client, reference, origin), `${origin}/tx/${hash}`);
  assert.equal(await resolveEvidencePath(client, reference), undefined, "no configured destination means no link");
  assert.equal(await resolveEvidencePath(client, reference, "https://example.invalid/injected/path"), undefined);
  const opaque: EvidenceMaterial = { ...material, content_type: "application/vnd.layerx.receipt" };
  store.evidence.set(reference.evidence_id, opaque);
  assert.equal(await resolveEvidencePath(client, reference, origin), undefined, "opaque receipt bytes have no declared native location");
  const checkpoint: EvidenceRef = { evidence_id: "evd_activitycheckpoint", class: "checkpoint-proof", verification: "checkpoint-finalised" };
  store.evidence.set(checkpoint.evidence_id, { ...material, ...checkpoint });
  assert.equal(await resolveEvidencePath(client, checkpoint, origin), undefined);
  store.evidence.set(reference.evidence_id, { ...material, evidence_id: "evd_differentresource" });
  await assert.rejects(resolveEvidencePath(client, reference, origin), /does not match/u);
  assert.equal(store.evidenceRequests.length, 6, "all resolutions use the real evidence.get transport and decoder");
}

async function activity_evidence_dead_route_rendered_link_and_no_destination(t: TestContext) {
  const { store, client, origin } = await peer(t);
  const { reference, material, hash } = nativeMaterial();
  const checkpoint: EvidenceRef = { evidence_id: "evd_activitycheckpoint", class: "checkpoint-proof", verification: "checkpoint-finalised" };
  store.evidence.set(reference.evidence_id, material);
  store.evidence.set(checkpoint.evidence_id, { ...material, ...checkpoint });
  const container = browser.document.createElement("div");
  browser.document.body.appendChild(container);
  const root = createRoot(container as unknown as HTMLElement);
  t.after(async () => { await act(async () => root.unmount()); container.remove(); });
  async function render(next: EvidenceRef) {
    await act(async () => {
      root.render(createElement(ActivityEvidenceLink, { client, reference: next, configuredOrigin: origin }));
    });
    await act(async () => {
      await until(() => store.evidenceRequests.includes(next.evidence_id));
      await new Promise((resolve) => setTimeout(resolve, 30));
    });
  }
  await render(reference);
  assert.equal(container.querySelector("a")?.getAttribute("href"), `${origin}/tx/${hash}`);
  await render(checkpoint);
  assert.equal(container.querySelector("a"), null);
  assert.doesNotMatch(container.innerHTML, /\/explorer\/(checkpoints|receipts)\//u);
  assert.deepEqual(store.evidenceRequests, [reference.evidence_id, checkpoint.evidence_id]);
}

test(activity_filter_response_race_pagination_and_rendered_results);
test(activity_filter_response_race_refresh_and_obsolete_failures);
test(activity_filter_response_race_account_and_unmount_invalidation);
test(activity_export_payload_key_response_loss_filter_change_and_reconciliation);
test(activity_export_payload_key_kind_and_principal_isolation);
test(activity_evidence_dead_route_material_to_registered_native_url);
test(activity_evidence_dead_route_rendered_link_and_no_destination);
