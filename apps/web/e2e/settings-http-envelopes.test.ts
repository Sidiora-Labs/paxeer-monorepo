import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import { createServer } from "node:http";
import test, { type TestContext } from "node:test";

import { request } from "@playwright/test";

import { readSettingsResponse } from "./browser/settings-http.ts";
import {
  decodeNotificationPreferences,
  decodeProfile,
  encodeNotificationPreferences,
  encodeProfile,
  encodeProfileUpdate,
  encodeResponseEnvelope,
  expectObject,
  notificationClassVariants,
  type JsonObject,
  type JsonValue,
  type NotificationPreferences,
  type Profile,
} from "../src/api/generated/index.ts";

interface HttpFixture {
  body: JsonValue;
  status?: number;
  rawBody?: string;
}

function success(result: JsonValue): JsonObject {
  return expectObject(encodeResponseEnvelope({
    ok: true,
    trace: "settings-http-envelope-fixture",
    result: expectObject(result, "fixture result"),
  }), "fixture envelope");
}

async function openFixture(t: TestContext, fixtures: ReadonlyMap<string, HttpFixture>) {
  const observed: string[] = [];
  const cookies: (string | undefined)[] = [];
  const server = createServer((incoming, outgoing) => {
    const key = `${incoming.method} ${incoming.url}`;
    observed.push(key);
    cookies.push(incoming.headers.cookie);
    incoming.resume();
    const fixture = fixtures.get(key);
    if (fixture === undefined) {
      outgoing.writeHead(404).end("unrecorded settings request");
      return;
    }
    outgoing.writeHead(fixture.status ?? 200, { "Content-Type": "application/json" });
    outgoing.end(fixture.rawBody ?? JSON.stringify(fixture.body));
  });
  await new Promise<void>((resolve) => server.listen(0, "127.0.0.1", resolve));
  const address = server.address();
  assert.ok(address !== null && typeof address !== "string");
  const context = await request.newContext({
    baseURL: `http://127.0.0.1:${address.port}`,
    storageState: {
      cookies: [{
        name: "human_session",
        value: "settings-recorded-session",
        domain: "127.0.0.1",
        path: "/",
        expires: -1,
        httpOnly: true,
        secure: false,
        sameSite: "Lax",
      }],
      origins: [],
    },
  });
  t.after(async () => {
    await context.dispose();
    await new Promise<void>((resolve, reject) => {
      server.close((error) => error === undefined ? resolve() : reject(error));
    });
  });
  return { context, observed, cookies };
}

async function settings_http_envelope_profile_result(t: TestContext) {
  const original: Profile = { display_name: "Settings profile qualification" };
  const avatarUrl = "https://example.invalid/avatar.png";
  const added: Profile = { ...original, avatar_url: avatarUrl };
  const fixtures = new Map<string, HttpFixture>([
    ["GET /human/v1/profile", { body: success(encodeProfile(original)) }],
    ["PATCH /human/v1/profile?avatar=added", { body: success(encodeProfile(added)) }],
    ["PATCH /human/v1/profile?avatar=cleared", { body: success(encodeProfile(original)) }],
  ]);
  const { context, observed, cookies } = await openFixture(t, fixtures);
  const initial = await readSettingsResponse(
    await context.get("/human/v1/profile"), decodeProfile, "authenticated profile",
  );
  assert.deepEqual(initial, original);
  const saved = await readSettingsResponse(
    await context.patch("/human/v1/profile?avatar=added", {
      data: encodeProfileUpdate({ avatar_url: avatarUrl }),
    }), decodeProfile, "saved local profile",
  );
  assert.deepEqual(saved, added);
  const cleared = await readSettingsResponse(
    await context.patch("/human/v1/profile?avatar=cleared", {
      data: encodeProfileUpdate({ avatar_url: "" }),
    }), decodeProfile, "cleared local profile",
  );
  assert.deepEqual(cleared, original);
  assert.equal(Object.hasOwn(cleared, "avatar_url"), false);
  const persisted = await readSettingsResponse(
    await context.get("/human/v1/profile"), decodeProfile, "persisted local profile",
  );
  assert.deepEqual(persisted, cleared);
  assert.deepEqual(observed, [...fixtures.keys(), "GET /human/v1/profile"]);
  assert.deepEqual(cookies, observed.map(() => "human_session=settings-recorded-session"));
}

