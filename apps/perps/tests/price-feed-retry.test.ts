import assert from "node:assert/strict";
import test, { type TestContext } from "node:test";
import { setImmediate } from "node:timers/promises";
import type { NextApiRequest, NextApiResponse } from "next";
import priceFeed from "../apps/web/src/pages/api/price-feed/[...path]";

const restKey = "price-feed-rest-test-credential";
const mintKey = "price-feed-mint-test-credential";
const socketToken = "price-feed-scoped-test-token";
const feedReply = { s: "no_data" };
const tokenReply = {
  token: socketToken,
  symbol: "eth",
  expiresAt: "2026-01-01T00:05:00.000Z",
  expiresIn: 300,
};

function request(path: string[] = ["markets", "tickers"], extra = {}) {
  return {
    method: "GET",
    query: { path },
    url: "/api/price-feed/" + path.join("/"),
    ...extra,
  } as NextApiRequest;
}

function mintRequest() {
  return request(["ws-token"], {
    query: { path: ["ws-token"], symbol: "eth" },
  });
}

function harness(
  t: TestContext,
  fetch: typeof globalThis.fetch,
  req = request(),
) {
  t.mock.timers.enable({ apis: ["setTimeout", "Date"], now: 0 });
  const previous = {
    PRICE_FEED_REST_API_KEY: process.env.PRICE_FEED_REST_API_KEY,
    PRICE_FEED_WS_API_KEY: process.env.PRICE_FEED_WS_API_KEY,
    PRICE_FEED_ORIGIN: process.env.PRICE_FEED_ORIGIN,
  };
  process.env.PRICE_FEED_REST_API_KEY = restKey;
  process.env.PRICE_FEED_WS_API_KEY = mintKey;
  delete process.env.PRICE_FEED_ORIGIN;
  t.after(() => {
    for (const [name, value] of Object.entries(previous)) {
      if (value === undefined) delete process.env[name];
      else process.env[name] = value;
    }
  });
  const logs: unknown[][] = [];
  for (const method of ["debug", "info", "log", "warn", "error"] as const) {
    t.mock.method(console, method, (...args: unknown[]) => logs.push(args));
  }
  t.after(() => assert.deepEqual(logs, []));
  const attempts: { time: number; signal: AbortSignal }[] = [];
  t.mock.method(globalThis, "fetch", (input, init) => {
    assert.equal(init?.redirect, "error");
    assert.ok(init?.signal);
    assert.equal(init.signal.aborted, false);
    const target = new URL(String(input));
    const mint = target.pathname === "/auth/ws-token";
    assert.equal(
      new Headers(init.headers).get("X-API-Key"),
      mint ? mintKey : restKey,
    );
    assert.equal(target.searchParams.has("api_key"), false);
    if (mint) {
      assert.equal(init.method, "POST");
      assert.deepEqual(JSON.parse(String(init.body)), { symbol: "eth" });
    }
    attempts.push({ time: Date.now(), signal: init.signal });
    return fetch(input, init);
  });
  const result = {
    status: 0,
    body: undefined as unknown,
    headers: new Map<string, string>(),
  };
  const res = {
    setHeader(name: string, value: string) {
      result.headers.set(name.toLowerCase(), value);
      return this;
    },
    status(status: number) {
      result.status = status;
      return this;
    },
    json(body: unknown) {
      result.body = body;
      return this;
    },
    send(body: unknown) {
      result.body = body;
      return this;
    },
    end() {
      return this;
    },
  } as unknown as NextApiResponse;
  const completion = priceFeed(req, res);
  async function advance(ms: number) {
    t.mock.timers.tick(ms);
    await setImmediate();
  }
  return { result, attempts, completion, advance };
}

function assertFailure(
  result: { status: number; body: unknown; headers: Map<string, string> },
  status: number,
) {
  assert.equal(result.status, status);
  assert.deepEqual(result.body, { error: "feed_request_failed" });
  assert.equal(result.headers.get("cache-control"), "no-store");
}

