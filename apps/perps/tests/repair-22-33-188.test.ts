import assert from "node:assert/strict";
import { once } from "node:events";
import { readFileSync } from "node:fs";
import { createServer } from "node:http";
import type { AddressInfo } from "node:net";
import { after, test, type TestContext } from "node:test";
import type { ErrorEvent } from "@sentry/nextjs";

const fixtureEnvironment = {
  NEXT_PUBLIC_PERPS_GATEWAY_URL: "https://gateway.fixture.test",
  NEXT_PUBLIC_PAXSCAN_BASE_URL: "https://explorer.fixture.test",
  NEXT_PUBLIC_PAXSCAN_API_URL: "https://explorer.fixture.test/api/v2",
  NEXT_PUBLIC_PAXSCAN_WS_URL: "wss://explorer.fixture.test/socket",
};
const savedEnvironment = new Map(
  Object.keys(fixtureEnvironment).map((key) => [key, process.env[key]]),
);
Object.assign(process.env, fixtureEnvironment);
after(() => {
  for (const [key, value] of savedEnvironment) {
    if (value === undefined) delete process.env[key];
    else process.env[key] = value;
  }
});

const secrets = {
  prefixedKey: `0x${"ab".repeat(32)}`,
  bareKey: "cd".repeat(32),
  address: `0x${"eF".repeat(20)}`,
  nativeAddress: `pax1${"q".repeat(38)}`,
  mnemonic: `${"abandon ".repeat(11)}about`,
  bearer: "fixture-bearer-188",
  password: "fixture-password-188",
  seed: "fixture-seed-188",
};

function secretBearingEvent(runtime: string): ErrorEvent {
  const text = `${secrets.prefixedKey} ${secrets.bareKey} ${secrets.address} ${secrets.nativeAddress}`;
  return {
    type: undefined,
    message: `repair-188-${runtime} ${text}`,
    level: "error",
    exception: {
      values: [{ type: "FixtureFailure", value: `failed ${text} ${secrets.mnemonic}` }],
    },
    request: {
      method: "POST",
      url: `https://gateway.fixture.test/orders?wallet=${secrets.address}&token=${secrets.bearer}`,
      headers: { Authorization: `Bearer ${secrets.bearer}`, "X-Safe": "application/json" },
      data: {
        operation: "submit",
        nested: { "Private_Key": secrets.prefixedKey, "Recovery Seed": secrets.seed },
        body: JSON.stringify({ password: secrets.password }),
      },
    },
    breadcrumbs: [{
      category: "order",
      message: `signer ${text}`,
      data: { nested: [{ Mnemonic: secrets.mnemonic, safe: "breadcrumb-kept" }] },
    }],
    user: { id: secrets.address, ip_address: "192.0.2.188" },
    tags: { operation: "place-order", signer: secrets.address },
    contexts: { order: { status: "rejected", nested: { access_token: secrets.bearer } } },
    extra: {
      text,
      nested: [{ Password: secrets.password, note: secrets.mnemonic }],
      [secrets.address]: "identifier-key",
      safe: "diagnostic-kept",
    },
  };
}

function assertNoSecrets(value: unknown): void {
  const serialized = JSON.stringify(value);
  for (const secret of Object.values(secrets)) {
    assert.equal(serialized.includes(secret), false, "secret must not leave the sanitizer");
  }
  assert.equal(serialized.includes("192.0.2.188"), false);
  assert.match(serialized, /\[redacted\]/);
}

