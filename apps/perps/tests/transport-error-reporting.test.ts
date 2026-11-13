import assert from "node:assert/strict";
import { once } from "node:events";
import { createServer, type RequestListener } from "node:http";
import type { AddressInfo } from "node:net";
import { test } from "node:test";
import { createPublicClient, encodeErrorResult, http, RpcRequestError } from "viem";

async function fixtureServer(listener: RequestListener) {
  const server = createServer(listener);
  server.listen(0, "127.0.0.1");
  await once(server, "listening");
  const origin = `http://127.0.0.1:${(server.address() as AddressInfo).port}`;
  process.env.NEXT_PUBLIC_PERPS_GATEWAY_URL = origin;
  process.env.NEXT_PUBLIC_PAXSCAN_API_URL = origin;
  process.env.NEXT_PUBLIC_PAXSCAN_WS_URL = origin.replace("http:", "ws:");
  process.env.NEXT_PUBLIC_PAXSCAN_BASE_URL = origin;
  return {
    origin,
    async close() {
      const closed = new Promise<void>((resolve, reject) => {
        server.close((error) => error ? reject(error) : resolve());
      });
      server.closeAllConnections();
      await closed;
    },
  };
}

async function rejection(promise: Promise<unknown>): Promise<unknown> {
  let rejected: unknown;
  await assert.rejects(promise, (error: unknown) => {
    rejected = error;
    return true;
  });
  return rejected;
}

async function fetch_timeout_classification() {
  const fixture = await fixtureServer(() => {});
  try {
    const { fetchWithPolicy, FetchPolicyError } = await import("../src/core/net/fetch");
    const { classifyError } = await import("../src/core/errors/classify");
    const { ERROR_CATALOG } = await import("../src/core/errors/catalog");
    const raw = await rejection(fetchWithPolicy(fixture.origin, { timeoutMs: 50, retries: 0 }));
    assert.ok(raw instanceof FetchPolicyError);
    assert.equal(raw.kind, "timeout");
    assert.equal(raw.message, "Request timed out");
    const error = classifyError(raw, "fetch/timeout");
    assert.equal(error.code, "API_TIMEOUT");
    assert.equal(error.message, ERROR_CATALOG.API_TIMEOUT.message);
    assert.equal(error.suggestedFix, ERROR_CATALOG.API_TIMEOUT.suggestedFix);
    assert.equal(error.isRetryable, true);
    assert.equal(error.isReportable, true);
    assert.equal(error.context, "fetch/timeout");
    assert.equal(error.cause, raw);
    // The typed producer contract must win even if diagnostic text changes.
    raw.message = "wallet timeout: execution reverted";
    assert.equal(classifyError(raw).code, "API_TIMEOUT");
  } finally {
    await fixture.close();
  }
}

async function fetch_cancellation_classification() {
  const controller = new AbortController();
  const fixture = await fixtureServer(() => controller.abort());
  try {
    const { fetchWithPolicy, FetchPolicyError } = await import("../src/core/net/fetch");
    const { classifyError } = await import("../src/core/errors/classify");
    const raw = await rejection(fetchWithPolicy(fixture.origin, {
      signal: controller.signal, timeoutMs: 2000, retries: 0,
    }));
    assert.ok(raw instanceof FetchPolicyError);
    assert.equal(raw.kind, "abort");
    assert.equal(raw.message, "Request cancelled");
    const error = classifyError(raw, "fetch/cancel");
    assert.equal(error.code, "RUNTIME_ABORTED");
    assert.equal(error.severity, "info");
    assert.equal(error.isReportable, false);
    assert.equal(error.suggestedFix, null);
    assert.equal(error.cause, raw);
  } finally {
    await fixture.close();
  }
}

async function fetch_transport_failure_classification() {
  const fixture = await fixtureServer((request) => request.socket.destroy());
  try {
    const { fetchWithPolicy, FetchPolicyError } = await import("../src/core/net/fetch");
    const { classifyError } = await import("../src/core/errors/classify");
    const { ERROR_CATALOG } = await import("../src/core/errors/catalog");
    const raw = await rejection(fetchWithPolicy(fixture.origin, { timeoutMs: 2000, retries: 0 }));
    assert.ok(raw instanceof FetchPolicyError);
    assert.equal(raw.kind, "network");
    assert.equal(raw.message, "Network unavailable");
    const error = classifyError(raw, "fetch/network");
    assert.equal(error.code, "API_OFFLINE");
    assert.equal(error.message, ERROR_CATALOG.API_OFFLINE.message);
    assert.equal(error.suggestedFix, ERROR_CATALOG.API_OFFLINE.suggestedFix);
    assert.equal(error.isRetryable, true);
    assert.equal(error.isReportable, false);
    assert.equal(error.cause, raw);
  } finally {
    await fixture.close();
  }
}

