import assert from "node:assert/strict";
import { existsSync, readFileSync } from "node:fs";
import { createServer } from "node:http";
import { registerHooks } from "node:module";
import type { AddressInfo } from "node:net";
import test from "node:test";
import { fileURLToPath } from "node:url";
import ts from "typescript";

import type { HumanApiClient, JsonObject } from "../src/api/generated/index.ts";

registerHooks({
  resolve(specifier, context, nextResolve) {
    if (specifier.startsWith(".") && context.parentURL !== undefined) {
      const candidate = new URL(specifier, context.parentURL);
      for (const suffix of [".ts", ".tsx", "/index.ts"]) {
        const source = candidate.href.endsWith(".js") ? new URL(candidate.href.slice(0, -3) + suffix) : new URL(candidate.href + suffix);
        if (existsSync(source)) return nextResolve(source.href, context);
      }
    }
    if (specifier === "server-only") return { url: "data:text/javascript,", shortCircuit: true };
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

const api = await import("../src/api/generated/index.ts");
const { loadSettingsSnapshot } = await import("../src/settings/settings-screen.tsx");

interface GoldenResponse {
  readonly status: number;
  readonly body: JsonObject;
}

function goldenResponse(operation: string, kind: "response" | "failure" = "response"): GoldenResponse {
  const path = new URL(`../../../services/human/schema/human-api/golden/${operation}.${kind}.json`, import.meta.url);
  return JSON.parse(readFileSync(path, "utf8")) as GoldenResponse;
}

const profile = goldenResponse("profile.get");
const notifications = goldenResponse("notification.preferences.get");
const passkeys = goldenResponse("security.passkey.list");
const sessions = goldenResponse("session.list");
const retiredBinding = goldenResponse("binding.status", "failure");
const snapshotRequests = [
  "GET /v1/profile",
  "GET /v1/notifications/preferences",
  "GET /v1/security/passkeys",
  "GET /v1/sessions",
].sort();
const expectedSnapshot = {
  profile: api.decodeProfile(profile.body["result"], "profile"),
  notifications: api.decodeNotificationPreferences(notifications.body["result"], "notifications"),
  passkeyCount: api.decodePasskeyList(passkeys.body["result"], "passkeys").passkeys.length,
  sessionCount: api.decodeSessionList(sessions.body["result"], "sessions").sessions.length,
};

async function withGoldenHttp(run: (client: HumanApiClient, requests: string[], baseUrl: string) => Promise<void>): Promise<void> {
  const responses = new Map<string, GoldenResponse>([
    ["GET /v1/profile", profile],
    ["GET /v1/notifications/preferences", notifications],
    ["GET /v1/security/passkeys", passkeys],
    ["GET /v1/sessions", sessions],
    ["GET /v1/wallet-binding", retiredBinding],
  ]);
  const requests: string[] = [];
  const server = createServer((request, response) => {
    const key = `${request.method} ${request.url}`;
    requests.push(key);
    const golden = responses.get(key);
    assert.ok(golden, `Unexpected settings request: ${key}`);
    response.writeHead(golden.status, { "content-type": "application/json" });
    response.end(JSON.stringify(golden.body));
  });
  await new Promise<void>((resolve, reject) => {
    server.once("error", reject);
    server.listen(0, "127.0.0.1", resolve);
  });
  try {
    const address = server.address() as AddressInfo;
    const baseUrl = `http://127.0.0.1:${address.port}`;
    await run(api.createHumanApiClient({ baseUrl }), requests, baseUrl);
  } finally {
    await new Promise<void>((resolve, reject) => {
      server.close((error) => { if (error) reject(error); else resolve(); });
    });
  }
}

async function settings_snapshot_loads_without_binding_status() {
  await withGoldenHttp(async (client, requests) => {
    const snapshot = await loadSettingsSnapshot(client);
    assert.deepEqual(snapshot, expectedSnapshot);
    assert.deepEqual(requests.sort(), snapshotRequests);
    assert.equal(Object.hasOwn(snapshot, "binding"), false);
  });
}

async function settings_snapshot_survives_operation_retired() {
  await withGoldenHttp(async (client, requests, baseUrl) => {
    const response = await fetch(`${baseUrl}/v1/wallet-binding`);
    assert.equal(response.status, 410);
    const body = await response.json() as { readonly error: { readonly code: string } };
    assert.equal(body.error.code, "operation-retired");
    assert.deepEqual(body, retiredBinding.body);
    assert.deepEqual(requests, ["GET /v1/wallet-binding"]);
    requests.length = 0;
    const snapshot = await loadSettingsSnapshot(client);
    assert.deepEqual(snapshot, expectedSnapshot);
    assert.deepEqual(requests.sort(), snapshotRequests);
    assert.equal(Object.hasOwn(snapshot, "binding"), false);
  });
}

test(settings_snapshot_loads_without_binding_status);
test(settings_snapshot_survives_operation_retired);