async function assertRuntimeTransport(runtime: "client" | "edge", t: TestContext): Promise<void> {
  const envelopes: string[] = [];
  const receiver = createServer((request, response) => {
    let body = "";
    request.setEncoding("utf8");
    request.on("data", (chunk: string) => { body += chunk; });
    request.on("end", () => {
      envelopes.push(body);
      response.writeHead(200, { "content-type": "application/json" });
      response.end("{}");
    });
  });
  receiver.listen(0, "127.0.0.1");
  await once(receiver, "listening");
  const previous = [process.env.SENTRY_DSN, process.env.NEXT_PUBLIC_SENTRY_DSN];
  const dsn = `http://fixture@127.0.0.1:${(receiver.address() as AddressInfo).port}/1`;
  process.env.SENTRY_DSN = dsn;
  process.env.NEXT_PUBLIC_SENTRY_DSN = dsn;
  const Sentry = await import("@sentry/nextjs");
  t.after(async () => {
    await Sentry.close(5_000);
    Sentry.getCurrentScope().setClient(undefined);
    for (const [index, key] of ["SENTRY_DSN", "NEXT_PUBLIC_SENTRY_DSN"].entries()) {
      const value = previous[index];
      if (value === undefined) delete process.env[key];
      else process.env[key] = value;
    }
    receiver.closeAllConnections();
    await new Promise<void>((resolve, reject) => {
      receiver.close((error) => error ? reject(error) : resolve());
    });
  });
  if (runtime === "client") await import("../sentry.client.config");
  else await import("../sentry.edge.config");

  const { sanitizeSentryEvent } = await import("../src/core/errors/sentrySanitizer");
  const options = Sentry.getClient()!.getOptions();
  assert.equal(options.beforeSend, sanitizeSentryEvent);
  assert.equal(options.sendDefaultPii, false);
  assert.equal(options.tracesSampleRate, 0);

  const input = secretBearingEvent(runtime);
  const sanitized = sanitizeSentryEvent(input, {});
  assert.ok(sanitized);
  assertNoSecrets(sanitized);
  assert.equal(input.message, secretBearingEvent(runtime).message, "sanitize without mutating the caller");
  assert.equal(sanitized.request?.method, "POST");
  assert.equal(sanitized.extra?.safe, "diagnostic-kept");
  assert.equal(sanitized.tags?.operation, "place-order");
  assert.match(JSON.stringify(sanitized.breadcrumbs), /breadcrumb-kept/);
  assert.equal(sanitizeSentryEvent(input, { originalException: { name: "AbortError" } }), null);

  const id = Sentry.captureEvent(input);
  assert.equal(await Sentry.flush(5_000), true);
  const envelope = envelopes.find((body) => body.includes(`"event_id":"${id}"`));
  assert.ok(envelope, "the real SDK must deliver the sanitized event to the local HTTP receiver");
  const sent = envelope.split("\n").map((line) => JSON.parse(line)).find(
    (item): item is ErrorEvent => item.event_id === id && item.message !== undefined,
  );
  assert.ok(sent);
  assertNoSecrets(sent);
  assert.equal(sent.extra?.safe, "diagnostic-kept");
  assert.equal(sent.tags?.operation, "place-order");
  assert.equal(sent.exception?.values?.[0].type, "FixtureFailure");
  assert.equal(sent.request?.method, "POST");

  const count = envelopes.length;
  Sentry.captureException(Object.assign(new Error(`abort-${runtime}`), { name: "AbortError" }));
  assert.equal(await Sentry.flush(5_000), true);
  assert.equal(envelopes.length, count, "AbortError remains suppressed at the transport boundary");
}

async function f001ClientSanitizesEntireSentryEvent(t: TestContext): Promise<void> {
  await assertRuntimeTransport("client", t);
}

async function f002EdgeUsesSharedSentrySanitizer(t: TestContext): Promise<void> {
  await assertRuntimeTransport("edge", t);
}

async function f044DocumentedStatsMatchWireAndConsumer(): Promise<void> {
  const guide = readFileSync(new URL("../../../docs/perps/data-api.md", import.meta.url), "utf8");
  const records = readFileSync(new URL("../../../services/platform/perps-data/internal/records/records.go", import.meta.url), "utf8");
  const { parseSidPaxStats } = await import("../src/hooks/perps/useSidPaxMarketStream");
  const { parseFundingRateHourly, fundingRowFigure, fundingRateFromJsonNumber } = await import("../src/core/perps/fundingRate");
  const fields = ["Stats", "PerpStats"].flatMap((name) => {
    const block = records.match(new RegExp(`type ${name} struct \\{([\\s\\S]*?)\\n\\}`));
    assert.ok(block);
    return [...block[1].matchAll(/json:"([^"]+)"/g)].map((field) => field[1]);
  }).sort();
  for (const heading of ["Perpetual stats wire example", "Perpetual stats before settlement"]) {
    const section = guide.split(`#### ${heading}\n`)[1];
    assert.ok(section, "guide must document both settled and unavailable wire examples");
    const example = section.match(/```json\n([\s\S]*?)\n```/);
    assert.ok(example);
    const wire = JSON.parse(example[1]);
    assert.deepEqual(Object.keys(wire).sort(), fields);
    const stats = parseSidPaxStats(wire, wire.symbol);
    assert.ok(stats, "the published example must decode through the active production adapter");
    assert.equal(stats.volume_24h_usd, wire.volume_24h);
    assert.equal(stats.oi_usd, wire.open_interest);
    assert.equal(stats.last_funding_rate, fundingRateFromJsonNumber(wire.last_funding_rate));
    assert.equal(stats.funding_rate, null);
    const hourly = wire.last_funding_rate_hourly === null ? null : parseFundingRateHourly(wire.last_funding_rate_hourly);
    assert.equal(fundingRowFigure(stats.funding_rate, hourly).label, "last settled funding (1h)");
    assert.equal(fundingRowFigure(stats.funding_rate, hourly).text, wire.symbol === "BTC" ? "+0.0013%" : "—");
    assert.equal(stats.last_funding_rate, wire.last_funding_rate_hourly);
  }
  assert.match(guide, /Optional external reference estimates use `funding_rate`/);
  assert.match(guide, /`volume_24h_usd` and `oi_usd` are client aliases/);
  assert.match(guide, /projection of the\nlast settlement, not an estimate/);
}

test(f001ClientSanitizesEntireSentryEvent);
test(f002EdgeUsesSharedSentrySanitizer);
test(f044DocumentedStatsMatchWireAndConsumer);