async function perps_refusal_guidance() {
  const refusals: `0x${string}`[] = [];
  const requests: string[] = [];
  const fixtureErrors: unknown[] = [];
  const fixture = await fixtureServer((request, response) => {
    void (async () => {
      const chunks: Buffer[] = [];
      for await (const chunk of request) chunks.push(Buffer.from(chunk));
      const rpc = JSON.parse(Buffer.concat(chunks).toString()) as { id: number; method: string };
      requests.push(rpc.method);
      const data = refusals.shift();
      assert.ok(data, "every RPC request has an ABI-encoded refusal");
      response.writeHead(200, { "content-type": "application/json" });
      response.end(JSON.stringify({
        jsonrpc: "2.0", id: rpc.id, error: { code: 3, message: "execution reverted", data },
      }));
    })().catch((error) => {
      fixtureErrors.push(error);
      response.destroy(error instanceof Error ? error : new Error(String(error)));
    });
  });
  try {
    const { PerpsPrecompileABI } = await import("../src/core/network/abi/PerpsPrecompile");
    const { classifyError } = await import("../src/core/errors/classify");
    const { ERROR_CATALOG } = await import("../src/core/errors/catalog");
    const { alertErrorText } = await import("../src/widgets/alerts/AlertsPanel");
    const client = createPublicClient({ transport: http(fixture.origin, { retryCount: 0 }) });
    for (const name of ["PerpsDeadlineExpired", "PerpsNoLiquidity"] as const) {
      const data = encodeErrorResult({ abi: PerpsPrecompileABI, errorName: name });
      refusals.push(data);
      const raw = await rejection(client.request({
        method: "eth_call", params: [{ to: "0x0000000000000000000000000000000000001015", data: "0x" }, "latest"],
      }));
      assert.ok(raw instanceof Error);
      // viem preserves the node refusal in its cause chain.
      let node: unknown = raw;
      while (node instanceof Error && !(node instanceof RpcRequestError)) node = node.cause;
      assert.ok(node instanceof RpcRequestError);
      assert.equal(node.code, 3);
      assert.equal(node.data, data);
      const error = classifyError(raw, "alerts/create");
      assert.equal(error.cause, raw);
      assert.equal(error.context, "alerts/create");
      if (name === "PerpsDeadlineExpired") {
        assert.equal(error.code, "RPC_REVERT");
        assert.equal(error.message, "The transaction deadline passed before it was executed.");
        assert.equal(alertErrorText(error), `${error.message} ${ERROR_CATALOG.RPC_REVERT.suggestedFix}`);
        assert.doesNotMatch(alertErrorText(error), /slippage/i);
      } else {
        assert.equal(error.code, "TRADE_INSUFFICIENT_LIQUIDITY");
        assert.equal(error.message, ERROR_CATALOG.TRADE_INSUFFICIENT_LIQUIDITY.message);
        assert.equal(alertErrorText(error), "There isn't enough on-book size to fill that order at the price you wanted. Reduce the size or use a more aggressive price.");
      }
      assert.deepEqual(error.fingerprint, [error.code, "alerts/create", name]);
      assert.equal(node.code, 3, "classification preserves the node code");
      assert.equal(node.data, data, "classification preserves the revert bytes");
    }
    assert.deepEqual(requests, ["eth_call", "eth_call"]);
    assert.deepEqual(fixtureErrors, []);
  } finally {
    await fixture.close();
  }
}

async function simultaneous_first_sentry_reports() {
  const events: import("@sentry/nextjs").Event[] = [];
  const fixtureErrors: unknown[] = [];
  const fixture = await fixtureServer((request, response) => {
    void (async () => {
      const chunks: Buffer[] = [];
      for await (const chunk of request) chunks.push(Buffer.from(chunk));
      const lines = Buffer.concat(chunks).toString().trim().split("\n");
      for (let i = 1; i < lines.length; i += 2) {
        if (JSON.parse(lines[i]).type === "event") events.push(JSON.parse(lines[i + 1]));
      }
      response.writeHead(200, { "content-type": "application/json" });
      response.end("{}");
    })().catch((error) => {
      fixtureErrors.push(error);
      response.destroy(error instanceof Error ? error : new Error(String(error)));
    });
  });
  const Sentry = await import("@sentry/nextjs");
  Sentry.init({
    dsn: `${fixture.origin.replace("://", "://public@")}/1`,
    defaultIntegrations: false,
    autoSessionTracking: false,
    tracesSampleRate: 0,
  });
  try {
    const { reportError, leaveBreadcrumb } = await import("../src/core/errors/report");
    await Promise.all([
      reportError(new Error("first concurrent failure"), { context: "repair/first", tags: { report: "first" } }),
      reportError(new Error("second concurrent failure"), { context: "repair/second", tags: { report: "second" } }),
      leaveBreadcrumb("repair", "concurrent breadcrumb", { task: "22.33.80" }),
    ]);
    await reportError(new Error("after initialization"), { tags: { report: "after" } });
    assert.equal(await Sentry.flush(3000), true);
    assert.deepEqual(fixtureErrors, []);
    assert.equal(events.length, 3, "both simultaneous first reports and the subsequent report reach Sentry transport");
    assert.deepEqual(events.map((event) => event.tags?.report).sort(), ["after", "first", "second"]);
    for (const label of ["first", "second"]) {
      const event = events.find((item) => item.tags?.report === label);
      assert.ok(event);
      assert.equal(event.tags?.["error.code"], "UNKNOWN");
      assert.equal(event.tags?.["error.context"], `repair/${label}`);
      assert.equal(event.exception?.values?.[0].value, `${label} concurrent failure`);
      assert.equal(event.contexts?.user_facing_error?.code, "UNKNOWN");
    }
    const after = events.find((event) => event.tags?.report === "after");
    assert.ok(after?.breadcrumbs?.some((crumb) => crumb.category === "repair" && crumb.message === "concurrent breadcrumb"));
  } finally {
    await Sentry.close(3000);
    await fixture.close();
  }
}

test(fetch_timeout_classification);
test(fetch_cancellation_classification);
test(fetch_transport_failure_classification);
test(perps_refusal_guidance);
test(simultaneous_first_sentry_reports);