function preferences(detail: NotificationPreferences["detail"]): NotificationPreferences {
  const channel = () => ({
    enabled: true,
    classes: notificationClassVariants.map((notificationClass) => ({
      class: notificationClass,
      enabled: true,
    })),
  });
  return { push: channel(), email: channel(), in_app: channel(), detail };
}

async function settings_http_envelope_preferences_result(t: TestContext) {
  const fixtures = new Map<string, HttpFixture>();
  for (const detail of ["full", "minimal", "summary"] as const) {
    fixtures.set(`POST /human/v1/notifications/preferences?detail=${detail}`, {
      body: success(encodeNotificationPreferences(preferences(detail))),
    });
  }
  const { context, observed, cookies } = await openFixture(t, fixtures);
  for (const detail of ["full", "minimal", "summary"] as const) {
    const decoded = await readSettingsResponse(
      await context.post(`/human/v1/notifications/preferences?detail=${detail}`, {
        data: encodeNotificationPreferences(preferences(detail)),
      }), decodeNotificationPreferences, "saved detail",
    );
    assert.equal(decoded.detail, detail);
    assert.deepEqual(decoded, preferences(detail));
    for (const notificationClass of ["security-recovery", "security-wallet-rebinding"] as const) {
      assert.ok(decoded.push.classes.some((entry) => entry.class === notificationClass && entry.enabled));
    }
  }
  assert.deepEqual(observed, [...fixtures.keys()]);
  assert.deepEqual(cookies, observed.map(() => "human_session=settings-recorded-session"));
}

async function proveMalformed<Result>(
  t: TestContext,
  result: JsonValue,
  decode: (value: JsonValue | undefined, at: string) => Result,
) {
  const missingOk = success(result);
  delete missingOk.ok;
  const missingTrace = success(result);
  delete missingTrace.trace;
  const missingResult = success(result);
  delete missingResult.result;
  const cases: readonly [string, HttpFixture, RegExp][] = [
    ["bare-result", { body: result }, /\.ok is missing/u],
    ["missing-ok", { body: missingOk }, /\.ok is missing/u],
    ["invalid-ok", { body: { ...success(result), ok: "true" } }, /\.ok must be a JSON boolean/u],
    ["service-failure", { body: { ...success(result), ok: false } }, /service request must succeed/u],
    ["missing-trace", { body: missingTrace }, /\.trace is missing/u],
    ["invalid-trace", { body: { ...success(result), trace: 42 } }, /\.trace must be a JSON string/u],
    ["missing-result", { body: missingResult }, /\.result is missing/u],
    ["null-result", { body: { ...success(result), result: null } }, /\.result must be a JSON object/u],
    ["invalid-result", { body: { ...success(result), result: {} } }, /\.result\.(display_name|push) is missing/u],
    ["http-failure", { body: success(result), status: 503 }, /HTTP request must succeed/u],
    ["invalid-json", { body: null, rawBody: "{" }, /JSON/u],
  ];
  const fixtures = new Map(cases.map(([name, fixture]) => [`GET /human/v1/${name}`, fixture]));
  const { context, observed, cookies } = await openFixture(t, fixtures);
  for (const [name, , failure] of cases) {
    const response = await context.get(`/human/v1/${name}`);
    await assert.rejects(readSettingsResponse(response, decode, name), failure, name);
  }
  assert.deepEqual(observed, [...fixtures.keys()]);
  assert.deepEqual(cookies, observed.map(() => "human_session=settings-recorded-session"));
}

async function settings_http_envelope_malformed_profile(t: TestContext) {
  await proveMalformed(t, encodeProfile({ display_name: "Settings profile" }), decodeProfile);
}

async function settings_http_envelope_malformed_preferences(t: TestContext) {
  await proveMalformed(t, encodeNotificationPreferences(preferences("summary")), decodeNotificationPreferences);
}

function settings_http_envelope_browser_wiring() {
  const source = readFileSync(new URL("./browser/settings.spec.ts", import.meta.url), "utf8");
  assert.match(source, /import \{ readSettingsResponse \} from "\.\/settings-http\.ts"/u);
  assert.doesNotMatch(source, /\.json\(|\.ok\(/u, "direct settings responses must use the validated reader");
}

test(settings_http_envelope_profile_result);
test(settings_http_envelope_preferences_result);
test(settings_http_envelope_malformed_profile);
test(settings_http_envelope_malformed_preferences);
test(settings_http_envelope_browser_wiring);
