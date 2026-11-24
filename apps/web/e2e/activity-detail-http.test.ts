import assert from "node:assert/strict";
import { once } from "node:events";
import { createServer } from "node:net";
import { existsSync, readFileSync } from "node:fs";
import { registerHooks } from "node:module";
import test, { type TestContext } from "node:test";
import { fileURLToPath } from "node:url";
import ts from "typescript";

import { createHumanApiClient, decodeActivityEntryDetail, type JsonValue } from "../src/api/generated/index.ts";

registerHooks({
  resolve(specifier, context, nextResolve) {
    if (specifier.startsWith(".") && context.parentURL !== undefined) {
      const candidate = new URL(specifier, context.parentURL);
      for (const suffix of [".ts", ".tsx", "/index.ts"]) {
        const source = new URL(candidate.href + suffix);
        if (existsSync(source)) return nextResolve(source.href, context);
      }
    }
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

const { createElement } = await import("react");
const { renderToStaticMarkup } = await import("react-dom/server");
const { copyEntry } = await import("../copy/runtime.ts");
const { PrivacyModeProvider } = await import("../src/settings/privacy.tsx");
const { ActivityDetailAmount, ActivityDetailFacts } = await import("../src/journeys/activity/facts.tsx");
const { plainSentence, stageView, validatedDetail } = await import("../src/journeys/activity/model.ts");

async function activity_detail_shape_producer_http_decoder_and_rendering(t: TestContext) {
  const recordings = JSON.parse(readFileSync(new URL(
    "../src/journeys/activity/activity-detail-http.json", import.meta.url,
  ), "utf8")) as Record<string, string>;
  const requests: string[] = [];
  const server = createServer((socket) => {
    let request = "";
    socket.setTimeout(10_000, () => socket.destroy(new Error("HTTP request deadline")));
    socket.on("data", (chunk: Buffer) => {
      request += chunk.toString("utf8");
      if (!request.includes("\r\n\r\n")) return;
      const [method, path] = request.split("\r\n", 1)[0]!.split(" ");
      assert.equal(method, "GET");
      assert.ok(path !== undefined && Object.hasOwn(recordings, path), "recorded producer route");
      const response = recordings[path];
      assert.ok(response !== undefined);
      assert.ok(response.startsWith("HTTP/1.1 200 OK\r\n"));
      requests.push(path);
      socket.end(response);
    });
  });
  t.after(() => new Promise<void>((resolve, reject) => {
    server.close((error) => { if (error) reject(error); else resolve(); });
  }));
  server.listen(0, "127.0.0.1");
  await once(server, "listening");
  const address = server.address();
  assert.ok(address !== null && typeof address !== "string");
  const baseUrl = `http://127.0.0.1:${address.port}`;
  const client = createHumanApiClient({ baseUrl });
  const cases = [
    ["act_repairdeposit", "activity.deposit.summary", "in", "done"],
    ["act_repairwithdrawal", "activity.withdrawal.summary", "out", "done"],
    ["act_repairapproval", "activity.approval.progress", undefined, "waiting-for-you"],
    ["act_repairsecurity", "activity.security.progress", undefined, "processing"],
    ["act_repairrefusedkept", "activity.refused.money_kept", undefined, "refused"],
    ["act_repairrefusedleft", "activity.refused.money_left", undefined, "refused"],
  ] as const;

  for (const [id, summaryKey, direction, state] of cases) {
    const response = await fetch(`${baseUrl}/v1/activity/${id}`, { signal: AbortSignal.timeout(10_000) });
    assert.equal(response.status, 200);
    const envelope = await response.json() as { ok: boolean; result: JsonValue };
    assert.equal(envelope.ok, true);
    const decoded = decodeActivityEntryDetail(envelope.result, "genuine activity HTTP output");
    const detail = validatedDetail(await client.activityEntry(id));
    assert.deepEqual(detail, decoded);
    assert.equal(detail.entry_id, id);
    assert.equal(detail.state, state);
    assert.equal(detail.occurred_at, "2026-08-15T10:30:00Z");
    assert.equal(detail.summary_copy_key, summaryKey);
    assert.equal(plainSentence(detail), copyEntry(summaryKey).message);
    assert.ok(copyEntry(detail.state_copy_key).message.length > 0);
    assert.equal(detail.direction, direction);
    assert.ok(detail.stages.length > 0);
    for (const stage of detail.stages) {
      assert.equal(stageView(stage).label, copyEntry(stage.copy_key).message);
    }
    const html = renderToStaticMarkup(createElement(PrivacyModeProvider, {
      principalScope: "activity-contract",
      children: createElement(ActivityDetailFacts, { detail }),
    }));
    assert.ok(html.includes(copyEntry(summaryKey).message));
    assert.ok(html.includes("15 Aug 2026, 10:30 UTC"));
    if (direction !== undefined) {
      assert.ok(detail.money);
      assert.ok(detail.fees);
      assert.deepEqual(detail.money, { amount: 25n, currency: "LXP" });
      assert.deepEqual(detail.fees, { amount: 1n, currency: "LXP" });
      assert.equal(detail.evidence[0]?.verification, "receipt-verified");
      assert.equal(detail.evidence[0]?.class, "layerx-receipt");
      assert.ok(html.includes("Amount in base units of LXP"));
      const amount = renderToStaticMarkup(createElement(ActivityDetailAmount, { money: detail.money, direction }));
      const fee = renderToStaticMarkup(createElement(ActivityDetailAmount, { money: detail.fees, fee: true }));
      assert.match(amount, /LXP/u);
      const sign = direction === "out" ? "-" : "+";
      const word = direction === "out" ? "Money out" : "Money in";
      assert.ok(amount.includes(`class="sr-only">${sign}LXP\u00a025, ${word}</span>`));
      assert.ok(amount.includes(`>${sign} 25 LXP</span>`));
      assert.ok(fee.includes('class="sr-only">-LXP\u00a01, Money out</span>'));
      assert.ok(fee.includes('>- 1 LXP</span>'));
      assert.doesNotMatch(amount, /25\.00/u);
      assert.ok(amount.includes("Amount in base units of LXP"));
    } else {
      assert.equal(Object.hasOwn(envelope.result as object, "money"), false);
      assert.equal(Object.hasOwn(envelope.result as object, "fees"), false);
      assert.equal(Object.hasOwn(envelope.result as object, "direction"), false);
      assert.equal(detail.evidence.length, 0);
      assert.ok(html.includes(copyEntry("activity.detail.no_amount").message));
      assert.ok(html.includes(copyEntry("activity.detail.no_fee").message));
    }
  }
  assert.deepEqual(Object.keys(recordings).sort(), cases.map(([id]) => `/v1/activity/${id}`).sort());
  assert.equal(requests.length, cases.length * 2);
  for (const [id] of cases) {
    assert.equal(requests.filter((path) => path === `/v1/activity/${id}`).length, 2);
  }
}

test(activity_detail_shape_producer_http_decoder_and_rendering);
