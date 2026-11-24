import assert from "node:assert/strict";
import { existsSync, readFileSync } from "node:fs";
import { createServer } from "node:http";
import { registerHooks } from "node:module";
import test from "node:test";
import { fileURLToPath } from "node:url";
import { Window } from "happy-dom";
import ts from "typescript";
import type { JsonValue, MoveQuoteRequest } from "../src/api/generated/index.ts";

registerHooks({
  resolve(specifier, context, nextResolve) {
    if (specifier.startsWith("@layerx/ui/")) {
      const source = new URL(
        `../packages/layerx-ui/src/${specifier.slice("@layerx/ui/".length)}`,
        import.meta.url,
      );
      for (const suffix of [".ts", ".tsx"]) {
        const candidate = new URL(source.href + suffix);
        if (existsSync(candidate)) return nextResolve(candidate.href, context);
      }
    }
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
        format: "module",
        shortCircuit: true,
        source: ts.transpileModule(readFileSync(new URL(url), "utf8"), {
          fileName: fileURLToPath(url),
          compilerOptions: {
            target: ts.ScriptTarget.ES2022,
            module: ts.ModuleKind.ESNext,
            jsx: ts.JsxEmit.ReactJSX,
          },
        }).outputText,
      };
    }
    const loaded = nextLoad(url, context);
    return loaded.format === "commonjs" ? { ...loaded, source: undefined } : loaded;
  },
});

const browser = new Window({ url: "http://localhost/app/move" });
Object.assign(globalThis, {
  window: browser,
  document: browser.document,
  HTMLElement: browser.HTMLElement,
  HTMLInputElement: browser.HTMLInputElement,
  HTMLFormElement: browser.HTMLFormElement,
  HTMLButtonElement: browser.HTMLButtonElement,
  Element: browser.Element,
  Node: browser.Node,
  NodeFilter: browser.NodeFilter,
  DocumentFragment: browser.DocumentFragment,
  Event: browser.Event,
  EventTarget: browser.EventTarget,
  FocusEvent: browser.FocusEvent,
  MouseEvent: browser.MouseEvent,
  PointerEvent: browser.PointerEvent,
  KeyboardEvent: browser.KeyboardEvent,
  CustomEvent: browser.CustomEvent,
  MutationObserver: browser.MutationObserver,
  ResizeObserver: browser.ResizeObserver,
  getComputedStyle: browser.getComputedStyle.bind(browser),
  requestAnimationFrame: browser.requestAnimationFrame.bind(browser),
  cancelAnimationFrame: browser.cancelAnimationFrame.bind(browser),
  fetch: browser.fetch.bind(browser),
  IS_REACT_ACT_ENVIRONMENT: true,
});

const { act, createElement } = await import("react");
const { createRoot } = await import("react-dom/client");
const { AppRouterContext } = await import("next/dist/shared/lib/app-router-context.shared-runtime.js");
const { PathnameContext } = await import("next/dist/shared/lib/hooks-client-context.shared-runtime.js");
const api = await import("../src/api/generated/index.ts");
const model = await import("../src/journeys/move/model.ts");
const { MoveMoney } = await import("../src/journeys/move/move.tsx");
const { AuthenticatedShell } = await import("../src/shell/app-shell.tsx");
const { ShellSelector } = await import("../src/shell/selector.ts");
const { PrivacyModeProvider } = await import("../src/settings/privacy.tsx");
const { NotificationCenterProvider } = await import("../src/journeys/notifications/store.tsx");
const { copyEntry } = await import("../copy/runtime.ts");

function golden(name: string): JsonValue {
  const recording = JSON.parse(readFileSync(new URL(
    `../../../services/human/schema/human-api/golden/${name}.response.json`,
    import.meta.url,
  ), "utf8")) as { body: { result: JsonValue } };
  return recording.body.result;
}

const cases = [
  ["9007199254740990", "LXP\u00a09,007,199,254,740,990"],
  ["9007199254740991", "LXP\u00a09,007,199,254,740,991"],
  ["9007199254740992", "LXP\u00a09,007,199,254,740,992"],
  ["9007199254740993", "LXP\u00a09,007,199,254,740,993"],
  ["99999999999999999999999999999999999999", "LXP\u00a099,999,999,999,999,999,999,999,999,999,999,999,999"],
] as const;

function exactMoveMoneyFormattingKeepsBaseUnitsAndCurrency() {
  for (const [input, expected] of cases) {
    const amount = model.parseMoveAmount(input);
    assert.equal(amount, BigInt(input));
    assert.equal(model.moneyWords({ amount: amount!, currency: "LXP" }), expected);
  }
  assert.equal(model.moneyWords({ amount: 0n, currency: "USD" }), "USD\u00a00");
  assert.equal(model.moneyWords({ amount: 12345n, currency: "USD" }), "USD\u00a012,345");
  assert.equal(model.moneyWords({ amount: 12345n, currency: "JPY" }), "JPY\u00a012,345");
  assert.equal(model.moneyWords({ amount: 9007199254740993n, currency: "EUR" }, "de-DE"),
    "9.007.199.254.740.993\u00a0EUR");
  assert.equal(model.parseMoveAmount(" 0001 "), 1n);
  assert.throws(() => model.moneyWords({ amount: 1n, currency: "lxp" }), /Currency/u);
  assert.throws(() => model.moneyWords({ amount: 1n, currency: "LXP" }, ""), RangeError);
}