async function connection_error_then_answer_uses_two_attempts(t: TestContext) {
  let calls = 0;
  const h = harness(t, async () => {
    if (++calls === 1) throw new TypeError(restKey);
    return Response.json(feedReply);
  });
  await setImmediate();
  assert.equal(h.attempts.length, 1);
  await h.advance(199);
  assert.equal(h.attempts.length, 1);
  await h.advance(1);
  await h.completion;
  assert.equal(h.result.status, 200);
  assert.deepEqual(h.result.body, feedReply);
  assert.deepEqual(
    h.attempts.map((attempt) => attempt.time),
    [0, 200],
  );
}
test(connection_error_then_answer_uses_two_attempts);

async function gateway_statuses_retry_after_a_pause(t: TestContext) {
  for (const status of [502, 503, 504]) {
    await t.test(String(status), async (t) => {
      let calls = 0;
      let discarded: Response | undefined;
      const h = harness(t, async () => {
        if (++calls === 1) {
          discarded = Response.json(
            { error: restKey },
            {
              status,
              headers: { "x-should-retry": "false", "Retry-After": "60" },
            },
          );
          return discarded;
        }
        return Response.json(feedReply);
      });
      await setImmediate();
      assert.equal(h.attempts.length, 1);
      assert.equal(discarded?.bodyUsed, true);
      await h.advance(200);
      await h.completion;
      assert.equal(h.result.status, 200);
      assert.deepEqual(h.result.body, feedReply);
      assert.deepEqual(
        h.attempts.map((attempt) => attempt.time),
        [0, 200],
      );
    });
  }
}
test(gateway_statuses_retry_after_a_pause);

async function other_statuses_preserve_the_error_without_retry(t: TestContext) {
  for (const status of [
    301, 400, 401, 403, 404, 408, 409, 422, 429, 500, 501,
  ]) {
    await t.test(String(status), async (t) => {
      const h = harness(t, async () =>
        Response.json(
          { error: `${restKey} ${mintKey} ${socketToken}` },
          { status, headers: { "x-should-retry": "true" } },
        ),
      );
      await h.completion;
      assertFailure(h.result, status);
      await h.advance(12_000);
      assert.equal(h.attempts.length, 1);
    });
  }
}
test(other_statuses_preserve_the_error_without_retry);

async function repeated_failures_stop_after_three_attempts(t: TestContext) {
  for (const status of [undefined, 502, 503, 504]) {
    await t.test(String(status ?? "connection"), async (t) => {
      const h = harness(t, async () => {
        if (status === undefined) throw new TypeError(restKey);
        return Response.json({ error: restKey }, { status });
      });
      await setImmediate();
      await h.advance(200);
      await h.advance(200);
      await h.completion;
      assertFailure(h.result, status ?? 502);
      assert.deepEqual(
        h.attempts.map((attempt) => attempt.time),
        [0, 200, 400],
      );
      await h.advance(12_000);
      assert.equal(h.attempts.length, 3);
    });
  }
}
test(repeated_failures_stop_after_three_attempts);

async function timed_out_attempt_retries_with_a_fresh_signal(t: TestContext) {
  let calls = 0;
  const h = harness(t, async (_input, init) => {
    if (++calls > 1) return Response.json(feedReply);
    return new Promise<Response>((_resolve, reject) => {
      init?.signal?.addEventListener(
        "abort",
        () => reject(init.signal?.reason),
        {
          once: true,
        },
      );
    });
  });
  await setImmediate();
  await h.advance(3_799);
  assert.equal(h.attempts.length, 1);
  assert.equal(h.attempts[0].signal.aborted, false);
  await h.advance(1);
  assert.equal(h.attempts[0].signal.aborted, true);
  await h.advance(199);
  assert.equal(h.attempts.length, 1);
  await h.advance(1);
  await h.completion;
  assert.equal(h.result.status, 200);
  assert.deepEqual(h.result.body, feedReply);
  assert.deepEqual(
    h.attempts.map((attempt) => attempt.time),
    [0, 4_000],
  );
  assert.notEqual(h.attempts[0].signal, h.attempts[1].signal);
}
test(timed_out_attempt_retries_with_a_fresh_signal);