async function largeMoveAmountsRenderThroughInputAndReview() {
  const balance = api.decodeAccountBalance(golden("account.balance"), "recorded balance");
  const sessions = api.decodeSessionList(golden("session.list"), "recorded sessions");
  const quote = api.decodeMoveQuote(golden("move.quote"), "recorded quote");
  const requests: { wire: JsonValue; decoded: MoveQuoteRequest }[] = [];
  const failures: unknown[] = [];
  const feeEstimate = 9007199254740993n;
  const feeCeiling = BigInt(cases[4][0]);
  const server = createServer(async (request, response) => {
    try {
      const path = new URL(request.url!, "http://localhost").pathname;
      let result: JsonValue;
      if (path === "/human/v1/moves/quote") {
        assert.equal(request.method, "POST");
        const chunks: Buffer[] = [];
        for await (const chunk of request) chunks.push(Buffer.from(chunk));
        const wire = JSON.parse(Buffer.concat(chunks).toString("utf8")) as JsonValue;
        const decoded = api.decodeMoveQuoteRequest(wire, "received quote request");
        requests.push({ wire, decoded });
        result = api.encodeMoveQuote({
          ...quote,
          money: decoded.money,
          fee_estimate: { amount: feeEstimate, currency: decoded.money.currency },
          fee_ceiling: { amount: feeCeiling, currency: decoded.money.currency },
          expires_at: new Date(Date.now() + 300_000).toISOString(),
        });
      } else if (path === "/human/v1/stream") {
        assert.equal(request.method, "POST");
        result = golden("stream.open");
      } else if (path.startsWith("/human/v1/stream/")) {
        assert.equal(request.method, "GET");
        const position = api.decodeStreamPosition(golden("stream.open"), "recorded stream position");
        assert.equal(path, `/human/v1/stream/${position.cursor}`);
        response.writeHead(200, {
          "Content-Type": "text/event-stream; charset=utf-8",
          "X-LayerX-Stream-Profile": "2",
        });
        response.flushHeaders();
        return;
      } else {
        assert.equal(request.method, "GET");
        switch (path) {
          case "/human/v1/agents":
            result = api.encodeAgentPage({ agents: [], next_cursor: "" });
            break;
          case "/human/v1/account/balance":
            result = api.encodeAccountBalance(balance);
            break;
          case "/human/v1/sessions":
            result = api.encodeSessionList(sessions);
            break;
          case "/human/v1/notifications":
            result = api.encodeNotificationPage({ groups: [], next_cursor: "", unread_count: 0 });
            break;
          case "/human/v1/approvals":
            result = api.encodeApprovalPage({ approvals: [], next_cursor: "" });
            break;
          case "/human/v1/program-approvals/page/start":
            result = api.encodeProgramApprovalPage({ approvals: [], next_cursor: "" });
            break;
          default:
            assert.fail(`Unexpected route: ${path}`);
        }
      }
      response.writeHead(200, { "Content-Type": "application/json" });
      response.end(JSON.stringify({ ok: true, result, trace: "trc_move_amount_render" }));
    } catch (error) {
      failures.push(error);
      response.writeHead(500).end();
    }
  });
  await new Promise<void>((resolve, reject) => {
    server.once("error", reject);
    server.listen(0, "127.0.0.1", resolve);
  });
  const address = server.address();
  assert.ok(address !== null && typeof address !== "string");
  browser.happyDOM.setURL(`http://127.0.0.1:${address.port}/app/move`);
  browser.localStorage.setItem(model.ACCOUNT_STORAGE_KEY, balance.account_id);
  const container = browser.document.createElement("div");
  browser.document.body.appendChild(container);
  const root = createRoot(container as unknown as HTMLElement);
  const router = {
    bfcacheId: "move-amount-regression",
    back() { browser.history.back(); },
    forward() { browser.history.forward(); },
    refresh() { browser.dispatchEvent(new browser.Event("refresh")); },
    push(href: string) { browser.history.pushState(null, "", href); },
    replace(href: string) { browser.history.replaceState(null, "", href); },
    prefetch(href: string) { void browser.fetch(href); },
  };
  function button(label: string) {
    const found = [...container.querySelectorAll("button")].find((node) => node.textContent === label);
    assert.ok(found, `Missing mounted control: ${label}`);
    return found;
  }
  async function click(label: string) {
    await act(async () => { button(label).click(); });
  }
  async function input(label: string, value: string) {
    const fieldLabel = [...container.querySelectorAll("label")].find((node) => node.textContent === label);
    assert.ok(fieldLabel, `Missing mounted field: ${label}`);
    const field = browser.document.getElementById(fieldLabel.htmlFor);
    assert.ok(field instanceof browser.HTMLInputElement);
    await act(async () => {
      const setter = Object.getOwnPropertyDescriptor(browser.HTMLInputElement.prototype, "value")?.set;
      assert.ok(setter);
      setter.call(field, value);
      field.dispatchEvent(new browser.Event("input", { bubbles: true }));
    });
    assert.equal(field.value, value);
    return field;
  }
  async function until(check: () => boolean) {
    for (let attempt = 0; attempt < 100; attempt++) {
      if (check()) return;
      await act(async () => { await new Promise((resolve) => setTimeout(resolve, 10)); });
    }
    assert.deepEqual(failures, []);
    assert.ok(check(), "The mounted move journey did not reach its expected state");
  }
  try {
    await act(async () => {
      root.render(createElement(AppRouterContext.Provider, { value: router },
        createElement(PathnameContext.Provider, { value: "/app/move" },
          createElement(PrivacyModeProvider, {
            principalScope: "move-amount-render",
            children: createElement(NotificationCenterProvider, {
              children: createElement(AuthenticatedShell, {
                initialSelection: ShellSelector.server({ viewportWidth: 1024, pointer: "none" }),
                children: createElement(MoveMoney),
              }),
            }),
          }))));
    });
    const otherAccount = copyEntry("move.destination.other").message;
    await until(() => [...container.querySelectorAll("button")].some((node) => node.textContent?.includes(otherAccount)));
    const destinationOption = [...container.querySelectorAll("button")].find((node) => node.textContent?.includes(otherAccount));
    assert.ok(destinationOption);
    await act(async () => { destinationOption.click(); });
    await input(copyEntry("move.destination.other.hint").message, "account-destination");
    assert.equal(button("Continue").disabled, false);
    await click("Continue");
    const amountLabel = "Amount in base units of LXP";
    assert.equal(button("Continue").disabled, true);
    for (const invalid of ["0", "-1", "1.5", "1e3", "letters", " ", "1".repeat(39)]) {
      const field = await input(amountLabel, invalid);
      assert.equal(model.parseMoveAmount(invalid), undefined);
      assert.equal(field.getAttribute("aria-invalid"), "true");
      const errorId = field.getAttribute("aria-describedby");
      assert.ok(errorId);
      const error = browser.document.getElementById(errorId);
      assert.equal(error?.getAttribute("role"), "alert");
      assert.equal(error?.textContent, model.MOVE_AMOUNT_VALIDATION_MESSAGE);
      assert.equal(button("Continue").disabled, true);
      assert.equal(requests.length, 0, "invalid input must never request a quote");
    }
    await input(amountLabel, "");
    assert.equal(container.querySelector('[role="alert"]'), null);
    assert.equal(button("Continue").disabled, true);
    for (const [index, [amount, expected]] of cases.entries()) {
      const field = await input(amountLabel, amount);
      assert.equal(field.getAttribute("aria-invalid"), "false");
      assert.equal(field.getAttribute("aria-describedby"), null);
      assert.equal(button("Continue").disabled, false);
      assert.ok([...container.querySelectorAll("dd")].some((node) => node.textContent === expected),
        "the live wizard summary must render the exact amount before a quote exists");
      await click("Continue");
      const moveLabel = copyEntry("home.actions.move").message;
      await until(() => !button(moveLabel).disabled);
      assert.equal(requests.length, index + 1);
      assert.deepEqual(requests[index]?.wire, {
        source: balance.account_id,
        destination: "account-destination",
        money: { amount, currency: "LXP" },
      });
      assert.equal(requests[index]?.decoded.money.amount, BigInt(amount));
      const figures = [...container.querySelectorAll('[data-private-figure="visible"]')];
      assert.equal(figures.filter((node) => node.textContent === expected).length, 2,
        "both the quote review and summary must preserve every base-unit digit");
      assert.ok(figures.some((node) => node.textContent?.includes(cases[3][1]) && node.textContent.includes(cases[4][1])),
        "the rendered review must also preserve the large fee estimate and ceiling");
      assert.deepEqual(failures, []);
      await click("Back");
    }
  } finally {
    await act(async () => { root.unmount(); });
    container.remove();
    await browser.happyDOM.abort();
    await new Promise<void>((resolve, reject) => {
      server.close((error) => { if (error) reject(error); else resolve(); });
      server.closeAllConnections();
    });
  }
}

test(exactMoveMoneyFormattingKeepsBaseUnitsAndCurrency);
test(largeMoveAmountsRenderThroughInputAndReview);