async function all_timeouts_finish_within_twelve_seconds(t: TestContext) {
  const h = harness(t, () => new Promise<Response>(() => {}));
  await setImmediate();
  await h.advance(3_800);
  await h.advance(200);
  await h.advance(3_800);
  await h.advance(200);
  await h.advance(3_800);
  await h.completion;
  assertFailure(h.result, 502);
  assert.deepEqual(
    h.attempts.map((attempt) => attempt.time),
    [0, 4_000, 8_000],
  );
  assert.ok(h.attempts.every((attempt) => attempt.signal.aborted));
  assert.equal(Date.now(), 11_800);
  assert.ok(Date.now() <= 12_000);
  await h.advance(12_000);
  assert.equal(h.attempts.length, 3);
}
test(all_timeouts_finish_within_twelve_seconds);

async function socket_token_mint_retries_without_changing_its_scope(
  t: TestContext,
) {
  for (const status of [undefined, 503]) {
    await t.test(String(status ?? "connection"), async (t) => {
      let calls = 0;
      const h = harness(
        t,
        async () => {
          if (++calls === 1) {
            if (status === undefined) throw new TypeError(mintKey);
            return Response.json({ error: socketToken }, { status });
          }
          return Response.json(tokenReply);
        },
        mintRequest(),
      );
      await setImmediate();
      await h.advance(200);
      await h.completion;
      assert.equal(h.result.status, 200);
      assert.equal(h.attempts.length, 2);
      assert.deepEqual(h.result.body, {
        ...tokenReply,
        url: "wss://data.feed.chainflowtrading.com/ws/eth",
      });
      assert.equal(h.result.headers.get("cache-control"), "no-store");
    });
  }
}
test(socket_token_mint_retries_without_changing_its_scope);

async function socket_token_errors_preserve_the_status_and_hide_credentials(
  t: TestContext,
) {
  for (const status of [undefined, 400, 401, 503]) {
    await t.test(String(status ?? "connection"), async (t) => {
      const h = harness(
        t,
        async () => {
          if (status === undefined) throw new TypeError(mintKey);
          return Response.json(
            { error: `${mintKey} ${socketToken}` },
            { status },
          );
        },
        mintRequest(),
      );
      await setImmediate();
      if (status === undefined || status === 503) {
        await h.advance(200);
        await h.advance(200);
      }
      await h.completion;
      assertFailure(h.result, status ?? 502);
      assert.equal(
        h.attempts.length,
        status === undefined || status === 503 ? 3 : 1,
      );
    });
  }
}
test(socket_token_errors_preserve_the_status_and_hide_credentials);

async function refused_requests_make_no_attempt(t: TestContext) {
  const cases: [NextApiRequest, number, string][] = [
    [
      request(["markets", "tickers"], { method: "POST" }),
      405,
      "method_not_allowed",
    ],
    [request(["markets", "admin"]), 404, "feed_not_found"],
    [request(["markets", "ticker", "UNSUPPORTED"]), 404, "feed_not_found"],
    [request(["eth", ".."]), 400, "invalid_feed_path"],
    [
      request(["ws-token"], {
        query: { path: ["ws-token"], symbol: "UNSUPPORTED" },
      }),
      400,
      "invalid_feed_symbol",
    ],
    [request(["eth", "history"]), 400, "invalid_history_range"],
    [
      request(["eth", "history"], {
        url: "/api/price-feed/eth/history?from=0&to=1&resolution=invalid",
      }),
      400,
      "unsupported_resolution",
    ],
  ];
  for (const [req, status, error] of cases) {
    await t.test(error + String(status), async (t) => {
      const h = harness(
        t,
        async () => assert.fail("Refused requests do not fetch"),
        req,
      );
      await h.completion;
      assert.equal(h.result.status, status);
      assert.deepEqual(h.result.body, { error });
      await h.advance(12_000);
      assert.equal(h.attempts.length, 0);
    });
  }
}
test(refused_requests_make_no_attempt);

async function head_requests_retry_and_finish_without_a_body(t: TestContext) {
  let calls = 0;
  const h = harness(
    t,
    async () => {
      if (++calls === 1)
        return Response.json({ error: restKey }, { status: 503 });
      return Response.json(feedReply);
    },
    request(["markets", "tickers"], { method: "HEAD" }),
  );
  await setImmediate();
  await h.advance(200);
  await h.completion;
  assert.equal(h.result.status, 200);
  assert.equal(h.result.body, undefined);
  assert.equal(h.attempts.length, 2);
}
test(head_requests_retry_and_finish_without_a_body);
